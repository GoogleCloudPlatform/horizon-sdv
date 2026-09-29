# Horizon SDV Development Rules

## Core Repository & Module Standards

1. **Mandatory Module Location (`gitops/modules/`)**:
   - Every module (core platform module, partner-contributed module, and modular workload) must reside in **`gitops/modules/<module-name>/`**.
   - The top-level `workloads/` folder is **deprecated** and must not receive new module additions.

2. **Strict Apache 2.0 Licensing for Contributions**:
   - All code, Helm charts, manifests, and documentation contributed under `gitops/modules/<module-name>/` **must be licensed under Apache 2.0**.
   - All partner modules reside directly in **`gitops/modules/<module-name>/`** (there is no top-level `partner/` directory).

3. **Strict Isolation of Non-Apache 2.0 Dependencies (`third_party/`)**:
   - Any external dependency, library, binary, or component that cannot be licensed under Apache 2.0 must be placed strictly in **`third_party/<component_name>/`**.

4. **Git Branching & Pull Request Targets**:
   - Feature and contribution branches must follow the naming pattern **`contrib/<feature-name>`**.
   - All PRs contributing code or modules must target the **`devel`** branch (PRs targeting `main` will be rejected).

5. **Module Naming & Identification**:
   - Module identifiers must be strictly lowercase kebab-case (`^[a-z0-9-]+$`).
   - Module directories, chart names, and child application names must reflect this kebab-case identifier.

6. **Dual Pipeline Structure**:
   - Modules providing Argo Workflows must provide both an initialization pipeline (`<module-name>-init`) and an execution pipeline (`<module-name>-execute`).
