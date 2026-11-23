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

// Package horizonauth verifies Keycloak access tokens for Horizon services and ranks callers
// by their Horizon role. Horizon API and Module Manager share it, so both accept the same
// tokens, rank the same roles and return the same forbidden response.
package horizonauth

import (
	"fmt"
	"regexp"
	"strings"
)

// DefaultRoleOrder is the Horizon role order, lowest first.
const DefaultRoleOrder = "viewer,developer,admin"

var roleNamePattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// RoleOrder ranks roles from lowest to highest. Each role includes every role before it.
// The first role is implicit: every signed-in user holds at least that rank, without a client role.
type RoleOrder struct {
	names []string
	rank  map[string]int
}

// ParseRoleOrder parses a comma-separated role list, lowest first, e.g. "viewer,developer,admin".
func ParseRoleOrder(list string) (RoleOrder, error) {
	var names []string
	for _, n := range strings.Split(list, ",") {
		if n = strings.TrimSpace(n); n != "" {
			names = append(names, n)
		}
	}
	if len(names) == 0 {
		return RoleOrder{}, fmt.Errorf("role order is empty")
	}
	rank := make(map[string]int, len(names))
	for i, n := range names {
		if !roleNamePattern.MatchString(n) {
			return RoleOrder{}, fmt.Errorf("invalid role name %q in role order", n)
		}
		if _, dup := rank[n]; dup {
			return RoleOrder{}, fmt.Errorf("role %q appears twice in role order", n)
		}
		rank[n] = i
	}
	return RoleOrder{names: names, rank: rank}, nil
}

// String returns the order in the form "viewer < developer < admin".
func (o RoleOrder) String() string {
	return strings.Join(o.names, " < ")
}

// Highest returns the highest ranked role among clientRoles, or the implicit lowest role when
// none of them is ranked. Client roles that are not ranked (such as impersonate) are ignored.
func (o RoleOrder) Highest(clientRoles []string) string {
	best := 0
	for _, r := range clientRoles {
		if i, ok := o.rank[r]; ok && i > best {
			best = i
		}
	}
	return o.names[best]
}

// Includes reports whether current ranks at or above required. An unranked role includes nothing.
func (o RoleOrder) Includes(current, required string) bool {
	c, okC := o.rank[current]
	r, okR := o.rank[required]
	return okC && okR && c >= r
}
