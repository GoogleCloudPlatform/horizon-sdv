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
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestWorkflowDrainerEmptyNamespaceDone(t *testing.T) {
	t.Parallel()

	c := workflowFakeClient(t)
	drainer := newWorkflowDrainer(c, c)

	done, err := drainer.Drain(context.Background())
	if err != nil {
		t.Fatalf("Drain returned error: %v", err)
	}
	if !done {
		t.Fatal("expected empty workflow namespace to be done")
	}
}

func TestWorkflowDrainerFreshWorkflowDeleted(t *testing.T) {
	t.Parallel()

	wf := workflow("workflows", "sample", []string{"workflows.argoproj.io/artifact-gc"}, nil)
	c := workflowFakeClient(t, wf)
	drainer := newWorkflowDrainer(c, c)

	_, err := drainer.Drain(context.Background())
	if err != nil {
		t.Fatalf("Drain returned error: %v", err)
	}

	got := &unstructured.Unstructured{}
	got.SetGroupVersionKind(workflowGVK)
	err = c.Get(context.Background(), client.ObjectKey{Namespace: "workflows", Name: "sample"}, got)
	if apierrors.IsNotFound(err) {
		return
	}
	if err != nil {
		t.Fatalf("get workflow: %v", err)
	}
	if got.GetDeletionTimestamp() == nil || got.GetDeletionTimestamp().IsZero() {
		t.Fatal("expected fresh Workflow to be deleted or marked for deletion")
	}
}

func TestWorkflowDrainerYoungDeletingWorkflowKeepsFinalizer(t *testing.T) {
	t.Parallel()

	now := metav1.Now()
	wf := workflow("workflows", "sample", []string{"workflows.argoproj.io/artifact-gc"}, &now)
	c := workflowFakeClient(t, wf)
	drainer := newWorkflowDrainer(c, c)
	drainer.GracefulTimeout = time.Hour

	done, err := drainer.Drain(context.Background())
	if err != nil {
		t.Fatalf("Drain returned error: %v", err)
	}
	if done {
		t.Fatal("expected deleting Workflow to require another reconcile")
	}

	got := getWorkflow(t, c, "sample")
	if !hasFinalizerUnstructured(got, "workflows.argoproj.io/artifact-gc") {
		t.Fatal("expected young deleting Workflow finalizer to remain")
	}
}

func TestWorkflowDrainerOldDeletingWorkflowClearsFinalizer(t *testing.T) {
	t.Parallel()

	old := metav1.NewTime(time.Now().Add(-10 * time.Minute))
	wf := workflow("workflows", "sample", []string{"workflows.argoproj.io/artifact-gc"}, &old)
	c := workflowFakeClient(t, wf)
	drainer := newWorkflowDrainer(c, c)
	drainer.GracefulTimeout = time.Minute

	done, err := drainer.Drain(context.Background())
	if err != nil {
		t.Fatalf("Drain returned error: %v", err)
	}
	// Like the API server, the fake client removes a deleting object once its last finalizer is cleared.
	if !done {
		t.Fatal("expected drain to be done once the old Workflow's finalizers are cleared")
	}

	got := &unstructured.Unstructured{}
	got.SetGroupVersionKind(workflowGVK)
	err = c.Get(context.Background(), types.NamespacedName{Namespace: "workflows", Name: "sample"}, got)
	if !apierrors.IsNotFound(err) {
		t.Fatalf("expected Workflow to be removed after its finalizers were cleared, got err=%v finalizers=%#v", err, got.GetFinalizers())
	}
}

func TestWorkflowDrainerMissingCRDDone(t *testing.T) {
	t.Parallel()

	c := workflowFakeClient(t)
	drainer := newWorkflowDrainer(c, noMatchReader{})

	done, err := drainer.Drain(context.Background())
	if err != nil {
		t.Fatalf("Drain returned error: %v", err)
	}
	if !done {
		t.Fatal("expected missing Workflow CRD to be considered done")
	}
}

type noMatchReader struct{}

func (noMatchReader) Get(context.Context, client.ObjectKey, client.Object, ...client.GetOption) error {
	return &meta.NoKindMatchError{GroupKind: workflowGVK.GroupKind(), SearchedVersions: []string{workflowGVK.Version}}
}

func (noMatchReader) List(context.Context, client.ObjectList, ...client.ListOption) error {
	return &meta.NoKindMatchError{GroupKind: workflowGVK.GroupKind(), SearchedVersions: []string{workflowGVK.Version}}
}

func newWorkflowDrainer(c client.Client, reader client.Reader) *WorkflowDrainer {
	return &WorkflowDrainer{
		Client:          c,
		APIReader:       reader,
		WorkflowsNS:     "workflows",
		GracefulTimeout: time.Minute,
		PageSize:        200,
	}
}

func workflowFakeClient(t *testing.T, objs ...client.Object) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	scheme.AddKnownTypeWithName(workflowGVK, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(workflowListGVK, &unstructured.UnstructuredList{})
	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
}

func workflow(namespace, name string, finalizers []string, deletionTimestamp *metav1.Time) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(workflowGVK)
	u.SetNamespace(namespace)
	u.SetName(name)
	u.SetFinalizers(finalizers)
	u.SetDeletionTimestamp(deletionTimestamp)
	return u
}

func getWorkflow(t *testing.T, c client.Client, name string) *unstructured.Unstructured {
	t.Helper()
	got := &unstructured.Unstructured{}
	got.SetGroupVersionKind(workflowGVK)
	if err := c.Get(context.Background(), types.NamespacedName{Namespace: "workflows", Name: name}, got); err != nil {
		t.Fatalf("get workflow: %v", err)
	}
	return got
}
