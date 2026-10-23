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

# Kubernetes Config Connector (KCC) in Horizon Modules

This guide details how to declare, configure, and manage Google Cloud Platform (GCP) resources directly within Horizon SDV modules using **Kubernetes Config Connector (KCC)**.

---

## 1. What is Config Connector in Horizon SDV?

**Kubernetes Config Connector (KCC)** allows developers to manage GCP infrastructure declaratively using Kubernetes Custom Resource Definitions (CRDs). In Horizon SDV:
- Platform-level infrastructure (VPC, GKE cluster, Cloud DNS, core IAM) is provisioned via **Terraform**.
- Module-level infrastructure (Pub/Sub topics, Cloud Storage buckets, BigQuery datasets, Cloud KMS keys) is provisioned via **KCC manifests** inside module Helm charts.

This ensures modules remain self-contained: enabling a module in the Developer Portal automatically provisions its required GCP cloud resources via Argo CD without running Terraform again.

---

## 2. Essential KCC Annotations & Rules

Every KCC resource declared in a module must adhere to these rules:

### 1. Project ID Annotation
KCC requires the target GCP project ID. In Horizon modules, this is injected from Helm values:

```yaml
annotations:
  cnrm.cloud.google.com/project-id: "{{ .Values.gcpProjectId }}"
```

### 2. Argo CD Sync Waves
KCC resources must be provisioned before the applications or workloads that consume them. Assign them to **Sync Wave 1** (workload pods typically run on Sync Wave 2 or higher):

```yaml
annotations:
  argocd.argoproj.io/sync-wave: "1"
```

### 3. Deletion Policy
Control whether the underlying GCP resource is deleted or retained when the module or Argo CD application is deleted:
- `cnrm.cloud.google.com/deletion-policy: "delete"` (default): Deletes the GCP resource when the Kubernetes CR is removed.
- `cnrm.cloud.google.com/deletion-policy: "abandon"`: Retains the GCP resource in Google Cloud even if the Kubernetes CR is deleted.

### 4. Cluster-mode Config Connector
Horizon runs Config Connector in cluster mode as `gke-config-connector-sa` (`gitops/templates/config-connector.yaml`, roles in `terraform/env/main.tf`). Resources in any namespace are reconciled. Do not create a `ConfigConnectorContext`; the operator rejects it in cluster mode.

---

## 3. Common KCC Resource Examples

### Example 1: Google Cloud Pub/Sub Topic

Manifest: `gitops/modules/<module-name>/<app-name>/templates/kcc-pubsub.yaml`

```yaml
apiVersion: pubsub.cnrm.cloud.google.com/v1beta1
kind: PubSubTopic
metadata:
  name: {{ .Values.parentModuleName }}-events
  namespace: {{ .Values.namespace }}
  annotations:
    argocd.argoproj.io/sync-wave: "1"
    cnrm.cloud.google.com/project-id: "{{ .Values.gcpProjectId }}"
  labels:
    app.kubernetes.io/name: {{ .Values.parentModuleName }}
spec:
  messageRetentionDuration: 86400s  # 1 day
```

### Example 2: Google Cloud Storage (GCS) Bucket

Manifest: `gitops/modules/<module-name>/<app-name>/templates/kcc-storage-bucket.yaml`

```yaml
apiVersion: storage.cnrm.cloud.google.com/v1beta1
kind: StorageBucket
metadata:
  name: {{ .Values.gcpProjectId }}-{{ .Values.parentModuleName }}-artifacts
  namespace: {{ .Values.namespace }}
  annotations:
    argocd.argoproj.io/sync-wave: "1"
    cnrm.cloud.google.com/project-id: "{{ .Values.gcpProjectId }}"
    cnrm.cloud.google.com/deletion-policy: "abandon"
  labels:
    app.kubernetes.io/name: {{ .Values.parentModuleName }}
spec:
  location: {{ .Values.config.region | default "europe-west1" }}
  uniformBucketLevelAccess: true
  versioning:
    enabled: true
  lifecycleRule:
    - action:
        type: Delete
      condition:
        age: 30  # Auto-cleanup artifacts after 30 days
```

### Example 3: BigQuery Dataset

Manifest: `gitops/modules/<module-name>/<app-name>/templates/kcc-bigquery.yaml`

```yaml
apiVersion: bigquery.cnrm.cloud.google.com/v1beta1
kind: BigQueryDataset
metadata:
  name: {{ .Values.parentModuleName }}-dataset
  namespace: {{ .Values.namespace }}
  annotations:
    argocd.argoproj.io/sync-wave: "1"
    cnrm.cloud.google.com/project-id: "{{ .Values.gcpProjectId }}"
  labels:
    app.kubernetes.io/name: {{ .Values.parentModuleName }}
spec:
  resourceID: {{ .Values.parentModuleName | replace "-" "_" }}_dataset  # BigQuery IDs allow `_`; Kubernetes names do not
  location: {{ .Values.config.region | default "europe-west1" }}
  description: "Analytics dataset for {{ .Values.parentModuleName }}"
  defaultTableExpirationMs: "2592000000" # 30 days
```

---

## 4. Verification & Troubleshooting

### Check KCC Resource Status
Use `kubectl` to inspect the reconciliation state of KCC objects:

```bash
# List resources in module namespace
kubectl get pubsubtopics,storagebuckets,bigquerydatasets -n <module-namespace>

# Describe resource for status conditions
kubectl describe pubsubtopic <topic-name> -n <module-namespace>
```

### Expected Status Conditions
Healthy KCC resources will display:
```yaml
Status:
  Conditions:
    Last Transition Time:  2026-09-03T10:00:00Z
    Message:               The resource is up to date
    Reason:                UpToDate
    Status:                True
    Type:                  Ready
```

### Common Issues
1. **`Permission Denied` / `403 Forbidden`**:
   - Cause: Config Connector's service account lacks IAM permissions in the GCP project to create the specified resource.
   - Fix: Config Connector acts as `gke-config-connector-sa` with `roles/editor` (`terraform/env/main.tf`). Editor covers creating most resources but not IAM policy changes. Missing roles must be added in Terraform, not granted manually.
2. **`Missing Project ID`**:
   - Cause: The `cnrm.cloud.google.com/project-id` annotation is missing or evaluated to an empty string.
   - Fix: Ensure `gcpProjectId: {{ .Values.config.projectID | quote }}` is passed in the parent child `Application` Helm values.
3. **`SERVICE_DISABLED` / API not enabled**:
   - Cause: Terraform enables only the APIs listed in `terraform/env/main.tf`; Pub/Sub and BigQuery are not in that list.
   - Fix: Check with `gcloud services list --enabled --project=<project-id>`. Document any required API in the module README as a prerequisite.
