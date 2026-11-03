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
Initialization step for AAOS builder.
Order: 6/10.
Dependencies: requires compute-vars output and clean completion.
*/ -}}

{{- define "aaos-builder.template.init" -}}
- name: init
  inputs:
    parameters:
      - name: sdkAndroidVersion
{{- if eq (include "aaos-builder.usePerPodGitArtifact" .) "true" }}
    artifacts:
      - name: pipeline-repo
        path: /workspace
        git:
          repo: {{ .Values.spec.pipelineRepoUrl | quote }}
          revision: {{ .Values.spec.pipelineRepoRevision | quote }}
{{- include "aaos-builder.gitArtifactCredsContent" . | nindent 10 }}
{{- end }}
  container:
    image: {{ include "aaos-builder.builderImage" . | quote }}
    workingDir: {{ include "aaos-builder.pipelineMonorepoPath" . | quote }}
    command: [bash, -euo, pipefail, -c]
{{- include "aaos-builder.cloudEnvFrom" . | nindent 4 }}
    env:
{{- include "aaos-builder.commonEnv" . | nindent 6 }}
      - name: AAOS_GERRIT_MANIFEST_URL
        value: "{{ "{{workflow.parameters.manifestUrl}}" }}"
      - name: AAOS_MANIFEST_NAME
        value: "{{ "{{workflow.parameters.manifestName}}" }}"
{{- include "aaos-builder.manifestUrlAllowEnv" . | nindent 6 }}
{{- include "aaos-builder.androidVersionEnv" . | nindent 6 }}
    resources:
      # spec.workflowStepResources.init (repo sync bounded by parallelSyncJobs).
{{- include "aaos-builder.workflowStepResources" (dict "step" "init" "root" .) | nindent 6 }}
    securityContext:
      privileged: true
    volumeMounts:
      - name: aaos-cache
        mountPath: /aaos-cache
{{- if eq (include "aaos-builder.useSharedPipelineWorkspaceVolume" .) "true" }}
      - name: pipeline-workspace
        mountPath: {{ include "aaos-builder.pipelineMonorepoPath" . | quote }}
{{- end }}
{{- if or .Values.localRepoHostPath .Values.localRepoPvcName }}
      - name: local-repo
        mountPath: {{ .Values.localRepoMountPath | quote }}
        readOnly: true
{{- end }}
    args:
      - |
        WORKDIR="${PIPELINE_REPO_ROOT:-{{ include "aaos-builder.pipelineMonorepoPath" . }}}"
        export WORKSPACE="${WORKDIR}"
        # manifestUrl must match the operator allowlist before repo sync.
{{- include "aaos-builder.manifestUrlCheck" . | nindent 8 }}
        "${WORKDIR}"/workloads/android/pipelines/builds/aaos_builder/aaos_initialise.sh
{{- end }}
