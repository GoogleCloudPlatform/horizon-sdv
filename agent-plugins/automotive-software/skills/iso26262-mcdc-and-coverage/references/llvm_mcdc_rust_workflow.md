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

# LLVM Source-Based MC/DC Instrumentation, `v3.1.0` JSON Schema & Effective Coverage Workflow

This reference provides ISO 26262-6:2018 Clauses 9 & 10 structural coverage tables, Nightly Rust and Clang/LLVM MC/DC compiler mechanics, the `llvm.coverage.json.export` (`v3.1.0`) positional array schemas, and the auditable Raw-vs.-Effective Coverage Justification (`COV_JUSTIFIED`) pipeline.

---

## 1. ISO 26262-6:2018 Clauses 9 & 10 Verification & Coverage Tables

Notation: `++` = Highly Recommended (expected unless a documented rationale justifies an alternative); `+` = Recommended; `o` = No recommendation for or against.

### 1.1 Clause 9.4.3 — Table 7: Methods for Software Unit Verification

| Method ID | Verification Method | ASIL A | ASIL B | ASIL C | ASIL D | Rust / LLVM Tooling Implementation |
|---|---|---:|---:|---:|---:|---|
| **1a** | Walk-through | `++` | `+` | `o` | `o` | Structured code review (ASIL A only) |
| **1b** | Pair-programming | `+` | `+` | `+` | `+` | Dual-author development for complex state machines |
| **1c** | Inspection | `+` | `++` | `++` | `++` | Formal checklist inspection against ISO 26262-6 Table 1/6/9 |
| **1d** | Semi-formal verification | `+` | `+` | `++` | `++` | Property-based testing (`proptest`) & state-machine models |
| **1e** | Formal verification | `o` | `o` | `+` | `+` | Bounded model checking (`cargo kani`) for wire parsers |
| **1f** | Control flow analysis | `+` | `+` | `++` | `++` | CFG extraction & unreachable block detection (`rustc` MIR / LLVM) |
| **1g** | Data flow analysis | `+` | `+` | `++` | `++` | Borrow checker + `cargo +nightly miri test` |
| **1h** | Static code analysis | `++` | `++` | `++` | `++` | `cargo clippy` hardened lint suite (`rust-automotive`) |
| **1i** | Static analyses based on abstract interpretation | `+` | `+` | `+` | `+` | Abstract domain bounds / Kani symbolic execution |
| **1j** | Requirements-based test | `++` | `++` | `++` | `++` | `#[test]` functions linked to software safety requirements (`traceability-audit`) |
| **1k** | Interface test | `++` | `++` | `++` | `++` | Public trait/API contract tests across valid/invalid inputs |
| **1l** | Fault injection test | `+` | `+` | `+` | `++` | Bit-flip, truncation, sequence-gap, and OOM fault injection |
| **1m** | Resource usage evaluation | `+` | `+` | `+` | `++` | Bounded slab/pool high-water-mark and stack-frame measurement |
| **1n** | Back-to-back comparison test between model and code | `+` | `+` | `++` | `++` | Differential testing between reference model/baseline and Rust implementation |

### 1.2 Clause 9.4.4 — Table 8: Methods for Deriving Test Cases for Software Unit Testing

| Method ID | Test Case Derivation Method | ASIL A | ASIL B | ASIL C | ASIL D | Concrete Engineering Rule |
|---|---|---:|---:|---:|---:|---|
| **1a** | Analysis of requirements | `++` | `++` | `++` | `++` | Every `SWE-*` / `ASR-*` requirement maps to >= 1 positive and >= 1 negative test |
| **1b** | Generation and analysis of equivalence classes | `+` | `++` | `++` | `++` | Partition inputs into valid classes and invalid classes (e.g., Message Type enum, Return Code enum, segmentation flag bit) |
| **1c** | Analysis of boundary values | `+` | `++` | `++` | `++` | Test `min - 1`, `min`, `min + 1`, `max - 1`, `max`, `max + 1`, `0`, and `T::MAX` for every numeric/length guard |
| **1d** | Error guessing based on knowledge or experience | `+` | `+` | `+` | `+` | Target historical bug patterns (0-length underflow, wrap-around offsets, coupled reboot flags) |

### 1.3 Clause 9.4.5 — Table 9: Structural Coverage Metrics at the Software Unit Level

| Method ID | Structural Coverage Metric | ASIL A | ASIL B | ASIL C | ASIL D | `llvm-cov export` JSON Field | Target Threshold |
|---|---|---:|---:|---:|---:|---|---|
| **1a** | Statement coverage | `++` | `++` | `+` | `+` | `summary.lines` / `segments` | `100.00%` Effective (Raw + `COV_JUSTIFIED`) |
| **1b** | Branch coverage | `+` | `++` | `++` | `++` | `summary.branches` / `branches` (9-tuple) | `100.00%` Effective (both `True` and `False` counts > 0) |
| **1c** | MC/DC (Modified Condition/Decision Coverage) | `+` | `+` | `+` | `++` | `summary.mcdc` / `mcdc_records` (11-tuple) | `100.00%` Effective (every `ConditionsArray[c] == true`) |

### 1.4 Clause 10.4.5 — Table 12: Structural Coverage at the Software Architectural Level

| Method ID | Architectural Coverage Metric | ASIL A | ASIL B | ASIL C | ASIL D | Verification Method |
|---|---|---:|---:|---:|---:|---|
| **1a** | Function coverage | `+` | `+` | `++` | `++` | `summary.functions` (`count == covered`, 100% of non-justified functions executed) |
| **1b** | Call coverage | `+` | `+` | `++` | `++` | Every inter-unit function call site and trait dispatch site executed at least once during integration tests |

---

## 2. Rust & LLVM Source-Based Coverage Architecture

### 2.1 How `rustc` Emits `.llvmcovmap` and MC/DC Records

1. **`-C instrument-coverage`**: Enables LLVM source-based coverage mapping (`__llvm_covmap` and `__llvm_covfun` ELF sections) and counter instrumentation (`__llvm_prf_cnts`, `__llvm_prf_data`, `__llvm_prf_names`).
2. **`-Z coverage-options=<level>`**:
   - `block` (default): Emits `MappingKind::Code(CovTerm)` and `MappingKind::Gap(CovTerm)` at the MIR basic-block level (Statement/Region coverage only).
   - `branch`: Emits `MappingKind::Branch { r#true: CovTerm, r#false: CovTerm }` for `if`, `match`, `while`, and `&&`/`||` branch edges.
   - `condition` / `mcdc`: Emits `MappingKind::MCDCDecision { bitmap_idx: u32, conditions_num: u32 }` for multi-condition boolean decisions and `MappingKind::MCDCBranch` for each atomic condition node in the binary decision diagram (BDD), plus global MC/DC test-vector bitmaps (`__llvm_prf_bits`).
3. **LLVM Condition Bitmap Cap (`--mcdc-max-conditions`)**:
   - Each `MCDCDecision` reserves a test-vector bitmap in `__llvm_prf_bits` to record which truth-table test vectors executed at runtime.
   - LLVM 18 (`rustc` ~1.78–1.81) defaulted to `--mcdc-max-conditions=6` ($2^6 = 64$ bits = 8 bytes per decision) and skipped MC/DC instrumentation on larger decisions unless `-C llvm-args=--mcdc-max-conditions=10` (Rust) or `-mllvm --mcdc-max-conditions=10` (Clang C++) was passed. LLVM 19+ (`rustc 1.82+`) raised the internal bitmap limit significantly.
   - **Best Practice**: Refactor decisions so $N \le 6$ (ideally $N \le 4$) for ISO 26262-6 Table 1 (`1a` low complexity), LLVM 18 toolchain compatibility, and tractable $N+1$ test synthesis. Only raise `--mcdc-max-conditions` temporarily when measuring legacy code prior to refactoring.

### 2.2 Complete Command-Line Workflow (Cargo & Bazel)

#### Cargo Nightly Workflow (Rust)

```bash
# 1. Clean stale artifacts and set deterministic MC/DC instrumentation flags
cargo clean
mkdir -p target/cov

export RUSTFLAGS="\
  -C instrument-coverage \
  -Z coverage-options=mcdc \
  -C opt-level=0 \
  -C debuginfo=2 \
  -C codegen-units=1 \
  -C link-dead-code \
  -C llvm-args=--mcdc-max-conditions=10 \
  -C llvm-args=-runtime-counter-relocation"

# Continuous mode flushes counters immediately via mmap so abnormal terminations
# or death tests still record coverage:
export LLVM_PROFILE_FILE="target/cov/%p-%m.profraw"
export LLVM_PROFILE_CONTINUOUS_MODE=1

# 2. Build and run unit & integration tests
cargo +nightly test

# 3. Merge raw profiles (including bitmap sections __llvm_prf_bits)
llvm-profdata merge -sparse target/cov/*.profraw -o target/cov/merged.profdata

# 4. Generate annotated HTML report with MC/DC and Branch views
TEST_BINS=$(find target/debug/deps -maxdepth 1 -executable -type f | sed 's/^/--object /' | sed '1s/--object //')
llvm-cov show \
  --format=html \
  --instr-profile=target/cov/merged.profdata \
  --show-branches=count \
  --show-mcdc \
  --show-mcdc-summary \
  --show-expansions \
  --coverage-watermark=100,80 \
  --ignore-filename-regex='(/.cargo/|/rustc/|/tests/)' \
  --output-dir=target/cov/html \
  $TEST_BINS

# 5. Export machine-readable v3.1.0 JSON for automated gatekeeping
llvm-cov export \
  --format=text \
  --instr-profile=target/cov/merged.profdata \
  --ignore-filename-regex='(/.cargo/|/rustc/|/tests/)' \
  $TEST_BINS > target/cov/coverage_export.json
```

#### Clang / Bazel Workflow (C++17 Baseline)

```ini
# coverage.bazelrc
coverage --instrumentation_filter="^//crates[/:],^//src[/:]"
coverage --cxxopt=-O0
coverage --cxxopt=-fcoverage-mapping
coverage --cxxopt=-fprofile-instr-generate
coverage --cxxopt=-fcoverage-mcdc
coverage --cxxopt=-mllvm
coverage --cxxopt=--mcdc-max-conditions=10
coverage --cxxopt=-mllvm
coverage --cxxopt=-runtime-counter-relocation
coverage --dynamic_mode=off
coverage --experimental_use_llvm_covmap
coverage --test_env=LLVM_PROFILE_CONTINUOUS_MODE=1
```

### 2.3 Critical Gotcha: Rust `.rlib` Archives in Baseline Coverage

When computing zero-coverage baselines for files not linked into any test binary (so completely untested crates cannot hide from the denominator), `rustc` packages libraries as `.rlib` (or `.a` symlinks in Bazel `rules_rust`).
- **Failure Mode**: Every Rust `.rlib` is a standard Unix `!<arch>\n` archive whose first member is `lib.rmeta`. When `llvm-cov` inspects an archive, if the first member is not an ELF object file, `llvm-cov` aborts with `"no coverage data found"`, ignoring the `.rcgu.o` members that contain `.llvmcovmap`.
- **Fix**: Extract `.o` members from any `.rlib`/`.a` containing `lib.rmeta` before passing `--object` to `llvm-cov`:

```python
# Extract .o members from Rust .rlib/.a archives (skipping lib.rmeta)
for index, (name, offset, size) in enumerate(ar_members):
    if name.endswith(".o"):
        f.seek(offset)
        (workdir / f"{stem}.{index}.o").write_bytes(f.read(size))
```

---

## 3. Exact `llvm.coverage.json.export` Schema (`v3.1.0`)

### 3.1 Top-Level Envelope

```json
{
  "version": "3.1.0",
  "type": "llvm.coverage.json.export",
  "data": [
    {
      "files": [
        {
          "filename": "src/discovery.rs",
          "branches": [ ... ],
          "expansions": [ ... ],
          "mcdc_records": [ ... ],
          "segments": [ ... ],
          "summary": { ... }
        }
      ],
      "functions": [
        {
          "name": "ecu_router::detect_peer_reboot",
          "count": 14,
          "filenames": ["src/discovery.rs"],
          "branches": [ ... ],
          "mcdc_records": [ ... ],
          "regions": [ ... ]
        }
      ],
      "totals": { ... }
    }
  ]
}
```

### 3.2 Positional Tuple Schemas inside `files[*]` and `functions[*]`

To minimize JSON payload size, `llvm-cov export` encodes `segments`, `regions`, `branches`, and `mcdc_records` as **positional arrays** rather than named objects:

#### A. `segments` — 6-Element Tuple
```
[Line, Col, Count, HasCount, IsRegionEntry, IsGapRegion]
```
| Index | Type | Description |
|---:|---|---|
| `0` | `int` | 1-based source line number (`Segment.Line`) |
| `1` | `int` | 1-based source column number (`Segment.Col`) |
| `2` | `int64` | Execution count clamped to `int64_t` (`Segment.Count`) |
| `3` | `bool` | `true` if segment has an execution count (`Segment.HasCount`) |
| `4` | `bool` | `true` if segment starts a new counter region (`Segment.IsRegionEntry`) |
| `5` | `bool` | `true` if segment is a gap region between statements (`Segment.IsGapRegion`) |

#### B. `regions` — 8-Element Tuple
```
[LineStart, ColumnStart, LineEnd, ColumnEnd, ExecutionCount, FileID, ExpandedFileID, Kind]
```

#### C. `branches` — 9-Element Tuple
```
[LineStart, ColumnStart, LineEnd, ColumnEnd, ExecutionCount, FalseExecutionCount, FileID, ExpandedFileID, Kind]
```
| Index | Type | Description |
|---:|---|---|
| `0` | `int` | `Region.LineStart` (1-based) |
| `1` | `int` | `Region.ColumnStart` (1-based) |
| `2` | `int` | `Region.LineEnd` (1-based) |
| `3` | `int` | `Region.ColumnEnd` (1-based) |
| `4` | `int64` | **`ExecutionCount` (True branch execution count)** |
| `5` | `int64` | **`FalseExecutionCount` (False branch execution count)** |
| `6` | `int` | `Region.FileID` |
| `7` | `int` | `Region.ExpandedFileID` |
| `8` | `int` | `Region.Kind` (`4` = `BranchRegion`, `6` = `MCDCBranchRegion`) |

- **Branch Coverage Rule**: Each `branches` entry contributes 2 outcomes (`True` and `False`). A branch is 100% covered iff `branch[4] > 0` AND `branch[5] > 0`.

#### D. `mcdc_records` — 11-Element Tuple
```
[LineStart, ColumnStart, LineEnd, ColumnEnd, TrueDecisions, FalseDecisions, FileID, ExpandedFileID, Kind, ConditionsArray, TestVectorsArray]
```
| Index | Type | Description |
|---:|---|---|
| `0` | `int` | Decision start line (1-based) |
| `1` | `int` | Decision start column (1-based) |
| `2` | `int` | Decision end line (1-based) |
| `3` | `int` | Decision end column (1-based) |
| `4` | `int64` | Count of distinct executed test vectors evaluating to `true` |
| `5` | `int64` | Count of distinct executed test vectors evaluating to `false` |
| `6` | `int` | File ID |
| `7` | `int` | Expanded macro file ID |
| `8` | `int` | `5` (`MCDCDecisionRegion`) |
| `9` | `bool[]` | **`ConditionsArray`**: `[bool, ...]` of length `NumConditions`, where element `c` is `true` iff condition `c` has a covered independence pair |
| `10` | `object[]` | **`TestVectorsArray`**: list of executed (and optionally unexecuted) truth-table test vectors |

#### E. `TestVectorsArray` Element Schema
```json
{
  "conditions": [true, false, null],
  "executed": true,
  "result": false
}
```
- `"conditions"`: Array of length `NumConditions`:
  - `MCDC_True` $\rightarrow$ `true`
  - `MCDC_False` $\rightarrow$ `false`
  - `MCDC_DontCare` $\rightarrow$ `null` (condition was short-circuited and not evaluated on this path)
- `"executed"`: `true` if this truth-table vector was observed in `.profdata`; `false` if emitted via `--show-mcdc-non-executed-vectors`.
- `"result"`: Decision outcome (`true` or `false`).

### 3.3 `summary` Object Schema

```json
"summary": {
  "lines":          { "count": 120, "covered": 118, "percent": 98.33333333333333 },
  "functions":      { "count": 15,  "covered": 15,  "percent": 100.0 },
  "instantiations": { "count": 18,  "covered": 18,  "percent": 100.0 },
  "regions":        { "count": 84,  "covered": 82,  "notcovered": 2, "percent": 97.61904761904762 },
  "branches":       { "count": 40,  "covered": 39,  "notcovered": 1, "percent": 97.5 },
  "mcdc":           { "count": 14,  "covered": 12,  "notcovered": 2, "percent": 85.71428571428571 }
}
```

---

## 4. ISO 26262-6 Clause 9.4.5 Coverage Justification Infrastructure (`COV_JUSTIFIED`)

ISO 26262-6:2018 Clause 9.4.5 requires achieving a high level of structural coverage and providing a **documented rationale** for any uncovered statements, branches, or MC/DC conditions (e.g., defensive programming checks that are unreachable unless hardware/kernel invariants fail).

### 4.1 Justification Database Schema (`coverage_justifications.yaml`)

```yaml
version: 1

justifications:
  - id: socket-clear-nonblock-defensive
    category: defensive_programming
    platforms: [linux, qnx]
    reason: >
      Defensive programming: POSIX fcntl(F_SETFL) can only fail with EBADF if the socket
      descriptor is invalid, which is impossible immediately after a successful accept().
    locations:
      - file: src/endpoints/tcp_server.rs
        line_start: 88
        line_end: 90

  - id: reassembler-exhaustive-enum-guard
    category: defensive_programming
    platforms: [linux, qnx]
    reason: >
      Unreachable defensive fallback arm on validated MessageType bitmask; retained per
      ISO 26262-6 Table 1 Method 1b (defensive implementation techniques).
```

### 4.2 In-Code Marker Syntax

Prefer in-code markers over hardcoded line numbers in YAML so line shifts during refactoring never desynchronize justifications:

1. **Single-Line Marker**: Justifies the statement, branch, and MC/DC conditions on the same line:
   ```rust
   return Err(EndpointError::InvalidDescriptor); // COV_JUSTIFIED socket-clear-nonblock-defensive
   ```
2. **Multi-Line Region Marker**: Justifies all lines between `START` and `STOP` (`start_line + 1 .. stop_line - 1`):
   ```rust
   // COV_JUSTIFIED_START reassembler-exhaustive-enum-guard
   log::error!("Defensive guard: unexpected message type bits {:#04x}", raw_type);
   return Err(TpError::MalformedHeader);
   // COV_JUSTIFIED_STOP
   ```

### 4.3 Raw vs. Effective Coverage & Stale Justification Rules

1. **Raw Coverage**:
   $$\text{Raw Coverage \%} = \left\lfloor \frac{\text{Covered}}{\text{Total}} \times 10000 \right\rfloor / 100$$
2. **Effective Coverage**:
   $$\text{Effective Coverage \%} = \left\lfloor \frac{\text{Covered} + \text{Justified Uncovered}}{\text{Total}} \times 10000 \right\rfloor / 100$$
3. **Stale Justification Detection**:
   A justification on line $L$ is **stale** if and only if:
   - Line $L$ is covered by tests (`Count > 0`), **AND**
   - Every branch at line $L$ has both `True` and `False` execution counts $> 0$, **AND**
   - Every MC/DC condition at line $L$ has `ConditionsArray[c] == true`.
   If line $L$ is executed (`Count > 0`) but still has an uncovered branch direction or uncovered MC/DC condition (e.g., an inline defensive check or `?` operator), the justification is **active** (not stale) and justifies the uncovered branch/condition.
4. **Validation Gates**:
   - Every `id` must match `^[a-z0-9]+(-[a-z0-9]+)*$` (unique `kebab-case`).
   - `category` must be one of `defensive_programming`, `tool_false_positive`, `platform_specific`, or `other`.
   - Every in-code `COV_JUSTIFIED <id>` must exist in `coverage_justifications.yaml` (dangling IDs fail CI).
   - Stale justifications must be reported as warnings and removed so `coverage_justifications.yaml` never masks future regressions.
