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
Main environment only: one mirror volume per cluster (sub-environments are out of scope for v1).
*/}}
{{- define "storage-mirror-module.mainEnv" -}}
{{- not (default false .Values.config.isSubEnvironment) -}}
{{- end -}}

{{- define "storage-mirror-module.horizonNS" -}}
{{- printf "%shorizon" .Values.config.namespacePrefix -}}
{{- end -}}

{{- define "storage-mirror-module.workflowsNS" -}}
{{- printf "%sworkflows" .Values.config.namespacePrefix -}}
{{- end -}}

{{- define "storage-mirror-module.jenkinsNS" -}}
{{- printf "%sjenkins" .Values.config.namespacePrefix -}}
{{- end -}}

{{- define "storage-mirror-module.horizonApiNS" -}}
{{- printf "%shorizon-api" .Values.config.namespacePrefix -}}
{{- end -}}

{{- define "storage-mirror-module.eventsNS" -}}
{{- printf "%sargo-events" .Values.config.namespacePrefix -}}
{{- end -}}

{{- define "storage-mirror-module.apiHost" -}}
{{- printf "storage-mirror-module.%s.svc.cluster.local" (include "storage-mirror-module.horizonNS" .) -}}
{{- end -}}
