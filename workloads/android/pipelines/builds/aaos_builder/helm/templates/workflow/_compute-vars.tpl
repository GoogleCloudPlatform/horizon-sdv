{{- /*
Copyright (c) 2026 Accenture, All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

        http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

Description:
Compute derived values for aaos-builder templates.
Order: 2/10.
Dependencies: outputs sdkAndroidVersion for init/build/storage.
*/ -}}

{{- define "aaos-builder.template.compute-vars" -}}
- name: compute-vars
  script:
    image: alpine:3.20
    command: [sh]
    source: |
      set -euo pipefail
      revision='{{ "{{" }}workflow.parameters.androidRevision{{ "}}" }}'
      case "${revision}" in
        *android-14*) android_version="14" ;;
        *android-15*) android_version="15" ;;
        *android-16*) android_version="16" ;;
        *)
          echo "Failed to determine Android version from revision: ${revision}" >&2
          exit 1
          ;;
      esac
      printf "%s" "${android_version}" > /tmp/sdk_android_version
  outputs:
    parameters:
      - name: sdkAndroidVersion
        valueFrom:
          path: /tmp/sdk_android_version
{{- end }}
