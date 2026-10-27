// Copyright (c) 2025-2026 Accenture, All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//         http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
//
// Common shared library to provide CVD launch capabilities for both
// CVD Launcher and CTS Execution.
//
// Optional config.aiReview (Map): enables Diagnostics stage "AI Review" (runCvdGeminiAiReview).
// Keys: preset "cts"|"cvd" (recommended; canonical copyArtifacts filters) OR copyArtifactsFilter (override),
// promptSequencedDir (optional; defaults by preset: cts → cts_execution/prompt/sequenced, cvd → cvd_launcher/prompt/sequenced),
// requireCtsNotListOnly (optional, for CTS: only run when CTS_TEST_LISTS_ONLY is false).
// env.MTK_CONNECT_STAGE_ENTERED: set to 'true' in script only when the "MTK Connect to Virtual Devices" stage runs.
// Do not put this in pipeline environment{} — Declarative reapplies that each stage and would reset the flag to 'false'
// after MTK Connect sets it, skipping "MTK Connect Delete Testbench".
// Gated stages: "MTK Connect Delete Testbench", "Delete Offline Testbenches" — use env.MTK_CONNECT_STAGE_ENTERED (set when MTK Connect ran); do not repeat connect_condition here (same check already passed for that stage).
// When "MTK Connect to Virtual Devices" runs and fails, env.MTK_CONNECT_STAGE_FAILED is set to true and AI Review is skipped
// (CVD logs can still show success while MTK Connect timed out — avoids false-flag Gemini output).
// CTS preset = CVD-common patterns + android-cts-results + HTML reports; CVD preset = CVD-common only.
// Optional config.mtkConnectTunnelPort: MTK Connect tunnel caller port (overrides env MTK_CONNECT_TUNNEL_PORT; default 8555).
// Optional preLaunchStages / postMtkConnectStages: lists of [ name: String, steps: Closure ] inserted before CVD launch and after MTK Connect.
// Legacy aliases customStageOne / customStageTwo are still accepted.
def call(Map config = [:]) {
  def preLaunchStages = config.preLaunchStages != null ? config.preLaunchStages : config.customStageOne
  def postMtkConnectStages = config.postMtkConnectStages != null ? config.postMtkConnectStages : config.customStageTwo

  def timeout_value = config.cleanup_container_timeout ?: 4
  def mtk_tunnel_port = (config.mtkConnectTunnelPort != null) ? "${config.mtkConnectTunnelPort}".trim() : (env.MTK_CONNECT_TUNNEL_PORT ?: '8555')
  if (!mtk_tunnel_port) {
    mtk_tunnel_port = '8555'
  }

  def launcher_cond = config.launcher_condition ?: []
  def extra_launcher_cond = {
    launcher_cond.every { cond ->
        evaluate(cond)
    }
  }

  def connect_cond = config.connect_condition ?: []
  def extra_connect_check = {
    connect_cond.every { cond ->
        evaluate(cond)
    }
  }

  def keep_dev_cond = config.keep_dev_alive_cond ?: ["currentBuild.currentResult == 'SUCCESS'"]
  def extra_keep_dev_cond = {
    keep_dev_cond.every { cond ->
        evaluate(cond)
    }
  }

  def stop_dev_cond = config.stop_devices_cond ?: []
  def extra_stop_dev_cond = {
    stop_dev_cond.every { cond ->
        evaluate(cond)
    }
  }

  // Diagnostics & Teardown (AI Review, VM cleanup, MTK delete): Android build container (ANDROID_BUILD_DOCKER_ARTIFACT_PATH_NAME),
  // same requests/limits as workloads/utilities/gemini_ai_assistant/Jenkinsfile (explicit QoS; not BestEffort). Node targeting:
  // workloadLabel: android + workloadType=android:NoSchedule (aaos-builder pool). No podAntiAffinity or aaos_pod label
  // (other aaos_pod Jenkins agents may schedule on the same node).
  // safe-to-evict "false": long Gemini/AI Review runs were still seeing Eviction API evictions with "true" (CA scale-down
  // and similar); align with gemini_ai_assistant. Node memory/disk pressure can still evict.
  // yamlMergeStrategy merge(): ensure this yaml wins over inherited Kubernetes cloud podTemplate (e.g. default
  // safe-to-evict "true"), so cluster-autoscaler is less likely to delete the agent Pod for node scale-down.
  def kubernetesPodTemplate = """
    apiVersion: v1
    kind: Pod
    metadata:
      annotations:
        cluster-autoscaler.kubernetes.io/safe-to-evict: "false"
    spec:
      serviceAccountName: ${JENKINS_SERVICE_ACCOUNT}
      nodeSelector:
        workloadLabel: android
      tolerations:
        - key: workloadType
          operator: Equal
          value: android
          effect: NoSchedule
      containers:
      - name: builder
        image: ${CLOUD_REGION}-docker.pkg.dev/${CLOUD_PROJECT}/${ANDROID_BUILD_DOCKER_ARTIFACT_PATH_NAME}:latest
        imagePullPolicy: Always
        resources:
          requests:
            cpu: 16000m
            memory: 48Gi
          limits:
            cpu: 32000m
            memory: 96Gi
        command:
        - sleep
        args:
        - ${timeout_value}h
  """.stripIndent()

  pipeline {
    // Parameters defined in groovy/job.groovy
    // MTK_CONNECT_STAGE_ENTERED is set only in the MTK Connect stage (script); never in environment{} (see file header).
    // If CVD --start failed, MTK Connect is skipped — do not run MTK --stop/--delete (no testbench was created).

    agent none

    stages {
      stage('Start VM Instance') {
        agent { label params.JENKINS_GCE_CLOUD_LABEL }

        stages {
          stage('Pre-launch') {
            when { expression { preLaunchStages != null } }
            steps {
              script {
                preLaunchStages.each { customStage ->
                  stage(customStage.name) {
                    customStage.steps()
                  }
                }
              }
            }
          }

          stage('Launch Virtual Devices') {
            when {
              allOf {
                expression { env.CUTTLEFISH_DOWNLOAD_URL }
                expression { extra_launcher_cond() }
              }
            }
            steps {
              script {
                currentBuild.description = "${params.JENKINS_GCE_CLOUD_LABEL}" + '<br/>' + "$BUILD_USER"
              }
              catchError(buildResult: 'FAILURE', stageResult: 'FAILURE') {
                script {
                  env.VM_NODE_NAME = env.NODE_NAME
                }
                sh '''
                  CUTTLEFISH_DOWNLOAD_URL="${CUTTLEFISH_DOWNLOAD_URL}" \
                  CUTTLEFISH_INSTALL_WIFI="${CUTTLEFISH_INSTALL_WIFI}" \
                  CUTTLEFISH_MAX_BOOT_TIME="${CUTTLEFISH_MAX_BOOT_TIME}" \
                  NUM_INSTANCES="${NUM_INSTANCES}" \
                  VM_CPUS="${VM_CPUS}" \
                  VM_MEMORY_MB="${VM_MEMORY_MB}" \
                  CVD_COMMAND_LINE="${CVD_COMMAND_LINE}" \
                  ./workloads/android/pipelines/tests/cvd_launcher/cvd_start_stop.sh --start
                '''
              }
              archiveArtifacts artifacts: 'wifi*.log', followSymlinks: false, onlyIfSuccessful: false, allowEmptyArchive: true
            }
          }

          stage('MTK Connect to Virtual Devices') {
            when {
              allOf {
                expression { extra_connect_check() }
                expression { env.CUTTLEFISH_DOWNLOAD_URL }
                expression { currentBuild.currentResult == 'SUCCESS' }
              }
            }

            steps {
              script {
                env.MTK_CONNECT_STAGE_ENTERED = 'true'
                def mtkExit = 0
                catchError(buildResult: 'FAILURE', stageResult: 'FAILURE') {
                  withCredentials([usernamePassword(credentialsId: 'jenkins-mtk-connect-apikey', passwordVariable: 'MTK_CONNECT_PASSWORD', usernameVariable: 'MTK_CONNECT_USERNAME')]) {
                    mtkExit = sh(returnStatus: true, script: '''
                      cd ./workloads/common/mtk-connect/ || true
                      sudo \
                      MTK_CONNECT_TUNNEL_PORT=''' + mtk_tunnel_port + ''' \
                      MTK_CONNECT_DOMAIN=${HORIZON_DOMAIN} \
                      MTK_CONNECT_USERNAME=${MTK_CONNECT_USERNAME} \
                      MTK_CONNECT_PASSWORD=${MTK_CONNECT_PASSWORD} \
                      MTK_CONNECTED_DEVICES="${NUM_INSTANCES}" \
                      MTK_CONNECT_TEST_ARTIFACT="${CUTTLEFISH_DOWNLOAD_URL}" \
                      MTK_CONNECT_TESTBENCH="${JOB_NAME}-${BUILD_NUMBER}" \
                      MTK_CONNECT_TESTBENCH_USER=$([ "$MTK_CONNECT_PUBLIC" = "true" ] && echo "everyone" || echo "$BUILD_USER_ID") \
                      timeout 15m ./mtk_connect.sh --start
                      cd - || true
                    '''.stripIndent())
                  }
                  if (mtkExit != 0) {
                    error("MTK Connect --start failed with exit code ${mtkExit}")
                  }
                }
                env.MTK_CONNECT_STAGE_FAILED = (mtkExit != 0) ? 'true' : 'false'
              }
            }
          }

          stage('Post-MTK Connect') {
            when { expression { postMtkConnectStages != null } }
            steps {
              script {
                postMtkConnectStages.each { customStage ->
                  stage(customStage.name) {
                    customStage.steps()
                  }
                }
              }
            }
          }

          stage('Keep Devices Alive') {
            when {
              allOf {
                expression { env.CUTTLEFISH_DOWNLOAD_URL }
                expression { extra_keep_dev_cond() }
              }
            }
            steps {
              catchError(buildResult: 'FAILURE', stageResult: 'FAILURE') {
                script {
                  sleep(time: "${CUTTLEFISH_KEEP_ALIVE_TIME}", unit: 'MINUTES')
                }
              }
            }
          }

          stage('MTK Connect Delete Testbench') {
            when {
              allOf {
                expression { env.CUTTLEFISH_DOWNLOAD_URL }
                expression { env.MTK_CONNECT_STAGE_ENTERED == 'true' }
              }
            }
            steps {
              catchError(buildResult: 'FAILURE', stageResult: 'FAILURE') {
                withCredentials([usernamePassword(credentialsId: 'jenkins-mtk-connect-apikey', passwordVariable: 'MTK_CONNECT_PASSWORD', usernameVariable: 'MTK_CONNECT_USERNAME')]) {
                  sh '''
                    cd ./workloads/common/mtk-connect/ || true
                    sudo \
                    MTK_CONNECT_TUNNEL_PORT=''' + mtk_tunnel_port + ''' \
                    MTK_CONNECT_DOMAIN=${HORIZON_DOMAIN} \
                    MTK_CONNECT_USERNAME=${MTK_CONNECT_USERNAME} \
                    MTK_CONNECT_PASSWORD=${MTK_CONNECT_PASSWORD} \
                    MTK_CONNECTED_DEVICES="${NUM_INSTANCES}" \
                    MTK_CONNECT_TESTBENCH="${JOB_NAME}-${BUILD_NUMBER}" \
                    timeout 10m ./mtk_connect.sh --stop || true
                    cd - || true
                  '''
                }
              }
            }
          }

          stage('Stop Virtual Devices') {
            when {
              allOf {
                expression { env.CUTTLEFISH_DOWNLOAD_URL }
                expression { extra_stop_dev_cond() }
              }
            }
            steps {
              catchError(buildResult: 'FAILURE', stageResult: 'FAILURE') {
                withCredentials([usernamePassword(credentialsId: 'jenkins-mtk-connect-apikey', passwordVariable: 'MTK_CONNECT_PASSWORD', usernameVariable: 'MTK_CONNECT_USERNAME')]) {
                  sh '''
                    ./workloads/android/pipelines/tests/cvd_launcher/cvd_start_stop.sh --stop || true
                  '''
                  archiveArtifacts artifacts: 'cvd*.log', followSymlinks: false, onlyIfSuccessful: false, allowEmptyArchive: true
                  archiveArtifacts artifacts: 'cuttlefish*.zip', followSymlinks: false, onlyIfSuccessful: false, allowEmptyArchive: true
                }
              }
            }
          }
        }
      }

      stage('Diagnostics & Teardown') {
        agent {
          kubernetes {
            yamlMergeStrategy merge()
            yaml kubernetesPodTemplate
          }
        }
        stages {
          stage('AI Review') {
            when {
              allOf {
                expression { config.aiReview != null }
                expression { env.ENABLE_GEMINI_AI_ASSISTANT == 'true' }
                expression { env.MTK_CONNECT_STAGE_FAILED != 'true' }
                expression {
                  // Default: run AI Review only on FAILURE (original behavior).
                  // Optional: allow AI Review on SUCCESS when GEMINI_ANALYSE_ON_SUCCESS=true.
                  currentBuild.currentResult == 'FAILURE' || env.GEMINI_ANALYSE_ON_SUCCESS == 'true'
                }
                expression {
                  def req = config.aiReview.get('requireCtsNotListOnly')
                  req != true || env.CTS_TEST_LISTS_ONLY == 'false'
                }
              }
            }
            steps {
              container(name: 'builder') {
                script {
                  def ar = new LinkedHashMap(config.aiReview)
                  ar.remove('requireCtsNotListOnly')
                  runCvdGeminiAiReview(ar)
                }
              }
            }
          }

          // Remove VM instances on error to avoid instances left running.
          stage('Remove VM Instance') {
            when { expression { currentBuild.currentResult != 'SUCCESS' } }
            steps {
              container(name: 'builder') {
                sh '''
                  echo "Removing " ${VM_NODE_NAME} " on error!" || true
                  yes Y | gcloud compute instances delete ${VM_NODE_NAME} --zone ${CLOUD_ZONE} || true
                '''
              }
            }
          }

          stage('Delete Offline Testbenches') {
            when {
              allOf {
                expression { currentBuild.currentResult != 'SUCCESS' }
                expression { env.MTK_CONNECT_STAGE_ENTERED == 'true' }
              }
            }
            steps {
              container(name: 'builder') {
                withCredentials([usernamePassword(credentialsId: 'jenkins-mtk-connect-apikey', passwordVariable: 'MTK_CONNECT_PASSWORD', usernameVariable: 'MTK_CONNECT_USERNAME')]) {
                  sh '''
                    cd ./workloads/common/mtk-connect/ || true
                    sudo \
                    MTK_CONNECT_TUNNEL_PORT=''' + mtk_tunnel_port + ''' \
                    MTK_CONNECT_DOMAIN=${HORIZON_DOMAIN} \
                    MTK_CONNECT_USERNAME=${MTK_CONNECT_USERNAME} \
                    MTK_CONNECT_PASSWORD=${MTK_CONNECT_PASSWORD} \
                    MTK_CONNECT_TESTBENCH="${JOB_NAME}-${BUILD_NUMBER}" \
                    MTK_CONNECT_DELETE_OFFLINE_TESTBENCHES=true \
                    MTK_CONNECT_CONTAINER_ONLY="true" \
                    timeout 15m ./mtk_connect.sh --delete || true
                    cd - || true
                  '''
                }
              }
            }
          }
        }
      }
    }
  }
}

// AI Review copyArtifacts: shared non-CTS patterns (CVD Launcher preset). CTS Execution adds CTS result trees only.
def aiReviewCopyArtifactsFilterCvdCommon() {
  '**/wifi*.log,**/cvd*.log,**/cts*.txt,**/cuttlefish*.zip'
}

def aiReviewCopyArtifactsFilterCtsExecution() {
  '**/*.zip,**/invocation_summary.txt,**/test_result*.html,' + aiReviewCopyArtifactsFilterCvdCommon()
}

// Default sequenced prompts + skills: CTS preset → cts_execution/prompt/sequenced; CVD preset → cvd_launcher/prompt/sequenced.
// Override with aiReview.promptSequencedDir when needed.
def runCvdGeminiAiReview(Map cfg = [:]) {
  def filter = cfg.copyArtifactsFilter
  if (!filter) {
    switch (cfg.preset?.toString()) {
      case 'cts':
        filter = aiReviewCopyArtifactsFilterCtsExecution()
        break
      case 'cvd':
        filter = aiReviewCopyArtifactsFilterCvdCommon()
        break
      default:
        break
    }
  }
  if (!filter) {
    error 'runCvdGeminiAiReview: set aiReview.preset ("cts" or "cvd") or copyArtifactsFilter'
  }
  def presetForPrompts = cfg.preset?.toString()
  cfg.remove('preset')
  def defaultPromptDir = (presetForPrompts == 'cvd')
    ? 'workloads/android/pipelines/tests/cvd_launcher/prompt/sequenced'
    : 'workloads/android/pipelines/tests/cts_execution/prompt/sequenced'
  def promptDir = cfg.promptSequencedDir ?: defaultPromptDir
  // Prefer JOB_NAME so copyArtifacts resolves the running job reliably (same as Copy Artifact UI / permissions).
  def projectName = cfg.projectName ?: env.JOB_NAME
  def flatten = cfg.containsKey('flatten') ? cfg.flatten : true

  catchError(buildResult: 'FAILURE', stageResult: 'FAILURE') {
    copyArtifacts(
      projectName: projectName,
      selector: specific(env.BUILD_NUMBER),
      filter: filter,
      flatten: flatten,
      optional: true,
      target: 'test-results/'
    )
    def promptBase = "${env.WORKSPACE}/${promptDir}"
    sh """
    set +x
    export GEMINI_ARTIFACTS_COMMAND='mkdir -p "\${WORKSPACE}/test-results/cvd" "\${WORKSPACE}/test-results/cts" 2>/dev/null || true; for z in "\${WORKSPACE}"/test-results/cuttlefish*.zip "\${WORKSPACE}"/cuttlefish*.zip; do if [ -f "\$z" ]; then echo "[cvdPipeline] Unpacking \$z -> test-results/cvd/"; unzip -q -o "\$z" -d "\${WORKSPACE}/test-results/cvd" 2>/dev/null || true; fi; done; for z in "\${WORKSPACE}"/test-results/android-cts-results/*.zip "\${WORKSPACE}"/android-cts-results/*.zip "\${WORKSPACE}"/test-results/*.zip; do if [ -f "\$z" ] && [[ "\$(basename "\$z")" != cuttlefish* ]]; then echo "[cvdPipeline] Unpacking \$z -> test-results/cts/"; unzip -q -o "\$z" -d "\${WORKSPACE}/test-results/cts" 2>/dev/null || true; for d in "\${WORKSPACE}"/test-results/cts/*; do if [ -d "\$d" ] && { [ -f "\$d/test_result.xml" ] || [ -f "\$d/invocation_summary.txt" ]; }; then echo "[cvdPipeline] Hoisting CTS session artifacts from \$d -> test-results/cts/"; find "\$d" -mindepth 1 -maxdepth 1 -exec mv -f {} "\${WORKSPACE}/test-results/cts/" \\; 2>/dev/null || true; rmdir "\$d" 2>/dev/null || true; fi; done; fi; done; if [ -f "\${WORKSPACE}/test-results/cts/invocation_summary.txt" ] && [ ! -f "\${WORKSPACE}/test-results/invocation_summary.txt" ]; then cp -f "\${WORKSPACE}/test-results/cts/invocation_summary.txt" "\${WORKSPACE}/test-results/invocation_summary.txt"; fi'
    export PIPELINE_REPO_ROOT="\${WORKSPACE}"
    export GEMINI_ANALYSIS_PATH="\${WORKSPACE}"
    export AI_REVIEW_RUNTIME="\${AI_REVIEW_RUNTIME:-agy}"
    export GEMINI_SKIP_MOVE_ARTIFACTS=1
    export GEMINI_AI_EXECUTION_TIMEOUT_MINUTES="\${GEMINI_AI_EXECUTION_TIMEOUT_MINUTES:-60}"
    export GEMINI_PREVIEW_FEATURES="\${GEMINI_PREVIEW_FEATURES}"
    [ "\${GEMINI_LOCATION_GLOBAL}" = "true" ] && export GOOGLE_CLOUD_LOCATION="global" || export GOOGLE_CLOUD_LOCATION="\${CLOUD_REGION}"
    export GOOGLE_CLOUD_PROJECT="\${CLOUD_PROJECT}"
    export GEMINI_MODEL="\${GEMINI_MODEL:-}"
    export GEMINI_COMMAND_LINE="\${GEMINI_COMMAND_LINE}"
    export GEMINI_ADDITIONAL_ARTIFACTS="\${WORKSPACE}/test-results/"
    CTS_PROMPT_DIR='${promptBase}'
    export GEMINI_PROMPT_FILE="\${CTS_PROMPT_DIR}/step1_triage.txt"
    export GEMINI_PROMPT_FILE_2="\${CTS_PROMPT_DIR}/step2_rca.txt"
    export GEMINI_PROMPT_FILE_3="\${CTS_PROMPT_DIR}/step3_fixes.txt"
    export GEMINI_POST_REVIEW_COMMAND='rm -rf "\${WORKSPACE}/test-results/cvd" "\${WORKSPACE}/test-results/cts" 2>/dev/null || true; export GEMINI_ARTIFACT_ROOT_NAME="\${ANDROID_BUILD_BUCKET_ROOT_NAME}"; export GEMINI_ARTIFACT_STORAGE_SOLUTION="\${CTS_ARTIFACT_STORAGE_SOLUTION}"; export STORAGE_LABELS="\${STORAGE_LABELS}"; "\${WORKSPACE}/workloads/common/agentic-ai/gemini/gemini_storage.sh"; find . -type f -name "headless*.json" -size 0 -delete'
    "\${WORKSPACE}"/workloads/common/agentic-ai/gemini/run_ai_review.sh
  """.stripIndent()
  }
  archiveArtifacts artifacts: 'gemini-assist/*', followSymlinks: false, onlyIfSuccessful: false, allowEmptyArchive: true
  archiveArtifacts artifacts: 'traces/*.json', followSymlinks: false, onlyIfSuccessful: false, allowEmptyArchive: true
  script {
    def runtime = (params.AI_REVIEW_RUNTIME ?: env.AI_REVIEW_RUNTIME ?: '').toString().trim()
    if (runtime != 'agy') {
      archiveArtifacts artifacts: 'headless_output*.json', followSymlinks: false, onlyIfSuccessful: false, allowEmptyArchive: false
    }
  }
  archiveArtifacts artifacts: '*artifacts*.txt', followSymlinks: false, onlyIfSuccessful: false, allowEmptyArchive: true
  script {
    if (fileExists('gemini-client-error.zip')) {
      archiveArtifacts artifacts: 'gemini-client-error.zip', followSymlinks: false, onlyIfSuccessful: false, allowEmptyArchive: true
    }
  }
}
