// Copyright (c) 2026 Accenture, All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
//
//         http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/term"
)

const submittedFromParamName = "horizonSubmittedFrom"
const submittedFromCLIValue = "horizon-cli"

func stringParamEmpty(v any) bool {
	if v == nil {
		return true
	}
	s, ok := v.(string)
	return ok && s == ""
}

// catalogParameter matches GET /v1/catalog entries[].parameters[].
type catalogParameter struct {
	Name        string `json:"name"`
	Default     string `json:"default,omitempty"`
	Description string `json:"description,omitempty"`
}

// catalogEntry matches one GET /v1/catalog entries[] item.
type catalogEntry struct {
	Module       string             `json:"module"`
	TemplateName string             `json:"templateName"`
	Parameters   []catalogParameter `json:"parameters"`
}

func findCatalogEntry(ctx context.Context, c *Client, module, template string) (*catalogEntry, error) {
	b, err := c.GetJSON(ctx, "/v1/catalog")
	if err != nil {
		return nil, err
	}
	var resp struct {
		Entries []catalogEntry `json:"entries"`
	}
	if err := json.Unmarshal(b, &resp); err != nil {
		return nil, fmt.Errorf("catalog JSON: %w", err)
	}
	for i := range resp.Entries {
		e := &resp.Entries[i]
		if e.Module == module && e.TemplateName == template {
			return e, nil
		}
	}
	return nil, fmt.Errorf("catalog missing module=%s template=%s", module, template)
}

// mergeSubmitParamsFromCatalog builds the submit "parameters" object from catalog-declared
// names and defaults, then overlays user JSON. Rejects keys not declared for this template.
func mergeSubmitParamsFromCatalog(user map[string]any, entry *catalogEntry) (map[string]any, error) {
	byName := make(map[string]catalogParameter, len(entry.Parameters))
	for _, p := range entry.Parameters {
		byName[p.Name] = p
	}
	for k := range user {
		if _, ok := byName[k]; !ok {
			return nil, fmt.Errorf("unknown workflow parameter %q (not in catalog for %s/%s); see GET /v1/catalog",
				k, entry.Module, entry.TemplateName)
		}
	}
	out := make(map[string]any, len(entry.Parameters))
	for _, p := range entry.Parameters {
		uv, userSet := user[p.Name]
		switch {
		case !userSet:
			out[p.Name] = p.Default
		case stringParamEmpty(uv):
			if p.Default != "" {
				out[p.Name] = p.Default
			} else {
				out[p.Name] = ""
			}
		default:
			out[p.Name] = uv
		}
	}
	return out, nil
}

func readParamsJSON(paramsFile, paramsEnv string) (map[string]any, error) {
	var raw []byte
	switch {
	case strings.TrimSpace(paramsFile) == "-":
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, fmt.Errorf("read params stdin: %w", err)
		}
		raw = b
	case strings.TrimSpace(paramsFile) != "":
		b, err := os.ReadFile(paramsFile)
		if err != nil {
			return nil, fmt.Errorf("read params file: %w", err)
		}
		raw = b
	case strings.TrimSpace(paramsEnv) != "":
		raw = []byte(paramsEnv)
	default:
		raw = []byte("{}")
	}
	raw = bytesTrimSpace(raw)
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("workflow parameters JSON: %w", err)
	}
	if m == nil {
		m = map[string]any{}
	}
	return m, nil
}

func bytesTrimSpace(b []byte) []byte {
	return []byte(strings.TrimSpace(string(b)))
}

func runningNames(ctx context.Context, c *Client, cfg *Config) ([]string, error) {
	path := fmt.Sprintf("/v1/workflows/running?limit=%d", cfg.RunningPollLimit)
	b, err := c.GetJSON(ctx, path)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Items []struct {
			Name string `json:"name"`
		} `json:"items"`
	}
	if err := json.Unmarshal(b, &resp); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(resp.Items))
	for _, it := range resp.Items {
		if it.Name != "" {
			names = append(names, it.Name)
		}
	}
	return names, nil
}

func pickNewName(before, after []string) string {
	bset := make(map[string]struct{}, len(before))
	for _, n := range before {
		bset[n] = struct{}{}
	}
	for _, n := range after {
		if _, ok := bset[n]; !ok {
			return n
		}
	}
	return ""
}

func waitForNewRunning(ctx context.Context, c *Client, cfg *Config, before []string) (string, error) {
	for i := 0; i < cfg.WaitNewAttempts; i++ {
		after, err := runningNames(ctx, c, cfg)
		if err != nil {
			return "", err
		}
		if w := pickNewName(before, after); w != "" {
			return w, nil
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(time.Duration(cfg.WaitNewSleepSec) * time.Second):
		}
	}
	return "", fmt.Errorf("no new workflow in time (check Argo Events)")
}

func workflowPhase(ctx context.Context, c *Client, wf string) (string, error) {
	b, err := c.GetJSON(ctx, "/v1/workflows/"+url.PathEscape(wf))
	if err != nil {
		return "", err
	}
	var detail struct {
		Phase string `json:"phase"`
	}
	if err := json.Unmarshal(b, &detail); err != nil {
		return "", err
	}
	return detail.Phase, nil
}

func hasPodHint(ctx context.Context, c *Client, wf string) (bool, error) {
	b, err := c.GetJSON(ctx, "/v1/workflows/"+url.PathEscape(wf))
	if err != nil {
		return false, err
	}
	var detail struct {
		Nodes []struct {
			PodName *string `json:"podName"`
		} `json:"nodes"`
	}
	if err := json.Unmarshal(b, &detail); err != nil {
		return false, err
	}
	for _, n := range detail.Nodes {
		if n.PodName != nil && strings.TrimSpace(*n.PodName) != "" {
			return true, nil
		}
	}
	return false, nil
}

func waitForPodHint(ctx context.Context, c *Client, cfg *Config, wf string) error {
	if cfg.LogWaitPodSecs <= 0 {
		return nil
	}
	deadline := time.Now().Add(time.Duration(cfg.LogWaitPodSecs) * time.Second)
	for time.Now().Before(deadline) {
		ok, err := hasPodHint(ctx, c, wf)
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return nil
}

// waitForPodHintVerbose prints progress on stderr until pods exist or wait budget ends (for --logs).
func waitForPodHintVerbose(ctx context.Context, c *Client, cfg *Config, wf string) error {
	if cfg.LogWaitPodSecs <= 0 {
		fmt.Fprintf(os.Stderr, "Logs: opening live stream for workflow %s (pod wait disabled).\n", wf)
		return nil
	}
	fmt.Fprintf(os.Stderr, "Logs: waiting for workflow pods (workflow %s, up to %ds) …\n", wf, cfg.LogWaitPodSecs)
	deadline := time.Now().Add(time.Duration(cfg.LogWaitPodSecs) * time.Second)
	for time.Now().Before(deadline) {
		ok, err := hasPodHint(ctx, c, wf)
		if err != nil {
			return err
		}
		if ok {
			fmt.Fprintf(os.Stderr, "Logs: pods are visible; streaming output below.\n")
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
		fmt.Fprintf(os.Stderr, "Logs: still waiting for pod assignment …\n")
	}
	fmt.Fprintf(os.Stderr, "Logs: opening stream anyway (pods may appear shortly).\n")
	return nil
}

func terminalPhase(ph string) bool {
	switch ph {
	case "Succeeded", "Failed", "Error", "Aborted":
		return true
	default:
		return false
	}
}

func phaseToExitCode(ph string) int {
	switch ph {
	case "Succeeded":
		return 0
	case "Aborted":
		return 2
	case "Failed", "Error":
		return 1
	default:
		return 1
	}
}

// stringFromAny turns a JSON-decoded value into a display string. nil and absent fields → "" (not "<nil>").
func stringFromAny(v any) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return strings.TrimSpace(x)
	default:
		return strings.TrimSpace(fmt.Sprint(x))
	}
}

func formatNDJSONLine(trimmed, originalLine []byte, formatted bool) ([]byte, bool) {
	if len(bytes.TrimSpace(trimmed)) == 0 {
		return nil, false
	}
	if !formatted {
		out := originalLine
		if !bytes.HasSuffix(out, []byte("\n")) {
			out = append(append([]byte{}, out...), '\n')
		}
		return out, true
	}
	var o map[string]any
	if err := json.Unmarshal(trimmed, &o); err != nil {
		out := originalLine
		if !bytes.HasSuffix(out, []byte("\n")) {
			out = append(append([]byte{}, out...), '\n')
		}
		return out, true
	}
	if hb, ok := o["heartbeat"].(bool); ok && hb {
		return nil, false
	}
	if res, _ := o["result"].(string); res == "done" {
		reason, _ := o["reason"].(string)
		ws, _ := o["workflowStatus"].(string)
		detail, _ := o["detail"].(string)
		stage := ws
		if stage == "" {
			stage = reason
		}
		if stage == "" {
			stage = "done"
		}
		msg := detail
		if msg == "" {
			msg = reason
		}
		if msg == "" {
			msg = "-"
		}
		return []byte(fmt.Sprintf("[%s] [-] [%s]\n", stage, singleLine(msg))), true
	}
	ts := stringFromAny(o["ts"])
	if ts == "" {
		ts = "-"
	}
	msg := stringFromAny(o["msg"])
	stage := stringFromAny(o["displayName"])
	if stage == "" {
		stage = stringFromAny(o["templateName"])
	}
	if stage == "" {
		stage = stringFromAny(o["podName"])
	}
	if stage == "" {
		stage = "log"
	}
	return []byte(fmt.Sprintf("[%s] [%s] [%s]\n", stage, ts, singleLine(msg))), true
}

func singleLine(s string) string {
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.TrimSpace(s)
}

func followWorkflowLogsDuringWait(ctx context.Context, c *Client, cfg *Config, wf string) error {
	if err := waitForPodHintVerbose(ctx, c, cfg, wf); err != nil {
		return err
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return streamLogs(ctx, c, cfg, wf)
}

// waitWithOptionalLogs runs pollUntilTerminal and, when followLogs is true, log streaming in parallel.
// Cancels the shared context when the workflow reaches a terminal phase (or poll errors).
func waitWithOptionalLogs(ctx context.Context, c *Client, cfg *Config, wf string, followLogs bool) (int, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var ph string
	var pollErr error
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		ph, pollErr = pollUntilTerminal(ctx, c, cfg, wf)
		cancel()
	}()

	var logErr error
	if followLogs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			logErr = followWorkflowLogsDuringWait(ctx, c, cfg, wf)
		}()
	}
	wg.Wait()

	if pollErr != nil {
		return 1, pollErr
	}
	if followLogs && logErr != nil && !errors.Is(logErr, context.Canceled) {
		fmt.Fprintf(os.Stderr, "logs: %v\n", logErr)
	}
	return phaseToExitCode(ph), nil
}

func streamLogs(ctx context.Context, c *Client, cfg *Config, wf string) error {
	ph, err := workflowPhase(ctx, c, wf)
	if err != nil {
		return err
	}
	follow := "true"
	if terminalPhase(ph) {
		follow = "false"
	}
	logURL := fmt.Sprintf("%s/v1/workflows/%s/log?follow=%s&container=main",
		cfg.BaseURL, url.PathEscape(wf), follow)
	fmt.Printf("━━ Horizon log stream ━━ %s (max-time=%ds follow=%s format=%s)\n",
		logURL, cfg.LogStreamMaxSecs, follow, cfg.LogStreamFormat)

	sub, cancel := context.WithTimeout(ctx, time.Duration(cfg.LogStreamMaxSecs)*time.Second)
	defer cancel()

	resp, err := c.StreamGET(sub, logURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	formatted := cfg.LogStreamFormat != "RAW"
	br := bufio.NewReader(resp.Body)
	for {
		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			trim := trimCRLFLine(line)
			out, ok := formatNDJSONLine(trim, line, formatted)
			if ok {
				os.Stdout.Write(out)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}
	fmt.Println("━━ log stream end ━━")
	return nil
}

func trimCRLFLine(b []byte) []byte {
	b = bytes.TrimSuffix(b, []byte("\n"))
	b = bytes.TrimSuffix(b, []byte("\r"))
	return b
}

// pollUntilTerminal blocks until workflow reaches a terminal phase or timeout.
func pollUntilTerminal(ctx context.Context, c *Client, cfg *Config, wf string) (string, error) {
	deadline := time.Now().Add(time.Duration(cfg.WaitTerminalSecs) * time.Second)
	for time.Now().Before(deadline) {
		ph, err := workflowPhase(ctx, c, wf)
		if err != nil {
			return "", err
		}
		if terminalPhase(ph) {
			fmt.Fprintf(os.Stderr, "Terminal phase: %s\n", ph)
			return ph, nil
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(time.Duration(cfg.TerminalPollInterval) * time.Second):
		}
	}
	return "", fmt.Errorf("timeout waiting for terminal phase on %s", wf)
}

type submitOutputMode string

const (
	submitOutText submitOutputMode = "text"
	submitOutJSON submitOutputMode = "json"
)

func printSubmitAckHuman(resp []byte, outMode submitOutputMode, quiet bool) {
	if quiet || outMode != submitOutText {
		return
	}
	var m map[string]any
	if err := json.Unmarshal(resp, &m); err != nil {
		fmt.Println(string(resp))
		return
	}
	st := strings.ToLower(stringFromAny(m["status"]))
	if st == "dispatched" {
		fmt.Println("Submit accepted: Horizon handed the run off to the workflow event dispatcher; it should show up in /v1/workflows/running momentarily.")
		return
	}
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, resp, "", "  "); err != nil {
		fmt.Println(string(resp))
	} else {
		fmt.Println(pretty.String())
	}
}

func catalogHasParam(entry *catalogEntry, paramName string) bool {
	for _, p := range entry.Parameters {
		if p.Name == paramName {
			return true
		}
	}
	return false
}

// cmdSubmit posts a workflow; prints workflow name; optional --wait returns phase exit code.
func cmdSubmit(c *Client, cfg *Config, paramsFile, paramsEnv string, wait, followLogs bool, outMode submitOutputMode, quiet bool) (int, error) {
	ctx := context.Background()
	entry, err := findCatalogEntry(ctx, c, cfg.Module, cfg.Template)
	if err != nil {
		return 1, err
	}
	inner, err := readParamsJSON(paramsFile, paramsEnv)
	if err != nil {
		return 1, err
	}
	params, err := mergeSubmitParamsFromCatalog(inner, entry)
	if err != nil {
		return 1, err
	}
	if catalogHasParam(entry, submittedFromParamName) && stringParamEmpty(params[submittedFromParamName]) {
		// Keep source explicit in submit payload for templates that expose this parameter.
		params[submittedFromParamName] = submittedFromCLIValue
	}
	bodyMap := map[string]any{
		"parameters": params,
	}
	body, err := json.Marshal(bodyMap)
	if err != nil {
		return 1, err
	}
	before, err := runningNames(ctx, c, cfg)
	if err != nil {
		return 1, err
	}
	path := fmt.Sprintf("/v1/modules/%s/workflowTemplates/%s/submit",
		url.PathEscape(cfg.Module), url.PathEscape(cfg.Template))
	if !quiet {
		fmt.Fprintf(os.Stderr, "Submitting %s/%s …\n", cfg.Module, cfg.Template)
	}
	resp, err := c.DoJSONWithHeaders(ctx, "POST", path, body, map[string]string{
		"X-Horizon-Submitted-From": submittedFromCLIValue,
	})
	if err != nil {
		return 1, err
	}
	if !quiet {
		if outMode == submitOutJSON {
			// Response body printed after we know workflowName (below).
		} else {
			printSubmitAckHuman(resp, outMode, quiet)
			fmt.Fprintln(os.Stderr, "Waiting for new workflow in /v1/workflows/running …")
		}
	}

	wf, err := waitForNewRunning(ctx, c, cfg, before)
	if err != nil {
		return 1, err
	}

	if outMode == submitOutJSON {
		var submitObj map[string]any
		_ = json.Unmarshal(resp, &submitObj)
		wrap := map[string]any{
			"module":         cfg.Module,
			"template":       cfg.Template,
			"workflowName":   wf,
			"submitResponse": submitObj,
		}
		b, _ := json.MarshalIndent(wrap, "", "  ")
		fmt.Println(string(b))
	} else {
		fmt.Println("module=" + cfg.Module)
		fmt.Println("template=" + cfg.Template)
		fmt.Println("workflowName=" + wf)
	}

	if !wait {
		return 0, nil
	}
	if followLogs {
		return waitWithOptionalLogs(ctx, c, cfg, wf, true)
	}
	ph, err := pollUntilTerminal(ctx, c, cfg, wf)
	if err != nil {
		return 1, err
	}
	return phaseToExitCode(ph), nil
}

// cmdWait polls until terminal; exit code matches finalize semantics.
func cmdWait(c *Client, cfg *Config, wf string, followLogs bool) (int, error) {
	ctx := context.Background()
	if !followLogs {
		ph, err := pollUntilTerminal(ctx, c, cfg, wf)
		if err != nil {
			return 1, err
		}
		return phaseToExitCode(ph), nil
	}
	return waitWithOptionalLogs(ctx, c, cfg, wf, true)
}

// cmdLogsForWorkflow streams logs for a workflow name.
func cmdLogsForWorkflow(c *Client, cfg *Config, wf string) error {
	ctx := context.Background()
	if err := waitForPodHint(ctx, c, cfg, wf); err != nil {
		return err
	}
	err := streamLogs(ctx, c, cfg, wf)
	if err != nil {
		fmt.Fprintf(os.Stderr, "log stream: %v\n", err)
	}
	return nil
}

type archivedLogLinks struct {
	Combined *struct {
		GcsURI string `json:"gcsUri"`
	} `json:"combined"`
	Steps []struct {
		GcsURI string `json:"gcsUri"`
	} `json:"steps"`
}

type outputArtifact struct {
	Name   string `json:"name"`
	GcsURI string `json:"gcsUri"`
}

func printWorkflowShowText(b []byte) error {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(b, &root); err != nil {
		return err
	}
	summary := make(map[string]json.RawMessage)
	for _, k := range []string{"name", "phase", "workflowTemplate", "submittedFrom", "archivedLogs", "outputArtifacts"} {
		if v, ok := root[k]; ok {
			summary[k] = v
		}
	}
	sum, _ := json.MarshalIndent(summary, "", "  ")
	fmt.Println("━━ Workflow detail ━━")
	fmt.Println(string(sum))

	var ph string
	if raw, ok := root["phase"]; ok {
		_ = json.Unmarshal(raw, &ph)
	}

	fmt.Println("━━ GCS / artifact URIs ━━")
	var al archivedLogLinks
	if raw, ok := root["archivedLogs"]; ok && string(raw) != "null" {
		_ = json.Unmarshal(raw, &al)
	}
	if al.Combined != nil && al.Combined.GcsURI != "" {
		fmt.Println("archivedLogs.combined:", al.Combined.GcsURI)
	}
	for _, s := range al.Steps {
		if s.GcsURI != "" {
			fmt.Println("archivedLogs.step:", s.GcsURI)
		}
	}
	var arts []outputArtifact
	if raw, ok := root["outputArtifacts"]; ok {
		_ = json.Unmarshal(raw, &arts)
	}
	for _, a := range arts {
		if a.GcsURI != "" {
			fmt.Printf("outputArtifact: %s %s\n", a.Name, a.GcsURI)
		}
	}
	return nil
}

// cmdWorkflowShow prints workflow detail (--output json for raw body).
func cmdWorkflowShow(ctx context.Context, c *Client, wf, output string) error {
	b, err := c.GetJSON(ctx, "/v1/workflows/"+url.PathEscape(wf))
	if err != nil {
		return err
	}
	if output == "json" {
		var pretty bytes.Buffer
		if err := json.Indent(&pretty, b, "", "  "); err != nil {
			fmt.Println(string(b))
		} else {
			fmt.Println(pretty.String())
		}
		return nil
	}
	return printWorkflowShowText(b)
}

func cmdAbortWorkflowName(c *Client, wf string) error {
	ctx := context.Background()
	ph, err := workflowPhase(ctx, c, wf)
	if err != nil {
		ph = ""
	}
	if terminalPhase(ph) {
		return nil
	}
	fmt.Fprintf(os.Stderr, "Aborting workflow %s via Horizon API …\n", wf)
	path := "/v1/workflows/" + url.PathEscape(wf) + "/abort"
	_, _ = c.DoJSON(ctx, "POST", path, []byte("{}"))
	return nil
}

func cmdDeleteWorkflowName(ctx context.Context, c *Client, wf string) error {
	fmt.Fprintf(os.Stderr, "Deleting workflow %s via Horizon API (server waits until the Workflow CR and finalizers are gone; may take several minutes)…\n", wf)
	path := "/v1/workflows/" + url.PathEscape(wf)
	b, err := c.DoJSON(ctx, "DELETE", path, nil)
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(b)) > 0 {
		fmt.Println(string(b))
	}
	return nil
}

func cmdCatalogGet(ctx context.Context, c *Client, output string, raw bool) error {
	b, err := c.GetJSON(ctx, "/v1/catalog")
	if err != nil {
		return err
	}
	switch strings.ToLower(strings.TrimSpace(output)) {
	case "text":
		if raw {
			return fmt.Errorf("-raw applies only to -output json")
		}
		return printCatalogText(b)
	case "json":
		if raw {
			fmt.Println(string(b))
			return nil
		}
		var pretty bytes.Buffer
		if err := json.Indent(&pretty, b, "", "  "); err != nil {
			fmt.Println(string(b))
		} else {
			fmt.Println(pretty.String())
		}
		return nil
	default:
		return fmt.Errorf("catalog get: -output must be text or json")
	}
}

func printCatalogText(b []byte) error {
	var resp struct {
		Entries []struct {
			Module       string `json:"module"`
			TemplateName string `json:"templateName"`
			Namespace    string `json:"namespace"`
			Parameters   []struct {
				Name        string `json:"name"`
				Default     string `json:"default,omitempty"`
				Description string `json:"description,omitempty"`
			} `json:"parameters"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(b, &resp); err != nil {
		return fmt.Errorf("catalog JSON: %w", err)
	}
	n := len(resp.Entries)
	if n == 0 {
		fmt.Println("Catalog: (no entries)")
		return nil
	}
	pl := "ies"
	if n == 1 {
		pl = "y"
	}
	fmt.Printf("Catalog (%d entr%s)\n", n, pl)
	for _, e := range resp.Entries {
		fmt.Printf("\n  %s / %s\n", e.Module, e.TemplateName)
		if strings.TrimSpace(e.Namespace) != "" {
			fmt.Printf("    namespace: %s\n", e.Namespace)
		}
		if len(e.Parameters) == 0 {
			fmt.Println("    parameters: (none)")
			continue
		}
		fmt.Println("    parameters:")
		for _, p := range e.Parameters {
			line := fmt.Sprintf("      - %s", p.Name)
			if p.Default != "" {
				line += fmt.Sprintf(" (default: %q)", p.Default)
			}
			fmt.Println(line)
			if strings.TrimSpace(p.Description) != "" {
				fmt.Printf("          %s\n", strings.TrimSpace(p.Description))
			}
		}
	}
	return nil
}

type runningListItem struct {
	Name          string `json:"name"`
	Phase         string `json:"phase"`
	SubmittedFrom string `json:"submittedFrom,omitempty"`
}

type historyListItem struct {
	Name          string `json:"name"`
	Phase         string `json:"phase"`
	SubmittedFrom string `json:"submittedFrom,omitempty"`
}

func cmdWorkflowList(ctx context.Context, c *Client, runningOnly bool, limit int, continueToken, phaseFilter, output string) error {
	if limit <= 0 {
		limit = 50
	}
	autoOut := output == ""
	if autoOut {
		if term.IsTerminal(int(os.Stdout.Fd())) {
			output = "text"
		} else {
			output = "json"
		}
	}

	var runJSON, histJSON []byte
	var err error
	runJSON, err = c.GetJSON(ctx, fmt.Sprintf("/v1/workflows/running?limit=%d", limit))
	if err != nil {
		return err
	}
	if !runningOnly {
		q := url.Values{}
		q.Set("limit", fmt.Sprintf("%d", limit))
		if continueToken != "" {
			q.Set("continue", continueToken)
		}
		if phaseFilter != "" {
			q.Set("phase", phaseFilter)
		}
		histJSON, err = c.GetJSON(ctx, "/v1/workflows/history?"+q.Encode())
		if err != nil {
			return err
		}
	}

	if output == "json" || output == "wide" {
		combined := map[string]json.RawMessage{"running": runJSON}
		if !runningOnly {
			combined["history"] = histJSON
		}
		b, err := json.MarshalIndent(combined, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(b))
		return nil
	}

	// text table
	var runWrap struct {
		Items []runningListItem `json:"items"`
	}
	_ = json.Unmarshal(runJSON, &runWrap)
	fmt.Println("━━ Running ━━")
	fmt.Println("workflow\tphase\tsubmitted-from")
	for _, it := range runWrap.Items {
		src := it.SubmittedFrom
		if src == "" {
			src = "—"
		}
		fmt.Printf("%s\t%s\t%s\n", it.Name, it.Phase, src)
	}
	if runningOnly {
		return nil
	}
	var histWrap struct {
		Items []historyListItem `json:"items"`
	}
	_ = json.Unmarshal(histJSON, &histWrap)
	fmt.Println("━━ History ━━")
	fmt.Println("workflow\tphase\tsubmitted-from")
	for _, it := range histWrap.Items {
		src := it.SubmittedFrom
		if src == "" {
			src = "—"
		}
		fmt.Printf("%s\t%s\t%s\n", it.Name, it.Phase, src)
	}
	return nil
}

// cmdWorkflowRunning prints running workflows (compat).
func cmdWorkflowRunning(ctx context.Context, c *Client, limit int) error {
	return cmdWorkflowList(ctx, c, true, limit, "", "", "json")
}

// cmdWorkflowHistory prints history (compat).
func cmdWorkflowHistory(ctx context.Context, c *Client, limit int, continueToken, phase string) error {
	if limit <= 0 {
		limit = 50
	}
	q := url.Values{}
	q.Set("limit", fmt.Sprintf("%d", limit))
	if continueToken != "" {
		q.Set("continue", continueToken)
	}
	if phase != "" {
		q.Set("phase", phase)
	}
	path := "/v1/workflows/history?" + q.Encode()
	b, err := c.GetJSON(ctx, path)
	if err != nil {
		return err
	}
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, b, "", "  "); err != nil {
		fmt.Println(string(b))
	} else {
		fmt.Println(pretty.String())
	}
	return nil
}

func cmdWorkflowGet(ctx context.Context, c *Client, name string) error {
	return cmdWorkflowShow(ctx, c, name, "json")
}

func formatIECBytes(n int64) string {
	switch {
	case n < 0:
		return "?"
	case n < 1024:
		return fmt.Sprintf("%d B", n)
	}
	x := float64(n)
	u := 0
	for x >= 1024 && u < 3 {
		x /= 1024
		u++
	}
	suf := []string{"KiB", "MiB", "GiB"}[u]
	if x >= 100 {
		return fmt.Sprintf("%.0f %s", x, suf)
	}
	return fmt.Sprintf("%.2f %s", x, suf)
}

func shortenPath(s string, max int) string {
	if len(s) <= max {
		return s
	}
	if max < 12 {
		return s[:max]
	}
	head := max/2 - 2
	tail := max - head - 3
	return s[:head] + "..." + s[len(s)-tail:]
}

type countingWriter struct {
	w io.Writer
	n *int64
}

func (c *countingWriter) Write(p []byte) (int, error) {
	nn, err := c.w.Write(p)
	if nn > 0 {
		atomic.AddInt64(c.n, int64(nn))
	}
	return nn, err
}

// copyDownloadWithProgress copies r to w while printing a live progress line to progOut (typically stderr).
func copyDownloadWithProgress(r io.Reader, w io.Writer, total int64, destPath string, progOut *os.File) (written int64, err error) {
	var n int64
	cw := &countingWriter{w: w, n: &n}
	fancy := term.IsTerminal(int(progOut.Fd()))
	barW := 28
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	start := time.Now()
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stop:
				return
			case <-ticker.C:
				cur := atomic.LoadInt64(&n)
				elapsed := time.Since(start)
				label := shortenPath(destPath, 56)
				if !fancy {
					var etaStr string
					if total > 0 && cur > 0 && elapsed >= time.Millisecond {
						rate := float64(cur) / elapsed.Seconds()
						if rate >= 1 {
							remSec := float64(total-cur) / rate
							if remSec > 0 {
								etaStr = fmt.Sprintf(" ETA~%s", time.Duration(remSec*float64(time.Second)).Round(time.Second))
							}
						}
					}
					sizePart := formatIECBytes(cur)
					if total > 0 {
						sizePart = formatIECBytes(cur) + " / " + formatIECBytes(total)
					} else {
						sizePart += " (unknown total)"
					}
					fmt.Fprintf(progOut, "%s  %s  elapsed %s%s\n", label, sizePart, elapsed.Round(time.Second), etaStr)
					continue
				}
				var etaStr string
				if total > 0 && cur > 0 && elapsed >= time.Millisecond {
					rate := float64(cur) / elapsed.Seconds()
					if rate >= 1 {
						remSec := float64(total-cur) / rate
						if remSec > 0 {
							etaStr = fmt.Sprintf("  ETA ~%s", time.Duration(remSec*float64(time.Second)).Round(time.Second))
						}
					}
				}
				pct := 0.0
				filled := 0
				if total > 0 {
					pct = 100 * float64(cur) / float64(total)
					if pct > 100 {
						pct = 100
					}
					filled = int(pct / 100 * float64(barW))
				}
				bar := strings.Repeat("#", filled) + strings.Repeat("-", barW-filled)
				sizePart := formatIECBytes(cur)
				if total > 0 {
					sizePart = formatIECBytes(cur) + " / " + formatIECBytes(total)
				} else {
					sizePart += " (unknown total)"
				}
				fmt.Fprintf(progOut, "\r%s  [%s] %5.1f%%  %s  elapsed %s%s",
					label, bar, pct, sizePart, elapsed.Round(time.Second), etaStr)
			}
		}
	}()
	_, err = io.Copy(cw, r)
	close(stop)
	wg.Wait()
	nn := atomic.LoadInt64(&n)
	if fancy {
		fmt.Fprintf(progOut, "\r%s\r", strings.Repeat(" ", 120))
	}
	if err != nil {
		fmt.Fprintf(progOut, "download failed after %s: %v (%s transferred)\n",
			time.Since(start).Round(time.Second), err, formatIECBytes(nn))
		return nn, err
	}
	elapsed := time.Since(start)
	if total >= 0 {
		fmt.Fprintf(progOut, "download complete: %s (%s) in %s -> %s\n",
			formatIECBytes(nn), formatIECBytes(total), elapsed.Round(time.Second), destPath)
	} else {
		fmt.Fprintf(progOut, "download complete: %s in %s -> %s\n",
			formatIECBytes(nn), elapsed.Round(time.Second), destPath)
	}
	return nn, err
}

func cmdWorkflowDownloadArtifact(ctx context.Context, c *Client, wfName, artName string, genURL bool, durationSec int, templateName, nodeID, outFile string, toStdout bool, outFormat string, quiet bool) error {
	if genURL && (toStdout || outFile != "") {
		return fmt.Errorf("--generate-signed-url cannot be combined with -o or -stdout")
	}
	if toStdout && outFile != "" {
		return fmt.Errorf("-stdout and -o are mutually exclusive")
	}

	apiPath := "/v1/workflows/" + url.PathEscape(wfName) + "/downloadArtifact/" + url.PathEscape(artName)
	q := url.Values{}
	if templateName != "" {
		q.Set("templateName", templateName)
	}
	if nodeID != "" {
		q.Set("nodeId", nodeID)
	}
	if durationSec > 0 {
		q.Set("durationSeconds", strconv.Itoa(durationSec))
	}
	if enc := q.Encode(); enc != "" {
		apiPath += "?" + enc
	}

	raw, code, err := c.GetJSONRaw(ctx, apiPath)
	if err != nil {
		return err
	}
	if code == http.StatusConflict && nodeID == "" {
		// Multiple nodes produced an artifact with this name. Auto-select the last
		// candidate (execution order) and retry. Use --node-id to override.
		var conflict struct {
			Candidates []struct {
				NodeID      string `json:"nodeId"`
				DisplayName string `json:"displayName"`
			} `json:"candidates"`
		}
		if jsonErr := json.Unmarshal(raw, &conflict); jsonErr == nil && len(conflict.Candidates) > 0 {
			last := conflict.Candidates[len(conflict.Candidates)-1]
			fmt.Fprintf(os.Stderr, "note: ambiguous artifact %q — auto-selected node %s (%s); use --node-id to override\n",
				artName, last.NodeID, last.DisplayName)
			q.Set("nodeId", last.NodeID)
			apiPath = "/v1/workflows/" + url.PathEscape(wfName) + "/downloadArtifact/" + url.PathEscape(artName) + "?" + q.Encode()
			raw, code, err = c.GetJSONRaw(ctx, apiPath)
			if err != nil {
				return err
			}
		}
	}
	if code < 200 || code > 299 {
		return fmt.Errorf("workflow: GET %s: HTTP %d %s", apiPath, code, truncate(string(raw), 400))
	}
	var meta struct {
		URL       string `json:"url"`
		ExpiresAt string `json:"expiresAt"`
		FileName  string `json:"fileName"`
	}
	if err := json.Unmarshal(raw, &meta); err != nil {
		return fmt.Errorf("decode downloadArtifact JSON: %w", err)
	}
	if meta.URL == "" {
		return fmt.Errorf("empty url in API response")
	}

	if genURL {
		switch outFormat {
		case "json":
			var buf bytes.Buffer
			if err := json.Indent(&buf, raw, "", "  "); err != nil {
				return err
			}
			fmt.Println(buf.String())
		default:
			fmt.Printf("signed-url: %s\n", meta.URL)
		}
		return nil
	}

	resp, err := GetURLPlain(ctx, meta.URL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode > 299 {
		slurp, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("GET signed URL: HTTP %d %s", resp.StatusCode, truncate(string(slurp), 400))
	}

	outName := outFile
	if outName == "" {
		outName = strings.TrimSpace(meta.FileName)
		if outName == "" {
			outName = filepath.Base(artName)
			if outName == "" || outName == "." {
				outName = "artifact"
			}
		}
	}

	if toStdout {
		if quiet {
			_, err = io.Copy(os.Stdout, resp.Body)
			return err
		}
		_, err = copyDownloadWithProgress(resp.Body, os.Stdout, resp.ContentLength, "(stdout)", os.Stderr)
		return err
	}

	f, err := os.Create(outName)
	if err != nil {
		return err
	}
	defer f.Close()
	absDest, err := filepath.Abs(outName)
	if err != nil {
		return err
	}
	if quiet {
		_, err = io.Copy(f, resp.Body)
		return err
	}
	_, err = copyDownloadWithProgress(resp.Body, f, resp.ContentLength, absDest, os.Stderr)
	return err
}
