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
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
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
	saNamespace = "mtk"
	saStoredKey = "stored-key-0000"
	saNewKey    = "new-service-key-1111"
)

// fakeInstanceMTK answers FindUserID with a scripted error (nil for success).
type fakeInstanceMTK struct {
	findErr error
}

func (f *fakeInstanceMTK) Version(context.Context) (string, error) { return "1.2.3", nil }
func (f *fakeInstanceMTK) FindUserID(context.Context, string) (string, error) {
	if f.findErr != nil {
		return "", f.findErr
	}
	return "user-1", nil
}
func (f *fakeInstanceMTK) Keys(context.Context, string) ([]mtk.Key, error) { return nil, nil }
func (f *fakeInstanceMTK) CreateKey(context.Context, string, time.Time) (string, string, error) {
	return "", "", errors.New("not used")
}
func (f *fakeInstanceMTK) DeleteKey(context.Context, string, string) error { return nil }

type fakeExec struct {
	calls int
	out   string
}

func (f *fakeExec) Run(context.Context, string, string, string, []string) (string, error) {
	f.calls++
	return f.out, nil
}

type saEnv struct {
	r    *InstanceReconciler
	exec *fakeExec
	mtk  *fakeInstanceMTK // used for the stored key; the new key always verifies
	inst *unstructured.Unstructured
	spec *InstanceSpec
	c    client.Client
}

func newSAEnv(t *testing.T, storedKey string) *saEnv {
	t.Helper()
	inst := &unstructured.Unstructured{Object: map[string]interface{}{"spec": map[string]interface{}{
		"domain": "horizon.example.com",
		"keycloak": map[string]interface{}{
			"url":            "http://kc.kc.svc.cluster.local:8080/auth",
			"adminSecretRef": map[string]interface{}{"namespace": "kc", "name": "creds"},
		},
		"server": map[string]interface{}{"url": "http://router.mtk.svc.cluster.local:8081"},
	}}}
	inst.SetGroupVersionKind(InstanceGVK)
	inst.SetNamespace(saNamespace)
	inst.SetName("mtk-connect")
	spec, _, err := parseSpec(inst)
	if err != nil {
		t.Fatal(err)
	}
	spec.Server.PodSelector = "app=mtk-server"

	objs := []client.Object{&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "server-0", Namespace: saNamespace, Labels: map[string]string{"app": "mtk-server"}},
		Status: corev1.PodStatus{
			Phase:      corev1.PodRunning,
			Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}},
		},
	}}
	if storedKey != "" {
		objs = append(objs, &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: spec.Server.APIKeySecret, Namespace: saNamespace},
			Data:       map[string][]byte{secretKeyPassword: []byte(storedKey)},
		})
	}
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()

	stored := &fakeInstanceMTK{}
	ex := &fakeExec{out: "creating service account\n" + saNewKey + "\n"}
	return &saEnv{
		r: &InstanceReconciler{
			Client: c, Direct: c, Exec: ex,
			NewMTK: func(_, _, key string) MTKAPI {
				if key == saNewKey {
					return &fakeInstanceMTK{}
				}
				return stored
			},
			Now: func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) },
		},
		exec: ex, mtk: stored, inst: inst, spec: spec, c: c,
	}
}

func (e *saEnv) sync(t *testing.T) (*serviceAccount, error) {
	t.Helper()
	return e.r.syncServiceAccount(context.Background(), e.inst, saNamespace, e.spec, &instanceStatus{}, e.r.now())
}

func (e *saEnv) storedKey(t *testing.T) string {
	t.Helper()
	v, _, err := readSecretValue(context.Background(), e.c, saNamespace, e.spec.Server.APIKeySecret, secretKeyPassword)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestSyncServiceAccountKeepsAWorkingKey(t *testing.T) {
	e := newSAEnv(t, saStoredKey)
	sa, err := e.sync(t)
	if err != nil {
		t.Fatal(err)
	}
	if sa.password != saStoredKey || e.exec.calls != 0 {
		t.Errorf("a working key must be kept: password=%q execCalls=%d", sa.password, e.exec.calls)
	}
}

func TestSyncServiceAccountBootstrapsWithoutAKey(t *testing.T) {
	e := newSAEnv(t, "")
	sa, err := e.sync(t)
	if err != nil {
		t.Fatal(err)
	}
	if sa.password != saNewKey || e.exec.calls != 1 || e.storedKey(t) != saNewKey {
		t.Errorf("want a bootstrapped and stored key: password=%q execCalls=%d stored=%q", sa.password, e.exec.calls, e.storedKey(t))
	}
}

// The server answered and accepted the key but has no such user: create it instead of waiting for ever.
func TestSyncServiceAccountCreatesAMissingUser(t *testing.T) {
	e := newSAEnv(t, saStoredKey)
	e.mtk.findErr = fmt.Errorf("%w: %q", mtk.ErrUserNotFound, "mtk-connect-admin")
	sa, err := e.sync(t)
	if err != nil {
		t.Fatal(err)
	}
	if sa.password != saNewKey || e.exec.calls != 1 || e.storedKey(t) != saNewKey {
		t.Errorf("want the service account created: password=%q execCalls=%d stored=%q", sa.password, e.exec.calls, e.storedKey(t))
	}
}

// A server that is down says nothing about the key: never replace it, and report the server as unavailable.
func TestSyncServiceAccountKeepsTheKeyWhileTheServerIsDown(t *testing.T) {
	for name, findErr := range map[string]error{
		"5xx":       &mtk.APIError{Status: 503},
		"transport": &url.Error{Op: "Get", URL: "http://router", Err: errors.New("connection refused")},
	} {
		t.Run(name, func(t *testing.T) {
			e := newSAEnv(t, saStoredKey)
			e.mtk.findErr = findErr
			if _, err := e.sync(t); !errors.Is(err, errServerUnavailable) {
				t.Fatalf("got %v, want errServerUnavailable", err)
			}
			if e.exec.calls != 0 || e.storedKey(t) != saStoredKey {
				t.Errorf("the key must not be replaced: execCalls=%d stored=%q", e.exec.calls, e.storedKey(t))
			}
		})
	}
}

// An answer the client cannot read is neither unavailability nor a rejected key.
func TestSyncServiceAccountKeepsTheKeyOnAnUnreadableAnswer(t *testing.T) {
	e := newSAEnv(t, saStoredKey)
	e.mtk.findErr = errors.New("mtk-connect GET /users: decode response: unexpected end of JSON input")
	_, err := e.sync(t)
	if err == nil || errors.Is(err, errServerUnavailable) {
		t.Fatalf("got %v, want a plain verification error", err)
	}
	if e.exec.calls != 0 || e.storedKey(t) != saStoredKey {
		t.Errorf("the key must not be replaced: execCalls=%d stored=%q", e.exec.calls, e.storedKey(t))
	}
}

// One 401 may be transient: only repeated rejections create a new key.
func TestSyncServiceAccountNeedsRepeatedRejections(t *testing.T) {
	e := newSAEnv(t, saStoredKey)
	e.mtk.findErr = &mtk.APIError{Status: 401}
	if _, err := e.sync(t); err == nil {
		t.Fatal("the first rejection must not succeed")
	}
	if e.exec.calls != 0 || e.storedKey(t) != saStoredKey {
		t.Fatalf("the first rejection must keep the key: execCalls=%d stored=%q", e.exec.calls, e.storedKey(t))
	}
	sa, err := e.sync(t)
	if err != nil {
		t.Fatal(err)
	}
	if sa.password != saNewKey || e.exec.calls != 1 || e.storedKey(t) != saNewKey {
		t.Errorf("the second rejection must bootstrap: password=%q execCalls=%d stored=%q", sa.password, e.exec.calls, e.storedKey(t))
	}
}

// Only the instance the other controllers use is reconciled; a second MTKConnectInstance is ignored.
func TestReconcileIgnoresOtherInstances(t *testing.T) {
	r := &InstanceReconciler{} // nil clients: touching anything would panic
	res, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: saNamespace, Name: "second"}})
	if err != nil || res.RequeueAfter != 0 {
		t.Fatalf("got %+v, %v; want an empty result", res, err)
	}
}

// Only a single key-shaped token is accepted as the output of createServiceAccount.js.
func TestAPIKeyPattern(t *testing.T) {
	for key, want := range map[string]bool{
		"0b6f2c9e-1111-4222-8333-444455556666": true,
		"abcDEF12.ghi_jk-9":                    true,
		"short":                                false,
		"Error: cannot find module":            false,
		"ReferenceError:":                      false,
		"at Object.<anonymous> (index.js:1:1)": false,
		"":                                     false,
	} {
		if got := apiKeyPattern.MatchString(key); got != want {
			t.Errorf("apiKeyPattern(%q) = %v, want %v", key, got, want)
		}
	}
}
