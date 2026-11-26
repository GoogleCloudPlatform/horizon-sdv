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
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/acn-horizon-sdv/horizon-api/internal/workflow"
	"github.com/acn-horizon-sdv/horizonauth"
	"github.com/acn-horizon-sdv/horizonauth/authtest"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/rest"
)

// Subjects of the test callers. The developer is the authtest default user: dev-alice with
// subject testUserSubject.
const (
	viewerSubject = "3a1c5e7f-9b2d-4f6a-8c0e-1d3f5a7b9c2e"
	bobSubject    = "0f6a4c1e-2b7d-4e5f-9a3b-7c8d9e0f1a2b"
	adminSubject  = "9d3e5b7a-1c2f-4a6b-8d0e-2f4a6c8e0b1d"
)

// Token claims of one caller per role. Keycloak makes admin a composite of developer, so an
// admin token lists both client roles.
var (
	viewerClaims    = map[string]any{"sub": viewerSubject, "preferred_username": "vera"}
	developerClaims = map[string]any{"resource_access": authtest.Roles("developer")}
	adminClaims     = map[string]any{"sub": adminSubject, "preferred_username": "anna", "resource_access": authtest.Roles("admin", "developer")}
)

// authFixture is a fake Keycloak realm and a verifier for it, shared by the servers of one test.
type authFixture struct {
	iss      *authtest.Issuer
	verifier *horizonauth.Verifier
}

func newAuthFixture(t *testing.T) authFixture {
	t.Helper()
	iss := authtest.NewIssuer(t)
	v, err := horizonauth.NewVerifier(context.Background(), iss.URL(), "horizon-api")
	if err != nil {
		t.Fatal(err)
	}
	return authFixture{iss: iss, verifier: v}
}

// testWorkflow describes a Workflow in the fake cluster.
type testWorkflow struct {
	name string
	// owner is the horizon-sdv.io/submitted-by annotation; empty leaves it out.
	owner string
	// startedBy is the horizon-sdv.io/submitted-by-name annotation; empty leaves it out.
	startedBy string
	// phase defaults to Running.
	phase string
	// hidden leaves out the submitted-from label, so Horizon API does not serve the workflow.
	hidden bool
}

func (w testWorkflow) object() *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetAPIVersion("argoproj.io/v1alpha1")
	u.SetKind("Workflow")
	u.SetNamespace("workflows")
	u.SetName(w.name)
	if !w.hidden {
		u.SetLabels(map[string]string{workflow.LabelSubmittedFrom: workflow.SubmittedFromDeveloperPortal})
	}
	annotations := map[string]string{}
	if w.owner != "" {
		annotations[workflow.LabelSubmittedBy] = w.owner
	}
	if w.startedBy != "" {
		annotations[workflow.AnnotationSubmittedByName] = w.startedBy
	}
	if len(annotations) > 0 {
		u.SetAnnotations(annotations)
	}
	phase := w.phase
	if phase == "" {
		phase = "Running"
	}
	_ = unstructured.SetNestedField(u.Object, phase, "status", "phase")
	return u
}

// roleEnv is a Horizon API server on a fake cluster and a fake Argo Events webhook.
type roleEnv struct {
	t       *testing.T
	auth    authFixture
	srv     *Server
	cluster *dynamicfake.FakeDynamicClient
	// submits counts the workflows dispatched to the webhook.
	submits atomic.Int32
}

// newRoleEnv starts a server with the given role order and role-check setting. The catalog has
// the enabled module "sample" with the templates "launcher" and "smoke".
func newRoleEnv(t *testing.T, auth authFixture, roleOrder string, roleChecks bool, workflows ...testWorkflow) *roleEnv {
	t.Helper()
	e := &roleEnv{t: t, auth: auth}
	objects := make([]runtime.Object, 0, len(workflows))
	for _, w := range workflows {
		objects = append(objects, w.object())
	}
	e.cluster = dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{
		workflow.GVR: "WorkflowList",
	}, objects...)
	webhook := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		e.submits.Add(1)
	}))
	t.Cleanup(webhook.Close)
	order, err := horizonauth.ParseRoleOrder(roleOrder)
	if err != nil {
		t.Fatal(err)
	}
	store, err := workflow.NewStore(serveWorkflows(t, e.cluster), "workflows")
	if err != nil {
		t.Fatal(err)
	}
	e.srv = New(Options{
		Catalog:          submitCatalog(t),
		Verifier:         auth.verifier,
		Authorizer:       horizonauth.NewAuthorizer(order, roleChecks),
		WorkflowsNS:      "workflows",
		EventsWebhookURL: webhook.URL,
		WorkflowStore:    store,
	})
	return e
}

// serveWorkflows starts a fake Kubernetes API server for the calls workflow.Store makes in the
// "workflows" namespace, and returns its client config. Workflows are read from and written to
// cluster. The pod list is empty, and any other resource does not exist.
func serveWorkflows(t *testing.T, cluster *dynamicfake.FakeDynamicClient) *rest.Config {
	t.Helper()
	const (
		workflowsPath = "/apis/argoproj.io/v1alpha1/namespaces/workflows/workflows"
		podsPath      = "/api/v1/namespaces/workflows/pods"
	)
	workflows := cluster.Resource(workflow.GVR).Namespace("workflows")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		name, isWorkflow := strings.CutPrefix(r.URL.Path, workflowsPath+"/")
		var obj any
		var err error
		switch {
		case r.URL.Path == podsPath && r.Method == http.MethodGet:
			obj = map[string]any{"kind": "PodList", "apiVersion": "v1", "items": []any{}}
		case r.URL.Path == workflowsPath && r.Method == http.MethodGet:
			obj, err = workflows.List(ctx, metav1.ListOptions{})
		case isWorkflow && r.Method == http.MethodGet:
			obj, err = workflows.Get(ctx, name, metav1.GetOptions{})
		case isWorkflow && r.Method == http.MethodPatch:
			patch, readErr := io.ReadAll(r.Body)
			if readErr != nil {
				t.Errorf("read patch: %v", readErr)
			}
			obj, err = workflows.Patch(ctx, name, types.PatchType(r.Header.Get("Content-Type")), patch, metav1.PatchOptions{})
		case isWorkflow && r.Method == http.MethodDelete:
			err = workflows.Delete(ctx, name, metav1.DeleteOptions{})
			obj = metav1.Status{TypeMeta: metav1.TypeMeta{Kind: "Status", APIVersion: "v1"}, Status: metav1.StatusSuccess}
		default:
			err = apierrors.NewNotFound(schema.GroupResource{}, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		if status, ok := err.(apierrors.APIStatus); ok {
			s := status.Status()
			s.TypeMeta = metav1.TypeMeta{Kind: "Status", APIVersion: "v1"}
			w.WriteHeader(int(s.Code))
			obj = s
		} else if err != nil {
			t.Errorf("%s %s: %v", r.Method, r.URL.Path, err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(obj)
	}))
	t.Cleanup(srv.Close)
	return &rest.Config{Host: srv.URL}
}

// call sends a request as the caller with the given token claims. Nil claims send no token.
func (e *roleEnv) call(method, path, body string, claims map[string]any) *httptest.ResponseRecorder {
	e.t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, reader)
	if claims != nil {
		req.Header.Set("Authorization", "Bearer "+e.auth.iss.Token(e.t, e.auth.iss.Claims(claims)))
	}
	rec := httptest.NewRecorder()
	e.srv.srv.Handler.ServeHTTP(rec, req)
	return rec
}

// stored returns the workflow as it is in the fake cluster, or nil when it does not exist.
func (e *roleEnv) stored(name string) *unstructured.Unstructured {
	e.t.Helper()
	u, err := e.cluster.Resource(workflow.GVR).Namespace("workflows").Get(context.Background(), name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		e.t.Fatal(err)
	}
	return u
}

// shutdownRequested reports whether an abort reached the workflow.
func (e *roleEnv) shutdownRequested(name string) bool {
	e.t.Helper()
	u := e.stored(name)
	if u == nil {
		e.t.Fatalf("workflow %s does not exist", name)
	}
	shutdown, _, _ := unstructured.NestedString(u.Object, "spec", "shutdown")
	return shutdown == "Stop"
}

// wantForbidden checks that rec is the shared 403 body and returns it.
func wantForbidden(t *testing.T, rec *httptest.ResponseRecorder, requiredRole, currentRole string) horizonauth.Forbidden {
	t.Helper()
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d %s, want 403", rec.Code, rec.Body)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("content type = %q, want application/json", got)
	}
	var f horizonauth.Forbidden
	dec := json.NewDecoder(rec.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&f); err != nil {
		t.Fatalf("403 body: %v", err)
	}
	if f.Code != "forbidden" || f.RequiredRole != requiredRole || f.CurrentRole != currentRole || f.Reason == "" {
		t.Errorf("403 body = %+v, want error forbidden, requiredRole %q, currentRole %q and a reason", f, requiredRole, currentRole)
	}
	return f
}

func TestPermissionMatrix(t *testing.T) {
	auth := newAuthFixture(t)
	callers := []struct {
		role   string
		claims map[string]any
	}{
		{"viewer", viewerClaims},
		{"developer", developerClaims},
		{"admin", adminClaims},
	}
	// The workflows belong to dev-alice, the developer, except where the name says otherwise.
	workflows := []testWorkflow{
		{name: "alice-running", owner: testUserSubject, startedBy: "dev-alice"},
		{name: "alice-finished", owner: testUserSubject, startedBy: "dev-alice", phase: "Succeeded"},
		{name: "bob-running", owner: bobSubject, startedBy: "dev-bob"},
		{name: "ownerless-running"},
	}
	// One row per kind of call, and at least one for every route that needs a token. status is
	// the answer of the handler when the call is allowed.
	// viewer, developer and admin hold what happens with role checks on: allowed, or the role
	// that the 403 names as required.
	const allowed = ""
	rows := []struct {
		name                     string
		method, path, body       string
		status                   int
		viewer, developer, admin string
	}{
		{"describe the caller", "GET", "/v1/me", "", 200, allowed, allowed, allowed},
		{"read the catalog", "GET", "/v1/catalog", "", 200, allowed, allowed, allowed},
		{"list running workflows", "GET", "/v1/workflows/running", "", 200, allowed, allowed, allowed},
		{"list finished workflows", "GET", "/v1/workflows/history", "", 200, allowed, allowed, allowed},
		{"read a workflow", "GET", "/v1/workflows/alice-running", "", 200, allowed, allowed, allowed},
		// Log streaming, artifact signing and the storage module are not configured here. The
		// handlers answer 503 and 501 for that, which shows the call got past the role check.
		{"read workflow logs", "GET", "/v1/workflows/alice-running/log", "", 503, allowed, allowed, allowed},
		{"download a workflow artifact", "GET", "/v1/workflows/alice-running/downloadArtifact/main-logs", "", 501, allowed, allowed, allowed},
		{"list artifact buckets", "GET", "/v1/artifacts/buckets", "", 200, allowed, allowed, allowed},
		{"list artifact objects", "GET", "/v1/artifacts/buckets/some-bucket/objects", "", 503, allowed, allowed, allowed},

		{"submit a template", "POST", "/v1/modules/sample/workflowTemplates/smoke/submit", `{"parameters":{"note":"n"}}`, 202, "developer", allowed, allowed},
		{"abort own running workflow", "POST", "/v1/workflows/alice-running/abort", "", 202, "developer", allowed, allowed},
		{"abort another user's workflow", "POST", "/v1/workflows/bob-running/abort", "", 202, "developer", "admin", allowed},
		{"abort a workflow with no owner", "POST", "/v1/workflows/ownerless-running/abort", "", 202, "developer", "admin", allowed},
		{"delete a workflow record", "DELETE", "/v1/workflows/alice-finished", "", 200, "admin", "admin", allowed},
	}

	for _, roleChecks := range []bool{true, false} {
		for _, caller := range callers {
			name := caller.role + " with role checks on"
			if !roleChecks {
				name = caller.role + " with role checks off"
			}
			t.Run(name, func(t *testing.T) {
				e := newRoleEnv(t, auth, horizonauth.DefaultRoleOrder, roleChecks, workflows...)
				wantSubmits := int32(0)
				for _, row := range rows {
					required := map[string]string{"viewer": row.viewer, "developer": row.developer, "admin": row.admin}[caller.role]
					if !roleChecks {
						// With role checks off a signed-in user keeps the access they had before.
						required = allowed
					}
					rec := e.call(row.method, row.path, row.body, caller.claims)
					if required != allowed {
						t.Run(row.name+" is refused", func(t *testing.T) {
							wantForbidden(t, rec, required, caller.role)
						})
						continue
					}
					if rec.Code != row.status {
						t.Errorf("%s = %d %s, want %d", row.name, rec.Code, rec.Body, row.status)
					}
					if row.status == http.StatusAccepted && strings.HasSuffix(row.path, "/submit") {
						wantSubmits++
					}
				}
				if got := e.submits.Load(); got != wantSubmits {
					t.Errorf("%d workflows were dispatched, want %d", got, wantSubmits)
				}
			})
		}
	}

	t.Run("no token is 401 on every call, before any role check", func(t *testing.T) {
		e := newRoleEnv(t, auth, horizonauth.DefaultRoleOrder, true, workflows...)
		for _, row := range rows {
			if rec := e.call(row.method, row.path, row.body, nil); rec.Code != http.StatusUnauthorized {
				t.Errorf("%s = %d, want 401", row.name, rec.Code)
			}
		}
	})

	t.Run("public routes need no token", func(t *testing.T) {
		e := newRoleEnv(t, auth, horizonauth.DefaultRoleOrder, true, workflows...)
		for _, path := range []string{"/healthz", "/openapi.json", "/swagger"} {
			if rec := e.call("GET", path, "", nil); rec.Code != http.StatusOK {
				t.Errorf("GET %s = %d, want 200", path, rec.Code)
			}
		}
	})
}

func TestAbortNeedsOwnWorkflowOrAdmin(t *testing.T) {
	auth := newAuthFixture(t)
	ci := map[string]any{
		"sub": testCISubject, "azp": "horizon-api-ci", "preferred_username": testCIUsername,
		"resource_access": authtest.Roles("developer", "impersonate"),
	}
	// A user who started a run and then lost the developer role.
	demoted := map[string]any{}

	tests := []struct {
		name       string
		workflow   testWorkflow
		claims     map[string]any
		roleChecks bool
		// wantStatus 403 uses wantRequired, wantCurrent and wantReason.
		wantStatus   int
		wantRequired string
		wantCurrent  string
		wantReason   string
	}{
		{
			name:     "developer aborts own workflow",
			workflow: testWorkflow{owner: testUserSubject, startedBy: "dev-alice"},
			claims:   developerClaims, roleChecks: true,
			wantStatus: http.StatusAccepted,
		},
		{
			name:     "developer cannot abort another user's workflow",
			workflow: testWorkflow{owner: bobSubject, startedBy: "dev-bob"},
			claims:   developerClaims, roleChecks: true,
			wantStatus: http.StatusForbidden, wantRequired: "admin", wantCurrent: "developer",
			wantReason: "You can only abort workflows you started. This one was started by dev-bob. Admins can abort any workflow.",
		},
		{
			name:     "developer cannot abort a workflow with no owner",
			workflow: testWorkflow{},
			claims:   developerClaims, roleChecks: true,
			wantStatus: http.StatusForbidden, wantRequired: "admin", wantCurrent: "developer",
			wantReason: "You can only abort workflows you started. This one has no recorded owner. Admins can abort any workflow.",
		},
		{
			// Before the owner came from the token, the portal stored the username it sent. It
			// is not a token subject, so the run has no trusted owner, even for that user.
			name:     "developer cannot abort a run from before owners came from the token",
			workflow: testWorkflow{owner: "dev-alice"},
			claims:   developerClaims, roleChecks: true,
			wantStatus: http.StatusForbidden, wantRequired: "admin", wantCurrent: "developer",
			wantReason: "You can only abort workflows you started. This one is recorded under another account, or was started before Horizon recorded owners. Admins can abort any workflow.",
		},
		{
			// The owner is the service account that submitted the run, not the user it named.
			name:     "developer cannot abort a run a service account submitted in their name",
			workflow: testWorkflow{owner: testCISubject, startedBy: "dev-alice"},
			claims:   developerClaims, roleChecks: true,
			wantStatus: http.StatusForbidden, wantRequired: "admin", wantCurrent: "developer",
			wantReason: "You can only abort workflows you started. This one is recorded under another account, or was started before Horizon recorded owners. Admins can abort any workflow.",
		},
		{
			name:     "service account aborts the run it submitted in a user's name",
			workflow: testWorkflow{owner: testCISubject, startedBy: "dev-alice"},
			claims:   ci, roleChecks: true,
			wantStatus: http.StatusAccepted,
		},
		{
			name:     "owner below developer cannot abort",
			workflow: testWorkflow{owner: testUserSubject, startedBy: "dev-alice"},
			claims:   demoted, roleChecks: true,
			wantStatus: http.StatusForbidden, wantRequired: "developer", wantCurrent: "viewer",
			wantReason: capAbortOwn.Reason,
		},
		{
			name:     "admin aborts another user's workflow",
			workflow: testWorkflow{owner: bobSubject, startedBy: "dev-bob"},
			claims:   adminClaims, roleChecks: true,
			wantStatus: http.StatusAccepted,
		},
		{
			name:     "admin aborts a workflow with no owner",
			workflow: testWorkflow{},
			claims:   adminClaims, roleChecks: true,
			wantStatus: http.StatusAccepted,
		},
		{
			name:     "own finished workflow is not running",
			workflow: testWorkflow{owner: testUserSubject, phase: "Succeeded"},
			claims:   developerClaims, roleChecks: true,
			wantStatus: http.StatusConflict,
		},
		{
			// Refused like a running one: the answer does not depend on the phase.
			name:     "developer is refused on another user's finished workflow",
			workflow: testWorkflow{owner: bobSubject, startedBy: "dev-bob", phase: "Succeeded"},
			claims:   developerClaims, roleChecks: true,
			wantStatus: http.StatusForbidden, wantRequired: "admin", wantCurrent: "developer",
			wantReason: "You can only abort workflows you started. This one was started by dev-bob. Admins can abort any workflow.",
		},
		{
			name:     "workflow not submitted through Horizon API is not found",
			workflow: testWorkflow{owner: testUserSubject, hidden: true},
			claims:   developerClaims, roleChecks: true,
			wantStatus: http.StatusNotFound,
		},
		{
			name:     "role checks off: developer aborts another user's workflow",
			workflow: testWorkflow{owner: bobSubject, startedBy: "dev-bob"},
			claims:   developerClaims, roleChecks: false,
			wantStatus: http.StatusAccepted,
		},
		{
			name:     "role checks off: viewer aborts a workflow with no owner",
			workflow: testWorkflow{},
			claims:   viewerClaims, roleChecks: false,
			wantStatus: http.StatusAccepted,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.workflow.name = "run"
			e := newRoleEnv(t, auth, horizonauth.DefaultRoleOrder, tc.roleChecks, tc.workflow)
			rec := e.call("POST", "/v1/workflows/run/abort", "", tc.claims)
			if tc.wantStatus == http.StatusForbidden {
				if f := wantForbidden(t, rec, tc.wantRequired, tc.wantCurrent); f.Reason != tc.wantReason {
					t.Errorf("reason = %q, want %q", f.Reason, tc.wantReason)
				}
			} else if rec.Code != tc.wantStatus {
				t.Fatalf("abort = %d %s, want %d", rec.Code, rec.Body, tc.wantStatus)
			}
			if got, want := e.shutdownRequested("run"), tc.wantStatus == http.StatusAccepted; got != want {
				t.Errorf("shutdown requested on the workflow = %t, want %t", got, want)
			}
		})
	}
}

func TestDeleteNeedsAdmin(t *testing.T) {
	auth := newAuthFixture(t)
	own := testWorkflow{name: "run", owner: testUserSubject, startedBy: "dev-alice", phase: "Succeeded"}
	tests := []struct {
		name        string
		claims      map[string]any
		roleChecks  bool
		wantDeleted bool
		wantCurrent string
	}{
		{"viewer", viewerClaims, true, false, "viewer"},
		{"developer, even for own workflow", developerClaims, true, false, "developer"},
		{"admin", adminClaims, true, true, ""},
		{"role checks off: developer", developerClaims, false, true, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := newRoleEnv(t, auth, horizonauth.DefaultRoleOrder, tc.roleChecks, own)
			rec := e.call("DELETE", "/v1/workflows/run", "", tc.claims)
			if tc.wantDeleted {
				if rec.Code != http.StatusOK {
					t.Fatalf("delete = %d %s, want 200", rec.Code, rec.Body)
				}
			} else if f := wantForbidden(t, rec, "admin", tc.wantCurrent); f.Reason != capDelete.Reason {
				t.Errorf("reason = %q, want %q", f.Reason, capDelete.Reason)
			}
			if deleted := e.stored("run") == nil; deleted != tc.wantDeleted {
				t.Errorf("workflow deleted = %t, want %t", deleted, tc.wantDeleted)
			}
		})
	}
}

func TestSubmitNeedsDeveloper(t *testing.T) {
	auth := newAuthFixture(t)
	e := newRoleEnv(t, auth, horizonauth.DefaultRoleOrder, true)
	rec := e.call("POST", "/v1/modules/sample/workflowTemplates/smoke/submit", `{"parameters":{"note":"n"}}`, viewerClaims)
	if f := wantForbidden(t, rec, "developer", "viewer"); f.Reason != capSubmit.Reason {
		t.Errorf("reason = %q, want %q", f.Reason, capSubmit.Reason)
	}
	// A caller whose only client role is impersonate is not a developer.
	onlyImpersonate := map[string]any{"sub": testCISubject, "resource_access": authtest.Roles("impersonate")}
	wantForbidden(t, e.call("POST", "/v1/modules/sample/workflowTemplates/smoke/submit", `{"parameters":{"note":"n"}}`, onlyImpersonate), "developer", "viewer")
	if got := e.submits.Load(); got != 0 {
		t.Errorf("%d workflows were dispatched for refused callers", got)
	}
}

func TestMe(t *testing.T) {
	auth := newAuthFixture(t)
	type capabilityRow struct {
		id, group, requiredRole string
	}
	// The capabilities in the order the Your access panel shows them.
	wantRows := []capabilityRow{
		{"platform.view", "platform", "viewer"},
		{"platform.modules", "platform", "admin"},
		{"platform.settings", "platform", "admin"},
		{"platform.adminApplications", "platform", "admin"},
		{"platform.openArgoWorkflows", "platform", "developer"},
		{"workflows.submit", "workflows", "developer"},
		{"workflows.abortOwn", "workflows", "developer"},
		{"workflows.abortOthers", "workflows", "admin"},
		{"workflows.delete", "workflows", "admin"},
	}
	viewerMay := []string{"platform.view"}
	developerMay := []string{"platform.view", "platform.openArgoWorkflows", "workflows.submit", "workflows.abortOwn"}
	var everything []string
	for _, row := range wantRows {
		everything = append(everything, row.id)
	}
	const fourRoles = "viewer,developer,maintainer,admin"

	tests := []struct {
		name        string
		roleOrder   string
		roleChecks  bool
		claims      map[string]any
		wantName    string
		wantSubject string
		wantRole    string
		wantAllowed []string
	}{
		{"viewer", horizonauth.DefaultRoleOrder, true, viewerClaims, "vera", viewerSubject, "viewer", viewerMay},
		{"developer", horizonauth.DefaultRoleOrder, true, developerClaims, "dev-alice", testUserSubject, "developer", developerMay},
		{"admin", horizonauth.DefaultRoleOrder, true, adminClaims, "anna", adminSubject, "admin", everything},
		{
			"token without a username is named by its subject", horizonauth.DefaultRoleOrder, true,
			map[string]any{"preferred_username": nil}, testUserSubject, testUserSubject, "viewer", viewerMay,
		},
		{
			"impersonate is not a ranked role", horizonauth.DefaultRoleOrder, true,
			map[string]any{"resource_access": authtest.Roles("impersonate")}, "dev-alice", testUserSubject, "viewer", viewerMay,
		},
		{
			"role added between developer and admin", fourRoles, true,
			map[string]any{"resource_access": authtest.Roles("maintainer")}, "dev-alice", testUserSubject, "maintainer", developerMay,
		},
		// The role is still reported, so the portal can show it, but nothing is locked.
		{"role checks off: viewer may do everything", horizonauth.DefaultRoleOrder, false, viewerClaims, "vera", viewerSubject, "viewer", everything},
		{"role checks off: developer may do everything", horizonauth.DefaultRoleOrder, false, developerClaims, "dev-alice", testUserSubject, "developer", everything},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := newRoleEnv(t, auth, tc.roleOrder, tc.roleChecks)
			rec := e.call("GET", "/v1/me", "", tc.claims)
			if rec.Code != http.StatusOK {
				t.Fatalf("GET /v1/me = %d %s, want 200", rec.Code, rec.Body)
			}
			if got := rec.Header().Get("Cache-Control"); got != "no-store" {
				t.Errorf("Cache-Control = %q, want no-store", got)
			}
			var me meResponse
			dec := json.NewDecoder(rec.Body)
			dec.DisallowUnknownFields()
			if err := dec.Decode(&me); err != nil {
				t.Fatal(err)
			}
			if me.Name != tc.wantName || me.Subject != tc.wantSubject || me.Role != tc.wantRole {
				t.Errorf("name %q, subject %q, role %q; want %q, %q, %q", me.Name, me.Subject, me.Role, tc.wantName, tc.wantSubject, tc.wantRole)
			}
			if want := strings.Split(tc.roleOrder, ","); !slices.Equal(me.Roles, want) {
				t.Errorf("roles = %v, want %v", me.Roles, want)
			}
			if me.RoleChecks != tc.roleChecks {
				t.Errorf("roleChecks = %t, want %t", me.RoleChecks, tc.roleChecks)
			}
			var rows []capabilityRow
			var allowed []string
			for _, c := range me.Capabilities {
				rows = append(rows, capabilityRow{c.ID, c.Group, c.RequiredRole})
				if c.Allowed {
					allowed = append(allowed, c.ID)
				}
				if c.Label == "" {
					t.Errorf("capability %s has no label", c.ID)
				}
				// Every role above the lowest is explained to a caller who lacks it.
				if (c.Reason == "") != (c.RequiredRole == "viewer") {
					t.Errorf("capability %s (requires %s) has reason %q", c.ID, c.RequiredRole, c.Reason)
				}
			}
			if !slices.Equal(rows, wantRows) {
				t.Errorf("capabilities = %v, want %v", rows, wantRows)
			}
			if !slices.Equal(allowed, tc.wantAllowed) {
				t.Errorf("allowed = %v, want %v", allowed, tc.wantAllowed)
			}
		})
	}

	t.Run("a role below viewer is refused and told its role", func(t *testing.T) {
		e := newRoleEnv(t, auth, "guest,viewer,developer,admin", true)
		f := wantForbidden(t, e.call("GET", "/v1/me", "", viewerClaims), "viewer", "guest")
		if f.Reason != "Requires the viewer role." {
			t.Errorf("reason = %q", f.Reason)
		}
	})
}

// GET /v1/me must stay cheap: the portal calls it on every page load and token refresh.
func TestMeNeedsNoBackend(t *testing.T) {
	auth := newAuthFixture(t)
	// No catalog, cluster client, workflow store, Argo client or webhook: only the token is read.
	s := New(Options{Verifier: auth.verifier})
	for i := range 3 {
		req := httptest.NewRequest("GET", "/v1/me", nil)
		req.Header.Set("Authorization", "Bearer "+auth.iss.Token(t, auth.iss.Claims(developerClaims)))
		rec := httptest.NewRecorder()
		s.srv.Handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("call %d: GET /v1/me = %d %s, want 200", i+1, rec.Code, rec.Body)
		}
	}
	// The realm's signing keys are fetched once and then cached; later calls do not reach Keycloak.
	if got := auth.iss.KeyFetches(); got != 1 {
		t.Errorf("signing keys were fetched %d times for 3 calls, want 1", got)
	}
}

func TestCheckRoleOrder(t *testing.T) {
	tests := []struct {
		order   string
		wantErr string
	}{
		{horizonauth.DefaultRoleOrder, ""},
		{"viewer,developer,maintainer,admin", ""},
		{"guest,viewer,developer,admin", ""},
		{"developer,admin", `role order "developer,admin" must include viewer`},
		{"reader,operator", `role order "reader,operator" must include viewer, developer, admin`},
	}
	for _, tc := range tests {
		order, err := horizonauth.ParseRoleOrder(tc.order)
		if err != nil {
			t.Fatal(err)
		}
		// The result must not depend on whether role checks are on.
		for _, roleChecks := range []bool{false, true} {
			err := CheckRoleOrder(horizonauth.NewAuthorizer(order, roleChecks))
			got := ""
			if err != nil {
				got = err.Error()
			}
			if got != tc.wantErr {
				t.Errorf("CheckRoleOrder(%q, role checks %t) = %q, want %q", tc.order, roleChecks, got, tc.wantErr)
			}
		}
	}
}

func TestOpenAPIDocumentsEndpointRoles(t *testing.T) {
	s := New(Options{Catalog: submitCatalog(t)})
	rec := httptest.NewRecorder()
	s.srv.Handler.ServeHTTP(rec, httptest.NewRequest("GET", "/openapi.json", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /openapi.json = %d", rec.Code)
	}
	var doc struct {
		Paths      map[string]map[string]json.RawMessage `json:"paths"`
		Components struct {
			Schemas map[string]json.RawMessage `json:"schemas"`
		} `json:"components"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	type response struct {
		Description string `json:"description"`
		Content     map[string]struct {
			Schema map[string]any `json:"schema"`
		} `json:"content"`
	}
	// responses returns the documented responses of an operation, by status.
	responses := func(method, path string) (map[string]response, bool) {
		t.Helper()
		raw, ok := doc.Paths[path][strings.ToLower(method)]
		if !ok {
			return nil, false
		}
		var op struct {
			Responses map[string]response `json:"responses"`
		}
		if err := json.Unmarshal(raw, &op); err != nil {
			t.Fatalf("%s %s: %v", method, path, err)
		}
		return op.Responses, true
	}
	// Every operation that needs a token, with its minimum role. The document has one submit
	// operation per catalog entry instead of the submit route pattern.
	gated := []struct{ method, path, role string }{
		{"GET", "/v1/me", "viewer"},
		{"GET", "/v1/catalog", "viewer"},
		{"POST", "/v1/modules/sample/workflowTemplates/launcher/submit", "developer"},
		{"POST", "/v1/modules/sample/workflowTemplates/smoke/submit", "developer"},
		{"GET", "/v1/workflows/running", "viewer"},
		{"GET", "/v1/workflows/history", "viewer"},
		{"GET", "/v1/workflows/{workflowName}/log", "viewer"},
		{"GET", "/v1/workflows/{workflowName}/downloadArtifact/{artifactName}", "viewer"},
		{"GET", "/v1/workflows/{workflowName}", "viewer"},
		{"DELETE", "/v1/workflows/{workflowName}", "admin"},
		{"POST", "/v1/workflows/{workflowName}/abort", "developer"},
		{"GET", "/v1/artifacts/buckets", "viewer"},
		{"GET", "/v1/artifacts/buckets/{bucket}/objects", "viewer"},
	}
	seen := make(map[string]bool, len(gated))
	for _, op := range gated {
		seen[op.method+" "+op.path] = true
		documented, ok := responses(op.method, op.path)
		if !ok {
			t.Errorf("%s %s is not in the OpenAPI document", op.method, op.path)
			continue
		}
		forbidden := documented["403"]
		if want := "below `" + op.role + "`"; !strings.Contains(forbidden.Description, want) {
			t.Errorf("%s %s: 403 is documented as %q, want it to say %q", op.method, op.path, forbidden.Description, want)
		}
		if got := forbidden.Content["application/json"].Schema["$ref"]; got != "#/components/schemas/Forbidden" {
			t.Errorf("%s %s: 403 body schema = %v, want the Forbidden schema", op.method, op.path, got)
		}
	}
	for path, methods := range doc.Paths {
		for method := range methods {
			key := strings.ToUpper(method) + " " + path
			if !seen[key] {
				t.Errorf("%s is in the OpenAPI document, but its minimum role is not checked here", key)
			}
		}
	}
	me, _ := responses("GET", "/v1/me")
	if got := me["200"].Content["application/json"].Schema["$ref"]; got != "#/components/schemas/MeResponse" {
		t.Errorf("GET /v1/me 200 body schema = %v, want the MeResponse schema", got)
	}
	objects, _ := responses("GET", "/v1/artifacts/buckets/{bucket}/objects")
	if got := objects["403"].Content["text/plain"].Schema["type"]; got != "string" {
		t.Errorf("GET artifact objects plain-text 403 schema type = %v, want string", got)
	}
	for _, name := range []string{"Forbidden", "MeResponse", "MeCapability"} {
		if _, ok := doc.Components.Schemas[name]; !ok {
			t.Errorf("schema %s is missing", name)
		}
	}
}
