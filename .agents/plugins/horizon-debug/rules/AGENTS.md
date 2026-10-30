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

# Horizon cluster debug rules

1. **No changes without approval**: Do not modify the cluster, GCP, or any files without explicit approval. Read-only commands (`kubectl get`, `describe`, `logs`, and `gcloud` reads) may run without asking.
2. **No GCP changes**: Do not change IAM, enable APIs, register a fleet, or change organization policy or billing. If access is missing, report it and stop. Approval does not override this.
3. **Confirm the target**: Show the project ID, cluster URL and gcloud account, and get a yes before generating kubectl credentials. Do not use `gcloud config get project` silently.
4. **Report, don't fix**: A failed sanity check or health check is reported. Do not repair it unless the user then asks, and that repair still needs its own approval.
5. **No unapproved repo edits**: Do not modify Terraform, gitops, Helm values, or other tracked files unless the user explicitly asks for that edit. Show the change and get a separate approval before writing it. The result must still match a normal Horizon deployment.
6. **Authentication failures**: If a connection fails because gcloud credentials are missing, expired, or need reauthentication, stop. Tell the user to run this in their own terminal, then say when it has finished. Do not run it for them.
   ```bash
   gcloud auth login <ACCOUNT>
   ```
   Use the account the failure names. If it names none, use `gcloud auth login`.
