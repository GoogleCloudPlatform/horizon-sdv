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

# Container Image Security Upgrade Guide

This guide explains how teams running their **own fork/mirror** of Horizon SDV on an **older release** can remediate the vulnerable OS packages that a container security scanner flags inside the platform's container images, and how to **test** the simpler upgrade path in an existing environment before adopting a new release.

There are **two kinds** of image upgrade. You can apply them **together** (one deployment) or **separately** (phased). Both are covered here.

## Table of Contents

- [Overview](#overview)
- [How the image pipeline works](#how-the-image-pipeline-works)
- [Nginx version: single source of truth](#nginx-single-source-of-truth)
- [Prerequisites](#prerequisites)
- [Worked example: a multi-image CVE batch](#worked-example)
- [Section #1 - Version-bump upgrade (no code change)](#section-1---version-bump-upgrade)
- [Section #2 - Code-change upgrade (pull a newer release)](#section-2---code-change-upgrade)
- [Section #3 - Applying both: together or separately](#section-3---applying-both)
- [Section #4 - Testing container image security upgrades in an existing environment](#section-4---testing)
  - [Test script](#section-4-test-script)
  - [Section #4a - Probe package versions at a tag](#section-4a---probe)
  - [Section #4b - Bump, push, deploy](#section-4b---bump-push-deploy)
  - [Section #4c - Confirm Argo CD rolled out](#section-4c---confirm-argocd)
  - [Section #4d - Compare snapshots (optional)](#section-4d---compare)
  - [Section #4e - Live-workload spot check](#section-4e---live-spot-check)
- [Common pitfalls](#common-pitfalls)
- [Related documentation](#related-documentation)

> [!NOTE]
> This guide is written for **any** team consuming Horizon SDV. The image names, versions, and CVE examples are **illustrative**, taken from a real remediation batch — you do **not** need access to the issue tracker they came from. Map them to the findings in **your own** scanner report. Where a commit references an internal ticket ID, substitute your own.

---

<a id="overview"></a>

## Overview

Every platform image is built by Terraform from a Dockerfile under `terraform/modules/sdv-container-images/images/...`, tagged with a version defined in [`terraform/modules/base/locals.tf`](../../terraform/modules/base/locals.tf), pushed to your Artifact Registry, and deployed by Argo CD.

A scanner finding falls into one of two categories:

| Kind | What it needs | Why |
|------|---------------|-----|
| **Type A - Rebuild / republish only** | A **version bump** (no source edit) | The Dockerfile already runs `apk upgrade` / `apt upgrade`. Terraform rebuilds when the tag or the build-context hash changes, and `pull_parent` refreshes the `FROM` image. Docker reuses unchanged layers. A version bump republishes the image, but the upgrade step runs again only when that instruction or its parent layer changes. If the parent tag is unchanged, edit the upgrade line so the new packages are installed. |
| **Type B - Code change** | Pull the fix from a **newer release** into your fork | The image does not self-heal on a plain rebuild - e.g. a Dockerfile that never runs `apk upgrade`, a **pinned base image** that must move (nginx), or an **interpreter/base bump** (Python). These fixes live in the platform source and ship in a later release. |

> [!IMPORTANT]
> A plain redeploy of the **same** image tag fixes nothing. Terraform's `docker_image` resource only rebuilds when the image **name/version** or the build-context hash changes (see [`terraform/modules/sdv-container-images/main.tf`](../../terraform/modules/sdv-container-images/main.tf)). Both kinds of upgrade therefore **require a version bump** - Type A explicitly, Type B brings the bump with the upstream change.

---

<a id="how-the-image-pipeline-works"></a>

## How the image pipeline works

Both halves of the pipeline read from the **remote** branch named in `scm_repo_branch` in `terraform/env/terraform.tfvars`:

- The deployment script [`tools/scripts/deployment/container-deploy.sh`](../../tools/scripts/deployment/container-deploy.sh) **clones** that branch (via [`deploy.sh`](../../tools/scripts/deployment/deploy.sh)) before it builds images.
- Argo CD **syncs** the GitOps manifests from that **same** branch.

> [!WARNING]
> **Every change must be committed and pushed to that remote branch, or nothing rolls out.**
> If the branch/commit is not on the remote, Argo CD reports `ComparisonError: unable to resolve '<branch>' to a commit SHA`, stays on the old state, and your running pods keep the **vulnerable** image even though a patched image may already exist in the registry. This is the single most common mistake.

---

<a id="nginx-single-source-of-truth"></a>

## Nginx version: single source of truth

Horizon uses nginx in more than one place:

| Consumer | How nginx is supplied |
|----------|------------------------|
| **headlamp-token-injector** | Direct Docker Hub pull of `nginx:<tag>` |
| **module-overview-http** (sample / workload modules) | Direct Docker Hub pull of `nginx:<tag>` |

The **single source of truth** for that tag is:

```hcl
# terraform/modules/base/locals.tf
common_nginx_version = "1.31.2-alpine3.23"
```

That value is wired as follows:

1. **GitOps sidecars** — `base/main.tf` passes `common_nginx_version` into the `sdv-gke-apps` module. That module publishes it on the root Argo CD Application as `config.nginx.image` / `config.nginx.tag`.
2. **token-injector** — `gitops/templates/headlamp-token-injector.yaml` forwards `config.nginx` into the chart; the Deployment uses `{{ .Values.config.nginx.image }}:{{ .Values.config.nginx.tag }}`.
3. **module-overview** — `gitops/templates/module-manager.yaml` includes `nginx` in `MODULE_CONFIG` so enabled module charts receive the same `config.nginx` values.

To remediate a nginx CVE (Type B base bump):

1. Set `common_nginx_version` in [`terraform/modules/base/locals.tf`](../../terraform/modules/base/locals.tf) to the fixed Docker Hub tag (for example `1.31.2-alpine3.23`).
2. Commit, push, and deploy ([Section #1](#section-1---version-bump-upgrade) steps 2–5).
3. Verify with `nginx -v` on landingpage, token-injector, and module-overview pods (all should report the same nginx binary version). The probe script in [Section #4](#section-4---testing) covers **landingpage-app** in the registry; Docker Hub sidecars need a live `kubectl exec ... nginx -v` check.

> [!IMPORTANT]
> Do **not** hardcode `nginx:<tag>` in GitOps templates. Always consume `config.nginx` so every Horizon nginx consumer stays on the same pin.

---

<a id="prerequisites"></a>

## Prerequisites

- You operate a **fork/mirror** and deploy from your own branch (see [deployment_guide.md - Fork the Repository](../deployment_guide.md#section-5d---fork-the-repository)). Below, `upstream` = the public Horizon repository; `origin` = your fork.
- The existing environment is healthy: Argo CD applications `Synced` and `Healthy`.
- You can run [`deploy.sh`](../../tools/scripts/deployment/deploy.sh) (native) or [`container-deploy.sh`](../../tools/scripts/deployment/container-deploy.sh) (containerized) from `tools/scripts/deployment/`.
- Tooling: Terraform `>= 1.14.2`, Docker, `gcloud`, `kubectl` (minimums enforced by `deploy.sh`).

---

<a id="worked-example"></a>

## Worked example: a multi-image CVE batch

A real remediation batch flagged multiple images. Classifying each as Type A or Type B drives the rest of this guide:

| Kind | Mechanism | Images (example) |
|------|-----------|------------------|
| **A - bump only (self-heal)** | bump `build_version` / `deploy_version` | `gerrit-post`, `gerrit-mcp-server-app`, `mtk-connect-post`, `grafana-post`, `keycloak-post-jenkins`, `keycloak-post-argocd`, `keycloak-post-grafana`, `keycloak-post-headlamp`, `keycloak-post-mcp-gateway-registry`, `keycloak-post-mtk-connect` |
| **B - code change** | Dockerfile / base bump + version bump | `keycloak-post` (add `apk update && apk upgrade`), `keycloak-post-gerrit` (pull `openssh-client` from Alpine [edge](https://dl-cdn.alpinelinux.org/alpine/edge/main) for `>= 10.3`), `mtk-connect-post-key` (base -> `python:3.13-slim-bookworm`); nginx sidecars via `common_nginx_version` (e.g. `1.28.1` -> `1.31.2-alpine3.23`) |

> [!NOTE]
> The split above is specific to this batch. **Derive your own:** for each flagged image, check whether a fresh rebuild already lands the required version. If yes, it is Type A; if not, it is Type B.

---

<a id="section-1---version-bump-upgrade"></a>

## Section #1 - Version-bump upgrade (no code change)

This is the **Type A** path. It can be done on your **current** Horizon version, with no upstream pull.

1. On your fork's deployment branch, edit [`terraform/modules/base/locals.tf`](../../terraform/modules/base/locals.tf). For **each flagged Type A image**, bump **both** versions, e.g. `1.0.0` -> `1.0.1`:

   ```hcl
   "gerrit-post" = {
     directory      = "gerrit"
     build_version  = "1.0.1"   # tag Terraform builds + pushes to Artifact Registry
     deploy_version = "1.0.1"   # tag Argo CD deploys / post-jobs consume
   }
   ```

   > [!IMPORTANT]
   > Bump **both** `build_version` and `deploy_version`. Bumping only `build_version` builds a new image that never gets deployed.

2. **Commit and push** to the remote branch referenced by `scm_repo_branch` (substitute your own ticket ID):

   ```bash
   git add terraform/modules/base/locals.tf
   git commit -m "<TICKET-ID>: bump <images> to <new-version> to clear scanner findings"
   git push origin <your-branch>
   ```

3. Deploy (rebuilds the bumped images, reusing unchanged layers, and pushes them to your registry):

   ```bash
   ./container-deploy.sh --apply      # or: ./deploy.sh --apply
   ```

4. Argo CD detects the changed manifests on the branch and rolls out:
   - long-running **Deployments** move to the new tag (`imagePullPolicy: Always`), and
   - **post-job** sync hooks re-run at the new tag.

5. **Verify** the running packages - see [Section #4 - Testing](#section-4---testing).

These Dockerfiles already run `apk`/`apt upgrade`. With layer caching on, that step is reused when the parent image is unchanged, so a version bump alone does not pull packages published after the last build. Change the upgrade instruction, or move the parent tag, before deploying if the finding is a new OS package.

---

<a id="section-2---code-change-upgrade"></a>

## Section #2 - Code-change upgrade (pull a newer release)

This is the **Type B** path. The fixes are in the platform **source** and ship in a later Horizon release. Bring them into your fork with one of:

- **Cherry-pick (most surgical - recommended for a single security batch):** take only the fix commit(s).
  ```bash
  git fetch upstream
  git cherry-pick <fix-commit-sha> [<more-shas>...]
  ```
- **Merge (preserves history; brings the whole release):** expect conflicts in `terraform.tfvars`, Dockerfiles, GitOps.
  ```bash
  git fetch upstream
  git merge <upstream-release-branch-or-tag>
  ```
- **Rebase (linear history; riskier on a shared/deployed branch - rewrites commits):**
  ```bash
  git fetch upstream
  git rebase <upstream-release-branch-or-tag>
  ```

> [!NOTE]
> The release-to-release [upgrade guides](README.md#upgrade-guides) document `git fetch / checkout / pull` onto a release branch and then **manually** aligning `terraform/env/terraform.tfvars` against the new `terraform.tfvars.sample`. They do **not** prescribe a merge/rebase strategy for a customized fork - the three options above fill that gap. After integrating, re-align your `terraform.tfvars` with the new sample (for example, a recent release renamed `enable_arm64` to `enable_arm64_dedicated_subnet`).

The upstream change already carries the bumped versions for the Type B images. After integrating and pushing, deploy and verify exactly as in [Section #1](#section-1---version-bump-upgrade) (steps 2-5).

Type B specifics worth validating after rollout (from the example batch):
- `keycloak-post-gerrit`: `openssh` comes from the Alpine **edge** branch (which also bumps **musl** image-wide) - sanity-check SSH and the Node runtime.
- `mtk-connect-post-key`: now on **Python 3.13** - sanity-check the job/script runs.
- nginx sidecars (token-injector, module-overview): pin via `common_nginx_version` (see [Nginx version: single source of truth](#nginx-single-source-of-truth)).


---

<a id="section-3---applying-both"></a>

## Section #3 - Applying both: together or separately

- **Together (recommended once the target release is available):** pull the upstream release ([Section #2](#section-2---code-change-upgrade)). It already includes the version bumps for **all** flagged images (Type A *and* Type B), so a **single** `--apply` + push remediates everything.
- **Separately / phased (recommended when you cannot take the full release yet):**
  1. **Now:** run [Section #1](#section-1---version-bump-upgrade) on your current version - immediately clears the self-healing (Type A) images.
  2. **Later:** run [Section #2](#section-2---code-change-upgrade) for the code-change (Type B) images after you adopt the target release.

---

<a id="section-4---testing"></a>

## Section #4 - Testing container image security upgrades in an existing environment

Use this to validate security upgrades against an **existing, older** environment — whether you applied **Type A only** (version bump), **Type B only** (code change), or **both** ([Section #3](#section-3---applying-both)). Validation is by **direct package probing with `kubectl`**, so you do not depend on the scanner's cadence to know whether the fix landed.

> [!NOTE]
> **Scope: flagged images in the probe script.** The `probe` command package-checks every image listed in `ALPINE_IMAGES` / `DEBIAN_IMAGES` (see the [worked example](#worked-example)). Run it **at any time** for any tag; save output with `-o` and `diff` snapshots yourself when comparing before/after. If you are testing a **phased Type A-only** rollout, remove the Type B image names from those lists first.

> [!NOTE]
> **Test on an older env, not a fresh one.** A freshly deployed environment auto-pulls the latest packages anyway, so it cannot demonstrate the "stale -> patched" delta. An **existing older** environment still carries the flagged versions, so it is the only true test bed.

<a id="section-4-test-script"></a>

### Test script

All probe, Argo CD, and spot-check commands are in:

[`tools/scripts/container-images/container-image-version-bump-test.sh`](../../tools/scripts/container-images/container-image-version-bump-test.sh)

**Commands:** `probe` (main — run anytime), `argocd`, `spot-check`, `cleanup`.

Make it executable, set your environment, then run:

```bash
chmod +x tools/scripts/container-images/container-image-version-bump-test.sh

export PROJECT=<PROJECT_ID>
export REGION=<gcp-region>          # e.g. us-central1
export CTX=connectgateway_${PROJECT}_${REGION}_sdv-cluster

gcloud container fleet memberships get-credentials sdv-cluster --project="${PROJECT}"
kubectl config use-context "${CTX}"

# Before upgrade — save a snapshot (optional: add --with-argocd)
./tools/scripts/container-images/container-image-version-bump-test.sh probe --tag 1.0.0 --with-argocd -o before_1.0.0.txt

# ... Section #4b: bump, push, deploy ...

# After upgrade — probe again and compare yourself
./tools/scripts/container-images/container-image-version-bump-test.sh probe --tag 1.0.1 -o after_1.0.1.txt
diff -u before_1.0.0.txt after_1.0.1.txt

./tools/scripts/container-images/container-image-version-bump-test.sh argocd --tag 1.0.1
./tools/scripts/container-images/container-image-version-bump-test.sh spot-check --tag 1.0.1
./tools/scripts/container-images/container-image-version-bump-test.sh cleanup
```

Pass `--help` for all flags. The script ships with the images from the worked example; edit `ALPINE_IMAGES` / `DEBIAN_IMAGES` at the top if your CVE batch differs.

<a id="section-4a---probe"></a>

### Section #4a - Probe package versions at a tag

```bash
./tools/scripts/container-images/container-image-version-bump-test.sh probe --tag <TAG> [-o snapshot.txt] [--with-argocd]
```

This spins up ephemeral pods from the flagged images at `:<TAG>` in your registry and prints the flagged OS package versions. Use `-o` to save output; omit it to print to stdout. Add `--with-argocd` to include the image references Argo CD is deploying (useful before an upgrade).

Run once before the upgrade (`--tag 1.0.0`) and again after (`--tag 1.0.1`) when validating a rollout. You can also run `probe` anytime to audit the current state of images in the registry.

<a id="section-4b---bump-push-deploy"></a>

### Section #4b - Bump, push, deploy

In [`terraform/modules/base/locals.tf`](../../terraform/modules/base/locals.tf), bump `build_version` **and** `deploy_version` for **every flagged image** you are testing (full remediation, or Type A only for a phased test — see scope note above), then commit, push, and deploy:

```bash
git add terraform/modules/base/locals.tf
git commit -m "TEST: bump flagged images to ${NEW_TAG} (security upgrade validation)"
git push origin <your-test-branch>

./container-deploy.sh --apply      # or: ./deploy.sh --apply
```

> The push is mandatory - Terraform clones, and Argo CD syncs, from the **remote** branch.

<a id="section-4c---confirm-argocd"></a>

### Section #4c - Confirm Argo CD rolled out

```bash
./tools/scripts/container-images/container-image-version-bump-test.sh argocd --tag <TAG>
```

Expect `sync=Synced`, `health=Healthy`, `rev=<a commit SHA>`, and **no** `ComparisonError`. If you see `unable to resolve '<branch>' to a commit SHA`, your branch/commit is not on the remote - push it. With `--tag`, the script also lists workloads referencing that tag.

<a id="section-4d---compare"></a>

### Section #4d - Compare snapshots (optional)

After running `probe` before and after the upgrade, compare the saved files:

```bash
diff -u before_1.0.0.txt after_1.0.1.txt
```

Every flagged package should have moved from its old version to the patched one. Example deltas from the worked batch:

| Image / package | BEFORE | AFTER |
|-----------------|--------------------|-------------------|
| Alpine `curl` (most images) | `8.17.x-r*` | `8.19.0-r0` |
| Alpine `libssl3` / `libcrypto3` | `3.5.5-r0` | `3.5.6-r0` |
| Alpine `vim` | older | `>= 9.2.0321-r0` |
| Debian `libssl3` / `openssl` | older `~deb12u*` | latest `~deb12u*` |
| **landingpage-app** `nginx` | older pin (e.g. `1.28.1-r1`) | current `common_nginx_version` (e.g. `1.31.2-r*`) |
| **landingpage-app** `libexpat` / `libpng` | `2.7.3` / `1.6.54` | `2.7.5-r0` / `1.6.58-r1` |
| **keycloak-post-gerrit** `openssh` | `10.2_p1-r0` | `10.3_p1-r0` |
| **mtk-connect-post-key** Python | `3.12.x` | `3.13.x` |

<a id="section-4e---live-spot-check"></a>

### Section #4e - Live-workload spot check

Probing `:$NEW_TAG` proves the **registry image** is patched. To prove the **running** workload was actually rolled:

```bash
./tools/scripts/container-images/container-image-version-bump-test.sh spot-check --tag <TAG>
```

By default this checks:
- **landingpage** Deployment (namespace `horizon`) — Type B; nginx + Alpine packages in the running pod
- **gerrit-mcp-server** Deployment (namespace `gerrit`) — Type A; Debian openssl packages
- **mtk-connect-api-key-config** CronJob (namespace `mtk-connect`) — Type B; forced one-off run on Python 3.13 image

When validating an nginx base bump, also confirm Docker Hub consumers that share `config.nginx` (not covered by the probe script's Artifact Registry list):

```bash
kubectl -n headlamp exec deploy/headlamp-token-injector -- nginx -v
kubectl get deploy -A -l app.kubernetes.io/name=module-overview \
  -o custom-columns=NS:.metadata.namespace,NAME:.metadata.name,IMAGE:.spec.template.spec.containers[0].image
# then: kubectl -n <ns> exec deploy/<overview> -- nginx -v
```

Override with `--gerrit-ns`, `--mtk-connect-ns`, `--mtk-cronjob`, or pass `--skip-cronjob` to skip the CronJob step.

> [!NOTE]
> Argo CD post-job **sync hooks** (e.g. `keycloak-post-*`) run during sync and are auto-deleted on success; the parent app being `Synced/Healthy` confirms they completed at the new tag.

**Cleanup:**

```bash
./tools/scripts/container-images/container-image-version-bump-test.sh cleanup
```

+**What this proves:** the images in the registry contain patched packages for the flagged images at the tag you probed, Argo CD rolled them out, and long-running workloads plus the `mtk-connect-post-key` CronJob are running the expected tag. Post-job sync hooks are confirmed by Argo CD `Synced/Healthy` plus a `probe` at the new tag.

---

<a id="common-pitfalls"></a>

## Common pitfalls

| Symptom | Cause | Fix |
|--------|-------|-----|
| Argo CD `ComparisonError: unable to resolve '<branch>' to a commit SHA`; pods stay on old image | Branch/commit not pushed to remote | `git push origin <branch>` |
| New image in registry but running pods unchanged | Bumped `build_version` only | Also bump `deploy_version` |
| Nothing rebuilds after deploy | Reused the same tag | Bump the version ([Section #1](#section-1---version-bump-upgrade)) or pull the upstream change ([Section #2](#section-2---code-change-upgrade)) |
| Old-tag `Completed` Job/CronJob pods still present | Historical job runs | Harmless; they age out via the job history limit |
| token-injector / module-overview still on an old `nginx:` tag | Sidecars not reading `config.nginx`, or `common_nginx_version` not passed into root GitOps values | Update `common_nginx_version` only in `locals.tf` and keep consumers on `config.nginx` ([Nginx version: single source of truth](#nginx-single-source-of-truth)) |

---

<a id="related-documentation"></a>

## Related documentation

| Area | Documentation |
|------|----------------|
| Greenfield deployment & forking | [deployment_guide.md](../deployment_guide.md) |
| Release-to-release upgrades | [guides/README.md](README.md#upgrade-guides) |
| Terraform variables | [terraform.md](../terraform.md) |
| Image versions & build args | [terraform/modules/base/locals.tf](../../terraform/modules/base/locals.tf) |
| Nginx pin (`common_nginx_version` → `config.nginx`) | [Nginx version: single source of truth](#nginx-single-source-of-truth) |
| Image build behaviour (`no_cache`, triggers) | [terraform/modules/sdv-container-images/main.tf](../../terraform/modules/sdv-container-images/main.tf) |
| Image build & version-bump test script | [tools/scripts/container-images/](../../tools/scripts/container-images/) |
| Container image security upgrade test script (Section #4) | [container-image-version-bump-test.sh](../../tools/scripts/container-images/container-image-version-bump-test.sh) |
| Deployment scripts | [tools/scripts/deployment/](../../tools/scripts/deployment/) |
