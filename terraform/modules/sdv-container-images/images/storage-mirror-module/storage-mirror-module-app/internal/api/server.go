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

// Package api serves the Storage Mirror REST API (public /api/v1 routes and agent /internal routes).
package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"

	smv1 "storage-mirror-module/api/v1alpha1"
	"storage-mirror-module/internal/state"
)

const (
	// HeaderUser carries the end-user subject from the Horizon API.
	HeaderUser = "X-Horizon-User"
	// HeaderSubmittedFrom carries the Horizon client (api, developer-portal, horizon-cli, custom).
	HeaderSubmittedFrom = "X-Horizon-Submitted-From"

	maxBody = 1 << 20
	// maxCatalogBody bounds catalog snapshots (metadata.yaml is a few KiB per mirror).
	maxCatalogBody = 2 << 20
)

var dnsLabel = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]{0,61}[a-z0-9])?$`)

// Server is the REST API.
type Server struct {
	// Client writes. Reader serves read-only routes and may be the manager cache. APIReader reads from the API server
	// and is used for every read that a write or a dispatch decision depends on.
	Client            client.Client
	Reader            client.Reader
	APIReader         client.Reader
	Namespace         string
	WorkflowNamespace string
	Tokens            []APIToken
	Progress          *state.Store
	MirrorEvents      chan event.GenericEvent
	ConfigEvents      chan event.GenericEvent
	Now               func() time.Time
}

func (s *Server) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *Server) reader() client.Reader {
	if s.Reader != nil {
		return s.Reader
	}
	return s.Client
}

func (s *Server) live() client.Reader {
	if s.APIReader != nil {
		return s.APIReader
	}
	return s.Client
}

// Handler returns the authenticated API mux.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/mirrors", s.listMirrors)
	mux.HandleFunc("POST /api/v1/mirrors", s.createMirror)
	mux.HandleFunc("GET /api/v1/mirrors/{name}", s.getMirror)
	mux.HandleFunc("PATCH /api/v1/mirrors/{name}", s.patchMirror)
	mux.HandleFunc("DELETE /api/v1/mirrors/{name}", s.deleteMirror)
	mux.HandleFunc("POST /api/v1/mirrors/{name}/sync", s.syncMirror)
	mux.HandleFunc("POST /api/v1/mirrors/{name}/sync/cancel", s.cancelSync)
	mux.HandleFunc("GET /api/v1/mirrors/{name}/status", s.mirrorStatus)
	mux.HandleFunc("GET /api/v1/mirrors/{name}/runs", s.mirrorRuns)
	mux.HandleFunc("POST /api/v1/syncs", s.batchSync)
	mux.HandleFunc("POST /api/v1/import", s.importMirrors)
	mux.HandleFunc("GET /api/v1/volume", s.getVolume)
	mux.HandleFunc("DELETE /api/v1/volume", s.destroyVolume)
	mux.HandleFunc("GET /api/v1/config", s.getConfig)
	mux.HandleFunc("PUT /internal/v1/runs/{runId}/progress", s.putProgress)
	mux.HandleFunc("POST /internal/v1/catalog", s.postCatalog)
	return s.withAuth(mux)
}

// HealthHandler serves GET /health (kubelet probes over plain HTTP).
func HealthHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	return mux
}

func (s *Server) wakeMirror(name string) {
	if s.MirrorEvents == nil {
		return
	}
	m := &smv1.StorageMirror{}
	m.Name, m.Namespace = name, s.Namespace
	select {
	case s.MirrorEvents <- event.GenericEvent{Object: m}:
	default:
	}
}

func (s *Server) wakeConfig() {
	if s.ConfigEvents == nil {
		return
	}
	c := &smv1.StorageMirrorConfig{}
	c.Name, c.Namespace = smv1.ConfigName, s.Namespace
	select {
	case s.ConfigEvents <- event.GenericEvent{Object: c}:
	default:
	}
}

func user(r *http.Request) string {
	if u := strings.TrimSpace(r.Header.Get(HeaderUser)); u != "" {
		return u
	}
	return CallerFromRequest(r)
}

var submittedFromValues = map[string]bool{"api": true, "developer-portal": true, "horizon-cli": true, "custom": true}

func submittedFrom(r *http.Request) string {
	v := strings.TrimSpace(r.Header.Get(HeaderSubmittedFrom))
	if submittedFromValues[v] {
		return v
	}
	return "api"
}

func decode(r *http.Request, v interface{}) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, maxBody))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, detail string) {
	writeJSON(w, code, map[string]string{"detail": detail})
}

func workflowObject() *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetAPIVersion("argoproj.io/v1alpha1")
	u.SetKind("Workflow")
	return u
}
