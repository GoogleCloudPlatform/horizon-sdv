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

package mtk

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestKeyLifecycle(t *testing.T) {
	var deleted string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, _ := r.BasicAuth()
		if user != "mtk-connect-admin" || pass != "secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/mtk-connect/api/v1/users":
			_, _ = w.Write([]byte(`{"data":[{"id":"u1"}]}`))
		case r.Method == http.MethodGet && r.URL.Path == "/mtk-connect/api/v1/users/u1":
			_, _ = w.Write([]byte(`{"data":{"keys":[{"id":"k1","key":"abcdefgh","creationTime":"2026-01-01T00:00:00.000Z","expiryTime":"2026-02-01T00:00:00Z"}]}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/mtk-connect/api/v1/users/u1/keys":
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["expiryTime"] == "" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"data":{"id":"k9","key":"new-key-value"}}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/mtk-connect/api/v1/users/u1/keys/k1":
			deleted = "k1"
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	ctx := context.Background()
	c := New(srv.URL+"/mtk-connect/", "mtk-connect-admin", "secret", srv.Client())

	id, err := c.FindUserID(ctx, "mtk-connect-admin")
	if err != nil || id != "u1" {
		t.Fatalf("FindUserID = %q, %v", id, err)
	}
	keys, err := c.Keys(ctx, id)
	if err != nil || len(keys) != 1 || keys[0].ID != "k1" || keys[0].Created.IsZero() || keys[0].Expires.IsZero() {
		t.Fatalf("Keys = %+v, %v", keys, err)
	}
	key, keyID, err := c.CreateKey(ctx, id, time.Now().Add(time.Hour))
	if err != nil || key != "new-key-value" || keyID != "k9" {
		t.Fatalf("CreateKey = %q, %q, %v", key, keyID, err)
	}
	if err := c.DeleteKey(ctx, id, "k1"); err != nil || deleted != "k1" {
		t.Fatalf("DeleteKey: err=%v deleted=%q", err, deleted)
	}
}

func TestUnauthorizedIsDetected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()
	c := New(srv.URL, "u", "bad", srv.Client())
	_, err := c.FindUserID(context.Background(), "u")
	if !IsUnauthorized(err) {
		t.Fatalf("want unauthorized error, got %v", err)
	}
}

func TestServerErrorIsNotUnauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()
	c := New(srv.URL, "u", "k", srv.Client())
	_, err := c.FindUserID(context.Background(), "u")
	if err == nil || IsUnauthorized(err) {
		t.Fatalf("5xx must be an error but not unauthorized, got %v", err)
	}
}
