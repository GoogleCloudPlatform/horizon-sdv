---
name: systems-perf-parity-and-optimization
description: >-
  Benchmarks C/C++ vs. Rust networking and IPC daemons across single-container
  UDS (Layout A) and dual-container Linux bridge UDP/TCP/Multicast (Layout B)
  testbeds, evaluates statistical performance parity using bootstrap confidence
  intervals, and applies evidence-driven post-parity Class B hot-path
  optimizations (zero-copy Bytes/Arc slicing, writev batching, try_write fast
  paths, RwLock + SmallVec routing tables, shared deadline timer wheels,
  process-wide poll/mio socket reactors, and lazy /proc peer credentials). Use
  when: benchmarking a Rust port against a C/C++ baseline, diagnosing why a safe
  Rust port is slower than C++ -O3, optimizing UDS/UDP/TCP RPC latency or
  throughput, or generating deterministic performance parity reports. Don't use
  for: initial Boost.Asio concurrency design (use cpp-asio-to-rust-migration),
  general Clippy safety linting (use rust-automotive), or FFI boundary checks
  (use iso26262-ffi-isolation).
---

<!-- Copyright 2026 Google LLC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

        http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License. -->

# Systems Performance Parity & Hot-Path Optimization (`systems-perf-parity-and-optimization`)

Guides staged performance benchmarking and post-parity hot-path optimization for Rust ports of C/C++ networking and IPC middleware, aligned with **[Google's FOSS Rust Style Guide](https://google.github.io/styleguide/rustguide.html)** ([`#prefer-borrows-over-deep-copies`](https://google.github.io/styleguide/rustguide.html#prefer-borrows-over-deep-copies), [`#avoid-rc-and-arc`](https://google.github.io/styleguide/rustguide.html#avoid-rc-and-arc)) and **[Effective Rust](https://www.lurklurk.org/effective-rust/)**.

## Quick Start

| Goal | Command |
|---|---|
| **Run built-in statistical evaluator self-test** | `python3 skills/systems-perf-parity-and-optimization/scripts/evaluate_perf_parity.py --self-test` |
| **Evaluate early-warning parity gate (`<= 2.0x` p50, `>= 0.5x` tput)** | `python3 skills/systems-perf-parity-and-optimization/scripts/evaluate_perf_parity.py --results-dir bench/results/latest --stage early-warning --strict` |
| **Evaluate full parity target (`>= 85%` cells meeting/beating C++ `-O3`)** | `python3 skills/systems-perf-parity-and-optimization/scripts/evaluate_perf_parity.py --results-dir bench/results/latest --stage parity-target --strict` |

## Why Unoptimized "Safe Rust" Ports Often Trail C++ `-O3`

A faithful, panic-free Rust port of a C++ networking stack frequently starts out **slower** than C++ `-O3` on small-payload IPC and high-fan-out routing because of seven common translation habits:
1. Replacing raw C++ pointer/buffer reuse with per-message `Vec<u8>` heap allocations (`Payload::new`, `Serializer::serialize_vec`).
2. Replacing direct socket writes with `mpsc` channel hops and unbatched per-frame `send()` syscalls.
3. Wrapping C++ routing and subscription `std::map`/`std::unordered_map` tables in `Mutex` and cloning `Vec<ClientId>` on every packet.
4. Spawning a separate async task or OS thread per outgoing RPC request timeout or per socket connection ($O(N)$ thread scaling).
5. Reading `/proc/<pid>/cmdline` synchronously on the Unix Domain Socket `accept()` path.
6. Using `Arc::make_mut` on nested `HashMap` subscriber maps on every event notification.
7. Wrapping `Vec<u8>` payloads in a `Mutex` instead of copy-on-write `bytes::Bytes` storage.

Optimizing these *during* the initial port risks breaking behavioral parity. Enforce a **three-stage parity-first lifecycle** instead.

---

## Pillar 1: Three-Stage Performance Governance Lifecycle

| Stage | Timing | Quantitative Gate | Purpose |
|---|---|---|---|
| **1. Early-Warning Gate** | Core transport & routing phases | Rust `p50 RTT <= 2.0x` C++; `throughput >= 0.5x` C++ | Catches severe architectural mistakes (extra thread hops or global lock serialization) early |
| **2. Unoptimized Parity Baseline** | Immediately after functional parity tag | Full latency/throughput/RSS/CPU matrix vs. C++ `-O3` and `-O2`; every miss gets a root cause + optimization ticket | Establishes an honest scientific baseline of the 1-to-1 port before any hot-path tuning |
| **3. Class B Hot-Path & Reactor Optimization** | Post-parity optimization pass | `p50 RTT <= 1.10x`, `p99 <= 1.20x`, `throughput >= 0.90x`, `RSS <= 1.20x`, `CPU/msg <= 1.10x` (`>= 85%` cells `OUTPERFORM` + `MEET TARGET`) | Applies internal micro-architectural optimizations without altering public API or wire format |

---

## Pillar 2: Representative Container Benchmark Architecture (`Layout A` & `Layout B`)

Run identical C++ (`-O3` Release and `-O2` RelWithDebInfo) and Rust (`--release`) benchmark apps across two container topologies:

1. **`Layout A` (Single Container — Intra-Node Unix Domain Socket IPC)**:
   - Runs the routing daemon, benchmark service, and benchmark client as separate OS processes communicating over `/tmp/*` UDS sockets.
   - Measures pure IPC serialization, kernel UDS socket buffer traversal, and local routing-table lookup overhead.
2. **`Layout B` (Two Bridge-Connected Containers — Inter-Node Ethernet UDP / TCP / Multicast)**:
   - Runs Container `node-1` (`10.88.0.10`) and Container `node-2` (`10.88.0.11`) connected via a Linux virtual Ethernet (`veth`) bridge (`224.224.224.245` multicast).
   - Exercises real Linux netns/bridge packet traversal, UDP/TCP payload segmentation (up to 64 KB), and multicast discovery.
3. **Cross-Stack Matrix (`cpp-cpp`, `rust-rust`, `rust-cpp`, `cpp-rust`)**:
   - Always benchmark mixed `Rust client <-> C++ service` and `C++ client <-> Rust service` pairs alongside homogeneous stacks. Mixed cells immediately isolate whether a latency or CPU bottleneck lives in the client library or the server/routing daemon.

---

## Pillar 3: Core Post-Parity Class B Optimizations

Apply these verified internal optimizations (code templates in [hot_path_optimization_catalog.md](references/hot_path_optimization_catalog.md)):

| Category | Unoptimized Port Bottleneck | Recommended Class B Optimization | Typical Impact |
|---|---|---|---|
| **Zero-Copy & CoW Framing** | Per-message `Mutex<Vec<u8>>` clones and multi-buffer IPC assembly | Copy-on-write `PayloadStorage::Shared(Bytes) \| Owned(Vec<u8>)` + single-allocation `BytesMut` IPC frame assembly | `4x–9x` faster serialize; `0` heap allocs on unmutated `clone`/`equals`; `-50%+` 64 KB UDS RTT |
| **Vectored Non-Blocking Writes** | Per-frame `mpsc` channel hop + single-buffer `send()` syscalls | Non-blocking `try_write` fast path when outbound queue is empty + `write_vectored` (`writev`) batch draining of up to 64 frames | `2x–3x` higher UDS/TCP throughput; `-50%` to `-75%` CPU/msg |
| **Read-Mostly Routing & Fan-Out** | `Mutex<HashMap>` locks and `Vec<ClientId>` heap clones per packet | `RwLock` routing tables + stack-allocated `SmallVec<[ClientId; 8]>` target lists + per-group subscriber locks | `+40%` to `+90%` 1-to-N subscriber fan-out throughput |
| **Shared Deadline Wheel** | Spawning a timer task or `thread::sleep` thread per RPC timeout / offer loop | Shared monotonic deadline wheel (`BTreeMap<(Instant, Key), ...>`) on a single `TimerReactor` thread | Eliminates per-request task allocation and scheduler jitter |
| **Lazy Peer Credentials** | Synchronous `std::fs::read_to_string("/proc/<pid>/cmdline")` on UDS `accept()` | Cache peer UID/GID from `SO_PEERCRED` immediately; read `/proc/<pid>/cmdline` lazily only if diagnostic logging requests it | `-30%` 100-client connection storm latency |
| **$O(1)$ Socket Reactors** | Dedicated OS threads per UDP/TCP/UDS socket + `50 ms` `poll()` timeouts | Multiplex all pollable UDS and UDP/TCP sockets onto shared `poll(2)`/`mio` reactors with slot-0 `eventfd` wakeup and cached `pollfd` vectors | $O(1)$ thread count across `50–100+` peers, `0.0/s` idle wake-ups, `-25%` to `-50%` RSS |

---

## Bundled Performance Parity Evaluator (`scripts/evaluate_perf_parity.py`)

Run the bundled statistical evaluator on `summary.json`, `*.samples` + `.rusage.json`, or `cells.json` benchmark outputs to compute `p50`/`p95`/`p99`, 95% bootstrap confidence intervals, and gate verdicts (`OUTPERFORM`, `MEET TARGET`, `DOCUMENTED`):

```bash
python3 skills/systems-perf-parity-and-optimization/scripts/evaluate_perf_parity.py \
  --results-dir bench/results/latest \
  --stage parity-target \
  --format markdown \
  --strict
```

### Supported CLI Flags
- `--results-dir <dir>`: Directory containing `summary.json`, `*.samples`, or `cells.json` benchmark files.
- `--stage {early-warning,parity-target}`: Gate thresholds (`early-warning`: `p50 <= 2.0x`, `tput >= 0.5x`; `parity-target`: `p50 <= 1.10x`, `p99 <= 1.20x`, `tput >= 0.90x`, `rss <= 1.20x`, `cpu <= 1.10x`).
- `--min-pass-rate <float>`: Minimum fraction of evaluated cells required to achieve `OUTPERFORM` or `MEET TARGET` under `--strict` (default: `0.85`).
- `--format {markdown,json,text}`: Output format (default: `markdown`).
- `--strict`: Exit `1` if `total_cells == 0` or the pass rate falls below `--min-pass-rate`.
- `--self-test`: Runs deterministic self-tests on synthetic latency, throughput, RSS, and CPU sample distributions.

---

## Deep-Dive References

- **[hot_path_optimization_catalog.md](references/hot_path_optimization_catalog.md)**: Side-by-side Rust code patterns for zero-copy `Bytes` CoW payloads, `try_write` + `writev` vectored socket draining (with callback re-entrancy protection), `SmallVec` fan-out routing, shared `poll(2)` + `eventfd` socket reactors, and kernel epoll floor diagnostics.
