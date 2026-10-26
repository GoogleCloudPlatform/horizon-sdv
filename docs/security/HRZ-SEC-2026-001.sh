#!/usr/bin/env bash
#
# Rotate the gerrit-admin HTTP password on a running Horizon deployment and
# update the Kubernetes secrets Jenkins and the Gerrit MCP server read it from.
#
# The current password is read from the jenkins-gerrit-http-password secret
# and used to authenticate the change through the Gerrit REST API.
#
# Usage:
#   ./HRZ-SEC-2026-001.sh <domain>
#   ./HRZ-SEC-2026-001.sh horizon.example.com
#
# <domain> is the Horizon domain, with or without https://. Gerrit is
# expected at https://<domain>/gerrit.
#
# Prints a summary and asks for confirmation before changing anything.
#
# Environment:
#   NAMESPACE_PREFIX  Namespace prefix for sub-environments (default empty)
#   DRY_RUN=1         Check access and credentials without changing anything

set -euo pipefail

if [ "$#" -ne 1 ] || [ -z "$1" ]; then
  echo "Usage: $0 <domain>" >&2
  exit 1
fi

DOMAIN="${1#https://}"
DOMAIN="${DOMAIN#http://}"
DOMAIN="${DOMAIN%%/*}"
GERRIT_URL="https://${DOMAIN}/gerrit"
NAMESPACE_PREFIX="${NAMESPACE_PREFIX:-}"
DRY_RUN="${DRY_RUN:-0}"

JENKINS_NS="${NAMESPACE_PREFIX}jenkins"
GERRIT_NS="${NAMESPACE_PREFIX}gerrit"
JENKINS_SECRET="jenkins-gerrit-http-password"
GERRIT_SECRET="gerrit-http-password"
MCP_DEPLOYMENT="${NAMESPACE_PREFIX}gerrit-mcp-server"
USERNAME="gerrit-admin"

die() { echo "ERROR: $*" >&2; exit 1; }
info() { echo "==> $*"; }

for cmd in kubectl curl jq base64; do
  command -v "$cmd" >/dev/null || die "$cmd is required"
done

KUBE_CONTEXT="$(kubectl config current-context 2>/dev/null || echo '<none>')"

cat <<EOF
This script rotates the gerrit-admin HTTP password.

  Gerrit:        ${GERRIT_URL}
  Kube context:  ${KUBE_CONTEXT}
  Namespaces:    ${JENKINS_NS}, ${GERRIT_NS}

EOF

if [ "$DRY_RUN" = "1" ]; then
  cat <<EOF
Dry run. It will only:
  1. Read the current password from ${JENKINS_NS}/${JENKINS_SECRET}.
  2. Check that Gerrit accepts it.
Nothing will be changed.

EOF
else
  cat <<EOF
It will:
  1. Read the current password from ${JENKINS_NS}/${JENKINS_SECRET}
     and check that Gerrit accepts it.
  2. Generate a random 32-character password.
  3. Set it on the ${USERNAME} account through the Gerrit REST API.
     The old password stops working immediately.
  4. Write it into ${JENKINS_NS}/${JENKINS_SECRET} and
     ${GERRIT_NS}/${GERRIT_SECRET}. Jenkins picks it up from the secret.
  5. Restart ${GERRIT_NS}/${MCP_DEPLOYMENT} so it loads the new password.

Anything else still using the old password will fail to authenticate.

EOF
  read -r -p "Continue? [y/N] " answer || answer=""
  case "$answer" in
    y|Y|yes|YES) echo ;;
    *) echo "Aborted, nothing changed."; exit 1 ;;
  esac
fi

b64() { printf '%s' "$1" | base64 | tr -d '\n'; }

# Prints the HTTP status of GET /a/accounts/self with the given password.
gerrit_auth_status() {
  curl -sS -o /dev/null -w '%{http_code}' -u "${USERNAME}:$1" \
    "${GERRIT_URL}/a/accounts/self"
}

read_secret_password() {
  kubectl -n "$1" get secret "$2" -o jsonpath='{.data.password}' | base64 --decode
}

info "Checking cluster access"
kubectl -n "$JENKINS_NS" get secret "$JENKINS_SECRET" >/dev/null \
  || die "cannot read secret $JENKINS_NS/$JENKINS_SECRET (run 'gcloud auth login' and check the kube context)"
kubectl -n "$GERRIT_NS" get secret "$GERRIT_SECRET" >/dev/null \
  || die "cannot read secret $GERRIT_NS/$GERRIT_SECRET"

CURRENT_PASSWORD="$(read_secret_password "$JENKINS_NS" "$JENKINS_SECRET")"
[ -n "$CURRENT_PASSWORD" ] || die "current password in $JENKINS_SECRET is empty"

info "Checking current password against ${GERRIT_URL}"
status="$(gerrit_auth_status "$CURRENT_PASSWORD")"
[ "$status" = "200" ] || die "current password rejected by Gerrit (HTTP $status)"

if [ "$DRY_RUN" = "1" ]; then
  info "Dry run: access and current password OK, nothing changed"
  exit 0
fi

# head closes the pipe early; without this, pipefail turns tr's SIGPIPE into a failure.
NEW_PASSWORD="$(set +o pipefail; LC_ALL=C tr -dc 'A-Za-z0-9' </dev/urandom | head -c 32)"
[ "${#NEW_PASSWORD}" = "32" ] || die "failed to generate a new password"

# Keeps the new password recoverable if a step fails after Gerrit is changed.
RECOVERY_FILE="$(mktemp "${TMPDIR:-/tmp}/gerrit-http-password.XXXXXX")"
chmod 600 "$RECOVERY_FILE"
printf '%s\n' "$NEW_PASSWORD" >"$RECOVERY_FILE"

info "Setting new HTTP password on Gerrit"
status="$(curl -sS -o /dev/null -w '%{http_code}' -u "${USERNAME}:${CURRENT_PASSWORD}" \
  -X PUT -H 'Content-Type: application/json' \
  -d "$(jq -n --arg p "$NEW_PASSWORD" '{http_password: $p}')" \
  "${GERRIT_URL}/a/accounts/self/password.http")"
case "$status" in
  200|201) ;;
  *) rm -f "$RECOVERY_FILE"; die "Gerrit rejected the password change (HTTP $status); nothing was changed" ;;
esac

status="$(gerrit_auth_status "$NEW_PASSWORD")"
[ "$status" = "200" ] \
  || die "new password not accepted by Gerrit (HTTP $status); new password saved in $RECOVERY_FILE"

info "Updating Kubernetes secrets"
patch="{\"data\":{\"password\":\"$(b64 "$NEW_PASSWORD")\"}}"
kubectl -n "$JENKINS_NS" patch secret "$JENKINS_SECRET" --type merge -p "$patch" >/dev/null \
  || die "failed to patch $JENKINS_NS/$JENKINS_SECRET; new password saved in $RECOVERY_FILE"
kubectl -n "$GERRIT_NS" patch secret "$GERRIT_SECRET" --type merge -p "$patch" >/dev/null \
  || die "failed to patch $GERRIT_NS/$GERRIT_SECRET; new password saved in $RECOVERY_FILE"

[ "$(read_secret_password "$JENKINS_NS" "$JENKINS_SECRET")" = "$NEW_PASSWORD" ] \
  || die "$JENKINS_SECRET does not hold the new password; new password saved in $RECOVERY_FILE"
[ "$(read_secret_password "$GERRIT_NS" "$GERRIT_SECRET")" = "$NEW_PASSWORD" ] \
  || die "$GERRIT_SECRET does not hold the new password; new password saved in $RECOVERY_FILE"

# The rotation is already durable at this point (Gerrit updated and both
# secrets verified), so the MCP restart is best-effort: a failure here does not
# lose the new password, it only means the MCP pod must be restarted manually.
# Keep it non-fatal (and out of 'set -e') and warn with accurate guidance.
# The recovery file is kept until the final old-password check passes.
if kubectl -n "$GERRIT_NS" get deployment "$MCP_DEPLOYMENT" >/dev/null 2>&1; then
  info "Restarting $GERRIT_NS/$MCP_DEPLOYMENT to load the new password"
  if ! kubectl -n "$GERRIT_NS" rollout restart deployment "$MCP_DEPLOYMENT" >/dev/null; then
    echo "WARNING: could not restart $GERRIT_NS/$MCP_DEPLOYMENT. The password is already rotated and stored; restart it manually so it reloads the new password." >&2
  elif ! kubectl -n "$GERRIT_NS" rollout status deployment "$MCP_DEPLOYMENT" --timeout=180s; then
    echo "WARNING: $GERRIT_NS/$MCP_DEPLOYMENT did not become ready within 180s. The password is already rotated and stored; check 'kubectl -n $GERRIT_NS rollout status deployment $MCP_DEPLOYMENT'." >&2
  fi
else
  info "No $MCP_DEPLOYMENT deployment in $GERRIT_NS, skipping restart"
fi

old_status="$(gerrit_auth_status "$CURRENT_PASSWORD")"
[ "$old_status" = "401" ] \
  || die "old password still accepted (HTTP $old_status); new password saved in $RECOVERY_FILE"

rm -f "$RECOVERY_FILE"

info "Done. Old password now returns HTTP 401 as expected."
info "Jenkins reads $JENKINS_SECRET through the Kubernetes credentials provider; no restart needed."
