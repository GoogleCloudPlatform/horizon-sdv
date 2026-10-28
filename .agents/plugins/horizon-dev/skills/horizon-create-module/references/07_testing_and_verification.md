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

# Testing, Verification & Troubleshooting

This guide details testing and verification procedures for validating newly created Horizon SDV modules locally and against a live GKE cluster.

---

## 1. Local Helm Validation

Run linting and template rendering locally to catch syntax errors, missing variables, or indentation issues before pushing code:

```bash
# 1. Lint Parent and Child Helm Charts
helm lint gitops/modules/<module-name> \
  -f .agents/plugins/horizon-dev/skills/horizon-create-module/scripts/lint-values.yaml
helm lint gitops/modules/<module-name>/<app-name>
helm lint gitops/modules/<module-name>/argo-workflows

# 2. Dry-Run Template Rendering (Parent Chart)
helm template test-release gitops/modules/<module-name> \
  -f .agents/plugins/horizon-dev/skills/horizon-create-module/scripts/lint-values.yaml

# 3. Dry-Run Child Chart Template Rendering
helm template test-app gitops/modules/<module-name>/<app-name> \
  --set namespace="test-ns" \
  --set gcpProjectId="my-gcp-project" \
  --set rootPath="/test-app"
```

`lint-values.yaml` stands in for the values Module Manager injects into the parent chart. Keep its `config` keys in sync with `gitops/templates/module-manager.yaml`.

---

## 2. Live Cluster Verification

### 1. ModuleCatalog Discovery
Verify that Module Manager reconciles the new module into its internal catalog:

```bash
# Verify ModuleCatalog CR is valid
kubectl get modulecatalog cluster -n module-manager -o yaml

# Inspect Module Manager logs
kubectl logs -n module-manager -l app.kubernetes.io/name=module-manager -c manager --tail=50
```

### 2. Enable Module via Developer Portal / API
1. Navigate to Developer Portal (`https://<domain>/developer-portal/`).
2. Go to **Administration** -> **Modules**.
3. Locate `<module-name>` and click **Enable**.
4. Confirm that any hard dependencies are recursively enabled.

### 3. Verify Argo CD Application Synchronization
Check that the parent and child Argo CD applications sync cleanly:

```bash
kubectl get applications -n argocd -o custom-columns=NAME:.metadata.name,HEALTH:.status.health.status,SYNC:.status.sync.status,MESSAGE:.status.operationState.message | grep <module-name>
```

In a sub-environment the namespace is `<namespacePrefix>argocd`.

Expected output:
```text
mod-<module-name>                      Healthy   Synced   Successfully synced
mod-<module-name>-<module-name>-app    Healthy   Synced   Successfully synced
mod-<module-name>-argo-workflows       Healthy   Synced   Successfully synced
```

### 4. Verify KCC GCP Resources
Confirm that Config Connector provisioned the cloud infrastructure:

```bash
kubectl get pubsubtopics,storagebuckets -n <module-namespace>
```

Check the `Ready` condition:
```bash
kubectl get pubsubtopic <topic-name> -n <module-namespace> -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}'
# Expected output: True
```

---

## 3. Workflow Execution Verification

### Using Horizon CLI

```bash
# 1. Verify template is catalog-exposed
horizon catalog get | grep <module-name>

# 2. Submit test workflow (execution pipeline)
horizon workflow submit \
  --module <module-name> \
  --template <module-name>-execute \
  --params-json '{"executionEnv":"test","buildId":"build-001"}' \
  --output json

# 3. Save workflow name and follow logs
export WF_NAME="<workflow-name-from-output>"
horizon workflow logs "$WF_NAME"

# 4. Check workflow phase
horizon workflow show "$WF_NAME" --output json | grep '"phase"'
```

---

## 4. Common Troubleshooting Scenarios

### Scenario 1: KCC Resource Remains in `Reconciling` / `403 Forbidden`
- **Symptom**: `kubectl get pubsubtopic` shows `Ready=False` with `PermissionDenied`.
- **Cause**: Config Connector acts as `gke-config-connector-sa`, which has `roles/editor` (`terraform/env/main.tf`). Editor does not cover IAM policy changes.
- **Resolution**: See [02_kcc_resources.md](./02_kcc_resources.md) §4 Common Issues. Missing roles must be added in Terraform, not granted manually.

### Scenario 2: HTTPRoute Returns `404 Not Found` or `502 Bad Gateway`
- **Symptom**: Accessing `https://<domain>/<rootPath>` fails.
- **Cause 1**: The HealthCheckPolicy is failing because the application doesn't return `200 OK` on `/`.
- **Cause 2**: NetworkPolicy is blocking ingress from the GKE Gateway.
- **Resolution**: Check `kubectl get healthcheckpolicies -n <namespace>` and ensure port and path match the container's readiness probe.

### Scenario 3: WorkflowTemplate Not Visible in Developer Portal
- **Symptom**: Workflow does not show under the module's tab.
- **Cause**: The `WorkflowTemplate` is missing `horizon-sdv.io/expose: "true"`, its `horizon-sdv.io/module` label does not match the module's `name` in the ModuleCatalog, or the module is not enabled.
- **Resolution**: Fix both labels in `argo-workflows/templates/workflowtemplates.yaml`, re-sync, and confirm the module is enabled.
