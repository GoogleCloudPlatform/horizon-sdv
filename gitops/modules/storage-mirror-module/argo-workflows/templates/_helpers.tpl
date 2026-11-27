{{/*
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
*/}}

{{/*
Parameter order per template. The Sensors map body fields by index and the controller (internal/dispatch) sends the
same names, so names are only ever appended at the end.
*/}}
{{- define "smw.params.mirror" -}}
horizonSubmittedFrom requestId mirror directory kind manifestUrl manifestRef manifestFile syncJobs gitUrl gitBranches gitCredentialsSecret lfs syncTimeout pruneRefs excludeRefs gc submittedBy
{{- end -}}
{{- define "smw.params.delete" -}}
horizonSubmittedFrom requestId mirror directory confirm
{{- end -}}
{{- define "smw.params.volume-init" -}}
horizonSubmittedFrom requestId
{{- end -}}

{{- define "smw.labels" -}}
horizon-sdv.io/module: storage-mirror
app.kubernetes.io/part-of: storage-mirror-module
{{- end -}}

{{/* Workflow-level labels, so the controller sees every run whoever submitted it. */}}
{{- define "smw.workflowMetadata" -}}
workflowMetadata:
  labels:
    horizon-sdv.io/module: storage-mirror
  labelsFrom:
    storagemirror.horizon-sdv.io/request-id:
      expression: workflow.parameters.requestId
{{- if .mirror }}
    storagemirror.horizon-sdv.io/mirror:
      expression: workflow.parameters.mirror
{{- end }}
{{- end -}}

{{- define "smw.common" -}}
serviceAccountName: storage-mirror-sync
podGC:
  strategy: OnPodCompletion
ttlStrategy:
  secondsAfterSuccess: {{ .Values.ttlSecondsAfterSuccess }}
  secondsAfterFailure: {{ .Values.ttlSecondsAfterFailure }}
podMetadata:
  labels:
    mirror_build_pod: "true"
    horizon-sdv.io/storage-mirror-api-client: "true"
    horizon-sdv.io/storage-mirror-writer: "true"
  annotations:
    # A sync can run for 20 h; eviction would lose it (the Jenkins pod set this to "true").
    cluster-autoscaler.kubernetes.io/safe-to-evict: "false"
securityContext:
  runAsUser: 1000
  runAsGroup: 1000
volumes:
  - name: mirror
    persistentVolumeClaim:
      claimName: {{ .Values.rwClaimName }}
  - name: api-tls
    secret:
      secretName: {{ .Values.apiTlsSecretName }}
{{- end -}}

{{- define "smw.android.scheduling" -}}
nodeSelector:
  workloadLabel: android
tolerations:
  - key: workloadType
    operator: Equal
    value: android
    effect: NoSchedule
affinity:
  podAntiAffinity:
    requiredDuringSchedulingIgnoredDuringExecution:
      - labelSelector:
          matchExpressions:
            - key: aaos_pod
              operator: Exists
        topologyKey: kubernetes.io/hostname
      - labelSelector:
          matchExpressions:
            - key: cloud_ws_horizon_asfp_build_pod
              operator: Exists
        topologyKey: kubernetes.io/hostname
      - labelSelector:
          matchExpressions:
            - key: cloud_ws_horizon_android_studio_build_pod
              operator: Exists
        topologyKey: kubernetes.io/hostname
{{- end -}}

{{/* Agent container. dict: root, command, resources, params (space-separated parameter names). */}}
{{- define "smw.agentContainer" -}}
{{- $v := .root.Values -}}
image: {{ $v.syncImage | quote }}
imagePullPolicy: IfNotPresent
command: [mirror-agent, {{ .command | quote }}]
env:
  {{- range splitList " " .params }}
  {{- if ne . "horizonSubmittedFrom" }}
  - name: SM_{{ . | snakecase | upper }}
    value: {{ printf "{{workflow.parameters.%s}}" . | quote }}
  {{- end }}
  {{- end }}
  # Defaults for empty parameters (Sensors map missing body fields to "").
  - name: SM_DEFAULT_MANIFEST_URL
    value: {{ $v.defaults.repo.manifestUrl | quote }}
  - name: SM_DEFAULT_MANIFEST_REF
    value: {{ $v.defaults.repo.manifestRef | quote }}
  - name: SM_DEFAULT_MANIFEST_FILE
    value: {{ $v.defaults.repo.manifestFile | quote }}
  - name: SM_DEFAULT_SYNC_JOBS
    value: {{ $v.defaults.repo.syncJobs | toString | quote }}
  - name: SM_DEFAULT_REPO_SYNC_TIMEOUT
    value: {{ $v.defaults.repo.syncTimeout | quote }}
  - name: SM_DEFAULT_GIT_BRANCHES
    value: {{ join "," $v.defaults.git.branches | quote }}
  - name: SM_DEFAULT_GIT_SYNC_TIMEOUT
    value: {{ $v.defaults.git.syncTimeout | quote }}
  - name: SM_DEFAULT_LFS
    value: {{ $v.defaults.lfs | lower | quote }}
  - name: SM_RUN_ID
    value: "{{ "{{workflow.name}}" }}"
  - name: SM_MOUNT_PATH
    value: {{ $v.mountPath | quote }}
  - name: SM_ROOT_SUBDIR
    value: {{ $v.rootSubdir | quote }}
  - name: SM_CONTROLLER_URL
    value: {{ $v.controllerUrl | quote }}
  - name: SM_CONTROLLER_CA_FILE
    value: /etc/storage-mirror/tls/ca.crt
  - name: SM_CONTROLLER_TOKEN
    valueFrom:
      secretKeyRef:
        name: {{ $v.apiTokenSecretName }}
        key: storage-mirror-agent
        optional: true
resources:
  requests:
    cpu: {{ .resources.cpu | quote }}
    memory: {{ .resources.memory | quote }}
  limits:
    cpu: {{ .resources.cpu | quote }}
    memory: {{ .resources.memory | quote }}
securityContext:
  runAsNonRoot: true
  allowPrivilegeEscalation: false
  capabilities:
    drop: [ALL]
volumeMounts:
  - name: mirror
    mountPath: {{ $v.mountPath | quote }}
  # Sibling of /etc/storage-mirror/git-credentials (run-git-credentials): Secret mounts are read-only, so none is
  # nested inside another.
  - name: api-tls
    mountPath: /etc/storage-mirror/tls
    readOnly: true
{{- end -}}
