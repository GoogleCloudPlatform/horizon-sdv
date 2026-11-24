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
	"strings"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	parentConfig := &wsv1alpha1.WorkstationConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "parent-config", Namespace: "cloud-workstations"},
	}

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(ws, parentConfig).
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

var guardWsKey = types.NamespacedName{Namespace: "cloud-workstations", Name: "test-ws"}

func guardConfig(name string, deleting bool) *wsv1alpha1.WorkstationConfig {
	cfg := &wsv1alpha1.WorkstationConfig{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "cloud-workstations"},
		Spec:       wsv1alpha1.WorkstationConfigSpec{ClusterRef: wsv1alpha1.ClusterRef{Name: "parent-cluster"}},
	}
	if deleting {
		now := metav1.NewTime(time.Now())
		cfg.DeletionTimestamp = &now
		cfg.Finalizers = []string{ConfigFinalizer}
	}
	return cfg
}

func guardWorkstation(config string, finalizers ...string) *wsv1alpha1.Workstation {
	return &wsv1alpha1.Workstation{
		ObjectMeta: metav1.ObjectMeta{Name: "test-ws", Namespace: "cloud-workstations", Finalizers: finalizers},
		Spec:       wsv1alpha1.WorkstationSpec{ConfigRef: wsv1alpha1.ConfigRef{Name: config}},
	}
}

func guardKCCWorkstation(config string) *unstructured.Unstructured {
	kccObj := &unstructured.Unstructured{}
	kccObj.SetGroupVersionKind(kccWorkstationGVK)
	kccObj.SetName("test-ws")
	kccObj.SetNamespace("cloud-workstations")
	_ = unstructured.SetNestedField(kccObj.Object, config, "spec", "parentRef", "name")
	return kccObj
}

func guardClient(objs ...client.Object) client.Client {
	return fake.NewClientBuilder().
		WithScheme(setupWorkstationTestScheme()).
		WithObjects(objs...).
		WithStatusSubresource(&wsv1alpha1.Workstation{}, &wsv1alpha1.WorkstationConfig{}).
		Build()
}

func guardReconcile(t *testing.T, r *WorkstationReconciler) ctrl.Result {
	t.Helper()
	res, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: guardWsKey})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	return res
}

func guardWorkstationGone(t *testing.T, c client.Client) bool {
	t.Helper()
	err := c.Get(context.Background(), guardWsKey, &wsv1alpha1.Workstation{})
	if err != nil && !apierrors.IsNotFound(err) {
		t.Fatalf("get workstation: %v", err)
	}
	return apierrors.IsNotFound(err)
}

func guardKCCExists(t *testing.T, c client.Client) bool {
	t.Helper()
	kccObj := &unstructured.Unstructured{}
	kccObj.SetGroupVersionKind(kccWorkstationGVK)
	err := c.Get(context.Background(), guardWsKey, kccObj)
	if err != nil && !apierrors.IsNotFound(err) {
		t.Fatalf("get KCC workstation: %v", err)
	}
	return err == nil
}

func guardExpectEvent(t *testing.T, rec *events.FakeRecorder, reason string) {
	t.Helper()
	select {
	case e := <-rec.Events:
		if !strings.HasPrefix(e, "Warning "+reason+" ") {
			t.Fatalf("expected Warning %s event, got %q", reason, e)
		}
	default:
		t.Fatalf("expected a Warning %s event, got none", reason)
	}
}

func guardExpectNoEvent(t *testing.T, rec *events.FakeRecorder) {
	t.Helper()
	select {
	case e := <-rec.Events:
		t.Fatalf("expected no event, got %q", e)
	default:
	}
}

func TestWorkstationGuard_ConfigDeletingRemovesNewWorkstation(t *testing.T) {
	c := guardClient(guardConfig("parent-config", true), guardWorkstation("parent-config"))
	rec := events.NewFakeRecorder(5)
	r := &WorkstationReconciler{Client: c, Recorder: rec, Scheme: c.Scheme()}

	guardReconcile(t, r)

	if !guardWorkstationGone(t, c) {
		t.Fatalf("expected the Workstation to be deleted while its config is deleting")
	}
	if guardKCCExists(t, c) {
		t.Fatalf("expected no Config Connector Workstation")
	}
	guardExpectEvent(t, rec, reasonConfigDeleting)
}

// A Workstation stuck Pending with our finalizer before this guard existed is cleaned up after upgrade.
func TestWorkstationGuard_StuckPendingWorkstationIsRemoved(t *testing.T) {
	c := guardClient(guardConfig("parent-config", true), guardWorkstation("parent-config", WorkstationFinalizer))
	r := &WorkstationReconciler{Client: c, Scheme: c.Scheme()}

	guardReconcile(t, r)
	if guardWorkstationGone(t, c) {
		t.Fatalf("expected the finalizer to hold the Workstation until reconcileDelete runs")
	}
	guardReconcile(t, r)

	if !guardWorkstationGone(t, c) {
		t.Fatalf("expected the Workstation to be gone after reconcileDelete")
	}
}

func TestWorkstationGuard_ProvisionedWorkstationIsKept(t *testing.T) {
	c := guardClient(guardConfig("parent-config", true), guardWorkstation("parent-config", WorkstationFinalizer), guardKCCWorkstation("parent-config"))
	rec := events.NewFakeRecorder(5)
	r := &WorkstationReconciler{Client: c, Recorder: rec, Scheme: c.Scheme()}

	guardReconcile(t, r)

	if guardWorkstationGone(t, c) {
		t.Fatalf("a provisioned Workstation must not be deleted by the guard")
	}
	if !guardKCCExists(t, c) {
		t.Fatalf("expected the Config Connector Workstation to stay")
	}
	guardExpectNoEvent(t, rec)
}

func TestWorkstationGuard_ConfigOnlyInLiveReadIsKept(t *testing.T) {
	cached := guardClient(guardWorkstation("parent-config"))
	live := guardClient(guardConfig("parent-config", false))
	r := &WorkstationReconciler{Client: cached, APIReader: live, Scheme: cached.Scheme()}

	if res := guardReconcile(t, r); !res.Requeue {
		t.Fatalf("expected requeue after adding the finalizer, got %+v", res)
	}

	updated := &wsv1alpha1.Workstation{}
	if err := cached.Get(context.Background(), guardWsKey, updated); err != nil {
		t.Fatalf("expected the Workstation to be kept when the live read finds its config: %v", err)
	}
	if len(updated.Finalizers) != 1 || updated.Finalizers[0] != WorkstationFinalizer {
		t.Fatalf("expected finalizer %q, got %v", WorkstationFinalizer, updated.Finalizers)
	}
}

// The cache can lag a delete that started a moment ago; an unprovisioned Workstation reads the config live.
func TestWorkstationGuard_StaleCacheStillSeesDelete(t *testing.T) {
	cached := guardClient(guardConfig("parent-config", false), guardWorkstation("parent-config", WorkstationFinalizer))
	live := guardClient(guardConfig("parent-config", true))
	rec := events.NewFakeRecorder(5)
	r := &WorkstationReconciler{Client: cached, APIReader: live, Recorder: rec, Scheme: cached.Scheme()}

	guardReconcile(t, r)

	if guardKCCExists(t, cached) {
		t.Fatalf("expected no Config Connector Workstation when the live config is deleting")
	}
	updated := &wsv1alpha1.Workstation{}
	if err := cached.Get(context.Background(), guardWsKey, updated); err != nil && !apierrors.IsNotFound(err) {
		t.Fatalf("get workstation: %v", err)
	} else if err == nil && updated.DeletionTimestamp.IsZero() {
		t.Fatalf("expected the Workstation to be marked for deletion")
	}
	guardExpectEvent(t, rec, reasonConfigDeleting)
}

func TestWorkstationGuard_ConfigNotFoundRemovesWorkstation(t *testing.T) {
	c := guardClient(guardWorkstation("missing-config"))
	rec := events.NewFakeRecorder(5)
	r := &WorkstationReconciler{Client: c, APIReader: c, Recorder: rec, Scheme: c.Scheme()}

	guardReconcile(t, r)

	if !guardWorkstationGone(t, c) {
		t.Fatalf("expected the Workstation to be deleted when its config does not exist")
	}
	guardExpectEvent(t, rec, reasonConfigNotFound)
}

func TestWorkstationGuard_ConfigNotFoundWithKCCWaits(t *testing.T) {
	c := guardClient(guardWorkstation("missing-config", WorkstationFinalizer), guardKCCWorkstation("missing-config"))
	r := &WorkstationReconciler{Client: c, Scheme: c.Scheme()}

	res := guardReconcile(t, r)

	if res.RequeueAfter <= 0 {
		t.Fatalf("expected a delayed requeue while waiting on the config, got %+v", res)
	}
	updated := &wsv1alpha1.Workstation{}
	if err := c.Get(context.Background(), guardWsKey, updated); err != nil {
		t.Fatalf("expected the provisioned Workstation to be kept: %v", err)
	}
	if updated.Status.Phase != "Pending" {
		t.Fatalf("expected phase Pending, got %q", updated.Status.Phase)
	}
}

func TestWorkstationGuard_UnrelatedConfigUnaffected(t *testing.T) {
	c := guardClient(guardConfig("deleting-config", true), guardConfig("parent-config", false), guardWorkstation("parent-config"))
	rec := events.NewFakeRecorder(5)
	r := &WorkstationReconciler{Client: c, Recorder: rec, Scheme: c.Scheme()}

	if res := guardReconcile(t, r); !res.Requeue {
		t.Fatalf("expected requeue after adding the finalizer, got %+v", res)
	}
	if guardWorkstationGone(t, c) {
		t.Fatalf("a Workstation on a Ready config must not be deleted")
	}
	guardExpectNoEvent(t, rec)
}

// Scenario 11: the config is already deleting when the Workstation first reconciles.
func TestWorkstationGuard_ConfigDeleteFinishes(t *testing.T) {
	c := guardClient(guardConfig("parent-config", true), guardWorkstation("parent-config"))
	cfgReconciler := &WorkstationConfigReconciler{Client: c, Scheme: c.Scheme(), HostServiceAccount: testHostServiceAccount}
	wsReconciler := &WorkstationReconciler{Client: c, Scheme: c.Scheme()}
	cfgReq := ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "cloud-workstations", Name: "parent-config"}}

	if _, err := cfgReconciler.Reconcile(context.Background(), cfgReq); err != nil {
		t.Fatalf("config reconcile: %v", err)
	}
	guardReconcile(t, wsReconciler)
	if _, err := cfgReconciler.Reconcile(context.Background(), cfgReq); err != nil {
		t.Fatalf("config reconcile: %v", err)
	}

	err := c.Get(context.Background(), cfgReq.NamespacedName, &wsv1alpha1.WorkstationConfig{})
	if !apierrors.IsNotFound(err) {
		t.Fatalf("expected the config delete to finish, got %v", err)
	}
}
