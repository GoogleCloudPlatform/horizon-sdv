// Copyright (c) 2024-2026 Accenture, All Rights Reserved.
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
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
)

var cliPlatformSegment = regexp.MustCompile(`^[a-z0-9]+$`)

type bearerVerifier interface {
	Verify(ctx context.Context, rawToken string) error
}

type oidcBearerVerifier struct {
	inner *oidc.IDTokenVerifier
}

func (v oidcBearerVerifier) Verify(ctx context.Context, rawToken string) error {
	_, err := v.inner.Verify(ctx, rawToken)
	return err
}

type cliArtifact struct {
	OS           string `json:"os"`
	Arch         string `json:"arch"`
	Filename     string `json:"filename"`
	SHA256       string `json:"sha256"`
	SizeBytes    int64  `json:"sizeBytes"`
	DownloadPath string `json:"downloadPath"`
}

type cliManifest struct {
	Name      string        `json:"name"`
	Version   string        `json:"version"`
	Commit    string        `json:"commit"`
	BuildDate string        `json:"buildDate"`
	Artifacts []cliArtifact `json:"artifacts"`
}

type cliHandler struct {
	distDir string
}

func registerCLIRoutes(mux *http.ServeMux, distDir string, v bearerVerifier) {
	h := &cliHandler{distDir: distDir}
	wrap := func(next http.HandlerFunc) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if v != nil {
				raw, err := bearerToken(r)
				if err != nil {
					http.Error(w, "unauthorized", http.StatusUnauthorized)
					return
				}
				if err := v.Verify(r.Context(), raw); err != nil {
					http.Error(w, "unauthorized", http.StatusUnauthorized)
					return
				}
			}
			next(w, r)
		})
	}
	mux.Handle("GET /api/cli/v1/manifest", wrap(h.serveManifest))
	mux.Handle("GET /api/cli/v1/download/{os}/{arch}", wrap(h.serveDownload))
}

func (h *cliHandler) loadManifest() (*cliManifest, error) {
	raw, err := os.ReadFile(filepath.Join(h.distDir, "manifest.json"))
	if err != nil {
		return nil, err
	}
	var m cliManifest
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	if m.Name == "" || len(m.Artifacts) == 0 {
		return nil, fmt.Errorf("invalid manifest")
	}
	return &m, nil
}

func (h *cliHandler) findArtifact(m *cliManifest, osName, arch string) (cliArtifact, bool) {
	for _, a := range m.Artifacts {
		if a.OS == osName && a.Arch == arch {
			return a, true
		}
	}
	return cliArtifact{}, false
}

func writeCLIUnavailable(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "private, no-store")
	w.WriteHeader(http.StatusServiceUnavailable)
	detail := "cli binaries unavailable"
	if err != nil {
		detail = err.Error()
	}
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error":  "cli binaries unavailable",
		"detail": detail,
	})
}

func (h *cliHandler) serveManifest(w http.ResponseWriter, r *http.Request) {
	m, err := h.loadManifest()
	if err != nil {
		writeCLIUnavailable(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "private, no-store")
	_ = json.NewEncoder(w).Encode(m)
}

func (h *cliHandler) serveDownload(w http.ResponseWriter, r *http.Request) {
	osName := r.PathValue("os")
	arch := r.PathValue("arch")
	if !cliPlatformSegment.MatchString(osName) || !cliPlatformSegment.MatchString(arch) {
		http.NotFound(w, r)
		return
	}
	m, err := h.loadManifest()
	if err != nil {
		writeCLIUnavailable(w, err)
		return
	}
	art, ok := h.findArtifact(m, osName, arch)
	if !ok || art.Filename == "" || filepath.Base(art.Filename) != art.Filename {
		http.NotFound(w, r)
		return
	}
	path := filepath.Join(h.distDir, art.OS, art.Arch, art.Filename)
	f, err := os.Open(path)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || st.IsDir() {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, art.Filename))
	w.Header().Set("X-Checksum-SHA256", art.SHA256)
	w.Header().Set("Cache-Control", "private, no-store")
	http.ServeContent(w, r, art.Filename, time.Time{}, f)
}
