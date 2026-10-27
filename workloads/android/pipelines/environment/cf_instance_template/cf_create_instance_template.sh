#!/usr/bin/env bash

# Copyright (c) 2024-2026 Accenture, All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#        http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
# Description:
# Create the Cuttlefish boilerplate template instance for use with Jenkins
# GCE plugin. Global image delete, global instance template delete, zonal
# instance stop/delete, and orphan **Packer** zonal boot disks use Compute Engine REST
# via the companion script
# cf_compute_rest.py (same directory). The shell obtains a cloud-platform OAuth
# token from the GCE metadata server (see gcp_metadata_access_token) and passes
# it per invocation as CF_COMPUTE_REST_TOKEN; see the module docstring in
# cf_compute_rest.py for subcommands, stdout lines (delete-global-image), and
# exit codes. OS Login SSH key pruning uses the same script (prune-os-login-ssh-keys).
# Non-compute gcloud may still be used for local defaults (project, bundled Python path).
#
# To run locally without metadata token, set PROJECT and use ADC-capable tooling
# for kubectl/Packer; REST cleanup paths may no-op with a warning.
#
# From command line, such as Google Cloud Shell, create templates for all
# versions of android-cuttlefish host tools/packages:
#
#  CUTTLEFISH_REVISION=v1.55.3 ./cf_create_instance_template.sh && \
#  CUTTLEFISH_REVISION=main ./cf_create_instance_template.sh
#
# The following environment variables can be set from the shell, Jenkins, or Argo.
# Default literals are assigned in exactly one place: the section banner
# "Environment defaults (authoritative — single place for default literals; override via env)" below.
# Do not restate default strings or versions in this header. When changing CI-facing defaults, also
# update groovy/job*.yaml / Helm values.yaml where those are the entrypoints.
#
#  - ADDITIONAL_NETWORKING: Extra Packer/network flags (ARM64 bare metal often needs nic-type=IDPF).
#  - CURL_UPDATE_COMMAND: Optional shell command run during host init to refresh curl (often empty on Ubuntu).
#  - CUSTOM_VM_TYPE / CUSTOM_CPU / CUSTOM_MEMORY: Custom machine shape when MACHINE_TYPE is unset.
#  - CUTTLEFISH_REVISION: android-cuttlefish branch or tag to build.
#  - CUTTLEFISH_URL: android-cuttlefish Git clone URL.
#  - CUTTLEFISH_POST_COMMAND: Optional command after checkout in that repo.
#  - BOOT_DISK_SIZE / BOOT_DISK_TYPE: Packer builder boot disk size and type.
#  - DEFAULT_USER: Linux account on the baked image; instance metadata jenkins-user.
#  - JAVA_VERSION: Apt JDK package (temurin-* adds Adoptium apt). Use temurin-21-jdk on Debian; Ubuntu often uses openjdk-21-jdk-headless.
#  - MACHINE_TYPE: GCE machine type for Packer build and template (or leave unset and set CUSTOM_*).
#  - MAX_RUN_DURATION: Published Cuttlefish **instance template** VM max run (default **12h** in script when unset;
#        Jenkins / Argo `maxRunDuration` also default 12h). **Not** the Packer builder VM (see PACKER_BUILD_MAX_RUN_DURATION).
#        Use 0 to omit max-run cap in the published template (see script / KCC spec).
#  - PACKER_BUILD_MAX_RUN_DURATION: Ephemeral **Packer builder** GCE VM max run only (default **4h** when unset).
#        **Do not confuse** with **`MAX_RUN_DURATION`**: the instance template governs long-lived Cuttlefish test VMs; the Packer builder exists only for the image bake.
#  - PACKER_USE_IAP / PACKER_SSH_TIMEOUT / PACKER_IAP_TUNNEL_LAUNCH_WAIT: Packer googlecompute SSH / IAP tuning.
#  - Argo Workflows: WorkflowTemplate spec.onExit runs ./cf_create_instance_template.sh orphan-disks in a
#        separate pod (prepare-pipeline-git-creds first when umbrella SCM auth is app/userpass). That path
#        deletes every **unattached** packer-* disk in ZONE (any size), not only the current BOOT_DISK_SIZE.
#  - NAMESPACE: Kubernetes namespace for the SSH private key Secret (often jenkins).
#  - NETWORK / SUBNET / REGION / ZONE: GCE network and placement for Packer and template.
#  - NODEJS_VERSION: Node version installed via nvm on the host image.
#  - OS_PROJECT / OS_VERSION: GCE image project and image name for the Packer source VM.
#  - PROJECT: GCP project (defaults from gcloud config when unset).
#  - REPO_USERNAME / REPO_PASSWORD: HTTPS credentials when CUTTLEFISH_URL is private.
#  - SERVICE_ACCOUNT: GCE service account email for instances created from the template.
#  - SSH_PRIVATE_KEY_NAME / SSH_PUBLIC_KEY_FILENAME: Kubernetes Secret key pair for template SSH (see README for shape).
#  - CUTTLEFISH_INSTANCE_NAME: Template name prefix (empty derives from CUTTLEFISH_REVISION).
#  - UPDATE_SSH_AUTHORIZED_KEYS: true to run stage 2 (metadata refresh only).
#  - WORKFLOWS_NAMESPACE: Namespace for KCC ComputeInstanceTemplate CRs.
#  - KCC_INSTANCE_TEMPLATE_DELETE_TIMEOUT: kubectl delete --timeout when dropping the CR before apply or in stage 3.
#  - COMPUTE_IMAGE_DELETE_DEBUG: true logs token identity (tokeninfo) before REST image delete (stderr only).
#
# The following arguments are optional and recommended run without args:

#  -h|--help :     - Print usage
#  1 : Run stage 1 - Build Cuttlefish image with Packer and create
#                    instance template from the baked image.
#  2 : Run stage 2 - Refresh SSH authorized_keys metadata on template
#                    without rebuilding image. Preflight: global disk image + GCE instance
#                    template must exist (Compute REST); KCC CR may be missing and will be applied.
#  3 : Run stage 3 - Delete instances, templates, and images for **this** target only
#        (same scoped teardown as partial failure cleanup; no namespace-wide CIT delete).
#  No args:          run stage 1.
# Include common functions and variables.
# shellcheck disable=SC1091
source "$(dirname "${BASH_SOURCE[0]}")"/cf_environment.sh "$0"

# -----------------------------------------------------------------------------
# Logging
# -----------------------------------------------------------------------------
GREEN='\033[1;32m'
RED='\033[1;31m'
ORANGE='\033[0;33m'
NC='\033[0m'
SCRIPT_NAME=$(basename "$0")

# -----------------------------------------------------------------------------
# Environment defaults (authoritative — single place for default literals; override via env)
# -----------------------------------------------------------------------------
# All ${VAR:-default} literals for this script are set in this block only (not in the header above).
# android-cuttlefish revisions can be of the form v1.7.0, main etc.
ADDITIONAL_NETWORKING=${ADDITIONAL_NETWORKING:-}
[ -n "${ADDITIONAL_NETWORKING}" ] && ADDITIONAL_NETWORKING=",${ADDITIONAL_NETWORKING}"
BOOT_DISK_SIZE=${BOOT_DISK_SIZE:-500GB}
BOOT_DISK_SIZE=$(echo "${BOOT_DISK_SIZE}" | awk '{print toupper($0)}' | xargs)
BOOT_DISK_TYPE=${BOOT_DISK_TYPE:-pd-balanced}
CURL_UPDATE_COMMAND=${CURL_UPDATE_COMMAND:-}
CUTTLEFISH_INSTANCE_NAME=${CUTTLEFISH_INSTANCE_NAME:-cuttlefish-vm}
CUTTLEFISH_INSTANCE_NAME=$(echo "${CUTTLEFISH_INSTANCE_NAME}" | awk '{print tolower($0)}' | xargs)
CUTTLEFISH_REVISION=${CUTTLEFISH_REVISION:-v1.55.3}
CUTTLEFISH_REVISION=$(echo "${CUTTLEFISH_REVISION}" | xargs)
CUTTLEFISH_URL=${CUTTLEFISH_URL:-https://github.com/google/android-cuttlefish.git}
CUTTLEFISH_URL=$(echo "${CUTTLEFISH_URL}" | xargs)
CUTTLEFISH_POST_COMMAND=${CUTTLEFISH_POST_COMMAND:-}
DEFAULT_USER=${DEFAULT_USER:-jenkins}
JAVA_VERSION=${JAVA_VERSION:-temurin-21-jdk}
MACHINE_TYPE=${MACHINE_TYPE:-}
MACHINE_TYPE=$(echo "${MACHINE_TYPE}" | xargs)
# Cuttlefish VMs created from the published instance template (default 12h; align with Jenkins / Helm maxRunDuration).
MAX_RUN_DURATION=${MAX_RUN_DURATION:-12h}
PACKER_USE_IAP=${PACKER_USE_IAP:-true}
PACKER_SSH_TIMEOUT=${PACKER_SSH_TIMEOUT:-15m}
PACKER_IAP_TUNNEL_LAUNCH_WAIT=${PACKER_IAP_TUNNEL_LAUNCH_WAIT:-300}
# hashicorp/googlecompute Packer plugin version bounds (overridable; see TAA-1839).
# Rendered into a generated plugins.pkr.hcl at build time, not hard-coded in the HCL.
#   MIN >= 1.2.3: needed for max_run_duration_in_seconds / instance_termination_action.
#   MAX <  1.2.5: v1.2.5 SIGSEGVs in StepImportOSLoginSSHKey on Argo read-only /workspace.
PACKER_GOOGLECOMPUTE_VERSION_MIN=$(echo "${PACKER_GOOGLECOMPUTE_VERSION_MIN:-1.2.3}" | xargs)
PACKER_GOOGLECOMPUTE_VERSION_MAX=$(echo "${PACKER_GOOGLECOMPUTE_VERSION_MAX:-1.2.5}" | xargs)
NAMESPACE=${NAMESPACE:-jenkins}
NETWORK=${NETWORK:-sdv-network}
NODEJS_VERSION=${NODEJS_VERSION:-20.9.0}
NODEJS_VERSION=$(echo "${NODEJS_VERSION}" | xargs)
OS_PROJECT=${OS_PROJECT:-debian-cloud}
OS_PROJECT=$(echo "${OS_PROJECT}" | xargs)
OS_VERSION=${OS_VERSION:-debian-12-bookworm-v20260114}
OS_VERSION=$(echo "${OS_VERSION}" | xargs)
PROJECT=${PROJECT:-$(gcloud config list --format 'value(core.project)'|head -n 1)}
REPO_USERNAME=${REPO_USERNAME:-}
REPO_USERNAME=$(echo "${REPO_USERNAME}" | xargs)
REPO_PASSWORD=${REPO_PASSWORD:-}
REPO_PASSWORD=$(echo "${REPO_PASSWORD}" | xargs)
SERVICE_ACCOUNT=${SERVICE_ACCOUNT:-$(gcloud projects describe "${PROJECT}" --format='get(projectNumber)')-compute@developer.gserviceaccount.com}
SSH_PRIVATE_KEY_NAME=${SSH_PRIVATE_KEY_NAME:-jenkins-cuttlefish-vm-ssh-private-key}
SSH_PUBLIC_KEY_FILENAME=${SSH_PUBLIC_KEY_FILENAME:-jenkins_private_key.pub}
UPDATE_SSH_AUTHORIZED_KEYS=${UPDATE_SSH_AUTHORIZED_KEYS:-false}
WORKFLOWS_NAMESPACE=${WORKFLOWS_NAMESPACE:-workflows}

IMAGE="projects/${OS_PROJECT}/global/images/${OS_VERSION}"

# Define architecture based on OS_VERSION as this will always include arch for arm.
if [[ "$OS_VERSION" == *arm64* ]]; then
    ARCHITECTURE="ARM64"
    VM_SUFFIX="-arm64"
    REGION=${REGION:-${ARM64_REGION:-us-central1}}
    ZONE=${ZONE:-${ARM64_ZONE:-us-central1-b}}
    SUBNET=${SUBNET:-${ARM64_SUBNETWORK:-sdv-subnet-arm64}}
else
    ARCHITECTURE="X86_64"
    REGION=${REGION:-${CLOUD_REGION:-europe-west1}}
    ZONE=${ZONE:-${CLOUD_ZONE:-europe-west1-d}}
    SUBNET=${SUBNET:-sdv-subnet}
fi
if [ -z "${MACHINE_TYPE}" ]; then
    if [[ -z "${CUSTOM_VM_TYPE}" || -z "${CUSTOM_CPU}" || -z "${CUSTOM_MEMORY}" ]]; then
        echo -e "${RED}ERROR: MACHINE_TYPE or all of CUSTOM_VM_TYPE, CUSTOM_CPU, CUSTOM_MEMORY must be defined.${NC}"
        exit 1
    fi
fi
VM_SUFFIX=${VM_SUFFIX:-}

# -----------------------------------------------------------------------------
# Derived GCP / KCC resource names (stable Jenkins plugin expectations)
# -----------------------------------------------------------------------------
# Instance names can only include specific characters, drop '.' and replace paths in branch, '/' with '-'.
declare -r vm_base_instance=vm-"${OS_VERSION}"
declare -r vm_base_instance_template=instance-template-vm-"${OS_VERSION}"
declare cuttlefish_version=${CUTTLEFISH_REVISION//./}
cuttlefish_version=${cuttlefish_version//\//-}
declare cuttlefish_name=${CUTTLEFISH_INSTANCE_NAME//./-}
if [[ "${cuttlefish_name}" == "cuttlefish-vm" ]]; then
    # If name is default, append version.
    cuttlefish_name="${cuttlefish_name}"-"${cuttlefish_version}""${VM_SUFFIX}"
fi
declare -r vm_cuttlefish_image=image-"${cuttlefish_name}"
declare -r vm_cuttlefish_instance_template=instance-template-"${cuttlefish_name}"
declare -r vm_cuttlefish_instance="${cuttlefish_name}"
declare -r packer_template_path="${CF_SCRIPT_PATH}/packer/cuttlefish.pkr.hcl"
declare -r packer_provision_script_path="${CF_SCRIPT_PATH}/packer/provision_cf_host.sh"
# Compute Engine + OS Login REST; see module docstring in cf_compute_rest.py.
declare -r cf_compute_rest_py="${CF_SCRIPT_PATH}/cf_compute_rest.py"
# Canonical startup script is colocated with this pipeline's Helm chart so KCC
# and CI jobs embed the same contents.
declare -r startup_script_path="${CF_SCRIPT_PATH}/helm/files/refresh_authorized_keys.sh"

# Packer googlecompute often creates a zonal boot disk named packer-* with cleanup
# deferred until after imaging. If the build fails or the process is killed, that
# disk can remain unattached. Remove only: zone=ZONE, name^packer-, users empty,
# sizeGb = Packer boot disk (Compute REST; same token as other cf_compute_rest paths).
function cleanup_orphan_packer_boot_disks() {
    local size_gb="$1"
    if [ ! -f "${cf_compute_rest_py}" ] || ! command -v python3 >/dev/null 2>&1; then
        echo -e "${ORANGE}WARNING: Missing ${cf_compute_rest_py} or python3; skipped orphan Packer disk cleanup${NC}" >&2
        return 0
    fi
    local token
    if ! token="$(gcp_metadata_access_token 2>/dev/null)" || [ -z "${token}" ]; then
        echo -e "${ORANGE}WARNING: Metadata token unavailable; skipped orphan Packer disk cleanup${NC}" >&2
        return 0
    fi
    echo -e "${GREEN}[$SCRIPT_NAME] Post-Packer orphan disk scan (zone=${ZONE}, sizeGb=${size_gb})${NC}" >&2
    CF_COMPUTE_REST_TOKEN="${token}" python3 "${cf_compute_rest_py}" cleanup-orphan-packer-disks \
        "${PROJECT}" "${ZONE}" "${size_gb}" || true
}

# Same REST filter as cleanup_orphan_packer_boot_disks but any boot disk size (still
# unattached packer-* only). Used by Argo onExit so stale disks from other job sizes
# are removed when the workflow finishes or the main pod dies.
function cleanup_all_unattached_packer_disks_in_zone() {
    if [ ! -f "${cf_compute_rest_py}" ] || ! command -v python3 >/dev/null 2>&1; then
        echo -e "${ORANGE}WARNING: Missing ${cf_compute_rest_py} or python3; skipped orphan Packer disk cleanup${NC}" >&2
        return 0
    fi
    local token
    if ! token="$(gcp_metadata_access_token 2>/dev/null)" || [ -z "${token}" ]; then
        echo -e "${ORANGE}WARNING: Metadata token unavailable; skipped orphan Packer disk cleanup${NC}" >&2
        return 0
    fi
    echo -e "${GREEN}[$SCRIPT_NAME] Orphan Packer disk scan — all unattached packer-* (zone=${ZONE})${NC}" >&2
    CF_COMPUTE_REST_TOKEN="${token}" python3 "${cf_compute_rest_py}" cleanup-orphan-packer-disks \
        "${PROJECT}" "${ZONE}" "any" || true
}

# -----------------------------------------------------------------------------
# Signal / wait helpers
# -----------------------------------------------------------------------------
function terminate() {
    echo -e "${RED}CTRL+C: exit requested!${NC}"
    # Packer can leave unattached packer-* disks if the client disconnects mid-build.
    local _sig_sz
    if _sig_sz="$(boot_disk_size_gb 2>/dev/null)"; then
        cleanup_orphan_packer_boot_disks "${_sig_sz}" || true
    fi
    exit 1
}
trap terminate SIGINT

# Progress spinner. Wait for PID to complete.
function progress_spinner() {
    local -r spinner='-\|/'
    local i=0
    while sleep 0.1; do
        i=$(( (i + 1) % 4 ))
        # Only show spinner on local, save on console noise.
        if [ -z "${WORKSPACE}" ]; then
            # shellcheck disable=SC2059
            printf "\r${spinner:$i:1}"
        fi
        if ! ps -p "$1" > /dev/null; then
            break
        fi
    done
    printf "\r"
    wait "${1}"
    rc=$?
    if [ "${rc}" -ne 0 ]; then
        echo -e "${RED}Process $1 failed, exit.${NC}"
        delete_cuttlefish_publish_target_only || true
        exit "${rc}"
    fi
}

# Echo formatted output.
function echo_formatted() {
    echo -e "\r${GREEN}[$SCRIPT_NAME] $1${NC}"
}

# Echo environment variables.
function echo_environment() {
    echo_formatted "Environment variables:"
    echo "ARCHITECTURE=${ARCHITECTURE}"
    echo "ADDITIONAL_NETWORKING=${ADDITIONAL_NETWORKING}"
    echo "BOOT_DISK_SIZE=${BOOT_DISK_SIZE}"
    echo "BOOT_DISK_TYPE=${BOOT_DISK_TYPE}"
    echo "CURL_UPDATE_COMMAND=${CURL_UPDATE_COMMAND}"
    echo "CUSTOM_VM_TYPE=${CUSTOM_VM_TYPE}"
    echo "CUSTOM_CPU=${CUSTOM_CPU}"
    echo "CUSTOM_MEMORY=${CUSTOM_MEMORY}"
    echo "CUTTLEFISH_INSTANCE_NAME=${cuttlefish_name}"
    echo "CUTTLEFISH_REVISION=${CUTTLEFISH_REVISION}"
    echo "CUTTLEFISH_POST_COMMAND=${CUTTLEFISH_POST_COMMAND}"
    echo "CUTTLEFISH_URL=${CUTTLEFISH_URL}"
    echo "DEFAULT_USER=${DEFAULT_USER}"
    echo "IMAGE=${IMAGE}"
    echo "JAVA_VERSION=${JAVA_VERSION}"
    echo "NAMESPACE=${NAMESPACE}"
    echo "MACHINE_TYPE=${MACHINE_TYPE}"
    echo "MAX_RUN_DURATION=${MAX_RUN_DURATION}"
    if [ -n "${PACKER_BUILD_MAX_RUN_DURATION:-}" ]; then
        echo "PACKER_BUILD_MAX_RUN_DURATION=${PACKER_BUILD_MAX_RUN_DURATION}"
    else
        echo "PACKER_BUILD_MAX_RUN_DURATION=(unset; Packer builder default 4h)"
    fi
    echo "PACKER_USE_IAP=${PACKER_USE_IAP}"
    echo "PACKER_SSH_TIMEOUT=${PACKER_SSH_TIMEOUT}"
    echo "PACKER_IAP_TUNNEL_LAUNCH_WAIT=${PACKER_IAP_TUNNEL_LAUNCH_WAIT}"
    echo "PACKER_GOOGLECOMPUTE_VERSION_MIN=${PACKER_GOOGLECOMPUTE_VERSION_MIN}"
    echo "PACKER_GOOGLECOMPUTE_VERSION_MAX=${PACKER_GOOGLECOMPUTE_VERSION_MAX}"
    # The Packer version is set when the builder image is created
    # (docker_image_template/Dockerfile, ARG PACKER_VERSION), not by this job.
    # This prints the version that's actually in the image. Combined with the
    # version limits above, it shows the complete version information in the job log (TAA-1839).
    local packer_binary_version
    packer_binary_version="$(packer version 2>/dev/null | head -1)" || true
    echo "PACKER_VERSION=${packer_binary_version:-(packer not on PATH)}"
    echo "NETWORK=${NETWORK}"
    echo "NODEJS_VERSION=${NODEJS_VERSION}"
    echo "OS_PROJECT=${OS_PROJECT}"
    echo "OS_VERSION=${OS_VERSION}"
    echo "PROJECT=${PROJECT}"
    echo "REGION=${REGION}"
    echo "SERVICE_ACCOUNT=${SERVICE_ACCOUNT}"
    echo "SSH_PRIVATE_KEY_NAME=${SSH_PRIVATE_KEY_NAME}"
    echo "SSH_PUBLIC_KEY_FILENAME=${SSH_PUBLIC_KEY_FILENAME}"
    echo "SUBNET=${SUBNET}"
    echo "UPDATE_SSH_AUTHORIZED_KEYS=${UPDATE_SSH_AUTHORIZED_KEYS}"
    echo "VM_SUFFIX=${VM_SUFFIX}"
    echo "WORKFLOWS_NAMESPACE=${WORKFLOWS_NAMESPACE}"
    echo "COMPUTE_IMAGE_DELETE_DEBUG=${COMPUTE_IMAGE_DELETE_DEBUG:-}"
    echo "WORKSPACE=${WORKSPACE}"
    echo "ZONE=${ZONE}"
    echo
}

# -----------------------------------------------------------------------------
# CLI usage and validation
# -----------------------------------------------------------------------------
function print_usage() {
    echo "Usage:
      ARCHITECTURE=${ARCHITECTURE} \\
      ADDITIONAL_NETWORKING=${ADDITIONAL_NETWORKING} \\
      BOOT_DISK_SIZE=${BOOT_DISK_SIZE} \\
      BOOT_DISK_TYPE=${BOOT_DISK_TYPE} \\
      CURL_UPDATE_COMMAND=${CURL_UPDATE_COMMAND} \\
      CUSTOM_VM_TYPE=${CUSTOM_VM_TYPE} \\
      CUSTOM_CPU=${CUSTOM_CPU} \\
      CUSTOM_MEMORY=${CUSTOM_MEMORY} \\
      CUTTLEFISH_INSTANCE_NAME=${cuttlefish_name} \\
      CUTTLEFISH_REVISION=${CUTTLEFISH_REVISION} \\
      CUTTLEFISH_URL=${CUTTLEFISH_URL} \\
      CUTTLEFISH_POST_COMMAND=${CUTTLEFISH_POST_COMMAND} \\
      DEFAULT_USER=${DEFAULT_USER} \\
      IMAGE=${IMAGE} \\
      JAVA_VERSION=${JAVA_VERSION} \\
      MACHINE_TYPE=${MACHINE_TYPE} \\
      MAX_RUN_DURATION=${MAX_RUN_DURATION} \\
      PACKER_USE_IAP=${PACKER_USE_IAP} \\
      PACKER_SSH_TIMEOUT=${PACKER_SSH_TIMEOUT} \\
      PACKER_IAP_TUNNEL_LAUNCH_WAIT=${PACKER_IAP_TUNNEL_LAUNCH_WAIT} \\
      PACKER_GOOGLECOMPUTE_VERSION_MIN=${PACKER_GOOGLECOMPUTE_VERSION_MIN} \\
      PACKER_GOOGLECOMPUTE_VERSION_MAX=${PACKER_GOOGLECOMPUTE_VERSION_MAX} \\
      NAMESPACE=${NAMESPACE} \\
      NETWORK=${NETWORK} \\
      NODEJS_VERSION=${NODEJS_VERSION} \\
      OS_PROJECT=${OS_PROJECT} \\
      OS_VERSION=${OS_VERSION} \\
      PROJECT=${PROJECT} \\
      REGION=${REGION} \\
      SERVICE_ACCOUNT=${SERVICE_ACCOUNT} \\
      SSH_PRIVATE_KEY_NAME=${SSH_PRIVATE_KEY_NAME} \\
      SSH_PUBLIC_KEY_FILENAME=${SSH_PUBLIC_KEY_FILENAME} \\
      SUBNET=${SUBNET} \\
      UPDATE_SSH_AUTHORIZED_KEYS=${UPDATE_SSH_AUTHORIZED_KEYS} \\
      VM_SUFFIX=${VM_SUFFIX} \\
      WORKFLOWS_NAMESPACE=${WORKFLOWS_NAMESPACE} \\
      COMPUTE_IMAGE_DELETE_DEBUG=${COMPUTE_IMAGE_DELETE_DEBUG:-} \\
      WORKSPACE=${WORKSPACE} \\
      ZONE=${ZONE} \\
      ./${SCRIPT_NAME}"
    echo "Use defaults or override environment variables."
    echo
    echo "Primary commands:"
    echo "  ./${SCRIPT_NAME} 1        Build image with Packer and create template"
    echo "  ./${SCRIPT_NAME} 2        Refresh SSH authorized_keys metadata only"
    echo "  ./${SCRIPT_NAME} 3        Delete generated artifacts for this target only"
    echo "  ./${SCRIPT_NAME} orphan-disks   Best-effort delete all unattached packer-* disks in ZONE (Argo onExit)"
}

# Check environment.
function check_environment() {
    if [ -z "${PROJECT}" ]; then
        echo -e "${RED}Environment variable PROJECT must be defined${NC}"
        exit 1
    fi
    if [ -z "${SERVICE_ACCOUNT}" ]; then
        echo -e "${RED}Environment variable SERVICE_ACCOUNT must be defined${NC}"
        exit 1
    fi
    if [[ "${cuttlefish_name}" != cuttlefish-vm* ]]; then
        echo -e "${RED}CUTTLEFISH_INSTANCE_NAME must start with cuttlefish-vm${NC}"
        exit 1
    fi
}

# Extract the local public key from Kubernetes secret if needed.
function ensure_ssh_public_key_local() {
    # Git artifacts in Argo often mount under /workspace with restrictive perms; use /tmp for key material.
    if [[ "${SSH_PUBLIC_KEY_FILENAME}" != /* ]]; then
        if ! ( : > "${SSH_PUBLIC_KEY_FILENAME}" ) 2>/dev/null; then
            SSH_PUBLIC_KEY_FILENAME="/tmp/${SSH_PUBLIC_KEY_FILENAME}"
        else
            rm -f "${SSH_PUBLIC_KEY_FILENAME}" 2>/dev/null || true
        fi
    fi
    if [ ! -f "${SSH_PUBLIC_KEY_FILENAME}" ]; then
        echo -e "${GREEN}Extracting public key ${SSH_PUBLIC_KEY_FILENAME}${NC}"
        local tmp_priv
        tmp_priv="$(mktemp /tmp/cf_ssh_priv.XXXXXX)"
        chmod 600 "${tmp_priv}"
        # shellcheck disable=SC1083
        kubectl get secrets -n "${NAMESPACE}" "${SSH_PRIVATE_KEY_NAME}" \
            --template={{.data.privateKey}} | base64 -d > "${tmp_priv}"
        ssh-keygen -y -f "${tmp_priv}" > "${SSH_PUBLIC_KEY_FILENAME}" || true
        rm -f "${tmp_priv}" || true

        if [ ! -f "${SSH_PUBLIC_KEY_FILENAME}" ]; then
            echo -e "${RED}ERROR: Failed to extract public key from private key${NC}"
            return 1
        fi
    else
        echo -e "${GREEN}Using local public key ${SSH_PUBLIC_KEY_FILENAME}${NC}"
    fi
}

# -----------------------------------------------------------------------------
# Packer / machine-type helpers
# -----------------------------------------------------------------------------
function get_packer_ssh_username() {
    local username="debian"
    if [[ "${OS_PROJECT}" == ubuntu* ]]; then
        username="ubuntu"
    fi
    echo "${username}"
}

function convert_memory_to_mb() {
    local value
    value=$(echo "$1" | awk '{print toupper($0)}' | xargs)
    if [[ "${value}" =~ ^([0-9]+)GB$ ]]; then
        echo "$((BASH_REMATCH[1] * 1024))"
        return 0
    fi
    if [[ "${value}" =~ ^([0-9]+)MB$ ]]; then
        echo "${BASH_REMATCH[1]}"
        return 0
    fi
    if [[ "${value}" =~ ^[0-9]+$ ]]; then
        echo "$((value * 1024))"
        return 0
    fi
    echo -e "${RED}Unsupported CUSTOM_MEMORY format: ${value}. Use MB or GB.${NC}"
    return 1
}

function get_packer_machine_type() {
    if [ -n "${MACHINE_TYPE}" ]; then
        echo "${MACHINE_TYPE}"
        return 0
    fi

    local memory_mb
    memory_mb=$(convert_memory_to_mb "${CUSTOM_MEMORY}") || return 1
    echo "${CUSTOM_VM_TYPE}-custom-${CUSTOM_CPU}-${memory_mb}"
}

function boot_disk_size_gb() {
    local value
    value=$(echo "${BOOT_DISK_SIZE}" | awk '{print toupper($0)}' | xargs)
    value=${value%GB}
    value=${value%G}
    echo "${value}"
}

function parse_duration_to_seconds() {
    local value
    value="$(echo "$1" | xargs)"
    if [ -z "${value}" ] || [ "${value}" = "0" ]; then
        echo "0"
        return 0
    fi
    if [[ "${value}" =~ ^([0-9]+)h$ ]]; then
        echo "$((BASH_REMATCH[1] * 3600))"
        return 0
    fi
    if [[ "${value}" =~ ^([0-9]+)m$ ]]; then
        echo "$((BASH_REMATCH[1] * 60))"
        return 0
    fi
    if [[ "${value}" =~ ^([0-9]+)s$ ]]; then
        echo "${BASH_REMATCH[1]}"
        return 0
    fi
    echo -e "${RED}Unsupported MAX_RUN_DURATION: ${value}. Use 0, Nh, Nm, or Ns (e.g. 12h).${NC}"
    return 1
}

# Kubernetes object name for the KCC ComputeInstanceTemplate (must be DNS-1123).
# spec.resourceID on the CR carries the actual GCP instance template name.
function cuttlefish_kcc_template_k8s_name() {
    local template_id="$1"
    echo "cf-it-${template_id}" | tr '[:upper:]' '[:lower:]' | sed -E 's/[^a-z0-9-]+/-/g; s/^-+//; s/-+$//; s/--+/-/g'
}

# -----------------------------------------------------------------------------
# GCP Compute REST (GKE / workload identity)
# -----------------------------------------------------------------------------
# cf_compute_rest.py performs global image/template DELETE and zonal instance
# stop/delete. The shell sets CF_COMPUTE_REST_TOKEN for each python3 invocation.
# delete-global-image: one stdout line (removed|absent|warn *); forwards
# COMPUTE_IMAGE_DELETE_DEBUG for optional tokeninfo stderr logging.
# delete-instance-template / delete-regional-instance-template: exits 0 on success/404, 1 on failure (stderr).
# zone-instance: always exits 0 best-effort (stderr on issues).
# Token discovery (metadata, not Compute API) stays in bash below.
#
# Access token for the pod/default workload identity SA via GCE metadata server (GKE).
function gcp_metadata_access_token() {
    local raw=""
    local base
    for base in \
        "http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/token" \
        "http://169.254.169.254/computeMetadata/v1/instance/service-accounts/default/token"; do
        raw="$(curl -fsS --connect-timeout 2 --max-time 8 \
            -H "Metadata-Flavor: Google" \
            "${base}?scopes=https%3A%2F%2Fwww.googleapis.com%2Fauth%2Fcloud-platform" 2>/dev/null)" && break
        raw=""
    done
    [ -n "${raw}" ] || return 1
    if ! command -v python3 >/dev/null 2>&1; then
        return 1
    fi
    printf '%s' "${raw}" | python3 -c 'import sys, json; print(json.load(sys.stdin)["access_token"])' 2>/dev/null || return 1
}

# Delete a global Compute Engine disk image (best effort) via
# cf_compute_rest.py delete-global-image (stdout contract in that file's docstring).
function delete_cuttlefish_global_image_best_effort() {
    local project="$1"
    local image_name="$2"
    local token
    local result
    local rc

    if token="$(gcp_metadata_access_token 2>/dev/null)" && [ -n "${token}" ]; then
        if [ ! -f "${cf_compute_rest_py}" ]; then
            echo_formatted "WARN: Missing ${cf_compute_rest_py}; skipped REST image delete for ${image_name}"
            return 0
        fi
        result="$(CF_COMPUTE_REST_TOKEN="${token}" COMPUTE_IMAGE_DELETE_DEBUG="${COMPUTE_IMAGE_DELETE_DEBUG:-}" \
            python3 "${cf_compute_rest_py}" delete-global-image "${project}" "${image_name}")"
        rc=$?
        if [ "${rc}" -ne 0 ]; then
            echo_formatted "WARN: delete-global-image helper failed for ${image_name}"
            return 0
        fi
        case "${result}" in
            removed)
                echo_formatted "Removed existing disk image ${image_name} (Compute Engine API)"
                ;;
            absent)
                echo_formatted "No existing disk image ${image_name} (Compute Engine API)"
                ;;
            warn\ TIMEOUT)
                echo_formatted "WARN: Global image delete operation timed out for ${image_name}"
                ;;
            warn\ OPERATION)
                echo_formatted "WARN: Global image delete operation failed for ${image_name}"
                ;;
            warn\ *)
                echo_formatted "WARN: Compute API image delete HTTP ${result#warn } for ${image_name}"
                ;;
            *)
                echo_formatted "WARN: Unexpected delete-global-image result for ${image_name}: ${result}"
                ;;
        esac
    else
        echo_formatted "WARN: Metadata token unavailable; skipped REST image delete for ${image_name}"
    fi
    return 0
}

# Delete a global instance template (best effort) via cf_compute_rest.py
# delete-instance-template; polls Operation until DONE.
function delete_global_instance_template_best_effort() {
    local project="$1"
    local template_name="$2"
    local token

    if token="$(gcp_metadata_access_token 2>/dev/null)" && [ -n "${token}" ]; then
        if [ ! -f "${cf_compute_rest_py}" ]; then
            echo_formatted "WARN: Missing ${cf_compute_rest_py}; skipped REST delete for instance template ${template_name}"
        elif CF_COMPUTE_REST_TOKEN="${token}" python3 "${cf_compute_rest_py}" delete-instance-template "${project}" "${template_name}"; then
            echo_formatted "Removed instance template ${template_name} (Compute Engine REST)"
            return 0
        else
            echo_formatted "WARN: Compute API instance template delete failed for ${template_name}"
        fi
    else
        echo_formatted "WARN: Metadata token unavailable; skipped REST delete for instance template ${template_name}"
    fi
}

# Delete a regional instance template (best effort) via cf_compute_rest.py
# delete-regional-instance-template (KCC spec.region).
function delete_regional_instance_template_best_effort() {
    local project="$1"
    local region="$2"
    local template_name="$3"
    local token

    if token="$(gcp_metadata_access_token 2>/dev/null)" && [ -n "${token}" ]; then
        if [ ! -f "${cf_compute_rest_py}" ]; then
            echo_formatted "WARN: Missing ${cf_compute_rest_py}; skipped REST regional delete for instance template ${template_name}"
        elif CF_COMPUTE_REST_TOKEN="${token}" python3 "${cf_compute_rest_py}" delete-regional-instance-template "${project}" "${region}" "${template_name}"; then
            echo_formatted "Removed regional instance template ${template_name} in ${region} (Compute Engine REST)"
            return 0
        else
            echo_formatted "WARN: Compute API regional instance template delete failed for ${template_name} (${region})"
        fi
    else
        echo_formatted "WARN: Metadata token unavailable; skipped REST regional delete for instance template ${template_name}"
    fi
}

# Stop or delete a zonal VM (best effort) via cf_compute_rest.py zone-instance.
# Args: action (stop|delete), project, zone, instance_name
function zone_instance_compute_best_effort() {
    local action="$1"
    local project="$2"
    local zone="$3"
    local instance_name="$4"
    local token

    if [ "${action}" != "stop" ] && [ "${action}" != "delete" ]; then
        return 0
    fi
    if ! token="$(gcp_metadata_access_token 2>/dev/null)" || [ -z "${token}" ]; then
        echo_formatted "WARN: Metadata token unavailable; skipped instances.${action} for ${instance_name}"
        return 0
    fi
    if [ ! -f "${cf_compute_rest_py}" ]; then
        echo_formatted "WARN: Missing ${cf_compute_rest_py}; skipped instances.${action} for ${instance_name}"
        return 0
    fi
    CF_COMPUTE_REST_TOKEN="${token}" python3 "${cf_compute_rest_py}" zone-instance "${action}" "${project}" "${zone}" "${instance_name}"
}

# Remove all SSH keys from the OS Login profile for the *current* metadata token identity
# (same identity Packer uses when importing an OS Login key). Uses Cloud OS Login REST
# (cf_compute_rest.py prune-os-login-ssh-keys), not gcloud.
# Stale keys from prior runs can fill the profile until Google returns:
#   Error 400: Login profile size exceeds 32 KiB. Delete profile values to make additional space.
function prune_os_login_ssh_keys_for_caller() {
    echo -e "${GREEN}Pruning OS Login SSH keys for current identity (frees profile space for Packer OS Login import).${NC}"
    local token removed
    if ! token="$(gcp_metadata_access_token 2>/dev/null)" || [ -z "${token}" ]; then
        echo -e "${ORANGE}WARNING: Metadata token unavailable; skipped OS Login SSH key prune.${NC}" >&2
        return 0
    fi
    if [ ! -f "${cf_compute_rest_py}" ] || ! command -v python3 >/dev/null 2>&1; then
        echo -e "${ORANGE}WARNING: Missing ${cf_compute_rest_py} or python3; cannot prune OS Login keys.${NC}" >&2
        return 0
    fi
    # https://cloud.google.com/compute/docs/troubleshooting/troubleshoot-os-login#invalid_argument
    removed="$(CF_COMPUTE_REST_TOKEN="${token}" python3 "${cf_compute_rest_py}" prune-os-login-ssh-keys | tail -n 1)" || removed="0"
    if ! [[ "${removed}" =~ ^[0-9]+$ ]]; then
        removed="0"
    fi
    echo -e "${GREEN}OS Login SSH key prune finished (removed ${removed} key(s)).${NC}"
}

# Stage 2 runs without Packer; fail fast if stage 1 GCP artifacts are missing (otherwise kubectl apply
# succeeds while KCC/GCP reconcile fails later and the job looks green). Preflight uses Compute REST
# (disk image + instance template), not the KCC CR: the CR may be absent while the GCE template still
# exists (e.g. after manual CR delete); publish_instance_template_from_image then recreates the CR.
function assert_gcp_cuttlefish_disk_image_exists() {
    local token
    if ! token="$(gcp_metadata_access_token 2>/dev/null)" || [ -z "${token}" ]; then
        echo -e "${RED}ERROR: Metadata token unavailable; cannot verify global disk image ${vm_cuttlefish_image}.${NC}" >&2
        return 1
    fi
    if [ ! -f "${cf_compute_rest_py}" ] || ! command -v python3 >/dev/null 2>&1; then
        echo -e "${RED}ERROR: Missing ${cf_compute_rest_py} or python3; cannot verify disk image.${NC}" >&2
        return 1
    fi
    echo_formatted "Verify global disk image exists: ${vm_cuttlefish_image}"
    if ! CF_COMPUTE_REST_TOKEN="${token}" python3 "${cf_compute_rest_py}" get-global-image "${PROJECT}" "${vm_cuttlefish_image}"; then
        echo -e "${RED}ERROR: Global disk image '${vm_cuttlefish_image}' not found in project ${PROJECT} (Compute images.get). Run stage 1 before SSH-only refresh.${NC}" >&2
        return 1
    fi
    return 0
}

function assert_gcp_cuttlefish_instance_template_exists() {
    local token
    if ! token="$(gcp_metadata_access_token 2>/dev/null)" || [ -z "${token}" ]; then
        echo -e "${RED}ERROR: Metadata token unavailable; cannot verify GCE instance template ${vm_cuttlefish_instance_template}.${NC}" >&2
        return 1
    fi
    if [ ! -f "${cf_compute_rest_py}" ] || ! command -v python3 >/dev/null 2>&1; then
        echo -e "${RED}ERROR: Missing ${cf_compute_rest_py} or python3; cannot verify instance template.${NC}" >&2
        return 1
    fi
    echo_formatted "Verify GCE instance template exists (Compute REST): ${vm_cuttlefish_instance_template} (project ${PROJECT}, region ${REGION} then global)"
    if ! CF_COMPUTE_REST_TOKEN="${token}" python3 "${cf_compute_rest_py}" get-instance-template "${PROJECT}" "${REGION}" "${vm_cuttlefish_instance_template}"; then
        echo -e "${RED}ERROR: GCE instance template '${vm_cuttlefish_instance_template}' not found in region ${REGION} or globally in ${PROJECT}. Run stage 1 (full publish) first.${NC}" >&2
        return 1
    fi
    return 0
}

# Render the googlecompute required_plugins block into a standalone *.pkr.hcl fragment so the
# version bounds come from PACKER_GOOGLECOMPUTE_VERSION_MIN/MAX (env / Helm / Jenkins params)
# instead of being hard-coded in cuttlefish.pkr.hcl (see TAA-1839). Packer evaluates
# required_plugins during `packer init`, before variables, so this cannot be a normal HCL var.
function generate_packer_plugins_fragment() {
    local out_file="$1"
    local version_re='^[0-9]+\.[0-9]+(\.[0-9]+)?$'
    if [[ ! "${PACKER_GOOGLECOMPUTE_VERSION_MIN}" =~ ${version_re} ]]; then
        echo -e "${RED}ERROR: PACKER_GOOGLECOMPUTE_VERSION_MIN='${PACKER_GOOGLECOMPUTE_VERSION_MIN}' is not a valid version (expected e.g. 1.2.3).${NC}" >&2
        return 1
    fi
    if [[ ! "${PACKER_GOOGLECOMPUTE_VERSION_MAX}" =~ ${version_re} ]]; then
        echo -e "${RED}ERROR: PACKER_GOOGLECOMPUTE_VERSION_MAX='${PACKER_GOOGLECOMPUTE_VERSION_MAX}' is not a valid version (expected e.g. 1.2.5).${NC}" >&2
        return 1
    fi
    local constraint=">= ${PACKER_GOOGLECOMPUTE_VERSION_MIN}, < ${PACKER_GOOGLECOMPUTE_VERSION_MAX}"
    echo_formatted "Packer googlecompute plugin constraint: '${constraint}' (min=${PACKER_GOOGLECOMPUTE_VERSION_MIN} max=${PACKER_GOOGLECOMPUTE_VERSION_MAX}, exclusive)"
    cat > "${out_file}" <<EOF || return 1
# GENERATED by cf_create_instance_template.sh (TAA-1839) - do not edit by hand.
# Version bounds come from PACKER_GOOGLECOMPUTE_VERSION_MIN / PACKER_GOOGLECOMPUTE_VERSION_MAX.
packer {
  required_plugins {
    googlecompute = {
      source  = "github.com/hashicorp/googlecompute"
      version = "${constraint}"
    }
  }
}
EOF
    return 0
}

# -----------------------------------------------------------------------------
# Pipeline stages (1 = Packer + KCC, 2 = metadata refresh, 3 = teardown)
# -----------------------------------------------------------------------------
function build_image_with_packer() {
    echo_formatted "1. Build Cuttlefish image with Packer"

    if ! command -v packer >/dev/null 2>&1; then
        echo -e "${RED}ERROR: packer binary not found in PATH.${NC}"
        exit 1
    fi
    if [ ! -f "${packer_template_path}" ] || [ ! -f "${packer_provision_script_path}" ]; then
        echo -e "${RED}ERROR: Packer files missing under ${CF_SCRIPT_PATH}/packer.${NC}"
        exit 1
    fi

    ensure_ssh_public_key_local || exit 1

    local ssh_public_key_b64
    ssh_public_key_b64=$(base64 < "${SSH_PUBLIC_KEY_FILENAME}" | tr -d '\n')
    local packer_machine_type
    packer_machine_type=$(get_packer_machine_type) || exit 1
    local packer_ssh_username
    packer_ssh_username=$(get_packer_ssh_username)
    local disk_size_gb
    disk_size_gb=$(boot_disk_size_gb)

    local _packer_default_duration='4h'
    local _packer_max_duration
    _packer_max_duration="${PACKER_BUILD_MAX_RUN_DURATION:-${_packer_default_duration}}"
    local packer_max_run_seconds
    packer_max_run_seconds="$(parse_duration_to_seconds "${_packer_max_duration}")" || exit 1
    if [ "${packer_max_run_seconds}" -eq 0 ]; then
        packer_max_run_seconds="$(parse_duration_to_seconds "${_packer_default_duration}")" || exit 1
        echo_formatted "PACKER_BUILD_MAX_RUN_DURATION resolved to 0; using default ${_packer_default_duration} for Packer builder VM."
    fi
    echo_formatted "Packer builder GCE max run: ${packer_max_run_seconds}s (PACKER_BUILD_MAX_RUN_DURATION=${PACKER_BUILD_MAX_RUN_DURATION:-${_packer_default_duration}}; auto-delete if job hangs or client is lost)"

    local packer_use_iap=true
    case "$(printf '%s' "${PACKER_USE_IAP}" | tr '[:upper:]' '[:lower:]' | tr -d '[:space:]')" in
        0|false|no|off) packer_use_iap=false ;;
    esac
    local packer_ssh_timeout
    packer_ssh_timeout="${PACKER_SSH_TIMEOUT}"
    local packer_iap_tunnel_wait
    packer_iap_tunnel_wait="${PACKER_IAP_TUNNEL_LAUNCH_WAIT}"
    echo_formatted "Packer SSH: use_iap=${packer_use_iap} ssh_timeout=${packer_ssh_timeout} iap_tunnel_launch_wait=${packer_iap_tunnel_wait}s"

    delete_cuttlefish_global_image_best_effort "${PROJECT}" "${vm_cuttlefish_image}"

    prune_os_login_ssh_keys_for_caller

    # Packer writes crash.log to cwd; Argo git artifacts often leave WORKSPACE=/workspace read-only.
    # We also render the plugin version constraint into this writable dir (the source packer/ dir may be
    # read-only on Argo), so Packer must run against ${packer_work}, not the in-repo template path.
    local packer_work
    packer_work="$(mktemp -d "${TMPDIR:-/tmp}/cf-packer-work.XXXXXX")" || exit 1
    chmod 700 "${packer_work}"
    # Copy the template + provisioner next to the generated fragment so path.root resolves inside
    # ${packer_work} and `packer init/build ${packer_work}` sees all *.pkr.hcl (template + plugins).
    cp "${packer_template_path}" "${packer_work}/cuttlefish.pkr.hcl" || exit 1
    cp "${packer_provision_script_path}" "${packer_work}/provision_cf_host.sh" || exit 1
    generate_packer_plugins_fragment "${packer_work}/plugins.pkr.hcl" || exit 1
    local packer_rc=0
    if ! (
        set -e
        cd "${packer_work}"
        packer init "${packer_work}"
        packer build \
            -var "project_id=${PROJECT}" \
            -var "zone=${ZONE}" \
            -var "region=${REGION}" \
            -var "network=${NETWORK}" \
            -var "subnetwork=${SUBNET}" \
            -var "source_image_project_id=${OS_PROJECT}" \
            -var "source_image=${OS_VERSION}" \
            -var "machine_type=${packer_machine_type}" \
            -var "disk_size_gb=${disk_size_gb}" \
            -var "disk_type=${BOOT_DISK_TYPE}" \
            -var "image_name=${vm_cuttlefish_image}" \
            -var "image_description=${vm_cuttlefish_image}" \
            -var "ssh_username=${packer_ssh_username}" \
            -var "default_user=${DEFAULT_USER}" \
            -var "cf_script_path=${CF_SCRIPT_PATH}" \
            -var "android_cuttlefish_revision=${CUTTLEFISH_REVISION}" \
            -var "cuttlefish_url=${CUTTLEFISH_URL}" \
            -var "cuttlefish_post_command=${CUTTLEFISH_POST_COMMAND}" \
            -var "repo_username=${REPO_USERNAME}" \
            -var "repo_password=${REPO_PASSWORD}" \
            -var "java_version=${JAVA_VERSION}" \
            -var "nodejs_version=${NODEJS_VERSION}" \
            -var "curl_update_command=${CURL_UPDATE_COMMAND}" \
            -var "os_version=${OS_VERSION}" \
            -var "cts_android_16_url=${CTS_ANDROID_16_URL}" \
            -var "cts_android_15_url=${CTS_ANDROID_15_URL}" \
            -var "cts_android_14_url=${CTS_ANDROID_14_URL}" \
            -var "ssh_public_key_b64=${ssh_public_key_b64}" \
            -var "use_iap=${packer_use_iap}" \
            -var "ssh_timeout=${packer_ssh_timeout}" \
            -var "iap_tunnel_launch_wait=${packer_iap_tunnel_wait}" \
            -var "packer_max_run_duration_seconds=${packer_max_run_seconds}" \
            "${packer_work}"
    ); then
        packer_rc=1
    fi
    rm -rf "${packer_work}"

    cleanup_orphan_packer_boot_disks "${disk_size_gb}" || true

    if [ "${packer_rc}" -ne 0 ]; then
        echo -e "${RED}ERROR: Packer build failed (exit ${packer_rc}); running scoped delete for this target only.${NC}" >&2
        echo -e "${RED}       The cause is in the Packer output above. If the googlecompute plugin crashed with SIGSEGV, change PACKER_GOOGLECOMPUTE_VERSION_MIN and PACKER_GOOGLECOMPUTE_VERSION_MAX. See docs/workloads/android/environment/cf_instance_template.md.${NC}" >&2
        delete_cuttlefish_publish_target_only || true
        exit "${packer_rc}"
    fi

    echo -e "${GREEN}Image ${vm_cuttlefish_image} built with Packer${NC}"
}

function publish_instance_template_from_image() {
    ensure_ssh_public_key_local || return 1
    if [ ! -f "${startup_script_path}" ]; then
        echo -e "${RED}ERROR: Startup script missing: ${startup_script_path}${NC}"
        return 1
    fi

    if ! command -v kubectl >/dev/null 2>&1; then
        echo -e "${RED}ERROR: kubectl binary not found in PATH.${NC}"
        return 1
    fi

    local kcc_namespace="${WORKFLOWS_NAMESPACE}"

    local boot_disk_size
    boot_disk_size="$(boot_disk_size_gb)"

    local packer_machine_type
    packer_machine_type="$(get_packer_machine_type)" || return 1

    local max_run_seconds
    max_run_seconds="$(parse_duration_to_seconds "${MAX_RUN_DURATION}")" || return 1

    local jenkins_authorized_key
    jenkins_authorized_key="$(tr -d '\n' < "${SSH_PUBLIC_KEY_FILENAME}")"

    local startup_script
    startup_script="$(cat "${startup_script_path}")"

    local nic_type=""
    if [[ "${ADDITIONAL_NETWORKING}" == *"nic-type=IDPF"* ]]; then
        nic_type="IDPF"
    fi

    local k8s_name
    k8s_name="$(cuttlefish_kcc_template_k8s_name "${vm_cuttlefish_instance_template}")"

    echo -e "${GREEN}Publishing instance template via Config Connector (namespace: ${kcc_namespace})${NC}"
    echo -e "${GREEN}Target GCP instance template: ${vm_cuttlefish_instance_template}${NC}"

    # Delete + recreate is the most reliable approach because most template fields are immutable.
    # Always kubectl delete the CR (wait): if the CR was already gone, KCC never removed the regional
    # GCP template and apply would only adopt the old object (stale creationTimestamp). After the CR
    # wait, best-effort global then regional REST deletes remove an orphan GCP template before apply.
    local kcc_delete_timeout="${KCC_INSTANCE_TEMPLATE_DELETE_TIMEOUT:-15m}"

    echo -e "${GREEN}Deleting KCC ComputeInstanceTemplate ${k8s_name} if present (kubectl wait up to ${kcc_delete_timeout})...${NC}"
    if ! kubectl -n "${kcc_namespace}" delete computeinstancetemplate "${k8s_name}" \
        --ignore-not-found=true \
        --wait=true \
        --timeout="${kcc_delete_timeout}"; then
        echo -e "${RED}ERROR: kubectl delete ComputeInstanceTemplate ${k8s_name} in namespace ${kcc_namespace} failed or timed out.${NC}" >&2
        return 1
    fi

    echo_formatted "Best-effort GCP instance template delete before apply (orphan when CR was absent): ${vm_cuttlefish_instance_template}"
    delete_global_instance_template_best_effort "${PROJECT}" "${vm_cuttlefish_instance_template}"
    delete_regional_instance_template_best_effort "${PROJECT}" "${REGION}" "${vm_cuttlefish_instance_template}"

    if ! kubectl -n "${kcc_namespace}" apply -f - <<EOF
apiVersion: compute.cnrm.cloud.google.com/v1beta1
kind: ComputeInstanceTemplate
metadata:
  name: ${k8s_name}
  labels:
    horizon-sdv.io/cuttlefish-kcc-template: "true"
  annotations:
    cnrm.cloud.google.com/project-id: "${PROJECT}"
spec:
  resourceID: "${vm_cuttlefish_instance_template}"
  description: "${vm_cuttlefish_instance_template}"
  region: "${REGION}"
  machineType: "${packer_machine_type}"
  instanceDescription: "${vm_cuttlefish_instance_template}"
  advancedMachineFeatures:
    enableNestedVirtualization: true
  shieldedInstanceConfig:
    enableVtpm: true
    enableSecureBoot: false
    enableIntegrityMonitoring: true
  disk:
    - boot: true
      autoDelete: true
      type: PERSISTENT
      diskType: "${BOOT_DISK_TYPE}"
      diskSizeGb: ${boot_disk_size}
      sourceImageRef:
        external: "https://www.googleapis.com/compute/v1/projects/${PROJECT}/global/images/${vm_cuttlefish_image}"
  networkInterface:
    - networkRef:
        external: "https://www.googleapis.com/compute/v1/projects/${PROJECT}/global/networks/${NETWORK}"
      subnetworkRef:
        external: "https://www.googleapis.com/compute/v1/projects/${PROJECT}/regions/${REGION}/subnetworks/${SUBNET}"
      stackType: "IPV4_ONLY"
$(if [ -n "${nic_type}" ]; then echo "      nicType: \"${nic_type}\""; fi)
  metadata:
    - key: enable-oslogin
      value: "true"
    - key: jenkins-user
      value: "${DEFAULT_USER}"
    - key: jenkins-authorized-key
      value: "${jenkins_authorized_key}"
  metadataStartupScript: |
$(printf '%s\n' "${startup_script}" | sed 's/^/    /')
  scheduling:
    automaticRestart: false
    onHostMaintenance: "TERMINATE"
    preemptible: false
$(if [ "${max_run_seconds}" != "0" ]; then cat <<EOM
    maxRunDuration:
      seconds: ${max_run_seconds}
    instanceTerminationAction: "DELETE"
EOM
fi)
  serviceAccount:
    serviceAccountRef:
      external: "${SERVICE_ACCOUNT}"
    scopes:
      - "https://www.googleapis.com/auth/devstorage.read_only"
      - "https://www.googleapis.com/auth/logging.write"
      - "https://www.googleapis.com/auth/monitoring.write"
      - "https://www.googleapis.com/auth/pubsub"
      - "https://www.googleapis.com/auth/service.management.readonly"
      - "https://www.googleapis.com/auth/servicecontrol"
      - "https://www.googleapis.com/auth/trace.append"
EOF
    then
        echo -e "${RED}ERROR: kubectl apply ComputeInstanceTemplate failed.${NC}" >&2
        return 1
    fi

    echo -e "${GREEN}ComputeInstanceTemplate applied; Config Connector will reconcile to GCP.${NC}"
}

function create_instance_template_from_image() {
    echo_formatted "1. Create Cuttlefish instance template from baked image"
    publish_instance_template_from_image
}

function refresh_ssh_authorized_keys_on_existing_template() {
    echo_formatted "2. Refresh SSH key metadata on existing template"
    assert_gcp_cuttlefish_disk_image_exists || exit 1
    assert_gcp_cuttlefish_instance_template_exists || exit 1
    if ! publish_instance_template_from_image; then
        echo -e "${RED}ERROR: Failed to refresh SSH key metadata on template.${NC}" >&2
        delete_cuttlefish_publish_target_only || true
        exit 1
    fi
    echo -e "${GREEN}Template SSH key metadata refreshed (no image rebuild).${NC}"
}

# Deletes KCC CR, GCP instance template, disk image, and build VMs for **this** run's
# vm_cuttlefish_* / vm_base_* names only (never other tracks' ComputeInstanceTemplate CRs).
function delete_cuttlefish_publish_target_only() {
    echo_formatted "Scoped teardown for this publish target only (${vm_cuttlefish_instance_template})"

    delete_global_instance_template_best_effort "${PROJECT}" "${vm_base_instance_template}"

    local kcc_delete_timeout="${KCC_INSTANCE_TEMPLATE_DELETE_TIMEOUT:-15m}"
    local kcc_namespace_del="${WORKFLOWS_NAMESPACE}"
    local k8s_name_del
    k8s_name_del="$(cuttlefish_kcc_template_k8s_name "${vm_cuttlefish_instance_template}")"

    if ! command -v kubectl >/dev/null 2>&1; then
        echo -e "${RED}ERROR: kubectl not in PATH; cannot delete KCC ComputeInstanceTemplate CRs in ${kcc_namespace_del}.${NC}" >&2
        return 1
    fi

    echo_formatted "   Deleting KCC ComputeInstanceTemplate ${k8s_name_del} (namespace ${kcc_namespace_del}, wait up to ${kcc_delete_timeout})"
    if ! kubectl -n "${kcc_namespace_del}" delete computeinstancetemplate "${k8s_name_del}" \
        --ignore-not-found=true \
        --wait=true \
        --timeout="${kcc_delete_timeout}"; then
        echo -e "${RED}ERROR: kubectl delete ComputeInstanceTemplate ${k8s_name_del} in ${kcc_namespace_del} failed or timed out.${NC}" >&2
        return 1
    fi

    # Best-effort GCP template removal if KCC lag / wrong CR: global then regional (KCC uses spec.region).
    delete_global_instance_template_best_effort "${PROJECT}" "${vm_cuttlefish_instance_template}"
    delete_regional_instance_template_best_effort "${PROJECT}" "${REGION}" "${vm_cuttlefish_instance_template}"

    delete_cuttlefish_global_image_best_effort "${PROJECT}" "${vm_cuttlefish_image}"
    echo_formatted "   Deleted ${vm_cuttlefish_image} (or already absent)"

    zone_instance_compute_best_effort stop "${PROJECT}" "${ZONE}" "${vm_base_instance}"
    echo_formatted "   Stopped ${vm_base_instance} (Compute Engine REST)"

    zone_instance_compute_best_effort delete "${PROJECT}" "${ZONE}" "${vm_base_instance}"
    echo_formatted "   Deleted ${vm_base_instance} (Compute Engine REST)"

    zone_instance_compute_best_effort stop "${PROJECT}" "${ZONE}" "${vm_cuttlefish_instance}"
    echo_formatted "   Stopped ${vm_cuttlefish_instance} (Compute Engine REST)"

    zone_instance_compute_best_effort delete "${PROJECT}" "${ZONE}" "${vm_cuttlefish_instance}"
    echo_formatted "   Deleted ${vm_cuttlefish_instance} (Compute Engine REST)"
    return 0
}

# Stage 3: explicit user delete — same scoped teardown as publish/Packer failure (this target only).
function delete_instances() {
    echo_formatted "3. Delete VM instances and artifacts"
    delete_cuttlefish_publish_target_only || return 1
}

# Build disk image with Packer, then publish ComputeInstanceTemplate via kubectl/KCC.
# On publish failure: scoped teardown for this target only (other tracks' KCC CRs
# untouched); always return the publish exit code so Argo/Jenkins still fail.
function run_packer_build_and_publish_template() {
    build_image_with_packer
    create_instance_template_from_image || {
        local rc=$?
        echo -e "${RED}ERROR: Instance template publish failed (exit ${rc}); running scoped delete for this target only.${NC}" >&2
        delete_cuttlefish_publish_target_only || true
        return "${rc}"
    }
    return 0
}

# Main: run all or allow the user to select which steps to run.
function main() {
    echo -e "${GREEN}HOST IP: ${NC} $(hostname -I || true)"
    if [[ "${1:-}" != "orphan-disks" ]]; then
        echo_environment
        check_environment
    fi
    case "$1" in
        orphan-disks)
            if [ -z "${PROJECT}" ] || [ -z "${ZONE}" ]; then
                echo -e "${RED}orphan-disks requires PROJECT and ZONE (e.g. from cloud ConfigMap or workflow env).${NC}" >&2
                exit 1
            fi
            echo -e "${GREEN}[$SCRIPT_NAME]${NC} orphan-disks: REST cleanup of all unattached packer-* disks (zone=${ZONE})" >&2
            cleanup_all_unattached_packer_disks_in_zone || true
            exit 0
            ;;
        1)
            run_packer_build_and_publish_template || exit 1
            ;;
        2)
            refresh_ssh_authorized_keys_on_existing_template
            ;;
        3)
            delete_instances || exit 1
            ;;
        *h*)
            print_usage
            exit 0
            ;;
        *)
            if run_packer_build_and_publish_template; then
                echo_formatted "Done. Please check the output above and enjoy Cuttlefish!"
            else
                exit 1
            fi
            ;;
    esac
}

main "$1"
