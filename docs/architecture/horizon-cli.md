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

# Horizon CLI

The Horizon CLI (`horizon`) runs Horizon workflows from a terminal or a pipeline. It is a thin client for the Horizon API: every command maps to an API call, so the CLI sees the same catalog and the same runs as the [Developer Portal](horizon-portal.md). Use it when a script, a CI job or a person at a terminal needs to submit a workflow and collect its logs and artifacts without a browser.

The CLI only works with modules that are enabled and that expose workflow templates. To learn how modules expose templates, see [Horizon modules overview](overview.md) and [How to create a new module](creating-modules.md#expose-label-catalog-and-portal-links).

Replace placeholder values (marked with `<...>`) with your environment-specific values before running any command on this page.

## Table of Contents

- [Basic sequence of operations](#basic-sequence-of-operations)
- [Installation](#installation)
- [Quick Start](#quick-start)
- [Configuration and sign-in](#configuration-and-sign-in)
- [Command reference](#command-reference)
- [Uninstall](#uninstall)
- [Appendix: Build from source](#appendix-build-from-source)

## Basic sequence of operations

Most sessions follow the same order. Each step depends on the one before it:

1. **Install the CLI.** Download the binary from **Tools → Horizon CLI** in the Developer Portal. Check it with `horizon version`.
2. **Point it at your cluster.** Set `HORIZON_DOMAIN`, or pass `--domain` on each command.
3. **Sign in.** Run `horizon auth login --device --write-config` once. The token is cached and refreshed automatically, so you don't need to sign in again for each command.
4. **Find a template.** Run `horizon catalog get` to see which templates enabled modules expose, and which parameters each one takes. You can only submit what the catalog lists.
5. **Submit a run.** Run `horizon workflow submit` with a module, a template and a JSON object of parameters. The CLI prints the generated workflow name, which every later command needs.
6. **Follow the run.** Use `horizon workflow logs <name>` to stream its output, or `horizon workflow wait <name>` to block until it finishes.
7. **Inspect the result.** Use `horizon workflow show <name>` to see the phase, archived logs and output artifacts.
8. **Collect artifacts.** Use `horizon workflow download-artifact <name> <artifact>` to download a file or get a signed URL.
9. **Tidy up.** Use `abort` to stop a run that's still going, `delete` to remove a finished run, and `horizon auth logout` to clear your cached token.

The CLI keeps no local state about workflows. You pass the workflow name from step 5 to every later command, so any terminal or CI step can pick up a run another one started.

In a pipeline, you can combine steps 5 to 7 into one blocking command:

```bash
horizon workflow submit --module <module> --template <template> \
  --params-json '<json>' --wait --logs
```

`--wait` makes the command exit only when the run finishes, and `--logs` streams the output while it waits. The exit code reflects the outcome, so the pipeline step passes or fails with the workflow:

| Exit code | Meaning |
|---|---|
| `0` | The workflow succeeded. |
| `1` | The workflow failed or errored, or the CLI hit an error. |
| `2` | The workflow was aborted, or the command was used incorrectly. |

## Installation

The CLI is built as part of every Horizon deployment, and the binaries are published in the Developer Portal. Download the binary that matches your cluster from there. That way the CLI version always matches the Horizon API it talks to. If you need to build it yourself, see [Appendix: Build from source](#appendix-build-from-source).

### Download from the Developer Portal

Sign in to the Developer Portal and open **Tools → Horizon CLI**. The page offers a ready-to-run binary built for your cluster, so you don't need a Go toolchain. It shows the CLI version and build date, a **Download for** button for the platform your browser runs on, and a table of every published binary with its SHA-256 checksum. The [Horizon Portal guide](horizon-portal.md#horizon-cli) describes the page itself.

Published platforms:

| OS | Arch | File |
| --- | --- | --- |
| linux | amd64 | `horizon` |
| linux | arm64 | `horizon` |
| darwin (macOS) | arm64 | `horizon` |
| windows | amd64 | `horizon.exe` |

To install:

1. Download the binary for your OS and architecture.
2. Compare the file checksum with the SHA-256 on that row (use the copy button).
3. Put the binary on `PATH`, using the commands below or the ones shown on the tab.
4. Verify with `horizon version`.

Checksum:

- Linux:

  ```bash
  sha256sum horizon
  ```

- macOS:

  ```bash
  shasum -a 256 horizon
  ```

- Windows (PowerShell):

  ```powershell
  Get-FileHash .\horizon.exe -Algorithm SHA256
  ```

Linux:

```bash
chmod +x horizon && sudo mv horizon /usr/local/bin/
```

macOS (Apple Silicon / arm64). Remove the quarantine attribute so Gatekeeper does not block this unsigned binary:

```bash
chmod +x horizon
xattr -d com.apple.quarantine horizon
sudo mv horizon /usr/local/bin/
```

Windows: `horizon.exe` is not digitally signed. SmartScreen may warn before the first run. Move the file onto your `PATH` as described in [Move to PATH](#move-to-path).

**Expected:** `horizon <version> (built <DD/MM/YYYY>)` matches the version and build date on the tab.

### Move to PATH
- For Linux/macOS:
```bash
sudo mv horizon /usr/local/bin/
```

Or a user-local install can be used instead:

```bash
mv horizon ~/go/bin/
```

- For Windows:

Create a directory for command-line tools, for example:
```powershell
New-Item -ItemType Directory -Force -Path C:\Tools
```
Move the binary:
```powershell
Move-Item .\horizon.exe C:\Tools\
```
Temporarily add C:\Tools to PATH for the current PowerShell session:
```powershell
$env:Path += ";C:\Tools"
```
Note: This change applies only to the current PowerShell session. To make it permanent, add C:\Tools to your User PATH through Windows Environment Variables.

From this point onward in the current PowerShell session, all examples can be executed as:
```powershell
horizon.exe <command>
```

### Verify installation
- Linux/macOS:
```bash
horizon --help
```
- Windows (in the current session, do not open a new tab):
```powershell
horizon.exe --help
```

**Expected:** Usage text is printed, starting with:

```
horizon — Horizon API CLI

Usage:
  horizon config <init|get|set> ...
  horizon auth <login|logout|refresh|whoami> [flags]
  horizon catalog get [--output text|json] [flags]   (default output: text)
  horizon version [--output text|json]
  horizon workflow <submit|wait|logs|abort|delete|show|list|get|running|history|download-artifact> ... [flags]
```

## Quick Start

This walkthrough takes you from a freshly installed CLI to a finished run, following the [basic sequence of operations](#basic-sequence-of-operations). It uses the `sample` module's `sample-smoke-test` template, so enable the `sample` module in **Administration → Modules** first. The module is described in [How to create a new module](creating-modules.md#module-manager). The sections after the Quick Start explain each setting and command in detail.

### Set your Horizon domain

Before running any command, set your Horizon domain:
- Linux/macOS:
```bash
export HORIZON_DOMAIN="<your-horizon-domain>"
```
- Windows:
```powershell
$env:HORIZON_DOMAIN = "<your-horizon-domain>"
```

Or use `--domain` on each command instead.

### Authentication and Catalog Access

#### Preconditions

- CLI installed (`horizon --help` works)
- Keycloak client configured for the target domain

#### Log in via device flow
- Linux/macOS:
```bash
horizon auth login --device --domain "$HORIZON_DOMAIN" --write-config
```
- Windows:
```powershell
horizon.exe auth login --device --domain $env:HORIZON_DOMAIN --write-config
```

A verification URL is printed to stderr. Open it in a browser and complete the sign-in.

**Expected output (stderr):**

- Linux/macOS:
```
Open this URL in a browser (sign in with your Horizon user):
https://<domain>/auth/realms/horizon/...
...
Logged in. Token saved to ~/.config/horizon/token.json
Wrote defaults to /home/<user>/.config/horizon/config.yaml
```
- Windows:
```
Logged in. Token saved to C:\Users\<user>\.config\horizon\token.json
Wrote defaults to C:\Users\<user>\AppData\Roaming\horizon\config.yaml
```

#### Verify token was stored
- Linux/macOS:
```bash
ls -la ~/.config/horizon/token.json
```
- Windows:
```powershell
Test-Path "$HOME\.config\horizon\token.json"
```

**Expected:**
- Linux/macOS: the file is listed with permissions `-rw-------` (`0600`).
- Windows:
```
True
```

#### Verify identity

```bash
horizon auth whoami
```

**Expected output (stdout):**

```
JWT claims (unverified):
  sub: <uuid>
  preferred_username: <your-username>
  exp: <timestamp> (valid for ...)
Horizon API catalog: OK (<N> bytes)
```

The `Horizon API catalog: OK` line confirms the token is accepted by the API.

#### List the catalog

```bash
horizon catalog get
```

**Expected output:** A list of available modules and templates with their parameters, e.g.:

```
Catalog (<N> entries)

sample / sample-smoke-test
  parameters:
  - sampleEnv
  - sampleBuildId
  ...
```

#### Catalog as JSON (optional verification)

```bash
horizon catalog get --output json
```

**Expected:** Valid JSON with an `"entries"` array containing objects with `"module"`, `"templateName"`, and `"parameters"` fields.

### Workflow Submit with Submitted-From Label

#### Preconditions

- The user is authenticated.
- A valid module, template, and parameters are known.
- Adjust the `--module`, `--template`, and `--params-json` values below to match a template from your catalog.

#### Submit a workflow

- Linux/macOS:
```bash
horizon workflow submit \
  --module sample \
  --template sample-smoke-test \
  --params-json '{"sampleEnv":"cli-test","sampleBuildId":"tcli02-001","sampleNote":"test-run"}' \
  --output json -q
```
- Windows:
```powershell
horizon.exe workflow submit `
  --module sample `
  --template sample-smoke-test `
  --params-json '{"sampleEnv":"cli-test","sampleBuildId":"tcli02-001","sampleNote":"test-run"}' `
  --output json -q
```

**Expected output (stdout):** JSON containing the workflow name:

```json
{
  "module": "sample",
  "template": "sample-smoke-test",
  "workflowName": "<generated-workflow-name>",
  "submitResponse": { ... }
}
```

Save the workflow name:

- Linux/macOS:
```bash
export WF_NAME="<generated-workflow-name>"
```
- Windows:
```powershell
$env:WF_NAME = "<generated-workflow-name>"
```

#### Inspect the workflow for submitted-from

- Linux/macOS:
```bash
horizon workflow show "$WF_NAME" --output json | grep -i submittedFrom
```
- Windows:
```powershell
horizon.exe workflow show $env:WF_NAME --output json | Select-String "submittedFrom"
```

**Expected output:** The JSON includes `"submittedFrom"` with value `"horizon-cli"`:

```
"submittedFrom": "horizon-cli",
```

This confirms the CLI sent the `X-Horizon-Submitted-From: horizon-cli` header and the `horizonSubmittedFrom` parameter.

#### Verify via text output

- Linux/macOS:
```bash
horizon workflow show "$WF_NAME"
```
- Windows:
```powershell
horizon.exe workflow show $env:WF_NAME
```

**Expected:** The summary block under "Workflow detail" includes a `"submittedFrom": "horizon-cli"` entry.

### Logs, Abort, and Artifact Download

#### Preconditions

- A workflow submitted via the CLI exists (from the previous step, or submit a new one). 
- `WF_NAME` is set.

#### Stream logs

- Linux/macOS:
```bash
horizon workflow logs "$WF_NAME"
```
- Windows:
```powershell
horizon.exe workflow logs $env:WF_NAME
```

**Expected:** Log lines stream to stdout in the format `[stageName] [timestamp] [message]`. The stream ends with:

```
━━ log stream end ━━
```

If the workflow has already completed, historical logs are printed (non-follow mode). If pods have not started yet, the CLI waits up to 60 seconds before opening the stream.

#### Abort a running workflow

Run this while the workflow is still active (phase is not Succeeded/Failed/Error/Aborted):

- Linux/macOS:
```bash
horizon workflow abort "$WF_NAME"
```
- Windows:
```powershell
horizon.exe workflow abort $env:WF_NAME
```

**Expected output (stderr):**

```
Aborting workflow <WF_NAME> via Horizon API ...
```

If the workflow has already reached a terminal phase, the command exits silently with code 0 (no error).

Verify the abort took effect:

- Linux/macOS:
```bash
horizon workflow show "$WF_NAME" --output json | grep '"phase"'
```
- Windows:
```powershell
horizon.exe workflow show $env:WF_NAME --output json | Select-String '"phase"'
```

**Expected:** `"phase": "Aborted"` (may take a few seconds to transition).

#### Retrieve an artifact signed URL

This requires a completed workflow that produced output artifacts. First check for available artifacts:

- Linux/macOS:
```bash
horizon workflow show "$WF_NAME"
```
- Windows:
```powershell
horizon.exe workflow show $env:WF_NAME
```

Look for `outputArtifact:` lines in the output. If artifacts exist, generate a signed URL:

- Linux/macOS:
```bash
horizon workflow download-artifact "$WF_NAME" "<artifact-name>" --generate-signed-url
```
- Windows:
```powershell
horizon.exe workflow download-artifact $env:WF_NAME "<artifact-name>" --generate-signed-url
```
Note: If several steps produced an artifact with this name, the CLI picks the last one and prints a note. Pass `--node-id` or `--template-name` to choose another. See [`horizon workflow download-artifact`](#horizon-workflow-download-artifact).

**Expected output (stdout):**

```
signed-url: https://storage.googleapis.com/...
```

#### Download an artifact to a file (optional)

- Linux/macOS:
```bash
horizon workflow download-artifact "$WF_NAME" "<artifact-name>" -o ./artifact-output
```
- Windows:
```powershell
horizon.exe workflow download-artifact $env:WF_NAME "<artifact-name>" -o ./artifact-output
```

**Expected:** A progress bar on stderr followed by a completion message. The file `./artifact-output` is created with the artifact contents.

### Cleanup

After the work is done, optionally log out and remove local state:

```bash
horizon auth logout
```

**Expected output (stderr):**

- Linux/macOS:
```
Removed ~/.config/horizon/token.json (if present).
```
- Windows:
```
Removed C:\Users\<user>\.config\horizon\token.json (if present).
```

## Configuration and sign-in

### Where settings come from

The CLI reads its settings from three places. Command-line flags win over environment variables, which win over the config file:

1. Flags on the command, such as `--domain`, `--api`, `--module` and `--template`.
2. Environment variables.
3. The config file `~/.config/horizon/config.yaml` (on Windows, `%APPDATA%\horizon\config.yaml`).

The most useful environment variables are:

| Variable | Purpose |
|---|---|
| `HORIZON_DOMAIN` | Your Horizon domain. The CLI derives the API URL (`https://<domain>/horizon-api`) and the Keycloak URL (`https://<domain>/auth`) from it. |
| `HORIZON_API_BASE_URL` | The full API URL, if it isn't at the default path. Overrides `HORIZON_DOMAIN` for the API. |
| `HORIZON_ACCESS_TOKEN` | A ready-made access token. When set, the CLI uses it as-is and doesn't refresh it. |
| `KEYCLOAK_CLIENT_SECRET` | The secret for the confidential `horizon-api-ci` client. When set, the CLI signs in non-interactively, which is what you want in CI. |
| `KEYCLOAK_CLIENT_ID`, `KEYCLOAK_REALM`, `KEYCLOAK_BASE` | Override the Keycloak client, realm (default `horizon`) or URL. |
| `MODULE`, `TEMPLATE` | Default module and template for `workflow submit`. |
| `WORKFLOW_PARAMETERS_JSON`, `WORKFLOW_PARAMETERS_JSON_FILE` | Default parameters for `workflow submit`, inline or from a file. |
| `WAIT_TERMINAL_SECS`, `LOG_STREAM_MAX_SECS`, `LOG_WAIT_POD_SECS` | How long `wait` waits (default 3600 seconds), how long a log stream may stay open (default 7200), and how long `logs` waits for pods to start (default 60). |

### Managing the config file

`horizon config` reads and writes `config.yaml`, so you don't have to set environment variables in every shell:

- `horizon config init` creates a commented template file. Add `--force` to overwrite an existing one.
- `horizon config set <key> <value>` sets one value. The keys are `domain`, `api_base_url`, `keycloak_base`, `keycloak_realm`, `module` and `template`.
- `horizon config get [key]` prints the file, or one key from it. Add `--merged` to print the values the CLI actually uses after combining the file and the environment.

`horizon auth login --write-config` also writes the domain and Keycloak settings to this file for you.

### How sign-in works

The CLI needs a Keycloak access token for the `horizon` realm. It looks for one in this order:

1. `HORIZON_ACCESS_TOKEN`, if set.
2. The token cache at `~/.config/horizon/token.json` (on Windows, `%USERPROFILE%\.config\horizon\token.json`), written by `horizon auth login`. The CLI refreshes an expired token automatically, and retries a request once if the API rejects the token.
3. `KEYCLOAK_CLIENT_SECRET`, if set. The CLI fetches a token with the client-credentials grant.

If none of these is available, the command fails with `not authenticated`.

There are two ways to sign in:

- **Device flow** (`--device`) is for people. The CLI prints a URL, and you open it in a browser and sign in with your Horizon user. It uses the public `horizon-api` client.
- **Client credentials** (`--client-credentials`, or automatically when `KEYCLOAK_CLIENT_SECRET` is set) is for CI and other automation. It uses the confidential `horizon-api-ci` client and needs no browser.

## Command reference

Every command accepts `--domain HOST` and `--api URL` to target a cluster other than the configured one. `horizon ci ...` is an alias for `horizon workflow ...`, kept for existing pipelines.

### `horizon version`

Prints the CLI version and build date. Add `--output json` for machine-readable output. A binary downloaded from the Developer Portal should match the version and date shown on the Tools page.

### `horizon auth`

| Command | What it does |
|---|---|
| `auth login` | Signs in and caches the token. Use `--device` for the browser-based device flow, or `--client-credentials` for a confidential client. `--write-config` saves the domain and Keycloak settings to `config.yaml`. `--open-browser` and `--no-browser` control whether the CLI opens the sign-in URL for you. |
| `auth refresh` | Exchanges the cached refresh token for a new access token, without signing in again. Normally you don't need this, because other commands refresh automatically. |
| `auth whoami` | Prints the user and expiry from the current token, then calls the catalog to prove the API accepts it. `Horizon API catalog: OK` means you're ready to go. |
| `auth logout` | Deletes the cached token file. |

### `horizon catalog get`

Lists every template you can submit, grouped by module, with each parameter's name, default value and description. Only templates from enabled modules that are marked as exposed appear. If a template you expect is missing, check in **Administration → Modules** that its module is enabled and ready.

Add `--output json` to get the raw catalog, which is useful for scripts. Add `--raw` as well for compact one-line JSON.

### `horizon workflow submit`

Starts a run of one template. It takes these options:

- **`--module` and `--template`** name the template, exactly as `catalog get` shows them.
- **`--params-json '<json>'`** passes parameters inline. `--params-file <path>` reads them from a file, and `--params-file -` reads them from standard input.

Before submitting, the CLI fetches the template's catalog entry and checks your parameters against it. Any parameter you leave out takes its catalog default. A parameter the template doesn't declare is rejected, so typos fail fast instead of producing a broken run.

The API hands the run to Argo Events, which creates the workflow. The CLI then waits until the new workflow appears and prints its generated name. With `--output json`, it prints a JSON object containing `workflowName`, which is easier to parse in a script. `-q` suppresses progress messages on standard error.

Add `--wait` to block until the run finishes and exit with its result code, and `--logs` as well to stream the output while you wait. Every run the CLI submits is tagged `submittedFrom: horizon-cli`, so you can tell CLI runs apart from portal and API runs.

### `horizon workflow wait`

Blocks until the named workflow finishes, then exits with `0`, `1` or `2` as described in [Basic sequence of operations](#basic-sequence-of-operations). Add `--logs` to stream the output while you wait. Use this when you submitted without `--wait` and want to rejoin the run later, perhaps from another CI step.

### `horizon workflow logs`

Streams the workflow's log output. Each line has the form `[stage] [timestamp] [message]`, and the stream ends with `━━ log stream end ━━`.

If the workflow is still running, the stream follows it live. If it has already finished, you get the archived logs instead. If the workflow's pods haven't started yet, the CLI waits up to 60 seconds for them before it opens the stream.

### `horizon workflow show` and `get`

`show` prints a summary of one workflow: its name, phase, template and where it was submitted from. It also lists the GCS locations of its archived logs and output artifacts, and the `outputArtifact:` lines give you the artifact names to download. `show --output json` and `get` both print the full API response instead.

### `horizon workflow list`, `running` and `history`

These commands find runs when you don't have a name:

- **`list`** shows running workflows and recent history together. In a terminal it prints a table with each workflow's name, phase and source. When its output is piped, it prints JSON instead. `--running-only` skips history.
- **`running`** prints running workflows as JSON.
- **`history`** prints finished workflows as JSON.

All three take `--limit N`, which defaults to 50. `history` also takes `--phase` (for example, `--phase Failed,Error`) and `--continue <token>`, which fetches the next page.

Finished workflows are kept only for a limited time, after which they no longer appear in history. By default, successful runs are kept for one day and failed runs for three days.

### `horizon workflow abort` and `delete`

`abort` stops a running workflow. If the workflow has already finished, it does nothing and exits successfully, so it's safe to call at the end of a CI job.

`delete` removes a finished workflow from the cluster. The server waits until the workflow and its finalizers are gone, which can take several minutes.

### `horizon workflow download-artifact`

Fetches one output artifact of a run. Take the artifact name from the `outputArtifact:` lines that `show` prints. The command has three modes:

- **With no output option**, it downloads the file into the current directory under its original name, showing progress on standard error.
- **With `-o <path>`**, it writes to that path. With `-stdout`, it writes the bytes to standard output so you can pipe them.
- **With `--generate-signed-url`**, it prints a time-limited GCS URL instead of downloading, so another tool or person can fetch the file. `--duration <seconds>` asks for a different lifetime, which the server may limit.

Sometimes several steps of one workflow produce an artifact with the same name. The API then reports a conflict, and the CLI automatically picks the last matching step and prints a note saying so. To choose a different step, pass `--node-id` with one of the IDs from that note, or `--template-name` with the step's template.

## Uninstall

### Remove the binary

- Linux/macOS:
```bash
sudo rm "$(which horizon)"
```
- Windows:
```powershell
Remove-Item (Get-Command horizon.exe).Source
```

Expected: No errors. Verify with:

- Linux/MacOS:
```bash
which horizon
```
- Windows
```powershell
Get-Command horizon.exe
```

Expected:
- Linux/macOS:
No output, or `horizon not found`.
- Windows:
Get-Command : The term 'horizon.exe' is not recognized as the name of a cmdlet, function,
script file, or operable program.

### Remove configuration and cached tokens

- Linux/macOS:
```bash
rm -rf ~/.config/horizon/
```
- Windows:
```powershell
Remove-Item "$env:APPDATA\horizon" -Recurse -Force
Remove-Item "$HOME\.config\horizon" -Recurse -Force
```

This deletes both `config.yaml` and `token.json`.

### Verify removal

- Linux/macOS:
```bash
ls ~/.config/horizon/
```
- Windows:
```powershell
Test-Path "$env:APPDATA\horizon"
Test-Path "$HOME\.config\horizon"
```

**Expected:**

- Linux/macOS:
```
ls: cannot access '/home/<user>/.config/horizon/': No such file or directory
```
- Windows:
```
False
False
```

## Appendix: Build from source

You don't normally need to build the CLI. Each deployment builds it and publishes the binaries in the Developer Portal, as described in [Installation](#installation). Build from source only if you are changing the CLI itself, or need a binary for a platform the portal doesn't publish.

### Prerequisite

Go [1.27+](https://go.dev/dl/) must be installed. Verify with:

```bash
go version
```

**Expected:** Output shows `go1.27` or higher.

### Build the binary

From the repository root:

```bash
cd tools/horizon
go build -o horizon .
```

**Expected:** No errors. A `horizon` binary is created in the current directory.

Then put the binary on your `PATH` as described in [Move to PATH](#move-to-path), and check it as in [Verify installation](#verify-installation).

### Alternative: install with `go install`

From the repository root:

```bash
go install ./tools/horizon
```

This places the binary in `$GOBIN` (defaults to `~/go/bin`). Ensure that directory is on your `PATH`.
