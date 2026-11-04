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

// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
)

// portalAccessPath is the Horizon API contract the Developer Portal calls for RBAC.
// While PORTAL_ACCESS_MOCK is not "false", the proxy verifies the caller and answers
// here. Set PORTAL_ACCESS_MOCK=false to forward this path with the same user token.
const portalAccessPath = "/v1/portal/access"

// Mock-only role switch so a signed-in user can be rendered as Viewer, Developer, or Admin
// before Keycloak group mapping is in place. Ignored once the real endpoint is forwarded.
const portalRoleOverrideHeader = "X-Horizon-Portal-Role"

type stringOrSlice []string

func (s *stringOrSlice) UnmarshalJSON(b []byte) error {
	if len(b) == 0 || string(b) == "null" {
		return nil
	}
	if b[0] == '"' {
		var one string
		if err := json.Unmarshal(b, &one); err != nil {
			return err
		}
		*s = []string{one}
		return nil
	}
	var many []string
	if err := json.Unmarshal(b, &many); err != nil {
		return err
	}
	*s = many
	return nil
}

type portalClaims struct {
	Sub               string        `json:"sub"`
	Email             string        `json:"email"`
	PreferredUsername string        `json:"preferred_username"`
	Name              string        `json:"name"`
	Groups            stringOrSlice `json:"groups"`
	RealmAccess       struct {
		Roles stringOrSlice `json:"roles"`
	} `json:"realm_access"`
}

type portalFunction struct {
	ID           string          `json:"id"`
	Label        string          `json:"label"`
	Grants       map[string]bool `json:"grants"`
	Denial       string          `json:"denial,omitempty"`
	Tooltip      string          `json:"tooltip,omitempty"`
	RequiredRole string          `json:"requiredRole,omitempty"`
}

type portalSection struct {
	ID        string           `json:"id"`
	Title     string           `json:"title"`
	Functions []portalFunction `json:"functions"`
}

type adminOnlyTemplate struct {
	Module  string `json:"module"`
	Pattern string `json:"pattern"`
	Label   string `json:"label"`
	Denial  string `json:"denial"`
}

type portalApplication struct {
	ID       string `json:"id"`
	Audience string `json:"audience"`
}

type portalAccess struct {
	Mock               bool                `json:"mock"`
	Subject            string              `json:"subject"`
	Email              string              `json:"email"`
	Role               string              `json:"role"`
	RoleLabel          string              `json:"roleLabel"`
	KeycloakGroup      string              `json:"keycloakGroup"`
	Summary            string              `json:"summary"`
	MoreAccess         string              `json:"moreAccess"`
	Roles              []portalRoleColumn  `json:"roles"`
	Sections           []portalSection     `json:"sections"`
	Applications       []portalApplication `json:"applications"`
	AdminOnlyTemplates []adminOnlyTemplate `json:"adminOnlyTemplates"`
}

type portalRoleColumn struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

func writePortalAccess(w http.ResponseWriter, r *http.Request, idToken *oidc.IDToken) {
	var claims portalClaims
	_ = idToken.Claims(&claims)
	if claims.Sub == "" {
		claims.Sub = idToken.Subject
	}
	access := accessFromClaims(claims, r.Header.Get(portalRoleOverrideHeader))
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(access)
}

func accessFromClaims(claims portalClaims, roleOverride string) portalAccess {
	subject := firstNonEmpty(claims.PreferredUsername, claims.Name, claims.Email, claims.Sub)
	email := firstNonEmpty(claims.Email, subject)
	groups := append([]string{}, claims.Groups...)
	groups = append(groups, claims.RealmAccess.Roles...)
	role, group, defaulted := resolvePortalRole(groups, roleOverride)
	return buildPortalAccess(subject, email, role, group, defaulted)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func resolvePortalRole(groups []string, roleOverride string) (role, group string, defaulted bool) {
	if canonical, ok := canonicalRole(roleOverride); ok {
		return canonical, canonicalGroup(canonical), false
	}
	bestRank := 0
	best := ""
	for _, raw := range groups {
		name := normalizeGroup(raw)
		rank := 0
		switch name {
		case "administrators", "admins", "admin", "horizon-administrators":
			rank = 3
			name = "admin"
		case "developers", "developer", "horizon-developers":
			rank = 2
			name = "developer"
		case "viewers", "viewer", "horizon-viewers":
			rank = 1
			name = "viewer"
		default:
			continue
		}
		if rank > bestRank {
			bestRank = rank
			best = name
		}
	}
	if best == "" {
		return "developer", "developers", true
	}
	return best, canonicalGroup(best), false
}

func canonicalRole(value string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "viewer", "developer", "admin":
		return strings.ToLower(strings.TrimSpace(value)), true
	default:
		return "", false
	}
}

func canonicalGroup(role string) string {
	switch role {
	case "admin":
		return "administrators"
	case "viewer":
		return "viewers"
	default:
		return "developers"
	}
}

func normalizeGroup(raw string) string {
	name := strings.ToLower(strings.TrimSpace(raw))
	name = strings.TrimPrefix(name, "/")
	if i := strings.LastIndex(name, "/"); i >= 0 {
		name = name[i+1:]
	}
	return name
}

func roleLabel(role string) string {
	switch role {
	case "admin":
		return "Admin"
	case "viewer":
		return "Viewer"
	default:
		return "Developer"
	}
}

func grants(viewer, developer, admin bool) map[string]bool {
	return map[string]bool{"viewer": viewer, "developer": developer, "admin": admin}
}

func buildPortalAccess(subject, email, role, group string, defaulted bool) portalAccess {
	summary := "Your role comes from your Keycloak group: " + group + ". Admin includes everything a Developer can do."
	if defaulted {
		summary = "No Keycloak group matched viewers, developers, or administrators, so this mock assigns Developer. Admin includes everything a Developer can do."
	}
	templates := []adminOnlyTemplate{
		{
			Module:  "workloads-android",
			Pattern: "cf-instance-template-*-delete",
			Label:   "cf-instance-template-*-delete",
			Denial:  "Admin only. Deletes the shared Cuttlefish instance template for everyone. You can open the form and read it, but Submit is disabled for Developers.",
		},
		{
			Module:  "remotive-topology",
			Pattern: "remotiv-instance-template-delete",
			Label:   "remotiv-instance-template-delete",
			Denial:  "Admin only. Deletes the shared Remotive instance template for everyone. You can open the form and read it, but Submit is disabled for Developers.",
		},
		{
			Module:  "remotive-topology",
			Pattern: "remotiv-builder-image",
			Label:   "remotiv-builder-image",
			Denial:  "Admin only. Publishes the shared Remotive builder image for everyone. You can open the form and read it, but Submit is disabled for Developers.",
		},
	}
	return portalAccess{
		Mock:          true,
		Subject:       subject,
		Email:         email,
		Role:          role,
		RoleLabel:     roleLabel(role),
		KeycloakGroup: group,
		Summary:       summary,
		MoreAccess:    "Need more access? Ask a Horizon administrator to add you to the developers or administrators group.",
		Roles: []portalRoleColumn{
			{ID: "viewer", Label: "Viewer"},
			{ID: "developer", Label: "Developer"},
			{ID: "admin", Label: "Admin"},
		},
		Sections: []portalSection{
			{
				ID:    "platform",
				Title: "PLATFORM",
				Functions: []portalFunction{
					{ID: "platform.view", Label: "View modules, workflows, logs, artifacts", Grants: grants(true, true, true)},
					{ID: "platform.modules", Label: "Enable, disable, pin or reset modules", Grants: grants(false, false, true), Denial: "Enabling a module creates cloud resources that everyone shares.", RequiredRole: "admin"},
					{ID: "platform.settings", Label: "Portal settings", Grants: grants(false, false, true), Denial: "Portal settings change what every signed-in user sees.", RequiredRole: "admin"},
					{ID: "platform.adminApplications", Label: "Open Admin applications", Grants: grants(false, false, true), Denial: "Admin applications manage the whole platform.", RequiredRole: "admin"},
					{ID: "platform.openArgoWorkflows", Label: "Open Argo Workflows", Grants: grants(true, true, true)},
				},
			},
			{
				ID:    "workflows",
				Title: "WORKFLOWS",
				Functions: []portalFunction{
					{ID: "workflows.submit", Label: "Submit workflow templates", Grants: grants(false, true, true), Denial: "Submitting a workflow uses shared build and test capacity.", RequiredRole: "developer"},
					{ID: "workflows.abortOwn", Label: "Abort workflows you started", Grants: grants(false, true, true), Denial: "Aborting a workflow stops a shared run.", RequiredRole: "developer"},
					{ID: "workflows.abortOthers", Label: "Abort others' or ownerless workflows", Grants: grants(false, false, true), Denial: "Aborting someone else's workflow stops their run for everyone.", RequiredRole: "admin"},
					{ID: "workflows.delete", Label: "Delete workflow records", Grants: grants(false, false, true), Tooltip: "Admin only. Deleting removes the workflow record and its logs for everyone, including workflows you started.", RequiredRole: "admin"},
				},
			},
			templateSection("workloads-android", templates[0]),
			templateSection("remotive-topology", templates[1], templates[2]),
		},
		Applications: []portalApplication{
			{ID: "gerrit", Audience: "developer"},
			{ID: "jenkins", Audience: "developer"},
			{ID: "mtk-connect", Audience: "developer"},
			{ID: "mcp-gateway", Audience: "developer"},
			{ID: "argo-workflows", Audience: "developer"},
			{ID: "keycloak", Audience: "admin"},
			{ID: "argo-cd", Audience: "admin"},
			{ID: "headlamp", Audience: "admin"},
			{ID: "grafana", Audience: "admin"},
		},
		AdminOnlyTemplates: templates,
	}
}

func templateSection(module string, rules ...adminOnlyTemplate) portalSection {
	fns := make([]portalFunction, 0, len(rules))
	for _, rule := range rules {
		fns = append(fns, portalFunction{
			ID:           "template." + module + "." + rule.Pattern,
			Label:        rule.Label,
			Grants:       grants(false, false, true),
			Tooltip:      rule.Denial,
			RequiredRole: "admin",
		})
	}
	return portalSection{
		ID:        "templates." + module,
		Title:     strings.ToUpper(module) + ": ADMIN-ONLY TEMPLATES",
		Functions: fns,
	}
}

func matchTemplatePattern(pattern, name string) bool {
	pattern = strings.TrimSpace(pattern)
	name = strings.TrimSpace(name)
	if pattern == "" || name == "" {
		return false
	}
	var b strings.Builder
	b.WriteString("(?i)^")
	for _, r := range pattern {
		if r == '*' {
			b.WriteString(".*")
			continue
		}
		if strings.ContainsRune(`.+?^${}()|[]\`, r) {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	b.WriteByte('$')
	re, err := regexp.Compile(b.String())
	if err != nil {
		return false
	}
	return re.MatchString(name)
}
