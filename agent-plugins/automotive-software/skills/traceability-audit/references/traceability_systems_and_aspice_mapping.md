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

# Traceability Systems & ASPICE 4.0 / ISO 26262-6 Mapping Reference

Detection signatures, file schemas, extraction patterns, and V-model mappings for the eight requirements traceability mechanisms supported across automotive C, C++, Python, and Rust repositories.

---

## 1. Automotive V-Model & ASPICE 4.0 / ISO 26262-6 Bidirectional Mapping

In **Automotive SPICE (ASPICE) 4.0** and **ISO 26262-6:2018**, requirements traceability spans six software engineering process groups (`SWE.1`–`SWE.6`) plus upstream system/protocol specifications (`SYS.2`):

| V-Model Layer | ASPICE 4.0 Process | ISO 26262:2018 Clause | Typical Repository Artifacts | Required Bidirectional Links |
|---|---|---|---|---|
| **System / Protocol Specification** | `SYS.2` System Requirements Analysis | Part 4 Clause 6 (Technical Safety Requirements) | External `.rst` specs (`.. feat_req::` / `.. spec_req::`), `assumed_system_requirements.trlc` (`AssumedSystemReq ASR_*`), `.reqif` | Satisfied by `SWE.1` requirements (`:satisfies:`, `derived_from`) or directly by `SWE.3` (`@satisfies`) and `SWE.6` (`@tests`) |
| **Software Requirements Analysis** | `SWE.1` Software Requirements Analysis | Part 6 Clause 6 (Software Safety Requirements) | `docs/requirements/**/*.rst` (`.. requirement::` `:id: REQ_*`), `*.trlc` (`FeatReq`, `CompReq`), `*.sdoc`, `.doorstop.yml` | **Upstream**: `:satisfies:` or `derived_from`. **Downstream**: `@implements REQ_*` (`SWE.3`) and `@tests REQ_*` (`SWE.4`–`SWE.6`) |
| **Software Architectural Design** | `SWE.2` Software Architectural Design | Part 6 Clause 7 (Software Architectural Design) | `docs/requirements/architecture.rst` (`REQ_ARCH_*`), `*.trlc` (`AoU`, `FailureMode`) | **Upstream**: `SWE.1` requirements. **Downstream**: Crate/module boundaries and integration tests (`SWE.5`) |
| **Software Detailed Design & Unit Construction** | `SWE.3` Software Detailed Design and Unit Construction | Part 6 Clause 8 (Software Unit Design and Implementation) | C++/Rust functions, structs, and traits annotated with `@implements`, `@satisfies`, `@req`, `#[req(...)]`, or `@sdoc[...]` | **Upstream**: Cites `SWE.1` (`REQ_*`) or `SYS.2` IDs. **Downstream**: Exercised by `SWE.4` unit tests |
| **Software Unit Verification** | `SWE.4` Software Unit Verification | Part 6 Clause 9 (Software Unit Verification) | GTest `TEST()`, Rust `#[test]` / `#[kani::proof]` with `@test_case`, `@tests`, `#[req(...)]`, or `RecordProperty("Verifies", ...)` | **Upstream**: Cites the `SWE.3` unit / `SWE.1` requirement it verifies |
| **Software Component Integration & Qualification** | `SWE.5` / `SWE.6` Verification | Part 6 Clauses 10 & 11 | Multi-process integration and conformance suites (`tests/**`) | **Upstream**: Links to `SWE.2` architectural interfaces, `SWE.1` requirements, and `SYS.2` protocol specs |

### Bidirectional Completeness Sets

Let $S$ be normative specification IDs, $I$ be implementation-cited IDs (`SWE.3`), and $T$ be test-cited IDs (`SWE.4`–`SWE.6`):
1. **Fully Traced**: $S \cap I \cap T$
2. **Implemented Only (Untested)**: $(S \cap I) \setminus T$
3. **Tested Only (Unannotated in Code)**: $(S \cap T) \setminus I$
4. **Orphan (Unimplemented & Untested)**: $S \setminus (I \cup T)$
5. **Dangling References**: $(I \setminus S) \cup (T \setminus S)$

---

## 2. Schemas & Extraction Rules for Supported Systems

### 2.1 Sphinx-Needs (`conf.py` + `*.rst`)
- **Signatures**: `"sphinx_needs"` in `conf.py`; `.. requirement::`, `.. feat_req::`, `.. spec_req::`, `.. heading::`, or `.. information::` blocks with `:id:` and optional `:reqtype:` / `:satisfies:`.
- **Rules**:
  - Replace indented `.. code-block::` / `.. code::` blocks with newline-preserving whitespace before regex extraction so documentation examples do not pollute $S$.
  - Classify `.. heading::` and `:reqtype: Information` as `is_normative = False`.

### 2.2 TRLC (`*.rsl`, `*.trlc`) & GoogleTest `RecordProperty` (`Verifies` / `lobster-tracing`)
- **Signatures**: `package <Pkg>` + typed blocks (`<Schema>.<Type> <Name> { ... }`) in `*.trlc`; C++ GTest calls `RecordProperty("Verifies", "<ID>")` or `RecordProperty("lobster-tracing", "<Pkg>.<ID>")`.
- **Rules**:
  - Register both unqualified (`<Name>`) and package-qualified (`<Pkg>.<Name>`) aliases (plus optional `/* broken_link_c/issue/<NUM> */` $\to$ `SCR-<NUM>` aliases).
  - Extract upstream links from `derived_from = [...]` (stripping `@<version>` suffixes) and `mitigates = "..."`.

### 2.3 StrictDoc (`strictdoc.toml`, `*.sdoc`, `@sdoc[...]`)
- **Signatures**: `[REQUIREMENT]` / `[COMPOSITE_REQUIREMENT]` / `[SECTION]` blocks with `UID:`, `TITLE:`, and `VALUE:` parent relations; `@sdoc[UID]` range markers in source/test files (`[SECTION]` is non-normative).

### 2.4 Doorstop (`.doorstop.yml` + Per-Item YAML)
- **Signatures**: `.doorstop.yml` alongside `*.yml`/`*.yaml` item files containing `active:` and `text:`. Items with `normative: false` are non-normative headings; `links:` lists upstream IDs.

### 2.5 Mantra (`#[req(...)]` / `#[requirements(...)]`)
- **Signatures**: Rust attribute macros `#[req(REQ_ID)]` or `#[requirements(REQ_A, REQ_B)]` on production items (`SWE.3`) or `#[test]` functions (`SWE.4`).

### 2.6 ReqIF (`*.reqif`)
- **Signatures**: XML `<REQ-IF>` documents containing `<SPEC-OBJECT>` elements (`LONG-NAME` or `IDENTIFIER` attributes).

### 2.7 Doxygen & Rustdoc Tags (`@implements`, `@satisfies`, `@req`, `@test_case`, `@tests`)
- Configure custom aliases in `Doxyfile` so Doxygen does not warn on traceability commands:
  ```ini
  ALIASES  = "implements=\xrefitem implements \"Implements Requirement\" \"Implemented Requirements\" "
  ALIASES += "satisfies=\xrefitem satisfies \"Satisfies Specification\" \"Satisfied Specifications\" "
  ALIASES += "test_case=\xrefitem test_case \"Test Case ID\" \"Test Cases\" "
  ALIASES += "tests=\xrefitem tests \"Verifies Requirement\" \"Verified Requirements\" "
  ALIASES += "req=\xrefitem req \"Requirement\" \"Requirements\" "
  ```
- In Rust `.rs` files, classify doc-comments (`///`, `//!`, `#[doc = "..."]`) appearing after `#[cfg(test)]` or immediately before `#[test]` / `#[kani::proof]` as `SWE.4` test citations rather than `SWE.3` implementation citations.
