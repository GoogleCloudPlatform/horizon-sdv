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
"""Automotive Requirements Traceability Discovery & Bidirectional Audit Tool.

Discovers requirements traceability frameworks (Sphinx-Needs, TRLC, StrictDoc,
Doorstop, ReqIF, Mantra #[req(...)], GoogleTest RecordProperty Verifies /
lobster-tracing, Doxygen tags, and Rust doc-attributes), extracts requirement
definitions (SYS.2 / SWE.1 / SWE.2), implementation references (SWE.3), and
verification test references (SWE.4-SWE.6), and generates Markdown and CSV
traceability matrices.
"""

from __future__ import annotations

import argparse
from collections import defaultdict
import csv
from dataclasses import dataclass, field
from pathlib import Path
import re
import sys
import tempfile
import xml.etree.ElementTree as ET


@dataclass
class RequirementDef:
    """Represents a requirement or specification node defined in spec files."""

    req_id: str
    canonical_id: str
    title: str = ""
    framework: str = "sphinx-needs"
    layer: str = "SWE.1"
    directive_type: str = "requirement"
    is_normative: bool = True
    locations: list[str] = field(default_factory=list)
    satisfies: list[str] = field(default_factory=list)
    aliases: list[str] = field(default_factory=list)


@dataclass
class CitationRef:
    """Represents an implementation (SWE.3) or verification test (SWE.4-6) citation."""

    ref_id: str
    file_path: str
    line_number: int
    symbol_name: str
    role: str  # "code" (SWE.3) or "test" (SWE.4-6)
    tag_kind: str
    target_ids: list[str] = field(default_factory=list)


def canonicalize_id(raw_id: str) -> str:
    """Normalize requirement IDs for consistent matching across tools."""
    cleaned = raw_id.strip().strip(",;[]()")
    if "@" in cleaned:
        cleaned = cleaned.split("@", 1)[0].strip()
    if cleaned.lower().startswith("feat_req_"):
        return cleaned.lower()
    if cleaned.upper().startswith(
        ("REQ_", "TC_", "SRS_", "SYS_", "ASR_", "FEAT_", "SCR-", "ARCH_", "AOU_", "FM_")
    ):
        return cleaned.upper()
    return cleaned


RST_CODE_BLOCK_RE = re.compile(
    r"(?:^\s*\.\.\s+(?:code-block|code|sourcecode)::[^\n]*\n(?:^\s*\n|^\s+[^\n]*\n)+)",
    re.MULTILINE,
)
RST_DIRECTIVE_RE = re.compile(
    r"^\s*\.\.\s+(requirement|feat_req|spec_req|heading|information|story|spec|impl|test|test_case)::\s*([^\n]*)\n((?:^\s+:[A-Za-z0-9_]+:[^\n]*\n)+)",
    re.MULTILINE | re.IGNORECASE,
)
RST_ID_OPTION_RE = re.compile(r"^\s*:id:\s*([A-Za-z0-9_.-]+)", re.MULTILINE | re.IGNORECASE)
RST_REQTYPE_OPTION_RE = re.compile(r"^\s*:reqtype:\s*([^\n]+)", re.MULTILINE | re.IGNORECASE)
RST_SATISFIES_OPTION_RE = re.compile(r"^\s*:satisfies:\s*([^\n]*)", re.MULTILINE | re.IGNORECASE)

TRLC_PACKAGE_RE = re.compile(r"^\s*package\s+([A-Za-z0-9_]+)", re.MULTILINE)
TRLC_OBJECT_RE = re.compile(
    r"(?:/\*\s*broken_link_c/issue/(\d+)\s*\*/\s*)?"
    r"\b([A-Za-z0-9_]+)\.([A-Za-z0-9_]+)\s+([A-Za-z0-9_]+)\s*\{(.*?)\n\s*\}",
    re.DOTALL,
)
TRLC_DERIVED_RE = re.compile(r"\bderived_from\s*=\s*\[([^\]]*)\]", re.DOTALL)
TRLC_MITIGATES_RE = re.compile(r'\bmitigates\s*=\s*"([^"]*)"')

SDOC_BLOCK_RE = re.compile(
    r"^\[((?:COMPOSITE_)?REQUIREMENT|SECTION)\]\s*\n((?:^[A-Z_]+:[^\n]*\n|^\s+[^\n]*\n|^>>>.*?^<<<\n)+)",
    re.MULTILINE | re.DOTALL,
)
SDOC_UID_RE = re.compile(r"^UID:\s*([A-Za-z0-9_.-]+)", re.MULTILINE)
SDOC_TITLE_RE = re.compile(r"^TITLE:\s*([^\n]+)", re.MULTILINE)
SDOC_REL_VALUE_RE = re.compile(r"^\s*VALUE:\s*([A-Za-z0-9_.-]+)", re.MULTILINE)

IMPLEMENTS_TAG_RE = re.compile(r"@implements\b\s+([^\n*]+)", re.IGNORECASE)
SATISFIES_TAG_RE = re.compile(r"@satisfies\b\s+([^\n*]+)", re.IGNORECASE)
TESTS_TAG_RE = re.compile(r"@tests\b\s+([^\n*]+)", re.IGNORECASE)
TEST_CASE_TAG_RE = re.compile(r"@test_case\b\s+(TC_[A-Za-z0-9_]+)", re.IGNORECASE)
DOXYGEN_REQ_TAG_RE = re.compile(r"@req\b\s*:?\s*\[?([A-Za-z0-9_.-]+)\]?", re.IGNORECASE)

MANTRA_ATTR_RE = re.compile(r"#\[\s*req(?:uirements)?\s*\(\s*([^)]+)\s*\)\s*\]")
RUST_DOC_ATTR_RE = re.compile(r'#\[\s*doc\s*=\s*"([^"]+)"\s*\]')
SDOC_MARKER_RE = re.compile(r"@sdoc\[([A-Za-z0-9_.-]+)\]")
GTEST_RECORD_PROP_RE = re.compile(
    r'RecordProperty\s*\(\s*"(Verifies|lobster-tracing)"\s*,\s*"([^"]+)"\s*\)'
)
ID_TOKEN_RE = re.compile(r"\b([A-Za-z][A-Za-z0-9_.:@-]*[A-Za-z0-9_])\b")


def parse_rst_file(
    rst_path: Path,
    rel_prefix: Path,
    layer_default: str,
    requirements: dict[str, RequirementDef],
    doc_example_leaks: list[str],
) -> int:
    """Parse Sphinx-Needs directives from an RST file, stripping code-block examples."""
    raw_text = rst_path.read_text(encoding="utf-8", errors="ignore")
    rel_path = rst_path.relative_to(rel_prefix) if rst_path.is_relative_to(rel_prefix) else rst_path

    for cb_match in RST_CODE_BLOCK_RE.finditer(raw_text):
        cb_text = cb_match.group(0)
        cb_line = raw_text[: cb_match.start()].count("\n") + 1
        for id_m in RST_ID_OPTION_RE.finditer(cb_text):
            leaked_id = canonicalize_id(id_m.group(1))
            doc_example_leaks.append(f"{rel_path}:{cb_line} ({leaked_id})")

    def _blank_lines(match: re.Match[str]) -> str:
        return "\n" * match.group(0).count("\n")

    cleaned_text = RST_CODE_BLOCK_RE.sub(_blank_lines, raw_text)
    count = 0

    for match in RST_DIRECTIVE_RE.finditer(cleaned_text):
        directive = match.group(1).lower()
        title = match.group(2).strip()
        options_block = match.group(3)
        line_num = cleaned_text[: match.start()].count("\n") + 1

        id_match = RST_ID_OPTION_RE.search(options_block)
        if not id_match:
            continue

        raw_id = id_match.group(1).strip()
        canon_id = canonicalize_id(raw_id)

        reqtype_match = RST_REQTYPE_OPTION_RE.search(options_block)
        reqtype_val = reqtype_match.group(1).strip().lower() if reqtype_match else ""

        if directive == "heading":
            dtype, is_normative = "heading", False
        elif directive == "information" or reqtype_val == "information":
            dtype, is_normative = "information", False
        else:
            dtype, is_normative = "requirement", True

        satisfies_list: list[str] = []
        sat_match = RST_SATISFIES_OPTION_RE.search(options_block)
        if sat_match and sat_match.group(1).strip():
            for tok in re.split(r"[,\s]+", sat_match.group(1).strip()):
                if tok:
                    satisfies_list.append(canonicalize_id(tok))

        layer = layer_default
        if canon_id.startswith(("feat_req_", "SYS_")):
            layer = "SYS.2"
        elif "_ARCH_" in canon_id or canon_id.startswith("ARCH_"):
            layer = "SWE.2"

        loc_str = f"{rel_path}:{line_num}"
        if canon_id in requirements:
            existing = requirements[canon_id]
            existing.locations.append(loc_str)
            for s in satisfies_list:
                if s not in existing.satisfies:
                    existing.satisfies.append(s)
        else:
            requirements[canon_id] = RequirementDef(
                req_id=raw_id,
                canonical_id=canon_id,
                title=title,
                framework="sphinx-needs",
                layer=layer,
                directive_type=dtype,
                is_normative=is_normative,
                locations=[loc_str],
                satisfies=satisfies_list,
            )
        count += 1

    return count


def parse_trlc_file(
    trlc_path: Path,
    rel_prefix: Path,
    requirements: dict[str, RequirementDef],
    alias_map: dict[str, str],
) -> int:
    """Parse TRLC (.trlc) requirement, AoU, and FailureMode objects."""
    raw_text = trlc_path.read_text(encoding="utf-8", errors="ignore")
    rel_path = trlc_path.relative_to(rel_prefix) if trlc_path.is_relative_to(rel_prefix) else trlc_path

    pkg_match = TRLC_PACKAGE_RE.search(raw_text)
    pkg_name = pkg_match.group(1) if pkg_match else ""
    count = 0

    for match in TRLC_OBJECT_RE.finditer(raw_text):
        issue_num, schema_pkg, type_name, obj_name, body = (
            match.group(1),
            match.group(2),
            match.group(3),
            match.group(4),
            match.group(5),
        )
        line_num = raw_text[: match.start()].count("\n") + 1

        qualified_id = f"{pkg_name}.{obj_name}" if pkg_name else obj_name
        canon_id = canonicalize_id(qualified_id)

        if type_name == "AssumedSystemReq":
            layer, dtype, is_norm = "SYS.2", "assumed_system_req", True
        elif type_name == "FeatReq":
            layer, dtype, is_norm = "SWE.1", "feature_req", True
        elif type_name == "CompReq":
            layer, dtype, is_norm = "SWE.1", "component_req", True
        elif type_name == "AoU":
            layer, dtype, is_norm = "SYS.2", "assumption_of_use", False
        elif type_name == "FailureMode":
            layer, dtype, is_norm = "SWE.2", "failure_mode", False
        else:
            layer, dtype, is_norm = "SWE.1", type_name.lower(), True

        satisfies: list[str] = []
        for dm in TRLC_DERIVED_RE.finditer(body):
            for tok in dm.group(1).split(","):
                tok = tok.strip()
                if tok:
                    satisfies.append(canonicalize_id(tok))
        for mm in TRLC_MITIGATES_RE.finditer(body):
            for tok in mm.group(1).split(","):
                tok = tok.strip()
                if tok:
                    satisfies.append(canonicalize_id(tok))

        aliases = [canonicalize_id(obj_name)]
        alias_map[canonicalize_id(obj_name)] = canon_id
        if issue_num:
            scr_alias = f"SCR-{issue_num}"
            aliases.append(scr_alias)
            alias_map[scr_alias] = canon_id

        loc_str = f"{rel_path}:{line_num}"
        if canon_id in requirements:
            requirements[canon_id].locations.append(loc_str)
        else:
            requirements[canon_id] = RequirementDef(
                req_id=qualified_id,
                canonical_id=canon_id,
                title=f"{schema_pkg}.{type_name} {obj_name}",
                framework="trlc",
                layer=layer,
                directive_type=dtype,
                is_normative=is_norm,
                locations=[loc_str],
                satisfies=satisfies,
                aliases=aliases,
            )
        count += 1

    return count


def parse_sdoc_file(
    sdoc_path: Path,
    rel_prefix: Path,
    requirements: dict[str, RequirementDef],
) -> int:
    """Parse StrictDoc (.sdoc) requirement blocks."""
    raw_text = sdoc_path.read_text(encoding="utf-8", errors="ignore")
    rel_path = sdoc_path.relative_to(rel_prefix) if sdoc_path.is_relative_to(rel_prefix) else sdoc_path
    count = 0

    for match in SDOC_BLOCK_RE.finditer(raw_text):
        block_kind = match.group(1).upper()
        body = match.group(2)
        line_num = raw_text[: match.start()].count("\n") + 1
        uid_m = SDOC_UID_RE.search(body)
        if not uid_m:
            continue
        raw_id = uid_m.group(1).strip()
        canon_id = canonicalize_id(raw_id)
        title_m = SDOC_TITLE_RE.search(body)
        title = title_m.group(1).strip() if title_m else ""
        rels = [canonicalize_id(m.group(1)) for m in SDOC_REL_VALUE_RE.finditer(body)]
        is_norm = block_kind != "SECTION"
        loc_str = f"{rel_path}:{line_num}"
        if canon_id in requirements:
            requirements[canon_id].locations.append(loc_str)
        else:
            requirements[canon_id] = RequirementDef(
                req_id=raw_id,
                canonical_id=canon_id,
                title=title,
                framework="strictdoc",
                layer="SWE.1",
                directive_type="requirement" if is_norm else "heading",
                is_normative=is_norm,
                locations=[loc_str],
                satisfies=rels,
            )
        count += 1
    return count


def parse_doorstop_dir(
    doorstop_cfg: Path,
    rel_prefix: Path,
    requirements: dict[str, RequirementDef],
) -> int:
    """Parse Doorstop YAML requirement items in a directory containing .doorstop.yml."""
    parent_dir = doorstop_cfg.parent
    count = 0
    for yml_file in sorted(parent_dir.glob("*.yml")) + sorted(parent_dir.glob("*.yaml")):
        if yml_file.name.startswith(".doorstop"):
            continue
        text = yml_file.read_text(encoding="utf-8", errors="ignore")
        if "active:" not in text or "text:" not in text:
            continue
        raw_id = yml_file.stem
        canon_id = canonicalize_id(raw_id)
        norm_match = re.search(r"^normative:\s*(true|false)", text, re.MULTILINE | re.IGNORECASE)
        is_norm = norm_match.group(1).lower() == "true" if norm_match else True
        links: list[str] = []
        in_links = False
        for line in text.splitlines():
            if re.match(r"^links:\s*$", line):
                in_links = True
                continue
            if in_links:
                if line.startswith("- "):
                    link_item = line[2:].strip().split(":", 1)[0].strip()
                    if link_item:
                        links.append(canonicalize_id(link_item))
                elif not line.startswith(" "):
                    in_links = False
        rel_path = yml_file.relative_to(rel_prefix) if yml_file.is_relative_to(rel_prefix) else yml_file
        loc_str = f"{rel_path}:1"
        if canon_id in requirements:
            requirements[canon_id].locations.append(loc_str)
        else:
            requirements[canon_id] = RequirementDef(
                req_id=raw_id,
                canonical_id=canon_id,
                title=f"Doorstop {raw_id}",
                framework="doorstop",
                layer="SWE.1",
                directive_type="requirement" if is_norm else "heading",
                is_normative=is_norm,
                locations=[loc_str],
                satisfies=links,
            )
        count += 1
    return count


def parse_reqif_file(
    reqif_path: Path,
    rel_prefix: Path,
    requirements: dict[str, RequirementDef],
) -> int:
    """Parse ReqIF (.reqif) XML specification objects."""
    rel_path = reqif_path.relative_to(rel_prefix) if reqif_path.is_relative_to(rel_prefix) else reqif_path
    try:
        tree = ET.parse(reqif_path)
    except ET.ParseError:
        return 0
    root = tree.getroot()
    count = 0
    for elem in root.iter():
        tag = elem.tag.split("}", 1)[-1] if "}" in elem.tag else elem.tag
        if tag == "SPEC-OBJECT":
            ident = elem.attrib.get("LONG-NAME") or elem.attrib.get("IDENTIFIER")
            if not ident:
                continue
            canon_id = canonicalize_id(ident)
            desc = elem.attrib.get("DESC", "")
            loc_str = f"{rel_path}:1"
            if canon_id not in requirements:
                requirements[canon_id] = RequirementDef(
                    req_id=ident,
                    canonical_id=canon_id,
                    title=desc or ident,
                    framework="reqif",
                    layer="SWE.1",
                    directive_type="requirement",
                    is_normative=True,
                    locations=[loc_str],
                )
                count += 1
    return count


def is_test_path(rel_path: Path) -> bool:
    """Return True if the file path belongs to a verification test directory or filename."""
    parts_lower = [p.lower() for p in rel_path.parts]
    if any(p in ("test", "tests", "fuzz", "integration_tests", "network_tests", "unit_tests") for p in parts_lower):
        return True
    stem_lower = rel_path.stem.lower()
    return (
        stem_lower.startswith("test_")
        or stem_lower.endswith("_test")
        or stem_lower.endswith("_tests")
    )


def find_following_symbol(content: str, pos: int, is_rust: bool) -> tuple[str, bool]:
    """Identify the symbol following a comment/attribute block and whether it is a test item."""
    window = content[pos : pos + 500]
    if is_rust:
        item_m = re.search(
            r"\b(?:pub(?:\([^)]*\))?\s+)?(?:async\s+)?(?:unsafe\s+)?(?:fn|struct|enum|trait|mod|const|type)\s+([A-Za-z0-9_]+)",
            window,
        )
        prefix = window[: item_m.start()] if item_m else window[:120]
        is_rust_test = bool(re.search(r"#\[\s*(?:[\w:]+::)?test\b|#\[\s*kani::proof\b", prefix))
        if item_m:
            return item_m.group(1), is_rust_test
        return "rust_item", is_rust_test

    gtest_m = re.search(r"\bTEST(?:_F|_P)?\s*\(\s*(\w+)\s*,\s*(\w+)\s*\)", window)
    if gtest_m:
        return f"{gtest_m.group(1)}.{gtest_m.group(2)}", True
    py_m = re.search(r"\bdef\s+(test_[A-Za-z0-9_]+)\s*\(", window)
    if py_m:
        return py_m.group(1), True
    cpp_fn = re.search(r"(?:(\w+)::)?(\w+)\s*\(", window)
    if cpp_fn:
        return cpp_fn.group(2), False
    cpp_cls = re.search(r"\b(?:class|struct)\s+(\w+)", window)
    if cpp_cls:
        return cpp_cls.group(1), False
    return "item", False


def extract_ids_from_tag_line(line_val: str) -> list[str]:
    """Extract requirement ID tokens from the remainder of a tag line."""
    tokens: list[str] = []
    cleaned = line_val.split("//", 1)[0].split("*/", 1)[0].strip()
    for m in ID_TOKEN_RE.finditer(cleaned):
        tok = m.group(1)
        if tok.upper().endswith("_XXX"):
            continue
        if (
            tok.lower().startswith("feat_req_")
            or tok.upper().startswith(("REQ_", "SRS_", "SYS_", "ASR_", "FEAT_", "SCR-", "ARCH_"))
            or "." in tok
        ):
            tokens.append(canonicalize_id(tok))
    return tokens


def scan_source_and_test_files(
    repo_root: Path,
    alias_map: dict[str, str],
) -> tuple[list[CitationRef], set[str]]:
    """Scan C, C++, Rust, and Python files for structured traceability citations."""
    citations: list[CitationRef] = []
    detected_mechanisms: set[str] = set()
    code_exts = {".cpp", ".cc", ".cxx", ".hpp", ".h", ".c", ".rs", ".py"}

    for file_path in sorted(repo_root.rglob("*")):
        if not file_path.is_file():
            continue
        rel_path = file_path.relative_to(repo_root)
        if any(
            part.startswith(".") or part in ("build", "_build", "target", "node_modules", "scripts")
            for part in rel_path.parts
        ):
            continue

        suffix = file_path.suffix.lower()
        if suffix not in code_exts:
            continue

        content = file_path.read_text(encoding="utf-8", errors="ignore")
        file_is_test = is_test_path(rel_path)
        is_rust = suffix == ".rs"

        rust_cfg_test_offset = len(content) + 1
        if is_rust:
            cfg_m = re.search(r"^\s*#\[\s*cfg\s*\(\s*test\s*\)\s*\]", content, re.MULTILINE)
            if cfg_m:
                rust_cfg_test_offset = cfg_m.start()

        blocks: list[tuple[int, int, str]] = []
        if suffix == ".py":
            for m in re.finditer(r'"""(.+?)"""|\'\'\'(.+?)\'\'\'', content, re.DOTALL):
                blocks.append((m.start(), m.end(), m.group(1) or m.group(2)))
        else:
            for m in re.finditer(r"/\*\*(.+?)\*/", content, re.DOTALL):
                blocks.append((m.start(), m.end(), m.group(1)))
            for m in re.finditer(r"(?:^[ \t]*//[/!][^\n]*\n)+", content, re.MULTILINE):
                blocks.append((m.start(), m.end(), m.group(0)))
            if is_rust:
                for m in RUST_DOC_ATTR_RE.finditer(content):
                    blocks.append((m.start(), m.end(), m.group(1)))

        for start, end, block_text in blocks:
            impl_ids: list[str] = []
            for m in IMPLEMENTS_TAG_RE.finditer(block_text):
                impl_ids.extend(extract_ids_from_tag_line(m.group(1)))
            sat_ids: list[str] = []
            for m in SATISFIES_TAG_RE.finditer(block_text):
                sat_ids.extend(extract_ids_from_tag_line(m.group(1)))
            req_ids = [canonicalize_id(m.group(1)) for m in DOXYGEN_REQ_TAG_RE.finditer(block_text)]
            test_ids: list[str] = []
            for m in TESTS_TAG_RE.finditer(block_text):
                test_ids.extend(extract_ids_from_tag_line(m.group(1)))
            tc_ids = [canonicalize_id(m.group(1)) for m in TEST_CASE_TAG_RE.finditer(block_text)]

            if not (impl_ids or sat_ids or req_ids or test_ids or tc_ids):
                continue

            line_num = content[:start].count("\n") + 1
            sym_name, sym_is_test = find_following_symbol(content, end, is_rust)
            in_test_context = file_is_test or sym_is_test or (is_rust and start >= rust_cfg_test_offset)
            detected_mechanisms.add("rust-doc-annotations" if is_rust else "doxygen-tags")

            if test_ids or tc_ids or (in_test_context and (impl_ids or sat_ids)):
                combined_targets = [alias_map.get(t, t) for t in (test_ids + impl_ids + sat_ids)]
                if combined_targets:
                    citations.append(
                        CitationRef(
                            ref_id=tc_ids[0] if tc_ids else f"TC_{file_path.stem}_{line_num}",
                            file_path=str(rel_path),
                            line_number=line_num,
                            symbol_name=sym_name,
                            role="test",
                            tag_kind="@tests",
                            target_ids=combined_targets,
                        )
                    )

            if (impl_ids or sat_ids or req_ids) and not in_test_context:
                combined_code = [alias_map.get(t, t) for t in (impl_ids + sat_ids + req_ids)]
                tag_label = "@req" if req_ids and not (impl_ids or sat_ids) else "@implements/@satisfies"
                citations.append(
                    CitationRef(
                        ref_id=f"CODE_{file_path.stem}_{line_num}",
                        file_path=str(rel_path),
                        line_number=line_num,
                        symbol_name=sym_name,
                        role="code",
                        tag_kind=tag_label,
                        target_ids=combined_code,
                    )
                )

        for m in GTEST_RECORD_PROP_RE.finditer(content):
            prop_kind, raw_targets = m.group(1), m.group(2)
            line_num = content[: m.start()].count("\n") + 1
            targets = [
                alias_map.get(canonicalize_id(t), canonicalize_id(t))
                for t in re.split(r"[,\s]+", raw_targets)
                if t.strip()
            ]
            if targets:
                detected_mechanisms.add(f"gtest-{prop_kind.lower()}")
                citations.append(
                    CitationRef(
                        ref_id=f"TEST_{file_path.stem}_{line_num}",
                        file_path=str(rel_path),
                        line_number=line_num,
                        symbol_name=f"RecordProperty({prop_kind})",
                        role="test",
                        tag_kind=f"RecordProperty:{prop_kind}",
                        target_ids=targets,
                    )
                )

        for m in MANTRA_ATTR_RE.finditer(content):
            line_num = content[: m.start()].count("\n") + 1
            sym_name, sym_is_test = find_following_symbol(content, m.end(), is_rust)
            in_test_ctx = file_is_test or sym_is_test or (is_rust and m.start() >= rust_cfg_test_offset)
            targets = [
                alias_map.get(canonicalize_id(t.strip("\"'")), canonicalize_id(t.strip("\"'")))
                for t in m.group(1).split(",")
                if t.strip()
            ]
            if targets:
                detected_mechanisms.add("mantra-proc-macro")
                citations.append(
                    CitationRef(
                        ref_id=f"MANTRA_{file_path.stem}_{line_num}",
                        file_path=str(rel_path),
                        line_number=line_num,
                        symbol_name=sym_name,
                        role="test" if in_test_ctx else "code",
                        tag_kind="#[req(...)]",
                        target_ids=targets,
                    )
                )

        for m in SDOC_MARKER_RE.finditer(content):
            uid = m.group(1)
            if uid.startswith("/"):
                continue
            line_num = content[: m.start()].count("\n") + 1
            in_test_ctx = file_is_test or (is_rust and m.start() >= rust_cfg_test_offset)
            canon_uid = alias_map.get(canonicalize_id(uid), canonicalize_id(uid))
            detected_mechanisms.add("strictdoc-markers")
            citations.append(
                CitationRef(
                    ref_id=f"SDOC_{file_path.stem}_{line_num}",
                    file_path=str(rel_path),
                    line_number=line_num,
                    symbol_name="sdoc_range",
                    role="test" if in_test_ctx else "code",
                    tag_kind="@sdoc",
                    target_ids=[canon_uid],
                )
            )

    return citations, detected_mechanisms


@dataclass
class AuditResult:
    """Aggregated results of a repository traceability audit."""

    repo_root: Path
    spec_dir: Path | None
    detected_frameworks: list[str]
    internal_reqs: dict[str, RequirementDef]
    external_reqs: dict[str, RequirementDef]
    citations: list[CitationRef]
    doc_example_leaks: list[str]
    duplicate_ids: dict[str, list[str]]
    dangling_spec_links: list[tuple[str, str]]
    dangling_code_refs: list[tuple[str, str]]
    dangling_test_refs: list[tuple[str, str]]
    code_by_req: dict[str, list[str]]
    test_by_req: dict[str, list[str]]
    satisfied_by_reqs: dict[str, list[str]]


def run_audit(repo_root: Path, spec_dir: Path | None = None) -> AuditResult:
    """Execute a full bidirectional traceability audit on repo_root and optional spec_dir."""
    repo_root = repo_root.resolve()
    spec_dir = spec_dir.resolve() if spec_dir else None

    detected_frameworks: set[str] = set()
    internal_reqs: dict[str, RequirementDef] = {}
    external_reqs: dict[str, RequirementDef] = {}
    alias_map: dict[str, str] = {}
    doc_example_leaks: list[str] = []

    for path in sorted(repo_root.rglob("*")):
        if not path.is_file():
            continue
        rel = path.relative_to(repo_root)
        if any(p.startswith(".") and p != ".doorstop.yml" for p in rel.parts):
            continue
        if any(p in ("build", "_build", "target", "node_modules") for p in rel.parts):
            continue

        if path.name == "conf.py":
            if "sphinx_needs" in path.read_text(encoding="utf-8", errors="ignore"):
                detected_frameworks.add("sphinx-needs")
        elif path.suffix.lower() == ".rst":
            if parse_rst_file(path, repo_root, "SWE.1", internal_reqs, doc_example_leaks) > 0:
                detected_frameworks.add("sphinx-needs")
        elif path.suffix.lower() == ".rsl":
            detected_frameworks.add("trlc")
        elif path.suffix.lower() == ".trlc":
            if parse_trlc_file(path, repo_root, internal_reqs, alias_map) > 0:
                detected_frameworks.add("trlc")
        elif path.suffix.lower() == ".sdoc" or path.name == "strictdoc.toml":
            detected_frameworks.add("strictdoc")
            if path.suffix.lower() == ".sdoc":
                parse_sdoc_file(path, repo_root, internal_reqs)
        elif path.name == ".doorstop.yml":
            detected_frameworks.add("doorstop")
            parse_doorstop_dir(path, repo_root, internal_reqs)
        elif path.suffix.lower() == ".reqif":
            if parse_reqif_file(path, repo_root, internal_reqs) > 0:
                detected_frameworks.add("reqif")
        elif path.name in ("doxygen.in", "Doxyfile"):
            detected_frameworks.add("doxygen-config")

    if spec_dir and spec_dir.exists() and spec_dir != repo_root:
        for rst_path in sorted(spec_dir.rglob("*.rst")):
            if parse_rst_file(rst_path, spec_dir, "SYS.2", external_reqs, doc_example_leaks) > 0:
                detected_frameworks.add("external-sphinx-needs-spec")

    citations, code_mechanisms = scan_source_and_test_files(repo_root, alias_map)
    detected_frameworks.update(code_mechanisms)

    all_known_reqs: dict[str, RequirementDef] = {}
    all_known_reqs.update(external_reqs)
    all_known_reqs.update(internal_reqs)

    code_by_req: dict[str, list[str]] = defaultdict(list)
    test_by_req: dict[str, list[str]] = defaultdict(list)
    satisfied_by_reqs: dict[str, list[str]] = defaultdict(list)

    dangling_spec_links: list[tuple[str, str]] = []
    dangling_code_refs: list[tuple[str, str]] = []
    dangling_test_refs: list[tuple[str, str]] = []

    for req_id, req_def in sorted(internal_reqs.items()):
        for up_raw in req_def.satisfies:
            up_id = alias_map.get(up_raw, up_raw)
            satisfied_by_reqs[up_id].append(req_id)
            if up_id.startswith(("feat_req_", "SYS_")) and external_reqs and up_id not in external_reqs:
                dangling_spec_links.append((f"{req_id} ({req_def.locations[0]})", up_id))
            elif not up_id.startswith(("feat_req_", "SYS_")) and up_id not in all_known_reqs:
                dangling_spec_links.append((f"{req_id} ({req_def.locations[0]})", up_id))

    for cit in citations:
        loc_tag = f"{cit.file_path}:{cit.line_number}"
        for target in cit.target_ids:
            resolved = alias_map.get(target, target)
            if cit.role == "code":
                if loc_tag not in code_by_req[resolved]:
                    code_by_req[resolved].append(loc_tag)
                if resolved in internal_reqs:
                    for up_id in internal_reqs[resolved].satisfies:
                        up_resolved = alias_map.get(up_id, up_id)
                        via_tag = f"{loc_tag} (via {resolved})"
                        if via_tag not in code_by_req[up_resolved]:
                            code_by_req[up_resolved].append(via_tag)
                if all_known_reqs and resolved not in all_known_reqs:
                    if not (resolved.startswith(("feat_req_", "SYS_")) and not external_reqs):
                        dangling_code_refs.append((loc_tag, resolved))
            else:
                if loc_tag not in test_by_req[resolved]:
                    test_by_req[resolved].append(loc_tag)
                if resolved in internal_reqs:
                    for up_id in internal_reqs[resolved].satisfies:
                        up_resolved = alias_map.get(up_id, up_id)
                        via_tag = f"{loc_tag} (via {resolved})"
                        if via_tag not in test_by_req[up_resolved]:
                            test_by_req[up_resolved].append(via_tag)
                if all_known_reqs and resolved not in all_known_reqs:
                    if not (resolved.startswith(("feat_req_", "SYS_")) and not external_reqs):
                        dangling_test_refs.append((loc_tag, resolved))

    duplicate_ids: dict[str, list[str]] = {
        req_id: req_def.locations
        for req_id, req_def in sorted(all_known_reqs.items())
        if len(req_def.locations) > 1
    }

    return AuditResult(
        repo_root=repo_root,
        spec_dir=spec_dir,
        detected_frameworks=sorted(detected_frameworks),
        internal_reqs=internal_reqs,
        external_reqs=external_reqs,
        citations=citations,
        doc_example_leaks=doc_example_leaks,
        duplicate_ids=duplicate_ids,
        dangling_spec_links=dangling_spec_links,
        dangling_code_refs=dangling_code_refs,
        dangling_test_refs=dangling_test_refs,
        code_by_req=code_by_req,
        test_by_req=test_by_req,
        satisfied_by_reqs=satisfied_by_reqs,
    )


def classify_status(
    req_def: RequirementDef,
    code_refs: list[str],
    test_refs: list[str],
    sat_by_reqs: list[str],
) -> str:
    """Classify the bidirectional traceability status of a requirement node."""
    has_code, has_test, has_req_link = bool(code_refs), bool(test_refs), bool(sat_by_reqs)
    if not req_def.is_normative:
        return "NON_NORMATIVE_LINKED" if (has_code or has_test or has_req_link) else "NON_NORMATIVE_INFO"
    if has_code and has_test:
        return "FULLY_TRACED"
    if has_code:
        return "IMPLEMENTED_UNTESTED"
    if has_test:
        return "TESTED_UNIMPLEMENTED"
    if has_req_link:
        return "SPEC_LINKED_ONLY"
    return "ORPHAN_UNIMPLEMENTED"


def generate_markdown_report(result: AuditResult) -> str:
    """Format a Markdown traceability audit report."""
    lines: list[str] = [
        "# Automotive Requirements Traceability Audit Report",
        "",
        f"- **Target Repository**: `{result.repo_root}`",
        (
            f"- **External Specification Corpus**: `{result.spec_dir}`"
            if result.spec_dir
            else "- **External Specification Corpus**: None (`--spec-dir` not specified)"
        ),
        f"- **Detected Frameworks & Mechanisms**: {', '.join(f'`{fw}`' for fw in result.detected_frameworks) or '`none-detected`'}",
        "",
    ]

    int_all = list(result.internal_reqs.values())
    int_norm = [r for r in int_all if r.is_normative]
    int_full = [r for r in int_norm if result.code_by_req.get(r.canonical_id) and result.test_by_req.get(r.canonical_id)]
    int_code_only = [r for r in int_norm if result.code_by_req.get(r.canonical_id) and not result.test_by_req.get(r.canonical_id)]
    int_test_only = [r for r in int_norm if not result.code_by_req.get(r.canonical_id) and result.test_by_req.get(r.canonical_id)]
    int_orphan = [r for r in int_norm if not result.code_by_req.get(r.canonical_id) and not result.test_by_req.get(r.canonical_id)]

    lines.extend([
        "## 1. Bidirectional Traceability Summary (`SWE.1`–`SWE.6`)",
        "",
        "| Metric | Count | Percentage (of Normative) |",
        "|---|---:|---:|",
        f"| Total Internal Spec Nodes (`SWE.1`/`SWE.2`) | {len(int_all)} | - |",
        f"| Normative Internal Requirements | {len(int_norm)} | 100.0% |",
    ])
    denom = len(int_norm) or 1
    lines.append(f"| Fully Traced (Code `SWE.3` + Test `SWE.4`–`SWE.6`) | {len(int_full)} | {(len(int_full) / denom * 100.0 if int_norm else 0.0):.1f}% |")
    lines.append(f"| Implemented Only (Missing Test `SWE.4`–`SWE.6`) | {len(int_code_only)} | {(len(int_code_only) / denom * 100.0 if int_norm else 0.0):.1f}% |")
    lines.append(f"| Tested Only (Missing Code Annotation `SWE.3`) | {len(int_test_only)} | {(len(int_test_only) / denom * 100.0 if int_norm else 0.0):.1f}% |")
    lines.append(f"| Orphan / Unimplemented Requirements | {len(int_orphan)} | {(len(int_orphan) / denom * 100.0 if int_norm else 0.0):.1f}% |")
    lines.append(f"| Extracted Code Citations (`SWE.3`) | {sum(1 for c in result.citations if c.role == 'code')} | - |")
    lines.append(f"| Extracted Test Citations (`SWE.4`–`SWE.6`) | {sum(1 for c in result.citations if c.role == 'test')} | - |")
    lines.append(f"| Duplicate Requirement ID Definitions | {len(result.duplicate_ids)} | - |")
    total_dangling = len(result.dangling_spec_links) + len(result.dangling_code_refs) + len(result.dangling_test_refs)
    lines.append(f"| Dangling References (Spec + Code + Test) | {total_dangling} | - |")
    lines.append("")

    if result.external_reqs:
        ext_all = list(result.external_reqs.values())
        ext_norm = [r for r in ext_all if r.is_normative]
        ext_norm_full = [
            r for r in ext_norm if result.code_by_req.get(r.canonical_id) and result.test_by_req.get(r.canonical_id)
        ]
        lines.extend([
            "## 2. External Specification Coverage (`SYS.2`)",
            "",
            "| Specification Metric | Count |",
            "|---|---:|",
            f"| Total External `:id:` Nodes | {len(ext_all)} |",
            f"| Normative Requirements (`:reqtype: Requirement`) | {len(ext_norm)} |",
            f"| Normative External Requirements Fully Traced (Code + Test) | {len(ext_norm_full)} |",
            "",
        ])

    lines.extend(["## 3. Traceability Integrity Findings", ""])
    if result.duplicate_ids:
        lines.append(f"### 3.1 Duplicate Requirement Definitions ({len(result.duplicate_ids)} found)")
        for dup_id, locs in sorted(result.duplicate_ids.items()):
            lines.append(f"- `{dup_id}` defined {len(locs)} times at: {', '.join(f'`{l}`' for l in locs)}")
        lines.append("")
    else:
        lines.extend(["### 3.1 Duplicate Requirement Definitions: None (0)", ""])

    if total_dangling:
        lines.append(f"### 3.2 Dangling / Unknown Requirement References ({total_dangling} found)")
        for src_loc, target_id in result.dangling_spec_links[:25]:
            lines.append(f"- **Spec `:satisfies:` Dangling Link**: `{src_loc}` references non-existent `{target_id}`")
        for src_loc, target_id in result.dangling_code_refs[:25]:
            lines.append(f"- **Code `SWE.3` Dangling Link**: `{src_loc}` references non-existent `{target_id}`")
        for src_loc, target_id in result.dangling_test_refs[:25]:
            lines.append(f"- **Test `SWE.4-6` Dangling Link**: `{src_loc}` references non-existent `{target_id}`")
        lines.append("")
    else:
        lines.extend(["### 3.2 Dangling / Unknown Requirement References: None (0)", ""])

    return "\n".join(lines) + "\n"


def write_csv_matrix(result: AuditResult, csv_path: Path) -> int:
    """Write the complete bidirectional traceability matrix to CSV."""
    csv_path.parent.mkdir(parents=True, exist_ok=True)
    combined: list[RequirementDef] = []
    seen: set[str] = set()
    for r in list(result.internal_reqs.values()) + list(result.external_reqs.values()):
        if r.canonical_id not in seen:
            seen.add(r.canonical_id)
            combined.append(r)

    rows_written = 0
    with csv_path.open("w", encoding="utf-8", newline="") as fh:
        writer = csv.writer(fh)
        writer.writerow([
            "requirement_id",
            "layer",
            "framework",
            "directive_type",
            "normative",
            "status",
            "spec_locations",
            "upstream_satisfies",
            "satisfied_by_reqs",
            "code_refs",
            "test_refs",
        ])
        for req in sorted(combined, key=lambda x: x.canonical_id):
            c_refs = result.code_by_req.get(req.canonical_id, [])
            t_refs = result.test_by_req.get(req.canonical_id, [])
            s_reqs = result.satisfied_by_reqs.get(req.canonical_id, [])
            writer.writerow([
                req.req_id,
                req.layer,
                req.framework,
                req.directive_type,
                "true" if req.is_normative else "false",
                classify_status(req, c_refs, t_refs, s_reqs),
                "; ".join(req.locations),
                "; ".join(req.satisfies),
                "; ".join(s_reqs),
                "; ".join(c_refs),
                "; ".join(t_refs),
            ])
            rows_written += 1
    return rows_written


def run_self_test() -> int:
    """Verify parser and bidirectional audit logic on a synthetic multi-framework fixture."""
    with tempfile.TemporaryDirectory(prefix="traceability_audit_selftest_") as tmp_dir:
        root = Path(tmp_dir)
        spec_dir = root / "external_spec"
        repo_dir = root / "sample_repo"
        spec_dir.mkdir(parents=True)
        (repo_dir / "docs" / "requirements").mkdir(parents=True)
        (repo_dir / "src").mkdir(parents=True)
        (repo_dir / "tests").mkdir(parents=True)

        (spec_dir / "protocol.rst").write_text(
            """
.. heading:: Header Structure
    :id: SYS_PROTO_029

.. spec_req:: Info Note
    :id: SYS_PROTO_030
    :reqtype: Information

.. spec_req:: Message ID Layout
    :id: SYS_PROTO_045
    :reqtype: Requirement

.. spec_req:: Minimum Length 8
    :id: SYS_PROTO_060
    :reqtype: Requirement

.. spec_req:: Unimplemented Spec Req
    :id: SYS_PROTO_999
    :reqtype: Requirement
""",
            encoding="utf-8",
        )

        (repo_dir / "docs" / "requirements" / "conf.py").write_text(
            'extensions = ["sphinx_needs"]\n', encoding="utf-8"
        )
        (repo_dir / "docs" / "requirements" / "reqs.rst").write_text(
            """
.. code-block:: rst

   .. requirement:: Example Only
      :id: REQ_EXAMPLE_999
      :satisfies: SYS_PROTO_045

.. requirement:: Parse Message ID
   :id: REQ_MSG_001
   :satisfies: SYS_PROTO_045

.. requirement:: Validate Length Field
   :id: REQ_MSG_002
   :satisfies: SYS_PROTO_060

.. requirement:: Duplicate ID Check
   :id: REQ_MSG_002
   :satisfies: SYS_PROTO_060

.. requirement:: Orphan Requirement With Dangling Spec Link
   :id: REQ_MSG_003
   :satisfies: SYS_PROTO_8888
""",
            encoding="utf-8",
        )

        (repo_dir / "src" / "lib.rs").write_text(
            """
/// Parses the 32-bit wire Message ID.
///
/// @implements REQ_MSG_001
/// @satisfies SYS_PROTO_045
pub fn parse_message_id(buf: &[u8]) -> Option<u32> {
    if buf.len() < 4 { return None; }
    Some(u32::from_be_bytes([buf[0], buf[1], buf[2], buf[3]]))
}

/// Validates minimum length (implemented, missing unit test).
///
/// @implements REQ_MSG_002, REQ_DANGLING_CODE_404
pub fn validate_length(len: u32) -> bool {
    len >= 8
}

#[cfg(test)]
mod tests {
    use super::*;

    /// Unit test for Message ID parsing.
    ///
    /// @test_case TC_MSG_001
    /// @tests REQ_MSG_001, REQ_DANGLING_TEST_505
    /// @tests SYS_PROTO_045
    #[test]
    fn test_parse_message_id() {
        assert_eq!(parse_message_id(&[0x12, 0x34, 0x00, 0x01]), Some(0x12340001));
    }
}
""",
            encoding="utf-8",
        )

        res = run_audit(repo_dir, spec_dir)
        md_out = root / "report.md"
        csv_out = root / "matrix.csv"
        md_out.write_text(generate_markdown_report(res), encoding="utf-8")
        rows = write_csv_matrix(res, csv_out)

        assert "REQ_EXAMPLE_999" not in res.internal_reqs
        assert len(res.doc_example_leaks) == 1
        assert "REQ_MSG_002" in res.duplicate_ids
        assert any(t == "SYS_PROTO_8888" for _, t in res.dangling_spec_links)
        assert any(t == "REQ_DANGLING_CODE_404" for _, t in res.dangling_code_refs)
        assert any(t == "REQ_DANGLING_TEST_505" for _, t in res.dangling_test_refs)
        assert res.code_by_req["REQ_MSG_001"] and res.test_by_req["REQ_MSG_001"]
        assert res.code_by_req["REQ_MSG_002"] and not res.test_by_req["REQ_MSG_002"]
        assert not res.code_by_req["REQ_MSG_003"] and not res.test_by_req["REQ_MSG_003"]
        assert md_out.exists() and csv_out.exists() and rows == 8

    print(
        "[SELF-TEST OK] Verified multi-file Rust/RST fixture: stripped doc-example leaks, "
        "detected duplicate IDs, dangling spec/code/test links, and emitted valid Markdown and CSV reports."
    )
    return 0


def build_arg_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        prog="audit_traceability.py",
        description=(
            "Discover requirements traceability systems (Sphinx-Needs, TRLC, StrictDoc, "
            "Doorstop, Mantra, ReqIF, GTest RecordProperty, Doxygen, Rust doc-attributes) "
            "and audit bidirectional ASPICE SWE.1-SWE.6 & ISO 26262-6 coverage."
        ),
    )
    parser.add_argument("positional_repo", nargs="?", default=None, help="Optional positional repository path.")
    parser.add_argument("--repo", type=Path, default=None, help="Target repository root directory (default: .).")
    parser.add_argument("--spec-dir", type=Path, default=None, help="Optional external specification directory.")
    parser.add_argument("--markdown-out", "--output", "-o", dest="markdown_out", type=Path, default=None)
    parser.add_argument("--csv-out", "--csv", dest="csv_out", type=Path, default=None)
    parser.add_argument("--self-test", action="store_true", help="Run built-in self-test suite and exit.")
    parser.add_argument(
        "--strict",
        action="store_true",
        help="Exit 1 if duplicate IDs or dangling spec/code/test references are found.",
    )
    return parser


def main(argv: list[str] | None = None) -> int:
    parser = build_arg_parser()
    args = parser.parse_args(argv)
    if args.self_test:
        return run_self_test()

    repo_path = args.repo or (Path(args.positional_repo) if args.positional_repo else Path.cwd())
    if not repo_path.exists():
        print(f"Error: repository path does not exist: {repo_path}", file=sys.stderr)
        return 2

    result = run_audit(repo_path, args.spec_dir)
    md_report = generate_markdown_report(result)
    if args.markdown_out:
        args.markdown_out.parent.mkdir(parents=True, exist_ok=True)
        args.markdown_out.write_text(md_report, encoding="utf-8")
        print(f"Wrote Markdown traceability report to {args.markdown_out}")
    else:
        print(md_report, end="")

    if args.csv_out:
        row_count = write_csv_matrix(result, args.csv_out)
        print(f"Wrote CSV traceability matrix ({row_count} rows) to {args.csv_out}")

    if args.strict and (
        result.duplicate_ids
        or result.dangling_spec_links
        or result.dangling_code_refs
        or result.dangling_test_refs
    ):
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
