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
- **ABFS License Deployment**: The ABFS license provided by Google must be deployed on the platform using GitHub environment secrets and Terraform workflow.
- **Docker Infra Image Template Job**:The Docker Infra Image Template job must be run, and the Docker image must be available in the registry.
- **ABFS Server**: The ABFS server must have been created and started for uploader to seed the server.

Additional details are available in `docs/workloads/android/abfs.md`.


## Environment Variables/Parameters <a name="environment-variables"></a>

**Jenkins Parameters:** Defined in the groovy job definition `groovy/job.groovy`.

### `ABFS_TERRAFORM_ACTION`

The action to perform to create, destroy, stop, start, restart server.

- `APPLY`: use this to create the instance based on the set of defined parameters.
- `DESTROY`: use this to delete the instance.
- `STOP`|`START`|`RESTART`: useful for stopping expensive instances and starting when required.

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

This is the Gerrit Android branch/tag the uploader will seed the ABFS server.

Note: you may use a comma separated list to define more than one branch, or apply new branches, simply change the name.

### `TERRAFORM_GITHUB_URL`

The URL for Google Terraform modules for ABFS.

### `TERRAFORM_GITHUB_VERSION`

The sha1 for Google Terraform modules for ABFS.

### `ABFS_LICENSE_B64`

This is a local override of the ABFS license that was created in GitHub environment secrets and stored as a kubernetes secret. It's purpose is to allow users test license updates locally, prior to updating GitHub, Terraform and ArgoCD to apply the license formally to the project.

## SYSTEM VARIABLES <a name="system-variables"></a>

There are a number of system environment variables that are unique to each platform but required by Jenkins build, test and environment pipelines.

These are defined in Jenkins CasC `jenkins.yaml` and can be viewed in Jenkins UI under `Manage Jenkins` -> `System` -> `Global Properties` -> `Environment variables`.

These are as follows:

-   `INFRA_DOCKER_ARTIFACT_PATH_NAME`
    - Defines the registry path where the Docker image used for ABFS infrastructure jobs is stored.

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
