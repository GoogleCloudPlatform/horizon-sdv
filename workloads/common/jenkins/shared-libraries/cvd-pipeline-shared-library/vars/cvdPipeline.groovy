// Copyright (c) 2025 Accenture, All Rights Reserved.
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
def call(Map config = [:]) {
  def timeout_value = config.cleanup_container_timeout ?: 4

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

  def kubernetesPodTemplate = """
    apiVersion: v1
    kind: Pod
    metadata:
      annotations:
        cluster-autoscaler.kubernetes.io/safe-to-evict: "true"
    spec:
      serviceAccountName: ${JENKINS_SERVICE_ACCOUNT}
      containers:
      - name: builder
        image: ${CLOUD_REGION}-docker.pkg.dev/${CLOUD_PROJECT}/${ANDROID_BUILD_DOCKER_ARTIFACT_PATH_NAME}:latest
        imagePullPolicy: Always
        command:
        - sleep
        args:
        - ${timeout_value}h
  """.stripIndent()

  pipeline {
    // Parameters defined in groovy/job.groovy

    agent none

    stages {
      stage('Start VM Instance') {
        agent { label params.JENKINS_GCE_CLOUD_LABEL }

        stages {
          stage('Custom stage (1)') {
            when { expression { config.customStageOne != null } }
            steps {
              script {
                config.customStageOne.each { customStage ->
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
                  CVD_ADDITIONAL_FLAGS="${CVD_ADDITIONAL_FLAGS}" \
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
              catchError(buildResult: 'FAILURE', stageResult: 'FAILURE') {
                withCredentials([usernamePassword(credentialsId: 'jenkins-mtk-connect-apikey', passwordVariable: 'MTK_CONNECT_PASSWORD', usernameVariable: 'MTK_CONNECT_USERNAME')]) {
                  sh '''
                    cd ./workloads/common/mtk-connect/ || true
                    sudo \
                    MTK_CONNECT_DOMAIN=${HORIZON_DOMAIN} \
                    MTK_CONNECT_USERNAME=${MTK_CONNECT_USERNAME} \
                    MTK_CONNECT_PASSWORD=${MTK_CONNECT_PASSWORD} \
                    MTK_CONNECTED_DEVICES="${NUM_INSTANCES}" \
                    MTK_CONNECT_TEST_ARTIFACT="${CUTTLEFISH_DOWNLOAD_URL}" \
                    MTK_CONNECT_TESTBENCH="${JOB_NAME}-${BUILD_NUMBER}" \
                    MTK_CONNECT_TESTBENCH_USER=$([ "$MTK_CONNECT_PUBLIC" = "true" ] && echo "everyone" || echo "$BUILD_USER_ID") \
                    timeout 15m ./mtk_connect.sh --start
                    cd - || true
                  '''
                }
              }
            }
          }

          stage('Custom stage (2)') {
            when { expression { config.customStageTwo != null } }
            steps {
              script {
                config.customStageTwo.each { customStage ->
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
                expression { extra_connect_check() }
              }
            }
            steps {
              catchError(buildResult: 'FAILURE', stageResult: 'FAILURE') {
                withCredentials([usernamePassword(credentialsId: 'jenkins-mtk-connect-apikey', passwordVariable: 'MTK_CONNECT_PASSWORD', usernameVariable: 'MTK_CONNECT_USERNAME')]) {
                  sh '''
                    cd ./workloads/common/mtk-connect/ || true
                    sudo \
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

      stage('Cleanup') {
        agent { kubernetes { yaml kubernetesPodTemplate } }
        stages {
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
                expression { extra_connect_check() }
              }
            }
            steps {
              container(name: 'builder') {
                withCredentials([usernamePassword(credentialsId: 'jenkins-mtk-connect-apikey', passwordVariable: 'MTK_CONNECT_PASSWORD', usernameVariable: 'MTK_CONNECT_USERNAME')]) {
                  sh '''
                    cd ./workloads/common/mtk-connect/ || true
                    sudo \
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
