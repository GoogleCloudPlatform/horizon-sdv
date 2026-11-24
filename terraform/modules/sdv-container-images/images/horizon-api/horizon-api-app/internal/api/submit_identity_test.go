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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/acn-horizon-sdv/horizon-api/internal/catalog"
	"github.com/acn-horizon-sdv/horizonauth"
	"github.com/acn-horizon-sdv/horizonauth/authtest"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	testUserSubject = "c7eae7a2-0420-4942-9a83-492a87401631" // authtest default sub, username dev-alice
	testCISubject   = "5b0e7c1d-8a44-4d0e-9a55-0c3f0a1f6e21"
	testCIUsername  = "service-account-horizon-api-ci"
)

// submitCatalog returns a catalog for the enabled module "sample" with two exposed templates:
// "launcher" declares the submittedBy parameter, "smoke" does not.
func submitCatalog(t *testing.T) *catalog.Catalog {
	t.Helper()
	object := func(kind, namespace, name string) *unstructured.Unstructured {
		u := &unstructured.Unstructured{}
		u.SetAPIVersion("horizon-sdv.io/v1alpha1")
		u.SetKind(kind)
		u.SetNamespace(namespace)
		u.SetName(name)
		return u
	}
	set := func(u *unstructured.Unstructured, value interface{}, fields ...string) {
		t.Helper()
		if err := unstructured.SetNestedField(u.Object, value, fields...); err != nil {
			t.Fatal(err)
		}
	}
	state := object("ModuleManagerState", "module-manager", "cluster")
	set(state, []interface{}{"id-sample"}, "status", "enabledModules")
	set(state, map[string]interface{}{"sample": "id-sample"}, "status", "moduleIds")
	modules := object("ModuleCatalog", "module-manager", "cluster")
	set(modules, []interface{}{map[string]interface{}{"name": "sample"}}, "spec", "modules")

	template := func(name string, params ...string) *unstructured.Unstructured {
		u := object("WorkflowTemplate", "workflows", name)
		u.SetAPIVersion("argoproj.io/v1alpha1")
		u.SetLabels(map[string]string{"horizon-sdv.io/expose": "true", "horizon-sdv.io/module": "sample"})
		list := make([]interface{}, 0, len(params))
		for _, p := range params {
			list = append(list, map[string]interface{}{"name": p, "value": "default"})
		}
		set(u, list, "spec", "arguments", "parameters")
		return u
	}

	cat := catalog.New("module-manager", "workflows", "cluster", "cluster")
	cat.InjectClient(fake.NewClientBuilder().WithScheme(runtime.NewScheme()).WithObjects(
		state, modules,
		template("launcher", "topology", "submittedBy"),
		template("smoke", "note"),
	).Build())
	if err := cat.Rebuild(context.Background()); err != nil {
		t.Fatal(err)
	}
	return cat
}

func TestSubmitTakesCallerIdentityFromToken(t *testing.T) {
	iss := authtest.NewIssuer(t)
	v, err := horizonauth.NewVerifier(context.Background(), iss.URL(), "horizon-api")
	if err != nil {
		t.Fatal(err)
	}
	var dispatched map[string]any
	webhook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		dispatched = nil
		if err := json.NewDecoder(r.Body).Decode(&dispatched); err != nil {
			t.Errorf("webhook body: %v", err)
		}
	}))
	t.Cleanup(webhook.Close)
	// Role checks are not configured, as in a default deployment: caller identity applies anyway.
	s := New(Options{Catalog: submitCatalog(t), Verifier: v, EventsWebhookURL: webhook.URL})

	ci := map[string]any{"sub": testCISubject, "azp": "horizon-api-ci", "preferred_username": testCIUsername}
	withRoles := func(base map[string]any, roles ...string) map[string]any {
		c := map[string]any{"resource_access": authtest.Roles(roles...)}
		for k, val := range base {
			c[k] = val
		}
		return c
	}

	tests := []struct {
		name            string
		claims          map[string]any
		template        string
		header          string // X-Horizon-Submitted-By
		params          string // JSON object
		wantOwner       string
		wantSubmittedBy string
	}{
		{
			name:     "portal user: username and subject come from the token",
			template: "launcher", params: `{"topology":"t1"}`,
			wantOwner: testUserSubject, wantSubmittedBy: "dev-alice",
		},
		{
			name:     "header naming another user is ignored",
			template: "smoke", header: "mallory", params: `{"note":"n"}`,
			wantOwner: testUserSubject, wantSubmittedBy: "dev-alice",
		},
		{
			name:     "submittedBy parameter naming another user is ignored",
			claims:   withRoles(nil, "developer"),
			template: "launcher", header: "mallory", params: `{"topology":"t1","submittedBy":"mallory"}`,
			wantOwner: testUserSubject, wantSubmittedBy: "dev-alice",
		},
		{
			name:     "admin without impersonate cannot name another user",
			claims:   withRoles(nil, "admin", "developer"),
			template: "launcher", params: `{"topology":"t1","submittedBy":"mallory"}`,
			wantOwner: testUserSubject, wantSubmittedBy: "dev-alice",
		},
		{
			name:     "token without a username falls back to the subject",
			claims:   map[string]any{"preferred_username": nil},
			template: "launcher", params: `{"topology":"t1"}`,
			wantOwner: testUserSubject, wantSubmittedBy: testUserSubject,
		},
		{
			name:     "service account without impersonate submits as itself",
			claims:   withRoles(ci, "developer"),
			template: "launcher", header: "bob", params: `{"topology":"t1","submittedBy":"bob"}`,
			wantOwner: testCISubject, wantSubmittedBy: testCIUsername,
		},
		{
			name:     "impersonating service account names the user in the parameter",
			claims:   withRoles(ci, "developer", "impersonate"),
			template: "launcher", params: `{"topology":"t1","submittedBy":" bob "}`,
			wantOwner: testCISubject, wantSubmittedBy: "bob",
		},
		{
			name:     "impersonating service account names the user in the header",
			claims:   withRoles(ci, "developer", "impersonate"),
			template: "smoke", header: "bob", params: `{"note":"n"}`,
			wantOwner: testCISubject, wantSubmittedBy: "bob",
		},
		{
			name:     "parameter wins over header",
			claims:   withRoles(ci, "impersonate"),
			template: "launcher", header: "carol", params: `{"topology":"t1","submittedBy":"bob"}`,
			wantOwner: testCISubject, wantSubmittedBy: "bob",
		},
		{
			name:     "impersonating service account that names nobody submits as itself",
			claims:   withRoles(ci, "impersonate"),
			template: "launcher", params: `{"topology":"t1"}`,
			wantOwner: testCISubject, wantSubmittedBy: testCIUsername,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dispatched = nil
			req := httptest.NewRequest(http.MethodPost,
				"/v1/modules/sample/workflowTemplates/"+tc.template+"/submit",
				strings.NewReader(`{"parameters":`+tc.params+`}`))
			req.Header.Set("Authorization", "Bearer "+iss.Token(t, iss.Claims(tc.claims)))
			if tc.header != "" {
				req.Header.Set("X-Horizon-Submitted-By", tc.header)
			}
			rec := httptest.NewRecorder()
			s.srv.Handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusAccepted {
				t.Fatalf("submit = %d %s, want 202", rec.Code, rec.Body)
			}
			// horizonSubmittedBy is what every sensor maps to the owner annotation.
			if got := dispatched["horizonSubmittedBy"]; got != tc.wantOwner {
				t.Errorf("owner (horizonSubmittedBy) = %v, want %q", got, tc.wantOwner)
			}
			if got := dispatched["submittedBy"]; got != tc.wantSubmittedBy {
				t.Errorf("submittedBy = %v, want %q", got, tc.wantSubmittedBy)
			}
		})
	}
}
