#!/usr/bin/env bash
# Copyright (c) 2026 Accenture, All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#         http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

set -euo pipefail

MODULE_NAME=""
MODULE_TITLE=""
MODULE_DESC="Horizon SDV modular workload extension"
PATH_PREFIX=""
BASE_DIR="gitops/modules"

usage() {
  cat << EOF
Usage: $0 --name <module-name> [options]

Options:
  --name, -n         Module identifier (kebab-case, e.g. sample-analytics) [Required]
  --title, -t        Human-readable module title (e.g. "Sample Analytics") [Default: capitalized name]
  --desc, -d         Module description [Default: "$MODULE_DESC"]
  --path-prefix, -p  HTTP Gateway path prefix (e.g. /analytics) [Default: /<module-name>]
  --base-dir, -b     Base modules directory [Default: gitops/modules]
  --help, -h         Show this help message
EOF
  exit 0
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --name|-n)
      MODULE_NAME="$2"
      shift 2
      ;;
    --title|-t)
      MODULE_TITLE="$2"
      shift 2
      ;;
    --desc|-d)
      MODULE_DESC="$2"
      shift 2
      ;;
    --path-prefix|-p)
      PATH_PREFIX="$2"
      shift 2
      ;;
    --base-dir|-b)
      BASE_DIR="$2"
      shift 2
      ;;
    --help|-h)
      usage
      ;;
    *)
      echo "Unknown argument: $1" >&2
      usage
      ;;
  esac
done

if [[ -z "$MODULE_NAME" ]]; then
  echo "Error: --name is required." >&2
  usage
fi

# Sanitize module name: lowercase, numbers and hyphens only
MODULE_NAME="$(echo "$MODULE_NAME" | tr '[:upper:]' '[:lower:]' | tr '_' '-' | tr -cd 'a-z0-9-')"

if [[ -z "$MODULE_TITLE" ]]; then
  MODULE_TITLE="$(echo "$MODULE_NAME" | sed 's/-/ /g' | awk '{for(i=1;i<=NF;i++)sub(/./,toupper(substr($i,1,1)),$i)}1')"
fi

if [[ -z "$PATH_PREFIX" ]]; then
  PATH_PREFIX="/${MODULE_NAME}"
fi

# Ensure leading slash and no trailing slash for path prefix
if [[ "${PATH_PREFIX:0:1}" != "/" ]]; then
  PATH_PREFIX="/${PATH_PREFIX}"
fi
PATH_PREFIX="${PATH_PREFIX%/}"

TARGET_DIR="${BASE_DIR}/${MODULE_NAME}"

if [[ -d "$TARGET_DIR" ]]; then
  echo "Error: Directory '$TARGET_DIR' already exists. Aborting." >&2
  exit 1
fi

echo "================================================================="
echo " Scaffolding Horizon SDV Module: $MODULE_NAME"
echo " Title:       $MODULE_TITLE"
echo " Description: $MODULE_DESC"
echo " Path Prefix: $PATH_PREFIX"
echo " Directory:   $TARGET_DIR"
echo "================================================================="

# Create folder structure
mkdir -p "${TARGET_DIR}/portal"
mkdir -p "${TARGET_DIR}/templates"
mkdir -p "${TARGET_DIR}/${MODULE_NAME}-app/templates"
mkdir -p "${TARGET_DIR}/argo-workflows/templates"

# 1. Parent Chart.yaml
cat << 'EOF' > "${TARGET_DIR}/Chart.yaml"
# Copyright (c) 2026 Accenture, All Rights Reserved.
apiVersion: v2
name: MODULE_NAME_PLACEHOLDER
description: MODULE_DESC_PLACEHOLDER
version: 0.1.0
type: application
appVersion: "0.1.0"
EOF
sed -i.bak "s/MODULE_NAME_PLACEHOLDER/${MODULE_NAME}/g" "${TARGET_DIR}/Chart.yaml"
sed -i.bak "s/MODULE_DESC_PLACEHOLDER/${MODULE_DESC}/g" "${TARGET_DIR}/Chart.yaml"
rm -f "${TARGET_DIR}/Chart.yaml.bak"

# 2. Parent values.yaml
cat << EOF > "${TARGET_DIR}/values.yaml"
# Copyright (c) 2026 Accenture, All Rights Reserved.

# Module name (injected dynamically by Module Manager when enabling).
moduleName: ""

# Namespace where Module Manager runs.
moduleManagerNamespace: module-manager

# ArgoCD namespace and project for child applications.
argocd:
  namespace: argocd
  project: horizon-sdv

# Source repository URL and revision (passed by Module Manager).
repo:
  url: ""
  revision: HEAD

# Namespace for child workloads.
appNamespace: ${MODULE_NAME}-app
overviewServiceName: mod-${MODULE_NAME}-overview
overviewNamespace: ${MODULE_NAME}-app

# Public Gateway route path.
app:
  rootPath: ${PATH_PREFIX}

# Environment config injected by Module Manager.
config: {}

# Soft feature flags injected dynamically by Module Manager.
softFeaturesEnabled: {}
EOF

# 3. Parent README.md
cat << EOF > "${TARGET_DIR}/README.md"
# ${MODULE_TITLE} Module

Helm chart path: \`${BASE_DIR}/${MODULE_NAME}\`

${MODULE_DESC}

## Components

- **Child Application**: \`mod-\${moduleName}-${MODULE_NAME}-app\` (\`${BASE_DIR}/${MODULE_NAME}/${MODULE_NAME}-app\`)
- **Argo Workflows**: \`mod-\${moduleName}-argo-workflows\` (\`${BASE_DIR}/${MODULE_NAME}/argo-workflows\`)
- **Developer Portal Overview**: \`mod-${MODULE_NAME}-overview\` serving \`portal/overview.html\`
- **Gateway Ingress**: \`${PATH_PREFIX}\`

## Enabling the Module

Enable via the Horizon Developer Portal (**Administration** -> **Modules**) or register in \`gitops/apps/module-manager/templates/module-catalog.yaml\`.
EOF

# 4. portal/overview.html
cat << EOF > "${TARGET_DIR}/portal/overview.html"
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8"/>
  <meta name="viewport" content="width=device-width, initial-scale=1"/>
  <title>${MODULE_TITLE}</title>
  <style>
    body {
      font-family: system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
      margin: 0;
      padding: 1.5rem;
      background: #0f172a;
      color: #e2e8f0;
      line-height: 1.6;
    }
    .header {
      display: flex;
      align-items: center;
      gap: 0.75rem;
      margin-bottom: 1.5rem;
    }
    h1 {
      font-size: 1.5rem;
      font-weight: 600;
      margin: 0;
      color: #f8fafc;
    }
    .badge {
      display: inline-block;
      padding: 0.25rem 0.5rem;
      font-size: 0.75rem;
      font-weight: 600;
      border-radius: 6px;
      background: #3b82f6;
      color: #ffffff;
    }
    .card {
      background: rgba(30, 41, 59, 0.7);
      border: 1px solid #334155;
      border-radius: 8px;
      padding: 1.25rem;
      margin-bottom: 1rem;
    }
    h2 {
      font-size: 1.1rem;
      font-weight: 600;
      margin-top: 0;
      margin-bottom: 0.75rem;
      color: #93c5fd;
    }
    ul {
      margin: 0;
      padding-left: 1.25rem;
    }
    li {
      margin-bottom: 0.35rem;
    }
    code {
      font-family: ui-monospace, monospace;
      color: #facc15;
      background: rgba(0, 0, 0, 0.3);
      padding: 0.15rem 0.35rem;
      border-radius: 4px;
    }
  </style>
</head>
<body>
  <div class="header">
    <h1>${MODULE_TITLE}</h1>
    <span class="badge">v0.1.0</span>
  </div>

  <div class="card">
    <h2>Overview</h2>
    <p>${MODULE_DESC}</p>
  </div>

  <div class="card">
    <h2>Endpoints & Capabilities</h2>
    <ul>
      <li><strong>Gateway Route:</strong> <code>${PATH_PREFIX}</code></li>
      <li><strong>Initialization Pipeline:</strong> <code>${MODULE_NAME}-init</code></li>
      <li><strong>Execution Pipeline:</strong> <code>${MODULE_NAME}-execute</code></li>
      <li><strong>Event Streaming:</strong> Managed Pub/Sub topic</li>
    </ul>
  </div>
</body>
</html>
EOF

# 5. templates/module-overview-http.yaml
cat << 'EOF' > "${TARGET_DIR}/templates/module-overview-http.yaml"
# Copyright (c) 2026 Accenture, All Rights Reserved.
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Values.overviewServiceName }}-html
  namespace: {{ .Values.overviewNamespace }}
  labels:
    app.kubernetes.io/name: module-overview
    app.kubernetes.io/component: module-overview-http
    horizon-sdv.io/module: {{ .Values.moduleName | quote }}
data:
  index.html: |-
{{ .Files.Get "portal/overview.html" | nindent 4 }}
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .Values.overviewServiceName }}
  namespace: {{ .Values.overviewNamespace }}
  labels:
    app.kubernetes.io/name: module-overview
    horizon-sdv.io/module: {{ .Values.moduleName | quote }}
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: module-overview
      horizon-sdv.io/module: {{ .Values.moduleName | quote }}
  template:
    metadata:
      labels:
        app.kubernetes.io/name: module-overview
        horizon-sdv.io/module: {{ .Values.moduleName | quote }}
    spec:
      containers:
        - name: nginx
          image: {{ .Values.config.nginx.image }}:{{ .Values.config.nginx.tag }}
          ports:
            - containerPort: 80
              name: http
          volumeMounts:
            - name: html
              mountPath: /usr/share/nginx/html
              readOnly: true
          readinessProbe:
            httpGet:
              path: /
              port: http
            initialDelaySeconds: 2
            periodSeconds: 10
          livenessProbe:
            httpGet:
              path: /
              port: http
            initialDelaySeconds: 5
            periodSeconds: 20
          resources:
            requests:
              cpu: 5m
              memory: 16Mi
            limits:
              memory: 64Mi
      volumes:
        - name: html
          configMap:
            name: {{ .Values.overviewServiceName }}-html
---
apiVersion: v1
kind: Service
metadata:
  name: {{ .Values.overviewServiceName }}
  namespace: {{ .Values.overviewNamespace }}
  labels:
    app.kubernetes.io/name: module-overview
    horizon-sdv.io/module: {{ .Values.moduleName | quote }}
spec:
  type: ClusterIP
  selector:
    app.kubernetes.io/name: module-overview
    horizon-sdv.io/module: {{ .Values.moduleName | quote }}
  ports:
    - name: http
      port: 80
      targetPort: http
EOF

# 6. templates/application-<app>.yaml
cat << EOF > "${TARGET_DIR}/templates/application-${MODULE_NAME}-app.yaml"
# Copyright (c) 2026 Accenture, All Rights Reserved.
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: mod-{{ .Values.moduleName }}-${MODULE_NAME}-app
  namespace: {{ .Values.argocd.namespace }}
  labels:
    app.kubernetes.io/name: {{ .Values.moduleName }}
    horizon-sdv.io/module: {{ .Values.moduleName }}
    horizon-sdv.io/app-role: child
    horizon-sdv.io/expose: "true"
    horizon-sdv.io/module-manager-managed: "true"
  annotations:
    horizon-sdv.io/portal-url: {{ .Values.app.rootPath | quote }}
    horizon-sdv.io/portal-title: "${MODULE_TITLE}"
    horizon-sdv.io/portal-id: "${MODULE_NAME}-app"
  finalizers:
    - resources-finalizer.argocd.argoproj.io
spec:
  project: {{ .Values.argocd.project }}
  source:
    repoURL: {{ .Values.repo.url | quote }}
    targetRevision: {{ .Values.repo.revision | quote }}
    path: ${BASE_DIR}/${MODULE_NAME}/${MODULE_NAME}-app
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
EOF

# 7. templates/application-argo-workflows.yaml
cat << EOF > "${TARGET_DIR}/templates/application-argo-workflows.yaml"
# Copyright (c) 2026 Accenture, All Rights Reserved.
apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: mod-{{ .Values.moduleName }}-argo-workflows
  namespace: {{ .Values.argocd.namespace }}
  labels:
    app.kubernetes.io/name: {{ .Values.moduleName }}
    horizon-sdv.io/module: {{ .Values.moduleName }}
    horizon-sdv.io/app-role: child
    horizon-sdv.io/module-manager-managed: "true"
  finalizers:
    - resources-finalizer.argocd.argoproj.io
spec:
  project: {{ .Values.argocd.project }}
  source:
    repoURL: {{ .Values.repo.url | quote }}
    targetRevision: {{ .Values.repo.revision | quote }}
    path: ${BASE_DIR}/${MODULE_NAME}/argo-workflows
    helm:
      values: |
        workflowNamespace: {{ .Values.config.namespacePrefix }}workflows
        eventsNamespace: {{ .Values.config.namespacePrefix }}argo-events
        parentModuleName: {{ .Values.moduleName | quote }}
        softFeaturesEnabled:
{{ .Values.softFeaturesEnabled | toYaml | nindent 10 }}
  destination:
    server: https://kubernetes.default.svc
    namespace: {{ .Values.config.namespacePrefix }}workflows
  syncPolicy:
    syncOptions:
      - CreateNamespace=true
    automated: {}
EOF

# 8. Child Chart: <MODULE_NAME>-app
cat << 'EOF' > "${TARGET_DIR}/${MODULE_NAME}-app/Chart.yaml"
# Copyright (c) 2026 Accenture, All Rights Reserved.
apiVersion: v2
name: MODULE_NAME_PLACEHOLDER-app
description: GKE Workload and KCC resources for MODULE_NAME_PLACEHOLDER
version: 0.1.0
type: application
appVersion: "0.1.0"
EOF
sed -i.bak "s/MODULE_NAME_PLACEHOLDER/${MODULE_NAME}/g" "${TARGET_DIR}/${MODULE_NAME}-app/Chart.yaml"
rm -f "${TARGET_DIR}/${MODULE_NAME}-app/Chart.yaml.bak"

cat << 'EOF' > "${TARGET_DIR}/${MODULE_NAME}-app/values.yaml"
# Copyright (c) 2026 Accenture, All Rights Reserved.
namespace: ""
rootPath: ""
gcpProjectId: ""
parentModuleName: ""
config:
  domain: ""
  namespacePrefix: ""
  enableNetworkPolicies: true
EOF

cat << 'EOF' > "${TARGET_DIR}/${MODULE_NAME}-app/templates/deployment.yaml"
# Copyright (c) 2026 Accenture, All Rights Reserved.
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .Values.parentModuleName }}-app
  namespace: {{ .Values.namespace }}
  labels:
    app.kubernetes.io/name: {{ .Values.parentModuleName }}
  annotations:
    argocd.argoproj.io/sync-wave: "2"
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: {{ .Values.parentModuleName }}
  template:
    metadata:
      labels:
        app.kubernetes.io/name: {{ .Values.parentModuleName }}
    spec:
      containers:
        - name: app
          image: busybox:1.36
          command: ["/bin/sh", "-c"]
          args:
            - |
              mkdir -p /www
              echo "<h1>${MODULE_TITLE}</h1><p>Running on GKE via Argo CD</p>" > /www/index.html
              exec busybox httpd -f -p 8080 -h /www
          ports:
            - containerPort: 8080
              name: http
          resources:
            requests:
              cpu: 5m
              memory: 16Mi
            limits:
              cpu: 50m
              memory: 64Mi
          readinessProbe:
            httpGet:
              path: /
              port: 8080
            initialDelaySeconds: 2
            periodSeconds: 10
          livenessProbe:
            httpGet:
              path: /
              port: 8080
            initialDelaySeconds: 5
            periodSeconds: 20
---
apiVersion: v1
kind: Service
metadata:
  name: {{ .Values.parentModuleName }}-service
  namespace: {{ .Values.namespace }}
  labels:
    app.kubernetes.io/name: {{ .Values.parentModuleName }}
  annotations:
    argocd.argoproj.io/sync-wave: "2"
spec:
  type: ClusterIP
  selector:
    app.kubernetes.io/name: {{ .Values.parentModuleName }}
  ports:
    - name: http
      port: 8080
      targetPort: http
EOF
sed -i.bak "s/\${MODULE_TITLE}/${MODULE_TITLE}/g" "${TARGET_DIR}/${MODULE_NAME}-app/templates/deployment.yaml"
rm -f "${TARGET_DIR}/${MODULE_NAME}-app/templates/deployment.yaml.bak"

cat << 'EOF' > "${TARGET_DIR}/${MODULE_NAME}-app/templates/gateway-route.yaml"
# Copyright (c) 2026 Accenture, All Rights Reserved.
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
EOF

cat << 'EOF' > "${TARGET_DIR}/${MODULE_NAME}-app/templates/network-policies.yaml"
# Copyright (c) 2026 Accenture, All Rights Reserved.
{{- if .Values.config.enableNetworkPolicies }}
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: allow-{{ .Values.parentModuleName }}-ingress-from-gateway
  namespace: {{ .Values.namespace }}
  annotations:
    argocd.argoproj.io/sync-wave: "0"
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/name: {{ .Values.parentModuleName }}
  policyTypes:
    - Ingress
  ingress:
    - ports:
        - protocol: TCP
          port: 8080
{{- end }}
EOF

cat << 'EOF' > "${TARGET_DIR}/${MODULE_NAME}-app/templates/kcc-pubsub.yaml"
# Copyright (c) 2026 Accenture, All Rights Reserved.
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
EOF

# 9. Child Chart: argo-workflows
cat << 'EOF' > "${TARGET_DIR}/argo-workflows/Chart.yaml"
# Copyright (c) 2026 Accenture, All Rights Reserved.
apiVersion: v2
name: MODULE_NAME_PLACEHOLDER-argo-workflows
description: Argo Workflows and Sensors for MODULE_NAME_PLACEHOLDER
version: 0.1.0
type: application
appVersion: "0.1.0"
EOF
sed -i.bak "s/MODULE_NAME_PLACEHOLDER/${MODULE_NAME}/g" "${TARGET_DIR}/argo-workflows/Chart.yaml"
rm -f "${TARGET_DIR}/argo-workflows/Chart.yaml.bak"

cat << 'EOF' > "${TARGET_DIR}/argo-workflows/values.yaml"
# Copyright (c) 2026 Accenture, All Rights Reserved.
workflowNamespace: ""
eventsNamespace: ""
parentModuleName: ""
softFeaturesEnabled: {}
EOF

cat << 'EOF' > "${TARGET_DIR}/argo-workflows/templates/workflowtemplates.yaml"
# Copyright (c) 2026 Accenture, All Rights Reserved.
# 1. Initialization Pipeline (prepares environment, config, seeds, or caches)
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
        description: "Target environment or seed dataset to initialize"
  templates:
    - name: run-init
      dag:
        tasks:
          - name: log-parameters
            template: log-parameters
          - name: setup-resources
            template: setup-resources
            depends: log-parameters
    - name: log-parameters
      container:
        image: alpine:3.19
        command: [sh, -c]
        args:
          - |
            echo "WorkflowTemplate: {{ .Values.parentModuleName }}-init"
            echo "Submitted From: {{ "{{workflow.parameters.horizonSubmittedFrom}}" }}"
            echo "Target: {{ "{{workflow.parameters.initTarget}}" }}"
            echo "Initialization started at $(date -u +%Y-%m-%dT%H:%M:%SZ)"
    - name: setup-resources
      container:
        image: alpine:3.19
        command: [sh, -c]
        args:
          - |
            echo "Initializing resources for {{ .Values.parentModuleName }}..."
            echo "Initialization successful - $(date -u +%Y-%m-%dT%H:%M:%SZ)" > /tmp/init-status.txt
            cat /tmp/init-status.txt
      outputs:
        artifacts:
          - name: init-status
            path: /tmp/init-status.txt
---
# 2. Execution Pipeline (main workload / build / test execution)
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
        description: "Target execution environment"
      - name: buildId
        value: "build-001"
        description: "Execution run identifier"
  templates:
    - name: run-execution
      dag:
        tasks:
          - name: log-parameters
            template: log-parameters
          - name: run-workload
            template: run-workload
            depends: log-parameters
    - name: log-parameters
      container:
        image: alpine:3.19
        command: [sh, -c]
        args:
          - |
            echo "WorkflowTemplate: {{ .Values.parentModuleName }}-execute"
            echo "Submitted From: {{ "{{workflow.parameters.horizonSubmittedFrom}}" }}"
            echo "Environment: {{ "{{workflow.parameters.executionEnv}}" }}"
            echo "Build ID: {{ "{{workflow.parameters.buildId}}" }}"
            echo "Execution started at $(date -u +%Y-%m-%dT%H:%M:%SZ)"
    - name: run-workload
      container:
        image: alpine:3.19
        command: [sh, -c]
        args:
          - |
            echo "Executing workload for {{ .Values.parentModuleName }}..."
            mkdir -p /tmp/output
            echo "Execution completed successfully at $(date -u +%Y-%m-%dT%H:%M:%SZ)" > /tmp/output/result.txt
            tar -czf /tmp/output/execution-artifact.tgz -C /tmp/output result.txt
            cat /tmp/output/result.txt
      outputs:
        artifacts:
          - name: result
            path: /tmp/output/result.txt
          - name: execution-artifact
            path: /tmp/output/execution-artifact.tgz
EOF

cat << 'EOF' > "${TARGET_DIR}/argo-workflows/templates/sensors.yaml"
# Copyright (c) 2026 Accenture, All Rights Reserved.
# 1. Webhook Sensor for Initialization Pipeline
apiVersion: argoproj.io/v1alpha1
kind: Sensor
metadata:
  name: webhook-{{ .Values.parentModuleName }}-init
  namespace: {{ .Values.eventsNamespace }}
  annotations:
    argocd.argoproj.io/sync-wave: "8"
spec:
  eventBusName: default
  template:
    serviceAccountName: sensor-submit-workflow
  dependencies:
    - name: webhook-dep
      eventSourceName: webhook
      eventName: workflow-dispatch
      filters:
        data:
          - path: body.workflowTemplateName
            type: string
            value:
              - "{{ .Values.parentModuleName }}-init"
  triggers:
    - template:
        name: submit-init
        argoWorkflow:
          operation: submit
          source:
            resource:
              apiVersion: argoproj.io/v1alpha1
              kind: Workflow
              metadata:
                generateName: webhook-{{ .Values.parentModuleName }}-init-
                namespace: {{ .Values.workflowNamespace }}
              spec:
                workflowTemplateRef:
                  name: {{ .Values.parentModuleName }}-init
                arguments:
                  parameters:
                    - name: horizonSubmittedFrom
                    - name: initTarget
          parameters:
            - src:
                dependencyName: webhook-dep
                dataKey: body.horizonSubmittedBy
              dest: metadata.annotations.horizon-sdv\.io/submitted-by
            - src:
                dependencyName: webhook-dep
                dataKey: body.horizonSubmittedFrom
              dest: metadata.labels.horizon-sdv\.io/submitted-from
            - src:
                dependencyName: webhook-dep
                dataKey: body.horizonSubmittedFrom
              dest: spec.arguments.parameters.0.value
            - src:
                dependencyName: webhook-dep
                dataKey: body.initTarget
              dest: spec.arguments.parameters.1.value
---
# 2. Webhook Sensor for Execution Pipeline
apiVersion: argoproj.io/v1alpha1
kind: Sensor
metadata:
  name: webhook-{{ .Values.parentModuleName }}-execute
  namespace: {{ .Values.eventsNamespace }}
  annotations:
    argocd.argoproj.io/sync-wave: "8"
spec:
  eventBusName: default
  template:
    serviceAccountName: sensor-submit-workflow
  dependencies:
    - name: webhook-dep
      eventSourceName: webhook
      eventName: workflow-dispatch
      filters:
        data:
          - path: body.workflowTemplateName
            type: string
            value:
              - "{{ .Values.parentModuleName }}-execute"
  triggers:
    - template:
        name: submit-execute
        argoWorkflow:
          operation: submit
          source:
            resource:
              apiVersion: argoproj.io/v1alpha1
              kind: Workflow
              metadata:
                generateName: webhook-{{ .Values.parentModuleName }}-execute-
                namespace: {{ .Values.workflowNamespace }}
              spec:
                workflowTemplateRef:
                  name: {{ .Values.parentModuleName }}-execute
                arguments:
                  parameters:
                    - name: horizonSubmittedFrom
                    - name: executionEnv
                    - name: buildId
          parameters:
            - src:
                dependencyName: webhook-dep
                dataKey: body.horizonSubmittedBy
              dest: metadata.annotations.horizon-sdv\.io/submitted-by
            - src:
                dependencyName: webhook-dep
                dataKey: body.horizonSubmittedFrom
              dest: metadata.labels.horizon-sdv\.io/submitted-from
            - src:
                dependencyName: webhook-dep
                dataKey: body.horizonSubmittedFrom
              dest: spec.arguments.parameters.0.value
            - src:
                dependencyName: webhook-dep
                dataKey: body.executionEnv
              dest: spec.arguments.parameters.1.value
            - src:
                dependencyName: webhook-dep
                dataKey: body.buildId
              dest: spec.arguments.parameters.2.value
EOF

echo "✓ Successfully scaffolded module at: $TARGET_DIR"
echo ""
echo "Next Steps:"
echo "1. Register in 'gitops/apps/module-manager/templates/module-catalog.yaml':"
echo "   - name: ${MODULE_NAME}"
echo "     path: ${BASE_DIR}/${MODULE_NAME}"
echo "     overviewPath: portal/overview.html"
echo "     overviewService: mod-${MODULE_NAME}-overview"
echo "     overviewServiceNamespace: ${MODULE_NAME}-app"
echo ""
echo "2. Run linting:"
echo "   helm lint ${TARGET_DIR}"
echo "   helm lint ${TARGET_DIR}/${MODULE_NAME}-app"
echo "   helm lint ${TARGET_DIR}/argo-workflows"
echo "================================================================="
