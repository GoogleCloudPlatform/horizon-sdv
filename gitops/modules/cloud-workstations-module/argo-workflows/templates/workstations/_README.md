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

# Workstations

These workflows create, inspect, and use a workstation. The configuration must already exist. See [configurations](../config/_README.md).

Run them from the Developer Portal. Any signed-in user can run the admin workflows as well as the user workflows. The Portal does not hide the admin ones.

You may see `horizonSubmittedFrom`. Leave it blank. The Portal already records that the run came from the Developer Portal. If you set it, the only accepted values are `developer-portal`, `api`, and `horizon-cli`.

Create, delete, user changes, start, and stop of the same workstation wait for each other. Lookup workflows do not wait. Different workstations do not wait for each other.

A workstation id is 1 to 63 characters: lowercase letters, digits, and `-`, starting with a letter and not ending with `-`.

## Admin workflows

### cws-admin-create-workstation

Creates a workstation from an existing configuration and waits until it is ready. Fails if the configuration is being deleted. The configuration cannot be changed later. Delete the workstation and create it again to use a different configuration.

| Parameter | Required | What to enter |
|-----------|----------|----------------|
| `workstationId` | Yes | New workstation id. |
| `configName` | Yes | Name of an existing configuration. |
| `displayName` | No | A readable name. Empty means none. |
| `users` | No | Comma-separated emails or IAM principals who can use it. Empty means nobody. |

### cws-admin-delete-workstation

Shows the workstation, then deletes it. Set `confirmDelete` to `true`. The form starts at `false`.

| Parameter | Required | What to enter |
|-----------|----------|----------------|
| `workstationId` | Yes | Workstation to delete. |
| `confirmDelete` | Yes | `true` to delete. `false` stops the run. |

### cws-admin-get-workstation

Shows one workstation: configuration, name, state, URL, users, and status. It does not change anything.

| Parameter | Required | What to enter |
|-----------|----------|----------------|
| `workstationId` | Yes | Workstation to show. |

### cws-admin-list-workstations

Lists workstations. It does not change anything. Each filter is a regular expression. The form starts at `.*`, which lists everything. Empty also lists everything.

| Parameter | What it filters |
|-----------|-----------------|
| `nameRegex` | Workstation id. Example: `^dev-` |
| `userRegex` | A user, with or without the `user:` prefix. |
| `configRegex` | Configuration name. |

An invalid pattern fails the run.

### cws-admin-add-workstation-users

Adds people to the workstation and waits until their access is applied. People already on the workstation are left as they are. This does not replace the whole list.

If Google Cloud rejects the new list, this run removes only the people it just added and reports which addresses failed. The workstation is left ready when that rollback succeeds.

| Parameter | Required | What to enter |
|-----------|----------|----------------|
| `workstationId` | Yes | Workstation to change. |
| `users` | Yes | Comma-separated emails or IAM principals to add. |

### cws-admin-remove-workstation-users

Removes people from the workstation and waits until their access is removed. Names that are not on the workstation are skipped.

If nobody remains, workstation user access granted without an extra condition is removed, including access that was granted outside Horizon. Other roles and conditional access are left as they are.

| Parameter | Required | What to enter |
|-----------|----------|----------------|
| `workstationId` | Yes | Workstation to change. |
| `users` | Yes | Comma-separated emails or IAM principals to remove. |

### How to write a user

Use a comma-separated list. A bare email is stored as `user:email`. `user:`, `group:`, and `serviceAccount:` must be followed by an email. `domain:`, `principal://`, and `principalSet://` are also accepted. The same address is kept once. Comparison ignores case.

Each person must be a real Google account, group, or service account. An address Google does not know is rejected.

## User workflows

The Portal does not ask who you are. It hides `submittedBy` and sends your signed-in username (`preferred_username`, otherwise `name`, otherwise `sub`). These workflows only show or start workstations whose user list contains that value. You cannot type a different person in the Portal form.

The username has to match the user stored on the workstation. A bare email on the workstation is stored as `user:email`. If your sign-in name is that email, it matches.

### cws-user-list-usable-workstations

Lists workstations that include you. It prints the name, configuration, state, and URL. There is nothing else to fill in.

### cws-user-get-workstation

Shows one workstation you are on: name, configuration, state, URL, users, and status. Fails if you are not on it.

| Parameter | Required | What to enter |
|-----------|----------|----------------|
| `workstationId` | Yes | Workstation to show. |

### cws-user-get-workstation-configuration

Shows the configuration of a workstation you are on, including the image, machine type, disk size, timeouts, SSH setting, allowed ports, and admins. Fails if you are not on that workstation.

| Parameter | Required | What to enter |
|-----------|----------|----------------|
| `workstationId` | Yes | Workstation whose configuration to show. |

### cws-user-start-workstation

Starts a workstation you are on. It waits until the configuration is ready, so the machine does not boot while a configuration change is still applying. Fails if you are not on it, or if the workstation is not ready yet.

The URL printed at the end comes from the recorded state and can lag the real start by a few minutes.

| Parameter | Required | What to enter |
|-----------|----------|----------------|
| `workstationId` | Yes | Workstation to start. |

### cws-user-stop-workstation

Stops a workstation you are on. It does not wait for the configuration to be ready. Fails if you are not on it, or if the workstation is not ready yet.

The state printed at the end can lag the real stop by a few minutes.

| Parameter | Required | What to enter |
|-----------|----------|----------------|
| `workstationId` | Yes | Workstation to stop. |
