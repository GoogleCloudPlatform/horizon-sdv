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
"""Audits C/C++-to-Rust port workspace provenance, waivers, pin-only gitlinks, and DEV/OPT ledgers.

Checks:
1. Pin-only gitlinks (`update = none` in `.gitmodules`) are empty in the working tree.
2. Every upstream C/C++ file listed in `milestones/*.md` up to `--milestone` is either cited
   in a Rust source file `Provenance:` header block or documented in a `## Waivers` section
   with a justification >= 20 characters.
3. Every `DEV-*` and `OPT-*` identifier referenced in Rust source comments resolves
   to a defined entry in `docs/DEVIATIONS.md` or `docs/OPTIMIZATIONS.md`.
"""

from __future__ import annotations

import argparse
from dataclasses import asdict, dataclass, field
import json
from pathlib import Path
import re
import sys
import tempfile
from typing import List, Set

UPSTREAM_TABLE_RE = re.compile(r"^\| `((?:interface|implementation|src|include)/[^`]+)` \|", re.M)
WAIVER_LINE_RE = re.compile(r"^- `([^`]+)`: (.{20,})$", re.M)
PROVENANCE_FILE_RE = re.compile(
    r"((?:interface|implementation|src|include)/[\w./-]+\.(?:hpp|cpp|h|c|cc|cxx|in))"
)
DEV_ID_RE = re.compile(r"\b(DEV-\d{2,4})\b")
OPT_ID_RE = re.compile(r"\b(OPT-\d{2,4})\b")
PHASE_NUM_RE = re.compile(r"^(?:phase[-_]?|stage[-_]?|step[-_]?|[A-Za-z])?(\d+)$", re.I)


@dataclass
class AuditReport:
    workspace: str
    rust_dir: str
    milestone: str
    required_files: int = 0
    cited_files: int = 0
    waived_files: int = 0
    uncited_files: List[str] = field(default_factory=list)
    dangling_dev_ids: List[str] = field(default_factory=list)
    dangling_opt_ids: List[str] = field(default_factory=list)
    quarantine_violations: List[str] = field(default_factory=list)
    passed: bool = True


def _check_pin_only_gitlinks(workspace: Path) -> List[str]:
    violations: List[str] = []
    gitmodules = workspace / ".gitmodules"
    if not gitmodules.is_file():
        return violations

    text = gitmodules.read_text(encoding="utf-8", errors="ignore")
    current_path: str | None = None
    current_update: str | None = None

    def check_block(p: str | None, upd: str | None) -> None:
        if p and upd == "none":
            sub_dir = workspace / p
            if sub_dir.is_dir() and any(sub_dir.iterdir()):
                violations.append(
                    f"Pin-only submodule '{p}' (update = none) is non-empty in working tree"
                )

    for raw_line in text.splitlines():
        line = raw_line.strip()
        if line.startswith("[submodule "):
            check_block(current_path, current_update)
            current_path = None
            current_update = None
        elif line.startswith("path") and "=" in line:
            current_path = line.split("=", 1)[1].strip()
        elif line.startswith("update") and "=" in line:
            current_update = line.split("=", 1)[1].strip()
    check_block(current_path, current_update)
    return violations


def _extract_leading_header_block(lines: List[str], max_lines: int = 120) -> str:
    """Extracts the leading comment/attribute header block of a Rust source file."""
    collected: List[str] = []
    for idx, line in enumerate(lines[:max_lines]):
        stripped = line.strip()
        if idx < 40 or not stripped or stripped.startswith(("//", "/*", "*", "#!")):
            collected.append(line)
        else:
            break
    return "\n".join(collected)


def _collect_milestone_files(
    milestones_dir: Path, max_phase: int | None
) -> tuple[List[str], Set[str]]:
    needed: List[str] = []
    waived: Set[str] = set()
    if not milestones_dir.is_dir():
        return needed, waived

    cards: List[tuple[int, Path]] = []
    for p in milestones_dir.glob("*.md"):
        if p.name.upper() == "README.MD":
            continue
        m = PHASE_NUM_RE.match(p.stem)
        if m:
            idx = int(m.group(1))
            if max_phase is None or idx <= max_phase:
                cards.append((idx, p))

    for _, card_path in sorted(cards, key=lambda item: (item[0], item[1].name)):
        content = card_path.read_text(encoding="utf-8", errors="ignore")
        for match in UPSTREAM_TABLE_RE.findall(content):
            if match not in needed:
                needed.append(match)
        if "## Waivers" in content:
            after_waivers = content.split("## Waivers", 1)[1]
            waiver_sec = re.split(r"^##\s+", after_waivers, maxsplit=1, flags=re.M)[0]
            for w_file, _ in WAIVER_LINE_RE.findall(waiver_sec):
                waived.add(w_file)
    return needed, waived


def audit_workspace(workspace: Path, rust_dir: Path, milestone: str | None) -> AuditReport:
    max_phase: int | None = None
    if milestone:
        m = PHASE_NUM_RE.match(milestone.strip())
        if m:
            max_phase = int(m.group(1))

    report = AuditReport(
        workspace=str(workspace),
        rust_dir=str(rust_dir),
        milestone=milestone or "ALL",
    )

    report.quarantine_violations = _check_pin_only_gitlinks(workspace)

    needed, waived = _collect_milestone_files(workspace / "milestones", max_phase)
    report.required_files = len(needed)
    report.waived_files = len(waived)

    cited: Set[str] = set()
    referenced_devs: Set[str] = set()
    referenced_opts: Set[str] = set()

    crates_root = rust_dir / "crates" if (rust_dir / "crates").is_dir() else rust_dir
    if crates_root.is_dir():
        for rs_file in crates_root.rglob("*.rs"):
            if "target" in rs_file.parts or ".git" in rs_file.parts:
                continue
            lines = rs_file.read_text(encoding="utf-8", errors="ignore").splitlines()
            head = _extract_leading_header_block(lines)
            cited.update(PROVENANCE_FILE_RE.findall(head))
            full_text = "\n".join(lines)
            referenced_devs.update(DEV_ID_RE.findall(full_text))
            referenced_opts.update(OPT_ID_RE.findall(full_text))

    for req in needed:
        if req in cited:
            report.cited_files += 1
        elif req not in waived:
            report.uncited_files.append(req)

    dev_doc = rust_dir / "docs" / "DEVIATIONS.md"
    if dev_doc.is_file():
        defined_devs = set(DEV_ID_RE.findall(dev_doc.read_text(encoding="utf-8", errors="ignore")))
        report.dangling_dev_ids = sorted(referenced_devs - defined_devs)

    opt_doc = rust_dir / "docs" / "OPTIMIZATIONS.md"
    if opt_doc.is_file():
        defined_opts = set(OPT_ID_RE.findall(opt_doc.read_text(encoding="utf-8", errors="ignore")))
        report.dangling_opt_ids = sorted(referenced_opts - defined_opts)

    report.passed = not (
        report.uncited_files
        or report.dangling_dev_ids
        or report.dangling_opt_ids
        or report.quarantine_violations
    )
    return report


def format_markdown(report: AuditReport) -> str:
    status = "PASS" if report.passed else "FAIL"
    lines = [
        f"# Port Provenance & Harness Audit ({status})",
        "",
        f"- **Phase / Milestone Scope**: `{report.milestone}`",
        f"- **Upstream Files Accounted For**: `{report.required_files - len(report.uncited_files)} / {report.required_files}` "
        f"(cited: `{report.cited_files}`, waived: `{report.waived_files}`)",
        f"- **Pin-Only Quarantine Violations**: `{len(report.quarantine_violations)}`",
        f"- **Dangling `DEV-*` IDs**: `{len(report.dangling_dev_ids)}`",
        f"- **Dangling `OPT-*` IDs**: `{len(report.dangling_opt_ids)}`",
    ]
    if report.uncited_files:
        lines.extend(["", "## Uncited & Unwaived Upstream Files"])
        lines.extend([f"- `{f}`" for f in report.uncited_files])
    if report.quarantine_violations:
        lines.extend(["", "## Quarantine Violations"])
        lines.extend([f"- {v}" for v in report.quarantine_violations])
    if report.dangling_dev_ids or report.dangling_opt_ids:
        lines.extend(["", "## Dangling Ledger Citations"])
        for d in report.dangling_dev_ids:
            lines.append(f"- `{d}` not defined in `docs/DEVIATIONS.md`")
        for o in report.dangling_opt_ids:
            lines.append(f"- `{o}` not defined in `docs/OPTIMIZATIONS.md`")
    return "\n".join(lines)


def run_self_test() -> int:
    with tempfile.TemporaryDirectory() as tmp:
        ws = Path(tmp)
        (ws / ".gitmodules").write_text(
            '[submodule "upstream/cpp"]\n\tpath = upstream/cpp\n\turl = https://example.com/cpp.git\n\tupdate = none\n',
            encoding="utf-8",
        )
        (ws / "upstream" / "cpp").mkdir(parents=True)
        (ws / "milestones").mkdir()
        (ws / "milestones" / "phase-00.md").write_text(
            "| File | LOC |\n|---|---|\n| `include/foo.hpp` | 10 |\n| `src/bar.cpp` | 20 |\n",
            encoding="utf-8",
        )
        rust = ws / "rust-port"
        (rust / "crates" / "core" / "src").mkdir(parents=True)
        (rust / "docs").mkdir(parents=True)
        (rust / "docs" / "DEVIATIONS.md").write_text("# Deviations\n- **DEV-01**: Documented.\n", encoding="utf-8")
        (rust / "docs" / "OPTIMIZATIONS.md").write_text("# Optimizations\n- **OPT-01**: Documented.\n", encoding="utf-8")
        (rust / "crates" / "core" / "src" / "lib.rs").write_text(
            "// Provenance: include/foo.hpp, src/bar.cpp\n// See DEV-01 and OPT-01\npub fn ok() {}\n",
            encoding="utf-8",
        )

        ok_report = audit_workspace(ws, rust, "phase-00")
        assert ok_report.passed, f"Expected compliant workspace to pass: {ok_report}"

        # Introduce violations: uncited file (with a `## Test porting notes` bullet after `## Waivers`),
        # dangling DEV-99, and non-empty pin-only dir
        (ws / "milestones" / "phase-01.md").write_text(
            "| File | LOC |\n|---|---|\n| `src/missing.cpp` | 50 |\n\n## Waivers\n\n## Test porting notes\n\n"
            "- `src/missing.cpp`: test note after Waivers must not act as a file waiver\n",
            encoding="utf-8",
        )
        (rust / "crates" / "core" / "src" / "lib.rs").write_text(
            "// Provenance: include/foo.hpp, src/bar.cpp\n// Dangling DEV-99\n",
            encoding="utf-8",
        )
        (ws / "upstream" / "cpp" / "checked_out.c").write_text("bad", encoding="utf-8")

        bad_report = audit_workspace(ws, rust, "phase-01")
        assert not bad_report.passed, "Expected non-compliant workspace to fail"
        assert "src/missing.cpp" in bad_report.uncited_files
        assert "DEV-99" in bad_report.dangling_dev_ids
        assert len(bad_report.quarantine_violations) == 1

    print("SELF-TEST PASS: verify_port_provenance.py verified compliant and non-compliant workspaces.")
    return 0


def main(argv: List[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--workspace", default=".", help="Port harness workspace root")
    parser.add_argument("--rust-dir", default=".", help="Rust output workspace directory")
    parser.add_argument("--milestone", default=None, help="Target phase/milestone cap (e.g., phase-03)")
    parser.add_argument("--format", choices=["markdown", "json", "text"], default="markdown")
    parser.add_argument("--strict", action="store_true", help="Exit 1 on any violation")
    parser.add_argument("--self-test", action="store_true", help="Run built-in self-tests")
    args = parser.parse_args(argv)

    if args.self_test:
        return run_self_test()

    ws = Path(args.workspace).resolve()
    rust_dir = Path(args.rust_dir)
    if not rust_dir.is_absolute():
        rust_dir = ws / rust_dir

    report = audit_workspace(ws, rust_dir, args.milestone)
    if args.format == "json":
        print(json.dumps(asdict(report), indent=2))
    else:
        print(format_markdown(report))

    if args.strict and not report.passed:
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
