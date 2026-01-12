# Config Admin Operations

## Table of contents
- [Introduction](#introduction)
- [Prerequisites](#prerequisites)
- [Pipelines](#pipelines)
  - [Create New Configuration](#pipelines-create-new-configuration)
  - [Delete Existing Configuration](#pipelines-delete-existing-configuration)
  - [Update Existing Configuration](#pipelines-update-existing-configuration)
  - [List Configurations](#pipelines-list-configurations)
  - [Get Configuration Details](#pipelines-get-configuration-details)
  - [List Workstations by Configuration](#pipelines-list-workstations-by-configuration)
- [Environment Variables/Parameters](#environment-variables)
  - [Create New Configuration](#environment-variables-create-new-configuration)
  - [Delete Existing Configuration](#environment-variables-delete-existing-configuration)
  - [Update Existing Configuration](#environment-variables-update-existing-configuration)
  - [List Configurations](#environment-variables-list-configurations)
  - [Get Configuration Details](#environment-variables-get-configuration-details)
  - [List Workstations by Configuration](#environment-variables-list-workstations-by-configuration)
- [System Variables](#system-variables)
- [Known Issues](#known-issues)

## Introduction <a name="introduction"></a>

The Jenkins folder `Cloud Workstations > Config Admin Operations` provides a set of six admin-level pipelines to manage the Cloud Workstation Configurations - which are like blueprints used by Workstations on GCP. These pipelines use `Terraform` to manage infrastructure resources on GCP. The six pipelines are:
- `Create New Configuration`
- `Delete Existing Configuration`
- `Update Existing Configuration`
- `List Configurations`
- `Get Configuration Details`
- `List Workstations by Configuration`

### References
- [Terraform for Workstation Configuration](https://registry.terraform.io/providers/hashicorp/google/latest/docs/resources/workstations_workstation_config)

## Prerequisites<a name="prerequisites"></a>

One-time setup requirements. Before running these pipelines:
  - Ensure that the following template has been created by running the corresponding job: `Cloud Workstations > Environment > Docker Image Template`
  - Ensure the Cluster has been created by running the corresponding job: `Cloud Workstations > Cluster Admin Operations > Create Cluster`

## Pipelines<a name="pipelines"></a>

Here are details about the six pipelines for Config Admin Operations:

### Create New Configuration<a name="pipelines-create-new-configuration"></a>
- This job creates a new Configuration for Cloud Workstations in your existing GCP project.
- This job fails if:
  - A configuration with same name already exists.
  - The Workstation Cluster does not exists. (Run `Create Cluster` operation pipeline in that case)
  
### Delete Existing Configuration<a name="pipelines-delete-existing-configuration"></a>
- This job deletes the specified Configuration.
- This job fails if:
  - Specified Configuration does not exists.
  - If the Configuration has child resources (i.e. Workstations), and won't delete the Configuration or any of its resources. (See [Known Issues](#known-issues) section)

### Update Existing Configuration<a name="pipelines-update-existing-configuration"></a>
- This job updates the specified Configuration.
- This job fails if the specified Configuration does not exists.
- **Important Notes**: 
  - This pipeline does NOT retain your original Config properties, i.e. whatever you set here, IS what the config will be. (Like create operation, but for an existing config)
  - You should leave a parameter empty, only if its "Optional" or has a "Default" value - and NOT because you think it will retain its original value - it will NOT.

### List Configurations<a name="pipelines-list-configurations"></a>
- This job displays a list of all existing Configurations of Cloud Workstations.
- This job fails if the specified Configuration does not exists.

### Get Configuration Details<a name="pipelines-get-configuration-details"></a>
- This job displays the full details of a specific Cloud Workstation Configuration.
- This job fails if the specified Configuration does not exists.

### List Workstations by Configuration<a name="pipelines-list-workstations-by-configuration"></a>
- This job displays a list of active Cloud Workstations that were created using a specific Configuration.
- This job fails if the specified Configuration does not exists.


## Environment Variables/Parameters <a name="environment-variables"></a>

**Jenkins Parameters:** Below are the parameters defined in the groovy job definition `groovy/job.groovy` for each of the pipelines.

### Create New Configuration<a name="environment-variables-create-new-configuration"></a>

#### `CLOUD_WS_CONFIG_NAME`
- REQUIRED: Unique Name for the new workstation config.

#### `WS_IDLE_TIMEOUT`
- Optional: Idle Timeout in seconds.
- Default: 1200 = 20 mins

#### `WS_RUNNING_TIMEOUT`
- Optional: Running Timeout in seconds.
- Default: 43200 = 12 hrs

#### `WS_REPLICA_ZONES`
- Optional: Comma-separated list of two zones in the europe-west1 region.
- Default: `europe-west1-b, europe-west1-d`
- Note:
  - EXACTLY TWO zones required, separated by a comma.
  - NO enclosing brackets or quotes allowed.

#### `HOST_MACHINE_TYPE`
- Optional: GCP Compute Engine Machine type for the host VM.
- Default: `e2-standard-4`

#### `HOST_QUICK_START_POOL_SIZE`
- Optional: Pool size of pre-created host VMs (0 means none and low cost).
- Default: 0

#### `HOST_BOOT_DISK_SIZE`
- Optional: Boot disk size (GB) for host VM (min: 30GB).
- Default: 30 (GB)

#### `HOST_DISABLE_PUBLIC_IP_ADDRESSES`
- Optional: If selected, your workstation will NOT have a public IP.
- Note: Enabling public IP addresses might be restricted in certain GCP projects by admin.

#### `HOST_DISABLE_SSH`
- Optional: If selected, your workstation will NOT have SSH enabled.
- Note: Enabling SSH connections might be restricted in certain GCP projects by admin.

#### `HOST_ENABLE_NESTED_VIRTUALIZATION`
- Optional: If selected, your workstation VMs will have nested virtualization enabled - which is generally needed for running Android emulators.
- Note:
  - Nested virtualization can ONLY be enabled on configurations that specify a HOST_MACHINE_TYPE in the N1 or N2 machine series.
  - This feature might be restricted in certain GCP projects by admin.

#### `PD_REQUIRED`
- Optional: If selected, your workstations using this config will include a mounted persistent disk (PD) and its details can be filled below.

#### `PD_MOUNT_PATH`
- Optional: Mount path for persistent disk.
- Default (if `PD_REQUIRED` selected): `/home`

#### `PD_FS_TYPE`
- Optional: Filesystem type (e.g., `ext4`, `xfs`).
- Default (if `PD_REQUIRED` selected): `ext4`

#### `PD_DISK_TYPE`
- Persistent Disk type.
- Default (if `PD_REQUIRED` selected): `pd-balanced`
- All Options: 
    - `pd-balanced`
    - `pd-ssd`
    - `pd-standard`
    - `pd-extreme`
- Note: If PD size is less than 200 GB, disk type must be `pd-balanced` or `pd-ssd`.

#### `PD_SIZE_GB`
- Disk size in GB.
- Default (if `PD_REQUIRED` selected): 10 (GB)
- All Options: 10, 50, 100, 200, 500, 1000

#### `PD_RECLAIM_POLICY`
- Disk Reclaim policy.
- Default (if `PD_REQUIRED` selected): `DELETE`
- All Options: `DELETE`, `RETAIN`

#### `PD_SOURCE_SNAPSHOT`
- Optional: Source snapshot name
- Note:
  - Do NOT prefix with full path, just provide name.
  - If `PD_SOURCE_SNAPSHOT` is set then `PD_FS_TYPE` or `PD_SIZE_GB` - CANNOT be specified.

#### `ED_REQUIRED`
- Optional: If selected, your workstations will include a temporary ephemeral disk (ED) and its details must be filled below.
- Note:
  - Only either of `ED_SOURCE_SNAPSHOT` or `ED_SOURCE_IMAGE` must be specified, but NOT together.
  - If `ED_SOURCE_SNAPSHOT` is set then `ED_READ_ONLY` must be selected and vice-versa.

#### `ED_MOUNT_PATH`
- Optional: Mount path for ephemeral disk.
- Default (if `ED_REQUIRED` selected): `/tmp`

#### `ED_DISK_TYPE`
- Temporary Persistent Disk type.
- Default (if `ED_REQUIRED` selected): `pd-standard`
- All Options: 
    - `pd-balanced`
    - `pd-ssd`
    - `pd-standard`
    - `pd-extreme`

#### `ED_SOURCE_SNAPSHOT`
- REQUIRED (if `ED_REQUIRED` selected): Source snapshot for ephemeral disk.
- Note: CANNOT be set together with `ED_SOURCE_IMAGE`

#### `ED_SOURCE_IMAGE`
- REQUIRED (if `ED_REQUIRED` selected): Source image for ephemeral disk.
- Note: CANNOT be set together with `ED_SOURCE_SNAPSHOT`

#### `ED_READ_ONLY`
- Optional: If selected, ephemeral disk will be mounted as read-only.
- CANNOT be UN-selected if `ED_SOURCE_SNAPSHOT` is set

#### `CONTAINER_IMAGE`
- Optional: Container image URI.
- Default: Full URI of the `horizon-code-oss` image.

#### `CONTAINER_ENTRYPOINT_COMMANDS`
- Optional: Comma separated list of Entrypoint commands for the container.
- Example: `"sh", "-c", "echo", "ls -al"`

#### `CONTAINER_ENTRYPOINT_ARGS`
- Optional: Comma separated list of Command arguments for the container.
- Example: `arg1, arg2`

#### `CONTAINER_WORKING_DIR`
- Optional: Working directory inside container.

#### `CONTAINER_ENV_VARS`
- Optional: JSON string objects for container env vars.
- Example: `{"ENV1":"val1", "ENV2":"val2"}`

#### `CONTAINER_USER`
- Optional: User to run container as.

#### `WS_ALLOWED_PORTS`
- Optional: List of port JSON objects, enclosed in square brackets '`[ ]`'
- Default: `[{"first":80, "last":80}, {"first":1024, "last":65535}]`
Note: The strings "`first`" and "`last`" are keys that must be specified AS IT IS.

#### `WS_ADMIN_IAM_MEMBERS`
- REQUIRED: Comma-separated list of new user emails to GRANT `"Workstation Admin`" privileges.
- Example: `user1@example.com, user2@example.com`

  
### Delete Existing Configuration<a name="environment-variables-delete-existing-configuration"></a>

#### `CLOUD_WS_CONFIG_NAME`
- REQUIRED: Name of the workstation Configuration to delete.

#### `CONFIRM_DELETE`
- REQUIRED: Check this box to confirm deletion. (Warning: This is irreversible and will delete associated workstations as well).

### Update Existing Configuration<a name="environment-variables-update-existing-configuration"></a>

#### `WS_ADMIN_IAM_MEMBERS`
- REQUIRED: Comma-separated list of new user emails to GRANT "Workstation Admin" privileges.
- [CAUTION]: Strict Allow list required - Existing WS Admins, if not specified, will have their access REVOKED for all workstations.
- Example: `user1@example.com, user2@example.com`

#### Immutable Properties
  - `CLOUD_WS_CONFIG_NAME`: Name of the workstation Config.
  - `WS_REPLICA_ZONES`: List of Replica Zones for workstations created using a Config.

#### Other Parameters
- Rest all parameters are mutable and are exactly the same as `Create New Configurations` pipeline job.

#### Update Effect
- The time of effect for update operation is different for each parameter and is mentioned below:
  - Machine Timeouts: Effective immediately on all workstations
  - Host Configuration: Effective on new and restarted workstations
  - Persistent Disk Configuration: Effective only on new workstations
  - Ephemeral Disk Configuration: Effective on new and restarted workstations
  - Container Configuration: Effective on new and restarted workstations
  - Allowed Ports: Effective on new and restarted workstations
  - Workstation Admin IAM Configuration: Effective immediately on all workstations


### List Configurations<a name="environment-variables-list-configurations"></a>

#### `CLOUD_WS_CONFIG_NAME_REGEX_PATTERN`
- Optional: Enter a valid regex pattern to filter configuration names.
- Leave empty to get list of ALL cloud workstation Configs.
- Example: `^my-config-.*$`


### Get Configuration Details<a name="environment-variables-get-configuration-details"></a>
#### `CLOUD_WS_CONFIG_NAME`
- REQUIRED: Enter the exact name of the workstation Configuration to retrieve details for.

### List Workstations by Configuration<a name="environment-variables-list-workstations-by-configuration"></a>

#### `CLOUD_WS_CONFIG_NAME`
- REQUIRED: Enter the exact name of the workstation configuration to list its associated workstations.


## System Variables <a name="system-variables"></a>

There are a number of system environment variables that are unique to each platform but required by these Jenkins Cloud Workstation `Config Admin Operations` pipelines.

These are defined in Jenkins CasC `values-jenkins.yaml` and can be viewed in Jenkins UI under `Manage Jenkins` -> `System` -> `Global Properties` -> `Environment variables`.

These are as follows:
-   `CLOUD_BACKEND_BUCKET`
    - Name of the bucket that stores Terraform state for platform and cloud workstation resources (in separate folders).

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

Below variables have their values defined in `gitops/values.yaml` and then referenced in Jenkins CasC `values-jenkins.yaml`.

-   `CLOUD_WS_WORKLOADS_ENV_IMAGE_NAME`
    - Name of the Docker image on GCP Artifact registry, that is used as an environment for Cloud Workstations workload pipelines.
 
-   `CLOUD_WS_CLUSTER_PRESET_NAME`
    - Name of the Cloud Workstations Cluster. There can only be one cluster for cloud workstations and hence this is preset.
 
-   `CLOUD_WS_CLUSTER_PRESET_NETWORK_NAME`
    - Name of the network where Cloud Workstations resources are created on GCP.
 
-   `CLOUD_WS_CLUSTER_PRESET_SUBNETWORK_NAME`
    - Name of the subnetwork where Cloud Workstations resources are created on GCP.
 
-   `CLOUD_WS_CLUSTER_PRESET_PRIVATE_CLUSTER`
    - A preset property for Cloud Workstations Cluster.

## Known Issues
### Delete Existing Configuration
Currently, the Terraform configuration for Cloud Workstations available as part of `google-beta` provider, does not have a `force` or `cascade` delete option, and hence if the configuration has child resources (workstations) then simply running `Delete Existing Configuration` pipeline won't delete all of its resources.
- Follow below steps in order to delete a Config with existing workstations:
    1. **Delete Child Workstations**
        - Run the job `Config Admin Operations > Get Workstations by Configuration` to get the list of all existing workstations for the specified config.
        - Run the job `Workstation Admin Operations > Delete Existing Workstation` for every Cloud Workstation present in the list you got in the previous step.
    2. **Delete Configurations**
        - Run the job `Config Admin Operations > Delete Existing Configuration` for the configuration you want to delete, finally.
