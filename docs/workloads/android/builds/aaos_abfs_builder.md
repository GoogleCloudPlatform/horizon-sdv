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

- `android-15.0.0_r36` (bp1a)

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

### `GERRIT_PROJECT` / `GERRIT_CHANGE_NUMBER / GERRIT_PATCHSET_NUMBER`

These are optional but allow the user to fetch a specific Gerrit patchset if required.

### `INSTANCE_RETENTION_TIME`

Keep the build VM instance and container running to allow user to connect to it. Useful for debugging build issues, determining target output archives etc. Time in minutes.

Access using `kubectl` e.g. `kubectl exec -it -n jenkins <pod name> -- bash` from `bastion` host.

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


## SYSTEM VARIABLES <a name="system-variables"></a>

There are a number of system environment variables that are unique to each platform but required by Jenkins build, test and environment pipelines.

These are defined in Jenkins CasC `jenkins.yaml` and can be viewed in Jenkins UI under `Manage Jenkins` -> `System` -> `Global Properties` -> `Environment variables`.

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

-   `HORIZON_GITHUB_URL`
    - The URL to the Horizon SDV GitHub repository.

-   `HORIZON_GITHUB_BRANCH`
    - The branch name the job will be configured for from `HORIZON_GITHUB_URL`.

-   `JENKINS_SERVICE_ACCOUNT`
    - Service account to use for pipelines. Required to ensure correct roles and permissions for GCP resources.

## KNOWN ISSUES <a name="known-issues"></a>

For details, reach out to Google.

