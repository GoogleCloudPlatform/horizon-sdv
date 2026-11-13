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
DAG templates of openbsw-builder-image. Both call ClusterWorkflowTemplate common-docker-image-build
(module workloads-common) on the OpenBSW Dockerfile.
  build          entrypoint; values come from the workflow parameters.
  build-defaults called by openbsw-bsw-builder through templateRef; values defaults, imageTag input.
*/ -}}

{{/*
DAG tasks: optional git credentials, then the BuildKit build.
Usage: include "openbsw.builderImage.tasks" (dict "root" . "fromWorkflow" true|false)
*/}}
{{- define "openbsw.builderImage.tasks" -}}
{{- $root := .root -}}
{{- $fromWorkflow := .fromWorkflow -}}
{{- $params := $root.Values.builderImage.parameters -}}
{{- $creds := eq (include "openbsw.useUmbrellaCreds" $root) "true" -}}
{{- if $creds }}
{{ include "openbsw.prepareGitCredsTask" $root }}
{{- end }}
- name: build-image
  templateRef:
    name: {{ $root.Values.builderImage.clusterWorkflowTemplateName | quote }}
    template: build
    clusterScope: true
{{- if $creds }}
  depends: prepare-pipeline-git-creds.Succeeded
{{- end }}
  arguments:
    artifacts:
      - name: source
        path: /workspace
{{- include "openbsw.pipelineRepoGitArtifact" $root | nindent 8 }}
    parameters:
      - name: horizonSubmittedFrom
        value: '{{ "{{" }}workflow.parameters.horizonSubmittedFrom{{ "}}" }}'
      - name: cloudProject
        value: {{ $root.Values.cloud.project | quote }}
      - name: cloudRegion
        value: {{ $root.Values.cloud.region | quote }}
      - name: dockerfileDir
        value: {{ $root.Values.builderImage.dockerfileDir | quote }}
      - name: platform
        value: linux/amd64
{{- if $fromWorkflow }}
      - name: dockerArtifactPathName
        value: '{{ "{{" }}workflow.parameters.dockerArtifactPathName{{ "}}" }}'
      - name: imageTag
        value: '{{ "{{" }}workflow.parameters.imageTag{{ "}}" }}'
      - name: dryRun
        value: '{{ "{{" }}workflow.parameters.dryRun{{ "}}" }}'
      - name: buildArgs
        value: |
{{- range $params }}
{{- if .buildArg }}
          {{ .buildArg }}={{ "{{" }}workflow.parameters.{{ .name }}{{ "}}" }}
{{- end }}
{{- end }}
{{- else }}
      - name: dockerArtifactPathName
        value: {{ include "openbsw.paramDefault" (dict "params" $params "name" "dockerArtifactPathName") | quote }}
      - name: imageTag
        value: '{{ "{{" }}inputs.parameters.imageTag{{ "}}" }}'
      # The calling build needs the image, so always push.
      - name: dryRun
        value: "false"
      - name: buildArgs
        value: |
{{- range $params }}
{{- if .buildArg }}
          {{ .buildArg }}={{ .value }}
{{- end }}
{{- end }}
{{- end }}
{{- end -}}

{{- define "openbsw.builderImage.templates" -}}
- name: build
  dag:
    tasks:
{{- include "openbsw.builderImage.tasks" (dict "root" . "fromWorkflow" true) | nindent 6 }}
- name: build-defaults
  inputs:
    parameters:
      - name: imageTag
        value: {{ include "openbsw.paramDefault" (dict "params" .Values.builderImage.parameters "name" "imageTag") | quote }}
  dag:
    tasks:
{{- include "openbsw.builderImage.tasks" (dict "root" . "fromWorkflow" false) | nindent 6 }}
{{- end -}}
