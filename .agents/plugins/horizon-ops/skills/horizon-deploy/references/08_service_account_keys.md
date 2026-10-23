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

# Service Account Keys Architecture & Policy Guide

This document provides architectural context, security considerations, and enterprise policy management for service account keys in **Horizon SDV**.

---

## 1. Architectural Purpose

Horizon SDV provisions a Google Cloud Service Account JSON key (`gce-creds`) managed by `module.sdv_sa_key_secret_gce_creds` and stored in **GCP Secret Manager**.

### Why is this key needed?
1. **Jenkins Google Compute Engine (GCE) Cloud Plugin**:
   - The platform uses Jenkins as its primary CI/CD execution engine for Android and automotive software stacks.
   - Heavy workloads (such as AOSP compilation and Cuttlefish Android Virtual Device emulator testing) require high-CPU, nested-virtualization Compute Engine VMs provisioned dynamically on demand.
   - The Jenkins GCE Cloud Plugin provisions these ephemeral worker VMs and terminates them upon job completion.
   - The plugin currently requires a service account JSON credential key (`gce-creds-json`) to authenticate to the Google Compute Engine API from within the Jenkins controller pod.

2. **Secret Lifecycle & Secret Manager Integration**:
   - The raw JSON key is never exposed on the developer's laptop or committed to version control.
   - Terraform uploads the key directly to GCP Secret Manager as secret `gce-creds`.
   - GKE's External Secrets Operator synchronizes `gce-creds` into a Kubernetes secret `jenkins-gce-creds-secret` inside the `jenkins` namespace.
   - Access to Secret Manager is restricted via Workload Identity to `serviceAccount:jenkins-sa` in namespace `jenkins`.

---

## 2. Organization Policy Constraint (`constraints/iam.disableServiceAccountKeyCreation`)

In enterprise GCP environments, Google Cloud enforces an organization policy constraint that blocks the creation of service account keys by default.

### Symptom
When deploying via Terraform or `deploy.sh` / `cloud-deploy.sh`, Terraform fails with:
```text
Error: Error creating service account key: googleapi: Error 400: Key creation is not allowed on this service account.
PreconditionFailure violations:
- type: "constraints/iam.disableServiceAccountKeyCreation"
```

### Remediation
To allow Horizon SDV to generate and manage the Jenkins GCE dynamic VM key on a dedicated project:

1. **Enable Organization Policy API** (if not already enabled):
   ```bash
   gcloud services enable orgpolicy.googleapis.com --project=<GCP_PROJECT_ID>
   ```

2. **Disable Constraint Enforcement on the Target Project**:
   ```bash
   gcloud resource-manager org-policies disable-enforce constraints/iam.disableServiceAccountKeyCreation \
     --project=<GCP_PROJECT_ID>
   ```

3. **Verify Policy Status**:
   ```bash
   gcloud org-policies describe constraints/iam.disableServiceAccountKeyCreation \
     --project=<GCP_PROJECT_ID> --effective
   ```

---

## 3. Key Rotation & Security Best Practices

1. **Least Privilege**:
   The GCE service account is scoped strictly to instance management, logging, monitoring, and artifact access.

2. **Key Rotation (manual, approval only)**:
   Key rotation is not supported by the standard deployment scripts. `force_update_secret_ids` does not cover `gce-creds`; it only applies to the `s5`…`s17` password secrets.
   To rotate the key, replace the key resource with Terraform from `terraform/env`:
   ```bash
   terraform apply -replace=module.base.module.sdv_sa_key_secret_gce_creds.google_service_account_key.sa_key
   ```
   > [!WARNING]
   > This is destructive: the old key is deleted and Jenkins GCE workers use the new key only after the `gce-creds` secret has synced. Run it only with explicit user approval (see `rules/AGENTS.md`, rule 4).

---

## 4. Modernization & Keyless Roadmap

To eliminate long-lived service account keys entirely in future platform revisions:
* **GKE Workload Identity Federation for GCE Plugin**: Migrate the Jenkins GCE agent provider to use GCP Workload Identity federation or ambient GKE metadata credentials directly once supported by upstream Jenkins plugins.
* **GKE Batch / Cloud Run Jobs**: For containerized build steps, execute directly on specialized GKE node pools (e.g. `sdv-abfs-build-node-pool`) using native GKE Workload Identity without requiring Compute Engine VM API calls.
