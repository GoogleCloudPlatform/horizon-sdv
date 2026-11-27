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

// Package controller reconciles the MTKConnectInstance custom resource.
package controller

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	// Group is the API group of the MTK Connect custom resources.
	Group = "mtkconnect.horizon-sdv.io"
	// Version is the API version of the MTK Connect custom resources.
	Version = "v1alpha1"
	// KindInstance is the kind of the singleton integration resource.
	KindInstance = "MTKConnectInstance"

	// AnnotationRotateNow forces an API key rotation when set to any non-empty value.
	AnnotationRotateNow = "mtkconnect.horizon-sdv.io/rotate-now"

	annotationKeycloakHash = "horizon-sdv.io/keycloak-secret-hash"
	managedByLabel         = "app.kubernetes.io/managed-by"
	managedByValue         = "mtk-connect-module"
)

// InstanceGVK identifies MTKConnectInstance.
var InstanceGVK = schema.GroupVersionKind{Group: Group, Version: Version, Kind: KindInstance}

// SecretRef points at a key of a Secret.
type SecretRef struct {
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name,omitempty"`
	Key       string `json:"key,omitempty"`
}

// KeycloakSpec configures the Keycloak SAML client.
type KeycloakSpec struct {
	URL            string    `json:"url,omitempty"`
	Realm          string    `json:"realm,omitempty"`
	AdminUser      string    `json:"adminUser,omitempty"`
	AdminSecretRef SecretRef `json:"adminSecretRef,omitempty"`
	SAMLClientID   string    `json:"samlClientId,omitempty"`
}

// ServerSpec describes the MTK Connect server the controller integrates with.
type ServerSpec struct {
	URL            string `json:"url,omitempty"`
	Deployment     string `json:"deployment,omitempty"`
	PodSelector    string `json:"podSelector,omitempty"`
	Container      string `json:"container,omitempty"`
	KeycloakSecret string `json:"keycloakSecret,omitempty"`
	APIKeySecret   string `json:"apiKeySecret,omitempty"`
}

// KeyRotationSpec controls API key rotation. Values are Go durations.
type KeyRotationSpec struct {
	Interval    string `json:"interval,omitempty"`
	KeyLifetime string `json:"keyLifetime,omitempty"`
	Overlap     string `json:"overlap,omitempty"`
}

// CredentialTarget is a Secret that receives the current service-account credentials.
type CredentialTarget struct {
	Namespace         string            `json:"namespace,omitempty"`
	SecretName        string            `json:"secretName,omitempty"`
	JenkinsCredential bool              `json:"jenkinsCredential,omitempty"`
	Labels            map[string]string `json:"labels,omitempty"`
	Annotations       map[string]string `json:"annotations,omitempty"`
}

// InstanceSpec is the spec of MTKConnectInstance.
type InstanceSpec struct {
	Domain                 string             `json:"domain,omitempty"`
	ServiceAccountUser     string             `json:"serviceAccountUser,omitempty"`
	Keycloak               KeycloakSpec       `json:"keycloak,omitempty"`
	Server                 ServerSpec         `json:"server,omitempty"`
	KeyRotation            KeyRotationSpec    `json:"keyRotation,omitempty"`
	CredentialDistribution []CredentialTarget `json:"credentialDistribution,omitempty"`
}

// Timings are the parsed key rotation durations.
type Timings struct {
	Interval    time.Duration
	KeyLifetime time.Duration
	Overlap     time.Duration
}

func orDefault(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return strings.TrimSpace(v)
}

// The names of the server Deployment and of the two Secrets in the module namespace are fixed: the module's
// Role (operator/templates/rbac.yaml) allows get/update/patch on exactly these names, so any other value
// would only fail later with a 403. validate() rejects a spec that sets a different one.
const (
	serverDeploymentName = "mtk-connect"
	keycloakSecretName   = "mtk-connect-keycloak"
	apiKeySecretName     = "mtk-connect-apikey"
)

func (s *InstanceSpec) applyDefaults() {
	s.ServiceAccountUser = orDefault(s.ServiceAccountUser, "mtk-connect-admin")
	s.Keycloak.Realm = orDefault(s.Keycloak.Realm, "horizon")
	s.Keycloak.AdminUser = orDefault(s.Keycloak.AdminUser, "admin")
	s.Keycloak.SAMLClientID = orDefault(s.Keycloak.SAMLClientID, "mtk-connect")
	s.Keycloak.AdminSecretRef.Key = orDefault(s.Keycloak.AdminSecretRef.Key, "password")
	s.Server.Deployment = orDefault(s.Server.Deployment, serverDeploymentName)
	s.Server.PodSelector = orDefault(s.Server.PodSelector, "app.kubernetes.io/name=mtk-connect")
	s.Server.Container = orDefault(s.Server.Container, "authenticator")
	s.Server.KeycloakSecret = orDefault(s.Server.KeycloakSecret, keycloakSecretName)
	s.Server.APIKeySecret = orDefault(s.Server.APIKeySecret, apiKeySecretName)
	s.KeyRotation.Interval = orDefault(s.KeyRotation.Interval, "24h")
	s.KeyRotation.KeyLifetime = orDefault(s.KeyRotation.KeyLifetime, "720h")
	s.KeyRotation.Overlap = orDefault(s.KeyRotation.Overlap, "48h")
	s.Domain = strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(s.Domain), "https://"), "http://")
}

var (
	// clusterServiceHost is <service>.<namespace>.svc or <service>.<namespace>.svc.cluster.local.
	clusterServiceHost = regexp.MustCompile(`^([a-z0-9]([-a-z0-9]*[a-z0-9])?)\.([a-z0-9]([-a-z0-9]*[a-z0-9])?)\.svc(\.cluster\.local)?$`)
	// domainName is a host name with an optional port; no scheme, path, user info or whitespace.
	domainName = regexp.MustCompile(`^[A-Za-z0-9]([-A-Za-z0-9]*[A-Za-z0-9])?(\.[A-Za-z0-9]([-A-Za-z0-9]*[A-Za-z0-9])?)*(:[0-9]{1,5})?$`)
)

// clusterServiceURL checks that raw is an http(s) URL of an in-cluster Service and returns the Service's
// namespace. The controller sends credentials to these URLs (the Keycloak admin password, the MTK Connect
// API key), so a URL that leaves the cluster, or carries user info, is refused.
func clusterServiceURL(field, raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("%s is not a valid URL: %w", field, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("%s must start with http:// or https://", field)
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("%s must not contain user info, a query or a fragment", field)
	}
	m := clusterServiceHost.FindStringSubmatch(u.Hostname())
	if m == nil {
		return "", fmt.Errorf("%s must be an in-cluster Service address (<service>.<namespace>.svc or <service>.<namespace>.svc.cluster.local), got host %q", field, u.Hostname())
	}
	return m[3], nil
}

func (s *InstanceSpec) validate() (Timings, error) {
	var t Timings
	if s.Domain == "" {
		return t, fmt.Errorf("spec.domain is required")
	}
	if !domainName.MatchString(s.Domain) {
		return t, fmt.Errorf("spec.domain %q must be a host name, without a scheme, path or user info", s.Domain)
	}
	if s.Keycloak.URL == "" {
		return t, fmt.Errorf("spec.keycloak.url is required")
	}
	if s.Keycloak.AdminSecretRef.Namespace == "" || s.Keycloak.AdminSecretRef.Name == "" {
		return t, fmt.Errorf("spec.keycloak.adminSecretRef.namespace and name are required")
	}
	kcNamespace, err := clusterServiceURL("spec.keycloak.url", s.Keycloak.URL)
	if err != nil {
		return t, err
	}
	// The admin password is read from this namespace and sent to this URL: they must be the same Keycloak.
	if kcNamespace != s.Keycloak.AdminSecretRef.Namespace {
		return t, fmt.Errorf("spec.keycloak.url is in namespace %q but spec.keycloak.adminSecretRef is in %q", kcNamespace, s.Keycloak.AdminSecretRef.Namespace)
	}
	if s.Server.URL == "" {
		return t, fmt.Errorf("spec.server.url is required")
	}
	if _, err := clusterServiceURL("spec.server.url", s.Server.URL); err != nil {
		return t, err
	}
	for _, f := range []struct{ field, got, want string }{
		{"spec.server.deployment", s.Server.Deployment, serverDeploymentName},
		{"spec.server.keycloakSecret", s.Server.KeycloakSecret, keycloakSecretName},
		{"spec.server.apiKeySecret", s.Server.APIKeySecret, apiKeySecretName},
	} {
		if f.got != f.want {
			return t, fmt.Errorf("%s must be %q: the controller's Role only allows that name", f.field, f.want)
		}
	}
	for i, tgt := range s.CredentialDistribution {
		if tgt.Namespace == "" || tgt.SecretName == "" {
			return t, fmt.Errorf("spec.credentialDistribution[%d]: namespace and secretName are required", i)
		}
	}
	if t.Interval, err = time.ParseDuration(s.KeyRotation.Interval); err != nil || t.Interval <= 0 {
		return t, fmt.Errorf("spec.keyRotation.interval %q is not a positive duration", s.KeyRotation.Interval)
	}
	if t.KeyLifetime, err = time.ParseDuration(s.KeyRotation.KeyLifetime); err != nil || t.KeyLifetime <= 0 {
		return t, fmt.Errorf("spec.keyRotation.keyLifetime %q is not a positive duration", s.KeyRotation.KeyLifetime)
	}
	if t.Overlap, err = time.ParseDuration(s.KeyRotation.Overlap); err != nil || t.Overlap < 0 {
		return t, fmt.Errorf("spec.keyRotation.overlap %q is not a duration", s.KeyRotation.Overlap)
	}
	if t.KeyLifetime <= t.Interval {
		return t, fmt.Errorf("spec.keyRotation.keyLifetime must be longer than interval")
	}
	return t, nil
}

// parseSpec reads, defaults and validates the spec of an MTKConnectInstance.
func parseSpec(obj *unstructured.Unstructured) (*InstanceSpec, Timings, error) {
	raw, found, err := unstructured.NestedMap(obj.Object, "spec")
	if err != nil {
		return nil, Timings{}, fmt.Errorf("read spec: %w", err)
	}
	if !found {
		return nil, Timings{}, fmt.Errorf("spec is missing")
	}
	var spec InstanceSpec
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(raw, &spec); err != nil {
		return nil, Timings{}, fmt.Errorf("decode spec: %w", err)
	}
	spec.applyDefaults()
	t, err := spec.validate()
	if err != nil {
		return nil, Timings{}, err
	}
	return &spec, t, nil
}
