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
	"errors"
	"strings"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

const testEnvironmentImage = "us-central1-docker.pkg.dev/proj/horizon-sdv/cloud-ws-workloads/cws-workflow:latest"

type imageResult struct {
	exists bool
	err    error
}

// fakeImages returns results in order and repeats the last one.
type fakeImages struct {
	results []imageResult
	calls   int
}

func (f *fakeImages) Exists(_ context.Context, image string) (bool, error) {
	if image != testEnvironmentImage {
		return false, errors.New("unexpected image " + image)
	}
	i := min(f.calls, len(f.results)-1)
	f.calls++
	return f.results[i].exists, f.results[i].err
}

func testTemplate() *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(workflowTemplateGVK)
	u.SetName(bootstrapTemplateName)
	u.SetNamespace("workflows")
	return u
}

func testBootstrapWorkflow(name, phase string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(workflowGVK)
	u.SetName(name)
	u.SetNamespace("workflows")
	u.SetLabels(map[string]string{bootstrapLabel: "true"})
	if phase != "" {
		_ = unstructured.SetNestedField(u.Object, phase, "status", "phase")
	}
	return u
}

func newTestBootstrap(images ImageChecker, c client.Client) *EnvironmentImageBootstrap {
	return &EnvironmentImageBootstrap{
		Client:           c,
		APIReader:        c,
		Images:           images,
		Image:            testEnvironmentImage,
		Namespace:        "workflows",
		TemplateInterval: time.Millisecond,
		RetryFirst:       time.Millisecond,
		RetryCeiling:     2 * time.Millisecond,
		RetryWindow:      20 * time.Millisecond,
	}
}

func bootstrapWorkflows(t *testing.T, c client.Client) []unstructured.Unstructured {
	t.Helper()
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(workflowListGVK)
	if err := c.List(context.Background(), list, client.InNamespace("workflows"), client.MatchingLabels{bootstrapLabel: "true"}); err != nil {
		t.Fatalf("list workflows: %v", err)
	}
	return list.Items
}

func TestEnvironmentImageBootstrap_SubmitsWhenImageMissing(t *testing.T) {
	c := fake.NewClientBuilder().WithObjects(testTemplate()).Build()
	b := newTestBootstrap(&fakeImages{results: []imageResult{{exists: false}}}, c)

	if err := b.Start(context.Background()); err != nil {
		t.Fatalf("Start returned %v", err)
	}

	items := bootstrapWorkflows(t, c)
	if len(items) != 1 {
		t.Fatalf("submitted %d workflows, want 1", len(items))
	}
	wf := items[0]
	if name, _, _ := unstructured.NestedString(wf.Object, "spec", "workflowTemplateRef", "name"); name != bootstrapTemplateName {
		t.Fatalf("workflowTemplateRef.name = %q", name)
	}
	if !strings.HasPrefix(wf.GetName(), bootstrapGenerateName) {
		t.Fatalf("workflow name = %q, want prefix %q", wf.GetName(), bootstrapGenerateName)
	}

	got := map[string]string{}
	params, _, _ := unstructured.NestedSlice(wf.Object, "spec", "arguments", "parameters")
	for _, p := range params {
		m := p.(map[string]any)
		got[m["name"].(string)] = m["value"].(string)
	}
	want := map[string]string{
		"buildEnvironmentImage": "true",
		"imageTag":              "latest",
		"buildPreflight":        "false",
		"buildGnome":            "false",
		"buildCodeOss":          "false",
		"buildAndroidStudio":    "false",
		"buildAsfp":             "false",
		"dryRun":                "false",
	}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("parameter %s = %q, want %q", k, got[k], v)
		}
	}
	if len(got) != len(want) {
		t.Fatalf("parameters = %v, want only %v", got, want)
	}
}

func TestEnvironmentImageBootstrap_SkipsWhenImagePresent(t *testing.T) {
	c := fake.NewClientBuilder().WithObjects(testTemplate()).Build()
	b := newTestBootstrap(&fakeImages{results: []imageResult{{exists: true}}}, c)

	if err := b.Start(context.Background()); err != nil {
		t.Fatalf("Start returned %v", err)
	}
	if n := len(bootstrapWorkflows(t, c)); n != 0 {
		t.Fatalf("submitted %d workflows, want 0", n)
	}
}

func TestEnvironmentImageBootstrap_SkipsWhenBuildInProgress(t *testing.T) {
	for _, phase := range []string{"", "Pending", "Running"} {
		t.Run("phase="+phase, func(t *testing.T) {
			running := testBootstrapWorkflow("cws-environment-image-bootstrap-abc", phase)
			c := fake.NewClientBuilder().WithObjects(testTemplate(), running).Build()
			b := newTestBootstrap(&fakeImages{results: []imageResult{{exists: false}}}, c)

			if err := b.Start(context.Background()); err != nil {
				t.Fatalf("Start returned %v", err)
			}
			items := bootstrapWorkflows(t, c)
			if len(items) != 1 || items[0].GetName() != running.GetName() {
				t.Fatalf("workflows = %d, want only the existing one", len(items))
			}
		})
	}
}

func TestEnvironmentImageBootstrap_SubmitsAfterFinishedBuild(t *testing.T) {
	for _, phase := range []string{"Succeeded", "Failed", "Error"} {
		t.Run("phase="+phase, func(t *testing.T) {
			finished := testBootstrapWorkflow("cws-environment-image-bootstrap-old", phase)
			c := fake.NewClientBuilder().WithObjects(testTemplate(), finished).Build()
			b := newTestBootstrap(&fakeImages{results: []imageResult{{exists: false}}}, c)

			if err := b.Start(context.Background()); err != nil {
				t.Fatalf("Start returned %v", err)
			}
			if n := len(bootstrapWorkflows(t, c)); n != 2 {
				t.Fatalf("workflows = %d, want 2 (finished one plus the new submit)", n)
			}
		})
	}
}

func TestEnvironmentImageBootstrap_RetriesRegistryErrorsThenSubmits(t *testing.T) {
	c := fake.NewClientBuilder().WithObjects(testTemplate()).Build()
	images := &fakeImages{results: []imageResult{
		{err: errors.New("HTTP 403")},
		{err: errors.New("HTTP 503")},
		{exists: false},
	}}
	b := newTestBootstrap(images, c)

	if err := b.Start(context.Background()); err != nil {
		t.Fatalf("Start returned %v", err)
	}
	if images.calls != 3 {
		t.Fatalf("registry calls = %d, want 3", images.calls)
	}
	if n := len(bootstrapWorkflows(t, c)); n != 1 {
		t.Fatalf("workflows = %d, want 1", n)
	}
}

func TestEnvironmentImageBootstrap_NeverSubmitsOnUncertainAnswer(t *testing.T) {
	c := fake.NewClientBuilder().WithObjects(testTemplate()).Build()
	images := &fakeImages{results: []imageResult{{err: errors.New("HTTP 403")}}}
	b := newTestBootstrap(images, c)

	if err := b.Start(context.Background()); err != nil {
		t.Fatalf("Start returned %v", err)
	}
	if images.calls < 2 {
		t.Fatalf("registry calls = %d, want retries", images.calls)
	}
	if n := len(bootstrapWorkflows(t, c)); n != 0 {
		t.Fatalf("workflows = %d, want 0", n)
	}
}

func TestEnvironmentImageBootstrap_WaitsForTemplate(t *testing.T) {
	c := fake.NewClientBuilder().Build()
	images := &fakeImages{results: []imageResult{{exists: false}}}
	b := newTestBootstrap(images, c)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if err := b.Start(ctx); err != nil {
		t.Fatalf("Start returned %v", err)
	}
	if images.calls != 0 {
		t.Fatalf("registry checked before the template existed (%d calls)", images.calls)
	}
	if n := len(bootstrapWorkflows(t, c)); n != 0 {
		t.Fatalf("workflows = %d, want 0", n)
	}
}

func TestEnvironmentImageBootstrap_CreateFailureDoesNotReturnError(t *testing.T) {
	c := fake.NewClientBuilder().WithObjects(testTemplate()).WithInterceptorFuncs(interceptor.Funcs{
		Create: func(context.Context, client.WithWatch, client.Object, ...client.CreateOption) error {
			return errors.New("forbidden")
		},
	}).Build()
	b := newTestBootstrap(&fakeImages{results: []imageResult{{exists: false}}}, c)

	if err := b.Start(context.Background()); err != nil {
		t.Fatalf("Start returned %v", err)
	}
	if n := len(bootstrapWorkflows(t, c)); n != 0 {
		t.Fatalf("workflows = %d, want 0", n)
	}
}

func TestRetryUntil_StopsAtWindow(t *testing.T) {
	calls := 0
	err := retryUntil(context.Background(), time.Millisecond, time.Millisecond, 10*time.Millisecond, func() error {
		calls++
		return errors.New("still failing")
	})
	if err == nil || calls < 2 {
		t.Fatalf("err = %v, calls = %d", err, calls)
	}
}
