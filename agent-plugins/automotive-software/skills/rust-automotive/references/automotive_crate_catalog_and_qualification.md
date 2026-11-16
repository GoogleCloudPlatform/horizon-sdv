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

# Curated Automotive Rust Crate Catalog & ISO 26262-8 Clause 12 Qualification Guide

This reference defines the preferred dependency stack, feature-flag pruning rules, banned/legacy crate list, and **ISO 26262-8:2018 Clause 12 (Qualification of software components)** workflow for automotive Rust projects (HPC Linux daemons, SOME/IP or IPC routers, and ASIL-partitioned ECU middleware).

---

## 1. Two-Tier Qualification Strategy (`HPC Daemon` vs. `Strict ASIL-B/D Partition`)

No third-party crate on `crates.io` ships with a pre-existing ISO 26262 certificate (Ferrocene qualifies the `rustc` compiler toolchain and `core` subset, not ecosystem crates). Under **ISO 26262-8 Clause 12** and **ISO 26262-6 Annex D (Freedom from Interference)**, select dependencies according to the software component's target ASIL and execution environment:

| Architectural Tier | Target Examples | Qualification Strategy | Recommended Runtime & Core Stack |
|---|---|---|---|
| **Tier 1: QM / ASIL-A / HPC Linux Daemon**<br>*(Process-isolated behind OS MMU, cgroups, and hardware watchdog)* | Network routing/discovery daemons, diagnostic logging daemons, RPC gateways, telemetry services | Prune unused features (`default-features = false`), enforce bounded queues, pin `Cargo.lock`, and qualify safety-supervising wrappers (e.g., E2E black-channel protector, watchdog client). | `tokio` (pruned features) or `mio`, `socket2`, `bytes`, `zerocopy`, `thiserror`, `tracing`, `serde` + `serde_json` (init-only) |
| **Tier 2: Strict ASIL-B to ASIL-D Partition**<br>*(Deterministic QNX/Linux safety island or `no_std` MCU)* | Safety monitor, shared-memory IPC runtime, brake/steering CAN/Ethernet actuator bridge, E2E black-channel library | Every linked crate must satisfy **ISO 26262-8 Clause 12** (specification, structural coverage per ISO 26262-6 Table 9 — Branch coverage `++` for ASIL-B/C, MC/DC `++` for ASIL-D — and known-errata audit). Minimize transitive LOC and `unsafe` surface area. | `mio` or `futures` (with static single-thread executor), `heapless`, `zerocopy`, `bitflags`, `thiserror` (`no_std`), `static_assertions` |

---

## 2. Preferred Automotive Rust Crate Matrix by Functional Domain

### 2.1 Async Runtime, Event Loop & Timers *(Replaces `boost::asio::io_context` & `steady_timer`)*

| Crate | Tier | Approved `Cargo.toml` Configuration | Automotive Rationale & Guardrails |
|---|---|---|---|
| **`tokio`** | Tier 1 (HPC Daemon) | `tokio = { version = "1", default-features = false, features = ["net", "rt", "rt-multi-thread", "sync", "time", "macros", "io-util", "signal"] }` | Async reactor for TCP/UDP/UDS endpoints (`io-util` provides `AsyncReadExt`/`AsyncWriteExt` for stream framing; `signal` provides clean `SIGTERM`/`SIGINT` daemon shutdown). **Never** use `features = ["full"]`, which pulls in `fs`, `process`, and `io-std` attack surface. |
| **`tokio-util`** | Tier 1 (HPC Daemon) | `tokio-util = { version = "0.7", default-features = false, features = ["rt", "time", "codec"] }` | Provides `CancellationToken` for structured shutdown, `DelayQueue<T>` for consolidating per-subscription TTL timers, and `FramedRead` for cancel-safe stream framing. |
| **`mio`** | Tier 1 & Tier 2 (ASIL-B) | `mio = { version = "1", default-features = false, features = ["net", "os-poll"] }` | ~5k LOC thin wrapper over `epoll`/`kqueue`. Ideal when deterministic single-threaded readiness polling with zero dynamic task allocation is required. |
| **`futures`** | Tier 2 (ASIL-B/D) | `futures = { version = "0.3", default-features = false }` | Defines zero-allocation `Future`/`Stream` traits over custom shared-memory or static single-threaded executors. |

### 2.2 Low-Level Socket Options & POSIX IPC *(Replaces `setsockopt`, `SO_BINDTODEVICE`, `SO_PEERCRED`)*

| Crate | Tier | Approved `Cargo.toml` Configuration | Automotive Rationale & Guardrails |
|---|---|---|---|
| **`socket2`** | Tier 1 & Tier 2 | `socket2 = { version = "0.5", default-features = false }` | Required to configure `SO_REUSEADDR`, `SO_BROADCAST`, `SO_RCVBUF`, `SO_BINDTODEVICE`, and IPv4/IPv6 multicast interface indices (`IPV6_MULTICAST_IF`) before converting descriptors into `mio` or `tokio` sockets. |
| **`rustix`** | Tier 1 | `rustix = { version = "0.38", default-features = false, features = ["net", "time", "fs", "process"] }` | Memory-safe, zero-overhead syscall wrappers (`getsockopt`, `SO_PEERCRED`, `clock_gettime`, `shm_open`) that eliminate raw `unsafe { libc::* }` pointer bugs. |
| **`libc`** | Tier 1 & Tier 2 | `libc = { version = "0.2", default-features = false }` | Keep minimal; use only for OS-specific constants or QNX Neutrino (`target_os = "nto"`) / Linux `SO_RCVBUFFORCE` FFI calls not yet covered by `socket2` or `rustix`. |

### 2.3 Wire Framing, Zero-Copy Parsing & Bounded Containers

| Crate | Tier | Approved `Cargo.toml` Configuration | Automotive Rationale & Guardrails |
|---|---|---|---|
| **`bytes`** | Tier 1 (HPC Daemon) | `bytes = { version = "1", default-features = false, features = ["std"] }` | Reference-counted `Bytes` and `BytesMut` allow zero-copy slicing when fanning out notification PDUs to multiple local UDS/TCP subscribers, plus checked cursor reads via `bytes::Buf`. |
| **`zerocopy`** | Tier 1 & Tier 2 (`no_std`) | `zerocopy = { version = "0.8", default-features = false, features = ["derive"] }` | Backed by formal Kani proofs (`FromBytes`, `IntoBytes`, `KnownLayout`, `Unaligned`). Replaces unsafe `reinterpret_cast` / `std::memcpy` header parsing with compile-time layout and alignment verification. |
| **`bitflags`** | Tier 1 & Tier 2 (`no_std`) | `bitflags = { version = "2", default-features = false }` | Type-safe bitmasks for protocol header flags (`Reboot`, `Unicast`, `ExplicitInitialDataControl`) and E2E status bits without undefined enum discriminant casts. |
| **`heapless`** | Tier 2 (`no_std` / ASIL) | `heapless = { version = "0.8", default-features = false }` | Provides `heapless::Vec<T, N>`, `String<N>`, and `spsc::Queue<T, N>` with compile-time fixed capacity, aligning with ISO 26262-6 Table 1 (`1f` *No dynamic objects or variables*). |

### 2.4 Error Handling, Synchronization & Configuration

| Crate | Tier | Approved `Cargo.toml` Configuration | Automotive Rationale & Guardrails |
|---|---|---|---|
| **`thiserror`** | Tier 1 & Tier 2 (`no_std` in v2) | `thiserror = { version = "2", default-features = false }` | Zero-cost `#[derive(Error)]` on exhaustive `#[must_use]` domain error enums (`WireError`, `RoutingError`, `E2eError`). Preserves structured fault variants so callers can execute deterministic fault reactions. |
| **`crossbeam-queue`** | Tier 1 | `crossbeam-queue = { version = "0.3", default-features = false, features = ["alloc"] }` | Provides `ArrayQueue<T>`, a pre-allocated, bounded, lock-free MPMC ring buffer for high-throughput log or packet pools. |
| **`parking_lot`** | Tier 1 (Optional) | `parking_lot = { version = "0.12", default-features = false }` | Non-poisoning `Mutex`/`RwLock` (avoids `.lock().unwrap()` panic cascades when a peer thread panics) and supports `features = ["deadlock_detection"]` in debug/CI builds. Prefer actor channels over shared mutexes first, and never use `ReentrantMutex<RefCell<T>>`. |
| **`serde` + `serde_json`** | Tier 1 (Init-Only) | `serde = { version = "1", default-features = false, features = ["derive", "std"] }`<br>`serde_json = { version = "1", default-features = false, features = ["std"] }` | Standard for parsing service/routing configurations (`"std"` enables `serde_json::from_reader` for multi-file directory config loading; use `"alloc"` for `no_std`). **Guardrail**: Invoke `serde_json` strictly during the **startup initialization phase** before entering the cyclic packet processing loop. |

### 2.5 Diagnostics, Logging & Verification (`[dev-dependencies]`)

| Crate | Tier | Approved `Cargo.toml` Configuration | Automotive Rationale & Guardrails |
|---|---|---|---|
| **`tracing`** | Tier 1 | `tracing = { version = "0.1", default-features = false, features = ["std", "attributes"] }` | Structured, async-aware instrumentation that attaches `service_id`, `instance_id`, `client_id`, and `session_id` spans across `.await` points; can bridge directly to DLT or OpenTelemetry. |
| **`log`** | Tier 1 & Tier 2 (`no_std`) | `log = { version = "0.4", default-features = false }` | Minimal facade for `no_std` or FFI logging bridges. |
| **`proptest`** | `[dev-dependencies]` | `proptest = "1"` | Property-based boundary-value testing (`ISO 26262-6 Table 8`) for serializer/deserializer roundtrips and segmentation/reassembly. |
| **`mockall`** | `[dev-dependencies]` | `mockall = "0.13"` | Mocks FFI/transport traits to inject hardware/network faults deterministically during unit testing (`ISO 26262-6 Table 7 1d`). |
| **`libfuzzer-sys` + `arbitrary`** | `fuzz/` workspace | `libfuzzer-sys = "0.4"`, `arbitrary = { version = "1", features = ["derive"] }` | Structure-aware coverage-guided fuzzing for ISO/SAE 21434 cybersecurity verification. |

---

## 3. Disallowed & Legacy Crates in Automotive Production Code (`RUST-AUTO-DEP-*`)

The bundled static auditor (`scripts/lint_automotive_rust.py`) flags the following dependency anti-patterns in `[dependencies]` and `[workspace.dependencies]`:

| Rule ID | Disallowed Crate / Pattern | Why It Fails Automotive Safety / Audit | Mandatory Replacement |
|---|---|---|---|
| **`RUST-AUTO-DEP-001`** | `tokio = { ..., features = ["full"] }` | Pulls in `fs`, `process`, `io-std`, and `parking_lot` unconditionally, inflating the ISO 26262-8 Clause 12 qualification surface and violating least-privilege build hygiene. | Specify `default-features = false` and list only required features (`["net", "rt", "rt-multi-thread", "sync", "time", "macros", "io-util", "signal"]`). |
| **`RUST-AUTO-DEP-002`** | `anyhow` or `eyre` in `[dependencies]` of a library/daemon crate | Erases concrete fault variants into a heap-allocated `Box<dyn Error>`, preventing callers from matching on recoverable protocol errors (`WireError`, `QuotaError`) and breaking deterministic error handling. | Use `thiserror` with exhaustive `#[must_use]` enums in libraries/daemons. Restrict `anyhow` to `[dev-dependencies]` or CLI binaries. |
| **`RUST-AUTO-DEP-003`** | `lazy_static` or `once_cell` | Superseded by Rust 1.70+ `std::sync::OnceLock` and Rust 1.80+ `std::sync::LazyLock`. Adds unnecessary external macro/atomic dependencies to the SBOM. | Use `std::sync::LazyLock` or `std::sync::OnceLock` from the standard library. |
| **`RUST-AUTO-DEP-003`** | `byteorder` | Superseded by Rust's built-in `u16::from_be_bytes`, `u32::from_be_bytes`, `u32::to_be_bytes`, and `bytes::Buf`. | Use `std` primitive endian conversions (`from_be_bytes` / `to_be_bytes`) or `bytes::Buf` / `zerocopy`. |
| **`RUST-AUTO-DEP-003`** | `async-std` | Unmaintained / discontinued runtime with unpatched soundness advisories in older transitive versions. | Use `tokio` (Tier 1) or `mio` / `futures` (Tier 2). |
| **`RUST-AUTO-DEP-003`** | `bincode` (v1) on untrusted network sockets | Reads a 64-bit length prefix from the wire and immediately calls `Vec::with_capacity(len)` before validating packet length, enabling single-packet ECU OOM crashes. | Write bounded wire parsers with `bytes` / `zerocopy` and pre-allocation quota checks. |

---

## 4. Drop-In Production `Cargo.toml` Dependency Template

```toml
[workspace]
resolver = "2"
members = [
    "crates/automotive-wire",
    "crates/automotive-e2e",
    "crates/automotive-runtime",
]

[workspace.dependencies]
# Tier 1 Async Runtime & Framing (strictly pruned feature flags — NO "full")
tokio = { version = "1.40", default-features = false, features = ["net", "rt", "rt-multi-thread", "sync", "time", "macros", "io-util", "signal"] }
tokio-util = { version = "0.7", default-features = false, features = ["rt", "time", "codec"] }
mio = { version = "1.0", default-features = false, features = ["net", "os-poll"] }
socket2 = { version = "0.5", default-features = false }

# Tier 1 & 2 Wire Parsing, Zero-Copy & Bounded Memory
bytes = { version = "1.7", default-features = false, features = ["std"] }
zerocopy = { version = "0.8", default-features = false, features = ["derive"] }
bitflags = { version = "2.6", default-features = false }
heapless = { version = "0.8", default-features = false }
crossbeam-queue = { version = "0.3", default-features = false, features = ["alloc"] }

# Error Handling, Diagnostics & Startup Config
thiserror = { version = "2.0", default-features = false }
tracing = { version = "0.1", default-features = false, features = ["std", "attributes"] }
serde = { version = "1.0", default-features = false, features = ["derive", "std"] }
serde_json = { version = "1.0", default-features = false, features = ["std"] }

# Verification & Testing Only ([dev-dependencies])
proptest = { version = "1.5", default-features = false, features = ["std"] }
mockall = { version = "0.13", default-features = false }
```

---

## 5. ISO 26262-8:2018 Clause 12 Qualification Checklist for Third-Party Crates

When linking a third-party crate into an `ASIL-A` through `ASIL-D` binary (without isolating it in a separate QM process), execute and document these 6 steps per **ISO 26262-8 Clause 12.4**:

1. **Pin Exact Version & Vendor Source (`12.4.2.1`)**:
   - Commit `Cargo.lock` and vendor exact crate sources (`cargo vendor`) so builds are 100% reproducible and immune to upstream yanks.
2. **Prune Feature Flags (`12.4.2.2`)**:
   - Set `default-features = false` on every dependency. Uncompiled feature branches do not enter the binary and are excluded from structural coverage (`MC/DC`) scope.
3. **Map Assumed Requirements & API Wrapper (`12.4.2.3`)**:
   - Wrap the third-party crate behind a narrow internal module/trait (e.g., `SteadyTimerWheel`, `HeaderSlice`) and document the exact subset of APIs used as Software Component Requirements (`SWC-REQ-*`).
4. **Audit `unsafe` Blocks & Run `cargo miri` / `cargo kani` (`12.4.3.2`)**:
   - Run `cargo geiger` or `lint_automotive_rust.py` on the vendored crate to inventory `unsafe` blocks, then execute your integration and property test suite under `MIRIFLAGS="-Zmiri-strict-provenance" cargo +nightly miri test`.
5. **Measure Structural Coverage on Active Call Paths (`12.4.3.3`)**:
   - Per ISO 26262-6:2018 Table 9, for `ASIL-B`/`ASIL-C` (Branch coverage `++`; MC/DC `+`) and `ASIL-D` (MC/DC coverage `++`), run the `iso26262-mcdc-and-coverage` workflow across the vendored crate functions reachable from your wrapper, and document any unreachable defensive branches via `COV_JUSTIFIED`.
6. **Continuous Vulnerability & Errata Monitoring (`12.4.4`)**:
   - Gate CI with `cargo audit` (RustSec advisory database `RUSTSEC-*`) to detect soundness advisories in transitive dependencies automatically.
