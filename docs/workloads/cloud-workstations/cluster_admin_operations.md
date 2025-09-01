# Cluster Admin Operations

## Table of contents
- [Introduction](#introduction)
- [Prerequisites](#prerequisites)
- [Pipelines](#pipelines)
  - [Create Cluster](#pipelines-create-cluster)
  - [Delete Cluster](#pipelines-delete-cluster)
- [Environment Variables/Parameters](#environment-variables)
  - [Create Cluster](#environment-variables-create-cluster)
  - [Delete Cluster](#environment-variables-delete-cluster)
- [System Variables](#system-variables)
- [Known Issues](#known-issues)

## Introduction <a name="introduction"></a>

The Jenkins folder `Cloud Workstations > Cluster Admin Operations` provides a set of two admin-level pipelines to manage the Cloud Workstation Cluster - which is a logical grouping of Cloud Workstations resources on GCP. These pipelines use `Terraform` to manage infrastructure resources on GCP. The two pipelines are:
  - `Create Cluster`
  - `Delete Cluster`

### References
- [Terraform for Workstation Cluster](https://registry.terraform.io/providers/hashicorp/google/latest/docs/resources/workstations_workstation_cluster)

## Prerequisites<a name="prerequisites"></a>

One-time setup requirements.

- Before running this pipeline job, ensure that the following template has been created by running the corresponding job:
  - Docker image template: `Cloud Workstations > Environment > Docker Image Template`

## Pipelines<a name="pipelines"></a>

Here are details about the two pipelines for Cluster Admin Operations:

### Create Cluster<a name="pipelines-create-cluster"></a>
- This job creates a new Cluster for Cloud Workstations in your existing GCP project.
- It need only be run once when getting started with Cloud Workstations.
- At any time, only one cluster is allowed.
- And hence, all of the Cluster properties are preset.
  
### Delete Cluster<a name="pipelines-delete-cluster"></a>
- This job deletes the only existing Cloud Workstations Cluster.
- This job fails if the Cluster has child resources (e.g. Config or Workstations), and won't delete the Cluster or any of its resources. (See [Known Issues](#known-issues) section)

## Environment Variables/Parameters <a name="environment-variables"></a>

**Jenkins Parameters:** Below are the parameters defined in the groovy job definition `groovy/job.groovy` for each of the pipelines.

### Create Cluster<a name="environment-variables-create-cluster"></a>

This pipeline takes **no parameters** as its properties have been preset and are displayed on the Jenkins pipeline page.

### Delete Cluster<a name="environment-variables-delete-cluster"></a>
This pipeline takes only **one parameter**:

#### `CONFIRM_DELETE`
- REQUIRED: Check this box to confirm deletion. This action is irreversible.

## System Variables <a name="system-variables"></a>

There are a number of system environment variables that are unique to each platform but required by these Jenkins Cloud Workstation `Cluster Admin Operations` pipelines.

These are defined in Jenkins CasC `jenkins.yaml` and can be viewed in Jenkins UI under `Manage Jenkins` -> `System` -> `Global Properties` -> `Environment variables`.

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

Below variables have their values defined in `gitops/env/stage2/values.yaml` and then referenced in Jenkins CasC `jenkins.yaml`.

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
### Delete Cluster
Currently, the Terraform configuration for Cloud Workstations available as part of `google-beta` provider, does not have a `force` or `cascade` delete option, and hence if the cluster has child resources then simply running `Delete Cluster` pipeline won't delete all of its resources.
- Follow below steps in order to delete the Cloud Workstations Cluster:
    1. **Delete All Workstations**
        - Run the job `Workstation Admin Operations > List Workstations` to get the list of all existing workstations.
        - Run the job `Workstation Admin Operations > Delete Existing Workstation` for every Cloud Workstation present in the list you got in the previous step.
    2. **Delete All Configurations**
        - Run the job `Config Admin Operations > List Configurations` to get the list of all existing configurations.
        - Run the job `Config Admin Operations > Delete Existing Configuration` for every configuration present in the list you got in the previous step.
    3. **Delete Cluster** by running this `Delete Cluster` pipeline, finally.