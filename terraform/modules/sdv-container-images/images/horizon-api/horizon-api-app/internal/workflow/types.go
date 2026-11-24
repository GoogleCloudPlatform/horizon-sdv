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

// Retention describes cluster Workflow TTL (mirrors Argo workflowDefaults.ttlStrategy).
type Retention struct {
	SecondsAfterSuccess    int    `json:"secondsAfterSuccess"`
	SecondsAfterFailure    int    `json:"secondsAfterFailure"`
	SecondsAfterCompletion int    `json:"secondsAfterCompletion"`
	Explanation            string `json:"explanation"`
}

// LogURIRef holds a single GCS URI for archived logs.
type LogURIRef struct {
	GcsURI string `json:"gcsUri,omitempty"`
}

// StepLogLink is one step/node archived log location.
type StepLogLink struct {
	NodeID       string `json:"nodeId"`
	DisplayName  string `json:"displayName,omitempty"`
	TemplateName string `json:"templateName,omitempty"`
	Phase        string `json:"phase,omitempty"`
	PodName      string `json:"podName,omitempty"`
	GcsURI       string `json:"gcsUri,omitempty"`
	// ArtifactName is the Workflow status outputs.artifacts name (e.g. main-logs) for GET .../downloadArtifact.
	ArtifactName string `json:"artifactName,omitempty"`
}

// ArchivedLogLinks combines workflow-level and per-step archived log URIs (gs://), when present in status.
type ArchivedLogLinks struct {
	Combined *LogURIRef    `json:"combined,omitempty"`
	Steps    []StepLogLink `json:"steps,omitempty"`
}

// PodLogTarget identifies a pod for Argo log streaming with optional display metadata.
type PodLogTarget struct {
	PodName      string
	NodeID       string
	DisplayName  string
	TemplateName string
}

// WorkflowSummary is a compact workflow row for list endpoints. Owner is the token subject of the
// caller who submitted the workflow (empty when none was recorded); StartedBy is a name to display.
type WorkflowSummary struct {
	Name             string            `json:"name"`
	Namespace        string            `json:"namespace"`
	Module           string            `json:"module,omitempty"`
	Phase            string            `json:"phase"`
	StartedAt        string            `json:"startedAt,omitempty"`
	FinishedAt       string            `json:"finishedAt,omitempty"`
	WorkflowTemplate string            `json:"workflowTemplate,omitempty"`
	StartedBy        string            `json:"startedBy,omitempty"`
	Owner            string            `json:"owner,omitempty"`
	SubmittedFrom    string            `json:"submittedFrom,omitempty"`
	Message          string            `json:"message,omitempty"`
	ArchivedLogs     *ArchivedLogLinks `json:"archivedLogs,omitempty"`
}

// OutputArtifact is a workflow output artifact with a resolvable GCS URI (when present in status).
type OutputArtifact struct {
	NodeID       string `json:"nodeId,omitempty"`
	Name         string `json:"name"`
	FileName     string `json:"fileName,omitempty"`
	DisplayName  string `json:"displayName,omitempty"`
	Module       string `json:"module,omitempty"`
	TemplateName string `json:"templateName,omitempty"`
	// WorkflowTemplate is the originating WorkflowTemplate for this artifact-producing node.
	WorkflowTemplate string `json:"workflowTemplate,omitempty"`
	GcsURI       string `json:"gcsUri,omitempty"`
}

type DependentWorkflowTemplate struct {
	Template string `json:"template"`
	Module   string `json:"module,omitempty"`
}

// WorkflowDetail extends summary with node overview.
type WorkflowDetail struct {
	WorkflowSummary
	UID                        string                      `json:"uid,omitempty"`
	DependentWorkflowTemplates []DependentWorkflowTemplate `json:"dependentWorkflowTemplates,omitempty"`
	Nodes                      []NodeBrief                 `json:"nodes,omitempty"`
	OutputArtifacts            []OutputArtifact            `json:"outputArtifacts,omitempty"`
}

// NodeBrief is a minimal node summary for status responses.
type NodeBrief struct {
	ID           string `json:"id"`
	DisplayName  string `json:"displayName,omitempty"`
	Module       string `json:"module,omitempty"`
	TemplateName string `json:"templateName,omitempty"`
	// WorkflowTemplate is the originating WorkflowTemplate for this node.
	WorkflowTemplate string `json:"workflowTemplate,omitempty"`
	Type         string `json:"type,omitempty"`
	Phase        string `json:"phase,omitempty"`
	PodName      string `json:"podName,omitempty"`
	StartedAt    string `json:"startedAt,omitempty"`
}
