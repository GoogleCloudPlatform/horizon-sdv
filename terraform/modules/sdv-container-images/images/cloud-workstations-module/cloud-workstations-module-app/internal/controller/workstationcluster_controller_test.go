// Copyright (c) 2026 Accenture, All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package controller

import (
	"context"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	wsv1alpha1 "cloud-workstations-module/api/v1alpha1"
)

func setupClusterTestScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(s)
	_ = wsv1alpha1.AddToScheme(s)
	return s
}

func TestWorkstationClusterReconcile_AddFinalizer(t *testing.T) {
	scheme := setupClusterTestScheme()
	cluster := &wsv1alpha1.WorkstationCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-cluster",
			Namespace: "cloud-workstations",
		},
		Spec: wsv1alpha1.WorkstationClusterSpec{
			Project:    "test-project",
			Location:   "us-central1",
			Network:    "projects/test-project/global/networks/sdv-network",
			Subnetwork: "projects/test-project/regions/us-central1/subnetworks/sdv-subnet",
		},
	}

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(cluster).
		WithStatusSubresource(cluster).
		Build()

	r := &WorkstationClusterReconciler{Client: c, Scheme: scheme}
	res, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: "cloud-workstations", Name: "test-cluster"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Requeue {
		t.Fatalf("expected requeue after adding finalizer")
	}

	updated := &wsv1alpha1.WorkstationCluster{}
	if err := c.Get(context.Background(), types.NamespacedName{Namespace: "cloud-workstations", Name: "test-cluster"}, updated); err != nil {
		t.Fatalf("failed to get cluster: %v", err)
	}
	found := false
	for _, f := range updated.Finalizers {
		if f == ClusterFinalizer {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected finalizer %q on cluster", ClusterFinalizer)
	}
}

func TestWorkstationClusterReconcile_CreateKCC(t *testing.T) {
	scheme := setupClusterTestScheme()
	enablePrivate := true
	cluster := &wsv1alpha1.WorkstationCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-cluster",
			Namespace:  "cloud-workstations",
			Finalizers: []string{ClusterFinalizer},
		},
		Spec: wsv1alpha1.WorkstationClusterSpec{
			Project:     "test-project",
			Location:    "us-central1",
			Network:     "projects/test-project/global/networks/sdv-network",
			Subnetwork:  "projects/test-project/regions/us-central1/subnetworks/sdv-subnet",
			DisplayName: "Test Cluster",
			ResourceID:  "custom-cluster-id",
			PrivateClusterConfig: &wsv1alpha1.PrivateClusterConfig{
				EnablePrivateEndpoint: &enablePrivate,
				AllowedProjects:       []string{"project-b"},
			},
			Labels:      map[string]string{"env": "test"},
			Annotations: map[string]string{"note": "automation"},
		},
	}

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(cluster).
		WithStatusSubresource(cluster).
		Build()

	r := &WorkstationClusterReconciler{Client: c, Scheme: scheme}
	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: "cloud-workstations", Name: "test-cluster"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify KCC object was created
	kccObj := &unstructured.Unstructured{}
	kccObj.SetGroupVersionKind(kccClusterGVK)
	if err := c.Get(context.Background(), types.NamespacedName{Namespace: "cloud-workstations", Name: "test-cluster"}, kccObj); err != nil {
		t.Fatalf("failed to get KCC cluster: %v", err)
	}

	proj, _, _ := unstructured.NestedString(kccObj.Object, "spec", "projectRef", "external")
	if proj != "test-project" {
		t.Errorf("expected projectRef.external = test-project, got %q", proj)
	}
	loc, _, _ := unstructured.NestedString(kccObj.Object, "spec", "location")
	if loc != "us-central1" {
		t.Errorf("expected location = us-central1, got %q", loc)
	}
	net, _, _ := unstructured.NestedString(kccObj.Object, "spec", "networkRef", "external")
	if net != "projects/test-project/global/networks/sdv-network" {
		t.Errorf("expected networkRef = projects/test-project/global/networks/sdv-network, got %q", net)
	}
	disp, _, _ := unstructured.NestedString(kccObj.Object, "spec", "displayName")
	if disp != "Test Cluster" {
		t.Errorf("expected displayName = Test Cluster, got %q", disp)
	}
	resID, _, _ := unstructured.NestedString(kccObj.Object, "spec", "resourceID")
	if resID != "custom-cluster-id" {
		t.Errorf("expected resourceID = custom-cluster-id, got %q", resID)
	}
	priv, _, _ := unstructured.NestedBool(kccObj.Object, "spec", "privateClusterConfig", "enablePrivateEndpoint")
	if !priv {
		t.Errorf("expected enablePrivateEndpoint = true, got false")
	}
}

func TestWorkstationClusterReconcile_StatusProjection(t *testing.T) {
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
		},
	}

	kccObj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "workstations.cnrm.cloud.google.com/v1beta1",
			"kind":       "WorkstationCluster",
			"metadata": map[string]interface{}{
				"name":      "test-cluster",
				"namespace": "cloud-workstations",
			},
			"spec": map[string]interface{}{
				"location": "us-central1",
			},
			"status": map[string]interface{}{
				"conditions": []interface{}{
					map[string]interface{}{
						"type":    "Ready",
						"status":  "True",
						"reason":  "UpToDate",
						"message": "Cluster is ready",
					},
				},
				"observedState": map[string]interface{}{
					"clusterHostname": "cluster-123.cloudworkstations.dev",
					"controlPlaneIP":  "10.0.0.1",
				},
			},
		},
	}

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(cluster, kccObj).
		WithStatusSubresource(cluster, kccObj).
		Build()

	r := &WorkstationClusterReconciler{Client: c, Scheme: scheme}
	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: "cloud-workstations", Name: "test-cluster"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	updated := &wsv1alpha1.WorkstationCluster{}
	if err := c.Get(context.Background(), types.NamespacedName{Namespace: "cloud-workstations", Name: "test-cluster"}, updated); err != nil {
		t.Fatalf("failed to get updated cluster: %v", err)
	}

	if updated.Status.Phase != "Ready" {
		t.Errorf("expected phase Ready, got %q", updated.Status.Phase)
	}
	if len(updated.Status.Conditions) == 0 || updated.Status.Conditions[0].Type != "Ready" {
		t.Errorf("expected Ready condition mirrored, got %+v", updated.Status.Conditions)
	}
	if updated.Status.KCC == nil {
		t.Errorf("expected KCC observedState captured in status.KCC")
	}
}

func TestWorkstationClusterReconcile_DeleteBlockedByChildConfig(t *testing.T) {
	scheme := setupClusterTestScheme()
	now := metav1.NewTime(time.Now())
	cluster := &wsv1alpha1.WorkstationCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "test-cluster",
			Namespace:         "cloud-workstations",
			Finalizers:        []string{ClusterFinalizer},
			DeletionTimestamp: &now,
		},
		Spec: wsv1alpha1.WorkstationClusterSpec{
			Project:    "test-project",
			Location:   "us-central1",
			Network:    "sdv-network",
			Subnetwork: "sdv-subnet",
		},
	}

	childConfig := &wsv1alpha1.WorkstationConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "child-config",
			Namespace: "cloud-workstations",
		},
		Spec: wsv1alpha1.WorkstationConfigSpec{
			ClusterRef: wsv1alpha1.ClusterRef{Name: "test-cluster"},
		},
	}

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(cluster, childConfig).
		WithStatusSubresource(cluster).
		Build()

	r := &WorkstationClusterReconciler{Client: c, Scheme: scheme}
	res, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: "cloud-workstations", Name: "test-cluster"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.RequeueAfter <= 0 {
		t.Fatalf("expected delayed requeue when deletion is blocked by child config, got %+v", res)
	}

	// Verify finalizer still present
	updated := &wsv1alpha1.WorkstationCluster{}
	if err := c.Get(context.Background(), types.NamespacedName{Namespace: "cloud-workstations", Name: "test-cluster"}, updated); err != nil {
		t.Fatalf("failed to get cluster: %v", err)
	}
	if len(updated.Finalizers) == 0 {
		t.Fatalf("expected finalizer to remain while child config exists")
	}
}

func TestWorkstationClusterReconcile_DeleteSucceedsWhenNoChildren(t *testing.T) {
	scheme := setupClusterTestScheme()
	now := metav1.NewTime(time.Now())
	cluster := &wsv1alpha1.WorkstationCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "test-cluster",
			Namespace:         "cloud-workstations",
			Finalizers:        []string{ClusterFinalizer},
			DeletionTimestamp: &now,
		},
		Spec: wsv1alpha1.WorkstationClusterSpec{
			Project:    "test-project",
			Location:   "us-central1",
			Network:    "sdv-network",
			Subnetwork: "sdv-subnet",
		},
	}

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(cluster).
		WithStatusSubresource(cluster).
		Build()

	r := &WorkstationClusterReconciler{Client: c, Scheme: scheme}
	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: "cloud-workstations", Name: "test-cluster"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Finalizer should now be removed
	updated := &wsv1alpha1.WorkstationCluster{}
	if err := c.Get(context.Background(), types.NamespacedName{Namespace: "cloud-workstations", Name: "test-cluster"}, updated); err != nil {
		// If object is completely deleted, that's fine too
		return
	}
	for _, f := range updated.Finalizers {
		if f == ClusterFinalizer {
			t.Fatalf("expected finalizer to be removed after successful delete")
		}
	}
}
