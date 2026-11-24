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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAgentLifecycle(t *testing.T) {
	var created map[string]string
	deleted := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/agents":
			switch r.URL.Query().Get("q") {
			case `{"registration":"hz-one"}`:
				_, _ = w.Write([]byte(`{"data":[{"id":"a1","name":"bench","registration":"hz-one","online":true}]}`))
			case `{"registration":"hz-dup"}`:
				_, _ = w.Write([]byte(`{"data":[{"id":"a1"},{"id":"a2"}]}`))
			default:
				_, _ = w.Write([]byte(`{"data":[]}`))
			}
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/agents":
			_ = json.NewDecoder(r.Body).Decode(&created)
			_, _ = w.Write([]byte(`{"data":{"id":42,"name":"n"}}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/agents/a1":
			deleted = "a1"
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	ctx := context.Background()
	c := New(srv.URL, "svc", "key", srv.Client())

	a, err := c.FindAgent(ctx, "hz-one")
	if err != nil || a == nil || a.ID != "a1" || !a.Online || a.Name != "bench" {
		t.Fatalf("FindAgent = %+v, %v", a, err)
	}
	if a, err := c.FindAgent(ctx, "hz-none"); err != nil || a != nil {
		t.Fatalf("FindAgent(none) = %+v, %v", a, err)
	}
	if _, err := c.FindAgent(ctx, "hz-dup"); err == nil {
		t.Fatal("FindAgent with two matches must fail")
	}

	got, err := c.CreateAgent(ctx, "bench", "hz-new")
	if err != nil || got.ID != "42" || got.Registration != "hz-new" {
		t.Fatalf("CreateAgent = %+v, %v", got, err)
	}
	if created["name"] != "bench" || created["registration"] != "hz-new" {
		t.Fatalf("create body = %v", created)
	}

	if err := c.DeleteAgent(ctx, "a1"); err != nil || deleted != "a1" {
		t.Fatalf("DeleteAgent: err=%v deleted=%q", err, deleted)
	}
	if err := c.DeleteAgent(ctx, "gone"); err != nil {
		t.Fatalf("DeleteAgent of a missing agent must succeed: %v", err)
	}
}

func TestGrantGroupAndUser(t *testing.T) {
	var groupBody, userBody map[string]interface{}
	createdUser := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/groups":
			if r.URL.Query().Get("q") != `{"name":"everyone"}` {
				_, _ = w.Write([]byte(`{"data":[]}`))
				return
			}
			_, _ = w.Write([]byte(`{"data":[{"id":"g1","name":"everyone"}]}`))
		case r.Method == http.MethodPut && r.URL.Path == "/api/v1/agents/a1/permissions/group":
			_ = json.NewDecoder(r.Body).Decode(&groupBody)
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/users":
			if r.URL.Query().Get("q") == `{"username":"known"}` {
				_, _ = w.Write([]byte(`{"data":[{"id":7,"username":"known"}]}`))
				return
			}
			_, _ = w.Write([]byte(`{"data":[]}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/users":
			createdUser = true
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["authentication"] != "saml" || body["username"] != "new" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"data":{"id":"u9"}}`))
		case r.Method == http.MethodPut && r.URL.Path == "/api/v1/agents/a1/permissions/user":
			_ = json.NewDecoder(r.Body).Decode(&userBody)
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	ctx := context.Background()
	c := New(srv.URL, "svc", "key", srv.Client())

	if err := c.GrantGroupByName(ctx, "a1", "everyone", "book"); err != nil {
		t.Fatalf("GrantGroupByName: %v", err)
	}
	if g, _ := groupBody["group"].(map[string]interface{}); g["id"] != "g1" || groupBody["permission"] != "book" {
		t.Fatalf("group body = %v", groupBody)
	}
	if err := c.GrantGroupByName(ctx, "a1", "missing", "book"); err == nil {
		t.Fatal("unknown group must fail")
	}

	// An existing user keeps its numeric id as a number.
	if err := c.GrantUserByName(ctx, "a1", "known", "book"); err != nil {
		t.Fatalf("GrantUserByName(known): %v", err)
	}
	if u, _ := userBody["user"].(map[string]interface{}); u["id"] != float64(7) || createdUser {
		t.Fatalf("user body = %v, created=%v", userBody, createdUser)
	}

	// An unknown user is created first.
	if err := c.GrantUserByName(ctx, "a1", "new", "book"); err != nil {
		t.Fatalf("GrantUserByName(new): %v", err)
	}
	if u, _ := userBody["user"].(map[string]interface{}); u["id"] != "u9" || !createdUser {
		t.Fatalf("user body = %v, created=%v", userBody, createdUser)
	}
}

func TestEnsureRegistration(t *testing.T) {
	names := map[string]bool{"already": true}
	var posted []map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/registrations":
			name := map[string]string{}
			_ = json.Unmarshal([]byte(r.URL.Query().Get("q")), &name)
			if names[name["name"]] {
				_, _ = w.Write([]byte(`{"data":[{"name":"` + name["name"] + `"}]}`))
				return
			}
			_, _ = w.Write([]byte(`{"data":[]}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/registrations":
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["token"] == "" {
				w.WriteHeader(http.StatusUnprocessableEntity)
				_, _ = w.Write([]byte(`{"errors":[{"status":"422","detail":"must have required property 'token'"}]}`))
				return
			}
			switch body["name"] {
			case "taken":
				w.WriteHeader(http.StatusConflict)
				return
			case "dup":
				w.WriteHeader(http.StatusUnprocessableEntity)
				_, _ = w.Write([]byte(`{"errors":[{"title":"name already exists"}]}`))
				return
			}
			names[body["name"]] = true
			posted = append(posted, body)
			w.WriteHeader(http.StatusCreated)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	ctx := context.Background()
	c := New(srv.URL, "svc", "key", srv.Client())
	if err := c.EnsureRegistration(ctx, "already", "tok-already"); err != nil {
		t.Fatal(err)
	}
	if err := c.EnsureRegistration(ctx, "hz-new", "tok-new"); err != nil {
		t.Fatal(err)
	}
	if err := c.EnsureRegistration(ctx, "hz-new", "tok-new"); err != nil {
		t.Fatal(err)
	}
	if err := c.EnsureRegistration(ctx, "taken", "tok-taken"); err != nil {
		t.Fatal(err)
	}
	if err := c.EnsureRegistration(ctx, "dup", "tok-dup"); err != nil {
		t.Fatal(err)
	}
	if err := c.EnsureRegistration(ctx, "notoken", ""); err == nil {
		t.Fatal("a registration without a token must be rejected")
	}
	if len(posted) != 1 || posted[0]["name"] != "hz-new" || posted[0]["token"] != "tok-new" {
		t.Fatalf("posted %v, want one create of hz-new with its token", posted)
	}
}

func TestAgentRegistrationObjectDecodes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"id":9,"name":"bench","registration":{"id":3,"name":"hz-obj"},"online":false}]}`))
	}))
	defer srv.Close()
	got, err := New(srv.URL, "svc", "key", srv.Client()).FindAgent(context.Background(), "hz-obj")
	if err != nil || got == nil || got.ID != "9" || got.Registration != "hz-obj" {
		t.Fatalf("FindAgent = %+v, %v", got, err)
	}
}
