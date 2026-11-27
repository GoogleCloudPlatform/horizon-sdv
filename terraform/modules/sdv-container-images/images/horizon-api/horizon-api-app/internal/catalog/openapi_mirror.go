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

package catalog

// openapiMirrorPaths documents /v1/storage-mirror/* (proxied to the storage-mirror module; 404 when it is disabled).
func openapiMirrorPaths() map[string]interface{} {
	nameParam := map[string]interface{}{"name": "name", "in": "path", "required": true, "schema": map[string]interface{}{"type": "string"}}
	jsonBody := func(desc string) map[string]interface{} {
		return map[string]interface{}{
			"required":    desc != "",
			"description": desc,
			"content": map[string]interface{}{"application/json": map[string]interface{}{
				"schema": map[string]interface{}{"type": "object", "additionalProperties": true},
			}},
		}
	}
	op := func(id, summary, desc string, params []interface{}, body map[string]interface{}) map[string]interface{} {
		o := map[string]interface{}{
			"tags":        []interface{}{"Storage Mirror"},
			"operationId": id,
			"summary":     summary,
			"description": desc,
			"responses": map[string]interface{}{
				"200": map[string]interface{}{"description": "OK"},
				"201": map[string]interface{}{"description": "Created"},
				"202": map[string]interface{}{"description": "Accepted (run requested; see requestId / runId)"},
				"400": map[string]interface{}{"description": "Invalid request"},
				"401": map[string]interface{}{"description": "Unauthorized"},
				"403": map[string]interface{}{"description": "Forbidden (admin role, or volume destroy not allowed in Git)"},
				"404": map[string]interface{}{"description": "Not found, or the storage-mirror module is not enabled"},
				"409": map[string]interface{}{"description": "Conflict (directory in use, run active, mirrors still exist)"},
				"422": map[string]interface{}{"description": "Validation failed (immutable field, bad kind)"},
				"502": map[string]interface{}{"description": "storage-mirror module error"},
				"503": map[string]interface{}{"description": "storage-mirror module URL, token or CA not configured"},
			},
		}
		if params != nil {
			o["parameters"] = params
		}
		if body != nil {
			o["requestBody"] = body
		}
		return o
	}
	q := func(n, d string) map[string]interface{} {
		return map[string]interface{}{"name": n, "in": "query", "schema": map[string]interface{}{"type": "string"}, "description": d}
	}
	byName := []interface{}{nameParam}
	return map[string]interface{}{
		"/v1/storage-mirror/mirrors": map[string]interface{}{
			"get": op("listMirrors", "List mirrors", "Managed mirrors (StorageMirror resources) and catalog-only entries of metadata.yaml (`managed: false`).",
				[]interface{}{q("kind", "repo or git"), q("phase", "Pending, Initializing, Syncing, Ready, Error, Deleting")}, nil),
			"post": op("createMirror", "Create a mirror", "Body: `{\"name\",\"kind\":\"repo|git\",\"directory\",\"repo\":{\"manifestUrl\",\"manifestRef\",\"manifestFile\",\"syncJobs\"},\"git\":{\"url\",\"branches\",\"credentialsSecretName\"},\"lfs\":\"inherit|disabled|enabled|auto\",\"schedule\":\"inherit|none|<cron>\",\"syncTimeout\"}`. Starts the create run.", nil, jsonBody("Mirror definition")),
		},
		"/v1/storage-mirror/mirrors/{name}": map[string]interface{}{
			"get":    op("getMirror", "Get a mirror", "Spec, status and catalog entry.", byName, nil),
			"patch":  op("updateMirror", "Update mutable fields", "manifestRef, manifestFile, syncJobs, branches, lfs, schedule, suspend, syncTimeout. kind, directory, manifestUrl and url are immutable.", byName, jsonBody("Fields to change")),
			"delete": op("deleteMirror", "Delete a mirror", "Unregisters the mirror and keeps its data. `purge=true&confirm=<name>` deletes the data too.", []interface{}{nameParam, q("purge", "true deletes the mirror directory"), q("confirm", "mirror name, required with purge")}, nil),
		},
		"/v1/storage-mirror/mirrors/{name}/sync": map[string]interface{}{
			"post": op("syncMirror", "Sync a mirror", "Requests a sync through the storage-mirror-sync WorkflowTemplate (Argo Events). Returns `requestId`, or `runId` when a run is already active.", byName, nil),
		},
		"/v1/storage-mirror/mirrors/{name}/sync/cancel": map[string]interface{}{
			"post": op("cancelMirrorSync", "Stop the active run", "", byName, nil),
		},
		"/v1/storage-mirror/mirrors/{name}/status": map[string]interface{}{
			"get": op("getMirrorStatus", "Sync status and live progress", "phase, currentRun.progress (projects/refs done, percentages, state), lastRun.", byName, nil),
		},
		"/v1/storage-mirror/mirrors/{name}/runs": map[string]interface{}{
			"get": op("listMirrorRuns", "Run history", "Run IDs are Workflow names; use /v1/workflows/{workflowName}/log for logs.", byName, nil),
		},
		"/v1/storage-mirror/syncs": map[string]interface{}{
			"post": op("syncMirrors", "Sync several mirrors", "Body `{\"all\":true}` or `{\"mirrors\":[...]}`.", nil, jsonBody("Selection")),
		},
		"/v1/storage-mirror/import": map[string]interface{}{
			"post": op("importMirrors", "Manage catalog-only mirrors", "Creates Retain StorageMirror resources for metadata.yaml entries without one. Body `{}` or `{\"mirrors\":[...]}`.", nil, jsonBody("")),
		},
		"/v1/storage-mirror/volume": map[string]interface{}{
			"get":    op("getMirrorVolume", "Volume state", "Filestore instance, capacity, usage and conditions.", nil, nil),
			"delete": op("destroyMirrorVolume", "Destroy the volume", "Two-key: needs mirror.volume.allowDestroy=true in Git and `confirm=<instance>`. Deletes every mirror's data.", []interface{}{q("confirm", "Filestore instance name"), q("force", "true also when mirrors exist")}, nil),
		},
		"/v1/storage-mirror/config": map[string]interface{}{
			"get": op("getMirrorConfig", "Effective StorageMirrorConfig", "Read-only; Git is the only writer.", nil, nil),
		},
	}
}
