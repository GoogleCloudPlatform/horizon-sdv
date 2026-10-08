// Copyright (c) 2026 Accenture, All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

// Package api implements the internal HTTP API for GCS operations.
package api

import (
	"archive/zip"
	"bytes"
	"context"
	crand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/storage"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// Server holds dependencies for HTTP handlers.
type Server struct {
	K8s              client.Client
	GCS              *storage.Client
	ManagerNamespace string
	DefaultBucket    string
	ProjectID        string

	// APITokens maps Bearer secrets to caller names (current and optional previous for rotation).
	APITokens []APIToken
	// APIBearerToken is a legacy single-token fallback when APITokens is empty.
	APIBearerToken string
	// StaticAllowedBuckets are always permitted in addition to GCSBucket CR names.
	StaticAllowedBuckets map[string]struct{}

	// GCS V4 signed URLs (IAM signBlob). Empty SigningServiceAccountEmail disables the endpoint.
	SigningServiceAccountEmail string
	SignedURLDefaultExpiry     time.Duration
	SignedURLMaxExpiry         time.Duration
}

const resumableConfigMapName = "storage-gcs-module-resumable"

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"detail": msg})
}

// writeOpErr logs the backend error and returns a generic message (no GCS/IAM details).
func writeOpErr(w http.ResponseWriter, err error) {
	log.Printf("storage-gcs api: %v", err)
	writeErr(w, http.StatusInternalServerError, "storage operation failed")
}

func writeGatewayErr(w http.ResponseWriter, err error) {
	log.Printf("storage-gcs api: %v", err)
	writeErr(w, http.StatusBadGateway, "storage operation failed")
}

func (s *Server) requireObjectPath(w http.ResponseWriter, objectPath string) bool {
	if objectPath == "" || invalidObjectPath(objectPath) {
		writeErr(w, http.StatusBadRequest, "invalid object path")
		return false
	}
	return true
}

// Handler returns an http.Handler with all routes (Go 1.22+ ServeMux patterns).
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /ready", s.handleReady)
	mux.HandleFunc("POST /api/v1/buckets/{bucket}/objects", s.handleUpload)
	mux.HandleFunc("POST /api/v1/buckets/{bucket}/packages", s.handleUploadPackage)
	// Go 1.22+ ServeMux: `{name...}` wildcard must be the last segment (no trailing /metadata).
	mux.HandleFunc("GET /api/v1/buckets/{bucket}/objects/{object_path...}", s.handleDownload)
	mux.HandleFunc("GET /api/v1/buckets/{bucket}/objects-meta/{object_path...}", s.handleMetadata)
	mux.HandleFunc("GET /api/v1/buckets/{bucket}/objects-meta-raw/{object_path...}", s.handleMetadataRaw)
	mux.HandleFunc("PATCH /api/v1/buckets/{bucket}/objects-meta/{object_path...}", s.handleMetadataUpdate)
	mux.HandleFunc("POST /api/v1/buckets/{bucket}/objects/filter", s.handleFilterObjects)
	mux.HandleFunc("GET /api/v1/buckets/{bucket}/objects-list", s.handleListObjects)
	mux.HandleFunc("POST /api/v1/buckets/{bucket}/objects-meta/batch-update", s.handleBatchMetadataUpdate)
	mux.HandleFunc("POST /api/v1/buckets/{bucket}/objects-meta/batch-remove", s.handleBatchMetadataRemove)
	mux.HandleFunc("POST /api/v1/buckets/{bucket}/objects-storage-class/list", s.handleStorageClassList)
	mux.HandleFunc("POST /api/v1/buckets/{bucket}/objects-storage-class/batch-update", s.handleBatchStorageClassUpdate)
	mux.HandleFunc("POST /api/v1/buckets/{bucket}/objects/batch-delete", s.handleBatchDelete)
	mux.HandleFunc("DELETE /api/v1/buckets/{bucket}/objects/{object_path...}", s.handleDelete)

	// Resumable uploads (GCS JSON API backed). Upload state stored in ConfigMap.
	mux.HandleFunc("POST /api/v1/resumable/buckets/{bucket}/objects", s.handleResumableStart)
	mux.HandleFunc("PUT /api/v1/resumable/uploads/{upload_id}", s.handleResumablePut)
	mux.HandleFunc("GET /api/v1/resumable/uploads/{upload_id}", s.handleResumableStatus)

	mux.HandleFunc("POST /api/v1/buckets/{bucket}/signed-urls", s.handleSignedURL)
	return s.withAPIAuth(mux)
}

func (s *Server) handleReady(w http.ResponseWriter, _ *http.Request) {
	if s.DefaultBucket == "" {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	_, err := s.GCS.Bucket(s.DefaultBucket).Attrs(ctx)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			writeErr(w, http.StatusServiceUnavailable, "default bucket check timed out")
			return
		}
		writeErr(w, http.StatusServiceUnavailable, "default bucket not reachable")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")
	if !s.requireBucket(w, r, bucket) {
		return
	}
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeErr(w, http.StatusBadRequest, "multipart form required")
		return
	}
	objectPath := r.FormValue("object_path")
	if objectPath == "" {
		writeErr(w, http.StatusBadRequest, "object_path required")
		return
	}
	if !s.requireObjectPath(w, objectPath) {
		return
	}
	file, _, err := r.FormFile("file")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "file field required")
		return
	}
	defer file.Close()

	ct := r.FormValue("content_type")
	if ct == "" {
		ct = "application/octet-stream"
	}
	wobj := s.GCS.Bucket(bucket).Object(objectPath)
	wc := wobj.NewWriter(context.Background())
	wc.ContentType = ct
	// Make large uploads resumable/chunked at the GCS layer.
	// 0 => default behavior; we set a sensible default chunk.
	wc.ChunkSize = 32 << 20 // 32MiB

	n, err := io.Copy(wc, file)
	if err != nil {
		_ = wc.Close()
		writeOpErr(w, err)
		return
	}
	if err := wc.Close(); err != nil {
		writeOpErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"bucket":     bucket,
		"objectPath": objectPath,
		"size":       n,
		"gsUri":      fmt.Sprintf("gs://%s/%s", bucket, objectPath),
	})
}

// handleUploadPackage streams a ZIP created from multiple uploaded files directly into GCS.
// multipart fields:
// - object_path (required): destination object key in GCS
// - file (repeatable): files to include
func (s *Server) handleUploadPackage(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")
	if !s.requireBucket(w, r, bucket) {
		return
	}
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeErr(w, http.StatusBadRequest, "multipart form required")
		return
	}
	objectPath := r.FormValue("object_path")
	if objectPath == "" {
		writeErr(w, http.StatusBadRequest, "object_path required")
		return
	}
	if !s.requireObjectPath(w, objectPath) {
		return
	}
	form := r.MultipartForm
	files := form.File["file"]
	if len(files) == 0 {
		writeErr(w, http.StatusBadRequest, "at least one file field is required")
		return
	}

	wobj := s.GCS.Bucket(bucket).Object(objectPath)
	wc := wobj.NewWriter(context.Background())
	wc.ContentType = "application/zip"
	wc.ChunkSize = 32 << 20

	zw := zip.NewWriter(wc)
	var total int64
	for _, fh := range files {
		name := path.Base(fh.Filename)
		if name == "" || name == "." || name == ".." {
			_ = zw.Close()
			_ = wc.Close()
			writeErr(w, http.StatusBadRequest, "invalid zip entry name")
			return
		}
		f, err := fh.Open()
		if err != nil {
			_ = zw.Close()
			_ = wc.Close()
			writeOpErr(w, err)
			return
		}
		zf, err := zw.Create(name)
		if err != nil {
			_ = f.Close()
			_ = zw.Close()
			_ = wc.Close()
			writeOpErr(w, err)
			return
		}
		n, err := io.Copy(zf, f)
		_ = f.Close()
		if err != nil {
			_ = zw.Close()
			_ = wc.Close()
			writeOpErr(w, err)
			return
		}
		total += n
	}
	if err := zw.Close(); err != nil {
		_ = wc.Close()
		writeOpErr(w, err)
		return
	}
	if err := wc.Close(); err != nil {
		writeOpErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"bucket":     bucket,
		"objectPath": objectPath,
		"size":       total,
		"package":    "zip",
		"gsUri":      fmt.Sprintf("gs://%s/%s", bucket, objectPath),
	})
}

// handleDownload streams object bytes. Supports HTTP Range requests for chunked downloads.
func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")
	if !s.requireBucket(w, r, bucket) {
		return
	}
	objectPath := r.PathValue("object_path")
	if !s.requireObjectPath(w, objectPath) {
		return
	}
	obj := s.GCS.Bucket(bucket).Object(objectPath)

	// HEAD-like attrs to set content headers.
	attrs, err := obj.Attrs(r.Context())
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotExist) {
			writeErr(w, http.StatusNotFound, "object not found")
			return
		}
		writeOpErr(w, err)
		return
	}
	if attrs.ContentType != "" {
		w.Header().Set("Content-Type", attrs.ContentType)
	} else {
		w.Header().Set("Content-Type", "application/octet-stream")
	}
	w.Header().Set("Accept-Ranges", "bytes")

	// Range support: only handle single range.
	if rng := r.Header.Get("Range"); rng != "" && strings.HasPrefix(rng, "bytes=") {
		start, end, ok := parseSingleByteRange(rng, attrs.Size)
		if !ok {
			w.Header().Set("Content-Range", fmt.Sprintf("bytes */%d", attrs.Size))
			w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
			return
		}
		length := end - start + 1
		rc, err := obj.NewRangeReader(r.Context(), start, length)
		if err != nil {
			writeOpErr(w, err)
			return
		}
		defer rc.Close()
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, attrs.Size))
		w.Header().Set("Content-Length", strconv.FormatInt(length, 10))
		w.WriteHeader(http.StatusPartialContent)
		_, _ = io.Copy(w, rc)
		return
	}

	rc, err := obj.NewReader(r.Context())
	if err != nil {
		writeOpErr(w, err)
		return
	}
	defer rc.Close()
	w.Header().Set("Content-Length", strconv.FormatInt(attrs.Size, 10))
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, rc)
}

func (s *Server) handleMetadata(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")
	if !s.requireBucket(w, r, bucket) {
		return
	}
	objectPath := r.PathValue("object_path")
	if !s.requireObjectPath(w, objectPath) {
		return
	}
	attrs, err := s.GCS.Bucket(bucket).Object(objectPath).Attrs(context.Background())
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotExist) {
			writeErr(w, http.StatusNotFound, "object not found")
			return
		}
		writeOpErr(w, err)
		return
	}
	md5hex := ""
	if len(attrs.MD5) > 0 {
		md5hex = hex.EncodeToString(attrs.MD5)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"bucket":      bucket,
		"objectPath":  objectPath,
		"size":        attrs.Size,
		"contentType": attrs.ContentType,
		"md5Hash":     md5hex,
		"updated":     attrs.Updated.Format("2006-01-02T15:04:05Z07:00"),
		"gsUri":       fmt.Sprintf("gs://%s/%s", bucket, objectPath),
	})
}

// handleMetadataRaw returns raw object attrs including custom metadata map.
func (s *Server) handleMetadataRaw(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")
	if !s.requireBucket(w, r, bucket) {
		return
	}
	objectPath := r.PathValue("object_path")
	if !s.requireObjectPath(w, objectPath) {
		return
	}
	attrs, err := s.GCS.Bucket(bucket).Object(objectPath).Attrs(r.Context())
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotExist) {
			writeErr(w, http.StatusNotFound, "object not found")
			return
		}
		writeOpErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"bucket":       bucket,
		"objectPath":   objectPath,
		"size":         attrs.Size,
		"contentType":  attrs.ContentType,
		"cacheControl": attrs.CacheControl,
		"metadata":     attrs.Metadata,
		"updated":      attrs.Updated.Format(time.RFC3339),
		"gsUri":        fmt.Sprintf("gs://%s/%s", bucket, objectPath),
	})
}

type metadataUpdateBody struct {
	ContentType        *string           `json:"contentType,omitempty"`
	CacheControl       *string           `json:"cacheControl,omitempty"`
	SetMetadata        map[string]string `json:"setMetadata,omitempty"`
	DeleteMetadataKeys []string          `json:"deleteMetadataKeys,omitempty"`
}

// handleMetadataUpdate applies raw metadata updates (custom metadata and select headers).
func (s *Server) handleMetadataUpdate(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")
	if !s.requireBucket(w, r, bucket) {
		return
	}
	objectPath := r.PathValue("object_path")
	if !s.requireObjectPath(w, objectPath) {
		return
	}
	var body metadataUpdateBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	updates := storage.ObjectAttrsToUpdate{}
	if body.ContentType != nil {
		updates.ContentType = *body.ContentType
	}
	if body.CacheControl != nil {
		updates.CacheControl = *body.CacheControl
	}
	if len(body.SetMetadata) > 0 || len(body.DeleteMetadataKeys) > 0 {
		updates.Metadata = map[string]string{}
		for k, v := range body.SetMetadata {
			updates.Metadata[k] = v
		}
		// Deletion: set key to empty string per GCS API semantics.
		for _, k := range body.DeleteMetadataKeys {
			updates.Metadata[k] = ""
		}
	}
	attrs, err := s.GCS.Bucket(bucket).Object(objectPath).Update(r.Context(), updates)
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotExist) {
			writeErr(w, http.StatusNotFound, "object not found")
			return
		}
		writeOpErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"bucket":       bucket,
		"objectPath":   objectPath,
		"contentType":  attrs.ContentType,
		"cacheControl": attrs.CacheControl,
		"metadata":     attrs.Metadata,
		"updated":      attrs.Updated.Format(time.RFC3339),
		"gsUri":        fmt.Sprintf("gs://%s/%s", bucket, objectPath),
	})
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")
	if !s.requireBucket(w, r, bucket) {
		return
	}
	objectPath := r.PathValue("object_path")
	if !s.requireObjectPath(w, objectPath) {
		return
	}
	obj := s.GCS.Bucket(bucket).Object(objectPath)
	if _, err := obj.Attrs(context.Background()); err == nil {
		if err := obj.Delete(context.Background()); err != nil {
			writeOpErr(w, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"status": "deleted",
		"gsUri":  fmt.Sprintf("gs://%s/%s", bucket, objectPath),
	})
}

// --- Resumable upload helpers ---

type resumableStartBody struct {
	ObjectPath  string            `json:"objectPath"`
	ContentType string            `json:"contentType,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

func (s *Server) handleResumableStart(w http.ResponseWriter, r *http.Request) {
	bucket := r.PathValue("bucket")
	if !s.requireBucket(w, r, bucket) {
		return
	}
	var body resumableStartBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if body.ObjectPath == "" {
		writeErr(w, http.StatusBadRequest, "objectPath required")
		return
	}
	if !s.requireObjectPath(w, body.ObjectPath) {
		return
	}
	ct := body.ContentType
	if ct == "" {
		ct = "application/octet-stream"
	}

	// Start a resumable session via GCS JSON API (manual HTTP so we can capture Location).
	ctx := r.Context()
	hc, err := gcsAuthedClient(ctx)
	if err != nil {
		writeOpErr(w, err)
		return
	}

	startURL := fmt.Sprintf(
		"https://storage.googleapis.com/upload/storage/v1/b/%s/o?uploadType=resumable&name=%s",
		url.PathEscape(bucket),
		url.QueryEscape(body.ObjectPath),
	)
	meta := map[string]any{
		"name":        body.ObjectPath,
		"contentType": ct,
	}
	if len(body.Metadata) > 0 {
		meta["metadata"] = body.Metadata
	}
	metaBytes, _ := json.Marshal(meta)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, startURL, bytes.NewReader(metaBytes))
	if err != nil {
		writeOpErr(w, err)
		return
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("X-Upload-Content-Type", ct)

	resp, err := hc.Do(req)
	if err != nil {
		writeOpErr(w, err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		writeGatewayErr(w, fmt.Errorf("gcs resumable start: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(b))))
		return
	}
	sessionURL := resp.Header.Get("Location")
	if sessionURL == "" {
		writeErr(w, http.StatusBadGateway, "gcs resumable start missing Location header")
		return
	}

	uploadID, err := newUploadID()
	if err != nil {
		writeOpErr(w, err)
		return
	}
	if err := s.storeResumableSession(ctx, uploadID, sessionURL); err != nil {
		writeOpErr(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"uploadId":    uploadID,
		"bucket":      bucket,
		"objectPath":  body.ObjectPath,
		"contentType": ct,
	})
}

func (s *Server) handleResumablePut(w http.ResponseWriter, r *http.Request) {
	uploadID := r.PathValue("upload_id")
	ctx := r.Context()
	sessionURL, err := s.loadResumableSession(ctx, uploadID)
	if err != nil {
		writeErr(w, http.StatusNotFound, "upload session not found")
		return
	}
	hc, err := gcsAuthedClient(ctx)
	if err != nil {
		writeOpErr(w, err)
		return
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, sessionURL, r.Body)
	if err != nil {
		writeOpErr(w, err)
		return
	}
	// Forward required resumable headers.
	if cr := r.Header.Get("Content-Range"); cr != "" {
		req.Header.Set("Content-Range", cr)
	}
	if ct := r.Header.Get("Content-Type"); ct != "" {
		req.Header.Set("Content-Type", ct)
	}
	if cl := r.Header.Get("Content-Length"); cl != "" {
		req.Header.Set("Content-Length", cl)
	}

	resp, err := hc.Do(req)
	if err != nil {
		writeGatewayErr(w, err)
		return
	}
	defer resp.Body.Close()

	// GCS resumable protocol:
	// - 308 Resume Incomplete: upload continues (Range header present)
	// - 200/201: done (JSON object metadata body)
	if resp.StatusCode == 308 {
		writeJSON(w, http.StatusOK, map[string]any{
			"status": "incomplete",
			"range":  resp.Header.Get("Range"),
		})
		return
	}
	if resp.StatusCode == 200 || resp.StatusCode == 201 {
		// Done. Remove session mapping.
		_ = s.deleteResumableSession(ctx, uploadID)
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(b)
		return
	}

	b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	writeGatewayErr(w, fmt.Errorf("gcs resumable put: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(b))))
}

func (s *Server) handleResumableStatus(w http.ResponseWriter, r *http.Request) {
	uploadID := r.PathValue("upload_id")
	ctx := r.Context()
	sessionURL, err := s.loadResumableSession(ctx, uploadID)
	if err != nil {
		writeErr(w, http.StatusNotFound, "upload session not found")
		return
	}
	hc, err := gcsAuthedClient(ctx)
	if err != nil {
		writeOpErr(w, err)
		return
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, sessionURL, http.NoBody)
	if err != nil {
		writeOpErr(w, err)
		return
	}
	// Query status: bytes */*
	req.Header.Set("Content-Range", "bytes */*")
	req.Header.Set("Content-Length", "0")

	resp, err := hc.Do(req)
	if err != nil {
		writeGatewayErr(w, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == 308 {
		writeJSON(w, http.StatusOK, map[string]any{
			"status": "incomplete",
			"range":  resp.Header.Get("Range"),
		})
		return
	}
	if resp.StatusCode == 200 || resp.StatusCode == 201 {
		_ = s.deleteResumableSession(ctx, uploadID)
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(b)
		return
	}
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	writeGatewayErr(w, fmt.Errorf("gcs resumable status: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(b))))
}

func gcsAuthedClient(ctx context.Context) (*http.Client, error) {
	ts, err := google.DefaultTokenSource(ctx, "https://www.googleapis.com/auth/devstorage.read_write")
	if err != nil {
		return nil, err
	}
	return oauth2.NewClient(ctx, ts), nil
}

func (s *Server) storeResumableSession(ctx context.Context, uploadID string, sessionURL string) error {
	var cm corev1.ConfigMap
	key := types.NamespacedName{Namespace: s.ManagerNamespace, Name: resumableConfigMapName}
	err := s.K8s.Get(ctx, key, &cm)
	if apierrors.IsNotFound(err) {
		cm = corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      resumableConfigMapName,
				Namespace: s.ManagerNamespace,
			},
			Data: map[string]string{},
		}
		cm.Data[uploadID] = sessionURL
		return s.K8s.Create(ctx, &cm)
	}
	if err != nil {
		return err
	}
	if cm.Data == nil {
		cm.Data = map[string]string{}
	}
	cm.Data[uploadID] = sessionURL
	return s.K8s.Update(ctx, &cm)
}

func (s *Server) loadResumableSession(ctx context.Context, uploadID string) (string, error) {
	var cm corev1.ConfigMap
	key := types.NamespacedName{Namespace: s.ManagerNamespace, Name: resumableConfigMapName}
	if err := s.K8s.Get(ctx, key, &cm); err != nil {
		return "", err
	}
	u := ""
	if cm.Data != nil {
		u = cm.Data[uploadID]
	}
	if u == "" {
		return "", errors.New("not found")
	}
	return u, nil
}

func (s *Server) deleteResumableSession(ctx context.Context, uploadID string) error {
	var cm corev1.ConfigMap
	key := types.NamespacedName{Namespace: s.ManagerNamespace, Name: resumableConfigMapName}
	if err := s.K8s.Get(ctx, key, &cm); err != nil {
		return err
	}
	if cm.Data == nil {
		return nil
	}
	delete(cm.Data, uploadID)
	return s.K8s.Update(ctx, &cm)
}

// newUploadID returns an unguessable resumable-session id (16 CSPRNG bytes, hex).
func newUploadID() (string, error) {
	var b [16]byte
	if _, err := crand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate upload id: %w", err)
	}
	return hex.EncodeToString(b[:]), nil
}

// parseSingleByteRange parses a single bytes=START-END range.
func parseSingleByteRange(h string, size int64) (int64, int64, bool) {
	// bytes=START-END
	spec := strings.TrimPrefix(h, "bytes=")
	if strings.Contains(spec, ",") {
		return 0, 0, false
	}
	parts := strings.Split(spec, "-")
	if len(parts) != 2 {
		return 0, 0, false
	}
	if parts[0] == "" {
		// suffix: -N
		n, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil || n <= 0 {
			return 0, 0, false
		}
		if n > size {
			n = size
		}
		return size - n, size - 1, true
	}
	start, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || start < 0 {
		return 0, 0, false
	}
	end := int64(math.MaxInt64)
	if parts[1] != "" {
		end, err = strconv.ParseInt(parts[1], 10, 64)
		if err != nil || end < start {
			return 0, 0, false
		}
	}
	if start >= size {
		return 0, 0, false
	}
	if end >= size {
		end = size - 1
	}
	return start, end, true
}
