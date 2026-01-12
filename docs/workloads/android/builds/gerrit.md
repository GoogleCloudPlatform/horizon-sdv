# Gerrit Patchset Build Pipeline

## Introduction
This pipeline builds Android targets based on a Gerrit patchset change. It is only applicable to Jenkins.

Currently it is designed to run a base set of builds that can be used to verify the change, together with a basic CTS test run.

It supports the following branches:

-   `horizon/android-14.0.0_r30`
-   `horizon/android-15.0.0_r36`
-   `horizon/android-16.0.0_r3`

The branch is used to derive the full name (build identifier) of the build targets, e.g.

-   `android-14.0.0_r30` -> `sdk_car_x86_64-ap1a-userdebug`
-   `android-15.0.0_r36` -> `sdk_car_x86_64-bp1a-userdebug`
-   `android-16.0.0_r3` -> `sdk_car_x86_64-bp3a-userdebug`

It builds the following targets:

-   `sdk_car_x86_64`
-   `sdk_car_arm64`
-   `aosp_cf_x86_64_auto`
-   `aosp_tangorpro_car`

Once completed the build artifacts are available to test within the GCS bucket. Artifact summaries are provided stored with the job to provide details of the location of these artifacts.

## Notes
**Pipeline scope and future plans**

This pipeline is a starting point for demonstrating automated build capabilities, but it is not intended to be a universal solution. Currently, it builds a single component. Future plans include enhancing the pipeline to support topic-based change sets, allowing for the automated build of all related changes.

## Prerequisites<a name="prerequisites"></a>

One-time setup requirements.

- Before running this pipeline job, ensure that the following templates have been created by running the corresponding jobs:
  - Docker image template: `Android Workflows/Environment/Docker Image Template`
  - Cuttlefish instance template: `Android Workflows/Environment/CF Instance Template`

To successfully run the pipeline, ensure that the referenced Cuttlefish instance template exists, as specified in the `JENKINS_GCE_CLOUD_LABEL` variable defined in the Android Seed job. If the template is missing, the job will fail. The variable must reference align with the `computeEngine` label of the instance you intend to use.

## Gerrit Triggers

The pipeline is triggered by a Gerrit patchset change, or topic update based on Gerrit Triggers plugin. It uses the Horizon default path and branch name prefixes:
- Project prefix path: `android` separates projects into Android workload.
- Branch prefix path: `horizon` and separates branch names from upstream branches.

The trigger for the job is configured in `workloads/android/pipelines/builds/gerrit/groovy/job.groovy`, e.g.

```
properties{
  pipelineTriggers{
    triggers{
      gerrit{
        buildCancellationPolicy{
          abortAbandonedPatchsets(false)
          abortManualPatchsets(true)
          abortNewPatchsets(false)
          abortSameTopic(true)
        }
        gerritProjects{
          gerritProject{
            compareType('REG_EXP')
            pattern('^android\\/(?!.*\\/manifest$).*')
            branches{
              branch{
                compareType('ANT')
                pattern('**/horizon/*')
              }
            }
            disableStrictForbiddenFileVerification(true)
          }
        }
        triggerOnEvents{
          patchsetCreated()
        }
      }
    }
  }
}
```

## SYSTEM VARIABLES

There are a number of system environment variables that are unique to each platform but required by Jenkins build, test and environment pipelines.

These are defined in Jenkins CasC `values-jenkins.yaml` and can be viewed in Jenkins UI under `Manage Jenkins` -> `System` -> `Global Properties` -> `Environment variables`.

These are as follows:

-   `ANDROID_BUILD_BUCKET_ROOT_NAME`
     - Defines the name of the Google Storage bucket that will be used to store build and test artifacts

-   `ANDROID_BUILD_DOCKER_ARTIFACT_PATH_NAME`
    - Defines the registry path where the Docker image used by builds, tests and environments is stored.

-   `CLOUD_PROJECT`
    - The GCP project, unique to each project. Important for bucket, registry paths used in pipelines.

-   `CLOUD_REGION`
    - The GCP project region. Important for bucket, registry paths used in pipelines.

-   `CLOUD_ZONE`
    - The GCP project zone. Important for bucket, registry paths used in pipelines.

-   `GERRIT_CREDENTIALS_ID`
    - The credential for access to Gerrit, required for build pipelines.

-   `GERRIT_CUTTLEFISH_INSTANCE_TEMPLATE_LABEL`
    - The name of the Cuttlefish instance template to use for the pipeline.

-   `HORIZON_DOMAIN`
    - The URL domain which is required by pipeline jobs to derive URL for tools and GCP.

-   `HORIZON_GITHUB_URL`
    - The URL to the Horizon SDV GitHub repository.

-   `HORIZON_GITHUB_BRANCH`
    - The branch name the job will be configured for from `HORIZON_GITHUB_URL`.

-   `JENKINS_AAOS_BUILD_CACHE_STORAGE_PREFIX`
    - This identifies the Persistent Volume Claim (PVC) prefix that is used to provision persistent storage for build cache, ensuring efficient reuse of cached resources across builds.  The default is [`pd-balanced`](https://cloud.google.com/compute/docs/disks/performance), which strikes a balance between optimal performance and cost-effectiveness.

-   `JENKINS_SERVICE_ACCOUNT`
    - Service account to use for pipelines. Required to ensure correct roles and permissions for GCP resources.

-    `AOSP_MIRROR_PRESET_FILESTORE_PVC_MOUNT_PATH_IN_CONTAINER`

-    `AOSP_MIRROR_PRESET_MIRROR_ROOT_SUBDIR_NAME`

-    `AOSP_MIRROR_DIR_NAME`

## KNOWN ISSUES <a name="known-issues"></a>

Refer to `docs/workloads/android/builds/aaos_builder.md` for details.
