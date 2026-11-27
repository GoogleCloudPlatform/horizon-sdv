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

package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	smv1 "storage-mirror-module/api/v1alpha1"
	"storage-mirror-module/internal/controller"
	"storage-mirror-module/internal/state"
)

func newServer(t *testing.T, objs ...client.Object) (*Server, client.Client) {
	t.Helper()
	s := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(s)
	_ = smv1.AddToScheme(s)
	s.AddKnownTypeWithName(controller.WorkflowGVK, &unstructured.Unstructured{})
	s.AddKnownTypeWithName(controller.WorkflowGVK.GroupVersion().WithKind("WorkflowList"), &unstructured.UnstructuredList{})
	cfg := &smv1.StorageMirrorConfig{ObjectMeta: metav1.ObjectMeta{Name: "default", Namespace: "horizon"}, Spec: smv1.StorageMirrorConfigSpec{
		Volume:   smv1.VolumeSpec{CapacityGb: 2048},
		Defaults: smv1.MirrorDefaults{Repo: smv1.RepoDefaults{ManifestURL: "https://android.googlesource.com/platform/manifest"}},
	}}
	c := fake.NewClientBuilder().WithScheme(s).WithObjects(append(objs, cfg)...).WithStatusSubresource(&smv1.StorageMirror{}).Build()
	tokens := []APIToken{{Caller: CallerHorizonAPI, Token: "hapi"}, {Caller: CallerAgent, Token: "agent"}}
	return &Server{Client: c, Namespace: "horizon", WorkflowNamespace: "workflows", Tokens: tokens, Progress: state.NewStore(),
		Now: func() time.Time { return time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC) }}, c
}

func do(t *testing.T, s *Server, method, path, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set(HeaderUser, "jane")
	req.Header.Set(HeaderSubmittedFrom, "horizon-cli")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	return rec
}

func TestCallerRouting(t *testing.T) {
	s, _ := newServer(t)
	if rec := do(t, s, "GET", "/api/v1/mirrors", "agent", ""); rec.Code != http.StatusForbidden {
		t.Fatalf("agent on public route: %d", rec.Code)
	}
	if rec := do(t, s, "PUT", "/internal/v1/runs/x/progress", "hapi", "{}"); rec.Code != http.StatusForbidden {
		t.Fatalf("horizon-api on internal route: %d", rec.Code)
	}
	if rec := do(t, s, "GET", "/api/v1/mirrors", "nope", ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("bad token: %d", rec.Code)
	}
	if rec := do(t, s, "GET", "/api/v1/mirrors", "hapi", ""); rec.Code != http.StatusOK {
		t.Fatalf("list: %d %s", rec.Code, rec.Body)
	}
}

func TestCreateSyncDelete(t *testing.T) {
	s, c := newServer(t)
	rec := do(t, s, "POST", "/api/v1/mirrors", "hapi", `{"name":"aosp-16","kind":"repo","repo":{"manifestRef":"android-16.0.0_r3"}}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", rec.Code, rec.Body)
	}
	var m smv1.StorageMirror
	if err := c.Get(context.Background(), client.ObjectKey{Namespace: "horizon", Name: "aosp-16"}, &m); err != nil {
		t.Fatal(err)
	}
	if m.Annotations[smv1.AnnotationCreatedBy] != "jane" || m.Annotations[controller.AnnotationSyncRequestID] == "" || m.Annotations[controller.AnnotationSubmittedFrom] != "horizon-cli" {
		t.Fatalf("annotations %v", m.Annotations)
	}
	if rec := do(t, s, "POST", "/api/v1/mirrors", "hapi", `{"name":"other","kind":"repo","directory":"aosp-16"}`); rec.Code != http.StatusConflict {
		t.Fatalf("directory reuse: %d", rec.Code)
	}
	rec = do(t, s, "POST", "/api/v1/mirrors/aosp-16/sync", "hapi", "")
	var res SyncResult
	_ = json.Unmarshal(rec.Body.Bytes(), &res)
	if rec.Code != http.StatusAccepted || res.RequestID == "" {
		t.Fatalf("sync: %d %s", rec.Code, rec.Body)
	}
	if rec := do(t, s, "PATCH", "/api/v1/mirrors/aosp-16", "hapi", `{"manifestUrl":"x"}`); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("immutable patch: %d", rec.Code)
	}
	if rec := do(t, s, "PATCH", "/api/v1/mirrors/aosp-16", "hapi", `{"manifestRef":"android-16.0.0_r4","syncJobs":4}`); rec.Code != http.StatusOK {
		t.Fatalf("patch: %d %s", rec.Code, rec.Body)
	}
	// The controller's finalizer keeps the CR visible after the delete.
	_ = c.Get(context.Background(), client.ObjectKey{Namespace: "horizon", Name: "aosp-16"}, &m)
	m.Finalizers = []string{smv1.Finalizer}
	_ = c.Update(context.Background(), &m)
	if rec := do(t, s, "DELETE", "/api/v1/mirrors/aosp-16?purge=true", "hapi", ""); rec.Code != http.StatusBadRequest {
		t.Fatalf("purge without confirm: %d", rec.Code)
	}
	if rec := do(t, s, "DELETE", "/api/v1/mirrors/aosp-16?purge=true&confirm=aosp-16", "hapi", ""); rec.Code != http.StatusAccepted {
		t.Fatalf("purge: %d %s", rec.Code, rec.Body)
	}
	_ = c.Get(context.Background(), client.ObjectKey{Namespace: "horizon", Name: "aosp-16"}, &m)
	if m.Annotations[smv1.AnnotationDeleteRequestedBy] != "jane" {
		t.Fatalf("delete submitter not recorded: %v", m.Annotations)
	}
}

func TestCreateRepoNeedsManifest(t *testing.T) {
	s, _ := newServer(t)
	// Config defaults carry a manifest URL but no ref.
	if rec := do(t, s, "POST", "/api/v1/mirrors", "hapi", `{"name":"aosp","kind":"repo"}`); rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("repo without manifestRef: %d %s", rec.Code, rec.Body)
	}
	if rec := do(t, s, "POST", "/api/v1/mirrors", "hapi", `{"name":"aosp","kind":"repo","repo":{"manifestRef":"main"}}`); rec.Code != http.StatusCreated {
		t.Fatalf("repo with ref from the request: %d %s", rec.Code, rec.Body)
	}
}

func TestSyncRefusedWhenSuspended(t *testing.T) {
	m := &smv1.StorageMirror{ObjectMeta: metav1.ObjectMeta{Name: "held", Namespace: "horizon"},
		Spec: smv1.StorageMirrorSpec{Kind: smv1.KindGit, Git: &smv1.GitSpec{URL: "https://example.com/h.git"}, Suspend: true}}
	s, c := newServer(t, m)
	if rec := do(t, s, "POST", "/api/v1/mirrors/held/sync", "hapi", ""); rec.Code != http.StatusConflict {
		t.Fatalf("sync of a suspended mirror: %d %s", rec.Code, rec.Body)
	}
	rec := do(t, s, "POST", "/api/v1/syncs", "hapi", `{"mirrors":["held"]}`)
	if rec.Code != http.StatusAccepted || !strings.Contains(rec.Body.String(), "suspended") {
		t.Fatalf("batch: %d %s", rec.Code, rec.Body)
	}
	var got smv1.StorageMirror
	_ = c.Get(context.Background(), client.ObjectKey{Namespace: "horizon", Name: "held"}, &got)
	if got.Annotations[smv1.AnnotationSyncRequestedAt] != "" {
		t.Fatal("a suspended mirror must not get a sync request")
	}
}

// TestLiveReadsBeforeMutations: routes that write or dispatch read through the API server, not the cache.
func TestLiveReadsBeforeMutations(t *testing.T) {
	m := &smv1.StorageMirror{ObjectMeta: metav1.ObjectMeta{Name: "fresh", Namespace: "horizon"},
		Spec: smv1.StorageMirrorSpec{Kind: smv1.KindGit, Git: &smv1.GitSpec{URL: "https://example.com/f.git"}}}
	s, c := newServer(t, m)
	// The "cache" has not seen the mirror yet.
	stale := interceptor.NewClient(c.(client.WithWatch), interceptor.Funcs{
		Get: func(ctx context.Context, cl client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			if key.Name == "fresh" {
				return apierrors.NewNotFound(smv1.GroupVersion.WithResource("storagemirrors").GroupResource(), key.Name)
			}
			return cl.Get(ctx, key, obj, opts...)
		},
	})
	s.Client, s.Reader, s.APIReader = stale, stale, c
	if rec := do(t, s, "POST", "/api/v1/mirrors/fresh/sync", "hapi", ""); rec.Code != http.StatusAccepted {
		t.Fatalf("sync read the cache: %d %s", rec.Code, rec.Body)
	}
}

func TestBuildMirrorValidation(t *testing.T) {
	for _, body := range []CreateRequest{
		{Name: "Bad_Name", Kind: "repo"},
		{Name: "g", Kind: "git", Git: &GitRequest{URL: "git@host:x"}},
		{Name: "r", Kind: "repo", Git: &GitRequest{URL: "https://x"}},
		{Name: "d", Kind: "repo", Directory: "../x"},
		{Name: "l", Kind: "repo", LFS: "sometimes"},
	} {
		if _, err := BuildMirror(body, "horizon"); err == nil {
			t.Errorf("expected rejection of %+v", body)
		}
	}
	m, err := BuildMirror(CreateRequest{Name: "k", Kind: "git", Git: &GitRequest{URL: "https://example.com/k.git", CredentialsSecretName: "c"}, LFS: "auto"}, "horizon")
	if err != nil || m.Spec.Kind != smv1.KindGit || m.Spec.LFS != smv1.LFSAuto || m.Spec.Git.CredentialsSecretRef.Name != "c" {
		t.Fatalf("%v %+v", err, m)
	}
}

func TestProgressRequiresModuleRun(t *testing.T) {
	wf := &unstructured.Unstructured{}
	wf.SetGroupVersionKind(controller.WorkflowGVK)
	wf.SetName("run-1")
	wf.SetNamespace("workflows")
	wf.SetLabels(map[string]string{smv1.LabelModule: smv1.ModuleName, smv1.LabelMirror: "aosp"})
	s, _ := newServer(t, wf)
	if rec := do(t, s, "PUT", "/internal/v1/runs/run-1/progress", "agent", `{"mirror":"other","progress":{}}`); rec.Code != http.StatusForbidden {
		t.Fatalf("wrong mirror: %d", rec.Code)
	}
	if rec := do(t, s, "PUT", "/internal/v1/runs/missing/progress", "agent", `{"mirror":"aosp","progress":{}}`); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown run: %d", rec.Code)
	}
	if rec := do(t, s, "PUT", "/internal/v1/runs/run-1/progress", "agent", `{"mirror":"aosp","progress":{"phase":"fetch","projectsDone":3}}`); rec.Code != http.StatusNoContent {
		t.Fatalf("progress: %d %s", rec.Code, rec.Body)
	}
	if rep, ok := s.Progress.Get("run-1"); !ok || rep.Progress.ProjectsDone != 3 {
		t.Fatal("progress not stored")
	}
}

func TestCatalogReportAndUnmanagedList(t *testing.T) {
	s, _ := newServer(t)
	body := `{"metadataYaml":"horizon-mirrors:\n  old:\n    type: repo\n    status: ready\n","usage":{"capacityBytes":10,"usedBytes":4,"availableBytes":6}}`
	if rec := do(t, s, "POST", "/internal/v1/catalog", "agent", body); rec.Code != http.StatusNoContent {
		t.Fatalf("catalog: %d %s", rec.Code, rec.Body)
	}
	rec := do(t, s, "GET", "/api/v1/mirrors", "hapi", "")
	if !strings.Contains(rec.Body.String(), `"managed":false`) || !strings.Contains(rec.Body.String(), `"name":"old"`) {
		t.Fatalf("unmanaged entry missing: %s", rec.Body)
	}
	if rec := do(t, s, "POST", "/api/v1/import", "hapi", `{}`); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"old"`) {
		t.Fatalf("import: %d %s", rec.Code, rec.Body)
	}
}

func TestGitMirrorNeedsReadOnlyJenkinsClaim(t *testing.T) {
	s, c := newServer(t)
	var cfg smv1.StorageMirrorConfig
	_ = c.Get(context.Background(), client.ObjectKey{Namespace: "horizon", Name: "default"}, &cfg)
	cfg.Spec.Consumers.Jenkins = smv1.JenkinsConsumer{Enabled: true, ReadOnly: false}
	_ = c.Update(context.Background(), &cfg)
	if rec := do(t, s, "POST", "/api/v1/mirrors", "hapi", `{"name":"k","kind":"git","git":{"url":"https://example.com/k.git"}}`); rec.Code != http.StatusConflict {
		t.Fatalf("got %d", rec.Code)
	}
}

func TestVolumeDestroyNeedsBothKeys(t *testing.T) {
	m := &smv1.StorageMirror{ObjectMeta: metav1.ObjectMeta{Name: "gone", Namespace: "horizon", Finalizers: []string{smv1.Finalizer}},
		Spec: smv1.StorageMirrorSpec{Kind: smv1.KindGit, Git: &smv1.GitSpec{URL: "https://example.com/g.git"}, DeletionPolicy: smv1.DeletionPolicyDelete}}
	cm := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "storage-mirror-catalog", Namespace: "horizon"}}
	s, c := newServer(t, m, cm)
	ctx := context.Background()
	if rec := do(t, s, "DELETE", "/api/v1/volume?confirm=sdv-storage-mirror", "hapi", ""); rec.Code != http.StatusForbidden {
		t.Fatalf("destroy without allowDestroy: %d", rec.Code)
	}
	var cfg smv1.StorageMirrorConfig
	_ = c.Get(ctx, client.ObjectKey{Namespace: "horizon", Name: "default"}, &cfg)
	cfg.Spec.Volume.AllowDestroy = true
	_ = c.Update(ctx, &cfg)
	if rec := do(t, s, "DELETE", "/api/v1/volume?confirm=wrong", "hapi", ""); rec.Code != http.StatusBadRequest {
		t.Fatalf("destroy with a wrong confirm: %d", rec.Code)
	}
	if rec := do(t, s, "DELETE", "/api/v1/volume?confirm=sdv-storage-mirror", "hapi", ""); rec.Code != http.StatusConflict {
		t.Fatalf("destroy with mirrors and no force: %d", rec.Code)
	}
	if rec := do(t, s, "DELETE", "/api/v1/volume?confirm=sdv-storage-mirror&force=true", "hapi", ""); rec.Code != http.StatusAccepted {
		t.Fatalf("forced destroy: %d %s", rec.Code, rec.Body)
	}
	var got smv1.StorageMirror
	if err := c.Get(ctx, client.ObjectKey{Namespace: "horizon", Name: "gone"}, &got); err != nil {
		t.Fatal(err)
	}
	// The finalizer stays for the controller, which releases Retain mirrors without a delete run.
	if got.DeletionTimestamp.IsZero() || got.Spec.DeletionPolicy != smv1.DeletionPolicyRetain {
		t.Fatalf("mirror not released as Retain: %+v", got)
	}
	if err := c.Get(ctx, client.ObjectKey{Namespace: "horizon", Name: "storage-mirror-catalog"}, &corev1.ConfigMap{}); err == nil {
		t.Fatal("catalog snapshot of the destroyed share kept")
	}
}
