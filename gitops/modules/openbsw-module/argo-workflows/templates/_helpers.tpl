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

{{/*
Pod scheduling for every OpenBSW workflow step.
Include under spec.templateDefaults:

  templateDefaults:
{{ include "openbsw.templateDefaults" . | nindent 4 }}

Every pod carries openbsw_pod and refuses a node that already runs an openbsw_pod pod. The Jenkins
OpenBSW pods use the same label and rule, so Argo and Jenkins pods never share a node either.
Resource requests are capacity settings only; they do not reserve the node.
*/}}
{{- define "openbsw.templateDefaults" -}}
metadata:
  annotations:
    cluster-autoscaler.kubernetes.io/safe-to-evict: "false"
  labels:
    horizon-sdv.io/openbsw-workflow: "true"
    openbsw_pod: "true"
nodeSelector:
  workloadLabel: {{ .Values.nodeSelector.workloadLabel | quote }}
  cloud.google.com/gke-nodepool: {{ .Values.nodePool.name | quote }}
tolerations:
{{ .Values.tolerations | toYaml }}
affinity:
  podAntiAffinity:
    requiredDuringSchedulingIgnoredDuringExecution:
      - labelSelector:
          matchExpressions:
            - key: openbsw_pod
              operator: Exists
        topologyKey: kubernetes.io/hostname
{{- end -}}

{{/*
Common WorkflowTemplate spec fields: archiveLogs, ServiceAccount, podGC, TTL.
*/}}
{{- define "openbsw.workflowSpecCommon" -}}
archiveLogs: true
serviceAccountName: {{ .Values.serviceAccountName | default "workflow-executor" | quote }}
{{- if .Values.podGcStrategy }}
podGC:
  strategy: {{ .Values.podGcStrategy | quote }}
{{- end }}
{{- if .Values.workflowTtlSecondsAfterCompletion }}
ttlStrategy:
  secondsAfterCompletion: {{ .Values.workflowTtlSecondsAfterCompletion }}
{{- end }}
{{- end -}}

{{- define "openbsw.cloudEnvFrom" -}}
{{- if .Values.cloudEnvConfigMapName }}
envFrom:
  - configMapRef:
      name: {{ .Values.cloudEnvConfigMapName | quote }}
{{- end }}
{{- end -}}

{{/*
Default value of a parameter in a values parameter list.
Usage: include "openbsw.paramDefault" (dict "params" .Values.builderImage.parameters "name" "imageTag")
*/}}
{{- define "openbsw.paramDefault" -}}
{{- $name := .name -}}
{{- range .params }}{{ if eq .name $name }}{{ .value }}{{ end }}{{ end -}}
{{- end -}}

{{/*
openbsw-builder image without its tag. The tag comes from the workflow (imageTag).
*/}}
{{- define "openbsw.builderImageRepository" -}}
{{- printf "%s-docker.pkg.dev/%s/%s" .Values.cloud.region .Values.cloud.project (include "openbsw.paramDefault" (dict "params" .Values.builderImage.parameters "name" "dockerArtifactPathName")) -}}
{{- end -}}

{{/*
openbsw-builder image with the tag from the workflow parameter imageTag.
*/}}
{{- define "openbsw.builderImageRef" -}}
{{ include "openbsw.builderImageRepository" . }}:{{ "{{" }}workflow.parameters.imageTag{{ "}}" }}
{{- end -}}

{{/*
WorkflowTemplate spec.arguments.parameters: horizonSubmittedFrom, then each entry of a values parameter list.
*/}}
{{- define "openbsw.workflowParameters" -}}
- name: horizonSubmittedFrom
  value: ""
  description: "Where the submit call originated (Horizon); mirrored on workflow label horizon-sdv.io/submitted-from."
{{- range . }}
- name: {{ .name }}
  value: {{ .value | toString | quote }}
{{- with .enum }}
  enum:
{{- range . }}
    - {{ . | toString | quote }}
{{- end }}
{{- end }}
  description: {{ .description | quote }}
{{- end }}
{{- end -}}

{{/*
Webhook Sensor for one WorkflowTemplate. Webhook body fields map to spec.arguments.parameters by
position, so the mapping is generated from the same list as the WorkflowTemplate parameters.
Missing body fields fall back to the WorkflowTemplate default. body.submittedBy (the caller's
username) also goes to the display-only annotation horizon-sdv.io/submitted-by-name, so the
Developer Portal lists runs by username; horizon-sdv.io/submitted-by stays the owner (token subject).
Usage: include "openbsw.webhookSensor" (dict "root" . "name" <template name> "generateName" <prefix> "params" <list>)
*/}}
{{- define "openbsw.webhookSensor" -}}
{{- $root := .root -}}
apiVersion: argoproj.io/v1alpha1
kind: Sensor
metadata:
  name: webhook-{{ .name }}
  namespace: {{ $root.Values.eventsNamespace | quote }}
  labels:
    horizon-sdv.io/module: {{ $root.Values.moduleName | quote }}
  annotations:
    argocd.argoproj.io/sync-wave: "9"
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
            # String filters are regular expressions; anchor so only this template matches.
            value:
              - {{ printf "^%s$" .name | quote }}
  triggers:
    - template:
        name: submit-{{ .name }}
        argoWorkflow:
          operation: submit
          source:
            resource:
              apiVersion: argoproj.io/v1alpha1
              kind: Workflow
              metadata:
                generateName: {{ .generateName }}
                namespace: {{ $root.Values.workflowNamespace | quote }}
              spec:
                workflowTemplateRef:
                  name: {{ .name | quote }}
                arguments:
                  parameters:
                    - name: horizonSubmittedFrom
{{- range .params }}
                    - name: {{ .name }}
{{- end }}
          parameters:
            - src:
                dependencyName: webhook-dep
                dataKey: body.horizonSubmittedBy
              dest: metadata.annotations.horizon-sdv\.io/submitted-by
            - src:
                dependencyName: webhook-dep
                dataKey: body.submittedBy
                value: ""
              dest: metadata.annotations.horizon-sdv\.io/submitted-by-name
            - src:
                dependencyName: webhook-dep
                dataKey: body.horizonSubmittedFrom
              dest: metadata.labels.horizon-sdv\.io/submitted-from
            - src:
                dependencyName: webhook-dep
                dataKey: body.horizonSubmittedFrom
                value: ""
              dest: spec.arguments.parameters.0.value
{{- range $i, $p := .params }}
            - src:
                dependencyName: webhook-dep
                dataKey: body.{{ $p.name }}
                value: {{ $p.value | toString | quote }}
              dest: spec.arguments.parameters.{{ add1 $i }}.value
{{- end }}
{{- end -}}

{{- define "openbsw.scmAuthMethod" -}}
{{- .Values.git.authMethod | default "" | trim -}}
{{- end -}}

{{/*
True when the pipeline repo needs per-workflow credentials (GitHub App or user/password).
*/}}
{{- define "openbsw.useUmbrellaCreds" -}}
{{- $auth := include "openbsw.scmAuthMethod" . -}}
{{- if or (eq $auth "app") (eq $auth "userpass") }}true{{- end -}}
{{- end -}}

{{/*
DAG task (or step) that mints the per-workflow {{workflow.uid}}-pipeline-git-creds Secret.
Usage: include "openbsw.prepareGitCredsTask" (dict "root" . "depends" "<expression>" "when" "<expression>")
depends and when are optional. openbsw-bsw-builder sets depends so this task never overlaps the creds
task that the image build runs inside build-defaults (same Secret name, same workflow uid). The MTK
Connect exit handlers set when, and mint again because a GitHub App token expires after an hour.
*/}}
{{- define "openbsw.prepareGitCredsTask" -}}
{{- $root := .root -}}
- name: prepare-pipeline-git-creds
{{- with .depends }}
  depends: {{ . | quote }}
{{- end }}
{{- with .when }}
  when: {{ . | quote }}
{{- end }}
  templateRef:
    name: prepare-pipeline-git-creds
    template: prepare-pipeline-git-creds
    clusterScope: true
  arguments:
    parameters:
      - name: scmAuthMethod
        value: {{ include "openbsw.scmAuthMethod" $root | quote }}
      - name: pipelineStaticGitSecretName
        value: {{ $root.Values.pipelineRepo.secret | default "workflow-pipeline-git-creds" | quote }}
      - name: horizonSubmittedFrom
        value: '{{ "{{" }}workflow.parameters.horizonSubmittedFrom{{ "}}" }}'
{{- end -}}

{{/*
Argo git artifact for the Horizon monorepo, with HTTPS credentials when needed.
Usage: include "openbsw.pipelineRepoGitArtifact" . | nindent N  (under an artifact entry)
*/}}
{{- define "openbsw.pipelineRepoGitArtifact" -}}
git:
  repo: {{ required "pipelineRepo.url is required (parent Application repo.url)" .Values.pipelineRepo.url | quote }}
  revision: {{ required "pipelineRepo.revision is required (parent Application repo.revision)" .Values.pipelineRepo.revision | quote }}
{{- if eq (include "openbsw.useUmbrellaCreds" .) "true" }}
  usernameSecret:
    name: "{{ "{{" }}workflow.uid{{ "}}" }}-pipeline-git-creds"
    key: username
  passwordSecret:
    name: "{{ "{{" }}workflow.uid{{ "}}" }}-pipeline-git-creds"
    key: password
{{- else if .Values.pipelineRepo.secret }}
  usernameSecret:
    name: {{ .Values.pipelineRepo.secret | quote }}
    key: username
  passwordSecret:
    name: {{ .Values.pipelineRepo.secret | quote }}
    key: password
{{- end }}
{{- end -}}

{{/*
Resources block from <resources>.<step>, where resources defaults to bswBuilder.resources.
Usage: include "openbsw.stepResources" (dict "root" . "step" "build" ["resources" .Values.posixTest.resources])
*/}}
{{- define "openbsw.stepResources" -}}
{{- $r := index (.resources | default .root.Values.bswBuilder.resources) .step -}}
requests:
  cpu: {{ $r.requests.cpu | quote }}
  memory: {{ $r.requests.memory | quote }}
limits:
  cpu: {{ $r.limits.cpu | quote }}
  memory: {{ $r.limits.memory | quote }}
{{- end -}}
