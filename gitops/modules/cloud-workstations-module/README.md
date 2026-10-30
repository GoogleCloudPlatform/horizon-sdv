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

# Cloud Workstations

Use this module to create and manage Google Cloud Workstations from the Horizon Developer Portal. It replaces the Jenkins jobs for workstation configurations, workstations, and workstation images. It does not replace the Jenkins cluster. Jenkins keeps its own cluster, `sdv-cloud-ws-cluster`.

The module runs on the main environment only. It is not deployed on a sub-environment.

## Before you enable it

1. Enable **workloads-common** first. Cloud Workstations needs it for image builds. Module Manager will not let you disable workloads-common while this module is on.
2. Enable **cloud-workstations** from the Developer Portal on the main environment.
3. Build the workflow image once per environment. Run `cws-image-build` with `buildEnvironmentImage` `true`, `imageTag` `latest`, `dryRun` `false`, and every other `build...` field `false`.

No other Cloud Workstations workflow runs until step 3 is done. A run that stays Pending with `ImagePullBackOff` means the image is missing: abort it, build the image, and submit again. The image stays in Artifact Registry when the module is disabled, so you do not build it again on re-enable. Run the same build to update it.

Enabling the module creates one Cloud Workstations cluster for Horizon. You do not create or delete that cluster from a Portal workflow.

## Where it runs

The controller and the workstation resources run in the `cloud-workstations` namespace. The resource definitions are cluster-scoped. Each resource you create is namespaced.

The Portal does not have a Cloud Workstations API. It lists these workflows from the existing catalog and submits them with the existing workflow submit endpoint. Any signed-in user can submit any of them.

## Experimental

This module is experimental. Any signed-in Horizon user can run any Cloud Workstations workflow, including the admin ones that create, delete, and change who can use a workstation.

The Portal does not offer separate admin and user permissions. It lists every workflow and shows every other input as a text box. It does not ask for an email on the user workflows. It hides `submittedBy` and sends the signed-in username (`preferred_username`, otherwise `name`, otherwise `sub`). Those workflows only show or start workstations that list that value. The Portal form cannot change it. Horizon API still accepts a different `submittedBy` when the request is not sent by that form.

Access on the workstation in Google Cloud is still applied. People listed as users get workstation user access. People listed as admins on a configuration get workstation admin access. Do not treat the Portal as a way to keep those actions limited to a role.

## What you can do

Open the module in the Developer Portal and submit a workflow.

| You want to | Workflow |
|-------------|----------|
| Create, change, delete, or look up a workstation configuration | `cws-create-config`, `cws-update-config`, `cws-delete-config`, `cws-get-config`, `cws-list-configs` |
| See which workstations use a configuration | `cws-list-workstations-by-config` |
| Create, delete, or look up a workstation, or change its users | `cws-admin-create-workstation`, `cws-admin-delete-workstation`, `cws-admin-get-workstation`, `cws-admin-list-workstations`, `cws-admin-add-workstation-users`, `cws-admin-remove-workstation-users` |
| See and use a workstation you are listed on | `cws-user-list-usable-workstations`, `cws-user-get-workstation`, `cws-user-get-workstation-configuration`, `cws-user-start-workstation`, `cws-user-stop-workstation` |
| Build workstation container images | `cws-image-build` |

Create a configuration before you create a workstation. The image choices `C-OSS (VCS)`, `AS`, and `ASfP` select the Code OSS, Android Studio, and ASfP images. You can also paste a full image address.

Parameter lists are next to the workflows: [configurations](argo-workflows/templates/config/_README.md), [workstations](argo-workflows/templates/workstations/_README.md), and [images](argo-workflows/templates/images/_README.md).

## How the pieces fit together

- A **configuration** is the template: container image, machine size, disks, idle time, who is an admin, and whether SSH is allowed.
- A **workstation** is one machine created from a configuration. It has its own list of users.
- **Start** and **stop** are separate workflows. They are not settings on the workstation.

**Pending** means the change is not finished yet. A common case is that the parent is missing or still being created: a workstation waits on its configuration, and a configuration waits on the module cluster. The module retries that wait on its own. Pending by itself does not mean it is stuck.

## Rules that are easy to miss

- **Update replaces the form.** On `cws-update-config`, a blank field is reset to its default. It does not mean "leave this as it is". The image and the two disk yes/no fields (`pdRequired`, `edRequired`) have no default on update, so you must set them or the run fails. The zone list cannot be changed after create.
- **Delete a configuration only when nothing uses it.** Child workstations must be gone first. The controller keeps a finalizer on the configuration until they are, and Google Cloud does not delete them for you. The delete workflow stops and lists them if any remain. Set `confirmDelete` to `true`. The same confirmation is required to delete a workstation.
- **Admin and user lists are the access list.** Create and update configuration require at least one admin. Anyone left off that list loses workstation admin access. A new workstation can have no users. Removing the last user removes workstation user access that was granted without an extra condition, including access added outside Horizon. Other access and conditional grants stay as they are.
- **Start waits. Stop does not.** Start waits until the configuration is ready, so a machine does not boot while a configuration change is still applying. Stop runs even if that configuration is updating or failed.
- **Only one image build runs at a time.** Pick at least one image and set a tag. If you build GNOME without building its base image in the same run, paste the existing base image address. The same applies to Android Studio and ASfP when GNOME is not built in that run. A dry run does not push images, so it cannot build an image and the image that depends on it in one run.
- **Adding a user that Google rejects is undone.** If Google rejects a new user, that workflow removes the users it just added and tells you which addresses failed.

## Turning the module off

Disabling **cloud-workstations** deletes, in order, its workstations, its configurations, and its Cloud Workstations cluster. It waits for each step to finish in Google Cloud before the next. The Jenkins cluster is not deleted.

Commit and push your work first. A persistent disk whose reclaim policy is **DELETE** is removed with the workstation. **RETAIN** keeps that disk in Google Cloud.

If a delete does not finish in time, the module stays enabled and can be disabled again. Nothing else is removed on that failed attempt.

## Check status

```bash
kubectl get workstationclusters,workstationconfigs,workstations -n cloud-workstations
```

`Ready` means the change has been applied. `Pending` means it is still waiting. `Failed` means the change was rejected. The workflow log prints the reason.

## More detail

[Configurations](argo-workflows/templates/config/_README.md), [workstations](argo-workflows/templates/workstations/_README.md), and [images](argo-workflows/templates/images/_README.md) document each workflow and its inputs. [Field ownership](core/README.md) lists which writer applies each field.
