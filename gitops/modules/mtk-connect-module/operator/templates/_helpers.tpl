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

{{/* Namespace of the MTK Connect server and of the controller. */}}
{{- define "mtk-connect-operator.namespace" -}}
{{- printf "%smtk-connect" .Values.config.namespacePrefix -}}
{{- end -}}

{{/* CRDs are cluster-wide: only the main environment installs them. */}}
{{- define "mtk-connect-operator.installCRDs" -}}
{{- not (default false .Values.config.isSubEnvironment) -}}
{{- end -}}

{{/* Namespaces where MTKConnectTestbench resources are watched, comma separated. */}}
{{- define "mtk-connect-operator.watchNamespaces" -}}
{{- $prefix := .Values.config.namespacePrefix -}}
{{- $list := list -}}
{{- range .Values.testbenchNamespaceSuffixes -}}
{{- $list = append $list (printf "%s%s" $prefix .) -}}
{{- end -}}
{{- join "," $list -}}
{{- end -}}
