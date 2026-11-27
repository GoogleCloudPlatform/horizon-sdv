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
	"fmt"
	"reflect"
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

const devAgentID = "agent-1"

// fakeDeviceMTK is an in-memory MTK Connect for one agent. Like the real server it numbers new devices itself,
// in creation order.
type fakeDeviceMTK struct {
	registration string
	devices      map[int]string
	configured   map[int]map[string]interface{}
	creates      int
	configures   int
	deleted      []int
	skew         int // added to the index the server reports for a new device
	configureErr error
}

func newFakeDeviceMTK() *fakeDeviceMTK {
	return &fakeDeviceMTK{registration: tbRegistration(), devices: map[int]string{}, configured: map[int]map[string]interface{}{}}
}

func (f *fakeDeviceMTK) FindDevice(_ context.Context, registration string, index int) (*mtk.Device, error) {
	name, ok := f.devices[index]
	if registration != f.registration || !ok {
		return nil, nil
	}
	return &mtk.Device{ID: fmt.Sprintf("dev-%d", index), Index: index, Name: name}, nil
}

func (f *fakeDeviceMTK) CreateDevice(_ context.Context, _, name string) (mtk.Device, error) {
	f.creates++
	index := len(f.devices) + 1
	f.devices[index] = name
	return mtk.Device{ID: fmt.Sprintf("dev-%d", index), Index: index + f.skew, Name: name}, nil
}

func (f *fakeDeviceMTK) ConfigureDevice(_ context.Context, _ string, index int, payload map[string]interface{}) error {
	if f.configureErr != nil {
		return f.configureErr
	}
	if _, ok := f.devices[index]; !ok {
		return fmt.Errorf("no device %d", index)
	}
	f.configures++
	f.configured[index] = payload
	return nil
}

func (f *fakeDeviceMTK) DeleteDevice(_ context.Context, _ string, index int) error {
	f.deleted = append(f.deleted, index)
	delete(f.devices, index)
	return nil
}

type devEnv struct {
	r   *DeviceReconciler
	c   client.Client
	api *fakeDeviceMTK
}

// readyTestbench is a testbench that has its agent.
func readyTestbench() *unstructured.Unstructured {
	tb := testbenchObject(publicSpec())
	_ = unstructured.SetNestedField(tb.Object, devAgentID, "status", "agentId")
	_ = unstructured.SetNestedField(tb.Object, tbRegistration(), "status", "registration")
	_ = unstructured.SetNestedField(tb.Object, TbReady, "status", "phase")
	return tb
}

func deviceObject(name string, index int64, spec map[string]interface{}) *unstructured.Unstructured {
	full := map[string]interface{}{"testbenchRef": tbName, "index": index}
	for k, v := range spec {
		full[k] = v
	}
	u := &unstructured.Unstructured{Object: map[string]interface{}{"spec": full}}
	u.SetGroupVersionKind(DeviceGVK)
	u.SetNamespace(tbNS)
	u.SetName(name)
	return u
}

func hostOnlySpec() map[string]interface{} {
	return map[string]interface{}{"profile": "HostOnly"}
}

func adbSpec() map[string]interface{} {
	return map[string]interface{}{
		"profile": "Adb",
		"adb":     map[string]interface{}{"host": "10.0.0.5", "port": int64(5555)},
		"tunnels": []interface{}{map[string]interface{}{"name": "vnc", "port": int64(5900)}},
	}
}

func newDevEnv(t *testing.T, objs ...client.Object) *devEnv {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	devStatus := &unstructured.Unstructured{}
	devStatus.SetGroupVersionKind(DeviceGVK)
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).WithStatusSubresource(devStatus).Build()
	api := newFakeDeviceMTK()
	return &devEnv{
		c:   c,
		api: api,
		r: &DeviceReconciler{
			Client:           c,
			Direct:           c,
			ManagerNamespace: tbManagerNS,
			NewAPI:           func(_, _, _ string) DeviceAPI { return api },
			Now:              func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) },
		},
	}
}

func (e *devEnv) reconcile(t *testing.T, name string) ctrl.Result {
	t.Helper()
	res, err := e.r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: tbNS, Name: name}})
	if err != nil {
		t.Fatalf("Reconcile %s: %v", name, err)
	}
	return res
}

func (e *devEnv) get(t *testing.T, name string) *unstructured.Unstructured {
	t.Helper()
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(DeviceGVK)
	if err := e.c.Get(context.Background(), types.NamespacedName{Namespace: tbNS, Name: name}, u); err != nil {
		t.Fatalf("get device %s: %v", name, err)
	}
	return u
}

func (e *devEnv) exists(t *testing.T, name string) bool {
	t.Helper()
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(DeviceGVK)
	err := e.c.Get(context.Background(), types.NamespacedName{Namespace: tbNS, Name: name}, u)
	if err != nil && !apierrors.IsNotFound(err) {
		t.Fatal(err)
	}
	return err == nil
}

func (e *devEnv) events(t *testing.T) []string {
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

func (e *devEnv) delete(t *testing.T, name string) {
	t.Helper()
	if err := e.c.Delete(context.Background(), e.get(t, name)); err != nil {
		t.Fatal(err)
	}
}

func baseObjects(extra ...client.Object) []client.Object {
	return append([]client.Object{instanceObject(), apiKeySecret(), readyTestbench()}, extra...)
}

func TestDeviceIsCreatedAndConfigured(t *testing.T) {
	spec := map[string]interface{}{
		"profile": "HostOnly",
		"host":    map[string]interface{}{"launchApplication": "gedit"},
	}
	e := newDevEnv(t, baseObjects(deviceObject("dev-a", 1, spec))...)

	res := e.reconcile(t, "dev-a")
	got := e.get(t, "dev-a")

	if e.api.creates != 1 || e.api.configures != 1 {
		t.Fatalf("creates=%d configures=%d, want 1 and 1", e.api.creates, e.api.configures)
	}
	if name := e.api.devices[1]; name != tbName+" 1" {
		t.Errorf("device name %q, want %q", name, tbName+" 1")
	}
	parsed, err := parseDeviceSpec(got)
	if err != nil {
		t.Fatal(err)
	}
	want, err := parsed.profileDevice().Payload()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(e.api.configured[1], want) {
		t.Errorf("payload sent to MTK Connect differs from the profile payload:\n got %v\nwant %v", e.api.configured[1], want)
	}
	raw, _ := json.Marshal(e.api.configured[1])
	for _, frag := range []string{"su builder", "gedit"} {
		if !strings.Contains(string(raw), frag) {
			t.Errorf("payload does not contain %q: %s", frag, raw)
		}
	}
	if st := statusString(got, "phase"); st != DevConfigured {
		t.Errorf("phase %q, want %q", st, DevConfigured)
	}
	if statusString(got, "agentId") != devAgentID || statusString(got, "deviceId") != "dev-1" {
		t.Errorf("status ids: %v", got.Object["status"])
	}
	if idx, _, _ := unstructured.NestedInt64(got.Object, "status", "index"); idx != 1 {
		t.Errorf("status.index %d, want 1", idx)
	}
	if s, _ := condStatus(got, DevCondConfigured); s != "True" {
		t.Errorf("DeviceConfigured %q, want True", s)
	}
	if len(got.GetFinalizers()) != 1 || got.GetFinalizers()[0] != FinalizerDeviceCleanup {
		t.Errorf("finalizers %v", got.GetFinalizers())
	}
	if refs := got.GetOwnerReferences(); len(refs) != 1 || refs[0].UID != types.UID(tbUID) || refs[0].Kind != KindTestbench {
		t.Errorf("owner references %v, want the testbench", refs)
	}
	if res.RequeueAfter != deviceResyncDelay {
		t.Errorf("requeue %v, want %v", res.RequeueAfter, deviceResyncDelay)
	}
}

func TestAdbDeviceWithTunnelsIsConfigured(t *testing.T) {
	e := newDevEnv(t, baseObjects(deviceObject("dev-a", 1, adbSpec()))...)
	e.reconcile(t, "dev-a")

	raw, _ := json.Marshal(e.api.configured[1])
	for _, frag := range []string{"10.0.0.5", "5555", "vnc", "5900"} {
		if !strings.Contains(string(raw), frag) {
			t.Errorf("adb payload does not contain %q: %s", frag, raw)
		}
	}
}

func TestDeviceReconcileIsIdempotent(t *testing.T) {
	e := newDevEnv(t, baseObjects(deviceObject("dev-a", 1, hostOnlySpec()))...)
	e.reconcile(t, "dev-a")
	e.reconcile(t, "dev-a")
	e.reconcile(t, "dev-a")

	if e.api.creates != 1 {
		t.Errorf("creates=%d, want 1: a repeated reconcile must not create another device", e.api.creates)
	}
	if e.api.configures != 3 {
		t.Errorf("configures=%d, want 3: the interface is re-applied each time", e.api.configures)
	}
}

func TestDeviceSpecChangeIsApplied(t *testing.T) {
	e := newDevEnv(t, baseObjects(deviceObject("dev-a", 1, hostOnlySpec()))...)
	e.reconcile(t, "dev-a")
	before, _ := json.Marshal(e.api.configured[1])

	dev := e.get(t, "dev-a")
	_ = unstructured.SetNestedField(dev.Object, "bob", "spec", "host", "terminalUser")
	if err := e.c.Update(context.Background(), dev); err != nil {
		t.Fatal(err)
	}
	e.reconcile(t, "dev-a")

	after, _ := json.Marshal(e.api.configured[1])
	if string(before) == string(after) || !strings.Contains(string(after), "su bob") {
		t.Errorf("the changed terminal user did not reach MTK Connect:\n before %s\n after  %s", before, after)
	}
	if e.api.creates != 1 {
		t.Errorf("creates=%d, want 1", e.api.creates)
	}
}

func TestDeviceWaitsForTestbench(t *testing.T) {
	cases := map[string][]client.Object{
		"missing":  {instanceObject(), apiKeySecret(), deviceObject("dev-a", 1, hostOnlySpec())},
		"no agent": {instanceObject(), apiKeySecret(), testbenchObject(publicSpec()), deviceObject("dev-a", 1, hostOnlySpec())},
	}
	for name, objs := range cases {
		t.Run(name, func(t *testing.T) {
			e := newDevEnv(t, objs...)
			res := e.reconcile(t, "dev-a")
			got := e.get(t, "dev-a")

			if e.api.creates != 0 || e.api.configures != 0 {
				t.Errorf("MTK Connect was called before the testbench had an agent")
			}
			if s, _ := condStatus(got, DevCondTestbench); s != "False" {
				t.Errorf("TestbenchReady %q, want False", s)
			}
			if st := statusString(got, "phase"); st != DevPending {
				t.Errorf("phase %q, want Pending", st)
			}
			if res.RequeueAfter <= 0 || res.RequeueAfter >= 30*time.Second {
				t.Errorf("retry after %v, want under 30s so a change shows within 30 seconds", res.RequeueAfter)
			}
			if len(e.events(t)) != 0 {
				t.Errorf("waiting must not raise Events, got %v", e.events(t))
			}
		})
	}
}

func TestDeviceWaitsForInstance(t *testing.T) {
	e := newDevEnv(t, readyTestbench(), deviceObject("dev-a", 1, hostOnlySpec()))
	res := e.reconcile(t, "dev-a")

	if s, reason := condStatus(e.get(t, "dev-a"), DevCondInstanceReady); s != "False" || reason != reasonInstanceNotReady {
		t.Errorf("InstanceReady %q/%q, want False/%s", s, reason, reasonInstanceNotReady)
	}
	if res.RequeueAfter <= 0 || res.RequeueAfter >= 30*time.Second {
		t.Errorf("retry after %v, want under 30s", res.RequeueAfter)
	}
}

func TestDeviceWaitsForLowerIndex(t *testing.T) {
	e := newDevEnv(t, baseObjects(deviceObject("dev-b", 2, hostOnlySpec()))...)
	res := e.reconcile(t, "dev-b")

	if e.api.creates != 0 {
		t.Fatalf("device 2 was created before device 1 existed")
	}
	if _, reason := condStatus(e.get(t, "dev-b"), DevCondConfigured); reason != reasonWaitingPrior {
		t.Errorf("reason %q, want %s", reason, reasonWaitingPrior)
	}
	if res.RequeueAfter <= 0 || res.RequeueAfter >= 30*time.Second {
		t.Errorf("retry after %v, want under 30s", res.RequeueAfter)
	}

	if err := e.c.Create(context.Background(), deviceObject("dev-a", 1, hostOnlySpec())); err != nil {
		t.Fatal(err)
	}
	e.reconcile(t, "dev-a")
	e.reconcile(t, "dev-b")

	if e.api.creates != 2 || e.api.devices[2] != tbName+" 2" {
		t.Errorf("creates=%d devices=%v, want both devices in order", e.api.creates, e.api.devices)
	}
	if st := statusString(e.get(t, "dev-b"), "phase"); st != DevConfigured {
		t.Errorf("phase %q, want Configured", st)
	}
}

func TestDeviceIndexMismatchFails(t *testing.T) {
	e := newDevEnv(t, baseObjects(deviceObject("dev-a", 1, hostOnlySpec()))...)
	e.api.skew = 1
	e.reconcile(t, "dev-a")
	got := e.get(t, "dev-a")

	if st := statusString(got, "phase"); st != DevFailed {
		t.Errorf("phase %q, want Failed", st)
	}
	if _, reason := condStatus(got, DevCondConfigured); reason != "IndexMismatch" {
		t.Errorf("reason %q, want IndexMismatch", reason)
	}
	if e.api.configures != 0 {
		t.Errorf("a misnumbered device must not be configured")
	}
	if len(e.api.deleted) != 1 || e.api.deleted[0] != 2 {
		t.Errorf("deleted %v, want the stray device 2 removed", e.api.deleted)
	}

	e.reconcile(t, "dev-a")
	if e.api.creates != 1 {
		t.Errorf("creates=%d, want 1: a mismatch must not create another device", e.api.creates)
	}
}

// The device is recorded in status before it is created, so deleting a resource whose create did not end
// cleanly still removes the device from MTK Connect instead of dropping the finalizer and leaving it behind.
func TestDeviceIsRecordedBeforeCreateSoDeleteCleansUp(t *testing.T) {
	e := newDevEnv(t, baseObjects(deviceObject("dev-a", 1, hostOnlySpec()))...)
	e.api.skew = 1 // the create ends in an error path with no configured device
	e.reconcile(t, "dev-a")
	got := e.get(t, "dev-a")
	if statusString(got, "agentId") != devAgentID {
		t.Fatalf("agentId %q was not recorded before the create", statusString(got, "agentId"))
	}
	if idx, _, _ := unstructured.NestedInt64(got.Object, "status", "index"); idx != 1 {
		t.Fatalf("index %d was not recorded before the create", idx)
	}

	e.delete(t, "dev-a")
	e.reconcile(t, "dev-a")
	if len(e.api.deleted) == 0 || e.api.deleted[len(e.api.deleted)-1] != 1 {
		t.Errorf("deleted %v, want device 1 removed on delete", e.api.deleted)
	}
	if e.exists(t, "dev-a") {
		t.Errorf("the finalizer should be gone after the cleanup")
	}
}

func TestDuplicateIndexFailsTheNewerDevice(t *testing.T) {
	first := deviceObject("dev-a", 1, hostOnlySpec())
	first.SetCreationTimestamp(metav1.NewTime(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)))
	second := deviceObject("dev-b", 1, hostOnlySpec())
	second.SetCreationTimestamp(metav1.NewTime(time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)))
	e := newDevEnv(t, baseObjects(first, second)...)

	e.reconcile(t, "dev-b")
	if st := statusString(e.get(t, "dev-b"), "phase"); st != DevFailed {
		t.Errorf("newer device phase %q, want Failed", st)
	}
	if _, reason := condStatus(e.get(t, "dev-b"), DevCondConfigured); reason != "DuplicateIndex" {
		t.Errorf("reason %q, want DuplicateIndex", reason)
	}
	if e.api.creates != 0 {
		t.Errorf("the newer device must not touch MTK Connect")
	}

	e.reconcile(t, "dev-a")
	if st := statusString(e.get(t, "dev-a"), "phase"); st != DevConfigured {
		t.Errorf("older device phase %q, want Configured", st)
	}
}

func TestInvalidDeviceSpecFails(t *testing.T) {
	bad := map[string]interface{}{"profile": "Adb"} // no adb connection
	e := newDevEnv(t, baseObjects(deviceObject("dev-a", 1, bad))...)
	e.reconcile(t, "dev-a")
	got := e.get(t, "dev-a")

	if st := statusString(got, "phase"); st != DevFailed {
		t.Errorf("phase %q, want Failed", st)
	}
	if s, _ := condStatus(got, DevCondSpecValid); s != "False" {
		t.Errorf("SpecValid %q, want False", s)
	}
	if e.api.creates != 0 || len(got.GetFinalizers()) != 0 {
		t.Errorf("an invalid device must not create anything")
	}
}

func TestConfigureFailureIsRetried(t *testing.T) {
	e := newDevEnv(t, baseObjects(deviceObject("dev-a", 1, hostOnlySpec()))...)
	e.api.configureErr = fmt.Errorf("boom")
	res := e.reconcile(t, "dev-a")

	if s, reason := condStatus(e.get(t, "dev-a"), DevCondConfigured); s != "False" || reason != "ConfigureFailed" {
		t.Errorf("DeviceConfigured %q/%q", s, reason)
	}
	if res.RequeueAfter <= 0 || res.RequeueAfter >= 30*time.Second {
		t.Errorf("retry after %v, want under 30s", res.RequeueAfter)
	}

	e.api.configureErr = nil
	e.reconcile(t, "dev-a")
	if e.api.creates != 1 || e.api.configures != 1 {
		t.Errorf("creates=%d configures=%d, want the existing device configured without a second create", e.api.creates, e.api.configures)
	}
}

func TestDeletingDeviceRemovesItFromMTK(t *testing.T) {
	e := newDevEnv(t, baseObjects(deviceObject("dev-a", 1, hostOnlySpec()))...)
	e.reconcile(t, "dev-a")

	e.delete(t, "dev-a")
	e.reconcile(t, "dev-a")

	if len(e.api.deleted) != 1 || e.api.deleted[0] != 1 {
		t.Errorf("deleted %v, want device 1", e.api.deleted)
	}
	if e.exists(t, "dev-a") {
		t.Errorf("the finalizer should be gone after the device was deleted in MTK Connect")
	}
}

func TestDeletingDeviceWithoutTestbenchSkipsTheAPI(t *testing.T) {
	e := newDevEnv(t, baseObjects(deviceObject("dev-a", 1, hostOnlySpec()))...)
	e.reconcile(t, "dev-a")

	if err := e.c.Delete(context.Background(), readyTestbench()); err != nil {
		t.Fatal(err)
	}
	e.delete(t, "dev-a")
	e.reconcile(t, "dev-a")

	if len(e.api.deleted) != 0 {
		t.Errorf("deleted %v: with the testbench gone the agent took its devices with it", e.api.deleted)
	}
	if e.exists(t, "dev-a") {
		t.Errorf("the device should be released once its testbench is gone")
	}
}

func TestDeletionBlockedWithoutInstance(t *testing.T) {
	dev := deviceObject("dev-a", 1, hostOnlySpec())
	e := newDevEnv(t, baseObjects(dev)...)
	e.reconcile(t, "dev-a")

	if err := e.c.Delete(context.Background(), instanceObject()); err != nil {
		t.Fatal(err)
	}
	e.delete(t, "dev-a")
	res := e.reconcile(t, "dev-a")

	if !e.exists(t, "dev-a") || res.RequeueAfter <= 0 {
		t.Fatalf("the device must be kept while MTK Connect cannot be reached")
	}
	if ev := e.events(t); !contains(ev, "CleanupBlocked") {
		t.Errorf("events %v, want CleanupBlocked", ev)
	}

	got := e.get(t, "dev-a")
	got.SetAnnotations(map[string]string{AnnotationSkipCleanup: "true"})
	if err := e.c.Update(context.Background(), got); err != nil {
		t.Fatal(err)
	}
	e.reconcile(t, "dev-a")
	if e.exists(t, "dev-a") {
		t.Errorf("the skip annotation should release the device")
	}
}

func TestParseDeviceSpec(t *testing.T) {
	spec, err := parseDeviceSpec(deviceObject("d", 3, adbSpec()))
	if err != nil {
		t.Fatal(err)
	}
	if spec.Name != tbName+" 3" || spec.Index != 3 || spec.Adb == nil || spec.Adb.Port != 5555 || len(spec.Tunnels) != 1 {
		t.Errorf("unexpected spec %+v", spec)
	}
	for name, s := range map[string]map[string]interface{}{
		"index too high": {"profile": "HostOnly", "index": int64(65)},
		"no testbench":   {"profile": "HostOnly", "testbenchRef": ""},
		"bad profile":    {"profile": "Other"},
	} {
		obj := deviceObject("d", 1, s)
		if _, err := parseDeviceSpec(obj); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
