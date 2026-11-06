---
name: horizon-cli
description: >-
  Operate the Horizon SDV CLI (`horizon`) to authenticate, inspect the module workflow catalog,
  submit and monitor CI/CD workflows, stream workflow logs, browse GCS artifacts, and explore
  Horizon API endpoints. Use whenever the user asks to run, check, debug, or automate Horizon
  workflows or artifacts from the terminal.
---

# Horizon CLI Skill (`horizon`)

Guides the agent in using the `horizon` CLI to interact with the Horizon API for workflow execution, log streaming, catalog discovery, and artifact retrieval.

## Canonical Documentation & Self-Discovery (No Duplication)

To avoid maintaining duplicate flag tables, always rely on these authoritative sources:

1. **Runtime CLI Self-Discovery** (preferred — works in any workspace where `horizon` is on `PATH`):
   - Discover commands and flags dynamically via `horizon --help --no-banner` or `horizon <command> [subcommand] --help --no-banner`.
   - Discover live or embedded API endpoints via `horizon api list --no-banner` and `horizon api show <operationId | METHOD PATH> --no-banner`.
2. **Official GitHub Documentation & Source** (works even when the plugin is installed standalone outside the `horizon-sdv` repo):
   - Full user guide, installation, env vars, and command reference: [`docs/architecture/03_horizon-cli.md`](https://github.com/GoogleCloudPlatform/horizon-sdv/blob/main/docs/architecture/03_horizon-cli.md) (or local `docs/architecture/03_horizon-cli.md` if in a `horizon-sdv` checkout)
   - Go CLI source code: [`tools/horizon/`](https://github.com/GoogleCloudPlatform/horizon-sdv/tree/main/tools/horizon)

---

## Agent Execution Procedure

Follow [`rules/AGENTS.md`](../../rules/AGENTS.md) at all times (read-only commands run automatically; mutating commands require confirmation).

### Step 1: Verify CLI Installation & Version
1. Check whether `horizon` is installed:
   ```bash
   horizon version --output json
   ```
2. **If `horizon` is not found on `PATH`**:
   - Point the user to the **Download from the Developer Portal** instructions in [`docs/architecture/03_horizon-cli.md#installation`](https://github.com/GoogleCloudPlatform/horizon-sdv/blob/main/docs/architecture/03_horizon-cli.md#installation) (**Tools → Horizon CLI** in the Developer Portal), or if inside a `horizon-sdv` repo checkout with Go installed, offer to build it from [`tools/horizon`](https://github.com/GoogleCloudPlatform/horizon-sdv/tree/main/tools/horizon) (`cd tools/horizon && go install .`).

### Step 2: Verify Target Configuration & Authentication
1. Inspect the active configuration (merges `~/.config/horizon/config.yaml` and environment variables):
   ```bash
   horizon config get --merged
   ```
2. Verify authentication and API reachability:
   ```bash
   horizon auth whoami --no-banner
   ```
   - Expect `Horizon API catalog: OK`.
   - If the target domain is missing, ask the user for their Horizon domain (or pass `--domain <host>`).
   - If authentication fails or is expired and `horizon auth refresh` cannot renew it, follow Rule 2 in [`rules/AGENTS.md`](../../rules/AGENTS.md): ask the user to run `horizon auth login --device --write-config --domain <domain>` in their terminal (or verify `KEYCLOAK_CLIENT_SECRET` / `HORIZON_ACCESS_TOKEN` in headless CI).

### Step 3: Discover Catalog Templates & Parameters (Before Submitting)
Never guess module names, workflow template names, or parameter keys.
1. Query the live catalog:
   ```bash
   horizon catalog get --output json
   ```
2. Identify the target `module`, `templateName`, and its declared `parameters` (`name`, `default`, `description`).
3. If an expected template is missing, inform the user that the owning module must be enabled and `READY` under **Administration → Modules** in the Developer Portal.

### Step 4: Submit a Workflow (Confirmation Required)
1. **Confirm with the user first**: Present the target domain/API, `--module`, `--template`, and the exact `--params-json` payload.
2. After approval, submit the workflow:
   ```bash
   horizon workflow submit \
     --module <module> \
     --template <template> \
     --params-json '<json>' \
     --output json -q
   ```
   - Extract `workflowName` from the JSON output for subsequent tracking commands.
   - Only add `--wait` (and `--logs`) directly to `submit` if the user wants a blocking run and the workflow is expected to finish within the command timeout. Exit codes: `0` = Succeeded, `1` = Failed/Error/CLI error, `2` = Aborted.

### Step 5: Monitor Runs, Inspect Failures & Retrieve Artifacts
- **Find runs**:
  ```bash
  horizon workflow list --output json
  horizon workflow history --phase Failed,Error --limit 20
  ```
- **Inspect workflow status, archived log URIs (`gs://`), and artifact names**:
  ```bash
  horizon workflow show --output json <workflowName>
  ```
- **Stream or inspect logs**:
  ```bash
  horizon workflow logs --no-banner <workflowName>
  ```
- **Download artifacts or generate signed URLs**:
  - Take the artifact name from `outputArtifacts` in `horizon workflow show`.
  - Download to a file (`-o <path>`) or print a signed URL (`--generate-signed-url`):
    ```bash
    horizon workflow download-artifact -o ./<filename> -q <workflowName> <artifactName>
    horizon workflow download-artifact --generate-signed-url --output json <workflowName> <artifactName>
    ```
  - If multiple workflow nodes produced the same artifact name (HTTP 409), disambiguate with `--template-name` or `--node-id` as described in [`docs/architecture/03_horizon-cli.md#horizon-workflow-download-artifact`](https://github.com/GoogleCloudPlatform/horizon-sdv/blob/main/docs/architecture/03_horizon-cli.md#horizon-workflow-download-artifact).
- **Browse GCS buckets/objects exposed by Horizon API**:
  ```bash
  horizon artifacts buckets --json
  horizon artifacts objects <bucket> --prefix <prefix> --limit 50 --json
  ```
- **Abort or delete a workflow (Confirmation Required)**:
  Confirm the `workflowName` with the user before running `horizon workflow abort <workflowName>` or `horizon workflow delete <workflowName>`.
