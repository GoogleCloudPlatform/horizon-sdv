# Horizon SDV Release Notes

## Horizon SDV - Release 2.0.1 (2025-09-24) 

### Summary
Hot fix release for Rel.2.0.1 with emergency fix for Helm repo endpoint issues, and minor documentation updates.

### New Features
N/A

### Improved Features
- New simplified Release Notes format.

### Bug Fixes

|  ID       | Summary                                                      |
|-----------|--------------------------------------------------------------|
| TAA-1002  | [Jenkins] Install ansicolor plugin for CWS                   |
| TAA-1005  | Horizon provisioning failure - Due to outdated Helm install steps |
| TAA-1007  | Cloud WS - Workstation Image builds fail due to Helm Debian repo (OSS) migration |
| TAA-1040  | Remove references to private repo in Horizon files           |
| TAA-1045  | OSS Bitnami helm charts EOL       

***
## Horizon SDV  - Release 2.0.0 (2025-09-01) 

### Summary
Horizon SDV 2.0.0 extends Android build capabilities with the integration of Google ABFS and introduces support for Android 15. This release also adds support for OpenBSW, the first non-Android automotive software platform in Horizon. Other major enhancements include Google Cloud Workstations with access to browser based IDEs Code-OSS, Android Studio (AS), and Android Studio for Platforms (ASfP). In addition, Horizon 2.0.0 delivers multiple feature improvements over Rel. 1.1.0 along with critical bug fixes.

### New Features

| ID       | Feature                           | Description                                                                                                                                                                                                                  |
|----------|----------------------------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| TAA-8    | ABFS for Build Workloads          | The Horizon-SDV platform now integrates Google's Android Build Filesystem (ABFS), a filesystem and caching solution designed to accelerate AOSP source code checkouts and builds.                                           |
| TAA-9    | Cloud Workstation integration     | The Horizon-SDV platform now includes GCP Cloud Workstations, enabling users to launch pre-configured, and ready-to-use development environments directly in browser.                                                         |
| TAA-375  | Android 15 Support                | Horizon previously supported Android 15 in Horizon-SDV but by default Android 14 was selected. In this release, Android 15 android-15.0.0_r36 is now the default revision.                                                 |
| TAA-381  | Add OpenBSW build targets         | Eclipse Foundation OpenBSW Workload: As part of the R2.0.0 delivery, a new workload has been introduced to support the Eclipse Foundation OpenBSW within the Horizon SDV platform. This workload enables users to work on the OpenBSW stack for build and testing. |
| TAA-915  | Cloud Android Orchestration - Pt.1| In R2.0.0 Horizon platform introduces significant improvements to Cuttlefish Virtual Devices (CVD). These enhancements include increased support for a larger number of devices, optimized device startup processes, a more robust recovery mechanism, and updated CTS Test Plans and Modules to ensure seamless integration and compatibility with CVD. |
| TAA-623  | Management of Jenkins Jobs using CasC | The CasC configuration has been updated to include a single job in the jenkins.yaml file, automatically started on each Jenkins restart. This job provides the "Build with Parameters" option for users. |
| TAA-462  | Kubernetes Dashboard              | The Horizon platform now includes the Headlamp application, a web-based tool to browse Kubernetes resources and diagnose problems.                                                                                            |
| TAA-717  | Multiple pre-warmed disk pools    | Horizon is changing to persistent volume storage for build caches to improve build times, cost, and efficiency. Pools are separated by Android major version and Raspberry Vanilla targets now have their own smaller pools. |
| TAA-596  | Jenkins RBAC                      | Jenkins has been configured with RBAC capability using the Role-based Authorization Strategy plugin.                                                                                                                        |
| TAA-611  | Argo CD SSO                       | Argo CD has been configured with SSO capabilities. Users can login either with admin credentials or via Keycloak.                                                                                                           |
| TAA-837  | Access Control tool               | Additional Access Control functionality provides a Python script tool and classes for managing user and access control on GCP level.    

### Improved Features
N/A

### Bug Fixes

|  ID      | Summary |
|----------|---------|
| TAA-980  | Access control issue: Workstation User Operations succeed for non-owned workstations                             |
| TAA-984  | [Kaniko] Increase CPU resource limits                                                                            |
| TAA-982  | [ABFS] Uploaders not seeding new branch/tag correctly                                                            |
| TAA-981  | [ABFS] CASFS kernel module update required (6.8.0-1027-gke)                                                      |
| TAA-977  | New Cloud Workstation configuration is created successfully, but user details are not added to the configuration |
| TAA-974  | kube-state-metrics Service Account missing causes StatefulSet pod creation failure                               |
| TAA-968  | [IAA] Elektrobit patches remain in PV and break gerrit0                                                          |
| TAA-966  | [ABFS] Kaniko out of memory                                                                                      |
| TAA-953  | Android CF/CTS: update revisions                                                                                 |
| TAA-964  | [Gerrit] Propagate seed values                                                                                   |
| TAA-959  | Reduce number of GCE CF VMs on startup                                                                           |
| TAA-932  | ABFS_LICENSE_B64 not propagated to k8s secrets correctly                                                         |
| TAA-958  | [Gerrit] repo sync - ensure we reset local changes before fetch                                                  |
| TAA-781  | GitHub environment secrets do not update when Terraform workload is executed                                     |
| TAA-933  | Failure to access ABFS artifact repository                                                                       |
| TAA-905  | AAOS build does not work with ABFS                                                                               |
| TAA-931  | Create common storage script                                                                                     |
| TAA-930  | Investigate build issues when using MTK Connect as HOST                                                          |
| TAA-923  | Cuttlefish limited to 10 devices                                                                                 |
| TAA-921  | [Cuttlefish] Building android-cuttlefish failing on The GNU Operating System and the Free Software Movement      |
| TAA-922  | MTK Connect device creation assumes sequential adb ports                                                         |
| TAA-920  | Android Developer Build and Test instances leave MTK Connect testbenches in place when aborted                   |
| TAA-563  | [Jenkins] Replace gsutils with gcloud storage                                                                    |
| TAA-886  | Conflict Between Role Strategy Plugin and Authorize Project Plugin                                               |
| TAA-814  | Android RPi builds failing: requires MESON update                                                                |
| TAA-863  | Workloads Guide: updates for R2.0.0                                                                              |
| TAA-867  | Gerrit triggers plugin deprecated                                                                                |
| TAA-890  | Persistent Storage Audit: Internal tool removal                                                                  |
| TAA-618  | MTK Connect access control for Cuttlefish Devices                                                                |
| TAA-711  | [Qwiklabs][Jenkins] GCE limits - VM instances blocked                                                            |

***
## Horizon SDV - Release 1.1.0 (2025-04-14)   

### Summary
Minor improvements in Jenkins configuration, additional pipelines implemented for massive build cache pre-warming simplification required for Hackathon and Gerrit post jobs cleanup.

### New Features

| ID       | Feature                   | Description                                                                                   |
|----------|---------------------------|-----------------------------------------------------------------------------------------------|
| TAA-431  | Jenkins R1 deployment extensions | Jenkins extensions to Platform Foundation deployment in Rel.1.0.0. Includes new job to pre-warm build volumes. |
| TAA-346  | Support Pixel devices     | Support for Google Pixel tablet hardware, full integration with MTK Connect.                  |

### Improved Features
N/A

### Bug Fixes

|   ID     | Summary                                                                                  |
|----------|------------------------------------------------------------------------------------------|
| TAA-683  | Change MTK Connect application version to 1.8.0 in helm chart                            |
| TAA-644  | self-hosted runners                                                                      |
| TAA-641  | [Jenkins] Horizon Gerrit URL path breaks upstream Gerrit FETCH                           |
| TAA-639  | Keycloak Sign-in Failure: Non-Admin Users Stuck on Loading Screen                        |
| TAA-631  | MTK Connect license file in wrong location                                               |
| TAA-628  | [Jenkins] CF instance creation (connection loss)                                         |
| TAA-627  | [Jenkins][Dev] Investigate build nodes not scaling past 13                               |
| TAA-622  | Workloads documentation - wrong paths                                                    |
| TAA-615  | Improve the Gerrit post job                                                              |
| TAA-401  | [Jenkins] Agent losing connection to instance                                            |
| TAA-309  | [Jenkins] 'Build Now' post restart    

***
## Horizon SDV - Release 1.0.0 (2025-03-18)    

### Summary
The main objective for Release 1.0.0 is to achieve Minimal Viable Product level for Horizon SDV platform where orchestration will be done using Terraform on GCP with the intention of deploying the tooling on the platform using a simple provisioner. Horizon SDV platform in Rel.1.0.0 supports:

- GCP platform / services.
- Terraform orchestration (IaC).
- IaC stored in GitHub repo and provisioned either via CLI or GitHub actions.
- Platform supports Gerrit to host Android (AAOS) repos and manifests, and allows users to create their own repos.
    - With some pre-submit checks: e.g., voting labels: code review and manual vs automated triggered builds.
    - Will mirror and fork AAOSP manifests repo, and one additional code repo for demonstrating the SDV Tooling pipeline. Locally mirrored/forked manifest will be updated to point to the internally mirrored code repo, all other repos will remain using the external OSS AAOS repos hosted by Google.
- Platform supports Jenkins to allow for concurrent, multiple builds for iterative builds from changes in open review in Gerrit, full builds (manually, when user requests) and CTS testing.
- Platform supports an artefact registry to hold all build artefacts and test results.
- Platform supports a means to run CTS tests and use the Accenture MTK Connect solution for UI/UX testing.

### New Features

| ID       | Feature                   | Description                                                                                   |
|----------|---------------------------|-----------------------------------------------------------------------------------------------|
| TAA-6    | Platform foundation       | Platform foundation including support for: GCP, Terraform workflow, Stage 1 and Stage 2 deployment with ArgoCD, Jenkins Orchestration and Authentication support through Keycloak. |
| TAA-12   | Github Setup              | Github support for Horizon SDV platform repositories.                                         |
| TAA-67   | Tooling for tooling       | Android build pipelines support.                                                              |
| TAA-5    | Gerrit                    | Gerrit support.                                                                               |
| TAA-61   | MTK Connect               | Test connections to CVD with MTK Connect support.                                             |
| TAA-2    | Android Virtual Devices   | Pipelines for Android Virtual Devices CVD and AVD.                                            |

### Improved Features
N/A

### Bug Fixes

|   ID     | Summary                                                                                  |
|----------|------------------------------------------------------------------------------------------|
| TAA-608  | MTK Connect - testbench registration failing                                             |
| TAA-593  | [Jenkins] Jenkins config auto reload affecting builds                                    |
| TAA-590  | [Jenkins] CTS_DOWNLOAD_URL : strip trailing slashes                                      |
| TAA-589  | [Jenkins] computeEngine: cuttlefish-vm-v110 points to incorrect instance template        |
| TAA-577  | [Jenkins] CF CVD launcher fails to boot devices                                          |
| TAA-562  | [Jenkins] Warnings from pipeline (Pipeline Groovy)                                       |
| TAA-532  | [Jenkins] Stage View bug (display pipeline)                                              |
| TAA-530  | [Jenkins] Regression: Exceptions raised on connection/instance loss                      |
| TAA-528  | [MTK Connect] node warnings: MaxListenersExceededWarning                                 |
| TAA-520  | [Jenkins] Reinstate cuttlefish-vm termination                                            |
| TAA-519  | TAA-518[Jenkins] Reinstate MTKC Test bench deletion env pipeline                         |
| TAA-518  | [Jenkins] Reinstate MTKC Test bench deletion env pipeline                                |
| TAA-518  | [Jenkins] CVD / CTS - hudson exceptions reported and jobs fail                           |
| TAA-516  | [Jenkins] Make test jobs more defensive + improvements                                   |
| TAA-508  | [MTK Connect] Not terminating                                                            |
| TAA-507  | [Jenkins] CVD/CTS test run: times out on android-14.0.0_r74                              |
| TAA-502  | Re-apply pull-request trigger to GitHub workflows                                        |
| TAA-501  | Invent a solution for restricting GitHub workflows to a given branch                     |
| TAA-498  | Gerrit-admin password is not created in Keycloak                                         |
| TAA-496  | [Android Studio] Arm builds throw an error due to config                                 |
| TAA-490  | [RPi] RPi4 again broken                                                                  |
| TAA-478  | [Jenkins] CLEAN_ALL: rsync errors                                                        |
| TAA-477  | [Gerrit] Branch name revision incorrect for 15 - build failures                          |
| TAA-425  | [Jenkins] Native Linux install of MTKC fails (unattended-upgr)                           |
| TAA-412  | [Jenkins] Russian Roulette with cache instance causing build failures                    |
| TAA-400  | [Jenkins] SSH issues                                                                     |
| TAA-398  | [Jenkins] GCE plugin losing connection with VM instance                                  |
| TAA-394  | [Gerrit] Admin password stored in secrets with newline                                   |
| TAA-354  | [Jenkins] CVD adb devices not always working as expected                                 |

***

