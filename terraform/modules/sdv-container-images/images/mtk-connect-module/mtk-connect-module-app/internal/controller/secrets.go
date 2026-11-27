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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"reflect"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	secretKeyUsername = "username"
	secretKeyPassword = "password"
	secretKeyPrivate  = "privateKey"
	secretKeyIDPCert  = "idpCert"

	jenkinsCredentialsTypeLabel        = "jenkins.io/credentials-type"
	jenkinsCredentialsDescriptionKey   = "jenkins.io/credentials-description"
	jenkinsCredentialsDescriptionValue = "MTK Connect APIKEY"
)

// hashStrings returns a stable hex digest of the given values (length-prefixed, so ("ab","c") != ("a","bc")).
func hashStrings(values ...string) string {
	h := sha256.New()
	for _, v := range values {
		fmt.Fprintf(h, "%d:%s;", len(v), v)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// readSecretValue returns one key of a Secret; (value, found, error). A missing Secret or key is not an error.
func readSecretValue(ctx context.Context, c client.Reader, namespace, name, key string) (string, bool, error) {
	var s corev1.Secret
	if err := c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, &s); err != nil {
		if apierrors.IsNotFound(err) {
			return "", false, nil
		}
		return "", false, err
	}
	v, ok := s.Data[key]
	if !ok || len(v) == 0 {
		return "", false, nil
	}
	return string(v), true, nil
}

// desiredSecret describes a Secret the controller owns the content of.
type desiredSecret struct {
	Namespace   string
	Name        string
	Data        map[string][]byte
	Labels      map[string]string
	Annotations map[string]string
}

// applySecret creates the Secret or updates it when data, labels or annotations differ.
// Labels and annotations are merged, so values set by other parties are preserved.
// It returns true when the Secret was created or changed.
func applySecret(ctx context.Context, c client.Client, d desiredSecret) (bool, error) {
	labels := map[string]string{managedByLabel: managedByValue}
	for k, v := range d.Labels {
		labels[k] = v
	}
	var cur corev1.Secret
	err := c.Get(ctx, client.ObjectKey{Namespace: d.Namespace, Name: d.Name}, &cur)
	if apierrors.IsNotFound(err) {
		s := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:   d.Namespace,
				Name:        d.Name,
				Labels:      labels,
				Annotations: copyMap(d.Annotations),
			},
			Type: corev1.SecretTypeOpaque,
			Data: d.Data,
		}
		if err := c.Create(ctx, s); err != nil {
			if apierrors.IsAlreadyExists(err) {
				return false, fmt.Errorf("secret %s/%s appeared concurrently, retrying", d.Namespace, d.Name)
			}
			return false, fmt.Errorf("create secret %s/%s: %w", d.Namespace, d.Name, err)
		}
		return true, nil
	}
	if err != nil {
		return false, fmt.Errorf("get secret %s/%s: %w", d.Namespace, d.Name, err)
	}

	changed := false
	if !reflect.DeepEqual(normalizeData(cur.Data), normalizeData(d.Data)) {
		cur.Data = d.Data
		cur.StringData = nil
		changed = true
	}
	if mergeInto(&cur.Labels, labels) {
		changed = true
	}
	if mergeInto(&cur.Annotations, d.Annotations) {
		changed = true
	}
	if !changed {
		return false, nil
	}
	if err := c.Update(ctx, &cur); err != nil {
		return false, fmt.Errorf("update secret %s/%s: %w", d.Namespace, d.Name, err)
	}
	return true, nil
}

func normalizeData(in map[string][]byte) map[string]string {
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = string(v)
	}
	return out
}

func copyMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// mergeInto sets every key of src in *dst; it reports whether *dst changed.
func mergeInto(dst *map[string]string, src map[string]string) bool {
	changed := false
	for k, v := range src {
		if *dst == nil {
			*dst = map[string]string{}
		}
		if cur, ok := (*dst)[k]; !ok || cur != v {
			(*dst)[k] = v
			changed = true
		}
	}
	return changed
}

// credentialSecrets returns the Secrets that carry the service-account credentials: the server's own
// Secret first, then every configured distribution target. Targets that collide with an earlier
// (namespace, name) are dropped.
func credentialSecrets(namespace string, spec *InstanceSpec, password string) []desiredSecret {
	data := func() map[string][]byte {
		return map[string][]byte{
			secretKeyUsername: []byte(spec.ServiceAccountUser),
			secretKeyPassword: []byte(password),
		}
	}
	out := []desiredSecret{{Namespace: namespace, Name: spec.Server.APIKeySecret, Data: data()}}
	seen := map[string]bool{namespace + "/" + spec.Server.APIKeySecret: true}
	for _, t := range spec.CredentialDistribution {
		id := t.Namespace + "/" + t.SecretName
		if seen[id] {
			continue
		}
		seen[id] = true
		d := desiredSecret{
			Namespace:   t.Namespace,
			Name:        t.SecretName,
			Data:        data(),
			Labels:      copyMap(t.Labels),
			Annotations: copyMap(t.Annotations),
		}
		if t.JenkinsCredential {
			if d.Labels == nil {
				d.Labels = map[string]string{}
			}
			if d.Annotations == nil {
				d.Annotations = map[string]string{}
			}
			d.Labels[jenkinsCredentialsTypeLabel] = "usernamePassword"
			d.Annotations[jenkinsCredentialsDescriptionKey] = jenkinsCredentialsDescriptionValue
		}
		out = append(out, d)
	}
	return out
}
