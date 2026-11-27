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

package controller

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"mtk-connect-module/internal/mtk"
)

var t0 = time.Date(2026, 1, 10, 12, 0, 0, 0, time.UTC)

func keyAt(id, value string, created time.Time) mtk.Key {
	return mtk.Key{ID: id, Value: value, Created: created}
}

func TestFindCurrentKey(t *testing.T) {
	keys := []mtk.Key{keyAt("1", "aaaaaaaa-old", t0), keyAt("2", "bbbbbbbb-new", t0.Add(time.Hour))}
	got, ok := findCurrentKey(keys, "bbbbbbbb-new-full-value")
	if !ok || got.ID != "2" {
		t.Fatalf("got %+v ok=%v, want key 2", got, ok)
	}
	if _, ok := findCurrentKey(keys, "zzzzzzzz"); ok {
		t.Fatal("unknown key must not match")
	}
	if _, ok := findCurrentKey(keys, "short"); ok {
		t.Fatal("too-short key must not match")
	}
	dup := append(keys, keyAt("3", "bbbbbbbb-other", t0))
	if _, ok := findCurrentKey(dup, "bbbbbbbb-new"); ok {
		t.Fatal("ambiguous prefix must not match")
	}
}

func TestKeysToDelete(t *testing.T) {
	overlap := 48 * time.Hour
	old := keyAt("old", "aaaaaaaa", t0)
	cur := keyAt("cur", "bbbbbbbb", t0.Add(24*time.Hour))
	newer := keyAt("newer", "cccccccc", t0.Add(30*time.Hour))
	unknown := mtk.Key{ID: "unknown", Value: "dddddddd"}
	keys := []mtk.Key{old, cur, newer, unknown}

	// Inside the overlap window nothing is deleted: the old key must keep working.
	if got := keysToDelete(keys, cur, true, overlap, cur.Created.Add(overlap-time.Minute)); got != nil {
		t.Fatalf("inside overlap: got %v, want none", got)
	}
	// After the overlap only keys older than the current key go.
	got := keysToDelete(keys, cur, true, overlap, cur.Created.Add(overlap))
	if !reflect.DeepEqual(got, []string{"old"}) {
		t.Fatalf("after overlap: got %v, want [old]", got)
	}
	// Never delete anything when the current key is not identified.
	if got := keysToDelete(keys, cur, false, overlap, t0.Add(1000*time.Hour)); got != nil {
		t.Fatalf("unidentified current key: got %v, want none", got)
	}
	if got := keysToDelete(keys, mtk.Key{ID: "cur"}, true, overlap, t0.Add(1000*time.Hour)); got != nil {
		t.Fatalf("current key without creation time: got %v, want none", got)
	}
}

// With the default timings (rotate every 24h, overlap 48h) the current key is never older than the
// overlap, so the overlap must be measured from when each key was replaced.
func TestKeysToDeleteRotatingFasterThanOverlap(t *testing.T) {
	overlap := 48 * time.Hour
	k0 := keyAt("k0", "aaaaaaaa", t0)
	k1 := keyAt("k1", "bbbbbbbb", t0.Add(24*time.Hour))
	k2 := keyAt("k2", "cccccccc", t0.Add(48*time.Hour))
	keys := []mtk.Key{k0, k1, k2}

	// k0 was replaced at 24h, so it must stay valid until 72h.
	if got := keysToDelete(keys, k2, true, overlap, t0.Add(72*time.Hour-time.Minute)); got != nil {
		t.Fatalf("before k0 overlap ends: got %v, want none", got)
	}
	// At 72h k0 goes; k1 (replaced at 48h) stays until 96h.
	if got := keysToDelete(keys, k2, true, overlap, t0.Add(72*time.Hour)); !reflect.DeepEqual(got, []string{"k0"}) {
		t.Fatalf("at 72h: got %v, want [k0]", got)
	}
	got := keysToDelete(keys, k2, true, overlap, t0.Add(96*time.Hour))
	if !reflect.DeepEqual(got, []string{"k0", "k1"}) {
		t.Fatalf("at 96h: got %v, want [k0 k1]", got)
	}
}

func TestRotationDue(t *testing.T) {
	interval := 24 * time.Hour
	cases := []struct {
		name  string
		last  time.Time
		now   time.Time
		force bool
		want  bool
	}{
		{"not yet", t0, t0.Add(interval - time.Second), false, false},
		{"exactly due", t0, t0.Add(interval), false, true},
		{"overdue", t0, t0.Add(3 * interval), false, true},
		{"forced", t0, t0.Add(time.Minute), true, true},
		{"unknown last is not due", time.Time{}, t0, false, false},
	}
	for _, c := range cases {
		if got := rotationDue(c.last, c.now, interval, c.force); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestParseSpecDefaultsAndValidation(t *testing.T) {
	obj := func(spec map[string]interface{}) *unstructured.Unstructured {
		return &unstructured.Unstructured{Object: map[string]interface{}{"spec": spec}}
	}
	valid := func() map[string]interface{} {
		return map[string]interface{}{
			"domain": "https://horizon.example.com",
			"keycloak": map[string]interface{}{
				"url":            "http://kc.kc.svc.cluster.local:8080/auth",
				"adminSecretRef": map[string]interface{}{"namespace": "kc", "name": "creds"},
			},
			"server": map[string]interface{}{"url": "http://router.mtk.svc.cluster.local:8081"},
			"credentialDistribution": []interface{}{
				map[string]interface{}{"namespace": "jenkins", "secretName": "j", "jenkinsCredential": true},
			},
		}
	}

	spec, timings, err := parseSpec(obj(valid()))
	if err != nil {
		t.Fatalf("valid spec rejected: %v", err)
	}
	if spec.Domain != "horizon.example.com" {
		t.Errorf("domain scheme not stripped: %q", spec.Domain)
	}
	if spec.ServiceAccountUser != "mtk-connect-admin" || spec.Keycloak.Realm != "horizon" ||
		spec.Server.APIKeySecret != "mtk-connect-apikey" || spec.Keycloak.AdminSecretRef.Key != "password" {
		t.Errorf("defaults not applied: %+v", spec)
	}
	if timings.Interval != 24*time.Hour || timings.KeyLifetime != 720*time.Hour || timings.Overlap != 48*time.Hour {
		t.Errorf("unexpected timings: %+v", timings)
	}

	bad := valid()
	bad["keyRotation"] = map[string]interface{}{"interval": "48h", "keyLifetime": "24h"}
	if _, _, err := parseSpec(obj(bad)); err == nil {
		t.Error("keyLifetime <= interval must be rejected")
	}
	bad = valid()
	delete(bad, "domain")
	if _, _, err := parseSpec(obj(bad)); err == nil {
		t.Error("missing domain must be rejected")
	}
	bad = valid()
	bad["keyRotation"] = map[string]interface{}{"interval": "tomorrow"}
	if _, _, err := parseSpec(obj(bad)); err == nil {
		t.Error("bad duration must be rejected")
	}
}

// The module's Role only allows the default Deployment and Secret names, so an override is refused up front
// instead of failing with a 403 later.
func TestParseSpecRefusesRenamedModuleObjects(t *testing.T) {
	for field, name := range map[string]string{"deployment": "other", "keycloakSecret": "other", "apiKeySecret": "other"} {
		t.Run(field, func(t *testing.T) {
			server := map[string]interface{}{"url": "http://router.mtk.svc:8081", field: name}
			obj := &unstructured.Unstructured{Object: map[string]interface{}{"spec": map[string]interface{}{
				"domain": "horizon.example.com",
				"keycloak": map[string]interface{}{
					"url":            "http://kc.kc.svc:8080/auth",
					"adminSecretRef": map[string]interface{}{"namespace": "kc", "name": "creds"},
				},
				"server": server,
			}}}
			if _, _, err := parseSpec(obj); err == nil {
				t.Fatalf("spec.server.%s=%q must be rejected", field, name)
			}
		})
	}
}

// The controller sends the Keycloak admin password and the MTK Connect API key to the URLs in the spec,
// so a URL that leaves the cluster must be refused before anything is sent.
func TestParseSpecRefusesUnsafeTargets(t *testing.T) {
	build := func(domain, kcURL, kcNamespace, serverURL string) *unstructured.Unstructured {
		return &unstructured.Unstructured{Object: map[string]interface{}{"spec": map[string]interface{}{
			"domain": domain,
			"keycloak": map[string]interface{}{
				"url":            kcURL,
				"adminSecretRef": map[string]interface{}{"namespace": kcNamespace, "name": "creds"},
			},
			"server": map[string]interface{}{"url": serverURL},
		}}}
	}
	const (
		okDomain = "horizon.example.com"
		okKC     = "http://kc.kc.svc.cluster.local:8080/auth"
		okServer = "http://router.mtk.svc:8081"
	)
	if _, _, err := parseSpec(build(okDomain, okKC, "kc", okServer)); err != nil {
		t.Fatalf("valid targets rejected: %v", err)
	}
	cases := []struct {
		name                         string
		domain, kc, kcNamespace, srv string
	}{
		{"external Keycloak", okDomain, "https://evil.example.com/auth", "kc", okServer},
		{"bare host", okDomain, "http://kc:8080/auth", "kc", okServer},
		{"user info", okDomain, "http://admin:x@kc.kc.svc:8080/auth", "kc", okServer},
		{"query", okDomain, "http://kc.kc.svc:8080/auth?x=1", "kc", okServer},
		{"scheme", okDomain, "ftp://kc.kc.svc/auth", "kc", okServer},
		{"secret namespace differs", okDomain, okKC, "other", okServer},
		{"external server", okDomain, okKC, "kc", "https://evil.example.com"},
		{"domain with path", "horizon.example.com/x", okKC, "kc", okServer},
		{"domain with user info", "evil.com@horizon.example.com", okKC, "kc", okServer},
		{"domain with space", "horizon example.com", okKC, "kc", okServer},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := parseSpec(build(tc.domain, tc.kc, tc.kcNamespace, tc.srv)); err == nil {
				t.Fatal("want the spec to be rejected")
			}
		})
	}
}

func TestCredentialSecrets(t *testing.T) {
	spec := &InstanceSpec{
		ServiceAccountUser: "mtk-connect-admin",
		Server:             ServerSpec{APIKeySecret: "mtk-connect-apikey"},
		CredentialDistribution: []CredentialTarget{
			{Namespace: "jenkins", SecretName: "jenkins-mtk-connect-apikey", JenkinsCredential: true},
			{Namespace: "workflows", SecretName: "workflow-mtk-connect-apikey",
				Labels: map[string]string{"horizon-sdv.io/credentials-purpose": "mtk-connect-argowf"}},
			{Namespace: "mtk", SecretName: "mtk-connect-apikey"}, // duplicate of the server Secret
		},
	}
	got := credentialSecrets("mtk", spec, "KEY")
	if len(got) != 3 {
		t.Fatalf("got %d secrets, want 3 (server + 2 targets)", len(got))
	}
	if got[0].Namespace != "mtk" || got[0].Name != "mtk-connect-apikey" {
		t.Errorf("first secret must be the server Secret: %+v", got[0])
	}
	j := got[1]
	if j.Labels[jenkinsCredentialsTypeLabel] != "usernamePassword" || j.Annotations[jenkinsCredentialsDescriptionKey] == "" {
		t.Errorf("jenkins credential metadata missing: %+v", j)
	}
	if got[2].Labels["horizon-sdv.io/credentials-purpose"] != "mtk-connect-argowf" {
		t.Errorf("workflow label missing: %+v", got[2])
	}
	for _, d := range got {
		if string(d.Data[secretKeyPassword]) != "KEY" || string(d.Data[secretKeyUsername]) != "mtk-connect-admin" {
			t.Errorf("wrong data in %s/%s", d.Namespace, d.Name)
		}
	}
}

func TestHashStringsIsUnambiguous(t *testing.T) {
	if hashStrings("ab", "c") == hashStrings("a", "bc") {
		t.Fatal("hash must distinguish value boundaries")
	}
	if hashStrings("x", "y") != hashStrings("x", "y") {
		t.Fatal("hash must be stable")
	}
}

func TestServerUnavailable(t *testing.T) {
	transport := fmt.Errorf("mtk-connect GET /users: %w", &url.Error{Op: "Get", URL: "http://router", Err: errors.New("connection refused")})
	if !serverUnavailable(transport) {
		t.Error("transport error must count as server unavailable")
	}
	if !serverUnavailable(fmt.Errorf("wrapped: %w", context.DeadlineExceeded)) {
		t.Error("a timeout must count as server unavailable")
	}
	if !serverUnavailable(&mtk.APIError{Status: 503}) {
		t.Error("5xx must count as server unavailable")
	}
	if serverUnavailable(&mtk.APIError{Status: 404}) || serverUnavailable(&mtk.APIError{Status: 401}) {
		t.Error("4xx is an answer from the server, not unavailability")
	}
	if serverUnavailable(fmt.Errorf("%w: %q", mtk.ErrUserNotFound, "mtk-connect-admin")) {
		t.Error("a missing service account is an answer from the server, not unavailability")
	}
	if serverUnavailable(errors.New("mtk-connect GET /users: decode response: unexpected end of JSON input")) {
		t.Error("an unreadable answer is not unavailability")
	}
}

func TestRejectionsAreCountedPerInstance(t *testing.T) {
	r := &InstanceReconciler{}
	if n := r.noteRejected("ns/a"); n != 1 {
		t.Fatalf("first rejection: got %d, want 1", n)
	}
	if n := r.noteRejected("ns/b"); n != 1 {
		t.Fatalf("other instance: got %d, want 1", n)
	}
	if n := r.noteRejected("ns/a"); n != rejectionsBeforeBootstrap {
		t.Fatalf("second rejection: got %d, want %d", n, rejectionsBeforeBootstrap)
	}
	r.resetRejected("ns/a")
	if n := r.noteRejected("ns/a"); n != 1 {
		t.Fatalf("after reset: got %d, want 1", n)
	}
}

func TestStatusPhase(t *testing.T) {
	mk := func(prev string, conds ...*condition) *instanceStatus {
		s := &instanceStatus{Phase: prev, Conditions: map[string]*condition{}}
		for _, c := range conds {
			s.Conditions[c.Type] = c
		}
		return s
	}
	if got := mk("", &condition{Type: CondSpecValid, Status: true}).phase(); got != PhaseReady {
		t.Errorf("all true: got %s", got)
	}
	waiting := &condition{Type: CondServiceAccountReady, Reason: reasonServerNotReady}
	if got := mk("", waiting).phase(); got != PhaseConfiguring {
		t.Errorf("first wait for server: got %s", got)
	}
	if got := mk(PhaseReady, waiting).phase(); got != PhaseDegraded {
		t.Errorf("server lost after Ready: got %s", got)
	}
	failed := &condition{Type: CondKeyRotated, Reason: "RotationFailed"}
	if got := mk("", failed).phase(); got != PhaseDegraded {
		t.Errorf("rotation failure: got %s", got)
	}
}
