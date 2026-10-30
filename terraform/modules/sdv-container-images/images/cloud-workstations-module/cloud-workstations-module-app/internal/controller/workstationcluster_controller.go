// Copyright (c) 2026 Accenture, All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package controller

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	wsv1alpha1 "cloud-workstations-module/api/v1alpha1"
)

const (
	ClusterFinalizer = "workstations.horizon-sdv.io/cluster-finalizer"
)

var (
	kccClusterGVK = schema.GroupVersionKind{
		Group:   "workstations.cnrm.cloud.google.com",
		Version: "v1beta1",
		Kind:    "WorkstationCluster",
	}
)

// WorkstationClusterReconciler reconciles a WorkstationCluster object.
type WorkstationClusterReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=workstations.horizon-sdv.io,resources=workstationclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=workstations.horizon-sdv.io,resources=workstationclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=workstations.horizon-sdv.io,resources=workstationclusters/finalizers,verbs=update
// +kubebuilder:rbac:groups=workstations.cnrm.cloud.google.com,resources=workstationclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=workstations.cnrm.cloud.google.com,resources=workstationclusters/status,verbs=get

func (r *WorkstationClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("workstationcluster", req.NamespacedName)
	logger.V(1).Info("reconciling WorkstationCluster")

	var cluster wsv1alpha1.WorkstationCluster
	if err := r.Get(ctx, req.NamespacedName, &cluster); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Handle deletion
	if !cluster.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, &cluster)
	}

	// Ensure finalizer
	if !controllerutil.ContainsFinalizer(&cluster, ClusterFinalizer) {
		controllerutil.AddFinalizer(&cluster, ClusterFinalizer)
		if err := r.Update(ctx, &cluster); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Reconcile KCC object
	return r.reconcileCreateOrUpdate(ctx, &cluster)
}

func (r *WorkstationClusterReconciler) reconcileCreateOrUpdate(ctx context.Context, cluster *wsv1alpha1.WorkstationCluster) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	kccObj := &unstructured.Unstructured{}
	kccObj.SetGroupVersionKind(kccClusterGVK)
	kccObj.SetName(cluster.Name)
	kccObj.SetNamespace(cluster.Namespace)

	op, err := controllerutil.CreateOrUpdate(ctx, r.Client, kccObj, func() error {
		// Set owner reference so KCC object is cleaned up if needed
		if err := controllerutil.SetControllerReference(cluster, kccObj, r.Scheme); err != nil {
			return fmt.Errorf("set controller reference: %w", err)
		}

		// Map KCC-owned fields
		specMap := map[string]interface{}{
			"projectRef": map[string]interface{}{
				"external": cluster.Spec.Project,
			},
			"location": cluster.Spec.Location,
			"networkRef": map[string]interface{}{
				"external": cluster.Spec.Network,
			},
			"subnetworkRef": map[string]interface{}{
				"external": cluster.Spec.Subnetwork,
			},
		}

		if cluster.Spec.DisplayName != "" {
			specMap["displayName"] = cluster.Spec.DisplayName
		}
		if cluster.Spec.ResourceID != "" {
			specMap["resourceID"] = cluster.Spec.ResourceID
		}
		if cluster.Spec.PrivateClusterConfig != nil {
			privMap := map[string]interface{}{}
			if cluster.Spec.PrivateClusterConfig.EnablePrivateEndpoint != nil {
				privMap["enablePrivateEndpoint"] = *cluster.Spec.PrivateClusterConfig.EnablePrivateEndpoint
			}
			if len(cluster.Spec.PrivateClusterConfig.AllowedProjects) > 0 {
				var allowed []interface{}
				for _, p := range cluster.Spec.PrivateClusterConfig.AllowedProjects {
					allowed = append(allowed, map[string]interface{}{"external": p})
				}
				privMap["allowedProjects"] = allowed
			}
			specMap["privateClusterConfig"] = privMap
		}

		setKeyValueList(specMap, "labels", cluster.Spec.Labels)
		setKeyValueList(specMap, "annotations", cluster.Spec.Annotations)
		kccObj.Object["spec"] = specMap

		return nil
	})

	if err != nil {
		logger.Error(err, "failed to create or update KCC WorkstationCluster")
		cluster.Status.Phase = "Failed"
		_ = r.Status().Update(ctx, cluster)
		return ctrl.Result{}, err
	}

	logger.V(1).Info("KCC WorkstationCluster reconciled", "operation", op)

	// Mirror KCC status onto Horizon object
	if err := r.mirrorStatus(ctx, cluster, kccObj); err != nil {
		logger.Error(err, "failed to mirror KCC status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *WorkstationClusterReconciler) mirrorStatus(ctx context.Context, cluster *wsv1alpha1.WorkstationCluster, kccObj *unstructured.Unstructured) error {
	mirrored := mirrorKCCStatus(kccObj, cluster.Generation, cluster.Status.Conditions, "")
	desired := wsv1alpha1.WorkstationClusterStatus{
		Phase:              mirrored.Phase,
		ObservedGeneration: mirrored.ObservedGeneration,
		Conditions:         mirrored.Conditions,
	}
	if len(mirrored.Raw) > 0 {
		desired.KCC = &runtime.RawExtension{Raw: mirrored.Raw}
	}
	return updateClusterStatus(ctx, r, cluster, desired)
}

func updateClusterStatus(ctx context.Context, r *WorkstationClusterReconciler, cluster *wsv1alpha1.WorkstationCluster, desired wsv1alpha1.WorkstationClusterStatus) error {
	if equality.Semantic.DeepEqual(cluster.Status, desired) {
		return nil
	}
	cluster.Status = desired
	return r.Status().Update(ctx, cluster)
}

// reconcileDelete enforces delete ordering: WorkstationCluster cannot be deleted while
// child WorkstationConfigs exist.
func (r *WorkstationClusterReconciler) reconcileDelete(ctx context.Context, cluster *wsv1alpha1.WorkstationCluster) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("workstationcluster", cluster.Name)

	if !controllerutil.ContainsFinalizer(cluster, ClusterFinalizer) {
		return ctrl.Result{}, nil
	}

	// Check if any Horizon WorkstationConfigs reference this cluster
	var childConfigs wsv1alpha1.WorkstationConfigList
	if err := r.List(ctx, &childConfigs, client.InNamespace(cluster.Namespace)); err != nil {
		return ctrl.Result{}, err
	}

	for _, cfg := range childConfigs.Items {
		if cfg.Spec.ClusterRef.Name == cluster.Name {
			logger.Info("deletion blocked: child WorkstationConfig exists", "config", cfg.Name)
			return kccWait(), nil
		}
	}

	blocked, err := kccChildBlocksDelete(ctx, r.Client, cluster.Namespace, cluster.Name, schema.GroupVersionKind{
		Group: kccClusterGVK.Group, Version: kccClusterGVK.Version, Kind: "WorkstationConfigList",
	})
	if err != nil {
		return ctrl.Result{}, err
	}
	if blocked != "" {
		logger.Info("deletion blocked: KCC WorkstationConfig exists", "kccConfig", blocked)
		return kccWait(), nil
	}
	return finishKCCDelete(ctx, r.Client, cluster, kccClusterGVK, ClusterFinalizer)
}

// SetupWithManager sets up the controller with the Manager.
func (r *WorkstationClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	kccObj := &unstructured.Unstructured{}
	kccObj.SetGroupVersionKind(kccClusterGVK)

	return ctrl.NewControllerManagedBy(mgr).
		For(&wsv1alpha1.WorkstationCluster{}).
		Owns(kccObj).
		Complete(r)
}
