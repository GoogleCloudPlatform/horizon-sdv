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
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"mtk-connect-module/internal/mtk"
)

const (
	tbManagerNS = "mtk-connect"
	tbNS        = "workflows"
	tbName      = "bench-a"
	tbUID       = "11111111-2222-3333-4444-555555555555"
)

// fakeMTK is an in-memory MTK Connect that records what the reconciler asked for.
type fakeMTK struct {
	agents        map[string]*mtk.Agent // by registration
	creates       int
	registrations int
	tokens        []string // the token of each registration call, in order
	deletes       int
	regDeletes    int
	grants        []string
	createErr     error
	grantErr      error
	nextID        int
	keys          []mtk.Key
	revoked       []string
	keySeq        int
}

func newFakeMTK() *fakeMTK { return &fakeMTK{agents: map[string]*mtk.Agent{}, nextID: 1} }

func (f *fakeMTK) FindAgent(_ context.Context, registration string) (*mtk.Agent, error) {
	if a, ok := f.agents[registration]; ok {
		c := *a
		return &c, nil
	}
	return nil, nil
}

func (f *fakeMTK) EnsureRegistration(_ context.Context, _, token string) error {
	f.registrations++
	f.tokens = append(f.tokens, token)
	return nil
}

func (f *fakeMTK) CreateAgent(_ context.Context, name, registration string) (mtk.Agent, error) {
	if f.createErr != nil {
		return mtk.Agent{}, f.createErr
	}
	f.creates++
	a := &mtk.Agent{ID: fmt.Sprintf("agent-%d", f.nextID), Name: name, Registration: registration}
	f.nextID++
	f.agents[registration] = a
	return *a, nil
}

func (f *fakeMTK) DeleteRegistration(_ context.Context, _ string) error {
	f.regDeletes++
	return nil
}

func (f *fakeMTK) DeleteAgent(_ context.Context, id string) error {
	for reg, a := range f.agents {
		if a.ID == id {
			delete(f.agents, reg)
			f.deletes++
		}
	}
	return nil
}

func (f *fakeMTK) GrantGroupByName(_ context.Context, agentID, group, permission string) error {
	if f.grantErr != nil {
		return f.grantErr
	}
	f.grants = append(f.grants, fmt.Sprintf("group:%s:%s:%s", agentID, group, permission))
	return nil
}

func (f *fakeMTK) GrantUserByName(_ context.Context, agentID, username, permission string) error {
	if f.grantErr != nil {
		return f.grantErr
	}
	f.grants = append(f.grants, fmt.Sprintf("user:%s:%s:%s", agentID, username, permission))
	return nil
}

func (f *fakeMTK) EnsureUser(context.Context, string) (string, error) { return "user-agent", nil }

func (f *fakeMTK) CreateKey(_ context.Context, _ string, expiry time.Time) (string, error) {
	f.keySeq++
	val := fmt.Sprintf("%08d-agent-key", f.keySeq)
	f.keys = append(f.keys, mtk.Key{ID: fmt.Sprintf("k%d", f.keySeq), Value: val, Created: expiry.Add(-time.Hour), Expires: expiry})
	return val, nil
}

func (f *fakeMTK) Keys(context.Context, string) ([]mtk.Key, error) {
	out := make([]mtk.Key, len(f.keys))
	copy(out, f.keys)
	return out, nil
}

func (f *fakeMTK) DeleteKey(_ context.Context, _, keyID string) error {
	f.revoked = append(f.revoked, keyID)
	kept := f.keys[:0]
	for _, k := range f.keys {
		if k.ID != keyID {
			kept = append(kept, k)
		}
	}
	f.keys = kept
	return nil
}

func tbRegistration() string { return registrationFor(tbNS, tbName, tbUID) }

func instanceObject() *unstructured.Unstructured {
	u := &unstructured.Unstructured{Object: map[string]interface{}{
		"spec": map[string]interface{}{
			"domain": "horizon.example.com",
			"keycloak": map[string]interface{}{
				"url":            "http://keycloak.keycloak.svc.cluster.local:8080/auth",
				"adminSecretRef": map[string]interface{}{"namespace": "keycloak", "name": "keycloak-initial-creds"},
			},
			"server": map[string]interface{}{"url": "http://mtk-connect-router.mtk-connect.svc:8081"},
		},
	}}
	u.SetGroupVersionKind(InstanceGVK)
	u.SetNamespace(tbManagerNS)
	u.SetName("default")
	return u
}

func apiKeySecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: tbManagerNS, Name: "mtk-connect-apikey"},
		Data:       map[string][]byte{secretKeyPassword: []byte("key")},
	}
}

func testbenchObject(spec map[string]interface{}) *unstructured.Unstructured {
	u := &unstructured.Unstructured{Object: map[string]interface{}{"spec": spec}}
	u.SetGroupVersionKind(TestbenchGVK)
	u.SetNamespace(tbNS)
	u.SetName(tbName)
	u.SetUID(types.UID(tbUID))
	u.SetLabels(map[string]string{LabelSubmittedFrom: "developer-portal"})
	return u
}

func publicSpec() map[string]interface{} {
	return map[string]interface{}{
		"displayName": "Bench A",
		"access":      map[string]interface{}{"public": true},
	}
}

type tbEnv struct {
	r   *TestbenchReconciler
	c   client.Client
	api *fakeMTK
}

func newTbEnv(t *testing.T, objs ...client.Object) *tbEnv {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	statusObj := &unstructured.Unstructured{}
	statusObj.SetGroupVersionKind(TestbenchGVK)
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).WithStatusSubresource(statusObj).Build()
	api := newFakeMTK()
	return &tbEnv{
		c:   c,
		api: api,
		r: &TestbenchReconciler{
			Client:           c,
			Direct:           c,
			ManagerNamespace: tbManagerNS,
			JenkinsNamespace: "jenkins",
			NewAPI:           func(_, _, _ string) TestbenchAPI { return api },
			Now:              func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) },
		},
	}
}

func (e *tbEnv) reconcile(t *testing.T) ctrl.Result {
	t.Helper()
	res, err := e.r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: tbNS, Name: tbName}})
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	return res
}

func (e *tbEnv) get(t *testing.T) *unstructured.Unstructured {
	t.Helper()
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(TestbenchGVK)
	if err := e.c.Get(context.Background(), types.NamespacedName{Namespace: tbNS, Name: tbName}, u); err != nil {
		t.Fatalf("get testbench: %v", err)
	}
	return u
}

func statusString(u *unstructured.Unstructured, field string) string {
	s, _, _ := unstructured.NestedString(u.Object, "status", field)
	return s
}

func condStatus(u *unstructured.Unstructured, typ string) (status, reason string) {
	conds, _, _ := unstructured.NestedSlice(u.Object, "status", "conditions")
	for _, c := range conds {
		m := c.(map[string]interface{})
		if m["type"] == typ {
			status, _ = m["status"].(string)
			reason, _ = m["reason"].(string)
		}
	}
	return status, reason
}

func (e *tbEnv) events(t *testing.T) []string {
	t.Helper()
	var list corev1.EventList
	if err := e.c.List(context.Background(), &list); err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, ev := range list.Items {
		out = append(out, ev.Reason)
	}
	return out
}

func TestTestbenchReachesReady(t *testing.T) {
	e := newTbEnv(t, instanceObject(), apiKeySecret(), testbenchObject(publicSpec()))

	res := e.reconcile(t)
	got := e.get(t)
	reg := tbRegistration()
	if got.GetFinalizers()[0] != FinalizerAgentCleanup {
		t.Fatalf("finalizers = %v", got.GetFinalizers())
	}
	if e.api.registrations != 1 {
		t.Fatalf("registrations=%d, want the registration created before the agent", e.api.registrations)
	}
	if e.api.creates != 1 || e.api.agents[reg] == nil || e.api.agents[reg].Name != "Bench A" {
		t.Fatalf("agent not created as expected: creates=%d agents=%v", e.api.creates, e.api.agents)
	}
	if len(e.api.grants) != 1 || e.api.grants[0] != "group:agent-1:everyone:book" {
		t.Fatalf("grants = %v", e.api.grants)
	}
	if p := statusString(got, "phase"); p != TbAwaitingAgent {
		t.Fatalf("phase = %q, want %s", p, TbAwaitingAgent)
	}
	if statusString(got, "registration") != reg || statusString(got, "agentId") != "agent-1" || statusString(got, "registrationTokenSecret") != tbManagerNS+"/"+testbenchTokenSecret {
		t.Fatalf("status = %v", got.Object["status"])
	}
	if want := "https://horizon.example.com/mtk-connect/"; statusString(got, "url") != want {
		t.Fatalf("url = %q, want %q", statusString(got, "url"), want)
	}
	if got, want := testbenchURL("d.example.com", "/mtk-connect/tb/{id}", "7"), "https://d.example.com/mtk-connect/tb/7"; got != want {
		t.Fatalf("testbenchURL = %q, want %q", got, want)
	}
	if res.RequeueAfter != testbenchPollDelay {
		t.Fatalf("requeue = %v", res.RequeueAfter)
	}

	// The agent connects.
	e.api.agents[reg].Online = true
	res = e.reconcile(t)
	got = e.get(t)
	if p := statusString(got, "phase"); p != TbReady {
		t.Fatalf("phase = %q, want %s", p, TbReady)
	}
	if s, _ := condStatus(got, TbCondAgentOnline); s != "True" {
		t.Fatalf("AgentOnline = %q", s)
	}
	if e.api.creates != 1 || len(e.api.grants) != 1 {
		t.Fatalf("second reconcile repeated work: creates=%d grants=%v", e.api.creates, e.api.grants)
	}
	if res.RequeueAfter != testbenchOnlineDelay {
		t.Fatalf("requeue = %v", res.RequeueAfter)
	}

	// It goes away.
	e.api.agents[reg].Online = false
	e.reconcile(t)
	if p := statusString(e.get(t), "phase"); p != TbOffline {
		t.Fatalf("phase = %q, want %s", p, TbOffline)
	}
}

func TestTestbenchGrantsUsers(t *testing.T) {
	spec := map[string]interface{}{"access": map[string]interface{}{"users": []interface{}{"alice@example.com", " bob@example.com ", "alice@example.com"}, "permission": "book"}}
	e := newTbEnv(t, instanceObject(), apiKeySecret(), testbenchObject(spec))
	e.reconcile(t)
	want := []string{"user:agent-1:alice@example.com:book", "user:agent-1:bob@example.com:book"}
	if strings.Join(e.api.grants, ",") != strings.Join(want, ",") {
		t.Fatalf("grants = %v, want %v", e.api.grants, want)
	}
	if name := e.api.agents[tbRegistration()].Name; name != tbName {
		t.Fatalf("agent name = %q, want default %q", name, tbName)
	}
}

// A controller restart after the agent was created but before the status was written must adopt the agent.
func TestTestbenchRestartCreatesNoDuplicate(t *testing.T) {
	e := newTbEnv(t, instanceObject(), apiKeySecret(), testbenchObject(publicSpec()))
	reg := tbRegistration()
	e.api.agents[reg] = &mtk.Agent{ID: "agent-existing", Name: "Bench A", Registration: reg}

	e.reconcile(t)
	e.reconcile(t)

	if e.api.creates != 0 || len(e.api.agents) != 1 {
		t.Fatalf("duplicate agent: creates=%d agents=%v", e.api.creates, e.api.agents)
	}
	if id := statusString(e.get(t), "agentId"); id != "agent-existing" {
		t.Fatalf("agentId = %q", id)
	}
	if len(e.api.grants) != 1 {
		t.Fatalf("access should be granted once, grants = %v", e.api.grants)
	}
}

func TestTestbenchDeletionRemovesAgent(t *testing.T) {
	e := newTbEnv(t, instanceObject(), apiKeySecret(), testbenchObject(publicSpec()))
	e.reconcile(t)
	if err := e.c.Delete(context.Background(), e.get(t)); err != nil {
		t.Fatal(err)
	}

	e.reconcile(t)

	if e.api.deletes != 1 || e.api.regDeletes != 1 || len(e.api.agents) != 0 {
		t.Fatalf("agent not deleted: deletes=%d registrations=%d agents=%v", e.api.deletes, e.api.regDeletes, e.api.agents)
	}
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(TestbenchGVK)
	err := e.c.Get(context.Background(), types.NamespacedName{Namespace: tbNS, Name: tbName}, u)
	if !apierrors.IsNotFound(err) {
		t.Fatalf("testbench should be gone, err=%v", err)
	}
}

func TestTestbenchDeletionBlockedWithoutInstance(t *testing.T) {
	e := newTbEnv(t, instanceObject(), apiKeySecret(), testbenchObject(publicSpec()))
	e.reconcile(t)

	inst := instanceObject()
	if err := e.c.Delete(context.Background(), inst); err != nil {
		t.Fatal(err)
	}
	if err := e.c.Delete(context.Background(), e.get(t)); err != nil {
		t.Fatal(err)
	}

	res := e.reconcile(t)
	if res.RequeueAfter == 0 || e.api.deletes != 0 {
		t.Fatalf("deletion must wait: requeue=%v deletes=%d", res.RequeueAfter, e.api.deletes)
	}
	if got := e.get(t); len(got.GetFinalizers()) != 1 {
		t.Fatalf("finalizer must stay, got %v", got.GetFinalizers())
	}

	// The escape hatch.
	got := e.get(t)
	got.SetAnnotations(map[string]string{AnnotationSkipCleanup: "true"})
	if err := e.c.Update(context.Background(), got); err != nil {
		t.Fatal(err)
	}
	e.reconcile(t)
	if len(e.api.revoked) != 0 {
		t.Fatalf("the key cannot be revoked without the instance, revoked=%v", e.api.revoked)
	}
	if err := e.c.Get(context.Background(), types.NamespacedName{Namespace: "jenkins", Name: jenkinsCredentialSecretName(tbNS, tbName)}, &corev1.Secret{}); !apierrors.IsNotFound(err) {
		t.Fatalf("the jenkins credential must be removed even when cleanup is skipped, err=%v", err)
	}
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(TestbenchGVK)
	if err := e.c.Get(context.Background(), types.NamespacedName{Namespace: tbNS, Name: tbName}, u); !apierrors.IsNotFound(err) {
		t.Fatalf("testbench should be gone after the skip annotation, err=%v", err)
	}
}

func TestSkipCleanupRevokesTheKeyWithoutDeletingTheAgent(t *testing.T) {
	e := newTbEnv(t, instanceObject(), apiKeySecret(), testbenchObject(publicSpec()))
	e.reconcile(t)

	if err := e.c.Delete(context.Background(), e.get(t)); err != nil {
		t.Fatal(err)
	}
	got := e.get(t)
	got.SetAnnotations(map[string]string{AnnotationSkipCleanup: "true"})
	if err := e.c.Update(context.Background(), got); err != nil {
		t.Fatal(err)
	}
	e.reconcile(t)

	if e.api.deletes != 0 || e.api.regDeletes != 0 {
		t.Fatalf("skip must leave the agent and registration: deletes=%d registrations=%d", e.api.deletes, e.api.regDeletes)
	}
	if len(e.api.revoked) != 1 || e.api.revoked[0] != "k1" {
		t.Fatalf("skip must still revoke the host key, revoked=%v", e.api.revoked)
	}
	if err := e.c.Get(context.Background(), types.NamespacedName{Namespace: "jenkins", Name: jenkinsCredentialSecretName(tbNS, tbName)}, &corev1.Secret{}); !apierrors.IsNotFound(err) {
		t.Fatalf("the jenkins credential must be removed, err=%v", err)
	}
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(TestbenchGVK)
	if err := e.c.Get(context.Background(), types.NamespacedName{Namespace: tbNS, Name: tbName}, u); !apierrors.IsNotFound(err) {
		t.Fatalf("testbench should be gone after the skip annotation, err=%v", err)
	}
}

func TestTestbenchWaitsForInstance(t *testing.T) {
	e := newTbEnv(t, testbenchObject(publicSpec()))

	res := e.reconcile(t)
	got := e.get(t)
	if e.api.creates != 0 {
		t.Fatal("no agent may be created without an MTKConnectInstance")
	}
	if p := statusString(got, "phase"); p != TbPending {
		t.Fatalf("phase = %q", p)
	}
	if s, reason := condStatus(got, TbCondInstanceReady); s != "False" || reason != reasonInstanceNotReady {
		t.Fatalf("InstanceReady = %q/%q", s, reason)
	}
	if res.RequeueAfter != testbenchRetryDelay {
		t.Fatalf("requeue = %v", res.RequeueAfter)
	}
	if len(got.GetFinalizers()) != 1 {
		t.Fatalf("finalizer should be set before anything else, got %v", got.GetFinalizers())
	}
}

func TestTestbenchInvalidSpecFails(t *testing.T) {
	e := newTbEnv(t, instanceObject(), apiKeySecret(), testbenchObject(map[string]interface{}{"access": map[string]interface{}{}}))

	res := e.reconcile(t)
	got := e.get(t)
	if p := statusString(got, "phase"); p != TbFailed {
		t.Fatalf("phase = %q", p)
	}
	if s, reason := condStatus(got, TbCondSpecValid); s != "False" || reason != "InvalidSpec" {
		t.Fatalf("SpecValid = %q/%q", s, reason)
	}
	if e.api.creates != 0 || res.RequeueAfter != 0 {
		t.Fatalf("invalid spec must not create anything or poll: creates=%d requeue=%v", e.api.creates, res.RequeueAfter)
	}
	if evs := e.events(t); len(evs) != 1 || evs[0] != "InvalidSpec" {
		t.Fatalf("events = %v", evs)
	}
}

func TestTestbenchGrantFailureIsRetried(t *testing.T) {
	e := newTbEnv(t, instanceObject(), apiKeySecret(), testbenchObject(publicSpec()))
	e.api.grantErr = errors.New("boom")

	res := e.reconcile(t)
	got := e.get(t)
	if s, reason := condStatus(got, TbCondAccessGranted); s != "False" || reason != "GrantFailed" {
		t.Fatalf("AccessGranted = %q/%q", s, reason)
	}
	if res.RequeueAfter != testbenchRetryDelay {
		t.Fatalf("requeue = %v", res.RequeueAfter)
	}
	if b, _, _ := unstructured.NestedBool(got.Object, "status", "accessGranted"); b {
		t.Fatal("accessGranted must stay false")
	}

	e.api.grantErr = nil
	e.reconcile(t)
	if len(e.api.grants) != 1 || e.api.creates != 1 {
		t.Fatalf("retry must grant on the same agent: grants=%v creates=%d", e.api.grants, e.api.creates)
	}
	if b, _, _ := unstructured.NestedBool(e.get(t).Object, "status", "accessGranted"); !b {
		t.Fatal("accessGranted must be true after the retry")
	}
}

func TestTestbenchMissingAgentIsNotRecreated(t *testing.T) {
	e := newTbEnv(t, instanceObject(), apiKeySecret(), testbenchObject(publicSpec()))
	e.reconcile(t)
	delete(e.api.agents, tbRegistration())

	e.reconcile(t)

	got := e.get(t)
	if e.api.creates != 1 {
		t.Fatalf("agent must not be recreated, creates=%d", e.api.creates)
	}
	if p := statusString(got, "phase"); p != TbFailed {
		t.Fatalf("phase = %q", p)
	}
	if _, reason := condStatus(got, TbCondAgentProvision); reason != "AgentMissing" {
		t.Fatalf("reason = %q", reason)
	}
}

func TestParseTestbenchSpec(t *testing.T) {
	cases := []struct {
		name string
		spec map[string]interface{}
		ok   bool
	}{
		{"public", map[string]interface{}{"access": map[string]interface{}{"public": true}}, true},
		{"users", map[string]interface{}{"access": map[string]interface{}{"users": []interface{}{"a@b.c"}}}, true},
		{"no access", map[string]interface{}{"access": map[string]interface{}{}}, false},
		{"blank user only", map[string]interface{}{"access": map[string]interface{}{"users": []interface{}{"  "}}}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseTestbenchSpec(testbenchObject(tc.spec))
			if (err == nil) != tc.ok {
				t.Fatalf("err = %v, want ok=%v", err, tc.ok)
			}
		})
	}
}

func TestParseTestbenchSpecRequiresOrigin(t *testing.T) {
	u := testbenchObject(map[string]interface{}{"access": map[string]interface{}{"public": true}})
	u.SetLabels(nil)
	if _, err := parseTestbenchSpec(u); err == nil {
		t.Fatal("a testbench without horizon-sdv.io/submitted-from must be rejected")
	}
}

func TestRegistrationIsStableAndPerResource(t *testing.T) {
	a := registrationFor("ns", "n", "uid-1")
	if a != registrationFor("ns", "n", "uid-1") {
		t.Fatal("registration must be deterministic")
	}
	if a == registrationFor("ns", "n", "uid-2") {
		t.Fatal("a re-created resource must get a new registration")
	}
	if !strings.HasPrefix(a, registrationPrefix) || len(a) != len(registrationPrefix)+registrationIDLen {
		t.Fatalf("registration = %q", a)
	}
}

func (e *tbEnv) storedTokens(t *testing.T) map[string]string {
	t.Helper()
	var s corev1.Secret
	err := e.c.Get(context.Background(), types.NamespacedName{Namespace: tbManagerNS, Name: testbenchTokenSecret}, &s)
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for k, v := range s.Data {
		out[k] = string(v)
	}
	return out
}

func TestNewRegistrationTokenIsRandomUUID(t *testing.T) {
	a, err := newRegistrationToken()
	if err != nil {
		t.Fatal(err)
	}
	b, _ := newRegistrationToken()
	if !uuidPattern.MatchString(a) || a == b {
		t.Fatalf("tokens %q and %q must be distinct UUIDs", a, b)
	}
	if a[14] != '4' {
		t.Fatalf("token %q is not a version 4 UUID", a)
	}
}

// The token must not be derivable from the resource, must not be readable from the testbench, and must be
// the same on every retry.
func TestRegistrationTokenIsRandomStoredAndNotInStatus(t *testing.T) {
	e := newTbEnv(t, instanceObject(), apiKeySecret(), testbenchObject(publicSpec()))
	e.reconcile(t)

	reg := tbRegistration()
	stored := e.storedTokens(t)[reg]
	if !uuidPattern.MatchString(stored) || stored == tbUID {
		t.Fatalf("stored token %q must be a random UUID, not the resource UID", stored)
	}
	if len(e.api.tokens) != 1 || e.api.tokens[0] != stored {
		t.Fatalf("registration sent %v, want the stored token %q", e.api.tokens, stored)
	}
	got := e.get(t)
	if raw, _ := json.Marshal(got.Object["status"]); strings.Contains(string(raw), stored) || strings.Contains(string(raw), "registrationToken\"") {
		t.Fatalf("the token must not be in status: %s", raw)
	}
}

// A retry after a failure between storing the token and creating the agent must send the same token.
func TestRegistrationTokenIsReusedOnRetry(t *testing.T) {
	e := newTbEnv(t, instanceObject(), apiKeySecret(), testbenchObject(publicSpec()))
	e.api.createErr = errors.New("boom")
	e.reconcile(t)
	e.api.createErr = nil
	e.reconcile(t)

	if len(e.api.tokens) != 2 || e.api.tokens[0] != e.api.tokens[1] {
		t.Fatalf("a retry must send the stored token, got %v", e.api.tokens)
	}
}

func TestRegistrationTokensAreIndependentPerTestbench(t *testing.T) {
	e := newTbEnv(t, apiKeySecret())
	a, err := e.r.registrationToken(context.Background(), "hz-aaaa")
	if err != nil {
		t.Fatal(err)
	}
	b, err := e.r.registrationToken(context.Background(), "hz-bbbb")
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Fatal("two registrations must not share a token")
	}
	again, _ := e.r.registrationToken(context.Background(), "hz-aaaa")
	if again != a {
		t.Fatalf("the token changed on a second read: %q != %q", again, a)
	}
}

func TestDeletingTestbenchForgetsItsToken(t *testing.T) {
	e := newTbEnv(t, instanceObject(), apiKeySecret(), testbenchObject(publicSpec()))
	e.reconcile(t)
	if len(e.storedTokens(t)) != 1 {
		t.Fatalf("tokens = %v, want one", e.storedTokens(t))
	}
	if err := e.c.Delete(context.Background(), e.get(t)); err != nil {
		t.Fatal(err)
	}
	e.reconcile(t)
	if got := e.storedTokens(t); len(got) != 0 {
		t.Fatalf("the token must be removed with the registration, got %v", got)
	}
}

func TestHostCredentialIsShortLivedAndNotTheAdminKey(t *testing.T) {
	e := newTbEnv(t, instanceObject(), apiKeySecret(), testbenchObject(publicSpec()))
	e.reconcile(t)

	var secret corev1.Secret
	if err := e.c.Get(context.Background(), types.NamespacedName{Namespace: tbNS, Name: tbName + "-agent"}, &secret); err != nil {
		t.Fatal(err)
	}
	if string(secret.Data[secretKeyUsername]) != agentUsername {
		t.Fatalf("username %q", secret.Data[secretKeyUsername])
	}
	if string(secret.Data[secretKeyPassword]) == "key" || string(secret.Data[secretKeyPassword]) != "00000001-agent-key" {
		t.Fatalf("host password %q must be the agent key, not the admin key", secret.Data[secretKeyPassword])
	}
	if string(secret.Data[hostSecretExpiry]) != "2026-01-01T06:00:00Z" {
		t.Fatalf("expiry %q, want the default 6h TTL", secret.Data[hostSecretExpiry])
	}
	if len(secret.OwnerReferences) != 1 || secret.OwnerReferences[0].UID != types.UID(tbUID) {
		t.Fatalf("owner %v", secret.OwnerReferences)
	}
	if secret.Labels[jenkinsCredentialsTypeLabel] != "" {
		t.Fatalf("the host Secret in %s is not visible to Jenkins and must not carry the credentials label", tbNS)
	}
	jenkinsName := jenkinsCredentialSecretName(tbNS, tbName)
	if jenkinsName == tbName+"-agent" || !strings.Contains(jenkinsName, tbNS) {
		t.Fatalf("jenkins credential name %q must include the testbench namespace", jenkinsName)
	}
	var jenkins corev1.Secret
	if err := e.c.Get(context.Background(), types.NamespacedName{Namespace: "jenkins", Name: jenkinsName}, &jenkins); err != nil {
		t.Fatal(err)
	}
	if string(jenkins.Data[secretKeyUsername]) != agentUsername || string(jenkins.Data[secretKeyPassword]) != string(secret.Data[secretKeyPassword]) {
		t.Fatalf("jenkins credential username %q", jenkins.Data[secretKeyUsername])
	}
	if jenkins.Labels[jenkinsCredentialsTypeLabel] != "usernamePassword" || jenkins.Labels[jenkinsCredentialLabel] != "true" {
		t.Fatalf("jenkins label %v", jenkins.Labels)
	}
	if _, ok := jenkins.Data[hostSecretToken]; ok || string(jenkins.Data[secretKeyPassword]) == "key" {
		t.Fatalf("jenkins credential must be the agent key only, not the token or the admin key")
	}
	if statusString(e.get(t), "agentCredentialSecret") != tbNS+"/"+tbName+"-agent" {
		t.Fatalf("status secret %q", statusString(e.get(t), "agentCredentialSecret"))
	}

	if statusString(e.get(t), "hostKeyId") != "k1" {
		t.Fatalf("hostKeyId %q, want the key recorded on the testbench", statusString(e.get(t), "hostKeyId"))
	}

	e.reconcile(t)
	if e.api.keySeq != 1 {
		t.Fatalf("a current key was recreated: seq=%d", e.api.keySeq)
	}

	if err := e.c.Delete(context.Background(), e.get(t)); err != nil {
		t.Fatal(err)
	}
	e.reconcile(t)
	if len(e.api.revoked) != 1 || e.api.revoked[0] != "k1" {
		t.Fatalf("delete must revoke the current key, revoked=%v", e.api.revoked)
	}
	if err := e.c.Get(context.Background(), types.NamespacedName{Namespace: "jenkins", Name: jenkinsCredentialSecretName(tbNS, tbName)}, &corev1.Secret{}); err == nil {
		t.Fatal("the jenkins credential must be removed with the testbench")
	}
}

func TestHostKeyRotationKeepsThePreviousKeyUntilTheOverlapEnds(t *testing.T) {
	e := newTbEnv(t, instanceObject(), apiKeySecret(), testbenchObject(publicSpec()))
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	now := start
	e.r.Now = func() time.Time { return now }
	e.reconcile(t)

	// Inside the 15 minute renew window of the default 6h TTL.
	now = start.Add(5*time.Hour + 50*time.Minute)
	e.reconcile(t)
	if e.api.keySeq != 2 || len(e.api.revoked) != 0 {
		t.Fatalf("renewal must not revoke the previous key yet: seq=%d revoked=%v", e.api.keySeq, e.api.revoked)
	}
	tb := e.get(t)
	if statusString(tb, "hostKeyId") != "k2" {
		t.Fatalf("hostKeyId %q", statusString(tb, "hostKeyId"))
	}
	retired, _, _ := unstructured.NestedSlice(tb.Object, "status", "retiredHostKeys")
	if len(retired) != 1 || retired[0].(map[string]interface{})["id"] != "k1" {
		t.Fatalf("retiredHostKeys %v", retired)
	}
	var jenkins corev1.Secret
	if err := e.c.Get(context.Background(), types.NamespacedName{Namespace: "jenkins", Name: jenkinsCredentialSecretName(tbNS, tbName)}, &jenkins); err != nil {
		t.Fatal(err)
	}
	if string(jenkins.Data[secretKeyPassword]) != "00000002-agent-key" {
		t.Fatalf("jenkins credential was not updated to the new agent key")
	}

	// The 48h overlap is longer than the key's own life, so it is revoked when it expires.
	now = start.Add(6 * time.Hour)
	e.reconcile(t)
	if len(e.api.revoked) != 1 || e.api.revoked[0] != "k1" {
		t.Fatalf("the previous key must be revoked once the overlap ends, revoked=%v", e.api.revoked)
	}
	if retired, _, _ := unstructured.NestedSlice(e.get(t).Object, "status", "retiredHostKeys"); len(retired) != 0 {
		t.Fatalf("retiredHostKeys %v after revocation", retired)
	}
}

func TestDeleteRevokesRecordedHostKeysWithoutTheSecret(t *testing.T) {
	e := newTbEnv(t, instanceObject(), apiKeySecret(), testbenchObject(publicSpec()))
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	now := start
	e.r.Now = func() time.Time { return now }
	e.reconcile(t)
	now = start.Add(5*time.Hour + 50*time.Minute)
	e.reconcile(t)

	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: tbNS, Name: tbName + "-agent"}}
	if err := e.c.Delete(context.Background(), secret); err != nil {
		t.Fatal(err)
	}
	if err := e.c.Delete(context.Background(), e.get(t)); err != nil {
		t.Fatal(err)
	}
	e.reconcile(t)
	got := map[string]bool{}
	for _, id := range e.api.revoked {
		got[id] = true
	}
	if !got["k1"] || !got["k2"] {
		t.Fatalf("delete must revoke the current and the retired key from the record, revoked=%v", e.api.revoked)
	}
}

func TestCredentialTTLIsCapped(t *testing.T) {
	obj := testbenchObject(publicSpec())
	_ = unstructured.SetNestedField(obj.Object, "48h", "spec", "credentialTTL")
	if _, err := parseTestbenchSpec(obj); err == nil {
		t.Fatal("a credentialTTL above 24h must be rejected")
	}
}
