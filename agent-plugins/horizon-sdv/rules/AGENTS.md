# Horizon SDV Agent Plugin Rules

The following guardrails apply to all skills within the `horizon-sdv` plugin.

## 1. Read-Only vs. Mutating Command Approval (`horizon` CLI)
- **Read-Only Commands (Allowed Automatically)**: You may run read-only discovery and inspection commands without asking first:
  - `horizon version`, `horizon config get [--merged]`, `horizon auth whoami`, `horizon history`
  - `horizon catalog get`
  - `horizon workflow list`, `horizon workflow running`, `horizon workflow history`, `horizon workflow show`, `horizon workflow get`, `horizon workflow logs`
  - `horizon artifacts buckets`, `horizon artifacts objects`
  - `horizon api list`, `horizon api paths`, `horizon api show`
- **Mutating & Destructive Commands (Confirmation Required)**: You MUST obtain explicit user confirmation before running any command that mutates cluster state, triggers jobs, downloads large files unless requested, or overwrites local configuration:
  - `horizon workflow submit` (always show the target domain/API, `module`, `template`, and JSON parameters before submitting)
  - `horizon workflow abort` and `horizon workflow delete`
  - `horizon config set` and `horizon config init --force`
  - `horizon auth logout` and `horizon api refresh`

## 2. Authentication & Credential Hygiene
- **Never Expose Secrets**: Never print, log, or echo `HORIZON_ACCESS_TOKEN`, `KEYCLOAK_CLIENT_SECRET`, or the contents of `~/.config/horizon/token.json`.
- **Interactive Login Hand-off**: If `horizon auth whoami` indicates the user is not authenticated and neither `HORIZON_ACCESS_TOKEN` nor `KEYCLOAK_CLIENT_SECRET` is set, do NOT run an interactive browser login silently. Instruct the user to run `horizon auth login --device --write-config --domain <HORIZON_DOMAIN>` in their terminal and wait for them to confirm completion.

## 3. Agent Output & Context Guardrails
- **Machine-Readable Invocation**: Prefer `--no-banner` (`-b`) and `--json` (`-j`) or `--output json` when running `horizon` commands so output is structured and free of ASCII banners.
- **Bounded Log & Artifact Reads**: Avoid streaming unbounded multi-hour workflow logs or piping binary artifacts (`--stdout`) directly into the conversation context. Save downloaded artifacts to disk (`-o <path>`) or generate signed URLs (`--generate-signed-url`).
