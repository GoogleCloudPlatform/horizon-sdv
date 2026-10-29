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
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
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
	"sync/atomic"
	"testing"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
)

func TestParseJWTExpiry(t *testing.T) {
	exp := time.Unix(1_700_000_000, 0)
	tok := makeUnsignedJWT(t, map[string]any{"exp": exp.Unix()})
	got, ok := parseJWTExpiry(tok)
	if !ok || !got.Equal(exp) {
		t.Fatalf("parseJWTExpiry() = %v, %v; want %v, true", got, ok, exp)
	}

	padded := paddedUnsignedJWT(t, map[string]any{"exp": exp.Unix()})
	got, ok = parseJWTExpiry(padded)
	if !ok || !got.Equal(exp) {
		t.Fatalf("padded parseJWTExpiry() = %v, %v; want %v, true", got, ok, exp)
	}

	parts := strings.Split(tok, ".")
	got, ok = parseJWTExpiry(parts[0] + "." + parts[1])
	if !ok || !got.Equal(exp) {
		t.Fatalf("two-segment parseJWTExpiry() = %v, %v", got, ok)
	}

	for _, raw := range []string{
		"",
		"not-a-jwt",
		"a.@@@.c",
		makeUnsignedJWT(t, map[string]any{"exp": 0}),
		makeUnsignedJWT(t, map[string]any{"exp": "soon"}),
		makeUnsignedJWT(t, map[string]any{"sub": "no-exp"}),
		"h." + base64.RawURLEncoding.EncodeToString([]byte("[]")) + ".s",
	} {
		if _, ok := parseJWTExpiry(raw); ok {
			t.Fatalf("parseJWTExpiry(%q) ok, want failure", raw)
		}
	}
}

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
	serveConfigJS(rec, "https://portal.example", "/dev", "/auth/token", "portal", "  ci-client  ")
	if rec.Header().Get("Content-Type") != "application/javascript; charset=utf-8" {
		t.Fatalf("content-type %q", rec.Header().Get("Content-Type"))
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("cache-control %q", rec.Header().Get("Cache-Control"))
	}
	want := `window.APP_CONFIG = window.APP_CONFIG || {"baseUrl":"https://portal.example","horizonApiOAuthClientId":"ci-client","keycloakClientId":"portal","keycloakUrl":"/auth/token","publicPath":"/dev"};` + "\n"
	if rec.Body.String() != want {
		t.Fatalf("body %s", rec.Body.String())
	}

	rec = httptest.NewRecorder()
	serveConfigJS(rec, "", "", "/token", "portal", "  ")
	if strings.Contains(rec.Body.String(), "horizonApiOAuthClientId") {
		t.Fatalf("blank client id should be omitted: %s", rec.Body.String())
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
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		gotAuth = r.Header.Get("Authorization")
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
	if gotPath != "" {
		t.Fatal("upstream called without a bearer token")
	}

	issuer := newTestOIDC(t)
	h := newMMProxy(upURL, issuer.verifier)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/mm/modules?team=a", nil)
	req.Header.Set("Authorization", "Bearer not-a-token")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("bad token status %d", rec.Code)
	}
	if gotPath != "" {
		t.Fatal("upstream called for a rejected token")
	}

	raw := issuer.sign(t, time.Hour)
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/mm/modules?team=a", nil)
	req.Header.Set("Authorization", "Bearer "+raw)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	if gotPath != "/modules" || gotQuery != "team=a" || gotAuth != "Bearer "+raw {
		t.Fatalf("upstream path %q query %q auth set %v", gotPath, gotQuery, gotAuth == "Bearer "+raw)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/mm", nil)
	req.Header.Set("Authorization", "Bearer "+raw)
	h.ServeHTTP(rec, req)
	if gotPath != "/" {
		t.Fatalf("trimmed path %q", gotPath)
	}
}

func TestRetryCI401Transport(t *testing.T) {
	t.Run("passes through non-401", func(t *testing.T) {
		rt := &seqRT{codes: []int{http.StatusOK}, bodies: []string{"ok"}}
		tr := &retryCI401Transport{rt: rt, ci: newCITokenSource("http://127.0.0.1:1", "id", "sec")}
		req := httptest.NewRequest(http.MethodGet, "http://upstream/v", nil)
		resp, err := tr.RoundTrip(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK || rt.n != 1 {
			t.Fatalf("status %d trips %d", resp.StatusCode, rt.n)
		}
	})

	t.Run("retries get after 401", func(t *testing.T) {
		ts, log := newTokenServer(t, func(w http.ResponseWriter, call tokenCall, n int) {
			writeTokenJSON(w, fmt.Sprintf("t%d", n), 3600, "", 0)
		})
		ci := newCITokenSource(ts.URL, "id", "sec")
		ci.token = "stale"
		ci.expiresAt = time.Now().Add(time.Hour)
		rt := &seqRT{codes: []int{http.StatusUnauthorized, http.StatusOK}, bodies: []string{"old", "new"}}
		tr := &retryCI401Transport{rt: rt, ci: ci}
		req := httptest.NewRequest(http.MethodGet, "http://upstream/v", nil)
		req.Header.Set("Authorization", "Bearer stale")
		resp, err := tr.RoundTrip(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK || string(body) != "new" {
			t.Fatalf("status %d body %s", resp.StatusCode, body)
		}
		if len(rt.auths) != 2 || rt.auths[0] != "Bearer stale" || rt.auths[1] != "Bearer t1" {
			t.Fatalf("auths %#v", rt.auths)
		}
		if grants := log.grants(); len(grants) != 1 || grants[0] != "client_credentials" {
			t.Fatalf("grants %#v", grants)
		}
	})

	t.Run("returns final 401", func(t *testing.T) {
		ts, _ := newTokenServer(t, func(w http.ResponseWriter, call tokenCall, n int) {
			writeTokenJSON(w, "fresh", 3600, "", 0)
		})
		ci := newCITokenSource(ts.URL, "id", "sec")
		rt := &seqRT{codes: []int{http.StatusUnauthorized, http.StatusUnauthorized}, bodies: []string{"a", "b"}}
		tr := &retryCI401Transport{rt: rt, ci: ci}
		req := httptest.NewRequest(http.MethodHead, "http://upstream/v", nil)
		resp, err := tr.RoundTrip(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusUnauthorized || string(body) != "b" || rt.n != 2 {
			t.Fatalf("status %d body %s trips %d", resp.StatusCode, body, rt.n)
		}
	})

	t.Run("does not retry post without getbody", func(t *testing.T) {
		rt := &seqRT{codes: []int{http.StatusUnauthorized}, bodies: []string{"once"}}
		tr := &retryCI401Transport{rt: rt, ci: newCITokenSource("http://127.0.0.1:1", "id", "sec")}
		req := httptest.NewRequest(http.MethodPost, "http://upstream/v", io.NopCloser(strings.NewReader("z")))
		req.GetBody = nil
		resp, err := tr.RoundTrip(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized || rt.n != 1 {
			t.Fatalf("status %d trips %d", resp.StatusCode, rt.n)
		}
	})

	t.Run("retries post with getbody", func(t *testing.T) {
		ts, _ := newTokenServer(t, func(w http.ResponseWriter, call tokenCall, n int) {
			writeTokenJSON(w, "replayed", 3600, "", 0)
		})
		ci := newCITokenSource(ts.URL, "id", "sec")
		rt := &readBodyRT{codes: []int{http.StatusUnauthorized, http.StatusCreated}}
		tr := &retryCI401Transport{rt: rt, ci: ci}
		payload := []byte("payload")
		req := httptest.NewRequest(http.MethodPost, "http://upstream/v", bytes.NewReader(payload))
		req.GetBody = func() (io.ReadCloser, error) {
			return io.NopCloser(bytes.NewReader(payload)), nil
		}
		resp, err := tr.RoundTrip(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("status %d", resp.StatusCode)
		}
		if len(rt.bodies) != 2 || rt.bodies[0] != "payload" || rt.bodies[1] != "payload" {
			t.Fatalf("bodies %#v", rt.bodies)
		}
		if rt.auths[1] != "Bearer replayed" {
			t.Fatalf("retry auth %q", rt.auths[1])
		}
	})

	t.Run("getbody error", func(t *testing.T) {
		ts, _ := newTokenServer(t, func(w http.ResponseWriter, call tokenCall, n int) {
			writeTokenJSON(w, "tok", 3600, "", 0)
		})
		ci := newCITokenSource(ts.URL, "id", "sec")
		rt := &seqRT{codes: []int{http.StatusUnauthorized}}
		tr := &retryCI401Transport{rt: rt, ci: ci}
		req := httptest.NewRequest(http.MethodPut, "http://upstream/v", strings.NewReader("x"))
		req.GetBody = func() (io.ReadCloser, error) { return nil, errors.New("nope") }
		resp, err := tr.RoundTrip(req)
		if err == nil || !strings.Contains(err.Error(), "nope") {
			t.Fatalf("err %v resp %v", err, resp)
		}
	})

	t.Run("token refresh error", func(t *testing.T) {
		rt := &seqRT{codes: []int{http.StatusUnauthorized}}
		tr := &retryCI401Transport{rt: rt, ci: newCITokenSource("http://127.0.0.1:1", "id", "sec")}
		req := httptest.NewRequest(http.MethodGet, "http://upstream/v", nil)
		resp, err := tr.RoundTrip(req)
		if err == nil || resp != nil {
			t.Fatalf("err %v resp %v", err, resp)
		}
	})

	t.Run("transport error", func(t *testing.T) {
		tr := &retryCI401Transport{rt: errRT{errors.New("dial")}, ci: newCITokenSource("http://127.0.0.1:1", "id", "sec")}
		req := httptest.NewRequest(http.MethodGet, "http://upstream/v", nil)
		resp, err := tr.RoundTrip(req)
		if err == nil || !strings.Contains(err.Error(), "dial") || resp != nil {
			t.Fatalf("err %v resp %v", err, resp)
		}
	})

	t.Run("nil response", func(t *testing.T) {
		tr := &retryCI401Transport{rt: errRT{}, ci: newCITokenSource("http://127.0.0.1:1", "id", "sec")}
		req := httptest.NewRequest(http.MethodGet, "http://upstream/v", nil)
		resp, err := tr.RoundTrip(req)
		if err != nil || resp != nil {
			t.Fatalf("err %v resp %v", err, resp)
		}
	})

	t.Run("nil transport uses default", func(t *testing.T) {
		up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}))
		defer up.Close()
		tr := &retryCI401Transport{ci: newCITokenSource("http://127.0.0.1:1", "id", "sec")}
		req, err := http.NewRequest(http.MethodPost, up.URL+"/x", nil)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := tr.RoundTrip(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("status %d", resp.StatusCode)
		}
	})
}

func TestNewHorizonProxy(t *testing.T) {
	var calls []string
	var auths []string
	var retryHits int
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		calls = append(calls, r.URL.RequestURI()+"#"+string(body))
		auths = append(auths, r.Header.Get("Authorization"))
		if r.URL.Path == "/retry" {
			retryHits++
			if retryHits == 1 {
				http.Error(w, "denied", http.StatusUnauthorized)
				return
			}
		}
		fmt.Fprintf(w, "upstream:%s", body)
	}))
	defer up.Close()
	upURL, err := url.Parse(up.URL)
	if err != nil {
		t.Fatal(err)
	}

	ts, log := newTokenServer(t, func(w http.ResponseWriter, call tokenCall, n int) {
		if call.id != "ci-id" || call.secret != "ci-sec" {
			http.Error(w, "bad client", http.StatusUnauthorized)
			return
		}
		writeTokenJSON(w, fmt.Sprintf("ci-%d", n), 3600, "", 0)
	})
	ci := newCITokenSource(ts.URL, "ci-id", "ci-sec")
	h := newHorizonProxy(upURL, ci)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/horizon/v1/catalog?q=1", nil)
	req.Header.Set("Authorization", "Bearer user")
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "upstream:" {
		t.Fatalf("status %d body %s", rec.Code, rec.Body.String())
	}
	if len(auths) != 1 || auths[0] != "Bearer ci-1" {
		t.Fatalf("auth %#v", auths)
	}
	if calls[0] != "/v1/catalog?q=1#" {
		t.Fatalf("forwarded %q", calls[0])
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/horizon/v1/jobs", strings.NewReader("hello"))
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Body.String() != "upstream:hello" {
		t.Fatalf("post status %d body %s", rec.Code, rec.Body.String())
	}
	if !strings.HasSuffix(calls[len(calls)-1], "#hello") {
		t.Fatalf("post forwarded %q", calls[len(calls)-1])
	}

	before := len(calls)
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/horizon/retry", nil)
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("retry status %d body %s", rec.Code, rec.Body.String())
	}
	if len(calls)-before != 2 {
		t.Fatalf("retry calls %d", len(calls)-before)
	}
	if auths[len(auths)-2] == auths[len(auths)-1] {
		t.Fatalf("expected a refreshed token, auths %#v", auths[len(auths)-2:])
	}
	if grants := log.grants(); len(grants) < 2 {
		t.Fatalf("grants %#v", grants)
	}

	dead := newCITokenSource("http://127.0.0.1:1", "id", "sec")
	rec = httptest.NewRecorder()
	silent := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("upstream called after token failure")
	}))
	defer silent.Close()
	silentURL, err := url.Parse(silent.URL)
	if err != nil {
		t.Fatal(err)
	}
	newHorizonProxy(silentURL, dead).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/horizon/x", nil))
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("token failure status %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	big := bytes.Repeat([]byte("a"), (4<<20)+1)
	req = httptest.NewRequest(http.MethodPost, "/api/horizon/v1/jobs", bytes.NewReader(big))
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("large body status %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPatch, "/api/horizon/v1/jobs", nil)
	req.Body = boomReadCloser{}
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "read body") {
		t.Fatalf("read error status %d body %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/api/horizon", strings.NewReader("gone"))
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || calls[len(calls)-1] != "/#gone" {
		t.Fatalf("delete status %d forwarded %q", rec.Code, calls[len(calls)-1])
	}
}

func TestCITokenSource(t *testing.T) {
	t.Run("cache hit", func(t *testing.T) {
		c := newCITokenSource("http://127.0.0.1:1", "id", "sec")
		c.token = "cached"
		c.expiresAt = time.Now().Add(ciTokenRefreshLead + 5*time.Second)
		got, err := c.get(context.Background())
		if err != nil || got != "cached" {
			t.Fatalf("got %q err %v", got, err)
		}
	})

	t.Run("client credentials", func(t *testing.T) {
		ts, log := newTokenServer(t, func(w http.ResponseWriter, call tokenCall, n int) {
			if call.id != "id" || call.secret != "sec" {
				http.Error(w, "bad client", http.StatusBadRequest)
				return
			}
			if call.grant == "refresh_token" && call.refresh != "refresh-1" {
				http.Error(w, "bad refresh", http.StatusBadRequest)
				return
			}
			if call.grant != "client_credentials" && call.grant != "refresh_token" {
				http.Error(w, "bad grant", http.StatusBadRequest)
				return
			}
			writeTokenJSON(w, "access-1", 3600, "refresh-1", 600)
		})
		c := newCITokenSource(ts.URL, "id", "sec")
		got, err := c.get(context.Background())
		if err != nil || got != "access-1" {
			t.Fatalf("got %q err %v", got, err)
		}
		if c.refreshTok != "refresh-1" || c.refreshExp.IsZero() {
			t.Fatalf("refresh %q exp zero %v", c.refreshTok, c.refreshExp.IsZero())
		}
		if grants := log.grants(); len(grants) != 1 {
			t.Fatalf("grants %#v", grants)
		}

		c.expiresAt = time.Now().Add(ciTokenRefreshLead - time.Second)
		got, err = c.get(context.Background())
		if err != nil || got != "access-1" {
			t.Fatalf("second get %q err %v", got, err)
		}
		if grants := log.grants(); len(grants) != 2 || grants[1] != "refresh_token" {
			t.Fatalf("grants %#v", grants)
		}
	})

	t.Run("refresh failure falls back", func(t *testing.T) {
		ts, log := newTokenServer(t, func(w http.ResponseWriter, call tokenCall, n int) {
			if call.grant == "refresh_token" {
				http.Error(w, "expired refresh", http.StatusBadRequest)
				return
			}
			writeTokenJSON(w, "from-cc", 3600, "", 0)
		})
		c := newCITokenSource(ts.URL, "id", "sec")
		c.refreshTok = "old-refresh"
		c.refreshExp = time.Time{}
		c.expiresAt = time.Now()
		got, err := c.get(context.Background())
		if err != nil || got != "from-cc" {
			t.Fatalf("got %q err %v", got, err)
		}
		if c.refreshTok != "" || !c.refreshExp.IsZero() {
			t.Fatalf("refresh not cleared: %q", c.refreshTok)
		}
		grants := log.grants()
		if len(grants) != 2 || grants[0] != "refresh_token" || grants[1] != "client_credentials" {
			t.Fatalf("grants %#v", grants)
		}
	})

	t.Run("refresh keeps previous token when not rotated", func(t *testing.T) {
		ts, _ := newTokenServer(t, func(w http.ResponseWriter, call tokenCall, n int) {
			writeTokenJSON(w, "access-2", 3600, "", 0)
		})
		c := newCITokenSource(ts.URL, "id", "sec")
		c.refreshTok = "keep-me"
		c.refreshExp = time.Now().Add(time.Hour)
		got, err := c.get(context.Background())
		if err != nil || got != "access-2" {
			t.Fatalf("got %q err %v", got, err)
		}
		if c.refreshTok != "keep-me" {
			t.Fatalf("refresh %q", c.refreshTok)
		}
	})

	t.Run("near refresh expiry uses client credentials and clears refresh", func(t *testing.T) {
		ts, log := newTokenServer(t, func(w http.ResponseWriter, call tokenCall, n int) {
			writeTokenJSON(w, "cc", 10, "", 0)
		})
		c := newCITokenSource(ts.URL, "id", "sec")
		c.refreshTok = "soon"
		c.refreshExp = time.Now().Add(ciRefreshMinRemaining - time.Second)
		if c.refreshAllowedLocked() {
			t.Fatal("refresh should be inside the minimum remaining window")
		}
		got, err := c.get(context.Background())
		if err != nil || got != "cc" || c.refreshTok != "" {
			t.Fatalf("got %q refresh %q err %v", got, c.refreshTok, err)
		}
		if grants := log.grants(); len(grants) != 1 || grants[0] != "client_credentials" {
			t.Fatalf("grants %#v", grants)
		}
	})

	t.Run("both grants fail", func(t *testing.T) {
		ts, _ := newTokenServer(t, func(w http.ResponseWriter, call tokenCall, n int) {
			http.Error(w, "nope", http.StatusUnauthorized)
		})
		c := newCITokenSource(ts.URL, "id", "sec")
		c.refreshTok = "r"
		_, err := c.get(context.Background())
		if err == nil || !strings.Contains(err.Error(), "token 401") {
			t.Fatalf("err %v", err)
		}
		if c.refreshTok != "" {
			t.Fatalf("refresh %q", c.refreshTok)
		}
	})

	t.Run("invalidate clears access only", func(t *testing.T) {
		c := newCITokenSource("http://example", "id", "sec")
		c.token = "a"
		c.expiresAt = time.Now().Add(time.Hour)
		c.refreshTok = "r"
		c.refreshExp = time.Now().Add(time.Hour)
		c.invalidate()
		if c.token != "" || !c.expiresAt.IsZero() || c.refreshTok != "r" {
			t.Fatalf("token %q exp zero %v refresh %q", c.token, c.expiresAt.IsZero(), c.refreshTok)
		}
	})

	t.Run("apply token response deadlines", func(t *testing.T) {
		c := newCITokenSource("http://example", "id", "sec")
		c.refreshTok = "previous"
		c.applyTokenResponse(&tokenEndpointJSON{AccessToken: "a", ExpiresIn: 0}, true)
		if d := time.Until(c.expiresAt); d < 4*time.Minute || d > 5*time.Minute+time.Second {
			t.Fatalf("default lifetime %s", d)
		}
		if c.refreshTok != "" {
			t.Fatalf("refresh cleared, got %q", c.refreshTok)
		}

		jwtExp := time.Now().Add(30 * time.Second).Unix()
		c.applyTokenResponse(&tokenEndpointJSON{
			AccessToken:      makeUnsignedJWT(t, map[string]any{"exp": jwtExp}),
			ExpiresIn:        7200,
			RefreshToken:     "rot",
			RefreshExpiresIn: 0,
		}, true)
		want := time.Unix(jwtExp, 0)
		if c.expiresAt.Sub(want).Abs() > time.Second {
			t.Fatalf("jwt cap expiresAt %s want %s", c.expiresAt, want)
		}
		if c.refreshTok != "rot" || !c.refreshExp.IsZero() {
			t.Fatalf("refresh %q zero %v", c.refreshTok, c.refreshExp.IsZero())
		}

		later := time.Now().Add(24 * time.Hour).Unix()
		c.applyTokenResponse(&tokenEndpointJSON{
			AccessToken:      makeUnsignedJWT(t, map[string]any{"exp": later}),
			ExpiresIn:        10,
			RefreshToken:     "rot2",
			RefreshExpiresIn: 120,
		}, false)
		if d := time.Until(c.expiresAt); d < 7*time.Second || d > 13*time.Second {
			t.Fatalf("expires_in wins when jwt exp is later: %s", d)
		}
		if c.refreshExp.IsZero() || time.Until(c.refreshExp) < time.Minute {
			t.Fatalf("refresh exp %s", c.refreshExp)
		}

		c.refreshTok = "stay"
		c.applyTokenResponse(&tokenEndpointJSON{AccessToken: "b", ExpiresIn: 30}, false)
		if c.refreshTok != "stay" {
			t.Fatalf("refresh %q", c.refreshTok)
		}
	})

	t.Run("refresh allowed", func(t *testing.T) {
		c := newCITokenSource("http://example", "id", "sec")
		if c.refreshAllowedLocked() {
			t.Fatal("empty refresh token")
		}
		c.refreshTok = "r"
		c.refreshExp = time.Time{}
		if !c.refreshAllowedLocked() {
			t.Fatal("missing refresh expiry should still allow refresh")
		}
		c.refreshExp = time.Now().Add(ciRefreshMinRemaining + 5*time.Second)
		if !c.refreshAllowedLocked() {
			t.Fatal("refresh outside the minimum window")
		}
		c.refreshExp = time.Now().Add(ciRefreshMinRemaining - time.Second)
		if c.refreshAllowedLocked() {
			t.Fatal("refresh inside the minimum window")
		}
	})

	t.Run("post token form errors", func(t *testing.T) {
		c := newCITokenSource("http://127.0.0.1:1", "id", "sec")
		if err := c.postTokenForm(context.Background(), url.Values{}, true); err == nil {
			t.Fatal("expected dial error")
		}

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Query().Get("mode") {
			case "status":
				http.Error(w, "  denied  ", http.StatusForbidden)
			case "json":
				_, _ = w.Write([]byte("{"))
			case "empty":
				_, _ = w.Write([]byte(`{"expires_in":30}`))
			default:
				_, _ = w.Write([]byte(`{"access_token":"ok","expires_in":30}`))
			}
		}))
		defer ts.Close()
		c.tokenURL = ts.URL + "?mode=status"
		err := c.postTokenForm(context.Background(), url.Values{}, true)
		if err == nil || !strings.Contains(err.Error(), "token 403: denied") {
			t.Fatalf("status err %v", err)
		}
		c.tokenURL = ts.URL + "?mode=json"
		if err := c.postTokenForm(context.Background(), url.Values{}, true); err == nil {
			t.Fatal("expected json error")
		}
		c.tokenURL = ts.URL + "?mode=empty"
		if err := c.postTokenForm(context.Background(), url.Values{}, true); err == nil || !strings.Contains(err.Error(), "no access_token") {
			t.Fatalf("empty token err %v", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		c.tokenURL = ts.URL
		if err := c.postTokenForm(ctx, url.Values{}, true); err == nil {
			t.Fatal("expected canceled context")
		}

		c.tokenURL = "http://[::1]:namedport"
		if err := c.postTokenForm(context.Background(), url.Values{}, true); err == nil {
			t.Fatal("expected request construction error")
		}
	})
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
	issuer := newTestOIDC(t)
	v := oidcBearerVerifier{inner: issuer.verifier}
	raw := issuer.sign(t, time.Hour)
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

	issuer := newTestOIDC(t)
	var mmPath string
	mm := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mmPath = r.URL.Path
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte("mm-ok"))
	}))
	defer mm.Close()
	var haAuth string
	ha := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		haAuth = r.Header.Get("Authorization")
		_, _ = io.WriteString(w, "ha-ok")
	}))
	defer ha.Close()

	var logs safeBuf
	cmd := exec.Command(os.Args[0], "-test.run=^TestProxyMainSmoke$")
	cmd.Env = envWith(map[string]string{
		"HORIZON_PROXY_TEST_RUN_MAIN":  "1",
		"LISTEN_ADDR":                  "127.0.0.1:0",
		"OIDC_ISSUER_URL":              issuer.issuer,
		"MODULE_MANAGER_BASE_URL":      mm.URL,
		"HORIZON_API_BASE_URL":         ha.URL,
		"HORIZON_API_CI_CLIENT_ID":     "ci-client",
		"HORIZON_API_CI_CLIENT_SECRET": "ci-secret",
		"PUBLIC_BASE_URL":              "https://portal.example",
		"PUBLIC_PATH":                  "dev",
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
	serveConfigJS(wantCfg, "https://portal.example", normalizePublicPath("dev"), "/auth/realms/horizon/protocol/openid-connect/token", "horizon-dev-portal", "ci-client")
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

	raw := issuer.sign(t, time.Hour)
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
	if mmResp.StatusCode != http.StatusOK || string(mmBody) != "mm-ok" || mmPath != "/status" {
		t.Fatalf("mm status %d body %s path %s", mmResp.StatusCode, mmBody, mmPath)
	}

	before := issuer.tokenHits.Load()
	haResp, err := client.Get(base + "/api/horizon/v1/ping")
	if err != nil {
		t.Fatal(err)
	}
	haBody, _ := io.ReadAll(haResp.Body)
	haResp.Body.Close()
	if haResp.StatusCode != http.StatusOK || string(haBody) != "ha-ok" {
		t.Fatalf("horizon %d %s\n%s", haResp.StatusCode, haBody, logs.String())
	}
	if haAuth != "Bearer smoke-access" {
		t.Fatalf("horizon auth %q", haAuth)
	}
	if issuer.tokenHits.Load() <= before {
		t.Fatal("main did not call the derived token endpoint")
	}
}

type testOIDC struct {
	issuer    string
	verifier  *oidc.IDTokenVerifier
	key       *rsa.PrivateKey
	tokenHits *atomic.Int32
}

func newTestOIDC(t *testing.T) *testOIDC {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	o := &testOIDC{key: key, tokenHits: &atomic.Int32{}}
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	o.issuer = srv.URL
	mux.HandleFunc("GET /.well-known/openid-configuration", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"issuer":                                o.issuer,
			"authorization_endpoint":                o.issuer + "/auth",
			"token_endpoint":                        o.issuer + "/protocol/openid-connect/token",
			"jwks_uri":                              o.issuer + "/keys",
			"response_types_supported":              []string{"id_token", "code"},
			"subject_types_supported":               []string{"public"},
			"id_token_signing_alg_values_supported": []string{"RS256"},
		})
	})
	mux.HandleFunc("GET /keys", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if o.key.E != 65537 {
			http.Error(w, "unexpected exponent", http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"keys": []map[string]string{{
				"kty": "RSA",
				"kid": "test-key",
				"alg": "RS256",
				"use": "sig",
				"n":   base64.RawURLEncoding.EncodeToString(o.key.N.Bytes()),
				"e":   base64.RawURLEncoding.EncodeToString([]byte{1, 0, 1}),
			}},
		})
	})
	mux.HandleFunc("POST /protocol/openid-connect/token", func(w http.ResponseWriter, r *http.Request) {
		o.tokenHits.Add(1)
		writeTokenJSON(w, "smoke-access", 3600, "", 0)
	})
	provider, err := oidc.NewProvider(context.Background(), o.issuer)
	if err != nil {
		t.Fatal(err)
	}
	o.verifier = provider.Verifier(&oidc.Config{SkipClientIDCheck: true})
	return o
}

func (o *testOIDC) sign(t *testing.T, ttl time.Duration) string {
	t.Helper()
	now := time.Now()
	claims := map[string]any{
		"iss": o.issuer,
		"sub": "user-1",
		"aud": "horizon-dev-portal",
		"iat": now.Add(-time.Minute).Unix(),
		"exp": now.Add(ttl).Unix(),
	}
	hdr := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","kid":"test-key","typ":"JWT"}`))
	pb, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	payload := base64.RawURLEncoding.EncodeToString(pb)
	signingInput := hdr + "." + payload
	sum := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, o.key, crypto.SHA256, sum[:])
	if err != nil {
		t.Fatal(err)
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig)
}

func makeUnsignedJWT(t *testing.T, claims map[string]any) string {
	t.Helper()
	hdr := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	pb, err := json.Marshal(claims)
	if err != nil {
		t.Fatal(err)
	}
	payload := base64.RawURLEncoding.EncodeToString(pb)
	return hdr + "." + payload + ".sig"
}

func paddedUnsignedJWT(t *testing.T, claims map[string]any) string {
	t.Helper()
	pad := 0
	var enc string
	for pad < 6 {
		extra, err := json.Marshal(map[string]any{"exp": claims["exp"], "pad": strings.Repeat("x", pad)})
		if err != nil {
			t.Fatal(err)
		}
		enc = base64.URLEncoding.EncodeToString(extra)
		if strings.Contains(enc, "=") {
			if _, err := base64.RawURLEncoding.DecodeString(enc); err == nil {
				t.Fatal("padded payload unexpectedly decoded as raw base64url")
			}
			break
		}
		pad++
	}
	if !strings.Contains(enc, "=") {
		t.Fatal("could not build a padded payload")
	}
	hdr := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	return hdr + "." + enc + ".sig"
}

type tokenCall struct {
	grant   string
	refresh string
	id      string
	secret  string
}

type tokenLog struct {
	mu    sync.Mutex
	calls []tokenCall
}

func (l *tokenLog) add(c tokenCall) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.calls = append(l.calls, c)
	return len(l.calls)
}

func (l *tokenLog) grants() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]string, len(l.calls))
	for i, c := range l.calls {
		out[i] = c.grant
	}
	return out
}

func newTokenServer(t *testing.T, fn func(w http.ResponseWriter, call tokenCall, n int)) (*httptest.Server, *tokenLog) {
	t.Helper()
	log := &tokenLog{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		call := tokenCall{
			grant:   r.Form.Get("grant_type"),
			refresh: r.Form.Get("refresh_token"),
			id:      r.Form.Get("client_id"),
			secret:  r.Form.Get("client_secret"),
		}
		n := log.add(call)
		fn(w, call, n)
	}))
	t.Cleanup(srv.Close)
	return srv, log
}

func writeTokenJSON(w http.ResponseWriter, access string, expires int, refresh string, refreshExp int) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"access_token":       access,
		"expires_in":         expires,
		"refresh_token":      refresh,
		"refresh_expires_in": refreshExp,
	})
}

type seqRT struct {
	codes  []int
	bodies []string
	auths  []string
	n      int
}

func (s *seqRT) RoundTrip(req *http.Request) (*http.Response, error) {
	s.auths = append(s.auths, req.Header.Get("Authorization"))
	code := s.codes[s.n]
	body := ""
	if s.n < len(s.bodies) {
		body = s.bodies[s.n]
	}
	s.n++
	return &http.Response{
		StatusCode: code,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
		Request:    req,
	}, nil
}

type readBodyRT struct {
	codes  []int
	bodies []string
	auths  []string
}

func (s *readBodyRT) RoundTrip(req *http.Request) (*http.Response, error) {
	s.auths = append(s.auths, req.Header.Get("Authorization"))
	var b []byte
	if req.Body != nil {
		var err error
		b, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
	}
	s.bodies = append(s.bodies, string(b))
	code := s.codes[len(s.bodies)-1]
	return &http.Response{
		StatusCode: code,
		Body:       io.NopCloser(strings.NewReader("ok")),
		Header:     make(http.Header),
		Request:    req,
	}, nil
}

type errRT struct{ err error }

func (e errRT) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, e.err
}

type boomReadCloser struct{}

func (boomReadCloser) Read([]byte) (int, error) { return 0, errors.New("boom") }
func (boomReadCloser) Close() error             { return nil }

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
