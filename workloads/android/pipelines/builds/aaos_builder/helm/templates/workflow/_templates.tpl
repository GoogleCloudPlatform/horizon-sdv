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
Workflow template fragments for aaos-builder.
*/ -}}

{{- define "aaos-builder.templates" -}}
{{- $ctx := . -}}
{{ include "aaos-builder.template.main" $ctx }}
{{ include "aaos-builder.template.compute-vars" $ctx }}
{{ include "aaos-builder.template.check-aaos-image" $ctx }}
{{ include "aaos-builder.template.fetch-pipeline" $ctx }}
{{ include "aaos-builder.template.clean" $ctx }}
{{ include "aaos-builder.template.init" $ctx }}
{{ include "aaos-builder.template.build" $ctx }}
{{ include "aaos-builder.template.gemini-review" $ctx }}
{{ include "aaos-builder.template.storage" $ctx }}
{{ include "aaos-builder.template.fail-workflow" $ctx }}
{{- end }}
