#!/bin/bash

# Copyright (c) 2024-2025 Accenture, All Rights Reserved.
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
# This script downloads the Android Open Source Project (AOSP) source code
# for a specified branch and then builds a chosen device target.

OUTPUT_DIR="${HOME}/aosp"
REPO_URL="https://android.googlesource.com/platform/manifest"
BRANCH="main"
BUILD_TARGET="aosp_cf_x86_64_phone-trunk_staging-userdebug"
THREAD_COUNT=$[$(nproc)*49/100]

if [[ "${EUID:-$(id -u)}" -eq 0 ]]; then
    exec runuser user "${BASH_SOURCE[0]}" "$@"
fi

#######################################
# Simple logging to stderr.
# Arguments:
#   Message to log
#######################################
function _info() {
  echo "[$(date +'%Y-%m-%dT%H:%M:%S%z')] [$(basename "$0")] [INFO]:  $*" >&2
}

#######################################
# Sync and build the given Android repository
# Arguments:
#   output_directory
#     A directory to perform repo sync and build in.
#   repo_url
#     Url of the repository to sync / build.
#   build_target
#     Android target to build
#   repo_threads
#     The number of threads to use when calling repo sync.
#   branch
#     Branch
#######################################
function _sync_and_build_repo() {
  local output_directory="${1}"
  local repo_url="${2}"
  local build_target="${3}"
  local repo_threads="${4}"
  local branch="${5}"

  mkdir -p "${output_directory}"
  pushd ${output_directory} > /dev/null 2>&1

  sudo chown user:user /usr/bin/repo

  if [[ -z $(git config get user.email) ]]; then
    _info "no git user.email configured, setting default"
    git config --global user.email "aosp-builder"
  fi

  if [[ -z $(git config get user.name) ]]; then
    _info "no git user.name configured, setting default"
    git config --global user.name "AOSP Builder"
  fi

  _info "initializing repo url: ${repo_url}"
  echo yes | repo init --partial-clone -b ${branch} -u ${repo_url}

  # TODO: Validate this no longer flakes when using < 50% of available CPUs.
  _info "synchronizing repo using ${repo_threads} threads"
  repo sync -j"${repo_threads}"

  source build/envsetup.sh
  _info "setting build target ${build_target}"
  lunch ${build_target}

  _info "building."
  m
  popd > /dev/null 2>&1
}

#######################################
# Prints script usage to stderr.
#######################################
function _print_usage() {
  (
    echo "usage: $(basename $0) [OPTIONS]"
    echo "  options:"
    echo "    -o --output_dir    Directory to clone / build the source. Defaults to "
    echo "                       \$HOME/aosp."
    echo "    -u --repo_url      Repo url to clone. Defaults to "
    echo "                       https://android.googlesource.com/platform/manifest"
    echo "    -b --branch        The branch to clone. Defaults to "
    echo "                       main"
    echo "    -t --build_target  Target to build. Defaults to "
    echo "                       aosp_cf_x86_64_phone-trunk_staging-userdebug"
    echo "    -c --thread_count  Number of threads specify when syncing the repository."
    echo "                       Defaults to ~50% of available processors."
    echo "    -h --help          Print usage."
  ) 1>&2
}

#######################################
# Prints cli error message then exits with exit code 1.
#######################################
function _cli_errors() {
  echo "Unrecognized argument recieved." 1>&2
  printUsage
  exit 1
}

function main() {
  while getopts 'o:u:b:t:c:h-:' arg; do
    case "$arg" in
      -)
        case "$OPTARG" in
          output_dir) OUTPUT_DIR="${!OPTIND}"; OPTIND=$(( $OPTIND + 1 ));;
          output_dir=*) OUTPUT_DIR="${OPTARG#*=}";;
          repo_url) REPO_URL="${!OPTIND}"; OPTIND=$(( $OPTIND + 1 ));;
          repo_url=*) REPO_URL="${OPTARG#*=}";;
          branch) BRANCH="${!OPTIND}"; OPTIND=$(( $OPTIND + 1 ));;
          branch=*) BRANCH="${OPTARG#*=}";;
          build_target) BUILD_TARGET="${!OPTIND}"; OPTIND=$(( $OPTIND + 1 ));;
          build_target=*) BUILD_TARGET="${OPTARG#*=}";;
          thread_count) THREAD_COUNT="${!OPTIND}"; OPTIND=$(( $OPTIND + 1 ));;
          thread_count=*) THREAD_COUNT="${OPTARG#*=}";;
          help) _print_usage; exit 0;;
          *)
            if [ "$OPTERR" = 1 ] && [ "${OPTSPEC:0:1}" != ":" ]; then
              cli_errors; exit 1;
            fi
            ;;
        esac;;
      o) OUTPUT_DIR="$OPTARG";;
      u) REPO_URL="$OPTARG";;
      b) BRANCH="$OPTARG";;
      t) BUILD_TARGET="$OPTARG";;
      c) THREAD_COUNT="$OPTARG";;
      h) _print_usage; exit 0;;
      *) cli_errors; exit 1;
    esac
  done

  _sync_and_build_repo "${OUTPUT_DIR}" "${REPO_URL}" "${BUILD_TARGET}" "${THREAD_COUNT}" "${BRANCH}"
}

main "$@"
