<!-- Copyright (c) 2026 Accenture, All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

        http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License. -->

# Interfaces

This page is a reference for the interfaces between the module components: the Horizon API, the Module Manager API, and the contract by which a module learns which of its soft dependencies are enabled. For how the components fit together, see [Modularization Architecture](04_module-architecture.md).

## Table of Contents

- [Horizon API endpoints](#horizon-api-endpoints)
- [Module Manager endpoints](#module-manager-endpoints)
- [Soft-feature contract](#soft-feature-contract)

## Horizon API endpoints

The Horizon API is served at `https://<your-horizon-domain>/horizon-api`. Every `/v1` call needs a Keycloak bearer token for the `horizon` realm. The Swagger page (`/swagger`) and the OpenAPI document (`/openapi.json`) don't.

| Method and path | Purpose |
|---|---|
| `GET /v1/catalog` | Exposed templates of enabled modules, with their parameters. |
| `POST /v1/modules/{module}/workflowTemplates/{template}/submit` | Start a run. Body: `{"parameters": {"<name>": "<value>"}}`. Returns `202` once the event is dispatched to Argo Events. |
| `GET /v1/workflows/running` | Runs in progress. `limit` defaults to 50, maximum 500. |
| `GET /v1/workflows/history` | Finished runs. Takes `limit`, `continue` and filters such as `phase`; `continue` can't be combined with filters. |
| `GET /v1/workflows/{workflowName}` | Detail of one run, including archived log and artifact locations. |
| `GET /v1/workflows/{workflowName}/log` | Log stream as newline-delimited JSON. Takes `follow` (default `true`), `container` (default `main`) and `podName`. |
| `GET /v1/workflows/{workflowName}/downloadArtifact/{artifactName}` | Signed URL for an artifact (`durationSeconds`, default 600), or the bytes with `inline=true`. `nodeId` or `templateName` choose between steps that produced the same name. |
| `POST /v1/workflows/{workflowName}/abort` | Stop a running workflow. |
| `DELETE /v1/workflows/{workflowName}` | Delete a finished run. Argo Workflows garbage-collects its artifacts in GCS. The call waits until the workflow's finalizers are gone. |

Submissions record where they came from in the `horizon-sdv.io/submitted-from` label on the workflow. Set it with the `X-Horizon-Submitted-From` header: `api` (the default), `developer-portal`, `horizon-cli`, or `custom` together with your integration ID in `X-Horizon-Submitted-From-Detail`. A `horizonSubmittedFrom` parameter in the body overrides the header. The **Workflows visibility** setting in the portal filters runs by this label.

## Module Manager endpoints

Module Manager is internal to the cluster. The Developer Portal reaches it through its proxy under `/developer-portal/api/mm/`, forwarding the signed-in user's token.

| Method and path | Purpose |
|---|---|
| `GET /modules` | All catalog modules with enabled state, dependencies and merged applications. |
| `GET /modules/{idOrName}` | One module. |
| `GET /modules/{idOrName}/status` | Argo CD sync and health of the module's Applications; the portal derives the status chip from it. |
| `GET /modules/{idOrName}/overview` | The module's overview HTML, fetched from its overview Service. `404` if the catalog has no overview Service for the module. |
| `POST /modules/{idOrName}/enable` | Enable, with hard dependencies first. An optional `targetRevision` in the body pins the module. `409` "platform teardown in progress" while the root Argo CD Application is being deleted. |
| `DELETE /modules/{idOrName}/disable` | Disable. `409` with `hardDependents` if an enabled module requires it. |
| `PUT /modules/{idOrName}/target-revision` | Pin an enabled module to a ref. Body: `{"targetRevision": "<ref>"}`. |
| `DELETE /modules/{idOrName}/target-revision` | Reset the module to the platform branch. |
| `GET` and `PUT /settings/workflows-visibility` | The **Workflows visibility** setting (`allowedSubmittedFrom`). |

A module's applications are the catalog entry's `applications` list merged with child Argo CD Applications that carry the labels `horizon-sdv.io/module: <module>`, `horizon-sdv.io/app-role: child` and `horizon-sdv.io/expose: "true"`, and the annotation `horizon-sdv.io/portal-url`. See [Expose label](05_creating-modules.md#expose-label-catalog-and-portal-links).

## Soft-feature contract

A parent module learns which of its soft dependencies are enabled through the catalog field `softFeaturesPropagation`:

| Value | What the parent gets |
|---|---|
| `HelmValues` (default) | A `softFeaturesEnabled` map (`<dependency>: true\|false`) in the parent Application's Helm values. |
| `ConfigMap` | A ConfigMap named `horizon-sdv-soft-features-<module>` in each namespace listed in `softFeaturesConfigMapNamespaces`. |
| `HelmValuesAndConfigMap` | Both. |

The ConfigMap has two keys. `features.env` holds one `SOFT_FEATURE_ENABLED_<DEPENDENCY>=true|false` line per soft dependency, with the name upper-cased and `-` and `.` replaced by `_` (so `sample-soft` becomes `SOFT_FEATURE_ENABLED_SAMPLE_SOFT`). `soft-features.json` holds the same flags as JSON. Module Manager updates both whenever a soft dependency is enabled or disabled. The `sample` module mounts the ConfigMap into its hello-world pod; see `gitops/modules/sample-module/hello-world`.
