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
casfs installer shared by abfs-service-module and abfs-client-module-ubuntu.
The consuming chart includes
"abfs-casfs-installer.configmap" and "abfs-casfs-installer.daemonset" with its
root context; everything is driven by .Values.casfs of that chart (see README.md).
*/}}

{{/*
"true" when the consuming chart installs casfs on its nodes
(casfs.provider=daemonset, Ubuntu). For any other provider nothing is rendered:
a node image that ships casfs (COS) needs no installer.
*/}}
{{- define "abfs-casfs-installer.enabled" -}}
{{- $casfs := .Values.casfs | default dict -}}
{{- if eq ($casfs.provider | default "") "daemonset" -}}
true
{{- end -}}
{{- end -}}

{{/* Name of the DaemonSet and of its ConfigMap. */}}
{{- define "abfs-casfs-installer.name" -}}
{{- ((.Values.casfs.daemonset | default dict).name) | default "abfs-casfs-installer" -}}
{{- end -}}

{{/* Common labels; same content as "abfs.labels" of the ABFS charts. */}}
{{- define "abfs-casfs-installer.labels" -}}
helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
app.kubernetes.io/part-of: abfs
app.kubernetes.io/instance: {{ .Release.Name }}
{{- if .Chart.AppVersion }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
{{- end }}
{{- with .Values.commonLabels }}
{{ toYaml . }}
{{- end }}
app.kubernetes.io/name: abfs
app.kubernetes.io/component: casfs-installer
{{- end -}}

{{/*
Source tarball, the same rule for every module:
<casfs.daemonset.sourceUrl>/casfs-<casfs.daemonset.version>.tar.gz
*/}}
{{- define "abfs-casfs-installer.sourceUrl" -}}
{{- $ds := .Values.casfs.daemonset -}}
{{- printf "%s/casfs-%s.tar.gz" (trimSuffix "/" $ds.sourceUrl) $ds.version -}}
{{- end -}}

{{/* Fails the render when the values of the consuming chart are incomplete. */}}
{{- define "abfs-casfs-installer.validate" -}}
{{- $ds := .Values.casfs.daemonset | default dict -}}
{{- if include "abfs-casfs-installer.enabled" . -}}
{{- if not $ds.sourceUrl -}}
{{- fail "casfs.provider=daemonset requires casfs.daemonset.sourceUrl (the directory that holds the casfs source tarball)" -}}
{{- end -}}
{{- if hasSuffix ".tar.gz" $ds.sourceUrl -}}
{{- fail "casfs.daemonset.sourceUrl is the directory of the tarball; set the version in casfs.daemonset.version" -}}
{{- end -}}
{{- if not $ds.version -}}
{{- fail "casfs.provider=daemonset requires casfs.daemonset.version (the casfs version)" -}}
{{- end -}}
{{- if $ds.aptRepository -}}
{{- if not $ds.aptProject -}}
{{- fail "casfs.daemonset.aptRepository requires casfs.daemonset.aptProject (the Artifact Registry project)" -}}
{{- end -}}
{{- if not $ds.aptLocation -}}
{{- fail "casfs.daemonset.aptRepository requires casfs.daemonset.aptLocation (the Artifact Registry location)" -}}
{{- end -}}
{{- end -}}
{{- end -}}
{{- end -}}

{{/*
Loader script. The same text for every module; the DaemonSet passes the settings
as CASFS_* environment variables. Replaced as a whole by casfs.daemonset.script.
*/}}
{{- define "abfs-casfs-installer.script" -}}
{{- $ds := .Values.casfs.daemonset | default dict -}}
{{- if $ds.script -}}
{{ $ds.script }}
{{- else -}}
#!/usr/bin/env bash
# Install and load the casfs kernel module on this Ubuntu node, then idle. Idempotent.
# The prebuilt casfs-kmod package is tried when CASFS_APT_REPOSITORY is set;
# otherwise, or when it does not load, casfs is built from CASFS_SOURCE_URL.
set -euo pipefail
log() { echo "[casfs-installer] $*"; }
KVER="$(uname -r)"
if [ -d /sys/module/casfs ]; then
  log "casfs already loaded (kernel ${KVER}); nothing to do"
  exec sleep infinity
fi

install_from_apt() {
  local pkg artifacts
  pkg="casfs-kmod-${KVER}_${CASFS_VERSION}"
  artifacts="/tmp/casfs-apt-list.txt"
  log "Fetching casfs apt ${pkg} from ${CASFS_APT_PROJECT}/${CASFS_APT_LOCATION}/${CASFS_APT_REPOSITORY}"
  gcloud artifacts files list --project="${CASFS_APT_PROJECT}" --location="${CASFS_APT_LOCATION}" --repository="${CASFS_APT_REPOSITORY}" >> "${artifacts}" 2>&1
  if grep -Fq "pool/${pkg}" "${artifacts}"; then
    echo "Apt ${pkg} found."
    grep -F "pool/${pkg}" "${artifacts}" | awk '{print $1}' | while read -r a; do
      gcloud artifacts files download --project="${CASFS_APT_PROJECT}" --location="${CASFS_APT_LOCATION}" --repository="${CASFS_APT_REPOSITORY}" --destination=/host/tmp/ "${a}"
    done
    if ls /host/tmp/*"${pkg}"* 1> /dev/null 2>&1; then
      echo "Apt ${pkg} downloaded."
      chroot /host /bin/bash -euo pipefail -c '
        export DEBIAN_FRONTEND=noninteractive
        KVER="$(uname -r)"
        # The downloaded package files keep their repository path in the name (pool...).
        find /tmp -maxdepth 1 -type f -name "pool*casfs-kmod*" -print -exec apt-get install -y {} +
        depmod -a "${KVER}"
        modprobe casfs && echo "Modprobe succeeded" || echo "Modprobe failed"
        find /tmp -maxdepth 1 -type f -name "pool*casfs-kmod*" -delete
      '
    else
      echo "Apt ${pkg} not downloaded."
    fi
  else
    echo "Apt ${pkg} not found."
  fi

  if [ -d /sys/module/casfs ]; then
    return 0
  fi
  return 1
}

install_from_source() {
  log "building casfs from source ${CASFS_SOURCE_URL}"
  gcloud storage cp "${CASFS_SOURCE_URL}" /host/tmp/casfs-src.tar.gz
  log "building + loading casfs for kernel ${KVER} (host toolchain)"
  chroot /host /bin/bash -euo pipefail -c '
    export DEBIAN_FRONTEND=noninteractive
    KVER="$(uname -r)"
    if ! command -v gcc >/dev/null 2>&1 || ! command -v make >/dev/null 2>&1; then
      apt-get update -qq && apt-get install -y -qq build-essential
    fi
    rm -rf /tmp/casfs-build && mkdir -p /tmp/casfs-build
    tar xf /tmp/casfs-src.tar.gz -C /tmp/casfs-build --strip-components=1
    cd /tmp/casfs-build
    make ko KERNEL_RELEASE="${KVER}"
    install -D -m 0644 casfs.ko "/lib/modules/${KVER}/kernel/fs/casfs/casfs.ko"
    depmod "${KVER}"
    modprobe casfs
  '
  if [ -d /sys/module/casfs ]; then
    return 0
  fi
  return 1
}

if [ -n "${CASFS_APT_REPOSITORY:-}" ] && install_from_apt; then
  log "casfs loaded from the Artifact Registry package on kernel ${KVER}"
elif install_from_source; then
  log "casfs loaded after source build on kernel ${KVER}"
else
  log "ERROR: casfs did not load from the package or from source on kernel ${KVER}" >&2
  exit 1
fi
exec sleep infinity
{{- end -}}
{{- end -}}

{{/* ConfigMap with the loader script. Arg: root context of the consuming chart. */}}
{{- define "abfs-casfs-installer.configmap" -}}
{{- if include "abfs-casfs-installer.enabled" . }}
{{- include "abfs-casfs-installer.validate" . }}
apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ include "abfs-casfs-installer.name" . }}
  labels:
    {{- include "abfs-casfs-installer.labels" . | nindent 4 }}
data:
  install.sh: |
    {{- include "abfs-casfs-installer.script" . | nindent 4 }}
{{- end }}
{{- end -}}

{{/*
DaemonSet that runs the loader script on every node selected by
casfs.daemonset.nodeSelector / tolerations (default: the chart's global
nodeSelector / tolerations). Arg: root context of the consuming chart.
*/}}
{{- define "abfs-casfs-installer.daemonset" -}}
{{- if include "abfs-casfs-installer.enabled" . }}
{{- include "abfs-casfs-installer.validate" . }}
{{- $ds := .Values.casfs.daemonset }}
{{- $image := .Values.image | default dict }}
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: {{ include "abfs-casfs-installer.name" . }}
  labels:
    {{- include "abfs-casfs-installer.labels" . | nindent 4 }}
  {{- with .Values.commonAnnotations }}
  annotations:
    {{- toYaml . | nindent 4 }}
  {{- end }}
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: abfs
      app.kubernetes.io/component: casfs-installer
  updateStrategy:
    type: RollingUpdate
  template:
    metadata:
      labels:
        {{- include "abfs-casfs-installer.labels" . | nindent 8 }}
        {{- with .Values.podLabels }}
        {{- toYaml . | nindent 8 }}
        {{- end }}
      annotations:
        checksum/script: {{ include "abfs-casfs-installer.script" . | sha256sum }}
        {{- with .Values.podAnnotations }}
        {{- toYaml . | nindent 8 }}
        {{- end }}
    spec:
      {{- with .Values.imagePullSecrets }}
      imagePullSecrets:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      # hostPID + hostNetwork: load into the host kernel and use the node's apt mirrors
      # and metadata (node service account) for the package or source download.
      hostPID: true
      hostNetwork: true
      {{- with .Values.priorityClassName }}
      priorityClassName: {{ . }}
      {{- end }}
      {{- with ($ds.nodeSelector | default .Values.nodeSelector) }}
      nodeSelector:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      {{- with ($ds.tolerations | default .Values.tolerations) }}
      tolerations:
        {{- toYaml . | nindent 8 }}
      {{- end }}
      containers:
        - name: casfs-installer
          image: {{ required "casfs.daemonset.image is required" $ds.image | quote }}
          imagePullPolicy: {{ $ds.imagePullPolicy | default $image.pullPolicy | default "IfNotPresent" }}
          # Privileged + chroot to the host: installs and inserts a kernel module.
          securityContext:
            privileged: true
          command: ["/bin/bash", "/scripts/install.sh"]
          env:
            - name: CASFS_SOURCE_URL
              value: {{ include "abfs-casfs-installer.sourceUrl" . | quote }}
            {{- if $ds.aptRepository }}
            - name: CASFS_VERSION
              value: {{ $ds.version | quote }}
            - name: CASFS_APT_PROJECT
              value: {{ $ds.aptProject | quote }}
            - name: CASFS_APT_LOCATION
              value: {{ $ds.aptLocation | quote }}
            - name: CASFS_APT_REPOSITORY
              value: {{ $ds.aptRepository | quote }}
            {{- end }}
          volumeMounts:
            - name: script
              mountPath: /scripts
              readOnly: true
            - name: host
              mountPath: /host
          {{- with $ds.resources }}
          resources:
            {{- toYaml . | nindent 12 }}
          {{- end }}
      volumes:
        - name: script
          configMap:
            name: {{ include "abfs-casfs-installer.name" . }}
            defaultMode: 0555
        - name: host
          hostPath:
            path: /
{{- end }}
{{- end -}}
