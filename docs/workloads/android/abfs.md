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

> [!IMPORTANT]
> **ABFS infrastructure is no longer provisioned from Jenkins.** The Server Administration
> and Uploader Administration jobs, and the Docker Infra Image Template that built the
> Terraform image they used, have been removed. The ABFS Server, ABFS Uploaders
> and their GCP resources are now reconciled by the `abfs-service-module` through Config
> Connector and Helm, enabled from the Developer Portal.
>
> This page still describes the Jenkins-based flow of ABFS 0.0.33 in places. It is rewritten
> for the module-based architecture. Until then, see `gitops/modules/abfs-service-module/README.md`.

Jenkins keeps two ABFS responsibilities: building the ABFS-enabled AAOS builder image, and
running ABFS builds.

1. **Android Workload**: prepare the Jenkins CI/CD system to support ABFS.
   - Open `Seed Workloads`
     - Review `ABFS_REPOSITORY`, `ABFS_VERSION` and `ABFS_CASFS_VERSION` match expectation. Refer to `aaos_abfs_builder.md` for additional details.
     - Select `SEED_WORKLOAD` `android` and `Build`
     - Wait for seed job to complete successfully.
   - Open `Android Workflows → Environment → ABFS → Docker Image Template`
     - Deselect `NO_PUSH` and wait for image creation to complete.
2. **ABFS infrastructure**: enable the `abfs-service-module` from the Developer Portal. It
   requires a valid ABFS license; the license and the underlying Spanner database and storage
   bucket are owned by the `abfs-data-module`.
   - Seeding from Gerrit takes ~24 hours per branch or tag. Monitor the uploader pods to
     confirm all repositories have been seeded. Discuss with Google for details.

Users can then build from the ABFS seeded source/cache, see `Android Workflows → Builds → AAOS Builder ABFS`. Enable `ABFS_CACHED_BUILD` if wishing to store the cacheman cache and ABFS source mount path in persistent storage in order to improve build times.

## Known issues<a name="knownissues"></a>

ABFS is still evolving and there are limitations at this time. Please reach out to your Google representative for details.

**Important**: Regardless of build outcomes, the `abfs_repository_list.txt` file will be generated. This file is crucial for correlating `ABFS_VERSION` and `ABFS_CASFS_VERSION` with the build instance kernel revision.

**Action Required**: Please review the output of this file and update the `Seed Workload` values for ABFS versions accordingly. This ensures you utilize the latest versions provided by Google, as they are subject to updates. Should a compatible version not exist, reach out to Google to request a new build be created and hosted.
