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

package api

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	smv1 "storage-mirror-module/api/v1alpha1"
	"storage-mirror-module/internal/catalog"
	"storage-mirror-module/internal/controller"
	"storage-mirror-module/internal/dispatch"
)

// MirrorView is one mirror in API responses.
type MirrorView struct {
	Name      string                    `json:"name"`
	Directory string                    `json:"directory"`
	Kind      string                    `json:"kind,omitempty"`
	Managed   bool                      `json:"managed"`
	Spec      *smv1.StorageMirrorSpec   `json:"spec,omitempty"`
	Status    *smv1.StorageMirrorStatus `json:"status,omitempty"`
	Catalog   catalog.Entry             `json:"catalog,omitempty"`
}

// RepoRequest is the repo part of a create body.
type RepoRequest struct {
	ManifestURL  string `json:"manifestUrl,omitempty"`
	ManifestRef  string `json:"manifestRef,omitempty"`
	ManifestFile string `json:"manifestFile,omitempty"`
	SyncJobs     int32  `json:"syncJobs,omitempty"`
}

// GitRequest is the git part of a create body.
type GitRequest struct {
	URL                   string   `json:"url"`
	Branches              []string `json:"branches,omitempty"`
	CredentialsSecretName string   `json:"credentialsSecretName,omitempty"`
}

// CreateRequest is the POST /api/v1/mirrors body.
type CreateRequest struct {
	Name           string       `json:"name"`
	Kind           string       `json:"kind"`
	Directory      string       `json:"directory,omitempty"`
	Repo           *RepoRequest `json:"repo,omitempty"`
	Git            *GitRequest  `json:"git,omitempty"`
	LFS            string       `json:"lfs,omitempty"`
	Schedule       string       `json:"schedule,omitempty"`
	SyncTimeout    string       `json:"syncTimeout,omitempty"`
	DeletionPolicy string       `json:"deletionPolicy,omitempty"`
}

// PatchRequest holds the mutable mirror fields.
type PatchRequest struct {
	ManifestRef  *string  `json:"manifestRef,omitempty"`
	ManifestFile *string  `json:"manifestFile,omitempty"`
	SyncJobs     *int32   `json:"syncJobs,omitempty"`
	Branches     []string `json:"branches,omitempty"`
	LFS          *string  `json:"lfs,omitempty"`
	Schedule     *string  `json:"schedule,omitempty"`
	Suspend      *bool    `json:"suspend,omitempty"`
	SyncTimeout  *string  `json:"syncTimeout,omitempty"`
	// Immutable fields are accepted only to reject them with 422.
	ManifestURL *string `json:"manifestUrl,omitempty"`
	URL         *string `json:"url,omitempty"`
	Kind        *string `json:"kind,omitempty"`
	Directory   *string `json:"directory,omitempty"`
}

func (s *Server) config(ctx context.Context) (*smv1.StorageMirrorConfig, error) {
	var cfg smv1.StorageMirrorConfig
	if err := s.live().Get(ctx, client.ObjectKey{Namespace: s.Namespace, Name: smv1.ConfigName}, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (s *Server) snapshot(ctx context.Context) (*catalog.Snapshot, error) {
	root := "horizon-mirrors"
	if cfg, err := s.config(ctx); err == nil {
		root = controller.RootKey(cfg)
	}
	return catalog.Load(ctx, s.reader(), s.Namespace, root)
}

func (s *Server) listMirrors(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var list smv1.StorageMirrorList
	if err := s.reader().List(ctx, &list, client.InNamespace(s.Namespace)); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	snap, err := s.snapshot(ctx)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	kind := strings.ToLower(r.URL.Query().Get("kind"))
	phase := r.URL.Query().Get("phase")
	seen := map[string]bool{}
	out := []MirrorView{}
	for i := range list.Items {
		m := &list.Items[i]
		seen[m.DirectoryOrName()] = true
		v := view(m, snap)
		if (kind == "" || strings.ToLower(v.Kind) == kind) && (phase == "" || strings.EqualFold(m.Status.Phase, phase)) {
			out = append(out, v)
		}
	}
	if phase == "" {
		for _, dir := range snap.Names() {
			if seen[dir] {
				continue
			}
			e := snap.Entries[dir]
			k := "repo"
			if e["type"] == "git" {
				k = "git"
			}
			if kind != "" && kind != k {
				continue
			}
			out = append(out, MirrorView{Name: dir, Directory: dir, Kind: k, Managed: false, Catalog: e})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": out, "catalogReportedAt": snap.ReportedAt})
}

func view(m *smv1.StorageMirror, snap *catalog.Snapshot) MirrorView {
	v := MirrorView{Name: m.Name, Directory: m.DirectoryOrName(), Kind: strings.ToLower(m.Spec.Kind), Managed: true, Spec: &m.Spec, Status: &m.Status}
	if snap != nil {
		v.Catalog = snap.Entries[m.DirectoryOrName()]
	}
	return v
}

func (s *Server) getMirror(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	name := r.PathValue("name")
	snap, err := s.snapshot(ctx)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	m, err := s.mirror(ctx, name)
	if apierrors.IsNotFound(err) {
		if e, ok := snap.Entries[name]; ok {
			writeJSON(w, http.StatusOK, MirrorView{Name: name, Directory: name, Managed: false, Catalog: e})
			return
		}
		writeErr(w, http.StatusNotFound, "mirror not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, view(m, snap))
}

func (s *Server) mirror(ctx context.Context, name string) (*smv1.StorageMirror, error) {
	var m smv1.StorageMirror
	if err := s.live().Get(ctx, client.ObjectKey{Namespace: s.Namespace, Name: name}, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// BuildMirror validates a create body and returns the CR to create.
func BuildMirror(req CreateRequest, namespace string) (*smv1.StorageMirror, error) {
	if !dnsLabel.MatchString(req.Name) {
		return nil, fmt.Errorf("name must be a DNS-1123 label of at most 63 characters")
	}
	m := &smv1.StorageMirror{
		ObjectMeta: metav1.ObjectMeta{Name: req.Name, Namespace: namespace, Labels: map[string]string{}, Annotations: map[string]string{}},
		Spec: smv1.StorageMirrorSpec{
			Directory:      req.Directory,
			LFS:            normaliseEnum(req.LFS, smv1.LFSInherit, smv1.LFSInherit, smv1.LFSDisabled, smv1.LFSEnabled, smv1.LFSAuto),
			Schedule:       normaliseSchedule(req.Schedule),
			SyncTimeout:    req.SyncTimeout,
			DeletionPolicy: normaliseEnum(req.DeletionPolicy, smv1.DeletionPolicyRetain, smv1.DeletionPolicyRetain, smv1.DeletionPolicyDelete),
		},
	}
	if m.Spec.LFS == "" || m.Spec.DeletionPolicy == "" {
		return nil, fmt.Errorf("invalid lfs or deletionPolicy")
	}
	switch strings.ToLower(req.Kind) {
	case "repo", "":
		if req.Git != nil {
			return nil, fmt.Errorf("git settings given for a repo mirror")
		}
		m.Spec.Kind = smv1.KindRepo
		rr := RepoRequest{}
		if req.Repo != nil {
			rr = *req.Repo
		}
		m.Spec.Repo = &smv1.RepoSpec{ManifestURL: rr.ManifestURL, ManifestRef: rr.ManifestRef, ManifestFile: rr.ManifestFile, SyncJobs: rr.SyncJobs}
	case "git":
		if req.Git == nil || !strings.HasPrefix(req.Git.URL, "https://") {
			return nil, fmt.Errorf("git mirrors need git.url starting with https://")
		}
		if req.Repo != nil {
			return nil, fmt.Errorf("repo settings given for a git mirror")
		}
		m.Spec.Kind = smv1.KindGit
		m.Spec.Git = &smv1.GitSpec{URL: req.Git.URL, Branches: req.Git.Branches}
		if req.Git.CredentialsSecretName != "" {
			m.Spec.Git.CredentialsSecretRef = &smv1.SecretRef{Name: req.Git.CredentialsSecretName}
		}
	default:
		return nil, fmt.Errorf("kind must be repo or git")
	}
	if strings.ContainsAny(req.Directory, "/\\") || req.Directory == "metadata.yaml" || strings.HasPrefix(req.Directory, ".") {
		return nil, fmt.Errorf("invalid directory")
	}
	m.Labels[smv1.LabelKind] = strings.ToLower(m.Spec.Kind)
	return m, nil
}

func normaliseEnum(v, def string, allowed ...string) string {
	if v == "" {
		return def
	}
	for _, a := range allowed {
		if strings.EqualFold(v, a) {
			return a
		}
	}
	return ""
}

func normaliseSchedule(v string) string {
	switch strings.ToLower(v) {
	case "", "inherit":
		return smv1.ScheduleInherit
	case "none":
		return smv1.ScheduleNone
	}
	return v
}

func (s *Server) createMirror(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req CreateRequest
	if err := decode(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	m, err := BuildMirror(req, s.Namespace)
	if err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	cfg, err := s.config(ctx)
	if err != nil {
		writeK8sErr(w, err)
		return
	}
	if m.Spec.Kind == smv1.KindGit && cfg.Spec.Consumers.Jenkins.Enabled && !cfg.Spec.Consumers.Jenkins.ReadOnly {
		// Jenkins Sync All (before the type guard in sync_mirror.sh) treats every catalog entry as a repo mirror.
		writeErr(w, http.StatusConflict, "git mirrors need consumers.jenkins.readOnly=true (Jenkins mirror jobs would write to the volume)")
		return
	}
	snap, err := s.snapshot(ctx)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if _, ok := snap.Entries[m.DirectoryOrName()]; ok {
		writeErr(w, http.StatusConflict, "directory already exists in the catalog; use POST /api/v1/import to manage it")
		return
	}
	var list smv1.StorageMirrorList
	if err := s.live().List(ctx, &list, client.InNamespace(s.Namespace)); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	for _, o := range list.Items {
		if o.DirectoryOrName() == m.DirectoryOrName() {
			writeErr(w, http.StatusConflict, "directory already used by mirror "+o.Name)
			return
		}
	}
	if err := dispatch.ValidateCreate(dispatch.MirrorRunParams(m, &cfg.Spec)); err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	id := dispatch.NewRequestID()
	m.Annotations[smv1.AnnotationCreatedBy] = user(r)
	m.Annotations[controller.AnnotationSyncRequestID] = id
	m.Annotations[controller.AnnotationSubmittedFrom] = submittedFrom(r)
	if err := s.Client.Create(ctx, m); err != nil {
		if apierrors.IsAlreadyExists(err) {
			writeErr(w, http.StatusConflict, "mirror already exists")
			return
		}
		if apierrors.IsInvalid(err) {
			writeErr(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.wakeMirror(m.Name)
	writeJSON(w, http.StatusCreated, map[string]interface{}{"name": m.Name, "requestId": id, "mirror": view(m, nil)})
}

func (s *Server) patchMirror(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req PatchRequest
	if err := decode(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.ManifestURL != nil || req.URL != nil || req.Kind != nil || req.Directory != nil {
		writeErr(w, http.StatusUnprocessableEntity, "kind, directory, manifestUrl and url are immutable; create a new mirror instead")
		return
	}
	m, err := s.mirror(ctx, r.PathValue("name"))
	if err != nil {
		writeK8sErr(w, err)
		return
	}
	patch := client.MergeFrom(m.DeepCopy())
	if err := ApplyPatch(m, req); err != nil {
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	if err := s.Client.Patch(ctx, m, patch); err != nil {
		writeK8sErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, view(m, nil))
}

// ApplyPatch applies the mutable fields to the mirror.
func ApplyPatch(m *smv1.StorageMirror, req PatchRequest) error {
	if req.ManifestRef != nil || req.ManifestFile != nil || req.SyncJobs != nil {
		if m.Spec.Kind != smv1.KindRepo {
			return fmt.Errorf("manifestRef, manifestFile and syncJobs apply to repo mirrors only")
		}
		if m.Spec.Repo == nil {
			m.Spec.Repo = &smv1.RepoSpec{}
		}
		if req.ManifestRef != nil {
			m.Spec.Repo.ManifestRef = *req.ManifestRef
		}
		if req.ManifestFile != nil {
			m.Spec.Repo.ManifestFile = *req.ManifestFile
		}
		if req.SyncJobs != nil {
			if *req.SyncJobs < 1 {
				return fmt.Errorf("syncJobs must be at least 1")
			}
			m.Spec.Repo.SyncJobs = *req.SyncJobs
		}
	}
	if req.Branches != nil {
		if m.Spec.Kind != smv1.KindGit || m.Spec.Git == nil {
			return fmt.Errorf("branches apply to git mirrors only")
		}
		m.Spec.Git.Branches = req.Branches
	}
	if req.LFS != nil {
		v := normaliseEnum(*req.LFS, smv1.LFSInherit, smv1.LFSInherit, smv1.LFSDisabled, smv1.LFSEnabled, smv1.LFSAuto)
		if v == "" {
			return fmt.Errorf("lfs must be inherit, disabled, enabled or auto")
		}
		m.Spec.LFS = v
	}
	if req.Schedule != nil {
		m.Spec.Schedule = normaliseSchedule(*req.Schedule)
	}
	if req.Suspend != nil {
		m.Spec.Suspend = *req.Suspend
	}
	if req.SyncTimeout != nil {
		m.Spec.SyncTimeout = *req.SyncTimeout
	}
	return nil
}

func (s *Server) syncMirror(w http.ResponseWriter, r *http.Request) {
	res, code := s.requestSync(r, r.PathValue("name"))
	writeJSON(w, code, res)
}

// SyncResult is the outcome of one sync request.
type SyncResult struct {
	Mirror    string `json:"mirror"`
	RequestID string `json:"requestId,omitempty"`
	RunID     string `json:"runId,omitempty"`
	Detail    string `json:"detail,omitempty"`
}

func (s *Server) requestSync(r *http.Request, name string) (SyncResult, int) {
	ctx := r.Context()
	m, err := s.mirror(ctx, name)
	if apierrors.IsNotFound(err) {
		return SyncResult{Mirror: name, Detail: "mirror not found or not managed (use POST /api/v1/import)"}, http.StatusNotFound
	}
	if err != nil {
		return SyncResult{Mirror: name, Detail: err.Error()}, http.StatusInternalServerError
	}
	if !m.DeletionTimestamp.IsZero() {
		return SyncResult{Mirror: name, Detail: "mirror is being deleted"}, http.StatusConflict
	}
	if m.Spec.Suspend {
		return SyncResult{Mirror: name, Detail: "mirror is suspended (PATCH suspend=false to resume)"}, http.StatusConflict
	}
	if m.Status.CurrentRun != nil {
		return SyncResult{Mirror: name, RunID: m.Status.CurrentRun.WorkflowName, Detail: "a run is already active"}, http.StatusAccepted
	}
	if m.Status.PendingRequestID != "" {
		return SyncResult{Mirror: name, RequestID: m.Status.PendingRequestID, Detail: "a run was already requested"}, http.StatusAccepted
	}
	id := dispatch.NewRequestID()
	if err := controller.RequestSync(ctx, s.Client, m, user(r), submittedFrom(r), id, s.now()); err != nil {
		return SyncResult{Mirror: name, Detail: err.Error()}, http.StatusInternalServerError
	}
	s.wakeMirror(name)
	return SyncResult{Mirror: name, RequestID: id}, http.StatusAccepted
}

func (s *Server) batchSync(w http.ResponseWriter, r *http.Request) {
	var req struct {
		All     bool     `json:"all"`
		Mirrors []string `json:"mirrors"`
	}
	if err := decode(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	names := req.Mirrors
	if req.All {
		var list smv1.StorageMirrorList
		if err := s.reader().List(r.Context(), &list, client.InNamespace(s.Namespace)); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		names = nil
		for _, m := range list.Items {
			if !m.Spec.Suspend {
				names = append(names, m.Name)
			}
		}
	}
	if len(names) == 0 {
		writeErr(w, http.StatusUnprocessableEntity, "no mirrors selected; set all or mirrors")
		return
	}
	results := make([]SyncResult, 0, len(names))
	for _, n := range names {
		res, _ := s.requestSync(r, n)
		results = append(results, res)
	}
	writeJSON(w, http.StatusAccepted, map[string]interface{}{"results": results})
}

func (s *Server) mirrorStatus(w http.ResponseWriter, r *http.Request) {
	m, err := s.mirror(r.Context(), r.PathValue("name"))
	if err != nil {
		writeK8sErr(w, err)
		return
	}
	st := m.Status.DeepCopy()
	if st.CurrentRun != nil && s.Progress != nil {
		if rep, ok := s.Progress.Get(st.CurrentRun.WorkflowName); ok {
			p := rep.Progress
			t := metav1.NewTime(rep.ReceivedAt)
			p.UpdatedAt = &t
			st.CurrentRun.Progress = &p
		}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"mirror": m.Name, "phase": st.Phase, "catalogStatus": st.CatalogStatus,
		"currentRun": st.CurrentRun, "lastRun": st.LastRun, "pendingRequestId": st.PendingRequestID,
		"lastSuccessfulSyncTime": st.LastSuccessfulSyncTime, "conditions": st.Conditions,
	})
}

func (s *Server) mirrorRuns(w http.ResponseWriter, r *http.Request) {
	m, err := s.mirror(r.Context(), r.PathValue("name"))
	if err != nil {
		writeK8sErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"mirror": m.Name, "current": m.Status.CurrentRun, "items": m.Status.History})
}

func (s *Server) cancelSync(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	m, err := s.mirror(ctx, r.PathValue("name"))
	if err != nil {
		writeK8sErr(w, err)
		return
	}
	if m.Status.CurrentRun == nil {
		writeErr(w, http.StatusConflict, "no active run")
		return
	}
	wf := workflowObject()
	if err := s.live().Get(ctx, client.ObjectKey{Namespace: s.WorkflowNamespace, Name: m.Status.CurrentRun.WorkflowName}, wf); err != nil {
		writeK8sErr(w, err)
		return
	}
	if wf.GetLabels()[smv1.LabelMirror] != m.Name {
		writeErr(w, http.StatusConflict, "active run does not belong to this mirror")
		return
	}
	patch := client.RawPatch("application/merge-patch+json", []byte(`{"spec":{"shutdown":"Stop"}}`))
	if err := s.Client.Patch(ctx, wf, patch); err != nil {
		writeK8sErr(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"mirror": m.Name, "runId": wf.GetName(), "status": "stopping"})
}

func (s *Server) deleteMirror(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	name := r.PathValue("name")
	m, err := s.mirror(ctx, name)
	if err != nil {
		writeK8sErr(w, err)
		return
	}
	purge := r.URL.Query().Get("purge") == "true"
	policy := smv1.DeletionPolicyRetain
	if purge {
		if c := r.URL.Query().Get("confirm"); c != m.Name && c != m.DirectoryOrName() {
			writeErr(w, http.StatusBadRequest, "purge deletes the mirror data; pass confirm=<mirror name>")
			return
		}
		policy = smv1.DeletionPolicyDelete
	}
	patch := client.MergeFrom(m.DeepCopy())
	m.Spec.DeletionPolicy = policy
	if m.Annotations == nil {
		m.Annotations = map[string]string{}
	}
	m.Annotations[smv1.AnnotationDeleteRequestedBy] = user(r)
	if err := s.Client.Patch(ctx, m, patch); err != nil {
		writeK8sErr(w, err)
		return
	}
	if err := s.Client.Delete(ctx, m); err != nil && !apierrors.IsNotFound(err) {
		writeK8sErr(w, err)
		return
	}
	s.wakeMirror(name)
	writeJSON(w, http.StatusAccepted, map[string]interface{}{"mirror": name, "purge": purge, "deletionPolicy": policy})
}

func (s *Server) importMirrors(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Mirrors []string `json:"mirrors"`
	}
	if err := decode(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	snap, err := s.snapshot(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	created, err := controller.Import(r.Context(), s.Client, s.Namespace, snap, req.Mirrors, "api:"+user(r))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"imported": created, "catalogReportedAt": snap.ReportedAt})
}

func writeK8sErr(w http.ResponseWriter, err error) {
	switch {
	case apierrors.IsNotFound(err):
		writeErr(w, http.StatusNotFound, "not found")
	case apierrors.IsInvalid(err):
		writeErr(w, http.StatusUnprocessableEntity, err.Error())
	case apierrors.IsConflict(err):
		writeErr(w, http.StatusConflict, err.Error())
	default:
		writeErr(w, http.StatusInternalServerError, err.Error())
	}
}
