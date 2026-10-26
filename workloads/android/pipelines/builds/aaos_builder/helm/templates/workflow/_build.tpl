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
Build step for AAOS builder.
Order: 7/10.
Dependencies: requires init and compute-vars output.
*/ -}}

{{- define "aaos-builder.template.build" -}}
- name: build
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
  # Enforce one build pod per node.
  podSpecPatch: |
    affinity:
      podAntiAffinity:
        requiredDuringSchedulingIgnoredDuringExecution:
          - labelSelector:
              matchExpressions:
                - key: aaos_pod
                  operator: Exists
            topologyKey: kubernetes.io/hostname
  container:
    image: {{ include "aaos-builder.builderImage" . | quote }}
    workingDir: {{ include "aaos-builder.pipelineMonorepoPath" . | quote }}
    command: [bash, -euo, pipefail, -c]
{{- include "aaos-builder.cloudEnvFrom" . | nindent 4 }}
    env:
{{- include "aaos-builder.commonEnv" . | nindent 6 }}
{{- include "aaos-builder.androidVersionEnv" . | nindent 6 }}
      - name: GEMINI_PROMPT_FILE
        value: {{ printf "%s/workloads/android/pipelines/builds/aaos_builder/prompt/build_prompt_single.txt" (include "aaos-builder.pipelineMonorepoPath" .) | quote }}
      - name: GEMINI_PREVIEW_FEATURES
        value: {{ .Values.spec.geminiPreviewFeatures | quote }}
      - name: GEMINI_LOCATION_GLOBAL
        value: {{ .Values.spec.geminiLocationGlobal | quote }}
      - name: GEMINI_COMMAND_LINE
        value: {{ include "aaos-builder.geminiCommandLineResolved" . | quote }}
      - name: GEMINI_AI_EXECUTION_TIMEOUT_HOURS
        value: {{ .Values.spec.geminiAiExecutionTimeoutHours | quote }}
    resources:
      # Build step sizing for c2d-highcpu-112 (leave headroom for system/Argo).
      limits:
        cpu: "104000m"
        memory: "200000Mi"
      requests:
        cpu: "90000m"
        memory: "180000Mi"
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
        "${WORKDIR}"/workloads/android/pipelines/builds/aaos_builder/aaos_build.sh
{{- end }}
