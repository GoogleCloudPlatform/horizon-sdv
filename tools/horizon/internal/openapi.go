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
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

//go:embed openapi_embed.json
var embeddedOpenAPI []byte

var openapiHTTPMethods = []string{"get", "post", "put", "patch", "delete", "head", "options", "trace"}

const openAPICacheFileName = "openapi.json"

// Operation is one OpenAPI path+method entry.
type Operation struct {
	Method      string          `json:"method"`
	Path        string          `json:"path"`
	OperationID string          `json:"operationId,omitempty"`
	Summary     string          `json:"summary,omitempty"`
	Description string          `json:"description,omitempty"`
	Tags        []string        `json:"tags,omitempty"`
	Parameters  []OpParameter   `json:"parameters,omitempty"`
	RequestBody json.RawMessage `json:"requestBody,omitempty"`
}

// OpParameter is a path, query, header, or cookie parameter.
type OpParameter struct {
	Name        string `json:"name"`
	In          string `json:"in"`
	Required    bool   `json:"required,omitempty"`
	Description string `json:"description,omitempty"`
	Type        string `json:"type,omitempty"`
	Default     any    `json:"default,omitempty"`
	Example     any    `json:"example,omitempty"`
}

// OperationListFilter selects operations for `horizon api list`.
type OperationListFilter struct {
	Tag        string
	Method     string
	PathPrefix string
}

// OpenAPIDocument is a parsed OpenAPI 3 document.
type OpenAPIDocument struct {
	Source     string
	Operations []Operation
	schemas    map[string]any
}

// EmbeddedOpenAPIJSON returns the bundled OpenAPI snapshot.
func EmbeddedOpenAPIJSON() []byte {
	return append([]byte(nil), embeddedOpenAPI...)
}

// HorizonOpenAPICachePath is ~/.config/horizon/openapi.json.
func HorizonOpenAPICachePath() (string, error) {
	dir, err := HorizonConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, openAPICacheFileName), nil
}

// LoadOpenAPIDocument prefers a refreshed cache, then the embedded snapshot.
func LoadOpenAPIDocument() (*OpenAPIDocument, error) {
	if path, err := HorizonOpenAPICachePath(); err == nil {
		b, err := os.ReadFile(path)
		if err == nil {
			doc, err := ParseOpenAPI(b)
			if err == nil {
				doc.Source = "cache:" + path
				return doc, nil
			}
			fmt.Fprintf(os.Stderr, "openapi cache %s is invalid (%v); using embedded spec\n", path, err)
		} else if !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "openapi cache: %v; using embedded spec\n", err)
		}
	}
	doc, err := ParseOpenAPI(embeddedOpenAPI)
	if err != nil {
		return nil, fmt.Errorf("embedded OpenAPI: %w", err)
	}
	doc.Source = "embedded"
	return doc, nil
}

// SaveOpenAPICache writes a live OpenAPI document after validating it parses.
func SaveOpenAPICache(raw []byte) (string, error) {
	if _, err := ParseOpenAPI(raw); err != nil {
		return "", fmt.Errorf("openapi.json is not a usable OpenAPI document: %w", err)
	}
	dir, err := HorizonConfigDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, openAPICacheFileName)
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, raw, "", "  "); err != nil {
		if err := os.WriteFile(path, raw, 0o644); err != nil {
			return "", err
		}
		return path, nil
	}
	pretty.WriteByte('\n')
	if err := os.WriteFile(path, pretty.Bytes(), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// RefreshOpenAPICache GETs {api}/openapi.json and stores it locally.
func RefreshOpenAPICache(ctx context.Context, c *Client) (string, error) {
	b, err := c.GetJSON(ctx, "/openapi.json")
	if err != nil {
		return "", err
	}
	return SaveOpenAPICache(b)
}

// ParseOpenAPI extracts operations from an OpenAPI 3 JSON document.
func ParseOpenAPI(raw []byte) (*OpenAPIDocument, error) {
	var root map[string]any
	if err := json.Unmarshal(raw, &root); err != nil {
		return nil, fmt.Errorf("parse OpenAPI JSON: %w", err)
	}
	ver := strings.TrimSpace(stringFromAny(root["openapi"]))
	if ver == "" {
		ver = strings.TrimSpace(stringFromAny(root["swagger"]))
	}
	if ver == "" {
		return nil, fmt.Errorf("missing openapi version")
	}
	paths, _ := root["paths"].(map[string]any)
	if len(paths) == 0 {
		return nil, fmt.Errorf("OpenAPI document has no paths")
	}
	schemas := map[string]any{}
	if comps, ok := root["components"].(map[string]any); ok {
		if s, ok := comps["schemas"].(map[string]any); ok {
			schemas = s
		}
	}
	doc := &OpenAPIDocument{schemas: schemas}
	for path, rawItem := range paths {
		item, ok := rawItem.(map[string]any)
		if !ok || strings.HasPrefix(path, "x-") {
			continue
		}
		shared := parseParameters(item["parameters"])
		for _, m := range openapiHTTPMethods {
			opRaw, ok := item[m].(map[string]any)
			if !ok {
				continue
			}
			op := Operation{
				Method:      strings.ToUpper(m),
				Path:        path,
				OperationID: strings.TrimSpace(stringFromAny(opRaw["operationId"])),
				Summary:     strings.TrimSpace(stringFromAny(opRaw["summary"])),
				Description: strings.TrimSpace(stringFromAny(opRaw["description"])),
				Tags:        stringSlice(opRaw["tags"]),
				Parameters:  mergeParameters(shared, parseParameters(opRaw["parameters"])),
			}
			if rb, ok := opRaw["requestBody"]; ok && rb != nil {
				if b, err := json.Marshal(rb); err == nil {
					op.RequestBody = b
				}
			}
			doc.Operations = append(doc.Operations, op)
		}
	}
	if len(doc.Operations) == 0 {
		return nil, fmt.Errorf("OpenAPI document has no operations")
	}
	sortOperations(doc.Operations)
	return doc, nil
}

func sortOperations(ops []Operation) {
	methodOrder := map[string]int{}
	for i, m := range openapiHTTPMethods {
		methodOrder[strings.ToUpper(m)] = i
	}
	sort.Slice(ops, func(i, j int) bool {
		if ops[i].Path != ops[j].Path {
			return ops[i].Path < ops[j].Path
		}
		oi, ok := methodOrder[ops[i].Method]
		if !ok {
			oi = 99
		}
		oj, ok := methodOrder[ops[j].Method]
		if !ok {
			oj = 99
		}
		if oi != oj {
			return oi < oj
		}
		return ops[i].Method < ops[j].Method
	})
}

func stringSlice(v any) []string {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, x := range arr {
		s := strings.TrimSpace(stringFromAny(x))
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func parseParameters(v any) []OpParameter {
	arr, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]OpParameter, 0, len(arr))
	for _, x := range arr {
		m, ok := x.(map[string]any)
		if !ok {
			continue
		}
		p := OpParameter{
			Name:        strings.TrimSpace(stringFromAny(m["name"])),
			In:          strings.TrimSpace(stringFromAny(m["in"])),
			Description: strings.TrimSpace(stringFromAny(m["description"])),
		}
		if p.Name == "" {
			continue
		}
		if b, ok := m["required"].(bool); ok {
			p.Required = b
		} else if strings.EqualFold(p.In, "path") {
			p.Required = true
		}
		schema, _ := m["schema"].(map[string]any)
		if schema != nil {
			p.Type = strings.TrimSpace(stringFromAny(schema["type"]))
			if d, ok := schema["default"]; ok {
				p.Default = d
			}
			if e, ok := schema["example"]; ok {
				p.Example = e
			}
		}
		if e, ok := m["example"]; ok && p.Example == nil {
			p.Example = e
		}
		out = append(out, p)
	}
	return out
}

func mergeParameters(pathItem, operation []OpParameter) []OpParameter {
	seen := make(map[string]int, len(pathItem)+len(operation))
	out := make([]OpParameter, 0, len(pathItem)+len(operation))
	add := func(p OpParameter) {
		key := strings.ToLower(p.In) + ":" + p.Name
		if i, ok := seen[key]; ok {
			out[i] = p
			return
		}
		seen[key] = len(out)
		out = append(out, p)
	}
	for _, p := range pathItem {
		add(p)
	}
	for _, p := range operation {
		add(p)
	}
	return out
}

// FilterOperations applies tag, method, and path-prefix filters.
func FilterOperations(ops []Operation, f OperationListFilter) []Operation {
	tag := strings.ToLower(strings.TrimSpace(f.Tag))
	method := strings.ToUpper(strings.TrimSpace(f.Method))
	prefix := strings.TrimSpace(f.PathPrefix)
	if tag == "" && method == "" && prefix == "" {
		return ops
	}
	out := make([]Operation, 0, len(ops))
	for _, op := range ops {
		if method != "" && op.Method != method {
			continue
		}
		if prefix != "" && !strings.HasPrefix(op.Path, prefix) {
			continue
		}
		if tag != "" {
			ok := false
			for _, t := range op.Tags {
				if strings.EqualFold(t, tag) {
					ok = true
					break
				}
			}
			if !ok {
				continue
			}
		}
		out = append(out, op)
	}
	return out
}

// FormatOperationList is the human-readable `horizon api list` table.
func FormatOperationList(ops []Operation) string {
	if len(ops) == 0 {
		return "No matching operations.\n"
	}
	var b strings.Builder
	for _, op := range ops {
		fmt.Fprintf(&b, "%-7s %s", op.Method, op.Path)
		if op.Summary != "" {
			fmt.Fprintf(&b, "  %s", op.Summary)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// OperationsListJSON is the machine-readable -j payload for `horizon api list`.
func OperationsListJSON(ops []Operation) ([]byte, error) {
	type row struct {
		Method      string   `json:"method"`
		Path        string   `json:"path"`
		OperationID string   `json:"operationId,omitempty"`
		Summary     string   `json:"summary,omitempty"`
		Tags        []string `json:"tags,omitempty"`
	}
	rows := make([]row, 0, len(ops))
	for _, op := range ops {
		rows = append(rows, row{
			Method:      op.Method,
			Path:        op.Path,
			OperationID: op.OperationID,
			Summary:     op.Summary,
			Tags:        op.Tags,
		})
	}
	return json.MarshalIndent(rows, "", "  ")
}

// PathsJSON is the machine-readable -j payload for `horizon api paths`.
func PathsJSON(paths []string) ([]byte, error) {
	return json.MarshalIndent(paths, "", "  ")
}

// ParseOperationRef accepts operationId, "METHOD PATH", or METHOD and PATH args.
func ParseOperationRef(args []string) (method, path, operationID string, err error) {
	if len(args) == 0 {
		return "", "", "", fmt.Errorf("missing operationId or METHOD PATH")
	}
	if len(args) >= 2 {
		m := strings.ToUpper(strings.TrimSpace(args[0]))
		p := strings.TrimSpace(strings.Join(args[1:], " "))
		if isHTTPMethod(m) {
			return m, normalizeAPIPath(p), "", nil
		}
		return "", "", "", fmt.Errorf("not a METHOD PATH: %q is not an HTTP method", args[0])
	}
	s := strings.TrimSpace(args[0])
	if parts := strings.Fields(s); len(parts) >= 2 && isHTTPMethod(parts[0]) {
		return strings.ToUpper(parts[0]), normalizeAPIPath(strings.Join(parts[1:], " ")), "", nil
	}
	return "", "", s, nil
}

func isHTTPMethod(s string) bool {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "GET", "POST", "PUT", "PATCH", "DELETE", "HEAD", "OPTIONS", "TRACE":
		return true
	default:
		return false
	}
}

func normalizeAPIPath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return p
	}
	// On Windows, Git Bash might convert paths like "/v1/catalog" to "C:/Program Files/Git/v1/catalog"
	// We need to ensure we only add a leading slash if it's not already there and it looks like a relative path
	if !strings.HasPrefix(p, "/") && !strings.Contains(p, ":") {
		p = "/" + p
	}
	return p
}

// FindOperation locates one operation by operationId or METHOD+PATH.
func FindOperation(ops []Operation, method, path, operationID string) (*Operation, error) {
	if operationID != "" {
		var matches []Operation
		for i := range ops {
			if ops[i].OperationID == operationID || strings.EqualFold(ops[i].OperationID, operationID) {
				matches = append(matches, ops[i])
			}
		}
		if len(matches) == 1 {
			return &matches[0], nil
		}
		if len(matches) > 1 {
			return nil, fmt.Errorf("operationId %q matches %d operations", operationID, len(matches))
		}
		return nil, fmt.Errorf("unknown operationId %q (try `%s api list`)", operationID, ProgName())
	}
	method = strings.ToUpper(strings.TrimSpace(method))
	path = normalizeAPIPath(path)
	for i := range ops {
		if ops[i].Method == method && ops[i].Path == path {
			op := ops[i]
			return &op, nil
		}
	}
	return nil, fmt.Errorf("unknown operation %s %s (try `%s api list`)", method, path, ProgName())
}

// ExampleAPIBase is the configured Horizon API origin for example URLs, or "{api}".
// A domain with no explicit base URL is expanded to https://<domain>/horizon-api.
func ExampleAPIBase(cfg *Config) string {
	if cfg == nil {
		return "{api}"
	}
	base := strings.TrimSpace(cfg.BaseURL)
	if base == "" {
		if domain := strings.TrimSpace(cfg.Domain); domain != "" {
			base = "https://" + domain + "/horizon-api"
		}
	}
	if base == "" {
		return "{api}"
	}
	return strings.TrimSuffix(base, "/")
}

type exampleRequest struct {
	Method      string         `json:"method"`
	URL         string         `json:"url"`
	Headers     map[string]any `json:"headers,omitempty"`
	Query       map[string]any `json:"query,omitempty"`
	PathParams  map[string]any `json:"pathParams,omitempty"`
	ContentType string         `json:"contentType,omitempty"`
	Body        any            `json:"body,omitempty"`
}

func (d *OpenAPIDocument) exampleRequest(op Operation, apiBase string) exampleRequest {
	ex := exampleRequest{
		Method:     op.Method,
		URL:        strings.TrimSuffix(apiBase, "/") + op.Path,
		Headers:    map[string]any{"Authorization": "Bearer <token>", "Accept": "application/json"},
		Query:      map[string]any{},
		PathParams: map[string]any{},
	}
	for _, p := range op.Parameters {
		val := paramExampleValue(p)
		switch strings.ToLower(p.In) {
		case "query":
			if val != nil {
				ex.Query[p.Name] = val
			}
		case "path":
			if val == nil {
				val = "{" + p.Name + "}"
			}
			ex.PathParams[p.Name] = val
		case "header":
			if val != nil {
				ex.Headers[p.Name] = val
			}
		}
	}
	if len(ex.Query) == 0 {
		ex.Query = nil
	}
	if len(ex.PathParams) == 0 {
		ex.PathParams = nil
	}
	if len(op.RequestBody) > 0 {
		ct, schema := requestBodySchema(op.RequestBody)
		ex.ContentType = ct
		if ct != "" {
			ex.Headers["Content-Type"] = ct
		}
		if schema != nil {
			ex.Body = exampleFromSchema(schema, d.schemas, map[string]bool{})
		}
	}
	return ex
}

func paramExampleValue(p OpParameter) any {
	if p.Example != nil {
		return p.Example
	}
	if p.Default != nil {
		return p.Default
	}
	if strings.EqualFold(p.In, "path") {
		return "{" + p.Name + "}"
	}
	return nil
}

func requestBodySchema(raw json.RawMessage) (contentType string, schema map[string]any) {
	var rb map[string]any
	if err := json.Unmarshal(raw, &rb); err != nil {
		return "", nil
	}
	content, _ := rb["content"].(map[string]any)
	if content == nil {
		return "", nil
	}
	prefer := []string{"application/json", "application/x-ndjson"}
	for _, ct := range prefer {
		if media, ok := content[ct].(map[string]any); ok {
			if s, ok := media["schema"].(map[string]any); ok {
				return ct, s
			}
		}
	}
	for ct, v := range content {
		media, ok := v.(map[string]any)
		if !ok {
			continue
		}
		if s, ok := media["schema"].(map[string]any); ok {
			return ct, s
		}
	}
	return "", nil
}

func exampleFromSchema(schema map[string]any, components map[string]any, visiting map[string]bool) any {
	if schema == nil {
		return nil
	}
	if ex, ok := schema["example"]; ok {
		return ex
	}
	if ref := strings.TrimSpace(stringFromAny(schema["$ref"])); ref != "" {
		name := strings.TrimPrefix(ref, "#/components/schemas/")
		if visiting[name] {
			return map[string]any{}
		}
		target, ok := components[name].(map[string]any)
		if !ok {
			return map[string]any{"$ref": ref}
		}
		visiting[name] = true
		defer delete(visiting, name)
		return exampleFromSchema(target, components, visiting)
	}
	if allOf, ok := schema["allOf"].([]any); ok {
		merged := map[string]any{}
		for _, part := range allOf {
			pm, ok := part.(map[string]any)
			if !ok {
				continue
			}
			ex := exampleFromSchema(pm, components, visiting)
			if m, ok := ex.(map[string]any); ok {
				for k, v := range m {
					merged[k] = v
				}
			}
		}
		return merged
	}
	if def, ok := schema["default"]; ok {
		return def
	}
	if enums, ok := schema["enum"].([]any); ok && len(enums) > 0 {
		return enums[0]
	}
	typ := strings.TrimSpace(stringFromAny(schema["type"]))
	if typ == "" {
		if _, ok := schema["properties"]; ok {
			typ = "object"
		} else if _, ok := schema["items"]; ok {
			typ = "array"
		} else if _, ok := schema["additionalProperties"]; ok {
			typ = "object"
		}
	}
	switch typ {
	case "object":
		props, _ := schema["properties"].(map[string]any)
		out := map[string]any{}
		keys := make([]string, 0, len(props))
		for k := range props {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			pm, ok := props[k].(map[string]any)
			if !ok {
				continue
			}
			out[k] = exampleFromSchema(pm, components, visiting)
		}
		if ap, ok := schema["additionalProperties"].(map[string]any); ok && len(out) == 0 {
			out["additionalProp1"] = exampleFromSchema(ap, components, visiting)
		}
		return out
	case "array":
		items, _ := schema["items"].(map[string]any)
		if items == nil {
			return []any{}
		}
		return []any{exampleFromSchema(items, components, visiting)}
	case "string":
		if f := strings.TrimSpace(stringFromAny(schema["format"])); f == "date-time" {
			return "2026-01-01T00:00:00Z"
		}
		return "string"
	case "integer", "number":
		return 0
	case "boolean":
		return false
	default:
		return nil
	}
}

// FormatOperationShow is the human-readable `horizon api show` output.
func FormatOperationShow(doc *OpenAPIDocument, op Operation, apiBase string) string {
	ex := doc.exampleRequest(op, apiBase)
	var b strings.Builder
	fmt.Fprintf(&b, "%s %s\n", op.Method, op.Path)
	if op.OperationID != "" {
		fmt.Fprintf(&b, "operationId: %s\n", op.OperationID)
	}
	if len(op.Tags) > 0 {
		fmt.Fprintf(&b, "tags: %s\n", strings.Join(op.Tags, ", "))
	}
	if op.Summary != "" {
		fmt.Fprintf(&b, "summary: %s\n", op.Summary)
	}
	if op.Description != "" {
		fmt.Fprintf(&b, "description: %s\n", op.Description)
	}
	if len(op.Parameters) > 0 {
		b.WriteString("\nParameters:\n")
		for _, p := range op.Parameters {
			req := "optional"
			if p.Required {
				req = "required"
			}
			typ := p.Type
			if typ == "" {
				typ = "string"
			}
			fmt.Fprintf(&b, "  %-6s %-24s %s  %s", p.In, p.Name, req, typ)
			if p.Default != nil {
				fmt.Fprintf(&b, "  default=%v", p.Default)
			}
			b.WriteByte('\n')
			if p.Description != "" {
				fmt.Fprintf(&b, "           %s\n", p.Description)
			}
		}
	}
	b.WriteString("\nExample request:\n")
	fmt.Fprintf(&b, "  %s %s\n", ex.Method, ex.URL)
	if len(ex.Headers) > 0 {
		keys := make([]string, 0, len(ex.Headers))
		for k := range ex.Headers {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(&b, "  %s: %v\n", k, ex.Headers[k])
		}
	}
	if len(ex.Query) > 0 {
		b.WriteString("  Query:\n")
		keys := make([]string, 0, len(ex.Query))
		for k := range ex.Query {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(&b, "    %s=%v\n", k, ex.Query[k])
		}
	}
	if ex.Body != nil {
		body, err := json.MarshalIndent(ex.Body, "  ", "  ")
		if err == nil {
			b.WriteString("  Body:\n  ")
			b.Write(body)
			b.WriteByte('\n')
		}
	}
	return b.String()
}

// OperationShowJSON is the machine-readable -j payload for `horizon api show`.
func OperationShowJSON(doc *OpenAPIDocument, op Operation, apiBase string) ([]byte, error) {
	ex := doc.exampleRequest(op, apiBase)
	wrap := map[string]any{
		"method":      op.Method,
		"path":        op.Path,
		"operationId": op.OperationID,
		"summary":     op.Summary,
		"description": op.Description,
		"tags":        op.Tags,
		"parameters":  op.Parameters,
		"example":     ex,
	}
	return json.MarshalIndent(wrap, "", "  ")
}
