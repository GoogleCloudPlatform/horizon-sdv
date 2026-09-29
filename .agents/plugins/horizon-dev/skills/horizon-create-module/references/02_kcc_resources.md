<!-- Copyright (c) 2026 Accenture, All Rights Reserved. -->

# Kubernetes Config Connector (KCC) in Horizon Modules

This guide details how to declare, configure, and manage Google Cloud Platform (GCP) resources directly within Horizon SDV modules using **Kubernetes Config Connector (KCC)**.

---

## 1. What is Config Connector in Horizon SDV?

**Kubernetes Config Connector (KCC)** allows developers to manage GCP infrastructure declaratively using Kubernetes Custom Resource Definitions (CRDs). In Horizon SDV:
- Platform-level infrastructure (VPC, GKE cluster, Cloud DNS, core IAM) is provisioned via **Terraform**.
- Module-level infrastructure (Pub/Sub topics, Cloud Storage buckets, BigQuery datasets, Cloud KMS keys, IAM bindings) is provisioned via **KCC manifests** inside module Helm charts.

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
  name: {{ .Values.parentModuleName | replace "-" "_" }}_dataset
  namespace: {{ .Values.namespace }}
  annotations:
    argocd.argoproj.io/sync-wave: "1"
    cnrm.cloud.google.com/project-id: "{{ .Values.gcpProjectId }}"
  labels:
    app.kubernetes.io/name: {{ .Values.parentModuleName }}
spec:
  location: {{ .Values.config.region | default "europe-west1" }}
  description: "Analytics dataset for {{ .Values.parentModuleName }}"
  defaultTableExpirationMs: "2592000000" # 30 days
```

### Example 4: IAM Policy Member (Workload Identity Binding)

Allowing a Kubernetes Service Account to access GCP services:

```yaml
apiVersion: iam.cnrm.cloud.google.com/v1beta1
kind: IAMPolicyMember
metadata:
  name: {{ .Values.parentModuleName }}-pubsub-publisher
  namespace: {{ .Values.namespace }}
  annotations:
    argocd.argoproj.io/sync-wave: "1"
    cnrm.cloud.google.com/project-id: "{{ .Values.gcpProjectId }}"
spec:
  member: "serviceAccount:{{ .Values.gcpProjectId }}.svc.id.goog[{{ .Values.namespace }}/{{ .Values.parentModuleName }}-sa]"
  role: "roles/pubsub.publisher"
  resourceRef:
    apiVersion: pubsub.cnrm.cloud.google.com/v1beta1
    kind: PubSubTopic
    name: {{ .Values.parentModuleName }}-events
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
   - Fix: Ensure the GKE / KCC service account has appropriate GCP project roles (e.g. `roles/pubsub.admin`, `roles/storage.admin`).
2. **`Missing Project ID`**:
   - Cause: The `cnrm.cloud.google.com/project-id` annotation is missing or evaluated to an empty string.
   - Fix: Ensure `gcpProjectId: {{ .Values.config.projectID | quote }}` is passed in the parent child `Application` Helm values.
