// Copyright (c) 2024-2026 Accenture, All Rights Reserved.
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

// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
)

//go:embed all:dist
var staticFS embed.FS

// Renew access token this long before JWT expiry (proactive refresh / refresh_token exchange).
const ciTokenRefreshLead = 90 * time.Second

// If Keycloak omits refresh_expires_in, we still try refresh until the server rejects it.
const ciRefreshMinRemaining = 30 * time.Second

// parseJWTExpiry returns access token exp (wall clock). Used to cap cache lifetime so we never
// send a JWT past exp when expires_in and cluster clocks disagree with Horizon's validation.
func parseJWTExpiry(accessToken string) (time.Time, bool) {
	parts := strings.Split(accessToken, ".")
	if len(parts) < 2 {
		return time.Time{}, false
	}
	payload := parts[1]
	raw, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		l := len(payload) % 4
		if l > 0 {
			payload += strings.Repeat("=", 4-l)
		}
		raw, err = base64.URLEncoding.DecodeString(payload)
		if err != nil {
			return time.Time{}, false
		}
	}
	var claims struct {
		Exp int64 `json:"exp"`
	}
	if err := json.Unmarshal(raw, &claims); err != nil || claims.Exp == 0 {
		return time.Time{}, false
	}
	return time.Unix(claims.Exp, 0), true
}

func main() {
	addr := env("LISTEN_ADDR", ":8080")
	issuer := mustEnv("OIDC_ISSUER_URL")
	tokenURL := env("KEYCLOAK_TOKEN_URL", "")
	if tokenURL == "" {
		tokenURL = strings.TrimSuffix(issuer, "/") + "/protocol/openid-connect/token"
	}
	mmBase := mustEnv("MODULE_MANAGER_BASE_URL")
	haBase := mustEnv("HORIZON_API_BASE_URL")
	ciID := mustEnv("HORIZON_API_CI_CLIENT_ID")
	ciSecret := mustEnv("HORIZON_API_CI_CLIENT_SECRET")

	ctx := context.Background()
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		log.Fatalf("oidc provider: %v", err)
	}
	verifier := provider.Verifier(&oidc.Config{SkipClientIDCheck: true})

	mmURL := mustParseURL(mmBase)
	haURL := mustParseURL(haBase)
	ci := newCITokenSource(tokenURL, ciID, ciSecret)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /config.js", func(w http.ResponseWriter, r *http.Request) {
		serveConfigJS(w, env("PUBLIC_BASE_URL", ""), normalizePublicPath(env("PUBLIC_PATH", "")), env("KEYCLOAK_TOKEN_PATH", "/auth/realms/horizon/protocol/openid-connect/token"), env("KEYCLOAK_CLIENT_ID", "horizon-dev-portal"), ciID)
	})
	mux.Handle("/api/mm/", newMMProxy(mmURL, verifier))
	mux.Handle("/api/horizon/", newHorizonProxy(haURL, ci))
	var cliVer bearerVerifier = oidcBearerVerifier{inner: verifier}
	registerCLIRoutes(mux, env("CLI_DIST_DIR", "/cli-dist"), cliVer)
	mux.Handle("/", spaHandler(htmlBaseTagHref(normalizePublicPath(env("PUBLIC_PATH", "")))))

	log.Printf("listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func env(k, def string) string {
	if v := strings.TrimSpace(os.Getenv(k)); v != "" {
		return v
	}
	return def
}

func mustEnv(k string) string {
	v := strings.TrimSpace(os.Getenv(k))
	if v == "" {
		log.Fatalf("missing env %s", k)
	}
	return v
}

func mustParseURL(raw string) *url.URL {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		log.Fatalf("bad URL: %s", raw)
	}
	return u
}

func normalizePublicPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	p = strings.TrimSuffix(p, "/")
	if p == "/" {
		return ""
	}
	return p
}

// htmlBaseTagHref is the value for <base href="..."> when the SPA is mounted under a subpath.
// It must end with "/" so relative URLs in index.html resolve under the mount (Vite uses base: './').
func htmlBaseTagHref(normalizedPublicPath string) string {
	if normalizedPublicPath == "" {
		return ""
	}
	return normalizedPublicPath + "/"
}

func injectHTMLBaseTag(html []byte, baseHref string) []byte {
	if baseHref == "" {
		return html
	}
	s := string(html)
	lower := strings.ToLower(s)
	idx := strings.Index(lower, "<head>")
	if idx < 0 {
		return html
	}
	insertAt := idx + len("<head>")
	inject := "\n    <base href=\"" + htmlEscapeAttr(baseHref) + "\">"
	return []byte(s[:insertAt] + inject + s[insertAt:])
}

func htmlEscapeAttr(s string) string {
	s = strings.ReplaceAll(s, `&`, `&amp;`)
	s = strings.ReplaceAll(s, `"`, `&quot;`)
	return s
}

func serveConfigJS(w http.ResponseWriter, publicBase, publicPath, keycloakTokenPath, clientID, horizonApiOAuthClientID string) {
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	cfg := map[string]string{
		"baseUrl":          publicBase,
		"publicPath":       publicPath,
		"keycloakUrl":      keycloakTokenPath,
		"keycloakClientId": clientID,
	}
	if strings.TrimSpace(horizonApiOAuthClientID) != "" {
		cfg["horizonApiOAuthClientId"] = strings.TrimSpace(horizonApiOAuthClientID)
	}
	payload, err := json.Marshal(cfg)
	if err != nil {
		http.Error(w, "config", http.StatusInternalServerError)
		return
	}
	_, _ = fmt.Fprintf(w, "window.APP_CONFIG = window.APP_CONFIG || %s;\n", string(payload))
}

func bearerToken(r *http.Request) (string, error) {
	h := strings.TrimSpace(r.Header.Get("Authorization"))
	parts := strings.SplitN(h, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", fmt.Errorf("missing bearer")
	}
	t := strings.TrimSpace(parts[1])
	if t == "" {
		return "", fmt.Errorf("empty bearer")
	}
	return t, nil
}

func newMMProxy(upstream *url.URL, verifier *oidc.IDTokenVerifier) http.Handler {
	rp := httputil.NewSingleHostReverseProxy(upstream)
	rp.Director = func(req *http.Request) {
		req.URL.Scheme = upstream.Scheme
		req.URL.Host = upstream.Host
		orig := req.URL.Path
		req.URL.Path = strings.TrimPrefix(orig, "/api/mm")
		if req.URL.Path == "" {
			req.URL.Path = "/"
		}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := bearerToken(r)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if _, err := verifier.Verify(r.Context(), raw); err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		rp.ServeHTTP(w, r)
	})
}

// retryCI401Transport retries GET/HEAD after upstream 401: ci.invalidate() then ci.get() prefers
// refresh_token (if Keycloak issued one), else client_credentials.
type retryCI401Transport struct {
	rt http.RoundTripper
	ci *ciTokenSource
}

func (t *retryCI401Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.rt
	if base == nil {
		base = http.DefaultTransport
	}
	retry401 := req.Method == http.MethodGet || req.Method == http.MethodHead || req.GetBody != nil
	if !retry401 {
		return base.RoundTrip(req)
	}
	const max401Attempts = 2
	cur := req
	for attempt := 0; attempt < max401Attempts; attempt++ {
		resp, err := base.RoundTrip(cur)
		if err != nil || resp == nil {
			return resp, err
		}
		if resp.StatusCode != http.StatusUnauthorized {
			return resp, err
		}
		if attempt == max401Attempts-1 {
			return resp, err
		}
		t.ci.invalidate()
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
		tok, err := t.ci.get(req.Context())
		if err != nil {
			return nil, err
		}
		r2 := req.Clone(req.Context())
		if r2.Header != nil {
			r2.Header = r2.Header.Clone()
		}
		r2.Header.Set("Authorization", "Bearer "+tok)
		if req.Method == http.MethodGet || req.Method == http.MethodHead {
			r2.Body = nil
		} else {
			body, err := req.GetBody()
			if err != nil {
				return nil, err
			}
			r2.Body = body
		}
		cur = r2
	}
	return nil, fmt.Errorf("retryCI401Transport: internal")
}

// newHorizonProxy forwards to Horizon API using only the confidential CI client (K8s secret).
// The browser does not send a user Bearer; ingress same-origin still limits who reaches this service.
func newHorizonProxy(upstream *url.URL, ci *ciTokenSource) http.Handler {
	rp := httputil.NewSingleHostReverseProxy(upstream)
	rp.Transport = &retryCI401Transport{rt: http.DefaultTransport, ci: ci}
	// Chunked NDJSON log streams: periodic flush so the browser sees lines without buffering the whole body.
	rp.FlushInterval = 100 * time.Millisecond
	rp.Director = func(req *http.Request) {
		req.URL.Scheme = upstream.Scheme
		req.URL.Host = upstream.Host
		orig := req.URL.Path
		req.URL.Path = strings.TrimPrefix(orig, "/api/horizon")
		if req.URL.Path == "" {
			req.URL.Path = "/"
		}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tok, err := ci.get(r.Context())
		if err != nil {
			http.Error(w, "token exchange failed", http.StatusBadGateway)
			return
		}
		r2 := r.Clone(r.Context())
		r2.Header.Del("Authorization")
		r2.Header.Set("Authorization", "Bearer "+tok)

		// Allow retryCI401Transport to replay POST/PUT/PATCH/DELETE after refreshing the CI token (same as GET catalog).
		if r2.Body != nil && (r2.Method == http.MethodPost || r2.Method == http.MethodPut || r2.Method == http.MethodPatch || r2.Method == http.MethodDelete) {
			const maxBody = 4 << 20
			b, err := io.ReadAll(io.LimitReader(r2.Body, maxBody+1))
			_ = r2.Body.Close()
			if err != nil {
				http.Error(w, "read body", http.StatusBadRequest)
				return
			}
			if len(b) > maxBody {
				http.Error(w, "body too large", http.StatusRequestEntityTooLarge)
				return
			}
			buf := bytes.Clone(b)
			r2.GetBody = func() (io.ReadCloser, error) {
				return io.NopCloser(bytes.NewReader(buf)), nil
			}
			body, err := r2.GetBody()
			if err != nil {
				http.Error(w, "body", http.StatusInternalServerError)
				return
			}
			r2.Body = body
			r2.ContentLength = int64(len(buf))
			r2.Header.Del("Transfer-Encoding")
		}

		rp.ServeHTTP(w, r2)
	})
}

type ciTokenSource struct {
	mu        sync.Mutex
	token     string
	expiresAt time.Time

	refreshTok string
	refreshExp time.Time // zero if Keycloak did not send refresh_expires_in

	tokenURL string
	id       string
	secret   string
}

func newCITokenSource(tokenURL, id, secret string) *ciTokenSource {
	return &ciTokenSource{tokenURL: tokenURL, id: id, secret: secret}
}

// invalidate clears only the access token so the next get() can try refresh_token before client_credentials.
func (c *ciTokenSource) invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.token = ""
	c.expiresAt = time.Time{}
}

type tokenEndpointJSON struct {
	AccessToken      string `json:"access_token"`
	ExpiresIn        int    `json:"expires_in"`
	RefreshToken     string `json:"refresh_token"`
	RefreshExpiresIn int    `json:"refresh_expires_in"`
}

// clearRefreshWhenMissing: for client_credentials, drop stored refresh if IdP omits refresh_token.
// For refresh_token grant, false preserves the previous refresh when the response does not rotate it.
func (c *ciTokenSource) applyTokenResponse(tr *tokenEndpointJSON, clearRefreshWhenMissing bool) {
	c.token = tr.AccessToken
	sec := tr.ExpiresIn
	if sec <= 0 {
		sec = 300
	}
	deadline := time.Now().Add(time.Duration(sec) * time.Second)
	if jwtExp, ok := parseJWTExpiry(tr.AccessToken); ok && jwtExp.Before(deadline) {
		deadline = jwtExp
	}
	c.expiresAt = deadline
	if tr.RefreshToken != "" {
		c.refreshTok = tr.RefreshToken
		if tr.RefreshExpiresIn > 0 {
			c.refreshExp = time.Now().Add(time.Duration(tr.RefreshExpiresIn) * time.Second)
		} else {
			c.refreshExp = time.Time{}
		}
		return
	}
	if clearRefreshWhenMissing {
		c.refreshTok = ""
		c.refreshExp = time.Time{}
	}
}

func (c *ciTokenSource) postTokenForm(ctx context.Context, form url.Values, clearRefreshWhenMissing bool) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("token %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var tr tokenEndpointJSON
	if err := json.Unmarshal(body, &tr); err != nil {
		return err
	}
	if tr.AccessToken == "" {
		return fmt.Errorf("no access_token")
	}
	c.applyTokenResponse(&tr, clearRefreshWhenMissing)
	return nil
}

func (c *ciTokenSource) refreshAllowedLocked() bool {
	if c.refreshTok == "" {
		return false
	}
	if c.refreshExp.IsZero() {
		return true
	}
	return time.Until(c.refreshExp) > ciRefreshMinRemaining
}

func (c *ciTokenSource) get(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token != "" && time.Until(c.expiresAt) > ciTokenRefreshLead {
		return c.token, nil
	}
	if c.refreshAllowedLocked() {
		form := url.Values{}
		form.Set("grant_type", "refresh_token")
		form.Set("refresh_token", c.refreshTok)
		form.Set("client_id", c.id)
		form.Set("client_secret", c.secret)
		if err := c.postTokenForm(ctx, form, false); err == nil {
			return c.token, nil
		}
		c.refreshTok = ""
		c.refreshExp = time.Time{}
	}
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", c.id)
	form.Set("client_secret", c.secret)
	if err := c.postTokenForm(ctx, form, true); err != nil {
		return "", err
	}
	return c.token, nil
}

func spaHandler(baseHref string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/")
		if p == "" {
			p = "index.html"
		}
		b, err := staticFS.ReadFile("dist/" + p)
		if err != nil {
			b, err = staticFS.ReadFile("dist/index.html")
			if err != nil {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(injectHTMLBaseTag(b, baseHref))
			return
		}
		switch {
		case strings.HasSuffix(p, ".js"):
			w.Header().Set("Content-Type", "application/javascript")
		case strings.HasSuffix(p, ".svg"):
			w.Header().Set("Content-Type", "image/svg+xml")
		case strings.HasSuffix(p, ".css"):
			w.Header().Set("Content-Type", "text/css")
		case strings.HasSuffix(p, ".html"):
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			if p == "index.html" {
				b = injectHTMLBaseTag(b, baseHref)
			}
		}
		_, _ = w.Write(b)
	})
}
