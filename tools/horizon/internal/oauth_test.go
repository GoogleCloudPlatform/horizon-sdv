// Copyright (c) 2026 Accenture, All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
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
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestTokenURL(t *testing.T) {
	got := tokenURL("https://auth.example/", "horizon")
	want := "https://auth.example/realms/horizon/protocol/openid-connect/token"
	if got != want {
		t.Fatalf("got %q", got)
	}
	if authBase("https://auth.example", "horizon") != "https://auth.example/realms/horizon/protocol/openid-connect" {
		t.Fatalf("auth base = %q", authBase("https://auth.example", "horizon"))
	}
}

func TestNewPKCE(t *testing.T) {
	verifier, challenge, err := newPKCE()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := base64.RawURLEncoding.DecodeString(verifier)
	if err != nil || len(raw) != 32 {
		t.Fatalf("verifier = %q %v", verifier, err)
	}
	sum := sha256.Sum256([]byte(verifier))
	if challenge != base64.RawURLEncoding.EncodeToString(sum[:]) {
		t.Fatalf("challenge = %q", challenge)
	}
	other, _, err := newPKCE()
	if err != nil {
		t.Fatal(err)
	}
	if other == verifier {
		t.Fatal("expected distinct verifiers")
	}
}

func TestTokenFromResponse(t *testing.T) {
	before := time.Now().Unix()
	got := tokenFromResponse(&tokenEndpointResponse{AccessToken: "a", RefreshToken: "r", ExpiresIn: 30}, "https://auth.example/", "horizon", "cli")
	if got.AccessToken != "a" || got.RefreshToken != "r" || got.KeycloakBase != "https://auth.example" || got.Realm != "horizon" || got.ClientID != "cli" {
		t.Fatalf("got %+v", got)
	}
	if got.ExpiresAt < before+30 || got.ExpiresAt > time.Now().Unix()+30 {
		t.Fatalf("expires = %d", got.ExpiresAt)
	}
	noExp := tokenFromResponse(&tokenEndpointResponse{AccessToken: "a"}, "https://auth.example", "horizon", "cli")
	if noExp.ExpiresAt != 0 {
		t.Fatalf("expires = %d", noExp.ExpiresAt)
	}
}

func TestClientCredentialsAndRefresh(t *testing.T) {
	var sawSecret string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		form, _ := url.ParseQuery(string(body))
		sawSecret = form.Get("client_secret")
		if r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
			t.Errorf("content-type = %q", r.Header.Get("Content-Type"))
		}
		switch form.Get("grant_type") {
		case "client_credentials":
			if form.Get("client_id") != "cli" {
				t.Errorf("client_id = %q", form.Get("client_id"))
			}
			_ = json.NewEncoder(w).Encode(tokenEndpointResponse{AccessToken: "access", RefreshToken: "refresh", ExpiresIn: 60})
		case "refresh_token":
			if form.Get("refresh_token") != "old-refresh" {
				t.Errorf("refresh = %q", form.Get("refresh_token"))
			}
			_ = json.NewEncoder(w).Encode(tokenEndpointResponse{AccessToken: "new-access", ExpiresIn: 15})
		default:
			http.Error(w, "nope", http.StatusBadRequest)
		}
	}))
	defer srv.Close()

	oauth := &OAuthClient{HTTP: srv.Client()}
	tok, err := oauth.ClientCredentials(context.Background(), srv.URL, "horizon", "cli", "secret")
	if err != nil {
		t.Fatal(err)
	}
	if tok.AccessToken != "access" || tok.RefreshToken != "refresh" || sawSecret != "secret" {
		t.Fatalf("token = %+v secret=%q", tok, sawSecret)
	}

	sawSecret = "unset"
	refreshed, err := oauth.RefreshToken(context.Background(), srv.URL, "horizon", "cli", "", "old-refresh")
	if err != nil {
		t.Fatal(err)
	}
	if refreshed.AccessToken != "new-access" || refreshed.RefreshToken != "old-refresh" {
		t.Fatalf("refreshed = %+v", refreshed)
	}
	if sawSecret != "" {
		t.Fatalf("empty client secret was sent: %q", sawSecret)
	}
}

func TestOAuthErrorResponses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		form, _ := url.ParseQuery(string(body))
		if form.Get("grant_type") == "client_credentials" && form.Get("client_id") == "bad" {
			http.Error(w, "denied", http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"error":"invalid_grant","error_description":"nope"}`))
	}))
	defer srv.Close()
	oauth := &OAuthClient{HTTP: srv.Client()}
	if _, err := oauth.ClientCredentials(context.Background(), srv.URL, "horizon", "bad", "secret"); err == nil || !strings.Contains(err.Error(), "HTTP 401") {
		t.Fatalf("status error = %v", err)
	}
	if _, err := oauth.RefreshToken(context.Background(), srv.URL, "horizon", "cli", "secret", "r"); err == nil || !strings.Contains(err.Error(), "invalid_grant") {
		t.Fatalf("body error = %v", err)
	}
}

func TestDeviceFlow(t *testing.T) {
	var polls int
	var opened string
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		form, _ := url.ParseQuery(string(body))
		switch {
		case strings.HasSuffix(r.URL.Path, "/auth/device"):
			if form.Get("code_challenge_method") != "S256" || form.Get("code_challenge") == "" || form.Get("scope") != "openid" {
				t.Errorf("device start form = %v", form)
			}
			_ = json.NewEncoder(w).Encode(deviceAuthStart{
				DeviceCode:              "device",
				UserCode:                "ABCD",
				VerificationURIComplete: srv.URL + "/verify",
				Interval:                1,
				ExpiresIn:               30,
			})
		default:
			polls++
			if form.Get("code_verifier") == "" || form.Get("device_code") != "device" {
				t.Errorf("poll form = %v", form)
			}
			if polls == 1 {
				_ = json.NewEncoder(w).Encode(tokenEndpointResponse{Error: "authorization_pending"})
				return
			}
			_ = json.NewEncoder(w).Encode(tokenEndpointResponse{AccessToken: "device-access", ExpiresIn: 20})
		}
	}))
	defer srv.Close()

	oauth := &OAuthClient{HTTP: srv.Client()}
	tok, err := oauth.DeviceFlow(context.Background(), srv.URL, "horizon", "cli", "", func(u string) error {
		opened = u
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if tok.AccessToken != "device-access" || opened != srv.URL+"/verify" || polls < 2 {
		t.Fatalf("token=%+v opened=%q polls=%d", tok, opened, polls)
	}
}

func TestDeviceFlowDenied(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/auth/device") {
			http.Error(w, "no", http.StatusBadRequest)
			return
		}
		t.Errorf("unexpected %s", r.URL.Path)
	}))
	defer srv.Close()
	oauth := &OAuthClient{HTTP: srv.Client()}
	if _, err := oauth.DeviceFlow(context.Background(), srv.URL, "horizon", "cli", "openid", nil); err == nil || !strings.Contains(err.Error(), "HTTP 400") {
		t.Fatalf("err = %v", err)
	}

	denied := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/auth/device") {
			_ = json.NewEncoder(w).Encode(deviceAuthStart{DeviceCode: "device", Interval: 1, ExpiresIn: 30})
			return
		}
		_ = json.NewEncoder(w).Encode(tokenEndpointResponse{Error: "access_denied", ErrorDescription: "user"})
	}))
	defer denied.Close()
	oauth = &OAuthClient{HTTP: denied.Client()}
	_, err := oauth.DeviceFlow(context.Background(), denied.URL, "horizon", "cli", "openid", nil)
	if err == nil || !strings.Contains(err.Error(), "access_denied") {
		t.Fatalf("err = %v", err)
	}
}
