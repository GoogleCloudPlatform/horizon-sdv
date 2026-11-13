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
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/acn-horizon-sdv/module-manager/internal/controller"
)

const (
	argoCDSkipReconcileAnnotationKey = "argocd.argoproj.io/skip-reconcile"
	cloudWorkstationsModuleName      = "cloud-workstations"
)

func managedChildApplicationPresent(managed []unstructured.Unstructured) bool {
	for i := range managed {
		labels := managed[i].GetLabels()
		if labels == nil {
			continue
		}
		if strings.TrimSpace(labels[labelAppRole]) == labelAppRoleChild {
			return true
		}
	}
	return false
}

func parentSkipReconcile(parent *unstructured.Unstructured) bool {
	if parent == nil {
		return false
	}
	v, found, err := unstructured.NestedString(parent.Object, "metadata", "annotations", argoCDSkipReconcileAnnotationKey)
	if err != nil || !found {
		return false
	}
	return strings.TrimSpace(v) == "true"
}

// fillPrefixedModuleStackStatus sets fields used by the Developer Portal so a healthy parent alone is not
// reported as READY while the child Application is missing (disable teardown or enable still materializing).
func fillPrefixedModuleStackStatus(moduleName string, enabled bool, parentErr error, parent *unstructured.Unstructured, listErr error, managed []unstructured.Unstructured, status *StatusResponse) {
	if !enabled || !controller.ModuleUsesPrefixedChildApplication(moduleName) {
		return
	}
	two := controller.PrefixedModuleExpectedManagedApplicationCount
	status.ExpectedManagedApplicationCount = &two
	present := false
	if listErr == nil {
		present = managedChildApplicationPresent(managed)
	}
	status.ManagedChildApplicationPresent = &present
	if parentErr == nil {
		skip := parentSkipReconcile(parent)
		status.ParentSkipReconcile = &skip
	}
}

// fillCloudWorkstationsDrainStatus reports the paused parent during a cloud-workstations disable drain.
// The drain runs before the state flips, while the paused Applications still report Synced/Healthy.
func fillCloudWorkstationsDrainStatus(moduleName string, enabled bool, parentErr error, parent *unstructured.Unstructured, status *StatusResponse) {
	if !enabled || moduleName != cloudWorkstationsModuleName || parentErr != nil {
		return
	}
	skip := parentSkipReconcile(parent)
	status.ParentSkipReconcile = &skip
}
