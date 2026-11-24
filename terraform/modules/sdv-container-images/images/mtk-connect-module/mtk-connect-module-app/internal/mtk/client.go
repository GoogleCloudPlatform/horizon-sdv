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

// Package mtk is a small client for the MTK Connect REST API (users and API keys).
package mtk

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

// Client calls the MTK Connect REST API with HTTP basic auth (service-account user and API key).
type Client struct {
	baseURL  string
	username string
	key      string
	http     *http.Client
}

// New returns a client. serverURL is the in-cluster router URL (for example
// http://mtk-connect-router.ns.svc.cluster.local:8081). The gateway strips the public /mtk-connect
// prefix before it reaches the router, so the in-cluster URL has no sub-path.
func New(serverURL, username, key string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}
	return &Client{
		baseURL:  strings.TrimRight(serverURL, "/") + "/api/v1",
		username: username,
		key:      key,
		http:     httpClient,
	}
}

// APIError is returned for non-2xx responses.
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
	return fmt.Sprintf("mtk-connect %s %s: HTTP %d: %s", e.Method, e.Path, e.Status, strings.TrimSpace(body))
}

// IsUnauthorized reports whether err is an HTTP 401 or 403 from MTK Connect.
func IsUnauthorized(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.Status == http.StatusUnauthorized || apiErr.Status == http.StatusForbidden
	}
	return false
}

// Key is one API key of a user as listed by MTK Connect.
type Key struct {
	ID      string
	Value   string // as returned by the list endpoint (may be truncated by the server)
	Created time.Time
	Expires time.Time
}

type flexString string

func (f *flexString) UnmarshalJSON(b []byte) error {
	s := strings.TrimSpace(string(b))
	if s == "null" {
		*f = ""
		return nil
	}
	if strings.HasPrefix(s, `"`) {
		var v string
		if err := json.Unmarshal(b, &v); err != nil {
			return err
		}
		*f = flexString(v)
		return nil
	}
	*f = flexString(s)
	return nil
}

type rawKey struct {
	ID           flexString `json:"id"`
	Key          string     `json:"key"`
	CreationTime string     `json:"creationTime"`
	ExpiryTime   string     `json:"expiryTime"`
}

func parseTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

func (c *Client) do(ctx context.Context, method, path string, in, out interface{}) error {
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
	req.SetBasicAuth(c.username, c.key)
	req.Header.Set("Accept", "application/json")
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("mtk-connect %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return fmt.Errorf("mtk-connect %s %s: read body: %w", method, path, err)
	}
	if resp.StatusCode/100 != 2 {
		return &APIError{Method: method, Path: path, Status: resp.StatusCode, Body: string(data)}
	}
	if out != nil && len(data) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return fmt.Errorf("mtk-connect %s %s: decode response: %w", method, path, err)
		}
	}
	return nil
}

// Version returns the MTK Connect server version string.
func (c *Client) Version(ctx context.Context) (string, error) {
	var out struct {
		Data json.RawMessage `json:"data"`
	}
	if err := c.do(ctx, http.MethodGet, "/config/version", nil, &out); err != nil {
		return "", err
	}
	var s string
	if json.Unmarshal(out.Data, &s) == nil {
		return s, nil
	}
	var m struct {
		Version string `json:"version"`
	}
	if json.Unmarshal(out.Data, &m) == nil && m.Version != "" {
		return m.Version, nil
	}
	v := strings.TrimSpace(string(out.Data))
	if len(v) > 64 {
		v = v[:64]
	}
	return v, nil
}

// ErrUserNotFound is returned by FindUserID when the server answers but has no such user. It is not a
// sign that the server is down: the service account simply does not exist (yet).
var ErrUserNotFound = errors.New("mtk-connect user not found")

// FindUserID returns the id of the user with the given username. Only a user whose name matches is
// returned; ErrUserNotFound is returned when there is none.
func (c *Client) FindUserID(ctx context.Context, username string) (string, error) {
	q, err := json.Marshal(map[string]string{"username": username})
	if err != nil {
		return "", err
	}
	var out struct {
		Data []struct {
			ID       flexString `json:"id"`
			Username string     `json:"username"`
		} `json:"data"`
	}
	if err := c.do(ctx, http.MethodGet, "/users?fields=id,username&q="+url.QueryEscape(string(q)), nil, &out); err != nil {
		return "", err
	}
	items := make([]named[string], 0, len(out.Data))
	for _, u := range out.Data {
		if u.ID != "" {
			items = append(items, named[string]{name: u.Username, val: string(u.ID)})
		}
	}
	id, ok := pickByName(items, username)
	if !ok {
		return "", fmt.Errorf("%w: %q", ErrUserNotFound, username)
	}
	return id, nil
}

// Keys lists the API keys of a user.
func (c *Client) Keys(ctx context.Context, userID string) ([]Key, error) {
	var out struct {
		Data struct {
			Keys []rawKey `json:"keys"`
		} `json:"data"`
	}
	if err := c.do(ctx, http.MethodGet, "/users/"+url.PathEscape(userID), nil, &out); err != nil {
		return nil, err
	}
	keys := make([]Key, 0, len(out.Data.Keys))
	for _, k := range out.Data.Keys {
		keys = append(keys, Key{
			ID:      string(k.ID),
			Value:   k.Key,
			Created: parseTime(k.CreationTime),
			Expires: parseTime(k.ExpiryTime),
		})
	}
	return keys, nil
}

// CreateKey creates an API key that expires at expiry and returns its value.
func (c *Client) CreateKey(ctx context.Context, userID string, expiry time.Time) (string, error) {
	body := map[string]string{
		"name":       "",
		"expiryTime": expiry.UTC().Format("2006-01-02T15:04:05Z"),
	}
	var out struct {
		Data struct {
			Key string `json:"key"`
		} `json:"data"`
	}
	if err := c.do(ctx, http.MethodPost, "/users/"+url.PathEscape(userID)+"/keys", body, &out); err != nil {
		return "", err
	}
	if out.Data.Key == "" {
		return "", errors.New("mtk-connect create key: response has no key")
	}
	return out.Data.Key, nil
}

// DeleteKey deletes one API key.
func (c *Client) DeleteKey(ctx context.Context, userID, keyID string) error {
	return c.do(ctx, http.MethodDelete, "/users/"+url.PathEscape(userID)+"/keys/"+url.PathEscape(keyID), nil, nil)
}
