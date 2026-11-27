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

package report

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"storage-mirror-sync/internal/progress"
)

func TestPublishAllChannels(t *testing.T) {
	var path, auth string
	var body map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, auth = r.URL.Path, r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	dir := t.TempDir()
	r := &Reporter{ControllerURL: srv.URL, Token: "tok", RunID: "run-1", Mirror: "m",
		ArgoProgressFile: filepath.Join(dir, "progress"), SnapshotFile: filepath.Join(dir, "snap", "m.json"), HTTP: srv.Client()}
	r.Publish(context.Background(), progress.Progress{Phase: "fetch", OverallPercent: 42}, true)
	if path != "/internal/v1/runs/run-1/progress" || auth != "Bearer tok" || body["mirror"] != "m" {
		t.Fatalf("request %s %s %v", path, auth, body)
	}
	if b, _ := os.ReadFile(r.ArgoProgressFile); string(b) != "42/100" {
		t.Fatalf("argo progress %q", b)
	}
	if _, err := os.Stat(r.SnapshotFile); err != nil {
		t.Fatal("snapshot missing")
	}
	r.Catalog(context.Background(), []byte("horizon-mirrors: {}\n"), Usage{CapacityBytes: 1})
	if path != "/internal/v1/catalog" || body["metadataYaml"] != "horizon-mirrors: {}\n" {
		t.Fatalf("catalog %s %v", path, body)
	}
}

// TestConcurrentPublish runs the reporting loop, a final Publish and SetLFSDetected at the same time (go test -race).
func TestConcurrentPublish(t *testing.T) {
	var last map[string]interface{}
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var b map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&b)
		mu.Lock()
		last = b
		mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	dir := t.TempDir()
	r := &Reporter{ControllerURL: srv.URL, Token: "tok", RunID: "run-1", Mirror: "m", HTTP: srv.Client(),
		ArgoProgressFile: filepath.Join(dir, "progress"), SnapshotFile: filepath.Join(dir, "m.json")}
	tr := progress.NewTracker("git", 3, nil)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { r.Loop(ctx, tr, time.Millisecond); close(done) }()
	for i := 0; i < 20; i++ {
		tr.SetCounts(int64(i), 20, "p")
		if i == 10 {
			r.SetLFSDetected()
		}
		r.Publish(ctx, tr.Snapshot(), i%5 == 0)
		time.Sleep(time.Millisecond)
	}
	r.Publish(ctx, tr.Snapshot(), true)
	cancel()
	<-done
	mu.Lock()
	defer mu.Unlock()
	if last["lfsDetected"] != true {
		t.Fatalf("last report %v", last)
	}
}

func TestStatFS(t *testing.T) {
	u, err := StatFS(t.TempDir())
	if err != nil || u.CapacityBytes <= 0 || u.AvailableBytes > u.CapacityBytes {
		t.Fatalf("%+v %v", u, err)
	}
}
