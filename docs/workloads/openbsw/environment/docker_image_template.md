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

The Dockerfile used in this project follows the same tooling layout as [OpenBSW `docker/development/Dockerfile`](https://github.com/eclipse-openbsw/openbsw/blob/main/docker/development/Dockerfile) (ARM GCC, LLVM embedded, Python venv bootstrap, **Rust via rustup** for `posix-rust` / `s32k148-rust-gcc`), but has been customized for Horizon-SDV and Google Cloud Platform. The job provides build arguments for the Linux distribution, toolchain URLs, **and Rust (`RUST_DEFAULT_TOOLCHAIN`, `RUSTUP_PROFILE`, `RUST_EMBEDDED_TARGET`, `CBINDGEN_VERSION`)** so upgrades do not require editing the Dockerfile.

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

### `LLVM_ARM_TOOLCHAIN_URL`

URL of LLVM Embedded Toolchain for Arm.

### `NODEJS_VERSION`

The NodeJS version to install in the Docker image. This is required in order to use MTK Connect with the container.

### `PYTHON_VERSION`

Python version version to install

### `TREEFMT_URL`

URL of the treefmt tools to install in the Docker image.

### `RUST_DEFAULT_TOOLCHAIN`

Rust toolchain passed to `rustup` (`--default-toolchain`), e.g. `1.90.0` or `stable`.

### `RUSTUP_PROFILE`

`rustup` install profile (`minimal`, `default`, etc.).

### `RUST_EMBEDDED_TARGET`

Extra target triple installed with `rustup target add` for embedded Rust (e.g. `thumbv7em-none-eabihf` for OpenBSW S32K148-class builds).

### `CBINDGEN_VERSION`

Version pin for `cargo install cbindgen --locked`; bump when OpenBSW upstream requires it.

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

### `ENABLE_GEMINI_AI_ASSISTANT`

Enable Gemini AI to support in diagnosis of build and test failures.

### `GEMINI_CLI_VERSION`

The version of gemini-cli to be installed.
Run `npm view @google/gemini-cli versions` for a full list of valid versions.

### `ANTIGRAVITY_SDK_VERSION`

The version of antigravity SDK to be installed.
Run `pip index versions google-antigravity` for a full list of valid versions.

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

-   `HORIZON_SCM_URL`
    - The URL to the Horizon SDV git repository.

-   `HORIZON_SCM_BRANCH`
    - The branch name the job will be configured for from `HORIZON_SCM_URL`.

-   `JENKINS_SERVICE_ACCOUNT`
    - Service account to use for pipelines. Required to ensure correct roles and permissions for GCP resources.
