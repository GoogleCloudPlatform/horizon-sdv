<!-- Copyright (c) 2026 Accenture, All Rights Reserved.
Modifications Copyright 2026 Google LLC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

        http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License. -->

# Partner & External Contributions Guide

This guide outlines the rules, directory structures, licensing requirements, and git workflows for external contributors and partners contributing modular extensions to the Horizon SDV repository.

---

## 1. Core Contribution & Licensing Principles

Every contribution to Horizon SDV must adhere to these foundational principles:

1. **All Modules in `gitops/modules/`**:
   All modules, including partner modules, must reside directly in **`gitops/modules/<module-name>/`**. For the correct location of all other files, follow `docs/contributing.md`.

2. **Strict Apache 2.0 Licensing for Module Contributions**:
   All partner-contributed code, Helm charts, manifests, and documentation under `gitops/modules/<module-name>/` **must be licensed under Apache 2.0**.

3. **Partner Content and Non-Apache 2.0 Dependencies**:
   Follow `docs/contributing.md` for where they go and how they are licensed.

4. **Self-Deployment Validation**:
   Before opening a Pull Request, you must verify your changes on a live, self-deployed Horizon SDV instance running in your own GCP project.

5. **Modularity & Self-Containment**:
   Contributions must be modular and easily removable without destabilizing core platform functions.

6. **Declarative Configuration over Custom Code**:
   Leverage Kubernetes manifests, Helm values, and KCC resources rather than custom scripting wherever possible.

---

## 2. Git Workflow & Collaboration Rules

### Branch Naming & Targets
Follow `docs/contributing.md` and `docs/branching_strategy.md`. In summary:
- **Branch Names**: Must use `contrib/feature/<title>` or `contrib/fix/<title>` (e.g. `contrib/feature/PartnerVendorRunner`, `contrib/fix/CvdAcceleratorTimeout`). For multi-part changes, separate the part with `__` (e.g. `contrib/feature/PartnerVendorRunner__Docs`); a second `/` is not allowed.
- **Target Branch**: All Pull Requests must be raised against **`main`**, from a branch created from an up-to-date `main`. All commits must be signed.

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
│       └── kcc-<resource>.yaml              # Declarative GCP resources (Pub/Sub, GCS, BigQuery)
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
---

## 4. Documentation & PR Checklist

When opening your Pull Request:

- [ ] I have signed the Google Contributor License Agreement (CLA).
- [ ] My branch and pull request follow `docs/contributing.md` and `docs/branching_strategy.md`.
- [ ] My PR branch is named `contrib/feature/<title>` or `contrib/fix/<title>` and targets branch **`main`**.
- [ ] All my commits are signed.
- [ ] My module is placed directly in **`gitops/modules/<module-name>/`**, and all other files follow the locations in `docs/contributing.md`.
- [ ] All code and Helm manifests in `gitops/modules/<module-name>/` are licensed under **Apache 2.0**.
- [ ] Partner content and any non-Apache 2.0 dependencies follow `docs/contributing.md`.
- [ ] I have tested and verified the module on my self-deployed Horizon SDV instance.
- [ ] All Helm charts pass `helm lint` and `helm template`.
- [ ] The module is registered in `gitops/apps/module-manager/templates/module-catalog.yaml`.
- [ ] A thorough `README.md` and inline comments explain usage, architecture, and configuration.
- [ ] PR description includes test execution logs or screenshots.
