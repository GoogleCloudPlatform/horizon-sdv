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

// Package dispatch starts mirror runs through the Argo Events webhook, the same entry Horizon API submit uses.
package dispatch

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	smv1 "storage-mirror-module/api/v1alpha1"
)

const (
	TemplateCreate     = "storage-mirror-create"
	TemplateSync       = "storage-mirror-sync"
	TemplateDelete     = "storage-mirror-delete"
	TemplateVolumeInit = "storage-mirror-volume-init"

	SubmittedFromAPI = "api"
	// SchedulerUser is recorded as submitter for schedule ticks.
	SchedulerUser = "storage-mirror-scheduler"
	// ControllerUser is recorded for runs the controller starts on its own (volume init, purge).
	ControllerUser = "storage-mirror-module"
)

// MirrorParams is the parameter order shared by storage-mirror-create and storage-mirror-sync.
// The Sensors map body fields by index, so new names are only ever appended.
var MirrorParams = []string{
	"horizonSubmittedFrom", "requestId", "mirror", "directory", "kind",
	"manifestUrl", "manifestRef", "manifestFile", "syncJobs",
	"gitUrl", "gitBranches", "gitCredentialsSecret",
	"lfs", "syncTimeout", "pruneRefs", "excludeRefs", "gc", "submittedBy",
}

// DeleteParams is the parameter order of storage-mirror-delete.
var DeleteParams = []string{"horizonSubmittedFrom", "requestId", "mirror", "directory", "confirm"}

// VolumeInitParams is the parameter order of storage-mirror-volume-init.
var VolumeInitParams = []string{"horizonSubmittedFrom", "requestId"}

// ParamsFor returns the parameter list of a template.
func ParamsFor(template string) []string {
	switch template {
	case TemplateCreate, TemplateSync:
		return MirrorParams
	case TemplateDelete:
		return DeleteParams
	case TemplateVolumeInit:
		return VolumeInitParams
	}
	return nil
}

// Request is one run to dispatch.
type Request struct {
	Template      string
	SubmittedBy   string
	SubmittedFrom string
	Params        map[string]string
}

// Dispatcher posts workflow-dispatch events.
type Dispatcher struct {
	WebhookURL        string
	WorkflowNamespace string
	HTTP              *http.Client
}

// NewRequestID returns a random request id (hex, label-safe).
func NewRequestID() string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// Body builds the webhook body: the Horizon submit contract plus one field per template parameter.
func (d *Dispatcher) Body(r Request) (map[string]interface{}, error) {
	names := ParamsFor(r.Template)
	if names == nil {
		return nil, fmt.Errorf("unknown template %q", r.Template)
	}
	from := r.SubmittedFrom
	if from == "" {
		from = SubmittedFromAPI
	}
	body := map[string]interface{}{
		"workflowTemplateName":      r.Template,
		"workflowTemplateNamespace": d.WorkflowNamespace,
		"horizonModule":             smv1.ModuleName,
		"horizonSubmittedBy":        r.SubmittedBy,
		"horizonSubmittedFrom":      from,
	}
	for _, n := range names {
		if n == "horizonSubmittedFrom" {
			continue
		}
		v := r.Params[n]
		if strings.ContainsAny(v, "\n\r") {
			return nil, fmt.Errorf("parameter %s must be a single line", n)
		}
		body[n] = v
	}
	if body["requestId"] == "" {
		return nil, fmt.Errorf("requestId is required")
	}
	return body, nil
}

// Dispatch posts the request and returns its request id.
func (d *Dispatcher) Dispatch(ctx context.Context, r Request) (string, error) {
	if d.WebhookURL == "" {
		return "", fmt.Errorf("events webhook URL not configured")
	}
	if r.Params == nil {
		r.Params = map[string]string{}
	}
	if r.Params["requestId"] == "" {
		r.Params["requestId"] = NewRequestID()
	}
	body, err := d.Body(r)
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.WebhookURL, bytes.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	client := d.HTTP
	if client == nil {
		client = &http.Client{Timeout: 25 * time.Second}
	}
	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(res.Body, 512))
		return "", fmt.Errorf("events webhook: HTTP %d: %s", res.StatusCode, strings.TrimSpace(string(snippet)))
	}
	_, _ = io.Copy(io.Discard, res.Body)
	return r.Params["requestId"], nil
}

// ValidateCreate checks that merged create parameters name an upstream; the agent cannot fall back to a catalog
// entry for a new mirror.
func ValidateCreate(p map[string]string) error {
	if p["kind"] == "git" {
		if p["gitUrl"] == "" {
			return fmt.Errorf("git mirror needs git.url")
		}
		return nil
	}
	if p["manifestUrl"] == "" || p["manifestRef"] == "" {
		return fmt.Errorf("repo mirror needs manifestUrl and manifestRef (in the request or in StorageMirrorConfig defaults.repo)")
	}
	return nil
}

// MirrorRunParams merges a mirror with the config defaults into create/sync parameters.
func MirrorRunParams(m *smv1.StorageMirror, cfg *smv1.StorageMirrorConfigSpec) map[string]string {
	p := map[string]string{
		"mirror":    m.Name,
		"directory": m.DirectoryOrName(),
		"kind":      strings.ToLower(m.Spec.Kind),
		"gc":        cfg.Retention.GC,
		"pruneRefs": strings.Join(cfg.Retention.PruneRefs, ","),
	}
	lfs := m.Spec.LFS
	if lfs == "" || lfs == smv1.LFSInherit {
		lfs = cfg.Defaults.LFS
	}
	if lfs == "" {
		lfs = smv1.LFSDisabled
	}
	p["lfs"] = strings.ToLower(lfs)
	switch m.Spec.Kind {
	case smv1.KindGit:
		d := cfg.Defaults.Git
		if m.Spec.Git != nil {
			p["gitUrl"] = m.Spec.Git.URL
			branches := m.Spec.Git.Branches
			if len(branches) == 0 {
				branches = d.Branches
			}
			p["gitBranches"] = strings.Join(branches, ",")
			if m.Spec.Git.CredentialsSecretRef != nil {
				p["gitCredentialsSecret"] = m.Spec.Git.CredentialsSecretRef.Name
			}
		}
		p["excludeRefs"] = strings.Join(d.ExcludeRefs, ",")
		p["syncTimeout"] = firstNonEmpty(m.Spec.SyncTimeout, d.SyncTimeout, "6h")
	default:
		d := cfg.Defaults.Repo
		r := smv1.RepoSpec{}
		if m.Spec.Repo != nil {
			r = *m.Spec.Repo
		}
		p["manifestUrl"] = firstNonEmpty(r.ManifestURL, d.ManifestURL)
		p["manifestRef"] = firstNonEmpty(r.ManifestRef, d.ManifestRef)
		p["manifestFile"] = firstNonEmpty(r.ManifestFile, d.ManifestFile, "default.xml")
		jobs := r.SyncJobs
		if jobs == 0 {
			jobs = d.SyncJobs
		}
		if jobs == 0 {
			jobs = 3
		}
		p["syncJobs"] = strconv.Itoa(int(jobs))
		p["syncTimeout"] = firstNonEmpty(m.Spec.SyncTimeout, d.SyncTimeout, "20h")
	}
	return p
}

func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if s != "" {
			return s
		}
	}
	return ""
}
