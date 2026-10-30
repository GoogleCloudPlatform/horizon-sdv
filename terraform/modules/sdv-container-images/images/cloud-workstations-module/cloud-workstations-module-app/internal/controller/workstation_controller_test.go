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

func setupWorkstationTestScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(s)
	_ = wsv1alpha1.AddToScheme(s)
	return s
}

func TestWorkstationReconcile_AddFinalizer(t *testing.T) {
	scheme := setupWorkstationTestScheme()
	ws := &wsv1alpha1.Workstation{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-ws",
			Namespace: "cloud-workstations",
		},
		Spec: wsv1alpha1.WorkstationSpec{
			ConfigRef: wsv1alpha1.ConfigRef{Name: "parent-config"},
		},
	}

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(ws).
		WithStatusSubresource(ws).
		Build()

	r := &WorkstationReconciler{Client: c, Scheme: scheme}
	res, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: "cloud-workstations", Name: "test-ws"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.Requeue {
		t.Fatalf("expected requeue after adding finalizer")
	}

	updated := &wsv1alpha1.Workstation{}
	if err := c.Get(context.Background(), types.NamespacedName{Namespace: "cloud-workstations", Name: "test-ws"}, updated); err != nil {
		t.Fatalf("failed to get workstation: %v", err)
	}
	found := false
	for _, f := range updated.Finalizers {
		if f == WorkstationFinalizer {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected finalizer %q on workstation", WorkstationFinalizer)
	}
}

func TestWorkstationReconcile_CreateKCC_MappingAndParentRef(t *testing.T) {
	scheme := setupWorkstationTestScheme()
	parentConfig := &wsv1alpha1.WorkstationConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "parent-config",
			Namespace: "cloud-workstations",
		},
		Spec: wsv1alpha1.WorkstationConfigSpec{
			ClusterRef: wsv1alpha1.ClusterRef{Name: "parent-cluster"},
			ResourceID: "cws-cfg-resource-id",
		},
	}

	ws := &wsv1alpha1.Workstation{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-ws",
			Namespace:  "cloud-workstations",
			Finalizers: []string{WorkstationFinalizer},
		},
		Spec: wsv1alpha1.WorkstationSpec{
			ConfigRef:   wsv1alpha1.ConfigRef{Name: "parent-config"},
			DisplayName: "My Dev Workstation",
			ResourceID:  "custom-ws-id",
			Labels:      map[string]string{"owner": "alice"},
			Annotations: map[string]string{"env": "development"},
		},
	}

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(parentConfig, ws).
		WithStatusSubresource(ws).
		Build()

	r := &WorkstationReconciler{Client: c, Scheme: scheme}
	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: "cloud-workstations", Name: "test-ws"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify KCC object was created
	kccObj := &unstructured.Unstructured{}
	kccObj.SetGroupVersionKind(kccWorkstationGVK)
	if err := c.Get(context.Background(), types.NamespacedName{Namespace: "cloud-workstations", Name: "test-ws"}, kccObj); err != nil {
		t.Fatalf("failed to get KCC workstation: %v", err)
	}

	parentRefName, _, _ := unstructured.NestedString(kccObj.Object, "spec", "parentRef", "name")
	if parentRefName != "parent-config" {
		t.Errorf("expected parentRef.name = parent-config (parent's own object name, not Spec.ResourceID), got %q", parentRefName)
	}

	disp, _, _ := unstructured.NestedString(kccObj.Object, "spec", "displayName")
	if disp != "My Dev Workstation" {
		t.Errorf("expected displayName = My Dev Workstation, got %q", disp)
	}

	resID, _, _ := unstructured.NestedString(kccObj.Object, "spec", "resourceID")
	if resID != "custom-ws-id" {
		t.Errorf("expected resourceID = custom-ws-id, got %q", resID)
	}
}

// TestWorkstationReconcile_NeverWritesAPIOwnedFields verifies that API-owned fields
// (users) are NEVER written to the KCC object. powerState is not a spec field.
func TestWorkstationReconcile_NeverWritesAPIOwnedFields(t *testing.T) {
	scheme := setupWorkstationTestScheme()
	parentConfig := &wsv1alpha1.WorkstationConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "parent-config",
			Namespace: "cloud-workstations",
		},
	}

	ws := &wsv1alpha1.Workstation{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-ws",
			Namespace:  "cloud-workstations",
			Finalizers: []string{WorkstationFinalizer},
		},
		Spec: wsv1alpha1.WorkstationSpec{
			ConfigRef: wsv1alpha1.ConfigRef{Name: "parent-config"},
			Users:     []string{"user:alice@example.com"},
		},
	}

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(parentConfig, ws).
		WithStatusSubresource(ws).
		Build()

	r := &WorkstationReconciler{Client: c, Scheme: scheme}
	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: "cloud-workstations", Name: "test-ws"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	kccObj := &unstructured.Unstructured{}
	kccObj.SetGroupVersionKind(kccWorkstationGVK)
	if err := c.Get(context.Background(), types.NamespacedName{Namespace: "cloud-workstations", Name: "test-ws"}, kccObj); err != nil {
		t.Fatalf("failed to get KCC workstation: %v", err)
	}

	specMap, _, _ := unstructured.NestedMap(kccObj.Object, "spec")

	// Assertions: None of the API-owned fields should exist in KCC spec!
	apiOwnedFields := []string{"powerState", "users"}
	for _, f := range apiOwnedFields {
		if _, exists := specMap[f]; exists {
			t.Errorf("VIOLATION: API-owned field %q was written to KCC Workstation spec!", f)
		}
	}
}

func TestWorkstationReconcile_StatusProjection(t *testing.T) {
	scheme := setupWorkstationTestScheme()
	ws := &wsv1alpha1.Workstation{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "test-ws",
			Namespace:  "cloud-workstations",
			Finalizers: []string{WorkstationFinalizer},
		},
		Spec: wsv1alpha1.WorkstationSpec{
			ConfigRef: wsv1alpha1.ConfigRef{Name: "parent-config"},
		},
	}

	kccObj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "workstations.cnrm.cloud.google.com/v1beta1",
			"kind":       "Workstation",
			"metadata": map[string]interface{}{
				"name":      "test-ws",
				"namespace": "cloud-workstations",
			},
			"status": map[string]interface{}{
				"conditions": []interface{}{
					map[string]interface{}{
						"type":    "Ready",
						"status":  "True",
						"reason":  "UpToDate",
						"message": "Workstation is ready",
					},
				},
				"observedState": map[string]interface{}{
					"host":  "10.1.0.50",
					"state": "STATE_RUNNING",
				},
			},
		},
	}

	parentConfig := &wsv1alpha1.WorkstationConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "parent-config",
			Namespace: "cloud-workstations",
		},
	}

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(ws, kccObj, parentConfig).
		WithStatusSubresource(ws, kccObj).
		Build()

	r := &WorkstationReconciler{Client: c, Scheme: scheme}
	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: "cloud-workstations", Name: "test-ws"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	updated := &wsv1alpha1.Workstation{}
	if err := c.Get(context.Background(), types.NamespacedName{Namespace: "cloud-workstations", Name: "test-ws"}, updated); err != nil {
		t.Fatalf("failed to get updated workstation: %v", err)
	}

	if updated.Status.Phase == "Ready" {
		t.Errorf("phase must stay pending until workstation IAM is synced, got Ready")
	}
	if !metaConditionTrue(updated.Status.Conditions, ConditionKCCReady) {
		t.Errorf("expected KCCReady condition, got %+v", updated.Status.Conditions)
	}
	if updated.Status.Host != "10.1.0.50" {
		t.Errorf("expected host 10.1.0.50, got %q", updated.Status.Host)
	}
	if updated.Status.State != "STATE_RUNNING" {
		t.Errorf("expected state STATE_RUNNING, got %q", updated.Status.State)
	}
}

func TestWorkstationReconcile_DeleteRemovesFinalizer(t *testing.T) {
	scheme := setupWorkstationTestScheme()
	now := metav1.NewTime(time.Now())
	ws := &wsv1alpha1.Workstation{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "test-ws",
			Namespace:         "cloud-workstations",
			Finalizers:        []string{WorkstationFinalizer},
			DeletionTimestamp: &now,
		},
		Spec: wsv1alpha1.WorkstationSpec{
			ConfigRef: wsv1alpha1.ConfigRef{Name: "parent-config"},
		},
	}

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(ws).
		WithStatusSubresource(ws).
		Build()

	r := &WorkstationReconciler{Client: c, Scheme: scheme}
	_, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: "cloud-workstations", Name: "test-ws"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	updated := &wsv1alpha1.Workstation{}
	if err := c.Get(context.Background(), types.NamespacedName{Namespace: "cloud-workstations", Name: "test-ws"}, updated); err != nil {
		// Object completely deleted is expected
		return
	}
	for _, f := range updated.Finalizers {
		if f == WorkstationFinalizer {
			t.Fatalf("expected finalizer to be removed on delete")
		}
	}
}
