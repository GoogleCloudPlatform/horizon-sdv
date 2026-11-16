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

# automotive-software (`v1.2.0`)

A modular suite of **12 general-purpose engineering skills**, **11 zero-dependency Python 3 verification CLIs**, and **1 read-only architecture reviewer subagent** for verifiable C/C++-to-Rust migration, **[Google FOSS Rust Style Guide](https://google.github.io/styleguide/rustguide.html)** architectural hardening (`1.0–10.0` benchmark), statistical performance parity optimization, and Software-Defined Vehicle (SDV) engineering aligned with **ISO 26262:2018** and **ASPICE 4.0**.

> [!IMPORTANT]
> **Non-Certification Disclaimer**: These skills and CLI utilities provide engineering guidance and static/dynamic readiness checks aligned with ISO 26262:2018 and ASPICE 4.0. They are **not** qualified software tools under ISO 26262-8:2018 Clause 11 and do not constitute functional safety certification or legal advice.

---

## Installation & Usage

### Option A: Google Antigravity (`agy`) Plugin Bundle
When working inside a clone of `horizon-sdv`, Antigravity discovers `.agents/plugins/automotive-software` automatically as a workspace plugin. You can also install or validate it globally via `agy`:

```bash
# Install the full 12-skill + 1-agent plugin from GitHub or a local horizon-sdv clone:
agy plugin install https://github.com/GoogleCloudPlatform/horizon-sdv/tree/main/.agents/plugins/automotive-software
agy plugin install .agents/plugins/automotive-software

# Validate, list, enable/disable, or reinstall to refresh:
agy plugin validate .agents/plugins/automotive-software
agy plugin list
agy plugin uninstall automotive-software && agy plugin install .agents/plugins/automotive-software
```

### Option B: Any `SKILL.md`-Compatible Coding Agent (Selective or Full)
Each directory under [`skills/`](skills/) follows the standard progressive-disclosure `SKILL.md` layout (`SKILL.md` + `references/` + stdlib `scripts/`) and works standalone in `.agents/skills/`, `.gemini/skills/`, or `.claude/skills/`:

```bash
# Copy all 12 skills — or cherry-pick Track A C++-to-Rust skills only — into a workspace:
mkdir -p .agents/skills
cp -R skills/* .agents/skills/
```

---

## Skills Included (`12` Skills)

### A. Verifiable C/C++-to-Rust Porting, Benchmarking & Hardening (`6` Skills)

| Skill | Purpose & Domain | Bundled CLI Utility |
|---|---|---|
| **[`verifiable-cpp-to-rust-port-harness`](skills/verifiable-cpp-to-rust-port-harness/)** | Phased port harnesses: pin-only `160000` upstream gitlinks (`update = none`), deterministic input sanitizers, `100%` file provenance & waiver tables, C++ container oracles, single-step verification gates, `let_underscore_must_use` & `checked_*` rules, and deviation/optimization ledgers. | `scripts/verify_port_provenance.py` |
| **[`cpp-to-rust-api-mirror-and-testkit`](skills/cpp-to-rust-api-mirror-and-testkit/)** | Mechanical 1-to-1 C++-to-Rust public API mirrors, compile-only signature locks, `#[repr(transparent)]` wire byte newtypes, scoped mutable slice closures, `pollable_fd()` reactor seams, and parallel-safe in-memory network `testkit` crates. | `scripts/audit_api_mirror.py` |
| **[`cpp-asio-to-rust-migration`](skills/cpp-asio-to-rust-migration/)** | Translates Boost.Asio (`io_context`, timers, strands, `enable_shared_from_this`) and `recursive_mutex` hierarchies into process-wide `poll(2)`/`mio` readiness reactors + `eventfd` wakeups or cancel-safe `tokio` actors, `Arc::new_cyclic`, `OnceLock`, and N-tier lock hierarchies. | `scripts/analyze_asio_concurrency.py` |
| **[`cpp-to-rust-port-review`](skills/cpp-to-rust-port-review/)** | Fresh-context, read-only function-by-function C/C++ → Rust fidelity review: control flow, wire/IPC endianness, translation hazards, `FAITHFUL` vs `WEAKENED` test audits, differential C++ `.bin` goldens, and `ARCHITECTURE.md` doc-vs-code truth checks. | – |
| **[`systems-perf-parity-and-optimization`](skills/systems-perf-parity-and-optimization/)** | Staged container benchmarking (`Layout A` UDS & `Layout B` `veth` UDP/TCP/Multicast), 95% bootstrap CI parity gates (`summary.json` / `*.samples`), and post-parity hot-path optimizations (`Bytes` CoW buffers, `try_write` + `writev`, `SmallVec`, shared timer wheels, `O(1)` socket reactors). | `scripts/evaluate_perf_parity.py` |
| **[`rust-systems-hardening-audit`](skills/rust-systems-hardening-audit/)** | **Post-parity `1.0–10.0` Rust architecture & quality benchmark** grounded in [Google's FOSS Rust Style Guide](https://google.github.io/styleguide/rustguide.html): scores doc-vs-code truth, reactor/thread/dep footprint, lock/ownership topology, `#[must_use] Result` error surface, `checked_*` wire math, binary `unsafe` quarantine, and CoW buffers. | `scripts/audit_rust_architecture.py` |

### B. Automotive Safety & SDV Engineering Aligned with ISO 26262 & ASPICE (`6` Skills)

| Skill | Purpose & Domain | Bundled CLI Utility |
|---|---|---|
| **[`rust-automotive`](skills/rust-automotive/)** | Safety-critical automotive Rust practices aligned with ISO 26262-6 (ASIL-B/D & QM), Ferrocene, MISRA, and [Google Rust Style](https://google.github.io/styleguide/rustguide.html): 19 static checks covering zero panics, bounded memory, monotonic `Instant`, `let_underscore_must_use`, `checked_*` wire math, checked thread spawns, and `pub(crate) mod sys` + binary `unsafe` quarantine. | `scripts/lint_automotive_rust.py` |
| **[`traceability-audit`](skills/traceability-audit/)** | Discovers and audits bidirectional `SWE.1`–`SWE.6` requirements traceability across Sphinx-Needs, TRLC, StrictDoc, Doorstop, ReqIF, Mantra (`#[req(...)]`), GoogleTest `RecordProperty` (`Verifies` / `lobster-tracing`), and Doxygen/rustdoc tags, exporting Markdown and CSV matrices. | `scripts/audit_traceability.py` |
| **[`iso26262-e2e-black-channel`](skills/iso26262-e2e-black-channel/)** | Clean-room ISO 26262-6:2018 Annex D.2.4 E2E black-channel communication protection (`F1`–`F7` fault matrix, `SealedFrame` / `VerifiedPayload` type-states, QM forwarding vs. ASIL quarantine) and public-domain CRCs (SAE-J1850 CRC-8, ITU-T X.25 CRC-16, IEEE 802.3 CRC-32, Castagnoli CRC-32C). | `scripts/verify_e2e_black_channel.py` |
| **[`iso26262-ffi-isolation`](skills/iso26262-ffi-isolation/)** | ISO 26262-6 Clause 7.4.6 & Annex D Freedom from Interference (spatial slab pools, temporal poll budgets, rate limiters) and safe `extern "C"` ABI boundaries (`catch_unwind`, `#[repr(C)]`, pointer/alignment checks). | `scripts/check_ffi_boundaries.py` |
| **[`iso26262-mcdc-and-coverage`](skills/iso26262-mcdc-and-coverage/)** | ISO 26262-6 Clause 9/10 structural coverage (Statement, Branch, MC/DC), `llvm-cov` v3.1.0 JSON analysis, $N+1$ independence-pair synthesis, and `COV_JUSTIFIED` effective coverage auditing (`--strict`). | `scripts/analyze_mcdc_coverage.py` |
| **[`iso26262-seooc-and-sw-fmea`](skills/iso26262-seooc-and-sw-fmea/)** | ISO 26262-10 Clause 9 Software SEooC (`ASR-*`, `AoU-*`) and ISO 26262-6 Clause 7 inductive SW-FMEA (`FM-*`) closed-loop mitigation validation across TRLC, YAML, Markdown, and rustdoc. | `scripts/validate_seooc_fmea.py` |

---

## Bundled Read-Only Reviewer Subagent (`agents/`)

| Agent | Role |
|---|---|
| **[`rust-hardening-reviewer`](agents/rust-hardening-reviewer.md)** | Fresh-context, read-only `1.0–10.0` Google FOSS Rust Style & embedded middleware architecture reviewer (`audit_rust_architecture.py` + dependency-ordered work-package handover generator). |

---

## Static Analysis Scope & Validation Provenance

- **Zero-Dependency Lexical/Regex Gates**: All 11 Python 3 CLIs rely strictly on the Python standard library (`0` external pip packages) so they run hermetically inside air-gapped CI containers. Source-scanning CLIs perform comment-stripped lexical and structural checks designed to **complement**—not replace—compiler and semantic verifiers (`rustc`, `cargo clippy`, `cargo miri`, `cargo kani`, and `llvm-cov`).
- **Validation Maturity**:
  - **Multi-Milestone Production Porting Validated**: All 6 Track A skills plus `rust-automotive`, `traceability-audit`, `iso26262-e2e-black-channel`, and `iso26262-ffi-isolation` were exercised and refined across an `83k`-LOC C++17-to-Rust multi-daemon networking/IPC migration (`1,000` tests, `211` container benchmark scenarios, `5.8 → 9.5/10` architectural hardening).
  - **Reference & Crate-Fixture Validated**: `iso26262-mcdc-and-coverage` and `iso26262-seooc-and-sw-fmea` (plus secondary ReqIF/Doorstop/StrictDoc/Mantra parsers in `traceability-audit`) are validated against `llvm-cov` v3.1.0 JSON exports, TRLC/YAML corpora, Rust crate fixtures, and built-in `--self-test` suites.

---

## Programmatic CLI Self-Tests (`0` External Dependencies)

```bash
for script in skills/*/scripts/*.py; do
  python3 "$script" --self-test
done
```

---

## Contributing

See the Horizon SDV [Contribution Guide](../../../docs/contributing.md) for contribution guidelines and CLA requirements.

## License

Licensed under the [Apache License, Version 2.0](../../../LICENSE).
