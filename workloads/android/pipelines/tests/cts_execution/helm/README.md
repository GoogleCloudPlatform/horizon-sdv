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

# CTS Execution (Argo Workflows)

Helm chart for **`cts-execution-on-gce`**: CTS on Cuttlefish using the same shell entrypoints as Jenkins (`cts_initialise.sh`, `cts_execution.sh`, `cvd_start_stop.sh`), on an **ephemeral GCE VM** from an **instance template**.

## Driver

Shared with CVD Launcher:

- [cvd_argo_gce/README.md](../cvd_argo_gce/README.md) — Path B operator reference.
- `workloads/android/pipelines/tests/cvd_argo_gce/cvd_argo_gce_ephemeral.sh`
- `workloads/android/pipelines/tests/cvd_argo_gce/cvd_argo_remote_entry.sh` (`CVD_ARGO_MODE=cts`; `run_cts_main` / `run_cts_teardown` via guest startup).

## Parameters (workflow submit)

| Parameter | Notes |
|-----------|--------|
| **`instanceTemplateName`** | GCE template name in GCP; run **`cf-instance-template-x86`** or **`-arm64`** first. |
| **`cuttlefishDownloadUrl`** | Folder with `cvd-host_package.tar.gz` and image zip (required for full run). |
| **`androidVersion`** | Selects **`/opt/android-cts_<ver>`** via runtime symlink (default `15`); CTS harness must be baked on the instance template (CF publish). |
| **`ctsTestplan`**, **`ctsModule`**, **`ctsRetryStrategy`**, **`ctsTimeout`** | Tradefed invocation (Jenkins parity). |
| **`numInstances`** | Cuttlefish instances (= **`SHARD_COUNT`** on guest). |
| **`mtkConnectEnable`** | When `true`, MTK before CTS (needs workflows MTK secret). |
| **`mtkConnectPublic`** | Default **`true`** — MTK testbench user **`everyone`**. Jenkins defaults private unless **`MTK_CONNECT_PUBLIC`** is checked. |
| **`submittedBy`** | Same as **`cvd-launcher`**: Portal **`authService.getUsername()`** → **`X-Horizon-Submitted-By`**; API → **`horizonSubmittedBy`** / pod **`BUILD_USER_ID`**. Empty → **`jenkins`**. |
| **`enableGeminiAiAssistant`**, **`geminiAnalyseOnSuccess`** | Gemini DAG (failure / optional success review). |

Deploy-time only (Helm **`spec.*`**, not submit UI): `pipelineRepoUrl`, `cloudProject`, `androidTestsWorkflowArtifactFolder`, Gemini model/resources — see **`values.yaml`**.

## Submitter identity (Portal / webhook)

Same model as **`cvd-launcher`**: Portal / Horizon API set **`horizon-sdv.io/submitted-by`** and **`submittedBy`** on the Workflow; Developer Portal lists **Triggered from** + **Submitted by** for all users with access. See **`cvd-launcher` chart README** (Submitter identity, MTK debug logs). Jenkins CTS jobs are not Argo workflows and do not appear in that list.

## Behaviour notes

- **Live logs:** same as [CVD Launcher](../cvd_launcher/helm/README.md#ephemeral-gce-path-b--kcc--gcs) — **serial** on x86; **Cloud Logging** forced for **`-arm64`** templates. See [cvd_argo_gce/README.md](../cvd_argo_gce/README.md).
- **CTS install path** is always **`/opt/android-cts`** on the VM (**`cts_environment.sh`** / **`cts_initialise.sh`**); not a workflow parameter (Jenkins and Argo parity).
- **List-only CTS** (`CTS_TEST_LISTS_ONLY`) is **Jenkins only**; Argo/webhook workflows always run full CTS on ephemeral GCE.
- **`mtkConnectEnable`**: when true, runs MTK Connect before CTS (and keep-alive + stop paths aligned with Jenkins `cvdPipeline` + `ctsCvdPipelineHooks`).

## Gemini AI Review

Same DAG pattern as **`cvd-launcher`**: **`prepare-gemini-on-*`** → **`gemini-ai-review-on-*`** using inline **`gemini-review`** (**`run_ai_review.sh`**, no cluster **`ai-review`** WorkflowTemplateRef), with **`cts_execution/prompt/sequenced`**.

| DAG step | Template | Role |
|----------|----------|------|
| **`prepare-gemini-on-failure`** / **`prepare-gemini-on-success`** | **`prepare-gemini-cts`** | **`gemini_argo_prepare_staging.sh`** with **`GEMINI_PREPARE_MODE=cts`** (Phase 0 suite file mirror + CVD unzip). |
| **`gemini-ai-review-on-*`** | **`gemini-review`** | **`run_ai_review.sh`**; post-hook **`cts_execution/hooks/review-post-cts.sh`**. |

Success-path steps require **`geminiAnalyseOnSuccess`** (Helm default **`spec.geminiAnalyseOnSuccess`**; webhook **`body.geminiAnalyseOnSuccess`**). Shared script and GCE driver docs: [`../../README.md`](../../README.md), **`cvd-launcher` chart README** for PVC/MTK skip/resource knobs and **single GCS upload** when Gemini is on (**`spec.geminiStagingStorageSize`**, **`spec.geminiPodResources`**, **`spec.geminiAiExecutionTimeoutHours`**).

**Gemini uploads (Argo):** **`gemini-review`** **`storageBucketDestination`** = **`{{tasks.compute-test-results-staging-uri.outputs.parameters.resolvedStagingUri}}/gemini-ai-review`** (same **`…-aaos/…/CTS_Execution_Workflows/<timestamp>_<workflow.name>/`** prefix as test staging). Jenkins **`cts_execution/groovy/job.groovy`** uses copyArtifacts paths under **`…-aaos/Android/Tests/CTS_Execution`** when not using this chart.

## GitOps & IAM

Same pattern as **`cvd-launcher`**: extra source on workloads-android Application, **`workflow-executor-elevated`** by default, MTK Secret **`workflow-mtk-connect-apikey`** / keys **`username`**/**`password`** (see **`cvd-launcher`** README). **`run-cts-ephemeral-gce`** uses Path B (KCC + GCS); see **`cvd-launcher`** README **Ephemeral GCE** section.

## GCS staging (Argo vs Jenkins)

- **Argo** default **`storageBucketDestination`**: parent `gs://<spec.cloudProject>-aaos/Android/Tests/<spec.androidTestsWorkflowArtifactFolder>` (default **`CTS_Execution_Workflows`**). **`compute-test-results-staging-uri`** appends **`/<YYYY-MM-DD-HHMMSS>_<workflow.name>`** (same idea as **aaos-builder** **`storage`**). Jenkins typically uses **`Android/Tests/CTS_Execution/<BUILD_NUMBER>`** on **`…-aaos`**.
- **Jenkins** CTS Execution commonly publishes under **`gs://<ANDROID_BUILD_BUCKET_ROOT_NAME>/Android/Tests/CTS_Execution/<BUILD_NUMBER>/`** when **`STORAGE_BUCKET_DESTINATION`** is left default (see job **`groovy/job.groovy`**).

Tune **`spec.androidTestsWorkflowArtifactFolder`** in Helm only. **`Sensor webhook-cts-execution-on-gce`** passes through **`body.storageBucketDestination`** when set; otherwise the WorkflowTemplate default applies.

## Usage

**Prerequisites:** [tests/README.md](../../README.md); baked **`/opt/android-cts_*`** on the instance template (required for Argo; Jenkins may still use **`CTS_DOWNLOAD_URL`** per job).

**Helm render:**

```bash
helm template cts-test workloads/android/pipelines/tests/cts_execution/helm \
  -f workloads/android/pipelines/tests/cts_execution/helm/values-local.yaml
```

**Submit (example):**

```bash
argo submit --from workflowtemplate/cts-execution-on-gce -n <prefix>workflows \
  -p instanceTemplateName=instance-template-cuttlefish-vm-main \
  -p cuttlefishDownloadUrl='gs://<bucket>/path/to/cvd-artifacts/' \
  -p androidVersion=15 \
  -p ctsTestplan=cts-system-virtual \
  -p enableGeminiAiAssistant=false
```

**Webhook:** Sensor **`webhook-cts-execution-on-gce`** — maps **`body.horizonSubmittedBy`** → annotation and **`submittedBy`** (same as CVD launcher).

**Further reading:** [tests/README.md](../README.md), [GCP helpers](../../common/gcp/README.md), Jenkins parameters in [cts_execution.md](../../../../docs/workloads/android/tests/cts_execution.md).
