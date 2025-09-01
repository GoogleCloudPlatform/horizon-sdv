# Android CTS Build

## Table of contents
- [Introduction](#introduction)
- [Prerequisites](#prerequisites)
- [Environment Variables/Parameters](#environment-variables)
  * [Targets](#targets)
- [System Variables](#system-variables)
- [Known Issues](#known-issues)

## Introduction <a name="introduction"></a>

This pipeline builds the Android Automotive Compatibility Test Suite ([CTS](https://source.android.com/docs/compatibility/cts)) test harness from the specified code base.

The following are examples of the environment variables and Jenkins build parameters that can be used.

## Prerequisites<a name="prerequisites"></a>

One-time setup requirements.

- Before running this pipeline job, ensure that the following template has been created by running the corresponding job:
  - Docker image template: `Android Workflows/Environment/Docker Image Template`

## Environment Variables/Parameters <a name="environment-variables"></a>

**Jenkins Parameters:** Defined in the groovy job definition `groovy/job.groovy`.

### `AAOS_GERRIT_MANIFEST_URL`

This provides the URL for the Android repo manifest. Such as:

- https://dev.horizon-sdv.com/gerrit/android/platform/manifest (default Horizon manifest)
- https://android.googlesource.com/platform/manifest (Google OSS manifest)

### `AAOS_REVISION`

The Android revision, i.e. branch or tag to build. Tested versions are below:

- `horizon/android-14.0.0_r30` (ap1a)
- `horizon/android-14.0.0_r74` (ap2a - refer to Known Issues)
- `horizon/android-15.0.0_r4` (ap3a)
- `horizon/android-15.0.0_r20` (bp1a)
- `horizon/android-15.0.0_r32` (bp1a)
- `horizon/android-15.0.0_r36` (bp1a - default)
- `android-14.0.0_r30` (ap1a)
- `android-14.0.0_r74` (ap2a, refer to Known Issues)
- `android-15.0.0_r4` (ap3a)
- `android-15.0.0_r20` (bp1a)
- `android-15.0.0_r32` (bp1a)
- `android-15.0.0_r36` (bp1a)

### `AAOS_LUNCH_TARGET` <a name="targets"></a>

The Android cuttlefish target to build CTS from. Must be one of the `aosp_cf` targets.

Reference: [Codenames, tags, and build numbers](https://source.android.com/docs/setup/reference/build-numbers)

Examples:

- Virtual Devices:
    -   `aosp_cf_x86_64_auto-ap1a-userdebug`
    -   `aosp_cf_x86_64_auto-ap2a-userdebug`
    -   `aosp_cf_x86_64_auto-ap3a-userdebug`
    -   `aosp_cf_x86_64_auto-bp1a-userdebug`
    -   `aosp_cf_arm64_auto-ap1a-userdebug`
    -   `aosp_cf_arm64_auto-ap2a-userdebug`
    -   `aosp_cf_arm64_auto-ap3a-userdebug`
    -   `aosp_cf_arm64_auto-bp1a-userdebug`

### `ANDROID_VERSION`

This specifies which build disk pool to use for build cache. If `default` then the job will determine the
pool based on `AAOS_REVISION`.

### `AAOS_CLEAN`

Option to clean the build workspace, either fully or simply for the `AAOS_LUNCH_TARGET` target defined.

### `GERRIT_REPO_SYNC_JOBS`

Defines the number of parallel sync jobs when running `repo sync`. Default provided by Seeding Android workloads.
The minimum is 1 and the maximum is 24.

### `INSTANCE_RETENTION_TIME`

Keep the build VM instance and container running to allow user to connect to it. Useful for debugging build issues, determining target output archives etc. Time in minutes.

Access using `kubectl` e.g. `kubectl exec -it -n jenkins <pod name> -- bash` from `bastion` host.

### `AAOS_ARTIFACT_STORAGE_SOLUTION`

Define storage solution used to push artifacts.

Currently `GCS_BUCKET` default pushes to GCS bucket, if empty then nothing will be stored.

### `GERRIT_PROJECT` / `GERRIT_CHANGE_NUMBER` / `GERRIT_PATCHSET_NUMBER`

These are optional but allow the user to fetch a specific Gerrit patchset if required.

## SYSTEM VARIABLES <a name="system-variables"></a>

There are a number of system environment variables that are unique to each platform but required by Jenkins build, test and environment pipelines.

These are defined in Jenkins CasC `jenkins.yaml` and can be viewed in Jenkins UI under `Manage Jenkins` -> `System` -> `Global Properties` -> `Environment variables`.

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

-   `GERRIT_CREDENTIALS_ID`
    - The credential for access to Gerrit, required for build pipelines.

-   `HORIZON_DOMAIN`
    - The URL domain which is required by pipeline jobs to derive URL for tools and GCP.

-   `HORIZON_GITHUB_URL`
    - The URL to the Horizon SDV GitHub repository.

-   `HORIZON_GITHUB_BRANCH`
    - The branch name the job will be configured for from `HORIZON_GITHUB_URL`.

-   `JENKINS_AAOS_BUILD_CACHE_STORAGE_PREFIX`
    - This identifies the Persistent Volume Claim (PVC) prefix that is used to provision persistent storage for build cache, ensuring efficient reuse of cached resources across builds.  The default is [`pd-balanced`](https://cloud.google.com/compute/docs/disks/performance), which strikes a balance between optimal performance and cost-effectiveness.

-   `JENKINS_SERVICE_ACCOUNT`
    - Service account to use for pipelines. Required to ensure correct roles and permissions for GCP resources.

## KNOWN ISSUES <a name="known-issues"></a>

Refer to `docs/workloads/android/builds/aaos_builder.md` for details.
