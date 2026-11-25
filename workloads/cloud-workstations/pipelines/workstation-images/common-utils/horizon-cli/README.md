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

# Horizon CLI on Cloud Workstation images

## Table of contents
- [Introduction](#introduction)
- [Scope](#scope)
- [Installation](#installation)
- [Configuration](#configuration)
- [Start a workstation](#start-a-workstation)
- [Use the CLI](#use-the-cli)

## Introduction <a name="introduction"></a>

A Cloud Workstation user can trigger Horizon workloads from the workstation terminal using the Horizon CLI. The CLI is built into the image, so there is no download or install. Before you submit a workflow, set `HORIZON_DOMAIN` (or pass `--domain`) and run `horizon auth login`. Only the MCP client gets `HORIZON_DOMAIN` at image build; the CLI does not.

`horizon` is installed by default at `/usr/local/bin/horizon`. Jenkins image jobs and the `cws-image-build` workflow use the same Dockerfiles. Command reference: [Horizon CLI](../../../../../../docs/architecture/03_horizon-cli.md).

## Scope <a name="scope"></a>

| Image | CLI |
| --- | --- |
| `horizon-gnome` | Installed at image build |
| `horizon-code-oss` | Installed at image build. Not a GNOME child |
| `horizon-android-studio`, `horizon-asfp` | Inherited from published `horizon-gnome` (`FROM ${BASE_IMAGE}`). Not installed again |
| `horizon-preflight` | Not installed. It is an Alpine export layer, not a user shell |

## Installation <a name="installation"></a>

The CLI is a default install on `PATH`. `install-horizon-cli.sh` runs when `INSTALL_HORIZON_CLI` is `true` (Dockerfile default). It installs `/usr/local/bin/horizon` and runs `horizon version`. The Dockerfile then checks `test -x /usr/local/bin/horizon`.

The script accepts one source, in this order: `HORIZON_CLI_BIN` (pre-built binary; what the Dockerfiles pass), `HORIZON_CLI_SRC` (`tools/horizon`; needs `go`), or `HORIZON_CLI_URL` (optional `HORIZON_CLI_SHA256`).

`horizon-gnome` and `horizon-code-oss` compile `tools/horizon` in a `golang:1.27.2-alpine` stage. The version is `components.horizoncore` in `versions.yaml`. The build does not call Developer Portal `/api/cli`.

The CLI does not start with the workstation. `horizon version` and `horizon --help` print the version and banner.

## Configuration <a name="configuration"></a>

`GOOGLE_CLOUD_PROJECT` is set in the image. `HORIZON_DOMAIN` is passed into the image build (Jenkins `HORIZON_DOMAIN`, or Argo `config.domain`) and stored as the MCP client default. It is not exported for the CLI. Set it in the shell, or save it with `horizon auth login --write-config`.

## Start a workstation <a name="start-a-workstation"></a>

Run these from the Developer Portal. The cloud-workstations module must already be enabled. Names are lowercase letters, digits, and `-`.

1. **Build the image** with `cws-image-build`. Set `dryRun` to `false`, `imageTag` to `latest`, and only the image you need (`buildCodeOss`, `buildGnome`, `buildAndroidStudio`, or `buildAsfp`). Leave the other `build...` fields `false`. If `buildGnome` is `true`, also set `buildPreflight` to `true` or pass `preflightImage`. If you build Android Studio or ASfP without GNOME in the same run, pass `gnomeImage`. Copy the `image@sha256:...` address from the log.
2. **Create a configuration** with `cws-create-config`. Set `cloudWsConfigName`, `wsAdminIamMembers` to your email, and `containerImage` to that address (`containerImageTag` is ignored for a full address). `C-OSS (VCS)`, `AS`, and `ASfP` select the published Code OSS, Android Studio, and ASfP images instead.
3. **Create the workstation** with `cws-admin-create-workstation`. Set `workstationId`, `configName`, and `users` to the email you sign in with. `cws-user-start-workstation` only starts a workstation that lists you; `cws-user-list-usable-workstations` lists those workstations.
4. **Start it** with `cws-user-start-workstation` and the same `workstationId`. Open the URL from the log, then open a terminal.

## Use the CLI <a name="use-the-cli"></a>

On that workstation:

```bash
which horizon
horizon version
export HORIZON_DOMAIN="<your-horizon-domain>"
horizon auth login --device --write-config
horizon catalog get
horizon workflow submit --module <module> --template <template> --params-json '<json>' --wait --logs
horizon workflow download-artifact -o ./out.bin <workflow-name> <artifact>
```

`which horizon` prints `/usr/local/bin/horizon`. `--wait --logs` streams the run and exits with its result. `download-artifact` collects a finished artifact.