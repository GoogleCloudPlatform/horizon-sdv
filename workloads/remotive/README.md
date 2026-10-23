# remotive-topology

Run [RemotiveTopology](https://developer.remotivelabs.com/) vehicle-system simulations
("system of systems") on ephemeral GCE VMs, following the Horizon SDV
Packer → KCC `ComputeInstanceTemplate` → ephemeral-GCE-driver pattern
(`cf_instance_template` + `cvd_launcher` parity).

GitHub repo [remotivelabs-topology-examples](https://github.com/remotivelabs/remotivelabs-topology-examples/)
includes several examples of RemotiveTopology platforms and instances.

## Layout

- `pipelines/environment/docker_image_template/` — builder container image
  (packer, gcloud, kubectl) used by all remotive workflow pods; builds via
  the shared `common-docker-image-build` ClusterWorkflowTemplate (module
  workloads-common).
- `pipelines/environment/remotive_instance_template/` — Packer bake
  (Docker CE, remotivebusd, remotivelabs-cli — translation of the upstream ansible
  roles) + KCC `ComputeInstanceTemplate` publish.
- `pipelines/tests/remotive_launcher/` — ephemeral VM driver (Path B: KCC
  `ComputeInstance` + GCS message passing, no SSH from the pod) and guest scripts
  that build and run the topology.

Shared GCP REST helpers are referenced from `workloads/android/pipelines/common/gcp/`
(generic library code; the monorepo is fully cloned in workflow pods regardless of
which modules are enabled).

## Workflow order

1. `remotive-builder-image` — once, and after Dockerfile changes.
2. `remotive-instance-template` — publishes `instance-template-<instanceName>`
   (default `instance-template-remotive-vm`).
3. `remotive-launcher` — per topology run. Takes the topology project as a
   downloadable archive (`topologyDownloadUrl`, see below); no topology project is
   shipped in this repository.

## One-time setup: RemotiveCloud auth Secret

Topology builds authenticate against RemotiveCloud. Create the Secret with a
**revocable, least-privilege service-account token** (never personal credentials):

```sh
kubectl -n <prefix>workflows create secret generic workflow-remotive-cloud-auth \
  --from-literal=token=<service-account-token> \
  --from-literal=organization=<organization-id>
```

The token is exported only into the topology process environment on the ephemeral
VM (not `/etc/environment` as in the upstream demo) and travels through the run's
GCS staging prefix like other job parameters — keep the token revocable.

## Providing the topology project

`remotive-launcher` requires two parameters that together identify the topology
project:

- `topologyDownloadUrl` (required) — a `gs://` or `https://` URL to a gzip tarball
  (`.tgz` or `.tar.gz`) of a self-contained topology project. The URL must end in
  one of those suffixes; directories, plain `.tar`, `.zip` and other formats are
  rejected by the workflow pod before a VM is booted.
- `topologyName` (required) — the project name. The archive is unpacked to
  `/opt/remotive/projects/<topologyName>` on the VM and the launcher looks for
  `<topologyName>.launcher.yaml` there.

The archive root must be the project root, i.e. the descriptor and the
`instances/`, `models/`, ... directories sit directly in the tarball, without a
wrapping top-level folder:

```sh
tar czf my_topology.tgz -C /path/to/my_topology .
gcloud storage cp my_topology.tgz gs://<bucket>/my_topology.tgz
```

`gs://` archives are downloaded on the VM with `gcloud storage cp` using the VM's
service account (the instance template's `SERVICE_ACCOUNT`, by default the
project's Compute Engine default service account), so the bucket must grant that
account object read access. `https://` archives are fetched with `curl` without
credentials and must be publicly readable or pre-signed.

The project must include a `<topologyName>.launcher.yaml` descriptor at the
project root (the path can be overridden with the optional `launcherFile`
workflow parameter, relative to the project root):

```yaml
topology_instances: # -f flags for `remotive topology build`, in order (required, non-empty)
  - instances/main.instance.yaml
compose_overlays: [] # extra docker compose -f files
compose_profiles: [] # long-running compose profiles
test_service: tester # compose service run when runTopologyTests=true
forward_ports: # topology ports exposed as MTK Connect tunnels
  - name: RemotiveBroker # tunnel name; required, like port
    port: 50051
adb_devices: # Android adb endpoints registered as streamed MTK Connect devices
  - name: IHU # MTK device name; required, like port
    port: 6520 # adb port published by the topology (cuttlefish default)
```

`forward_ports` lists only topology ports; each entry requires both `name` (used
as the MTK Connect tunnel name and in the printed access info) and `port`. The
primary UI, RemotiveStudio, is started by the launcher on port `57123` (see
below) — not a descriptor entry.

`adb_devices` lists Android adb endpoints (e.g. a cuttlefish instance's adb
port `6520`); each entry requires both `name` and `port`, and the port must be
published by the topology's compose so it is reachable on the VM. Each entry
becomes a full MTK Connect Android device — screen streaming, adb terminal,
logcat, touch — instead of the default HOST-only device (see below). Note that
the cuttlefish web UI (port `8443`) does **not** work through a `forward_ports`
tunnel: its WebRTC media stream cannot traverse a single-port TCP tunnel, which
is exactly what `adb_devices` is for.

The descriptor and a non-empty `topology_instances` list are required — the
workflow fails immediately when either is missing. The other keys are optional
(test service defaults to `tester`, no forwarded topology ports, no adb
devices).

## Accessing a running topology

All interactive access goes through **MTK Connect** tunnels and devices. This
requires the `workflow-mtk-connect-apikey` Secret in the workflows namespace
(created by the mtk-connect module's post job and kept in sync on key rotation)
— it is **required**; without it a keep-alive run has no access path.

During the keep-alive window (`keepAliveTime` minutes) the launcher starts
**RemotiveStudio** on port `57123` (bound to `0.0.0.0`, connected to the topology
broker on `localhost:50051`) as the primary UI. RemotiveStudio (`57123`) and the
topology's `forward_ports` (e.g. broker gRPC `50051`) are exposed through MTK
Connect as raw TCP tunnels. The testbench (`remotive-launcher-<n>`) registers one
`Remotive` device with:

- **`studio` (tunnel, raw TCP)** — for the MTK Connect Tunnel client: create a
  tunnel to the testbench on local port `57123`, then browse
  `http://localhost:57123` for RemotiveStudio.
- **one tunnel (raw TCP) per `forward_port`**, named after the descriptor `name`
  — e.g. `RemotiveBroker` for the broker gRPC port `50051`; tunnels to the same
  local port.
- **`HOST` (terminal)** — a shell on the VM as the image's interactive user.

With `adb_devices` in the descriptor, the testbench instead registers one MTK
Connect **Android device per entry** (named after the descriptor `name`), each
with live screen streaming, adb + HOST terminals, logcat and touch input — the
in-browser equivalent of the cuttlefish UI, without its WebRTC limitation. The
raw TCP tunnels above (studio + `forward_ports`) are attached to the first
device.

RemotiveStudio and MTK Connect are skipped entirely when `keepAliveTime` is `0`
(e.g. a pure test run), since there is no window in which to reach them.
RemotiveStudio is started fire-and-forget: it is not stopped in teardown but goes
away with the ephemeral VM. Its log is captured in the run artifacts
(`remotive-argo-studio.log`).

## Namespaces

- `<prefix>workflows` (platform namespace): the remotive Workflows and
  WorkflowTemplates, the `workflow-remotive-cloud-auth` and
  `workflow-mtk-connect-apikey` Secrets, and the ephemeral `ComputeInstance` CRs
  created per launcher run.
- `<prefix>remotive-kcc` (module-owned): the KCC `ComputeInstanceTemplate` CRs
  published by `remotive-instance-template`. The `mod-remotive-topology` parent
  chart creates this namespace before the child Application syncs and prunes it on
  disable. Keeping these CRs out of the shared workflows namespace means the
  workloads-android disable path (which removes _every_ `ComputeInstanceTemplate`
  in `<prefix>workflows`) no longer touches remotive templates. The launcher
  resolves the template by its GCP self-link, so it is unaffected by the namespace.

Disabling `remotive-topology` removes the remotive `ComputeInstanceTemplate` CRs:
Module Manager deletes every CR in `<prefix>remotive-kcc` as soon as the child
Application delete is issued and waits for Config Connector to delete the GCP
templates.

Horizon runs Config Connector in cluster mode, so the KCC namespace needs no
ConfigConnectorContext; `manageConfigConnectorContext` is only relevant for
namespaced-mode Config Connector and defaults to false.
