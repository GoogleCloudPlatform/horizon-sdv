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

package wsapi

import (
	"sort"
	"strings"

	iampb "cloud.google.com/go/iam/apiv1/iampb"
	"google.golang.org/protobuf/proto"
)

const (
	RoleConfigAdmin     = "roles/workstations.admin"
	RoleWorkstationUser = "roles/workstations.user"
)

// ReconcileRoleBinding returns a policy whose single unconditional binding for
// role matches desired. Other roles and conditional bindings are copied through.
// allowGrant false only removes members; it does not add any.
// Member comparison is case-insensitive. The returned policy keeps the input version and etag.
func ReconcileRoleBinding(policy *iampb.Policy, role string, desired []string, allowGrant bool) (*iampb.Policy, bool) {
	var out *iampb.Policy
	if policy == nil {
		out = &iampb.Policy{Version: 3}
	} else {
		out = proto.Clone(policy).(*iampb.Policy)
	}

	wanted := normalizeMembers(desired)
	var current []string
	kept := make([]*iampb.Binding, 0, len(out.GetBindings()))
	for _, binding := range out.GetBindings() {
		if binding.GetRole() == role && binding.GetCondition() == nil {
			current = append(current, binding.GetMembers()...)
			continue
		}
		kept = append(kept, binding)
	}

	var next []string
	if allowGrant {
		next = wanted
	} else {
		seen := map[string]struct{}{}
		for _, member := range current {
			key := memberKey(member)
			if _, ok := seen[key]; ok {
				continue
			}
			if !containsMember(wanted, member) {
				continue
			}
			seen[key] = struct{}{}
			next = append(next, member)
		}
		sort.Strings(next)
	}

	changed := !sameMembers(current, next)
	if len(next) > 0 {
		kept = append(kept, &iampb.Binding{Role: role, Members: next})
	}
	out.Bindings = kept
	return out, changed
}

func normalizeMembers(members []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(members))
	for _, member := range members {
		member = strings.TrimSpace(member)
		if member == "" {
			continue
		}
		key := memberKey(member)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, member)
	}
	sort.Strings(out)
	return out
}

func sameMembers(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	a := normalizeMembers(left)
	b := normalizeMembers(right)
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if memberKey(a[i]) != memberKey(b[i]) {
			return false
		}
	}
	return true
}

func containsMember(members []string, candidate string) bool {
	key := memberKey(candidate)
	for _, member := range members {
		if memberKey(member) == key {
			return true
		}
	}
	return false
}

func memberKey(member string) string {
	return strings.ToLower(strings.TrimSpace(member))
}
