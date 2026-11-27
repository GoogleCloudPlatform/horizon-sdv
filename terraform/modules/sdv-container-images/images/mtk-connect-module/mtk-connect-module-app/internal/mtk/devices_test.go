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

func TestDeviceLifecycle(t *testing.T) {
	var created map[string]string
	var patched map[string]interface{}
	deleted := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/devices":
			switch r.URL.Query().Get("q") {
			case `{"agent.registration":"hz-one","index":1}`:
				_, _ = w.Write([]byte(`{"data":[{"id":"d1","index":1,"name":"AAOS 1"}]}`))
			case `{"agent.registration":"hz-one","index":9}`:
				_, _ = w.Write([]byte(`{"data":[{"id":"x"},{"id":"y"}]}`))
			default:
				_, _ = w.Write([]byte(`{"data":[]}`))
			}
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/agents/a1/devices":
			_ = json.NewDecoder(r.Body).Decode(&created)
			_, _ = w.Write([]byte(`{"data":{"id":7,"index":2}}`))
		case r.Method == http.MethodPatch && r.URL.Path == "/api/v1/agents/a1/devices/2":
			_ = json.NewDecoder(r.Body).Decode(&patched)
			_, _ = w.Write([]byte(`{}`))
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/agents/a1/devices/2":
			deleted = "2"
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	ctx := context.Background()
	c := New(srv.URL, "svc", "key", srv.Client())

	d, err := c.FindDevice(ctx, "hz-one", 1)
	if err != nil || d == nil || d.ID != "d1" || d.Index != 1 {
		t.Fatalf("FindDevice = %+v, %v", d, err)
	}
	if d, err := c.FindDevice(ctx, "hz-one", 5); err != nil || d != nil {
		t.Fatalf("FindDevice(none) = %+v, %v", d, err)
	}
	if _, err := c.FindDevice(ctx, "hz-one", 9); err == nil {
		t.Fatal("FindDevice with two matches must fail")
	}

	got, err := c.CreateDevice(ctx, "a1", "AAOS 2")
	if err != nil || got.ID != "7" || got.Index != 2 || created["name"] != "AAOS 2" {
		t.Fatalf("CreateDevice = %+v, %v (body %v)", got, err, created)
	}

	payload := map[string]interface{}{"interface": map[string]interface{}{"fs": map[string]interface{}{}}}
	if err := c.ConfigureDevice(ctx, "a1", 2, payload); err != nil {
		t.Fatalf("ConfigureDevice: %v", err)
	}
	if _, ok := patched["interface"]; !ok {
		t.Fatalf("patch body = %v", patched)
	}

	if err := c.DeleteDevice(ctx, "a1", 2); err != nil || deleted != "2" {
		t.Fatalf("DeleteDevice: err=%v deleted=%q", err, deleted)
	}
	if err := c.DeleteDevice(ctx, "a1", 3); err != nil {
		t.Fatalf("DeleteDevice of a missing device must succeed: %v", err)
	}
}
