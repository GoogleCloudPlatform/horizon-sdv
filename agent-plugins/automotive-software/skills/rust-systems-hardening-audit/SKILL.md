---
name: rust-systems-hardening-audit
description: >-
  Audits and hardens C/C++-to-Rust ports and embedded/systems Rust middleware
  daemons against Google's FOSS Rust Style Guide, Rust API Guidelines, and
  embedded resource invariants on a calibrated 1-to-10 rubric, and sequences
  post-parity Class B refactorings without breaking frozen public APIs or wire
  compatibility. Use when: reviewing a Rust daemon or completed C/C++ port for
  "C++ written in Rust" architectural debt, benchmarking code quality (1-10),
  eliminating O(N) per-socket threads or vestigial tokio dependencies, replacing
  Mutex<Weak<Self>>, C-style bool/0 errors, silent let _ = drops, or saturating_*
  wire-parser math, or planning post-parity hardening work packages. Don't use
  for: function-by-function C++ parity verification (use cpp-to-rust-port-review)
  or initial Boost.Asio mapping (use cpp-asio-to-rust-migration).
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

# Rust Systems & Middleware Post-Parity Hardening Audit (`rust-systems-hardening-audit`)

Scores and hardens Rust systems daemons and post-parity C/C++-to-Rust ports on a calibrated **1.0–10.0 benchmark** grounded in **[Google's FOSS Rust Style Guide](https://google.github.io/styleguide/rustguide.html)**, **[Comprehensive Rust](https://google.github.io/comprehensive-rust/)**, **[Effective Rust](https://www.lurklurk.org/effective-rust/)**, and the **[Rust API Guidelines](https://rust-lang.github.io/api-guidelines/)**.

## Quick Start

| Goal | Command |
|---|---|
| **Run built-in calibration self-test** | `python3 skills/rust-systems-hardening-audit/scripts/audit_rust_architecture.py --self-test` |
| **Score a Rust workspace (`1.0–10.0` Markdown scorecard)** | `python3 skills/rust-systems-hardening-audit/scripts/audit_rust_architecture.py --workspace /path/to/rust-workspace --arch-doc docs/ARCHITECTURE.md` |
| **Enforce `>= 9.0/10` CI / hardening gate** | `python3 skills/rust-systems-hardening-audit/scripts/audit_rust_architecture.py --workspace /path/to/rust-workspace --min-score 9.0 --strict` |
| **Emit JSON telemetry for a fresh-context reviewer subagent** | `python3 skills/rust-systems-hardening-audit/scripts/audit_rust_architecture.py --workspace /path/to/rust-workspace --format json --output /tmp/arch_scorecard.json` |

---

## Why Functional Parity (`PASS`) Often Scores `~6.0 / 10` Without Post-Parity Hardening

During a 1-to-1 C/C++-to-Rust port, preserving upstream class boundaries and public API signatures routinely leaks C++17 structural anti-patterns into internal Rust modules even when `cargo clippy`, safety linters, and integration suites are 100% green:

1. **Doc-vs-Code Architectural Drift**: `ARCHITECTURE.md` describes an aspirational `tokio::select!` + `CancellationToken` + `JoinSet` design while `src/` actually spawns blocking OS threads per socket.
2. **$O(N)$ Thread Sprawl & Vestigial Async Runtimes**: Dedicated reader/writer OS threads per TCP/UDP socket coexist alongside a `tokio` dependency linked solely for a single signal loop.
3. **Mechanical `enable_shared_from_this` & Lock Debt**: `self_weak: Mutex<Weak<Self>>`, write-once `Mutex<Option<Arc<T>>>` fields, multi-dozen-mutex God structs without a documented lock hierarchy, and mixed `SeqCst`/`Relaxed` stop flags.
4. **Swallowed Errors & C-Style `bool` / `0`-on-Error Contracts**: Hundreds of silent `let _ =` discards, unchecked `thread::Builder::spawn` calls, and internal `deserialize(&mut self, &[u8]) -> u32` (`0` on error) functions that bypass `must_use`.
5. **Blanket `saturating_*` Lint Suppression in Wire Codecs**: Replacing `+`/`-`/`*` with `.saturating_add()`/`.saturating_sub()` to silence `clippy::arithmetic_side_effects`, clamping corrupted wire lengths instead of returning `Err(...)` via `checked_*`.
6. **Binary Crate-Root `unsafe` Loophole**: `src/lib.rs` sets `#![deny(unsafe_code)]`, while `src/bin/daemon.rs` (a separate crate root) omits it and calls `unsafe { libc::fork() }` directly.
7. **`Mutex<Vec<u8>>` Payload Deep Copies**: Cloning or comparing immutable payloads locks a mutex and heap-allocates a fresh `Vec<u8>` instead of sharing `bytes::Bytes` copy-on-write slices.

---

## Two-Layer Benchmark Protocol (`1.0–10.0` Rubric)

To evaluate a codebase reproducibly across `50k–100k+` LOC without false positives or missed files, always combine **Layer 1 (Deterministic Telemetry CLI)** with **Layer 2 (Fresh-Context Semantic Review)**.

### Layer 1: Calibrated `10.0`-Point Rubric & FOSS Google Rust Style Mapping

Run `scripts/audit_rust_architecture.py` first. It separates **Part A: Safety Foundation (`5.00 pts`)** (distinguishing `1–3/10` fragile code from safe code) from **Part B: Idiomatic & Embedded Architecture (`5.00 pts`)**:

| ID | Dimension (`Max Pts`) | FOSS Google Rust Style & Ecosystem Reference | What `audit_rust_architecture.py` Measures |
|---|---|---|---:|
| **`F0`** | **Safety & Compiler Foundation (`5.00`)** | [Prefer pattern matching over `unwrap`](https://google.github.io/styleguide/rustguide.html#prefer-pattern-matching-over-check-then-unwrap), ISO 26262-6 Table 1 | Workspace `deny` lints (`unwrap_used`, `expect_used`, `indexing_slicing`, `undocumented_unsafe_blocks`), `panic = "abort"`, `overflow-checks = true`, `0` prod panics, monotonic `Instant`. |
| **`D1`** | **Doc-vs-Code Integrity (`0.80`)** | [When and how to write documentation](https://google.github.io/styleguide/rustguide.html#when-and-how-to-write-documentation) | Cross-checks runtime/concurrency primitives claimed in `docs/ARCHITECTURE.md` (`CancellationToken`, `JoinSet`, `tokio::select!`, `DelayQueue`) against actual `src/` occurrences. |
| **`D2`** | **Reactor, Thread & Dep Footprint (`1.10`)** | [Don't start threads via `std::thread` ad-hoc](https://google.github.io/styleguide/rustguide.html#dont-start-new-threads-through-the-rust-standard-library-chrome), [Comprehensive Rust](https://google.github.io/comprehensive-rust/) | Flags vestigial `tokio` linkage (`<= 15` call sites), $O(N)$ fixed-timeout `poll_readable` loops, hybrid dedicated-reader caps, `thread::sleep` loops, and unchecked `Builder::spawn` calls. |
| **`D3`** | **Lock Topology & Ownership (`0.90`)** | [Avoid `Rc` and `Arc` cycles/bloat](https://google.github.io/styleguide/rustguide.html#avoid-rc-and-arc), [Handling poison](https://google.github.io/styleguide/rustguide.html#handling-poison) | Counts `Mutex<Weak<Self>>` (`-> Arc::new_cyclic`), `Mutex<Option<Arc<T>>>` (`-> OnceLock`), `>12`-lock God structs, `SeqCst`/`Relaxed` asymmetries, lock-tier docs, and `parking_lot/deadlock_detection`. |
| **`D4`** | **Error Surface & `#[must_use]` (`0.80`)** | [Reporting errors](https://google.github.io/styleguide/rustguide.html#reporting-errors), [Prefer `enum` over `bool`](https://google.github.io/styleguide/rustguide.html#prefer-enum-over-bool-in-apis), [`#[must_use]`](https://google.github.io/styleguide/rustguide.html#must_use-on-functions), [C-GOOD-ERR](https://rust-lang.github.io/api-guidelines/interoperability.html#c-good-err) | Verifies `let_underscore_must_use = "deny"`, counts un-annotated `let _ = <call>` sites, C-style `deserialize -> u32` (`0`-on-error) functions, `thiserror` coverage, and `bool` vs `Result` ratio. |
| **`D5`** | **Checked Wire Arithmetic (`0.60`)** | [Effective Rust Item 3](https://www.lurklurk.org/effective-rust/), ISO 26262-6 Table 1 (`1d`/`1g`) | Scans wire, IPC, segmentation, discovery, and Netlink codecs for `.saturating_add/sub/mul` masking corrupted wire offsets instead of `.checked_add/sub/mul` + `?`. |
| **`D6`** | **Crate-Root `unsafe` Quarantine (`0.40`)** | [Unsafe code](https://google.github.io/styleguide/rustguide.html#unsafe-code) | Verifies `0` `unsafe` blocks/fns outside `src/sys/` and `#![forbid(unsafe_code)]` on every binary crate root (`src/bin/*.rs`, `main.rs`). |
| **`D7`** | **Zero-Copy CoW Buffers (`0.40`)** | [Prefer borrows over deep copies](https://google.github.io/styleguide/rustguide.html#prefer-borrows-over-deep-copies), [Closure accessors](https://google.github.io/styleguide/rustguide.html#more-on-accessors-that-accept-closures) | Verifies copy-on-write `bytes::Bytes` / `Owned(Vec<u8>)` payload storage + scoped `with_slice` accessors instead of `Mutex<Vec<u8>>` deep clones. |

### Layer 2: Fresh-Context Semantic Audit Checklist (What Static Regexes Cannot Prove)

After generating `/tmp/arch_scorecard.json`, inspect the flagged modules (or launch the bundled `rust-hardening-reviewer` subagent) to verify four semantic invariants:
1. **Stale-FD Eviction on Reactor Reconnect**: In custom `poll(2)`/`mio` reactors, verify `register()` evicts stale entries with the same raw `fd` or dead `Weak<Endpoint>`, and the wakeup loop checks both `socket.pollable_fd() == Some(entry.fd)` and `(revents & libc::POLLNVAL) == 0` before calling `read_some()`.
2. **Signal Mask Ordering & Spawn Rollback**: In POSIX daemons using `sigwait`, verify `pthread_sigmask(SIG_BLOCK, ...)` runs on `main()` **before** spawning any worker/reactor threads (so workers inherit the blocked signal mask), and that if spawning the signal-waiter thread fails (`EAGAIN`), the daemon logs an error, stops the runtime, and exits non-zero rather than running deaf to `SIGTERM`.
3. **Code-vs-Table Lock Acquisition Order**: Pick the top 3 multi-lock endpoints/managers and verify every nested lock acquisition strictly ascends the tier numbers documented in `docs/ARCHITECTURE.md` (and never invokes upward callbacks while holding an endpoint/queue lock).
4. **Test Integrity**: Confirm `0` upstream tests were deleted or weakened during refactoring and every work package adds targeted regression tests (e.g., `EAGAIN` spawn failure, `usize::MAX` framing wrap-around, concurrent `Payload::equals(a, b)` vs `equals(b, a)`).

---

## Dependency-Ordered Class B Refactoring Sequence (`WP-A → WP-B → WP-C → WP-D`)

When turning audit findings into a refactoring pass under a frozen public API (`api_signatures.rs`) and benchmark suite, execute in this four-step dependency order to avoid touching codec/endpoint files twice:

```text
WP-A (Leaf Cleanup & Footprint)        WP-B (Error Surface -> Checked Math)
Drop vestigial tokio linkage,      ──► Define internal #[must_use] Result<T, E>
move bin unsafe + sigwait to sys/,     FIRST, enable let_underscore_must_use,
Mutex<Weak<Self>> -> Arc::new_cyclic   then convert saturating_* -> checked_* + ?
                                                          │
                                                          ▼
WP-D (Reactors & Lock Hierarchy)       WP-C (CoW Bytes Payload Backing)
Multiplex UDS/UDP/TCP on poll(2)   ◄── Refactor PayloadImpl to RwLock<Shared(Bytes)
reactors + WakeFd, replace sleep       | Owned(Vec<u8>)> with 0-alloc clone/equals
threads with TimerReactor, lock doc    after codec signatures in WP-B settle
```

> [!IMPORTANT]
> **Reviewer vs. Fixer Separation Rule**: When acting as the quality reviewer in a two-agent workflow, output **(1)** the calibrated `X.Y / 10` scorecard with `file:line` evidence and **(2)** a self-contained handover prompt ordered `WP-A` → `WP-D` for the implementation agent rather than editing production files directly.

---

## Deep-Dive References

- **[post_parity_refactoring_playbook.md](references/post_parity_refactoring_playbook.md)**: Production Rust templates for `SignalSetWaiter` (`pthread_sigmask` + `sigwait`), stale-FD-safe `poll(2)` reactors + `eventfd` wakeups, checked worker thread spawning with rollback, 3-class `let _ =` triage, `read_wire_slice` checked cursors, and `PayloadStorage::Shared(Bytes)` CoW backing preserving C++ `capacity()` invariants.
