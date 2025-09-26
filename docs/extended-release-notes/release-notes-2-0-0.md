# Release Notes - SDV Tooling - Release 2.0.0

| | |
|-|-|
| __Platform__ | Horizon SDV |
| __Date__ | 29.08.2025 |
| __Version__ | Release 2.0.0 |
| __Contributors__ | @Wojtek Kowalski @Wojciech Kobryn @Prashanth Habib Eshwar @Bavya Sakthivel @Lynn Sheehy @Dave M. Smith @Lukasz Domke @Adireddi Keerthi @Colm Murphy @Marta Kania @Akshay Kaura |


## Summary

Horizon SDV 2.0.0 extends Android build capabilities with the integration of Google ABFS and introduces support for Android 15. This release also adds support for OpenBSW, the first non-Android automotive software platform in Horizon. Other major enhancements include Google Cloud Workstations with access to browser based IDEs Code-OSS, Android Studio (AS), and Android Studio for Platforms (ASfP). In addition, Horizon 2.0.0 delivers multiple feature improvements over Rel. 1.1.0 along with critical bug fixes.


## New Features

| Issue key | Summary |
| --------- | ------- |
| TAA-8 | ABFS for Build Workloads |
| TAA-9 | Cloud Workstation integration |
| TAA-375 | Android 15 Support |
| TAA-381 | Add OpenBSW build targets |
| TAA-915 | Cloud Android Orchestration - Part 1 |
| TAA-623 | Management of Jenkins Jobs using CasC |
| TAA-462 | Kubernetes Dashboard |
| TAA-717 | Multiple pre-warmed disk pools |
| TAA-596 | Jenkins RBAC |
| TAA-611 | Argo CD SSO |
| TAA-837 | Access Control script deployment in R.2.0.0 |

---

### TAA-8 | ABFS for Build Workloads

#### Release Note

#### Summary
The Horizon-SDV platform now integrates Google's **Android Build Filesystem (ABFS)**, a filesystem and caching solution designed to accelerate AOSP source code checkouts and builds.

This feature introduces new Jenkins workload pipelines to deploy and manage the ABFS infrastructure.

> **Important:** This is a licensed feature available under an **Early Access Program (EAP)**. Please contact your Google representative for details on licensing and eligibility before proceeding.

#### **Key Benefits**
* **Accelerated AOSP Builds:** Leverage Google's unique filesystem and caching to significantly reduce compilation times.
* **Faster Source Code Syncs:** Dramatically decrease the time required for `repo sync` operations.

#### **New Features**

* **ABFS Infrastructure Provisioning**
    * The platform now includes Jenkins workload pipelines that execute Terraform workflows to deploy and manage the core components of the ABFS infrastructure:
        * **ABFS Server:** A VM instance that serves the filesystem to your build clients.
        * **ABFS Uploader:** A set of VM instances responsible for seeding the initial AOSP source code into the ABFS cache.

* **Management via Jenkins Pipelines**
    * A new set of Jenkins jobs under `Android Workflows > Environment > ABFS` is now available to manage the ABFS ecosystem.
    * **Environment Setup**: Pipelines to build the required Docker container images for the ABFS Jenkins agents and uploader instances.
    * **Server & Uploader Deployment**: Pipelines that use Terraform workflows to deploy the ABFS Server and Uploader VMs.
    * **AOSP Build Integration**: The new `Android Workflows > Builds > AAOS Builder ABFS` job allows developers to run builds using the ABFS filesystem.

#### **Getting Started with ABFS**

##### **Prerequisites**
Setup for this feature is an admin-led process that requires coordination with Google.

1.  **Early Access Program (EAP):** You must be enrolled in the ABFS EAP and receive a license string from Google.
2.  **Service Account:** A dedicated service account (e.g., `abfs-server`) must be created in your GCP project. This account requires a specific set of IAM roles for licensing and image retrieval.  
    For the complete list of required roles, please refer to the setup guide at:  
    `docs/workloads/android/abfs.md`

##### **Initial Setup (Admin Workflow)**
The initial setup is a multi-step process involving applying the license secret, seeding Jenkins, building images, and deploying the infrastructure.  
For the detailed, step-by-step instructions, please refer to the setup guide at:  
`docs/workloads/android/abfs.md`

##### **Running an ABFS-Powered Build (Developer)**
Once the admin setup is complete, use the `Android Workflows > Builds > AAOS Builder ABFS` pipeline to run your AOSP builds against the accelerated filesystem.

#### **Known Issues & Action Required**
* **Action Required: Version Correlation:** The ABFS Uploader job generates a file named `abfs_repository_list.txt`. It is crucial to review the output of this file and update the `Seed Workload` job's values for `ABFS_VERSION` and `ABFS_CASFS_VERSION` accordingly. This ensures you are using the latest compatible versions provided by Google.
* **EAP Limitations:** As an EAP feature, ABFS is still evolving. Please reach out to your Google representative for details on current limitations.

---

### TAA-9 | Cloud Workstation integration

#### Release Note

#### Summary
The Horizon-SDV platform now includes **GCP Cloud Workstations**, enabling users to launch pre-configured, and ready-to-use development environments directly in browser.

This feature automates the entire provisioning and management process through a new set of Jenkins workload pipelines.

#### Key Benefits
- **Zero Local Setup:** Instantly access a pre-configured cloud IDE with all tools and dependencies.
- **Scalable Performance:** Run builds and emulators on powerful cloud machines matched to your task.
- **Consistent Workspaces:** Enjoy standardized environments for every developer, eliminating setup issues.
- **Access Anywhere:** Secure, browser-based workstations available from any location.

#### New Features

* **Browser-Based Development Environments**
  * You can now build and launch standardized IDEs directly in your browser.
  * The following environments are now available:
    * **Horizon Code OSS**: A lightweight, general-purpose IDE based on open-source VS Code. The container image for this environment extends the public base image available [here](https://us-central1-docker.pkg.dev/cloud-workstations-images/predefined/code-oss:latest).
    * **Horizon Android Studio (AS)**: The standard IDE for developing Android applications. The container image for this environment extends the public image template provided by Google [here](https://github.com/GoogleCloudPlatform/cloud-workstations-custom-image-examples/tree/main/examples/images/android-open-source-project/android-studio-for-platform).
    * **Horizon Android Studio for Platform (ASfP)**: The specialized IDE for AOSP and core Android OS development. The container image for this environment extends the public image template provided by Google [here](https://github.com/GoogleCloudPlatform/cloud-workstations-custom-image-examples/tree/main/examples/images/android/android-studio).

* **Automated Lifecycle Management via Jenkins Pipelines**
  * A full suite of Jenkins jobs is now available to manage the entire Cloud Workstations ecosystem, from the underlying infrastructure to individual developer instances.
  * **Platform Setup (One-Time Admin Tasks)**
    * **Prerequisites**:
      * **Role Based Strategy Permissions and Seeding Workload**: To complete the setup, please refer to the following documentation for instructions on seeding the workload, configuring permissions in Keycloak and Jenkins, and other necessary steps:
        > TAA-623: Management of Jenkins using CasC | Action Required:
      * For this setup, make sure to run the `Seed Workloads` job with parameter `cloud-workstations` (or `all` parameter, if you want to seed all other jobs as well).
      * On GCP, make sure you have the **Cloud Workstations API** enabled.

    * **Build Pipeline Environment**:
      * `Cloud Workstations > Environment > Docker Image Template`
      * This Pipeline builds the Docker image that acts as an execution environment for all operation pipelines. 
      * The image is published to GCP Artifact Registry at `horizon-sdv/cloud-ws-workloads/env`.
      * Installs tools required for infra provisioning (Terraform) and lifecycle management (`gcloud`).
      
    * **Build Workstation Images**:
      * `Cloud Workstations > Workstation Images`
      * Provides a set of pipelines that build pre-set Docker images used by Workstations as containers. 
      * The images are published to GCP Artifact Registry under `horizon-sdv/cloud-ws-images/*`.

    * **Create Cluster**:
      * `Cloud Workstations > Cluster Admin Operations > Create Cluster`
      * Provisions the top-level Workstation Cluster that houses all resources.

  * **Cluster Management (For Admins)**
    * `Cloud Workstations > Cluster Admin Operations`
    * Provides a set of two admin-level pipelines to manage the Cloud Workstation Cluster - which is a logical grouping of Cloud Workstation resources on GCP:
      * *Create Cluster*
      * *Delete Cluster*
    * All Cluster properties are **preset**.
    * At any time, only **one** cluster can exist.

  * **Configuration Management (For Admins)**
    * `Cloud Workstations > Config Admin Operations`
    * Provides a set of six admin-level pipelines to manage the Cloud Workstation Configurations - which are like blueprints used by Workstations:
      * *Create New Configuration*
      * *Delete Existing Configuration*
      * *Update Existing Configuration*
      * *List Configurations*
      * *Get Configuration Details*
      * *List Workstations by Configuration*

  * **Workstation Management (For Admins)**
    * `Cloud Workstations > Workstation Admin Pipelines`
    * Provides a set of six admin-level pipelines to manage the Workstations:
      * *Create New Workstation*
      * *Delete Existing Workstation*
      * *List Workstations*
      * *Get Workstation Details*
      * *Add Users to Workstation*
      * *Remove Workstation Users*
  
  * **Workstation Usage (For All Users)**
    * `Cloud Workstations > Workstation User Pipelines`
    * Provides a set of five user-level pipelines to manage lifecycle of Workstations:
      * *Get Workstation Details*
      * *List Workstations*
      * *Get Workstation Configuration*
      * *Start Workstation*
      * *Stop Workstation*

#### Getting Started: Your First Workstation

  * **Prerequisite Admin Tasks**
    * **One-time Setup**: Ensure all the admin tasks listed in section "**Platform Setup (One-Time Admin Tasks)**" above have been completed.
    * **Create a Configuration**: Use the `Config Admin Operations > Create New Configuration` job to define a template that uses the image built in the previous step.
  * **Create your Workstation (User)**: Run the `Workstation Admin Operations > Create New Workstation` job, selecting the configuration created in previous step.
  * **Start your Workstation (User)**: Run the `Workstation User Operations > Start Workstation` job. The secure **URL** to access your IDE in the browser will be provided in the Jenkins console output.

---

### TAA-375 | Android 15 Support

#### Release Note

##### Android 15 Support
We previously supported Android 15 in Horizon-SDV but by default Android 14 was selected. In this release, Android 15 android-15.0.0_r36is now the default revision.

#####  Changes:
- **AAOS Builder:** default AAOS_REVISION moved to horizon/android-15.0.0_r36

Raspberry Vanilla updates: support for android-15.0.0_r36 and android-14.0.0_r30, including both RPi4 and RPi5 hardware.

- **CTS Builder:** default AAOS_REVISION moved to horizon/android-15.0.0_r36

- **Gerrit and Warm Build Caches:** now support android-15.0.0_r36 to dynamically determine the codename/build tag for the target to build, e.g. aosp_cf_x86_64_auto-bp1a-userdebug

- **Development Instance:** defaulted to Android 15 disk pool, user may override.

- **Cuttlefish Instance Templates:** debian upgrade, Cuttlefish host package updates and updated CTS test harness for Android 15 and 14.

- **Docker Image Template:** packages upgrades for Android 15 RPi builds.

##### Action Required:
To ensure a smooth transition it is advisable users update their build image and cuttflefish instance templates.

###### Docker Image Template update:
In Jenkins select Android Workflows → Environment → Docker Image Template:

Deselect NO_PUSH

Select Build

###### Cuttlefish Instance Template update:
In Jenkins select Android Workflows → Environment → CF Instance Template:

Delete the oldv1.1.0 instance template:

Set ANDROID_CUTTLEFISH_REVISION to v1.1.0
Select DELETE
Select Build

- R2.0.0 no longer references this version and as such, this will delete the instance template and disk image for v1.1.0 and save on cost.

Upgrade the main instance template:

Set ANDROID_CUTTLEFISH_REVISION to main

Select Build

Create the new v1.18.0 instance template:

Set ANDROID_CUTTLEFISH_REVISION to v1.18.0

Select Build

Your platform is now updated for latest versions of build images and instance templates to support Android 15.

---

### TAA-381 | Add OpenBSW build targets

#### Release Note
Eclipse Foundation OpenBSW Workload
As part of the R2.0.0 delivery, a new workload has been introduced to support the Eclipse Foundation OpenBSW within the Horizon SDV platform. This workload enables users to work on the OpenBSW stack for build and testing.

#### Workload Processes:

The OpenBSW workload automates the following key processes:

- Build: Compiles and builds OpenBSW, including:
    - Unit tests
    - Code coverage
    - POSIX targets
    - Hardware targets
- Test: Executes tests on the POSIX platform
- Artifact Storage: Stores build reports, artifacts, and other outputs in:
    - Cloud storage
    - Local Jenkins storage

####  Workflow and Pipeline Folder Structure:

The workload follows the same principles as the Android workload and consists of the following pipeline folder structure:

- OpenBSW Workflow/Environment: Contains administrative jobs that create Docker container build images and provide additional tools for users.

- OpenBSW Workflow/Builds: Supports building and running OpenBSW unit tests, code coverage, and building POSIX and Hardware targets.

- OpenBSW Workflow/Tests: Supports testing the OpenBSW POSIX application on the POSIX host platform.

This new workload provides a comprehensive environment for users to work on the OpenBSW stack within the Horizon SDV platform.

#### Prerequisites
##### Role Based Strategy Permissions and Seeding Workload
To complete the setup, please refer to the following documentation for instructions on seeding the workload, configuring permissions in Keycloak and Jenkins, and other necessary steps:

- TAA-623: Management of Jenkins using CasC | Action Required: 

##### Creating Docker Image Template
Once the workload is created, you need to create a Docker container image that will be used by the builds and tests jobs. To do this:

1. Go to Jenkins → OpenBSW Workflows → Environment → Docker Image Template. 

2. Choose to create the image using the default values or override the tools defined within this job

3. Ensure that the NO_PUSHoption is deselected, so the resulting image is stored in the artifact registry and accessible to other jobs.

By completing these steps, you will have successfully created the Docker image template required for the OpenBSW workload.

#### Building OpenBSW Targets
With the Docker image created, you can now start building and testing OpenBSW.

The OpenBSW Workflows/Builds/BSW Builder job offers the following build options:

- Unit Tests:
    - Build unit tests
    - Run unit tests (all or individual test library)
    - Generate code coverage reports
- Platform Builds:
    - POSIX Platform builds
    - NXP S32K148 Platform builds

#### Testing the POSIX Target
After running the POSIX build pipeline, you can test the resulting application binary using the OpenBSW Workflows/Tests/POSIX job.

This job provides the following testing options:

MTK Connect: Establishes a direct connection to the POSIX host platform.

Application Launch: Choose to automatically launch the POSIX application or launch it manually.

#### Additional Resources
For more information on this update and how to use the new job, please refer to the following documentation:

- docs/workloads/guides/pipeline_guide.md

- docs/workloads/seed.md

- docs/workloads/android/guides/developer_guide.md

- docs/workloads/openbsw/environment/docker_image_template.md

- docs/workloads/openbsw/environment/dev_instance.md

- docs/workloads/openbsw/environment/delete_mtkc_testbench.md

- docs/workloads/openbsw/builds/bsw_builder.md

- docs/workloads/openbsw/tests/posix.md

- docs/workloads/openbsw/guides/developer_guide.md

- Role Based Strategy and seeding the new OpenBSW workload

- TAA-623: Management of Jenkins using CasC | Action Required: 

---

### TAA-915 | Cloud Android Orchestration - Part 1

#### Release Note

#### Cuttlefish Virtual Devices (CVD) and Compatibility Test Suite (CTS) Enhancements in R2.0.0

In the latest release, R2.0.0, we've introduced significant improvements to Cuttlefish Virtual Devices (CVD). These enhancements include increased support for a larger number of devices, optimised device startup processes, and a more robust recovery mechanism. Additionally, we've updated the Compatibility Test Suite (CTS) Test Plans and Modules to ensure seamless integration and compatibility with CVD.

#### Summary of Changes:

#### CTS Execution Job
- `Android Workflows → Tests → CTS Execution`

**Key Changes:**
1. Test Plan and Module Updates: Defaulted to support DEQP (drawElements Quality Program) tests for Android 15.
2. `CTS_TESTPLAN`: Changed to `cts-system-virtual`
3. `CTS_MODULE`: Changed to `CtsDeqpTestCases`
4. User Selection: Users can choose their preferred test plan and module.

**Important Notes:**
- `cts-system-virtual` and `CtsDeqpTestCases`tests take a significant amount of time (up to 6 hours with the default machine type).
- Users may want to change the machine type when creating the Cuttlefish Instance Template.
- `CTS_TIMEOUT` increased to 600 minutes to accommodate longer test execution times.


#### CF Instance Template job
- `Android Workflows → Environment → CF Instance Template`

**Key Changes:**
1. Due to the changes to support DEQP (drawElements Quality Program) and the length of time it takes to execute the tests, the `MAX_RUN_DURATION` has been increased to 10 hours. This ensures the worst case the instance will remain running for is 10 hours before being forcibly terminated. If test jobs finish within that time, the instance will be terminated normally as per Jenkins pipeline management of GCE VM nodes.
2. Users must regenerate their VM instance templates to have this change applied otherwise instances will terminate before the test execution has completed.

#### CVD Launcher job
- `Android Workflows → Tests → CVD Launcher`

**Key Changes:**
1. The method to start CVD instances has changed. Previously the `launch_cvd` command from the AOSP CF built `cvd-host_package.tar.gz` is no longer used. Instead it now uses `/usr/bin/cvd` that is provided by the Android Cuttlefish host debian packages built and installed from [android-cuttlefish.git](https://github.com/google/android-cuttlefish "https://github.com/google/android-cuttlefish") during the CF Instance Template build.
2. This new mechanism is more reliable than `launch_cvd` and results in the requested devices matching the booted devices. The implementation still has a recovery retry mechanism, i.e requested != booted then retry, and this has proven to be far more reliable than previous use of `launch_cvd` which could still end up with less devices booted.
3. The number of CVD devices in Cuttlefish was limited to 10, `NUM_INSTANCES=10`. Users may wish to create additional instances, as such the cuttlefish instance resources are updated when > 10 is requested and the service is restarted. This is useful considering the additional test support in Android 15, including DEQP.

#### Development Build Instance
- `Android Workflows → Environment → Development Build Instance`

**Key Changes:**
1. This replaces `Android Workflows → Environment → Development Instance`.
2. It supports the capability to create the k8s build instance and connect to the host using MTK Connect.

#### Development Test Instance
- `Android Workflows → Environment → Development Test Instance`

**Key Changes:**
1. This is a new pipeline job allowing users to create a test instance.
2. It supports the capability to create the GCE CF VM instance and connect to the host using MTK Connect.

#### Action Required:
**Update Cuttlefish Instance Templates**

**Important:** Due to changes in this release, users must recreate their Cuttlefish Instance Templates.

**Step-by-Step Instructions:**

1. Ensure the `Seed Workloads` job has been run for `android`
2. Go to `Android Workflows → Environment → CF Instance Template`.
3. Select `ANDROID_CUTTLEFISH_REVISION` to the revision of CF to build, choose a suitable `MACHINE_TYPE` supported in your region and select `Build` to recreate the templates.

**Update Development Instance**

**Important**: Due to changes in this release, `Development Instance` job has been replaced by `Development Build Instance` job.

**Step-by-Step Instructions:**

1. Go to `Android Workflows → Environment → Development Instance`.
    - Click `Delete pipeline` to manually remove the old job (note: job seed does not automatically delete old jobs).
2. New build job: `Android Workflows → Environment → Development Build Instance`.

---

### TAA-623 | Management of Jenkins Jobs using CasC

#### Release Note
##### Management of Jenkins Jobs using CasC: Seeding Workloads
###### Jenkins Pipeline Job Configuration Update

In R1.1.0, Jenkins pipeline jobs were configured using the jenkins.yaml file as a Configuration as Code (CasC) approach. However, this method proved to be inconvenient for developers to update parameters, descriptions, and add new jobs.

###### New Approach

To address this issue, the CasC configuration has been updated to include a single job in the jenkins.yaml file, which is automatically started on each Jenkins restart. This job provides the "Build with Parameters" option, allowing users to populate the workload of their choice or all workloads.

###### Job Functionality

This job performs the following key functions:

1. Folder and Job Discovery: It searches for folder Groovy files that define the Jenkins pipeline folder structure and job Groovy files that define parameters, descriptions, and other job configurations.

2. Environment Variable Replacement: It parses the files for any environment variables that need to be replaced with their actual values. This replacement is necessary to avoid the need for explicit Script Approval, which is typically required for Groovy methods like getProperty.

By performing these functions, the job streamlines the process of managing Jenkins pipeline jobs and eliminates the need for manual Script Approval and better still, maintaining parameters local to the pipeline job rather than in the central jenkins.yaml file which can be unwieldy to maintain.

###### Benefits:

- **Decoupling Jenkins Configuration from GitOps**
By separating Jenkins configuration from GitOps (ArgoCD), developers can manage the Seed job, folders, and pipeline jobs independently, without affecting the GitOps workflow. This approach offers several benefits:

    - Simplified Management: Developers can manage workloads and pipeline jobs without relying on GitOps, reducing the complexity of managing multiple systems.

    - Improved Error Handling: If mistakes occur while updating the jenkins.yaml file, using this new approach allows users to more easily identify the issue by checking the Jenkins build console log, without needing access to Kubernetes logs.

    - Enhanced Pipeline Deployment: The separation of concerns ensures that Jenkins is always configured correctly, and pipeline deployment is manageable and transparent.

    - Increased Visibility: Users can verify the accuracy of their Seed job and job definitions directly from the Jenkins build console log, eliminating the need to access Kubernetes logs.

Overall, this approach streamlines Jenkins management, reduces errors, and improves the overall developer experience albeit they must either approve themselves, or seek approval from a team member in a position of responsibility.

###### Additional Resources

For more information on this update and how to use the new job, please refer to the following documentation:

- `docs/workloads/guides/pipeline_guide.md`
- `docs/workloads/seed.md`
- `docs/workloads/android/guides/development_guide.md`

##### Action Required:
The addition of the Role Based Strategy Plugin to Jenkins requires users perform some additional prerequisite setup steps.

1. Keycloak:

    - Users must be added to one of the following Keyclock Groups:

|Keycloak Group | Jenkins Role | Access Level|
|-|-|-|
|horizon-jenkins-administrators|Global: Admin|Full admin access|
|horizon-jenkins-workloads-developers|Item: workloads-developers|Full build/config rights for Workloads|
|horizon-jenkins-workloads-users|Item: workloads-users|Limited build access for Workloads|

2. Jenkins:

- To seed the jobs, users must be added to the appropriate roles in: Jenkins → Manage Jenkins → Manage and Assign Roles → Assign Roles. Those roles are:

    - Global:
        - horizon-jenkins-administrators
        - horizon-jenkins-workloads-developers

    - Items:

        - workloads-developers
        - workloads-users

- Add the user to appropriate Global and Item Roles:

    - In Global Roles select Add User and select the appropriate Keycloak Group.

    - In Item Roles select Add User and select the appropriate Jenkins Item Role.

    - Select Save



> [!WARNING]  
> Persistence across restarts
> 
> These values do not persist across a Jenkins restart and as such we recommend > if using the Role Based Strategy plugin, then users be added `togitops/env/> stage2/templates/jenkins.yaml`.
> 
> 1. Update `authorizationStrategy` -> `roleBased` -> `roles` -> `global`: 
> 
>       - Add the user to the respective group entry, e.g.
> `- user: "john.example.doe@accenture.com"`
> 
> 2. Update `authorizationStrategy` -> `roleBased` -> `roles` -> `items`:
> 
>       - Add the user to the respective group entry, e.g.
> `- user: "jane.example.doe@accenture.com"`
> 
> Alternatively you will have to manually configure Jenkins again.

> [!NOTE]  
>Disabling the plugin
>
>If user wishes to disable the plugin, then remove the plugin and configuration `from gitops/env/stage2/templates/jenkins.yaml`:
>
>1. Remove the plugin from the additionalPlugins section within the yaml file:
>
>   - `role-strategy:743.v142ea_b_d5f1d3`
>
>2. Replace all within `authorizationStrategy` with the following default values:
>
>
>```
>            authorizationStrategy: |-
>              loggedInUsersCanDoAnything:
>                allowAnonymousRead: false
>```


### TAA-462 | Kubernetes Dashboard

#### Release Note

#### Summary
The Horizon platform now includes the **Headlamp** application, a web-based tool to browse Kubernetes resources and diagnose problems.

#### Key Benefits
* **Resource Visibility:** Provides a UI to view Kubernetes resources within the Horizon platform.  
* **Problem Diagnosis:** Helps developers and operators diagnose cluster issues more easily.  

#### New Features
* **Application Access**
  * The Headlamp application is now available under the `/headlamp/` prefix.  
  * **Important:** The prefix must be `/headlamp/` (with a trailing slash), not `/headlamp`.  

* **Authentication**
  * Authentication to Headlamp is currently supported **only via token-based login**.  

#### Getting Started with Headlamp

##### Generate Access Token
1. Create a token using the following command:  
   ```bash
   kubectl create token headlamp-admin -n headlamp
   ```
2. Use the generated token to log into the Headlamp application via `/headlamp/`.
  ![headlamp_auth_menu.png](/docs/images/headlamp_auth_menu.png)
3. Browse and edit Kubernetes resources on dashboard.
  ![headlamp_main_dashboard.png](/docs/images/headlamp_main_dashboard.png)

##### Using the Application
Once logged in, you can browse and edit Kubernetes resources directly through the Headlamp UI.

#### Known Limitations
Authentication: Only token-based authentication is available at this time.

---

### TAA-596 | Jenkins RBAC

#### Release Notes
Jenkins has been configured with RBAC capability using the [Role-based Authorization Strategy](https://plugins.jenkins.io/role-strategy/) (ID: `role-strategy`) plugin.

##### Changes
1. Added New realm roles, groups and client roles on Keycloak for user access management for Jenkins.

2. Installed and configured the role-strategy on Jenkins using JCasC.

3. User access level to Jenkins can now be controlled via assigning the users to required groups on Keycloak.

4. Added 3 new groups on Keycloak as below,

| Keycloak Group | Jenkins Role | Access Level|
|-|-|-|
| `horizon-jenkins-administrators `| Global: Admin| Full admin access|
| `horizon-jenkins-workloads-android-developers`| Item: workloads-android-developers| Full build/config rights for Android| 
| `horizon-jenkins-workloads-android-users`| Item: workloads-android-users| Limited build access for Android |

> [!NOTE]  
>Users need to logout and login again to Jenkins for the updated groups configuration made on Keycloak to take effect on Keycloak.

##### Action required
1. Run the GitHub Actions Terraform workflow to update the container images required for the Jenkins post-jobs.

2. Login to Argo CD and trigger synchronization, to run the post-jobs.

The post-jobs configured for Jenkins must run and finish successfully for the successful configuration of the Jenkins client and creation of the required groups on Keycloak.

In case the groups are not visible on Keycloak, 

- Login to Argo CD and sync horizon-sdv application and wait for the post-jobs to finish successfully.

- Check if the post-job container images on artifact registry were updated.

- Make sure the secret JENKINS_INITIAL_PASSWORD has been set with a complex value which includes a mix of uppercase letters, lowercase letters, special characters and numbers.

---

### TAA-611 | Argo CD SSO

#### Release Notes
Argo CD has been configured with SSO capabilities. It is now possible to Login to Argo CD either by using the configured admin credentials or by clicking the “Login via Keycloak” button.

##### Changes
1. Created argocd client and mapped required client scope and mappers on Keycloak.

2. Created and mapped required group (`horizon-argocd-administrators`) group on Keycloak.

3. Update the Kubernetes secrets and configmaps required for enablement of Argo CD SSO via Keycloak.

##### Available Groups
Below table details the Keycloak to Argo CD mapping with their access level granted to users within the respective groups.

| Keycloak Group | Argo CD Role | Access Level|
|-|-|-|
|`horizon-argocd-administrators`|role: admin|Full admin access|

> [!NOTE]  
> Users need to logout and login again to Argo CD for the updated groups configuration made on Keycloak to take effect on Keycloak.

##### Action required
1. Run the GitHub Actions Terraform workflow to update the container images required for the Argo CD post-jobs.

2. Login to Argo CD (in case SSO is not available, use the admin user credentials) and trigger synchronization, to run the post-jobs.

The post-jobs configured for Argo CD must run and finish successfully for the successful configuration of the Argo CD client and creation of the required groups on Keycloak.

In case the groups are not visible on Keycloak, login to Argo CD and sync horizon-sdv application and wait for the post-jobs to finish successfully. Check if the post-job container images on artifact registry were updated.

---

### TAA-717 | Multiple pre-warmed disk pools

#### Release Note
##### Update to Persistent Volume Storage for Android Build Caches
We have made changes to the persistent volume storage for build caches to improve build times, cost and efficiency. These pools are separated by Android major version, e.g. Android 14 and 15, but also Raspberry Vanilla (RPi) targets now have their own smaller pools rather than sharing the original common pool.

##### Changes:

- Separate Storage Pools for Android Versions: We have created separate persistent storage pools for each major Android version, specifically for Android 14 and 15. This will help to avoid conflicts and improve performance. New storage classes:

    - `reclaimable-storage-class-android-14`
    - `reclaimable-storage-class-android-15`

- Additional Storage Pools for RPi Targets: We have also created additional storage pools to separate RPi targets from non-RPi targets. This will prevent repo sync/manifest conflicts and improve overall stability. New storage classes:

    - `reclaimable-storage-class-android-14-rpi`
    - `reclaimable-storage-class-android-15-rpi`

- Reduced Volume Size: We have reduced the volume size from 2TB to 1000Gi to optimise storage usage and reduced RPi pools to 500Gi.

- Build jobs storage pool selection: User may allow the build job to dynamically derive the appropriate storage pool, or may explicitly override using ANDROID_VERSION parameter. RPi pools are derived from the lunch target.

    The `ANDROID_VERSION` options are:

    - ‘default’ - let the job determine the storage based on branch name/version string 

    - ‘14’ - use the Android 14 build pool

    - ‘15’ - use the Android 15 build pool

    **Note:** Users may update the logic in those jobs to reflect their nomenclature, what is provided is based on the Horizon-SDV based delivery. Users may wish to create a class not simply based on major Android version but also revision, e.g.reclaimable-storage-class-android-15.0.0_r32, and update the logic in the pipeline jobs that use the disk pools.


##### Action Required:
To ensure a smooth transition and reduce costs, we recommend that you audit your Kubernetes persistent volumes and delete the older volumes and associated GCE disks. These volumes will no longer be referenced and used by the build jobs.

###### Audit and Delete Steps:
1. Run the following command to get a list of persistent volumes (PVs) in the 

```
kubectl get pv -n jenkins -o jsonpath='{.items[?(@.spec.storageClassName=="reclaimable-storage-class")].metadata.name}'
```
**Note:** reclaimable-storage-class is the Horizon-SDV 1.1.0 Release storage class, newer classes are defined above.

2. Using the list of PVs, selectively delete them by repeating the following command per PV:
```
kubectl delete pv <pv name> -n jenkins
gcloud compute disks delete <pv name> --zone=<zone>
```
    Replace <pv-name> with the actual name of the PV you want to delete, and <zone> being that of your GCP project ZONE, e.g. europe-west1-d.

By taking these steps, you will help to optimise your storage usage and reduce costs. If you have any questions or concerns, please don't hesitate to reach out.

---

### TAA-837 | Access Control script

#### Release Note

#### Summary
The Access Control functionality provides a Python script and classes for managing user and access control  on GCP level. 
* It handles user roles defined for Horizon project and their mapping to GCP roles. This is useful in cases where roles within Horizon need to be mapped to specific permissions or roles in GCP, allowing for more efficient access control management in a cloud environment. 
* Additionally, it can directly manage user's GCP roles (e.g. delete, preview, assign) and supports automated user provisioning in GCP when needed.

#### Documentation: 
This project uses Sphinx to generate project documentation. The documentation is written in reStructuredText format and can be built into HTML files. Also   README.md file is present in the main path of the tool. README also contains information on  generating the Sphinx documentation locally.

---


## Bug fixes

| Issue key | Summary |
| --------- | ------- |
| TAA-980   | Access control issue: Workstation User Operations succeed for non-owned workstations  |
| TAA-984   | [Kaniko] Increase CPU resource limits   |
| TAA-982   | [ABFS] Uploaders not seeding new branch/tag correctly  |
| TAA-981   | [ABFS] CASFS kernel module update required (6.8.0-1027-gke)  |
| TAA-977   | New Cloud Workstation configuration is created successfully, but user details are not added to the configuration  |
| TAA-974   | kube-state-metrics ServiceAccount missing causes StatefulSet pod creation failure  |
| TAA-968   | [IAA] Elektrobit patches remain in PV and break gerrit0  |
| TAA-966   | [ABFS] Kaniko out of memory   |
| TAA-953   | Android CF/CTS: update revisions   |
| TAA-964   | [Gerrit] Propagate seed values   |
| TAA-959   | Reduce number of GCE CF VMs on startup |
| TAA-932   | ABFS_LICENSE_B64 not propagated to k8s secrets correctly  |
| TAA-958   | [Gerrit] repo sync - ensure we reset local changes before fetch  |
| TAA-781   | GitHub environment secrets do not update when Terraform workload is executed.  |
| TAA-933   | Failure to access ABFS artifact repository    |
| TAA-905   | AAOS build does not work with ABFS  |
| TAA-931   | Create common storage script   |
| TAA-930   | Investigate build issues when using MTK Connect as HOST  |
| TAA-923   | Cuttlefish limited to 10 devices |
| TAA-921   | [Cuttlefish] Building android-cuttlefish failing on gnu.org  |
| TAA-922   | MTK Connect device creation assumes sequential adb ports  |
| TAA-920   | Android Developer Build and Test instances leave MTK Connect testbenches in place when aborted  |
| TAA-563   | [Jenkins] Replace gsutils with gcloud storage  |
| TAA-886   | Conflict Between Role Strategy Plugin and Authorize Project Plugin  |
| TAA-814   | Android RPi builds failing: requires MESON update |
| TAA-863   | Workloads Guide: updates for R2.0.0   |
| TAA-867   | Gerrit triggers plugin deprecated |
| TAA-890   | Persistent Storage Audit: Internal tool removal  |
| TAA-618   | MTK Connect access control for Cuttlefish Devices  |
| TAA-711   | [Qwiklabs][Jenkins] GCE limits - VM instances blocked  |