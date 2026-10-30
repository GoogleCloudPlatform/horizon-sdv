{{/*
Copyright (c) 2026 Accenture, All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
*/}}

{{/*
Main environment only — sub-environments must not deploy module controller resources.
*/}}
{{- define "cloud-workstations-core.mainEnv" -}}
{{- not (default false .Values.config.isSubEnvironment) -}}
{{- end -}}

