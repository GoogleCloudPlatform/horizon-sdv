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
	"time"

	"github.com/robfig/cron/v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	smv1 "storage-mirror-module/api/v1alpha1"
	"storage-mirror-module/internal/dispatch"
)

// Scheduler turns cron schedules into sync requests (annotation sync-requested-at). It runs on the leader only.
type Scheduler struct {
	Client    client.Client
	Namespace string
	Interval  time.Duration
	// MissedWindow is how far back a tick missed during downtime still fires once.
	MissedWindow time.Duration
	Now          func() time.Time
	last         time.Time
}

var cronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)

// NeedLeaderElection keeps the scheduler on the leader.
func (s *Scheduler) NeedLeaderElection() bool { return true }

// Start runs until ctx ends.
func (s *Scheduler) Start(ctx context.Context) error {
	interval := s.Interval
	if interval == 0 {
		interval = time.Minute
	}
	window := s.MissedWindow
	if window == 0 {
		window = time.Hour
	}
	s.last = s.now().Add(-window)
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		if err := s.Tick(ctx); err != nil {
			log.FromContext(ctx).Error(err, "scheduler tick")
		}
		select {
		case <-ctx.Done():
			return nil
		case <-t.C:
		}
	}
}

func (s *Scheduler) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

// Tick requests syncs for every schedule that fired since the previous tick.
func (s *Scheduler) Tick(ctx context.Context) error {
	now := s.now()
	from := s.last
	if from.IsZero() {
		from = now.Add(-time.Minute)
	}
	s.last = now
	var cfg smv1.StorageMirrorConfig
	if err := s.Client.Get(ctx, client.ObjectKey{Namespace: s.Namespace, Name: smv1.ConfigName}, &cfg); err != nil {
		return client.IgnoreNotFound(err)
	}
	var list smv1.StorageMirrorList
	if err := s.Client.List(ctx, &list, client.InNamespace(s.Namespace)); err != nil {
		return err
	}
	for i := range list.Items {
		m := &list.Items[i]
		if m.Spec.Suspend || !m.DeletionTimestamp.IsZero() {
			continue
		}
		due, err := DueAt(m, cfg.Spec.Schedules, from, now)
		if err != nil {
			log.FromContext(ctx).Error(err, "schedule", "mirror", m.Name)
			continue
		}
		if due.IsZero() || startedSince(m, due) {
			continue
		}
		if err := RequestSync(ctx, s.Client, m, dispatch.SchedulerUser, dispatch.SubmittedFromAPI, "", now); err != nil {
			log.FromContext(ctx).Error(err, "request scheduled sync", "mirror", m.Name)
		}
	}
	return nil
}

// DueAt returns the latest schedule tick in (from, now] that applies to the mirror, or zero.
func DueAt(m *smv1.StorageMirror, schedules []smv1.ScheduleSpec, from, now time.Time) (time.Time, error) {
	var latest time.Time
	check := func(expr, tz string) error {
		loc := time.UTC
		if tz != "" {
			l, err := time.LoadLocation(tz)
			if err != nil {
				return fmt.Errorf("time zone %q: %w", tz, err)
			}
			loc = l
		}
		sched, err := cronParser.Parse(expr)
		if err != nil {
			return fmt.Errorf("cron %q: %w", expr, err)
		}
		for t := sched.Next(from.In(loc)); !t.After(now); t = sched.Next(t) {
			if t.After(latest) {
				latest = t
			}
		}
		return nil
	}
	switch m.Spec.Schedule {
	case smv1.ScheduleNone:
		return time.Time{}, nil
	case "", smv1.ScheduleInherit:
		for _, s := range schedules {
			if s.Selector != nil {
				sel, err := metav1.LabelSelectorAsSelector(s.Selector)
				if err != nil {
					return time.Time{}, err
				}
				if !sel.Matches(labels.Set(m.Labels)) {
					continue
				}
			}
			if err := check(s.Cron, s.TimeZone); err != nil {
				return time.Time{}, err
			}
		}
	default:
		if err := check(m.Spec.Schedule, "UTC"); err != nil {
			return time.Time{}, err
		}
	}
	return latest, nil
}

func startedSince(m *smv1.StorageMirror, t time.Time) bool {
	if m.Status.CurrentRun != nil || m.Status.PendingRequestID != "" {
		return true
	}
	if lr := m.Status.LastRun; lr != nil && lr.StartedAt != nil && !lr.StartedAt.Time.Before(t) {
		return true
	}
	return false
}

// RequestSync records a sync request on the CR; the MirrorReconciler dispatches it.
func RequestSync(ctx context.Context, c client.Client, m *smv1.StorageMirror, by, from, requestID string, now time.Time) error {
	patch := client.MergeFrom(m.DeepCopy())
	if m.Annotations == nil {
		m.Annotations = map[string]string{}
	}
	if requestID == "" {
		requestID = dispatch.NewRequestID()
	}
	m.Annotations[smv1.AnnotationSyncRequestedAt] = now.UTC().Format(time.RFC3339Nano)
	m.Annotations[AnnotationSyncRequestID] = requestID
	m.Annotations[AnnotationSyncRequestedBy] = by
	m.Annotations[AnnotationSubmittedFrom] = from
	return c.Patch(ctx, m, patch)
}
