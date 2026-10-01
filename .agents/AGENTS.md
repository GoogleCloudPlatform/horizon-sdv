# Horizon SDV Platform Agent Guidelines

Guidance for coding agents working in this repository. It is vendor-neutral, highly opinionated, and applies to any coding agent, LLM-based tool, or human collaborator. Human contributors: start with `README.md` and `docs/`.

---

## What this repository is

A complete deployment and operations monorepo for the **Horizon Software-Defined Vehicle (SDV)** platform on **GKE Standard**.

Horizon SDV is an enterprise automotive software engineering platform that orchestrates automotive development, virtual verification, and deployment pipelines. It integrates core developer services:
- **GitOps Engine**: ArgoCD managing in-cluster applications declaratively.
- **Source Control & MCP**: Gerrit providing collaborative review, integrated with a Model Context Protocol (MCP) server app.
- **Identity & Authentication**: Keycloak acting as the single sign-on broker backed by a Google Identity Provider.
- **CI/CD Orchestration**: Jenkins for heavy build/test automation.
- **Telemetry & Monitoring**: Prometheus and Grafana for cluster and application observability.
- **Automotive Workloads**: Automated pipelines for building Android Automotive OS (**AAOS**) and running Android Virtual Devices (dedicated **ARM64 Cuttlefish emulators** on Compute Engine nested virtualization nodes).

---

## Layout

- `terraform/` — Terraform configurations:
  - `terraform/env/` — Environment definitions (e.g. `main.tf`, `variables.tf`, `locals.tf`, `terraform.tfvars`) defining project metadata, region, cluster settings, and passwords.
  - `terraform/modules/` — Reusable infrastructure components (base network, GKE Standard cluster, Certificate Manager, IAM, KMS, SSL policies, and GKE Application bootstrap).
- `gitops/` — Declarative GitOps charts and configurations:
  - `gitops/apps/` — Application charts (e.g. `horizon-dev-portal`, `mcp-gateway-registry`, `horizon-api`, `module-manager`).
  - `gitops/modules/` — System modules (common workflows, workloads pipelines).
  - `gitops/templates/` — GKE gateway routing configurations, ingress definitions, and key developer tools (Keycloak, ArgoCD, Jenkins, Gerrit).
- `workloads/` — Heavy automotive pipeline code and scripts:
  - `workloads/android/` — AAOS compilation workflows and ARM64 Cuttlefish VM templates.
  - `workloads/openbsw/` — Classic/Adaptive AUTOSAR and OpenBSW pipelines.
  - `workloads/cloud-workstations/` — Declarative developer desktop images, GKE cluster-admin tooling, and workstations configuration.
- `tools/` — Operational binaries and helper utilities:
  - `tools/scripts/deployment/` — Contains native deployment (`deploy.sh`) and the containerized wrapper (`container-deploy.sh`).
  - `tools/users_mgmt/` — Keycloak-to-platform Python user and group provisioning scripts.
- `docs/` — Comprehensive platform documentation (architecture, deployment guides, upgrades, and workload runbooks).
- `.agents/` — System-generated custom agent rules (`AGENTS.md`) and deployment/troubleshooting skills (`skills/horizon-sdv/SKILL.md`).

---

## Tooling

To interact with the deployment engine, you must use the following pre-installed workstation tools:
1.  **Docker (Engine & CLI)**: For executing isolated deployer container wrapper runs.
2.  **Google Cloud CLI (`gcloud`)**: For cluster auth, secrets management, and global resource diagnostics.
3.  **Kubectl (`kubectl`)**: For querying the running GKE Standard namespaces and services.
4.  **No other build/deploy systems**: All terraform and environment state alterations must use the approved container wrapper. Do not run native `terraform` on your host.

---

## Validate changes (run before proposing any change)

Before proposing any commit, structural patch, or manifest alteration, verify state compliance:

1.  **Dry-run Terraform Configuration**:
    Run a plan with the containerized wrapper to catch syntax errors or module dependency mismatches:
    ```bash
    ./tools/scripts/deployment/container-deploy.sh --plan
    ```
2.  **Verify GitOps YAML Structure**:
    If modifying Helm values or K8s manifests under `gitops/`, validate that charts render cleanly:
    ```bash
    helm lint gitops/apps/<app-name>/
    ```

---

## Conventions and invariants (do not break these)

- **Parameterization Invariant**:
  Every environment-specific parameter must be defined in `terraform/env/terraform.tfvars`. Never hardcode an IP address, region, subnet CIDR, GCS bucket, custom domain, or project ID into module source files (`terraform/modules/`).
- **Hermetic Containerized Execution**:
  All platform terraform actions (Apply, Plan, Destroy) must run via `./tools/scripts/deployment/container-deploy.sh`. This ensures tool chain version locks (Terraform 1.14.2, pinned gcloud SDK) are strictly maintained and prevents configuration drift.
- **Static A Records Setup**:
  When `sdv_dns_use_static_a_records = true` is active, the delegated subdomain zones are not created. The agent/operator must manually map the public Gateway external IP address (retrieved from `gcloud compute addresses list`) on the parent DNS registrar.
- **Zero Secrets in VCS**:
  Never commit explicit passwords, OAuth Client secrets, or private keys to Git. Pass credentials via `terraform.tfvars` or let Terraform fetch/create them inside Google Secret Manager (e.g. `argocd-admin-password-b64`).
- **GKE Standard Constraint**:
  Do not convert GKE Standard cluster resources to GKE Autopilot. Heavy workloads and Android emulators (Cuttlefish VMs) require nested virtualization, custom host and node pooling, and privileged mode execution (`hostPID`), which are unsupported in fully managed GKE models.
- **Documentation & Git Integrity**:
  Always preserve existing copyright headers, inline code comments, and shell docstrings. Ensure that any commits are authored using the approved local Git identity (`mikesson`).

---

## Deploy & Operate End-to-End

A high-level sequence of deploying and maintaining the Horizon SDV platform:

1.  **Remediate Org Policies**: Overwrite Service Account Key creation limits, nested virtualization blocks, and external IP constraints on the GCP target project.
2.  **Assemble Configuration**: Populate variables and custom domain parameters in `terraform/env/terraform.tfvars`.
3.  **Deploy base Infrastructure**: Run the containerized Terraform deployer to provision networks, GKE, gateways, and bootstrap services:
    ```bash
    ./tools/scripts/deployment/container-deploy.sh --apply
    ```
4.  **Verify GKE Sync**: Validate that external load balancer IPs are mapped, and that the Certificate Manager has completed domain ownership validation.
5.  **Troubleshoot and Incident Triage**:
    If any step of the deployment, authentication handshake, or cluster sync fails, immediately consult the local agent deployment skill under:
    👉 **[.agents/skills/horizon-sdv/SKILL.md](file:///usr/local/google/home/mikeannau/.gemini/antigravity/scratch/horizon-sdv/.agents/skills/horizon-sdv/SKILL.md)**
