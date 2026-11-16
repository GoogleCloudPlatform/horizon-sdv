---
name: iso26262-mcdc-and-coverage
description: >-
  Measures, analyzes, and closes ISO 26262-6:2018 Clauses 9 and 10 structural
  code coverage (Statement, Branch, Function, and Modified Condition/Decision
  Coverage - MC/DC) for automotive Rust and C++ software using LLVM source-based
  coverage and llvm-cov JSON exports. Guides Equivalence Class and Boundary
  Value Analysis (BVA) test vectors, refactoring coupled or >6-condition boolean
  decisions, and auditing COV_JUSTIFIED markers. Use when: measuring MC/DC or
  branch coverage, parsing llvm-cov JSON reports, synthesizing N+1 independence
  pairs, or refactoring complex boolean guards. Don't use for: requirements
  traceability matrices (use traceability-audit), general Clippy linting (use
  rust-automotive), or SW-FMEA and SEooC authoring (use
  iso26262-seooc-and-sw-fmea).
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

# ISO 26262-6 Structural Coverage, MC/DC & Boundary Value Analysis (`iso26262-mcdc-and-coverage`)

Guides structural coverage closure aligned with ISO 26262-6:2018 Clause 9 (Software Unit Verification — Tables 7, 8, and 9) and Clause 10 (Software Integration & Verification — Table 12) for safety-critical automotive Rust and C++17 software (`ASIL-A` through `ASIL-D`), using LLVM's `llvm.coverage.json.export` (`v3.1.0`) schema and `rustc` `-Z coverage-options=mcdc` instrumentation.

## Quick Start

| Goal | Command |
|---|---|
| **Run built-in MC/DC & BVA self-test** | `python3 skills/iso26262-mcdc-and-coverage/scripts/analyze_mcdc_coverage.py --self-test` |
| **Static boolean decision scan (`>6` conditions & coupled vars)** | `python3 skills/iso26262-mcdc-and-coverage/scripts/analyze_mcdc_coverage.py --scan-source /path/to/crates --asil D --strict` |
| **Evaluate `llvm-cov export` JSON + `COV_JUSTIFIED` effective coverage** | `python3 skills/iso26262-mcdc-and-coverage/scripts/analyze_mcdc_coverage.py --llvm-json coverage.json --asil D --strict` |

## Why Naive Line Coverage Fails ISO 26262-6 Qualification

Standard line coverage (`cargo tarpaulin` or default `-C instrument-coverage`) is insufficient for automotive ASIL qualification because of five recurring failure modes:

1. **Missing Condition Independence Pairs (ISO 26262-6 Table 9 Method `1c` — MC/DC)**:
   Executing both the `true` and `false` outcomes of a multi-condition decision (`100%` branch coverage) does **not** prove that each atomic condition independently affects the decision outcome while holding other conditions fixed. A 5-condition port filter `A && ((B && C) || (D && E))` achieves 100% line and branch coverage with only 2 tests (`(T,T,T,F,F)->T` and `(F,T,T,F,F)->F`), leaving conditions `B`, `C`, `D`, and `E` with **0% MC/DC** (`ConditionsArray = [true, false, false, false, false]`).
2. **Strongly Coupled Conditions (`x` and `!x` in the Same Decision)**:
   When `is_multicast` (`C2`) and `!is_multicast` (`C4`) appear in two disjuncts of the same `if` statement (`reboot_flag && ((is_multicast && !mc_prev) || (!is_multicast && !uc_prev))`), `C2` and `C4` always flip together. Holding `C4` fixed while toggling `C2` is mathematically impossible under Unique-Cause MC/DC.
3. **Oversized Boolean Decisions (`>6` Conditions & LLVM 18 Bitmap Cap)**:
   LLVM 18 (`rustc` ~1.78–1.81) defaulted to `--mcdc-max-conditions=6` ($2^6$-bit test-vector bitmap per decision in `.llvmcovmap`), silently dropping MC/DC instrumentation on larger decisions unless `-C llvm-args=--mcdc-max-conditions=10` was passed. Although LLVM 19+ (`rustc 1.82+`) raised the internal bitmap limit, keeping decisions at $\le 6$ independent conditions remains essential for ISO 26262-6 Table 1 (`1a` low complexity) and tractable $N+1$ test synthesis.
4. **Rust `.rlib` Archive Rejection in `llvm-cov`**:
   When running `llvm-cov` across Rust static libraries (`.rlib` or `.a` archives produced by `cargo` or Bazel `rules_rust`), the archive starts with a `lib.rmeta` metadata member. `llvm-cov` rejects any archive whose first member is `lib.rmeta` with `"no coverage data found"` even though the subsequent `.o` members carry valid `.llvmcovmap` sections.
5. **Unverified Boundary Values & Undocumented Coverage Exclusions**:
   ISO 26262-6 Clause 9.4.4 Table 8 requires deriving unit tests from **Analysis of Equivalence Classes (`1b`)** and **Analysis of Boundary Values (`1c`)** (`++` for ASIL B/C/D). Furthermore, ISO 26262-6 Clause 9.4.5 NOTE 2 discourages suppressing coverage instrumentation via `#[coverage(off)]` to hide untested branches; dead defensive guards should remain instrumented and be justified with machine-audited `COV_JUSTIFIED` rationales that compute **Raw vs. Effective Coverage** and flag **Stale Justifications** whenever tests later cover the line.

---

## Bundled Coverage & Boolean Decision Analyzer (`scripts/analyze_mcdc_coverage.py`)

Run the bundled analyzer both **before** test execution (static boolean decision scan on Rust/C++ source files) and **after** test execution (parsing `llvm-cov export --format=text` JSON with optional `COV_JUSTIFIED` justifications):

```bash
# 1. Static source scan: detect >6-condition decisions, coupled variables, and synthesize N+1 MC/DC test vectors
python3 skills/iso26262-mcdc-and-coverage/scripts/analyze_mcdc_coverage.py \
  --scan-source /path/to/crate_or_cpp_dir \
  --asil D \
  --format markdown \
  --output /tmp/mcdc_source_audit.md

# 2. Post-run LLVM JSON analysis: evaluate Statement, Branch (9-tuple), Function, and MC/DC (11-tuple) + Effective Coverage
python3 skills/iso26262-mcdc-and-coverage/scripts/analyze_mcdc_coverage.py \
  --llvm-json /path/to/coverage_export.json \
  --justifications /path/to/coverage_justifications.yaml \
  --source-root /path/to/workspace \
  --asil D \
  --format markdown \
  --output /tmp/mcdc_coverage_report.md
```

### Supported CLI Flags
- `--llvm-json <path>`: Parses `llvm-cov export --format=text` JSON (`v3.1.0` schema) to audit `lines`, `functions`, `branches` (9-tuple), and `mcdc_records` (11-tuple) against ISO 26262-6 Table 9 & Table 12 ASIL targets, pinpointing uncovered conditions (`ConditionsArray[c] == false`) and synthesizing missing independence-pair vectors from `TestVectorsArray`.
- `--scan-source <path>` (or `--scan-rust <path>`): Statically scans `.rs`, `.cpp`, `.cc`, `.c`, `.hpp`, and `.h` files for multi-condition boolean decisions (`&&` / `||`), flags decisions exceeding `--max-conditions` (default `6`) or containing coupled variables (`x` and `!x`), and synthesizes the minimal $N+1$ Unique-Cause MC/DC test vectors.
- `--asil {A,B,C,D}`: Selects ISO 26262-6 Table 9 & Table 12 mandatory metrics (`A`: Statement; `B`: Statement + Branch; `C`: Statement + Branch + Function; `D`: Statement + Branch + Function + MC/DC; default: `D`).
- `--justifications <yaml_or_json>`: Loads a `coverage_justifications.yaml` / `.json` database (and scans `--source-root` for `// COV_JUSTIFIED <id>` and `// COV_JUSTIFIED_START <id>` / `// COV_JUSTIFIED_STOP` markers) to compute **Raw Coverage** vs. **Effective Coverage** and detect **Stale Justifications**.
- `--strict`: Exits with code `1` if mandatory ASIL metrics have unjustified gaps, dangling `COV_JUSTIFIED` IDs exist, or static source scan detects coupled / `> --max-conditions` boolean decisions.
- `--format {markdown,json,text}`: Output format (default: `markdown`).
- `--output <file>`: Writes the report to `<file>` (prints to `stdout` if omitted).
- `--self-test`: Generates `llvm-cov` v3.1.0 MC/DC JSON exports, Rust/C++ boolean fixtures, and `COV_JUSTIFIED` files in a temporary directory and verifies both gap detection and 100% effective closure.

---

## Five Core Engineering Pillars & Side-by-Side Patterns

### Pillar 1: Nightly Rust & LLVM Source-Based MC/DC Instrumentation

Rust integrates directly with LLVM's source-based coverage mapping (`.llvmcovmap` + `.llvmprf`). To emit `MCDCBranch` and `MCDCDecision` counter regions, compile at `-C opt-level=0` so LLVM's `SimplifyCFG` pass does not fold short-circuit branches into branchless `select` instructions.

#### Anti-Pattern: Default Line Coverage with Release Optimizations or Unexpanded `.rlib` Archives
```bash
# 1. Omit `-Z coverage-options=mcdc` -> `mcdc_records` is empty in llvm-cov export.
# 2. `--release` folds boolean expressions into branchless `select` instructions.
# 3. Passing `libfoo.rlib` directly to `llvm-cov` fails with "no coverage data found"
#    because `lib.rmeta` is the first ar archive member.
RUSTFLAGS="-C instrument-coverage" cargo test --release
llvm-cov export --instr-profile=merged.profdata target/release/libfoo.rlib
```

#### Recommended Pattern: Unoptimized MC/DC Instrumentation, Continuous Profiling, and `.o` Extraction
```bash
# 1. Instrument with MC/DC and raise LLVM's condition bitmap cap when legacy decisions have >6 conditions
export RUSTFLAGS="-C instrument-coverage -Z coverage-options=mcdc -C opt-level=0 -C debuginfo=2 -C llvm-args=--mcdc-max-conditions=10 -C llvm-args=-runtime-counter-relocation"
export LLVM_PROFILE_FILE="target/cov/ecu-%p-%m.profraw"
export LLVM_PROFILE_CONTINUOUS_MODE=1

cargo +nightly test --no-run

# 2. If inspecting baseline `.rlib` archives alongside test binaries, extract `.o` members first
#    (skipping `lib.rmeta` which causes llvm-cov to reject the archive):
mkdir -p target/cov/rlib_objs && cd target/cov/rlib_objs
for rlib in ../../debug/deps/*.rlib; do ar x "$rlib" && rm -f lib.rmeta; done && cd -

# 3. Execute tests, merge `.profraw` profiles, and export v3.1.0 JSON
cargo +nightly test
llvm-profdata merge -sparse target/cov/ecu-*.profraw -o target/cov/merged.profdata
llvm-cov export --format=text \
  --instr-profile=target/cov/merged.profdata \
  --ignore-filename-regex='(/.cargo/|/rustc/|tests/)' \
  target/debug/deps/ecu_router-* > target/cov/coverage_export.json
```

For the `llvm.coverage.json.export` v3.1.0 positional tuple schemas (`branches` 9-tuple and `mcdc_records` 11-tuple), `rustc_session` flag internals, and ISO 26262-6 Tables 7, 8, 9, and 12 mappings, load [llvm_mcdc_rust_workflow.md](references/llvm_mcdc_rust_workflow.md).

---

### Pillar 2: Refactoring Coupled Conditions & $>6$-Condition Decisions

Under ISO 26262-6 Table 9 (`1c`), an atomic condition $C_i$ achieves MC/DC only when a pair of executed test vectors $(V_a, V_b)$ toggles $C_i \in \{\text{T}, \text{F}\}$ and flips the decision result $D(V_a) \neq D(V_b)$ while every other evaluated condition $C_j$ ($j \neq i$) remains constant (or masked by short-circuiting). Whenever a variable $x$ and its negation $!x$ both appear in the same boolean expression, $x$ and $!x$ are **strongly coupled** and cannot vary independently.

#### Anti-Pattern: Coupled Complementary Conditions & 10-Condition Disjunctive Normal Form
```cpp
// 1. Coupled variable `is_multicast` (C2) and `!is_multicast` (C4):
//    Holding C4 fixed while toggling C2 is logically impossible!
if (reboot_flag
    && ((is_multicast && !prev_mc_reboot)
        || (!is_multicast && !prev_uc_reboot))) {
    result = true;
}

// 2. 10-condition DNF with coupled endpoints across 3 disjuncts:
//    Exceeds LLVM's 6-condition default bitmap limit AND couples `reliable_ep`
//    with `!reliable_ep`!
if ((reliable_ep && unreliable_ep && tcp_ready && udp_ready)
    || (reliable_ep && !unreliable_ep && tcp_ready)
    || (unreliable_ep && !reliable_ep && udp_ready)) {
    // ...
}
```

#### Recommended Pattern: Single-Occurrence Selector & Exhaustive `match` in Rust
```rust
/// Step 1 selects the relevant prior reboot flag (`is_multicast` evaluated once: 2 branches).
/// Step 2 evaluates 2 independent conditions (`reboot_flag && !prev_reboot_flag`),
/// requiring only N + 1 = 3 test vectors: (T,T)->T, (F,T)->F, (T,F)->F.
pub fn detect_peer_reboot(
    reboot_flag: bool,
    is_multicast: bool,
    prev_multicast_reboot: bool,
    prev_unicast_reboot: bool,
) -> bool {
    let prev_reboot_flag = if is_multicast {
        prev_multicast_reboot
    } else {
        prev_unicast_reboot
    };
    reboot_flag && !prev_reboot_flag
}

/// Exhaustive pattern match over `(has_reliable_ep, has_unreliable_ep)` eliminates all 10 coupled
/// DNF conditions and reduces each arm to <= 2 independent conditions.
pub fn is_subscription_transport_ready(
    has_reliable_ep: bool,
    has_unreliable_ep: bool,
    tcp_established: bool,
    udp_established: bool,
) -> bool {
    match (has_reliable_ep, has_unreliable_ep) {
        (true, true) => tcp_established && udp_established,
        (true, false) => tcp_established,
        (false, true) => udp_established,
        (false, false) => false,
    }
}
```

---

### Pillar 3: Eliminating Chained Combinators & De Morgan Double Negation

Chained closure combinators (`.and_then(|x| ...).or_else(|| ...).filter(|y| ...)`) split a single logical decision across multiple synthetic closure functions in `.llvmcovmap`, inflating function/instantiation counts and obscuring MC/DC condition pairs. Similarly, double negations (`!(A && !B) && !(C && !D)`) obscure which test vector proves each condition.

#### Anti-Pattern: Double Negation & Opaque Combinator Chains
```rust
pub fn should_nack_unreliable_bad(
    first_port: Option<u16>,
    first_reliable: bool,
    second_port: Option<u16>,
    second_reliable: bool,
) -> bool {
    first_port
        .filter(|_| !first_reliable)
        .or_else(|| second_port.filter(|_| !second_reliable))
        .is_none()
}
```

#### Recommended Pattern: Flat Single-Occurrence Boolean Predicates with Explicit $N+1$ Tests
```rust
/// Unreliable provider requires at least one configured port to be unreliable (UDP).
/// NACK iff neither port provides UDP: 4 independent conditions, 5 MC/DC test vectors.
pub fn should_nack_unreliable_provider(
    first_port_set: bool,
    is_first_reliable: bool,
    second_port_set: bool,
    is_second_reliable: bool,
) -> bool {
    let first_is_udp = first_port_set && !is_first_reliable;
    let second_is_udp = second_port_set && !is_second_reliable;
    !first_is_udp && !second_is_udp
}
```

For step-by-step truth-table derivations and complete Rust `#[test]` suites, load [bva_and_mcdc_refactoring_patterns.md](references/bva_and_mcdc_refactoring_patterns.md).

---

### Pillar 4: Equivalence Classes & Boundary Value Analysis (ISO 26262-6 Table 8 `1b`/`1c`)

Structural coverage alone cannot detect off-by-one relational errors (`>` vs. `>=`, `< 16` vs. `<= 16`). Every numeric, length, or alignment guard in binary wire protocols must be tested at **three points per boundary**: `boundary - 1`, `boundary`, and `boundary + 1` (plus `0` and `T::MAX`).

| Protocol Framing Rule | Guard Predicate | Mandatory BVA Test Points |
|---|---|---|
| RPC Header Minimum Length | `frame_len >= 16 && header_len >= 8` | `frame_len`: `0, 15, 16, 17`; `header_len`: `0, 7, 8, 9, u32::MAX` |
| Segmented Frame Minimum Size | `data_len >= 20` (`16` RPC + `4` segment header) | `data_len`: `0, 16, 19, 20, 21` |
| Max Aligned Segment Payload | `seg_len >= 1 && seg_len <= 1392` (`87 * 16`) | `seg_len`: `0, 1, 1391, 1392, 1393, 1400` |
| Non-Final 16-Byte Alignment | `!more_segments \|\| (seg_len % 16 == 0)` | `(more=true, len=15)` -> Err, `(more=true, len=16)` -> Ok, `(more=true, len=17)` -> Err, `(more=false, len=17)` -> Ok |
| Reassembly Buffer Capacity | `offset.checked_add(seg_len) <= Some(max_msg_size)` | `offset + seg_len`: `max - 1, max, max + 1`, plus `offset = u32::MAX & !0xF` (overflow) |

---

### Pillar 5: Auditable Coverage Justifications (`COV_JUSTIFIED` — ISO 26262-6 Clause 9.4.5)

When defensive code (such as hardware-specific fallbacks, OS descriptor errors after `accept()`, or compiler-generated branch artifacts) cannot be reached by unit/integration tests, **never** strip assertions or annotate functions with `#[coverage(off)]`:

#### Anti-Pattern: Suppressing Coverage Instrumentation or Deleting Defensive Checks
```rust
// Hides the entire function from `.llvmcovmap` so newly added branches are silently unmeasured!
#[coverage(off)]
pub fn clear_nonblocking_on_accepted_socket(fd: i32) -> Result<(), i32> {
    // ...
    Ok(())
}
```

#### Recommended Pattern: In-Code `COV_JUSTIFIED` Markers Linked to `coverage_justifications.yaml`
```rust
pub fn clear_nonblocking_on_accepted_socket(_fd: i32, syscall_rc: i32) -> Result<(), i32> {
    if syscall_rc < 0 {
        // COV_JUSTIFIED_START gateway-clear-nonblock-on-accepted-socket
        return Err(syscall_rc);
        // COV_JUSTIFIED_STOP
    }
    Ok(())
}
```

```yaml
# coverage_justifications.yaml
version: 1
justifications:
  - id: gateway-clear-nonblock-on-accepted-socket
    category: defensive_programming
    platforms: [linux, qnx]
    reason: >
      Defensive programming: POSIX fcntl call can only fail if the socket file descriptor
      is invalid, but the descriptor was just validated by accept() in the preceding step.
```

---

## Step-by-Step MC/DC Closure Workflow (`Validate → Refactor → Synthesize → Justify`)

1. **Static Pre-Flight Scan**: Run `python3 skills/iso26262-mcdc-and-coverage/scripts/analyze_mcdc_coverage.py --scan-source <dir> --asil D --strict` to identify every boolean decision with $>6$ conditions or coupled variables (`x` and `!x`).
2. **Refactor Coupled & Oversized Decisions**: Rewrite coupled boolean expressions into single-occurrence variables or exhaustive `match` expressions (Pillar 2 & Pillar 3) so every condition has a valid Unique-Cause independence pair and $\le 6$ conditions per decision.
3. **Derive $N+1$ Independence-Pair & BVA Unit Tests**: Use the truth-table vectors synthesized by `analyze_mcdc_coverage.py` and the BVA tables in [bva_and_mcdc_refactoring_patterns.md](references/bva_and_mcdc_refactoring_patterns.md) to author `#[test]` cases covering every condition toggle and boundary point (`boundary - 1`, `boundary`, `boundary + 1`).
4. **Collect LLVM Source-Based MC/DC Coverage**: Execute the Nightly `cargo test` + `llvm-profdata merge` + `llvm-cov export --format=text` pipeline in [llvm_mcdc_rust_workflow.md](references/llvm_mcdc_rust_workflow.md).
5. **Audit Raw & Effective Coverage Closure**: Run `python3 skills/iso26262-mcdc-and-coverage/scripts/analyze_mcdc_coverage.py --llvm-json coverage_export.json --justifications coverage_justifications.yaml --source-root . --asil D --strict` to verify `100.00%` effective coverage across Statement, Branch, Function, and MC/DC with **zero** unjustified gaps and **zero** dangling justifications.

## References

- **LLVM MC/DC Instrumentation, `v3.1.0` JSON Schema & Effective Coverage Pipeline**: [llvm_mcdc_rust_workflow.md](references/llvm_mcdc_rust_workflow.md)
- **Truth-Table Independence Pairs, Boolean Refactorings & Wire Framing BVA Suites**: [bva_and_mcdc_refactoring_patterns.md](references/bva_and_mcdc_refactoring_patterns.md)
