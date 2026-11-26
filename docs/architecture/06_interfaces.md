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

| Method and path | Minimum role | Purpose |
|---|---|---|
| `GET /v1/me` | viewer | The caller's name, role and capabilities. |
| `GET /v1/catalog` | viewer | Exposed templates of enabled modules, with their parameters. |
| `POST /v1/modules/{module}/workflowTemplates/{template}/submit` | developer | Start a run. Body: `{"parameters": {"<name>": "<value>"}}`. Returns `202` once the event is dispatched to Argo Events. |
| `GET /v1/workflows/running` | viewer | Runs in progress. `limit` defaults to 50, maximum 500. |
| `GET /v1/workflows/history` | viewer | Finished runs. Takes `limit`, `continue` and filters such as `phase`; `continue` can't be combined with filters. |
| `GET /v1/workflows/{workflowName}` | viewer | Detail of one run, including archived log and artifact locations. |
| `GET /v1/workflows/{workflowName}/log` | viewer | Log stream as newline-delimited JSON. Takes `follow` (default `true`), `container` (default `main`) and `podName`. |
| `GET /v1/workflows/{workflowName}/downloadArtifact/{artifactName}` | viewer | Signed URL for an artifact (`durationSeconds`, default 600), or the bytes with `inline=true`. `nodeId` or `templateName` choose between steps that produced the same name. |
| `POST /v1/workflows/{workflowName}/abort` | developer for your own run, admin for any other | Stop a running workflow. |
| `DELETE /v1/workflows/{workflowName}` | admin | Delete a finished run. Argo Workflows garbage-collects its artifacts in GCS. The call waits until the workflow's finalizers are gone. |
| `GET /v1/artifacts/buckets` | viewer | The GCS buckets reachable through the API: the static `ARTIFACTS_ALLOWED_BUCKETS` allowlist plus `GCSBucket` resources, filtered by bucket-name rules and an optional project-id prefix. |
| `GET /v1/artifacts/buckets/{bucket}/objects` | viewer | Objects in a bucket by name. Takes `prefix`, `pageToken` and `limit` (1–1000). Proxied in-cluster to the storage-gcs module. |

The minimum roles apply when role checks are on. Set `config.roleChecks: true` in the GitOps values to turn them on; they are off by default, and then every signed-in caller can make every call. A caller's role is the highest ranked `horizon-api` client role in the token, `developer` or `admin`, and a signed-in caller with neither is a `viewer`. Each role includes the roles below it.

A caller below the minimum role gets `403` with this body, which the portal and the CLI show as it is:

```json
{
  "error": "forbidden",
  "requiredRole": "admin",
  "currentRole": "developer",
  "reason": "Deleting removes the workflow record and its logs for everyone, including workflows you started."
}
```

Abort also depends on who owns the run. A run is yours when its `owner` is your token subject, and a developer can abort only their own runs. Every other run needs `admin`: another user's run, a run with no recorded owner, a run that a service account submitted in your name, and a run submitted before Horizon API took the owner from the token.

`GET /v1/me` returns the caller's `name`, `subject`, `role`, the role order (`roles`), whether role checks are on (`roleChecks`), and `capabilities`. Each capability is a row of the permission matrix with its `requiredRole` and whether the caller is `allowed`. The portal's **Your access** panel and the CLI read this list, so they show the same decisions that the endpoints enforce. The answer is computed from the token alone, with no call to Keycloak, Kubernetes or Module Manager.

Submissions record where they came from in the `horizon-sdv.io/submitted-from` label on the workflow. Set it with the `X-Horizon-Submitted-From` header: `api` (the default), `developer-portal`, `horizon-cli`, or `custom` together with your integration ID in `X-Horizon-Submitted-From-Detail`. A `horizonSubmittedFrom` parameter in the body overrides the header. The **Workflows visibility** setting in the portal filters runs by this label.

The submitter comes from the caller's verified access token, never from the request. Horizon API sets the `submittedBy` parameter to the token's `preferred_username`, and records the token subject (`sub`) as the workflow owner in the `horizon-sdv.io/submitted-by` annotation. Every sensor also stores the username in the `horizon-sdv.io/submitted-by-name` annotation; it is for display only and never decides access. A `submittedBy` parameter or `X-Horizon-Submitted-By` header sent by the client is ignored. The exception is a service account with the `horizon-api` client role `impersonate`: it may name a user in either of them, so a pipeline can attribute a run to a person, and the owner stays the service account. The running, history and detail responses return the owner as `owner`, and `startedBy` as the name to display: the `submitted-by-name` annotation, else the `submittedBy` parameter, else the owner.

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
