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

{{/*
Squish settings passed to both child charts, with region and network resolved from the
Module Manager config (config.region, config.network, config.subnetwork) where available.
*/}}
{{- define "squish-workstation.settings" -}}
{{- $config := .Values.config | default dict -}}
{{- $resolved := dict
  "region" (.Values.squish.region | default $config.region | default "")
  "network" ($config.network | default .Values.squish.network)
  "subnetwork" ($config.subnetwork | default .Values.squish.subnetwork)
-}}
{{- merge $resolved (deepCopy .Values.squish) | toYaml -}}
{{- end -}}
