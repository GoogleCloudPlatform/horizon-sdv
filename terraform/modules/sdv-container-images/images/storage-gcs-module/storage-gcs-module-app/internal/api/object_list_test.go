// Copyright (c) 2026 Accenture, All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/storage"
	"golang.org/x/oauth2"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
)

func TestValidateObjectsListPrefix(t *testing.T) {
	t.Parallel()
	ok := []string{"", "builds", "builds/", "builds/image.tar"}
	for _, prefix := range ok {
		if err := validateObjectsListPrefix(prefix); err != nil {
			t.Fatalf("prefix %q: %v", prefix, err)
		}
	}
	bad := []string{"/abs", "a/../b", ".."}
	for _, prefix := range bad {
		if err := validateObjectsListPrefix(prefix); err == nil {
			t.Fatalf("prefix %q: expected error", prefix)
		}
	}
}

func TestParseObjectsListPageSize(t *testing.T) {
	t.Parallel()
	n, err := parseObjectsListPageSize("")
	if err != nil || n != objectsListDefaultPageSize {
		t.Fatalf("default: n=%d err=%v", n, err)
	}
	n, err = parseObjectsListPageSize(" 10 ")
	if err != nil || n != 10 {
		t.Fatalf("10: n=%d err=%v", n, err)
	}
	if _, err := parseObjectsListPageSize("1000"); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{"0", "-1", "1001", "abc"} {
		if _, err := parseObjectsListPageSize(raw); err == nil {
			t.Fatalf("pageSize %q: expected error", raw)
		}
	}
}

func TestWriteListObjectsErr(t *testing.T) {
	t.Parallel()
	missing := fmt.Errorf("%w: %w", storage.ErrBucketNotExist, &googleapi.Error{Code: http.StatusNotFound, Message: "No such bucket"})
	cases := []struct {
		name   string
		err    error
		status int
		detail string
	}{
		{"bucket missing", missing, http.StatusNotFound, "bucket not found"},
		{"bare not found", &googleapi.Error{Code: http.StatusNotFound}, http.StatusNotFound, "bucket not found"},
		{"forbidden", &googleapi.Error{Code: http.StatusForbidden}, http.StatusForbidden, "bucket not accessible"},
		{"bad page token", &googleapi.Error{Code: http.StatusBadRequest}, http.StatusBadRequest, "invalid pageToken"},
		{"backend failure", errors.New("dial tcp: connection refused"), http.StatusInternalServerError, "storage operation failed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rr := httptest.NewRecorder()
			writeListObjectsErr(rr, tc.err)
			if rr.Code != tc.status {
				t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
			}
			var body struct {
				Detail string `json:"detail"`
			}
			if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body.Detail != tc.detail {
				t.Fatalf("detail %q", body.Detail)
			}
		})
	}
}

func TestHandleListObjectsGCSStatuses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		code := http.StatusInternalServerError
		msg := "unexpected"
		switch {
		case strings.Contains(r.URL.Path, "/b/missing/"):
			code, msg = http.StatusNotFound, "The specified bucket does not exist."
		case strings.Contains(r.URL.Path, "/b/forbidden/"):
			code, msg = http.StatusForbidden, "Permission denied."
		case strings.Contains(r.URL.Path, "/b/badtoken/"):
			code, msg = http.StatusBadRequest, "Invalid page token."
		default:
			t.Errorf("path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		fmt.Fprintf(w, `{"error":{"code":%d,"message":%q}}`, code, msg)
	}))
	t.Cleanup(srv.Close)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)
	hc := &http.Client{Transport: &oauth2.Transport{
		Source: oauth2.StaticTokenSource(&oauth2.Token{
			AccessToken: "test-token",
			TokenType:   "Bearer",
			Expiry:      time.Now().Add(time.Hour),
		}),
	}}
	client, err := storage.NewClient(ctx, option.WithHTTPClient(hc), option.WithEndpoint(srv.URL))
	if err != nil {
		t.Fatalf("storage client: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })

	s := &Server{
		GCS: client,
		StaticAllowedBuckets: map[string]struct{}{
			"missing":   {},
			"forbidden": {},
			"badtoken":  {},
		},
	}
	cases := []struct {
		bucket string
		query  string
		status int
		detail string
	}{
		{"missing", "", http.StatusNotFound, "bucket not found"},
		{"forbidden", "", http.StatusForbidden, "bucket not accessible"},
		{"badtoken", "pageToken=not-a-token", http.StatusBadRequest, "invalid pageToken"},
	}
	for _, tc := range cases {
		t.Run(tc.bucket, func(t *testing.T) {
			target := "/api/v1/buckets/" + tc.bucket + "/objects-list"
			if tc.query != "" {
				target += "?" + tc.query
			}
			req := httptest.NewRequest(http.MethodGet, target, nil).WithContext(ctx)
			req.SetPathValue("bucket", tc.bucket)
			rr := httptest.NewRecorder()
			s.handleListObjects(rr, req)
			if rr.Code != tc.status {
				t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
			}
			var body struct {
				Detail string `json:"detail"`
			}
			if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body.Detail != tc.detail {
				t.Fatalf("detail %q", body.Detail)
			}
		})
	}
}
