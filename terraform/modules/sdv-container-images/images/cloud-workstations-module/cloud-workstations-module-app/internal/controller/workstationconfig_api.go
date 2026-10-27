// Copyright (c) 2026 Accenture, All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package controller

import (
	"context"
	"time"

	pb "cloud.google.com/go/workstations/apiv1/workstationspb"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"

	wsv1alpha1 "cloud-workstations-module/api/v1alpha1"
	"cloud-workstations-module/internal/wsapi"
)

func (r *WorkstationConfigReconciler) reconcileConfigAPI(ctx context.Context, cfg *wsv1alpha1.WorkstationConfig, desired *wsv1alpha1.WorkstationConfigStatus) ctrl.Result {
	logger := log.FromContext(ctx)
	previous := cfg.Status.Conditions
	if r.WS == nil {
		markWaiting(cfg.Generation, &desired.Conditions, previous, "WaitingForAPI", "workstations client is not configured", ConditionPostureApplied, ConditionIAMPolicySynced)
		desired.Phase = "Pending"
		return kccWait()
	}

	live, err := r.WS.GetConfig(ctx, desired.ExternalRef)
	if err != nil {
		logger.Error(err, "get workstation config")
		return r.failAPI(cfg.Generation, desired, previous, err, ConditionPostureApplied, ConditionIAMPolicySynced)
	}
	if live.GetReconciling() {
		setCondition(&desired.Conditions, previous, cfg.Generation, ConditionPostureApplied, metav1.ConditionFalse, "Reconciling", "workstation config is reconciling")
		if result, stop := r.syncConfigIAM(ctx, cfg, desired, false); stop {
			return result
		}
		desired.Phase = "Pending"
		return ctrl.Result{RequeueAfter: apiWaitInterval}
	}

	want := wsapi.DesiredPosture(cfg.Spec)
	if !wsapi.PostureMatches(live, want) {
		if err := r.WS.PatchConfigPosture(ctx, desired.ExternalRef, live.GetEtag(), want); err != nil {
			logger.Error(err, "patch workstation config posture")
			if result, stop := r.syncConfigIAM(ctx, cfg, desired, false); stop {
				return result
			}
			return r.failAPI(cfg.Generation, desired, previous, err, ConditionPostureApplied)
		}
		setCondition(&desired.Conditions, previous, cfg.Generation, ConditionPostureApplied, metav1.ConditionFalse, "Updating", "posture update accepted; waiting for read-back")
		if result, stop := r.syncConfigIAM(ctx, cfg, desired, false); stop {
			return result
		}
		desired.Posture = nil
		desired.Phase = "Pending"
		return ctrl.Result{RequeueAfter: apiWaitInterval}
	}

	setCondition(&desired.Conditions, previous, cfg.Generation, ConditionPostureApplied, metav1.ConditionTrue, "Applied", "posture matches the spec")
	desired.Posture = postureStatusFromLive(live)
	if result, stop := r.syncConfigIAM(ctx, cfg, desired, true); stop {
		return result
	}
	aggregateReady(cfg.Generation, &desired.Conditions, previous, ConditionKCCReady, ConditionPostureApplied, ConditionIAMPolicySynced)
	if metaConditionTrue(desired.Conditions, ConditionReady) {
		desired.Phase = "Ready"
		return ctrl.Result{RequeueAfter: r.resync()}
	}
	desired.Phase = "Pending"
	return ctrl.Result{RequeueAfter: apiWaitInterval}
}

func (r *WorkstationConfigReconciler) syncConfigIAM(ctx context.Context, cfg *wsv1alpha1.WorkstationConfig, desired *wsv1alpha1.WorkstationConfigStatus, allowGrant bool) (ctrl.Result, bool) {
	err := r.syncRole(ctx, desired.ExternalRef, wsapi.RoleConfigAdmin, cfg.Spec.Admins, allowGrant)
	if err != nil {
		log.FromContext(ctx).Error(err, "sync config IAM")
		return r.failAPI(cfg.Generation, desired, cfg.Status.Conditions, err, ConditionIAMPolicySynced), true
	}
	if !allowGrant {
		setCondition(&desired.Conditions, cfg.Status.Conditions, cfg.Generation, ConditionIAMPolicySynced, metav1.ConditionFalse, "WaitingForPosture", "admin grants wait until posture is applied")
		return ctrl.Result{}, false
	}
	setCondition(&desired.Conditions, cfg.Status.Conditions, cfg.Generation, ConditionIAMPolicySynced, metav1.ConditionTrue, "IAMSynced", "config admin binding matches spec.admins")
	return ctrl.Result{}, false
}

func (r *WorkstationConfigReconciler) syncRole(ctx context.Context, resource, role string, members []string, allowGrant bool) error {
	policy, err := r.WS.GetIamPolicy(ctx, resource)
	if err != nil {
		return err
	}
	next, changed := wsapi.ReconcileRoleBinding(policy, role, members, allowGrant)
	if !changed {
		return nil
	}
	_, err = r.WS.SetIamPolicy(ctx, resource, next)
	return err
}

func (r *WorkstationConfigReconciler) failAPI(generation int64, desired *wsv1alpha1.WorkstationConfigStatus, previous []metav1.Condition, err error, types ...string) ctrl.Result {
	wait, failed := applyAPIError(generation, &desired.Conditions, previous, err, types...)
	desired.Phase = "Pending"
	if failed {
		desired.Phase = "Failed"
	}
	return ctrl.Result{RequeueAfter: wait}
}

func (r *WorkstationConfigReconciler) resync() time.Duration {
	if r.ResyncInterval > 0 {
		return r.ResyncInterval
	}
	return defaultAPIResync
}

func postureStatusFromLive(live *pb.WorkstationConfig) *wsv1alpha1.WorkstationConfigPostureStatus {
	out := &wsv1alpha1.WorkstationConfigPostureStatus{
		DisableSsh: live.GetHost().GetGceInstance().GetDisableSsh(),
	}
	for _, port := range live.GetAllowedPorts() {
		out.AllowedPorts = append(out.AllowedPorts, wsv1alpha1.PortRange{First: port.GetFirst(), Last: port.GetLast()})
	}
	for _, dir := range live.GetEphemeralDirectories() {
		item := wsv1alpha1.EphemeralDirectory{MountPath: dir.GetMountPath()}
		if disk := dir.GetGcePd(); disk != nil {
			readOnly := disk.GetReadOnly()
			item.GcePD = &wsv1alpha1.EphemeralPersistentDisk{
				DiskType:       disk.GetDiskType(),
				SourceSnapshot: disk.GetSourceSnapshot(),
				SourceImage:    disk.GetSourceImage(),
				ReadOnly:       &readOnly,
			}
		}
		out.EphemeralDirectories = append(out.EphemeralDirectories, item)
	}
	return out
}
