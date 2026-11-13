// Copyright (c) 2024-2026 Accenture, All Rights Reserved.
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

package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/acn-horizon-sdv/module-manager/internal/api"
	"github.com/acn-horizon-sdv/module-manager/internal/controller"
)

const labelModuleManagerManaged = "horizon-sdv.io/module-manager-managed"

// moduleHelmStartup runs once after cache warm-up: heals OpenAPI-placeholder Git refs in
// ModuleManagerState and parent Argo CD Applications, then merges MODULE_CONFIG into Helm values.
type moduleHelmStartup struct {
	client       client.Client
	apiReader    client.Reader
	argocdNS     string
	repoURL      string
	defaultRev   string
	moduleConfig string
	stateStore   controller.StateStoreInterface
}

func (s *moduleHelmStartup) Start(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return nil
	case <-time.After(5 * time.Second):
	}

	def := strings.TrimSpace(s.defaultRev)
	if def == "" {
		def = "HEAD"
	}

	if s.stateStore != nil {
		if err := controller.ResumeCloudWorkstationsAfterRestart(ctx, s.client, s.stateStore, s.argocdNS); err != nil {
			log.Printf("module-manager startup: resume Cloud Workstations Applications: %v", err)
		}
		if err := s.healStatePlaceholderRevisions(ctx, def); err != nil {
			log.Printf("module-manager startup: heal ModuleManagerState target revisions: %v", err)
		}
	}

	if err := s.healApplicationPlaceholderRevisions(ctx, def); err != nil {
		log.Printf("module-manager startup: heal Argo CD Application target revisions: %v", err)
	}

	cfg := strings.TrimSpace(s.moduleConfig)
	if cfg == "" {
		return nil
	}

	// Only parent mod-* Applications take MODULE_CONFIG. Child Applications (app-role=child) are
	// rendered by the parent's chart; rewriting their helm values here makes the parent OutOfSync
	// against its own rendering (the Developer Portal then shows UPDATE IN PROGRESS indefinitely,
	// because parents sync with prune but without selfHeal).
	ul := &unstructured.UnstructuredList{}
	ul.SetGroupVersionKind(schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "ApplicationList"})
	if err := s.apiReader.List(ctx, ul,
		client.InNamespace(s.argocdNS),
		client.MatchingLabels{
			labelModuleManagerManaged:               "true",
			controller.ModuleManagerAppRoleLabelKey: controller.ModuleManagerAppRoleParent,
		},
	); err != nil {
		return fmt.Errorf("list module-manager-managed parent Applications: %w", err)
	}
	for i := range ul.Items {
		name := ul.Items[i].GetName()
		if err := controller.SyncApplicationHelmValuesConfig(ctx, s.client, s.apiReader, s.argocdNS, name, cfg); err != nil {
			return fmt.Errorf("sync MODULE_CONFIG into Application %q: %w", name, err)
		}
	}
	return nil
}

func (s *moduleHelmStartup) healStatePlaceholderRevisions(ctx context.Context, def string) error {
	st, err := s.stateStore.Get(ctx)
	if err != nil {
		return err
	}
	if st.ModuleTargetRevisions == nil {
		return nil
	}
	changed := false
	for k, v := range st.ModuleTargetRevisions {
		if controller.IsOpenAPIExamplePlaceholderRevision(v) {
			st.ModuleTargetRevisions[k] = def
			changed = true
		}
	}
	if !changed {
		return nil
	}
	if err := s.stateStore.Update(ctx, st); err != nil {
		return err
	}
	s.stateStore.InvalidateCache()
	return nil
}

func (s *moduleHelmStartup) healApplicationPlaceholderRevisions(ctx context.Context, def string) error {
	ul := &unstructured.UnstructuredList{}
	ul.SetGroupVersionKind(schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "ApplicationList"})
	if err := s.apiReader.List(ctx, ul,
		client.InNamespace(s.argocdNS),
		client.MatchingLabels{
			labelModuleManagerManaged: "true",
			"horizon-sdv.io/app-role": "parent",
		},
	); err != nil {
		return fmt.Errorf("list parent module Applications: %w", err)
	}
	for i := range ul.Items {
		app := &ul.Items[i]
		rev, _, err := unstructured.NestedString(app.Object, "spec", "source", "targetRevision")
		if err != nil || !controller.IsOpenAPIExamplePlaceholderRevision(rev) {
			continue
		}
		mod := strings.TrimSpace(app.GetLabels()["horizon-sdv.io/module"])
		if mod == "" {
			log.Printf("module-manager startup: skip Application %q: placeholder targetRevision but no horizon-sdv.io/module label", app.GetName())
			continue
		}
		if err := api.PatchApplicationTargetRevision(ctx, s.client, s.argocdNS, app.GetName(), s.repoURL, def, mod, s.moduleConfig, ""); err != nil {
			return fmt.Errorf("patch target revision for Application %q: %w", app.GetName(), err)
		}
		log.Printf("module-manager startup: repaired placeholder Git ref on Application %q (module %q)", app.GetName(), mod)
	}
	return nil
}
