---
name: mtkc-cli
description: >-
  Download, install, authenticate, and use the MediaTek Connect CLI (mtkc) and its built-in MCP server on Cloud Workstations.
  Prompts the user to sign in to MTK Connect and obtain an API key (or the get-cli install command),
  runs the official /mtk-connect/get-cli?platform=linux installer script with Basic Auth,
  registers mtkc mcp serve in Antigravity MCP config, and runs testbench and device commands.
---

# MTK Connect CLI (`mtkc`) & MCP Skill

This skill guides AI agents and developers through installing, authenticating, configuring the built-in MCP server, and running the **MTK Connect CLI (`mtkc`)** via the `/mtk-connect/get-cli` installer endpoint to interact with physical and virtual testbenches and Cuttlefish / AAOS devices on Horizon SDV.

---

## Onboarding & Setup Workflow Overview

```mermaid
flowchart LR
    A["1. Give User MTK Connect URL"] --> B["2. User Creates API Key / AUTH"]
    B --> C["3. Run /mtk-connect/get-cli Installer"]
    C --> D["4. Register mtkc MCP in Antigravity"]
    D --> E["5. Run Smoke Tests"]
```

> [!IMPORTANT]
> **Secured Installer Flow via `/mtk-connect/get-cli` (API Key Required Upfront)**:
> The `/mtk-connect/get-cli?platform=linux` endpoint serves the bootstrap installer script, which requires an authenticated `AUTH` token (`base64("<username>:<api-key>")`) to download the `mtkc` binary, unpack it to `/usr/local/bin` or `~/.local/bin`, log in (`mtkc auth login` + `mtkc auth rotate`), and configure MCP servers.
> Therefore, the agent **must first provide the MTK Connect Portal URL** to the user and ask them to sign in, create an API key in their profile, and paste their **MTK Connect Username & API Key** (or the `AUTH` install snippet from the UI) into the chat before running `/mtk-connect/get-cli`.

---

## Step-by-Step Installation, Auth & MCP Setup

### Step 1: Discover `HORIZON_DOMAIN` & Prompt User for MTK Connect API Key

1. Detect `HORIZON_DOMAIN` from the environment, `~/.config/horizon/config.yaml`, or `/usr/local/lib/horizon-mcp/mcp_registry_client.py`:
   ```bash
   HORIZON_DOMAIN="${HORIZON_DOMAIN:-$(python3 -c 'import sys; sys.path.insert(0, "/usr/local/lib/horizon-mcp"); import mcp_registry_client as m; print(m.HORIZON_DOMAIN)' 2>/dev/null)}"
   ```
2. Present the clickable MTK Connect URL to the user and pause for their credentials:
   > **To install and configure the MTK Connect CLI (`mtkc`) and its MCP server:**
   > 1. Open **MTK Connect** in your browser and sign in with your Horizon account:
   >    `https://<HORIZON_DOMAIN>/mtk-connect`
   > 2. Click your user profile icon (top right) -> **Profile / API Keys** -> **Create API Key** (or copy the CLI install command).
   > 3. Paste your **MTK Connect Username** and **API Key** (or the `AUTH=...` install command) back into this chat.

### Step 2: Install & Authenticate `mtkc` via `/mtk-connect/get-cli`

Once the user provides either:
- Their `MTKC_USERNAME` and `MTKC_API_KEY`, **or**
- A pre-encoded `AUTH` token / `get-cli` snippet from the MTK Connect UI,

run the `/mtk-connect/get-cli?platform=linux` installer script, which automatically detects the architecture, downloads the right binary using Basic Auth, installs `mtkc` into `/usr/local/bin` or `~/.local/bin`, updates `PATH`, runs `mtkc auth login` + `mtkc auth rotate`, and runs `mtkc mcp install --silent`:

```bash
# 1. Compute base64 AUTH token if username + API key were provided separately
if [ -z "${AUTH:-}" ]; then
  AUTH="$(printf '%s:%s' "${MTKC_USERNAME}" "${MTKC_API_KEY}" | base64 -w0)"
fi
export AUTH

# 2. Run the official MTK Connect get-cli installer script
curl -fsSL "https://${HORIZON_DOMAIN}/mtk-connect/get-cli?platform=linux" | AUTH="${AUTH}" bash

# 3. Ensure ~/.local/bin is on PATH in the current shell session
export PATH="$HOME/.local/bin:/usr/local/bin:$PATH"

# 4. Also copy to /usr/local/bin if installed to ~/.local/bin and passwordless sudo is available
if [ -x "$HOME/.local/bin/mtkc" ] && [ ! -x /usr/local/bin/mtkc ]; then
  sudo install -m 0755 "$HOME/.local/bin/mtkc" /usr/local/bin/mtkc 2>/dev/null || true
fi
```

### Step 3: Register the `mtkc` MCP Server in Antigravity

The `get-cli` script runs `mtkc mcp install --silent` (which configures Cursor, VS Code, Copilot, and Claude Desktop). To ensure Antigravity also loads the `mtkc` MCP server (`mtkc mcp serve`), register it in `~/.gemini/config/mcp_config.json` and `~/.gemini/antigravity/mcp_config.json`:

```bash
MTKC_BIN="$(command -v mtkc)"
python3 - "$MTKC_BIN" <<'PY'
import json
import sys
from pathlib import Path

mtkc_bin = sys.argv[1]
target_files = [
    Path.home() / ".gemini" / "config" / "mcp_config.json",
    Path.home() / ".gemini" / "antigravity" / "mcp_config.json",
]

for cfg_path in target_files:
    cfg_path.parent.mkdir(parents=True, exist_ok=True)
    data = {"mcpServers": {}}
    if cfg_path.exists():
        try:
            loaded = json.loads(cfg_path.read_text())
            if isinstance(loaded, dict):
                data = loaded
        except Exception:
            data = {"mcpServers": {}}
    mcp_servers = data.setdefault("mcpServers", {})
    mcp_servers["mtkc"] = {
        "command": mtkc_bin,
        "args": ["mcp", "serve"],
    }
    cfg_path.write_text(json.dumps(data, indent=2) + "\n")
    print(f"Configured mtkc MCP server in {cfg_path}")
PY
```

### Step 4: Run First Verification & Smoke Tests

After `get-cli` completes and the MCP server is registered, run these read-only verification commands and report the output to the user:

```bash
# 1. Check mtkc CLI & server version
mtkc version

# 2. Confirm active context and authentication status
mtkc auth status

# 3. List available testbenches
mtkc testbench list

# 4. List connected physical and virtual (Cuttlefish/AAOS) devices
mtkc device list
```

---

## `mtkc` CLI Command Reference

### 1. Authentication & Context Management

```bash
# Log in non-interactively and create/update context (handled automatically by get-cli)
mtkc auth login --server "https://<HORIZON_DOMAIN>/mtk-connect" --basic-auth "<BASE64_USER_KEY>"
mtkc auth login --server "https://<HORIZON_DOMAIN>/mtk-connect" --username "<USER>" --api-key "<KEY>"

# Check authentication status for current context
mtkc auth status

# Manage API keys or rotate current key
mtkc auth keys
mtkc auth rotate

# List or switch configuration contexts
mtkc context
```

### 2. Resource Inspection (Testbenches, Devices, Bookings, Artifacts)

All inspection commands support `-o table|json|yaml`:

```bash
# List and inspect testbenches
mtkc testbench list
mtkc testbench list -o json

# List and inspect devices (e.g. Cuttlefish / AAOS virtual devices)
mtkc device list
mtkc device list -o json

# Manage testbench and device bookings
mtkc booking

# List, upload, or download artifacts
mtkc artifact

# Inspect live OpenAPI schema
mtkc schema
```

### 3. Target Operations & Device Sessions

Interact with a specific `<testbench>/<device>` target directly from the CLI or agent:

```bash
# Run a command on a testbench or device (e.g. via ADB exec interface)
mtkc exec <testbench>/<device> -- <command>

# Stream logs from a device or testbench
mtkc log <testbench>/<device>

# Capture a screenshot from a device
mtkc screenshot <testbench>/<device>

# Copy files to/from or bidirectionally sync a directory with a testbench/device
mtkc cp <src> <dest>
mtkc sync <local-dir> <testbench>/<device>:<remote-dir>

# Open local device tunnels (e.g. local ADB forwarding)
mtkc tunnel
```

### 4. Built-in MCP Server (`mtkc mcp`)

```bash
# Start stdio MCP server (invoked automatically by Antigravity via mcp_config.json)
mtkc mcp serve

# Install/uninstall MCP config for VS Code, Cursor, Copilot, Claude
mtkc mcp install --silent
mtkc mcp uninstall
```
