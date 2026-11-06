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

# Mirror Pipelines

## Table of contents
- Introduction
- Prerequisites
- Pipelines
  - 1. Docker Image Template
  - 2. Create Mirror Infra
  - 3. Sync Mirror
  - 4. Delete Mirror
- System Variables

## Introduction <a name="introduction"></a>

This document describes the Jenkins pipelines for managing an NFS-based mirror in Google Cloud Platform (GCP), particularly the AOSP (Android Open Source Project) mirror. These pipelines automate the creation, synchronization, and deletion of the mirror infrastructure, which can be used to accelerate `repo sync` operations in Android builds.

## Prerequisites<a name="prerequisites"></a>

- Before running any mirror pipeline, ensure that the `Android Workflows/Environment/Mirror/Docker Image Template` pipeline has been run to build the necessary container image.
- The `Create Mirror Infra` pipeline must be run before any sync operations can be performed.

## Pipelines

### 1. Docker Image Template <a name="1-docker-image-template"></a>
**Path:** `Android Workflows/Environment/Mirror/Docker Image Template`

This pipeline builds the Docker image used as the environment for all other mirror operations.

#### `IMAGE_TAG`
Tag for the Docker image to build.

#### `BUILDKIT_RELEASE_TAG`
Buildkit version to use for building the image.

#### `NO_PUSH`
If true, the image will be built but not pushed to the artifact registry.

### 2. Create Mirror Infra <a name="2-create-mirror-infrastructure"></a>
**Path:** `Android Workflows/Environment/Mirror/Create Mirror Infra`

This pipeline automates the provisioning of high-performance NFS storage for Git mirrors. It dynamically allocates a Google Cloud Filestore instance and configures the necessary Kubernetes Persistent Volume (PV) and Claim (PVC) to make the storage available to the cluster.

#### `IMAGE_TAG`
The tag of the Docker image to use for the build environment. Defaults to `latest`.
`
#### `MIRROR_VOLUME_CAPACITY_GB`
The size of the Filestore volume in GiB. The minimum is 1024. Size can only be increased, in multiples of 256GB, but not decreased. Tip: A full AOSP Mirror consumes around 1946Gi (1.9Ti) of storage. So 2048Gi of total volume capacity is recommended.

### 3. Sync Mirror <a name="3-sync-mirror"></a>
**Path:** `Android Workflows/Environment/Mirror/Sync Mirror`

This pipeline downloads a new mirror or syncs an existing mirror in specified directory on the NFS volume, with specified remote manifest repository via parameters.

#### `IMAGE_TAG`
The tag of the Docker image to use for the build environment. Defaults to `latest`.

#### `SYNC_ALL_EXISTING_MIRRORS`
If true, all existing mirrors will be synced, and other mirror-specific parameters will be ignored. Ideal for scheduled periodic updates of all managed mirrors.

#### `MIRROR_DIR`
The directory name for the mirror. Required if not syncing all existing mirrors. Note: If you provide 'my-mirror' as value, the absolute container path of the mirror will be '/mnt/mirror-filestore-pvc/horizon-mirrors/my-mirror', where 'horizon-mirrors.' is the root subdirectory for all mirrors.

#### `MIRROR_MANIFEST_URL`
The Git URL for the manifest repository. Note: Once set for a mirror, this value cannot be changed without recreating the mirror.

#### `MIRROR_MANIFEST_REF`
The Git branch or tag for the manifest. Note: This value can be updated in subsequent syncs to point to a different branch or tag.

#### `MIRROR_MANIFEST_FILE`
The manifest XML file name within the manifest repository. Note: This value can be updated in subsequent syncs to point to a different manifest file.

#### `REPO_SYNC_JOBS`
The number of parallel jobs to use for `repo sync`. Note: Default value is defined by the Android Seed job.
Max recommended value for AOSP mirror is 3 due to rate-limiting constraints set by Google.

### 4. Delete Mirror <a name="4-delete-mirror"></a>
**Path:** `Android Workflows/Environment/Mirror/Delete Mirror`

This pipeline deletes individual mirrors or the entire mirror infrastructure from GCP.

#### `IMAGE_TAG`
The tag of the Docker image to use for the build environment. Defaults to `latest`.

#### `CONFIRM_DELETE`
A safety measure; must be set to `true` to proceed with any deletion.

#### `MIRROR_DIR_TO_DELETE`
The specific mirror directory to delete. This is required if `DELETE_ENTIRE_MIRROR_SETUP` is false.

#### `DELETE_ENTIRE_MIRROR_SETUP`
If true, the entire mirror infrastructure (Filestore, PV, PVC) will be destroyed. Note: When set to true, the MIRROR_DIR_TO_DELETE parameter is ignored.

## System Variables <a name="system-variables"></a>

There are a number of system environment variables that are unique to each platform but required by Jenkins pipelines.

These are defined in Jenkins CasC `values-jenkins.yaml` and can be viewed in Jenkins UI under `Manage Jenkins` -> `System` -> `Global Properties` -> `Environment variables`.

These are as follows:

-   `CLOUD_PROJECT`
    - The GCP project ID.

-   `CLOUD_REGION`
    - The GCP project region.

-   `CLOUD_ZONE`
    - The GCP project zone.

-   `CLOUD_BACKEND_BUCKET`
    - The GCS bucket used for the Terraform state backend.

-   `TERRAFORM_WORKLOADS_SERVICE_ACCOUNT`
    - The Kubernetes service account used for GCP operations.

-   `MIRROR_PRESET_FILESTORE_STORAGE_CLASS_NAME`
    - The preset name for the Kubernetes StorageClass for the Filestore PV.

-   `MIRROR_PRESET_FILESTORE_PVC_NAME`
    - The preset name for the Persistent Volume Claim for the Filestore.

-   `MIRROR_PRESET_FILESTORE_PVC_MOUNT_PATH_IN_CONTAINER`
    - The mount path for the PVC inside the build containers.

-   `MIRROR_PRESET_MIRROR_ROOT_SUBDIR_NAME`
    - The root subdirectory name where mirrors are stored on the volume.

-   `MIRROR_PRESET_NETWORK_NAME`
    - The preset name of the GCP VPC network.

-   `MIRROR_PRESET_SUBNETWORK_NAME`
    - The preset name of the GCP subnetwork.

-   `MIRROR_WORKLOADS_ENV_IMAGE_NAME`
    - The name of the Docker image repository in the Artifact Registry.

-   `BUILDKIT_RELEASE_TAG` (Seed job parameter, not a CasC system variable)
    - The version of Buildkit to use to build the container image.

-   `DOCKER_CREDENTIALS_URL` (Seed job parameter, not a CasC system variable)
    - URL of Google docker credentials helper, required to allow access to the project artifact registry.
    