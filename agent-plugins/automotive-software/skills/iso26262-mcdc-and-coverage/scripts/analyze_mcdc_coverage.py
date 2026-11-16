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
"""ISO 26262-6 Clauses 9 & 10 Structural Coverage, MC/DC & Boolean Decision Analyzer.

Capabilities:
1. Parses `llvm-cov export --format=text` JSON (`llvm.coverage.json.export` v3.1.0)
   to evaluate Statement (`lines`), Function (`functions`), Branch (`branches` 9-tuple),
   and MC/DC (`mcdc_records` 11-tuple) coverage against ISO 26262-6 Table 9 and Table 12
   ASIL A-D targets.
2. Pinpoints uncovered branch outcomes and uncovered MC/DC conditions
   (`ConditionsArray[c] == false`), inspects `TestVectorsArray`, and synthesizes
   missing independence-pair test vectors.
3. Resolves ISO 26262-6 Clause 9.4.5 coverage justifications from YAML/JSON
   (`coverage_justifications.yaml`) and in-code `COV_JUSTIFIED` /
   `COV_JUSTIFIED_START`..`COV_JUSTIFIED_STOP` markers to compute Raw vs. Effective
   Coverage and flag Stale Justifications.
4. Statically scans Rust (`.rs`) and C/C++ (`.cpp`, `.cc`, `.c`, `.hpp`, `.h`) source
   files to inventory multi-condition boolean decisions (`&&` / `||`), detect
   decisions exceeding a configurable condition-count cap (default `6`, matching
   LLVM 18's default `--mcdc-max-conditions=6` cap and ISO 26262 complexity hygiene;
   LLVM 19+ raised the internal bitmap limit) or containing coupled variables
   (`x` and `!x`), and synthesize minimal N+1 MC/DC truth-table test vectors.
"""

from __future__ import annotations

import argparse
from dataclasses import dataclass, field
import json
import math
from pathlib import Path
import re
import sys
import tempfile
from typing import Any, Dict, List, Optional, Set, Tuple


# ---------------------------------------------------------------------------
# Constants & ISO 26262-6 Table 9 / Table 12 Thresholds
# ---------------------------------------------------------------------------

ASIL_REQUIREMENTS: Dict[str, Dict[str, str]] = {
    "A": {
        "statement": "++ (Mandatory)",
        "branch": "+ (Recommended)",
        "mcdc": "+ (Recommended)",
        "function": "+ (Recommended)",
    },
    "B": {
        "statement": "++ (Mandatory)",
        "branch": "++ (Mandatory)",
        "mcdc": "+ (Recommended)",
        "function": "+ (Recommended)",
    },
    "C": {
        "statement": "+ (Recommended)",
        "branch": "++ (Mandatory)",
        "mcdc": "+ (Recommended)",
        "function": "++ (Mandatory)",
    },
    "D": {
        "statement": "+ (Recommended)",
        "branch": "++ (Mandatory)",
        "mcdc": "++ (Mandatory)",
        "function": "++ (Mandatory)",
    },
}

MANDATORY_METRICS_BY_ASIL: Dict[str, Set[str]] = {
    "A": {"statement"},
    "B": {"statement", "branch"},
    "C": {"statement", "branch", "function"},
    "D": {"statement", "branch", "function", "mcdc"},
}

COV_JUSTIFIED_LINE_RE = re.compile(r"COV_JUSTIFIED\s+([a-zA-Z0-9_-]+)")
COV_JUSTIFIED_START_RE = re.compile(r"COV_JUSTIFIED_START\s+([a-zA-Z0-9_-]+)")
COV_JUSTIFIED_STOP_RE = re.compile(r"COV_JUSTIFIED_STOP\b")

SOURCE_EXTENSIONS = {".rs", ".cpp", ".cc", ".cxx", ".c", ".hpp", ".h", ".ipp"}


def floor_two_decimals(value: float) -> float:
    """Floor a percentage value to two decimal places (automotive safety middleware convention)."""
    return math.floor(value * 100.0) / 100.0


def pct(covered: int, total: int) -> float:
    """Compute percentage floored to 2 decimals; 100.0 when total == 0."""
    if total <= 0:
        return 100.0
    return floor_two_decimals((100.0 * covered) / total)


# ---------------------------------------------------------------------------
# Pure-Stdlib Minimal YAML / JSON Justification Loader & Marker Scanner
# ---------------------------------------------------------------------------


def parse_simple_yaml(text: str) -> Dict[str, Any]:
    """Parse `coverage_justifications.yaml` using only the Python 3 standard library."""
    stripped = text.lstrip()
    if stripped.startswith("{"):
        return json.loads(text)

    result: Dict[str, Any] = {"version": 1, "justifications": []}
    lines = text.splitlines()
    i = 0
    current_entry: Optional[Dict[str, Any]] = None
    current_loc: Optional[Dict[str, Any]] = None
    in_locations = False
    in_reason_block = False
    reason_indent = 0

    while i < len(lines):
        raw = lines[i]
        # Strip comments unless inside reason block
        line_no_comment = re.sub(r"\s+#.*$", "", raw) if not in_reason_block else raw
        if not line_no_comment.strip() or line_no_comment.lstrip().startswith("#"):
            i += 1
            continue

        indent = len(raw) - len(raw.lstrip(" "))
        s = line_no_comment.strip()

        if in_reason_block:
            if indent >= reason_indent and not s.startswith("- ") and ":" not in s.split()[0]:
                if current_entry is not None:
                    prev = current_entry.get("reason", "")
                    current_entry["reason"] = (prev + " " + s).strip()
                i += 1
                continue
            else:
                in_reason_block = False

        if s.startswith("version:"):
            val = s.split(":", 1)[1].strip()
            if val.isdigit():
                result["version"] = int(val)
            i += 1
            continue

        if s.startswith("justifications:"):
            i += 1
            continue

        # New justification item
        if s.startswith("- id:") or (s.startswith("-") and "id:" in s and indent <= 6):
            current_loc = None
            in_locations = False
            jid = s.split("id:", 1)[1].strip().strip("'\"")
            current_entry = {
                "id": jid,
                "category": "defensive_programming",
                "platforms": ["linux", "qnx"],
                "reason": "",
                "locations": [],
            }
            result["justifications"].append(current_entry)
            i += 1
            continue

        if current_entry is not None:
            if s.startswith("locations:"):
                in_locations = True
                current_loc = None
                i += 1
                continue

            if in_locations and s.startswith("- file:"):
                fpath = s.split("file:", 1)[1].strip().strip("'\"")
                current_loc = {"file": fpath}
                current_entry["locations"].append(current_loc)
                i += 1
                continue

            if in_locations and current_loc is not None:
                if s.startswith("line_start:"):
                    current_loc["line_start"] = int(s.split(":", 1)[1].strip())
                    i += 1
                    continue
                if s.startswith("line_end:"):
                    current_loc["line_end"] = int(s.split(":", 1)[1].strip())
                    i += 1
                    continue
                if s.startswith("line:"):
                    current_loc["line"] = int(s.split(":", 1)[1].strip())
                    i += 1
                    continue
                if s.startswith("lines:"):
                    arr_str = s.split(":", 1)[1].strip().strip("[]")
                    current_loc["lines"] = [
                        int(x.strip()) for x in arr_str.split(",") if x.strip().isdigit()
                    ]
                    i += 1
                    continue

            if s.startswith("category:"):
                in_locations = False
                current_entry["category"] = s.split(":", 1)[1].strip().strip("'\"")
            elif s.startswith("platforms:"):
                in_locations = False
                arr_str = s.split(":", 1)[1].strip().strip("[]")
                current_entry["platforms"] = [
                    x.strip().strip("'\"") for x in arr_str.split(",") if x.strip()
                ]
            elif s.startswith("reason:"):
                in_locations = False
                rval = s.split(":", 1)[1].strip()
                if rval in (">", "|", ">-", "|-", ""):
                    in_reason_block = True
                    reason_indent = indent + 2
                else:
                    current_entry["reason"] = rval.strip("'\"")

        i += 1

    return result


def resolve_justifications(
    justifications_path: Optional[Path],
    source_root: Optional[Path],
) -> Tuple[Dict[str, Dict[int, Dict[str, str]]], List[str]]:
    """Resolve file:line justifications from YAML/JSON and in-code COV_JUSTIFIED markers."""
    resolved: Dict[str, Dict[int, Dict[str, str]]] = {}
    warnings: List[str] = []
    by_id: Dict[str, Dict[str, Any]] = {}

    if justifications_path and justifications_path.exists():
        raw_text = justifications_path.read_text(encoding="utf-8", errors="replace")
        data = parse_simple_yaml(raw_text)
        for entry in data.get("justifications", []):
            jid = entry.get("id", "")
            if not jid:
                continue
            by_id[jid] = entry
            for loc in entry.get("locations", []):
                fpath = loc.get("file", "")
                if not fpath:
                    continue
                lines: List[int] = []
                if "lines" in loc:
                    lines = list(loc["lines"])
                elif "line_start" in loc and "line_end" in loc:
                    lines = list(range(int(loc["line_start"]), int(loc["line_end"]) + 1))
                elif "line" in loc:
                    lines = [int(loc["line"])]
                resolved.setdefault(fpath, {})
                for ln in lines:
                    resolved[fpath][ln] = {
                        "id": jid,
                        "category": entry.get("category", "defensive_programming"),
                        "reason": entry.get("reason", "").strip(),
                    }

    if source_root and source_root.exists():
        files_to_scan: List[Path] = []
        if source_root.is_file():
            files_to_scan = [source_root]
            base_dir = source_root.parent
        else:
            base_dir = source_root
            for p in sorted(source_root.rglob("*")):
                if p.is_file() and p.suffix in SOURCE_EXTENSIONS:
                    files_to_scan.append(p)

        for src_file in files_to_scan:
            rel_path = str(src_file.relative_to(base_dir))
            try:
                src_lines = src_file.read_text(encoding="utf-8", errors="replace").splitlines()
            except OSError:
                continue

            stack: List[Tuple[int, str]] = []
            for ln_idx, line_text in enumerate(src_lines, start=1):
                m_start = COV_JUSTIFIED_START_RE.search(line_text)
                if m_start:
                    jid = m_start.group(1)
                    if by_id and jid not in by_id:
                        warnings.append(
                            f"{rel_path}:{ln_idx}: COV_JUSTIFIED_START references unknown ID '{jid}'"
                        )
                    stack.append((ln_idx, jid))
                    continue

                if COV_JUSTIFIED_STOP_RE.search(line_text):
                    if not stack:
                        warnings.append(
                            f"{rel_path}:{ln_idx}: COV_JUSTIFIED_STOP without matching START"
                        )
                    else:
                        start_ln, jid = stack.pop()
                        entry = by_id.get(
                            jid,
                            {
                                "id": jid,
                                "category": "defensive_programming",
                                "reason": "In-code COV_JUSTIFIED_START/STOP marker",
                            },
                        )
                        resolved.setdefault(rel_path, {})
                        for covered_ln in range(start_ln + 1, ln_idx):
                            resolved[rel_path][covered_ln] = {
                                "id": jid,
                                "category": entry.get("category", "defensive_programming"),
                                "reason": entry.get("reason", "").strip(),
                            }
                    continue

                m_line = COV_JUSTIFIED_LINE_RE.search(line_text)
                if m_line:
                    jid = m_line.group(1)
                    if by_id and jid not in by_id:
                        warnings.append(
                            f"{rel_path}:{ln_idx}: COV_JUSTIFIED references unknown ID '{jid}'"
                        )
                    entry = by_id.get(
                        jid,
                        {
                            "id": jid,
                            "category": "defensive_programming",
                            "reason": "In-code COV_JUSTIFIED marker",
                        },
                    )
                    resolved.setdefault(rel_path, {})
                    resolved[rel_path][ln_idx] = {
                        "id": jid,
                        "category": entry.get("category", "defensive_programming"),
                        "reason": entry.get("reason", "").strip(),
                    }

            for start_ln, jid in stack:
                warnings.append(
                    f"{rel_path}:{start_ln}: unclosed COV_JUSTIFIED_START '{jid}'"
                )

    return resolved, warnings


def match_file_justifications(
    filename: str, justified_files: Dict[str, Dict[int, Dict[str, str]]]
) -> Dict[int, Dict[str, str]]:
    """Find line justifications matching `filename` (exact or suffix match)."""
    if filename in justified_files:
        return justified_files[filename]
    for jfile, lmap in justified_files.items():
        if filename.endswith(jfile) or jfile.endswith(filename):
            return lmap
    return {}


# ---------------------------------------------------------------------------
# Part 1: LLVM `llvm.coverage.json.export` v3.1.0 Parser & MC/DC Gap Analyzer
# ---------------------------------------------------------------------------


def format_cond_state(val: Optional[bool]) -> str:
    """Format an LLVM CondState (`true`, `false`, `null` -> `T`, `F`, `-`)."""
    if val is True:
        return "T"
    if val is False:
        return "F"
    return "-"


def format_vector(conds: List[Optional[bool]], result: Optional[bool]) -> str:
    """Format a test vector as `(T, F, -) -> T`."""
    inner = ", ".join(format_cond_state(c) for c in conds)
    res_str = format_cond_state(result)
    return f"({inner}) -> {res_str}"


def is_independence_pair(
    vec_a: Dict[str, Any], vec_b: Dict[str, Any], cond_idx: int
) -> bool:
    """Check if two test vectors form a valid MC/DC independence pair for `cond_idx`."""
    conds_a = vec_a.get("conditions", [])
    conds_b = vec_b.get("conditions", [])
    if cond_idx >= len(conds_a) or cond_idx >= len(conds_b):
        return False

    # Target condition must be evaluated in both vectors and have opposite truth values
    ca = conds_a[cond_idx]
    cb = conds_b[cond_idx]
    if ca is None or cb is None or ca == cb:
        return False

    # Decision result must flip
    res_a = vec_a.get("result")
    res_b = vec_b.get("result")
    if res_a is None or res_b is None or res_a == res_b:
        return False

    # Every other condition j != cond_idx must either be held equal or be short-circuited (None)
    for j in range(len(conds_a)):
        if j == cond_idx:
            continue
        cja = conds_a[j]
        cjb = conds_b[j]
        if cja is not None and cjb is not None and cja != cjb:
            return False

    return True


def synthesize_missing_pair_for_condition(
    test_vectors: List[Dict[str, Any]], cond_idx: int
) -> Dict[str, str]:
    """Identify or synthesize a missing independence-pair test vector for `cond_idx`."""
    executed = [tv for tv in test_vectors if tv.get("executed", False)]
    not_executed = [tv for tv in test_vectors if not tv.get("executed", False)]

    # 1. Check if an executed vector pairs with a non-executed vector in TestVectorsArray
    for ex in executed:
        for nex in not_executed:
            if is_independence_pair(ex, nex, cond_idx):
                return {
                    "base_executed_vector": format_vector(
                        ex.get("conditions", []), ex.get("result")
                    ),
                    "required_new_vector": format_vector(
                        nex.get("conditions", []), nex.get("result")
                    ),
                    "derivation": "Matched unexecuted vector from LLVM TestVectorsArray",
                }

    # 2. Otherwise synthesize directly from the closest executed vector that evaluated C[cond_idx]
    for ex in executed:
        conds = list(ex.get("conditions", []))
        res = ex.get("result")
        if cond_idx < len(conds) and conds[cond_idx] is not None and res is not None:
            new_conds = list(conds)
            new_conds[cond_idx] = not conds[cond_idx]
            # Any condition after cond_idx that was None (short-circuited) may need evaluation
            return {
                "base_executed_vector": format_vector(conds, res),
                "required_new_vector": format_vector(new_conds, not res),
                "derivation": (
                    f"Toggle C{cond_idx + 1} from {format_cond_state(conds[cond_idx])} "
                    f"to {format_cond_state(new_conds[cond_idx])} while holding evaluated "
                    f"peers fixed to flip decision outcome to {format_cond_state(not res)}"
                ),
            }

    return {
        "base_executed_vector": "None (condition never reached)",
        "required_new_vector": f"Reach decision and toggle C{cond_idx + 1} between T and F",
        "derivation": "Condition was short-circuited in all executed vectors",
    }


def analyze_llvm_coverage_json(
    llvm_json_path: Path,
    asil: str = "D",
    justified_files: Optional[Dict[str, Dict[int, Dict[str, str]]]] = None,
) -> Dict[str, Any]:
    """Parse `llvm-cov export` v3.1.0 JSON and compute Raw & Effective coverage + gaps."""
    justified_files = justified_files or {}
    payload = json.loads(llvm_json_path.read_text(encoding="utf-8"))

    export_type = payload.get("type", "")
    export_version = payload.get("version", "")
    data_entries = payload.get("data", [])

    uncovered_branches: List[Dict[str, Any]] = []
    uncovered_mcdc_decisions: List[Dict[str, Any]] = []
    applied_justifications: List[Dict[str, Any]] = []
    stale_justifications: List[Dict[str, Any]] = []
    per_file_reports: List[Dict[str, Any]] = []

    raw_lines_total = 0
    raw_lines_covered = 0
    justified_lines_count = 0

    raw_funcs_total = 0
    raw_funcs_covered = 0
    justified_funcs_count = 0

    raw_branches_total = 0
    raw_branches_covered = 0
    justified_branches_count = 0

    raw_mcdc_total = 0
    raw_mcdc_covered = 0
    justified_mcdc_count = 0

    for export_obj in data_entries:
        files_list = export_obj.get("files", [])
        for fobj in files_list:
            filename = fobj.get("filename", "<unknown>")
            file_justs = match_file_justifications(filename, justified_files)
            summary = fobj.get("summary", {})

            f_lines_total = int(summary.get("lines", {}).get("count", 0))
            f_lines_cov = int(summary.get("lines", {}).get("covered", 0))
            f_funcs_total = int(summary.get("functions", {}).get("count", 0))
            f_funcs_cov = int(summary.get("functions", {}).get("covered", 0))
            f_br_total = int(summary.get("branches", {}).get("count", 0))
            f_br_cov = int(summary.get("branches", {}).get("covered", 0))
            f_mcdc_total = int(summary.get("mcdc", {}).get("count", 0))
            f_mcdc_cov = int(summary.get("mcdc", {}).get("covered", 0))

            # Track line-level coverage from segments (6-tuple: [Line, Col, Count, HasCount, IsEntry, IsGap])
            line_counts: Dict[int, int] = {}
            for seg in fobj.get("segments", []):
                if len(seg) >= 4 and seg[3]:  # HasCount == true
                    ln = int(seg[0])
                    cnt = int(seg[2])
                    line_counts[ln] = max(line_counts.get(ln, 0), cnt)

            lines_with_uncovered_branch: Set[int] = set()
            lines_with_uncovered_mcdc: Set[int] = set()

            # Inspect 9-tuple branches:
            # [LineStart, ColStart, LineEnd, ColEnd, TrueCount, FalseCount, FileID, ExpFileID, Kind]
            f_just_branches = 0
            for br in fobj.get("branches", []):
                if len(br) < 6:
                    continue
                l_start, c_start, l_end, c_end = int(br[0]), int(br[1]), int(br[2]), int(br[3])
                true_cnt, false_cnt = int(br[4]), int(br[5])
                missing_dirs: List[str] = []
                if true_cnt == 0:
                    missing_dirs.append("True")
                if false_cnt == 0:
                    missing_dirs.append("False")

                if missing_dirs:
                    lines_with_uncovered_branch.add(l_start)
                    jinfo = file_justs.get(l_start)
                    if jinfo:
                        f_just_branches += len(missing_dirs)
                        applied_justifications.append(
                            {
                                "file": filename,
                                "line": l_start,
                                "metric": f"branch ({','.join(missing_dirs)})",
                                "id": jinfo["id"],
                                "category": jinfo["category"],
                                "reason": jinfo["reason"],
                            }
                        )
                    else:
                        uncovered_branches.append(
                            {
                                "file": filename,
                                "line_start": l_start,
                                "col_start": c_start,
                                "line_end": l_end,
                                "col_end": c_end,
                                "true_count": true_cnt,
                                "false_count": false_cnt,
                                "missing_directions": missing_dirs,
                            }
                        )

            # Inspect 11-tuple mcdc_records:
            # [LineStart, ColStart, LineEnd, ColEnd, TrueDec, FalseDec, FileID, ExpFileID, Kind, ConditionsArray, TestVectorsArray]
            f_just_mcdc = 0
            for rec in fobj.get("mcdc_records", []):
                if len(rec) < 11:
                    continue
                l_start, c_start, l_end, c_end = (
                    int(rec[0]),
                    int(rec[1]),
                    int(rec[2]),
                    int(rec[3]),
                )
                true_dec, false_dec = int(rec[4]), int(rec[5])
                conds_array: List[bool] = list(rec[9])
                tv_array: List[Dict[str, Any]] = list(rec[10])

                uncovered_cond_indices = [
                    idx for idx, is_cov in enumerate(conds_array) if not is_cov
                ]
                if uncovered_cond_indices:
                    lines_with_uncovered_mcdc.add(l_start)
                    # Check if any line in the decision range [l_start..l_end] is justified
                    jinfo = None
                    for ln in range(l_start, l_end + 1):
                        if ln in file_justs:
                            jinfo = file_justs[ln]
                            break

                    if jinfo:
                        f_just_mcdc += len(uncovered_cond_indices)
                        applied_justifications.append(
                            {
                                "file": filename,
                                "line": l_start,
                                "metric": (
                                    "mcdc_conditions ("
                                    + ", ".join(f"C{i + 1}" for i in uncovered_cond_indices)
                                    + ")"
                                ),
                                "id": jinfo["id"],
                                "category": jinfo["category"],
                                "reason": jinfo["reason"],
                            }
                        )
                    else:
                        synthesized = {
                            f"C{idx + 1}": synthesize_missing_pair_for_condition(tv_array, idx)
                            for idx in uncovered_cond_indices
                        }
                        executed_vectors = [
                            format_vector(tv.get("conditions", []), tv.get("result"))
                            for tv in tv_array
                            if tv.get("executed", False)
                        ]
                        uncovered_mcdc_decisions.append(
                            {
                                "file": filename,
                                "line_start": l_start,
                                "col_start": c_start,
                                "line_end": l_end,
                                "col_end": c_end,
                                "num_conditions": len(conds_array),
                                "true_decisions": true_dec,
                                "false_decisions": false_dec,
                                "conditions_covered": conds_array,
                                "uncovered_conditions": [
                                    f"C{idx + 1}" for idx in uncovered_cond_indices
                                ],
                                "executed_vectors": executed_vectors,
                                "synthesized_missing_pairs": synthesized,
                            }
                        )

            # Evaluate line-level justifications & stale justifications (REQ-COV-005)
            f_just_lines = 0
            for jline, jinfo in sorted(file_justs.items()):
                cnt = line_counts.get(jline)
                if cnt == 0:
                    f_just_lines += 1
                    applied_justifications.append(
                        {
                            "file": filename,
                            "line": jline,
                            "metric": "statement/line",
                            "id": jinfo["id"],
                            "category": jinfo["category"],
                            "reason": jinfo["reason"],
                        }
                    )
                elif cnt is not None and cnt > 0:
                    # Stale iff line is covered AND has no uncovered branches or MC/DC conditions
                    if (
                        jline not in lines_with_uncovered_branch
                        and jline not in lines_with_uncovered_mcdc
                    ):
                        stale_justifications.append(
                            {
                                "file": filename,
                                "line": jline,
                                "id": jinfo["id"],
                                "reason": (
                                    "Line, branches, and MC/DC conditions on this line "
                                    "are 100% covered by tests; justification is stale."
                                ),
                            }
                        )

            # Cap justified counts at uncovered counts so effective coverage never exceeds 100%
            f_just_lines = min(f_just_lines, max(0, f_lines_total - f_lines_cov))
            f_just_branches = min(f_just_branches, max(0, f_br_total - f_br_cov))
            f_just_mcdc = min(f_just_mcdc, max(0, f_mcdc_total - f_mcdc_cov))

            raw_lines_total += f_lines_total
            raw_lines_covered += f_lines_cov
            justified_lines_count += f_just_lines

            raw_funcs_total += f_funcs_total
            raw_funcs_covered += f_funcs_cov

            raw_branches_total += f_br_total
            raw_branches_covered += f_br_cov
            justified_branches_count += f_just_branches

            raw_mcdc_total += f_mcdc_total
            raw_mcdc_covered += f_mcdc_cov
            justified_mcdc_count += f_just_mcdc

            per_file_reports.append(
                {
                    "filename": filename,
                    "lines": {
                        "total": f_lines_total,
                        "raw_covered": f_lines_cov,
                        "justified": f_just_lines,
                        "raw_pct": pct(f_lines_cov, f_lines_total),
                        "effective_pct": pct(f_lines_cov + f_just_lines, f_lines_total),
                    },
                    "branches": {
                        "total": f_br_total,
                        "raw_covered": f_br_cov,
                        "justified": f_just_branches,
                        "raw_pct": pct(f_br_cov, f_br_total),
                        "effective_pct": pct(f_br_cov + f_just_branches, f_br_total),
                    },
                    "mcdc": {
                        "total": f_mcdc_total,
                        "raw_covered": f_mcdc_cov,
                        "justified": f_just_mcdc,
                        "raw_pct": pct(f_mcdc_cov, f_mcdc_total),
                        "effective_pct": pct(f_mcdc_cov + f_just_mcdc, f_mcdc_total),
                    },
                    "functions": {
                        "total": f_funcs_total,
                        "raw_covered": f_funcs_cov,
                        "raw_pct": pct(f_funcs_cov, f_funcs_total),
                    },
                }
            )

    summary_metrics = {
        "statement": {
            "total": raw_lines_total,
            "raw_covered": raw_lines_covered,
            "justified": justified_lines_count,
            "unjustified_uncovered": max(
                0, raw_lines_total - raw_lines_covered - justified_lines_count
            ),
            "raw_pct": pct(raw_lines_covered, raw_lines_total),
            "effective_pct": pct(raw_lines_covered + justified_lines_count, raw_lines_total),
        },
        "branch": {
            "total": raw_branches_total,
            "raw_covered": raw_branches_covered,
            "justified": justified_branches_count,
            "unjustified_uncovered": max(
                0, raw_branches_total - raw_branches_covered - justified_branches_count
            ),
            "raw_pct": pct(raw_branches_covered, raw_branches_total),
            "effective_pct": pct(
                raw_branches_covered + justified_branches_count, raw_branches_total
            ),
        },
        "mcdc": {
            "total": raw_mcdc_total,
            "raw_covered": raw_mcdc_covered,
            "justified": justified_mcdc_count,
            "unjustified_uncovered": max(
                0, raw_mcdc_total - raw_mcdc_covered - justified_mcdc_count
            ),
            "raw_pct": pct(raw_mcdc_covered, raw_mcdc_total),
            "effective_pct": pct(raw_mcdc_covered + justified_mcdc_count, raw_mcdc_total),
        },
        "function": {
            "total": raw_funcs_total,
            "raw_covered": raw_funcs_covered,
            "justified": justified_funcs_count,
            "unjustified_uncovered": max(
                0, raw_funcs_total - raw_funcs_covered - justified_funcs_count
            ),
            "raw_pct": pct(raw_funcs_covered, raw_funcs_total),
            "effective_pct": pct(raw_funcs_covered + justified_funcs_count, raw_funcs_total),
        },
    }

    mandatory_set = MANDATORY_METRICS_BY_ASIL.get(asil.upper(), MANDATORY_METRICS_BY_ASIL["D"])
    asil_compliant = all(
        summary_metrics[m]["unjustified_uncovered"] == 0 for m in mandatory_set
    )

    return {
        "schema_type": export_type,
        "schema_version": export_version,
        "asil": asil.upper(),
        "asil_compliant": asil_compliant,
        "summary": summary_metrics,
        "per_file": per_file_reports,
        "uncovered_branches": uncovered_branches,
        "uncovered_mcdc_decisions": uncovered_mcdc_decisions,
        "applied_justifications": applied_justifications,
        "stale_justifications": stale_justifications,
    }


# ---------------------------------------------------------------------------
# Part 2: Static Boolean Decision Scanner, Coupled-Variable Detector &
#         Short-Circuit MC/DC Test-Vector Synthesizer
# ---------------------------------------------------------------------------


@dataclass
class BoolNode:
    """AST node for a boolean decision expression (`And`, `Or`, `Not`, `Atom`)."""

    kind: str  # "and", "or", "not", "atom"
    children: List["BoolNode"] = field(default_factory=list)
    atom_index: Optional[int] = None
    atom_text: str = ""


def strip_comments_preserve_lines(source: str) -> str:
    """Replace comments and string literals with spaces while preserving line breaks."""
    out: List[str] = []
    i = 0
    n = len(source)
    while i < n:
        ch = source[i]
        # Line comment //
        if ch == "/" and i + 1 < n and source[i + 1] == "/":
            while i < n and source[i] != "\n":
                out.append(" ")
                i += 1
            continue
        # Block comment /* ... */
        if ch == "/" and i + 1 < n and source[i + 1] == "*":
            out.append("  ")
            i += 2
            while i < n and not (source[i] == "*" and i + 1 < n and source[i + 1] == "/"):
                out.append("\n" if source[i] == "\n" else " ")
                i += 1
            if i < n:
                out.append("  ")
                i += 2
            continue
        # Double-quoted string literal
        if ch == '"':
            out.append('"')
            i += 1
            while i < n and source[i] != '"':
                if source[i] == "\\" and i + 1 < n:
                    out.append("  ")
                    i += 2
                    continue
                out.append("\n" if source[i] == "\n" else " ")
                i += 1
            if i < n:
                out.append('"')
                i += 1
            continue
        out.append(ch)
        i += 1
    return "".join(out)


def split_top_level_op(expr: str, op: str) -> List[str]:
    """Split `expr` by top-level boolean operator `op` (`||` or `&&`)."""
    parts: List[str] = []
    depth_paren = 0
    depth_bracket = 0
    i = 0
    n = len(expr)
    last = 0
    op_len = len(op)

    while i < n:
        ch = expr[i]
        if ch == "(":
            depth_paren += 1
        elif ch == ")":
            depth_paren = max(0, depth_paren - 1)
        elif ch == "[":
            depth_bracket += 1
        elif ch == "]":
            depth_bracket = max(0, depth_bracket - 1)
        elif (
            depth_paren == 0
            and depth_bracket == 0
            and expr[i : i + op_len] == op
        ):
            parts.append(expr[last:i].strip())
            i += op_len
            last = i
            continue
        i += 1

    if last <= n:
        tail = expr[last:].strip()
        if tail:
            parts.append(tail)
    return parts if len(parts) > 1 else [expr.strip()]


def unwrap_outer_parens(expr: str) -> str:
    """Remove balanced outer parentheses wrapping the entire expression."""
    s = expr.strip()
    while s.startswith("(") and s.endswith(")"):
        depth = 0
        balanced_outer = True
        for idx, ch in enumerate(s):
            if ch == "(":
                depth += 1
            elif ch == ")":
                depth -= 1
                if depth == 0 and idx < len(s) - 1:
                    balanced_outer = False
                    break
        if balanced_outer and depth == 0:
            s = s[1:-1].strip()
        else:
            break
    return s


def parse_bool_ast(expr: str, atoms: List[str]) -> BoolNode:
    """Parse a C++/Rust boolean expression into a `BoolNode` AST."""
    s = unwrap_outer_parens(expr)

    # 1. Lowest precedence: top-level `||`
    or_parts = split_top_level_op(s, "||")
    if len(or_parts) > 1:
        return BoolNode(
            kind="or",
            children=[parse_bool_ast(p, atoms) for p in or_parts],
        )

    # 2. Next precedence: top-level `&&`
    and_parts = split_top_level_op(s, "&&")
    if len(and_parts) > 1:
        return BoolNode(
            kind="and",
            children=[parse_bool_ast(p, atoms) for p in and_parts],
        )

    # 3. Leading unary negation `!` where the negated sub-expression is parenthesized
    if s.startswith("!"):
        inner = s[1:].strip()
        if inner.startswith("(") and unwrap_outer_parens(inner) != inner:
            unwrapped = unwrap_outer_parens(inner)
            if "&&" in unwrapped or "||" in unwrapped:
                return BoolNode(
                    kind="not",
                    children=[parse_bool_ast(unwrapped, atoms)],
                )

    # 4. Atomic condition leaf
    idx = len(atoms)
    atoms.append(re.sub(r"\s+", " ", s))
    return BoolNode(kind="atom", atom_index=idx, atom_text=atoms[idx])


def eval_bool_ast_short_circuit(
    node: BoolNode, assignment: Tuple[bool, ...]
) -> Tuple[bool, List[Optional[bool]]]:
    """Evaluate `node` under `assignment` with left-to-right short-circuiting.

    Returns `(decision_result, evaluated_conditions)` where unevaluated conditions
    are `None` (matching `MCDC_DontCare` / `null` in `CoverageExporterJson.cpp:133`).
    """
    observed: List[Optional[bool]] = [None] * len(assignment)

    def _eval(n: BoolNode) -> bool:
        if n.kind == "atom":
            assert n.atom_index is not None
            val = assignment[n.atom_index]
            observed[n.atom_index] = val
            return val
        if n.kind == "not":
            return not _eval(n.children[0])
        if n.kind == "and":
            for child in n.children:
                if not _eval(child):
                    return False
            return True
        if n.kind == "or":
            for child in n.children:
                if _eval(child):
                    return True
            return False
        raise ValueError(f"Unknown BoolNode kind: {n.kind}")

    result = _eval(node)
    return result, observed


def normalize_atom_variable(atom: str) -> Tuple[str, bool]:
    """Normalize an atomic condition to `(canonical_expr, is_negated)` to detect coupling."""
    s = unwrap_outer_parens(atom.strip())
    negated = False
    while s.startswith("!"):
        negated = not negated
        s = unwrap_outer_parens(s[1:].strip())

    # Canonicalize `x != y` vs `x == y`
    if "!=" in s and "==" not in s:
        parts = [p.strip() for p in s.split("!=", 1)]
        if len(parts) == 2 and parts[1] not in ("ILLEGAL_PORT", "ANY_MAJOR", "ANY_MINOR", "ANY_INSTANCE", "0", "nullptr", "None"):
            s = f"{parts[0]} == {parts[1]}"
            negated = not negated

    canonical = re.sub(r"\s+", "", s)
    return canonical, negated


def detect_coupled_conditions(atoms: List[str]) -> List[Dict[str, Any]]:
    """Detect duplicate or complementary (`x` and `!x`) conditions in the same decision."""
    by_var: Dict[str, List[Tuple[int, str, bool]]] = {}
    for idx, atom in enumerate(atoms):
        canon, is_neg = normalize_atom_variable(atom)
        by_var.setdefault(canon, []).append((idx, atom, is_neg))

    coupled: List[Dict[str, Any]] = []
    for canon, occurrences in by_var.items():
        if len(occurrences) > 1:
            cond_labels = [f"C{idx + 1} (`{txt}`)" for idx, txt, _ in occurrences]
            neg_states = {neg for _, _, neg in occurrences}
            relation = (
                "complementary (`x` and `!x`)"
                if len(neg_states) > 1
                else "duplicate (`x` repeated)"
            )
            coupled.append(
                {
                    "canonical_predicate": canon,
                    "relation": relation,
                    "conditions": cond_labels,
                }
            )
    return coupled


def synthesize_mcdc_vectors_for_ast(
    root: BoolNode, num_conditions: int
) -> Dict[str, Any]:
    """Compute truth-table MC/DC independence pairs and minimal N+1 test vector set."""
    if num_conditions > 10:
        return {
            "evaluatable": False,
            "reason": (
                f"Decision has {num_conditions} conditions (> 10); refactor into smaller "
                f"functions or pattern matching before truth-table synthesis."
            ),
            "per_condition_pairs": {},
            "unreachable_conditions": [],
            "minimal_test_vectors": [],
        }

    # Evaluate all 2^N truth-table assignments ( deduplicated by short-circuit signature )
    unique_vectors: Dict[Tuple[Optional[bool], ...], Dict[str, Any]] = {}
    for mask in range(1 << num_conditions):
        assignment = tuple(bool((mask >> (num_conditions - 1 - b)) & 1) for b in range(num_conditions))
        res, observed = eval_bool_ast_short_circuit(root, assignment)
        sig = tuple(observed)
        if sig not in unique_vectors:
            unique_vectors[sig] = {
                "conditions": observed,
                "result": res,
                "executed": True,
            }

    vec_list = list(unique_vectors.values())
    pairs_by_cond: Dict[int, List[Tuple[int, int]]] = {c: [] for c in range(num_conditions)}

    for i in range(len(vec_list)):
        for j in range(i + 1, len(vec_list)):
            for c in range(num_conditions):
                if is_independence_pair(vec_list[i], vec_list[j], c):
                    pairs_by_cond[c].append((i, j))

    # Greedy selection of minimal vector subset (target N + 1 vectors)
    selected_indices: Set[int] = set()
    chosen_pair_for_cond: Dict[str, Dict[str, str]] = {}
    unreachable_conds: List[str] = []

    # Sort conditions by fewest available pairs first so constrained pairs anchor the set
    cond_order = sorted(range(num_conditions), key=lambda c: len(pairs_by_cond[c]))
    for c in cond_order:
        candidates = pairs_by_cond[c]
        if not candidates:
            unreachable_conds.append(f"C{c + 1}")
            continue
        # Pick pair maximizing overlap with already-selected vectors
        best_pair = max(
            candidates,
            key=lambda p: (int(p[0] in selected_indices) + int(p[1] in selected_indices)),
        )
        selected_indices.add(best_pair[0])
        selected_indices.add(best_pair[1])
        va = vec_list[best_pair[0]]
        vb = vec_list[best_pair[1]]
        chosen_pair_for_cond[f"C{c + 1}"] = {
            "vector_a": format_vector(va["conditions"], va["result"]),
            "vector_b": format_vector(vb["conditions"], vb["result"]),
        }

    minimal_vectors = [
        format_vector(vec_list[idx]["conditions"], vec_list[idx]["result"])
        for idx in sorted(selected_indices)
    ]

    return {
        "evaluatable": True,
        "total_reachable_short_circuit_paths": len(vec_list),
        "per_condition_pairs": chosen_pair_for_cond,
        "unreachable_conditions": unreachable_conds,
        "minimal_test_vectors": minimal_vectors,
    }


def extract_boolean_decisions_from_file(
    file_path: Path, rel_path: str, max_conditions: int = 6
) -> List[Dict[str, Any]]:
    """Scan a Rust or C/C++ file for multi-condition boolean decisions (`&&` / `||`)."""
    raw_text = file_path.read_text(encoding="utf-8", errors="replace")
    cleaned = strip_comments_preserve_lines(raw_text)

    # Precompute line start offsets
    line_starts = [0]
    for idx, ch in enumerate(cleaned):
        if ch == "\n":
            line_starts.append(idx + 1)

    def offset_to_line(offset: int) -> int:
        lo, hi = 0, len(line_starts) - 1
        while lo <= hi:
            mid = (lo + hi) // 2
            if line_starts[mid] <= offset:
                lo = mid + 1
            else:
                hi = mid - 1
        return hi + 1

    decisions: List[Dict[str, Any]] = []
    is_rust = file_path.suffix == ".rs"

    # Match `if` / `else if` / `while` headers
    kw_pattern = re.compile(r"\b(?:if|while)\b")
    for m in kw_pattern.finditer(cleaned):
        start_kw = m.start()
        pos = m.end()
        n = len(cleaned)
        while pos < n and cleaned[pos].isspace():
            pos += 1

        # Skip `if let` / `while let` pattern bindings unless they have `&&` guards
        if is_rust and cleaned[pos : pos + 4].startswith("let "):
            continue

        if is_rust:
            # Rust `if <expr> {` (may start with `(` for grouped sub-expressions like `if (a && b) || c {`)
            depth_paren = 0
            i = pos
            while i < n:
                if cleaned[i] == "(":
                    depth_paren += 1
                elif cleaned[i] == ")":
                    depth_paren = max(0, depth_paren - 1)
                elif cleaned[i] == "{" and depth_paren == 0:
                    break
                elif cleaned[i] == ";" and depth_paren == 0:
                    break
                i += 1
            cond_expr = unwrap_outer_parens(cleaned[pos:i].strip())
            end_pos = i
        elif pos < n and cleaned[pos] == "(":
            # C/C++ condition: read balanced outer parentheses `if (...)`
            depth = 0
            i = pos
            while i < n:
                if cleaned[i] == "(":
                    depth += 1
                elif cleaned[i] == ")":
                    depth -= 1
                    if depth == 0:
                        break
                i += 1
            cond_expr = cleaned[pos + 1 : i].strip()
            end_pos = i
        else:
            continue

        # Must contain at least one logical `&&` or `||` (and not be a C++11 initializer with `;`)
        if ("&&" not in cond_expr and "||" not in cond_expr) or ";" in cond_expr:
            continue

        atoms: List[str] = []
        ast_root = parse_bool_ast(cond_expr, atoms)
        if len(atoms) < 2:
            continue

        line_start = offset_to_line(start_kw)
        line_end = offset_to_line(end_pos)
        coupled = detect_coupled_conditions(atoms)
        exceeds_llvm_limit = len(atoms) > max_conditions
        synthesis = synthesize_mcdc_vectors_for_ast(ast_root, len(atoms))

        decisions.append(
            {
                "file": rel_path,
                "line_start": line_start,
                "line_end": line_end,
                "expression": re.sub(r"\s+", " ", cond_expr),
                "num_conditions": len(atoms),
                "conditions": [f"C{i + 1}: {a}" for i, a in enumerate(atoms)],
                "exceeds_llvm_default_limit": exceeds_llvm_limit,
                "coupled_variables": coupled,
                "mcdc_synthesis": synthesis,
            }
        )

    return decisions


def scan_source_path(
    target_path: Path, max_conditions: int = 6
) -> Dict[str, Any]:
    """Scan a file or directory for multi-condition boolean decisions & MC/DC hazards."""
    files: List[Path] = []
    if target_path.is_file():
        files = [target_path]
        base_dir = target_path.parent
    else:
        base_dir = target_path
        for p in sorted(target_path.rglob("*")):
            if p.is_file() and p.suffix in SOURCE_EXTENSIONS:
                files.append(p)

    all_decisions: List[Dict[str, Any]] = []
    for fpath in files:
        rel = str(fpath.relative_to(base_dir))
        all_decisions.extend(
            extract_boolean_decisions_from_file(fpath, rel, max_conditions=max_conditions)
        )

    exceeding_limit = [d for d in all_decisions if d["exceeds_llvm_default_limit"]]
    with_coupled = [d for d in all_decisions if d["coupled_variables"]]
    with_unreachable = [
        d
        for d in all_decisions
        if d["mcdc_synthesis"].get("unreachable_conditions")
    ]

    return {
        "scanned_path": str(target_path),
        "files_scanned": len(files),
        "max_conditions_limit": max_conditions,
        "total_multi_condition_decisions": len(all_decisions),
        "decisions_exceeding_limit_count": len(exceeding_limit),
        "decisions_with_coupled_variables_count": len(with_coupled),
        "decisions_with_unreachable_mcdc_count": len(with_unreachable),
        "decisions": all_decisions,
    }


# ---------------------------------------------------------------------------
# Report Formatting (Markdown / JSON / Text)
# ---------------------------------------------------------------------------


def format_report_markdown(
    llvm_report: Optional[Dict[str, Any]],
    source_report: Optional[Dict[str, Any]],
    justification_warnings: List[str],
) -> str:
    """Render a comprehensive Markdown report for ISO 26262-6 Clauses 9 & 10."""
    lines: List[str] = [
        "# ISO 26262-6 Structural Coverage & MC/DC Analysis Report",
        "",
    ]

    if llvm_report is not None:
        asil = llvm_report["asil"]
        status_str = (
            "PASS (100% Effective Coverage on Mandatory ASIL Metrics)"
            if llvm_report["asil_compliant"]
            else "GAP DETECTED (Unjustified Coverage Gaps Present)"
        )
        lines.extend(
            [
                f"## 1. LLVM Source-Based Coverage Summary (`ASIL-{asil}` — `{llvm_report['schema_type']}` v`{llvm_report['schema_version']}`)",
                "",
                f"- **Overall ASIL-{asil} Gate Status**: **{status_str}**",
                "",
                "| Metric (ISO 26262-6 Table 9 / 12) | ASIL Requirement | Total | Raw Covered | Justified (`COV_JUSTIFIED`) | Unjustified Gaps | Raw % | Effective % |",
                "|---|---|---:|---:|---:|---:|---:|---:|",
            ]
        )
        req_map = ASIL_REQUIREMENTS.get(asil, ASIL_REQUIREMENTS["D"])
        for metric_key, label in (
            ("statement", "Statement Coverage (Table 9 `1a`)"),
            ("branch", "Branch Coverage (Table 9 `1b`)"),
            ("mcdc", "MC/DC Coverage (Table 9 `1c`)"),
            ("function", "Function Coverage (Table 12 `1a`)"),
        ):
            m = llvm_report["summary"][metric_key]
            lines.append(
                f"| {label} | {req_map[metric_key]} | {m['total']} | {m['raw_covered']} | "
                f"{m['justified']} | {m['unjustified_uncovered']} | "
                f"{m['raw_pct']:.2f}% | **{m['effective_pct']:.2f}%** |"
            )
        lines.append("")

        if llvm_report["uncovered_mcdc_decisions"]:
            lines.extend(
                [
                    "### 1.1 Uncovered MC/DC Decisions & Synthesized Independence Pairs",
                    "",
                ]
            )
            for dec in llvm_report["uncovered_mcdc_decisions"]:
                lines.append(
                    f"- **`{dec['file']}:{dec['line_start']}:{dec['col_start']}`** "
                    f"({dec['num_conditions']} conditions, uncovered: `{', '.join(dec['uncovered_conditions'])}`):"
                )
                lines.append(
                    f"  - Executed vectors: `{'; '.join(dec['executed_vectors']) or 'none'}`"
                )
                for cond_name, synth in dec["synthesized_missing_pairs"].items():
                    lines.append(
                        f"  - **Missing Pair for `{cond_name}`**: Base `{synth['base_executed_vector']}` "
                        f"-> Add Test Vector **`{synth['required_new_vector']}`** ({synth['derivation']})"
                    )
            lines.append("")

        if llvm_report["uncovered_branches"]:
            lines.extend(
                [
                    "### 1.2 Uncovered Branch Outcomes (`branches` 9-Tuple)",
                    "",
                ]
            )
            for br in llvm_report["uncovered_branches"]:
                lines.append(
                    f"- **`{br['file']}:{br['line_start']}:{br['col_start']}`**: "
                    f"Missing `{', '.join(br['missing_directions'])}` "
                    f"(TrueCount={br['true_count']}, FalseCount={br['false_count']})"
                )
            lines.append("")

        if llvm_report["applied_justifications"]:
            lines.extend(
                [
                    "### 1.3 Applied ISO 26262-6 Clause 9.4.5 Justifications (`COV_JUSTIFIED`)",
                    "",
                    "| File:Line | Metric | ID | Category | Rationale |",
                    "|---|---|---|---|---|",
                ]
            )
            for aj in llvm_report["applied_justifications"]:
                lines.append(
                    f"| `{aj['file']}:{aj['line']}` | `{aj['metric']}` | `{aj['id']}` | "
                    f"`{aj['category']}` | {aj['reason']} |"
                )
            lines.append("")

        if llvm_report["stale_justifications"]:
            lines.extend(
                [
                    "### 1.4 Stale Justifications (REQ-COV-005 Warnings)",
                    "",
                ]
            )
            for sj in llvm_report["stale_justifications"]:
                lines.append(
                    f"- **WARNING `{sj['file']}:{sj['line']}` (`{sj['id']}`)**: {sj['reason']}"
                )
            lines.append("")

    if source_report is not None:
        lines.extend(
            [
                "## 2. Static Boolean Decision & MC/DC Hazard Scan",
                "",
                f"- **Scanned Path**: `{source_report['scanned_path']}` ({source_report['files_scanned']} files)",
                f"- **Multi-Condition Decisions (`&&` / `||`)**: {source_report['total_multi_condition_decisions']}",
                f"- **Decisions Exceeding LLVM Default Limit (`>{source_report['max_conditions_limit']}` conditions)**: {source_report['decisions_exceeding_limit_count']}",
                f"- **Decisions with Coupled Variables (`x` and `!x` / duplicate)**: {source_report['decisions_with_coupled_variables_count']}",
                f"- **Decisions with Unreachable Unique-Cause MC/DC Conditions**: {source_report['decisions_with_unreachable_mcdc_count']}",
                "",
            ]
        )

        hazards = [
            d
            for d in source_report["decisions"]
            if d["exceeds_llvm_default_limit"]
            or d["coupled_variables"]
            or d["num_conditions"] >= 4
        ]
        if hazards:
            lines.extend(
                [
                    "### 2.1 High-Complexity & Hazard Decisions (Coupled / `>= 4` Conditions)",
                    "",
                ]
            )
            for dec in hazards[:25]:
                flags: List[str] = []
                if dec["exceeds_llvm_default_limit"]:
                    flags.append(
                        f"EXCEEDS_LLVM_6_COND_LIMIT ({dec['num_conditions']} conditions)"
                    )
                if dec["coupled_variables"]:
                    flags.append("COUPLED_VARIABLES")
                if dec["mcdc_synthesis"].get("unreachable_conditions"):
                    flags.append(
                        "UNREACHABLE_MCDC("
                        + ",".join(dec["mcdc_synthesis"]["unreachable_conditions"])
                        + ")"
                    )
                flag_str = f" — **[{' | '.join(flags)}]**" if flags else ""
                lines.append(
                    f"#### `{dec['file']}:{dec['line_start']}-{dec['line_end']}` ({dec['num_conditions']} conditions){flag_str}"
                )
                lines.append(f"- **Expression**: `{dec['expression']}`")
                for c_str in dec["conditions"]:
                    lines.append(f"  - `{c_str}`")
                for cp in dec["coupled_variables"]:
                    lines.append(
                        f"- **Coupling Hazard**: `{cp['canonical_predicate']}` appears as {cp['relation']} in {', '.join(cp['conditions'])}"
                    )
                synth = dec["mcdc_synthesis"]
                if synth.get("minimal_test_vectors"):
                    lines.append(
                        f"- **Synthesized Minimal MC/DC Vectors ({len(synth['minimal_test_vectors'])} vectors)**: "
                        + ", ".join(f"`{v}`" for v in synth["minimal_test_vectors"])
                    )
                lines.append("")

    if justification_warnings:
        lines.extend(["## 3. Justification Validation Warnings", ""])
        for w in justification_warnings:
            lines.append(f"- {w}")
        lines.append("")

    return "\n".join(lines)


# ---------------------------------------------------------------------------
# Built-In Verification Suite (`--self-test`)
# ---------------------------------------------------------------------------


def run_self_test() -> int:
    """Generate realistic LLVM v3.1.0 JSON + Rust boolean fixtures and verify all gates."""
    with tempfile.TemporaryDirectory(prefix="iso26262_mcdc_selftest_") as tmpdir:
        tmp = Path(tmpdir)
        src_dir = tmp / "src"
        src_dir.mkdir(parents=True, exist_ok=True)

        # 1. Create Rust source fixture with in-code COV_JUSTIFIED markers and boolean decisions
        rust_fixture = src_dir / "service_discovery.rs"
        rust_fixture.write_text(
            """// Synthetic SOME/IP-SD & TP fixture for ISO 26262-6 MC/DC verification

pub fn detect_sd_reboot_coupled(
    reboot_flag: bool,
    is_multicast: bool,
    mc_prev: bool,
    uc_prev: bool,
) -> bool {
    if reboot_flag && ((is_multicast && !mc_prev) || (!is_multicast && !uc_prev)) {
        return true;
    }
    false
}

pub fn subscription_ready_10_conds(
    rel: bool,
    unrel: bool,
    tcp: bool,
    udp: bool,
) -> bool {
    if (rel && unrel && tcp && udp) || (rel && !unrel && tcp) || (unrel && !rel && udp) {
        return true;
    }
    false
}

pub fn validate_secure_stop_offer(
    is_secure: bool,
    rel_port_valid: bool,
    rel_port_sec: bool,
    unrel_port_valid: bool,
    unrel_port_sec: bool,
    syscall_rc: i32,
) -> Result<bool, i32> {
    if syscall_rc < 0 {
        return Err(syscall_rc); // COV_JUSTIFIED sd-socket-defensive-guard
    }
    if is_secure && ((rel_port_valid && !rel_port_sec) || (unrel_port_valid && !unrel_port_sec)) {
        return Ok(false);
    }
    Ok(true) // COV_JUSTIFIED stale-covered-line-check
}
""",
            encoding="utf-8",
        )

        # 2. Create `coverage_justifications.yaml` fixture
        just_yaml = tmp / "coverage_justifications.yaml"
        just_yaml.write_text(
            """version: 1
justifications:
  - id: sd-socket-defensive-guard
    category: defensive_programming
    platforms: [linux, qnx]
    reason: >
      Defensive programming: kernel socket descriptor validated prior to StopOffer check.
    locations:
      - file: src/service_discovery.rs
        line_start: 35
        line_end: 36
  - id: stale-covered-line-check
    category: tool_false_positive
    platforms: [linux]
    reason: >
      Intentional stale justification test entry on an already covered line.
""",
            encoding="utf-8",
        )

        # 3. Create realistic `llvm-cov export` v3.1.0 JSON fixture:
        #    - Decision at line 38 has 3 conditions (simplified security check),
        #      where C1 & C2 are covered (`[true, true, false]`) and C3 is uncovered!
        #    - Defensive branch/line at line 36 (`syscall_rc < 0`) is uncovered in raw
        #      execution but justified by `COV_JUSTIFIED sd-socket-defensive-guard`.
        #    - Line 41 has `COV_JUSTIFIED stale-covered-line-check` and IS covered (`Count = 5`),
        #      triggering stale justification detection.
        llvm_export_unclosed = {
            "version": "3.1.0",
            "type": "llvm.coverage.json.export",
            "data": [
                {
                    "files": [
                        {
                            "filename": "src/service_discovery.rs",
                            "segments": [
                                [35, 1, 5, True, True, False],
                                [36, 9, 0, True, True, False],
                                [38, 8, 5, True, True, False],
                                [41, 5, 4, True, True, False],
                            ],
                            "branches": [
                                # 9-tuple: [LineStart, ColStart, LineEnd, ColEnd, TrueCount, FalseCount, FileID, ExpFileID, Kind]
                                [35, 8, 35, 22, 0, 5, 0, 0, 4],  # Line 35/36 defensive branch
                                [38, 8, 38, 95, 2, 3, 0, 0, 6],
                            ],
                            "mcdc_records": [
                                # 11-tuple: [LineStart, ColStart, LineEnd, ColEnd, TrueDec, FalseDec, FileID, ExpFileID, Kind, ConditionsArray, TestVectorsArray]
                                [
                                    38,
                                    8,
                                    38,
                                    95,
                                    1,
                                    2,
                                    0,
                                    0,
                                    5,
                                    [True, True, False],
                                    [
                                        {
                                            "conditions": [True, True, True],
                                            "executed": True,
                                            "result": True,
                                        },
                                        {
                                            "conditions": [False, None, None],
                                            "executed": True,
                                            "result": False,
                                        },
                                        {
                                            "conditions": [True, False, None],
                                            "executed": True,
                                            "result": False,
                                        },
                                        {
                                            "conditions": [True, True, False],
                                            "executed": False,
                                            "result": False,
                                        },
                                    ],
                                ]
                            ],
                            "summary": {
                                "lines": {"count": 10, "covered": 9, "percent": 90.0},
                                "functions": {"count": 3, "covered": 3, "percent": 100.0},
                                "instantiations": {"count": 3, "covered": 3, "percent": 100.0},
                                "regions": {
                                    "count": 8,
                                    "covered": 7,
                                    "notcovered": 1,
                                    "percent": 87.5,
                                },
                                "branches": {
                                    "count": 4,
                                    "covered": 3,
                                    "notcovered": 1,
                                    "percent": 75.0,
                                },
                                "mcdc": {
                                    "count": 3,
                                    "covered": 2,
                                    "notcovered": 1,
                                    "percent": 66.66666666666666,
                                },
                            },
                        }
                    ],
                    "functions": [],
                    "totals": {},
                }
            ],
        }
        json_unclosed_path = tmp / "coverage_unclosed.json"
        json_unclosed_path.write_text(json.dumps(llvm_export_unclosed, indent=2), encoding="utf-8")

        just_map, just_warns = resolve_justifications(just_yaml, tmp)
        assert not just_warns, f"Unexpected justification warnings: {just_warns}"

        rep_unclosed = analyze_llvm_coverage_json(
            json_unclosed_path, asil="D", justified_files=just_map
        )

        # Verify:
        # 1) Statement & Branch reach 100% Effective Coverage thanks to `sd-socket-defensive-guard`
        assert rep_unclosed["summary"]["statement"]["effective_pct"] == 100.0, rep_unclosed["summary"]["statement"]
        assert rep_unclosed["summary"]["branch"]["effective_pct"] == 100.0, rep_unclosed["summary"]["branch"]
        # 2) MC/DC has 1 unjustified uncovered condition (C3 at line 38) -> ASIL-D is False
        assert not rep_unclosed["asil_compliant"]
        assert len(rep_unclosed["uncovered_mcdc_decisions"]) == 1
        dec_gap = rep_unclosed["uncovered_mcdc_decisions"][0]
        assert dec_gap["uncovered_conditions"] == ["C3"]
        assert (
            dec_gap["synthesized_missing_pairs"]["C3"]["required_new_vector"]
            == "(T, T, F) -> F"
        )
        # 3) Stale justification on line 41 is detected
        assert len(rep_unclosed["stale_justifications"]) == 1
        assert rep_unclosed["stale_justifications"][0]["id"] == "stale-covered-line-check"

        # 4) Now test 100% MC/DC closure when vector `(T, T, F) -> F` is executed
        llvm_export_closed = json.loads(json.dumps(llvm_export_unclosed))
        mcdc_rec = llvm_export_closed["data"][0]["files"][0]["mcdc_records"][0]
        mcdc_rec[9] = [True, True, True]
        mcdc_rec[10][3]["executed"] = True
        llvm_export_closed["data"][0]["files"][0]["summary"]["mcdc"] = {
            "count": 3,
            "covered": 3,
            "notcovered": 0,
            "percent": 100.0,
        }
        json_closed_path = tmp / "coverage_closed.json"
        json_closed_path.write_text(json.dumps(llvm_export_closed, indent=2), encoding="utf-8")

        rep_closed = analyze_llvm_coverage_json(
            json_closed_path, asil="D", justified_files=just_map
        )
        assert rep_closed["asil_compliant"], "Expected ASIL-D compliance after closing C3"
        assert rep_closed["summary"]["mcdc"]["effective_pct"] == 100.0

        # 5) Verify Static Source Scanner on `rust_fixture`
        src_scan = scan_source_path(rust_fixture, max_conditions=6)
        assert src_scan["total_multi_condition_decisions"] == 3, src_scan
        assert src_scan["decisions_exceeding_limit_count"] == 1, src_scan
        assert src_scan["decisions_with_coupled_variables_count"] == 2, src_scan

        # Verify the clean 5-condition decision (`validate_secure_stop_offer`) synthesizes 6 vectors (N+1)
        clean_dec = [
            d for d in src_scan["decisions"] if d["num_conditions"] == 5 and not d["coupled_variables"]
        ][0]
        assert len(clean_dec["mcdc_synthesis"]["minimal_test_vectors"]) == 6
        assert not clean_dec["mcdc_synthesis"]["unreachable_conditions"]

        # 6) Verify `--strict` exit-code gatekeeping (1 on unclosed/coupled/dangling, 0 on closed)
        devnull = tmp / "report_out.md"
        assert (
            main(
                [
                    "--llvm-json",
                    str(json_unclosed_path),
                    "--justifications",
                    str(just_yaml),
                    "--source-root",
                    str(tmp),
                    "--asil",
                    "D",
                    "--strict",
                    "--output",
                    str(devnull),
                ]
            )
            == 1
        ), "Expected --strict to exit 1 when ASIL-D MC/DC condition C3 is uncovered"
        assert (
            main(
                [
                    "--llvm-json",
                    str(json_closed_path),
                    "--justifications",
                    str(just_yaml),
                    "--source-root",
                    str(tmp),
                    "--asil",
                    "D",
                    "--strict",
                    "--output",
                    str(devnull),
                ]
            )
            == 0
        ), "Expected --strict to exit 0 when all ASIL-D metrics reach 100% effective coverage"
        assert (
            main(["--scan-source", str(rust_fixture), "--strict", "--output", str(devnull)]) == 1
        ), "Expected --strict to exit 1 when >6-condition or coupled boolean decisions exist"

    print(
        json.dumps(
            {
                "self_test": "PASSED",
                "checks_verified": [
                    "llvm.coverage.json.export v3.1.0 9-tuple branches & 11-tuple mcdc_records parsing",
                    "Uncovered MC/DC condition C3 detection & missing independence-pair synthesis ((T, T, F) -> F)",
                    "ISO 26262-6 Clause 9.4.5 COV_JUSTIFIED Raw vs. Effective coverage calculation",
                    "Stale justification detection (REQ-COV-005)",
                    "Static boolean AST scanner: >6-condition complexity / LLVM 18 bitmap overflow detection (10-cond DNF)",
                    "Coupled complementary variable detection (is_multicast & !is_multicast)",
                    "Exact N+1 (6-vector) Unique-Cause MC/DC test-vector synthesis for 5-condition decision",
                    "--strict non-zero exit code on uncovered ASIL metrics, dangling justifications, and coupled/>6 decisions",
                ],
            },
            indent=2,
        )
    )
    return 0


# ---------------------------------------------------------------------------
# CLI Entry Point
# ---------------------------------------------------------------------------


def parse_args(argv: Optional[List[str]] = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description=(
            "ISO 26262-6 Clauses 9 & 10 Structural Coverage, MC/DC & Boolean Decision Analyzer"
        )
    )
    parser.add_argument(
        "--llvm-json",
        type=Path,
        default=None,
        help="Path to `llvm-cov export --format=text` JSON file (v3.1.0 schema)",
    )
    parser.add_argument(
        "--scan-source",
        "--scan-rust",
        "--scan-dir",
        dest="scan_source",
        type=Path,
        default=None,
        help="Path to Rust/C++ file or directory to scan for multi-condition boolean decisions",
    )
    parser.add_argument(
        "--asil",
        choices=["A", "B", "C", "D"],
        default="D",
        help="Target ISO 26262-6 ASIL level (default: D)",
    )
    parser.add_argument(
        "--justifications",
        type=Path,
        default=None,
        help="Optional path to coverage_justifications.yaml or JSON file",
    )
    parser.add_argument(
        "--source-root",
        type=Path,
        default=None,
        help="Optional source root directory for scanning in-code COV_JUSTIFIED markers",
    )
    parser.add_argument(
        "--max-conditions",
        type=int,
        default=6,
        help="Maximum conditions per decision before flagging complexity / LLVM 18 cap (default: 6)",
    )
    parser.add_argument(
        "--format",
        choices=["markdown", "json", "text"],
        default="markdown",
        help="Output report format (default: markdown)",
    )
    parser.add_argument(
        "--output",
        type=Path,
        default=None,
        help="Optional file path to write output report",
    )
    parser.add_argument(
        "--dump-fixtures",
        type=Path,
        default=None,
        help="Optional directory to materialize the v3.1.0 LLVM JSON, Rust, and YAML test fixtures",
    )
    parser.add_argument(
        "--strict",
        action="store_true",
        help=(
            "Exit with code 1 if mandatory ASIL coverage has unjustified gaps, "
            "dangling COV_JUSTIFIED IDs exist, or static scan finds coupled / >max-conditions decisions"
        ),
    )
    parser.add_argument(
        "--self-test",
        action="store_true",
        help="Run built-in verification against realistic LLVM v3.1.0 MC/DC & Rust fixtures",
    )
    return parser.parse_args(argv)


def dump_fixtures_to_dir(out_dir: Path) -> None:
    """Materialize realistic LLVM v3.1.0 JSON, Rust boolean, and YAML justification fixtures."""
    out_dir.mkdir(parents=True, exist_ok=True)
    src_dir = out_dir / "src"
    src_dir.mkdir(parents=True, exist_ok=True)

    (src_dir / "service_discovery.rs").write_text(
        """// SOME/IP-SD & TP fixture for ISO 26262-6 MC/DC verification
pub fn detect_sd_reboot_coupled(
    reboot_flag: bool,
    is_multicast: bool,
    mc_prev: bool,
    uc_prev: bool,
) -> bool {
    if reboot_flag && ((is_multicast && !mc_prev) || (!is_multicast && !uc_prev)) {
        return true;
    }
    false
}

pub fn subscription_ready_10_conds(
    rel: bool,
    unrel: bool,
    tcp: bool,
    udp: bool,
) -> bool {
    if (rel && unrel && tcp && udp) || (rel && !unrel && tcp) || (unrel && !rel && udp) {
        return true;
    }
    false
}

pub fn validate_secure_stop_offer(
    is_secure: bool,
    rel_port_valid: bool,
    rel_port_sec: bool,
    unrel_port_valid: bool,
    unrel_port_sec: bool,
    syscall_rc: i32,
) -> Result<bool, i32> {
    if syscall_rc < 0 {
        return Err(syscall_rc); // COV_JUSTIFIED sd-socket-defensive-guard
    }
    if is_secure && ((rel_port_valid && !rel_port_sec) || (unrel_port_valid && !unrel_port_sec)) {
        return Ok(false);
    }
    Ok(true)
}
""",
        encoding="utf-8",
    )

    (out_dir / "coverage_justifications.yaml").write_text(
        """version: 1
justifications:
  - id: sd-socket-defensive-guard
    category: defensive_programming
    platforms: [linux, qnx]
    reason: >
      Defensive programming: kernel socket descriptor validated prior to StopOffer check.
    locations:
      - file: src/service_discovery.rs
        line_start: 33
        line_end: 35
""",
        encoding="utf-8",
    )

    llvm_export = {
        "version": "3.1.0",
        "type": "llvm.coverage.json.export",
        "data": [
            {
                "files": [
                    {
                        "filename": "src/service_discovery.rs",
                        "segments": [
                            [33, 1, 5, True, True, False],
                            [34, 9, 0, True, True, False],
                            [36, 8, 5, True, True, False],
                            [39, 5, 4, True, True, False],
                        ],
                        "branches": [
                            [33, 8, 33, 22, 0, 5, 0, 0, 4],
                            [36, 8, 36, 95, 2, 3, 0, 0, 6],
                        ],
                        "mcdc_records": [
                            [
                                36,
                                8,
                                36,
                                95,
                                1,
                                2,
                                0,
                                0,
                                5,
                                [True, True, False],
                                [
                                    {"conditions": [True, True, True], "executed": True, "result": True},
                                    {"conditions": [False, None, None], "executed": True, "result": False},
                                    {"conditions": [True, False, None], "executed": True, "result": False},
                                    {"conditions": [True, True, False], "executed": False, "result": False},
                                ],
                            ]
                        ],
                        "summary": {
                            "lines": {"count": 10, "covered": 9, "percent": 90.0},
                            "functions": {"count": 3, "covered": 3, "percent": 100.0},
                            "instantiations": {"count": 3, "covered": 3, "percent": 100.0},
                            "regions": {"count": 8, "covered": 7, "notcovered": 1, "percent": 87.5},
                            "branches": {"count": 4, "covered": 3, "notcovered": 1, "percent": 75.0},
                            "mcdc": {"count": 3, "covered": 2, "notcovered": 1, "percent": 66.66666666666666},
                        },
                    }
                ],
                "functions": [],
                "totals": {},
            }
        ],
    }
    (out_dir / "coverage_export_v3_1_0.json").write_text(
        json.dumps(llvm_export, indent=2) + "\n", encoding="utf-8"
    )


def main(argv: Optional[List[str]] = None) -> int:
    args = parse_args(argv)

    if args.dump_fixtures is not None:
        dump_fixtures_to_dir(args.dump_fixtures)

    if args.self_test:
        return run_self_test()

    if args.llvm_json is None and args.scan_source is None:
        if args.dump_fixtures is not None:
            return 0
        # Default to running self-test if invoked with no target arguments
        return run_self_test()

    src_root = args.source_root or (
        args.scan_source if args.scan_source and args.scan_source.is_dir() else None
    )
    just_map, just_warnings = resolve_justifications(args.justifications, src_root)

    llvm_report: Optional[Dict[str, Any]] = None
    if args.llvm_json is not None:
        llvm_report = analyze_llvm_coverage_json(
            args.llvm_json, asil=args.asil, justified_files=just_map
        )

    source_report: Optional[Dict[str, Any]] = None
    if args.scan_source is not None:
        source_report = scan_source_path(
            args.scan_source, max_conditions=args.max_conditions
        )

    if args.format == "json":
        rendered = json.dumps(
            {
                "llvm_coverage": llvm_report,
                "source_boolean_analysis": source_report,
                "justification_warnings": just_warnings,
            },
            indent=2,
        )
    else:
        rendered = format_report_markdown(llvm_report, source_report, just_warnings)

    if args.output is not None:
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(rendered + "\n", encoding="utf-8")
    else:
        print(rendered)

    if args.strict:
        if just_warnings:
            return 1
        if llvm_report is not None and not llvm_report["asil_compliant"]:
            return 1
        if source_report is not None and (
            source_report["decisions_exceeding_limit_count"] > 0
            or source_report["decisions_with_coupled_variables_count"] > 0
            or source_report["decisions_with_unreachable_mcdc_count"] > 0
        ):
            return 1

    return 0


if __name__ == "__main__":
    sys.exit(main())
