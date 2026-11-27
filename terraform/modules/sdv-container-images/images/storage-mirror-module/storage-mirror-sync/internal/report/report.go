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

// Package report publishes progress: controller REST API, Argo progress file, log lines and a disk snapshot.
// Every channel is best effort; a failing one never fails the run.
package report

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"storage-mirror-sync/internal/progress"
)

// Usage is the share usage from statfs.
type Usage struct {
	CapacityBytes  int64 `json:"capacityBytes"`
	UsedBytes      int64 `json:"usedBytes"`
	AvailableBytes int64 `json:"availableBytes"`
}

// StatFS returns the usage of the file system holding path.
func StatFS(path string) (Usage, error) {
	var s syscall.Statfs_t
	if err := syscall.Statfs(path, &s); err != nil {
		return Usage{}, err
	}
	bs := int64(s.Bsize)
	total := int64(s.Blocks) * bs
	avail := int64(s.Bavail) * bs
	return Usage{CapacityBytes: total, UsedBytes: total - int64(s.Bfree)*bs, AvailableBytes: avail}, nil
}

// Reporter sends progress for one run. Publish is safe for concurrent use (the reporting loop and the final report).
type Reporter struct {
	ControllerURL string
	Token         string
	RunID         string
	Mirror        string
	// ArgoProgressFile is $ARGO_PROGRESS_FILE ("N/M").
	ArgoProgressFile string
	// SnapshotFile is <root>/.storage-mirror/progress/<dir>.json.
	SnapshotFile string
	HTTP         *http.Client

	lfsDetected atomic.Bool
	mu          sync.Mutex
	lastLog     time.Time
	lastDisk    time.Time
}

// SetLFSDetected marks the run as having found git-lfs content.
func (r *Reporter) SetLFSDetected() { r.lfsDetected.Store(true) }

// LFSDetected reports whether SetLFSDetected was called.
func (r *Reporter) LFSDetected() bool { return r.lfsDetected.Load() }

// NewHTTPClient trusts the controller CA file when present.
func NewHTTPClient(caFile string) *http.Client {
	tr := &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}}
	if caFile != "" {
		if pem, err := os.ReadFile(caFile); err == nil {
			pool := x509.NewCertPool()
			if pool.AppendCertsFromPEM(pem) {
				tr.TLSClientConfig.RootCAs = pool
			}
		}
	}
	return &http.Client{Timeout: 10 * time.Second, Transport: tr}
}

// Publish sends one progress snapshot. final forces every channel.
func (r *Reporter) Publish(ctx context.Context, p progress.Progress, final bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now()
	if r.ArgoProgressFile != "" {
		_ = os.WriteFile(r.ArgoProgressFile, []byte(p.ArgoProgress()), 0o644)
	}
	body, _ := json.Marshal(p)
	if final || now.Sub(r.lastLog) >= 30*time.Second {
		fmt.Printf("STORAGE_MIRROR_PROGRESS %s\n", body)
		r.lastLog = now
	}
	if r.SnapshotFile != "" && (final || now.Sub(r.lastDisk) >= 30*time.Second) {
		_ = os.MkdirAll(filepath.Dir(r.SnapshotFile), 0o2775)
		snap, _ := json.Marshal(map[string]interface{}{"runId": r.RunID, "mirror": r.Mirror, "progress": p, "updatedAt": now.UTC().Format(time.RFC3339)})
		_ = os.WriteFile(r.SnapshotFile, snap, 0o664)
		r.lastDisk = now
	}
	if r.ControllerURL == "" || r.Token == "" || r.RunID == "" {
		return
	}
	payload, _ := json.Marshal(map[string]interface{}{"mirror": r.Mirror, "progress": p, "lfsDetected": r.LFSDetected()})
	if err := r.send(ctx, http.MethodPut, "/internal/v1/runs/"+r.RunID+"/progress", payload); err != nil && final {
		log.Printf("progress report: %v", err)
	}
}

// Catalog sends the catalog snapshot and the share usage.
func (r *Reporter) Catalog(ctx context.Context, metadata []byte, usage Usage) {
	if r.ControllerURL == "" || r.Token == "" {
		return
	}
	payload, _ := json.Marshal(map[string]interface{}{"metadataYaml": string(metadata), "usage": usage})
	if err := r.send(ctx, http.MethodPost, "/internal/v1/catalog", payload); err != nil {
		log.Printf("catalog report: %v", err)
	}
}

func (r *Reporter) send(ctx context.Context, method, path string, payload []byte) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, r.ControllerURL+path, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+r.Token)
	client := r.HTTP
	if client == nil {
		client = NewHTTPClient("")
	}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return fmt.Errorf("%s %s: HTTP %d", method, path, res.StatusCode)
	}
	return nil
}

// Loop publishes the tracker every interval until ctx ends.
func (r *Reporter) Loop(ctx context.Context, t *progress.Tracker, interval time.Duration) {
	tick := time.NewTicker(interval)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			r.Publish(ctx, t.Snapshot(), false)
		}
	}
}
