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

package progress

import (
	"testing"
	"time"
)

func TestRepoFetchLines(t *testing.T) {
	tr := NewTracker("repo", 3, nil)
	// repo 2.4x progress line (pty), and the older "Fetching projects" form.
	if !tr.ParseLine("\x1b[KFetching:  3% (35/1100) 0:31 | 4 jobs | 0:12 platform/art @ art") {
		t.Fatal("not parsed")
	}
	p := tr.Snapshot()
	if p.Phase != PhaseFetch || p.ProjectsDone != 35 || p.ProjectsTotal != 1100 || p.PhasePercent != 3 || p.CurrentProject != "platform/art" {
		t.Fatalf("%+v", p)
	}
	tr.ParseLine("Fetching projects: 50% (550/1100)")
	if p := tr.Snapshot(); p.ProjectsDone != 550 || p.OverallPercent != 2+78*50/100 {
		t.Fatalf("%+v", p)
	}
	if tr.ParseLine("Fetching: 40% (440/1100) 1:00 | 4 jobs | platform/build"); tr.Snapshot().OverallPercent < 41 {
		t.Fatal("overall percentage must not move backwards")
	}
}

func TestRepoInterleavedSyncLines(t *testing.T) {
	// repo 2.5x syncs interleaved and prints "Syncing:" with a status word instead of a project.
	tr := NewTracker("repo", 3, nil)
	if !tr.ParseLine("\x1b[KSyncing: 60% (3/5) 0:00 | ..working..") {
		t.Fatal("not parsed")
	}
	p := tr.Snapshot()
	if p.Phase != PhaseFetch || p.ProjectsDone != 3 || p.ProjectsTotal != 5 || p.CurrentProject != "" {
		t.Fatalf("%+v", p)
	}
	tr.ParseLine("Syncing:  0% (0/5) 0:00 | None")
	if p := tr.Snapshot(); p.CurrentProject != "" {
		t.Fatalf("placeholder taken as project: %+v", p)
	}
}

func TestGitLines(t *testing.T) {
	tr := NewTracker("git", 3, nil)
	tr.ParseLine("Receiving objects:  50% (500/1000), 12.50 MiB | 3.00 MiB/s")
	p := tr.Snapshot()
	if p.ObjectsReceived != 500 || p.ObjectsTotal != 1000 || p.BytesReceived != 12.5*(1<<20) || p.PhasePercent != 45 {
		t.Fatalf("%+v", p)
	}
	tr.ParseLine("Resolving deltas: 100% (300/300), done.")
	if p := tr.Snapshot(); p.PhasePercent != 100 {
		t.Fatalf("%+v", p)
	}
	tr.ParseLine(" * [new branch]      main       -> main")
	tr.ParseLine("   1a2b3c4d..5e6f7a8b  release -> release")
	tr.ParseLine(" * [new tag]         v1.0       -> v1.0")
	if p := tr.Snapshot(); p.RefsUpdated != 3 {
		t.Fatalf("refs %d", p.RefsUpdated)
	}
	if tr.ParseLine("From https://example.com/repo") {
		t.Fatal("noise must not count")
	}
}

func TestLFSAndCounts(t *testing.T) {
	tr := NewTracker("git", 1, nil)
	tr.ParseLine("Downloading LFS objects:  25% (1/4), 2.0 MB | 1.0 MB/s")
	if p := tr.Snapshot(); p.Phase != PhaseLFS || p.LFSObjectsDone != 1 || p.LFSObjectsTotal != 4 {
		t.Fatalf("%+v", p)
	}
	tr.SetPhase(PhaseGC)
	tr.SetCounts(1, 2, "repo")
	if p := tr.Snapshot(); p.PhasePercent != 50 || p.OverallPercent != 2+83+5+5 {
		t.Fatalf("%+v", p)
	}
	tr.Update(func(p *Progress) { p.State = StateSucceeded })
	if p := tr.Snapshot(); p.OverallPercent != 100 || p.ArgoProgress() != "100/100" {
		t.Fatalf("%+v", p)
	}
}

func TestEta(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	tr := NewTracker("repo", 1, func() time.Time { return now })
	tr.SetPhase(PhaseFetch)
	now = now.Add(10 * time.Minute)
	tr.ParseLine("Fetching: 25% (25/100)")
	if p := tr.Snapshot(); p.EtaSeconds != 1800 || p.ElapsedSeconds != 600 {
		t.Fatalf("%+v", p)
	}
}
