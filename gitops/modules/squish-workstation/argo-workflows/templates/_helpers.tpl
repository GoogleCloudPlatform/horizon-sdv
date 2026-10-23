{{- /*
Copyright (c) 2026 The Qt Company Ltd., All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

        http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/ -}}

{{/* Default horizon-gnome base image (published by the Jenkins Cloud Workstations image jobs). */}}
{{- define "squish-workstation-wf.baseImage" -}}
{{- .Values.squish.build.baseImage | default (printf "%s-docker.pkg.dev/%s/horizon-sdv/cloud-ws-images/horizon-gnome:latest" .Values.squish.region .Values.gcpProjectId) -}}
{{- end -}}

{{/* true when git credentials come from prepare-pipeline-git-creds (scm.authMethod app or userpass). */}}
{{- define "squish-workstation-wf.umbrellaCreds" -}}
{{- if or (eq .Values.scmAuthMethod "app") (eq .Values.scmAuthMethod "userpass") }}true{{ end -}}
{{- end -}}

{{/* Argo git artifact HTTPS credentials (same semantics as aaos-builder-runtime-image). */}}
{{- define "squish-workstation-wf.gitArtifactCreds" -}}
{{- if include "squish-workstation-wf.umbrellaCreds" . }}
usernameSecret:
  name: "{{ "{{" }}workflow.uid{{ "}}" }}-pipeline-git-creds"
  key: username
passwordSecret:
  name: "{{ "{{" }}workflow.uid{{ "}}" }}-pipeline-git-creds"
  key: password
{{- else if .Values.sourceRepoSecret }}
usernameSecret:
  name: {{ .Values.sourceRepoSecret | quote }}
  key: username
passwordSecret:
  name: {{ .Values.sourceRepoSecret | quote }}
  key: password
{{- end }}
{{- end -}}
