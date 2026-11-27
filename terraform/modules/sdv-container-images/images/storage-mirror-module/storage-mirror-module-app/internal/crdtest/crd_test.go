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

// Package crdtest checks the module CRDs (OpenAPI and CEL rules) and the rendered WorkflowTemplates against a real
// kube-apiserver (controller-runtime envtest). It is skipped unless KUBEBUILDER_ASSETS points at envtest binaries:
//
//	setup-envtest use 1.35.x --bin-dir /tmp/envtest -p path
//	KUBEBUILDER_ASSETS=<that path> go test ./internal/crdtest/
//
// With ARGO_CRDS_DIR (Argo Workflows manifests/base/crds/full) and SM_RENDERED_TEMPLATES (helm template output of
// gitops/modules/storage-mirror-module/argo-workflows) it also applies the WorkflowTemplates with strict field
// validation, which checks workflowMetadata.labelsFrom and the plural synchronization fields against Argo's schema.
package crdtest

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"

	smv1 "storage-mirror-module/api/v1alpha1"
)

func repoFile(t *testing.T, rel string) string {
	t.Helper()
	dir, _ := os.Getwd()
	for i := 0; i < 12 && dir != "/"; i++ {
		p := filepath.Join(dir, rel)
		if _, err := os.Stat(p); err == nil {
			return p
		}
		dir = filepath.Dir(dir)
	}
	t.Skipf("%s not found", rel)
	return ""
}

func startAPIServer(t *testing.T) client.Client {
	t.Helper()
	if os.Getenv("KUBEBUILDER_ASSETS") == "" {
		t.Skip("KUBEBUILDER_ASSETS not set (see package doc)")
	}
	ctrllog.SetLogger(logr.Discard())
	dirs := []string{repoFile(t, "gitops/modules/storage-mirror-module/crds")}
	if d := os.Getenv("ARGO_CRDS_DIR"); d != "" {
		dirs = append(dirs, d)
	}
	env := &envtest.Environment{CRDDirectoryPaths: dirs, ErrorIfCRDPathMissing: true}
	cfg, err := env.Start()
	if err != nil {
		t.Fatalf("envtest: %v", err)
	}
	t.Cleanup(func() { _ = env.Stop() })
	s := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(s)
	_ = smv1.AddToScheme(s)
	c, err := client.New(cfg, client.Options{Scheme: s})
	if err != nil {
		t.Fatal(err)
	}
	for _, ns := range []string{"horizon", "workflows"} {
		if err := c.Create(context.Background(), &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns}}); err != nil {
			t.Fatal(err)
		}
	}
	return c
}

func validConfig() *smv1.StorageMirrorConfig {
	return &smv1.StorageMirrorConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "default", Namespace: "horizon"},
		Spec:       smv1.StorageMirrorConfigSpec{Volume: smv1.VolumeSpec{CapacityGb: 2048, Zone: "europe-west1-d", Network: "sdv-network"}},
	}
}

func mustFail(t *testing.T, what string, err error, contains string) {
	t.Helper()
	if err == nil {
		t.Errorf("%s: accepted, want rejection", what)
		return
	}
	if contains != "" && !strings.Contains(err.Error(), contains) {
		t.Errorf("%s: %v (want %q)", what, err, contains)
	}
}

func TestConfigCEL(t *testing.T) {
	c := startAPIServer(t)
	ctx := context.Background()

	other := validConfig()
	other.Name = "other"
	mustFail(t, "config not named default", c.Create(ctx, other), "must be named default")
	small := validConfig()
	small.Spec.Volume.CapacityGb = 1000
	mustFail(t, "capacity below 1024", c.Create(ctx, small), "")
	odd := validConfig()
	odd.Spec.Volume.CapacityGb = 1100
	mustFail(t, "capacity not a multiple of 256", c.Create(ctx, odd), "")
	for _, gb := range []int64{10000, 10496, 11000} {
		step := validConfig()
		step.Spec.Volume.CapacityGb = gb
		mustFail(t, fmt.Sprintf("capacity %d off the zonal steps", gb), c.Create(ctx, step), "multiple of 2560")
	}
	nested := validConfig()
	nested.Spec.Layout.RootSubdir = "a/b"
	mustFail(t, "rootSubdir with a slash", c.Create(ctx, nested), "rootSubdir")

	cfg := validConfig()
	if err := c.Create(ctx, cfg); err != nil {
		t.Fatalf("valid config: %v", err)
	}
	if cfg.Spec.Volume.ResourceID != "sdv-storage-mirror" || cfg.Spec.Layout.RootSubdir != "horizon-mirrors" || cfg.Spec.Defaults.LFS != "Disabled" {
		t.Errorf("defaults not applied: %+v", cfg.Spec)
	}
	cfg.Spec.Volume.CapacityGb = 1792
	mustFail(t, "capacity decrease", c.Update(ctx, cfg), "cannot be decreased")
	_ = c.Get(ctx, client.ObjectKeyFromObject(cfg), cfg)
	cfg.Spec.Volume.CapacityGb = 2304
	if err := c.Update(ctx, cfg); err != nil {
		t.Errorf("capacity increase: %v", err)
	}
	_ = c.Get(ctx, client.ObjectKeyFromObject(cfg), cfg)
	cfg.Spec.Volume.CapacityGb = 12800
	if err := c.Update(ctx, cfg); err != nil {
		t.Errorf("capacity increase to 12800 (2560 step): %v", err)
	}
	_ = c.Get(ctx, client.ObjectKeyFromObject(cfg), cfg)
	cfg.Spec.Layout.RootSubdir = "other"
	mustFail(t, "layout change", c.Update(ctx, cfg), "layout is immutable")
	_ = c.Get(ctx, client.ObjectKeyFromObject(cfg), cfg)
	cfg.Spec.Volume.ResourceID = "renamed"
	mustFail(t, "resourceID change", c.Update(ctx, cfg), "resourceID is immutable")
}

func TestMirrorCEL(t *testing.T) {
	c := startAPIServer(t)
	ctx := context.Background()
	repo := func(name string) *smv1.StorageMirror {
		return &smv1.StorageMirror{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "horizon"},
			Spec:       smv1.StorageMirrorSpec{Kind: smv1.KindRepo, Repo: &smv1.RepoSpec{ManifestURL: "https://example.com/manifest"}},
		}
	}

	both := repo("both")
	both.Spec.Git = &smv1.GitSpec{URL: "https://example.com/x.git"}
	mustFail(t, "repo kind with git", c.Create(ctx, both), "must match spec.kind")
	gitNoSpec := &smv1.StorageMirror{ObjectMeta: metav1.ObjectMeta{Name: "g0", Namespace: "horizon"}, Spec: smv1.StorageMirrorSpec{Kind: smv1.KindGit}}
	mustFail(t, "git kind without git", c.Create(ctx, gitNoSpec), "must match spec.kind")
	insecure := &smv1.StorageMirror{ObjectMeta: metav1.ObjectMeta{Name: "g1", Namespace: "horizon"}, Spec: smv1.StorageMirrorSpec{Kind: smv1.KindGit, Git: &smv1.GitSpec{URL: "http://example.com/x.git"}}}
	mustFail(t, "git over http", c.Create(ctx, insecure), "")
	reserved := repo("reserved")
	reserved.Spec.Directory = "metadata.yaml"
	mustFail(t, "reserved directory", c.Create(ctx, reserved), "reserved")
	escape := repo("escape")
	escape.Spec.Directory = "../x"
	mustFail(t, "path escape", c.Create(ctx, escape), "")
	badKind := repo("badkind")
	badKind.Spec.Kind = "Svn"
	mustFail(t, "unknown kind", c.Create(ctx, badKind), "")

	m := repo("aosp")
	m.Spec.Directory = "My_Mirror"
	if err := c.Create(ctx, m); err != nil {
		t.Fatalf("valid mirror: %v", err)
	}
	if m.Spec.DeletionPolicy != "Retain" || m.Spec.LFS != "Inherit" || m.Spec.Schedule != "Inherit" {
		t.Errorf("defaults not applied: %+v", m.Spec)
	}
	m.Spec.Repo.ManifestURL = "https://example.com/other"
	mustFail(t, "manifestUrl change", c.Update(ctx, m), "manifestUrl is immutable")
	_ = c.Get(ctx, client.ObjectKeyFromObject(m), m)
	m.Spec.Directory = "renamed"
	mustFail(t, "directory change", c.Update(ctx, m), "directory is immutable")
	_ = c.Get(ctx, client.ObjectKeyFromObject(m), m)
	m.Spec.Kind = smv1.KindGit
	m.Spec.Repo = nil
	m.Spec.Git = &smv1.GitSpec{URL: "https://example.com/x.git"}
	mustFail(t, "kind change", c.Update(ctx, m), "kind is immutable")
	_ = c.Get(ctx, client.ObjectKeyFromObject(m), m)
	m.Spec.Repo.ManifestRef = "android-16.0.0_r4"
	m.Spec.Suspend = true
	if err := c.Update(ctx, m); err != nil {
		t.Errorf("mutable fields: %v", err)
	}
	m.Status.Phase = smv1.PhaseReady
	m.Status.CurrentRun = &smv1.RunStatus{WorkflowName: "w", Progress: &smv1.Progress{Phase: "fetch", OverallPercent: 40}}
	if err := c.Status().Update(ctx, m); err != nil {
		t.Errorf("status subresource: %v", err)
	}
}

// TestWorkflowTemplatesAgainstArgoSchema applies the rendered templates with strict field validation.
func TestWorkflowTemplatesAgainstArgoSchema(t *testing.T) {
	rendered := os.Getenv("SM_RENDERED_TEMPLATES")
	if os.Getenv("ARGO_CRDS_DIR") == "" || rendered == "" {
		t.Skip("ARGO_CRDS_DIR and SM_RENDERED_TEMPLATES not set")
	}
	c := startAPIServer(t)
	b, err := os.ReadFile(rendered)
	if err != nil {
		t.Fatal(err)
	}
	dec := utilyaml.NewYAMLOrJSONDecoder(bytes.NewReader(b), 4096)
	n := 0
	for {
		u := &unstructured.Unstructured{}
		if err := dec.Decode(&u.Object); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			t.Fatal(err)
		}
		if u.GetKind() != "WorkflowTemplate" {
			continue
		}
		u.SetNamespace("workflows")
		if err := c.Create(context.Background(), u, client.FieldValidation("Strict")); err != nil {
			t.Errorf("%s: %v", u.GetName(), err)
		}
		n++
	}
	if n != 4 {
		t.Fatalf("applied %d WorkflowTemplates, want 4", n)
	}
}
