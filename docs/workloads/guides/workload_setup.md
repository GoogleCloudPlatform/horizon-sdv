[Copyright (c) 2026 Accenture, All Rights Reserved.]::

[Licensed under the Apache License, Version 2.0 (the 'License');]::
[you may not use this file except in compliance with the License.]::
[  You may obtain a copy of the License at]::

[          http://www.apache.org/licenses/LICENSE-2.0]::

[  Unless required by applicable law or agreed to in writing, software]::
[  distributed under the License is distributed on an 'AS IS' BASIS,]::
[WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.]::
[  See the License for the specific language governing permissions and]::
[  limitations under the License.]::


# <span style="color:#335bff">Workload Setup</span>

This document covers the setup of **Android**, **Cloud Workstations**, and **OpenBSW** Jenkins workloads, including the setup of access requirements and underlying dependencies such as Gerrit repos.

It includes pointers to workload documentation under [`docs/workloads/`](../../workloads/) (job parameters and scripts are described there and in `workloads/*/pipelines/**/README.md`).

> **Note:** This setup document combines with the [workload_usage.md](workload_usage.md), [workload_usage_android.md](workload_usage_android.md) and [android_labs.md](android_labs.md) documents to supersede the former Android `docs/workloads/android/guides/developer_guide.md` (deprecated). 

## Table of Contents
- [Prerequisites](#prerequisites)
- [Access Setup](#access)
  - [Google Cloud CLI](#cloud-cli)
  - [Jenkins](#jenkins)
  - [MTK Connect](#mtk-connect)
  - [Gerrit access (optional)](#access-gerrit)
- [Gerrit setup (optional)](#gerrit-setup)
  - [Project Fork Creation](#gerrit-fork-creation)
  - [Patches](gerrit-patches)
- [Workload setup](#workload)
  - [Seed Jenkins jobs](#workloads-jenkins-jobs)
  - [Docker image template](#workloads-docker-image-template)
  - [Android](#android)
    - [Cuttlefish instance template(s)](#workloads-android-cuttlefish-instance-template)
    - [Warm build caches (optional)](#workloads-android-warm-build-caches)
    - [AOSP mirror (optional)](#workloads-android-mirror)
  - [Cloud Workstations](#cloud-workstations)
    - [Create cluster](#workloads-cloud-workstations-create-cluster)
    - [Create workstation configuration](#workloads-cloud-workstations-configs)
    - [Publish IDE/container image(s)](#workloads-cloud-workstations-publish-images)
    - [Create workstation(s)](#workloads-cloud-workstations-create-workstations)
- [Appendices](#appendices)
  - [Jenkins clouds & GCE instance templates](#appendix-jenkins-clouds)

Before developers can use the Horizon SDV platform tools, several setup steps must be completed.

The following setup instructions should be executed immediately after the platform has been initialised or updated.
Unless stated otherwise, they should be carried out in the order provided.

Setup operations need to be carried out only once (although per-user access setup will obviously need to be carried out whenever a new user is added).

[ ============================================================================= ]::

## <span style="color:#335bff">Prerequisites<a name="prerequisites"></a></span>

| Prerequisite | Description                                                                                                                                                                                            |
| --- |--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Horizon SDV Platform Provisioned | - User added to Horizon Keycloak and appropriate Jenkins group/role has been assigned <br> - User can access Horizon applications in the browser (e.g. Developer Portal, Gerrit, Jenkins, MTK Connect) |
| PC Tools | PC (Mac, Linux, Windows) with [git](https://git-scm.com/book/en/v2/Getting-Started-Installing-Git) and [gcloud CLI](https://cloud.google.com/sdk/docs/install) installed                               |
| Google Cloud Platform Project set up | User has access to the Google Cloud Platform project (verify on [Google Cloud Console](https://console.cloud.google.com/) - ensure the correct project is selected)                                    |


[ ============================================================================= ]::

## <span style="color:#335bff">ACCESS SETUP<a name="access"></a></span>

[ ========================== ]::

### <span style="color:#335bff">Access Setup: Google Cloud CLI<a name="cloud-cli"></a></span>

Google Cloud CLI can be accessed in the following ways:


<details id="google-cloud-cli"><summary>Cloud Shell Terminal</summary><hr width="50%">

  - Log into the Google Cloud Console on your browser
  - Click on the _Activate Cloud Shell_ button on the top right of the page

<hr width="50%"></details>

<details id="pc-cloud"><summary>PC or Cloud Instance</summary><hr width="50%">

  - Open any terminal
  - Install gcloud CLI as per [instructions](https://docs.cloud.google.com/sdk/docs/install-sdk) 
  - Run `gcloud auth login` and follow the instructions
  - Set the required project ID (if not already correct)

<hr width="50%"></details>

[ ========================== ]::

### <span style="color:#335bff">Access Setup: Jenkins <a name="jenkins"></a></span>

To access and run Jenkins workload jobs, users must be granted Group access on Keycloak.

Keycloak uses a **Role-based Authorization Strategy (RBAC)** to integrate with Jenkins. 
Group assignment in Keycloak corresponds to role assignment in Jenkins, so the Jenkins role assigned to a user determines what permissions they will have. 
Default roles assigned during Keycloak Group Access can be retained unless instructed otherwise. 

The **administrator role** needs to be assigned in order for a user to perform certain operations (e.g. using Gerrit and running the [Seed job](../seed.md) which sets up / updates the workload jobs).
<br>

<details id="change-roles-temp"><summary>Change RBAC Roles in Jenkins - Temporary</summary><hr width="50%">

  - In `Jenkins` → `Manage Jenkins` → `Manage and Assign Roles` → `Assign Roles`.
    - Available Roles: 
        - Global: _administrators_, _developers_, _users_
        - Item: _workloads-developers_, _workloads-users_
  - Add the user to appropriate Global / Item Role(s):
    - In the appropriate section, select `Add User`, enter the email address of the user and select the appropriate role(s).
    - Select `Save`

  These manually updated permissions do not persist across a Jenkins restart.

<hr width="50%"></details>

<details id="change-roles-perm"><summary>Change RBAC Roles in Jenkins - Perisistent</summary><hr width="50%">

  To ensure persistence of RBAC Roles, we recommend adding users to the `gitops/templates/jenkins.yaml` file, e.g..
  - Update `authorizationStrategy` → `roleBased` → `roles` → `global`:
    - Add the user to the respective group entry, e.g. ` - user: "john.example.doe@accenture.com"`
  - Update `authorizationStrategy` → `roleBased` → `roles` → `items`:
    - Add the user to the respective group entry, e.g. ` - user: "jane.example.doe@accenture.com"`

<hr width="50%"></details>

<details id="disable-rbac-plugin"><summary>OPTIONAL: Disabling the RBAC plugin</summary><hr width="50%">

  If user wishes to disable the rbac plugin, then remove the plugin and configuration from `gitops/templates/jenkins.yaml`:
  - Remove the plugin from the `additionalPlugins` section:
    - `role-strategy:743.v142ea_b_d5f1d3`

  - Replace all within `authorizationStrategy` with the following default values:
  ```
              authorizationStrategy: |-
                loggedInUsersCanDoAnything:
                  allowAnonymousRead: false
  ```
  Sync Jenkins using ArgoCD and restart Jenkins.

<hr width="50%"></details> 

[ ========================== ]::

### <span style="color:#335bff">Access Setup: MTK Connect<a name="mtk-connect"></a></span>
In order to successfully run a test job which creates / populates an MTK Connect testbench (e.g. [Android/Tests/CVD Launcher](../android/tests/cvd_launcher.md)), the user needs to log into MTK Connect before executing the job. Simply entering the MTK Connect application from the main dashboard is sufficient.

[ ========================== ]::

### <span style="color:#335bff">Access Setup: Gerrit<a name="access-gerrit"></a></span>

Access setup for Gerrit is <b>OPTIONAL</b> - it is necessary only if the setup of a Gerrit repo is required.

| Prerequisite | Description |
| --- | --- |
| Gerrit Application | Gerrit application provisioned & initialised in the Horizon platform |
| User Sign-in | User has signed into Gerrit at least once |
| Tools | [kubectl](https://kubernetes.io/docs/reference/kubectl/) installed on the machine being used for Google Cloud CLI access |

The following setup tasks should be carried out once (in the given order):

<details id="gerrit-admin-pswd"><summary>1. Retrieve Gerrit Admin Password</summary><hr width="50%">

  The gerrit admin password is required for performing administrative tasks in gerrit (e.g. adding users to the `Administrators` group).

  To obtain Gerrit admin user name and password, perform the following command on a terminal (PC or cloud instance) which has logged in to the cloud project. 

  1. Retrieve membership credentials from fleet management, e.g.:<br/>
  <pre>
      gcloud container fleet memberships list
      gcloud container fleet memberships get-credentials sdv-cluster
  </pre>
  2. Retrieve the username and password:<br/>
  <pre>
      kubectl get secrets -n keycloak keycloak-gerrit-admin -o json | jq -r '.data.username' | base64 -d
      kubectl get secrets -n keycloak keycloak-gerrit-admin -o json | jq -r '.data.password' | base64 -d
  </pre>

<hr width="50%"></details> 

<details id="gerrit-user-access"><summary>2. Set Gerrit Admin Access for All Users</summary><hr width="50%">

  All users who need to work with Gerrit must be added to the _Adminstrator_ group and must create their own HTTP token/password.

  - Login to Gerrit as <code>gerrit-admin</code> and associated password retrieved from keycloak secrets earlier.
  - Select `BROWSE` → `Groups` → `Administrators`
  - Select `Members` and add your email and any other users to the `Administrators` group:
      - _If no option to ADD appears after the email address or name is entered, check for Browser issues:_
          - _delete cookies in Browser, log in as yourself, then delete cookies again and log back in as <code>gerrit-admin</code>_
          - _Try incognito mode_
  
<hr width="50%"></details> 

<details id="gerrit-project-access"><summary>3. Gerrit Project Access</summary><hr width="50%">

  In order to create projects based on Google Android Opensource tags, some additional permissions are required to allow the forks to be created. These permissions are set as part of the `All-Projects` project (`refs/meta/config`) and can be set using the Gerrit UI.

  - Select `BROWSE` → `Repositories` → `All-Projects` → `Access` → `Edit` (bottom of page)
  - Navigate to the section: `Reference:refs/heads/*`
  - Perform the following steps for each of the permissions [`Forge Server Identity`, `Push`, `Push Merge Commit`, `Label Ready-for-Build`]:
    - if the required permission doesn't already exist in this section.
      - Navigate to the end of the section (i.e. `Reference:refs/heads/*`)
      - Select the required permission in the `Add Permission` box
      - Click `ADD` 
    - if `ALLOW`:`Administrators` is not already set on the required permission:
      - In `Add group` text box, type `Administrators` & click on the option when it appears
  - Select `SAVE` at the bottom of the page to update the configuration.

  > **NOTE**
  > - the [Skip Validation](https://gerrit-review.googlesource.com/Documentation/user-upload.html#skip_validation) push option is required for projects with a large number of commits and also to retain the original committer etc.
  
<hr width="50%"></details>

All users of Gerrit need to create their own HTTP Token/Password:

<details id="gerrit-user-token"><summary>Create User HTTP Token/Password</summary><hr width="50%">

  - Login to Gerrit as yourself (user), **not** as `gerrit-admin`.
  - Select `USER`(Top Right Hand Side)→`Settings`→`HTTP Credentials`
  - Select `GENERATE NEW PASSWORD`
  - Note the password for later usage.

<hr width="50%"></details>

[ ============================================================================= ]::

## <span style="color:#335bff">GERRIT SETUP<a name="gerrit-setup"></a></span>

The setup of a Gerrit repo is <b>OPTIONAL</b> - it should be done only if a private repo is required. Once set up, the private gerrit repo manifest can be used in build jobs instead of a public manifest.
- E.g. For an Android build, the parameter `AAOS_MANIFEST_URL` can be set to `https://example.horizon-sdv.com/gerrit/android/platform/manifest` instead of the public `https://android.googlesource.com/platform/manifest`. 

The setup instructions below are for the Android projects which are required by the [Gerrit](../android/builds/gerrit.md) job in the Android Workload area, but the process can be used to set up repositories for use in any Workload area.

> **IMPORTANT**
> - Replace `example.horizon-sdv.com` in the example URLs with your domain
> - Your Horizon SDV Gerrit credentials and HTTP token/password will need to be used.
> - Other projects and branches can be added as required - these are provided as examples. However, read to the end of this section for info on build support for additional branches/tags.

[ ========================== ]::

### <span style="color:#335bff">Gerrit Setup: Project Fork Creation<a name="gerrit-fork-creation"></a></span>

The following Android projects will be forked from upstream [Google AOSP](https://android-review.googlesource.com). 

| Source Projects | Branches |
| --- | --- |
| `platform/manifest`<br>`platform/frameworks/native`<br>`platform/packages/ervices/ar`<br>`platform/platform_testing`<br>`platform/hardware/interfaces`<br>`platform/packages/apps/Car/Launcher` | `android-14.0.0_r30`<br>`android-15.0.0_r36`<br>`android-16.0.0_r3` |

For clarity, the names of the projects and branches in Gerrit will be prefixed with `android/`. (e.g. `platform/manifest` becomes `android/platform/manifest` and `android-14.0.0_r30` becomes `horizon/android-14.0.0_r30`)


<details id="gerrit-create-empty-repo"><summary>Create Empty Gerrit Repos</summary><hr width="50%">

  For each project being forked, create an empty repo as follows:
  - In Gerrit (logged in as user), select `BROWSE` → `Repositories` → `CREATE NEW`
      - Enter the `Repository Name` as it will exist in Gerrit (e.g. `android/platform/manifest`)
      - Enter the `Default Branch` (e.g. `horizon/android-14.0.0_r30`)
      - Set `Create Empty Commit` to `False` to retain original upstream history
  - Select `CREATE`.

<hr width="50%"></details> 

<details id="gerrit-create-forks"><summary>Create Forks of Upstream Repos</summary><hr width="50%">

For each project being forked, perform the following steps:

- Clone the AOSP repo - example: `<UPSTREAM PROJECT NAME>`=`platform/manifest`:

  ```
  git clone <https://android.googlesource.com/<UPSTREAM PROJECT NAME>
  ```
  
- Add the Horizon SDV Gerrit remote:
  ```
  git remote add horizon https://example.horizon-sdv.com/gerrit/android/<UPSTREAM PROJECT NAME>
  ```
- Create the forked branch and push to Gerrit (repeat for all required branches) -  example: `<TAG|BRANCH>`=`android-14.0.0_r30`:
  ```
  git checkout -b horizon/<TAG|BRANCH> <TAG|BRANCH>``
  git push -o skip-validation horizon horizon/<TAG|BRANCH>
  ```

Example of project forking for `platform/manifest`:
<pre>
git clone https://android.googlesource.com/platform/manifest
cd manifest
git remote add horizon https://example.horizon-sdv.com/gerrit/android/platform/manifest
git checkout -b horizon/android-14.0.0_r30 android-14.0.0_r30
git push -o skip-validation horizon horizon/android-14.0.0_r30
git checkout -b horizon/android-15.0.0_r36 android-15.0.0_r36
git push -o skip-validation horizon horizon/android-15.0.0_r36
git checkout -b horizon/android-16.0.0_r3 android-16.0.0_r3
git push -o skip-validation horizon horizon/android-16.0.0_r3
</pre>

<hr width="50%"></details> 

<details id="gerrit-update-manifests"><summary>Update Manifests</summary><hr width="50%">

  In order to use the forked repos, the Horizon SDV Gerrit manifests must be updated to reference the forked repos, with a reference added for each branch that will be needed. Follow these steps to update the manifests, performing step 2 for each required branch.

  <details id="gerrit-clone-manifest"><summary>1. Clone the Horizon SDV Gerrit manifest</summary>
  <hr width="25%">

  - In Gerrit select `BROWSE` → `Repositories` → select the required repo (e.g.`android/platform/manifest`)
  - Copy the `Clone with commit-msg hook`
  - Paste the copied command to clone the repo, e.g.:
    <pre>
    git clone "https://example.horizon-sdv.com/gerrit/android/platform/manifest" && (cd "manifest" && mkdir -p `git rev-parse --git-dir`/hooks/ && curl -Lo `git rev-parse --git-dir`/hooks/commit-msg https://example.horizon-sdv.com/gerrit/tools/hooks/commit-msg && chmod +x `git rev-parse --git- dir`/hooks/commit-msg)
    </pre>
  <hr width="25%">

  </details>

  <details id="gerrit-update-manfifest-branches"><summary>2. Update each branch</summary><hr width="25%">

  - Checkout the branch: `git checkout horizon/android-14.0.0_r30`
  - Edit the `default.xml` file to update the remote values as follows (you will need to add the `gerrit` remote and ensure the URL matches your domain):<br/>
      ```
      <remote name="aosp"
              fetch="https://android.googlesource.com"
              review="https://android-review.googlesource.com/" />
      <remote name="gerrit"
              fetch="https://example.horizon-sdv.com/gerrit"
              review="https://example.horizon-sdv.com/gerrit/" />
      <default revision="refs/tags/android-14.0.0_r30"
              remote="aosp"
              sync-j="4" />
    ```
  - Find the `<project path=` entry for each of the projects you forked
  - Edit those entries as follows: 
    - update `name` and `revision` to include `android/` prefix 
    - add `remote=gerrit`
      Example:
      ```
      <project path="frameworks/native" name="android/platform/frameworks/native" groups="pdk" remote="gerrit" revision="horizon/android-14.0.0_r30" />

      <project path="hardware/interfaces" name="android/platform/hardware/interfaces" groups="pdk,sysui- studio" remote="gerrit" revision="horizon/android-14.0.0_r30" />

      <project path="packages/apps/Car/Launcher" name="android/platform/packages/apps/Car/Launcher" groups="pdk-fs" remote="gerrit" revision="horizon/android-14.0.0_r30" />

      <project path="packages/services/Car" name="android/platform/packages/services/Car" groups="pdk-cw- fs,pdk-fs" remote="gerrit" revision="horizon/android-14.0.0_r30" />

      <project path="platform_testing" name="android/platform/platform_testing" groups="pdk-fs,pdk-cw- fs,cts,sysui-studio" remote="gerrit" revision="horizon/android-14.0.0_r30" />
      ```
  - Commit: `git commit -am "Update android-14.0.0_r30 manifest"`
  - Update commit-id: `git commit --amend --no-edit`
  - Push for review: `git push origin HEAD:refs/for/horizon/android-14.0.0_r30`
  - Review and Submit change in Gerrit:
    - In Gerrit, select `CHANGES` → `OPEN` and click on the change or open the CLI link reported in the console after `push`.
    - Review and submit the change: `REPLY` → `CODE-REVIEW+2` → `SUBMIT` → `CONTINUE`

  <hr width="25%"></details>

<hr width="50%"></details> 


<details id="gerrit-job-additional-branches"><summary>Optional: Android Workflow - Gerrit Build job with Additional Branches/Tags</summary><hr width="50%">

If more than the default set of forked branches/tags listed earlier is required for Android builds, the Gerrit Build job [Jenkinsfile](../../../workloads/android/pipelines/builds/gerrit/Jenkinsfile) must be updated to add logic to determine the build version (e.g. `ap1a`, `ap2a` …. `bp1a`, `bp3a`); this logic is vital for determining the lunch target name vs android revision.

<hr width="50%"></details> 


[ ========================== ]::

### <span style="color:#335bff">Gerrit Setup: Patches<a name="gerrit-patches"></a></span>

If required, patches can be applied to any of the forked repos in Gerrit and pushed for use by all users - this is simpler than users having to manually include the patch in builds.


<details id="gerrit-patches-example"><summary>Example Patch: Pixel Audio Patch - `android-14.0.0_r30`</summary><hr width="50%">

  ```
  # Clone android/platform/packages/services/Car
  git clone https://example.horizon-sdv.com/gerrit/android/platform/packages/services/Car -b horizon/android-14.0.0_r30
  cd Car

  # FETCH upstream patch
  git fetch https://android.googlesource.com/platform/packages/services/Car refs/changes/83/3037383/2 && git cherry-pick FETCH_HEAD
  # Push to Horizon SDV Gerrit repo
  git push origin horizon/android-14.0.0_r30
  cd -
  rm -rf Car
  ```
  
<hr width="50%"></details>

[ ============================================================================= ]::

## <span style="color:#335bff">WORKLOAD SETUP<a name="workload"></a></span>

The following table outlines which workload setup tasks are requried for each Workload area (follow links to instructions below the table); for each workload, the tasks need to be carried out in the order given.

Note that for each workload area, a Docker image needs to be created using a _Docker Image Template_ Jenkins job; the existence of this Docker image is a requirement for all other jobs.

| Workload Area | Setup Task | Comment |
| ----- | ----- | ----- |
| All | [Setup of Jenkins Jobs](#workloads-jenkins-jobs) | Perform for all workstations <br> or individually for each |
| Android | [Create Docker Image Template](#workloads-docker-image-template) | Perform specifically for this workstation | 
|   | [Build Cuttlefish Instance Template(s)](#workloads-android-cuttlefish-instance-template) | Unique to this workstation <br> - required for all Android test jobs|
|   | [Warm Build Caches (optional)](#workloads-android-warm-build-caches) | Unique to this workstation |
|   | [AOSP Mirror (optional)](#workloads-android-mirror) | Unique to this workstation |
| OpenBSW | [Create Docker Image Template](#workloads-docker-image-template) | Perform specifically for this workstation | 
| Cloud Workstations | [Create Docker Image Template](#workloads-docker-image-template) | Perform specifically for this workstation | 
|   | [Create Cluster](#workloads-cloud-workstations-create-cluster) | Unique to this workstation |
|   | [Create New Workstation Configuration](#workloads-cloud-workstations-configs) | Unique to this workstation |
|   | [Publish IDE/Container image(s)](#workloads-cloud-workstations-publish-images) | Unique to this workstation |
|   | [Create New Workstation(s)](#workloads-cloud-workstations-create-workstations) | Unique to this workstation |

[ ========================== ]::

<details id="workloads-jenkins-jobs">
<summary>All Workloads - Setup of Jenkins Jobs</summary><hr width="50%">

  The [_Seed Workloads_](../seed.md) Jenkins job needs to be run after initial deployment to populate the Jenkins folders and jobs in all required workload areas; it can be run to populate each workload area individually or it can populate them all at once. See [pipeline_guide.md](pipeline_guide.md) for more info.

  <b>Step 1:</b> Run with parameter `SEED_WORKLOAD` set to `none` to ensure that job parameters are correctly updated.

  <b>Step 2+:</b> Run with parameter `SEED_WORKLOAD` set to `all` or to the desired individual workload (e.g. `android`, `openbsw`, `utilities`, etc.). 

<hr width="50%"></details>

[ ========================== ]::

<details id="workloads-docker-image-template">
<summary>All Workloads - Create Docker Image Template</summary><hr width="50%">

For each workload being used, a Docker container image needs to be built; this will be used by Kubernetes to execute various pipeline jobs (e.g. building Android/OpenBSW targets, running Cloud Workstations).

The Jenkins job _Docker Image Template_ is used to perform this creation task; a version of the job is provided in the _Environment_ folder in each workload area and needs to be run once for each workflow (see documentation for [Android](../android/environment/docker_image_template.md), [OpenBSW](../openbsw/environment/docker_image_template.md), [Cloud Workstations](../cloud-workstations/environment.md) jobs)

The required Jenkins job should be run with the _NO_PUSH_ parameter unticked; this ensures that the docker image that is generated is pushed to the registry. The job needs to be run only once for each workload area.

>[!NOTE] When re-building Docker Image Templates after an upgrade, the old templates do not need to be explicitly deleted first; new runs will over-write the _latest_ tags.

<hr width="50%"></details>

[ ========================== ]::

### <span style="color:#335bff">Android<a name="android"></a></span>

<details id="workloads-android-cuttlefish-instance-template">
<summary>Android - Build Cuttlefish Instance Template(s)</summary><hr width="50%">

<details id="workloads-android-cuttlefish-instance-template-notes"">
<summary><b>Important Notes - please read</b></summary><hr width="25%">

- **Rebuilding after Upgrade** When re-building Cuttlefish Instance Templates after an upgrade, the old templates should be deleted first by performing a run of the same job (with the same parameters as would be used for creation) but selecting the `DELETE` build parameter. Only when this run is complete should a 'creation' run be executed.

- **Parallel Execution:** This job can be run concurrently with other setup stages, such as [Gerrit Setup](#gerrit-setup) and [Warm Build Caches](#workloads-android-warm-build-caches), allowing for efficient use of resources and minimising overall provisioning time.

- **Execution Time** Instance templates takes a significant amount of time (~1 hour) to create. This is due to the install of android-cuttlefish, the download & install of Android 14, 15 and 16 CTS, as well as GCP gcloud CLI commands which can complete before the change is visible, thus requiring some mandatory delays.

- **Jenkins Cloud Mapping** Each instance template that is created needs to be mapped (via label) to a Jenkins cloud whose template URL points at the new instance template. It is the Jenkins cloud that is referenced in the Jenkins test pipelines, so it is essential to ensure that each instance template has a working Jenkins cloud linking to it. See [appendix](#appendix-jenkins-clouds) for more info.

<hr width="25%"></details>

Google Compute Engine (GCE) instance templates are required by the Android test pipelines to provision [Cuttlefish-ready](https://source.android.com/docs/devices/cuttlefish) and [CTS-ready](https://source.android.com/docs/compatibility/cts) cloud instances.

The Jenkins jobs [_CF Instance Template_](../android/environment/cf_instance_template.md) and _CF Instance Template ARM64_ (both located in _Android Workflows_ → _Environment_) are used to perform this creation task.

At least one Cuttlefish Instance Template should be created for each architecture (i.e. arm or x86); it is recommended to build with the 'main' and _latest_ [cuttlefish tags/branches](https://github.com/google/android-cuttlefish/tags) as follows. The job needs to be run only once for each architecture/cuttlefish combination.

| Job | Parameters |
| --- | --- |
|  `CF Instance Template`| `ANDROID_CUTTLEFISH_REVISION` = `main` |
| `CF Instance Template` | `ANDROID_CUTTLEFISH_REVISION` =  latest tag/branch (e.g. `v1.55.3`) |
| `CF Instance Template ARM64` | `ANDROID_CUTTLEFISH_REVISION` = `main` |
| `CF Instance Template ARM64` | `ANDROID_CUTTLEFISH_REVISION` =  latest tag/branch (e.g. `v1.55.3`) |

_Note: the default naming mechanism can be used_

<details id="workloads-android-cuttlefish-instance-template-view"">
<summary>View Instance Templates</summary><hr width="25%">

- From the Browser enter your Google Cloud Platform project welcome page
- Select `Compute Engine` → `Instance Templates`
- Verify that the required templates have been created.
  - `instance-template-cuttlefish-vm-main`
  - `instance-template-cuttlefish-vm-v1553`
  - `instance-template-cuttlefish-vm-main-arm64`
  - `instance-template-cuttlefish-vm-v1553-arm64`

<hr width="25%"></details>

<hr width="50%"></details>

[ ========================== ]::

<details id="workloads-android-warm-build-caches">
<summary>Android - Warm Build Caches (optional)</summary><hr width="50%">

>[!NOTE:] If using private Gerrit repos, the [_Gerrit Setup_](#gerrit-setup) task must be completed before running this job.

The [_Warm Build Caches_](../android/environment/warm_build_caches.md) job ( located in _Android Workflows_ → _Environment_) is provided as an aid for improving on build times; as such, it is an optional (but recommended) setup step.

Build times are improved by pre-warming the build caches (i.e. the persistent volumes) ahead of time; this is done by running a number of standard builds against the defined manifest and revision, thus filling the build cache on the persistent volumes (PVC).

Perform the following actions for each branch that is required for building:
  - Click on `Build with Parameters`
  - Accept default `ANDROID_MANIFEST_URL` (i.e. gerrit manifest URL)
  - Set `ANDROID_REVISION` as required (e.g. `horizon/android-16.0.0_r2`)
  - Select `Build`
  - Repeat job N times (max 20 due to kubernetes cap):
    - N is the number of persistent volumes you want warmed - set it to max allowed number of parallel builds plus 25%
    - Repeat the jobs immediately to ensure they run in parallel; the _Replay_ option can be used to do this (select from a previous run's options list)
    - In order to reduce lab build times, this will create N x 2TB pd-balanced persistent volumes pre-warmed with the required builds

<hr width="50%"></details>

[ ========================== ]::

<details id="workloads-android-mirror">
<summary>Android - AOSP Mirror (optional)</summary><hr width="50%">

Using an NFS-based mirror in Google Cloud Platform (GCP) - particularly the AOSP (Android Open Source Project) mirror - can be used to accelerate `repo sync` operations in Android builds.

Refer to the Jenkins page at _Android Workflows_ → _Environment_ → _Mirror_ as well as [this](../android/environment/mirror.md) document for instructions on how to set up, manage and use such a mirror.

<hr width="50%"></details>

[ ========================== ]::
### <span style="color:#335bff">Cloud Workstations<a name="cloud-workstations"></a></span>

<details id="workloads-cloud-workstations-create-cluster">
<summary>Cloud Workstations - Create Cluster</summary><hr width="50%">

Run the _**Create New Cluster**_ Jenkins job (located in _Cloud Workstations_ → _Cluster Admin Operations_) to create a new cluster for GCP Cloud Workstations in your existing GCP project. This should be run only once.
 
See [Cluster Admin Operations](../cloud-workstations/cluster_admin_operations.md) documentation for more info. 

<hr width="50%"></details>

[ ========================== ]::

<details id="workloads-cloud-workstations-configs">
<summary>Cloud Workstations - Create New Workstation Configuration</summary><hr width="50%">

Run the _**Create New Configuration**_ Jenkins job (located in _Cloud Workstations_ → _Config Admin Operations_) to create a new Workstation Configuration which acts as a blueprint for creating individual Cloud Workstations.

The other jobs in _Cloud Workstations_ → _Config Admin Operations_ can be used to manage workstation configurations. See [Config Admin Operations](../cloud-workstations/config_admin_operations.md) documentation for more info.


<hr width="50%"></details>

[ ========================== ]::

<details id="workloads-cloud-workstations-publish-images">
<summary>Cloud Workstations - Publish IDE/Container image(s)</summary><hr width="50%">

For each type of workstation that is required (e.g. general purpose IDE, Android Studio-based IDE, specialized IDE for AOSP), a Docker container image needs to be built.

The Jenkins jobs in _Cloud Workstations_ → _Workstation Images_ perform these creation tasks; a separate job is provided for each supported workstation type. 

The jobs should be run with the _NO_PUSH_ parameter unticked (to ensure that the docker image that is generated is pushed to the registry). Each job needs to be run only once.

See [Workstation Images](../cloud-workstations/workstation_images.md) documentation for more info.

<hr width="50%"></details>

[ ========================== ]::

<details id="workloads-cloud-workstations-create-workstations">
<summary>Cloud Workstations - Create New Workstation(s)</summary><hr width="50%">

Before a workstation can be used a new workstation instance needs to be created/provisioned using a specified configuration.

The Jenkin jobs _Create New Workstation_ (located in _Cloud Workstations_ → _Workstation Admin Operations_ performs this task; users can also be added during the creation process. This job needs to be run only once for each unique workstation.

The other jobs in _Cloud Workstations_ → _Workstation Admin Operations_ can be used to manage workstation instances. See [Workstation Admin Operations](../cloud-workstations/workstation_admin_operations.md) documentation for more info.

<hr width="50%"></details>

>[!NOTE] Refer to `docs/guides/mcp_setup.md` for details on how to setup and use MCP servers in cloud workstations.

[ ============================================================================= ]::


## <span style="color:#335bff">APPENDICES<a name="appendices"></a></span>

[ ========================== ]::

<details id="appendix-jenkins-clouds">
<summary>Jenkins Clouds & GC Instance Templates</summary><hr width="50%">

In order for an instance template to be used by Jenkins to spin up a virtual machine, it needs to be mapped (via label) to a Jenkins cloud whose template URL points at the saved instance template.
- Example: in an Android test pipeline job, set the `JENKINS_GCE_CLOUD_LABEL` parameter to a cloud label (e.g. `cuttlefish-vm-main`) in order to use the desired instance template (e.g. `instance-template-cuttlefish-vm-main`). 

Existing Jenkins clouds can be seen in the Jenkins UI: `Settings` &rarr; `Clouds`.
There are some clouds which are defined by default.

If you create an instance template whose name does not match a predefined Jenkins cloud (see Jenkins UI: `Settings` &rarr; `Clouds`) then you need to create a new cloud with a link to your new instance template; this will ensure that the instance template you create can be used in Jenkins jobs. 

Example: Creation of a new cloud is likely to be necessary if you use the Android [Cuttlefish Instance Template](../android/environment/cf_instance_template.md) job with parameters that match either of the following scenarios:
  - using a custom name for `CUTTLEFISH_INSTANCE_NAME` which does not match a predefined Jenkins cloud
  - using a value for `ANDROID_CUTTLEFISH_REVISION` which will result in a name which does not match a predefined Jenkins cloud (e.g. private branch names) - see [naming convention](../android/environment/cf_instance_template.md#private-repo-and-branch-eg-horizonmain--artifacts-and-jenkins-gce). 
 
To set up and use a new Jenkins Cloud see instructions [here](../android/environment/cf_instance_template.md#private-repo-and-branch-eg-horizonmain--artifacts-and-jenkins-gce) on how to _Wire Jenkins GCE Cloud_ and _Point test jobs at the new cloud_.

<hr width="50%"></details>

[ ========================== ]::



