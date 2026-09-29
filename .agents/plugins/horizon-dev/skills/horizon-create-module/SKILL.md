---
name: horizon-create-module
description: >-
  Interactive guide and procedural workflow to scaffold, implement, configure, and contribute new modules to Horizon SDV.
  Enforces repository rules: all modules must reside in gitops/modules (workloads directory is deprecated),
  all partner contributions must be Apache 2.0 licensed, and non-Apache 2.0 dependencies must be placed in third_party.
  Covers the Helm App-of-Apps pattern, declarative GCP infrastructure via Kubernetes Config Connector (KCC),
  Argo Workflows and Argo Events Sensors, GKE-based applications with Gateway API HTTPRoutes,
  and registration in the Module Manager catalog (ModuleCatalog CR) with Developer Portal integration.
---

# Horizon SDV Module Creation Skill

This skill guides developers and AI agents through designing, scaffolding, implementing, registering, testing, and contributing new modular extensions to the **Horizon SDV** platform.

---

## Quick Start Questionnaire

> [!IMPORTANT]
> **Condensed Setup Questionnaire**: When starting a module creation request, gather these 4 basic inputs:
> 1. **Module Name & Title**: Target identifier in kebab-case and display title, using repository workload/workflow naming conventions:
>    - `aaos-builder` (*"AAOS Image Builder"*)
>    - `cvd-launcher` (*"Cuttlefish Virtual Device Launcher"*)
>    - `cts-testbench` (*"Android CTS Testbench"*)
>    *(All modules are placed under `gitops/modules/<name>/`)*.
> 2. **Cloud Infrastructure (KCC)**: GCP resources needed (e.g. Pub/Sub topic, Cloud Storage bucket, BigQuery dataset, IAM member, or none).
> 3. **Application & Ingress**: Does it expose a web UI / API on the GKE Gateway? (e.g. root path `/<module-name>`).
> 4. **Dependencies**: Any required modules (`hardDependencies`) or optional integrations (`softDependencies`)? *(e.g. `workloads-common`, `storage-gcs`)*.
>
> *(By default, dual Argo Workflows pipelines are generated: `<name>-init` for setup/seeding and `<name>-execute` for workload execution)*.

---

## Core Repository & Licensing Rules

1. **All Modules in `gitops/modules/`**:
   - Every module (core platform modules, partner-contributed modules, and modular workloads) must reside in **`gitops/modules/<module-name>/`**.
   - There is no top-level `partner/` directory; partners contribute extensions as standard modules under `gitops/modules/`.
   - The top-level `workloads/` folder is **deprecated** going forward in favor of `gitops/modules/`.
2. **Apache 2.0 License Requirement for Contributions**:
   - All module code, Helm charts, manifests, and documentation in `gitops/modules/<module-name>/` **must be licensed under Apache 2.0**.
3. **Non-Apache 2.0 Dependencies in `third_party/`**:
   - Any third-party dependency, library, or component that **cannot be licensed under Apache 2.0** must be isolated strictly within the **`third_party/<component_name>/`** directory.

---

## Real Module Examples from `gitops/modules/`

Horizon SDV includes reference module implementations in `gitops/modules/`:

| Module Directory | Primary Role & Capabilities | Pipeline Naming (`init` / `exec`) | Real Reference Path |
| :--- | :--- | :--- | :--- |
| **`sample-module`** | Full App-of-Apps reference with Pub/Sub KCC, web UI, soft feature toggles | `sample-init` / `sample-smoke-test` | [`gitops/modules/sample-module`](../../../../gitops/modules/sample-module) |
| **`sample-hard-module`** | Hard dependency target; auto-enabled and blocks parent deletion | `sample-hard-init` / `sample-hard-smoke-test` | [`gitops/modules/sample-hard-module`](../../../../gitops/modules/sample-hard-module) |
| **`sample-soft-module`** | Soft dependency target; dynamically toggled feature flags | `sample-soft-init` / `sample-soft-smoke-test` | [`gitops/modules/sample-soft-module`](../../../../gitops/modules/sample-soft-module) |
| **`sample-data-module`** | Declarative GCP Storage bucket CR (`GCSBucket` reconciled by storage operator) | Bucket lifecycle management | [`gitops/modules/sample-data-module`](../../../../gitops/modules/sample-data-module) |
| **`storage-gcs-module`** | Go operator + REST API for dynamic GCS bucket & object lifecycle | `storage-gcs-upload` (`upload`, `resumable-upload`) | [`gitops/modules/storage-gcs-module`](../../../../gitops/modules/storage-gcs-module) |
| **`workloads-common`** | Shared cluster pipelines (Docker builds, GitHub App credentials) | `prepare-pipeline-git-creds` / `common-docker-image-build` | [`gitops/modules/workloads-common`](../../../../gitops/modules/workloads-common) |
| **`workloads-android`** | Android build, test, and Cuttlefish virtual device workloads | Workload execution DAGs | [`gitops/modules/workloads-android`](../../../../gitops/modules/workloads-android) |

---

## Default Pipeline Structure: Initialization & Execution

By default, every scaffolded module includes **two standard pipeline definitions**:

1. **Initialization Pipeline (`<module-name>-init`)**:
   - **Purpose**: Prepares runtime credentials, validates KCC cloud resources, sets up buckets/topics, initializes database schemas, seeds assets, or warms build caches.
   - **Template**: `{{ .Values.parentModuleName }}-init`
   - **Sensor**: `webhook-{{ .Values.parentModuleName }}-init`
   - **Label**: `app.kubernetes.io/name: init`, `horizon-sdv.io/expose: "true"`
2. **Execution Pipeline (`<module-name>-execute`)**:
   - **Purpose**: Runs the main workload, test suite, artifact generation, or build step.
   - **Template**: `{{ .Values.parentModuleName }}-execute`
   - **Sensor**: `webhook-{{ .Values.parentModuleName }}-execute`
   - **Label**: `app.kubernetes.io/name: execute`, `horizon-sdv.io/expose: "true"`

---

## Step-by-Step Module Creation Workflow

### Phase 1: Planning & Quick Questionnaire

Confirm the 4 core parameters:
1. **Module Name & Title**: Kebab-case identifier and human-readable title based on repository workload/workflow patterns:
   - `aaos-builder` (*"AAOS Image Builder"*)
   - `cvd-launcher` (*"Cuttlefish Virtual Device Launcher"*)
   - `cts-testbench` (*"Android CTS Testbench"*)
2. **KCC Cloud Resources**: GCP resources to declare (Pub/Sub, GCS, BigQuery, IAM).
3. **Application & Routing**: Gateway HTTPRoute prefix (e.g. `/<module-name>`).
4. **Dependencies**: Hard dependencies (e.g. `workloads-common`) or soft dependencies (e.g. `storage-gcs`).

---

### Phase 2: Scaffolding the Module Directory

Run the scaffolding script to generate the complete module skeleton inside `gitops/modules/`:

```bash
./.agents/plugins/horizon-dev/skills/horizon-create-module/scripts/scaffold-module.sh \
  --name my-module \
  --title "My Module Title" \
  --description "Module description here" \
  --path-prefix "/my-module"
```

#### Standard Core Module Layout (`gitops/modules/<module-name>/`)

```text
gitops/modules/<module-name>/
├── Chart.yaml                               # Parent Helm Chart definition
├── README.md                                # Module documentation & usage
├── values.yaml                              # Standard Module Manager injection values
├── portal/
│   └── overview.html                        # Rich HTML card/documentation for Dev Portal
├── templates/
│   ├── module-overview-http.yaml            # ConfigMap + Nginx deployment + Service for portal
│   ├── application-<app-name>.yaml          # ArgoCD Application for GKE app / KCC resources
│   └── application-argo-workflows.yaml      # ArgoCD Application for Argo Workflows child chart
├── <app-name>/                              # Child Helm chart (GKE app + KCC resources)
│   ├── Chart.yaml
│   ├── values.yaml
│   └── templates/
│       ├── deployment.yaml                  # Kubernetes Deployment
│       ├── service.yaml                     # Kubernetes Service
│       ├── gateway-<app-name>.yaml          # Gateway API HTTPRoute & HealthCheckPolicy
│       ├── network-policies.yaml            # NetworkPolicy (defense in depth)
│       └── kcc-<resource>.yaml              # Config Connector manifests (e.g. PubSubTopic)
└── argo-workflows/                          # Child Helm chart for workflows & sensors
    ├── Chart.yaml
    ├── values.yaml
    └── templates/
        ├── workflowtemplates.yaml           # Argo WorkflowTemplates (init + execute)
        └── sensors.yaml                     # Argo Events Sensors (init + execute)
```

---

### Phase 3: Implementing Core Module Components

#### 1. Parent Chart `Chart.yaml` & `values.yaml`

In `gitops/modules/<module-name>/Chart.yaml`:
```yaml
apiVersion: v2
name: <module-name>
description: <Brief description>
version: 0.1.0
type: application
appVersion: "0.1.0"
```

In `gitops/modules/<module-name>/values.yaml`:
```yaml
moduleName: ""
moduleManagerNamespace: module-manager

argocd:
  namespace: argocd
  project: horizon-sdv

repo:
  url: ""
  revision: HEAD

appNamespace: <module-name>-app
overviewServiceName: mod-<module-name>-overview
overviewNamespace: <module-name>-app

app:
  rootPath: /<module-name>

config: {}
softFeaturesEnabled: {}
```

#### 2. Child Application Manifests

In `gitops/modules/<module-name>/templates/application-<app-name>.yaml`:
```yaml
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: mod-{{ .Values.moduleName }}-{{ .Values.appNamespace }}
  namespace: {{ .Values.argocd.namespace }}
  labels:
    app.kubernetes.io/name: {{ .Values.moduleName }}
    horizon-sdv.io/module: {{ .Values.moduleName }}
    horizon-sdv.io/app-role: child
    horizon-sdv.io/expose: "true"
    horizon-sdv.io/module-manager-managed: "true"
  annotations:
    horizon-sdv.io/portal-url: {{ .Values.app.rootPath | quote }}
    horizon-sdv.io/portal-title: "<Application Title>"
    horizon-sdv.io/portal-id: "<app-id>"
  finalizers:
    - resources-finalizer.argocd.argoproj.io
spec:
  project: {{ .Values.argocd.project }}
  source:
    repoURL: {{ .Values.repo.url | quote }}
    targetRevision: {{ .Values.repo.revision | quote }}
    path: gitops/modules/<module-name>/<app-name>
    helm:
      values: |
        namespace: {{ .Values.appNamespace }}
        rootPath: {{ .Values.app.rootPath | quote }}
        gcpProjectId: {{ .Values.config.projectID | quote }}
        parentModuleName: {{ .Values.moduleName | quote }}
        config:
{{ .Values.config | toYaml | nindent 10 }}
  destination:
    server: https://kubernetes.default.svc
    namespace: {{ .Values.appNamespace }}
  syncPolicy:
    syncOptions:
      - CreateNamespace=true
    automated: {}
```

#### 3. Kubernetes Config Connector (KCC) Resources

In `gitops/modules/<module-name>/<app-name>/templates/kcc-pubsub.yaml`:
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
spec: {}
```

#### 4. GKE Gateway API HTTPRoute & HealthCheck

In `gitops/modules/<module-name>/<app-name>/templates/gateway-route.yaml`:
```yaml
apiVersion: gateway.networking.k8s.io/v1beta1
kind: HTTPRoute
metadata:
  name: {{ .Values.parentModuleName }}-route
  namespace: {{ .Values.namespace }}
  labels:
    gateway: gke-gateway
  annotations:
    argocd.argoproj.io/sync-wave: "5"
spec:
  parentRefs:
    - kind: Gateway
      name: gke-gateway
      namespace: {{ .Values.config.namespacePrefix }}gke-gateway
      sectionName: https
  hostnames:
    - {{ .Values.config.domain }}
  rules:
    - matches:
        - path:
            type: PathPrefix
            value: {{ .Values.rootPath | quote }}
      filters:
        - type: URLRewrite
          urlRewrite:
            hostname: {{ .Values.config.domain }}
            path:
              type: ReplacePrefixMatch
              replacePrefixMatch: /
      backendRefs:
        - name: {{ .Values.parentModuleName }}-service
          port: 8080
---
apiVersion: networking.gke.io/v1
kind: HealthCheckPolicy
metadata:
  name: {{ .Values.parentModuleName }}-healthcheck
  namespace: {{ .Values.namespace }}
  annotations:
    argocd.argoproj.io/sync-wave: "5"
spec:
  default:
    checkIntervalSec: 15
    timeoutSec: 15
    healthyThreshold: 1
    unhealthyThreshold: 2
    config:
      type: HTTP
      httpHealthCheck:
        port: 8080
        requestPath: /
  targetRef:
    group: ""
    kind: Service
    name: {{ .Values.parentModuleName }}-service
```

#### 5. Argo Workflows (`init` + `execute`) & Argo Events Sensors

In `gitops/modules/<module-name>/argo-workflows/templates/workflowtemplates.yaml`:
```yaml
# 1. Initialization Pipeline
apiVersion: argoproj.io/v1alpha1
kind: WorkflowTemplate
metadata:
  name: {{ .Values.parentModuleName }}-init
  namespace: {{ .Values.workflowNamespace }}
  labels:
    app.kubernetes.io/name: init
    horizon-sdv.io/expose: "true"
    horizon-sdv.io/module: {{ .Values.parentModuleName }}
  annotations:
    argocd.argoproj.io/sync-wave: "7"
spec:
  entrypoint: run-init
  serviceAccountName: workflow-executor
  arguments:
    parameters:
      - name: horizonSubmittedFrom
        value: ""
      - name: initTarget
        value: "default"
  templates:
    - name: run-init
      dag:
        tasks:
          - name: setup-step
            template: setup-step
    - name: setup-step
      container:
        image: alpine:3.19
        command: [sh, -c]
        args: ["echo Initializing {{ .Values.parentModuleName }}"]
---
# 2. Execution Pipeline
apiVersion: argoproj.io/v1alpha1
kind: WorkflowTemplate
metadata:
  name: {{ .Values.parentModuleName }}-execute
  namespace: {{ .Values.workflowNamespace }}
  labels:
    app.kubernetes.io/name: execute
    horizon-sdv.io/expose: "true"
    horizon-sdv.io/module: {{ .Values.parentModuleName }}
  annotations:
    argocd.argoproj.io/sync-wave: "7"
spec:
  entrypoint: run-execution
  serviceAccountName: workflow-executor
  arguments:
    parameters:
      - name: horizonSubmittedFrom
        value: ""
      - name: executionEnv
        value: "test"
      - name: buildId
        value: "build-001"
  templates:
    - name: run-execution
      dag:
        tasks:
          - name: exec-step
            template: exec-step
    - name: exec-step
      container:
        image: alpine:3.19
        command: [sh, -c]
        args: ["echo Executing workload for {{ .Values.parentModuleName }}"]
```

---

### Phase 4: Registering in ModuleCatalog

Open `gitops/apps/module-manager/templates/module-catalog.yaml` and add the new module under `spec.modules`:

```yaml
    - name: <module-name>
      path: gitops/modules/<module-name>
      overviewPath: portal/overview.html
      overviewService: mod-<module-name>-overview
      overviewServiceNamespace: <module-name>-app
      # Declare dependencies if applicable:
      hardDependencies:
        - workloads-common
      softDependencies:
        - storage-gcs
      softFeaturesPropagation: HelmValuesAndConfigMap
      softFeaturesConfigMapNamespaces:
        - <module-name>-app
```

---

### Phase 5: Verification & Testing Checklist

1. **Helm Template & Lint Validation**:
   ```bash
   helm lint gitops/modules/<module-name>
   helm lint gitops/modules/<module-name>/<app-name>
   helm lint gitops/modules/<module-name>/argo-workflows
   ```
2. **GitOps Sync Verification**:
   ```bash
   kubectl get applications -n argocd | grep mod-<module-name>
   ```
3. **KCC Resource Reconciliation**:
   ```bash
   kubectl get pubsubtopics -n <module-name>-app
   ```
4. **Developer Portal Catalog Verification**:
   - Open Developer Portal -> **Administration** -> **Modules**.
   - Enable `<module-name>` and observe automatic dependency resolution and overview rendering.
5. **Workflow Execution**:
   ```bash
   # Test initialization pipeline
   horizon workflow submit --module <module-name> --template <module-name>-init --output json

   # Test execution pipeline
   horizon workflow submit --module <module-name> --template <module-name>-execute --output json
   ```

---

## Detailed References

- **[01_module_architecture.md](./references/01_module_architecture.md)** — App-of-Apps pattern, folder structures, and `values.yaml` schema.
- **[02_kcc_resources.md](./references/02_kcc_resources.md)** — Google Cloud Config Connector manifests, project ID injection, and sync waves.
- **[03_argo_workflows_and_events.md](./references/03_argo_workflows_and_events.md)** — WorkflowTemplates (`init` + `execute`), parameters, artifacts, and Argo Events Sensors.
- **[04_gke_apps_and_gateway.md](./references/04_gke_apps_and_gateway.md)** — GKE workloads, Gateway API HTTPRoute, HealthCheckPolicy, and NetworkPolicy.
- **[05_module_catalog_registration.md](./references/05_module_catalog_registration.md)** — ModuleCatalog CR, hard/soft dependencies, and portal HTML overview.
- **[06_partner_contributions.md](./references/06_partner_contributions.md)** — Partner contribution workflow, standard module placement in `gitops/modules/`, Apache 2.0 license requirement, non-Apache 2.0 dependencies in `third_party/`, branch rules (`contrib/*` -> `devel`), and CLA.
- **[07_testing_and_verification.md](./references/07_testing_and_verification.md)** — Linting, Argo CD sync, KCC status checks, and Horizon CLI commands.
