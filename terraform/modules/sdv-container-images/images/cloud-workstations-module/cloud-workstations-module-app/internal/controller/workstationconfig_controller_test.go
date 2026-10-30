// Copyright (c) 2026 Accenture, All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package controller

import (
	"context"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	wsv1alpha1 "cloud-workstations-module/api/v1alpha1"
)

const testHostServiceAccount = "gke-cws-host-vm-sa@test-project.iam.gserviceaccount.com"

func setupConfigTestScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(s)
	_ = wsv1alpha1.AddToScheme(s)
	return s
}

func TestWorkstationConfigReconcile_AddFinalizer(t *testing.T) {
	scheme := setupConfigTestScheme()
	cfg := &wsv1alpha1.WorkstationConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-config",
			Namespace: "cloud-workstations",
		},
		Spec: wsv1alpha1.WorkstationConfigSpec{
			ClusterRef: wsv1alpha1.ClusterRef{Name: "test-cluster"},
		},
	}

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(cfg).
		WithStatusSubresource(cfg).
		Build()

	r := &WorkstationConfigReconciler{Client: c, Scheme: scheme, HostServiceAccount: testHostServiceAccount}
	res, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: "cloud-workstations", Name: "test-config"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Requeue {
		t.Fatalf("expected requeue after adding finalizer")
	}

	updated := &wsv1alpha1.WorkstationConfig{}
	if err := c.Get(context.Background(), types.NamespacedName{Namespace: "cloud-workstations", Name: "test-config"}, updated); err != nil {
		t.Fatalf("failed to get config: %v", err)
	}
	found := false
	for _, f := range updated.Finalizers {
		if f == ConfigFinalizer {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected finalizer %q on config", ConfigFinalizer)
	}
}

func TestWorkstationConfigReconcile_CreateKCC_MappingAndParentRef(t *testing.T) {
	scheme := setupConfigTestScheme()
	cluster := &wsv1alpha1.WorkstationCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "parent-cluster",
			Namespace: "cloud-workstations",
		},
		Spec: wsv1alpha1.WorkstationClusterSpec{
			Project:    "test-project",
			Location:   "us-central1",
			ResourceID: "cws-cluster-resource-id",
		},
	}

	runAsUser := int64(1000)
	bootDisk := int64(100)
	disablePub := true
	nestedVirt := true
	poolSize := int64(2)
	sizeGB := int64(50)

	cfg := &wsv1alpha1.WorkstationConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-config",
			Namespace:  "cloud-workstations",
			Finalizers: []string{ConfigFinalizer},
		},
		Spec: wsv1alpha1.WorkstationConfigSpec{
			ClusterRef:     wsv1alpha1.ClusterRef{Name: "parent-cluster"},
			DisplayName:    "Test Config",
			IdleTimeout:    "7200s",
			RunningTimeout: "43200s",
			ReplicaZones:   []string{"us-central1-a", "us-central1-b"},
			Container: &wsv1alpha1.ContainerConfig{
				Image:      "us-central1-docker.pkg.dev/proj/repo/img:1.0",
				Command:    []string{"/entrypoint.sh"},
				Args:       []string{"--verbose"},
				RunAsUser:  &runAsUser,
				WorkingDir: "/home/user",
			},
			Host: &wsv1alpha1.HostConfig{
				GceInstance: &wsv1alpha1.GceInstanceConfig{
					MachineType:                "e2-standard-4",
					BootDiskSizeGB:             &bootDisk,
					DisablePublicIPAddresses:   &disablePub,
					EnableNestedVirtualization: &nestedVirt,
					PoolSize:                   &poolSize,
					Tags:                       []string{"workstation-node"},
				},
			},
			PersistentDirectories: []wsv1alpha1.PersistentDirectory{
				{
					MountPath: "/home",
					GcePD: &wsv1alpha1.GcePersistentDisk{
						DiskType:      "pd-ssd",
						FsType:        "ext4",
						ReclaimPolicy: "DELETE",
						SizeGB:        &sizeGB,
					},
				},
			},
			ReadinessChecks: []wsv1alpha1.ReadinessCheck{
				{Path: "/healthz", Port: 80},
			},
			EncryptionKey: &wsv1alpha1.EncryptionKeyConfig{
				KmsCryptoKeyRef: &wsv1alpha1.CryptoKeyRef{External: "projects/p/locations/l/keyRings/r/cryptoKeys/k"},
			},
			ResourceID: "custom-config-id",
		},
	}

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(cluster, cfg).
		WithStatusSubresource(cfg).
		Build()

	r := &WorkstationConfigReconciler{Client: c, Scheme: scheme, HostServiceAccount: testHostServiceAccount}
	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: "cloud-workstations", Name: "test-config"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify KCC object was created
	kccObj := &unstructured.Unstructured{}
	kccObj.SetGroupVersionKind(kccConfigGVK)
	if err := c.Get(context.Background(), types.NamespacedName{Namespace: "cloud-workstations", Name: "test-config"}, kccObj); err != nil {
		t.Fatalf("failed to get KCC config: %v", err)
	}

	parentRefName, _, _ := unstructured.NestedString(kccObj.Object, "spec", "parentRef", "name")
	if parentRefName != "parent-cluster" {
		t.Errorf("expected parentRef.name = parent-cluster (parent's own object name, not Spec.ResourceID), got %q", parentRefName)
	}

	machType, _, _ := unstructured.NestedString(kccObj.Object, "spec", "host", "gceInstance", "machineType")
	if machType != "e2-standard-4" {
		t.Errorf("expected machineType = e2-standard-4, got %q", machType)
	}

	saEmail, _, _ := unstructured.NestedString(kccObj.Object, "spec", "host", "gceInstance", "serviceAccountRef", "external")
	if saEmail != testHostServiceAccount {
		t.Errorf("expected host VM SA = %s, got %q", testHostServiceAccount, saEmail)
	}

	img, _, _ := unstructured.NestedString(kccObj.Object, "spec", "container", "image")
	if img != "us-central1-docker.pkg.dev/proj/repo/img:1.0" {
		t.Errorf("expected container image, got %q", img)
	}
}

func TestWorkstationConfigReconcile_InjectsHostServiceAccountWithoutHostSpec(t *testing.T) {
	scheme := setupConfigTestScheme()
	cluster := &wsv1alpha1.WorkstationCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "parent-cluster", Namespace: "cloud-workstations"},
	}
	cfg := &wsv1alpha1.WorkstationConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-config",
			Namespace:  "cloud-workstations",
			Finalizers: []string{ConfigFinalizer},
		},
		Spec: wsv1alpha1.WorkstationConfigSpec{
			ClusterRef: wsv1alpha1.ClusterRef{Name: "parent-cluster"},
		},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster, cfg).WithStatusSubresource(cfg).Build()
	r := &WorkstationConfigReconciler{Client: c, Scheme: scheme, HostServiceAccount: testHostServiceAccount}

	if _, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: "cloud-workstations", Name: "test-config"},
	}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	kccObj := &unstructured.Unstructured{}
	kccObj.SetGroupVersionKind(kccConfigGVK)
	if err := c.Get(context.Background(), types.NamespacedName{Namespace: "cloud-workstations", Name: "test-config"}, kccObj); err != nil {
		t.Fatal(err)
	}
	saEmail, _, _ := unstructured.NestedString(kccObj.Object, "spec", "host", "gceInstance", "serviceAccountRef", "external")
	poolSize, _, _ := unstructured.NestedInt64(kccObj.Object, "spec", "host", "gceInstance", "poolSize")
	if saEmail != testHostServiceAccount || poolSize != 0 {
		t.Fatalf("host defaults: serviceAccount=%q poolSize=%d", saEmail, poolSize)
	}
}

func TestWorkstationConfigReconcile_RejectsMissingHostServiceAccount(t *testing.T) {
	scheme := setupConfigTestScheme()
	cluster := &wsv1alpha1.WorkstationCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "parent-cluster", Namespace: "cloud-workstations"},
	}
	cfg := &wsv1alpha1.WorkstationConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-config",
			Namespace:  "cloud-workstations",
			Finalizers: []string{ConfigFinalizer},
		},
		Spec: wsv1alpha1.WorkstationConfigSpec{
			ClusterRef: wsv1alpha1.ClusterRef{Name: "parent-cluster"},
		},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster, cfg).WithStatusSubresource(cfg).Build()
	r := &WorkstationConfigReconciler{Client: c, Scheme: scheme}

	if _, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: "cloud-workstations", Name: "test-config"},
	}); err == nil || err.Error() != "host service account is not configured" {
		t.Fatalf("expected host service account error, got %v", err)
	}
	kccObj := &unstructured.Unstructured{}
	kccObj.SetGroupVersionKind(kccConfigGVK)
	err := c.Get(context.Background(), types.NamespacedName{Namespace: "cloud-workstations", Name: "test-config"}, kccObj)
	if !apierrors.IsNotFound(err) {
		t.Fatalf("KCC config was written without a host service account: %v", err)
	}
}

// TestWorkstationConfigReconcile_NeverWritesAPIOwnedFields verifies that API-owned fields
// (disableSsh, allowedPorts, ephemeralDirectories, admins) are NEVER written to the KCC object.
func TestWorkstationConfigReconcile_NeverWritesAPIOwnedFields(t *testing.T) {
	scheme := setupConfigTestScheme()
	cluster := &wsv1alpha1.WorkstationCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "parent-cluster",
			Namespace: "cloud-workstations",
		},
	}

	disableSsh := true
	cfg := &wsv1alpha1.WorkstationConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-config",
			Namespace:  "cloud-workstations",
			Finalizers: []string{ConfigFinalizer},
		},
		Spec: wsv1alpha1.WorkstationConfigSpec{
			ClusterRef: wsv1alpha1.ClusterRef{Name: "parent-cluster"},
			DisableSsh: &disableSsh,
			AllowedPorts: []wsv1alpha1.PortRange{
				{First: 80, Last: 80},
				{First: 1024, Last: 65535},
			},
			EphemeralDirectories: []wsv1alpha1.EphemeralDirectory{
				{MountPath: "/tmp"},
			},
			Admins: []string{"user:admin@example.com"},
		},
	}

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(cluster, cfg).
		WithStatusSubresource(cfg).
		Build()

	r := &WorkstationConfigReconciler{Client: c, Scheme: scheme, HostServiceAccount: testHostServiceAccount}
	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: "cloud-workstations", Name: "test-config"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	kccObj := &unstructured.Unstructured{}
	kccObj.SetGroupVersionKind(kccConfigGVK)
	if err := c.Get(context.Background(), types.NamespacedName{Namespace: "cloud-workstations", Name: "test-config"}, kccObj); err != nil {
		t.Fatalf("failed to get KCC config: %v", err)
	}

	specMap, _, _ := unstructured.NestedMap(kccObj.Object, "spec")

	// Assertions: None of the API-owned fields should exist in KCC spec!
	apiOwnedFields := []string{"disableSsh", "allowedPorts", "ephemeralDirectories", "admins"}
	for _, f := range apiOwnedFields {
		if _, exists := specMap[f]; exists {
			t.Errorf("VIOLATION: API-owned field %q was written to KCC WorkstationConfig spec!", f)
		}
	}
}

func TestWorkstationConfigReconcile_DeleteBlockedByChildWorkstation(t *testing.T) {
	scheme := setupConfigTestScheme()
	now := metav1.NewTime(time.Now())
	cfg := &wsv1alpha1.WorkstationConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "test-config",
			Namespace:         "cloud-workstations",
			Finalizers:        []string{ConfigFinalizer},
			DeletionTimestamp: &now,
		},
		Spec: wsv1alpha1.WorkstationConfigSpec{
			ClusterRef: wsv1alpha1.ClusterRef{Name: "parent-cluster"},
		},
	}

	childWs := &wsv1alpha1.Workstation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "child-ws",
			Namespace: "cloud-workstations",
		},
		Spec: wsv1alpha1.WorkstationSpec{
			ConfigRef: wsv1alpha1.ConfigRef{Name: "test-config"},
		},
	}

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(cfg, childWs).
		WithStatusSubresource(cfg).
		Build()

	r := &WorkstationConfigReconciler{Client: c, Scheme: scheme, HostServiceAccount: testHostServiceAccount}
	res, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: "cloud-workstations", Name: "test-config"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.RequeueAfter <= 0 {
		t.Fatalf("expected delayed requeue when deletion is blocked by child workstation, got %+v", res)
	}

	updated := &wsv1alpha1.WorkstationConfig{}
	if err := c.Get(context.Background(), types.NamespacedName{Namespace: "cloud-workstations", Name: "test-config"}, updated); err != nil {
		t.Fatalf("failed to get config: %v", err)
	}
	if len(updated.Finalizers) == 0 {
		t.Fatalf("expected finalizer to remain while child workstation exists")
	}
	assertConfigDeleteStatus(t, updated, "DeletionBlocked", `Workstation "child-ws" still uses this config`)
}

func TestWorkstationConfigReconcile_DeleteBlockedByKCCWorkstation(t *testing.T) {
	scheme := setupConfigTestScheme()
	now := metav1.NewTime(time.Now())
	cfg := &wsv1alpha1.WorkstationConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "test-config",
			Namespace:         "cloud-workstations",
			Finalizers:        []string{ConfigFinalizer},
			DeletionTimestamp: &now,
		},
		Spec: wsv1alpha1.WorkstationConfigSpec{
			ClusterRef: wsv1alpha1.ClusterRef{Name: "parent-cluster"},
		},
	}
	kccWs := &unstructured.Unstructured{}
	kccWs.SetGroupVersionKind(kccWorkstationGVK)
	kccWs.SetName("manual-ws")
	kccWs.SetNamespace("cloud-workstations")
	_ = unstructured.SetNestedField(kccWs.Object, "test-config", "spec", "parentRef", "name")

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(cfg, kccWs).
		WithStatusSubresource(cfg).
		Build()

	r := &WorkstationConfigReconciler{Client: c, Scheme: scheme, HostServiceAccount: testHostServiceAccount}
	req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "cloud-workstations", Name: "test-config"}}
	res, err := r.Reconcile(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.RequeueAfter <= 0 {
		t.Fatalf("expected delayed requeue when deletion is blocked by KCC workstation, got %+v", res)
	}

	updated := &wsv1alpha1.WorkstationConfig{}
	if err := c.Get(context.Background(), req.NamespacedName, updated); err != nil {
		t.Fatalf("failed to get config: %v", err)
	}
	if len(updated.Finalizers) == 0 {
		t.Fatalf("expected finalizer to remain while KCC workstation exists")
	}
	assertConfigDeleteStatus(t, updated, "DeletionBlocked", `Config Connector Workstation "manual-ws" still references this config`)

	// An unchanged blocker must not rewrite status on every requeue.
	before := updated.ResourceVersion
	if _, err := r.Reconcile(context.Background(), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := c.Get(context.Background(), req.NamespacedName, updated); err != nil {
		t.Fatalf("failed to get config: %v", err)
	}
	if updated.ResourceVersion != before {
		t.Fatalf("resourceVersion changed from %s to %s with an unchanged blocker", before, updated.ResourceVersion)
	}
}

func TestWorkstationConfigReconcile_DeleteClearsDeletionBlocked(t *testing.T) {
	scheme := setupConfigTestScheme()
	now := metav1.NewTime(time.Now())
	cfg := &wsv1alpha1.WorkstationConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "test-config",
			Namespace:         "cloud-workstations",
			Finalizers:        []string{ConfigFinalizer},
			DeletionTimestamp: &now,
		},
		Spec: wsv1alpha1.WorkstationConfigSpec{
			ClusterRef: wsv1alpha1.ClusterRef{Name: "parent-cluster"},
		},
		Status: wsv1alpha1.WorkstationConfigStatus{
			Phase: "Deleting",
			Conditions: []metav1.Condition{{
				Type:               ConditionReady,
				Status:             metav1.ConditionFalse,
				Reason:             "DeletionBlocked",
				Message:            `Workstation "child-ws" still uses this config`,
				LastTransitionTime: now,
			}},
		},
	}
	// The KCC config keeps its own finalizer, so the Horizon config stays while KCC deletes it.
	kccCfg := &unstructured.Unstructured{}
	kccCfg.SetGroupVersionKind(kccConfigGVK)
	kccCfg.SetName("test-config")
	kccCfg.SetNamespace("cloud-workstations")
	kccCfg.SetFinalizers([]string{"cnrm.cloud.google.com/finalizer"})

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(cfg, kccCfg).
		WithStatusSubresource(cfg).
		Build()

	r := &WorkstationConfigReconciler{Client: c, Scheme: scheme, HostServiceAccount: testHostServiceAccount}
	req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "cloud-workstations", Name: "test-config"}}
	if _, err := r.Reconcile(context.Background(), req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	updated := &wsv1alpha1.WorkstationConfig{}
	if err := c.Get(context.Background(), req.NamespacedName, updated); err != nil {
		t.Fatalf("failed to get config: %v", err)
	}
	if len(updated.Finalizers) == 0 {
		t.Fatalf("expected finalizer to remain while the KCC config is being deleted")
	}
	assertConfigDeleteStatus(t, updated, "Deleting", "deleting the Config Connector config")
}

func assertConfigDeleteStatus(t *testing.T, cfg *wsv1alpha1.WorkstationConfig, reason, message string) {
	t.Helper()
	if cfg.Status.Phase != "Deleting" {
		t.Fatalf("phase = %q, want Deleting", cfg.Status.Phase)
	}
	ready := meta.FindStatusCondition(cfg.Status.Conditions, ConditionReady)
	if ready == nil {
		t.Fatalf("expected a Ready condition, got %+v", cfg.Status.Conditions)
	}
	if ready.Status != metav1.ConditionFalse || ready.Reason != reason || ready.Message != message {
		t.Fatalf("Ready = %s/%s/%q, want False/%s/%q", ready.Status, ready.Reason, ready.Message, reason, message)
	}
}

func TestWorkstationConfigReconcile_DeleteSucceedsWhenNoChildren(t *testing.T) {
	scheme := setupConfigTestScheme()
	now := metav1.NewTime(time.Now())
	cfg := &wsv1alpha1.WorkstationConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "test-config",
			Namespace:         "cloud-workstations",
			Finalizers:        []string{ConfigFinalizer},
			DeletionTimestamp: &now,
		},
		Spec: wsv1alpha1.WorkstationConfigSpec{
			ClusterRef: wsv1alpha1.ClusterRef{Name: "parent-cluster"},
		},
	}

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(cfg).
		WithStatusSubresource(cfg).
		Build()

	r := &WorkstationConfigReconciler{Client: c, Scheme: scheme, HostServiceAccount: testHostServiceAccount}
	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: "cloud-workstations", Name: "test-config"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	updated := &wsv1alpha1.WorkstationConfig{}
	if err := c.Get(context.Background(), types.NamespacedName{Namespace: "cloud-workstations", Name: "test-config"}, updated); err != nil {
		return // Deleted completely
	}
	for _, f := range updated.Finalizers {
		if f == ConfigFinalizer {
			t.Fatalf("expected finalizer to be removed when no children exist")
		}
	}
}

func TestWorkstationConfigReconcile_StatusProjection(t *testing.T) {
	scheme := setupConfigTestScheme()
	cluster := &wsv1alpha1.WorkstationCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "parent-cluster",
			Namespace: "cloud-workstations",
		},
	}
	cfg := &wsv1alpha1.WorkstationConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-config",
			Namespace:  "cloud-workstations",
			Finalizers: []string{ConfigFinalizer},
		},
		Spec: wsv1alpha1.WorkstationConfigSpec{
			ClusterRef: wsv1alpha1.ClusterRef{Name: "parent-cluster"},
		},
	}

	kccObj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "workstations.cnrm.cloud.google.com/v1beta1",
			"kind":       "WorkstationConfig",
			"metadata": map[string]interface{}{
				"name":      "test-config",
				"namespace": "cloud-workstations",
			},
			"status": map[string]interface{}{
				"conditions": []interface{}{
					map[string]interface{}{
						"type":    "Ready",
						"status":  "True",
						"reason":  "UpToDate",
						"message": "Config is ready",
					},
				},
				"observedState": map[string]interface{}{
					"createTime": "2026-09-21T00:00:00Z",
				},
			},
		},
	}

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(cluster, cfg, kccObj).
		WithStatusSubresource(cfg, kccObj).
		Build()

	r := &WorkstationConfigReconciler{Client: c, Scheme: scheme, HostServiceAccount: testHostServiceAccount}
	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: "cloud-workstations", Name: "test-config"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	updated := &wsv1alpha1.WorkstationConfig{}
	if err := c.Get(context.Background(), types.NamespacedName{Namespace: "cloud-workstations", Name: "test-config"}, updated); err != nil {
		t.Fatalf("failed to get updated config: %v", err)
	}

	if updated.Status.Phase == "Ready" {
		t.Errorf("phase must stay pending until posture and IAM are applied, got Ready")
	}
	if !metaConditionTrue(updated.Status.Conditions, ConditionKCCReady) {
		t.Errorf("expected KCCReady condition, got %+v", updated.Status.Conditions)
	}
	if metaConditionTrue(updated.Status.Conditions, ConditionReady) {
		t.Errorf("aggregate Ready was set before the API path, conditions %+v", updated.Status.Conditions)
	}
}
