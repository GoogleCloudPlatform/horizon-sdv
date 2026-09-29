<!--
Copyright (c) 2026 Accenture, All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

        http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
-->

# aaos-sdv-dev-env

Builds, snapshots and validates the **AAOS SDV Development Environment** Cloud Workstation image:
Android Studio for Platform (ASfP) plus CARLA 0.9.15, a SOME/IP bridge and AAOS
demo utilities.

The Dockerfile and every asset it copies live in the public repository
[`aaos-sdv-demos`](https://github.com/google/aaos-sdv-demos) and are cloned at run
time. Nothing is vendored in this repository, and no credentials are needed.

## Layout

```text
gitops/modules/aaos-sdv-dev-env/
├── Chart.yaml
├── values.yaml                               # Module Manager injection schema
├── portal/overview.html                      # Developer Portal page
├── templates/
│   ├── module-overview-http.yaml
│   └── application-argo-workflows.yaml       # child Argo CD Application
└── argo-workflows/
    ├── Chart.yaml
    ├── values.yaml                           # all deploy-time configuration
    ├── files/                                # step bodies, mounted at /scripts
    │   ├── build-snapshot.sh
    │   ├── test-artifacts.sh
    │   ├── promote-snapshot.sh
    │   └── test_artifacts.bats               # acceptance suite, runs on the workstation
    └── templates/
        ├── _helpers.tpl
        ├── workflowtemplates.yaml            # aaos-sdv-dev-env-init + aaos-sdv-dev-env-execute
        ├── configmap-scripts.yaml            # publishes files/ to the workflow namespace
        └── sensors.yaml
```

Step bodies live under `files/` rather than as inline `args:` blocks so they can be
read, diffed, shell-linted and executed outside the cluster. This mirrors
`workloads-common/prepare-github-app-git-creds`, which mounts
`files/github_app_installation_token.py` the same way.

## Pipelines

### `aaos-sdv-dev-env-init`

Preflight. Checks that `aaos-sdv-demos` is reachable and that the configured
revision exists. Run this first: it fails in seconds where the build would fail
after tens of minutes.

### `aaos-sdv-dev-env-execute`

| # | Task | What it does |
|---|------|--------------|
| 1 | `build-image` | Builds and pushes the workstation image via `docker/docker-bake.hcl` in `aaos-sdv-demos`, using the shared `common-docker-image-build` ClusterWorkflowTemplate (`docker buildx bake` against its `buildkitd` sidecar). |
| 2 | `build-snapshot` | Provisions a builder workstation, then snapshots its persistent disk. |
| 3 | `test-artifacts` | Boots a GPU test workstation from the snapshot and runs `files/test_artifacts.bats` on it. |
| 4 | `promote-snapshot` | Moves the `is-latest=true` label onto the validated snapshot **and** publishes the `aaos-sdv-dev-env-latest` workstation config built from it. |

The acceptance suite is uploaded to the test workstation as a file and executed
with `bats`, rather than being inlined into a `gcloud workstations ssh --command`
string. `bats` is expected to be present in the image; it is installed by the
`aaos-sdv-demos` Dockerfile.

## Using the result

The pipeline's deliverable is the **workstation config** `aaos-sdv-dev-env-latest`, not a
running workstation. Creating workstations from it is deliberately left to the
developer — there is no launch pipeline, because a workstation is a long-lived,
per-person, billable resource whose lifecycle should not be tied to a CI run.

Launch one from the Cloud Console (**Cloud Workstations → Workstations → Create**,
choosing the `aaos-sdv-dev-env-latest` config), or from the CLI:

```bash
gcloud workstations create "${USER}-aaos-sdv-dev-env" \
    --project="${PROJECT}" --region="${REGION}" \
    --cluster=sdv-cluster --config=aaos-sdv-dev-env-latest

gcloud workstations start "${USER}-aaos-sdv-dev-env" \
    --project="${PROJECT}" --region="${REGION}" \
    --cluster=sdv-cluster --config=aaos-sdv-dev-env-latest
```

The remote desktop is served on port 80, i.e. at the workstation's own hostname.
It is a headless X server captured by Selkies and encoded on the T4's NVENC, not
the GNOME/Guacamole session inherited from the base image — the demo layer masks
that stack deliberately, since compositing a GNOME session on this hardware falls
back to llvmpipe. See `docker/README.md` in `aaos-sdv-demos` for the full rationale.

```bash
gcloud workstations describe "${USER}-aaos-sdv-dev-env" \
    --project="${PROJECT}" --region="${REGION}" \
    --cluster=sdv-cluster --config=aaos-sdv-dev-env-latest \
    --format='value(host)'
# -> https://<host>
```

`/home` is restored from the promoted snapshot, so `~/Workspace/carla-installation`
and a fully built `~/Workspace/aaos-26q2` are present on first boot. See
`/google/sdv-bashrc-hook/README` on the workstation for the demo quickstart
(`launch_2vm`, then `launch_carla`).

Remember to stop workstations when idle — the config requests an `n1-standard-32`
with an attached NVIDIA T4.

## Why `gcloud` and not Config Connector (KCC)

Per the horizon-dev module conventions, a module should declare its GCP resources
as Config Connector CRs rather than Terraform. This module declares no GCP
resources at all: it needs no IAM of its own (see below), and it drives Cloud
Workstations and snapshots through `gcloud`. The reasons are specific rather than
stylistic:

* **No CRDs exist for most of it.** The cluster runs the GKE Config Connector
  add-on at **v1.126.0**, which ships `WorkstationCluster` but neither
  `WorkstationConfig` nor `Workstation`. Upstream KCC only promoted
  `WorkstationConfig` to `v1beta1` in **1.132.0**, and the GKE add-on omits alpha
  CRDs. Its version is chosen by Google and tied to the GKE control-plane version,
  so it cannot be bumped from this repo.
* **The disk name only exists at runtime.** The snapshot source is discovered with
  `gcloud compute disks list --filter=labels.workstation_id=…` after the builder
  workstation has started. A Helm-rendered CR cannot reference it.
* **The lifecycle is one-shot, not reconciled.** `create → start → stop → snapshot
  → delete` is imperative by nature, and KCC's default deletion policy would
  delete the promoted snapshot along with its CR.

The one resource that *would* suit KCC is the long-lived `aaos-sdv-dev-env-latest`
config produced by `promote-snapshot`. Revisit this if the cluster ever moves to
Config Connector ≥ 1.132.

## IAM

The pipeline runs as `workflows/workflow-executor-elevated`, bound by Workload
Identity to `gke-argo-workflows-elevated-sa` (selected by
`spec.useElevatedWorkflowIam: true`). Beyond what every pipeline uses —
`roles/artifactregistry.writer` to push the image — it needs:

| Role | Used for |
|------|----------|
| `roles/workstations.admin` | create / start / stop / delete workstations and workstation configs |
| `roles/compute.instanceAdmin.v1` | list the workstation persistent disk, create the snapshot, label it |

**This module adds no IAM of its own, because it does not have to.** The platform
Terraform already grants both roles to that service account — see the
`gke-argo-workflows-elevated-sa` entry in `terraform/env/main.tf`, and the
`argo_workflows_elevated` service-account template in `terraform/env/locals.tf`
that sub-environments are generated from. A stock deployment therefore needs no
IAM changes to run this module.

> Deployments that narrow the platform role set must re-add these two roles;
> without them `build-snapshot` fails at `gcloud workstations create` or at
> `gcloud compute disks list`.

An earlier revision of this module shipped the grants as Config Connector
`IAMPolicyMember` CRs. That was removed, and is not worth reinstating:

* It is redundant — the roles are already present, as above.
* It cannot work by default. Config Connector acts as `gke-config-connector-sa`,
  which the platform grants `roles/editor`; basic Editor does **not** include
  `resourcemanager.projects.setIamPolicy`, so the CRs fail to reconcile with
  `Error 403: The caller does not have permission`.
* That failure is worse than a no-op. The CRs sit in **sync-wave 1**, and Argo CD
  will not advance to later waves while a wave is unhealthy — so a failing grant
  silently prevents the WorkflowTemplates (wave 7) and Sensors (wave 8) from
  being created at all. The module would appear to deploy, and then not exist.

## Parameters

| Parameter | Required | Description |
|-----------|----------|-------------|
| `horizonSubmittedFrom` | no | Populated by the Developer Portal / Horizon CLI. Leave empty. |

There are no credential parameters: the source repository is public. Everything
else — repository URL and revision, image names, workstation cluster and builder
config — is deploy-time configuration in
[`argo-workflows/values.yaml`](argo-workflows/values.yaml), not a submit-time
parameter.

> [!IMPORTANT]
> Parameters are mapped **positionally** by the Sensors
> (`spec.arguments.parameters.N.value`). If you add, remove or reorder a
> parameter in `workflowtemplates.yaml`, you must update `sensors.yaml` to match.

## Images

This module builds everything it consumes using `docker/docker-bake.hcl` from `aaos-sdv-demos`:

```text
cloud-workstations predefined/base
  └── preflight + common   Cloud Workstations systemd contract & preflight UI
        └── remote-desktop (INSTALL_GUACAMOLE=false)   nginx, Chrome, openssh
              └── android-studio-for-platform   ASfP canary, Cuttlefish
                    └── aaos-sdv-dev-env (aaos-sdv-demos)
```

| Role | Artifact Registry Path |
|------|------------------------|
| Output workstation image | `<region>-docker.pkg.dev/<project>/horizon-sdv/aaos-sdv-dev-env:latest` |
| BuildKit registry cache | `<region>-docker.pkg.dev/<project>/horizon-sdv/aaos-sdv-dev-env-cache:latest` |

### Why `docker/docker-bake.hcl` builds the base chain

`aaos-sdv-demos/docker/docker-bake.hcl` wires `android-studio-for-platform` directly
onto `remote-desktop` (`INSTALL_GUACAMOLE=false`), omitting the `gnome` layer
entirely (`ubuntu-desktop-minimal`, Mutter, GNOME Shell 46, and
`gnome-remote-desktop` are never installed) and replacing Guacamole with a
headless X11 + openbox + tint2 + Selkies NVENC desktop stack.

Because `docker buildx bake` compiles all five targets into a single unified
BuildKit LLB graph against the `buildkitd` sidecar, intermediate targets link in
memory without separate registry pushes/pulls, and BuildKit caches all layers in
`CACHE_REPO` (`type=registry,mode=min`) across runs.

## Dependencies

Hard dependency on **`workloads-common`**, which publishes the
`common-docker-image-build` ClusterWorkflowTemplate (with `bakeFile` support).

## Verifying changes

```bash
helm lint gitops/modules/aaos-sdv-dev-env
helm lint gitops/modules/aaos-sdv-dev-env/argo-workflows
helm template aaos-sdv-dev-env gitops/modules/aaos-sdv-dev-env/argo-workflows \
  --set parentModuleName=aaos-sdv-dev-env \
  --set gcpProjectId=<project> --set gcpRegion=<region>

kubectl get modulecatalog cluster -n module-manager -o yaml
horizon catalog get
horizon workflow submit --module aaos-sdv-dev-env --template aaos-sdv-dev-env-init --output json
```
