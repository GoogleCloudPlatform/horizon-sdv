# Seed Workload Pipeline

## Table of contents
- [Introduction](#introduction)
- [Prerequisites](#prerequisites)
- [Parameters](#parameters)
- [Groovy Script Approval](#approvals)
- [Update Mechanism](#update)

## Introduction <a name="introduction"></a>

This pipeline job is responsible for managing workload folders and jobs in Jenkins. It serves two primary purposes:

- Initializing workload areas
- Updating Jenkins to incorporate changes to existing jobs and new job definitions

This pipeline job is responsible for managing workload folders and jobs in Jenkins. It serves two primary purposes:

Updating Jenkins to incorporate changes to existing jobs and new job definitions

- Top-level folder definitions: `workloads/<workload_name>/pipelines/groovy/folders.groovy`
- Individual job definitions: `workloads/<workload_name>/pipelines/<folder_name>/<job_name>/groovy/job.groovy`

## Prerequisites<a name="prerequisites"></a>

Refer to [Pipeline Guide](guides/pipeline_guide.md#prerequisites) for common prerequisites.

## Parameters <a name="parameters"></a>

### `SEED_WORKLOAD`
Specifies which workload(s) to seed

- `none` (default) seed nothing - derisk triggering by accident or on Jenkins restart.
- `all` seed all workloads.
- `android` seed the Android workload.
- `openbsw` seed the OpenBSW workload.
- `cloud-workstations` seed the Cloud Workstations workload.

### `BUILDKIT_RELEASE_TAG`
The version of Buildkit to use to build the container image.

### `DOCKER_CREDENTIALS_URL`
URL of Google docker credentials helper, required to allow access to the project artifact registry.

### `GCLOUD_CLI_VERSION`
Version of [Google Cloud CLI](https://docs.cloud.google.com/sdk/docs/release-notes) to install.
Define `latest` if wishing to use the latest available version.

### `KUBECTL_VERSION`
Version of `kubectl` to install. The version is typically `1:${GCLOUD_CLI_VERSION}`.
Define `latest` if wishing to use the latest available version.

### `REPO_SYNC_JOBS`
Defines the number of parallel sync jobs when running `repo sync`.
This value will propogate to Android pipeline jobs.

Consider the value carefully and whether using Google Opensource Gerrit repos, local repos or even the AOSP Mirror.

### `CUTTLEFISH_GCE_CLOUD_LABEL`
This is the label that identifies the GCE Cloud label which will be used to identify the Cuttlefish VM instance.
This value will propogate to Android pipeline jobs.

## `USE_LOCAL_AOSP_MIRROR`
If checked, the build will use the AOSP Mirror setup in your GCP project to fetch Android source code during `repo sync`.
**Note:**
-  The AOSP Mirror must be setup prior to running this job. If not setup, the job will fail.
-  The setup jobs are in folder `Android Workflows -> Environment -> Mirror`.

### `AOSP_MIRROR_DIR_NAME`
This defines the directory name on the Filestore volume where the Mirror is located.
**Note:**
-  This is required if `USE_LOCAL_AOSP_MIRROR` is checked.
-  e.g. If you provided `my-mirror` when creating the mirror, provide the same value here.

## `ABFS_VERSION`
Defines the version for use with the ABFS server, uploader and build jobs.

## `ABFS_CASFS_VERSION`
Defines the ABFS CASFS version for use with the ABFS build jobs.

## `ABFS_COS_IMAGE_REF`
Defines the ABFS Containerized OS images used on server and uploader instances.

Use `gcloud compute images list --no-standard-images --project=cos-cloud | grep lts` to check for available LTS images.

## `ABFS_REPOSITORY`
Defines the artifact repository from where to retrieve the ABFS packages.

## `UPLOADER_MANIFEST_SERVER`
ABFS manifest source URL. Used for seeding ABFS builds, blobs/objects.

### `OPENBSW_IMAGE_TAG`
Defines the name of the build image tag used for OpenBSW pipelines.
This value will propogate to OpenBSW pipeline jobs.

### `OPENBSW_GIT_URL`
This provides the URL for the OpenBSW repository. Such as:
- https://github.com/eclipse-openbsw/openbsw.git

### `OPENBSW_GIT_BRANCH`
This provides the branch/tag revision for the OpenBSW repository.

### Groovy Scripts <a name="groovyscripts"></a>

This job uses the "Authorize Project" plugin to set an authorization property, allowing the job to run as the user who triggered the build. This is configured as follows:

```
    properties {
      authorizeProjectProperty {
        strategy {
          triggeringUsersAuthorizationStrategy()
        }
      }
    }
```

In conjunction with the sandbox protection utility for Job DSL, this setup ensures that explicit script approval is not required every time a change is made to a Groovy script. The Job DSL targets are specified as follows:

	`jobDsl targets: 'workloads/<workload_name>/pipelines/*/*/groovy/*.groovy', sandbox: true`

### Groovy Methods - Environment Variable Handling <a name="groovymethods"></a>

To avoid explicit script approval, environment variables used in Groovy files are replaced with their actual values before the files are referenced in the Jenkinsfile.

This replacement is performed in the _"Prepare Groovy files"_ stage, using a predefined replacement list. This approach bypasses the need for explicit script approval, which is typically required when using the `getProperty` Groovy method to resolve environment variables from within the job.groovy files.

A replacement list is used to substitute environment variables with their actual values, ensuring that the Groovy files can be executed without requiring explicit script approval. There is a common list of variables to replace that is used by all workloads, e.g.

```
def HEADER_STYLE = ' color: white; background: blue; padding: 8px; text-align: center; '
def SEPARATOR_STYLE = ' border: 0; border-bottom: 1px solid #ccc; background: #999; '

// Single quotes simply match string, double quotes expand the value of the variable.
// This avoids DSL scripts (loosely based on Groovy) needing to approve getProperty method
// which is a real security risk across Jenkins.
// This array can be updated to include other mappings as required.
def replacements = [
 ['${CLOUD_REGION}', "${CLOUD_REGION}"],
 ['${CLOUD_PROJECT}', "${CLOUD_PROJECT}"],
 ['${HORIZON_DOMAIN}', "${HORIZON_DOMAIN}"],
 ['${HORIZON_GITHUB_URL}', "${HORIZON_GITHUB_URL}"],
 ['${HORIZON_GITHUB_BRANCH}', "${HORIZON_GITHUB_BRANCH}"],
 ['${HEADER_STYLE}', "${HEADER_STYLE}"],
 ['${SEPARATOR_STYLE}', "${SEPARATOR_STYLE}"]]
```

Then the workload stages append their unique replacements, e.g.

```
                replacements += [
                  ['${OPENBSW_BUILD_BUCKET_ROOT_NAME}', "${OPENBSW_BUILD_BUCKET_ROOT_NAME}"],
                  ['${OPENBSW_BUILD_DOCKER_ARTIFACT_PATH_NAME}', "${OPENBSW_BUILD_DOCKER_ARTIFACT_PATH_NAME}"],
                  ['${OPENBSW_IMAGE_TAG}', "${OPENBSW_IMAGE_TAG}"]
                ]

```
> [!NOTE]
> - Separating the lists reduces time for script replacement stage and also maintenance.
> - The values used to make the replacements can be seen in the console output
> - See the following example where the <i>CLOUD_REGION</i> system variable is replaced with the string 'europe-west':
>   - `sed -i s/${CLOUD_REGION}/europe-west1/g`

> [!IMPORTANT]
> Any new environment variables added to groovy files need to be added to this replacements list as per the existing entries.

## Update Mechanism<a name="update"></a>

> [!NOTE]
> Build History is preserved when a job is edited.


> [!IMPORTANT]
> Environment variables can be referenced in Groovy files, but they are replaced with their actual values before the Groovy files are executed by the seed job. This approach avoids the need for explicit script approval, which is required when using the `getProperty` Groovy method to resolve environment variables.
> The replacement of environment variables with their actual values is performed in the _"Prepare Groovy files"_ stage of the seed job. Therefore, it is crucial to update the replacements list in the Jenkinsfile whenever new environment variables are added to Groovy scripts.
> To ensure that environment variables are properly replaced, please refer to the [Seed Workloads](seed.md#groovymethods) documentation for more information on how to update the replacements list in the Jenkinsfile.
> Environment variables are defined in the `gitops/workloads/values-jenkins.yaml` file (CasC).

To make changes to pipeline jobs (or folders):

1.  Edit the groovy file
2.  Commit and push the change
3.  Run this job & wait for completion

For more detailed information on various types of edits, see [Pipeline Guide](guides/pipeline_guide.md#edits).
