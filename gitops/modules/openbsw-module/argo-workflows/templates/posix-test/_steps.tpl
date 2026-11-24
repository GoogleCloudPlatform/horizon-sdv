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
Templates of openbsw-posix-test (port of workloads/openbsw/pipelines/tests/posix/Jenkinsfile).

  main          DAG below
  test          Jenkins stages Download artifacts .. Stop Virtual Devices, in one pod: the MTK Connect
                agent started by mtk_connect.sh --start runs in the background until --stop
  exit-handler  deletes the testbench when the workflow did not succeed
  mtk-cleanup   mtk_connect.sh --delete for this workflow's testbench
*/ -}}

{{- define "openbsw.posixTest.testbench" -}}
{{ include "openbsw.mtk.testbench" (dict "prefix" .Values.posixTest.testbenchPrefix) }}
{{- end -}}

{{- define "openbsw.posixTest.downloadUrlAllowPrefixes" -}}
{{- $default := list (printf "gs://%s-%s/" .Values.cloud.project .Values.bswBuilder.storageBucketSuffix) -}}
{{- join "," (.Values.posixTest.downloadUrlAllowPrefixes | default $default) -}}
{{- end -}}

{{- define "openbsw.posixTest.template.main" -}}
{{- $creds := eq (include "openbsw.useUmbrellaCreds" .) "true" -}}
- name: main
  dag:
    tasks:
{{- if $creds }}
{{- include "openbsw.prepareGitCredsTask" (dict "root" .) | nindent 6 }}
{{- end }}
      - name: test
        template: test
{{- if $creds }}
        depends: prepare-pipeline-git-creds.Succeeded
{{- end }}
{{- end -}}

{{- define "openbsw.posixTest.template.test" -}}
- name: test
  inputs:
    artifacts:
      - name: pipeline-repo
        path: {{ include "openbsw.mtk.repoPath" . | quote }}
{{- include "openbsw.pipelineRepoGitArtifact" . | nindent 8 }}
  # Time for mtk_connect.sh --stop after SIGTERM (argo stop, pod deletion).
  podSpecPatch: '{"terminationGracePeriodSeconds": {{ .Values.mtkConnect.stopGracePeriodSeconds }}}'
  container:
    image: {{ include "openbsw.builderImageRef" . | quote }}
    workingDir: /home/builder
    # No -e: the stop stage must run after a failed download or MTK Connect start.
    command: [bash, -uo, pipefail, -c]
{{- include "openbsw.cloudEnvFrom" . | nindent 4 }}
    env:
{{- include "openbsw.mtk.env" . | nindent 6 }}
      - name: OPENBSW_DOWNLOAD_URL
        value: '{{ "{{" }}workflow.parameters.openbswDownloadUrl{{ "}}" }}'
      - name: DOWNLOAD_URL_ALLOW_PREFIXES
        value: {{ include "openbsw.posixTest.downloadUrlAllowPrefixes" . | quote }}
      - name: POSIX_KEEP_ALIVE_TIME
        value: '{{ "{{" }}workflow.parameters.posixKeepAliveTime{{ "}}" }}'
      - name: MTK_CONNECTED_DEVICES
        value: '{{ "{{" }}workflow.parameters.numHostInstances{{ "}}" }}'
      - name: MTK_CONNECT_PUBLIC
        value: '{{ "{{" }}workflow.parameters.mtkConnectPublic{{ "}}" }}'
      - name: SUBMITTED_BY
        value: '{{ "{{" }}workflow.parameters.submittedBy{{ "}}" }}'
      - name: MTK_CONNECT_TESTBENCH
        value: {{ include "openbsw.posixTest.testbench" . | quote }}
      - name: MTK_CONNECT_TEST_ARTIFACT
        value: '{{ "{{" }}workflow.parameters.openbswDownloadUrl{{ "}}" }}'
    resources:
{{- include "openbsw.stepResources" (dict "root" . "step" "test" "resources" .Values.posixTest.resources) | nindent 6 }}
    securityContext:
      # Same as the Jenkins pod: the POSIX application brings up a tap interface and vcan0.
      privileged: true
    volumeMounts:
      - name: host-lib-modules
        mountPath: /lib/modules
        readOnly: true
    args:
      - |
{{ include "openbsw.mtk.sessionFunctions" . | indent 8 }}

        # The artifact is run on a privileged pod, so it must come from an allowed bucket.
        url="${OPENBSW_DOWNLOAD_URL%/}"
        if [[ -z "${url}" ]]; then
          echo "ERROR: openbswDownloadUrl is required (the posix/ folder of an openbsw-bsw-builder run)." >&2
          exit 1
        fi
        allowed=""
        IFS=',' read -r -a prefixes <<< "${DOWNLOAD_URL_ALLOW_PREFIXES}"
        for prefix in "${prefixes[@]}"; do
          [[ -n "${prefix}" && "${url}/" == "${prefix}"* ]] && allowed=1
        done
        if [[ -z "${allowed}" || ! "${url}" =~ ^gs://[A-Za-z0-9._/-]+$ ]]; then
          echo "ERROR: openbswDownloadUrl must start with one of: ${DOWNLOAD_URL_ALLOW_PREFIXES}" >&2
          exit 1
        fi
        mtk_prepare || exit 1

        echo "===== Stage: Download artifacts ====="
        OPENBSW_DOWNLOAD_URL="${url}" timeout 30m \
          bash "${PIPELINE_REPO_ROOT}/workloads/openbsw/pipelines/tests/posix/bsw_download_artifact.sh"
        if [[ ! -d "${HOME}/posix/tools" || ! -d "${HOME}/posix/test/pyTest" ]]; then
          echo "ERROR: ${url} did not unpack to ${HOME}/posix (expected posix.tgz with tools/ and test/pyTest/)." >&2
          exit 1
        fi

        echo "===== Stage: MTK Connect to POSIX Host Device ====="
        if ! mtk_start; then
          echo "ERROR: MTK Connect start failed." >&2
          mtk_stop
          exit 1
        fi

        echo "===== Stage: Keep Devices Alive (${POSIX_KEEP_ALIVE_TIME} minutes) ====="
        echo "Testbenches: https://${HORIZON_DOMAIN}/mtk-connect/portal/testbenches"
        echo "The POSIX application is in ${HOME}/posix. Open a HOST terminal there to run it or pyTest."
        mtk_hold "$(( 10#${POSIX_KEEP_ALIVE_TIME} * 60 ))"

        echo "===== Stage: Stop Virtual Devices ====="
        mtk_stop
{{- end -}}

{{- define "openbsw.posixTest.templates" -}}
{{ include "openbsw.posixTest.template.main" . }}
{{ include "openbsw.posixTest.template.test" . }}
{{ include "openbsw.mtk.exitHandler" (dict "root" . "testbench" (include "openbsw.posixTest.testbench" .)) }}
{{- end -}}
