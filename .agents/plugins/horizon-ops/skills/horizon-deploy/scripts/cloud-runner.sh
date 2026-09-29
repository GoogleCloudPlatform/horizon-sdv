#!/usr/bin/env bash

# Copyright (c) 2026 Accenture, All Rights Reserved.
#
# Cloud Build Container Runner for Horizon SDV Terraform Deployment.

set -euo pipefail

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO] $1${NC}"; }
log_warn() { echo -e "${YELLOW}[WARN] $1${NC}"; }
log_err() { echo -e "${RED}[ERROR] $1${NC}"; }

TF_DIR="/workspace/terraform/env"
TFVARS_FILE="${TF_DIR}/terraform.tfvars"

if [[ ! -f "$TFVARS_FILE" ]]; then
  log_err "Config file not found at $TFVARS_FILE"
  exit 1
fi

if grep -q "<REQUIRED>" "$TFVARS_FILE"; then
  log_err "You still have '<REQUIRED>' placeholders in terraform.tfvars."
  exit 1
fi

PROJECT_ID=$(awk -F'"' '/sdv_gcp_project_id/ {print $2}' "$TFVARS_FILE")
BACKEND_BUCKET=$(awk -F'"' '/sdv_gcp_backend_bucket/ {print $2}' "$TFVARS_FILE")
LOCATION=$(awk -F'"' '/sdv_gcp_region/ {print $2}' "$TFVARS_FILE")
KMS_ENABLED=$(awk '/sdv_enable_kms_encryption/ {print $3}' "$TFVARS_FILE" | tr -d ' ')

if [[ -z "$BACKEND_BUCKET" || -z "$PROJECT_ID" || -z "$LOCATION" ]]; then
  log_err "Failed to decode required variables from terraform.tfvars."
  exit 1
fi

MODE="plan"
case "${1:- -p}" in
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

# Handle KMS infrastructure if encryption is enabled
KMS_DIR="/workspace/terraform/kms"
if [[ "$KMS_ENABLED" == "true" ]] && [[ -d "$KMS_DIR" ]] && [[ "$MODE" == "apply" ]]; then
  log_info "KMS encryption enabled - checking KMS infrastructure..."
  if ! gcloud kms keyrings describe "gke-secrets-keyring" --location="$LOCATION" --project="$PROJECT_ID" &>/dev/null; then
    log_info "Deploying KMS infrastructure..."
    cd "$KMS_DIR"
    terraform init -upgrade
    terraform apply -auto-approve -var-file="$TFVARS_FILE"
    cd "$TF_DIR"
  fi
fi

log_info "Initializing Terraform (Bucket: $BACKEND_BUCKET, Project: $PROJECT_ID)..."
terraform init -upgrade -reconfigure -backend-config="bucket=$BACKEND_BUCKET"

if [[ "$MODE" == "plan" ]]; then
  log_info "Running Terraform Plan..."
  terraform plan
elif [[ "$MODE" == "destroy" ]]; then
  log_warn "Running Terraform Destroy..."
  terraform destroy -auto-approve
else
  log_info "Running Terraform Apply..."
  terraform apply -auto-approve
fi
