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

import "testing"

func TestResolvePortalRole(t *testing.T) {
	role, group, defaulted := resolvePortalRole([]string{"/horizon-developers", "viewers"}, "")
	if role != "developer" || group != "developers" || defaulted {
		t.Fatalf("developer got %s %s defaulted %v", role, group, defaulted)
	}
	role, group, defaulted = resolvePortalRole([]string{"developers", "administrators"}, "")
	if role != "admin" || group != "administrators" || defaulted {
		t.Fatalf("admin got %s %s defaulted %v", role, group, defaulted)
	}
	role, group, defaulted = resolvePortalRole(nil, "viewer")
	if role != "viewer" || group != "viewers" || defaulted {
		t.Fatalf("override got %s %s defaulted %v", role, group, defaulted)
	}
	role, _, defaulted = resolvePortalRole(nil, "nope")
	if role != "developer" || !defaulted {
		t.Fatalf("default got %s defaulted %v", role, defaulted)
	}
}

func TestAccessFromClaims(t *testing.T) {
	access := accessFromClaims(portalClaims{
		Email:             "robert.colbert@accenture.com",
		PreferredUsername: "robert.colbert@accenture.com",
		Groups:            stringOrSlice{"developers"},
	}, "")
	if access.Role != "developer" || access.Email != "robert.colbert@accenture.com" || !access.Mock {
		t.Fatalf("access %+v", access)
	}
	if len(access.Sections) < 4 {
		t.Fatalf("sections %d", len(access.Sections))
	}
	if access.Sections[2].Title != "WORKLOADS-ANDROID: ADMIN-ONLY TEMPLATES" {
		t.Fatalf("template section %q", access.Sections[2].Title)
	}
	if !matchTemplatePattern("cf-instance-template-*-delete", "cf-instance-template-arm64-delete") {
		t.Fatal("expected wildcard match")
	}
	if matchTemplatePattern("cf-instance-template-*-delete", "aaos-builder") {
		t.Fatal("unexpected match")
	}
}
