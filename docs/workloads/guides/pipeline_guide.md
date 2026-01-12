# Pipeline Guide

## Table of contents
- [Overview](#overview)
- [Prerequisites](#prerequisites)
- [Groovy Definitions](#definitions)
- [Creating / Editing Job](#editing)
- [Seed Job](#seed)

## Overview <a name="overview"></a>

Immediately after initial launch, Jenkins contains a single job - _Seed Workloads_ - which is defined in `gitops/workloads/values-jenkins.yaml` (CasC).

The _Seed Workloads_ job uses the groovy definitions to initialise the jobs required for each workload.
<br>It also allows users to update Jenkins to pull in any changes made to the groovy definitions - these can be new job definitions and/or changes to existing jobs.

## Prerequisites<a name="prerequisites"></a>

### Role Based Strategy<a name="rolebasedstrategy"></a>
To run pipeline jobs, users must have access to Jenkins and be granted permissions to access jobs in the workloads.

- Users given appropriate Keycloak Group access as per the instructions detailed in [Jenkins Access via Keycloak Groups](../../deployment_guide.md#section-5d---jenkins-access-via-keycloak-groups), i.e. `docs/deployment_guide.md`.
- Jenkins must be updated to provide the users permissions to seed jobs:
  - In `Jenkins` → `Manage Jenkins` → `Manage and Assign Roles` → `Assign Roles`.
    - Those roles are:
      - `Global:`
        - `horizon-jenkins-administrators`
        - `horizon-jenkins-workloads-developers`
        - `horizon-jenkins-workloads-users`
      - `Items:`
        - `workloads-developers`
        - `workloads-users`
  - Add the user to appropriate Global and Item Roles:
    - In `Global Roles` select `Add User`, enter the email address of the user and select the appropriate Keycloak Group.
    - In `Item Roles` select `Add User`, enter the email address of the user and select the appropriate Jenkins Item Role.
    - Select `Save`

> [!IMPORTANT]
> **Persistence**
>
> These manually updated permissions do not persist across a Jenkins restart. To ensure persistence, we recommend adding users to the `gitops/templates/jenkins.yaml` file, e.g..
> - Update `authorizationStrategy` → `roleBased` → `roles` → `global`:
>   - Add the user to the respective group entry, e.g.
> ```
>     - user: "john.example.doe@accenture.com"
> ```
> - Update `authorizationStrategy` → `roleBased` → `roles` → `items`:
>   - Add the user to the respective group entry, e.g.
> ```
>     - user: "jane.example.doe@accenture.com"
> ```

> [!NOTE]
> **Disabling the plugin**
>
> If user wishes to disable the plugin, then remove the plugin and configuration from `gitops/templates/jenkins.yaml`:
> - Remove the plugin from the `additionalPlugins` section:
>   - `role-strategy:743.v142ea_b_d5f1d3`
>
> - Replace all within `authorizationStrategy` with the following default values:
> ```
>            authorizationStrategy: |-
>              loggedInUsersCanDoAnything:
>                allowAnonymousRead: false
> ```
> Sync Jenkins using ArgoCD restart Jenkins.

### Jenkins Pipeline Job Organization
Each Jenkins Pipeline Job is defined in a separate Groovy file, stored within its own dedicated job directory, e.g.:
- `workloads/android/pipelines/builds/aaos_builder/groovy/job.groovy`
- `workloads/openbsw/pipelines/builds/bsw_builder/groovy/job.groovy`

There are also Groovy files that define the folder structure within Jenkins, e.g
- `Android Workflows` → `Builds` → `AAOS Builder`
- `OpenBSW Workflows` → `Builds` → `BSW Builder`

### Initial Jenkins Configuration

Upon initial launch, Jenkins contains a single job called _"Seed Workloads"_, defined in `gitops/templates/jenkins.yaml` (CasC).

### Seed Workloads Job Functionality

The _Seed Workloads_ job serves the following purpose:

1. It initializes the jobs required for each workload using the Groovy definitions.
2. It allows users to update Jenkins to incorporate changes made to the Groovy definitions, including new job definitions and modifications to existing jobs.
3. Allows common job parameters to be populated via the seed job.

## Groovy Definitions <a name="definitions"></a>

### Folders

The Jenkins folder structure for each workload is defined in a top-level Groovy file located at::
- `workloads/<workload_name>/pipelines/groovy/folders.groovy`
- This file is executed before the job files are called, and is responsible for setting up the folder hierarchy.

**Folder Properties**

Each folder is defined with the following properties:
- **path**: The path to the folder.
- **displayName**: The display name of the folder.
- **description**: A brief HTML description of the folder.

Here is an example of how folders are defined for Android:

```
	folder('Android') {
	  displayName('Android Workflows')
	  description('<p>This is the top-level workload folder</p>')
	}
	folder('Android/Builds') {
	  displayName('Builds')
	  description('<p>This sub-folder contains jobs to build Android targets.</p>')
	}
```

The format is much the same across all workloads.

The resulting folder and subfolder structure in Jenkins will be:
`Android Workflows` → `Builds`


### Jobs
Individual jobs are stored in the following directory structure:

`workloads/<workload_name>/pipelines/<folder_name>/<job_name>/groovy/job.groovy`

Each job definition includes the following components:

- **Description** (optional): An HTML description that appears on the Jenkins job page.
- **Parameters** (optional): Build parameters that can be passed to the job.
- **Triggers** (optional): Build triggers that determine when the job should be executed.
- **LogRotator** (optional): Settings that specify how long artifacts and build histories are retained.
- **Definition**: The source code definitions for the job.

For more detailed information on each of these components, refer to existing Groovy files for examples and guidance.

## Creating / Editing Jobs <a name="edits"></a>

> [!IMPORTANT]
> Environment variables can be referenced in Groovy files, but they are replaced with their actual values before the Groovy files are executed by the seed job. This approach avoids the need for explicit script approval, which is required when using the `getProperty` Groovy method to resolve environment variables.
> The replacement of environment variables with their actual values is performed in the _"Prepare Groovy files"_ stage of the seed job. Therefore, it is crucial to update the replacements list in the Jenkinsfile whenever new environment variables are added to Groovy scripts.
> To ensure that environment variables are properly replaced, please refer to the [Seed Workloads](../seed.md#groovymethods) documentation for more information on how to update the replacements list in the Jenkinsfile.
> Environment variables are defined in the `gitops/templates/jenkins.yaml` file (CasC).


### Update Existing Jobs

To make any changes to pipeline jobs (or folders):

1.  Edit the job's groovy file.
2.  Commit and push the change
3.  Run the _Seed Workloads_ job & wait for completion

### Create a New Job

1. Using an existing job in the repo as reference, create a new job in the repo (including the job folder, groovy folder, Jenkinsfile and groovy script)
2. Commit and push the change
3. Run the _Seed Workloads_ job & wait for completion

> [!NOTE]
> The folder location and name of a job are specified together in the groovy definition.
> - `pipelineJob('<folders>/<jobname>')`
> - e.g.:
>   - `pipelineJob('Android/Builds/AAOS Builder')`
>   - `pipelineJob('OpenBSW/Builds/BSW Builder')`

### Delete a Job:

1. Delete the job's groovy file (and optionally its job folder, groovy folder, Jenkinsfile)
2. Commit and push the change
3. Navigate to the job on Jenkins
4. Select _Delete Pipeline_ from the options on the left hand side
5. Optional: Run the _Seed Workloads_ job & wait for completion

> [!NOTE]
> The _Seed Workloads_ job will never remove existing jobs/folders from Jenkins; while removing a job's groovy definition ensures that it will not be re-created when the job runs, the actual deletion needs to be done manually by the user.

### Rename a Job:
If a job is renamed in its groovy definition (`pipelineJob('<folders>/<jobname>')`) it is treated as a new job and the old job will still be retained. See previous sections for how new / deleted jobs are handled.


## Seed Job <a name="seed"></a>

For additional information on the seed job refer to [Seed Workloads](../seed.md).

## Workload Prerequisites

Refer to the respective workload jobs README files or the `developer_guide.md` for examples of preparatory work that is
required in order to use the build and test jobs, i.e. environment / adminstrative setup.
