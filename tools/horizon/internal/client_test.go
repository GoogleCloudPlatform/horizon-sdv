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

package horizon

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func testClient(t *testing.T, h http.Handler) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	c := &Client{
		cfg:        &Config{BaseURL: srv.URL, KeycloakBase: srv.URL, KeycloakRealm: "horizon"},
		httpClient: srv.Client(),
		bearer:     "access",
		envToken:   true,
	}
	return c, srv
}

func TestTruncate(t *testing.T) {
	if got := truncate("a\nb", 10); got != "a b" {
		t.Fatalf("got %q", got)
	}
	got := truncate("abcdef", 3)
	if got != "abc…" {
		t.Fatalf("got %q", got)
	}
}

func TestDoJSONSuccessAndError(t *testing.T) {
	var gotAuth, gotType, gotBody string
	c, _ := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotType = r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		if r.URL.Path == "/fail" {
			http.Error(w, "nope", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	body, err := c.DoJSON(context.Background(), http.MethodPost, "/v1/x", []byte(`{"a":1}`))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != `{"ok":true}` || gotAuth != "Bearer access" || gotType != "application/json" || gotBody != `{"a":1}` {
		t.Fatalf("body=%s auth=%s type=%s req=%s", body, gotAuth, gotType, gotBody)
	}
	if _, err := c.DoJSON(context.Background(), http.MethodGet, "/fail", nil); err == nil || !strings.Contains(err.Error(), "HTTP 400") {
		t.Fatalf("err = %v", err)
	}
}

func TestDoJSONEnvTokenDoesNotRefresh(t *testing.T) {
	var hits atomic.Int32
	c, _ := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	t.Setenv("KEYCLOAK_CLIENT_SECRET", "")
	if _, err := c.DoJSON(context.Background(), http.MethodGet, "/v1/x", nil); err == nil || !strings.Contains(err.Error(), "HTTP 401") {
		t.Fatalf("err = %v", err)
	}
	if hits.Load() != 1 {
		t.Fatalf("hits = %d", hits.Load())
	}
}

func TestDoJSONRefreshesOnce(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("KEYCLOAK_CLIENT_SECRET", "")
	t.Setenv("HORIZON_ACCESS_TOKEN", "")

	var hits atomic.Int32
	c, srv := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/protocol/openid-connect/token") {
			b, _ := io.ReadAll(r.Body)
			if !strings.Contains(string(b), "refresh_token=old") {
				t.Errorf("refresh body = %s", b)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "fresh", "refresh_token": "newer", "expires_in": 30})
			return
		}
		n := hits.Add(1)
		if r.Header.Get("Authorization") == "Bearer access" && n == 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if r.Header.Get("Authorization") != "Bearer fresh" {
			t.Errorf("auth = %q", r.Header.Get("Authorization"))
		}
		if r.Header.Get("X-Horizon-Submitted-From") != "horizon-cli" {
			t.Errorf("submitted-from = %q", r.Header.Get("X-Horizon-Submitted-From"))
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	c.envToken = false
	c.tokenCache = &TokenCache{AccessToken: "access", RefreshToken: "old", KeycloakBase: srv.URL, Realm: "horizon", ClientID: "cli"}

	body, err := c.DoJSONWithHeaders(context.Background(), http.MethodPost, "/v1/submit", []byte(`{}`), map[string]string{
		"X-Horizon-Submitted-From": "horizon-cli",
		" ":                        "ignored",
	})
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != `{"ok":true}` || hits.Load() != 2 {
		t.Fatalf("body=%s hits=%d", body, hits.Load())
	}
	if c.bearer != "fresh" {
		t.Fatalf("bearer = %q", c.bearer)
	}
}

func TestGetJSONRawAndPlainURL(t *testing.T) {
	c, srv := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" && r.URL.Path == "/plain" {
			t.Errorf("plain GET carried auth %q", r.Header.Get("Authorization"))
		}
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"candidates":[{"nodeId":"n1"}]}`))
	}))
	body, code, err := c.GetJSONRaw(context.Background(), "/v1/download")
	if err != nil || code != http.StatusConflict || !bytes.Contains(body, []byte("n1")) {
		t.Fatalf("raw = %d %s %v", code, body, err)
	}

	resp, err := GetURLPlain(context.Background(), srv.URL+"/plain")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("status = %d", resp.StatusCode)
	}
}

func TestNewClientFromEnvToken(t *testing.T) {
	t.Setenv("HORIZON_ACCESS_TOKEN", " env-token ")
	c, err := NewClient(&Config{BaseURL: "https://api.example"})
	if err != nil {
		t.Fatal(err)
	}
	if c.bearer != "env-token" || !c.envToken {
		t.Fatalf("client = %+v", c)
	}
}

func TestNewClientMissingCredentials(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("HORIZON_ACCESS_TOKEN", "")
	t.Setenv("KEYCLOAK_CLIENT_SECRET", "")
	if _, err := NewClient(&Config{BaseURL: "https://api.example"}); err == nil {
		t.Fatal("expected auth error")
	}
}

func TestListArtifacts(t *testing.T) {
	var gotPath string
	c, _ := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.RequestURI()
		switch {
		case r.URL.Path == "/v1/artifacts/buckets":
			_, _ = w.Write([]byte(`{"buckets":[{"name":"logs"}]}`))
		default:
			_, _ = w.Write([]byte(`{"objects":[{"name":"a.bin","size":12}],"nextPageToken":"next"}`))
		}
	}))
	buckets, err := c.ListBuckets(context.Background())
	if err != nil || len(buckets.Buckets) != 1 || buckets.Buckets[0].Name != "logs" {
		t.Fatalf("buckets = %+v %v", buckets, err)
	}
	objects, err := c.ListObjects(context.Background(), "my bucket", "pre fix", "tok", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(objects.Objects) != 1 || objects.Objects[0].Size != 12 || objects.NextPageToken != "next" {
		t.Fatalf("objects = %+v", objects)
	}
	if !strings.Contains(gotPath, "/v1/artifacts/buckets/my%20bucket/objects?") || !strings.Contains(gotPath, "prefix=pre+fix") || !strings.Contains(gotPath, "pageToken=tok") || !strings.Contains(gotPath, "limit=5") {
		t.Fatalf("path = %q", gotPath)
	}
}

func TestStreamGET(t *testing.T) {
	c, srv := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") != "application/x-ndjson" {
			t.Errorf("accept = %q", r.Header.Get("Accept"))
		}
		_, _ = w.Write([]byte("line\n"))
	}))
	resp, err := c.StreamGET(context.Background(), srv.URL+"/v1/logs")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if string(b) != "line\n" {
		t.Fatalf("body = %q", b)
	}

	fail, _ := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusBadGateway)
	}))
	if _, err := fail.StreamGET(context.Background(), fail.cfg.BaseURL+"/v1/logs"); err == nil || !strings.Contains(err.Error(), "HTTP 502") {
		t.Fatalf("err = %v", err)
	}
}
