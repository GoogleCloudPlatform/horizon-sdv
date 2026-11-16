---
name: rust-automotive
description: >-
  Enforces safety-critical automotive Rust engineering practices aligned with
  ISO 26262-6:2018 (Tables 1 and 6), ISO 26262-8 Clause 12, Ferrocene, MISRA,
  and Google's FOSS Rust Style Guide. Covers workspace compiler and Clippy lint
  hardening, curated automotive crate selection (tokio, mio, socket2, bytes,
  zerocopy, heapless, thiserror), panic-free wire parsing without unwrap or
  expect, bounded memory and fixed-capacity queues, monotonic clock enforcement
  (Instant vs SystemTime), documented unsafe invariants, and Miri/Kani proofs.
  Use when: authoring, reviewing, or linting automotive Rust crates, selecting
  dependencies, eliminating panics or unbounded allocations in networking or
  logging daemons, or writing Kani proofs. Don't use for: C++ Boost.Asio to Rust
  migration (use cpp-asio-to-rust-migration), extern "C" FFI and Annex D
  isolation (use iso26262-ffi-isolation), or LLVM MC/DC coverage analysis (use
  iso26262-mcdc-and-coverage).
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

# Safety-Critical Automotive Rust (`rust-automotive`)

Guides engineering practices aligned with ISO 26262-6:2018 (Tables 1, 6, and 7), ISO 26262-8:2018 Clause 12, Ferrocene toolchain guidelines, MISRA defensive coding rules, and **[Google's FOSS Rust Style Guide](https://google.github.io/styleguide/rustguide.html)** across `ASIL-A`..`ASIL-D` partitions and `QM` HPC Linux middleware.

## Quick Start

| Goal | Command |
|---|---|
| **Run built-in 19-rule self-test** | `python3 skills/rust-automotive/scripts/lint_automotive_rust.py --self-test` |
| **Enforce CI / milestone safety gate (`--strict`)** | `python3 skills/rust-automotive/scripts/lint_automotive_rust.py --path /path/to/rust-workspace --strict` |
| **Verify zero production `tokio`/`mio` bloat (sync daemons)** | `cargo tree -p <facade_crate> -e normal --locked` |
| **Score post-parity Rust architecture (`1–10`)** | See sibling skill `rust-systems-hardening-audit` (`scripts/audit_rust_architecture.py`) |

### Authoritative FOSS Google & Ecosystem Rust Standards
- **[Google Rust Style Guide](https://google.github.io/styleguide/rustguide.html)**: [`#reporting-errors`](https://google.github.io/styleguide/rustguide.html#reporting-errors), [`#prefer-enum-over-bool-in-apis`](https://google.github.io/styleguide/rustguide.html#prefer-enum-over-bool-in-apis), [`#must_use-on-functions`](https://google.github.io/styleguide/rustguide.html#must_use-on-functions), [`#avoid-rc-and-arc`](https://google.github.io/styleguide/rustguide.html#avoid-rc-and-arc), [`#prefer-borrows-over-deep-copies`](https://google.github.io/styleguide/rustguide.html#prefer-borrows-over-deep-copies), [`#unsafe-code`](https://google.github.io/styleguide/rustguide.html#unsafe-code), [`#dont-start-new-threads-through-the-rust-standard-library-chrome`](https://google.github.io/styleguide/rustguide.html#dont-start-new-threads-through-the-rust-standard-library-chrome), and [`#prefer-pattern-matching-over-check-then-unwrap`](https://google.github.io/styleguide/rustguide.html#prefer-pattern-matching-over-check-then-unwrap).
- **[Comprehensive Rust (Google Android)](https://google.github.io/comprehensive-rust/)**, **[Effective Rust](https://www.lurklurk.org/effective-rust/)**, and **[Rust API Guidelines (`C-GOOD-ERR`)](https://rust-lang.github.io/api-guidelines/interoperability.html#c-good-err)**.

### Applicability: QM vs. ASIL

| Measure | QM Component (e.g., HPC Linux Middleware) | ASIL Partition |
|---|---|---|
| Pillars 1–4 (lints, zero-panic parsing, bounded memory, monotonic clocks) | Required | Required |
| `arithmetic_side_effects`, `cast_*` lints | `warn` (with `overflow-checks = true`) | `deny` |
| Crate catalog | Tier 1 | Tier 2 + ISO 26262-8 Cl. 12 qualification |
| Miri | Crates containing `unsafe` without OS socket/epoll I/O | Required for all `unsafe` |
| Kani proofs, MC/DC, qualified toolchain (Ferrocene) | Optional / informational | Per safety plan |
| Strong typing of IDs (newtypes, ISO 26262-6 Table 1, `1c`) | Recommended for new APIs; C++-compatible mirrors may use aliases + additive newtypes | Required at partition boundaries |

A QM component makes **no** ISO 26262 ASIL claim; state the QM classification and assumptions of use in the project `README.md`.

---

## Bundled Static Auditor (`scripts/lint_automotive_rust.py`)

Fast, zero-dependency lexical/regex pre-flight scanner complementing `rustc` and `cargo clippy`:

```bash
python3 skills/rust-automotive/scripts/lint_automotive_rust.py \
  --path /path/to/rust-workspace \
  --strict \
  --format markdown \
  --output /tmp/rust_automotive_audit.md
```

- `--path <dir_or_file>`: Target Rust workspace, crate directory, or `.rs` file (skips `.git`, `.cargo`, `target`; skips `#[cfg(test)]` and `tests/` unless `--include-tests` is passed).
- `--strict` / `--fail-on {HIGH,MEDIUM,LOW}`: Exits `1` when findings at or above threshold (`MEDIUM` default) exist. Always pass `--strict` in CI.
- `--format {markdown,json,text}`, `--output <file>`, `--self-test` (verifies all `19` `RUST-AUTO-*` rules across compliant and non-compliant fixtures).

---

## Six Core Safety Pillars

### Pillar 1: Workspace Compiler & Clippy Hardening (`RUST-AUTO-CARGO-001`..`003`)
- **Build Profiles**: Set `panic = "abort"` and `overflow-checks = true` in both `[profile.release]` and `[profile.dev]` so release builds never wrap silently on `+`/`-`/`*`.
- **Mandatory Denials**: Deny `unwrap_used`, `expect_used`, `panic`, `indexing_slicing`, `let_underscore_must_use`, `await_holding_lock`, `undocumented_unsafe_blocks`, `missing_safety_doc`, `mem_forget`, `large_stack_arrays`, `dbg_macro`, `todo`, `unimplemented`, and `rust::unsafe_op_in_unsafe_fn`.
- **Zero Silent `let _ =` & C-Style `bool` Errors ([`#reporting-errors`](https://google.github.io/styleguide/rustguide.html#reporting-errors), [`#must_use-on-functions`](https://google.github.io/styleguide/rustguide.html#must_use-on-functions))**: Return `#[must_use] Result<T, E>` (`thiserror`) on internal codecs/I/O instead of `bool`/`0`; route OS thread spawns through a single `sys::spawn_os_thread` helper (`RUST-AUTO-THR-001`) that rolls back `AtomicBool` running flags on `EAGAIN`; annotate any intentional C++-parity discard with `#[expect(clippy::let_underscore_must_use, reason = "...")]`.
- **Full `Cargo.toml` + `clippy.toml` Templates**: Load [clippy_and_compiler_hardening.md](references/clippy_and_compiler_hardening.md).

### Pillar 2: Zero-Panic Wire Parsing & Checked Arithmetic (`RUST-AUTO-PANIC-*`, `RUST-AUTO-INDEX-001`)
- Never index untrusted network/IPC slices with `buf[offset..offset + 4]` or `.unwrap()`, and **never** use `.saturating_add()`/`.saturating_sub()`/`.saturating_mul()` in wire/IPC/segmentation parsers to silence `clippy::arithmetic_side_effects` (saturation clamps corrupted lengths to `0` or `usize::MAX`).
- Always combine `.checked_add()`/`.checked_sub()`/`.checked_mul()` + `?` with `slice.get(offset..end)` (`WireReader` template in [miri_kani_and_bounded_memory_patterns.md](references/miri_kani_and_bounded_memory_patterns.md)):

```rust
let end = offset.checked_add(HEADER_LEN).ok_or(WireError::ArithmeticOverflow)?;
let hdr = buf.get(offset..end).ok_or(WireError::UnexpectedEof { needed: end, remaining: buf.len() })?;
```

### Pillar 3: Bounded Memory, Fixed Pools & Queue Quotas (`RUST-AUTO-MEM-001`..`003`)
- Ban `tokio::sync::mpsc::unbounded_channel`, `std::sync::mpsc::channel`, `mem::forget`, and `Box::leak` via `clippy.toml` `disallowed-methods`.
- Validate `msg_size <= max_pdu_bytes` and `checked_add` queue byte quotas before calling `Vec::try_reserve_exact(msg_size)` (Tier 1), or use const-generic `FixedRingBuffer<T, N>` / `heapless` pools (Tier 2). Full `EndpointQueueQuota` and `FixedRingBuffer` implementations: [miri_kani_and_bounded_memory_patterns.md](references/miri_kani_and_bounded_memory_patterns.md).

### Pillar 4: Monotonic Clock Enforcement (`RUST-AUTO-CLOCK-001`)
- Ban `std::time::SystemTime` via `clippy.toml` `disallowed-types`. Measure all timeouts, discovery TTLs, debounce windows, and watchdogs with `std::time::Instant` (`CLOCK_MONOTONIC`) so NTP/gPTP clock steps cannot trigger false timeouts or hangs.
- *Narrow Logger Exception*: If a console/file log formatter requires calendar wall-clock timestamps, isolate that single call in a logger helper annotated with `#[allow(clippy::disallowed_types)]` and `// RUST-AUTO-ALLOW: CLOCK-001 (human-readable log timestamp only)`.

### Pillar 5: Audited `mod sys` Quarantine, Binary `#![forbid(unsafe_code)]`, Miri & Kani (`RUST-AUTO-UNSAFE-*`, `RUST-AUTO-BIN-001`)
- **Pure logic crates**: `#![forbid(unsafe_code)]` in `src/lib.rs`.
- **Crates requiring POSIX syscalls (`fcntl`, `atexit`, `fork`, `pthread_sigmask`)**: Use `#![deny(unsafe_code)]` in `src/lib.rs` with a single `#[allow(unsafe_code)] pub(crate) mod sys;` (avoids the failure mode where `#![forbid(unsafe_code)]` causes ports to silently stub OS syscalls). Require `# Safety` docs, per-block `// SAFETY:` proofs, `catch_unwind` at `extern "C"` callbacks, and OS-level integration tests.
- **Binary crate roots (`src/main.rs`, `src/bin/*.rs`)**: Do **not** inherit `lib.rs` lints — place `#![forbid(unsafe_code)]` at the top of every binary root and delegate daemonization/signals to `sys`.

### Pillar 6: Two-Tier Crate Selection & ISO 26262-8 Clause 12 Hygiene (`RUST-AUTO-DEP-001`..`003`)

| Domain | Tier 1 Preferred (HPC Linux Daemon) | Tier 2 Preferred (Strict ASIL-B/D / `no_std`) | Disallowed in Production `[dependencies]` |
|---|---|---|---|
| **Async / Reactor** | `tokio` (pruned features — **never** `"full"`), `tokio-util`, `mio` | `mio` (`net`, `os-poll`), `futures` (`default-features = false`) | `tokio = { features = ["full"] }` (`DEP-001`), `async-std` (`DEP-003`) |
| **Sockets & Syscalls** | `socket2`, `rustix` | `libc` (`default-features = false`) | Unwrapped raw `libc::setsockopt` where `socket2` suffices |
| **Buffers & Parsing** | `bytes`, `zerocopy`, `bitflags` | `heapless`, `zerocopy`, `bitflags` | `byteorder` (`DEP-003`), `bincode` v1 on untrusted sockets |
| **Errors & Sync** | `thiserror`, `std::sync::{LazyLock, OnceLock}`, `crossbeam-queue` | `thiserror` (`no_std`), `core::sync::atomic` | `anyhow` / `eyre` in lib/daemon crates (`DEP-002`), `lazy_static` / `once_cell` (`DEP-003`) |

---

## Verification Workflow (`Audit → Harden → Prove`)

1. **Static Audit**: `python3 skills/rust-automotive/scripts/lint_automotive_rust.py --path . --strict`
2. **Compiler & Clippy Hardening**: Apply `Cargo.toml` + `clippy.toml` from [clippy_and_compiler_hardening.md](references/clippy_and_compiler_hardening.md) and run `cargo clippy --workspace --all-targets -- -D warnings`.
3. **Formal / UB Verification**:
   ```bash
   MIRIFLAGS="-Zmiri-strict-provenance -Zmiri-symbolic-alignment-check" cargo +nightly miri test
   cargo kani --all-targets
   ```

## Deep-Dive References

- **[clippy_and_compiler_hardening.md](references/clippy_and_compiler_hardening.md)**: Drop-in workspace `Cargo.toml` and `clippy.toml` templates, stack-usage (`-Z emit-stack-sizes`) checks, and ISO 26262-6 / MISRA C++:2023 lint mapping matrix.
- **[miri_kani_and_bounded_memory_patterns.md](references/miri_kani_and_bounded_memory_patterns.md)**: Complete `WireReader`, `FixedRingBuffer<T, N>`, `EndpointQueueQuota`, `TypedTimestamp<SteadyDomain>`, audited `mod sys` / `init_shm_ring`, and `cargo miri` + `#[kani::proof]` harnesses.
- **[automotive_crate_catalog_and_qualification.md](references/automotive_crate_catalog_and_qualification.md)**: Two-tier automotive crate catalog, approved feature flags, banned crates (`RUST-AUTO-DEP-*`), and the 6-step ISO 26262-8 Clause 12 qualification checklist.
