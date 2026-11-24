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

package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/acn-horizon-sdv/horizonauth"
	"github.com/acn-horizon-sdv/horizonauth/authtest"
	"github.com/coreos/go-oidc/v3/oidc"
)

func TestEnvAndMustEnv(t *testing.T) {
	const key = "HORIZON_PROXY_TEST_ENV"
	t.Setenv(key, "  value  ")
	if got := env(key, "default"); got != "value" {
		t.Fatalf("env() = %q", got)
	}
	t.Setenv(key, "   ")
	if got := env(key, "default"); got != "default" {
		t.Fatalf("blank env() = %q", got)
	}
	if got := env("HORIZON_PROXY_TEST_ENV_UNSET", "default"); got != "default" {
		t.Fatalf("unset env() = %q", got)
	}

	t.Setenv(key, "  required  ")
	if got := mustEnv(key); got != "required" {
		t.Fatalf("mustEnv() = %q", got)
	}
}

func TestMustEnvFatal(t *testing.T) {
	if os.Getenv("HORIZON_PROXY_TEST_FATAL") == "mustEnv" {
		mustEnv("HORIZON_PROXY_TEST_MISSING_ENV")
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=^TestMustEnvFatal$")
	cmd.Env = envWith(map[string]string{"HORIZON_PROXY_TEST_FATAL": "mustEnv"})
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected process to exit, output: %s", out)
	}
	if !strings.Contains(string(out), "missing env HORIZON_PROXY_TEST_MISSING_ENV") {
		t.Fatalf("output %q, err %v", out, err)
	}
}

func TestMustParseURL(t *testing.T) {
	u := mustParseURL("https://module-manager.example:8443/base")
	if u.Scheme != "https" || u.Host != "module-manager.example:8443" || u.Path != "/base" {
		t.Fatalf("parsed %#v", u)
	}
}

func TestMustParseURLFatal(t *testing.T) {
	if os.Getenv("HORIZON_PROXY_TEST_FATAL") == "parse" {
		mustParseURL(os.Getenv("HORIZON_PROXY_TEST_BAD_URL"))
		return
	}
	for _, raw := range []string{"", "example.com", "http://", "/only-path"} {
		cmd := exec.Command(os.Args[0], "-test.run=^TestMustParseURLFatal$")
		cmd.Env = envWith(map[string]string{
			"HORIZON_PROXY_TEST_FATAL":   "parse",
			"HORIZON_PROXY_TEST_BAD_URL": raw,
		})
		out, err := cmd.CombinedOutput()
		if err == nil {
			t.Fatalf("url %q: expected process to exit, output %s", raw, out)
		}
		if !strings.Contains(string(out), "bad URL: "+raw) {
			t.Fatalf("url %q: output %q, err %v", raw, out, err)
		}
	}
}

func TestNormalizePublicPath(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"", ""},
		{"   ", ""},
		{"/", ""},
		{"dev", "/dev"},
		{"/dev", "/dev"},
		{"/dev/", "/dev"},
		{"/dev/portal/", "/dev/portal"},
		{"  /dev/  ", "/dev"},
	}
	for _, tc := range tests {
		if got := normalizePublicPath(tc.in); got != tc.want {
			t.Errorf("normalizePublicPath(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestHTMLBaseTagHref(t *testing.T) {
	if got := htmlBaseTagHref(""); got != "" {
		t.Fatalf("empty = %q", got)
	}
	if got := htmlBaseTagHref("/dev"); got != "/dev/" {
		t.Fatalf("subpath = %q", got)
	}
}

func TestInjectHTMLBaseTag(t *testing.T) {
	html := []byte("<html><head><title>x</title></head></html>")
	if got := injectHTMLBaseTag(html, ""); !bytes.Equal(got, html) {
		t.Fatalf("empty href changed body: %s", got)
	}
	got := string(injectHTMLBaseTag(html, "/dev/"))
	want := "<html><head>\n    <base href=\"/dev/\"><title>x</title></head></html>"
	if got != want {
		t.Fatalf("inject = %s", got)
	}
	upper := injectHTMLBaseTag([]byte("<HTML><HEAD></HEAD>"), "/p/")
	if !bytes.Contains(upper, []byte(`<base href="/p/">`)) {
		t.Fatalf("case-insensitive head: %s", upper)
	}
	unchanged := []byte("<html><body>no head</body></html>")
	if got := injectHTMLBaseTag(unchanged, "/p/"); !bytes.Equal(got, unchanged) {
		t.Fatalf("missing head changed body: %s", got)
	}
	attrs := []byte(`<head lang="en"></head>`)
	if got := injectHTMLBaseTag(attrs, "/p/"); !bytes.Equal(got, attrs) {
		t.Fatalf("attributed head changed body: %s", got)
	}
	escaped := string(injectHTMLBaseTag([]byte("<head></head>"), `/a&b"c/`))
	if !strings.Contains(escaped, `<base href="/a&amp;b&quot;c/">`) {
		t.Fatalf("escaped = %s", escaped)
	}
}

func TestHTMLEscapeAttr(t *testing.T) {
	if got := htmlEscapeAttr(`a&b"c`); got != `a&amp;b&quot;c` {
		t.Fatalf("htmlEscapeAttr = %q", got)
	}
}

func TestServeConfigJS(t *testing.T) {
	rec := httptest.NewRecorder()
	serveConfigJS(rec, "https://portal.example", "/dev", "/auth/token", "portal", " 4.2.0 ")
	if rec.Header().Get("Content-Type") != "application/javascript; charset=utf-8" {
		t.Fatalf("content-type %q", rec.Header().Get("Content-Type"))
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("cache-control %q", rec.Header().Get("Cache-Control"))
	}
	want := `window.APP_CONFIG = window.APP_CONFIG || {"baseUrl":"https://portal.example","horizonVersion":"4.2.0","keycloakClientId":"portal","keycloakUrl":"/auth/token","publicPath":"/dev"};` + "\n"
	if rec.Body.String() != want {
		t.Fatalf("body %s", rec.Body.String())
	}

	rec = httptest.NewRecorder()
	serveConfigJS(rec, "", "", "/token", "portal", "  ")
	if strings.Contains(rec.Body.String(), "horizonVersion") {
		t.Fatalf("blank version should be omitted: %s", rec.Body.String())
	}
}

func TestBearerToken(t *testing.T) {
	tests := []struct {
		header  string
		want    string
		wantErr bool
	}{
		{"Bearer tok", "tok", false},
		{"bearer   tok  ", "tok", false},
		{"", "", true},
		{"Bearer", "", true},
		{"Bearer   ", "", true},
		{"Basic tok", "", true},
		{"Token tok", "", true},
	}
	for _, tc := range tests {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		if tc.header != "" {
			req.Header.Set("Authorization", tc.header)
		}
		got, err := bearerToken(req)
		if tc.wantErr {
			if err == nil {
				t.Errorf("header %q: expected error, got %q", tc.header, got)
			}
			continue
		}
		if err != nil || got != tc.want {
			t.Errorf("header %q: got %q, %v; want %q", tc.header, got, err, tc.want)
		}
	}
}

func TestNewMMProxy(t *testing.T) {
	var gotPath, gotQuery, gotAuth string
	var gotMu sync.Mutex
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMu.Lock()
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		gotAuth = r.Header.Get("Authorization")
		gotMu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	}))
	defer up.Close()
	upURL, err := url.Parse(up.URL)
	if err != nil {
		t.Fatal(err)
	}

	unauth := newMMProxy(upURL, nil)
	for _, header := range []string{"", "Basic abc"} {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/mm/modules", nil)
		if header != "" {
			req.Header.Set("Authorization", header)
		}
		unauth.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("header %q: status %d", header, rec.Code)
		}
	}
	gotMu.Lock()
	path := gotPath
	gotMu.Unlock()
	if path != "" {
		t.Fatal("upstream called without a bearer token")
	}

	iss := authtest.NewIssuer(t)
	h := newMMProxy(upURL, realmVerifier(t, iss.URL()))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/mm/modules?team=a", nil)
	req.Header.Set("Authorization", "Bearer not-a-token")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("bad token status %d", rec.Code)
	}
	gotMu.Lock()
	path = gotPath
	gotMu.Unlock()
	if path != "" {
		t.Fatal("upstream called for a rejected token")
	}

	// /api/mm checks only the signature: a token for another client of the realm is accepted.
	raw := iss.Token(t, iss.Claims(map[string]any{"azp": "grafana", "aud": "grafana"}))
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/mm/modules?team=a", nil)
	req.Header.Set("Authorization", "Bearer "+raw)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	gotMu.Lock()
	path, query, auth := gotPath, gotQuery, gotAuth
	gotMu.Unlock()
	if path != "/modules" || query != "team=a" || auth != "Bearer "+raw {
		t.Fatalf("upstream path %q query %q auth set %v", path, query, auth == "Bearer "+raw)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/mm", nil)
	req.Header.Set("Authorization", "Bearer "+raw)
	h.ServeHTTP(rec, req)
	gotMu.Lock()
	path = gotPath
	gotMu.Unlock()
	if path != "/" {
		t.Fatalf("trimmed path %q", path)
	}
}

func TestNewHorizonProxy(t *testing.T) {
	var calls, auths []string
	var upstreamMu sync.Mutex
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		upstreamMu.Lock()
		calls = append(calls, r.Method+" "+r.URL.RequestURI()+"#"+string(body))
		auths = append(auths, r.Header.Get("Authorization"))
		upstreamMu.Unlock()
		if r.URL.Path == "/denied" {
			http.Error(w, "denied", http.StatusUnauthorized)
			return
		}
		fmt.Fprintf(w, "upstream:%s", body)
	}))
	defer up.Close()
	upURL, err := url.Parse(up.URL)
	if err != nil {
		t.Fatal(err)
	}
	upstream := func() ([]string, []string) {
		upstreamMu.Lock()
		defer upstreamMu.Unlock()
		return append([]string(nil), calls...), append([]string(nil), auths...)
	}

	iss := authtest.NewIssuer(t)
	h := newHorizonProxy(upURL, horizonVerifier(t, iss.URL()))
	serve := func(method, path, header, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		if header != "" {
			req.Header.Set("Authorization", header)
		}
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	rejected := map[string]string{
		"no header":                   "",
		"basic credentials":           "Basic abc",
		"not a JWT":                   "Bearer not-a-token",
		"token for another client":    "Bearer " + iss.Token(t, iss.Claims(map[string]any{"azp": "grafana", "aud": []string{"grafana", "account"}})),
		"ID token":                    "Bearer " + iss.Token(t, iss.Claims(map[string]any{"typ": "ID"})),
		"expired token":               "Bearer " + iss.Token(t, iss.Claims(map[string]any{"exp": time.Now().Add(-time.Minute).Unix()})),
		"token signed by another key": "Bearer " + authtest.Sign(t, authtest.NewKey(t), iss.Claims(nil)),
	}
	for name, header := range rejected {
		if rec := serve(http.MethodGet, "/api/horizon/v1/catalog", header, ""); rec.Code != http.StatusUnauthorized {
			t.Errorf("%s: status %d, want 401", name, rec.Code)
		}
	}
	if got, _ := upstream(); len(got) != 0 {
		t.Fatalf("upstream called for rejected requests: %v", got)
	}

	raw := iss.Token(t, iss.Claims(map[string]any{"resource_access": authtest.Roles("developer")}))
	if rec := serve(http.MethodGet, "/api/horizon/v1/catalog?q=1", "Bearer "+raw, ""); rec.Code != http.StatusOK || rec.Body.String() != "upstream:" {
		t.Fatalf("GET status %d body %s", rec.Code, rec.Body.String())
	}
	if rec := serve(http.MethodPost, "/api/horizon/v1/modules/m/workflowTemplates/t/submit", "Bearer "+raw, `{"parameters":{}}`); rec.Code != http.StatusOK {
		t.Fatalf("POST status %d body %s", rec.Code, rec.Body.String())
	}
	if rec := serve(http.MethodDelete, "/api/horizon", "Bearer "+raw, ""); rec.Code != http.StatusOK {
		t.Fatalf("DELETE status %d body %s", rec.Code, rec.Body.String())
	}
	gotCalls, gotAuths := upstream()
	wantCalls := []string{
		"GET /v1/catalog?q=1#",
		`POST /v1/modules/m/workflowTemplates/t/submit#{"parameters":{}}`,
		"DELETE /#",
	}
	if strings.Join(gotCalls, "\n") != strings.Join(wantCalls, "\n") {
		t.Fatalf("forwarded %q, want %q", gotCalls, wantCalls)
	}
	for i, a := range gotAuths {
		if a != "Bearer "+raw {
			t.Fatalf("call %d forwarded Authorization %q, want the caller's token", i, a)
		}
	}

	// A 401 from Horizon API reaches the browser unchanged, so the portal refreshes the user's token.
	if rec := serve(http.MethodGet, "/api/horizon/denied", "Bearer "+raw, ""); rec.Code != http.StatusUnauthorized {
		t.Fatalf("upstream 401 became %d", rec.Code)
	}
	if got, _ := upstream(); len(got) != len(wantCalls)+1 {
		t.Fatalf("upstream 401 was retried: %d calls", len(got))
	}

	if rec := serve(http.MethodPost, "/api/horizon/v1/modules/m/workflowTemplates/t/submit", "Bearer "+raw, strings.Repeat("a", maxHorizonBody+1)); rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized POST status %d, want 413", rec.Code)
	}

	dead := newHorizonProxy(&url.URL{Scheme: "http", Host: "127.0.0.1:1"}, horizonVerifier(t, iss.URL()))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/horizon/v1/catalog", nil)
	req.Header.Set("Authorization", "Bearer "+raw)
	dead.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("unreachable upstream status %d, want 502", rec.Code)
	}
}

func TestNewHorizonProxyDocsPaths(t *testing.T) {
	var auths []string
	var gotMu sync.Mutex
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMu.Lock()
		auths = append(auths, r.Header.Get("Authorization"))
		gotMu.Unlock()
		_, _ = w.Write([]byte("docs"))
	}))
	defer up.Close()
	upURL, err := url.Parse(up.URL)
	if err != nil {
		t.Fatal(err)
	}
	h := newHorizonProxy(upURL, horizonVerifier(t, authtest.NewIssuer(t).URL()))

	for _, path := range []string{"/api/horizon/swagger", "/api/horizon/openapi.json", "/api/horizon/swagger/"} {
		for _, header := range []string{"", "Bearer unchecked"} {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			if header != "" {
				req.Header.Set("Authorization", header)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("%s with %q: status %d", path, header, rec.Code)
			}
		}
	}
	gotMu.Lock()
	defer gotMu.Unlock()
	for _, a := range auths {
		if a != "" {
			t.Fatalf("docs request forwarded Authorization %q", a)
		}
	}
}

func TestSpaHandler(t *testing.T) {
	if staticRoot != "spa" {
		t.Fatalf("static root %q; fixture must not live under a dist directory", staticRoot)
	}
	index, err := staticFS.ReadFile(staticRoot + "/index.html")
	if err != nil {
		t.Fatal(err)
	}
	h := spaHandler("/dev/")

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("index status %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Fatalf("index content-type %q", ct)
	}
	if !bytes.Equal(rec.Body.Bytes(), injectHTMLBaseTag(index, "/dev/")) {
		t.Fatalf("index body %s", rec.Body.String())
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/tools/cli", nil))
	if !bytes.Equal(rec.Body.Bytes(), injectHTMLBaseTag(index, "/dev/")) {
		t.Fatalf("fallback body %s", rec.Body.String())
	}

	rec = httptest.NewRecorder()
	spaHandler("").ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/index.html", nil))
	if !bytes.Equal(rec.Body.Bytes(), index) {
		t.Fatalf("root base href changed index: %s", rec.Body.String())
	}

	for _, suffix := range []string{".js", ".css", ".svg"} {
		rel, body, ok := embeddedBySuffix(suffix)
		if !ok {
			t.Fatalf("no embedded %s in %s", suffix, staticRoot)
		}
		rec = httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/"+rel, nil))
		if rec.Code != http.StatusOK || !bytes.Equal(rec.Body.Bytes(), body) {
			t.Fatalf("%s status %d", rel, rec.Code)
		}
		var wantType string
		switch suffix {
		case ".js":
			wantType = "application/javascript"
		case ".css":
			wantType = "text/css"
		case ".svg":
			wantType = "image/svg+xml"
		}
		if rec.Header().Get("Content-Type") != wantType {
			t.Fatalf("%s content-type %q", rel, rec.Header().Get("Content-Type"))
		}
	}

	pageRel, page, ok := embeddedBySuffix("page.html")
	if !ok {
		t.Fatal("embedded fixture has no nested html file")
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/"+pageRel, nil))
	if rec.Header().Get("Content-Type") != "text/html; charset=utf-8" || !bytes.Equal(rec.Body.Bytes(), page) {
		t.Fatalf("nested html status %d body changed %v", rec.Code, !bytes.Equal(rec.Body.Bytes(), page))
	}

	txtRel, txt, ok := embeddedBySuffix(".txt")
	if !ok {
		t.Fatal("embedded fixture has no plain file")
	}
	plain := &headerWriter{h: make(http.Header)}
	h.ServeHTTP(plain, httptest.NewRequest(http.MethodGet, "/"+txtRel, nil))
	if plain.h.Get("Content-Type") != "" || !bytes.Equal(plain.body.Bytes(), txt) {
		t.Fatalf("plain file content-type %q", plain.h.Get("Content-Type"))
	}
}

type headerWriter struct {
	h    http.Header
	body bytes.Buffer
	code int
}

func (w *headerWriter) Header() http.Header  { return w.h }
func (w *headerWriter) WriteHeader(code int) { w.code = code }
func (w *headerWriter) Write(p []byte) (int, error) {
	if w.code == 0 {
		w.code = http.StatusOK
	}
	return w.body.Write(p)
}

func TestOIDCBearerVerifier(t *testing.T) {
	iss := authtest.NewIssuer(t)
	v := oidcBearerVerifier{inner: realmVerifier(t, iss.URL())}
	raw := iss.Token(t, iss.Claims(nil))
	if err := v.Verify(context.Background(), raw); err != nil {
		t.Fatal(err)
	}
	if err := v.Verify(context.Background(), "not-a-token"); err == nil {
		t.Fatal("expected verify error")
	}
}

func TestProxyMainSmoke(t *testing.T) {
	if os.Getenv("HORIZON_PROXY_TEST_RUN_MAIN") == "1" {
		main()
		return
	}

	iss := authtest.NewIssuer(t)
	var mmPath string
	var mmMu sync.Mutex
	mm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mmMu.Lock()
		mmPath = r.URL.Path
		mmMu.Unlock()
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte("mm-ok"))
	}))
	defer mm.Close()
	var haAuth string
	var haMu sync.Mutex
	ha := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		haMu.Lock()
		haAuth = r.Header.Get("Authorization")
		haMu.Unlock()
		_, _ = io.WriteString(w, "ha-ok")
	}))
	defer ha.Close()

	var logs safeBuf
	cmd := exec.Command(os.Args[0], "-test.run=^TestProxyMainSmoke$")
	cmd.Env = envWith(map[string]string{
		"HORIZON_PROXY_TEST_RUN_MAIN": "1",
		"LISTEN_ADDR":                 "127.0.0.1:0",
		"OIDC_ISSUER_URL":             iss.URL(),
		"MODULE_MANAGER_BASE_URL":     mm.URL,
		"HORIZON_API_BASE_URL":        ha.URL,
		"PUBLIC_BASE_URL":             "https://portal.example",
		"PUBLIC_PATH":                 "dev",
	})
	cmd.Stdout = &logs
	cmd.Stderr = &logs
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
	})

	client := &http.Client{Timeout: 2 * time.Second}
	base := "http://" + waitListenAddr(t, &logs)
	resp := pollGET(t, client, base+"/healthz", &logs)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || string(body) != "ok" {
		t.Fatalf("healthz %d %s\n%s", resp.StatusCode, body, logs.String())
	}

	cfgResp, err := client.Get(base + "/config.js")
	if err != nil {
		t.Fatalf("config.js: %v\n%s", err, logs.String())
	}
	cfgBody, _ := io.ReadAll(cfgResp.Body)
	cfgResp.Body.Close()
	wantCfg := httptest.NewRecorder()
	serveConfigJS(wantCfg, "https://portal.example", normalizePublicPath("dev"), "/auth/realms/horizon/protocol/openid-connect/token", "horizon-dev-portal", "")
	if cfgResp.StatusCode != http.StatusOK || !bytes.Equal(cfgBody, wantCfg.Body.Bytes()) {
		t.Fatalf("config.js %d\n got %s\nwant %s", cfgResp.StatusCode, cfgBody, wantCfg.Body.Bytes())
	}

	spaResp, err := client.Get(base + "/")
	if err != nil {
		t.Fatal(err)
	}
	spaBody, _ := io.ReadAll(spaResp.Body)
	spaResp.Body.Close()
	if spaResp.StatusCode != http.StatusOK || !strings.Contains(spaResp.Header.Get("Content-Type"), "text/html") {
		t.Fatalf("spa %d %q", spaResp.StatusCode, spaResp.Header.Get("Content-Type"))
	}
	if strings.Contains(strings.ToLower(string(spaBody)), "<head>") && !strings.Contains(string(spaBody), `<base href="/dev/">`) {
		t.Fatalf("spa base tag missing: %s", spaBody)
	}

	mmResp, err := client.Get(base + "/api/mm/status")
	if err != nil {
		t.Fatal(err)
	}
	mmResp.Body.Close()
	if mmResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("mm without token %d", mmResp.StatusCode)
	}

	raw := iss.Token(t, iss.Claims(nil))
	mmReq, err := http.NewRequest(http.MethodGet, base+"/api/mm/status", nil)
	if err != nil {
		t.Fatal(err)
	}
	mmReq.Header.Set("Authorization", "Bearer "+raw)
	mmResp, err = client.Do(mmReq)
	if err != nil {
		t.Fatal(err)
	}
	mmBody, _ := io.ReadAll(mmResp.Body)
	mmResp.Body.Close()
	mmMu.Lock()
	gotMMPath := mmPath
	mmMu.Unlock()
	if mmResp.StatusCode != http.StatusOK || string(mmBody) != "mm-ok" || gotMMPath != "/status" {
		t.Fatalf("mm status %d body %s path %s", mmResp.StatusCode, mmBody, gotMMPath)
	}

	getHorizon := func(token string) (int, string) {
		t.Helper()
		req, err := http.NewRequest(http.MethodGet, base+"/api/horizon/v1/ping", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(b)
	}
	// main defaults the required audience to horizon-api.
	other := iss.Token(t, iss.Claims(map[string]any{"azp": "grafana", "aud": "grafana"}))
	if code, _ := getHorizon(other); code != http.StatusUnauthorized {
		t.Fatalf("horizon with a token for another client: %d", code)
	}
	if code, b := getHorizon(raw); code != http.StatusOK || b != "ha-ok" {
		t.Fatalf("horizon %d %s\n%s", code, b, logs.String())
	}
	haMu.Lock()
	gotHAAuth := haAuth
	haMu.Unlock()
	if gotHAAuth != "Bearer "+raw {
		t.Fatalf("horizon got Authorization %q, want the caller's token", gotHAAuth)
	}
}

// realmVerifier is the signature-only check main uses for /api/mm and /api/cli.
func realmVerifier(t *testing.T, issuer string) *oidc.IDTokenVerifier {
	t.Helper()
	provider, err := oidc.NewProvider(context.Background(), issuer)
	if err != nil {
		t.Fatal(err)
	}
	return provider.Verifier(&oidc.Config{SkipClientIDCheck: true})
}

// horizonVerifier is the check main uses for /api/horizon.
func horizonVerifier(t *testing.T, issuer string) *horizonauth.Verifier {
	t.Helper()
	v, err := horizonauth.NewVerifier(context.Background(), issuer, "horizon-api")
	if err != nil {
		t.Fatal(err)
	}
	return v
}

type safeBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *safeBuf) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *safeBuf) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

func envWith(overrides map[string]string) []string {
	skip := make(map[string]bool, len(overrides))
	for k := range overrides {
		skip[k] = true
	}
	var out []string
	for _, e := range os.Environ() {
		k, _, _ := strings.Cut(e, "=")
		if skip[k] {
			continue
		}
		out = append(out, e)
	}
	for k, v := range overrides {
		out = append(out, k+"="+v)
	}
	return out
}

var listenAddrPattern = regexp.MustCompile(`listening on (127\.0\.0.1:\d+)`)

func waitListenAddr(t *testing.T, logs *safeBuf) string {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if m := listenAddrPattern.FindStringSubmatch(logs.String()); m != nil {
			return m[1]
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("child did not report a listen address\n%s", logs.String())
	return ""
}

func pollGET(t *testing.T, client *http.Client, rawURL string, logs *safeBuf) *http.Response {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	var last error
	for time.Now().Before(deadline) {
		resp, err := client.Get(rawURL)
		if err == nil {
			return resp
		}
		last = err
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("GET %s: %v\n%s", rawURL, last, logs.String())
	return nil
}

func embeddedBySuffix(suffix string) (string, []byte, bool) {
	var rel string
	var body []byte
	err := fs.WalkDir(staticFS, staticRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || rel != "" {
			return err
		}
		if strings.HasSuffix(path, suffix) {
			rel = strings.TrimPrefix(path, staticRoot+"/")
			body, err = staticFS.ReadFile(path)
		}
		return err
	})
	if err != nil || rel == "" {
		return "", nil, false
	}
	return rel, body, true
}
