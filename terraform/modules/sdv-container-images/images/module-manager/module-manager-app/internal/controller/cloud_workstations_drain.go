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
	"slices"
	"sort"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	// CloudWorkstationsModuleName is the catalog name of the Cloud Workstations module.
	CloudWorkstationsModuleName  = "cloud-workstations"
	cloudWorkstationsNamespace   = "cloud-workstations"
	cloudWorkstationsCoreAppName = "mod-cloud-workstations-core"

	cloudWorkstationsHorizonGroup = "workstations.horizon-sdv.io"
	cloudWorkstationsKCCGroup     = "workstations.cnrm.cloud.google.com"
)

// pauseCloudWorkstationsForDrain sets skip-reconcile on the parent, then on the core Application, so neither
// Git nor a parent sync can recreate what the drain deletes. The parent goes first so it cannot reapply the
// core Application in between. On error everything already paused is resumed.
func pauseCloudWorkstationsForDrain(ctx context.Context, c client.Client, argocdNamespace, moduleName string) error {
	if moduleName != CloudWorkstationsModuleName {
		return nil
	}
	if err := pauseCloudWorkstationsApplications(ctx, c, argocdNamespace, moduleName); err != nil {
		if resumeErr := resumeCloudWorkstationsAfterFailedDrain(ctx, c, argocdNamespace, moduleName); resumeErr != nil {
			return fmt.Errorf("pause Cloud Workstations Applications for drain: %w; resume: %v", err, resumeErr)
		}
		return fmt.Errorf("pause Cloud Workstations Applications for drain: %w", err)
	}
	return nil
}

func pauseCloudWorkstationsApplications(ctx context.Context, c client.Client, argocdNamespace, moduleName string) error {
	for _, appName := range []string{ApplicationName(moduleName), cloudWorkstationsCoreAppName} {
		if err := terminateArgoCDApplicationOperationIfAny(ctx, c, argocdNamespace, appName); err != nil {
			return err
		}
		if err := setArgoCDApplicationSkipReconcile(ctx, c, argocdNamespace, appName, true); err != nil {
			return err
		}
	}
	return nil
}

// resumeCloudWorkstationsCoreCleanupTimeout bounds the fresh context the resume helpers use for their own
// API calls, independent of the caller's ctx deadline/cancellation.
const resumeCloudWorkstationsCoreCleanupTimeout = 30 * time.Second

// resumeCloudWorkstationsAfterFailedDrain clears the pause on the core and parent Applications after a
// disable error. selfHeal is never changed, so Argo CD manages both again. The module is still enabled.
//
// This deliberately does not use the caller's ctx for its own API calls. It runs on the error path of a
// long drain (up to cwsDrainStageTimeout per stage) that, for the REST disable handler, is driven by
// r.Context() - cancelled the moment the client/proxy disconnects. Reusing that ctx here would make the
// resume call fail immediately with the same cancellation/deadline that triggered this cleanup in the
// first place, leaving the Applications stuck paused with no recovery. A fresh, short-lived context
// gives the cleanup a real chance to reach the API server even when the triggering ctx is already dead.
func resumeCloudWorkstationsAfterFailedDrain(ctx context.Context, c client.Client, argocdNamespace, moduleName string) error {
	if moduleName != CloudWorkstationsModuleName {
		return nil
	}
	cleanupCtx, cancel := context.WithTimeout(context.Background(), resumeCloudWorkstationsCoreCleanupTimeout)
	defer cancel()
	var errs []error
	for _, appName := range []string{cloudWorkstationsCoreAppName, ApplicationName(moduleName)} {
		if err := setArgoCDApplicationSkipReconcile(cleanupCtx, c, argocdNamespace, appName, false); err != nil {
			errs = append(errs, fmt.Errorf("resume %q: %w", appName, err))
		}
	}
	if err := stderrors.Join(errs...); err != nil {
		return fmt.Errorf("resume Cloud Workstations Applications after failed disable: %w", err)
	}
	return nil
}

// ResumeCloudWorkstationsAfterRestart clears a pause left by a disable that a restart aborted. While the
// module is still enabled the paused parent also locks the Portal switch. It is a no-op when the module is
// disabled, or when a lifecycle operation holds ModuleOpsMutex, which only a live disable in this process can.
func ResumeCloudWorkstationsAfterRestart(ctx context.Context, c client.Client, stateStore StateStoreInterface, argocdNamespace string) error {
	if !ModuleOpsMutex.TryLock() {
		return nil
	}
	defer ModuleOpsMutex.Unlock()

	state, err := stateStore.Get(ctx)
	if err != nil {
		return err
	}
	if id := state.ModuleIDs[CloudWorkstationsModuleName]; id == "" || !slices.Contains(state.EnabledModules, id) {
		return nil
	}
	return resumeCloudWorkstationsAfterFailedDrain(ctx, c, argocdNamespace, CloudWorkstationsModuleName)
}

// completeCloudWorkstationsCoreDrain deletes the paused core Application, then resumes it so its finalizer
// can prune. Argo CD does not sync an Application that is being deleted, so the drained cluster stays gone.
func completeCloudWorkstationsCoreDrain(ctx context.Context, c client.Client, argocdNamespace, moduleName string) error {
	if moduleName != CloudWorkstationsModuleName {
		return nil
	}
	app := &unstructured.Unstructured{}
	app.SetGroupVersionKind(argoCDApplicationGVK)
	key := client.ObjectKey{Namespace: argocdNamespace, Name: cloudWorkstationsCoreAppName}
	if err := c.Get(ctx, key, app); err != nil {
		return client.IgnoreNotFound(err)
	}
	if app.GetDeletionTimestamp().IsZero() {
		if err := c.Delete(ctx, app); err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("delete Cloud Workstations core Application: %w", err)
		}
	}
	if err := setArgoCDApplicationSkipReconcile(ctx, c, argocdNamespace, cloudWorkstationsCoreAppName, false); err != nil {
		return fmt.Errorf("resume deleted Cloud Workstations core Application: %w", err)
	}
	return nil
}

// resumeCloudWorkstationsParentAfterDelete clears skip-reconcile on the deleted parent Application so its
// finalizer can prune. The state already says disabled and nothing can retry this, so it uses a fresh context.
func resumeCloudWorkstationsParentAfterDelete(ctx context.Context, c client.Client, argocdNamespace, moduleName string) error {
	if moduleName != CloudWorkstationsModuleName {
		return nil
	}
	cleanupCtx, cancel := context.WithTimeout(context.Background(), resumeCloudWorkstationsCoreCleanupTimeout)
	defer cancel()
	if err := setArgoCDApplicationSkipReconcile(cleanupCtx, c, argocdNamespace, ApplicationName(moduleName), false); err != nil {
		return fmt.Errorf("resume deleted Cloud Workstations parent Application: %w", err)
	}
	return nil
}

// logCloudWorkstationsResumeError logs a resume failure on paths that already return another error or
// cannot be retried through the disable API. Each Application gets its own command so a missing one
// does not hide the others.
func logCloudWorkstationsResumeError(ctx context.Context, argocdNamespace string, err error, appNames ...string) {
	if err == nil {
		return
	}
	cmds := make([]string, 0, len(appNames))
	for _, name := range appNames {
		cmds = append(cmds, fmt.Sprintf("kubectl -n %s annotate application %s %s-", argocdNamespace, name, argoCDSkipReconcileAnnotationKey))
	}
	log.FromContext(ctx).Error(err, "Cloud Workstations Application left paused", "manual", strings.Join(cmds, " ; "))
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
	if moduleName != CloudWorkstationsModuleName {
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
