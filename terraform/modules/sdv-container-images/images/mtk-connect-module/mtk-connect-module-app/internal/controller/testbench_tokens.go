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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// testbenchTokenSecret is the Secret, in the module namespace, that holds the registration token of every
// testbench: one key per registration name, the value a random UUID. The module's Role limits get and update
// to this name (operator/templates/rbac.yaml); keep the two in step.
const testbenchTokenSecret = "mtk-connect-testbench-tokens"

// registrationToken returns the token stored for a registration and creates a random one first when there is
// none. The token is generated once and then read from the Secret, so every retry sends the token the server
// already stored, and it is not derivable from the resource.
func (r *TestbenchReconciler) registrationToken(ctx context.Context, registration string) (string, error) {
	var s corev1.Secret
	key := client.ObjectKey{Namespace: r.ManagerNamespace, Name: testbenchTokenSecret}
	err := r.Direct.Get(ctx, key, &s)
	missing := apierrors.IsNotFound(err)
	if err != nil && !missing {
		return "", fmt.Errorf("read secret %s/%s: %w", key.Namespace, key.Name, err)
	}
	if err == nil {
		if tok := string(s.Data[registration]); uuidPattern.MatchString(tok) {
			return tok, nil
		}
	}

	tok, err := newRegistrationToken()
	if err != nil {
		return "", err
	}
	if missing {
		s = corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: key.Namespace,
				Name:      key.Name,
				Labels:    map[string]string{managedByLabel: managedByValue},
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{registration: []byte(tok)},
		}
		if err := r.Direct.Create(ctx, &s); err != nil {
			// AlreadyExists means another reconcile created it first; the next one reads that token.
			return "", fmt.Errorf("store registration token in %s/%s: %w", key.Namespace, key.Name, err)
		}
		return tok, nil
	}
	if s.Data == nil {
		s.Data = map[string][]byte{}
	}
	s.Data[registration] = []byte(tok)
	if err := r.Direct.Update(ctx, &s); err != nil {
		return "", fmt.Errorf("store registration token in %s/%s: %w", key.Namespace, key.Name, err)
	}
	return tok, nil
}

// forgetRegistrationToken removes the token of a registration that no longer exists.
func (r *TestbenchReconciler) forgetRegistrationToken(ctx context.Context, registration string) error {
	var s corev1.Secret
	key := client.ObjectKey{Namespace: r.ManagerNamespace, Name: testbenchTokenSecret}
	if err := r.Direct.Get(ctx, key, &s); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("read secret %s/%s: %w", key.Namespace, key.Name, err)
	}
	if _, ok := s.Data[registration]; !ok {
		return nil
	}
	delete(s.Data, registration)
	if err := r.Direct.Update(ctx, &s); err != nil {
		return fmt.Errorf("remove registration token from %s/%s: %w", key.Namespace, key.Name, err)
	}
	return nil
}
