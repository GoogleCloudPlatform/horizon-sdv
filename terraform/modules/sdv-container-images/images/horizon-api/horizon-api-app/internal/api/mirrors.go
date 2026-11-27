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
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/acn-horizon-sdv/horizon-api/internal/auth"
	"github.com/acn-horizon-sdv/horizon-api/internal/workflow"
)

// MirrorModuleName is the Module Manager catalog name of the Storage Mirror module.
const MirrorModuleName = "storage-mirror"

const (
	mirrorProxyTimeout = 30 * time.Second
	mirrorMaxBody      = 1 << 20
)

var mirrorNameRe = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]{0,61}[a-z0-9])?$`)

// mirrorRoute is one public Storage Mirror route and its upstream path on the module.
type mirrorRoute struct {
	method   string
	pattern  string
	upstream func(r *http.Request) string
	mutating bool
}

func mirrorRoutes() []mirrorRoute {
	name := func(suffix string) func(*http.Request) string {
		return func(r *http.Request) string { return "/api/v1/mirrors/" + url.PathEscape(r.PathValue("name")) + suffix }
	}
	fixed := func(p string) func(*http.Request) string { return func(*http.Request) string { return p } }
	return []mirrorRoute{
		{"GET", "/v1/storage-mirror/mirrors", fixed("/api/v1/mirrors"), false},
		{"POST", "/v1/storage-mirror/mirrors", fixed("/api/v1/mirrors"), true},
		{"GET", "/v1/storage-mirror/mirrors/{name}", name(""), false},
		{"PATCH", "/v1/storage-mirror/mirrors/{name}", name(""), true},
		{"DELETE", "/v1/storage-mirror/mirrors/{name}", name(""), true},
		{"POST", "/v1/storage-mirror/mirrors/{name}/sync", name("/sync"), true},
		{"POST", "/v1/storage-mirror/mirrors/{name}/sync/cancel", name("/sync/cancel"), true},
		{"GET", "/v1/storage-mirror/mirrors/{name}/status", name("/status"), false},
		{"GET", "/v1/storage-mirror/mirrors/{name}/runs", name("/runs"), false},
		{"POST", "/v1/storage-mirror/syncs", fixed("/api/v1/syncs"), true},
		{"POST", "/v1/storage-mirror/import", fixed("/api/v1/import"), true},
		{"GET", "/v1/storage-mirror/volume", fixed("/api/v1/volume"), false},
		{"DELETE", "/v1/storage-mirror/volume", fixed("/api/v1/volume"), true},
		{"GET", "/v1/storage-mirror/config", fixed("/api/v1/config"), false},
	}
}

func (s *Server) registerMirrorRoutes(mux *http.ServeMux) {
	for _, rt := range mirrorRoutes() {
		rt := rt
		mux.HandleFunc(rt.method+" "+rt.pattern, s.withAuth(func(w http.ResponseWriter, r *http.Request, p *auth.Principal) {
			s.handleMirrorProxy(w, r, p, rt)
		}))
	}
}

func (s *Server) mirrorModuleEnabled(ctx context.Context) (bool, error) {
	if s.opt.MirrorModuleEnabled != nil {
		return s.opt.MirrorModuleEnabled(ctx)
	}
	if s.opt.Catalog == nil {
		return false, errors.New("catalog not configured")
	}
	return s.opt.Catalog.ModuleEnabled(ctx, MirrorModuleName)
}

// handleMirrorProxy forwards one Storage Mirror call to the module's in-cluster REST API as caller horizon-api.
func (s *Server) handleMirrorProxy(w http.ResponseWriter, r *http.Request, p *auth.Principal, rt mirrorRoute) {
	if n := r.PathValue("name"); n != "" && !mirrorNameRe.MatchString(n) {
		http.Error(w, "invalid mirror name", http.StatusBadRequest)
		return
	}
	enabled, err := s.mirrorModuleEnabled(r.Context())
	if err != nil {
		log.Printf("storage-mirror enabled check: %v", err)
		http.Error(w, "could not resolve module state", http.StatusInternalServerError)
		return
	}
	if !enabled {
		http.Error(w, "storage-mirror module is not enabled", http.StatusNotFound)
		return
	}
	if rt.mutating && s.opt.MirrorAdminRole != "" && !p.HasRole(s.opt.MirrorAdminRole) {
		http.Error(w, "role "+s.opt.MirrorAdminRole+" required", http.StatusForbidden)
		return
	}
	if s.opt.MirrorModuleURL == "" {
		http.Error(w, "storage-mirror module not configured", http.StatusServiceUnavailable)
		return
	}
	token, err := readSecretFile(s.opt.MirrorModuleTokenFile, s.opt.MirrorModuleToken)
	if err != nil || token == "" {
		http.Error(w, "storage-mirror module token not configured", http.StatusServiceUnavailable)
		return
	}
	client, err := moduleHTTPClient(s.opt.MirrorModuleCAFile, mirrorProxyTimeout)
	if err != nil {
		log.Printf("storage-mirror client: %v", err)
		http.Error(w, "storage-mirror module CA not configured", http.StatusServiceUnavailable)
		return
	}
	target := strings.TrimSuffix(s.opt.MirrorModuleURL, "/") + rt.upstream(r)
	if q := r.URL.RawQuery; q != "" {
		target += "?" + q
	}
	var body io.Reader
	if r.Body != nil && r.Method != http.MethodGet {
		body = io.LimitReader(r.Body, mirrorMaxBody)
	}
	req, err := http.NewRequestWithContext(r.Context(), r.Method, target, body)
	if err != nil {
		http.Error(w, "failed to create proxy request", http.StatusInternalServerError)
		return
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Horizon-User", p.Subject)
	if from, err := workflow.ParseSubmittedFromHeader(r); err == nil && from != "" {
		req.Header.Set("X-Horizon-Submitted-From", from)
	}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("storage-mirror proxy: %v", err)
		if errors.Is(err, context.DeadlineExceeded) {
			http.Error(w, "storage-mirror module timed out", http.StatusGatewayTimeout)
			return
		}
		http.Error(w, "failed to proxy to storage-mirror module", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		http.Error(w, "storage-mirror module rejected the Horizon API token", http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(w, io.LimitReader(resp.Body, 16<<20))
}

// readSecretFile reads a mounted secret key on every call (rotation), falling back to a static value.
func readSecretFile(path, fallback string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return strings.TrimSpace(fallback), nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

func moduleHTTPClient(caFile string, timeout time.Duration) (*http.Client, error) {
	base, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		base = &http.Transport{}
	}
	tr := base.Clone()
	if caFile = strings.TrimSpace(caFile); caFile != "" {
		pemBytes, err := os.ReadFile(caFile)
		if err != nil {
			return nil, fmt.Errorf("read CA: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pemBytes) {
			return nil, errors.New("parse CA")
		}
		tr.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: pool}
	}
	return &http.Client{Timeout: timeout, Transport: tr}, nil
}
