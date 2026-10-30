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

# Workstation configurations

These workflows create and manage a workstation configuration. A configuration is the template for workstations: image, machine size, disks, timeouts, admins, and whether SSH is allowed. Create one before you create a workstation.

Run them from the Developer Portal. The module cluster must already exist. Enabling the module creates it.

You may see `horizonSubmittedFrom`. Leave it blank. The Portal already records that the run came from the Developer Portal. If you set it, the only accepted values are `developer-portal`, `api`, and `horizon-cli`.

Create, update, and delete of the same configuration wait for each other. Different configurations do not.

## cws-create-config

Creates a new configuration and waits until it is ready. Fails if that name already exists, or if the module cluster does not exist or is being deleted.

`cloudWsConfigName` and `wsAdminIamMembers` are required. Every other field can be left as shown.

## cws-update-config

Replaces the settings in the table below. A blank field is reset to the "If cleared" value. It does not mean "leave this unchanged".

`containerImage`, `pdRequired`, and `edRequired` have no default on update. You must set them. A blank image or a blank disk yes/no would otherwise change the image or remove a disk.

`wsReplicaZones` is not on the update form. Zones cannot be changed after create. Settings this form does not ask for, such as a display name, stay as they are.

The configuration must already exist.

## cws-delete-config

Deletes one configuration and its admin access. Set `confirmDelete` to `true`. The form starts at `false`.

Child workstations must be gone first. The workflow fails and lists any that remain. Google Cloud does not cascade this delete. The controller keeps a finalizer on the configuration until those workstations, and their Config Connector objects, are gone.

| Parameter | Required | What to enter |
|-----------|----------|----------------|
| `cloudWsConfigName` | Yes | Exact configuration name. |
| `confirmDelete` | Yes | `true` to delete. `false` stops the run. |

## cws-get-config

Prints the settings and status of one configuration. It does not change anything.

| Parameter | Required | What to enter |
|-----------|----------|----------------|
| `cloudWsConfigName` | Yes | Exact configuration name. |

## cws-list-configs

Lists configuration names. It does not change anything.

| Parameter | Starts as | What to enter |
|-----------|-----------|----------------|
| `cloudWsConfigNameRegexPattern` | Empty | A regular expression, for example `^dev-`. Empty lists every configuration. An invalid pattern fails the run. |

## cws-list-workstations-by-config

Lists workstation names that use one configuration. It does not change anything.

| Parameter | Required | What to enter |
|-----------|----------|----------------|
| `cloudWsConfigName` | Yes | Exact configuration name. The configuration must exist. |

## Create and update fields

`true` and `false` must be those words.

| Parameter | Create starts as | If cleared | On update |
|-----------|------------------|------------|-----------|
| `cloudWsConfigName` | Empty, required | | Required. The configuration to change. |
| `containerImage` | `C-OSS (VCS)` | Code OSS | Required. No default. |
| `containerImageTag` | `latest` | `latest` | Same. Ignored when `containerImage` is a full image address. |
| `wsIdleTimeout` | `1200` | `1200` (20 minutes) | Same. Seconds. |
| `wsRunningTimeout` | `43200` | `43200` (12 hours) | Same. Seconds. |
| `wsReplicaZones` | Empty | Google Cloud default | Not offered. Create only. |
| `hostMachineType` | `e2-standard-4` | `e2-standard-4` | Same. |
| `hostQuickStartPoolSize` | `0` | `0` (no pre-warmed machines) | Same. |
| `hostBootDiskSize` | `40` | `40` | Same. Boot disk size in GB. |
| `hostDisablePublicIpAddresses` | `true` | `true` | Same. `true` means no public IP. |
| `hostDisableSsh` | `true` | `true` | Same. `true` means SSH is off. |
| `hostEnableNestedVirtualization` | `false` | `false` | Same. Only for N1 and N2 machine types. |
| `pdRequired` | `false` | `false` | Required. `true` adds one persistent disk. |
| `pdMountPath` | `/home` | `/home` | Used when `pdRequired` is `true`. |
| `pdFsType` | `ext4` | `ext4` when there is no snapshot | Must be empty if `pdSourceSnapshot` is set. |
| `pdDiskType` | `pd-balanced` | `pd-standard` | `pd-balanced`, `pd-ssd`, `pd-standard`, or `pd-extreme`. |
| `pdSizeGb` | `10` | `200` | `10`, `50`, `100`, `200`, `500`, or `1000`. Below 200 requires `pd-balanced` or `pd-ssd`. Must be empty if `pdSourceSnapshot` is set. |
| `pdReclaimPolicy` | `DELETE` | `DELETE` | `DELETE` removes the disk with the workstation. `RETAIN` keeps it. |
| `pdSourceSnapshot` | Empty | No snapshot | Snapshot name only, not a full path. |
| `edRequired` | `false` | `false` | Required. `true` adds one ephemeral disk. |
| `edMountPath` | `/tmp` | `/tmp` | Used when `edRequired` is `true`. |
| `edDiskType` | `pd-standard` | `pd-standard` | `pd-standard`, `pd-ssd`, `pd-balanced`, or `pd-extreme`. |
| `edSourceSnapshot` | Empty | None | When `edRequired` is `true`, set this or `edSourceImage`. Not both. A snapshot also requires `edReadOnly` `true`. |
| `edSourceImage` | Empty | None | The alternative to `edSourceSnapshot` when `edRequired` is `true`. |
| `edReadOnly` | `false` | `false` | `true` requires `edSourceSnapshot`. A snapshot requires `true`. |
| `containerEntrypointCommands` | Empty | Image default | Comma-separated. |
| `containerEntrypointArgs` | Empty | None | Comma-separated. |
| `containerWorkingDir` | Empty | None | Directory inside the container. |
| `containerEnvVars` | Empty | None | JSON object, for example `{"ENV1":"val1"}`. |
| `containerUser` | Empty | Image default | Numeric user id. |
| `wsAllowedPorts` | Empty | Port 80 and ports 1024-65535 | JSON list, for example `[{"first":80,"last":80}]`. Each range must have `first` less than or equal to `last`, and must be 22, 80, or inside 1024-65535. |
| `wsAdminIamMembers` | Empty, required | | Required. Cannot be empty. |

### Names, image, and admins

`cloudWsConfigName` is 1 to 63 characters: lowercase letters, digits, and `-`, starting with a letter and not ending with `-`.

`containerImage` accepts:

- `C-OSS (VCS)` for Code OSS
- `AS` for Android Studio
- `ASfP` for ASfP
- a full image address, used as written

`wsAdminIamMembers` is the full admin list, not a list of people to add. Anyone left off loses workstation admin access that was granted without an extra condition, including access added outside Horizon. Conditional grants stay as they are. Use comma-separated email addresses. A bare email is stored as `user:email`. `user:`, `group:`, and `serviceAccount:` must be followed by an email. `domain:`, `principal://`, and `principalSet://` are also accepted. Repeated addresses are kept once.

A persistent-disk snapshot only works when `pdFsType` and `pdSizeGb` are empty. Clear the values the form starts with.
