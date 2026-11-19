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

import "fmt"

// Authorizer ranks callers and applies the minimum role each endpoint declares.
type Authorizer struct {
	order      RoleOrder
	roleChecks bool
}

// NewAuthorizer returns an Authorizer. With roleChecks off, Check allows every signed-in caller.
func NewAuthorizer(order RoleOrder, roleChecks bool) *Authorizer {
	return &Authorizer{order: order, roleChecks: roleChecks}
}

// Role returns the caller's rank. It is computed whether or not role checks are on.
func (a *Authorizer) Role(p *Principal) string {
	return a.order.Highest(p.ClientRoles)
}

// Check returns nil when p may call an endpoint whose minimum role is required, otherwise the
// forbidden response to send. reason describes the action for the caller; empty uses a default.
// A required role that is not in the order denies everyone, so a typo cannot open an endpoint.
func (a *Authorizer) Check(p *Principal, required, reason string) *Forbidden {
	if !a.roleChecks {
		return nil
	}
	current := a.Role(p)
	if a.order.Includes(current, required) {
		return nil
	}
	if reason == "" {
		reason = fmt.Sprintf("Requires the %s role.", required)
	}
	return &Forbidden{Code: "forbidden", RequiredRole: required, CurrentRole: current, Reason: reason}
}
