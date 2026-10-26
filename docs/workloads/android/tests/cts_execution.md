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

# CTS Execution Pipeline

## Table of contents
- [Introduction](#introduction)
- [CTS vs CVD Launcher (scope)](#cts-vs-cvd-launcher-scope)
- [Jenkins pipeline and shared library hooks](#jenkins-pipeline-and-shared-library-hooks)
- [Prerequisites](#prerequisites)
- [Environment Variables/Parameters](#environment-variables)
  * [`ENABLE_GEMINI_AI_ASSISTANT` / Gemini prompts](#enable_gemini_ai_assistant)
  * [`GEMINI_ANALYSE_ON_SUCCESS`](#gemini_analyse_on_success)
- [Example Usage](#examples)
- [System Variables](#system-variables)
- [Known Issues](#known-issues)

## Introduction <a name="introduction"></a>

This pipeline is run on GCE Cuttlefish VM instances from the instance templates that were previously created by the environment pipeline. It allows users to run the Compatibility Test Suite (CTS) against their Cuttlefish virtual device (CVD) builds.

The pipeline first runs CVD on the Cuttlefish VM Instance to instantiate the specified number of devices and then runs CTS against the resulting virtual devices (tradefed - the tool used by CTS can spread / shard the tests across the multiple virtual devices).

Note:

- This pipeline offers the flexibility to run using a user-defined CTS suite (built by the `AAOS Builder` pipeline with `AAOS_BUILD_CTS` enabled) instead of the default Android 14, 15 and 16 CTS suites provided by google.
- It allows user to enable MTK Connect should they wish to view the virtual devices during testing (e.g. useful for UI tests).
- It allows users to keep the cuttlefish virtual devices alive for a certain amount of time after the CTS run has completed in order to facilitate debugging via MTK Connect. MTK Connect must be enabled for this option.
- To view Test Results in Jenkins with CSS, you may wish to lower the [content security level](https://www.jenkins.io/doc/book/security/configuring-content-security-policy/) from `Script Console`, allowing the full HTML to be accessible, e.g. `System.setProperty("hudson.model.DirectoryBrowserSupport.CSP", "")`

### Argo Workflows (`cts-execution-on-gce`) <a name="argo-workflows"></a>

Same ephemeral GCE driver as CVD Launcher (**`cvd_argo_gce_ephemeral.sh`**, **`CVD_ARGO_MODE=cts`**). Overview: [`tests/README.md`](../../../../workloads/android/pipelines/tests/README.md). Helm: [`cts_execution/helm/README.md`](../../../../workloads/android/pipelines/tests/cts_execution/helm/README.md). GCP helpers: [`common/gcp/README.md`](../../../../workloads/android/pipelines/common/gcp/README.md).

**WorkflowTemplate `cts-execution-on-gce`** (GitOps / **`argo submit`** / Sensor **`webhook-cts-execution-on-gce`**) defaults **`storageBucketDestination`** to **`gs://<project>-aaos/Android/Tests/CTS_Execution_Workflows`** and appends **`/<YYYY-MM-DD-HHMMSS>_<workflow.name>/`** per run (**aaos-builder** **`storage`** parity). **Jenkins** often uses **`gs://<ANDROID_BUILD_BUCKET_ROOT_NAME>/Android/Tests/CTS_Execution/<BUILD_NUMBER>/`** when **`STORAGE_BUCKET_DESTINATION`** is left at its job default.

**CTS on the guest** is always under **`/opt/android-cts`** (symlink to **`/opt/android-cts_<ANDROID_VERSION>`** from baked CF images or **`CTS_DOWNLOAD_URL`**); not a workflow parameter.

**Gemini:** When **`enableGeminiAiAssistant`** is true and not list-only, **`prepare-gemini-cts`** → **`gemini-review`** (**`run_ai_review.sh`**, preset **`cts`**); uploads via **`hooks/review-post-cts.sh`**. **`geminiAnalyseOnSuccess`** gates success-path review. MTK connect failure skips Gemini (Jenkins parity).

### CTS vs CVD Launcher (scope) <a name="cts-vs-cvd-launcher-scope"></a>

This job is the **CTS Execution** pipeline: it runs **Tradefed** against Cuttlefish virtual devices and publishes **Compatibility Test Suite** results (XML/HTML under `android-cts-results/` and `android-cts-results-html/`). When **Gemini AI Review** is enabled on a failed build, it uses **`preset: 'cts'`** and sequenced prompts that **prioritize failed tests from Tradefed** when suite artifacts exist, and **still analyze guest `kernel.log` and other CVD logs** for boot and bring-up issues (including when **devices never became healthy** and Tradefed left no usable summary—see [Gemini prompts and artifacts](#gemini-prompts-and-artifacts)). See `prompt/sequenced/README_SKILLS.md`.

The **[CVD Launcher](../cvd_launcher.md)** job is **not** a substitute for CTS: it does **not** run the Compatibility Test Suite. It exists to exercise **Cuttlefish runtime** (launch, MTK Connect, logs) without Tradefed. Use it when you need a **dedicated deep dive into CVD runtime issues**—guest `kernel.log`, host orchestration, boot failures, and related artifacts—without CTS test-result correlation. Its AI Review uses **`preset: 'cvd'`** and **guest-first** prompts tuned only for that scenario. If your question is purely “why did the virtual device fail to boot or misbehave at runtime,” CVD Launcher (or its documentation) is the clearer entry point; if your question is “which CTS tests failed and why,” use this CTS job.

**Resources:**

Ensure you select appropriate values for `NUM_INSTANCES`, `VM_CPUS`, `VM_MEMORY_MB` that align with the VM instance used for test, ie `JENKINS_GCE_CLOUD_LABEL`.

### Jenkins pipeline and shared library hooks <a name="jenkins-pipeline-and-shared-library-hooks"></a>

The job’s **`Jenkinsfile`** (`workloads/android/pipelines/tests/cts_execution/Jenkinsfile`) calls the shared library step **`cvdPipeline(...)`** from `cvd-pipeline-shared-library`. CTS-specific steps (list tests when in list-only mode, full CTS run after devices are up) are not inlined in the Jenkinsfile; they are supplied as **hook lists**:

- **`preLaunchStages`** — runs in the **Pre-launch** stage **before** **Launch Virtual Devices** (e.g. list test plans/modules when `CTS_TEST_LISTS_ONLY` is true).
- **`postMtkConnectStages`** — runs in the **Post-MTK Connect** stage **after** a successful **MTK Connect to Virtual Devices** step (or after launch when MTK is disabled), e.g. **`CTS execution`** with tradefed, result archives, and HTML report publishing.

Hook bodies are defined in **`ctsCvdPipelineHooks()`** (`workloads/common/jenkins/shared-libraries/cvd-pipeline-shared-library/vars/ctsCvdPipelineHooks.groovy`). The Jenkinsfile passes `preLaunchStages: ctsHooks.preLaunchStages` and `postMtkConnectStages: ctsHooks.postMtkConnectStages`.

[CVD Launcher](../cvd_launcher/README.md#jenkins-pipeline-and-shared-library) uses the same **`cvdPipeline`** without these hooks (Cuttlefish + MTK + keep-alive only). Full parameter and stage reference: **`workloads/common/jenkins/shared-libraries/cvd-pipeline-shared-library/vars/README.md`**.

### References <a name="references"></a>

- [Cuttlefish Virtual Devices](https://source.android.com/docs/devices/cuttlefish) for use with [CTS](https://source.android.com/docs/compatibility/cts) and emulators.
- [Compatibility Test Suite downloads](https://source.android.com/docs/compatibility/cts/downloads)

## Prerequisites<a name="prerequisites"></a>

One-time setup requirements.

- Before running this pipeline job, ensure that the following templates have been created by running the corresponding jobs:
  - Docker image template: `Android Workflows/Environment/Docker Image Template`
  - Cuttlefish instance template: `Android Workflows/Environment/CF Instance Template`
    - The CF job now uses a Packer-based build flow for template creation.
    - Script stage mapping is `1=build`, `2=ssh refresh`, `3=delete` (documented in `docs/workloads/android/environment/cf_instance_template.md`).
    - Must be rebuilt if using `CUTTLEFISH_INSTALL_WIFI` option, to ensure WiFi APK is stored with the image files.

## Environment Variables/Parameters <a name="environment-variables"></a>

**Jenkins Parameters:** Defined in the groovy job definition `groovy/job.groovy`.

### `JENKINS_GCE_CLOUD_LABEL`

This is the label that identifies the GCE Cloud label which will be used to identify the Cuttlefish VM instance, e.g.

- `cuttlefish-vm-main`
- `cuttlefish-vm-v1180`

Note: The value provided must correspond to a cloud instance or the job will hang.

### `CTS_TEST_LISTS_ONLY`
Skip running tests and simply list the available test plans and test modules.

### `CUTTLEFISH_DOWNLOAD_URL`

This is the Cuttlefish Virtual Device image that is to be tested. It is built from `AAOS Builder` for the `aosp_cf` build targets.

The URL must point to the bucket where the host packages and virtual devices images archives are stored:

- `cvd-host_package.tar.gz`
- `osp_cf_x86_64_auto-img-builder.zip`

- **Jenkins AAOS Builder:** `gs://<ANDROID_BUILD_BUCKET_ROOT_NAME>/Android/Builds/AAOS_Builder/<BUILD_NUMBER>/` (or `<STORAGE_BUCKET_DESTINATION>/` if overridden).
- **Argo WorkflowTemplate `aaos-builder`:** `gs://<project>-aaos/Android/Builds/AAOS_Builder_Workflows/<YYYY-MM-DD-HHMMSS>_<workflow.name>/` (Helm **`spec.androidBuildsArtifactFolder`**). Use the directory that contains the Cuttlefish host packages and image zip.

### `CUTTLEFISH_INSTALL_WIFI`

This allows the user to install Wifi utility APK on all Cuttlefish virtual devices.

### `ANDROID_VERSION`

Defines the Android and thus CTS version to use. Cuttlefish instance templates bake CTS under **`/opt/android-cts_<version>/android-cts`** (see **`cf_host_initialise.sh`**). At runtime, **`cts_initialise.sh`** symlinks **`/opt/android-cts`** to **`/opt/android-cts_<version>`** so Tradefed sees the standard layout **`/opt/android-cts/android-cts/{tools,jdk,results,...}`** (Jenkins and Argo; not configurable).

### `CTS_DOWNLOAD_URL`

Optional.

This allows the user to use their own CTS that was built using the `AAOS Builder` build job.

The URL must point to the bucket where the Android CTS archive is stored:

- `android-cts.zip`

- **Jenkins:** `gs://<ANDROID_BUILD_BUCKET_ROOT_NAME>/Android/Builds/AAOS_Builder/<BUILD_NUMBER>/android-cts.zip` (or `<STORAGE_BUCKET_DESTINATION>/android-cts.zip` if overridden).
- **Argo `aaos-builder`:** `gs://<project>-aaos/Android/Builds/AAOS_Builder_Workflows/<YYYY-MM-DD-HHMMSS>_<workflow.name>/android-cts.zip` when CTS was produced by the workflow **`storage`** step under that prefix.

When set, the archive is unpacked under **`/opt/android-cts`**, replacing that directory’s contents for that job. If the zip layout is flat (**`tools/`** at the root), **`cts_initialise.sh`** nests it under **`android-cts/`** so Tradefed paths match the baked image.

### `CTS_TESTPLAN`

Mandatory.

This defines the CTS test plan that will be run. Default is: `cts-system-virtual` which is only available in Android 15 and 16.

Android 14 users should pick a test plan that is compatible with their version of Cuttlefish, e.g `cts-virtual-device-stable`.

### `CTS_MODULE`

Optional.

This defines the CTS test module that will be run, e.g. Android 14 `CtsHostsideNumberBlockingTestCases`, Android 15 and later, `CtsDeqpTestCases` but if field is left empty, all CTS test modules will be run.

### `CTS_RETRY_STRATEGY`

Default: `RETRY_ANY_FAILURE`

Refer to [`--retry-strategy`](https://source.android.com/reference/tradefed/com/android/tradefed/retry/RetryStrategy).

### `CTS_MAX_TESTCASE_RUN_COUNT`

Default: `2`

Option is dependent on `CTS_RETRY_STRATEGY`, refer to [`--max-testcase-run-count`](https://source.android.com/docs/core/tests/tradefed/testing/through-tf/auto-retry).

### `CUTTLEFISH_MAX_BOOT_TIME`

Cuttlefish virtual devices need time to boot up. This defines the maximum time to wait for the virtual device(s) to boot up. Cuttlefish virtual devices can take a serious amount of time before booting, hence this is quite large.

Time is in seconds.

### `NUM_INSTANCES`

Defines the number of Cuttlefish virtual devices to run CTS against.

This applies to CVD `num-instances` and CTS `shards` parameters.

### `VM_CPUS`

Defines the number of CPU cores to allocate to the Cuttlefish virtual device.

This applies to CVD `cpus` parameter.

### `VM_MEMORY_MB`

Defines total memory available to guest.

This applies to CVD `memory_mb` parameter.

### `CTS_TIMEOUT`

This defines the maximum time, in minutes, to wait for CTS to complete.

### `MTK_CONNECT_ENABLE`

Enable if user wishes to view devices via MTK Connect (e.g. to watch UI tests).

### `MTK_CONNECT_PUBLIC`

When checked, the MTK Connect testbench is visible to everyone and can be shared.
By default, testbenches are private and only visible to their creator and MTK Connect administrators.

### `MTK_CONNECT_TUNNEL_PORT`

ADB tunnel **`caller.port`** for MTK Connect testbench creation (`workloads/common/mtk-connect/create-testbench.js`). Default **8555**; override if the port conflicts on the agent. Used when MTK Connect runs (`MTK_CONNECT_ENABLE`). Same behavior as documented for [CVD Launcher](../cvd_launcher/README.md#mtk_connect_tunnel_port).

### `CUTTLEFISH_KEEP_ALIVE_TIME`

If wishing to debug HOST using MTK Connect, Cuttlefish VM instance must be allowed to continue to run. This timeout, in
minutes, gives the tester time to keep the instance alive so they may work with the host via MTK Connect.

It is only applicable when `MTK_CONNECT_ENABLE` is enabled.

### `CVD_COMMAND_LINE`

Same semantics as in [CVD Launcher](../cvd_launcher/README.md#cvd_command_line): default is the full `/usr/bin/cvd create …` line with shell placeholders `${NUM_INSTANCES}`, `${VM_CPUS}`, `${VM_MEMORY_MB}` and CI-oriented flags `--setupwizard_mode DISABLED`, `--enable_host_bluetooth false`, `--gpu_mode guest_swiftshader` (those values derive from the respective job parameters automatically); an empty value clears to the script default. Edit the parameter to change launch arguments.

Optional `cvd` flags are **not** a separate parameter: add them **inside** the `CVD_COMMAND_LINE` string. Further example fragments (see [CVD Launcher — `CVD_COMMAND_LINE`](../cvd_launcher/README.md#cvd_command_line) for context):

- `--display0=width=1920,height=1080,dpi=160`
- `--verbosity=DEBUG`

### `ENABLE_GEMINI_AI_ASSISTANT` <a name="enable_gemini_ai_assistant"></a>

Enable Gemini **AI Review** in the shared `cvd-pipeline-shared-library` **Diagnostics** stage (`cvdPipeline`). The **AI Review** sub-stage runs only when **all** of these are true (see `cvdPipeline.groovy`): **`ENABLE_GEMINI_AI_ASSISTANT`** is **`true`**, the overall pipeline result is **`FAILURE`**, **`MTK_CONNECT_STAGE_FAILED`** is not **`true`**, and—because this job’s **`Jenkinsfile`** passes **`aiReview`** with **`requireCtsNotListOnly: true`**—**`CTS_TEST_LISTS_ONLY`** is **`false`** (list-only / plan-discovery runs skip AI Review).

### `GEMINI_ANALYSE_ON_SUCCESS` <a name="gemini_analyse_on_success"></a>

Optional. Default **`false`**.

When **`true`**, the shared pipeline’s **Diagnostics → AI Review** stage is allowed to run even when the overall pipeline result is **`SUCCESS`**. This is useful to analyze CTS/CVD logs for hidden / underlying issues without forcing a failing build.

Notes:

- AI Review still requires **`ENABLE_GEMINI_AI_ASSISTANT=true`** and still skips when `CTS_TEST_LISTS_ONLY` is `true` (`requireCtsNotListOnly`).
- When **`GEMINI_ANALYSE_ON_SUCCESS=true`**, a failing Gemini step in AI Review can still mark the overall job **`FAILURE`** (same `catchError` behaviour as when AI Review runs after a failed pipeline).
- **Offline CVD analysis via Gemini AI Assistant (success or failure):** to investigate CVD/Cuttlefish behaviour for a CTS run that has already completed — whether tests passed, tests failed, or devices never booted — use **`docs/workloads/utilities/gemini_ai_assistant.md`** (**Workloads → Utilities → Gemini AI Assistant**) with the **CVD Launcher** sequenced prompts and `skills.yaml` from `workloads/android/pipelines/tests/cvd_launcher/prompt/sequenced/` (**not** the CTS Execution set; the CVD Launcher prompts cover both the boot-failure lane and the runtime-health lane via Phase 0). Point **`GEMINI_ARTIFACTS_COMMAND`** at the CTS Execution build's archived Cuttlefish/CVD artifacts (host `cvd*.log`, unpacked `cuttlefish_logs*.zip`, guest `kernel.log`/`launcher.log`/`logcat`, `wifi*.log`), then upload the three CVD Launcher prompts and matching **`GEMINI_SKILLS_YAML`**. **Phase 0 — CVD boot preflight** classifies `CVD_STATUS` from artifacts only (no pipeline state) and auto-routes to boot-failure triage on failed boots or runtime-health analysis on booted devices — leaving CTS suite/plan semantics to the normal CTS Execution AI Review.

### Gemini prompts and artifacts <a name="gemini-prompts-and-artifacts"></a>

The job uses prompt files from the repository only; there is no Jenkins parameter to override the default path. Sequenced prompts (order matters), under `workloads/android/pipelines/tests/cts_execution/prompt/sequenced/`: `step1_triage.txt`, `step2_rca.txt`, `step3_fixes.txt`. Outputs: `step1_output.md`, `step2_output.md`, `step3_output.md`. Skills are defined in `skills.yaml` (`triage-cts`, `rca-cts`, `fix-cts`); see `workloads/android/pipelines/tests/cts_execution/prompt/sequenced/README_SKILLS.md` for how `gemini_initialise.sh` loads them. [CVD Launcher](../cvd_launcher.md#gemini-prompts-and-ai-review) uses a separate `cvd_launcher/prompt/sequenced` set when `preset: 'cvd'`—that preset is **dedicated to Cuttlefish/CVD runtime** triage without CTS suite semantics; see [CTS vs CVD Launcher (scope)](#cts-vs-cvd-launcher-scope) above.

**CVD logs when devices do not boot:** On failure, AI Review still receives **Cuttlefish-oriented** artifacts (`**/cvd*.log`, **`cuttlefish_logs*.zip`**, guest logs under **`test-results/cvd/**` after unpack, Wi‑Fi logs, etc.) alongside any CTS XML/HTML. Skills **always** treat guest **`kernel.log`** as high signal: when Tradefed produced a failure report, triage correlates **failed tests** but still runs **early-boot / bootconfig-oriented** greps on **`kernel.log`** (those errors usually do **not** contain testcase names). When **no** usable Tradefed failure summary exists—typical if virtual devices **never came up** or CTS aborted before writing results—the skills follow a **CVD-only** path aligned with [CVD Launcher](../cvd_launcher.md) (guest-first analysis and optional follow-up to run a **CVD Launcher** job with **`preset: 'cvd'`** for a dedicated boot/runtime pass). Details: `prompt/sequenced/README_SKILLS.md`.

Cuttlefish/CVD line-number grep guidance (example failure buckets, log paths, and substrings, plus what to do when the failure is non-obvious) is under `skills.yaml` → `global_constraints` → **“CVD errors — what to do”**. That block is aligned with [CVD Launcher](../cvd_launcher.md#gemini-prompts-and-ai-review) and applies on this job’s **CVD-only** triage path when Tradefed suite artifacts are missing.

For AI Review, the shared library copies artifacts from the current build using the **CTS Execution** filter: `android-cts-results/**`, `android-cts-results-html/**`, plus shared Cuttlefish-oriented patterns (`**/wifi*.log`, `**/cvd*.log`, `**/cts_execution_parameters.txt`, `**/cuttlefish_logs*.zip`). The canonical filter list lives with **`cvdPipeline`** / **`aiReview`** in `workloads/common/jenkins/shared-libraries/cvd-pipeline-shared-library/vars/` (see that README for implementation pointers).

### `GEMINI_COMMAND_LINE`

Interface for the headless [gemini-cli](https://geminicli.com/docs/cli/headless/).
Use this to specify settings such as the [Gemini model](https://ai.google.dev/gemini-api/docs/models) etc, e.g.
`--debug` to include debug output.
Note: Prompts are piped via `stdin` and output is redirected to a JSON file.

## Example Usage <a name="examples"></a>

Refer to `docs/workloads/android/tests/cvd_launcher.md` for an example of how to create and set up a test instance and boot the Cuttlefish Virtual Devices. Once the devices are booted, CTS tests can be run as follows:

```
ANDROID_VERSION=14 \
./workloads/android/pipelines/tests/cts_execution/cts_initialise.sh
# CTS is always under /opt/android-cts (symlink to /opt/android-cts_${ANDROID_VERSION} when using baked CF images).
CTS_TESTPLAN="cts-system-virtual" \
CTS_MODULE="CtsDeqpTestCases" \
CTS_TIMEOUT=600 \
SHARD_COUNT=1 \
./workloads/android/pipelines/tests/cts_execution/cts_execution.sh
```

## SYSTEM VARIABLES <a name="system-variables"></a>

There are a number of system environment variables that are unique to each platform but required by Jenkins build, test and environment pipelines.

These are defined in Jenkins CasC `values-jenkins.yaml` and can be viewed in Jenkins UI under `Manage Jenkins` -> `System` -> `Global Properties` -> `Environment variables`.

These are as follows:

-   `ANDROID_BUILD_BUCKET_ROOT_NAME`
     - Defines the name of the Google Storage bucket that will be used to store build and test artifacts

-   `ANDROID_BUILD_DOCKER_ARTIFACT_PATH_NAME`
    - Defines the registry path where the Docker image used by builds, tests and environments is stored.

-   `CLOUD_PROJECT`
    - The GCP project, unique to each project. Important for bucket, registry paths used in pipelines.

-   `CLOUD_REGION`
    - The GCP project region. Important for bucket, registry paths used in pipelines.

-   `CLOUD_ZONE`
    - The GCP project zone. Important for bucket, registry paths used in pipelines.

-   `HORIZON_DOMAIN`
    - The URL domain which is required by pipeline jobs to derive URL for tools and GCP.

-   `HORIZON_SCM_URL`
    - The URL to the Horizon SDV git repository.

-   `HORIZON_SCM_BRANCH`
    - The branch name the job will be configured for from `HORIZON_SCM_URL`.

-   `JENKINS_SERVICE_ACCOUNT`
    - Service account to use for pipelines. Required to ensure correct roles and permissions for GCP resources.

## KNOWN ISSUES <a name="known-issues"></a>

Refer to `docs/workloads/android/tests/cvd_launcher.md` for details.
