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
# Build and push container images to Google Cloud Artifact Registry.
# Parses image definitions from Terraform configuration (locals.tf) and
# GCP configuration from terraform.tfvars. Supports building all images
# or a targeted subset, with optional cache control.
#
# This script is a development convenience tool for fast local iteration.
# It does NOT update Terraform state. For official/production builds, use
# Terraform (terraform apply), which tracks image state and triggers.

set -euo pipefail

# ---------------------------------------------------------------------------
# Constants & Paths
# ---------------------------------------------------------------------------
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"
IMAGES_DIR="${REPO_ROOT}/terraform/modules/sdv-container-images/images"
TFVARS_FILE="${REPO_ROOT}/terraform/env/terraform.tfvars"
LOCALS_FILE="${REPO_ROOT}/terraform/modules/base/locals.tf"
VERSIONS_FILE="${REPO_ROOT}/versions.yaml"
ENV_MAIN_FILE="${REPO_ROOT}/terraform/env/main.tf"

# ---------------------------------------------------------------------------
# Defaults
# ---------------------------------------------------------------------------
NO_CACHE=false
DRY_RUN=false
SKIP_PUSH=false
BUILD_ALL=false
QUIET=false
TARGET_IMAGES=()
GCP_PROJECT=""
GCP_REGION=""
REGISTRY_ID=""
LIST_MODE=false

# ---------------------------------------------------------------------------
# Output Formatting
# ---------------------------------------------------------------------------
# Respect NO_COLOR (https://no-color.org/) and non-interactive terminals.
# Unicode symbols are always used; only ANSI color codes are conditional.
if [[ -t 1 ]] && [[ -z "${NO_COLOR:-}" ]]; then
  GREEN='\033[0;32m'
  YELLOW='\033[1;33m'
  RED='\033[0;31m'
  CYAN='\033[0;36m'
  BOLD='\033[1m'
  DIM='\033[2m'
  NC='\033[0m'
else
  GREEN='' YELLOW='' RED='' CYAN='' BOLD='' DIM='' NC=''
fi

log_info()   { echo -e "  ${DIM}·${NC}  $1"; }
log_ok()     { echo -e "  ${GREEN}✓${NC}  $1"; }
log_warn()   { echo -e "  ${YELLOW}⚠  $1${NC}"; }
log_err()    { echo -e "  ${RED}✗  $1${NC}" >&2; }
log_step()   { echo -e "  ${CYAN}▶${NC}  ${BOLD}$1${NC}"; }

print_banner() {
  local rule
  rule="$(printf '━%.0s' {1..52})"
  echo ""
  echo -e "  ${CYAN}${rule}${NC}"
  echo -e "    ${BOLD}Horizon SDV${NC}  ${DIM}·${NC}  Container Image Builder"
  echo -e "  ${CYAN}${rule}${NC}"
}

format_duration() {
  local secs=$1
  if (( secs >= 3600 )); then
    printf '%dh %dm %ds' $((secs/3600)) $((secs%3600/60)) $((secs%60))
  elif (( secs >= 60 )); then
    printf '%dm %ds' $((secs/60)) $((secs%60))
  else
    printf '%ds' "$secs"
  fi
}

# ---------------------------------------------------------------------------
# Usage
# ---------------------------------------------------------------------------
usage() {
  local self
  self="$(basename "$0")"

  cat <<EOF
Usage: ${self} (-a | -i NAME) [OPTIONS]

Build and push container images to Google Cloud Artifact Registry.

Requires either -a (all images) or -i (single image) to start a build.
Running without either flag prints this help message.

EOF
  echo -e "${YELLOW}⚠  This is a development tool for fast local build-and-test cycles."
  echo -e "   It does NOT update Terraform state. For official or production"
  echo -e "   builds, use Terraform (terraform apply) which manages image state,"
  echo -e "   change detection, and registry push as part of the deployment pipeline.${NC}"
  cat <<EOF

GCP configuration (project, region) is auto-detected from
terraform/env/terraform.tfvars. The Artifact Registry repository ID is
auto-detected from terraform/env/main.tf. All values can be overridden
with flags.

TARGET (one required):
  -a, --all               Build all images in the catalog
  -i, --image NAME        Build the specified image(s); repeatable (e.g. -i app1 -i app2)

OPTIONS:
  -n, --no-cache          Build images without Docker cache
  -q, --quiet             Suppress Docker build and push output
      --dry-run           Show what would be built without executing
      --skip-push         Build only; do not push to registry
  -p, --project ID        GCP project ID (overrides terraform.tfvars)
  -r, --region REGION     GCP region (overrides terraform.tfvars)
      --registry ID       Artifact Registry repository ID (overrides main.tf)
  -l, --list              List all images defined in the catalog and exit
  -h, --help              Show this help message

EXAMPLES:
  ${self} -a                                  Build and push all images
  ${self} -i gerrit-post                  Build and push one image
  ${self} -i gerrit-post -i grafana-post  Build and push two images
  ${self} -a -n                               Build all with no cache
  ${self} -a -q                               Build all, suppress Docker output
  ${self} -a --dry-run                        Preview without executing
  ${self} -a --skip-push                      Build all, skip push
  ${self} -l                                  List images in the catalog
EOF
  exit 0
}

# ---------------------------------------------------------------------------
# Argument Parsing
# ---------------------------------------------------------------------------
parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      -i|--image)
        [[ -z "${2:-}" ]] && { log_err "Option $1 requires an argument."; exit 1; }
        TARGET_IMAGES+=("$2"); shift 2
        ;;
      -a|--all)        BUILD_ALL=true; shift ;;
      -n|--no-cache)   NO_CACHE=true; shift ;;
      -q|--quiet)      QUIET=true; shift ;;
      --dry-run)       DRY_RUN=true; shift ;;
      --skip-push)     SKIP_PUSH=true; shift ;;
      -p|--project)
        [[ -z "${2:-}" ]] && { log_err "Option $1 requires an argument."; exit 1; }
        GCP_PROJECT="$2"; shift 2
        ;;
      -r|--region)
        [[ -z "${2:-}" ]] && { log_err "Option $1 requires an argument."; exit 1; }
        GCP_REGION="$2"; shift 2
        ;;
      --registry)
        [[ -z "${2:-}" ]] && { log_err "Option $1 requires an argument."; exit 1; }
        REGISTRY_ID="$2"; shift 2
        ;;
      -l|--list)       LIST_MODE=true; shift ;;
      -h|--help)       usage ;;
      *)
        log_err "Unknown option: $1"
        echo "Run '$(basename "$0") --help' for usage." >&2
        exit 1
        ;;
    esac
  done
}

# ---------------------------------------------------------------------------
# Load GCP Configuration from terraform.tfvars
# ---------------------------------------------------------------------------
load_gcp_config() {
  if [[ ! -f "$TFVARS_FILE" ]]; then
    if [[ -z "$GCP_PROJECT" || -z "$GCP_REGION" ]]; then
      log_err "terraform.tfvars not found at: ${TFVARS_FILE}"
      log_err "Provide GCP configuration via flags: --project <ID> --region <REGION>"
      exit 1
    fi
    log_warn "terraform.tfvars not found; using values from command-line flags."
    return
  fi

  if [[ -z "$GCP_PROJECT" ]]; then
    GCP_PROJECT=$(awk -F'"' '/sdv_gcp_project_id[[:space:]]*=/ {print $2}' "$TFVARS_FILE")
  fi
  if [[ -z "$GCP_REGION" ]]; then
    GCP_REGION=$(awk -F'"' '/sdv_gcp_region[[:space:]]*=/ {print $2}' "$TFVARS_FILE")
  fi

  if [[ -z "$GCP_PROJECT" ]]; then
    log_err "Could not determine GCP project ID."
    log_err "Set 'sdv_gcp_project_id' in terraform.tfvars or pass --project <ID>."
    exit 1
  fi
  if [[ -z "$GCP_REGION" ]]; then
    log_err "Could not determine GCP region."
    log_err "Set 'sdv_gcp_region' in terraform.tfvars or pass --region <REGION>."
    exit 1
  fi
}

# ---------------------------------------------------------------------------
# Load Artifact Registry Repository ID
# ---------------------------------------------------------------------------
load_registry_id() {
  [[ -n "$REGISTRY_ID" ]] && return

  if [[ -f "$ENV_MAIN_FILE" ]]; then
    REGISTRY_ID=$(awk -F'"' '/sdv_artifact_registry_repository_id[[:space:]]*=/ {print $2}' "$ENV_MAIN_FILE")
  fi

  if [[ -z "$REGISTRY_ID" ]]; then
    log_err "Could not determine Artifact Registry repository ID."
    log_err "Set it in terraform/env/main.tf or pass --registry <ID>."
    exit 1
  fi
}

# ---------------------------------------------------------------------------
# Parse Image Catalog from locals.tf
#
# Populates globals:
#   IMAGE_NAMES       - ordered array of image names
#   IMAGE_DIRS[]      - associative: image name -> build directory
#   IMAGE_VERSIONS[]  - associative: image name -> version tag
#   IMAGE_BUILD_ARGS[] - associative: image name -> comma-separated KEY=VAL pairs
# ---------------------------------------------------------------------------
declare -A IMAGE_DIRS=()
declare -A IMAGE_VERSIONS=()
declare -A IMAGE_BUILD_ARGS=()
IMAGE_NAMES=()

# Horizon core version from versions.yaml (components.core). Terraform reads the same
# file (local.core_version in locals.tf), so locals.tf references to it are resolved here.
CORE_VERSION=""

read_core_version() {
  if [[ ! -f "$VERSIONS_FILE" ]]; then
    log_err "versions.yaml not found at: ${VERSIONS_FILE}"
    exit 1
  fi
  CORE_VERSION="$(sed -n 's/^[[:space:]]*core:[[:space:]]*"\{0,1\}\([^"#[:space:]]*\)"\{0,1\}.*/\1/p' "$VERSIONS_FILE" | head -n 1)"
  if [[ -z "$CORE_VERSION" ]]; then
    log_err "Could not read components.core from ${VERSIONS_FILE}."
    exit 1
  fi
}

parse_image_catalog() {
  if [[ ! -f "$LOCALS_FILE" ]]; then
    log_err "locals.tf not found at: ${LOCALS_FILE}"
    log_err "Cannot determine image definitions. Ensure the Terraform codebase is intact."
    exit 1
  fi

  read_core_version

  # Pass 1: extract simple local variable assignments for resolving build_arg references
  # (e.g.  common_nginx_version = "1.28.1-alpine3.23")
  declare -A local_vars
  while IFS= read -r line; do
    [[ "$line" =~ ^[[:space:]]*# ]] && continue
    if [[ "$line" =~ ^[[:space:]]+([a-z_][a-z0-9_]*)[[:space:]]*=[[:space:]]*\"([^\"]*)\" ]]; then
      local_vars["${BASH_REMATCH[1]}"]="${BASH_REMATCH[2]}"
    fi
  done < "$LOCALS_FILE"

  # Pass 2: parse the `images = { ... }` block using a brace-depth state machine
  local in_images=0 brace_depth=0 in_build_args=0
  local cur_image="" cur_dir="" cur_ver="" cur_ba=""

  while IFS= read -r line; do
    [[ "$line" =~ ^[[:space:]]*# ]] && continue

    # Detect start of the images block
    if [[ $in_images -eq 0 ]]; then
      if [[ "$line" =~ images[[:space:]]*=[[:space:]]*\{ ]]; then
        in_images=1
        brace_depth=1
      fi
      continue
    fi

    # Count braces on this line
    local opens="${line//[^\{]/}"
    local closes="${line//[^\}]/}"
    brace_depth=$(( brace_depth + ${#opens} - ${#closes} ))

    # End of the entire images block
    if [[ $brace_depth -le 0 ]]; then
      if [[ -n "$cur_image" ]]; then
        IMAGE_NAMES+=("$cur_image")
        IMAGE_DIRS["$cur_image"]="$cur_dir"
        IMAGE_VERSIONS["$cur_image"]="$cur_ver"
        IMAGE_BUILD_ARGS["$cur_image"]="$cur_ba"
      fi
      break
    fi

    # New image entry:  "image-name" = {
    if [[ $in_build_args -eq 0 && "$line" =~ \"([^\"]+)\"[[:space:]]*=[[:space:]]*\{ ]]; then
      # Save previous entry
      if [[ -n "$cur_image" ]]; then
        IMAGE_NAMES+=("$cur_image")
        IMAGE_DIRS["$cur_image"]="$cur_dir"
        IMAGE_VERSIONS["$cur_image"]="$cur_ver"
        IMAGE_BUILD_ARGS["$cur_image"]="$cur_ba"
      fi
      cur_image="${BASH_REMATCH[1]}"
      cur_dir=""
      cur_ver=""
      cur_ba=""
      continue
    fi

    # build_args block start
    if [[ $in_build_args -eq 0 && "$line" =~ build_args[[:space:]]*=[[:space:]]*\{ ]]; then
      in_build_args=1
      continue
    fi

    # Inside build_args block
    if [[ $in_build_args -eq 1 ]]; then
      if [[ "$line" =~ \} ]]; then
        in_build_args=0
        continue
      fi
      if [[ "$line" =~ ([A-Z_][A-Z0-9_]*)[[:space:]]*=[[:space:]]*(.*) ]]; then
        local ba_key="${BASH_REMATCH[1]}"
        local ba_val
        ba_val="$(echo "${BASH_REMATCH[2]}" | xargs)"
        # Resolve local.* references
        if [[ "$ba_val" =~ ^local\.(.+)$ ]]; then
          local ref="${BASH_REMATCH[1]}"
          ba_val="${local_vars[$ref]:-}"
          if [[ -z "$ba_val" ]]; then
            log_warn "Could not resolve local.${ref} for build_arg ${ba_key} in image '${cur_image}'"
          fi
        else
          ba_val="${ba_val//\"/}"
        fi
        [[ -n "$cur_ba" ]] && cur_ba+=","
        cur_ba+="${ba_key}=${ba_val}"
      fi
      continue
    fi

    # Parse directory
    if [[ "$line" =~ directory[[:space:]]*=[[:space:]]*\"([^\"]*)\" ]]; then
      cur_dir="${BASH_REMATCH[1]}"
    fi

    # Parse build_version
    if [[ "$line" =~ build_version[[:space:]]*=[[:space:]]*\"([^\"]*)\" ]]; then
      cur_ver="${BASH_REMATCH[1]}"
    elif [[ "$line" =~ build_version[[:space:]]*=[[:space:]]*local\.([a-z_][a-z0-9_]*) ]]; then
      # build_version = local.<name>: core_version comes from versions.yaml, others from simple locals
      local ver_ref="${BASH_REMATCH[1]}"
      if [[ "$ver_ref" == "core_version" ]]; then
        cur_ver="$CORE_VERSION"
      else
        cur_ver="${local_vars[$ver_ref]:-}"
      fi
      if [[ -z "$cur_ver" ]]; then
        log_warn "Could not resolve local.${ver_ref} for build_version of image '${cur_image}'"
      fi
    fi
  done < "$LOCALS_FILE"

  if [[ ${#IMAGE_NAMES[@]} -eq 0 ]]; then
    log_err "No images found in the 'images' block of ${LOCALS_FILE}."
    exit 1
  fi

  log_ok "Parsed ${#IMAGE_NAMES[@]} image(s) from catalog."
  echo ""
}

# ---------------------------------------------------------------------------
# List Images
# ---------------------------------------------------------------------------
list_images() {
  local hrule
  hrule="$(printf '─%.0s' {1..68})"

  echo ""
  echo -e "  ${CYAN}┌${NC} ${BOLD}Image Catalog${NC}  ${DIM}(source: terraform/modules/base/locals.tf)${NC}"
  echo -e "  ${CYAN}│${NC}"
  printf "  ${CYAN}│${NC}  ${BOLD}%-4s %-38s %-10s %s${NC}\n" "#" "IMAGE NAME" "VERSION" "DIRECTORY"
  echo -e "  ${CYAN}│${NC}  ${DIM}${hrule}${NC}"

  local idx=0
  for name in "${IMAGE_NAMES[@]}"; do
    idx=$(( idx + 1 ))
    printf "  ${CYAN}│${NC}  ${DIM}%-4s${NC} %-38s ${GREEN}%-10s${NC} ${DIM}%s${NC}\n" \
      "${idx}" "$name" "${IMAGE_VERSIONS[$name]}" "${IMAGE_DIRS[$name]}"
  done

  echo -e "  ${CYAN}│${NC}"
  echo -e "  ${CYAN}└${NC} ${BOLD}${#IMAGE_NAMES[@]}${NC} image(s) defined in catalog"
  echo ""
}

# ---------------------------------------------------------------------------
# Prerequisite Checks
# ---------------------------------------------------------------------------
check_prerequisites() {
  log_step "Checking prerequisites..."

  if ! command -v docker &>/dev/null; then
    log_err "Docker is not installed or not in PATH."
    exit 1
  fi

  if ! docker info &>/dev/null 2>&1; then
    log_err "Docker daemon is not running or current user lacks permission. Start Docker and retry."
    exit 1
  fi
  log_ok "Docker is available and running."

  if ! command -v gcloud &>/dev/null; then
    log_err "Google Cloud SDK (gcloud) is not installed or not in PATH."
    exit 1
  fi
  log_ok "Google Cloud SDK is available."

  if ! gcloud auth print-access-token &>/dev/null 2>&1; then
    log_err "Not authenticated with gcloud. Run 'gcloud auth login' first."
    exit 1
  fi
  log_ok "gcloud authentication verified."
}

# ---------------------------------------------------------------------------
# Configure Docker for Artifact Registry
# ---------------------------------------------------------------------------
configure_docker_auth() {
  local registry_host="${GCP_REGION}-docker.pkg.dev"
  log_step "Configuring Docker credential helper for ${registry_host}..."

  if ! gcloud auth configure-docker "${registry_host}" --quiet 2>/dev/null; then
    log_err "Failed to configure Docker authentication for ${registry_host}."
    log_err "Ensure gcloud is authenticated and has Artifact Registry permissions."
    exit 1
  fi
  log_ok "Docker credential helper configured for ${registry_host}."
}

# ---------------------------------------------------------------------------
# Build a Single Image
# ---------------------------------------------------------------------------
build_image() {
  local name="$1"
  local counter="$2"
  local directory="${IMAGE_DIRS[$name]}"
  local version="${IMAGE_VERSIONS[$name]}"
  local build_args_raw="${IMAGE_BUILD_ARGS[$name]:-}"
  local registry_host="${GCP_REGION}-docker.pkg.dev"

  # The portal embeds the horizon CLI; stamp it with the core version, as Terraform does
  # (terraform/modules/base/main.tf passes HORIZON_CLI_VERSION = local.core_version).
  if [[ "$name" == "horizon-dev-portal" ]]; then
    [[ -n "$build_args_raw" ]] && build_args_raw+=","
    build_args_raw+="HORIZON_CLI_VERSION=${CORE_VERSION}"
  fi
  local full_image="${registry_host}/${GCP_PROJECT}/${REGISTRY_ID}/${name}:${version}"
  local context_dir="${IMAGES_DIR}/${directory}/${name}"
  local dockerfile_path=""
  local platform_flag=""

  if [[ ! -d "$context_dir" ]]; then
    log_err "Build context directory not found: ${context_dir}"
    return 1
  fi

  if [[ -n "$dockerfile_path" ]]; then
    if [[ ! -f "$dockerfile_path" ]]; then
      log_err "Dockerfile not found: ${dockerfile_path}"
      return 1
    fi
  elif [[ ! -f "${context_dir}/Dockerfile" ]]; then
    log_err "Dockerfile not found in: ${context_dir}"
    return 1
  fi

  log_step "${counter} ${CYAN}Building${NC} ${BOLD}${name}:${version}${NC}"
  log_info "  Context:  ${DIM}${context_dir}${NC}"
  [[ -n "$dockerfile_path" ]] && log_info "  Dockerfile: ${DIM}${dockerfile_path}${NC}"
  log_info "  Tag:      ${full_image}"

  local -a cmd=(docker build)
  local cache_label="enabled"

  if [[ "$NO_CACHE" == "true" ]]; then
    cmd+=(--no-cache)
    cache_label="disabled"
  fi

  if [[ "$QUIET" == "true" ]]; then
    cmd+=(--quiet)
  fi

  if [[ -n "$platform_flag" ]]; then
    cmd+=(--platform "$platform_flag")
  fi

  local details="${DIM}Cache: ${cache_label}${NC}"
  [[ -n "$platform_flag" ]] && details+="${DIM} | platform=${platform_flag}${NC}"

  if [[ -n "$build_args_raw" ]]; then
    IFS=',' read -ra ba_pairs <<< "$build_args_raw"
    for pair in "${ba_pairs[@]}"; do
      cmd+=(--build-arg "$pair")
      details+="${DIM} | ${pair}${NC}"
    done
  fi

  log_info "  ${details}"
  if [[ -n "$dockerfile_path" ]]; then
    cmd+=(-f "$dockerfile_path" -t "$full_image" "$context_dir")
  else
    cmd+=(-t "$full_image" "$context_dir")
  fi

  if [[ "$DRY_RUN" == "true" ]]; then
    log_info "  ${DIM}[DRY RUN] ${cmd[*]}${NC}"
    return 0
  fi

  local build_start=$SECONDS
  if "${cmd[@]}"; then
    local elapsed=$(( SECONDS - build_start ))
    log_ok "Built ${GREEN}${name}:${version}${NC} ${DIM}in $(format_duration $elapsed)${NC}"
  else
    log_err "Build failed: ${name}:${version}"
    return 1
  fi
}

# ---------------------------------------------------------------------------
# Push a Single Image
# ---------------------------------------------------------------------------
push_image() {
  local name="$1"
  local version="${IMAGE_VERSIONS[$name]}"
  local registry_host="${GCP_REGION}-docker.pkg.dev"
  local full_image="${registry_host}/${GCP_PROJECT}/${REGISTRY_ID}/${name}:${version}"

  if [[ "$DRY_RUN" == "true" ]]; then
    log_info "  ${DIM}[DRY RUN] docker push ${full_image}${NC}"
    return 0
  fi

  local -a push_cmd=(docker push)
  if [[ "$QUIET" == "true" ]]; then
    push_cmd+=(--quiet)
  fi
  push_cmd+=("$full_image")

  local push_start=$SECONDS
  if "${push_cmd[@]}"; then
    local elapsed=$(( SECONDS - push_start ))
    log_ok "Pushed ${DIM}in $(format_duration $elapsed)${NC}"
  else
    log_err "Push failed: ${full_image}"
    return 1
  fi
}

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------
main() {
  parse_args "$@"

  print_banner

  load_gcp_config
  load_registry_id
  parse_image_catalog

  # --list: print catalog and exit
  if [[ "$LIST_MODE" == "true" ]]; then
    list_images
    exit 0
  fi

  # Require explicit build target: -a (all) or -i (one or more images)
  if [[ "$BUILD_ALL" != "true" && ${#TARGET_IMAGES[@]} -eq 0 ]]; then
    usage
  fi

  # Mutual exclusion: -a and -i cannot be combined
  if [[ "$BUILD_ALL" == "true" && ${#TARGET_IMAGES[@]} -gt 0 ]]; then
    log_err "Flags -a/--all and -i/--image are mutually exclusive."
    log_err "Use -a to build everything, or -i to pick specific images."
    exit 1
  fi

  # Validate each --image target exists in catalog
  if [[ ${#TARGET_IMAGES[@]} -gt 0 ]]; then
    local -a invalid_images=()
    for target in "${TARGET_IMAGES[@]}"; do
      local found=false
      for name in "${IMAGE_NAMES[@]}"; do
        [[ "$name" == "$target" ]] && { found=true; break; }
      done
      [[ "$found" != "true" ]] && invalid_images+=("$target")
    done
    if [[ ${#invalid_images[@]} -gt 0 ]]; then
      for bad in "${invalid_images[@]}"; do
        log_err "Image '${bad}' not found in catalog."
      done
      echo "" >&2
      echo "  Available images:" >&2
      for name in "${IMAGE_NAMES[@]}"; do
        echo -e "       ${DIM}-${NC} ${name}" >&2
      done
      exit 1
    fi
  fi

  # Determine build list
  local -a build_list
  if [[ ${#TARGET_IMAGES[@]} -gt 0 ]]; then
    build_list=("${TARGET_IMAGES[@]}")
  else
    build_list=("${IMAGE_NAMES[@]}")
  fi

  # Configuration summary (tree-style)
  local registry_host="${GCP_REGION}-docker.pkg.dev"
  local cache_label push_label
  cache_label="$( [[ "$NO_CACHE" == "true" ]] && echo "disabled" || echo "enabled" )"
  push_label="$( [[ "$SKIP_PUSH" == "true" ]] && echo "skip" || echo "enabled" )"
  local tree_rule
  tree_rule="$(printf '─%.0s' {1..52})"

  echo ""
  echo -e "  ${CYAN}┌${NC} ${BOLD}Build Configuration${NC}"
  echo -e "  ${CYAN}│${NC}"
  echo -e "  ${CYAN}│${NC}  GCP Project     ${BOLD}${GCP_PROJECT}${NC}"
  echo -e "  ${CYAN}│${NC}  GCP Region      ${GCP_REGION}"
  echo -e "  ${CYAN}│${NC}  Registry        ${DIM}${registry_host}/${GCP_PROJECT}/${REGISTRY_ID}${NC}"
  echo -e "  ${CYAN}│${NC}  Images          ${BOLD}${#build_list[@]}${NC} of ${#IMAGE_NAMES[@]}"
  local quiet_label
  quiet_label="$( [[ "$QUIET" == "true" ]] && echo "on" || echo "off" )"

  echo -e "  ${CYAN}│${NC}  Cache           ${cache_label}"
  echo -e "  ${CYAN}│${NC}  Push            ${push_label}"
  echo -e "  ${CYAN}│${NC}  Quiet           ${quiet_label}"
  if [[ "$DRY_RUN" == "true" ]]; then
    echo -e "  ${CYAN}│${NC}"
    echo -e "  ${CYAN}│${NC}  ${YELLOW}⚠  DRY RUN - no changes will be made${NC}"
  fi
  echo -e "  ${CYAN}│${NC}"
  echo -e "  ${CYAN}└${tree_rule}${NC}"
  echo ""

  # Pre-flight checks (skip for dry-run to allow offline previews)
  if [[ "$DRY_RUN" != "true" ]]; then
    check_prerequisites
    configure_docker_auth
    echo ""
  fi

  # Build and push loop
  local total=${#build_list[@]}
  local build_ok=0 build_fail=0 push_ok=0 push_fail=0 push_skip=0
  local -a failed_images=()
  local run_start=$SECONDS
  local idx=0

  for name in "${build_list[@]}"; do
    idx=$(( idx + 1 ))
    local counter="${DIM}[${idx}/${total}]${NC}"
    echo ""

    if build_image "$name" "$counter"; then
      build_ok=$(( build_ok + 1 ))

      if [[ "$SKIP_PUSH" == "true" || "$DRY_RUN" == "true" ]]; then
        push_skip=$(( push_skip + 1 ))
      else
        if push_image "$name"; then
          push_ok=$(( push_ok + 1 ))
        else
          push_fail=$(( push_fail + 1 ))
          failed_images+=("$name")
        fi
      fi
    else
      build_fail=$(( build_fail + 1 ))
      failed_images+=("$name")
    fi
  done

  local total_elapsed=$(( SECONDS - run_start ))

  # Summary (tree-style)
  local build_sym push_sym
  if [[ $build_fail -eq 0 ]]; then
    build_sym="${GREEN}✓${NC}"
  else
    build_sym="${RED}✗${NC}"
  fi

  echo ""
  echo -e "  ${CYAN}┌${NC} ${BOLD}Summary${NC}"
  echo -e "  ${CYAN}│${NC}"
  echo -e "  ${CYAN}│${NC}  ${build_sym}  Build    ${GREEN}${build_ok} succeeded${NC}, ${RED}${build_fail} failed${NC}"

  if [[ "$SKIP_PUSH" != "true" && "$DRY_RUN" != "true" ]]; then
    if [[ $push_fail -eq 0 ]]; then
      push_sym="${GREEN}✓${NC}"
    else
      push_sym="${RED}✗${NC}"
    fi
    echo -e "  ${CYAN}│${NC}  ${push_sym}  Push     ${GREEN}${push_ok} succeeded${NC}, ${RED}${push_fail} failed${NC}"
  elif [[ "$DRY_RUN" == "true" ]]; then
    echo -e "  ${CYAN}│${NC}  ${DIM}·${NC}  Push     ${DIM}skipped (dry run)${NC}"
  else
    echo -e "  ${CYAN}│${NC}  ${DIM}·${NC}  Push     ${DIM}skipped (--skip-push)${NC}"
  fi

  echo -e "  ${CYAN}│${NC}  ⏱  Elapsed  $(format_duration $total_elapsed)"

  if [[ ${#failed_images[@]} -gt 0 ]]; then
    echo -e "  ${CYAN}│${NC}"
    echo -e "  ${CYAN}│${NC}  ${RED}Failed:${NC}"
    for fname in "${failed_images[@]}"; do
      echo -e "  ${CYAN}│${NC}    ${RED}✗${NC}  ${fname}"
    done
  fi

  echo -e "  ${CYAN}│${NC}"
  echo -e "  ${CYAN}└${tree_rule}${NC}"
  echo ""

  if [[ $build_fail -gt 0 || $push_fail -gt 0 ]]; then
    exit 1
  fi

  log_ok "All operations completed successfully."
}

main "$@"
