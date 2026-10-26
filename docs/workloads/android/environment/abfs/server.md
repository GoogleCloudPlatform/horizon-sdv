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

# ABFS Server Creation

## Table of contents
- [Introduction](#introduction)
- [Prerequisites](#prerequisites)
- [Environment Variables/Parameters](#environment-variables)
- [System Variables](#system-variables)

## Introduction <a name="introduction"></a>

This job creates a virtual machine (VM) instance for the ABFS Server, which is required for the ABFS build job to mount the ABFS source(cache).
The ABFS Server VM instance will be seeded with the desired Android revision by the Uploaders.

## Prerequisites<a name="prerequisites"></a>

Before creating the ABFS Server VM instance, the following dependencies must be met:
- **Seed Workloads**: Android workload must be seeded to ensure the common parameters are set for the job.
- **Service Account Creation**: The abfs-server service account must be created in the GCP project.
- **ABFS License Deployment**: The ABFS license provided by Google must be deployed on the platform via Jenkins.
- **Docker Infra Image Template Job**:The Docker Infra Image Template job must be run, and the Docker image must be available in the registry.

Consider using `Get Server Details` to ensure the server has been provisioned correctly.

Additional details are available in `docs/workloads/android/abfs.md`.


## Environment Variables/Parameters <a name="environment-variables"></a>

**Jenkins Parameters:** Defined in the groovy job definition `groovy/job.groovy`.

### `ABFS_TERRAFORM_ACTION`

The action to perform to create, destroy, stop, start, restart server.

- `APPLY`: use this to create the instance based on the set of defined parameters.
- `DESTROY`: use this to delete the instance.
- `STOP`|`START`|`RESTART`: useful for stopping expensive instances and starting when required.

### `ABFS_LICENSE_B64`

Google provided license file converted to base64. This is mandatory for `ABFS_TERRAFORM_ACTION` `APPLY` actions. Without this license, the ABFS server will not be functional.

### `SERVER_MACHINE_TYPE`

This defines the VM instance machine type. The default is what Google recommended but users are free to choose their own type.

### `INFRA_IMAGE_TAG`

This is the tag of the container/image created by `Docker Infra Instance Template` that will be used by the job to create the server VM instance.

Default is always `latest`.

### `ABFS_VERSION`

The version of ABFS the server will be created from, i.e. the docker file it will pull from Google registry.

Note: this can change, so the `Seed Workloads` job supports this parameter to allow it to be replaced across all jobs.  You may replace locally but remember the `Seed` will overwrite when run again.

### `DOCKER_REGISTRY_NAME`

This is the Docker registry the ABDS docker image/containers for server will be pulled from.

### `GOOGLE_ABFS_TERRAFORM_GIT_URL`

The URL for Google Terraform modules for ABFS.

### `GOOGLE_ABFS_TERRAFORM_VERSION`

The git tag or sha1 for Google Terraform modules for ABFS. Default is `v0.10.0`.

### `ABFS_EXTRA_PARAMS`

Optional.

Purpose: pass advanced ABFS server runtime flags without changing Terraform/module code.

Format: JSON array of strings, e.g. `["--flag=value","--other-flag"]`.

Default: `[]` (no extra parameters).

### `EXISTING_BUCKET_NAME`

Optional.

Purpose: reuse an existing GCS bucket for ABFS data and avoid creating a new one.

Format: bucket name string.

Default: empty string `""` (Terraform creates and manages ABFS bucket).

### `ABFS_SPANNER_INSTANCE_MIN_NODES`

Optional.

Purpose: lower bound for ABFS Spanner instance autoscaling.

Format: integer value.

Default: `1`.

### `ABFS_SPANNER_INSTANCE_MAX_NODES`

Optional.

Purpose: upper bound for ABFS Spanner instance autoscaling.

Format: integer value.

Default: `10`.

### `ABFS_SPANNER_DATABASE_CREATE_TABLES`

Optional.

Purpose: control whether module creates ABFS Spanner tables from schema metadata.

Format: boolean (`true` or `false`).

Default: `false`.

Upgrade guidance: keep this as `false` for Terraform module version upgrades or when a legacy ABFS Spanner DB already exists.

New DB guidance: set this to `true` only when provisioning a new ABFS Spanner DB and you want module-driven table creation.

Pipeline guard: for `APPLY`, the pipeline fails if this is `true` and an ABFS database already exists.

### `ABFS_SPANNER_DATABASE_SCHEMA_VERSION`

Optional.

Purpose: choose schema version applied when table creation is enabled (`ABFS_SPANNER_DATABASE_CREATE_TABLES=true`).

Format: schema version string (for example `0.0.31`).

Default: `0.0.31`.

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
