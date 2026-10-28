<!-- Copyright 2026 Google LLC
Modifications Copyright (c) 2026 Accenture, All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

        http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License. -->

# Horizon SDV Development Rules

## Core Repository & Module Standards

1. **Mandatory Module Location (`gitops/modules/`)**:
   - Every module (core platform module, partner-contributed module, and modular workload) must reside in **`gitops/modules/<module-name>/`**.
   - For the correct location of all other files, follow `docs/contributing.md`.

2. **Strict Apache 2.0 Licensing for Contributions**:
   - All code, Helm charts, manifests, and documentation contributed under `gitops/modules/<module-name>/` **must be licensed under Apache 2.0**.
   - All partner modules reside directly in **`gitops/modules/<module-name>/`**.

3. **Partner Content and Non-Apache 2.0 Dependencies**:
   - Follow `docs/contributing.md` for where they go and how they are licensed.

4. **Git Branching & Pull Requests**:
   - Follow `docs/contributing.md` and `docs/branching_strategy.md`. Read them before creating a branch or pull request.
   - External contribution branches must be named **`contrib/feature/<title>`** or **`contrib/fix/<title>`** (internal: `feature/<ticketID>-<title>` or `fix/<ticketID>-<title>`); use `__` for multi-part changes.
   - All PRs must target **`main`**, and all commits must be signed.

5. **Module Naming & Identification**:
   - Module identifiers must be strictly lowercase kebab-case (`^[a-z0-9-]+$`).
   - Module directories, chart names, and child application names must reflect this kebab-case identifier.

6. **Pipeline Naming**:
   - Prefix WorkflowTemplate and Sensor names with the module name (`<module-name>-<purpose>`, `webhook-<module-name>-<purpose>`). The scaffold creates `<module-name>-init` and `<module-name>-execute` as a starting point; rename, add or remove them to fit the module.
