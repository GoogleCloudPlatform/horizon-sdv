# Horizon SDV Release Notes

## Horizon SDV - Release 3.0.0 (2025-12-19) 

### Summary

Horizon SDV 3.0.0 extends platform capabilities with support for Android 15 and the latest extensions of OpenBSW. Horizon 3.0.0 also delivers multiple new feature and several improvements over Rel. 2.0.1 along with critical bug fixes.

The set of new features in version 3.0.0 includes, among others:

- **Simplified Deployment Flow :** We have overhauled the deployment process to make it more intuitive and efficient. The new flow reduces complexity, minimizing the steps required to get your environment up and running.

- **ARM64 Support (Bare Metal) :** We have expanded our infrastructure support to include ARM64 Bare Metal. This allows you to run your workloads natively on ARM architecture, ensuring higher performance and closer parity with automotive edge hardware.

- **Gemini Code Assist :** Supercharge your development with the integration of **Gemini Code Assist** and the Gerrit MCP Server. You can now leverage Google's state-of-the-art AI to generate code, explain complex logic, debug issues faster and make use of agentic code review workflows directly within your development environment.

- **Advanced Monitoring with Grafana :** Gain deeper insights into your infrastructure with our new Grafana integration. You can now visualize and monitor POD and Instance metrics in real-time, helping you optimize resource usage and diagnose performance bottlenecks quickly.

***
### New Features

| ID | Feature | Description |
|----|--------|-------------|
| TAA-924 | Simplified Horizon Deployment Flow | Simplified and automated the Horizon SDV platform deployment by removing GitHub Actions, enabling faster adoption by community teams and reducing human error. |
| TAA-511 | Gemini Code Assist in R3 – Gerrit MCP Server integration | Use company’s codebase as a knowledge base for Gemini Code Assist within the IDE to receive code suggestions & explanations tailored to known codebase, libraries and corporate standards. |
| TAA-365 | ARM64 GCP VM (Bare Metal) support for Cuttlefish | ARM64 GCP VM support for Android builds and testing with Cuttlefish |
| TAA-595 | Monitoring of POD/Instance metrics with Grafana | Access to CPU/Memory/Storage metrics for pods and instances, to more easily investigate and debug container, pod and instance related problems and its impact on platform performance. |
| TAA-944 | Android pipeline update to Android 16 | Support for Android16 for AAOS, CF and CTS in Horizon pipelines. |
| TAA-946 | Extend OpenBSW support with additional features | Support for Eclipse Foundation OpenBSW workload features that were not included in Horizon-SDV R2.0.0 |
| TAA-889 | Horizon R3 Security update | Selected open-source applications and tools which are part of Horizon SDV platform are updated to the latest stable versions |
| TAA-377 | Google AOSP Repo Mirroring | NFS based mirror of AOSP repos deployed in the K8s cluster. |
| TAA-947 | ABFS update for R3 | Corrections and minor ABFS updates delivered from Google in Release 3.0.0 timeframe. |
| TAA-1072 | Cloud Artefact storage management | Android and OpenBSW build jobs have been modified to allow the user to specify metadata to be added to the stored artifacts during the upload process. Implementation is supported for GCP storage option only |
| TAA-1001 | Kubernetes Dashboard SSO integration | Kubernetes Dashboard SSO integration |
| TAA-945 | Replace deprecated Kaniko tool | Replace deprecated Google Kaniko tool for building container images with new Buildkit tool. |
| TAA-941 | IAA demo case. | Support for Partner demo in IAA Messe show. The main technical scope is to apply a binary APK file to the Android code, help building it and flash it to selected targets (Cuttlefish and potentially Pixel) according to Partner specification. |

***

### Improved Features

See details in `horizon-sdv/docs/release-notes-3-0-0.md`

| ID | Summary |
|----|-------------|
| TAA-1171 | Create Workloads area in Gitops section |
| TAA-862 | Improvements Structure of Test pipelines |
| TAA-1111 | Unified CTS Build process |
| TAA-1265 | [Gerrit] Support GERRIT_TOPIC with existing gerrit-triggers plugin |
| TAA-1271 | Support custom machine types for Cuttlefish |
| TAA-1269 | Adjust CTS/CVD options |

***

### Bug Fixes

| ID        | Summary |
|-----------|-------------|
| TAA-993   | [ABFS] Missing permission for jenkins-sa for ABFS server |
| TAA-1063  | [Security] Axios Security update 1.12.0 (dependabot) |
| TAA-904   | ABFS unmount doesn't work |
| TAA-1090  | [Android 16] Cuttlefish builds fail (x86/arm) |
| TAA-1080  | [OpenBSW] Builds no longer functional (main) |
| TAA-1110  | [OpenBSW] pyTest failure |
| TAA-1103  | [Android 16] CTS 16_r2 reports 15_r5 |
| TAA-1145  | Update filter (gcloud compute instance-templates list) |
| TAA-1161  | [ARM64] Subnet working utils too quiet |
| TAA-1113  | [ABFS] COS Images no longer available |
| TAA-1118  | [ABFS] CASFS kernel module update required (6.8.0-1029-gke) |
| TAA-1176  | [CF] CTS CtsDeqpTestCases execution on main not completing in reasonable time (x86) |
| TAA-1186  | Incorrect Headlamp Token Injector Argo CD App Project |
| TAA-1196  | AOSP Mirror changes break standard builds |
| TAA-1201  | AOSP Mirror sync failures |
| TAA-1200  | AOSP Mirror URLs and branches incorrect |
| TAA-1203  | AOSP Mirror repo sync failing on HTTP 429 (rate limits) |
| TAA-1205  | AOSP Mirror - no support for dev build instance |
| TAA-1198  | AOSP Mirror does not support Warm nor Gerrit Builds |
| TAA-1204  | AOSP Mirror repo sync failing - SyncFailFastError |
| TAA-1214  | AOSP Mirror ab is an |
| TAA-1219  | [Cuttlefish] Host installer failures masked |
| TAA-1202  | AOSP Mirror blocking concurrent jobs incorrectly configured |
| TAA-1238  | [Cuttlefish] Update to v1.31.0 - v1.30.0 has changed from stable to unstable. |
| TAA-1241  | [Android] Mirror should not be using OpenBSW nodes for jobs AM |
| TAA-1247  | [Workloads] Remove chmod and use git executable bit |
| TAA-1249  | [GCP] Client Secret now masked (security clarification) |
| TAA-1264  | [CVD] Logs are no longer being archived |
| TAA-1261  | [Cuttlefish] gnu.org down blocking builds |
| TAA-1266  | Pipeline does not fail when IMAGE_TAG is empty and NO_PUSH=true |
| TAA-1267  | [CWS] OSS Workstation blocking regex incorrect (non-blocking) |
| TAA-1258  | [Cuttlefish] VM instance template default disk too small. |
| TAA-1233  | [Jenkins] Plugin updates for fixes |
| TAA-1278  | [Cuttlefish] SSH/SCP errors on VM instance creation |
| TAA-1283  | Mismatch in githubApp secrets (TAA-1054) |
| TAA-1277  | [Jenkins] Plugin updates for fixes |
| TAA-1279  | [RPI] Android 16 RPI builds now failing |
| TAA-1282  | [GCP] Cluster deletion not removing load balancers |
| TAA-1257  | [Cuttlefish] android-cuttlefish build failure (regression) |
| TAA-1273  | [Cuttlefish] android-cuttlefish CVD device issues (regression) |
| TAA-1149  | [K8S] Reduce parallel jobs to reduce costs |
| TAA-1162  | [K8S] Revert parallel jobs change to reduce costs |
| TAA-1191  | Monitoring deployment related hotfixes |
| TAA-1114  | [ABFS] Update env/dev license (Oct'25) |
| TAA-1116  | [Android] Android 15 and 16 AVD missing SPDX BOM |
| TAA-1192  | [MTKC] Support additional hosts for dev and test instances |
| TAA-1207  | Mirror/Create-Mirror: Add parameter for size of the mirror NFS PVC |
| TAA-1208  | Mirror/Sync-Mirror: Sync all mirrors when SYNC_ALL_EXISTING_MIRRORS is selected |
| TAA-1211  | [Android] Simplify Dev Build instance job |
| TAA-1218  | [Grafana] ArgoCD on Dev shows 'Out Of Sync' |
| TAA-1231  | R2 - GitHub Actions workflow fails |
| TAA-1038  | [Jenkins] CF scripts - update to retain color |
| TAA-907   | Multibranch is not supported in ABFS |
| TAA-862   | Improvement to structure of Test pipelines |
| TAA-788   | Jenkins AAOS Build failure - Gerrit secrets/tokens mismatch |
| TAA-1088  | [NPM] Move wait-on post node install |
| TAA-1115  | [STORAGE] Override default paths |
| TAA-1160  | [ARM64] Lack of available instances on us-central1-b/f zone |
| TAA-1274  | [Cuttlefish] CTS hangs - android-cuttlefish issues |
| TAA-1290  | [Cuttlefish] ARM64 builds broken on f2fs-tools (missing) |
| TAA-1253  | [MTK Connect] ERROR: script returned exit code 92/1 |

***
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
