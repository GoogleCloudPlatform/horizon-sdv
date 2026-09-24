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

# Modularization Know How

## Table of Contents

- [Developer Portal](#developer-portal)
  - [Module manager](#module-manager)
    - [Description](#description)
    - [Browsing](#browsing)
    - [Hard dependency (`sample` → `sample-hard`)](#hard-dependency-sample--sample-hard)
      - [Enable behavior](#enable-behavior)
      - [Disable behavior](#disable-behavior)
      - [Runtime / deploy](#runtime--deploy)
      - [Auto-disable](#auto-disable)
    - [Soft dependency (`sample` → `sample-soft`)](#soft-dependency-sample--sample-soft)
      - [Enable behavior](#enable-behavior-1)
      - [Disable behavior](#disable-behavior-1)
      - [Auto-disable](#auto-disable-1)
    - [How the three modules differ (summary)](#how-the-three-modules-differ-summary)
    - [Change git ref in developer portal](#change-git-ref-in-developer-portal)
  - [Workflows via Developer Portal](#workflows-via-developer-portal)
    - [Submit](#submit)
    - [History](#history)
    - [How to abort/delete workflows](#how-to-abortdelete-workflows)
    - [Hello-world application](#hello-world-application)
  - [Workflows via Horizon CLI](#workflows-via-horizon-cli)
    - [Installation](#installation)
      - [Prerequisite](#prerequisite)
      - [Build the binary](#build-the-binary)
      - [Move to PATH](#move-to-path)
      - [Verify installation](#verify-installation)
      - [Alternative: install via `go install`](#alternative-install-via-go-install)
    - [Environment Setup](#environment-setup)
    - [Authentication and Catalog Access](#authentication-and-catalog-access)
      - [Preconditions](#preconditions)
      - [Log in via device flow](#log-in-via-device-flow)
      - [Verify token was stored](#verify-token-was-stored)
      - [Verify identity](#verify-identity)
      - [List the catalog](#list-the-catalog)
      - [Catalog as JSON (optional verification)](#catalog-as-json-optional-verification)
    - [Workflow Submit with Submitted-From Label](#workflow-submit-with-submitted-from-label)
      - [Preconditions](#preconditions-1)
      - [Submit a workflow](#submit-a-workflow)
      - [Inspect the workflow for submitted-from](#inspect-the-workflow-for-submitted-from)
      - [Verify via text output](#verify-via-text-output)
    - [Logs, Abort, and Artifact Download](#logs-abort-and-artifact-download)
      - [Preconditions](#preconditions-2)
      - [Stream logs](#stream-logs)
      - [Abort a running workflow](#abort-a-running-workflow)
      - [Retrieve an artifact signed URL](#retrieve-an-artifact-signed-url)
      - [Download an artifact to a file (optional)](#download-an-artifact-to-a-file-optional)
    - [Cleanup](#cleanup)
    - [Uninstall](#uninstall)
      - [Remove the binary](#remove-the-binary)
      - [Remove configuration and cached tokens](#remove-configuration-and-cached-tokens)
      - [Verify removal](#verify-removal)
  - [Expose label (catalog and portal links)](#expose-label-catalog-and-portal-links)
    - [1. WorkflowTemplate Visibility in the Developer Portal](#1-workflowtemplate-visibility-in-the-developer-portal)
      - [Updating WorkflowTemplate Visibility](#updating-workflowtemplate-visibility)
    - [2. Exposing Child Applications in the Portal Navigation](#2-exposing-child-applications-in-the-portal-navigation)
      - [Configuring Application Visibility](#configuring-application-visibility)
      - [Removing Application Visibility](#removing-application-visibility)
    - [3. Visibility Restrictions Based on Module Enablement](#3-visibility-restrictions-based-on-module-enablement)

## Developer Portal

### Module manager

#### Description

Developer portal is a proxy to Horizon and Module Manager internal services. Users can discover modules, submit workflows and navigate platform views in one UI.

**Modules:**

Each module is a separate ArgoCD Application (app-of-apps) and is created/deleted by Module Controller based on Developer Portal requests.

`https://<your-horizon-domain>/developer-portal/`

To find modules, go to “Administration” tab, or click on “Open Administration → Modules”.

Currently there are 5 modules available:
- sample-hard
- sample-soft
- sample
- workloads-common
- workloads-android

#### Browsing

There are 3 tabs available by default:

- Welcome
- Landing
- Administration

The Landing Page provides a simple and clear home page for the Horizon SDV project with developer (Gerrit, Jenkins, MTK Connect, MCP Gateway Registry) and admin (Keycloak, Argo CD, Headlamp, Grafana) applications as cards opening cluster paths in a new tab.

In Administration tab the modules can be enabled/disabled; if the module is enabled it should be listed with the other tabs.

**Dependencies** are how one module relies on another. **Hard** vs **soft** describes how tightly and necessarily that reliance is. For example: `sample` is dependent on `sample-hard`. If `sample` is enabled, then `sample-hard` cannot be disabled and will result in an error in developer-portal.

#### Hard dependency (`sample` → `sample-hard`)

A **hard dependency** means: **the parent cannot be correctly enabled or kept running without the dependency module**.

##### **Enable behavior**

When `sample` is enabled, Module Manager **recursively enables** `sample-hard` **first** (if it is not already enabled). Enable fails if a hard dependency cannot be satisfied.

##### **Disable behavior**

`sample-hard` cannot be disabled while `sample` is still enabled. The API returns **409** with `hardDependents: ["sample"]`. The dev portal surfaces this as _“Cannot disable: required by: sample”_.

##### **Runtime / deploy**

`sample-hard` must be deployed for `sample` to be valid. Its Argo workflows (`sample-hard-smoke-test`) are **not** exposed on the Horizon API catalog — they are internal infrastructure.

##### **Auto-disable**

`sample-hard` is auto-disabled when **no enabled module lists it as a hard dependency** (only `sample` does).
Auto-disable of modules is optional. By default - the modules do not get disabled if it has no dependency.

#### Soft dependency (`sample` → `sample-soft`)

A **soft dependency** means: **the parent can run without the dependency**, but may **enable extra behavior** when the soft module is on.

##### **Enable behavior**

Enabling `sample` **does not auto-enable** `sample-soft`. You turn `sample-soft` on separately if you want the optional path.

##### **Disable behavior**

You **can disable** `sample-soft` while `sample` **stays enabled**. There is no hard-block on disable (unlike hard deps). Parents that list a soft dep are **not** prevented from disabling the soft module.

##### **Auto-disable**

`sample-soft` is auto-disabled when **no enabled module depends on it** (hard or soft). While `sample` is enabled, `sample-soft` counts as a soft dependent, so it stays eligible to remain on.
Auto-disable of modules is optional. By default - the modules do not get disabled if it has no dependency.

#### How the three modules differ (summary)

| | sample-hard | sample-soft | sample |
| --- | --- | --- | --- |
| **Dependency type** | Hard dep _target_ | Soft dep _target_ | Parent with both |
| **Required for sample?** | Yes | No | — |
| **Auto-enabled with sample?** | Yes | No | — |
| **Horizon-exposed workflows** | No | No | Yes (`sample-smoke-test`) |
| **When unused** | Auto-disable (no hard dependents) | Auto-disable (no hard/soft dependents) | Stays while you want the app |

#### Change git ref in developer portal

Git reference can be easily changed for the modules. It can be done from Administration tab in Modules section. After the change the Target Revision for this particular app should be aligned automatically in ArgoCD.

### Workflows via Developer Portal

#### Submit

The module must be enabled to run the workflow.

Open "sample" tab in developer-portal and go to "Workflow Templates".

Currently there are 2 workflows available: _sample-smoke-test_ and _sample-smoke-test-alt_.

**Available parameters:**

- horizonSubmittedFrom*
- sampleEnv*
- sampleBuildId*
- sampleNote*

After the module is run it can be found in "Running Workflow" tab. To open the running workflow and see the details, click on the name of the workflow. From here it is possible to view logs, download artifacts or move to GCS.

#### History

When the workflow is completed it is moved to "History" tab, where all the details can be found. The workflow will be visible there for 1 day.

The details of the available workflows can be also found in "Argo Workflows" → "Workflow Templates" → "workflows".

The artifacts can also be located and downloaded from GCP "Cloud Storage" → "Buckets" → "*-argo-workflows".

#### How to abort/delete workflows

To abort/deleve a workflow, open the running workflow in Developer Portal and press “Abort” on top-right corner. “Abort” button should change to “Delete”, status should be shown as “Aborted”. Workflow can be found in History tab with correct status.

To delete the workflow, go to history tab, open the details of workflow and press “Delete”.

#### Hello-world application

_Hello World_ application can be found in "Developer Portal" → "sample tab" → "Applications", where it can be opened directly. It should show _SOFT_FEATURE_ENABLED_SAMPLE_SOFT_ flag with a correct value:

- true — if sample-soft is enabled
- false — if sample-soft is disabled

### Workflows via Horizon CLI

Replace placeholder values (marked with `<...>`) with your environment-specific values before running.

#### Installation

##### Download from Developer Portal (recommended)

Sign in to Developer Portal (`https://<your-horizon-domain>/developer-portal/`) and open **Tools** in the sidebar (or Welcome → Tools).

1. Download the binary for your OS and architecture.
2. Linux: `chmod +x horizon` and move it to your `PATH` (e.g.: `sudo mv horizon /usr/local/bin/horizon`).
3. macOS (Apple Silicon / arm64): `chmod +x horizon`, remove the quarantine attribute (`xattr -d com.apple.quarantine horizon`) to avoid macOS Gatekeeper warnings for the unsigned binary, and move it to your `PATH` (e.g.: `sudo mv horizon /usr/local/bin/horizon`).
4. Windows: keep `horizon.exe` on your `PATH`. Windows SmartScreen may warn because the file is unsigned.
5. Verify:

```bash
horizon version
```

**Expected:** The version and the build date match the values displayed on the Tools page. The build date is displayed directly below the version.

##### Contributor: build from source

Go [1.22+](https://go.dev/dl/) must be installed. Verify with:

```bash
go version
```

**Expected:** Output shows `go1.22` or higher.

##### Build the binary

From the repository root:

```bash
cd tools/horizon
go build -o horizon .
```

**Expected:** No errors. A `horizon` binary is created in the current directory.

##### Move to PATH

```bash
sudo mv horizon /usr/local/bin/
```

Or a user-local install can be used instead:

```bash
mv horizon ~/go/bin/
```

##### Verify installation

```bash
horizon --help
```

**Expected:** Usage text is printed, starting with:

```
horizon - Horizon API CLI

Usage:
  horizon config <init|get|set> ...
  horizon auth <login|logout|refresh|whoami> [flags]
  horizon catalog get [--output text|json] [flags]  (default output: text)
  horizon workflow <submit|wait|logs|abort|delete|show|list|get|running|history|download-artifact> ... [flags]
```

##### Alternative: install via `go install`

From the repository root:

```bash
go install ./tools/horizon
```

This places the binary in `$GOBIN` (defaults to `~/go/bin`). Ensure that directory is on your `PATH`.

#### Environment Setup

Before running any test, set your Horizon domain:

```bash
export HORIZON_DOMAIN="<your-horizon-domain>"
```

Or use `--domain` on each command instead.

#### Authentication and Catalog Access

##### Preconditions

- CLI installed (`horizon --help` works)
- Keycloak client configured for the target domain

##### Log in via device flow

```bash
horizon auth login --device --domain "$HORIZON_DOMAIN" --write-config
```

A verification URL is printed to stderr. Open it in a browser and complete the sign-in.

**Expected output (stderr):**

```
Open this URL in a browser (sign in with your Horizon user):
https://<domain>/auth/realms/horizon/...
...
Logged in. Token saved to ~/.config/horizon/token.json
Wrote defaults to /home/<user>/.config/horizon/config.yaml
```

##### Verify token was stored

```bash
ls -la ~/.config/horizon/token.json
```

**Expected:** File exists with permissions `0600`.

##### Verify identity

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

##### List the catalog

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

##### Catalog as JSON (optional verification)

```bash
horizon catalog get --output json
```

**Expected:** Valid JSON with an `"entries"` array containing objects with `"module"`, `"templateName"`, and `"parameters"` fields.

#### Workflow Submit with Submitted-From Label

##### Preconditions

- The user is authenticated.
- A valid module, template, and parameters are known.
- Adjust the `--module`, `--template`, and `--params-json` values below to match a template from your catalog.

##### Submit a workflow

```bash
horizon workflow submit \
  --module sample \
  --template sample-smoke-test \
  --params-json '{"sampleEnv":"cli-test","sampleBuildId":"tcli02-001","sampleNote":"test-run"}' \
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

```bash
export WF_NAME="<generated-workflow-name>"
```

##### Inspect the workflow for submitted-from

```bash
horizon workflow show "$WF_NAME" --output json | grep -i submittedFrom
```

**Expected output:** The JSON includes `"submittedFrom"` with value `"horizon-cli"`:

```
"submittedFrom": "horizon-cli",
```

This confirms the CLI sent the `X-Horizon-Submitted-From: horizon-cli` header and the `horizonSubmittedFrom` parameter.

##### Verify via text output

```bash
horizon workflow show "$WF_NAME"
```

**Expected:** The summary block under "Workflow detail" includes a `"submittedFrom": "horizon-cli"` entry.

#### Logs, Abort, and Artifact Download

##### Preconditions

- A workflow submitted via the CLI exists (from T-CLI-02, or submit a new one). 
- `WF_NAME` is set.

##### Stream logs

```bash
horizon workflow logs "$WF_NAME"
```

**Expected:** Log lines stream to stdout in the format `[stageName] [timestamp] [message]`. The stream ends with:

```
━━ log stream end ━━
```

If the workflow has already completed, historical logs are printed (non-follow mode). If pods have not started yet, the CLI waits up to 60 seconds before opening the stream.

##### Abort a running workflow

Run this while the workflow is still active (phase is not Succeeded/Failed/Error/Aborted):

```bash
horizon workflow abort "$WF_NAME"
```

**Expected output (stderr):**

```
Aborting workflow <WF_NAME> via Horizon API ...
```

If the workflow has already reached a terminal phase, the command exits silently with code 0 (no error).

Verify the abort took effect:

```bash
horizon workflow show "$WF_NAME" --output json | grep '"phase"'
```

**Expected:** `"phase": "Aborted"` (may take a few seconds to transition).

##### Retrieve an artifact signed URL

This requires a completed workflow that produced output artifacts. First check for available artifacts:

```bash
horizon workflow show "$WF_NAME"
```

Look for `outputArtifact:` lines in the output. If artifacts exist, generate a signed URL:

```bash
horizon workflow download-artifact "$WF_NAME" "<artifact-name>" --generate-signed-url
```

**Expected output (stdout):**

```
signed-url: https://storage.googleapis.com/...
```

##### Download an artifact to a file (optional)

```bash
horizon workflow download-artifact "$WF_NAME" "<artifact-name>" -o ./artifact-output
```

**Expected:** A progress bar on stderr followed by a completion message. The file `./artifact-output` is created with the artifact contents.

#### Cleanup

After the work is done, optionally log out and remove local state:

```bash
horizon auth logout
```

**Expected output (stderr):**

```
Removed ~/.config/horizon/token.json (if present).
```

#### Uninstall

##### Remove the binary

```bash
sudo rm "$(which horizon)"
```

Expected: No errors. Verify with:

```bash
which horizon
```

Expected: No output, or `horizon not found`.

##### Remove configuration and cached tokens

```bash
rm -rf ~/.config/horizon/
```

This deletes both `config.yaml` and `token.json`.

##### Verify removal

```bash
ls ~/.config/horizon/
```

**Expected:**

```
ls: cannot access '/home/<user>/.config/horizon/': No such file or directory
```

### Expose label (catalog and portal links)

The Horizon platform supports controlled visibility of **WorkflowTemplates** and child applications in the Developer Portal using annotations. These annotations determine whether resources are displayed in the catalog, portal navigation, and Module Manager views.

#### 1. WorkflowTemplate Visibility in the Developer Portal

WorkflowTemplates can be exposed in the Developer Portal by adding the following annotation to the WorkflowTemplate manifest:

```yaml
horizon-sdv.io/expose: "true"
```

When this annotation is enabled and the corresponding module is active, the WorkflowTemplate becomes visible in the **Workflow Templates** section of the Developer Portal.

If the annotation is removed or set to `"false"`, the WorkflowTemplate is hidden from the portal UI. The underlying Kubernetes or Argo resource may still exist in the cluster, but it will no longer be accessible from the portal.

##### Updating WorkflowTemplate Visibility

1. Open the WorkflowTemplate manifest in Argo Workflows or the GitOps source repository.
2. **Add** or **update** the annotation:

   ```yaml
   horizon-sdv.io/expose: "true"
   ```

3. Save the changes and sync the application.

To **hide** the WorkflowTemplate, either remove the annotation or change the value to:

```yaml
horizon-sdv.io/expose: "false"
```

After synchronization, refresh the Developer Portal to verify the visibility changes.

#### 2. Exposing Child Applications in the Portal Navigation

Child applications can be displayed in the Developer Portal navigation and Module Manager merged applications list using the following annotations:

```yaml
horizon-sdv.io/expose: "true"
horizon-sdv.io/portal-url: "<valid-url>"
```

Both annotations are required for the application link to appear in the portal.

- `horizon-sdv.io/expose` controls whether the application is eligible for portal visibility.
- `horizon-sdv.io/portal-url` defines the navigation link displayed in the portal.

##### Configuring Application Visibility

1. Open the child application in Argo CD.
2. Navigate to **Details → Annotations.**
3. Add or verify the following annotations:

   ```yaml
   horizon-sdv.io/expose: "true"
   horizon-sdv.io/portal-url: "https://<application-url>"
   ```

4. Save the changes and sync the application.

Once synchronization is complete, the application becomes visible in:

- Developer Portal navigation
- Module Manager merged applications list

##### Removing Application Visibility

To remove the application link from the portal:

- Remove `horizon-sdv.io/portal-url`
- OR set:

  ```yaml
  horizon-sdv.io/expose: "false"
  ```

After synchronization and portal refresh, the application link is removed from the UI.

#### 3. Visibility Restrictions Based on Module Enablement

WorkflowTemplate visibility also depends on module or soft-feature availability.

Even when the following annotation is configured:

```yaml
horizon-sdv.io/expose: "true"
```

the WorkflowTemplate is only accessible if the related module or feature is enabled and in a READY state.

If the module or feature is disabled:

- The WorkflowTemplate is removed from the Developer Portal UI
- Workflow submission requests are blocked
- Access attempts may return responses such as: `404 Not Found`

This behavior ensures that exposed resources are only accessible when the owning module or feature is enabled and available to users.