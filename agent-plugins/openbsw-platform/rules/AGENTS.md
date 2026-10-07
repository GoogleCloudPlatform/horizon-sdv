# OpenBSW Platform Global Rules

The following constraints apply to all interactions and skills within the Eclipse OpenBSW platform domain.

## Build System & Code Exploration Guardrails
- **Search Exclusions:** Do not search or suggest files under `build/`, `_build/`, or `.git/`.
- **Log Parsing Limits:** Avoid reading multi-GB log files end-to-end. Use bounded reads (e.g. `tail -n 2000 bsw-build.log`) or `grep_search`.

## Fixing & Output Generation
- **Patch Location:** All proposed code patches, fixes, and RCA summaries must be written to the `antigravity-fixes/` directory in the workspace root. Do NOT create, write, or save files to other arbitrary directories.
- **Diff Accuracy:** Only provide Git-style diffs (`-` for original, `+` for suggested). Always confirm the original file contents before writing a diff. Do not invent code.
