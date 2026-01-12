# GCS Utilities

## Table of contents
- [Introduction](#introduction)
- [Prerequisites](#prerequisites)
- [Environment Variables/Parameters](#environment-variables)
- [System Variables](#system-variables)

## Introduction <a name="introduction"></a>

This job allows the user to change the storage class of all objects in a bucket path based on the metadata that is set on them.
The user can choose to change objects with specific metadata (matching the provided key and/or value),
objects with any metadata or objects with no metadata.

## Prerequisites<a name="prerequisites"></a>

Run the `Jenkins → Utilities → Docker Image Template` to create a container for kubernetes that will support this job.

## Environment Variables/Parameters <a name="environment-variables"></a>

**Jenkins Parameters:** Defined in the groovy job definition `groovy/job.groovy`.

### `BUCKET_PATH`

Path to desired folder (ending with / or /*) (e.g. `gs://bucketname/path/`)

### `FILTER_TYPE`
Type of filtering to be done: Specific Metadata, Any Metadata, No Metadata

### `KEYVALUE_PAIRS`
Applicable only if 'Specific Metadata' filter type is selected.
Specify the keys and/or key/value pairs to filter the output

Note: if left blank, no  objects are selected

e.g. `key1=1 key2=2 key5 key6`

### `STORAGE_CLASS`
The new storage class for the filtered objects: STANDARD, NEARLINE, COLDLINE or ARCHIVE

Note that different storages classes incur different costs and may have minimum storage durations
([more info](https://cloud.google.com/storage/pricing)).


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
