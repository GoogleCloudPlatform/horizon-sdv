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
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/acn-horizon-sdv/horizonauth"
	"github.com/coreos/go-oidc/v3/oidc"
)

// SPA files are embedded from spa. That directory is not named dist, so it
// stays in git while **/dist/ remains ignored. The image build copies the Vite
// output over spa before compiling.
//
//go:embed all:spa
var staticFS embed.FS

const staticRoot = "spa"

func main() {
	addr := env("LISTEN_ADDR", ":8080")
	issuer := mustEnv("OIDC_ISSUER_URL")
	mmBase := mustEnv("MODULE_MANAGER_BASE_URL")
	haBase := mustEnv("HORIZON_API_BASE_URL")
	// Keycloak client a token must be meant for to reach Horizon API (Horizon API's --oidc-client-id).
	haClientID := env("HORIZON_API_CLIENT_ID", "horizon-api")

	ctx := context.Background()
	provider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		log.Fatalf("oidc provider: %v", err)
	}
	verifier := provider.Verifier(&oidc.Config{SkipClientIDCheck: true})
	haVerifier, err := horizonauth.NewVerifier(ctx, issuer, haClientID)
	if err != nil {
		log.Fatalf("oidc: %v", err)
	}
	log.Printf("auth: /api/horizon forwards the caller's token; it must have aud %q", haClientID)

	mmURL := mustParseURL(mmBase)
	haURL := mustParseURL(haBase)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /config.js", func(w http.ResponseWriter, r *http.Request) {
		serveConfigJS(w, env("PUBLIC_BASE_URL", ""), normalizePublicPath(env("PUBLIC_PATH", "")), env("KEYCLOAK_TOKEN_PATH", "/auth/realms/horizon/protocol/openid-connect/token"), env("KEYCLOAK_CLIENT_ID", "horizon-dev-portal"), env("HORIZON_VERSION", ""))
	})
	mux.Handle("/api/mm/", newMMProxy(mmURL, verifier))
	var cliVer bearerVerifier = oidcBearerVerifier{inner: verifier}
	mux.Handle("/api/horizon/", newHorizonProxy(haURL, haVerifier))
	registerCLIRoutes(mux, env("CLI_DIST_DIR", "/cli-dist"), cliVer)
	mux.Handle("/", spaHandler(htmlBaseTagHref(normalizePublicPath(env("PUBLIC_PATH", "")))))

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}
	log.Printf("listening on %s", ln.Addr().String())
	log.Fatal(http.Serve(ln, mux))
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

func serveConfigJS(w http.ResponseWriter, publicBase, publicPath, keycloakTokenPath, clientID, horizonVersion string) {
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	cfg := map[string]string{
		"baseUrl":          publicBase,
		"publicPath":       publicPath,
		"keycloakUrl":      keycloakTokenPath,
		"keycloakClientId": clientID,
	}
	if strings.TrimSpace(horizonVersion) != "" {
		cfg["horizonVersion"] = strings.TrimSpace(horizonVersion)
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

// horizonDocsPaths are reachable without a bearer: the embedded Swagger UI iframe cannot send an
// Authorization header, and the spec it fetches is public developer documentation.
var horizonDocsPaths = []string{"/api/horizon/swagger", "/api/horizon/openapi.json"}

func isHorizonDocsPath(path string) bool {
	p := strings.TrimSuffix(path, "/")
	for _, d := range horizonDocsPaths {
		if p == d {
			return true
		}
	}
	return false
}

// maxHorizonBody caps request bodies forwarded to Horizon API, which decodes them without a limit.
const maxHorizonBody = 4 << 20

// newHorizonProxy forwards to Horizon API with the caller's own access token. The token gets the
// same checks as in Horizon API, so the proxy forwards only tokens Horizon API accepts.
func newHorizonProxy(upstream *url.URL, verifier *horizonauth.Verifier) http.Handler {
	rp := httputil.NewSingleHostReverseProxy(upstream)
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
	rp.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			http.Error(w, "body too large", http.StatusRequestEntityTooLarge)
			return
		}
		log.Printf("http: proxy error: %v", err)
		w.WriteHeader(http.StatusBadGateway)
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, maxHorizonBody)
		if isHorizonDocsPath(r.URL.Path) {
			// Sent without credentials, so no unchecked token reaches Horizon API.
			r = r.Clone(r.Context())
			r.Header.Del("Authorization")
			rp.ServeHTTP(w, r)
			return
		}
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

func spaHandler(baseHref string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/")
		if p == "" {
			p = "index.html"
		}
		b, err := staticFS.ReadFile(staticRoot + "/" + p)
		if err != nil {
			b, err = staticFS.ReadFile(staticRoot + "/index.html")
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
