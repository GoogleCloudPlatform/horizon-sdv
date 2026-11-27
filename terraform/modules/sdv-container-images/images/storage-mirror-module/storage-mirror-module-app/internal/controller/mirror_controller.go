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
	"fmt"
	"strings"
	"sync"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	smv1 "storage-mirror-module/api/v1alpha1"
	"storage-mirror-module/internal/dispatch"
	"storage-mirror-module/internal/state"
)

const (
	// AnnotationSyncRequestID carries the request id the API chose for the pending sync or create.
	AnnotationSyncRequestID = "storagemirror.horizon-sdv.io/sync-request-id"
	// AnnotationSyncRequestedBy records who asked for the sync.
	AnnotationSyncRequestedBy = "storagemirror.horizon-sdv.io/sync-requested-by"
	// AnnotationSubmittedFrom records the Horizon client that asked for the run.
	AnnotationSubmittedFrom = "storagemirror.horizon-sdv.io/submitted-from"

	pendingRequestTTL = 2 * time.Minute
	statusWriteEvery  = 15 * time.Second
	progressStaleAge  = 60 * time.Second
)

// Dispatcher starts runs (implemented by dispatch.Dispatcher).
type Dispatcher interface {
	Dispatch(ctx context.Context, r dispatch.Request) (string, error)
}

// MirrorReconciler keeps StorageMirror status in step with the module's Workflows and starts runs.
type MirrorReconciler struct {
	client.Client
	Namespace         string
	WorkflowNamespace string
	Dispatcher        Dispatcher
	Progress          *state.Store
	// Events lets the REST API wake the reconciler (progress reports, sync requests).
	Events chan event.GenericEvent
	Now    func() time.Time

	// inflight remembers dispatches until their Workflow shows up: the cached CR read by the next reconcile may not
	// carry the pendingRequestId yet, and the webhook returns no Workflow name to check against.
	mu       sync.Mutex
	inflight map[string]inflightRequest
}

type inflightRequest struct {
	id string
	at time.Time
}

// inFlight reports a recent dispatch for the mirror whose Workflow has not appeared yet.
func (r *MirrorReconciler) inFlight(name string, runs []Run) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	req, ok := r.inflight[name]
	if !ok {
		return false
	}
	for _, run := range runs {
		if run.RequestID == req.id {
			delete(r.inflight, name)
			return false
		}
	}
	if r.now().Sub(req.at) > pendingRequestTTL {
		delete(r.inflight, name)
		return false
	}
	return true
}

func (r *MirrorReconciler) remember(name, id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.inflight == nil {
		r.inflight = map[string]inflightRequest{}
	}
	r.inflight[name] = inflightRequest{id: id, at: r.now()}
}

func (r *MirrorReconciler) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

// SetupWithManager registers watches on StorageMirror, module Workflows and the API event channel.
func (r *MirrorReconciler) SetupWithManager(mgr ctrl.Manager) error {
	wf := &unstructured.Unstructured{}
	wf.SetGroupVersionKind(WorkflowGVK)
	moduleOnly := predicate.NewPredicateFuncs(func(o client.Object) bool {
		return o.GetLabels()[smv1.LabelModule] == smv1.ModuleName && o.GetNamespace() == r.WorkflowNamespace
	})
	b := ctrl.NewControllerManagedBy(mgr).
		Named("storagemirror").
		For(&smv1.StorageMirror{}).
		Watches(wf, handler.EnqueueRequestsFromMapFunc(r.mapWorkflow), builder.WithPredicates(moduleOnly))
	if r.Events != nil {
		b = b.WatchesRawSource(source.Channel(r.Events, &handler.EnqueueRequestForObject{}))
	}
	return b.Complete(r)
}

func (r *MirrorReconciler) mapWorkflow(_ context.Context, o client.Object) []reconcile.Request {
	m := o.GetLabels()[smv1.LabelMirror]
	if m == "" {
		return nil
	}
	return []reconcile.Request{{NamespacedName: client.ObjectKey{Namespace: r.Namespace, Name: m}}}
}

// Reconcile handles one mirror.
func (r *MirrorReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	runs, err := ListRuns(ctx, r.Client, r.WorkflowNamespace, req.Name)
	if err != nil {
		return ctrl.Result{}, err
	}
	var m smv1.StorageMirror
	if err := r.Get(ctx, req.NamespacedName, &m); err != nil {
		if apierrors.IsNotFound(err) {
			// Workflows of a mirror without a CR (deleted, or posted straight to the webhook) are not adopted;
			// POST /api/v1/import brings their catalog entry under management.
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	var cfg smv1.StorageMirrorConfig
	if err := r.Get(ctx, client.ObjectKey{Namespace: r.Namespace, Name: smv1.ConfigName}, &cfg); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("StorageMirrorConfig default not found; waiting")
			return ctrl.Result{RequeueAfter: time.Minute}, nil
		}
		return ctrl.Result{}, err
	}

	if !m.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, &m, &cfg, runs)
	}
	if !controllerutil.ContainsFinalizer(&m, smv1.Finalizer) {
		controllerutil.AddFinalizer(&m, smv1.Finalizer)
		if err := r.Update(ctx, &m); err != nil {
			return ctrl.Result{}, err
		}
	}

	status := ComputeStatus(&m, runs, r.Progress, cfg.Spec.Retention, r.now())
	active := status.CurrentRun != nil || status.PendingRequestID != "" || r.inFlight(m.Name, runs)

	switch {
	case !hasRun(runs, dispatch.TemplateCreate) && status.Phase == smv1.PhasePending && !active && !imported(&m):
		if err := dispatch.ValidateCreate(dispatch.MirrorRunParams(&m, &cfg.Spec)); err != nil {
			setCondition(&status, "Dispatched", metav1.ConditionFalse, "InvalidSpec", err.Error(), m.Generation)
			break
		}
		id, err := r.dispatchMirror(ctx, &m, &cfg, dispatch.TemplateCreate)
		if err != nil {
			setCondition(&status, "Dispatched", metav1.ConditionFalse, "DispatchFailed", err.Error(), m.Generation)
		} else {
			r.remember(m.Name, id)
			markPending(&status, id, r.now())
		}
	case syncRequested(&m):
		requested := m.Annotations[smv1.AnnotationSyncRequestedAt]
		status.LastHandledSyncRequest = requested
		if active {
			logger.Info("sync request folded into active run", "mirror", m.Name)
			break
		}
		if m.Spec.Suspend {
			setCondition(&status, "Dispatched", metav1.ConditionFalse, "Suspended", "mirror is suspended", m.Generation)
			break
		}
		id, err := r.dispatchMirror(ctx, &m, &cfg, dispatch.TemplateSync)
		if err != nil {
			setCondition(&status, "Dispatched", metav1.ConditionFalse, "DispatchFailed", err.Error(), m.Generation)
		} else {
			r.remember(m.Name, id)
			markPending(&status, id, r.now())
		}
	}

	if err := r.writeStatus(ctx, &m, status); err != nil {
		return ctrl.Result{}, err
	}
	if status.CurrentRun != nil || status.PendingRequestID != "" {
		return ctrl.Result{RequeueAfter: statusWriteEvery}, nil
	}
	return ctrl.Result{RequeueAfter: 10 * time.Minute}, nil
}

// imported reports a mirror imported from the catalog: it is never created automatically (an uninitialized entry
// waits for a sync request, which creates the mirror when its directory is missing).
func imported(m *smv1.StorageMirror) bool {
	_, ok := m.Annotations[AnnotationImportedStatus]
	return ok
}

func syncRequested(m *smv1.StorageMirror) bool {
	v := m.Annotations[smv1.AnnotationSyncRequestedAt]
	return v != "" && v != m.Status.LastHandledSyncRequest
}

func hasRun(runs []Run, template string) bool {
	for _, run := range runs {
		if run.Template == template {
			return true
		}
	}
	return false
}

func markPending(st *smv1.StorageMirrorStatus, id string, now time.Time) {
	st.PendingRequestID = id
	t := metav1.NewTime(now)
	st.PendingRequestAt = &t
}

func (r *MirrorReconciler) dispatchMirror(ctx context.Context, m *smv1.StorageMirror, cfg *smv1.StorageMirrorConfig, template string) (string, error) {
	params := dispatch.MirrorRunParams(m, &cfg.Spec)
	params["requestId"] = m.Annotations[AnnotationSyncRequestID]
	if params["requestId"] == "" || params["requestId"] == m.Status.PendingRequestID || runWithRequest(m, params["requestId"]) {
		params["requestId"] = dispatch.NewRequestID()
	}
	by := m.Annotations[AnnotationSyncRequestedBy]
	if template == dispatch.TemplateCreate {
		by = m.Annotations[smv1.AnnotationCreatedBy]
	}
	if by == "" {
		by = dispatch.ControllerUser
	}
	params["submittedBy"] = by
	return r.Dispatcher.Dispatch(ctx, dispatch.Request{
		Template:      template,
		SubmittedBy:   by,
		SubmittedFrom: m.Annotations[AnnotationSubmittedFrom],
		Params:        params,
	})
}

func runWithRequest(m *smv1.StorageMirror, id string) bool {
	if m.Status.LastRun != nil && m.Status.LastRun.RequestID == id {
		return true
	}
	for _, h := range m.Status.History {
		if h.RequestID == id {
			return true
		}
	}
	return false
}

func (r *MirrorReconciler) reconcileDelete(ctx context.Context, m *smv1.StorageMirror, cfg *smv1.StorageMirrorConfig, runs []Run) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(m, smv1.Finalizer) {
		return ctrl.Result{}, nil
	}
	if m.Spec.DeletionPolicy != smv1.DeletionPolicyDelete {
		controllerutil.RemoveFinalizer(m, smv1.Finalizer)
		return ctrl.Result{}, r.Update(ctx, m)
	}
	reqID := m.Annotations[smv1.AnnotationDeleteRequestID]
	if reqID != "" {
		for _, run := range runs {
			if run.Template != dispatch.TemplateDelete || run.RequestID != reqID {
				continue
			}
			if run.Succeeded() {
				controllerutil.RemoveFinalizer(m, smv1.Finalizer)
				return ctrl.Result{}, r.Update(ctx, m)
			}
			if !run.Active() {
				st := ComputeStatus(m, runs, r.Progress, cfg.Spec.Retention, r.now())
				setCondition(&st, "Deleted", metav1.ConditionFalse, "DeleteFailed", run.Message, m.Generation)
				return ctrl.Result{RequeueAfter: time.Minute}, r.writeStatus(ctx, m, st)
			}
			return ctrl.Result{RequeueAfter: statusWriteEvery}, nil
		}
		if at := m.Status.PendingRequestAt; at != nil && r.now().Sub(at.Time) < pendingRequestTTL {
			return ctrl.Result{RequeueAfter: statusWriteEvery}, nil
		}
	}
	for _, run := range runs {
		if run.Active() && run.Template != dispatch.TemplateDelete {
			// Wait for the running create or sync; the per-mirror mutex would hold the delete anyway.
			return ctrl.Result{RequeueAfter: statusWriteEvery}, nil
		}
	}
	if r.inFlight(m.Name, runs) {
		return ctrl.Result{RequeueAfter: statusWriteEvery}, nil
	}
	id := dispatch.NewRequestID()
	if _, err := r.Dispatcher.Dispatch(ctx, dispatch.Request{
		Template:    dispatch.TemplateDelete,
		SubmittedBy: firstNonEmpty(m.Annotations[smv1.AnnotationDeleteRequestedBy], dispatch.ControllerUser),
		Params: map[string]string{
			"requestId": id, "mirror": m.Name, "directory": m.DirectoryOrName(), "confirm": m.DirectoryOrName(),
		},
	}); err != nil {
		return ctrl.Result{RequeueAfter: time.Minute}, err
	}
	r.remember(m.Name, id)
	if m.Annotations == nil {
		m.Annotations = map[string]string{}
	}
	m.Annotations[smv1.AnnotationDeleteRequestID] = id
	if err := r.Update(ctx, m); err != nil {
		return ctrl.Result{}, err
	}
	st := ComputeStatus(m, runs, r.Progress, cfg.Spec.Retention, r.now())
	st.Phase = smv1.PhaseDeleting
	markPending(&st, id, r.now())
	return ctrl.Result{RequeueAfter: statusWriteEvery}, r.writeStatus(ctx, m, st)
}

func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if s != "" {
			return s
		}
	}
	return ""
}

func (r *MirrorReconciler) writeStatus(ctx context.Context, m *smv1.StorageMirror, st smv1.StorageMirrorStatus) error {
	st.ObservedGeneration = m.Generation
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var cur smv1.StorageMirror
		if err := r.Get(ctx, client.ObjectKeyFromObject(m), &cur); err != nil {
			return client.IgnoreNotFound(err)
		}
		if !statusChanged(cur.Status, st, r.now()) {
			return nil
		}
		cur.Status = st
		return r.Status().Update(ctx, &cur)
	})
}

// statusChanged skips progress-only writes more often than statusWriteEvery.
func statusChanged(old, new smv1.StorageMirrorStatus, now time.Time) bool {
	o, n := old.DeepCopy(), new.DeepCopy()
	op, np := progressOf(o), progressOf(n)
	if o.CurrentRun != nil {
		o.CurrentRun.Progress = nil
	}
	if n.CurrentRun != nil {
		n.CurrentRun.Progress = nil
	}
	if fmt.Sprintf("%+v", stripTimes(o)) != fmt.Sprintf("%+v", stripTimes(n)) {
		return true
	}
	if (op == nil) != (np == nil) {
		return true
	}
	if op == nil {
		return false
	}
	if op.State != np.State || op.Phase != np.Phase || op.Attempt != np.Attempt {
		return true
	}
	if op.UpdatedAt == nil || np.UpdatedAt == nil {
		return op.UpdatedAt != np.UpdatedAt
	}
	return np.UpdatedAt.Sub(op.UpdatedAt.Time) >= statusWriteEvery || now.Sub(op.UpdatedAt.Time) >= statusWriteEvery && np.UpdatedAt.After(op.UpdatedAt.Time)
}

func progressOf(s *smv1.StorageMirrorStatus) *smv1.Progress {
	if s.CurrentRun == nil {
		return nil
	}
	return s.CurrentRun.Progress
}

func stripTimes(s *smv1.StorageMirrorStatus) *smv1.StorageMirrorStatus {
	for i := range s.Conditions {
		s.Conditions[i].LastTransitionTime = metav1.Time{}
	}
	return s
}

func setCondition(st *smv1.StorageMirrorStatus, t string, status metav1.ConditionStatus, reason, msg string, gen int64) {
	meta.SetStatusCondition(&st.Conditions, metav1.Condition{Type: t, Status: status, Reason: reason, Message: truncate(msg, 512), ObservedGeneration: gen})
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
