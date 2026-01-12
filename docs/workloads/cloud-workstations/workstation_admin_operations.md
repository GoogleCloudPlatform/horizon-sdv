# Workstation Admin Operations

## Table of contents
- [Introduction](#introduction)
- [Prerequisites](#prerequisites)
- [Pipelines](#pipelines)
  - [Create New Workstation](#pipelines-create-new-workstation)
  - [Delete Existing Workstation](#pipelines-delete-existing-workstation)
  - [List Workstations](#pipelines-list-workstations)
  - [Get Workstation Details](#pipelines-get-workstation-details)
  - [Add Users to Workstation](#pipelines-add-users-to-workstation)
  - [Remove Workstation Users](#pipelines-remove-workstation-users)
- [Environment Variables/Parameters](#environment-variables)
  - [Create New Workstation](#environment-variables-create-new-workstation)
  - [Delete Existing Workstation](#environment-variables-delete-existing-workstation)
  - [List Workstations](#environment-variables-list-workstations)
  - [Get Workstation Details](#environment-variables-get-workstation-details)
  - [Add Users to Workstation](#environment-variables-add-users-to-workstation)
  - [Remove Workstation Users](#environment-variables-remove-workstation-users)
- [System Variables](#system-variables)

## Introduction <a name="introduction"></a>

The Jenkins folder `Cloud Workstations > Workstation Admin Operations` provides a set of six admin-level pipelines to manage the Cloud Workstations on GCP. These pipelines use `Terraform` and `gcloud` to manage and fetch details of infrastructure resources on GCP. The six pipelines are:
- `Create New Workstation`
- `Delete Existing Workstation`
- `List Workstations`
- `Get Workstation Details`
- `Add Users to Workstation`
- `Remove Workstation Users`

### References
- [Terraform for Workstation](https://registry.terraform.io/providers/hashicorp/google/latest/docs/resources/workstations_workstation_workstations)

## Prerequisites<a name="prerequisites"></a>

One-time setup requirements. Before running these pipelines:
  - Ensure that the following template has been created by running the corresponding job: `Cloud Workstations > Environment > Docker Image Template`
  - Ensure the Cluster has been created by running the corresponding job: `Cloud Workstations > Cluster Admin Operations > Create Cluster`

Also ensure that the configuration you are going to specify for a workstation, exists.

## Pipelines<a name="pipelines"></a>

Here are details about the six pipelines for Workstation Admin Operations:

### Create New Workstation<a name="pipelines-create-new-workstation"></a>
- This job creates a new Cloud Workstation instance based on a specified configuration.
- This job fails if:
  - A workstation with same name already exists.
  - The specified Config does not exists.
  
### Delete Existing Workstation<a name="pipelines-delete-existing-workstation"></a>
- This job deletes the specified Workstation.
- This job fails if specified Workstation does not exists.

### List Workstations<a name="pipelines-list-workstations"></a>
- This job displays a list of all existing Workstations.
- This job fails if the specified Workstation does not exists.

### Get Workstation Details<a name="pipelines-get-workstation-details"></a>
- This job displays the full details of a specific Cloud Workstation.
- This job fails if the specified Workstation does not exists.

### Add Users to Workstation<a name="pipelines-add-users-to-workstation"></a>
- This job adds one or more specified users to an existing Cloud Workstation, granting them access as `Workstation User`.
- This job fails if the specified Workstation does not exists.

### Remove Workstation Users<a name="pipelines-remove-workstation-users"></a>
- This job removes access for one or more specified users from an existing Cloud Workstation.
- This job fails if the specified Workstation does not exists.


## Environment Variables/Parameters <a name="environment-variables"></a>

**Jenkins Parameters:** Below are the parameters defined in the groovy job definition `groovy/job.groovy` for each of the pipelines.

### Create New Workstation<a name="environment-variables-create-new-workstation"></a>

#### `WORKSTATION_NAME`
- REQUIRED: A unique name for the new workstation instance (e.g., "`my-android-dev-ws`").

#### `WORKSTATION_DISPLAY_NAME`
- Optional: A user-friendly display name for the workstation. Leave empty for no display name.

#### `WORKSTATION_CONFIG_NAME`
- REQUIRED: The name of the workstation configuration blueprint to use (e.g., "`android-dev-config`").

#### `INITIAL_WORKSTATION_USER_EMAILS_TO_ADD`
- OPTIONAL: Comma-separated list of user emails (e.g., `user1@example.com,user2@example.com`) to grant access to upon creation. These users will get the "`workstations.user`" role.


### Delete Existing Workstation<a name="environment-variables-delete-existing-workstation"></a>

#### `WORKSTATION_NAME`
- REQUIRED: The exact name of the workstation to delete.

#### `CONFIRM_DELETE`
- REQUIRED: Check this box to confirm deletion. This action is irreversible.


### List Workstations<a name="environment-variables-list-workstations"></a>

#### `WORKSTATION_NAME_PATTERN`
- Optional: Filter by workstation name (regex, e.g., "`dev-.*`" or "`ws[0-9]+`").

#### `WORKSTATION_USER_EMAIL_PATTERN`
- Optional: Filter by user email (regex, e.g., "`.*@example\.com`" or "`joe|jane`").

#### `WORKSTATION_CONFIG_NAME_PATTERN`
- Optional: Filter by config name (regex, e.g., "`config-.*`" or "`code-.*`").


### Get Workstation Details<a name="environment-variables-get-workstation-details"></a>

#### `WORKSTATION_NAME`
- REQUIRED: The exact name of the workstation to retrieve details for.


### Add Users to Workstation<a name="environment-variables-add-users-to-workstation"></a>

#### `WORKSTATION_NAME`
- REQUIRED: The exact name of the workstation to add users to.

#### `WORKSTATION_USER_EMAILS_TO_ADD`
- REQUIRED: Comma-separated list of user emails (e.g., `user1@example.com,user2@example.com`) to grant access.


### Remove Workstation Users<a name="environment-variables-remove-workstation-users"></a>

#### `WORKSTATION_NAME`
- REQUIRED: The exact name of the workstation to remove users from.

#### `WORKSTATION_USER_EMAILS_TO_REMOVE`
- REQUIRED: Comma-separated list of user emails (e.g., `user1@example.com,user2@example.com`) to revoke access for.


## System Variables <a name="system-variables"></a>

There are a number of system environment variables that are unique to each platform but required by these Jenkins Cloud Workstation `Workstation Admin Operations` pipelines.

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
