<!-- Copyright (c) 2026 Accenture and Google LLC, All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

        http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License. -->

# Phase 3: Deployment Execution

This guide covers the execution of Terraform workflows using the deployment scripts in `tools/scripts/deployment`. The standard scripts (§2 and §3) are the default; the Cloud Build runner (§4) is an opt-in variant (see `rules/AGENTS.md`, rule 4).

---

## 1. Prerequisites Check

Before running the deployment:
- `terraform/env/terraform.tfvars` must exist and have all required fields filled in.
- `gcloud` must be authenticated to your target GCP Project.

---

## 2. Linux Native Deployment (Default)

> [!IMPORTANT]
> The native script `deploy.sh` is intended solely for Debian/Ubuntu-based Linux environments.

The agent may run `deploy.sh` itself when the machine is Linux and the local tools meet the minimum versions that `deploy.sh` checks (Terraform, Docker, gcloud). `deploy.sh` also checks ADC authentication before running Terraform.

```bash
cd tools/scripts/deployment

# Plan
./deploy.sh -p

# Apply
./deploy.sh -a

# Destroy
./deploy.sh -d
```

---

## 3. Containerized Deployment (Default, Run by the User)

The containerized workflow packages all required CLI dependencies (`terraform`, `kubectl`, `gcloud`, etc.) inside a local container image (`horizon-sdv-deployer:latest`). `container-deploy.sh` starts the container with `docker run -it`, so it needs an interactive terminal: hand the commands to the user to run.

### Running Commands
The script must be run from `tools/scripts/deployment` (it builds `./container/Dockerfile` relative to the current directory):

```bash
cd tools/scripts/deployment

# 1. Preview changes (Terraform Plan)
./container-deploy.sh -p

# 2. Provision / Update infrastructure (Terraform Apply)
./container-deploy.sh -a

# 3. Destroy infrastructure (Deprovision)
./container-deploy.sh -d
```

### Environment Compatibility
- **Google Cloud Shell**
- **Linux** (Ubuntu, Debian, RHEL, CentOS)
- **Windows** (via WSL2)
- **macOS** (with Docker Desktop / Colima)

---

## 4. Google Cloud Build Deployment (Opt-in Variant)

The Cloud Build workflow runs the containerized Terraform on Google Cloud Build infrastructure (`E2_HIGHCPU_8` workers) in your project. Use it only if the user asks for it or no local tooling is available, and state its differences from `deploy.sh` first: it runs on Cloud Build, runs as the Cloud Build service account, and needs extra setup (cloudbuild/orgpolicy APIs, `roles/owner` grants, source staging in the state bucket). See [06_cloud_build_deployment.md](./06_cloud_build_deployment.md).

> [!TIP]
> **No Local Machine Dependencies**: You do **not** need Docker, Brew, Terraform, or kubectl installed on your local laptop. The script submits `terraform.tfvars`, clones `scm_repo_url` at `scm_repo_branch` in Cloud Build (as `deploy.sh` does in container mode), and streams real-time logs back to your terminal.

### Running Commands
From the repository root:

```bash
# 1. Preview changes (Terraform Plan on Google Cloud Build)
./.agents/plugins/horizon-ops/skills/horizon-deploy/scripts/cloud-deploy.sh -p
# or: ./.agents/plugins/horizon-ops/skills/horizon-deploy/scripts/cloud-deploy.sh --plan

# 2. Provision / Update infrastructure (Terraform Apply on Google Cloud Build)
./.agents/plugins/horizon-ops/skills/horizon-deploy/scripts/cloud-deploy.sh -a
# or: ./.agents/plugins/horizon-ops/skills/horizon-deploy/scripts/cloud-deploy.sh --apply

# 3. Destroy infrastructure (Deprovision)
./.agents/plugins/horizon-ops/skills/horizon-deploy/scripts/cloud-deploy.sh -d
# or: ./.agents/plugins/horizon-ops/skills/horizon-deploy/scripts/cloud-deploy.sh --destroy

# 4. Check status of recent Cloud Builds
./.agents/plugins/horizon-ops/skills/horizon-deploy/scripts/cloud-deploy.sh --status

# Help / Usage
./.agents/plugins/horizon-ops/skills/horizon-deploy/scripts/cloud-deploy.sh -h
```

---

## 5. Resource & Build Considerations

- **Docker Memory & CPU**: Image building during Terraform deployment (e.g. `gerrit-mcp-server-app`) can be resource-intensive. If builds fail with exit code 100 or memory timeouts, allocate at least 4 CPU cores and 8GB RAM to the Docker engine.
- **Persistent GCE Disks**: GCE Disks provisioned dynamically by Kubernetes workloads (e.g., PVCs) are not managed directly by Terraform. After running `--destroy`, inspect GCP Cloud Console (Compute Engine → Disks) and delete unattached disks manually if no longer needed.
