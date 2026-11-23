// Copyright (c) 2026 Accenture, All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package api

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/acn-horizon-sdv/horizon-api/internal/workflow"
	"github.com/acn-horizon-sdv/horizonauth"
)

func (s *Server) handleWorkflowLogs(w http.ResponseWriter, r *http.Request, _ *horizonauth.Principal) {
	if r.Method != http.MethodGet {
		http.NotFound(w, r)
		return
	}
	if s.opt.Argo == nil || strings.TrimSpace(s.opt.Argo.BaseURL) == "" {
		http.Error(w, "workflow log streaming is not configured (argo-base-url)", http.StatusServiceUnavailable)
		return
	}
	ctx := r.Context()
	name := r.PathValue("workflowName")
	if name == "" {
		http.NotFound(w, r)
		return
	}
	q := r.URL.Query()
	podName := strings.TrimSpace(q.Get("podName"))
	container := q.Get("container")
	if container == "" {
		container = "main"
	}
	follow := true
	if v := q.Get("follow"); v != "" {
		follow, _ = strconv.ParseBool(v)
	}

	if s.opt.WorkflowStore != nil {
		u, gerr := s.opt.WorkflowStore.Get(ctx, name)
		if gerr != nil || !workflow.IsHorizonClientVisible(u) {
			http.Error(w, "workflow not found or argo error", http.StatusNotFound)
			return
		}
	}

	phase, _, err := s.opt.Argo.WorkflowPhase(ctx, s.opt.WorkflowsNS, name)
	if err != nil {
		http.Error(w, "workflow not found or argo error", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/x-ndjson; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	// Hint reverse proxies (nginx, some L7 LBs) not to buffer the streaming body.
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	fl, _ := w.(http.Flusher)
	if fl != nil {
		fl.Flush()
	}

	terminal := logTerminalPhase(phase)
	var streamErr error
	if podName != "" {
		streamErr = s.streamArgoLogs(ctx, w, fl, name, podName, container, follow, terminal)
	} else if follow && !terminal && s.opt.WorkflowStore != nil {
		// Live follow: Argo's workflow-level combined log often batches until steps finish.
		// Poll Workflow CR for new pod nodes and attach per-pod follow streams so lines emit as each step runs.
		streamErr = s.streamArgoLogsDynamicPods(ctx, w, fl, name, container, s.opt.WorkflowsNS)
	} else {
		streamErr = s.streamArgoLogsCombined(ctx, w, fl, name, container, follow, terminal)
	}
	if streamErr != nil {
		_ = writeLogTerminal(w, fl, "upstream_error", "", streamErr.Error())
		return
	}
	s.finalizeWorkflowLogStream(ctx, w, fl, s.opt.WorkflowsNS, name)
}

func logTerminalPhase(phase string) bool {
	switch strings.TrimSpace(phase) {
	case "Succeeded", "Failed", "Error":
		return true
	default:
		return false
	}
}

// streamArgoLogsCombined prefers per-pod streams from Workflow status (ordered by node startedAt) with stage metadata
// on each line. Argo's workflow-level log (empty podName) often returns only the active step or batches oddly.
// When Workflow status has no pod nodes, falls back to Argo's combined stream.
func (s *Server) streamArgoLogsCombined(ctx context.Context, w http.ResponseWriter, fl http.Flusher, workflowName, container string, follow bool, workflowTerminal bool) error {
	if s.opt.WorkflowStore != nil {
		if u, gerr := s.opt.WorkflowStore.Get(ctx, workflowName); gerr == nil {
			podNames, _ := s.opt.WorkflowStore.ListPodNamesForWorkflow(ctx, workflowName)
			targets := workflow.PodLogTargetsFromRunningPods(u, podNames)
			if len(targets) > 0 {
				workflow.SortPodLogTargetsByStartedAt(u, targets)
				return s.streamArgoLogsPerPodSequential(ctx, w, fl, workflowName, container, follow, workflowTerminal, targets)
			}
		}
	}

	body, err := s.opt.Argo.OpenLogStream(ctx, s.opt.WorkflowsNS, workflowName, "", container, false)
	if err == nil {
		rerr := s.readArgoNDJSON(ctx, w, fl, body, nil)
		_ = body.Close()
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if !(follow && !workflowTerminal) {
			if rerr != nil && !errors.Is(rerr, io.EOF) {
				return rerr
			}
			return nil
		}
		body2, err2 := s.opt.Argo.OpenLogStream(ctx, s.opt.WorkflowsNS, workflowName, "", container, true)
		if err2 == nil {
			tailErr := s.readArgoNDJSON(ctx, w, fl, body2, nil)
			_ = body2.Close()
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if tailErr != nil && !errors.Is(tailErr, io.EOF) {
				return tailErr
			}
			return nil
		}
		return nil
	}
	if s.opt.WorkflowStore == nil {
		if err != nil {
			return fmt.Errorf("argo combined log: %w", err)
		}
		return err
	}
	u, gerr := s.opt.WorkflowStore.Get(ctx, workflowName)
	if gerr != nil {
		if err != nil {
			return fmt.Errorf("argo combined log: %v; workflow get: %w", err, gerr)
		}
		return gerr
	}
	podNames, _ := s.opt.WorkflowStore.ListPodNamesForWorkflow(ctx, workflowName)
	targets := workflow.PodLogTargetsFromRunningPods(u, podNames)
	if len(targets) == 0 {
		if err != nil {
			return fmt.Errorf("argo combined log: %w; no pod nodes in workflow status", err)
		}
		return fmt.Errorf("no pod nodes to stream logs from")
	}
	workflow.SortPodLogTargetsByStartedAt(u, targets)
	return s.streamArgoLogsPerPodSequential(ctx, w, fl, workflowName, container, follow, workflowTerminal, targets)
}

func (s *Server) streamArgoLogsPerPodSequential(ctx context.Context, w http.ResponseWriter, fl http.Flusher, workflowName, container string, follow bool, workflowTerminal bool, targets []workflow.PodLogTarget) error {
	for i := range targets {
		t := &targets[i]
		if err := s.streamArgoSinglePodLogs(ctx, w, fl, workflowName, t.PodName, container, follow, workflowTerminal, t); err != nil {
			return err
		}
	}
	return nil
}

type multiplexLine struct {
	t    workflow.PodLogTarget
	line []byte
}

// followPodLogToChannel forwards one pod's Argo NDJSON log lines to ch. Unless skipInitialDrain is set,
// it drains retained logs with follow=false first (so clients see output from container start), then if
// follow is true opens a live tail (follow=true). skipInitialDrain is used when those logs were already
// emitted in a prior snapshot pass. OpenLogStream is retried with backoff while the pod is not yet available (404).
func (s *Server) followPodLogToChannel(ctx context.Context, ch chan<- multiplexLine, t workflow.PodLogTarget, workflowName, container string, follow, skipInitialDrain bool) {
	if !skipInitialDrain {
		s.drainPodLogStreamToChannel(ctx, ch, t, workflowName, container, false)
	}
	if !follow || ctx.Err() != nil {
		return
	}
	max := s.opt.LogMaxReconnect
	if max <= 0 {
		max = 10
	}
	backoff := 500 * time.Millisecond
	for attempt := 0; attempt < max; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			if backoff < 30*time.Second {
				backoff *= 2
			}
		}
		body, err := s.opt.Argo.OpenLogStream(ctx, s.opt.WorkflowsNS, workflowName, t.PodName, container, true)
		if err != nil {
			if attempt < max-1 {
				continue
			}
			return
		}
		func() {
			defer body.Close()
			br := bufio.NewReaderSize(body, 32*1024)
			for {
				line, rerr := br.ReadBytes('\n')
				if len(line) > 0 {
					cp := append([]byte(nil), line...)
					select {
					case ch <- multiplexLine{t: t, line: cp}:
					case <-ctx.Done():
						return
					}
				}
				if rerr != nil {
					return
				}
			}
		}()
		return
	}
}

func (s *Server) drainPodLogStreamToChannel(ctx context.Context, ch chan<- multiplexLine, t workflow.PodLogTarget, workflowName, container string, follow bool) {
	max := s.opt.LogMaxReconnect
	if max <= 0 {
		max = 10
	}
	backoff := 500 * time.Millisecond
	for attempt := 0; attempt < max; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return
			case <-time.After(backoff):
			}
			if backoff < 30*time.Second {
				backoff *= 2
			}
		}
		body, err := s.opt.Argo.OpenLogStream(ctx, s.opt.WorkflowsNS, workflowName, t.PodName, container, follow)
		if err != nil {
			if attempt < max-1 {
				continue
			}
			return
		}
		func() {
			defer body.Close()
			br := bufio.NewReaderSize(body, 32*1024)
			for {
				line, rerr := br.ReadBytes('\n')
				if len(line) > 0 {
					cp := append([]byte(nil), line...)
					select {
					case ch <- multiplexLine{t: t, line: cp}:
					case <-ctx.Done():
						return
					}
				}
				if rerr != nil {
					return
				}
			}
		}()
		return
	}
}

// streamArgoLogsDynamicPods follows a running workflow by multiplexing per-pod live tails. First it
// emits a snapshot (follow=false) for every known pod in startedAt order so completed steps are visible
// even if the client connected mid-run; then it tails each pod, skipping the duplicate drain for pods
// already snapshotted. New pods discovered later get a full drain+tail.
func (s *Server) streamArgoLogsDynamicPods(ctx context.Context, w http.ResponseWriter, fl http.Flusher, workflowName, container, ns string) error {
	if s.opt.WorkflowStore == nil {
		return fmt.Errorf("workflow store required for live multi-pod logs")
	}
	u, gerr := s.opt.WorkflowStore.Get(ctx, workflowName)
	if gerr != nil {
		return fmt.Errorf("workflow get: %w", gerr)
	}
	podNames, _ := s.opt.WorkflowStore.ListPodNamesForWorkflow(ctx, workflowName)
	targets := workflow.PodLogTargetsFromRunningPods(u, podNames)
	workflow.SortPodLogTargetsByStartedAt(u, targets)

	snapshotted := map[string]bool{}
	for _, t := range targets {
		if t.PodName != "" {
			snapshotted[t.PodName] = true
		}
	}
	if len(targets) > 0 {
		if err := s.streamArgoLogsPerPodSequential(ctx, w, fl, workflowName, container, false, true, targets); err != nil {
			return err
		}
	}

	ch := make(chan multiplexLine, 256)
	var streamWg sync.WaitGroup
	var seenMu sync.Mutex
	seen := map[string]bool{}

	startStream := func(t workflow.PodLogTarget, skipInitialDrain bool) {
		streamWg.Add(1)
		go func(t workflow.PodLogTarget, skipInitialDrain bool) {
			defer streamWg.Done()
			s.followPodLogToChannel(ctx, ch, t, workflowName, container, true, skipInitialDrain)
		}(t, skipInitialDrain)
	}

	go func() {
		defer close(ch)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			phase, _, err := s.opt.Argo.WorkflowPhase(ctx, ns, workflowName)
			if err != nil || logTerminalPhase(phase) {
				streamWg.Wait()
				return
			}
			podNames, _ := s.opt.WorkflowStore.ListPodNamesForWorkflow(ctx, workflowName)
			u2, gerr2 := s.opt.WorkflowStore.Get(ctx, workflowName)
			if gerr2 == nil {
				for _, t := range workflow.PodLogTargetsFromRunningPods(u2, podNames) {
					if t.PodName == "" {
						continue
					}
					skipDrain := snapshotted[t.PodName]
					seenMu.Lock()
					already := seen[t.PodName]
					if !already {
						seen[t.PodName] = true
					}
					seenMu.Unlock()
					if !already {
						startStream(t, skipDrain)
					}
				}
			}
			select {
			case <-ctx.Done():
				streamWg.Wait()
				return
			case <-ticker.C:
			}
		}
	}()

	return s.mergeMultiplexNDJSON(ctx, w, fl, ch)
}

func (s *Server) mergeMultiplexNDJSON(ctx context.Context, w http.ResponseWriter, fl http.Flusher, ch <-chan multiplexLine) error {
	idle := s.opt.LogReadIdle
	if idle <= 0 {
		idle = 30 * time.Second
	}
	t := time.NewTimer(idle)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			if err := writeLogHeartbeat(w, fl); err != nil {
				return err
			}
			if !t.Stop() {
				select {
				case <-t.C:
				default:
				}
			}
			t.Reset(idle)
		case ml, ok := <-ch:
			if !ok {
				return nil
			}
			if !t.Stop() {
				select {
				case <-t.C:
				default:
				}
			}
			t.Reset(idle)
			if len(ml.line) == 0 {
				continue
			}
			msg, pod, ok := parseArgoLogLine(ml.line)
			if !ok {
				msg = strings.TrimSpace(string(ml.line))
				if msg == "" {
					continue
				}
				pod = ml.t.PodName
			} else if msg == "" {
				continue
			}
			if pod == "" {
				pod = ml.t.PodName
			}
			if err := writeLogNDJSONLineWithMeta(w, fl, pod, msg, ml.t.NodeID, ml.t.DisplayName, ml.t.TemplateName); err != nil {
				return err
			}
		}
	}
}

func (s *Server) streamArgoLogs(ctx context.Context, w http.ResponseWriter, fl http.Flusher, workflowName, podName, container string, follow, workflowTerminal bool) error {
	var meta *workflow.PodLogTarget
	if s.opt.WorkflowStore != nil && podName != "" {
		if u, gerr := s.opt.WorkflowStore.Get(ctx, workflowName); gerr == nil {
			podNames, _ := s.opt.WorkflowStore.ListPodNamesForWorkflow(ctx, workflowName)
			for _, t := range workflow.PodLogTargetsFromRunningPods(u, podNames) {
				if t.PodName == podName {
					tcopy := t
					meta = &tcopy
					break
				}
			}
		}
	}
	return s.streamArgoSinglePodLogs(ctx, w, fl, workflowName, podName, container, follow, workflowTerminal, meta)
}

// streamArgoSinglePodLogs streams one pod's main container logs. meta adds displayName/templateName/nodeId to each NDJSON line when non-nil.
func (s *Server) streamArgoSinglePodLogs(ctx context.Context, w http.ResponseWriter, fl http.Flusher, workflowName, podName, container string, follow, workflowTerminal bool, meta *workflow.PodLogTarget) error {
	max := s.opt.LogMaxReconnect
	if max <= 0 {
		max = 10
	}
	backoff := 500 * time.Millisecond
	var body io.ReadCloser
	var err error
	for attempt := 0; attempt < max; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
			if backoff < 30*time.Second {
				backoff *= 2
			}
		}
		body, err = s.opt.Argo.OpenLogStream(ctx, s.opt.WorkflowsNS, workflowName, podName, container, false)
		if err == nil {
			break
		}
		if attempt < max-1 {
			continue
		}
		return err
	}
	rerr := s.readArgoNDJSON(ctx, w, fl, body, meta)
	_ = body.Close()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if !(follow && !workflowTerminal) {
		if rerr != nil && !errors.Is(rerr, io.EOF) {
			return rerr
		}
		return nil
	}
	backoff = 500 * time.Millisecond
	for attempt := 0; attempt < max; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
			if backoff < 30*time.Second {
				backoff *= 2
			}
		}
		body, err = s.opt.Argo.OpenLogStream(ctx, s.opt.WorkflowsNS, workflowName, podName, container, true)
		if err != nil {
			if attempt < max-1 {
				continue
			}
			return nil
		}
		tailErr := s.readArgoNDJSON(ctx, w, fl, body, meta)
		_ = body.Close()
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if tailErr != nil && !errors.Is(tailErr, io.EOF) {
			return tailErr
		}
		return nil
	}
	return fmt.Errorf("exceeded reconnect attempts")
}

type readLineResult struct {
	line []byte
	err  error
}

// meta optional: when set, add nodeId/displayName/templateName to each log line.
func (s *Server) readArgoNDJSON(ctx context.Context, w http.ResponseWriter, fl http.Flusher, body io.Reader, meta *workflow.PodLogTarget) error {
	br := bufio.NewReaderSize(body, 64*1024)
	lines := make(chan readLineResult, 1)
	go func() {
		for {
			line, err := br.ReadBytes('\n')
			select {
			case lines <- readLineResult{line, err}:
				if err != nil {
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	idle := s.opt.LogReadIdle
	if idle <= 0 {
		idle = 30 * time.Second
	}
	t := time.NewTimer(idle)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-t.C:
			if err := writeLogHeartbeat(w, fl); err != nil {
				return err
			}
			if !t.Stop() {
				select {
				case <-t.C:
				default:
				}
			}
			t.Reset(idle)
		case r := <-lines:
			if !t.Stop() {
				select {
				case <-t.C:
				default:
				}
			}
			t.Reset(idle)
			if len(r.line) > 0 {
				msg, pod, ok := parseArgoLogLine(r.line)
				if ok && msg != "" {
					var err error
					if meta != nil {
						if pod == "" {
							pod = meta.PodName
						}
						err = writeLogNDJSONLineWithMeta(w, fl, pod, msg, meta.NodeID, meta.DisplayName, meta.TemplateName)
					} else {
						err = writeLogNDJSONLine(w, fl, pod, msg)
					}
					if err != nil {
						return err
					}
				}
			}
			if r.err == io.EOF {
				return nil
			}
			if r.err != nil {
				return r.err
			}
		}
	}
}

type argoLogEnvelope struct {
	Result *struct {
		Content string `json:"content"`
		PodName string `json:"podName"`
	} `json:"result"`
}

func parseArgoLogLine(b []byte) (msg, podName string, ok bool) {
	var env argoLogEnvelope
	if err := json.Unmarshal(b, &env); err != nil || env.Result == nil {
		return "", "", false
	}
	return env.Result.Content, env.Result.PodName, true
}

func writeLogNDJSONLine(w io.Writer, fl http.Flusher, podName, msg string) error {
	o := map[string]string{
		"ts":      time.Now().UTC().Format(time.RFC3339Nano),
		"msg":     msg,
		"podName": podName,
	}
	return writeLogJSONLine(w, fl, o)
}

func writeLogNDJSONLineWithMeta(w io.Writer, fl http.Flusher, podName, msg, nodeID, displayName, templateName string) error {
	o := map[string]string{
		"ts":      time.Now().UTC().Format(time.RFC3339Nano),
		"msg":     msg,
		"podName": podName,
	}
	if nodeID != "" {
		o["nodeId"] = nodeID
	}
	if displayName != "" {
		o["displayName"] = displayName
	}
	if templateName != "" {
		o["templateName"] = templateName
	}
	return writeLogJSONLine(w, fl, o)
}

func writeLogHeartbeat(w io.Writer, fl http.Flusher) error {
	return writeLogJSONLine(w, fl, map[string]bool{"heartbeat": true})
}

func writeLogTerminal(w io.Writer, fl http.Flusher, reason, workflowStatus, detail string) error {
	m := map[string]string{"result": "done", "reason": reason}
	if workflowStatus != "" {
		m["workflowStatus"] = workflowStatus
	}
	if detail != "" {
		m["detail"] = detail
	}
	return writeLogJSONLine(w, fl, m)
}

func writeLogJSONLine(w io.Writer, fl http.Flusher, v interface{}) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	if _, err := w.Write(b); err != nil {
		return err
	}
	if fl != nil {
		fl.Flush()
	}
	return nil
}

func (s *Server) finalizeWorkflowLogStream(ctx context.Context, w http.ResponseWriter, fl http.Flusher, ns, name string) {
	var display string
	if s.opt.WorkflowStore != nil {
		if u, err := s.opt.WorkflowStore.Get(ctx, name); err == nil {
			display = workflow.DisplayPhaseForAPI(u)
		}
	}
	if display == "" && s.opt.Argo != nil {
		p, _, err := s.opt.Argo.WorkflowPhase(ctx, ns, name)
		if err != nil {
			_ = writeLogTerminal(w, fl, "upstream_error", "", err.Error())
			return
		}
		display = p
	}
	switch display {
	case "Aborted":
		_ = writeLogTerminal(w, fl, "workflow_aborted", "Aborted", "")
	case "Succeeded":
		_ = writeLogTerminal(w, fl, "workflow_completed", "Succeeded", "")
	case "Failed":
		_ = writeLogTerminal(w, fl, "workflow_failed", "Failed", "")
	case "Error":
		_ = writeLogTerminal(w, fl, "workflow_failed", "Error", "")
	default:
		_ = writeLogTerminal(w, fl, "workflow_completed", display, "")
	}
}
