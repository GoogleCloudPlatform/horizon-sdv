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

import "testing"

func mustOrder(t *testing.T, list string) RoleOrder {
	t.Helper()
	o, err := ParseRoleOrder(list)
	if err != nil {
		t.Fatal(err)
	}
	return o
}

func TestParseRoleOrder(t *testing.T) {
	o := mustOrder(t, " viewer, developer ,admin,")
	if o.String() != "viewer < developer < admin" {
		t.Errorf("String = %q", o.String())
	}
	for _, bad := range []string{"", " , ", "viewer,viewer", "viewer,Admin", "viewer,dev ops"} {
		if _, err := ParseRoleOrder(bad); err == nil {
			t.Errorf("ParseRoleOrder(%q) accepted", bad)
		}
	}
}

func TestHighest(t *testing.T) {
	o := mustOrder(t, DefaultRoleOrder)
	tests := []struct {
		roles []string
		want  string
	}{
		{nil, "viewer"},
		{[]string{"developer"}, "developer"},
		{[]string{"admin", "developer"}, "admin"},
		{[]string{"admin"}, "admin"},
		{[]string{"impersonate"}, "viewer"},
		{[]string{"impersonate", "developer"}, "developer"},
	}
	for _, tc := range tests {
		if got := o.Highest(tc.roles); got != tc.want {
			t.Errorf("Highest(%v) = %q, want %q", tc.roles, got, tc.want)
		}
	}
}

func TestIncludes(t *testing.T) {
	o := mustOrder(t, DefaultRoleOrder)
	allowed := map[[2]string]bool{
		{"viewer", "viewer"}: true, {"viewer", "developer"}: false, {"viewer", "admin"}: false,
		{"developer", "viewer"}: true, {"developer", "developer"}: true, {"developer", "admin"}: false,
		{"admin", "viewer"}: true, {"admin", "developer"}: true, {"admin", "admin"}: true,
		{"admin", "impersonate"}: false, {"impersonate", "viewer"}: false,
	}
	for pair, want := range allowed {
		if got := o.Includes(pair[0], pair[1]); got != want {
			t.Errorf("Includes(%q, %q) = %v, want %v", pair[0], pair[1], got, want)
		}
	}
}

func TestInsertedRoleNeedsNoCodeChange(t *testing.T) {
	o := mustOrder(t, "viewer,developer,maintainer,admin")
	if got := o.Highest([]string{"developer", "maintainer"}); got != "maintainer" {
		t.Errorf("Highest = %q, want maintainer", got)
	}
	if !o.Includes("maintainer", "developer") || o.Includes("maintainer", "admin") {
		t.Error("maintainer is not ranked between developer and admin")
	}
}
