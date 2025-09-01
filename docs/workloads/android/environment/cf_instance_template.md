# Cuttlefish Instance Template Pipeline

## Table of contents
- [Introduction](#introduction)
- [Prerequisites](#prerequisites)
- [Environment Variables/Parameters](#environment-variables)
- [Example Usage](#examples)
- [System Variables](#system-variables)

## Introduction <a name="introduction"></a>

This pipeline creates (or deletes) Cuttlefish instance templates which are used by the Jenkins test pipelines to spin up cloud instances which are cuttlefish-ready and CTS-ready; these cloud instances are then used to launch CVD and run CTS tests.

During the process of creating an instance template, this pipeline also creates a custom image which is referenced by the created instance template. This image is created using the same naming convention as the instance template.

For example:

- <b>Name (provided or auto-generated)</b>: cuttlefish-vm-main
- <b>Image Name</b>: image-cuttlefish-vm-main
- <b>Instance Template Name</b>: instance-template-cuttlefish-vm-main

The following gcloud commands can be used to view images and instance templates:

- gcloud compute instance-templates list | grep cuttlefish-vm
- gcloud compute instances list | grep cuttlefish-vm

<b>Important:</b> This pipeline may not be run concurrently - this is to avoid clashes with temporary artifacts the job creates in order to produce the Cuttlefish instance template.

### References <a name="references"></a>

- [Cuttlefish Virtual Devices](https://source.android.com/docs/devices/cuttlefish) for use with [CTS](https://source.android.com/docs/compatibility/cts) and emulators.
- [Virtual Device for Android host-side utilities](https://github.com/google/android-cuttlefish)
- [Compatibility Test Suite downloads](https://source.android.com/docs/compatibility/cts/downloads)
- [Compute Instance Templates](https://cloud.google.com/sdk/gcloud/reference/compute/instance-templates/create)

## Prerequisites<a name="prerequisites"></a>

One-time setup requirements.

- Before running this pipeline job, ensure that the following template has been created by running the corresponding job:
  - Docker image template: ``Android Workflows/Environment/Docker Image Template`
- The Google Compute Engine is configured with `noDelayProvisioning: false` in `gitops/env/stage2/templates/jenkins.yaml` to help reduce costs. With this setting, multiple VM instances are not started immediately, which lowers expenses for each run. However, disabling immediate provisioning may slightly increase VM startup times. This trade-off allows users to choose between faster VM availability and lower operational costs.

## Environment Variables/Parameters <a name="environment-variables"></a>

**Jenkins Parameters:** Defined in the groovy job definition `groovy/job.groovy`.

### `ANDROID_CUTTLEFISH_REVISION`

This defines the version of [Android Cuttlefish](https://github.com/google/android-cuttlefish.git) host packages to use, e.g.

- `main` - the main working branch of `android-cuttlefish`
- `v1.1.0` - the latest tagged version.

User may define any valid version so long as that version contains `tools/buildutils/build_packages.sh` which is a dependency for these scripts.

### `CUTTLEFISH_INSTANCE_UNIQUE_NAME`
**Note:** Name must be a match of regex `(?:[a-z](?:[-a-z0-9]{0,61}[a-z0-9])?)`, i.e lower case.

Optional parameter to allow users to create their own unique instance templates for use in development and/or testing.

If left empty, the name will be derived from `ANDROID_CUTTLEFISH_REVISION` e.g. `cuttlefish-vm-main` and create
an instance template `instance-template-cuttlefish-vm-main` and an image `image-cuttlefish-vm-main`.

If user defines a unique name, ensure the following is met:

- The name should start with `cuttlefish-vm`
- Jenkins CasC (`jenkins.yaml`) must be updated to provide a new `computeEngine` entry for this unique template. For reference, see existing entry for `cuttlefish-vm-main`.
  - Choose a sensible `cloudName`, such as `cuttlefish-vm-unique-name` (e.g. the same name as the instance template with the "instance-template" prefix removed).
  - Once synced, this new cloud will appear in `Manage Jenkins` -> `Clouds`
  - Tests jobs may then reference that unique instance by setting the `JENKINS_GCE_CLOUD_LABEL` parameter to the new cloud label (`cloudName`).


### `MACHINE_TYPE`

The machine type to be used for the VM instance, default is `n1-standard-64`.

### `BOOT_DISK_SIZE`

A boot disk is required to create the instance, therefore define the size of disk required.

### `MAX_RUN_DURATION`

VM instances are expensive so it is advisable to define the maximum amount of time to run the instance before it will automatically be terminated. This avoids leaving expensive instances in running state and consuming resources.

User may disable by setting the value to 0, but they must be aware of any costs that they may incur to their project.  Setting to 0 is useful when creating development test instances so users can connect directly to the VM instance.

### `DEBIAN_OS_VERSION`

Override the OS version. These regularly become deprecated and superceded, hence option to update to newer version.

Keep an eye out in the console logs for `deprecated` and update as required.

### `NODEJS_VERSION`

MTK Connect requires NodeJS; this option allows you to update the version to install on the instance template.

### `DELETE`

Allows deletion of an existing instance templates and its referenced image.

If deleting a standard instance template (i.e. name auto-generated), simply define the version in `ANDROID_CUTTLEFISH_REVISION` and the required names will be derived automatically.

- `ANDROID_CUTTLEFISH_REVISION`: choose the version you wish to delete
- `DELETE`: This ensures the instance template, disk image and VM instance are deleted.
- `Build` : trigger build to delete all artifacts.

If user is deleting a uniquely-created instance template (i.e. name specified by `CUTTLEFISH_INSTANCE_UNIQUE_NAME`), then define `CUTTLEFISH_INSTANCE_UNIQUE_NAME` as was used to create it (i.e. the same name as the instance template with the "instance-template" prefix removed).

- `CUTTLEFISH_INSTANCE_UNIQUE_NAME`: choose the template unique name you wish to delete
- `DELETE`: This ensures the instance template, disk image and VM instance are deleted.
- `Build` : trigger build to delete all artifacts.

### `VM_INSTANCE_CREATE`

**Enable Stopped VM Instance Creation**

If enabled, this job will create a Cuttlefish VM instance from the final instance template. It will be placed in stop
state after creation. This is provided for development testing and debugging.

This would allow developers to:
- Start the instance via the bastion host
- Connect to the instance directly
- Run tests on the instance manually, bypassing Jenkins

**Important:**
- Be aware that creating this instance may incur additional costs for your project.
- Enable this only for instance templates created for developement purposes that are created with a well defined `CUTTLEFISH_INSTANCE_UNIQUE_NAME`.
- Set `MAX_RUN_DURATION` to 0 to ensure VM instance is never deleted on runtime expiry.
- It is advisable to `DELETE` these development instances when testing is completed.

## Example Usage <a name="examples"></a>

If user wishes to create a temporary test instance to work with, then they can do so as follows from Jenkins:

- `ANDROID_CUTTLEFISH_REVISION`: choose the version you wish to build the template from
- `CUTTLEFISH_INSTANCE_UNIQUE_NAME` : provide a unique name, starting with cuttlefish-vm, e.g. `cuttlefish-vm-test-instance-v110.`
- `MAX_RUN_DURATION` : set to 0 to avoid instance being deleted after this time.
- `VM_INSTANCE_CREATE` : Enable this option so that the instance template will create a VM instance for user to start, connect to and work with.
- `Build`

Once they have finished with the instances, they should delete to avoid excessive costs.

- `CUTTLEFISH_INSTANCE_UNIQUE_NAME` : provide a unique name, starting with cuttlefish-vm, e.g. `cuttlefish-vm-test-instance-v110.`
- `DELETE` : This ensures the instance template, disk image and VM instance are deleted.
- `Build`

## SYSTEM VARIABLES <a name="system-variables"></a>

There are a number of system environment variables that are unique to each platform but required by Jenkins build, test and environment pipelines.

These are defined in Jenkins CasC `jenkins.yaml` and can be viewed in Jenkins UI under `Manage Jenkins` -> `System` -> `Global Properties` -> `Environment variables`.

These are as follows:

-   `ANDROID_BUILD_DOCKER_ARTIFACT_PATH_NAME`
    - Defines the registry path where the Docker image used by builds, tests and environments is stored.

-   `CLOUD_PROJECT`
    - The GCP project, unique to each project. Important for bucket, registry paths used in pipelines.

-   `CLOUD_REGION`
    - The GCP project region. Important for bucket, registry paths used in pipelines.

-   `CLOUD_ZONE`
    - The GCP project zone. Important for bucket, registry paths used in pipelines.

-   `HORIZON_DOMAIN`
    - The URL domain which is required by pipeline jobs to derive URL for tools and GCP.

-   `HORIZON_GITHUB_URL`
    - The URL to the Horizon SDV GitHub repository.

-   `HORIZON_GITHUB_BRANCH`
    - The branch name the job will be configured for from `HORIZON_GITHUB_URL`.

-   `JENKINS_SERVICE_ACCOUNT`
    - Service account to use for pipelines. Required to ensure correct roles and permissions for GCP resources.
