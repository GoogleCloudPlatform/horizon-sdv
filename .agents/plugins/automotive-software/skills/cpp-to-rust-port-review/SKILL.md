---
name: cpp-to-rust-port-review
description: >-
  Independent, read-only fidelity review of a C/C++-to-Rust port increment.
  Compares translated Rust modules function by function against original C/C++
  sources (API coverage, control flow, constants, wire/IPC layout and endianness,
  state machines, timers, error paths, dropped logic), verifies doc-vs-code truth,
  and checks Rust translation hazards against Google's FOSS Rust Style Guide.
  Produces a PASS/FAIL JSON verdict with file:line evidence. Use when: verifying
  a migration milestone or work package in a fresh-context subagent, auditing
  behavioral equivalence and test-weakening, or gating a port merge. Don't use
  for: performing the translation itself (use cpp-asio-to-rust-migration) or
  post-parity whole-crate architectural scoring (use rust-systems-hardening-audit).
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

# C/C++ → Rust Port Fidelity Review

Answer one question with evidence: **is this Rust code a faithful, functionally equivalent translation of the listed C/C++ sources?** Read code, not claims. Commit messages, worker reports and comments are not evidence. Run this review in a **fresh context** (a new subagent per review round) that did not take part in the translation. Never edit tracked files; throwaway checks go under `/tmp` only.

## Quick Start (Fresh-Context Reviewer Checklist)

| Step | Action |
|---|---|
| **1. Run baseline structural & lint gates** | `./tools/verify.sh` (or single steps `./tools/verify.sh fmt` / `clippy` if tests are already running) |
| **2. Audit provenance & API mirror** | Run `verify_port_provenance.py --strict` and `audit_api_mirror.py --strict` |
| **3. Inspect code function-by-function** | Compare every file `> 500` LOC (and `>= 5` core files) line-by-line against C++ and **[Google's FOSS Rust Style Guide](https://google.github.io/styleguide/rustguide.html)** |
| **4. Emit machine-readable verdict** | Return the single `PASS \| FAIL` JSON object below with `file:line` evidence |

## Inputs (the caller provides them; if one is missing, return FAIL with "missing input")

1. **Scope:** the original C/C++ files of the increment and the path of a read-only reference checkout.
2. **Targets:** the Rust files that translate them. They are usually found through a `Provenance:` header that names the source file.
3. **Review range:** e.g. `git diff <base>..HEAD` plus uncommitted changes.
4. **Invariants:** the project document that lists mandated deviations and technical rules (endianness, panic policy, concurrency model). Any deviation not listed there is not permitted.
5. Optional: test commands and interop/differential scripts with their expected results.

## Procedure

1. **Coverage.** Map every in-scope source file to at least one Rust target, or to an explicit waiver with a concrete reason. Unmapped files are blocking.
2. **Function-by-function review** of the largest and most complex sources (at least 5, or all if fewer, always including every file over 500 LOC). For each:
   - every public/protected function has a Rust equivalent, or a documented, justified merge;
   - control flow, branch conditions, constants, defaults, timeouts and retry counts match;
   - wire and file formats match: field order, widths, endianness, length fields, padding, alignment, magic values, flags and offsets;
   - state machines have the same states, transitions, timers and re-entry behaviour;
   - error and edge paths are preserved: malformed input, size and queue limits, timeouts, disconnects, shutdown while busy;
   - nothing is silently dropped: no `todo!()`, stubbed branches, `Ok(())` placeholders, catch-all `_ =>` arms or `#[allow(dead_code)]` hiding unported logic.
3. **Translation hazards** (search, then read the hits in context):
   - panics on external input: `unwrap`, `expect`, unchecked indexing or arithmetic on parsed data;
   - unbounded channels, or allocations sized directly from wire length fields;
   - locks held across `.await` or across user/parent callbacks (including non-blocking `try_write` / `flush_queue` error-escalation paths); re-entrant locking emulated with `RefCell`; registry-to-socket lock-order inversions;
   - `read_exact` / `write_all` used directly as `select!` branches (not cancel-safe);
   - `SystemTime` used for timeouts instead of a monotonic clock;
   - `Arc` ownership cycles where the original used `shared_from_this` plus weak back-links;
   - silently stubbed OS syscalls (`fcntl` lockfiles, `atexit`, `nice`, `stat`, `SO_PEERCRED`) hidden behind `#![forbid(unsafe_code)]` instead of implemented in an audited `mod sys` boundary;
   - wire/IPC enums defined as strict Rust `enum` types that reject unknown raw byte values instead of `#[repr(transparent)]` `u8` newtypes when the upstream C++ stores raw `uint8_t`;
   - `saturating_add` / `saturating_sub` / `saturating_mul` used in wire, IPC, TP, SD, or Netlink parsers to silence `clippy::arithmetic_side_effects`, clamping malformed lengths instead of returning `Err(...)` via `checked_*` + `?`;
   - silent error swallowing (`let _ = <fallible_call>` without an upstream `file:line` `#[expect(clippy::let_underscore_must_use, reason = "Ignored as upstream (...)")]` justification, unchecked `thread::Builder::spawn`, or C-style `0`-on-error `deserialize -> u32` contracts — see [Google Rust Style `#reporting-errors`](https://google.github.io/styleguide/rustguide.html#reporting-errors));
   - `Mutex<Weak<Self>>` instead of `Arc::new_cyclic` ([Google Rust Style `#avoid-rc-and-arc`](https://google.github.io/styleguide/rustguide.html#avoid-rc-and-arc)), asymmetric `SeqCst` / `Relaxed` stop flags, or ad-hoc `thread::sleep` polling threads;
   - `unsafe` without a `// SAFETY:` justification, `unsafe` placed in binary roots (`src/bin/*.rs` / `main.rs`) outside `mod sys`, or FFI types without `#[repr(C)]` layout tests.
4. **Ported tests & test-weakening audit.** Count the original test cases in scope (`TEST`, `TEST_F`, `TEST_P`, …) and match every one to a Rust test by name and scenario:
   - Classify each ported test case as:
     - `FAITHFUL`: tests the exact same inputs, state transitions, and assertions as upstream C++;
     - `EQUIVALENT`: adapts the mechanics (e.g., in-process `testkit` instead of raw sockets) while proving the same behavioral invariants;
     - `STRENGTHENED`: covers everything upstream checks plus additional property/edge assertions;
     - `NOT-COMPILED-UPSTREAM`: upstream test is dead code / `#if 0` / excluded by the reference build profile (with `file:line` proof);
     - `WEAKENED` (**blocking**): assertions were dropped, multi-process/lock/state scenarios were replaced with trivial struct-construction checks, or the test only asserts "does not panic".
   - Every missing or `WEAKENED` test case is a **blocking** finding.
5. **Differential binary golden fixtures & execution.**
   - For wire protocols and local IPC command codecs, Rust-only round-trip tests (`serialize -> deserialize`) are insufficient because symmetric bugs (wrong endianness, swapped field order, wrong header size) pass round-trip testing. Verify that binary `.bin` / `.expected` golden fixtures generated by the C++ reference implementation are tested byte-for-byte against Rust.
   - Run the test commands and any differential/interop scripts you were given. Where reading is inconclusive (e.g., lockfile election or wire byte layout), write and run a small targeted probe under `/tmp`.
6. **Architecture doc-vs-code reality check.**
   - Cross-check every runtime crate, concurrency primitive (`tokio::select!`, `CancellationToken`, `JoinSet`, `BoardnetReactor`, `TimerReactor`), error-handling claim (`thiserror`), and `unsafe` boundary claimed in `docs/ARCHITECTURE.md` / `docs/DEVIATIONS.md` against actual `crates/*/src` code (`grep`). Aspirational documentation that contradicts the implementation is a **blocking** finding.

## Verdict

Return exactly one JSON object as the final message:

```json
{
  "verdict": "PASS | FAIL",
  "checks": [
    {"name": "<source file> vs <rust file>", "result": "PASS | FAIL", "evidence": "function-level notes with file:line"}
  ],
  "tests": {
    "original_cases": 0,
    "ported": 0,
    "faithful": 0,
    "equivalent": 0,
    "strengthened": 0,
    "justified_not_ported": 0,
    "weakened": [],
    "unjustified_missing": []
  },
  "blocking": ["<source file:line vs rust file:line: expected vs actual>"],
  "non_blocking": ["<style or follow-up notes>"]
}
```

- Any behavioural deviation not mandated by the invariants document is **blocking**. So are unmapped files, `WEAKENED` tests, and unjustified missing tests.
- PASS requires every check to be PASS and `blocking` to be empty.
- Every blocking item names both locations and the expected vs actual behaviour.
