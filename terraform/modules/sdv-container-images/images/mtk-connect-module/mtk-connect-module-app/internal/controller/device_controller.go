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
	"errors"
	"fmt"
	"reflect"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"mtk-connect-module/internal/mtk"
)

const (
	// A device change reaches MTK Connect as soon as the watch event arrives. While a device waits for its
	// testbench, the instance or a lower-numbered device, it is retried at this interval, which is inside
	// the 30 seconds the story allows.
	deviceRetryDelay  = 15 * time.Second
	deviceResyncDelay = 5 * time.Minute // re-applies the interface, repairing changes made in MTK Connect

	reasonTestbenchNotFound = "TestbenchNotFound"
	reasonTestbenchNotReady = "TestbenchNotReady"
	reasonTestbenchDeleting = "TestbenchDeleting"
	reasonWaitingPrior      = "WaitingForPredecessor"
)

// DeviceAPI is the part of the MTK Connect client the device reconciler needs.
type DeviceAPI interface {
	FindDevice(ctx context.Context, registration string, index int) (*mtk.Device, error)
	CreateDevice(ctx context.Context, agentID, name string) (mtk.Device, error)
	ConfigureDevice(ctx context.Context, agentID string, index int, payload map[string]interface{}) error
	DeleteDevice(ctx context.Context, agentID string, index int) error
}

// DeviceReconciler reconciles MTKConnectDevice.
//
// The server numbers the devices of an agent itself, in creation order, and the interface of a device is
// addressed by that number. The reconciler therefore creates a device only when the device below it exists,
// checks the number it got, and runs with one worker so two devices of a testbench are never created at once.
type DeviceReconciler struct {
	Client client.Client
	Direct client.Client

	ManagerNamespace string
	InstanceName     string

	NewAPI func(serverURL, username, key string) DeviceAPI
	Now    func() time.Time
}

type deviceStatus struct {
	Phase      string
	AgentID    string
	DeviceID   string
	Index      int64
	Generation int64
	Conditions map[string]*condition
}

func (r *DeviceReconciler) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

func (r *DeviceReconciler) instanceName() string {
	return orDefault(r.InstanceName, instanceObjectDefault)
}

// SetupWithManager registers the controller with one worker. Status writes change neither the generation nor
// the annotations, so they do not trigger another reconcile.
func (r *DeviceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(DeviceGVK)
	changed := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			return e.ObjectOld.GetGeneration() != e.ObjectNew.GetGeneration() ||
				!e.ObjectOld.GetDeletionTimestamp().Equal(e.ObjectNew.GetDeletionTimestamp()) ||
				!reflect.DeepEqual(e.ObjectOld.GetAnnotations(), e.ObjectNew.GetAnnotations())
		},
	}
	return ctrl.NewControllerManagedBy(mgr).
		Named("mtkconnectdevice").
		WithOptions(controller.Options{MaxConcurrentReconciles: 1}).
		For(u, builder.WithPredicates(changed)).
		Complete(r)
}

// Reconcile creates the device in MTK Connect and applies its interface.
func (r *DeviceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	dev := &unstructured.Unstructured{}
	dev.SetGroupVersionKind(DeviceGVK)
	if err := r.Client.Get(ctx, req.NamespacedName, dev); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if !dev.GetDeletionTimestamp().IsZero() {
		return r.finalize(ctx, dev)
	}

	st := loadDeviceStatus(dev)
	st.Generation = dev.GetGeneration()

	spec, err := parseDeviceSpec(dev)
	if err != nil {
		log.FromContext(ctx).Error(err, "invalid MTKConnectDevice spec")
		r.setCondition(ctx, dev, st, DevCondSpecValid, false, "InvalidSpec", err.Error())
		st.Phase = DevFailed
		return ctrl.Result{}, r.writeStatus(ctx, dev, st)
	}
	r.setCondition(ctx, dev, st, DevCondSpecValid, true, "Valid", "")

	if !controllerutil.ContainsFinalizer(dev, FinalizerDeviceCleanup) {
		orig := dev.DeepCopy()
		controllerutil.AddFinalizer(dev, FinalizerDeviceCleanup)
		if err := r.Client.Patch(ctx, dev, client.MergeFromWithOptions(orig, client.MergeFromWithOptimisticLock{})); err != nil {
			return ctrl.Result{}, fmt.Errorf("add finalizer: %w", err)
		}
	}

	after := r.sync(ctx, dev, spec, st)
	if err := r.writeStatus(ctx, dev, st); err != nil {
		return ctrl.Result{}, err
	}
	if after <= 0 {
		return ctrl.Result{}, nil
	}
	return ctrl.Result{RequeueAfter: after}, nil
}

// sync drives the device towards the spec and returns the delay until the next reconcile.
func (r *DeviceReconciler) sync(ctx context.Context, dev *unstructured.Unstructured, spec *DeviceSpec, st *deviceStatus) time.Duration {
	logger := log.FromContext(ctx)
	if st.Phase == "" || st.Phase == DevFailed {
		st.Phase = DevPending
	}
	index := int(spec.Index)

	if dup, err := r.duplicateOf(ctx, dev, spec); err != nil {
		r.setCondition(ctx, dev, st, DevCondConfigured, false, "LookupFailed", err.Error())
		return deviceRetryDelay
	} else if dup != "" {
		msg := fmt.Sprintf("device %s already uses index %d of testbench %s", dup, index, spec.TestbenchRef)
		r.setCondition(ctx, dev, st, DevCondConfigured, false, "DuplicateIndex", msg)
		st.Phase = DevFailed
		return deviceResyncDelay
	}

	tb := &unstructured.Unstructured{}
	tb.SetGroupVersionKind(TestbenchGVK)
	if err := r.Client.Get(ctx, types.NamespacedName{Namespace: dev.GetNamespace(), Name: spec.TestbenchRef}, tb); err != nil {
		reason, msg := reasonTestbenchNotFound, fmt.Sprintf("testbench %s not found", spec.TestbenchRef)
		if client.IgnoreNotFound(err) != nil {
			reason, msg = reasonTestbenchNotReady, err.Error()
		}
		r.setCondition(ctx, dev, st, DevCondTestbench, false, reason, msg)
		return deviceRetryDelay
	}
	if !tb.GetDeletionTimestamp().IsZero() {
		r.setCondition(ctx, dev, st, DevCondTestbench, false, reasonTestbenchDeleting, "testbench "+spec.TestbenchRef+" is being deleted")
		return deviceRetryDelay
	}
	tbPhase, _, _ := unstructured.NestedString(tb.Object, "status", "phase")
	agentID, _, _ := unstructured.NestedString(tb.Object, "status", "agentId")
	registration, _, _ := unstructured.NestedString(tb.Object, "status", "registration")
	if tbPhase == TbFailed || agentID == "" || registration == "" {
		r.setCondition(ctx, dev, st, DevCondTestbench, false, reasonTestbenchNotReady,
			fmt.Sprintf("testbench %s has no usable agent (phase %q)", spec.TestbenchRef, tbPhase))
		return deviceRetryDelay
	}
	r.setCondition(ctx, dev, st, DevCondTestbench, true, "Ready", "")
	r.ensureOwner(ctx, dev, tb)

	conn, err := loadConnection(ctx, r.Client, r.Direct, r.ManagerNamespace, r.instanceName())
	if err != nil {
		logger.Info("MTK Connect is not reachable yet", "reason", err.Error())
		r.setCondition(ctx, dev, st, DevCondInstanceReady, false, reasonInstanceNotReady, err.Error())
		return deviceRetryDelay
	}
	r.setCondition(ctx, dev, st, DevCondInstanceReady, true, "Ready", "")
	api := r.NewAPI(conn.ServerURL, conn.Username, conn.Key)

	payload, err := spec.profileDevice().Payload()
	if err != nil {
		r.setCondition(ctx, dev, st, DevCondSpecValid, false, "InvalidSpec", err.Error())
		st.Phase = DevFailed
		return 0
	}

	existing, err := api.FindDevice(ctx, registration, index)
	if err != nil {
		logger.Error(err, "look up device failed")
		r.setCondition(ctx, dev, st, DevCondConfigured, false, "LookupFailed", err.Error())
		return deviceRetryDelay
	}
	if existing == nil {
		// A wrong index is not repaired by trying again: the server would hand out yet another number.
		if prev := st.Conditions[DevCondConfigured]; prev != nil && !prev.Status && prev.Reason == "IndexMismatch" {
			st.Phase = DevFailed
			return deviceResyncDelay
		}
		if index > 1 {
			prior, err := api.FindDevice(ctx, registration, index-1)
			if err != nil {
				r.setCondition(ctx, dev, st, DevCondConfigured, false, "LookupFailed", err.Error())
				return deviceRetryDelay
			}
			if prior == nil {
				r.setCondition(ctx, dev, st, DevCondConfigured, false, reasonWaitingPrior,
					fmt.Sprintf("waiting for device %d of testbench %s: devices are created in index order", index-1, spec.TestbenchRef))
				return deviceRetryDelay
			}
		}
		// Record where the device will be before it is created. If the controller stops, or the status write
		// after the create fails, finalize still knows which device to delete (deleting a missing one is fine).
		st.AgentID = agentID
		st.Index = spec.Index
		if err := r.writeStatus(ctx, dev, st); err != nil {
			logger.Error(err, "record device before create failed")
			r.setCondition(ctx, dev, st, DevCondConfigured, false, "StatusFailed", err.Error())
			return deviceRetryDelay
		}
		created, err := api.CreateDevice(ctx, agentID, spec.Name)
		if err != nil {
			logger.Error(err, "create device failed")
			r.setCondition(ctx, dev, st, DevCondConfigured, false, "CreateFailed", err.Error())
			return deviceRetryDelay
		}
		if err := r.checkNumber(ctx, api, agentID, registration, index, created); err != nil {
			r.setCondition(ctx, dev, st, DevCondConfigured, false, "IndexMismatch", err.Error())
			st.Phase = DevFailed
			return deviceResyncDelay
		}
		existing = &created
		r.event(ctx, dev, corev1.EventTypeNormal, "DeviceCreated", fmt.Sprintf("created device %d of agent %s", index, agentID))
	}
	st.AgentID = agentID
	st.Index = spec.Index
	st.DeviceID = existing.ID

	if err := api.ConfigureDevice(ctx, agentID, index, payload); err != nil {
		logger.Error(err, "configure device failed")
		r.setCondition(ctx, dev, st, DevCondConfigured, false, "ConfigureFailed", err.Error())
		return deviceRetryDelay
	}
	r.setCondition(ctx, dev, st, DevCondConfigured, true, "Configured", "")
	st.Phase = DevConfigured
	return deviceResyncDelay
}

// checkNumber confirms that a new device got the index the resource asked for. If it did not, the device is
// removed again so no stray device is left under a number nobody asked for.
func (r *DeviceReconciler) checkNumber(ctx context.Context, api DeviceAPI, agentID, registration string, want int, created mtk.Device) error {
	if created.Index == want {
		return nil
	}
	if created.Index == 0 {
		found, err := api.FindDevice(ctx, registration, want)
		if err == nil && found != nil {
			return nil
		}
		return fmt.Errorf("the new device was not found at index %d; devices must be created in index order without gaps", want)
	}
	if err := api.DeleteDevice(ctx, agentID, created.Index); err != nil {
		log.FromContext(ctx).Error(err, "remove misnumbered device failed", "index", created.Index)
	}
	return fmt.Errorf("MTK Connect numbered the new device %d, not %d; devices must be created in index order without gaps", created.Index, want)
}

// duplicateOf returns the name of an older device resource with the same testbench and index, if any. Only
// the oldest one owns the index; a later one fails instead of taking over the device.
func (r *DeviceReconciler) duplicateOf(ctx context.Context, dev *unstructured.Unstructured, spec *DeviceSpec) (string, error) {
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(DeviceGVK.GroupVersion().WithKind(KindDevice + "List"))
	if err := r.Client.List(ctx, list, client.InNamespace(dev.GetNamespace())); err != nil {
		return "", fmt.Errorf("list devices: %w", err)
	}
	for i := range list.Items {
		other := &list.Items[i]
		if other.GetName() == dev.GetName() || !other.GetDeletionTimestamp().IsZero() {
			continue
		}
		o, err := parseDeviceSpec(other)
		if err != nil || o.TestbenchRef != spec.TestbenchRef || o.Index != spec.Index {
			continue
		}
		if older(other, dev) {
			return other.GetName(), nil
		}
	}
	return "", nil
}

func older(a, b *unstructured.Unstructured) bool {
	ta, tb := a.GetCreationTimestamp(), b.GetCreationTimestamp()
	if !ta.Equal(&tb) {
		return ta.Before(&tb)
	}
	return a.GetName() < b.GetName()
}

// ensureOwner makes the testbench the owner of the device, so deleting the testbench removes its devices.
func (r *DeviceReconciler) ensureOwner(ctx context.Context, dev, tb *unstructured.Unstructured) {
	for _, ref := range dev.GetOwnerReferences() {
		if ref.UID == tb.GetUID() {
			return
		}
	}
	orig := dev.DeepCopy()
	dev.SetOwnerReferences(append(dev.GetOwnerReferences(), metav1.OwnerReference{
		APIVersion: Group + "/" + Version,
		Kind:       KindTestbench,
		Name:       tb.GetName(),
		UID:        tb.GetUID(),
	}))
	if err := r.Client.Patch(ctx, dev, client.MergeFrom(orig)); err != nil {
		log.FromContext(ctx).Error(err, "set owner reference failed")
	}
}

// finalize removes the device from MTK Connect, then lets the resource go. When the testbench is gone or
// going, its agent is deleted with all its devices, so there is nothing left to delete.
func (r *DeviceReconciler) finalize(ctx context.Context, dev *unstructured.Unstructured) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(dev, FinalizerDeviceCleanup) {
		return ctrl.Result{}, nil
	}
	logger := log.FromContext(ctx)

	if dev.GetAnnotations()[AnnotationSkipCleanup] == "true" {
		r.event(ctx, dev, corev1.EventTypeWarning, "CleanupSkipped", "the MTK Connect device was not deleted ("+AnnotationSkipCleanup+")")
		return ctrl.Result{}, r.removeFinalizer(ctx, dev)
	}

	st := loadDeviceStatus(dev)
	if st.AgentID == "" || st.Index == 0 {
		return ctrl.Result{}, r.removeFinalizer(ctx, dev)
	}
	if st.Phase != DevDeleting {
		st.Phase = DevDeleting
		st.Generation = dev.GetGeneration()
		if err := r.writeStatus(ctx, dev, st); err != nil {
			logger.Error(err, "record Deleting phase failed")
		}
	}

	spec, err := parseDeviceSpec(dev)
	if err == nil {
		tb := &unstructured.Unstructured{}
		tb.SetGroupVersionKind(TestbenchGVK)
		gerr := r.Client.Get(ctx, types.NamespacedName{Namespace: dev.GetNamespace(), Name: spec.TestbenchRef}, tb)
		if client.IgnoreNotFound(gerr) == nil && (gerr != nil || !tb.GetDeletionTimestamp().IsZero()) {
			return ctrl.Result{}, r.removeFinalizer(ctx, dev)
		}
	}

	conn, err := loadConnection(ctx, r.Client, r.Direct, r.ManagerNamespace, r.instanceName())
	if err != nil {
		msg := "cannot delete the MTK Connect device: " + err.Error()
		if errors.Is(err, errInstanceMissing) {
			msg += "; set annotation " + AnnotationSkipCleanup + "=true to delete the resource anyway"
		}
		logger.Info(msg)
		r.event(ctx, dev, corev1.EventTypeWarning, "CleanupBlocked", msg)
		return ctrl.Result{RequeueAfter: deviceRetryDelay}, nil
	}
	api := r.NewAPI(conn.ServerURL, conn.Username, conn.Key)
	if err := api.DeleteDevice(ctx, st.AgentID, int(st.Index)); err != nil {
		logger.Error(err, "delete device failed")
		r.event(ctx, dev, corev1.EventTypeWarning, "CleanupFailed", err.Error())
		return ctrl.Result{RequeueAfter: deviceRetryDelay}, nil
	}
	r.event(ctx, dev, corev1.EventTypeNormal, "DeviceDeleted", fmt.Sprintf("deleted device %d of agent %s", st.Index, st.AgentID))
	return ctrl.Result{}, r.removeFinalizer(ctx, dev)
}

func (r *DeviceReconciler) removeFinalizer(ctx context.Context, dev *unstructured.Unstructured) error {
	orig := dev.DeepCopy()
	controllerutil.RemoveFinalizer(dev, FinalizerDeviceCleanup)
	err := r.Client.Patch(ctx, dev, client.MergeFromWithOptions(orig, client.MergeFromWithOptimisticLock{}))
	if err != nil {
		return client.IgnoreNotFound(fmt.Errorf("remove finalizer: %w", err))
	}
	return nil
}

// setCondition records a condition and emits a Warning Event when a failure starts or its message changes.
// Waiting for another resource is not a failure and raises no Event.
func (r *DeviceReconciler) setCondition(ctx context.Context, dev *unstructured.Unstructured, st *deviceStatus, typ string, ok bool, reason, message string) {
	prev := st.Conditions[typ]
	c := &condition{Type: typ, Status: ok, Reason: reason, Message: message, LastTransitionTime: r.now().UTC().Format(time.RFC3339)}
	if prev != nil && prev.Status == ok {
		c.LastTransitionTime = prev.LastTransitionTime
	}
	st.Conditions[typ] = c
	waiting := map[string]bool{
		reasonTestbenchNotFound: true, reasonTestbenchNotReady: true, reasonTestbenchDeleting: true,
		reasonWaitingPrior: true, reasonInstanceNotReady: true,
	}
	if !ok && !waiting[reason] && (prev == nil || prev.Status != ok || prev.Message != message) {
		r.event(ctx, dev, corev1.EventTypeWarning, reason, message)
	}
}

func (r *DeviceReconciler) event(ctx context.Context, dev *unstructured.Unstructured, typ, reason, message string) {
	recordEvent(ctx, r.Direct, dev, r.now(), typ, reason, message)
}

func loadDeviceStatus(dev *unstructured.Unstructured) *deviceStatus {
	base := loadStatus(dev)
	st := &deviceStatus{Phase: base.Phase, Conditions: base.Conditions}
	st.AgentID, _, _ = unstructured.NestedString(dev.Object, "status", "agentId")
	st.DeviceID, _, _ = unstructured.NestedString(dev.Object, "status", "deviceId")
	st.Index, _, _ = unstructured.NestedInt64(dev.Object, "status", "index")
	return st
}

func (r *DeviceReconciler) writeStatus(ctx context.Context, dev *unstructured.Unstructured, st *deviceStatus) error {
	conds := make([]interface{}, 0, len(deviceConditionOrder))
	for _, typ := range deviceConditionOrder {
		c := st.Conditions[typ]
		if c == nil {
			continue
		}
		status := "False"
		if c.Status {
			status = "True"
		}
		conds = append(conds, map[string]interface{}{
			"type":               c.Type,
			"status":             status,
			"reason":             c.Reason,
			"message":            c.Message,
			"lastTransitionTime": c.LastTransitionTime,
		})
	}
	status := map[string]interface{}{
		"phase":              st.Phase,
		"observedGeneration": st.Generation,
		"conditions":         conds,
	}
	if st.AgentID != "" {
		status["agentId"] = st.AgentID
	}
	if st.DeviceID != "" {
		status["deviceId"] = st.DeviceID
	}
	if st.Index != 0 {
		status["index"] = st.Index
	}
	orig := dev.DeepCopy()
	if err := unstructured.SetNestedField(dev.Object, status, "status"); err != nil {
		return fmt.Errorf("build status: %w", err)
	}
	if err := r.Client.Status().Patch(ctx, dev, client.MergeFrom(orig)); err != nil {
		return fmt.Errorf("patch status: %w", err)
	}
	return nil
}
