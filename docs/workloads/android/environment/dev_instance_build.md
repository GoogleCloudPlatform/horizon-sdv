# Development Build Instance

## Table of contents
- [Introduction](#introduction)
- [Prerequisites](#prerequisites)
- [Environment Variables/Parameters](#environment-variables)
- [System Variables](#system-variables)

## Introduction <a name="introduction"></a>

During developing the Android workload and workflow/pipelines, sometimes it may be necessary to gain access to a VM build instance in order to develop build jobs. The instance will have the build caches, persistent storage mounted.

Those that require access must be able to connect to the pod using `kubectl`, e.g.

```
kubectl exec -it -n jenkins <pod name> -- bash
```

Reference [Fleet management](https://docs.cloud.google.com/kubernetes-engine/enterprise/multicluster-management/gateway) to fetch credentials for a fleet-registered cluster to be used in Connect Gateway, e.g.
- `gcloud container fleet memberships list`
- `gcloud container fleet memberships get-credentials sdv-cluster`

Alternatively access Host via MTK Connect by enabling MTK_CONNECT_ENABLE.

- These instances only remain active for a limited time, defined by `INSTANCE_MAX_UPTIME`.
- User can find `<pod name>` from either the Jenkins UI console log or from the Jenkins Build Executor nodes.
- Users are responsible for managing their work and saving to their own storage, that's beyond the purpose of this job.

## Prerequisites<a name="prerequisites"></a>

One-time setup requirements.

- Before running this pipeline job, ensure that the following template has been created by running the corresponding job:
  - Docker image template: `Android Workflows/Environment/Docker Image Template`

## Environment Variables/Parameters <a name="environment-variables"></a>

**Jenkins Parameters:** Defined in the groovy job definition `groovy/job.groovy`.

### `ANDROID_VOLUME`

This specifies which build disk pool to use for the development instance.

For ABFS build instances you select the `abfs` to mount persistent volume used to store the ABFS cache.

### `INSTANCE_MAX_UPTIME`

This is the maximum time that the instance may be running before it is automatically terminated and deleted. This is important to avoid leaving expensive instances in running state.

### `MTK_CONNECT_ENABLE`

Enable if wishing to use MTK Connect to connect to the host instance rather than kubectl.

### `MTK_CONNECT_PUBLIC`

When checked, the MTK Connect testbench is visible to everyone and can be shared.
By default, testbenches are private and only visible to their creator and MTK Connect administrators.

### `NUM_HOST_INSTANCES`

Number of host instances to create for dev instances. This is effectively the number of devices that will be created associated with the development instance testbench in MTK Connect.

### `USE_LOCAL_AOSP_MIRROR`

If checked, the build instance will mount the AOSP Mirror setup in your GCP project to fetch Android source code during `repo sync`.
**Note:**
-  The AOSP Mirror must be setup prior to running this job. If not setup, the job will fail.
-  The setup jobs are in folder `Android Workflows -> Environment -> Mirror`.

### `AOSP_MIRROR_DIR_NAME`

This defines the directory name on the Filestore volume where the Mirror is located.
**Note:**
-  This is required if `USE_LOCAL_AOSP_MIRROR` is checked.
-  e.g. If you provided `my-mirror` when creating the mirror, provide the same value here.

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

-   `JENKINS_AAOS_BUILD_CACHE_STORAGE_PREFIX`
    - This identifies the Persistent Volume Claim (PVC) prefix that is used to provision persistent storage for build cache, ensuring efficient reuse of cached resources across builds.  The default is [`pd-balanced`](https://cloud.google.com/compute/docs/disks/performance), which strikes a balance between optimal performance and cost-effectiveness.

-   `JENKINS_SERVICE_ACCOUNT`
    - Service account to use for pipelines. Required to ensure correct roles and permissions for GCP resources.

-    `AOSP_MIRROR_PRESET_FILESTORE_PVC_MOUNT_PATH_IN_CONTAINER`

-    `AOSP_MIRROR_PRESET_MIRROR_ROOT_SUBDIR_NAME`

-    `AOSP_MIRROR_DIR_NAME`
