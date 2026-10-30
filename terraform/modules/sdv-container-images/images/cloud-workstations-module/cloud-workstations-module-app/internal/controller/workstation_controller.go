// Copyright (c) 2026 Accenture, All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package controller

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	wsv1alpha1 "cloud-workstations-module/api/v1alpha1"
	"cloud-workstations-module/internal/wsapi"
)

const (
	WorkstationFinalizer = "workstations.horizon-sdv.io/workstation-finalizer"
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
	Scheme         *runtime.Scheme
	WS             wsapi.API
	ResyncInterval time.Duration
}

// +kubebuilder:rbac:groups=workstations.horizon-sdv.io,resources=workstations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=workstations.horizon-sdv.io,resources=workstations/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=workstations.horizon-sdv.io,resources=workstations/finalizers,verbs=update
// +kubebuilder:rbac:groups=workstations.cnrm.cloud.google.com,resources=workstations,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=workstations.cnrm.cloud.google.com,resources=workstations/status,verbs=get

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

	// Ensure finalizer
	if !controllerutil.ContainsFinalizer(&ws, WorkstationFinalizer) {
		controllerutil.AddFinalizer(&ws, WorkstationFinalizer)
		if err := r.Update(ctx, &ws); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Reconcile KCC object
	return r.reconcileCreateOrUpdate(ctx, &ws)
}

func (r *WorkstationReconciler) reconcileCreateOrUpdate(ctx context.Context, ws *wsv1alpha1.Workstation) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Resolve parent WorkstationConfig
	parentConfig := &wsv1alpha1.WorkstationConfig{}
	configKey := client.ObjectKey{Namespace: ws.Namespace, Name: ws.Spec.ConfigRef.Name}
	if err := r.Get(ctx, configKey, parentConfig); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("parent WorkstationConfig not found, waiting", "config", ws.Spec.ConfigRef.Name)
			ws.Status.Phase = "Pending"
			_ = r.Status().Update(ctx, ws)
			return kccWait(), nil
		}
		return ctrl.Result{}, err
	}
	if !parentConfig.DeletionTimestamp.IsZero() {
		logger.Info("parent WorkstationConfig is deleting, waiting", "config", ws.Spec.ConfigRef.Name)
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
