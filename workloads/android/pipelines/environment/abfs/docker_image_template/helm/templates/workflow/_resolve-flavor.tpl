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
Map templateFlavor to the Argo default Ubuntu/COS registry tag and reject contradictory
canonical tags. Jenkins uses latest (Ubuntu) and cos-latest (COS). Argo uses
argowf-latest (Ubuntu) and argowf-cos-latest (COS). Portal always sends both fields
(WT defaults); ubuntu default tags with flavor=cos are rewritten to argowf-cos-latest.
*/ -}}

{{- define "abfs-builder-runtime-image.template.resolve-flavor" -}}
- name: resolve-flavor
  inputs:
    parameters:
      - name: templateFlavor
      - name: imageTag
  script:
    image: "gcr.io/google.com/cloudsdktool/cloud-sdk:slim"
    command: [bash, -euo, pipefail, -c]
    env:
      - name: TEMPLATE_FLAVOR
        value: '{{ "{{" }}inputs.parameters.templateFlavor{{ "}}" }}'
      - name: IMAGE_TAG
        value: '{{ "{{" }}inputs.parameters.imageTag{{ "}}" }}'
    source: |
      flavor="$(printf '%s' "${TEMPLATE_FLAVOR}" | tr '[:upper:]' '[:lower:]' | tr -d '[:space:]')"
      tag="$(printf '%s' "${IMAGE_TAG}" | tr -d '[:space:]')"

      is_ubuntu_canonical() {
        [ "$1" = "latest" ] || [ "$1" = "argowf-latest" ]
      }
      is_cos_canonical() {
        [ "$1" = "cos-latest" ] || [ "$1" = "argowf-cos-latest" ] \
          || printf '%s' "$1" | grep -q '^cos-' \
          || printf '%s' "$1" | grep -q '^argowf-cos-'
      }

      case "${flavor}" in
        ubuntu)
          if is_cos_canonical "${tag}"; then
            echo "ERROR: templateFlavor=ubuntu cannot be combined with COS image tag '${tag}'." >&2
            echo "Use imageTag=argowf-latest (Ubuntu) or set templateFlavor=cos." >&2
            exit 1
          fi
          if [ -z "${tag}" ]; then
            tag="argowf-latest"
            echo "templateFlavor=ubuntu: default imageTag=${tag}" >&2
          fi
          ;;
        cos)
          if is_ubuntu_canonical "${tag}" || [ -z "${tag}" ]; then
            echo "templateFlavor=cos: selecting default COS imageTag=argowf-cos-latest (input tag='${tag:-empty}')" >&2
            tag="argowf-cos-latest"
          fi
          ;;
        *)
          echo "ERROR: templateFlavor must be ubuntu or cos (got '${TEMPLATE_FLAVOR}')." >&2
          exit 1
          ;;
      esac
      printf '%s\n' "${tag}" > /tmp/image_tag
      echo "Resolved imageTag=${tag} for templateFlavor=${flavor}" >&2
  outputs:
    parameters:
      - name: imageTag
        valueFrom:
          path: /tmp/image_tag
{{- end }}
