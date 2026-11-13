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
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/acn-horizon-sdv/module-manager/internal/controller"
)

func TestManagedChildApplicationPresent(t *testing.T) {
	parent := unstructured.Unstructured{}
	parent.SetLabels(map[string]string{labelAppRole: labelAppRoleParent})
	child := unstructured.Unstructured{}
	child.SetLabels(map[string]string{labelAppRole: labelAppRoleChild})

	if managedChildApplicationPresent([]unstructured.Unstructured{parent}) {
		t.Fatal("expected false with parent only")
	}
	if !managedChildApplicationPresent([]unstructured.Unstructured{parent, child}) {
		t.Fatal("expected true with parent and child")
	}
}

func TestParentSkipReconcile(t *testing.T) {
	app := &unstructured.Unstructured{}
	app.Object = map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]interface{}{
				argoCDSkipReconcileAnnotationKey: "true",
			},
		},
	}
	if !parentSkipReconcile(app) {
		t.Fatal("expected skip-reconcile true")
	}
	app.Object = map[string]interface{}{"metadata": map[string]interface{}{}}
	if parentSkipReconcile(app) {
		t.Fatal("expected skip-reconcile false")
	}
}

func TestFillPrefixedModuleStackStatus(t *testing.T) {
	status := &StatusResponse{}
	fillPrefixedModuleStackStatus("workloads-android", true, nil, nil, nil, nil, status)
	if status.ExpectedManagedApplicationCount == nil || *status.ExpectedManagedApplicationCount != 2 {
		t.Fatalf("expectedManagedApplicationCount: got %v", status.ExpectedManagedApplicationCount)
	}
	if status.ManagedChildApplicationPresent == nil || *status.ManagedChildApplicationPresent {
		t.Fatalf("managedChildApplicationPresent: got %v", status.ManagedChildApplicationPresent)
	}

	status2 := &StatusResponse{}
	fillPrefixedModuleStackStatus("sample", true, nil, nil, nil, nil, status2)
	if status2.ExpectedManagedApplicationCount != nil {
		t.Fatal("sample module should not set prefixed stack fields")
	}
}

func TestFillCloudWorkstationsDrainStatus(t *testing.T) {
	paused := &unstructured.Unstructured{Object: map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]interface{}{argoCDSkipReconcileAnnotationKey: "true"},
		},
	}}

	status := &StatusResponse{}
	fillCloudWorkstationsDrainStatus(controller.CloudWorkstationsModuleName, true, nil, paused, status)
	if status.ParentSkipReconcile == nil || !*status.ParentSkipReconcile {
		t.Fatalf("parentSkipReconcile: got %v, want true", status.ParentSkipReconcile)
	}
	if status.ExpectedManagedApplicationCount != nil {
		t.Fatal("cloud-workstations must not set prefixed stack fields")
	}

	for name, tc := range map[string]struct {
		module  string
		enabled bool
	}{
		"other module": {module: "sample", enabled: true},
		"disabled":     {module: controller.CloudWorkstationsModuleName, enabled: false},
	} {
		st := &StatusResponse{}
		fillCloudWorkstationsDrainStatus(tc.module, tc.enabled, nil, paused, st)
		if st.ParentSkipReconcile != nil {
			t.Errorf("%s: parentSkipReconcile must be omitted, got %v", name, *st.ParentSkipReconcile)
		}
	}
}
