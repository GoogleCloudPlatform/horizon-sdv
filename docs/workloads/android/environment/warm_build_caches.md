<!-- Copyright (c) 2026 Accenture, All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

        http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License. -->

# Warm Build Caches

Jenkins job **`Android/Environment/Warm Build Caches`** pre-populates AAOS build cache persistent storage by running a fixed sequence of AAOS Builder–style builds for a chosen manifest and revision.

**Definitions:** `workloads/android/pipelines/environment/warm_build_caches/groovy/job.groovy` (parameters, job UI), `workloads/android/pipelines/environment/warm_build_caches/Jenkinsfile` (pipeline).

## Table of contents

- [Introduction](#introduction)
- [Prerequisites](#prerequisites)
- [Parameters](#parameters)
- [Pipeline behavior](#pipeline-behavior)
- [Lunch targets and revision prefixes](#lunch-targets-and-revision-prefixes)
- [Kubernetes agent](#kubernetes-agent)
- [System variables](#system-variables)

## Introduction

The job accelerates later AAOS builds by filling the build cache PVC before normal developer or CI builds attach to it. Each run uses an **ephemeral** `aaos-cache` volume (see [Kubernetes agent](#kubernetes-agent)); the storage class is keyed off the Android pool (`ANDROID_VERSION` / inferred SDK version).

Run multiple warm-cache jobs **in parallel** if you need several warm PVs (respect cluster limits, e.g. Kubernetes caps).

## Prerequisites

- **Docker image:** Build image must exist (e.g. from **Android Workflows → Environment → Docker Image Template**), matching what AAOS Builder uses (`ANDROID_BUILD_DOCKER_ARTIFACT_PATH_NAME`).
- **Empty build PVs:** Per job UI, **delete existing build cache PVCs** before running so warm runs start from a defined state (see groovy description).
- **AOSP mirror (optional):** If **`USE_LOCAL_AOSP_MIRROR`** is enabled, mirror PVC and **Android Workflows → Environment → Mirror** setup must exist or the job will fail.

## Parameters

Defined in **`groovy/job.groovy`** (Jenkins UI).

| Parameter | Description |
|-----------|-------------|
| **`AAOS_GERRIT_MANIFEST_URL`** | Repo manifest URL (default: Horizon Gerrit manifest). |
| **`AAOS_REVISION`** | Branch or tag (default in groovy: `horizon/android-17.0.0_r1`). Must contain `android-14`, `android-15`, `android-16`, or `android-17` so the job can infer the SDK / storage pool when **`ANDROID_VERSION`** is `default` (see **`Jenkinsfile`** `getAndroidVersion()`). |
| **`ANDROID_VERSION`** | `default` \| `14` \| `15` \| `16` \| `17`. Selects the build-cache disk pool / `STORAGE_CLASS_SUFFIX` (`android-${version}`). With **`default`**, the job derives the version from **`AAOS_REVISION`**. |
| **`GERRIT_REPO_SYNC_JOBS`** | Parallel jobs for `repo sync` (default from seed / `${REPO_SYNC_JOBS}`). |
| **`AAOS_CLEAN`** | `NO_CLEAN` \| `CLEAN_BUILD` \| `CLEAN_ALL`. Controls clean behavior across stages (see [Pipeline behavior](#pipeline-behavior)). |
| **`ARCHIVE_ARTIFACTS`** | If true, artifacts are stored via GCS (`ARTIFACT_STORAGE_SOLUTION=GCS_BUCKET` in **`Jenkinsfile`**); if false, storage is effectively noop for that flag. |
| **`USE_LOCAL_AOSP_MIRROR`** | Mounts the preset Filestore mirror PVC read-only and sets mirror paths for `repo sync`. |
| **`AOSP_MIRROR_DIR_NAME`** | Mirror directory on Filestore (required when mirror is enabled). |

### `AAOS_CLEAN` (aligned with `Jenkinsfile`)

- **`CLEAN_ALL`:** Applied only to the **first** build stage (**Build: aosp_cf_x86_64_auto**) via `AAOS_CLEAN_FIRST_STAGE`. Later stages use **`CLEAN_BUILD`**, not another full **`CLEAN_ALL`**.
- **`CLEAN_BUILD`:** Same mode on all stages (`AAOS_CLEAN_LATER_STAGES` matches).
- **`NO_CLEAN`:** Same on all stages.

## Pipeline behavior

Single top-level stage **Start Build VM Instance** runs these stages in order:

1. **Initialise** — Records build description, resolves **`ANDROID_BUILD_ID`** prefix from **`AAOS_REVISION`** (see table below), sets **`AAOS_CLEAN_FIRST_STAGE`** / **`AAOS_CLEAN_LATER_STAGES`**, writes **`build_cache_volume.txt`**, configures Gerrit git credentials, archives `build_cache*.txt`.
2. **Build: aosp_cf_x86_64_auto** — First full warm; uses **`AAOS_CLEAN_FIRST_STAGE`**.
3. **Build: aosp_cf_arm64_auto** — Uses **`AAOS_CLEAN_LATER_STAGES`**.
4. **Build: sdk_car_x86_64** — Sets **`ANDROID_VERSION`** to the resolved SDK version; runs **`aaos_avd_sdk.sh`** (allowed to fail: `|| true`); uses later-stage clean mode.
5. **Build: sdk_car_arm64** — Same pattern as sdk_car_x86_64.
6. **Build: aosp_tangorpro_car** — Android 16 and Android 17 revisions are skipped because compatible device binaries are not configured.

Each build stage invokes **`aaos_initialise.sh`**, **`aaos_build.sh`**, and **`aaos_storage.sh`** (and **`aaos_avd_sdk.sh`** where shown), with **`AAOS_LUNCH_TARGET`** and **`AAOS_BUILD_NUMBER`** set per stage. Stages use **`catchError`** so a failure is recorded but later stages may still run depending on Jenkins behavior.

Gerrit credentials: **Initialise** and **sdk_car_x86_64** use **`GERRIT_CREDENTIALS_ID`**; other build stages use the fixed credential id **`jenkins-gerrit-http-password`** (as in **`Jenkinsfile`**).

## Lunch targets and revision prefixes

**`AAOS_LUNCH_TARGET`** per stage (suffix is **`${ANDROID_BUILD_ID}userdebug`**):

| Stage | Lunch target base |
|-------|-------------------|
| aosp_cf_x86_64_auto | `aosp_cf_x86_64_auto-…` |
| aosp_cf_arm64_auto | `aosp_cf_arm64_auto-…` |
| sdk_car_x86_64 | `sdk_car_x86_64-…` |
| sdk_car_arm64 | `sdk_car_arm64-…` |
| aosp_tangorpro_car | `aosp_tangorpro_car-…` |

**`ANDROID_BUILD_ID`** prefix (empty if no match), from **`Jenkinsfile`**:

| If `AAOS_REVISION` contains | `ANDROID_BUILD_ID` |
|-----------------------------|--------------------|
| `android-14.0.0_r30` | `ap1a-` |
| `android-14.0.0_r74` | `ap2a-` |
| `android-15.0.0_r36` | `bp1a-` |
| `android-16.0.0_r3` | `bp3a-` |
| `android-16.0.0_r4` | `bp4a-` |
| `android-17.0.0_r1` | `cp2a-` |

If **`AAOS_REVISION`** does not match a supported Android 14–17 revision, the job fails in **`getAndroidVersion()`** when **`ANDROID_VERSION`** is **`default`**.

## Kubernetes agent

From **`Jenkinsfile`** `environment` block:

- **Agent:** Kubernetes pod from **`POD_TEMPLATE`**: without mirror (**`kubernetesPodTemplateWithoutMirror`**) or with mirror (**`kubernetesPodTemplateWithMirror`**) when **`USE_LOCAL_AOSP_MIRROR`** is true.
- **Pool:** `nodeSelector` **`workloadLabel: android`**, toleration **`workloadType=android:NoSchedule`**, pod anti-affinity on **`aaos_pod`** so pods spread across nodes.
- **Container `builder`:** AAOS build image (`ANDROID_BUILD_DOCKER_ARTIFACT_PATH_NAME`), **privileged**, **`sleep`** 6h (no mirror) or 5h (mirror).
- **Resources:** `98000m` CPU, `180000Mi` memory (limits = requests).
- **Cache volume:** Ephemeral PVC **`aaos-cache`** mounted at **`/aaos-cache`**, **1000Gi**, storage class **`${JENKINS_AAOS_BUILD_CACHE_STORAGE_PREFIX}-${STORAGE_CLASS_SUFFIX}`** with **`STORAGE_CLASS_SUFFIX`** = `android-${SDK_ANDROID_VERSION}`.
- **Mirror (optional):** Extra read-only mount from **`MIRROR_PRESET_FILESTORE_PVC_NAME`** at **`MIRROR_PRESET_FILESTORE_PVC_MOUNT_PATH_IN_CONTAINER`**.

## System variables

Shared Jenkins / Horizon variables (CasC, global properties) used by this and other Android pipelines include:

- **`ANDROID_BUILD_BUCKET_ROOT_NAME`**, **`ANDROID_BUILD_DOCKER_ARTIFACT_PATH_NAME`**
- **`CLOUD_PROJECT`**, **`CLOUD_REGION`**, **`CLOUD_ZONE`**
- **`GERRIT_CREDENTIALS_ID`**, **`HORIZON_DOMAIN`**, **`HORIZON_SCM_URL`**, **`HORIZON_SCM_BRANCH`**
- **`JENKINS_AAOS_BUILD_CACHE_STORAGE_PREFIX`**, **`JENKINS_SERVICE_ACCOUNT`**
- **`MIRROR_PRESET_FILESTORE_PVC_MOUNT_PATH_IN_CONTAINER`**, **`MIRROR_PRESET_MIRROR_ROOT_SUBDIR_NAME`**, **`MIRROR_PRESET_FILESTORE_PVC_NAME`** (and related mirror settings)

See Jenkins **Manage Jenkins → System → Global properties** and `gitops/workloads/values-jenkins.yaml` for the authoritative list.