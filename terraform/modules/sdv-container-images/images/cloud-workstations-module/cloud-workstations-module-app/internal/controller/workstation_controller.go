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
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	wsv1alpha1 "cloud-workstations-module/api/v1alpha1"
	"cloud-workstations-module/internal/wsapi"
)

const (
	WorkstationFinalizer = "workstations.horizon-sdv.io/workstation-finalizer"

	reasonConfigDeleting = "ConfigDeleting"
	reasonConfigNotFound = "ConfigNotFound"

	// A Workstation applied just before its config (for example with kubectl) gets this long for the
	// config to appear before it is removed.
	defaultConfigNotFoundGrace = time.Minute
)

var (
	kccWorkstationGVK = schema.GroupVersionKind{
		Group:   "workstations.cnrm.cloud.google.com",
		Version: "v1beta1",
		Kind:    "Workstation",
	}
)

// WorkstationReconciler reconciles a Workstation object.
type WorkstationReconciler struct {
	client.Client
	APIReader      client.Reader
	Recorder       events.EventRecorder
	Scheme         *runtime.Scheme
	WS             wsapi.API
	ResyncInterval time.Duration
	// ConfigNotFoundGrace is how long a new Workstation waits for a missing config. Zero means the default.
	ConfigNotFoundGrace time.Duration
}

// +kubebuilder:rbac:groups=workstations.horizon-sdv.io,resources=workstations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=workstations.horizon-sdv.io,resources=workstations/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=workstations.horizon-sdv.io,resources=workstations/finalizers,verbs=update
// +kubebuilder:rbac:groups=workstations.cnrm.cloud.google.com,resources=workstations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=workstations.cnrm.cloud.google.com,resources=workstations/status,verbs=get
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch

func (r *WorkstationReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("workstation", req.NamespacedName)
	logger.V(1).Info("reconciling Workstation")

	var ws wsv1alpha1.Workstation
	if err := r.Get(ctx, req.NamespacedName, &ws); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Handle deletion
	if !ws.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, &ws)
	}

	// Runs before the finalizer is added: a Workstation left waiting on a deleting config would
	// block that config's delete. Until the KCC Workstation exists the config is read live, because
	// a cached read can miss a delete that started a moment ago.
	provisioned, err := r.kccWorkstationExists(ctx, &ws)
	if err != nil {
		return ctrl.Result{}, err
	}
	parentConfig, unusable, err := r.resolveParentConfig(ctx, &ws, !provisioned)
	if err != nil {
		return ctrl.Result{}, err
	}
	if unusable != "" && !provisioned {
		if unusable == reasonConfigNotFound {
			if remaining := r.configNotFoundGrace() - time.Since(ws.CreationTimestamp.Time); remaining > 0 {
				return r.waitForConfig(ctx, &ws, remaining)
			}
		}
		return ctrl.Result{}, r.removeUnprovisioned(ctx, &ws, unusable)
	}

	// Ensure finalizer
	if !controllerutil.ContainsFinalizer(&ws, WorkstationFinalizer) {
		base := ws.DeepCopy()
		controllerutil.AddFinalizer(&ws, WorkstationFinalizer)
		if err := r.Patch(ctx, &ws, finalizerPatch(base)); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Reconcile KCC object
	return r.reconcileCreateOrUpdate(ctx, &ws, parentConfig)
}

func (r *WorkstationReconciler) kccWorkstationExists(ctx context.Context, ws *wsv1alpha1.Workstation) (bool, error) {
	kccObj := &unstructured.Unstructured{}
	kccObj.SetGroupVersionKind(kccWorkstationGVK)
	err := r.Get(ctx, client.ObjectKeyFromObject(ws), kccObj)
	if apierrors.IsNotFound(err) || meta.IsNoMatchError(err) {
		return false, nil
	}
	return err == nil, err
}

// resolveParentConfig returns the parent config, or nil when it does not exist, and the reason it
// cannot take a new workstation (reasonConfigDeleting, reasonConfigNotFound), or "" when it can.
// live reads through APIReader; a nil APIReader falls back to the cached Client.
func (r *WorkstationReconciler) resolveParentConfig(ctx context.Context, ws *wsv1alpha1.Workstation, live bool) (*wsv1alpha1.WorkstationConfig, string, error) {
	var reader client.Reader = r.Client
	if live && r.APIReader != nil {
		reader = r.APIReader
	}
	cfg := &wsv1alpha1.WorkstationConfig{}
	err := reader.Get(ctx, client.ObjectKey{Namespace: ws.Namespace, Name: ws.Spec.ConfigRef.Name}, cfg)
	switch {
	case apierrors.IsNotFound(err):
		return nil, reasonConfigNotFound, nil
	case err != nil:
		return nil, "", err
	case !cfg.DeletionTimestamp.IsZero():
		return cfg, reasonConfigDeleting, nil
	}
	return cfg, "", nil
}

func (r *WorkstationReconciler) configNotFoundGrace() time.Duration {
	if r.ConfigNotFoundGrace > 0 {
		return r.ConfigNotFoundGrace
	}
	return defaultConfigNotFoundGrace
}

// waitForConfig holds a new Workstation while its config does not exist yet. No finalizer is added,
// so nothing is blocked, and the Workstation is removed once the grace period has passed.
func (r *WorkstationReconciler) waitForConfig(ctx context.Context, ws *wsv1alpha1.Workstation, remaining time.Duration) (ctrl.Result, error) {
	name := ws.Spec.ConfigRef.Name
	log.FromContext(ctx).Info("parent WorkstationConfig not found, waiting for it", "config", name, "remaining", remaining.Round(time.Second))
	desired := *ws.Status.DeepCopy()
	desired.Phase = "Pending"
	markWaiting(ws.Generation, &desired.Conditions, ws.Status.Conditions, "WaitingForConfig",
		fmt.Sprintf("WorkstationConfig %q does not exist yet", name))
	if err := updateWorkstationStatus(ctx, r, ws, desired); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: min(kccWaitInterval, remaining)}, nil
}

// removeUnprovisioned deletes a Workstation that has no KCC Workstation yet, so it cannot block its
// config's delete. With our finalizer it then goes through reconcileDelete.
func (r *WorkstationReconciler) removeUnprovisioned(ctx context.Context, ws *wsv1alpha1.Workstation, reason string) error {
	state := "is being deleted"
	if reason == reasonConfigNotFound {
		state = "does not exist"
	}
	log.FromContext(ctx).Info("removing Workstation before provisioning: parent WorkstationConfig "+state, "config", ws.Spec.ConfigRef.Name)
	if r.Recorder != nil {
		r.Recorder.Eventf(ws, nil, corev1.EventTypeWarning, reason, "Delete",
			"WorkstationConfig %q %s; Workstation removed before provisioning", ws.Spec.ConfigRef.Name, state)
	}
	// The UID precondition keeps a Workstation recreated under the same name since this read.
	uid := ws.UID
	return client.IgnoreNotFound(r.Delete(ctx, ws, client.Preconditions{UID: &uid}))
}

func (r *WorkstationReconciler) reconcileCreateOrUpdate(ctx context.Context, ws *wsv1alpha1.Workstation, parentConfig *wsv1alpha1.WorkstationConfig) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Only reached for a provisioned workstation whose config is gone, which config delete ordering prevents.
	if parentConfig == nil {
		logger.Info("parent WorkstationConfig not found, waiting", "config", ws.Spec.ConfigRef.Name)
		ws.Status.Phase = "Pending"
		_ = r.Status().Update(ctx, ws)
		return kccWait(), nil
	}

	// parentRef.name must be the parent KCC object's own metadata.name (KCC resolves
	// it as a same-namespace object reference); the parent KCC object is always
	// created with SetName(parentConfig.Name), never Spec.ResourceID.
	kccParentName := parentConfig.Name

	kccObj := &unstructured.Unstructured{}
	kccObj.SetGroupVersionKind(kccWorkstationGVK)
	kccObj.SetName(ws.Name)
	kccObj.SetNamespace(ws.Namespace)

	op, err := controllerutil.CreateOrUpdate(ctx, r.Client, kccObj, func() error {
		if err := controllerutil.SetControllerReference(ws, kccObj, r.Scheme); err != nil {
			return fmt.Errorf("set controller reference: %w", err)
		}

		specMap := map[string]interface{}{
			"parentRef": map[string]interface{}{
				"name": kccParentName,
			},
		}

		if ws.Spec.DisplayName != "" {
			specMap["displayName"] = ws.Spec.DisplayName
		}
		if ws.Spec.ResourceID != "" {
			specMap["resourceID"] = ws.Spec.ResourceID
		}

		setKeyValueList(specMap, "labels", ws.Spec.Labels)
		setKeyValueList(specMap, "annotations", ws.Spec.Annotations)

		// NOTE: API-owned fields (users) are explicitly EXCLUDED from specMap.
		// They are never written to KCC. Power is not a spec field.

		kccObj.Object["spec"] = specMap
		return nil
	})

	if err != nil {
		logger.Error(err, "failed to create or update KCC Workstation")
		ws.Status.Phase = "Failed"
		_ = r.Status().Update(ctx, ws)
		return ctrl.Result{}, err
	}

	logger.V(1).Info("KCC Workstation reconciled", "operation", op)

	desired := r.projectWorkstationStatus(ws, kccObj)
	kccReady := metaConditionTrue(desired.Conditions, ConditionKCCReady)
	if !kccReady || desired.ExternalRef == "" {
		markWaiting(ws.Generation, &desired.Conditions, ws.Status.Conditions, "WaitingForKCC", "waiting for the Config Connector workstation to become Ready", ConditionIAMPolicySynced)
		desired.Phase = "Pending"
		if err := updateWorkstationStatus(ctx, r, ws, desired); err != nil {
			return ctrl.Result{}, err
		}
		return kccWait(), nil
	}
	result := r.reconcileWorkstationAPI(ctx, ws, parentConfig, &desired)
	if err := updateWorkstationStatus(ctx, r, ws, desired); err != nil {
		return ctrl.Result{}, err
	}
	return result, nil
}

func (r *WorkstationReconciler) projectWorkstationStatus(ws *wsv1alpha1.Workstation, kccObj *unstructured.Unstructured) wsv1alpha1.WorkstationStatus {
	mirrored := mirrorKCCStatus(kccObj, ws.Generation, ws.Status.Conditions, ConditionKCCReady)
	desired := wsv1alpha1.WorkstationStatus{
		Phase:              "Pending",
		ObservedGeneration: mirrored.ObservedGeneration,
		Conditions:         mirrored.Conditions,
		Host:               mirrored.Host,
		State:              mirrored.State,
		ExternalRef:        mirrored.ExternalRef,
	}
	if len(mirrored.Raw) > 0 {
		desired.KCC = &runtime.RawExtension{Raw: mirrored.Raw}
	}
	return desired
}

func updateWorkstationStatus(ctx context.Context, r *WorkstationReconciler, ws *wsv1alpha1.Workstation, desired wsv1alpha1.WorkstationStatus) error {
	if equality.Semantic.DeepEqual(ws.Status, desired) {
		return nil
	}
	ws.Status = desired
	return r.Status().Update(ctx, ws)
}

// reconcileDelete removes the KCC Workstation and waits for it to be gone before removing finalizer.
func (r *WorkstationReconciler) reconcileDelete(ctx context.Context, ws *wsv1alpha1.Workstation) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(ws, WorkstationFinalizer) {
		return ctrl.Result{}, nil
	}
	return finishKCCDelete(ctx, r.Client, ws, kccWorkstationGVK, WorkstationFinalizer)
}

// SetupWithManager sets up the controller with the Manager.
func (r *WorkstationReconciler) SetupWithManager(mgr ctrl.Manager) error {
	kccObj := &unstructured.Unstructured{}
	kccObj.SetGroupVersionKind(kccWorkstationGVK)

	return ctrl.NewControllerManagedBy(mgr).
		For(&wsv1alpha1.Workstation{}).
		Owns(kccObj).
		Complete(r)
}
