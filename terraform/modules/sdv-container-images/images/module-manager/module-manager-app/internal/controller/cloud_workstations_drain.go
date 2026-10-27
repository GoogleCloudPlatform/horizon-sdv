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
	stderrors "errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	cloudWorkstationsModuleName  = "cloud-workstations"
	cloudWorkstationsNamespace   = "cloud-workstations"
	cloudWorkstationsCoreAppName = "mod-cloud-workstations-core"

	cloudWorkstationsHorizonGroup = "workstations.horizon-sdv.io"
	cloudWorkstationsKCCGroup     = "workstations.cnrm.cloud.google.com"
)

// pauseCloudWorkstationsCoreForDrain prevents Argo CD self-heal from recreating resources while the
// module drain deletes them. A missing core Application is already drained and is therefore a no-op.
func pauseCloudWorkstationsCoreForDrain(ctx context.Context, c client.Client, argocdNamespace, moduleName string) error {
	if moduleName != cloudWorkstationsModuleName {
		return nil
	}
	if err := setArgoCDApplicationSkipReconcile(ctx, c, argocdNamespace, cloudWorkstationsCoreAppName, true); err != nil {
		return fmt.Errorf("pause Cloud Workstations core Application for drain: %w", err)
	}
	return nil
}

// completeCloudWorkstationsCoreDrain turns self-heal off before resuming reconciliation. This allows the
// parent Application deletion to prune the core Application without recreating the drained cluster.
func completeCloudWorkstationsCoreDrain(ctx context.Context, c client.Client, argocdNamespace, moduleName string) error {
	if moduleName != cloudWorkstationsModuleName {
		return nil
	}
	app := &unstructured.Unstructured{}
	app.SetGroupVersionKind(argoCDApplicationGVK)
	key := client.ObjectKey{Namespace: argocdNamespace, Name: cloudWorkstationsCoreAppName}
	if err := c.Get(ctx, key, app); err != nil {
		return client.IgnoreNotFound(err)
	}
	patch := client.RawPatch(types.MergePatchType, []byte(`{"spec":{"syncPolicy":{"automated":{"selfHeal":false}}}}`))
	if err := c.Patch(ctx, app, patch); err != nil {
		return fmt.Errorf("disable self-heal on Cloud Workstations core Application: %w", err)
	}
	if err := setArgoCDApplicationSkipReconcile(ctx, c, argocdNamespace, cloudWorkstationsCoreAppName, false); err != nil {
		return fmt.Errorf("resume Cloud Workstations core Application after drain: %w", err)
	}
	return nil
}

// Vars (not consts) so tests can shorten them. Cluster deletion on GCP alone was observed at ~16 minutes,
// so each stage gets 20 minutes.
var (
	cwsDrainPollInterval = 10 * time.Second
	cwsDrainStageTimeout = 20 * time.Minute
)

type cwsDrainStage struct {
	kind       string
	horizonGVK schema.GroupVersionKind
	kccGVK     schema.GroupVersionKind
}

// cloudWorkstationsDrainStages is ordered child-first: a Workstation must be fully gone (Horizon and KCC)
// before its WorkstationConfig is deleted, and every config before its WorkstationCluster.
var cloudWorkstationsDrainStages = []cwsDrainStage{
	newCWSDrainStage("Workstation"),
	newCWSDrainStage("WorkstationConfig"),
	newCWSDrainStage("WorkstationCluster"),
}

func newCWSDrainStage(kind string) cwsDrainStage {
	return cwsDrainStage{
		kind:       kind,
		horizonGVK: schema.GroupVersionKind{Group: cloudWorkstationsHorizonGroup, Version: "v1alpha1", Kind: kind},
		kccGVK:     schema.GroupVersionKind{Group: cloudWorkstationsKCCGroup, Version: "v1beta1", Kind: kind},
	}
}

type cwsStageTimeoutError struct {
	message string
}

func (e *cwsStageTimeoutError) Error() string { return e.message }

func isCWSStageTimeout(err error) bool {
	var timeout *cwsStageTimeoutError
	return stderrors.As(err, &timeout)
}

// DrainCloudWorkstationsModule deletes the Horizon Workstation, WorkstationConfig and WorkstationCluster CRs
// in the cloud-workstations namespace, child first, and waits until both the Horizon objects and their
// owner-linked KCC objects are gone before starting the next stage.
//
// It must run while the Cloud Workstations controller is still running: only the controller releases the
// Horizon finalizers and drives KCC/GCP deletion. Finalizers are never stripped; on timeout an error naming
// the blocking objects is returned and nothing else is touched, so the caller can leave the module enabled
// and retry. It is a no-op for every module other than cloud-workstations.
func DrainCloudWorkstationsModule(ctx context.Context, c client.Client, moduleName string) error {
	return drainCloudWorkstations(ctx, c, moduleName, false)
}

// DrainCloudWorkstationsForPlatformTeardown is the platform-destruction path.
// A stuck Config Connector finalizer must not abort teardown of the whole platform.
// Single-module disable keeps using DrainCloudWorkstationsModule.
func DrainCloudWorkstationsForPlatformTeardown(ctx context.Context, c client.Client, moduleName string) error {
	return drainCloudWorkstations(ctx, c, moduleName, true)
}

func drainCloudWorkstations(ctx context.Context, c client.Client, moduleName string, bestEffort bool) error {
	if moduleName != cloudWorkstationsModuleName {
		return nil
	}
	for _, st := range cloudWorkstationsDrainStages {
		if err := drainCloudWorkstationsStage(ctx, c, st, bestEffort); err != nil {
			return err
		}
	}
	return nil
}

func drainCloudWorkstationsStage(ctx context.Context, c client.Client, st cwsDrainStage, bestEffort bool) error {
	logger := log.FromContext(ctx).WithName("cloud-workstations-drain")

	err := waitCloudWorkstationsStage(ctx, c, st)
	if err == nil || !bestEffort || !isCWSStageTimeout(err) {
		return err
	}

	logger.Info("platform teardown stripping stuck Config Connector finalizers", "kind", st.kind, "namespace", cloudWorkstationsNamespace)
	if err := stripDeletingCloudWorkstationsFinalizer(ctx, c, st.kccGVK, kccFinalizer); err != nil {
		return err
	}
	err = waitCloudWorkstationsStage(ctx, c, st)
	if err == nil || !isCWSStageTimeout(err) {
		return err
	}

	logger.Info("platform teardown releasing stuck Horizon finalizers", "kind", st.kind, "namespace", cloudWorkstationsNamespace)
	return stripDeletingCloudWorkstationsFinalizer(ctx, c, st.horizonGVK, "")
}

func waitCloudWorkstationsStage(ctx context.Context, c client.Client, st cwsDrainStage) error {
	logger := log.FromContext(ctx).WithName("cloud-workstations-drain")
	deadline := time.Now().Add(cwsDrainStageTimeout)
	var last string
	for {
		if err := deleteUndeletedCloudWorkstations(ctx, c, st); err != nil {
			return err
		}
		blocking, err := cloudWorkstationsStageBlockers(ctx, c, st)
		if err != nil {
			return err
		}
		if len(blocking) == 0 {
			if last != "" {
				logger.Info("stage drained", "kind", st.kind, "namespace", cloudWorkstationsNamespace)
			}
			return nil
		}
		desc := strings.Join(blocking, ", ")
		if desc != last {
			logger.Info("waiting for Horizon/KCC objects to be removed", "kind", st.kind, "namespace", cloudWorkstationsNamespace, "remaining", desc)
			last = desc
		}
		if !time.Now().Before(deadline) {
			return &cwsStageTimeoutError{message: fmt.Sprintf("timed out after %s waiting for %s objects to be removed from namespace %q; still present: %s",
				cwsDrainStageTimeout, st.kind, cloudWorkstationsNamespace, desc)}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(cwsDrainPollInterval):
		}
	}
}

func deleteUndeletedCloudWorkstations(ctx context.Context, c client.Client, st cwsDrainStage) error {
	items, err := listCloudWorkstationsKind(ctx, c, st.horizonGVK)
	if err != nil {
		return fmt.Errorf("list %s in namespace %q: %w", st.kind, cloudWorkstationsNamespace, err)
	}
	for i := range items {
		if items[i].GetDeletionTimestamp() != nil {
			continue
		}
		if err := c.Delete(ctx, &items[i]); err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("delete %s %q in namespace %q: %w", st.kind, items[i].GetName(), cloudWorkstationsNamespace, err)
		}
	}
	return nil
}

// stripDeletingCloudWorkstationsFinalizer removes one finalizer, or every finalizer when drop is empty,
// from objects of gvk that are already being deleted.
func stripDeletingCloudWorkstationsFinalizer(ctx context.Context, c client.Client, gvk schema.GroupVersionKind, drop string) error {
	items, err := listCloudWorkstationsKind(ctx, c, gvk)
	if err != nil {
		return err
	}
	for i := range items {
		item := &items[i]
		if item.GetDeletionTimestamp().IsZero() {
			continue
		}
		kept, changed := withoutFinalizer(item.GetFinalizers(), drop)
		if !changed {
			continue
		}
		item.SetFinalizers(kept)
		if err := c.Update(ctx, item); err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("strip finalizer on %s %q: %w", gvk.Kind, item.GetName(), err)
		}
	}
	return nil
}

func withoutFinalizer(in []string, drop string) ([]string, bool) {
	if drop == "" {
		return nil, len(in) > 0
	}
	kept := make([]string, 0, len(in))
	changed := false
	for _, f := range in {
		if f == drop {
			changed = true
			continue
		}
		kept = append(kept, f)
	}
	return kept, changed
}

// cloudWorkstationsStageBlockers returns the sorted names of Horizon and KCC objects of this stage's kind
// that still exist in the cloud-workstations namespace.
func cloudWorkstationsStageBlockers(ctx context.Context, c client.Client, st cwsDrainStage) ([]string, error) {
	var out []string
	horizon, err := listCloudWorkstationsKind(ctx, c, st.horizonGVK)
	if err != nil {
		return nil, fmt.Errorf("list %s in namespace %q: %w", st.kind, cloudWorkstationsNamespace, err)
	}
	for i := range horizon {
		out = append(out, st.kind+"/"+horizon[i].GetName())
	}
	kcc, err := listCloudWorkstationsKind(ctx, c, st.kccGVK)
	if err != nil {
		return nil, fmt.Errorf("list KCC %s in namespace %q: %w", st.kind, cloudWorkstationsNamespace, err)
	}
	for i := range kcc {
		out = append(out, "KCC "+st.kind+"/"+kcc[i].GetName())
	}
	sort.Strings(out)
	return out, nil
}

// listCloudWorkstationsKind lists objects of gvk in the cloud-workstations namespace. A missing CRD
// (NoMatch) or NotFound means there is nothing to drain.
func listCloudWorkstationsKind(ctx context.Context, c client.Client, gvk schema.GroupVersionKind) ([]unstructured.Unstructured, error) {
	ul := &unstructured.UnstructuredList{}
	ul.SetGroupVersionKind(schema.GroupVersionKind{Group: gvk.Group, Version: gvk.Version, Kind: gvk.Kind + "List"})
	if err := c.List(ctx, ul, client.InNamespace(cloudWorkstationsNamespace)); err != nil {
		if meta.IsNoMatchError(err) || errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return ul.Items, nil
}
