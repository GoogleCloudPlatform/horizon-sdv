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
Helm helpers for common env blocks in AAOS workflows.
*/ -}}

{{- define "aaos-builder.cloudEnvFrom" -}}
{{- if .Values.cloudEnvConfigMapName }}
envFrom:
  - configMapRef:
      name: {{ .Values.cloudEnvConfigMapName | quote }}
{{- end }}
{{- end }}

{{- define "aaos-builder.commonEnv" -}}
- name: WORKSPACE
{{- if or .Values.localRepoHostPath .Values.localRepoPvcName }}
  value: /workspace
{{- else }}
  value: {{ include "aaos-builder.pipelineMonorepoPath" . | quote }}
{{- end }}
- name: PIPELINE_REPO_ROOT
{{- if or .Values.localRepoHostPath .Values.localRepoPvcName }}
  value: {{ .Values.localRepoMountPath | quote }}
{{- else }}
  value: {{ include "aaos-builder.pipelineMonorepoPath" . | quote }}
{{- end }}
- name: AAOS_LUNCH_TARGET
  value: '{{ "{{" }}workflow.parameters.lunchTarget{{ "}}" }}'
- name: AAOS_REVISION
  value: '{{ "{{" }}workflow.parameters.androidRevision{{ "}}" }}'
- name: AAOS_CLEAN
  value: '{{ "{{" }}workflow.parameters.cleanBuild{{ "}}" }}'
- name: AAOS_BUILD_CTS
  value: '{{ "{{" }}workflow.parameters.buildCtsOnly{{ "}}" }}'
- name: AAOS_ARTIFACT_STORAGE_SOLUTION
  value: "GCS_BUCKET"
- name: STORAGE_BUCKET_BASE
  value: 'gs://{{ .Values.spec.cloudProject }}-aaos/Android/Builds/AAOS_Builder'
- name: WORKFLOW_NAME
  # Argo generateName (e.g., aaos-builder-6v79q). Storage step composes <date>-<time>_{workflow.name} at runtime.
  value: '{{ "{{" }}workflow.name{{ "}}" }}'
- name: STORAGE_LABELS
  value: '{{ "{{" }}workflow.parameters.storageLabels{{ "}}" }}'
- name: ENABLE_GEMINI_AI_ASSISTANT
  value: '{{ "{{" }}workflow.parameters.enableGeminiAiAssistant{{ "}}" }}'
{{- if not .Values.cloudEnvConfigMapName }}
- name: HORIZON_DOMAIN
  value: {{ .Values.spec.horizonDomain | quote }}
- name: CLOUD_PROJECT
  value: {{ .Values.spec.cloudProject | quote }}
- name: CLOUD_REGION
  value: {{ .Values.spec.cloudRegion | quote }}
- name: CLOUD_ZONE
  value: {{ .Values.spec.cloudZone | quote }}
{{- end }}
- name: GERRIT_CREDENTIALS_SECRET
  value: "jenkins-gerrit-http-password"
- name: GERRIT_USERNAME_KEY
  value: "username"
- name: GERRIT_PASSWORD_KEY
  value: "password"
- name: GERRIT_REPO_SYNC_JOBS
  value: '{{ "{{" }}workflow.parameters.parallelSyncJobs{{ "}}" }}'
- name: AAOS_CACHE_STORAGE_CLASS_NAME
  value: 'reclaimable-storage-class-android-{{ printf "{{=sprig.regexFind(\"[0-9]+\", workflow.parameters.androidRevision)}}" }}{{ printf "{{=sprig.regexFind(\"rpi\", workflow.parameters.lunchTarget) != \"\" ? \"-rpi\" : \"\"}}" }}'
- name: AAOS_CACHE_SIZE
  value: "1000Gi"
{{- end }}

{{- define "aaos-builder.androidVersionEnv" -}}
- name: ANDROID_VERSION
  value: '{{ "{{" }}inputs.parameters.sdkAndroidVersion{{ "}}" }}'
{{- end }}

{{- define "aaos-builder.sdkAndroidVersionEnv" -}}
- name: SDK_ANDROID_VERSION
  value: '{{ "{{" }}inputs.parameters.sdkAndroidVersion{{ "}}" }}'
{{- end }}
