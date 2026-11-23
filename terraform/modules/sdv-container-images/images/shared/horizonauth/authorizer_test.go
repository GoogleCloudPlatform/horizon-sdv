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

package horizonauth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

var (
	viewer    = &Principal{Subject: "v"}
	developer = &Principal{Subject: "d", ClientRoles: []string{"developer"}}
	admin     = &Principal{Subject: "a", ClientRoles: []string{"admin", "developer"}}
)

func TestCheckWithRoleChecksOffAllowsEveryone(t *testing.T) {
	a := NewAuthorizer(mustOrder(t, DefaultRoleOrder), false)
	for _, p := range []*Principal{viewer, developer, admin} {
		for _, required := range []string{"viewer", "developer", "admin", "typo"} {
			if f := a.Check(p, required, ""); f != nil {
				t.Errorf("Check(%s, %s) = %+v with role checks off", p.Subject, required, f)
			}
		}
	}
	if got := a.Role(developer); got != "developer" {
		t.Errorf("Role = %q, want developer even with role checks off", got)
	}
}

func TestCheckWithRoleChecksOn(t *testing.T) {
	a := NewAuthorizer(mustOrder(t, DefaultRoleOrder), true)
	tests := []struct {
		p        *Principal
		required string
		allowed  bool
	}{
		{viewer, "viewer", true},
		{viewer, "developer", false},
		{developer, "developer", true},
		{developer, "admin", false},
		{admin, "admin", true},
		{admin, "developer", true},
		{admin, "admn", false},
	}
	for _, tc := range tests {
		f := a.Check(tc.p, tc.required, "")
		if (f == nil) != tc.allowed {
			t.Errorf("Check(%s, %s) = %+v, want allowed=%v", tc.p.Subject, tc.required, f, tc.allowed)
		}
	}
}

func TestCheckDeniesNilPrincipal(t *testing.T) {
	for _, roleChecks := range []bool{false, true} {
		a := NewAuthorizer(mustOrder(t, DefaultRoleOrder), roleChecks)
		if got := a.Role(nil); got != "" {
			t.Errorf("Role(nil) = %q with role checks %t, want no rank", got, roleChecks)
		}
		f := a.Check(nil, "viewer", "")
		if f == nil {
			t.Fatalf("nil principal allowed with role checks %t", roleChecks)
		}
		if f.RequiredRole != "viewer" || f.CurrentRole != "" {
			t.Errorf("Check(nil) = %+v with role checks %t", f, roleChecks)
		}
	}
}

func TestForbiddenResponse(t *testing.T) {
	a := NewAuthorizer(mustOrder(t, DefaultRoleOrder), true)
	f := a.Check(developer, "admin", "Disabling a module deletes its cloud resources.")
	if f == nil {
		t.Fatal("developer allowed an admin action")
	}
	rec := httptest.NewRecorder()
	WriteForbidden(rec, f)
	if rec.Code != http.StatusForbidden || rec.Header().Get("Content-Type") != "application/json" {
		t.Errorf("status %d, content type %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"error":        "forbidden",
		"requiredRole": "admin",
		"currentRole":  "developer",
		"reason":       "Disabling a module deletes its cloud resources.",
	}
	if len(body) != len(want) {
		t.Errorf("body = %v, want %v", body, want)
	}
	for k, v := range want {
		if body[k] != v {
			t.Errorf("body[%q] = %q, want %q", k, body[k], v)
		}
	}
	if f := a.Check(viewer, "developer", ""); f == nil || f.Reason != "Requires the developer role." {
		t.Errorf("default reason: %+v", f)
	}
}

func TestBearerToken(t *testing.T) {
	tests := map[string]bool{
		"Bearer aaa.bbb.ccc":   true,
		"bearer  aaa.bbb.ccc ": true,
		"":                     false,
		"aaa.bbb.ccc":          false,
		"Basic aaa.bbb.ccc":    false,
		"Bearer ":              false,
	}
	for header, ok := range tests {
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		if header != "" {
			r.Header.Set("Authorization", header)
		}
		raw, err := BearerToken(r)
		if (err == nil) != ok {
			t.Errorf("BearerToken(%q) = %q, %v; want ok=%v", header, raw, err, ok)
		}
		if ok && raw != "aaa.bbb.ccc" {
			t.Errorf("BearerToken(%q) = %q", header, raw)
		}
	}
}
