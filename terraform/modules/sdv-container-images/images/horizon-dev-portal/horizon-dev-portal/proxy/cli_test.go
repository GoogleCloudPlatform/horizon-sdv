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

package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadManifestAndFindArtifact(t *testing.T) {
	dir := t.TempDir()
	h := &cliHandler{distDir: dir}
	if _, err := h.loadManifest(); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing manifest err %v", err)
	}

	writeManifest(t, dir, cliManifest{Name: "", Artifacts: []cliArtifact{{OS: "linux", Arch: "amd64", Filename: "horizon"}}})
	if _, err := h.loadManifest(); err == nil || !strings.Contains(err.Error(), "invalid manifest") {
		t.Fatalf("empty name err %v", err)
	}

	writeManifest(t, dir, cliManifest{Name: "horizon", Artifacts: nil})
	if _, err := h.loadManifest(); err == nil || !strings.Contains(err.Error(), "invalid manifest") {
		t.Fatalf("empty artifacts err %v", err)
	}

	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := h.loadManifest(); err == nil {
		t.Fatal("expected json error")
	}

	want := cliManifest{
		Name:    "horizon",
		Version: "1.2.3",
		Commit:  "abc",
		Artifacts: []cliArtifact{
			{OS: "linux", Arch: "amd64", Filename: "first"},
			{OS: "linux", Arch: "amd64", Filename: "second"},
			{OS: "darwin", Arch: "arm64", Filename: "mac"},
		},
	}
	writeManifest(t, dir, want)
	got, err := h.loadManifest()
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != want.Name || got.Version != want.Version || len(got.Artifacts) != 3 {
		t.Fatalf("manifest %+v", got)
	}
	art, ok := h.findArtifact(got, "linux", "amd64")
	if !ok || art.Filename != "first" {
		t.Fatalf("find %+v ok %v", art, ok)
	}
	if _, ok := h.findArtifact(got, "windows", "amd64"); ok {
		t.Fatal("unexpected artifact")
	}
}

func TestWriteCLIUnavailable(t *testing.T) {
	rec := httptest.NewRecorder()
	writeCLIUnavailable(rec, nil)
	assertCLIUnavailable(t, rec, "cli binaries unavailable")

	rec = httptest.NewRecorder()
	writeCLIUnavailable(rec, errors.New("disk offline"))
	assertCLIUnavailable(t, rec, "disk offline")
}

func TestCLIRoutes(t *testing.T) {
	dir := t.TempDir()
	const fileBody = "cli-bytes"
	if err := os.MkdirAll(filepath.Join(dir, "linux", "amd64"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "linux", "amd64", "horizon"), []byte(fileBody), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "linux", "arm64", "not-a-file"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeManifest(t, dir, cliManifest{
		Name:      "horizon",
		Version:   "1.2.3",
		Commit:    "abc",
		BuildDate: "2026-01-01",
		Artifacts: []cliArtifact{
			{OS: "linux", Arch: "amd64", Filename: "horizon", SHA256: "deadbeef", SizeBytes: int64(len(fileBody)), DownloadPath: "/api/cli/v1/download/linux/amd64"},
			{OS: "linux", Arch: "arm64", Filename: "not-a-file", SHA256: "dir"},
			{OS: "linux", Arch: "386", Filename: "missing", SHA256: "gone"},
			{OS: "darwin", Arch: "arm64", Filename: "nested/horizon", SHA256: "bad"},
			{OS: "windows", Arch: "amd64", Filename: "..", SHA256: "up"},
		},
	})

	t.Run("open routes", func(t *testing.T) {
		mux := http.NewServeMux()
		registerCLIRoutes(mux, dir, nil)

		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/cli/v1/manifest", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("manifest status %d body %s", rec.Code, rec.Body.String())
		}
		if rec.Header().Get("Content-Type") != "application/json; charset=utf-8" || rec.Header().Get("Cache-Control") != "private, no-store" {
			t.Fatalf("headers %#v", rec.Header())
		}
		var m cliManifest
		if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
			t.Fatal(err)
		}
		if m.Version != "1.2.3" || len(m.Artifacts) != 5 {
			t.Fatalf("manifest %+v", m)
		}

		rec = httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/cli/v1/download/linux/amd64", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("download status %d body %s", rec.Code, rec.Body.String())
		}
		if rec.Body.String() != fileBody {
			t.Fatalf("body %q", rec.Body.String())
		}
		if rec.Header().Get("Content-Type") != "application/octet-stream" {
			t.Fatalf("content-type %q", rec.Header().Get("Content-Type"))
		}
		if rec.Header().Get("Content-Disposition") != `attachment; filename="horizon"` {
			t.Fatalf("disposition %q", rec.Header().Get("Content-Disposition"))
		}
		if rec.Header().Get("X-Checksum-SHA256") != "deadbeef" || rec.Header().Get("Cache-Control") != "private, no-store" {
			t.Fatalf("checksum headers %#v", rec.Header())
		}

		for _, path := range []string{
			"/api/cli/v1/download/Linux/amd64",
			"/api/cli/v1/download/linux/AMD64",
			"/api/cli/v1/download/linux_x86/amd64",
			"/api/cli/v1/download/darwin/arm64",
			"/api/cli/v1/download/linux/386",
			"/api/cli/v1/download/linux/arm64",
			"/api/cli/v1/download/windows/amd64",
			"/api/cli/v1/download/freebsd/amd64",
		} {
			rec = httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
			if rec.Code != http.StatusNotFound {
				t.Fatalf("%s status %d body %s", path, rec.Code, rec.Body.String())
			}
		}
	})

	t.Run("direct path values", func(t *testing.T) {
		h := &cliHandler{distDir: dir}
		req := httptest.NewRequest(http.MethodGet, "/download", nil)
		req.SetPathValue("os", "")
		req.SetPathValue("arch", "amd64")
		rec := httptest.NewRecorder()
		h.serveDownload(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("empty os status %d", rec.Code)
		}
	})

	t.Run("manifest unavailable", func(t *testing.T) {
		mux := http.NewServeMux()
		registerCLIRoutes(mux, t.TempDir(), nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/cli/v1/manifest", nil))
		assertCLIUnavailable(t, rec, "manifest.json")

		rec = httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/cli/v1/download/linux/amd64", nil))
		assertCLIUnavailable(t, rec, "manifest.json")

		rec = httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/cli/v1/download/Linux/amd64", nil))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("invalid platform with missing manifest status %d", rec.Code)
		}
	})

	t.Run("auth", func(t *testing.T) {
		v := &stubVerifier{}
		mux := http.NewServeMux()
		registerCLIRoutes(mux, dir, v)

		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/cli/v1/manifest", nil))
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("missing token status %d", rec.Code)
		}

		v.err = errors.New("bad signature")
		req := httptest.NewRequest(http.MethodGet, "/api/cli/v1/manifest", nil)
		req.Header.Set("Authorization", "Bearer nope")
		rec = httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized || v.got != "nope" {
			t.Fatalf("status %d verified %q", rec.Code, v.got)
		}

		v.err = nil
		req = httptest.NewRequest(http.MethodGet, "/api/cli/v1/download/linux/amd64", nil)
		req.Header.Set("Authorization", "bearer  good-token  ")
		rec = httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK || rec.Body.String() != fileBody || v.got != "good-token" {
			t.Fatalf("status %d body %q token %q", rec.Code, rec.Body.String(), v.got)
		}
	})
}

type stubVerifier struct {
	err error
	got string
}

func (s *stubVerifier) Verify(ctx context.Context, rawToken string) error {
	s.got = rawToken
	return s.err
}

func writeManifest(t *testing.T, dir string, m cliManifest) {
	t.Helper()
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertCLIUnavailable(t *testing.T, rec *httptest.ResponseRecorder, detailSub string) {
	t.Helper()
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Content-Type") != "application/json; charset=utf-8" || rec.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("headers %#v", rec.Header())
	}
	var payload map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["error"] != "cli binaries unavailable" || !strings.Contains(payload["detail"], detailSub) {
		t.Fatalf("payload %#v", payload)
	}
}
