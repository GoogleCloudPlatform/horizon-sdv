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
set -euo pipefail

function abfs_override_tf() {
  cat >main_override.tf <<EOL
module "abfs-server" {
  source = "git::${GOOGLE_ABFS_TERRAFORM_GIT_URL}//modules/server?ref=${GOOGLE_ABFS_TERRAFORM_VERSION}"
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

function abfs_server_run() {
  echo "ABFS Server Run"
  local restore_server_running_state="false"
  local apply_exit_code=0

  export TF_VAR_project_id="${CLOUD_PROJECT}"
  export TF_VAR_region="${CLOUD_REGION}"
  export TF_VAR_zone="${CLOUD_ZONE}"
  export TF_VAR_sdv_network="sdv-network"
  export TF_VAR_abfs_server_machine_type="${SERVER_MACHINE_TYPE}"
  export TF_VAR_abfs_docker_image_uri="${DOCKER_REGISTRY_NAME}"
  export TF_VAR_abfs_extra_params="${ABFS_EXTRA_PARAMS:-[]}"
  export TF_VAR_existing_bucket_name="${EXISTING_BUCKET_NAME:-}"
  export TF_VAR_abfs_server_cos_image_ref="${ABFS_COS_IMAGE_REF}"
  export TF_VAR_abfs_spanner_instance_min_nodes="${ABFS_SPANNER_INSTANCE_MIN_NODES:-1}"
  export TF_VAR_abfs_spanner_instance_max_nodes="${ABFS_SPANNER_INSTANCE_MAX_NODES:-10}"
  export TF_VAR_abfs_spanner_database_create_tables="${ABFS_SPANNER_DATABASE_CREATE_TABLES:-false}"
  export TF_VAR_abfs_spanner_database_schema_version="${ABFS_SPANNER_DATABASE_SCHEMA_VERSION:-0.0.31}"
  export TF_VAR_abfs_license
  TF_VAR_abfs_license="$(echo "${ABFS_LICENSE_B64}" | base64 -d)"

  if [ "${ABFS_TERRAFORM_ACTION}" = "APPLY" ] && [ "${ABFS_SPANNER_DATABASE_CREATE_TABLES:-false}" = "true" ]; then
    if gcloud --project "${CLOUD_PROJECT}" spanner databases describe abfs --instance=abfs >/dev/null 2>&1; then
      echo "ABFS Spanner database 'abfs' already exists. Set ABFS_SPANNER_DATABASE_CREATE_TABLES=false for upgrades/legacy DBs."
      exit 1
    fi
  fi

  if [ "${ABFS_TERRAFORM_ACTION}" = "APPLY" ]; then
    # Boot disk replacements (for example COS image changes) require TERMINATED state.
    if gcloud compute instances describe abfs-server --zone="${CLOUD_ZONE}" >/dev/null 2>&1; then
      VM_STATUS=$(gcloud compute instances describe abfs-server --zone="${CLOUD_ZONE}" --format='get(status)')
      if [ "${VM_STATUS}" = "RUNNING" ]; then
        echo "abfs-server is RUNNING; stopping instance before APPLY to allow boot disk updates."
        gcloud compute instances stop abfs-server --zone="${CLOUD_ZONE}"
        restore_server_running_state="true"
      fi
    fi
  fi

  terraform init -backend-config bucket="${CLOUD_BACKEND_BUCKET}" -upgrade

  if [ "${ABFS_TERRAFORM_ACTION}" = "APPLY" ]; then
    set +e
    terraform plan
    apply_exit_code=$?
    if [ "${apply_exit_code}" -eq 0 ]; then
      terraform apply -auto-approve
      apply_exit_code=$?
    fi
    set -e

    if [ "${restore_server_running_state}" = "true" ]; then
      if gcloud compute instances describe abfs-server --zone="${CLOUD_ZONE}" >/dev/null 2>&1; then
        VM_STATUS=$(gcloud compute instances describe abfs-server --zone="${CLOUD_ZONE}" --format='get(status)')
        if [ "${VM_STATUS}" != "RUNNING" ]; then
          echo "abfs-server was RUNNING before APPLY; restoring original state by starting instance."
          gcloud compute instances start abfs-server --zone="${CLOUD_ZONE}"
        fi
      else
        echo "abfs-server does not exist after APPLY; skipping state restore."
      fi
    fi

    if [ "${apply_exit_code}" -ne 0 ]; then
      exit "${apply_exit_code}"
    fi
  elif [ "${ABFS_TERRAFORM_ACTION}" = "DESTROY" ]; then
    terraform plan -destroy
    terraform destroy --auto-approve
  elif [ "${ABFS_TERRAFORM_ACTION}" = "START" ]; then
    gcloud compute instances start abfs-server --zone="${CLOUD_ZONE}"
  elif [ "${ABFS_TERRAFORM_ACTION}" = "STOP" ]; then
    gcloud compute instances stop abfs-server --zone="${CLOUD_ZONE}"
  elif [ "${ABFS_TERRAFORM_ACTION}" = "RESTART" ]; then
    gcloud compute instances reset abfs-server --zone="${CLOUD_ZONE}"
  else
    echo "WRONG ACTION"
  fi
}

abfs_clean_ssh_keys
abfs_override_tf
abfs_server_run
