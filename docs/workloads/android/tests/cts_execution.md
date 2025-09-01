# CTS Execution Pipeline

## Table of contents
- [Introduction](#introduction)
- [Prerequisites](#prerequisites)
- [Environment Variables/Parameters](#environment-variables)
- [Example Usage](#examples)
- [System Variables](#system-variables)
- [Known Issues](#known-issues)

## Introduction <a name="introduction"></a>

This pipeline is run on GCE Cuttlefish VM instances from the instance templates that were previously created by the environment pipeline. It allows users to run the Compatibility Test Suite (CTS) against their Cuttlefish virtual device (CVD) builds.

The pipeline first runs CVD on the Cuttlefish VM Instance to instantiate the specified number of devices and then runs CTS against the resulting virtual devices (tradefed - the tool used by CTS can spread / shard the tests across the multiple virtual devices).

Note:

- This pipeline offers the flexibility to run using a user-defined CTS suite (built by the `CTS Builder` pipeline) instead of the default Android 14 and Android 15 CTS suites provided by google.
- It allows user to enable MTK Connect should they wish to view the virtual devices during testing (e.g. useful for UI tests).
- It allows users to keep the cuttlefish virtual devices alive for a certain amount of time after the CTS run has completed in order to facilitate debugging via MTK Connect. MTK Connect must be enabled for this option.

### References <a name="references"></a>

- [Cuttlefish Virtual Devices](https://source.android.com/docs/devices/cuttlefish) for use with [CTS](https://source.android.com/docs/compatibility/cts) and emulators.
- [Compatibility Test Suite downloads](https://source.android.com/docs/compatibility/cts/downloads)

## Prerequisites<a name="prerequisites"></a>

One-time setup requirements.

- Before running this pipeline job, ensure that the following templates have been created by running the corresponding jobs:
  - Docker image template: `Android Workflows/Environment/Docker Image Template`
  - Cuttlefish instance template: `Android Workflows/Environment/CF Instance Template`
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

URL is of the form `gs://<ANDROID_BUILD_BUCKET_ROOT_NAME>/Android/Builds/AAOS_Builder/<BUILD_NUMBER>` where `ANDROID_BUILD_BUCKET_ROOT_NAME` is a system environment variable defined in Jenkins CasC `jenkins.yaml` and `BUILD_NUMBER` is the Jenkins build number.

### `CUTTLEFISH_INSTALL_WIFI`

This allows the user to install Wifi utility APK on all Cuttlefish virtual devices.

### `ANDROID_VERSION`

Defines the Android and thus CTS version to use. The Cuttlefish VM Instance is already pre-installed with Android 14 CTS and Android 15 CTS, so this defines which version to use.

### `CTS_DOWNLOAD_URL`

Optional.

This allows the user to use their own CTS that was built using the `CTS Builder` build job.

The URL must point to the bucket where the Android CTS archive is stored:

- `android-cts.zip`

URL is of the form `gs://<ANDROID_BUILD_BUCKET_ROOT_NAME>/Android/Builds/CTS_Builder/<BUILD_NUMBER>` where `ANDROID_BUILD_BUCKET_ROOT_NAME` is a system environment variable defined in Jenkins CasC `jenkins.yaml` and `BUILD_NUMBER` is the Jenkins build number.

### `CTS_TESTPLAN`

This defines the CTS test plan that will be run. Default is: `cts-system-virtual` which is only available in Android 15.

Android 14 users should pick a test plan that is compatible with their version of Cuttlefish.

Note: `cts-virtual-device-stable` was the previous default and takes less time than `cts-system-virtual`.

### `CTS_MODULE`

Optional.

This defines the CTS test module that will be run. Default is: `CtsDeqpTestCases` but if field is left empty, all CTS test modules will be run.
Note: `CtsHostsideNumberBlockingTestCases` is the previous default simply because it was quick.

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

### `CUTTLEFISH_KEEP_ALIVE_TIME`

If wishing to debug HOST using MTK Connect, Cuttlefish VM instance must be allowed to continue to run. This timeout, in
minutes, gives the tester time to keep the instance alive so they may work with the host via MTK Connect.

It is only applicable when `MTK_CONNECT_ENABLE` is enabled.

### `CVD_ADDITIONAL_FLAGS`

Append additional flags to `cvd` command, e.g. --display0=width=1920,height=1080,dpi=160

## Example Usage <a name="examples"></a>

Refer to `docs/workloads/android/tests/cvd_launcher.md` for an example of how to create and set up a test instance and boot the Cuttlefish Virtual Devices. Once the devices are booted, CTS tests can be run as follows:


```
ANDROID_VERSION=14 \
./workloads/android/pipelines/tests/cts_execution/cts_initialise.sh
CTS_TESTPLAN="cts-system-virtual" \
CTS_MODULE="CtsDeqpTestCases" \
CTS_TIMEOUT=600 \
SHARD_COUNT=1 \
./workloads/android/pipelines/tests/cts_execution/cts_execution.sh
```

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

-   `HORIZON_DOMAIN`
    - The URL domain which is required by pipeline jobs to derive URL for tools and GCP.

-   `HORIZON_GITHUB_URL`
    - The URL to the Horizon SDV GitHub repository.

-   `HORIZON_GITHUB_BRANCH`
    - The branch name the job will be configured for from `HORIZON_GITHUB_URL`.

-   `JENKINS_SERVICE_ACCOUNT`
    - Service account to use for pipelines. Required to ensure correct roles and permissions for GCP resources.

## KNOWN ISSUES <a name="known-issues"></a>

Refer to `docs/workloads/android/tests/cvd_launcher.md` for details.
