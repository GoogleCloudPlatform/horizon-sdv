#!/bin/bash

# Copyright 2025-2026 Google LLC
#
# Modifications Copyright (c) 2026 Accenture, All Rights Reserved.
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

# shellcheck source=/dev/null
source /google/scripts/common.sh

asfp_studio_location_fix() {
  # asfp-studio lives under assets/google/scripts (root .gitignore ignores
  # assets/**/usr/local/bin). Install it where .desktop Exec expects it.
  local src="/google/scripts/asfp-studio"
  if [[ ! -f "${src}" ]]; then
    error "ASfP launcher missing: ${src}"
  fi
  install -m 755 "${src}" /usr/local/bin/asfp-studio
  log "Installed /usr/local/bin/asfp-studio"
}

main() {
  asfp_studio_location_fix
}

if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
  main "$@"
fi
