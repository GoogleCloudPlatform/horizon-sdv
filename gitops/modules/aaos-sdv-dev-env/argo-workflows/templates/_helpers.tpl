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

Description:
Helm helpers for the aaos-sdv-dev-env Argo Workflows chart.
*/ -}}

{{- define "aaos-sdv-dev-env.workflowServiceAccountName" -}}
{{- if .Values.spec.useElevatedWorkflowIam -}}
workflow-executor-elevated
{{- else -}}
{{- .Values.spec.serviceAccountName | default "workflow-executor" -}}
{{- end -}}
{{- end -}}

{{/* Artifact Registry host for the deployment region. */}}
{{- define "aaos-sdv-dev-env.registry" -}}
{{- printf "%s-docker.pkg.dev" .Values.gcpRegion -}}
{{- end -}}

{{/*
Registry cache repository passed to docker/docker-bake.hcl via CACHE_REPO.
*/}}
{{- define "aaos-sdv-dev-env.cacheRepo" -}}
{{- printf "%s/%s/%s" (include "aaos-sdv-dev-env.registry" .) .Values.gcpProjectId .Values.spec.cacheArtifactPathName -}}
{{- end -}}

{{/*
Image produced by this pipeline. Mirrors how common-docker-image-build composes
its destination: <region>-docker.pkg.dev/<project>/<dockerArtifactPathName>:<imageTag>.
*/}}
{{- define "aaos-sdv-dev-env.outputImage" -}}
{{- printf "%s/%s/%s:%s" (include "aaos-sdv-dev-env.registry" .) .Values.gcpProjectId .Values.spec.dockerArtifactPathName .Values.spec.imageTag -}}
{{- end -}}
