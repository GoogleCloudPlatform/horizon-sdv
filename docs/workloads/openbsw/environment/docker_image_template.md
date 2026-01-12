# Docker Image Template

## Table of contents
- [Introduction](#introduction)
- [Prerequisites](#prerequisites)
- [Environment Variables/Parameters](#environment-variables)
- [System Variables](#system-variables)

## Introduction <a name="introduction"></a>

This pipeline builds the container image used on Kubernetes for building and testing OpenBSW targets, together with miscellaneous environment pipelines.

This need only be run once, or when Dockerfile is updated. There is an option not to push the resulting image to the registry, so that devs can test their changes before committing the image.

### Dockerfile Overview

The Dockerfile used in this project is based on the [Eclipse Foundation OpenBSW Dockerfile](https://github.com/eclipse-openbsw/openbsw/blob/main/docker/Dockerfile.dev), but has been customized for Horizon-SDV and Google Cloud Platform. Additionally, the job provide a flexible mechanism for users to update the tools and Linux distribution used to create the Docker image, which is utilized for builds and tests jobs.

### References
- [buildkit](https://hub.docker.com/r/moby/buildkit)
- [Welcome to Eclipse OpenBSW](https://eclipse-openbsw.github.io/openbsw/sphinx_docs/doc/index.html) GitHub repo.
- [Eclipse Foundation OpenBSW](https://github.com/eclipse-openbsw/openbsw) documentation.

## Prerequisites<a name="prerequisites"></a>

This depends only on [`buildkit`](https://hub.docker.com/r/moby/buildkit) which should be installed by default.

## Environment Variables/Parameters <a name="environment-variables"></a>

**Jenkins Parameters:** Defined in the groovy job definition `groovy/job.groovy`.

### `NO_PUSH`

Build the container image but don't push to the registry.

### `IMAGE_TAG`

This is the tag that will be applied when the container image is pushed to the registry. The default value is defined by
the `Seed Workloads` pipeline job. Users may override to provide a unique tag that describes the Linux distribution and
tool chain versions.

### `OPENBSW_GIT_URL`

This provides the URL for the OpenBSW repository. Such as:
- https://github.com/eclipse-openbsw/openbsw.git

### `OPENBSW_GIT_BRANCH`

This provides the branch/tag revision for the OpenBSW repository.

### `POST_GIT_CLONE_COMMAND`

Useful to pin OpenBSW to a particular sha1 when creating the python test packages.

### `LINUX_DISTRIBUTION`

Define the Linux Distribution to create the Docker image from. Values must be supported by the Dockerfile `FROM` instruction.

### `ARM_TOOLCHAIN_URL`

User may override the default ARM GNU toolchain that will be installed in the Docker image and used for builds. Available toolchains are provided under [Arm GNU Toolchain Downloads](https://developer.arm.com/downloads/-/arm-gnu-toolchain-downloads).

### `CLANG_TOOLS_URL`

URL of the CLANG tools to install in the Docker image.

### `CMAKE_URL`

URL of the CMAKE shell script to install in the Docker image.

### `LLVM_ARM_TOOLCHAIN_URL`

URL of LLVM Embedded Toolchain for Arm.

### `LLVM_PROJECT_URL`

URL of LLVM Compiler Infrastructure.

### `NODEJS_VERSION`

The NodeJS version to install in the Docker image. This is required in order to use MTK Connect with the container.

### `PYELFTOOLS_VERSION`

pyelftools package version to install

### `PYTHON_VERSION`

Python version version to install

### `SSCACHE_URL`

URL of Shared Compilation Cache.

### `TREEFMT_URL`

URL of the treefmt tools to install in the Docker image.

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

-   `OPENBSW_BUILD_DOCKER_ARTIFACT_PATH_NAME`
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
