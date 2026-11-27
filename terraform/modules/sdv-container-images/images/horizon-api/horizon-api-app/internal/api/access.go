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
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/acn-horizon-sdv/horizon-api/internal/workflow"
	"github.com/acn-horizon-sdv/horizonauth"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Roles the permission matrix names. The configured role order must rank all three; it may rank
// more roles around them.
const (
	roleViewer    = "viewer"
	roleDeveloper = "developer"
	roleAdmin     = "admin"
)

// capability is one row of the permission matrix: an action, the minimum role for it and why
// that role is needed. Every endpoint declares the capability it needs and GET /v1/me returns
// the whole list, so the portal and the CLI show the same decisions that the endpoints enforce.
type capability struct {
	// ID is stable; clients look a capability up by it. The part before the dot is its group.
	ID           string
	Label        string
	RequiredRole string
	// Reason tells a caller below RequiredRole why the role is needed. It is the reason in the
	// 403 body and in GET /v1/me. Empty uses the default reason of horizonauth.
	Reason string
}

// Capabilities that Horizon API endpoints enforce.
var (
	capView = capability{
		ID:           "platform.view",
		Label:        "View modules, catalog, workflows, logs, artifacts",
		RequiredRole: roleViewer,
	}
	// capSubmit is the floor for every template. A template's own required role is checked on
	// top of it once templates declare one.
	capSubmit = capability{
		ID:           "workflows.submit",
		Label:        "Submit a developer-level workflow template",
		RequiredRole: roleDeveloper,
		Reason:       "Submitting a workflow uses shared build and test capacity.",
	}
	capAbortOwn = capability{
		ID:           "workflows.abortOwn",
		Label:        "Abort own running workflow",
		RequiredRole: roleDeveloper,
		Reason:       "Aborting a workflow stops a run.",
	}
	capAbortOthers = capability{
		ID:           "workflows.abortOthers",
		Label:        "Abort another user's or an ownerless workflow",
		RequiredRole: roleAdmin,
		Reason:       "Aborting someone else's workflow stops their run.",
	}
	capDelete = capability{
		ID:           "workflows.delete",
		Label:        "Delete workflow record (history, logs)",
		RequiredRole: roleAdmin,
		Reason:       "Deleting removes the workflow record and its logs for everyone, including workflows you started.",
	}
)

// Capabilities that Horizon API does not enforce itself: Module Manager and the applications do.
// They are listed so that the Your access panel is built from GET /v1/me alone and the portal
// keeps no capability list of its own.
var (
	capModules = capability{
		ID:           "platform.modules",
		Label:        "Module enable, disable, pin to ref, edit ref, reset to platform branch",
		RequiredRole: roleAdmin,
		Reason:       "Enabling a module creates cloud resources that everyone shares, and disabling it deletes them.",
	}
	capSettings = capability{
		ID:           "platform.settings",
		Label:        "Portal settings (workflow visibility)",
		RequiredRole: roleAdmin,
		Reason:       "Portal settings change what every signed-in user sees.",
	}
	capAdminApplications = capability{
		ID:           "platform.adminApplications",
		Label:        "Open Admin applications (Keycloak, Argo CD, Headlamp, Grafana)",
		RequiredRole: roleAdmin,
		Reason:       "Admin applications manage the whole platform.",
	}
	capOpenArgoWorkflows = capability{
		ID:           "platform.openArgoWorkflows",
		Label:        "Open Argo Workflows",
		RequiredRole: roleDeveloper,
		Reason:       "Argo Workflows gives direct control over every workflow on the platform.",
	}
)

// capabilities is the permission matrix in the order GET /v1/me returns it: the platform group,
// then the workflows group.
var capabilities = []capability{
	capView, capModules, capSettings, capAdminApplications, capOpenArgoWorkflows,
	capSubmit, capAbortOwn, capAbortOthers, capDelete,
}

// CheckRoleOrder returns an error when the role order does not rank a role that the permission
// matrix names. With role checks on, an endpoint that requires an unranked role refuses every
// caller, so a deployment should not start with such an order.
func CheckRoleOrder(a *horizonauth.Authorizer) error {
	roles := a.Roles()
	var missing []string
	for _, role := range []string{roleViewer, roleDeveloper, roleAdmin} {
		if !slices.Contains(roles, role) {
			missing = append(missing, role)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("role order %q must include %s", strings.Join(roles, ","), strings.Join(missing, ", "))
	}
	return nil
}

// meResponse is the body of GET /v1/me.
type meResponse struct {
	// Name is the caller's username, or the token subject when the token has none.
	Name string `json:"name"`
	// Subject is the token subject. A workflow whose owner equals it is the caller's own.
	Subject string `json:"subject"`
	// Role is the caller's rank. It is reported whether or not role checks are on.
	Role string `json:"role"`
	// Roles is the role order, lowest first, so a client can compare a required role with Role.
	Roles []string `json:"roles"`
	// RoleChecks reports whether minimum roles are enforced. When false, every capability is allowed.
	RoleChecks   bool           `json:"roleChecks"`
	Capabilities []meCapability `json:"capabilities"`
}

type meCapability struct {
	ID           string `json:"id"`
	Group        string `json:"group"`
	Label        string `json:"label"`
	RequiredRole string `json:"requiredRole"`
	// Allowed is the decision an endpoint that needs this capability makes for the caller.
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason,omitempty"`
}

// handleMe describes the caller: name, role and what they may do. It reads only the verified
// token and the role settings; it makes no call to Keycloak, Kubernetes or Module Manager.
func (s *Server) handleMe(w http.ResponseWriter, _ *http.Request, p *horizonauth.Principal) {
	name := strings.TrimSpace(p.Username)
	if name == "" {
		name = p.Subject
	}
	authz := s.opt.Authorizer
	resp := meResponse{
		Name:         name,
		Subject:      p.Subject,
		Role:         authz.Role(p),
		Roles:        authz.Roles(),
		RoleChecks:   authz.RoleChecks(),
		Capabilities: make([]meCapability, 0, len(capabilities)),
	}
	for _, c := range capabilities {
		group, _, _ := strings.Cut(c.ID, ".")
		resp.Capabilities = append(resp.Capabilities, meCapability{
			ID:           c.ID,
			Group:        group,
			Label:        c.Label,
			RequiredRole: c.RequiredRole,
			Allowed:      authz.Check(p, c.RequiredRole, "") == nil,
			Reason:       c.Reason,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	// The answer depends on the caller's token, and the caller's role can change.
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(resp)
}

// abortForbidden returns nil when p may abort workflow u, otherwise the response to send. The
// endpoint has already required capAbortOwn.
//
// A workflow is the caller's own when its owner equals the token subject. Horizon API sets the
// owner from the verified token on submit, so a caller cannot claim someone else's workflow.
// Any other workflow needs capAbortOthers: another caller's, one with no recorded owner, and one
// from before the owner came from the token, whose owner is a username and so matches no subject.
func (s *Server) abortForbidden(p *horizonauth.Principal, u *unstructured.Unstructured) *horizonauth.Forbidden {
	owner := workflow.Owner(u)
	if owner != "" && owner == p.Subject {
		return nil
	}
	return s.opt.Authorizer.Check(p, capAbortOthers.RequiredRole, abortOthersReason(p, owner, workflow.StartedBy(u)))
}

// abortOthersReason says why the caller may not abort a workflow that is not their own.
func abortOthersReason(p *horizonauth.Principal, owner, startedBy string) string {
	const (
		rule   = "You can only abort workflows you started. "
		admins = " Admins can abort any workflow."
	)
	switch {
	case owner == "":
		return rule + "This one has no recorded owner." + admins
	case startedBy == "" || startedBy == p.Username || startedBy == p.Subject:
		// The run shows the caller's own name but is not recorded as theirs: a service account
		// submitted it in their name, or it was started before the owner came from the token.
		return rule + "This one is recorded under another account, or was started before Horizon recorded owners." + admins
	default:
		return rule + "This one was started by " + startedBy + "." + admins
	}
}
