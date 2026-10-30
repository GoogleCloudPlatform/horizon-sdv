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

# Android Build Filesystem (ABFS) Builder

## Table of contents
- [Introduction](#introduction)
- [Prerequisites](#prerequisites)
- [Environment Variables/Parameters](#environment-variables)
  * [Targets](#targets)
- [System Variables](#system-variables)
- [Known Issues](#known-issues)

## Introduction <a name="introduction"></a>

This job is used to build Android Automotive virtual devices and platform targets using the source and build caches from the Android Build Filesystem (ABFS). The sources are mounted from the ABFS server (`abfs mount -b <AAOS_REVISION>`) instead of `repo sync`, and ninja reuses cached outputs through `abfs cacheman`.

It is available in two forms sharing the same scripts (`workloads/android/pipelines/builds/aaos_builder/`):

- **Argo Workflow** `aaos-builder-abfs`, deployed by the `workloads-android` module (chart `aaos_builder/helm` with `values-abfs.yaml`). Submit it from the Developer Portal: **workloads-android → Workflow Templates → aaos-builder-abfs → Configure & submit**. Parameters are the camelCase equivalents of the Jenkins ones (`androidRevision`, `lunchTarget`, `postRepoCommand`, `abfs.server`, ...). The build runs in one pod in namespace `workflows`.
- **Jenkins job** `Android Workflows → Builds → AAOS Builder ABFS`, parameters below.

For the ABFS platform (modules, license, seeding) see [ABFS integration](../abfs.md).

This pipeline/scripts supports builds for:

- [Android Virtual Devices](https://source.android.com/docs/automotive/start/avd/android_virtual_device) for use with [Android Studio](https://source.android.com/docs/automotive/start/avd/android_virtual_device#share-an-avd-image-with-android-studio-users)
- [Cuttlefish Virtual Devices](https://source.android.com/docs/devices/cuttlefish) for use with [CTS](https://source.android.com/docs/compatibility/cts) and emulators.
- [Pixel Tablets](https://source.android.com/docs/automotive/start/pixelxl) Reference hardware platforms.

The following provides examples of the environment variables and Jenkins build parameters that are required.

**Note:** the build, whether successful or not, will create the file `abfs_repository_list.txt` which can be used to correlate `ABFS_VERSION` and `ABFS_CASFS_VERSION` based on the build instance kernel revision. Versions do get updated and therefore it is best that users pay attention to the output of this file and update the `Seed Workflow` values for ABFS versions so as to utilise the latest provided by Google.

## Prerequisites<a name="prerequisites"></a>

One-time setup requirements.

- ABFS modules enabled from the Developer Portal in this order: `abfs-data` (license imported), `abfs-service-module`, `abfs-client-ubuntu`. The build pods run on the `abfs-build-node-pool-ubuntu` pool created by the client module and connect to `abfs-server.abfs.svc.cluster.local:50051`.
- The ABFS builder image pushed to the project registry (`horizon-sdv/aaos-abfs-builder`): Argo `workloads-android → abfs-builder-runtime-image` with `NO_PUSH=false`, or Jenkins `Android Workflows/Environment/ABFS/Docker Image Template`. See [Docker Image Template](../environment/abfs/docker_image_template.md).
- The requested `AAOS_REVISION` seeded into ABFS: the matching row must have been active in `abfs-data → abfs-uploader-config-select` long enough for the uploaders to finish. A build of an unseeded revision fails at `abfs mount`. The current implementation seeds a single branch/revision at a time; if the `AndroidLatest` row is seeded first, later seeds are faster because the sources of the new branch already reside on the ABFS server.

## Environment Variables/Parameters <a name="environment-variables"></a>

**Jenkins Parameters:** Defined in the groovy job definition `groovy/job.groovy`.

### `AAOS_REVISION`

The Android revision, i.e. branch or tag to build. It must be seeded into ABFS (see Prerequisites). Tested versions:

- `android-14.0.0_r30` (ap2a)
- `android-15.0.0_r36` (bp1a)
- `android-16.0.0_r3` (bp3a)

### `AAOS_MANIFEST_NAME`

Manifest XML filename inside the manifest repository. Defaults to `default.xml` when unset.

### `AAOS_LUNCH_TARGET` <a name="targets"></a>

The Android target to build Android cuttlefish, virtual devices, Pixel and RPi targets.

Reference: [Codenames, tags, and build numbers](https://source.android.com/docs/setup/reference/build-numbers)

Examples:

- Virtual Devices:
    -   `sdk_car_x86_64-bp1a-userdebug`
    -   `sdk_car_arm64-bp1a-userdebug`
    -   `aosp_cf_x86_64_auto-bp1a-userdebug`
    -   `aosp_cf_arm64_auto-bp1a-userdebug`
-   Pixel Devices:
    -   `aosp_tangorpro_car-bp1a-userdebug`

### `AAOS_BUILD_CTS`

This builds the Android Automotive Compatibility Test Suite ([CTS](https://source.android.com/docs/compatibility/cts)) test harness from the specified code base, if the `AAOS_LUNCH_TARGET` is that of Cuttlefish, i.e `aosp_cf`.

### `ANDROID_VERSION`

Only applicable for sdk AVD targets, this is used to derive the Android API version for the SDK addons and device files.

### `ABFS_CACHED_BUILD`

The ABFS cache and ABFS source mount path will be stored in a persistent volume for other builds to share.
Used in conjunction with `ABFS_CACHEMAN_TIMEOUT` and may improve future build times but at the cost of additional persistent volume storage.

### `ABFS_CACHEMAN_TIMEOUT`

Timeout in seconds for cacheman to wait on sync. Only effective when `ABFS_CACHED_BUILD` is enabled, so the cache can sync to persistent storage.

### `ABFS_CLEAN_CACHE`

Delete the ABFS cache directory prior to build.

### `POST_REPO_COMMAND`

Optional parameter that allows the user to include additional commands to run after the repo has been synced, or cloned.

The build/soong fix ABFS needs (Soong runs `soong_build` in a sandbox where the ABFS mount is read-only) is applied automatically by `aaos_environment.sh` based on `AAOS_REVISION`, so this parameter can stay empty:

| `AAOS_REVISION` | Applied automatically |
|-----------------|-----------------------|
| `android-14*`, `android-15.0.0_r4` | nothing |
| `android-15*` (tested: android-15.0.0_r36) | build/soong cherry-picks 3619490, 3619491, 3619492 (`ui/build/sandbox_linux.go`) plus the same `sed` on `ui/build/soong.go` as Android 16; the cherry-picks alone still fail with `read-only file system` when the Soong evaluation is not served from the ninjago cache |
| `android-16*` (tested: android-16.0.0_r3) | `sed` on `ui/build/soong.go` so `TOP` and `LOG_DIR` are passed to `soong_build` as sandbox paths |
| branch names (`android-latest-release`, `main`, custom) | nothing; the revision carries no Android version, so add the fix through this parameter if the build needs it |

Anything set here runs after the automatic fix. This is a single command line, so use of logical operators to execute subsequent commands is essential.

Useful for installing additional code, tools etc, prior to build.

e.g: [Pixel Devices](https://source.android.com/docs/automotive/start/pixelxl) where you need to download and extract vendor device images.

### `OVERRIDE_MAKE_COMMAND`

Optional parameter that allows the user to override the default target make command with their own.

This is a single command line, so use of logical operators to execute subsequent commands is essential.

### `AAOS_PARALLEL_BUILD_JOBS`

Optional parameter that allows the user to override the number of parallel build jobs.

Default: 16. Empty lets the build system decide. If the ABFS cacheman socket gets overloaded during the build, the value may be too high.

### `AAOS_GERRIT_MANIFEST_URL`

Optional for fetching a patch set from Gerrit.

This provides the URL for the Android repo manifest. Such as:

- https://dev.horizon-sdv.com/gerrit/android/platform/manifest (default Horizon manifest)

This is required in order to derive the path to patch within the source tree from the project name.

### `GERRIT_PROJECT` / `GERRIT_CHANGE_NUMBER / GERRIT_PATCHSET_NUMBER / GERRIT_TOPIC`

These are optional but allow the user to fetch a specific Gerrit patchset if required.

### `INSTANCE_RETENTION_TIME`

Keep the build VM instance and container running to allow user to connect to it. Useful for debugging build issues, determining target output archives etc. Time in minutes.

Access using `kubectl` e.g. `kubectl exec -it -n jenkins <pod name> -- bash` .

Reference [Fleet management](https://docs.cloud.google.com/kubernetes-engine/enterprise/multicluster-management/gateway) to fetch credentials for a fleet-registered cluster to be used in Connect Gateway, e.g.
- `gcloud container fleet memberships list`
- `gcloud container fleet memberships get-credentials sdv-cluster`

### `ABFS_VERSION`

The version of ABFS client to use for builds.

Note: this can change, so the `Seed Workloads` job supports this parameter to allow it to be replaced across all jobs.  You may replace locally but remember the `Seed` will overwrite when run again.

### `ABFS_CASFS_VERSION`

The version of the ABFS CASFS module expected on the build node. Since ABFS 0.1.14 the kernel module is not installed by the build: the `abfs-client-ubuntu` module runs a DaemonSet that installs the prebuilt `casfs-kmod-<kernel>_<version>` package or builds casfs from source for the node kernel, and the build only waits for `/sys/module/casfs`. The `abfs_repository_list.txt` artifact records the precompiled ABFS component versions that are available in the apt repository for download at the time of the build.

Note: this can change, so the `Seed Workloads` job supports this parameter to allow it to be replaced across all jobs.  You may replace locally but remember the `Seed` will overwrite when run again.

### `ABFS_REPOSITORY`

The ABFS aptitude repository to fetch the ABFS client artifacts, e.g. abfs-apt-alpha-public.

### `ABFS_SERVER`

The ABFS server endpoint (`host:port`) passed to `abfs config -w` during initialisation, default `abfs-server.abfs.svc.cluster.local:50051`.

Build pods run in the `jenkins` (Jenkins) or `workflows` (Argo) namespace, so the ABFS server Service in the `abfs` namespace must be addressed by its fully qualified name. The server is deployed by `abfs-service-module`; `server.service.internalLoadBalancer` additionally exposes it on an internal IP for clients outside the cluster.

Note: this can change, so the `Seed Workloads` job supports this parameter to allow it to be replaced across all jobs.

### `ABFS_CLIENT_FLAGS`

Flags passed to the ABFS client commands. Default `--disable-tls=true --auth-type none`.

Change this on an ABFS build to test session encryption or authentication.

### `UPLOADER_MANIFEST_SERVER`

Host passed to `abfs --manifest-server` (default `android.googlesource.com`). It must be the host the uploaders seed from (manifest URL of the active row in `abfs-uploader-config`). If the Horizon internal mirror is used to mirror the Android source and manifest repositories, the Horizon Gerrit domain can be used here to seed from that mirror.

### `AAOS_ARTIFACT_STORAGE_SOLUTION`

Define storage solution used to push artifacts.

Currently `GCS_BUCKET` default pushes to GCS bucket, if empty then nothing will be stored.

### `STORAGE_BUCKET_DESTINATION`

Lets you override the default artifact storage destination. If not set, the build derives it automatically, for example:

`gs://${ANDROID_BUILD_BUCKET_ROOT_NAME}/Android/Builds/AAOS_Builder_ABFS/<BUILD_NUMBER>`

The override must be a full GCS URI, including the `gs://` prefix, bucket name, and the artifact path. For example:

`gs://${ANDROID_BUILD_BUCKET_ROOT_NAME}/Android/Releases/010129`

### `STORAGE_LABELS`

Lets you add labels to the artifacts being uploaded to storage.

For GCS buckets, these labels can be applied as key=value pairs and can be provided as a comma-separated or space-separated list.

E.g. `Release=X.Y.Z,Workload=Android`

### `ENABLE_GEMINI_AI_ASSISTANT`

Enable Gemini AI to support in diagnosis of build and test failures.

### Gemini prompts

The job uses the prompt file from the repository only; there is no Jenkins parameter to override it (default: `build_prompt_v2.txt`).

### `GEMINI_COMMAND_LINE`

Interface for the headless [gemini-cli](https://geminicli.com/docs/cli/headless/).
Use this to specify settings such as the [Gemini model](https://ai.google.dev/gemini-api/docs/models) etc, e.g.
`--debug` to include debug output.
Note: Prompts are piped via `stdin` and output is redirected to a JSON file. Antigravity does not read this parameter.

### `AI_REVIEW_RUNTIME`

`agy` (Antigravity, default) or `gemini-cli`. Jenkins lists `agy` first. When the variable is unset, `run_ai_review.sh` uses `agy`. This job has no Argo workflow.

### `GEMINI_AI_EXECUTION_TIMEOUT_MINUTES`

Maximum duration in minutes before the review is terminated. Jenkins choices are `60`, `45`, `30`, `20`, `10`, `90`, and `120` (first choice `60`). When unset, `run_ai_review.sh` uses 60 minutes.

## SYSTEM VARIABLES <a name="system-variables"></a>

There are a number of system environment variables that are unique to each platform but required by Jenkins build, test and environment pipelines.

These are defined in Jenkins CasC `values-jenkins.yaml` and can be viewed in Jenkins UI under `Manage Jenkins` -> `System` -> `Global Properties` -> `Environment variables`.

These are as follows:

-   `ANDROID_BUILD_BUCKET_ROOT_NAME`
     - Defines the name of the Google Storage bucket that will be used to store build and test artifacts

-   `ABFS_BUILD_DOCKER_ARTIFACT_PATH_NAME`
    - Defines the registry path where the Docker image used by builds, tests and environments is stored.

-   `CLOUD_PROJECT`
    - The GCP project, unique to each project. Important for bucket, registry paths used in pipelines.

-   `CLOUD_REGION`
    - The GCP project region. Important for bucket, registry paths used in pipelines.

-   `CLOUD_ZONE`
    - The GCP project zone. Important for bucket, registry paths used in pipelines.

-   `GERRIT_CREDENTIALS_ID`
    - The credential for access to Gerrit, required for build pipelines.

-   `HORIZON_DOMAIN`
    - The URL domain which is required by pipeline jobs to derive URL for tools and GCP.

-   `HORIZON_SCM_URL`
    - The URL to the Horizon SDV Git repository.

-   `HORIZON_SCM_BRANCH`
    - The branch name the job will be configured for from `HORIZON_SCM_URL`.

-   `JENKINS_SERVICE_ACCOUNT`
    - Service account to use for pipelines. Required to ensure correct roles and permissions for GCP resources.

## KNOWN ISSUES <a name="known-issues"></a>

Known issues of ABFS builds are listed in one place: [ABFS integration → Known issues](../abfs.md#knownissues).

