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

package controller

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	smv1 "storage-mirror-module/api/v1alpha1"
)

// WorkflowGVK is the Argo Workflow kind.
var WorkflowGVK = schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "Workflow"}

// Run is the part of an Argo Workflow the controller uses.
type Run struct {
	Name        string
	Template    string
	Mirror      string
	RequestID   string
	SubmittedBy string
	Phase       string
	Message     string
	Progress    string
	Created     time.Time
	StartedAt   *metav1.Time
	FinishedAt  *metav1.Time
	Params      map[string]string
}

// Active reports whether the run has not finished.
func (r Run) Active() bool {
	return r.Phase == "" || r.Phase == "Pending" || r.Phase == "Running"
}

// Succeeded reports a successful run.
func (r Run) Succeeded() bool { return r.Phase == "Succeeded" }

// RunFromWorkflow extracts a Run from an unstructured Workflow.
func RunFromWorkflow(u *unstructured.Unstructured) Run {
	labels := u.GetLabels()
	r := Run{
		Name:        u.GetName(),
		Mirror:      labels[smv1.LabelMirror],
		RequestID:   labels[smv1.LabelRequestID],
		Template:    labels["workflows.argoproj.io/workflow-template"],
		SubmittedBy: u.GetAnnotations()["horizon-sdv.io/submitted-by"],
		Created:     u.GetCreationTimestamp().Time,
		Params:      map[string]string{},
	}
	if r.Template == "" {
		r.Template, _, _ = unstructured.NestedString(u.Object, "spec", "workflowTemplateRef", "name")
	}
	r.Phase, _, _ = unstructured.NestedString(u.Object, "status", "phase")
	r.Message, _, _ = unstructured.NestedString(u.Object, "status", "message")
	r.Progress, _, _ = unstructured.NestedString(u.Object, "status", "progress")
	r.StartedAt = nestedTime(u, "status", "startedAt")
	r.FinishedAt = nestedTime(u, "status", "finishedAt")
	params, _, _ := unstructured.NestedSlice(u.Object, "spec", "arguments", "parameters")
	for _, p := range params {
		m, _ := p.(map[string]interface{})
		name, _ := m["name"].(string)
		val, _ := m["value"].(string)
		if name != "" {
			r.Params[name] = val
		}
	}
	if r.Mirror == "" {
		r.Mirror = r.Params["mirror"]
	}
	if r.RequestID == "" {
		r.RequestID = r.Params["requestId"]
	}
	return r
}

func nestedTime(u *unstructured.Unstructured, path ...string) *metav1.Time {
	s, _, _ := unstructured.NestedString(u.Object, path...)
	if s == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil
	}
	mt := metav1.NewTime(t)
	return &mt
}

// ListRuns lists module Workflows, optionally for one mirror, oldest first.
func ListRuns(ctx context.Context, c client.Reader, namespace, mirror string) ([]Run, error) {
	var list unstructured.UnstructuredList
	list.SetGroupVersionKind(schema.GroupVersionKind{Group: WorkflowGVK.Group, Version: WorkflowGVK.Version, Kind: "WorkflowList"})
	sel := client.MatchingLabels{smv1.LabelModule: smv1.ModuleName}
	if mirror != "" {
		sel[smv1.LabelMirror] = mirror
	}
	if err := c.List(ctx, &list, client.InNamespace(namespace), sel); err != nil {
		return nil, err
	}
	out := make([]Run, 0, len(list.Items))
	for i := range list.Items {
		out = append(out, RunFromWorkflow(&list.Items[i]))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Created.Before(out[j].Created) })
	return out, nil
}

// ParseArgoProgress parses Argo's "N/M" node progress.
func ParseArgoProgress(s string) (int64, int64, bool) {
	parts := strings.SplitN(strings.TrimSpace(s), "/", 2)
	if len(parts) != 2 {
		return 0, 0, false
	}
	n, err1 := strconv.ParseInt(parts[0], 10, 64)
	m, err2 := strconv.ParseInt(parts[1], 10, 64)
	if err1 != nil || err2 != nil || m <= 0 {
		return 0, 0, false
	}
	return n, m, true
}
