# Horizon SDV Agent Skills & Workstation Onboarding Guide

This repository includes built-in AI agent plugins and skills for **Antigravity** and **Gemini CLI** to manage Horizon SDV workflows, build modules, interact with remote Cuttlefish/AAOS testbenches via MTK Connect, and debug Android Automotive OS (AAOS) and OpenBSW builds.

---

## Available Agent Plugins & Skills in This Repository

| Plugin Path | Skill Name | Skill File | Purpose |
| :--- | :--- | :--- | :--- |
| `.agents/plugins/horizon-ops` | **`horizon-cli`** | [`.agents/plugins/horizon-ops/skills/horizon-cli/SKILL.md`](./.agents/plugins/horizon-ops/skills/horizon-cli/SKILL.md) | Build `horizon` CLI from `./tools/horizon`, authenticate via Keycloak OAuth 2.0 Device Flow, and run catalog/workflow/artifact commands. |
| `.agents/plugins/horizon-ops` | **`mtkc-cli`** | [`.agents/plugins/horizon-ops/skills/mtkc-cli/SKILL.md`](./.agents/plugins/horizon-ops/skills/mtkc-cli/SKILL.md) | Prompt user for MTK Connect API key, download `mtkc` via Basic Auth, log in, register `mtkc mcp serve` in Antigravity, and run testbench/device commands. |
| `.agents/plugins/horizon-ops` | **`horizon-deploy`** | [`.agents/plugins/horizon-ops/skills/horizon-deploy/SKILL.md`](./.agents/plugins/horizon-ops/skills/horizon-deploy/SKILL.md) | Deploy, configure, and verify Horizon SDV infrastructure on GKE with Terraform and Argo CD. |
| `.agents/plugins/horizon-dev` | **`horizon-create-module`** | [`.agents/plugins/horizon-dev/skills/horizon-create-module/SKILL.md`](./.agents/plugins/horizon-dev/skills/horizon-create-module/SKILL.md) | Scaffold, implement, and register modular workloads under `gitops/modules/`. |
| `agent-plugins/aaos-platform` | **`aaos-build-debugger`** | [`agent-plugins/aaos-platform/skills/aaos-build-debugger/SKILL.md`](./agent-plugins/aaos-platform/skills/aaos-build-debugger/SKILL.md) | Diagnose and fix AAOS Soong/Ninja compilation and build failures. |
| `agent-plugins/aaos-platform` | **`aaos-cts-debugger`** | [`agent-plugins/aaos-platform/skills/aaos-cts-debugger/SKILL.md`](./agent-plugins/aaos-platform/skills/aaos-cts-debugger/SKILL.md) | Analyze Android Compatibility Test Suite (CTS) failures and logcat output. |
| `agent-plugins/openbsw-platform` | **`openbsw-build-debugger`** | [`agent-plugins/openbsw-platform/skills/openbsw-build-debugger/SKILL.md`](./agent-plugins/openbsw-platform/skills/openbsw-build-debugger/SKILL.md) | Diagnose and fix Eclipse OpenBSW CMake/C++20 and POSIX simulator build issues. |

---

## Cloud Workstation Onboarding Procedure (For AI Agents)

When asked by a user to onboard or configure their Cloud Workstation with the Horizon CLI (`horizon`), MTK Connect CLI (`mtkc`), and Horizon skills:

1. **Read the CLI Skills First**:
   - Read [`.agents/plugins/horizon-ops/skills/horizon-cli/SKILL.md`](./.agents/plugins/horizon-ops/skills/horizon-cli/SKILL.md)
   - Read [`.agents/plugins/horizon-ops/skills/mtkc-cli/SKILL.md`](./.agents/plugins/horizon-ops/skills/mtkc-cli/SKILL.md)

2. **Register Repository Plugins Globally in Antigravity (`~/.gemini/config/plugins.json`)**:
   Ensure all plugins in `.agents/plugins` and `agent-plugins` are enabled globally so their skills and rules remain available across any workspace on the Cloud Workstation:
   ```bash
   REPO_ROOT="$(pwd)"
   python3 - "$REPO_ROOT" <<'PY'
   import json
   import sys
   from pathlib import Path

   repo_root = Path(sys.argv[1]).resolve()
   plugins_file = Path.home() / ".gemini" / "config" / "plugins.json"
   plugins_file.parent.mkdir(parents=True, exist_ok=True)

   roots = [
       str(repo_root / ".agents" / "plugins"),
       str(repo_root / "agent-plugins"),
   ]
   enabled = ["horizon-ops", "horizon-dev", "aaos-platform", "openbsw-platform"]

   data = {"pluginRoots": [], "enabledPlugins": {}}
   if plugins_file.exists():
       try:
           loaded = json.loads(plugins_file.read_text())
           if isinstance(loaded, dict):
               data.update(loaded)
       except Exception:
           pass

   existing_roots = data.setdefault("pluginRoots", [])
   for r in roots:
       if r not in existing_roots:
           existing_roots.append(r)

   enabled_map = data.setdefault("enabledPlugins", {})
   for p in enabled:
       enabled_map[p] = True

   plugins_file.write_text(json.dumps(data, indent=2) + "\n")
   print(f"Registered plugins in {plugins_file}")
   PY
   ```

3. **Build `horizon` CLI from `./tools/horizon` & Start Device Login**:
   - Follow **`horizon-cli`**: detect `HORIZON_DOMAIN` (from environment or `/usr/local/lib/horizon-mcp/mcp_registry_client.py`), build `horizon` from `./tools/horizon`, and launch `horizon auth login --device --no-browser --domain "$HORIZON_DOMAIN" --write-config` in the background to capture the Keycloak device verification URL.

4. **Prompt User Once for Browser Sign-In & MTK Connect API Key**:
   - Follow **`mtkc-cli`** (Option B - secured download): in the same message where you share the clickable `horizon` Keycloak device verification URL, also share the clickable MTK Connect URL (`https://<HORIZON_DOMAIN>/mtk-connect`) and instruct the user to sign in, create an API key under their profile, and paste their **MTK Connect Username** and **API Key** into the chat.

5. **Complete `mtkc` Download, Auth, MCP Registration & Smoke Tests**:
   - Once the user completes the browser sign-in and replies with their MTK Connect username and API key:
     - Verify `horizon` CLI (`horizon version`, `horizon auth whoami`, `horizon catalog get`, `horizon workflow list`).
     - Download `mtkc` using Basic Auth (`-u "${MTKC_USERNAME}:${MTKC_API_KEY}"`) from `https://${HORIZON_DOMAIN}/mtk-connect/api/v1/installer?component=cli&platform=linux&arch=amd64&ext=tgz`.
     - Run `mtkc auth login --server "https://${HORIZON_DOMAIN}/mtk-connect" --username "${MTKC_USERNAME}" --api-key "${MTKC_API_KEY}"`.
     - Register `mtkc mcp serve` in `~/.gemini/config/mcp_config.json` and `~/.gemini/antigravity/mcp_config.json`.
     - Verify `mtkc` CLI (`mtkc version`, `mtkc auth status`, `mtkc testbench list`, `mtkc device list`).
