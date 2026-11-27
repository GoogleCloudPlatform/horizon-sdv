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

# Storage Mirror Module

---

## Module Metadata

| Metadata Field | Value |
| :--- | :--- |
| **STATUS** | experimental |
| **OWNER** | Accenture |
| **CONTACT** | [horizon-sdv@accenture.com](mailto:horizon-sdv@accenture.com) |
| **CHART VERSION** | 0.1.0 |
| **CONTAINS_COMMERCIAL** | no |

---

## Overview

Helm chart: `gitops/modules/storage-mirror-module`. Argo CD parent Application `mod-storage-mirror`, catalog name
`storage-mirror`. Design: [docs/design/TAA-1593-storage-mirror-design.md](../../../docs/design/TAA-1593-storage-mirror-design.md).

The module keeps AOSP `repo` manifest mirrors (`repo init --mirror` + `repo sync`) and single-repository mirrors
(`git clone --mirror` + `git remote update`) on one shared Filestore volume. It replaces the Jenkins jobs under
`Android/Environment/Mirror` and keeps their on-disk layout: `/mnt/mirror-filestore-pvc/horizon-mirrors/<dir>` and the
`horizon-mirrors/metadata.yaml` catalog.

## What it deploys

| Component | Where | Notes |
|---|---|---|
| CRDs `StorageMirrorConfig`, `StorageMirror` (`storagemirror.horizon-sdv.io/v1alpha1`) | cluster | `Delete=false,Prune=false`: disabling the module keeps them |
| `StorageMirrorConfig/default` | `{prefix}horizon` | Rendered from `values.yaml` `mirror.*`; Git is its only writer |
| Controller `storage-mirror-module` | `{prefix}horizon` | Reconciles the volume, keeps mirror status, REST API on `:8443` (TLS), probes on `:8080` |
| `FilestoreInstance storage-mirror` (Config Connector) | `{prefix}horizon` | Created by the controller, `deletion-policy: abandon`, not tracked by Argo CD |
| Static PVs and claims `storage-mirror-rw`, `storage-mirror-ro` (`{prefix}workflows`), `sdv-mirror-filestore-pvc` (`{prefix}jenkins`, read-only) | cluster / namespaces | Created by the controller, `Retain`, not tracked by Argo CD |
| WorkflowTemplates `storage-mirror-create`, `-sync`, `-delete` and `-volume-init` (none exposed to the Horizon API catalog) with Sensors `webhook-storage-mirror-*` | `{prefix}workflows`, `{prefix}argo-events` | Child Application `mod-storage-mirror-argo-workflows` |
| Overview | `storage-mirror-overview` | nginx, Module Manager overview |

## Prerequisites

- Main environment only (`config.isSubEnvironment` false).
- Terraform from this branch: Workload Identity of `{prefix}workflows/storage-mirror-sync` on `gke-argo-workflows-sa`
  (log archiving), images `storage-mirror-module-app` and `storage-mirror-sync`, MODULE_CONFIG `storageMirror.network`
  and `containerImages.storageMirror*`.
- Config Connector (cluster mode, `roles/editor`): it creates and resizes the Filestore instance. No IAM rights are
  needed or granted.
- `file.googleapis.com` and the Filestore CSI driver (already enabled by Terraform).

## How runs start

Every run enters through the shared Argo Events webhook (`/events/workflow`), like every other Horizon module:

- API, CLI and schedules: the controller posts the `workflow-dispatch` body; Sensor `webhook-storage-mirror-<purpose>`
  submits the Workflow.
- The templates carry `horizon-sdv.io/expose: "false"`, so the Developer Portal and `horizon workflow submit` do not
  list them; mirror changes go through `/v1/storage-mirror/*`, where Horizon API enforces the admin role.

The controller watches every Workflow labelled `horizon-sdv.io/module: storage-mirror` and keeps the CR status in step.
Workflows of a mirror without a `StorageMirror` (for example posted straight to the webhook) are not adopted; their
catalog entry can be brought under management with `POST /api/v1/import`. Argo enforces concurrency: semaphore
`storage-mirror-semaphores/sync` (value `mirror.concurrency.maxParallelSyncs`) and a mutex per mirror directory.

The sync workflows are the only writers of `metadata.yaml` (under `flock`), exactly like the Jenkins scripts were. Each
run reports the catalog and the volume usage back to the controller, which keeps the last snapshot in ConfigMap
`storage-mirror-catalog`.

## REST API (in-cluster)

Base URL `https://storage-mirror-module.{prefix}horizon.svc.cluster.local:8443`. All routes need
`Authorization: Bearer <token>` from Secret `storage-mirror-module-api-token` (callers `default`, `horizon-api`,
`storage-mirror-agent`). End users go through Horizon API `/v1/storage-mirror/*` instead.

The controller creates the token Secret and the TLS Secret `storage-mirror-module-api-tls` (own CA) on its first start,
with copies holding only the agent token and `ca.crt` in `{prefix}workflows` and only the `horizon-api` token and
`ca.crt` in `{prefix}horizon-api`. They are not chart resources: renders never regenerate them and disabling the
module keeps them, like the mirror data. To rotate a token, set `previous` and a new `current` for the caller in
`tokens.json` and restart the controller; it rewrites the per-caller keys and the copies.

Horizon API requires the Keycloak realm role `horizon-storage-mirror-admin` for every mutating call (create, update,
sync, cancel, delete, import, volume destroy); reads need only a signed-in user. `keycloak-post-horizon-api` creates the
role and grants it to the `administrators` group and the `realm_admin` composite. Grant it to other users or to
`service-account-horizon-api-ci` in Keycloak as needed, or set `mirrorAdminRole: ""` in the horizon-api values to opt
out.

| Method and path | Purpose |
|---|---|
| `GET /api/v1/mirrors` | List managed and unmanaged (catalog-only) mirrors |
| `POST /api/v1/mirrors` | Create (`{"name","kind":"repo|git","repo":{...}|"git":{"url",...},"lfs","schedule"}`) |
| `GET/PATCH/DELETE /api/v1/mirrors/{name}` | Get, update mutable fields, unregister (`?purge=true&confirm=<name>` deletes data) |
| `POST /api/v1/mirrors/{name}/sync`, `POST /api/v1/syncs` | Sync one, or `{"all":true}` / `{"mirrors":[..]}` |
| `GET /api/v1/mirrors/{name}/status`, `/runs` | Live progress and run history |
| `POST /api/v1/mirrors/{name}/sync/cancel` | Stop the active run |
| `POST /api/v1/import` | Create Retain CRs for catalog entries without one |
| `GET/DELETE /api/v1/volume`, `GET /api/v1/config` | Volume state, two-key destroy, effective config |

## Data safety

| Action | Effect |
|---|---|
| Disable the module | Controller and templates go. CRDs, mirrors, API Secrets, Filestore instance, PVs, claims and data stay. |
| Delete a mirror (default) | The CR goes; the directory and its catalog entry stay (shown as unmanaged, can be imported). |
| Delete with `purge=true&confirm=<name>` | `storage-mirror-delete` removes the directory and the catalog entry, then the CR goes. |
| Destroy the volume | Needs `mirror.volume.allowDestroy: true` in Git **and** `DELETE /api/v1/volume?confirm=<instance>`. |

## git-lfs

Off by default (`mirror.defaults.lfs: Disabled`). Per mirror: `Enabled` fetches LFS objects for the mirrored branch
tips; `Auto` does so only when a `.gitattributes` at a branch tip uses `filter=lfs`. `repo init --reference` and
`git clone --reference` share git objects, not LFS objects; a consumer that wants the mirrored LFS objects sets
`git config lfs.url file:///mnt/mirror-filestore-pvc/horizon-mirrors/<dir>` (git kind) or the project path inside the
repo mirror.

## Debugging

```bash
kubectl get storagemirrorconfig default -n horizon -o yaml
kubectl get storagemirrors -n horizon
kubectl get workflows -n workflows -l horizon-sdv.io/module=storage-mirror
kubectl get filestoreinstance storage-mirror -n horizon
```
