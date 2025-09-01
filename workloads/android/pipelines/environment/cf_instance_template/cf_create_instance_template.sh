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
#  CUTTLEFISH_REVISION=v1.18.0 ./cf_create_instance_template.sh && \
#  CUTTLEFISH_REVISION=main ./cf_create_instance_template.sh
#
# The following variables are required to run the script, choose to use
# default values or override from command line.
#
#  - CUTTLEFISH_REVISION: the branch/tag version of Android Cuttlefish
#        to use. Default: main
#  - BOOT_DISK_SIZE: Disk image size in GB. Default: 200GB
#  - DEBIAN_OS_VERSION: Default: debian-12-bookworm-v20250812
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
#  - MACHINE_TYPE: The machine type to create instance templates for. Default:
#        n1-standard-64
#  - MAX_RUN_DURATION: Limits how long this VM instance can run. Default: 10h
#  - NETWORK: The name of the VPC network. Default: sdv-network
#  - NODEJS_VERSION: The version of nodejs to install. Default: 20.9.0
#  - PROJECT: The GCP project. Default: derived from gcloud config.
#  - REGION: The GCP region. Default: europe-west1
#  - SERVICE_ACCOUNT: The GCP service account. Default: derived from gcloud
#        projects describe.
#  - SUBNET: The name of the subnet. Default: sdv-subnet
#  - CUTTLEFISH_INSTANCE_UNIQUE_NAME: The name used to identify the instance.
#        Default: cuttlefish-vm
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

# Environment variables that can be overridden from command line.
# android-cuttlefish revisions can be of the form v1.7.0, main etc.
CUTTLEFISH_INSTANCE_UNIQUE_NAME=${CUTTLEFISH_INSTANCE_UNIQUE_NAME:-cuttlefish-vm}
CUTTLEFISH_INSTANCE_UNIQUE_NAME=$(echo "${CUTTLEFISH_INSTANCE_UNIQUE_NAME}" | awk '{print tolower($0)}' | xargs)
CUTTLEFISH_REVISION=${CUTTLEFISH_REVISION:-main}
CUTTLEFISH_REVISION=$(echo "${CUTTLEFISH_REVISION}" | xargs)
BOOT_DISK_SIZE=${BOOT_DISK_SIZE:-200GB}
BOOT_DISK_SIZE=$(echo "${BOOT_DISK_SIZE}" | awk '{print toupper($0)}' | xargs)
DEBIAN_OS_VERSION=${DEBIAN_OS_VERSION:-debian-12-bookworm-v20250812}
DEBIAN_OS_VERSION=$(echo "${DEBIAN_OS_VERSION}" | xargs)
JENKINS_NAMESPACE=${JENKINS_NAMESPACE:-jenkins}
JENKINS_PRIVATE_SSH_KEY_NAME=${JENKINS_PRIVATE_SSH_KEY_NAME:-jenkins-cuttlefish-vm-ssh-private-key}
JENKINS_SSH_PUB_KEY_FILE=${JENKINS_SSH_PUB_KEY_FILE:-jenkins_rsa.pub}
MACHINE_TYPE=${MACHINE_TYPE:-n1-standard-64}
MACHINE_TYPE=$(echo "${MACHINE_TYPE}" | xargs)
MAX_RUN_DURATION=${MAX_RUN_DURATION:-10h}
NETWORK=${NETWORK:-sdv-network}
NODEJS_VERSION=${NODEJS_VERSION:-20.9.0}
NODEJS_VERSION=$(echo "${NODEJS_VERSION}" | xargs)
PROJECT=${PROJECT:-$(gcloud config list --format 'value(core.project)'|head -n 1)}
REGION=${REGION:-europe-west1}
SERVICE_ACCOUNT=${SERVICE_ACCOUNT:-$(gcloud projects describe "${PROJECT}" --format='get(projectNumber)')-compute@developer.gserviceaccount.com}
SUBNET=${SUBNET:-sdv-subnet}
VM_INSTANCE_CREATE=${VM_INSTANCE_CREATE:-true}
ZONE=${ZONE:-europe-west1-d}

# Instance names can only include specific characters, drop '.'.
declare -r vm_base_instance=vm-debian
declare -r vm_base_instance_template=instance-template-vm-debian
declare -r cuttlefish_version=${CUTTLEFISH_REVISION//./}
declare cuttlefish_unique_name=${CUTTLEFISH_INSTANCE_UNIQUE_NAME//./-}
if [[ "${cuttlefish_unique_name}" == "cuttlefish-vm" ]]; then
    # If unique name is default, append version.
    cuttlefish_unique_name="${cuttlefish_unique_name}"-"${cuttlefish_version}"
fi
declare -r vm_cuttlefish_image=image-"${cuttlefish_unique_name}"
declare -r vm_cuttlefish_instance_template=instance-template-"${cuttlefish_unique_name}"
declare -r vm_cuttlefish_instance="${cuttlefish_unique_name}"

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

# Colours for logging.
if [ -z "${WORKSPACE}" ]; then
    GREEN='\033[0;32m'
    ORANGE='\033[0;33m'
    RED='\033[0;31m'
    NC='\033[0m'
else
    GREEN=''
    ORANGE=''
    RED=''
    NC=''
fi
SCRIPT_NAME=$(basename "$0")

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
}

# Echo formatted output.
function echo_formatted() {
    echo -e "\r${GREEN}[$SCRIPT_NAME] $1${NC}"
}

# Echo environment variables.
function echo_environment() {
    echo_formatted "Environment variables:"
    echo_formatted "CUTTLEFISH_REVISION=${CUTTLEFISH_REVISION}"
    echo_formatted "BOOT_DISK_SIZE=${BOOT_DISK_SIZE}"
    echo_formatted "DEBIAN_OS_VERSION=${DEBIAN_OS_VERSION}"
    echo_formatted "JENKINS_NAMESPACE=${JENKINS_NAMESPACE}"
    echo_formatted "JENKINS_PRIVATE_SSH_KEY_NAME=${JENKINS_PRIVATE_SSH_KEY_NAME}"
    echo_formatted "JENKINS_SSH_PUB_KEY_FILE=${JENKINS_SSH_PUB_KEY_FILE}"
    echo_formatted "MACHINE_TYPE=${MACHINE_TYPE}"
    echo_formatted "MAX_RUN_DURATION=${MAX_RUN_DURATION}"
    echo_formatted "NETWORK=${NETWORK}"
    echo_formatted "PROJECT=${PROJECT}"
    echo_formatted "REGION=${REGION}"
    echo_formatted "SERVICE_ACCOUNT=${SERVICE_ACCOUNT}"
    echo_formatted "SUBNET=${SUBNET}"
    echo_formatted "CUTTLEFISH_INSTANCE_UNIQUE_NAME=${cuttlefish_unique_name}"
    echo_formatted "VM_INSTANCE_CREATE=${VM_INSTANCE_CREATE}"
    echo_formatted "ZONE=${ZONE}"
}

function print_usage() {
    echo "Usage:
      CUTTLEFISH_REVISION=${CUTTLEFISH_REVISION} \\
      BOOT_DISK_SIZE=${BOOT_DISK_SIZE} \\
      DEBIAN_OS_VERSION=${DEBIAN_OS_VERSION} \\
      JENKINS_NAMESPACE=${JENKINS_NAMESPACE} \\
      JENKINS_PRIVATE_SSH_KEY_NAME=${JENKINS_PRIVATE_SSH_KEY_NAME} \\
      JENKINS_SSH_PUB_KEY_FILE=${JENKINS_SSH_PUB_KEY_FILE} \\
      MACHINE_TYPE=${MACHINE_TYPE} \\
      MAX_RUN_DURATION=${MAX_RUN_DURATION} \\
      NETWORK=${NETWORK} \\
      PROJECT=${PROJECT} \\
      REGION=${REGION} \\
      SERVICE_ACCOUNT=${SERVICE_ACCOUNT} \\
      SUBNET=${SUBNET} \\
      CUTTLEFISH_INSTANCE_UNIQUE_NAME=${cuttlefish_unique_name} \\
      VM_INSTANCE_CREATE=${VM_INSTANCE_CREATE} \\
      ZONE=${ZONE} \\
      ./${SCRIPT_NAME}"
    echo "Use defaults or override environment variables."
}

# Check environment.
function check_environment() {
    if [ -z "${PROJECT}" ]; then
        echo -r "${RED}Environment variable PROJECT must be defined${NC}"
        exit 1
    fi
    if [ -z "${SERVICE_ACCOUNT}" ]; then
        echo -r "${RED}Environment variable SERVICE_ACCOUNT must be defined${NC}"
        exit 1
    fi
    if [[ "${cuttlefish_unique_name}" != cuttlefish-vm* ]]; then
        echo "CUTTLEFISH_INSTANCE_UNIQUE_NAME must start with cuttlefish-vm"
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
        --machine-type="${MACHINE_TYPE}" \
        --image-project=debian-cloud \
        --create-disk=mode=rw,architecture=X86_64,boot=yes,size="${BOOT_DISK_SIZE}",auto-delete=true,type=pd-balanced,device-name="${vm_base_instance}",image=projects/debian-cloud/global/images/"${DEBIAN_OS_VERSION}",interface=SCSI \
        --tags=http-server,https-server \
        --metadata=enable-oslogin=true \
        --reservation-affinity=any \
        --enable-nested-virtualization \
        --region="${REGION}" \
        --network-interface=network="${NETWORK}",subnet="${SUBNET}",stack-type=IPV4_ONLY,no-address \
        ${max_run_duration_args} >/dev/null 2>&1 &
    progress_spinner "$!"
    echo -e "${ORANGE}Instance template ${vm_base_instance_template} created${NC}"
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

    echo -e "${ORANGE}Sleep for 2 minutes while instance stabilises${NC}"
    sleep 120
    echo -e "${ORANGE}VM Instance ${vm_base_instance} created${NC}"
}

# Install host tools on the base VM instance.
# Host must be rebooted when installed (use stop/start to achieve it)
function install_host_tools() {
    echo_formatted "3. Populate Cuttlefish Host tools/packages on VM instance"

    # https://cloud.google.com/compute/docs/troubleshooting/troubleshoot-os-login#invalid_argument
    # Clean old SSH keys
    echo -e "${ORANGE}Remove old SSH keys${NC}"
    for k in $(gcloud compute os-login ssh-keys list --format="table[no-heading](value.fingerprint)"); do
        gcloud compute os-login ssh-keys remove --key "${k}" || true
    done

    gcloud compute ssh --zone "${ZONE}" "${vm_base_instance}" --tunnel-through-iap --project "${PROJECT}" \
        --command='mkdir -p cf' >/dev/null &
    progress_spinner "$!"

    gcloud compute scp "${CF_SCRIPT_PATH}"/*.sh "${vm_base_instance}":~/cf/ --zone="${ZONE}" >/dev/null &
    progress_spinner "$!"

    # Keep debug so we can see what's happening.
    gcloud compute ssh --zone "${ZONE}" "${vm_base_instance}" --tunnel-through-iap --project "${PROJECT}" \
        --command="CUTTLEFISH_REVISION=${CUTTLEFISH_REVISION} NODEJS_VERSION=${NODEJS_VERSION} ./cf/cf_host_initialise.sh" &
    progress_spinner "$!"

    gcloud compute ssh --zone "${ZONE}" "${vm_base_instance}" --tunnel-through-iap --project "${PROJECT}" \
        --command='rm -rf cf' >/dev/null &
    progress_spinner "$!"


    # Alternative to reboot instance. Must be rebooted/restarted to ensure
    # user/groups are applied correctly before image is created from the
    # instance.
    echo -e "${ORANGE}Rebooting VM instance ${vm_base_instance}${NC}"
    gcloud compute instances stop "${vm_base_instance}" --discard-local-ssd=false \
        --zone="${ZONE}" >/dev/null 2>&1 &
    progress_spinner "$!"

    gcloud compute instances start "${vm_base_instance}" --zone="${ZONE}" >/dev/null 2>&1 &
    progress_spinner "$!"

    echo -e "${ORANGE}Sleep for 2 minutes while instance reboot completes.${NC}"
    sleep 120
    echo -e "${ORANGE}VM instance ${vm_base_instance} rebooted!${NC}"
}

# Add SSH key for Jenkins.
# - Ensure the private key is added to Jenkins credentials that are
#   used for GCE plugin and SSH for Cuttlefish template instances.
function create_ssh_key() {
    echo_formatted "4. Create jenkins SSH Key on VM instance"

    # Jenkins will extract from credentials and if not present, extract
    # from k8s secrets. Useful for running locally outside of Jenkins.
    if [ ! -f "${JENKINS_SSH_PUB_KEY_FILE}" ]; then
        echo -e "${ORANGE}Extracting public key ${JENKINS_SSH_PUB_KEY_FILE}${NC}"
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
            echo "ERROR: Failed to extract public key from private key"
            return 1
        fi
    else
        echo -e "${ORANGE}Using local public key ${JENKINS_SSH_PUB_KEY_FILE}${NC}"
    fi

    echo -e "${ORANGE}SSH Public key:${NC}"
    cat "${JENKINS_SSH_PUB_KEY_FILE}"

    gcloud compute ssh --zone "${ZONE}" "${vm_base_instance}" --tunnel-through-iap \
        --project "${PROJECT}" \
        --command='sudo rm -rf /home/jenkins/.ssh && sudo mkdir /home/jenkins/.ssh && sudo chmod 700 /home/jenkins/.ssh && sudo chown jenkins:jenkins /home/jenkins/.ssh' >/dev/null &
    progress_spinner "$!"

    gcloud compute scp "${JENKINS_SSH_PUB_KEY_FILE}" "${vm_base_instance}":/tmp/authorized_keys \
        --zone="${ZONE}" >/dev/null &
    progress_spinner "$!"

    gcloud compute ssh --zone "${ZONE}" "${vm_base_instance}" --tunnel-through-iap \
        --project "${PROJECT}" \
        --command='sudo mv /tmp/authorized_keys /home/jenkins/.ssh/authorized_keys && sudo chown -R jenkins:jenkins /home/jenkins/.ssh' >/dev/null &
    progress_spinner "$!"

    # Clean up
    rm -f "${JENKINS_SSH_PUB_KEY_FILE}"
}

# Create the final Cuttlefish template for use with Jenkins GCE plugin
# allowing Cuttlefish to run on the Jenkins VM Instance.
function create_cuttlefish_boilerplate_template() {
    echo_formatted "5. Create Cuttlefish boilerplate template instance from VM Instance"
    gcloud compute instances stop "${vm_base_instance}" --zone="${ZONE}" >/dev/null 2>&1 || true &
    progress_spinner "$!"

    yes Y | gcloud compute images delete "${vm_cuttlefish_image}" >/dev/null 2>&1 || true &
    progress_spinner "$!"

    echo -e "${ORANGE}Sleep for 1 minute while image deletion completes${NC}"
    sleep 60

    gcloud compute images create "${vm_cuttlefish_image}" \
        --source-disk="${vm_base_instance}" \
        --source-disk-zone="${ZONE}" \
        --storage-location="${REGION}" \
        --source-disk-project="${PROJECT}" &
    progress_spinner "$!"

    echo -e "${ORANGE}Sleep for 1 minute while image creation completes${NC}"
    sleep 60
    echo -e "${ORANGE}Image ${vm_cuttlefish_image} created${NC}"

    yes Y | gcloud compute instances delete "${vm_base_instance}" \
        --zone="${ZONE}" >/dev/null 2>&1 || true &
    progress_spinner "$!"

    echo -e "${ORANGE}Sleep for 1 minute while instance deletion completes${NC}"
    sleep 60

    yes Y | gcloud compute instance-templates delete \
        "${vm_cuttlefish_instance_template}" >/dev/null 2>&1 || true &
    progress_spinner "$!"

    echo -e "${ORANGE}Sleep for 1 minute while instance template deletion completes${NC}"
    sleep 60

    # shellcheck disable=SC2086
    gcloud compute instance-templates create "${vm_cuttlefish_instance_template}" \
        --description="${vm_cuttlefish_instance_template}" \
        --shielded-integrity-monitoring \
        --key-revocation-action-type=none \
        --service-account="${SERVICE_ACCOUNT}" \
        --machine-type="${MACHINE_TYPE}" \
        --image-project=debian-cloud \
        --create-disk=image="${vm_cuttlefish_image}",boot=yes,auto-delete=yes,type=pd-balanced \
        --metadata=enable-oslogin=true \
        --reservation-affinity=any \
        --enable-nested-virtualization \
        --region="${REGION}" \
        --network-interface network="${NETWORK}",subnet="${SUBNET}",stack-type=IPV4_ONLY,no-address \
        ${max_run_duration_args} &
    progress_spinner "$!"

    echo -e "${ORANGE}Sleep for 4 minute while instance template creation completes, GCP settles.${NC}"
    sleep 240

    # Check the instance template was created.
    template_exists=$(gcloud compute instance-templates list --filter="name=${vm_cuttlefish_instance_template}" --format='get(name)')
     if [ "${template_exists}" != "${vm_cuttlefish_instance_template}" ]; then
       echo -r "${RED}ERROR: Failed to create template: ${vm_cuttlefish_instance_template}, review logs.${NC}"
       return 1
    else
       echo -e "${ORANGE}Instance Template ${vm_cuttlefish_instance_template} created${NC}"
    fi

    # Delete and Recreate a VM instance for local tests.
    yes Y | gcloud compute instances delete "${vm_cuttlefish_instance}" \
        --zone="${ZONE}" >/dev/null 2>&1 || true &
    progress_spinner "$!"

    if [ "${VM_INSTANCE_CREATE}" = true ]; then
        gcloud compute instances create "${vm_cuttlefish_instance}" \
            --source-instance-template "${vm_cuttlefish_instance_template}" \
            --zone="${ZONE}" &
        progress_spinner "$!"

        echo -e "${ORANGE}Sleep for 1 minute while instance creation completes${NC}"
        sleep 60
        echo -e "${ORANGE}VM Instance ${vm_cuttlefish_instance} created${NC}"

        # Stop the VM instance.
        gcloud compute instances stop "${vm_cuttlefish_instance}" \
            --zone="${ZONE}" >/dev/null 2>&1 || true &
        progress_spinner "$!"
        echo -e "${ORANGE}VM Instance ${vm_cuttlefish_instance} stopped${NC}"
    fi

    # Delete the base template
    yes Y | gcloud compute instance-templates delete \
        "${vm_base_instance_template}" >/dev/null 2>&1 || true &
    progress_spinner "$!"

}

# Delete all VM instances and artifacts
function delete_instances() {
    echo_formatted "6. Delete VM instances and artifacts"

    yes Y | gcloud compute instance-templates delete "${vm_base_instance_template}" >/dev/null 2>&1 || true &
    progress_spinner "$!"
    echo_formatted "   Deleted ${vm_base_instance_template}"

    yes Y | gcloud compute instance-templates delete "${vm_cuttlefish_instance_template}" >/dev/null 2>&1 || true &
    progress_spinner "$!"
    echo_formatted "   Deleted ${vm_cuttlefish_instance_template}"

    yes Y | gcloud compute images delete "${vm_cuttlefish_image}" >/dev/null 2>&1 || true &
    progress_spinner "$!"
    echo_formatted "   Deleted ${vm_cuttlefish_image}"

    gcloud compute instances stop "${vm_base_instance}" --zone="${ZONE}" >/dev/null 2>&1 || true &
    progress_spinner "$!"
    echo_formatted "   Stopped ${vm_base_instance}"

    yes Y | gcloud compute instances delete "${vm_base_instance}" --zone="${ZONE}" >/dev/null 2>&1 || true &
    progress_spinner "$!"
    echo_formatted "   Deleted ${vm_base_instance}"

    gcloud compute instances stop "${vm_cuttlefish_instance}" --zone="${ZONE}" >/dev/null 2>&1 || true &
    progress_spinner "$!"
    echo_formatted "   Stopped ${vm_cuttlefish_instance}"

    yes Y | gcloud compute instances delete "${vm_cuttlefish_instance}" --zone="${ZONE}" >/dev/null 2>&1 || true &
    progress_spinner "$!"
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
