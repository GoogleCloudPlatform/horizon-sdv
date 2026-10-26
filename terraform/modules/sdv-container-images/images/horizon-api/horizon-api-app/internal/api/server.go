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
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/acn-horizon-sdv/horizon-api/internal/argo"
	"github.com/acn-horizon-sdv/horizon-api/internal/auth"
	"github.com/acn-horizon-sdv/horizon-api/internal/catalog"
	"github.com/acn-horizon-sdv/horizon-api/internal/invoke"
	"github.com/acn-horizon-sdv/horizon-api/internal/softfeature"
	"github.com/acn-horizon-sdv/horizon-api/internal/workflow"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Options struct {
	Catalog          *catalog.Catalog
	OIDC             *auth.OIDC
	WorkflowsNS      string
	HTTPAddr         string
	EventsWebhookURL string
	// Argo REST client (in-cluster SA token → Argo server with --auth-mode=client). Nil or empty BaseURL disables log streaming.
	Argo            *argo.Client
	LogReadIdle     time.Duration
	LogMaxReconnect int
	WorkflowStore   *workflow.Store
	// GCSArtifactBucket builds gs:// links in archivedLogs when status uses relative keys (optional).
	GCSArtifactBucket string
	// GCSSigningServiceAccount is the service account email used to sign V4 GET URLs for artifact download; empty disables signing (501).
	GCSSigningServiceAccount string
	Retention                workflow.Retention
	// WorkflowDeleteWaitTimeout bounds how long DELETE /v1/workflows/{name} waits for the CR to disappear (finalizers). Zero means 10 minutes.
	WorkflowDeleteWaitTimeout time.Duration
	// K8sClient is used for runtime injection (e.g. sampleSoftEnabled). Nil skips injection.
	K8sClient       client.Client
	ModuleManagerNS string
	StateCRName     string
	CatalogCRName   string
}

type Server struct {
	opt Options
	srv *http.Server
}

func New(opt Options) *Server {
	mux := http.NewServeMux()
	s := &Server{opt: opt}
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /v1/catalog", s.withAuth(s.handleCatalog))
	mux.HandleFunc("GET /openapi.json", s.handleOpenAPI)
	mux.HandleFunc("GET /swagger", s.handleSwagger)
	mux.HandleFunc("POST /v1/modules/{module}/workflowTemplates/{template}/submit", s.withAuth(s.handleSubmit))
	mux.HandleFunc("GET /v1/workflows/running", s.withAuth(s.handleWorkflowsRunning))
	mux.HandleFunc("GET /v1/workflows/history", s.withAuth(s.handleWorkflowsHistory))
	mux.HandleFunc("GET /v1/workflows/{workflowName}/log", s.withAuth(s.handleWorkflowLogs))
	mux.HandleFunc("GET /v1/workflows/{workflowName}/downloadArtifact/{artifactName}", s.withAuth(s.handleWorkflowDownloadArtifact))
	mux.HandleFunc("GET /v1/workflows/{workflowName}", s.withAuth(s.handleWorkflowGet))
	mux.HandleFunc("DELETE /v1/workflows/{workflowName}", s.withAuth(s.handleWorkflowDelete))
	mux.HandleFunc("POST /v1/workflows/{workflowName}/abort", s.withAuth(s.handleWorkflowAbort))
	s.srv = &http.Server{Addr: opt.HTTPAddr, Handler: mux}
	return s
}

func (s *Server) Start(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		log.Printf("http: listening on %s", s.srv.Addr)
		if err := s.srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			return
		}
		errCh <- nil
	}()
	select {
	case <-ctx.Done():
		shCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return s.srv.Shutdown(shCtx)
	case err := <-errCh:
		return err
	}
}

func (s *Server) bearerAuth(r *http.Request) (*auth.Principal, error) {
	h := strings.TrimSpace(r.Header.Get("Authorization"))
	parts := strings.SplitN(h, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
		return nil, fmt.Errorf("missing bearer")
	}
	raw := strings.TrimSpace(parts[1])
	if raw == "" {
		return nil, fmt.Errorf("empty token")
	}
	if strings.Count(raw, ".") != 2 {
		return nil, fmt.Errorf("Keycloak access token JWT required")
	}
	return s.opt.OIDC.Verify(r.Context(), raw)
}

func (s *Server) withAuth(next func(http.ResponseWriter, *http.Request, *auth.Principal)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, err := s.bearerAuth(r)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r, p)
	}
}

func (s *Server) handleCatalog(w http.ResponseWriter, r *http.Request, _ *auth.Principal) {
	if r.Method != http.MethodGet {
		http.NotFound(w, r)
		return
	}
	entries := s.opt.Catalog.Snapshot()
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"entries": entries})
}

func (s *Server) handleOpenAPI(w http.ResponseWriter, r *http.Request) {
	meta := catalog.RetentionOpenAPI{
		SecondsAfterSuccess:    s.opt.Retention.SecondsAfterSuccess,
		SecondsAfterFailure:    s.opt.Retention.SecondsAfterFailure,
		SecondsAfterCompletion: s.opt.Retention.SecondsAfterCompletion,
		Explanation:            s.opt.Retention.Explanation,
	}
	b, err := catalog.OpenAPISpec(s.opt.Catalog.Snapshot(), meta)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(b)
}

func (s *Server) handleSwagger(w http.ResponseWriter, r *http.Request) {
	const html = `<!DOCTYPE html><html><head><title>Horizon API</title>
<link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css"/>
</head><body><div id="swagger-ui"></div>
<script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
<script>SwaggerUIBundle({url:new URL('openapi.json',window.location.href).href,dom_id:'#swagger-ui'});</script>
</body></html>`
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(html))
}

func (s *Server) handleSubmit(w http.ResponseWriter, r *http.Request, p *auth.Principal) {
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	module := r.PathValue("module")
	tpl := r.PathValue("template")
	if module == "" || tpl == "" {
		http.Error(w, "bad path", http.StatusBadRequest)
		return
	}
	ent, ok := s.opt.Catalog.Get(module, tpl)
	if !ok {
		http.Error(w, "unknown or disabled template", http.StatusNotFound)
		return
	}
	var req struct {
		Parameters map[string]string `json:"parameters"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	if err := validateParams(ent, req.Parameters); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if s.opt.EventsWebhookURL == "" {
		http.Error(w, "Argo Events webhook URL not configured", http.StatusServiceUnavailable)
		return
	}
	submittedFrom, err := workflow.ParseSubmittedFromHeader(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.Parameters != nil {
		if raw := strings.TrimSpace(req.Parameters[workflow.SubmitParamSubmittedFrom]); raw != "" {
			submittedFrom, err = workflow.ParseSubmittedFromValue(raw)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		}
	}
	horizonSubmittedBy := workflow.ParseSubmittedByHeader(r)
	if req.Parameters != nil {
		if v := strings.TrimSpace(req.Parameters[workflow.SubmitParamSubmittedBy]); v != "" {
			horizonSubmittedBy = v
		}
	}
	if horizonSubmittedBy == "" {
		horizonSubmittedBy = strings.TrimSpace(p.Subject)
	}
	body := map[string]interface{}{
		"workflowTemplateName":            ent.TemplateName,
		"workflowTemplateNamespace":       ent.Namespace,
		"horizonModule":                   module,
		workflow.WebhookFieldSubmittedBy:  horizonSubmittedBy,
		workflow.SubmitParamSubmittedFrom: submittedFrom,
	}
	if horizonSubmittedBy != "" {
		body[workflow.SubmitParamSubmittedBy] = horizonSubmittedBy
	}
	if req.Parameters != nil {
		for k, v := range req.Parameters {
			if k == workflow.SubmitParamSubmittedFrom || k == workflow.SubmitParamSubmittedBy {
				continue
			}
			body[k] = v
		}
	}
	if module == "sample" && ent.TemplateName == "sample-smoke-test" {
		val := "false"
		if s.opt.K8sClient != nil {
			enabled, err := softfeature.SoftModuleEnabledForParent(
				r.Context(),
				s.opt.K8sClient,
				s.opt.ModuleManagerNS,
				s.opt.StateCRName,
				s.opt.CatalogCRName,
				"sample",
				"sample-soft",
			)
			if err != nil {
				log.Printf("sampleSoftEnabled: %v", err)
				http.Error(w, "could not resolve soft features: "+err.Error(), http.StatusInternalServerError)
				return
			}
			if enabled {
				val = "true"
			}
		}
		// Always set so Argo Events Sensor can map body.sampleSoftEnabled (fifth parameter) on every submit.
		body[workflow.SubmitParamSampleSoftEnabled] = val
	}
	if err := invoke.PostArgoEventsWebhook(r.Context(), s.opt.EventsWebhookURL, body); err != nil {
		log.Printf("events webhook: %v", err)
		http.Error(w, "could not dispatch to Argo Events: "+err.Error(), http.StatusBadGateway)
		return
	}
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "dispatched"})
}

func validateParams(ent catalog.Entry, got map[string]string) error {
	if got == nil {
		got = map[string]string{}
	}
	allowed := make(map[string]bool)
	for _, p := range ent.Parameters {
		allowed[p.Name] = true
		if p.Default != "" {
			continue
		}
		// Resolved from X-Horizon-Submitted-From / X-Horizon-Submitted-By unless body override is non-empty.
		if p.Name == workflow.SubmitParamSubmittedFrom || p.Name == workflow.SubmitParamSubmittedBy {
			continue
		}
		if _, ok := got[p.Name]; !ok {
			return fmt.Errorf("missing parameter %q", p.Name)
		}
	}
	for k := range got {
		if k == "" || !allowed[k] {
			return fmt.Errorf("unknown or empty parameter key %q", k)
		}
	}
	return nil
}
