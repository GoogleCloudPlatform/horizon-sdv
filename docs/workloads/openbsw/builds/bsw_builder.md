# OpenBSW Builds

## Table of contents
- [Introduction](#introduction)
- [Prerequisites](#prerequisites)
- [Environment Variables/Parameters](#environment-variables)
  * [Targets](#targets)
- [System Variables](#system-variables)
- [Known Limitations](#known-limitations)

## Introduction <a name="introduction"></a>

**Eclipse Foundation OpenBSW Software Build Job**

This job automates the build process for the Eclipse Foundation OpenBSW software from a specified source repository and branch.

**Supported Targets and Build Options**

The job offers the following build targets and options:

- Unit Tests:
  - Build unit tests
  - Run unit tests (all or individual test library)
  - Generate code coverage reports
- Platform Builds:
  - POSIX Platform builds
  - NXP S32K148 Platform builds

**Artifact Storage**

Build artifacts are stored in two locations for easy access:

- **Jenkins Artifact Storage**: some artifacts such as test results, artifact summary are stored with the respective Jenkins build, providing a convenient reference point for retrieving artifacts from either location.
- **Google Cloud Storage**: larger artifacts are uploaded to the designated Google Cloud Storage bucket for the workload.

**Build Customization**

Users can choose to build all targets or select specific target groups using the provided parameters. There are also
options to override the build and test commands.

### References

- [Welcome to Eclipse OpenBSW](https://eclipse-openbsw.github.io/openbsw/sphinx_docs/doc/index.html).
- [Building and Running Unit Tests.](https://eclipse-openbsw.github.io/openbsw/sphinx_docs/doc/learning/unit_tests/index.html).
- [POSIX Platform](https://eclipse-openbsw.github.io/openbsw/sphinx_docs/doc/learning/setup/setup_posix_ubuntu_build.html#setup-posix-ubuntu-build).
- [NXP S32K148 Platform](https://eclipse-openbsw.github.io/openbsw/sphinx_docs/doc/learning/setup/setup_s32k148_ubuntu_build.html).
- [OpenBSW GitHub repo](https://github.com/eclipse-openbsw/openbsw.git).

## Prerequisites<a name="prerequisites"></a>

One-time setup requirements.

- Before running this pipeline job, ensure that the following template has been created by running the corresponding job:
  - Docker image template: `OpenBSW Workflows/Environment/Docker Image Template`

## Environment Variables/Parameters <a name="environment-variables"></a>

**Jenkins Parameters:** Defined in the groovy job definition `groovy/job.groovy`.

### `OPENBSW_GIT_URL`

This provides the URL for the OpenBSW repository. Such as:
- https://github.com/eclipse-openbsw/openbsw.git

### `OPENBSW_GIT_BRANCH`

This provides the branch/tag revision for the OpenBSW repository.

### `POST_GIT_CLONE_COMMAND`

Optional parameter that allows the user to include additional commands to run after the repository has been cloned.

### `IMAGE_TAG`

Specifies the name of the Docker image to be used when running this job.

The default value is defined by the `Seed Workloads` pipeline job. Users may override to provide a unique tag that describes the Linux distribution and tool chain versions.

### `CMAKE_SYNC_JOBS`

Defines the number of parallel sync jobs when running `cmake` commands.

### `CODE_COVERAGE`

Enable code coverage for unit tests. Only applicable when `BUILD_UNIT_TESTS` and `RUN_UNIT_TESTS` are enabled.

### `LIST_UNIT_TESTS`

This will create an artifact that shows all available unit tests, should users wish to target test to an individual test
rather than all.

### `LIST_UNIT_TESTS_CMDLINE`

The command that is used to list unit tests. Users may choose to override or retain default.

### `BUILD_UNIT_TESTS`

Build the unit tests. This will build `all` or that which is specified in `UNIT_TEST_TARGET`.

### `UNIT_TEST_TARGET`

Specify whether to build all tests or a specific test. See `LIST_UNIT_TESTS` which will generate a list of all available
tests.

### `UNIT_TESTS_CMDLINE`

The command that is used to build unit tests. Users may choose to override or retain default.

### `RUN_UNIT_TESTS`

Once unit tests are built, this will ensure unit tests are run. This is dependent on `BUILD_UNIT_TESTS` being enabled.

### `RUN_UNIT_TESTS_CMDLINE`

The command that is used to run unit tests. If the `UNIT_TEST_TARGET` is `all` this can be left as is. But if using
individual targets, it is recommended to either run the test target directly or use `ctest` and specify the test target
directory.

e.g. `UNIT_TEST_TARGET` set to `bspTest`:

`cmake-build-unit-tests/openbsw/libs/bsw/bsp/test/gtest/bspTest`
or
`ctest --test-dir cmake-build-unit-tests/openbsw/libs/bsw/bsp/test/gtest -j7`

### `BUILD_POSIX`

Build the OpenBSW POSIX target. This will build the `app.referenceApp.elf` application and store in respective GCS bucket, for later use in the test pipeline job.

### `POSIX_BUILD_CMDLINE`

The command that is used to build the POSIX platform target. Users may choose to override or retain default.

### `POSIX_ARTIFACT`

The artifact to store. Default is the `app.referenceApp.elf`.

### `BUILD_NXP_S32K148`

Build the OpenBSW S32K148 Hardware target. This will build the `app.referenceApp.elf` application and store in respective GCS bucket for user to retrieve and install on their physical hardware.

### `NXP_S32K148_BUILD_CMDLINE`

The command that is used to build the NXP S32K148 platform target. Users may choose to override or retain default.

### `NXP_S32K148_ARTIFACT`

The artifact to store. Default is the `app.referenceApp.elf`.

### `INSTANCE_RETENTION_TIME`

Keep the build VM instance and container running to allow user to connect to it. Useful for debugging build issues, determining target output archives etc. Time in minutes.

Access using `kubectl` e.g. `kubectl exec -it -n jenkins <pod name> -- bash` from `bastion` host.

### `OPENBSW_ARTIFACT_STORAGE_SOLUTION`

Define storage solution used to push artifacts.

Currently `GCS_BUCKET` default pushes to GCS bucket, if empty then nothing will be stored.

## SYSTEM VARIABLES <a name="system-variables"></a>

There are a number of system environment variables that are unique to each platform but required by Jenkins build, test and environment pipelines.

These are defined in Jenkins CasC `jenkins.yaml` and can be viewed in Jenkins UI under `Manage Jenkins` -> `System` -> `Global Properties` -> `Environment variables`.

These are as follows:

-   `OPENBSW_BUILD_BUCKET_ROOT_NAME`
     - Defines the name of the Google Storage bucket that will be used to store build and test artifacts

-   `OPENBSW_BUILD_DOCKER_ARTIFACT_PATH_NAME`
    - Defines the registry path where the Docker image used by builds, tests and environments is stored.

-   `CLOUD_PROJECT`
    - The GCP project, unique to each project. Important for bucket, registry paths used in pipelines.

-   `CLOUD_REGION`
    - The GCP project region. Important for bucket, registry paths used in pipelines.

-   `HORIZON_DOMAIN`
    - The URL domain which is required by pipeline jobs to derive URL for tools and GCP.

-   `HORIZON_GITHUB_URL`
    - The URL to the Horizon SDV GitHub repository.

-   `HORIZON_GITHUB_BRANCH`
    - The branch name the job will be configured for from `HORIZON_GITHUB_URL`.

-   `JENKINS_SERVICE_ACCOUNT`
    - Service account to use for pipelines. Required to ensure correct roles and permissions for GCP resources.

## Known Limitations<a name="known-limitations">

**Document Generation:**

This will be added in future releases.

**Repository Access Control:**

Please note that support is only provided for open-source repositories with no access control. If access control is required, additional credentials will be necessary in Horizon-SDV, and the Jenkinsfile will need to be updated to include the retrieval and storage of these credentials in Git.
