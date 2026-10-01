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

package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

// Client calls Horizon API with bearer token refresh on 401.
type Client struct {
	cfg        *Config
	httpClient *http.Client
	mu         sync.Mutex
	bearer     string
	// tokenCache is non-nil when using ~/.config/horizon/token.json (refresh can update file).
	tokenCache *TokenCache
	envToken   bool // HORIZON_ACCESS_TOKEN set; never refresh from Keycloak in-process for 401
}

func defaultHTTPClient() *http.Client {
	// Use DefaultTransport clone with HTTP/2 enabled (ALPN h2). An empty TLSNextProto map
	// disables h2 and breaks Keycloak/API calls behind modern ingress that speaks HTTP/2.
	return &http.Client{Transport: http.DefaultTransport.(*http.Transport).Clone()}
}

func newClient(cfg *Config) (*Client, error) {
	c := &Client{
		cfg:        cfg,
		httpClient: defaultHTTPClient(),
	}

	if t := strings.TrimSpace(os.Getenv("HORIZON_ACCESS_TOKEN")); t != "" {
		c.bearer = t
		c.envToken = true
		return c, nil
	}

	tc, err := loadTokenCache()
	if err != nil {
		return nil, err
	}
	if tc != nil && tc.AccessToken != "" && tc.KeycloakBase != "" {
		if !tc.IsExpired(90 * time.Second) {
			c.bearer = tc.AccessToken
			c.tokenCache = tc
			return c, nil
		}
		if tc.RefreshToken != "" {
			oauth := &OAuthClient{HTTP: c.httpClient}
			nt, err := oauth.RefreshToken(context.Background(), tc.KeycloakBase, tc.Realm, tc.ClientID, strings.TrimSpace(os.Getenv("KEYCLOAK_CLIENT_SECRET")), tc.RefreshToken)
			if err == nil {
				c.bearer = nt.AccessToken
				c.tokenCache = nt
				_ = saveTokenCache(nt)
				return c, nil
			}
		}
	}

	if sec := strings.TrimSpace(os.Getenv("KEYCLOAK_CLIENT_SECRET")); sec != "" {
		if cfg.KeycloakBase == "" {
			return nil, fmt.Errorf("KEYCLOAK_CLIENT_SECRET set but KEYCLOAK_BASE or HORIZON_DOMAIN is required")
		}
		clientID := envOr("KEYCLOAK_CLIENT_ID", "horizon-api-ci")
		oauth := &OAuthClient{HTTP: c.httpClient}
		nt, err := oauth.ClientCredentials(context.Background(), cfg.KeycloakBase, cfg.KeycloakRealm, clientID, sec)
		if err != nil {
			return nil, fmt.Errorf("client_credentials: %w", err)
		}
		c.bearer = nt.AccessToken
		_ = os.Setenv("HORIZON_ACCESS_TOKEN", nt.AccessToken)
		return c, nil
	}

	if tc != nil && tc.AccessToken != "" {
		c.bearer = tc.AccessToken
		c.tokenCache = tc
		return c, nil
	}

	return nil, fmt.Errorf("not authenticated: set HORIZON_ACCESS_TOKEN, KEYCLOAK_CLIENT_SECRET, or run `%s auth login`", progName())
}

func (c *Client) refreshBearer(ctx context.Context) error {
	if c.envToken {
		return fmt.Errorf("HORIZON_ACCESS_TOKEN rejected by server (401)")
	}
	if c.tokenCache != nil && c.tokenCache.RefreshToken != "" && c.tokenCache.KeycloakBase != "" {
		oauth := &OAuthClient{HTTP: c.httpClient}
		nt, err := oauth.RefreshToken(ctx, c.tokenCache.KeycloakBase, c.tokenCache.Realm, c.tokenCache.ClientID, strings.TrimSpace(os.Getenv("KEYCLOAK_CLIENT_SECRET")), c.tokenCache.RefreshToken)
		if err == nil {
			c.bearer = nt.AccessToken
			c.tokenCache = nt
			_ = saveTokenCache(nt)
			_ = os.Setenv("HORIZON_ACCESS_TOKEN", nt.AccessToken)
			return nil
		}
	}
	if sec := strings.TrimSpace(os.Getenv("KEYCLOAK_CLIENT_SECRET")); sec != "" && c.cfg.KeycloakBase != "" {
		clientID := envOr("KEYCLOAK_CLIENT_ID", "horizon-api-ci")
		oauth := &OAuthClient{HTTP: c.httpClient}
		nt, err := oauth.ClientCredentials(ctx, c.cfg.KeycloakBase, c.cfg.KeycloakRealm, clientID, sec)
		if err != nil {
			return err
		}
		c.bearer = nt.AccessToken
		_ = os.Setenv("HORIZON_ACCESS_TOKEN", nt.AccessToken)
		return nil
	}
	return fmt.Errorf("cannot refresh token: run `%s auth login` or set KEYCLOAK_CLIENT_SECRET", progName())
}

func (c *Client) doOnce(ctx context.Context, method, path string, body io.Reader, contentType string) ([]byte, int, error) {
	u, err := url.Parse(c.cfg.BaseURL + path)
	if err != nil {
		return nil, 0, err
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, 0, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("Accept", "application/json")
	if c.bearer != "" {
		req.Header.Set("Authorization", "Bearer "+c.bearer)
	}
	resp, err := c.httpClient.Do(req)
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

// DoJSON performs a request and retries once on 401 after refreshing the bearer token.
func (c *Client) DoJSON(ctx context.Context, method, path string, jsonBody []byte) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var body io.Reader
	ct := ""
	if jsonBody != nil {
		body = bytes.NewReader(jsonBody)
		ct = "application/json"
	}
	b, code, err := c.doOnce(ctx, method, path, body, ct)
	if err != nil {
		return nil, err
	}
	if code == http.StatusUnauthorized && !c.envToken {
		if err := c.refreshBearer(ctx); err != nil {
			return nil, err
		}
		if jsonBody != nil {
			body = bytes.NewReader(jsonBody)
		} else {
			body = nil
		}
		b, code, err = c.doOnce(ctx, method, path, body, ct)
		if err != nil {
			return nil, err
		}
	}
	if code < 200 || code > 299 {
		return nil, fmt.Errorf("%s %s: HTTP %d %s", method, path, code, truncate(string(b), 400))
	}
	return b, nil
}

// DoJSONWithHeaders is like DoJSON but merges extra HTTP headers (e.g. X-Horizon-Submitted-From).
func (c *Client) DoJSONWithHeaders(ctx context.Context, method, path string, jsonBody []byte, headers map[string]string) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var body io.Reader
	ct := ""
	if jsonBody != nil {
		body = bytes.NewReader(jsonBody)
		ct = "application/json"
	}
	b, code, err := c.doOnceWithHeaders(ctx, method, path, body, ct, headers)
	if err != nil {
		return nil, err
	}
	if code == http.StatusUnauthorized && !c.envToken {
		if err := c.refreshBearer(ctx); err != nil {
			return nil, err
		}
		if jsonBody != nil {
			body = bytes.NewReader(jsonBody)
		} else {
			body = nil
		}
		b, code, err = c.doOnceWithHeaders(ctx, method, path, body, ct, headers)
		if err != nil {
			return nil, err
		}
	}
	if code < 200 || code > 299 {
		return nil, fmt.Errorf("%s %s: HTTP %d %s", method, path, code, truncate(string(b), 400))
	}
	return b, nil
}

func (c *Client) doOnceWithHeaders(ctx context.Context, method, path string, body io.Reader, contentType string, headers map[string]string) ([]byte, int, error) {
	u, err := url.Parse(c.cfg.BaseURL + path)
	if err != nil {
		return nil, 0, err
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, 0, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("Accept", "application/json")
	for k, v := range headers {
		if strings.TrimSpace(k) != "" {
			req.Header.Set(k, v)
		}
	}
	if c.bearer != "" {
		req.Header.Set("Authorization", "Bearer "+c.bearer)
	}
	resp, err := c.httpClient.Do(req)
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

func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// GetJSON GETs a JSON resource.
func (c *Client) GetJSON(ctx context.Context, path string) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	b, code, err := c.doOnce(ctx, http.MethodGet, path, nil, "")
	if err != nil {
		return nil, err
	}
	if code == http.StatusUnauthorized && !c.envToken {
		if err := c.refreshBearer(ctx); err != nil {
			return nil, err
		}
		b, code, err = c.doOnce(ctx, http.MethodGet, path, nil, "")
		if err != nil {
			return nil, err
		}
	}
	if code < 200 || code > 299 {
		return nil, fmt.Errorf("GET %s: HTTP %d %s", path, code, truncate(string(b), 400))
	}
	return b, nil
}

// GetJSONRaw GETs a JSON resource and returns the raw status code alongside the
// body, allowing callers to handle specific non-2xx responses (e.g. 409) themselves.
// A 401 is still retried once after token refresh before returning.
func (c *Client) GetJSONRaw(ctx context.Context, path string) ([]byte, int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	b, code, err := c.doOnce(ctx, http.MethodGet, path, nil, "")
	if err != nil {
		return nil, 0, err
	}
	if code == http.StatusUnauthorized && !c.envToken {
		if err := c.refreshBearer(ctx); err != nil {
			return nil, code, err
		}
		b, code, err = c.doOnce(ctx, http.MethodGet, path, nil, "")
		if err != nil {
			return nil, 0, err
		}
	}
	return b, code, nil
}

// StreamGET opens a GET response for streaming (caller must close body).
func (c *Client) StreamGET(ctx context.Context, rawURL string) (*http.Response, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	do := func() (*http.Response, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/x-ndjson")
		req.Header.Set("Accept-Encoding", "identity")
		if c.bearer != "" {
			req.Header.Set("Authorization", "Bearer "+c.bearer)
		}
		return c.httpClient.Do(req)
	}
	resp, err := do()
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusUnauthorized && !c.envToken {
		resp.Body.Close()
		if err := c.refreshBearer(ctx); err != nil {
			return nil, err
		}
		resp, err = do()
		if err != nil {
			return nil, err
		}
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("stream GET: HTTP %d %s", resp.StatusCode, truncate(string(b), 400))
	}
	return resp, nil
}

// GetURLPlain performs an HTTP GET without adding Authorization (e.g. GCS V4 signed URL).
func GetURLPlain(ctx context.Context, rawURL string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	return defaultHTTPClient().Do(req)
}
