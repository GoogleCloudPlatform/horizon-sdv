#!/usr/bin/env bash

# Copyright (c) 2026 Accenture, All Rights Reserved.
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
#
# Description:
# Main deployment script to setup or destroy the platform using Terraform.
# Handles both local and containerized execution environments.
# Handles authentication, workspace setup, requirement checks, and Terraform execution.
# Removes ArgoCD resources gracefully during destruction to avoid namespace hanging.

set -e

# Configuration
CONTAINER_CONFIG="/tmp/terraform.tfvars"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GKE_CLUSTER="sdv-cluster"

# Output Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO] $1${NC}"; }
log_warn() { echo -e "${YELLOW}[WARN] $1${NC}"; }
log_err() { echo -e "${RED}[ERROR] $1${NC}"; }

version_ge() {
  [ "$2" = "$(echo -e "$1\n$2" | sort -V | head -n1)" ]
}

# Help command
usage() {
  echo ""
  echo "Please use one of the valid commands below:"
  echo "  ./deploy.sh [OPTION] for e.g. ./deploy.sh -p or --plan (recommended for native runs on Linux distributions)"
  echo "  ./container-deploy.sh [OPTION] for e.g. ./container-deploy.sh -p or --plan (for containerized execution)"
  echo ""
  echo "Options:"
  echo "  -p, --plan       Run Terraform plan"
  echo "  -a, --apply      Run Terraform apply"
  echo "  -d, --destroy    Run Terraform destroy"
  echo "  -h, --help       Help message"
  echo ""
  exit 0
}

# Argument validation before any terraform logic
if [[ $# -eq 0 ]]; then
  usage
fi

case "$1" in
  -h|--help)
    usage
    ;;
  -p|--plan|-a|--apply|-d|--destroy)
    ;;
  *)
    echo "Invalid option: $1"
    usage
    ;;
esac


# Workspace Setup
setup_workspace() {
  if [[ -f "$CONTAINER_CONFIG" ]]; then
    # Clone repository if running within container
    log_info "Container environment detected. Setting up workspace..."

    # Extract SCM configuration from terraform.tfvars
    SCM_TYPE=$(grep '^\s*scm_type\s*=' "$CONTAINER_CONFIG" | cut -d'"' -f2)
    SCM_AUTH_METHOD=$(grep '^\s*scm_auth_method\s*=' "$CONTAINER_CONFIG" | cut -d'"' -f2)
    SCM_REPO_URL=$(grep '^\s*scm_repo_url\s*=' "$CONTAINER_CONFIG" | cut -d'"' -f2)
    SCM_REPO_BRANCH=$(grep '^\s*scm_repo_branch\s*=' "$CONTAINER_CONFIG" | cut -d'"' -f2)
    SCM_USERNAME=$(grep '^\s*scm_username\s*=' "$CONTAINER_CONFIG" | cut -d'"' -f2)
    SCM_USERNAME=$(echo -n "$SCM_USERNAME" | jq -sRr @uri)
    SCM_PASSWORD=$(grep '^\s*scm_password\s*=' "$CONTAINER_CONFIG" | cut -d'"' -f2)
    SCM_PASSWORD=$(echo -n "$SCM_PASSWORD" | jq -sRr @uri)

    log_info "Cloning from ${SCM_REPO_URL} (Branch: ${SCM_REPO_BRANCH})..."

    # Handle authentication based on method
    if [[ "$SCM_AUTH_METHOD" == "userpass" ]]; then
      # Build authenticated URL for username/password
      SCM_HOST=$(echo "$SCM_REPO_URL" | sed -e 's|^https://||' -e 's|/.*||')
      SCM_PATH=$(echo "$SCM_REPO_URL" | sed -e 's|^https://[^/]*/||')
      AUTH_URL="https://${SCM_USERNAME}:${SCM_PASSWORD}@${SCM_HOST}/${SCM_PATH}"
      git clone -q -b "$SCM_REPO_BRANCH" "$AUTH_URL" .
    elif [[ "$SCM_AUTH_METHOD" == "none" ]]; then
      # Public repository - no authentication
      git clone -q -b "$SCM_REPO_BRANCH" "$SCM_REPO_URL" .
    else
      # Try public clone first, then prompt if needed
      if ! git clone -q -b "$SCM_REPO_BRANCH" "$SCM_REPO_URL" . 2>/dev/null; then
        log_warn "Public clone failed. Repository might be private."
        read -s -p "Enter password/token: " MANUAL_PASSWORD
        echo ""
        if [[ -z "$MANUAL_PASSWORD" ]]; then
          log_err "No password provided."
          exit 1
        fi

        SCM_HOST=$(echo "$SCM_REPO_URL" | sed -e 's|^https://||' -e 's|/.*||')
        SCM_PATH=$(echo "$SCM_REPO_URL" | sed -e 's|^https://[^/]*/||')
        AUTH_URL="https://${SCM_USERNAME:-git}:${MANUAL_PASSWORD}@${SCM_HOST}/${SCM_PATH}"
        git clone -q -b "$SCM_REPO_BRANCH" "$AUTH_URL" .
      fi
    fi

    # Copy the terraform.tfvars file
    DEST_TFVARS="terraform/env/terraform.tfvars"
    mkdir -p "$(dirname "$DEST_TFVARS")"
    cp "$CONTAINER_CONFIG" "$DEST_TFVARS"

    # Set TF_DIR to the newly cloned location
    TF_DIR="$(pwd)/terraform/env"

  else
    # Skip cloning repository if running on local/native machine
    log_info "Local environment detected. Skipping clone."
    TF_DIR="${SCRIPT_DIR}/../../../terraform/env"

    if [[ ! -f "${TF_DIR}/terraform.tfvars" ]]; then
      log_err "Config file not found at ${TF_DIR}/terraform.tfvars"
      exit 1
    fi
  fi
} 

check_requirements() {
  log_info "Checking requirements..."

  # Required Minimum Tool Versions
  MIN_TF_VER="1.14.2"
  MIN_DOCKER_VER="29.1.3"
  MIN_GCLOUD_VER="549.0.1"

  # Terraform
  if ! command -v terraform &>/dev/null; then
    log_err "Terraform not found"
    exit 1
  fi
  TF_CURRENT=$(terraform version -json | grep -o '"terraform_version":"[^"]*"' | cut -d'"' -f4)
  if [[ -z "$TF_CURRENT" ]]; then
    TF_CURRENT=$(terraform version | head -n1 | cut -d ' ' -f 2 | tr -d 'v')
  fi
  if ! version_ge "$TF_CURRENT" "$MIN_TF_VER"; then
    log_err "Terraform version $TF_CURRENT is too old. Required: >= $MIN_TF_VER"
    exit 1
  fi
  log_info "Terraform version $TF_CURRENT verified."

  # Docker
  if [[ ! -f /.dockerenv ]]; then
    if command -v docker &>/dev/null; then
      D_CURRENT=$(docker version --format '{{.Client.Version}}' 2>/dev/null)
      if ! version_ge "$D_CURRENT" "$MIN_DOCKER_VER"; then
        log_err "Docker version $D_CURRENT is too old. Required: >= $MIN_DOCKER_VER"
        exit 1
      fi
      log_info "Docker version $D_CURRENT verified."
    fi
  fi

  # Gcloud Check
  if command -v gcloud &>/dev/null; then
    G_CURRENT=$(gcloud version --format='value(core)' 2>/dev/null)
    if ! version_ge "$G_CURRENT" "$MIN_GCLOUD_VER"; then
      log_err "Google Cloud SDK version $G_CURRENT is too old. Required: >= $MIN_GCLOUD_VER"
      exit 1
    fi
    log_info "Gcloud version $G_CURRENT verified."
  else
    log_err "Google Cloud SDK not found."
    exit 1
  fi
}

#  Authentication
check_auth() {
  # Define the path to the ADC file path (default)
  ADC_FILE="$HOME/.config/gcloud/application_default_credentials.json"

  if gcloud auth application-default print-access-token >/dev/null 2>&1; then
    NEED_LOGIN=false
  else
    log_warn "Application Default Credentials not available."
    NEED_LOGIN=true
  fi

  # Login if needed
  if [[ "$NEED_LOGIN" == "true" ]]; then
    log_info "Starting interactive login..."

    gcloud auth application-default login --no-launch-browser
    gcloud auth login --no-launch-browser

    log_info "Authentication successful."
  else
    log_info "Authentication verified (Volume)."
  fi
}

# Terraform Execution
run_terraform() {
  MODE="apply"

  case "$1" in
    -p|--plan)
      MODE="plan"
      ;;
    -a|--apply)
      MODE="apply"
      ;;
    -d|--destroy)
      MODE="destroy"
      ;;
  esac

  cd "$TF_DIR"
  TFVARS_FILE="terraform.tfvars"

  if grep -q "<REQUIRED>" "$TFVARS_FILE"; then
    log_err "You still have '<REQUIRED>' placeholders in terraform.tfvars."
    log_err "Please fill in Project ID, Secrets, and Keys."
    exit 1
  fi

  # Fetch required GCP Project details
  BACKEND_BUCKET=$(awk -F'"' '/sdv_gcp_backend_bucket/ {print $2}' "$TFVARS_FILE")
  PROJECT_ID=$(awk -F'"' '/sdv_gcp_project_id/ {print $2}' "$TFVARS_FILE")
  LOCATION=$(awk -F'"' '/sdv_gcp_region/ {print $2}' "$TFVARS_FILE")

  # Check if KMS encryption is enabled
  KMS_ENABLED=$(awk '/sdv_enable_kms_encryption/ {print $3}' "$TFVARS_FILE" | tr -d ' ')

  if [[ -z "$BACKEND_BUCKET" || -z "$PROJECT_ID" || -z "$LOCATION" ]]; then
    log_err "Failed to decode one or more required variables (Bucket, Project, or Region). Check terraform.tfvars."
    exit 1
  fi

  # Handle KMS infrastructure if encryption is enabled
    KMS_DIR="$(dirname "$TF_DIR")/kms"
    if [[ "$KMS_ENABLED" == "true" ]] && [[ -d "$KMS_DIR" ]] && [[ "$MODE" == "apply" ]] && [[ ! "$SKIP_KMS" == "true" ]]; then
        log_info "KMS encryption enabled - checking KMS infrastructure..."
        
        # Check if KMS resources exist in GCP (using explicit project/location flags)
        if ! gcloud kms keyrings describe "gke-secrets-keyring" \
            --location="$LOCATION" \
            --project="$PROJECT_ID" &>/dev/null; then
            
            log_warn "KMS infrastructure not found in GCP. Deploying persistent KMS resources..."
            
            log_info "Deploying KMS infrastructure (one-time setup)..."
            cd "$KMS_DIR"
            
            # Use absolute path to env tfvars (works in both local and container modes)
            ENV_TFVARS_PATH="$TF_DIR/terraform.tfvars"
            
            terraform init -upgrade
            terraform apply -auto-approve -var-file="$ENV_TFVARS_PATH"
            
            log_info "✓ KMS infrastructure deployed successfully"
            cd "$TF_DIR"
        else
            log_info "✓ KMS infrastructure already exists in GCP"
        fi
    elif [[ "$KMS_ENABLED" == "false" ]]; then
        log_info "KMS encryption disabled - skipping KMS infrastructure check"
    fi

  log_info "Initializing Terraform (Bucket: $BACKEND_BUCKET, Project: $PROJECT_ID)..."

  terraform init -upgrade -reconfigure \
    -backend-config="bucket=$BACKEND_BUCKET"

# Execute based on selected mode
if [[ "$MODE" == "plan" ]]; then
  log_info "Running Terraform plan..."
  terraform plan

elif [[ "$MODE" == "destroy" ]]; then
  log_warn "!!! DESTRUCTION MODE ENABLED !!!"
  log_info "Running Terraform destroy..."
  log_info "Note: KMS resources (if any) will persist in GCP (separate state)"
  terraform destroy -auto-approve

else
  log_info "Running Terraform Apply..."
  terraform apply -auto-approve
fi
}

setup_workspace
check_requirements
check_auth
run_terraform "$@"