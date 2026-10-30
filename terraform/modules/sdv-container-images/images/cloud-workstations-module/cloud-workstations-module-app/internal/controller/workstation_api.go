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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"

	wsv1alpha1 "cloud-workstations-module/api/v1alpha1"
	"cloud-workstations-module/internal/wsapi"
)

func (r *WorkstationReconciler) reconcileWorkstationAPI(ctx context.Context, ws *wsv1alpha1.Workstation, parent *wsv1alpha1.WorkstationConfig, desired *wsv1alpha1.WorkstationStatus) ctrl.Result {
	logger := log.FromContext(ctx)
	previous := ws.Status.Conditions
	if r.WS == nil {
		markWaiting(ws.Generation, &desired.Conditions, previous, "WaitingForAPI", "workstations client is not configured", ConditionIAMPolicySynced)
		desired.Phase = "Pending"
		return kccWait()
	}

	allowGrant := metaConditionTrue(parent.Status.Conditions, ConditionReady)
	err := r.syncWorkstationRole(ctx, desired.ExternalRef, ws.Spec.Users, allowGrant)
	if err != nil {
		logger.Error(err, "sync workstation IAM")
		wait, failed := applyAPIError(ws.Generation, &desired.Conditions, previous, err, ConditionIAMPolicySynced)
		desired.Phase = "Pending"
		if failed {
			desired.Phase = "Failed"
		}
		return ctrl.Result{RequeueAfter: wait}
	}
	if !allowGrant {
		setCondition(&desired.Conditions, previous, ws.Generation, ConditionIAMPolicySynced, metav1.ConditionFalse, "WaitingForConfig", "user grants wait until the parent config is Ready")
		desired.Phase = "Pending"
		return ctrl.Result{RequeueAfter: apiWaitInterval}
	}
	setCondition(&desired.Conditions, previous, ws.Generation, ConditionIAMPolicySynced, metav1.ConditionTrue, "IAMSynced", "workstation user binding matches spec.users")
	aggregateReady(ws.Generation, &desired.Conditions, previous, ConditionKCCReady, ConditionIAMPolicySynced)
	if metaConditionTrue(desired.Conditions, ConditionReady) {
		desired.Phase = "Ready"
		if r.ResyncInterval > 0 {
			return ctrl.Result{RequeueAfter: r.ResyncInterval}
		}
		return ctrl.Result{RequeueAfter: defaultAPIResync}
	}
	desired.Phase = "Pending"
	return ctrl.Result{RequeueAfter: apiWaitInterval}
}

func (r *WorkstationReconciler) syncWorkstationRole(ctx context.Context, resource string, members []string, allowGrant bool) error {
	policy, err := r.WS.GetIamPolicy(ctx, resource)
	if err != nil {
		return err
	}
	next, changed := wsapi.ReconcileRoleBinding(policy, wsapi.RoleWorkstationUser, members, allowGrant)
	if !changed {
		return nil
	}
	_, err = r.WS.SetIamPolicy(ctx, resource, next)
	return err
}
