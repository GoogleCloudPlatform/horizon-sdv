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

# POSIX Target Test

## Table of contents
- [Introduction](#introduction)
- [Why Rust tests need an extra config line](#why-the-rust-toml-snippet-matters-plain-english)
- [Step-by-step: posix-rust](#step-by-step-posix-rust)
- [Prerequisites](#prerequisites)
- [Environment Variables/Parameters](#environment-variables)
- [System Variables](#system-variables)
- [Known Limitations](#known-limitations)

## Introduction <a name="introduction"></a>

This job enables users to test a prior build of the POSIX application on the OpenBSW POSIX platform using MTK Connect.

Use `NUM_HOST_INSTANCES` to set how many device sessions to create. These sessions appear under the testbench in the MTK Connect application. They all attach to the same host instance but run independently, allowing you to use separate shell sessions. This will allow interaction with a running POSIX application from a separate shell session.

The following options are available for testing the POSIX reference application:

**POSIX reference application**

Bring up networking and run the ELF from the artifact that matches your BSW Builder preset:

| Preset | Command |
|--------|---------|
| posix-freertos | `./build/posix-freertos/executables/referenceApp/application/Release/app.referenceApp.elf` |
| posix-threadx | `./build/posix-threadx/executables/referenceApp/application/Release/app.referenceApp.elf` |
| posix-rust | `./build/posix-rust/executables/referenceApp/application/Release/app.referenceApp.elf` |

Run these from **`${HOME}/posix`** after unpack (this job puts artifacts there). Prefix with `./tools/enet/bring-up-ethernet.sh && ./tools/can/bring-up-vcan0.sh &&` when needed (use `sudo` if the host requires it).

**POSIX pyTest**

Examples (`pytest` runs under `test/pyTest/` relative to **`${HOME}/posix`**):

- FreeRTOS: `pytest --target=posix --app=freertos`
- ThreadX: `pytest --target=posix --app=threadx`
- Rust (`posix-rust` artifact): `pytest --target=posix --app=rust`

After download, this job adds the **Rust** lines to the small test config file **`test/pyTest/target_posix.toml`** when they are missing (see below). You normally do nothing manually.

## Why Rust tests need an extra config line <a name="why-the-rust-toml-snippet-matters-plain-english"></a>

**In short:** OpenBSW’s automated POSIX tests read a file (**`target_posix.toml`**) that lists **which built program to run** for each flavor (FreeRTOS, ThreadX, Rust). The stock project lists FreeRTOS and ThreadX but **not Rust yet**. Without adding Rust, the test runner stops immediately—it never gets to real tests.

Think of that file as **labeled switches**: “When someone picks Rust, start **this** program.” Our **`target_posix_rust.fragment.toml`** is just that missing Rust switch (pointing at the **`posix-rust`** build). **BSW Builder** and **this job** copy it into **`target_posix.toml`** for you when needed.

**Manual unpack only:** append **`workloads/openbsw/pipelines/tests/posix/target_posix_rust.fragment.toml`** to **`test/pyTest/target_posix.toml`**, or copy its **`[rust.target_process]`** block by hand.

## Step-by-step: posix-rust <a name="step-by-step-posix-rust"></a>

Use this when your **BSW Builder** run used **`RTOS_PLATFORM=rust`** (CMake preset **`posix-rust`**).

1. **Produce the artifact** — Complete a BSW Builder job with **`BUILD_POSIX`** enabled and **`RTOS_PLATFORM=rust`**. Note the build number and the GCS path **`…/OpenBSW/Builds/BSW_Builder/&lt;NN&gt;/posix/`** (zero-pad **`NN`** if required).
2. **Run this POSIX Test job** — Set **`OPENBSW_DOWNLOAD_URL`** to that **`posix/`** folder. Wait for **Download artifacts** to finish; the script **adds the Rust test config** to **`target_posix.toml`** when it is missing.
3. **MTK Connect** — Use the portal session created by the job (see **`NUM_HOST_INSTANCES`**).
4. **Shell on the host** — `cd "${HOME}/posix"` (unpack directory for `posix.tgz`), then:
   - `./tools/enet/bring-up-ethernet.sh`
   - `./tools/can/bring-up-vcan0.sh`
5. **Run the app** — `./build/posix-rust/executables/referenceApp/application/Release/app.referenceApp.elf`
6. **Optional pyTest** — In another shell: `cd "${HOME}/posix/test/pyTest" && pytest --target=posix --app=rust`

### References <a name="references"></a>

- [Application Console](https://eclipse-openbsw.github.io/openbsw/sphinx_docs/doc/learning/console/index.html)
- [POSIX](https://eclipse-openbsw.github.io/openbsw/sphinx_docs/doc/platforms/posix/index.html#posix)

## Prerequisites<a name="prerequisites"></a>

One-time setup requirements.

- Before running this pipeline job, ensure that the following templates have been created by running the corresponding jobs:
  - Docker image template: `OpenBSW Workflows/Environment/Docker Image Template`

## Environment Variables/Parameters <a name="environment-variables"></a>

**Jenkins Parameters:** Defined in the groovy job definition `groovy/job.groovy`. The Developer Portal workflow `openbsw-posix-test` uses the names in parentheses. Defaults are in `gitops/modules/openbsw-module/argo-workflows/values.yaml`.

### `OPENBSW_DOWNLOAD_URL` (`openbswDownloadUrl`)

Storage URL of the POSIX artifact folder (`posix.tgz`) from BSW Builder.

Jenkins, when `STORAGE_BUCKET_DESTINATION` is empty: `gs://${OPENBSW_BUILD_BUCKET_ROOT_NAME}/OpenBSW/Builds/BSW_Builder/<BUILD_NUMBER>/posix`.

Portal workflow `openbsw-bsw-builder`, when `storageBucketDestination` is empty: `gs://<project>-openbsw/OpenBSW/Builds/BSW_Builder/<date>-<time>_<workflow>/posix`.

An empty `openbswDownloadUrl`, or a URL outside `posixTest.downloadUrlAllowPrefixes`, fails the workflow. The default allow-list is the module bucket `gs://<project>-openbsw/`.

### `IMAGE_TAG` (`imageTag`)

Specifies the name of the Docker image to be used when running this job.

The Jenkins default is the `Seed Workloads` tag `OPENBSW_IMAGE_TAG`. The portal default is `argowf-latest`.

### `NUM_HOST_INSTANCES` (`numHostInstances`)

Number of host instances to create for testing the POSIX application. This is effectively the number of devices that
will be created associated with the development instance testbench in MTK Connect. The portal accepts 1 to 8.

### `POSIX_KEEP_ALIVE_TIME` (`posixKeepAliveTime`)

When using MTK Connect to test the POSIX images, the VM instance must be allowed to continue to run. This timeout, in
minutes, gives the tester time to keep the instance alive so they may work with the devices via MTK Connect.

### `MTK_CONNECT_PUBLIC`

When checked, the MTK Connect testbench is visible to everyone and can be shared.
By default, testbenches are private and only visible to their creator and MTK Connect administrators.

## SYSTEM VARIABLES <a name="system-variables"></a>

There are a number of system environment variables that are unique to each platform but required by Jenkins build, test and environment pipelines.

These are defined in Jenkins CasC `values-jenkins.yaml` and can be viewed in Jenkins UI under `Manage Jenkins` -> `System` -> `Global Properties` -> `Environment variables`.

These are as follows:

-   `OPENBSW_BUILD_BUCKET_ROOT_NAME`
     - Defines the name of the Google Storage bucket that will be used to store build and test artifacts

-   `OPENBSW_BUILD_DOCKER_ARTIFACT_PATH_NAME`
    - Defines the registry path where the Docker image used by builds, tests and environments is stored.

-   `CLOUD_PROJECT`
    - The GCP project, unique to each project. Important for bucket, registry paths used in pipelines.

-   `CLOUD_REGION`
    - The GCP project region. Important for bucket, registry paths used in pipelines.

-   `HORIZON_DOMAIN`
    - The URL domain which is required by pipeline jobs to derive URL for tools and GCP.

-   `HORIZON_SCM_URL`
    - The URL to the Horizon SDV git repository.

-   `HORIZON_SCM_BRANCH`
    - The branch name the job will be configured for from `HORIZON_SCM_URL`.

-   `JENKINS_SERVICE_ACCOUNT`
    - Service account to use for pipelines. Required to ensure correct roles and permissions for GCP resources.

## Known Limitations<a name="known-limitations"></a>

**CAN Virtualisation:**

This is not supported currently because the POSIX target is running in a Docker container in kubernetes POD. CAN
virtualisation will be supported in later releases.

**Hardware Support:**

The NXP S32K148 is not supported in this test, this may be added in a future release.
