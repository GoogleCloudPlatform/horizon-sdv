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

package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/acn-horizon-sdv/module-manager/internal/controller"
	"github.com/acn-horizon-sdv/module-manager/internal/overviewfetch"
)

// ModuleApplication is catalog metadata for a module child application (public URL or path).
type ModuleApplication struct {
	ID    string `json:"id"`
	Title string `json:"title,omitempty"`
	URL   string `json:"url"`
}

// ModuleResponse is the JSON shape for a single module.
type ModuleResponse struct {
	ID                   string   `json:"id"`
	Name                 string   `json:"name"`
	Enabled              bool     `json:"enabled"`
	HardDependencies     []string `json:"hardDependencies,omitempty"`
	SoftDependencies     []string `json:"softDependencies,omitempty"`
	ApplicationName      string   `json:"applicationName,omitempty"`
	ApplicationNamespace string   `json:"applicationNamespace,omitempty"`
	// Applications lists Developer Portal links: ModuleCatalog entries plus Argo child Applications
	// labeled horizon-sdv.io/expose=true with horizon-sdv.io/portal-url (see module_applications_discover.go).
	Applications []ModuleApplication `json:"applications,omitempty"`
	// AutoDisableWhenUnused mirrors ModuleCatalog.spec.modules[].autoDisableWhenUnused; when unset in JSON it is false.
	AutoDisableWhenUnused bool `json:"autoDisableWhenUnused,omitempty"`
	// HardDependentCount is the number of enabled modules that list this module as a hard dependency.
	HardDependentCount *int `json:"hardDependentCount,omitempty"`
	// HardDependents are those module names (sorted).
	HardDependents []string `json:"hardDependents,omitempty"`
	// SoftDependentCount is the number of enabled modules that list this module as a soft dependency.
	SoftDependentCount *int `json:"softDependentCount,omitempty"`
	// SoftDependents are those soft-dependent module names (sorted).
	SoftDependents []string `json:"softDependents,omitempty"`
	// OverviewPath is a path relative to the catalog module path (e.g. portal/overview.html) for chart packaging only.
	OverviewPath string `json:"overviewPath,omitempty"`
	// OverviewService is the in-cluster Kubernetes Service that serves overview HTML.
	OverviewService string `json:"overviewService,omitempty"`
	// OverviewServiceNamespace is the namespace containing OverviewService.
	OverviewServiceNamespace string `json:"overviewServiceNamespace,omitempty"`
	// TargetRevision is the effective Git ref for this module's Argo CD Application (per-module or cluster default).
	TargetRevision string `json:"targetRevision,omitempty"`
	// ClusterTargetRevision is the Module Manager process default (--target-revision).
	ClusterTargetRevision string `json:"clusterTargetRevision,omitempty"`
	// Pinned is true when the module has an explicit Git ref in ModuleManagerState (not following the platform default).
	Pinned bool `json:"pinned,omitempty"`
}

// WorkflowsVisibilityDTO is the JSON body for GET/PUT /settings/workflows-visibility.
type WorkflowsVisibilityDTO struct {
	// AllowedSubmittedFrom lists horizon-sdv.io/submitted-from values shown in the Developer Portal.
	// JSON null or omitted field means show all sources (no restriction). Present empty array hides every workflow.
	AllowedSubmittedFrom *[]string `json:"allowedSubmittedFrom,omitempty"`
}

// StatusResponse is the JSON shape for module status (ArgoCD sync/health).
type StatusResponse struct {
	SyncStatus      string `json:"syncStatus,omitempty"`
	HealthStatus    string `json:"healthStatus,omitempty"`
	OperationPhase  string `json:"operationPhase,omitempty"`
	DesiredRevision string `json:"desiredRevision,omitempty"`
	SyncRevision    string `json:"syncRevision,omitempty"`
	// ApplicationDeletionTimestamp is set when the parent or any module-managed Application has metadata.deletionTimestamp.
	ApplicationDeletionTimestamp string `json:"applicationDeletionTimestamp,omitempty"`
	// RemainingManagedApplications is set only when the module is disabled: count of parent+child Argo CD Applications still present for this module.
	RemainingManagedApplications *int `json:"remainingManagedApplications,omitempty"`
	// ManagedApplicationCount is set when the module is enabled and listing managed Applications succeeded: parent+child count (labeled horizon-sdv.io/app-role).
	ManagedApplicationCount *int `json:"managedApplicationCount,omitempty"`
	// ExpectedManagedApplicationCount is 2 for modules with a prefixed child Application (workloads-android, workloads-common).
	ExpectedManagedApplicationCount *int `json:"expectedManagedApplicationCount,omitempty"`
	// ManagedChildApplicationPresent is false when the labeled child Application is absent (disable teardown or enable not finished).
	ManagedChildApplicationPresent *bool `json:"managedChildApplicationPresent,omitempty"`
	// ParentSkipReconcile is true when the mod-* parent has argocd.argoproj.io/skip-reconcile (set during prefixed-module disable).
	ParentSkipReconcile *bool `json:"parentSkipReconcile,omitempty"`
}

func fillArgoAppStatus(app *unstructured.Unstructured, status *StatusResponse) {
	if s, _, _ := unstructured.NestedString(app.Object, "status", "sync", "status"); s != "" {
		status.SyncStatus = s
	}
	if s, _, _ := unstructured.NestedString(app.Object, "status", "health", "status"); s != "" {
		status.HealthStatus = s
	}
	if s, _, _ := unstructured.NestedString(app.Object, "status", "operationState", "phase"); s != "" {
		status.OperationPhase = s
	}
	if s, _, _ := unstructured.NestedString(app.Object, "spec", "source", "targetRevision"); s != "" {
		status.DesiredRevision = s
	}
	if s, _, _ := unstructured.NestedString(app.Object, "status", "sync", "revision"); s != "" {
		status.SyncRevision = s
	}
	if ts := metadataDeletionTimestampRFC3339(app); ts != "" {
		status.ApplicationDeletionTimestamp = ts
	}
}

// anyManagedApplicationDeletionTimestamp returns metadata.deletionTimestamp from the parent Application or any
// module-managed Argo CD Application (parent/child). During disable the child is often deleted before the parent
// while ModuleManagerState still reports enabled; exposing this timestamp keeps UI from treating prune as install.
func anyManagedApplicationDeletionTimestamp(parentErr error, parent *unstructured.Unstructured, managed []unstructured.Unstructured) string {
	if parentErr == nil && parent != nil {
		if ts := metadataDeletionTimestampRFC3339(parent); ts != "" {
			return ts
		}
	}
	for i := range managed {
		if ts := metadataDeletionTimestampRFC3339(&managed[i]); ts != "" {
			return ts
		}
	}
	return ""
}

// Handler implements the REST API for the Module Manager.
type Handler struct {
	client             client.Client
	apiReader          client.Reader
	stateStore         controller.StateStoreInterface
	catalogStore       controller.CatalogStoreInterface
	namespace          string
	argocdNamespace    string
	argocdProject      string
	repoURL            string
	targetRevision     string
	moduleConfig       string
	gitOpsRootAppNames []string
}

// NewHandler returns a new API handler.
func NewHandler(
	c client.Client,
	apiReader client.Reader,
	stateStore controller.StateStoreInterface,
	catalogStore controller.CatalogStoreInterface,
	namespace, argocdNamespace, argocdProject, repoURL, targetRevision, moduleConfig string,
	gitOpsRootAppNames []string,
) *Handler {
	return &Handler{
		client:             c,
		apiReader:          apiReader,
		stateStore:         stateStore,
		catalogStore:       catalogStore,
		namespace:          namespace,
		argocdNamespace:    argocdNamespace,
		argocdProject:      argocdProject,
		repoURL:            repoURL,
		targetRevision:     targetRevision,
		moduleConfig:       moduleConfig,
		gitOpsRootAppNames: gitOpsRootAppNames,
	}
}

// Routes returns the http.ServeMux for the API.
// Register more specific paths first so /modules/foo/status matches before /modules/foo.
func (h *Handler) Routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /openapi.json", h.serveOpenAPI)
	mux.HandleFunc("GET /swagger", h.serveSwaggerUI)
	mux.HandleFunc("GET /settings/workflows-visibility", h.getWorkflowsVisibility)
	mux.HandleFunc("PUT /settings/workflows-visibility", h.putWorkflowsVisibility)
	mux.HandleFunc("GET /modules", h.listModules)
	mux.HandleFunc("GET /modules/{idOrName}/status", h.getModuleStatus)
	mux.HandleFunc("GET /modules/{idOrName}/overview", h.getModuleOverview)
	mux.HandleFunc("PUT /modules/{idOrName}/target-revision", h.putModuleTargetRevision)
	mux.HandleFunc("DELETE /modules/{idOrName}/target-revision", h.deleteModuleTargetRevision)
	mux.HandleFunc("POST /modules/{idOrName}/enable", h.enableModule)
	mux.HandleFunc("DELETE /modules/{idOrName}/disable", h.disableModule)
	mux.HandleFunc("GET /modules/{idOrName}", h.getModule)
	return mux
}

func (h *Handler) serveOpenAPI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(OpenAPISpec())
}

func (h *Handler) serveSwaggerUI(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// Serve a minimal HTML page that loads Swagger UI and points to /openapi.json
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	const swaggerHTML = `<!DOCTYPE html>
<html>
<head>
  <title>Module Manager API - Swagger UI</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5.9.0/swagger-ui.css" />
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5.9.0/swagger-ui-bundle.js" crossorigin></script>
  <script>
    window.onload = () => {
      window.ui = SwaggerUIBundle({
        url: "./openapi.json",
        dom_id: "#swagger-ui",
        presets: [
          SwaggerUIBundle.presets.apis,
          SwaggerUIBundle.SwaggerUIStandalonePreset
        ]
      });
    };
  </script>
</body>
</html>
`
	w.Write([]byte(swaggerHTML))
}

func (h *Handler) getWorkflowsVisibility(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx := r.Context()
	state, err := h.stateStore.Get(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var dto WorkflowsVisibilityDTO
	if state != nil && state.WorkflowsVisibility != nil {
		s := append([]string(nil), state.WorkflowsVisibility.AllowedSubmittedFrom...)
		dto.AllowedSubmittedFrom = &s
	}
	writeJSON(w, dto)
}

func (h *Handler) putWorkflowsVisibility(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx := r.Context()
	var raw map[string]json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	state, err := h.stateStore.Get(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if v, ok := raw["allowedSubmittedFrom"]; !ok || string(v) == "null" {
		state.WorkflowsVisibility = nil
	} else {
		var list []string
		if err := json.Unmarshal(v, &list); err != nil {
			http.Error(w, "allowedSubmittedFrom must be a JSON array of strings", http.StatusBadRequest)
			return
		}
		out := make([]string, 0, len(list))
		for _, s := range list {
			s = strings.TrimSpace(s)
			if s != "" {
				out = append(out, s)
			}
		}
		state.WorkflowsVisibility = &controller.WorkflowsVisibilitySettings{AllowedSubmittedFrom: out}
	}
	if err := h.stateStore.Update(ctx, state); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.stateStore.InvalidateCache()
	writeJSON(w, map[string]string{"status": "ok"})
}

func (h *Handler) listModules(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ctx := r.Context()

	nameFilter := r.URL.Query().Get("name")

	catalogEntries, err := h.catalogStore.List(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	state, err := h.stateStore.Get(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	enabledSet := make(map[string]bool)
	for _, id := range state.EnabledModules {
		enabledSet[id] = true
	}

	var list []ModuleResponse
	for i := range catalogEntries {
		entry := &catalogEntries[i]
		if nameFilter != "" && entry.Name != nameFilter {
			continue
		}
		id := state.ModuleIDs[entry.Name]
		enabled := id != "" && enabledSet[id]
		mod := ModuleResponse{
			Name:                  entry.Name,
			ID:                    id,
			Enabled:               enabled,
			HardDependencies:      append([]string(nil), entry.HardDependencies...),
			SoftDependencies:      append([]string(nil), entry.SoftDependencies...),
			AutoDisableWhenUnused: entry.AutoDisableWhenUnused,
			OverviewPath:          strings.TrimSpace(entry.OverviewPath),
		}
		if enabled {
			mod.ApplicationName = controller.ApplicationName(entry.Name)
			mod.ApplicationNamespace = h.argocdNamespace
		}
		h.fillOverviewFromCatalog(&mod, entry)
		mod.Applications = h.mergedApplicationsForModule(ctx, entry.Name, entry.Applications)
		if err := h.attachDependents(ctx, &mod, state); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		h.attachRevisionInfo(&mod, state)
		list = append(list, mod)
	}

	writeJSON(w, list)
}

func (h *Handler) attachDependents(ctx context.Context, mod *ModuleResponse, state *controller.State) error {
	if !mod.Enabled {
		mod.HardDependents = nil
		mod.HardDependentCount = nil
		mod.SoftDependents = nil
		mod.SoftDependentCount = nil
		return nil
	}
	edges, err := controller.ListEnabledReverseDependents(ctx, h.client, h.catalogStore, h.namespace, state, mod.Name)
	if err != nil {
		return err
	}
	mod.HardDependents = edges.Hard
	hardCount := len(edges.Hard)
	mod.HardDependentCount = &hardCount
	mod.SoftDependents = edges.Soft
	softCount := len(edges.Soft)
	mod.SoftDependentCount = &softCount
	return nil
}

func (h *Handler) getModule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	idOrName := r.PathValue("idOrName")
	if idOrName == "" {
		http.Error(w, "idOrName required", http.StatusBadRequest)
		return
	}
	ctx := r.Context()

	mod, err := h.resolveModule(ctx, idOrName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if mod == nil {
		http.Error(w, "module not found", http.StatusNotFound)
		return
	}
	writeJSON(w, mod)
}

func (h *Handler) getModuleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	idOrName := r.PathValue("idOrName")
	if idOrName == "" {
		http.Error(w, "idOrName required", http.StatusBadRequest)
		return
	}
	ctx := r.Context()

	mod, err := h.resolveModule(ctx, idOrName)
	if err != nil || mod == nil {
		http.Error(w, "module not found", http.StatusNotFound)
		return
	}

	status := StatusResponse{}
	argoNS := h.argocdNamespace
	argoName := controller.ApplicationName(mod.Name)
	if mod.ApplicationName != "" && mod.ApplicationNamespace != "" {
		argoNS = mod.ApplicationNamespace
		argoName = mod.ApplicationName
	}
	parentApp := &unstructured.Unstructured{}
	parentApp.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "argoproj.io",
		Version: "v1alpha1",
		Kind:    "Application",
	})
	parentOnly := StatusResponse{}
	parentErr := h.apiReader.Get(ctx, client.ObjectKey{Namespace: argoNS, Name: argoName}, parentApp)
	if parentErr == nil {
		fillArgoAppStatus(parentApp, &parentOnly)
	}

	managed, listErr := h.listModuleManagedArgoApplications(ctx, mod.Name)
	if listErr != nil {
		log.Printf("module status: list managed Argo Applications for %q: %v", mod.Name, listErr)
		if parentErr == nil {
			status = parentOnly
		}
	} else {
		agg := make([]StatusResponse, 0, len(managed))
		for i := range managed {
			var st StatusResponse
			fillArgoAppStatus(&managed[i], &st)
			agg = append(agg, st)
		}
		if len(agg) == 0 {
			if parentErr == nil {
				status = parentOnly
			}
		} else {
			sync, health, op := mergeManagedApplicationStatuses(agg)
			status.SyncStatus = sync
			status.HealthStatus = health
			status.OperationPhase = op
			status.DesiredRevision = parentOnly.DesiredRevision
			status.SyncRevision = parentOnly.SyncRevision
		}
	}

	if ts := anyManagedApplicationDeletionTimestamp(parentErr, parentApp, managed); ts != "" {
		status.ApplicationDeletionTimestamp = ts
	}

	fillPrefixedModuleStackStatus(mod.Name, mod.Enabled, parentErr, parentApp, listErr, managed, &status)

	if listErr == nil {
		n := countRemainingModuleApplications(parentErr, parentApp, managed)
		zero := 0
		if mod.Enabled {
			status.ManagedApplicationCount = &n
		} else {
			if n == 0 {
				ts := status.ApplicationDeletionTimestamp
				status = StatusResponse{
					RemainingManagedApplications: &zero,
				}
				if ts != "" {
					status.ApplicationDeletionTimestamp = ts
				}
			} else {
				status.RemainingManagedApplications = &n
			}
		}
	}

	writeJSON(w, status)
}

func (h *Handler) getModuleOverview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	idOrName := r.PathValue("idOrName")
	if idOrName == "" {
		http.Error(w, "idOrName required", http.StatusBadRequest)
		return
	}
	ctx := r.Context()

	mod, err := h.resolveModule(ctx, idOrName)
	if err != nil || mod == nil {
		http.Error(w, "module not found", http.StatusNotFound)
		return
	}
	entry := h.getCatalogEntry(ctx, mod.Name)
	if entry == nil {
		http.Error(w, "module not found", http.StatusNotFound)
		return
	}
	svc := strings.TrimSpace(entry.OverviewService)
	ns := h.resolvedOverviewServiceNamespace(entry)
	if svc == "" || ns == "" {
		http.Error(w, "configure overview in-cluster service (overviewService and overviewServiceNamespace) in the module catalog", http.StatusNotFound)
		return
	}
	pageURL, err := overviewfetch.BuildInClusterOverviewURL(svc, ns)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	html, err := overviewfetch.FetchHTTPOverview(ctx, nil, pageURL)
	if err != nil {
		if errors.Is(err, overviewfetch.ErrOverviewNotFound) {
			http.Error(w, "module overview not found", http.StatusNotFound)
			return
		}
		log.Printf("getModuleOverview %q from %s: %v", mod.Name, pageURL, err)
		http.Error(w, "failed to load overview from in-cluster service: "+err.Error(), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	_, _ = w.Write(html)
}

func (h *Handler) putModuleTargetRevision(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	idOrName := r.PathValue("idOrName")
	if idOrName == "" {
		http.Error(w, "idOrName required", http.StatusBadRequest)
		return
	}
	var body struct {
		TargetRevision string `json:"targetRevision"`
	}
	if r.Body == nil {
		http.Error(w, `body required: {"targetRevision":"..."}`, http.StatusBadRequest)
		return
	}
	defer r.Body.Close()
	if err := json.NewDecoder(io.LimitReader(r.Body, 64<<10)).Decode(&body); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}
	rev := strings.TrimSpace(body.TargetRevision)
	if rev == "" {
		http.Error(w, "targetRevision must be non-empty", http.StatusBadRequest)
		return
	}
	if controller.IsOpenAPIExamplePlaceholderRevision(rev) {
		http.Error(w, `targetRevision must be a real Git ref, not the OpenAPI placeholder "string"`, http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	mod, err := h.resolveModule(ctx, idOrName)
	if err != nil || mod == nil {
		http.Error(w, "module not found", http.StatusNotFound)
		return
	}
	if !mod.Enabled || mod.ApplicationName == "" || mod.ApplicationNamespace == "" {
		http.Error(w, "module is not enabled", http.StatusConflict)
		return
	}

	state, err := h.stateStore.Get(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	oldRev := ""
	if state.ModuleTargetRevisions != nil {
		oldRev = state.ModuleTargetRevisions[mod.Name]
	}
	if state.ModuleTargetRevisions == nil {
		state.ModuleTargetRevisions = make(map[string]string)
	}
	state.ModuleTargetRevisions[mod.Name] = rev
	if err := h.stateStore.Update(ctx, state); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	parentONS := ""
	if e := h.getCatalogEntry(ctx, mod.Name); e != nil {
		parentONS = h.parentHelmOverviewNamespace(e)
	}
	if err := PatchApplicationTargetRevision(ctx, h.client, mod.ApplicationNamespace, mod.ApplicationName, h.repoURL, rev, mod.Name, h.moduleConfig, parentONS); err != nil {
		state2, gerr := h.stateStore.Get(ctx)
		if gerr == nil {
			if oldRev == "" {
				delete(state2.ModuleTargetRevisions, mod.Name)
			} else {
				if state2.ModuleTargetRevisions == nil {
					state2.ModuleTargetRevisions = make(map[string]string)
				}
				state2.ModuleTargetRevisions[mod.Name] = oldRev
			}
			_ = h.stateStore.Update(ctx, state2)
		}
		h.stateStore.InvalidateCache()
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.stateStore.InvalidateCache()
	writeJSON(w, map[string]string{"status": "ok"})
}

func (h *Handler) deleteModuleTargetRevision(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	idOrName := r.PathValue("idOrName")
	if idOrName == "" {
		http.Error(w, "idOrName required", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	mod, err := h.resolveModule(ctx, idOrName)
	if err != nil || mod == nil {
		http.Error(w, "module not found", http.StatusNotFound)
		return
	}
	if !mod.Enabled || mod.ApplicationName == "" || mod.ApplicationNamespace == "" {
		http.Error(w, "module is not enabled", http.StatusConflict)
		return
	}

	state, err := h.stateStore.Get(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	oldRev := ""
	if state.ModuleTargetRevisions != nil {
		oldRev = state.ModuleTargetRevisions[mod.Name]
	}
	if state.ModuleTargetRevisions != nil {
		delete(state.ModuleTargetRevisions, mod.Name)
		if len(state.ModuleTargetRevisions) == 0 {
			state.ModuleTargetRevisions = nil
		}
	}
	if err := h.stateStore.Update(ctx, state); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defaultRev := strings.TrimSpace(h.targetRevision)
	parentONS := ""
	if e := h.getCatalogEntry(ctx, mod.Name); e != nil {
		parentONS = h.parentHelmOverviewNamespace(e)
	}
	if err := PatchApplicationTargetRevision(ctx, h.client, mod.ApplicationNamespace, mod.ApplicationName, h.repoURL, defaultRev, mod.Name, h.moduleConfig, parentONS); err != nil {
		state2, gerr := h.stateStore.Get(ctx)
		if gerr == nil {
			if oldRev != "" {
				if state2.ModuleTargetRevisions == nil {
					state2.ModuleTargetRevisions = make(map[string]string)
				}
				state2.ModuleTargetRevisions[mod.Name] = oldRev
			}
			_ = h.stateStore.Update(ctx, state2)
		}
		h.stateStore.InvalidateCache()
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.stateStore.InvalidateCache()
	writeJSON(w, map[string]string{"status": "ok"})
}

// platformTeardownBlocksEnable reports whether any configured root Application is deleting,
// so a module enabled now would race the platform drain.
func (h *Handler) platformTeardownBlocksEnable(ctx context.Context) (bool, error) {
	for _, name := range h.gitOpsRootAppNames {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		u := &unstructured.Unstructured{}
		u.SetGroupVersionKind(schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "Application"})
		err := h.client.Get(ctx, client.ObjectKey{Namespace: h.argocdNamespace, Name: name}, u)
		if apierrors.IsNotFound(err) {
			continue
		}
		if err != nil {
			return false, err
		}
		if ts := u.GetDeletionTimestamp(); ts != nil && !ts.IsZero() {
			return true, nil
		}
	}
	return false, nil
}

func (h *Handler) enableModule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	idOrName := r.PathValue("idOrName")
	if idOrName == "" {
		http.Error(w, "idOrName required", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	controller.ModuleOpsMutex.Lock()
	defer controller.ModuleOpsMutex.Unlock()

	blocking, err := h.platformTeardownBlocksEnable(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if blocking {
		http.Error(w, "platform teardown in progress", http.StatusConflict)
		return
	}

	mod, err := h.resolveModule(ctx, idOrName)
	if err != nil || mod == nil {
		http.Error(w, "module not found", http.StatusNotFound)
		return
	}
	if mod.Enabled {
		writeJSON(w, map[string]string{"status": "already enabled"})
		return
	}

	defaultRev := strings.TrimSpace(h.targetRevision)
	var enableBody struct {
		TargetRevision string `json:"targetRevision"`
	}
	if r.Body != nil {
		defer r.Body.Close()
		dec := json.NewDecoder(io.LimitReader(r.Body, 64<<10))
		if err := dec.Decode(&enableBody); err != nil && !errors.Is(err, io.EOF) {
			http.Error(w, "invalid json body", http.StatusBadRequest)
			return
		}
	}
	explicitPin := strings.TrimSpace(enableBody.TargetRevision) != ""
	rev := strings.TrimSpace(enableBody.TargetRevision)
	if controller.IsOpenAPIExamplePlaceholderRevision(rev) {
		rev = ""
	}
	if rev == "" {
		rev = defaultRev
	}
	if rev == "" {
		http.Error(w, "targetRevision cannot be empty", http.StatusBadRequest)
		return
	}

	if err := h.enableOneModule(ctx, mod.Name, rev, explicitPin); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// After enabling, recompute softFeaturesEnabled on parents that list this module as a soft dependency.
	h.patchDependentsSoftFeature(ctx, mod.Name)

	if err := controller.RunAutoDisableSweep(ctx, h.apiReader, h.client, h.namespace, h.argocdNamespace, h.stateStore, h.catalogStore); err != nil {
		log.Printf("auto-disable sweep after enable %q: %v", mod.Name, err)
	}

	w.WriteHeader(http.StatusOK)
	writeJSON(w, map[string]string{"status": "enabled"})
}

// enableOneModule enables a single module by name, recursively enabling its hard dependencies first.
// targetRevision is the Git ref for this module's Argo CD Application (branch, tag, or commit).
// When pinned is false the module follows the cluster default (no moduleTargetRevisions entry).
func (h *Handler) enableOneModule(ctx context.Context, moduleName, targetRevision string, pinned bool) error {
	targetRevision = strings.TrimSpace(targetRevision)
	if controller.IsOpenAPIExamplePlaceholderRevision(targetRevision) {
		targetRevision = strings.TrimSpace(h.targetRevision)
	}
	if targetRevision == "" {
		return fmt.Errorf("targetRevision cannot be empty")
	}
	state, err := h.stateStore.Get(ctx)
	if err != nil {
		return err
	}
	mod, err := h.moduleFromName(ctx, moduleName, state)
	if err != nil {
		return err
	}
	if mod == nil {
		return fmt.Errorf("module not found: %s", moduleName)
	}
	if mod.Enabled {
		return nil
	}

	path, err := h.catalogStore.GetPath(ctx, moduleName)
	if err != nil {
		return err
	}
	if path == "" {
		return fmt.Errorf("module path not in catalog: %s", moduleName)
	}

	hardDeps := mod.HardDependencies
	if len(hardDeps) == 0 {
		if entry := h.getCatalogEntry(ctx, moduleName); entry != nil {
			hardDeps = entry.HardDependencies
		}
	}

	depDefaultRev := strings.TrimSpace(h.targetRevision)
	enabledSet := make(map[string]bool)
	for _, id := range state.EnabledModules {
		enabledSet[id] = true
	}
	for _, dep := range hardDeps {
		depID := state.ModuleIDs[dep]
		if depID != "" && enabledSet[depID] {
			continue
		}
		if err := h.enableOneModule(ctx, dep, depDefaultRev, false); err != nil {
			return err
		}
		state, err = h.stateStore.Get(ctx)
		if err != nil {
			return err
		}
		enabledSet = make(map[string]bool)
		for _, id := range state.EnabledModules {
			enabledSet[id] = true
		}
	}

	if err := controller.WaitPrefixedModuleChildApplicationAbsent(ctx, h.client, h.argocdNamespace, h.moduleConfig, moduleName); err != nil {
		return err
	}
	if err := controller.WaitWorkflowsConfigConnectorContextAbsentOrNotTerminating(ctx, h.client, h.moduleConfig, moduleName); err != nil {
		return err
	}
	if err := controller.WaitModuleKCCNamespaceNotTerminating(ctx, h.client, h.moduleConfig, moduleName); err != nil {
		return err
	}

	appName := controller.ApplicationName(moduleName)
	parentONS := ""
	if e := h.getCatalogEntry(ctx, moduleName); e != nil {
		parentONS = h.parentHelmOverviewNamespace(e)
	}
	app := BuildArgoCDApplication(appName, moduleName, h.argocdNamespace, h.argocdProject, h.namespace, h.repoURL, targetRevision, path, h.moduleConfig, parentONS)
	if err := createApplicationIdempotent(ctx, h.client, app); err != nil {
		return err
	}

	// Refresh state and ensure we have an ID (generate if first enable).
	state, err = h.stateStore.Get(ctx)
	if err != nil {
		return err
	}
	mod, _ = h.moduleFromName(ctx, moduleName, state)
	id := mod.ID
	if id == "" {
		b := make([]byte, 4)
		if _, err := rand.Read(b); err != nil {
			return err
		}
		id = "mod-" + hex.EncodeToString(b)
	}
	if state.ModuleIDs == nil {
		state.ModuleIDs = make(map[string]string)
	}
	state.ModuleIDs[moduleName] = id
	if pinned {
		if state.ModuleTargetRevisions == nil {
			state.ModuleTargetRevisions = make(map[string]string)
		}
		state.ModuleTargetRevisions[moduleName] = targetRevision
	} else if state.ModuleTargetRevisions != nil {
		delete(state.ModuleTargetRevisions, moduleName)
		if len(state.ModuleTargetRevisions) == 0 {
			state.ModuleTargetRevisions = nil
		}
	}
	found := false
	for _, eid := range state.EnabledModules {
		if eid == id {
			found = true
			break
		}
	}
	if !found {
		state.EnabledModules = append(state.EnabledModules, id)
	}
	if err := h.stateStore.Update(ctx, state); err != nil {
		return err
	}

	if err := controller.SyncSoftFeaturesForModule(ctx, h.apiReader, h.client, h.argocdNamespace, h.namespace, h.stateStore, h.catalogStore, moduleName); err != nil {
		return err
	}
	return nil
}

func (h *Handler) resolvedOverviewServiceNamespace(entry *controller.CatalogEntry) string {
	if entry == nil {
		return ""
	}
	svc := strings.TrimSpace(entry.OverviewService)
	if svc == "" {
		return ""
	}
	ns := ResolveOverviewServiceNamespace(entry.Name, entry.OverviewServiceNamespace, h.moduleConfig, h.namespace)
	if ns == "" {
		return ""
	}
	return ns
}

func (h *Handler) parentHelmOverviewNamespace(entry *controller.CatalogEntry) string {
	if entry == nil {
		return ""
	}
	svc := strings.TrimSpace(entry.OverviewService)
	if svc == "" {
		return ""
	}
	ns := h.resolvedOverviewServiceNamespace(entry)
	if ns == "" {
		return ""
	}
	return ns
}

func (h *Handler) fillOverviewFromCatalog(mod *ModuleResponse, entry *controller.CatalogEntry) {
	mod.OverviewService = strings.TrimSpace(entry.OverviewService)
	mod.OverviewServiceNamespace = h.resolvedOverviewServiceNamespace(entry)
	if mod.OverviewService == "" || mod.OverviewServiceNamespace == "" {
		mod.OverviewService = ""
		mod.OverviewServiceNamespace = ""
	}
}

func (h *Handler) getCatalogEntry(ctx context.Context, name string) *controller.CatalogEntry {
	entries, err := h.catalogStore.List(ctx)
	if err != nil {
		return nil
	}
	for i := range entries {
		if entries[i].Name == name {
			return &entries[i]
		}
	}
	return nil
}

// patchDependentsSoftFeature recomputes softFeaturesEnabled on each enabled parent that lists enabledModuleName as a soft dependency.
func (h *Handler) patchDependentsSoftFeature(ctx context.Context, enabledModuleName string) {
	if err := controller.ResyncSoftFeaturesForParentsOfSoftDep(ctx, h.apiReader, h.client, h.argocdNamespace, h.namespace, h.stateStore, h.catalogStore, enabledModuleName); err != nil {
		log.Printf("patchDependentsSoftFeature(%q): %v", enabledModuleName, err)
	}
}

func (h *Handler) disableModule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	idOrName := r.PathValue("idOrName")
	if idOrName == "" {
		http.Error(w, "idOrName required", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	controller.ModuleOpsMutex.Lock()
	defer controller.ModuleOpsMutex.Unlock()

	mod, err := h.resolveModule(ctx, idOrName)
	if err != nil || mod == nil {
		http.Error(w, "module not found", http.StatusNotFound)
		return
	}
	if !mod.Enabled {
		writeJSON(w, map[string]string{"status": "already disabled"})
		return
	}

	// Check no enabled module has a hard dependency on this one.
	state, err := h.stateStore.Get(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	deps, err := controller.ListHardDependents(ctx, h.client, h.catalogStore, h.namespace, state, mod.Name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if len(deps) > 0 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		writeJSON(w, map[string]interface{}{
			"error":          "cannot disable: module is a hard dependency",
			"hardDependents": deps,
		})
		return
	}

	// Synchronous disable: state is flipped to disabled inside PerformModuleDisable before parent delete so
	// GET /modules and Portal polling show UNINSTALL IN PROGRESS during long Argo/KCC teardown. We do not
	// return 202 Accepted here: the Portal and API clients expect 200 with status disabled today, and async
	// disable would require releasing ModuleOpsMutex early and idempotent job semantics. workloads-android
	// disable can take many minutes (child Application wait, ComputeInstanceTemplate, ConfigConnectorContext);
	// ensure ingress/proxy timeouts exceed that path or disable via automation that tolerates slow responses.
	if err := controller.DisableModuleAndRefresh(ctx, h.apiReader, h.client, h.stateStore, h.catalogStore, h.argocdNamespace, h.namespace, mod.Name, mod.ID, true, h.moduleConfig); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	writeJSON(w, map[string]string{"status": "disabled"})
}

func (h *Handler) resolveModule(ctx context.Context, idOrName string) (*ModuleResponse, error) {
	state, err := h.stateStore.Get(ctx)
	if err != nil {
		return nil, err
	}
	// By ID
	for name, id := range state.ModuleIDs {
		if id == idOrName {
			mod, err := h.moduleFromName(ctx, name, state)
			if err != nil || mod == nil {
				return mod, err
			}
			if err := h.attachDependents(ctx, mod, state); err != nil {
				return nil, err
			}
			h.attachRevisionInfo(mod, state)
			return mod, nil
		}
	}
	// By name (catalog or registration)
	mod, err := h.moduleFromName(ctx, idOrName, state)
	if err != nil || mod == nil {
		return mod, err
	}
	if err := h.attachDependents(ctx, mod, state); err != nil {
		return nil, err
	}
	h.attachRevisionInfo(mod, state)
	return mod, nil
}

func (h *Handler) attachRevisionInfo(mod *ModuleResponse, state *controller.State) {
	def := strings.TrimSpace(h.targetRevision)
	mod.ClusterTargetRevision = def
	mod.TargetRevision = controller.EffectiveTargetRevision(state, mod.Name, def)
	mod.Pinned = controller.IsModulePinned(state, mod.Name)
}

func (h *Handler) moduleFromName(ctx context.Context, name string, state *controller.State) (*ModuleResponse, error) {
	enabledSet := make(map[string]bool)
	for _, id := range state.EnabledModules {
		enabledSet[id] = true
	}
	id := state.ModuleIDs[name]
	enabled := id != "" && enabledSet[id]
	mod := &ModuleResponse{Name: name, ID: id, Enabled: enabled}
	entry := h.getCatalogEntry(ctx, name)
	if entry == nil && mod.ID == "" {
		return nil, nil
	}
	if entry != nil {
		mod.HardDependencies = append([]string(nil), entry.HardDependencies...)
		mod.SoftDependencies = append([]string(nil), entry.SoftDependencies...)
		mod.AutoDisableWhenUnused = entry.AutoDisableWhenUnused
		mod.OverviewPath = strings.TrimSpace(entry.OverviewPath)
		h.fillOverviewFromCatalog(mod, entry)
	}
	if enabled {
		mod.ApplicationName = controller.ApplicationName(name)
		mod.ApplicationNamespace = h.argocdNamespace
	}
	var catApps []controller.CatalogApplication
	if entry != nil {
		catApps = entry.Applications
	}
	mod.Applications = h.mergedApplicationsForModule(ctx, name, catApps)
	return mod, nil
}

func catalogApplicationsToResponse(in []controller.CatalogApplication) []ModuleApplication {
	if len(in) == 0 {
		return nil
	}
	out := make([]ModuleApplication, 0, len(in))
	for _, a := range in {
		if a.ID == "" || a.URL == "" {
			continue
		}
		out = append(out, ModuleApplication{ID: a.ID, Title: a.Title, URL: a.URL})
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
