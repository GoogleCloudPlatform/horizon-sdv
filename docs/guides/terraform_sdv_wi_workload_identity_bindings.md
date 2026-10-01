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

# Terraform `sdv-wi`: Workload Identity bindings on Google service accounts

This guide explains how the **`sdv-wi`** module (`terraform/modules/sdv-wi`) wires **GKE Workload Identity (WI)** and why **IAM bindings must be attached to each Google service account (GSA)**, not to the **project**.

For Argo Workflows–specific KSA/GSA naming and pod behavior, see the [Argo Workflows Service Accounts](https://argo-workflows.readthedocs.io/en/latest/service-accounts/) documentation.

---

## Symptom

Workloads that call Google APIs using WI (for example **External Secrets Operator** reading **Secret Manager** via a `SecretStore` with `gcpsm` + `workloadIdentity`) can fail during token exchange with:

```text
Permission 'iam.serviceAccounts.getAccessToken' denied on resource (or it may not exist).
```

Example: syncing a secret such as `github-app-id-b64` from GSM into the cluster.

---

## Root cause

For GKE Workload Identity, the Kubernetes service account (KSA) must be allowed to **impersonate** the target GSA. Google’s documented binding is:

- **Role:** `roles/iam.workloadIdentityUser`
- **Resource:** the **Google service account** (not the project)
- **Member:** `serviceAccount:PROJECT_ID.svc.id.goog[NAMESPACE/KUBERNETES_SA_NAME]`

Granting `roles/iam.workloadIdentityUser` at **project** scope to that member does **not** replace the per–service-account binding required for impersonation. Clients then fail when obtaining an access token for the GSA.

---

## What the `sdv-wi` module does

1. **`google_service_account.sdv_wi_sa`** — Creates one GSA per entry in `var.wi_service_accounts` (for example `gke-argo-workflows-sa`, `gke-argocd-sa`).

2. **`google_project_iam_member.sdv_wi_sa_iam_2`** — Grants **project-level roles** to each GSA **email** (Secret Manager, storage, etc., as defined in `roles` for each SA). This is unchanged.

3. **`google_service_account_iam_member.sdv_wi_sa_workload_identity_user`** — For each `(GSA, KSA)` pair in `gke_sas`, grants **`roles/iam.workloadIdentityUser`** on **that GSA** to the WI member  
   `serviceAccount:PROJECT_ID.svc.id.goog[gke_ns/gke_sa]`.

The third block is the **Workload Identity** link between cluster identities and GSAs.

---

## Terraform apply notes

After switching from project-level WI bindings to **GSA-level** `google_service_account_iam_member`:

- **Plan** will show **destroy** of old `google_project_iam_member` resources that previously attempted WI at project scope (if still in state), and **create** of `google_service_account_iam_member` resources.
- **Short risk:** brief window while bindings are replaced; re-run apply if needed.
- No change is required in **gitops** KSA annotations (`iam.gke.io/gcp-service-account`) if they already point at the correct GSA email.

---

## References

- [Authenticate to Google Cloud APIs from GKE workloads](https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity) (Google Cloud)
- Module: `terraform/modules/sdv-wi/main.tf`
