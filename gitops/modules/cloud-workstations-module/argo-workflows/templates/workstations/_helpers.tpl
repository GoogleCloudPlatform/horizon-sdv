{{/*
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
*/}}

{{/*
Labels for the exposed workstation WorkflowTemplates. Call with (dict "root" $ "family" "admin|user").
horizon-sdv.io/family groups the two operation families for readers; Horizon API and the Portal do
not authorize on it, so any authenticated caller can submit either family.
*/}}
{{- define "cws-ws.templateLabels" -}}
{{ include "cws.labels" .root }}
horizon-sdv.io/expose: "true"
horizon-sdv.io/family: {{ .family }}
{{- end -}}

{{/*
One workflow at a time for a given workstation. Create, delete, user changes, start, and stop share
this lock. Argo trims workstationId when the workflow is submitted, so " ws1" and "ws1" take the
same lock. Different workstations do not block each other. Read-only templates do not take the lock.
*/}}
{{- define "cws-ws.workstationMutex" -}}
synchronization:
  mutexes:
    - name: {{ printf "cws-workstation-%s" "{{=sprig.trim(workflow.parameters.workstationId)}}" | quote }}
{{- end -}}

{{/*
Workflow parameters per template. The WorkflowTemplates and the Sensors both render from this map,
so a Sensor's index mapping always follows its template's parameter order.
An empty default makes the key required on Horizon submit (internal/api/server.go validateParams)
and makes the Portal draw a required asterisk. Delete asks for confirmDelete, a text box pre-filled
with false that must be changed to true. List filters default to .* so they stay optional.
*/}}
{{- define "cws-ws.parameters" -}}
cws-admin-create-workstation:
  - name: horizonSubmittedFrom
    value: ""
  - name: workstationId
    value: ""
    description: "Required. Workstation ID: lowercase letters, digits and hyphens, starting with a letter, at most 63 characters."
  - name: configName
    value: ""
    description: "Required. Name of an existing WorkstationConfig. Cannot be changed after the workstation is created."
  - name: displayName
    value: ""
    description: "Optional. Human-readable name. Empty means no display name."
  - name: users
    value: ""
    description: "Optional. Comma-separated emails or IAM principals granted Workstation User. Bare emails become user:<email>. Empty means no users."
cws-admin-delete-workstation:
  - name: horizonSubmittedFrom
    value: ""
  - name: workstationId
    value: ""
    description: "Required. Workstation ID to fetch and delete."
  - name: confirmDelete
    value: "false"
    description: "Required. Change from false to true to confirm. Deletion cannot be undone."
cws-admin-get-workstation:
  - name: horizonSubmittedFrom
    value: ""
  - name: workstationId
    value: ""
    description: "Required. Workstation to show."
cws-admin-list-workstations:
  - name: horizonSubmittedFrom
    value: ""
  - name: nameRegex
    value: ".*"
    description: "Optional. Regular expression on the workstation ID. Default .* lists all. Example: ^dev-"
  - name: userRegex
    value: ".*"
    description: "Optional. Regular expression on each user, with or without the user: prefix. Default .* lists all."
  - name: configRegex
    value: ".*"
    description: "Optional. Regular expression on the config name. Default .* lists all."
cws-admin-add-workstation-users:
  - name: horizonSubmittedFrom
    value: ""
  - name: workstationId
    value: ""
    description: "Required. Workstation to add users to."
  - name: users
    value: ""
    description: "Required. Comma-separated emails or IAM principals to grant Workstation User. Bare emails become user:<email>."
cws-admin-remove-workstation-users:
  - name: horizonSubmittedFrom
    value: ""
  - name: workstationId
    value: ""
    description: "Required. Workstation to remove users from."
  - name: users
    value: ""
    description: "Required. Comma-separated emails or IAM principals to remove. Bare emails become user:<email>."
cws-user-list-usable-workstations:
  - name: horizonSubmittedFrom
    value: ""
  - name: submittedBy
    value: ""
    description: "Required. One email or IAM principal. Bare emails become user:<email>. Lists workstations that include it in spec.users. This is a filter, not access control."
cws-user-get-workstation:
  - name: horizonSubmittedFrom
    value: ""
  - name: workstationId
    value: ""
    description: "Required. Workstation to show."
  - name: submittedBy
    value: ""
    description: "Required. One email or IAM principal. Bare emails become user:<email>. Must be in spec.users. This is a filter, not access control."
cws-user-get-workstation-configuration:
  - name: horizonSubmittedFrom
    value: ""
  - name: workstationId
    value: ""
    description: "Required. Workstation whose configuration to show."
  - name: submittedBy
    value: ""
    description: "Required. One email or IAM principal. Bare emails become user:<email>. Must be in spec.users. This is a filter, not access control."
cws-user-start-workstation:
  - name: horizonSubmittedFrom
    value: ""
  - name: workstationId
    value: ""
    description: "Required. Workstation to start."
  - name: submittedBy
    value: ""
    description: "Required. One email or IAM principal. Bare emails become user:<email>. Must be in spec.users. This is a filter, not access control."
cws-user-stop-workstation:
  - name: horizonSubmittedFrom
    value: ""
  - name: workstationId
    value: ""
    description: "Required. Workstation to stop."
  - name: submittedBy
    value: ""
    description: "Required. One email or IAM principal. Bare emails become user:<email>. Must be in spec.users. This is a filter, not access control."
{{- end -}}

{{/* arguments.parameters for one template. Call with (dict "root" $ "name" "<template name>"). */}}
{{- define "cws-ws.arguments" -}}
{{- $all := include "cws-ws.parameters" .root | fromYaml -}}
{{- $params := index $all .name -}}
{{- if not $params }}{{ fail (printf "cws-ws.parameters has no entry for %s" .name) }}{{ end -}}
parameters:
{{ toYaml $params | indent 2 }}
{{- end -}}

{{/*
Workstation-specific bash functions, used on top of cws.shellLib (which supplies fail, resource_for,
cr_json, ready_state, progress_line, print_status, wait_ready and wait_gone). Requires CWS_NAMESPACE.
Keep double braces out of this block: Argo would read them as workflow parameters.
*/}}
{{- define "cws-ws.shlib" -}}
{{ include "cws.shellLib" . }}

WS_RES=$(resource_for workstation)
CFG_RES=$(resource_for workstationconfig)

trim() { echo "$1" | sed -e 's/^[[:space:]]*//' -e 's/[[:space:]]*$//'; }

# resource_name PARAM VALUE: prints the trimmed GCP resource ID, or fails.
resource_name() {
  local value
  value=$(trim "$2")
  [[ -n "$value" ]] || fail "$1 is required"
  [[ "$value" =~ ^[a-z]([-a-z0-9]{0,61}[a-z0-9])?$ ]] \
    || fail "$1 '$value' must start with a lowercase letter, contain only lowercase letters, digits and hyphens, end with a letter or digit, and be at most 63 characters"
  echo "$value"
}

# principals_json VALUE: comma-separated members to a JSON array of principals.
# user:, group: and serviceAccount: must be followed by an email (name@domain.tld).
# A bare email becomes user:<email>. The same address is kept once, ignoring case.
principals_json() {
  jq -Rsc '
    def email: "[^@[:space:]:]+@[^@[:space:].]+(\\.[^@[:space:].]+)+";
    split(",") | map(gsub("^\\s+|\\s+$"; "")) | map(select(length > 0))
    | map(
        if test("^(user:|group:|serviceAccount:)" + email + "$") then .
        elif test("^(domain:|principal://|principalSet://).+") then .
        elif test("^" + email + "$") then "user:" + .
        else error("\"" + . + "\" is not an email like name@domain.tld, or an IAM principal (user:, group:, serviceAccount:, domain:, principal://, principalSet://)")
        end)
    | unique_by(ascii_downcase)' <<<"$1"
}

# single_principal PARAM VALUE: prints exactly one normalised principal, or fails.
single_principal() {
  local list
  list=$(principals_json "$2") || fail "$1 is not a valid email or IAM principal"
  [[ "$(jq 'length' <<<"$list")" == 1 ]] || fail "$1 must be exactly one email or IAM principal"
  jq -r '.[0]' <<<"$list"
}

require_ws() {
  local json
  json=$(cr_json "$WS_RES" "$1") || fail "could not read workstation '$1'"
  [[ -n "$json" ]] || fail "workstation '$1' does not exist in namespace $CWS_NAMESPACE"
  echo "$json"
}

require_cfg() {
  local json
  json=$(cr_json "$CFG_RES" "$1") || fail "could not read workstation config '$1'"
  [[ -n "$json" ]] || fail "workstation config '$1' does not exist in namespace $CWS_NAMESPACE"
  echo "$json"
}

# require_member WORKSTATION_JSON PRINCIPAL
require_member() {
  # Same comparison as the controller: case-insensitive, stored spelling unchanged.
  jq -e --arg m "$2" 'def key: ascii_downcase; (.spec.users // []) | any(key == ($m | key))' >/dev/null <<<"$1" \
    || fail "$2 is not in spec.users of workstation '$(jq -r '.metadata.name' <<<"$1")'"
}

# The workstation fields that print_status does not cover; call print_status for phase and conditions.
print_workstation() {
  jq -r '
    "name: \(.metadata.name)",
    "config: \(.spec.configRef.name)",
    "displayName: \(.spec.displayName // "-")",
    "state: \(.status.state // "-")",
    "url: \((.status.host // "") | if . == "" then "-" elif test("^https?://") then . else "https://" + . end)",
    "users: \((.spec.users // []) | if length == 0 then "-" else join(", ") end)"' <<<"$1"
}

print_config() {
  jq -r '
    def ports: (. // []) | map(if .first == .last then "\(.first)" else "\(.first)-\(.last)" end)
      | if length == 0 then "-" else join(", ") end;
    "name: \(.metadata.name)",
    "displayName: \(.spec.displayName // "-")",
    "image: \(.spec.container.image // "-")",
    "machineType: \(.spec.host.gceInstance.machineType // "-")",
    "bootDiskSizeGB: \(.spec.host.gceInstance.bootDiskSizeGB // "-")",
    "poolSize: \(.spec.host.gceInstance.poolSize // 0)",
    "idleTimeout: \(.spec.idleTimeout // "-")",
    "runningTimeout: \(.spec.runningTimeout // "-")",
    "disableSsh: \(.spec.disableSsh | tostring) (live: \(if .status.posture then (.status.posture.disableSsh | tostring) else "not read back yet" end))",
    "allowedPorts: \(.spec.allowedPorts | ports) (live: \(if .status.posture then (.status.posture.allowedPorts | ports) else "not read back yet" end))",
    "admins: \((.spec.admins // []) | if length == 0 then "-" else join(", ") end)"' <<<"$1"
}

# update_users NAME add|remove PRINCIPALS_JSON
# Read, modify, write with a resourceVersion test so two concurrent runs cannot lose a change.
# Membership is case-insensitive, matching the controller. The stored spelling is kept.
# Sets USERS_CHANGED, and USERS_ADDED to the principals an add actually wrote.
update_users() {
  local name=$1 action=$2 wanted=$3 attempt=1 json rv current updated added='[]' patch rc
  USERS_CHANGED=false
  USERS_ADDED='[]'
  while :; do
    json=$(require_ws "$name")
    rv=$(jq -r '.metadata.resourceVersion' <<<"$json")
    current=$(jq -c '.spec.users // []' <<<"$json")
    if [[ "$action" == add ]]; then
      added=$(jq -nc --argjson c "$current" --argjson w "$wanted" '
        def key: ascii_downcase;
        reduce $w[] as $item ({seen: ($c | map(key)), add: []};
          if ((.seen | index($item | key)) != null) then .
          else .seen += [$item | key] | .add += [$item]
          end) | .add')
      updated=$(jq -nc --argjson c "$current" --argjson a "$added" '$c + $a')
      jq -nr --argjson c "$current" --argjson w "$wanted" '
        def key: ascii_downcase;
        [$c[] | select(. as $x | any($w[]; key == ($x | key)))][] | "already a user: \(.)"'
      jq -r '.[] | "adding: \(.)"' <<<"$added"
    else
      updated=$(jq -nc --argjson c "$current" --argjson w "$wanted" '
        def key: ascii_downcase;
        ($w | map(key)) as $drop
        | [$c[] | select(. as $x | ($drop | index($x | key)) == null)]')
      jq -nr --argjson c "$current" --argjson w "$wanted" '
        def key: ascii_downcase;
        [$w[] | select(. as $x | any($c[]; key == ($x | key)) | not)][] | "not a user: \(.)"'
      jq -nr --argjson c "$current" --argjson w "$wanted" '
        def key: ascii_downcase;
        [$c[] | select(. as $x | any($w[]; key == ($x | key)))][] | "removing: \(.)"'
    fi
    if [[ "$updated" == "$current" ]]; then
      echo "spec.users already matches the request; nothing to change"
      return 0
    fi
    patch=$(jq -nc --arg rv "$rv" --argjson u "$updated" '
      [{op: "test", path: "/metadata/resourceVersion", value: $rv},
       {op: "add", path: "/spec/users", value: $u}]')
    # A controller status write bumps resourceVersion between the read and the patch.
    # That fails the test operation and is retried. Any other error, including validation, is final.
    apply_users_patch() {
      local out
      if out=$(kubectl patch "$WS_RES" "$name" -n "$CWS_NAMESPACE" --type=json -p "$patch" "$@" -o name 2>&1); then
        echo "$out"
        return 0
      fi
      echo "$out" >&2
      if [[ "$out" == *"test operation does not apply"* ]]; then
        return 10
      fi
      fail "could not update spec.users on workstation '$name': $out"
    }
    echo "Validating spec.users on workstation $name (server-side dry run)"
    apply_users_patch --dry-run=server && rc=0 || rc=$?
    if (( rc == 10 )); then
      (( attempt < 5 )) || fail "workstation '$name' changed during the update; tried 5 times"
      echo "Workstation '$name' changed during the update, retrying ($attempt/5)"
      attempt=$(( attempt + 1 ))
      sleep 2
      continue
    fi
    apply_users_patch && rc=0 || rc=$?
    if (( rc == 10 )); then
      (( attempt < 5 )) || fail "workstation '$name' changed during the update; tried 5 times"
      echo "Workstation '$name' changed during the update, retrying ($attempt/5)"
      attempt=$(( attempt + 1 ))
      sleep 2
      continue
    fi
    USERS_CHANGED=true
    USERS_ADDED=$added
    return 0
  done
}
{{- end -}}

{{/*
Steps for cws-user-start-workstation and cws-user-stop-workstation.
Call with (dict "root" $ "action" "start|stop"). The WorkflowTemplate header, including the
workstation mutex, lives in workflowtemplates-user.yaml.
resolve reads Horizon objects with the read-only Role. Start waits for the parent config to be
Ready so no workstation boots while a new config may still allow SSH on port 22. Stop does not:
a config update or failure must not block stopping a running workstation. power runs gcloud as the
dedicated start and stop Google identity and writes nothing to Kubernetes (LLD D1: power is not a
spec field, so this is the one workflow that calls GCP).
*/}}
{{- define "cws-ws.powerTemplates" -}}
{{- $root := .root -}}
{{- $action := .action }}
  templates:
    - name: main
      steps:
        - - name: resolve
            template: resolve
        - - name: {{ $action }}
            template: power
            arguments:
              parameters:
                {{- range list "project" "region" "cluster" "config" "workstation" }}
                - name: {{ . }}
                  value: {{ printf "{{steps.resolve.outputs.parameters.%s}}" . | quote }}
                {{- end }}
        - - name: report
            template: report
    - name: resolve
      container:
        {{- include "cws.stepContainer" $root | nindent 8 }}
        env:
          - name: CWS_NAMESPACE
            value: {{ $root.Values.resourceNamespace | quote }}
          - name: WORKSTATION_NAME
            value: "{{ "{{workflow.parameters.workstationId}}" }}"
          - name: USERS
            value: "{{ "{{workflow.parameters.submittedBy}}" }}"
          - name: TIMEOUT_SECONDS
            value: {{ $root.Values.readyTimeoutSeconds | quote }}
        args:
          - |
            set -euo pipefail
            {{- include "cws-ws.shlib" $root | nindent 12 }}

            check_timeout "$TIMEOUT_SECONDS"
            name=$(resource_name workstationId "$WORKSTATION_NAME")
            users=$(single_principal submittedBy "$USERS")
            json=$(require_ws "$name")
            require_member "$json" "$users"

            config=$(jq -r '.spec.configRef.name' <<<"$json")
            require_cfg "$config" >/dev/null
            echo "Workstation $name uses config $config"
            {{- if eq $action "start" }}
            # Only start waits. Stop must still run when the config is updating or Failed.
            wait_ready "$CFG_RES" "$config" "$TIMEOUT_SECONDS"
            {{- end }}

            json=$(require_ws "$name") || exit 1
            ref=$(jq -r '.status.externalRef // ""' <<<"$json")
            [[ -n "$ref" ]] || fail "workstation '$name' has no status.externalRef yet; wait until it is Ready and submit again"
            ref=${ref#//workstations.googleapis.com/}
            [[ "$ref" =~ ^projects/([^/]+)/locations/([^/]+)/workstationClusters/([^/]+)/workstationConfigs/([^/]+)/workstations/([^/]+)$ ]] \
              || fail "unexpected status.externalRef format: '$ref'"
            printf '%s' "${BASH_REMATCH[1]}" > /tmp/project
            printf '%s' "${BASH_REMATCH[2]}" > /tmp/region
            printf '%s' "${BASH_REMATCH[3]}" > /tmp/cluster
            printf '%s' "${BASH_REMATCH[4]}" > /tmp/config
            printf '%s' "${BASH_REMATCH[5]}" > /tmp/workstation
            echo "Resolved $ref"
      outputs:
        parameters:
          {{- range list "project" "region" "cluster" "config" "workstation" }}
          - name: {{ . }}
            valueFrom:
              path: /tmp/{{ . }}
          {{- end }}
    - name: power
      inputs:
        parameters:
          {{- range list "project" "region" "cluster" "config" "workstation" }}
          - name: {{ . }}
          {{- end }}
      container:
        # Same image as cws.stepContainer, with more memory for gcloud.
        image: {{ include "cws.stepImage" $root | quote }}
        command: [bash, -c]
        securityContext:
          runAsNonRoot: true
          runAsUser: 65534
          runAsGroup: 65534
          allowPrivilegeEscalation: false
          capabilities:
            drop: [ALL]
          seccompProfile:
            type: RuntimeDefault
        resources:
          requests:
            cpu: 100m
            memory: 256Mi
          limits:
            memory: 512Mi
        env:
          # gcloud writes its configuration under CLOUDSDK_CONFIG; the default path is not writable here.
          - name: CLOUDSDK_CONFIG
            value: /tmp/gcloud
          - name: CLOUDSDK_CORE_DISABLE_PROMPTS
            value: "1"
          {{- range list "project" "region" "cluster" "config" "workstation" }}
          - name: {{ upper . }}
            value: {{ printf "{{inputs.parameters.%s}}" . | quote }}
          {{- end }}
          - name: TIMEOUT_SECONDS
            value: {{ $root.Values.readyTimeoutSeconds | quote }}
        args:
          - |
            set -euo pipefail
            echo "Google identity: $(gcloud auth list --filter=status:ACTIVE --format='value(account)')"

            # cws-workstation-power intentionally has no workstations.workstations.get (only start,
            # stop and operations.get), so the operation is started async and polled by name through
            # the Workstations API directly: a synchronous gcloud call would describe the resulting
            # workstation afterwards and fail PERMISSION_DENIED on that describe call.
            op=$(gcloud workstations {{ $action }} "$WORKSTATION" \
              --project="$PROJECT" \
              --region="$REGION" \
              --cluster="$CLUSTER" \
              --config="$CONFIG" \
              --async \
              --format='value(name)')
            echo "Operation: $op"

            deadline=$(( $(date +%s) + 10#$TIMEOUT_SECONDS ))
            while :; do
              token=$(gcloud auth print-access-token)
              if resp=$(curl -sf -H "Authorization: Bearer $token" \
                  "https://workstations.googleapis.com/v1/${op}"); then
                if jq -e '.done == true' >/dev/null <<<"$resp"; then
                  err=$(jq -r '.error.message // ""' <<<"$resp")
                  if [[ -n "$err" ]]; then
                    echo "$resp" >&2
                    echo "ERROR: {{ $action }} failed: $err" >&2
                    exit 1
                  fi
                  break
                fi
              else
                echo "Read failed, retrying"
              fi
              if (( $(date +%s) >= deadline )); then
                echo "ERROR: timed out after ${TIMEOUT_SECONDS}s waiting for {{ $action }} to finish" >&2
                exit 1
              fi
              sleep 5
            done
            echo "gcloud workstations {{ $action }} finished for $WORKSTATION"
    - name: report
      container:
        {{- include "cws.stepContainer" $root | nindent 8 }}
        env:
          - name: CWS_NAMESPACE
            value: {{ $root.Values.resourceNamespace | quote }}
          - name: WORKSTATION_NAME
            value: "{{ "{{workflow.parameters.workstationId}}" }}"
        args:
          - |
            set -euo pipefail
            {{- include "cws-ws.shlib" $root | nindent 12 }}

            echo "state and url come from Config Connector observed state and can lag the {{ $action }} by a few minutes"
            ws=$(require_ws "$(trim "$WORKSTATION_NAME")") || exit 1
            print_workstation "$ws"
{{- end -}}
