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
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

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
	role, group, defaulted = resolvePortalRole([]string{"developers"}, " ADMIN ")
	if role != "admin" || group != "administrators" || defaulted {
		t.Fatalf("trimmed override got %s %s defaulted %v", role, group, defaulted)
	}
	role, group, defaulted = resolvePortalRole([]string{"/org/horizon-administrators"}, "")
	if role != "admin" || group != "administrators" || defaulted {
		t.Fatalf("path group got %s %s defaulted %v", role, group, defaulted)
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

func TestAccessFromClaimsSubjectFallback(t *testing.T) {
	access := accessFromClaims(portalClaims{Sub: "sub-1"}, "")
	if access.Subject != "sub-1" || access.Email != "sub-1" {
		t.Fatalf("sub fallback got subject %q email %q", access.Subject, access.Email)
	}
	access = accessFromClaims(portalClaims{Name: "Some User"}, "")
	if access.Subject != "Some User" || access.Email != "Some User" {
		t.Fatalf("name fallback got subject %q email %q", access.Subject, access.Email)
	}
}

func TestMatchTemplatePattern(t *testing.T) {
	tests := []struct {
		pattern, name string
		want          bool
	}{
		{"", "name", false},
		{"pattern", "", false},
		{"a.b", "axb", false},
		{"a.b", "A.B", true},
		{"a+b", "a+b", true},
		{"a+b", "aab", false},
		{"CF-*-delete", "cf-x86-DELETE", true},
	}
	for _, tc := range tests {
		if got := matchTemplatePattern(tc.pattern, tc.name); got != tc.want {
			t.Errorf("matchTemplatePattern(%q, %q) = %v, want %v", tc.pattern, tc.name, got, tc.want)
		}
	}
}

func TestStringOrSlice(t *testing.T) {
	var one stringOrSlice
	if err := json.Unmarshal([]byte(`"developers"`), &one); err != nil || len(one) != 1 || one[0] != "developers" {
		t.Fatalf("single string got %v, %v", one, err)
	}
	var many stringOrSlice
	if err := json.Unmarshal([]byte(`["a","b"]`), &many); err != nil || len(many) != 2 || many[1] != "b" {
		t.Fatalf("array got %v, %v", many, err)
	}
	var none stringOrSlice
	if err := json.Unmarshal([]byte(`null`), &none); err != nil || len(none) != 0 {
		t.Fatalf("null got %v, %v", none, err)
	}
	var bad stringOrSlice
	if err := json.Unmarshal([]byte(`1`), &bad); err == nil {
		t.Fatal("expected error for a number")
	}
}

func TestWritePortalAccessSubjectFallback(t *testing.T) {
	issuer := newTestOIDC(t)
	raw := issuer.signClaims(t, map[string]any{
		"iss": issuer.issuer,
		"sub": "a<b",
		"aud": "horizon-dev-portal",
		"iat": time.Now().Add(-time.Minute).Unix(),
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	idToken, err := issuer.verifier.Verify(context.Background(), raw)
	if err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	writePortalAccess(rec, httptest.NewRequest(http.MethodGet, "/api/horizon/v1/portal/access", nil), idToken)

	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control %q", got)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"subject":"a<b"`) {
		t.Fatalf("subject not written unescaped: %s", body)
	}
	var access portalAccess
	if err := json.Unmarshal(rec.Body.Bytes(), &access); err != nil {
		t.Fatal(err)
	}
	if access.Subject != "a<b" || access.Email != "a<b" {
		t.Fatalf("access %+v", access)
	}
}
