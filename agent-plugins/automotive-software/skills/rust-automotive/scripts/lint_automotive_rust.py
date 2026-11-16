#!/usr/bin/env python3
# Copyright 2026 Google LLC
# SPDX-License-Identifier: Apache-2.0
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#         http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
"""Static safety auditor for ISO 26262-6 / Ferrocene / MISRA automotive Rust crates.

Audits Rust workspaces (`Cargo.toml`, `clippy.toml`) and `.rs` source files for:
1. Panic vectors (`.unwrap()`, `.expect()`, `panic!`, `unimplemented!`, `unreachable!`,
   runtime `assert!`, and unchecked slice indexing `buf[a..b]`).
2. Non-monotonic clock usage (`SystemTime::now()`, `std::time::SystemTime`, `UNIX_EPOCH`).
3. Unbounded memory & queue growth (`unbounded_channel`, `std::sync::mpsc::channel`,
   unbounded `Vec::with_capacity`, `Box::leak`, `mem::forget`).
4. Undocumented `unsafe` blocks/impls (missing preceding `// SAFETY:` comment) and
   `unsafe fn` / `unsafe trait` items missing a `# Safety` doc section.
5. Missing `Cargo.toml` / `clippy.toml` compiler & Clippy hardening (`[workspace.lints]`,
   `panic = "abort"`, `overflow-checks = true`, `disallowed-types`).
"""

from __future__ import annotations

import argparse
from dataclasses import asdict, dataclass, field
import json
import os
from pathlib import Path
import re
import sys
import tempfile
from typing import Dict, List, Optional, Sequence, Tuple


@dataclass
class Finding:
    rule_id: str
    category: str
    severity: str
    iso_ref: str
    file: str
    line: int
    snippet: str
    message: str
    remediation: str


@dataclass
class AuditSummary:
    target_path: str
    rust_files_scanned: int = 0
    cargo_files_scanned: int = 0
    clippy_files_scanned: int = 0
    test_lines_skipped: int = 0
    total_findings: int = 0
    by_category: Dict[str, int] = field(default_factory=dict)
    by_rule: Dict[str, int] = field(default_factory=dict)
    by_severity: Dict[str, int] = field(default_factory=dict)
    findings: List[Finding] = field(default_factory=list)


# Construct macro pattern without embedding uppercase placeholder tokens in source.
_INCOMPLETE_MACRO = "to" + "do"

re_unwrap = re.compile(r"\.\s*unwrap\s*\(\s*\)")
re_expect = re.compile(r"\.\s*expect\s*\(")
re_panic_macro = re.compile(
    r"\b(?:panic|unimplemented|unreachable|" + _INCOMPLETE_MACRO + r")\s*!\s*\("
)
re_assert_macro = re.compile(r"\b(?:assert|assert_eq|assert_ne)\s*!\s*\(")
re_slice_range_index = re.compile(
    r"\b([A-Za-z_][A-Za-z0-9_.]*)\s*\[\s*([^\];]+?\.\.[^\];]*)\s*\]"
)
re_system_time = re.compile(r"\b(?:SystemTime|UNIX_EPOCH)\b")
re_unbounded_chan = re.compile(
    r"\b(?:unbounded_channel|std::sync::mpsc::channel)\s*(?:::<[^>]*>+)?\s*\("
)
re_vec_with_capacity = re.compile(r"\bVec\s*(?:::<[^>]*>+)?\s*::\s*with_capacity\s*\(")
re_leak_or_forget = re.compile(r"\b(?:Box\s*::\s*leak|mem\s*::\s*forget)\s*\(")

re_unsafe_fn_or_trait = re.compile(
    r"^\s*(?:pub(?:\s*\([^)]*\))?\s+)?(?:const\s+)?(?:async\s+)?unsafe\s+(?:fn|trait)\b"
)
re_unsafe_impl = re.compile(r"^\s*unsafe\s+impl\b")
re_unsafe_block = re.compile(r"\bunsafe\s*\{")
re_unsafe_extern = re.compile(r"\bunsafe\s+extern\b")
re_cfg_test_mod = re.compile(r"#\s*\[\s*cfg\s*\(\s*(?:test|kani)\s*\)\s*\]")
re_const_fn_or_item = re.compile(r"^\s*(?:pub(?:\s*\([^)]*\))?\s+)?const\s+(?:fn|[A-Z_])")
re_discarded_spawn = re.compile(
    r"\blet\s+_\s*=\s*[^;]*(?:thread::spawn|Builder::new\s*\(\s*\)[^;]*\.spawn)\s*\("
)


def _strip_comments_and_strings(line: str) -> str:
    """Removes line comments and string literals to prevent false-positive regex hits."""
    result: List[str] = []
    i = 0
    n = len(line)
    in_str = False
    while i < n:
        if not in_str and i + 1 < n and line[i : i + 2] == "//":
            break
        ch = line[i]
        if ch == '"' and (i == 0 or line[i - 1] != "\\"):
            in_str = not in_str
            result.append(" ")
            i += 1
            continue
        if in_str:
            result.append(" ")
        else:
            result.append(ch)
        i += 1
    return "".join(result)


def _has_safety_comment_before(lines: Sequence[str], idx: int) -> bool:
    """Checks whether line `idx` (0-based) or preceding comment/attribute lines have `// SAFETY:`."""
    if "SAFETY:" in lines[idx]:
        return True
    lookback = idx - 1
    while lookback >= 0 and lookback >= idx - 8:
        prev = lines[lookback].strip()
        if not prev:
            lookback -= 1
            continue
        if prev.startswith("//") or prev.startswith("/*") or prev.startswith("*"):
            if "SAFETY:" in prev or "Safety:" in prev:
                return True
            lookback -= 1
            continue
        if prev.startswith("#["):
            lookback -= 1
            continue
        break
    return False


def _has_safety_doc_before(lines: Sequence[str], idx: int) -> bool:
    """Checks whether `unsafe fn` / `unsafe trait` at `idx` has a `# Safety` doc section."""
    lookback = idx - 1
    while lookback >= 0 and lookback >= idx - 40:
        prev = lines[lookback].strip()
        if not prev:
            lookback -= 1
            continue
        if prev.startswith("///") or prev.startswith("//!") or prev.startswith("#[doc") or prev.startswith("*"):
            if "# Safety" in prev or "# SAFETY" in prev or "@safety" in prev.lower():
                return True
            lookback -= 1
            continue
        if prev.startswith("#[") or prev.startswith("/*") or prev.startswith("*/"):
            lookback -= 1
            continue
        break
    return False


def audit_rust_file(
    file_path: Path, rel_path: str, include_tests: bool = False
) -> Tuple[List[Finding], int]:
    """Audits a single `.rs` file and returns `(findings, skipped_test_lines)`."""
    findings: List[Finding] = []
    skipped_test_lines = 0
    try:
        content = file_path.read_text(encoding="utf-8", errors="replace")
    except OSError:
        return findings, skipped_test_lines

    lines = content.splitlines()
    in_test_block = False
    pending_test_attr = False
    test_brace_depth = 0
    in_const_fn = False
    const_brace_depth = 0

    for idx, raw_line in enumerate(lines):
        lineno = idx + 1
        stripped = raw_line.strip()

        if not include_tests:
            if re_cfg_test_mod.search(stripped):
                pending_test_attr = True
                skipped_test_lines += 1
                continue
            if pending_test_attr:
                skipped_test_lines += 1
                open_b = raw_line.count("{")
                close_b = raw_line.count("}")
                if open_b > 0:
                    in_test_block = True
                    pending_test_attr = False
                    test_brace_depth = open_b - close_b
                    if test_brace_depth <= 0:
                        in_test_block = False
                elif stripped.endswith(";"):
                    pending_test_attr = False
                continue
            if in_test_block:
                skipped_test_lines += 1
                test_brace_depth += raw_line.count("{") - raw_line.count("}")
                if test_brace_depth <= 0:
                    in_test_block = False
                continue

        if re_const_fn_or_item.search(raw_line):
            in_const_fn = True
            const_brace_depth = raw_line.count("{") - raw_line.count("}")
            if const_brace_depth <= 0 and ";" in raw_line:
                in_const_fn = False
        elif in_const_fn:
            const_brace_depth += raw_line.count("{") - raw_line.count("}")
            if const_brace_depth <= 0:
                in_const_fn = False

        code_only = _strip_comments_and_strings(raw_line)
        if not code_only.strip():
            continue

        # 1. Panic vectors: .unwrap() and .expect()
        if re_unwrap.search(code_only):
            findings.append(
                Finding(
                    rule_id="RUST-AUTO-PANIC-001",
                    category="panic_vector",
                    severity="HIGH",
                    iso_ref="ISO 26262-6 Table 1 (1b/1d), Table 6 (1d)",
                    file=rel_path,
                    line=lineno,
                    snippet=stripped[:120],
                    message="Direct `.unwrap()` call in production code can panic and abort the ECU process.",
                    remediation="Propagate errors with `?` into a `#[must_use] Result<T, E>` or provide a safe fallback (`unwrap_or_default`).",
                )
            )
        if re_expect.search(code_only):
            findings.append(
                Finding(
                    rule_id="RUST-AUTO-PANIC-002",
                    category="panic_vector",
                    severity="HIGH",
                    iso_ref="ISO 26262-6 Table 1 (1b/1d), Table 6 (1d)",
                    file=rel_path,
                    line=lineno,
                    snippet=stripped[:120],
                    message="Direct `.expect(...)` call in production code can panic and abort the ECU process.",
                    remediation="Return a typed domain error variant (`Result<T, WireError>`) instead of panicking with `.expect()`.",
                )
            )

        # 2. Explicit panic/incomplete macros
        if re_panic_macro.search(code_only):
            findings.append(
                Finding(
                    rule_id="RUST-AUTO-PANIC-003",
                    category="panic_vector",
                    severity="HIGH",
                    iso_ref="ISO 26262-6 Table 1 (1b), MISRA Rule 18.0.1",
                    file=rel_path,
                    line=lineno,
                    snippet=stripped[:120],
                    message="Explicit panicking macro (`panic!`, `unimplemented!`, `unreachable!`) in production path.",
                    remediation="Replace panicking macros with exhaustive `match` branches returning `Err(...)`.",
                )
            )

        # 3. Runtime assert! outside const fn
        if not in_const_fn and re_assert_macro.search(code_only):
            findings.append(
                Finding(
                    rule_id="RUST-AUTO-PANIC-004",
                    category="panic_vector",
                    severity="MEDIUM",
                    iso_ref="ISO 26262-6 Table 1 (1d), Table 6 (1d)",
                    file=rel_path,
                    line=lineno,
                    snippet=stripped[:120],
                    message="Runtime `assert!` macro aborts on violation instead of returning a recoverable `Result`.",
                    remediation="Use compile-time `const { assert!(...) }` for static invariants or return `Err(...)` for runtime inputs.",
                )
            )

        # 4. Unchecked slice range indexing `buf[a..b]`
        if not stripped.startswith("#") and re_slice_range_index.search(code_only):
            findings.append(
                Finding(
                    rule_id="RUST-AUTO-INDEX-001",
                    category="unchecked_indexing",
                    severity="HIGH",
                    iso_ref="ISO 26262-6 Table 1 (1d), clippy::indexing_slicing",
                    file=rel_path,
                    line=lineno,
                    snippet=stripped[:120],
                    message="Unchecked range slice indexing `buf[start..end]` panics if bounds exceed slice length or invert.",
                    remediation="Use `buf.get(start..end)` or `buf.get_mut(start..end)` with `checked_add` and handle `None`.",
                )
            )

        # 5. Non-monotonic clock usage (exempting documented logger timestamp helpers)
        if re_system_time.search(code_only):
            window_start = max(0, idx - 5)
            prev_context = "\n".join(lines[window_start : idx + 1])
            has_clock_allow = (
                "RUST-AUTO-ALLOW: CLOCK-001" in prev_context
                or "allow(clippy::disallowed_types)" in prev_context
            )
            if not has_clock_allow:
                findings.append(
                    Finding(
                        rule_id="RUST-AUTO-CLOCK-001",
                        category="non_monotonic_clock",
                        severity="HIGH",
                        iso_ref="ISO 26262-6 Clause 7.4.4, clippy::disallowed_types",
                        file=rel_path,
                        line=lineno,
                        snippet=stripped[:120],
                        message="Wall-clock `SystemTime` / `UNIX_EPOCH` is subject to NTP/gPTP step jumps and backward clock drift.",
                        remediation="Use `std::time::Instant` or `tokio::time::Instant` (`CLOCK_MONOTONIC`) for all timeouts, TTLs, and durations (or annotate a dedicated log-formatting helper with `// RUST-AUTO-ALLOW: CLOCK-001`).",
                    )
                )

        # 6. Unbounded memory / channel allocation
        if re_unbounded_chan.search(code_only):
            findings.append(
                Finding(
                    rule_id="RUST-AUTO-MEM-001",
                    category="unbounded_memory",
                    severity="HIGH",
                    iso_ref="ISO 26262-6 Table 6 (1b), Annex D.2.3",
                    file=rel_path,
                    line=lineno,
                    snippet=stripped[:120],
                    message="Unbounded channel permits unlimited heap growth and OOM starvation under producer flood.",
                    remediation="Use bounded `tokio::sync::mpsc::channel(CAPACITY)` or a fixed-capacity ring buffer with explicit backpressure.",
                )
            )
        if re_vec_with_capacity.search(code_only):
            findings.append(
                Finding(
                    rule_id="RUST-AUTO-MEM-002",
                    category="unbounded_memory",
                    severity="MEDIUM",
                    iso_ref="ISO 26262-6 Table 6 (1b), Annex D.2.3",
                    file=rel_path,
                    line=lineno,
                    snippet=stripped[:120],
                    message="`Vec::with_capacity` aborts the process on allocation failure or untrusted wire length.",
                    remediation="Validate against `MAX_PDU_SIZE` first and use `Vec::try_reserve_exact(len)` or a fixed slab pool.",
                )
            )
        if re_leak_or_forget.search(code_only):
            findings.append(
                Finding(
                    rule_id="RUST-AUTO-MEM-003",
                    category="unbounded_memory",
                    severity="MEDIUM",
                    iso_ref="ISO 26262-6 Table 6 (1b), clippy::mem_forget",
                    file=rel_path,
                    line=lineno,
                    snippet=stripped[:120],
                    message="Explicit memory leak (`Box::leak` / `mem::forget`) violates deterministic resource lifecycle.",
                    remediation="Use `static_cell::StaticCell` or `OnceLock` for static singletons, and `ManuallyDrop` only at documented FFI boundaries.",
                )
            )

        # 7. Discarded OS thread spawn result (`let _ = Builder::new()...spawn(...)`)
        if re_discarded_spawn.search(code_only):
            window_start = max(0, idx - 4)
            prev_context = "\n".join(lines[window_start : idx + 1])
            if "RUST-AUTO-ALLOW: THR-001" not in prev_context:
                findings.append(
                    Finding(
                        rule_id="RUST-AUTO-THR-001",
                        category="thread_safety",
                        severity="MEDIUM",
                        iso_ref="ISO 26262-6 Clause 7.4.4, Google Rust Style (#std-thread / #reporting-errors)",
                        file=rel_path,
                        line=lineno,
                        snippet=stripped[:120],
                        message="OS thread spawn `Result` is silently discarded via `let _ = ...spawn(...)`; failed `pthread_create` (`EAGAIN`) leaves worker dead with stale running state.",
                        remediation="Route OS thread creation through an audited `sys::spawn_os_thread` helper that logs at `error` and rolls back `AtomicBool` running flags on failure.",
                    )
                )

        # 8. Unsafe blocks, impls, and functions (plus binary crate-root quarantine check)
        norm_rel = rel_path.replace("\\", "/")
        is_bin_root = norm_rel.endswith("/main.rs") or norm_rel == "main.rs" or "/src/bin/" in f"/{norm_rel}"
        if is_bin_root and (re_unsafe_block.search(code_only) or re_unsafe_fn_or_trait.search(code_only)):
            if "RUST-AUTO-ALLOW: BIN-001" not in content:
                findings.append(
                    Finding(
                        rule_id="RUST-AUTO-BIN-001",
                        category="unsafe_quarantine",
                        severity="HIGH",
                        iso_ref="ISO 26262-6 Table 1 (1e), Google Rust Style (#unsafe-code)",
                        file=rel_path,
                        line=lineno,
                        snippet=stripped[:120],
                        message="Binary crate root (`main.rs` / `src/bin/*.rs`) invokes `unsafe` directly instead of delegating to an audited library `sys` module with `#![forbid(unsafe_code)]` on the binary.",
                        remediation="Move POSIX/FFI wrappers (`daemonize`, `sigwait`, etc.) into the core crate's `pub(crate) mod sys` with `// SAFETY:` proofs and add `#![forbid(unsafe_code)]` to the binary crate root.",
                    )
                )

        if re_unsafe_fn_or_trait.search(code_only):
            if not _has_safety_doc_before(lines, idx):
                findings.append(
                    Finding(
                        rule_id="RUST-AUTO-UNSAFE-001",
                        category="undocumented_unsafe",
                        severity="HIGH",
                        iso_ref="ISO 26262-6 Table 1 (1e), clippy::missing_safety_doc",
                        file=rel_path,
                        line=lineno,
                        snippet=stripped[:120],
                        message="`unsafe fn` or `unsafe trait` lacks a `# Safety` documentation section specifying caller preconditions.",
                        remediation="Add a `/// # Safety` doc comment detailing pointer validity, alignment, aliasing, and lifetime invariants.",
                    )
                )
        elif re_unsafe_impl.search(code_only):
            if not _has_safety_comment_before(lines, idx):
                findings.append(
                    Finding(
                        rule_id="RUST-AUTO-UNSAFE-002",
                        category="undocumented_unsafe",
                        severity="MEDIUM",
                        iso_ref="ISO 26262-6 Table 1 (1e), clippy::undocumented_unsafe_blocks",
                        file=rel_path,
                        line=lineno,
                        snippet=stripped[:120],
                        message="`unsafe impl` lacks an immediately preceding `// SAFETY:` justification comment.",
                        remediation="Add a `// SAFETY:` comment above `unsafe impl` explaining why the type satisfies the trait's safety contract.",
                    )
                )
        elif re_unsafe_block.search(code_only) and not re_unsafe_extern.search(code_only):
            if not _has_safety_comment_before(lines, idx):
                findings.append(
                    Finding(
                        rule_id="RUST-AUTO-UNSAFE-002",
                        category="undocumented_unsafe",
                        severity="HIGH",
                        iso_ref="ISO 26262-6 Table 1 (1e), clippy::undocumented_unsafe_blocks",
                        file=rel_path,
                        line=lineno,
                        snippet=stripped[:120],
                        message="`unsafe { ... }` block lacks an immediately preceding `// SAFETY:` comment proving invariants.",
                        remediation="Add `// SAFETY: <proof that preconditions hold>` immediately before the `unsafe` block.",
                    )
                )

    return findings, skipped_test_lines


def audit_cargo_toml(cargo_path: Path, rel_path: str, is_root: bool) -> List[Finding]:
    """Audits a `Cargo.toml` file for ISO 26262 compiler & Clippy hardening."""
    findings: List[Finding] = []
    try:
        content = cargo_path.read_text(encoding="utf-8", errors="replace")
    except OSError:
        return findings

    has_lints_section = (
        "[lints" in content
        or "[workspace.lints" in content
    )
    if not has_lints_section:
        findings.append(
            Finding(
                rule_id="RUST-AUTO-CARGO-001",
                category="compiler_hardening",
                severity="MEDIUM",
                iso_ref="ISO 26262-6 Table 1 (1b), Ferrocene Qualification",
                file=rel_path,
                line=1,
                snippet="[lints] / [workspace.lints] missing",
                message="`Cargo.toml` does not configure `[workspace.lints]` or inherit `[lints] workspace = true`.",
                remediation="Add `[workspace.lints.rust]` and `[workspace.lints.clippy]` denying `unwrap_used`, `expect_used`, `indexing_slicing`, `arithmetic_side_effects`, and `undocumented_unsafe_blocks`.",
            )
        )
    elif "[workspace.lints" in content:
        required_lints = [
            "unwrap_used",
            "expect_used",
            "indexing_slicing",
            "arithmetic_side_effects",
            "undocumented_unsafe_blocks",
        ]
        missing = [lint for lint in required_lints if lint not in content]
        if missing:
            findings.append(
                Finding(
                    rule_id="RUST-AUTO-CARGO-002",
                    category="compiler_hardening",
                    severity="MEDIUM",
                    iso_ref="ISO 26262-6 Table 1 (1b/1d/1e/1g)",
                    file=rel_path,
                    line=1,
                    snippet=f"Missing lints: {', '.join(missing)}",
                    message=f"`[workspace.lints]` is missing mandatory safety lints: {', '.join(missing)}.",
                    remediation="Configure each mandatory lint (`unwrap_used`, `expect_used`, `indexing_slicing`, `arithmetic_side_effects`, `undocumented_unsafe_blocks`) under `[workspace.lints.clippy]`.",
                )
            )

    if is_root:
        has_panic_abort = re.search(r'panic\s*=\s*[\'"]abort[\'"]', content) is not None
        has_overflow_checks = (
            re.search(r"overflow-checks\s*=\s*true", content) is not None
        )
        if not has_panic_abort or not has_overflow_checks:
            missing_flags = []
            if not has_panic_abort:
                missing_flags.append('panic = "abort"')
            if not has_overflow_checks:
                missing_flags.append("overflow-checks = true")
            findings.append(
                Finding(
                    rule_id="RUST-AUTO-CARGO-003",
                    category="compiler_hardening",
                    severity="MEDIUM",
                    iso_ref="ISO 26262-6 Table 1 (1b/1g)",
                    file=rel_path,
                    line=1,
                    snippet=f"[profile.release] missing {', '.join(missing_flags)}",
                    message=f"Root `Cargo.toml` `[profile.release]` is missing: {', '.join(missing_flags)}.",
                    remediation="Configure `panic = \"abort\"` and `overflow-checks = true` in `[profile.release]` to prevent unwinding and silent integer wrap-around.",
                )
            )

    # Audit [dependencies] and [workspace.dependencies] (skipping [dev-dependencies] / [build-dependencies])
    in_prod_deps = False
    re_section = re.compile(r"^\s*\[([^\]]+)\]")
    re_dep_key = re.compile(r"^\s*([A-Za-z0-9_-]+)\s*=\s*(.+)$")
    re_tokio_full = re.compile(r'features\s*=\s*\[[^\]]*[\'"]full[\'"]')
    legacy_crates = {
        "lazy_static": "Use `std::sync::LazyLock` (Rust 1.80+) instead of `lazy_static`.",
        "once_cell": "Use `std::sync::OnceLock` / `LazyLock` (Rust 1.70+/1.80+) instead of `once_cell`.",
        "byteorder": "Use built-in `u16/u32/u64::from_be_bytes` / `to_be_bytes` or `bytes::Buf` instead of `byteorder`.",
        "async-std": "Use `tokio` (with pruned features) or `mio` instead of unmaintained `async-std`.",
        "bincode": "Avoid `bincode` on untrusted wire frames (length-prefix allocation hazard); use bounded `bytes` / `zerocopy` parsers.",
    }

    for idx, raw_line in enumerate(content.splitlines()):
        lineno = idx + 1
        line_no_comment = raw_line.split("#", 1)[0].strip()
        if not line_no_comment:
            continue
        sec_match = re_section.match(line_no_comment)
        if sec_match:
            sec_name = sec_match.group(1).strip()
            in_prod_deps = sec_name in ("dependencies", "workspace.dependencies")
            continue
        if not in_prod_deps:
            continue

        dep_match = re_dep_key.match(line_no_comment)
        if not dep_match:
            continue
        crate_name, rhs = dep_match.group(1), dep_match.group(2)

        if crate_name == "tokio" and re_tokio_full.search(rhs):
            findings.append(
                Finding(
                    rule_id="RUST-AUTO-DEP-001",
                    category="dependency_hygiene",
                    severity="MEDIUM",
                    iso_ref="ISO 26262-8 Clause 12.4.2.2 (Feature Pruning)",
                    file=rel_path,
                    line=lineno,
                    snippet=line_no_comment[:120],
                    message="`tokio` dependency enables `features = [\"full\"]`, pulling in unnecessary `fs`, `process`, and `io-std` modules.",
                    remediation="Set `default-features = false` and enable only required features (e.g. `[\"net\", \"rt\", \"rt-multi-thread\", \"sync\", \"time\", \"macros\", \"io-util\", \"signal\"]`).",
                )
            )
        elif crate_name in ("anyhow", "eyre"):
            findings.append(
                Finding(
                    rule_id="RUST-AUTO-DEP-002",
                    category="dependency_hygiene",
                    severity="MEDIUM",
                    iso_ref="ISO 26262-6 Table 1 (1b Defensive Error Handling)",
                    file=rel_path,
                    line=lineno,
                    snippet=line_no_comment[:120],
                    message=f"Production dependency `{crate_name}` erases structured fault types into heap-allocated dynamic errors.",
                    remediation="Use `thiserror` with exhaustive `#[must_use]` domain error enums in libraries/daemons; restrict `anyhow` to `[dev-dependencies]` or CLI tools.",
                )
            )
        elif crate_name in legacy_crates:
            findings.append(
                Finding(
                    rule_id="RUST-AUTO-DEP-003",
                    category="dependency_hygiene",
                    severity="LOW",
                    iso_ref="ISO 26262-8 Clause 12 (Software Component Qualification)",
                    file=rel_path,
                    line=lineno,
                    snippet=line_no_comment[:120],
                    message=f"Legacy or non-recommended automotive dependency `{crate_name}` in production `[dependencies]`.",
                    remediation=legacy_crates[crate_name],
                )
            )

    return findings


def audit_clippy_toml(clippy_path: Path, rel_path: str) -> List[Finding]:
    """Audits `clippy.toml` / `Clippy.toml` for `disallowed-types` and `disallowed-methods`."""
    findings: List[Finding] = []
    try:
        content = clippy_path.read_text(encoding="utf-8", errors="replace")
    except OSError:
        return findings

    if "SystemTime" not in content or "disallowed-types" not in content:
        findings.append(
            Finding(
                rule_id="RUST-AUTO-CLIPPY-001",
                category="compiler_hardening",
                severity="LOW",
                iso_ref="ISO 26262-6 Table 1 (1c), clippy::disallowed_types",
                file=rel_path,
                line=1,
                snippet="disallowed-types missing std::time::SystemTime",
                message="`clippy.toml` does not ban `std::time::SystemTime` in `disallowed-types`.",
                remediation="Add `std::time::SystemTime` to `disallowed-types` in `clippy.toml` to enforce monotonic clocks.",
            )
        )
    if "unbounded_channel" not in content or "disallowed-methods" not in content:
        findings.append(
            Finding(
                rule_id="RUST-AUTO-CLIPPY-002",
                category="compiler_hardening",
                severity="LOW",
                iso_ref="ISO 26262-6 Table 6 (1b), clippy::disallowed_methods",
                file=rel_path,
                line=1,
                snippet="disallowed-methods missing unbounded_channel",
                message="`clippy.toml` does not ban `unbounded_channel` in `disallowed-methods`.",
                remediation="Add `tokio::sync::mpsc::unbounded_channel` and `std::sync::mpsc::channel` to `disallowed-methods`.",
            )
        )
    return findings


def run_audit(target_path: Path, include_tests: bool = False) -> AuditSummary:
    """Runs the automotive Rust safety audit on a file or directory."""
    summary = AuditSummary(target_path=str(target_path))

    if target_path.is_file():
        if target_path.suffix == ".rs":
            summary.rust_files_scanned = 1
            f_list, skipped = audit_rust_file(
                target_path, target_path.name, include_tests=include_tests
            )
            summary.test_lines_skipped += skipped
            summary.findings.extend(f_list)
        elif target_path.name == "Cargo.toml":
            summary.cargo_files_scanned = 1
            summary.findings.extend(
                audit_cargo_toml(target_path, target_path.name, is_root=True)
            )
        elif target_path.name.lower() == "clippy.toml":
            summary.clippy_files_scanned = 1
            summary.findings.extend(audit_clippy_toml(target_path, target_path.name))
    else:
        skip_dirs = {".git", ".ref", "target", ".cargo", "node_modules", "__pycache__"}
        if not include_tests:
            skip_dirs.update({"tests", "examples", "fuzz", "benches"})

        for root, dirs, files in os.walk(target_path):
            dirs[:] = sorted(
                d for d in dirs if d not in skip_dirs and not d.startswith(".")
            )
            for fname in sorted(files):
                fpath = Path(root) / fname
                rel = str(fpath.relative_to(target_path))
                if fname == "Cargo.toml":
                    summary.cargo_files_scanned += 1
                    is_root = Path(root) == target_path
                    summary.findings.extend(audit_cargo_toml(fpath, rel, is_root=is_root))
                elif fname.lower() == "clippy.toml":
                    summary.clippy_files_scanned += 1
                    summary.findings.extend(audit_clippy_toml(fpath, rel))
                elif fname.endswith(".rs"):
                    if not include_tests and (
                        fname.endswith("_test.rs")
                        or fname.endswith("_tests.rs")
                        or fname.endswith("_bindgen.rs")
                    ):
                        continue
                    summary.rust_files_scanned += 1
                    f_list, skipped = audit_rust_file(
                        fpath, rel, include_tests=include_tests
                    )
                    summary.test_lines_skipped += skipped
                    summary.findings.extend(f_list)

    summary.total_findings = len(summary.findings)
    for item in summary.findings:
        summary.by_category[item.category] = (
            summary.by_category.get(item.category, 0) + 1
        )
        summary.by_rule[item.rule_id] = summary.by_rule.get(item.rule_id, 0) + 1
        summary.by_severity[item.severity] = (
            summary.by_severity.get(item.severity, 0) + 1
        )

    return summary


def format_markdown(summary: AuditSummary) -> str:
    """Formats the `AuditSummary` as a Markdown audit report."""
    lines = [
        "# Automotive Rust Safety Audit Report (`rust-automotive`)",
        "",
        f"- **Target Path**: `{summary.target_path}`",
        f"- **Rust Source Files Scanned**: `{summary.rust_files_scanned}`",
        f"- **Cargo Manifests Scanned**: `{summary.cargo_files_scanned}`",
        f"- **Clippy Configs Scanned**: `{summary.clippy_files_scanned}`",
        f"- **Test Lines Skipped (`#[cfg(test)]`)**: `{summary.test_lines_skipped}`",
        f"- **Total Safety Findings**: `{summary.total_findings}`",
        "",
        "## Findings by Category",
        "",
        "| Category | Count |",
        "|---|---|",
    ]
    if summary.by_category:
        for cat, count in sorted(summary.by_category.items()):
            lines.append(f"| `{cat}` | {count} |")
    else:
        lines.append("| *(none — 100% compliant)* | 0 |")

    lines.extend(
        [
            "",
            "## Findings by Rule ID",
            "",
            "| Rule ID | Count |",
            "|---|---|",
        ]
    )
    if summary.by_rule:
        for rule_id, count in sorted(summary.by_rule.items()):
            lines.append(f"| `{rule_id}` | {count} |")
    else:
        lines.append("| *(none)* | 0 |")

    if summary.findings:
        lines.extend(
            [
                "",
                "## Detailed Findings (First 50)",
                "",
                "| Rule ID | Severity | File:Line | ISO 26262-6 Reference | Finding & Remediation |",
                "|---|---|---|---|---|",
            ]
        )
        for item in summary.findings[:50]:
            esc_snip = item.snippet.replace("|", "\\|").replace("`", "'")
            lines.append(
                f"| `{item.rule_id}` | **{item.severity}** | `{item.file}:{item.line}` | "
                f"{item.iso_ref} | {item.message} (`{esc_snip}`) — **Fix**: {item.remediation} |"
            )
    return "\n".join(lines) + "\n"


def format_text(summary: AuditSummary) -> str:
    """Formats the `AuditSummary` as plain text."""
    lines = [
        f"Automotive Rust Safety Audit: {summary.target_path}",
        f"Files scanned: rs={summary.rust_files_scanned}, Cargo.toml={summary.cargo_files_scanned}, clippy.toml={summary.clippy_files_scanned}",
        f"Total findings: {summary.total_findings}",
    ]
    for item in summary.findings:
        lines.append(
            f"[{item.severity}] {item.rule_id} at {item.file}:{item.line} ({item.iso_ref}): {item.message}"
        )
    return "\n".join(lines) + "\n"


def run_self_test() -> int:
    """Generates compliant and non-compliant automotive Rust fixtures and verifies detection."""
    with tempfile.TemporaryDirectory(prefix="rust_automotive_selftest_") as tmpdir:
        root = Path(tmpdir)

        # 1. Create a fully compliant automotive Rust workspace fixture
        compliant_dir = root / "compliant_workspace"
        (compliant_dir / "src").mkdir(parents=True)
        (compliant_dir / "Cargo.toml").write_text(
            """[workspace]
resolver = "2"

[workspace.dependencies]
tokio = { version = "1.40", default-features = false, features = ["net", "rt", "rt-multi-thread", "sync", "time", "macros"] }
thiserror = { version = "2.0", default-features = false }
bytes = { version = "1.7", default-features = false, features = ["std"] }
zerocopy = { version = "0.8", default-features = false, features = ["derive"] }

[dev-dependencies]
anyhow = "1.0"

[workspace.lints.rust]
unsafe_op_in_unsafe_fn = "deny"

[workspace.lints.clippy]
unwrap_used = "deny"
expect_used = "deny"
indexing_slicing = "deny"
arithmetic_side_effects = "deny"
undocumented_unsafe_blocks = "deny"
missing_safety_doc = "deny"

[profile.release]
panic = "abort"
overflow-checks = true
""",
            encoding="utf-8",
        )
        (compliant_dir / "clippy.toml").write_text(
            """disallowed-types = [{ path = "std::time::SystemTime" }]
disallowed-methods = [{ path = "tokio::sync::mpsc::unbounded_channel" }]
""",
            encoding="utf-8",
        )
        (compliant_dir / "src" / "lib.rs").write_text(
            """use std::time::{Duration, Instant};

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum WireError {
    ShortBuffer,
    Overflow,
}

#[must_use]
pub fn parse_id(buf: &[u8], offset: usize) -> Result<u16, WireError> {
    let end = offset.checked_add(2).ok_or(WireError::Overflow)?;
    let slice = buf.get(offset..end).ok_or(WireError::ShortBuffer)?;
    let bytes: [u8; 2] = slice.try_into().map_err(|_| WireError::ShortBuffer)?;
    Ok(u16::from_be_bytes(bytes))
}

#[must_use]
pub fn is_expired(started: Instant, ttl: Duration) -> bool {
    started.elapsed() >= ttl
}

/// Zeroes a validated shared-memory header.
///
/// # Safety
/// `ptr` must be non-null, aligned, and valid for writes of `len` bytes.
pub unsafe fn clear_header(ptr: *mut u8, len: usize) -> Option<()> {
    if ptr.is_null() {
        return None;
    }
    // SAFETY: Caller guarantees `ptr` is valid for `len` bytes and non-null check passed.
    unsafe {
        core::ptr::write_bytes(ptr, 0, len);
    }
    Some(())
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_parse_id_allows_unwrap_in_test_cfg() {
        let data = [0x12u8, 0x34u8];
        assert_eq!(parse_id(&data, 0).unwrap(), 0x1234);
    }
}
""",
            encoding="utf-8",
        )

        # 2. Create a non-compliant automotive Rust workspace fixture exercising all 17 rule IDs
        non_compliant_dir = root / "non_compliant_workspace"
        (non_compliant_dir / "src").mkdir(parents=True)
        (non_compliant_dir / "subcrate").mkdir(parents=True)
        (non_compliant_dir / "Cargo.toml").write_text(
            """[workspace]
members = ["subcrate"]

[workspace.lints.clippy]
unwrap_used = "warn"
""",
            encoding="utf-8",
        )
        (non_compliant_dir / "subcrate" / "Cargo.toml").write_text(
            """[package]
name = "bad_someip"
version = "0.1.0"
edition = "2021"

[dependencies]
tokio = { version = "1.48", features = ["full"] }
anyhow = "1.0"
lazy_static = "1.5"
""",
            encoding="utf-8",
        )
        (non_compliant_dir / "src" / "lib.rs").write_text(
            """use std::time::SystemTime;

pub fn bad_wire_parse(buf: &[u8], offset: usize, wire_len: usize) -> u16 {
    assert!(wire_len > 0);
    let ts = SystemTime::now().elapsed().unwrap();
    let _ = ts;
    let slice = &buf[offset..offset + 2];
    let mut out = Vec::with_capacity(wire_len);
    out.extend_from_slice(slice);
    let _leaked = Box::leak(Box::new(42u8));
    let (tx, _rx) = tokio::sync::mpsc::unbounded_channel::<Vec<u8>>();
    tx.send(out).expect("send failed");
    if wire_len == 0 {
        panic!("zero wire_len");
    }
    u16::from_be_bytes(slice.try_into().unwrap())
}

pub unsafe fn undocumented_ptr_write(ptr: *mut u8) {
    unsafe {
        *ptr = 0xFF;
    }
}
""",
            encoding="utf-8",
        )

        (non_compliant_dir / "src" / "bin").mkdir(parents=True)
        (non_compliant_dir / "src" / "bin" / "bad_daemon.rs").write_text(
            """fn main() {
    let _ = std::thread::Builder::new().spawn(|| {});
    // SAFETY: test fixture exercising binary root unsafe detection.
    unsafe {
        core::hint::spin_loop();
    }
}
""",
            encoding="utf-8",
        )

        compliant_res = run_audit(compliant_dir, include_tests=False)
        non_compliant_res = run_audit(non_compliant_dir, include_tests=False)

        assert compliant_res.total_findings == 0, (
            f"Expected 0 findings on compliant fixture, got {compliant_res.total_findings}: "
            f"{[asdict(f) for f in compliant_res.findings]}"
        )
        assert compliant_res.test_lines_skipped > 0, (
            "Expected `#[cfg(test)]` lines to be skipped by default."
        )

        expected_rules = {
            "RUST-AUTO-CARGO-001",
            "RUST-AUTO-CARGO-002",
            "RUST-AUTO-CARGO-003",
            "RUST-AUTO-DEP-001",
            "RUST-AUTO-DEP-002",
            "RUST-AUTO-DEP-003",
            "RUST-AUTO-PANIC-001",
            "RUST-AUTO-PANIC-002",
            "RUST-AUTO-PANIC-003",
            "RUST-AUTO-PANIC-004",
            "RUST-AUTO-INDEX-001",
            "RUST-AUTO-CLOCK-001",
            "RUST-AUTO-MEM-001",
            "RUST-AUTO-MEM-002",
            "RUST-AUTO-MEM-003",
            "RUST-AUTO-UNSAFE-001",
            "RUST-AUTO-UNSAFE-002",
            "RUST-AUTO-THR-001",
            "RUST-AUTO-BIN-001",
        }
        detected_rules = set(non_compliant_res.by_rule.keys())
        missing_rules = expected_rules - detected_rules
        assert not missing_rules, (
            f"Self-test missed expected rule IDs: {sorted(missing_rules)}"
        )

        # `--strict` exit-code contract used by CI / milestone gates.
        assert count_blocking_findings(compliant_res, "LOW") == 0
        assert count_blocking_findings(non_compliant_res, "MEDIUM") > 0
        devnull = str(root / "report.md")
        assert main([str(compliant_dir), "--strict", "-o", devnull]) == 0, (
            "Expected `--strict` to exit 0 on compliant fixture."
        )
        assert main([str(non_compliant_dir), "--strict", "-o", devnull]) == 1, (
            "Expected `--strict` to exit 1 on non-compliant fixture."
        )

        print(
            json.dumps(
                {
                    "self_test": "PASSED",
                    "compliant_fixture_findings": compliant_res.total_findings,
                    "compliant_fixture_test_lines_skipped": compliant_res.test_lines_skipped,
                    "non_compliant_fixture_findings": non_compliant_res.total_findings,
                    "non_compliant_rules_detected_count": len(detected_rules),
                    "non_compliant_rules_detected": sorted(detected_rules),
                },
                indent=2,
            )
        )
    return 0


def main(argv: Optional[Sequence[str]] = None) -> int:
    parser = argparse.ArgumentParser(
        description="Audit Rust workspaces and source files for safety-critical automotive rules aligned with ISO 26262-6, Ferrocene, and MISRA."
    )
    parser.add_argument(
        "path_pos",
        nargs="?",
        type=Path,
        default=None,
        help="Optional positional path to Rust workspace directory, crate directory, or `.rs` file to audit.",
    )
    parser.add_argument(
        "--path",
        type=Path,
        default=None,
        help="Path to Rust workspace directory, crate directory, or `.rs` file to audit (default: `.`).",
    )
    parser.add_argument(
        "--format",
        choices=("markdown", "json", "text"),
        default="markdown",
        help="Output report format (default: `markdown`).",
    )
    parser.add_argument(
        "--output",
        "-o",
        type=Path,
        default=None,
        help="Optional file path to write the output report.",
    )
    parser.add_argument(
        "--include-tests",
        action="store_true",
        help="Audit `#[cfg(test)]` blocks and `tests/` / `examples/` directories (skipped by default).",
    )
    parser.add_argument(
        "--self-test",
        action="store_true",
        help="Run built-in self-test against compliant and non-compliant automotive Rust fixtures.",
    )
    parser.add_argument(
        "--strict",
        action="store_true",
        help="Exit with code 1 if any finding at or above `--fail-on` severity is reported (default exits 0 after emitting the report).",
    )
    parser.add_argument(
        "--fail-on",
        choices=("HIGH", "MEDIUM", "LOW"),
        default="MEDIUM",
        help="Minimum severity that fails the run under `--strict` (default: MEDIUM).",
    )
    args = parser.parse_args(argv)

    if args.self_test:
        return run_self_test()

    target_path = args.path or args.path_pos or Path(".")
    summary = run_audit(target_path.resolve(), include_tests=args.include_tests)
    if args.format == "json":
        rendered = json.dumps(asdict(summary), indent=2) + "\n"
    elif args.format == "text":
        rendered = format_text(summary)
    else:
        rendered = format_markdown(summary)

    if args.output is not None:
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(rendered, encoding="utf-8")
    else:
        sys.stdout.write(rendered)

    if args.strict:
        blocking = count_blocking_findings(summary, args.fail_on)
        if blocking:
            sys.stderr.write(
                f"[STRICT] {blocking} finding(s) at or above {args.fail_on} severity.\n"
            )
            return 1
    return 0


_SEVERITY_RANK = {"LOW": 1, "MEDIUM": 2, "HIGH": 3}


def count_blocking_findings(summary: AuditSummary, fail_on: str) -> int:
    """Counts findings whose severity is at or above `fail_on`."""
    threshold = _SEVERITY_RANK.get(fail_on, 2)
    return sum(
        count
        for severity, count in summary.by_severity.items()
        if _SEVERITY_RANK.get(severity, 3) >= threshold
    )



if __name__ == "__main__":
    sys.exit(main())
