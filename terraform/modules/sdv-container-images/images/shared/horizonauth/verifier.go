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
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
)

// Principal is the caller of a verified access token.
type Principal struct {
	// Subject is the Keycloak user ID (sub). For a service account it is the service-account user.
	Subject string
	// Username is preferred_username, for logs and display only.
	Username string
	// ClientRoles are the caller's roles from resource_access.<verifier client>.roles.
	ClientRoles []string
}

// HasClientRole reports whether the caller holds a client role, ranked or not (e.g. impersonate).
func (p *Principal) HasClientRole(name string) bool {
	return slices.Contains(p.ClientRoles, name)
}

// Verifier checks Keycloak access tokens locally against the realm's signing keys.
type Verifier struct {
	verifier *oidc.IDTokenVerifier
	clientID string
}

// NewVerifier reads the issuer's discovery document once. go-oidc caches the realm's signing
// keys and fetches them again only when a token's signature matches none of the cached keys
// (key rotation)
//
// clientID is the Keycloak client the token must be meant for: it must appear in aud, and the
// caller's roles are read from resource_access.<clientID>.roles only.
func NewVerifier(ctx context.Context, issuerURL, clientID string) (*Verifier, error) {
	if clientID == "" {
		return nil, errors.New("OIDC client ID (required token audience) is required")
	}
	provider, err := oidc.NewProvider(ctx, strings.TrimSuffix(issuerURL, "/"))
	if err != nil {
		return nil, fmt.Errorf("oidc provider: %w", err)
	}
	return &Verifier{
		verifier: provider.Verifier(&oidc.Config{ClientID: clientID}),
		clientID: clientID,
	}, nil
}

// Verify checks the signature, issuer, expiry and audience of raw and returns its caller.
func (v *Verifier) Verify(ctx context.Context, raw string) (*Principal, error) {
	tok, err := v.verifier.Verify(ctx, raw)
	if err != nil {
		return nil, err
	}
	var claims struct {
		Typ               string                     `json:"typ"`
		Sub               string                     `json:"sub"`
		PreferredUsername string                     `json:"preferred_username"`
		ResourceAccess    map[string]json.RawMessage `json:"resource_access"`
	}
	if err := tok.Claims(&claims); err != nil {
		return nil, fmt.Errorf("token claims: %w", err)
	}
	// An ID token issued to the CLI client also has aud horizon-api; accept access tokens only.
	if claims.Typ != "Bearer" {
		return nil, fmt.Errorf("not an access token (typ %q)", claims.Typ)
	}
	if claims.Sub == "" {
		return nil, errors.New("token has no subject")
	}
	// Only this client's entry is decoded; the roles listed for other clients are never read.
	var access struct {
		Roles []string `json:"roles"`
	}
	if entry, ok := claims.ResourceAccess[v.clientID]; ok {
		if err := json.Unmarshal(entry, &access); err != nil {
			return nil, fmt.Errorf("token resource_access.%s: %w", v.clientID, err)
		}
	}
	return &Principal{
		Subject:     claims.Sub,
		Username:    claims.PreferredUsername,
		ClientRoles: access.Roles,
	}, nil
}
