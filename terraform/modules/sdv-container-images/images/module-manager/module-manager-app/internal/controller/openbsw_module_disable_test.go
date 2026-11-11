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

package controller

import (
	"context"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestPerformModuleDisable_OpenBSWDeletesParentApplication(t *testing.T) {
	parent := openbswApp("mod-openbsw-module")
	child := openbswApp("mod-openbsw-module-argo-workflows")
	child.SetLabels(map[string]string{
		"horizon-sdv.io/module":                 "openbsw-module",
		"horizon-sdv.io/app-role":               "child",
		"horizon-sdv.io/module-manager-managed": "true",
	})
	c := fake.NewClientBuilder().WithObjects(parent, child).Build()
	store := &cwsFakeStateStore{st: &State{
		EnabledModules: []string{"openbsw-id"},
		ModuleIDs:      map[string]string{"openbsw-module": "openbsw-id"},
	}}

	err := PerformModuleDisable(context.Background(), c, store, nil, "argocd", "module-manager", "openbsw-module", "openbsw-id", false, "")
	if err != nil {
		t.Fatalf("PerformModuleDisable: %v", err)
	}
	if len(store.st.EnabledModules) != 0 {
		t.Fatalf("enabled modules = %v", store.st.EnabledModules)
	}
	assertDeleted(t, c, "mod-openbsw-module")
	// The child is a manifest of the parent chart and carries the Argo CD resources
	// finalizer. Deleting the parent is what prunes it. Module Manager does not
	// delete this child by name (that path is only for prefixed workload modules).
	if _, err := getApp(c, "mod-openbsw-module-argo-workflows"); err != nil {
		t.Fatalf("child Application should remain until the parent finalizer prunes it: %v", err)
	}
}

func openbswApp(name string) *unstructured.Unstructured {
	app := &unstructured.Unstructured{}
	app.SetGroupVersionKind(cwsTestArgoAppGVK)
	app.SetNamespace("argocd")
	app.SetName(name)
	app.SetFinalizers([]string{"resources-finalizer.argocd.argoproj.io"})
	return app
}

func getApp(c client.Client, name string) (*unstructured.Unstructured, error) {
	got := &unstructured.Unstructured{}
	got.SetGroupVersionKind(cwsTestArgoAppGVK)
	err := c.Get(context.Background(), client.ObjectKey{Namespace: "argocd", Name: name}, got)
	return got, err
}

func assertDeleted(t *testing.T, c client.Client, name string) {
	t.Helper()
	got, err := getApp(c, name)
	if apierrors.IsNotFound(err) {
		return
	}
	if err != nil {
		t.Fatalf("get %s: %v", name, err)
	}
	if got.GetDeletionTimestamp().IsZero() {
		t.Fatalf("%s is still present and not deleting", name)
	}
}
