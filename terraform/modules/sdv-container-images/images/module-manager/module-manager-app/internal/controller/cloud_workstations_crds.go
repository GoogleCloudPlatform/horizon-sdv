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
	"fmt"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Vars (not consts) so tests can shorten them. Neither value has been measured on a live cluster.
var (
	cwsParentAppDeleteWait = 10 * time.Minute
	cwsCRDDeleteWait       = 5 * time.Minute
)

var cwsCRDGVK = schema.GroupVersionKind{Group: "apiextensions.k8s.io", Version: "v1", Kind: "CustomResourceDefinition"}

// cloudWorkstationsHorizonCRDNames are the module's own Horizon CRDs. The Config Connector CRDs
// (workstations.cnrm.cloud.google.com) are platform-owned and must never be listed here; the
// module-manager-horizon-workstations ClusterRole restricts delete to exactly these names.
var cloudWorkstationsHorizonCRDNames = []string{
	"workstationclusters." + cloudWorkstationsHorizonGroup,
	"workstationconfigs." + cloudWorkstationsHorizonGroup,
	"workstations." + cloudWorkstationsHorizonGroup,
}

// RemoveCloudWorkstationsCRDs deletes the three Horizon Cloud Workstations CRDs after a module disable so that
// a disabled module leaves nothing behind. Argo CD does not remove them with the parent Application.
//
// It first waits for the parent Application to be gone (the controller is pruned with it), then deletes the
// CRDs and waits for them to disappear. It must only run after DrainCloudWorkstationsModule: deleting a CRD
// deletes every custom resource of that kind, and custom resources with finalizers keep the CRD Terminating
// until they are gone. Finalizers are never stripped. It is a no-op for every module other than cloud-workstations.
//
// It runs after the module state already says disabled, when nothing can re-run it (the disable API answers
// "already disabled"). It therefore ignores cancellation of the caller's context, such as an HTTP client that
// disconnected during the long teardown, and is bounded by its own budget instead.
func RemoveCloudWorkstationsCRDs(ctx context.Context, c client.Client, argocdNamespace, moduleName string) error {
	if moduleName != cloudWorkstationsModuleName {
		return nil
	}
	logger := log.FromContext(ctx).WithName("cloud-workstations-crds")
	if ctx.Err() != nil {
		logger.Info("caller context cancelled; continuing Horizon CRD cleanup detached from it", "reason", ctx.Err().Error())
	}

	budget := cwsParentAppDeleteWait + time.Duration(len(cloudWorkstationsHorizonCRDNames))*cwsCRDDeleteWait + time.Minute
	detached, cancel := context.WithTimeout(context.WithoutCancel(ctx), budget)
	defer cancel()

	if err := removeCloudWorkstationsCRDs(detached, c, argocdNamespace, moduleName); err != nil {
		logger.Error(err, "Horizon Cloud Workstations CRD cleanup failed")
		return err
	}
	return nil
}

func removeCloudWorkstationsCRDs(ctx context.Context, c client.Client, argocdNamespace, moduleName string) error {
	logger := log.FromContext(ctx).WithName("cloud-workstations-crds")
	manual := "kubectl delete crd " + strings.Join(cloudWorkstationsHorizonCRDNames, " ")

	appName := ApplicationName(moduleName)
	present, err := waitForObjectAbsent(ctx, c, argoCDApplicationGVK, client.ObjectKey{Namespace: argocdNamespace, Name: appName}, cwsParentAppDeleteWait)
	if err != nil {
		return fmt.Errorf("wait for parent Application %q to be deleted: %w", appName, err)
	}
	if present {
		return fmt.Errorf("timed out after %s waiting for parent Application %q to be deleted; Horizon CRDs were not removed (once it is gone: %s)",
			cwsParentAppDeleteWait, appName, manual)
	}

	for _, name := range cloudWorkstationsHorizonCRDNames {
		crd := &unstructured.Unstructured{}
		crd.SetGroupVersionKind(cwsCRDGVK)
		crd.SetName(name)
		if err := c.Delete(ctx, crd); err != nil && !errors.IsNotFound(err) && !meta.IsNoMatchError(err) {
			return fmt.Errorf("delete CRD %q: %w", name, err)
		}
	}
	for _, name := range cloudWorkstationsHorizonCRDNames {
		present, err := waitForObjectAbsent(ctx, c, cwsCRDGVK, client.ObjectKey{Name: name}, cwsCRDDeleteWait)
		if err != nil {
			return fmt.Errorf("wait for CRD %q to be removed: %w", name, err)
		}
		if present {
			return fmt.Errorf("timed out after %s waiting for CRD %q to be removed; custom resources of this kind still exist and their finalizers are not stripped (list them with: kubectl get %s -A)",
				cwsCRDDeleteWait, name, name)
		}
	}
	logger.Info("Horizon Cloud Workstations CRDs removed", "crds", cloudWorkstationsHorizonCRDNames)
	return nil
}

// waitForObjectAbsent polls until the object is NotFound (or its kind is not served), returning
// stillPresent=true if the timeout elapses first. Errors other than NotFound/NoMatch are returned.
func waitForObjectAbsent(ctx context.Context, c client.Client, gvk schema.GroupVersionKind, key client.ObjectKey, timeout time.Duration) (stillPresent bool, err error) {
	deadline := time.Now().Add(timeout)
	for {
		u := &unstructured.Unstructured{}
		u.SetGroupVersionKind(gvk)
		err := c.Get(ctx, key, u)
		if errors.IsNotFound(err) || meta.IsNoMatchError(err) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		if !time.Now().Before(deadline) {
			return true, nil
		}
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-time.After(cwsDrainPollInterval):
		}
	}
}
