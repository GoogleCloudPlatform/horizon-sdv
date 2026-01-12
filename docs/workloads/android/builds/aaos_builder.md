# Android Builds

## Table of contents
- [Introduction](#introduction)
- [Prerequisites](#prerequisites)
- [Environment Variables/Parameters](#environment-variables)
  * [Targets](#targets)
- [Example Usage](#examples)
  * [`aaos_environment.sh`](#aaos_environment)
  * [`aaos_initialise.sh`](#aaos_initialise)
  * [`aaos_build.sh`](#aaos_build)
  * [`aaos_avd_sdk.sh`](#aaos_avd_sdk)
  * [`aaos_storage.sh`](#aaos_storage)
- [System Variables](#system-variables)
- [Known Issues](#known-issues)

## Introduction <a name="introduction"></a>

This job is used to build Android Automotive virtual devices and platform targets from the provided source manifest.

This pipeline/scripts supports builds for:

- [Android Virtual Devices](https://source.android.com/docs/automotive/start/avd/android_virtual_device) for use with [Android Studio](https://source.android.com/docs/automotive/start/avd/android_virtual_device#share-an-avd-image-with-android-studio-users)
- [Cuttlefish Virtual Devices](https://source.android.com/docs/devices/cuttlefish) for use with [CTS](https://source.android.com/docs/compatibility/cts) and emulators.
- Reference hardware platforms such as [RPi](https://github.com/raspberry-vanilla/android_local_manifest) and [Pixel Tablets](https://source.android.com/docs/automotive/start/pixelxl).

The following provides examples of the environment variables and Jenkins build parameters that are required.
It also demonstrates how to run the scripts standalone on build instances.

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
- `horizon/android-15.0.0_r36` (bp1a)
- `horizon/android-16.0.0_r3` (bp3a - default)
- `android-14.0.0_r30` (ap1a)
- `android-15.0.0_r36` (bp1a)
- `android-16.0.0_r3` (bp3a)

### `AAOS_LUNCH_TARGET` <a name="targets"></a>

The Android target to build Android cuttlefish, virtual devices, Pixel and RPi targets.

Reference: [Codenames, tags, and build numbers](https://source.android.com/docs/setup/reference/build-numbers)

Examples:

- Virtual Devices:
    -   `sdk_car_x86_64-ap1a-userdebug` (`android-14.0.0_r30`)
    -   `sdk_car_x86_64-bp1a-userdebug` (`android-15.0.0_r36` )
    -   `sdk_car_x86_64-bp3a-userdebug` (`android-16.0.0_r3`)
    -   `sdk_car_arm64-ap1a-userdebug` (`android-14.0.0_r30`)
    -   `sdk_car_arm64-bp1a-userdebug` (`android-15.0.0_r36` )
    -   `sdk_car_arm64-bp3a-userdebug` (`android-16.0.0_r3`)
    -   `aosp_cf_x86_64_auto-ap1a-userdebug` (`android-14.0.0_r30`)
    -   `aosp_cf_x86_64_auto-bp1a-userdebug` (`android-15.0.0_r36` )
    -   `aosp_cf_x86_64_auto-bp3a-userdebug` (`android-16.0.0_r3`)
    -   `aosp_cf_arm64_auto-ap1a-userdebug` (`android-14.0.0_r30`)
    -   `aosp_cf_arm64_auto-bp1a-userdebug` (`android-15.0.0_r36` )
    -   `aosp_cf_arm64_auto-bp3a-userdebug` (`android-16.0.0_r3`)
-   Pixel Devices:
    -   `aosp_tangorpro_car-ap1a-userdebug` (`android-14.0.0_r30`)
    -   `aosp_tangorpro_car-bp1a-userdebug` (`android-15.0.0_r36` )
-   Raspberry Pi:
    -   `aosp_rpi4_car-ap1a-userdebug` (`android-14.0.0_r30`)
    -   `aosp_rpi5_car-ap1a-userdebug` (`android-14.0.0_r30`)
    -   `aosp_rpi4_car-bp1a-userdebug` (`android-15.0.0_r36` )
    -   `aosp_rpi5_car-bp1a-userdebug` (`android-15.0.0_r36` )
    -   `aosp_rpi4_car-bp3a-userdebug` (`android-16.0.0_r3`)
    -   `aosp_rpi5_car-bp3a-userdebug` (`android-16.0.0_r3`)

### `AAOS_BUILD_CTS`

This builds the Android Automotive Compatibility Test Suite ([CTS](https://source.android.com/docs/compatibility/cts)) test harness from the specified code base, if the `AAOS_LUNCH_TARGET` is that of Cuttlefish, i.e `aosp_cf`.

### `ANDROID_VERSION`

This specifies which build disk pool to use for build cache. If `default` then the job will determine the pool based on `AAOS_REVISION` and target. For sdk AVD targets, this is also used to derive the Android API version for the SDK addons and device files.

### `POST_REPO_INITIALISE_COMMAND`

Optional parameter that allows the user to include additional commands to run after the repo has been initialised.

Some build targets already define this command, so if user updates this then the default will be overridden. This is a single command line, so use of logical operators to execute subsequent commands is essential.

Useful for tasks such as updating manifests, such as those used to build RPi targets.

### `POST_REPO_COMMAND`

Optional parameter that allows the user to include additional commands to run after the repo has been synced, or cloned.

Some build targets already define this command, so if user updates this then the default will be overridden. This is a single command line, so use of logical operators to execute subsequent commands is essential.

Useful for installing additional code, tools etc, prior to build.

e.g: [Pixel Devices](https://source.android.com/docs/automotive/start/pixelxl) where you need to download and extract vendor device images.

### `OVERRIDE_MAKE_COMMAND`

Optional parameter that allows the user to override the default target make command with their own.

This is a single command line, so use of logical operators to execute subsequent commands is essential.

### `AAOS_CLEAN`

Option to clean the build workspace, either fully or simply for the `AAOS_LUNCH_TARGET` target defined.

### `GERRIT_REPO_SYNC_JOBS`

Defines the number of parallel sync jobs when running `repo sync`. Default provided by Seeding Android workloads.
The minimum is 1 and the maximum is 24.

### `INSTANCE_RETENTION_TIME`

Keep the build VM instance and container running to allow user to connect to it. Useful for debugging build issues, determining target output archives etc. Time in minutes.

Access using `kubectl` e.g. `kubectl exec -it -n jenkins <pod name> -- bash` .

Reference [Fleet management](https://docs.cloud.google.com/kubernetes-engine/enterprise/multicluster-management/gateway) to fetch credentials for a fleet-registered cluster to be used in Connect Gateway, e.g.
- `gcloud container fleet memberships list`
- `gcloud container fleet memberships get-credentials sdv-cluster`

### `AAOS_ARTIFACT_STORAGE_SOLUTION`

Define storage solution used to push artifacts.

Currently `GCS_BUCKET` default pushes to GCS bucket, if empty then nothing will be stored.

### `STORAGE_BUCKET_DESTINATION`

Lets you override the default artifact storage destination. If not set, the build derives it automatically, for example:

`gs://${ANDROID_BUILD_BUCKET_ROOT_NAME}/Android/Builds/AAOS_Builder/<BUILD_NUMBER>`

The override must be a full GCS URI, including the `gs://` prefix, bucket name, and the artifact path. For example:

`gs://${ANDROID_BUILD_BUCKET_ROOT_NAME}/Android/Releases/010129`

### `STORAGE_LABELS`

Lets you add labels to the artifacts being uploaded to storage.

For GCS buckets, these labels can be applied as key=value pairs and can be provided as a comma-separated or space-separated list.

E.g. `Release=X.Y.Z,Workload=Android`

### `USE_LOCAL_AOSP_MIRROR`

If checked, the build will use the AOSP Mirror setup in your GCP project to fetch Android source code during `repo sync`.
**Note:**
-  The AOSP Mirror must be setup prior to running this job. If not setup, the job will fail.
-  The setup jobs are in folder `Android Workflows -> Environment -> Mirror`.

### `AOSP_MIRROR_DIR_NAME`

This defines the directory name on the Filestore volume where the Mirror is located.
**Note:**
-  This is required if `USE_LOCAL_AOSP_MIRROR` is checked.
-  e.g. If you provided `my-mirror` when creating the mirror, provide the same value here.

### `GERRIT_PROJECT` / `GERRIT_CHANGE_NUMBER / GERRIT_PATCHSET_NUMBER / GERRIT_TOPIC`

These are optional but allow the user to fetch a specific Gerrit patchset if required.

## Example Usage <a name="examples"></a>

The following examples show how the scripts may be used standalone on build instances.

### `aaos_environment.sh` <a name="aaos_environment"></a>

This script is responsible for setting up the environment for the build scripts. It is included by all other scripts but can be run standalone to clean the build workspace and recreate.

`AAOS_CLEAN` can be set to either `CLEAN_BUILD`, `CLEAN_ALL` or `NO_CLEAN`.

Example 1: Delete the build `out` folder
```
AAOS_CLEAN=CLEAN_BUILD \
AAOS_LUNCH_TARGET=aosp_cf_x86_64_auto-bp1a-userdebug \
./workloads/android/pipelines/builds/aaos_builder/aaos_environment.sh
```

Example 2: Delete the full cache/build workspace
```
AAOS_CLEAN=CLEAN_ALL \
./workloads/android/pipelines/builds/aaos_builder/aaos_environment.sh
```

### `aaos_initialise.sh` <a name="aaos_initialise"></a>
This script is responsible for initialising the repos for the given manifest, branch and target.

Some targets have their own definitions for `POST_REPO_INITIALISE_COMMAND` and `POST_REPO_COMMAND` but these can be overridden.

Example 1: Initialise the repos for `aosp_cf_x86_64_auto-bp1a-userdebug`
```
AAOS_GERRIT_MANIFEST_URL=https://dev.horizon-sdv.com/gerrit/android/platform/manifest \
AAOS_REVISION=horizon/android-16.0.0_r3 \
AAOS_LUNCH_TARGET=aosp_cf_x86_64_auto-bp3a-userdebug \
./workloads/android/pipelines/builds/aaos_builder/aaos_initialise.sh
```

Example 2: Initialise the repos for `aosp_tangorpro_car-bp1a-userdebug` with Gerrit patch set.
```
AAOS_GERRIT_MANIFEST_URL=https://dev.horizon-sdv.com/gerrit/android/platform/manifest \
AAOS_REVISION=horizon/android-16.0.0_r3 \
AAOS_LUNCH_TARGET=aosp_tangorpro_car-bp3a-userdebug \
GERRIT_SERVER_URL=https://dev.horizon-sdv.com/gerrit \
GERRIT_CHANGE_NUMBER=82 \
GERRIT_PATCHSET_NUMBER=1 \
GERRIT_PROJECT=android/platform/packages/services/Car \
./workloads/android/pipelines/builds/aaos_builder/aaos_initialise.sh
```

### `aaos_build.sh` <a name="aaos_build"></a>
This script is responsible for building the given target.
```
AAOS_LUNCH_TARGET=sdk_car_x86_64-bp3a-userdebug \
AAOS_PARALLEL_BUILD_JOBS=64 \
./workloads/android/pipelines/builds/aaos_builder/aaos_build.sh
```

### `aaos_avd_sdk.sh` <a name="aaos_avd_sdk"></a>
This script creates the addon and devices files required for using AVD images with Android studio.

This is only applicable to AVD `sdk_car` based targets.

```
AAOS_LUNCH_TARGET=sdk_car_x86_64-bp3a-userdebug \
ANDROID_VERSION=16 \
./workloads/android/pipelines/builds/aaos_builder/aaos_avd_sdk.sh
```

### `aaos_storage.sh` <a name="aaos_storage"></a>
Not applicable in standalone mode. Storage is currently dependent on Jenkins `BUILD_NUMBER`.
Developers may upload their build artifacts to their own storage solution.

```
AAOS_LUNCH_TARGET=sdk_car_x86_64-bp3a-userdebug \
./workloads/android/pipelines/builds/aaos_builder/aaos_storage.sh
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

-    `AOSP_MIRROR_PRESET_FILESTORE_PVC_MOUNT_PATH_IN_CONTAINER`

-    `AOSP_MIRROR_PRESET_MIRROR_ROOT_SUBDIR_NAME`

-    `AOSP_MIRROR_DIR_NAME`

## KNOWN ISSUES <a name="known-issues"></a>

### `android-qpr1-automotiveos-release` and Cuttlefish Virtual Devices:

-   Avoid using for Cuttlefish Virtual Devices. Use `android-14.0.0_r30` instead.
    -   Black Screen, unresponsive, sluggish UI issues.

### `android-14.0.0_r30` and `tangorpro_car-bp1a`:

-   Fix the audio crash:

    -   Take a patch (https://android-review.googlesource.com/c/platform/packages/services/Car/+/3037383):
        -  Build with the following parameters:
	    - `GERRIT_PROJECT=platform/packages/services/Car`
	    - `GERRIT_CHANGE_NUMBER=3037383`
	    - `GERRIT_PATCHSET_NUMBER=2`
    -   Reference: [Pixel Tablets](https://source.android.com/docs/automotive/start/pixelxl)

### `android-14.0.0_r74` and some earlier releases:

-   To avoid DEX build issues for AAOSP builds on standalone build instances:

    -   Build with `WITH_DEXPREOPT=false`, e.g. `m WITH_DEXPREOPT=false`

-   Avoid surround view automotive test issues breaking builds:

    -   i.e. Unknown installed file for module `sv_2d_session_tests`/`sv_3d_session_tests`

    -   Either [Revert](https://android.googlesource.com/platform/platform_testing/+/b608b75b5f2a5f614bd75599023a45f3c321d4a9 "https://android.googlesource.com/platform/platform_testing/+/b608b75b5f2a5f614bd75599023a45f3c321d4a9") commit, or download the revert change from Gerrit review, e.g. upstream Gerrit patchset:
	    - `GERRIT_PROJECT=platform/platform_testing`
	    - `GERRIT_CHANGE_NUMBER=3183939`
	    - `GERRIT_PATCHSET_NUMBER=1`

	  or locally remove erroneous tests from native_test_list.mk:
	   -   `sed -i '/sv_2d_session_tests/,/sv_3d_session_tests/d' build/tasks/tests/native_test_list.mk`
       -   `sed -i 's/evsmanagerd_test \\/evsmanagerd_test/' build/tasks/tests/native_test_list.mk`

### `android-15.0.0_r10` and Cuttlefish Virtual Devices

-   Avoid multiple instances when running Cuttlefish. Instance 1 works fine, instance 2 and onwards do not work.
    -   `android-15.0.0_r4` is a more reliable release.

-   CTS (full) does not complete in timely manner:
    -   `android-15.0.0_r4`  : 43m29s
    -   `android-15.0.0_r10` : 3h and not completed (stuck in `CtsLibcoreOjTestCases` tests).
    -   `android-15.0.0_r10` : very new, latest and thus expect bugs.

### Cuttlefish and CTS

-   Some releases of Android have issues with launching cuttlefish virtual devices.
-   Consider tailoring the CTS Execution resources to suit those of the version under test.

### RPi Targets

-   [RPi](https://github.com/raspberry-vanilla/android_local_manifest) targets and branch names can change. Currently we define limited support in [aaos_environment.sh](#aaos_environment) but user may override the `repo init` command to include newer manifests and branch names that may not align with Google main branch. Simply update `POST_REPO_INITIALISE_COMMAND` with the RPi command that you prefer post `repo init`.

### Resource Limits (Pod)

-    The resource limits in the Jenkins Pod templates were chosen to give the optimal performance of builds. Higher values exposed issues with Jenkins kubernetes plugin and losing connection with the agent. e.g. The instance has 112 cores but some of those are required by Jenkins agent, 96 was most reliable to get the optimal performance.

### Build Cache Corruption

- The shared build cache can significantly accelerate build jobs, but it's not without risks. If a build job crashes or is aborted during initialization, the cache can become unstable, causing subsequent jobs to encounter issues with the `repo sync` command due to lingering lock files. To mitigate this, a retry and recovery process is triggered after multiple failed attempts, which involves deleting and recreating the cache. However, this process can substantially prolong the build job duration.
