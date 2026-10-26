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

# CVD Launcher Pipeline

## Table of contents
- [Introduction](#introduction)
- [Jenkins pipeline and shared library](#jenkins-pipeline-and-shared-library)
- [Prerequisites](#prerequisites)
- [Environment Variables/Parameters](#environment-variables)
  * [`CVD_COMMAND_LINE`](#cvd_command_line)
  * [`ENABLE_GEMINI_AI_ASSISTANT`](#enable_gemini_ai_assistant)
  * [`GEMINI_ANALYSE_ON_SUCCESS`](#gemini_analyse_on_success)
  * [Gemini prompts and AI Review](#gemini-prompts-and-ai-review)
- [Example Usage](#examples)
- [System Variables](#system-variables)
- [Known Issues](#known-issues)

## Introduction <a name="introduction"></a>

This pipeline is run on GCE Cuttlefish VM instances from the instance templates that were previously created by the environment pipeline. It allows users to test their Cuttlefish virtual device (CVD) image builds.

The pipeline first runs CVD on the Cuttlefish VM Instance to instantiate the specified number of virtual devices and then connects to MTK Connect so that users can test their builds (UI and adb). Devices are kept alive for the user-specified amount of time.

When **Gemini AI Review** is enabled and the build **fails**, the **Diagnostics** stage can run **AI Review** (`preset: 'cvd'`) over archived **Cuttlefish/CVD logs** to surface **boot, launch, and runtime** issues—for example when devices **do not boot** or **never stabilize**—using guest **`kernel.log`**, **`launcher.log`**, and related artifacts (see [Gemini prompts and AI Review](#gemini-prompts-and-ai-review)).

### Jenkins pipeline and shared library <a name="jenkins-pipeline-and-shared-library"></a>

The job **`Jenkinsfile`** (`workloads/android/pipelines/tests/cvd_launcher/Jenkinsfile`) calls **`cvdPipeline`** with **`aiReview`** (`preset: 'cvd'`) only — it does **not** pass **`preLaunchStages`** or **`postMtkConnectStages`**. Those optional hooks are used by [CTS Execution](../cts_execution/README.md#jenkins-pipeline-and-shared-library-hooks) to plug list-tests and CTS run stages into the same shared pipeline.

See **`workloads/common/jenkins/shared-libraries/cvd-pipeline-shared-library/vars/README.md`** for stage order, `config` keys, and **`ctsCvdPipelineHooks`**.

### Argo Workflows (`cvd-launcher-on-gce`) <a name="argo-workflows"></a>

Ephemeral Cuttlefish runs on GCE via **`workloads/android/pipelines/tests/cvd_argo_gce/`** (Path B: KCC + GCS — see [`tests/README.md`](../../../../workloads/android/pipelines/tests/README.md)). Helm chart: [`cvd_launcher/helm/README.md`](../../../../workloads/android/pipelines/tests/cvd_launcher/helm/README.md). CF GCP helpers: [`common/gcp/README.md`](../../../../workloads/android/pipelines/common/gcp/README.md).

**WorkflowTemplate `cvd-launcher-on-gce`** (GitOps / **`argo submit`** / Sensor **`webhook-cvd-launcher-on-gce`**) defaults **`storageBucketDestination`** to **`gs://<project>-aaos/Android/Tests/CVD_Launcher_Workflows`**, with per-run suffix **`/<YYYY-MM-DD-HHMMSS>_<workflow.name>/`** from **`compute-test-results-staging-uri`**.

When **Gemini AI Review** is enabled, **`prepare-gemini-cvd`** runs **`gemini_argo_prepare_staging.sh`**, then **`gemini-review`** invokes **`workloads/common/agentic-ai/gemini/run_ai_review.sh`** (same pattern as **aaos-builder**, not a cluster **`ai-review`** template). Uploads use **`hooks/review-post-cvd.sh`** under **`…/gemini-ai-review/`**. When Gemini is off or success review is disabled, **`sync-vm-staging-if-no-ai-review`** still uploads VM artifacts to GCS.

### References <a name="references"></a>

- [Cuttlefish Virtual Devices](https://source.android.com/docs/devices/cuttlefish)
- [Android Cuttlefish](https://github.com/google/android-cuttlefish)

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

### `CUTTLEFISH_DOWNLOAD_URL`

This is the Cuttlefish Virtual Device image that is to be tested. It is built from `AAOS Builder` (or an SDV Cuttlefish build) for `aosp_cf*` or `sdv_*_cf*` lunch targets.

The URL must point to the bucket (or HTTP directory) where the host package and **device image zip(s)** are stored:

- `cvd-host_package.tar.gz` (required)
- At least one Cuttlefish guest image zip whose name matches **`*img*.zip`** (e.g. `aosp_cf_*_auto-img*.zip` or `sdv_*_cf*-img*.zip` from AAOS / SDV uploads).

For **`gs://`** URLs, the launcher runs `gcloud storage cp` with remote wildcard `*img*.zip` into the working directory (original object basenames). For non-gs URLs, `wget` uses the same `*img*.zip` accept pattern. After `cvd-host_package.tar.gz` is unpacked, every local `*img*.zip` is unzipped and then removed.

URL is of the form `gs://<ANDROID_BUILD_BUCKET_ROOT_NAME>/Android/Builds/AAOS_Builder/<BUILD_NUMBER>` where `ANDROID_BUILD_BUCKET_ROOT_NAME` is a system environment variable defined in Jenkins CasC `values-jenkins.yaml` and `BUILD_NUMBER` is the Jenkins build number. Alternatively, `<STORAGE_BUCKET_DESTINATION>` if destination was overridden (including paths under `AAOS SDV Builder` or similar jobs).

### `CUTTLEFISH_INSTALL_WIFI`

This allows the user to install Wifi utility APK on all Cuttlefish virtual devices.

### `CUTTLEFISH_MAX_BOOT_TIME`

Cuttlefish virtual devices need time to boot up. This defines the maximum time to wait for the virtual device(s) to boot up. Cuttlefish virtual devices can take a serious amount of time before booting, hence this is quite large.

Time is in seconds.

### `CUTTLEFISH_KEEP_ALIVE_TIME`

If wishing to test using MTK Connect, Cuttlefish VM instance must be allowed to continue to run. This timeout, in
minutes, gives the tester time to keep the instance alive so they may work with the devices via MTK Connect.

### `NUM_INSTANCES`

Defines the number of Cuttlefish virtual devices to launch.

This applies to CVD `num-instances` parameters.

### `VM_CPUS`

Defines the number of CPU cores to allocate to the Cuttlefish virtual device.

This applies to CVD `cpus` parameter.

### `VM_MEMORY_MB`

Defines total memory available to guest.

This applies to CVD `memory_mb` parameter.

### `MTK_CONNECT_PUBLIC`

When checked, the MTK Connect testbench is visible to everyone and can be shared.
By default, testbenches are private and only visible to their creator and MTK Connect administrators.

### `MTK_CONNECT_TUNNEL_PORT` <a name="mtk_connect_tunnel_port"></a>

TCP port passed into MTK Connect as the ADB tunnel **`caller.port`** when creating the testbench (`workloads/common/mtk-connect/create-testbench.js`). Default **8555**. Change it if that port is already in use on the agent. The shared library passes this job parameter through to `mtk_connect.sh` as the environment variable of the same name; the shell script and Node script both default to **8555** when unset.

### `CVD_COMMAND_LINE` <a name="cvd_command_line"></a>

Full Cuttlefish launch command: the binary plus all arguments. The start script runs it with `sudo HOME=<working directory>` and log redirection.

- **Default (Jenkins and `cvd_environment.sh`):**

  `/usr/bin/cvd create --noresume -config=auto -report_anonymous_usage_stats=no --num_instances="${NUM_INSTANCES}" --cpus="${VM_CPUS}" --memory_mb="${VM_MEMORY_MB}" --console=true --setupwizard_mode DISABLED --enable_host_bluetooth false --gpu_mode guest_swiftshader`

  The `${NUM_INSTANCES}`, `${VM_CPUS}`, and `${VM_MEMORY_MB}` fragments are **shell** placeholders: they are expanded when the start script runs `eval` on the launch command, using values that **derive from the job parameters** of the same names. They are **not** Groovy interpolations—the job DSL stores that string literally in the parameter default.

  The trailing flags target **typical CI**: non-interactive boot (`--setupwizard_mode DISABLED`), no host Bluetooth (`--enable_host_bluetooth false`), and **software** guest rendering (`--gpu_mode guest_swiftshader`) when agents do not use GPU passthrough. Confirm supported flag names against your Cuttlefish package version.

- **Empty:** if you clear the parameter so it is empty after trim, `cvd_environment.sh` sets the same default as above.

- **Override:** set a different full command line to change any `cvd create` arguments (for example host GPU modes, display geometry, or verbosity).

Optional flags are not a separate Jenkins field: **include them in the `CVD_COMMAND_LINE` value** together with the `cvd create` binary and any other arguments you need.

Examples:

AAOS:

- `/usr/bin/cvd create --display0=width=1920,height=1080,dpi=160`
- `/usr/bin/cvd create --verbosity=DEBUG`

AAOS SDV:

- `./sdv-cf create --noresume -config=auto -report_anonymous_usage_stats=no --num_instances=2 --cpus=4 --memory_mb=8192 --console=true`
- `/usr/bin/cvd create --noresume … --setupwizard_mode DISABLED --gpu_mode guest_swiftshader`

### `ENABLE_GEMINI_AI_ASSISTANT` <a name="enable_gemini_ai_assistant"></a>

Enable Gemini **AI Review** in the shared `cvd-pipeline-shared-library` **Diagnostics** stage (`cvdPipeline`, same family as [CTS Execution](../cts_execution/README.md)). The **AI Review** sub-stage runs only when **`ENABLE_GEMINI_AI_ASSISTANT`** is **`true`**, the overall pipeline result is **`FAILURE`**, and **`MTK_CONNECT_STAGE_FAILED`** is not **`true`** (see `cvdPipeline.groovy`). This job does **not** use **`requireCtsNotListOnly`** (that gate applies only to CTS Execution).

The **`Jenkinsfile`** sets **`aiReview`** with **`preset: 'cvd'`**: artifact collection matches CTS Execution’s Cuttlefish/host patterns but **does not** include CTS result trees (`android-cts-results/**`, `android-cts-results-html/**`), which do not apply to this job.

### `GEMINI_ANALYSE_ON_SUCCESS` <a name="gemini_analyse_on_success"></a>

Optional. Default **`false`**.

When **`true`**, the shared pipeline’s **Diagnostics → AI Review** stage is allowed to run even when the overall pipeline result is **`SUCCESS`**. This is useful to analyze CVD logs for hidden / underlying issues without forcing a failing build.

The sequenced prompts and **`triage-cvd` / `rca-cvd` / `fix-cvd`** skills recognise both outcomes via a **Phase 0 — CVD boot preflight**. Triage emits a **`## CVD boot preflight`** header with **`CVD_STATUS`** (`BOOT_OK` / `BOOT_FAILED` / `BOOT_UNKNOWN`) derived from `"status": "Running"` in host CVD JSON logs and **`VIRTUAL_DEVICE_BOOT_COMPLETED`** per guest. On **`BOOT_OK`**, bootconfig warnings are **informational** and runtime/logcat findings are **primary**; if no actionable issue is found, Step 1 emits a single **`[CVD_HEALTHY]`** row, Step 2 confirms no root cause, and Step 3 writes a single observations note (or returns `FIX_UNKNOWN`). On **`BOOT_FAILED`** / **`BOOT_UNKNOWN`**, the existing boot-first priority is preserved.

**Offline analysis via Gemini AI Assistant (success or failure).** The [Utilities / Gemini AI Assistant](../utilities/gemini_ai_assistant.md) job can run the **same** CVD Launcher sequenced prompts and `skills.yaml` against archived `test-results/` for any previous CVD Launcher **or** CTS Execution build — pipeline **passed or failed**, devices **booted or not**. Always use the **CVD Launcher** prompts (not the CTS Execution set) whenever the focus is CVD / Cuttlefish behaviour: the same prompts cover both the boot-failure lane and the runtime-health lane via Phase 0. Upload `step1_triage.txt`, `step2_rca.txt`, `step3_fixes.txt`, and `skills.yaml` from `workloads/android/pipelines/tests/cvd_launcher/prompt/sequenced/`, and set `GEMINI_ARTIFACTS_COMMAND` to the archived results location (e.g. `gcloud storage cp -r gs://.../<BUILD_NUMBER>/test-results/ .`). Phase 0 classifies `CVD_STATUS` from artifacts only (no pipeline state) and routes triage to the correct lane — useful both when a failed pipeline needs deeper CVD triage and when a passed pipeline masked questionable device behaviour.

Notes:

- AI Review still requires **`ENABLE_GEMINI_AI_ASSISTANT=true`**.
- When **`GEMINI_ANALYSE_ON_SUCCESS=true`**, a failing Gemini step in AI Review can still mark the overall job **`FAILURE`** (same `catchError` behaviour as when AI Review runs after a failed pipeline).
- **Post-hoc analysis of a successful boot (recommended):** rather than re-running the pipeline with `GEMINI_ANALYSE_ON_SUCCESS=true`, use [**Workloads → Utilities → Gemini AI Assistant**](../utilities/gemini_ai_assistant.md) against the archived artifacts. Point **`GEMINI_ARTIFACTS_COMMAND`** at the CVD Launcher build's Cuttlefish/CVD artifacts (e.g. `gcloud storage cp -r gs://<bucket>/<path>/ .`), upload the three CVD Launcher sequenced prompts as **`GEMINI_PROMPT_FILE`** / **`GEMINI_PROMPT_FILE_2`** / **`GEMINI_PROMPT_FILE_3`** from `workloads/android/pipelines/tests/cvd_launcher/prompt/sequenced/step{1,2,3}_*.txt`, and upload the matching **`skills.yaml`** as **`GEMINI_SKILLS_YAML`**. The **Phase 0 — CVD boot preflight** will classify the run as **`BOOT_OK`** and route Gemini into the runtime-health lane, highlighting logcat/WiFi/late-kernel issues observed on booted devices without re-running the build.

### `GEMINI_COMMAND_LINE`

Interface for the headless [gemini-cli](https://geminicli.com/docs/cli/headless/). Same role as on CTS Execution; defaults are seeded in `groovy/job.groovy` (including optional `GEMINI_MODEL` via job environment).

### `CTS_ARTIFACT_STORAGE_SOLUTION`

Where to upload Gemini outputs after analysis (for example `GCS_BUCKET`, or empty to skip upload). Same variable name as CTS Execution for shared scripts.

### `STORAGE_BUCKET_DESTINATION`

Optional override for Gemini artifact destination; align with [CTS Execution](../cts_execution/README.md) if you use bucket overrides there.

### `STORAGE_LABELS`

Optional GCS object metadata labels for stored Gemini artifacts.

### Gemini prompts and AI Review <a name="gemini-prompts-and-ai-review"></a>

Default sequenced prompts and `skills.yaml` for this job live under `workloads/android/pipelines/tests/cvd_launcher/prompt/sequenced/` (CVD host/guest/kernel/logcat-focused skills: `triage-cvd`, `rca-cvd`, `fix-cvd`). See `workloads/android/pipelines/tests/cvd_launcher/prompt/sequenced/README_SKILLS.md` for loading behavior. [CTS Execution](../cts_execution/README.md) continues to use `cts_execution/prompt/sequenced` for suite + CVD correlation.

Cuttlefish/CVD line-number grep guidance (example failure buckets, log paths, and substrings, plus what to do when the failure is non-obvious) is under `skills.yaml` → `global_constraints` → **“CVD errors — what to do”**; the same subsection is duplicated under [CTS Execution](../cts_execution/README.md#gemini-prompts-and-artifacts) `skills.yaml` for **CVD-only** parity.

**CVD logs and devices that do not boot:** This job does **not** run Tradefed. When Gemini is enabled and the build **fails**, AI Review analyzes the copied Cuttlefish/CVD material to explain **launch and runtime** problems—especially **guest `kernel.log`** (bootconfig, panics, early boot), **`launcher.log`** (crosvm / assemble), workspace **`cvd*.log`**, Wi‑Fi logs, and **`cuttlefish_logs*.zip`**—not Compatibility Test assertions. That is the right diagnostic path when virtual devices **fail to boot**, **fail to stabilize**, or **never become usable** before any CTS run would matter. For failures that also require **which CTS tests failed** and Tradefed HTML/XML, use [CTS Execution](../cts_execution/README.md) (`preset: 'cts'`).

For AI Review, the shared library copies artifacts using the **CVD** filter: `**/wifi*.log`, `**/cvd*.log`, `**/cts_execution_parameters.txt`, `**/cuttlefish_logs*.zip`. The job definition grants **Copy Artifact** permission on itself so the Diagnostics stage can copy from the same build.

## Example Usage <a name="examples"></a>

The following examples show how the scripts may be used standalone on a test instance.

From `Workloads/Android/Environment/CF Instance Template` create a Cuttlefish test instance:

- `ANDROID_CUTTLEFISH_REVISION`: choose the version you wish to build the template from
- `CUTTLEFISH_INSTANCE_NAME` : provide a unique name, starting with cuttlefish-vm, e.g. `cuttlefish-vm-test-instance-v110.`
- `MAX_RUN_DURATION` : set to 0 to avoid instance being deleted after this time.

Connect to the instance, e.g.

```

# Set up fleet management:
gcloud container fleet memberships list
# sdv-cluster may be default but derive the membership name from list
gcloud container fleet memberships get-credentials sdv-cluster

# If user wishes to use MTK Connect then retrieve the MTK Connect API key:
# Retrieve the MTK_CONNECT_USERNAME:
kubectl get secrets -n mtk-connect mtk-connect-apikey -o json | jq -r '.data.username' | base64 -d
# Retrieve the MTK_CONNECT_PASSWORD:
kubectl get secrets -n mtk-connect mtk-connect-apikey -o json | jq -r '.data.password' | base64 -d

# Start the instance
gcloud compute instances start cuttlefish-vm-test-instance-v110 --zone=europe-west1-d
# Connect to the instance
gcloud compute ssh --zone "europe-west1-d" "cuttlefish-vm-test-instance-v110" --tunnel-through-iap --project "sdva-2108202401"
```
**Authentication Required:** You may be prompted to authenticate during this process. To complete the authentication, follow the on-screen instructions or run `gcloud auth login`.

Once you have access to the instance, follow these steps:

- Clone the Horizon SDV repository on the instance.
- Run the CVD Launcher scripts as per the following examples.

```
CUTTLEFISH_DOWNLOAD_URL="gs://sdva-2108202401-aaos/Android/Builds/AAOS_Builder/10/" \
CUTTLEFISH_MAX_BOOT_TIME=180 \
NUM_INSTANCES=1 \
VM_CPUS=16 \
VM_MEMORY_MB="16384" \
./workloads/android/pipelines/tests/cvd_launcher/cvd_start_stop.sh --start
```

Users should stop CVD and devices with the following command when complete:
```
./workloads/android/pipelines/tests/cvd_launcher/cvd_start_stop.sh --stop
```

**MTK Connect:**

Users may optionally connect devices to MTK Connect in order to utilise the UI. Ensure the devices are running before
following the instructions below.

```
# Start MTK Connect (use the credentials from earlier)
cd ./workloads/common/mtk-connect/
sudo \
MTK_CONNECT_DOMAIN="dev.horizon-sdv.com" \
MTK_CONNECT_USERNAME=${MTK_CONNECT_USERNAME} \
MTK_CONNECT_PASSWORD=${MTK_CONNECT_PASSWORD} \
MTK_CONNECTED_DEVICES=1 \
MTK_CONNECT_TESTBENCH="Example-Testbench" \
MTK_CONNECT_TESTBENCH_USER="joeb@company.com" \
./mtk_connect.sh --start
cd -

# When complete, stop MTK Connect and delete the testbench.
cd ./workloads/common/mtk-connect/
sudo \
MTK_CONNECT_DOMAIN="dev.horizon-sdv.com" \
MTK_CONNECT_USERNAME=${MTK_CONNECT_USERNAME} \
MTK_CONNECT_PASSWORD=${MTK_CONNECT_PASSWORD} \
MTK_CONNECTED_DEVICES=1 \
MTK_CONNECT_TESTBENCH="Example-Testbench" \
./mtk_connect.sh --stop
cd -
```

When testing is complete, it is advisable to stop the instance, e.g.
`gcloud compute instances stop cuttlefish-vm-test-instance-v110 --zone=europe-west1-d`

When entirely finished with the instance, delete it. e.g.

From `Workloads/Android/Environment/CF Instance Template` delete the Cuttlefish test instance:

- `CUTTLEFISH_INSTANCE_NAME` : provide a unique name, starting with cuttlefish-vm, e.g. `cuttlefish-vm-test-instance-v110.`
- `DELETE` : This ensures the instance template, disk image and VM instance are deleted.

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

### Cuttlefish Virtual Devices not booting:

-   The CVD launcher will exit if it cannot boot the desired number of devices. Due to existing issues with CVD's device creation and booting process, it is safer to terminate and report failure rather than attempting to recover with fewer devices, as this may cause connectivity problems with some of the remaining devices.
     - Future plans include implementing mitigation strategies to ensure that devices that boot with fewer than the requested number can be trusted and utilized. Currently, these devices cannot be relied upon to function correctly.
-    WiFi: Some versions of Android, e.g. `android-14.0.0_r30` are not so reliable when it comes to connecting WiFi to the network. If the device cannot connect to the network, it is not possible to test WiFi connectivity. In future releases we will remove devices that fail to connect from the test.
