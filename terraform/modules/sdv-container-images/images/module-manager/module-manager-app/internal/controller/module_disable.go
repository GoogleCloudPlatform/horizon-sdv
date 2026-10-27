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

package controller

import (
	"context"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// PerformModuleDisable deletes the Argo CD Application and updates ModuleManagerState to
// reflect a disabled module. It does not refresh dependents or resync soft features;
// callers should invoke RunAutoDisableSweep and ResyncSoftFeaturesForParentsOfSoftDep
// after disable side effects as appropriate.
//
// For workloads-android and remotive-topology, TeardownPrefixedModuleChildApplication removes the child
// Application and clears ComputeInstanceTemplate CRs in the module's KCC namespace ({prefix}workflows resp.
// {prefix}remotive-kcc; CNRM deletes matching GCP instance templates when those CRs are removed). For
// workloads-android it also waits for ConfigConnectorContext to be absent so the cluster matches the
// pre-enable / pre–cf_instance_template baseline.
//
// For cloud-workstations, DrainCloudWorkstationsModule runs first: it deletes Horizon Workstations, then
// WorkstationConfigs, then WorkstationClusters (waiting for the KCC objects too) while the controller is still
// running, and fails without touching state or the parent Application if that times out. After the parent
// Application is gone, RemoveCloudWorkstationsCRDs deletes the module's three Horizon CRDs.
//
// Ordering note (vs platform drain): this path updates ModuleManagerState before deleting the parent
// mod-* Application so the Portal API reflects disabled during long Argo prunes. PlatformDrainer
// deletes the parent mod-* Application before updating state (whole-platform teardown).
//
// After clearParentSkipReconcile, deletePrefixedChildApplicationIfPresent runs so a Git reconcile cannot
// leave workloads-android / workloads-common child Applications healthy while the module is disabled.
//
// When enforceNoHardDependents is true, ListHardDependents must be empty or the call fails.
// REST disable sets this to true. Auto-disable sets it to false because eligibility is
// determined by hard- and soft-dependent checks before this runs.
//
// moduleConfig is Helm MODULE_CONFIG YAML/JSON (may be empty). For workloads-android, workloads-common and
// remotive-topology, the prefixed multi-source child Application is torn down first; empty moduleConfig falls
// back to MODULE_CONFIG env.
func PerformModuleDisable(ctx context.Context, c client.Client, stateStore StateStoreInterface, catalogStore CatalogStoreInterface, argocdNamespace, mmNamespace string, moduleName, moduleID string, enforceNoHardDependents bool, moduleConfig string) error {
	logger := log.FromContext(ctx)
	if moduleName == "" || moduleID == "" {
		return fmt.Errorf("perform module disable: module name and module id are required")
	}

	state, err := stateStore.Get(ctx)
	if err != nil {
		return err
	}
	if enforceNoHardDependents {
		deps, err := ListHardDependents(ctx, c, catalogStore, mmNamespace, state, moduleName)
		if err != nil {
			return err
		}
		if len(deps) > 0 {
			return fmt.Errorf("perform module disable: module %q still has hard dependents %v", moduleName, deps)
		}
	}

	// Drain Horizon CRs while the controller (core child) is still running; it alone releases their finalizers.
	// A failure here leaves the module enabled and the parent Application untouched, so disable can be retried.
	if err := pauseCloudWorkstationsCoreForDrain(ctx, c, argocdNamespace, moduleName); err != nil {
		return fmt.Errorf("perform module disable: %w", err)
	}
	if err := DrainCloudWorkstationsModule(ctx, c, moduleName); err != nil {
		return fmt.Errorf("perform module disable: drain cloud-workstations CRs: %w", err)
	}
	if err := completeCloudWorkstationsCoreDrain(ctx, c, argocdNamespace, moduleName); err != nil {
		return fmt.Errorf("perform module disable: %w", err)
	}

	if err := TeardownPrefixedModuleChildApplication(ctx, c, argocdNamespace, moduleConfig, moduleName); err != nil {
		return fmt.Errorf("perform module disable: teardown child Application for %q: %w", moduleName, err)
	}
	// Teardown sets skip-reconcile on the parent mod-* Application so Git cannot recreate the child during delete.
	// Clear it before we delete the parent: otherwise Argo does not prune managed resources (overview Deployment,
	// etc.) and resources-finalizer.argocd.argoproj.io keeps mod-* stuck in Deleting indefinitely.
	clearParentSkipReconcileIfPrefixedModule(ctx, c, argocdNamespace, moduleName)
	if err := deletePrefixedChildApplicationIfPresent(ctx, c, argocdNamespace, moduleConfig, moduleName); err != nil {
		return fmt.Errorf("perform module disable: remove recreated child Application for %q: %w", moduleName, err)
	}

	// Persist disabled state before deleting the parent Application so GET /modules reports enabled=false while the
	// parent Argo app still prunes (Synced + Progressing, e.g. KCC/PreDelete). Otherwise the Developer Portal maps
	// that Argo phase to "INSTALLATION IN PROGRESS" because ModuleManagerState still showed enabled until the end.
	var newEnabled []string
	for _, id := range state.EnabledModules {
		if id != moduleID {
			newEnabled = append(newEnabled, id)
		}
	}
	state.EnabledModules = newEnabled
	if state.ModuleTargetRevisions != nil {
		delete(state.ModuleTargetRevisions, moduleName)
	}
	if err := stateStore.Update(ctx, state); err != nil {
		clearParentSkipReconcileIfPrefixedModule(ctx, c, argocdNamespace, moduleName)
		return err
	}

	appName := ApplicationName(moduleName)
	app := &unstructured.Unstructured{}
	app.SetGroupVersionKind(schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "Application"})
	app.SetNamespace(argocdNamespace)
	app.SetName(appName)
	logger.Info("disabling module", "module", moduleName, "moduleID", moduleID, "application", appName, "enforceNoHardDependents", enforceNoHardDependents)
	if err := c.Delete(ctx, app); err != nil && !errors.IsNotFound(err) {
		clearParentSkipReconcileIfPrefixedModule(ctx, c, argocdNamespace, moduleName)
		return err
	}
	if err := deletePrefixedChildApplicationIfPresent(ctx, c, argocdNamespace, moduleConfig, moduleName); err != nil {
		return fmt.Errorf("perform module disable: remove child Application after parent delete for %q: %w", moduleName, err)
	}

	// State already says disabled, so a failure here cannot be retried through the disable API (it reports
	// "already disabled"). Log it and continue so the dependent sweep still runs.
	if err := RemoveCloudWorkstationsCRDs(ctx, c, argocdNamespace, moduleName); err != nil {
		logger.Error(err, "Horizon CRD cleanup failed after the module was disabled",
			"manual", "kubectl delete crd "+strings.Join(cloudWorkstationsHorizonCRDNames, " "))
	}
	return nil
}

// DisableModuleAndRefresh performs the shared disable workflow used by the REST API:
// disable the module, refresh dependents, then resync soft-feature parents.
// moduleConfig is passed to PerformModuleDisable (prefixed child teardown for workloads-android / workloads-common).
func DisableModuleAndRefresh(ctx context.Context, apiReader client.Reader, c client.Client, stateStore StateStoreInterface, catalogStore CatalogStoreInterface, argocdNamespace, mmNamespace, moduleName, moduleID string, enforceNoHardDependents bool, moduleConfig string) error {
	if err := PerformModuleDisable(ctx, c, stateStore, catalogStore, argocdNamespace, mmNamespace, moduleName, moduleID, enforceNoHardDependents, moduleConfig); err != nil {
		return err
	}
	if err := RunAutoDisableSweep(ctx, apiReader, c, mmNamespace, argocdNamespace, stateStore, catalogStore); err != nil {
		return fmt.Errorf("refresh dependents after disabling %q: %w", moduleName, err)
	}
	if err := ResyncSoftFeaturesForParentsOfSoftDep(ctx, apiReader, c, argocdNamespace, mmNamespace, stateStore, catalogStore, moduleName); err != nil {
		return fmt.Errorf("resync soft features after disabling %q: %w", moduleName, err)
	}
	return nil
}
