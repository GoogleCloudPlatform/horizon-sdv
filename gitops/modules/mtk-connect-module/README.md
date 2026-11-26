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

# mtk-connect module

Helm chart: `gitops/modules/mtk-connect-module`  
Argo CD parent Application: **`mod-mtk-connect`** (enabled from the Developer Portal, catalog name `mtk-connect`).

Packages MTK Connect as a Horizon module so it can be enabled and disabled from Administration → Modules.

## Module Metadata

| Metadata Field | Value |
| :--- | :--- |
| **STATUS** | proposed |
| **OWNER** | Accenture |
| **CONTACT** | module: [horizon-sdv@accenture.com](mailto:horizon-sdv@accenture.com) |
| **CHART VERSION** | 0.2.0 |
| **CONTAINS_COMMERCIAL** | no |

## Catalog

| Setting | Value |
|---------|--------|
| Module name | `mtk-connect` |
| Hard dependencies | none |
| `autoDisableWhenUnused` | `false` (MongoDB data belongs to this product; stays enabled until you disable it) |
| Overview service | `mod-mtk-connect-overview` in namespace `<prefix>mtk-connect-overview` |

The catalog entry is in `gitops/apps/module-manager/templates/module-catalog.yaml`.

## Controller

Child Applications:

| Application | Chart | Purpose |
|-------------|-------|---------|
| `mod-<module>-operator` | `operator/` | Controller Deployment, RBAC, NetworkPolicies and (main environment only) the `MTKConnectInstance` CRD |
| `mod-<module>-integration` | `integration/` | The `MTKConnectInstance` resource `default` in `<prefix>mtk-connect` |

The controller (`mtk-connect-module-app` image, source in `terraform/modules/sdv-container-images/images/mtk-connect-module`) runs without privileged jobs and:

- creates or updates the Keycloak SAML client `mtk-connect` and writes `mtk-connect-keycloak`; the server Deployment is rolled once per change of the key material (pod-template annotation `horizon-sdv.io/keycloak-secret-hash`);
- creates the `mtk-connect-admin` service account (through `createServiceAccount.js` in the server pod) when `mtk-connect-apikey` is missing or its key is rejected;
- rotates the API key every `keyRotation.interval`, writes it to `mtk-connect-apikey` and every entry of `credentialDistribution`, and deletes keys superseded for more than `keyRotation.overlap`;
- reports `status.phase` (`Configuring`, `Ready`, `Degraded`), conditions, Kubernetes Events and the metrics `mtkc_rotation_failures_total`, `mtkc_key_expiry_seconds` and `mtkc_api_requests_total`.

Force a rotation:

```bash
kubectl -n <prefix>mtk-connect annotate mtkconnectinstance default \
  mtkconnect.horizon-sdv.io/rotate-now="$(date +%s)" --overwrite
```

Notes:

- The CRD is `operator/templates/crd-mtkconnectinstance.yaml`. It is cluster-wide, so only the main environment installs it (`Prune=false,Delete=false`); a sub-environment needs the module enabled in the main environment first.
- The controller Secret and Deployment names (`mtk-connect-keycloak`, `mtk-connect-apikey`, `mtk-connect`, plus `mtk-connect-testbench-tokens` for testbench registration tokens) and the Keycloak admin Secret (`keycloak-initial-creds`) are fixed in `operator/templates/rbac.yaml`. `spec.server.deployment`, `keycloakSecret` and `apiKeySecret` stay in the CRD for compatibility but must keep these values; any other value is reported as `InvalidSpec` instead of failing later with a 403. The Secrets that receive the credentials (`credentialDistribution`) take their names from the chart, so those can be changed.
- The controller reaches the Kubernetes API through the GKE private control-plane address (`kubeApiServerCIDRs`, default `10.0.0.0/28`); it is not a pod in `kube-system`. Change it if the cluster's `master_ipv4_cidr_block` changes.
- The controller calls the router in-cluster without the public `/mtk-connect` prefix, because the gateway strips it before the request reaches the router.
- A stored API key is replaced only after it is rejected on two consecutive reconciles; a missing `mtk-connect-apikey` is bootstrapped immediately. While the server is down or not yet ready the instance reports `Configuring` or `Degraded` with reason `ServerNotReady`.
- Deleting the `MTKConnectInstance` leaves the Keycloak client, Secrets and keys in place.
- Only the `MTKConnectInstance` named by `INSTANCE_NAME` (default `default`) is reconciled, the same one the testbench and device controllers read. Another instance in the namespace is ignored.
- `spec.keycloak.url` and `spec.server.url` must be in-cluster Service addresses (`<service>.<namespace>.svc` or `.svc.cluster.local`, no user info, query or fragment), because the controller sends the Keycloak admin password and the API key to them. The namespace in `spec.keycloak.url` must be the namespace of `spec.keycloak.adminSecretRef`, and `spec.domain` must be a plain host name. A spec that breaks these rules is reported as `InvalidSpec` and nothing is sent.
- The controller image reference comes from `config.modules.mtk-connect.containerImage` (see [Module-specific MODULE_CONFIG](../../../docs/architecture/06_interfaces.md#module-specific-module_config)).

## Platform couplings and permissions

The module depends on platform components, not on other modules, so the catalog declares no dependencies. The couplings are:

| Coupling | Why |
|----------|-----|
| Keycloak (`<prefix>keycloak`): read `keycloak-initial-creds` | Log in as admin to create the SAML client `mtk-connect`. The Role grants `get` on that one Secret. |
| `workflows` namespace: write the Secrets listed in `credentialDistribution` | Argo workflows (CVD/CTS) read the distributed admin API key (`workflow-mtk-connect-apikey`). The distribution Role grants `get` and `update` by `resourceNames`, and `create`. The testbench Role in this same namespace also grants `get`, `create` and `update` on every Secret; the policy `<prefix>mtk-connect-workflow-secret` limits those writes to `<testbench>-agent` and this named Secret. `get` stays unscoped, because admission does not apply to it. The admin key is not written into the `jenkins` namespace. |
| `workflows` namespace: write `<testbench>-agent` | The host credential (an `mtk-connect-agent` key, the registration and its token). It is not a Jenkins credential. |
| `jenkins` namespace: write `mtkc-<namespace>-<testbench>-agent` | The same agent username and password, and nothing else, labelled `jenkins.io/credentials-type: usernamePassword`. The name includes the testbench namespace. The credentials provider only lists Secrets in this namespace. Admission allows this ServiceAccount to change or delete that Secret only when it already has `mtkconnect.horizon-sdv.io/jenkins-credential`. A testbench that itself lives in this namespace also has its host Secret `<testbench>-agent`, which must not carry that label, including when that name starts with `mtkc-`. `get` on Secrets in this namespace stays unscoped: admission does not apply to it, and the Secret name is not known when the chart is rendered. |
| The `mtk-connect` Application (`gitops/apps/mtk-connect`) | It deploys the server, MongoDB and router that this module integrates with; see "Coexistence" below. |

`pods/exec` is granted on the pods of `<prefix>mtk-connect`, because the controller runs `createServiceAccount.js` in the server pod to create the service account. Kubernetes cannot limit `pods/exec` by label, so the grant also covers the other pods in that namespace, MongoDB included. The controller code only execs into a ready pod matching `spec.server.podSelector`, with a fixed command.

The `MTKConnectInstance`, `MTKConnectTestbench` and `MTKConnectDevice` CRDs are cluster-wide and installed by the main environment only. A sub-environment applies the integration Application without installing them, so enable the module in the main environment first; the integration Application uses `SkipDryRunOnMissingResource` and syncs once the CRDs exist.

## Testbenches

An `MTKConnectTestbench` (short name `mtkctb`) is one MTK Connect testbench. Create it in `<prefix>workflows` or `<prefix>jenkins` (`testbenchNamespaceSuffixes`):

```yaml
apiVersion: mtkconnect.horizon-sdv.io/v1alpha1
kind: MTKConnectTestbench
metadata:
  name: bench-a
  namespace: workflows
  labels:
    horizon-sdv.io/submitted-from: developer-portal
spec:
  displayName: Bench A
  access:
    public: true          # the MTK Connect "everyone" group
    users: []             # or e-mail addresses
```

```bash
kubectl -n workflows get mtkctb bench-a -o wide
kubectl -n workflows get mtkctb bench-a -o jsonpath='{.status.registration}'
```

The controller:

- adds the finalizer `mtkconnect.horizon-sdv.io/agent-cleanup` first, then creates an MTK Connect registration named `hz-<16 hex>` (derived from the resource UID) with a token, and the agent that uses that registration. The server rejects an agent whose registration does not already exist, and it rejects a registration that has no token. The registration name is the same on every reconcile, so a controller restart finds the existing agent and creates no second one. The token is a random UUID, generated once and kept in the Secret `mtk-connect-testbench-tokens` in the module namespace (one key per registration name, the value the token); a retry reads it from there. It is not derived from the resource and is never written to `status`, Events or logs. `status.registrationTokenSecret` names that Secret, and the key is `status.registration`. Deleting the testbench removes its key. It then grants `access` to the group or users;
- writes Secret `<testbench>-agent` in the testbench namespace, owned by the testbench. It contains a key of the user `mtk-connect-agent` that expires after `spec.credentialTTL` (default `6h`, at most `testbenchPolicy.maxCredentialTTL`), the registration name and that testbench's registration token. It does not contain the admin API key. `status.agentCredentialSecret` names this Secret. `mtkc-agent` on the host reads it, refuses any other username, and starts the MTK Connect node agent with the registration and its token. The same username and password, without the token, are copied to Secret `mtkc-<namespace>-<testbench>-agent` in the `jenkins` namespace and labelled `jenkins.io/credentials-type: usernamePassword`, because the credentials provider only watches that namespace and uses the Secret name as the credentials id. A Jenkins job that uses this testbench binds it with `usernamePassword(credentialsId: 'mtkc-<namespace>-<testbench>-agent', ...)`. Deleting the testbench removes that copy;
- records the id of every host key on the testbench (`status.hostKeyId`, `status.retiredHostKeys`) before the key is written to the Secret. The controller replaces the key before it expires. The replaced key stays valid until the instance's `keyRotation.overlap` ends, or until it expires if that is sooner, so a host that loaded it is not cut off, and is revoked then. Deleting the testbench revokes the current and the retired keys from that record, so a missing or deleted `<testbench>-agent` Secret does not leave a key alive;
- reports `status.phase`: `Pending` (waiting for the instance or its API key), `AgentProvisioned`, `AwaitingAgent` (agent record exists, nothing has connected), `Ready` (the agent is online), `Offline` (it was online and is not now), `Deleting` or `Failed`;
- shows `status.registration` (the name an agent must connect with), `status.agentId` and `status.url`. `status.url` is the portal root `/mtk-connect/` unless `testbenchURLPath` is set to a path with `{id}`;
- deletes the agent and its registration when the resource is deleted. If the `MTKConnectInstance` is missing, deletion waits; set the annotation `mtkconnect.horizon-sdv.io/skip-agent-cleanup=true` to delete the resource and leave the agent and its registration behind. That annotation still removes the Jenkins credential. It revokes the host key when the API can be reached, and otherwise leaves the key until `credentialTTL`;
- reports the number of testbenches by phase as `mtkc_testbenches{phase}`.

The spec is the agent record only, and it is immutable (delete and re-create to change it). It declares no `devices` or `agent` fields, so the API drops them instead of storing a device count the controller would ignore. Devices are a separate resource. The CRD also rejects an `access` with neither `public` nor users.

The ValidatingAdmissionPolicy `<prefix>mtk-connect-testbench` applies on create in this environment's testbench namespaces. It requires the label `horizon-sdv.io/submitted-from` to be one of `testbenchPolicy.allowedSubmittedFrom` (`api`, `developer-portal`, `horizon-cli`, `custom`), and it enforces `allowPublicAccess` and `allowedPermissions`. The controller also refuses a testbench without that label, so one created while the policy is absent still records where it came from.

A testbench stays `AwaitingAgent` until an agent connects with `status.registration`. On the device host (or in the agent container), `mtkc-agent` starts that agent from the `<testbench>-agent` credential:

```bash
mtkc-agent start --mode host --domain <domain> --registration <status.registration> --credentials <dir or JSON file>
mtkc-agent status --mode host
mtkc-agent stop --mode host
```

`start` launches the MTK Connect node agent in its own process group with `agent__uri=https://<domain>/mtk-connect`, `agent__log__appender=file`, `agent__registration__name` and `agent__registration__token` in the child's environment (never on the command line or in the output). `agent__uri` and `agent__log__appender` are the names `mtk_connect.sh` writes for the agent. The registration names follow that same nesting; the node agent is not in this repo, so those two are not confirmed by its source. `start` records the PID in the config directory and waits until the agent has written `registration.name`. `stop` stops that process group. `--mode` selects the default config directory and agent command: host `/usr/src/config` and `node runAgent.js` in `/opt/mtk-connect-agent`, container `/opt/mtk-connect-agent/config` and `node index.js` in `/usr/src/app`. `--agent-dir` and `--agent-cmd` override the agent location.

Notes:

- The CRD `operator/templates/crd-mtkconnecttestbench.yaml` is installed by the main environment only, like the instance CRD.
- A testbench whose agent is deleted in MTK Connect is reported `Failed` (`AgentMissing`) and is not re-created, because that would change its identity silently. Delete and re-create the resource.

## Devices

An `MTKConnectDevice` (short name `mtkcd`) is one device of a testbench in the same namespace. Create the devices of a testbench in index order, starting at 1, with no gaps: MTK Connect numbers them itself in creation order. `testbenchRef`, `index` and `profile` cannot change. Profiles are `Adb`, `HostOnly` and `TunnelCarrier`. A change is applied on the next watch event, and a device that is waiting is retried every 15 seconds.

The ValidatingAdmissionPolicy `<prefix>mtk-connect-device` applies on create and on spec changes in this environment's testbench namespaces. It enforces `testbenchPolicy.maxDevices` (8) and `tunnelPortMin`..`tunnelPortMax` (1024 to 65535).

Deleting the device removes it from MTK Connect. If the testbench is already gone or being deleted, the agent deletion removes its devices and no separate call is made. The annotation `mtkconnect.horizon-sdv.io/skip-agent-cleanup=true` releases the resource and leaves the device behind.

## Coexistence with the existing MTK Connect deployment

The module does not replace the existing deployment. The Argo CD Application `<prefix>mtk-connect` (`gitops/templates/mtk-connect.yaml`, `gitops/apps/mtk-connect`) keeps deploying the server, MongoDB, gateway route, storage and network policies, and remains the integration used by `remotive-topology` and the Argo workflows, which read `workflow-mtk-connect-apikey`. With the module disabled the behaviour is as before.

With the module enabled, the controller runs next to it in the same namespace:

- it uses its own pod label (`app.kubernetes.io/name: mtk-connect-operator`), so the existing policies, `kubectl` selectors and Services that match `mtk-connect` never select it; it is allowed only by its own NetworkPolicies on top of the namespace default-deny;
- it writes the same Secrets, with the same keys, as the existing post jobs (`mtk-connect-keycloak`, `mtk-connect-apikey`, `workflow-mtk-connect-apikey`). It does not write `jenkins-mtk-connect-apikey`. The post job and the API-key CronJob delete that Secret instead of refreshing the admin key. `cvdPipeline.groovy`, the OpenBSW `dev_instance` and `tests/posix` jobs, the Android `dev_instance_build` and `dev_instance_test` jobs, and both `delete_mtkc_testbench` jobs still bind that id and call `mtk_connect.sh`, which creates a testbench through the admin API. They fail at `withCredentials` until those jobs are moved onto an `MTKConnectTestbench` and bind `usernamePassword(credentialsId: 'mtkc-<namespace>-<testbench>-agent')`. The agent key cannot stand in for that admin call;
- it patches the server Deployment pod template only when the Keycloak key material changes;
- the existing `mtk-connect-post` job and `mtk-connect-api-key-config` CronJob keep running. Whichever writes last wins and every key stays valid until it is deleted, so both can run; the controller rewrites the distributed Secrets on every reconcile. The CronJob can be removed once the module is the only integration.
