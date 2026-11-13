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
	"strings"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestEnableOpenBSW_CreatesParentApplication(t *testing.T) {
	t.Parallel()

	c := fake.NewClientBuilder().Build()
	app := BuildArgoCDApplication(
		"mod-openbsw-module",
		"openbsw-module",
		"argocd",
		"horizon-sdv",
		"module-manager",
		"https://example.com/horizon-sdv.git",
		"main",
		"gitops/modules/openbsw-module",
		"isSubEnvironment: false\nnamespacePrefix: \"\"\n",
		"openbsw-overview",
	)
	if err := createApplicationIdempotentWithBackoff(context.Background(), c, app, wait.Backoff{Duration: time.Millisecond, Factor: 1, Steps: 2}); err != nil {
		t.Fatalf("create parent Application: %v", err)
	}

	got := &unstructured.Unstructured{}
	got.SetGroupVersionKind(app.GroupVersionKind())
	if err := c.Get(context.Background(), client.ObjectKey{Namespace: "argocd", Name: "mod-openbsw-module"}, got); err != nil {
		t.Fatalf("get parent Application: %v", err)
	}
	if got.GetLabels()["horizon-sdv.io/app-role"] != "parent" {
		t.Fatalf("app-role = %q", got.GetLabels()["horizon-sdv.io/app-role"])
	}
	if got.GetLabels()["horizon-sdv.io/module-manager-managed"] != "true" {
		t.Fatal("parent Application must be module-manager managed")
	}
	finalizers := got.GetFinalizers()
	if len(finalizers) != 1 || finalizers[0] != "resources-finalizer.argocd.argoproj.io" {
		t.Fatalf("finalizers = %v", finalizers)
	}
	path, _, _ := unstructured.NestedString(got.Object, "spec", "source", "path")
	if path != "gitops/modules/openbsw-module" {
		t.Fatalf("path = %q", path)
	}
	prune, found, err := unstructured.NestedBool(got.Object, "spec", "syncPolicy", "automated", "prune")
	if err != nil || !found || !prune {
		t.Fatalf("parent prune = %v found=%v err=%v", prune, found, err)
	}
	values, _, _ := unstructured.NestedString(got.Object, "spec", "source", "helm", "values")
	if !strings.Contains(values, `overviewNamespace: "openbsw-overview"`) {
		t.Fatalf("helm values missing overview namespace:\n%s", values)
	}
}
