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

# abfs-data module

Helm chart: `gitops/modules/abfs-data-module`  
Argo CD parent Application: **`mod-abfs-data`** (enabled from the Developer Portal, catalog name `abfs-data`).

Owns **persistent ABFS platform data**: the license GCP service account and Kubernetes Secret, Spanner, GCS, the uploader manifest table, and the workflows that operate them (license import/clear, uploader-config selection, Spanner backup schedules). It runs no ABFS process; the runtime lives in `abfs-service-module`, the build nodes in `abfs-client-module-ubuntu`. The platform-level picture and procedures are in [docs/workloads/android/abfs.md](../../../docs/workloads/android/abfs.md).

## Catalog

| Setting | Value |
|---------|--------|
| Module name | `abfs-data` |
| Hard dependencies | none |
| `autoDisableWhenUnused` | `false` (stays enabled until you disable it) |
| Overview service | `mod-abfs-data-overview` in namespace `abfs-data-overview` (`files/overview-data.py`: license Secret state, valid-until, active uploader row) |

Enable order for ABFS: **abfs-data** → import license → `abfs-service-module` → `abfs-client-ubuntu`.

## What it deploys

Four child Applications (main environment only; skipped for sub-environments):

| Child Application | Chart | Purpose |
|-------------------|--------|---------|
| `mod-abfs-data-sa` | `abfs-license-sa/` | License SA, Secret, Spanner instance/database, GCS bucket, UniqueID Jobs |
| `mod-abfs-data-license-workflows` | `license-workflows/` | WorkflowTemplates `abfs-license-import`, `abfs-license-clear` + Sensors |
| `mod-abfs-data-uploader-config` | `uploader-config/` | ConfigMap `abfs-uploader-config` (manifest table), WorkflowTemplate `abfs-uploader-config-select` + Sensor |
| `mod-abfs-data-spanner-workflows` | `spanner-workflows/` | WorkflowTemplate `abfs-spanner-backup-schedule` + Sensor |

### Resources (`{prefix}horizon`)

| Resource | Name / ID | On disable |
|----------|-----------|------------|
| KCC `IAMServiceAccount` | `abfs-license` → `abfs-license@<project>.iam.gserviceaccount.com` | Soft-deleted in GCP (recoverable ~30 days) |
| ConfigMap (not Helm-owned) | `abfs-license-sa-identity` (`uniqueId`, `email`) | **Survives** prune; used to undelete the SA. If undelete fails, the Job exits 1 and does not clear it. |
| Secret | `abfs-license` (key `license`) | Deleted |
| KCC Spanner instance + database | `abfs` / `abfs` (`regional-<region>`, 1 node) with the ABFS `spec.ddl` (tables Objects, Links, Chunks, ChunkTouches, Maps, Projects, Refs; do not edit after first apply) | CRs pruned; GCP resources **abandoned** (data kept, still billed) |
| KCC `StorageBucket` | GCS `<projectID>-abfs` (`deletion-policy: abandon`) | CR pruned; GCP bucket **abandoned** (objects kept, still billed) |
| ConfigMap | `abfs-uploader-config` (key `table`) | Deleted; the service falls back to `pusher.fallbackRepo` |

Project ID comes from Module Manager `config.projectID` (not stored in the identity ConfigMap). Existing GCP resources with these ids are **acquired**, not recreated (see the migration section of the ABFS document).

The license SA is the Google identity of the whole ABFS runtime: `abfs-service-module` uses it as node service account of the server and uploader pools (`GCE_METADATA`) and grants it the project roles (Spanner, GCS, logging, monitoring). It must be allow-listed by Google for Artifact Registry `abfs-binaries`. It is never recreated silently: the license is bound to its numeric UniqueID.

### Sync notes (`mod-abfs-data-sa`)

1. **PreSync** RBAC (wave -5)  
2. **PreSync** SA undelete Job (wave -2)  
3. Main sync: SA, Secret, Spanner instance (wave 2), database and bucket (wave 3)  
4. **PostSync** UniqueID persist Job (wave 1)  

Jobs use Argo hooks + `HookSucceeded` so they do not leave the app Missing/OutOfSync.  
`ignoreDifferences` + `RespectIgnoreDifferences` keep a written `license` (Secret) and an active row written by the selection workflow (ConfigMap `data`, WorkflowTemplate `Configuration` parameter) from being wiped on sync.

## License workflows (`{prefix}workflows` + Sensors in `{prefix}argo-events`)

| WorkflowTemplate | Effect on Secret `abfs-license` | ABFS server and uploader node pools (when the CR exists) |
|------------------|----------------------------------|----------------------------------|
| `abfs-license-import` | Verify on Apply, then write or replace `license` | Delete `abfs-data-server-node-pool` and `abfs-data-uploader-node-pool` `ContainerNodePool`s so Argo recreates them with create-time metadata; a missing pool is skipped. ABFS client node pools are not affected. |
| `abfs-license-clear` | Empty `license` value; **keep** Secret (`confirm=yes`) | Same as import (recreate without license key) |

Import verifies the pasted payload (`files/verify_license.py`) before it is stored: license type, the service account's UniqueID, the GCP project number and the expiry. A failure is printed in the workflow log and the Secret is left unchanged:

- `license type unknown`
- `license service account incorrect`
- `license not applicable to this project`
- `license already expired`

The Secret is the source of truth. The ABFS server and git-pusher read GCE instance attribute `abfs-license` on their node pools (`GCE_METADATA`); `abfs-service-module` stamps it **at pool create** and re-creates the pool when Secret and metadata differ (see its README). Changing the license after the pools exist therefore **deletes and recreates** the pools (disruptive; prefer import **before** enabling `abfs-service-module`).

**Encoding:** metadata value = **base64(license payload)** — same bytes as Secret `data.license`.

```bash
# Status written by the service node-pool PostSync Job
kubectl get configmap abfs-license-metadata-status -n abfs -o yaml

# From a server pod on the data pool (after licensed workloads are up):
kubectl exec -n abfs deploy/abfs-server -- \
  curl -s -H "Metadata-Flavor: Google" \
  http://metadata.google.internal/computeMetadata/v1/instance/attributes/abfs-license | wc -c
```

**Preferred:** Developer Portal → **abfs-data** → **Workflow Templates**. Portal submit needs the Sensors (`webhook-abfs-license-*`); `argo submit` works without them.

| Action | Portal / parameters |
|--------|---------------------|
| Import | `licensePayload` = license JSON (or JWT). A later import replaces the stored license. |
| Clear | `confirm` = `yes` |

**CLI:**

```bash
NS=horizon
WF=workflows

argo submit -n "$WF" --from workflowtemplate/abfs-license-import \
  -p licensePayload="$(cat license.json)" --watch
kubectl get secret abfs-license -n "$NS" -o jsonpath='{.data.license}' | base64 -d | head -c 80; echo

argo submit -n "$WF" --from workflowtemplate/abfs-license-clear \
  -p confirm=yes --watch
kubectl get secret abfs-license -n "$NS" -o jsonpath='{.data.license}' | base64 -d | wc -c
# expect: 0 (Secret still exists)
```

GCP Console hides Secret values; use `kubectl` to decode `data.license`.

**Portal Running / History:** runs must match this module's templates and pass **Administration → Settings → Workflows visibility**. Portal submits are labeled `developer-portal`. Plain `argo submit` often has no submit-source label and may be hidden when an allowlist is set.

## Uploader configuration (which Android revision is seeded)

ConfigMap `horizon/abfs-uploader-config`, key `table`, is a YAML list of rows; exactly one row is `active` (and `synced`). The uploaders of `abfs-service-module` seed the manifest of the active row. The table lives here so it survives a disable of the service module.

Default rows (`uploader-config/values.yaml`, `uploaderConfig.entries`): `Android14_r30`, `Android15_r36` (active), `Android16_r3`, `AndroidLatest` (`android-latest-release`), `AndroidCustom` (no stored repo until a custom config is submitted). Shared settings: manifest URL `https://android.googlesource.com/platform/manifest`, file `default.xml`, `fetch-ref-spec` `refs/*:refs/*` minus Gerrit meta refs.

WorkflowTemplate **`abfs-uploader-config-select`** (portal: **abfs-data** → Workflow Templates):

| Parameter | Meaning |
|-----------|---------|
| `Android_Versions` | Dropdown of the rows, shown as `<name> (<branch>)`; the active row is preselected |
| `Configuration` | Read-only copy of the row's repo block for predefined rows; editable YAML (`name: AndroidCustom`, `repo: {url, branch, file, fetch-ref-spec}`) for `AndroidCustom`. The submitted custom block is saved into the WorkflowTemplate so the form shows it next time. |

The workflow (`uploader-config/scripts/set_active_uploader_config.py`) marks the chosen row active/synced and every other row inactive, patches the ConfigMap, and, when StatefulSet `abfs/abfs-gerrit-uploader` exists, deletes pod `-0`, restarts the StatefulSet and waits (`syncTimeoutSeconds` 5400) for Job `abfs-pusher-config-sync` to write the new ABFS ref and for the rollout to finish. Selecting the already active row changes nothing. Seeding a new branch takes about a day; earlier revisions remain in Spanner/GCS.

```bash
kubectl get configmap abfs-uploader-config -n horizon -o jsonpath='{.data.table}'
argo submit -n workflows --from workflowtemplate/abfs-uploader-config-select \
  -p Android_Versions='Android16_r3 (android-16.0.0_r3)' --watch
```

To add a predefined row, append it to `uploaderConfig.entries` and sync; the parent Application ignores `/data` of the ConfigMap, so a row made active from the portal is not reverted.

## Spanner backup schedules

WorkflowTemplate **`abfs-spanner-backup-schedule`** manages Cloud Spanner backup schedules of database `abfs` on instance `abfs`:

| Parameter | Values |
|-----------|--------|
| `backupScheduleAction` | `DETAILS` (default, lists schedules), `CREATE`, `UPDATE`, `DELETE` |
| `backupScheduleId` | required except for `DETAILS` |
| `cron` | default `0 23 * * *` (CREATE/UPDATE) |
| `retentionDuration` | seconds, default `604800` (7 days) |
| `confirm` | `yes` to DELETE |

## Validate after enable

Use `{namespacePrefix}` if your environment has one (e.g. `dev-horizon`).

```bash
PROJECT=<gcp-project-id>
NS=horizon
WF=workflows
EV=argo-events

kubectl get application -n argocd | grep abfs-data
# expect: mod-abfs-data, -sa, -license-workflows, -uploader-config, -spanner-workflows — Synced/Healthy

kubectl get secret abfs-license -n "$NS"
kubectl get configmap abfs-uploader-config abfs-license-sa-identity -n "$NS"
kubectl get spannerinstance,spannerdatabase,storagebucket -n "$NS" -l horizon-sdv.io/module=abfs-data
kubectl get workflowtemplate -n "$WF" -l horizon-sdv.io/module=abfs-data
# expect: abfs-license-import, abfs-license-clear, abfs-uploader-config-select, abfs-spanner-backup-schedule
kubectl get sensor -n "$EV" -l horizon-sdv.io/module=abfs-data

gcloud iam service-accounts describe abfs-license@${PROJECT}.iam.gserviceaccount.com --format='value(email,uniqueId)'
gcloud spanner instances describe abfs --project="$PROJECT"
gcloud spanner databases describe abfs --instance=abfs --project="$PROJECT"
gcloud spanner databases ddl describe abfs --instance=abfs --project="$PROJECT"
# expect: Objects, Links, Chunks, ChunkTouches, Maps, Projects, Refs
gcloud storage buckets describe "gs://${PROJECT}-abfs"
```

### After disable

Secret, ConfigMap `abfs-uploader-config`, WorkflowTemplates and Sensors are pruned. Spanner and GCS **CRs** are pruned; the GCP instance, database and bucket **remain** (`deletion-policy: abandon`) and keep billing until deleted in GCP. The GCP SA is soft-deleted; identity ConfigMap remains in `horizon`. Re-enable adopts the same GCP resource IDs (`abfs`, `<projectID>-abfs`).

```bash
kubectl get secret abfs-license -n "$NS"                 # NotFound
kubectl get spannerinstance,spannerdatabase,storagebucket -n "$NS"  # NotFound
gcloud spanner instances describe abfs --project="$PROJECT"         # still exists
gcloud spanner databases describe abfs --instance=abfs --project="$PROJECT"
gcloud storage buckets describe "gs://${PROJECT}-abfs"              # still exists
```

---

## UniqueID continuity (SA soft-delete window, &lt; ~30 days)

On re-enable, a PreSync Job undeletes the GCP SA using ConfigMap `abfs-license-sa-identity` so the UniqueID (and Google license) is preserved. If undelete cannot restore that identity, the Job **exits 1** and leaves the ConfigMap unchanged — Argo reports Failed and KCC does not create a new SA. Clearing `uniqueId` to allow a new SA is a conscious operator step (and needs a new license from Google).

```bash
PROJECT=<gcp-project-id>
NS=horizon

ID1=$(gcloud iam service-accounts describe \
  abfs-license@${PROJECT}.iam.gserviceaccount.com --format='value(uniqueId)')
kubectl get configmap abfs-license-sa-identity -n "$NS" -o yaml

# Disable abfs-data; wait until GCP SA is gone (soft-deleted)
# Re-enable; wait Synced/Healthy

ID2=$(gcloud iam service-accounts describe \
  abfs-license@${PROJECT}.iam.gserviceaccount.com --format='value(uniqueId)')
test "$ID1" = "$ID2" && echo "OK: UniqueID preserved"
```

### Simulate undelete failure (sync must fail)

```bash
PROJECT=<gcp-project-id>
NS=horizon
APP=mod-abfs-data-sa

# Disable abfs-data; wait until SA is gone
kubectl patch configmap abfs-license-sa-identity -n "$NS" --type merge \
  -p '{"data":{"uniqueId":"999999999999999999999"}}'

# Enable abfs-data — PreSync undelete Job must fail; app stays Degraded/Failed
kubectl get application "$APP" -n argocd
kubectl logs -n workflows -l app.kubernetes.io/name=abfs-license-sa-undelete --tail=50

# UniqueID in the ConfigMap must still be the bogus value (not cleared)
kubectl get configmap abfs-license-sa-identity -n "$NS" \
  -o jsonpath='{.data.uniqueId}{"\n"}'

# Conscious new identity (new Google license required):
kubectl patch configmap abfs-license-sa-identity -n "$NS" --type merge \
  -p '{"data":{"uniqueId":"","email":""}}'
# Re-sync / wait Synced/Healthy — KCC creates a new SA
```

---

## Clean up old `abfs-runtime` leftovers

Earlier module revisions named the runtime account `abfs-runtime`. Remove its remains once `abfs-license` is in place:

```bash
PROJECT=<gcp-project-id>
NS=horizon
WF=workflows

gcloud iam service-accounts delete abfs-runtime@${PROJECT}.iam.gserviceaccount.com --quiet || true
kubectl delete iamserviceaccount abfs-runtime -n "$NS" --ignore-not-found
kubectl delete configmap abfs-runtime-sa-identity -n "$NS" --ignore-not-found
kubectl delete job -n "$WF" -l horizon-sdv.io/module=abfs-data --ignore-not-found
```
