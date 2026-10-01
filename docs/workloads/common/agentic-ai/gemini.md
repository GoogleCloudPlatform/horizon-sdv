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

# Gemini CLI integration

Scripts and environment for running the [Gemini CLI](https://geminicli.com/docs/cli/headless/) in headless mode (e.g. from Jenkins or Argo). Used by the AAOS Builder AI review step and the Gemini AI Assistant utility.

## Argo Workflows and Jenkins

**Entrypoint:** **`workloads/common/agentic-ai/gemini/run_ai_review.sh`** — same contract for **Argo** (**aaos-builder** inline **`gemini-review`**), **Jenkins** (AAOS, ABFS, OpenBSW, **`gemini_ai_assistant`**), and **CVD/CTS** (**`cvdPipeline.groovy`**). Callers set **`PIPELINE_REPO_ROOT`** (or **`WORKSPACE`** / **`REPO_ROOT`**), **`GEMINI_*`**, **`CLOUD_*`**, optional **`GEMINI_HOOK_DIR`** / **`GEMINI_HOOK_PROFILE`**, optional **`GEMINI_ARTIFACTS_COMMAND`** / **`GEMINI_POST_REVIEW_COMMAND`**, and **`GEMINI_SKIP_MOVE_ARTIFACTS`** when outputs already live under **`WORKSPACE`**.

**Argo Workflows:** **aaos-builder**, **cvd-launcher**, and **cts-execution** run **`run_ai_review.sh`** from an inline **`gemini-review`** template in each workflow chart (native **`volumeMounts`** for workflow PVCs — **`templateRef`** does not merge per-step mounts on this controller). CVD/CTS first run **`gemini_argo_prepare_staging.sh`** on PVC **`gemini-test-results`**, then **`gemini-review`** with **`GEMINI_SKIP_MOVE_ARTIFACTS=1`** and profile **`cvd`** / **`cts`** post-hooks. See **[tests README](../../../../workloads/android/pipelines/tests/README.md)** and chart READMEs under **`workloads/android/pipelines/tests/*/helm/`**.

### Argo integration (inline templates)

| Step | Where |
|------|--------|
| Prepare staging (CVD/CTS) | **`workloads/android/pipelines/tests/gemini_argo_prepare_staging.sh`** — **`prepare-gemini-cvd`** / **`prepare-gemini-cts`** in each chart’s **`_gemini.tpl`** |
| **`gemini-review`** pod | Inline in **`aaos_builder/helm/templates/workflow/_gemini-review.tpl`**, **`cvd_launcher/helm/.../_gemini.tpl`**, **`cts_execution/helm/.../_gemini.tpl`** — all call **`run_ai_review.sh`** |
| Cluster **`ai-review`** + **`templateRef`** | Not used (AAOS, CVD, and CTS inline only on this controller) — see **[AI Review](../../guides/workload_usage.md#ai-review)** |

## Scripts

| Script | Purpose |
|--------|--------|
| `gemini_environment.sh` | Sets and exports variables (prompt file, command line, artifact path, preview/location). Source this before other scripts. |
| `gemini_initialise.sh` | Cleans artifacts, installs/upgrades gemini-cli, writes `.gemini/settings.json`. If skills.yaml is found (`GEMINI_SKILLS_YAML` or `skills.yaml` next to prompt path), runs `gemini_skills_from_yaml.py` to populate `.gemini/skills/`. Skills file is always named `skills.yaml`. In Jenkins / `CI=true` / Argo workflow runs, copies every `*.toml` under `gemini/policies/` into `.gemini/policies/` (see [Workspace policy files](#workspace-policy-files-toml)). |
| `gemini_analysis.sh` | One prompt → single run. Two or three prompts → **sequenced**: **one Gemini CLI invocation per step**, each with its own JSON output file (`headless_output_stepN_<timestamp>_<random>.json`). Prior-step context is file-based (`stepN_output.md`). Step 3 includes **both** `step1_output.md` and `step2_output.md` when present (optional byte caps). Set `GEMINI_PROMPT_FILE` (step 1), optionally `GEMINI_PROMPT_FILE_2` and `GEMINI_PROMPT_FILE_3`. Default when unset: AAOS three-step. Requires `jq` for extraction. See [Sequenced analysis](#sequenced-analysis). |
| `run_ai_review.sh` | **Shared runner:** reads **`GEMINI_*`**, **`CLOUD_*`**, repo root (**`PIPELINE_REPO_ROOT`** / **`WORKSPACE`** / **`REPO_ROOT`**), optional hooks, **`gemini_initialise.sh`** + **`gemini_analysis.sh`**, optional **`move_gemini_artifacts`**, client-error zip handling. |
| `workloads/android/pipelines/tests/gemini_argo_prepare_staging.sh` | **Argo only (CVD/CTS):** merge ephemeral VM artifacts into **`/workspace/test-results`** on the **`gemini-test-results`** PVC before **`gemini-review`**. Set **`GEMINI_PREPARE_MODE`** to **`cvd`** or **`cts`**. Invoked from **`prepare-gemini-cvd`** / **`prepare-gemini-cts`** in chart **`_gemini.tpl`**. |
| `review-pre-<profile>.sh`, `review-post-<profile>.sh` | Optional hooks under **`${REPO_ROOT}/${GEMINI_HOOK_DIR}`** when **`GEMINI_HOOK_DIR`** is set. **`run_ai_review.sh`** dispatches by **`GEMINI_HOOK_PROFILE`**. **AAOS** uses profile **`aaos`**; **CVD/CTS Argo** use **`cvd`** / **`cts`** (**`review-post-*.sh`** under **`cvd_launcher/hooks`** and **`cts_execution/hooks`** runs **`gemini_storage.sh`** after analysis). |

## Key environment variables

- **PIPELINE_REPO_ROOT** / **WORKSPACE** / **REPO_ROOT** – Monorepo root for **`run_ai_review.sh`**; **`PIPELINE_REPO_ROOT`** is preferred in Argo, **`WORKSPACE`** in Jenkins.
- **GEMINI_ANALYSIS_PATH** – Working directory for Gemini analysis (default `/workspace` in the script).
- **GEMINI_HOOK_DIR**, **GEMINI_HOOK_PROFILE** – Optional pre/post hooks (see Scripts table); AAOS uses **`workloads/android/pipelines/builds/aaos_builder/hooks`** and profile **`aaos`**.
- **GEMINI_ARTIFACTS_COMMAND** – Optional shell snippet **`eval`**’d at the start of **`run_ai_review.sh`** (e.g. unzip Cuttlefish logs for CVD).
- **GEMINI_POST_REVIEW_COMMAND** – Optional snippet **`eval`**’d after move / empty JSON cleanup, before **`gemini-client-error.zip`** handling (e.g. **`gemini_storage.sh`** for CTS).
- **GEMINI_SKIP_MOVE_ARTIFACTS** – Set to **`1`** to skip **`move_gemini_artifacts`** when outputs are already under **`WORKSPACE`** (Argo CVD/CTS set this after **`gemini_argo_prepare_staging.sh`**).
- **GEMINI_PREPARE_MODE** – **`cvd`** or **`cts`** for **`gemini_argo_prepare_staging.sh`** only (Argo prepare step).
- **GEMINI_AI_EXECUTION_TIMEOUT_HOURS** – Hours for **`gemini_analysis.sh`**; falls back to **`GEMINI_AI_EXECUTION_TIMEOUT`** or **`2`**.
- **GEMINI_AAOS_LOG_TAIL_MODE** – **`tail`** (default) or **`fullcopy`** for **`aaos-build.log`** → **`aaos-build.log.tail`** in **`review-pre-aaos.sh`** (ABFS uses **`fullcopy`**).
- **GEMINI_PROMPT_FILE** – Step 1 prompt file path or base64-encoded content.
- **GEMINI_PROMPT_FILE_2** – Optional step 2 prompt; required only for sequenced (two or three steps).
- **GEMINI_PROMPT_FILE_3** – Optional step 3 prompt (do not reorder).
- **GEMINI_COMMAND_LINE** – Full CLI invocation (e.g. `gemini --yolo --output-format json`). To pin the model and avoid auto-routing to preview-only models, add `--model <model name>` (e.g. `--model gemini-2.5-pro`).
- **GEMINI_PREVIEW_FEATURES**, **GEMINI_LOCATION_GLOBAL** – Passed into `.gemini/settings.json` and location.
- **GEMINI_SKILLS_YAML** – Optional path to `skills.yaml`. When set, `gemini_initialise.sh` converts it to `.gemini/skills/*/SKILL.md` (requires Python PyYAML). The skills file is always named `skills.yaml`.
- **GEMINI_ARTIFACT_PATH** – Directory for analysis output (default `gemini-assist`).
- **GEMINI_OUTPUT_FILE_NAME** – JSON path for the CLI’s `--output-format json` sink. In **sequenced** runs, `gemini_analysis.sh` **overwrites** this per step with `headless_output_stepN_*.json`; do not rely on a single global name across steps.
- **GEMINI_STEP2_PRIOR_CONTEXT_BYTES** – If set to a positive integer, maximum bytes of `step1_output.md` appended into the **step 2** composed prompt (`head -c`). Unset or `0` = append full step 1 text. Some jobs set `131072` to limit prompt size.
- **GEMINI_STEP3_PRIOR_STEP1_BYTES** – Maximum bytes of `step1_output.md` inlined into the **step 3** composed prompt (when step 3 includes both prior steps). Unset or `0` = full file.
- **GEMINI_STEP3_PRIOR_STEP2_BYTES** – Maximum bytes of `step2_output.md` for the same step 3 composed prompt. Unset or `0` = full file.
- **GEMINI_CLI_TRUST_WORKSPACE** – Default `true`. Bypasses the headless [Folder Trust](https://geminicli.com/docs/cli/trusted-folders/) check so the CLI loads our workspace `.gemini/settings.json` (auth `selectedType=vertex-ai`, policies, skills). Without this, recent Gemini CLI versions run in restricted *safe mode* and ignore workspace settings, surfacing as `Please set an Auth method ... GEMINI_API_KEY, GOOGLE_GENAI_USE_VERTEXAI, GOOGLE_GENAI_USE_GCA` even though `GOOGLE_GENAI_USE_VERTEXAI=True` is exported. Equivalent to passing `--skip-trust` on the CLI; set to `false` only if you intentionally want safe mode.
- **TERM** – Forced to `xterm-256color` when unset or `dumb` (Jenkins `sh` steps export `TERM=dumb`; Argo leaves it unset). Silences the CLI's `Warning: Basic terminal detected (TERM=dumb)...` and `Warning: 256-color support not detected...` advisories. The checks are on terminal capability, not colour output, so `NO_COLOR` alone does not silence them. Set to any other non-`dumb` value in the caller to override.
- **COLORTERM** – Default `truecolor`. Standard advertisement for 24-bit colour; silences the CLI's `Warning: True color (24-bit) support not detected...` warning in headless runs.
- **NO_COLOR** – Default `1`. Standard [`NO_COLOR`](https://no-color.org) opt-out that suppresses ANSI styling in the CLI's stderr/stdout, keeping the JSON output clean for `jq`. Set to empty (`NO_COLOR=`) in the caller to re-enable colour.

## Sequenced analysis

`gemini_analysis.sh` treats each step as a **separate** headless CLI run (stdin = composed prompt for that step only). There is no single growing chat session in the shell; **handoff** is via files on disk.

**Benefits of this separation (vs one ambiguous multi-step blob):**

- **Clearer artifacts:** Each step has its own `headless_output_stepN_*.json` and matching `stepN_output.md`, so extraction and debugging target the **failing step** without guessing which JSON is “newest.”
- **Failure isolation:** If step 2 or 3 fails, step 1 output is still on disk for replay or manual review; you are not forced to redo triage unless step 1 itself failed.
- **Tunable context per step:** Byte caps apply **per handoff** (`GEMINI_STEP2_*`, `GEMINI_STEP3_PRIOR_*`), so you can trim **RCA** or **fix** inputs independently instead of one oversized combined prompt.
- **Richer fix step:** Step 3 can include **both** triage and RCA markdown explicitly, so remediation sees **structured** prior outputs rather than only whatever step 2 chose to repeat from step 1.

**Trade-off:** Step 3’s composed prompt can be **larger** than the older “append only step 2” behavior unless you use the step 3 byte caps—see [Prompt size and token budget](#prompt-size-and-token-budget-adjust-measure-set).

| Step | Composed input | Raw JSON output |
|------|----------------|-----------------|
| 1 | Step 1 prompt file only | `headless_output_step1_<timestamp>_<random>.json` |
| 2 | Step 2 prompt + `## Context from previous step(s)` + contents of `step1_output.md` (optional cap: `GEMINI_STEP2_PRIOR_CONTEXT_BYTES`) | `headless_output_step2_*.json` |
| 3 | Step 3 prompt + context sections for **`step1_output.md`** and **`step2_output.md`** when both exist (optional caps: `GEMINI_STEP3_PRIOR_STEP1_BYTES`, `GEMINI_STEP3_PRIOR_STEP2_BYTES`). If those files are missing, falls back to appending only the immediate prior step file as in older behavior. | `headless_output_step3_*.json` |

After each successful step, the script extracts the model text from **that step’s JSON** into `step1_output.md` / `step2_output.md` / `step3_output.md` (using `jq`). Step outputs are also copied into `GEMINI_ARTIFACT_PATH` (default `gemini-assist/`). Stray `*proposed_fix*.md` files in the working directory are moved into `GEMINI_ARTIFACT_PATH` after each successful step.

**Downstream tooling:** Jobs that assume **exactly one** `headless_output*.json` per run should be updated to expect **one JSON file per sequenced step** (or to select by glob / newest as appropriate).

### Prompt size and token budget (adjust, measure, set)

**What the repo controls:** `gemini_analysis.sh` limits **how many bytes** of prior-step markdown are **appended** into the composed prompt for step 2 and step 3 (`GEMINI_STEP2_PRIOR_CONTEXT_BYTES`, `GEMINI_STEP3_PRIOR_STEP1_BYTES`, `GEMINI_STEP3_PRIOR_STEP2_BYTES`). It does **not** cap the step prompt files themselves, skills content, or the Gemini CLI’s own system overhead. Billing and **token** usage are reported by **Google Cloud / Vertex** (or the CLI) if your org enables that; the shell script does **not** print token counts.

**Set (where):** Export or inject these variables in the same place you set other `GEMINI_*` vars for the job—e.g. Jenkins pipeline env, Argo workflow parameters, or a wrapper before sourcing `gemini_environment.sh` and calling `gemini_analysis.sh`. Values are **byte counts** (e.g. `131072` ≈ 128 KiB of appended prior-step text per variable).

**Measure (practical):**

- **Rough size of composed input:** After a run, `wc -c step2_composed.txt` / `wc -c step3_composed.txt` in the workspace (these files are written during sequenced runs when prior context is appended). Add the byte size of the base prompt file and account for skills loaded by the CLI separately.
- **Rough token estimate:** There is no single universal rule; for a **ballpark** on Latin-heavy text, **≈ 1 token per 4 characters** is sometimes used, but model tokenizers differ. Use Cloud metrics or API usage dashboards for **authoritative** token counts.
- **Symptoms of oversize:** API errors mentioning **context length**, **max tokens**, or **request too large**; or step failures with no useful `stepN_output.md`. If failures are intermittent, compare **composed file sizes** across runs.

**Tune (suggested order):**

1. If step 2 fails or is slow, set **`GEMINI_STEP2_PRIOR_CONTEXT_BYTES`** (AAOS-style jobs often use `131072` as a starting point).
2. If step 3 hits limits after step 3 started including **both** prior outputs, set **`GEMINI_STEP3_PRIOR_STEP1_BYTES`** and **`GEMINI_STEP3_PRIOR_STEP2_BYTES`** independently (truncate the larger or more repetitive file first).
3. Re-run and compare triage/RCA/fix quality; **increase** caps only where truncation clearly hurts results; **decrease** if limits errors persist.

**Production / roadmap:** Hard enforcement, per-stage budgets, and observability hooks are broader **Agentic AI** / platform work; these variables are the **current** levers in this repository.

## Workspace skills

Skills are populated in `.gemini/skills/` before analysis from **skills.yaml** (always this filename; contains a `skills:` list of `name`, `description`, `system_instructions`). When the job finds `skills.yaml` next to the prompt path—or when `GEMINI_SKILLS_YAML` is set—it converts it to `.gemini/skills/<name>/SKILL.md` via `gemini_skills_from_yaml.py` (requires `pip install pyyaml`).

**Prompt vs skill:** The **prompt** is the *task* (the short invocation for this run). The **skill** is the *instruction* (how to do it: role, procedure, rules, output format). In skills, use "Procedure" or "Steps" for the steps to follow—reserve "task" for the prompt. Keep the prompt minimal; put all behavior in the skill.

## Workspace policy files (TOML)

The Gemini CLI [policy engine](https://geminicli.com/docs/reference/policy-engine) reads rules from **`.toml` files** (not JSON or YAML). User policies default to `~/.gemini/policies/*.toml`; **workspace** policies—scoped to the current working directory—live under **`.gemini/policies/*.toml`**. The upstream doc defines the `[[rule]]` schema (`toolName`, `decision`, `priority`, `interactive`, and so on).

Policy TOML files live under **`workloads/common/agentic-ai/gemini/policies/`** (any **`*.toml`**; the repo ships **`00-ci-headless-shell.toml`**). `gemini_initialise.sh` copies **all** of them into **`.gemini/policies/`** when Vertex auth runs **and** the environment looks like CI: **`JENKINS_URL`** set, **`CI=true`**, or **`ARGO_WORKFLOW_NAME`** set (Argo Workflows). That shipped rule **`allow`s `run_shell_command`** when **`interactive = false`**, so shell is not blocked where the default policy would treat **`ask_user`** as **deny** without a TTY.

## Prompt structure and customisation

**Layout (per workload):** Put prompts under e.g. `workloads/.../prompt/sequenced/`:

| File | Role |
|------|------|
| `step1_triage.txt` | Short task for step 1 (e.g. “Run triage… Follow your skill instructions.”). |
| `step2_rca.txt` | Step 2 task; may mention `step1_output.md` as prior context. |
| `step3_fixes.txt` | Step 3 task; may mention `step1_output.md` and `step2_output.md`. |
| `skills.yaml` | Single source of truth: `skills:` list with `name`, `description`, optional `output_schema`, `system_instructions` per skill. |

**Sequenced run:** See [Sequenced analysis](#sequenced-analysis) for per-step JSON files, step 3 dual context, and token-related environment variables. Fix artifacts go under `GEMINI_ARTIFACT_PATH` (default `gemini-assist/`).

**How to customise:**

| What | How |
|------|-----|
| Different prompts | Set `GEMINI_PROMPT_FILE`, `GEMINI_PROMPT_FILE_2`, `GEMINI_PROMPT_FILE_3` (file path or base64). Do not reorder steps. |
| Different skills | Place `skills.yaml` beside those prompts, or set `GEMINI_SKILLS_YAML` (path or base64). |
| Model / CLI flags | Set `GEMINI_COMMAND_LINE` (e.g. add `--model <name>`). |
| Artifact dir / globs | `GEMINI_ARTIFACT_PATH`, `GEMINI_ARTIFACT_FILES_WILDCARD` (see `gemini_environment.sh`). |
| Limit prompt size (sequenced) | `GEMINI_STEP2_PRIOR_CONTEXT_BYTES`, `GEMINI_STEP3_PRIOR_STEP1_BYTES`, `GEMINI_STEP3_PRIOR_STEP2_BYTES` (see [Sequenced analysis](#sequenced-analysis)). |
| Defaults when unset | If all three prompt env vars are empty, defaults point at AAOS builder `prompt/sequenced/` (see `gemini_analysis.sh`). |

## Usage

- **Single or sequenced:** One prompt file → single run. For sequenced (context chaining), set `GEMINI_PROMPT_FILE` and `GEMINI_PROMPT_FILE_2` (and optionally `GEMINI_PROMPT_FILE_3`). Order matters. We do not ship single-prompt files. When no prompt env is set, default AAOS three-step prompts are used.

## Known issues

### Model pinning and `GEMINI_PREVIEW_FEATURES=FALSE`

- **What you configure:** You pin a model (via the `GEMINI_MODEL` environment variable or `--model <model name>` in the CLI) and set `GEMINI_PREVIEW_FEATURES=FALSE`, expecting only the pinned model to be used.
- **What happens:** The main analysis uses your pinned model. Follow-up or sub-agent calls may still *attempt* to use a preview model. The Gemini CLI's model-routing allows this (Pro for planning, Flash for implementation), and built-in sub-agents are enabled by default. Because preview is disabled, those calls fall back to your pinned model instead.
- **Bottom line:** The console logs may show that there were attempts made to access the preview models. In practice, only the pinned model is used; the "attempt to use preview model and then fallback" is internal behaviour and does not change the models actually used.
- **Recommended settings:** Enable preview and pin a model. Preview models are available only through the global endpoint, so set `GEMINI_LOCATION_GLOBAL=TRUE` with `GEMINI_PREVIEW_FEATURES=TRUE`, and pin via `GEMINI_MODEL` or `--model <name>` (e.g. `GEMINI_MODEL=gemini-2.5-pro`).
- **Related bug:** [google-gemini/gemini-cli#13475](https://github.com/google-gemini/gemini-cli/issues/13475) (closed; fix was reverted in [PR #13483](https://github.com/google-gemini/gemini-cli/pull/13483)).
- **Further reading:** [Sub Agents](https://geminicli.com/docs/core/subagents/), [model routing](https://github.com/google-gemini/gemini-cli/blob/3d4956aa57539977f64c9fc83de9334b0e6c8106/docs/reference/configuration.md).

### Model output quality and proposed fixes

- **Hallucination:** The model can occasionally invent paths, APIs, or references that do not match your tree or branch. This is  uncommon but possible.
- **Proposed fixes:** Suggestions in `gemini-assist/` (and in step outputs) are **assistive only**. They may be incomplete, wrong for your configuration, or miss edge cases. Always **verify** against the real build log, source, and your product constraints before applying changes.
- **Expectation:** Treat outputs as a starting point for triage and investigation—not a substitute for engineer review and further debugging when needed.

### `missing pgrep output` in the debug console

- **What it is:** A message that can appear in the Gemini CLI **debug console** (for example after tool use), not a generic operating-system error from your host.
- **Why it appears:** The CLI uses **process monitoring** around shell/tool runs: it may run `pgrep` to correlate child processes. If the child **exits very quickly**, `pgrep` can find no matching process. The tooling treats that as “missing pgrep output” and surfaces it in the debug console. In many cases this is **noise** from timing/race behaviour rather than evidence of a failed command.
- **Bottom line:** You can usually **ignore** this when the underlying tool run actually succeeded; it reflects internal bookkeeping, not necessarily a real failure on your machine or in your pipeline.
- **Related:** [google-gemini/gemini-cli#4095](https://github.com/google-gemini/gemini-cli/issues/4095).

### Linux: CLI hangs after OAuth (GNOME Keyring / keytar)

- **What happens:** On some Linux desktops (e.g. Ubuntu with GNOME), the CLI can **block silently** after authentication when token storage uses the system keyring and `keytar.setPassword()` does not return.
- **Workaround:** Set `GEMINI_FORCE_FILE_STORAGE=true` to use encrypted file storage instead of the keychain path (as described in upstream discussion).
- **Related bug:** [google-gemini/gemini-cli#21622](https://github.com/google-gemini/gemini-cli/issues/21622).

## Maintenance

| File | Role | When to touch |
|------|------|----------------|
| `gemini_environment.sh` | Defaults and exports for all jobs. | Adding a new `GEMINI_*` or `GOOGLE_*` env var; document in the header block. |
| `gemini_initialise.sh` | Cleanup, CLI install, auth, skills setup; copies `gemini/policies/*.toml` into `.gemini/policies/` in CI. | Changing skills resolution (e.g. new source for skills.yaml); flow order; add or edit policy files under `gemini/policies/` (see [Workspace policy files](#workspace-policy-files-toml)). |
| `gemini_analysis.sh` | Prompt resolution, single/sequenced run, JSON extraction, per-step `headless_output_stepN_*.json`, step 3 composed context from step1+step2. | Adding step 4+; changing default prompt dir; jq extraction paths; caps for prior-step bytes. |
| `gemini_skills_from_yaml.py` | Converts `skills.yaml` → `.gemini/skills/<name>/SKILL.md`. | New skill YAML fields (e.g. `tools`, `tags`); SKILL.md frontmatter format. |

- **Skills schema:** `skills.yaml` has a top-level `skills:` list; each entry: `name` (required), `description`, `system_instructions`. The Python script writes frontmatter and body only; if Gemini CLI adds new frontmatter keys, extend `gemini_skills_from_yaml.py` and this table.
- **Prompts:** Generic filenames are `step1_triage.txt`, `step2_rca.txt`, `step3_fixes.txt`. Each job points to its own prompt directory; skills file is always `skills.yaml` beside those prompts (or via `GEMINI_SKILLS_YAML`).
- **PyYAML:** Required for skills conversion. Provided by `python3-yaml` (apt) in the utilities infra image; Builder/CTS images use pip or apt as per their Dockerfiles.
