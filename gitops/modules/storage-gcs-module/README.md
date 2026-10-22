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

# Storage GCS Manager

Helm chart path in this repository: `gitops/modules/storage-gcs-module` (synced by Module Manager as Argo CD Application `mod-storage-gcs` when enabled from the Developer Portal as catalog module **`storage-gcs`**).

Project **GCS buckets** `{project_id}-aaos` and `{project_id}-openbsw` are provisioned by **Terraform** (`terraform/modules/base/main.tf`, module `sdv-gcs`). **`{project_id}-sample-workloads-data`** is declared by **`sample-data`** as a **`GCSBucket`** CR reconciled by this operator (enable this module before **sample-data**; see `gitops/modules/sample-data-module`). **`{project_id}-argo-workflows`** is provisioned by platform GitOps KCC (`gitops/templates/argo-workflows-bucket.yaml`, `deletion-policy: abandon`). Terraform no longer manages that bucket: `removed { from = module.sdv_gcs_argo_workflows ... destroy = false }` in `terraform/modules/base/main.tf` drops it from state without deleting GCP objects (or use `terraform state rm` on the old address if needed). **Cluster-mode** `ConfigConnector` is platform GitOps in `gitops/templates/config-connector.yaml` (root `horizon-sdv` app, main env only).

Disabling **storage-gcs** is blocked while **sample-data** is enabled (hard dependent). Disable **sample-data** first; that CR uses `deletionPolicy: Delete`, so the operator empties and deletes `{project_id}-sample-workloads-data`. Other CRs default to **Retain** (GCP bucket is kept).

Additional buckets can be created dynamically by applying more **`GCSBucket`** resources in the manager namespace.

The **Go** binary (`controller-runtime`) reconciles **`GCSBucket`** CRs and serves an internal **ClusterIP** REST API on port **8443** (TLS). Kubelet probes use plain HTTP on **8080** (`/health` only).

### Security (in-cluster API)

- **NetworkPolicy:** ingress on `:8080` is limited to kubelet probe sources on node primary CIDRs (`config.probeSourceCIDRs`). Ingress on `:8443` is limited to pods in `{prefix}workflows` labeled `horizon-sdv.io/storage-gcs-api-client: "true"`. There is no ports-only (open-to-all) ingress rule. Chart defaults are `10.1.0.0/24` and `10.2.0.0/24`; production values come from Terraform `nodes_range` / `arm64_nodes_range` via root GitOps → Module Manager **MODULE_CONFIG** (`probeSourceCIDRs`). Wrong CIDRs cause silent probe failures under horizon default-deny.
- **TLS:** HTTPS on `:8443` with cluster-generated CA (`storage-gcs-module-api-tls`). Workflow clients mount `ca.crt` and set `CURL_CA_BUNDLE`.
- **Authentication:** all `/api/v1/*` routes and `/ready` require `Authorization: Bearer <token>`. Secret `storage-gcs-module-api-token` maps each caller to a current token and an optional previous token (rotation overlap). Horizon holds the full map (`tokens.json`); workflows only get caller `storage-gcs-internal`. Keys are generated once and preserved across syncs (`lookup`). `/health` stays unauthenticated for kubelet probes. API logs `caller=` and `token=current|previous` on each authenticated request. Invalid `tokens.json` fails closed (pod does not start; no fallback to the legacy token).
- **Bucket allowlist:** requests are allowed only for buckets in `ALLOWED_BUCKETS` (Helm `config.allowedBuckets`, project-prefixed by default: `{project}-aaos`, `{project}-openbsw`, `{project}-argo-workflows`) or for `spec.bucketName` of a **`GCSBucket`** CR. The name must be `{projectId}-...`. Others return `403`. **`ValidatingAdmissionPolicy`** rejects invalid `GCSBucket` specs at admission (prefix + no `deletionPolicy: Delete` on platform buckets).

Do not call the HTTP API from arbitrary pods; use **`storage-gcs-upload`** (`templateRef`) which authenticates as caller `storage-gcs-internal`.

Source: `terraform/modules/sdv-container-images/images/storage-gcs-module/storage-gcs-module-app` (built as image `storage-gcs-module-app`).

Registered in the Module Manager catalog (`gitops/apps/module-manager/templates/module-catalog.yaml`). Enable from the Developer Portal on the **main** environment only (chart templates are skipped when `config.isSubEnvironment` is true). Operator and API deploy to the `horizon` namespace (with optional name prefix). Image ref is merged from Module Manager `MODULE_CONFIG` (`config.containerImages.storageGcsModule`).

**Argo Workflows:** child Application `mod-storage-gcs-argo-workflows` deploys **`storage-gcs-upload`** (`horizon-sdv.io/expose: "false"` — not listed in the Developer Portal catalog; other modules use `templateRef`, templates **`upload`** and **`resumable-upload`**) and **`storage-gcs-module-internal`** into `{namespacePrefix}workflows` (`gitops/modules/storage-gcs-module/argo-workflows`). **`storage-gcs-upload`** is the only supported entry for other modules; callers must pass **`bucket`** (GCS bucket name) and **`objectPath`**. This module does **not** hardcode client buckets (e.g. sample-data’s `{project_id}-sample-workloads-data` is supplied by the sample chart). **`resumable-upload`** streams zero-filled chunks via the resumable REST API (`POST/PUT /api/v1/resumable/...`) with per-chunk MiB/bytes/decimal-GB progress logging. Low-level REST calls live in **`storage-gcs-module-internal`** (`push-object`, `verify-object`); workflow pods are labelled `horizon-sdv.io/storage-gcs-api-client: "true"` and send `Authorization: Bearer` from Secret `storage-gcs-module-api-token` key `storage-gcs-internal`. Egress from `{prefix}workflows` to the API is allowed by platform network policy `allow-workflow-pods-to-storage-gcs-module`. Do not call the HTTP API directly from other workflow charts.

**Overview:** static HTML is served by nginx in namespace `storage-gcs-overview`. The operator and API remain in `horizon`.

## Workload Identity

GCP service account: `gke-storage-gcs-module-sa@<PROJECT_ID>.iam.gserviceaccount.com`  
Kubernetes: `horizon/storage-gcs-module` (main env).

Terraform (`sa13`): `roles/storage.admin` is bound with an IAM condition so it applies only to buckets named `{projectId}-*`. `storage.buckets.create` is a separate custom role (conditions cannot restrict create). `roles/iam.serviceAccountTokenCreator` is granted **on this SA only** (`sa_roles`) so SignBlob works without project-wide TokenCreator.

## Debugging (GKE)

The runtime image is **distroless** (no shell). Use an ephemeral debug container:

```bash
kubectl debug -it deploy/storage-gcs-module -n <namespace> --target=manager --image=busybox:1.36
```

Use your real namespace (for example `horizon` or a prefixed name from GitOps). The manager process runs as UID **65532** (`nonroot`).

## REST API (in-cluster)

Base URL: `https://storage-gcs-module.<namespace>.svc.cluster.local:8443`

All `/api/v1/*` routes require header `Authorization: Bearer <token>` (Secret `storage-gcs-module-api-token`). `{bucket}` must be allowlisted (static `ALLOWED_BUCKETS` and/or a `GCSBucket` CR `spec.bucketName`).

Horizon Secret keys:

| Key | Purpose |
|-----|---------|
| `tokens.json` | `{"<caller>":{"current":"...","previous":"..."}}` — mounted as env `API_TOKENS` |
| `token` | Caller `default` (legacy / ad-hoc); optional env `API_BEARER_TOKEN` if `tokens.json` is empty |
| `storage-gcs-internal` | Workflow templates (`storage-gcs-module-internal`) |
| `token-previous` / `storage-gcs-internal-previous` | Optional overlap during rotation |

**Rotate one caller** (does not change other callers). Helm owns `tokens.json` from the per-caller Secret keys — patch those keys, not `tokens.json` by hand:

1. Copy that caller’s current Secret key to the matching `*-previous` key (`token` → `token-previous`, `storage-gcs-internal` → `storage-gcs-internal-previous`).
2. Write a new value into the current key.
3. On the next GitOps sync Helm rewrites `tokens.json` and the Deployment rolls (`checksum/api-token`). For an immediate cutover, write the same map into `tokens.json` and `kubectl rollout restart deploy/storage-gcs-module`.
4. After clients use the new current token, delete the `*-previous` key and sync (or restart) again.

To revoke one caller, delete its keys from the Secret and restart; other callers keep working.

| Method | Path | Purpose |
|--------|------|---------|
| GET | `/health` | Liveness and Kubernetes readiness probes (fast; no auth) |
| GET | `/ready` | Optional deep check: default bucket reachable via GCS (not used by probes; can be slow; **requires auth**) |
| POST | `/api/v1/buckets/{bucket}/objects` | Multipart: form field `object_path` + `file` |
| GET | `/api/v1/buckets/{bucket}/objects/{object_path...}` | Download object (supports HTTP `Range`) |
| POST | `/api/v1/buckets/{bucket}/packages` | Multipart: `object_path` + repeated `file` → streams ZIP into GCS |
| GET | `/api/v1/buckets/{bucket}/objects-meta/{object_path...}` | Object metadata (`object_path` may contain `/`) |
| GET | `/api/v1/buckets/{bucket}/objects-meta-raw/{object_path...}` | Raw metadata (includes custom `metadata` map) |
| PATCH | `/api/v1/buckets/{bucket}/objects-meta/{object_path...}` | Update raw metadata (contentType/cacheControl/custom metadata) |
| GET | `/api/v1/buckets/{bucket}/objects-list` | List objects by name prefix (`prefix`, `pageToken`, `pageSize`). Empty `prefix` lists from the start of the bucket. A prefix without a trailing `/` is a string prefix, not an exact object name. `pageSize` defaults to 1000 and must be 1–1000. |
| POST | `/api/v1/buckets/{bucket}/objects/filter` | Query objects by path/wildcards and metadata filters |
| POST | `/api/v1/buckets/{bucket}/objects-meta/batch-update` | Add/update metadata on filtered objects |
| POST | `/api/v1/buckets/{bucket}/objects-meta/batch-remove` | Remove selected or all metadata from filtered objects |
| POST | `/api/v1/buckets/{bucket}/objects-storage-class/list` | List storage classes for matching objects |
| POST | `/api/v1/buckets/{bucket}/objects-storage-class/batch-update` | Change storage class for filtered objects |
| POST | `/api/v1/buckets/{bucket}/objects/batch-delete` | Delete filtered objects (`dryRun` supported) |
| DELETE | `/api/v1/buckets/{bucket}/objects/{object_path...}` | Delete object |
| POST | `/api/v1/buckets/{bucket}/signed-urls` | Mint **GCS V4 signed URL** (JSON body below). Uses IAM `signBlob` as `gke-storage-gcs-module-sa` (override with `GCS_SIGNED_URL_SERVICE_ACCOUNT`). Default TTL **48h**, max **168h** (cap **7d**); configure via Helm `config.signedUrl` or env `GCS_SIGNED_URL_DEFAULT_EXPIRY` / `GCS_SIGNED_URL_MAX_EXPIRY` (Go durations, e.g. `48h`). |

**Signed URL request body:** `{"objectPath":"path/to/key","method":"GET"|"PUT","expiresInSeconds":<optional>,"contentType":"<optional for PUT>"}`

## Resumable uploads (big files)

This API supports two options for big files:

- **Streamed upload**: `POST /api/v1/buckets/{bucket}/objects` streams to GCS with a GCS chunk size.
- **Explicit resumable sessions** (GCS JSON API): start a session, then send chunks with `Content-Range`.

| Method | Path | Purpose |
|--------|------|---------|
| POST | `/api/v1/resumable/buckets/{bucket}/objects` | Start session (`{"objectPath":"...","contentType":"...","metadata":{...}}`) → returns `uploadId` |
| PUT | `/api/v1/resumable/uploads/{uploadId}` | Send bytes with `Content-Range` |
| GET | `/api/v1/resumable/uploads/{uploadId}` | Query session status |

## GCSBucket (bucket lifecycle)

| Field | Notes |
|-------|--------|
| `spec.bucketName` | Global GCS bucket name (required). Must be `{projectId}-...`. |
| `spec.location` | Region or multi-region (required; must match an existing bucket’s location). |
| `spec.uniformBucketLevelAccess` | Optional; defaults to `true`. |
| `spec.labels` | Optional GCP bucket labels. |
| `spec.deletionPolicy` | `Retain` (default) or `Delete`. Retain keeps the GCP bucket when the CR is removed. Delete empties all object generations then deletes the bucket. **Admission** rejects `Delete` on platform buckets (`-aaos`, `-openbsw`, `-argo-workflows`). |

**Troubleshooting `storage.buckets.get` 403:** GCS bucket names are **global**. A bare name like `sample-workloads-data` is often already owned by another project; the operator then gets *Permission denied* (not *not found*) when calling `Attrs()`. Use project-scoped names (`{project_id}-sample-workloads-data`, same pattern as Terraform buckets `-aaos` / `-openbsw`). Also confirm Terraform applied `gke-storage-gcs-module-sa` with prefix-conditioned `roles/storage.admin` (`terraform/env/main.tf`, `sa13`) and Workload Identity on `horizon/storage-gcs-module`.

**Migrating from GitOps-managed `-aaos` / `-openbsw`:** If `GCSBucket` CRs named `aaos` or `openbsw` already exist, delete the CRs only when you intend to drop operator management. Default `deletionPolicy: Retain` keeps the GCP bucket; set `Delete` only if you want the operator to empty and remove it. Prefer adopting under Terraform before deleting those CRs if you need to keep the data. Then run `terraform apply` to import or adopt the buckets under Terraform state.

Example (dynamic bucket):

```yaml
apiVersion: gcsmanager.horizon.io/v1alpha1
kind: GCSBucket
metadata:
  name: my-team-data
  namespace: horizon
spec:
  bucketName: my-project-my-team-data
  location: US
  uniformBucketLevelAccess: true
  deletionPolicy: Retain
```

## Utility migration from Jenkins

The former Jenkins GCS utility jobs are now expected to use the `storage-gcs-module` API directly. The batch endpoints accept object paths relative to the bucket or `gs://bucket/...` paths, support wildcard matching (`*`, `?`), and cover:

- listing object metadata
- filtering by specific / any / no metadata
- adding / removing metadata on matching objects
- listing / updating storage class on matching objects
- deleting matching objects with optional dry-run

## Local build

```bash
cd terraform/modules/sdv-container-images/images/storage-gcs-module/storage-gcs-module-app
go build -o manager ./cmd/manager
```
