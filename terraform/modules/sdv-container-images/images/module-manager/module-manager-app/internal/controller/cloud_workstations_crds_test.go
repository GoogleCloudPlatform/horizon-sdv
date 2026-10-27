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
	"strings"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

// The Config Connector CRD is platform-owned; tests assert it is never touched.
const cwsKCCCRDName = "workstations.workstations.cnrm.cloud.google.com"

func cwsCRD(name string, finalizers ...string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(cwsCRDGVK)
	u.SetName(name)
	if len(finalizers) > 0 {
		u.SetFinalizers(finalizers)
	}
	return u
}

func withFastCWSCleanup(t *testing.T, poll, parentWait, crdWait time.Duration) {
	t.Helper()
	withFastCWSDrain(t, poll, time.Second)
	oldParent, oldCRD := cwsParentAppDeleteWait, cwsCRDDeleteWait
	cwsParentAppDeleteWait, cwsCRDDeleteWait = parentWait, crdWait
	t.Cleanup(func() { cwsParentAppDeleteWait, cwsCRDDeleteWait = oldParent, oldCRD })
}

func allHorizonCRDs() []client.Object {
	var out []client.Object
	for _, n := range cloudWorkstationsHorizonCRDNames {
		out = append(out, cwsCRD(n))
	}
	return out
}

func TestRemoveCloudWorkstationsCRDs_ExactlyTheThreeHorizonCRDs(t *testing.T) {
	want := map[string]bool{
		"workstationclusters.workstations.horizon-sdv.io": true,
		"workstationconfigs.workstations.horizon-sdv.io":  true,
		"workstations.workstations.horizon-sdv.io":        true,
	}
	if len(cloudWorkstationsHorizonCRDNames) != len(want) {
		t.Fatalf("got %v", cloudWorkstationsHorizonCRDNames)
	}
	for _, n := range cloudWorkstationsHorizonCRDNames {
		if !want[n] {
			t.Errorf("unexpected CRD name %q", n)
		}
	}
}

func TestRemoveCloudWorkstationsCRDs_NoOpForOtherModules(t *testing.T) {
	c := cwsTestClient(interceptor.Funcs{
		Get: func(context.Context, client.WithWatch, client.ObjectKey, client.Object, ...client.GetOption) error {
			t.Fatal("must not read anything for a module other than cloud-workstations")
			return nil
		},
	}, allHorizonCRDs()...)
	if err := RemoveCloudWorkstationsCRDs(context.Background(), c, "argocd", "workloads-common"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRemoveCloudWorkstationsCRDs_DeletesOnlyHorizonCRDsOnceParentIsGone(t *testing.T) {
	withFastCWSCleanup(t, 5*time.Millisecond, time.Second, time.Second)

	c := cwsTestClient(interceptor.Funcs{}, append(allHorizonCRDs(), cwsCRD(cwsKCCCRDName))...)
	if err := RemoveCloudWorkstationsCRDs(context.Background(), c, "argocd", cloudWorkstationsModuleName); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, n := range cloudWorkstationsHorizonCRDNames {
		if _, ok := cwsExists2(t, c, cwsCRDGVK, "", n); ok {
			t.Errorf("CRD %s should have been deleted", n)
		}
	}
	if _, ok := cwsExists2(t, c, cwsCRDGVK, "", cwsKCCCRDName); !ok {
		t.Error("the Config Connector CRD must never be deleted")
	}
}

func TestRemoveCloudWorkstationsCRDs_AlreadyAbsentIsNoOp(t *testing.T) {
	withFastCWSCleanup(t, 5*time.Millisecond, time.Second, time.Second)

	c := cwsTestClient(interceptor.Funcs{})
	if err := RemoveCloudWorkstationsCRDs(context.Background(), c, "argocd", cloudWorkstationsModuleName); err != nil {
		t.Fatalf("expected no error when the CRDs are already gone, got: %v", err)
	}
}

func TestRemoveCloudWorkstationsCRDs_WaitsForParentApplicationBeforeDeleting(t *testing.T) {
	withFastCWSCleanup(t, 5*time.Millisecond, 5*time.Second, time.Second)

	app := cwsParentApp()
	app.SetFinalizers([]string{cwsTestFinalizer})
	var c client.Client
	crdDeletedWhileAppPresent := false
	c = cwsTestClient(interceptor.Funcs{
		Delete: func(ctx context.Context, cl client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
			if obj.GetObjectKind().GroupVersionKind().Kind == "CustomResourceDefinition" {
				if _, present := cwsExists2(t, cl, cwsTestArgoAppGVK, "argocd", ApplicationName(cloudWorkstationsModuleName)); present {
					crdDeletedWhileAppPresent = true
				}
			}
			return cl.Delete(ctx, obj, opts...)
		},
	}, append(allHorizonCRDs(), app)...)

	// The parent Application (with an Argo finalizer) is marked for deletion and disappears shortly after.
	if err := c.Delete(context.Background(), app.DeepCopy()); err != nil {
		t.Fatalf("mark parent for deletion: %v", err)
	}
	go func() {
		time.Sleep(60 * time.Millisecond)
		u, ok := cwsExists2(t, c, cwsTestArgoAppGVK, "argocd", ApplicationName(cloudWorkstationsModuleName))
		if ok {
			u.SetFinalizers(nil)
			_ = c.Update(context.Background(), u)
		}
	}()

	if err := RemoveCloudWorkstationsCRDs(context.Background(), c, "argocd", cloudWorkstationsModuleName); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if crdDeletedWhileAppPresent {
		t.Error("CRDs were deleted while the parent Application still existed")
	}
	for _, n := range cloudWorkstationsHorizonCRDNames {
		if _, ok := cwsExists2(t, c, cwsCRDGVK, "", n); ok {
			t.Errorf("CRD %s should have been deleted", n)
		}
	}
}

func TestRemoveCloudWorkstationsCRDs_ParentAppTimeoutLeavesCRDsAndGivesManualCommand(t *testing.T) {
	withFastCWSCleanup(t, 5*time.Millisecond, 100*time.Millisecond, time.Second)

	c := cwsTestClient(interceptor.Funcs{}, append(allHorizonCRDs(), cwsParentApp())...)
	err := RemoveCloudWorkstationsCRDs(context.Background(), c, "argocd", cloudWorkstationsModuleName)
	if err == nil {
		t.Fatal("expected a timeout error")
	}
	for _, want := range []string{"timed out", ApplicationName(cloudWorkstationsModuleName), "kubectl delete crd", cloudWorkstationsHorizonCRDNames[0]} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err.Error(), want)
		}
	}
	for _, n := range cloudWorkstationsHorizonCRDNames {
		if crd, ok := cwsExists2(t, c, cwsCRDGVK, "", n); !ok || crd.GetDeletionTimestamp() != nil {
			t.Errorf("CRD %s must be untouched while the parent Application exists", n)
		}
	}
}

func TestRemoveCloudWorkstationsCRDs_StuckCRDTimesOutNamesItAndKeepsFinalizer(t *testing.T) {
	withFastCWSCleanup(t, 5*time.Millisecond, time.Second, 100*time.Millisecond)

	stuck := cloudWorkstationsHorizonCRDNames[1]
	c := cwsTestClient(interceptor.Funcs{}, cwsCRD(cloudWorkstationsHorizonCRDNames[0]), cwsCRD(stuck, cwsTestFinalizer), cwsCRD(cloudWorkstationsHorizonCRDNames[2]))

	err := RemoveCloudWorkstationsCRDs(context.Background(), c, "argocd", cloudWorkstationsModuleName)
	if err == nil {
		t.Fatal("expected a timeout error for the stuck CRD")
	}
	if !strings.Contains(err.Error(), stuck) || !strings.Contains(err.Error(), "finalizers are not stripped") {
		t.Errorf("error %q should name the stuck CRD and say finalizers are not stripped", err.Error())
	}
	crd, ok := cwsExists2(t, c, cwsCRDGVK, "", stuck)
	if !ok {
		t.Fatal("stuck CRD should still exist")
	}
	if got := crd.GetFinalizers(); len(got) != 1 || got[0] != cwsTestFinalizer {
		t.Errorf("finalizers must be left untouched, got %v", got)
	}
}

// A client that disconnects during the long teardown cancels the request context after the state has flipped
// to disabled. The CRD cleanup must still complete, because nothing can re-run it.
func TestRemoveCloudWorkstationsCRDs_CallerCancellationDoesNotAbortCleanup(t *testing.T) {
	withFastCWSCleanup(t, 5*time.Millisecond, 5*time.Second, time.Second)

	app := cwsParentApp()
	app.SetFinalizers([]string{cwsTestFinalizer})
	c := cwsTestClient(interceptor.Funcs{}, append(allHorizonCRDs(), cwsCRD(cwsKCCCRDName), app)...)
	if err := c.Delete(context.Background(), app.DeepCopy()); err != nil {
		t.Fatalf("mark parent for deletion: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(20*time.Millisecond, cancel)
	time.AfterFunc(80*time.Millisecond, func() {
		u, ok := cwsExists2(t, c, cwsTestArgoAppGVK, "argocd", ApplicationName(cloudWorkstationsModuleName))
		if ok {
			u.SetFinalizers(nil)
			_ = c.Update(context.Background(), u)
		}
	})

	if err := RemoveCloudWorkstationsCRDs(ctx, c, "argocd", cloudWorkstationsModuleName); err != nil {
		t.Fatalf("cleanup must survive caller cancellation, got: %v", err)
	}
	for _, n := range cloudWorkstationsHorizonCRDNames {
		if _, ok := cwsExists2(t, c, cwsCRDGVK, "", n); ok {
			t.Errorf("CRD %s should have been deleted despite the cancelled caller context", n)
		}
	}
	if _, ok := cwsExists2(t, c, cwsCRDGVK, "", cwsKCCCRDName); !ok {
		t.Error("the Config Connector CRD must never be deleted")
	}
}

// Detaching from the caller must not make the cleanup unbounded.
func TestRemoveCloudWorkstationsCRDs_DetachedCleanupIsStillBounded(t *testing.T) {
	withFastCWSCleanup(t, 5*time.Millisecond, 100*time.Millisecond, time.Second)

	c := cwsTestClient(interceptor.Funcs{}, append(allHorizonCRDs(), cwsParentApp())...)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := RemoveCloudWorkstationsCRDs(ctx, c, "argocd", cloudWorkstationsModuleName)
	if err == nil || !strings.Contains(err.Error(), "timed out") || !strings.Contains(err.Error(), "kubectl delete crd") {
		t.Fatalf("expected the bounded timeout error with the manual command, got: %v", err)
	}
	for _, n := range cloudWorkstationsHorizonCRDNames {
		if crd, ok := cwsExists2(t, c, cwsCRDGVK, "", n); !ok || crd.GetDeletionTimestamp() != nil {
			t.Errorf("CRD %s must be untouched while the parent Application exists", n)
		}
	}
}
