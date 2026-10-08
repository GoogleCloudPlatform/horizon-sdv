// Copyright (c) 2026 Accenture, All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package catalog

import (
	"encoding/json"
	"testing"
)

func TestOpenAPIWorkflowsHistoryParameters(t *testing.T) {
	t.Parallel()
	b, err := OpenAPISpec(nil, RetentionOpenAPI{})
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Paths map[string]struct {
			Get *struct {
				OperationID string        `json:"operationId"`
				Parameters  []interface{} `json:"parameters"`
			} `json:"get"`
		} `json:"paths"`
		Info struct {
			Version string `json:"version"`
		} `json:"info"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	h := doc.Paths["/v1/workflows/history"].Get
	if h == nil {
		t.Fatal("missing GET /v1/workflows/history")
	}
	if h.OperationID != "listWorkflowsHistory" {
		t.Fatalf("operationId: got %q", h.OperationID)
	}
	want := []string{"limit", "continue", "phase", "startedAfter", "startedBefore", "finishedAfter", "finishedBefore", "nameGlob", "nameRegex"}
	if len(h.Parameters) != len(want) {
		t.Fatalf("parameters: got %d want %d", len(h.Parameters), len(want))
	}
	for i, name := range want {
		m, ok := h.Parameters[i].(map[string]interface{})
		if !ok {
			t.Fatalf("param %d: not an object", i)
		}
		if m["name"] != name {
			t.Fatalf("param %d name: got %v want %q", i, m["name"], name)
		}
	}
	r := doc.Paths["/v1/workflows/running"].Get
	if r == nil || r.OperationID != "listWorkflowsRunning" {
		t.Fatalf("running operationId: %+v", r)
	}
	if len(r.Parameters) != 1 {
		t.Fatalf("running should have only limit, got %d", len(r.Parameters))
	}
	if m, ok := r.Parameters[0].(map[string]interface{}); !ok || m["name"] != "limit" {
		t.Fatalf("running first param: %+v", r.Parameters[0])
	}
}

func TestOpenAPIWorkflowLogPath(t *testing.T) {
	t.Parallel()
	b, err := OpenAPISpec(nil, RetentionOpenAPI{})
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Paths map[string]interface{} `json:"paths"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	if _, ok := doc.Paths["/v1/workflows/{workflowName}/log"]; !ok {
		t.Fatal("missing GET /v1/workflows/{workflowName}/log")
	}
	if _, bad := doc.Paths["/v1/logs/{workflowName}"]; bad {
		t.Fatal("legacy /v1/logs/{workflowName} must not appear in OpenAPI")
	}
	lg := doc.Paths["/v1/workflows/{workflowName}/log"].(map[string]interface{})
	op := lg["get"].(map[string]interface{})
	if op["operationId"] != "streamWorkflowLogs" {
		t.Fatalf("log operationId: %v", op["operationId"])
	}
}

func TestOpenAPIArtifactsPaths(t *testing.T) {
	t.Parallel()
	b, err := OpenAPISpec(nil, RetentionOpenAPI{})
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Paths map[string]struct {
			Get *struct {
				OperationID string                   `json:"operationId"`
				Parameters  []map[string]interface{} `json:"parameters"`
				Responses   map[string]interface{}   `json:"responses"`
			} `json:"get"`
		} `json:"paths"`
		Components struct {
			Schemas map[string]interface{} `json:"schemas"`
		} `json:"components"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	buckets := doc.Paths["/v1/artifacts/buckets"].Get
	if buckets == nil || buckets.OperationID != "listArtifactsBuckets" {
		t.Fatalf("list buckets: %+v", doc.Paths["/v1/artifacts/buckets"].Get)
	}
	if _, ok := buckets.Responses["200"]; !ok {
		t.Fatal("list buckets missing 200")
	}
	objects := doc.Paths["/v1/artifacts/buckets/{bucket}/objects"].Get
	if objects == nil || objects.OperationID != "listArtifactsObjects" {
		t.Fatalf("list objects: %+v", doc.Paths["/v1/artifacts/buckets/{bucket}/objects"].Get)
	}
	want := []string{"bucket", "prefix", "pageToken", "limit"}
	if len(objects.Parameters) != len(want) {
		t.Fatalf("object parameters: got %d want %d", len(objects.Parameters), len(want))
	}
	for i, name := range want {
		if objects.Parameters[i]["name"] != name {
			t.Fatalf("param %d name: got %v want %q", i, objects.Parameters[i]["name"], name)
		}
	}
	for _, schema := range []string{"BucketInfo", "ListBucketsResponse", "ObjectInfo", "ListObjectsResponse"} {
		if _, ok := doc.Components.Schemas[schema]; !ok {
			t.Fatalf("missing schema %s", schema)
		}
	}
}

func TestOpenAPIDownloadArtifactPath(t *testing.T) {
	t.Parallel()
	b, err := OpenAPISpec(nil, RetentionOpenAPI{})
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Paths map[string]interface{} `json:"paths"`
		Info  struct {
			Version string `json:"version"`
		} `json:"info"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		t.Fatal(err)
	}
	const p = "/v1/workflows/{workflowName}/downloadArtifact/{artifactName}"
	if _, ok := doc.Paths[p]; !ok {
		t.Fatalf("missing path %s", p)
	}
	op := doc.Paths[p].(map[string]interface{})["get"].(map[string]interface{})
	if op["operationId"] != "downloadWorkflowArtifact" {
		t.Fatalf("operationId: %v", op["operationId"])
	}
	if doc.Info.Version != "0.3.5" {
		t.Fatalf("info.version: %q", doc.Info.Version)
	}
}
