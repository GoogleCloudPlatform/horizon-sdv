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

// Package kexec runs a command in a pod container through the pods/exec subresource.
//
// It speaks the Kubernetes WebSocket channel protocol (v4.channel.k8s.io) with golang.org/x/net/websocket,
// which keeps the controller free of the SPDY dependency tree. Only stdout, stderr and the exit status
// are supported (no stdin, no TTY).
package kexec

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"time"

	"golang.org/x/net/websocket"
	"k8s.io/client-go/rest"
)

const subProtocol = "v4.channel.k8s.io"

const (
	channelStdout byte = 1
	channelStderr byte = 2
	channelError  byte = 3
)

// Executor runs commands in pods using the given REST config.
type Executor struct {
	Config  *rest.Config
	Timeout time.Duration
}

type execStatus struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Reason  string `json:"reason"`
}

// Run executes command in the container and returns its stdout. A non-zero exit status is an error.
func (e *Executor) Run(ctx context.Context, namespace, pod, container string, command []string) (string, error) {
	tlsCfg, err := rest.TLSConfigFor(e.Config)
	if err != nil {
		return "", fmt.Errorf("exec tls config: %w", err)
	}
	wsURL, err := execURL(e.Config.Host, namespace, pod, container, command)
	if err != nil {
		return "", err
	}
	token, err := bearerToken(e.Config)
	if err != nil {
		return "", err
	}
	wcfg, err := websocket.NewConfig(wsURL, "http://localhost")
	if err != nil {
		return "", fmt.Errorf("exec websocket config: %w", err)
	}
	wcfg.Protocol = []string{subProtocol}
	wcfg.TlsConfig = tlsCfg
	if token != "" {
		wcfg.Header.Set("Authorization", "Bearer "+token)
	}
	timeout := e.Timeout
	if timeout <= 0 {
		timeout = 2 * time.Minute
	}
	// One deadline covers the dial (TCP, TLS, HTTP upgrade) and the exchange, so a stalled API server
	// cannot hold the reconcile worker past the timeout, and cancelling ctx aborts the dial.
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	deadline, _ := ctx.Deadline()

	ws, err := wcfg.DialContext(ctx)
	if err != nil {
		return "", fmt.Errorf("exec dial %s/%s: %w", namespace, pod, err)
	}
	defer ws.Close()

	if err := ws.SetDeadline(deadline); err != nil {
		return "", fmt.Errorf("exec set deadline: %w", err)
	}
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-ctx.Done():
			_ = ws.Close()
		case <-done:
		}
	}()

	var stdout, stderr, status bytes.Buffer
	for {
		var msg []byte
		if err := websocket.Message.Receive(ws, &msg); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return "", fmt.Errorf("exec read %s/%s: %w", namespace, pod, err)
		}
		if len(msg) < 2 {
			continue // channel announcement without payload
		}
		switch msg[0] {
		case channelStdout:
			stdout.Write(msg[1:])
		case channelStderr:
			stderr.Write(msg[1:])
		case channelError:
			status.Write(msg[1:])
		}
	}
	if err := statusError(status.Bytes(), stderr.String()); err != nil {
		return "", err
	}
	return stdout.String(), nil
}

// statusError converts the channel-3 status document into an error (nil for Success or no document).
func statusError(raw []byte, stderr string) error {
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	var st execStatus
	if err := json.Unmarshal(raw, &st); err != nil {
		return fmt.Errorf("exec: unreadable status: %w", err)
	}
	if st.Status == "" || st.Status == "Success" {
		return nil
	}
	detail := strings.TrimSpace(stderr)
	if len(detail) > 200 {
		detail = detail[:200]
	}
	return fmt.Errorf("exec failed: %s (%s) %s", st.Message, st.Reason, detail)
}

func execURL(host, namespace, pod, container string, command []string) (string, error) {
	if !strings.Contains(host, "://") {
		host = "https://" + host
	}
	u, err := url.Parse(host)
	if err != nil {
		return "", fmt.Errorf("exec: bad API server host %q: %w", host, err)
	}
	switch u.Scheme {
	case "https":
		u.Scheme = "wss"
	case "http":
		u.Scheme = "ws"
	default:
		return "", fmt.Errorf("exec: unsupported API server scheme %q", u.Scheme)
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/api/v1/namespaces/" + url.PathEscape(namespace) + "/pods/" + url.PathEscape(pod) + "/exec"
	q := url.Values{}
	q.Set("container", container)
	for _, c := range command {
		q.Add("command", c)
	}
	q.Set("stdout", "true")
	q.Set("stderr", "true")
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// bearerToken prefers the token file because projected service-account tokens rotate.
func bearerToken(cfg *rest.Config) (string, error) {
	if cfg.BearerTokenFile != "" {
		b, err := os.ReadFile(cfg.BearerTokenFile)
		if err != nil {
			return "", fmt.Errorf("exec: read token file: %w", err)
		}
		return strings.TrimSpace(string(b)), nil
	}
	return cfg.BearerToken, nil
}
