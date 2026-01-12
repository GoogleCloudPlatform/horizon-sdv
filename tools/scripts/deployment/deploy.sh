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
# Main platform deployment script which can either be run natively on Linux
# or within the provided container environment.

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
log_err()  { echo -e "${RED}[ERROR] $1${NC}"; }

version_ge() {
    [ "$2" = "$(echo -e "$1\n$2" | sort -V | head -n1)" ]
}

# Workspace Setup 
setup_workspace() {
    if [[ -f "$CONTAINER_CONFIG" ]]; then
        # Clone repository if running within container
        log_info "Container environment detected. Setting up workspace..."

        # Extract Repo Details from terraform.tfvars
        REPO_OWNER=$(grep '^\s*sdv_github_repo_owner\s*=' "$CONTAINER_CONFIG" | cut -d'"' -f2)
        REPO_NAME=$(grep '^\s*sdv_github_repo_name\s*=' "$CONTAINER_CONFIG" | cut -d'"' -f2)
        REPO_BRANCH=$(grep '^\s*sdv_github_repo_branch\s*=' "$CONTAINER_CONFIG" | cut -d'"' -f2)
        GITHUB_PAT=$(grep '^\s*sdv_github_pat\s*=' "$CONTAINER_CONFIG" | cut -d'"' -f2)

        # Clone repository
        if [[ "$GITHUB_PAT" == "<OPTIONAL>" || "$GITHUB_PAT" == "<REQUIRED>" ]]; then GITHUB_PAT=""; fi
        
        log_info "Cloning ${REPO_OWNER}/${REPO_NAME} (Branch: ${REPO_BRANCH})..."
        if [[ -n "$GITHUB_PAT" ]]; then
            git clone -q -b "$REPO_BRANCH" "https://${GITHUB_PAT}@github.com/${REPO_OWNER}/${REPO_NAME}.git" .
        else
            if ! git clone -q -b "$REPO_BRANCH" "https://github.com/${REPO_OWNER}/${REPO_NAME}.git" . 2>/dev/null; then
                log_warn "Public clone failed. Repository might be private."
                read -s -p "Enter GitHub PAT: " MANUAL_PAT; echo ""
                if [[ -z "$MANUAL_PAT" ]]; then log_err "No token provided."; exit 1; fi
                git clone -q -b "$REPO_BRANCH" "https://${MANUAL_PAT}@github.com/${REPO_OWNER}/${REPO_NAME}.git" .
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
    MIN_KUBECTL_VER="1.34.3"
    MIN_DOCKER_VER="29.1.3"
    MIN_GCLOUD_VER="549.0.1"

    # Terraform
    if ! command -v terraform &> /dev/null; then log_err "Terraform not found"; exit 1; fi
    TF_CURRENT=$(terraform version -json | grep -o '"terraform_version":"[^"]*"' | cut -d'"' -f4)
    if [[ -z "$TF_CURRENT" ]]; then
        TF_CURRENT=$(terraform version | head -n1 | cut -d ' ' -f 2 | tr -d 'v')
    fi
    if ! version_ge "$TF_CURRENT" "$MIN_TF_VER"; then
        log_err "Terraform version $TF_CURRENT is too old. Required: >= $MIN_TF_VER"
        exit 1
    fi
    log_info "Terraform version $TF_CURRENT verified."

    # Kubectl
    if command -v kubectl &> /dev/null; then
        K_CURRENT=$(kubectl version --client --output=json 2>/dev/null | grep -o '"gitVersion": "[^"]*"' | head -n1 | cut -d'"' -f4 | tr -d 'v')
        if ! version_ge "$K_CURRENT" "$MIN_KUBECTL_VER"; then
            log_err "Kubectl version $K_CURRENT is too old. Required: >= $MIN_KUBECTL_VER"
            exit 1
        fi
        log_info "Kubectl version $K_CURRENT verified."
    else
        log_warn "Kubectl not found. Skipping version check (ensure it is installed if needed)."
    fi

    # Docker
    if [[ ! -f /.dockerenv ]]; then
        if command -v docker &> /dev/null; then
             D_CURRENT=$(docker version --format '{{.Client.Version}}' 2>/dev/null)
             if ! version_ge "$D_CURRENT" "$MIN_DOCKER_VER"; then
                 log_err "Docker version $D_CURRENT is too old. Required: >= $MIN_DOCKER_VER"
                 exit 1
             fi
             log_info "Docker version $D_CURRENT verified."
        fi
    fi

    # Gcloud Check
    if command -v gcloud &> /dev/null; then
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

    # Check if the ADC file exists
    if [[ ! -f "$ADC_FILE" ]]; then
        log_warn "Application Default Credentials (ADC) file not found."
        NEED_LOGIN=true
    else
        if ! gcloud auth application-default print-access-token &>/dev/null; then
             log_warn "ADC token is expired or invalid."
             NEED_LOGIN=true
        else
             NEED_LOGIN=false
        fi
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

    if [[ -z "$BACKEND_BUCKET" || -z "$PROJECT_ID" || -z "$LOCATION" ]]; then
        log_err "Failed to decode one or more required variables (Bucket, Project, or Region). Check terraform.tfvars."
        exit 1
    fi

    log_info "Initializing Terraform (Bucket: $BACKEND_BUCKET, Project: $PROJECT_ID)..."
    
    terraform init -upgrade -reconfigure \
        -backend-config="bucket=$BACKEND_BUCKET"

    # Check for Destroy Mode
    DESTROY_MODE=false
    for arg in "$@"; do
        if [[ "$arg" == "--destroy" || "$arg" == "-d" ]]; then DESTROY_MODE=true; fi
    done

    if [[ "$DESTROY_MODE" == "true" ]]; then
        log_warn "!!! DESTRUCTION MODE ENABLED !!!"
        
        # Check for Fleet Membership
        if gcloud container fleet memberships describe "$GKE_CLUSTER" --project="$PROJECT_ID" --location="$LOCATION" &>/dev/null; then
             log_info "Cleaning up Fleet Membership..."
             gcloud container fleet memberships get-credentials "$GKE_CLUSTER" --project="$PROJECT_ID"
             kubectl delete applications --all -n argocd --ignore-not-found
             kubectl delete appprojects --all -n argocd --ignore-not-found
             
             log_info "Waiting 5 minutes for Kubernetes resource removal..."
             sleep 5m
        fi

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