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
ABFS build step: initialise, build, AI review, storage, cache sync, in one pod.
Order: 6/10 (replaces init, build, gemini-review and storage).
Dependencies: requires compute-vars output and clean completion.

Why one pod: aaos_initialise.sh starts the cacheman daemon and mounts casfs
over WORKSPACE. The sources, the build output (OUT_DIR=out, which has to stay
on the casfs tree) and the daemon exist only in the pod that ran it. Build,
AI review and storage all read that tree, so they run here, in the order and
under the conditions of the Jenkins job's stages. RESULT plays the role of
currentBuild.currentResult.
*/ -}}

{{- define "aaos-builder.template.abfs-build" -}}
- name: abfs-build
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
                - key: aaos_abfs_pod
                  operator: Exists
            topologyKey: kubernetes.io/hostname
  container:
    image: {{ include "aaos-builder.builderImage" . | quote }}
    workingDir: {{ include "aaos-builder.pipelineMonorepoPath" . | quote }}
    # No -e: a failing stage sets RESULT and later stages decide whether to
    # run, as the Jenkins when-conditions do.
    command: [bash, -uo, pipefail, -c]
{{- include "aaos-builder.cloudEnvFrom" . | nindent 4 }}
    env:
{{- include "aaos-builder.commonEnv" . | nindent 6 }}
{{- include "aaos-builder.androidVersionEnv" . | nindent 6 }}
{{- include "aaos-builder.sdkAndroidVersionEnv" . | nindent 6 }}
      - name: AAOS_GERRIT_MANIFEST_URL
        value: "{{ "{{workflow.parameters.manifestUrl}}" }}"
      - name: AAOS_MANIFEST_NAME
        value: "{{ "{{workflow.parameters.manifestName}}" }}"
      # AI review. Prompt and skills paths in values start with /workspace;
      # the script below maps them onto the pipeline checkout of this pod.
      - name: AI_REVIEW_RUNTIME
        value: '{{ "{{" }}workflow.parameters.aiReviewRuntime{{ "}}" }}'
      - name: GEMINI_PROMPT_FILE
        value: {{ .Values.spec.geminiPromptFile | quote }}
      - name: GEMINI_PROMPT_FILE_2
        value: {{ .Values.spec.geminiPromptFile2 | default "" | quote }}
      - name: GEMINI_PROMPT_FILE_3
        value: {{ .Values.spec.geminiPromptFile3 | default "" | quote }}
      - name: GEMINI_SKILLS_YAML
        value: {{ required "spec.geminiSkillsYaml is required (filesystem path to skills.yaml for Gemini)" .Values.spec.geminiSkillsYaml | quote }}
      - name: GEMINI_LOCATION_GLOBAL
        value: {{ .Values.spec.geminiLocationGlobal | quote }}
      - name: GEMINI_PREVIEW_FEATURES
        value: {{ .Values.spec.geminiPreviewFeatures | quote }}
      - name: GEMINI_COMMAND_LINE
        value: {{ include "aaos-builder.geminiCommandLineResolved" . | quote }}
      - name: GEMINI_AI_EXECUTION_TIMEOUT_MINUTES
        value: {{ .Values.spec.geminiAiExecutionTimeoutMinutes | default "60" | quote }}
      - name: GEMINI_STEP2_PRIOR_CONTEXT_BYTES
        value: "131072"
      # The ABFS workspace: /aaos-cache/abfs when cached, /abfs when not
      # (aaos_environment.sh); the Jenkins job derives it the same way.
      - name: GEMINI_ANALYSIS_PATH
        value: '{{ printf "{{=workflow.parameters.abfsCachedBuild == \"true\" ? \"/aaos-cache/abfs\" : \"/abfs\"}}" }}'
      # Gemini's own output goes to a plain directory on the PVC, outside the
      # casfs mount, where storage picks it up. Jenkins does not set this.
      - name: GEMINI_ARTIFACT_WRITE_ROOT
        value: "/aaos-cache/aaos_builds"
      - name: GEMINI_SKIP_MOVE_ARTIFACTS
        value: "1"
      - name: GEMINI_HOOK_PROFILE
        value: "aaos"
      - name: GEMINI_HOOK_DIR
        value: "workloads/android/pipelines/builds/aaos_builder/hooks"
      # Give the review the whole build log, not the last 2500 lines, as the
      # Jenkins job does (hooks/review-pre-aaos.sh).
      - name: GEMINI_AAOS_LOG_TAIL_MODE
        value: "fullcopy"
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
      # As both Jenkins ABFS pod templates.
      appArmorProfile:
        type: Unconfined
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
        REPO="${PIPELINE_REPO_ROOT:-{{ include "aaos-builder.pipelineMonorepoPath" . }}}"
        SCRIPTS="${REPO}/workloads/android/pipelines/builds/aaos_builder"
        export WORKSPACE="${REPO}"
        RESULT=SUCCESS
        stage() { echo; echo "=================== $* ==================="; }

        stage "Initialise"
        "${SCRIPTS}/aaos_initialise.sh" || RESULT=FAILURE

        if [ "${RESULT}" = "SUCCESS" ]; then
          stage "Build"
          "${SCRIPTS}/aaos_build.sh" || RESULT=FAILURE
        fi

        if [ "${RESULT}" = "FAILURE" ] && [ "${ENABLE_GEMINI_AI_ASSISTANT:-false}" = "true" ]; then
          stage "AI Review"
          (
            export PIPELINE_REPO_ROOT="${REPO}"
            export GEMINI_PROMPT_FILE="${GEMINI_PROMPT_FILE/#\/workspace/${REPO}}"
            export GEMINI_PROMPT_FILE_2="${GEMINI_PROMPT_FILE_2/#\/workspace/${REPO}}"
            export GEMINI_PROMPT_FILE_3="${GEMINI_PROMPT_FILE_3/#\/workspace/${REPO}}"
            export GEMINI_SKILLS_YAML="${GEMINI_SKILLS_YAML/#\/workspace/${REPO}}"
            bash "${REPO}/workloads/common/agentic-ai/gemini/run_ai_review.sh"
          ) || echo "AI review failed; the result is already FAILURE"
        fi

        if [ -n "${AAOS_ARTIFACT_STORAGE_SOLUTION:-}" ] && { [ "${RESULT}" = "SUCCESS" ] || [ "${ENABLE_GEMINI_AI_ASSISTANT:-false}" = "true" ]; }; then
          stage "Storage"
          if [ "${RESULT}" = "SUCCESS" ]; then
            export AAOS_BUILD_STAGE_FAILED=false
            "${SCRIPTS}/aaos_avd_sdk.sh" || true
          else
            export AAOS_BUILD_STAGE_FAILED=true
          fi
          # Storage path: <date>-<time>_<workflow.name>
          export STORAGE_BUCKET_DESTINATION="${STORAGE_BUCKET_BASE}/$(date +%Y-%m-%d-%H%M%S)_${WORKFLOW_NAME}"
          "${SCRIPTS}/aaos_storage.sh" || RESULT=FAILURE
        fi

        # Cache Sync: let cacheman finish uploading to the cache before the pod
        # and its daemon go away. Same command and semantics as the Jenkins
        # stage: a failed wait does not fail the build, a failed sync does.
        # Like Jenkins, only when a lunch target was built.
        if [ "${RESULT}" = "SUCCESS" ] && [ -n "${AAOS_LUNCH_TARGET:-}" ] && [ "${ABFS_CACHEMAN_TIMEOUT:-0}" != "0" ]; then
          stage "Cache Sync"
          echo "Waiting on cacheman to sync ..."
          abfs cacheman wait -t "${ABFS_CACHEMAN_TIMEOUT}"
          echo "abfs cacheman wait exit code: $? (not fatal)"
          sync || RESULT=FAILURE
          echo "Sync complete"
        fi

        stage "Result: ${RESULT}"
        [ "${RESULT}" = "SUCCESS" ]
{{- end }}
