#  Android Build Filesystem (ABFS) Integration in Horizon SDV Platform

## Table of contents
- [Overview](#overview)
- [Prerequisites](#prerequisites)
  - [Service Account](#serviceaccount)
  - [ABFS License](#license)
  - [Horizon-SDV Setup](#horizonsetup)
- [Known Issues](#knownissues)

## Introduction <a name="overview"></a>

This document outlines the integration of Google's Android Build Filesystem (ABFS) into the Horizon SDV platform. ABFS provides a unique filesystem and caching solution designed to accelerate Android builds and source code checkouts.

> [!IMPORTANT]
> **Early Access Program (EAP)**: This feature is currently available under the Early Access Program (EAP) and is subject to certain restrictions and requirements.
>
> **Licensed Feature**: The use of ABFS is licensed by Google, and its inclusion in the Horizon SDV platform is subject to the terms and conditions of the license agreement.
>
> **Contact Google Representative**: For more information about ABFS and its integration in the Horizon SDV platform, please contact your Google representative.

## Prerequisites<a name="prerequisites"></a>

Before using ABFS, the following prerequisites must be satisfied:

### Service Account<a name="serviceaccount"></a>

In order to request a license, the GCP project must contain a new service account used for licensing of ABFS. The
licensing is used to provide access to Google registries to retrieve the ABFS images and packages. The service account
will be `allow-listed` by Google when approved.

To create the `abfs-server` service account, do the following:

1. Open GCP Cloud Console → `Service Accounts`
2. Select `Create service account`
  - `Service Account Name: abfs-server`
  - `Service Account Description: ABFS License Service Account`
  - Select `Create and continue`
3. Update `Permissions`:
  - `Artifact Registry Reader`
  - `Cloud Spanner Database User`
  - `Log Writer`
  - `Monitoring Metric Writer`
  - `Monitoring Viewer`
  - `Stackdriver Resource Metadata Writer`
  - `Storage Admin Object`
4. Select `Continue` and `Done`
5. Note the Email and Unique ID for later license request to Google.

### ABFS License<a name="license"></a>

Android Build File System - EAP</a>: Android Build File System is currently available to selected partners in an early access program. If you are interested in ABFS, please <a href=https://docs.google.com/forms/d/e/1FAIpQLSe-nqkIEADve-JqOlJEZf4E1hOyx6FXUXeH6Y64vrW3qj45Ng/viewform>submit this form</a>.

### Horizon-SDV Setup<a name="horizonsetup"></a>

Once Google provide you the ABFS license in JSON form, you will be required to create a new secret in the Horizon-SDV GitHub environment secrets, e.g.

1. **Base64 encode the license** Ensure there are no stray spaces in the license nor new lines. Then:
   ```echo -n '<LICENSE STRING>' | base64```
2. **GitHub secret**: the secret now needs to be added to the GitHub environment.
   - Open ```https://github.com/<your horizon sdv fork>/settings → Environment```
   - Select your environment, e.g. `main`
   - In `Environment secrets` select` Add Environment Secret`
     - `Name`: `ABFS_LICENSE_B64`
     - `Value`: <paste the base64 encoded license from step 1>
     - Then select `Add Secret`
3. **GitHub Terraform Workflow**: the secret must now be applied to the project using Terraform workflow
   - Open ```https://github.com/<your horizon sdv fork>/actions```
   - Select `Actions → Terraform`
   - Select `Run Workflow`
   - Select the branch, e.g. main
   - Select `Run workflow` and wait for Terraform to apply and complete all stages.
4. **ArgoCD**: ensure the secret is propogated.
   - Open ArgoCD from the Horizon-SDV landing page.
   - Select `Horizon-SDV`, select `SYNC`  and then `SYNCHRONIZE`
   - Wait for sync to complete.
5. **Android Workload**: this next step will prepare the Jenkins CI/CD system to support ABFS.
   - Open `Seed Workloads`
     - Review `ABFS_REPOSITORY`, `ABFS_VERSION` and `ABFS_CASFS_VERSION` match expectation. Refer to `aaos_abfs_builder.md` for additional details.
     - Select `SEED_WORKLOAD` `android` and `Build`
     - Wait for seed job to complete successfully.
   - Open `Android Workflows → Environment → ABFS → Docker Infra Image Template`
     - Deselect `NO_PUSH` and wait for image creation to complete.
   - Open `Android Workflows → Environment → ABFS → Docker Image Template`
     - Deselect `NO_PUSH` and wait for image creation to complete.
   - Open `Android Workflows → Environment → ABFS → Server`
     - Ensure `ABFS_TERRAFORM_ACTION` is `APPLY`
     - Select `Build` and wait for server VM instance to complete creation.
   - Open `Android Workflows → Environment → ABFS → Uploader`
     - Ensure `ABFS_TERRAFORM_ACTION` is `APPLY`
     - Select `Build` and wait for uploader VM instances to complete creation.
   - **Note**:
      - There are additional parameters, currently set to defaults, e.g. `UPLOADER_GIT_BRANCH` is set to seed `android-15.0.0_r36`
      - Refer to specific README files and parameter descriptions for additional details.
      - This task can take ~24 hours per branch/tag and the only way of knowing it is complete is to monitor the docker
        logs on the uploader instances to ensure all repositories have been seeded fully. Discuss with Google for details.

Users can now build from the ABFS seeded source/cache, see `Android Workflows → Builds → AAOS Builder ABFS`. Enable `ABFS_CACHED_BUILD` if wishing to store the cacheman cache and ABFS source mount path in persistent storage in order to improve build times.

## Known issues<a name="knownissues"></a>

ABFS is still evolving and there are limitations at this time. Please reach out to your Google representative for details.

**Important**: Regardless of build outcomes, the `abfs_repository_list.txt` file will be generated. This file is crucial for correlating `ABFS_VERSION` and `ABFS_CASFS_VERSION` with the build instance kernel revision.

**Action Required**: Please review the output of this file and update the `Seed Workload` values for ABFS versions accordingly. This ensures you utilize the latest versions provided by Google, as they are subject to updates. Should a compatible version not exist, reach out to Google to request a new build be created and hosted.
