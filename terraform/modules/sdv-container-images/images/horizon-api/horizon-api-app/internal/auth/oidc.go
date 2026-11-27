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

package auth

import (
	"context"
	"fmt"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
)

// OIDC verifies Keycloak-issued JWT access tokens (e.g. client_credentials and interactive users).
type OIDC struct {
	verifier *oidc.IDTokenVerifier
}

func NewOIDC(ctx context.Context, issuerURL, clientID string, skipClientIDCheck bool) (*OIDC, error) {
	provider, err := oidc.NewProvider(ctx, strings.TrimSuffix(issuerURL, "/"))
	if err != nil {
		return nil, fmt.Errorf("oidc provider: %w", err)
	}
	cfg := &oidc.Config{SkipClientIDCheck: skipClientIDCheck}
	if clientID != "" && !skipClientIDCheck {
		cfg.ClientID = clientID
	}
	return &OIDC{verifier: provider.Verifier(cfg)}, nil
}

// Principal is set after successful Bearer authentication (Keycloak access token).
type Principal struct {
	Subject string
	Kind    string // "oidc"
	// Roles are the Keycloak realm roles (realm_access.roles) of the token.
	Roles []string
}

// HasRole reports whether the principal holds the realm role.
func (p *Principal) HasRole(role string) bool {
	if p == nil {
		return false
	}
	for _, r := range p.Roles {
		if r == role {
			return true
		}
	}
	return false
}

func (o *OIDC) Verify(ctx context.Context, raw string) (*Principal, error) {
	tok, err := o.verifier.Verify(ctx, raw)
	if err != nil {
		return nil, err
	}
	var claims struct {
		Sub         string `json:"sub"`
		RealmAccess struct {
			Roles []string `json:"roles"`
		} `json:"realm_access"`
	}
	if err := tok.Claims(&claims); err != nil {
		return nil, err
	}
	return &Principal{Subject: claims.Sub, Kind: "oidc", Roles: claims.RealmAccess.Roles}, nil
}
