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
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"cloud-workstations-module/internal/artifactregistry"
)

const (
	bootstrapLabel        = "horizon-sdv.io/cws-environment-bootstrap"
	bootstrapTemplateName = "cws-image-build"
	bootstrapGenerateName = "cws-environment-image-bootstrap-"
)

var (
	workflowGVK         = schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "Workflow"}
	workflowListGVK     = workflowGVK.GroupVersion().WithKind("WorkflowList")
	workflowTemplateGVK = schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "WorkflowTemplate"}
)

// bootstrapParameters builds only the environment image. Every other image stays a manual run.
var bootstrapParameters = []struct{ name, value string }{
	{"buildEnvironmentImage", "true"},
	{"buildPreflight", "false"},
	{"buildGnome", "false"},
	{"buildCodeOss", "false"},
	{"buildAndroidStudio", "false"},
	{"buildAsfp", "false"},
	{"dryRun", "false"},
}

// ImageChecker reports whether a tagged image exists in the registry.
type ImageChecker interface {
	Exists(ctx context.Context, image string) (bool, error)
}

// EnvironmentImageBootstrap submits one cws-image-build run when the environment image is missing.
// It runs as a leader-only manager runnable, not on the reconcile path, so enable and sync never wait for it.
// Start always returns nil: a returned error would stop the whole manager.
type EnvironmentImageBootstrap struct {
	Client    client.Client
	APIReader client.Reader
	Images    ImageChecker
	Image     string
	Namespace string

	TemplateInterval time.Duration
	RetryFirst       time.Duration
	RetryCeiling     time.Duration
	RetryWindow      time.Duration

	// MaxFailedBuilds stops automatic submits once this many bootstrap builds ended Failed or Error. 0 disables the cap.
	MaxFailedBuilds int
}

// NewEnvironmentImageBootstrap uses the production intervals.
func NewEnvironmentImageBootstrap(c client.Client, reader client.Reader, images ImageChecker, image, namespace string) *EnvironmentImageBootstrap {
	return &EnvironmentImageBootstrap{
		Client:           c,
		APIReader:        reader,
		Images:           images,
		Image:            image,
		Namespace:        namespace,
		TemplateInterval: 30 * time.Second,
		RetryFirst:       30 * time.Second,
		RetryCeiling:     5 * time.Minute,
		RetryWindow:      30 * time.Minute,
		MaxFailedBuilds:  3,
	}
}

// NeedLeaderElection keeps a single replica from submitting twice.
func (b *EnvironmentImageBootstrap) NeedLeaderElection() bool { return true }

func (b *EnvironmentImageBootstrap) Start(ctx context.Context) error {
	logger := log.FromContext(ctx).WithName("environment-image-bootstrap")

	if err := b.waitForTemplate(ctx); err != nil {
		return nil
	}

	var exists bool
	err := retryUntil(ctx, b.RetryFirst, b.RetryCeiling, b.RetryWindow, func() error {
		var err error
		exists, err = b.Images.Exists(ctx, b.Image)
		return err
	})
	if err != nil {
		if ctx.Err() == nil {
			logger.Error(err, "could not check the environment image, not submitting a build", "image", b.Image)
		}
		return nil
	}
	if exists {
		logger.Info("environment image present, no build needed", "image", b.Image)
		return nil
	}

	running, failed, err := b.bootstrapState(ctx)
	if err != nil {
		logger.Error(err, "could not list environment image builds, not submitting a build")
		return nil
	}
	if running != "" {
		logger.Info("environment image build already in progress", "workflow", running)
		return nil
	}
	if b.MaxFailedBuilds > 0 && failed >= b.MaxFailedBuilds {
		logger.Error(nil, "environment image builds keep failing, not submitting another; run cws-image-build with buildEnvironmentImage=true, or delete the failed bootstrap Workflows to retry",
			"failedBuilds", failed, "limit", b.MaxFailedBuilds, "image", b.Image)
		return nil
	}

	name, err := b.submit(ctx)
	if err != nil {
		logger.Error(err, "submitting environment image build failed", "image", b.Image)
		return nil
	}
	logger.Info("submitted environment image build", "workflow", name, "image", b.Image)
	return nil
}

func (b *EnvironmentImageBootstrap) waitForTemplate(ctx context.Context) error {
	logger := log.FromContext(ctx).WithName("environment-image-bootstrap")
	for {
		tmpl := &unstructured.Unstructured{}
		tmpl.SetGroupVersionKind(workflowTemplateGVK)
		err := b.APIReader.Get(ctx, client.ObjectKey{Namespace: b.Namespace, Name: bootstrapTemplateName}, tmpl)
		if err == nil {
			return nil
		}
		if !apierrors.IsNotFound(err) && !apimeta.IsNoMatchError(err) {
			logger.Info("waiting for WorkflowTemplate", "template", bootstrapTemplateName, "reason", err.Error())
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(b.TemplateInterval):
		}
	}
}

// bootstrapState reads live state. It returns the name of a build still in progress (a Workflow with
// no phase yet counts as in progress) and how many bootstrap Workflows ended Failed or Error. Argo
// removes finished Workflows after its ttlStrategy, which lifts the failure cap on its own.
func (b *EnvironmentImageBootstrap) bootstrapState(ctx context.Context) (string, int, error) {
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(workflowListGVK)
	if err := b.APIReader.List(ctx, list,
		client.InNamespace(b.Namespace),
		client.MatchingLabels{bootstrapLabel: "true"},
	); err != nil {
		return "", 0, err
	}
	failed := 0
	for i := range list.Items {
		phase := workflowPhase(&list.Items[i])
		if !finishedWorkflowPhase(phase) {
			return list.Items[i].GetName(), 0, nil
		}
		if phase == "Failed" || phase == "Error" {
			failed++
		}
	}
	return "", failed, nil
}

func (b *EnvironmentImageBootstrap) submit(ctx context.Context) (string, error) {
	ref, err := artifactregistry.ParseRef(b.Image)
	if err != nil {
		return "", err
	}

	params := make([]any, 0, len(bootstrapParameters)+1)
	params = append(params, map[string]any{"name": "imageTag", "value": ref.Tag})
	for _, p := range bootstrapParameters {
		params = append(params, map[string]any{"name": p.name, "value": p.value})
	}

	wf := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": workflowGVK.GroupVersion().String(),
		"kind":       workflowGVK.Kind,
		"metadata": map[string]any{
			"generateName": bootstrapGenerateName,
			"namespace":    b.Namespace,
			"labels":       map[string]any{bootstrapLabel: "true"},
		},
		"spec": map[string]any{
			"workflowTemplateRef": map[string]any{"name": bootstrapTemplateName},
			"arguments":           map[string]any{"parameters": params},
		},
	}}
	if err := b.Client.Create(ctx, wf); err != nil {
		return "", err
	}
	return wf.GetName(), nil
}

func workflowPhase(wf *unstructured.Unstructured) string {
	phase, _, _ := unstructured.NestedString(wf.Object, "status", "phase")
	return phase
}

func finishedWorkflowPhase(phase string) bool {
	switch phase {
	case "Succeeded", "Failed", "Error":
		return true
	}
	return false
}

// retryUntil calls fn until it succeeds, backing off from first up to ceiling, and gives up after window.
func retryUntil(ctx context.Context, first, ceiling, window time.Duration, fn func() error) error {
	deadline := time.Now().Add(window)
	delay := first
	for {
		err := fn()
		if err == nil {
			return nil
		}
		if time.Now().Add(delay).After(deadline) {
			return err
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
		delay = min(2*delay, ceiling)
	}
}
