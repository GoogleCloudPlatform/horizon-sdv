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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	ConfigFinalizer = "workstations.horizon-sdv.io/config-finalizer"
)

var (
	kccConfigGVK = schema.GroupVersionKind{
		Group:   "workstations.cnrm.cloud.google.com",
		Version: "v1beta1",
		Kind:    "WorkstationConfig",
	}
)

// WorkstationConfigReconciler reconciles a WorkstationConfig object.
type WorkstationConfigReconciler struct {
	client.Client
	Scheme             *runtime.Scheme
	WS                 wsapi.API
	ResyncInterval     time.Duration
	HostServiceAccount string
}

// +kubebuilder:rbac:groups=workstations.horizon-sdv.io,resources=workstationconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=workstations.horizon-sdv.io,resources=workstationconfigs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=workstations.horizon-sdv.io,resources=workstationconfigs/finalizers,verbs=update
// +kubebuilder:rbac:groups=workstations.cnrm.cloud.google.com,resources=workstationconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=workstations.cnrm.cloud.google.com,resources=workstationconfigs/status,verbs=get

func (r *WorkstationConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("workstationconfig", req.NamespacedName)
	logger.V(1).Info("reconciling WorkstationConfig")

	var config wsv1alpha1.WorkstationConfig
	if err := r.Get(ctx, req.NamespacedName, &config); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Handle deletion
	if !config.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, &config)
	}

	// Ensure finalizer
	if !controllerutil.ContainsFinalizer(&config, ConfigFinalizer) {
		controllerutil.AddFinalizer(&config, ConfigFinalizer)
		if err := r.Update(ctx, &config); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// Reconcile KCC object
	return r.reconcileCreateOrUpdate(ctx, &config)
}

func (r *WorkstationConfigReconciler) reconcileCreateOrUpdate(ctx context.Context, cfg *wsv1alpha1.WorkstationConfig) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	if r.HostServiceAccount == "" {
		return ctrl.Result{}, fmt.Errorf("host service account is not configured")
	}

	// Resolve parent WorkstationCluster
	parentCluster := &wsv1alpha1.WorkstationCluster{}
	clusterKey := client.ObjectKey{Namespace: cfg.Namespace, Name: cfg.Spec.ClusterRef.Name}
	if err := r.Get(ctx, clusterKey, parentCluster); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("parent WorkstationCluster not found, waiting", "cluster", cfg.Spec.ClusterRef.Name)
			cfg.Status.Phase = "Pending"
			_ = r.Status().Update(ctx, cfg)
			return kccWait(), nil
		}
		return ctrl.Result{}, err
	}
	if !parentCluster.DeletionTimestamp.IsZero() {
		logger.Info("parent WorkstationCluster is deleting, waiting", "cluster", cfg.Spec.ClusterRef.Name)
		cfg.Status.Phase = "Pending"
		_ = r.Status().Update(ctx, cfg)
		return kccWait(), nil
	}

	// parentRef.name must be the parent KCC object's own metadata.name (KCC resolves
	// it as a same-namespace object reference); the parent KCC object is always
	// created with SetName(parentCluster.Name), never Spec.ResourceID.
	kccParentName := parentCluster.Name

	kccObj := &unstructured.Unstructured{}
	kccObj.SetGroupVersionKind(kccConfigGVK)
	kccObj.SetName(cfg.Name)
	kccObj.SetNamespace(cfg.Namespace)

	op, err := controllerutil.CreateOrUpdate(ctx, r.Client, kccObj, func() error {
		if err := controllerutil.SetControllerReference(cfg, kccObj, r.Scheme); err != nil {
			return fmt.Errorf("set controller reference: %w", err)
		}

		specMap := map[string]interface{}{
			"parentRef": map[string]interface{}{
				"name": kccParentName,
			},
		}

		if cfg.Spec.DisplayName != "" {
			specMap["displayName"] = cfg.Spec.DisplayName
		}
		if cfg.Spec.IdleTimeout != "" {
			specMap["idleTimeout"] = cfg.Spec.IdleTimeout
		}
		if cfg.Spec.RunningTimeout != "" {
			specMap["runningTimeout"] = cfg.Spec.RunningTimeout
		}
		if len(cfg.Spec.ReplicaZones) > 0 {
			specMap["replicaZones"] = toInterfaceSlice(cfg.Spec.ReplicaZones)
		}
		if cfg.Spec.ResourceID != "" {
			specMap["resourceID"] = cfg.Spec.ResourceID
		}

		// Container settings
		if cfg.Spec.Container != nil {
			cMap := map[string]interface{}{}
			if cfg.Spec.Container.Image != "" {
				cMap["image"] = cfg.Spec.Container.Image
			}
			if len(cfg.Spec.Container.Command) > 0 {
				cMap["command"] = toInterfaceSlice(cfg.Spec.Container.Command)
			}
			if len(cfg.Spec.Container.Args) > 0 {
				cMap["args"] = toInterfaceSlice(cfg.Spec.Container.Args)
			}
			if len(cfg.Spec.Container.Env) > 0 {
				var envs []interface{}
				for _, e := range cfg.Spec.Container.Env {
					envs = append(envs, map[string]interface{}{"name": e.Name, "value": e.Value})
				}
				cMap["env"] = envs
			}
			if cfg.Spec.Container.RunAsUser != nil {
				cMap["runAsUser"] = *cfg.Spec.Container.RunAsUser
			}
			if cfg.Spec.Container.WorkingDir != "" {
				cMap["workingDir"] = cfg.Spec.Container.WorkingDir
			}
			specMap["container"] = cMap
		}

		// Every module-managed workstation VM runs as the dedicated host identity.
		gceMap := map[string]interface{}{
			"serviceAccountRef": map[string]interface{}{"external": r.HostServiceAccount},
			"poolSize":          int64(0),
		}

		// User-configurable host VM settings
		if cfg.Spec.Host != nil && cfg.Spec.Host.GceInstance != nil {
			gce := cfg.Spec.Host.GceInstance

			if gce.MachineType != "" {
				gceMap["machineType"] = gce.MachineType
			}
			if gce.BootDiskSizeGB != nil {
				gceMap["bootDiskSizeGB"] = *gce.BootDiskSizeGB
			}
			if len(gce.ServiceAccountScopes) > 0 {
				gceMap["serviceAccountScopes"] = toInterfaceSlice(gce.ServiceAccountScopes)
			}
			if gce.DisablePublicIPAddresses != nil {
				gceMap["disablePublicIPAddresses"] = *gce.DisablePublicIPAddresses
			}
			if gce.EnableNestedVirtualization != nil {
				gceMap["enableNestedVirtualization"] = *gce.EnableNestedVirtualization
			}
			if gce.PoolSize != nil {
				gceMap["poolSize"] = *gce.PoolSize
			} else {
				gceMap["poolSize"] = int64(0)
			}
			if len(gce.Tags) > 0 {
				gceMap["tags"] = toInterfaceSlice(gce.Tags)
			}
			if gce.ShieldedInstanceConfig != nil {
				sc := map[string]interface{}{}
				if gce.ShieldedInstanceConfig.EnableIntegrityMonitoring != nil {
					sc["enableIntegrityMonitoring"] = *gce.ShieldedInstanceConfig.EnableIntegrityMonitoring
				}
				if gce.ShieldedInstanceConfig.EnableSecureBoot != nil {
					sc["enableSecureBoot"] = *gce.ShieldedInstanceConfig.EnableSecureBoot
				}
				if gce.ShieldedInstanceConfig.EnableVTPM != nil {
					sc["enableVTPM"] = *gce.ShieldedInstanceConfig.EnableVTPM
				}
				gceMap["shieldedInstanceConfig"] = sc
			}
			if gce.ConfidentialInstanceConfig != nil {
				cc := map[string]interface{}{}
				if gce.ConfidentialInstanceConfig.EnableConfidentialCompute != nil {
					cc["enableConfidentialCompute"] = *gce.ConfidentialInstanceConfig.EnableConfidentialCompute
				}
				gceMap["confidentialInstanceConfig"] = cc
			}
		}
		specMap["host"] = map[string]interface{}{"gceInstance": gceMap}

		// Persistent directories
		if len(cfg.Spec.PersistentDirectories) > 0 {
			var dirs []interface{}
			for _, pd := range cfg.Spec.PersistentDirectories {
				d := map[string]interface{}{"mountPath": pd.MountPath}
				if pd.GcePD != nil {
					gpd := map[string]interface{}{}
					if pd.GcePD.DiskType != "" {
						gpd["diskType"] = pd.GcePD.DiskType
					}
					if pd.GcePD.FsType != "" {
						gpd["fsType"] = pd.GcePD.FsType
					}
					if pd.GcePD.ReclaimPolicy != "" {
						gpd["reclaimPolicy"] = pd.GcePD.ReclaimPolicy
					}
					if pd.GcePD.SizeGB != nil {
						gpd["sizeGB"] = *pd.GcePD.SizeGB
					}
					if pd.GcePD.SourceSnapshot != "" {
						gpd["sourceSnapshot"] = pd.GcePD.SourceSnapshot
					}
					d["gcePD"] = gpd
				}
				dirs = append(dirs, d)
			}
			specMap["persistentDirectories"] = dirs
		}

		// Readiness checks
		if len(cfg.Spec.ReadinessChecks) > 0 {
			var rcList []interface{}
			for _, rc := range cfg.Spec.ReadinessChecks {
				rcList = append(rcList, map[string]interface{}{"path": rc.Path, "port": int64(rc.Port)})
			}
			specMap["readinessChecks"] = rcList
		}

		// Encryption key
		if cfg.Spec.EncryptionKey != nil {
			ek := map[string]interface{}{}
			if cfg.Spec.EncryptionKey.KmsCryptoKeyRef != nil {
				if kref := objectRef(cfg.Spec.EncryptionKey.KmsCryptoKeyRef.External, cfg.Spec.EncryptionKey.KmsCryptoKeyRef.Name, cfg.Spec.EncryptionKey.KmsCryptoKeyRef.Namespace); kref != nil {
					ek["kmsCryptoKeyRef"] = kref
				}
			}
			if cfg.Spec.EncryptionKey.ServiceAccountRef != nil {
				if sref := objectRef(cfg.Spec.EncryptionKey.ServiceAccountRef.External, cfg.Spec.EncryptionKey.ServiceAccountRef.Name, cfg.Spec.EncryptionKey.ServiceAccountRef.Namespace); sref != nil {
					ek["serviceAccountRef"] = sref
				}
			}
			if len(ek) > 0 {
				specMap["encryptionKey"] = ek
			}
		}

		setKeyValueList(specMap, "labels", cfg.Spec.Labels)
		setKeyValueList(specMap, "annotations", cfg.Spec.Annotations)

		// NOTE: API-owned fields (disableSsh, allowedPorts, ephemeralDirectories, admins)
		// are explicitly EXCLUDED from specMap. They are never written to KCC.

		kccObj.Object["spec"] = specMap
		return nil
	})

	if err != nil {
		logger.Error(err, "failed to create or update KCC WorkstationConfig")
		cfg.Status.Phase = "Failed"
		_ = r.Status().Update(ctx, cfg)
		return ctrl.Result{}, err
	}

	logger.V(1).Info("KCC WorkstationConfig reconciled", "operation", op)

	desired := r.projectConfigStatus(cfg, kccObj)
	kccReady := metaConditionTrue(desired.Conditions, ConditionKCCReady)
	if !kccReady || desired.ExternalRef == "" {
		markWaiting(cfg.Generation, &desired.Conditions, cfg.Status.Conditions, "WaitingForKCC", "waiting for the Config Connector config to become Ready", ConditionPostureApplied, ConditionIAMPolicySynced)
		desired.Phase = "Pending"
		if err := updateConfigStatus(ctx, r, cfg, desired); err != nil {
			return ctrl.Result{}, err
		}
		return kccWait(), nil
	}
	result := r.reconcileConfigAPI(ctx, cfg, &desired)
	if err := updateConfigStatus(ctx, r, cfg, desired); err != nil {
		return ctrl.Result{}, err
	}
	return result, nil
}

func (r *WorkstationConfigReconciler) projectConfigStatus(cfg *wsv1alpha1.WorkstationConfig, kccObj *unstructured.Unstructured) wsv1alpha1.WorkstationConfigStatus {
	mirrored := mirrorKCCStatus(kccObj, cfg.Generation, cfg.Status.Conditions, ConditionKCCReady)
	desired := wsv1alpha1.WorkstationConfigStatus{
		Phase:              "Pending",
		ObservedGeneration: mirrored.ObservedGeneration,
		Conditions:         mirrored.Conditions,
		ExternalRef:        mirrored.ExternalRef,
		Posture:            cfg.Status.Posture,
	}
	if len(mirrored.Raw) > 0 {
		desired.KCC = &runtime.RawExtension{Raw: mirrored.Raw}
	}
	return desired
}

func updateConfigStatus(ctx context.Context, r *WorkstationConfigReconciler, cfg *wsv1alpha1.WorkstationConfig, desired wsv1alpha1.WorkstationConfigStatus) error {
	if equality.Semantic.DeepEqual(cfg.Status, desired) {
		return nil
	}
	cfg.Status = desired
	return r.Status().Update(ctx, cfg)
}

// reconcileDelete enforces delete ordering: WorkstationConfig cannot be deleted while
// child Workstations exist.
func (r *WorkstationConfigReconciler) reconcileDelete(ctx context.Context, cfg *wsv1alpha1.WorkstationConfig) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithValues("workstationconfig", cfg.Name)

	if !controllerutil.ContainsFinalizer(cfg, ConfigFinalizer) {
		return ctrl.Result{}, nil
	}

	// Check if any Horizon Workstations reference this config
	var childWorkstations wsv1alpha1.WorkstationList
	if err := r.List(ctx, &childWorkstations, client.InNamespace(cfg.Namespace)); err != nil {
		return ctrl.Result{}, err
	}

	for _, ws := range childWorkstations.Items {
		if ws.Spec.ConfigRef.Name == cfg.Name {
			logger.Info("deletion blocked: child Workstation exists", "workstation", ws.Name)
			if err := setConfigDeleteStatus(ctx, r, cfg, "DeletionBlocked", fmt.Sprintf("Workstation %q still uses this config", ws.Name)); err != nil {
				return ctrl.Result{}, err
			}
			return kccWait(), nil
		}
	}

	blocked, err := kccChildBlocksDelete(ctx, r.Client, cfg.Namespace, cfg.Name, schema.GroupVersionKind{
		Group: kccConfigGVK.Group, Version: kccConfigGVK.Version, Kind: "WorkstationList",
	})
	if err != nil {
		return ctrl.Result{}, err
	}
	if blocked != "" {
		logger.Info("deletion blocked: KCC Workstation exists", "kccWorkstation", blocked)
		if err := setConfigDeleteStatus(ctx, r, cfg, "DeletionBlocked", fmt.Sprintf("Config Connector Workstation %q still references this config", blocked)); err != nil {
			return ctrl.Result{}, err
		}
		return kccWait(), nil
	}
	if err := setConfigDeleteStatus(ctx, r, cfg, "Deleting", "deleting the Config Connector config"); err != nil {
		return ctrl.Result{}, err
	}
	return finishKCCDelete(ctx, r.Client, cfg, kccConfigGVK, ConfigFinalizer)
}

// setConfigDeleteStatus reports why deletion is waiting on the CR, so workflows polling it can
// stop on DeletionBlocked instead of timing out. Deleting replaces a DeletionBlocked that has cleared.
func setConfigDeleteStatus(ctx context.Context, r *WorkstationConfigReconciler, cfg *wsv1alpha1.WorkstationConfig, reason, message string) error {
	desired := *cfg.Status.DeepCopy()
	desired.Phase = "Deleting"
	setCondition(&desired.Conditions, cfg.Status.Conditions, cfg.Generation, ConditionReady, metav1.ConditionFalse, reason, message)
	return updateConfigStatus(ctx, r, cfg, desired)
}

// SetupWithManager sets up the controller with the Manager.
func (r *WorkstationConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	kccObj := &unstructured.Unstructured{}
	kccObj.SetGroupVersionKind(kccConfigGVK)

	return ctrl.NewControllerManagedBy(mgr).
		For(&wsv1alpha1.WorkstationConfig{}).
		Owns(kccObj).
		Complete(r)
}
