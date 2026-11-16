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
"""Quantitative Rust systems/middleware architecture & post-parity hardening auditor.

Evaluates a Rust workspace on a calibrated 1.0–10.0 scale grounded in:
- Google's FOSS Rust Style Guide (https://google.github.io/styleguide/rustguide.html)
- Comprehensive Rust by Google Android (https://google.github.io/comprehensive-rust/)
- Effective Rust by David Drysdale (https://www.lurklurk.org/effective-rust/)
- Rust API Guidelines (https://rust-lang.github.io/api-guidelines/)
- ISO 26262-6 / Embedded Middleware Engineering Practices

Scoring Model (10.0 points total, floor 1.0):
- Part A: Safety & Compiler Foundation (5.00 pts)
  - F1 (1.50): Workspace Clippy & rustc lint hardening (`unwrap_used`, `expect_used`, `indexing_slicing`, `undocumented_unsafe_blocks`)
  - F2 (1.00): Release/dev profile hardening (`panic = "abort"`, `overflow-checks = true`)
  - F3 (1.50): Zero production panics & unchecked slice indexing (`.unwrap()`, `.expect()`, `panic!`)
  - F4 (1.00): Monotonic clocks (`Instant` vs `SystemTime`) & documented `// SAFETY:` invariants
- Part B: Idiomatic & Embedded Middleware Architecture (5.00 pts)
  - D1 (0.80): Doc-vs-Code architectural integrity (`ARCHITECTURE.md` claims verified in `src/`)
  - D2 (1.10): Reactor multiplexing, thread scaling & minimal runtime dependency footprint
  - D3 (0.90): Lock hierarchy, `Arc::new_cyclic` / `OnceLock` idioms & `deadlock_detection`
  - D4 (0.80): Typed `#[must_use] Result` error surface & zero silent `let _ =` swallowing
  - D5 (0.60): Checked wire/codec arithmetic (no blanket `saturating_*` cursor masking)
  - D6 (0.40): Complete crate-root `unsafe` quarantine (including `src/bin/*.rs` & `main.rs`)
  - D7 (0.40): Copy-on-write zero-copy internal buffers (`bytes::Bytes` vs `Mutex<Vec<u8>>`)
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
from typing import Any, Dict, List, Optional, Sequence, Tuple


@dataclass
class DimensionScore:
    id: str
    name: str
    earned: float
    maximum: float
    style_ref: str
    findings: List[str] = field(default_factory=list)


@dataclass
class HardeningReport:
    workspace: str
    production_rs_files: int = 0
    production_loc: int = 0
    foundation_score: float = 0.0
    architecture_score: float = 0.0
    total_score: float = 1.0
    min_score: float = 8.5
    passed: bool = False
    metrics: Dict[str, Any] = field(default_factory=dict)
    dimensions: List[DimensionScore] = field(default_factory=list)
    recommended_work_packages: List[str] = field(default_factory=list)


RE_CFG_TEST = re.compile(r"#\s*\[\s*cfg\s*\(\s*(?:test|kani)\s*\)\s*\]")
RE_UNWRAP = re.compile(r"\.\s*unwrap\s*\(\s*\)")
RE_EXPECT = re.compile(r"\.\s*expect\s*\(")
RE_PANIC = re.compile(r"\b(?:panic|unimplemented|todo)\s*!\s*\(")
RE_SYSTEM_TIME = re.compile(r"\b(?:SystemTime|UNIX_EPOCH)\b")
RE_UNSAFE_BLOCK = re.compile(r"\bunsafe\s*\{")
RE_UNSAFE_FN = re.compile(r"^\s*(?:pub(?:\s*\([^)]*\))?\s+)?(?:const\s+)?(?:async\s+)?unsafe\s+fn\b")
RE_MUTEX_WEAK = re.compile(r"\bMutex\s*<\s*(?:std::sync::|alloc::sync::)?Weak\s*<")
RE_MUTEX_OPT_ARC = re.compile(r"\bMutex\s*<\s*Option\s*<\s*(?:std::sync::|alloc::sync::)?Arc\s*<")
RE_MUTEX_FIELD = re.compile(r"^\s*(?:pub(?:\s*\([^)]*\))?\s+)?[A-Za-z_][A-Za-z0-9_]*\s*:\s*(?:parking_lot::|std::sync::)?(?:Mutex|RwLock)\s*<")
RE_STRUCT_DECL = re.compile(r"^\s*(?:pub(?:\s*\([^)]*\))?\s+)?struct\s+([A-Za-z0-9_]+)")
RE_LET_UNDERSCORE_CALL = re.compile(r"^\s*let\s+_\s*=\s*([^;]+);")
RE_CALL_EXPR = re.compile(r"[A-Za-z0-9_]\s*\(")
RE_SATURATING = re.compile(r"\.\s*saturating_(?:add|sub|mul)\s*\(")
RE_CHECKED = re.compile(r"\.\s*checked_(?:add|sub|mul)\s*\(")
RE_THISERROR = re.compile(r"\bthiserror::Error\b")
RE_FN_BOOL = re.compile(r"\bfn\s+[A-Za-z0-9_]+\s*(?:<[^>]*>)?\s*\([^)]*\)\s*->\s*bool\b")
RE_FN_RESULT = re.compile(r"\bfn\s+[A-Za-z0-9_]+\s*(?:<[^>]*>)?\s*\([^)]*\)\s*->\s*(?:[A-Za-z0-9_:]*Result|Option)\b")
RE_C_DESERIALIZE_U32 = re.compile(r"\bfn\s+deserialize\b[^{;]*->\s*u32\b")
RE_THREAD_SLEEP = re.compile(r"\bthread::sleep\s*\(")
RE_RAW_SPAWN = re.compile(r"\b(?:std::thread::spawn|thread::spawn|Builder::new\s*\(\s*\)[^;]*\.spawn)\s*\(")
RE_DISCARDED_SPAWN = re.compile(r"\blet\s+_\s*=\s*[^;]*(?:thread::spawn|Builder::new\s*\(\s*\)[^;]*\.spawn)\s*\(")
RE_POLL_50MS = re.compile(r"\bpoll_readable\s*\(\s*(?:50|20|100)\s*\)")
RE_DEDICATED_READER_CAP = re.compile(r"\bMAX_DEDICATED_(?:CLIENT|SERVER)_READERS\b")
RE_TOKIO_USE = re.compile(r"\btokio(?:_util)?::")
RE_FORBID_OR_DENY_UNSAFE = re.compile(r"#!\s*\[\s*(?:forbid|deny)\s*\(\s*unsafe_code\s*\)\s*\]")

CODEC_PATH_KEYWORDS = (
    "/protocol/",
    "/message/",
    "deserializ",
    "serializ",
    "/tp.rs",
    "tp_message",
    "tp_reassembler",
    "entry_impl",
    "option_impl",
    "-sd/src/message",
    "/sd/src/message",
    "/codec",
    "/wire",
)


def _strip_line_comment_and_strings(line: str) -> str:
    out: List[str] = []
    i, n = 0, len(line)
    in_str = False
    while i < n:
        if not in_str and i + 1 < n and line[i : i + 2] == "//":
            break
        ch = line[i]
        if ch == '"' and (i == 0 or line[i - 1] != "\\"):
            in_str = not in_str
            out.append(" ")
            i += 1
            continue
        out.append(" " if in_str else ch)
        i += 1
    return "".join(out)


def _is_codec_file(rel_path: str) -> bool:
    low = rel_path.lower().replace("\\", "/")
    if "service_discovery_impl" in low:
        return False
    return any(k in low for k in CODEC_PATH_KEYWORDS)


def _collect_production_rust_files(workspace: Path) -> List[Path]:
    crates_dir = workspace / "crates" if (workspace / "crates").is_dir() else workspace
    skip_dirs = {".git", "target", ".cargo", "tests", "benches", "examples", "fuzz", "node_modules"}
    result: List[Path] = []
    for root, dirs, files in os.walk(crates_dir):
        dirs[:] = sorted(
            d
            for d in dirs
            if d not in skip_dirs
            and not d.startswith(".")
            and not d.endswith("-testkit")
            and d != "testkit"
        )
        for fname in sorted(files):
            if not fname.endswith(".rs"):
                continue
            if fname.endswith(("_test.rs", "_tests.rs")):
                continue
            result.append(Path(root) / fname)
    return result


def _parse_prod_deps_from_cargo(cargo_toml: Path) -> Tuple[Set[str], Set[str]]:
    """Returns `(default_prod_deps, optional_deps)` from a `Cargo.toml`."""
    default_deps: Set[str] = set()
    optional_deps: Set[str] = set()
    if not cargo_toml.is_file():
        return default_deps, optional_deps

    in_deps = False
    default_features_line = ""
    sec_re = re.compile(r"^\s*\[([^\]]+)\]")
    dep_re = re.compile(r"^\s*([A-Za-z0-9_-]+)(?:\.[A-Za-z0-9_-]+)?\s*=\s*(.+)$")

    for raw in cargo_toml.read_text(encoding="utf-8", errors="ignore").splitlines():
        line = raw.split("#", 1)[0].strip()
        if not line:
            continue
        sm = sec_re.match(line)
        if sm:
            sec = sm.group(1).strip()
            in_deps = sec == "dependencies" or (sec.startswith("target.") and sec.endswith(".dependencies"))
            continue
        if line.startswith("default") and "=" in line and not in_deps:
            default_features_line += " " + line
        if not in_deps:
            continue
        dm = dep_re.match(line)
        if not dm:
            continue
        name, rhs = dm.group(1), dm.group(2)
        if re.search(r"\boptional\s*=\s*true\b", rhs):
            if f'"{name}"' in default_features_line or f"'{name}'" in default_features_line:
                default_deps.add(name)
            else:
                optional_deps.add(name)
        else:
            default_deps.add(name)
    return default_deps, optional_deps


def _resolve_arch_doc(workspace: Path, arch_doc_arg: Optional[str]) -> Optional[Path]:
    if arch_doc_arg:
        p = Path(arch_doc_arg)
        if not p.is_absolute():
            p = workspace / p
        return p if p.is_file() else None
    for cand in (workspace / "docs" / "ARCHITECTURE.md", workspace / "ARCHITECTURE.md"):
        if cand.is_file():
            return cand
    return None


def audit_architecture(
    workspace: Path, arch_doc_arg: Optional[str] = None, min_score: float = 8.5
) -> HardeningReport:
    report = HardeningReport(workspace=str(workspace), min_score=min_score)
    rs_files = _collect_production_rust_files(workspace)
    report.production_rs_files = len(rs_files)

    root_cargo = workspace / "Cargo.toml"
    root_cargo_text = root_cargo.read_text(encoding="utf-8", errors="ignore") if root_cargo.is_file() else ""

    all_cargo_files = [root_cargo] if root_cargo.is_file() else []
    if (workspace / "crates").is_dir():
        for cf in sorted((workspace / "crates").glob("*/Cargo.toml")):
            if not cf.parent.name.endswith("-testkit"):
                all_cargo_files.append(cf)

    prod_deps: Set[str] = set()
    all_cargo_text = root_cargo_text
    for cf in all_cargo_files:
        txt = cf.read_text(encoding="utf-8", errors="ignore")
        if cf != root_cargo:
            all_cargo_text += "\n" + txt
            d_set, _ = _parse_prod_deps_from_cargo(cf)
            prod_deps.update(d_set)
        elif "[package]" in txt:
            d_set, _ = _parse_prod_deps_from_cargo(cf)
            prod_deps.update(d_set)

    # Counters across production source files
    panic_sites = 0
    clock_violations = 0
    undocumented_unsafe = 0
    unsafe_outside_sys: List[str] = []
    bin_roots_missing_forbid: List[str] = []
    mutex_weak_sites: List[str] = []
    mutex_opt_arc_count = 0
    total_mutex_fields = 0
    god_lock_structs: List[str] = []
    seqcst_files = 0
    asymmetric_atomic_files = 0
    unannotated_let_underscore: List[str] = []
    thiserror_count = 0
    fn_bool_count = 0
    fn_result_count = 0
    c_deserialize_u32_sites: List[str] = []
    codec_saturating_sites: List[str] = []
    codec_checked_count = 0
    thread_sleep_sites: List[str] = []
    raw_spawn_outside_sys: List[str] = []
    discarded_spawn_sites: List[str] = []
    poll_50ms_sites = 0
    dedicated_reader_caps = 0
    has_boardnet_or_mio_reactor = False
    tokio_prod_uses = 0
    has_cow_bytes_payload = False
    has_mutex_vec_payload = False
    all_prod_code_concat: List[str] = []

    for fpath in rs_files:
        rel = str(fpath.relative_to(workspace)).replace("\\", "/")
        raw_text = fpath.read_text(encoding="utf-8", errors="ignore")
        lines = raw_text.splitlines()
        report.production_loc += len(lines)

        is_sys = "/sys/" in f"/{rel}" or rel.endswith("/sys.rs")
        is_bin = rel.endswith("/main.rs") or rel == "main.rs" or "/src/bin/" in f"/{rel}"
        is_codec = _is_codec_file(rel)

        if is_bin and not RE_FORBID_OR_DENY_UNSAFE.search(raw_text):
            bin_roots_missing_forbid.append(rel)

        if "boardnet_reactor" in rel or "mio::Poll" in raw_text or "BoardnetReactor" in raw_text:
            has_boardnet_or_mio_reactor = True

        if "payload_impl" in rel or "payload.rs" in rel:
            if "Bytes" in raw_text and ("Shared(" in raw_text or "RwLock" in raw_text or "as_bytes" in raw_text):
                has_cow_bytes_payload = True
            elif "Mutex<" in raw_text and "Vec<" in raw_text:
                has_mutex_vec_payload = True

        in_test = False
        pending_test = False
        brace_depth = 0
        current_struct: Optional[str] = None
        struct_depth = 0
        struct_mutex_count = 0
        file_has_seqcst = False
        file_has_relaxed = False

        prod_lines_this_file: List[str] = []

        for idx, raw_line in enumerate(lines):
            lineno = idx + 1
            stripped = raw_line.strip()

            if RE_CFG_TEST.search(stripped):
                pending_test = True
                continue
            if pending_test:
                ob, cb = raw_line.count("{"), raw_line.count("}")
                if ob > 0:
                    in_test = True
                    pending_test = False
                    brace_depth = ob - cb
                    if brace_depth <= 0:
                        in_test = False
                elif stripped.endswith(";"):
                    pending_test = False
                continue
            if in_test:
                brace_depth += raw_line.count("{") - raw_line.count("}")
                if brace_depth <= 0:
                    in_test = False
                continue

            code = _strip_line_comment_and_strings(raw_line)
            prod_lines_this_file.append(code)
            if not code.strip():
                continue

            # Struct lock-density tracking
            sm = RE_STRUCT_DECL.match(code)
            if sm and "{" in code:
                current_struct = sm.group(1)
                struct_depth = code.count("{") - code.count("}")
                struct_mutex_count = 0
                if struct_depth <= 0:
                    current_struct = None
            elif current_struct is not None:
                if RE_MUTEX_FIELD.match(code):
                    struct_mutex_count += 1
                    total_mutex_fields += 1
                struct_depth += code.count("{") - code.count("}")
                if struct_depth <= 0:
                    if struct_mutex_count > 35:
                        god_lock_structs.append(f"{rel}:{current_struct} ({struct_mutex_count} locks)")
                    current_struct = None
            elif RE_MUTEX_FIELD.match(code):
                total_mutex_fields += 1

            if RE_UNWRAP.search(code) or RE_EXPECT.search(code) or RE_PANIC.search(code):
                panic_sites += 1

            if RE_SYSTEM_TIME.search(code):
                prev_ctx = "\n".join(lines[max(0, idx - 5) : idx + 1])
                if "CLOCK-001" not in prev_ctx and "disallowed_types" not in prev_ctx:
                    clock_violations += 1

            if (RE_UNSAFE_BLOCK.search(code) or RE_UNSAFE_FN.search(code)) and "unsafe extern" not in code:
                if not is_sys:
                    unsafe_outside_sys.append(f"{rel}:{lineno}")
                prev_safety = "\n".join(lines[max(0, idx - 8) : idx + 1])
                if "SAFETY:" not in prev_safety and "# Safety" not in prev_safety:
                    undocumented_unsafe += 1

            if RE_MUTEX_WEAK.search(code):
                mutex_weak_sites.append(f"{rel}:{lineno}")
            if RE_MUTEX_OPT_ARC.search(code):
                mutex_opt_arc_count += 1

            if "Ordering::SeqCst" in code:
                file_has_seqcst = True
            if "Ordering::Relaxed" in code:
                file_has_relaxed = True

            m_let = RE_LET_UNDERSCORE_CALL.match(code)
            if m_let:
                rhs = m_let.group(1).strip()
                # Ignore simple identifier/tuple drops like `let _ = token;` or `.equals(` probes
                if RE_CALL_EXPR.search(rhs) and ".equals(" not in rhs:
                    prev_annot = "\n".join(lines[max(0, idx - 4) : idx + 1])
                    if (
                        "let_underscore_must_use" not in prev_annot
                        and "Ignored as upstream" not in prev_annot
                    ):
                        unannotated_let_underscore.append(f"{rel}:{lineno}")

            if RE_THISERROR.search(code):
                thiserror_count += 1
            if RE_FN_BOOL.search(code):
                fn_bool_count += 1
            if RE_FN_RESULT.search(code):
                fn_result_count += 1
            if RE_C_DESERIALIZE_U32.search(code):
                prev_doc = "\n".join(lines[max(0, idx - 3) : idx + 1])
                if "#[must_use]" not in prev_doc:
                    c_deserialize_u32_sites.append(f"{rel}:{lineno}")

            if is_codec:
                if RE_SATURATING.search(code):
                    # Allow explicit upstream saturation comment on the line or previous line
                    prev_2 = "\n".join(lines[max(0, idx - 2) : idx + 1]).lower()
                    if "saturat" not in prev_2.split("saturating_")[0] and "clamp" not in prev_2:
                        codec_saturating_sites.append(f"{rel}:{lineno}")
                if RE_CHECKED.search(code):
                    codec_checked_count += len(RE_CHECKED.findall(code))

            if RE_THREAD_SLEEP.search(code) and not is_sys:
                thread_sleep_sites.append(f"{rel}:{lineno}")
            if RE_DISCARDED_SPAWN.search(code):
                discarded_spawn_sites.append(f"{rel}:{lineno}")
            if RE_RAW_SPAWN.search(code) and not is_sys:
                raw_spawn_outside_sys.append(f"{rel}:{lineno}")
            if RE_POLL_50MS.search(code):
                poll_50ms_sites += 1
            if RE_DEDICATED_READER_CAP.search(code):
                dedicated_reader_caps += 1
            if RE_TOKIO_USE.search(code):
                tokio_prod_uses += len(RE_TOKIO_USE.findall(code))

        if file_has_seqcst:
            seqcst_files += 1
            if file_has_relaxed:
                asymmetric_atomic_files += 1
        all_prod_code_concat.append("\n".join(prod_lines_this_file))

    joined_prod_src = "\n".join(all_prod_code_concat)

    # Check ARCHITECTURE.md ghost claims
    arch_doc = _resolve_arch_doc(workspace, arch_doc_arg)
    arch_text = arch_doc.read_text(encoding="utf-8", errors="ignore") if arch_doc else ""
    ghost_claims: List[str] = []
    if arch_text:
        claim_probes = [
            ("CancellationToken", "CancellationToken"),
            ("JoinSet", "JoinSet"),
            ("tokio::select!", "tokio::select!"),
            ("DelayQueue", "DelayQueue"),
            ("FramedRead", "FramedRead"),
        ]
        for label, needle in claim_probes:
            if label in arch_text:
                # Ensure ARCHITECTURE.md isn't explicitly stating "zero JoinSet" / "no CancellationToken"
                negated = re.search(
                    rf"(?:no|zero|without|neither|dropped|removed|eliminated)[^\n.]*\b{re.escape(label)}",
                    arch_text,
                    re.I,
                )
                if not negated and needle not in joined_prod_src:
                    ghost_claims.append(
                        f"`{arch_doc.name}` claims `{label}`, which has 0 occurrences in production `src/`"
                    )

    has_lock_hierarchy_doc = bool(
        re.search(r"lock\s+(?:acquisition\s+)?hierarchy|lock\s+order|tier\s+1", arch_text, re.I)
    )
    has_deadlock_detection = "deadlock_detection" in all_cargo_text
    has_let_underscore_lint = re.search(
        r'let_underscore_must_use\s*=\s*[\'"]deny[\'"]', all_cargo_text
    ) is not None

    # =========================================================================
    # Part A: Safety & Compiler Foundation (5.00 pts)
    # =========================================================================
    f_earned = 5.00
    f_notes: List[str] = []
    req_lints = ("unwrap_used", "expect_used", "indexing_slicing", "undocumented_unsafe_blocks")
    missing_lints = [l for l in req_lints if l not in root_cargo_text]
    if missing_lints:
        f_earned -= min(1.50, 0.40 * len(missing_lints))
        f_notes.append(f"Missing workspace Clippy lints: {', '.join(missing_lints)}")
    if not re.search(r'panic\s*=\s*[\'"]abort[\'"]', root_cargo_text):
        f_earned -= 0.50
        f_notes.append('Missing `panic = "abort"` in `[profile.release]`')
    if not re.search(r"overflow-checks\s*=\s*true", root_cargo_text):
        f_earned -= 0.50
        f_notes.append("Missing `overflow-checks = true` in Cargo profiles")
    if panic_sites > 0:
        f_earned -= min(1.50, 0.15 * panic_sites)
        f_notes.append(f"{panic_sites} `.unwrap()`/`.expect()`/`panic!()` sites in production code")
    if clock_violations > 0:
        f_earned -= min(0.50, 0.25 * clock_violations)
        f_notes.append(f"{clock_violations} un-exempted `SystemTime` sites")
    if undocumented_unsafe > 0:
        f_earned -= min(0.50, 0.25 * undocumented_unsafe)
        f_notes.append(f"{undocumented_unsafe} undocumented `unsafe` blocks/fns")
    f_earned = round(max(0.0, f_earned), 2)

    report.dimensions.append(
        DimensionScore(
            id="F0",
            name="Safety,Bounded-Memory & Compiler Foundation",
            earned=f_earned,
            maximum=5.00,
            style_ref="ISO 26262-6 Table 1 & https://google.github.io/styleguide/rustguide.html#prefer-expect-over-unwrap",
            findings=f_notes,
        )
    )

    # =========================================================================
    # Part B: Idiomatic & Embedded Middleware Architecture (5.00 pts)
    # =========================================================================

    # D1 (0.80): Doc-vs-Code Architectural Integrity
    d1 = 0.80
    d1_notes: List[str] = []
    if not arch_doc:
        d1 -= 0.30
        d1_notes.append("No `docs/ARCHITECTURE.md` found to verify runtime and lock contracts")
    elif ghost_claims:
        d1 -= min(0.80, 0.25 * len(ghost_claims))
        d1_notes.extend(ghost_claims)
    d1 = round(max(0.0, d1), 2)
    report.dimensions.append(
        DimensionScore(
            id="D1",
            name="Doc-vs-Code Architectural Integrity",
            earned=d1,
            maximum=0.80,
            style_ref="https://google.github.io/styleguide/rustguide.html#when-and-how-to-write-documentation",
            findings=d1_notes,
        )
    )

    # D2 (1.10): Reactor Multiplexing, Thread Scaling & Dependency Footprint
    d2 = 1.10
    d2_notes: List[str] = []
    tokio_in_prod = "tokio" in prod_deps or "tokio-util" in prod_deps
    if tokio_in_prod and tokio_prod_uses <= 15:
        d2 -= 0.30
        d2_notes.append(
            f"Vestigial `tokio` in default production `[dependencies]` with only {tokio_prod_uses} `tokio::` references (replace signal/timer glue with POSIX `sigwait` / reactor)"
        )
    if not has_boardnet_or_mio_reactor and (poll_50ms_sites > 0 or len(raw_spawn_outside_sys) > 10):
        d2 -= 0.45
        d2_notes.append(
            f"Per-socket/per-peer OS thread scaling detected ({poll_50ms_sites} fixed-timeout `poll_readable` loops, {len(raw_spawn_outside_sys)} raw spawns); multiplex onto `poll(2)`/`mio` reactor with `eventfd` wakeup"
        )
    if dedicated_reader_caps > 0:
        d2 -= 0.15
        d2_notes.append(
            "Hybrid `MAX_DEDICATED_*_READERS` thread caps spawn extra UDS reader threads alongside `LocalReadReactor`"
        )
    if len(thread_sleep_sites) > 5:
        d2 -= 0.20
        d2_notes.append(
            f"{len(thread_sleep_sites)} `thread::sleep` sites in production code (e.g. {', '.join(thread_sleep_sites[:3])}); schedule delays on a shared timer wheel"
        )
    elif len(thread_sleep_sites) > 0:
        d2 -= 0.05
        d2_notes.append(
            f"{len(thread_sleep_sites)} bounded teardown/drain `thread::sleep` site(s) ({', '.join(thread_sleep_sites[:3])})"
        )
    if discarded_spawn_sites:
        d2 -= min(0.25, 0.05 * len(discarded_spawn_sites))
        d2_notes.append(
            f"{len(discarded_spawn_sites)} unchecked OS thread spawns discarding `io::Result<JoinHandle>`"
        )
    d2 = round(max(0.0, d2), 2)
    report.dimensions.append(
        DimensionScore(
            id="D2",
            name="Reactor Multiplexing, Thread Scaling & Dependency Footprint",
            earned=d2,
            maximum=1.10,
            style_ref="https://google.github.io/styleguide/rustguide.html#dont-start-new-threads-through-the-rust-standard-library-chrome",
            findings=d2_notes,
        )
    )

    # D3 (0.90): Lock Hierarchy, Arc::new_cyclic / OnceLock & Atomic Symmetry
    d3 = 0.90
    d3_notes: List[str] = []
    if mutex_weak_sites:
        d3 -= min(0.30, 0.04 * len(mutex_weak_sites))
        d3_notes.append(
            f"{len(mutex_weak_sites)} `Mutex<Weak<Self>>` self-links; construct with `Arc::new_cyclic` + lock-free `Weak<Self>`"
        )
    if mutex_opt_arc_count > 25:
        d3 -= 0.15
        d3_notes.append(
            f"{mutex_opt_arc_count} `Mutex<Option<Arc<...>>>` fields; convert write-once handles to `std::sync::OnceLock`"
        )
    if not has_lock_hierarchy_doc:
        d3 -= 0.20
        d3_notes.append("Missing numbered lock-acquisition hierarchy table in `ARCHITECTURE.md`")
    if not has_deadlock_detection and total_mutex_fields > 20:
        d3 -= 0.15
        d3_notes.append("Missing `parking_lot/deadlock_detection` watchdog in test configuration")
    if god_lock_structs:
        d3 -= min(0.15, 0.08 * len(god_lock_structs))
        d3_notes.append(f"High per-struct lock fragmentation: {', '.join(god_lock_structs[:3])}")
    if seqcst_files > 15 and asymmetric_atomic_files > 5:
        d3 -= 0.10
        d3_notes.append(
            f"{asymmetric_atomic_files} files mix `Ordering::SeqCst` and `Ordering::Relaxed`; standardize lifecycle flags to `Release`/`Acquire`"
        )
    if total_mutex_fields > 150:
        d3 -= 0.15
        d3_notes.append(
            f"{total_mutex_fields} `Mutex`/`RwLock` fields across translated classes (residual C++ structural lock density)"
        )
    d3 = round(max(0.0, d3), 2)
    report.dimensions.append(
        DimensionScore(
            id="D3",
            name="Lock Hierarchy, Arc::new_cyclic / OnceLock & Atomic Symmetry",
            earned=d3,
            maximum=0.90,
            style_ref="https://google.github.io/styleguide/rustguide.html#avoid-rc-and-arc",
            findings=d3_notes,
        )
    )

    # D4 (0.80): Typed #[must_use] Result Error Surface & Zero Silent `let _ =`
    d4 = 0.80
    d4_notes: List[str] = []
    if not has_let_underscore_lint:
        d4 -= 0.25
        d4_notes.append('Missing `let_underscore_must_use = "deny"` in `[workspace.lints.clippy]`')
    if len(unannotated_let_underscore) > 25:
        d4 -= 0.35
        d4_notes.append(
            f"{len(unannotated_let_underscore)} un-annotated `let _ = <call>` sites silently discarding errors"
        )
    elif len(unannotated_let_underscore) > 0:
        d4 -= min(0.20, 0.03 * len(unannotated_let_underscore))
        d4_notes.append(
            f"{len(unannotated_let_underscore)} un-annotated `let _ = <call>` site(s): {', '.join(unannotated_let_underscore[:5])}"
        )
    if c_deserialize_u32_sites or (thiserror_count < 4 and fn_bool_count > 50):
        d4 -= 0.20
        d4_notes.append(
            f"C-style error flattening ({len(c_deserialize_u32_sites)} un-annotated `deserialize -> u32` sites, {thiserror_count} `thiserror` enums); use `#[must_use] Result<T, E>`"
        )
    elif fn_bool_count > fn_result_count and fn_bool_count > 200:
        d4 -= 0.15
        d4_notes.append(
            f"Internal C++ parity helpers still return `bool` ({fn_bool_count} fns) more often than `Result`/`Option` ({fn_result_count} fns)"
        )
    d4 = round(max(0.0, d4), 2)
    report.dimensions.append(
        DimensionScore(
            id="D4",
            name="Typed #[must_use] Result Error Surface & Zero Silent Drops",
            earned=d4,
            maximum=0.80,
            style_ref="https://google.github.io/styleguide/rustguide.html#reporting-errors & #must_use-on-functions",
            findings=d4_notes,
        )
    )

    # D5 (0.60): Checked Wire/Codec Arithmetic Discipline
    d5 = 0.60
    d5_notes: List[str] = []
    if len(codec_saturating_sites) > 20:
        d5 -= 0.55
        d5_notes.append(
            f"{len(codec_saturating_sites)} `saturating_*` calls in wire/IPC/codec modules mask malformed frame offsets; replace with `checked_*` + `?`"
        )
    elif len(codec_saturating_sites) > 0:
        d5 -= min(0.40, 0.05 * len(codec_saturating_sites))
        d5_notes.append(
            f"{len(codec_saturating_sites)} `saturating_*` call(s) in codec modules: {', '.join(codec_saturating_sites[:5])}"
        )
    d5 = round(max(0.0, d5), 2)
    report.dimensions.append(
        DimensionScore(
            id="D5",
            name="Checked Wire/Codec Arithmetic (No `saturating_*` Masking)",
            earned=d5,
            maximum=0.60,
            style_ref="https://www.lurklurk.org/effective-rust/ & ISO 26262-6 Table 1 (1d/1g)",
            findings=d5_notes,
        )
    )

    # D6 (0.40): Complete Crate-Root `unsafe` Quarantine
    d6 = 0.40
    d6_notes: List[str] = []
    if unsafe_outside_sys:
        d6 -= 0.30
        d6_notes.append(
            f"{len(unsafe_outside_sys)} `unsafe` block(s)/fn(s) outside `sys/` ({', '.join(unsafe_outside_sys[:4])}); move into `pub(crate) mod sys`"
        )
    if bin_roots_missing_forbid:
        d6 -= min(0.10, 0.05 * len(bin_roots_missing_forbid))
        d6_notes.append(
            f"Binary crate root(s) without `#![forbid(unsafe_code)]`: {', '.join(bin_roots_missing_forbid)}"
        )
    d6 = round(max(0.0, d6), 2)
    report.dimensions.append(
        DimensionScore(
            id="D6",
            name="Crate-Root `unsafe` Quarantine (Libraries & Binaries)",
            earned=d6,
            maximum=0.40,
            style_ref="https://google.github.io/styleguide/rustguide.html#unsafe-code",
            findings=d6_notes,
        )
    )

    # D7 (0.40): Copy-on-Write Zero-Copy Internal Buffers
    d7 = 0.40
    d7_notes: List[str] = []
    if has_mutex_vec_payload and not has_cow_bytes_payload:
        d7 -= 0.35
        d7_notes.append(
            "Payload storage wraps `Vec<u8>` in a `Mutex` (deep-copying on `clone`/`equals`); use copy-on-write `bytes::Bytes` + scoped slice closures"
        )
    d7 = round(max(0.0, d7), 2)
    report.dimensions.append(
        DimensionScore(
            id="D7",
            name="Copy-on-Write `Bytes` Internal Buffer Seams",
            earned=d7,
            maximum=0.40,
            style_ref="https://google.github.io/styleguide/rustguide.html#prefer-borrows-over-deep-copies & #more-on-accessors-that-accept-closures",
            findings=d7_notes,
        )
    )

    report.foundation_score = f_earned
    report.architecture_score = round(d1 + d2 + d3 + d4 + d5 + d6 + d7, 2)
    report.total_score = round(max(1.0, min(10.0, report.foundation_score + report.architecture_score)), 2)
    report.passed = report.total_score >= min_score

    report.metrics = {
        "panic_sites": panic_sites,
        "clock_violations": clock_violations,
        "unsafe_outside_sys": len(unsafe_outside_sys),
        "bin_roots_missing_forbid": bin_roots_missing_forbid,
        "ghost_arch_claims": len(ghost_claims),
        "tokio_in_default_prod_deps": tokio_in_prod,
        "tokio_prod_call_sites": tokio_prod_uses,
        "mutex_weak_sites": len(mutex_weak_sites),
        "mutex_opt_arc_fields": mutex_opt_arc_count,
        "total_mutex_rwlock_fields": total_mutex_fields,
        "unannotated_let_underscore_calls": len(unannotated_let_underscore),
        "thiserror_enums": thiserror_count,
        "codec_saturating_calls": len(codec_saturating_sites),
        "codec_checked_calls": codec_checked_count,
        "prod_thread_sleep_sites": len(thread_sleep_sites),
        "discarded_thread_spawns": len(discarded_spawn_sites),
        "cow_bytes_payload": has_cow_bytes_payload,
    }

    # Dependency-ordered Class B Work-Package Sequencer (WP-A -> WP-B -> WP-C -> WP-D)
    if tokio_in_prod and tokio_prod_uses <= 8 or unsafe_outside_sys or mutex_weak_sites:
        report.recommended_work_packages.append(
            "WP-A (Leaf Cleanup & Footprint): Prune vestigial async runtime deps (`tokio`), move binary `unsafe` into `sys::*` (`sigwait`/`daemonize`), and replace `Mutex<Weak<Self>>` with `Arc::new_cyclic`."
        )
    if not has_let_underscore_lint or unannotated_let_underscore or codec_saturating_sites or c_deserialize_u32_sites:
        report.recommended_work_packages.append(
            "WP-B (Error Contracts -> Checked Codec Math): Introduce `#[must_use] Result<T, E>` (`thiserror`) on internal codecs/I/O FIRST, enable `clippy::let_underscore_must_use = \"deny\"`, and replace `saturating_*` wire/IPC math with `checked_*` + `?`."
        )
    if has_mutex_vec_payload and not has_cow_bytes_payload:
        report.recommended_work_packages.append(
            "WP-C (Zero-Copy CoW Buffers): Back `PayloadImpl` with copy-on-write `bytes::Bytes` / `Owned(Vec<u8>)` under `RwLock` with `with_slice` / `as_bytes` internal accessors."
        )
    if poll_50ms_sites or dedicated_reader_caps or len(thread_sleep_sites) > 3 or not has_lock_hierarchy_doc:
        report.recommended_work_packages.append(
            "WP-D (Reactor & Lock Consolidation): Multiplex sockets onto `poll(2)`/`mio` readiness reactors + `eventfd` `WakeFd`, replace `thread::sleep` threads with `TimerReactor` deadlines, and document/test the N-tier lock hierarchy (`parking_lot/deadlock_detection`)."
        )

    return report


def format_markdown(report: HardeningReport) -> str:
    verdict = "PASS" if report.passed else "NEEDS HARDENING"
    lines = [
        f"# Rust Systems & Middleware Architecture Hardening Audit (`{report.total_score:.1f} / 10.0` — {verdict})",
        "",
        f"- **Workspace**: `{report.workspace}`",
        f"- **Production Rust Scope**: `{report.production_rs_files}` files (`{report.production_loc:,}` LOC, excluding tests/testkit)",
        f"- **Part A — Safety & Compiler Foundation**: **`{report.foundation_score:.2f} / 5.00`**",
        f"- **Part B — Idiomatic & Embedded Architecture**: **`{report.architecture_score:.2f} / 5.00`**",
        f"- **Overall Calibrated Score**: **`{report.total_score:.2f} / 10.00`** (target `>= {report.min_score:.1f}`)",
        "",
        "## Score Breakdown by Dimension",
        "",
        "| ID | Dimension | Score | Primary FOSS Reference | Key Findings |",
        "|---|---|---:|---|---|",
    ]
    for d in report.dimensions:
        notes = "; ".join(d.findings) if d.findings else "Clean (`0` defects)"
        lines.append(
            f"| `{d.id}` | **{d.name}** | `{d.earned:.2f} / {d.maximum:.2f}` | {d.style_ref} | {notes} |"
        )

    if report.recommended_work_packages:
        lines.extend(["", "## Dependency-Ordered Class B Refactoring Sequence", ""])
        for idx, wp in enumerate(report.recommended_work_packages, 1):
            lines.append(f"{idx}. {wp}")

    return "\n".join(lines) + "\n"


def run_self_test() -> int:
    with tempfile.TemporaryDirectory(prefix="rust_arch_audit_test_") as tmp:
        root = Path(tmp)

        # 1. C++-transliterated baseline workspace (~6.0/10)
        unhardened = root / "baseline_port"
        (unhardened / "crates" / "core" / "src" / "protocol").mkdir(parents=True)
        (unhardened / "crates" / "core" / "src" / "message").mkdir(parents=True)
        (unhardened / "crates" / "daemon" / "src" / "bin").mkdir(parents=True)
        (unhardened / "docs").mkdir(parents=True)
        (unhardened / "Cargo.toml").write_text(
            '[workspace]\nmembers = ["crates/*"]\n[workspace.lints.clippy]\nunwrap_used = "deny"\nexpect_used = "deny"\nindexing_slicing = "deny"\nundocumented_unsafe_blocks = "deny"\n[profile.release]\npanic = "abort"\noverflow-checks = true\n',
            encoding="utf-8",
        )
        (unhardened / "crates" / "core" / "Cargo.toml").write_text(
            '[package]\nname = "core"\nversion = "0.1.0"\n[dependencies]\ntokio = "1"\n',
            encoding="utf-8",
        )
        (unhardened / "docs" / "ARCHITECTURE.md").write_text(
            "Endpoints use tokio::select!, CancellationToken, and JoinSet.\n",
            encoding="utf-8",
        )
        (unhardened / "crates" / "core" / "src" / "lib.rs").write_text(
            "use std::sync::{Arc, Mutex, Weak};\n"
            + "pub struct Ep { self_w: Mutex<Weak<Ep>> }\n" * 6
            + "pub fn run() { poll_readable(50); let _ = std::thread::Builder::new().spawn(|| {}); }\n"
            + "\n".join(f"pub fn drop_err_{i}() {{ let _ = do_io({i}); }}" for i in range(30)),
            encoding="utf-8",
        )
        (unhardened / "crates" / "core" / "src" / "protocol" / "deserialize.rs").write_text(
            "pub fn deserialize(buf: &[u8]) -> u32 {\n"
            + "\n".join(f"    let p{i} = {i}usize.saturating_add(4);" for i in range(25))
            + "\n    0\n}\n",
            encoding="utf-8",
        )
        (unhardened / "crates" / "core" / "src" / "message" / "payload_impl.rs").write_text(
            "use std::sync::Mutex;\npub struct PayloadImpl { state: Mutex<Vec<u8>> }\n",
            encoding="utf-8",
        )
        (unhardened / "crates" / "daemon" / "src" / "bin" / "rmd.rs").write_text(
            "fn main() { // SAFETY: test\n unsafe { core::hint::spin_loop(); } }\n",
            encoding="utf-8",
        )

        r_base = audit_architecture(unhardened, min_score=8.5)
        assert 5.0 <= r_base.total_score <= 6.8, f"Expected baseline fixture in 5.0..6.8, got {r_base.total_score}: {r_base}"
        assert not r_base.passed

        # 2. Hardened production workspace (>= 9.4/10)
        hardened = root / "hardened_port"
        (hardened / "crates" / "core" / "src" / "sys").mkdir(parents=True)
        (hardened / "crates" / "core" / "src" / "protocol").mkdir(parents=True)
        (hardened / "crates" / "core" / "src" / "message").mkdir(parents=True)
        (hardened / "crates" / "daemon" / "src" / "bin").mkdir(parents=True)
        (hardened / "docs").mkdir(parents=True)
        (hardened / "Cargo.toml").write_text(
            '[workspace]\nmembers = ["crates/*"]\n[workspace.lints.clippy]\nunwrap_used = "deny"\nexpect_used = "deny"\nindexing_slicing = "deny"\nundocumented_unsafe_blocks = "deny"\nlet_underscore_must_use = "deny"\n[dev-dependencies]\nparking_lot = { version = "0.12", features = ["deadlock_detection"] }\n[profile.release]\npanic = "abort"\noverflow-checks = true\n',
            encoding="utf-8",
        )
        (hardened / "crates" / "core" / "Cargo.toml").write_text(
            '[package]\nname = "core"\nversion = "0.1.0"\n[dependencies]\nbytes = "1"\nthiserror = "2"\n',
            encoding="utf-8",
        )
        (hardened / "docs" / "ARCHITECTURE.md").write_text(
            "# Architecture\nUses BoardnetReactor (`poll`) and an 8-tier lock acquisition hierarchy (Tier 1..Tier 8).\n",
            encoding="utf-8",
        )
        (hardened / "crates" / "core" / "src" / "sys" / "mod.rs").write_text(
            "pub fn daemonize() -> Result<(), i32> { // SAFETY: single-threaded\n unsafe { core::hint::spin_loop(); } Ok(()) }\n",
            encoding="utf-8",
        )
        (hardened / "crates" / "core" / "src" / "lib.rs").write_text(
            "#![deny(unsafe_code)]\n#[allow(unsafe_code)]\npub(crate) mod sys;\npub struct BoardnetReactor;\n"
            + "\n".join(f"#[derive(Debug, thiserror::Error)]\npub enum Err{i} {{ #[error(\"e\")] E }}" for i in range(5)),
            encoding="utf-8",
        )
        (hardened / "crates" / "core" / "src" / "protocol" / "deserialize.rs").write_text(
            "#[must_use]\npub fn try_deserialize(buf: &[u8], off: usize) -> Option<usize> { off.checked_add(buf.len()) }\n",
            encoding="utf-8",
        )
        (hardened / "crates" / "core" / "src" / "message" / "payload_impl.rs").write_text(
            "use bytes::Bytes;\npub enum PayloadStorage { Shared(Bytes), Owned(Vec<u8>) }\npub struct PayloadImpl { state: parking_lot::RwLock<PayloadStorage> }\n",
            encoding="utf-8",
        )
        (hardened / "crates" / "daemon" / "src" / "bin" / "rmd.rs").write_text(
            "#![forbid(unsafe_code)]\nfn main() {}\n",
            encoding="utf-8",
        )

        r_hard = audit_architecture(hardened, min_score=8.5)
        assert r_hard.total_score >= 9.4 and r_hard.passed, f"Expected hardened fixture >= 9.4, got {r_hard.total_score}: {r_hard}"

    print(
        f"SELF-TEST PASS: audit_rust_architecture.py calibrated baseline={r_base.total_score:.2f}/10 vs hardened={r_hard.total_score:.2f}/10."
    )
    return 0


def main(argv: Optional[Sequence[str]] = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--workspace", default=".", help="Path to Rust workspace root")
    parser.add_argument("--arch-doc", default=None, help="Optional path to ARCHITECTURE.md")
    parser.add_argument("--min-score", type=float, default=8.5, help="Minimum passing score under --strict (default: 8.5)")
    parser.add_argument("--format", choices=("markdown", "json", "text"), default="markdown")
    parser.add_argument("--output", default=None, help="Optional file path to write report")
    parser.add_argument("--strict", action="store_true", help="Exit 1 if total_score < --min-score")
    parser.add_argument("--self-test", action="store_true", help="Run built-in calibration self-tests")
    args = parser.parse_args(argv)

    if args.self_test:
        return run_self_test()

    report = audit_architecture(Path(args.workspace).resolve(), args.arch_doc, args.min_score)
    if args.format == "json":
        rendered = json.dumps(asdict(report), indent=2) + "\n"
    else:
        rendered = format_markdown(report)

    if args.output:
        Path(args.output).write_text(rendered, encoding="utf-8")
    else:
        sys.stdout.write(rendered)

    if args.strict and not report.passed:
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
