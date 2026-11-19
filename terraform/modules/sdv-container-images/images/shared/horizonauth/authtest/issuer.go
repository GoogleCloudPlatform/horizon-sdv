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

// Package authtest runs a fake Keycloak realm for tests of services that use horizonauth.
package authtest

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
)

// Issuer serves a discovery document and signing keys, and signs access tokens.
type Issuer struct {
	srv        *httptest.Server
	key        *rsa.PrivateKey
	keyFetches atomic.Int32
}

// NewIssuer starts an issuer that is stopped when the test ends.
func NewIssuer(t testing.TB) *Issuer {
	t.Helper()
	i := &Issuer{key: NewKey(t)}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                                i.srv.URL,
			"jwks_uri":                              i.srv.URL + "/certs",
			"authorization_endpoint":                i.srv.URL + "/auth",
			"token_endpoint":                        i.srv.URL + "/token",
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	})
	mux.HandleFunc("GET /certs", func(w http.ResponseWriter, r *http.Request) {
		i.keyFetches.Add(1)
		_ = json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{
			{Key: &i.key.PublicKey, KeyID: "k1", Algorithm: "RS256", Use: "sig"},
		}})
	})
	i.srv = httptest.NewServer(mux)
	t.Cleanup(i.srv.Close)
	return i
}

// NewKey returns a fresh RSA signing key.
func NewKey(t testing.TB) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return key
}

// URL is the issuer URL to pass to horizonauth.NewVerifier.
func (i *Issuer) URL() string {
	return i.srv.URL
}

// KeyFetches counts requests for the signing keys.
func (i *Issuer) KeyFetches() int {
	return int(i.keyFetches.Load())
}

// Claims returns the claims of a developer portal access token for a user with no Horizon role.
// Each override replaces a claim; a nil value removes it.
func (i *Issuer) Claims(overrides map[string]any) map[string]any {
	now := time.Now()
	c := map[string]any{
		"iss":                i.srv.URL,
		"sub":                "c7eae7a2-0420-4942-9a83-492a87401631",
		"typ":                "Bearer",
		"azp":                "horizon-dev-portal",
		"aud":                []string{"horizon-api", "account"},
		"preferred_username": "dev-alice",
		"iat":                now.Unix(),
		"exp":                now.Add(5 * time.Minute).Unix(),
	}
	for k, v := range overrides {
		if v == nil {
			delete(c, k)
		} else {
			c[k] = v
		}
	}
	return c
}

// Token signs claims with the issuer's key.
func (i *Issuer) Token(t testing.TB, claims map[string]any) string {
	t.Helper()
	return Sign(t, i.key, claims)
}

// Roles returns a resource_access claim with the given horizon-api client roles.
func Roles(names ...string) map[string]any {
	return map[string]any{"horizon-api": map[string]any{"roles": names}}
}

// Sign signs claims as an RS256 JWT with key ID k1.
func Sign(t testing.TB, key *rsa.PrivateKey, claims map[string]any) string {
	t.Helper()
	signer, err := jose.NewSigner(
		jose.SigningKey{Algorithm: jose.RS256, Key: jose.JSONWebKey{Key: key, KeyID: "k1"}},
		(&jose.SignerOptions{}).WithType("JWT"),
	)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	obj, err := signer.Sign(payload)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := obj.CompactSerialize()
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
