# Copyright 2026 Google LLC
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

IMPORTANT — selector labels are immutable on a live Deployment/StatefulSet. The
`*.selectorLabels` helpers below must stay byte-stable across upgrades; only the
non-selector labels in `abfs.labels` may grow.

NAMING IS FIXED BY DESIGN — do NOT switch to release-name-prefixed names
(`{{ .Release.Name }}-...` / fullnameOverride). ABFS is a per-namespace singleton
data plane and the fixed names are load-bearing in three places:
  1. External IAM + scheduling — the fixed object/KSA names are referenced by infra/
     (KCC) IAM and by the dedicated node-pool wiring; renaming them silently breaks
     the data plane.
  2. Internal identity — uploader pod hostnames (<namePrefix>-<ordinal>) must equal
     the pusher-config pusher names, and the server is reached by a fixed DNS name.
  3. Clean upgrades — fixed names let `helm upgrade` update the SAME objects in place
     (Server-Side Apply owns the fields), so new ABFS releases roll without orphans.
Multiple deployments are achieved per-namespace/project via scripts/render.sh
instances, NOT by multiple Helm releases in one namespace. New ABFS versions are
shipped by bumping image.tag / image.digest, not by changing names.
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

{{/* Server names / selectors (selector is IMMUTABLE — do not change). */}}
{{- define "abfs.server.name" -}}abfs-server{{- end -}}
{{- define "abfs.server.selectorLabels" -}}
app.kubernetes.io/name: abfs
app.kubernetes.io/component: server
{{- end -}}

{{/* Uploader names / selectors (selector is IMMUTABLE — do not change). */}}
{{- define "abfs.uploader.name" -}}{{ .Values.uploader.namePrefix }}{{- end -}}
{{- define "abfs.uploader.selectorLabels" -}}
app.kubernetes.io/name: abfs
app.kubernetes.io/component: uploader
{{- end -}}

{{/* UI names / selectors (selector is IMMUTABLE — do not change). */}}
{{- define "abfs.ui.name" -}}abfs-ui{{- end -}}
{{- define "abfs.ui.selectorLabels" -}}
app.kubernetes.io/name: abfs
app.kubernetes.io/component: ui
{{- end -}}

{{/*
The ABFS runtime service account email — the single licensed SA the dedicated ABFS
node pool runs AS (its node service account), which MUST appear in the license's
allowed_service_accounts. Under the GCE-metadata identity model the workloads take
their Google identity from the node SA (NOT Workload Identity), so this is set on the
node pool (--service-account, see docs/02 §1b) and granted roles in infra/12; here it
is informational (NOTES). runtimeServiceAccountName is the local part; the email is
built with projectId. An empty or unrendered REPLACE_* value yields "".
*/}}
{{- define "abfs.runtimeServiceAccount" -}}
{{- $name := .Values.runtimeServiceAccountName | default "" -}}
{{- if and $name (not (hasPrefix "REPLACE_" $name)) -}}
{{- $name }}@{{ .Values.projectId }}.iam.gserviceaccount.com
{{- end -}}
{{- end -}}

{{/* In-cluster server endpoint used by uploaders + bootstrap job. */}}
{{- define "abfs.server.endpoint" -}}{{ include "abfs.server.name" . }}.{{ .Release.Namespace }}.svc.cluster.local:{{ .Values.server.service.port }}{{- end -}}

{{/*
ABFS client transport/auth flags for in-cluster client commands (uploader, bootstrap
Job). The server serves plaintext + (default) no client auth, but ABFS clients default
to TLS — so emit --disable-tls=true unless client.tls, plus --auth-type. Two forms:
  abfs.clientFlags     -> single space-separated string (for bash command lines)
  abfs.clientArgsYaml  -> YAML "- flag" list items (for container args:, use with nindent)
*/}}
{{- define "abfs.clientFlags" -}}
{{- if not .Values.client.tls }}--disable-tls=true {{ end }}--auth-type {{ .Values.client.authType | default "none" }}
{{- end -}}
{{- define "abfs.clientArgsYaml" -}}
{{- if not .Values.client.tls }}
- --disable-tls=true
{{- end }}
- --auth-type
- {{ .Values.client.authType | default "none" | quote }}
{{- end -}}

{{/*
Guard: are the ABFS workloads licensed/enabled? The license itself lives in the
dedicated node pool's instance metadata (see docs/02 §1b), NOT in the chart; this is
a simple boolean the operator flips to true once that licensed pool exists, so the
server/uploaders/bootstrap render only when the data plane can actually serve.
*/}}
{{- define "abfs.licensed" -}}
{{- if .Values.licensed -}}true{{- end -}}
{{- end -}}

{{/*
Fully-qualified image reference. Pins by digest when image.digest is set
(repository@sha256:...), otherwise repository:tag. Tag falls back to the chart
appVersion so the default tracks Chart.yaml.
*/}}
{{- define "abfs.image" -}}
{{- $img := .Values.image -}}
{{- if $img.digest -}}
{{- printf "%s@%s" $img.repository $img.digest -}}
{{- else -}}
{{- printf "%s:%s" $img.repository ($img.tag | default .Chart.AppVersion) -}}
{{- end -}}
{{- end -}}

{{/*
Probe builder. Usage:
  {{- include "abfs.probe" (dict "cfg" .Values.server.probes.readiness "port" $port) | nindent 12 }}
Supported cfg.type: tcpSocket (default), grpc, httpGet (cfg.path), exec (cfg.command).
*/}}
{{- define "abfs.probe" -}}
{{- $cfg := .cfg -}}
{{- $port := .port -}}
{{- $type := $cfg.type | default "tcpSocket" -}}
{{- if eq $type "grpc" }}
grpc:
  port: {{ $port }}
{{- else if eq $type "httpGet" }}
httpGet:
  path: {{ $cfg.path | default "/" }}
  port: {{ $port }}
{{- else if eq $type "exec" }}
exec:
  command:
    {{- toYaml $cfg.command | nindent 4 }}
{{- else }}
tcpSocket:
  port: {{ $port }}
{{- end }}
{{- with $cfg.initialDelaySeconds }}
initialDelaySeconds: {{ . }}
{{- end }}
periodSeconds: {{ $cfg.periodSeconds | default 10 }}
timeoutSeconds: {{ $cfg.timeoutSeconds | default 1 }}
failureThreshold: {{ $cfg.failureThreshold | default 3 }}
successThreshold: {{ $cfg.successThreshold | default 1 }}
{{- end -}}

{{/*
GOMAXPROCS env (emitted as an env list item) sourced from the pod's CPU request via the
downward API, rounded UP to an integer (min 1). ABFS binaries are Go; without this, Go sets
GOMAXPROCS to the node's core count and is then CPU-throttled against the pod's CPU cgroup.
With requests.cpu sized to the node and no CPU limit (see values: no cpu in resources.limits),
this lets ABFS use all the cores it is given. Arg: root context.
*/}}
{{- define "abfs.gomaxprocsEnv" -}}
- name: GOMAXPROCS
  valueFrom:
    resourceFieldRef:
      resource: requests.cpu
      divisor: "1"
{{- end -}}

{{/*
wait-for-casfs initContainer (emitted as a list item). Blocks until the casfs kernel
module is loaded on the node, so ABFS never starts on a node without it. /sys/module
reflects the HOST kernel and is visible to a normal container, so this needs no
privilege/hostPID. In daemonset mode it waits for the abfs-casfs-installer DaemonSet;
in image mode it's a fast verify that fails clearly if the node image lacks casfs.
Rendered when .Values.casfs.requireReady. Arg: root context.
*/}}
{{- define "abfs.waitForCasfsInit" -}}
- name: wait-for-casfs
  image: {{ include "abfs.image" . }}
  imagePullPolicy: {{ .Values.image.pullPolicy }}
  command:
    - sh
    - -c
    - |
      end=$(( $(date +%s) + {{ .Values.casfs.requireReadyTimeoutSeconds | default 300 }} ))
      until [ -d /sys/module/casfs ]; do
        if [ "$(date +%s)" -ge "$end" ]; then
          echo "casfs kernel module not loaded on this node (casfs.provider={{ .Values.casfs.provider }})." >&2
          echo "image mode: the node image must ship casfs. daemonset mode: check the abfs-casfs-installer DaemonSet and that the pool has ENFORCE_SIGNED_MODULES (docs/02 #1b)." >&2
          exit 1
        fi
        sleep 2
      done
      echo "casfs kernel module is loaded."
  {{- with .Values.restrictedSecurityContext }}
  securityContext:
    {{- toYaml . | nindent 4 }}
  {{- end }}
  resources:
    requests:
      cpu: 10m
      memory: 32Mi
    limits:
      memory: 64Mi
{{- end -}}
