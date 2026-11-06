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

# Antigravity on Horizon Cloud Workstation images

Horizon integrates two **separate** Google Antigravity products into workstation images:

| Product | Version line | Purpose | Images |
|---------|--------------|---------|--------|
| **Antigravity 2.0 Agent** (desktop) | 2.x | GUI agent via GNOME app menu | `horizon-gnome` base (inherited by `horizon-android-studio` and `horizon-asfp`) |
| **Antigravity CLI (`agy`)** | 1.x | Terminal agent | `horizon-code-oss` (required) |

Antigravity IDE is **not** installed. Gemini Code Assist on workstation images is **unchanged** by Antigravity tooling.

For MCP Gateway Registry setup with Antigravity, use `antigravity-mcp-agent` (Antigravity Agent and CLI). Gemini clients continue to use `gemini-mcp-agent`. See [MCP Setup and Usage Guide](../../../guides/mcp_setup.md#mcp-client--tool-mapping) and the [image matrix](#image-matrix) below.

**Architecture note:** the Agent is a single, shared build step baked into the `horizon-gnome` base image (behind `INSTALL_ANTIGRAVITY_AGENT`, default `true`) so every GNOME thin-child inherits it without re-installing. `horizon-android-studio` and `horizon-asfp` are thin children of `horizon-gnome` and inherit the Agent; neither child installs it itself. `horizon-code-oss` does not derive from `horizon-gnome` and is unaffected - it only installs the CLI.

Install scripts live under:

`workloads/cloud-workstations/pipelines/workstation-images/common-utils/antigravity/`

Build-time install scripts for **Antigravity 2.0 Agent** (desktop) and **Antigravity CLI (`agy`)** on Horizon workstation images.

| Script | Used on | Installs |
|---|---|---|
| `install-antigravity-agent.sh` | `horizon-gnome` base image (inherited by Android Studio and ASfP) | Antigravity **2.0 Agent** (not IDE) |
| `install-antigravity-cli.sh` | on Code OSS image | `agy` CLI (pinned GitHub release tarball) |

Scripts run at **image build time** (root). **OAuth / first sign-in** happen at **runtime** in the browser (Chrome is in GNOME images).

---


## Image matrix

| Jenkins image job | Agent (desktop) | CLI (`agy`) | Launch desktop |
|-------------------|-----------------|-------------|----------------|
| Horizon GNOME (base) | Yes (source of the install) | Yes (bundled with the agent) | N/A (base image only) |
| Horizon Android Studio | Yes (inherited from `horizon-gnome`) | Yes (bundled with the agent) | GNOME menu → Antigravity 2.0 |
| Horizon Android Studio for Platform (ASfP) | Yes (inherited from `horizon-gnome`) | Yes (bundled with the agent) | GNOME menu → Antigravity 2.0 |
| Horizon Code OSS | No | Yes | N/A (terminal: `agy`) |

**Note:** `agy` still appears on GNOME workstations (e.g. `/usr/bin/agy`) from Google’s CLI installer. It is installed by default. That is independent of `install-antigravity-cli.sh`, which installs to `/usr/local/bin/agy`.

---

## Build pipelines

Image jobs under `Cloud Workstations → Workstation Images` copy `common-utils/antigravity/` at build time. Set versions from each job’s **Build with Parameters**:

- **Antigravity CLI (`agy`)** — **Horizon Code OSS**. Provide `ANTIGRAVITY_CLI_VERSION`, `ANTIGRAVITY_CLI_TARBALL_URL`, and `ANTIGRAVITY_CLI_SHA256`. Release assets and SHA-256 digests are listed on the [Antigravity CLI releases](https://github.com/google-antigravity/antigravity-cli/releases) page.
- **Antigravity 2.0 Agent** (desktop) — **Horizon GNOME** (inherited by Android Studio and ASfP). Provide `ANTIGRAVITY_2_VERSION`, `ANTIGRAVITY_2_TARBALL_URL`, and `ANTIGRAVITY_2_TARBALL_SHA256`.

| Parameter | Horizon GNOME (base) | Code OSS |
|-----------|-----------------------|----------|
| `INSTALL_ANTIGRAVITY_AGENT` | Required, default `true` | — |
| `ANTIGRAVITY_2_VERSION` | Required (e.g. `2.2.1`) | — |
| `ANTIGRAVITY_2_TARBALL_URL` | Linux x64 tarball URL | — |
| `ANTIGRAVITY_2_TARBALL_SHA256` | SHA-256 of that tarball (lowercase hex) | — |
| `ANTIGRAVITY_CLI_VERSION` | — | Required (e.g. `1.1.0`) |
| `ANTIGRAVITY_CLI_TARBALL_URL` | — | GitHub release asset URL (e.g. `agy_cli_linux_x64.tar.gz`) |
| `ANTIGRAVITY_CLI_SHA256` | — | SHA-256 of that tarball (lowercase hex) |

`horizon-android-studio` and `horizon-asfp` do not have their own `ANTIGRAVITY_2_*` parameters. Both consume the Agent baked into the `horizon-gnome` image they reference via `BASE_IMAGE`. To upgrade the Agent, rebuild and republish `horizon-gnome` with the new `ANTIGRAVITY_2_*` parameters first, then point each child's `BASE_IMAGE` at the new base tag or digest.

Defaults are set in each image `Dockerfile` and matching Jenkins `job.groovy`.

When changing either the CLI or the Agent version, you **must** supply all three corresponding values (version, tarball URL, and SHA-256). Updating only the version (or omitting URL/SHA) causes Antigravity installation to fail at image build time.

---

## Runtime (workstation)

### Desktop Agent (AS / ASfP)

1. Open the workstation in the browser (Guacamole over RDP).
2. Launch **Antigravity 2.0** from the app menu (no autostart).
3. Complete Google sign-in in Chrome when prompted.

### CLI (Code OSS)

```bash
agy --version
agy
```

## Session persistence
Login tokens live under `/home/user/…`.  
Enable persistent disk on the workstation config (PD_REQUIRED=true, mount /home) so OAuth survives stop/start.  
Without PD, expect sign-in again after restart.

## Workstation config notes
* Use lowercase config names (GCP requirement).
* Android Studio / ASfP images are large; use `HOST_BOOT_DISK_SIZE ≥ 31 GB`. "Start Workstation" fails at any value less than 31 GB.

## Install scripts
### `install-antigravity-agent.sh`
Desktop 2.0 Agent with a GPU-safe X11 wrapper, compatible with RDP/XWayland on the GNOME-based images (Android Studio and ASfP).
```bash
ANTIGRAVITY_2_INSTALL_MODE=tarball \
ANTIGRAVITY_2_VERSION=2.2.1 \
ANTIGRAVITY_2_TARBALL_URL=<url> \
ANTIGRAVITY_2_TARBALL_SHA256=<sha256> \
./install-antigravity-agent.sh
```
Creates:

| Path  | Role  |
|------|----------|
|/opt/antigravity/ | Real binary (tarball mode) |
|/usr/local/bin/antigravity | GPU-safe X11 wrapper |
|/usr/share/applications/antigravity.desktop | App menu entry |

Does not create `/etc/xdg/autostart/antigravity.desktop` entry.

### `install-antigravity-cli.sh`

Installs via pinned CLI install from a GitHub release tarball (not the floating install.sh).
```bash
ANTIGRAVITY_CLI_VERSION=1.1.0 \
ANTIGRAVITY_CLI_TARBALL_URL=<url> \
ANTIGRAVITY_CLI_SHA256=<sha256> \
./install-antigravity-cli.sh
```
Installs to `/usr/local/bin/agy` after SHA-256 verification.  
Example release: [antigravity-cli 1.1.0](https://github.com/google-antigravity/antigravity-cli/releases/tag/1.1.0)

## Build-time smoke checks
Agent (Dockerfile of the image that runs `install-antigravity-agent.sh`):
```bash
test -x /usr/local/bin/antigravity
test -f /usr/share/applications/antigravity.desktop
grep -q 'MimeType=.*x-scheme-handler/antigravity' /usr/share/applications/antigravity.desktop
! test -f /etc/xdg/autostart/antigravity.desktop
```
The `MimeType` line is written by `install-antigravity-agent.sh` itself. Callers should assert it wherever they install the Agent; it is not owned by Android Studio or ASfP specifically.
CLI (Dockerfile):

```bash
agy --version
```

## Known issues
### Antigravity OAuth requires Chrome launcher + `antigravity://` handler (TAA-2017)
Sign-in opens a browser and redirects to `antigravity://oauth-success`. Chrome must be reachable via `/usr/local/bin/google-chrome-stable` on `horizon-gnome` (wrapper after the package divert). The Agent installer registers `MimeType=x-scheme-handler/antigravity;` in `antigravity.desktop`. Without those, the Agent stays on "Awaiting Authentication..." or GNOME reports no app for the URI. Fixed under TAA-2017.

### GNOME Keyring unlock can block Antigravity token storage (TAA-2017)
The Agent stores OAuth tokens via Secret Service. Guacamole/RDP does not PAM-unlock the login keyring. `setup_keyring()` in `horizon-gnome` must unlock the session `gnome-keyring-daemon` (blank password) without starting a second conflicting daemon. Fixed under TAA-2017.

### Antigravity Desktop App v2.0 fails to reopen after being closed, requires force termination
- **What is expected:** Antigravity should close completely when dismissed. When the Antigravity icon is clicked, it should reopen without any issues.
- **What happens:** Closing the Antigravity window does not fully quit the app. A background process stays running (single-instance Electron-style lifecycle). The next click on the app menu starts another launch attempt, which sees that process and tries to hand off to it instead of starting cleanly — but the window never comes back. No error is shown.
  Fixing this properly needs a product change in Antigravity 2.0 (full quit on close, or restore the window on second launch).
- **Workaround:** Manually terminate the app. In a terminal on the workstation:
```bash
ps aux | grep -i antigravity
pkill -f antigravity   # or kill -9 <pid> if needed
```
**Related bug:** [[Bug] Antigravity Desktop App v2.0 Fails to Reopen After Being Closed on Windows 11 Requires Force Termination](https://github.com/google-antigravity/antigravity-cli/issues/108)

## References
* Scripts: workloads/cloud-workstations/pipelines/workstation-images/common-utils/antigravity/
* Config ops: Cloud Workstations config admin operations
* GitHub release https://github.com/google-antigravity/antigravity-cli/releases#release-1.1.0
