#!/usr/bin/env bash

# Copyright (c) 2025 Accenture, All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#         http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

function abfs_override_tf() {
  cat > main_override.tf <<EOL
module "abfs-uploaders" {
  source = "git::${TERRAFORM_GITHUB_URL}//modules/uploaders?ref=${TERRAFORM_GITHUB_VERSION}"
}
EOL
}

# Clean old SSH keys
function abfs_clean_ssh_keys() {
  # https://cloud.google.com/compute/docs/troubleshooting/troubleshoot-os-login#invalid_argument
  echo -e "Remove old SSH keys"
  for k in $(gcloud compute os-login ssh-keys list --format="table[no-heading](value.fingerprint)"); do
    gcloud compute os-login ssh-keys remove --key "${k}" || true
  done
}

function abfs_uploader_run() {
  echo "ABFS Uploader Run"

  export TF_VAR_project_id=${CLOUD_PROJECT}
  export TF_VAR_region=${CLOUD_REGION}
  export TF_VAR_zone=${CLOUD_ZONE}
  export TF_VAR_abfs_gerrit_uploader_count=${UPLOADER_COUNT}
  export TF_VAR_abfs_gerrit_uploader_machine_type=${UPLOADER_MACHINE_TYPE}
  export TF_VAR_abfs_gerrit_uploader_datadisk_size_gb=${UPLOADER_DATADISK_SIZE_GB}
  export TF_VAR_abfs_gerrit_uploader_datadisk_type="pd-balanced"
  export TF_VAR_abfs_docker_image_uri="${DOCKER_REGISTRY_NAME}"
  export TF_VAR_abfs_gerrit_uploader_manifest_server=${UPLOADER_MANIFEST_SERVER}
  export TF_VAR_abfs_gerrit_uploader_git_branch=${UPLOADER_GIT_BRANCH}
  export TF_VAR_abfs_manifest_file=${UPLOADER_MANIFEST_FILE}
  export TF_VAR_abfs_license
  TF_VAR_abfs_license="$(echo "${ABFS_LICENSE_B64}" | base64 -d)"

  terraform init -backend-config bucket="${CLOUD_BACKEND_BUCKET}" -upgrade

  if [ "${ABFS_TERRAFORM_ACTION}" = "APPLY" ]; then
    terraform plan
    terraform apply -auto-approve

    VM_LIST=$(terraform show -json | jq -r '.values.root_module | recurse(.child_modules[]?)  | .resources[]? | select(.type == "google_compute_instance") | "\(.values.name)"' | xargs)
    for vm in $VM_LIST; do
      echo "${vm}"
      VM_STATUS=$(gcloud compute instances describe "${vm}" --zone="${CLOUD_ZONE}" --format='get(status)')
      if [[ $VM_STATUS == "RUNNING" ]]; then
        #shellcheck disable=SC2154,SC2086
        BR_L_CUR=$(gcloud compute ssh --quiet --zone=europe-west1-d --tunnel-through-iap ${vm} --command="PID=\$(ps -efww --no-headers | grep -v grep | grep \"/usr/local/bin/abfs\" | awk '{uid=\$1; pid=\$2; ppid=\$3; c=\$4; stime=\$5; tty=\$6; time=\$7; cmd=\"\"; for (i=8; i<=NF; i++) cmd=cmd $i \" \"; print pid}') && BRANCH_LIST=\$(tr '\0' '\n' < /proc/\${PID}/cmdline | awk 'found { print; exit } \$0 == \"--branch\" { found = 1 } ' | tr ',' '\n' | sort -n | uniq | xargs ) && echo \${BRANCH_LIST}")
        BR_L_NEW=$(echo "${UPLOADER_GIT_BRANCH}" | tr -d '[]"' | tr ',' '\n' | sed 's/^ *//;s/ *$//' | sort | uniq | xargs)
        echo "Current branch $BR_L_CUR, requested branch(es) $BR_L_NEW"
        if [[ "${BR_L_CUR}" != "${BR_L_NEW}" ]]; then
          gcloud compute instances reset "${vm}" --zone="${CLOUD_ZONE}"
        fi
      else
        echo "WARNING: ${vm} is not running, so has not been updated. Consider running with START/RESTART."
      fi
    done

  elif [ "${ABFS_TERRAFORM_ACTION}" = "DESTROY" ]; then
    terraform plan -destroy
    terraform destroy --auto-approve
  elif [ "${ABFS_TERRAFORM_ACTION}" = "START" ]; then
    VM_LIST=$(terraform show -json | jq -r '.values.root_module | recurse(.child_modules[]?)  | .resources[]? | select(.type == "google_compute_instance") | "\(.values.name)"' | xargs)
    for vm in $VM_LIST; do
      echo "${vm}"
      VM_STATUS=$(gcloud compute instances describe "${vm}" --zone="${CLOUD_ZONE}" --format='get(status)')
      if [[ $VM_STATUS == "TERMINATED" ]]; then
        gcloud compute instances start "${vm}" --zone="${CLOUD_ZONE}"
      fi
    done
  elif [ "${ABFS_TERRAFORM_ACTION}" = "STOP" ]; then
    VM_LIST=$(terraform show -json | jq -r '.values.root_module | recurse(.child_modules[]?)  | .resources[]? | select(.type == "google_compute_instance") | "\(.values.name)"' | xargs)
    for vm in $VM_LIST; do
      echo "${vm}"
      VM_STATUS=$(gcloud compute instances describe "${vm}" --zone="${CLOUD_ZONE}" --format='get(status)')
      if [[ $VM_STATUS == "RUNNING" ]]; then
        gcloud compute instances stop "${vm}" --zone="${CLOUD_ZONE}"
      fi
    done
  elif [ "${ABFS_TERRAFORM_ACTION}" = "RESTART" ]; then
    VM_LIST=$(terraform show -json | jq -r '.values.root_module | recurse(.child_modules[]?)  | .resources[]? | select(.type == "google_compute_instance") | "\(.values.name)"' | xargs)
    for vm in $VM_LIST; do
      echo "${vm}"
      VM_STATUS=$(gcloud compute instances describe "${vm}" --zone="${CLOUD_ZONE}" --format='get(status)')
      if [[ $VM_STATUS == "RUNNING" ]]; then
        gcloud compute instances reset "${vm}" --zone="${CLOUD_ZONE}"
      fi
    done
  else
    echo "WRONG ACTION"
  fi
}

abfs_clean_ssh_keys
abfs_override_tf
abfs_uploader_run
