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

# CVD Launcher (Argo Workflows)

Helm chart for **`cvd-launcher-on-gce`**: run the same Cuttlefish host scripts as the Jenkins CVD Launcher job on an **ephemeral GCE VM** created from an existing **instance template** (replacing the Jenkins GCE cloud agent).

## Driver

- [cvd_argo_gce/README.md](../cvd_argo_gce/README.md) — Path B operator reference (GCS layout, status, logs, dispatch).
- `workloads/android/pipelines/tests/cvd_argo_gce/cvd_argo_gce_ephemeral.sh` — **Path B:** KCC `ComputeInstance` + GCS inputs; guest **`cvd_argo_guest_startup.sh`** runs **`cvd_argo_remote_entry.sh`** (`run_*_main` / `run_*_teardown`), uploads artifacts + status; pod polls GCS and deletes the CR.
- `workloads/android/pipelines/tests/cvd_argo_gce/cvd_argo_remote_entry.sh` — guest-side stages (CVD / MTK / keep-alive in **main**; MTK stop, CVD stop, gather in **teardown**).

## Parameters

| Parameter | Notes |
|-----------|--------|
| **`instanceTemplateName`** | GCE template name in GCP; run **`cf-instance-template-x86`** or **`-arm64`** first (e.g. `instance-template-cuttlefish-vm-main`). |
| **`zoneOverride`** | Optional; default **`CLOUD_ZONE`** from `horizon-workflow-cloud-env`. |
| **`cuttlefishDownloadUrl`** | Required for a full run (same as Jenkins). |
| **`mtkConnectPublic`** | Default **`true`** (Argo WorkflowTemplate). When **`true`**, MTK testbench user is **`everyone`**. Jenkins job UI defaults **`MTK_CONNECT_PUBLIC`** to **false** — different default. |
| **`submittedBy`** | Jenkins **`BUILD_USER_ID`** equivalent for **private** MTK benches. Set on the webhook body by Horizon API (see **Submitter identity**). Empty in the pod → **`jenkins`**. |

## Submitter identity (Portal / webhook)

| Where | Who | Channel |
|-------|-----|---------|
| **Developer Portal** | Keycloak **`preferred_username`** via **`authService.getUsername()`** → HTTP header **`X-Horizon-Submitted-By`** and workflow param **`submittedBy`** | Label **`horizon-sdv.io/submitted-from=developer-portal`** |
| **Horizon API** | Resolves **`horizonSubmittedBy`**: **`X-Horizon-Submitted-By`** → **`parameters.submittedBy`** → OIDC **`sub`** (CLI/REST fallback) | Same webhook body keys; Sensor maps to annotation and **`submittedBy`** |
| **Horizon CLI / curl** | OIDC **`sub`** unless caller sets **`parameters.submittedBy`** | **`X-Horizon-Submitted-From`**: **`horizon-cli`** / **`api`** |
| **`argo submit` only** | Often empty → **`jenkins`** fallback for **`BUILD_USER_ID`** | May lack Horizon labels (not listed in Portal) |
| **Jenkins CVD Launcher job** | **`BUILD_USER_ID`** in Jenkins only | Not an Argo Workflow — does not appear in Portal |

**Portal (after horizon-dev-portal rollout):** **Running / History** show **Triggered from** and **Submitted by** ( **`—`** when unknown). All users see all Horizon-dispatched runs for the module (not filtered to “my” runs).

**kubectl:**

```bash
kubectl get wf <name> -n <prefix>workflows \
  -o jsonpath='{.metadata.annotations.horizon-sdv\.io/submitted-by}{"\n"}'
```

## MTK Connect

Default **`spec.mtkConnectSecretName`**: **`workflow-mtk-connect-apikey`** in the workflows namespace, with Secret keys **`username`** / **`password`**. The **`mtk-connect-post-job`** (`configure.sh`) creates it from the MTK admin service-account key; **`mtk-connect-api-key-config`** (`configure-key.py`) keeps it in sync when the API key rotates. Jenkins does not receive that admin key. The run pod maps those keys to **`MTK_CONNECT_USERNAME`** / **`MTK_CONNECT_PASSWORD`** via **`secretKeyRef`** (**`optional: true`** until the first MTK deploy/sync). Override **`mtkConnectUsernameKey`** / **`mtkConnectPasswordKey`** if your Secret uses different field names.

**Testbench user:** Guest script sets **`MTK_CONNECT_TESTBENCH_USER`** to **`everyone`** when **`mtkConnectPublic=true`**, else **`BUILD_USER_ID`** (from **`submittedBy`**). With the default public bench, submitter id does not affect MTK ACL until operators set **`mtkConnectPublic=false`**.

**Debug logs (search workflow / archived guest logs):**

- **`[cvd-argo] MTK env (workflow pod → guest):`** — **`MTK_CONNECT_PUBLIC`**, **`BUILD_USER_ID`**, resolved **`MTK_CONNECT_TESTBENCH_USER`** in job env uploaded to GCS.
- **`[cvd-argo-remote] MTK Connect --start:`** — same resolution on the guest before **`mtk_connect.sh`**.
- **`mtk_connect.sh`** — **`Environment:`** block and **`MTK Connect Testbench User:`** on success (password redacted).

## GitOps

Deployed as an extra Helm source on the **workloads-android** Module Manager child Application (same namespace as AAOS builder / CF templates).

## GCS staging (Argo vs Jenkins)

- **Argo WorkflowTemplate** default **`workflow.parameters.storageBucketDestination`**: parent prefix `gs://<spec.cloudProject>-aaos/Android/Tests/<spec.androidTestsWorkflowArtifactFolder>` (default folder **`CVD_Launcher_Workflows`**). Template **`compute-test-results-staging-uri`** resolves once per run to **`…/<YYYY-MM-DD-HHMMSS>_<workflow.name>`**, matching **aaos-builder** **`storage`** layout; ephemeral sync and **prepare-gemini** use that URI.
- **Jenkins** CVD Launcher may use **`STORAGE_BUCKET_DESTINATION`** / **`ANDROID_BUILD_BUCKET_ROOT_NAME`** under **`gs://…-aaos/…`** when operators configure it — not the same path as the workflow default.

Override the folder segment via GitOps **`spec.androidTestsWorkflowArtifactFolder`** only (not exposed as a WorkflowTemplate submit parameter).

## Gemini AI Review

When **`enableGeminiAiAssistant`** is **`true`**, the workflow runs **prepare** then **inline `gemini-review`** after the ephemeral VM stage (same **`run_ai_review.sh`** entrypoint as **aaos-builder** — no cluster **`ai-review`** WorkflowTemplateRef).

| DAG step | Template | Role |
|----------|----------|------|
| **`prepare-gemini-on-failure`** / **`prepare-gemini-on-success`** | **`prepare-gemini-cvd`** | Copies VM bundle into PVC **`gemini-test-results`** via **`gemini_argo_prepare_staging.sh`** (`GEMINI_PREPARE_MODE=cvd`). |
| **`gemini-ai-review-on-failure`** / **`gemini-ai-review-on-success`** | **`gemini-review`** | Runs **`workloads/common/agentic-ai/gemini/run_ai_review.sh`** with **`GEMINI_SKIP_MOVE_ARTIFACTS=1`**; post-hook **`cvd_launcher/hooks/review-post-cvd.sh`** uploads via **`gemini_storage.sh`**. |

Success-path steps require workflow parameter **`geminiAnalyseOnSuccess`** (default from Helm **`spec.geminiAnalyseOnSuccess`**; override per submit / webhook **`body.geminiAnalyseOnSuccess`**).

**Shared staging script:** [`../gemini_argo_prepare_staging.sh`](../gemini_argo_prepare_staging.sh) — merge **`cvd-argo-artifacts`**, unzip **`cuttlefish_logs-<BUILD_NUMBER>.zip`**, MTK-skip flag, optional GCS sync when review is skipped. See also [`../README.md`](../README.md).

**`templates/workflow/_gemini.tpl`** defines **`prepare-gemini-cvd`**, **`gemini-review`**, and DAG parameter wiring. PVC size: **`spec.geminiStagingStorageSize`** (default **`5Gi`**). **`Workflow.spec.securityContext.fsGroup: 1000`** keeps the PVC group-writable for the builder image user.

**MTK vs Gemini:** When MTK Connect **`--start`** fails, the driver mirrors **`mtk_connect_stage_failed`** under **`cvd-argo-artifacts/.meta/`**; prepare writes **`skipGemini=true`** and **`gemini-ai-review-*`** is skipped (no analysis). Full runs **exit non-zero** from the guest when MTK start fails so the ephemeral step fails like Jenkins.

**Gemini uploads (Argo):** **`gemini-review`** receives **`storageBucketDestination`** = **`{{tasks.compute-test-results-staging-uri.outputs.parameters.resolvedStagingUri}}/gemini-ai-review`** — same per-run prefix as VM/test staging (**`…-aaos/Android/Tests/CVD_Launcher_Workflows/<YYYY-MM-DD-HHMMSS>_<workflow.name>/`**), with a dedicated subfolder so **`gemini_storage.sh`** pre-upload cleanup does not remove sibling **`gcloud storage rsync`** artifacts. **`geminiArtifactRootName`** remains **`<cloudProject>-aaos`** for env/metadata helpers. Jenkins jobs use **`groovy/job.groovy`** copyArtifacts paths under **`…-aaos/Android/Tests/CVD_Launcher`** when not using this WorkflowTemplate.

**GCS upload (one `storage.sh` per run, Jenkins parity):** When **`enableGeminiAiAssistant`** is **`true`**, **`run-cvd-ephemeral-gce`** does **not** call **`cvd_argo_sync_vm_artifacts_to_staging.sh`** — uploads are **`sync-vm-staging-if-no-ai-review`** (Gemini on, no success review) or **`gemini_storage.sh`** after **`gemini-review`** (failure or **`geminiAnalyseOnSuccess`**). When Gemini is off, only the ephemeral step uploads.

## MTK offline cleanup

If the remote VM stage exits non-zero and MTK had run (**`/tmp/cvd-argo.marker`** on the guest), the driver runs **`mtk_connect.sh --delete`** with **`MTK_CONNECT_DELETE_OFFLINE_TESTBENCHES=true`** from the workflow pod **before** deleting the VM (Jenkins **Delete Offline Testbenches** parity).

## Ephemeral GCE (Path B — KCC + GCS)

**No pod SSH/IAP.** **`run-cvd-ephemeral-gce`** uploads job env + script bundle to GCS, applies a **`ComputeInstance`** CR, polls **`ephemeral-output/status.json`**, then deletes the CR. Guest **`cvd_argo_guest_startup.sh`** runs main + teardown on the VM.

| Helm value | Env / behavior |
|------------|----------------|
| **`spec.guestPollFirstSeconds`** (default **30**) | First GCS status poll delay |
| **`spec.guestPollSeconds`** (default **45**) | Poll interval |
| **`spec.kccInstanceWaitTimeout`** (default **20m**) | `kubectl wait` for KCC Ready |
| **`spec.gcpInstanceWaitTimeout`** (default **15m**) | Wait for zonal VM in Compute API before serial/logging |
| **`spec.guestSerialPort`** (default **2**) | `CVD_ARGO_SERIAL_PORT` — reads GCE serial port **2** (`/dev/ttyS1` app tee + any platform noise on that port) |
| **`spec.guestLogSink`** (default **serial**) | `serial` \| `cloud` \| `both` — live guest logs during poll ([details](../cvd_argo_gce/README.md)) |

Guest runtime logs: **x86** uses **serial port 2** by default (`getSerialPortOutput`); **`-arm64` templates** force **Cloud Logging** in the driver (`CVD_ARGO_LOG_SINK=cloud`) regardless of **`guestLogSink`**. See [cvd_argo_gce/README.md](../cvd_argo_gce/README.md). Jenkins console logs are **SSH + `~/cvd-*.log`**, not this stream.

**GCE service account:** ephemeral VMs use the **instance template’s** VM SA plus **`devstorage.read_write`** on the KCC spec. The workflow pod uses **`workflow-executor-elevated`** for **`kubectl`** on `computeinstances` and GCS. See [common/gcp/README.md](../../common/gcp/README.md).

**`spec.remoteUser`** (default **`jenkins`**) sets **`BUILD_USER`** in job env (MTK parity), not pod SSH.

## IAM

Uses **`workflow-executor-elevated`** when **`spec.useElevatedWorkflowIam: true`** (default): **`computeinstances`** RBAC + GCS on the staging bucket (not Compute REST VM insert from the pod).

## Usage

**Prerequisites:** Same as [tests/README.md](../README.md) (Argo, instance template, cloud env ConfigMap, elevated SA).

**Helm render (CI or local):**

```bash
helm template cvd-test workloads/android/pipelines/tests/cvd_launcher/helm \
  -f workloads/android/pipelines/tests/cvd_launcher/helm/values-local.yaml
```

**Submit (cluster; set project/zone via ConfigMap or chart `spec.*`):**

```bash
argo submit --from workflowtemplate/cvd-launcher-on-gce -n <prefix>workflows \
  -p instanceTemplateName=instance-template-cuttlefish-vm-main \
  -p cuttlefishDownloadUrl='gs://<bucket>/path/to/cvd-artifacts/' \
  -p enableGeminiAiAssistant=false
```

**Webhook:** Sensor **`webhook-cvd-launcher-on-gce`** — `workflowTemplateName: cvd-launcher-on-gce` in the dispatch body. Horizon API sets **`horizonSubmittedBy`**, **`horizonSubmittedFrom`**, and **`submittedBy`** (same resolved username when known). Sensor maps **`body.horizonSubmittedBy`** to annotation **`horizon-sdv.io/submitted-by`** and workflow parameter **`submittedBy`**.

**GitOps:** Chart is a Helm source on **`workloads-android`**; set **`spec.pipelineRepoUrl`** / **`spec.pipelineRepoRevision`** from module config (same pattern as aaos-builder). Do not rely on manual `kubectl apply` if Argo CD auto-sync is on.

**Further reading:** [tests/README.md](../README.md), [GCP helpers](../../common/gcp/README.md), [cvd_launcher.md](../../../../docs/workloads/android/tests/cvd_launcher.md).
