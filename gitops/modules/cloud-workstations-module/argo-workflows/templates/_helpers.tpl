{{/*
Copyright (c) 2026 Accenture, All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
*/}}

{{/* WorkflowTemplate spec.arguments.parameters from values.imageBuild.parameters. */}}
{{- define "cws.imageArguments" -}}
{{- range .Values.imageBuild.parameters }}
- name: {{ .name }}
  value: {{ .value | quote }}
  {{- if .description }}
  description: {{ .description | quote }}
  {{- end }}
{{- end }}
{{- end -}}

{{/*
Sensor trigger: submitted-by / submitted-from, then one mapping per imageBuild parameter.
src.value is the template default so omitted API fields still submit a Workflow.
*/}}
{{- define "cws.imageSensor" -}}
- src:
    dependencyName: webhook-dep
    dataKey: body.horizonSubmittedBy
  dest: metadata.annotations.horizon-sdv\.io/submitted-by
- src:
    dependencyName: webhook-dep
    dataKey: body.horizonSubmittedFrom
    value: {{ (index .Values.imageBuild.parameters 0).value | quote }}
  dest: metadata.labels.horizon-sdv\.io/submitted-from
{{- range $i, $p := .Values.imageBuild.parameters }}
- src:
    dependencyName: webhook-dep
    dataKey: body.{{ $p.name }}
    value: {{ $p.value | quote }}
  dest: spec.arguments.parameters.{{ $i }}.value
{{- end }}
{{- end -}}

{{- define "cws.labels" -}}
app.kubernetes.io/part-of: cloud-workstations-module
horizon-sdv.io/module: {{ .Values.moduleName }}
{{- end -}}

{{/* SCM method from MODULE_CONFIG (userpass | app | none / empty). */}}
{{- define "cws.scmAuthMethod" -}}
{{- $scm := (.Values.config | default dict).scm | default dict -}}
{{- $scm.authMethod | default "" -}}
{{- end -}}

{{/*
True when prepare-pipeline-git-creds must run and the git artifact uses the per-workflow Secret.
*/}}
{{- define "cws.pipelineGitCredsEnabled" -}}
{{- $auth := include "cws.scmAuthMethod" . | trim -}}
{{- if or (eq $auth "app") (eq $auth "userpass") -}}
true
{{- end -}}
{{- end -}}

{{/* HTTPS keys for the Argo git artifact (same Secret names as workloads-android). */}}
{{- define "cws.gitArtifactCreds" -}}
{{- if eq (include "cws.pipelineGitCredsEnabled" . | trim) "true" }}
usernameSecret:
  name: "{{ "{{" }}workflow.uid{{ "}}" }}-pipeline-git-creds"
  key: username
passwordSecret:
  name: "{{ "{{" }}workflow.uid{{ "}}" }}-pipeline-git-creds"
  key: password
{{- end }}
{{- end -}}

{{/* Git artifact passed to common-docker-image-build as input "source". */}}
{{- define "cws.gitSourceArtifact" -}}
- name: source
  path: /workspace
  git:
    repo: {{ include "cws.pipelineRepoUrl" . | quote }}
    revision: {{ include "cws.pipelineRepoRevision" . | quote }}
{{- include "cws.gitArtifactCreds" . | nindent 4 }}
{{- end -}}

{{/* Same clone, input name "repo", for the common-utils staging step. */}}
{{- define "cws.gitRepoArtifact" -}}
- name: repo
  path: /src
  git:
    repo: {{ include "cws.pipelineRepoUrl" . | quote }}
    revision: {{ include "cws.pipelineRepoRevision" . | quote }}
{{- include "cws.gitArtifactCreds" . | nindent 4 }}
{{- end -}}

{{- define "cws.commonBuildParams" -}}
- name: horizonSubmittedFrom
  value: '{{ "{{" }}workflow.parameters.horizonSubmittedFrom{{ "}}" }}'
- name: cloudProject
  value: {{ include "cws.cloudProject" . | quote }}
- name: cloudRegion
  value: {{ include "cws.cloudRegion" . | quote }}
- name: imageTag
  value: '{{ "{{" }}workflow.parameters.imageTag{{ "}}" }}'
- name: dryRun
  value: '{{ "{{" }}workflow.parameters.dryRun{{ "}}" }}'
- name: platform
  value: linux/amd64
{{- end -}}

{{- define "cws.pipelineRepoUrl" -}}
{{- required "pipelineRepoUrl is required (parent Application repo.url)" .Values.pipelineRepoUrl -}}
{{- end -}}

{{- define "cws.pipelineRepoRevision" -}}
{{- required "pipelineRepoRevision is required (parent Application repo.revision)" .Values.pipelineRepoRevision -}}
{{- end -}}

{{- define "cws.cloudProject" -}}
{{- required "config.projectID is required" ((.Values.config | default dict).projectID) -}}
{{- end -}}

{{- define "cws.cloudRegion" -}}
{{- required "config.region is required" ((.Values.config | default dict).region) -}}
{{- end -}}

{{/* region-docker.pkg.dev/project/path:tag  (tag is an Argo expression, already escaped). */}}
{{- define "cws.imageRef" -}}
{{- $root := .root -}}
{{- printf "%s-docker.pkg.dev/%s/%s:%s" (include "cws.cloudRegion" $root) (include "cws.cloudProject" $root) .path .tag -}}
{{- end -}}

{{- define "cws.horizonDomain" -}}
{{- ((.Values.config | default dict).domain) | default "" -}}
{{- end -}}

{{/*
Container fields shared by every Cloud Workstations workflow step.
The image is built by cws-image-build (buildEnvironmentImage=true); steps stay Pending until it is pushed.
*/}}
{{- define "cws.stepImage" -}}
{{- include "cws.imageRef" (dict "root" . "path" .Values.images.environment.dockerArtifactPathName "tag" .Values.images.environment.tag) -}}
{{- end -}}

{{- define "cws.nonRootSecurityContext" -}}
securityContext:
  runAsNonRoot: true
  runAsUser: 65534
  runAsGroup: 65534
  allowPrivilegeEscalation: false
  capabilities:
    drop: [ALL]
  seccompProfile:
    type: RuntimeDefault
{{- end -}}

{{- define "cws.stepContainer" -}}
image: {{ include "cws.stepImage" . | quote }}
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
    cpu: 50m
    memory: 64Mi
  limits:
    memory: 128Mi
{{- end -}}

{{/*
One write workflow at a time per config: create, update and delete share this lock, different configs
do not block each other. Argo resolves the name when the workflow runs; the trim matches the name the
validate step uses, so " a" and "a" share a lock. Read-only templates do not take it.
*/}}
{{- define "cws.configMutex" -}}
synchronization:
  mutexes:
    - name: {{ printf "cws-config-%s" "{{=sprig.trim(workflow.parameters.cloudWsConfigName)}}" | quote }}
{{- end -}}

{{/*
Bash functions shared by the step scripts. Requires CWS_NAMESPACE.
Horizon CRs share kind names with Config Connector, so kubectl always gets the full resource.group name.
Keep double braces out of this block: Argo would treat them as parameters.
*/}}
{{- define "cws.shellLib" -}}
fail() { echo "ERROR: $*" >&2; exit 1; }

resource_for() {
  case "$1" in
    workstationconfig) echo workstationconfigs.workstations.horizon-sdv.io ;;
    workstation) echo workstations.workstations.horizon-sdv.io ;;
    *) fail "kind must be workstationconfig or workstation, got '$1'" ;;
  esac
}

check_timeout() {
  if ! [[ "$1" =~ ^[0-9]+$ ]] || (( 10#$1 == 0 )); then
    fail "timeoutSeconds must be a positive integer, got '$1'"
  fi
}

# Prints the object as JSON, or nothing when it does not exist.
cr_json() {
  local err
  err=$(mktemp)
  if kubectl get "$1" "$2" -n "$CWS_NAMESPACE" -o json 2>"$err"; then
    rm -f "$err"
    return 0
  fi
  if grep -q '(NotFound)' "$err"; then
    rm -f "$err"
    return 0
  fi
  cat "$err" >&2
  rm -f "$err"
  return 1
}

# ready: Ready=True for the current generation.
# failed: phase Failed with conditions for the current generation (terminal API error, e.g. InvalidArgument).
# waiting: anything else, including Failed left over from an older generation.
ready_state() {
  jq -r '
    .metadata.generation as $g
    | ((.status.conditions // []) | map(select(.type == "Ready")) | first) as $r
    | if $r != null and $r.status == "True" and $r.observedGeneration == $g then "ready"
      elif .status.phase == "Failed" and $r != null and $r.observedGeneration == $g then "failed"
      else "waiting" end'
}

progress_line() {
  jq -r '
    ((.status.conditions // []) | map(select(.type == "Ready")) | first) as $r
    | "phase=\(.status.phase // "-") generation=\(.metadata.generation) ready=\($r.status // "-") reason=\($r.reason // "-") message=\($r.message // "-")"
      + (if .metadata.deletionTimestamp then " deleting finalizers=\(.metadata.finalizers // [] | join(","))" else "" end)'
}

print_status() {
  jq -r '
    "name: \(.metadata.name)",
    "phase: \(.status.phase // "-")",
    "generation: \(.metadata.generation), observedGeneration: \(.status.observedGeneration // "-")",
    "externalRef: \(.status.externalRef // "-")",
    ((.status.conditions // [])[] | "condition \(.type)=\(.status) reason=\(.reason // "-") message=\(.message // "-")")'
}

# wait_ready RESOURCE NAME TIMEOUT
wait_ready() {
  local resource=$1 name=$2 timeout=$3 deadline last="" line json state
  deadline=$(( $(date +%s) + 10#$timeout ))
  echo "Waiting up to ${timeout}s for $name to become Ready"
  while :; do
    if ! json=$(cr_json "$resource" "$name"); then
      echo "Read failed, retrying"
    elif [[ -z "$json" ]]; then
      fail "$name no longer exists"
    else
      line=$(progress_line <<<"$json")
      [[ "$line" != "$last" ]] && echo "$(date -u +%H:%M:%S) $line" && last=$line
      state=$(ready_state <<<"$json")
      if [[ "$state" == ready ]]; then
        print_status <<<"$json"
        return 0
      fi
      if [[ "$state" == failed ]]; then
        print_status <<<"$json"
        fail "$name failed; see the conditions above"
      fi
    fi
    if (( $(date +%s) >= deadline )); then
      [[ -n "${json:-}" ]] && print_status <<<"$json"
      fail "timed out after ${timeout}s waiting for $name to become Ready"
    fi
    sleep 10
  done
}

# wait_gone RESOURCE NAME TIMEOUT
# Stops early on Ready reason DeletionBlocked: the controller only sets it while deleting and
# will not finish until the blocker is removed, so waiting out the timeout adds nothing.
wait_gone() {
  local resource=$1 name=$2 timeout=$3 deadline last="" line json blocked
  deadline=$(( $(date +%s) + 10#$timeout ))
  echo "Waiting up to ${timeout}s for $name to be deleted"
  while :; do
    if ! json=$(cr_json "$resource" "$name"); then
      echo "Read failed, retrying"
    elif [[ -z "$json" ]]; then
      echo "$name deleted"
      return 0
    else
      line=$(progress_line <<<"$json")
      [[ "$line" != "$last" ]] && echo "$(date -u +%H:%M:%S) $line" && last=$line
      blocked=$(jq -r 'select(.metadata.deletionTimestamp) | (.status.conditions // [])[] | select(.type == "Ready" and .reason == "DeletionBlocked") | .message' <<<"$json")
      if [[ -n "$blocked" ]]; then
        print_status <<<"$json"
        fail "$name deletion is blocked: ${blocked}. It stays marked for deletion and is removed once the blocker is gone."
      fi
    fi
    if (( $(date +%s) >= deadline )); then
      [[ -n "${json:-}" ]] && print_status <<<"$json"
      fail "timed out after ${timeout}s waiting for $name to be deleted"
    fi
    sleep 10
  done
}
{{- end -}}

{{/*
Platform values for the config workflows, passed to steps as env. Rendering fails when MODULE_CONFIG lacks them.
*/}}
{{- define "cws.platformEnv" -}}
{{- $cws := .Values.config.cloudWorkstations | default dict -}}
- name: CWS_NAMESPACE
  value: {{ .Values.resourceNamespace | quote }}
- name: CWS_CLUSTER_ID
  value: {{ required "config.cloudWorkstations.clusterId is required" $cws.clusterId | quote }}
- name: CWS_PROJECT_ID
  value: {{ required "config.projectID is required" .Values.config.projectID | quote }}
- name: CWS_REGION
  value: {{ required "config.region is required" .Values.config.region | quote }}
- name: CWS_IMAGE_CODE_OSS
  value: {{ .Values.workstationImages.codeOss | quote }}
- name: CWS_IMAGE_ANDROID_STUDIO
  value: {{ .Values.workstationImages.androidStudio | quote }}
- name: CWS_IMAGE_ASFP
  value: {{ .Values.workstationImages.asfp | quote }}
{{- end -}}

{{/*
Ordered parameter list of cws-create-config ("create") or cws-update-config ("update").
Usage: include "cws.configParamList" (dict "root" . "mode" "create") | fromJsonArray
*/}}
{{- define "cws.configParamList" -}}
{{- $out := list -}}
{{- range .root.Values.configParameters -}}
{{- if and .updateRequired (eq $.mode "update") -}}
{{- $out = append $out (merge (dict "description" (printf "REQUIRED: %s" .description)) (omit . "value")) -}}
{{- else if not (and .createOnly (eq $.mode "update")) -}}
{{- $out = append $out . -}}
{{- end -}}
{{- end -}}
{{- toJson $out -}}
{{- end -}}

{{/*
WorkflowTemplate spec.arguments.parameters from a list of {name, value?, description?}.
horizonSubmittedFrom comes first, as the Sensor maps it to index 0.
*/}}
{{- define "cws.arguments" -}}
- name: horizonSubmittedFrom
  value: ""
{{- range . }}
- name: {{ .name }}
  {{- if hasKey . "value" }}
  value: {{ .value | quote }}
  {{- end }}
  {{- with .description }}
  description: {{ . | quote }}
  {{- end }}
{{- end }}
{{- end -}}

{{/*
Step env carrying each workflow parameter under its own name. User input never goes into script text.
*/}}
{{- define "cws.parameterEnv" -}}
{{- range . }}
- name: {{ .name }}
  value: "{{ printf "{{workflow.parameters.%s}}" .name }}"
{{- end }}
{{- end -}}

{{/*
Sensor submitting one WorkflowTemplate from the platform webhook.
Usage: include "cws.sensor" (dict "root" . "template" "cws-get-config" "params" $list)
Parameters are mapped by index, in the same order as cws.arguments. A key the Horizon API caller
omits (allowed when the parameter has a default) falls back to that default.
*/}}
{{- define "cws.sensor" -}}
apiVersion: argoproj.io/v1alpha1
kind: Sensor
metadata:
  name: webhook-{{ .template }}
  namespace: {{ .root.Values.eventsNamespace }}
  labels:
    {{- include "cws.labels" .root | nindent 4 }}
  annotations:
    argocd.argoproj.io/sync-wave: "8"
spec:
  eventBusName: default
  template:
    serviceAccountName: sensor-submit-workflow
  dependencies:
    - name: webhook-dep
      eventSourceName: webhook
      eventName: workflow-dispatch
      filters:
        data:
          - path: body.workflowTemplateName
            type: string
            value:
              - {{ .template | quote }}
  triggers:
    - template:
        name: submit-{{ .template }}
        argoWorkflow:
          operation: submit
          source:
            resource:
              apiVersion: argoproj.io/v1alpha1
              kind: Workflow
              metadata:
                generateName: {{ .template }}-
                namespace: {{ .root.Values.workflowNamespace }}
              spec:
                workflowTemplateRef:
                  name: {{ .template }}
                arguments:
                  parameters:
                    - name: horizonSubmittedFrom
                    {{- range .params }}
                    - name: {{ .name }}
                    {{- end }}
          parameters:
            - src:
                dependencyName: webhook-dep
                dataKey: body.horizonSubmittedBy
              dest: metadata.annotations.horizon-sdv\.io/submitted-by
            - src:
                dependencyName: webhook-dep
                dataKey: body.horizonSubmittedFrom
              dest: metadata.labels.horizon-sdv\.io/submitted-from
            - src:
                dependencyName: webhook-dep
                dataKey: body.horizonSubmittedFrom
              dest: spec.arguments.parameters.0.value
            {{- range $i, $p := .params }}
            - src:
                dependencyName: webhook-dep
                dataKey: body.{{ $p.name }}
                {{- if hasKey $p "value" }}
                value: {{ $p.value | quote }}
                {{- end }}
              dest: spec.arguments.parameters.{{ add1 $i }}.value
            {{- end }}
{{- end -}}

{{/*
Jenkins log helpers (workloads/cloud-workstations/pipelines/utils/terraform-utils.sh) without ANSI colours.
Keep double braces out of this block: Argo would treat them as parameters.
*/}}
{{- define "cws.jenkinsLog" -}}
print_header() {
  echo
  echo "┌────────────────────────────────────────────────────────────────┐"
  printf "│ %-62s │\n" "$1"
  echo "└────────────────────────────────────────────────────────────────┘"
}

print_result() {
  local line
  echo "┌─RESULT"
  while IFS= read -r line || [[ -n $line ]]; do
    printf '    %s\n' "$line"
  done
  echo "└─"
}

log_info() { printf '\n [INFO] %s \n\n' "$1" >&2; }
log_success() { printf '\n [SUCCESS] %s \n\n' "$1" >&2; }
log_warning() { printf '\n [WARNING] %s \n\n' "$1" >&2; }
log_error() { printf '\n [ERROR] %s\n' "$1" >&2; exit 1; }

trim() {
  local s=$1
  s="${s#"${s%%[![:space:]]*}"}"
  s="${s%"${s##*[![:space:]]}"}"
  printf '%s' "$s"
}

# require_config_name: sets CONFIG_NAME from cloudWsConfigName.
require_config_name() {
  CONFIG_NAME=$(trim "$cloudWsConfigName")
  [[ -n "$CONFIG_NAME" ]] || log_error "Required Parameter value NOT provided as input: cloudWsConfigName. A Workstation Config name is needed."
  [[ "$CONFIG_NAME" =~ ^[a-z]([-a-z0-9]{0,61}[a-z0-9])?$ ]] \
    || log_error "cloudWsConfigName must be 1-63 lowercase letters, digits or '-', start with a letter and not end with '-', got '${CONFIG_NAME}'."
}

# require_cluster [write]: write also rejects a deleting cluster, as the controller keeps
# configs under it Pending and their presence blocks the cluster's own deletion.
require_cluster() {
  local json
  log_info "Checking if Workstation Cluster exists..."
  json=$(cr_json workstationclusters.workstations.horizon-sdv.io "$CWS_CLUSTER_ID") \
    || log_error "Failed to read Workstation Cluster '${CWS_CLUSTER_ID}'."
  [[ -n "$json" ]] || log_error "Workstation Cluster '${CWS_CLUSTER_ID}' must exist before any operation of Workstation Config. Enable the cloud-workstations module to create it."
  if [[ "${1:-}" == write ]]; then
    [[ -z "$(jq -r '.metadata.deletionTimestamp // empty' <<<"$json")" ]] \
      || log_error "Workstation Cluster '${CWS_CLUSTER_ID}' is being deleted. Workstation Configs cannot be created or updated."
    [[ "$(jq -r '.status.phase // empty' <<<"$json")" == Ready ]] \
      || log_warning "Workstation Cluster '${CWS_CLUSTER_ID}' is not Ready yet. The config will wait for cluster provisioning."
  fi
  log_info "Workstation Cluster exists. Proceeding..."
}

# load_config NAME: sets CONFIG_JSON (empty when the config does not exist).
load_config() {
  CONFIG_JSON=$(cr_json "$(resource_for workstationconfig)" "$1") \
    || log_error "Failed to read Workstation Config '$1'."
}

require_existing_config() {
  load_config "$CONFIG_NAME"
  [[ -n "$CONFIG_JSON" ]] || log_error "Please enter a workstation Configuration name that exists. Aborting..."
  log_info "Input WS Config: '${CONFIG_NAME}' found in existing WS Configs."
}

# Names of the Workstations whose spec.configRef.name is $1, one per line.
workstations_of_config() {
  local json
  json=$(kubectl get "$(resource_for workstation)" -n "$CWS_NAMESPACE" -o json) \
    || log_error "Failed to list Workstations."
  jq -r --arg config_name "$1" '.items | map(select(.spec.configRef.name == $config_name)) | .[].metadata.name' <<<"$json"
}
{{- end -}}

{{/*
Builds the WorkstationConfig spec from the Jenkins create/update parameters (env, same names).
Conversions and defaults follow the Jenkinsfile buildTfvarsMap and workloads/cloud-workstations/terraform/config.
Requires cws.shellLib, cws.jenkinsLog and cws.platformEnv. Keep double braces out of this block.
*/}}
{{- define "cws.configSpecLib" -}}
# param_bool NAME DEFAULT
param_bool() {
  local v
  v=$(trim "${!1}")
  if [[ -z "$v" ]]; then echo "$2"; return; fi
  case "${v,,}" in
    true|false) echo "${v,,}" ;;
    *) log_error "$1 must be true or false, got '${v}'." ;;
  esac
}

# param_int NAME DEFAULT (DEFAULT may be empty: prints null)
param_int() {
  local v
  v=$(trim "${!1}")
  v=${v:-$2}
  if [[ -z "$v" ]]; then echo null; return; fi
  [[ "$v" =~ ^[0-9]+$ ]] || log_error "$1 must be a whole number, got '${v}'."
  echo $(( 10#$v ))
}

# param_str NAME DEFAULT
param_str() {
  local v
  v=$(trim "${!1}")
  printf '%s' "${v:-$2}"
}

# param_list NAME: comma-separated values as a JSON array, trimmed, empty items dropped.
param_list() {
  jq -cn --arg v "${!1}" '$v | split(",") | map(gsub("^\\s+|\\s+$"; "")) | map(select(length > 0))'
}

param_image() {
  local choice tag path
  choice=$(trim "$containerImage")
  tag=$(param_str containerImageTag latest)
  case "$choice" in
    ""|"C-OSS (VCS)") path=$CWS_IMAGE_CODE_OSS ;;
    AS) path=$CWS_IMAGE_ANDROID_STUDIO ;;
    ASfP) path=$CWS_IMAGE_ASFP ;;
    */*) printf '%s' "$choice"; return ;;
    *) log_error "containerImage must be 'C-OSS (VCS)', 'AS', 'ASfP' or a full image URI, got '${choice}'." ;;
  esac
  printf '%s' "${CWS_REGION}-docker.pkg.dev/${CWS_PROJECT_ID}/${path}:${tag}"
}

param_env_vars() {
  local v
  v=$(trim "$containerEnvVars")
  if [[ -z "$v" ]]; then echo '[]'; return; fi
  jq -cse 'if length == 1 then .[0] else error("invalid") end
    | if type == "object" and all(.[]; type == "string" or type == "number" or type == "boolean")
    then to_entries | map({name: .key, value: (.value | tostring)}) else error("invalid") end' <<<"$v" 2>/dev/null \
    || log_error "containerEnvVars must be a JSON object of string values, e.g. {\"ENV1\":\"val1\", \"ENV2\":\"val2\"}, got '${v}'."
}

param_ports() {
  local v
  v=$(trim "$wsAllowedPorts")
  if [[ -z "$v" ]]; then echo '[{"first":80,"last":80},{"first":1024,"last":65535}]'; return; fi
  jq -cse 'def port: type == "number" and . == floor;
    if length == 1 then .[0] else error("invalid") end
    | if type == "array" and length > 0 and all(.[]; type == "object" and (.first | port) and (.last | port))
    then map({first, last}) else error("invalid") end' <<<"$v" 2>/dev/null \
    || log_error "wsAllowedPorts must be a JSON list of port objects, e.g. [{\"first\":80, \"last\":80}, {\"first\":1024, \"last\":65535}], got '${v}'."
}

# Bare emails become user:<email> (as the Terraform IAM binding did). Bare, user:, group: and
# serviceAccount: entries must be emails, so a typo fails here rather than at IAM after the config exists.
# Duplicates are dropped ignoring case, first spelling kept, as the controller compares members.
param_admins() {
  local parsed bad
  parsed=$(jq -cn --arg v "$wsAdminIamMembers" '
    def email: test("^[^@\\s]+@[^@\\s]+\\.[^@\\s]+$");
    ($v | split(",") | map(gsub("^\\s+|\\s+$"; "")) | map(select(length > 0))) as $in
    | {
        bad: ($in | map(select(
          if test("^(user|group|serviceAccount):") then (sub("^[^:]+:"; "") | email | not)
          elif test("^(domain:|principal://|principalSet://)") then false
          else (email | not) end)) | join(", ")),
        members: ($in
          | map(if test("^(user:|group:|serviceAccount:|domain:|principal://|principalSet://)") then . else "user:" + . end)
          | reduce .[] as $m ([]; if any(.[]; ascii_downcase == ($m | ascii_downcase)) then . else . + [$m] end))
      }')
  bad=$(jq -r .bad <<<"$parsed")
  [[ -z "$bad" ]] \
    || log_error "wsAdminIamMembers entries must be email addresses, optionally prefixed with user:, group: or serviceAccount:, got '${bad}'."
  jq -c .members <<<"$parsed"
}

# Checks done by the Jenkinsfile before Terraform ran.
check_config_required_inputs() {
  local ed_required
  require_config_name
  [[ -n "$(trim "$wsAdminIamMembers")" ]] \
    || log_error "Required Parameter value NOT provided as input: wsAdminIamMembers. Atleast one user email address is needed to be granted Workstation Admin access."
  ed_required=$(param_bool edRequired false)
  if [[ "$ed_required" == true && -z "$(trim "$edSourceSnapshot")" && -z "$(trim "$edSourceImage")" ]]; then
    log_error "Required Parameter value NOT provided as input: either of edSourceSnapshot or edSourceImage must be set when edRequired is selected."
  fi
}

# require_update_inputs NAME...: these have no default on update, so blank is rejected rather than
# defaulted (a default would replace the current image or drop a disk).
require_update_inputs() {
  local name
  for name in "$@"; do
    [[ -n "$(trim "${!name}")" ]] \
      || log_error "Required Parameter value NOT provided as input: ${name}. Update replaces every field, so it must be set explicitly."
  done
}

# parse_config_inputs MODE (create or update): validates formats and sets the P_* values.
parse_config_inputs() {
  P_IMAGE=$(param_image)
  P_IDLE=$(param_int wsIdleTimeout 1200)
  P_RUNNING=$(param_int wsRunningTimeout 43200)
  P_ZONES='[]'
  if [[ "$1" == create ]]; then P_ZONES=$(param_list wsReplicaZones); fi
  P_MACHINE=$(param_str hostMachineType e2-standard-4)
  P_POOL=$(param_int hostQuickStartPoolSize 0)
  P_BOOT=$(param_int hostBootDiskSize 40)
  P_NO_PUBLIC_IP=$(param_bool hostDisablePublicIpAddresses true)
  P_NO_SSH=$(param_bool hostDisableSsh true)
  P_NESTED=$(param_bool hostEnableNestedVirtualization false)
  P_PD_REQUIRED=$(param_bool pdRequired false)
  P_PD_MOUNT=$(param_str pdMountPath /home)
  P_PD_FS=$(param_str pdFsType "")
  P_PD_DISK=$(param_str pdDiskType pd-standard)
  P_PD_SIZE=$(param_int pdSizeGb "")
  P_PD_RECLAIM=$(param_str pdReclaimPolicy DELETE)
  P_PD_SNAPSHOT=$(param_str pdSourceSnapshot "")
  P_ED_REQUIRED=$(param_bool edRequired false)
  P_ED_MOUNT=$(param_str edMountPath /tmp)
  P_ED_DISK=$(param_str edDiskType pd-standard)
  P_ED_SNAPSHOT=$(param_str edSourceSnapshot "")
  P_ED_IMAGE=$(param_str edSourceImage "")
  P_ED_READ_ONLY=$(param_bool edReadOnly false)
  P_COMMANDS=$(param_list containerEntrypointCommands)
  P_ARGS=$(param_list containerEntrypointArgs)
  P_WORKDIR=$(param_str containerWorkingDir "")
  P_ENV=$(param_env_vars)
  P_USER=$(param_int containerUser "")
  P_PORTS=$(param_ports)
  P_ADMINS=$(param_admins)
  [[ "$P_ADMINS" != "[]" ]] \
    || log_error "Required Parameter value NOT provided as input: wsAdminIamMembers. Atleast one user email address is needed to be granted Workstation Admin access."
}

# Validation rules of variable sdv_cloud_ws_configs (terraform/config/variables.tf), same messages.
# The snapshot rule is checked on the user input, so a snapshot works once pdFsType and pdSizeGb are blank.
check_config_rules() {
  local pd_size=${P_PD_SIZE/null/200}
  if [[ -n "$P_PD_SNAPSHOT" && ( -n "$P_PD_FS" || "$P_PD_SIZE" != null ) ]]; then
    log_error "If a Persistent disk 'source snapshot' is set then 'file system type' or 'disk size' cannot be specified."
  fi
  case "$pd_size" in
    10|50|100|200|500|1000) ;;
    *) log_error "Persistent disk size must be one of: 10, 50, 100, 200, 500, 1000 GB." ;;
  esac
  if (( pd_size < 200 )) && [[ "$P_PD_DISK" != pd-balanced && "$P_PD_DISK" != pd-ssd ]]; then
    log_error "If Persistent disk size is less than 200 GB, disk type must be 'pd-balanced' or 'pd-ssd'."
  fi
  if [[ -n "$P_ED_SNAPSHOT" && -n "$P_ED_IMAGE" ]]; then
    log_error "Ephemeral disk 'source snapshot' and 'source image' cannot be specified together."
  fi
  if [[ -n "$P_ED_SNAPSHOT" && "$P_ED_READ_ONLY" == false ]]; then
    log_error "If Ephemeral disk 'source snapshot' is set then disk must be 'read-only'."
  fi
  if [[ "$P_ED_READ_ONLY" == true && -z "$P_ED_SNAPSHOT" ]]; then
    log_error "If Ephemeral disk is set as 'read-only' then a 'disk source snapshot' must also be specified."
  fi
}

# config_spec MODE: the spec fields the Jenkins jobs manage, as JSON. create also sets clusterRef and replicaZones.
config_spec() {
  jq -cn \
    --arg mode "$1" --arg cluster "$CWS_CLUSTER_ID" --arg project "$CWS_PROJECT_ID" \
    --arg image "$P_IMAGE" --argjson idle "$P_IDLE" --argjson running "$P_RUNNING" --argjson zones "$P_ZONES" \
    --arg machine "$P_MACHINE" --argjson pool "$P_POOL" --argjson boot "$P_BOOT" \
    --argjson noPublicIp "$P_NO_PUBLIC_IP" --argjson noSsh "$P_NO_SSH" --argjson nested "$P_NESTED" \
    --argjson pdRequired "$P_PD_REQUIRED" --arg pdMount "$P_PD_MOUNT" --arg pdFs "$P_PD_FS" --arg pdDisk "$P_PD_DISK" \
    --argjson pdSize "$P_PD_SIZE" --arg pdReclaim "$P_PD_RECLAIM" --arg pdSnapshot "$P_PD_SNAPSHOT" \
    --argjson edRequired "$P_ED_REQUIRED" --arg edMount "$P_ED_MOUNT" --arg edDisk "$P_ED_DISK" \
    --arg edSnapshot "$P_ED_SNAPSHOT" --arg edImage "$P_ED_IMAGE" --argjson edReadOnly "$P_ED_READ_ONLY" \
    --argjson commands "$P_COMMANDS" --argjson args "$P_ARGS" --arg workdir "$P_WORKDIR" \
    --argjson env "$P_ENV" --argjson user "$P_USER" --argjson ports "$P_PORTS" --argjson admins "$P_ADMINS" '
    def unless_empty($k; $v): if ($v | length) > 0 then {($k): $v} else {} end;
    {
      idleTimeout: "\($idle)s",
      runningTimeout: "\($running)s",
      container: (
        {image: $image}
        + unless_empty("command"; $commands)
        + unless_empty("args"; $args)
        + unless_empty("workingDir"; $workdir)
        + unless_empty("env"; $env)
        + (if $user != null then {runAsUser: $user} else {} end)
      ),
      host: {gceInstance: {
        machineType: $machine,
        poolSize: $pool,
        bootDiskSizeGB: $boot,
        disablePublicIPAddresses: $noPublicIp,
        enableNestedVirtualization: $nested
      }},
      disableSsh: $noSsh,
      allowedPorts: $ports,
      admins: $admins
    }
    + (if $mode == "create" then {clusterRef: {name: $cluster}} + unless_empty("replicaZones"; $zones) else {} end)
    + (if $pdRequired then {persistentDirectories: [{
        mountPath: $pdMount,
        gcePD: ({diskType: $pdDisk, reclaimPolicy: $pdReclaim}
          + (if $pdSnapshot != "" then {sourceSnapshot: "projects/\($project)/global/snapshots/\($pdSnapshot)"}
             else {fsType: (if $pdFs == "" then "ext4" else $pdFs end), sizeGB: ($pdSize // 200)} end))
      }]} else {} end)
    + (if $edRequired then {ephemeralDirectories: [{
        mountPath: $edMount,
        gcePD: ({diskType: $edDisk, readOnly: $edReadOnly}
          + unless_empty("sourceSnapshot"; $edSnapshot)
          + unless_empty("sourceImage"; $edImage))
      }]} else {} end)'
}

# Update replaces every Jenkins-managed field; other spec fields (replicaZones, displayName, labels, ...) stay.
# Reads the current object on stdin and the managed spec in $1; prints a JSON merge patch {spec: ...}.
config_update_patch() {
  jq -c --argjson managed "$1" '
    def merge_patch($a; $b):
      if ($a | type) == "object" and ($b | type) == "object" then
        reduce ((($a | keys_unsorted) + ($b | keys_unsorted)) | unique[]) as $k ({};
          if ($b | has($k) | not) then .[$k] = null
          elif ($a | has($k)) and $a[$k] == $b[$k] then .
          else .[$k] = merge_patch($a[$k]; $b[$k]) end)
      else $b end;
    .spec as $current
    | ($current
        | del(.idleTimeout, .runningTimeout, .container, .persistentDirectories, .ephemeralDirectories, .disableSsh, .allowedPorts, .admins)
        | .host.gceInstance = ((.host.gceInstance // {}) | del(.machineType, .poolSize, .bootDiskSizeGB, .disablePublicIPAddresses, .enableNestedVirtualization))
      ) * $managed
    | {spec: merge_patch($current; .)}'
}

# Config details in the key layout of get_existing_ws_configs_with_ws_admins (terraform-utils.sh); object on stdin.
config_details() {
  jq '
    .spec as $s
    | ($s.persistentDirectories // [])[0] as $pd
    | ($s.ephemeralDirectories // [])[0] as $ed
    | def nonempty: if . == null or . == "" then null else . end;
    {
      ws_idle_timeout: ($s.idleTimeout | if . == null then null else ((sub("s$"; "") | tonumber?) // .) end),
      ws_running_timeout: ($s.runningTimeout | if . == null then null else ((sub("s$"; "") | tonumber?) // .) end),
      ws_replica_zones: ($s.replicaZones // []),
      host_machine_type: $s.host.gceInstance.machineType,
      host_quickstart_pool_size: $s.host.gceInstance.poolSize,
      host_boot_disk_size_gb: $s.host.gceInstance.bootDiskSizeGB,
      host_disable_public_ip_addresses: $s.host.gceInstance.disablePublicIPAddresses,
      host_disable_ssh: $s.disableSsh,
      host_enable_nested_virtualization: $s.host.gceInstance.enableNestedVirtualization,
      pd_required: ($pd != null),
      pd_mount_path: $pd.mountPath,
      pd_fs_type: $pd.gcePD.fsType,
      pd_disk_type: $pd.gcePD.diskType,
      pd_size_gb: $pd.gcePD.sizeGB,
      pd_reclaim_policy: $pd.gcePD.reclaimPolicy,
      pd_source_snapshot: ($pd.gcePD.sourceSnapshot | nonempty | if . then sub("^.*snapshots/"; "") else . end),
      ed_required: ($ed != null),
      ed_mount_path: $ed.mountPath,
      ed_disk_type: $ed.gcePD.diskType,
      ed_source_snapshot: ($ed.gcePD.sourceSnapshot | nonempty),
      ed_source_image: ($ed.gcePD.sourceImage | nonempty),
      ed_read_only: $ed.gcePD.readOnly,
      container_image: $s.container.image,
      container_entrypoint_commands: ($s.container.command // []),
      container_entrypoint_args: ($s.container.args // []),
      container_working_dir: ($s.container.workingDir | nonempty),
      container_env_vars: (($s.container.env // []) | map({(.name): .value}) | add // {}),
      container_user: ($s.container.runAsUser | if . == 0 then null else . end),
      ws_allowed_ports: ($s.allowedPorts // []),
      ws_admin_iam_members: (($s.admins // []) | map(sub("^user:"; "")) | unique)
    }'
}
{{- end -}}

{{/*
Final step of the write workflows: prints the Jenkins success lines.
*/}}
{{- define "cws.reportTemplate" -}}
- name: report
  inputs:
    parameters:
      - name: message
      - name: status
  container:
    {{- include "cws.stepContainer" . | nindent 4 }}
    env:
      - name: MESSAGE
        value: "{{ "{{inputs.parameters.message}}" }}"
      - name: STATUS
        value: "{{ "{{inputs.parameters.status}}" }}"
    args:
      - |
        set -euo pipefail
        {{- include "cws.jenkinsLog" . | nindent 8 }}

        log_success "$MESSAGE"
        echo "$STATUS"
{{- end -}}
