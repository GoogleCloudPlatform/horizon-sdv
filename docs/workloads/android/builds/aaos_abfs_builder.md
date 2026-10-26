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

This job is used to build Android Automotive virtual devices and platform targets using the source and build caches from the Android Build Fielsystem.

This pipeline/scripts supports builds for:

- [Android Virtual Devices](https://source.android.com/docs/automotive/start/avd/android_virtual_device) for use with [Android Studio](https://source.android.com/docs/automotive/start/avd/android_virtual_device#share-an-avd-image-with-android-studio-users)
- [Cuttlefish Virtual Devices](https://source.android.com/docs/devices/cuttlefish) for use with [CTS](https://source.android.com/docs/compatibility/cts) and emulators.
- [Pixel Tablets](https://source.android.com/docs/automotive/start/pixelxl) Reference hardware platforms.

The following provides examples of the environment variables and Jenkins build parameters that are required.

**Note:** the build, whether successful or not, will create the file `abfs_repository_list.txt` which can be used to correlate `ABFS_VERSION` and `ABFS_CASFS_VERSION` based on the build instance kernel revision. Versions do get updated and therefore it is best that users pay attention to the output of this file and update the `Seed Workflow` values for ABFS versions so as to utilise the latest provided by Google.

## Prerequisites<a name="prerequisites"></a>

One-time setup requirements.

- Before running this pipeline job, ensure that the following template has been created by running the corresponding job:
  - Docker image template: `Android Workflows/Environment/ABFS/Docker Image Template`

- Ensure ABFS server has been seeded with the required version of Android.

## Environment Variables/Parameters <a name="environment-variables"></a>

**Jenkins Parameters:** Defined in the groovy job definition `groovy/job.groovy`.

### `AAOS_REVISION`

The Android revision, i.e. branch or tag to build. Tested versions are below:

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
For ABFS this has default patches for soong to support building with the ABFS.

Some build targets already define this command, so if user updates this then the default will be overridden. This is a single command line, so use of logical operators to execute subsequent commands is essential.

Useful for installing additional code, tools etc, prior to build.

e.g: [Pixel Devices](https://source.android.com/docs/automotive/start/pixelxl) where you need to download and extract vendor device images.

### `OVERRIDE_MAKE_COMMAND`

Optional parameter that allows the user to override the default target make command with their own.

This is a single command line, so use of logical operators to execute subsequent commands is essential.

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

The version of ABFS CASFS module to install on the client. This version in the most part should be the same as ABFS_VERSION but in some cases the CASFS module needs to be updated to match the kernel version and thus may not align with the common ABFS version.

Note: this can change, so the `Seed Workloads` job supports this parameter to allow it to be replaced across all jobs.  You may replace locally but remember the `Seed` will overwrite when run again.

### `ABFS_REPOSITORY`

The ABFS aptitude repository to fetch the ABFS client artifacts, e.g. abfs-apt-alpha-public.

### `UPLOADER_MANIFEST_SERVER`

ABFS manifest source URL. Used for seeding ABFS builds, blobs/objects.

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
Note: Prompts are piped via `stdin` and output is redirected to a JSON file.

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

For details, reach out to Google.

