# POSIX Target Test

## Table of contents
- [Introduction](#introduction)
- [Prerequisites](#prerequisites)
- [Environment Variables/Parameters](#environment-variables)
- [System Variables](#system-variables)
- [Known Limitations](#known-limitations)

## Introduction <a name="introduction"></a>

This job enables users to test a prior build of the POSIX application on the OpenBSW POSIX platform.

**Test Configuration**

The test will launch MTK Connect, allowing users to connect to the POSIX host and access the application via the MTK Connect HOST API interface. The test behavior is controlled by the `LAUNCH_APPLICATION_NAME` parameter, which determines whether the application is launched automatically or user will launch manually when connected to the host.

### References <a name="references"></a>

- [Application Console](https://eclipse-openbsw.github.io/openbsw/sphinx_docs/doc/learning/console/index.html)
- [POSIX](https://eclipse-openbsw.github.io/openbsw/sphinx_docs/doc/platforms/posix/index.html#posix)

## Prerequisites<a name="prerequisites"></a>

One-time setup requirements.

- Before running this pipeline job, ensure that the following templates have been created by running the corresponding jobs:
  - Docker image template: `OpenBSW Workflows/Environment/Docker Image Template`

## Environment Variables/Parameters <a name="environment-variables"></a>

**Jenkins Parameters:** Defined in the groovy job definition `groovy/job.groovy`.

### `OPENBSW_DOWNLOAD_URL`

Storage URL pointing to the location of the POSIX target application image that was build using `BSW Builder`, e.g.`gs://${OPENBSW_BUILD_BUCKET_ROOT_NAME}/OpenBSW/Builds/BSW_Builder/<BUILD_NUMBER>/posix`

### `LAUNCH_APPLICATION_NAME`

Name of the application to launch, or empty to manually launch. Default is the standard POSIX target application.

If defined, then MTK Connect will launch the application automatically. If not defined, then user will find the
application under the `posix` directory and they many manually start.

### `IMAGE_TAG`

Specifies the name of the Docker image to be used when running this job.

The default value is defined by the `Seed Workloads` pipeline job. Users may override to provide a unique tag that describes the Linux distribution and tool chain versions.

### `POSIX_KEEP_ALIVE_TIME`

When using MTK Connect to test the POSIX images, the VM instance must be allowed to continue to run. This timeout, in
minutes, gives the tester time to keep the instance alive so they may work with the devices via MTK Connect.

## SYSTEM VARIABLES <a name="system-variables"></a>

There are a number of system environment variables that are unique to each platform but required by Jenkins build, test and environment pipelines.

These are defined in Jenkins CasC `jenkins.yaml` and can be viewed in Jenkins UI under `Manage Jenkins` -> `System` -> `Global Properties` -> `Environment variables`.

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

-   `HORIZON_GITHUB_URL`
    - The URL to the Horizon SDV GitHub repository.

-   `HORIZON_GITHUB_BRANCH`
    - The branch name the job will be configured for from `HORIZON_GITHUB_URL`.

-   `JENKINS_SERVICE_ACCOUNT`
    - Service account to use for pipelines. Required to ensure correct roles and permissions for GCP resources.

## Known Limitations<a name="known-limitations"></a>

**CAN Virtualisation:**

This is not supported currently because the POSIX target is running in a Docker container in kubernetes POD. CAN
virtualisation will be supported in later releases.

**Hardware Support:**

The NXP S32K148 is not supported in this test, this may be added in a future release.
