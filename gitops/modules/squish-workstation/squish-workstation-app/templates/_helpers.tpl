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

{{/* Common labels for all Config Connector resources. */}}
{{- define "squish-workstation-app.labels" -}}
app.kubernetes.io/name: {{ .Values.parentModuleName }}
horizon-sdv.io/module: {{ .Values.parentModuleName }}
{{- end -}}

{{/* Workstation VM service account e-mail. */}}
{{- define "squish-workstation-app.serviceAccountEmail" -}}
{{ .Values.squish.serviceAccountId }}@{{ .Values.gcpProjectId }}.iam.gserviceaccount.com
{{- end -}}

{{/* Full image URI of the Squish workstation image (pushed by squish-workstation-init). */}}
{{- define "squish-workstation-app.image" -}}
{{ .Values.squish.region }}-docker.pkg.dev/{{ .Values.gcpProjectId }}/{{ .Values.squish.artifactRepository }}/{{ .Values.squish.imageName }}:{{ .Values.squish.imageTag }}
{{- end -}}
