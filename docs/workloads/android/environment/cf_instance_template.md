# Cuttlefish Instance Template Pipeline

## Table of contents
- [Introduction](#introduction)
- [Prerequisites](#prerequisites)
- [Environment Variables/Parameters](#environment-variables)
- [Example Usage](#examples)
- [System Variables](#system-variables)

## Introduction <a name="introduction"></a>

This pipeline creates (or deletes) ARM64 and x86_64 Cuttlefish instance templates which are used by the Jenkins test pipelines to spin up cloud instances which are cuttlefish-ready and CTS-ready; these cloud instances are then used to launch CVD and run CTS tests.

Users may select from standard machine types or create custom machine types.  If the `MACHINE_TYPE` parameter is set to an empty string, the custom parameter values will be used to create the machine type, i.e.:

- `CUSTOM_VM_TYPE`
- `CUSTOM_CPUS`
- `CUSTOM_MEMORY`

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
- The Google Compute Engine is configured with `noDelayProvisioning: false` in `gitops/workloads/values-jenkins.yaml` to help reduce costs. With this setting, multiple VM instances are not started immediately, which lowers expenses for each run. However, disabling immediate provisioning may slightly increase VM startup times. This trade-off allows users to choose between faster VM availability and lower operational costs.

## Environment Variables/Parameters <a name="environment-variables"></a>

**Jenkins Parameters:** Defined in the groovy job definition `groovy/job.groovy`.

### `ANDROID_CUTTLEFISH_REPO_URL`

This defines which repository will be used to create the cuttlefish instance from. Users may choose to use the standard Google repository, or their own fork and revisions. This allows users to fix issues in android-cuttlefish builds from their own repository versions.

If using your own repository, and it a private repository, ensure `REPO_USERNAME` and `REPO_PASSWORD` have been defined.

### `ANDROID_CUTTLEFISH_REVISION`

This defines the branch/tag to use from `ANDROID_CUTTLEFISH_REPO_URL`, e.g.

- `main` - the main working branch of `android-cuttlefish`
- `v1.27.0` - the latest tagged version.
- `horizon/main` - a private repository fork of `main`
- `horizon/v1.35.0` - a private fork of tag `v1.35.0`

User may define any valid version so long as that version contains `tools/buildutils/build_packages.sh` which is a dependency for these scripts.

### `CUTTLEFISH_INSTANCE_NAME`
**Note:** Name must be a match of regex `(?:[a-z](?:[-a-z0-9]{0,61}[a-z0-9])?)`, i.e lower case.

Optional parameter to allow users to create their own unique instance templates for use in development and/or testing.

If left empty, the name will be derived from `ANDROID_CUTTLEFISH_REVISION` e.g. `cuttlefish-vm-main` and create
an instance template `instance-template-cuttlefish-vm-main` and an image `image-cuttlefish-vm-main`. If the `ANDROID_CUTTLEFISH_REVISION` contains special characters, these will be replaced, eg. `/` replaced by `-` and `.` removed, this is to comply with GCE regex requirements.

If user defines a unique name, ensure the following is met:

- The name should start with `cuttlefish-vm`
- Jenkins CasC (`values-jenkins.yaml`) must be updated to provide a new `computeEngine` entry for this unique template. For reference, see existing entry for `cuttlefish-vm-main`.
  - Choose a sensible `cloudName`, such as `cuttlefish-vm-unique-name` (e.g. the same name as the instance template with the "instance-template" prefix removed).
  - Once synced, this new cloud will appear in `Manage Jenkins` -> `Clouds`
  - Tests jobs may then reference that unique instance by setting the `JENKINS_GCE_CLOUD_LABEL` parameter to the new cloud label (`cloudName`).

### `DELETE`

Allows deletion of an existing instance templates and its referenced image.

If deleting a standard instance template (i.e. name auto-generated), simply define the version in `ANDROID_CUTTLEFISH_REVISION` and the required names will be derived automatically.

- `ANDROID_CUTTLEFISH_REVISION`: choose the version you wish to delete
- `DELETE`: This ensures the instance template, disk image and VM instance are deleted.
- `Build` : trigger build to delete all artifacts.

If user is deleting a uniquely-created instance template (i.e. name specified by `CUTTLEFISH_INSTANCE_NAME`), then define `CUTTLEFISH_INSTANCE_NAME` as was used to create it (i.e. the same name as the instance template with the "instance-template" prefix removed).

- `CUTTLEFISH_INSTANCE_NAME`: choose the template unique name you wish to delete
- `DELETE`: This ensures the instance template, disk image and VM instance are deleted.
- `Build` : trigger build to delete all artifacts.

### `REPO_USERNAME`

Required if using a private repository defined in `ANDROID_CUTTLEFISH_REPO_URL`.

### `REPO_PASSWORD`

Required if using a private repository defined in `ANDROID_CUTTLEFISH_REPO_URL`.

### `ANDROID_CUTTLEFISH_POST_COMMAND`

Command to run in the `ANDROID_CUTTLEFISH_REPO_URL` defined repo. e.g.
- To fix the netsimd build issues with cxxbridge:
  - `git cherry-pick 78b66377`
- Replace stale repos cuttlefish may be using, such as old kernel.org repos that have been deleted:
  - `sed -i 's|https://git.kernel.org/pub/scm/linux/kernel/git/jaegeuk/f2fs-tools|https://github.com/jaegeuk/f2fs-tools|g' base/cvd/MODULE.bazel`

### `ANDROID_CUTTLEFISH_PREBUILT`

Users have the option to build cuttlefish from scratch, ie. from [android-cuttlefish.git](https://github.com/google/android-cuttlefish.git) repository. Alternatively they may choose to install Google prebuilt versions of cuttlefish.

Disabled: build and install from repo.
Enabled:  download and install Google prebuilt versions.

Note: this is only applicable to `ANDROID_CUTTLEFISH_REVISION` `main` branch currently, and if packages are not found it will default to building cuttlefish from scratch.

### `VM_INSTANCE_CREATE`

**Enable Stopped VM Instance Creation**

If enabled, this job will create a Cuttlefish VM instance from the final instance template. It will be placed in stop
state after creation. This is provided for development testing and debugging.

This would allow developers to:
- Connect to the instance directly
- Run tests on the instance manually, bypassing Jenkins

**Important:**
- Be aware that creating this instance may incur additional costs for your project.
- Enable this only for instance templates created for developement purposes that are created with a well defined `CUTTLEFISH_INSTANCE_NAME`.
- Set `MAX_RUN_DURATION` to 0 to ensure VM instance is never deleted on runtime expiry.
- It is advisable to `DELETE` these development instances when testing is completed.

### `MACHINE_TYPE`

The machine type to be used for the VM instance. For x86, the default is `n1-standard-64`. Whereas ARM64 currently only `c4a-highmem-96-metal` is available.

Defines the (--machine-type)[https://cloud.google.com/compute/docs/general-purpose-machines] parameter.

To create a custom machine type, do not define `MACHINE_TYPE` and instead define the 3 `CUSTOM` options which will specify the machine type.

### `CUSTOM_VM_TYPE`

Specifies a custom machine type.

Defines the (--custom-vm-type)[https://cloud.google.com/sdk/gcloud/reference/compute/instance-templates/create] parameter.

### `CUSTOM_CPU`

Specifies the number of cores needed for custom machine type.

Defines the (--custom-cpu)[https://cloud.google.com/sdk/gcloud/reference/compute/instance-templates/create] parameter.

### `CUSTOM_MEMORY`

Specifies the memory needed for custom machine type.

Defines the (--custom-memory)[https://cloud.google.com/sdk/gcloud/reference/compute/instance-templates/create] parameter.

### `BOOT_DISK_SIZE`

A boot disk is required to create the instance, therefore define the size of disk required.

### `BOOT_DISK_TYPE`

Define the Boot disk type. Typically:
- x86_64: `pd-balanced`
- ARM64: `hyperdisk-balanced`

### `MAX_RUN_DURATION`

VM instances are expensive so it is advisable to define the maximum amount of time to run the instance before it will automatically be terminated. This avoids leaving expensive instances in running state and consuming resources.

User may disable by setting the value to 0, but they must be aware of any costs that they may incur to their project.  Setting to 0 is useful when creating development test instances so users can connect directly to the VM instance.

### `JAVA_VERSION`

Specify the version of Java to install (`openjdk-17-jdk-headless`).

Must be OpenJDK and headless to avoid installation issues with various operating system versions.

### `OS_VERSION`

Override the OS version. These regularly become deprecated and superceded, hence option to update to newer version.

- x86_64: use versions from debian family only.
- ARM64: use versions from `ubuntu-2204-lts-arm64` family.

Refer to `gcloud compute images list` for the version names based on family.

### `OS_PROJECT`

Disk image project.

Refer to `gcloud compute images list` for the project names based on family and OS version.

### `CURL_UPDATE_COMMAND`

Command provided to upgrade Curl from standard OS release versions. In the case of debian, bakports are used.

e.g. `"sudo apt install -t bookworm-backports -y curl libcurl4` would update Curl to latest from Debian backports.

### `NODEJS_VERSION`

MTK Connect requires NodeJS; this option allows you to update the version to install on the instance template.

### `CTS_ANDROID_<14|15|16>_URL`

Defines the URL where to retrieve and install the Android CTS test harness. Leave blank if not required, or override the
current default using your own version, e.g. from bucket storage.

### `ARM64 Unique Configuration`

The following are unique to ARM64 support because support is currently in preview and limited to United States region,
therefore users may need to override if their projects are not located within `us-central1`.

#### `ADDITIONAL_NETWORKING`

Only applicable to ARM64 instances is still in early development support.

ARM64 bare metal currently require `nic-type=IDPF`

#### `SUBNET`

Define the subnet to use for ARM64 instances, or leave blank to use default platform subnet.

#### `REGION`

Region of the instance to create. Leave black to use the default platform region.

#### `ZONE`

Region of the instance to create. Leave black to use the default platform zone.

## Example Usage <a name="examples"></a>

If user wishes to create a temporary test instance to work with, then they can do so as follows from Jenkins:

- `ANDROID_CUTTLEFISH_REVISION`: choose the version you wish to build the template from
- `CUTTLEFISH_INSTANCE_NAME` : provide a name, starting with cuttlefish-vm, e.g. `cuttlefish-vm-test-instance-v110.`
- `MAX_RUN_DURATION` : set to 0 to avoid instance being deleted after this time.
- `VM_INSTANCE_CREATE` : Enable this option so that the instance template will create a VM instance for user to start, connect to and work with.
- `Build`

Once they have finished with the instances, they should delete to avoid excessive costs.

- `CUTTLEFISH_INSTANCE_NAME` : provide a unique name, starting with cuttlefish-vm, e.g. `cuttlefish-vm-test-instance-v110.`
- `DELETE` : This ensures the instance template, disk image and VM instance are deleted.
- `Build`

## SYSTEM VARIABLES <a name="system-variables"></a>

There are a number of system environment variables that are unique to each platform but required by Jenkins build, test and environment pipelines.

These are defined in Jenkins CasC `values-jenkins.yaml` and can be viewed in Jenkins UI under `Manage Jenkins` -> `System` -> `Global Properties` -> `Environment variables`.

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
