<!-- Copyright (c) 2026 Accenture, All Rights Reserved. -->

# Partner & External Contributions Guide

This guide outlines the rules, directory structures, licensing requirements, and git workflows for external contributors and partners contributing modular extensions to the Horizon SDV repository.

---

## 1. Core Contribution & Licensing Principles

Every contribution to Horizon SDV must adhere to these foundational principles:

1. **All Modules in `gitops/modules/` (No Top-Level `partner/` Folder)**:
   All deployable extensions—including partner-developed solutions, virtual device workloads, and CI/CT pipelines—must reside directly in **`gitops/modules/<module-name>/`**.
   > [!IMPORTANT]
   > There is **no top-level `partner/` directory** in the repository. Partner solutions are first-class modules within `gitops/modules/` and follow the standard Helm App-of-Apps and ModuleCatalog architecture.

2. **Strict Apache 2.0 Licensing for Module Contributions**:
   All partner-contributed code, Helm charts, manifests, and documentation under `gitops/modules/<module-name>/` **must be licensed under Apache 2.0**.

3. **Third-Party Non-Apache 2.0 Isolation in `third_party/`**:
   Any external dependency, pre-built binary, or library required by a partner solution that **cannot be licensed under Apache 2.0** must be isolated strictly within the **`third_party/<component_name>/`** directory.

4. **Self-Deployment Validation**:
   Before opening a Pull Request, you must verify your changes on a live, self-deployed Horizon SDV instance running in your own GCP project.

5. **Modularity & Self-Containment**:
   Contributions must be modular and easily removable without destabilizing core platform functions.

6. **Declarative Configuration over Custom Code**:
   Leverage Kubernetes manifests, Helm values, and KCC resources rather than custom scripting wherever possible.

---

## 2. Git Workflow & Collaboration Rules

### Branch Naming & Targets
- **Feature Branches**: Must use the naming prefix `contrib/*` (e.g. `contrib/partner-vendor-runner`, `contrib/cvd-accelerator`).
- **Target Branch**: All Pull Requests with functionality changes must be raised against the **`devel`** branch.
  > [!WARNING]
  > PRs raised against the `main` branch will be rejected and requested to rebase onto `devel`.

### Contributor License Agreement (CLA)
Contributions must be accompanied by a signed Google CLA.
- Sign or verify agreements at: [https://cla.developers.google.com/](https://cla.developers.google.com/)

---

## 3. Packaging Partner Solutions as Standard Modules

Partner extensions (such as virtual device runners, custom emulator controllers, vendor SoC testbenches, and CI/CD tools) are packaged using the standard module structure in `gitops/modules/<module-name>/`:

```text
gitops/modules/<partner-module-name>/
├── Chart.yaml                               # Parent Helm Chart definition (Apache 2.0)
├── README.md                                # Partner module documentation & architectural overview
├── values.yaml                              # Module Manager schema & configuration defaults
├── portal/
│   └── overview.html                        # Partner solution card & overview for Developer Portal
├── templates/
│   ├── module-overview-http.yaml            # Portal overview Nginx service
│   ├── application-<workload-app>.yaml      # ArgoCD Application for partner workloads / KCC
│   └── application-argo-workflows.yaml      # ArgoCD Application for partner Argo Workflows
├── <workload-app>/                          # Workload Helm subchart (Pods, Services, KCC resources)
│   ├── Chart.yaml
│   ├── values.yaml
│   └── templates/
│       ├── deployment.yaml                  # Partner controller / runner deployment
│       ├── service.yaml
│       ├── gateway-route.yaml               # Gateway API HTTPRoute (if UI / API exposed)
│       ├── network-policies.yaml
│       └── kcc-<resource>.yaml              # Declarative GCP resources (Pub/Sub, GCS, IAM)
└── argo-workflows/                          # Workflows Helm subchart (Pipelines & Sensors)
    ├── Chart.yaml
    ├── values.yaml
    └── templates/
        ├── workflowtemplates.yaml           # Argo WorkflowTemplates (init + execute)
        └── sensors.yaml                     # Argo Events Webhook Sensors (init + execute)
```

### Organizing Partner Capabilities within the Module

| Capability | Module Location | Description |
| :--- | :--- | :--- |
| **Virtual Device / Emulator Controllers** | `<workload-app>/templates/deployment.yaml` | Pods managing vendor QEMU instances, Cuttlefish launchers, or device orchestration. |
| **Partner CI/CD Workflows** | `argo-workflows/templates/workflowtemplates.yaml` | Reusable Argo WorkflowTemplates for partner-specific build, test, and CTS pipelines. |
| **Partner Web UI / API** | `<workload-app>/templates/gateway-route.yaml` | Exposes partner dashboards or APIs via GKE Gateway API HTTPRoutes. |
| **Developer Portal Documentation** | `portal/overview.html` | Custom HTML overview page describing features, endpoints, and workflows in the Portal. |
| **Declarative GCP Infrastructure** | `<workload-app>/templates/kcc-*.yaml` | Config Connector manifests for dedicated Pub/Sub topics, GCS buckets, or BigQuery datasets. |
| **Non-Apache Dependencies** | `third_party/<component_name>/` | Proprietary binaries, vendor trial SDKs, or non-Apache licensed tooling. |

---

## 4. Documentation & PR Checklist

When opening your Pull Request:

- [ ] I have signed the Google Contributor License Agreement (CLA).
- [ ] My PR branch is named `contrib/<feature-name>` and targets branch **`devel`**.
- [ ] My module is placed directly in **`gitops/modules/<module-name>/`** (not in deprecated `workloads/` or non-existent `partner/`).
- [ ] All code and Helm manifests in `gitops/modules/<module-name>/` are licensed under **Apache 2.0**.
- [ ] Any non-Apache 2.0 dependencies are placed strictly in **`third_party/<component_name>/`**.
- [ ] I have tested and verified the module on my self-deployed Horizon SDV instance.
- [ ] All Helm charts pass `helm lint` and `helm template`.
- [ ] The module is registered in `gitops/apps/module-manager/templates/module-catalog.yaml`.
- [ ] A thorough `README.md` and inline comments explain usage, architecture, and configuration.
- [ ] PR description includes test execution logs or screenshots.
