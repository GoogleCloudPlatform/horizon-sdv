#!/usr/bin/env bash

# Copyright (c) 2026 Accenture and Google LLC, All Rights Reserved.
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
# Google Cloud Build deployment runner for Horizon SDV (opt-in variant of
# tools/scripts/deployment/deploy.sh).
# Runs Terraform Plan / Apply / Destroy on Google Cloud Build from
# scm_repo_url at scm_repo_branch, with zero local workstation dependencies.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || (cd "${SCRIPT_DIR}/../../../../../.." && pwd))"
LOCAL_TFVARS="${REPO_ROOT}/terraform/env/terraform.tfvars"
CLOUDBUILD_CONFIG="${SCRIPT_DIR}/cloudbuild.yaml"
# Relative to REPO_ROOT (gcloud resolves --ignore-file against the source dir)
IGNORE_FILE=".agents/plugins/horizon-ops/skills/horizon-deploy/scripts/gcloudignore"

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO] $1${NC}"; }
log_warn() { echo -e "${YELLOW}[WARN] $1${NC}"; }
log_err() { echo -e "${RED}[ERROR] $1${NC}"; }

usage() {
  echo ""
  echo "Horizon SDV Cloud-Native Deployer (Google Cloud Build)"
  echo ""
  echo "Usage: ./.agents/plugins/horizon-ops/skills/horizon-deploy/scripts/cloud-deploy.sh [OPTION] [-y|--yes]"
  echo ""
  echo "Options:"
  echo "  -p, --plan          Run Terraform Plan on Google Cloud Build"
  echo "  -a, --apply         Run Terraform Apply on Google Cloud Build"
  echo "  -d, --destroy       Run Terraform Destroy on Google Cloud Build"
  echo "  --status            View status of recent Cloud Builds"
  echo "  -y, --yes           Do not ask for confirmation when the local tree"
  echo "                      differs from origin/<scm_repo_branch>"
  echo "  -h, --help          Show this help message"
  echo ""
  echo "Environment:"
  echo "  SKIP_KMS=true       Skip the KMS bootstrap (same as deploy.sh)"
  echo ""
  exit "${1:-0}"
}

ACTION=""
ASSUME_YES="false"
for arg in "$@"; do
  case "$arg" in
    -p|--plan)    ACTION="-p" ;;
    -a|--apply)   ACTION="-a" ;;
    -d|--destroy) ACTION="-d" ;;
    --status)     ACTION="status" ;;
    -y|--yes)     ASSUME_YES="true" ;;
    -h|--help)    usage 0 ;;
    *)
      log_err "Unknown option: $arg"
      usage 1
      ;;
  esac
done

if [[ -z "$ACTION" ]]; then
  usage 1
fi

if [[ ! -f "$LOCAL_TFVARS" ]]; then
  log_err "Configuration file not found at $LOCAL_TFVARS"
  exit 1
fi

read_tfvar() {
  grep -E "^\s*$1\s*=" "$LOCAL_TFVARS" | head -n 1 | awk -F '"' '{print $2}' || true
}

PROJECT_ID=$(read_tfvar sdv_gcp_project_id)
BACKEND_BUCKET=$(read_tfvar sdv_gcp_backend_bucket)
SCM_REPO_BRANCH=$(read_tfvar scm_repo_branch)

if [[ -z "$PROJECT_ID" ]]; then
  log_err "Could not read sdv_gcp_project_id from $LOCAL_TFVARS"
  exit 1
fi

if [[ "$ACTION" == "status" ]]; then
  gcloud builds list --project="$PROJECT_ID" --limit=5
  exit 0
fi

if [[ -z "$BACKEND_BUCKET" ]]; then
  log_err "Could not read sdv_gcp_backend_bucket from $LOCAL_TFVARS"
  exit 1
fi

if [[ -z "$SCM_REPO_BRANCH" ]]; then
  log_err "Could not read scm_repo_branch from $LOCAL_TFVARS"
  exit 1
fi

# Cloud Build deploys scm_repo_url at scm_repo_branch, not the local files
check_local_tree() {
  local reasons=()
  if [[ -n "$(git -C "$REPO_ROOT" status --porcelain 2>/dev/null)" ]]; then
    reasons+=("the local working tree has uncommitted changes")
  fi
  local remote_ref
  remote_ref=$(git -C "$REPO_ROOT" rev-parse --verify -q "origin/${SCM_REPO_BRANCH}" || true)
  if [[ -z "$remote_ref" ]]; then
    reasons+=("origin/${SCM_REPO_BRANCH} was not found locally (run git fetch)")
  elif [[ "$remote_ref" != "$(git -C "$REPO_ROOT" rev-parse HEAD)" ]]; then
    reasons+=("HEAD differs from origin/${SCM_REPO_BRANCH}")
  fi

  if [[ ${#reasons[@]} -eq 0 ]]; then
    return
  fi

  for reason in "${reasons[@]}"; do
    log_warn "$reason"
  done
  log_warn "Cloud Build deploys scm_repo_url at '${SCM_REPO_BRANCH}' from the remote. Local changes are not deployed."

  if [[ "$ASSUME_YES" == "true" ]]; then
    log_warn "Continuing (--yes)."
    return
  fi
  read -r -p "Continue anyway? [y/N] " answer
  if [[ ! "$answer" =~ ^[Yy]$ ]]; then
    log_err "Aborted."
    exit 1
  fi
}

run_cloud_build() {
  local action="$1"
  check_local_tree
  log_info "Submitting Cloud Build job ($action) for project: $PROJECT_ID..."

  gcloud builds submit "$REPO_ROOT" \
    --config="$CLOUDBUILD_CONFIG" \
    --ignore-file="$IGNORE_FILE" \
    --gcs-source-staging-dir="gs://${BACKEND_BUCKET}/cloudbuild-source" \
    --substitutions="_ACTION=${action},_SKIP_KMS=${SKIP_KMS:-false}" \
    --project="$PROJECT_ID"
}

run_cloud_build "$ACTION"
