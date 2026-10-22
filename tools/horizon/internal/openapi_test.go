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

package horizon

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sampleOpenAPI = `{
  "openapi": "3.0.0",
  "paths": {
    "/v1/b": {
      "get": {"operationId": "getB", "summary": "Get B", "tags": ["Items"]}
    },
    "/v1/items/{id}": {
      "parameters": [
        {"name": "id", "in": "path", "description": "old", "schema": {"type": "string"}}
      ],
      "get": {"operationId": "getItem", "summary": "Get item", "tags": ["Items"]},
      "post": {
        "operationId": "createItem",
        "summary": "Create item",
        "tags": ["Items"],
        "parameters": [
          {"name": "id", "in": "path", "description": "new", "schema": {"type": "string", "example": "item-1"}},
          {"name": "verbose", "in": "query", "schema": {"type": "boolean", "default": true}},
          {"name": "X-Trace", "in": "header", "schema": {"type": "string", "example": "trace"}}
        ],
        "requestBody": {
          "content": {
            "application/json": {"schema": {"$ref": "#/components/schemas/Item"}}
          }
        }
      },
      "delete": {"operationId": "deleteItem", "summary": "Delete item", "tags": ["Other"]}
    }
  },
  "components": {
    "schemas": {
      "Item": {
        "allOf": [
          {"type": "object", "properties": {"name": {"type": "string", "example": "widget"}}},
          {"type": "object", "properties": {
            "count": {"type": "integer"},
            "when": {"type": "string", "format": "date-time"},
            "ok": {"type": "boolean"},
            "tags": {"type": "array", "items": {"type": "string"}},
            "status": {"type": "string", "enum": ["new", "old"]},
            "child": {"$ref": "#/components/schemas/Item"}
          }}
        ]
      }
    }
  }
}`

func TestParseOpenAPI(t *testing.T) {
	doc, err := ParseOpenAPI([]byte(sampleOpenAPI))
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Operations) != 4 {
		t.Fatalf("ops = %d", len(doc.Operations))
	}
	// Paths sort alphabetically; methods follow GET, POST, ..., DELETE.
	wantOrder := []string{"GET /v1/b", "GET /v1/items/{id}", "POST /v1/items/{id}", "DELETE /v1/items/{id}"}
	var got []string
	for _, op := range doc.Operations {
		got = append(got, op.Method+" "+op.Path)
	}
	if strings.Join(got, ",") != strings.Join(wantOrder, ",") {
		t.Fatalf("order = %v", got)
	}

	op, err := FindOperation(doc.Operations, "POST", "v1/items/{id}", "")
	if err != nil {
		t.Fatal(err)
	}
	if op.OperationID != "createItem" {
		t.Fatalf("op = %+v", op)
	}
	var id OpParameter
	for _, p := range op.Parameters {
		if p.Name == "id" {
			id = p
		}
	}
	if id.Description != "new" || !id.Required || id.Example != "item-1" {
		t.Fatalf("id param = %+v", id)
	}

	ex := doc.exampleRequest(*op, "https://api.example/")
	if ex.URL != "https://api.example/v1/items/{id}" {
		t.Fatalf("url = %q", ex.URL)
	}
	if ex.PathParams["id"] != "item-1" || ex.Query["verbose"] != true || ex.Headers["X-Trace"] != "trace" {
		t.Fatalf("example = %+v", ex)
	}
	if ex.ContentType != "application/json" || ex.Headers["Content-Type"] != "application/json" {
		t.Fatalf("content type = %+v", ex)
	}
	body, ok := ex.Body.(map[string]any)
	if !ok {
		t.Fatalf("body = %#v", ex.Body)
	}
	if body["name"] != "widget" || body["count"] != 0 || body["when"] != "2026-01-01T00:00:00Z" || body["ok"] != false || body["status"] != "new" {
		t.Fatalf("body = %#v", body)
	}
	tags, ok := body["tags"].([]any)
	if !ok || len(tags) != 1 || tags[0] != "string" {
		t.Fatalf("tags = %#v", body["tags"])
	}
	child, ok := body["child"].(map[string]any)
	if !ok || len(child) != 0 {
		t.Fatalf("cycle child = %#v", body["child"])
	}
}

func TestParseOpenAPIErrors(t *testing.T) {
	cases := []string{
		`{`,
		`{"paths":{"/x":{"get":{"summary":"x"}}}}`,
		`{"openapi":"3.0.0","paths":{}}`,
		`{"openapi":"3.0.0","paths":{"x-internal":{"get":{"summary":"hidden"}},"/skip":{"x-foo":true}}}`,
	}
	for _, raw := range cases {
		if _, err := ParseOpenAPI([]byte(raw)); err == nil {
			t.Fatalf("expected error for %s", raw)
		}
	}
}

func TestFilterFormatAndRefs(t *testing.T) {
	doc, err := ParseOpenAPI([]byte(sampleOpenAPI))
	if err != nil {
		t.Fatal(err)
	}
	all := FilterOperations(doc.Operations, OperationListFilter{})
	if len(all) != len(doc.Operations) {
		t.Fatalf("unfiltered = %d", len(all))
	}
	got := FilterOperations(doc.Operations, OperationListFilter{Tag: "items", Method: "get", PathPrefix: "/v1/items"})
	if len(got) != 1 || got[0].OperationID != "getItem" {
		t.Fatalf("filtered = %+v", got)
	}
	if text := FormatOperationList(nil); text != "No matching operations.\n" {
		t.Fatalf("empty list = %q", text)
	}
	text := FormatOperationList(got)
	if !strings.Contains(text, "GET     /v1/items/{id}  Get item") {
		t.Fatalf("list = %q", text)
	}

	method, path, id, err := ParseOperationRef([]string{"get", "/v1/items/{id}"})
	if err != nil || method != "GET" || path != "/v1/items/{id}" || id != "" {
		t.Fatalf("args = %s %s %s %v", method, path, id, err)
	}
	method, path, id, err = ParseOperationRef([]string{"post v1/b"})
	if err != nil || method != "POST" || path != "/v1/b" || id != "" {
		t.Fatalf("combined = %s %s %s %v", method, path, id, err)
	}
	_, _, id, err = ParseOperationRef([]string{"createItem"})
	if err != nil || id != "createItem" {
		t.Fatalf("id = %q %v", id, err)
	}
	if _, _, _, err := ParseOperationRef(nil); err == nil {
		t.Fatal("expected missing ref")
	}
	if _, _, _, err := ParseOperationRef([]string{"nope", "/v1"}); err == nil {
		t.Fatal("expected bad method")
	}
	if normalizeAPIPath("C:/Program Files/Git/v1/catalog") != "C:/Program Files/Git/v1/catalog" {
		t.Fatal("drive path should be left unchanged")
	}
	if normalizeAPIPath("  ") != "" {
		t.Fatal("blank path")
	}

	found, err := FindOperation(doc.Operations, "", "", "CREATEITEM")
	if err != nil || found.OperationID != "createItem" {
		t.Fatalf("find id = %+v %v", found, err)
	}
	if _, err := FindOperation(doc.Operations, "PUT", "/missing", ""); err == nil {
		t.Fatal("expected unknown operation")
	}

	dup, err := ParseOpenAPI([]byte(`{"openapi":"3.0.0","paths":{"/a":{"get":{"operationId":"same"}},"/b":{"get":{"operationId":"same"}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := FindOperation(dup.Operations, "", "", "same"); err == nil || !strings.Contains(err.Error(), "matches 2") {
		t.Fatalf("dup = %v", err)
	}

	show := FormatOperationShow(doc, *opByID(t, doc, "createItem"), "https://api.example")
	if !strings.Contains(show, "POST /v1/items/{id}") || !strings.Contains(show, "operationId: createItem") || !strings.Contains(show, "widget") {
		t.Fatalf("show = %s", show)
	}
	raw, err := OperationShowJSON(doc, *opByID(t, doc, "getB"), "{api}")
	if err != nil || !json.Valid(raw) {
		t.Fatalf("show json = %s %v", raw, err)
	}
	listJSON, err := OperationsListJSON(got)
	if err != nil || !strings.Contains(string(listJSON), `"operationId": "getItem"`) {
		t.Fatalf("list json = %s %v", listJSON, err)
	}
	paths, err := PathsJSON([]string{"/v1/b"})
	if err != nil || strings.TrimSpace(string(paths)) == "" {
		t.Fatalf("paths = %s %v", paths, err)
	}
}

func opByID(t *testing.T, doc *OpenAPIDocument, id string) *Operation {
	t.Helper()
	op, err := FindOperation(doc.Operations, "", "", id)
	if err != nil {
		t.Fatal(err)
	}
	return op
}

func TestExampleFromSchemaShapes(t *testing.T) {
	components := map[string]any{}
	got := exampleFromSchema(map[string]any{
		"type": "object",
		"additionalProperties": map[string]any{
			"type": "number",
		},
	}, components, map[string]bool{})
	obj, ok := got.(map[string]any)
	if !ok || obj["additionalProp1"] != 0 {
		t.Fatalf("additional = %#v", got)
	}
	if exampleFromSchema(map[string]any{"$ref": "#/components/schemas/Missing"}, components, map[string]bool{}) == nil {
		t.Fatal("missing ref should still produce a placeholder")
	}
	if exampleFromSchema(nil, nil, nil) != nil {
		t.Fatal("nil schema")
	}
}

func TestExampleAPIBase(t *testing.T) {
	if ExampleAPIBase(nil) != "{api}" || ExampleAPIBase(&Config{}) != "{api}" {
		t.Fatal("empty base")
	}
	if ExampleAPIBase(&Config{BaseURL: "https://api.example/"}) != "https://api.example" {
		t.Fatal("trailing slash")
	}
}

func TestOpenAPICache(t *testing.T) {
	dir := isolateConfigEnv(t)
	doc, err := LoadOpenAPIDocument()
	if err != nil {
		t.Fatal(err)
	}
	if doc.Source != "embedded" {
		t.Fatalf("source = %q", doc.Source)
	}
	if _, err := ParseOpenAPI(EmbeddedOpenAPIJSON()); err != nil {
		t.Fatal(err)
	}
	found, err := FindOperation(doc.Operations, "GET", "/v1/catalog", "")
	if err != nil || found.OperationID != "listCatalog" {
		t.Fatalf("embedded catalog = %+v %v", found, err)
	}

	cachePath := filepath.Join(dir, "horizon", "openapi.json")
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cachePath, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	doc, err = LoadOpenAPIDocument()
	if err != nil || doc.Source != "embedded" {
		t.Fatalf("invalid cache source = %q %v", doc.Source, err)
	}

	if _, err := SaveOpenAPICache([]byte("{")); err == nil {
		t.Fatal("expected invalid cache error")
	}
	path, err := SaveOpenAPICache([]byte(sampleOpenAPI))
	if err != nil {
		t.Fatal(err)
	}
	if path != cachePath {
		t.Fatalf("path = %q", path)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "\n  ") || !strings.HasSuffix(string(b), "\n") {
		t.Fatal("cache should be pretty-printed")
	}
	doc, err = LoadOpenAPIDocument()
	if err != nil || !strings.HasPrefix(doc.Source, "cache:") {
		t.Fatalf("cache source = %q %v", doc.Source, err)
	}
}

func TestStringSliceAndRequestBodyFallback(t *testing.T) {
	if got := stringSlice([]any{" a ", "", "b"}); strings.Join(got, ",") != "a,b" {
		t.Fatalf("slice = %#v", got)
	}
	if stringSlice("nope") != nil {
		t.Fatal("non-array")
	}
	ct, schema := requestBodySchema(json.RawMessage(`{"content":{"text/plain":{"schema":{"type":"string"}}}}`))
	if ct != "text/plain" || schema["type"] != "string" {
		t.Fatalf("ct=%q schema=%v", ct, schema)
	}
	if ct, schema := requestBodySchema(json.RawMessage(`{`)); ct != "" || schema != nil {
		t.Fatalf("bad body schema %q %#v", ct, schema)
	}
}
