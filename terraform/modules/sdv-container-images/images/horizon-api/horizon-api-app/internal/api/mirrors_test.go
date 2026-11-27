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
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/acn-horizon-sdv/horizon-api/internal/auth"
	"github.com/acn-horizon-sdv/horizon-api/internal/catalog"
)

func routeFor(t *testing.T, method, pattern string) mirrorRoute {
	t.Helper()
	for _, rt := range mirrorRoutes() {
		if rt.method == method && rt.pattern == pattern {
			return rt
		}
	}
	t.Fatalf("no route %s %s", method, pattern)
	return mirrorRoute{}
}

func mirrorServer(t *testing.T, upstream http.Handler, enabled bool, role string) *Server {
	t.Helper()
	srv := httptest.NewServer(upstream)
	t.Cleanup(srv.Close)
	tok := filepath.Join(t.TempDir(), "horizon-api")
	if err := os.WriteFile(tok, []byte("module-token\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return &Server{opt: Options{
		MirrorModuleURL:       srv.URL,
		MirrorModuleTokenFile: tok,
		MirrorAdminRole:       role,
		MirrorModuleEnabled:   func(context.Context) (bool, error) { return enabled, nil },
	}}
}

func TestMirrorProxyForwards(t *testing.T) {
	t.Parallel()
	var gotPath, gotQuery, gotAuth, gotUser, gotFrom, gotBody string
	s := mirrorServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery = r.URL.Path, r.URL.RawQuery
		gotAuth, gotUser, gotFrom = r.Header.Get("Authorization"), r.Header.Get("X-Horizon-User"), r.Header.Get("X-Horizon-Submitted-From")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte(`{"requestId":"r1"}`))
	}), true, "")
	req := httptest.NewRequest("DELETE", "/v1/storage-mirror/mirrors/aosp?purge=true&confirm=aosp", strings.NewReader(`{}`))
	req.SetPathValue("name", "aosp")
	req.Header.Set("X-Horizon-Submitted-From", "horizon-cli")
	rec := httptest.NewRecorder()
	s.handleMirrorProxy(rec, req, &auth.Principal{Subject: "jane"}, routeFor(t, "DELETE", "/v1/storage-mirror/mirrors/{name}"))
	if rec.Code != http.StatusAccepted || !strings.Contains(rec.Body.String(), "r1") {
		t.Fatalf("response %d %s", rec.Code, rec.Body)
	}
	if gotPath != "/api/v1/mirrors/aosp" || gotQuery != "purge=true&confirm=aosp" || gotAuth != "Bearer module-token" || gotUser != "jane" || gotFrom != "horizon-cli" || gotBody != "{}" {
		t.Fatalf("upstream %q %q %q %q %q %q", gotPath, gotQuery, gotAuth, gotUser, gotFrom, gotBody)
	}
}

func TestMirrorProxyGatesAndRoles(t *testing.T) {
	t.Parallel()
	called := false
	up := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })
	get := routeFor(t, "GET", "/v1/storage-mirror/mirrors")
	post := routeFor(t, "POST", "/v1/storage-mirror/mirrors")

	disabled := mirrorServer(t, up, false, "")
	rec := httptest.NewRecorder()
	disabled.handleMirrorProxy(rec, httptest.NewRequest("GET", "/v1/storage-mirror/mirrors", nil), &auth.Principal{}, get)
	if rec.Code != http.StatusNotFound || called {
		t.Fatalf("disabled module: %d called=%v", rec.Code, called)
	}

	gated := mirrorServer(t, up, true, "horizon-storage-mirror-admin")
	rec = httptest.NewRecorder()
	gated.handleMirrorProxy(rec, httptest.NewRequest("POST", "/v1/storage-mirror/mirrors", strings.NewReader("{}")), &auth.Principal{Subject: "u"}, post)
	if rec.Code != http.StatusForbidden || called {
		t.Fatalf("missing role: %d", rec.Code)
	}
	rec = httptest.NewRecorder()
	gated.handleMirrorProxy(rec, httptest.NewRequest("GET", "/v1/storage-mirror/mirrors", nil), &auth.Principal{Subject: "u"}, get)
	if rec.Code != http.StatusOK || !called {
		t.Fatalf("reads need no role: %d", rec.Code)
	}
	called = false
	rec = httptest.NewRecorder()
	gated.handleMirrorProxy(rec, httptest.NewRequest("POST", "/v1/storage-mirror/mirrors", strings.NewReader("{}")), &auth.Principal{Subject: "u", Roles: []string{"horizon-storage-mirror-admin"}}, post)
	if rec.Code != http.StatusOK || !called {
		t.Fatalf("admin: %d", rec.Code)
	}

	bad := httptest.NewRequest("GET", "/v1/storage-mirror/mirrors/x", nil)
	bad.SetPathValue("name", "Bad_Name")
	rec = httptest.NewRecorder()
	gated.handleMirrorProxy(rec, bad, &auth.Principal{}, routeFor(t, "GET", "/v1/storage-mirror/mirrors/{name}"))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("name validation: %d", rec.Code)
	}
}

func TestMirrorProxyUpstreamUnauthorized(t *testing.T) {
	t.Parallel()
	s := mirrorServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusUnauthorized) }), true, "")
	rec := httptest.NewRecorder()
	s.handleMirrorProxy(rec, httptest.NewRequest("GET", "/v1/storage-mirror/volume", nil), &auth.Principal{}, routeFor(t, "GET", "/v1/storage-mirror/volume"))
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("got %d", rec.Code)
	}
}

func TestOpenAPIDocumentsMirrorPaths(t *testing.T) {
	t.Parallel()
	b, err := catalog.OpenAPISpec(nil, catalog.RetentionOpenAPI{})
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Paths map[string]interface{} `json:"paths"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	for _, rt := range mirrorRoutes() {
		if _, ok := doc.Paths[rt.pattern]; !ok {
			t.Errorf("OpenAPI misses %s", rt.pattern)
		}
	}
}
