---
name: traceability-audit
description: >-
  Discovers, documents, and audits bidirectional requirements traceability
  systems across automotive C, C++, Python, and Rust repositories aligned with
  ASPICE 4.0 (SWE.1 through SWE.6) and ISO 26262-6. Detects Sphinx-Needs, TRLC,
  StrictDoc, Doorstop, ReqIF, Mantra (#[req(...)]), GoogleTest RecordProperty
  (Verifies / lobster-tracing), and Doxygen / rustdoc tags, generating Markdown
  and CSV traceability matrices. Use when: discovering how a repository tracks
  requirements, linking system or software requirements to Rust or C++ code and
  tests, or auditing SWE.1-SWE.6 coverage. Don't use for: LLVM MC/DC structural
  coverage (use iso26262-mcdc-and-coverage) or SEooC/SW-FMEA authoring (use
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

# Automotive Requirements Traceability Discovery & Bidirectional Audit (`traceability-audit`)

## Quick Start

| Goal | Command |
|---|---|
| **Run built-in multi-framework self-test** | `python3 skills/traceability-audit/scripts/audit_traceability.py --self-test` |
| **Enforce CI traceability integrity gate (`--strict`)** | `python3 skills/traceability-audit/scripts/audit_traceability.py --repo /path/to/repo --spec-dir /path/to/external-spec --strict` |
| **Export Markdown report & spreadsheet CSV matrix** | `python3 skills/traceability-audit/scripts/audit_traceability.py --repo /path/to/repo --markdown-out /tmp/trace.md --csv-out /tmp/trace.csv` |

Automotive safety and quality standards—**Automotive SPICE (ASPICE) 4.0** (`SWE.1` through `SWE.6`) and **ISO 26262-6:2018** (Clauses 6.4.5, 7.4.15, 8.4.5, 9.4.2, and 10.4.2)—require **bidirectional traceability** across software engineering layers:

1. **Forward Traceability**: Every system/protocol specification item (`SYS.2`) traces forward to a Software Requirement (`SWE.1`), Software Architectural Element (`SWE.2`), Detailed Design / Source Implementation (`SWE.3`), and Verification Test (`SWE.4` Unit, `SWE.5` Integration, `SWE.6` Qualification).
2. **Backward Traceability**: Every implementation function (`SWE.3`) and verification test case (`SWE.4`–`SWE.6`) traces backward to a valid, existing requirement ID (preventing unintended functionality, dead requirements, or dangling citations after spec updates).

---

## Supported Frameworks & Mechanisms (`8` Parsers)

`scripts/audit_traceability.py` parses eight open-source specification and code-tracing formats directly from source files:

1. **Sphinx-Needs** (`conf.py` + `*.rst` `.. requirement::` / `.. feat_req::` / `.. spec_req::` with `:id:` and `:satisfies:`, stripping `.. code-block::` examples)
2. **TRLC** (`*.rsl` schemas + `*.trlc` packages, `derived_from = [...]`, `mitigates = "..."`, and issue-comment aliases)
3. **StrictDoc** (`strictdoc.toml` + `*.sdoc` `[REQUIREMENT]` / `UID:` / `VALUE:` blocks + `@sdoc[...]` source markers)
4. **Doorstop** (`.doorstop.yml` + per-item `*.yml` / `*.yaml` `active:`, `normative:`, and `links:`)
5. **ReqIF** (`*.reqif` XML `<SPEC-OBJECT>` `LONG-NAME` / `IDENTIFIER` extraction)
6. **Mantra** (Rust `#[req(...)]` / `#[requirements(...)]` attribute macros)
7. **GoogleTest Property Annotations (including LOBSTER C++ tags)** (`RecordProperty("Verifies", ...)` and `RecordProperty("lobster-tracing", ...)` in C++ tests; note: scans C++ source annotations rather than `.lobster` JSON outputs)
8. **Doxygen & Rustdoc Structured Tags** (`@implements`, `@satisfies`, `@req`, `@test_case`, `@tests` across C, C++, Rust, and Python)

---

## Bundled Audit Script (`scripts/audit_traceability.py`)

```bash
python3 skills/traceability-audit/scripts/audit_traceability.py \
  --repo /path/to/target-repo \
  --spec-dir /path/to/external-spec \
  --markdown-out /tmp/traceability_report.md \
  --csv-out /tmp/traceability_matrix.csv
```

### Supported CLI Flags
- `--repo <path>` (or positional `repo`): Target repository or directory to audit (default: `.`).
- `--spec-dir <path>`: Optional external specification directory (`*.rst`) to cross-check upstream `:satisfies:` / `@satisfies` links.
- `--markdown-out <path>`: Write the Markdown traceability audit report to `<path>` (prints to stdout if omitted).
- `--csv-out <path>`: Write the spreadsheet-compatible CSV traceability matrix to `<path>`.
- `--self-test`: Run built-in verification against a synthetic multi-file Rust/RST fixture.
- `--strict`: Exit `1` if duplicate requirement IDs or dangling spec/code/test references are found. Orphan (unimplemented/untested) requirements are reported without failing the integrity check so incremental coverage gates can be enforced separately.

---

## Non-Obvious Parsing & Auditing Rules

1. **Separate Normative Requirements from Headings and Informational Nodes**:
   - Never penalize a repository for lacking unit tests on `.. heading::`, `:reqtype: Information`, or `normative: false` items; compute coverage percentages strictly against normative requirements.
2. **Strip Code-Block Examples Before Extracting RST Requirements**:
   - Documentation files frequently embed example directives inside `.. code-block:: rst` blocks. Strip indented code blocks (preserving line numbers) before harvesting `:id:` options.
3. **Detect Duplicate Requirement Definitions Across Specification Files**:
   - Record all `(file, line)` declaration sites per canonical ID and flag `len(sites) > 1` as a **Duplicate Requirement Definition** defect rather than silently overwriting in a `set`.
4. **Distinguish Rust `#[cfg(test)]` Blocks from Production `SWE.3` Code**:
   - In single-file Rust modules (`src/*.rs`), track the byte offset of `#[cfg(test)]` and `#[test]` / `#[kani::proof]` attributes so unit tests at the bottom of `src/*.rs` are classified as `SWE.4` test citations rather than `SWE.3` implementation citations.
5. **Prevent Catastrophic Regex Backtracking on Multi-Line C++ Signatures**:
   - Use bounded lookahead windows (`500` chars) and linear-time function/symbol matchers when resolving the C++ or Rust item following a doc-comment block.

---

## Side-by-Side Patterns (`Anti-Pattern` vs. `Recommended Pattern`)

### 1. Implementation Traceability (`SWE.3` → `SWE.1` / `SYS.2`)

```cpp
// Anti-Pattern: Unstructured prose comment that automated tools cannot extract
void on_frame(const uint8_t* data, size_t len) {
    // frame must be at least 16 bytes per protocol spec section 4.1
    if (len < 16) return;
}
```

```rust
// Recommended Pattern: Machine-parseable rustdoc @implements & @satisfies tags
/// Parses and validates a 16-byte wire header from an untrusted datagram.
///
/// @implements REQ_MSG_001, REQ_MSG_002
/// @satisfies SYS_PROTO_045, SYS_PROTO_060
pub fn parse_header(frame: &[u8]) -> Result<WireHeader, HeaderError> {
    if frame.len() < WIRE_HEADER_BYTES {
        return Err(HeaderError::FrameTooShort { actual: frame.len() });
    }
    // ...
}
```

### 2. Verification Test Traceability (`SWE.4`–`SWE.6` → `SWE.1` / `SYS.2`)

```rust
#[cfg(test)]
mod tests {
    use super::*;

    /// Verifies that truncated wire headers (< 16 bytes) are rejected deterministically.
    ///
    /// @test_case TC_HDR_001
    /// @tests REQ_MSG_001, SYS_PROTO_045
    #[test]
    fn rejects_truncated_header() {
        assert!(parse_header(&[0u8; 15]).is_err());
    }
}
```

---

## Reference Documentation

- **[traceability_systems_and_aspice_mapping.md](references/traceability_systems_and_aspice_mapping.md)**: Detection signatures, file schemas, and extraction rules for Sphinx-Needs, TRLC, GoogleTest `RecordProperty` (`Verifies` / `lobster-tracing`), StrictDoc, Doorstop, Mantra, ReqIF, Doxygen, and Rust doc-attributes, mapped to ASPICE 4.0 `SWE.1`–`SWE.6` and ISO 26262-6 Clauses 6–10.
