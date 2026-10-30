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
	"errors"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/go-logr/logr/funcr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"
)

const testArgoNS = "argocd"

func newApp(name string, labels map[string]string, finalizers []string, deletionTime *time.Time) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(argoApplicationGVK)
	u.SetName(name)
	u.SetNamespace(testArgoNS)
	if labels != nil {
		u.SetLabels(labels)
	}
	if finalizers != nil {
		u.SetFinalizers(finalizers)
	}
	if deletionTime != nil {
		t := metav1.NewTime(*deletionTime)
		u.SetDeletionTimestamp(&t)
	}
	return u
}

// newFakeClient builds a fake client that knows about the Argo Application GVK so
// list-by-label queries on unstructured objects work correctly, plus core types for the drain gate.
func newFakeClient(objs ...client.Object) client.Client {
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		panic(err)
	}
	scheme.AddKnownTypeWithName(argoApplicationGVK, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(
		schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "ApplicationList"},
		&unstructured.UnstructuredList{})
	return fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objs...).
		Build()
}

func getFinalizers(t *testing.T, c client.Client, name string) []string {
	t.Helper()
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(argoApplicationGVK)
	if err := c.Get(context.Background(), client.ObjectKey{Namespace: testArgoNS, Name: name}, u); err != nil {
		t.Fatalf("get %s: %v", name, err)
	}
	out := append([]string(nil), u.GetFinalizers()...)
	sort.Strings(out)
	return out
}

// ---------------------------------------------------------------------------
// PlatformDrainer unit tests
// ---------------------------------------------------------------------------

func TestRemovePlatformDrainFinalizer_RemovesOnlyPlatformFinalizer(t *testing.T) {
	app := newApp("horizon-sdv", nil,
		[]string{"resources-finalizer.argocd.argoproj.io", PlatformDrainFinalizer}, nil)
	c := newFakeClient(app)
	d := &PlatformDrainer{Client: c, APIReader: c, ArgoCDNamespace: testArgoNS}

	if err := d.RemovePlatformDrainFinalizer(context.Background(), testArgoNS, "horizon-sdv"); err != nil {
		t.Fatalf("RemovePlatformDrainFinalizer: %v", err)
	}
	got := getFinalizers(t, c, "horizon-sdv")
	if len(got) != 1 || got[0] != "resources-finalizer.argocd.argoproj.io" {
		t.Fatalf("finalizers = %v, want [resources-finalizer.argocd.argoproj.io]", got)
	}
}

func TestRemovePlatformDrainFinalizer_NoopWhenAbsent(t *testing.T) {
	app := newApp("horizon-sdv", nil, []string{"resources-finalizer.argocd.argoproj.io"}, nil)
	c := newFakeClient(app)
	d := &PlatformDrainer{Client: c, APIReader: c, ArgoCDNamespace: testArgoNS}

	if err := d.RemovePlatformDrainFinalizer(context.Background(), testArgoNS, "horizon-sdv"); err != nil {
		t.Fatalf("RemovePlatformDrainFinalizer: %v", err)
	}
	got := getFinalizers(t, c, "horizon-sdv")
	if len(got) != 1 || got[0] != "resources-finalizer.argocd.argoproj.io" {
		t.Fatalf("finalizers should be unchanged, got %v", got)
	}
}

func TestManagedModuleApplicationsRemaining_TrueWhenLabeled(t *testing.T) {
	now := time.Now()
	managed := newApp("mod-sample",
		map[string]string{ModuleManagerManagedLabelKey: "true"},
		[]string{"resources-finalizer.argocd.argoproj.io"}, &now)
	unmanaged := newApp("mod-other", nil, nil, nil)

	c := newFakeClient(managed, unmanaged)
	d := &PlatformDrainer{Client: c, APIReader: c, ArgoCDNamespace: testArgoNS}

	remaining, err := d.ManagedModuleApplicationsRemaining(context.Background(), testArgoNS)
	if err != nil {
		t.Fatalf("ManagedModuleApplicationsRemaining: %v", err)
	}
	if !remaining {
		t.Fatal("expected remaining=true when a managed app exists")
	}
}

func TestManagedModuleApplicationsRemaining_FalseWhenNoneLabeled(t *testing.T) {
	unmanaged := newApp("mod-other", nil, nil, nil)
	c := newFakeClient(unmanaged)
	d := &PlatformDrainer{Client: c, APIReader: c, ArgoCDNamespace: testArgoNS}

	remaining, err := d.ManagedModuleApplicationsRemaining(context.Background(), testArgoNS)
	if err != nil {
		t.Fatalf("ManagedModuleApplicationsRemaining: %v", err)
	}
	if remaining {
		t.Fatal("expected remaining=false when no managed apps exist")
	}
}

// ---------------------------------------------------------------------------
// RootGitOpsApplicationReconciler unit tests
// ---------------------------------------------------------------------------

// stubDrainer satisfies the platformDrainer interface for reconciler tests.
type stubDrainer struct {
	drainErr            error
	drainCalls          int
	disableErr          error
	disableCalls        int
	managedRemaining    bool
	managedRemainingErr error
	stripKCCCalled      int
	platformFinRemoved  bool
	destNamespaces      []string
	gatePresent         bool
	gatePresentErr      error
	gateReleased        bool
	steps               []string
}

func (s *stubDrainer) DrainAllEnabledModules(ctx context.Context) error {
	s.drainCalls++
	return s.drainErr
}
func (s *stubDrainer) DisableAllEnabledModules(ctx context.Context) error {
	s.disableCalls++
	return s.disableErr
}
func (s *stubDrainer) PlatformDrainGatePresent(ctx context.Context) (bool, error) {
	return s.gatePresent, s.gatePresentErr
}
func (s *stubDrainer) ReleasePlatformDrainGate(ctx context.Context) error {
	s.gateReleased = true
	s.steps = append(s.steps, "release-gate")
	return nil
}
func (s *stubDrainer) ManagedModuleApplicationsRemaining(ctx context.Context, ns string) (bool, error) {
	return s.managedRemaining, s.managedRemainingErr
}
func (s *stubDrainer) platformDrainNamespaces(ctx context.Context) []string {
	return s.destNamespaces
}
func (s *stubDrainer) StripKCCFinalizersInNamespaces(ctx context.Context, namespaces []string) error {
	s.stripKCCCalled++
	return nil
}
func (s *stubDrainer) RemovePlatformDrainFinalizer(ctx context.Context, ns, name string) error {
	s.platformFinRemoved = true
	s.steps = append(s.steps, "remove-root-finalizer")
	return nil
}

func TestReconcileRoot_RemovesPlatformFinalizerWhenNoAppsRemain(t *testing.T) {
	delTime := time.Now()
	root := newApp("horizon-sdv", nil, []string{PlatformDrainFinalizer}, &delTime)
	c := newFakeClient(root)
	stub := &stubDrainer{managedRemaining: false}

	r := &RootGitOpsApplicationReconciler{
		Client:             c,
		Drainer:            stub,
		ArgoCDNamespace:    testArgoNS,
		GitOpsRootAppNames: []string{"horizon-sdv"},
	}
	if _, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKey{Namespace: testArgoNS, Name: "horizon-sdv"},
	}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if !stub.platformFinRemoved {
		t.Fatal("platform-drain finalizer should be removed when no managed apps remain")
	}
	if stub.stripKCCCalled != 0 {
		t.Fatal("KCC stripper should not be called when no apps remain")
	}
}

func TestReconcileRoot_StripsKCCAndRequeuesWhileAppsRemain(t *testing.T) {
	delTime := time.Now()
	root := newApp("horizon-sdv", nil, []string{PlatformDrainFinalizer}, &delTime)
	c := newFakeClient(root)
	stub := &stubDrainer{managedRemaining: true, destNamespaces: []string{"sample-module-hello"}}

	r := &RootGitOpsApplicationReconciler{
		Client:             c,
		Drainer:            stub,
		ArgoCDNamespace:    testArgoNS,
		GitOpsRootAppNames: []string{"horizon-sdv"},
	}
	res, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKey{Namespace: testArgoNS, Name: "horizon-sdv"},
	})
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if res.RequeueAfter == 0 {
		t.Fatal("expected non-zero RequeueAfter while apps remain")
	}
	if stub.stripKCCCalled != 1 {
		t.Fatalf("KCC stripper should be called once while apps remain, got %d", stub.stripKCCCalled)
	}
	if stub.platformFinRemoved {
		t.Fatal("platform-drain finalizer must not be removed while apps remain")
	}
}

func TestReconcileRoot_NoopWhenNotDeleting(t *testing.T) {
	root := newApp("horizon-sdv", nil, []string{PlatformDrainFinalizer}, nil) // no deletion timestamp
	c := newFakeClient(root)
	stub := &stubDrainer{gatePresent: true}

	r := &RootGitOpsApplicationReconciler{
		Client:             c,
		Drainer:            stub,
		ArgoCDNamespace:    testArgoNS,
		GitOpsRootAppNames: []string{"horizon-sdv"},
	}
	if _, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: client.ObjectKey{Namespace: testArgoNS, Name: "horizon-sdv"},
	}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if stub.platformFinRemoved {
		t.Fatal("should not remove finalizer when app is not being deleted")
	}
	if stub.disableCalls != 0 || stub.drainCalls != 0 || stub.gateReleased {
		t.Fatal("should not drain or release the gate when the root is not being deleted")
	}
}

type memStateStore struct {
	mu       sync.Mutex
	state    *State
	disabled []string
}

func (m *memStateStore) Get(context.Context) (*State, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return cloneTestState(m.state), nil
}

func (m *memStateStore) Update(_ context.Context, st *State) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	prev := map[string]bool{}
	for _, id := range m.state.EnabledModules {
		prev[id] = true
	}
	nameByID := map[string]string{}
	for name, id := range m.state.ModuleIDs {
		nameByID[id] = name
	}
	for _, id := range st.EnabledModules {
		delete(prev, id)
	}
	var removed []string
	for id := range prev {
		removed = append(removed, nameByID[id])
	}
	sort.Strings(removed)
	m.disabled = append(m.disabled, removed...)
	m.state = cloneTestState(st)
	return nil
}

func (m *memStateStore) InvalidateCache() {}

func (m *memStateStore) disabledOrder() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]string(nil), m.disabled...)
}

type memCatalogStore struct {
	entries []CatalogEntry
}

func (m *memCatalogStore) List(context.Context) ([]CatalogEntry, error) {
	return append([]CatalogEntry(nil), m.entries...), nil
}

func (m *memCatalogStore) GetPath(context.Context, string) (string, error) {
	return "", nil
}

func cloneTestState(st *State) *State {
	if st == nil {
		return &State{ModuleIDs: map[string]string{}}
	}
	out := &State{
		EnabledModules: append([]string(nil), st.EnabledModules...),
		ModuleIDs:      map[string]string{},
	}
	for k, v := range st.ModuleIDs {
		out.ModuleIDs[k] = v
	}
	return out
}

func stateForModules(names ...string) *State {
	st := &State{ModuleIDs: map[string]string{}}
	for _, n := range names {
		st.ModuleIDs[n] = n
		st.EnabledModules = append(st.EnabledModules, n)
	}
	return st
}

func testDrainer(c client.Client, state *State, entries []CatalogEntry) (*PlatformDrainer, *memStateStore) {
	store := &memStateStore{state: cloneTestState(state)}
	return &PlatformDrainer{
		Client:          c,
		APIReader:       c,
		StateStore:      store,
		CatalogStore:    &memCatalogStore{entries: entries},
		ArgoCDNamespace: testArgoNS,
	}, store
}

func TestDisableAllEnabledModules_DependentsBeforeWorkloadsCommon(t *testing.T) {
	entries := []CatalogEntry{
		{Name: "workloads-common"},
		{Name: "workloads-android", HardDependencies: []string{"workloads-common"}},
		{Name: "remotive-topology", HardDependencies: []string{"workloads-common"}},
		{Name: "cloud-workstations", HardDependencies: []string{"workloads-common"}},
	}
	var parents []client.Object
	for _, name := range []string{"workloads-common", "workloads-android", "remotive-topology"} {
		parents = append(parents, newApp(ApplicationName(name), nil, nil, nil))
	}
	c := newFakeClient(parents...)
	d, store := testDrainer(c, stateForModules("workloads-common", "cloud-workstations", "workloads-android", "remotive-topology"), entries)

	if err := d.DisableAllEnabledModules(context.Background()); err != nil {
		t.Fatalf("DisableAllEnabledModules: %v", err)
	}
	got := store.disabledOrder()
	want := []string{"cloud-workstations", "remotive-topology", "workloads-android", "workloads-common"}
	if len(got) != len(want) {
		t.Fatalf("disable order = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("disable order = %v, want %v", got, want)
		}
	}
}

func TestDisableAllEnabledModules_SampleDataBeforeStorage(t *testing.T) {
	entries := []CatalogEntry{
		{Name: "storage-gcs"},
		{Name: "sample-data", HardDependencies: []string{"storage-gcs"}},
	}
	c := newFakeClient()
	d, store := testDrainer(c, stateForModules("storage-gcs", "sample-data"), entries)

	if err := d.DisableAllEnabledModules(context.Background()); err != nil {
		t.Fatalf("DisableAllEnabledModules: %v", err)
	}
	got := store.disabledOrder()
	if len(got) != 2 || got[0] != "sample-data" || got[1] != "storage-gcs" {
		t.Fatalf("disable order = %v, want [sample-data storage-gcs]", got)
	}
}

func TestDisableAllEnabledModules_CycleReturnsError(t *testing.T) {
	entries := []CatalogEntry{
		{Name: "a", HardDependencies: []string{"b"}},
		{Name: "b", HardDependencies: []string{"a"}},
	}
	c := newFakeClient()
	d, _ := testDrainer(c, stateForModules("a", "b"), entries)

	err := d.DisableAllEnabledModules(context.Background())
	if err == nil || !strings.Contains(err.Error(), "no module eligible") {
		t.Fatalf("error = %v, want cycle eligibility error", err)
	}
}

func TestDisableOne_ClearsSkipReconcileBeforeParentDelete(t *testing.T) {
	parent := newApp(ApplicationName("workloads-common"), nil, nil, nil)
	parent.SetAnnotations(map[string]string{argoCDSkipReconcileAnnotationKey: "true"})
	sawDelete := false
	scheme := runtime.NewScheme()
	scheme.AddKnownTypeWithName(argoApplicationGVK, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(
		schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "ApplicationList"},
		&unstructured.UnstructuredList{})
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(parent).
		WithInterceptorFuncs(interceptor.Funcs{
			Delete: func(ctx context.Context, cl client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
				if obj.GetNamespace() == testArgoNS && obj.GetName() == ApplicationName("workloads-common") {
					sawDelete = true
					got := &unstructured.Unstructured{}
					got.SetGroupVersionKind(argoApplicationGVK)
					if err := cl.Get(ctx, client.ObjectKey{Namespace: testArgoNS, Name: obj.GetName()}, got); err != nil {
						t.Errorf("get parent before delete: %v", err)
					} else if _, ok := got.GetAnnotations()[argoCDSkipReconcileAnnotationKey]; ok {
						t.Errorf("skip-reconcile still set when parent delete starts: %v", got.GetAnnotations())
					}
				}
				return cl.Delete(ctx, obj, opts...)
			},
		}).
		Build()
	d, _ := testDrainer(c, stateForModules("workloads-common"), []CatalogEntry{{Name: "workloads-common"}})

	if err := d.disableOne(context.Background(), "workloads-common", false); err != nil {
		t.Fatalf("disableOne: %v", err)
	}
	if !sawDelete {
		t.Fatal("parent Application was not deleted")
	}
}

func reconcileTestRoot(t *testing.T, root *unstructured.Unstructured, stub *stubDrainer) (ctrl.Result, error) {
	t.Helper()
	return reconcileTestRootWithContext(t, context.Background(), root, stub)
}

func reconcileTestRootWithContext(t *testing.T, ctx context.Context, root *unstructured.Unstructured, stub *stubDrainer) (ctrl.Result, error) {
	t.Helper()
	r := &RootGitOpsApplicationReconciler{
		Client:             newFakeClient(root),
		Drainer:            stub,
		ArgoCDNamespace:    testArgoNS,
		GitOpsRootAppNames: []string{"horizon-sdv"},
	}
	return r.Reconcile(ctx, ctrl.Request{
		NamespacedName: client.ObjectKey{Namespace: testArgoNS, Name: "horizon-sdv"},
	})
}

func TestReconcileRoot_GatedDrainReleasesGateThenFinalizer(t *testing.T) {
	now := time.Now()
	stub := &stubDrainer{gatePresent: true}
	if _, err := reconcileTestRoot(t, newApp("horizon-sdv", nil, []string{PlatformDrainFinalizer}, &now), stub); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if stub.disableCalls != 1 || stub.drainCalls != 0 {
		t.Fatalf("disable calls = %d, fallback drain calls = %d; want 1 and 0", stub.disableCalls, stub.drainCalls)
	}
	if stub.stripKCCCalled != 0 {
		t.Fatal("gated drain must not strip Config Connector finalizers")
	}
	want := []string{"release-gate", "remove-root-finalizer"}
	if len(stub.steps) != len(want) || stub.steps[0] != want[0] || stub.steps[1] != want[1] {
		t.Fatalf("steps = %v, want %v", stub.steps, want)
	}
}

func TestReconcileRoot_GatedDrainHoldsGateWhileAppsRemain(t *testing.T) {
	now := time.Now()
	stub := &stubDrainer{gatePresent: true, managedRemaining: true, destNamespaces: []string{"sample-module-hello"}}
	res, err := reconcileTestRoot(t, newApp("horizon-sdv", nil, []string{PlatformDrainFinalizer}, &now), stub)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if res.RequeueAfter == 0 {
		t.Fatal("expected non-zero RequeueAfter while module Applications remain")
	}
	if stub.gateReleased || stub.platformFinRemoved {
		t.Fatal("gate and root finalizer must stay while module Applications remain")
	}
	if stub.stripKCCCalled != 0 {
		t.Fatal("gated drain must not strip Config Connector finalizers while waiting")
	}
}

func TestReconcileRoot_GatedDrainHoldsGateOnDisableError(t *testing.T) {
	now := time.Now()
	stub := &stubDrainer{gatePresent: true, disableErr: errors.New("cloud-workstations drain timed out")}
	var logOutput strings.Builder
	logger := funcr.New(func(prefix, args string) {
		logOutput.WriteString(prefix)
		logOutput.WriteString(args)
	}, funcr.Options{})
	ctx := crlog.IntoContext(context.Background(), logger)

	res, err := reconcileTestRootWithContext(t, ctx, newApp("horizon-sdv", nil, []string{PlatformDrainFinalizer}, &now), stub)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if res.RequeueAfter == 0 {
		t.Fatal("expected a retry after a disable error")
	}
	if stub.gateReleased || stub.platformFinRemoved {
		t.Fatal("gate and root finalizer must stay after a disable error")
	}
	gotLog := logOutput.String()
	for _, want := range []string{
		"gateHeld",
		"true",
		strconv.Quote(platformDrainGateEscapeHatch(testArgoNS)),
	} {
		if !strings.Contains(gotLog, want) {
			t.Errorf("error log does not contain %q: %s", want, gotLog)
		}
	}
}

func TestReconcileRoot_GatedDrainWithoutRootFinalizer(t *testing.T) {
	now := time.Now()
	stub := &stubDrainer{gatePresent: true}
	root := newApp("horizon-sdv", nil, []string{"resources-finalizer.argocd.argoproj.io"}, &now)
	if _, err := reconcileTestRoot(t, root, stub); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if stub.disableCalls != 1 || !stub.gateReleased {
		t.Fatal("gated drain should run and release the gate without the root platform-drain finalizer")
	}
}

func TestReconcileRoot_NoGateNoFinalizerIsNoop(t *testing.T) {
	now := time.Now()
	stub := &stubDrainer{}
	root := newApp("horizon-sdv", nil, []string{"resources-finalizer.argocd.argoproj.io"}, &now)
	if _, err := reconcileTestRoot(t, root, stub); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if stub.disableCalls != 0 || stub.drainCalls != 0 || stub.platformFinRemoved {
		t.Fatal("nothing should run without the gate or the root platform-drain finalizer")
	}
}

func TestReconcileRoot_GateLookupErrorRetries(t *testing.T) {
	now := time.Now()
	stub := &stubDrainer{gatePresentErr: errors.New("forbidden")}
	if _, err := reconcileTestRoot(t, newApp("horizon-sdv", nil, []string{PlatformDrainFinalizer}, &now), stub); err == nil {
		t.Fatal("expected the gate lookup error to be returned")
	}
	if stub.disableCalls != 0 || stub.drainCalls != 0 {
		t.Fatal("must not pick a drain path when the gate lookup fails")
	}
}

func newGate(finalizers []string, deletionTime *time.Time) *corev1.ConfigMap {
	cm := &corev1.ConfigMap{}
	cm.SetNamespace(testArgoNS)
	cm.SetName(PlatformDrainGateName)
	cm.SetFinalizers(finalizers)
	if deletionTime != nil {
		t := metav1.NewTime(*deletionTime)
		cm.SetDeletionTimestamp(&t)
	}
	return cm
}

func TestPlatformDrainGatePresent(t *testing.T) {
	cases := []struct {
		name string
		objs []client.Object
		want bool
	}{
		{"gate with finalizer", []client.Object{newGate([]string{PlatformDrainGateFinalizer}, nil)}, true},
		{"gate without finalizer", []client.Object{newGate(nil, nil)}, false},
		{"no gate", nil, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := newFakeClient(tc.objs...)
			d := &PlatformDrainer{Client: c, APIReader: c, ArgoCDNamespace: testArgoNS}
			got, err := d.PlatformDrainGatePresent(context.Background())
			if err != nil {
				t.Fatalf("PlatformDrainGatePresent: %v", err)
			}
			if got != tc.want {
				t.Fatalf("present = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestReleasePlatformDrainGate_RemovesOnlyGateFinalizer(t *testing.T) {
	c := newFakeClient(newGate([]string{"example.com/other", PlatformDrainGateFinalizer}, nil))
	d := &PlatformDrainer{Client: c, APIReader: c, ArgoCDNamespace: testArgoNS}
	if err := d.ReleasePlatformDrainGate(context.Background()); err != nil {
		t.Fatalf("ReleasePlatformDrainGate: %v", err)
	}
	cm := &corev1.ConfigMap{}
	if err := c.Get(context.Background(), client.ObjectKey{Namespace: testArgoNS, Name: PlatformDrainGateName}, cm); err != nil {
		t.Fatalf("get gate: %v", err)
	}
	if got := cm.GetFinalizers(); len(got) != 1 || got[0] != "example.com/other" {
		t.Fatalf("finalizers = %v, want [example.com/other]", got)
	}
}

func TestReleasePlatformDrainGate_DeletingGateIsRemoved(t *testing.T) {
	now := time.Now()
	c := newFakeClient(newGate([]string{PlatformDrainGateFinalizer}, &now))
	d := &PlatformDrainer{Client: c, APIReader: c, ArgoCDNamespace: testArgoNS}
	if err := d.ReleasePlatformDrainGate(context.Background()); err != nil {
		t.Fatalf("ReleasePlatformDrainGate: %v", err)
	}
	err := c.Get(context.Background(), client.ObjectKey{Namespace: testArgoNS, Name: PlatformDrainGateName}, &corev1.ConfigMap{})
	if !apierrors.IsNotFound(err) {
		t.Fatalf("get gate after release = %v, want NotFound", err)
	}
}

func TestReleasePlatformDrainGate_NoopWhenAbsent(t *testing.T) {
	c := newFakeClient()
	d := &PlatformDrainer{Client: c, APIReader: c, ArgoCDNamespace: testArgoNS}
	if err := d.ReleasePlatformDrainGate(context.Background()); err != nil {
		t.Fatalf("ReleasePlatformDrainGate: %v", err)
	}
}

func TestRootIsDeleting(t *testing.T) {
	names := map[string]struct{}{"horizon-sdv": {}}
	now := time.Now()
	if !rootIsDeleting(newApp("horizon-sdv", nil, []string{"resources-finalizer.argocd.argoproj.io"}, &now), testArgoNS, names) {
		t.Fatal("deleting root should match without the platform-drain finalizer")
	}
	if rootIsDeleting(newApp("horizon-sdv", nil, []string{PlatformDrainFinalizer}, nil), testArgoNS, names) {
		t.Fatal("root that is not deleting must not match")
	}
	if rootIsDeleting(newApp("mod-sample", nil, []string{"resources-finalizer.argocd.argoproj.io"}, &now), testArgoNS, names) {
		t.Fatal("non-root applications must not match")
	}
	if rootIsDeleting(newApp("horizon-sdv", nil, []string{PlatformDrainFinalizer}, &now), "other-argocd", names) {
		t.Fatal("root in another namespace must not match")
	}
}
