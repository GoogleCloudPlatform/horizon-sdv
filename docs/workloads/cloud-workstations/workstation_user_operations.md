# Workstation User Operations

## Table of contents
- [Introduction](#introduction)
- [Prerequisites](#prerequisites)
- [Pipelines](#pipelines)
  - [Get Workstation Details](#pipelines-get-workstation-details)
  - [List Workstations](#pipelines-list-workstations)
  - [Get Workstation Configuration](#pipelines-get-workstation-configuration)
  - [Start Workstation](#pipelines-start-workstation)
  - [Stop Workstation](#pipelines-start-workstation)
- [Environment Variables/Parameters](#environment-variables)
  - [Get Workstation Details](#environment-variables-get-workstation-details)
  - [List Workstations](#environment-variables-list-workstations)
  - [Get Workstation Configuration](#environment-variables-get-workstation-configuration)
  - [Start Workstation](#environment-variables-start-workstation)
  - [Stop Workstation](#environment-variables-start-workstation)
- [System Variables](#system-variables)

## Introduction <a name="introduction"></a>

The Jenkins folder `Cloud Workstations > Workstation User Operations` provides a set of five user-level pipelines to manage lifecycle of Workstations on GCP. These pipelines use `Terraform` and `gcloud` to manage and fetch details of infrastructure resources on GCP. The five pipelines are:
- `Get Workstation Details`
- `List Workstations`
- `Get Workstation Configuration`
- `Start Workstation`
- `Stop Workstation`

### References
- [Terraform for Workstation](https://registry.terraform.io/providers/hashicorp/google/latest/docs/resources/workstations_workstation_workstations)

## Prerequisites<a name="prerequisites"></a>

One-time setup requirements. Before running these pipelines:
  - Ensure that the following template has been created by running the corresponding job: `Cloud Workstations > Environment > Docker Image Template`
  - Ensure the Cluster has been created by running the corresponding job: `Cloud Workstations > Cluster Admin Operations > Create Cluster`

## Pipelines<a name="pipelines"></a>

Here are details about the five pipelines for Workstation User Operations:

### Get Workstation Details<a name="pipelines-get-workstation-details"></a>
- This job fetches the full details and current status for a specific Cloud Workstation that current user has access to.
- This job fails if the specified Workstation does not exists.
  
### List Workstations<a name="pipelines-list-workstations"></a>
- This job retrieves a list of all Cloud Workstations that the current user has access to.
- This job fails if specified Workstation does not exists.

### Get Workstation Configuration<a name="pipelines-get-workstation-configuration"></a>
- This job fetches the detailed Workstation Configuration of a specific Cloud Workstation that the current user has access to.
- This job fails if the specified Workstation does not exists.

### Start Workstation<a name="pipelines-start-workstation"></a>
- This job initiates the startup process for a specified Cloud Workstation that the current user has access to, making it ready for use. Once running, it will provide the URL to access the workstation in the job's console output.
- This job fails if the specified Workstation does not exists.

### Stop Workstation<a name="pipelines-stop-workstation"></a>
- This job initiates the shutdown process for a specified Cloud Workstation, stopping its compute resources to reduce costs.
- This job fails if the specified Workstation does not exists.


## Environment Variables/Parameters <a name="environment-variables"></a>

**Jenkins Parameters:** Below are the parameters defined in the groovy job definition `groovy/job.groovy` for each of the pipelines.

### Get Workstation Details<a name="environment-variables-get-workstation-details"></a>

#### `CLOUD_WS_WORKSTATION_NAME`
- REQUIRED: The exact name of the workstation for which to retrieve details.


### List Workstations<a name="environment-variables-list-workstations"></a>

This job takes **no parameters** as it lists down all the workstations the current user has access to, along with their URLs.


### Get Workstation Configuration<a name="environment-variables-get-workstation-configuration"></a>

#### `CLOUD_WS_WORKSTATION_NAME`
- REQUIRED: The exact name of the workstation for which to retrieve workstation Configuration details.


### Start Workstation<a name="environment-variables-start-workstation"></a>

#### `CLOUD_WS_WORKSTATION_NAME`
- REQUIRED: The exact name of the workstation to start.


### Stop Workstation<a name="environment-variables-stop-workstation"></a>

#### `CLOUD_WS_WORKSTATION_NAME`
- REQUIRED: The exact name of the workstation to stop.


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
