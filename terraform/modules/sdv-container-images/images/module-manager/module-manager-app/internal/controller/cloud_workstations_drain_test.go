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
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

const cwsTestFinalizer = "workstations.horizon-sdv.io/test-finalizer"

var cwsTestArgoAppGVK = schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "Application"}

func withFastCWSDrain(t *testing.T, poll, stage time.Duration) {
	t.Helper()
	oldPoll, oldStage := cwsDrainPollInterval, cwsDrainStageTimeout
	cwsDrainPollInterval, cwsDrainStageTimeout = poll, stage
	t.Cleanup(func() { cwsDrainPollInterval, cwsDrainStageTimeout = oldPoll, oldStage })
}

func cwsTestScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	gvks := []schema.GroupVersionKind{cwsTestArgoAppGVK, cwsCRDGVK}
	for _, st := range cloudWorkstationsDrainStages {
		gvks = append(gvks, st.horizonGVK, st.kccGVK)
	}
	for _, gvk := range gvks {
		s.AddKnownTypeWithName(gvk, &unstructured.Unstructured{})
		s.AddKnownTypeWithName(schema.GroupVersionKind{Group: gvk.Group, Version: gvk.Version, Kind: gvk.Kind + "List"}, &unstructured.UnstructuredList{})
	}
	return s
}

func cwsTestClient(funcs interceptor.Funcs, objs ...client.Object) client.Client {
	return fake.NewClientBuilder().WithScheme(cwsTestScheme()).WithObjects(objs...).WithInterceptorFuncs(funcs).Build()
}

func cwsObj(gvk schema.GroupVersionKind, name string, finalizers ...string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(gvk)
	u.SetNamespace(cloudWorkstationsNamespace)
	u.SetName(name)
	if len(finalizers) > 0 {
		u.SetFinalizers(finalizers)
	}
	return u
}

func cwsHorizon(kind, name string, finalizers ...string) *unstructured.Unstructured {
	return cwsObj(newCWSDrainStage(kind).horizonGVK, name, finalizers...)
}

func cwsKCC(kind, name string, finalizers ...string) *unstructured.Unstructured {
	return cwsObj(newCWSDrainStage(kind).kccGVK, name, finalizers...)
}

func cwsExists(t *testing.T, c client.Client, gvk schema.GroupVersionKind, name string) (*unstructured.Unstructured, bool) {
	t.Helper()
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(gvk)
	err := c.Get(context.Background(), client.ObjectKey{Namespace: cloudWorkstationsNamespace, Name: name}, u)
	if errors.IsNotFound(err) {
		return nil, false
	}
	if err != nil {
		t.Fatalf("get %s/%s: %v", gvk.Kind, name, err)
	}
	return u, true
}

func TestDrainCloudWorkstations_NoOpForOtherModules(t *testing.T) {
	c := cwsTestClient(interceptor.Funcs{
		List: func(context.Context, client.WithWatch, client.ObjectList, ...client.ListOption) error {
			t.Fatal("must not list anything for a module other than cloud-workstations")
			return nil
		},
	})
	if err := DrainCloudWorkstationsModule(context.Background(), c, "workloads-common"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDrainCloudWorkstations_CRDsNotInstalledIsNoOp(t *testing.T) {
	c := cwsTestClient(interceptor.Funcs{
		List: func(_ context.Context, _ client.WithWatch, list client.ObjectList, _ ...client.ListOption) error {
			gvk := list.GetObjectKind().GroupVersionKind()
			return &meta.NoKindMatchError{GroupKind: gvk.GroupKind(), SearchedVersions: []string{gvk.Version}}
		},
	})
	if err := DrainCloudWorkstationsModule(context.Background(), c, CloudWorkstationsModuleName); err != nil {
		t.Fatalf("expected no-op when CRDs are absent, got: %v", err)
	}
}

func TestDrainCloudWorkstations_DeletesChildFirstAndSucceeds(t *testing.T) {
	withFastCWSDrain(t, 5*time.Millisecond, 2*time.Second)

	var mu sync.Mutex
	var order []string
	c := cwsTestClient(interceptor.Funcs{
		Delete: func(ctx context.Context, cl client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
			mu.Lock()
			order = append(order, obj.GetObjectKind().GroupVersionKind().Kind)
			mu.Unlock()
			return cl.Delete(ctx, obj, opts...)
		},
	},
		cwsHorizon("WorkstationCluster", "cl"),
		cwsHorizon("WorkstationConfig", "cfg"),
		cwsHorizon("Workstation", "ws"),
	)

	if err := DrainCloudWorkstationsModule(context.Background(), c, CloudWorkstationsModuleName); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"Workstation", "WorkstationConfig", "WorkstationCluster"}
	if strings.Join(order, ",") != strings.Join(want, ",") {
		t.Fatalf("delete order = %v, want %v", order, want)
	}
}

// The config must not be deleted until the workstation (with a finalizer that only the controller would
// release) is completely gone, and likewise the cluster after the config.
func TestDrainCloudWorkstations_WaitsForStageBeforeNextStage(t *testing.T) {
	withFastCWSDrain(t, 5*time.Millisecond, 5*time.Second)

	var c client.Client
	var violations []string
	var mu sync.Mutex
	c = cwsTestClient(interceptor.Funcs{
		Delete: func(ctx context.Context, cl client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
			kind := obj.GetObjectKind().GroupVersionKind().Kind
			var childKind, childName string
			switch kind {
			case "WorkstationConfig":
				childKind, childName = "Workstation", "ws"
			case "WorkstationCluster":
				childKind, childName = "WorkstationConfig", "cfg"
			}
			if childKind != "" {
				if _, present := cwsExists(t, cl, newCWSDrainStage(childKind).horizonGVK, childName); present {
					mu.Lock()
					violations = append(violations, kind+" deleted while "+childKind+" still exists")
					mu.Unlock()
				}
			}
			return cl.Delete(ctx, obj, opts...)
		},
	},
		cwsHorizon("Workstation", "ws", cwsTestFinalizer),
		cwsHorizon("WorkstationConfig", "cfg", cwsTestFinalizer),
		cwsHorizon("WorkstationCluster", "cl"),
	)

	// Simulate the controller: release each finalizer a little after the object is marked for deletion.
	done := make(chan struct{})
	go func() {
		defer close(done)
		for _, kind := range []string{"Workstation", "WorkstationConfig"} {
			gvk := newCWSDrainStage(kind).horizonGVK
			name := map[string]string{"Workstation": "ws", "WorkstationConfig": "cfg"}[kind]
			for i := 0; i < 400; i++ {
				u, ok := cwsExists(t, c, gvk, name)
				if ok && u.GetDeletionTimestamp() != nil {
					time.Sleep(40 * time.Millisecond)
					u.SetFinalizers(nil)
					if err := c.Update(context.Background(), u); err != nil {
						t.Errorf("release finalizer on %s: %v", kind, err)
					}
					break
				}
				time.Sleep(10 * time.Millisecond)
			}
		}
	}()

	if err := DrainCloudWorkstationsModule(context.Background(), c, CloudWorkstationsModuleName); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	<-done
	if len(violations) > 0 {
		t.Fatalf("ordering violated: %v", violations)
	}
}

func TestDrainCloudWorkstations_TimeoutNamesBlockerAndKeepsFinalizersAndParents(t *testing.T) {
	withFastCWSDrain(t, 5*time.Millisecond, 100*time.Millisecond)

	c := cwsTestClient(interceptor.Funcs{},
		cwsHorizon("Workstation", "stuck-ws", cwsTestFinalizer),
		cwsHorizon("WorkstationConfig", "cfg"),
		cwsHorizon("WorkstationCluster", "cl"),
	)

	err := DrainCloudWorkstationsModule(context.Background(), c, CloudWorkstationsModuleName)
	if err == nil {
		t.Fatal("expected a timeout error")
	}
	for _, want := range []string{"timed out", "Workstation/stuck-ws", cloudWorkstationsNamespace} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err.Error(), want)
		}
	}

	ws, ok := cwsExists(t, c, newCWSDrainStage("Workstation").horizonGVK, "stuck-ws")
	if !ok {
		t.Fatal("stuck workstation should still exist")
	}
	if ws.GetDeletionTimestamp() == nil {
		t.Error("stuck workstation should have been deleted (marked for deletion)")
	}
	if got := ws.GetFinalizers(); len(got) != 1 || got[0] != cwsTestFinalizer {
		t.Errorf("finalizers must be left untouched, got %v", got)
	}
	for _, kind := range []string{"WorkstationConfig", "WorkstationCluster"} {
		name := map[string]string{"WorkstationConfig": "cfg", "WorkstationCluster": "cl"}[kind]
		u, ok := cwsExists(t, c, newCWSDrainStage(kind).horizonGVK, name)
		if !ok || u.GetDeletionTimestamp() != nil {
			t.Errorf("%s must not be deleted while a Workstation blocks the drain (exists=%v)", kind, ok)
		}
	}
}

func TestDrainCloudWorkstations_LeftoverKCCObjectBlocksAndIsNamed(t *testing.T) {
	withFastCWSDrain(t, 5*time.Millisecond, 100*time.Millisecond)

	c := cwsTestClient(interceptor.Funcs{}, cwsKCC("Workstation", "orphan-ws"))

	err := DrainCloudWorkstationsModule(context.Background(), c, CloudWorkstationsModuleName)
	if err == nil {
		t.Fatal("expected a timeout error while a KCC Workstation remains")
	}
	if !strings.Contains(err.Error(), "KCC Workstation/orphan-ws") {
		t.Errorf("error %q should name the KCC blocker", err.Error())
	}
}

func TestDrainCloudWorkstations_ContextCancelStopsWaiting(t *testing.T) {
	withFastCWSDrain(t, 5*time.Millisecond, time.Minute)

	c := cwsTestClient(interceptor.Funcs{}, cwsHorizon("Workstation", "stuck-ws", cwsTestFinalizer))
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	if err := DrainCloudWorkstationsModule(ctx, c, CloudWorkstationsModuleName); err != context.DeadlineExceeded {
		t.Fatalf("expected context.DeadlineExceeded, got %v", err)
	}
}

type cwsFakeStateStore struct {
	mu      sync.Mutex
	st      *State
	updates int
}

func (f *cwsFakeStateStore) Get(context.Context) (*State, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := *f.st
	cp.EnabledModules = append([]string(nil), f.st.EnabledModules...)
	return &cp, nil
}

func (f *cwsFakeStateStore) Update(_ context.Context, st *State) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.st = st
	f.updates++
	return nil
}

func (f *cwsFakeStateStore) InvalidateCache() {}

func cwsParentApp() *unstructured.Unstructured {
	app := &unstructured.Unstructured{}
	app.SetGroupVersionKind(cwsTestArgoAppGVK)
	app.SetNamespace("argocd")
	app.SetName(ApplicationName(CloudWorkstationsModuleName))
	return app
}

func cwsCoreApp() *unstructured.Unstructured {
	app := &unstructured.Unstructured{}
	app.SetGroupVersionKind(cwsTestArgoAppGVK)
	app.SetNamespace("argocd")
	app.SetName(cloudWorkstationsCoreAppName)
	app.Object["spec"] = map[string]interface{}{
		"syncPolicy": map[string]interface{}{
			"automated": map[string]interface{}{
				"prune":    true,
				"selfHeal": true,
			},
		},
	}
	return app
}

func TestPerformModuleDisable_CloudWorkstationsDrainFailureLeavesModuleEnabled(t *testing.T) {
	withFastCWSDrain(t, 5*time.Millisecond, 100*time.Millisecond)

	c := cwsTestClient(interceptor.Funcs{}, cwsParentApp(), cwsCoreApp(), cwsHorizon("Workstation", "stuck-ws", cwsTestFinalizer), cwsCRD(cloudWorkstationsHorizonCRDNames[2]))
	store := &cwsFakeStateStore{st: &State{
		EnabledModules: []string{"mod-cws"},
		ModuleIDs:      map[string]string{CloudWorkstationsModuleName: "mod-cws"},
	}}

	err := PerformModuleDisable(context.Background(), c, store, nil, "argocd", "module-manager", CloudWorkstationsModuleName, "mod-cws", false, "")
	if err == nil || !strings.Contains(err.Error(), "Workstation/stuck-ws") {
		t.Fatalf("expected drain error naming the blocking workstation, got %v", err)
	}
	if store.updates != 0 || len(store.st.EnabledModules) != 1 {
		t.Errorf("state must be untouched on drain failure (updates=%d, enabled=%v)", store.updates, store.st.EnabledModules)
	}
	if _, ok := cwsExists2(t, c, cwsTestArgoAppGVK, "argocd", ApplicationName(CloudWorkstationsModuleName)); !ok {
		t.Error("parent Application must not be deleted when the drain fails")
	}
	core, ok := cwsExists2(t, c, cwsTestArgoAppGVK, "argocd", cloudWorkstationsCoreAppName)
	if !ok {
		t.Fatal("core Application must remain when the drain fails")
	}
	if _, found := core.GetAnnotations()[argoCDSkipReconcileAnnotationKey]; found {
		t.Errorf("core Application skip-reconcile must be cleared after a drain failure, got annotations %v", core.GetAnnotations())
	}
	if parent, _ := cwsExists2(t, c, cwsTestArgoAppGVK, "argocd", ApplicationName(CloudWorkstationsModuleName)); parent != nil {
		if _, found := parent.GetAnnotations()[argoCDSkipReconcileAnnotationKey]; found {
			t.Errorf("parent Application skip-reconcile must be cleared after a drain failure, got annotations %v", parent.GetAnnotations())
		}
	}
	if selfHeal, found, err := unstructured.NestedBool(core.Object, "spec", "syncPolicy", "automated", "selfHeal"); err != nil || !found || !selfHeal {
		t.Errorf("core Application selfHeal must stay true after a failed drain (selfHeal=%v, found=%v, err=%v)", selfHeal, found, err)
	}
	if crd, ok := cwsExists2(t, c, cwsCRDGVK, "", cloudWorkstationsHorizonCRDNames[2]); !ok || crd.GetDeletionTimestamp() != nil {
		t.Error("CRDs must not be touched when the drain fails")
	}
}

const cwsTestArgoFinalizer = "resources-finalizer.argocd.argoproj.io"

func cwsSkipReconcile(app *unstructured.Unstructured) bool {
	return app.GetAnnotations()[argoCDSkipReconcileAnnotationKey] == "true"
}

func cwsAssertResumedAndSelfHealIntact(t *testing.T, c client.Client, ns string) {
	t.Helper()
	for _, name := range []string{ApplicationName(CloudWorkstationsModuleName), cloudWorkstationsCoreAppName} {
		app, ok := cwsExists2(t, c, cwsTestArgoAppGVK, ns, name)
		if !ok {
			t.Fatalf("%s must remain", name)
		}
		if _, found := app.GetAnnotations()[argoCDSkipReconcileAnnotationKey]; found {
			t.Errorf("%s skip-reconcile must be cleared, got annotations %v", name, app.GetAnnotations())
		}
	}
	core, _ := cwsExists2(t, c, cwsTestArgoAppGVK, ns, cloudWorkstationsCoreAppName)
	if selfHeal, found, err := unstructured.NestedBool(core.Object, "spec", "syncPolicy", "automated", "selfHeal"); err != nil || !found || !selfHeal {
		t.Errorf("core Application selfHeal must stay true (selfHeal=%v, found=%v, err=%v)", selfHeal, found, err)
	}
}

func cwsFailCoreDelete() interceptor.Funcs {
	return interceptor.Funcs{
		Delete: func(ctx context.Context, cl client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
			if obj.GetName() == cloudWorkstationsCoreAppName {
				return fmt.Errorf("injected core Application delete failure")
			}
			return cl.Delete(ctx, obj, opts...)
		},
	}
}

func cwsDrainedFixtures() []client.Object {
	return []client.Object{
		cwsParentApp(), cwsCoreApp(), cwsHorizon("Workstation", "ws"), cwsHorizon("WorkstationConfig", "cfg"), cwsHorizon("WorkstationCluster", "cl"),
		cwsCRD(cloudWorkstationsHorizonCRDNames[0]), cwsCRD(cloudWorkstationsHorizonCRDNames[1]), cwsCRD(cloudWorkstationsHorizonCRDNames[2]), cwsCRD(cwsKCCCRDName),
	}
}

func TestPerformModuleDisable_CoreDeleteFailureResumesApplications(t *testing.T) {
	withFastCWSDrain(t, 5*time.Millisecond, 2*time.Second)

	c := cwsTestClient(cwsFailCoreDelete(), cwsDrainedFixtures()...)
	store := &cwsFakeStateStore{st: &State{
		EnabledModules: []string{"mod-cws"},
		ModuleIDs:      map[string]string{CloudWorkstationsModuleName: "mod-cws"},
	}}

	err := PerformModuleDisable(context.Background(), c, store, nil, "argocd", "module-manager", CloudWorkstationsModuleName, "mod-cws", false, "")
	if err == nil || !strings.Contains(err.Error(), "injected core Application delete failure") {
		t.Fatalf("expected the injected core delete failure, got %v", err)
	}
	if store.updates != 0 || len(store.st.EnabledModules) != 1 {
		t.Errorf("state must be untouched when the core delete fails (updates=%d, enabled=%v)", store.updates, store.st.EnabledModules)
	}
	cwsAssertResumedAndSelfHealIntact(t, c, "argocd")
}

func TestPerformModuleDisable_ParentDeleteFailureLeavesParentPaused(t *testing.T) {
	withFastCWSDrain(t, 5*time.Millisecond, 2*time.Second)

	parentName := ApplicationName(CloudWorkstationsModuleName)
	c := cwsTestClient(interceptor.Funcs{
		Delete: func(ctx context.Context, cl client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
			if obj.GetName() == parentName {
				return fmt.Errorf("injected parent Application delete failure")
			}
			return cl.Delete(ctx, obj, opts...)
		},
	}, cwsDrainedFixtures()...)
	store := &cwsFakeStateStore{st: &State{
		EnabledModules: []string{"mod-cws"},
		ModuleIDs:      map[string]string{CloudWorkstationsModuleName: "mod-cws"},
	}}

	err := PerformModuleDisable(context.Background(), c, store, nil, "argocd", "module-manager", CloudWorkstationsModuleName, "mod-cws", false, "")
	if err == nil || !strings.Contains(err.Error(), "injected parent Application delete failure") {
		t.Fatalf("expected the injected parent delete failure, got %v", err)
	}
	if store.updates != 1 || len(store.st.EnabledModules) != 0 {
		t.Errorf("state must record disabled before the parent delete (updates=%d, enabled=%v)", store.updates, store.st.EnabledModules)
	}
	parent, ok := cwsExists2(t, c, cwsTestArgoAppGVK, "argocd", parentName)
	if !ok {
		t.Fatal("parent Application must remain")
	}
	if !cwsSkipReconcile(parent) {
		t.Errorf("parent Application must stay paused, annotations %v", parent.GetAnnotations())
	}
	if parent.GetDeletionTimestamp() != nil {
		t.Error("parent Application delete must not have been recorded")
	}
	if _, ok := cwsExists2(t, c, cwsTestArgoAppGVK, "argocd", cloudWorkstationsCoreAppName); ok {
		t.Error("core Application should already have been deleted")
	}
}

func TestPlatformDrainer_DisableOne_CoreDeleteFailureResumesApplications(t *testing.T) {
	withFastCWSDrain(t, 5*time.Millisecond, 2*time.Second)

	c := cwsTestClient(cwsFailCoreDelete(), cwsDrainedFixtures()...)
	d, _ := testDrainer(c, &State{
		EnabledModules: []string{"mod-cws"},
		ModuleIDs:      map[string]string{CloudWorkstationsModuleName: "mod-cws"},
	}, []CatalogEntry{{Name: CloudWorkstationsModuleName}})

	err := d.disableOne(context.Background(), CloudWorkstationsModuleName, false)
	if err == nil || !strings.Contains(err.Error(), "injected core Application delete failure") {
		t.Fatalf("expected the injected core delete failure, got %v", err)
	}
	cwsAssertResumedAndSelfHealIntact(t, c, testArgoNS)
}

// cwsArgoSimClient behaves like Argo CD for the parent and core Applications: their finalizer only runs
// once the Application is deleted and no longer paused. It records every ordering violation.
func cwsArgoSimClient(t *testing.T, ns string, violations *[]string) client.Client {
	t.Helper()
	var mu sync.Mutex
	violate := func(msg string) {
		mu.Lock()
		*violations = append(*violations, msg)
		mu.Unlock()
	}
	getApp := func(ctx context.Context, cl client.WithWatch, name string) (*unstructured.Unstructured, bool) {
		u := &unstructured.Unstructured{}
		u.SetGroupVersionKind(cwsTestArgoAppGVK)
		if err := cl.Get(ctx, client.ObjectKey{Namespace: ns, Name: name}, u); err != nil {
			return nil, false
		}
		return u, true
	}
	parentName := ApplicationName(CloudWorkstationsModuleName)

	parent := cwsParentApp()
	parent.SetNamespace(ns)
	parent.SetFinalizers([]string{cwsTestArgoFinalizer})
	core := cwsCoreApp()
	core.SetNamespace(ns)
	core.SetFinalizers([]string{cwsTestArgoFinalizer})

	return cwsTestClient(interceptor.Funcs{
		Delete: func(ctx context.Context, cl client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
			p, pOK := getApp(ctx, cl, parentName)
			cr, crOK := getApp(ctx, cl, cloudWorkstationsCoreAppName)
			switch {
			case obj.GetObjectKind().GroupVersionKind().Kind == "WorkstationCluster":
				if !pOK || !cwsSkipReconcile(p) || !crOK || !cwsSkipReconcile(cr) {
					violate("parent and core must be paused while the WorkstationCluster is deleted")
				}
			case obj.GetName() == cloudWorkstationsCoreAppName:
				if !crOK || !cwsSkipReconcile(cr) || !pOK || !cwsSkipReconcile(p) {
					violate("core must be deleted while it and the parent are paused")
				}
				if _, ok := cwsExists(t, cl, newCWSDrainStage("WorkstationCluster").horizonGVK, "cl"); ok {
					violate("WorkstationCluster must be gone before the core delete")
				}
				if selfHeal, _, _ := unstructured.NestedBool(cr.Object, "spec", "syncPolicy", "automated", "selfHeal"); !selfHeal {
					violate("core selfHeal must not be changed")
				}
			case obj.GetName() == parentName:
				if crOK {
					violate("core Application must be gone before the parent delete")
				}
				if !pOK || !cwsSkipReconcile(p) {
					violate("parent must be deleted while paused")
				}
			}
			return cl.Delete(ctx, obj, opts...)
		},
		Patch: func(ctx context.Context, cl client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
			name := obj.GetName()
			if obj.GetObjectKind().GroupVersionKind() != cwsTestArgoAppGVK || (name != parentName && name != cloudWorkstationsCoreAppName) {
				return cl.Patch(ctx, obj, patch, opts...)
			}
			before, _ := getApp(ctx, cl, name)
			if err := cl.Patch(ctx, obj, patch, opts...); err != nil {
				return err
			}
			after, ok := getApp(ctx, cl, name)
			if !ok || before == nil || !cwsSkipReconcile(before) || cwsSkipReconcile(after) {
				return nil
			}
			if after.GetDeletionTimestamp().IsZero() {
				violate(name + " resumed before it was deleted")
				return nil
			}
			after.SetFinalizers(nil)
			return cl.Update(ctx, after)
		},
	}, parent, core, cwsHorizon("Workstation", "ws"), cwsHorizon("WorkstationConfig", "cfg"), cwsHorizon("WorkstationCluster", "cl"),
		cwsCRD(cloudWorkstationsHorizonCRDNames[0]), cwsCRD(cloudWorkstationsHorizonCRDNames[1]), cwsCRD(cloudWorkstationsHorizonCRDNames[2]), cwsCRD(cwsKCCCRDName))
}

func TestPerformModuleDisable_CloudWorkstationsDrainsThenDeletesParent(t *testing.T) {
	withFastCWSDrain(t, 5*time.Millisecond, 2*time.Second)

	var violations []string
	c := cwsArgoSimClient(t, "argocd", &violations)
	store := &cwsFakeStateStore{st: &State{
		EnabledModules: []string{"mod-cws"},
		ModuleIDs:      map[string]string{CloudWorkstationsModuleName: "mod-cws"},
	}}

	if err := PerformModuleDisable(context.Background(), c, store, nil, "argocd", "module-manager", CloudWorkstationsModuleName, "mod-cws", false, ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(violations) > 0 {
		t.Fatalf("disable ordering violated: %v", violations)
	}
	for _, kind := range []string{"Workstation", "WorkstationConfig", "WorkstationCluster"} {
		name := map[string]string{"Workstation": "ws", "WorkstationConfig": "cfg", "WorkstationCluster": "cl"}[kind]
		if _, ok := cwsExists(t, c, newCWSDrainStage(kind).horizonGVK, name); ok {
			t.Errorf("%s should have been drained", kind)
		}
	}
	for _, name := range []string{ApplicationName(CloudWorkstationsModuleName), cloudWorkstationsCoreAppName} {
		if _, ok := cwsExists2(t, c, cwsTestArgoAppGVK, "argocd", name); ok {
			t.Errorf("%s should be deleted after a successful drain", name)
		}
	}
	for _, name := range cloudWorkstationsHorizonCRDNames {
		if _, ok := cwsExists2(t, c, cwsCRDGVK, "", name); ok {
			t.Errorf("Horizon CRD %s should be removed by a successful disable", name)
		}
	}
	if _, ok := cwsExists2(t, c, cwsCRDGVK, "", cwsKCCCRDName); !ok {
		t.Error("the Config Connector CRD must never be removed")
	}
	if store.updates != 1 || len(store.st.EnabledModules) != 0 {
		t.Errorf("state should record the module as disabled (updates=%d, enabled=%v)", store.updates, store.st.EnabledModules)
	}
}

func TestPlatformDrainer_DisableOne_CloudWorkstationsDeletesPausedApplications(t *testing.T) {
	withFastCWSDrain(t, 5*time.Millisecond, 2*time.Second)

	var violations []string
	c := cwsArgoSimClient(t, testArgoNS, &violations)
	d, store := testDrainer(c, &State{
		EnabledModules: []string{"mod-cws"},
		ModuleIDs:      map[string]string{CloudWorkstationsModuleName: "mod-cws"},
	}, []CatalogEntry{{Name: CloudWorkstationsModuleName}})

	if err := d.disableOne(context.Background(), CloudWorkstationsModuleName, false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(violations) > 0 {
		t.Fatalf("disable ordering violated: %v", violations)
	}
	for _, name := range []string{ApplicationName(CloudWorkstationsModuleName), cloudWorkstationsCoreAppName} {
		if _, ok := cwsExists2(t, c, cwsTestArgoAppGVK, testArgoNS, name); ok {
			t.Errorf("%s should be deleted after a successful drain", name)
		}
	}
	if st, _ := store.Get(context.Background()); len(st.EnabledModules) != 0 {
		t.Errorf("cloud-workstations should be removed from enabled modules, got %v", st.EnabledModules)
	}
}

func cwsExists2(t *testing.T, c client.Client, gvk schema.GroupVersionKind, ns, name string) (*unstructured.Unstructured, bool) {
	t.Helper()
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(gvk)
	err := c.Get(context.Background(), client.ObjectKey{Namespace: ns, Name: name}, u)
	if errors.IsNotFound(err) {
		return nil, false
	}
	if err != nil {
		t.Fatalf("get %s/%s: %v", gvk.Kind, name, err)
	}
	return u, true
}

type cwsFixedCatalog []CatalogEntry

func (c cwsFixedCatalog) List(context.Context) ([]CatalogEntry, error) {
	return append([]CatalogEntry(nil), c...), nil
}
func (cwsFixedCatalog) GetPath(context.Context, string) (string, error) { return "", nil }

func cwsStuckDeletingWorkstation(t *testing.T) client.Client {
	t.Helper()
	kcc := cwsKCC("Workstation", "stuck-ws", kccFinalizer)
	c := cwsTestClient(interceptor.Funcs{}, cwsHorizon("Workstation", "stuck-ws", cwsTestFinalizer), kcc)
	if err := c.Delete(context.Background(), kcc.DeepCopy()); err != nil {
		t.Fatalf("mark KCC workstation deleting: %v", err)
	}
	return c
}

func TestDrainCloudWorkstations_StrictTimeoutKeepsHorizonAndKCCFinalizers(t *testing.T) {
	withFastCWSDrain(t, 5*time.Millisecond, 80*time.Millisecond)
	c := cwsStuckDeletingWorkstation(t)

	err := DrainCloudWorkstationsModule(context.Background(), c, CloudWorkstationsModuleName)
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("expected a timeout error, got %v", err)
	}
	ws, ok := cwsExists(t, c, newCWSDrainStage("Workstation").horizonGVK, "stuck-ws")
	if !ok {
		t.Fatal("stuck Horizon workstation should still exist")
	}
	if got := ws.GetFinalizers(); len(got) != 1 || got[0] != cwsTestFinalizer {
		t.Errorf("Horizon finalizer must stay, got %v", got)
	}
	kcc, ok := cwsExists(t, c, newCWSDrainStage("Workstation").kccGVK, "stuck-ws")
	if !ok {
		t.Fatal("stuck Config Connector workstation should still exist")
	}
	if got := kcc.GetFinalizers(); len(got) != 1 || got[0] != kccFinalizer {
		t.Errorf("Config Connector finalizer must stay, got %v", got)
	}
}

func TestDrainAllEnabledModules_StuckKCCFinalizerStillCompletes(t *testing.T) {
	withFastCWSDrain(t, 5*time.Millisecond, 80*time.Millisecond)
	c := cwsStuckDeletingWorkstation(t)
	store := &cwsFakeStateStore{st: &State{
		EnabledModules: []string{"mod-cws"},
		ModuleIDs:      map[string]string{CloudWorkstationsModuleName: "mod-cws"},
	}}
	p := &PlatformDrainer{
		Client: c, APIReader: c, StateStore: store, CatalogStore: cwsFixedCatalog{},
		Namespace: "module-manager", ArgoCDNamespace: "argocd",
	}
	if err := p.DrainAllEnabledModules(context.Background()); err != nil {
		t.Fatalf("platform drain should finish while a Config Connector finalizer is stuck, got %v", err)
	}
	if len(store.st.EnabledModules) != 0 {
		t.Errorf("cloud-workstations should be removed from enabled modules, got %v", store.st.EnabledModules)
	}
	if _, ok := cwsExists(t, c, newCWSDrainStage("Workstation").kccGVK, "stuck-ws"); ok {
		t.Error("deleting Config Connector workstation should have had its finalizer removed")
	}
	found := false
	for _, ns := range p.platformDrainNamespaces(context.Background()) {
		if ns == cloudWorkstationsNamespace {
			found = true
		}
	}
	if !found {
		t.Errorf("platform strip namespaces must include %s", cloudWorkstationsNamespace)
	}
}

func TestDrainCloudWorkstations_DeletesObjectCreatedDuringWait(t *testing.T) {
	withFastCWSDrain(t, 5*time.Millisecond, 2*time.Second)
	c := cwsTestClient(interceptor.Funcs{}, cwsHorizon("Workstation", "stuck-ws", cwsTestFinalizer))
	done := make(chan struct{})
	go func() {
		defer close(done)
		var created bool
		for i := 0; i < 400; i++ {
			u, ok := cwsExists(t, c, newCWSDrainStage("Workstation").horizonGVK, "stuck-ws")
			if ok && u.GetDeletionTimestamp() != nil && !created {
				if err := c.Create(context.Background(), cwsHorizon("Workstation", "late-ws")); err != nil {
					t.Errorf("create late workstation: %v", err)
					return
				}
				created = true
			}
			if created {
				if _, still := cwsExists(t, c, newCWSDrainStage("Workstation").horizonGVK, "late-ws"); !still {
					u.SetFinalizers(nil)
					_ = c.Update(context.Background(), u)
					return
				}
			}
			time.Sleep(5 * time.Millisecond)
		}
		t.Error("late workstation was not deleted during the wait")
	}()
	if err := DrainCloudWorkstationsModule(context.Background(), c, CloudWorkstationsModuleName); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	<-done
	if _, ok := cwsExists(t, c, newCWSDrainStage("Workstation").horizonGVK, "late-ws"); ok {
		t.Error("workstation created during the wait should have been deleted")
	}
}

func TestDrainAllEnabledModules_OtherModuleLeavesCloudWorkstationsUntouched(t *testing.T) {
	withFastCWSDrain(t, 5*time.Millisecond, 80*time.Millisecond)
	c := cwsTestClient(interceptor.Funcs{}, cwsHorizon("Workstation", "kept-ws", cwsTestFinalizer))
	store := &cwsFakeStateStore{st: &State{
		EnabledModules: []string{"mod-common"},
		ModuleIDs:      map[string]string{"workloads-common": "mod-common"},
	}}
	p := &PlatformDrainer{
		Client: c, APIReader: c, StateStore: store, CatalogStore: cwsFixedCatalog{},
		Namespace: "module-manager", ArgoCDNamespace: "argocd",
	}
	if err := p.DrainAllEnabledModules(context.Background()); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := cwsExists(t, c, newCWSDrainStage("Workstation").horizonGVK, "kept-ws")
	if !ok || got.GetDeletionTimestamp() != nil {
		t.Fatalf("a non-cloud-workstations module must not drain cloud-workstations (exists=%v)", ok)
	}
}

func TestDisableModuleAndRefresh_CRDCleanupTimeoutStillSweeps(t *testing.T) {
	withFastCWSCleanup(t, 5*time.Millisecond, 20*time.Millisecond, 30*time.Millisecond)

	stuck := cloudWorkstationsHorizonCRDNames[1]
	c := cwsTestClient(interceptor.Funcs{},
		cwsCRD(cloudWorkstationsHorizonCRDNames[0]),
		cwsCRD(stuck, cwsTestFinalizer),
		cwsCRD(cloudWorkstationsHorizonCRDNames[2]),
	)
	store := &cwsFakeStateStore{st: &State{
		EnabledModules: []string{"mod-cws", "mod-helper"},
		ModuleIDs: map[string]string{
			CloudWorkstationsModuleName: "mod-cws",
			"unused-helper":             "mod-helper",
		},
	}}
	catalog := cwsFixedCatalog{{Name: "unused-helper", AutoDisableWhenUnused: true}}
	err := DisableModuleAndRefresh(context.Background(), c, c, store, catalog, "argocd", "module-manager", CloudWorkstationsModuleName, "mod-cws", false, "")
	if err != nil {
		t.Fatalf("CRD cleanup timeout must not fail the disable, got %v", err)
	}
	if len(store.st.EnabledModules) != 0 {
		t.Errorf("disable and the dependent sweep should both finish, enabled=%v", store.st.EnabledModules)
	}
}

func TestDisableModuleAndRefresh_CloudWorkstationsSweepFinishesAfterCallerCancels(t *testing.T) {
	withFastCWSDrain(t, 5*time.Millisecond, 2*time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c := &cwsCtxStrictClient{Client: cwsTestClient(interceptor.Funcs{}, cwsDrainedFixtures()...)}
	fake := &cwsFakeStateStore{st: &State{
		EnabledModules: []string{"mod-cws", "mod-helper"},
		ModuleIDs: map[string]string{
			CloudWorkstationsModuleName: "mod-cws",
			"unused-helper":             "mod-helper",
		},
	}}
	catalog := cwsFixedCatalog{{Name: "unused-helper", AutoDisableWhenUnused: true}}
	err := DisableModuleAndRefresh(ctx, c, c, cwsCtxStrictStore{fake}, catalog, "argocd", "module-manager", CloudWorkstationsModuleName, "mod-cws", false, "")
	if err != nil {
		t.Fatalf("dependent sweep must finish after the caller cancels, got %v", err)
	}
	if len(fake.st.EnabledModules) != 0 {
		t.Errorf("disable and the dependent sweep should both finish, enabled=%v", fake.st.EnabledModules)
	}
}

func cwsPausedApps() []client.Object {
	apps := []*unstructured.Unstructured{cwsParentApp(), cwsCoreApp()}
	out := make([]client.Object, 0, len(apps))
	for _, app := range apps {
		app.SetAnnotations(map[string]string{argoCDSkipReconcileAnnotationKey: "true"})
		out = append(out, app)
	}
	return out
}

func cwsRestartStore(enabled bool) *cwsFakeStateStore {
	st := &State{ModuleIDs: map[string]string{CloudWorkstationsModuleName: "mod-cws"}}
	if enabled {
		st.EnabledModules = []string{"mod-cws"}
	}
	return &cwsFakeStateStore{st: st}
}

func TestResumeCloudWorkstationsAfterRestart_EnabledModuleResumesPausedApplications(t *testing.T) {
	c := cwsTestClient(interceptor.Funcs{}, cwsPausedApps()...)

	if err := ResumeCloudWorkstationsAfterRestart(context.Background(), c, cwsRestartStore(true), "argocd"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	cwsAssertResumedAndSelfHealIntact(t, c, "argocd")
}

// cwsTerminating marks an Application as being deleted. The fake client needs a finalizer for that.
func cwsTerminating(app *unstructured.Unstructured) *unstructured.Unstructured {
	now := metav1.Now()
	app.SetDeletionTimestamp(&now)
	app.SetFinalizers([]string{cwsTestArgoFinalizer})
	return app
}

func cwsPaused(app *unstructured.Unstructured) *unstructured.Unstructured {
	app.SetAnnotations(map[string]string{argoCDSkipReconcileAnnotationKey: "true"})
	return app
}

func cwsCountDeletes(n *int) interceptor.Funcs {
	return interceptor.Funcs{
		Delete: func(ctx context.Context, cl client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
			*n++
			return cl.Delete(ctx, obj, opts...)
		},
	}
}

func TestResumeCloudWorkstationsAfterRestart_DisabledModuleFinishesPendingParentDelete(t *testing.T) {
	parent := cwsPaused(cwsParentApp())
	parent.SetFinalizers([]string{cwsTestArgoFinalizer})
	c := cwsTestClient(interceptor.Funcs{}, parent, cwsPaused(cwsTerminating(cwsCoreApp())))

	if err := ResumeCloudWorkstationsAfterRestart(context.Background(), c, cwsRestartStore(false), "argocd"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, name := range []string{ApplicationName(CloudWorkstationsModuleName), cloudWorkstationsCoreAppName} {
		app, ok := cwsExists2(t, c, cwsTestArgoAppGVK, "argocd", name)
		if !ok {
			t.Fatalf("%s should remain until its finalizer runs", name)
		}
		if app.GetDeletionTimestamp().IsZero() {
			t.Errorf("%s must be deleted", name)
		}
		if cwsSkipReconcile(app) {
			t.Errorf("%s must be resumed so its finalizer can prune", name)
		}
	}
}

func TestResumeCloudWorkstationsAfterRestart_DisabledModuleResumesDeletingApplicationsWithoutDelete(t *testing.T) {
	deletes := 0
	c := cwsTestClient(cwsCountDeletes(&deletes), cwsPaused(cwsTerminating(cwsParentApp())), cwsPaused(cwsTerminating(cwsCoreApp())))

	if err := ResumeCloudWorkstationsAfterRestart(context.Background(), c, cwsRestartStore(false), "argocd"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deletes != 0 {
		t.Errorf("an Application that is already being deleted must not be deleted again, got %d deletes", deletes)
	}
	for _, name := range []string{ApplicationName(CloudWorkstationsModuleName), cloudWorkstationsCoreAppName} {
		if app, ok := cwsExists2(t, c, cwsTestArgoAppGVK, "argocd", name); !ok || cwsSkipReconcile(app) {
			t.Errorf("%s must be resumed (exists=%v)", name, ok)
		}
	}
}

func TestResumeCloudWorkstationsAfterRestart_DisabledModuleLeavesOtherApplicationsAlone(t *testing.T) {
	deletes := 0
	// Parent we did not pause, and a core that is paused but not being deleted.
	c := cwsTestClient(cwsCountDeletes(&deletes), cwsParentApp(), cwsPaused(cwsCoreApp()))

	if err := ResumeCloudWorkstationsAfterRestart(context.Background(), c, cwsRestartStore(false), "argocd"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deletes != 0 {
		t.Errorf("nothing should be deleted, got %d deletes", deletes)
	}
	if parent, ok := cwsExists2(t, c, cwsTestArgoAppGVK, "argocd", ApplicationName(CloudWorkstationsModuleName)); !ok || !parent.GetDeletionTimestamp().IsZero() {
		t.Errorf("an unpaused parent must be left alone (exists=%v)", ok)
	}
	if core, ok := cwsExists2(t, c, cwsTestArgoAppGVK, "argocd", cloudWorkstationsCoreAppName); !ok || !cwsSkipReconcile(core) {
		t.Errorf("a core that is not being deleted must stay paused (exists=%v)", ok)
	}
}

func withFastCWSRestartLock(t *testing.T, poll time.Duration) {
	t.Helper()
	old := cwsRestartLockPoll
	cwsRestartLockPoll = poll
	t.Cleanup(func() { cwsRestartLockPoll = old })
}

func TestResumeCloudWorkstationsAfterRestart_WaitsForLifecycleOperation(t *testing.T) {
	withFastCWSRestartLock(t, 5*time.Millisecond)
	c := cwsTestClient(interceptor.Funcs{}, cwsPausedApps()...)

	ModuleOpsMutex.Lock()
	done := make(chan error, 1)
	go func() {
		done <- ResumeCloudWorkstationsAfterRestart(context.Background(), c, cwsRestartStore(true), "argocd")
	}()
	select {
	case err := <-done:
		ModuleOpsMutex.Unlock()
		t.Fatalf("resume must wait while a lifecycle operation holds ModuleOpsMutex, returned %v", err)
	case <-time.After(60 * time.Millisecond):
	}
	for _, name := range []string{ApplicationName(CloudWorkstationsModuleName), cloudWorkstationsCoreAppName} {
		if app, ok := cwsExists2(t, c, cwsTestArgoAppGVK, "argocd", name); !ok || !cwsSkipReconcile(app) {
			t.Errorf("%s must stay paused while the lock is held (exists=%v)", name, ok)
		}
	}
	ModuleOpsMutex.Unlock()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("resume must run once the lock is released")
	}
	cwsAssertResumedAndSelfHealIntact(t, c, "argocd")
}

func TestResumeCloudWorkstationsAfterRestart_CancelWhileWaitingTouchesNothing(t *testing.T) {
	withFastCWSRestartLock(t, 5*time.Millisecond)
	c := cwsTestClient(interceptor.Funcs{}, cwsPausedApps()...)

	ModuleOpsMutex.Lock()
	defer ModuleOpsMutex.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if err := ResumeCloudWorkstationsAfterRestart(ctx, c, cwsRestartStore(true), "argocd"); err != context.DeadlineExceeded {
		t.Fatalf("expected context.DeadlineExceeded, got %v", err)
	}
	for _, name := range []string{ApplicationName(CloudWorkstationsModuleName), cloudWorkstationsCoreAppName} {
		if app, ok := cwsExists2(t, c, cwsTestArgoAppGVK, "argocd", name); !ok || !cwsSkipReconcile(app) {
			t.Errorf("%s must stay paused (exists=%v)", name, ok)
		}
	}
}

func cwsFailParentPatch(failures *int, failFirst int) interceptor.Funcs {
	return interceptor.Funcs{
		Patch: func(ctx context.Context, cl client.WithWatch, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
			if obj.GetName() == ApplicationName(CloudWorkstationsModuleName) {
				*failures++
				if failFirst < 0 || *failures <= failFirst {
					return fmt.Errorf("injected parent patch failure")
				}
			}
			return cl.Patch(ctx, obj, patch, opts...)
		},
	}
}

func TestResumeCloudWorkstationsParentAfterDelete_RetriesUntilItSucceeds(t *testing.T) {
	old := cwsResumeRetryInterval
	cwsResumeRetryInterval = 5 * time.Millisecond
	t.Cleanup(func() { cwsResumeRetryInterval = old })

	calls := 0
	c := cwsTestClient(cwsFailParentPatch(&calls, 2), cwsPaused(cwsTerminating(cwsParentApp())))

	if err := resumeCloudWorkstationsParentAfterDelete(context.Background(), c, "argocd", CloudWorkstationsModuleName); err != nil {
		t.Fatalf("unexpected error after retries: %v", err)
	}
	if calls != 3 {
		t.Errorf("expected 2 failures and 1 success, got %d patch attempts", calls)
	}
	if app, ok := cwsExists2(t, c, cwsTestArgoAppGVK, "argocd", ApplicationName(CloudWorkstationsModuleName)); !ok || cwsSkipReconcile(app) {
		t.Errorf("parent must be resumed (exists=%v)", ok)
	}
}

func TestResumeCloudWorkstationsParentAfterDelete_GivesUpWithinBudget(t *testing.T) {
	oldRetry, oldBudget := cwsResumeRetryInterval, resumeCloudWorkstationsCoreCleanupTimeout
	cwsResumeRetryInterval, resumeCloudWorkstationsCoreCleanupTimeout = 5*time.Millisecond, 50*time.Millisecond
	t.Cleanup(func() { cwsResumeRetryInterval, resumeCloudWorkstationsCoreCleanupTimeout = oldRetry, oldBudget })

	calls := 0
	c := cwsTestClient(cwsFailParentPatch(&calls, -1), cwsPaused(cwsTerminating(cwsParentApp())))

	start := time.Now()
	err := resumeCloudWorkstationsParentAfterDelete(context.Background(), c, "argocd", CloudWorkstationsModuleName)
	if err == nil || !strings.Contains(err.Error(), "injected parent patch failure") {
		t.Fatalf("expected the last patch error, got %v", err)
	}
	if calls < 2 {
		t.Errorf("expected several retries, got %d attempts", calls)
	}
	if time.Since(start) > 2*time.Second {
		t.Errorf("must stop at the cleanup budget, took %v", time.Since(start))
	}
}

// cwsCtxStrictClient fails every call made with a cancelled context, like a real API client would, and can
// cancel the caller's context after the first Workstation delete to simulate a disconnect mid-drain.
type cwsCtxStrictClient struct {
	client.Client
	cancel context.CancelFunc
	once   sync.Once
}

func (c *cwsCtxStrictClient) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return c.Client.Get(ctx, key, obj, opts...)
}

func (c *cwsCtxStrictClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return c.Client.List(ctx, list, opts...)
}

func (c *cwsCtxStrictClient) Update(ctx context.Context, obj client.Object, opts ...client.UpdateOption) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return c.Client.Update(ctx, obj, opts...)
}

func (c *cwsCtxStrictClient) Patch(ctx context.Context, obj client.Object, patch client.Patch, opts ...client.PatchOption) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return c.Client.Patch(ctx, obj, patch, opts...)
}

func (c *cwsCtxStrictClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	err := c.Client.Delete(ctx, obj, opts...)
	if c.cancel != nil && obj.GetObjectKind().GroupVersionKind().Kind == "Workstation" {
		c.once.Do(c.cancel)
	}
	return err
}

type cwsCtxStrictStore struct{ *cwsFakeStateStore }

func (s cwsCtxStrictStore) Update(ctx context.Context, st *State) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return s.cwsFakeStateStore.Update(ctx, st)
}

func TestPerformModuleDisable_CloudWorkstationsFinishesAfterCallerCancels(t *testing.T) {
	withFastCWSDrain(t, 5*time.Millisecond, 2*time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var violations []string
	c := &cwsCtxStrictClient{Client: cwsArgoSimClient(t, "argocd", &violations), cancel: cancel}
	fake := &cwsFakeStateStore{st: &State{
		EnabledModules: []string{"mod-cws"},
		ModuleIDs:      map[string]string{CloudWorkstationsModuleName: "mod-cws"},
	}}

	if err := PerformModuleDisable(ctx, c, cwsCtxStrictStore{fake}, nil, "argocd", "module-manager", CloudWorkstationsModuleName, "mod-cws", false, ""); err != nil {
		t.Fatalf("disable must finish after the caller cancels, got %v", err)
	}
	if ctx.Err() == nil {
		t.Fatal("test setup: the caller context was never cancelled")
	}
	if len(violations) > 0 {
		t.Fatalf("disable ordering violated: %v", violations)
	}
	for _, name := range []string{ApplicationName(CloudWorkstationsModuleName), cloudWorkstationsCoreAppName} {
		if _, ok := cwsExists2(t, c, cwsTestArgoAppGVK, "argocd", name); ok {
			t.Errorf("%s should be deleted", name)
		}
	}
	for _, name := range cloudWorkstationsHorizonCRDNames {
		if _, ok := cwsExists2(t, c, cwsCRDGVK, "", name); ok {
			t.Errorf("Horizon CRD %s should be removed", name)
		}
	}
	if fake.updates != 1 || len(fake.st.EnabledModules) != 0 {
		t.Errorf("state should record the module as disabled (updates=%d, enabled=%v)", fake.updates, fake.st.EnabledModules)
	}
}

func TestPerformModuleDisable_CloudWorkstationsDrainTimeoutAfterCallerCancelsStillResumes(t *testing.T) {
	withFastCWSDrain(t, 5*time.Millisecond, 100*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c := &cwsCtxStrictClient{Client: cwsTestClient(interceptor.Funcs{}, cwsParentApp(), cwsCoreApp(), cwsHorizon("Workstation", "stuck-ws", cwsTestFinalizer))}
	fake := &cwsFakeStateStore{st: &State{
		EnabledModules: []string{"mod-cws"},
		ModuleIDs:      map[string]string{CloudWorkstationsModuleName: "mod-cws"},
	}}

	err := PerformModuleDisable(ctx, c, cwsCtxStrictStore{fake}, nil, "argocd", "module-manager", CloudWorkstationsModuleName, "mod-cws", false, "")
	if err == nil || !strings.Contains(err.Error(), "Workstation/stuck-ws") {
		t.Fatalf("expected the stage timeout naming the blocker, not a cancellation, got %v", err)
	}
	if fake.updates != 0 || len(fake.st.EnabledModules) != 1 {
		t.Errorf("state must be untouched on drain failure (updates=%d, enabled=%v)", fake.updates, fake.st.EnabledModules)
	}
	cwsAssertResumedAndSelfHealIntact(t, c, "argocd")
}

func TestPerformModuleDisable_OtherModuleStillStopsWhenCallerCancels(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c := &cwsCtxStrictClient{Client: cwsTestClient(interceptor.Funcs{})}
	fake := &cwsFakeStateStore{st: &State{
		EnabledModules: []string{"mod-sample"},
		ModuleIDs:      map[string]string{"sample": "mod-sample"},
	}}

	err := PerformModuleDisable(ctx, c, cwsCtxStrictStore{fake}, nil, "argocd", "module-manager", "sample", "mod-sample", false, "")
	if err != context.Canceled {
		t.Fatalf("a non-cloud-workstations disable must keep the caller context, got %v", err)
	}
	if fake.updates != 0 || len(fake.st.EnabledModules) != 1 {
		t.Errorf("state must be untouched (updates=%d, enabled=%v)", fake.updates, fake.st.EnabledModules)
	}
}

type cwsCtxKey struct{}

func TestCloudWorkstationsDisableContext_IgnoresCancelKeepsValuesAndIsBounded(t *testing.T) {
	withFastCWSDrain(t, 5*time.Millisecond, 2*time.Second)

	parent, cancel := context.WithCancel(context.WithValue(context.Background(), cwsCtxKey{}, "kept"))
	cancel()
	ctx, stop := cloudWorkstationsDisableContext(parent)
	defer stop()

	if ctx.Err() != nil {
		t.Fatalf("detached context must ignore the caller cancel, got %v", ctx.Err())
	}
	if ctx.Value(cwsCtxKey{}) != "kept" {
		t.Error("context values must be preserved")
	}
	budget := time.Duration(len(cloudWorkstationsDrainStages))*cwsDrainStageTimeout + cwsDisableOverhead
	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("detached context must have a deadline")
	}
	if left := time.Until(deadline); left > budget || left < budget-time.Minute {
		t.Errorf("deadline should be about %v away, got %v", budget, left)
	}

	oldOverhead := cwsDisableOverhead
	cwsDisableOverhead = 20 * time.Millisecond
	t.Cleanup(func() { cwsDisableOverhead = oldOverhead })
	cwsDrainStageTimeout = 0
	short, stopShort := cloudWorkstationsDisableContext(context.Background())
	defer stopShort()
	select {
	case <-short.Done():
		if short.Err() != context.DeadlineExceeded {
			t.Errorf("expected DeadlineExceeded, got %v", short.Err())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("detached context must end once its budget is spent")
	}
}
