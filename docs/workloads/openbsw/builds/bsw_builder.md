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

# OpenBSW Builds

## Table of contents
- [Introduction](#introduction)
- [Prerequisites](#prerequisites)
- [Rust reference build (step by step)](#rust-reference-build-step-by-step)
- [Environment Variables/Parameters](#environment-variables)
  * [Targets](#targets)
- [System Variables](#system-variables)
- [Known Limitations](#known-limitations)

## Introduction <a name="introduction"></a>

**Eclipse Foundation OpenBSW Software Build Job**

This job automates the build process for the Eclipse Foundation OpenBSW software from a specified source repository and branch.

**Supported Targets and Build Options**

The job offers the following build targets and options:

- Documentation:
  - Creates OpenBSW documentation from doxygen.
- Unit Tests:
  - Build unit tests
  - Run unit tests (all or individual test library)
  - Generate code coverage reports
- Platform Builds:
  - POSIX Platform builds (`posix-freertos`, `posix-threadx`, or `posix-rust`)
  - NXP S32K148 Platform builds (`s32k148-*-*` presets; Rust uses `s32k148-rust-gcc` only)

**Artifact Storage**

Build artifacts are stored in two locations for easy access:

- **Jenkins Artifact Storage**: some artifacts such as test results, artifact summary are stored with the respective Jenkins build, providing a convenient reference point for retrieving artifacts from either location.
- **Google Cloud Storage**: larger artifacts are uploaded to the designated Google Cloud Storage bucket for the workload.

**Build Customization**

Users can choose to build all targets or select specific target groups using the provided parameters. There are also
options to override the build and test commands.

### References

- [Welcome to Eclipse OpenBSW](https://eclipse-openbsw.github.io/openbsw/sphinx_docs/doc/dev/index.html).
- [Building and Running Unit Tests.](https://eclipse-openbsw.github.io/openbsw/sphinx_docs/doc/dev/learning/unit_tests/index.html).
- [POSIX Platform](https://eclipse-openbsw.github.io/openbsw/sphinx_docs/doc/dev/learning/setup/setup_posix_build.html#setup-posix-build).
- [NXP S32K148 Platform](https://eclipse-openbsw.github.io/openbsw/sphinx_docs/doc/dev/learning/setup/setup_s32k148_ubuntu_build.html).
- [OpenBSW GitHub repo](https://github.com/eclipse-openbsw/openbsw.git).

## Prerequisites<a name="prerequisites"></a>

One-time setup requirements.

- Before running this pipeline job, ensure that the following template has been created by running the corresponding job:
  - Docker image template: `OpenBSW Workflows/Environment/Docker Image Template`

## Rust reference build (step by step) <a name="rust-reference-build-step-by-step"></a>

**Rust** here means the **`posix-rust`** (on Linux/POSIX) and **`s32k148-rust-gcc`** (on NXP) builds. For **why Rust needs an extra line in the POSIX test config file**, read [POSIX Target Test — simple explanation](../../tests/posix/README.md#why-the-rust-toml-snippet-matters-plain-english).

### Build in Jenkins

1. Build a fresh **Docker Image Template** so the image includes Rust, then use that **`IMAGE_TAG`** in BSW Builder.
2. Set **`RTOS_PLATFORM=rust`**. Turn on **`BUILD_POSIX`** and/or **`BUILD_NXP_S32K148`** as needed.
3. Optional: enable **`POSIX_PYTEST`** to run **automated POSIX tests** in the same job. With Rust selected, the pipeline **adds the missing Rust test config** so those tests know which program to launch (you should not need to edit files by hand).
4. Outputs: POSIX bundle **`…/posix/posix.tgz`** (includes **`build/posix-rust/…`**); S32K148 under **`build/s32k148-rust-gcc/…`**.

### Test **posix-rust** (POSIX Test job)

1. Copy the **`posix/`** folder URL from a Rust build (pad the build number if your storage layout requires it).
2. Run **OpenBSW → Tests → POSIX** and paste that URL into **`OPENBSW_DOWNLOAD_URL`**. The job adds the Rust test config when needed.
3. On the device host: bring up network/CAN, run the **`posix-rust`** app, or run automated tests from **`${HOME}/posix/test/pyTest`** (see the POSIX Test job screen for exact commands).

## Environment Variables/Parameters <a name="environment-variables"></a>

**Jenkins Parameters:** Defined in the groovy job definition `groovy/job.groovy`.

### `OPENBSW_GIT_URL`

This provides the URL for the OpenBSW repository. Such as:
- https://github.com/eclipse-openbsw/openbsw.git

### `OPENBSW_GIT_BRANCH`

This provides the branch/tag revision for the OpenBSW repository.

### `POST_GIT_CLONE_COMMAND`

Optional parameter that allows the user to include additional commands to run after the repository has been cloned.
Useful to pin OpenBSW to a particular sha1.

### `RTOS_PLATFORM`

Select `freertos`, `threadx`, or `rust`. The first two map to CMake presets `posix-*` / `s32k148-*-<toolchain>`. `rust` selects `posix-rust` for POSIX builds and `s32k148-rust-gcc` for S32K148 (GCC only; upstream does not ship a Clang Rust preset).

Note: Ensure the selected toolchain supports the specific kernel port.

### `BUILD_CONFIG`

Compilation profile, `debug` or `release`.

### `TOOLCHAIN`

Select the compiler toolchain for S32K148 (`gcc` or `clang`). GCC is standard; Clang provides enhanced static analysis. Ignored for the fixed `s32k148-rust-gcc` preset when `RTOS_PLATFORM` is `rust`.

### `IMAGE_TAG`

Specifies the name of the Docker image to be used when running this job.

The default value is defined by the `Seed Workloads` pipeline job. Users may override to provide a unique tag that describes the Linux distribution and tool chain versions.

### `CMAKE_SYNC_JOBS`

Defines the number of parallel sync jobs when running `cmake` commands.

### `CODE_COVERAGE`

Enable code coverage for unit tests. Only applicable when `BUILD_UNIT_TESTS` and `RUN_UNIT_TESTS` are enabled.

### `BUILD_DOCUMENTATION`

Use this to build the OpenBSW documentation using doxygen. PublishHTML is used in Jenkins so you can view the HTML output, or simply download the archive.

To view in Jenkins correctly, you would have to lower the [content security level](https://www.jenkins.io/doc/book/security/configuring-content-security-policy/) from `Script Console`, allowing the full HTML to be accessible, e.g.

`System.setProperty("hudson.model.DirectoryBrowserSupport.CSP", "")`

### `LIST_UNIT_TESTS`

This will create an artifact that shows all available unit tests, should users wish to target test to an individual test
rather than all.

### `LIST_UNIT_TESTS_CMDLINE`

The command that is used to list unit tests. Users may choose to override or retain default.

### `BUILD_UNIT_TESTS`

Build the unit tests. This will build `all` or that which is specified in `UNIT_TEST_TARGET`.

### `UNIT_TEST_TARGET`

Specify whether to build all tests or a specific test. See `LIST_UNIT_TESTS` which will generate a list of all available
tests.

e.g. `UNIT_TEST_TARGET` set to `bspTest`:

Creates `build/tests/Debug/libs/bsw/bsp/test/gtest` which can be used with `RUN_UNIT_TESTS_CMDLINE`.

### `UNIT_TESTS_CMDLINE`

The command that is used to build unit tests. Users may choose to override or retain default.

### `RUN_UNIT_TESTS`

Once unit tests are built, this will ensure unit tests are run. This is dependent on `BUILD_UNIT_TESTS` being enabled.

### `RUN_UNIT_TESTS_CMDLINE`

The command that is used to run unit tests. If the `UNIT_TEST_TARGET` is `all` this can be left as is. But if using
individual targets, it is recommended to either run the test target directly or use `ctest` and specify the test target
directory.

e.g. `UNIT_TEST_TARGET` set to `bspTest` use the following override:

`ctest --test-dir build/tests/posix/Debug/libs/bsw/bsp/test --parallel ${CMAKE_SYNC_JOBS}`

### `BUILD_POSIX`

Build the OpenBSW POSIX target. This will build the `app.referenceApp.elf` application and store in respective GCS bucket, for later use in the test pipeline job.

### `POSIX_BUILD_CMDLINE`

The command that is used to build the POSIX platform target. Users may choose to override or retain default.

### `POSIX_ARTIFACT`

The artifact to store. Default is the `app.referenceApp.elf`.

### `POSIX_PYTEST`

When enabled, runs **automated POSIX tests** (Python/pytest) after the POSIX build. You can turn this off and use the **POSIX Test** job instead. With **Rust** selected, the pipeline fills in the test config file so those tests can find the Rust build.

### `POSIX_PYTEST_CMDLINE`

The command used to run **automated POSIX tests**. By default it follows **`RTOS_PLATFORM`** (e.g. Rust → `--app=rust`). For Rust, the build script **adds the missing Rust test config** to `test/pyTest/target_posix.toml` before pytest when upstream does not supply it.

### `BUILD_NXP_S32K148`

Build the OpenBSW S32K148 Hardware target. This will build the `app.referenceApp.elf` application and store in respective GCS bucket for user to retrieve and install on their physical hardware.

### `NXP_S32K148_BUILD_CMDLINE`

The command that is used to build the NXP S32K148 platform target. Users may choose to override or retain default.

### `NXP_S32K148_ARTIFACT`

The artifact to store. Default is the `app.referenceApp.elf`.

### `INSTANCE_RETENTION_TIME`

Keep the build VM instance and container running to allow user to connect to it. Useful for debugging build issues, determining target output archives etc. Time in minutes.

Access using `kubectl` e.g. `kubectl exec -it -n jenkins <pod name> -- bash`

Reference [Fleet management](https://docs.cloud.google.com/kubernetes-engine/enterprise/multicluster-management/gateway) to fetch credentials for a fleet-registered cluster to be used in Connect Gateway, e.g.
- `gcloud container fleet memberships list`
- `gcloud container fleet memberships get-credentials sdv-cluster`

### `OPENBSW_ARTIFACT_STORAGE_SOLUTION`

Define storage solution used to push artifacts.

Currently `GCS_BUCKET` default pushes to GCS bucket, if empty then nothing will be stored.

### `STORAGE_BUCKET_DESTINATION`

Lets you override the default artifact storage destination. If not set, the build derives it automatically, for example:

`gs://${OPENBSW_BUILD_BUCKET_ROOT_NAME}/OpenBSW/Builds/BSW_Builder/<BUILD_NUMBER>`

The override must be a full GCS URI, including the `gs://` prefix, bucket name, and the artifact path. For example:

`gs://${OPENBSW_BUILD_BUCKET_ROOT_NAME}/OpenBSW/Releases/010129`

### `ENABLE_GEMINI_AI_ASSISTANT`

Enable Gemini AI to support in diagnosis of build and test failures.

### Gemini prompts

The job uses prompt files from the repository only; there is no Jenkins parameter to override them. Sequenced prompts (order matters): `prompt/sequenced/step1_triage.txt`, `step2_rca.txt`, `step3_fixes.txt`. Outputs: `step1_output.md`, `step2_output.md`, `step3_output.md`.

### `GEMINI_COMMAND_LINE`

Interface for the headless [gemini-cli](https://geminicli.com/docs/cli/headless/).
Use this to specify settings such as the [Gemini model](https://ai.google.dev/gemini-api/docs/models) etc, e.g.
`--debug` to include debug output.
Note: Prompts are piped via `stdin` and output is redirected to a JSON file.

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

### Gemini AI assistant (`ENABLE_GEMINI_AI_ASSISTANT`)

- **Experimental:** Gemini-assisted diagnosis in this pipeline is experimental; behavior, quality, and availability can change without notice.
- **Examples only:** Repository prompts and skills are **illustrative examples**—tune, replace, or disable them for your environment.
- **Upstream issues:** Problems may come from [Gemini CLI](https://github.com/google-gemini/gemini-cli) itself; see [open issues](https://github.com/google-gemini/gemini-cli/issues) for known bugs and workarounds.

**Document Generation:**

This will be added in future releases.

**Repository Access Control:**

Please note that support is only provided for open-source repositories with no access control. If access control is required, additional credentials will be necessary in Horizon-SDV, and the Jenkinsfile will need to be updated to include the retrieval and storage of these credentials in Git.
