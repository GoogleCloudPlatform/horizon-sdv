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
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPickByName(t *testing.T) {
	tests := []struct {
		name  string
		items []named[string]
		want  string
		got   string
		ok    bool
	}{
		{"exact among several", []named[string]{{"bench-10", "10"}, {"bench-1", "1"}}, "bench-1", "1", true},
		{"case insensitive", []named[string]{{"Alice@Example.com", "7"}}, "alice@example.com", "7", true},
		{"loose match only", []named[string]{{"bench-10", "10"}}, "bench-1", "", false},
		{"names not reported, single result", []named[string]{{"", "5"}}, "x", "5", true},
		{"names not reported, several results", []named[string]{{"", "5"}, {"", "6"}}, "x", "", false},
		{"empty", nil, "x", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := pickByName(tc.items, tc.want)
			if got != tc.got || ok != tc.ok {
				t.Fatalf("pickByName = %q, %v; want %q, %v", got, ok, tc.got, tc.ok)
			}
		})
	}
}

func TestFindUserIDIgnoresLooseMatches(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"id":"u2","username":"mtk-connect-admin-2"}]}`))
	}))
	defer srv.Close()
	_, err := New(srv.URL, "u", "k", srv.Client()).FindUserID(context.Background(), "mtk-connect-admin")
	if !errors.Is(err, ErrUserNotFound) {
		t.Fatalf("want ErrUserNotFound, got %v", err)
	}
}

func TestDeleteRegistrationOnlyDeletesTheNamedOne(t *testing.T) {
	var deleted []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/registrations":
			_, _ = w.Write([]byte(`{"data":[{"id":"r2","name":"hz-abc-extra"},{"id":"r1","name":"hz-abc"}]}`))
		case r.Method == http.MethodDelete:
			deleted = append(deleted, r.URL.Path)
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	c := New(srv.URL, "u", "k", srv.Client())
	if err := c.DeleteRegistration(context.Background(), "hz-abc"); err != nil {
		t.Fatal(err)
	}
	if len(deleted) != 1 || deleted[0] != "/api/v1/registrations/r1" {
		t.Fatalf("deleted %v, want only r1", deleted)
	}
	deleted = nil
	if err := c.DeleteRegistration(context.Background(), "hz-ab"); err != nil || len(deleted) != 0 {
		t.Fatalf("a name with no exact match must delete nothing: err=%v deleted=%v", err, deleted)
	}
}

func TestGrantGroupRejectsLooseMatch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"data":[{"id":"g9","name":"everyone-admins"}]}`))
			return
		}
		t.Errorf("unexpected %s %s: a group that does not match must not be granted", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	if err := New(srv.URL, "u", "k", srv.Client()).GrantGroupByName(context.Background(), "a1", "everyone", "book"); err == nil {
		t.Fatal("want an error for a group that is not an exact match")
	}
}

func TestRegistrationAlreadyExists(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"transport", errors.New("boom"), false},
		{"conflict", &APIError{Status: http.StatusConflict}, true},
		{"already exists", &APIError{Status: http.StatusUnprocessableEntity, Body: `{"title":"name already exists"}`}, true},
		{"duplicate key", &APIError{Status: http.StatusBadRequest, Body: "Duplicate key"}, true},
		{"does not exist", &APIError{Status: http.StatusBadRequest, Body: "agent does not exist"}, false},
		{"doesn't exist", &APIError{Status: http.StatusUnprocessableEntity, Body: "registration doesn't exist"}, false},
		{"required property", &APIError{Status: http.StatusUnprocessableEntity, Body: "must have required property 'token'"}, false},
		{"server error", &APIError{Status: http.StatusInternalServerError, Body: "already"}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := registrationAlreadyExists(tc.err); got != tc.want {
				t.Fatalf("registrationAlreadyExists = %v, want %v", got, tc.want)
			}
		})
	}
}

// A server may echo the request body in an error; the registration token must not reach the error text.
func TestRedactSecret(t *testing.T) {
	const token = "0b6f2c9e-1111-4222-8333-444455556666"
	err := &APIError{Method: "POST", Path: "/registrations", Status: 422, Body: `{"message":"invalid token ` + token + `"}`}
	got := redactSecret(err, token)
	if strings.Contains(got.Error(), token) {
		t.Fatalf("token leaked: %v", got)
	}
	var apiErr *APIError
	if !errors.As(got, &apiErr) || apiErr.Status != 422 {
		t.Fatalf("redaction must keep the API error: %v", got)
	}
	if !strings.Contains(err.Body, token) {
		t.Fatal("the original error must not be modified")
	}
	other := errors.New("boom")
	if redactSecret(other, token) != other || redactSecret(nil, token) != nil {
		t.Fatal("errors without the secret must pass through unchanged")
	}
}