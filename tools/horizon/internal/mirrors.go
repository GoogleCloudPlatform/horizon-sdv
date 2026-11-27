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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"text/tabwriter"
	"time"
)

const mirrorAPIBase = "/v1/storage-mirror"

// MirrorProgress mirrors the storage-mirror Progress object.
type MirrorProgress struct {
	Phase           string `json:"phase,omitempty"`
	State           string `json:"state,omitempty"`
	Attempt         int    `json:"attempt,omitempty"`
	MaxAttempts     int    `json:"maxAttempts,omitempty"`
	ProjectsDone    int64  `json:"projectsDone,omitempty"`
	ProjectsTotal   int64  `json:"projectsTotal,omitempty"`
	CurrentProject  string `json:"currentProject,omitempty"`
	RefsUpdated     int64  `json:"refsUpdated,omitempty"`
	ObjectsReceived int64  `json:"objectsReceived,omitempty"`
	ObjectsTotal    int64  `json:"objectsTotal,omitempty"`
	LFSObjectsDone  int64  `json:"lfsObjectsDone,omitempty"`
	LFSObjectsTotal int64  `json:"lfsObjectsTotal,omitempty"`
	PhasePercent    int    `json:"phasePercent,omitempty"`
	OverallPercent  int    `json:"overallPercent,omitempty"`
	ElapsedSeconds  int64  `json:"elapsedSeconds,omitempty"`
	EtaSeconds      int64  `json:"etaSeconds,omitempty"`
	Stale           bool   `json:"stale,omitempty"`
	Message         string `json:"message,omitempty"`
}

// MirrorRun is one run of a mirror.
type MirrorRun struct {
	WorkflowName string          `json:"workflowName,omitempty"`
	Template     string          `json:"template,omitempty"`
	RequestID    string          `json:"requestId,omitempty"`
	SubmittedBy  string          `json:"submittedBy,omitempty"`
	Phase        string          `json:"phase,omitempty"`
	StartedAt    string          `json:"startedAt,omitempty"`
	FinishedAt   string          `json:"finishedAt,omitempty"`
	Message      string          `json:"message,omitempty"`
	Progress     *MirrorProgress `json:"progress,omitempty"`
}

// MirrorStatus is GET .../mirrors/{name}/status.
type MirrorStatus struct {
	Mirror                 string     `json:"mirror"`
	Phase                  string     `json:"phase"`
	CatalogStatus          string     `json:"catalogStatus"`
	CurrentRun             *MirrorRun `json:"currentRun"`
	LastRun                *MirrorRun `json:"lastRun"`
	PendingRequestID       string     `json:"pendingRequestId"`
	LastSuccessfulSyncTime string     `json:"lastSuccessfulSyncTime"`
}

// MirrorItem is one entry of GET .../mirrors.
type MirrorItem struct {
	Name      string            `json:"name"`
	Directory string            `json:"directory"`
	Kind      string            `json:"kind"`
	Managed   bool              `json:"managed"`
	Status    *MirrorStatusCR   `json:"status,omitempty"`
	Catalog   map[string]string `json:"catalog,omitempty"`
}

// MirrorStatusCR is the subset of StorageMirror status shown in lists.
type MirrorStatusCR struct {
	Phase                  string     `json:"phase"`
	LastSuccessfulSyncTime string     `json:"lastSuccessfulSyncTime"`
	CurrentRun             *MirrorRun `json:"currentRun"`
}

// MirrorDo calls /v1/storage-mirror/<path> and returns the raw JSON.
func (c *Client) MirrorDo(ctx context.Context, method, path string, body interface{}) ([]byte, error) {
	var payload []byte
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		payload = b
	}
	return c.DoJSONWithHeaders(ctx, method, mirrorAPIBase+path, payload, map[string]string{"X-Horizon-Submitted-From": submittedFromCLIValue})
}

// MirrorPath escapes a mirror name into a path.
func MirrorPath(name, suffix string) string {
	return "/mirrors/" + url.PathEscape(name) + suffix
}

// CmdMirrorList prints the mirrors.
func CmdMirrorList(ctx context.Context, c *Client, kind, phase string, out io.Writer) error {
	q := url.Values{}
	if kind != "" {
		q.Set("kind", kind)
	}
	if phase != "" {
		q.Set("phase", phase)
	}
	p := "/mirrors"
	if len(q) > 0 {
		p += "?" + q.Encode()
	}
	b, err := c.MirrorDo(ctx, http.MethodGet, p, nil)
	if err != nil {
		return err
	}
	if IsJSON() {
		return printJSON(out, b)
	}
	var resp struct {
		Items []MirrorItem `json:"items"`
	}
	if err := json.Unmarshal(b, &resp); err != nil {
		return err
	}
	if len(resp.Items) == 0 {
		fmt.Fprintln(out, "No mirrors.")
		return nil
	}
	tw := tabwriter.NewWriter(out, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tKIND\tPHASE\tPROGRESS\tLAST SYNC\tDIRECTORY")
	for _, m := range resp.Items {
		phase, last, prog := "unmanaged", m.Catalog["last_successful_sync_time"], ""
		if m.Status != nil {
			phase, last = m.Status.Phase, m.Status.LastSuccessfulSyncTime
			if r := m.Status.CurrentRun; r != nil && r.Progress != nil {
				prog = fmt.Sprintf("%d%%", r.Progress.OverallPercent)
			}
		} else if s := m.Catalog["status"]; s != "" {
			phase = "unmanaged (" + s + ")"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", m.Name, m.Kind, phase, prog, last, m.Directory)
	}
	return tw.Flush()
}

// FormatProgress renders one status line (and a detail line) for a mirror run.
func FormatProgress(st MirrorStatus) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s  %s", st.Mirror, st.Phase)
	r := st.CurrentRun
	if r == nil {
		if st.PendingRequestID != "" {
			fmt.Fprintf(&b, "  requested (%s), waiting for the workflow", st.PendingRequestID)
		} else if st.LastRun != nil {
			fmt.Fprintf(&b, "  last run %s %s", st.LastRun.WorkflowName, st.LastRun.Phase)
		}
		return b.String()
	}
	p := r.Progress
	if p == nil {
		fmt.Fprintf(&b, "  run %s (no progress yet)", r.WorkflowName)
		return b.String()
	}
	if p.MaxAttempts > 0 {
		fmt.Fprintf(&b, "  attempt %d/%d", p.Attempt, p.MaxAttempts)
	}
	fmt.Fprintf(&b, "  %s", p.Phase)
	if p.ProjectsTotal > 0 {
		fmt.Fprintf(&b, "  %d/%d projects", p.ProjectsDone, p.ProjectsTotal)
	}
	if p.ObjectsTotal > 0 {
		fmt.Fprintf(&b, "  %d/%d objects", p.ObjectsReceived, p.ObjectsTotal)
	}
	if p.RefsUpdated > 0 {
		fmt.Fprintf(&b, "  %d refs", p.RefsUpdated)
	}
	fmt.Fprintf(&b, "  %s %d%%", bar(p.OverallPercent, 20), p.OverallPercent)
	fmt.Fprintf(&b, "  %s elapsed", hms(p.ElapsedSeconds))
	if p.EtaSeconds > 0 {
		fmt.Fprintf(&b, "  ~%s left", hms(p.EtaSeconds))
	}
	if p.State != "" && p.State != "running" {
		fmt.Fprintf(&b, "  [%s]", p.State)
	}
	if p.Stale {
		b.WriteString("  (stale)")
	}
	fmt.Fprintf(&b, "\n  run: %s", r.WorkflowName)
	if p.CurrentProject != "" {
		fmt.Fprintf(&b, "  current: %s", p.CurrentProject)
	}
	b.WriteString("  (horizon workflow logs " + r.WorkflowName + ")")
	return b.String()
}

func bar(pct, width int) string {
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	n := pct * width / 100
	return "[" + strings.Repeat("#", n) + strings.Repeat(".", width-n) + "]"
}

func hms(s int64) string {
	d := time.Duration(s) * time.Second
	return fmt.Sprintf("%02d:%02d:%02d", int(d.Hours()), int(d.Minutes())%60, int(d.Seconds())%60)
}

// MirrorStatusOnce fetches one status.
func MirrorStatusOnce(ctx context.Context, c *Client, name string) (MirrorStatus, []byte, error) {
	b, err := c.MirrorDo(ctx, http.MethodGet, MirrorPath(name, "/status"), nil)
	if err != nil {
		return MirrorStatus{}, nil, err
	}
	var st MirrorStatus
	if err := json.Unmarshal(b, &st); err != nil {
		return st, b, err
	}
	return st, b, nil
}

// CmdMirrorStatus prints status; with watch it polls until the mirror has no active or requested run.
// It returns an error when the run that was watched failed.
func CmdMirrorStatus(ctx context.Context, c *Client, name string, watch bool, interval time.Duration, out io.Writer) error {
	seenRun := false
	for {
		st, raw, err := MirrorStatusOnce(ctx, c, name)
		if err != nil {
			return err
		}
		if IsJSON() {
			if err := printJSON(out, raw); err != nil {
				return err
			}
		} else {
			fmt.Fprintln(out, FormatProgress(st))
		}
		active := st.CurrentRun != nil || st.PendingRequestID != ""
		if active {
			seenRun = true
		}
		if !watch || !active {
			if watch && seenRun && st.LastRun != nil && st.LastRun.Phase != "Succeeded" {
				return fmt.Errorf("run %s finished %s: %s", st.LastRun.WorkflowName, st.LastRun.Phase, st.LastRun.Message)
			}
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
}

// CmdMirrorPrint prints a JSON response (pretty) or a short text summary.
func CmdMirrorPrint(out io.Writer, raw []byte, text func(map[string]interface{}) string) error {
	if IsJSON() || text == nil {
		return printJSON(out, raw)
	}
	var m map[string]interface{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return printJSON(out, raw)
	}
	fmt.Fprintln(out, text(m))
	return nil
}

// CmdMirrorRuns prints run history.
func CmdMirrorRuns(ctx context.Context, c *Client, name string, out io.Writer) error {
	b, err := c.MirrorDo(ctx, http.MethodGet, MirrorPath(name, "/runs"), nil)
	if err != nil {
		return err
	}
	if IsJSON() {
		return printJSON(out, b)
	}
	var resp struct {
		Current *MirrorRun  `json:"current"`
		Items   []MirrorRun `json:"items"`
	}
	if err := json.Unmarshal(b, &resp); err != nil {
		return err
	}
	runs := resp.Items
	if resp.Current != nil {
		runs = append([]MirrorRun{*resp.Current}, runs...)
	}
	sort.SliceStable(runs, func(i, j int) bool { return runs[i].StartedAt > runs[j].StartedAt })
	tw := tabwriter.NewWriter(out, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "RUN\tTEMPLATE\tPHASE\tSTARTED\tFINISHED\tBY")
	for _, r := range runs {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n", r.WorkflowName, strings.TrimPrefix(r.Template, "storage-mirror-"), r.Phase, r.StartedAt, r.FinishedAt, r.SubmittedBy)
	}
	return tw.Flush()
}

func printJSON(out io.Writer, raw []byte) error {
	var v interface{}
	if err := json.Unmarshal(raw, &v); err != nil {
		_, err := out.Write(raw)
		return err
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, string(b))
	return err
}
