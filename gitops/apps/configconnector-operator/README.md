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

# Config Connector operator pin

GitOps installs Config Connector from this Helm chart. Argo CD Application
`configconnector-operator` points at this path. The GKE Config Connector
add-on stays **disabled** (`terraform/modules/sdv-gke-cluster/main.tf`).

The cluster-mode `ConfigConnector` CR is **not** in this chart. It stays in the
parent `horizon-sdv` Application (`gitops/templates/config-connector-init.yaml`),
which has owned it since before the Git-pinned operator. If the CR moved to this
Application, the parent would prune it on the first sync of the upgrade, and the
operator would uninstall Config Connector (CRDs, `cnrm-system`, webhooks).
The parent applies it at wave 1 after the `kcc-crd-gate` hook sees this
chart's `ConfigConnector` CRD established, then `kcc-readiness-gate` waits for
the operator to reconcile it. Discovery RBAC and the webhook monitor are in the
same parent template.

| Field | Value |
| --- | --- |
| Version | `1.156.0` |
| Image | `gcr.io/gke-release/cnrm/operator:1.156.0` |
| Digest | `sha256:ed4bd32055c435af80a33131ff8a92c9d75b22c29bf9de5e9c8c3a677db2156c` |
| Bundle | `gs://configconnector-operator/1.156.0/release-bundle.tar.gz` |

Keep this table and the StatefulSet `image:` in
`templates/configconnector-operator.yaml` in sync. Do not add Helm `{{` to
that vendored file.

`network-policy.yaml` allows operator egress to the GKE private control plane
(`10.0.0.0/28`, same as `master_ipv4_cidr_block`) on 443/6443 and to
`kube-dns` on 53. Do not replace that with a `namespaceSelector` to
`default`/`kube-system`. The API is not a Pod in those namespaces, and an
egress-only policy otherwise blocks reconcile.

The parent `horizon-sdv` app annotates the `ConfigConnector` CR and the Argo
Workflows `StorageBucket` with `SkipDryRunOnMissingResource=true`. On
greenfield their CRDs do not exist when the parent sync starts, and
`storage.cnrm` CRDs are not in this operator bundle.

`WorkstationCluster`, `WorkstationConfig`, and `Workstation` CRDs are installed
by the operator after it reconciles the cluster-mode `ConfigConnector` CR.
They are not in the operator manifest. After cutover, record `kubectl explain`
for those three kinds.

## Cutover (brownfield)

1. Inventory live CNRM objects (name, external ID, deletion policy, Ready).
2. Apply Terraform with the add-on disabled. Do **not** delete the
   `ConfigConnector` CR or CNRM resources.
3. Wait until GKE finishes add-on teardown (`configconnector-operator-system`
   may be reclaimed).
4. Let Argo CD sync. The parent keeps owning the existing `ConfigConnector` CR,
   and this chart replaces the add-on operator.
5. Confirm operator and `cnrm-system` are Ready, Workstations CRDs exist, and
   inventoried CNRM objects are still Ready.

## Field ownership

Do not set `cnrm.cloud.google.com/reconcile-interval-in-seconds: "0"` to stop Config Connector from fighting over fields another writer owns. A value of `0` stops reconciliation of that object, including the fields Config Connector is supposed to apply. Cloud Workstations keeps the Workstations API fields off the Config Connector object instead. The split is in [the module field-ownership matrix](../../modules/cloud-workstations-module/core/README.md).

## Rollback

Do **not** re-enable the GKE add-on if this pin's CRDs are newer than the
add-on bundle (Google does not support CRD downgrade). Roll back by pinning an
older operator that is still **greater than or equal to** the live CRD
generation, or leave the add-on off.

## Bump the pin

1. Download a **versioned** bundle (never `latest`):
   `gs://configconnector-operator/<VERSION>/release-bundle.tar.gz`
2. Replace `templates/configconnector-operator.yaml` with
   `operator-system/configconnector-operator.yaml` from that bundle.
3. Resolve `docker-content-digest` for
   `gcr.io/gke-release/cnrm/operator:<VERSION>`.
4. Set the StatefulSet `image:` in `templates/configconnector-operator.yaml` to
   `gcr.io/gke-release/cnrm/operator:<VERSION>@sha256:<DIGEST>`, then update
   this table. Do not edit the `ConfigConnector` CR as part of a pin bump.
5. The new version must be **>=** the version already on the cluster.
