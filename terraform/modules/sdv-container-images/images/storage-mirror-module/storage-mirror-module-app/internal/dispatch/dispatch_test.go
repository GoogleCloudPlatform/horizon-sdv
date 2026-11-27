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

package dispatch

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	smv1 "storage-mirror-module/api/v1alpha1"
)

func TestBodyCarriesEveryParameter(t *testing.T) {
	d := &Dispatcher{WorkflowNamespace: "workflows"}
	body, err := d.Body(Request{Template: TemplateSync, SubmittedBy: "jane", Params: map[string]string{"requestId": "r1", "mirror": "aosp"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range MirrorParams {
		if _, ok := body[n]; !ok {
			t.Errorf("missing %s", n)
		}
	}
	if body["workflowTemplateName"] != TemplateSync || body["horizonModule"] != "storage-mirror" || body["horizonSubmittedFrom"] != "api" {
		t.Errorf("unexpected envelope: %v", body)
	}
}

func TestBodyRejectsMultiline(t *testing.T) {
	d := &Dispatcher{}
	if _, err := d.Body(Request{Template: TemplateDelete, Params: map[string]string{"requestId": "r", "mirror": "a\nb"}}); err == nil {
		t.Fatal("expected error")
	}
	if _, err := d.Body(Request{Template: "other"}); err == nil {
		t.Fatal("expected unknown template error")
	}
}

func TestDispatchPosts(t *testing.T) {
	var got map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&got)
	}))
	defer srv.Close()
	d := &Dispatcher{WebhookURL: srv.URL, WorkflowNamespace: "workflows"}
	id, err := d.Dispatch(context.Background(), Request{Template: TemplateVolumeInit})
	if err != nil || id == "" {
		t.Fatalf("dispatch: %v %q", err, id)
	}
	if got["requestId"] != id || got["workflowTemplateNamespace"] != "workflows" {
		t.Fatalf("body %v", got)
	}
}

func TestMirrorRunParamsDefaults(t *testing.T) {
	cfg := &smv1.StorageMirrorConfigSpec{
		Defaults: smv1.MirrorDefaults{
			Repo: smv1.RepoDefaults{ManifestURL: "https://android.googlesource.com/platform/manifest", ManifestRef: "android-16.0.0_r3", SyncJobs: 3},
			Git:  smv1.GitDefaults{Branches: []string{"*"}, ExcludeRefs: []string{"refs/pull/*"}},
			LFS:  smv1.LFSDisabled,
		},
		Retention: smv1.RetentionSpec{GC: "Aggressive", PruneRefs: []string{"refs/changes/*"}},
	}
	repo := &smv1.StorageMirror{ObjectMeta: metav1.ObjectMeta{Name: "aosp"}, Spec: smv1.StorageMirrorSpec{Kind: smv1.KindRepo, Repo: &smv1.RepoSpec{ManifestRef: "main"}}}
	p := MirrorRunParams(repo, cfg)
	if p["manifestUrl"] != cfg.Defaults.Repo.ManifestURL || p["manifestRef"] != "main" || p["manifestFile"] != "default.xml" || p["syncJobs"] != "3" || p["syncTimeout"] != "20h" || p["lfs"] != "disabled" || p["directory"] != "aosp" {
		t.Fatalf("repo params %v", p)
	}
	git := &smv1.StorageMirror{ObjectMeta: metav1.ObjectMeta{Name: "k"}, Spec: smv1.StorageMirrorSpec{Kind: smv1.KindGit, Directory: "kernel", LFS: smv1.LFSAuto, Git: &smv1.GitSpec{URL: "https://example.com/k.git", CredentialsSecretRef: &smv1.SecretRef{Name: "creds"}}}}
	p = MirrorRunParams(git, cfg)
	if p["kind"] != "git" || p["gitBranches"] != "*" || p["gitCredentialsSecret"] != "creds" || p["lfs"] != "auto" || p["syncTimeout"] != "6h" || p["directory"] != "kernel" || p["excludeRefs"] != "refs/pull/*" {
		t.Fatalf("git params %v", p)
	}
}
