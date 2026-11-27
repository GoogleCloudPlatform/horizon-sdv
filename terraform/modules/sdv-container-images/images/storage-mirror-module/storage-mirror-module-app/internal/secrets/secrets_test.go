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

package secrets

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

var cfg = Config{Namespace: "horizon", WorkflowNamespace: "workflows", HorizonAPINamespace: "horizon-api",
	TokenSecret: "storage-mirror-module-api-token", TLSSecret: "storage-mirror-module-api-tls", ServiceName: "storage-mirror-module"}

func newClient() client.Client {
	s := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(s)
	return fake.NewClientBuilder().WithScheme(s).Build()
}

func secret(t *testing.T, c client.Client, ns, name string) *corev1.Secret {
	t.Helper()
	var s corev1.Secret
	if err := c.Get(context.Background(), client.ObjectKey{Namespace: ns, Name: name}, &s); err != nil {
		t.Fatalf("%s/%s: %v", ns, name, err)
	}
	return &s
}

func TestEnsureCreatesAndReuses(t *testing.T) {
	c := newClient()
	ctx := context.Background()
	m1, err := Ensure(ctx, c, cfg)
	if err != nil {
		t.Fatal(err)
	}
	var tokens map[string]tokenEntry
	if err := json.Unmarshal([]byte(m1.TokensJSON), &tokens); err != nil {
		t.Fatal(err)
	}
	for _, caller := range Callers {
		if len(tokens[caller].Current) != 48 {
			t.Fatalf("token for %s: %q", caller, tokens[caller].Current)
		}
	}
	if string(secret(t, c, "workflows", cfg.TokenSecret).Data[CallerAgent]) != tokens[CallerAgent].Current {
		t.Fatal("agent token copy")
	}
	hapi := secret(t, c, "horizon-api", cfg.TokenSecret)
	if string(hapi.Data[CallerHorizonAPI]) != tokens[CallerHorizonAPI].Current || len(hapi.Data) != 1 {
		t.Fatalf("horizon-api copy must hold its own token only: %v", hapi.Data)
	}
	if s := secret(t, c, "horizon", cfg.TLSSecret); s.Type != corev1.SecretTypeTLS {
		t.Fatalf("tls secret type %s", s.Type)
	}
	// The certificate is valid for the Service name and signed by the copied CA.
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(secret(t, c, "workflows", cfg.TLSSecret).Data[KeyCA]) {
		t.Fatal("CA copy")
	}
	pair, err := tls.X509KeyPair(m1.CertPEM, m1.KeyPEM)
	if err != nil {
		t.Fatal(err)
	}
	leaf, _ := x509.ParseCertificate(pair.Certificate[0])
	if _, err := leaf.Verify(x509.VerifyOptions{DNSName: "storage-mirror-module.horizon.svc.cluster.local", Roots: pool}); err != nil {
		t.Fatalf("verify: %v", err)
	}

	// A second start reuses everything and restores a tampered copy.
	cp := secret(t, c, "workflows", cfg.TokenSecret)
	cp.Data[CallerAgent] = []byte("tampered")
	_ = c.Update(ctx, cp)
	m2, err := Ensure(ctx, c, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if m2.TokensJSON != m1.TokensJSON || string(m2.CertPEM) != string(m1.CertPEM) {
		t.Fatal("restart regenerated the tokens or the certificate")
	}
	if string(secret(t, c, "workflows", cfg.TokenSecret).Data[CallerAgent]) != tokens[CallerAgent].Current {
		t.Fatal("tampered copy not restored")
	}
}

func TestEnsureKeepsRotationEntries(t *testing.T) {
	c := newClient()
	ctx := context.Background()
	raw, _ := json.Marshal(map[string]tokenEntry{CallerHorizonAPI: {Current: "new-token", Previous: "old-token"}})
	s := &corev1.Secret{Data: map[string][]byte{KeyTokens: raw}}
	s.Name, s.Namespace = cfg.TokenSecret, cfg.Namespace
	if err := c.Create(ctx, s); err != nil {
		t.Fatal(err)
	}
	m, err := Ensure(ctx, c, cfg)
	if err != nil {
		t.Fatal(err)
	}
	var tokens map[string]tokenEntry
	_ = json.Unmarshal([]byte(m.TokensJSON), &tokens)
	if tokens[CallerHorizonAPI] != (tokenEntry{Current: "new-token", Previous: "old-token"}) || tokens[CallerAgent].Current == "" {
		t.Fatalf("tokens %+v", tokens)
	}
	if string(secret(t, c, "horizon-api", cfg.TokenSecret).Data[CallerHorizonAPI]) != "new-token" {
		t.Fatal("copy must carry the current token")
	}
}
