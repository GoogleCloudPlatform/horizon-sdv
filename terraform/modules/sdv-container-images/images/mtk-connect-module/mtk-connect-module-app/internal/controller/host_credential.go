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
	"net/http"
	"reflect"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"mtk-connect-module/internal/mtk"
)

const (
	// agentUsername is the low-privilege MTK Connect user that owns one short-lived key per testbench.
	// The admin service account is never written into the host Secret.
	agentUsername = "mtk-connect-agent"

	// credentialRenewBefore is how long before expiry a still-valid key is replaced. It is shortened
	// to half the TTL when the TTL itself is shorter, so a key is not rotated on every reconcile.
	credentialRenewBefore = 15 * time.Minute

	hostSecretKeyID        = "keyId"
	hostSecretPreviousID   = "previousKeyId"
	hostSecretExpiry       = "expiry"
	hostSecretToken        = "token"
	hostSecretRegistration = "registration"

	// jenkinsCredentialLabel marks the copy the controller is allowed to delete. The Jenkins Role cannot
	// name every Secret in advance, so admission allows delete only when this label is already set.
	jenkinsCredentialLabel = "mtkconnect.horizon-sdv.io/jenkins-credential"
)

func hostCredentialSecretName(testbench string) string { return testbench + "-agent" }

// jenkinsCredentialSecretName is unique per testbench namespace. The credentials provider uses the
// Secret name as the credentials id, and two namespaces can contain a testbench of the same name.
func jenkinsCredentialSecretName(namespace, testbench string) string {
	return "mtkc-" + namespace + "-" + testbench + "-agent"
}

// ensureHostCredential writes Secret <testbench>-agent in the testbench namespace. The Secret holds one
// API key of agentUsername that expires after credentialTTL, plus the registration token for that
// testbench. It does not contain the admin key. A key that is still outside the renew window is left
// in place.
//
// Every key id is recorded in the testbench status before the key is written to the Secret, so it can be
// revoked even if the Secret is lost. A replaced key stays valid until the instance's keyRotation.overlap
// ends, or until it expires if that is sooner, because a host may still be using it; it is revoked then.
func (r *TestbenchReconciler) ensureHostCredential(ctx context.Context, tb *unstructured.Unstructured, st *testbenchStatus, api TestbenchAPI, registration, token, ttlRaw string, overlap time.Duration) (string, error) {
	ttl, err := credentialTTL(ttlRaw)
	if err != nil {
		return "", err
	}
	userID, err := api.EnsureUser(ctx, agentUsername)
	if err != nil {
		return "", fmt.Errorf("agent user: %w", err)
	}
	now := r.now()
	name := hostCredentialSecretName(tb.GetName())
	ref := tb.GetNamespace() + "/" + name

	var cur corev1.Secret
	getErr := r.Direct.Get(ctx, client.ObjectKey{Namespace: tb.GetNamespace(), Name: name}, &cur)
	if getErr != nil && !apierrors.IsNotFound(getErr) {
		return "", fmt.Errorf("read secret %s: %w", ref, getErr)
	}
	var stored string
	var storedExpiry time.Time
	if getErr == nil {
		stored = string(cur.Data[hostSecretKeyID])
		storedExpiry, _ = time.Parse(time.RFC3339, string(cur.Data[hostSecretExpiry]))
		// A Secret written before the ids were recorded on the testbench: adopt its key.
		if st.HostKeyID == "" && stored != "" {
			st.HostKeyID, st.HostKeyExpiry = stored, storedExpiry
		}
	}
	if err := revokeRetiredHostKeys(ctx, api, userID, st, now); err != nil {
		return "", err
	}
	if getErr == nil && stored != "" && stored == st.HostKeyID && hostCredentialCurrent(&cur, registration, token, now, ttl, st.HostKeyExpiry) {
		if err := r.ownHostSecret(ctx, tb, &cur); err != nil {
			return "", err
		}
		if err := r.publishJenkinsCredential(ctx, tb, string(cur.Data[secretKeyPassword])); err != nil {
			return "", err
		}
		return ref, nil
	}

	prevID, prevExpiry := st.HostKeyID, st.HostKeyExpiry
	prevRetired := append([]retiredHostKey(nil), st.RetiredHostKeys...)
	// A host may have loaded either key, even when the Secret no longer shows it.
	st.retireHostKey(st.HostKeyID, earliest(now.Add(overlap), st.HostKeyExpiry))
	if stored != st.HostKeyID {
		st.retireHostKey(stored, earliest(now.Add(overlap), storedExpiry))
	}

	expiry := now.Add(ttl)
	value, err := api.CreateKey(ctx, userID, expiry)
	if err != nil {
		return "", fmt.Errorf("create host key: %w", err)
	}
	keys, err := api.Keys(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("list host keys: %w", err)
	}
	created, ok := findCurrentKey(keys, value)
	if !ok || created.ID == "" {
		return "", fmt.Errorf("host key was created but cannot be identified for later revocation")
	}
	st.HostKeyID, st.HostKeyExpiry = created.ID, expiry
	if err := r.writeStatus(ctx, tb, st); err != nil {
		// Without the record the key could not be revoked later, so it is not handed out.
		st.HostKeyID, st.HostKeyExpiry, st.RetiredHostKeys = prevID, prevExpiry, prevRetired
		if delErr := deleteHostKey(ctx, api, userID, created.ID); delErr != nil {
			return "", fmt.Errorf("record host key: %w (also failed to revoke the unrecorded key: %v)", err, delErr)
		}
		return "", fmt.Errorf("record host key: %w", err)
	}

	data := map[string][]byte{
		secretKeyUsername:      []byte(agentUsername),
		secretKeyPassword:      []byte(value),
		hostSecretRegistration: []byte(registration),
		hostSecretToken:        []byte(token),
		hostSecretKeyID:        []byte(created.ID),
		hostSecretExpiry:       []byte(expiry.UTC().Format(time.RFC3339)),
	}
	if _, err := applySecret(ctx, r.Direct, desiredSecret{Namespace: tb.GetNamespace(), Name: name, Data: data}); err != nil {
		// The key is recorded, so it is retired by the next reconcile and revoked on delete.
		return "", err
	}
	if err := r.Direct.Get(ctx, client.ObjectKey{Namespace: tb.GetNamespace(), Name: name}, &cur); err != nil {
		return "", fmt.Errorf("read secret %s: %w", ref, err)
	}
	if err := r.ownHostSecret(ctx, tb, &cur); err != nil {
		return "", err
	}
	if err := r.publishJenkinsCredential(ctx, tb, value); err != nil {
		return "", err
	}
	return ref, nil
}

// publishJenkinsCredential writes the credential the Jenkins provider can see. That provider only lists
// Secrets in the Jenkins namespace, so a label on the host Secret in another namespace is never bound.
// The copy holds the mtk-connect-agent username and password only: not the admin key, and not the
// registration token. Its name includes the testbench namespace, because the credentials id is the
// Secret name and the testbench name alone is not unique. The host Secret is never labelled for
// Jenkins, including when the testbench itself is in the Jenkins namespace, so the token stays out of it.
func (r *TestbenchReconciler) publishJenkinsCredential(ctx context.Context, tb *unstructured.Unstructured, password string) error {
	ns := r.JenkinsNamespace
	if ns == "" || password == "" {
		return nil
	}
	name := jenkinsCredentialSecretName(tb.GetNamespace(), tb.GetName())
	labels := map[string]string{
		jenkinsCredentialsTypeLabel: "usernamePassword",
		jenkinsCredentialLabel:      "true",
	}
	annotations := map[string]string{jenkinsCredentialsDescriptionKey: "MTK Connect agent key for testbench " + tb.GetNamespace() + "/" + tb.GetName()}
	if _, err := applySecret(ctx, r.Direct, desiredSecret{
		Namespace:   ns,
		Name:        name,
		Data:        map[string][]byte{secretKeyUsername: []byte(agentUsername), secretKeyPassword: []byte(password)},
		Labels:      labels,
		Annotations: annotations,
	}); err != nil {
		return err
	}
	if ns != tb.GetNamespace() {
		return nil
	}
	var cur corev1.Secret
	if err := r.Direct.Get(ctx, client.ObjectKey{Namespace: ns, Name: name}, &cur); err != nil {
		return fmt.Errorf("read jenkins credential %s/%s: %w", ns, name, err)
	}
	return r.ownHostSecret(ctx, tb, &cur)
}

// deleteJenkinsCredential removes the Jenkins-namespace copy. The host Secret is garbage-collected with
// the testbench. This copy is a different object, and an owner reference cannot cross namespaces.
func (r *TestbenchReconciler) deleteJenkinsCredential(ctx context.Context, tb *unstructured.Unstructured) error {
	ns := r.JenkinsNamespace
	if ns == "" {
		return nil
	}
	name := jenkinsCredentialSecretName(tb.GetNamespace(), tb.GetName())
	err := r.Direct.Delete(ctx, &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
		Namespace: ns,
		Name:      name,
	}})
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("delete jenkins credential %s/%s: %w", ns, name, err)
	}
	return nil
}

// retireHostKey keeps id valid until revokeAfter. A key already retired keeps the earlier time.
func (st *testbenchStatus) retireHostKey(id string, revokeAfter time.Time) {
	if id == "" {
		return
	}
	for i := range st.RetiredHostKeys {
		if st.RetiredHostKeys[i].ID == id {
			st.RetiredHostKeys[i].RevokeAfter = earliest(st.RetiredHostKeys[i].RevokeAfter, revokeAfter)
			return
		}
	}
	st.RetiredHostKeys = append(st.RetiredHostKeys, retiredHostKey{ID: id, RevokeAfter: revokeAfter})
}

// revokeRetiredHostKeys revokes the retired keys whose overlap has ended.
func revokeRetiredHostKeys(ctx context.Context, api TestbenchAPI, userID string, st *testbenchStatus, now time.Time) error {
	kept := st.RetiredHostKeys[:0]
	for _, k := range st.RetiredHostKeys {
		if now.Before(k.RevokeAfter) {
			kept = append(kept, k)
			continue
		}
		if err := deleteHostKey(ctx, api, userID, k.ID); err != nil {
			return err
		}
	}
	st.RetiredHostKeys = kept
	return nil
}

// earliest returns the earlier of a and b, ignoring a zero time.
func earliest(a, b time.Time) time.Time {
	if a.IsZero() || (!b.IsZero() && b.Before(a)) {
		return b
	}
	return a
}

// hostCredentialCurrent reports whether the Secret already holds a usable key for this registration.
func hostCredentialCurrent(s *corev1.Secret, registration, token string, now time.Time, ttl time.Duration, expiry time.Time) bool {
	if string(s.Data[secretKeyUsername]) != agentUsername || len(s.Data[secretKeyPassword]) == 0 {
		return false
	}
	if string(s.Data[hostSecretRegistration]) != registration || string(s.Data[hostSecretToken]) != token {
		return false
	}
	if string(s.Data[hostSecretKeyID]) == "" || expiry.IsZero() {
		return false
	}
	window := credentialRenewBefore
	if half := ttl / 2; window > half {
		window = half
	}
	return now.Add(window).Before(expiry)
}

func (r *TestbenchReconciler) ownHostSecret(ctx context.Context, tb *unstructured.Unstructured, secret *corev1.Secret) error {
	before := append([]metav1.OwnerReference(nil), secret.OwnerReferences...)
	if err := controllerutil.SetControllerReference(tb, secret, r.Client.Scheme()); err != nil {
		return fmt.Errorf("own secret %s/%s: %w", secret.Namespace, secret.Name, err)
	}
	if reflect.DeepEqual(before, secret.OwnerReferences) {
		return nil
	}
	if err := r.Direct.Update(ctx, secret); err != nil {
		return fmt.Errorf("own secret %s/%s: %w", secret.Namespace, secret.Name, err)
	}
	return nil
}

// revokeHostCredential deletes every host key recorded on the testbench: the current one and the retired
// ones still in their overlap. Ids that only the host Secret lists (written before the record existed) are
// revoked too. A missing Secret does not stop the recorded keys from being revoked.
func (r *TestbenchReconciler) revokeHostCredential(ctx context.Context, api TestbenchAPI, tb *unstructured.Unstructured, st *testbenchStatus) error {
	ids := []string{st.HostKeyID}
	for _, k := range st.RetiredHostKeys {
		ids = append(ids, k.ID)
	}
	var secret corev1.Secret
	name := hostCredentialSecretName(tb.GetName())
	err := r.Direct.Get(ctx, client.ObjectKey{Namespace: tb.GetNamespace(), Name: name}, &secret)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("read secret %s/%s: %w", tb.GetNamespace(), name, err)
	}
	if err == nil {
		ids = append(ids, string(secret.Data[hostSecretKeyID]), string(secret.Data[hostSecretPreviousID]))
	}

	var userID string
	seen := map[string]bool{}
	for _, id := range ids {
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		if userID == "" {
			if userID, err = api.EnsureUser(ctx, agentUsername); err != nil {
				return fmt.Errorf("agent user: %w", err)
			}
		}
		if err := deleteHostKey(ctx, api, userID, id); err != nil {
			return err
		}
	}
	return nil
}

// deleteHostKey revokes one key. A key the server no longer has is already revoked.
func deleteHostKey(ctx context.Context, api TestbenchAPI, userID, keyID string) error {
	if keyID == "" {
		return nil
	}
	err := api.DeleteKey(ctx, userID, keyID)
	var apiErr *mtk.APIError
	if errors.As(err, &apiErr) && apiErr.Status == http.StatusNotFound {
		return nil
	}
	if err != nil {
		return fmt.Errorf("revoke host key %s: %w", keyID, err)
	}
	return nil
}
