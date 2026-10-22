// Copyright (c) 2026 Accenture, All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package api

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/iterator"
)

const (
	// objectsListDefaultPageSize is used when pageSize is omitted.
	// GCS objects.list returns at most 1000 items per page.
	objectsListDefaultPageSize = 1000
	objectsListMaxPageSize     = 1000
)

// handleListObjects lists object names by GCS prefix.
// Unlike POST .../objects/filter, this does not apply a metadata filter,
// accepts an empty prefix, treats a prefix without a trailing slash as a
// string prefix, and pages with pageToken / pageSize so the caller does not
// receive the whole bucket.
func (s *Server) handleListObjects(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")
	if !s.requireBucket(w, r, bucket) {
		return
	}
	prefix := strings.TrimSpace(r.URL.Query().Get("prefix"))
	if err := validateObjectsListPrefix(prefix); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	pageSize, err := parseObjectsListPageSize(r.URL.Query().Get("pageSize"))
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if s.GCS == nil {
		writeOpErr(w, errors.New("gcs client is not configured"))
		return
	}

	it := s.GCS.Bucket(bucket).Objects(r.Context(), &storage.Query{Prefix: prefix})
	pager := iterator.NewPager(it, pageSize, strings.TrimSpace(r.URL.Query().Get("pageToken")))
	var attrs []*storage.ObjectAttrs
	next, err := pager.NextPage(&attrs)
	if err != nil {
		var gerr *googleapi.Error
		if errors.As(err, &gerr) && gerr.Code == http.StatusBadRequest {
			writeErr(w, http.StatusBadRequest, "invalid pageToken")
			return
		}
		writeOpErr(w, err)
		return
	}

	objects := make([]filteredObject, 0, len(attrs))
	for _, a := range attrs {
		if a == nil {
			continue
		}
		objects = append(objects, filteredObject{
			ObjectPath:  a.Name,
			GSURI:       fmt.Sprintf("gs://%s/%s", bucket, a.Name),
			Size:        a.Size,
			ContentType: a.ContentType,
			Updated:     a.Updated.UTC().Format(time.RFC3339),
		})
	}
	resp := map[string]any{
		"count":   len(objects),
		"objects": objects,
	}
	if next != "" {
		resp["nextPageToken"] = next
	}
	writeJSON(w, http.StatusOK, resp)
}

func validateObjectsListPrefix(prefix string) error {
	if prefix == "" {
		return nil
	}
	if invalidObjectPath(prefix) {
		return errors.New("invalid object prefix")
	}
	return nil
}

func parseObjectsListPageSize(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return objectsListDefaultPageSize, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > objectsListMaxPageSize {
		return 0, fmt.Errorf("pageSize must be between 1 and %d", objectsListMaxPageSize)
	}
	return n, nil
}
