<!-- Copyright (c) 2026 Accenture, All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

        http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License. -->

## MCP Setup and Usage Guide

This guide provides instructions for setting up MCP servers in MCP Gateway Registry and using them with Gemini CLI, Gemini Code Assist in IDEs, and Antigravity (Agent and CLI) on Cloud Workstations.

## Table of Contents

- [Prerequisites](#prerequisites)
- [MCP client → tool mapping](#mcp-client--tool-mapping)
- [Setup Steps](#setup-steps)
  - [Step 1: Enable MCP Servers in MCP Gateway Registry](#step-1-enable-mcp-servers-in-mcp-gateway-registry)
  - [Step 2: Add New MCP Server (Optional)](#step-2-add-new-mcp-server-optional)
  - [Step 3: Configure Your Local Environment](#step-3-configure-your-local-environment)
  - [Step 4: Use MCP Servers with Gemini-CLI and Gemini Code Assist](#step-4-use-mcp-servers-with-gemini-cli-and-gemini-code-assist)
    - [Gemini-CLI in Terminal](#gemini-cli-in-terminal)
    - [Gemini Code Assist in Horizon Code OSS (VS Code)](#gemini-code-assist-in-horizon-code-oss-vs-code)
    - [Gemini Code Assist in Android Studio and Android Studio for Platform](#gemini-code-assist-in-android-studio-and-android-studio-for-platform)
  - [Step 5: Use MCP Servers with Antigravity](#step-5-use-mcp-servers-with-antigravity)
- [Known Issues and Workarounds](#known-issues-and-workarounds)
  - [Server registration](#server-registration)
  - [Server enablement](#server-enablement)
  - [MCP configuration caching across Gemini clients](#mcp-configuration-caching-across-gemini-clients)
  - [Android Studio / ASfP: mcp.json deleted after applying empty config](#android-studio--asfp-mcpjson-deleted-after-applying-empty-config)
- [`gemini-mcp-agent` Documentation](#gemini-mcp-agent-documentation)
- [`antigravity-mcp-agent` Documentation](#antigravity-mcp-agent-documentation)

## Prerequisites

The following prerequisites must be met before proceeding with the setup:

- Access to MCP Gateway Registry with appropriate permissions.
  - User must be added to either of Keycloak groups:
    - `administrators`: Admins can register new or edit existing MCP servers and agents. They have full access to all MCP servers, agents and this app’s API.
    - `viewers`: Users can only view existing registered MCP servers and agents but have full use access to all MCP servers and agents; and read-only access to this app’s API.
- Workstation Images with Gemini-CLI and Gemini Code Assist installed (for Gemini clients).
- For Antigravity MCP: a workstation image that includes Antigravity (Agent and/or `agy`) and `antigravity-mcp-agent` (see [antigravity.md](../workloads/common/agentic-ai/antigravity.md)).

## MCP client → tool mapping

Use the matching agent for the client you want to configure. Agents share Keycloak / MCP Gateway Registry authentication via a common registry client (`mcp_registry_client`); you only need one successful login for both tools on the same workstation home directory.


| Tool                    | Configures                                                                                      | Client config path(s)                                                                                                                                     |
| ----------------------- | ----------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `gemini-mcp-agent`      | Gemini CLI; Gemini Code Assist in Horizon Code OSS; Gemini Code Assist in Android Studio & ASfP | `~/.gemini/settings.json` (Gemini CLI / Code OSS); `mcp.json` under `~/.config/Google/AndroidStudio*/` (Android Studio / ASfP)                            |
| `antigravity-mcp-agent` | Antigravity 2.0 Agent (desktop) and Antigravity CLI (`agy`)                                     | Default: `~/.gemini/config/mcp_config.json`. Override with `ANTIGRAVITY_MCP_FILE_PATH` (e.g. `~/.gemini/antigravity-cli/mcp_config.json` for CLI layouts) |


Workstation images install a shared MCP core (`mcp_registry_client` under `common-utils/mcp-agents/`) plus thin front-ends `gemini-mcp-agent` and `antigravity-mcp-agent`.

## Setup Steps

### Step 1: Enable MCP Servers in MCP Gateway Registry

1. Log in to MCP Gateway Registry at `https://mcp.<SUB_DOMAIN>.<HORIZON_DOMAIN>/` as an admin using your Keycloak credentials.
2. Navigate to the "MCP Servers" section.
3. In order to access any MCP server, including the pre-registered `gerrit-mcp-server`, make sure that it is ENABLED on the app (bottom right toggle button in server card). By default, newly registered MCP servers are disabled.

### Step 2: Add New MCP Server (Optional)

Steps:

1. Click on the "Register New Server" button.
2. Fill in the details.
3. Make sure you enter the path as `/my-mcp-server/` with trailing slashes on both ends. See [Known Issue](#server-registration).
4. New server should now be visible.
5. Make sure to ENABLE it before use (bottom right toggle button in server card). See [Known Issue](#server-enablement).

### Step 3: Configure Your Local Environment

Open your Cloud Workstation, then follow the steps below.

Optional `.env` overrides below are shared by `gemini-mcp-agent` and `antigravity-mcp-agent`. Registry enablement is [Steps 1–2](#step-1-enable-mcp-servers-in-mcp-gateway-registry); Gemini client setup is [Step 4](#step-4-use-mcp-servers-with-gemini-cli-and-gemini-code-assist); Antigravity is [Step 5](#step-5-use-mcp-servers-with-antigravity).

Steps:

1. Open a terminal.
2. (Optional) If you need to override default settings, create a `.env` file in your current directory (or set `ENV_FILE_PATH` to point to an existing one):

   ```
   HORIZON_DOMAIN=your-horizon-domain
   KEYCLOAK_URL=your-keycloak-url
   REALM=your-realm
   CLIENT_ID=your-client-id
   MCP_REGISTRY_URL=your-mcp-registry-url

   # ONLY set these if you are using non-standard installation paths:
   # GEMINI_CONFIG_HOME=/custom/path/to/.gemini/ (Directory)
   # ANDROID_STUDIO_MCP_FILE_PATH=~/.config/Google/AndroidStudio<VERSION>/mcp.json
   # ANTIGRAVITY_MCP_FILE_PATH=~/.gemini/antigravity-cli/mcp_config.json
   ```

   **Note on Variable Priority:** The tool looks for configuration in this order:

   - Terminal Environment Variables (Active exports)
   - `.env` file via `ENV_FILE_PATH` environment variable (if set)
   - `.env` file in your Current Working Directory
   - `~/.gemini/.env` (Global Fallback)

Next: run `gemini-mcp-agent` ([Step 4](#step-4-use-mcp-servers-with-gemini-cli-and-gemini-code-assist)) and/or `antigravity-mcp-agent` ([Step 5](#step-5-use-mcp-servers-with-antigravity)).

### Step 4: Use MCP Servers with Gemini-CLI and Gemini Code Assist

This step uses the `gemini-mcp-agent` command-line tool to handle authentication and configuration for Gemini clients. For detailed Gemini usage and all available options, see the [`gemini-mcp-agent` Documentation](#gemini-mcp-agent-documentation).

1. Run `gemini-mcp-agent`
2. Follow the prompts to authenticate.
3. After the initial setup, you will be prompted to start a background sync. Prefer a single background sync (`gemini-mcp-agent` or `antigravity-mcp-agent`), not both — both agents share one refresh token, so only one daemon needs to run.
   - Enter `y` or `yes` to start the background sync now.
   - If you choose not to, you can always start it later by running:

     ```bash
     gemini-mcp-agent --daemon-start
     ```

> **Note**: If your SSO session expires (500 Internal Server Error), or `gemini-mcp-agent` is restarted, run `gemini-mcp-agent` again to re-authenticate.  
> For Android Studio/ASfP, refresh IDE MCP settings: disable MCP servers (**Apply**), re-enable MCP servers (**Apply**), then click **OK**. Keep these IDE refresh steps in the Gemini flow only; Antigravity does not use `mcp.json`.

Once the `gemini-mcp-agent` is running, your tools are configured to connect to the MCP servers.
For registry-managed MCP servers, the agent writes command-based MCP entries that launch `mcp-client-bridge` for all supported Gemini clients (Gemini CLI, Code OSS, Android Studio, and ASfP). This ensures each request uses fresh runtime authentication instead of cached config tokens.

##### Gemini-CLI in Terminal

1. Open a terminal.
2. Before running Gemini CLI in this terminal session, set your GCP project:

   ```bash
   export GOOGLE_CLOUD_PROJECT="<GCP-PROJECT-NUMBER>"
   ```

   You can get this project number from your admin. It is also available in Jenkins pipeline description: `Cloud Workstations > Cluster Admin Operations > Create New Cluster` as `PROJECT`.
3. Run `gemini`
4. Log in to Gemini
5. Run `/mcp`
6. You should see the registered MCP servers along with the pre-registered `gerrit-mcp-server` with status Connected.
7. Now you can run any Gerrit-related query and Gemini will query the MCP server for answers.

##### Gemini Code Assist in Horizon Code OSS (VS Code)

1. Open Horizon Code OSS cloud workstation.
2. From the left sidebar, click on the Gemini icon to open Gemini Code Assist.
3. Complete initial setup > Select `Gemini for Businesses`.
4. Select `Google Cloud Project` and complete further setup
5. In chat window, enable "Agent" mode by clicking on the "Agent" button at the bottom right of the chat window.
6. Run `/mcp`
7. You should see the pre-registered `gerrit-mcp-server` with status Connected.
8. Now you can run any Gerrit-related query and Gemini will query the MCP server for answers.

##### Gemini Code Assist in Android Studio and Android Studio for Platform

1. Open Android Studio or Android Studio for Platform cloud workstation.
2. Open IDE.
3. In Settings > Google Accounts, add and log in with your enterprise account.
4. Open Gemini Code Assist from sidebar.
5. Complete initial setup:
   - Select `Gemini for Businesses`
   - Select the correct `Google Cloud Project`
6. If `gemini-mcp-agent` is not already running, run it from terminal and complete authentication.
7. In IDE settings, open MCP Servers (`Tools > AI > MCP Servers`) and enable MCP servers. Click **Apply** and then **OK**.

   ![MCP Servers settings in Android Studio](../images/cloud-ws-android-studio-ide-enable-mcp-servers.png)

8. In chat window, enable "Agent" mode by clicking the "Agent" button at the bottom right.
9. Run `/mcp`.
10. You should see the pre-registered `gerrit-mcp-server` with status Connected.
11. Now you can run any Gerrit-related query and Gemini will query the MCP server for answers.

### Step 5: Use MCP Servers with Antigravity

This step configures MCP for **Antigravity 2.0 Agent** (desktop) and **Antigravity CLI** (`agy`). Both products share the Antigravity MCP config written by `antigravity-mcp-agent`.

Antigravity MCP setup reuses the same MCP Gateway Registry enablement as Gemini ([Steps 1–2](#step-1-enable-mcp-servers-in-mcp-gateway-registry)). Gemini Code Assist is unchanged; do **not** use `antigravity-mcp-agent` for Gemini clients.

A **one-time** `antigravity-mcp-agent` run is required to generate the Antigravity MCP config (default: `~/.gemini/config/mcp_config.json`). After that, re-authenticate only when SSO expires — the same rule as for `gemini-mcp-agent`.

1. Complete [Steps 1–2](#step-1-enable-mcp-servers-in-mcp-gateway-registry) (shared registry setup). Optional `.env` overrides are the same as in [Step 3](#step-3-configure-your-local-environment).
2. Open a terminal.
3. Run `antigravity-mcp-agent` once to generate the Antigravity MCP config and complete Keycloak device-flow login if no valid shared token exists.
   - If you already authenticated with `gemini-mcp-agent` on the same home directory, the shared token cache is reused and a second onboarding is usually not required.
4. Prefer a single background sync (`gemini-mcp-agent` or `antigravity-mcp-agent`), not both. Both agents can `--daemon-start` and share one refresh token, so only one background sync should run at a time. If needed, start background sync later:

   ```bash
   antigravity-mcp-agent --daemon-start
   ```

5. Confirm config was written (default path):

   ```bash
   cat ~/.gemini/config/mcp_config.json
   ```

6. Use Antigravity:

   ```bash
   agy --version
   agy
   ```

   On **Horizon Android Studio** and **Horizon Android Studio for Platform (ASfP)**, the **Antigravity 2.0 Agent** (desktop) is also installed. Launch it from the App menu (**Antigravity 2.0**). The Agent uses the same MCP config as the CLI (`agy`).

**When to re-authenticate:** Re-run `antigravity-mcp-agent` only when SSO expires (for example a 500 Internal Server Error from the registry) — the same as Gemini. You do not need to re-run it on every workstation login while the SSO session remains valid; optional daemon sync refreshes the token/server list until SSO expiry.

**Config path and `serverUrl`:** Antigravity Agent and CLI read MCP servers from `~/.gemini/config/mcp_config.json` by default (override with `ANTIGRAVITY_MCP_FILE_PATH`). For **static remote** servers you add yourself, Antigravity expects a `serverUrl` field. For **registry-managed** servers, `antigravity-mcp-agent` writes **command-mode bridge** entries (not a persisted `serverUrl` token) so each request injects a fresh JWT via the shared bridge — same pattern as Gemini registry entries.

## Known Issues and Workarounds
1. #### Server registration
   While registering new MCP server, make sure to enter the path with trailing slashes on both ends, e.g. `/my-mcp-server/`. Otherwise, Gemini-CLI, Gemini Code Assist and Antigravity will not be able to connect to the server.

2. #### Server enablement
   Server is disabled by default on new registration, including the pre-registered gerrit-mcp-server. Make sure to ENABLE it before use.

3. #### MCP configuration caching across Gemini clients
   Gemini clients may cache MCP configuration for the active session (`settings.json` for Gemini-CLI/Code OSS, `mcp.json` for Android Studio/ASfP).
   - To avoid stale-token failures, `gemini-mcp-agent` configures registry-managed MCP servers to use `mcp-client-bridge` across all supported Gemini clients.
   - The bridge reads fresh auth from `~/.gemini/mcp-gateway-registry-token.json` for every forwarded request.
   - This keeps authentication current even if the client keeps an older cached config in memory.
   - For Gemini-CLI terminal sessions, set `GOOGLE_CLOUD_PROJECT` before running `gemini`:
     ```bash
     export GOOGLE_CLOUD_PROJECT="<GCP-PROJECT-NUMBER>"
     ```
   > Client refresh notes:
   > - Gemini-CLI / Code OSS: If the client was already running before `gemini-mcp-agent` setup/sync, restart the client session to load the latest MCP server entries.
   > - Android Studio / ASfP (mandatory when SSO expires, `gemini-mcp-agent` restarts, or remote MCP servers are changed):
   >   1. Open Android Studio Settings > MCP Servers (`Tools > AI > MCP Servers`)
   >   2. Disable MCP servers and click **Apply**
   >   3. Re-enable MCP servers and click **Apply**
   >   4. Click **OK**

4. #### Android Studio / ASfP: mcp.json deleted after applying empty config
   If MCP servers are enabled and you clear the MCP config text box, then click **Apply**, IDE can remove `mcp.json` from filesystem.
   Use these recovery steps:
   1. Open Settings > MCP Servers and disable MCP servers.
   2. Close the Settings window completely. Do not keep it open.
   3. In terminal, stop background sync:
      ```bash
      gemini-mcp-agent --daemon-stop
      ```
   4. Run setup again:
      ```bash
      gemini-mcp-agent
      ```
   5. Go back to Settings > MCP Servers, enable MCP servers, then click **Apply** and **OK**.

## `gemini-mcp-agent` Documentation

The `gemini-mcp-agent` is a command-line tool that simplifies using MCP (Model Context Protocol) servers by handling authentication and configuration for Gemini CLI and Gemini Code Assist in IDEs.

**Source:** thin front-end [`common-utils/mcp-agents/gemini_mcp_agent.py`](/workloads/cloud-workstations/pipelines/workstation-images/common-utils/mcp-agents/gemini_mcp_agent.py) over shared [`mcp_registry_client.py`](/workloads/cloud-workstations/pipelines/workstation-images/common-utils/mcp-agents/mcp_registry_client.py), installed as `/usr/local/bin/gemini-mcp-agent`.

### What it Does

1. **Authenticates**: Connects to MCP Gateway Registry using browser-based login (Keycloak Device Flow).
2. **Fetches Config**: Retrieves the latest list of available MCP servers and stores the latest auth token in `~/.gemini/mcp-gateway-registry-token.json`.
3. **Updates Client Config Files**: Updates MCP configuration for all supported Gemini clients:
  - `~/.gemini/settings.json` for Gemini CLI and Horizon Code OSS
  - `mcp.json` for Android Studio and ASfP
4. **Uses Bridge Mode for Registry Servers**: For registry-managed servers, writes command-based entries that run `mcp-client-bridge` instead of storing short-lived tokens directly in client config files.
5. **Injects Fresh Token at Runtime**: In bridge mode, forwards `JSON-RPC` requests over HTTPS and injects the latest token from `~/.gemini/mcp-gateway-registry-token.json` for each request.
6. **Keeps Sessions Active**: Supports foreground and background sync to refresh sessions automatically until you stop it or SSO expires.
7. **Manages Registry Server Entries**: Provides `--prune` and `--force` options to clean up or replace tool-managed server entries.
8. **Gemini CLI Environment Note**: The agent does not set shell environment variables. For Gemini CLI terminal sessions, you still need:

   ```bash
   export GOOGLE_CLOUD_PROJECT="<GCP-PROJECT-NUMBER>"
   ```

This agent does **not** configure Antigravity Agent or CLI. Use [`antigravity-mcp-agent`](#antigravity-mcp-agent-documentation) for that.

### Command-Line Options


| Option                | Description                                                                                                                                                                                                                                  |
| --------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `gemini-mcp-agent`    | Default command. Runs a one-time sync. Uses saved session if available; otherwise starts interactive login.                                                                                                                                  |
| `--watch`             | Runs continuous sync in the foreground. Keeps the current terminal occupied until `Ctrl+C`.                                                                                                                                                  |
| `--daemon-start`      | Starts continuous sync as a background process.                                                                                                                                                                                              |
| `--daemon-stop`       | Stops the background sync process.                                                                                                                                                                                                           |
| `--daemon-status`     | Checks if the background sync process is running.                                                                                                                                                                                            |
| `--prune`             | During sync, removes server entries managed by this tool that no longer exist in MCP Gateway Registry.                                                                                                                                       |
| `--force`             | During sync, replaces configured MCP servers with the MCP Gateway Registry list. **Warning**: This removes manually added servers.                                                                                                           |
| `--mcp-client-bridge` | (Internal use) Used by generated config entries for registry-managed servers to proxy MCP traffic and inject fresh auth at runtime. See [MCP configuration caching across Gemini clients](#mcp-configuration-caching-across-gemini-clients). |
| `--mcp-server`        | (Internal use) Specifies the target MCP server for `--mcp-client-bridge`.                                                                                                                                                                    |

## `antigravity-mcp-agent` Documentation

`antigravity-mcp-agent` configures MCP for **Antigravity 2.0 Agent** (desktop) and **Antigravity CLI** (`agy`). It does not update Gemini CLI `settings.json`, Code OSS, or Android Studio / ASfP `mcp.json`. Gemini Code Assist behaviour is unchanged.

**Source:** [`common-utils/mcp-agents/antigravity_mcp_agent.py`](/workloads/cloud-workstations/pipelines/workstation-images/common-utils/mcp-agents/antigravity_mcp_agent.py) over shared [`mcp_registry_client.py`](/workloads/cloud-workstations/pipelines/workstation-images/common-utils/mcp-agents/mcp_registry_client.py), installed as `/usr/local/bin/antigravity-mcp-agent`.

### What it Does

1. **Authenticates**: Reuses the shared MCP Gateway Registry session (Keycloak Device Flow); prompts only if no valid token exists in `~/.gemini/mcp-gateway-registry-token.json`.
2. **Fetches Config**: Retrieves the latest list of available MCP servers from the registry.
3. **Updates Antigravity Config**: Writes `~/.gemini/config/mcp_config.json` (or `ANTIGRAVITY_MCP_FILE_PATH`) for both Antigravity Agent and CLI.
4. **Registry vs `serverUrl`**: Registry-managed servers are written as command-mode bridge entries (fresh JWT per request). Static remote servers that you add manually use Antigravity’s `serverUrl` field in the same config file.
5. **Keeps Sessions Active**: Same daemon / watch options as the Gemini agent (`--daemon-start`, `--watch`, and so on).

### Quick start

```bash
# Shared registry setup (enable servers) is described in Setup Steps above.
# One-time run required to generate Antigravity MCP config.
antigravity-mcp-agent
antigravity-mcp-agent --daemon-start   # recommended after first login
agy   # CLI; or launch Antigravity 2.0 from the desktop app menu on AS / ASfP
```

### When to re-authenticate

Re-run `antigravity-mcp-agent` **only when SSO expires** (for example 500 Internal Server Error from the registry) — the same as `gemini-mcp-agent`. A one-time run is enough to generate config; do not re-run on every login while SSO remains valid.

### Command-Line Options

Options match the shared agent core (same flags as [`gemini-mcp-agent`](#command-line-options)): default one-shot sync, `--watch`, `--daemon-start`, `--daemon-stop`, `--daemon-status`, `--prune`, `--force`, plus internal `--mcp-client-bridge` / `--mcp-server` for bridge entries.

### Related docs

- Antigravity install matrix and build parameters: [antigravity.md](../workloads/common/agentic-ai/antigravity.md)
