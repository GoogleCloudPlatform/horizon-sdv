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
Build task definition for abfs-builder-runtime-image.
Order: 1/1.
Dependencies: uses shared ClusterWorkflowTemplate (build template).
*/ -}}

{{- define "abfs-builder-runtime-image.template.build" -}}
{{- $useLocalRepo := or .Values.localRepoHostPath .Values.localRepoPvcName -}}
{{- $auth := include "abfs-builder-runtime-image.scmAuthMethod" . | trim -}}
{{- $umbrellaCreds := and (not $useLocalRepo) (or (eq $auth "app") (eq $auth "userpass")) -}}
- name: build
  dag:
    tasks:
{{- if $umbrellaCreds }}
      - name: prepare-pipeline-git-creds
        templateRef:
          name: prepare-pipeline-git-creds
          template: prepare-pipeline-git-creds
          clusterScope: true
        arguments:
          parameters:
            - name: scmAuthMethod
              value: {{ include "abfs-builder-runtime-image.scmAuthMethod" . | trim | quote }}
            - name: pipelineStaticGitSecretName
              value: {{ .Values.spec.pipelineRepoSecret | default "workflow-pipeline-git-creds" | quote }}
            - name: horizonSubmittedFrom
              value: '{{ "{{" }}workflow.parameters.horizonSubmittedFrom{{ "}}" }}'
{{- end }}
      - name: resolve-flavor
        template: resolve-flavor
        arguments:
          parameters:
            - name: templateFlavor
              value: '{{ "{{" }}workflow.parameters.templateFlavor{{ "}}" }}'
            - name: imageTag
              value: '{{ "{{" }}workflow.parameters.imageTag{{ "}}" }}'
      - name: build-image
        templateRef:
          name: {{ .Values.clusterWorkflowTemplateName | quote }}
          template: build
          clusterScope: true
{{- if $umbrellaCreds }}
        depends: "prepare-pipeline-git-creds.Succeeded && resolve-flavor.Succeeded"
{{- else }}
        depends: resolve-flavor.Succeeded
{{- end }}
        arguments:
{{- if not $useLocalRepo }}
          artifacts:
            - name: source
              path: /workspace
              git:
                repo: {{ .Values.spec.pipelineRepoUrl | quote }}
                revision: {{ .Values.spec.pipelineRepoRevision | quote }}
{{- include "abfs-builder-runtime-image.gitArtifactCredsContent" . | nindent 16 }}
{{- end }}
          parameters:
            - name: horizonSubmittedFrom
              value: '{{ "{{" }}workflow.parameters.horizonSubmittedFrom{{ "}}" }}'
            - name: cloudProject
              value: {{ .Values.spec.cloudProject | quote }}
            - name: cloudRegion
              value: {{ .Values.spec.cloudRegion | quote }}
            - name: dockerArtifactPathName
              value: "{{ "{{" }}workflow.parameters.dockerArtifactPathName{{ "}}" }}"
            - name: imageTag
              value: '{{ "{{" }}tasks.resolve-flavor.outputs.parameters.imageTag{{ "}}" }}'
            - name: dryRun
              value: "{{ "{{" }}workflow.parameters.NO_PUSH{{ "}}" }}"
            - name: dockerfileDir
              value: {{ .Values.spec.dockerfileDir | quote }}
            - name: buildArgs
              value: |
                ABFS=true
                LINUX_DISTRIBUTION={{ "{{" }}workflow.parameters.linuxDistribution{{ "}}" }}
                NODEJS_VERSION={{ "{{" }}workflow.parameters.nodejsVersion{{ "}}" }}
                GOOGLE_DISTRIBUTION_REGISTRY={{ "{{" }}workflow.parameters.googleDistributionRegistry{{ "}}" }}
                ABFS_DISTRIBUTION_REGISTRY={{ "{{" }}workflow.parameters.abfsDistributionRegistry{{ "}}" }}
                GCLOUD_CLI_VERSION={{ "{{" }}workflow.parameters.gcloudCliVersion{{ "}}" }}
                KUBECTL_VERSION={{ "{{" }}workflow.parameters.kubectlVersion{{ "}}" }}
                PACKER_VERSION={{ "{{" }}workflow.parameters.packerVersion{{ "}}" }}
                ENABLE_GEMINI_AI_ASSISTANT={{ "{{" }}workflow.parameters.enableGeminiAiAssistant{{ "}}" }}
                GEMINI_CLI_VERSION={{ "{{" }}workflow.parameters.geminiCliVersion{{ "}}" }}
                GOOGLE_ADK_VERSION={{ "{{" }}workflow.parameters.googleAdkVersion{{ "}}" }}
            - name: platform
              value: linux/amd64
{{- end }}

{{- define "abfs-builder-runtime-image.template.build-defaults" -}}
{{- $useLocalRepo := or .Values.localRepoHostPath .Values.localRepoPvcName -}}
{{- $auth := include "abfs-builder-runtime-image.scmAuthMethod" . | trim -}}
{{- $umbrellaCreds := and (not $useLocalRepo) (or (eq $auth "app") (eq $auth "userpass")) -}}
- name: build-defaults
  dag:
    tasks:
{{- if $umbrellaCreds }}
      - name: prepare-pipeline-git-creds
        templateRef:
          name: prepare-pipeline-git-creds
          template: prepare-pipeline-git-creds
          clusterScope: true
        arguments:
          parameters:
            - name: scmAuthMethod
              value: {{ include "abfs-builder-runtime-image.scmAuthMethod" . | trim | quote }}
            - name: pipelineStaticGitSecretName
              value: {{ .Values.spec.pipelineRepoSecret | default "workflow-pipeline-git-creds" | quote }}
            - name: horizonSubmittedFrom
              value: '{{ "{{" }}workflow.parameters.horizonSubmittedFrom{{ "}}" }}'
{{- end }}
      - name: resolve-flavor
        template: resolve-flavor
        arguments:
          parameters:
            - name: templateFlavor
              value: {{ .Values.spec.templateFlavor | quote }}
            - name: imageTag
              value: {{ .Values.spec.imageTag | quote }}
      - name: build-image
        templateRef:
          name: {{ .Values.clusterWorkflowTemplateName | quote }}
          template: build
          clusterScope: true
{{- if $umbrellaCreds }}
        depends: "prepare-pipeline-git-creds.Succeeded && resolve-flavor.Succeeded"
{{- else }}
        depends: resolve-flavor.Succeeded
{{- end }}
        arguments:
{{- if not $useLocalRepo }}
          artifacts:
            - name: source
              path: /workspace
              git:
                repo: {{ .Values.spec.pipelineRepoUrl | quote }}
                revision: {{ .Values.spec.pipelineRepoRevision | quote }}
{{- include "abfs-builder-runtime-image.gitArtifactCredsContent" . | nindent 16 }}
{{- end }}
          parameters:
            - name: horizonSubmittedFrom
              value: '{{ "{{" }}workflow.parameters.horizonSubmittedFrom{{ "}}" }}'
            - name: cloudProject
              value: {{ .Values.spec.cloudProject | quote }}
            - name: cloudRegion
              value: {{ .Values.spec.cloudRegion | quote }}
            - name: dockerArtifactPathName
              value: {{ .Values.spec.dockerArtifactPathName | quote }}
            - name: imageTag
              value: '{{ "{{" }}tasks.resolve-flavor.outputs.parameters.imageTag{{ "}}" }}'
            - name: dryRun
              value: {{ .Values.spec.noPush | quote }}
            - name: dockerfileDir
              value: {{ .Values.spec.dockerfileDir | quote }}
            - name: buildArgs
              value: |
                ABFS=true
                LINUX_DISTRIBUTION={{ .Values.spec.linuxDistribution }}
                NODEJS_VERSION={{ .Values.spec.nodejsVersion }}
                GOOGLE_DISTRIBUTION_REGISTRY={{ .Values.spec.googleDistributionRegistry }}
                ABFS_DISTRIBUTION_REGISTRY={{ include "abfs-builder-runtime-image.abfsDistributionRegistry" . }}
                GCLOUD_CLI_VERSION={{ .Values.spec.gcloudCliVersion }}
                KUBECTL_VERSION={{ .Values.spec.kubectlVersion }}
                PACKER_VERSION={{ .Values.spec.packerVersion }}
                ENABLE_GEMINI_AI_ASSISTANT={{ .Values.spec.enableGeminiAiAssistant }}
                GEMINI_CLI_VERSION={{ .Values.spec.geminiCliVersion }}
                GOOGLE_ADK_VERSION={{ .Values.spec.googleAdkVersion }}
            - name: platform
              value: linux/amd64
{{- end }}
