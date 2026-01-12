# Docker Image Template

## Table of contents
- [Introduction](#introduction)
- [Prerequisites](#prerequisites)
- [Environment Variables/Parameters](#environment-variables)
- [System Variables](#system-variables)

## Introduction <a name="introduction"></a>

This pipeline builds the container image used on Kubernetes for building use with Android Build File System(ABFS) targets and miscellaneous environment pipelines.

This need only be run once, or when Dockerfile is updated. There is an option not to push the resulting image to the registry, so that devs can test their changes before committing the image.

### References
- [buildkit](https://hub.docker.com/r/moby/buildkit)

## Prerequisites<a name="prerequisites"></a>

This depends only on [`buildkit`](https://hub.docker.com/r/moby/buildkit) which should be installed by default.

## Environment Variables/Parameters <a name="environment-variables"></a>

**Jenkins Parameters:** Defined in the groovy job definition `groovy/job.groovy`.

### `NO_PUSH`

Build the container image but don't push to the registry.

### `IMAGE_TAG`

This is the tag that will be applied when the container image is pushed to the registry. For the current release we
simply use `latest` because all pipelines that depend on this container image are using `latest`.

### `LINUX_DISTRIBUTION`

Define the Linux Distribution to create the Docker image from. Values must be supported by the Dockerfile `FROM` instruction.

### `GOOGLE_DISTRIBUTION_REGISTRY`

Defines the artifact registry required for ABFS package installation.

### `ABFS_DISTRIBUTION_REGISTRY`

Defines the ABFS binaries repository.

### `NODEJS_VERSION`

The version of NodeJS to install which is required by MTK Connect.

### `BUILDKIT_RELEASE_TAG`

The version of Buildkit to use to build the container image.

### `DOCKER_CREDENTIALS_URL`

URL of Google docker credentials helper, required to allow access to the project artifact registry.

### `GCLOUD_CLI_VERSION`

Version of [Google Cloud CLI](https://docs.cloud.google.com/sdk/docs/release-notes) to install.
Define `latest` if wishing to use the latest available version.

### `KUBECTL_VERSION`

Version of `kubectl` to install. The version is typically `1:${GCLOUD_CLI_VERSION}`.
Define `latest` if wishing to use the latest available version.

## SYSTEM VARIABLES <a name="system-variables"></a>

There are a number of system environment variables that are unique to each platform but required by Jenkins build, test and environment pipelines.

These are defined in Jenkins CasC `values-jenkins.yaml` and can be viewed in Jenkins UI under `Manage Jenkins` -> `System` -> `Global Properties` -> `Environment variables`.

These are as follows:

-   `ABFS_BUILD_DOCKER_ARTIFACT_PATH_NAME`
    - Defines the registry path where the Docker image used by builds, tests and environments is stored.

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
