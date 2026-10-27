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

# Horizon Cloud Workstations resources

The custom resource definitions are in `crds/`. Each instance is namespaced and lives in `cloud-workstations`, with the controller. The definitions themselves are cluster-scoped.

Config Connector is the GitOps pin `1.156.0` (`gitops/apps/configconnector-operator`). The controller creates a same-named Config Connector object for each Horizon resource and copies only the fields in the Config Connector column. It never writes the Workstations API column onto that object.

Do not set `cnrm.cloud.google.com/reconcile-interval-in-seconds: "0"` to keep Config Connector away from the API-owned fields. A value of `0` stops Config Connector from reconciling that object, including the fields it does own. The split below is how those fields stay with the Workstations API. Cutover and rollback for the operator pin are in the [Config Connector operator README](../../../apps/configconnector-operator/README.md).

Start and stop are not fields. The `cws-user-start-workstation` and `cws-user-stop-workstation` workflows call Google Cloud directly.

## WorkstationCluster

| Horizon field | Writer |
|---------------|--------|
| `project`, `location`, `network`, `subnetwork` | Config Connector, as `projectRef`, `location`, `networkRef`, and `subnetworkRef` |
| `displayName`, `resourceID`, `privateClusterConfig`, `labels`, `annotations` | Config Connector, when set |

`project`, `location`, `network`, `subnetwork`, and `resourceID` cannot be changed after they are set.

## WorkstationConfig

| Horizon field | Writer |
|---------------|--------|
| `clusterRef` | Config Connector `parentRef`. The parent name is the Horizon cluster name. |
| `displayName`, `idleTimeout`, `runningTimeout`, `replicaZones`, `resourceID` | Config Connector, when set |
| `container` | Config Connector. Copied fields: `image`, `command`, `args`, `env`, `runAsUser`, `workingDir`. |
| `host.gceInstance` | Config Connector. Copied fields: `machineType`, `bootDiskSizeGB`, `serviceAccountScopes`, `disablePublicIPAddresses`, `enableNestedVirtualization`, `poolSize`, `tags`, `shieldedInstanceConfig`, `confidentialInstanceConfig`. |
| Host VM service account | Config Connector `serviceAccountRef`. The controller always sets `gke-cws-host-vm-sa`. It is not a Horizon spec field. `poolSize` is `0` when unset. |
| `persistentDirectories` | Config Connector. Copied fields: `mountPath`, and on `gcePD`: `diskType`, `fsType`, `reclaimPolicy`, `sizeGB`, `sourceSnapshot`. |
| `readinessChecks`, `encryptionKey`, `labels`, `annotations` | Config Connector, when set |
| `disableSsh`, `allowedPorts`, `ephemeralDirectories` | Workstations API. The controller sends only `host.gce_instance.disable_ssh`, `allowed_ports`, and `ephemeral_directories`. The last read is `status.posture`. |
| `admins` | Workstations API `setIamPolicy` for `roles/workstations.admin`. An empty list removes every unconditional member of that role, including members granted outside Horizon. Other roles and conditional bindings stay. |

`clusterRef` and `resourceID` cannot be changed after they are set. `replicaZones` is create-only in the Portal workflow.

## Workstation

| Horizon field | Writer |
|---------------|--------|
| `configRef` | Config Connector `parentRef`. The parent name is the Horizon configuration name. |
| `displayName`, `resourceID`, `labels`, `annotations` | Config Connector, when set |
| `users` | Workstations API `setIamPolicy` for `roles/workstations.user`. An empty list removes every unconditional member of that role, including members granted outside Horizon. Other roles and conditional bindings stay. |

`configRef` and `resourceID` cannot be changed after they are set.

`status` on all three kinds is written by the controller. It includes `phase`, conditions, and, when Config Connector has reported them, the external name and observed state.
