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
	"testing"

	iampb "cloud.google.com/go/iam/apiv1/iampb"
	expr "google.golang.org/genproto/googleapis/type/expr"
)

func TestReconcileRoleBindingReplacesOnlyTheUnconditionalRole(t *testing.T) {
	policy := &iampb.Policy{
		Version: 3,
		Etag:    []byte("v3"),
		Bindings: []*iampb.Binding{
			{Role: RoleConfigAdmin, Members: []string{"user:old@example.com", "user:keep@example.com"}},
			{Role: "roles/workstations.user", Members: []string{"user:other@example.com"}},
			{
				Role:      RoleConfigAdmin,
				Members:   []string{"user:conditional@example.com"},
				Condition: &expr.Expr{Expression: "request.time < timestamp('2030-01-01T00:00:00Z')"},
			},
		},
	}
	next, changed := ReconcileRoleBinding(policy, RoleConfigAdmin, []string{"user:Keep@example.com", "user:new@example.com"}, true)
	if !changed {
		t.Fatal("expected a change")
	}
	if next.GetVersion() != 3 || string(next.GetEtag()) != "v3" {
		t.Fatalf("version/etag = %d %q", next.GetVersion(), next.GetEtag())
	}
	var managed, conditional, other int
	for _, binding := range next.GetBindings() {
		switch {
		case binding.GetRole() == RoleConfigAdmin && binding.GetCondition() == nil:
			managed++
			if !sameMembers(binding.GetMembers(), []string{"user:Keep@example.com", "user:new@example.com"}) {
				t.Fatalf("managed members = %v", binding.GetMembers())
			}
		case binding.GetRole() == RoleConfigAdmin && binding.GetCondition() != nil:
			conditional++
			if binding.GetMembers()[0] != "user:conditional@example.com" {
				t.Fatalf("conditional binding changed: %+v", binding)
			}
		case binding.GetRole() == "roles/workstations.user":
			other++
		default:
			t.Fatalf("unexpected binding %+v", binding)
		}
	}
	if managed != 1 || conditional != 1 || other != 1 {
		t.Fatalf("binding counts managed=%d conditional=%d other=%d", managed, conditional, other)
	}
	if len(policy.GetBindings()[0].GetMembers()) != 2 {
		t.Fatal("input policy was mutated")
	}
}

func TestReconcileRoleBindingEmptyDesiredRemovesBinding(t *testing.T) {
	policy := &iampb.Policy{Version: 1, Etag: []byte("e"), Bindings: []*iampb.Binding{
		{Role: RoleWorkstationUser, Members: []string{"user:alice@example.com"}},
	}}
	next, changed := ReconcileRoleBinding(policy, RoleWorkstationUser, nil, true)
	if !changed {
		t.Fatal("expected removal")
	}
	if len(next.GetBindings()) != 0 {
		t.Fatalf("bindings = %+v", next.GetBindings())
	}
	if next.GetVersion() != 1 || string(next.GetEtag()) != "e" {
		t.Fatalf("version/etag not preserved: %+v", next)
	}
}

func TestReconcileRoleBindingGrantDisabledOnlyRevokes(t *testing.T) {
	policy := &iampb.Policy{Bindings: []*iampb.Binding{{
		Role:    RoleWorkstationUser,
		Members: []string{"user:alice@example.com", "user:bob@example.com"},
	}}}
	next, changed := ReconcileRoleBinding(policy, RoleWorkstationUser, []string{"user:Alice@example.com", "user:cara@example.com"}, false)
	if !changed {
		t.Fatal("expected bob to be revoked")
	}
	members := next.GetBindings()[0].GetMembers()
	if len(members) != 1 || memberKey(members[0]) != "user:alice@example.com" {
		t.Fatalf("members = %v", members)
	}
}

func TestReconcileRoleBindingCaseOnlyDifferenceIsUnchanged(t *testing.T) {
	policy := &iampb.Policy{Bindings: []*iampb.Binding{{
		Role:    RoleConfigAdmin,
		Members: []string{"user:alice@example.com"},
	}}}
	next, changed := ReconcileRoleBinding(policy, RoleConfigAdmin, []string{"user:Alice@example.com"}, true)
	if changed {
		t.Fatalf("case-only difference was treated as drift: %+v", next.GetBindings())
	}
}
