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

# Gemini AI Assistant utility

Jenkins job that runs the Gemini AI assistant CLI against user-provided artifacts. You supply a command to fetch artifacts (e.g. from GCS) and a prompt file; the job runs the CLI in headless mode and archives the output.

## Parameters

- **GEMINI_ARTIFACTS_COMMAND** (required) – Command to populate the workspace with content to analyse (e.g. `gcloud storage cp -r gs://bucket/path/ .`).
- **GEMINI_PROMPT_FILE** (required) – Step 1 prompt file (upload only).
- **GEMINI_PROMPT_FILE_2** (optional) – Step 2 prompt file (upload). Required only for sequenced analysis; when set with step 1, runs two or three steps with context chaining (order matters). We do not ship single-prompt files.
- **GEMINI_PROMPT_FILE_3** (optional) – Step 3 prompt file (upload). For three-step sequenced analysis.
- **GEMINI_COMMAND_LINE** – Full Gemini CLI invocation (e.g. `gemini --yolo --output-format json`). To pin the model and avoid auto-routing to preview-only models, add `--model <model name>` (e.g. `--model gemini-3.8-flash`).
- **GEMINI_SKILLS_YAML** (optional) – Upload `skills.yaml` (file parameter, like prompts). When provided, the job decodes it and converts to `.gemini/skills/*/SKILL.md` before analysis. Use when prompts are uploaded so the job cannot auto-detect the prompt dir.

Prompts are provided only via upload; there are no workspace path parameters.

### Why `skills.yaml` and `gemini_skills_from_yaml.py` (not only `SKILL.md` files)

The Gemini CLI discovers skills under **`.gemini/skills/<skill-name>/SKILL.md`**, with YAML frontmatter (`name`, `description`) and a Markdown body. You *could* maintain those directories by hand and skip the converter.

This repo uses **`skills.yaml` plus `gemini_skills_from_yaml.py`** because:

- **Single source of truth** – One file holds **global_constraints** (prepended to every skill) and all step skills. Shared rules are not duplicated or edited in three separate Markdown files.
- **Structured fields** – Per-skill `output_schema`, `system_instructions`, and descriptions stay explicit; the script appends **Expected Output Format** from `output_schema` consistently.
- **Automation** – `gemini_initialise.sh` runs the converter when `skills.yaml` is present (or when `GEMINI_SKILLS_YAML` is set), so `.gemini/skills/` is regenerated before each run with no manual copy step.
- **Upload-friendly jobs** – For this utility, **`GEMINI_SKILLS_YAML`** can be supplied as base64 or a path; generating the CLI layout from YAML is simpler than uploading three separate `SKILL.md` trees.

The CLI expects **one directory per skill**, not one monolithic `skills.md`. A single mega-file would still need splitting to match that layout; YAML keeps editing in one place while matching the expected on-disk shape.

## Artifacts

- `gemini-assist/*` – Analysis output (e.g. proposed fixes).
- `headless_output*.json` – Raw CLI output.
- `step1_output.md`, `step2_output.md`, `step3_output.md` – Extracted step outputs when using sequenced prompts.
- `gemini-client-error.zip` – Error report if the CLI fails.

## Offline analysis & prompt/skill iteration

Use this utility against archived artifacts from any pipeline — Build, CVD Launcher, CTS Execution — to analyse passes or failures without re-running the original job, and as a sandbox for prompt and `skills.yaml` development.

- **Pick your artifacts.** Set `GEMINI_ARTIFACTS_COMMAND` to fetch the archived workspace or `test-results/` of the run you want to investigate (success or failure). Examples:
  - `gcloud storage cp -r gs://${ANDROID_BUILD_BUCKET_ROOT_NAME}/Android/Tests/CVD_Launcher/<BUILD_NUMBER>/test-results/ .`
  - `gcloud storage cp -r gs://${ANDROID_BUILD_BUCKET_ROOT_NAME}/Android/Tests/CTS_Execution/<BUILD_NUMBER>/test-results/ .`
- **Pick the matching prompts** from the relevant pipeline's `prompt/sequenced/` directory (`step1_triage.txt` → `GEMINI_PROMPT_FILE`, `step2_rca.txt` → `_2`, `step3_fixes.txt` → `_3`) and its `skills.yaml` → `GEMINI_SKILLS_YAML`.
- **For Cuttlefish/virtual-device focus, always use the CVD Launcher prompts** (not the CTS Execution set). Their Phase 0 boot preflight classifies `CVD_STATUS` (`BOOT_OK` / `BOOT_FAILED` / `BOOT_UNKNOWN`) from artifact-only signals (`"status":"Running"` in host CVD JSON, `VIRTUAL_DEVICE_BOOT_COMPLETED` per guest), so the same prompts auto-route between the runtime-health lane and the boot-failure lane — covering CVD logs whether captured by CVD Launcher or by CTS Execution.
- **Iterate on prompts and skills.** Edit `step*_<role>.txt` or `skills.yaml` locally, re-upload, re-run against the same archived artifacts, and compare `step*_output.md` until happy — then promote the changes back into the source pipeline's `prompt/sequenced/`.

## See also

- Scripts: `workloads/common/agentic-ai/gemini/` (e.g. `gemini_analysis.sh`).
- AAOS Builder AI review: `workloads/android/pipelines/builds/aaos_builder/` (sequenced prompts for deep scan).
- CVD Launcher AI Review: [`docs/workloads/android/tests/cvd_launcher.md`](../android/tests/cvd_launcher.md#gemini_analyse_on_success).
- CTS Execution AI Review: [`docs/workloads/android/tests/cts_execution.md`](../android/tests/cts_execution.md#gemini_analyse_on_success).
