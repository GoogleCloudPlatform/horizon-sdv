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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	smv1 "storage-mirror-module/api/v1alpha1"
	"storage-mirror-module/internal/dispatch"
	"storage-mirror-module/internal/state"
)

// AnnotationImportedStatus is the catalog status of a mirror imported from metadata.yaml.
const AnnotationImportedStatus = "storagemirror.horizon-sdv.io/imported-status"

// ComputeStatus derives the mirror status from its Workflows, the live progress store and the previous status.
func ComputeStatus(m *smv1.StorageMirror, runs []Run, store *state.Store, ret smv1.RetentionSpec, now time.Time) smv1.StorageMirrorStatus {
	st := *m.Status.DeepCopy()
	limit := int(ret.SyncHistoryLimit)
	if limit <= 0 {
		limit = 10
	}

	var current *Run
	for i := range runs {
		run := runs[i]
		if run.RequestID != "" && run.RequestID == st.PendingRequestID {
			st.PendingRequestID = ""
			st.PendingRequestAt = nil
		}
		if run.Active() {
			current = &runs[i]
			continue
		}
		rs := runStatus(run)
		st.History = upsertHistory(st.History, rs, limit)
		if st.LastRun == nil || !rs.FinishedAt.Before(st.LastRun.FinishedAt) {
			cp := rs
			st.LastRun = &cp
		}
		if run.Succeeded() && (run.Template == dispatch.TemplateCreate || run.Template == dispatch.TemplateSync) {
			if st.LastSuccessfulSyncTime == nil || rs.FinishedAt.After(st.LastSuccessfulSyncTime.Time) {
				st.LastSuccessfulSyncTime = rs.FinishedAt
			}
		}
		if store != nil {
			if rep, ok := store.Get(run.Name); ok && rep.LFSDetected {
				st.LFSDetected = true
			}
			store.Forget(run.Name)
		}
	}
	if st.PendingRequestAt != nil && now.Sub(st.PendingRequestAt.Time) > pendingRequestTTL {
		st.PendingRequestID = ""
		st.PendingRequestAt = nil
	}

	st.CurrentRun = nil
	if current != nil {
		rs := runStatus(*current)
		rs.Progress = liveProgress(*current, store, now)
		if store != nil {
			if rep, ok := store.Get(current.Name); ok && rep.LFSDetected {
				st.LFSDetected = true
			}
		}
		if m.Status.CurrentRun != nil && m.Status.CurrentRun.WorkflowName == rs.WorkflowName && rs.Progress == nil {
			rs.Progress = m.Status.CurrentRun.Progress
		}
		st.CurrentRun = &rs
	}

	st.Phase, st.CatalogStatus = phaseFor(m, current, st.LastRun)
	if !m.DeletionTimestamp.IsZero() {
		st.Phase = smv1.PhaseDeleting
	}
	stale := metav1.ConditionFalse
	reason := "Fresh"
	days := ret.StaleAfterDays
	if days <= 0 {
		days = 14
	}
	if st.LastSuccessfulSyncTime != nil && now.Sub(st.LastSuccessfulSyncTime.Time) > time.Duration(days)*24*time.Hour {
		stale, reason = metav1.ConditionTrue, "NotSyncedRecently"
	}
	setCondition(&st, "Stale", stale, reason, "", m.Generation)
	ready := metav1.ConditionFalse
	if st.Phase == smv1.PhaseReady || (st.Phase == smv1.PhaseSyncing && st.LastSuccessfulSyncTime != nil) {
		ready = metav1.ConditionTrue
	}
	setCondition(&st, "Ready", ready, st.Phase, "", m.Generation)
	return st
}

func phaseFor(m *smv1.StorageMirror, current *Run, last *smv1.RunStatus) (string, string) {
	if current != nil {
		switch current.Template {
		case dispatch.TemplateCreate:
			return smv1.PhaseInitializing, "syncing"
		case dispatch.TemplateDelete:
			return smv1.PhaseDeleting, m.Status.CatalogStatus
		default:
			return smv1.PhaseSyncing, "syncing"
		}
	}
	if last != nil {
		if last.Phase == "Succeeded" {
			return smv1.PhaseReady, "ready"
		}
		return smv1.PhaseError, "error"
	}
	if s, ok := m.Annotations[AnnotationImportedStatus]; ok {
		switch s {
		case "ready":
			return smv1.PhaseReady, s
		case "uninitialized":
			return smv1.PhasePending, s
		default:
			return smv1.PhaseError, s
		}
	}
	if m.Status.Phase != "" {
		return m.Status.Phase, m.Status.CatalogStatus
	}
	return smv1.PhasePending, "uninitialized"
}

func runStatus(run Run) smv1.RunStatus {
	rs := smv1.RunStatus{
		WorkflowName: run.Name,
		Template:     run.Template,
		RequestID:    run.RequestID,
		SubmittedBy:  run.SubmittedBy,
		Phase:        run.Phase,
		StartedAt:    run.StartedAt,
		FinishedAt:   run.FinishedAt,
		Message:      truncate(run.Message, 512),
	}
	if rs.Phase == "" {
		rs.Phase = "Pending"
	}
	if rs.FinishedAt == nil && !run.Active() {
		t := metav1.NewTime(run.Created)
		rs.FinishedAt = &t
	}
	return rs
}

func liveProgress(run Run, store *state.Store, now time.Time) *smv1.Progress {
	if store != nil {
		if rep, ok := store.Get(run.Name); ok {
			p := rep.Progress
			t := metav1.NewTime(rep.ReceivedAt)
			p.UpdatedAt = &t
			p.Stale = now.Sub(rep.ReceivedAt) > progressStaleAge
			return &p
		}
	}
	if n, total, ok := ParseArgoProgress(run.Progress); ok {
		return &smv1.Progress{
			State:          "running",
			ProjectsDone:   n,
			ProjectsTotal:  total,
			OverallPercent: int32(n * 100 / total),
			Stale:          true,
			Message:        "from Argo node progress; agent report not received",
		}
	}
	return nil
}

func upsertHistory(h []smv1.RunStatus, rs smv1.RunStatus, limit int) []smv1.RunStatus {
	for i := range h {
		if h[i].WorkflowName == rs.WorkflowName {
			h[i] = rs
			return h
		}
	}
	h = append([]smv1.RunStatus{rs}, h...)
	if len(h) > limit {
		h = h[:limit]
	}
	return h
}
