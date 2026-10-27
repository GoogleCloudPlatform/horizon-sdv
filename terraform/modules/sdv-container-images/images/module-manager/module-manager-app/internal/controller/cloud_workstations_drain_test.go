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
	"sync"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
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
	if err := DrainCloudWorkstationsModule(context.Background(), c, cloudWorkstationsModuleName); err != nil {
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

	if err := DrainCloudWorkstationsModule(context.Background(), c, cloudWorkstationsModuleName); err != nil {
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

	if err := DrainCloudWorkstationsModule(context.Background(), c, cloudWorkstationsModuleName); err != nil {
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

	err := DrainCloudWorkstationsModule(context.Background(), c, cloudWorkstationsModuleName)
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

	err := DrainCloudWorkstationsModule(context.Background(), c, cloudWorkstationsModuleName)
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

	if err := DrainCloudWorkstationsModule(ctx, c, cloudWorkstationsModuleName); err != context.DeadlineExceeded {
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
	app.SetName(ApplicationName(cloudWorkstationsModuleName))
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
		ModuleIDs:      map[string]string{cloudWorkstationsModuleName: "mod-cws"},
	}}

	err := PerformModuleDisable(context.Background(), c, store, nil, "argocd", "module-manager", cloudWorkstationsModuleName, "mod-cws", false, "")
	if err == nil || !strings.Contains(err.Error(), "Workstation/stuck-ws") {
		t.Fatalf("expected drain error naming the blocking workstation, got %v", err)
	}
	if store.updates != 0 || len(store.st.EnabledModules) != 1 {
		t.Errorf("state must be untouched on drain failure (updates=%d, enabled=%v)", store.updates, store.st.EnabledModules)
	}
	if _, ok := cwsExists2(t, c, cwsTestArgoAppGVK, "argocd", ApplicationName(cloudWorkstationsModuleName)); !ok {
		t.Error("parent Application must not be deleted when the drain fails")
	}
	core, ok := cwsExists2(t, c, cwsTestArgoAppGVK, "argocd", cloudWorkstationsCoreAppName)
	if !ok {
		t.Fatal("core Application must remain when the drain fails")
	}
	if core.GetAnnotations()[argoCDSkipReconcileAnnotationKey] != "true" {
		t.Errorf("core Application must stay paused after a drain failure, got annotations %v", core.GetAnnotations())
	}
	if selfHeal, found, err := unstructured.NestedBool(core.Object, "spec", "syncPolicy", "automated", "selfHeal"); err != nil || !found || !selfHeal {
		t.Errorf("core Application selfHeal must stay true while paused (selfHeal=%v, found=%v, err=%v)", selfHeal, found, err)
	}
	if crd, ok := cwsExists2(t, c, cwsCRDGVK, "", cloudWorkstationsHorizonCRDNames[2]); !ok || crd.GetDeletionTimestamp() != nil {
		t.Error("CRDs must not be touched when the drain fails")
	}
}

func TestPerformModuleDisable_CloudWorkstationsDrainsThenDeletesParent(t *testing.T) {
	withFastCWSDrain(t, 5*time.Millisecond, 2*time.Second)

	sawParentDelete := false
	c := cwsTestClient(interceptor.Funcs{
		Delete: func(ctx context.Context, cl client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
			if obj.GetName() == ApplicationName(cloudWorkstationsModuleName) {
				sawParentDelete = true
				core := &unstructured.Unstructured{}
				core.SetGroupVersionKind(cwsTestArgoAppGVK)
				if err := cl.Get(ctx, client.ObjectKey{Namespace: "argocd", Name: cloudWorkstationsCoreAppName}, core); err != nil {
					t.Errorf("get core Application before parent delete: %v", err)
				} else {
					if _, found := core.GetAnnotations()[argoCDSkipReconcileAnnotationKey]; found {
						t.Errorf("core Application skip-reconcile must be cleared before parent delete: %v", core.GetAnnotations())
					}
					selfHeal, found, err := unstructured.NestedBool(core.Object, "spec", "syncPolicy", "automated", "selfHeal")
					if err != nil || !found || selfHeal {
						t.Errorf("core Application selfHeal must be false before parent delete (selfHeal=%v, found=%v, err=%v)", selfHeal, found, err)
					}
				}
				if _, ok := cwsExists(t, cl, newCWSDrainStage("WorkstationCluster").horizonGVK, "cl"); ok {
					t.Error("WorkstationCluster must be gone before parent delete")
				}
			}
			return cl.Delete(ctx, obj, opts...)
		},
	}, cwsParentApp(), cwsCoreApp(), cwsHorizon("Workstation", "ws"), cwsHorizon("WorkstationConfig", "cfg"), cwsHorizon("WorkstationCluster", "cl"),
		cwsCRD(cloudWorkstationsHorizonCRDNames[0]), cwsCRD(cloudWorkstationsHorizonCRDNames[1]), cwsCRD(cloudWorkstationsHorizonCRDNames[2]), cwsCRD(cwsKCCCRDName))
	store := &cwsFakeStateStore{st: &State{
		EnabledModules: []string{"mod-cws"},
		ModuleIDs:      map[string]string{cloudWorkstationsModuleName: "mod-cws"},
	}}

	if err := PerformModuleDisable(context.Background(), c, store, nil, "argocd", "module-manager", cloudWorkstationsModuleName, "mod-cws", false, ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, kind := range []string{"Workstation", "WorkstationConfig", "WorkstationCluster"} {
		name := map[string]string{"Workstation": "ws", "WorkstationConfig": "cfg", "WorkstationCluster": "cl"}[kind]
		if _, ok := cwsExists(t, c, newCWSDrainStage(kind).horizonGVK, name); ok {
			t.Errorf("%s should have been drained", kind)
		}
	}
	if _, ok := cwsExists2(t, c, cwsTestArgoAppGVK, "argocd", ApplicationName(cloudWorkstationsModuleName)); ok {
		t.Error("parent Application should be deleted after a successful drain")
	}
	if !sawParentDelete {
		t.Error("parent Application delete was not observed")
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

	err := DrainCloudWorkstationsModule(context.Background(), c, cloudWorkstationsModuleName)
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
		ModuleIDs:      map[string]string{cloudWorkstationsModuleName: "mod-cws"},
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
	if err := DrainCloudWorkstationsModule(context.Background(), c, cloudWorkstationsModuleName); err != nil {
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
			cloudWorkstationsModuleName: "mod-cws",
			"unused-helper":             "mod-helper",
		},
	}}
	catalog := cwsFixedCatalog{{Name: "unused-helper", AutoDisableWhenUnused: true}}
	err := DisableModuleAndRefresh(context.Background(), c, c, store, catalog, "argocd", "module-manager", cloudWorkstationsModuleName, "mod-cws", false, "")
	if err != nil {
		t.Fatalf("CRD cleanup timeout must not fail the disable, got %v", err)
	}
	if len(store.st.EnabledModules) != 0 {
		t.Errorf("disable and the dependent sweep should both finish, enabled=%v", store.st.EnabledModules)
	}
}
