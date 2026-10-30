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

# Workstation images

`cws-image-build` builds the container images used by workstation configurations. Run it from the Developer Portal. Only one image build runs at a time.

You may see `horizonSubmittedFrom`. Leave it blank. The Portal already records that the run came from the Developer Portal. If you set it, the only accepted values are `developer-portal`, `api`, and `horizon-cli`.

`true` and `false` must be those words.

## What each image is

Images are built in this order. An image that is turned off is skipped.

1. **Environment** (`buildEnvironmentImage`). The image every other Cloud Workstations workflow runs on. It is not a workstation image. Build it once per environment before running any other workflow, on its own, with `imageTag` `latest` and every other `build...` field `false`. Until it exists, those workflows stay Pending with `ImagePullBackOff`. Leave this `false` unless you are building or updating it.
2. **Preflight** (`buildPreflight`). The base image.
3. **GNOME** (`buildGnome`). Built on preflight.
4. **Code OSS** (`buildCodeOss`). Built on its own, not on GNOME.
5. **Android Studio** (`buildAndroidStudio`) and **ASfP** (`buildAsfp`). Built on GNOME. They can run in the same build.

When the run pushes images, the log ends with an `image@sha256:...` address for each image that was built. Paste that address into `containerImage` on [create or update configuration](../config/_README.md). The shortcuts `C-OSS (VCS)`, `AS`, and `ASfP` point at the Code OSS, Android Studio, and ASfP images from this build.

## Parameters

| Parameter | Starts as | What it does |
|-----------|-----------|----------------|
| `imageTag` | Empty, required | Tag applied to every image built in this run. `-` is not accepted as empty. |
| `dryRun` | `false` | `true` builds without pushing. See the limits below. |
| `buildEnvironmentImage` | `false` | `true` builds the image the other workflows run on. Use `imageTag` `latest`. |
| `buildPreflight` | `true` | `true` builds preflight. `false` reuses `preflightImage`. |
| `preflightImage` | `-` | Existing preflight address when `buildPreflight` is `false`. `-` or blank means unused. |
| `buildGnome` | `true` | `true` builds GNOME on preflight. `false` reuses `gnomeImage`. |
| `gnomeImage` | `-` | Existing GNOME address when `buildGnome` is `false`. `-` or blank means unused. |
| `buildCodeOss` | `true` | `true` builds Code OSS. |
| `buildAndroidStudio` | `true` | `true` builds Android Studio on GNOME. |
| `buildAsfp` | `true` | `true` builds ASfP on GNOME. |
| `installAntigravityAgent` | `false` | `true` installs Antigravity into GNOME. Used only when GNOME is built in this run. |

At least one `build...` field must be `true`.

## When a base image must be named

- Building GNOME without building preflight in the same run requires `preflightImage`.
- Building Android Studio or ASfP without building GNOME in the same run requires `gnomeImage`.

## Dry run

`dryRun` `true` does not push, so a later image in the same run cannot pull what this run just built.

- Do not build preflight and GNOME together.
- Do not build GNOME together with Android Studio or ASfP.

Build those as separate runs, or set `dryRun` to `false`.
