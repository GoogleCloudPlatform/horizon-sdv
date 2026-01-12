#!/usr/bin/env bash

# Copyright (c) 2024-2025 Accenture, All Rights Reserved.
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
# GCE plugin.
#
# To run, ensure gcloud is set up, authenticated and tunneling is
# configured, eg. --tunnel-through-iap.
#
# From command line, such as Google Cloud Shell, create templates for all
# versions of android-cuttlefish host tools/packages:
#
#  CUTTLEFISH_REVISION=v1.35.0 ./cf_create_instance_template.sh && \
#  CUTTLEFISH_REVISION=main ./cf_create_instance_template.sh
#
# The following variables are required to run the script, choose to use
# default values or override from command line.
#
#  - ANDROID_CUTTLEFISH_PREBUILT: build or install prebuilt versions of
#        cuttlefish.
#  - ADDITIONAL_NETWORKING: ARM64 Bare metal requires IDPF network interface.
#  - CURL_UPDATE_COMMAND: command to update/upgrade Curl
#        eg. Debian backports: apt install -t bookworm-backports -y curl libcurl4
#  - CUSTOM_VM_TYPE: Custom machine VM type.
#  - CUSTOM_CPU: Custom machine CPUs.
#  - CUSTOM_MEMORY: Custom machine memory.
#  - CUTTLEFISH_REVISION: the branch/tag version of Android Cuttlefish
#        to use. Default: main
#  - CUTTLEFISH_URL: the repo URL for android cuttlefish.
#  - CUTTLEFISH_POST_COMMAND: command to run in android-cuttlefish repo.
#  - BOOT_DISK_SIZE: Disk image size in GB. Default: 250GB
#  - BOOT_DISK_TYPE: Disk image disk type.
#  - JAVA_VERSION: Update Java version (must be openjdk headless)
#  - JENKINS_NAMESPACE: k8s namespace. Default: jenkins
#  - JENKINS_PRIVATE_SSH_KEY_NAME: SSH key name to extract public key from
#        Private key would be created similar to:
#        ssh-keygen -t rsa -b 4096 -C "jenkins" -f jenkins_rsa -q -N ""
#        -C: comment 'jenkins'
#        -N: no passphrase
#        Then added to k8s secrets and defined in Jenkins credentials.
#        Default: jenkins-cuttlefish-vm-ssh-private-key
#  - JENKINS_SSH_PUB_KEY_FILE: Public key file name.
#        Default: jenkins_rsa.pub
#  - MACHINE_TYPE: The machine type to create instance templates for.
#       If undefined, the CUSTOM_ parameters must be.
#  - MAX_RUN_DURATION: Limits how long this VM instance can run. Default: 10h
#  - NETWORK: The name of the VPC network. Default: sdv-network
#  - NODEJS_VERSION: The version of nodejs to install. Default: 20.9.0
#  - OS_VERSION: Default: debian-12-bookworm-v20251209
#  - PROJECT: The GCP project. Default: derived from gcloud config.
#  - REGION: The GCP region. Default: europe-west1
#  - REPO_USERNAME: username for access to private repo.
#  - REPO_PASSWORD: password for access to private repo.
#  - SERVICE_ACCOUNT: The GCP service account. Default: derived from gcloud
#        projects describe.
#  - SUBNET: The name of the subnet. Default: sdv-subnet
#  - CUTTLEFISH_INSTANCE_NAME: The name used to identify the instance.
#        Default: cuttlefish-vm-<branch-name>
#  - VM_INSTANCE_CREATE: If 'true', then create a stopped VM instance from
#        the final instance template. Useful for devs to experiment with the
#        VM instances. May be disabled to reduce managed disk costs.
#        Default: true
#  - ZONE: The GCP zone. Default: europe-west1-d
#
# The following arguments are optional and recommended run without args:

#  -h|--help :     - Print usage
#  1 : Run stage 1 - create the base instance template (debian +
#                    virtualisation)
#  2 : Run stage 2 - create the VM instance from base instance template.
#  3 : Run stage 3 - Populate the VM instance with CF host packages
#                    and other dependent packages.
#                    Create 'jenkins' user and groups.
#                    Add CF groups to jenkins account.
#  4 : Run stage 4 - Create the SSH key for Jenkins account if key is not
#                    available, and store public key as authorized_key for
#                    Jenkins account. If key exists, reuse the existing public
#                    key.
#  5 : Run stage 5 - Stop the VM instance that is now configured for CF.
#                    Create a boot disk image of that VM instance.
#                    Create Cuttlefish instance template from that boot disk
#                    image.
#                    Create the Cuttlefish VM instance and stop that instance.
#                    The instance template is what GCE Plugin uses, the VM
#                    instance is purely created for reference.
#  6 : Run stage 6 - Allow admins to clean up instances, artifacts.
#                    Simply a helper job, only required if admins wish to
#                    drop older versions of Cuttlefish.
#  No args:          run all stages with exception 6 (delete).
# Include common functions and variables.
# shellcheck disable=SC1091
source "$(dirname "${BASH_SOURCE[0]}")"/cf_environment.sh "$0"

# Colours for logging.
GREEN='\033[1;32m'
ORANGE='\033[1;33m'
RED='\033[1;31m'
NC='\033[0m'
SCRIPT_NAME=$(basename "$0")

# Environment variables that can be overridden from command line.
# android-cuttlefish revisions can be of the form v1.7.0, main etc.
ANDROID_CUTTLEFISH_PREBUILT=${ANDROID_CUTTLEFISH_PREBUILT:-false}
ADDITIONAL_NETWORKING=${ADDITIONAL_NETWORKING:-}
[ -n "${ADDITIONAL_NETWORKING}" ] && ADDITIONAL_NETWORKING=",${ADDITIONAL_NETWORKING}"
BOOT_DISK_SIZE=${BOOT_DISK_SIZE:-500GB}
BOOT_DISK_SIZE=$(echo "${BOOT_DISK_SIZE}" | awk '{print toupper($0)}' | xargs)
BOOT_DISK_TYPE=${BOOT_DISK_TYPE:-pd-balanced}
CURL_UPDATE_COMMAND=${CURL_UPDATE_COMMAND:-}
CUTTLEFISH_INSTANCE_NAME=${CUTTLEFISH_INSTANCE_NAME:-cuttlefish-vm}
CUTTLEFISH_INSTANCE_NAME=$(echo "${CUTTLEFISH_INSTANCE_NAME}" | awk '{print tolower($0)}' | xargs)
CUTTLEFISH_REVISION=${CUTTLEFISH_REVISION:-v1.35.0}
CUTTLEFISH_REVISION=$(echo "${CUTTLEFISH_REVISION}" | xargs)
CUTTLEFISH_URL=${CUTTLEFISH_URL:-https://github.com/google/android-cuttlefish.git}
CUTTLEFISH_URL=$(echo "${CUTTLEFISH_URL}" | xargs)
CUTTLEFISH_POST_COMMAND=${CUTTLEFISH_POST_COMMAND:-}
JAVA_VERSION=${JAVA_VERSION:-openjdk-17-jdk-headless}
JENKINS_NAMESPACE=${JENKINS_NAMESPACE:-jenkins}
JENKINS_PRIVATE_SSH_KEY_NAME=${JENKINS_PRIVATE_SSH_KEY_NAME:-jenkins-cuttlefish-vm-ssh-private-key}
JENKINS_SSH_PUB_KEY_FILE=${JENKINS_SSH_PUB_KEY_FILE:-jenkins_rsa.pub}
MACHINE_TYPE=${MACHINE_TYPE:-}
MACHINE_TYPE=$(echo "${MACHINE_TYPE}" | xargs)
MAX_RUN_DURATION=${MAX_RUN_DURATION:-10h}
NETWORK=${NETWORK:-sdv-network}
NODEJS_VERSION=${NODEJS_VERSION:-20.9.0}
NODEJS_VERSION=$(echo "${NODEJS_VERSION}" | xargs)
OS_PROJECT=${OS_PROJECT:-debian-cloud}
OS_PROJECT=$(echo "${OS_PROJECT}" | xargs)
OS_VERSION=${OS_VERSION:-debian-12-bookworm-v20251209}
OS_VERSION=$(echo "${OS_VERSION}" | xargs)
PROJECT=${PROJECT:-$(gcloud config list --format 'value(core.project)'|head -n 1)}
REGION=${REGION:-europe-west1}
REPO_USERNAME=${REPO_USERNAME:-}
REPO_USERNAME=$(echo "${REPO_USERNAME}" | xargs)
REPO_PASSWORD=${REPO_PASSWORD:-}
REPO_PASSWORD=$(echo "${REPO_PASSWORD}" | xargs)
SERVICE_ACCOUNT=${SERVICE_ACCOUNT:-$(gcloud projects describe "${PROJECT}" --format='get(projectNumber)')-compute@developer.gserviceaccount.com}
SUBNET=${SUBNET:-sdv-subnet}
VM_INSTANCE_CREATE=${VM_INSTANCE_CREATE:-true}
ZONE=${ZONE:-europe-west1-d}

IMAGE="projects/${OS_PROJECT}/global/images/${OS_VERSION}"

# Define architecture based on OS_VERSION as this will always include arch for arm.
if [[ "$OS_VERSION" == *arm64* ]]; then
    ARCHITECTURE="ARM64"
    VM_SUFFIX="-arm64"
else
    ARCHITECTURE="X86_64"
fi
# Machine type or custom type
declare machine_type_args=""
if [ -z "${MACHINE_TYPE}" ]; then
    if [[ -z "${CUSTOM_VM_TYPE}" || -z "${CUSTOM_CPU}"  || -z "${CUSTOM_VM_TYPE}" ]]; then
        echo -e "${RED}ERROR: MACHINE_TYPE or CUSTOM options must be defined.${NC}"
        exit 1
    else
        machine_type_args="--custom-vm-type=${CUSTOM_VM_TYPE} --custom-cpu=${CUSTOM_CPU} --custom-memory=${CUSTOM_MEMORY}"
    fi
else
    machine_type_args="--machine-type=${MACHINE_TYPE}"
fi
VM_SUFFIX=${VM_SUFFIX:-}

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

# This timeout was just a coverall for GCE issues that have since been resolved
# in Jenkins. But if creating a VM instance from the template, it is best not to set
# because the VM instance would have been deleted after the duration.
# 0 indicates not to limit run duration.
declare max_run_duration_args=""
if [ "${MAX_RUN_DURATION}" != '0' ]; then
    max_run_duration_args="--max-run-duration=${MAX_RUN_DURATION} --instance-termination-action=delete"
fi

# Increase the IAP TCP upload bandwidth
# shellcheck disable=SC2155
export PATH=$PATH:$(gcloud info --format="value(basic.python_location)")
$(gcloud info --format="value(basic.python_location)") -m pip install --upgrade pip --no-warn-script-location > /dev/null 2>&1 || true
$(gcloud info --format="value(basic.python_location)") -m pip install numpy --no-warn-script-location > /dev/null 2>&1 || true
export CLOUDSDK_PYTHON_SITEPACKAGES=1

# Catch Ctrl+C and terminate all
trap terminate SIGINT
function terminate() {
    echo -e "${RED}CTRL+C: exit requested!${NC}"
    exit 1
}

# Progress spinner. Wait for PID to complete.
function progress_spinner() {
    local -r spinner='-\|/'
    while sleep 0.1; do
        i=$(( (i+1) %4 ))
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
        # Ensure we cleanup leftovers
        delete_instances
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
    echo "ANDROID_CUTTLEFISH_PREBUILT=${ANDROID_CUTTLEFISH_PREBUILT}"
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
    echo "IMAGE=${IMAGE}"
    echo "JAVA_VERSION=${JAVA_VERSION}"
    echo "JENKINS_NAMESPACE=${JENKINS_NAMESPACE}"
    echo "JENKINS_PRIVATE_SSH_KEY_NAME=${JENKINS_PRIVATE_SSH_KEY_NAME}"
    echo "JENKINS_SSH_PUB_KEY_FILE=${JENKINS_SSH_PUB_KEY_FILE}"
    echo "MACHINE_TYPE=${MACHINE_TYPE}"
    echo "MAX_RUN_DURATION=${MAX_RUN_DURATION}"
    echo "NETWORK=${NETWORK}"
    echo "NODEJS_VERSION=${NODEJS_VERSION}"
    echo "OS_PROJECT=${OS_PROJECT}"
    echo "OS_VERSION=${OS_VERSION}"
    echo "PROJECT=${PROJECT}"
    echo "REGION=${REGION}"
    echo "SERVICE_ACCOUNT=${SERVICE_ACCOUNT}"
    echo "SUBNET=${SUBNET}"
    echo "VM_INSTANCE_CREATE=${VM_INSTANCE_CREATE}"
    echo "VM_SUFFIX=${VM_SUFFIX}"
    echo "WORKSPACE=${WORKSPACE}"
    echo "ZONE=${ZONE}"
    echo
}

function print_usage() {
    echo "Usage:
      ANDROID_CUTTLEFISH_PREBUILT=${ANDROID_CUTTLEFISH_PREBUILT} \\
      ARCHITECTURE=${ARCHITECTURE} \\
      ADDITIONAL_NETWORKING=${ADDITIONAL_NETWORKING} \\
      CURL_UPDATE_COMMAND=${CURL_UPDATE_COMMAND} \\
      CUSTOM_VM_TYPE=${CUSTOM_VM_TYPE} \\
      CUSTOM_CPU=${CUSTOM_CPU} \\
      CUSTOM_MEMORY=${CUSTOM_MEMORY} \\
      CUTTLEFISH_INSTANCE_NAME=${cuttlefish_name} \\
      CUTTLEFISH_REVISION=${CUTTLEFISH_REVISION} \\
      CUTTLEFISH_URL=${CUTTLEFISH_URL} \\
      CUTTLEFISH_POST_COMMAND=${CUTTLEFISH_POST_COMMAND} \\
      BOOT_DISK_SIZE=${BOOT_DISK_SIZE} \\
      BOOT_DISK_TYPE=${BOOT_DISK_TYPE} \\
      IMAGE=${IMAGE} \\
      JAVA_VERSION=${JAVA_VERSION} \\
      JENKINS_NAMESPACE=${JENKINS_NAMESPACE} \\
      JENKINS_PRIVATE_SSH_KEY_NAME=${JENKINS_PRIVATE_SSH_KEY_NAME} \\
      JENKINS_SSH_PUB_KEY_FILE=${JENKINS_SSH_PUB_KEY_FILE} \\
      MACHINE_TYPE=${MACHINE_TYPE} \\
      MAX_RUN_DURATION=${MAX_RUN_DURATION} \\
      NETWORK=${NETWORK} \\
      NODEJS_VERSION=${NODEJS_VERSION} \\
      OS_PROJECT=${OS_PROJECT} \\
      OS_VERSION=${OS_VERSION} \\
      PROJECT=${PROJECT} \\
      REGION=${REGION} \\
      SERVICE_ACCOUNT=${SERVICE_ACCOUNT} \\
      SUBNET=${SUBNET} \\
      VM_INSTANCE_CREATE=${VM_INSTANCE_CREATE} \\
      VM_SUFFIX=${VM_SUFFIX} \\
      WORKSPACE=${WORKSPACE} \\
      ZONE=${ZONE} \\
      ./${SCRIPT_NAME}"
    echo "Use defaults or override environment variables."
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

# Create the initial template boot disk
function create_base_template_instance() {
    echo_formatted "1. Create base template"
    yes Y | gcloud compute instance-templates delete "${vm_base_instance_template}" >/dev/null 2>&1 || true
    # shellcheck disable=SC2086
    gcloud compute instance-templates create "${vm_base_instance_template}" \
        --description="Instance template: ${vm_base_instance_template}" \
        --shielded-integrity-monitoring \
        --key-revocation-action-type=none \
        --service-account="${SERVICE_ACCOUNT}" \
        ${machine_type_args} \
        --maintenance-policy=TERMINATE \
        --image-project="${OS_PROJECT}" \
        --create-disk=mode=rw,architecture="${ARCHITECTURE}",boot=yes,size="${BOOT_DISK_SIZE}",auto-delete=true,type="${BOOT_DISK_TYPE}",device-name="${vm_base_instance}",image="${IMAGE}",image-project="${OS_PROJECT}",interface=SCSI \
        --metadata=enable-oslogin=true \
        --reservation-affinity=any \
        --enable-nested-virtualization \
        --region="${REGION}" \
        --network-interface=network="${NETWORK}",subnet="${SUBNET}",stack-type=IPV4_ONLY,no-address"${ADDITIONAL_NETWORKING}" \
        ${max_run_duration_args} >/dev/null &
    progress_spinner "$!"
    echo -e "${GREEN}Instance template ${vm_base_instance_template} created${NC}"
}

# Create a VM instance from the base tenplate instance.
function create_vm_instance() {
    echo_formatted "2. Create VM Instance from base template"
    yes Y | gcloud compute instances delete "${vm_base_instance}" \
        --zone="${ZONE}" >/dev/null 2>&1 || true &
    progress_spinner "$!"

    gcloud compute instances create "${vm_base_instance}" \
        --source-instance-template "${vm_base_instance_template}" \
        --zone="${ZONE}" &
    progress_spinner "$!"

    echo -e "${ORANGE}Sleep for 3 minutes while instance stabilises${NC}"; echo
    sleep 3m
    echo -e "${GREEN}VM Instance ${vm_base_instance} created${NC}"
}

# Install host tools on the base VM instance.
# Host must be rebooted when installed (use stop/start to achieve it)
function install_host_tools() {
    echo_formatted "3. Populate Cuttlefish Host tools/packages on VM instance"

    # https://cloud.google.com/compute/docs/troubleshooting/troubleshoot-os-login#invalid_argument
    # Clean old SSH keys
    echo -e "${GREEN}Remove old SSH keys${NC}"
    for k in $(gcloud compute os-login ssh-keys list --format="table[no-heading](value.fingerprint)"); do
        gcloud compute os-login ssh-keys remove --key "${k}" || true
        sleep 1m
    done

    gcloud compute ssh --quiet --zone="${ZONE}" "${vm_base_instance}" --tunnel-through-iap --project "${PROJECT}" --command="" --ssh-flag="-T" >/dev/null 2>&1 || true
    echo -e "${GREEN}Create CF directory for scripts${NC}"
    gcloud compute ssh --zone "${ZONE}" "${vm_base_instance}" --tunnel-through-iap --project "${PROJECT}" \
        --command='mkdir -p cf' >/dev/null &
    progress_spinner "$!"

    echo -e "${GREEN}Copy CF host install scripts${NC}"
    gcloud compute scp "${CF_SCRIPT_PATH}"/*.sh "${vm_base_instance}":~/cf/ --zone="${ZONE}" \
        --tunnel-through-iap --project "${PROJECT}" >/dev/null &
    progress_spinner "$!"

    # Keep debug so we can see what's happening.
    echo -e "${GREEN}Installing CF host ....${NC}"
    if ! gcloud compute ssh --zone "${ZONE}" "${vm_base_instance}" --tunnel-through-iap --project "${PROJECT}" \
        --command="CUTTLEFISH_REVISION=${CUTTLEFISH_REVISION} \
        ANDROID_CUTTLEFISH_PREBUILT=${ANDROID_CUTTLEFISH_PREBUILT} \
        ARCHITECTURE=${ARCHITECTURE} \
        CURL_UPDATE_COMMAND=\"${CURL_UPDATE_COMMAND}\" \
        CTS_ANDROID_16_URL=${CTS_ANDROID_16_URL} \
        CTS_ANDROID_15_URL=${CTS_ANDROID_15_URL} \
        CTS_ANDROID_14_URL=${CTS_ANDROID_14_URL} \
        CUTTLEFISH_POST_COMMAND=\"${CUTTLEFISH_POST_COMMAND}\" \
        CUTTLEFISH_URL=${CUTTLEFISH_URL} \
        CUTTLEFISH_REPO_URL=${CUTTLEFISH_REPO_URL} \
        JAVA_VERSION=${JAVA_VERSION} \
        NODEJS_VERSION=${NODEJS_VERSION} \
        OS_VERSION=${OS_VERSION} \
        REPO_USERNAME=${REPO_USERNAME} \
        REPO_PASSWORD=${REPO_PASSWORD} \
        WORKSPACE=\"${WORKSPACE}\" \
        ./cf/cf_host_initialise.sh; exit \$?"; then
        echo -e "${RED}Installing CF host failed.${NC}"
        delete_instances
        exit 1
    fi
    echo -e "${GREEN}Installing CF host completed.${NC}"

    gcloud compute ssh --quiet --zone="${ZONE}" "${vm_base_instance}" --tunnel-through-iap --project "${PROJECT}" --command="sudo ufw allow 22 || true" --ssh-flag="-T"  >/dev/null 2>&1|| true
    echo -e "${GREEN}Copying ${CUTTLEFISH_LATEST_SHA1_FILENAME}${NC}"
    gcloud compute scp "${vm_base_instance}":~/"${CUTTLEFISH_LATEST_SHA1_FILENAME}" "${WORKSPACE}"/ --zone="${ZONE}" \
         --tunnel-through-iap --project "${PROJECT}" >/dev/null 2>&1 || true

    echo -e "${GREEN}Cleanup CF host files.${NC}"
    gcloud compute ssh --zone "${ZONE}" "${vm_base_instance}" --tunnel-through-iap --project "${PROJECT}" \
        --command="rm -rf ~/cf && sudo ufw allow 22 || true" >/dev/null 2>&1 || true

    # Alternative to reboot instance. Must be rebooted/restarted to ensure
    # user/groups are applied correctly before image is created from the
    # instance.
    echo -e "${GREEN}Rebooting VM instance ${vm_base_instance}${NC}"
    gcloud compute instances stop "${vm_base_instance}" --discard-local-ssd=false \
        --zone="${ZONE}" >/dev/null 2>&1 &
    progress_spinner "$!"

    gcloud compute instances start "${vm_base_instance}" --zone="${ZONE}" >/dev/null 2>&1 &
    progress_spinner "$!"

    echo -e "${ORANGE}Sleep for 3 minutes while instance reboot completes.${NC}"; echo
    sleep 3m
    echo -e "${GREEN}VM instance ${vm_base_instance} rebooted!${NC}"
}

# Add SSH key for Jenkins.
# - Ensure the private key is added to Jenkins credentials that are
#   used for GCE plugin and SSH for Cuttlefish template instances.
function create_ssh_key() {
    echo_formatted "4. Create jenkins SSH Key on VM instance"

    # Jenkins will extract from credentials and if not present, extract
    # from k8s secrets. Useful for running locally outside of Jenkins.
    if [ ! -f "${JENKINS_SSH_PUB_KEY_FILE}" ]; then
        echo -e "${GREEN}Extracting public key ${JENKINS_SSH_PUB_KEY_FILE}${NC}"
        # Extract the public key from the private key.
        # - Use template arg to extract the private key and decode the base64.
        # - Append new line and correct file permissions so ssh-keygen
        #   can read and extract the public key.
        # shellcheck disable=SC1083
        kubectl get secrets -n "${JENKINS_NAMESPACE}" "${JENKINS_PRIVATE_SSH_KEY_NAME}" \
            --template={{.data.privateKey}} | base64 -d | \
            awk '1; END {print ""}' > jenkins_rsa || true
        chmod 400 jenkins_rsa || true

        ssh-keygen -y -f jenkins_rsa > "${JENKINS_SSH_PUB_KEY_FILE}" || true
        rm -f jenkins_rsa || true

        if [ ! -f "${JENKINS_SSH_PUB_KEY_FILE}" ]; then
            echo -e "${RED}ERROR: Failed to extract public key from private key${NC}"
            return 1
        fi
    else
        echo -e "${GREEN}Using local public key ${JENKINS_SSH_PUB_KEY_FILE}${NC}"
    fi

    echo -e "${GREEN}SSH Public key:${NC}"
    cat "${JENKINS_SSH_PUB_KEY_FILE}"

    gcloud compute ssh --quiet --zone="${ZONE}" "${vm_base_instance}" --tunnel-through-iap --project "${PROJECT}" --command="" --ssh-flag="-T" || true
    gcloud compute ssh --zone "${ZONE}" "${vm_base_instance}" --tunnel-through-iap \
        --project "${PROJECT}" \
        --command='sudo rm -rf /home/jenkins/.ssh && sudo mkdir /home/jenkins/.ssh && sudo chmod 700 /home/jenkins/.ssh && sudo chown jenkins:jenkins /home/jenkins/.ssh' >/dev/null &
    progress_spinner "$!"

    gcloud compute scp "${JENKINS_SSH_PUB_KEY_FILE}" "${vm_base_instance}":/tmp/authorized_keys \
        --zone="${ZONE}" >/dev/null --tunnel-through-iap --project "${PROJECT}" &
    progress_spinner "$!"

    gcloud compute ssh --zone "${ZONE}" "${vm_base_instance}" --tunnel-through-iap \
        --project "${PROJECT}" \
        --command='sudo mv /tmp/authorized_keys /home/jenkins/.ssh/authorized_keys && sudo chown -R jenkins:jenkins /home/jenkins/.ssh' >/dev/null &
    progress_spinner "$!"

    # Clean up
    rm -f "${JENKINS_SSH_PUB_KEY_FILE}"
    echo -e "${GREEN}SSH key installed.${NC}"
}

# Create the final Cuttlefish template for use with Jenkins GCE plugin
# allowing Cuttlefish to run on the Jenkins VM Instance.
function create_cuttlefish_boilerplate_template() {
    echo_formatted "5. Create Cuttlefish boilerplate template instance from VM Instance"
    echo -e "${GREEN}Stopping ${vm_base_instance}${NC}"
    gcloud compute instances stop "${vm_base_instance}" --zone="${ZONE}" >/dev/null 2>&1 || true &
    progress_spinner "$!"

    echo -e "${GREEN}Deleting ${vm_cuttlefish_image}${NC}"
    yes Y | gcloud compute images delete "${vm_cuttlefish_image}" >/dev/null 2>&1 || true &
    progress_spinner "$!"

    echo -e "${GREEN}Creating ${vm_cuttlefish_image}${NC}"
    gcloud compute images create "${vm_cuttlefish_image}" \
        --source-disk="${vm_base_instance}" \
        --source-disk-zone="${ZONE}" \
        --storage-location="${REGION}" \
        --source-disk-project="${PROJECT}" &
    progress_spinner "$!"

    echo -e "${GREEN}Delete ${vm_base_instance}${NC}"
    yes Y | gcloud compute instances delete "${vm_base_instance}" \
        --zone="${ZONE}" >/dev/null 2>&1 || true &
    progress_spinner "$!"

    echo -e "${GREEN}Deleting ${vm_cuttlefish_instance_template}${NC}"
    yes Y | gcloud compute instance-templates delete \
        "${vm_cuttlefish_instance_template}" >/dev/null 2>&1 || true &
    progress_spinner "$!"

    echo -e "${GREEN}Creating ${vm_cuttlefish_instance_template}${NC}"
    # shellcheck disable=SC2086
    gcloud compute instance-templates create "${vm_cuttlefish_instance_template}" \
        --description="${vm_cuttlefish_instance_template}" \
        --shielded-integrity-monitoring \
        --key-revocation-action-type=none \
        --service-account="${SERVICE_ACCOUNT}" \
        ${machine_type_args} \
        --maintenance-policy=TERMINATE \
        --image-project="${OS_PROJECT}" \
        --create-disk=image="${vm_cuttlefish_image}",boot=yes,auto-delete=yes,type="${BOOT_DISK_TYPE}" \
        --metadata=enable-oslogin=true \
        --reservation-affinity=any \
        --enable-nested-virtualization \
        --region="${REGION}" \
        --network-interface network="${NETWORK}",subnet="${SUBNET}",stack-type=IPV4_ONLY,no-address"${ADDITIONAL_NETWORKING}" \
        ${max_run_duration_args} &
    progress_spinner "$!"

    # Check the instance template was created.
    template_exists=$(gcloud compute instance-templates list --filter="name=${vm_cuttlefish_instance_template}" --format='get(name)')
    if [ "${template_exists}" != "${vm_cuttlefish_instance_template}" ]; then
       echo -e "${RED}ERROR: Failed to create template: ${vm_cuttlefish_instance_template}, review logs.${NC}"
       return 1
    else
       echo -e "${GREEN}Instance Template ${vm_cuttlefish_instance_template} created${NC}"
    fi

    echo -e "${GREEN}Deleting ${vm_cuttlefish_instance}${NC}"
    # Delete and Recreate a VM instance for local tests.
    yes Y | gcloud compute instances delete "${vm_cuttlefish_instance}" \
        --zone="${ZONE}" >/dev/null 2>&1 || true &
    progress_spinner "$!"

    if [ "${VM_INSTANCE_CREATE}" = true ]; then
        gcloud compute instances create "${vm_cuttlefish_instance}" \
            --source-instance-template "${vm_cuttlefish_instance_template}" \
            --zone="${ZONE}" &
        progress_spinner "$!"
        echo -e "${GREEN}VM Instance ${vm_cuttlefish_instance} created${NC}"

        # Stop the VM instance.
        gcloud compute instances stop "${vm_cuttlefish_instance}" \
            --zone="${ZONE}" >/dev/null 2>&1 || true &
        progress_spinner "$!"
        echo -e "${GREEN}VM Instance ${vm_cuttlefish_instance} stopped${NC}"
    fi

    echo -e "${GREEN}Deleting ${vm_base_instance_template}${NC}"
    # Delete the base template
    yes Y | gcloud compute instance-templates delete \
        "${vm_base_instance_template}" >/dev/null 2>&1 || true &
    progress_spinner "$!"

    echo -e "${GREEN}Cuttlefish boilerplate template instance completed."
}

# Delete all VM instances and artifacts
function delete_instances() {
    echo_formatted "6. Delete VM instances and artifacts"

    yes Y | gcloud compute instance-templates delete "${vm_base_instance_template}" >/dev/null 2>&1 || true
    echo_formatted "   Deleted ${vm_base_instance_template}"

    yes Y | gcloud compute instance-templates delete "${vm_cuttlefish_instance_template}" >/dev/null 2>&1 || true
    echo_formatted "   Deleted ${vm_cuttlefish_instance_template}"

    yes Y | gcloud compute images delete "${vm_cuttlefish_image}" >/dev/null 2>&1 || true
    echo_formatted "   Deleted ${vm_cuttlefish_image}"

    gcloud compute instances stop "${vm_base_instance}" --zone="${ZONE}" >/dev/null 2>&1 || true
    echo_formatted "   Stopped ${vm_base_instance}"

    yes Y | gcloud compute instances delete "${vm_base_instance}" --zone="${ZONE}" >/dev/null 2>&1 || true
    echo_formatted "   Deleted ${vm_base_instance}"

    gcloud compute instances stop "${vm_cuttlefish_instance}" --zone="${ZONE}" >/dev/null 2>&1 || true
    echo_formatted "   Stopped ${vm_cuttlefish_instance}"

    yes Y | gcloud compute instances delete "${vm_cuttlefish_instance}" --zone="${ZONE}" >/dev/null 2>&1 || true
    echo_formatted "   Deleted ${vm_cuttlefish_instance}"
}

# Main: run all or allow the user to select which steps to run.
function main() {
    echo_environment
    check_environment
    case "$1" in
        1)  create_base_template_instance ;;
        2)  create_vm_instance ;;
        3)  install_host_tools ;;
        4)  if ! create_ssh_key; then
                delete_instances # Clean up on SSH error
                exit 1
            fi
            ;;
        5)  if ! create_cuttlefish_boilerplate_template; then
                delete_instances # Clean up on error
                exit 1
            fi
            ;;
        6)  delete_instances ;;
        *h*)
            print_usage
            exit 0
            ;;
        *)  create_base_template_instance
            create_vm_instance
            install_host_tools
            if ! create_ssh_key; then
                delete_instances # Clean up on SSH error
                exit 1
            fi

            if ! create_cuttlefish_boilerplate_template; then
                delete_instances # Clean up on error
                exit 1
            fi
            echo_formatted "Done. Please check the output above and enjoy Cuttlefish!"
            ;;
    esac
}

main "$1"
