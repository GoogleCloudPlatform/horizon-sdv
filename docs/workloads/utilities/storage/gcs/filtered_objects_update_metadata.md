# GCS Utilities

## Table of contents
- [Introduction](#introduction)
- [Prerequisites](#prerequisites)
- [Environment Variables/Parameters](#environment-variables)
- [System Variables](#system-variables)

## Introduction <a name="introduction"></a>

This pipeline allows the user to find all objects in a bucket path which have the specified metadata set and update the value of that custom metadata item.

## Prerequisites<a name="prerequisites"></a>

Run the `Jenkins → Utilities → Docker Image Template` to create a container for kubernetes that will support this job.

## Environment Variables/Parameters <a name="environment-variables"></a>

**Jenkins Parameters:** Defined in the groovy job definition `groovy/job.groovy`.

### `BUCKET_PATH`

Path to desired folder (ending with / or /*) (e.g. `gs://bucketname/path/`)


### `KEY_OR_KEYVALUE_PAIR`

Key or key/value pair that will be used to select objects for update

e.g. `key` or `key1=1`

### `UPDATE_VALUE`

New value to be set for the key defined previously

e.g. `2` or `release`

## SYSTEM VARIABLES <a name="system-variables"></a>

There are a number of system environment variables that are unique to each platform but required by Jenkins build, test and environment pipelines.

These are defined in Jenkins CasC `values-jenkins.yaml` and can be viewed in Jenkins UI under `Manage Jenkins` -> `System` -> `Global Properties` -> `Environment variables`.
These are as follows:

-   `UTILITIES_DOCKER_ARTIFACT_PATH_NAME`
    - Defines the registry path where the Docker image used to create the ABFS Server and Uploader VM instances.

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
