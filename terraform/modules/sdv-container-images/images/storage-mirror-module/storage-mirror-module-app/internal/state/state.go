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

// Package state holds the in-memory live progress reported by mirror-agent.
package state

import (
	"sync"
	"time"

	smv1 "storage-mirror-module/api/v1alpha1"
)

// Report is the latest agent report for one run.
type Report struct {
	Mirror      string
	Progress    smv1.Progress
	LFSDetected bool
	ReceivedAt  time.Time
}

// Store keeps the latest report per run (Workflow name).
type Store struct {
	mu   sync.RWMutex
	runs map[string]Report
}

// NewStore returns an empty store.
func NewStore() *Store {
	return &Store{runs: map[string]Report{}}
}

// Put records a report.
func (s *Store) Put(run string, r Report) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runs[run] = r
}

// Get returns the latest report of a run.
func (s *Store) Get(run string) (Report, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.runs[run]
	return r, ok
}

// Forget drops reports of runs that are no longer active.
func (s *Store) Forget(run string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.runs, run)
}
