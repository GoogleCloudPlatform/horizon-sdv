# abfs-data module

Helm chart: `gitops/modules/abfs-data-module`  
Argo CD parent Application: **`mod-abfs-data`** (enabled from the Developer Portal).

Owns **persistent ABFS platform data**: license GCP service account and Kubernetes Secret, Spanner, GCS, and license import/change/clear workflows.

## Catalog

| Setting | Value |
|---------|--------|
| Module name | `abfs-data` |
| Hard dependencies | none |
| `autoDisableWhenUnused` | `false` (stays enabled until you disable it) |

Enable order for ABFS later: **abfs-data** → service → client modules.

## What it deploys

Two child Applications (main environment only; skipped for sub-environments):

| Child Application | Chart | Purpose |
|-------------------|--------|---------|
| `mod-abfs-data-sa` | `abfs-license-sa/` | License SA, Secret, Spanner, GCS, UniqueID Jobs |
| `mod-abfs-data-license-workflows` | `license-workflows/` | WorkflowTemplates + Argo Events Sensors |

### Resources (`{prefix}horizon`)

| Resource | Name / ID | On disable |
|----------|-----------|------------|
| KCC `IAMServiceAccount` | `abfs-license` → `abfs-license@<project>.iam.gserviceaccount.com` | Soft-deleted in GCP (recoverable ~30 days) |
| ConfigMap (not Helm-owned) | `abfs-license-sa-identity` (`uniqueId`, `email`) | **Survives** prune; used to undelete the SA. If undelete fails, the Job exits 1 and does not clear it. |
| Secret | `abfs-license` (key `license`) | Deleted |
| KCC Spanner instance + database | `abfs` (`regional-<region>`, 1 node) with ABFS `spec.ddl` (do not edit after first apply) | CRs pruned; GCP resources **abandoned** (data kept, still billed) |
| KCC `StorageBucket` | GCS `<projectID>-abfs` (`deletion-policy: abandon`) | CR pruned; GCP bucket **abandoned** (objects kept, still billed) |

Project ID comes from Module Manager `config.projectID` (not stored in the identity ConfigMap).

### License workflows (`{prefix}workflows` + Sensors in `{prefix}argo-events`)

| WorkflowTemplate | Effect on Secret `abfs-license` | ABFS server and uploader node pools (when CR exists) |
|------------------|----------------------------------|----------------------------------|
| `abfs-license-import` | Verify on Apply, then write or replace `license` | Delete the ABFS Server (`abfs-data-server-node-pool`) and ABFS Uploader (`abfs-data-uploader-node-pool`) `ContainerNodePool`s so Argo recreates them with create-time metadata; a missing pool is skipped. ABFS Client node pools are not affected. |
| `abfs-license-clear` | Empty `license` value; **keep** Secret (`confirm=yes`) | Same as import (recreate without licence key) |

Apply verifies the pasted payload before it is stored. A failure is printed in the workflow log and the Secret is left unchanged:

- `license type unknown`
- `license service account incorrect`
- `license not applicable to this project`
- `license already expired`

Secret is source of truth. The ABFS server reads GCE instance attribute `abfs-license` on the data pool (`GCE_METADATA`). Metadata is stamped **at pool create** via Helm `lookup` of the Secret into `ContainerNodePool.spec.nodeConfig.metadata` (not committed in git).

**Why not patch / gcloud update?** KCC treats `nodeConfig.metadata` as immutable after create. `gcloud container node-pools update` has **no** `--metadata` flag. Changing the licence after the pool exists therefore **deletes and recreates** the pool CR (disruptive; prefer import **before** enabling `abfs-service`).

**Encoding:** metadata value = **base64(license payload)** — same bytes as Secret `data.license`.

**Enable order:** import licence → enable `abfs-service` → pool is created **with** metadata (no recreate). After import, sync `mod-*-data-plane` so Helm `license.autoDetect` can mobilise gated workloads.

```bash
# Status written by the service node-pool PostSync Job
kubectl get configmap abfs-license-metadata-status -n abfs -o yaml

# From a server pod on the data pool (after licensed workloads are up):
kubectl exec -n abfs deploy/abfs-server -- \
  curl -s -H "Metadata-Flavor: Google" \
  http://metadata.google.internal/computeMetadata/v1/instance/attributes/abfs-license | wc -c
```

Portal **Configure & submit** requires the Sensors (`webhook-abfs-license-*`). `argo submit` works without them.

GCP Console hides Secret values; use `kubectl` to decode `data.license`.

### Sync notes (`mod-abfs-data-sa`)

1. **PreSync** RBAC (wave -5)  
2. **PreSync** SA undelete Job (wave -2)  
3. Main sync: SA, Secret, Spanner, GCS  
4. **PostSync** UniqueID persist Job (wave 1)  

Jobs use Argo hooks + `HookSucceeded` so they do not leave the app Missing/OutOfSync.  
`ignoreDifferences` + `RespectIgnoreDifferences` keep a written `license` from being wiped on sync.

## Validate after enable

Use `{namespacePrefix}` if your environment has one (e.g. `dev-horizon`).

```bash
PROJECT=<gcp-project-id>
NS=horizon
WF=workflows
EV=argo-events

kubectl get application -n argocd | grep abfs-data
# expect: mod-abfs-data, mod-abfs-data-sa, mod-abfs-data-license-workflows — Synced/Healthy

kubectl get secret abfs-license -n "$NS"
kubectl get spannerinstance,spannerdatabase,storagebucket -n "$NS" -l horizon-sdv.io/module=abfs-data
kubectl get workflowtemplate -n "$WF" -l horizon-sdv.io/module=abfs-data
kubectl get sensor -n "$EV" -l horizon-sdv.io/module=abfs-data

gcloud spanner instances describe abfs --project="$PROJECT"
gcloud spanner databases describe abfs --instance=abfs --project="$PROJECT"
gcloud spanner databases ddl describe abfs --instance=abfs --project="$PROJECT"
# expect: Objects, Links, Chunks, ChunkTouches, Maps, Projects, Refs
gcloud storage buckets describe "gs://${PROJECT}-abfs"
```

### After disable

Secret, WorkflowTemplates, and Sensors are pruned. Spanner and GCS **CRs** are pruned; the GCP instance, database, and bucket **remain** (`deletion-policy: abandon`) and keep billing until deleted in GCP. The GCP SA is soft-deleted; identity ConfigMap remains in `horizon`. Re-enable adopts the same GCP resource IDs (`abfs`, `<projectID>-abfs`).

```bash
kubectl get secret abfs-license -n "$NS"                 # NotFound
kubectl get spannerinstance,spannerdatabase,storagebucket -n "$NS"  # NotFound
gcloud spanner instances describe abfs --project="$PROJECT"         # still exists
gcloud spanner databases describe abfs --instance=abfs --project="$PROJECT"
gcloud storage buckets describe "gs://${PROJECT}-abfs"              # still exists
```

---

## License workflows

**Preferred:** Developer Portal → **abfs-data** → **Workflow Templates**.

| Action | Portal / parameters |
|--------|---------------------|
| Import | `licensePayload` = licence JSON. A later import replaces the stored licence. |
| Clear | `confirm` = `yes` |

**CLI:**

```bash
NS=horizon
WF=workflows

argo submit -n "$WF" --from workflowtemplate/abfs-license-import \
  -p licensePayload='{"example":true}' --watch
kubectl get secret abfs-license -n "$NS" -o jsonpath='{.data.license}' | base64 -d; echo

argo submit -n "$WF" --from workflowtemplate/abfs-license-import \
  -p licensePayload='{"example":"updated"}' --watch

argo submit -n "$WF" --from workflowtemplate/abfs-license-clear \
  -p confirm=yes --watch
kubectl get secret abfs-license -n "$NS" -o jsonpath='{.data.license}' | base64 -d | wc -c
# expect: 0 (Secret still exists)
```

**Portal Running / History:** runs must match this module’s templates and pass **Administration → Settings → Workflows visibility**. Portal submits are labeled `developer-portal`. Plain `argo submit` often has no submit-source label and may be hidden when an allowlist is set.

---

## UniqueID continuity (SA soft-delete window, &lt; ~30 days)

On re-enable, a PreSync Job undeletes the GCP SA using ConfigMap `abfs-license-sa-identity` so the UniqueID (and Google licence) is preserved. If undelete cannot restore that identity, the Job **exits 1** and leaves the ConfigMap unchanged — Argo reports Failed and KCC does not create a new SA. Clearing `uniqueId` to allow a new SA is a conscious operator step (and needs a new licence from Google).

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

# Conscious new identity (new Google licence required):
kubectl patch configmap abfs-license-sa-identity -n "$NS" --type merge \
  -p '{"data":{"uniqueId":"","email":""}}'
# Re-sync / wait Synced/Healthy — KCC creates a new SA
```

---

## Clean up old `abfs-runtime` leftovers

```bash
PROJECT=<gcp-project-id>
NS=horizon
WF=workflows

gcloud iam service-accounts delete abfs-runtime@${PROJECT}.iam.gserviceaccount.com --quiet || true
kubectl delete iamserviceaccount abfs-runtime -n "$NS" --ignore-not-found
kubectl delete configmap abfs-runtime-sa-identity -n "$NS" --ignore-not-found
kubectl delete job -n "$WF" -l horizon-sdv.io/module=abfs-data --ignore-not-found
```
