# AAOS Platform Global Rules

The following constraints apply to all interactions and skills within the Android Automotive OS (AAOS) platform domain.

## Build System & Code Exploration Guardrails
- **No Heavy Scans:** Do NOT run heavy tree-wide searches (`find`, `grep -r`, `rg`) over the entire AOSP root (`$ANDROID_BUILD_TOP`). Always scope searches (`grep_search` or `list_directory`) to specific modules (e.g., `device/vendor/`, `frameworks/base/`).
- **Ignore Build Outputs:** Never search or suggest edits within the `out/` directory, any `out_*` tree, or `.repo/`.
- **Log Parsing Limits:** NEVER use `read_file` to ingest multi-GB log files end-to-end. Always use bounded reads (e.g. `tail`), or `grep_search` to find failure markers (`#error`, `FAILED:`, `fatal error:`) before inspecting context.

## Fixing & Output Generation
- **Patch Location:** All proposed code patches, fixes, and RCA summaries must be written to the `antigravity-fixes/` directory in the workspace root.
- **Diff Accuracy:** Only provide Git-style diffs (`-` for original, `+` for suggested). Always confirm the original file contents with `read_file` before writing a diff. Do not invent code.
- **Suite Mono-Fix Ban:** If a test suite fails across multiple distinct tests (`Test#Method`), generate one fix per distinct failure. Do not hallucinate a single framework fix that magically resolves all suite errors unless you have definitive line-level proof.
