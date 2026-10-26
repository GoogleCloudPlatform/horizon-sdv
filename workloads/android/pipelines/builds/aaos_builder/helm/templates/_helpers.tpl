{{- /*
Copyright (c) 2026 Accenture, All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

        http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

Description:
Helm template helpers for aaos-builder (workflow namespace and service account).
*/ -}}

{{/*
Workflow namespace: explicit .Values.namespace, or namespacePrefix + "workflows" (matches GitOps namespacePrefix + workflows).
*/}}
{{- define "aaos-builder.workflowNamespace" -}}
{{- coalesce .Values.namespace (printf "%s%s" (.Values.namespacePrefix | default "") "workflows") -}}
{{- end -}}

{{/*
Workflow pod service account: when useElevatedWorkflowIam is true, workflow-executor-elevated (Terraform env/main.tf argo_workflows_elevated → gke-argo-workflows-elevated-sa). Otherwise spec.serviceAccountName (default workflow-executor, argo_workflows → gke-argo-workflows-sa).
*/}}
{{- define "aaos-builder.workflowServiceAccountName" -}}
{{- if .Values.spec.useElevatedWorkflowIam -}}
workflow-executor-elevated
{{- else -}}
{{- .Values.spec.serviceAccountName | default "workflow-executor" -}}
{{- end -}}
{{- end -}}

{{/*
SCM auth: default values and GitOps set .Values.git.authMethod; .Values.scm.authMethod is accepted for compatibility.
*/}}
{{- define "aaos-builder.scmAuthMethod" -}}
{{- $scm := .Values.scm | default dict -}}
{{- coalesce .Values.git.authMethod $scm.authMethod "" -}}
{{- end -}}

{{/*
Full Artifact Registry reference for the AAOS builder runtime image (tag from spec.imageBuild.imageTag).
*/}}
{{- define "aaos-builder.builderImage" -}}
{{- printf "%s-docker.pkg.dev/%s/%s:%s" .Values.spec.cloudRegion .Values.spec.cloudProject .Values.spec.imageBuild.artifactPathName .Values.spec.imageBuild.imageTag -}}
{{- end -}}

{{/*
Gemini CLI for the AAOS **build** step only (`GEMINI_COMMAND_LINE` in build container).
When **spec.geminiModel** is set, emits `gemini --model … --yolo --output-format json`; else **spec.geminiCommandLine**.
The **`gemini-review`** DAG step is an **inline** container template: it sets **`GEMINI_*`** / **`CLOUD_*`** env and runs **`workloads/common/agentic-ai/gemini/run_ai_review.sh`**. **`spec.gemini*`** keys tune both **build** and **inline gemini-review** unless you split them later.
*/}}
{{- define "aaos-builder.geminiCommandLineResolved" -}}
{{- if .Values.spec.geminiModel -}}
gemini --model {{ .Values.spec.geminiModel }} --yolo --output-format json
{{- else -}}
{{- .Values.spec.geminiCommandLine -}}
{{- end -}}
{{- end -}}

{{/*
True when using the per-workflow pipeline-workspace PVC (remote repo + sharedPipelineWorkspace).
*/}}
{{- define "aaos-builder.useSharedPipelineWorkspaceVolume" -}}
{{- if and (not (or .Values.localRepoHostPath .Values.localRepoPvcName)) .Values.sharedPipelineWorkspace }}true{{- end -}}
{{- end -}}

{{/*
True when each pod should use an Argo git artifact for the pipeline repo (remote, not shared volume).
Used by clean|init|build|storage only when sharedPipelineWorkspace is false.
*/}}
{{- define "aaos-builder.usePerPodGitArtifact" -}}
{{- if and (not (or .Values.localRepoHostPath .Values.localRepoPvcName)) (not .Values.sharedPipelineWorkspace) }}true{{- end -}}
{{- end -}}

{{/*
True when gemini-review should clone the pipeline repo via Argo git artifact into /workspace in its own pod.
Always for remote git (even when other WT steps use shared pipeline-workspace PVC at /horizon).
*/}}
{{- define "aaos-builder.useGeminiReviewGitArtifact" -}}
{{- if and (not (or .Values.localRepoHostPath .Values.localRepoPvcName)) }}true{{- end -}}
{{- end -}}

{{/*
Filesystem path for the Horizon monorepo checkout: /workspace when using per-pod git artifacts; /horizon when
using shared pipeline-workspace PVC (avoids Argo reserving /workspace for git init vs explicit volumeMounts).
*/}}
{{- define "aaos-builder.pipelineMonorepoPath" -}}
{{- if eq (include "aaos-builder.useSharedPipelineWorkspaceVolume" .) "true" -}}/horizon{{- else -}}/workspace{{- end -}}
{{- end -}}

{{/*
Kubernetes resources block for spec.workflowStepResources.<step> (fetchPipeline, init, clean, storage, geminiReview).
*/}}
{{- define "aaos-builder.workflowStepResources" -}}
{{- $r := index .root.Values.spec.workflowStepResources .step }}
requests:
  cpu: {{ $r.requests.cpu | quote }}
  memory: {{ $r.requests.memory | quote }}
limits:
  cpu: {{ $r.limits.cpu | quote }}
  memory: {{ $r.limits.memory | quote }}
{{- end -}}

{{/*
Argo git artifact HTTPS credentials: GitHub App (per-workflow Secret) or PAT Secret name from spec.pipelineRepoSecret.
Indent with nindent from each call site (e.g. nindent 10 under init git:, nindent 16 under gemini-review git:).
*/}}
{{- define "aaos-builder.gitArtifactCredsContent" -}}
{{- $auth := include "aaos-builder.scmAuthMethod" . | trim -}}
{{- if and (or (eq $auth "app") (eq $auth "userpass")) (not (or .Values.localRepoHostPath .Values.localRepoPvcName)) }}
usernameSecret:
  name: "{{ "{{" }}workflow.uid{{ "}}" }}-pipeline-git-creds"
  key: username
passwordSecret:
  name: "{{ "{{" }}workflow.uid{{ "}}" }}-pipeline-git-creds"
  key: password
{{- else if .Values.spec.pipelineRepoSecret }}
usernameSecret:
  name: {{ .Values.spec.pipelineRepoSecret | quote }}
  key: username
passwordSecret:
  name: {{ .Values.spec.pipelineRepoSecret | quote }}
  key: password
{{- end }}
{{- end -}}

{{/*
Pod affinity: schedule this step on the same node as other pods for the same Argo Workflow
(RWO PVC / node-local cache with siblings). Argo substitutes {{workflow.name}} at runtime.
*/}}
{{- define "aaos-builder.podAffinitySameWorkflow" -}}
affinity:
  podAffinity:
    requiredDuringSchedulingIgnoredDuringExecution:
      - labelSelector:
          matchExpressions:
            - key: workflows.argoproj.io/workflow
              operator: In
              values:
                - {{ "{{workflow.name}}" }}
        topologyKey: kubernetes.io/hostname
{{- end -}}
