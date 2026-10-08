// Copyright (c) 2026 Accenture, All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
//
//         http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package horizon

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type tokenEndpointResponse struct {
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	ExpiresIn        int    `json:"expires_in"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

func tokenURL(keycloakBase, realm string) string {
	return strings.TrimSuffix(keycloakBase, "/") + "/realms/" + realm + "/protocol/openid-connect/token"
}

func authBase(keycloakBase, realm string) string {
	return strings.TrimSuffix(keycloakBase, "/") + "/realms/" + realm + "/protocol/openid-connect"
}

// OAuthClient performs Keycloak OAuth2 calls (stdlib only).
type OAuthClient struct {
	HTTP *http.Client
}

func (o *OAuthClient) postForm(ctx context.Context, u string, data url.Values) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	var client *http.Client
	if o.HTTP != nil {
		client = o.HTTP
	} else {
		client = DefaultHTTPClient()
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return b, resp.StatusCode, nil
}

func (o *OAuthClient) ClientCredentials(ctx context.Context, keycloakBase, realm, clientID, clientSecret string) (*TokenCache, error) {
	u := tokenURL(keycloakBase, realm)
	data := url.Values{}
	data.Set("grant_type", "client_credentials")
	data.Set("client_id", clientID)
	data.Set("client_secret", clientSecret)
	b, code, err := o.postForm(ctx, u, data)
	if err != nil {
		return nil, err
	}
	if code < 200 || code > 299 {
		return nil, fmt.Errorf("oauth: HTTP %d %s", code, truncate(string(b), 300))
	}
	var tr tokenEndpointResponse
	if err := json.Unmarshal(b, &tr); err != nil {
		return nil, err
	}
	if tr.AccessToken == "" {
		return nil, fmt.Errorf("oauth: %s %s", tr.Error, tr.ErrorDescription)
	}
	return tokenFromResponse(&tr, keycloakBase, realm, clientID), nil
}

func (o *OAuthClient) RefreshToken(ctx context.Context, keycloakBase, realm, clientID, clientSecret, refresh string) (*TokenCache, error) {
	u := tokenURL(keycloakBase, realm)
	data := url.Values{}
	data.Set("grant_type", "refresh_token")
	data.Set("client_id", clientID)
	data.Set("refresh_token", refresh)
	if clientSecret != "" {
		data.Set("client_secret", clientSecret)
	}
	b, code, err := o.postForm(ctx, u, data)
	if err != nil {
		return nil, err
	}
	if code < 200 || code > 299 {
		return nil, fmt.Errorf("oauth refresh: HTTP %d %s", code, truncate(string(b), 300))
	}
	var tr tokenEndpointResponse
	if err := json.Unmarshal(b, &tr); err != nil {
		return nil, err
	}
	if tr.AccessToken == "" {
		return nil, fmt.Errorf("oauth refresh: %s %s", tr.Error, tr.ErrorDescription)
	}
	t := tokenFromResponse(&tr, keycloakBase, realm, clientID)
	if tr.RefreshToken != "" {
		t.RefreshToken = tr.RefreshToken
	} else {
		t.RefreshToken = refresh
	}
	return t, nil
}

func tokenFromResponse(tr *tokenEndpointResponse, keycloakBase, realm, clientID string) *TokenCache {
	t := &TokenCache{
		AccessToken:  tr.AccessToken,
		RefreshToken: tr.RefreshToken,
		KeycloakBase: strings.TrimSuffix(keycloakBase, "/"),
		Realm:        realm,
		ClientID:     clientID,
	}
	if tr.ExpiresIn > 0 {
		t.ExpiresAt = time.Now().Unix() + int64(tr.ExpiresIn)
	}
	return t
}

func newPKCE() (verifier string, challenge string, err error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", err
	}
	verifier = base64.RawURLEncoding.EncodeToString(raw)
	h := sha256.Sum256([]byte(verifier))
	challenge = base64.RawURLEncoding.EncodeToString(h[:])
	return verifier, challenge, nil
}

type deviceAuthStart struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	Interval                int    `json:"interval"`
	ExpiresIn               int    `json:"expires_in"`
	Error                   string `json:"error"`
	ErrorDescription        string `json:"error_description"`
}

// DeviceFlow runs OAuth 2.0 device authorization with PKCE (S256).
func (o *OAuthClient) DeviceFlow(ctx context.Context, keycloakBase, realm, clientID, scope string, openBrowser func(url string) error) (*TokenCache, error) {
	verifier, challenge, err := newPKCE()
	if err != nil {
		return nil, err
	}
	ab := authBase(keycloakBase, realm) + "/auth/device"
	data := url.Values{}
	data.Set("client_id", clientID)
	if scope == "" {
		scope = "openid"
	}
	data.Set("scope", scope)
	data.Set("code_challenge_method", "S256")
	data.Set("code_challenge", challenge)
	b, code, err := o.postForm(ctx, ab, data)
	if err != nil {
		return nil, err
	}
	if code < 200 || code > 299 {
		return nil, fmt.Errorf("device auth start: HTTP %d %s", code, truncate(string(b), 300))
	}
	var da deviceAuthStart
	if err := json.Unmarshal(b, &da); err != nil {
		return nil, err
	}
	if da.DeviceCode == "" {
		return nil, fmt.Errorf("device auth: %s %s", da.Error, da.ErrorDescription)
	}
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Open this URL in a browser (sign in with your Horizon user):")
	if da.VerificationURIComplete != "" {
		fmt.Fprintln(os.Stderr, " ", da.VerificationURIComplete)
		if openBrowser != nil {
			_ = openBrowser(da.VerificationURIComplete)
		}
	} else {
		fmt.Fprintln(os.Stderr, " ", da.VerificationURI)
		fmt.Fprintln(os.Stderr, "  Enter code:", da.UserCode)
		if openBrowser != nil && da.VerificationURI != "" {
			_ = openBrowser(da.VerificationURI)
		}
	}
	fmt.Fprintln(os.Stderr, "")
	interval := time.Duration(da.Interval) * time.Second
	if interval <= 0 {
		interval = 5 * time.Second
	}
	expires := time.Duration(da.ExpiresIn) * time.Second
	if expires <= 0 {
		expires = 10 * time.Minute
	}
	deadline := time.Now().Add(expires)
	tokURL := tokenURL(keycloakBase, realm)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(interval):
		}
		poll := url.Values{}
		poll.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")
		poll.Set("client_id", clientID)
		poll.Set("device_code", da.DeviceCode)
		poll.Set("code_verifier", verifier)
		rb, _, err := o.postForm(ctx, tokURL, poll)
		if err != nil {
			continue
		}
		var tr tokenEndpointResponse
		if err := json.Unmarshal(rb, &tr); err != nil {
			continue
		}
		if tr.AccessToken != "" {
			return tokenFromResponse(&tr, keycloakBase, realm, clientID), nil
		}
		switch tr.Error {
		case "authorization_pending", "":
			continue
		case "slow_down":
			interval += 5 * time.Second
			continue
		case "access_denied", "expired_token":
			return nil, fmt.Errorf("device authorization %s: %s", tr.Error, tr.ErrorDescription)
		default:
			if tr.Error != "" {
				return nil, fmt.Errorf("device token: %s %s", tr.Error, tr.ErrorDescription)
			}
		}
	}
	return nil, fmt.Errorf("device authorization timed out")
}
