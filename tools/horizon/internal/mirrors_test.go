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

package horizon

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestFormatProgress(t *testing.T) {
	st := MirrorStatus{Mirror: "aosp-16", Phase: "Syncing", CurrentRun: &MirrorRun{WorkflowName: "storage-mirror-sync-x7k2p", Progress: &MirrorProgress{
		Phase: "fetch", State: "running", Attempt: 1, MaxAttempts: 3, ProjectsDone: 812, ProjectsTotal: 1204,
		OverallPercent: 55, ElapsedSeconds: 4210, EtaSeconds: 2030, CurrentProject: "platform/frameworks/base",
	}}}
	got := FormatProgress(st)
	for _, want := range []string{"aosp-16  Syncing", "attempt 1/3", "812/1204 projects", "[###########.........] 55%", "01:10:10 elapsed", "~00:33:50 left", "current: platform/frameworks/base", "horizon workflow logs storage-mirror-sync-x7k2p"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in %q", want, got)
		}
	}
	if got := FormatProgress(MirrorStatus{Mirror: "m", Phase: "Pending", PendingRequestID: "r1"}); !strings.Contains(got, "requested (r1)") {
		t.Fatal(got)
	}
}

func TestMirrorStatusWatchUntilDone(t *testing.T) {
	var n atomic.Int32
	c, _ := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/storage-mirror/mirrors/m/status" || r.Header.Get("X-Horizon-Submitted-From") != "horizon-cli" {
			t.Errorf("request %s %v", r.URL.Path, r.Header)
		}
		if n.Add(1) < 3 {
			_, _ = w.Write([]byte(`{"mirror":"m","phase":"Syncing","currentRun":{"workflowName":"w","progress":{"phase":"fetch","overallPercent":40}}}`))
			return
		}
		_, _ = w.Write([]byte(`{"mirror":"m","phase":"Error","lastRun":{"workflowName":"w","phase":"Failed","message":"fetch failed"}}`))
	}))
	var out bytes.Buffer
	err := CmdMirrorStatus(context.Background(), c, "m", true, time.Millisecond, &out)
	if err == nil || !strings.Contains(err.Error(), "fetch failed") || n.Load() != 3 {
		t.Fatalf("err=%v polls=%d\n%s", err, n.Load(), out.String())
	}
	if strings.Count(out.String(), "40%") != 2 {
		t.Fatalf("output:\n%s", out.String())
	}
}

func TestMirrorList(t *testing.T) {
	c, _ := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RawQuery != "kind=git" {
			t.Errorf("query %q", r.URL.RawQuery)
		}
		_, _ = w.Write([]byte(`{"items":[{"name":"k","directory":"k","kind":"git","managed":true,"status":{"phase":"Ready","lastSuccessfulSyncTime":"2026-11-25T03:00:00Z"}},{"name":"old","directory":"old","kind":"repo","managed":false,"catalog":{"status":"ready"}}]}`))
	}))
	var out bytes.Buffer
	if err := CmdMirrorList(context.Background(), c, "git", "", &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "Ready") || !strings.Contains(out.String(), "unmanaged (ready)") {
		t.Fatalf("output:\n%s", out.String())
	}
}
