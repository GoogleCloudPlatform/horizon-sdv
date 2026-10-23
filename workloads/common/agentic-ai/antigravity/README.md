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

# Antigravity SDK AI review

Antigravity SDK is the replacement for the headless Gemini
CLI execution loop.

The existing pipeline entrypoint remains:

```text
workloads/common/agentic-ai/gemini/run_ai_review.sh
```

`run_ai_review.sh` selects this harness when `AI_REVIEW_RUNTIME` is `agy` or unset. That is the default: Jenkins parameters list `agy` first, Helm charts set `spec.aiReviewRuntime` to `agy`, and the AAOS, CTS, and CVD templates fall back to `agy` when the value is empty. Set `AI_REVIEW_RUNTIME=gemini-cli` to use the headless Gemini CLI instead.

## Runtime behavior

`run_ai_review.sh` runs `GEMINI_ARTIFACTS_COMMAND` and any configured pre-hook, then starts `run_agy_ai_review.sh` when `AI_REVIEW_RUNTIME` is `agy` or unset. `gemini-cli` runs `gemini_initialise.sh` and `gemini_analysis.sh` instead.

The harness:

1. Loads every skill package under `workloads/common/agentic-ai/antigravity/skills/` (`aaos_builder`, `cts_execution`, `cvd_launcher`, `openbsw`). The package name is the classification. The agent sets it from the logs. It does not use the Jenkins job name.
2. Allows only read-only workspace tools: list directory, search, find file, view file, and finish. Shell commands, file edits, and subagents are denied.
3. Runs one `agent.chat()` session. The SDK may take several tool turns inside that session.
4. Returns a structured analysis. The harness stamps the model name and the tools that actually ran.
5. Prints the analysis JSON to the build log.
6. Writes `gemini-assist/agy_analysis_and_proposed_fix.md`.
7. Writes `traces/agent_trace_<workflow>_<session>.json`.

The fix is a recommendation only. The harness does not edit files, run commands, or apply patches.

`GEMINI_MODEL` selects the model when it is set. Otherwise the SDK default is used. `GEMINI_COMMAND_LINE` is not read.

In Utilities->Gemini AI Assistant, uploaded prompt files and `GEMINI_SKILLS_YAML` are used only by `gemini-cli`. AGY does not require these files. It ignores them if provided.

A seed log is picked from the workspace, at most two directories deep, in this order: `aaos-build.log.tail`, `aaos-build.log`, `bsw-build.log`, `launcher.log`. At most 16 KB from the tail of that file is copied into the first prompt (`--seed-log-bytes`). If `test-results/` exists, the harness lists at most 80 paths (`.json`, `.log`, `.txt`, `.xml`, `.zip`, and `logcat`); HTML and build-output trees are omitted. The agent can read other workspace files with its tools. When CTS HTML is present, the harness writes `test-results/cts_module_summary.txt` before that inventory and copies it into the artifact directory.

The tool hook denies shell, file edits, subagents, broad searches such as `fail` or `error`, and opening HTML reports. A directory search aimed at a file path is rewritten to the parent directory. If structured output is missing, a fallback parser builds `HorizonAnalysis` from the response text. The harness exits 0 after a completed analysis so Jenkins and Argo can publish the review when the original build or test failed.

## Outputs

`gemini-assist/agy_analysis_and_proposed_fix.md` contains:

- model, tools used, status, classification, and confidence
- summary and root cause
- assumptions, when the model returned any
- one section per distinct failure, with evidence
- one or more recommended fixes per failure: summary, files, commands, patch text, and verification steps

Repeated cascading errors for the same failure are not given a second fix.

`traces/agent_trace_<workflow>_<session>.json` is the local trace (session events, tool calls, token counts). A failure to write this file does not fail the review. When `GOOGLE_CLOUD_PROJECT` is set, the same events are sent to Cloud Logging under the log name `sdlc-agent-runtime`. A Cloud Logging failure does not fail the review.

Jenkins archives `gemini-assist/*` and `traces/*.json`. For `agy`, `headless_output*.json` is not archived. `gemini-cli` still archives that JSON.

## Prerequisites

- Python 3.10 or newer
- google-antigravity>=0.1.16
- pydantic>=2
- Application Default Credentials / workload identity with Vertex model access
- `GOOGLE_CLOUD_PROJECT`
- `GOOGLE_CLOUD_LOCATION`
- `GOOGLE_GENAI_USE_VERTEXAI=true`

## Direct AAOS AI Review test

Run from the build workspace containing the staged AAOS log:

```bash
export GOOGLE_GENAI_USE_VERTEXAI=true
export GOOGLE_CLOUD_PROJECT=YOUR_PROJECT
export GOOGLE_CLOUD_LOCATION=global

python3 /path/to/horizon/workloads/common/agentic-ai/antigravity/harness.py \
  --workspace "$PWD" \
  --log-file "$PWD/aaos-build.log.tail" \
  --no-mcp \
  --verbose
```

Use `GEMINI_MODEL` only when the platform requires a specific approved
model. Otherwise the SDK default is used.

## Direct CTS and CVD tests AI Review test

The workspace should contain the already-staged `test-results/` tree. Jenkins and Argo preparation unpack `cuttlefish*.zip` under `test-results/cvd` and CTS result zips under `test-results/cts` before the agent runs.

```bash
python3 /path/to/horizon/workloads/common/agentic-ai/antigravity/harness.py \
  --workspace "$PWD" \
  --no-mcp \
  --verbose

python3 /path/to/horizon/workloads/common/agentic-ai/antigravity/harness.py \
  --workspace "$PWD" \
  --no-mcp \
  --verbose
```

## Direct OpenBSW builder test
Run from the build workspace containing the staged bsw-build.log:

```bash
python3 /path/to/horizon/workloads/common/agentic-ai/antigravity/harness.py \
  --workspace "$PWD" \
  --log-file "$PWD/bsw-build.log" \
  --no-mcp \
  --verbose
```

## End-to-end pipeline test

Jenkins jobs default **AI Review runtime** to `agy`. Choose `gemini-cli` on the same job to compare CLI output. From a shell:

```bash
export AI_REVIEW_RUNTIME=agy
export GEMINI_ANALYSIS_PATH=/path/to/workspace
bash workloads/common/agentic-ai/gemini/run_ai_review.sh
```

The existing pre/post hooks and artifact movement still run.

Argo charts already set:

```yaml
spec:
  aiReviewRuntime: agy
```

| Job | Default |
| --- | --- |
| AAOS Builder, CTS Execution, CVD Launcher | Jenkins **AI Review runtime** = `agy`. Helm `aiReviewRuntime: agy`. CTS Execution and CVD Launcher run AI Review from `cvd-pipeline-shared-library`. |
| AAOS ABFS Builder, OpenBSW BSW Builder | Jenkins **AI Review runtime** = `agy`. These jobs have no Argo workflow. |
| Utilities / Gemini AI Assistant | **AI Review runtime** = `agy`. `GEMINI_ARTIFACTS_COMMAND` is required. Uploaded prompts and `GEMINI_SKILLS_YAML` are not. |
| AAOS SDV Builder | Timeout parameter only. This job has no `AI_REVIEW_RUNTIME` parameter. |

The utility and CTS/CVD stages set `GEMINI_ANALYSIS_PATH` to the Jenkins workspace and do not move artifacts afterward. The trace file must be at `${WORKSPACE}/traces/*.json` for the build page to show it. The bucket upload uses the same `traces/` directory when `gemini_storage.sh` runs. `headless_output*.json` is archived only when the runtime is not `agy`.

`GEMINI_AI_EXECUTION_TIMEOUT_MINUTES` bounds the run. When unset, `run_ai_review.sh` uses 60 minutes and `timeout --kill-after=1m`. Jenkins choices are 60, 45, 30, 20, 10, 90, and 120. AAOS Builder, CTS Execution, and CVD Launcher Helm charts set `geminiAiExecutionTimeoutMinutes` to `60`.

## MCP status

`mcp_config.json` is valid but empty for now.
The agent uses Antigravity's workspace-scoped read-only file tools. Add MCP
servers only after their executable implementations are present in the
pipeline image.

## Exit codes

- `0`: analysis or validation completed and artifacts were written
- `2`: invalid input/configuration, missing SDK, authentication failure, or
  Antigravity runtime failure
