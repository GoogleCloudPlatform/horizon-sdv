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
	"io"
	"net"
	"net/url"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"mtk-connect-module/internal/keycloak"
	"mtk-connect-module/internal/metrics"
	"mtk-connect-module/internal/mtk"
)

// Condition types reported in status.conditions.
const (
	CondSpecValid           = "SpecValid"
	CondKeycloakReady       = "KeycloakReady"
	CondServiceAccountReady = "ServiceAccountReady"
	CondCredsDistributed    = "CredentialsDistributed"
	CondKeyRotated          = "KeyRotated"
	CondKeysCleaned         = "OldKeysCleaned"
)

// Phases reported in status.phase.
const (
	PhaseConfiguring = "Configuring"
	PhaseReady       = "Ready"
	PhaseDegraded    = "Degraded"
)

const (
	reasonServerNotReady = "ServerNotReady"

	retryDelay  = time.Minute
	resyncDelay = 5 * time.Minute

	maxEventMessage = 900
)

var conditionOrder = []string{
	CondSpecValid, CondKeycloakReady, CondServiceAccountReady,
	CondCredsDistributed, CondKeyRotated, CondKeysCleaned,
}

// KeycloakAPI is the part of the Keycloak client the reconciler needs.
type KeycloakAPI interface {
	Login(ctx context.Context, username, password string) error
	EnsureSAMLClient(ctx context.Context, realm string, spec keycloak.SAMLClientSpec) (keycloak.Material, error)
}

// MTKAPI is the part of the MTK Connect client the reconciler needs.
type MTKAPI interface {
	Version(ctx context.Context) (string, error)
	FindUserID(ctx context.Context, username string) (string, error)
	Keys(ctx context.Context, userID string) ([]mtk.Key, error)
	CreateKey(ctx context.Context, userID string, expiry time.Time) (string, error)
	DeleteKey(ctx context.Context, userID, keyID string) error
}

// Execer runs a command in a pod container and returns stdout.
type Execer interface {
	Run(ctx context.Context, namespace, pod, container string, command []string) (string, error)
}

// InstanceReconciler reconciles MTKConnectInstance.
//
// Client reads the custom resource through the manager cache (restricted to the module namespace).
// Direct is an uncached client used for Secrets, Pods, Deployments and Events, which live in several namespaces.
type InstanceReconciler struct {
	Client client.Client
	Direct client.Client
	Exec   Execer

	// InstanceName is the one MTKConnectInstance this controller reconciles (default "default"), the same
	// resource the testbench and device controllers read. Others in the namespace are ignored.
	InstanceName string

	NewKeycloak func(baseURL string) KeycloakAPI
	NewMTK      func(serverURL, username, key string) MTKAPI
	Now         func() time.Time

	mu       sync.Mutex
	rejected map[string]int // consecutive reconciles that saw the stored API key rejected, per instance
}

type condition struct {
	Type               string
	Status             bool
	Reason             string
	Message            string
	LastTransitionTime string
}

type instanceStatus struct {
	Phase            string
	Conditions       map[string]*condition
	CurrentKeyID     string
	LastRotationTime time.Time
	RotateNowHandled string
	ServerVersion    string
	Generation       int64
}

func (r *InstanceReconciler) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

// SetupWithManager registers the controller. Status writes do not change the generation or the
// annotations, so they do not trigger another reconcile.
func (r *InstanceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(InstanceGVK)
	changed := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			return e.ObjectOld.GetGeneration() != e.ObjectNew.GetGeneration() ||
				!reflect.DeepEqual(e.ObjectOld.GetAnnotations(), e.ObjectNew.GetAnnotations())
		},
	}
	return ctrl.NewControllerManagedBy(mgr).
		Named("mtkconnectinstance").
		For(u, builder.WithPredicates(changed)).
		Complete(r)
}

// Reconcile drives the Keycloak client, the service account, key rotation and Secret distribution
// towards the spec. Failures are recorded in status, an Event and metrics, then retried; the
// function returns an error only when the status itself cannot be written.
func (r *InstanceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	if req.Name != orDefault(r.InstanceName, instanceObjectDefault) {
		return ctrl.Result{}, nil
	}
	inst := &unstructured.Unstructured{}
	inst.SetGroupVersionKind(InstanceGVK)
	if err := r.Client.Get(ctx, req.NamespacedName, inst); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if !inst.GetDeletionTimestamp().IsZero() {
		return ctrl.Result{}, nil
	}

	st := loadStatus(inst)
	st.Generation = inst.GetGeneration()

	spec, timings, err := parseSpec(inst)
	var after time.Duration
	if err != nil {
		logger.Error(err, "invalid MTKConnectInstance spec")
		r.setCondition(ctx, inst, st, CondSpecValid, false, "InvalidSpec", err.Error())
		after = 0 // wait for the spec to change
	} else {
		r.setCondition(ctx, inst, st, CondSpecValid, true, "Valid", "")
		after = r.sync(ctx, inst, spec, timings, st)
	}
	st.Phase = st.phase()
	if err := r.writeStatus(ctx, inst, st); err != nil {
		return ctrl.Result{}, err
	}
	if after <= 0 {
		return ctrl.Result{}, nil
	}
	return ctrl.Result{RequeueAfter: after}, nil
}

// sync runs all steps and returns the delay until the next reconcile.
func (r *InstanceReconciler) sync(ctx context.Context, inst *unstructured.Unstructured, spec *InstanceSpec, t Timings, st *instanceStatus) time.Duration {
	logger := log.FromContext(ctx)
	now := r.now()
	ns := inst.GetNamespace()
	failed := false

	// 1. Keycloak SAML client and mtk-connect-keycloak Secret; roll the server when it changes.
	if err := r.syncKeycloak(ctx, ns, spec); err != nil {
		failed = true
		logger.Error(err, "keycloak sync failed")
		r.setCondition(ctx, inst, st, CondKeycloakReady, false, "KeycloakFailed", err.Error())
	} else {
		r.setCondition(ctx, inst, st, CondKeycloakReady, true, "Configured", "")
	}

	// 2. Service account and API key (bootstrap on first run or when the key was revoked).
	sa, err := r.syncServiceAccount(ctx, inst, ns, spec, st, now)
	if err != nil {
		failed = true
		reason := "ServiceAccountFailed"
		if errors.Is(err, errNoServerPod) || errors.Is(err, errServerUnavailable) {
			reason = reasonServerNotReady
		}
		logger.Error(err, "service account sync failed")
		r.setCondition(ctx, inst, st, CondServiceAccountReady, false, reason, err.Error())
		// Without a usable key there is nothing to rotate, distribute or clean up.
		for _, c := range []string{CondCredsDistributed, CondKeyRotated, CondKeysCleaned} {
			r.setCondition(ctx, inst, st, c, false, "Blocked", "waiting for the service account")
		}
		return retryDelay
	}
	r.setCondition(ctx, inst, st, CondServiceAccountReady, true, "Ready", "")

	// The creation time of the key in use is the source of truth for when it was rotated. Status can be
	// behind it (a status write that failed after a rotation), and trusting it would rotate again.
	keys, keysErr := sa.api.Keys(ctx, sa.userID)
	if keysErr != nil {
		logger.Error(keysErr, "list API keys failed")
	}
	if cur, ok := findCurrentKey(keys, sa.password); ok && !cur.Created.IsZero() {
		st.LastRotationTime = cur.Created
	} else if st.LastRotationTime.IsZero() {
		st.LastRotationTime = now
	}

	// 3. Rotation.
	token := inst.GetAnnotations()[AnnotationRotateNow]
	force := token != "" && token != st.RotateNowHandled
	if rotationDue(st.LastRotationTime, now, t.Interval, force) {
		if err := r.rotate(ctx, ns, spec, t, sa, st, now); err != nil {
			failed = true
			metrics.RotationFailures.Inc()
			logger.Error(err, "key rotation failed; keeping the existing key")
			r.setCondition(ctx, inst, st, CondKeyRotated, false, "RotationFailed", err.Error())
		} else {
			if force {
				st.RotateNowHandled = token
			}
			r.event(ctx, inst, corev1.EventTypeNormal, "KeyRotated", "API key rotated")
			r.setCondition(ctx, inst, st, CondKeyRotated, true, "Rotated", "")
		}
	} else {
		r.setCondition(ctx, inst, st, CondKeyRotated, true, "Current",
			"next rotation "+nextRotation(st.LastRotationTime, t.Interval).UTC().Format(time.RFC3339))
	}

	// 4. Distribute the current key to every Secret (also heals Secrets that were deleted or edited).
	if err := r.distribute(ctx, ns, spec, sa.password); err != nil {
		failed = true
		logger.Error(err, "credential distribution failed")
		r.setCondition(ctx, inst, st, CondCredsDistributed, false, "DistributionFailed", err.Error())
	} else {
		r.setCondition(ctx, inst, st, CondCredsDistributed, true, "Distributed", "")
	}

	// 5. Delete keys that were superseded more than the overlap ago.
	if err := r.cleanupKeys(ctx, spec, t, sa, st, now); err != nil {
		failed = true
		logger.Error(err, "old key cleanup failed")
		r.setCondition(ctx, inst, st, CondKeysCleaned, false, "CleanupFailed", err.Error())
	} else {
		r.setCondition(ctx, inst, st, CondKeysCleaned, true, "Cleaned", "")
	}

	if failed {
		return retryDelay
	}
	next := nextRotation(st.LastRotationTime, t.Interval).Sub(now)
	if next < 10*time.Second {
		next = 10 * time.Second
	}
	if next > resyncDelay {
		next = resyncDelay
	}
	return next
}

// syncKeycloak ensures the SAML client, writes the Secret the server mounts and rolls the server once per change.
func (r *InstanceReconciler) syncKeycloak(ctx context.Context, ns string, spec *InstanceSpec) error {
	ref := spec.Keycloak.AdminSecretRef
	password, found, err := readSecretValue(ctx, r.Direct, ref.Namespace, ref.Name, ref.Key)
	if err != nil {
		return fmt.Errorf("read Keycloak admin secret %s/%s: %w", ref.Namespace, ref.Name, err)
	}
	if !found {
		return fmt.Errorf("Keycloak admin secret %s/%s (key %q) not found", ref.Namespace, ref.Name, ref.Key)
	}
	kc := r.NewKeycloak(spec.Keycloak.URL)
	if err := kc.Login(ctx, spec.Keycloak.AdminUser, password); err != nil {
		return err
	}
	consume := "https://" + spec.Domain + "/mtk-connect/saml/consume"
	material, err := kc.EnsureSAMLClient(ctx, spec.Keycloak.Realm, keycloak.SAMLClientSpec{
		ClientID:     spec.Keycloak.SAMLClientID,
		AdminURL:     consume,
		RedirectURIs: []string{consume},
		Attributes:   map[string]string{"saml.assertion.signature": "true"},
		Mappers: []keycloak.ProtocolMapper{{
			Name:           "X500 email",
			Protocol:       "saml",
			ProtocolMapper: "saml-user-property-mapper",
			Config: map[string]string{
				"attribute.nameformat": "urn:oasis:names:tc:SAML:2.0:attrname-format:uri",
				"user.attribute":       "email",
				"friendly.name":        "email",
				"attribute.name":       "urn:oid:1.2.840.113549.1.9.1",
			},
		}},
	})
	if err != nil {
		return err
	}
	hash := hashStrings(material.PrivateKey, material.IDPCert)
	if _, err := applySecret(ctx, r.Direct, desiredSecret{
		Namespace: ns,
		Name:      spec.Server.KeycloakSecret,
		Data: map[string][]byte{
			secretKeyPrivate: []byte(material.PrivateKey),
			secretKeyIDPCert: []byte(material.IDPCert),
		},
		Annotations: map[string]string{annotationKeycloakHash: hash},
	}); err != nil {
		return err
	}
	return r.rollServer(ctx, ns, spec.Server.Deployment, hash)
}

// rollServer stamps the server pod template with the Keycloak material hash, which restarts the pods
// only when the hash differs from the one already applied.
func (r *InstanceReconciler) rollServer(ctx context.Context, ns, name, hash string) error {
	var dep appsv1.Deployment
	if err := r.Direct.Get(ctx, client.ObjectKey{Namespace: ns, Name: name}, &dep); err != nil {
		if apierrors.IsNotFound(err) {
			return nil // not deployed yet; it starts with the Secret that now exists
		}
		return fmt.Errorf("get deployment %s/%s: %w", ns, name, err)
	}
	if dep.Spec.Template.Annotations[annotationKeycloakHash] == hash {
		return nil
	}
	orig := dep.DeepCopy()
	if dep.Spec.Template.Annotations == nil {
		dep.Spec.Template.Annotations = map[string]string{}
	}
	dep.Spec.Template.Annotations[annotationKeycloakHash] = hash
	if err := r.Direct.Patch(ctx, &dep, client.MergeFrom(orig)); err != nil {
		return fmt.Errorf("roll deployment %s/%s: %w", ns, name, err)
	}
	log.FromContext(ctx).Info("Keycloak material changed; rolling MTK Connect server", "deployment", name)
	return nil
}

type serviceAccount struct {
	api      MTKAPI
	userID   string
	password string
}

var (
	errNoServerPod       = errors.New("no ready MTK Connect server pod")
	errServerUnavailable = errors.New("MTK Connect server is not reachable")
)

// rejectionsBeforeBootstrap is how many consecutive reconciles must see the stored key rejected
// before a new service-account key is created.
const rejectionsBeforeBootstrap = 2

// apiKeyPattern is what the last line of createServiceAccount.js must look like to be taken as the new key:
// one token, so an error message or a stack-trace line is never stored as the credential.
var apiKeyPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{8,128}$`)

// serverUnavailable reports whether err means the MTK Connect server is not (yet) answering: a
// transport error, a timeout or an HTTP 5xx. It is not a verdict on the stored key, and it is not
// true when the server answered but has no such user (mtk.ErrUserNotFound) or sent something
// unexpected: those are reported as they are.
func serverUnavailable(err error) bool {
	var apiErr *mtk.APIError
	if errors.As(err, &apiErr) {
		return apiErr.Status >= 500
	}
	if errors.Is(err, mtk.ErrUserNotFound) {
		return false
	}
	var urlErr *url.Error
	var netErr net.Error
	return errors.As(err, &urlErr) || errors.As(err, &netErr) ||
		errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) || errors.Is(err, io.EOF)
}

func (r *InstanceReconciler) noteRejected(key string) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.rejected == nil {
		r.rejected = map[string]int{}
	}
	r.rejected[key]++
	return r.rejected[key]
}

func (r *InstanceReconciler) resetRejected(key string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.rejected, key)
}

// syncServiceAccount returns a verified client. It bootstraps the service account through the server pod
// when no key is stored or the stored key is rejected (401/403). Any other failure keeps the stored key.
func (r *InstanceReconciler) syncServiceAccount(ctx context.Context, inst *unstructured.Unstructured, ns string, spec *InstanceSpec, st *instanceStatus, now time.Time) (*serviceAccount, error) {
	user := spec.ServiceAccountUser
	password, found, err := readSecretValue(ctx, r.Direct, ns, spec.Server.APIKeySecret, secretKeyPassword)
	if err != nil {
		return nil, fmt.Errorf("read secret %s/%s: %w", ns, spec.Server.APIKeySecret, err)
	}
	var api MTKAPI
	var userID string
	bootstrap := !found
	instKey := inst.GetNamespace() + "/" + inst.GetName()
	if found {
		api = r.NewMTK(spec.Server.URL, user, password)
		userID, err = api.FindUserID(ctx, user)
		switch {
		case err == nil:
			r.resetRejected(instKey)
		case mtk.IsUnauthorized(err):
			// A single 401/403 can be transient (the server restarting, another writer rotating the key),
			// so a new key is only created when the rejection is seen on consecutive reconciles.
			if n := r.noteRejected(instKey); n < rejectionsBeforeBootstrap {
				return nil, fmt.Errorf("stored API key rejected (%d of %d); confirming on the next reconcile: %w",
					n, rejectionsBeforeBootstrap, err)
			}
			r.resetRejected(instKey)
			log.FromContext(ctx).Info("stored API key rejected repeatedly; creating a new service-account key")
			bootstrap = true
		case errors.Is(err, mtk.ErrUserNotFound):
			// The server answers and accepts the key, but the service account named in the spec does
			// not exist (for example spec.serviceAccountUser changed): create it.
			r.resetRejected(instKey)
			log.FromContext(ctx).Info("service account not found; creating it", "user", user)
			bootstrap = true
		case serverUnavailable(err):
			return nil, fmt.Errorf("%w: %v", errServerUnavailable, err)
		default:
			return nil, fmt.Errorf("verify API key: %w", err)
		}
	}
	if bootstrap {
		password, err = r.bootstrapKey(ctx, ns, spec)
		if err != nil {
			return nil, err
		}
		// Store before anything else so the new key cannot be lost.
		if _, err := applySecret(ctx, r.Direct, credentialSecrets(ns, spec, password)[0]); err != nil {
			return nil, fmt.Errorf("store new API key: %w", err)
		}
		st.LastRotationTime = now
		r.event(ctx, inst, corev1.EventTypeNormal, "ServiceAccountCreated", "service account API key created")
		api = r.NewMTK(spec.Server.URL, user, password)
		if userID, err = api.FindUserID(ctx, user); err != nil {
			return nil, fmt.Errorf("verify new API key: %w", err)
		}
	}
	if v, err := api.Version(ctx); err == nil {
		st.ServerVersion = v
	}
	return &serviceAccount{api: api, userID: userID, password: password}, nil
}

// bootstrapKey creates the service account (or a new key for it) with the server's own script.
func (r *InstanceReconciler) bootstrapKey(ctx context.Context, ns string, spec *InstanceSpec) (string, error) {
	sel, err := labels.Parse(spec.Server.PodSelector)
	if err != nil {
		return "", fmt.Errorf("spec.server.podSelector: %w", err)
	}
	var pods corev1.PodList
	if err := r.Direct.List(ctx, &pods, client.InNamespace(ns), client.MatchingLabelsSelector{Selector: sel}); err != nil {
		return "", fmt.Errorf("list server pods: %w", err)
	}
	var pod *corev1.Pod
	for i := range pods.Items {
		if podReady(&pods.Items[i]) {
			pod = &pods.Items[i]
			break
		}
	}
	if pod == nil {
		return "", errNoServerPod
	}
	out, err := r.Exec.Run(ctx, ns, pod.Name, spec.Server.Container,
		[]string{"node", "createServiceAccount.js", spec.ServiceAccountUser})
	if err != nil {
		return "", fmt.Errorf("create service account in pod %s: %w", pod.Name, err)
	}
	key := lastLine(out)
	if !apiKeyPattern.MatchString(key) {
		return "", errors.New("create service account: unexpected output from the server")
	}
	return key, nil
}

func podReady(p *corev1.Pod) bool {
	if p.DeletionTimestamp != nil || p.Status.Phase != corev1.PodRunning {
		return false
	}
	for _, c := range p.Status.Conditions {
		if c.Type == corev1.PodReady {
			return c.Status == corev1.ConditionTrue
		}
	}
	return false
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}

// rotate creates a new key with the current one and stores it in the server Secret, which is the source of
// truth. The other Secrets are written by distribute, which also runs on every reconcile, so a partial
// failure heals itself. The previous key stays valid until cleanupKeys removes it after the overlap.
func (r *InstanceReconciler) rotate(ctx context.Context, ns string, spec *InstanceSpec, t Timings, sa *serviceAccount, st *instanceStatus, now time.Time) error {
	newKey, err := sa.api.CreateKey(ctx, sa.userID, now.Add(t.KeyLifetime))
	if err != nil {
		return fmt.Errorf("create new API key: %w", err)
	}
	if _, err := applySecret(ctx, r.Direct, credentialSecrets(ns, spec, newKey)[0]); err != nil {
		// The key exists but nobody knows it; remove it so it does not stay valid until it expires.
		r.discardKey(ctx, sa, newKey)
		return fmt.Errorf("new API key created but not stored (key discarded): %w", err)
	}
	sa.password = newKey
	sa.api = r.NewMTK(spec.Server.URL, spec.ServiceAccountUser, newKey)
	st.LastRotationTime = now
	st.CurrentKeyID = ""
	return nil
}

// discardKey deletes a freshly created key that could not be stored. It is best effort: if the key
// cannot be found or deleted it is logged and left to expire.
func (r *InstanceReconciler) discardKey(ctx context.Context, sa *serviceAccount, keyValue string) {
	logger := log.FromContext(ctx)
	keys, err := sa.api.Keys(ctx, sa.userID)
	if err != nil {
		logger.Error(err, "could not list API keys to discard the unstored key")
		return
	}
	k, ok := findCurrentKey(keys, keyValue)
	if !ok {
		logger.Info("unstored API key not found in the key list; it will expire on its own")
		return
	}
	if err := sa.api.DeleteKey(ctx, sa.userID, k.ID); err != nil {
		logger.Error(err, "could not delete the unstored API key", "keyId", k.ID)
	}
}

// distribute writes the credentials to every Secret, continuing past individual failures.
func (r *InstanceReconciler) distribute(ctx context.Context, ns string, spec *InstanceSpec, password string) error {
	var failures []string
	for _, d := range credentialSecrets(ns, spec, password) {
		if _, err := applySecret(ctx, r.Direct, d); err != nil {
			failures = append(failures, err.Error())
		}
	}
	if len(failures) > 0 {
		return errors.New(strings.Join(failures, "; "))
	}
	return nil
}

// cleanupKeys records the current key id and deletes keys superseded longer than the overlap ago.
func (r *InstanceReconciler) cleanupKeys(ctx context.Context, spec *InstanceSpec, t Timings, sa *serviceAccount, st *instanceStatus, now time.Time) error {
	keys, err := sa.api.Keys(ctx, sa.userID)
	if err != nil {
		return fmt.Errorf("list API keys: %w", err)
	}
	cur, found := findCurrentKey(keys, sa.password)
	if found {
		st.CurrentKeyID = cur.ID
	}
	expiry := st.LastRotationTime.Add(t.KeyLifetime)
	if found && !cur.Expires.IsZero() {
		expiry = cur.Expires
	}
	metrics.KeyExpirySeconds.Set(expiry.Sub(now).Seconds())

	var failures []string
	for _, id := range keysToDelete(keys, cur, found, t.Overlap, now) {
		if err := sa.api.DeleteKey(ctx, sa.userID, id); err != nil {
			failures = append(failures, fmt.Sprintf("delete key %s: %v", id, err))
			continue
		}
		log.FromContext(ctx).Info("deleted superseded API key", "keyId", id)
	}
	if len(failures) > 0 {
		return errors.New(strings.Join(failures, "; "))
	}
	return nil
}

// setCondition records a condition and emits a Warning Event when a condition starts failing or its message changes.
func (r *InstanceReconciler) setCondition(ctx context.Context, inst *unstructured.Unstructured, st *instanceStatus, typ string, ok bool, reason, message string) {
	prev := st.Conditions[typ]
	c := &condition{Type: typ, Status: ok, Reason: reason, Message: message, LastTransitionTime: r.now().UTC().Format(time.RFC3339)}
	if prev != nil && prev.Status == ok {
		c.LastTransitionTime = prev.LastTransitionTime
	}
	st.Conditions[typ] = c
	if !ok && reason != "Blocked" && (prev == nil || prev.Status != ok || prev.Message != message) {
		r.event(ctx, inst, corev1.EventTypeWarning, reason, message)
	}
}

// event creates a Kubernetes Event about the instance. Failures are only logged.
func (r *InstanceReconciler) event(ctx context.Context, inst *unstructured.Unstructured, typ, reason, message string) {
	recordEvent(ctx, r.Direct, inst, r.now(), typ, reason, message)
}

// recordEvent creates a Kubernetes Event about obj, a resource of this API group. Failures are only logged.
func recordEvent(ctx context.Context, c client.Client, obj *unstructured.Unstructured, at time.Time, typ, reason, message string) {
	if len(message) > maxEventMessage {
		message = message[:maxEventMessage]
	}
	now := metav1.NewTime(at)
	ev := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{GenerateName: obj.GetName() + ".", Namespace: obj.GetNamespace()},
		InvolvedObject: corev1.ObjectReference{
			APIVersion: Group + "/" + Version,
			Kind:       obj.GetKind(),
			Namespace:  obj.GetNamespace(),
			Name:       obj.GetName(),
			UID:        obj.GetUID(),
		},
		Reason:         reason,
		Message:        message,
		Type:           typ,
		Source:         corev1.EventSource{Component: managedByValue},
		FirstTimestamp: now,
		LastTimestamp:  now,
		Count:          1,
	}
	if err := c.Create(ctx, ev); err != nil {
		log.FromContext(ctx).Error(err, "create event failed", "reason", reason)
	}
}

// phase derives the overall phase from the conditions.
func (s *instanceStatus) phase() string {
	allOK := true
	waitingForServer := false
	for _, typ := range conditionOrder {
		c := s.Conditions[typ]
		if c == nil || c.Status {
			continue
		}
		allOK = false
		if c.Reason == reasonServerNotReady {
			waitingForServer = true
		}
	}
	switch {
	case allOK:
		return PhaseReady
	case waitingForServer && s.Phase != PhaseReady && s.Phase != PhaseDegraded:
		return PhaseConfiguring
	default:
		return PhaseDegraded
	}
}

func loadStatus(inst *unstructured.Unstructured) *instanceStatus {
	st := &instanceStatus{Conditions: map[string]*condition{}}
	st.Phase, _, _ = unstructured.NestedString(inst.Object, "status", "phase")
	st.CurrentKeyID, _, _ = unstructured.NestedString(inst.Object, "status", "currentKeyId")
	st.RotateNowHandled, _, _ = unstructured.NestedString(inst.Object, "status", "rotateNowHandled")
	st.ServerVersion, _, _ = unstructured.NestedString(inst.Object, "status", "serverVersion")
	if s, _, _ := unstructured.NestedString(inst.Object, "status", "lastRotationTime"); s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			st.LastRotationTime = t
		}
	}
	raw, _, _ := unstructured.NestedSlice(inst.Object, "status", "conditions")
	for _, item := range raw {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		c := &condition{}
		c.Type, _ = m["type"].(string)
		status, _ := m["status"].(string)
		c.Status = status == "True"
		c.Reason, _ = m["reason"].(string)
		c.Message, _ = m["message"].(string)
		c.LastTransitionTime, _ = m["lastTransitionTime"].(string)
		if c.Type != "" {
			st.Conditions[c.Type] = c
		}
	}
	return st
}

func (r *InstanceReconciler) writeStatus(ctx context.Context, inst *unstructured.Unstructured, st *instanceStatus) error {
	conds := make([]interface{}, 0, len(conditionOrder))
	for _, typ := range conditionOrder {
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
		"conditions":         conds,
	}
	if st.CurrentKeyID != "" {
		status["currentKeyId"] = st.CurrentKeyID
	}
	if !st.LastRotationTime.IsZero() {
		status["lastRotationTime"] = st.LastRotationTime.UTC().Format(time.RFC3339)
	}
	if st.RotateNowHandled != "" {
		status["rotateNowHandled"] = st.RotateNowHandled
	}
	if st.ServerVersion != "" {
		status["serverVersion"] = st.ServerVersion
	}
	orig := inst.DeepCopy()
	if err := unstructured.SetNestedField(inst.Object, status, "status"); err != nil {
		return fmt.Errorf("build status: %w", err)
	}
	if err := r.Client.Status().Patch(ctx, inst, client.MergeFrom(orig)); err != nil {
		return fmt.Errorf("patch status: %w", err)
	}
	return nil
}
