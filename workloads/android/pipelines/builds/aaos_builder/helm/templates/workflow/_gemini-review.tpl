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
Inline Gemini review step (runs workloads/common/agentic-ai/gemini/run_ai_review.sh).
Same pod volume wiring as build; env matches Jenkins AAOS Gemini review.
*/ -}}

{{- define "aaos-builder.template.gemini-review" -}}
- name: gemini-review
  inputs:
    artifacts:
      - name: pipeline-repo
        path: /workspace
        optional: true
  podSpecPatch: |
{{- include "aaos-builder.podAffinitySameWorkflow" . | nindent 4 }}
  container:
    image: {{ include "aaos-builder.builderImage" . | quote }}
{{- if or .Values.localRepoHostPath .Values.localRepoPvcName }}
    workingDir: {{ .Values.localRepoMountPath | quote }}
{{- else }}
    workingDir: "/workspace"
{{- end }}
    command: [bash, -euo, pipefail, -c]
{{- include "aaos-builder.cloudEnvFrom" . | nindent 4 }}
    env:
{{- include "aaos-builder.commonEnv" . | nindent 6 }}
{{- if not (or .Values.localRepoHostPath .Values.localRepoPvcName) }}
      # Per-step git artifact uses /workspace (not /horizon shared pipeline-workspace checkout).
      - name: PIPELINE_REPO_ROOT
        value: "/workspace"
      - name: WORKSPACE
        value: "/workspace"
{{- end }}
      - name: AI_REVIEW_RUNTIME
        value: {{ .Values.spec.aiReviewRuntime | default "gemini-cli" | quote }}
      - name: GEMINI_PROMPT_FILE
{{- if and (or .Values.localRepoHostPath .Values.localRepoPvcName) (hasPrefix "/workspace" .Values.spec.geminiPromptFile) }}
        value: {{ printf "%s%s" .Values.localRepoMountPath (trimPrefix "/workspace" .Values.spec.geminiPromptFile) | quote }}
{{- else }}
        value: {{ .Values.spec.geminiPromptFile | quote }}
{{- end }}
      - name: GEMINI_PROMPT_FILE_2
{{- if and (or .Values.localRepoHostPath .Values.localRepoPvcName) (hasPrefix "/workspace" (.Values.spec.geminiPromptFile2 | default "")) }}
        value: {{ printf "%s%s" .Values.localRepoMountPath (trimPrefix "/workspace" .Values.spec.geminiPromptFile2) | quote }}
{{- else }}
        value: {{ .Values.spec.geminiPromptFile2 | default "" | quote }}
{{- end }}
      - name: GEMINI_PROMPT_FILE_3
{{- if and (or .Values.localRepoHostPath .Values.localRepoPvcName) (hasPrefix "/workspace" (.Values.spec.geminiPromptFile3 | default "")) }}
        value: {{ printf "%s%s" .Values.localRepoMountPath (trimPrefix "/workspace" .Values.spec.geminiPromptFile3) | quote }}
{{- else }}
        value: {{ .Values.spec.geminiPromptFile3 | default "" | quote }}
{{- end }}
      - name: GEMINI_LOCATION_GLOBAL
        value: {{ .Values.spec.geminiLocationGlobal | quote }}
      - name: GEMINI_PREVIEW_FEATURES
        value: {{ .Values.spec.geminiPreviewFeatures | quote }}
      - name: GEMINI_COMMAND_LINE
        value: {{ include "aaos-builder.geminiCommandLineResolved" . | quote }}
      - name: GEMINI_AI_EXECUTION_TIMEOUT_MINUTES
        value: {{ .Values.spec.geminiAiExecutionTimeoutMinutes | default "60" | quote }}
{{- $geminiSkillsYaml := required "spec.geminiSkillsYaml is required (filesystem path to skills.yaml for Gemini)" .Values.spec.geminiSkillsYaml }}
      - name: GEMINI_SKILLS_YAML
{{- if and (or .Values.localRepoHostPath .Values.localRepoPvcName) (hasPrefix "/workspace" $geminiSkillsYaml) }}
        value: {{ printf "%s%s" .Values.localRepoMountPath (trimPrefix "/workspace" $geminiSkillsYaml) | quote }}
{{- else }}
        value: {{ $geminiSkillsYaml | quote }}
{{- end }}
      - name: GEMINI_STEP2_PRIOR_CONTEXT_BYTES
        value: "131072"
      - name: GEMINI_ANALYSIS_PATH
        value: "/aaos-cache/aaos_builds"
      # Write gemini-assist/ and CLI JSON on the workflow PVC so storage uploads them (not /workspace git clone).
      - name: GEMINI_ARTIFACT_WRITE_ROOT
        value: "/aaos-cache/aaos_builds"
      - name: GEMINI_SKIP_MOVE_ARTIFACTS
        value: "1"
      - name: GEMINI_HOOK_PROFILE
        value: "aaos"
      - name: GEMINI_HOOK_DIR
        value: "workloads/android/pipelines/builds/aaos_builder/hooks"
    resources:
{{- include "aaos-builder.workflowStepResources" (dict "root" . "step" "geminiReview") | nindent 6 }}
    securityContext:
      privileged: true
    volumeMounts:
      - name: aaos-cache
        mountPath: /aaos-cache
{{- if or .Values.localRepoHostPath .Values.localRepoPvcName }}
      - name: local-repo
        mountPath: {{ .Values.localRepoMountPath | quote }}
        readOnly: true
{{- end }}
    args:
      - bash "${PIPELINE_REPO_ROOT}/workloads/common/agentic-ai/gemini/run_ai_review.sh"
{{- end }}
