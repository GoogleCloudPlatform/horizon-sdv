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

package kexec

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"k8s.io/client-go/rest"
)

// A server that accepts the connection but never completes the WebSocket upgrade must not hold Run
// beyond its timeout.
func TestRunDialHonoursTimeout(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()
	defer close(release)

	e := &Executor{Config: &rest.Config{Host: srv.URL}, Timeout: 300 * time.Millisecond}
	start := time.Now()
	_, err := e.Run(context.Background(), "ns", "pod", "c", []string{"true"})
	if err == nil {
		t.Fatal("a stalled upgrade must fail")
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("Run took %v, want it to stop near the 300ms timeout", elapsed)
	}
}

func TestRunDialHonoursCancel(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()
	defer close(release)

	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(200*time.Millisecond, cancel)
	e := &Executor{Config: &rest.Config{Host: srv.URL}, Timeout: time.Minute}
	start := time.Now()
	if _, err := e.Run(ctx, "ns", "pod", "c", []string{"true"}); err == nil {
		t.Fatal("a cancelled context must fail the exec")
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("Run took %v after cancel, want it to return promptly", elapsed)
	}
}

func TestExecURL(t *testing.T) {
	got, err := execURL("https://10.0.0.1:443", "ns", "pod-1", "authenticator", []string{"node", "createServiceAccount.js", "mtk-connect-admin"})
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(got)
	if err != nil {
		t.Fatal(err)
	}
	if u.Scheme != "wss" || u.Path != "/api/v1/namespaces/ns/pods/pod-1/exec" {
		t.Errorf("unexpected URL %s", got)
	}
	q := u.Query()
	if q.Get("container") != "authenticator" || q.Get("stdout") != "true" || q.Get("stderr") != "true" {
		t.Errorf("unexpected query %v", q)
	}
	cmd := q["command"]
	if len(cmd) != 3 || cmd[0] != "node" || cmd[2] != "mtk-connect-admin" {
		t.Errorf("command order lost: %v", cmd)
	}
	if _, err := execURL("ftp://x", "ns", "p", "c", nil); err == nil {
		t.Error("unsupported scheme must be rejected")
	}
}

func TestStatusError(t *testing.T) {
	if err := statusError(nil, ""); err != nil {
		t.Errorf("no status document must be success, got %v", err)
	}
	if err := statusError([]byte(`{"status":"Success"}`), ""); err != nil {
		t.Errorf("Success must be nil, got %v", err)
	}
	err := statusError([]byte(`{"status":"Failure","message":"command terminated with exit code 1","reason":"NonZeroExitCode"}`), "boom")
	if err == nil {
		t.Fatal("Failure must be an error")
	}
	if err := statusError([]byte(`not json`), ""); err == nil {
		t.Error("unreadable status must be an error")
	}
}
