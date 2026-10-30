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

# Horizon Portal

## Table of Contents

- [Developer Portal](#developer-portal)
  - [Description](#description)
  - [Sign in](#sign-in)
  - [Browsing](#browsing)
  - [Administration](#administration)
    - [Modules](#modules)
    - [Settings](#settings)
  - [Module pages](#module-pages)
    - [Submit a workflow](#submit-a-workflow)
    - [Running Workflows and History](#running-workflows-and-history)
    - [Workflow details, abort and delete](#workflow-details-abort-and-delete)
    - [Logs and artifacts](#logs-and-artifacts)
  - [Tools](#tools)
    - [Horizon CLI](#horizon-cli)
    - [Horizon API](#horizon-api)

## Developer Portal

### Description

The Developer Portal is a single web UI in front of Horizon API and Module Manager, which are internal services. It also serves the Horizon CLI binaries built for this cluster. Users can enable and disable modules, submit and follow workflows, download the Horizon CLI, browse the Horizon API, and open the other platform applications.

`https://<your-horizon-domain>/developer-portal/`

Each module is a separate Argo CD Application (app-of-apps). Module Manager creates or deletes it when you enable or disable the module in the portal. The modules on offer come from the Module Catalog (`gitops/apps/module-manager/templates/module-catalog.yaml`). This release ships:

- sample-hard
- sample-soft
- sample
- sample-data
- storage-gcs
- workloads-common
- workloads-android
- remotive-topology
- abfs-data
- abfs-service-module
- abfs-client-ubuntu
- cloud-workstations (experimental, main environment only; requires workloads-common)

### Sign in

Opening the portal without a session takes you to the sign-in page. **Sign in with SSO (Keycloak)** uses the `horizon` realm. A username and password form is also shown; it only works if direct password sign-in is enabled on the `horizon-dev-portal` Keycloak client.

The session lasts for the browser tab and is refreshed automatically while you use the portal. If it can no longer be refreshed, the portal returns you to the sign-in page. The app bar shows your username, a light/dark theme toggle (remembered in this browser) and a sign-out button.

### Browsing

The sidebar always shows these entries:

- **Welcome**: introduction to Horizon, the portal, modules and platform delivery, with shortcuts to **Administration → Modules**, **Administration → Settings** and **Tools**, and links to the GitHub repository and the deployment guide.
- **Landing page**: cards for the platform applications, each opening in a new tab:
  - Developer: Gerrit, Jenkins, MTK Connect, MCP Gateway Registry
  - Admin: Keycloak, Argo CD, Headlamp, Grafana, Argo Workflows

  All cards open a path on your Horizon domain, except MCP Gateway Registry, which opens the `mcp.<your-horizon-domain>` subdomain.
- **Tools**: the Horizon CLI download and the Horizon API reference. See [Tools](#tools).
- **Administration**: module management and portal settings. See [Administration](#administration).

Below these, the sidebar lists one entry per enabled module, once the module is **READY**. A module that is still installing or updating does not appear yet. A module that is being uninstalled stays listed until the uninstall finishes. The sidebar refreshes every few seconds.

### Administration

Administration has two tabs: **Modules** and **Settings**.

#### Modules

Each module in the catalog has a card showing its name, a status chip, its hard and soft dependencies and, when enabled, the modules that depend on it.

| Status | Meaning |
|---|---|
| NOT INSTALLED | The module is disabled and nothing is deployed. |
| DEPLOYMENT PENDING | The module is enabled, but Argo CD has no matching Application yet. This can be reconcile lag, a manually deleted Application, or Module Manager watching a different cluster or namespace. Disable the module if this is unintended. |
| INSTALLATION IN PROGRESS | The Argo CD Application exists and is still syncing or becoming healthy. |
| UPDATE IN PROGRESS | A healthy module is syncing a change, for example after a ref change. |
| UNINSTALL IN PROGRESS | The module is disabled and its Applications are being removed. |
| READY | Synced and healthy. The module appears in the sidebar. |

On each card you can:

- **Enable or disable** the module with the switch. Enabling a module also enables its hard dependencies. Disabling is refused while another enabled module has it as a hard dependency; the card shows *"Cannot disable: required by: …"*. First installs and uninstalls can take several minutes, and disabling may wait while the module's cloud resources are removed.
- **Pin to ref** (enabled module) or **Pin ref before install** (disabled module) to deploy the module from a specific branch, tag or commit instead of the platform branch. A pinned module shows a **Pinned** chip and the ref; otherwise it shows **Following platform** and the cluster's ref. Use **Edit ref** to change a pin and **Reset to platform branch** to remove it.

Do not reload or close the tab while an enable, disable, pin or reset is running. Reloading cancels the browser request, while the server may still apply all or part of the change, so the page can disagree with the cluster until the operation completes. The buttons are locked while an operation or a status change is in progress. When idle, the page refreshes status every few seconds and when you return to the tab.

Hard and soft dependencies are explained in [Horizon modules overview](01_overview.md#how-modules-depend-on-each-other), the sample modules in [How to create a new module](05_creating-modules.md#sample-modules-and-dependencies), and what pinning does in the cluster in [Pinning and Resetting module Git refs](04_module-architecture.md#pinning-and-resetting-module-git-refs).

#### Settings

- **Workflows visibility** controls which workflow runs appear under **Running Workflows** and **History** in every module, based on where they were submitted from (the `horizon-sdv.io/submitted-from` label: API, Developer Portal, Horizon CLI, or other values seen in recent runs). **Show all sources (no filter)** is the default. If you clear it and select no sources, no runs are shown. While a filter is active, runs without the label are hidden. The setting is saved in Module Manager, so it applies to all users.
- **Workflow log buffer (browser)** sets **Max lines**, the number of log lines the log viewer keeps in memory. The default is 50,000 and the range is 5,000–200,000. Higher values use more memory in the browser tab. The setting is stored only in this browser.

### Module pages

Opening a module from the sidebar shows up to five tabs. The selected tab is part of the URL (`?tab=`), so you can bookmark it.

- **Overview**: the module's own overview page, served from inside the cluster and styled to match the portal theme. If the module has no overview configured, the tab says so.
- **Applications**: one card per application the module exposes, each with an **Open** button that opens it in a new tab. Shown only if the module exposes applications.
- **Workflow Templates**, **Running Workflows** and **History**: shown only if the module exposes workflow templates in the Horizon catalog. See [Expose label (catalog and portal links)](05_creating-modules.md#expose-label-catalog-and-portal-links).

#### Submit a workflow

**Workflow Templates** lists the module's exposed templates. **Configure & submit** opens a form with one field per template parameter:

- Fields start with the template's default value. Parameters with a fixed set of values are drop-downs.
- Fields without a default are marked `*`. The portal does not block an empty value, so check these before submitting.
- Multi-line values open in a YAML editor.
- **Submitted by** is filled from your username and is not shown.

The portal submits the run as `developer-portal`. If no response arrives within 45 seconds, the form shows a timeout error. On success, the page switches to **Running Workflows**.

#### Running Workflows and History

**Running Workflows** lists runs still in progress: template, start time, live duration, where it was triggered from, who submitted it, and phase. **History** lists finished runs with their finish time and **Result**. Both are newest first, show at most 200 runs and refresh every few seconds. Each row shows the template name; hover over it to see the run name. Click a row to open the run.

Finished runs are removed from the cluster after 1 day if they succeeded and after 3 days if they failed. The **Workflows visibility** setting can hide runs from both lists; see [Settings](#settings).

#### Workflow details, abort and delete

The run dialog shows the template, run name, who submitted it and from where, and an **Open in Argo Workflows** link. Argo Workflows as a whole opens from its card on the Landing page. For a walk-through with the `sample` module, see [Workflows via Developer Portal](05_creating-modules.md#workflows-via-developer-portal). The link is disabled once the run has been removed from the cluster. You can resize the dialog from its bottom-right corner; the size is remembered in this browser.

- **Abort** is shown while the run is in progress. After you confirm, the run stops gracefully, so it can take a moment to finish, and usually ends as *Aborted*.
- **Delete** is shown once the run has finished, from either **Running Workflows** or **History**. After you confirm, the run and its artifacts are permanently removed. This can take a few minutes; the run shows *Deletion in progress* meanwhile.

#### Logs and artifacts

- **Files** lists the run's output artifacts, each with **GCS** (open the object in Cloud Storage) and **Download** (a short-lived signed link, opened in a new tab).
- **Open live stream** (or **Open stream** for a finished run) streams the logs of the whole run, each line tagged with its step. Once the run has finished and its pods are gone, the stream is empty; use the per-step logs instead.
- The steps table lists each step with its phase and pod. **Show** streams that step's logs while it runs and loads the archived log from Cloud Storage once it has finished. **GCS** and **Download** in this table refer to the step's archived log.

The log viewer keeps the number of lines set in **Max lines** (see [Settings](#settings)). When the limit is reached, the oldest lines are dropped.

### Tools

Sign in to Developer Portal (`https://<your-horizon-domain>/developer-portal/`) and open **Tools** in the sidebar, or Welcome → **Tools**.

The page opens on **Horizon CLI** (`/developer-portal/tools/horizon-cli`). Switch to **Horizon API** (`/developer-portal/tools/horizon-api`) for the API reference.

#### Horizon CLI

Download a ready-to-run `horizon` binary built for this cluster. A Go toolchain is not required.

The tab shows:

- **Version** and, directly under it, the **build date** (`DD/MM/YYYY`).
- A **Download for** button for the operating system and architecture detected in this browser. That row is highlighted in the table. If the exact architecture is not published, the button uses another binary for the same operating system when one exists. macOS binaries are published for Apple silicon (arm64) only, so on an Intel Mac the button offers a binary that will not run.
- A table of every published binary: operating system, architecture, size, SHA-256 (copy), and a download named `horizon` or `horizon.exe`.
- Short install and verify steps for Linux, macOS and Windows.

If this environment has no CLI binaries, the tab shows an error and no download table.

The published platforms, checksum and install steps for each operating system, and everything you can do with the CLI once it's installed are in [Horizon CLI](03_horizon-cli.md#download-from-the-developer-portal).

#### Horizon API

The **Horizon API** tab embeds Swagger UI for this cluster. The document is built from the live catalog: each exposed workflow template is a submit operation, next to the shared catalog and workflow lifecycle operations.

Templates that are not exposed (for example `sample-hard-smoke-test`) do not appear. See [Expose label (catalog and portal links)](05_creating-modules.md#expose-label-catalog-and-portal-links). With no enabled module exposing workflows, the catalog is empty and there are no submit operations.

**Try it out** calls `https://<your-horizon-domain>/horizon-api/`. Those requests need a Keycloak access token for the `horizon` realm:

1. Sign in with the CLI: [Sign in](03_horizon-cli.md#1-sign-in).
2. In Swagger UI, click **Authorize**, select **bearerAuth**, and paste the `access_token` value from the CLI token file (`~/.config/horizon/token.json` on Linux and macOS, `%USERPROFILE%\.config\horizon\token.json` on Windows).
3. Execute the operation. Optional submit header `X-Horizon-Submitted-From` is `api` by default. `developer-portal`, `horizon-cli` and `custom` are the other standard values; with `custom`, put your integration ID in `X-Horizon-Submitted-From-Detail`.

The same document is also served directly at `https://<your-horizon-domain>/horizon-api/swagger` (`openapi.json` is next to it). Swagger UI loads its scripts and styles from `unpkg.com`. If the embedded page stays blank, your browser or network is probably blocking that site; use the direct URL, or `openapi.json` with a local OpenAPI viewer.
