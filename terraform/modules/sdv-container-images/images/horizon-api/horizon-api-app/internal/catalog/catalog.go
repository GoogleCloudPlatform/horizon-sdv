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

package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/acn-horizon-sdv/horizon-api/internal/workflow"
)

const (
	labelExpose    = "horizon-sdv.io/expose"
	labelModule    = "horizon-sdv.io/module"
	labelExposeVal = "true"
)

type Entry struct {
	Module       string      `json:"module"`
	TemplateName string      `json:"templateName"`
	Namespace    string      `json:"namespace"`
	Parameters   []Parameter `json:"parameters"`
}

type Parameter struct {
	Name        string `json:"name"`
	Default     string `json:"default,omitempty"`
	Description string `json:"description,omitempty"`
}

// RetentionOpenAPI documents workflow TTL values echoed by list endpoints (mirrors Argo workflowDefaults.ttlStrategy).
type RetentionOpenAPI struct {
	SecondsAfterSuccess    int
	SecondsAfterFailure    int
	SecondsAfterCompletion int
	Explanation            string
}

// Catalog holds exposed WorkflowTemplates and ClusterWorkflowTemplates for enabled modules (rebuilt by the catalog controller).
type Catalog struct {
	mu sync.RWMutex
	// key: module/templateName -> entry (templates unique per module name in MVP)
	entries map[string]Entry

	moduleManagerNS string
	workflowsNS     string
	stateName       string
	catalogName     string
	cli             client.Client
}

func New(moduleManagerNS, workflowsNS, stateName, catalogName string) *Catalog {
	return &Catalog{
		entries:         make(map[string]Entry),
		moduleManagerNS: moduleManagerNS,
		workflowsNS:     workflowsNS,
		stateName:       stateName,
		catalogName:     catalogName,
	}
}

func (c *Catalog) InjectClient(cl client.Client) {
	c.cli = cl
}

// Rebuild refreshes the in-memory catalog from the API server.
func (c *Catalog) Rebuild(ctx context.Context) error {
	if c.cli == nil {
		return fmt.Errorf("catalog: client not injected")
	}
	return c.rebuild(ctx)
}

func (c *Catalog) Snapshot() []Entry {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]Entry, 0, len(c.entries))
	for _, e := range c.entries {
		out = append(out, e)
	}
	return out
}

func (c *Catalog) Get(module, templateName string) (Entry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	e, ok := c.entries[key(module, templateName)]
	return e, ok
}

func key(module, template string) string {
	return module + "/" + template
}

func (c *Catalog) rebuild(ctx context.Context) error {
	enabledModules, err := c.loadEnabledModules(ctx)
	if err != nil {
		return err
	}
	next := make(map[string]Entry)

	uList := &unstructured.UnstructuredList{}
	uList.SetGroupVersionKind(schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "WorkflowTemplateList"})
	if err := c.cli.List(ctx, uList, client.InNamespace(c.workflowsNS)); err != nil {
		return fmt.Errorf("list workflowtemplates: %w", err)
	}
	for i := range uList.Items {
		c.ingestExposedTemplate(next, enabledModules, &uList.Items[i], uList.Items[i].GetNamespace())
	}

	cwList := &unstructured.UnstructuredList{}
	cwList.SetGroupVersionKind(schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "ClusterWorkflowTemplateList"})
	if err := c.cli.List(ctx, cwList); err != nil {
		return fmt.Errorf("list clusterworkflowtemplates: %w", err)
	}
	for i := range cwList.Items {
		// Cluster-scoped: Entry.Namespace is "" (OpenAPI clients use empty string for cluster WorkflowTemplate refs).
		c.ingestExposedTemplate(next, enabledModules, &cwList.Items[i], "")
	}

	c.mu.Lock()
	c.entries = next
	c.mu.Unlock()
	return nil
}

// ingestExposedTemplate adds one catalog entry when labels request exposure and the module is enabled.
// entryNamespace is the WorkflowTemplate namespace; use "" for ClusterWorkflowTemplate (cluster scope).
func (c *Catalog) ingestExposedTemplate(next map[string]Entry, enabledModules map[string]bool, item *unstructured.Unstructured, entryNamespace string) {
	labels := item.GetLabels()
	if labels[labelExpose] != labelExposeVal {
		return
	}
	mod := labels[labelModule]
	if mod == "" || !enabledModules[mod] {
		return
	}
	name := item.GetName()
	params, err := extractParameters(item.Object)
	if err != nil {
		loc := entryNamespace
		if loc == "" {
			loc = "cluster"
		}
		log.Printf("catalog: skip %s/%s: %v", loc, name, err)
		return
	}
	e := Entry{
		Module:       mod,
		TemplateName: name,
		Namespace:    entryNamespace,
		Parameters:   params,
	}
	next[key(mod, name)] = e
}

// loadEnabledModules returns the set of enabled module names, reading ModuleManagerState
// (runtime enabled IDs + module name-to-ID map) and ModuleCatalog (authoritative list of
// known module names). A module is considered enabled when its ModuleCatalog entry has a
// corresponding ID in ModuleManagerState.status.moduleIds and that ID is listed in
// ModuleManagerState.status.enabledModules.
func (c *Catalog) loadEnabledModules(ctx context.Context) (map[string]bool, error) {
	state := &unstructured.Unstructured{}
	state.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "horizon-sdv.io", Version: "v1alpha1", Kind: "ModuleManagerState",
	})
	if err := c.cli.Get(ctx, client.ObjectKey{Namespace: c.moduleManagerNS, Name: c.stateName}, state); err != nil {
		if apierrors.IsNotFound(err) {
			return make(map[string]bool), nil
		}
		return nil, fmt.Errorf("get ModuleManagerState: %w", err)
	}
	enabledIDs := map[string]bool{}
	moduleIDByName := map[string]string{}
	if ids, found, _ := unstructured.NestedStringSlice(state.Object, "status", "enabledModules"); found {
		for _, id := range ids {
			enabledIDs[id] = true
		}
	}
	if m, found, _ := unstructured.NestedStringMap(state.Object, "status", "moduleIds"); found {
		for name, id := range m {
			moduleIDByName[name] = id
		}
	}

	catalogCR := &unstructured.Unstructured{}
	catalogCR.SetGroupVersionKind(schema.GroupVersionKind{
		Group: "horizon-sdv.io", Version: "v1alpha1", Kind: "ModuleCatalog",
	})
	if err := c.cli.Get(ctx, client.ObjectKey{Namespace: c.moduleManagerNS, Name: c.catalogName}, catalogCR); err != nil {
		if apierrors.IsNotFound(err) {
			return make(map[string]bool), nil
		}
		return nil, fmt.Errorf("get ModuleCatalog: %w", err)
	}
	modules, found, err := unstructured.NestedSlice(catalogCR.Object, "spec", "modules")
	if err != nil || !found {
		return make(map[string]bool), nil
	}
	out := make(map[string]bool, len(modules))
	for _, mod := range modules {
		m, ok := mod.(map[string]interface{})
		if !ok {
			continue
		}
		name, _, _ := unstructured.NestedString(m, "name")
		if name == "" {
			continue
		}
		id := moduleIDByName[name]
		if id != "" && enabledIDs[id] {
			out[name] = true
		}
	}
	return out, nil
}

func extractParameters(obj map[string]interface{}) ([]Parameter, error) {
	params, found, err := unstructured.NestedSlice(obj, "spec", "arguments", "parameters")
	if err != nil || !found {
		return nil, nil
	}
	var out []Parameter
	for _, p := range params {
		m, ok := p.(map[string]interface{})
		if !ok {
			continue
		}
		name, _, _ := unstructured.NestedString(m, "name")
		if name == "" {
			continue
		}
		if workflow.IsHorizonInjectedWorkflowParameter(name) {
			continue
		}
		def, _, _ := unstructured.NestedString(m, "value")
		if def == "" {
			def, _, _ = unstructured.NestedString(m, "default")
		}
		desc, _, _ := unstructured.NestedString(m, "description")
		out = append(out, Parameter{Name: name, Default: def, Description: desc})
	}
	return out, nil
}

var opIDSanitizer = regexp.MustCompile(`[^a-zA-Z0-9]+`)

func sanitizeOperationIDPart(s string) string {
	s = opIDSanitizer.ReplaceAllString(s, "_")
	return strings.Trim(s, "_")
}

func submitPathForEntry(module, templateName string) string {
	return "/v1/modules/" + url.PathEscape(module) + "/workflowTemplates/" + url.PathEscape(templateName) + "/submit"
}

func submitRequestBodySchema(e Entry) map[string]interface{} {
	paramProps := make(map[string]interface{})
	var paramRequired []interface{}
	for _, p := range e.Parameters {
		prop := map[string]interface{}{"type": "string"}
		if p.Description != "" {
			prop["description"] = p.Description
		}
		if p.Default != "" {
			prop["default"] = p.Default
		}
		if p.Name == workflow.SubmitParamSubmittedFrom {
			prop["maxLength"] = 63
			suffix := " Optional: omit to use X-Horizon-Submitted-From (default api). Built-in values: api, developer-portal, horizon-cli (aliases: rest-api, portal, cli). Any other value must follow Kubernetes label value rules (e.g. my-ci, string). For OpenAPI-only clients, you may set header X-Horizon-Submitted-From to \"custom\" and send the id in X-Horizon-Submitted-From-Detail instead."
			if d, _ := prop["description"].(string); d != "" {
				prop["description"] = strings.TrimSpace(d) + " " + suffix
			} else {
				prop["description"] = strings.TrimSpace(suffix)
			}
		}
		paramProps[p.Name] = prop
		if p.Default == "" && p.Name != workflow.SubmitParamSubmittedFrom && p.Name != workflow.SubmitParamSubmittedBy {
			paramRequired = append(paramRequired, p.Name)
		}
	}
	inner := map[string]interface{}{
		"type":                 "object",
		"properties":           paramProps,
		"additionalProperties": false,
	}
	if len(paramRequired) > 0 {
		inner["required"] = paramRequired
	}
	root := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"parameters": inner,
		},
	}
	if len(paramRequired) > 0 {
		root["required"] = []interface{}{"parameters"}
	}
	return root
}

func submitOperationParameters() []interface{} {
	return []interface{}{
		map[string]interface{}{
			"name":        "X-Horizon-Submitted-From",
			"in":          "header",
			"required":    false,
			"description": "Where the submit call originated. Built-in: `api` (default), `developer-portal`, `horizon-cli`. `custom`: set `" + workflow.HeaderSubmittedFromDetail + "` to the integration id. Any other single header value is accepted if it is a valid Kubernetes label value (custom integration id). Stored as label horizon-sdv.io/submitted-from on the Workflow.",
			"schema": map[string]interface{}{
				"type":        "string",
				"maxLength":   63,
				"description": "Built-in tokens or a custom integration id; use `custom` with " + workflow.HeaderSubmittedFromDetail + " for arbitrary ids in constrained clients.",
			},
		},
		map[string]interface{}{
			"name":        workflow.HeaderSubmittedFromDetail,
			"in":          "header",
			"required":    false,
			"description": "When `X-Horizon-Submitted-From` is `custom`, required: integration id (Kubernetes label value rules). Ignored otherwise.",
			"schema": map[string]interface{}{
				"type":      "string",
				"maxLength": 63,
			},
		},
	}
}

func submitResponsesDoc() map[string]interface{} {
	return map[string]interface{}{
		"202": map[string]interface{}{"description": "Dispatched to Argo Events"},
		"400": map[string]interface{}{"description": "Bad request"},
		"401": map[string]interface{}{"description": "Unauthorized"},
		"404": map[string]interface{}{"description": "Unknown module/template"},
		"502": map[string]interface{}{"description": "Argo Events webhook error"},
		"503": map[string]interface{}{"description": "Webhook URL not configured"},
	}
}

func openapiComponentSchemas() map[string]interface{} {
	return map[string]interface{}{
		"CatalogParameter": map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name":        map[string]interface{}{"type": "string"},
				"default":     map[string]interface{}{"type": "string"},
				"description": map[string]interface{}{"type": "string"},
			},
			"required": []interface{}{"name"},
		},
		"CatalogEntry": map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"module":       map[string]interface{}{"type": "string"},
				"templateName": map[string]interface{}{"type": "string"},
				"namespace":    map[string]interface{}{"type": "string"},
				"parameters": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"$ref": "#/components/schemas/CatalogParameter",
					},
				},
			},
			"required": []interface{}{"module", "templateName", "namespace", "parameters"},
		},
		"CatalogResponse": map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"entries": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"$ref": "#/components/schemas/CatalogEntry",
					},
				},
			},
			"required": []interface{}{"entries"},
		},
	}
}

func workflowOpenAPIComponentSchemas() map[string]interface{} {
	return map[string]interface{}{
		"WorkflowRetention": map[string]interface{}{
			"type":        "object",
			"description": "Configured Argo workflow TTL (cluster); workflows disappear after pruning — expect 404 for pruned names.",
			"properties": map[string]interface{}{
				"secondsAfterSuccess":    map[string]interface{}{"type": "integer", "format": "int32"},
				"secondsAfterFailure":    map[string]interface{}{"type": "integer", "format": "int32"},
				"secondsAfterCompletion": map[string]interface{}{"type": "integer", "format": "int32"},
				"explanation":            map[string]interface{}{"type": "string"},
			},
			"required": []interface{}{"secondsAfterSuccess", "secondsAfterFailure", "secondsAfterCompletion", "explanation"},
		},
		"LogURIRef": map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"gcsUri": map[string]interface{}{"type": "string", "description": "gs:// bucket/object for archived logs when present"},
			},
		},
		"StepLogLink": map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"nodeId":       map[string]interface{}{"type": "string"},
				"displayName":  map[string]interface{}{"type": "string"},
				"templateName": map[string]interface{}{"type": "string"},
				"phase":        map[string]interface{}{"type": "string"},
				"podName":      map[string]interface{}{"type": "string"},
				"gcsUri":       map[string]interface{}{"type": "string"},
				"artifactName": map[string]interface{}{"type": "string", "description": "Workflow status artifact name for downloadArtifact (e.g. main-logs)."},
			},
			"required": []interface{}{"nodeId"},
		},
		"ArchivedLogLinks": map[string]interface{}{
			"type":        "object",
			"description": "Historical archived log locations from Workflow status (artifact outputs); null or omitted while running or when no log artifacts.",
			"properties": map[string]interface{}{
				"combined": map[string]interface{}{"$ref": "#/components/schemas/LogURIRef"},
				"steps": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"$ref": "#/components/schemas/StepLogLink",
					},
				},
			},
		},
		"WorkflowSummary": map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name":      map[string]interface{}{"type": "string"},
				"namespace": map[string]interface{}{"type": "string"},
				"phase": map[string]interface{}{
					"type":        "string",
					"description": "API phase: Running, Succeeded, Failed, Error, or Aborted (graceful stop via spec.shutdown / Stop or Terminate — Argo may still store Failed for the same run).",
				},
				"startedAt":        map[string]interface{}{"type": "string"},
				"finishedAt":       map[string]interface{}{"type": "string"},
				"workflowTemplate": map[string]interface{}{"type": "string"},
				"startedBy": map[string]interface{}{
					"type":        "string",
					"description": "Horizon portal subject (annotation horizon-sdv.io/submitted-by) or Argo creator label when present.",
				},
				"submittedFrom": map[string]interface{}{
					"type":        "string",
					"maxLength":   63,
					"description": "How the workflow was submitted into the cluster (label horizon-sdv.io/submitted-from): built-in api, developer-portal, horizon-cli, or a custom integration id.",
				},
				"message":      map[string]interface{}{"type": "string"},
				"archivedLogs": map[string]interface{}{"$ref": "#/components/schemas/ArchivedLogLinks"},
			},
			"required": []interface{}{"name", "namespace", "phase"},
		},
		"NodeBrief": map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"id":           map[string]interface{}{"type": "string"},
				"displayName":  map[string]interface{}{"type": "string"},
				"templateName": map[string]interface{}{"type": "string"},
				"type":         map[string]interface{}{"type": "string"},
				"phase":        map[string]interface{}{"type": "string"},
				"podName":      map[string]interface{}{"type": "string"},
				"startedAt":    map[string]interface{}{"type": "string", "description": "RFC3339 from Argo status.nodes[id].startedAt (stable ordering in UIs)."},
			},
			"required": []interface{}{"id"},
		},
		"OutputArtifact": map[string]interface{}{
			"type":        "object",
			"description": "Workflow status output artifact with a gs:// URI when the cluster records GCS (or default bucket) on the artifact.",
			"properties": map[string]interface{}{
				"nodeId":       map[string]interface{}{"type": "string"},
				"name":         map[string]interface{}{"type": "string"},
				"fileName":     map[string]interface{}{"type": "string", "description": "Basename of the GCS object key when it differs from the workflow artifact name (e.g. smoke-result.tgz vs smoke-result)."},
				"displayName":  map[string]interface{}{"type": "string"},
				"templateName": map[string]interface{}{"type": "string"},
				"gcsUri":       map[string]interface{}{"type": "string"},
			},
			"required": []interface{}{"name"},
		},
		"WorkflowDetail": map[string]interface{}{
			"type":        "object",
			"description": "Workflow status detail; archivedLogs populated for terminal workflows when log artifacts exist in status.",
			"allOf": []interface{}{
				map[string]interface{}{"$ref": "#/components/schemas/WorkflowSummary"},
				map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"uid":   map[string]interface{}{"type": "string"},
						"nodes": map[string]interface{}{"type": "array", "items": map[string]interface{}{"$ref": "#/components/schemas/NodeBrief"}},
						"outputArtifacts": map[string]interface{}{
							"type":        "array",
							"items":       map[string]interface{}{"$ref": "#/components/schemas/OutputArtifact"},
							"description": "All node output artifacts with a GCS URI (not limited to log-named artifacts).",
						},
					},
				},
			},
		},
		"WorkflowListResponse": map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"items": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"$ref": "#/components/schemas/WorkflowSummary",
					},
				},
				"continue": map[string]interface{}{
					"type":        "string",
					"description": "Reserved. GET /v1/workflows/running and GET /v1/workflows/history (without filters) walk cluster list pages server-side and always return an empty string here. For filtered history, omit continue (server returns 400 if combined with filters).",
				},
				"retention": map[string]interface{}{"$ref": "#/components/schemas/WorkflowRetention"},
				"truncated": map[string]interface{}{
					"type":        "boolean",
					"description": "Only on filtered history: true if the matching set may extend beyond this response (scan limit or unfetched cluster pages).",
				},
				"scanned": map[string]interface{}{
					"type":        "integer",
					"format":      "int64",
					"description": "Only on filtered history: number of workflow objects examined (including non-terminal) while collecting items.",
				},
			},
			"required": []interface{}{"items", "retention"},
		},
		"WorkflowAbortResponse": map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"status": map[string]interface{}{"type": "string", "example": "aborting"},
			},
			"required": []interface{}{"status"},
		},
		"WorkflowDeleteResponse": map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"status": map[string]interface{}{"type": "string", "example": "deleted"},
			},
			"required": []interface{}{"status"},
		},
		"DownloadArtifactResponse": map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"url":       map[string]interface{}{"type": "string", "description": "HTTPS GET URL; use without Horizon Authorization header."},
				"expiresAt": map[string]interface{}{"type": "string", "format": "date-time", "description": "RFC3339 UTC expiry of the signature."},
				"fileName":  map[string]interface{}{"type": "string", "description": "Suggested local filename (GCS object basename); matches Content-Disposition on the signed URL."},
			},
			"required": []interface{}{"url", "expiresAt"},
		},
		"DownloadArtifactCandidate": map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"nodeId":       map[string]interface{}{"type": "string"},
				"templateName": map[string]interface{}{"type": "string"},
				"name":         map[string]interface{}{"type": "string"},
				"fileName":     map[string]interface{}{"type": "string"},
				"gcsUri":       map[string]interface{}{"type": "string"},
				"displayName":  map[string]interface{}{"type": "string"},
			},
		},
		"DownloadArtifactConflict": map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"error":        map[string]interface{}{"type": "string"},
				"candidates":   map[string]interface{}{"type": "array", "items": map[string]interface{}{"$ref": "#/components/schemas/DownloadArtifactCandidate"}},
				"artifactName": map[string]interface{}{"type": "string"},
			},
		},
	}
}

func mergeSchemaMaps(a, b map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(a)+len(b))
	for k, v := range a {
		out[k] = v
	}
	for k, v := range b {
		out[k] = v
	}
	return out
}

func openapiWorkflowPaths() map[string]interface{} {
	listParams := []interface{}{
		map[string]interface{}{"name": "limit", "in": "query", "schema": map[string]interface{}{"type": "integer", "default": 50, "maximum": 500}},
		map[string]interface{}{"name": "continue", "in": "query", "schema": map[string]interface{}{"type": "string"}},
	}
	listOK := map[string]interface{}{
		"description": "Workflow rows (cluster-local; pruned workflows are absent)",
		"content": map[string]interface{}{
			"application/json": map[string]interface{}{
				"schema": map[string]interface{}{"$ref": "#/components/schemas/WorkflowListResponse"},
			},
		},
	}
	runningParams := []interface{}{
		map[string]interface{}{"name": "limit", "in": "query", "schema": map[string]interface{}{"type": "integer", "default": 50, "maximum": 500}, "description": "Max non-terminal workflows returned; the server may scan up to 10000 cluster workflows (200 per page) so active runs are not hidden behind terminal-only pages."},
	}
	historyParams := append(append([]interface{}(nil), listParams...), []interface{}{
		map[string]interface{}{"name": "phase", "in": "query", "schema": map[string]interface{}{"type": "string"}, "description": "Comma-separated display phases (case-insensitive), e.g. succeeded,failed,aborted. Matches WorkflowSummary.phase."},
		map[string]interface{}{"name": "startedAfter", "in": "query", "schema": map[string]interface{}{"type": "string", "format": "date-time"}, "description": "RFC3339 or RFC3339Nano; workflow status.startedAt must be >= this time."},
		map[string]interface{}{"name": "startedBefore", "in": "query", "schema": map[string]interface{}{"type": "string", "format": "date-time"}, "description": "RFC3339 or RFC3339Nano; status.startedAt must be <= this time."},
		map[string]interface{}{"name": "finishedAfter", "in": "query", "schema": map[string]interface{}{"type": "string", "format": "date-time"}, "description": "RFC3339 or RFC3339Nano; status.finishedAt must be >= this time."},
		map[string]interface{}{"name": "finishedBefore", "in": "query", "schema": map[string]interface{}{"type": "string", "format": "date-time"}, "description": "RFC3339 or RFC3339Nano; status.finishedAt must be <= this time."},
		map[string]interface{}{"name": "nameGlob", "in": "query", "schema": map[string]interface{}{"type": "string"}, "description": "Shell-style glob on metadata.name (Go path/filepath Match), e.g. my-wf-*."},
		map[string]interface{}{"name": "nameRegex", "in": "query", "schema": map[string]interface{}{"type": "string", "maxLength": 256}, "description": "Regular expression on metadata.name (Go regexp syntax; max 256 chars)."},
	}...)
	return map[string]interface{}{
		"/v1/workflows/running": map[string]interface{}{
			"get": map[string]interface{}{
				"tags":        []interface{}{"Workflows"},
				"summary":     "List non-terminal workflows in the workflows namespace",
				"description": "Lists workflows labelled as Horizon-originated (portal, CLI, or API). Paginates cluster List internally so the first page is not only completed runs (which would yield an empty running list).",
				"operationId": "listWorkflowsRunning",
				"parameters":  runningParams,
				"responses":   map[string]interface{}{"200": listOK, "401": map[string]interface{}{"description": "Unauthorized"}, "503": map[string]interface{}{"description": "Workflow store not configured"}},
			},
		},
		"/v1/workflows/history": map[string]interface{}{
			"get": map[string]interface{}{
				"tags":        []interface{}{"Workflows"},
				"summary":     "List terminal workflows (API phases include Aborted) with optional archivedLogs per item",
				"operationId": "listWorkflowsHistory",
				"description": "Without filters, the server walks cluster List pages (200 per request, up to 10000 workflows) until enough terminal rows are collected (same problem as running: a single page can be all non-terminal). With phase / started* / finished* / nameGlob / nameRegex, scans up to 5000 workflows in pages of 100; continue is not supported with filters and truncated+scanned indicate completeness.",
				"parameters":  historyParams,
				"responses": map[string]interface{}{
					"200": listOK,
					"400": map[string]interface{}{"description": "Invalid filter parameters or continue combined with filters"},
					"401": map[string]interface{}{"description": "Unauthorized"},
					"503": map[string]interface{}{"description": "Workflow store not configured"},
				},
			},
		},
		"/v1/workflows/{workflowName}": map[string]interface{}{
			"get": map[string]interface{}{
				"tags":    []interface{}{"Workflows"},
				"summary": "Get workflow status by name (metadata.name in workflows namespace)",
				"parameters": []interface{}{
					map[string]interface{}{"name": "workflowName", "in": "path", "required": true, "schema": map[string]interface{}{"type": "string"}},
				},
				"responses": map[string]interface{}{
					"200": map[string]interface{}{
						"description": "Workflow detail",
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{"$ref": "#/components/schemas/WorkflowDetail"},
							},
						},
					},
					"401": map[string]interface{}{"description": "Unauthorized"},
					"404": map[string]interface{}{"description": "Workflow not found"},
					"503": map[string]interface{}{"description": "Workflow store not configured"},
				},
			},
			"delete": map[string]interface{}{
				"tags":        []interface{}{"Workflows"},
				"operationId": "deleteWorkflow",
				"summary":     "Delete a terminal workflow CR (must be Succeeded, Failed, Error, or Aborted)",
				"description": "Deletes the Workflow and **blocks** until the object is no longer returned by the API (finalizers and artifact cleanup may take minutes). Configure max wait via server flag `--workflow-delete-wait-timeout` or `WORKFLOW_DELETE_WAIT_TIMEOUT`.",
				"parameters": []interface{}{
					map[string]interface{}{"name": "workflowName", "in": "path", "required": true, "schema": map[string]interface{}{"type": "string"}},
				},
				"responses": map[string]interface{}{
					"200": map[string]interface{}{
						"description": "Workflow fully removed from the cluster",
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{"$ref": "#/components/schemas/WorkflowDeleteResponse"},
							},
						},
					},
					"401": map[string]interface{}{"description": "Unauthorized"},
					"404": map[string]interface{}{"description": "Workflow not found"},
					"409": map[string]interface{}{"description": "Workflow is not in a terminal phase"},
					"504": map[string]interface{}{
						"description": "Workflow delete admission succeeded but the CR was still present when the server wait budget expired (finalizers may still be running)",
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"error": map[string]interface{}{"type": "string"},
									},
								},
							},
						},
					},
					"503": map[string]interface{}{"description": "Workflow store not configured"},
				},
			},
		},
		"/v1/workflows/{workflowName}/abort": map[string]interface{}{
			"post": map[string]interface{}{
				"tags":    []interface{}{"Workflows"},
				"summary": "Request graceful shutdown (spec.shutdown Stop) for a running workflow",
				"parameters": []interface{}{
					map[string]interface{}{"name": "workflowName", "in": "path", "required": true, "schema": map[string]interface{}{"type": "string"}},
				},
				"responses": map[string]interface{}{
					"202": map[string]interface{}{
						"description": "Shutdown requested (Accepted)",
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{"$ref": "#/components/schemas/WorkflowAbortResponse"},
							},
						},
					},
					"401": map[string]interface{}{"description": "Unauthorized"},
					"404": map[string]interface{}{"description": "Workflow not found"},
					"409": map[string]interface{}{"description": "Workflow is not running (terminal phase)"},
					"503": map[string]interface{}{"description": "Workflow store not configured"},
				},
			},
		},
		"/v1/workflows/{workflowName}/log": map[string]interface{}{
			"get": map[string]interface{}{
				"tags":        []interface{}{"Workflows"},
				"operationId": "streamWorkflowLogs",
				"summary":     "Stream live workflow logs as one NDJSON stream (combined across steps when podName is omitted)",
				"description": "Without `podName`, the server prefers Argo workflow-level logs; if unavailable, it multiplexes pod streams. " +
					"Log lines may include `nodeId`, `displayName`, and `templateName` when multiplexed. Ordering is best-effort when multiplexed. " +
					"With `podName`, streams that pod only (debug).",
				"parameters": []interface{}{
					map[string]interface{}{"name": "workflowName", "in": "path", "required": true, "schema": map[string]interface{}{"type": "string"}},
					map[string]interface{}{
						"name": "podName", "in": "query", "required": false,
						"description": "Omit for combined workflow log stream; set to restrict to one pod.",
						"schema":      map[string]interface{}{"type": "string"},
					},
					map[string]interface{}{"name": "container", "in": "query", "schema": map[string]interface{}{"type": "string", "default": "main"}},
					map[string]interface{}{"name": "follow", "in": "query", "schema": map[string]interface{}{"type": "boolean", "default": true}},
				},
				"responses": map[string]interface{}{
					"200": map[string]interface{}{
						"description": "Chunked application/x-ndjson; log lines {\"ts\",\"msg\",\"podName\",...}; heartbeat {\"heartbeat\":true}; terminal {\"result\":\"done\",...}",
						"content": map[string]interface{}{
							"application/x-ndjson": map[string]interface{}{"schema": map[string]interface{}{"type": "string"}},
						},
					},
					"400": map[string]interface{}{"description": "Bad request"},
					"401": map[string]interface{}{"description": "Unauthorized"},
					"404": map[string]interface{}{"description": "Workflow not found"},
					"503": map[string]interface{}{"description": "Log streaming not configured"},
				},
			},
		},
		"/v1/workflows/{workflowName}/downloadArtifact/{artifactName}": map[string]interface{}{
			"get": map[string]interface{}{
				"tags":        []interface{}{"Workflows"},
				"operationId": "downloadWorkflowArtifact",
				"summary":     "Signed URL or inlined artifact bytes for a workflow output artifact from GCS",
				"description": "Without `inline`: returns 200 JSON with `url` and `expiresAt` (no HTTP redirect); perform a second GET on `url` without a bearer token. With `inline=true` (`1` / `yes`): returns the artifact body with `Content-Type` from GCS (same auth as JSON mode) — use this from browser apps that cannot follow cross-origin signed URLs due to bucket CORS. When multiple `outputArtifacts` share the same `name`, pass `templateName` and/or `nodeId` from `GET /v1/workflows/{workflowName}`.",
				"parameters": []interface{}{
					map[string]interface{}{"name": "workflowName", "in": "path", "required": true, "schema": map[string]interface{}{"type": "string"}},
					map[string]interface{}{
						"name": "artifactName", "in": "path", "required": true, "schema": map[string]interface{}{"type": "string"},
						"description": "Path segment — use percent-encoding for special characters; matches `outputArtifacts[].name`.",
					},
					map[string]interface{}{
						"name": "durationSeconds", "in": "query", "required": false,
						"description": "Optional signed-URL lifetime in seconds; default 600; server clamps to [60, 43199] (<12h).",
						"schema":      map[string]interface{}{"type": "integer", "minimum": 0},
					},
					map[string]interface{}{
						"name": "templateName", "in": "query", "required": false,
						"description": "When several artifacts share `artifactName`, pass `templateName` from `outputArtifacts[]` (often the friendlier disambiguator).",
						"schema":      map[string]interface{}{"type": "string"},
					},
					map[string]interface{}{
						"name": "nodeId", "in": "query", "required": false,
						"description": "When several artifacts share `artifactName`, pass `nodeId` from `outputArtifacts[]`. May be combined with `templateName` to narrow further.",
						"schema":      map[string]interface{}{"type": "string"},
					},
					map[string]interface{}{
						"name": "inline", "in": "query", "required": false,
						"description": "When `true`/`1`/`yes`, stream the artifact bytes in the response body instead of returning a JSON signed URL.",
						"schema":      map[string]interface{}{"type": "boolean"},
					},
				},
				"responses": map[string]interface{}{
					"200": map[string]interface{}{
						"description": "JSON (`DownloadArtifactResponse`) unless `inline` is set; then raw artifact bytes.",
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{"$ref": "#/components/schemas/DownloadArtifactResponse"},
							},
						},
					},
					"400": map[string]interface{}{"description": "Invalid query or path"},
					"401": map[string]interface{}{"description": "Unauthorized"},
					"404": map[string]interface{}{"description": "Workflow or artifact not found, or artifact has no gs:// URI"},
					"409": map[string]interface{}{
						"description": "Ambiguous artifact name",
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{"$ref": "#/components/schemas/DownloadArtifactConflict"},
							},
						},
					},
					"501": map[string]interface{}{"description": "GCS artifact signing not configured"},
					"503": map[string]interface{}{"description": "Workflow store not configured"},
				},
			},
		},
	}
}

func retentionOpenAPIDescription(meta RetentionOpenAPI) string {
	ss, sf, sc := meta.SecondsAfterSuccess, meta.SecondsAfterFailure, meta.SecondsAfterCompletion
	ex := strings.TrimSpace(meta.Explanation)
	if ss <= 0 {
		ss = 86400
	}
	if sf <= 0 {
		sf = 259200
	}
	if sc <= 0 {
		sc = 86400
	}
	if ex == "" {
		ex = "Workflow CRs are removed from the cluster after Argo ttlStrategy; expect 404 when pruned."
	}
	return " Workflow list APIs include a `retention` object: secondsAfterSuccess=" + strconv.Itoa(ss) +
		", secondsAfterFailure=" + strconv.Itoa(sf) + ", secondsAfterCompletion=" + strconv.Itoa(sc) + ". " + ex
}

// OpenAPISpec builds OpenAPI 3: catalog, workflow lifecycle, live logs, and one POST submit path per catalog entry.
func OpenAPISpec(entries []Entry, meta RetentionOpenAPI) ([]byte, error) {
	paths := map[string]interface{}{
		"/v1/catalog": map[string]interface{}{
			"get": map[string]interface{}{
				"summary": "List exposed workflow templates",
				"responses": map[string]interface{}{
					"200": map[string]interface{}{
						"description": "Catalog entries",
						"content": map[string]interface{}{
							"application/json": map[string]interface{}{
								"schema": map[string]interface{}{"$ref": "#/components/schemas/CatalogResponse"},
							},
						},
					},
				},
			},
		},
	}
	for k, v := range openapiWorkflowPaths() {
		paths[k] = v
	}
	responses := submitResponsesDoc()
	for _, e := range entries {
		pathKey := submitPathForEntry(e.Module, e.TemplateName)
		opID := "submitWorkflow_" + sanitizeOperationIDPart(e.Module) + "_" + sanitizeOperationIDPart(e.TemplateName)
		if opID == "submitWorkflow__" {
			opID = "submitWorkflow"
		}
		paths[pathKey] = map[string]interface{}{
			"post": map[string]interface{}{
				"tags":        []interface{}{e.Module},
				"summary":     fmt.Sprintf("Submit workflow template %s/%s (Argo Events webhook)", e.Module, e.TemplateName),
				"operationId": opID,
				"parameters":  submitOperationParameters(),
				"requestBody": map[string]interface{}{
					"required": true,
					"content": map[string]interface{}{
						"application/json": map[string]interface{}{
							"schema": submitRequestBodySchema(e),
						},
					},
				},
				"responses": responses,
			},
		}
	}
	doc := map[string]interface{}{
		"openapi": "3.0.3",
		"info": map[string]interface{}{
			"title":   "Horizon API",
			"version": "0.3.5",
			"description": "Invoke exposed WorkflowTemplates (and ClusterWorkflowTemplates when labeled exposed) for enabled modules. Authentication: Keycloak realm `horizon` access token (JWT) only. " +
				"Clients: `horizon-api` (public — Authorization Code+PKCE and OAuth 2.0 device authorization grant for CLI), `horizon-api-ci` (confidential — client_credentials with client_id and client_secret from Keycloak Admin). " +
				"Endpoints: `{issuer}/protocol/openid-connect/auth/device` (device), `{issuer}/protocol/openid-connect/token` (code + device token + client_credentials). " +
				"Submit runs a template via `POST /v1/modules/{module}/workflowTemplates/{template}/submit` with body `{\"parameters\":{...}}` and optional header `X-Horizon-Submitted-From` (`api` default, `developer-portal`, `horizon-cli`); OpenAPI lists one such operation per catalog entry with named parameters. " +
				"Workflow lifecycle: `GET /v1/workflows/running`, `GET /v1/workflows/history`, `GET /v1/workflows/{workflowName}`, `DELETE /v1/workflows/{workflowName}` (terminal only), `GET /v1/workflows/{workflowName}/log`, `POST /v1/workflows/{workflowName}/abort`, `GET /v1/workflows/{workflowName}/downloadArtifact/{artifactName}` (JSON signed URL by default; `inline` streams bytes for browsers). " +
				"History and terminal status may include `archivedLogs` with `gs://` URIs derived from Workflow status artifacts. " +
				"Live logs: `GET /v1/workflows/{workflowName}/log` without `podName` for a combined NDJSON stream (Argo workflow log or multiplexed pods); optional `podName` for a single pod. " +
				"Argo Server uses `--auth-mode=client` for the service account." + retentionOpenAPIDescription(meta),
		},
		// Required when the API is exposed behind a path prefix (e.g. GKE Gateway strips /horizon-api/ before the pod). Without this, Swagger UI calls /v1/catalog on the site origin and hits the wrong backend (404 nginx).
		"servers": []interface{}{
			map[string]interface{}{
				"url":         "/horizon-api",
				"description": "External gateway prefix; backend receives paths without this segment.",
			},
		},
		"paths": paths,
		"components": map[string]interface{}{
			"securitySchemes": map[string]interface{}{
				"bearerAuth": map[string]interface{}{
					"type":         "http",
					"scheme":       "bearer",
					"bearerFormat": "JWT",
					"description":  "Keycloak access_token for realm horizon (see info.description).",
				},
			},
			"schemas": mergeSchemaMaps(openapiComponentSchemas(), workflowOpenAPIComponentSchemas()),
		},
		"security": []interface{}{
			map[string]interface{}{"bearerAuth": []interface{}{}},
		},
	}
	return json.MarshalIndent(doc, "", "  ")
}
