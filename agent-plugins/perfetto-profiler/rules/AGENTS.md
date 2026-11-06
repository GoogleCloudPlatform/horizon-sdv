# Perfetto Profiler Global Rules

The following constraints apply to all interactions and skills within the Perfetto trace profiling domain for both Android Applications and AAOS Platform engineering.

## Trace Staging & Query Execution Guardrails
- **No Raw Binary Reads:** NEVER use `read_file` or text search tools (`grep`, `rg`) to read `.pftrace`, `.perfetto-trace`, or `.pb` files. Always query trace contents through the official `trace_processor` SQL engine.
- **Release-Tag & SHA-256 Verified Bootstrap Only:** Always fetch the official Google Perfetto `trace_processor` binary from the pinned release tag (`v59.1` at `https://commondatastorage.googleapis.com/perfetto-luci-artifacts/v59.1/linux-amd64/trace_processor_shell`), verify its SHA-256 checksum (`2434129a3af4dc9e2034a476f7b76447b490975213e301769310d3bd0d5d6458`), and delete the file and abort if verification fails. Never run unverified third-party scripts or `npx` runners.
- **Maximum Trace File Size (1 GB):** Enforce a maximum trace file size limit of **1 GB (`1073741824` bytes)** and verify the staged trace file's SHA-256 checksum before loading into `trace_processor`.
- **Workspace Hygiene:** Stage all traces downloaded from Google Cloud Storage (`gs://`) and temporary SQL files strictly under `/tmp/perfetto-traces/`, never inside a Git repository checkout.

## Evidence Grounding & Output Generation
- **Schema Verification:** Never guess PerfettoSQL table or column names; verify schemas with `SELECT * FROM <table> LIMIT 0;` and query `__intrinsic_stdlib_objects` first.
- **Evidence Tags:** Tag every root-cause claim with `[SQL]` (backed by concrete queried numbers and timestamps), `[INFERRED]` (logical deduction from trace data), or `[GAP]` (missing trace config signal or dropped ring-buffer events).
- **Report Location:** All persisted RCA summaries, triage reports, and proposed code fixes must be written to the `antigravity-fixes/` directory in the workspace root (`antigravity-fixes/perfetto_rca_<trace_name>_<YYYYMMDD_HHMMSS>.md`).
