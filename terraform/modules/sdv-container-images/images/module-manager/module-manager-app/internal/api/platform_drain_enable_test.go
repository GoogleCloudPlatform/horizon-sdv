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
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/acn-horizon-sdv/module-manager/internal/controller"
)

func TestEnableModule_ConflictWhileRootDeleting(t *testing.T) {
	now := metav1.NewTime(time.Now())
	root := &unstructured.Unstructured{}
	root.SetGroupVersionKind(schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "Application"})
	root.SetNamespace("argocd")
	root.SetName("horizon-sdv")
	root.SetDeletionTimestamp(&now)
	root.SetFinalizers([]string{controller.PlatformDrainFinalizer})

	scheme := runtime.NewScheme()
	scheme.AddKnownTypeWithName(root.GroupVersionKind(), &unstructured.Unstructured{})
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(root).Build()
	h := NewHandler(c, c, nil, nil, "module-manager", "argocd", "horizon-sdv", "https://example.com/repo.git", "main", "", []string{"horizon-sdv"})

	req := httptest.NewRequest(http.MethodPost, "/modules/sample/enable", nil)
	rr := httptest.NewRecorder()
	h.Routes().ServeHTTP(rr, req)
	if rr.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body %s", rr.Code, rr.Body.String())
	}
}
