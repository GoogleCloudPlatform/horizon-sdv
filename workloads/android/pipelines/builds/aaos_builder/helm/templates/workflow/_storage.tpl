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
Artifact collection step for aaos-builder.
Order: 10/10.
Dependencies: runs after build tasks and optional gemini-review.
Storage maps workflow.parameters.enableGeminiAiAssistant → ENABLE_GEMINI_AI_ASSISTANT
(via commonEnv) so aaos_environment.sh / aaos_storage.sh can derive Gemini artifacts.
*/ -}}

{{- define "aaos-builder.template.storage" -}}
- name: storage
  # Keep storage pod on the same node as other workflow pods that use the cache PVC.
  podSpecPatch: |
{{- include "aaos-builder.podAffinitySameWorkflow" . | nindent 4 }}
  inputs:
    parameters:
      - name: sdkAndroidVersion
      - name: runAvdSdk
        default: "true"
        description: "Run aaos_avd_sdk.sh (only when build succeeded)."
      - name: buildStageFailed
        default: "false"
        description: "When true, aaos_environment.sh skips OUT_DIR artifacts."
{{- if eq (include "aaos-builder.usePerPodGitArtifact" .) "true" }}
    artifacts:
      - name: pipeline-repo
        path: /workspace
        git:
          repo: {{ .Values.spec.pipelineRepoUrl | quote }}
          revision: {{ .Values.spec.pipelineRepoRevision | quote }}
{{- include "aaos-builder.gitArtifactCredsContent" . | nindent 10 }}
{{- end }}
  # outputs:
  #   artifacts:
  #     # Requires an Argo artifact repository to be configured.
  #     - name: build-info
  #       path: /aaos-cache/aaos_builds/aaos-build-info.txt
  #     - name: build-log
  #       path: /aaos-cache/aaos_builds/aaos-build.log
  container:
    image: {{ include "aaos-builder.builderImage" . | quote }}
{{- if or .Values.localRepoHostPath .Values.localRepoPvcName }}
    workingDir: {{ .Values.localRepoMountPath | quote }}
{{- else }}
    workingDir: {{ include "aaos-builder.pipelineMonorepoPath" . | quote }}
{{- end }}
    command: [bash, -euo, pipefail, -c]
{{- include "aaos-builder.cloudEnvFrom" . | nindent 4 }}
    env:
      # commonEnv includes ENABLE_GEMINI_AI_ASSISTANT from workflow.parameters.enableGeminiAiAssistant
{{- include "aaos-builder.commonEnv" . | nindent 6 }}
{{- include "aaos-builder.androidVersionEnv" . | nindent 6 }}
{{- include "aaos-builder.sdkAndroidVersionEnv" . | nindent 6 }}
      - name: RUN_AVD_SDK
        value: "{{ "{{" }}inputs.parameters.runAvdSdk{{ "}}" }}"
      - name: AAOS_BUILD_STAGE_FAILED
        value: "{{ "{{" }}inputs.parameters.buildStageFailed{{ "}}" }}"
    resources:
      # spec.workflowStepResources.storage
{{- include "aaos-builder.workflowStepResources" (dict "step" "storage" "root" .) | nindent 6 }}
    securityContext:
      privileged: true
    volumeMounts:
      - name: aaos-cache
        mountPath: /aaos-cache
{{- if eq (include "aaos-builder.aospMirrorEnabled" .) "true" }}
{{- include "aaos-builder.aospMirrorMount" . | nindent 6 }}
{{- end }}
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
        # 1. Storage path: <date>-<time>_<workflow.name> (e.g. 2026-02-27-1110425_aaos-builder-xyzs)
        export STORAGE_BUCKET_DESTINATION="${STORAGE_BUCKET_BASE}/$(date +%Y-%m-%d-%H%M%S)_${WORKFLOW_NAME}"

        # 2. Storage application (skip aaos_avd_sdk on build failure)
        WORKDIR="${PIPELINE_REPO_ROOT:-{{ include "aaos-builder.pipelineMonorepoPath" . }}}"
        export WORKSPACE="${WORKDIR}"
        if [ "${RUN_AVD_SDK:-true}" = "true" ]; then
          "${WORKDIR}"/workloads/android/pipelines/builds/aaos_builder/aaos_avd_sdk.sh || true
        fi
        "${WORKDIR}"/workloads/android/pipelines/builds/aaos_builder/aaos_storage.sh
{{- end }}
