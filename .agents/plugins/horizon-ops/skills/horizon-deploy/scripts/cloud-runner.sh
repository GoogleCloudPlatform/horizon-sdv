#!/usr/bin/env bash

# Copyright (c) 2026 Accenture, All Rights Reserved.
# Modifications Copyright 2026 Google LLC
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
# Cloud Build container runner for Horizon SDV Terraform deployment.
# Mirrors tools/scripts/deployment/deploy.sh in container mode: clones
# scm_repo_url at scm_repo_branch, copies terraform.tfvars into the clone
# and runs Terraform from there.

set -euo pipefail

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO] $1${NC}"; }
log_warn() { echo -e "${YELLOW}[WARN] $1${NC}"; }
log_err() { echo -e "${RED}[ERROR] $1${NC}"; }

SOURCE_TFVARS="/workspace/terraform/env/terraform.tfvars"
CLONE_DIR="/workspace/horizon-sdv-clone"

case "${1:-}" in
  -p|--plan)
    MODE="plan"
    ;;
  -a|--apply)
    MODE="apply"
    ;;
  -d|--destroy)
    MODE="destroy"
    ;;
  *)
    log_err "Invalid or missing option: '${1:-}'. Use -p, -a or -d."
    exit 1
    ;;
esac

if [[ ! -f "$SOURCE_TFVARS" ]]; then
  log_err "Config file not found at $SOURCE_TFVARS"
  exit 1
fi

# Clone the repository exactly as deploy.sh does in container mode
SCM_AUTH_METHOD=$(grep '^\s*scm_auth_method\s*=' "$SOURCE_TFVARS" | cut -d'"' -f2 || true)
SCM_REPO_URL=$(grep '^\s*scm_repo_url\s*=' "$SOURCE_TFVARS" | cut -d'"' -f2 || true)
SCM_REPO_BRANCH=$(grep '^\s*scm_repo_branch\s*=' "$SOURCE_TFVARS" | cut -d'"' -f2 || true)
SCM_USERNAME=$(grep '^\s*scm_username\s*=' "$SOURCE_TFVARS" | cut -d'"' -f2 || true)
SCM_USERNAME=$(echo -n "$SCM_USERNAME" | jq -sRr @uri)
SCM_PASSWORD=$(grep '^\s*scm_password\s*=' "$SOURCE_TFVARS" | cut -d'"' -f2 || true)
SCM_PASSWORD=$(echo -n "$SCM_PASSWORD" | jq -sRr @uri)

if [[ -z "$SCM_REPO_URL" || -z "$SCM_REPO_BRANCH" ]]; then
  log_err "Failed to read scm_repo_url or scm_repo_branch from terraform.tfvars."
  exit 1
fi

log_info "Cloning from ${SCM_REPO_URL} (Branch: ${SCM_REPO_BRANCH})..."
rm -rf "$CLONE_DIR"

if [[ "$SCM_AUTH_METHOD" == "userpass" ]]; then
  SCM_HOST=$(echo "$SCM_REPO_URL" | sed -e 's|^https://||' -e 's|/.*||')
  SCM_PATH=$(echo "$SCM_REPO_URL" | sed -e 's|^https://[^/]*/||')
  AUTH_URL="https://${SCM_USERNAME}:${SCM_PASSWORD}@${SCM_HOST}/${SCM_PATH}"
  git clone -q -b "$SCM_REPO_BRANCH" "$AUTH_URL" "$CLONE_DIR"
elif [[ "$SCM_AUTH_METHOD" == "none" ]]; then
  git clone -q -b "$SCM_REPO_BRANCH" "$SCM_REPO_URL" "$CLONE_DIR"
else
  log_err "scm_auth_method '${SCM_AUTH_METHOD}' is not supported by the Cloud Build runner (no interactive prompt). Use 'userpass' or 'none', or deploy with container-deploy.sh."
  exit 1
fi

TF_DIR="${CLONE_DIR}/terraform/env"
TFVARS_FILE="${TF_DIR}/terraform.tfvars"
mkdir -p "$TF_DIR"
cp "$SOURCE_TFVARS" "$TFVARS_FILE"

cd "$TF_DIR"

if grep -q "<REQUIRED>" "$TFVARS_FILE"; then
  log_err "You still have '<REQUIRED>' placeholders in terraform.tfvars."
  exit 1
fi

BACKEND_BUCKET=$(awk -F'"' '/sdv_gcp_backend_bucket/ {print $2}' "$TFVARS_FILE")
PROJECT_ID=$(awk -F'"' '/sdv_gcp_project_id/ {print $2}' "$TFVARS_FILE")
LOCATION=$(awk -F'"' '/sdv_gcp_region/ {print $2}' "$TFVARS_FILE")
KMS_ENABLED=$(awk '/sdv_enable_kms_encryption/ {print $3}' "$TFVARS_FILE" | tr -d ' ')

if [[ -z "$BACKEND_BUCKET" || -z "$PROJECT_ID" || -z "$LOCATION" ]]; then
  log_err "Failed to decode one or more required variables (Bucket, Project, or Region). Check terraform.tfvars."
  exit 1
fi

# Handle KMS infrastructure if encryption is enabled
KMS_DIR="$(dirname "$TF_DIR")/kms"
if [[ "$KMS_ENABLED" == "true" ]] && [[ -d "$KMS_DIR" ]] && [[ "$MODE" == "apply" ]] && [[ ! "${SKIP_KMS:-}" == "true" ]]; then
  log_info "KMS encryption enabled - checking KMS infrastructure..."
  if ! gcloud kms keyrings describe "gke-secrets-keyring" --location="$LOCATION" --project="$PROJECT_ID" &>/dev/null; then
    log_warn "KMS infrastructure not found in GCP. Deploying persistent KMS resources..."
    cd "$KMS_DIR"
    terraform init -upgrade
    terraform apply -auto-approve -var-file="$TFVARS_FILE"
    log_info "KMS infrastructure deployed successfully"
    cd "$TF_DIR"
  else
    log_info "KMS infrastructure already exists in GCP"
  fi
elif [[ "$KMS_ENABLED" == "false" ]]; then
  log_info "KMS encryption disabled - skipping KMS infrastructure check"
fi

log_info "Initializing Terraform (Bucket: $BACKEND_BUCKET, Project: $PROJECT_ID)..."
terraform init -upgrade -reconfigure -backend-config="bucket=$BACKEND_BUCKET"

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
