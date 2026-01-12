# Release Notes - SDV Tooling - Release 3.0.0

| | |
|-|-|
| __Platform__ | Horizon SDV |
| __Date__ | 19.12.2025 |
| __Version__ | Release 3.0.0 |
| __Contributors__ | @Wojtek Kowalski @Wojciech Kobryn @Prashanth Habib Eshwar @Bavya Sakthivel @Lynn Sheehy @Dave M. Smith @Lukasz Domke @Adireddi Keerthi @Colm Murphy @Akshay Kaura |

## Summary

Horizon SDV 3.0.0 extends platform capabilities with support for Android 15 and the latest extensions of OpenBSW. Horizon 3.0.0 also delivers multiple new feature and several improvements over Rel. 2.0.1 along with critical bug fixes.

## New Features

|Issue ID | Summary |
|----|--------|
| TAA-924 | Simplified Horizon Deployment Flow | 
| TAA-511 | Gemini Code Assist in R3 – Gerrit MCP Server integration | 
| TAA-365 | ARM64 GCP VM (Bare Metal) support for Cuttlefish |
| TAA-595 | Monitoring of POD/Instance metrics with Grafana | 
| TAA-944 | Android pipeline update to Android 16 |
| TAA-946 | Extend OpenBSW support with additional features |
| TAA-889 | Horizon R3 Security update |
| TAA-377 | Google AOSP Repo Mirroring | 
| TAA-947 | ABFS update for R3 | 
| TAA-1072 | Cloud Artifact storage management |
| TAA-1001 | Kubernetes Dashboard SSO integration |
| TAA-945 | Replace deprecated Kaniko tool |
| TAA-941 | IAA demo case. | 

---

### TAA-924 | Simplified Horizon Deployment Flow

#### Release Note

Deployment flow for Horizon platform has been updated and simplified. The new deployment procedure does not require GitHub Actions; it can be run on local machine or Google Cloud Shell.

#### Changes

- Bastion host has been removed, connection to the GKE cluster is now enabled via GKE Connect Gateway using Fleet.
- Removed Workload Identity Federation and its required GCP Service Account.
- Add External DNS deployment to manager DNS records for the Horizon Platform.
- New Terraform modules
   - `sdv-container-images`: Build and Push container images to GCP Artifact Registry.
   - `sdv-dns-zone`: Create Cloud DNS Zone.
   - `sdv-gke-apps`: Deploy and configure required Kubernetes resources and apps.
   - `sdv-parameters`: Create parameters with non-sensitive values using GCP parameter manager
   - `sdv-ssh-keypair`: Generate required SSH keys for Gerrit and Cuttlefish VM.
- Credentials for most Apps can now be auto generated.
- Branch and environment name are no longer dependent on each other.
- Terraform variables and secrets are placed either in GCP Secret Manager or Parameter Manager based on sensitivity of the value.
- Convert required parameters to base64 encoded values
- Remove GitHub Actions and related documentation.
- Added Deployment script (container based and native)
- Updated deployment guide along with GitOps and Terraform documentations.
- Under `gitops/env`, `stage1` and `stage2` have been removed. The contents of `stage2` have been moved directly under `gitops/`
- Contents of `gitops/env/stage2/configs` have been moved to `terraform/modules/sdv-container-images/images`

#### Action Required

As GitHub environment secrets or variables are not being used in the updated simplified deployment, it is required to create a new `terraform.tfvars` from the given `terraform.tfvars.sample` file and update it with actual values. The file is located at the path: `terraform/env/terraform.tfvars.sample`

---

### TAA-511 | Gemini Code Assist in R3 – Gerrit MCP Server integration

#### Release Note

#### Gerrit MCP Server

This MCP (Model Context Protocol) server has been added to the Horizon platform. is used to facilitate communication between AI tools and the Gerrit code review system. It provides a standardized API interface for AI tools to perform operations such as code reviews, submissions, and other interactions with Gerrit.

#### Changes

- Gerrit MCP server container image is built from [source code](https://gerrit.googlesource.com/gerrit-mcp-server) during platform infra deployment (Terraform).
- Then deployed as an app to platform using gitops (ArgoCD).

Note that Gerrit MCP Server depends on Gerrit being installed and configured. Also, since it does not have any authentication mechanism of its own, it relies on MCP Gateway Registry's authentication and authorization system to control access.

#### Action Required

In order to add this new app to your platform, pull changes into the branch/environment of your choice, and then

- Run Terraform workflow and wait for it to finish successfully.
- Login to Argo CD and sync the `horizon-sdv` app.

#### MCP Gateway Registry

This [app](https://github.com/agentic-community/mcp-gateway-registry) has been added to the Horizon platform. It is a centralized application for managing, monitoring and authenticating to MCP (Model Context Protocol) servers and agents deployed in Horizon platform or other environments.

#### Changes

- The app is deployed at its own subdomain: `https://mcp.<SUB_DOMAIN>.<HORIZON_DOMIAN>.COM` using official prebuilt [container images](https://hub.docker.com/u/mcpgateway) with a custom helm chart, created for Horizon SDV project.
- To enable authentication via Keycloak, the `keycloak-post-mcp-gateway-registry` post-job is added, which creates and configures the necessary clients, admin user, groups, and client mappers in Keycloak. It also generates and updates the required client secrets in Kubernetes for secure communication.
- By default, a single MCP server `gerrit-mcp-server` is pre-registered which is running in Horizon platform.
- No agent has been pre-registered in this release.

#### Action Required

In order to add this new app to your platform, pull changes into the branch/environment of your choice, and then

- Run Terraform workflow and wait for it to finish successfully.
- Login to Argo CD and sync the `horizon-sdv` app.
- Wait for `keycloak-post-mcp-gateway-registry` post-job and `mcp-gateway` app sync to complete.
- To enable Keycloak SSO for a user of choice, login to Keycloak and add the user to `horizon-mcp-gateway-registry-admins` group.
- To start using the pre-registered `gerrit-mcp-server`, make sure that it is ENABLED on the app (bottom right toggle button in server card). By default, newly registered MCP servers are disabled.

Refer to `docs/guides/mcp_setup.md` for other setup details.

#### Updates to Cloud Workstation images

#### Changes

All three Cloud Workstation Images have been upgraded to offer pre-installed `gemini-cli` and `Gemini Code Assist`  in IDE.

#### Action Required

In order to get started with Gemini in cloud workstations, pull changes into the branch/environment of your choice, and then run the following Jenkins pipelines

- Cloud Workstations > Workstation Images > Build all three images
- Cloud Workstations > Config Admin Pipelines > Create Configurations using the built images
- Cloud Workstations > Workstation Admin Pipelines > Create Workstations using the created configs and add your user
- Cloud Workstations > Workstation User Pipelines > Start Workstation

Refer to `docs/guides/mcp_setup.md` for details on how to setup and use MCP servers in cloud workstations.

#### MCP server's config setup automation script for Gemini-CLI

#### Changes

- A python script named `gemini-mcp-setup` has been added as an executable package in all three Cloud Workstation Images below

  - horizon-code-oss (VS Code)
  - horizon-android-studio (Android Studio)
  - horizon-asfp (Android Studio for Platform)

- The script facilitates automatic creation and update of the MCP servers config file `~/.gemini/settings.json`used by Gemini-CLI to sync its JWT token and MCP servers list from MCP Gateway registry.

- The script is compatible with any Linux or Windows based environment.

#### Action Required

In order to start using MCP servers with Gemini-CLI in any workstation image mentioned above, pull changes into the branch/environment of your choice, and then

- Make sure you have built the latest workstation image and configuration prior to workstation creation.
- Open Terminal and run `gemini-mcp-setup` script.

In order to run this script on a local environment, make sure you set the `HORIZON_DOMAIN` environment variable before executing script. (e.g. `demo.horizon-sdv.com`)

Refer to `docs/guides/mcp_setup.md` for details on how to setup and use MCP servers in cloud workstations.

#### Known Issues

#### MCP Gateway Registry

- MCP server paths during registration must have trailing slashes on both ends `/my-mcp-server/`
- Server is disabled by default on new registration, including the pre-registered `gerrit-mcp-server`

#### Gemini Code Assist MCP Servers setup in Android Studio and Android Studio for Platform

- The `gemini-mcp-setup` script does NOT configure the `mcp.json`config file used by Android Studio IDE right now, because in order to load refreshed or synced version of `mcp.json` file, the IDE must be restarted.
- Hence, it is recommended to increase the token expiry time in Keycloak in order to avoid intermittent MCP auth issues.
- Refer to `docs/guides/mcp_setup.md` for details on how to setup and use MCP servers with Gemini Code assist in cloud workstations.

---

### TAA-365 | ARM64 GCP VM (Bare Metal) support for Cuttlefish

#### Release Note

#### ARM64 GCP VM support for Cuttlefish

In this release, Google have provided preview access to ARM64 bare metal instances for use with ARM64 Android Cuttlefish virtual devices.

The machine type is limited to `us-central1` region and platforms (project id) must be approved by Google for preview access. At a later time, these instances may be made available across more regions and zones.

Due to the current region restriction, tooling has been provided to aid in creation of a second subnetwork for the platform should the existing network and subnetwork not be within `us-central1`. Current regions the instances are supported from are `us-central1-b` and `us-central1-f`.

#### Changes

- `jenkins.yaml`
  - `gitops/env/stage2/terraform/jenkins.yaml`
  - Additional computeEngine entries for ARM64 bare metal instances.

  > **Note** : 
  Due to preview availability, the region is set to `us-central1-b` however sometimes availability is limited so `us-central1-f` may be used instead.

    - Users may modify the region directly in the YAML file or directly from within Jenkins, e.g:  
`Manage Jenkins → Clouds → gce-cuttlefish-vm-main-arm64 → Configure → Zone → us-central1-f`

- `Android → Environment → Cuttlefish Instance Template`
    - Legacy support for creating x86_64 cuttlefish VM instances for use with x86_64 Cuttlefish devices.
    - Additional flexibility added through more parameterization, e.g. users may select the versions of CTS they may wish to install on the instance.

- `Android → Environment → Cuttlefish Instance Templates ARM64`
    - Support for creating ARM64 cuttlefish VM instances for use with ARM64 Cuttlefish devices.
    - Uses the same Jenkinsfile, scripts as the legacy X86 version but is separated for convenience to clearly identify X86 configuration vs ARM64.
    - Networking parameters must match those set up in `Utilities → Networking → Subnetworking Operation`

- `Android → Environment → Development Test Instance`
    - Support to connect to an ARM64 test instance.

- `Android → Tests → CVD Launcher` 
    - Additional support for ARM64 Cuttlefish devices on the ARM64 VM instances.

- `Android → Tests → CTS Execution`
    - Additional support to test ARM64 Cuttlefish devices on the ARM64 VM instances.
    - Additional parameters added to modify retry strategy.

#### Action Required 

To ensure a smooth transition it is advisable users update their build image and cuttlefish instance templates.

#### Subnetworking update

Update networking to support subnet and NAT for us-central1 in order to gain access to ARM64 metal instances.

ARM64 enablement is controlled through a GitHub environment variable:

> FEATURES_BARE_METAL_ARM64

If flag is **true** → ARM64 networking is created  
If **false** or **missing** → ARM64 networking will be disabled, had it been enabled previously.

The variables that are used in creation of the additional subnet and NAT are defined in `base/variables.tf`, e.g.

```
variable "enable_arm64"           { type = bool   default = false }  
variable "arm64_region"           { type = string default = "us-central1" }  
variable "arm64_subnetwork"       { type = string default = "sdv-subnet-us" }  
variable "arm64_pods_range"       { type = string default = "10.20.0.0/16" }  
variable "arm64_services_range"   { type = string default = "10.22.0.0/16" }
```

#### Known Issues 

- **ARM64 CTS JDK compatibility issues**

```
TAA-1167: ARM64 CTS workaround  

ARM CTS packages contain the wrong Java architecture: they ship only x86_64 binaries instead of ARM64-compatible ones. As a temporary workaround to prevent CTS failures, the system Java will be upgraded to version 21 for compatibility. Google must correct their CTS deliveries so they include ARM64 binaries that match their tests.
```

  - ARM64 instances are not suited to Android 14 CTS tests.

  - Official stance is that ARM64 CTS is not yet supported and requires users to tunnel between CTS running on an x86 host to CF device running on ARM64 metal.

    - The fact that CTS works on ARM64 instances is not an official support stance but that may change in future releases.

    - This is not in scope of Horizon-SDV support. CTS is supported to a degree when correct JDK version is provided on the ARM64 system.

---

### TAA-595 | Monitoring of POD/Instance metrics with Grafana

#### Release Note

- Added key components for pod and instance monitoring, e.g. **Grafana**, **prometheus-ui**, **node-exporter**, and **kube-state-metrics**
- Configured imporing Grafana dashboards `(IDs: 1860, 15661, 11848)` and authentication & authorization via Keycloak. Updated Terraform for Cloud Monitoring features and documented IAM bindings.
- Configuration of Collector and OperatorConfig for metric collection.

---
### TAA-944 | Android pipeline update to Android 16

#### Release Note

#### Android 16 Support

In this release, Android 16 `android-16.0.0_r4` is now the default revision.

#### Changes

- **AAOS Builder:** default `AAOS_REVISION` moved to `horizon/android-16.0.0_r4`
  - Raspberry Vanilla updates: support for `android-16.0.0_r4`, `android-15.0.0_r36` and `android-14.0.0_r30`, including both RPi4 and RPi5 hardware.
- **CTS Builder:** default `AAOS_REVISION` moved to `horizon/android-16.0.0_r4`
- **Gerrit and Warm Build Caches:** now support `android-16.0.0_r4` to dynamically determine the codename/build tag for the target to build, e.g. `aosp_cf_x86_64_auto-bp4a-userdebug`
- **Development Instance:** defaulted to Android 16 disk pool, user may override.
- **Cuttlefish Instance Templates:** debian upgrade, Cuttlefish host package updates and updated CTS test harness for Android 16, 15 and 14, and new parameter:
  - `ANDROID_CUTTLEFISH_POST_COMMAND`
    - Workaround android-cuttlefish.git issues caused by Google or third-party dependencies.
    - Use git commands to switch to a specific sha1 when using branches (main), or cherry-pick workarounds/fixes to tagged versions which cannot be modified by google. e.g.    
       - `git cherry-pick <sha1>`
       - `sed -i 's|https://git.kernel.org/pub/scm/linux/kernel/git/jaegeuk/f2fs-tools|https://github.com/jaegeuk/f2fs-tools|g' base/cvd/MODULE.bazel`
- **Docker Image Template:** packages upgrades for Android 16.
- **Jenkins Build Artifacts:** zero padded Jenkins build numbers in artifact storage, so build 1 will be 01, build 9 will be 09. This to aid in ordering lists.
    - This is important for test and other jobs that reference artifacts. The jobs Jenkins artifact stored with the jobs reflects the change to using zero padding.

#### Action Required

To ensure a smooth transition it is advisable users update their build image and cuttlefish instance templates.

#### Docker Image Template update

In Jenkins select Android Workflows → Environment → Docker Image Template:

Deselect `NO_PUSH`

Select `Build`

#### Cuttlefish Instance Template update

In Jenkins select Android Workflows → Environment → CF Instance Template:

Delete the old `v1.18.0` instance template:

  Set `ANDROID_CUTTLEFISH_REVISION` to `v1.18.0`  
  Select `DELETE`  
  Select `Build`   

- R3.0.0 no longer references this version and as such, this will delete the instance template and disk image for v1.18.0 and save on cost.

Upgrade the `main` instance template:

Set `ANDROID_CUTTLEFISH_REVISION` to `main`  
Select `Build` 

Create the new `v1.35.0` instance template:

Set `ANDROID_CUTTLEFISH_REVISION` to `v1.35.0`  
Select `Build`  

> **Note** :  Refer to `ANDROID_CUTTLEFISH_POST_COMMAND` should you need to workaround android-cuttlefish.git build issues.

Your platform is now updated for latest versions of build images and instance templates to support Android 16.

---

### TAA-946 | Extend OpenBSW support with additional features

#### Release Note

OpenBSW has been updated to incorporate the latest features and tooling enhancements:

- **Dockerized build environment:** Docker container image updates for additional tools, packages and build support.
- **Ethernet support:** Enabled for the POSIX reference application.
- **Virtual CAN support:** Enabled for the POSIX reference application.
- **Documentation generation:** Added automated docs support.
- **Test infrastructure:** Added pytest support for the POSIX reference application.
- **FreeRTOS and ThreadX:** support for Eclipse ThreadX added.

#### Actions Required

Run TF workflow to apply updates to OpenBSW host instance machine type, to improve container creation and build times.

Reseed the OpenBSW Workload as follows:

- Seed Update: `Seed Workloads → Build With Parameters → SEED_WORKLOAD=none`

  - Pick up latest parameter changes

- Seed OpenBSW: `Seed Workloads → Build With Parameters → SEED_WORKLOAD=openbsw`

  - Update the OpenBSW parameters

Rebuild the Docker Image Template:

`OpenBSW Workflow → Environment → Docker Image Template → Build With Parameters → NO_PUSH=false`

  - Docker has been extensively changed to support the tip OpenBSW main branch and it’s new build mechanism (use of presets).

  - Ensure this is built before running the BSW Build and Test jobs.

#### Change Summary

`OpenBSW Workflows → Environment → Docker Image Template updates:`
- Tools and python updated.

`OpenBSW Workflows → Builds → BSW Builder updates:`
- Added new stage to allow pyTest to be run using the POSIX application.
- Artifacts are uploaded to bucket storage for OpenBSW → Tests → POSIX to reference.

`BUILD_DOCUMENTATION:`
- Documentation created using doxygen and uploaded to GCS bucket.
- `coverxygen` is used to provide documentation coverage report summary.
- Documentation archive is also stored as an artifact with the job and also HTML using `publishHTML` plugin, e.g.
  
| | |
|-------|-------|
| ![openbsw-bsw-builder-artifacts](/docs/images/openbsw_bsw_builder_doc_artifacts.png) | • Refer to `bsw_builder.md` for content security considerations if wishing to view the full documentation from within Jenkins, otherwise download the archive and view locally.   ![Eclipse-OpenBSW-preview](/docs/images/eclipse_openbsw_preview.png) |

`OpenBSW Workflows → Tests → POSIX` updates:

- Removed `LAUNCH_APPLICATION_NAME` because the user will now manually launch the POSIX reference application, or run pyTest against the application. This is beneficial because it keeps the HOST session created from MTK Connect alive.

- Use new parameter `NUM_HOST_INSTANCES` to set how many device sessions to create. These sessions appear under the testbench in the MTK Connect application. They all attach to the same host instance but run independently, allowing you to use separate shell sessions. This will allow interaction with a running POSIX application from a separate shell session.

- Application Tests covered in the jobs description but summarised here:

**POSIX Application Test Execution Guide**

`Use this concise guide to bring up networking, launch the reference app, and run tests.`

`One-Time Setup`

`Run these once per machine boot (or when networking state is reset):`

```
# Bring up Ethernet
./posix/tools/enet/bring-up-ethernet.sh
# Bring up virtual CAN on vcan0
./posix/tools/can/bring-up-vcan0.sh
```
`Launch the Reference Application:`

`Starts the POSIX reference application console:`

```
./posix/build/posix/executables/referenceApp/application/Release/app.referenceApp.elf
```
- Keep this running while testing.

- Stop with Ctrl+C when done.

`Run POSIX pyTest:`

`Execute pyTests targeting the POSIX build::`
```
cd posix/test/pyTest/ && pytest --target=posix-freertos
```
---
### TAA-889 | Horizon R3 Security update 

Open Source Modules BOM and versions.

[Horizon SDV Rel.3.0.0 Open Source modules](bom/horizon-sdv-rel.3.0.0-open-source-modules.md)

[Jenkins BOM(3.0.0)](bom/jenkins-bom-3.0.0.md)

[Gerrit BOM(R3.0.0)](bom/gerrit-bom-r3.0.0.md)

---

### TAA-377 | Google AOSP Repo Mirroring

 #### Release Note

 #### Changes

- Introduced a **standardized Jenkins pipeline suite** to manage AOSP mirror lifecycle on GCP.
- Automated **infrastructure provisioning** using Terraform (Filestore + PV/PVC).
- Added **NFS-based mirror storage** backed by Google Cloud Filestore.
- Implemented **Buildkit-based Docker image build** (no Docker-in-Docker).
- Enabled **mirror sync workflows** for single or multiple AOSP mirrors.
- Added **safe deletion workflows** for individual mirrors or full infrastructure.
- Enforced **global build blocking** to prevent concurrent write operations.
- Standardized **security context** (non-root containers with init permission fix).
- Centralized shared logic via **utils.sh** for logging, validation, and Terraform ops.

#### Action Required

- **First-time setup:**
  - Run **Docker Image Template** pipeline to build the base image.
  - Run **Create Mirror Infrastructure** to provision Filestore and PVC.

- **Day-to-day usage:**
  - Use **Sync Mirror** to create or update AOSP mirrors.
  - Use **Delete Mirror** to remove specific mirrors or tear down all resources.

- **Before syncing:**
  - Verify required Jenkins environment variables are configured.
  - Ensure sufficient Filestore capacity (recommended: 2048 GiB).

- **Operational hygiene:**
  - Avoid running create/delete/sync jobs concurrently.
  - Periodically clean up unused mirrors to reclaim storage.
  - Monitor sync logs for git lock retries or capacity warnings.

---

 ### TAA-947 | ABFS update for R3

Corrections and minor ABFS updates delivered from Google in Release 3.0.0 timeframe.

---

### TAA-1072 | Cloud Artifact storage management

 #### Artifact Tagging in GCS

In GCP, [custom metadata](https://cloud.google.com/storage/docs/metadata#custom-metadata) can be applied to stored artifacts as a means of labelling those objects. By employing a sensible tagging strategy, objects can be easily selected and managed using custom metadata.

Custom metadata takes the form of `key=value` pairs, both of which are entirely customisable (although non-ascii characters should be avoided). Any number of `key=value` pairs can be applied to an object. Custom metadata associated with an object can be updated and/or deleted at any time.

It should be noted that custom metadata in GCP is subject to a [size limit](https://cloud.google.com/storage/quotas#objects) and incurs [storage costs](https://cloud.google.com/storage/pricing#storage-notes).

GCP Storage uses the concept of virtual folders where folders themselves don't actually exist and are simulated as a result of hierarchical naming of objects (e.g. *Android/Builds/AAOSBuilder/01/build_info.txt*); as a result, custom metadata can only be applied to objects, not folders. Instead of tagging an entire folder, all objects in the folder can be tagged.

#### Setting / Manipulation of Custom Metadata On New Objects

The following build jobs provide a parameter `STORAGE_LABELS` which allows users to add labels to the artifacts being uploaded to storage buckets. When using the GCP storage option, these labels are implemented as custom metadata.

- Android / AAOS Builder
- Android / AAOS Builder ABFS
- OpenBSW / BSW Builder

#### Setting / Manipulation of Custom Metadata On Existing Objects

The following utility jobs can be used to view, add, modify and delete custom metadata on individual objects or groups of objects (grouped by folder or by existing metadata via filtering):

- **Object - List Metadata** This job allows the user to inspect the metadata of objects stored in a GCS bucket.

- **Object - Add Metadata** This job allows the user to add metadata (key/value pairs) to objects stored in a GCS bucket.

- **Object - Remove Metadata** This job allows the user to remove specify or all metadata from objects stored in a GCS bucket.

- **Filter Objects by Metadata** This job allows the user to list all objects in a bucket path based on the metadata that is set on them. The user can choose to list objects with specific metadata, objects with any metadata or objects with no metadata.

- **Filtered Objects - Remove Metadata** This job allows the user to find all objects in the bucket which have a specified metadata item set and remove that metadata item from the objects.

- **Filtered Objects - Update Metadata** This job allows the user to find all objects in a bucket path which have the specified metadata set and update the value of that custom metadata item.

#### Cleanup in GCP

##### Using Lifecycle Policies

In GCP, [lifecycle](https://cloud.google.com/storage/docs/lifecycle) management is done at a bucket level.

Lifecycle configurations contain a set of rules, each of which contains an [action](https://cloud.google.com/storage/docs/lifecycle#actions) (e.g. delete) and [conditions](https://cloud.google.com/storage/docs/lifecycle#conditions) (e.g. age=100days). When any object in the bucket meets all conditions, the specified action is taken.

In GCP, lifecycle conditions can be based on the following:
- Object name prefix or suffix
- Fixed-key metadata set on the object

A limitation with GCP is that custom metadata cannot be used to create lifecycle management rules. However, a workaround is explained in a following section.

##### Using Storage Classes

The [storage class](https://cloud.google.com/storage/docs/storage-classes) of a GCP object determines its availability and storage cost.

The default storage class used for a bucket is set during the bucket setup process and all uploaded objects inherit this default storage class. The storage class of any object can be changed explicitly.

Storage classes can be used in lifecycle management policies in order to manage costs of objects which need to be retained for extended periods.

##### Using Custom Metadata

The following utility jobs have been provided so that the user can perform cleanup operations on GCP objects based on the custom metadata that is set on them:

- **Object - List Storage Class** This job allows the user to inspect the storage class of objects stored in a GCS bucket.

- **Filtered Objects - Delete** This job allows the user to find all objects in a bucket path which have the specified metadata set and delete those objects.

- **Filtered Objects - Move** This job allows the user to find all objects in a bucket path which have the specified metadata set and change the storage class of those objects.

---

### TAA-1001 | Kubernetes Dashboard SSO integration

#### Release Note

#### Headlamp Application

Headlamp application has been added to the Horizon platform. It helpful to browse Kubernetes resources and diagnose problems.

#### Changes

Added Single Sign On (SSO) capability to Headlamp using OAuth2 Proxy and Nginx based token injector: 
1. Added headlamp post-job:
   
    a. Creates `oauth2-headlamp` client on Keycloak and maps `groups` client scope.

    b. Creates `horizon-headlamp-administrators`, which grants users within this group access to Headlamp.

    c. Updates required Kubernetes secret with required values and restarts OAuth2 Proxy deployment.

2. Headlamp SSO
   
    a. When visiting `/headlamp/`, users are now redirected to OAuth2 Proxy Keycloak sign-in (Sign-in with Google), which after authentication (and if user has required access) get redirected to Headlamp.

#### Actions

In order to get this feature to work on any deployment:

1. Once the changes have been pulled into the branch/environment of your choice, run the Terraform workflow and wait for it to finish successfully.
2. Login to Argo CD and sync the `horizon-sdv` app and wait for the headlamp post-job to finish successfully.
3. To enable Headlamp SSO for a user of choice, login to Keycloak and add the user to `horizon-headlamp-administrators` group.
4. Opening Headlamp will now redirect to the Keycloak sign-in page (via OAuth2 Proxy). Click on the **G** button (Sign in with Keycloak) to sign in and access Headlamp functionality.

---

### TAA-945 | Replace deprecated Kaniko tool

#### Release Note

#### Kaniko Replacement

Google have retired support for kaniko container builder, as such we have migrated docker container builds to use buildkit instead of kaniko.

As such we recommend users rerun seed jobs and regenerate the docker containers as follows. By all means retain your existing container builds, but we advise based on metrics that buildkit can offer performance improvements over kaniko for size, and pod start up time.

#### Changes

#### Action Required

**Seed update**

In Jenkins select Seed Workloads

Select `Build`  
When job has completed, new parameters for buildkit and [Release](https://github.com/GoogleCloudPlatform/docker-credential-gcr/releases) will be available.

In Jenkins select Seed Workloads → Build With Parameters

Select `SEED_WORKLOAD all`

**Docker Image Template updates**

In Jenkins select Android Workflows → Environment → Docker Image Template:

Deselect `NO_PUSH`
Select `Build` 

In Jenkins select Android Workflows → Environment → ABFS → Docker Image Template:

Deselect `NO_PUSH`
Select `Build` 

In Jenkins select Android Workflows → Environment → ABFS → Docker Infra Image Template:

Deselect `NO_PUSH`  
Select `Build` 

In Jenkins select OpenBSW Workflows → Environment → Docker Image Template:

Deselect `NO_PUSH` 
Select `Build` 

In Jenkins select Cloud Workstations → Workstation Images (repeat for all workstations you wish to support)

Select your workstation job and then:

Deselect `NO_PUSH` 
Select `Build`  

The images will have been updated in the artifact registry for all dependent jobs to reference.

---

### TAA-941 | IAA demo case

Support for Partner demo in IAA Messe show. The main technical scope is to apply a binary APK file to the Android code, help building it and flash it to selected targets (Cuttlefish and potentially Pixel) according to Partner specification.

---

## Improved Features

|Issue ID |Summary|
|---------|-------|
|TAA-1171|Create Workloads area in Gitops section|
|TAA-862 |Improvements Structure of Test pipelines|
|TAA-1111|Unified CTS Build process|
|TAA-1265|[Gerrit] Support GERRIT_TOPIC with existing gerrit-triggers plugin|
|TAA-1271|Support custom machine types for Cuttlefish|
|TAA-1269|Adjust CTS/CVD options|

---
### TAA-1171 | Create Workloads area in Gitops section

#### Release note

A new **workloads directory** was introduced under `gitops/env/stage2/workloads`. Helm chart values and settings related to Android workloads  were moved there for better separation and maintainability (eg. workloads-android.yaml, workloads-openbsw.yaml)

**Jenkins configuration updates**

Workload-specific Jenkins settings were moved into `values-jenkins.yaml`. The `jenkins.yaml` template now contains the Jenkins Helm chart with core platform-dependent settings and variables, ensuring a cleaner structure and easier customization (e.g., Jenkins plugin list)

---

### TAA-862 | Improvements Structure of Test pipelines

#### Release Note

#### Jenkins Shared Library
This suggestion came from the OSS community and recommendations to reduce pipeline definition duplication by using Jenkins shared libraries.

#### Changes:

**Android Workflows → Tests → CVD Launcher**

- This is now a common shared library located under  
`workloads/common/jenkins/shared-libraries/cvd-pipeline-shared-library/`

- The Jenkinsfile simply references and calls the `cvdPipeline` function.

> **Note** : There are 2 custom stages that are ignored by CVD Launcher pipeline, effectively NOOPs. These are utilised by the CTS Execution pipeline to include Compatibility Test Suite specific stages.

`Android Workflows → Tests → CTS Execution`

- As much of this Jenkins pipeline definition was common with CVD Launcher, it has now been updated to use the shared library above but defines specific custom stages in the pipeline.

**Global Shared Library configuration**

- This defines the shared library, path, branch etc that the two jobs now reference using @Library definition.

- Jenkins configuration:  
`gitops/env/stage2/templates/jenkins.yaml` (see `globalLibraries: libraries:`)

- See also  Jenkins → Manage Jenkins → System → Global Trusted Pipeline Libraries.

---

### TAA-1111 | Unified CTS Build process

#### Release Note

#### Unify CTS builds

The dedicated `CTS Builder` job has been removed. CTS development builds can now be built via `AAOS Builder` and `AAOS Builder ABFS` from the provided manifest, reducing duplicate maintenance for near-identical jobs.

Both build jobs now support a new parameter, `AAOS_BUILD_CTS`. When enabled, it builds CTS from the given source tree. This flag only applies to Cuttlefish targets (`aosp_cf*`); for non-Cuttlefish targets it is ignored. Users must still set `AAOS_LUNCH_TARGET` to enable CTS builds.

#### Action Required

Remove the now defunct CTS Builder job:

- Select `Android Workflows → Builds → CTS Builder`
- Select `Delete Folder`

Seed the new parameter to the Server and Uploader Operations jobs:

- Select `Seed Workloads → Build with Parameters → SEED_WORKLOAD = none`  
   - This will ensure the parameters get updated to include the new `ABFS_COS_IMAGE_REF` parameter.

- Select `Seed Workloads → Build with Parameters → SEED_WORKLOAD = android`  
    - This will ensure the new parameter `AAOS_BUILD_CTS` is available within the `AAOS Builder` and `AAOS Builder ABFS` jobs.

CTS development builds may now be built using the `AAOS Builder` and `AAOS Builder ABFS` jobs.

--- 

### TAA-1265 | [Gerrit] Support GERRIT_TOPIC with existing gerrit-triggers plugin

#### Release Note

We added support so build jobs handle not only single-project Gerrit updates but also groups of changes identified by the `GERRIT_TOPIC` parameter (eg. `repo upload -t <TOPIC>`).

- **Why:** This lets developers test changes that span multiple repositories and can prevent the Gerrit trigger plugin from creating a separate Jenkins build per change when the changes belong to the same topic, reducing unnecessary builds and overhead.

    - Gerrit Trigger remains limited to voting on one change but the purpose of adding this support is to allow building of changes that span multiple repositories.

- **Jobs updated**

    - **Android Workflows → Builds → AAOS Builder**
      - Added a new manual parameter: `GERRIT_TOPIC`.
      - When `GERRIT_TOPIC` is provided, the build initialization/sync step will request all changes for that topic and apply (fetch/cherry-pick) them across the relevant projects in the repository.

    - **Android Workflows → Builds → Gerrit**
      - The Gerrit trigger is now configured to prefer a single topic and is delayed to ensure all related changes are handled together.
      - If `GERRIT_TOPIC` exists, the triggered build will fetch and apply every change in that topic across the corresponding projects.
      - Each build stage will provide a comment indicating build success/failure to all the changes associated with the `GERRIT_TOPIC`.
      - The overall **Verified** vote will only be **+1** if all build stages complete successfully, otherwise **-1** will be reported.

- **Fallback:** If `GERRIT_TOPIC` is not set, jobs keep their existing (legacy) behaviour.

- **Note:** know Gerrit limitations, e.g. when abandoning changes, remove the TOPIC.

**Future Release Plan:** we plan on changing this so users manage the builds from a label and vote within the Gerrit open review item.

---

### TAA-1271 & TAA-1269 | Support custom machine types for Cuttlefish and adjust CTS/CVD options

#### Release Note

#### Cuttlefish Updates
Provide additional flexibility for creation of Cuttlefish instance templates.

- **Why:**

  - Let developers create Cuttlefish VM instance templates from their custom machine types, i.e. specific series, CPU and Memory configuration over the standard machine types.

  - Provide additional options to support building Cuttlefish from other repositories as an alternative to standard Google repository.

- #### Jobs updated

  - **Android Workflows → Environment → CF Instance Template**

    - Added support to define the Android Cuttlefish repository to use:  
`ANDROID_CUTTLEFISH_REPO_URL`
      - If a private repo, then provide credentials by defining `REPO_USERNAME` and `REPO_PASSWORD`.
        - **Note:** the password is never exposed in Jenkins nor console. Added support for custom machine types:  
  `CUSTOM_VM_TYPE`  
  `CUSTOM_CPU`  
  `CUSTOM_MEMORY`
      - Simply unset `MACHINE_TYPE` and define custom fields that match users requirements.
      - Default `MACHINE_TYPE` has changed to `n2-standard-32` as a trade off for costs vs performance.
      - #### Miscellaneous
        - **Curl Upgrade Support** has been added to update the version on debian-12 based Cuttlefish instances, see:  
`CURL_UPDATE_COMMAND`

        - `x86_64`:
          - The default parameter will upgrade curl to 8.1x from debian backports.
          - Users may remove the parameter if they wish to remain on 7.88.1.
        - **ARM64:**
          - ARM instances currently only support ubuntu 22.04 LTS version, so the parameter has no default defined.
        - New parameter to support working around android-cuttlefish.git build issues:  
`ANDROID_CUTTLEFISH_POST_COMMAND`
          - Use git commands to switch to a specific sha1 when using branches (main), or cherry-pick workarounds/fixes to tagged versions which cannot be modified by google, e.g.
            - `git cherry-pick <sha1>`
            - `sed -i 's|https://git.kernel.org/pub/scm/linux/kernel/git/jaegeuk/f2fs-tools|https://github.com/jaegeuk/f2fs-tools|g' base/cvd/MODULE.bazel`
        - Options to update CTS revisions and even offer capability to pull from GCS and not just the official download site (ie add more control to versions because the download site has updated the same revision several times in the past, changing the tests):
`CTS_ANDROID_16_URL`  
`CTS_ANDROID_15_URL`  
`CTS_ANDROID_14_URL`

  - **Android Workflows → Environment → CF Instance Template ARM64**
    - Same as CF Instance Template but ARM64 is currently only supported on one machine type. So use `MACHINE_TYPE` for now.

  - **Android Workflows → Tests → CTS Execution**

    - The resources have changed to align with the default Cuttlefish instance `MACHINE_TYPE`.
      -  `NUM_INSTANCES = 7`, `CPU = 4`, `MEMORY = 8192`
      -  Adjust these to suit your test instance, these are simply set to align with the Horizon default Cuttlefish instance. For example, the ARM64 instance is a 96CPU instance and thus more resources are available.
     - Failures are summarised in the test_result_failures_suite.html file with the respective CTS job and published to Jenkins.
  
      ![Test-Result](/docs/images/test_result.png)

#### Future Release Plan

- ARM64 may be limited to one machine type currently, but if Google expand support to custom, we should be able to support.

- Both jobs may be combined into one job once ARM64 is not restricted to a single region.

---

### Bug Fixes

| ID        | Summary |
|-----------|---------|
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
| TAA-1238  | [Cuttlefish] Update to v1.31.0 - v1.30.0 has changed from stable to unstable |
| TAA-1241  | [Android] Mirror should not be using OpenBSW nodes for jobs AM |
| TAA-1247  | [Workloads] Remove chmod and use git executable bit |
| TAA-1249  | [GCP] Client Secret now masked (security clarification) |
| TAA-1264  | [CVD] Logs are no longer being archived |
| TAA-1261  | [Cuttlefish] gnu.org down blocking builds |
| TAA-1266  | Pipeline does not fail when IMAGE_TAG is empty and NO_PUSH=true |
| TAA-1267  | [CWS] OSS Workstation blocking regex incorrect (non-blocking) |
| TAA-1258  | [Cuttlefish] VM instance template default disk too small |
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
| TAA-1208  | Mirror/Sync-Mirror: Sync all mirrors when `SYNC_ALL_EXISTING_MIRRORS` is selected |
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
| TAA-906	  | Git command issues on ABFS mounted directory |
| TAA-1293	| credentials generation for postgresql |	 	 	 	 
| TAA-1294	| improvements for credentials generation -moving ssh-key generation into module|
| TAA-1303  |	Jenkins test jobs using k8s agent when none is expected	 |	 
| TAA-1304	| [Jenkins] Plugin updates for bug fixes	|			

---