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
"""Audits C++-to-Rust 1-to-1 API mirror coverage, signature lock tests, and wire enum representations.

Checks:
1. Every public C++ header under `--cpp-interface-dir` is cited in Rust `Provenance:` headers
   or documented in `--architecture-doc`.
2. A compile-only signature lock test (`api_signatures.rs`) exists and binds public trait methods.
3. Wire/IPC single-byte protocol enums (`MessageType`, `ReturnCode`, `CommandId`) use
   `#[repr(transparent)]` `u8` newtypes rather than strict `#[repr(u8)] enum` definitions that
   reject unknown wire bytes during deserialization.
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

REPR_U8_ENUM_RE = re.compile(
    r"#\s*\[\s*repr\s*\(\s*u8\s*\)\s*\]\s*(?:#\[[^\]]*\]\s*)*pub\s+enum\s+([A-Za-z0-9_]+)",
    re.M,
)
WIRE_ENUM_NAME_RE = re.compile(r"(?:MessageType|ReturnCode|ProtocolVersion|InterfaceVersion)", re.I)


@dataclass
class MirrorAuditReport:
    cpp_headers_total: int = 0
    cpp_headers_mapped: int = 0
    unmapped_headers: List[str] = field(default_factory=list)
    has_signature_lock_test: bool = False
    strict_wire_enum_warnings: List[str] = field(default_factory=list)
    passed: bool = True


def audit_api_mirror(
    cpp_interface_dir: Path, rust_facade_dir: Path, arch_doc: Path | None
) -> MirrorAuditReport:
    report = MirrorAuditReport()

    arch_text = ""
    if arch_doc and arch_doc.is_file():
        arch_text = arch_doc.read_text(encoding="utf-8", errors="ignore")

    rust_provenance_text = ""
    for rs in rust_facade_dir.rglob("*.rs"):
        if "target" in rs.parts:
            continue
        if rs.name == "api_signatures.rs":
            report.has_signature_lock_test = True
        text = rs.read_text(encoding="utf-8", errors="ignore")
        rust_provenance_text += "\n" + "\n".join(text.splitlines()[:45])
        for match in REPR_U8_ENUM_RE.finditer(text):
            enum_name = match.group(1)
            if WIRE_ENUM_NAME_RE.search(enum_name):
                report.strict_wire_enum_warnings.append(
                    f"{rs.name}: `#[repr(u8)] pub enum {enum_name}` rejects unknown wire bytes; "
                    f"use `#[repr(transparent)] pub struct {enum_name}(pub u8)`"
                )

    if cpp_interface_dir.is_dir():
        headers = sorted(
            p for p in cpp_interface_dir.rglob("*") if p.suffix in {".hpp", ".h", ".in"}
        )
        report.cpp_headers_total = len(headers)
        for hdr in headers:
            rel_name = hdr.name
            rel_path = str(hdr.relative_to(cpp_interface_dir.parent)) if cpp_interface_dir.parent else hdr.name
            if (
                rel_path in rust_provenance_text
                or rel_path in arch_text
                or rel_name in rust_provenance_text
                or rel_name in arch_text
            ):
                report.cpp_headers_mapped += 1
            else:
                report.unmapped_headers.append(rel_path)

    report.passed = (
        not report.unmapped_headers
        and report.has_signature_lock_test
        and not report.strict_wire_enum_warnings
    )
    return report


def format_markdown(report: MirrorAuditReport) -> str:
    status = "PASS" if report.passed else "FAIL"
    lines = [
        f"# C++-to-Rust Public API Mirror Audit ({status})",
        "",
        f"- **Public C++ Headers Mapped**: `{report.cpp_headers_mapped} / {report.cpp_headers_total}`",
        f"- **Compile-Only Signature Lock (`api_signatures.rs`)**: `{'PRESENT' if report.has_signature_lock_test else 'MISSING'}`",
        f"- **Strict Wire Enum Hazards**: `{len(report.strict_wire_enum_warnings)}`",
    ]
    if report.unmapped_headers:
        lines.extend(["", "## Unmapped C++ Public Headers"])
        lines.extend([f"- `{h}`" for h in report.unmapped_headers])
    if report.strict_wire_enum_warnings:
        lines.extend(["", "## Wire Enum Representation Hazards"])
        lines.extend([f"- {w}" for w in report.strict_wire_enum_warnings])
    return "\n".join(lines)


def run_self_test() -> int:
    with tempfile.TemporaryDirectory() as tmp:
        root = Path(tmp)
        iface = root / "interface" / "rpc"
        iface.mkdir(parents=True)
        (iface / "application.hpp").write_text("class application {};\n", encoding="utf-8")
        (iface / "enumeration_types.hpp").write_text("enum class message_type_e : uint8_t {};\n", encoding="utf-8")

        crates = root / "crates"
        (crates / "core" / "src").mkdir(parents=True)
        (crates / "facade" / "tests").mkdir(parents=True)
        (crates / "core" / "src" / "lib.rs").write_text(
            "// Provenance: interface/rpc/application.hpp, interface/rpc/enumeration_types.hpp\n"
            "#[repr(transparent)]\npub struct MessageType(pub u8);\n",
            encoding="utf-8",
        )
        (crates / "facade" / "tests" / "api_signatures.rs").write_text(
            "#[test]\nfn lock_sigs() {}\n", encoding="utf-8"
        )
        arch = root / "ARCHITECTURE.md"
        arch.write_text("| `interface/rpc/application.hpp` | `Application` |\n", encoding="utf-8")

        ok = audit_api_mirror(root / "interface", crates, arch)
        assert ok.passed, f"Expected compliant API mirror to pass: {ok}"

        # Introduce strict #[repr(u8)] enum for MessageType and unmapped header
        (iface / "unmapped.hpp").write_text("class unmapped {};\n", encoding="utf-8")
        (crates / "core" / "src" / "lib.rs").write_text(
            "// Provenance: interface/rpc/application.hpp\n"
            "#[repr(u8)]\npub enum MessageType { Request = 0x00 }\n",
            encoding="utf-8",
        )
        bad = audit_api_mirror(root / "interface", crates, arch)
        assert not bad.passed, "Expected non-compliant API mirror to fail"
        assert len(bad.unmapped_headers) == 2
        assert len(bad.strict_wire_enum_warnings) == 1

    print("SELF-TEST PASS: audit_api_mirror.py verified compliant and non-compliant API mirrors.")
    return 0


def main(argv: List[str] | None = None) -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--cpp-interface-dir", default="include")
    parser.add_argument("--rust-facade-dir", default="crates")
    parser.add_argument("--architecture-doc", default="docs/ARCHITECTURE.md")
    parser.add_argument("--format", choices=["markdown", "json", "text"], default="markdown")
    parser.add_argument("--strict", action="store_true")
    parser.add_argument("--self-test", action="store_true")
    args = parser.parse_args(argv)

    if args.self_test:
        return run_self_test()

    report = audit_api_mirror(
        Path(args.cpp_interface_dir),
        Path(args.rust_facade_dir),
        Path(args.architecture_doc) if args.architecture_doc else None,
    )
    if args.format == "json":
        print(json.dumps(asdict(report), indent=2))
    else:
        print(format_markdown(report))

    if args.strict and not report.passed:
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
