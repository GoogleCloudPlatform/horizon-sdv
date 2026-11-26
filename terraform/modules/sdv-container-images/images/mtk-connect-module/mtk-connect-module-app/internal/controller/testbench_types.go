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
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	// KindTestbench is the kind of the testbench custom resource.
	KindTestbench = "MTKConnectTestbench"

	// FinalizerAgentCleanup makes the controller delete the MTK Connect agent before the resource goes away.
	FinalizerAgentCleanup = "mtkconnect.horizon-sdv.io/agent-cleanup"

	// AnnotationSkipCleanup lets a testbench be deleted while MTK Connect cannot be reached because the
	// MTKConnectInstance no longer exists. The agent and its registration are left behind. The Jenkins
	// credential is still deleted. The host key is revoked when the API can be reached, and otherwise
	// stays until credentialTTL.
	AnnotationSkipCleanup = "mtkconnect.horizon-sdv.io/skip-agent-cleanup"

	// registrationPrefix marks the agents this controller created.
	registrationPrefix = "hz-"
	registrationIDLen  = 16

	defaultPermission = "book"
	everyoneGroup     = "everyone"

	// defaultCredentialTTL is how long a host credential stays valid when the spec omits credentialTTL.
	defaultCredentialTTL = "6h"
	// maxCredentialTTL is the longest credentialTTL the controller accepts. The admission policy uses the same cap.
	maxCredentialTTL = 24 * time.Hour

	// LabelSubmittedFrom records which integration created the testbench (developer-portal, horizon-cli, api, custom).
	LabelSubmittedFrom = "horizon-sdv.io/submitted-from"
)

// TestbenchGVK identifies MTKConnectTestbench.
var TestbenchGVK = schema.GroupVersionKind{Group: Group, Version: Version, Kind: KindTestbench}

// Testbench phases reported in status.phase.
const (
	TbPending          = "Pending"
	TbAgentProvisioned = "AgentProvisioned"
	TbAwaitingAgent    = "AwaitingAgent"
	TbReady            = "Ready"
	TbOffline          = "Offline"
	TbDeleting         = "Deleting"
	TbFailed           = "Failed"
)

// Testbench condition types.
const (
	TbCondSpecValid      = "SpecValid"
	TbCondInstanceReady  = "InstanceReady"
	TbCondAgentProvision = "AgentProvisioned"
	TbCondAccessGranted  = "AccessGranted"
	TbCondHostCredential = "HostCredential"
	TbCondAgentOnline    = "AgentOnline"
)

var testbenchConditionOrder = []string{
	TbCondSpecValid, TbCondInstanceReady, TbCondAgentProvision, TbCondAccessGranted, TbCondHostCredential, TbCondAgentOnline,
}

var testbenchPhases = []string{
	TbPending, TbAgentProvisioned, TbAwaitingAgent, TbReady, TbOffline, TbDeleting, TbFailed,
}

// TestbenchAccess says who may book the testbench.
type TestbenchAccess struct {
	Public     bool     `json:"public,omitempty"`
	Users      []string `json:"users,omitempty"`
	Permission string   `json:"permission,omitempty"`
}

// TestbenchSpec is the spec of MTKConnectTestbench. It is the agent record only, and it is immutable once
// created. Devices are their own resource. credentialTTL is how long the host credential stays valid.
type TestbenchSpec struct {
	DisplayName   string          `json:"displayName,omitempty"`
	Access        TestbenchAccess `json:"access,omitempty"`
	CredentialTTL string          `json:"credentialTTL,omitempty"`
}

func (s *TestbenchSpec) applyDefaults(name string) {
	s.DisplayName = orDefault(s.DisplayName, name)
	s.CredentialTTL = orDefault(s.CredentialTTL, defaultCredentialTTL)
	s.Access.Permission = orDefault(s.Access.Permission, defaultPermission)
	users := make([]string, 0, len(s.Access.Users))
	seen := map[string]bool{}
	for _, u := range s.Access.Users {
		u = strings.TrimSpace(u)
		if u == "" || seen[u] {
			continue
		}
		seen[u] = true
		users = append(users, u)
	}
	s.Access.Users = users
}

func (s *TestbenchSpec) validate() error {
	if !s.Access.Public && len(s.Access.Users) == 0 {
		return fmt.Errorf("spec.access: set public to true or list at least one user")
	}
	if _, err := credentialTTL(s.CredentialTTL); err != nil {
		return err
	}
	return nil
}

// parseTestbenchSpec reads, defaults and validates the spec of an MTKConnectTestbench.
func parseTestbenchSpec(obj *unstructured.Unstructured) (*TestbenchSpec, error) {
	raw, found, err := unstructured.NestedMap(obj.Object, "spec")
	if err != nil {
		return nil, fmt.Errorf("read spec: %w", err)
	}
	if !found {
		return nil, fmt.Errorf("spec is missing")
	}
	var spec TestbenchSpec
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(raw, &spec); err != nil {
		return nil, fmt.Errorf("decode spec: %w", err)
	}
	spec.applyDefaults(obj.GetName())
	if err := spec.validate(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(obj.GetLabels()[LabelSubmittedFrom]) == "" {
		return nil, fmt.Errorf("label %s is required", LabelSubmittedFrom)
	}
	return &spec, nil
}

// credentialTTL parses spec.credentialTTL. An empty value is the default. Longer than maxCredentialTTL is rejected.
func credentialTTL(raw string) (time.Duration, error) {
	d, err := time.ParseDuration(orDefault(raw, defaultCredentialTTL))
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("spec.credentialTTL %q is not a positive duration", raw)
	}
	if d > maxCredentialTTL {
		return 0, fmt.Errorf("spec.credentialTTL %q is longer than %s", raw, maxCredentialTTL)
	}
	return d, nil
}

// registrationFor returns the registration name of the testbench's agent. It is derived from the resource's
// UID, so it is the same on every reconcile and after a controller restart: an agent created just before a
// crash is found again by its registration instead of being created twice.
func registrationFor(namespace, name, uid string) string {
	id := uid
	if id == "" {
		id = namespace + "/" + name
	}
	sum := sha256.Sum256([]byte(id))
	return registrationPrefix + hex.EncodeToString(sum[:])[:registrationIDLen]
}

// uuidPattern is the format MTK Connect requires for a registration token.
var uuidPattern = regexp.MustCompile(`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// newRegistrationToken returns a random version-4 UUID, the format MTK Connect requires for a registration
// token. The token is a credential (whoever has it can attach an agent to the testbench), so it comes from
// crypto/rand and is never derived from anything an observer can read, such as the resource UID.
func newRegistrationToken() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate registration token: %w", err)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	h := hex.EncodeToString(b[:])
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32], nil
}

// defaultURLPath is the MTK Connect portal. The portal's deep link to a single testbench is not documented,
// so the default is the portal root; set TESTBENCH_URL_PATH to a path containing {id} to link to the testbench.
const defaultURLPath = "/mtk-connect/"

// testbenchURL is the address reported in status.url.
func testbenchURL(domain, path, agentID string) string {
	path = orDefault(path, defaultURLPath)
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return "https://" + domain + strings.ReplaceAll(path, "{id}", agentID)
}
