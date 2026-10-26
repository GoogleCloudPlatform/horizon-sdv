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
Clean step for aaos-builder.
Order: 4/10.
Dependencies: runs after compute-vars and image check/build.
*/ -}}

{{- define "aaos-builder.template.clean" -}}
- name: clean
{{- if eq (include "aaos-builder.usePerPodGitArtifact" .) "true" }}
  inputs:
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
    workingDir: {{ if or .Values.localRepoHostPath .Values.localRepoPvcName }}"/workspace"{{ else }}{{ include "aaos-builder.pipelineMonorepoPath" . | quote }}{{ end }}
    command: [bash, -euo, pipefail, -c]
{{- include "aaos-builder.cloudEnvFrom" . | nindent 4 }}
    env:
      - name: WORKSPACE
{{- if or .Values.localRepoHostPath .Values.localRepoPvcName }}
        value: /workspace
{{- else }}
        value: {{ include "aaos-builder.pipelineMonorepoPath" . | quote }}
{{- end }}
      - name: PIPELINE_REPO_ROOT
{{- if or .Values.localRepoHostPath .Values.localRepoPvcName }}
        value: {{ .Values.localRepoMountPath | quote }}
{{- else }}
        value: {{ include "aaos-builder.pipelineMonorepoPath" . | quote }}
{{- end }}
      - name: AAOS_LUNCH_TARGET
        value: '{{ "{{" }}workflow.parameters.lunchTarget{{ "}}" }}'
      - name: AAOS_CLEAN
        value: '{{ "{{" }}workflow.parameters.cleanBuild{{ "}}" }}'
    resources:
      # spec.workflowStepResources.clean
{{- include "aaos-builder.workflowStepResources" (dict "step" "clean" "root" .) | nindent 6 }}
    securityContext:
      privileged: true
{{- if or (eq (include "aaos-builder.useSharedPipelineWorkspaceVolume" .) "true") (or .Values.localRepoHostPath .Values.localRepoPvcName) }}
    volumeMounts:
{{- if eq (include "aaos-builder.useSharedPipelineWorkspaceVolume" .) "true" }}
      - name: pipeline-workspace
        mountPath: {{ include "aaos-builder.pipelineMonorepoPath" . | quote }}
{{- end }}
{{- if or .Values.localRepoHostPath .Values.localRepoPvcName }}
      - name: local-repo
        mountPath: {{ .Values.localRepoMountPath | quote }}
        readOnly: true
{{- end }}
{{- end }}
    args:
      - |
        WORKDIR="${PIPELINE_REPO_ROOT:-{{ include "aaos-builder.pipelineMonorepoPath" . }}}"
        export WORKSPACE="${WORKDIR}"
        AAOS_LUNCH_TARGET="${AAOS_LUNCH_TARGET}" \
        AAOS_CLEAN="${AAOS_CLEAN}" \
        "${WORKDIR}"/workloads/android/pipelines/builds/aaos_builder/aaos_environment.sh
{{- end }}
