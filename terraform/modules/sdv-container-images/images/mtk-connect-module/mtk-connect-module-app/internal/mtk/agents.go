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

package mtk

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// Agent is a testbench (an MTK Connect agent record) as returned by the REST API.
type Agent struct {
	ID           string
	Name         string
	Registration string
	Online       bool
}

type rawAgent struct {
	ID           flexString        `json:"id"`
	Name         string            `json:"name"`
	Registration registrationField `json:"registration"`
	Online       bool              `json:"online"`
}

func (r rawAgent) agent() Agent {
	return Agent{ID: string(r.ID), Name: r.Name, Registration: r.Registration.Name, Online: r.Online}
}

// registrationField accepts the registration as a name or as an object. The create call sends a name,
// and the server answers 422 until that registration exists; the record it returns may still be an object.
type registrationField struct {
	Name string
}

func (f *registrationField) UnmarshalJSON(b []byte) error {
	b = bytes.TrimSpace(b)
	if len(b) == 0 || string(b) == "null" {
		return nil
	}
	if b[0] == '"' {
		return json.Unmarshal(b, &f.Name)
	}
	var obj struct {
		Name string     `json:"name"`
		ID   flexString `json:"id"`
	}
	if err := json.Unmarshal(b, &obj); err != nil {
		return err
	}
	f.Name = obj.Name
	if f.Name == "" {
		f.Name = string(obj.ID)
	}
	return nil
}

func query(v map[string]string) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return url.QueryEscape(string(b)), nil
}

// FindAgent returns the agent with the given registration, or nil when there is none. More than one
// match is an error: the registration is meant to identify exactly one agent.
func (c *Client) FindAgent(ctx context.Context, registration string) (*Agent, error) {
	q, err := query(map[string]string{"registration": registration})
	if err != nil {
		return nil, err
	}
	var out struct {
		Data []rawAgent `json:"data"`
	}
	if err := c.do(ctx, http.MethodGet, "/agents?q="+q, nil, &out); err != nil {
		return nil, err
	}
	switch len(out.Data) {
	case 0:
		return nil, nil
	case 1:
		a := out.Data[0].agent()
		if a.ID == "" {
			return nil, errors.New("mtk-connect agent has no id")
		}
		return &a, nil
	default:
		return nil, fmt.Errorf("mtk-connect: %d agents share registration %q", len(out.Data), registration)
	}
}

// EnsureRegistration creates the registration an agent record has to point at, if it does not exist yet.
// POST /agents answers 422 "registration not found" for a name no agent process has registered. The
// existing setup script only sends a name the agent binary already registered; the controller creates
// that record itself so the testbench can exist before anything connects.
func (c *Client) EnsureRegistration(ctx context.Context, name, token string) error {
	q, err := query(map[string]string{"name": name})
	if err != nil {
		return err
	}
	var found struct {
		Data []struct {
			Name string `json:"name"`
		} `json:"data"`
	}
	if err := c.do(ctx, http.MethodGet, "/registrations?q="+q, nil, &found); err != nil {
		return err
	}
	// The name filter is not reliable: the server can return other registrations. Only an exact name
	// means this registration already exists. Any other row must still be created.
	for _, reg := range found.Data {
		if reg.Name == name {
			return nil
		}
	}
	err = c.do(ctx, http.MethodPost, "/registrations", map[string]string{"name": name, "token": token}, nil)
	if registrationAlreadyExists(err) {
		return nil
	}
	return redactSecret(err, token)
}

// redactSecret removes secret from the body of an APIError. A server may echo the request in an error
// answer, and the error text ends up in status conditions and Events, which more people can read than the
// Secret the token is kept in.
func redactSecret(err error, secret string) error {
	var apiErr *APIError
	if secret == "" || !errors.As(err, &apiErr) || !strings.Contains(apiErr.Body, secret) {
		return err
	}
	clean := *apiErr
	clean.Body = strings.ReplaceAll(apiErr.Body, secret, "<redacted>")
	return &clean
}

// registrationAlreadyExists reports a create that lost a race with an existing registration.
// A schema rejection such as "required property" is not that, and must still be returned.
func registrationAlreadyExists(err error) bool {
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	if apiErr.Status == http.StatusConflict {
		return true
	}
	if apiErr.Status != http.StatusUnprocessableEntity && apiErr.Status != http.StatusBadRequest {
		return false
	}
	body := strings.ToLower(apiErr.Body)
	// "does not exist", "doesn't exist" and "not found" mean the opposite of a lost race.
	for _, negative := range []string{"required property", "not found", "not exist", "does not", "doesn't"} {
		if strings.Contains(body, negative) {
			return false
		}
	}
	return strings.Contains(body, "already") || strings.Contains(body, "exists") || strings.Contains(body, "duplicate") || strings.Contains(body, "unique")
}

// DeleteRegistration removes the registration created for an agent. A missing registration, or a server
// that does not allow the delete, does not fail the caller: the testbench must still be able to go away.
func (c *Client) DeleteRegistration(ctx context.Context, name string) error {
	q, err := query(map[string]string{"name": name})
	if err != nil {
		return err
	}
	var found struct {
		Data []struct {
			ID   flexString `json:"id"`
			Name string     `json:"name"`
		} `json:"data"`
	}
	if err := c.do(ctx, http.MethodGet, "/registrations?q="+q, nil, &found); err != nil {
		return err
	}
	items := make([]named[flexString], 0, len(found.Data))
	for _, r := range found.Data {
		if r.ID != "" {
			items = append(items, named[flexString]{name: r.Name, val: r.ID})
		}
	}
	id, ok := pickByName(items, name)
	if !ok {
		return nil // nothing registered under exactly this name
	}
	err = c.do(ctx, http.MethodDelete, "/registrations/"+url.PathEscape(string(id)), nil, nil)
	var apiErr *APIError
	if errors.As(err, &apiErr) && (apiErr.Status == http.StatusNotFound || apiErr.Status == http.StatusMethodNotAllowed) {
		return nil
	}
	return err
}

// CreateAgent creates an agent record with a chosen registration name.
func (c *Client) CreateAgent(ctx context.Context, name, registration string) (Agent, error) {
	var out struct {
		Data rawAgent `json:"data"`
	}
	body := map[string]string{"name": name, "registration": registration}
	if err := c.do(ctx, http.MethodPost, "/agents", body, &out); err != nil {
		return Agent{}, err
	}
	a := out.Data.agent()
	if a.ID == "" {
		return Agent{}, errors.New("mtk-connect create agent: response has no id")
	}
	if a.Registration == "" {
		a.Registration = registration
	}
	return a, nil
}

// DeleteAgent deletes an agent and, with it, its devices. An agent that is already gone is not an error.
func (c *Client) DeleteAgent(ctx context.Context, id string) error {
	err := c.do(ctx, http.MethodDelete, "/agents/"+url.PathEscape(id), nil, nil)
	var apiErr *APIError
	if errors.As(err, &apiErr) && apiErr.Status == http.StatusNotFound {
		return nil
	}
	return err
}

// listedID is one element of a group or user list response. The identifier stays raw JSON so it can be
// sent back with the type (string or number) the server uses.
type listedID struct {
	ID       json.RawMessage `json:"id"`
	Name     string          `json:"name"`
	Username string          `json:"username"`
}

// idByName returns the identifier of the element called want. See pickByName for how a loose server
// match is handled.
func idByName(data []listedID, want string) (json.RawMessage, bool) {
	items := make([]named[json.RawMessage], 0, len(data))
	for _, d := range data {
		if len(d.ID) == 0 || string(d.ID) == "null" {
			continue
		}
		name := d.Name
		if name == "" {
			name = d.Username
		}
		items = append(items, named[json.RawMessage]{name: name, val: d.ID})
	}
	return pickByName(items, want)
}

// GrantGroupByName gives the named group the permission on the agent.
func (c *Client) GrantGroupByName(ctx context.Context, agentID, group, permission string) error {
	q, err := query(map[string]string{"name": group})
	if err != nil {
		return err
	}
	var out struct {
		Data []listedID `json:"data"`
	}
	if err := c.do(ctx, http.MethodGet, "/groups?fields=id,name&q="+q, nil, &out); err != nil {
		return err
	}
	id, ok := idByName(out.Data, group)
	if !ok {
		return fmt.Errorf("mtk-connect group %q not found", group)
	}
	body := map[string]interface{}{"group": map[string]interface{}{"id": id}, "permission": permission}
	return c.do(ctx, http.MethodPut, "/agents/"+url.PathEscape(agentID)+"/permissions/group", body, nil)
}

// GrantUserByName gives the named user the permission on the agent. A SAML user that does not exist yet
// is created, so a user who has never logged in to MTK Connect can still be granted access.
func (c *Client) GrantUserByName(ctx context.Context, agentID, username, permission string) error {
	q, err := query(map[string]string{"username": username})
	if err != nil {
		return err
	}
	var found struct {
		Data []listedID `json:"data"`
	}
	if err := c.do(ctx, http.MethodGet, "/users?fields=id,username&q="+q, nil, &found); err != nil {
		return err
	}
	id, ok := idByName(found.Data, username)
	if !ok {
		var created struct {
			Data struct {
				ID json.RawMessage `json:"id"`
			} `json:"data"`
		}
		body := map[string]string{"username": username, "authentication": "saml"}
		if err := c.do(ctx, http.MethodPost, "/users", body, &created); err != nil {
			return err
		}
		if len(created.Data.ID) == 0 || string(created.Data.ID) == "null" {
			return fmt.Errorf("mtk-connect create user %q: response has no id", username)
		}
		id = created.Data.ID
	}
	body := map[string]interface{}{"user": map[string]interface{}{"id": id}, "permission": permission}
	return c.do(ctx, http.MethodPut, "/agents/"+url.PathEscape(agentID)+"/permissions/user", body, nil)
}
