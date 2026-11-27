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
	"reflect"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"mtk-connect-module/internal/metrics"
	"mtk-connect-module/internal/mtk"
)

const (
	reasonInstanceNotReady = "InstanceNotReady"
	reasonAwaitingAgent    = "AwaitingAgent"

	testbenchRetryDelay   = 30 * time.Second
	testbenchPollDelay    = 15 * time.Second // waiting for the agent to connect
	testbenchOnlineDelay  = time.Minute      // checking that a connected agent stays connected
	testbenchResyncDelay  = 5 * time.Minute  // terminal failures that may still be repaired outside the cluster
	instanceObjectDefault = "default"
)

var (
	errInstanceMissing  = errors.New("MTKConnectInstance not found")
	errInstanceNotReady = errors.New("MTKConnectInstance has no usable API key yet")
)

// TestbenchAPI is the part of the MTK Connect client the testbench reconciler needs.
type TestbenchAPI interface {
	FindAgent(ctx context.Context, registration string) (*mtk.Agent, error)
	EnsureRegistration(ctx context.Context, name, token string) error
	CreateAgent(ctx context.Context, name, registration string) (mtk.Agent, error)
	DeleteAgent(ctx context.Context, id string) error
	DeleteRegistration(ctx context.Context, name string) error
	GrantGroupByName(ctx context.Context, agentID, group, permission string) error
	GrantUserByName(ctx context.Context, agentID, username, permission string) error
}

// TestbenchReconciler reconciles MTKConnectTestbench.
//
// Client reads the custom resources and the MTKConnectInstance through the manager cache. Direct is an
// uncached client used for Secrets and Events. The controller keeps no state of its own: the agent is found
// again by its registration name on every reconcile, so a restart at any point creates no duplicates.
type TestbenchReconciler struct {
	Client client.Client
	Direct client.Client

	// ManagerNamespace and InstanceName locate the MTKConnectInstance that holds the server URL and API key.
	ManagerNamespace string
	InstanceName     string

	// URLPath is the portal path reported in status.url; {id} is replaced by the agent id.
	URLPath string

	// WatchNamespaces are the namespaces whose testbenches are counted. A list without a namespace is
	// cluster-scoped, and this account only has a Role in each of these namespaces.
	WatchNamespaces []string

	NewAPI func(serverURL, username, key string) TestbenchAPI
	Now    func() time.Time
}

type testbenchStatus struct {
	Phase         string
	AgentID       string
	Registration  string
	TokenSecret   string // namespace/name of the Secret that holds the registration token, never the token
	URL           string
	AccessGranted bool
	Generation    int64
	Conditions    map[string]*condition
}

func (r *TestbenchReconciler) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

func (r *TestbenchReconciler) instanceName() string {
	return orDefault(r.InstanceName, instanceObjectDefault)
}

// SetupWithManager registers the controller. Status writes change neither the generation nor the
// annotations, so they do not trigger another reconcile; the controller polls with RequeueAfter instead.
func (r *TestbenchReconciler) SetupWithManager(mgr ctrl.Manager) error {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(TestbenchGVK)
	changed := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			return e.ObjectOld.GetGeneration() != e.ObjectNew.GetGeneration() ||
				!e.ObjectOld.GetDeletionTimestamp().Equal(e.ObjectNew.GetDeletionTimestamp()) ||
				!reflect.DeepEqual(e.ObjectOld.GetAnnotations(), e.ObjectNew.GetAnnotations())
		},
	}
	return ctrl.NewControllerManagedBy(mgr).
		Named("mtkconnecttestbench").
		For(u, builder.WithPredicates(changed)).
		Complete(r)
}

// Reconcile creates the agent for a testbench, grants access, and reports its state.
func (r *TestbenchReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	tb := &unstructured.Unstructured{}
	tb.SetGroupVersionKind(TestbenchGVK)
	if err := r.Client.Get(ctx, req.NamespacedName, tb); err != nil {
		if client.IgnoreNotFound(err) == nil {
			r.updateMetrics(ctx)
		}
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	defer r.updateMetrics(ctx)

	if !tb.GetDeletionTimestamp().IsZero() {
		return r.finalize(ctx, tb)
	}

	st := loadTestbenchStatus(tb)
	st.Generation = tb.GetGeneration()

	spec, err := parseTestbenchSpec(tb)
	if err != nil {
		log.FromContext(ctx).Error(err, "invalid MTKConnectTestbench spec")
		r.setCondition(ctx, tb, st, TbCondSpecValid, false, "InvalidSpec", err.Error())
		st.Phase = TbFailed
		return ctrl.Result{}, r.writeStatus(ctx, tb, st)
	}
	r.setCondition(ctx, tb, st, TbCondSpecValid, true, "Valid", "")

	// The finalizer goes on before anything is created in MTK Connect, so a deleted resource can never
	// leave an agent behind that nobody is responsible for.
	if !controllerutil.ContainsFinalizer(tb, FinalizerAgentCleanup) {
		orig := tb.DeepCopy()
		controllerutil.AddFinalizer(tb, FinalizerAgentCleanup)
		if err := r.Client.Patch(ctx, tb, client.MergeFromWithOptions(orig, client.MergeFromWithOptimisticLock{})); err != nil {
			return ctrl.Result{}, fmt.Errorf("add finalizer: %w", err)
		}
	}

	after := r.sync(ctx, tb, spec, st)
	if err := r.writeStatus(ctx, tb, st); err != nil {
		return ctrl.Result{}, err
	}
	if after <= 0 {
		return ctrl.Result{}, nil
	}
	return ctrl.Result{RequeueAfter: after}, nil
}

// sync drives the agent towards the spec and returns the delay until the next reconcile.
func (r *TestbenchReconciler) sync(ctx context.Context, tb *unstructured.Unstructured, spec *TestbenchSpec, st *testbenchStatus) time.Duration {
	logger := log.FromContext(ctx)
	if st.Phase == "" || st.Phase == TbFailed {
		st.Phase = TbPending
	}

	api, domain, err := r.connect(ctx)
	if err != nil {
		reason := reasonInstanceNotReady
		logger.Info("MTK Connect is not reachable yet", "reason", err.Error())
		r.setCondition(ctx, tb, st, TbCondInstanceReady, false, reason, err.Error())
		return testbenchRetryDelay
	}
	r.setCondition(ctx, tb, st, TbCondInstanceReady, true, "Ready", "")

	registration := registrationFor(tb.GetNamespace(), tb.GetName(), string(tb.GetUID()))
	st.Registration = registration

	agent, err := api.FindAgent(ctx, registration)
	if err != nil {
		logger.Error(err, "look up agent failed")
		r.setCondition(ctx, tb, st, TbCondAgentProvision, false, "LookupFailed", err.Error())
		return testbenchRetryDelay
	}
	switch {
	case agent == nil && st.AgentID != "":
		// The agent was recorded earlier and is gone now. Creating another one would hide that someone or
		// something deleted it, and would change the URL, so report it and keep checking.
		msg := fmt.Sprintf("agent %s no longer exists in MTK Connect; delete and re-create the testbench", st.AgentID)
		r.setCondition(ctx, tb, st, TbCondAgentProvision, false, "AgentMissing", msg)
		st.Phase = TbFailed
		return testbenchResyncDelay
	case agent == nil:
		// The token is only needed here, so a steady-state reconcile never reads the Secret. It is stored
		// before the registration is created, so a retry after a crash sends the same token.
		token, err := r.registrationToken(ctx, registration)
		if err != nil {
			logger.Error(err, "registration token unavailable")
			r.setCondition(ctx, tb, st, TbCondAgentProvision, false, "TokenFailed", err.Error())
			return testbenchRetryDelay
		}
		st.TokenSecret = r.ManagerNamespace + "/" + testbenchTokenSecret
		if err := api.EnsureRegistration(ctx, registration, token); err != nil {
			logger.Error(err, "create registration failed")
			r.setCondition(ctx, tb, st, TbCondAgentProvision, false, "RegistrationFailed", err.Error())
			return testbenchRetryDelay
		}
		created, err := api.CreateAgent(ctx, spec.DisplayName, registration)
		if err != nil {
			logger.Error(err, "create agent failed")
			r.setCondition(ctx, tb, st, TbCondAgentProvision, false, "CreateFailed", err.Error())
			return testbenchRetryDelay
		}
		agent = &created
		r.event(ctx, tb, corev1.EventTypeNormal, "AgentCreated", fmt.Sprintf("created MTK Connect agent %s (registration %s)", created.ID, registration))
	}
	st.AgentID = agent.ID
	st.URL = testbenchURL(domain, r.URLPath, agent.ID)
	r.setCondition(ctx, tb, st, TbCondAgentProvision, true, "Provisioned", "")
	if st.Phase == TbPending {
		st.Phase = TbAgentProvisioned
	}

	if !st.AccessGranted {
		if err := grantAccess(ctx, api, agent.ID, spec); err != nil {
			logger.Error(err, "grant access failed")
			r.setCondition(ctx, tb, st, TbCondAccessGranted, false, "GrantFailed", err.Error())
			return testbenchRetryDelay
		}
		st.AccessGranted = true
	}
	r.setCondition(ctx, tb, st, TbCondAccessGranted, true, "Granted", "")

	switch {
	case agent.Online:
		st.Phase = TbReady
		r.setCondition(ctx, tb, st, TbCondAgentOnline, true, "Online", "")
		return testbenchOnlineDelay
	case st.Phase == TbReady || st.Phase == TbOffline:
		st.Phase = TbOffline
		r.setCondition(ctx, tb, st, TbCondAgentOnline, false, "AgentOffline", "the agent is not connected to MTK Connect")
		return testbenchRetryDelay
	default:
		st.Phase = TbAwaitingAgent
		msg := fmt.Sprintf("waiting for an agent to register as %q", registration)
		r.setCondition(ctx, tb, st, TbCondAgentOnline, false, reasonAwaitingAgent, msg)
		return testbenchPollDelay
	}
}

func grantAccess(ctx context.Context, api TestbenchAPI, agentID string, spec *TestbenchSpec) error {
	if spec.Access.Public {
		if err := api.GrantGroupByName(ctx, agentID, everyoneGroup, spec.Access.Permission); err != nil {
			return fmt.Errorf("grant group %s: %w", everyoneGroup, err)
		}
	}
	for _, u := range spec.Access.Users {
		if err := api.GrantUserByName(ctx, agentID, u, spec.Access.Permission); err != nil {
			return fmt.Errorf("grant user %s: %w", u, err)
		}
	}
	return nil
}

// finalize deletes the agent, then lets the resource go.
func (r *TestbenchReconciler) finalize(ctx context.Context, tb *unstructured.Unstructured) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(tb, FinalizerAgentCleanup) {
		return ctrl.Result{}, nil
	}
	logger := log.FromContext(ctx)

	if tb.GetAnnotations()[AnnotationSkipCleanup] == "true" {
		logger.Info("skipping agent cleanup as requested", "annotation", AnnotationSkipCleanup)
		r.event(ctx, tb, corev1.EventTypeWarning, "CleanupSkipped", "the MTK Connect agent was not deleted ("+AnnotationSkipCleanup+")")
		return ctrl.Result{}, r.removeFinalizer(ctx, tb)
	}

	st := loadTestbenchStatus(tb)
	if st.Phase != TbDeleting {
		st.Phase = TbDeleting
		st.Generation = tb.GetGeneration()
		if err := r.writeStatus(ctx, tb, st); err != nil {
			logger.Error(err, "record Deleting phase failed")
		}
	}

	api, _, err := r.connect(ctx)
	if err != nil {
		msg := "cannot delete the MTK Connect agent: " + err.Error()
		if errors.Is(err, errInstanceMissing) {
			msg += "; set annotation " + AnnotationSkipCleanup + "=true to delete the testbench anyway"
		}
		logger.Info(msg)
		r.event(ctx, tb, corev1.EventTypeWarning, "CleanupBlocked", msg)
		return ctrl.Result{RequeueAfter: testbenchRetryDelay}, nil
	}

	registration := registrationFor(tb.GetNamespace(), tb.GetName(), string(tb.GetUID()))
	agent, err := api.FindAgent(ctx, registration)
	if err == nil && agent != nil {
		err = api.DeleteAgent(ctx, agent.ID)
		if err == nil {
			r.event(ctx, tb, corev1.EventTypeNormal, "AgentDeleted", fmt.Sprintf("deleted MTK Connect agent %s", agent.ID))
		}
	}
	if err == nil {
		err = api.DeleteRegistration(ctx, registration)
	}
	if err == nil {
		err = r.forgetRegistrationToken(ctx, registration)
	}
	if err != nil {
		logger.Error(err, "delete agent failed")
		r.event(ctx, tb, corev1.EventTypeWarning, "CleanupFailed", err.Error())
		return ctrl.Result{RequeueAfter: testbenchRetryDelay}, nil
	}
	return ctrl.Result{}, r.removeFinalizer(ctx, tb)
}

func (r *TestbenchReconciler) removeFinalizer(ctx context.Context, tb *unstructured.Unstructured) error {
	orig := tb.DeepCopy()
	controllerutil.RemoveFinalizer(tb, FinalizerAgentCleanup)
	err := r.Client.Patch(ctx, tb, client.MergeFromWithOptions(orig, client.MergeFromWithOptimisticLock{}))
	if err != nil {
		return client.IgnoreNotFound(fmt.Errorf("remove finalizer: %w", err))
	}
	return nil
}

// connect builds an MTK Connect client from the MTKConnectInstance in the manager namespace. The
// instance is the single place that says where the server is and which Secret holds the API key.
func (r *TestbenchReconciler) connect(ctx context.Context) (TestbenchAPI, string, error) {
	conn, err := loadConnection(ctx, r.Client, r.Direct, r.ManagerNamespace, r.instanceName())
	if err != nil {
		return nil, "", err
	}
	return r.NewAPI(conn.ServerURL, conn.Username, conn.Key), conn.Domain, nil
}

// connection says how to reach MTK Connect.
type connection struct {
	ServerURL string
	Username  string
	Key       string
	Domain    string
}

// loadConnection reads the server URL, user and API key from the MTKConnectInstance and its API key Secret.
func loadConnection(ctx context.Context, reader, direct client.Reader, namespace, name string) (*connection, error) {
	inst := &unstructured.Unstructured{}
	inst.SetGroupVersionKind(InstanceGVK)
	if err := reader.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, inst); err != nil {
		if client.IgnoreNotFound(err) == nil {
			return nil, errInstanceMissing
		}
		return nil, fmt.Errorf("read MTKConnectInstance: %w", err)
	}
	spec, _, err := parseSpec(inst)
	if err != nil {
		return nil, fmt.Errorf("MTKConnectInstance is invalid: %w", err)
	}
	password, found, err := readSecretValue(ctx, direct, inst.GetNamespace(), spec.Server.APIKeySecret, secretKeyPassword)
	if err != nil {
		return nil, fmt.Errorf("read API key Secret: %w", err)
	}
	if !found {
		return nil, errInstanceNotReady
	}
	return &connection{ServerURL: spec.Server.URL, Username: spec.ServiceAccountUser, Key: password, Domain: spec.Domain}, nil
}

func (r *TestbenchReconciler) setCondition(ctx context.Context, tb *unstructured.Unstructured, st *testbenchStatus, typ string, ok bool, reason, message string) {
	prev := st.Conditions[typ]
	c := &condition{Type: typ, Status: ok, Reason: reason, Message: message, LastTransitionTime: r.now().UTC().Format(time.RFC3339)}
	if prev != nil && prev.Status == ok {
		c.LastTransitionTime = prev.LastTransitionTime
	}
	st.Conditions[typ] = c
	if !ok && reason != reasonAwaitingAgent && reason != reasonInstanceNotReady && (prev == nil || prev.Status != ok || prev.Message != message) {
		r.event(ctx, tb, corev1.EventTypeWarning, reason, message)
	}
}

func (r *TestbenchReconciler) event(ctx context.Context, tb *unstructured.Unstructured, typ, reason, message string) {
	recordEvent(ctx, r.Direct, tb, r.now(), typ, reason, message)
}

// updateMetrics sets mtkc_testbenches from the cached resources. Failures are only logged.
func (r *TestbenchReconciler) updateMetrics(ctx context.Context) {
	namespaces := r.WatchNamespaces
	if len(namespaces) == 0 && r.ManagerNamespace != "" {
		namespaces = []string{r.ManagerNamespace}
	}
	counts := map[string]int{}
	for _, ns := range namespaces {
		list := &unstructured.UnstructuredList{}
		list.SetGroupVersionKind(TestbenchGVK.GroupVersion().WithKind(KindTestbench + "List"))
		if err := r.Client.List(ctx, list, client.InNamespace(ns)); err != nil {
			log.FromContext(ctx).Error(err, "list testbenches for metrics failed", "namespace", ns)
			return
		}
		for i := range list.Items {
			phase, _, _ := unstructured.NestedString(list.Items[i].Object, "status", "phase")
			if phase == "" {
				phase = TbPending
			}
			counts[phase]++
		}
	}
	for _, p := range testbenchPhases {
		metrics.Testbenches.WithLabelValues(p).Set(float64(counts[p]))
	}
}

func loadTestbenchStatus(tb *unstructured.Unstructured) *testbenchStatus {
	base := loadStatus(tb)
	st := &testbenchStatus{Phase: base.Phase, Conditions: base.Conditions}
	st.AgentID, _, _ = unstructured.NestedString(tb.Object, "status", "agentId")
	st.Registration, _, _ = unstructured.NestedString(tb.Object, "status", "registration")
	st.TokenSecret, _, _ = unstructured.NestedString(tb.Object, "status", "registrationTokenSecret")
	st.URL, _, _ = unstructured.NestedString(tb.Object, "status", "url")
	st.AccessGranted, _, _ = unstructured.NestedBool(tb.Object, "status", "accessGranted")
	return st
}

func (r *TestbenchReconciler) writeStatus(ctx context.Context, tb *unstructured.Unstructured, st *testbenchStatus) error {
	conds := make([]interface{}, 0, len(testbenchConditionOrder))
	for _, typ := range testbenchConditionOrder {
		c := st.Conditions[typ]
		if c == nil {
			continue
		}
		status := "False"
		if c.Status {
			status = "True"
		}
		conds = append(conds, map[string]interface{}{
			"type":               c.Type,
			"status":             status,
			"reason":             c.Reason,
			"message":            c.Message,
			"lastTransitionTime": c.LastTransitionTime,
		})
	}
	status := map[string]interface{}{
		"phase":              st.Phase,
		"observedGeneration": st.Generation,
		"accessGranted":      st.AccessGranted,
		"conditions":         conds,
	}
	if st.AgentID != "" {
		status["agentId"] = st.AgentID
	}
	if st.Registration != "" {
		status["registration"] = st.Registration
	}
	if st.TokenSecret != "" {
		status["registrationTokenSecret"] = st.TokenSecret
	}
	if st.URL != "" {
		status["url"] = st.URL
	}
	orig := tb.DeepCopy()
	if err := unstructured.SetNestedField(tb.Object, status, "status"); err != nil {
		return fmt.Errorf("build status: %w", err)
	}
	if err := r.Client.Status().Patch(ctx, tb, client.MergeFrom(orig)); err != nil {
		return fmt.Errorf("patch status: %w", err)
	}
	return nil
}
