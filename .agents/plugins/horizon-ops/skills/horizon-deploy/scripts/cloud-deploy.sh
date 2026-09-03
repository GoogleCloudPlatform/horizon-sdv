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
# Google Cloud Build deployment runner for Horizon SDV.
# Runs Terraform Plan / Apply / Destroy directly on Google Cloud Build
# with zero local workstation dependencies.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || (cd "${SCRIPT_DIR}/../../../../../.." && pwd))"
LOCAL_TFVARS="${REPO_ROOT}/terraform/env/terraform.tfvars"
CLOUDBUILD_CONFIG="${SCRIPT_DIR}/cloudbuild.yaml"

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO] $1${NC}"; }
log_warn() { echo -e "${YELLOW}[WARN] $1${NC}"; }
log_err() { echo -e "${RED}[ERROR] $1${NC}"; }

if [[ ! -f "$LOCAL_TFVARS" ]]; then
  log_err "Configuration file not found at $LOCAL_TFVARS"
  exit 1
fi

PROJECT_ID=$(grep -E '^\s*sdv_gcp_project_id\s*=' "$LOCAL_TFVARS" | head -n 1 | awk -F '"' '{print $2}')
BACKEND_BUCKET=$(grep -E '^\s*sdv_gcp_backend_bucket\s*=' "$LOCAL_TFVARS" | head -n 1 | awk -F '"' '{print $2}')

if [[ -z "$PROJECT_ID" ]]; then
  log_err "Could not read sdv_gcp_project_id from $LOCAL_TFVARS"
  exit 1
fi

if [[ -z "$BACKEND_BUCKET" ]]; then
  BACKEND_BUCKET="${PROJECT_ID}-horizon-tfstate"
fi

usage() {
  echo ""
  echo "Horizon SDV Cloud-Native Deployer (Google Cloud Build)"
  echo ""
  echo "Usage: ./.agents/plugins/horizon-ops/skills/horizon-deploy/scripts/cloud-deploy.sh [OPTION]"
  echo ""
  echo "Options:"
  echo "  -p, --plan          Run Terraform Plan on Google Cloud Build"
  echo "  -a, --apply         Run Terraform Apply on Google Cloud Build"
  echo "  -d, --destroy       Run Terraform Destroy on Google Cloud Build"
  echo "  --status            View status of recent Cloud Builds"
  echo "  -h, --help          Show this help message"
  echo ""
  exit 0
}

run_cloud_build() {
  local action="$1"
  log_info "Submitting Cloud Build job ($action) for project: $PROJECT_ID..."
  
  gcloud builds submit "$REPO_ROOT" \
    --config="$CLOUDBUILD_CONFIG" \
    --gcs-source-staging-dir="gs://${BACKEND_BUCKET}/cloudbuild-source" \
    --substitutions="_ACTION=$action" \
    --project="$PROJECT_ID"
}

if [[ $# -eq 0 ]]; then
  usage
fi

case "$1" in
  -p|--plan)
    run_cloud_build "-p"
    ;;
  -a|--apply)
    run_cloud_build "-a"
    ;;
  -d|--destroy)
    run_cloud_build "-d"
    ;;
  --status)
    gcloud builds list --project="$PROJECT_ID" --limit=5
    ;;
  -h|--help)
    usage
    ;;
  *)
    log_err "Unknown option: $1"
    usage
    ;;
esac
