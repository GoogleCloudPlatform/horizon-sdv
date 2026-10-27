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
  Argo templates for CTS Execution Gemini review (no cluster WorkflowTemplateRef).
  prepare-gemini-cts → gemini_argo_prepare_staging.sh on PVC; gemini-review → run_ai_review.sh.
*/ -}}

{{- define "cts-execution.template.prepare-gemini-cts" -}}
{{- $root := . }}
{{- $mono := include "cts-execution.pipelineMonorepoPath" $root }}
    - name: prepare-gemini-cts
{{- include "cts-execution.pipelineWorkspacePodSpecPatch" $root }}
      inputs:
        parameters:
          - name: buildNumber
          - name: testResultsStagingGcsUri
        artifacts:
{{- if eq (include "cts-execution.usePerPodGitArtifact" $root) "true" }}
{{ include "cts-execution.pipelineRepoGitArtifactItem" $root | nindent 10 }}
{{- end }}
          - name: cvd-argo-artifacts
            path: /tmp/cvd-argo-artifacts
            optional: true
      outputs:
        parameters:
          - name: skipGemini
            valueFrom:
              path: /tmp/gemini-skip.flag
{{- if or $root.Values.spec.runPodLabels $root.Values.spec.runPodAnnotations }}
      metadata:
{{- if $root.Values.spec.runPodLabels }}
        labels:
{{- toYaml $root.Values.spec.runPodLabels | nindent 10 }}
{{- end }}
{{- if $root.Values.spec.runPodAnnotations }}
        annotations:
{{- toYaml $root.Values.spec.runPodAnnotations | nindent 10 }}
{{- end }}
{{- end }}
{{- if $root.Values.spec.nodeSelector }}
      nodeSelector:
{{- toYaml $root.Values.spec.nodeSelector | nindent 8 }}
{{- end }}
{{- if $root.Values.spec.tolerations }}
      tolerations:
{{- toYaml $root.Values.spec.tolerations | nindent 8 }}
{{- end }}
      container:
        image: {{ include "cts-execution.builderImage" $root | quote }}
        imagePullPolicy: Always
        workingDir: {{ $mono | quote }}
{{- include "cts-execution.cloudEnvFrom" $root | nindent 8 }}
        volumeMounts:
{{- include "cts-execution.pipelineWorkspaceVolumeMounts" $root }}
          - name: gemini-test-results
            mountPath: /workspace/test-results
        resources:
{{- if $root.Values.spec.geminiPodResources }}
{{- toYaml $root.Values.spec.geminiPodResources | nindent 10 }}
{{- else }}
          requests:
            cpu: "16000m"
            memory: "48Gi"
          limits:
            cpu: "32000m"
            memory: "96Gi"
{{- end }}
        command: [bash, -euo, pipefail, -c]
        args:
          - |
            export WORKSPACE={{ $mono | quote }}
            export GEMINI_TEST_RESULTS_DIR=/workspace/test-results
            export TEST_RESULTS_STAGING_GCS_URI='{{ "{{" }}inputs.parameters.testResultsStagingGcsUri{{ "}}" }}'
            export BUILD_NUMBER='{{ "{{" }}inputs.parameters.buildNumber{{ "}}" }}'
            export GEMINI_PREPARE_MODE=cts
            bash "${WORKSPACE}/workloads/android/pipelines/tests/gemini_argo_prepare_staging.sh"
{{- end }}

{{- define "cts-execution.template.gemini-review" -}}
{{- $root := . }}
    - name: gemini-review
      inputs:
        parameters:
          - name: storageBucketDestination
        artifacts:
          - name: pipeline-repo
            path: /workspace
            optional: true
      podSpecPatch: |
{{ include "cts-execution.podAffinitySameWorkflow" $root | nindent 8 }}
{{- if or $root.Values.spec.runPodLabels $root.Values.spec.runPodAnnotations }}
      metadata:
{{- if $root.Values.spec.runPodLabels }}
        labels:
{{- toYaml $root.Values.spec.runPodLabels | nindent 10 }}
{{- end }}
{{- if $root.Values.spec.runPodAnnotations }}
        annotations:
{{- toYaml $root.Values.spec.runPodAnnotations | nindent 10 }}
{{- end }}
{{- end }}
{{- if $root.Values.spec.nodeSelector }}
      nodeSelector:
{{- toYaml $root.Values.spec.nodeSelector | nindent 8 }}
{{- end }}
{{- if $root.Values.spec.tolerations }}
      tolerations:
{{- toYaml $root.Values.spec.tolerations | nindent 8 }}
{{- end }}
      container:
        image: {{ include "cts-execution.builderImage" $root | quote }}
        imagePullPolicy: Always
        workingDir: /workspace
        command: [bash, -euo, pipefail, -c]
{{- include "cts-execution.cloudEnvFrom" $root | nindent 8 }}
        env:
          - name: PIPELINE_REPO_ROOT
            value: "/workspace"
          - name: WORKSPACE
            value: "/workspace"
          - name: AI_REVIEW_RUNTIME
            value: '{{ "{{" }}workflow.parameters.aiReviewRuntime{{ "}}" }}'
          - name: JOB_NAME
            value: '{{ "{{" }}workflow.name{{ "}}" }}'
          - name: BUILD_NUMBER
            value: '{{ "{{" }}workflow.uid{{ "}}" }}'
          - name: GEMINI_ANALYSIS_PATH
            value: "/workspace/test-results"
          - name: ANALYSIS_WORKING_DIRECTORY
            value: "/workspace/test-results"
          - name: GEMINI_SKIP_MOVE_ARTIFACTS
            value: "1"
          - name: GEMINI_PROMPT_FILE
            value: "/workspace/workloads/android/pipelines/tests/cts_execution/prompt/sequenced/step1_triage.txt"
          - name: GEMINI_PROMPT_FILE_2
            value: "/workspace/workloads/android/pipelines/tests/cts_execution/prompt/sequenced/step2_rca.txt"
          - name: GEMINI_PROMPT_FILE_3
            value: "/workspace/workloads/android/pipelines/tests/cts_execution/prompt/sequenced/step3_fixes.txt"
          - name: GEMINI_SKILLS_YAML
            value: "/workspace/workloads/android/pipelines/tests/cts_execution/prompt/sequenced/skills.yaml"
          - name: GEMINI_LOCATION_GLOBAL
            value: {{ $root.Values.spec.geminiLocationGlobal | default "true" | quote }}
          - name: GEMINI_PREVIEW_FEATURES
            value: {{ $root.Values.spec.geminiPreviewFeatures | default "true" | quote }}
          - name: GEMINI_COMMAND_LINE
            value: {{ include "cts-execution.geminiCommandLineResolved" $root | quote }}
          - name: GEMINI_AI_EXECUTION_TIMEOUT_MINUTES
            value: {{ $root.Values.spec.geminiAiExecutionTimeoutMinutes | default "60" | quote }}
          - name: GEMINI_STEP2_PRIOR_CONTEXT_BYTES
            value: {{ $root.Values.spec.geminiStep2PriorContextBytes | default "131072" | quote }}
          - name: GEMINI_ADDITIONAL_ARTIFACTS
            value: "/workspace/test-results/"
          - name: GEMINI_HOOK_PROFILE
            value: "cts"
          - name: GEMINI_HOOK_DIR
            value: "workloads/android/pipelines/tests/cts_execution/hooks"
          - name: GEMINI_ARTIFACT_ROOT_NAME
            value: {{ if $root.Values.spec.geminiArtifactRootName }}{{ $root.Values.spec.geminiArtifactRootName | quote }}{{ else if $root.Values.spec.cloudProject }}{{ printf "%s-aaos" $root.Values.spec.cloudProject | quote }}{{ else }}{{ "" | quote }}{{ end }}
          - name: GEMINI_ARTIFACT_STORAGE_SOLUTION
            value: '{{ "{{" }}workflow.parameters.ctsArtifactStorageSolution{{ "}}" }}'
          - name: GEMINI_STORAGE_BUCKET_DESTINATION
            value: '{{ "{{" }}inputs.parameters.storageBucketDestination{{ "}}" }}'
          - name: STORAGE_LABELS
            value: '{{ "{{" }}workflow.parameters.storageLabels{{ "}}" }}'
{{- if not $root.Values.cloudEnvConfigMapName }}
          - name: CLOUD_PROJECT
            value: {{ $root.Values.spec.cloudProject | quote }}
          - name: CLOUD_REGION
            value: {{ $root.Values.spec.cloudRegion | quote }}
{{- end }}
        resources:
{{- if $root.Values.spec.geminiPodResources }}
{{- toYaml $root.Values.spec.geminiPodResources | nindent 10 }}
{{- else }}
          requests:
            cpu: "16000m"
            memory: "48Gi"
          limits:
            cpu: "32000m"
            memory: "96Gi"
{{- end }}
        securityContext:
          privileged: true
        volumeMounts:
          - name: gemini-test-results
            mountPath: /workspace/test-results
        args:
          - bash "${PIPELINE_REPO_ROOT}/workloads/common/agentic-ai/gemini/run_ai_review.sh"
{{- end }}
