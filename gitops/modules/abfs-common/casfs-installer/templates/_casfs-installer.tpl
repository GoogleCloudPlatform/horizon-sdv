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
casfs installer shared by abfs-service-module, abfs-client-module-ubuntu and
abfs-client-module-cos. The consuming chart includes
"abfs-casfs-installer.configmap" and "abfs-casfs-installer.daemonset" with its
root context; everything is driven by .Values.casfs of that chart (see README.md).
*/}}

{{/*
Mode of the installer for the consuming chart:
  install  casfs.provider=daemonset: install casfs on the node (Ubuntu).
  image    casfs.provider=image and casfs.daemonset.enabled=true: only load the
           module the node image ships (COS).
  ""       nothing is rendered.
*/}}
{{- define "abfs-casfs-installer.mode" -}}
{{- $casfs := .Values.casfs | default dict -}}
{{- $ds := $casfs.daemonset | default dict -}}
{{- if eq ($casfs.provider | default "") "daemonset" -}}
install
{{- else if and (eq ($casfs.provider | default "") "image") $ds.enabled -}}
image
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
Source tarball. casfs.daemonset.sourceUrl is either the tarball itself (*.tar.gz)
or the directory that holds casfs-<casfs.daemonset.version>.tar.gz.
*/}}
{{- define "abfs-casfs-installer.sourceUrl" -}}
{{- $ds := .Values.casfs.daemonset -}}
{{- if hasSuffix ".tar.gz" $ds.sourceUrl -}}
{{- $ds.sourceUrl -}}
{{- else -}}
{{- printf "%s/casfs-%s.tar.gz" (trimSuffix "/" $ds.sourceUrl) (required "casfs.daemonset.version is required when casfs.daemonset.sourceUrl is a directory" $ds.version) -}}
{{- end -}}
{{- end -}}

{{/* Fails the render when the values of the consuming chart are incomplete. */}}
{{- define "abfs-casfs-installer.validate" -}}
{{- $ds := .Values.casfs.daemonset | default dict -}}
{{- if eq (include "abfs-casfs-installer.mode" .) "install" -}}
{{- if not $ds.sourceUrl -}}
{{- fail "casfs.provider=daemonset requires casfs.daemonset.sourceUrl (the casfs kernel-module source tarball)" -}}
{{- end -}}
{{- if $ds.aptRepository -}}
{{- if not $ds.aptProject -}}
{{- fail "casfs.daemonset.aptRepository requires casfs.daemonset.aptProject (the Artifact Registry project)" -}}
{{- end -}}
{{- if not $ds.aptLocation -}}
{{- fail "casfs.daemonset.aptRepository requires casfs.daemonset.aptLocation (the Artifact Registry location)" -}}
{{- end -}}
{{- if not $ds.version -}}
{{- fail "casfs.daemonset.aptRepository requires casfs.daemonset.version (the prebuilt casfs-kmod package version)" -}}
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
# Load the casfs kernel module on this node, then idle. Idempotent.
#   CASFS_MODE=install  Ubuntu: prebuilt casfs-kmod package when CASFS_APT_REPOSITORY
#                       is set, otherwise (or when it fails) a build from CASFS_SOURCE_URL.
#   CASFS_MODE=image    COS: the node image ships casfs; only modprobe it.
set -euo pipefail
log() { echo "[casfs-installer] $*"; }
KVER="$(uname -r)"
if [ -d /sys/module/casfs ]; then
  log "casfs already loaded (kernel ${KVER}); nothing to do"
  exec sleep infinity
fi

load_from_image() {
  log "loading casfs shipped in the node image (kernel ${KVER})"
  chroot /host /usr/bin/env PATH=/usr/sbin:/usr/bin:/sbin:/bin modprobe casfs || true
  if [ -d /sys/module/casfs ]; then
    return 0
  fi
  return 1
}

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
        CMD="find /tmp/ -maxdepth 1 -type f -name \"pool*\" -exec apt-get install -y \"./{}\" \\;"
        echo "Command: ${CMD}"
        eval "${CMD}"
        depmod -a "${KVER}"
        modprobe casfs && echo "Modprobe succeeded" || echo "Modprobe failed"
        CMD="find /tmp/ -maxdepth 1 -type f -name \"pool*\" -exec rm -rf \"./{}\" \\;"
        echo "Command: ${CMD}"
        eval "${CMD}"
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

case "${CASFS_MODE:-install}" in
  image)
    if load_from_image; then
      log "casfs loaded from the node image on kernel ${KVER}"
    else
      log "ERROR: the node image has no loadable casfs module for kernel ${KVER}" >&2
      exit 1
    fi
    ;;
  install)
    if [ -n "${CASFS_APT_REPOSITORY:-}" ] && install_from_apt; then
      log "casfs loaded from the Artifact Registry package on kernel ${KVER}"
    elif install_from_source; then
      log "casfs loaded after source build on kernel ${KVER}"
    else
      log "ERROR: casfs did not load from the package or from source on kernel ${KVER}" >&2
      exit 1
    fi
    ;;
  *)
    log "ERROR: unknown CASFS_MODE '${CASFS_MODE}'" >&2
    exit 1
    ;;
esac
exec sleep infinity
{{- end -}}
{{- end -}}

{{/* ConfigMap with the loader script. Arg: root context of the consuming chart. */}}
{{- define "abfs-casfs-installer.configmap" -}}
{{- if include "abfs-casfs-installer.mode" . }}
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
{{- $mode := include "abfs-casfs-installer.mode" . }}
{{- if $mode }}
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
            - name: CASFS_MODE
              value: {{ $mode | quote }}
            {{- if eq $mode "install" }}
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
