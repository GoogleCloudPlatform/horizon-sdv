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

# Horizon Operations & Deployment Rules

## Strict Communication & Decision Rules

1. **Always Ask Clarifying Questions**:
   - Never make assumptions regarding project configuration, domain names, credentials, infrastructure sizing, environment variables, target environments, or architectural decisions.
   - If any parameter, option, or instruction is missing, ambiguous, or underspecified, explicitly ask the user for clarification before proceeding with any action or writing configuration files.

2. **No Silent Defaults**:
   - Before writing `terraform/env/terraform.tfvars`, present every value in one review table, marking each as user-supplied, derived (e.g. project number, compute service account), sample default (from `terraform.tfvars.sample`), or generated (passwords shown masked).
   - The user approves or edits the table once. No value may be applied that did not appear in that table.

3. **Validate Inputs**:
   - Confirm requirements and collected parameters with the user prior to executing deployment, modification, or teardown operations.

4. **Deployment Parity with `deploy.sh`**:
   - By default, deploy only via `tools/scripts/deployment/deploy.sh`
     (native) or `container-deploy.sh` (run by the user). The resulting
     deployment must match what those scripts produce from the same
     `terraform.tfvars`.
   - Do not modify Terraform, gitops, Helm values, scripts or other
     tracked files outside `.agents/`, and do not create or change GCP
     resources beyond `docs/deployment_guide.md`.
   - The Cloud Build runner is an opt-in variant. Its only permitted
     differences are: runs on Cloud Build, runs as the Cloud Build
     service account, and the setup it needs (cloudbuild/orgpolicy
     APIs, `roles/owner` grants, source staging in the state bucket).
     State these to the user before using it.
   - Exception — failures and blockers: the agent may take the minimum
     action needed to unblock a failed step. It must state the failure,
     the proposed fix and whether it diverges from the standard flow,
     and get confirmation first. Destructive fixes (deleting resources,
     `terraform apply -replace`, state edits) always need explicit
     approval.
   - Leave no changes in tracked files outside `.agents/`. Report any
     workaround so it can be fixed upstream.
