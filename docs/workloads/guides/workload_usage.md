[Copyright (c) 2026 Accenture, All Rights Reserved.]::

[Licensed under the Apache License, Version 2.0 (the 'License');]::
[you may not use this file except in compliance with the License.]::
[  You may obtain a copy of the License at]::

[          http://www.apache.org/licenses/LICENSE-2.0]::

[  Unless required by applicable law or agreed to in writing, software]::
[  distributed under the License is distributed on an 'AS IS' BASIS,]::
[WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.]::
[  See the License for the specific language governing permissions and]::
[  limitations under the License.]::


# <span style="color:#335bff">Workload Usage</span>

This document covers the usage of **Android**, **Cloud Workstations**, and **OpenBSW** Jenkins workloads.

It includes pointers to workload documentation under [`docs/workloads/`](../../workloads/) (job parameters and scripts are described there and in `workloads/*/pipelines/**/README.md`).

> **Note:** This setup document combines with the [workload_setup.md](workload_setup.md), [workload_usage_android.md](workload_usage_android.md) and [android_labs.md](android_labs.md) documents to supersede the former Android `docs/workloads/android/guides/developer_guide.md` (deprecated). 

## Table of Contents
- [Prerequisites](#prerequisites)
- [ANDROID WORKLOAD](#android)
- [OPENBSW WORKLOAD](#openbsw)
- [CLOUD WORKSTATIONS](#cloud-workstations)
- [AI REVIEW](#ai-review)
  - [Supported Jobs](#ai-review-jobs)
  - [Analysis Mechanism](#ai-review-analysis-mechanism)
  - [Configuration Parameters](#ai-review-params)
  - [Perform an AI Review](#ai-review-perform)
- [APPENDICES](#appendices)
  - [Jenkins Job Runs](#appendix-jenkins-jobs)
    - [Run Info](#appendix-jenkins-jobs-run-info)
    - [Run Artifacts](#appendix-jenkins-jobs-run-artifacts)
  - [Machine Types](#appendix-machine-types)
    - [View Table](#appendix-machine-types-table)
  - [Standalone Build and Test Scripts](#appendix-standalone-scripts)
  - [Debugging and Extending Build and Test Jobs](#appendix-debug-extend-jobs)
  - [MTK Connect Testbench Access](#appendix-mtk-connect-testbench-access)
    - [MTK Connect Testbench - Browser](#appendix-mtk-connect-testbench-access-browser)
      - [Device Interaction](#device-interaction)
    - [MTK Connect Testbench - Tunnel](#appendix-mtk-connect-testbench-access-tunnel)

[ ============================================================================= ]::
[ ============================================================================= ]::
[ ============================================================================= ]::

<hr>

> [!IMPORTANT]
> - The URLs referenced in the instructions reference an example domain `example.horizon-sdv.com`; replace these URLs with your domain
> - Jenkins Dashboard: https://example.horizon-sdv.com/jenkins/

## <span style="color:#335bff">Prerequisites<a name="prerequisites"></a></span>

1. **Platform deployed** — Horizon SDV cluster and services (including Jenkins) are available for your environment; see your deployment guide.
2. **Workload Setup** — setup completed as per [workload_setup.md](workload_setup.md).
3. **Developer tools** — As required by the workloads you use (for example `git`, Google Cloud CLI, `adb` / `fastboot` for Android device flows).

[ ============================================================================= ]::

## <span style="color:#335bff">ANDROID WORKLOAD<a name="android"></a></span>

Jenkins folder display name: **Android Workflows**

A separate Android Workflow usage document is located at [workload_usage_android.md](workload_usage_android.md). It contains information on various topics relating to the Android Workload jobs and how they can be used. 

While some of these training materials are Android-specific some may still contain information that could be applied to other workload areas (e.g. [gerrit](workload_usage_android.md#gerrit)).

[ ============================================================================= ]::

## <span style="color:#335bff">OPENBSW WORKLOAD<a name="openbsw"></a></span>

Jenkins folder display name: **OpenBSW Workflows**

### OpenBSW — Example Usage

| Goal | Documentation |
| --- | --- |
| Builds (unit tests, POSIX, hardware targets) | [BSW Builder](../openbsw/builds/bsw_builder.md) |
| POSIX tests pipeline | [POSIX tests](../openbsw/tests/posix.md) |
| Step-by-step Jenkins walkthrough | [OpenBSW developer guide](../openbsw/guides/developer_guide.md) | 


[ ============================================================================= ]::

## <span style="color:#335bff">CLOUD WORKSTATIONS<a name="cloud-workstations"></a></span>


Jenkins folder display name: **Cloud Workstations**

### Cloud Workstations — Example Usage

| Goal | Documentation |
| --- | --- |
| User lifecycle (list/start/stop/get) | [Workstation user operations](../cloud-workstations/workstation_user_operations.md) |
| Cluster create/delete | [Cluster admin operations](../cloud-workstations/cluster_admin_operations.md) |
| Config admin flows | [Config admin operations](../cloud-workstations/config_admin_operations.md) |
| Workstation admin flows | [Workstation admin operations](../cloud-workstations/workstation_admin_operations.md) |


[ ============================================================================= ]::

## <span style="color:#335bff">AI REVIEW</span> <a name="ai-review"></a>

An **AI Review** stage is included in a selection of jobs to assist the user in diagnosing any job failure(s) and to provide remediation suggestions.

The stage is **invoked when the build/test stages of the run have failed** and the user has opted in via the `ENABLE_GEMINI_AI_ASSISTANT` parameter. Additionally, the `GEMINI_ANALYSE_ON_SUCCESS` parameter in test jobs can be enabled to analyze CTS/CVD logs for hidden/underlying issues. 

An LLM-backed agent (Gemini, via Vertex AI) is used to run a sequenced, three-step analysis on the available logs and artifacts; it produces:
- A **triage** summary (the first fatal error and a failure matrix).
- A **root cause analysis** (RCA) for each blocker.
- One or more **proposed fixes** as markdown files under `gemini-assist/`.

_Note: The output is intended as a starting point for triage - not a substitute for engineer review._

> [!IMPORTANT]
> - The AI Review stage is **experimental**; behaviour, model availability and output quality can change without notice.

> [!NOTE]
> - The AI Review stage uses the headless [Gemini CLI](https://geminicli.com/docs/cli/headless/) on Vertex AI. Your GCP project must have the relevant Gemini models enabled and the Jenkins service account must have permission to invoke them.
> - Repository prompts and skills shipped with the platform are **illustrative examples only** and should be tuned (or disabled) for your environment. See the [Gemini CLI integration](../common/agentic-ai/gemini.md) doc for further details.

[ --- Collapsing Section --- ]::
<details id="ai-review-jobs"><summary>Supported Jobs</summary><hr width="50%">

The following Jenkins pipeline jobs implement an `AI Review` stage:
- _Android Workflows_: [AAOS Builder](../android/builds/aaos_builder.md), [AAOS ABFS Builder](../android/builds/aaos_abfs_builder.md), [CTS Execution](../android/tests/cts_execution.md), [CVD Launcher](../android/tests/cvd_launcher.md) 
- _OpenBSW Workflows_ → [BSW Builder](../openbsw/builds/bsw_builder.md) 

In addition, a standalone [Gemini AI Assistant](../utilities/gemini_ai_assistant.md) job is available in the _Utilities_ area for ad-hoc analysis of user-provided artifacts (e.g. fetched from a GCS bucket).

<hr width="50%"></details>


[ --- Collapsing Section --- ]::
<details id="ai-review-analysis-mechanism"><summary>Analysis Mechanism</summary><hr width="50%"> 

The **AI Review** stage uses the same shared Gemini integration scripts (`gemini_initialise.sh`, `gemini_analysis.sh`, `gemini_environment.sh`), located in `workloads/common/agentic-ai/gemini/` no matter which pipeline job it is used in. 

For a deeper technical reference, see [Gemini CLI integration](../common/agentic-ai/gemini.md).
A short summary is as follows:

A **three-step sequenced** analysis (triage, rca, fix) is executed whereby each step is a separate prompt; the output of step _N_ (`step<N>_output.md`) is appended into the markdown prompt for step _N+1_. The final 'fix' step outputs one or more proposed fix files: `gemini-assist/gemini_proposed_fixes_*.md`.

**Prompt = task.** Each prompt file is an individual *task* for a run (i.e. a short invocation - e.g. "Run triage on the test results in /test-results").
Prompt file contents change for each job / type of job (jobs of similar type may share prompt files) and are stored in `prompt/sequenced` within each pipeline job directory.

**Skill = instruction.** The skill (in `skills.yaml` located with the prompt files for each job) is the *instruction* for how to carry out the tasks. It defines behavior (role, procedure, rules, output format), with headings such as "Procedure:" or "Steps:". 
 
See examples of CTS skills and prompts [here](../../../workloads/android/pipelines/tests/cts_execution/prompt/sequenced/README_SKILLS.md).

<hr width="50%"></details>


[ --- Collapsing Section --- ]::
<details id="ai-review-params"><summary>Configuration Parameters</summary><hr width="50%">

The AI Review stage is controlled by a small set of parameters/environment variables. They are exposed on the individual build/test job pages in the **`Agentic AI: Configuration (Experimental)`** section of the parameters forms. 

> [!TIP]
> The recommended model configuration when targeting preview functionality is `GEMINI_LOCATION_GLOBAL=true`, `GEMINI_PREVIEW_FEATURES=true` and a pinned model (`GEMINI_MODEL=gemini-3.8-flash` or via `--model` in `GEMINI_COMMAND_LINE`). See [Gemini CLI integration / Known issues](../common/agentic-ai/gemini.md#known-issues) for further detail.

The prompt files themselves are **not** exposed as Jenkins parameters in any of the four pipelines - they are taken directly from the repository at `workloads/<area>/pipelines/.../prompt/sequenced/`. To experiment with different prompts or skills without changing the repo, use the [Gemini AI Assistant](../utilities/gemini_ai_assistant.md) utility job, which accepts uploaded prompt and `skills.yaml` files.

<hr width="50%"></details>


[ --- Collapsing Section --- ]::
<details id="ai-review-perform"><summary>Perform an AI Review</summary><hr width="50%">

**Enable AI Review on a Build/Test Job**
>[!NOTE]
> The AI Review stage is usually **executed when the previous build/test stages have produced a `FAILURE`**; the only exception is if the `GEMINI_ANALYSE_ON_SUCCESS` parameter is enabled in a test job. To exercise the _AI Review_ stage for a failures do either of the following:
> - Build/test against a target/revision that you know is broken.
> - push a deliberately broken [Android Gerrit change](workload_usage_android.md#gerrit) and [build](workload_usage_android.md#gerrit-build) that change (ensuring that the AI Review feature is turned on).

> [!TIP]
> - the [Additional Modifications](android_labs.md#android-modifications) section of the lab_exercises.md document includes example modifications that you can choose to deliberately break to try to provoke a build failure

- Navigate to the desired pipeline job
- Select `Build with Parameters`.
- In the `Agentic AI: Configuration (Experimental)` section:
    - Confirm `ENABLE_GEMINI_AI_ASSISTANT` is **checked** (it is not always on by default).
    - Leave `AI_REVIEW_RUNTIME` on `agy` (the default) or choose `gemini-cli` for the sequenced CLI. Argo charts also default to `agy`.
    - Optionally edit `GEMINI_COMMAND_LINE` when using `gemini-cli` (e.g. to pin a different model). Antigravity uses `GEMINI_MODEL`.
    - Optionally change `GEMINI_AI_EXECUTION_TIMEOUT_MINUTES` (choices start at 60). The runner uses 60 minutes when the variable is unset.
- Configure the rest of the job parameters as required.
- Click `Build`.

**ALTERNATIVE: Perform an AI Review on a previously failed build**
The standalone [Gemini AI Assistant](../utilities/gemini_ai_assistant.md) job is also available in the _Utilities_ area for ad-hoc analysis of user-provided artifacts (e.g. fetched from a GCS bucket); this allows analysis (with custom prompts/skills) to be done on a previously-generated set of failed artifacts

**Locate the AI Review Stage in the Run**
- Open the Jenkins run page for the job.
- The pipeline view shows the `AI Review` stage immediately after `Build` stage (for the Android build jobs), `CTS execution` stage (for the Android CTS test job), `Diagnostics & Teardown` stage (for the Android CVD Launcher job) `platform builds` for OpenBSW build job.
- All review artifacts are presented as Jenkins job artifacts.

> [!NOTE]
> If you do not see an `AI Review` stage in the pipeline view at all, check that `ENABLE_GEMINI_AI_ASSISTANT` was `true` for the run **and** that the prior stages produced a `FAILURE` result. A successful build skips the stage by design.

<hr width="50%"></details>


[ ============================================================================= ]::

## <span style="color:#335bff">APPENDICES<a name="appendices"></a></span>


[ --- Collapsing Section --- ]::
<details id="appendix-jenkins-jobs"><summary>Jenkins Job Runs</summary><hr width="50%">


[ --- Collapsing SubSection --- ]::
<details id="appendix-jenkins-jobs-run-info"><summary>Run Info</summary><hr width="25%">

To identify your particular run of a pipeline job, refer to the <b>_Builds_ summary</b> section on the LHS of the job page. This section provides a concise description of each run:
- Who kicked off the run
- Various relevant parameters (depending on the job)

To view the <b>information page</b> for each run, click on the run number.

<hr width="25%"></details>


[ --- Collapsing SubSection --- ]::
<details id="appendix-jenkins-jobs-run-artifacts"><summary>Run Artifacts</summary><hr width="25%">

When the run completes, the <b>artifacts</b> that were stored are listed on the information page for the run. 

Depending on the job, artifacts may also be stored in a Google Cloud Storage bucket as well as in Jenkins itself. In such cases, the artifacts may include an <b>artifacts file</b> (with `_artifacts.txt` postfix) which indicates the location of the storage bucket, as well as providing the links and commands required to view/download the artifacts.

<hr width="25%"></details>

<hr width="50%"></details>


[ --- Collapsing Section --- ]::
<details id="appendix-machine-types"><summary>Machine Types</summary><hr width="50%">

The table below shows the templates and machine types used for the Android workflows.

[ --- Collapsing SubSection --- ]::
<details id="appendix-machine-types-table"><summary>View Table</summary><hr width="25%">

| Job Name             | [buildkit](https://hub.docker.com/r/moby/buildkit) | Docker Image Template | CF Instance Template |
| :----------------------------------------------------------------| :---------------------------------------------------: | :-------------------: | :------------------: |
| `Android Workflows / Environment / Docker Image Template`         |  ✅ |    |    |
| `Android Workflows / Environment / CF Instance Template`          |     | ✅ <sup>1</sup>|    |
| `Android Workflows / Environment / Delete Cuttlefish VM Instance` |     | ✅ <sup>1</sup>|    |
| `Android Workflows / Environment / Delete MTK Connect Testbench`  |     | ✅ <sup>1</sup>|    |
| `Android Workflows / Environment / Development Instance`          |     | ✅ <sup>2</sup>|    |
| `Android Workflows / Environment / Warm Build Caches`             |     | ✅ <sup>2</sup>|    |
| `Android Workflows / Builds / AAOS Builder`                       |     | ✅ <sup>2</sup>|    |
| `Android Workflows / Tests / CTS Execution`                       |     | ✅ <sup>1</sup>| ✅ <sup>3</sup> |
| `Android Workflows / Tests / CVD Launcher`                        |     | ✅ <sup>1</sup>| ✅ <sup>3</sup> |

<sup>1: Uses any available node: Horizon standard nodes are `n1-standard-4` shared across tools and platform.</sup><br/>
<sup>2: Uses build nodes: `c2d-highcpu-112`</sup><br/>
<sup>3: Uses test nodes: `n2-standard-32`</sup>


<hr width="25%"></details>

To read more on how the machine types are configured, refer to the following within the OSS repo: [horizon-sdv](https://github.com/googlecloudplatform/horizon-sdv)

[ --- Collapsing SubSection --- ]::
<details><summary>Android Build Jobs: _c2d-highcpu-112_</summary><hr width="25%">

- `./terraform/env/main.tf`: `sdv_build_node_pool_machine_type   = "c2d-highcpu-112"`
- `./terraform/modules/base/variables.tf`: ` default     = "c2d-highcpu-112"`
  - Re-run the deployment script to apply any Terraform changes.
- Jenkinsfile `kubernetesPodTemplate` POD configuration:
  - Refer to `resources`, `limits` and `requests` in the Jenkinsfile. These are optimised to the Jenkins pipeline builds and
    need for additional resources for the Jenkins agent etc. You may adjust to your machine limits.
  - `workloads/android/pipelines/environment/warm_build_caches/Jenkinsfile`
  - `workloads/android/pipelines/environment/dev_instance/Jenkinsfile`
  - `workloads/android/pipelines/builds/aaos_builder/Jenkinsfile`

<hr width="25%"></details>

[ --- Collapsing SubSection --- ]::
<details><summary>OpenBSW Build Jobs: ~_n1-standard-8_</summary><hr width="25%">

- `./terraform/env/main.tf`: `sdv_openbsw_build_node_pool_machine_type   = "n1-standard-8"`
- `./terraform/modules/base/variables.tf`: ` default     = "n1-standard-8"`
  - Re-run the deployment script to apply any Terraform changes.

<hr width="25%"></details>

[ --- Collapsing SubSection --- ]::
<details><summary>Test Jobs: _n2-standard-32_</summary><hr width="25%">

- This is part of the `Android Workflows` → `Environment` → `CF Instance Template` configuration.
- Change `MACHINE_TYPE` parameter to the machine type you wish to use and regenerate the Instance Templates.

<hr width="25%"></details>


<hr width="50%"></details>


[ --- Collapsing Section --- ]::
<details id="appendix-standalone-scripts"><summary>Standalone Build and Test Scripts</summary><hr width="50%">

Most Jenkins pipeline jobs execute scripts that can be run directly on build and CF instances, without relying on Jenkins. These scripts are available in the OSS repository: [horizon-sdv](https://github.com/googlecloudplatform/horizon-sdv). These scripts can be run indepedent of Jenkins.

Documented examples are provided in the following directories:
- `docs/workloads`
- `workloads/android/pipelines/`
- Respective areas, environment, build and test directories

The exceptions are the `Docker Image Template` and `CF Instance Template jobs`, which must be run through Jenkins. However, running scripts directly can facilitate development and testing, allowing you to:
- Iterate faster on changes
- Test and validate scripts independently
- Simplify debugging and troubleshooting

<hr width="50%"></details>


[ --- Collapsing Section --- ]::
<details id="appendix-debug-extend-jobs"><summary>Debugging and Extending Build and Test Jobs</summary><hr width="50%">

When running build and test jobs from Jenkins, you have the option to extend the job and connect to the instance for debugging and further testing. This feature allows developers to:
- Investigate build and test failures in real-time
- Run additional tests or experiments
- Gather more information for debugging purposes

For more information on how to do so, refer to the documentation in the OSS repository: [horizon-sdv](https://github.com/googlecloudplatform/horizon-sdv).

Helper jobs are provided in the _Android_ and _OpenBSW_ Workload areas (`Environment` → `Development Instance`) to aid with creating your own build instance and running scripts/jobs locally.

<hr width="50%"></details>


[ --- Collapsing Section --- ]::
<details id="appendix-mtk-connect-testbench-access"><summary>MTK Connect Testbench Access</summary><hr width="50%">

>[!NOTE:] Although the _Test_ pipeline jobs in the Android Workloads area set up a MTK Connect Testbench and connect the virtual devices that were launched by the jobs, MTK Connect is not limited to using Testbenches created in this way, nor is it limited to accessing virtual devices - it can also be used to connect to Testbenches with physical devices.
>- Official MTK Connect documentation: https://example.horizon-sdv.com/mtk-connect/docs/


[ --- Collapsing SubSection --- ]::
<details id="appendix-mtk-connect-testbench-access-browser"><summary><b>MTK Connect Testbench - Browser</b></summary><hr width="50%">

**_Prerequisites:_**
- MTK Connect Testbench set up with connection(s) to device(s) 
  - e.g. Test pipeline job currently running and in the `Keep Devices Alive` stage.
- Open MTK Connect application at (https://example.horizon-sdv.com/mtk-connect)  
- Select `TESTBENCHES` tab within the application:
- Find the relevant testbench 
    - Testbench created by test jobs are identifiable from the test job name and number (e.g. `Android/Tests/CVD_Launcher-8`). The full link to the testbench is also reported within the console log for these jobs.
- On your Testbench, lock the testbench or individual device(s) by selecting the padlock symbol on the full testbench or on a single device and elect the duration you wish to `Book the device`
    - Refresh the browser page if your testbench has yet to appear.
- The device(s) in the testbench are now ready to interact with through the UI:
    - <b>Device Interaction:</b><a name="device-interaction"></a>
        - Click on the screen directly to simulate touch-screen interaction
        - Use the remote for general operations (e.g. left, right, ok, home, etc)
    - <b>adb Access to Device:</b>
        - Select the three dots symbol -> `Launch` -> `adb`
    - <b>adb Access to Host Platform:</b>
        - Select the three dots symbol -> `Launch` -> `HOST`

<hr width="50%"></details>

[ --- Collapsing SubSection --- ]::
<details id="appendix-mtk-connect-testbench-access-tunnel"><summary><b>MTK Connect Testbench - Tunnel</b></summary><hr width="50%"> 

A tunnel can be used to access an MTK Connect Testbench from a local machine.

**_Prerequisites:_**
- MTK Connect Testbench set up with connection(s) to device(s) - note the name of the Testbench (e.g. `Android/Tests/CVD_Launcher-52`)
  - e.g. Test pipeline job currently running and in the `Keep Devices Alive` stage.
- [adb](https://developer.android.com/tools/adb) is installed on your local machine

**_Install Tunnel on Local Machine:_**
- Open MTK Connect application (https://example.horizon-sdv.com/mtk-connect)
- Click on the User icon and select `Tunnels`
- Click on the _'Please install MTK Connect Tunnel'_ link:
- Follow the instructions to install the Tunnel based on your PC (Mac, Windows, Linux),

**_Set up Tunnel to Testbench/Device:_**
- Once installed, return to MTK Connect application, `Tunnels` and click `+`
  - Enter Testbench name 
  - Choose an appropriate port (e.g. 8555) on your PC
  - Select a Device (if there are more than 1 in your Testbench)
  - Select `Save`

**_Connect to Testbench via Tunnel_**
- Open a terminal session 
  - e.g. _Command Prompt_ on PC, _Terminal Session_ in Android Studio
- Connect `adb` (using the port number you used to create the tunnel, e.g. 8555)
  - e.g. `adb connect localhost:8555`
- Any adb commands can now be sent to the testbench and/or devices
  - e.g. install apk: `adb install app-debug.apk`
  - e.g. send key press 'Enter': `adb shell input keyevent 66`

>[!NOTE:]
> This tunnel can also be used to allow the devices in the testbench be accessed interactively in Android Studio - see more info [here](android_labs.md#access-android-studio).

<hr width="50%"></details>

<hr width="50%"></details>

[ ============================================================================= ]::



