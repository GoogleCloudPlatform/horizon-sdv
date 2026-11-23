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
	"context"
	"crypto/rsa"
	"slices"
	"testing"
	"time"

	"github.com/acn-horizon-sdv/horizonauth/authtest"
)

func TestVerify(t *testing.T) {
	iss := authtest.NewIssuer(t)
	v, err := NewVerifier(context.Background(), iss.URL()+"/", "horizon-api")
	if err != nil {
		t.Fatal(err)
	}
	otherKey := authtest.NewKey(t)

	tests := []struct {
		name      string
		key       *rsa.PrivateKey
		claims    map[string]any
		wantErr   bool
		wantRoles []string
	}{
		{name: "portal developer", claims: iss.Claims(map[string]any{"resource_access": authtest.Roles("developer")}), wantRoles: []string{"developer"}},
		{name: "admin holds both roles", claims: iss.Claims(map[string]any{"resource_access": authtest.Roles("admin", "developer")}), wantRoles: []string{"admin", "developer"}},
		{name: "viewer has no client role", claims: iss.Claims(nil)},
		{name: "aud as a single string", claims: iss.Claims(map[string]any{"aud": "horizon-api"})},
		{name: "CI service account", claims: iss.Claims(map[string]any{"azp": "horizon-api-ci", "resource_access": authtest.Roles("developer")}), wantRoles: []string{"developer"}},
		{
			name: "flattened roles claim is ignored",
			claims: iss.Claims(map[string]any{
				"roles":           []string{"admin", "developer"},
				"resource_access": map[string]any{"argocd": map[string]any{"roles": []string{"admin"}}},
			}),
		},
		{
			name: "another client's entry is not decoded",
			claims: iss.Claims(map[string]any{"resource_access": map[string]any{
				"argocd":      map[string]any{"roles": "admin"},
				"horizon-api": map[string]any{"roles": []string{"developer"}},
			}}),
			wantRoles: []string{"developer"},
		},
		{name: "horizon-api roles are not a list", claims: iss.Claims(map[string]any{"resource_access": map[string]any{"horizon-api": map[string]any{"roles": "admin"}}}), wantErr: true},
		{name: "token issued for another application", claims: iss.Claims(map[string]any{"azp": "grafana", "aud": []string{"account"}}), wantErr: true},
		{name: "no aud", claims: iss.Claims(map[string]any{"aud": nil}), wantErr: true},
		{name: "expired", claims: iss.Claims(map[string]any{"exp": time.Now().Add(-time.Minute).Unix()}), wantErr: true},
		{name: "other issuer", claims: iss.Claims(map[string]any{"iss": "https://elsewhere.example/auth/realms/horizon"}), wantErr: true},
		{name: "signed with an unknown key", key: otherKey, claims: iss.Claims(nil), wantErr: true},
		{name: "ID token", claims: iss.Claims(map[string]any{"typ": "ID", "azp": "horizon-api"}), wantErr: true},
		{name: "no subject", claims: iss.Claims(map[string]any{"sub": nil}), wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			raw := iss.Token(t, tc.claims)
			if tc.key != nil {
				raw = authtest.Sign(t, tc.key, tc.claims)
			}
			p, err := v.Verify(context.Background(), raw)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("Verify succeeded, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("Verify: %v", err)
			}
			if !slices.Equal(p.ClientRoles, tc.wantRoles) {
				t.Errorf("ClientRoles = %v, want %v", p.ClientRoles, tc.wantRoles)
			}
			if p.Subject != tc.claims["sub"] || p.Username != "dev-alice" {
				t.Errorf("Principal = %+v", p)
			}
		})
	}
	if _, err := v.Verify(context.Background(), "opaque-token"); err == nil {
		t.Error("Verify accepted a token that is not a JWT")
	}
}

func TestVerifyCachesSigningKeys(t *testing.T) {
	iss := authtest.NewIssuer(t)
	v, err := NewVerifier(context.Background(), iss.URL(), "horizon-api")
	if err != nil {
		t.Fatal(err)
	}
	raw := iss.Token(t, iss.Claims(nil))
	for range 5 {
		if _, err := v.Verify(context.Background(), raw); err != nil {
			t.Fatal(err)
		}
	}
	if got := iss.KeyFetches(); got != 1 {
		t.Errorf("signing keys fetched %d times for 5 verifications, want 1", got)
	}
}

func TestNewVerifierRequiresClientID(t *testing.T) {
	iss := authtest.NewIssuer(t)
	if _, err := NewVerifier(context.Background(), iss.URL(), ""); err == nil {
		t.Error("empty client ID accepted, so no audience would be required")
	}
}

func TestHasClientRole(t *testing.T) {
	p := &Principal{ClientRoles: []string{"developer", "impersonate"}}
	if !p.HasClientRole("impersonate") || p.HasClientRole("admin") {
		t.Errorf("HasClientRole wrong for %v", p.ClientRoles)
	}
	var none *Principal
	if none.HasClientRole("impersonate") {
		t.Error("nil principal holds a client role")
	}
}
