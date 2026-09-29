# Phase 3: Deployment Execution

This guide covers the execution of Terraform workflows using the deployment scripts in `tools/scripts/deployment`.

---

## 1. Prerequisites Check

Before running the deployment:
- `terraform/env/terraform.tfvars` must exist and have all required fields filled in.
- `gcloud` must be authenticated to your target GCP Project.

---

## 2. Google Cloud Build Deployment (Recommended — Zero Local Dependencies)

The Cloud Build workflow executes containerized Terraform directly on Google Cloud Build infrastructure (`E2_HIGHCPU_8` workers) in your project.

> [!TIP]
> **No Local Machine Dependencies**: You do **not** need Docker, Brew, Terraform, or kubectl installed on your local laptop. The script packages the workspace configuration (including `terraform.tfvars`) and runs the deployment directly in Google Cloud, streaming real-time logs back to your terminal.

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

## 3. Containerized Deployment (Local Docker)

The containerized workflow packages all required CLI dependencies (`terraform`, `kubectl`, `gcloud`, etc.) inside a local container image (`horizon-sdv-deployer:latest`).

### Running Commands
From the repository root:

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

## 4. Linux Native Deployment

> [!IMPORTANT]
> The native script `deploy.sh` is intended solely for Debian/Ubuntu-based Linux environments.

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

## 4. Resource & Build Considerations

- **Docker Memory & CPU**: Image building during Terraform deployment (e.g. `gerrit-mcp-server-app`) can be resource-intensive. If builds fail with exit code 100 or memory timeouts, allocate at least 4 CPU cores and 8GB RAM to the Docker engine.
- **Persistent GCE Disks**: GCE Disks provisioned dynamically by Kubernetes workloads (e.g., PVCs) are not managed directly by Terraform. After running `--destroy`, inspect GCP Cloud Console (Compute Engine → Disks) and delete unattached disks manually if no longer needed.
