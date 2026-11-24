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
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestSummaryWorkflowTemplateRef(t *testing.T) {
	u := &unstructured.Unstructured{Object: map[string]interface{}{}}
	_ = unstructured.SetNestedField(u.Object, "my-wft", "spec", "workflowTemplateRef", "name")
	s := Summary(u, "workflows", "")
	if s.WorkflowTemplate != "my-wft" {
		t.Fatalf("workflowTemplateRef: got %q", s.WorkflowTemplate)
	}
}

func TestSummaryOwnerAndStartedBy(t *testing.T) {
	const subject = "c7eae7a2-0420-4942-9a83-492a87401631"
	workflow := func(annotation, submittedBy string, labels map[string]string) *unstructured.Unstructured {
		u := &unstructured.Unstructured{Object: map[string]interface{}{}}
		if annotation != "" {
			u.SetAnnotations(map[string]string{LabelSubmittedBy: annotation})
		}
		u.SetLabels(labels)
		params := []interface{}{map[string]interface{}{"name": "topology", "value": "t1"}}
		if submittedBy != "" {
			params = append(params, map[string]interface{}{"name": SubmitParamSubmittedBy, "value": submittedBy})
		}
		_ = unstructured.SetNestedSlice(u.Object, params, "spec", "arguments", "parameters")
		return u
	}
	creator := map[string]string{"workflows.argoproj.io/creator-preferred-username": "argo-ui-user"}

	tests := []struct {
		name          string
		u             *unstructured.Unstructured
		wantOwner     string
		wantStartedBy string
	}{
		{"template with submittedBy: name from the parameter", workflow(subject, "dev-alice", nil), subject, "dev-alice"},
		{"template without submittedBy: the owner is all there is", workflow(subject, "", nil), subject, subject},
		{"impersonated run: owner stays the service account", workflow("sa-subject", "bob", nil), "sa-subject", "bob"},
		{"owner wins over Argo creator labels", workflow(subject, "", creator), subject, subject},
		{"not submitted through Horizon API: no owner", workflow("", "", creator), "", "argo-ui-user"},
		{"nothing recorded", workflow("", "", nil), "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := Summary(tc.u, "workflows", "")
			if s.Owner != tc.wantOwner {
				t.Errorf("owner = %q, want %q", s.Owner, tc.wantOwner)
			}
			if s.StartedBy != tc.wantStartedBy {
				t.Errorf("startedBy = %q, want %q", s.StartedBy, tc.wantStartedBy)
			}
		})
	}
}

func TestSummaryClusterWorkflowTemplateRef(t *testing.T) {
	u := &unstructured.Unstructured{Object: map[string]interface{}{}}
	_ = unstructured.SetNestedField(u.Object, "my-cwft", "spec", "clusterWorkflowTemplateRef", "name")
	s := Summary(u, "workflows", "")
	if s.WorkflowTemplate != "my-cwft" {
		t.Fatalf("clusterWorkflowTemplateRef: got %q", s.WorkflowTemplate)
	}
}
