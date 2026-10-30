---
name: horizon-debug-cluster
description: 'Connect to a Horizon SDV GKE cluster via GKE Fleet Connect Gateway, sanity-check gcloud and kubectl, and offer a read-only cluster health check. Starts from a cluster URL or a GCP project ID. Use when the user wants to connect to a Horizon cluster, debug a live cluster, or check whether a deployment is healthy. Works on macOS, Linux, and Windows (via WSL).'
argument-hint: 'Optional: Horizon cluster URL or GCP project ID, plus optional gcloud account email'
---

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

# Horizon cluster debug

Gets a machine with no prior GCP setup from zero to running `kubectl` against a Horizon SDV GKE cluster, which is fleet-registered and reached only through **Connect Gateway** (there is no direct GKE endpoint). The user can name the target by **cluster URL** or by **GCP project ID**; the skill works out the other. The target can belong to a different Google account than the one active in `gcloud` (e.g. a temporary lab or training account); the skill then uses a separate `gcloud` configuration so the user's normal setup is untouched.

Scripts live in `scripts/` next to this file. Refer to that directory as `<SKILL_DIR>/scripts`.

## When to Use

- User wants `kubectl` access to a Horizon cluster for the first time
- User has a Horizon URL but does not know the GCP project behind it
- User needs `gcloud`, `kubectl`, or `gke-gcloud-auth-plugin` installed/updated
- `kubectl` commands hang, 403, or say `Reauthentication required.`
- `kubectl config current-context` shows the wrong project/cluster
- `gcloud container fleet memberships list` returns nothing

## Rules

1. **No changes without approval**: Do not modify the cluster, GCP, or any files without explicit approval. Read-only commands (`kubectl get`, `describe`, `logs`, and `gcloud` reads) may run without asking.
2. **No GCP changes**: Do not change IAM, enable APIs, register a fleet, or change organization policy or billing. If access is missing, report it and stop. Approval does not override this.
3. **Confirm the target**: Show the project ID, cluster URL and gcloud account, and get a yes before generating kubectl credentials. Do not use `gcloud config get project` silently.
4. **Report, don't fix**: A failed sanity check or health check is reported. Do not repair it unless the user then asks, and that repair still needs its own approval.
5. **No unapproved repo edits**: Do not modify Terraform, gitops, Helm values, or other tracked files unless the user explicitly asks for that edit. Show the change and get a separate approval before writing it. The result must still match a normal Horizon deployment.
6. **Authentication failures**: If a connection fails because gcloud credentials are missing, expired, or need reauthentication, stop. Tell the user to run this in their own terminal, then say when it has finished. Do not run it for them.
   ```bash
   gcloud auth login <ACCOUNT>
   ```
   Use the account the failure names. If it names none, use `gcloud auth login`.

## Procedure

Work through the steps in order. Detect the OS first, then run OS-specific commands only where noted — most steps are identical across macOS, Linux, and Windows because they go through `gcloud`.

### Step 0 — Gather the target

The target is **either** a Horizon cluster URL **or** a GCP project ID.

- If one was passed as the skill argument, use it.
- Otherwise ask the user for it. Mention they can paste the portal URL if they don't know the project. Show `gcloud config get project` as a possible default, but do not silently use it.

The user may also name a **gcloud account** (e.g. `bob@example.com`). If they do, record it as `<ACCOUNT>` and pass `--account <ACCOUNT>` to every script below. Otherwise don't ask; the active account is used, and Step 5 suggests other signed-in accounts if the target isn't found.

Do not ask for the fleet membership name up front — it is discovered in Step 7 and there is usually only one.

Detect the OS before running install commands:
- macOS: `uname -s` reports `Darwin`
- Linux: `uname -s` reports `Linux`
- Windows: the tools always run inside **WSL** (Windows Subsystem for Linux); use the Linux commands there and never PowerShell, winget, or Chocolatey installs. Cursor itself may run outside WSL — follow [Windows (WSL)](#windows-wsl) before Step 1.

### Step 1 — Install the Google Cloud CLI (if missing)

Check first: `gcloud --version`. If not found, install per OS:

**macOS (Homebrew):**
```bash
brew install --cask google-cloud-sdk
```

**Linux and Windows (WSL) — official quick-install script:**
```bash
curl -sSL https://sdk.cloud.google.com | bash
exec -l $SHELL
```

### Step 2 — Install kubectl and the GKE auth plugin

Connect Gateway requires `gke-gcloud-auth-plugin` in addition to `kubectl`. These commands are identical on macOS, Linux, and Windows:

```bash
gcloud components install kubectl
gcloud components install gke-gcloud-auth-plugin
```

> Note: on Debian/Ubuntu installs of `gcloud` via `apt`, `gcloud components install` is disabled — install `kubectl` via `sudo apt-get install kubectl` and `google-cloud-cli-gke-gcloud-auth-plugin` instead.

### Step 3 — Authenticate with Google Cloud

Ask the user to run this in their own terminal. It opens a browser, so do not run it for them. Use their account when they named one.

```bash
gcloud auth login <ACCOUNT>
```

When they say it has finished, confirm the account is present with `gcloud auth list`. It does not have to be the active account.

Any later failure whose message is missing credentials, expired credentials, or `Reauthentication required.` is an authentication failure: follow rule 6, then retry the step that failed.

### Step 4 — Verify prerequisites

```bash
bash <SKILL_DIR>/scripts/check-prerequisites.sh [--account <ACCOUNT>]
```

- `[OK]` — requirement met.
- `[FAIL]` — blocking; the script exits 1. List each failure with its remediation and **stop** until the user fixes it (go back to Steps 1–3 as needed).
- `[WARN]` — non-blocking (no default project, no ADC, no `dig`). Mention it and continue.

### Step 5 — Resolve the target

First check the remembered targets:

```bash
bash <SKILL_DIR>/scripts/targets.sh lookup "<URL_OR_PROJECT_ID>"
```

Each successful connection is saved (Step 9) to `targets.tsv` under `${XDG_CONFIG_HOME:-$HOME/.config}/horizon-cluster-connect/`, outside the skill. Exit `2` means not remembered; run `resolve-target.sh` below. Exit `0` prints one block of `HOSTNAME=`, `PROJECT_ID=`, `ACCOUNT=`, `MEMBERSHIP=`, `LOCATION=` and `CONTEXT=` per entry. A project ID can match several hostnames; they share the same project, so pick the first unless the accounts differ, in which case ask. If the user named an `<ACCOUNT>` that differs from the remembered one, use theirs. Then go to Step 6, saying the target is remembered, and after a yes follow [Reconnecting to a remembered target](#reconnecting-to-a-remembered-target) instead of Steps 7–8.

If nothing is remembered:

```bash
bash <SKILL_DIR>/scripts/resolve-target.sh [--account <ACCOUNT>] "<URL_OR_PROJECT_ID>"
```

The script treats input containing `.` or `/` as a URL and anything else as a project ID. Results are `KEY=VALUE` lines on stdout (including `ACCOUNT=`, the account the lookup ran as); progress goes to stderr. `--account` runs the lookup as that account without changing the active gcloud configuration.

| Exit | Meaning | What to do |
| --- | --- | --- |
| `0` | Resolved. `PROJECT_ID=` plus one or more `HOSTNAME=` lines | Go to Step 6 |
| `2` | No match (URL not served by any accessible project, or project ID not accessible) | If the script lists other signed-in accounts, ask the user whether to retry with one of them (`--account`), or to give the project ID and account. Otherwise stop and relay the likely causes; see [Troubleshooting](./references/troubleshooting.md) |
| `3` | Ambiguous URL. One `CANDIDATE=` line per project | Show a numbered list (plus `0` = cancel) and ask the user to pick |
| `1` | Usage, tooling or account error (e.g. account not signed in or expired) | Relay the script's message. If it is an authentication failure, follow rule 6, then re-run this step |

How it works:
- **Project ID** — reads hostnames from the project's `horizon-sdv-map` Certificate Manager map (wildcard entries dropped). If none are printed, tell the user the URL could not be confirmed and ask whether to continue.
- **URL** — strips scheme, path and port, lists every project from `gcloud projects list`, and checks each project's `horizon-sdv-map` in parallel (16 at a time), stopping at the first hostname match. Projects without the Certificate Manager API, or without read access, simply don't match. Exact hostname matches beat wildcard (`*.<host>`) matches. If several projects match, the URL's DNS IP is compared with each project's global forwarding rules to pick the owner.
- With many projects a URL scan can take 10–30 s; tell the user it is scanning. Flags: `--parallel N` for more concurrency, `--all` to scan every project instead of stopping at the first match.

### Step 6 — Confirm the target with the user

Show the resolved pair, e.g.:

> Project `my-horizon-prod` serves `https://prod.horizon.example.com`. Connect to it as `alice@example.com`?

If a gcloud configuration `horizon-<PROJECT_ID>` already exists, say it will be activated. Otherwise, if `ACCOUNT=` differs from the active gcloud account, say that a separate configuration `horizon-<PROJECT_ID>` will be created and activated. In both cases their current configuration is left as it is.

Only continue on an explicit yes.

### Step 7 — Set the active project and find the fleet membership

```bash
bash <SKILL_DIR>/scripts/use-gcloud-config.sh --project <GCP_PROJECT_ID> [--account <ACCOUNT>]
gcloud container fleet memberships list --project <GCP_PROJECT_ID>
```

`use-gcloud-config.sh` prints `CONFIG=` and `PREVIOUS_CONFIG=`:
- A configuration named `horizon-<PROJECT_ID>` already exists: activates it (setting `--account` on it when given).
- Same account as the active one (or no `--account`): sets the project on the active configuration.
- Different account: creates or reuses the configuration `horizon-<PROJECT_ID>` with that account and project, and activates it. This is required, not cosmetic: kubectl's Connect Gateway auth uses the **active** configuration's account on every call, so the active account must have access to the cluster.

When the configuration changed, tell the user how to get back: `gcloud config configurations activate <PREVIOUS_CONFIG>` plus `kubectl config use-context <their previous context>`. Contexts and configurations must be switched together.

The cluster is a fleet membership, not a plain GKE cluster, so credentials must be requested by **membership name**. Take the value in the `NAME` column (e.g. `sdv-horizon-cluster`) as `<MEMBERSHIP_NAME>`. If there is more than one, ask the user which. If the list is empty, see [Troubleshooting](./references/troubleshooting.md).

### Step 8 — Generate the kubectl context via Connect Gateway

```bash
gcloud container fleet memberships get-credentials <MEMBERSHIP_NAME> --project <GCP_PROJECT_ID>
```

This fetches a Connect Gateway kubeconfig entry (named like `connectgateway_<project>_<location>_<membership>`) and sets it as the current context.

### Step 9 — Sanity check

This is the only check to run when a connection is made. Do not run `cluster-health.sh` here.

```bash
gcloud config get-value account
gcloud config get-value project
kubectl config current-context
kubectl get ns
```

The account and project must be the ones just confirmed. Expect the context `connectgateway_<GCP_PROJECT_ID>_<location>_<MEMBERSHIP_NAME>` and a namespace list including `argo-workflows`, `workflows`, `horizon`, `module-manager`, `keycloak`. Report the context and whether those namespaces are present. Do not paste the full namespace list.

If `kubectl get ns` succeeded, remember the target. Use the hostname from Step 5 (skip saving if none was confirmed), the account the connection used, and the `LOCATION` column from the memberships list:

```bash
bash <SKILL_DIR>/scripts/targets.sh save <HOSTNAME> <GCP_PROJECT_ID> <ACCOUNT> <MEMBERSHIP_NAME> <LOCATION>
```

Save once per hostname when a project serves several. Saving an existing hostname replaces its entry.

### Reconnecting to a remembered target

Use the values from `targets.sh lookup`, after the user confirmed in Step 6:

```bash
bash <SKILL_DIR>/scripts/use-gcloud-config.sh --project <PROJECT_ID> --account <ACCOUNT>
kubectl config get-contexts -o name | grep -qxF <CONTEXT> \
  && kubectl config use-context <CONTEXT> \
  || gcloud container fleet memberships get-credentials <MEMBERSHIP> --project <PROJECT_ID> --location <LOCATION>
```

Tell the user how to get back if the configuration changed (as in Step 7), then run Step 9.

If this fails because of missing, expired, or reauthentication credentials, keep the remembered entry and follow rule 6, then retry this section.

Any other failure (configuration, `403`, missing project or membership, or `kubectl get ns`) means the entry is stale. Tell the user it was dropped, remove it, then run `resolve-target.sh` from Step 5 and continue from Step 6 with a fresh confirmation:

```bash
bash <SKILL_DIR>/scripts/targets.sh remove <HOSTNAME>
```

### Optional check — only when the user says yes

Ask: "Run a full cluster health check?" Run it only if the user says yes:

```bash
bash <SKILL_DIR>/scripts/cluster-health.sh
```

Quote the Verdict line and the [ISSUE] lines. Do not repair findings unless the user then asks, and that repair needs its own approval.

## Windows (WSL)

On Windows, `gcloud`, `kubectl`, `gke-gcloud-auth-plugin` and the skill scripts run inside WSL. Cursor may run on Windows itself, in which case the agent's shell is PowerShell or `cmd`, not WSL, and every command must be sent into WSL.

### Where the agent's shell runs

- `uname -s` prints `Linux` and `/proc/version` contains `microsoft`: the shell is already inside WSL (Cursor connected to WSL). Run everything as on Linux and skip the rest of this section.
- `uname -s` fails, or prints `MINGW…`, `MSYS…` or `CYGWIN…`: Cursor is on Windows. Use the rules below for every command in the procedure.

### Running commands from a Windows shell

```powershell
wsl.exe -e bash -lc 'source ~/google-cloud-sdk/path.bash.inc 2>/dev/null; kubectl get ns'
```

- Ubuntu's `.bashrc` stops early for non-interactive shells, so the `PATH` lines the gcloud quick-install adds there are not loaded. Sourcing `path.bash.inc` fixes that and is harmless when gcloud came from `apt`.
- Each `wsl.exe` call is a fresh shell: `cd`, variables and functions do not carry over. Chain dependent commands in one call.
- If `wsl.exe -l -v` lists more than one distribution, ask the user which one has gcloud and add `-d <DISTRO>` to every call.
- Wrap the bash command in single quotes. Inside a PowerShell single-quoted string, write each `'` as `''`. Windows PowerShell 5.1 drops embedded `"` when calling native programs, so avoid double quotes or use `pwsh` 7.3+.

### Skill scripts

`<SKILL_DIR>` is a Windows path that WSL cannot use directly, and files checked out or edited on Windows may have CRLF line endings, which make bash fail with `$'\r': command not found`. At the start of each session, copy the scripts into the WSL home with carriage returns removed and use the copy as `<SKILL_DIR>`:

```bash
src="$(wslpath -a '<WINDOWS_SKILL_DIR>')/scripts"
dst=~/.cache/horizon-cluster-connect/scripts
mkdir -p "$dst" && for f in "$src"/*.sh; do tr -d '\r' < "$f" > "$dst/${f##*/}"; done
```

Then run scripts as `bash ~/.cache/horizon-cluster-connect/scripts/<script>.sh`. Re-copy after the skill changes.

### What stays inside WSL

- **Sign-in**: the user runs `gcloud auth login` in their own WSL terminal (it needs a browser). If no browser opens, use `gcloud auth login --no-launch-browser` and paste the URL into a Windows browser.
- **kubeconfig**: contexts are written to `~/.kube/config` in WSL. Windows-side tools (Windows `kubectl`, Lens, Headlamp desktop) do not see them; say so if the user expects them to.
- **Remembered targets**: `~/.config/horizon-cluster-connect/targets.tsv` is in the WSL home and is not shared with any Windows-side install.
- **`dig`**: often missing on WSL Ubuntu (`sudo apt-get install dnsutils`). Optional; only used to break ties when several projects claim one URL.

## Required IAM Permissions

Connect Gateway proxies every `kubectl` call through Google's gateway using the user's live `gcloud` OAuth token (no long-lived cluster certificate). One of these project-level IAM roles is required:

| Role | Access level |
| --- | --- |
| `roles/gkehub.gatewayReader` | Read-only (`get`, `list`, `logs`) |
| `roles/gkehub.gatewayEditor` | Read/write (`create`, `apply`, `delete`) |
| `roles/gkehub.gatewayAdmin` | Full admin |

Each project's roles are granted to a specific Google account. If the user works with several accounts (e.g. corporate and lab accounts), pass the right one with `--account`.

URL lookup additionally needs read access to Certificate Manager on the owning project (`certificatemanager.certmapentries.list`, e.g. via `roles/certificatemanager.viewer` or project Viewer). Without it, connect by project ID instead.

In-cluster Kubernetes RBAC must *also* grant the account the desired verbs on the desired namespaces/resources — the IAM role alone only grants reachability to the gateway, not in-cluster permissions. If verification fails with a permissions error, report it and stop rather than retrying installs.

## Troubleshooting

See [troubleshooting reference](./references/troubleshooting.md) for fixes to common errors (403 Forbidden, empty membership list, URL not found or ambiguous, wrong project/cluster active, stale credentials).
