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
	"time"

	"google.golang.org/grpc/codes"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"cloud-workstations-module/internal/wsapi"
)

const (
	ConditionKCCReady        = "KCCReady"
	ConditionPostureApplied  = "PostureApplied"
	ConditionIAMPolicySynced = "IAMPolicySynced"
	ConditionReady           = "Ready"

	apiWaitInterval  = 10 * time.Second
	apiErrorInterval = 30 * time.Second
	defaultAPIResync = 5 * time.Minute
)

func setCondition(conditions *[]metav1.Condition, previous []metav1.Condition, generation int64, conditionType string, status metav1.ConditionStatus, reason, message string) {
	condition := metav1.Condition{
		Type:               conditionType,
		Status:             status,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: generation,
	}
	if prev := meta.FindStatusCondition(previous, conditionType); prev != nil && prev.Status == status && prev.Reason == reason && prev.Message == message && !prev.LastTransitionTime.IsZero() {
		condition.LastTransitionTime = prev.LastTransitionTime
	}
	meta.SetStatusCondition(conditions, condition)
}

func markWaiting(generation int64, conditions *[]metav1.Condition, previous []metav1.Condition, reason, message string, types ...string) {
	for _, conditionType := range types {
		setCondition(conditions, previous, generation, conditionType, metav1.ConditionFalse, reason, message)
	}
	setCondition(conditions, previous, generation, ConditionReady, metav1.ConditionFalse, reason, message)
}

func metaConditionTrue(conditions []metav1.Condition, conditionType string) bool {
	return meta.IsStatusConditionTrue(conditions, conditionType)
}

func aggregateReady(generation int64, conditions *[]metav1.Condition, previous []metav1.Condition, required ...string) {
	ready := true
	for _, conditionType := range required {
		if !metaConditionTrue(*conditions, conditionType) {
			ready = false
			break
		}
	}
	status := metav1.ConditionFalse
	reason := "NotReady"
	message := "API-owned fields are not in sync"
	if ready {
		status = metav1.ConditionTrue
		reason = "Ready"
		message = "KCC and API-owned fields are in sync"
	}
	setCondition(conditions, previous, generation, ConditionReady, status, reason, message)
}

func applyAPIError(generation int64, conditions *[]metav1.Condition, previous []metav1.Condition, err error, types ...string) (time.Duration, bool) {
	reason := "APIError"
	wait := apiErrorInterval
	failed := false
	switch wsapi.Code(err) {
	case codes.Aborted:
		reason = "Aborted"
		wait = apiWaitInterval
	case codes.PermissionDenied:
		reason = "PermissionDenied"
	case codes.NotFound:
		reason = "NotFound"
		wait = apiWaitInterval
	case codes.FailedPrecondition:
		reason = "FailedPrecondition"
		wait = apiWaitInterval
	case codes.InvalidArgument:
		reason = "InvalidArgument"
		failed = true
	}
	message := err.Error()
	for _, conditionType := range types {
		setCondition(conditions, previous, generation, conditionType, metav1.ConditionFalse, reason, message)
	}
	setCondition(conditions, previous, generation, ConditionReady, metav1.ConditionFalse, reason, message)
	return wait, failed
}
