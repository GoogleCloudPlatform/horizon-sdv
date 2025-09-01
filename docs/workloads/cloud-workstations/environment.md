# Environment > Docker Image Template

## Table of contents
- [Introduction](#introduction)
- [Prerequisites](#prerequisites)
- [Environment Variables/Parameters](#environment-variables)
- [System Variables](#system-variables)

## Introduction <a name="introduction"></a>

The Jenkins pipeline `Environment > Docker Image Template` builds the container image that is used as pipeline environment on Kubernetes, for GCP Cloud Workstations workload jobs.

This pipeline need only be run once, or when Dockerfile is updated. There is an option not to push the resulting image to the registry, so that devs can test their changes before committing the image.

### References
- [Kaniko](https://github.com/GoogleContainerTools/kaniko)

## Prerequisites<a name="prerequisites"></a>

This depends only on [`kaniko`](https://github.com/GoogleContainerTools/kaniko) which should be installed by default.

## Environment Variables/Parameters <a name="environment-variables"></a>

**Jenkins Parameters:** Defined in the groovy job definition `groovy/job.groovy`.

### `NO_PUSH`

Build the container image but don't push to the registry.

### `IMAGE_TAG`

This is the tag that will be applied when the container image is pushed to the registry. For the current release we
simply use `latest` because all pipelines that depend on this container image are using `latest`.

## System Variables <a name="system-variables"></a>

There are a number of system environment variables that are unique to each platform but required by this Jenkins Cloud Workstation environment pipeline.

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

-   `CLOUD_WS_WORKLOADS_ENV_IMAGE_NAME`
    - Name of the Docker image on GCP Artifact registry, that is used as an environment for Cloud Workstations workload pipelines.
