// Copyright (c) 2026 Accenture, All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//         http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	iamcredentials "google.golang.org/api/iamcredentials/v1"
)

// GCS V4 signed URL maximum lifetime (seconds).
const signedURLMaxSecondsGCS = 604800 // 7 days

type signedURLRequest struct {
	ObjectPath       string `json:"objectPath"`
	Method           string `json:"method"`
	ExpiresInSeconds *int64 `json:"expiresInSeconds,omitempty"`
	ContentType      string `json:"contentType,omitempty"`
}

func (s *Server) handleSignedURL(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if s.SigningServiceAccountEmail == "" {
		writeErr(w, http.StatusServiceUnavailable, "signed URLs disabled: set GCS_SIGNED_URL_SERVICE_ACCOUNT or GOOGLE_CLOUD_PROJECT")
		return
	}

	bucket := r.PathValue("bucket")
	if !s.requireBucket(w, r, bucket) {
		return
	}

	var body signedURLRequest
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

	method := strings.ToUpper(strings.TrimSpace(body.Method))
	if method == "" {
		method = http.MethodGet
	}
	if method != http.MethodGet && method != http.MethodPut {
		writeErr(w, http.StatusBadRequest, "method must be GET or PUT")
		return
	}

	defaultSecs := int64(s.SignedURLDefaultExpiry / time.Second)
	maxSecs := int64(s.SignedURLMaxExpiry / time.Second)
	if maxSecs > signedURLMaxSecondsGCS {
		maxSecs = signedURLMaxSecondsGCS
	}
	if defaultSecs < 1 {
		defaultSecs = 48 * 3600
	}
	if maxSecs < 1 {
		maxSecs = signedURLMaxSecondsGCS
	}
	if maxSecs < defaultSecs {
		maxSecs = defaultSecs
	}

	var ttl int64
	if body.ExpiresInSeconds != nil && *body.ExpiresInSeconds > 0 {
		ttl = *body.ExpiresInSeconds
	} else {
		ttl = defaultSecs
	}
	if ttl > maxSecs {
		writeErr(w, http.StatusBadRequest, fmt.Sprintf("expiresInSeconds exceeds max (%d)", maxSecs))
		return
	}
	if ttl > signedURLMaxSecondsGCS {
		writeErr(w, http.StatusBadRequest, fmt.Sprintf("expiresInSeconds exceeds GCS V4 limit (%d)", signedURLMaxSecondsGCS))
		return
	}

	expires := time.Now().Add(time.Duration(ttl) * time.Second)

	ctx := r.Context()
	signer, err := newIAMSignBytesFunc(ctx, s.SigningServiceAccountEmail)
	if err != nil {
		writeOpErr(w, err)
		return
	}

	opts := &storage.SignedURLOptions{
		Scheme:         storage.SigningSchemeV4,
		Method:         method,
		Expires:        expires,
		GoogleAccessID: s.SigningServiceAccountEmail,
		SignBytes:      signer,
	}
	if method == http.MethodPut && strings.TrimSpace(body.ContentType) != "" {
		opts.Headers = []string{fmt.Sprintf("Content-Type:%s", body.ContentType)}
	}

	u, err := storage.SignedURL(bucket, body.ObjectPath, opts)
	if err != nil {
		writeOpErr(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"url":        u,
		"bucket":     bucket,
		"objectPath": body.ObjectPath,
		"method":     method,
		"expiresAt":  expires.UTC().Format(time.RFC3339),
		"expiresInSeconds": ttl,
	})
}

func newIAMSignBytesFunc(ctx context.Context, serviceAccountEmail string) (func([]byte) ([]byte, error), error) {
	svc, err := iamcredentials.NewService(ctx)
	if err != nil {
		return nil, fmt.Errorf("iamcredentials client: %w", err)
	}
	name := fmt.Sprintf("projects/-/serviceAccounts/%s", serviceAccountEmail)
	return func(b []byte) ([]byte, error) {
		req := &iamcredentials.SignBlobRequest{
			Payload: base64.StdEncoding.EncodeToString(b),
		}
		resp, err := svc.Projects.ServiceAccounts.SignBlob(name, req).Context(ctx).Do()
		if err != nil {
			return nil, err
		}
		return base64.StdEncoding.DecodeString(resp.SignedBlob)
	}, nil
}
