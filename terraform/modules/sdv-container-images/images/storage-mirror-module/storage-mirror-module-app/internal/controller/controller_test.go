// Copyright (c) 2026 Accenture, All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//         http://www.apache.org/licenses/LICENSE-2.0
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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	smv1 "storage-mirror-module/api/v1alpha1"
	"storage-mirror-module/internal/catalog"
	"storage-mirror-module/internal/dispatch"
	"storage-mirror-module/internal/state"
	"storage-mirror-module/internal/volume"
)

type fakeDispatcher struct{ reqs []dispatch.Request }

func (f *fakeDispatcher) Dispatch(_ context.Context, r dispatch.Request) (string, error) {
	if r.Params == nil {
		r.Params = map[string]string{}
	}
	if r.Params["requestId"] == "" {
		r.Params["requestId"] = dispatch.NewRequestID()
	}
	f.reqs = append(f.reqs, r)
	return r.Params["requestId"], nil
}

var now = time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)

func testScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(s)
	_ = smv1.AddToScheme(s)
	for _, gvk := range []schema.GroupVersionKind{WorkflowGVK, volume.FilestoreGVK} {
		s.AddKnownTypeWithName(gvk, &unstructured.Unstructured{})
		s.AddKnownTypeWithName(gvk.GroupVersion().WithKind(gvk.Kind+"List"), &unstructured.UnstructuredList{})
	}
	return s
}

func config() *smv1.StorageMirrorConfig {
	return &smv1.StorageMirrorConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "default", Namespace: "horizon"},
		Spec: smv1.StorageMirrorConfigSpec{
			Volume:    smv1.VolumeSpec{CapacityGb: 2048, Zone: "europe-west1-d", Network: "sdv-network"},
			Defaults:  smv1.MirrorDefaults{Repo: smv1.RepoDefaults{ManifestURL: "https://android.googlesource.com/platform/manifest", ManifestRef: "android-16.0.0_r3"}},
			Consumers: smv1.ConsumersSpec{Jenkins: smv1.JenkinsConsumer{Enabled: true, ReadOnly: true}},
		},
	}
}

func mirror() *smv1.StorageMirror {
	return &smv1.StorageMirror{
		ObjectMeta: metav1.ObjectMeta{Name: "aosp", Namespace: "horizon", Annotations: map[string]string{}},
		Spec:       smv1.StorageMirrorSpec{Kind: smv1.KindRepo, Repo: &smv1.RepoSpec{}, DeletionPolicy: smv1.DeletionPolicyRetain},
	}
}

func workflow(name, template, mirror, reqID, phase string, created time.Time) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(WorkflowGVK)
	u.SetName(name)
	u.SetNamespace("workflows")
	u.SetCreationTimestamp(metav1.NewTime(created))
	lbl := map[string]string{smv1.LabelModule: smv1.ModuleName, "workflows.argoproj.io/workflow-template": template, smv1.LabelRequestID: reqID}
	if mirror != "" {
		lbl[smv1.LabelMirror] = mirror
	}
	u.SetLabels(lbl)
	_ = unstructured.SetNestedField(u.Object, phase, "status", "phase")
	_ = unstructured.SetNestedField(u.Object, created.Format(time.RFC3339), "status", "startedAt")
	if phase == "Succeeded" || phase == "Failed" {
		_ = unstructured.SetNestedField(u.Object, created.Add(time.Hour).Format(time.RFC3339), "status", "finishedAt")
	}
	return u
}

func newMirrorReconciler(objs ...client.Object) (*MirrorReconciler, *fakeDispatcher, client.Client) {
	c := fake.NewClientBuilder().WithScheme(testScheme()).WithObjects(objs...).
		WithStatusSubresource(&smv1.StorageMirror{}, &smv1.StorageMirrorConfig{}).Build()
	d := &fakeDispatcher{}
	return &MirrorReconciler{Client: c, Namespace: "horizon", WorkflowNamespace: "workflows", Dispatcher: d, Progress: state.NewStore(), Now: func() time.Time { return now }}, d, c
}

func reconcileMirror(t *testing.T, r *MirrorReconciler, name string) smv1.StorageMirror {
	t.Helper()
	if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKey{Namespace: "horizon", Name: name}}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	var m smv1.StorageMirror
	_ = r.Get(context.Background(), client.ObjectKey{Namespace: "horizon", Name: name}, &m)
	return m
}

func TestNewMirrorDispatchesCreateOnce(t *testing.T) {
	m := mirror()
	m.Annotations[AnnotationSyncRequestID] = "req-create"
	m.Annotations[smv1.AnnotationCreatedBy] = "jane"
	r, d, _ := newMirrorReconciler(config(), m)
	got := reconcileMirror(t, r, "aosp")
	if len(d.reqs) != 1 || d.reqs[0].Template != dispatch.TemplateCreate || d.reqs[0].Params["requestId"] != "req-create" || d.reqs[0].SubmittedBy != "jane" {
		t.Fatalf("dispatch %+v", d.reqs)
	}
	if got.Status.PendingRequestID != "req-create" || got.Status.Phase != smv1.PhasePending {
		t.Fatalf("status %+v", got.Status)
	}
	if len(got.Finalizers) != 1 {
		t.Fatal("finalizer missing")
	}
	reconcileMirror(t, r, "aosp")
	if len(d.reqs) != 1 {
		t.Fatal("pending request must not be dispatched twice")
	}
	// A reconcile that reads a stale CR (no pendingRequestId yet) must not dispatch again either.
	stale := mirror()
	stale.Annotations[AnnotationSyncRequestID] = "req-create"
	r2, d2, c2 := newMirrorReconciler(config(), stale)
	reconcileMirror(t, r2, "aosp")
	var cur smv1.StorageMirror
	_ = c2.Get(context.Background(), client.ObjectKey{Namespace: "horizon", Name: "aosp"}, &cur)
	cur.Status = smv1.StorageMirrorStatus{}
	_ = c2.Status().Update(context.Background(), &cur)
	reconcileMirror(t, r2, "aosp")
	if len(d2.reqs) != 1 {
		t.Fatalf("stale read dispatched again: %d", len(d2.reqs))
	}
}

func TestImportedUninitializedWaitsForSync(t *testing.T) {
	m := mirror()
	m.Annotations[AnnotationImportedStatus] = "uninitialized"
	r, d, c := newMirrorReconciler(config(), m)
	got := reconcileMirror(t, r, "aosp")
	if len(d.reqs) != 0 || got.Status.Phase != smv1.PhasePending {
		t.Fatalf("imported mirror created automatically: %+v %s", d.reqs, got.Status.Phase)
	}
	if err := RequestSync(context.Background(), c, &got, "jane", "api", "req-1", now); err != nil {
		t.Fatal(err)
	}
	reconcileMirror(t, r, "aosp")
	if len(d.reqs) != 1 || d.reqs[0].Template != dispatch.TemplateSync {
		t.Fatalf("sync request: %+v", d.reqs)
	}
}

func TestCreateNotDispatchedWithoutManifest(t *testing.T) {
	cfg := config()
	cfg.Spec.Defaults.Repo = smv1.RepoDefaults{}
	r, d, _ := newMirrorReconciler(cfg, mirror())
	got := reconcileMirror(t, r, "aosp")
	if len(d.reqs) != 0 {
		t.Fatalf("dispatched a repo create without manifest: %+v", d.reqs)
	}
	c := meta.FindStatusCondition(got.Status.Conditions, "Dispatched")
	if c == nil || c.Reason != "InvalidSpec" {
		t.Fatalf("conditions %+v", got.Status.Conditions)
	}
}

func TestRunsDriveStatusAndProgress(t *testing.T) {
	m := mirror()
	r, d, c := newMirrorReconciler(config(), m, workflow("create-1", dispatch.TemplateCreate, "aosp", "a", "Running", now.Add(-time.Hour)))
	r.Progress.Put("create-1", state.Report{Mirror: "aosp", Progress: smv1.Progress{Phase: "fetch", State: "running", ProjectsDone: 5, ProjectsTotal: 10, OverallPercent: 40}, ReceivedAt: now})
	got := reconcileMirror(t, r, "aosp")
	if got.Status.Phase != smv1.PhaseInitializing || got.Status.CurrentRun == nil || got.Status.CurrentRun.Progress.ProjectsDone != 5 || got.Status.CatalogStatus != "syncing" {
		t.Fatalf("status %+v", got.Status)
	}
	if len(d.reqs) != 0 {
		t.Fatal("no dispatch while create runs")
	}
	wf := workflow("create-1", dispatch.TemplateCreate, "aosp", "a", "Succeeded", now.Add(-time.Hour))
	cur := &unstructured.Unstructured{}
	cur.SetGroupVersionKind(WorkflowGVK)
	_ = c.Get(context.Background(), client.ObjectKey{Namespace: "workflows", Name: "create-1"}, cur)
	wf.SetResourceVersion(cur.GetResourceVersion())
	if err := c.Update(context.Background(), wf); err != nil {
		t.Fatal(err)
	}
	got = reconcileMirror(t, r, "aosp")
	if got.Status.Phase != smv1.PhaseReady || got.Status.CurrentRun != nil || got.Status.LastSuccessfulSyncTime == nil || len(got.Status.History) != 1 {
		t.Fatalf("status %+v", got.Status)
	}
}

func TestSyncRequestDispatchesAndDeduplicates(t *testing.T) {
	m := mirror()
	m.Annotations[smv1.AnnotationSyncRequestedAt] = now.Format(time.RFC3339)
	m.Annotations[AnnotationSyncRequestID] = "s1"
	r, d, _ := newMirrorReconciler(config(), m, workflow("create-1", dispatch.TemplateCreate, "aosp", "a", "Succeeded", now.Add(-3*time.Hour)))
	got := reconcileMirror(t, r, "aosp")
	if len(d.reqs) != 1 || d.reqs[0].Template != dispatch.TemplateSync || got.Status.LastHandledSyncRequest == "" {
		t.Fatalf("dispatch %+v status %+v", d.reqs, got.Status)
	}
	reconcileMirror(t, r, "aosp")
	if len(d.reqs) != 1 {
		t.Fatal("handled request must not be dispatched again")
	}

	m2 := mirror()
	m2.Annotations[smv1.AnnotationSyncRequestedAt] = now.Format(time.RFC3339)
	r2, d2, _ := newMirrorReconciler(config(), m2, workflow("sync-1", dispatch.TemplateSync, "aosp", "x", "Running", now))
	reconcileMirror(t, r2, "aosp")
	if len(d2.reqs) != 0 {
		t.Fatal("sync request while a run is active is folded into it")
	}
}

func TestDeleteRetainReleasesFinalizer(t *testing.T) {
	m := mirror()
	m.Finalizers = []string{smv1.Finalizer}
	r, d, c := newMirrorReconciler(config(), m, workflow("create-1", dispatch.TemplateCreate, "aosp", "a", "Succeeded", now.Add(-time.Hour)))
	if err := c.Delete(context.Background(), m); err != nil {
		t.Fatal(err)
	}
	reconcileMirror(t, r, "aosp")
	var left smv1.StorageMirror
	if err := c.Get(context.Background(), client.ObjectKey{Namespace: "horizon", Name: "aosp"}, &left); err == nil {
		t.Fatal("Retain delete must release the CR")
	}
	if len(d.reqs) != 0 {
		t.Fatal("Retain must not dispatch a delete run")
	}
}

func TestDeletePurgeWaitsForDeleteRun(t *testing.T) {
	m := mirror()
	m.Finalizers = []string{smv1.Finalizer}
	m.Spec.DeletionPolicy = smv1.DeletionPolicyDelete
	m.Annotations[smv1.AnnotationDeleteRequestedBy] = "jane"
	m.Annotations[AnnotationSyncRequestedBy] = "someone-else"
	r, d, c := newMirrorReconciler(config(), m)
	_ = c.Delete(context.Background(), m)
	reconcileMirror(t, r, "aosp")
	if len(d.reqs) != 1 || d.reqs[0].Template != dispatch.TemplateDelete || d.reqs[0].Params["confirm"] != "aosp" || d.reqs[0].SubmittedBy != "jane" {
		t.Fatalf("dispatch %+v", d.reqs)
	}
	id := d.reqs[0].Params["requestId"]
	got := reconcileMirror(t, r, "aosp")
	if len(got.Finalizers) != 1 || len(d.reqs) != 1 {
		t.Fatal("finalizer must stay until the delete run succeeds")
	}
	if err := c.Create(context.Background(), workflow("delete-1", dispatch.TemplateDelete, "aosp", id, "Succeeded", now)); err != nil {
		t.Fatal(err)
	}
	reconcileMirror(t, r, "aosp")
	if err := c.Get(context.Background(), client.ObjectKey{Namespace: "horizon", Name: "aosp"}, &got); err == nil {
		t.Fatal("CR must go after a successful purge")
	}
}

func TestWorkflowWithoutCRIsNotAdopted(t *testing.T) {
	// A create run posted straight to the webhook (no StorageMirror) must not create a CR.
	wf := workflow("create-p", dispatch.TemplateCreate, "kernel", "p", "Succeeded", now)
	_ = unstructured.SetNestedSlice(wf.Object, []interface{}{
		map[string]interface{}{"name": "kind", "value": "git"},
		map[string]interface{}{"name": "gitUrl", "value": "https://example.com/k.git"},
	}, "spec", "arguments", "parameters")
	r, d, c := newMirrorReconciler(config(), wf)
	reconcileMirror(t, r, "kernel")
	var m smv1.StorageMirror
	if err := c.Get(context.Background(), client.ObjectKey{Namespace: "horizon", Name: "kernel"}, &m); err == nil {
		t.Fatal("a Workflow without a StorageMirror must not be adopted")
	}
	if len(d.reqs) != 0 {
		t.Fatalf("dispatched %d runs", len(d.reqs))
	}
}

func TestConfigReconcileCreatesInstanceThenClaims(t *testing.T) {
	jenkinsPVC := &corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "sdv-mirror-filestore-pvc", Namespace: "jenkins"}}
	c := fake.NewClientBuilder().WithScheme(testScheme()).WithObjects(config(), jenkinsPVC).
		WithStatusSubresource(&smv1.StorageMirrorConfig{}).Build()
	d := &fakeDispatcher{}
	r := &ConfigReconciler{Client: c, Namespace: "horizon", WorkflowNamespace: "workflows", ProjectID: "proj", Dispatcher: d, Now: func() time.Time { return now }}
	req := ctrl.Request{NamespacedName: client.ObjectKey{Namespace: "horizon", Name: "default"}}
	if _, err := r.Reconcile(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	fs := &unstructured.Unstructured{}
	fs.SetGroupVersionKind(volume.FilestoreGVK)
	if err := c.Get(context.Background(), client.ObjectKey{Namespace: "horizon", Name: volume.FilestoreInstanceName}, fs); err != nil {
		t.Fatalf("instance: %v", err)
	}
	var pvs corev1.PersistentVolumeList
	_ = c.List(context.Background(), &pvs)
	if len(pvs.Items) != 0 {
		t.Fatal("no PVs before the instance is ready")
	}
	_ = unstructured.SetNestedSlice(fs.Object, []interface{}{map[string]interface{}{"type": "Ready", "status": "True"}}, "status", "conditions")
	_ = unstructured.SetNestedSlice(fs.Object, []interface{}{map[string]interface{}{"ipAddresses": []interface{}{"10.9.9.9"}}}, "status", "observedState", "networks")
	if err := c.Update(context.Background(), fs); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Reconcile(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	_ = c.List(context.Background(), &pvs)
	if len(pvs.Items) != 2 {
		t.Fatalf("expected the two workflows PVs (Jenkins claim is foreign), got %d", len(pvs.Items))
	}
	var cfg smv1.StorageMirrorConfig
	_ = c.Get(context.Background(), req.NamespacedName, &cfg)
	if cfg.Status.Volume.IP != "10.9.9.9" || len(d.reqs) != 1 || d.reqs[0].Template != dispatch.TemplateVolumeInit {
		t.Fatalf("status %+v dispatch %+v", cfg.Status, d.reqs)
	}
	if _, err := r.Reconcile(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if len(d.reqs) != 1 {
		t.Fatalf("volume-init must not be dispatched again while the first request is in flight: %d", len(d.reqs))
	}
	var jp corev1.PersistentVolumeClaim
	_ = c.Get(context.Background(), client.ObjectKey{Namespace: "jenkins", Name: "sdv-mirror-filestore-pvc"}, &jp)
	if jp.Labels[smv1.LabelModule] != "" {
		t.Fatal("foreign Jenkins claim must stay untouched")
	}
	_ = c.Get(context.Background(), req.NamespacedName, &cfg)
	bigger := cfg.DeepCopy()
	bigger.Spec.Volume.CapacityGb = 2304
	if err := c.Update(context.Background(), bigger); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Reconcile(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	_ = c.Get(context.Background(), client.ObjectKey{Namespace: "horizon", Name: volume.FilestoreInstanceName}, fs)
	if volume.InstanceCapacityGb(fs) != 2304 {
		t.Fatal("resize not applied")
	}
}

func TestDueAt(t *testing.T) {
	m := mirror()
	scheds := []smv1.ScheduleSpec{{Name: "nightly", Cron: "0 1 * * *"}}
	due, err := DueAt(m, scheds, time.Date(2026, 11, 25, 0, 30, 0, 0, time.UTC), time.Date(2026, 11, 25, 1, 0, 30, 0, time.UTC))
	if err != nil || due.Hour() != 1 {
		t.Fatalf("due %v %v", due, err)
	}
	m.Spec.Schedule = smv1.ScheduleNone
	if due, _ := DueAt(m, scheds, now.Add(-48*time.Hour), now); !due.IsZero() {
		t.Fatal("None disables schedules")
	}
	m.Spec.Schedule = "*/30 * * * *"
	if due, _ := DueAt(m, nil, now.Add(-time.Minute), now); !due.Equal(now) {
		t.Fatalf("own cron: %v", due)
	}
	m.Spec.Schedule = smv1.ScheduleInherit
	m.Labels = map[string]string{"tier": "a"}
	sel := []smv1.ScheduleSpec{{Name: "b", Cron: "* * * * *", Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"tier": "b"}}}}
	if due, _ := DueAt(m, sel, now.Add(-time.Hour), now); !due.IsZero() {
		t.Fatal("selector must filter")
	}
}

func TestImportCreatesRetainCRs(t *testing.T) {
	snap, err := catalog.Parse([]byte("horizon-mirrors:\n  My_Mirror:\n    type: repo\n    manifest_url: u\n    status: ready\n  managed:\n    type: repo\n"), "horizon-mirrors")
	if err != nil {
		t.Fatal(err)
	}
	m := mirror()
	m.Name, m.Spec.Directory = "managed", "managed"
	c := fake.NewClientBuilder().WithScheme(testScheme()).WithObjects(m).Build()
	created, err := Import(context.Background(), c, "horizon", snap, nil, "test")
	if err != nil || len(created) != 1 || created[0] != "my-mirror" {
		t.Fatalf("import %v %v", created, err)
	}
	var got smv1.StorageMirror
	_ = c.Get(context.Background(), client.ObjectKey{Namespace: "horizon", Name: "my-mirror"}, &got)
	if got.Spec.Directory != "My_Mirror" || got.Spec.DeletionPolicy != smv1.DeletionPolicyRetain || got.Annotations[AnnotationImportedStatus] != "ready" {
		t.Fatalf("imported %+v", got)
	}
}

func TestParseArgoProgress(t *testing.T) {
	if n, m, ok := ParseArgoProgress("3/7"); !ok || n != 3 || m != 7 {
		t.Fatal("parse")
	}
	if _, _, ok := ParseArgoProgress("0/0"); ok {
		t.Fatal("zero total")
	}
}

func TestConfigReconcileTestNFSDriver(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(testScheme()).WithObjects(config()).
		WithStatusSubresource(&smv1.StorageMirrorConfig{}).Build()
	r := &ConfigReconciler{Client: c, Namespace: "horizon", WorkflowNamespace: "workflows", Dispatcher: &fakeDispatcher{},
		TestNFS: &volume.TestNFS{Server: "nfs.e2e.svc", Path: "/exports"}, Now: func() time.Time { return now }}
	if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKey{Namespace: "horizon", Name: "default"}}); err != nil {
		t.Fatal(err)
	}
	fs := &unstructured.Unstructured{}
	fs.SetGroupVersionKind(volume.FilestoreGVK)
	if err := c.Get(context.Background(), client.ObjectKey{Namespace: "horizon", Name: volume.FilestoreInstanceName}, fs); err == nil {
		t.Fatal("test driver must not create a FilestoreInstance")
	}
	var pv corev1.PersistentVolume
	if err := c.Get(context.Background(), client.ObjectKey{Name: "storage-mirror-workflows-ro"}, &pv); err != nil {
		t.Fatal(err)
	}
	if pv.Spec.NFS == nil || pv.Spec.NFS.Server != "nfs.e2e.svc" || pv.Spec.NFS.Path != "/exports" || !pv.Spec.NFS.ReadOnly || pv.Spec.CSI != nil {
		t.Fatalf("pv %+v", pv.Spec.PersistentVolumeSource)
	}
}

func TestVolumeInitLostDispatchIsResent(t *testing.T) {
	c := fake.NewClientBuilder().WithScheme(testScheme()).WithObjects(config()).
		WithStatusSubresource(&smv1.StorageMirrorConfig{}).Build()
	cur := now
	d := &fakeDispatcher{}
	r := &ConfigReconciler{Client: c, Namespace: "horizon", WorkflowNamespace: "workflows", Dispatcher: d,
		TestNFS: &volume.TestNFS{Server: "nfs"}, Now: func() time.Time { return cur }}
	req := ctrl.Request{NamespacedName: client.ObjectKey{Namespace: "horizon", Name: "default"}}
	step := func(after time.Duration) {
		t.Helper()
		cur = cur.Add(after)
		if _, err := r.Reconcile(context.Background(), req); err != nil {
			t.Fatal(err)
		}
	}
	step(0)
	step(time.Minute)
	if len(d.reqs) != 1 {
		t.Fatalf("dispatched %d times within the lost window", len(d.reqs))
	}
	step(2 * time.Minute)
	if len(d.reqs) != 2 {
		t.Fatalf("lost dispatch not resent: %d", len(d.reqs))
	}
	var cfg smv1.StorageMirrorConfig
	_ = c.Get(context.Background(), req.NamespacedName, &cfg)
	if err := c.Create(context.Background(), workflow("vi-1", dispatch.TemplateVolumeInit, "", cfg.Status.VolumeInitRequestID, "Failed", cur)); err != nil {
		t.Fatal(err)
	}
	step(3 * time.Minute)
	if len(d.reqs) != 2 {
		t.Fatalf("a run that started must get the long back-off: %d", len(d.reqs))
	}
	step(15 * time.Minute)
	if len(d.reqs) != 3 {
		t.Fatalf("no retry after the back-off: %d", len(d.reqs))
	}
}
