// Copyright (c) 2026 Accenture, All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package controller

import (
	"context"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	wsv1alpha1 "cloud-workstations-module/api/v1alpha1"
)

func TestWorkstationConfigReconcile_EncryptionKeyObjectRefs(t *testing.T) {
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
			EncryptionKey: &wsv1alpha1.EncryptionKeyConfig{
				KmsCryptoKeyRef:   &wsv1alpha1.CryptoKeyRef{Name: "my-key", Namespace: "cloud-workstations"},
				ServiceAccountRef: &wsv1alpha1.GceServiceAccountRef{Name: "my-sa", Namespace: "cloud-workstations"},
			},
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
		t.Fatalf("failed to get KCC config: %v", err)
	}
	keyName, _, _ := unstructured.NestedString(kccObj.Object, "spec", "encryptionKey", "kmsCryptoKeyRef", "name")
	keyNS, _, _ := unstructured.NestedString(kccObj.Object, "spec", "encryptionKey", "kmsCryptoKeyRef", "namespace")
	saName, _, _ := unstructured.NestedString(kccObj.Object, "spec", "encryptionKey", "serviceAccountRef", "name")
	saNS, _, _ := unstructured.NestedString(kccObj.Object, "spec", "encryptionKey", "serviceAccountRef", "namespace")
	if keyName != "my-key" || keyNS != "cloud-workstations" {
		t.Errorf("kmsCryptoKeyRef = %s/%s, want cloud-workstations/my-key", keyNS, keyName)
	}
	if saName != "my-sa" || saNS != "cloud-workstations" {
		t.Errorf("serviceAccountRef = %s/%s, want cloud-workstations/my-sa", saNS, saName)
	}
}

func TestWorkstationClusterReconcile_LabelOrderIsStable(t *testing.T) {
	scheme := setupClusterTestScheme()
	cluster := &wsv1alpha1.WorkstationCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-cluster",
			Namespace:  "cloud-workstations",
			Finalizers: []string{ClusterFinalizer},
		},
		Spec: wsv1alpha1.WorkstationClusterSpec{
			Project:    "test-project",
			Location:   "us-central1",
			Network:    "sdv-network",
			Subnetwork: "sdv-subnet",
			Labels: map[string]string{
				"k0": "v", "k1": "v", "k2": "v", "k3": "v", "k4": "v", "k5": "v",
			},
		},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster).WithStatusSubresource(cluster).Build()
	r := &WorkstationClusterReconciler{Client: c, Scheme: scheme}
	req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "cloud-workstations", Name: "test-cluster"}}
	if _, err := r.Reconcile(context.Background(), req); err != nil {
		t.Fatalf("first reconcile: %v", err)
	}
	kccObj := &unstructured.Unstructured{}
	kccObj.SetGroupVersionKind(kccClusterGVK)
	if err := c.Get(context.Background(), req.NamespacedName, kccObj); err != nil {
		t.Fatalf("failed to get KCC cluster: %v", err)
	}
	before := kccObj.GetResourceVersion()
	if _, err := r.Reconcile(context.Background(), req); err != nil {
		t.Fatalf("second reconcile: %v", err)
	}
	if err := c.Get(context.Background(), req.NamespacedName, kccObj); err != nil {
		t.Fatalf("failed to get KCC cluster: %v", err)
	}
	if kccObj.GetResourceVersion() != before {
		t.Fatalf("KCC resourceVersion changed from %s to %s with an unchanged spec", before, kccObj.GetResourceVersion())
	}
}

func TestWorkstationClusterReconcile_StatusIsStable(t *testing.T) {
	scheme := setupClusterTestScheme()
	cluster := &wsv1alpha1.WorkstationCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-cluster",
			Namespace:  "cloud-workstations",
			Finalizers: []string{ClusterFinalizer},
		},
		Spec: wsv1alpha1.WorkstationClusterSpec{
			Project: "test-project", Location: "us-central1", Network: "sdv-network", Subnetwork: "sdv-subnet",
		},
	}
	kccObj := &unstructured.Unstructured{}
	kccObj.SetGroupVersionKind(kccClusterGVK)
	kccObj.SetName("test-cluster")
	kccObj.SetNamespace("cloud-workstations")
	kccObj.Object["status"] = map[string]interface{}{
		"conditions": []interface{}{
			map[string]interface{}{
				"type":               "Ready",
				"status":             "True",
				"reason":             "UpToDate",
				"message":            "Cluster is ready",
				"lastTransitionTime": "2026-01-01T00:00:00Z",
			},
		},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster, kccObj).WithStatusSubresource(cluster).Build()
	r := &WorkstationClusterReconciler{Client: c, Scheme: scheme}
	req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "cloud-workstations", Name: "test-cluster"}}
	if _, err := r.Reconcile(context.Background(), req); err != nil {
		t.Fatalf("first reconcile: %v", err)
	}
	updated := &wsv1alpha1.WorkstationCluster{}
	if err := c.Get(context.Background(), req.NamespacedName, updated); err != nil {
		t.Fatal(err)
	}
	before := updated.ResourceVersion
	if _, err := r.Reconcile(context.Background(), req); err != nil {
		t.Fatalf("second reconcile: %v", err)
	}
	if err := c.Get(context.Background(), req.NamespacedName, updated); err != nil {
		t.Fatal(err)
	}
	if updated.ResourceVersion != before {
		t.Fatalf("Horizon resourceVersion changed from %s to %s with unchanged KCC status", before, updated.ResourceVersion)
	}
	if len(updated.Status.Conditions) != 1 || updated.Status.Conditions[0].LastTransitionTime.UTC().Format("2006-01-02T15:04:05Z") != "2026-01-01T00:00:00Z" {
		t.Fatalf("expected KCC lastTransitionTime to be copied, got %+v", updated.Status.Conditions)
	}
}

func TestWorkstationReconcile_DeleteGetErrorKeepsFinalizer(t *testing.T) {
	scheme := setupWorkstationTestScheme()
	now := metav1.Now()
	ws := &wsv1alpha1.Workstation{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "test-ws",
			Namespace:         "cloud-workstations",
			Finalizers:        []string{WorkstationFinalizer},
			DeletionTimestamp: &now,
		},
		Spec: wsv1alpha1.WorkstationSpec{ConfigRef: wsv1alpha1.ConfigRef{Name: "cfg"}},
	}
	kccObj := &unstructured.Unstructured{}
	kccObj.SetGroupVersionKind(kccWorkstationGVK)
	kccObj.SetName("test-ws")
	kccObj.SetNamespace("cloud-workstations")
	kccObj.SetFinalizers([]string{"cnrm.cloud.google.com/finalizer"})

	failGet := true
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(ws, kccObj).
		WithInterceptorFuncs(interceptor.Funcs{
			Get: func(ctx context.Context, cl client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
				if failGet {
					if u, ok := obj.(*unstructured.Unstructured); ok && u.GroupVersionKind().Group == kccWorkstationGVK.Group {
						return apierrors.NewServiceUnavailable("simulated API timeout")
					}
				}
				return cl.Get(ctx, key, obj, opts...)
			},
		}).Build()
	r := &WorkstationReconciler{Client: c, Scheme: scheme}
	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: "cloud-workstations", Name: "test-ws"},
	})
	if !apierrors.IsServiceUnavailable(err) {
		t.Fatalf("expected ServiceUnavailable, got %v", err)
	}
	failGet = false

	updated := &wsv1alpha1.Workstation{}
	if err := c.Get(context.Background(), types.NamespacedName{Namespace: "cloud-workstations", Name: "test-ws"}, updated); err != nil {
		t.Fatalf("Horizon Workstation was removed: %v", err)
	}
	if len(updated.Finalizers) != 1 || updated.Finalizers[0] != WorkstationFinalizer {
		t.Fatalf("finalizer = %v, want %s", updated.Finalizers, WorkstationFinalizer)
	}
	got := &unstructured.Unstructured{}
	got.SetGroupVersionKind(kccWorkstationGVK)
	if err := c.Get(context.Background(), types.NamespacedName{Namespace: "cloud-workstations", Name: "test-ws"}, got); err != nil {
		t.Fatalf("KCC Workstation was removed: %v", err)
	}
}

func TestWorkstationClusterReconcile_KCCListErrorDoesNotDeleteParent(t *testing.T) {
	scheme := setupClusterTestScheme()
	now := metav1.Now()
	cluster := &wsv1alpha1.WorkstationCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "test-cluster",
			Namespace:         "cloud-workstations",
			Finalizers:        []string{ClusterFinalizer},
			DeletionTimestamp: &now,
		},
	}
	kccCluster := &unstructured.Unstructured{}
	kccCluster.SetGroupVersionKind(kccClusterGVK)
	kccCluster.SetName("test-cluster")
	kccCluster.SetNamespace("cloud-workstations")
	kccCluster.SetFinalizers([]string{"cnrm.cloud.google.com/finalizer"})
	kccConfig := &unstructured.Unstructured{}
	kccConfig.SetGroupVersionKind(kccConfigGVK)
	kccConfig.SetName("child")
	kccConfig.SetNamespace("cloud-workstations")
	_ = unstructured.SetNestedField(kccConfig.Object, "test-cluster", "spec", "parentRef", "name")

	failList := true
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster, kccCluster, kccConfig).
		WithInterceptorFuncs(interceptor.Funcs{
			List: func(ctx context.Context, cl client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
				if failList {
					if u, ok := list.(*unstructured.UnstructuredList); ok && u.GroupVersionKind().Group == kccClusterGVK.Group {
						return apierrors.NewServiceUnavailable("simulated API timeout")
					}
				}
				return cl.List(ctx, list, opts...)
			},
		}).Build()
	r := &WorkstationClusterReconciler{Client: c, Scheme: scheme}
	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: "cloud-workstations", Name: "test-cluster"},
	})
	if !apierrors.IsServiceUnavailable(err) {
		t.Fatalf("expected ServiceUnavailable, got %v", err)
	}
	failList = false

	got := &unstructured.Unstructured{}
	got.SetGroupVersionKind(kccClusterGVK)
	if err := c.Get(context.Background(), types.NamespacedName{Namespace: "cloud-workstations", Name: "test-cluster"}, got); err != nil {
		t.Fatalf("KCC cluster was removed: %v", err)
	}
	if got.GetDeletionTimestamp() != nil {
		t.Fatal("KCC cluster delete was issued while listing child configs failed")
	}
}

func TestWorkstationReconcile_ObservedGenerationWaitsForKCC(t *testing.T) {
	scheme := setupWorkstationTestScheme()
	cfg := &wsv1alpha1.WorkstationConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "parent-config", Namespace: "cloud-workstations"},
	}
	ws := &wsv1alpha1.Workstation{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-ws", Namespace: "cloud-workstations", Finalizers: []string{WorkstationFinalizer},
			Generation: 2,
		},
		Spec: wsv1alpha1.WorkstationSpec{ConfigRef: wsv1alpha1.ConfigRef{Name: "parent-config"}},
	}
	kccObj := &unstructured.Unstructured{}
	kccObj.SetGroupVersionKind(kccWorkstationGVK)
	kccObj.SetName("test-ws")
	kccObj.SetNamespace("cloud-workstations")
	kccObj.SetGeneration(2)
	kccObj.Object["status"] = map[string]interface{}{
		"observedGeneration": int64(1),
		"conditions": []interface{}{map[string]interface{}{
			"type": "Ready", "status": "True", "reason": "UpToDate", "message": "ready",
			"lastTransitionTime": "2026-01-01T00:00:00Z",
		}},
		"observedState": map[string]interface{}{"host": "old.example"},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cfg, ws, kccObj).WithStatusSubresource(ws).Build()
	r := &WorkstationReconciler{Client: c, Scheme: scheme}
	req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "cloud-workstations", Name: "test-ws"}}
	if _, err := r.Reconcile(context.Background(), req); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	got := &wsv1alpha1.Workstation{}
	if err := c.Get(context.Background(), req.NamespacedName, got); err != nil {
		t.Fatal(err)
	}
	if got.Status.ObservedGeneration == got.Generation {
		t.Fatalf("observedGeneration %d reported the new generation before Config Connector caught up", got.Status.ObservedGeneration)
	}
	if got.Status.Phase == "Ready" {
		t.Fatal("phase must stay pending while Config Connector status is stale")
	}

	stored := &unstructured.Unstructured{}
	stored.SetGroupVersionKind(kccWorkstationGVK)
	if err := c.Get(context.Background(), req.NamespacedName, stored); err != nil {
		t.Fatal(err)
	}
	_ = unstructured.SetNestedField(stored.Object, int64(stored.GetGeneration()), "status", "observedGeneration")
	if err := c.Update(context.Background(), stored); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Reconcile(context.Background(), req); err != nil {
		t.Fatalf("caught-up reconcile: %v", err)
	}
	if err := c.Get(context.Background(), req.NamespacedName, got); err != nil {
		t.Fatal(err)
	}
	before := got.ResourceVersion
	if got.Status.ObservedGeneration != got.Generation {
		t.Fatalf("observedGeneration = %d, generation = %d", got.Status.ObservedGeneration, got.Generation)
	}
	if _, err := r.Reconcile(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if err := c.Get(context.Background(), req.NamespacedName, got); err != nil {
		t.Fatal(err)
	}
	if got.ResourceVersion != before {
		t.Fatalf("status resourceVersion changed from %s to %s after Config Connector caught up", before, got.ResourceVersion)
	}
}

func TestWorkstationReconcile_ClearsHostWhenKCCOmitsIt(t *testing.T) {
	scheme := setupWorkstationTestScheme()
	cfg := &wsv1alpha1.WorkstationConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "parent-config", Namespace: "cloud-workstations"},
	}
	ws := &wsv1alpha1.Workstation{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-ws", Namespace: "cloud-workstations", Finalizers: []string{WorkstationFinalizer},
		},
		Spec:   wsv1alpha1.WorkstationSpec{ConfigRef: wsv1alpha1.ConfigRef{Name: "parent-config"}},
		Status: wsv1alpha1.WorkstationStatus{Host: "old.example", State: "STATE_RUNNING"},
	}
	kccObj := &unstructured.Unstructured{}
	kccObj.SetGroupVersionKind(kccWorkstationGVK)
	kccObj.SetName("test-ws")
	kccObj.SetNamespace("cloud-workstations")
	kccObj.SetGeneration(1)
	kccObj.Object["status"] = map[string]interface{}{
		"observedGeneration": int64(1),
		"observedState":      map[string]interface{}{"uid": "abc"},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cfg, ws, kccObj).WithStatusSubresource(ws).Build()
	r := &WorkstationReconciler{Client: c, Scheme: scheme}
	req := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "cloud-workstations", Name: "test-ws"}}
	if _, err := r.Reconcile(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	got := &wsv1alpha1.Workstation{}
	if err := c.Get(context.Background(), req.NamespacedName, got); err != nil {
		t.Fatal(err)
	}
	if got.Status.Host != "" || got.Status.State != "" {
		t.Fatalf("stale host/state kept: host=%q state=%q", got.Status.Host, got.Status.State)
	}
}

func TestWorkstationReconcile_DeletingParentDoesNotCreateKCC(t *testing.T) {
	scheme := setupWorkstationTestScheme()
	now := metav1.Now()
	cfg := &wsv1alpha1.WorkstationConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: "parent-config", Namespace: "cloud-workstations",
			Finalizers: []string{ConfigFinalizer}, DeletionTimestamp: &now,
		},
	}
	ws := &wsv1alpha1.Workstation{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-ws", Namespace: "cloud-workstations", Finalizers: []string{WorkstationFinalizer},
		},
		Spec: wsv1alpha1.WorkstationSpec{ConfigRef: wsv1alpha1.ConfigRef{Name: "parent-config"}},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cfg, ws).WithStatusSubresource(ws).Build()
	r := &WorkstationReconciler{Client: c, Scheme: scheme}
	if _, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: "cloud-workstations", Name: "test-ws"},
	}); err != nil {
		t.Fatal(err)
	}
	kccObj := &unstructured.Unstructured{}
	kccObj.SetGroupVersionKind(kccWorkstationGVK)
	err := c.Get(context.Background(), types.NamespacedName{Namespace: "cloud-workstations", Name: "test-ws"}, kccObj)
	if !apierrors.IsNotFound(err) {
		t.Fatalf("Config Connector workstation must not be created while the parent is deleting, err=%v", err)
	}
}

func TestWorkstationConfigReconcile_DeletingParentDoesNotCreateKCC(t *testing.T) {
	scheme := setupConfigTestScheme()
	now := metav1.Now()
	cluster := &wsv1alpha1.WorkstationCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "parent-cluster", Namespace: "cloud-workstations",
			Finalizers: []string{ClusterFinalizer}, DeletionTimestamp: &now,
		},
	}
	cfg := &wsv1alpha1.WorkstationConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-config", Namespace: "cloud-workstations", Finalizers: []string{ConfigFinalizer},
		},
		Spec: wsv1alpha1.WorkstationConfigSpec{ClusterRef: wsv1alpha1.ClusterRef{Name: "parent-cluster"}},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster, cfg).WithStatusSubresource(cfg).Build()
	r := &WorkstationConfigReconciler{Client: c, Scheme: scheme, HostServiceAccount: testHostServiceAccount}
	if _, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: "cloud-workstations", Name: "test-config"},
	}); err != nil {
		t.Fatal(err)
	}
	kccObj := &unstructured.Unstructured{}
	kccObj.SetGroupVersionKind(kccConfigGVK)
	err := c.Get(context.Background(), types.NamespacedName{Namespace: "cloud-workstations", Name: "test-config"}, kccObj)
	if !apierrors.IsNotFound(err) {
		t.Fatalf("Config Connector config must not be created while the parent is deleting, err=%v", err)
	}
}
