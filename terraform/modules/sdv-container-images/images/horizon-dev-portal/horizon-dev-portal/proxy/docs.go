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
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// Bundled documentation lives in the embedded SPA tree (written there by scripts/bundle-docs.mjs).
// It is only reachable through the authenticated /api/docs routes, never as a static file.
const (
	docsFilesDir  = "bundled-docs"
	docsImagesDir = "docs-images"
)

var docsImageTypes = map[string]string{
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".gif":  "image/gif",
	".svg":  "image/svg+xml",
	".webp": "image/webp",
}

func isDocsStaticPath(p string) bool {
	return p == docsFilesDir || p == docsImagesDir ||
		strings.HasPrefix(p, docsFilesDir+"/") || strings.HasPrefix(p, docsImagesDir+"/")
}

// cleanDocsPath accepts only already-normalized relative paths, so "..", "." and empty
// segments are rejected instead of being resolved.
func cleanDocsPath(p string) (string, bool) {
	if p == "" || strings.HasPrefix(p, "/") || strings.Contains(p, "\\") {
		return "", false
	}
	if path.Clean(p) != p {
		return "", false
	}
	for _, seg := range strings.Split(p, "/") {
		if seg == "." || seg == ".." {
			return "", false
		}
	}
	return p, true
}

func registerDocsRoutes(mux *http.ServeMux, spa fs.FS, v bearerVerifier) {
	mux.Handle("GET /api/docs/v1/files/{path...}", requireBearer(v, func(w http.ResponseWriter, r *http.Request) {
		p, ok := cleanDocsPath(r.PathValue("path"))
		var contentType string
		switch {
		case !ok:
		case p == "manifest.json":
			contentType = "application/json; charset=utf-8"
		case strings.HasSuffix(p, ".md"):
			contentType = "text/markdown; charset=utf-8"
		default:
			ok = false
		}
		if !ok {
			http.NotFound(w, r)
			return
		}
		serveDocsFile(w, r, spa, docsFilesDir+"/"+p, contentType)
	}))
	mux.Handle("GET /api/docs/v1/images/{path...}", requireBearer(v, func(w http.ResponseWriter, r *http.Request) {
		p, ok := cleanDocsPath(r.PathValue("path"))
		contentType := docsImageTypes[strings.ToLower(path.Ext(p))]
		if !ok || contentType == "" {
			http.NotFound(w, r)
			return
		}
		serveDocsFile(w, r, spa, docsImagesDir+"/"+p, contentType)
	}))
}

func serveDocsFile(w http.ResponseWriter, r *http.Request, spa fs.FS, name, contentType string) {
	b, err := fs.ReadFile(spa, name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if contentType == "image/svg+xml" {
		w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
	}
	_, _ = w.Write(b)
}
