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

package controller

import (
	"context"
	"reflect"
	"testing"
	"time"

	iampb "cloud.google.com/go/iam/apiv1/iampb"
	pb "cloud.google.com/go/workstations/apiv1/workstationspb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/proto"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	wsv1alpha1 "cloud-workstations-module/api/v1alpha1"
	"cloud-workstations-module/internal/wsapi"
)

const configExternalRef = "projects/p/locations/l/workstationClusters/c/workstationConfigs/test-config"

type recordingAPI struct {
	calls    []string
	cfg      *pb.WorkstationConfig
	getErr   error
	patchErr error
	policies map[string]*iampb.Policy
	last     wsapi.Posture
}

func (f *recordingAPI) GetConfig(context.Context, string) (*pb.WorkstationConfig, error) {
	f.calls = append(f.calls, "GetConfig")
	if f.getErr != nil {
		return nil, f.getErr
	}
	if f.cfg == nil {
		return &pb.WorkstationConfig{}, nil
	}
	return proto.Clone(f.cfg).(*pb.WorkstationConfig), nil
}

func (f *recordingAPI) PatchConfigPosture(_ context.Context, name, etag string, posture wsapi.Posture) error {
	f.calls = append(f.calls, "PatchConfigPosture")
	f.last = posture
	if f.patchErr != nil {
		return f.patchErr
	}
	req, err := wsapi.BuildPostureUpdate(name, etag, posture)
	if err != nil {
		return err
	}
	updated := req.GetWorkstationConfig()
	if f.cfg == nil {
		f.cfg = updated
		return nil
	}
	f.cfg.Host = updated.GetHost()
	f.cfg.AllowedPorts = updated.GetAllowedPorts()
	f.cfg.EphemeralDirectories = updated.GetEphemeralDirectories()
	f.cfg.Reconciling = false
	return nil
}

func (f *recordingAPI) GetIamPolicy(_ context.Context, resource string) (*iampb.Policy, error) {
	f.calls = append(f.calls, "GetIamPolicy")
	if f.policies == nil {
		f.policies = map[string]*iampb.Policy{}
	}
	policy := f.policies[resource]
	if policy == nil {
		policy = &iampb.Policy{Version: 3, Etag: []byte("etag")}
	}
	return proto.Clone(policy).(*iampb.Policy), nil
}

func (f *recordingAPI) SetIamPolicy(_ context.Context, resource string, policy *iampb.Policy) (*iampb.Policy, error) {
	f.calls = append(f.calls, "SetIamPolicy")
	if f.policies == nil {
		f.policies = map[string]*iampb.Policy{}
	}
	f.policies[resource] = proto.Clone(policy).(*iampb.Policy)
	return f.policies[resource], nil
}

func (f *recordingAPI) count(name string) int {
	n := 0
	for _, call := range f.calls {
		if call == name {
			n++
		}
	}
	return n
}

func TestAPIInterfaceHasNoStartOrStop(t *testing.T) {
	apiType := reflect.TypeOf((*wsapi.API)(nil)).Elem()
	for i := 0; i < apiType.NumMethod(); i++ {
		name := apiType.Method(i).Name
		if name == "StartWorkstation" || name == "StopWorkstation" {
			t.Fatalf("API exposes %s", name)
		}
	}
}

func TestWorkstationSpecHasNoPowerState(t *testing.T) {
	spec := reflect.TypeOf(wsv1alpha1.WorkstationSpec{})
	if _, ok := spec.FieldByName("PowerState"); ok {
		t.Fatal("WorkstationSpec still has PowerState")
	}
}

func readyConfigObject(externalRef string, ready bool) *unstructured.Unstructured {
	status := "False"
	if ready {
		status = "True"
	}
	obj := &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "workstations.cnrm.cloud.google.com/v1beta1",
		"kind":       "WorkstationConfig",
		"metadata": map[string]interface{}{
			"name":      "test-config",
			"namespace": "cloud-workstations",
		},
		"status": map[string]interface{}{
			"externalRef": externalRef,
			"conditions": []interface{}{
				map[string]interface{}{
					"type":    "Ready",
					"status":  status,
					"reason":  "UpToDate",
					"message": "Config is ready",
				},
			},
		},
	}}
	return obj
}

func configReconciler(t *testing.T, cfg *wsv1alpha1.WorkstationConfig, kccObj *unstructured.Unstructured, api wsapi.API) *WorkstationConfigReconciler {
	t.Helper()
	scheme := setupConfigTestScheme()
	cluster := &wsv1alpha1.WorkstationCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "parent-cluster", Namespace: "cloud-workstations"},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(cluster, cfg, kccObj).WithStatusSubresource(cfg).Build()
	return &WorkstationConfigReconciler{
		Client:             c,
		Scheme:             scheme,
		WS:                 api,
		ResyncInterval:     time.Minute,
		HostServiceAccount: testHostServiceAccount,
	}
}

func reconcileConfig(t *testing.T, r *WorkstationConfigReconciler) (ctrl.Result, *wsv1alpha1.WorkstationConfig) {
	t.Helper()
	res, err := r.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{Namespace: "cloud-workstations", Name: "test-config"},
	})
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	updated := &wsv1alpha1.WorkstationConfig{}
	if err := r.Get(context.Background(), types.NamespacedName{Namespace: "cloud-workstations", Name: "test-config"}, updated); err != nil {
		t.Fatal(err)
	}
	return res, updated
}

func openAPIConfig() *pb.WorkstationConfig {
	return &pb.WorkstationConfig{
		Name: configExternalRef,
		Etag: "etag-1",
		Host: &pb.WorkstationConfig_Host{
			Config: &pb.WorkstationConfig_Host_GceInstance_{
				GceInstance: &pb.WorkstationConfig_Host_GceInstance{DisableSsh: false},
			},
		},
		AllowedPorts: []*pb.WorkstationConfig_PortRange{{First: 22, Last: 22}},
	}
}

func TestWorkstationConfigReconcile_NoAPIBeforeKCCReady(t *testing.T) {
	api := &recordingAPI{cfg: openAPIConfig()}
	cfg := &wsv1alpha1.WorkstationConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "test-config", Namespace: "cloud-workstations", Finalizers: []string{ConfigFinalizer}},
		Spec:       wsv1alpha1.WorkstationConfigSpec{ClusterRef: wsv1alpha1.ClusterRef{Name: "parent-cluster"}},
	}
	r := configReconciler(t, cfg, readyConfigObject(configExternalRef, false), api)
	if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "cloud-workstations", Name: "test-config"}}); err != nil {
		t.Fatal(err)
	}
	if len(api.calls) != 0 {
		t.Fatalf("API calls before KCCReady: %v", api.calls)
	}
}

func TestWorkstationConfigReconcile_DefaultPosturePatchAndReadBack(t *testing.T) {
	api := &recordingAPI{cfg: openAPIConfig()}
	cfg := &wsv1alpha1.WorkstationConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "test-config", Namespace: "cloud-workstations", Finalizers: []string{ConfigFinalizer}},
		Spec:       wsv1alpha1.WorkstationConfigSpec{ClusterRef: wsv1alpha1.ClusterRef{Name: "parent-cluster"}},
	}
	r := configReconciler(t, cfg, readyConfigObject(configExternalRef, true), api)
	res, updated := reconcileConfig(t, r)
	if res.RequeueAfter != apiWaitInterval {
		t.Fatalf("requeue after patch = %s", res.RequeueAfter)
	}
	if api.count("PatchConfigPosture") != 1 {
		t.Fatalf("calls = %v", api.calls)
	}
	if !api.last.DisableSsh {
		t.Fatal("default disableSsh was not true")
	}
	if len(api.last.AllowedPorts) != 2 || api.last.AllowedPorts[0] != (wsv1alpha1.PortRange{First: 80, Last: 80}) || api.last.AllowedPorts[1] != (wsv1alpha1.PortRange{First: 1024, Last: 65535}) {
		t.Fatalf("patched ports = %+v", api.last.AllowedPorts)
	}
	req, err := wsapi.BuildPostureUpdate(configExternalRef, "etag-1", api.last)
	if err != nil {
		t.Fatal(err)
	}
	got := append([]string(nil), req.GetUpdateMask().GetPaths()...)
	want := []string{wsapi.MaskDisableSSH, wsapi.MaskAllowedPorts, wsapi.MaskEphemeralDirectories}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("mask = %v", got)
	}
	if metaConditionTrue(updated.Status.Conditions, ConditionPostureApplied) || metaConditionTrue(updated.Status.Conditions, ConditionReady) {
		t.Fatalf("ready before read-back: %+v", updated.Status.Conditions)
	}

	res, updated = reconcileConfig(t, r)
	if res.RequeueAfter != time.Minute {
		t.Fatalf("resync = %s", res.RequeueAfter)
	}
	if api.count("PatchConfigPosture") != 1 {
		t.Fatalf("read-back patched again: %v", api.calls)
	}
	if !metaConditionTrue(updated.Status.Conditions, ConditionPostureApplied) || !metaConditionTrue(updated.Status.Conditions, ConditionReady) {
		t.Fatalf("expected posture and Ready after read-back, got %+v", updated.Status.Conditions)
	}
	if updated.Status.Phase != "Ready" || updated.Status.Posture == nil || !updated.Status.Posture.DisableSsh {
		t.Fatalf("status = %+v", updated.Status)
	}
}

func TestWorkstationConfigReconcile_ReconcilingWaits(t *testing.T) {
	live := openAPIConfig()
	live.Reconciling = true
	api := &recordingAPI{cfg: live}
	cfg := &wsv1alpha1.WorkstationConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "test-config", Namespace: "cloud-workstations", Finalizers: []string{ConfigFinalizer}},
		Spec:       wsv1alpha1.WorkstationConfigSpec{ClusterRef: wsv1alpha1.ClusterRef{Name: "parent-cluster"}},
	}
	r := configReconciler(t, cfg, readyConfigObject(configExternalRef, true), api)
	res, updated := reconcileConfig(t, r)
	if api.count("PatchConfigPosture") != 0 {
		t.Fatalf("patched while reconciling: %v", api.calls)
	}
	if res.RequeueAfter != apiWaitInterval {
		t.Fatalf("requeue = %s", res.RequeueAfter)
	}
	cond := conditionByType(updated.Status.Conditions, ConditionPostureApplied)
	if cond == nil || cond.Reason != "Reconciling" {
		t.Fatalf("posture condition = %+v", cond)
	}
}

func TestWorkstationConfigReconcile_RepatchesConsoleDrift(t *testing.T) {
	api := &recordingAPI{cfg: openAPIConfig()}
	cfg := &wsv1alpha1.WorkstationConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "test-config", Namespace: "cloud-workstations", Finalizers: []string{ConfigFinalizer}},
		Spec:       wsv1alpha1.WorkstationConfigSpec{ClusterRef: wsv1alpha1.ClusterRef{Name: "parent-cluster"}},
	}
	r := configReconciler(t, cfg, readyConfigObject(configExternalRef, true), api)
	if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "cloud-workstations", Name: "test-config"}}); err != nil {
		t.Fatal(err)
	}
	api.cfg.AllowedPorts = []*pb.WorkstationConfig_PortRange{{First: 22, Last: 22}}
	if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "cloud-workstations", Name: "test-config"}}); err != nil {
		t.Fatal(err)
	}
	if api.count("PatchConfigPosture") != 2 {
		t.Fatalf("drift was not re-patched: %v", api.calls)
	}
}

func TestWorkstationConfigReconcile_AdminGrantsWaitForPosture(t *testing.T) {
	api := &recordingAPI{
		cfg: openAPIConfig(),
		policies: map[string]*iampb.Policy{
			configExternalRef: {
				Version: 3,
				Etag:    []byte("etag"),
				Bindings: []*iampb.Binding{{
					Role:    wsapi.RoleConfigAdmin,
					Members: []string{"user:bob@example.com"},
				}},
			},
		},
	}
	cfg := &wsv1alpha1.WorkstationConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "test-config", Namespace: "cloud-workstations", Finalizers: []string{ConfigFinalizer}},
		Spec: wsv1alpha1.WorkstationConfigSpec{
			ClusterRef: wsv1alpha1.ClusterRef{Name: "parent-cluster"},
			Admins:     []string{"user:alice@example.com"},
		},
	}
	r := configReconciler(t, cfg, readyConfigObject(configExternalRef, true), api)
	_, updated := reconcileConfig(t, r)
	members := bindingMembers(api.policies[configExternalRef], wsapi.RoleConfigAdmin)
	if len(members) != 0 {
		t.Fatalf("grant happened before posture: %v", members)
	}
	if metaConditionTrue(updated.Status.Conditions, ConditionIAMPolicySynced) {
		t.Fatal("IAM reported synced before posture")
	}
	_, updated = reconcileConfig(t, r)
	members = bindingMembers(api.policies[configExternalRef], wsapi.RoleConfigAdmin)
	if len(members) != 1 || members[0] != "user:alice@example.com" {
		t.Fatalf("admins after posture = %v", members)
	}
	if !metaConditionTrue(updated.Status.Conditions, ConditionIAMPolicySynced) || updated.Status.Phase != "Ready" {
		t.Fatalf("status after grant = %+v", updated.Status)
	}
}

func TestWorkstationConfigReconcile_PermissionDeniedAndAborted(t *testing.T) {
	denied := &recordingAPI{getErr: status.Error(codes.PermissionDenied, "denied")}
	cfg := &wsv1alpha1.WorkstationConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "test-config", Namespace: "cloud-workstations", Finalizers: []string{ConfigFinalizer}},
		Spec:       wsv1alpha1.WorkstationConfigSpec{ClusterRef: wsv1alpha1.ClusterRef{Name: "parent-cluster"}},
	}
	r := configReconciler(t, cfg, readyConfigObject(configExternalRef, true), denied)
	res, updated := reconcileConfig(t, r)
	if res.RequeueAfter != apiErrorInterval {
		t.Fatalf("permission denied requeue = %s", res.RequeueAfter)
	}
	cond := conditionByType(updated.Status.Conditions, ConditionPostureApplied)
	if cond == nil || cond.Reason != "PermissionDenied" {
		t.Fatalf("condition = %+v", cond)
	}

	aborted := &recordingAPI{cfg: openAPIConfig(), patchErr: status.Error(codes.Aborted, "etag")}
	cfg = &wsv1alpha1.WorkstationConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "test-config", Namespace: "cloud-workstations", Finalizers: []string{ConfigFinalizer}},
		Spec:       wsv1alpha1.WorkstationConfigSpec{ClusterRef: wsv1alpha1.ClusterRef{Name: "parent-cluster"}},
	}
	r = configReconciler(t, cfg, readyConfigObject(configExternalRef, true), aborted)
	res, updated = reconcileConfig(t, r)
	if res.RequeueAfter != apiWaitInterval {
		t.Fatalf("aborted requeue = %s", res.RequeueAfter)
	}
	cond = conditionByType(updated.Status.Conditions, ConditionPostureApplied)
	if cond == nil || cond.Reason != "Aborted" {
		t.Fatalf("condition = %+v", cond)
	}
}

func TestWorkstationConfigReconcile_IdleTimeoutDoesNotPatchPosture(t *testing.T) {
	api := &recordingAPI{cfg: &pb.WorkstationConfig{
		Name: configExternalRef,
		Etag: "etag-1",
		Host: &pb.WorkstationConfig_Host{Config: &pb.WorkstationConfig_Host_GceInstance_{
			GceInstance: &pb.WorkstationConfig_Host_GceInstance{DisableSsh: true},
		}},
		AllowedPorts: []*pb.WorkstationConfig_PortRange{{First: 80, Last: 80}, {First: 1024, Last: 65535}},
	}}
	cfg := &wsv1alpha1.WorkstationConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "test-config", Namespace: "cloud-workstations", Finalizers: []string{ConfigFinalizer}},
		Spec: wsv1alpha1.WorkstationConfigSpec{
			ClusterRef:  wsv1alpha1.ClusterRef{Name: "parent-cluster"},
			IdleTimeout: "7200s",
			Host:        &wsv1alpha1.HostConfig{GceInstance: &wsv1alpha1.GceInstanceConfig{MachineType: "e2-standard-4"}},
		},
	}
	r := configReconciler(t, cfg, readyConfigObject(configExternalRef, true), api)
	_, updated := reconcileConfig(t, r)
	if api.count("PatchConfigPosture") != 0 {
		t.Fatalf("idleTimeout change wrote posture: %v", api.calls)
	}
	if updated.Status.Phase != "Ready" {
		t.Fatalf("phase = %s", updated.Status.Phase)
	}
	kccObj := &unstructured.Unstructured{}
	kccObj.SetGroupVersionKind(kccConfigGVK)
	if err := r.Get(context.Background(), types.NamespacedName{Namespace: "cloud-workstations", Name: "test-config"}, kccObj); err != nil {
		t.Fatal(err)
	}
	idle, _, _ := unstructured.NestedString(kccObj.Object, "spec", "idleTimeout")
	if idle != "7200s" {
		t.Fatalf("idleTimeout = %q", idle)
	}
	pool, found, err := unstructured.NestedInt64(kccObj.Object, "spec", "host", "gceInstance", "poolSize")
	if err != nil || !found || pool != 0 {
		t.Fatalf("poolSize found=%v value=%d err=%v", found, pool, err)
	}
}

func conditionByType(conditions []metav1.Condition, conditionType string) *metav1.Condition {
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return &conditions[i]
		}
	}
	return nil
}

func bindingMembers(policy *iampb.Policy, role string) []string {
	if policy == nil {
		return nil
	}
	for _, binding := range policy.GetBindings() {
		if binding.GetRole() == role && binding.GetCondition() == nil {
			return binding.GetMembers()
		}
	}
	return nil
}

func TestWorkstationReconcile_UserGrantWaitsForParentReady(t *testing.T) {
	const wsRef = "projects/p/locations/l/workstationClusters/c/workstationConfigs/cfg/workstations/test-ws"
	api := &recordingAPI{policies: map[string]*iampb.Policy{
		wsRef: {Version: 3, Etag: []byte("etag"), Bindings: []*iampb.Binding{{
			Role: wsapi.RoleWorkstationUser, Members: []string{"user:bob@example.com"},
		}}},
	}}
	scheme := setupWorkstationTestScheme()
	parent := &wsv1alpha1.WorkstationConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "parent-config", Namespace: "cloud-workstations"},
		Status: wsv1alpha1.WorkstationConfigStatus{Phase: "Pending", Conditions: []metav1.Condition{{
			Type: ConditionReady, Status: metav1.ConditionFalse, Reason: "NotReady", Message: "posture",
		}}},
	}
	ws := &wsv1alpha1.Workstation{
		ObjectMeta: metav1.ObjectMeta{Name: "test-ws", Namespace: "cloud-workstations", Finalizers: []string{WorkstationFinalizer}},
		Spec:       wsv1alpha1.WorkstationSpec{ConfigRef: wsv1alpha1.ConfigRef{Name: "parent-config"}, Users: []string{"user:alice@example.com"}},
	}
	kccObj := readyWorkstationObject(wsRef)
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(parent, ws, kccObj).WithStatusSubresource(parent, ws).Build()
	r := &WorkstationReconciler{Client: c, Scheme: scheme, WS: api}
	res, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "cloud-workstations", Name: "test-ws"}})
	if err != nil {
		t.Fatal(err)
	}
	if res.RequeueAfter != apiWaitInterval {
		t.Fatalf("requeue = %s", res.RequeueAfter)
	}
	members := bindingMembers(api.policies[wsRef], wsapi.RoleWorkstationUser)
	if len(members) != 0 {
		t.Fatalf("user grant before parent Ready: %v", members)
	}
	for _, call := range api.calls {
		if call == "StartWorkstation" || call == "StopWorkstation" {
			t.Fatalf("power call %s", call)
		}
	}
	got := &wsv1alpha1.Workstation{}
	if err := c.Get(context.Background(), types.NamespacedName{Namespace: "cloud-workstations", Name: "test-ws"}, got); err != nil {
		t.Fatal(err)
	}
	if got.Status.Phase == "Ready" {
		t.Fatal("workstation became Ready before the parent config")
	}
	cond := conditionByType(got.Status.Conditions, ConditionIAMPolicySynced)
	if cond == nil || cond.Reason != "WaitingForConfig" {
		t.Fatalf("iam condition = %+v", cond)
	}

	parent.Status.Conditions[0].Status = metav1.ConditionTrue
	parent.Status.Conditions[0].Reason = "Ready"
	if err := c.Status().Update(context.Background(), parent); err != nil {
		t.Fatal(err)
	}
	res, err = r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "cloud-workstations", Name: "test-ws"}})
	if err != nil {
		t.Fatal(err)
	}
	members = bindingMembers(api.policies[wsRef], wsapi.RoleWorkstationUser)
	if len(members) != 1 || members[0] != "user:alice@example.com" {
		t.Fatalf("users after parent Ready = %v", members)
	}
	if err := c.Get(context.Background(), types.NamespacedName{Namespace: "cloud-workstations", Name: "test-ws"}, got); err != nil {
		t.Fatal(err)
	}
	if got.Status.Phase != "Ready" || res.RequeueAfter != defaultAPIResync {
		t.Fatalf("phase %s requeue %s conditions %+v", got.Status.Phase, res.RequeueAfter, got.Status.Conditions)
	}
}

func readyWorkstationObject(externalRef string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "workstations.cnrm.cloud.google.com/v1beta1",
		"kind":       "Workstation",
		"metadata": map[string]interface{}{
			"name":      "test-ws",
			"namespace": "cloud-workstations",
		},
		"status": map[string]interface{}{
			"externalRef": externalRef,
			"conditions": []interface{}{map[string]interface{}{
				"type": "Ready", "status": "True", "reason": "UpToDate", "message": "ready",
			}},
		},
	}}
}
