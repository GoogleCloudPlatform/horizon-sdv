# Copyright 2026 Google LLC
#
# Modifications Copyright (c) 2026 Accenture, All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

{{/*
Template helpers for the ABFS chart.
*/}}

{{/* Chart name+version, sanitised for use as a label value (SemVer '+' is illegal). */}}
{{- define "abfs.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" -}}
{{- end -}}

{{/*
Common (non-selector) labels applied to every object's metadata and pod templates.
Excludes app.kubernetes.io/name + /component, which live in the per-component
selectorLabels so the selectors stay immutable. commonLabels lets operators inject
extra labels (cost allocation, mesh, etc.).
*/}}
{{- define "abfs.labels" -}}
helm.sh/chart: {{ include "abfs.chart" . }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: abfs
app.kubernetes.io/instance: {{ .Release.Name }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
{{- with .Values.commonLabels }}
{{ toYaml . }}
{{- end }}
{{- end -}}
