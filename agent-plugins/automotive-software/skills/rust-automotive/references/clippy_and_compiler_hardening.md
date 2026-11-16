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

# Clippy & Compiler Hardening for ISO 26262 Automotive Rust

This reference provides drop-in `Cargo.toml` and `clippy.toml` configurations for safety-critical automotive Rust workspaces (`ASIL-A` through `ASIL-D`), along with a mapping from **ISO 26262-6:2018** (Tables 1, 6, and 7) and **MISRA C++:2023** to `rustc` and `clippy` lints.

---

## 1. Common Gaps Without Workspace-Level Compiler Hardening

Without explicit workspace lints and profile settings, Rust projects commonly leave four safety gaps open:
1. Missing `overflow-checks = true` in `[profile.release]`, allowing silent integer wrap-around in production builds.
2. Missing `unwrap_used`, `expect_used`, and `indexing_slicing` denials in library crates.
3. Missing `disallowed-types` blocking `std::time::SystemTime` in timeout/TTL modules.
4. Missing per-block `// SAFETY:` proofs on `unsafe` blocks and `unsafe impl` items.

---

## 2. Drop-In Workspace `Cargo.toml` Configuration

Place the following configuration in the root `Cargo.toml` of your automotive Rust workspace, and inherit it in every member crate using `[lints] workspace = true`:

```toml
[workspace]
resolver = "2"
members = [
    "crates/*",
]

[workspace.package]
edition = "2021"
rust-version = "1.81.0"
license = "Apache-2.0"

# ============================================================================
# ISO 26262-6 / Ferrocene Compiler & Clippy Lints
# ============================================================================
[workspace.lints.rust]
# Require explicit `unsafe {}` blocks inside `unsafe fn` so every unsafe operation
# has a narrow lexical scope and its own `// SAFETY:` justification.
unsafe_op_in_unsafe_fn = "deny"

# Prevent silent data loss or ABI mismatch on FFI and wire types.
improper_ctypes = "deny"
improper_ctypes_definitions = "deny"
non_ascii_idents = "deny"

# Enforce unused result checking (`#[must_use]` on `Result` and safety guards).
unused_must_use = "deny"
unused_results = "warn"

# Prevent future-incompatibility hazards and missing documentation.
future_incompatible = { level = "deny", priority = -1 }
let_underscore_drop = "deny"
missing_debug_implementations = "warn"

[workspace.lints.clippy]
# --- Category 1: Zero-Panic Enforcement (ISO 26262-6 Table 1.1b, Table 6.1d) ---
unwrap_used = "deny"
expect_used = "deny"
panic = "deny"
panic_in_result_fn = "deny"
unimplemented = "deny"
todo = "deny"
unreachable = "deny"
indexing_slicing = "deny"
string_slice = "deny"
exit = "deny"

# --- Category 2: Checked Arithmetic & Numeric Casts (ISO 26262-6 Table 1.1g, MISRA 7.0.x) ---
# Set `arithmetic_side_effects = "deny"` on wire parsers & Tier-2 ASIL crates; may be set to `"warn"`
# in Tier-1 HPC daemon workspaces when `overflow-checks = true` is enforced in `[profile.release]` and `[profile.dev]`.
arithmetic_side_effects = "deny"
cast_possible_truncation = "deny"
cast_sign_loss = "deny"
cast_possible_wrap = "deny"
cast_precision_loss = "warn"
cast_lossless = "warn"
float_cmp = "deny"
float_arithmetic = "warn"

# --- Category 3: Memory, Stack, and Resource Bounds (ISO 26262-6 Table 6.1f, Annex D.2.3) ---
large_stack_arrays = "deny"
large_types_passed_by_value = "warn"
mem_forget = "deny"
rc_buffer = "deny"
rc_mutex = "deny"
infinite_iter = "deny"
fallible_impl_from = "deny"

# --- Category 4: Unsafe Code & Pointer Provenance (ISO 26262-6 Table 1.1e, MISRA 8.7.x) ---
undocumented_unsafe_blocks = "deny"
missing_safety_doc = "deny"
multiple_unsafe_ops_per_block = "deny"
ptr_as_ptr = "deny"
cast_ptr_alignment = "deny"
transmute_ptr_to_ptr = "deny"
not_unsafe_ptr_arg_deref = "deny"

# --- Category 5: Concurrency, Clocks, Error Handling, and Diagnostics (ISO 26262-6 Clause 7.4.4) ---
let_underscore_must_use = "deny" # Ban silent `let _ =` discards on Result / #[must_use] (Google Rust Style #reporting-errors)
disallowed_types = "deny"
disallowed_methods = "deny"
mutex_atomic = "warn"
await_holding_lock = "deny"
dbg_macro = "deny"
print_stdout = "deny"
print_stderr = "deny"
cognitive_complexity = "warn"
fn_params_excessive_bools = "deny"

# ============================================================================
# Deterministic & Hardened Build Profiles
# ============================================================================
[profile.dev]
opt-level = 0
debug = true
debug-assertions = true
overflow-checks = true
panic = "abort"

[profile.release]
opt-level = 3
debug = true              # Preserve DWARF symbols for DLT crash decoding and WCET analysis
debug-assertions = false
overflow-checks = true    # CRITICAL: Never allow silent integer wrap-around in production
lto = "thin"
codegen-units = 1         # Deterministic codegen for safety qualification
panic = "abort"           # Eliminate .eh_frame unwinding tables and catch_unwind complexity
```

In each member crate's `Cargo.toml`, inherit the workspace lints:

```toml
[package]
name = "someip-wire"
version.workspace = true
edition.workspace = true

[lints]
workspace = true
```

---

## 3. Drop-In `clippy.toml` Configuration

Place this `clippy.toml` file in the root of your workspace (alongside `Cargo.toml`). It configures stack-frame limits, cognitive complexity thresholds, and bans hazardous standard-library types and methods (`SystemTime`, `unbounded_channel`, `mem::forget`):

```toml
# Reject stack arrays larger than 4096 bytes to prevent stack overflow on bounded RTOS/QNX threads.
max-include-file-size = 65536
array-size-threshold = 4096
pass-by-value-size-limit = 256
too-large-for-stack = 1024

# Enforce small, testable functions for ISO 26262-6 Clause 9 MC/DC closure.
cognitive-complexity-threshold = 15
max-fn-params-bools = 2
type-complexity-threshold = 250

# Allow `.unwrap()` and `.expect()` inside `#[cfg(test)]` unit tests while keeping
# them strictly denied in production library/daemon targets.
allow-unwrap-in-tests = true
allow-expect-in-tests = true
allow-dbg-in-tests = true
allow-print-in-tests = true

# Disallow wall-clock time types and non-Send/poisoning primitives in safety-critical crates.
disallowed-types = [
    { path = "std::time::SystemTime", reason = "Non-monotonic wall-clock time jumps under NTP/gPTP synchronization; use std::time::Instant or tokio::time::Instant (CLOCK_MONOTONIC) for timeouts, debouncing, and TTLs." },
    { path = "std::collections::LinkedList", reason = "Causes per-node heap allocation and cache misses; use a fixed-capacity ring buffer or VecDeque with pre-reserved capacity." },
    { path = "std::rc::Rc", reason = "Non-atomic reference counting is !Send and risks reference cycles; use ownership or bounded actor channels." },
]

# Disallow unbounded channels, wall-clock epochs, and leak/forget primitives.
# `allow-invalid = true` on third-party paths: Clippy reports paths that do not resolve
# in a crate's dependency graph (e.g. a workspace member without tokio) as a config error.
disallowed-methods = [
    { path = "tokio::sync::mpsc::unbounded_channel", allow-invalid = true, reason = "Unbounded channels violate ISO 26262-6 Annex D.2.3 resource quotas and allow a noisy client to exhaust ECU memory; use tokio::sync::mpsc::channel(N)." },
    { path = "std::sync::mpsc::channel", reason = "std::sync::mpsc::channel is unbounded; use std::sync::mpsc::sync_channel(N) or tokio::sync::mpsc::channel(N)." },
    { path = "std::mem::forget", reason = "Leaking resources or bypassing Drop guards violates RAII lifecycle invariants; use ManuallyDrop with a documented SAFETY block only when required for FFI." },
    { path = "std::boxed::Box::leak", reason = "Dynamic memory leaks are prohibited after initialization; use static_cell::StaticCell for immortal singletons." },
]
```

---

## 4. ISO 26262-6:2018 & MISRA C++:2023 Mapping Matrix

Use this table in Software Safety Plan (SSP) documentation to map `rustc` and `clippy` lints to ISO 26262-6 coding guidelines:

| ISO 26262-6:2018 Reference | Topic / Safety Requirement | MISRA C++:2023 Equivalent | Rust / Clippy Enforcement Mechanism | C/C++ Hazard Prevented |
|---|---|---|---|---|
| **Table 1, 1a** | Enforcement of low complexity | MISRA Dir 4.10 | `clippy::cognitive_complexity`, `clippy::type_complexity` (`cognitive-complexity-threshold = 15`) | Deeply nested multi-clause boolean conditions that block MC/DC verification. |
| **Table 1, 1b** | Use of language subsets (exclude hazardous constructs) | MISRA Rule 18.0.1 (no exceptions/unwinding in ASIL) | `panic = "abort"`, `clippy::unwrap_used`, `clippy::expect_used`, `clippy::panic`, `clippy::indexing_slicing` | Uncaught C++ exceptions or runtime `assert!()` / `.unwrap()` aborts in library code. |
| **Table 1, 1c** | Enforcement of strong typing | MISRA Rule 7.0.1–7.0.6 | Rust type system + Newtype pattern (`struct ServiceId(u16)`), `SteadyInstant` vs `SystemTime` (`disallowed-types`) | Mixing `CLOCK_REALTIME` (`SystemTime`) and `CLOCK_MONOTONIC` (`Instant`) across timer APIs. |
| **Table 1, 1d** | Use of defensive implementation techniques | MISRA Dir 4.11 (check validity of values) | `#[must_use]` `Result<T, E>`, `slice::get()`, `checked_add()`, `checked_sub()`, `TryFrom<u8>` | Unchecked raw byte-pointer reads when datagram size is shorter than header length. |
| **Table 1, 1e** | Use of well-trusted design principles (encapsulation of `unsafe`) | MISRA Rule 8.7.1 (pointer arithmetic) | `rust::unsafe_op_in_unsafe_fn`, `clippy::undocumented_unsafe_blocks`, `clippy::missing_safety_doc`, `clippy::multiple_unsafe_ops_per_block` | Unchecked pointer offset writes before buffer bounds validation. |
| **Table 1, 1f** | No unambiguous / implicit conversions | MISRA Rule 7.0.5 (no narrowing conversions) | `clippy::cast_possible_truncation`, `clippy::cast_sign_loss`, `clippy::cast_possible_wrap`, `clippy::ptr_as_ptr` | Silent `static_cast<uint16_t>(len)` truncation when payloads exceed 64 KiB. |
| **Table 1, 1g / Table 6, 1d** | Prevention of arithmetic overflow & underflow | MISRA Rule 7.0.6 (unsigned wrap-around) | `overflow-checks = true` in `[profile.release]`, `clippy::arithmetic_side_effects` | Unsigned subtraction underflow passed to `memset()` or manual wrap-around checks. |
| **Table 6, 1b / 1c** | No dynamic objects or unbounded recursion / bounded stack | MISRA Rule 21.6.1 (no dynamic heap after init), Rule 17.2.1 (no recursion) | `rustc` `-Z emit-stack-sizes`, `clippy::large_stack_arrays`, `disallowed-methods` (`unbounded_channel`), `try_reserve_exact`, `FixedRingBuffer` | Unbounded `std::vector` or reassembly map growth under packet floods. |
| **Table 6, 1e / Clause 7.4.4** | No deadlocks or data races in concurrent execution | Concurrency & lock-order rules | Rust `Send`/`Sync` borrow checker + `clippy::await_holding_lock` + actor channels / N-tier lock hierarchy | `std::recursive_mutex` callback re-entrancy and lock-order inversions. |

---

## 5. Compiler Flags for Stack Usage & Symbol Verification

To verify **ISO 26262-6 Clause 7.4.6 / Annex D.2.3** bounded stack usage on Nightly / Ferrocene toolchains:

```bash
# Emit per-function stack frame sizes into the ELF `.stack_sizes` section
RUSTFLAGS="-Z emit-stack-sizes -C link-dead-code=no" cargo +nightly build --release

# Inspect functions exceeding 1024-byte stack frames using llvm-readobj
llvm-readobj --stack-sizes target/release/libsomeip_wire.rlib
```

To verify that zero unwinding panic symbols (`rust_begin_unwind` with unwind tables or `__cxa_throw`) remain in an `ASIL` binary compiled with `panic = "abort"`:

```bash
nm -C target/release/someip_daemon | grep -E "unwind_resume|__gxx_personality"
```
(Expected output: empty — zero unwinding personality routines linked.)
