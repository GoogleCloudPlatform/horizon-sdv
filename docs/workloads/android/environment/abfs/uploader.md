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

# ABFS Uploaders Creation

## Table of contents
- [Introduction](#introduction)
- [Prerequisites](#prerequisites)
- [Environment Variables/Parameters](#environment-variables)
- [System Variables](#system-variables)

## Introduction <a name="introduction"></a>

This job creates a virtual machine (VM) instance for the ABFS Uploaders, which is required for the ABFS build job to mount the ABFS source(cache).<br/>
The ABFS Uploader VM instances will be start to seed the ABFS server with the requested Android revision.

## Prerequisites<a name="prerequisites"></a>

Before creating the ABFS Uploader(s) VM instance, the following dependencies must be met:

- **Seed Workloads**: Android workload must be seeded to ensure the common parameters are set for the job.
- **Service Account Creation**: The abfs-server service account must be created in the GCP project.
- **ABFS License Deployment**: The ABFS license provided by Google must be deployed on the platform via Jenkins.
- **Docker Infra Image Template Job**:The Docker Infra Image Template job must be run, and the Docker image must be available in the registry.
- **ABFS Server**: The ABFS server must have been created and started for uploader to seed the server.

Consider using `Get Uploader Details` to ensure the uploader instances have been provisioned correctly.

Additional details are available in `docs/workloads/android/abfs.md`.


## Environment Variables/Parameters <a name="environment-variables"></a>

**Jenkins Parameters:** Defined in the groovy job definition `groovy/job.groovy`.

### `ABFS_TERRAFORM_ACTION`

The action to perform to create, destroy, stop, start, restart server.

- `APPLY`: use this to create the instance based on the set of defined parameters.
- `DESTROY`: use this to delete the instance.
- `STOP`|`START`|`RESTART`: useful for stopping expensive instances and starting when required.

### `ABFS_LICENSE_B64`

Google provided license file converted to base64. This is mandatory for `ABFS_TERRAFORM_ACTION` `APPLY` actions. Without this license, the ABFS uploaders will not be functional.

### `UPLOADER_COUNT`

This defines the number of uploader instances to create in order to seed the server with the specific Android version.

### `UPLOADER_MACHINE_TYPE`

This defines the VM instance machine type. The default is what Google recommended but users are free to choose their own type.

### `UPLOADER_DATADISK_SIZE_GB`

Disk size for each uploader instance.

### `INFRA_IMAGE_TAG`

This is the tag of the container/image created by `Docker Infra Instance Template` that will be used by the job to create the uploader VM instances.

Default is always `latest`.

### `ABFS_VERSION`

The version of ABFS the server will be created from, i.e. the docker file it will pull from Google registry.

Note: this can change, so the `Seed Workloads` job supports this parameter to allow it to be replaced across all jobs.  You may replace locally but remember the `Seed` will overwrite when run again.

### `DOCKER_REGISTRY_NAME`

This is the Docker registry the ABDS docker image/containers for server will be pulled from.

### `UPLOADER_MANIFEST_SERVER`

This is the Gerrit server from which the uploader will seed the ABFS server.

### `UPLOADER_MANIFEST_FILE`

This is the Gerrit manifest file from which the uploader will seed the ABFS server based on `UPLOADER_GIT_BRANCH`.

### `UPLOADER_GIT_BRANCH`

This is the Gerrit Android branch/tag list the uploader will seed to the ABFS server.

Format: JSON array of strings.

Examples:
- Single branch: `["android-15.0.0_r36"]`
- Multiple branches: `["android-15.0.0_r36","android-16.0.0_r3"]`

Usage notes:
- Keep branch names explicit in the JSON list.
- Adding a branch adds it to desired seeding state.
- Removing a branch/tag from the list removes it from desired seeding state.

### `GOOGLE_ABFS_TERRAFORM_GIT_URL`

The URL for Google Terraform modules for ABFS.

### `GOOGLE_ABFS_TERRAFORM_VERSION`

The git tag or sha1 for Google Terraform modules for ABFS. Default is `v0.10.0`.

### `UPLOADER_MANIFEST_SCHEME`

Optional.

Purpose: set the URL scheme used by uploaders when connecting to the manifest server.

Format: string (for example `https` or `http`).

Default: `https`.

### `ABFS_EXTRA_PARAMS`

Optional.

Purpose: pass advanced ABFS runtime flags for uploader instances.

Format: JSON array of strings, e.g. `["--flag=value","--other-flag"]`.

Default: `[]`.

### `ABFS_GERRIT_UPLOADER_EXTRA_PARAMS`

Optional.

Purpose: pass advanced flags specifically to the `gerrit upload-daemon` subcommand.

Format: JSON array of strings.

Default: `[]`.

### `ABFS_ENABLE_GIT_LFS`

Optional.

Purpose: enable Git LFS support in uploader workflows when repositories use large file storage.

Format: boolean (`true` or `false`).

Default: `false`.

### `PRE_START_HOOKS`

Optional.

Purpose: provide custom pre-start scripts for environment-specific setup before uploader runtime starts.

Format: absolute local directory path string.

Default: empty value in Jenkins, which results in Terraform module default `null` (no hooks).

Behavior:
- The directory is passed to Terraform as `pre_start_hooks`.
- Hook scripts are copied/mounted by the uploader module and executed during pre-start.

Recommended usage:
- Keep scripts idempotent and non-interactive.
- Keep scripts short and deterministic to avoid startup delays.
- Use executable scripts and a predictable naming convention/order.

### `ABFS_COS_IMAGE_REF`
Defines the ABFS Containerized OS images used on server and uploader instances.

## SYSTEM VARIABLES <a name="system-variables"></a>

There are a number of system environment variables that are unique to each platform but required by Jenkins build, test and environment pipelines.

These are defined in Jenkins CasC `values-jenkins.yaml` and can be viewed in Jenkins UI under `Manage Jenkins` -> `System` -> `Global Properties` -> `Environment variables`.

These are as follows:

-   `INFRA_DOCKER_ARTIFACT_PATH_NAME`
    - Defines the registry path where the Docker image used for ABFS infrastructure jobs is stored.

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
