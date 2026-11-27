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

// Package progress parses repo, git and git-lfs progress output into structured progress.
// The JSON shape matches storagemirror.horizon-sdv.io/v1alpha1 Progress in the controller.
package progress

import (
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Phases of a run.
const (
	PhaseInit      = "init"
	PhaseFetch     = "fetch"
	PhasePruneRefs = "prune-refs"
	PhaseLFS       = "lfs"
	PhaseGC        = "gc"
	PhaseDelete    = "delete"
)

// States of a run.
const (
	StateRunning   = "running"
	StateRetrying  = "retrying"
	StateSucceeded = "succeeded"
	StateFailed    = "failed"
	StateCancelled = "cancelled"
)

// Progress is the structured progress of one run.
type Progress struct {
	Phase           string `json:"phase,omitempty"`
	State           string `json:"state,omitempty"`
	Attempt         int32  `json:"attempt,omitempty"`
	MaxAttempts     int32  `json:"maxAttempts,omitempty"`
	ProjectsDone    int64  `json:"projectsDone,omitempty"`
	ProjectsTotal   int64  `json:"projectsTotal,omitempty"`
	CurrentProject  string `json:"currentProject,omitempty"`
	RefsUpdated     int64  `json:"refsUpdated,omitempty"`
	ObjectsReceived int64  `json:"objectsReceived,omitempty"`
	ObjectsTotal    int64  `json:"objectsTotal,omitempty"`
	BytesReceived   int64  `json:"bytesReceived,omitempty"`
	BytesFreed      int64  `json:"bytesFreed,omitempty"`
	LFSObjectsDone  int64  `json:"lfsObjectsDone,omitempty"`
	LFSObjectsTotal int64  `json:"lfsObjectsTotal,omitempty"`
	PhasePercent    int32  `json:"phasePercent,omitempty"`
	OverallPercent  int32  `json:"overallPercent,omitempty"`
	ElapsedSeconds  int64  `json:"elapsedSeconds,omitempty"`
	EtaSeconds      int64  `json:"etaSeconds,omitempty"`
	Message         string `json:"message,omitempty"`
}

// phase weights in percent of the whole run, per kind. Fixed, so the overall bar never moves backwards.
var weights = map[string][]struct {
	phase  string
	weight int
}{
	"repo":   {{PhaseInit, 2}, {PhaseFetch, 78}, {PhasePruneRefs, 2}, {PhaseLFS, 3}, {PhaseGC, 15}},
	"git":    {{PhaseInit, 2}, {PhaseFetch, 83}, {PhasePruneRefs, 0}, {PhaseLFS, 5}, {PhaseGC, 10}},
	"delete": {{PhaseDelete, 100}},
}

// Overall converts a phase percentage into the overall percentage for the kind.
func Overall(kind, phase string, phasePercent int32) int32 {
	done := 0
	for _, w := range weights[kind] {
		if w.phase == phase {
			return int32(done) + int32(w.weight)*clamp(phasePercent)/100
		}
		done += w.weight
	}
	return 0
}

func clamp(p int32) int32 {
	if p < 0 {
		return 0
	}
	if p > 100 {
		return 100
	}
	return p
}

// Tracker holds the live progress of a run; safe for concurrent use.
type Tracker struct {
	mu      sync.Mutex
	kind    string
	p       Progress
	started time.Time
	phaseAt time.Time
	now     func() time.Time
}

// NewTracker starts tracking a run of the given kind (repo, git or delete).
func NewTracker(kind string, maxAttempts int32, now func() time.Time) *Tracker {
	if now == nil {
		now = time.Now
	}
	t := now()
	return &Tracker{kind: kind, now: now, started: t, phaseAt: t, p: Progress{State: StateRunning, MaxAttempts: maxAttempts}}
}

// Snapshot returns a copy with elapsed time and an estimate.
func (t *Tracker) Snapshot() Progress {
	t.mu.Lock()
	defer t.mu.Unlock()
	p := t.p
	now := t.now()
	p.ElapsedSeconds = int64(now.Sub(t.started).Seconds())
	if p.PhasePercent > 0 && p.PhasePercent < 100 {
		spent := now.Sub(t.phaseAt).Seconds()
		p.EtaSeconds = int64(spent * float64(100-p.PhasePercent) / float64(p.PhasePercent))
	}
	return p
}

// Update applies fn under the lock and recomputes the overall percentage.
func (t *Tracker) Update(fn func(*Progress)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	before := t.p.Phase
	fn(&t.p)
	if t.p.Phase != before {
		t.phaseAt = t.now()
	}
	t.p.PhasePercent = clamp(t.p.PhasePercent)
	// Monotonic: a retry restarts the phase, the overall bar stays where it was.
	if o := Overall(t.kind, t.p.Phase, t.p.PhasePercent); o > t.p.OverallPercent {
		t.p.OverallPercent = o
	}
	if t.p.State == StateSucceeded {
		t.p.OverallPercent = 100
	}
}

// SetPhase starts a phase.
func (t *Tracker) SetPhase(phase string) {
	t.Update(func(p *Progress) {
		p.Phase = phase
		p.PhasePercent = 0
		p.CurrentProject = ""
		if p.State == StateRetrying {
			p.State = StateRunning
		}
	})
}

var (
	repoFetchRe = regexp.MustCompile(`(?i)(?:fetching(?: projects)?|syncing):\s+(\d+)%\s+\((\d+)/(\d+)\)(.*)`)
	gitStageRe  = regexp.MustCompile(`(Counting objects|Compressing objects|Receiving objects|Resolving deltas):\s+(\d+)%\s+\((\d+)/(\d+)\)(?:,\s+([\d.]+)\s+(bytes|KiB|MiB|GiB))?`)
	gitRefRe    = regexp.MustCompile(`^\s*(?:[+*=!-]|\s)?\s*(?:\[new (?:branch|tag|ref)\]|[0-9a-f]{7,}\.\.\.?[0-9a-f]{7,}|\[deleted\])\s+\S+\s+->\s+\S+`)
	lfsRe       = regexp.MustCompile(`Downloading LFS objects:\s+(\d+)%\s+\((\d+)/(\d+)\)(?:,\s+([\d.]+)\s+(B|KB|MB|GB))?`)
	repoProjRe  = regexp.MustCompile(`^\s*(?:\d+:\d+(?::\d+)?\s+)?(\S+)(?:\s+@\s+\S+)?\s*$`)
)

// ParseLine applies one line of tool output; it returns true when the line changed progress.
func (t *Tracker) ParseLine(line string) bool {
	line = strings.TrimSpace(stripANSI(line))
	if line == "" {
		return false
	}
	if m := repoFetchRe.FindStringSubmatch(line); m != nil {
		pct, done, total := atoi32(m[1]), atoi(m[2]), atoi(m[3])
		project := ""
		if parts := strings.Split(m[4], "|"); len(parts) > 1 {
			if pm := repoProjRe.FindStringSubmatch(parts[len(parts)-1]); pm != nil && !strings.HasSuffix(pm[1], "jobs") && !repoPlaceholder(pm[1]) {
				project = pm[1]
			}
		}
		t.Update(func(p *Progress) {
			p.Phase = PhaseFetch
			p.PhasePercent, p.ProjectsDone, p.ProjectsTotal = pct, done, total
			if project != "" {
				p.CurrentProject = project
			}
		})
		return true
	}
	if m := lfsRe.FindStringSubmatch(line); m != nil {
		pct, done, total := atoi32(m[1]), atoi(m[2]), atoi(m[3])
		t.Update(func(p *Progress) {
			p.Phase = PhaseLFS
			p.LFSObjectsDone, p.LFSObjectsTotal = done, total
			if p.ProjectsTotal <= 1 {
				p.PhasePercent = pct
			}
		})
		return true
	}
	if m := gitStageRe.FindStringSubmatch(line); m != nil {
		stage, pct, done, total := m[1], atoi32(m[2]), atoi(m[3]), atoi(m[4])
		bytes := toBytes(m[5], m[6])
		t.Update(func(p *Progress) {
			if t.kind != "git" {
				return
			}
			p.Phase = PhaseFetch
			switch stage {
			case "Receiving objects":
				p.ObjectsReceived, p.ObjectsTotal = done, total
				if bytes > 0 {
					p.BytesReceived = bytes
				}
				p.PhasePercent = pct * 90 / 100
			case "Resolving deltas":
				p.PhasePercent = 90 + pct*10/100
			}
			p.Message = stage
		})
		return true
	}
	if t.kind == "git" && gitRefRe.MatchString(line) {
		t.Update(func(p *Progress) { p.RefsUpdated++ })
		return true
	}
	return false
}

// SetCounts sets projects done and total for phases the agent iterates itself (prune-refs, lfs, gc).
func (t *Tracker) SetCounts(done, total int64, current string) {
	t.Update(func(p *Progress) {
		p.ProjectsDone, p.ProjectsTotal, p.CurrentProject = done, total, current
		if total > 0 {
			p.PhasePercent = int32(done * 100 / total)
		}
	})
}

// ArgoProgress renders Argo's "N/M" node progress.
func (p Progress) ArgoProgress() string {
	return strconv.Itoa(int(p.OverallPercent)) + "/100"
}

func atoi(s string) int64 {
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}

func atoi32(s string) int32 { return int32(atoi(s)) }

func toBytes(num, unit string) int64 {
	f, err := strconv.ParseFloat(num, 64)
	if err != nil {
		return 0
	}
	switch unit {
	case "KiB", "KB":
		f *= 1 << 10
	case "MiB", "MB":
		f *= 1 << 20
	case "GiB", "GB":
		f *= 1 << 30
	}
	return int64(f)
}

// repoPlaceholder matches the interleaved-sync status words repo prints instead of a project name.
func repoPlaceholder(s string) bool { return s == "None" || strings.HasPrefix(s, "..") }

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;?]*[A-Za-z]`)

func stripANSI(s string) string { return ansiRe.ReplaceAllString(s, "") }
