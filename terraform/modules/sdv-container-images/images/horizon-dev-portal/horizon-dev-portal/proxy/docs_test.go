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

// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func docsFixture() fstest.MapFS {
	return fstest.MapFS{
		"bundled-docs/manifest.json":                   {Data: []byte(`{"files":["docs/guide.md"]}`)},
		"bundled-docs/docs/guide.md":                   {Data: []byte("# Guide\n")},
		"bundled-docs/docs/notes.txt":                  {Data: []byte("not served")},
		"docs-images/shot.png":                         {Data: []byte("png")},
		"docs-images/diagram.svg":                      {Data: []byte("<svg/>")},
		"docs-images/architecture/images/overview.jpg": {Data: []byte("jpg")},
		"docs-images/readme.txt":                       {Data: []byte("not served")},
		"index.html":                                   {Data: []byte("<html></html>")},
	}
}

func docsGet(t *testing.T, mux *http.ServeMux, target, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, target, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func TestDocsRoutes(t *testing.T) {
	v := &stubVerifier{}
	mux := http.NewServeMux()
	registerDocsRoutes(mux, docsFixture(), v)

	t.Run("requires token", func(t *testing.T) {
		if rec := docsGet(t, mux, "/api/docs/v1/files/manifest.json", ""); rec.Code != http.StatusUnauthorized {
			t.Fatalf("missing token status %d", rec.Code)
		}
		v.err = errors.New("bad signature")
		defer func() { v.err = nil }()
		if rec := docsGet(t, mux, "/api/docs/v1/images/shot.png", "nope"); rec.Code != http.StatusUnauthorized {
			t.Fatalf("bad token status %d", rec.Code)
		}
	})

	t.Run("serves files", func(t *testing.T) {
		cases := []struct {
			target, body, contentType string
		}{
			{"/api/docs/v1/files/manifest.json", `{"files":["docs/guide.md"]}`, "application/json; charset=utf-8"},
			{"/api/docs/v1/files/docs/guide.md", "# Guide\n", "text/markdown; charset=utf-8"},
			{"/api/docs/v1/images/shot.png", "png", "image/png"},
			{"/api/docs/v1/images/diagram.svg", "<svg/>", "image/svg+xml"},
			{"/api/docs/v1/images/architecture/images/overview.jpg", "jpg", "image/jpeg"},
		}
		for _, c := range cases {
			rec := docsGet(t, mux, c.target, "good")
			if rec.Code != http.StatusOK || rec.Body.String() != c.body {
				t.Fatalf("%s status %d body %q", c.target, rec.Code, rec.Body.String())
			}
			if rec.Header().Get("Content-Type") != c.contentType || rec.Header().Get("X-Content-Type-Options") != "nosniff" {
				t.Fatalf("%s headers %#v", c.target, rec.Header())
			}
		}
		if csp := docsGet(t, mux, "/api/docs/v1/images/diagram.svg", "good").Header().Get("Content-Security-Policy"); csp == "" {
			t.Fatal("svg served without content security policy")
		}
	})

	t.Run("rejects other paths", func(t *testing.T) {
		for _, target := range []string{
			"/api/docs/v1/files/docs/notes.txt",
			"/api/docs/v1/files/docs/missing.md",
			"/api/docs/v1/files/docs/../manifest.json",
			"/api/docs/v1/files/docs/%2e%2e/manifest.json",
			"/api/docs/v1/files/docs//guide.md",
			"/api/docs/v1/files/./docs/guide.md",
			"/api/docs/v1/images/readme.txt",
			"/api/docs/v1/images/%2e%2e/index.html",
		} {
			// ServeMux redirects non-clean paths to their clean form, which is again behind requireBearer.
			rec := docsGet(t, mux, target, "good")
			switch rec.Code {
			case http.StatusNotFound:
			case http.StatusMovedPermanently, http.StatusTemporaryRedirect, http.StatusPermanentRedirect:
				if loc := rec.Header().Get("Location"); !strings.HasPrefix(loc, "/api/docs/v1/") || strings.Contains(loc, "..") {
					t.Fatalf("%s redirected to %q", target, loc)
				}
			default:
				t.Fatalf("%s status %d", target, rec.Code)
			}
		}
	})
}

func TestCleanDocsPath(t *testing.T) {
	for _, p := range []string{"", "/docs/a.md", "docs/../a.md", "..", "docs/./a.md", "docs//a.md", "docs\\a.md", "docs/"} {
		if _, ok := cleanDocsPath(p); ok {
			t.Fatalf("accepted %q", p)
		}
	}
	if got, ok := cleanDocsPath("docs/guides/README.md"); !ok || got != "docs/guides/README.md" {
		t.Fatalf("rejected valid path: %q %v", got, ok)
	}
}

func TestSpaHandlerHidesDocs(t *testing.T) {
	h := spaHandler("")
	for _, target := range []string{
		"/bundled-docs/manifest.json",
		"/bundled-docs/docs/deployment_guide.md",
		"/docs-images/shot.png",
		"/docs-images",
		"/assets/../bundled-docs/manifest.json",
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("%s status %d", target, rec.Code)
		}
	}
}
