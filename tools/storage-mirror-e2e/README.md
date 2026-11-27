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

# Storage Mirror end-to-end harness

Runs the `storage-mirror` module (`gitops/modules/storage-mirror-module`) against a local cluster without GCP. It never
touches GKE, Filestore, Config Connector or any existing kubectl context.

| Mode | Needs | What runs |
|---|---|---|
| `run.sh` (kind) | Docker usable by your user, kind, the host `nfsd` kernel module, internet | Real images, real Argo Workflows `v3.7.10` and Argo Events `v1.7.6` (the platform chart pins `0.47.4` / `2.3.2`), an in-cluster NFS server instead of Filestore, Gitea over HTTPS with LFS instead of the upstreams |
| `run.sh --apiserver-only` | Go, envtest binaries (`setup-envtest`), no container runtime | A throwaway kube-apiserver + etcd with the module CRDs, Argo Workflows **full-schema** CRDs and Argo Events CRDs; the controller runs as a local process; Argo is simulated by creating and patching Workflow objects; a local HTTP server stands in for the Argo Events webhook |
| `run.sh --render-only` | helm, python3 + PyYAML | Renders and patches the manifests into `/tmp/storage-mirror-e2e/render` |

## Safety

- All state lives under `/tmp/storage-mirror-e2e` (`WORK_DIR`). `KUBECONFIG` is unset; every `kubectl` and `helm` call
  passes `--kubeconfig ${WORK_DIR}/kubeconfig` and the dedicated context (`kind-storage-mirror-e2e`, or the envtest
  context).
- Before anything is applied, `assert_local_cluster` checks that the API server URL is `127.0.0.1`, `localhost` or
  `[::1]`, and stops otherwise.
- The kind cluster is deleted at the end of every run unless `--keep` (or `--reuse`) is given. The apiserver mode stops
  its control plane, the controller and the fake webhook on exit.
- No `gcloud`, no Terraform, no `docker push`. Images are built locally and loaded with `kind load docker-image`.

## Prerequisites

```bash
GOBIN=/tmp/storage-mirror-e2e-tools go install sigs.k8s.io/kind@v0.27.0
GOBIN=/tmp/storage-mirror-e2e-tools go install helm.sh/helm/v3/cmd/helm@v3.16.4
GOBIN=/tmp/storage-mirror-e2e-tools go install sigs.k8s.io/controller-runtime/tools/setup-envtest@release-0.23   # --apiserver-only
```

`kubectl`, `jq`, `curl`, `python3` with PyYAML. For the kind mode: `docker info` must work for your user (member of
the `docker` group, or rootless Docker), and the host must provide `nfsd` (`sudo modprobe nfsd`). The kind nodes get
`nfs-common` installed by the harness (kubelet mounts the NFS volumes).

## Run

```bash
tools/storage-mirror-e2e/run.sh                      # full kind run, cluster deleted afterwards
tools/storage-mirror-e2e/run.sh --keep               # keep the cluster for debugging
tools/storage-mirror-e2e/run.sh --reuse --skip-build --only create-repo --only sync
tools/storage-mirror-e2e/run.sh --apiserver-only     # no Docker
```

Scenario names for `--only`: `cel volume vap flock create-repo create-git-lfs create-git-credentials sync batch mutex cancel restart retain
purge`. The run ends with a PASS/FAIL/SKIP table and a non-zero exit code if anything failed.

## What it changes for the test

`render.py` applies test-only patches to the `helm template` output; nothing in the charts is test-specific.

- The controller gets `TEST_NFS_SERVER` / `TEST_NFS_PATH`. With them it skips the Config Connector
  `FilestoreInstance` and creates the same static PVs and claims with an `nfs:` source
  (`internal/volume.PersistentVolumeTestNFS`). The variable is never set by the module chart.
- Argo CD Applications are dropped: the child chart is rendered with the exact values the parent passes to
  `mod-storage-mirror-argo-workflows`.
- WorkflowTemplate containers get `GIT_SSL_NO_VERIFY=true` because Gitea uses a self-signed certificate.
- Resource requests are small (`values/storage-mirror.yaml`).

## Coverage

| Check | kind | apiserver-only |
|---|---|---|
| CRD CEL rules (config name, capacity, immutable fields, kind/spec match) against a real API server | yes | yes |
| Admission policy: only `storage-mirror-sync` may mount `storage-mirror-rw` | yes | yes |
| Module, WorkflowTemplates and Sensors accepted with strict field validation (Argo full schema incl. `workflowMetadata.labelsFrom`, plural `semaphores` / `mutexes`) | yes | yes |
| Static PVs and claims (RO for builders and Jenkins) | yes (Bound) | yes (created; no PV controller) |
| volume-init dispatched once; catalog reported | yes | dispatch only |
| create, sync, Ready, Error, history, live progress through the controller API | real runs | simulated Workflows + agent progress API |
| A create Workflow without a StorageMirror (posted to the webhook) is not adopted | n/a | yes |
| Purge (delete run, then CR released) and Retain delete | yes | yes |
| repo mirror (`repo init --mirror`, prune-refs), git mirror, git-lfs auto detection and fetch | yes | no |
| git mirror of a private repository through a credentials Secret (`run-git-credentials`) | yes | no |
| `metadata.yaml` on NFS in the Jenkins format; `flock` on the NFS share | yes | no |
| Argo `labelsFrom` labels on real Workflows, semaphore (max 1 Running), per-mirror mutex | yes | no |
| Cancel, controller restart mid-sync | yes | no |
| Argo CD `Delete=false` on CRDs | no (no Argo CD) | no |
| Horizon CLI end to end | no (needs Horizon API + Keycloak; the CLI is unit-tested) | no |

kind nodes share the host kernel, so the `flock` check proves that locks work on the NFS mount, not locking between
two different NFS client machines.

## Layout

```text
run.sh             entry point (modes, cluster lifecycle, installs)
lib.sh             k / h wrappers with the pinned kubeconfig, loopback assertion, result table
scenarios.sh       kind scenarios (sc_*) and apiserver-mode scenarios (sc_sim_*)
render.py          test-only patches of the rendered charts
fake-webhook.py    Argo Events webhook stand-in for --apiserver-only
apiserver/         Go helper starting the envtest control plane
kind-config.yaml   two nodes on 127.0.0.1; the worker carries workloadLabel=android
manifests/         infra (NFS server, Gitea), platform slice (namespaces, Argo Events RBAC, EventBus, EventSource), seed.sh
values/            chart values for the harness
```
