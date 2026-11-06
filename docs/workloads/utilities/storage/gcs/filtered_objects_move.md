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

Path to desired folder - can contain any number of wildcard characters (e.g. `gs://bucketname/path/` or `gs://bucketname/subpath/*x86*`)

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
    - Defines the registry path of the Utilities Docker image used by the Utilities pipeline jobs (GCS storage jobs, Gemini AI Assistant).

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
