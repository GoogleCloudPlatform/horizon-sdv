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
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/acn-horizon-sdv/horizon-api/internal/gcs"
	"github.com/acn-horizon-sdv/horizon-api/internal/workflow"
	"github.com/acn-horizon-sdv/horizonauth"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// workflowListPageSize is each Kubernetes List chunk; workflowListMaxScan caps total workflows
// read across chunks so one page of mostly TTL-retained terminal runs cannot hide active workflows.
const (
	workflowListPageSize int64 = 200
	workflowListMaxScan  int64 = 10000
)

func workflowRootTemplateName(u *unstructured.Unstructured) string {
	rootTpl, _, _ := unstructured.NestedString(u.Object, "spec", "workflowTemplateRef", "name")
	if rootTpl == "" {
		rootTpl, _, _ = unstructured.NestedString(u.Object, "spec", "workflowRef", "name")
	}
	if rootTpl == "" {
		rootTpl, _, _ = unstructured.NestedString(u.Object, "spec", "clusterWorkflowTemplateRef", "name")
	}
	return strings.TrimSpace(rootTpl)
}

func enrichWorkflowDetailModules(ctx context.Context, store *workflow.Store, d *workflow.WorkflowDetail) {
	if d == nil || store == nil {
		return
	}
	seen := make(map[string]bool)
	var names []string
	add := func(name string) {
		n := strings.TrimSpace(name)
		if n == "" || seen[n] {
			return
		}
		seen[n] = true
		names = append(names, n)
	}
	add(d.WorkflowTemplate)
	for i := range d.Nodes {
		add(d.Nodes[i].WorkflowTemplate)
	}
	for i := range d.OutputArtifacts {
		add(d.OutputArtifacts[i].WorkflowTemplate)
	}
	for i := range d.DependentWorkflowTemplates {
		add(d.DependentWorkflowTemplates[i].Template)
	}
	if len(names) == 0 {
		return
	}
	moduleByTemplate := store.ResolveWorkflowTemplateModules(ctx, names)
	rootModule := strings.TrimSpace(d.Module)
	if rootModule == "" && strings.TrimSpace(d.WorkflowTemplate) != "" {
		rootModule = strings.TrimSpace(moduleByTemplate[strings.TrimSpace(d.WorkflowTemplate)])
		if rootModule != "" {
			d.Module = rootModule
		}
	}
	resolve := func(template, fallback string) string {
		t := strings.TrimSpace(template)
		if t != "" {
			if m := strings.TrimSpace(moduleByTemplate[t]); m != "" {
				return m
			}
		}
		return strings.TrimSpace(fallback)
	}
	for i := range d.Nodes {
		d.Nodes[i].Module = resolve(d.Nodes[i].WorkflowTemplate, d.Nodes[i].Module)
		if d.Nodes[i].Module == "" {
			d.Nodes[i].Module = rootModule
		}
	}
	for i := range d.OutputArtifacts {
		d.OutputArtifacts[i].Module = resolve(d.OutputArtifacts[i].WorkflowTemplate, d.OutputArtifacts[i].Module)
		if d.OutputArtifacts[i].Module == "" {
			d.OutputArtifacts[i].Module = rootModule
		}
	}
	for i := range d.DependentWorkflowTemplates {
		d.DependentWorkflowTemplates[i].Module = resolve(d.DependentWorkflowTemplates[i].Template, d.DependentWorkflowTemplates[i].Module)
	}
}

func (s *Server) retentionPayload() map[string]interface{} {
	r := s.opt.Retention
	return map[string]interface{}{
		"secondsAfterSuccess":    r.SecondsAfterSuccess,
		"secondsAfterFailure":    r.SecondsAfterFailure,
		"secondsAfterCompletion": r.SecondsAfterCompletion,
		"explanation":            r.Explanation,
	}
}

func (s *Server) handleWorkflowsRunning(w http.ResponseWriter, r *http.Request, _ *horizonauth.Principal) {
	if r.Method != http.MethodGet {
		http.NotFound(w, r)
		return
	}
	if s.opt.WorkflowStore == nil {
		http.Error(w, "workflow store not configured", http.StatusServiceUnavailable)
		return
	}
	limit := parseListLimit(r.URL.Query().Get("limit"), 50, 500)
	items, err := s.listRunningSummaries(r.Context(), limit)
	if err != nil {
		http.Error(w, "list workflows failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"items":     items,
		"continue":  "",
		"retention": s.retentionPayload(),
	})
}

func (s *Server) handleWorkflowsHistory(w http.ResponseWriter, r *http.Request, _ *horizonauth.Principal) {
	if r.Method != http.MethodGet {
		http.NotFound(w, r)
		return
	}
	if s.opt.WorkflowStore == nil {
		http.Error(w, "workflow store not configured", http.StatusServiceUnavailable)
		return
	}
	q := r.URL.Query()
	limit := parseListLimit(q.Get("limit"), 50, 500)
	hf, err := parseHistoryFilters(q)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if hf != nil && hf.anyApplied && strings.TrimSpace(q.Get("continue")) != "" {
		http.Error(w, "continue token is not supported together with history filters (phase, time, nameGlob, nameRegex)", http.StatusBadRequest)
		return
	}

	var items []workflow.WorkflowSummary
	var continueToken string
	var truncated bool
	var scanned int64

	if hf != nil && hf.anyApplied {
		items, continueToken, truncated, scanned, err = s.listHistoryFiltered(r.Context(), limit, hf)
		if err != nil {
			http.Error(w, "list workflows failed", http.StatusInternalServerError)
			return
		}
	} else {
		var lerr error
		items, lerr = s.listTerminalSummariesPaged(r.Context(), limit)
		if lerr != nil {
			http.Error(w, "list workflows failed", http.StatusInternalServerError)
			return
		}
		continueToken = ""
	}

	resp := map[string]interface{}{
		"items":     items,
		"continue":  continueToken,
		"retention": s.retentionPayload(),
	}
	if hf != nil && hf.anyApplied {
		resp["truncated"] = truncated
		resp["scanned"] = scanned
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

// listRunningSummaries walks workflow List pages until wantLimit non-terminal workflows are collected
// or workflowListMaxScan items have been inspected (avoids empty /running when the first chunk is all terminal).
func (s *Server) listRunningSummaries(ctx context.Context, wantLimit int64) ([]workflow.WorkflowSummary, error) {
	var out []workflow.WorkflowSummary
	var scanned int64
	continueTok := ""
	for int64(len(out)) < wantLimit && scanned < workflowListMaxScan {
		list, err := s.opt.WorkflowStore.List(ctx, workflowListPageSize, continueTok)
		if err != nil {
			return nil, err
		}
		if len(list.Items) == 0 {
			break
		}
		for i := range list.Items {
			scanned++
			u := &list.Items[i]
			if workflow.TerminalPhase(workflow.Phase(u)) {
				continue
			}
			out = append(out, workflow.Summary(u, s.opt.WorkflowsNS, s.opt.GCSArtifactBucket))
			if int64(len(out)) >= wantLimit {
				break
			}
		}
		continueTok = list.GetContinue()
		if continueTok == "" {
			break
		}
	}
	return out, nil
}

// listTerminalSummariesPaged walks List pages until wantLimit terminal workflows are collected or scan cap hit.
func (s *Server) listTerminalSummariesPaged(ctx context.Context, wantLimit int64) ([]workflow.WorkflowSummary, error) {
	var out []workflow.WorkflowSummary
	var scanned int64
	continueTok := ""
	for int64(len(out)) < wantLimit && scanned < workflowListMaxScan {
		list, err := s.opt.WorkflowStore.List(ctx, workflowListPageSize, continueTok)
		if err != nil {
			return nil, err
		}
		if len(list.Items) == 0 {
			break
		}
		for i := range list.Items {
			scanned++
			u := &list.Items[i]
			if !workflow.TerminalPhase(workflow.Phase(u)) {
				continue
			}
			out = append(out, workflow.Summary(u, s.opt.WorkflowsNS, s.opt.GCSArtifactBucket))
			if int64(len(out)) >= wantLimit {
				break
			}
		}
		continueTok = list.GetContinue()
		if continueTok == "" {
			break
		}
	}
	return out, nil
}

// listHistoryFiltered scans up to historyMaxScan cluster workflows (chunked list) and returns up to wantLimit
// terminal rows matching hf. continue is never returned (pagination across filtered scans is not supported).
func (s *Server) listHistoryFiltered(ctx context.Context, wantLimit int64, hf *historyFilter) ([]workflow.WorkflowSummary, string, bool, int64, error) {
	var items []workflow.WorkflowSummary
	var scanned int64
	truncated := false
	continueTok := ""

	for int64(len(items)) < wantLimit && scanned < historyMaxScan {
		list, err := s.opt.WorkflowStore.List(ctx, historyListChunk, continueTok)
		if err != nil {
			return nil, "", false, scanned, err
		}
		for i := range list.Items {
			if scanned >= historyMaxScan {
				truncated = true
				break
			}
			u := list.Items[i]
			scanned++
			if !workflow.TerminalPhase(workflow.Phase(&u)) {
				continue
			}
			if !hf.matchesTerminal(&u) {
				continue
			}
			items = append(items, workflow.Summary(&u, s.opt.WorkflowsNS, s.opt.GCSArtifactBucket))
			if int64(len(items)) >= wantLimit {
				break
			}
		}
		continueTok = list.GetContinue()
		if int64(len(items)) >= wantLimit {
			if continueTok != "" {
				truncated = true
			}
			break
		}
		if truncated || continueTok == "" {
			break
		}
	}
	return items, "", truncated, scanned, nil
}

func (s *Server) handleWorkflowGet(w http.ResponseWriter, r *http.Request, _ *horizonauth.Principal) {
	if r.Method != http.MethodGet {
		http.NotFound(w, r)
		return
	}
	if s.opt.WorkflowStore == nil {
		http.Error(w, "workflow store not configured", http.StatusServiceUnavailable)
		return
	}
	name := r.PathValue("workflowName")
	if name == "" {
		http.NotFound(w, r)
		return
	}
	u, err := s.opt.WorkflowStore.Get(r.Context(), name)
	if err != nil {
		http.Error(w, "workflow not found", http.StatusNotFound)
		return
	}
	if !workflow.IsHorizonClientVisible(u) {
		http.Error(w, "workflow not found", http.StatusNotFound)
		return
	}
	podNames, _ := s.opt.WorkflowStore.ListPodNamesForWorkflow(r.Context(), name)
	d := workflow.Detail(u, s.opt.WorkflowsNS, s.opt.GCSArtifactBucket, podNames)
	enrichWorkflowDetailModules(r.Context(), s.opt.WorkflowStore, &d)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(d)
}

func (s *Server) handleWorkflowDownloadArtifact(w http.ResponseWriter, r *http.Request, _ *horizonauth.Principal) {
	if r.Method != http.MethodGet {
		http.NotFound(w, r)
		return
	}
	if s.opt.WorkflowStore == nil {
		http.Error(w, "workflow store not configured", http.StatusServiceUnavailable)
		return
	}
	if strings.TrimSpace(s.opt.GCSSigningServiceAccount) == "" {
		http.Error(w, "GCS artifact signing not configured", http.StatusNotImplemented)
		return
	}
	wfName := r.PathValue("workflowName")
	rawArt := r.PathValue("artifactName")
	if wfName == "" || rawArt == "" {
		http.NotFound(w, r)
		return
	}
	artifactName, err := url.PathUnescape(rawArt)
	if err != nil {
		http.Error(w, "invalid artifactName in path", http.StatusBadRequest)
		return
	}
	nodeID := strings.TrimSpace(r.URL.Query().Get("nodeId"))
	templateName := strings.TrimSpace(r.URL.Query().Get("templateName"))
	inline := false
	switch strings.ToLower(strings.TrimSpace(r.URL.Query().Get("inline"))) {
	case "1", "true", "yes":
		inline = true
	}
	ds := strings.TrimSpace(r.URL.Query().Get("durationSeconds"))
	var durSec int
	if ds != "" {
		durSec, err = strconv.Atoi(ds)
		if err != nil || durSec < 0 {
			http.Error(w, "durationSeconds must be a non-negative integer", http.StatusBadRequest)
			return
		}
	}
	durSec = gcs.ClampDurationSeconds(durSec)

	u, err := s.opt.WorkflowStore.Get(r.Context(), wfName)
	if err != nil {
		http.Error(w, "workflow not found", http.StatusNotFound)
		return
	}
	if !workflow.IsHorizonClientVisible(u) {
		http.Error(w, "workflow not found", http.StatusNotFound)
		return
	}
	rootTpl := workflowRootTemplateName(u)
	rootModule := workflow.ModuleLabelValue(u)
	arts := workflow.BuildOutputArtifacts(u, s.opt.GCSArtifactBucket, rootTpl, rootModule)
	matches, err := workflow.MatchOutputArtifacts(arts, artifactName, nodeID, templateName)
	var amb *workflow.AmbiguousArtifactsError
	if errors.As(err, &amb) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		type cand struct {
			NodeID       string `json:"nodeId,omitempty"`
			TemplateName string `json:"templateName,omitempty"`
			Name         string `json:"name"`
			FileName     string `json:"fileName,omitempty"`
			GcsURI       string `json:"gcsUri,omitempty"`
			Display      string `json:"displayName,omitempty"`
		}
		list := make([]cand, 0, len(amb.Candidates))
		for _, m := range amb.Candidates {
			list = append(list, cand{NodeID: m.NodeID, TemplateName: m.TemplateName, Name: m.Name, FileName: m.FileName, GcsURI: m.GcsURI, Display: m.DisplayName})
		}
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error":        "ambiguous artifact name; pass nodeId or templateName query parameter",
			"candidates":   list,
			"artifactName": artifactName,
		})
		return
	}
	if err != nil || len(matches) == 0 {
		http.Error(w, "artifact not found for workflow", http.StatusNotFound)
		return
	}
	one := matches[0]
	bucket, object, err := gcs.ParseGCSURI(one.GcsURI)
	if err != nil {
		http.Error(w, "artifact has no valid gs:// URI", http.StatusNotFound)
		return
	}
	exp := time.Now().UTC().Add(time.Duration(durSec) * time.Second)
	downloadName := path.Base(object)
	if downloadName == "" || downloadName == "." {
		downloadName = artifactName
	}
	signed, err := gcs.SignedGETURL(r.Context(), s.opt.GCSSigningServiceAccount, bucket, object, exp, downloadName)
	if err != nil {
		http.Error(w, fmt.Sprintf("sign url: %v", err), http.StatusInternalServerError)
		return
	}
	if inline {
		req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, signed, nil)
		if err != nil {
			http.Error(w, fmt.Sprintf("build gcs request: %v", err), http.StatusInternalServerError)
			return
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			http.Error(w, fmt.Sprintf("fetch gcs artifact: %v", err), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode < http.StatusOK || resp.StatusCode > 299 {
			slurp, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
			http.Error(w, fmt.Sprintf("gcs GET %d: %s", resp.StatusCode, strings.TrimSpace(string(slurp))), http.StatusBadGateway)
			return
		}
		ct := strings.TrimSpace(resp.Header.Get("Content-Type"))
		if ct == "" {
			ct = "text/plain; charset=utf-8"
		}
		w.Header().Set("Content-Type", ct)
		w.WriteHeader(http.StatusOK)
		_, _ = io.Copy(w, resp.Body)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	jsonResp := map[string]string{
		"url":       signed,
		"expiresAt": exp.Format(time.RFC3339),
		"fileName":  downloadName,
	}
	_ = json.NewEncoder(w).Encode(jsonResp)
}

func (s *Server) handleWorkflowDelete(w http.ResponseWriter, r *http.Request, _ *horizonauth.Principal) {
	if r.Method != http.MethodDelete {
		http.NotFound(w, r)
		return
	}
	if s.opt.WorkflowStore == nil {
		http.Error(w, "workflow store not configured", http.StatusServiceUnavailable)
		return
	}
	name := r.PathValue("workflowName")
	if name == "" {
		http.NotFound(w, r)
		return
	}
	u, err := s.opt.WorkflowStore.Get(r.Context(), name)
	if err != nil {
		http.Error(w, "workflow not found", http.StatusNotFound)
		return
	}
	if !workflow.IsHorizonClientVisible(u) {
		http.Error(w, "workflow not found", http.StatusNotFound)
		return
	}
	if !workflow.TerminalPhase(workflow.Phase(u)) {
		http.Error(w, "workflow must be in a terminal phase (Succeeded, Failed, Error, or Aborted) before it can be deleted", http.StatusConflict)
		return
	}
	waitDur := s.opt.WorkflowDeleteWaitTimeout
	if waitDur <= 0 {
		waitDur = 10 * time.Minute
	}
	poll := 750 * time.Millisecond

	waitCtx, cancel := context.WithTimeout(r.Context(), waitDur)
	defer cancel()

	if err := s.opt.WorkflowStore.Delete(waitCtx, name); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := s.opt.WorkflowStore.WaitUntilWorkflowDeleted(waitCtx, name, poll); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusGatewayTimeout)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"error": "timeout waiting for workflow to be fully removed (finalizers or artifact cleanup may still be running)",
			})
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})
}

func (s *Server) handleWorkflowAbort(w http.ResponseWriter, r *http.Request, p *horizonauth.Principal) {
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	if s.opt.WorkflowStore == nil {
		http.Error(w, "workflow store not configured", http.StatusServiceUnavailable)
		return
	}
	name := r.PathValue("workflowName")
	if name == "" {
		http.NotFound(w, r)
		return
	}
	u, err := s.opt.WorkflowStore.Get(r.Context(), name)
	if err != nil {
		http.Error(w, "workflow not found", http.StatusNotFound)
		return
	}
	if !workflow.IsHorizonClientVisible(u) {
		http.Error(w, "workflow not found", http.StatusNotFound)
		return
	}
	// Decide before looking at the phase: a caller who may not abort this workflow gets the
	// same answer whether or not it is still running.
	if f := s.abortForbidden(p, u); f != nil {
		horizonauth.WriteForbidden(w, f)
		return
	}
	if workflow.TerminalPhase(workflow.Phase(u)) {
		http.Error(w, "workflow is not running", http.StatusConflict)
		return
	}
	if err := s.opt.WorkflowStore.PatchShutdown(r.Context(), name); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "aborting"})
}

func parseListLimit(s string, def, max int64) int64 {
	if strings.TrimSpace(s) == "" {
		return def
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil || n < 1 {
		return def
	}
	if n > max {
		return max
	}
	return n
}
