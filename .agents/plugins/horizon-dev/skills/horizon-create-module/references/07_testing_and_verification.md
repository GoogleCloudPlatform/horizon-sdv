<!-- Copyright (c) 2026 Accenture, All Rights Reserved. -->

# Testing, Verification & Troubleshooting

This guide details testing and verification procedures for validating newly created Horizon SDV modules locally and against a live GKE cluster.

---

## 1. Local Helm Validation

Run linting and template rendering locally to catch syntax errors, missing variables, or indentation issues before pushing code:

```bash
# 1. Lint Parent and Child Helm Charts
helm lint gitops/modules/<module-name>
helm lint gitops/modules/<module-name>/<app-name>
helm lint gitops/modules/<module-name>/argo-workflows

# 2. Dry-Run Template Rendering (Parent Chart)
helm template test-release gitops/modules/<module-name> \
  --set moduleName="my-test-mod" \
  --set repo.url="https://github.com/my-org/horizon-sdv.git" \
  --set repo.revision="HEAD"

# 3. Dry-Run Child Chart Template Rendering
helm template test-app gitops/modules/<module-name>/<app-name> \
  --set namespace="test-ns" \
  --set gcpProjectId="my-gcp-project" \
  --set rootPath="/test-app"
```

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

Expected output:
```text
mod-<module-name>              Healthy   Synced   Successfully synced
mod-<module-name>-app          Healthy   Synced   Successfully synced
mod-<module-name>-argo-workflows Healthy Synced   Successfully synced
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
- **Cause**: Config Connector service account does not have IAM roles on the GCP project.
- **Resolution**: Check IAM policy bindings for the GKE compute SA or KCC controller identity.

### Scenario 2: HTTPRoute Returns `404 Not Found` or `502 Bad Gateway`
- **Symptom**: Accessing `https://<domain>/<rootPath>` fails.
- **Cause 1**: The HealthCheckPolicy is failing because the application doesn't return `200 OK` on `/`.
- **Cause 2**: NetworkPolicy is blocking ingress from the GKE Gateway.
- **Resolution**: Check `kubectl get healthcheckpolicies -n <namespace>` and ensure port and path match the container's readiness probe.

### Scenario 3: WorkflowTemplate Not Visible in Developer Portal
- **Symptom**: Workflow does not show under the module's tab.
- **Cause**: Missing `horizon-sdv.io/expose: "true"` label on the `WorkflowTemplate` metadata.
- **Resolution**: Add the label to `argo-workflows/templates/workflowtemplates.yaml` and re-sync.
