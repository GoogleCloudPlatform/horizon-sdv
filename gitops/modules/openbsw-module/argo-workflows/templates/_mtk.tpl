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
MTK Connect helpers for openbsw-posix-test, openbsw-dev-instance and openbsw-delete-mtkc-testbench.
They run workloads/common/mtk-connect/mtk_connect.sh from the Horizon checkout, as the Jenkins jobs do,
with the API key from Secret mtkConnect.secretName (workflow-mtk-connect-apikey, the same key as
jenkins-mtk-connect-apikey).

mtk_connect.sh writes the testbench name and owner into a dotenv file, so the scripts accept plain
names only. A newline would otherwise add MTK_CONNECT_* settings, such as another API domain.
*/ -}}

{{- define "openbsw.mtk.repoPath" -}}/horizon{{- end -}}

{{- define "openbsw.mtk.env" -}}
- name: PIPELINE_REPO_ROOT
  value: {{ include "openbsw.mtk.repoPath" . | quote }}
- name: MTK_SECRET_NAME
  value: {{ .Values.mtkConnect.secretName | quote }}
# optional: the scripts check for the key and say which Secret is missing.
- name: MTK_CONNECT_USERNAME
  valueFrom:
    secretKeyRef:
      name: {{ .Values.mtkConnect.secretName | quote }}
      key: {{ .Values.mtkConnect.usernameKey | quote }}
      optional: true
- name: MTK_CONNECT_PASSWORD
  valueFrom:
    secretKeyRef:
      name: {{ .Values.mtkConnect.secretName | quote }}
      key: {{ .Values.mtkConnect.passwordKey | quote }}
      optional: true
- name: MTK_CONNECT_DEVICE_PREFIX
  value: {{ .Values.mtkConnect.devicePrefix | quote }}
- name: MTK_MAX_DEVICES
  value: {{ .Values.mtkConnect.maxDevices | toString | quote }}
- name: MTK_TESTBENCH_MUST_CONTAIN
  value: {{ .Values.mtkConnect.testbenchMustContain | quote }}
{{- end -}}

{{/*
mtk_check_api: API key, domain and testbench name. Shared by the session and the delete scripts.
*/}}
{{- define "openbsw.mtk.checkApi" -}}
mtk_check_api() {
  if [[ -z "${MTK_CONNECT_USERNAME:-}" || -z "${MTK_CONNECT_PASSWORD:-}" ]]; then
    echo "ERROR: no MTK Connect API key in Secret ${MTK_SECRET_NAME} (keys username and password)." >&2
    return 1
  fi
  if [[ -z "${HORIZON_DOMAIN:-}" ]]; then
    echo "ERROR: HORIZON_DOMAIN is not set (ConfigMap horizon-workflow-cloud-env)." >&2
    return 1
  fi
  if [[ ! "${MTK_CONNECT_TESTBENCH}" =~ ^[A-Za-z0-9._/-]+$ ]]; then
    echo "ERROR: invalid MTK Connect testbench name '${MTK_CONNECT_TESTBENCH}'. Allowed: A-Z a-z 0-9 . _ / -" >&2
    return 1
  fi
  if [[ "${MTK_CONNECT_TESTBENCH}" != *"${MTK_TESTBENCH_MUST_CONTAIN}"* ]]; then
    echo "ERROR: only testbenches whose name contains '${MTK_TESTBENCH_MUST_CONTAIN}' are handled here." >&2
    return 1
  fi
  export MTK_CONNECT_DOMAIN="${HORIZON_DOMAIN}"
}
{{- end -}}

{{/*
Session functions: mtk_prepare, mtk_start, mtk_stop and mtk_hold, plus a SIGTERM trap that removes the
testbench when the workflow is stopped. Inputs arrive as environment variables, never substituted
into the script.
*/}}
{{- define "openbsw.mtk.sessionFunctions" -}}
mtk_dir="${PIPELINE_REPO_ROOT}/workloads/common/mtk-connect"
mtk_env=MTK_CONNECT_DOMAIN,MTK_CONNECT_USERNAME,MTK_CONNECT_PASSWORD,MTK_CONNECTED_DEVICES,MTK_CONNECT_TESTBENCH,MTK_CONNECT_TESTBENCH_USER,MTK_CONNECT_TEST_ARTIFACT,MTK_CONNECT_CONTAINER_ONLY,MTK_CONNECT_HOST_ONLY,MTK_CONNECT_DEVICE_PREFIX
mtk_started=0

{{ include "openbsw.mtk.checkApi" . }}

# Checks the inputs and sets the testbench owner (Jenkins: everyone, or BUILD_USER_ID).
mtk_prepare() {
  mtk_check_api || return 1
  if [[ ! "${MTK_CONNECTED_DEVICES}" =~ ^[0-9]+$ ]] || (( 10#${MTK_CONNECTED_DEVICES} < 1 || 10#${MTK_CONNECTED_DEVICES} > MTK_MAX_DEVICES )); then
    echo "ERROR: numHostInstances must be between 1 and ${MTK_MAX_DEVICES}, got '${MTK_CONNECTED_DEVICES}'." >&2
    return 1
  fi
  if [[ "${MTK_CONNECT_PUBLIC}" == "true" ]]; then
    MTK_CONNECT_TESTBENCH_USER=everyone
  elif [[ -z "${SUBMITTED_BY}" ]]; then
    echo "ERROR: submittedBy is empty. Submit through Horizon, which sets it to the signed-in user, or set mtkConnectPublic=true." >&2
    return 1
  elif [[ ! "${SUBMITTED_BY}" =~ ^[A-Za-z0-9._@+-]+$ ]]; then
    echo "ERROR: invalid submittedBy '${SUBMITTED_BY}'." >&2
    return 1
  else
    MTK_CONNECT_TESTBENCH_USER="${SUBMITTED_BY}"
  fi
  MTK_CONNECTED_DEVICES=$(( 10#${MTK_CONNECTED_DEVICES} ))
  export MTK_CONNECT_TESTBENCH_USER MTK_CONNECTED_DEVICES
  export MTK_CONNECT_CONTAINER_ONLY=true MTK_CONNECT_HOST_ONLY=true
}

mtk_start() {
  echo "===== MTK Connect: start testbench ${MTK_CONNECT_TESTBENCH} for ${MTK_CONNECT_TESTBENCH_USER} (${MTK_CONNECTED_DEVICES} devices) ====="
  mtk_started=1
  # --preserve-env passes the API key without putting it on a command line.
  (cd "${mtk_dir}" && sudo --preserve-env="${mtk_env}" timeout 15m bash ./mtk_connect.sh --start)
}

mtk_stop() {
  (( mtk_started )) || return 0
  mtk_started=0
  echo "===== MTK Connect: stop testbench ${MTK_CONNECT_TESTBENCH} ====="
  (cd "${mtk_dir}" && sudo --preserve-env="${mtk_env}" timeout 10m bash ./mtk_connect.sh --stop) ||
    echo "WARNING: MTK Connect stop failed. The exit handler deletes the testbench once it is offline."
}

trap 'echo "Termination requested."; mtk_stop; exit 143' TERM INT

# sleep runs in the background so the trap fires as soon as the signal arrives.
mtk_hold() {
  sleep "$1" &
  wait $!
}
{{- end -}}

{{/*
Template that deletes an offline testbench through the MTK Connect API (mtk_connect.sh --delete).
No agent is started, so a small Node image is enough. An online testbench is skipped.
Usage: include "openbsw.mtk.deleteTemplate" (dict "root" . "name" <template> "testbench" <value> "delaySeconds" <n>)
*/}}
{{- define "openbsw.mtk.deleteTemplate" -}}
{{- $root := .root -}}
- name: {{ .name }}
  inputs:
    artifacts:
      - name: pipeline-repo
        path: {{ include "openbsw.mtk.repoPath" $root | quote }}
{{- include "openbsw.pipelineRepoGitArtifact" $root | nindent 8 }}
  container:
    image: {{ $root.Values.mtkConnect.apiImage | quote }}
    workingDir: {{ printf "%s/workloads/common/mtk-connect" (include "openbsw.mtk.repoPath" $root) | quote }}
    command: [bash, -euo, pipefail, -c]
{{- include "openbsw.cloudEnvFrom" $root | nindent 4 }}
    env:
{{- include "openbsw.mtk.env" $root | nindent 6 }}
      - name: MTK_CONNECT_TESTBENCH
        value: {{ .testbench | quote }}
      - name: DELETE_DELAY_SECONDS
        value: {{ .delaySeconds | toString | quote }}
    resources:
{{- include "openbsw.stepResources" (dict "root" $root "step" "delete" "resources" $root.Values.mtkConnect.resources) | nindent 6 }}
    args:
      - |
{{ include "openbsw.mtk.checkApi" $root | indent 8 }}

        mtk_check_api
        if (( DELETE_DELAY_SECONDS > 0 )); then
          echo "Waiting ${DELETE_DELAY_SECONDS}s so MTK Connect marks the stopped agent offline."
          sleep "${DELETE_DELAY_SECONDS}"
        fi
        echo "===== MTK Connect: delete offline testbench ${MTK_CONNECT_TESTBENCH} ====="
        MTK_CONNECT_DELETE_OFFLINE_TESTBENCHES=true MTK_CONNECT_CONTAINER_ONLY=true \
          timeout 15m bash ./mtk_connect.sh --delete
        echo "An online testbench is left in place: stop the workflow that owns it first."
{{- end -}}

{{/*
Exit handler of the session templates. A session that succeeded has removed its testbench. Otherwise
(failed step, evicted pod, argo stop) the testbench may still exist, so it is deleted once offline.
Git credentials are minted again, because the token from the start of a long session has expired.
Usage: include "openbsw.mtk.exitHandler" (dict "root" . "testbench" <value> "when" <extra condition, optional>)
*/}}
{{- define "openbsw.mtk.exitHandler" -}}
{{- $root := .root -}}
{{- $when := printf "'%sworkflow.status%s' != 'Succeeded'" "{{" "}}" -}}
{{- with .when }}{{ $when = printf "%s && %s" $when . }}{{ end -}}
- name: exit-handler
  steps:
{{- if eq (include "openbsw.useUmbrellaCreds" $root) "true" }}
    -
{{- include "openbsw.prepareGitCredsTask" (dict "root" $root "when" $when) | nindent 6 }}
{{- end }}
    - - name: mtk-cleanup
        template: mtk-cleanup
        when: {{ $when | quote }}
{{ include "openbsw.mtk.deleteTemplate" (dict "root" $root "name" "mtk-cleanup" "testbench" .testbench "delaySeconds" $root.Values.mtkConnect.cleanupDelaySeconds) }}
{{- end -}}

{{/*
Testbench name: <prefix>-<workflow name>, like Jenkins <job name>-<build number>.
*/}}
{{- define "openbsw.mtk.testbench" -}}
{{ .prefix }}-{{ "{{" }}workflow.name{{ "}}" }}
{{- end -}}
