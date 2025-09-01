# Workstation Images

## Table of contents
- [Introduction](#introduction)
- [Prerequisites](#prerequisites)
- [Environment Variables/Parameters](#environment-variables)
- [System Variables](#system-variables)
- [Known Issues](#known-issues)

## Introduction <a name="introduction"></a>

The Jenkins folder `Cloud Workstations > Workstation Images` houses pipelines that build Docker images that can later be used in Cloud Workstations as containers. 

The following pipelines are currently available:
- **Horizon Code OSS**: Builds a lightweight, general-purpose IDE based on open-source VS Code.
- **Horizon Android Studio**: Builds the standard IDE for developing Android applications.
- **Horizon Android Studio for Platform (ASfP)**: Builds the specialized IDE for AOSP and core Android OS development.

These pipelines need only be run once, or when their Dockerfile is updated. There is an option not to push the resulting image to the registry, so that devs can test their changes before committing the image.

### References
- [Kaniko](https://github.com/GoogleContainerTools/kaniko)
- [Base Image for Horizon Code OSS](https://github.com/AGBG-ASG/acn-horizon-sdv/blob/env/sbx/docs/us-central1-docker.pkg.dev/cloud-workstations-images/predefined/code-oss:latest)
- [Base Template for Horizon Android Studio for Platform (ASfP)](https://github.com/GoogleCloudPlatform/cloud-workstations-custom-image-examples/tree/main/examples/images/android-open-source-project/android-studio-for-platform)
- [Base Template for Horizon Android Studio](https://github.com/GoogleCloudPlatform/cloud-workstations-custom-image-examples/tree/main/examples/images/android/android-studio)

## Prerequisites<a name="prerequisites"></a>

All of these pipelines depend on [`kaniko`](https://github.com/GoogleContainerTools/kaniko) which should be installed by default.

## Environment Variables/Parameters <a name="environment-variables"></a>

**Jenkins Parameters:** All of these pipelines have the **same** following parameters defined in their respective groovy job definition `groovy/job.groovy`.

### `NO_PUSH`

Build the container image but don't push to the registry.

### `IMAGE_TAG`

This is the tag that will be applied when the container image is pushed to the registry. For the current release we
simply use `latest` because all pipelines that depend on this container image are using `latest`.

## System Variables <a name="system-variables"></a>

There are a number of system environment variables that are unique to each platform but required by these Jenkins Cloud Workstation `Workstation Images` pipelines.

These are defined in Jenkins CasC `jenkins.yaml` and can be viewed in Jenkins UI under `Manage Jenkins` -> `System` -> `Global Properties` -> `Environment variables`.

These are as follows:

-   `CLOUD_PROJECT`
    - The GCP project, unique to each project. Important for bucket, registry paths used in pipelines.

-   `CLOUD_REGION`
    - The GCP project region. Important for bucket, registry paths used in pipelines.

-   `HORIZON_GITHUB_URL`
    - The URL to the Horizon SDV GitHub repository.

-   `HORIZON_GITHUB_BRANCH`
    - The branch name the job will be configured for from `HORIZON_GITHUB_URL`.

-   `JENKINS_SERVICE_ACCOUNT`
    - Service account to use for pipelines. Required to ensure correct roles and permissions for GCP resources.

Below variables have their values defined in `gitops/env/stage2/values.yaml` and then referenced in Jenkins CasC `jenkins.yaml`.

-   `CLOUD_WS_HORIZON_CODE_OSS_IMAGE_NAME`
    - Name of the Docker image on GCP Artifact registry for VS Code IDE (`horizon-code-oss`), that is used in Cloud Workstations.
    - Used by pipeline: `Horizon Code OSS`

-   `CLOUD_WS_HORIZON_ASFP_IMAGE_NAME`
    - Name of the Docker image on GCP Artifact registry for Android Studio for Platform (`horizon-asfp`), that is used in Cloud Workstations.
    - Used by pipeline: `Horizon Android Studio for Platform (ASfP)`

-   `CLOUD_WS_HORIZON_ANDROID_STUDIO_IMAGE_NAME`
    - Name of the Docker image on GCP Artifact registry for Android Studio (`horizon-android-studio`), that is used in Cloud Workstations.
    - Used by pipeline: `Horizon Android Studio`

## Known Issues <a name="known-issues"></a>
THe builds for `Horizon Android Studio for Platform (ASfP)` and `Horizon Android Studio` pipelines may fail during a certain period citing 503 HTTP issues. This is a problem at Google's end - something out of our scope and we can only try building these images later again.