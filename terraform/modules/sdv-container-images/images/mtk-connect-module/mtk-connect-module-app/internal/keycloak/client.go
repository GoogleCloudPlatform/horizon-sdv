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

// Package keycloak configures the MTK Connect SAML client through the Keycloak Admin REST API.
// It replaces the keycloak-post-mtk-connect job (keycloak.mjs).
package keycloak

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const samlSigningKeyAttribute = "saml.signing.private.key"

// Client is a Keycloak Admin REST API client authenticated with the admin-cli password grant.
type Client struct {
	baseURL string
	http    *http.Client
	token   string
}

// New returns a client. baseURL includes the relative path (for example http://host:8080/auth).
func New(baseURL string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), http: httpClient}
}

// APIError is returned for unexpected HTTP statuses.
type APIError struct {
	Method string
	Path   string
	Status int
	Body   string
}

func (e *APIError) Error() string {
	body := e.Body
	if len(body) > 200 {
		body = body[:200]
	}
	return fmt.Sprintf("keycloak %s %s: HTTP %d: %s", e.Method, e.Path, e.Status, strings.TrimSpace(body))
}

// ProtocolMapper is a client protocol mapper that must exist.
type ProtocolMapper struct {
	Name            string
	Protocol        string
	ProtocolMapper  string
	ConsentRequired bool
	Config          map[string]string
}

// SAMLClientSpec is the desired state of the SAML client.
type SAMLClientSpec struct {
	ClientID     string
	AdminURL     string
	RedirectURIs []string
	Attributes   map[string]string
	Mappers      []ProtocolMapper
}

// Material is the PEM data the MTK Connect authenticator needs.
type Material struct {
	PrivateKey string
	IDPCert    string
}

// Login authenticates as a master-realm admin.
func (c *Client) Login(ctx context.Context, username, password string) error {
	form := url.Values{
		"client_id":  {"admin-cli"},
		"username":   {username},
		"password":   {password},
		"grant_type": {"password"},
	}
	const path = "/realms/master/protocol/openid-connect/token"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("keycloak login: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("keycloak login: read body: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return &APIError{Method: http.MethodPost, Path: path, Status: resp.StatusCode, Body: "login rejected"}
	}
	var out struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(data, &out); err != nil || out.AccessToken == "" {
		return errors.New("keycloak login: response has no access_token")
	}
	c.token = out.AccessToken
	return nil
}

func (c *Client) do(ctx context.Context, method, path string, in, out interface{}, ok ...int) error {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("keycloak %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return fmt.Errorf("keycloak %s %s: read body: %w", method, path, err)
	}
	accepted := false
	for _, s := range ok {
		if resp.StatusCode == s {
			accepted = true
		}
	}
	if !accepted {
		return &APIError{Method: method, Path: path, Status: resp.StatusCode, Body: string(data)}
	}
	if out != nil && len(data) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("keycloak %s %s: decode response: %w", method, path, err)
		}
	}
	return nil
}

func realmPath(realm string) string {
	return "/admin/realms/" + url.PathEscape(realm)
}

// RealmCertificate returns the PEM certificate of the realm's active RSA signing key.
func (c *Client) RealmCertificate(ctx context.Context, realm string) (string, error) {
	var out struct {
		Keys []struct {
			Type        string `json:"type"`
			Use         string `json:"use"`
			Status      string `json:"status"`
			Certificate string `json:"certificate"`
		} `json:"keys"`
	}
	if err := c.do(ctx, http.MethodGet, realmPath(realm)+"/keys", nil, &out, http.StatusOK); err != nil {
		return "", err
	}
	cert := ""
	for _, k := range out.Keys {
		// A passive or disabled key is not what Keycloak signs with, so it is never a fallback.
		if k.Type == "RSA" && k.Use == "SIG" && k.Status == "ACTIVE" && k.Certificate != "" {
			cert = k.Certificate
			break
		}
	}
	if cert == "" {
		return "", fmt.Errorf("realm %q has no active RSA signing certificate", realm)
	}
	return "-----BEGIN CERTIFICATE-----\n" + cert + "\n-----END CERTIFICATE-----", nil
}

// findClient returns the internal id and representation of a client, or ("", nil, nil) when absent.
func (c *Client) findClient(ctx context.Context, realm, clientID string) (string, map[string]interface{}, error) {
	var list []map[string]interface{}
	path := realmPath(realm) + "/clients?clientId=" + url.QueryEscape(clientID)
	if err := c.do(ctx, http.MethodGet, path, nil, &list, http.StatusOK); err != nil {
		return "", nil, err
	}
	for _, rep := range list {
		if id, _ := rep["clientId"].(string); id == clientID {
			internal, _ := rep["id"].(string)
			if internal == "" {
				return "", nil, fmt.Errorf("client %q has no id", clientID)
			}
			return internal, rep, nil
		}
	}
	return "", nil, nil
}

// mergeClient applies the desired fields onto an existing (or empty) client representation.
func mergeClient(existing map[string]interface{}, spec SAMLClientSpec) map[string]interface{} {
	out := existing
	if out == nil {
		out = map[string]interface{}{}
	}
	out["clientId"] = spec.ClientID
	out["adminUrl"] = spec.AdminURL
	uris := make([]interface{}, 0, len(spec.RedirectURIs))
	for _, u := range spec.RedirectURIs {
		uris = append(uris, u)
	}
	out["redirectUris"] = uris
	out["protocol"] = "saml"
	out["fullScopeAllowed"] = false
	attrs, _ := out["attributes"].(map[string]interface{})
	if attrs == nil {
		attrs = map[string]interface{}{}
	}
	for k, v := range spec.Attributes {
		attrs[k] = v
	}
	out["attributes"] = attrs
	return out
}

func canonical(m map[string]interface{}) string {
	b, err := json.Marshal(m)
	if err != nil {
		return ""
	}
	return string(b)
}

// EnsureSAMLClient creates or updates the SAML client and its protocol mappers and returns the
// signing key and realm certificate. It is idempotent: an unchanged client is not written.
func (c *Client) EnsureSAMLClient(ctx context.Context, realm string, spec SAMLClientSpec) (Material, error) {
	cert, err := c.RealmCertificate(ctx, realm)
	if err != nil {
		return Material{}, err
	}
	internalID, rep, err := c.findClient(ctx, realm, spec.ClientID)
	if err != nil {
		return Material{}, err
	}
	if rep == nil {
		if err := c.do(ctx, http.MethodPost, realmPath(realm)+"/clients", mergeClient(nil, spec), nil, http.StatusCreated); err != nil {
			return Material{}, err
		}
	} else {
		before := canonical(rep)
		desired := mergeClient(rep, spec)
		if canonical(desired) != before {
			path := realmPath(realm) + "/clients/" + url.PathEscape(internalID)
			if err := c.do(ctx, http.MethodPut, path, desired, nil, http.StatusNoContent, http.StatusOK); err != nil {
				return Material{}, err
			}
		}
	}
	// Re-read: a new client carries the signing key Keycloak generated.
	internalID, rep, err = c.findClient(ctx, realm, spec.ClientID)
	if err != nil {
		return Material{}, err
	}
	if rep == nil {
		return Material{}, fmt.Errorf("client %q not found after create", spec.ClientID)
	}
	if err := c.ensureMappers(ctx, realm, internalID, spec.Mappers); err != nil {
		return Material{}, err
	}
	attrs, _ := rep["attributes"].(map[string]interface{})
	key, _ := attrs[samlSigningKeyAttribute].(string)
	if key == "" {
		return Material{}, fmt.Errorf("client %q has no %s attribute", spec.ClientID, samlSigningKeyAttribute)
	}
	return Material{
		PrivateKey: "-----BEGIN RSA PRIVATE KEY-----\n" + key + "\n-----END RSA PRIVATE KEY-----",
		IDPCert:    cert,
	}, nil
}

func (c *Client) ensureMappers(ctx context.Context, realm, internalID string, want []ProtocolMapper) error {
	base := realmPath(realm) + "/clients/" + url.PathEscape(internalID) + "/protocol-mappers/models"
	var have []struct {
		Name string `json:"name"`
	}
	if err := c.do(ctx, http.MethodGet, base, nil, &have, http.StatusOK); err != nil {
		return err
	}
	existing := map[string]bool{}
	for _, m := range have {
		existing[m.Name] = true
	}
	for _, m := range want {
		if existing[m.Name] {
			continue
		}
		body := map[string]interface{}{
			"name":            m.Name,
			"protocol":        m.Protocol,
			"protocolMapper":  m.ProtocolMapper,
			"consentRequired": m.ConsentRequired,
			"config":          m.Config,
		}
		if err := c.do(ctx, http.MethodPost, base, body, nil, http.StatusCreated); err != nil {
			return err
		}
	}
	return nil
}
