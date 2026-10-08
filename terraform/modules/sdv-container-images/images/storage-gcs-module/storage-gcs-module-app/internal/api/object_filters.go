// Copyright (c) 2026 Accenture, All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
)

type clientRequestError struct{ msg string }

func (e *clientRequestError) Error() string { return e.msg }

func clientReq(msg string) error { return &clientRequestError{msg} }

func writeQueryErr(w http.ResponseWriter, err error) {
	var ce *clientRequestError
	if errors.As(err, &ce) {
		writeErr(w, http.StatusBadRequest, ce.Error())
		return
	}
	writeOpErr(w, err)
}

type objectFilterRequest struct {
	Path                 string   `json:"path"`
	FilterType           string   `json:"filterType,omitempty"`
	MetadataFilters      []string `json:"metadataFilters,omitempty"`
	IncludeMetadata      bool     `json:"includeMetadata,omitempty"`
	IncludeMatchedFields bool     `json:"includeMatchedFields,omitempty"`
	IncludeStorageClass  bool     `json:"includeStorageClass,omitempty"`
}

type batchMetadataUpdateRequest struct {
	Path            string            `json:"path"`
	FilterType      string            `json:"filterType,omitempty"`
	MetadataFilters []string          `json:"metadataFilters,omitempty"`
	SetMetadata     map[string]string `json:"setMetadata"`
}

type batchMetadataRemoveRequest struct {
	Path            string   `json:"path"`
	FilterType      string   `json:"filterType,omitempty"`
	MetadataFilters []string `json:"metadataFilters,omitempty"`
	DeleteKeys      []string `json:"deleteKeys,omitempty"`
	RemoveAll       bool     `json:"removeAll,omitempty"`
}

type batchStorageClassUpdateRequest struct {
	Path            string   `json:"path"`
	FilterType      string   `json:"filterType,omitempty"`
	MetadataFilters []string `json:"metadataFilters,omitempty"`
	StorageClass    string   `json:"storageClass"`
}

type batchDeleteRequest struct {
	Path            string   `json:"path"`
	FilterType      string   `json:"filterType,omitempty"`
	MetadataFilters []string `json:"metadataFilters,omitempty"`
	DryRun          bool     `json:"dryRun,omitempty"`
}

type metadataExpectation struct {
	Key      string
	Value    string
	HasValue bool
}

type filteredObject struct {
	ObjectPath      string            `json:"objectPath"`
	GSURI           string            `json:"gsUri"`
	Size            int64             `json:"size,omitempty"`
	ContentType     string            `json:"contentType,omitempty"`
	Metadata        map[string]string `json:"metadata,omitempty"`
	MatchedMetadata map[string]string `json:"matchedMetadata,omitempty"`
	StorageClass    string            `json:"storageClass,omitempty"`
	Updated         string            `json:"updated,omitempty"`
}

func (s *Server) handleFilterObjects(w http.ResponseWriter, r *http.Request) {
	if !s.requireBucket(w, r, r.PathValue("bucket")) {
		return
	}
	var body objectFilterRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	objects, err := s.queryFilteredObjects(r.Context(), r.PathValue("bucket"), body)
	if err != nil {
		writeQueryErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"count":   len(objects),
		"objects": objects,
	})
}

func (s *Server) handleStorageClassList(w http.ResponseWriter, r *http.Request) {
	if !s.requireBucket(w, r, r.PathValue("bucket")) {
		return
	}
	var body objectFilterRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	body.IncludeStorageClass = true
	objects, err := s.queryFilteredObjects(r.Context(), r.PathValue("bucket"), body)
	if err != nil {
		writeQueryErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"count":   len(objects),
		"objects": objects,
	})
}

func (s *Server) handleBatchMetadataUpdate(w http.ResponseWriter, r *http.Request) {
	if !s.requireBucket(w, r, r.PathValue("bucket")) {
		return
	}
	var body batchMetadataUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if len(body.SetMetadata) == 0 {
		writeErr(w, http.StatusBadRequest, "setMetadata required")
		return
	}
	filterReq := objectFilterRequest{
		Path:                 body.Path,
		FilterType:           body.FilterType,
		MetadataFilters:      body.MetadataFilters,
		IncludeMetadata:      true,
		IncludeMatchedFields: true,
	}
	objects, err := s.queryFilteredObjects(r.Context(), r.PathValue("bucket"), filterReq)
	if err != nil {
		writeQueryErr(w, err)
		return
	}
	if len(objects) == 0 {
		writeErr(w, http.StatusNotFound, "no matching objects found")
		return
	}

	results := make([]filteredObject, 0, len(objects))
	for _, obj := range objects {
		attrs, err := s.GCS.Bucket(r.PathValue("bucket")).Object(obj.ObjectPath).Update(r.Context(), storage.ObjectAttrsToUpdate{
			Metadata: body.SetMetadata,
		})
		if err != nil {
			writeOpErr(w, err)
			return
		}
		results = append(results, filteredObject{
			ObjectPath:      obj.ObjectPath,
			GSURI:           obj.GSURI,
			Metadata:        attrs.Metadata,
			MatchedMetadata: obj.MatchedMetadata,
			Updated:         attrs.Updated.Format(time.RFC3339),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"updated": len(results),
		"objects": results,
	})
}

func (s *Server) handleBatchMetadataRemove(w http.ResponseWriter, r *http.Request) {
	if !s.requireBucket(w, r, r.PathValue("bucket")) {
		return
	}
	var body batchMetadataRemoveRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if !body.RemoveAll && len(body.DeleteKeys) == 0 {
		writeErr(w, http.StatusBadRequest, "deleteKeys required when removeAll is false")
		return
	}
	filterReq := objectFilterRequest{
		Path:                 body.Path,
		FilterType:           body.FilterType,
		MetadataFilters:      body.MetadataFilters,
		IncludeMetadata:      true,
		IncludeMatchedFields: true,
	}
	objects, err := s.queryFilteredObjects(r.Context(), r.PathValue("bucket"), filterReq)
	if err != nil {
		writeQueryErr(w, err)
		return
	}
	if len(objects) == 0 {
		writeErr(w, http.StatusNotFound, "no matching objects found")
		return
	}

	results := make([]filteredObject, 0, len(objects))
	for _, obj := range objects {
		updates := storage.ObjectAttrsToUpdate{}
		if body.RemoveAll {
			updates.Metadata = map[string]string{}
			for k := range obj.Metadata {
				updates.Metadata[k] = ""
			}
		} else {
			updates.Metadata = map[string]string{}
			for _, k := range body.DeleteKeys {
				updates.Metadata[k] = ""
			}
		}
		attrs, err := s.GCS.Bucket(r.PathValue("bucket")).Object(obj.ObjectPath).Update(r.Context(), updates)
		if err != nil {
			writeOpErr(w, err)
			return
		}
		results = append(results, filteredObject{
			ObjectPath:      obj.ObjectPath,
			GSURI:           obj.GSURI,
			Metadata:        attrs.Metadata,
			MatchedMetadata: obj.MatchedMetadata,
			Updated:         attrs.Updated.Format(time.RFC3339),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"updated": len(results),
		"objects": results,
	})
}

func (s *Server) handleBatchStorageClassUpdate(w http.ResponseWriter, r *http.Request) {
	if !s.requireBucket(w, r, r.PathValue("bucket")) {
		return
	}
	var body batchStorageClassUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if body.StorageClass == "" {
		writeErr(w, http.StatusBadRequest, "storageClass required")
		return
	}
	filterReq := objectFilterRequest{
		Path:                 body.Path,
		FilterType:           body.FilterType,
		MetadataFilters:      body.MetadataFilters,
		IncludeMatchedFields: true,
		IncludeStorageClass:  true,
	}
	objects, err := s.queryFilteredObjects(r.Context(), r.PathValue("bucket"), filterReq)
	if err != nil {
		writeQueryErr(w, err)
		return
	}
	results := make([]filteredObject, 0, len(objects))
	for _, obj := range objects {
		attrs, err := s.updateObjectStorageClass(r.Context(), r.PathValue("bucket"), obj.ObjectPath, body.StorageClass)
		if err != nil {
			writeOpErr(w, err)
			return
		}
		results = append(results, filteredObject{
			ObjectPath:      obj.ObjectPath,
			GSURI:           obj.GSURI,
			MatchedMetadata: obj.MatchedMetadata,
			StorageClass:    attrs.StorageClass,
			Updated:         attrs.Updated.Format(time.RFC3339),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"updated": len(results),
		"objects": results,
	})
}

func (s *Server) updateObjectStorageClass(ctx context.Context, bucket, objectPath, storageClass string) (*storage.ObjectAttrs, error) {
	obj := s.GCS.Bucket(bucket).Object(objectPath)
	copier := obj.CopierFrom(obj)
	copier.StorageClass = storageClass
	return copier.Run(ctx)
}

func (s *Server) handleBatchDelete(w http.ResponseWriter, r *http.Request) {
	if !s.requireBucket(w, r, r.PathValue("bucket")) {
		return
	}
	var body batchDeleteRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	filterReq := objectFilterRequest{
		Path:                 body.Path,
		FilterType:           body.FilterType,
		MetadataFilters:      body.MetadataFilters,
		IncludeMatchedFields: true,
	}
	objects, err := s.queryFilteredObjects(r.Context(), r.PathValue("bucket"), filterReq)
	if err != nil {
		writeQueryErr(w, err)
		return
	}
	if body.DryRun {
		writeJSON(w, http.StatusOK, map[string]any{
			"dryRun":  true,
			"count":   len(objects),
			"objects": objects,
		})
		return
	}
	deleted := make([]filteredObject, 0, len(objects))
	for _, obj := range objects {
		if err := s.GCS.Bucket(r.PathValue("bucket")).Object(obj.ObjectPath).Delete(r.Context()); err != nil && !errors.Is(err, storage.ErrObjectNotExist) {
			writeOpErr(w, err)
			return
		}
		deleted = append(deleted, obj)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"deleted": len(deleted),
		"objects": deleted,
	})
}

func (s *Server) queryFilteredObjects(ctx context.Context, bucket string, req objectFilterRequest) ([]filteredObject, error) {
	path := strings.TrimSpace(req.Path)
	if path == "" {
		return nil, clientReq("path required")
	}
	objectAttrs, err := s.listMatchingObjects(ctx, bucket, path)
	if err != nil {
		return nil, err
	}
	filterType := normalizeFilterType(req.FilterType)
	expectations := parseMetadataExpectations(req.MetadataFilters)
	includeMetadata := req.IncludeMetadata || req.IncludeMatchedFields

	objects := make([]filteredObject, 0, len(objectAttrs))
	for _, attrs := range objectAttrs {
		if !matchesFilterType(attrs.Metadata, filterType, expectations) {
			continue
		}
		item := filteredObject{
			ObjectPath:  attrs.Name,
			GSURI:       fmt.Sprintf("gs://%s/%s", bucket, attrs.Name),
			Size:        attrs.Size,
			ContentType: attrs.ContentType,
			Updated:     attrs.Updated.Format(time.RFC3339),
		}
		if includeMetadata {
			item.Metadata = cloneMetadata(attrs.Metadata)
		}
		if req.IncludeMatchedFields && len(expectations) > 0 {
			item.MatchedMetadata = matchedMetadata(attrs.Metadata, expectations)
		}
		if req.IncludeStorageClass {
			item.StorageClass = attrs.StorageClass
		}
		objects = append(objects, item)
	}

	sort.Slice(objects, func(i, j int) bool {
		return objects[i].ObjectPath < objects[j].ObjectPath
	})
	return objects, nil
}

func (s *Server) listMatchingObjects(ctx context.Context, bucket string, inputPath string) ([]*storage.ObjectAttrs, error) {
	objectPath, err := normalizeBucketPath(bucket, inputPath)
	if err != nil {
		return nil, err
	}
	hasWildcard := strings.ContainsAny(objectPath, "*?")
	if !hasWildcard && !strings.HasSuffix(objectPath, "/") {
		attrs, err := s.GCS.Bucket(bucket).Object(objectPath).Attrs(ctx)
		if err != nil {
			if errors.Is(err, storage.ErrObjectNotExist) {
				return []*storage.ObjectAttrs{}, nil
			}
			return nil, err
		}
		return []*storage.ObjectAttrs{attrs}, nil
	}

	prefix := objectPath
	var re *regexp.Regexp
	if hasWildcard {
		prefix = wildcardListPrefix(objectPath)
		re, err = wildcardRegexp(objectPath)
		if err != nil {
			return nil, clientReq("invalid wildcard path")
		}
	}

	it := s.GCS.Bucket(bucket).Objects(ctx, &storage.Query{Prefix: prefix})
	var results []*storage.ObjectAttrs
	for {
		attrs, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, err
		}
		if strings.HasSuffix(objectPath, "/") && !hasWildcard {
			results = append(results, attrs)
			continue
		}
		if re != nil && re.MatchString(attrs.Name) {
			results = append(results, attrs)
		}
	}
	return results, nil
}

func normalizeBucketPath(bucket string, input string) (string, error) {
	path := strings.TrimSpace(input)
	if strings.HasPrefix(path, "gs://") {
		trimmed := strings.TrimPrefix(path, "gs://")
		parts := strings.SplitN(trimmed, "/", 2)
		if len(parts) == 0 || parts[0] == "" {
			return "", clientReq("invalid gs:// path")
		}
		if parts[0] != bucket {
			return "", clientReq(fmt.Sprintf("path bucket %q does not match route bucket %q", parts[0], bucket))
		}
		if len(parts) == 1 {
			return "", clientReq("object path required after bucket name")
		}
		path = parts[1]
	}
	path = strings.TrimPrefix(path, "/")
	if path == "" {
		return "", clientReq("object path required")
	}
	if invalidObjectPath(path) {
		return "", clientReq("invalid object path")
	}
	return path, nil
}

func wildcardListPrefix(path string) string {
	firstWildcard := strings.IndexAny(path, "*?")
	if firstWildcard == -1 {
		return path
	}
	prefix := path[:firstWildcard]
	lastSlash := strings.LastIndex(prefix, "/")
	if lastSlash == -1 {
		return ""
	}
	return prefix[:lastSlash+1]
}

func wildcardRegexp(path string) (*regexp.Regexp, error) {
	var b strings.Builder
	b.WriteString("^")
	for _, r := range path {
		switch r {
		case '*':
			b.WriteString(".*")
		case '?':
			b.WriteString(".")
		default:
			b.WriteString(regexp.QuoteMeta(string(r)))
		}
	}
	b.WriteString("$")
	return regexp.Compile(b.String())
}

func normalizeFilterType(v string) string {
	switch strings.TrimSpace(strings.ToLower(v)) {
	case "", "specific metadata":
		return "specific"
	case "any metadata":
		return "any"
	case "no metadata":
		return "none"
	default:
		return "specific"
	}
}

func parseMetadataExpectations(filters []string) []metadataExpectation {
	expectations := make([]metadataExpectation, 0, len(filters))
	for _, raw := range filters {
		for _, part := range strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == ' ' || r == ';' }) {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			key, value, hasValue := strings.Cut(part, "=")
			key = strings.TrimSpace(key)
			value = strings.TrimSpace(value)
			if key == "" {
				continue
			}
			expectations = append(expectations, metadataExpectation{
				Key:      key,
				Value:    value,
				HasValue: hasValue,
			})
		}
	}
	return expectations
}

func matchesFilterType(metadata map[string]string, filterType string, expectations []metadataExpectation) bool {
	switch filterType {
	case "any":
		return len(metadata) > 0
	case "none":
		return len(metadata) == 0
	default:
		if len(expectations) == 0 {
			return false
		}
		for _, exp := range expectations {
			actual, ok := metadata[exp.Key]
			if !ok {
				return false
			}
			if exp.HasValue && actual != exp.Value {
				return false
			}
		}
		return true
	}
}

func matchedMetadata(metadata map[string]string, expectations []metadataExpectation) map[string]string {
	if len(expectations) == 0 {
		return nil
	}
	out := map[string]string{}
	for _, exp := range expectations {
		if actual, ok := metadata[exp.Key]; ok {
			out[exp.Key] = actual
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func cloneMetadata(metadata map[string]string) map[string]string {
	if len(metadata) == 0 {
		return nil
	}
	out := make(map[string]string, len(metadata))
	for k, v := range metadata {
		out[k] = v
	}
	return out
}
