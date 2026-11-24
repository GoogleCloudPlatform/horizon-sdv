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
Templates of openbsw-dev-instance (port of workloads/openbsw/pipelines/environment/dev_instance/Jenkinsfile).

  main          DAG below
  instance      Jenkins stages MTK Connect to Host Device .. Stop Virtual Devices, in one pod
  exit-handler  deletes the testbench when MTK Connect was enabled and the workflow did not succeed
  mtk-cleanup   mtk_connect.sh --delete for this workflow's testbench
*/ -}}

{{- define "openbsw.devInstance.testbench" -}}
{{ include "openbsw.mtk.testbench" (dict "prefix" .Values.devInstance.testbenchPrefix) }}
{{- end -}}

{{- define "openbsw.devInstance.template.main" -}}
{{- $creds := eq (include "openbsw.useUmbrellaCreds" .) "true" -}}
- name: main
  dag:
    tasks:
{{- if $creds }}
{{- include "openbsw.prepareGitCredsTask" (dict "root" .) | nindent 6 }}
{{- end }}
      - name: instance
        template: instance
{{- if $creds }}
        depends: prepare-pipeline-git-creds.Succeeded
{{- end }}
{{- end -}}

{{- define "openbsw.devInstance.template.instance" -}}
- name: instance
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
    # No -e: as in Jenkins, the instance is retained when MTK Connect fails to start.
    command: [bash, -uo, pipefail, -c]
{{- include "openbsw.cloudEnvFrom" . | nindent 4 }}
    env:
{{- include "openbsw.mtk.env" . | nindent 6 }}
      - name: INSTANCE_MAX_UPTIME
        value: '{{ "{{" }}workflow.parameters.instanceMaxUptime{{ "}}" }}'
      - name: MTK_CONNECT_ENABLE
        value: '{{ "{{" }}workflow.parameters.mtkConnectEnable{{ "}}" }}'
      - name: MTK_CONNECTED_DEVICES
        value: '{{ "{{" }}workflow.parameters.numHostInstances{{ "}}" }}'
      - name: MTK_CONNECT_PUBLIC
        value: '{{ "{{" }}workflow.parameters.mtkConnectPublic{{ "}}" }}'
      - name: SUBMITTED_BY
        value: '{{ "{{" }}workflow.parameters.submittedBy{{ "}}" }}'
      - name: MTK_CONNECT_TESTBENCH
        value: {{ include "openbsw.devInstance.testbench" . | quote }}
      - name: MTK_CONNECT_TEST_ARTIFACT
        value: "N/A"
      - name: WORKFLOW_NAMESPACE
        value: {{ .Values.workflowNamespace | quote }}
    resources:
{{- include "openbsw.stepResources" (dict "root" . "step" "instance" "resources" .Values.devInstance.resources) | nindent 6 }}
    securityContext:
      # Same as the Jenkins pod, so POSIX builds can bring up tap and vcan0 interfaces.
      privileged: true
    volumeMounts:
      - name: host-lib-modules
        mountPath: /lib/modules
        readOnly: true
    args:
      - |
{{ include "openbsw.mtk.sessionFunctions" . | indent 8 }}

        rc=0
        if [[ "${MTK_CONNECT_ENABLE}" == "true" ]]; then
          echo "===== Stage: MTK Connect to Host Device ====="
          if ! { mtk_prepare && mtk_start; }; then
            echo "ERROR: MTK Connect start failed. The instance is kept, reachable with kubectl only." >&2
            mtk_stop
            rc=1
          fi
        fi

        echo "===== Stage: Retain instance (${INSTANCE_MAX_UPTIME} hours) ====="
        echo "Open a shell: kubectl -n ${WORKFLOW_NAMESPACE} exec -it ${HOSTNAME} -c main -- bash"
        echo "Save your work to persistent storage before the instance stops."
        mtk_hold "$(( 10#${INSTANCE_MAX_UPTIME} * 3600 ))"

        if [[ "${MTK_CONNECT_ENABLE}" == "true" ]]; then
          echo "===== Stage: Stop Virtual Devices ====="
          mtk_stop
        fi
        exit "${rc}"
{{- end -}}

{{- define "openbsw.devInstance.templates" -}}
{{ include "openbsw.devInstance.template.main" . }}
{{ include "openbsw.devInstance.template.instance" . }}
{{ include "openbsw.mtk.exitHandler" (dict
  "root" .
  "testbench" (include "openbsw.devInstance.testbench" .)
  "when" (printf "'%sworkflow.parameters.mtkConnectEnable%s' == 'true'" "{{" "}}")) }}
{{- end -}}
