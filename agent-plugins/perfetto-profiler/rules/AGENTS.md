# Perfetto Profiler Global Rules

The following constraints apply to all interactions and skills within the Perfetto trace profiling domain for both Android Applications and AAOS Platform engineering.

## Trace Staging & Query Execution Guardrails
- **No Raw Binary Reads:** NEVER use `read_file` or text search tools (`grep`, `rg`) to read `.pftrace`, `.perfetto-trace`, or `.pb` files. Always query trace contents through the official `trace_processor` SQL engine.
- **SHA-256 Verified Bootstrap Only:** Always verify the SHA-256 checksum (`f4acbe5388d8e5d9f3aac9c509026902a1cf6e50ad0b888bdba9acc12b83d886`) when bootstrapping the official Google Perfetto `v59.1` `trace_processor` wrapper into `/tmp/perfetto-traces/bin/`. Never run unverified third-party scripts or `npx` runners.
- **Workspace Hygiene:** Stage all traces downloaded from Google Cloud Storage (`gs://`) and temporary SQL files strictly under `/tmp/perfetto-traces/`, never inside a Git repository checkout.

## Evidence Grounding & Output Generation
- **Schema Verification:** Never guess PerfettoSQL table or column names; verify schemas with `SELECT * FROM <table> LIMIT 0;` and query `__intrinsic_stdlib_objects` first.
- **Evidence Tags:** Tag every root-cause claim with `[SQL]` (backed by concrete queried numbers and timestamps), `[INFERRED]` (logical deduction from trace data), or `[GAP]` (missing trace config signal or dropped ring-buffer events).
- **Report Location:** All persisted RCA summaries, triage reports, and proposed code fixes must be written to the `antigravity-fixes/` directory in the workspace root (`antigravity-fixes/perfetto_rca_<trace_name>_<YYYYMMDD_HHMMSS>.md`).
