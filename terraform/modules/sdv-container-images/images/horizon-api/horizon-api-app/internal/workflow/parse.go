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
//
package workflow

import (
	"sort"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Owner returns the token subject of the caller who submitted the workflow through Horizon API
// (annotation horizon-sdv.io/submitted-by), or "" when none was recorded.
func Owner(u *unstructured.Unstructured) string {
	if u == nil {
		return ""
	}
	return strings.TrimSpace(u.GetAnnotations()[LabelSubmittedBy])
}

// StartedBy returns a name to display for who started the workflow: the submitter's username
// (annotation horizon-sdv.io/submitted-by-name), else the submittedBy workflow parameter, else
// the owner, else Argo UI creator labels when set. Workflows from sensors that do not map the
// username yet (for example a module pinned to an older revision) fall back to the later sources.
func StartedBy(u *unstructured.Unstructured) string {
	if u == nil {
		return ""
	}
	if v := strings.TrimSpace(u.GetAnnotations()[AnnotationSubmittedByName]); v != "" {
		return v
	}
	if v := argumentValue(u, SubmitParamSubmittedBy); v != "" {
		return v
	}
	if v := Owner(u); v != "" {
		return v
	}
	if labels := u.GetLabels(); labels != nil {
		for _, k := range []string{
			"workflows.argoproj.io/creator",
			"workflows.argoproj.io/creator-email",
			"workflows.argoproj.io/creator-preferred-username",
		} {
			if v := strings.TrimSpace(labels[k]); v != "" {
				return v
			}
		}
	}
	return ""
}

// argumentValue returns the value of a workflow argument parameter (spec.arguments.parameters), or "".
func argumentValue(u *unstructured.Unstructured, name string) string {
	raw, _, _ := unstructured.NestedFieldNoCopy(u.Object, "spec", "arguments", "parameters")
	params, _ := raw.([]interface{})
	for _, p := range params {
		m, ok := p.(map[string]interface{})
		if !ok {
			continue
		}
		if n, _ := m["name"].(string); n == name {
			v, _ := m["value"].(string)
			return strings.TrimSpace(v)
		}
	}
	return ""
}

// TerminalPhase reports whether the workflow phase is finished.
func TerminalPhase(phase string) bool {
	switch strings.TrimSpace(phase) {
	case "Succeeded", "Failed", "Error":
		return true
	default:
		return false
	}
}

// Phase returns status.phase (Argo's raw value).
func Phase(u *unstructured.Unstructured) string {
	p, _, _ := unstructured.NestedString(u.Object, "status", "phase")
	return p
}

// DisplayPhaseForAPI returns the phase shown in Horizon JSON APIs. When Argo stops a workflow via
// spec.shutdown (Stop/Terminate), status.phase is often still "Failed"; we surface that as Aborted.
// Some Argo versions use phase "Stopped" — also mapped to Aborted for a stable contract.
func DisplayPhaseForAPI(u *unstructured.Unstructured) string {
	p := strings.TrimSpace(Phase(u))
	if strings.EqualFold(p, "Stopped") {
		return "Aborted"
	}
	if shutdownAbortDetected(u) {
		return "Aborted"
	}
	return p
}

func shutdownAbortDetected(u *unstructured.Unstructured) bool {
	if !TerminalPhase(Phase(u)) {
		return false
	}
	shut, _, _ := unstructured.NestedString(u.Object, "spec", "shutdown")
	switch strings.TrimSpace(shut) {
	case "Stop", "Terminate":
		return true
	}
	msg, _, _ := unstructured.NestedString(u.Object, "status", "message")
	lm := strings.ToLower(msg)
	// Argo: "Stopped with strategy 'Stop': ..." / Terminate
	if strings.Contains(lm, "stopped with strategy") {
		return true
	}
	if strings.Contains(lm, "workflow shutdown with strategy") {
		return true
	}
	return false
}

// DisplayPhaseForNode returns the phase shown in Horizon JSON APIs for a workflow node.
// When the parent workflow was aborted via spec.shutdown, Argo often marks interrupted nodes as Failed;
// remap those to Aborted for consistency with workflow-level DisplayPhaseForAPI.
func DisplayPhaseForNode(nodePhase string, workflowAborted bool, node map[string]interface{}) string {
	phase := strings.TrimSpace(nodePhase)
	if phase == "" || !workflowAborted {
		return nodePhase
	}
	if strings.EqualFold(phase, "Stopped") {
		return "Aborted"
	}
	if !strings.EqualFold(phase, "Failed") && !strings.EqualFold(phase, "Error") {
		return nodePhase
	}
	if nodeInterruptedByShutdown(node) {
		return "Aborted"
	}
	return nodePhase
}

func nodeInterruptedByShutdown(node map[string]interface{}) bool {
	if node == nil {
		return true
	}
	msg, _, _ := unstructured.NestedString(node, "message")
	lm := strings.ToLower(strings.TrimSpace(msg))
	if lm == "" {
		return true
	}
	if strings.Contains(lm, "stopped with strategy") {
		return true
	}
	if strings.Contains(lm, "workflow shutdown") {
		return true
	}
	return false
}

// Summary builds a WorkflowSummary from an unstructured Workflow.
func Summary(u *unstructured.Unstructured, ns, defaultBucket string) WorkflowSummary {
	return summaryWithArchivePods(u, ns, defaultBucket, nil)
}

func summaryWithArchivePods(u *unstructured.Unstructured, ns, defaultBucket string, k8sPodNames []string) WorkflowSummary {
	name := u.GetName()
	started, _, _ := unstructured.NestedString(u.Object, "status", "startedAt")
	finished, _, _ := unstructured.NestedString(u.Object, "status", "finishedAt")
	msg, _, _ := unstructured.NestedString(u.Object, "status", "message")
	tplRef, _, _ := unstructured.NestedString(u.Object, "spec", "workflowTemplateRef", "name")
	if tplRef == "" {
		tplRef, _, _ = unstructured.NestedString(u.Object, "spec", "workflowRef", "name")
	}
	if tplRef == "" {
		tplRef, _, _ = unstructured.NestedString(u.Object, "spec", "clusterWorkflowTemplateRef", "name")
	}
	s := WorkflowSummary{
		Name:             name,
		Namespace:        ns,
		Module:           ModuleLabelValue(u),
		Phase:            DisplayPhaseForAPI(u),
		StartedAt:        started,
		FinishedAt:       finished,
		WorkflowTemplate: tplRef,
		StartedBy:        StartedBy(u),
		Owner:            Owner(u),
		SubmittedFrom:    SubmittedFromLabelValue(u),
		Message:          msg,
	}
	// Expose archived log URIs whenever they appear in status (including running workflows as steps finish).
	// Previously we only set this after terminal phase, which hid GCS / combined links until completion.
	al := BuildArchivedLogLinks(u, ns, defaultBucket, k8sPodNames)
	if al != nil && (al.Combined != nil && al.Combined.GcsURI != "" || len(al.Steps) > 0) {
		s.ArchivedLogs = al
	}
	return s
}

// Detail builds WorkflowDetail. Pass k8sPodNames from label workflows.argoproj.io/workflow when available so empty status.podName is back-filled in nodes and archivedLogs.
func Detail(u *unstructured.Unstructured, ns, defaultBucket string, k8sPodNames []string) WorkflowDetail {
	sum := summaryWithArchivePods(u, ns, defaultBucket, k8sPodNames)
	d := WorkflowDetail{WorkflowSummary: sum, UID: string(u.GetUID())}
	nodes, _, _ := unstructured.NestedMap(u.Object, "status", "nodes")
	parentOf := buildNodeParentMap(nodes)
	dependentTplSet := make(map[string]bool)
	rootTpl := strings.TrimSpace(sum.WorkflowTemplate)
	rootModule := strings.TrimSpace(sum.Module)
	workflowAborted := shutdownAbortDetected(u)
	for id, raw := range nodes {
		m, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		typ, _, _ := unstructured.NestedString(m, "type")
		disp, _, _ := unstructured.NestedString(m, "displayName")
		tpl, _, _ := unstructured.NestedString(m, "templateName")
		originTpl := effectiveWorkflowTemplateForNode(id, m, nodes, parentOf, rootTpl)
		if originTpl != "" && originTpl != rootTpl {
			dependentTplSet[originTpl] = true
		}
		ph, _, _ := unstructured.NestedString(m, "phase")
		pod, _, _ := unstructured.NestedString(m, "podName")
		if pod == "" && len(k8sPodNames) > 0 && typ == "Pod" {
			pod = matchPodNameForNodeID(id, k8sPodNames)
		}
		nStarted, _, _ := unstructured.NestedString(m, "startedAt")
		d.Nodes = append(d.Nodes, NodeBrief{
			ID:           id,
			DisplayName:  disp,
			Module:       rootModule,
			TemplateName: tpl,
			WorkflowTemplate: originTpl,
			Type:         typ,
			Phase:        DisplayPhaseForNode(ph, workflowAborted, m),
			PodName:      pod,
			StartedAt:    nStarted,
		})
	}
	sort.SliceStable(d.Nodes, func(i, j int) bool {
		ti := nodeStartedAt(nodes, d.Nodes[i].ID)
		tj := nodeStartedAt(nodes, d.Nodes[j].ID)
		if ti.Equal(tj) || (ti.IsZero() && tj.IsZero()) {
			return d.Nodes[i].ID < d.Nodes[j].ID
		}
		if ti.IsZero() {
			return false
		}
		if tj.IsZero() {
			return true
		}
		return ti.Before(tj)
	})
	if len(dependentTplSet) > 0 {
		d.DependentWorkflowTemplates = make([]DependentWorkflowTemplate, 0, len(dependentTplSet))
		for name := range dependentTplSet {
			d.DependentWorkflowTemplates = append(d.DependentWorkflowTemplates, DependentWorkflowTemplate{
				Template: name,
				Module:   rootModule,
			})
		}
		sort.SliceStable(d.DependentWorkflowTemplates, func(i, j int) bool {
			return d.DependentWorkflowTemplates[i].Template < d.DependentWorkflowTemplates[j].Template
		})
	}
	if oa := BuildOutputArtifacts(u, defaultBucket, rootTpl, rootModule); len(oa) > 0 {
		d.OutputArtifacts = oa
	}
	return d
}

// PodLogTargets returns pod targets suitable for multiplexed log streaming (Pod-type nodes with a pod name).
func PodLogTargets(u *unstructured.Unstructured) []PodLogTarget {
	return PodLogTargetsFromRunningPods(u, nil)
}

// PodLogTargetsFromRunningPods returns Pod-type log targets.
// When status leaves podName empty (common in some Argo versions), pass k8s Pod names listed with
// label workflows.argoproj.io/workflow=<workflowName>; targets are matched by the node's numeric suffix
// (node id …-3141758486 ↔ pod …-echo-and-artifact-3141758486).
// SortPodLogTargetsByStartedAt orders pod log targets by status.nodes[id].startedAt (workflow order).
// Nodes without a parseable time sort after those with times; ties break on pod name.
func SortPodLogTargetsByStartedAt(u *unstructured.Unstructured, targets []PodLogTarget) {
	nodes, _, _ := unstructured.NestedMap(u.Object, "status", "nodes")
	sort.SliceStable(targets, func(i, j int) bool {
		ti := nodeStartedAt(nodes, targets[i].NodeID)
		tj := nodeStartedAt(nodes, targets[j].NodeID)
		if ti.IsZero() && tj.IsZero() {
			return targets[i].PodName < targets[j].PodName
		}
		if ti.IsZero() {
			return false
		}
		if tj.IsZero() {
			return true
		}
		if !ti.Equal(tj) {
			return ti.Before(tj)
		}
		return targets[i].PodName < targets[j].PodName
	})
}

func nodeStartedAt(nodes map[string]interface{}, nodeID string) time.Time {
	raw, ok := nodes[nodeID]
	if !ok {
		return time.Time{}
	}
	m, ok := raw.(map[string]interface{})
	if !ok {
		return time.Time{}
	}
	s, _, _ := unstructured.NestedString(m, "startedAt")
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

func PodLogTargetsFromRunningPods(u *unstructured.Unstructured, k8sPodNames []string) []PodLogTarget {
	nodes, _, _ := unstructured.NestedMap(u.Object, "status", "nodes")
	var out []PodLogTarget
	for id, raw := range nodes {
		m, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		typ, _, _ := unstructured.NestedString(m, "type")
		if typ != "Pod" {
			continue
		}
		pod, _, _ := unstructured.NestedString(m, "podName")
		if pod == "" && len(k8sPodNames) > 0 {
			pod = matchPodNameForNodeID(id, k8sPodNames)
		}
		if pod == "" {
			continue
		}
		disp, _, _ := unstructured.NestedString(m, "displayName")
		tpl, _, _ := unstructured.NestedString(m, "templateName")
		out = append(out, PodLogTarget{
			PodName:      pod,
			NodeID:       id,
			DisplayName:  disp,
			TemplateName: tpl,
		})
	}
	return out
}

func matchPodNameForNodeID(nodeID string, k8sPodNames []string) string {
	suf := nodeIDNumericSuffix(nodeID)
	if suf == "" {
		return ""
	}
	want := "-" + suf
	for _, name := range k8sPodNames {
		if strings.HasSuffix(name, want) {
			return name
		}
	}
	return ""
}

func nodeIDNumericSuffix(nodeID string) string {
	i := strings.LastIndex(nodeID, "-")
	if i < 0 {
		return ""
	}
	s := nodeID[i+1:]
	if s == "" {
		return ""
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return ""
		}
	}
	return s
}
