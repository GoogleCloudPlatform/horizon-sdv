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
Templates of openbsw-bsw-builder (port of workloads/openbsw/pipelines/builds/bsw_builder/Jenkinsfile).

  main                  DAG below
  check-image           does <openbsw-builder>:<imageTag> exist (or forceImageBuild)?
  build-image           openbsw-builder-image build-defaults, only when needed
  fetch-pipeline        Horizon checkout onto the bsw-workspace volume
  build                 Jenkins stages Initialise .. Build NXP S32K148 Target
  ai-review             on build failure: workloads/common/agentic-ai/gemini/run_ai_review.sh (agy by default)
  storage               bsw_storage.sh
  retain                optional sleep with the workspace mounted (Jenkins "Retain instance")
  fail-if-build-failed  marks the workflow Failed after storage when the build failed

The bsw-workspace volume holds /horizon (checkout; Jenkins WORKSPACE, where logs and AI review output
are written) and /home/builder/bsw-builds (bsw_environment.sh build directory with the OpenBSW clone).
*/ -}}

{{- define "openbsw.bswBuilder.workspacePath" -}}/horizon{{- end -}}

{{- define "openbsw.bswBuilder.image" -}}
{{ include "openbsw.builderImageRepository" . }}:{{ "{{" }}workflow.parameters.imageTag{{ "}}" }}
{{- end -}}

{{/*
Env shared by build and storage. bsw_environment.sh derives the artifact list from the same flags,
so storage needs them too.
*/}}
{{- define "openbsw.bswBuilder.env" -}}
- name: WORKSPACE
  value: {{ include "openbsw.bswBuilder.workspacePath" . | quote }}
- name: PIPELINE_REPO_ROOT
  value: {{ include "openbsw.bswBuilder.workspacePath" . | quote }}
- name: JOB_NAME
  value: {{ .Values.bswBuilder.storageBuildsDir | quote }}
- name: WORKFLOW_NAME
  value: '{{ "{{" }}workflow.name{{ "}}" }}'
{{- $env := list
  (list "OPENBSW_GIT_URL" "openbswGitUrl")
  (list "OPENBSW_GIT_BRANCH" "openbswGitBranch")
  (list "POST_GIT_CLONE_COMMAND" "postGitCloneCommand")
  (list "RTOS_PLATFORM" "rtosPlatform")
  (list "BUILD_CONFIG" "buildConfig")
  (list "TOOLCHAIN" "toolchain")
  (list "CMAKE_SYNC_JOBS" "cmakeSyncJobs")
  (list "BUILD_DOCUMENTATION" "buildDocumentation")
  (list "CODE_COVERAGE" "codeCoverage")
  (list "LIST_UNIT_TESTS" "listUnitTests")
  (list "LIST_UNIT_TESTS_CMDLINE" "listUnitTestsCmdline")
  (list "BUILD_UNIT_TESTS" "buildUnitTests")
  (list "UNIT_TEST_TARGET" "unitTestTarget")
  (list "UNIT_TESTS_CMDLINE" "unitTestsCmdline")
  (list "RUN_UNIT_TESTS" "runUnitTests")
  (list "RUN_UNIT_TESTS_CMDLINE" "runUnitTestsCmdline")
  (list "BUILD_POSIX" "buildPosix")
  (list "POSIX_BUILD_CMDLINE" "posixBuildCmdline")
  (list "POSIX_ARTIFACT" "posixArtifact")
  (list "POSIX_PYTEST" "posixPytest")
  (list "POSIX_PYTEST_CMDLINE" "posixPytestCmdline")
  (list "BUILD_NXP_S32K148" "buildNxpS32k148")
  (list "NXP_S32K148_BUILD_CMDLINE" "nxpS32k148BuildCmdline")
  (list "NXP_S32K148_ARTIFACT" "nxpS32k148Artifact")
  (list "ENABLE_GEMINI_AI_ASSISTANT" "enableGeminiAiAssistant")
  (list "OPENBSW_ARTIFACT_STORAGE_SOLUTION" "artifactStorageSolution")
  (list "STORAGE_BUCKET_DESTINATION" "storageBucketDestination")
  (list "STORAGE_LABELS" "storageLabels")
}}
{{- range $env }}
- name: {{ index . 0 }}
  value: '{{ "{{" }}workflow.parameters.{{ index . 1 }}{{ "}}" }}'
{{- end }}
{{- end -}}

{{/*
bsw-workspace mounts for the builder image (USER builder, HOME /home/builder).
*/}}
{{- define "openbsw.bswBuilder.workspaceMounts" -}}
- name: bsw-workspace
  subPath: horizon
  mountPath: {{ include "openbsw.bswBuilder.workspacePath" . | quote }}
- name: bsw-workspace
  subPath: bsw-builds
  mountPath: /home/builder/bsw-builds
{{- end -}}

{{- define "openbsw.bswBuilder.template.main" -}}
{{- $creds := eq (include "openbsw.useUmbrellaCreds" .) "true" -}}
- name: main
  dag:
    tasks:
{{- if $creds }}
{{- include "openbsw.prepareGitCredsTask" . | nindent 6 }}
{{- end }}
      - name: check-image
        template: check-image
      - name: build-image
        templateRef:
          name: {{ .Values.builderImage.workflowTemplateName | quote }}
          template: build-defaults
        depends: check-image.Succeeded
        when: >-
          '{{ "{{" }}tasks.check-image.outputs.parameters.shouldBuild{{ "}}" }}' == "true"
        arguments:
          parameters:
            - name: imageTag
              value: '{{ "{{" }}workflow.parameters.imageTag{{ "}}" }}'
      - name: fetch-pipeline
        template: fetch-pipeline
        depends: >-
{{- if $creds }}
          prepare-pipeline-git-creds.Succeeded &&
{{- end }}
          (build-image.Succeeded || build-image.Skipped)
      - name: build
        template: build
        depends: fetch-pipeline.Succeeded
      - name: ai-review
        template: ai-review
        depends: build.Failed || build.Errored
        when: >-
          '{{ "{{" }}workflow.parameters.enableGeminiAiAssistant{{ "}}" }}' == "true"
      - name: storage
        template: storage
        depends: >-
          build.Succeeded ||
          ai-review.Succeeded ||
          ai-review.Failed ||
          ai-review.Errored ||
          ai-review.Skipped
        when: >-
          '{{ "{{" }}workflow.parameters.artifactStorageSolution{{ "}}" }}' != ""
      - name: retain
        template: retain
        depends: >-
          storage.Succeeded ||
          storage.Failed ||
          storage.Errored ||
          storage.Skipped
        when: >-
          '{{ "{{" }}workflow.parameters.instanceRetentionTime{{ "}}" }}' != "0"
      - name: fail-if-build-failed
        # Without this the workflow ends Succeeded, because ai-review and storage handle build.Failed.
        template: fail-workflow
        depends: >-
          retain.Succeeded ||
          retain.Failed ||
          retain.Errored ||
          retain.Skipped
        when: >-
          '{{ "{{" }}tasks.build.status{{ "}}" }}' == "Failed" ||
          '{{ "{{" }}tasks.build.status{{ "}}" }}' == "Error"
{{- end -}}

{{- define "openbsw.bswBuilder.template.check-image" -}}
- name: check-image
  script:
    # cloud-sdk image, so the check runs before the builder image exists.
    image: "gcr.io/google.com/cloudsdktool/cloud-sdk:slim"
    command: [bash, -euo, pipefail]
    env:
      - name: IMAGE
        value: {{ include "openbsw.bswBuilder.image" . | quote }}
      - name: CLOUD_PROJECT
        value: {{ .Values.cloud.project | quote }}
      - name: FORCE_IMAGE_BUILD
        value: '{{ "{{" }}workflow.parameters.forceImageBuild{{ "}}" }}'
    source: |
      if [ "${FORCE_IMAGE_BUILD}" = "true" ]; then
        echo "forceImageBuild=true: rebuilding ${IMAGE}"
        echo "true" > /tmp/should_build
        exit 0
      fi
      if gcloud --project="${CLOUD_PROJECT}" artifacts docker images describe "${IMAGE}" >/tmp/image_check.out 2>&1; then
        echo "Found ${IMAGE}"
        echo "false" > /tmp/should_build
      else
        echo "Image not found (or check failed), building it: ${IMAGE}"
        cat /tmp/image_check.out || true
        echo "true" > /tmp/should_build
      fi
  outputs:
    parameters:
      - name: shouldBuild
        valueFrom:
          path: /tmp/should_build
{{- end -}}

{{- define "openbsw.bswBuilder.template.fetch-pipeline" -}}
- name: fetch-pipeline
  podSpecPatch: |
{{- include "openbsw.podAffinitySameWorkflow" . | nindent 4 }}
  inputs:
    artifacts:
      - name: pipeline-repo
        # Argo rejects a git artifact path that is also a volume mount path.
        path: /tmp/pipeline-repo-staging
{{- include "openbsw.pipelineRepoGitArtifact" . | nindent 8 }}
  container:
    image: {{ include "openbsw.bswBuilder.image" . | quote }}
    workingDir: /tmp
    command: [bash, -euo, pipefail, -c]
    args:
      - |
        mkdir -p /bsw-workspace/horizon /bsw-workspace/bsw-builds
        cp -a /tmp/pipeline-repo-staging/. /bsw-workspace/horizon/
        chown -R 1000:1000 /bsw-workspace/horizon /bsw-workspace/bsw-builds
        test -d /bsw-workspace/horizon/workloads/openbsw
    resources:
{{- include "openbsw.stepResources" (dict "root" . "step" "fetchPipeline") | nindent 6 }}
    securityContext:
      # The new volume is root-owned; later steps run as builder (uid 1000).
      runAsUser: 0
      runAsGroup: 0
    volumeMounts:
      - name: bsw-workspace
        mountPath: /bsw-workspace
{{- end -}}

{{- define "openbsw.bswBuilder.template.build" -}}
- name: build
  podSpecPatch: |
{{- include "openbsw.podAffinitySameWorkflow" . | nindent 4 }}
  container:
    image: {{ include "openbsw.bswBuilder.image" . | quote }}
    workingDir: {{ include "openbsw.bswBuilder.workspacePath" . | quote }}
    command: [bash, -euo, pipefail, -c]
{{- include "openbsw.cloudEnvFrom" . | nindent 4 }}
    env:
{{- include "openbsw.bswBuilder.env" . | nindent 6 }}
    resources:
{{- include "openbsw.stepResources" (dict "root" . "step" "build") | nindent 6 }}
    securityContext:
      # POSIX pyTest brings up vcan0 and a tap interface.
      privileged: true
    volumeMounts:
{{- include "openbsw.bswBuilder.workspaceMounts" . | nindent 6 }}
      - name: host-lib-modules
        mountPath: /lib/modules
        readOnly: true
    args:
      - |
        scripts="${PIPELINE_REPO_ROOT}/workloads/openbsw/pipelines/builds/bsw_builder"

        echo "===== Stage: Initialise ====="
        if ! "${scripts}/bsw_initialise.sh"; then
          echo "===== Stage failed: Initialise ====="
          exit 1
        fi

        # Same order and conditions as the Jenkins stages. Each stage runs bsw_build.sh with only
        # its own flag set, because bsw_build.sh runs every enabled step plus its post-build commands.
        failed=0
        stage() {
          local name="$1"
          shift
          echo "===== Stage: ${name} ====="
          if ! env BUILD_DOCUMENTATION=false LIST_UNIT_TESTS=false BUILD_UNIT_TESTS=false \
              RUN_UNIT_TESTS=false CODE_COVERAGE=false BUILD_POSIX=false POSIX_PYTEST=false \
              BUILD_NXP_S32K148=false "$@" "${scripts}/bsw_build.sh"; then
            echo "===== Stage failed: ${name} ====="
            failed=1
          fi
        }
        # Jenkins skips most stages once an earlier one failed (currentBuild.currentResult).
        passed() { [ "${failed}" -eq 0 ]; }

        if [ "${BUILD_DOCUMENTATION}" = "true" ] && passed; then
          stage "Build Documentation" BUILD_DOCUMENTATION=true
        fi
        if [ "${LIST_UNIT_TESTS}" = "true" ]; then
          stage "List Unit Tests" LIST_UNIT_TESTS=true
        fi
        if [ "${BUILD_UNIT_TESTS}" = "true" ]; then
          stage "Build Unit Tests" BUILD_UNIT_TESTS=true
        fi
        if [ "${BUILD_UNIT_TESTS}" = "true" ] && [ "${RUN_UNIT_TESTS}" = "true" ] && passed; then
          stage "Run Unit Tests" RUN_UNIT_TESTS=true
        fi
        if [ "${BUILD_UNIT_TESTS}" = "true" ] && [ "${RUN_UNIT_TESTS}" = "true" ] && [ "${CODE_COVERAGE}" = "true" ] && passed; then
          stage "Code Coverage" CODE_COVERAGE=true
        fi
        if [ "${BUILD_POSIX}" = "true" ] && passed; then
          stage "Build POSIX Target" BUILD_POSIX=true
        fi
        if [ "${BUILD_POSIX}" = "true" ] && [ "${POSIX_PYTEST}" = "true" ] && passed; then
          stage "Run POSIX pyTest" POSIX_PYTEST=true
        fi
        if [ "${BUILD_NXP_S32K148}" = "true" ] && passed; then
          stage "Build NXP S32K148 Target" BUILD_NXP_S32K148=true
        fi
        exit "${failed}"
{{- end -}}

{{- define "openbsw.bswBuilder.template.ai-review" -}}
{{- $ws := include "openbsw.bswBuilder.workspacePath" . -}}
{{- $prompts := printf "%s/workloads/openbsw/pipelines/builds/bsw_builder/prompt/sequenced" $ws -}}
- name: ai-review
  podSpecPatch: |
{{- include "openbsw.podAffinitySameWorkflow" . | nindent 4 }}
  container:
    image: {{ include "openbsw.bswBuilder.image" . | quote }}
    workingDir: {{ $ws | quote }}
    command: [bash, -euo, pipefail, -c]
{{- include "openbsw.cloudEnvFrom" . | nindent 4 }}
    env:
      # Same settings as the Jenkins "AI Review" stage: the workspace root holds bsw-build.log,
      # pytest_result.txt and receives gemini-assist/ and traces/ for storage.
      - name: WORKSPACE
        value: {{ $ws | quote }}
      - name: PIPELINE_REPO_ROOT
        value: {{ $ws | quote }}
      - name: GEMINI_ANALYSIS_PATH
        value: {{ $ws | quote }}
      - name: GEMINI_SKIP_MOVE_ARTIFACTS
        value: "1"
      - name: AI_REVIEW_RUNTIME
        value: '{{ "{{" }}workflow.parameters.aiReviewRuntime{{ "}}" }}'
      - name: GEMINI_AI_EXECUTION_TIMEOUT_MINUTES
        value: '{{ "{{" }}workflow.parameters.geminiAiExecutionTimeoutMinutes{{ "}}" }}'
      - name: JOB_NAME
        value: '{{ "{{" }}workflow.name{{ "}}" }}'
      - name: BUILD_NUMBER
        value: '{{ "{{" }}workflow.uid{{ "}}" }}'
      - name: GEMINI_MODEL
        value: {{ .Values.bswBuilder.geminiModel | quote }}
      - name: GEMINI_PREVIEW_FEATURES
        value: {{ .Values.bswBuilder.geminiPreviewFeatures | quote }}
      - name: GEMINI_LOCATION_GLOBAL
        value: {{ .Values.bswBuilder.geminiLocationGlobal | quote }}
      # gemini-cli only (agy uses workloads/common/agentic-ai/antigravity/skills/openbsw).
      - name: GEMINI_COMMAND_LINE
{{- if .Values.bswBuilder.geminiModel }}
        value: {{ printf "gemini --model %s --yolo --output-format json" .Values.bswBuilder.geminiModel | quote }}
{{- else }}
        value: {{ .Values.bswBuilder.geminiCommandLine | quote }}
{{- end }}
      - name: GEMINI_PROMPT_FILE
        value: {{ printf "%s/step1_triage.txt" $prompts | quote }}
      - name: GEMINI_PROMPT_FILE_2
        value: {{ printf "%s/step2_rca.txt" $prompts | quote }}
      - name: GEMINI_PROMPT_FILE_3
        value: {{ printf "%s/step3_fixes.txt" $prompts | quote }}
      - name: GEMINI_SKILLS_YAML
        value: {{ printf "%s/skills.yaml" $prompts | quote }}
    resources:
{{- include "openbsw.stepResources" (dict "root" . "step" "aiReview") | nindent 6 }}
    volumeMounts:
{{- include "openbsw.bswBuilder.workspaceMounts" . | nindent 6 }}
    args:
      - bash "${PIPELINE_REPO_ROOT}/workloads/common/agentic-ai/gemini/run_ai_review.sh"
{{- end -}}

{{- define "openbsw.bswBuilder.template.storage" -}}
- name: storage
  podSpecPatch: |
{{- include "openbsw.podAffinitySameWorkflow" . | nindent 4 }}
  container:
    image: {{ include "openbsw.bswBuilder.image" . | quote }}
    workingDir: {{ include "openbsw.bswBuilder.workspacePath" . | quote }}
    command: [bash, -euo, pipefail, -c]
{{- include "openbsw.cloudEnvFrom" . | nindent 4 }}
    env:
{{- include "openbsw.bswBuilder.env" . | nindent 6 }}
      - name: OPENBSW_ARTIFACT_ROOT_NAME
        value: {{ printf "%s-%s" .Values.cloud.project .Values.bswBuilder.storageBucketSuffix | quote }}
    resources:
{{- include "openbsw.stepResources" (dict "root" . "step" "storage") | nindent 6 }}
    volumeMounts:
{{- include "openbsw.bswBuilder.workspaceMounts" . | nindent 6 }}
    args:
      - |
        # Default destination: gs://<root>/<JOB_NAME>/<date>-<time>_<workflow.name>
        export OPENBSW_BUILD_NUMBER="$(date +%Y-%m-%d-%H%M%S)_${WORKFLOW_NAME}"
        "${PIPELINE_REPO_ROOT}"/workloads/openbsw/pipelines/builds/bsw_builder/bsw_storage.sh
{{- end -}}

{{- define "openbsw.bswBuilder.template.retain" -}}
- name: retain
  podSpecPatch: |
{{- include "openbsw.podAffinitySameWorkflow" . | nindent 4 }}
  container:
    image: {{ include "openbsw.bswBuilder.image" . | quote }}
    workingDir: /home/builder/bsw-builds
    command: [bash, -euo, pipefail, -c]
    env:
      - name: INSTANCE_RETENTION_TIME
        value: '{{ "{{" }}workflow.parameters.instanceRetentionTime{{ "}}" }}'
    resources:
{{- include "openbsw.stepResources" (dict "root" . "step" "retain") | nindent 6 }}
    securityContext:
      privileged: true
    volumeMounts:
{{- include "openbsw.bswBuilder.workspaceMounts" . | nindent 6 }}
      - name: host-lib-modules
        mountPath: /lib/modules
        readOnly: true
    args:
      - |
        echo "Keeping the build workspace for ${INSTANCE_RETENTION_TIME} minutes."
        echo "OpenBSW build: /home/builder/bsw-builds/openbsw. Logs: {{ include "openbsw.bswBuilder.workspacePath" . }}."
        echo "Open a shell: kubectl -n {{ .Values.workflowNamespace }} exec -it ${HOSTNAME} -c main -- bash"
        sleep "$(( INSTANCE_RETENTION_TIME * 60 ))"
{{- end -}}

{{- define "openbsw.bswBuilder.template.fail-workflow" -}}
- name: fail-workflow
  container:
    image: {{ include "openbsw.bswBuilder.image" . | quote }}
    command: [bash, -c]
    args:
      - "echo 'Build failed; marking workflow as Failed.'; exit 1"
{{- end -}}

{{- define "openbsw.bswBuilder.templates" -}}
{{ include "openbsw.bswBuilder.template.main" . }}
{{ include "openbsw.bswBuilder.template.check-image" . }}
{{ include "openbsw.bswBuilder.template.fetch-pipeline" . }}
{{ include "openbsw.bswBuilder.template.build" . }}
{{ include "openbsw.bswBuilder.template.ai-review" . }}
{{ include "openbsw.bswBuilder.template.storage" . }}
{{ include "openbsw.bswBuilder.template.retain" . }}
{{ include "openbsw.bswBuilder.template.fail-workflow" . }}
{{- end -}}
