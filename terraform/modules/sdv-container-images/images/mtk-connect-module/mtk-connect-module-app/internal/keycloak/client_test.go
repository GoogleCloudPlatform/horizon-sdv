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

package keycloak

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fakeKeycloak is a minimal in-memory Keycloak admin API.
type fakeKeycloak struct {
	client  map[string]interface{} // nil until created
	mappers []string
	puts    int
	posts   int
}

func (f *fakeKeycloak) handler(t *testing.T) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case path == "/auth/realms/master/protocol/openid-connect/token":
			_, _ = w.Write([]byte(`{"access_token":"tok"}`))
		case r.Header.Get("Authorization") != "Bearer tok":
			w.WriteHeader(http.StatusUnauthorized)
		case path == "/auth/admin/realms/horizon/keys":
			_, _ = w.Write([]byte(`{"keys":[
				{"type":"RSA","use":"SIG","status":"PASSIVE","certificate":"OLD"},
				{"type":"RSA","use":"SIG","status":"ACTIVE","certificate":"ACTIVE"}]}`))
		case path == "/auth/admin/realms/horizon/clients" && r.Method == http.MethodGet:
			if f.client == nil {
				_, _ = w.Write([]byte(`[]`))
				return
			}
			_ = json.NewEncoder(w).Encode([]interface{}{f.client})
		case path == "/auth/admin/realms/horizon/clients" && r.Method == http.MethodPost:
			f.posts++
			var body map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			body["id"] = "internal-1"
			attrs, _ := body["attributes"].(map[string]interface{})
			attrs["saml.signing.private.key"] = "PRIVKEY"
			f.client = body
			w.WriteHeader(http.StatusCreated)
		case path == "/auth/admin/realms/horizon/clients/internal-1" && r.Method == http.MethodPut:
			f.puts++
			var body map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			f.client = body
			w.WriteHeader(http.StatusNoContent)
		case path == "/auth/admin/realms/horizon/clients/internal-1/protocol-mappers/models" && r.Method == http.MethodGet:
			out := []map[string]string{}
			for _, m := range f.mappers {
				out = append(out, map[string]string{"name": m})
			}
			_ = json.NewEncoder(w).Encode(out)
		case path == "/auth/admin/realms/horizon/clients/internal-1/protocol-mappers/models" && r.Method == http.MethodPost:
			var body map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			f.mappers = append(f.mappers, body["name"].(string))
			w.WriteHeader(http.StatusCreated)
		default:
			t.Errorf("unexpected request %s %s", r.Method, path)
			w.WriteHeader(http.StatusNotFound)
		}
	})
}

func testSpec() SAMLClientSpec {
	return SAMLClientSpec{
		ClientID:     "mtk-connect",
		AdminURL:     "https://h.example.com/mtk-connect/saml/consume",
		RedirectURIs: []string{"https://h.example.com/mtk-connect/saml/consume"},
		Attributes:   map[string]string{"saml.assertion.signature": "true"},
		Mappers:      []ProtocolMapper{{Name: "X500 email", Protocol: "saml", ProtocolMapper: "saml-user-property-mapper"}},
	}
}

func TestEnsureSAMLClientCreatesAndIsIdempotent(t *testing.T) {
	f := &fakeKeycloak{}
	srv := httptest.NewServer(f.handler(t))
	defer srv.Close()

	ctx := context.Background()
	c := New(srv.URL+"/auth", srv.Client())
	if err := c.Login(ctx, "admin", "pw"); err != nil {
		t.Fatal(err)
	}
	m, err := c.EnsureSAMLClient(ctx, "horizon", testSpec())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(m.PrivateKey, "-----BEGIN RSA PRIVATE KEY-----\nPRIVKEY\n") {
		t.Errorf("private key PEM wrong: %q", m.PrivateKey)
	}
	if m.IDPCert != "-----BEGIN CERTIFICATE-----\nACTIVE\n-----END CERTIFICATE-----" {
		t.Errorf("must use the ACTIVE realm certificate, got %q", m.IDPCert)
	}
	if f.posts != 1 || len(f.mappers) != 1 {
		t.Fatalf("expected one client and one mapper, posts=%d mappers=%v", f.posts, f.mappers)
	}

	// Second run: nothing to write.
	m2, err := c.EnsureSAMLClient(ctx, "horizon", testSpec())
	if err != nil {
		t.Fatal(err)
	}
	if f.puts != 0 || f.posts != 1 || len(f.mappers) != 1 {
		t.Errorf("not idempotent: puts=%d posts=%d mappers=%v", f.puts, f.posts, f.mappers)
	}
	if m2 != m {
		t.Errorf("material changed between identical runs")
	}

	// Drift in the redirect URI is repaired with one update.
	f.client["redirectUris"] = []interface{}{"https://wrong"}
	if _, err := c.EnsureSAMLClient(ctx, "horizon", testSpec()); err != nil {
		t.Fatal(err)
	}
	if f.puts != 1 {
		t.Errorf("drift must trigger one PUT, got %d", f.puts)
	}
}

func TestLoginFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid_grant","password":"leak?"}`))
	}))
	defer srv.Close()
	err := New(srv.URL, srv.Client()).Login(context.Background(), "admin", "bad")
	if err == nil {
		t.Fatal("want error")
	}
	if strings.Contains(err.Error(), "leak?") {
		t.Errorf("login error must not echo the response body: %v", err)
	}
}

// A passive or disabled key is never used as the signing certificate.
func TestRealmCertificateRequiresAnActiveKey(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/protocol/openid-connect/token") {
			_, _ = w.Write([]byte(`{"access_token":"tok"}`))
			return
		}
		_, _ = w.Write([]byte(`{"keys":[
			{"type":"RSA","use":"SIG","status":"PASSIVE","certificate":"OLD"},
			{"type":"RSA","use":"SIG","status":"DISABLED","certificate":"OFF"}]}`))
	}))
	defer srv.Close()

	ctx := context.Background()
	c := New(srv.URL+"/auth", srv.Client())
	if err := c.Login(ctx, "admin", "pw"); err != nil {
		t.Fatal(err)
	}
	if cert, err := c.RealmCertificate(ctx, "horizon"); err == nil {
		t.Fatalf("got certificate %q, want an error without an active key", cert)
	}
}