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
	"encoding/json"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	smv1 "storage-mirror-module/api/v1alpha1"
	"storage-mirror-module/internal/catalog"
	"storage-mirror-module/internal/dispatch"
	"storage-mirror-module/internal/volume"
)

// AnnotationVolumeDestroyed stops the controller from recreating a volume destroyed through the API.
const AnnotationVolumeDestroyed = "storagemirror.horizon-sdv.io/volume-destroyed"

const (
	// volumeInitLost: a dispatch whose Workflow has not appeared by then was dropped (for example the webhook
	// accepted it before the Sensor subscribed to the event bus, as happens right after install) and is sent again.
	volumeInitLost = 2 * time.Minute
	// volumeInitRetry: back-off after a volume-init run that started but did not report the catalog.
	volumeInitRetry = 15 * time.Minute
)

// Usage is the volume usage reported by mirror-agent (statfs on the share).
type Usage struct {
	CapacityBytes  int64 `json:"capacityBytes"`
	UsedBytes      int64 `json:"usedBytes"`
	AvailableBytes int64 `json:"availableBytes"`
}

// ConfigReconciler reconciles StorageMirrorConfig/default into the Filestore instance and consumer claims.
type ConfigReconciler struct {
	client.Client
	Namespace         string
	WorkflowNamespace string
	ProjectID         string
	Dispatcher        Dispatcher
	Events            chan event.GenericEvent
	Now               func() time.Time
	// TestNFS replaces Filestore with a plain NFS export (kind end-to-end harness only).
	TestNFS *volume.TestNFS
}

func (r *ConfigReconciler) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

// SetupWithManager watches the config, the Filestore instance, volume-init Workflows and API events.
func (r *ConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	fs := &unstructured.Unstructured{}
	fs.SetGroupVersionKind(volume.FilestoreGVK)
	wf := &unstructured.Unstructured{}
	wf.SetGroupVersionKind(WorkflowGVK)
	toConfig := handler.EnqueueRequestsFromMapFunc(func(context.Context, client.Object) []reconcile.Request {
		return []reconcile.Request{{NamespacedName: client.ObjectKey{Namespace: r.Namespace, Name: smv1.ConfigName}}}
	})
	volumeInit := predicate.NewPredicateFuncs(func(o client.Object) bool {
		return o.GetLabels()[smv1.LabelModule] == smv1.ModuleName && o.GetLabels()[smv1.LabelMirror] == ""
	})
	b := ctrl.NewControllerManagedBy(mgr).
		Named("storagemirrorconfig").
		For(&smv1.StorageMirrorConfig{}).
		Watches(wf, toConfig, builder.WithPredicates(volumeInit))
	if r.TestNFS == nil {
		b = b.Watches(fs, toConfig)
	}
	if r.Events != nil {
		b = b.WatchesRawSource(source.Channel(r.Events, toConfig))
	}
	return b.Complete(r)
}

// Reconcile handles StorageMirrorConfig/default.
func (r *ConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	if req.Name != smv1.ConfigName {
		return ctrl.Result{}, nil
	}
	var cfg smv1.StorageMirrorConfig
	if err := r.Get(ctx, req.NamespacedName, &cfg); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	st := *cfg.Status.DeepCopy()
	st.ObservedGeneration = cfg.Generation

	if cfg.Annotations[AnnotationVolumeDestroyed] == "true" {
		st.Volume = smv1.VolumeStatus{Phase: "Destroyed"}
		cond(&st, "VolumeReady", metav1.ConditionFalse, "Destroyed", "volume destroyed through the API; remove the annotation to recreate", cfg.Generation)
		return ctrl.Result{}, r.writeStatus(ctx, &cfg, st)
	}

	var ip string
	if r.TestNFS != nil {
		ip = r.TestNFS.Server
		st.Volume.InstanceName = "test-nfs"
		st.Volume.IP = ip
		st.Volume.CapacityGb = cfg.Spec.Volume.CapacityGb
	} else {
		inst, err := r.ensureInstance(ctx, &cfg, &st)
		if err != nil {
			cond(&st, "VolumeReady", metav1.ConditionFalse, "InstanceError", err.Error(), cfg.Generation)
			_ = r.writeStatus(ctx, &cfg, st)
			return ctrl.Result{RequeueAfter: time.Minute}, err
		}
		ip = volume.InstanceIP(inst)
		st.Volume.InstanceName = volume.InstanceID(cfg.Spec.Volume)
		st.Volume.IP = ip
		st.Volume.CapacityGb = volume.InstanceCapacityGb(inst)
		if !volume.InstanceReady(inst) {
			ip = ""
		}
	}
	if ip == "" {
		st.Volume.Phase = "Provisioning"
		cond(&st, "VolumeReady", metav1.ConditionFalse, "Provisioning", "waiting for Config Connector to report the Filestore instance Ready with an IP", cfg.Generation)
		return ctrl.Result{RequeueAfter: 30 * time.Second}, r.writeStatus(ctx, &cfg, st)
	}
	st.Volume.Phase = "Ready"
	cond(&st, "VolumeReady", metav1.ConditionTrue, "Ready", "", cfg.Generation)

	conflicts, err := r.ensureConsumers(ctx, &cfg, ip)
	if err != nil {
		cond(&st, "ConsumersReady", metav1.ConditionFalse, "Error", err.Error(), cfg.Generation)
		_ = r.writeStatus(ctx, &cfg, st)
		return ctrl.Result{RequeueAfter: time.Minute}, err
	}
	if len(conflicts) > 0 {
		cond(&st, "ConsumersReady", metav1.ConditionFalse, "JenkinsClaimConflict", fmt.Sprintf("claims not owned by the module left untouched: %v", conflicts), cfg.Generation)
	} else {
		cond(&st, "ConsumersReady", metav1.ConditionTrue, "Bound", "", cfg.Generation)
	}

	if err := r.reconcileCatalog(ctx, &cfg, &st); err != nil {
		logger.Error(err, "catalog")
	}
	requeue := 5 * time.Minute
	if c := apimeta.FindStatusCondition(st.Conditions, "VolumeInitialized"); c == nil || c.Status != metav1.ConditionTrue {
		requeue = time.Minute
	}
	return ctrl.Result{RequeueAfter: requeue}, r.writeStatus(ctx, &cfg, st)
}

func (r *ConfigReconciler) ensureInstance(ctx context.Context, cfg *smv1.StorageMirrorConfig, st *smv1.StorageMirrorConfigStatus) (*unstructured.Unstructured, error) {
	if err := volume.ValidateCapacity(cfg.Spec.Volume.CapacityGb); err != nil {
		cond(st, "ResizeAccepted", metav1.ConditionFalse, "ResizeRejected", err.Error(), cfg.Generation)
		return nil, err
	}
	desired := volume.FilestoreInstance(r.Namespace, r.ProjectID, cfg.Spec.Volume)
	cur := &unstructured.Unstructured{}
	cur.SetGroupVersionKind(volume.FilestoreGVK)
	err := r.Get(ctx, client.ObjectKey{Namespace: r.Namespace, Name: volume.FilestoreInstanceName}, cur)
	if apierrors.IsNotFound(err) {
		if err := r.Create(ctx, desired); err != nil {
			return nil, fmt.Errorf("create FilestoreInstance: %w", err)
		}
		return desired, nil
	}
	if err != nil {
		return nil, err
	}
	have := volume.InstanceCapacityGb(cur)
	want := cfg.Spec.Volume.CapacityGb
	switch {
	case want > have:
		volume.SetInstanceCapacityGb(cur, want)
		if err := r.Update(ctx, cur); err != nil {
			return nil, fmt.Errorf("resize FilestoreInstance: %w", err)
		}
		cond(st, "ResizeAccepted", metav1.ConditionTrue, "Resizing", fmt.Sprintf("%d -> %d GiB", have, want), cfg.Generation)
	case want < have:
		cond(st, "ResizeAccepted", metav1.ConditionFalse, "ResizeRejected", fmt.Sprintf("shrinking from %d to %d GiB is not supported", have, want), cfg.Generation)
	default:
		cond(st, "ResizeAccepted", metav1.ConditionTrue, "Steady", "", cfg.Generation)
	}
	return cur, nil
}

// ensureConsumers creates or updates the static PVs and claims; foreign claims are reported, never touched.
func (r *ConfigReconciler) ensureConsumers(ctx context.Context, cfg *smv1.StorageMirrorConfig, ip string) ([]string, error) {
	var conflicts []string
	for _, c := range volume.Consumers(cfg.Spec) {
		var pvc corev1.PersistentVolumeClaim
		err := r.Get(ctx, client.ObjectKey{Namespace: c.Namespace, Name: c.ClaimName}, &pvc)
		if err == nil && pvc.Labels[smv1.LabelModule] != smv1.ModuleName {
			conflicts = append(conflicts, c.Namespace+"/"+c.ClaimName)
			continue
		}
		if err != nil && !apierrors.IsNotFound(err) {
			return conflicts, err
		}
		pv := volume.PersistentVolume(c, cfg.Spec.Volume, ip)
		if r.TestNFS != nil {
			pv = volume.PersistentVolumeTestNFS(c, cfg.Spec.Volume, *r.TestNFS)
		}
		if err := r.ensurePV(ctx, pv); err != nil {
			return conflicts, err
		}
		if apierrors.IsNotFound(err) {
			if err := r.Create(ctx, volume.PersistentVolumeClaim(c)); err != nil && !apierrors.IsAlreadyExists(err) {
				return conflicts, fmt.Errorf("create claim %s/%s: %w", c.Namespace, c.ClaimName, err)
			}
		}
	}
	return conflicts, nil
}

func (r *ConfigReconciler) ensurePV(ctx context.Context, want *corev1.PersistentVolume) error {
	var pv corev1.PersistentVolume
	err := r.Get(ctx, client.ObjectKey{Name: want.Name}, &pv)
	if apierrors.IsNotFound(err) {
		return r.Create(ctx, want)
	}
	if err != nil {
		return err
	}
	if pv.Labels[smv1.LabelModule] != smv1.ModuleName {
		return fmt.Errorf("PersistentVolume %s exists and is not owned by the module", want.Name)
	}
	if pv.Spec.Capacity.Storage().Cmp(want.Spec.Capacity[corev1.ResourceStorage]) != 0 {
		pv.Spec.Capacity = want.Spec.Capacity
		return r.Update(ctx, &pv)
	}
	return nil
}

// reconcileCatalog starts volume-init until the agent has reported the catalog, applies usage and adoption.
func (r *ConfigReconciler) reconcileCatalog(ctx context.Context, cfg *smv1.StorageMirrorConfig, st *smv1.StorageMirrorConfigStatus) error {
	var cm corev1.ConfigMap
	err := r.Get(ctx, client.ObjectKey{Namespace: r.Namespace, Name: catalog.ConfigMapName}, &cm)
	if apierrors.IsNotFound(err) {
		cond(st, "VolumeInitialized", metav1.ConditionFalse, "Pending", "waiting for storage-mirror-volume-init to report the catalog", cfg.Generation)
		return r.maybeVolumeInit(ctx, st)
	}
	if err != nil {
		return err
	}
	cond(st, "VolumeInitialized", metav1.ConditionTrue, "CatalogReported", "", cfg.Generation)
	var u Usage
	if json.Unmarshal([]byte(cm.Data[catalog.KeyUsage]), &u) == nil && u.CapacityBytes > 0 {
		st.Volume.UsedBytes = u.UsedBytes
		st.Volume.AvailableBytes = u.AvailableBytes
		if t, err := time.Parse(time.RFC3339, cm.Data[catalog.KeyReportedAt]); err == nil {
			mt := metav1.NewTime(t)
			st.Volume.UsageReportedAt = &mt
		}
	}
	if cfg.Spec.AdoptExisting {
		snap, err := catalog.Parse([]byte(cm.Data[catalog.KeyMetadata]), RootKey(cfg))
		if err != nil {
			return err
		}
		_, err = Import(ctx, r.Client, r.Namespace, snap, nil, "adoptExisting")
		return err
	}
	return nil
}

func (r *ConfigReconciler) maybeVolumeInit(ctx context.Context, st *smv1.StorageMirrorConfigStatus) error {
	runs, err := ListRuns(ctx, r.Client, r.WorkflowNamespace, "")
	if err != nil {
		return err
	}
	seen := false
	for _, run := range runs {
		if run.Template != dispatch.TemplateVolumeInit {
			continue
		}
		if run.Active() {
			return nil
		}
		if st.VolumeInitRequestID != "" && run.RequestID == st.VolumeInitRequestID {
			seen = true
		}
	}
	// The webhook returns no Workflow name: give a dispatched request time to appear, and a run that appeared time
	// to finish, before retrying.
	if at := st.VolumeInitRequestedAt; at != nil {
		wait := volumeInitLost
		if seen {
			wait = volumeInitRetry
		}
		if r.now().Sub(at.Time) < wait {
			return nil
		}
	}
	id, err := r.Dispatcher.Dispatch(ctx, dispatch.Request{
		Template:    dispatch.TemplateVolumeInit,
		SubmittedBy: dispatch.ControllerUser,
		Params:      map[string]string{},
	})
	if err != nil {
		return err
	}
	st.VolumeInitRequestID = id
	t := metav1.NewTime(r.now())
	st.VolumeInitRequestedAt = &t
	return nil
}

// RootKey is the metadata.yaml root key: the root subdirectory, a single path component (CRD pattern), as in the
// Jenkins scripts and mirror-agent.
func RootKey(cfg *smv1.StorageMirrorConfig) string {
	if cfg.Spec.Layout.RootSubdir != "" {
		return cfg.Spec.Layout.RootSubdir
	}
	return "horizon-mirrors"
}

func (r *ConfigReconciler) writeStatus(ctx context.Context, cfg *smv1.StorageMirrorConfig, st smv1.StorageMirrorConfigStatus) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var cur smv1.StorageMirrorConfig
		if err := r.Get(ctx, client.ObjectKeyFromObject(cfg), &cur); err != nil {
			return client.IgnoreNotFound(err)
		}
		cur.Status = st
		return r.Status().Update(ctx, &cur)
	})
}

func cond(st *smv1.StorageMirrorConfigStatus, t string, s metav1.ConditionStatus, reason, msg string, gen int64) {
	meta.SetStatusCondition(&st.Conditions, metav1.Condition{Type: t, Status: s, Reason: reason, Message: truncate(msg, 512), ObservedGeneration: gen})
}
