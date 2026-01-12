# Development Test Instance

## Table of contents
- [Introduction](#introduction)
- [Prerequisites](#prerequisites)
- [Environment Variables/Parameters](#environment-variables)
- [System Variables](#system-variables)

## Introduction <a name="introduction"></a>

During developing the Android workload and workflow/pipelines, sometimes it may be necessary to gain access to a VM test instance in order to develop test jobs.

User may access via MTK Connect HOST from the respective testbench. Alternatively access via Google Cloud Platform
Compute Engine VM instances.

- These instances only remain active for a limited time, defined by `INSTANCE_MAX_UPTIME`.
- Users are responsible for managing their work and saving to their own storage, that's beyond the purpose of this job.

## Prerequisites<a name="prerequisites"></a>

One-time setup requirements.

- Before running this pipeline job, ensure that the following template has been created by running the corresponding job:
  - Cuttlefish Instance template: `Android Workflows/Environment/CF Instance Template`
  - VM instances used for test must exist.

## Environment Variables/Parameters <a name="environment-variables"></a>

**Jenkins Parameters:** Defined in the groovy job definition `groovy/job.groovy`.

### `JENKINS_GCE_CLOUD_LABEL`

This is the label that identifies the GCE Cloud label which will be used to identify the VM instance, e.g.

- `cuttlefish-vm-main`
- `cuttlefish-vm-v1180`

Note: The value provided must correspond to a cloud instance or the job will hang.

### `MTK_CONNECT_PUBLIC`

When checked, the MTK Connect testbench is visible to everyone and can be shared.
By default, testbenches are private and only visible to their creator and MTK Connect administrators.

### `INSTANCE_MAX_UPTIME`

This is the maximum time that the instance may be running before it is automatically terminated and deleted. This is important to avoid leaving expensive instances in running state.

### `NUM_HOST_INSTANCES`

Number of host instances to create for test instances. This is effectively the number of devices that will be created associated with the development instance testbench in MTK Connect.

## SYSTEM VARIABLES <a name="system-variables"></a>

There are a number of system environment variables that are unique to each platform but required by Jenkins build, test and environment pipelines.

These are defined in Jenkins CasC `values-jenkins.yaml` and can be viewed in Jenkins UI under `Manage Jenkins` -> `System` -> `Global Properties` -> `Environment variables`.

These are as follows:

-   `HORIZON_DOMAIN`
    - The URL domain which is required by pipeline jobs to derive URL for tools and GCP.

-   `HORIZON_GITHUB_URL`
    - The URL to the Horizon SDV GitHub repository.

-   `HORIZON_GITHUB_BRANCH`
    - The branch name the job will be configured for from `HORIZON_GITHUB_URL`.

