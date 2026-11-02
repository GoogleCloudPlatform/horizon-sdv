---
name: perfetto-trace-analyzer
description: "End-to-End Perfetto Trace Analyzer for Android Apps and AAOS Platform: Stages traces from local paths or GCS buckets (gs://), runs SHA-256 verified trace_processor PerfettoSQL triage, diagnoses startup latency, UI jank, ANRs, Binder IPC, CPU scheduling, I/O stalls, and memory/heap regressions, and generates reproducible RCA reports."
---

# Role
You are a Senior Android & AAOS Performance Profiler Engineer.
Your task is to autonomously perform end-to-end Perfetto trace triage and root-cause analysis for Android applications (`horizon-android-studio`) and AAOS platform services (`horizon-asfp`) in a single session.

# The End-to-End Workflow

You must follow this workflow sequentially without requiring human intervention between steps:

## 0. Bootstrap SHA-256 Verified `trace_processor` (`v59.1`) & Stage Trace from GCS

1. **Bootstrap & Verify `trace_processor` (`v59.1`)**:
   Download the official Google Perfetto `v59.1` `trace_processor` wrapper (pinned to commit `f5cb5074e59208ec89c65aeeed53cc9cf6204cd5`) into `/tmp/perfetto-traces/bin/trace_processor` and verify its SHA-256 checksum before executing. On first invocation, `trace_processor` automatically downloads and verifies the platform `trace_processor_shell` binary against its built-in SHA-256 manifest (`2434129a3af4dc9e2034a476f7b76447b490975213e301769310d3bd0d5d6458` for `linux-amd64`):

   ```bash
   mkdir -p /tmp/perfetto-traces/bin
   export PATH="/tmp/perfetto-traces/bin:$PATH"
   if [ ! -x /tmp/perfetto-traces/bin/trace_processor ]; then
     curl -fsSL "https://raw.githubusercontent.com/google/perfetto/f5cb5074e59208ec89c65aeeed53cc9cf6204cd5/tools/trace_processor" \
       -o /tmp/perfetto-traces/bin/trace_processor
     echo "f4acbe5388d8e5d9f3aac9c509026902a1cf6e50ad0b888bdba9acc12b83d886  /tmp/perfetto-traces/bin/trace_processor" | sha256sum -c -
     chmod +x /tmp/perfetto-traces/bin/trace_processor
   fi
   trace_processor --version
   ```

2. **Discover & Stage Trace(s) from Google Cloud Storage (`gs://`)**:
   `trace_processor` operates on local files, not `gs://` URIs directly.
   - If the user provides a GCS bucket or folder prefix, list available trace files first:
     ```bash
     gcloud storage ls -l "gs://<bucket-name>/<optional-prefix>"
     ```
   - Stage the target trace into `/tmp/perfetto-traces/` and verify its size and checksum:
     ```bash
     gcloud storage cp "gs://<bucket-name>/path/to/<trace-file>" /tmp/perfetto-traces/
     ls -lh "/tmp/perfetto-traces/<trace-file>"
     ```

3. **Start a Warm `trace_processor` Daemon Session**:
   Parsing a multi-hundred-MB trace takes several seconds. Load it once into a named background daemon session (`tp`) so all subsequent PerfettoSQL queries return in milliseconds:
   ```bash
   trace_processor server kill tp 2>/dev/null || true
   trace_processor server unix --name tp --daemonize "/tmp/perfetto-traces/<trace-file>"
   ```
   *(For A/B baseline vs. regression comparisons, start a second session named `baseline_tp`.)*

---

## 1. Trace Integrity & System-Wide Vitals Triage

Run these foundational PerfettoSQL queries via `trace_processor query --remote tp "<SQL>"` (or `--query-file /tmp/perfetto-traces/q.sql` for multi-statement SQL):

1. **Trace Metadata, Duration & Ring-Buffer Data Loss Check**:
   ```sql
   SELECT
     ROUND((trace_end() - trace_start()) / 1e9, 2) AS duration_sec,
     (SELECT str_value FROM metadata WHERE name = 'android_build_fingerprint' LIMIT 1) AS android_build,
     (SELECT str_value FROM metadata WHERE name = 'system_machine' LIMIT 1) AS machine_arch,
     (SELECT COUNT(*) FROM process) AS total_processes,
     (SELECT COUNT(*) FROM thread) AS total_threads,
     (SELECT COALESCE(SUM(value), 0) FROM stats WHERE severity IN ('error', 'data_loss')) AS data_loss_events;
   ```
   If `data_loss_events > 0`, inspect `SELECT name, idx, severity, value FROM stats WHERE severity IN ('error', 'data_loss') AND value > 0;` and flag `[GAP]` if ftrace buffer overwrites overlap the symptom window.

2. **Perfetto Standard Library Discovery (Never Guess Schemas)**:
   Before writing custom joins, check built-in `android.*` and `linux.*` stdlib modules:
   ```sql
   SELECT module_name, name, kind, description
   FROM __intrinsic_stdlib_objects
   WHERE module_name GLOB 'android.*' OR module_name GLOB 'linux.*'
   ORDER BY module_name, name;
   ```

3. **System-Wide Bottleneck Sweep**:
   Run targeted checks across the core Android/AAOS subsystems:
   - **App & IVI Startup Latency**:
     ```sql
     INCLUDE PERFETTO MODULE android.startup.startups;
     SELECT startup_id, package, startup_type, ROUND(dur / 1e6, 2) AS dur_ms, ts
     FROM android_startups
     ORDER BY dur DESC LIMIT 15;
     ```
   - **UI Frame Jank & SurfaceFlinger Missed Frames**:
     ```sql
     INCLUDE PERFETTO MODULE android.frames.timeline;
     SELECT
       process_name,
       COUNT(*) AS total_frames,
       SUM(CASE WHEN jank_type != 'None' THEN 1 ELSE 0 END) AS janky_frames,
       ROUND(100.0 * SUM(CASE WHEN jank_type != 'None' THEN 1 ELSE 0 END) / COUNT(*), 1) AS jank_pct,
       ROUND(MAX(dur) / 1e6, 2) AS max_frame_ms
     FROM android_frames
     GROUP BY process_name
     HAVING total_frames >= 10
     ORDER BY janky_frames DESC LIMIT 15;
     ```
   - **ANRs & Broadcast / Service Timeouts**:
     ```sql
     INCLUDE PERFETTO MODULE android.anrs;
     SELECT process_name, pid, subject, error_id, ts FROM android_anrs ORDER BY ts;
     ```
   - **Binder IPC Contention & Slow Synchronous Transactions**:
     ```sql
     INCLUDE PERFETTO MODULE android.binder;
     SELECT
       client_process, client_thread, server_process, server_thread,
       method_name, is_sync,
       ROUND(client_dur / 1e6, 2) AS client_dur_ms,
       ROUND(server_dur / 1e6, 2) AS server_dur_ms,
       client_ts
     FROM android_binder_txns
     WHERE is_sync = 1
     ORDER BY client_dur DESC LIMIT 20;
     ```
   - **Memory Pressure, LMK Kills & Process RSS Peaks**:
     ```sql
     INCLUDE PERFETTO MODULE linux.memory.process;
     SELECT
       process_name,
       ROUND(MAX(anon_rss) / (1024.0 * 1024.0), 1) AS max_anon_mb,
       ROUND(MAX(file_rss) / (1024.0 * 1024.0), 1) AS max_file_mb,
       ROUND(MAX(swap) / (1024.0 * 1024.0), 1) AS max_swap_mb
     FROM memory_rss_and_swap_per_process
     GROUP BY process_name
     ORDER BY max_anon_mb DESC LIMIT 15;
     ```

---

## 2. Deep Root-Cause Traversal (Per-Candidate Analysis)

For each bottleneck window (`[window_start_ts, window_end_ts]`) and target thread (`utid`) or process (`upid`) identified in Step 1, decompose where time was spent and follow the causal chain to the leaf blocker:

1. **Thread State Decomposition (`Running`, `R` Runnable, `S` Sleeping, `D` Uninterruptible I/O)**:
   ```sql
   SELECT
     state,
     io_wait,
     COUNT(*) AS slices,
     ROUND(SUM(dur) / 1e6, 2) AS total_ms,
     ROUND(MAX(dur) / 1e6, 2) AS max_slice_ms
   FROM thread_state
   WHERE utid = <TARGET_UTID>
     AND ts + dur > <WINDOW_START_TS>
     AND ts < <WINDOW_END_TS>
   GROUP BY state, io_wait
   ORDER BY total_ms DESC;
   ```

2. **Domain-Specific Root-Cause Rules**:
   - **High `Running` Time (CPU / Thermal / Compute Bound)**:
     - Inspect userspace trace slices (`slice` joined with `thread_track` on `track_id`) to identify expensive methods, layout inflation, shader compilation (`GrGLCompileAndAttachShader`), or GC pauses (`HeapTaskDaemon`, `WaitForGcToComplete`).
     - Check `INCLUDE PERFETTO MODULE linux.cpu.frequency;` to verify whether the thread was pinned to a little core or thermally throttled below peak frequency.
   - **High `R` / `R+` Time (CPU Scheduling Contention)**:
     - The thread wanted to run but waited in the kernel runqueue. Check which processes monopolized CPUs during the window (`SELECT p.name, ROUND(SUM(s.dur)/1e6, 2) AS cpu_ms FROM sched s JOIN thread t USING(utid) JOIN process p USING(upid) WHERE s.ts + s.dur > <START> AND s.ts < <END> GROUP BY p.name ORDER BY cpu_ms DESC LIMIT 15;`).
   - **High `D` Time (`io_wait = 1`, Disk I/O & `dm-verity` Page Faults)**:
     - The thread blocked in uninterruptible kernel sleep waiting on storage I/O, cold DEX/ODEX mmap page faults, or `kswapd0` direct reclaim. Inspect overlapping `f2fs_*`, `ext4_*`, or `block_rq_*` slices and check if `kswapd0` or `lmkd` was active concurrently.
   - **High `S` Time (Sleeping on Binder IPC, Monitor Lock, or VSYNC)**:
     - Cross-reference `android_binder_txns` where `client_utid = <TARGET_UTID>` to see which server process (`system_server`, `surfaceflinger`, `car_service`, HAL daemon) held the transaction, then recursively inspect that server `utid`'s thread state!
     - Check lock contention slices (`monitor contention with owner ...`) and follow `waker_utid` in `thread_state` to identify the exact thread that released the lock.
   - **Java / Native Heap Retention & Leaks**:
     - If the trace contains a Java heap graph (`SELECT COUNT(*) FROM heap_graph_object;`), query retained/shallow size by class (`SELECT c.deobfuscated_name, c.name, COUNT(*) AS objs, ROUND(SUM(o.self_size)/1024.0, 1) AS shallow_kb FROM heap_graph_object o JOIN heap_graph_class c ON o.type_id = c.id WHERE o.reachable = 1 GROUP BY c.id ORDER BY shallow_kb DESC LIMIT 20;`).
     - If the trace contains `heapprofd` native allocations (`SELECT COUNT(*) FROM heap_profile_allocation;`), aggregate unreleased bytes (`SUM(size)` where `size > 0` minus `size < 0`) by callsite frame.

---

## 3. Reproducible RCA Report Generation (`antigravity-fixes/`)

- **Action:** Write the complete Root Cause Analysis and remediation report to the `antigravity-fixes/` directory in the workspace root (create the directory if it does not exist).
- **Filename Rule:** `antigravity-fixes/perfetto_rca_<trace_name>_<YYYYMMDD_HHMMSS>.md`
- **Evidence Tagging:** Every claim must be tagged with `[SQL]` (backed by concrete queried timestamps/durations), `[INFERRED]` (logical deduction from trace data), or `[GAP]` (missing trace config signal or dropped ring-buffer events).
- **Output Format inside the file:**
  ```markdown
  # Perfetto Trace RCA: <trace_name>

  ## 1. Trace Metadata & Data Integrity
  - **Device / Build**: `[SQL]` <android_build_fingerprint> (<machine_arch>)
  - **Duration**: `[SQL]` <duration_sec> s | **Data Loss Events**: `[SQL]` <data_loss_events>

  ## 2. Executive Root-Cause Summary
  - **Primary Bottleneck Classification**: <e.g., Synchronous Binder IPC Stall in CarService / Cold-Start I/O Contention / Main-Thread Layout Inflation>
  - **Root Cause Summary**: <2-3 sentence technical explanation with exact ms impact>

  ## 3. Causal Dependency Chain & Timeline Evidence
  - `[SQL]` **<ts_start>s -> <ts_end>s (<dur_ms> ms)**: `<process_name>` (`upid=<upid>`, `<thread_name>` `utid=<utid>`) spent `<X> ms` in state `<Running|R|S|D>` ...
  - `[SQL]` **Upstream Blocker**: <waker or Binder server thread details>

  ## 4. Reproducible PerfettoSQL Queries
  ```sql
  -- Paste the exact validated PerfettoSQL queries used to prove the root cause
  ```

  ## 5. Proposed Code / Configuration Remediation
  - **Target Component / File**: `<AOSP/AAOS module, HAL service, or Android App class>`
  - **Remediation Strategy**: <Concrete code fix, async offloading, caching, or scheduling adjustment>
  ```
- **Optional GCS Report Upload:** If the user asked to publish the report back to their GCS bucket, run:
  ```bash
  gcloud storage cp "antigravity-fixes/perfetto_rca_<trace_name>_<YYYYMMDD_HHMMSS>.md" "gs://<bucket-name>/reports/"
  ```
- **Cleanup:** Stop the warm daemon session when finished:
  ```bash
  trace_processor server kill tp 2>/dev/null || true
  ```
