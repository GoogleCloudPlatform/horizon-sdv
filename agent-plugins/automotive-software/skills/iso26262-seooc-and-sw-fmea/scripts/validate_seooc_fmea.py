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
"""ISO 26262-10 Clause 9 SEooC & ISO 26262-6 Clause 7 SW-FMEA Validator.

Validates closed-loop traceability and structural completeness across:
1. TRLC (`.trlc`) SEooC & SW-FMEA specifications (`AssumedSystemReq`,
   `FeatReq`, `AoU`, `FailureMode`).
2. Structured JSON, YAML, or Markdown SEooC Safety Manual & SW-FMEA worksheets.
3. Automotive Rust crate public API boundaries (`pub fn`, `pub trait`,
   `unsafe fn`, `extern "C" fn`), checking `# Safety Requirements` (`@satisfies
   ASR-*`), `# Failure Modes` (`@fmea FM-*`), `# Assumptions of Use` (`@aou
   AoU-*`), and `#[must_use]` error handling contracts.

Uses only the Python 3 standard library.
"""

from __future__ import annotations

import argparse
from dataclasses import asdict, dataclass, field
import json
import pathlib
import re
import sys
import tempfile
from typing import Any

VALID_GUIDEWORDS = {
    "LossOfFunction",
    "UnintendedFunction",
    "DelayedFunction",
    "EarlyOrLateExecution",
    "PartialFunction",
    "PartialOrCorruptedExecution",
    "ExceedingFunction",
    "IncorrectAsil",
}

VALID_ASILS = {
    "QM",
    "ASIL-A",
    "ASIL-B",
    "ASIL-C",
    "ASIL-D",
    "A",
    "B",
    "C",
    "D",
}


@dataclass
class AssumedSafetyReq:
  req_id: str
  asil: str
  description: str
  rationale: str = ""
  verification_tests: list[str] = field(default_factory=list)
  derived_from: list[str] = field(default_factory=list)
  source_file: str = ""
  line: int = 1


@dataclass
class AssumptionOfUse:
  aou_id: str
  asil: str
  description: str
  note: str = ""
  mitigates: list[str] = field(default_factory=list)
  integrator_verification: str = ""
  source_file: str = ""
  line: int = 1


@dataclass
class FailureModeRecord:
  fm_id: str
  guidewords: list[str]
  asil: str
  description: str
  failure_effect: str
  rationale: str = ""
  interface: str = ""
  section_path: str = ""
  mitigated_by: list[str] = field(default_factory=list)
  verification_test: str = ""
  source_file: str = ""
  line: int = 1


@dataclass
class RustPublicBoundary:
  item_kind: str  # "pub_fn", "trait_fn", "unsafe_fn", "extern_c_fn", "pub_trait"
  name: str
  qualified_name: str
  returns_result: bool
  has_must_use: bool
  has_safety_doc: bool
  asr_refs: list[str] = field(default_factory=list)
  fm_refs: list[str] = field(default_factory=list)
  aou_refs: list[str] = field(default_factory=list)
  source_file: str = ""
  line: int = 1


@dataclass
class ValidationFinding:
  rule_id: str
  severity: str  # "ERROR", "WARNING", "INFO"
  category: str
  item_id: str
  message: str
  remediation: str
  source_file: str = ""
  line: int = 1


@dataclass
class AuditSummary:
  target_path: str
  rust_crate_path: str
  asr_count: int
  feat_req_count: int
  aou_count: int
  failure_mode_count: int
  guideword_distribution: dict[str, int]
  rust_boundaries_total: int
  rust_boundaries_covered: int
  closed_loop_fm_count: int
  unmitigated_fm_count: int
  unlinked_aou_count: int
  error_count: int
  warning_count: int
  findings: list[ValidationFinding] = field(default_factory=list)
  asrs: list[AssumedSafetyReq] = field(default_factory=list)
  aous: list[AssumptionOfUse] = field(default_factory=list)
  failure_modes: list[FailureModeRecord] = field(default_factory=list)
  rust_boundaries: list[RustPublicBoundary] = field(default_factory=list)


def _strip_trlc_comments(text: str) -> str:
  """Removes // and /* */ comments while preserving string literals and newlines."""
  out: list[str] = []
  i = 0
  n = len(text)
  in_str = False
  triple_str = False
  while i < n:
    if not in_str:
      if text.startswith('"""', i):
        in_str = True
        triple_str = True
        out.append('"""')
        i += 3
        continue
      if text[i] == '"':
        in_str = True
        triple_str = False
        out.append('"')
        i += 1
        continue
      if text.startswith("//", i):
        while i < n and text[i] != "\n":
          i += 1
        continue
      if text.startswith("/*", i):
        end = text.find("*/", i + 2)
        if end == -1:
          break
        newlines = text[i : end + 2].count("\n")
        out.append("\n" * newlines)
        i = end + 2
        continue
      out.append(text[i])
      i += 1
    else:
      if triple_str and text.startswith('"""', i):
        in_str = False
        triple_str = False
        out.append('"""')
        i += 3
        continue
      if not triple_str and text[i] == '"' and (i == 0 or text[i - 1] != "\\"):
        in_str = False
        out.append('"')
        i += 1
        continue
      out.append(text[i])
      i += 1
  return "".join(out)


def _extract_trlc_fields(body: str) -> dict[str, Any]:
  """Extracts key = value attributes from a TRLC record body."""
  fields: dict[str, Any] = {}
  # Triple-quoted strings
  for m in re.finditer(r'(\w+)\s*=\s*"""(.*?)"""', body, re.DOTALL):
    fields[m.group(1)] = m.group(2).strip()
  # Single-quoted strings (skipping already matched triple-quoted keys)
  for m in re.finditer(r'(\w+)\s*=\s*"((?:[^"\\]|\\.)*)"', body, re.DOTALL):
    key = m.group(1)
    if key not in fields:
      fields[key] = m.group(2).strip()
  # Bracketed lists
  for m in re.finditer(r"(\w+)\s*=\s*\[(.*?)\]", body, re.DOTALL):
    key = m.group(1)
    items = [
        tok.strip()
        for tok in re.split(r"[, \n\r\t]+", m.group(2).strip())
        if tok.strip()
    ]
    fields[key] = items
  # Bare identifiers / numbers
  for m in re.finditer(r"(\w+)\s*=\s*([A-Za-z0-9_.@]+)", body):
    key = m.group(1)
    if key not in fields:
      fields[key] = m.group(2).strip()
  return fields


def parse_trlc_file(
    path: pathlib.Path,
) -> tuple[
    list[AssumedSafetyReq],
    list[AssumedSafetyReq],
    list[AssumptionOfUse],
    list[FailureModeRecord],
]:
  """Parses a TRLC (.trlc) file into ASR, FeatReq, AoU, and FailureMode records."""
  raw_text = path.read_text(encoding="utf-8", errors="replace")
  clean_text = _strip_trlc_comments(raw_text)

  asrs: list[AssumedSafetyReq] = []
  feat_reqs: list[AssumedSafetyReq] = []
  aous: list[AssumptionOfUse] = []
  fms: list[FailureModeRecord] = []

  header_re = re.compile(
      r"\b(?:[A-Za-z_][A-Za-z0-9_]*\.)?(AssumedSystemReq|FeatReq|AoU|FailureMode)\s+([A-Za-z0-9_]+)\s*\{",
      re.MULTILINE,
  )

  for m in header_re.finditer(clean_text):
    kind = m.group(1)
    rec_id = m.group(2)
    start_brace = m.end() - 1
    line_no = clean_text.count("\n", 0, m.start()) + 1

    depth = 0
    end_brace = start_brace
    in_str = False
    triple = False
    i = start_brace
    n = len(clean_text)
    while i < n:
      if not in_str:
        if clean_text.startswith('"""', i):
          in_str = True
          triple = True
          i += 3
          continue
        if clean_text[i] == '"':
          in_str = True
          triple = False
          i += 1
          continue
        if clean_text[i] == "{":
          depth += 1
        elif clean_text[i] == "}":
          depth -= 1
          if depth == 0:
            end_brace = i
            break
      else:
        if triple and clean_text.startswith('"""', i):
          in_str = False
          triple = False
          i += 3
          continue
        if (
            not triple
            and clean_text[i] == '"'
            and (i == 0 or clean_text[i - 1] != "\\")
        ):
          in_str = False
      i += 1

    body = clean_text[start_brace + 1 : end_brace]
    attrs = _extract_trlc_fields(body)
    asil = re.sub(r"^[A-Za-z_][A-Za-z0-9_]*\.Asil\.", "ASIL-", str(attrs.get("safety", "QM")))
    if asil == "ASIL-QM":
      asil = "QM"

    if kind == "AssumedSystemReq":
      asrs.append(
          AssumedSafetyReq(
              req_id=rec_id,
              asil=asil,
              description=str(attrs.get("description", "")),
              rationale=str(attrs.get("rationale", "")),
              source_file=str(path),
              line=line_no,
          )
      )
    elif kind == "FeatReq":
      derived = attrs.get("derived_from", [])
      if isinstance(derived, str):
        derived = [derived]
      feat_reqs.append(
          AssumedSafetyReq(
              req_id=rec_id,
              asil=asil,
              description=str(attrs.get("description", "")),
              rationale="Derived feature requirement",
              derived_from=list(derived),
              source_file=str(path),
              line=line_no,
          )
      )
    elif kind == "AoU":
      raw_mit = str(attrs.get("mitigates", "")).strip()
      mit_list = [
          tok.strip().removeprefix("Communication.")
          for tok in raw_mit.split(",")
          if tok.strip()
      ]
      aous.append(
          AssumptionOfUse(
              aou_id=rec_id,
              asil=asil,
              description=str(attrs.get("description", "")),
              note=str(attrs.get("note", "")),
              mitigates=mit_list,
              integrator_verification=str(attrs.get("note", "")),
              source_file=str(path),
              line=line_no,
          )
      )
    elif kind == "FailureMode":
      raw_gws = attrs.get("guidewords", [])
      if isinstance(raw_gws, str):
        raw_gws = [raw_gws]
      gws = [re.sub(r"^[A-Za-z_][A-Za-z0-9_]*\.Guideword\.", "", str(g)) for g in raw_gws]
      fms.append(
          FailureModeRecord(
              fm_id=rec_id,
              guidewords=gws,
              asil=asil,
              description=str(attrs.get("description", "")),
              failure_effect=str(attrs.get("failureeffect", "")),
              rationale=str(attrs.get("rationale", "")),
              interface=str(attrs.get("interface", "")),
              source_file=str(path),
              line=line_no,
          )
      )

  return asrs, feat_reqs, aous, fms


def _parse_simple_yaml_or_json(path: pathlib.Path) -> dict[str, Any]:
  """Parses JSON or simple structured YAML using stdlib only."""
  text = path.read_text(encoding="utf-8", errors="replace").strip()
  if text.startswith("{"):
    return json.loads(text)

  # Minimal indentation-based parser for the canonical seooc_fmea.yaml schema
  result: dict[str, Any] = {
      "assumed_safety_requirements": [],
      "assumptions_of_use": [],
      "failure_modes": [],
  }
  current_section: str | None = None
  current_item: dict[str, Any] | None = None
  current_list_key: str | None = None

  for raw_line in text.splitlines():
    stripped = raw_line.strip()
    if not stripped or stripped.startswith("#"):
      continue
    indent = len(raw_line) - len(raw_line.lstrip(" "))

    if indent == 0 and stripped.endswith(":"):
      sec_name = stripped[:-1].strip()
      if sec_name in result:
        current_section = sec_name
      else:
        current_section = None
      current_item = None
      current_list_key = None
      continue

    if current_section is None:
      continue

    if stripped.startswith("- ") and indent <= 4:
      current_item = {}
      result[current_section].append(current_item)
      current_list_key = None
      rem = stripped[2:].strip()
      if ":" in rem:
        k, v = rem.split(":", 1)
        current_item[k.strip()] = v.strip().strip('"').strip("'")
      continue

    if current_item is not None:
      if stripped.startswith("- ") and current_list_key is not None:
        val = stripped[2:].strip().strip('"').strip("'")
        current_item.setdefault(current_list_key, []).append(val)
      elif ":" in stripped:
        k, v = stripped.split(":", 1)
        k = k.strip()
        v = v.strip()
        if not v:
          current_list_key = k
          current_item[k] = []
        elif v.startswith("[") and v.endswith("]"):
          items = [
              x.strip().strip('"').strip("'")
              for x in v[1:-1].split(",")
              if x.strip()
          ]
          current_item[k] = items
          current_list_key = None
        else:
          current_item[k] = v.strip('"').strip("'")
          current_list_key = None

  return result


def parse_structured_fmea_file(
    path: pathlib.Path,
) -> tuple[
    list[AssumedSafetyReq],
    list[AssumedSafetyReq],
    list[AssumptionOfUse],
    list[FailureModeRecord],
]:
  """Parses a JSON, YAML, or Markdown SEooC/SW-FMEA file."""
  asrs: list[AssumedSafetyReq] = []
  feat_reqs: list[AssumedSafetyReq] = []
  aous: list[AssumptionOfUse] = []
  fms: list[FailureModeRecord] = []

  if path.suffix in (".json", ".yaml", ".yml"):
    data = _parse_simple_yaml_or_json(path)
    for item in data.get("assumed_safety_requirements", []):
      vtests = item.get("verification_tests", [])
      if isinstance(vtests, str):
        vtests = [vtests] if vtests else []
      asrs.append(
          AssumedSafetyReq(
              req_id=str(item.get("id", "")),
              asil=str(item.get("asil", "ASIL-B")),
              description=str(item.get("description", "")),
              rationale=str(item.get("rationale", "")),
              verification_tests=list(vtests),
              source_file=str(path),
              line=1,
          )
      )
    for item in data.get("assumptions_of_use", []):
      mits = item.get("mitigates", [])
      if isinstance(mits, str):
        mits = [m.strip() for m in mits.split(",") if m.strip()]
      aous.append(
          AssumptionOfUse(
              aou_id=str(item.get("id", "")),
              asil=str(item.get("asil", "ASIL-B")),
              description=str(item.get("description", "")),
              note=str(item.get("note", "")),
              mitigates=list(mits),
              integrator_verification=str(
                  item.get("integrator_verification", "")
              ),
              source_file=str(path),
              line=1,
          )
      )
    for item in data.get("failure_modes", []):
      gws = item.get("guideword") or item.get("guidewords", [])
      if isinstance(gws, str):
        gws = [g.strip() for g in gws.split(",") if g.strip()]
      mits = item.get("mitigated_by", [])
      if isinstance(mits, str):
        mits = [m.strip() for m in mits.split(",") if m.strip()]
      fms.append(
          FailureModeRecord(
              fm_id=str(item.get("id", "")),
              guidewords=list(gws),
              asil=str(item.get("asil", "ASIL-B")),
              description=str(item.get("description", "")),
              failure_effect=str(
                  item.get("system_effect") or item.get("failure_effect", "")
              ),
              rationale=str(
                  item.get("prevention_mechanism") or item.get("rationale", "")
              ),
              interface=str(item.get("interface", "")),
              mitigated_by=list(mits),
              verification_test=str(item.get("verification_test", "")),
              source_file=str(path),
              line=1,
          )
      )
  elif path.suffix == ".md":
    text = path.read_text(encoding="utf-8", errors="replace")
    for line_no, line in enumerate(text.splitlines(), start=1):
      if not line.strip().startswith("|"):
        continue
      cols = [c.strip().strip("`") for c in line.strip().strip("|").split("|")]
      if not cols or cols[0].startswith("-"):
        continue
      first = cols[0]
      if first.startswith("ASR-") and len(cols) >= 3:
        asrs.append(
            AssumedSafetyReq(
                req_id=first,
                asil=cols[1],
                description=cols[2],
                rationale=cols[3] if len(cols) > 3 else "Documented in table",
                verification_tests=[cols[4]]
                if len(cols) > 4 and cols[4]
                else [],
                source_file=str(path),
                line=line_no,
            )
        )
      elif first.startswith("AoU-") and len(cols) >= 4:
        mits = [
            m.strip().strip("`") for m in re.findall(r"FM-[A-Za-z0-9_-]+", cols[2])
        ]
        if not mits and cols[2]:
          mits = [m.strip() for m in cols[2].split(",") if m.strip()]
        aous.append(
            AssumptionOfUse(
                aou_id=first,
                asil=cols[1],
                description=cols[3],
                mitigates=mits,
                integrator_verification=cols[4] if len(cols) > 4 else "",
                source_file=str(path),
                line=line_no,
            )
        )
      elif first.startswith("FM-") and len(cols) >= 5:
        mits = re.findall(r"(?:ASR|AoU)-[A-Za-z0-9_-]+", line)
        fms.append(
            FailureModeRecord(
                fm_id=first,
                interface=cols[1],
                guidewords=[cols[2]],
                description=cols[3],
                failure_effect=cols[4],
                asil="ASIL-B",
                rationale=cols[5] if len(cols) > 5 else "",
                mitigated_by=mits,
                source_file=str(path),
                line=line_no,
            )
        )

  return asrs, feat_reqs, aous, fms


def scan_rust_crate(crate_path: pathlib.Path) -> list[RustPublicBoundary]:
  """Scans Rust source files for public API boundaries and SEooC/SW-FMEA doc tags."""
  rs_files = (
      [crate_path]
      if crate_path.is_file()
      else sorted(crate_path.rglob("*.rs"))
  )
  boundaries: list[RustPublicBoundary] = []

  fn_re = re.compile(
      r"^\s*(?:pub(?:\s*\([^)]*\))?\s+)?(?:const\s+)?(?:async\s+)?(unsafe\s+)?(?:extern\s+\"C(?:-unwind)?\"\s+)?fn\s+([A-Za-z0-9_]+)\s*(?:<[^>]*>)?\s*\((.*?)\)(?:\s*->\s*([^{;]+))?",
      re.DOTALL,
  )
  trait_re = re.compile(r"^\s*pub\s+(?:unsafe\s+)?trait\s+([A-Za-z0-9_]+)")
  impl_re = re.compile(
      r"^\s*(?:unsafe\s+)?impl(?:\s*<[^>]*>)?\s+(?:([A-Za-z0-9_:]+)\s+for\s+)?([A-Za-z0-9_]+)"
  )

  for rs_file in rs_files:
    lines = rs_file.read_text(encoding="utf-8", errors="replace").splitlines()
    doc_buffer: list[str] = []
    has_must_use = False
    current_container: str | None = None
    in_public_trait = False
    brace_depth = 0
    container_depth = 0

    i = 0
    while i < len(lines):
      line = lines[i]
      stripped = line.strip()

      if stripped.startswith("#[cfg(test)]"):
        break

      if stripped.startswith("///") or stripped.startswith("//!"):
        doc_buffer.append(stripped[3:].strip())
        i += 1
        continue

      if stripped.startswith("#[must_use"):
        has_must_use = True
        i += 1
        continue

      if stripped.startswith("#["):
        i += 1
        continue

      trait_m = trait_re.match(line)
      if trait_m:
        current_container = trait_m.group(1)
        in_public_trait = True
        container_depth = brace_depth
        brace_depth += line.count("{") - line.count("}")
        doc_buffer.clear()
        has_must_use = False
        i += 1
        continue

      impl_m = impl_re.match(line)
      if impl_m and "{" in line:
        current_container = impl_m.group(2)
        in_public_trait = bool(impl_m.group(1))
        container_depth = brace_depth
        brace_depth += line.count("{") - line.count("}")
        doc_buffer.clear()
        has_must_use = False
        i += 1
        continue

      is_pub_fn = stripped.startswith("pub ") and "fn " in stripped
      is_trait_fn = in_public_trait and (
          stripped.startswith("fn ") or stripped.startswith("unsafe fn ")
      )

      if is_pub_fn or is_trait_fn:
        sig_lines = [line]
        j = i
        while (
            j + 1 < len(lines)
            and "{" not in sig_lines[-1]
            and ";" not in sig_lines[-1]
        ):
          j += 1
          sig_lines.append(lines[j])
        full_sig = " ".join(s.strip() for s in sig_lines)
        m = fn_re.match(full_sig)
        if m:
          is_unsafe = bool(m.group(1)) or "unsafe " in full_sig
          is_extern_c = 'extern "C' in full_sig
          fn_name = m.group(2)
          ret_type = (m.group(4) or "").strip()
          returns_result = "Result<" in ret_type or ret_type.startswith(
              "Result"
          )

          qual_name = (
              f"{current_container}::{fn_name}"
              if current_container
              else fn_name
          )
          doc_text = "\n".join(doc_buffer)
          asr_refs = sorted(
              set(re.findall(r"\bASR[-_][A-Za-z0-9_-]+\b", doc_text))
          )
          fm_refs = sorted(
              set(re.findall(r"\bFM[-_][A-Za-z0-9_-]+\b", doc_text))
          )
          aou_refs = sorted(
              set(re.findall(r"\bAoU[-_][A-Za-z0-9_-]+\b", doc_text))
          )
          has_safety_doc = (
              "# Safety" in doc_text
              or "# Safety Requirements" in doc_text
              or "# Assumptions of Use" in doc_text
          )

          kind = (
              "extern_c_fn"
              if is_extern_c
              else (
                  "unsafe_fn"
                  if is_unsafe
                  else ("trait_fn" if is_trait_fn else "pub_fn")
              )
          )
          boundaries.append(
              RustPublicBoundary(
                  item_kind=kind,
                  name=fn_name,
                  qualified_name=qual_name,
                  returns_result=returns_result,
                  has_must_use=has_must_use,
                  has_safety_doc=has_safety_doc,
                  asr_refs=asr_refs,
                  fm_refs=fm_refs,
                  aou_refs=aou_refs,
                  source_file=str(rs_file),
                  line=i + 1,
              )
          )
        doc_buffer.clear()
        has_must_use = False
        for s in sig_lines:
          brace_depth += s.count("{") - s.count("}")
        i = j + 1
        continue

      if stripped:
        doc_buffer.clear()
        has_must_use = False

      brace_depth += line.count("{") - line.count("}")
      if current_container and brace_depth <= container_depth:
        current_container = None
        in_public_trait = False
      i += 1

  return boundaries


def validate_seooc_and_fmea(
    fmea_path: pathlib.Path | None,
    rust_crate_path: pathlib.Path | None = None,
) -> AuditSummary:
  """Runs closed-loop SEooC & SW-FMEA validation across artifacts and Rust code."""
  asrs: list[AssumedSafetyReq] = []
  feat_reqs: list[AssumedSafetyReq] = []
  aous: list[AssumptionOfUse] = []
  fms: list[FailureModeRecord] = []
  findings: list[ValidationFinding] = []

  if fmea_path is not None and fmea_path.exists():
    files_to_parse: list[pathlib.Path] = []
    if fmea_path.is_dir():
      files_to_parse.extend(sorted(fmea_path.rglob("*.trlc")))
      if not files_to_parse:
        files_to_parse.extend(sorted(fmea_path.rglob("*.yaml")))
        files_to_parse.extend(sorted(fmea_path.rglob("*.yml")))
        files_to_parse.extend(sorted(fmea_path.rglob("*.json")))
        files_to_parse.extend(sorted(fmea_path.rglob("*.md")))
    else:
      files_to_parse.append(fmea_path)

    for fp in files_to_parse:
      if fp.suffix == ".trlc":
        a, fr, u, f = parse_trlc_file(fp)
      else:
        a, fr, u, f = parse_structured_fmea_file(fp)
      asrs.extend(a)
      feat_reqs.extend(fr)
      aous.extend(u)
      fms.extend(f)

  # Map reverse links from AoU.mitigates -> FailureMode.mitigated_by
  fm_by_id = {fm.fm_id: fm for fm in fms}
  asr_by_id = {a.req_id: a for a in asrs}
  aou_by_id = {u.aou_id: u for u in aous}

  for u in aous:
    for target_fm in u.mitigates:
      if target_fm in fm_by_id and u.aou_id not in fm_by_id[target_fm].mitigated_by:
        fm_by_id[target_fm].mitigated_by.append(u.aou_id)

  guideword_dist: dict[str, int] = {}
  closed_loop_fms = 0
  unmitigated_fms = 0

  # 1. Validate Failure Modes (ISO 26262-6 Clause 7.4.10)
  for fm in fms:
    if not fm.guidewords:
      findings.append(
          ValidationFinding(
              rule_id="FMEA-001",
              severity="ERROR",
              category="MissingGuideword",
              item_id=fm.fm_id,
              message=f"FailureMode '{fm.fm_id}' has no HAZOP guideword assigned.",
              remediation=(
                  "Assign at least one valid HAZOP guideword (LossOfFunction,"
                  " UnintendedFunction, DelayedFunction, PartialFunction,"
                  " ExceedingFunction)."
              ),
              source_file=fm.source_file,
              line=fm.line,
          )
      )
    for gw in fm.guidewords:
      guideword_dist[gw] = guideword_dist.get(gw, 0) + 1
      if gw not in VALID_GUIDEWORDS:
        findings.append(
            ValidationFinding(
                rule_id="FMEA-002",
                severity="ERROR",
                category="InvalidGuideword",
                item_id=fm.fm_id,
                message=(
                    f"FailureMode '{fm.fm_id}' uses non-standard HAZOP"
                    f" guideword '{gw}'."
                ),
                remediation=f"Use one of: {', '.join(sorted(VALID_GUIDEWORDS))}.",
                source_file=fm.source_file,
                line=fm.line,
            )
        )

    if not fm.failure_effect.strip():
      findings.append(
          ValidationFinding(
              rule_id="FMEA-003",
              severity="ERROR",
              category="MissingFailureEffect",
              item_id=fm.fm_id,
              message=f"FailureMode '{fm.fm_id}' is missing a failure effect.",
              remediation=(
                  "Document both the local software effect and system-level"
                  " safety effect."
              ),
              source_file=fm.source_file,
              line=fm.line,
          )
      )

    has_valid_rationale = bool(
        fm.rationale.strip() and fm.rationale.strip() != "-"
    )
    has_explicit_mitigation = bool(fm.mitigated_by)

    if not has_valid_rationale and not has_explicit_mitigation:
      unmitigated_fms += 1
      findings.append(
          ValidationFinding(
              rule_id="FMEA-004",
              severity="WARNING" if fm.source_file.endswith(".trlc") else "ERROR",
              category="UnmitigatedFailureMode",
              item_id=fm.fm_id,
              message=(
                  f"FailureMode '{fm.fm_id}' has an empty rationale"
                  f" ('{fm.rationale}') and no linked AoU/ASR mitigation."
              ),
              remediation=(
                  "Link this FailureMode to an internal ASR-* preventive"
                  " requirement or external AoU-* obligation and provide a"
                  " complete safety rationale."
              ),
              source_file=fm.source_file,
              line=fm.line,
          )
      )
    else:
      closed_loop_fms += 1

    # Verify any explicit mitigated_by IDs exist
    for ref_id in fm.mitigated_by:
      if ref_id not in asr_by_id and ref_id not in aou_by_id:
        findings.append(
            ValidationFinding(
                rule_id="FMEA-005",
                severity="ERROR",
                category="DanglingMitigationRef",
                item_id=fm.fm_id,
                message=(
                    f"FailureMode '{fm.fm_id}' references unknown mitigation ID"
                    f" '{ref_id}'."
                ),
                remediation=(
                    f"Define '{ref_id}' in assumed_safety_requirements or"
                    " assumptions_of_use."
                ),
                source_file=fm.source_file,
                line=fm.line,
            )
        )

  # 2. Validate Assumptions of Use (ISO 26262-10 Clause 9)
  unlinked_aous = 0
  for u in aous:
    if not u.mitigates:
      unlinked_aous += 1
      findings.append(
          ValidationFinding(
              rule_id="SEooC-AoU-001",
              severity="WARNING" if u.source_file.endswith(".trlc") else "ERROR",
              category="UnlinkedAoU",
              item_id=u.aou_id,
              message=(
                  f"Assumption of Use '{u.aou_id}' has an empty or missing"
                  " 'mitigates' link to SW-FMEA failure modes."
              ),
              remediation=(
                  "Populate 'mitigates' with the exact FailureMode ID(s) that"
                  " this AoU controls at the system integration level."
              ),
              source_file=u.source_file,
              line=u.line,
          )
      )
    if not u.integrator_verification.strip():
      findings.append(
          ValidationFinding(
              rule_id="SEooC-AoU-002",
              severity="WARNING",
              category="MissingIntegratorCheck",
              item_id=u.aou_id,
              message=(
                  f"Assumption of Use '{u.aou_id}' lacks an explicit integrator"
                  " verification note/criteria."
              ),
              remediation=(
                  "Specify how the OEM/Tier-1 integrator must verify this AoU"
                  " during ISO 26262-6 Clause 11 integration."
              ),
              source_file=u.source_file,
              line=u.line,
          )
      )

  # 3. Validate Assumed Safety Requirements (ISO 26262-10 Clause 9.2.3)
  for a in asrs:
    if not a.rationale.strip():
      findings.append(
          ValidationFinding(
              rule_id="SEooC-ASR-001",
              severity="ERROR",
              category="MissingAsrRationale",
              item_id=a.req_id,
              message=(
                  f"AssumedSafetyReq '{a.req_id}' is missing a safety"
                  " rationale."
              ),
              remediation=(
                  "Add a rationale explaining why this requirement is assumed"
                  " for the SEooC boundary."
              ),
              source_file=a.source_file,
              line=a.line,
          )
      )
    if not a.source_file.endswith(".trlc") and not a.verification_tests:
      findings.append(
          ValidationFinding(
              rule_id="SEooC-ASR-002",
              severity="ERROR",
              category="UnverifiedAsr",
              item_id=a.req_id,
              message=(
                  f"AssumedSafetyReq '{a.req_id}' has no linked verification"
                  " test."
              ),
              remediation=(
                  "Link at least one unit, fault-injection, or Kani test target"
                  " verifying this ASR."
              ),
              source_file=a.source_file,
              line=a.line,
          )
      )

  # 4. Validate Rust Crate Public Boundaries (if provided)
  rust_boundaries: list[RustPublicBoundary] = []
  rust_covered = 0
  if rust_crate_path is not None and rust_crate_path.exists():
    rust_boundaries = scan_rust_crate(rust_crate_path)
    fmea_interfaces = {fm.interface for fm in fms if fm.interface}
    fmea_short_names = {
        fm.interface.split("::")[-1].split(".")[-1]
        for fm in fms
        if fm.interface
    }

    for rb in rust_boundaries:
      covered_by_doc = bool(rb.fm_refs and (rb.asr_refs or rb.aou_refs))
      covered_by_fmea = (
          rb.qualified_name in fmea_interfaces
          or rb.name in fmea_short_names
          or any(fid in fm_by_id for fid in rb.fm_refs)
      )
      if covered_by_doc and covered_by_fmea:
        rust_covered += 1
      else:
        findings.append(
            ValidationFinding(
                rule_id="RUST-SEooC-001",
                severity="ERROR",
                category="UnanalyzedRustBoundary",
                item_id=rb.qualified_name,
                message=(
                    f"Public Rust boundary '{rb.qualified_name}' ({rb.item_kind})"
                    " is missing closed-loop SW-FMEA / SEooC doc annotations"
                    f" (ASR={rb.asr_refs}, FM={rb.fm_refs}, AoU={rb.aou_refs})."
                ),
                remediation=(
                    "Add '# Safety Requirements (@satisfies ASR-*)', '# Failure"
                    " Modes (@fmea FM-*)', and '# Assumptions of Use (@aou"
                    " AoU-*)' rustdoc sections and link the function in the"
                    " SW-FMEA table."
                ),
                source_file=rb.source_file,
                line=rb.line,
            )
        )

      if rb.returns_result and not rb.has_must_use:
        findings.append(
            ValidationFinding(
                rule_id="RUST-SEooC-002",
                severity="ERROR",
                category="MissingMustUseOnFallibleBoundary",
                item_id=rb.qualified_name,
                message=(
                    f"Fallible public boundary '{rb.qualified_name}' returns"
                    " Result<T, E> without #[must_use], allowing callers to"
                    " silently ignore SW-FMEA detection errors."
                ),
                remediation=(
                    'Add #[must_use = "ASIL-B AoU: caller must handle error"]'
                    " to enforce caller error checking at compile time."
                ),
                source_file=rb.source_file,
                line=rb.line,
            )
        )

  err_count = sum(1 for f in findings if f.severity == "ERROR")
  warn_count = sum(1 for f in findings if f.severity == "WARNING")

  return AuditSummary(
      target_path=str(fmea_path) if fmea_path else "",
      rust_crate_path=str(rust_crate_path) if rust_crate_path else "",
      asr_count=len(asrs),
      feat_req_count=len(feat_reqs),
      aou_count=len(aous),
      failure_mode_count=len(fms),
      guideword_distribution=guideword_dist,
      rust_boundaries_total=len(rust_boundaries),
      rust_boundaries_covered=rust_covered,
      closed_loop_fm_count=closed_loop_fms,
      unmitigated_fm_count=unmitigated_fms,
      unlinked_aou_count=unlinked_aous,
      error_count=err_count,
      warning_count=warn_count,
      findings=findings,
      asrs=asrs,
      aous=aous,
      failure_modes=fms,
      rust_boundaries=rust_boundaries,
  )


def format_report(summary: AuditSummary, fmt: str) -> str:
  """Formats the AuditSummary as markdown, json, or text."""
  if fmt == "json":
    return json.dumps(asdict(summary), indent=2)

  if fmt == "text":
    lines = [
        "=== ISO 26262-10 SEooC & ISO 26262-6 SW-FMEA Validation Report ===",
        f"Target FMEA Artifact : {summary.target_path or 'N/A'}",
        f"Target Rust Crate    : {summary.rust_crate_path or 'N/A'}",
        f"Assumed System Reqs  : {summary.asr_count} (FeatReqs: {summary.feat_req_count})",
        f"Assumptions of Use   : {summary.aou_count} (Unlinked: {summary.unlinked_aou_count})",
        f"SW-FMEA FailureModes : {summary.failure_mode_count} (Closed-Loop: {summary.closed_loop_fm_count}, Unmitigated: {summary.unmitigated_fm_count})",
        f"Rust API Boundaries  : {summary.rust_boundaries_covered}/{summary.rust_boundaries_total} covered",
        f"Findings             : {summary.error_count} ERROR(s), {summary.warning_count} WARNING(s)",
    ]
    for f in summary.findings:
      lines.append(
          f"  [{f.severity}] {f.rule_id} ({f.item_id}): {f.message} ->"
          f" {f.remediation}"
      )
    return "\n".join(lines)

  # Default: markdown
  lines = [
      "# ISO 26262-10 Clause 9 SEooC & ISO 26262-6 Clause 7 SW-FMEA Validation Report",
      "",
      "## 1. Executive Summary",
      "",
      "| Metric | Value |",
      "|---|---|",
      f"| **FMEA / SEooC Artifact Path** | `{summary.target_path or 'N/A'}` |",
      f"| **Rust Crate Path** | `{summary.rust_crate_path or 'N/A'}` |",
      f"| **Assumed Safety Requirements (`ASR-*`)** | `{summary.asr_count}` |",
      f"| **Derived Feature Requirements (`FEAT_*`)** | `{summary.feat_req_count}` |",
      f"| **Assumptions of Use (`AoU-*`)** | `{summary.aou_count}` (`{summary.aou_count - summary.unlinked_aou_count}` linked, `{summary.unlinked_aou_count}` unlinked) |",
      f"| **SW-FMEA Failure Modes (`FM-*`)** | `{summary.failure_mode_count}` (`{summary.closed_loop_fm_count}` closed-loop, `{summary.unmitigated_fm_count}` unmitigated) |",
      f"| **Rust Public API Boundary Coverage** | `{summary.rust_boundaries_covered} / {summary.rust_boundaries_total}` |",
      f"| **Validation Findings** | `{summary.error_count}` Errors, `{summary.warning_count}` Warnings |",
      "",
      "## 2. HAZOP Guideword Distribution (`ISO 26262-6 Clause 7.4.10`)",
      "",
      "| HAZOP Guideword | Failure Mode Count |",
      "|---|---|",
  ]
  for gw, cnt in sorted(summary.guideword_distribution.items()):
    lines.append(f"| `{gw}` | `{cnt}` |")
  if not summary.guideword_distribution:
    lines.append("| *(none)* | `0` |")

  if summary.rust_boundaries:
    lines.extend([
        "",
        "## 3. Rust Public Boundary Traceability Matrix",
        "",
        "| Public Boundary | Kind | Returns `Result` | `#[must_use]` | Linked `ASR-*` | Linked `FM-*` | Linked `AoU-*` |",
        "|---|---|---|---|---|---|---|",
    ])
    for rb in summary.rust_boundaries:
      lines.append(
          f"| `{rb.qualified_name}` | `{rb.item_kind}` |"
          f" `{rb.returns_result}` | `{rb.has_must_use}` |"
          f" {', '.join(f'`{x}`' for x in rb.asr_refs) or '*(none)*'} |"
          f" {', '.join(f'`{x}`' for x in rb.fm_refs) or '*(none)*'} |"
          f" {', '.join(f'`{x}`' for x in rb.aou_refs) or '*(none)*'} |"
      )

  lines.extend([
      "",
      "## 4. Audit Findings & Remediation Actions",
      "",
  ])
  if not summary.findings:
    lines.append(
        "Zero errors or warnings detected. All SEooC `ASR-*`, `AoU-*`, and"
        " SW-FMEA `FM-*` records form a closed traceability loop."
    )
  else:
    lines.extend([
        "| Severity | Rule ID | Item ID | Location | Finding & Remediation |",
        "|---|---|---|---|---|",
    ])
    for f in summary.findings:
      loc = f"`{pathlib.Path(f.source_file).name}:{f.line}`" if f.source_file else "N/A"
      lines.append(
          f"| **{f.severity}** | `{f.rule_id}` | `{f.item_id}` | {loc} |"
          f" {f.message} **Fix**: {f.remediation} |"
      )

  return "\n".join(lines) + "\n"


COMPLIANT_FMEA_YAML = """seooc_metadata:
  crate_name: "someip_seooc_runtime"
  target_asil: "ASIL-B"

assumed_safety_requirements:
  - id: "ASR-COM-001"
    asil: "ASIL-B"
    description: "All fallible public APIs shall return #[must_use] Result<T, Error> without panicking or aborting."
    rationale: "Prevents sudden process termination and silent message drops."
    verification_tests:
      - "tests::seooc::test_offer_and_send_error_paths"
  - id: "ASR-E2E-001"
    asil: "ASIL-B"
    description: "Inbound frames shall be verified via public-domain CRC-16/CRC-32 and sequence counter before payload access."
    rationale: "Prevents delivery of corrupted payloads over the black channel."
    verification_tests:
      - "tests::seooc::test_verify_frame_rejects_corrupted_crc"

assumptions_of_use:
  - id: "AoU-AVAIL-001"
    asil: "ASIL-B"
    description: "The system integrator shall monitor service availability and transition to safe state within FTTI on communication loss."
    mitigates:
      - "FM-ROU-001"
      - "FM-EP-001"
    integrator_verification: "Fault-injection test disconnecting service provider and verifying safe-state transition <= FTTI."
  - id: "AoU-TIM-001"
    asil: "ASIL-B"
    description: "The system integrator shall schedule the runtime worker thread under SCHED_FIFO supervised by a deadline watchdog."
    mitigates:
      - "FM-RT-002"
    integrator_verification: "Target ECU scheduling priority audit and watchdog starvation injection test."

failure_modes:
  - id: "FM-ROU-001"
    interface: "SomeIpRuntime::offer_service"
    guideword: "LossOfFunction"
    asil: "ASIL-B"
    description: "Service offer fails to register due to full command queue or duplicate owner."
    system_effect: "Consumers do not discover the offered service instance."
    prevention_mechanism: "Typed #[must_use] Result<OfferGuard, OfferError> return contract."
    mitigated_by:
      - "ASR-COM-001"
      - "AoU-AVAIL-001"
    verification_test: "tests::seooc::test_offer_and_send_error_paths"
  - id: "FM-EP-001"
    interface: "SomeIpRuntime::send"
    guideword: "LossOfFunction"
    asil: "ASIL-B"
    description: "Outbound message is rejected when per-client bounded queue quota is full."
    system_effect: "RPC request or event notification is not transmitted."
    prevention_mechanism: "Pre-allocated per-client byte quota returning SendError::QueueFull."
    mitigated_by:
      - "ASR-COM-001"
      - "AoU-AVAIL-001"
    verification_test: "tests::seooc::test_offer_and_send_error_paths"
  - id: "FM-RT-002"
    interface: "SomeIpRuntime::poll_dispatch"
    guideword: "DelayedFunction"
    asil: "ASIL-B"
    description: "Dispatch cycle exceeds expected execution budget due to OS preemption."
    system_effect: "Delayed processing of inbound SOME/IP frames."
    prevention_mechanism: "Bounded batch limit per poll iteration."
    mitigated_by:
      - "ASR-COM-001"
      - "AoU-TIM-001"
    verification_test: "tests::seooc::test_poll_dispatch_budget"
  - id: "FM-E2E-001"
    interface: "SomeIpRuntime::verify_frame"
    guideword: "PartialOrCorruptedExecution"
    asil: "ASIL-B"
    description: "Wire frame payload is corrupted during transmission over UDP/TCP black channel."
    system_effect: "Corrupted frame is rejected with E2eError::CorruptedCrc before reaching application logic."
    prevention_mechanism: "Typestate VerifiedFrame constructor gated by CRC and counter check."
    mitigated_by:
      - "ASR-E2E-001"
    verification_test: "tests::seooc::test_verify_frame_rejects_corrupted_crc"
"""

COMPLIANT_RUST_RS = """#![forbid(unsafe_code)]

#[derive(Debug, PartialEq, Eq)]
pub enum OfferError {
    QueueFull,
    AlreadyOffered,
}

#[derive(Debug, PartialEq, Eq)]
pub enum SendError {
    QueueFull,
    Disconnected,
}

#[derive(Debug, PartialEq, Eq)]
pub enum E2eError {
    CorruptedCrc,
    SequenceGap,
}

pub struct SomeIpRuntime {
    max_batch: usize,
}

impl SomeIpRuntime {
    /// Offers a SOME/IP service instance.
    ///
    /// # Safety Requirements
    /// - `@satisfies ASR-COM-001`
    ///
    /// # Failure Modes (SW-FMEA)
    /// - `@fmea FM-ROU-001` (`LossOfFunction`)
    ///
    /// # Assumptions of Use
    /// - `@aou AoU-AVAIL-001`
    #[must_use = "ASIL-B AoU-AVAIL-001: caller must handle OfferError"]
    pub fn offer_service(&mut self, service_id: u16, instance_id: u16) -> Result<(), OfferError> {
        if service_id == 0 || instance_id == 0 {
            return Err(OfferError::QueueFull);
        }
        Ok(())
    }

    /// Sends a bounded SOME/IP frame.
    ///
    /// # Safety Requirements
    /// - `@satisfies ASR-COM-001`
    ///
    /// # Failure Modes (SW-FMEA)
    /// - `@fmea FM-EP-001` (`LossOfFunction`)
    ///
    /// # Assumptions of Use
    /// - `@aou AoU-AVAIL-001`
    #[must_use = "ASIL-B AoU-AVAIL-001: caller must handle SendError"]
    pub fn send(&mut self, payload: &[u8]) -> Result<usize, SendError> {
        if payload.is_empty() {
            return Err(SendError::QueueFull);
        }
        Ok(payload.len())
    }

    /// Polls bounded dispatch queue.
    ///
    /// # Safety Requirements
    /// - `@satisfies ASR-COM-001`
    ///
    /// # Failure Modes (SW-FMEA)
    /// - `@fmea FM-RT-002` (`DelayedFunction`)
    ///
    /// # Assumptions of Use
    /// - `@aou AoU-TIM-001`
    #[must_use = "ASIL-B AoU-TIM-001: caller must check processed count"]
    pub fn poll_dispatch(&mut self, limit: usize) -> Result<usize, SendError> {
        Ok(limit.min(self.max_batch))
    }

    /// Verifies an inbound E2E black-channel frame.
    ///
    /// # Safety Requirements
    /// - `@satisfies ASR-E2E-001`
    ///
    /// # Failure Modes (SW-FMEA)
    /// - `@fmea FM-E2E-001` (`PartialOrCorruptedExecution`)
    #[must_use = "ASIL-B ASR-E2E-001: caller must inspect E2eError"]
    pub fn verify_frame(&self, frame: &[u8]) -> Result<&[u8], E2eError> {
        if frame.len() < 4 {
            return Err(E2eError::CorruptedCrc);
        }
        Ok(&frame[4..])
    }
}
"""

NON_COMPLIANT_FMEA_YAML = """seooc_metadata:
  crate_name: "broken_someip_crate"
  target_asil: "ASIL-B"

assumed_safety_requirements:
  - id: "ASR-BROKEN-001"
    asil: "ASIL-B"
    description: "Requirement missing both rationale and verification tests."
    rationale: ""

assumptions_of_use:
  - id: "AoU-BROKEN-001"
    asil: "ASIL-B"
    description: "Unlinked AoU with empty mitigates list and no integrator verification."
    mitigates: []
    integrator_verification: ""

failure_modes:
  - id: "FM-BROKEN-001"
    interface: "BrokenRuntime::send_unchecked"
    guideword: "InvalidMadeUpGuideword"
    asil: "ASIL-B"
    description: "Failure mode with invalid guideword, empty rationale, and unknown mitigation."
    system_effect: ""
    prevention_mechanism: ""
    mitigated_by:
      - "ASR-DOES-NOT-EXIST"
"""

NON_COMPLIANT_RUST_RS = """pub struct BrokenRuntime;

impl BrokenRuntime {
    // Missing #[must_use] and missing ASR/FM/AoU doc annotations
    pub fn send_unchecked(&mut self, buf: &[u8]) -> Result<usize, u8> {
        Ok(buf.len())
    }
}
"""


def dump_fixtures(target_dir: pathlib.Path) -> tuple[pathlib.Path, pathlib.Path, pathlib.Path, pathlib.Path]:
  """Writes compliant and non-compliant SEooC/SW-FMEA + Rust fixtures to target_dir."""
  target_dir.mkdir(parents=True, exist_ok=True)
  comp_fmea = target_dir / "compliant_seooc_fmea.yaml"
  comp_rs = target_dir / "compliant_runtime.rs"
  noncomp_fmea = target_dir / "non_compliant_seooc_fmea.yaml"
  noncomp_rs = target_dir / "non_compliant_runtime.rs"

  comp_fmea.write_text(COMPLIANT_FMEA_YAML, encoding="utf-8")
  comp_rs.write_text(COMPLIANT_RUST_RS, encoding="utf-8")
  noncomp_fmea.write_text(NON_COMPLIANT_FMEA_YAML, encoding="utf-8")
  noncomp_rs.write_text(NON_COMPLIANT_RUST_RS, encoding="utf-8")
  return comp_fmea, comp_rs, noncomp_fmea, noncomp_rs


def run_self_test() -> int:
  """Executes built-in self-tests on synthetic YAML, TRLC, and Rust fixtures."""
  with tempfile.TemporaryDirectory(prefix="seooc_fmea_selftest_") as tmp:
    tdir = pathlib.Path(tmp)
    comp_fmea, comp_rs, noncomp_fmea, noncomp_rs = dump_fixtures(tdir)

    # 1. Verify compliant fixture passes with 0 errors and 4/4 Rust boundaries covered
    comp_summary = validate_seooc_and_fmea(comp_fmea, comp_rs)
    assert comp_summary.error_count == 0, (
        f"Expected 0 errors on compliant fixture, got {comp_summary.error_count}:"
        f" {comp_summary.findings}"
    )
    assert comp_summary.rust_boundaries_total == 4
    assert comp_summary.rust_boundaries_covered == 4
    assert comp_summary.closed_loop_fm_count == 4

    # 2. Verify non-compliant fixture triggers expected rule IDs
    noncomp_summary = validate_seooc_and_fmea(noncomp_fmea, noncomp_rs)
    rule_ids = {f.rule_id for f in noncomp_summary.findings}
    expected_rules = {
        "FMEA-002",
        "FMEA-003",
        "FMEA-005",
        "SEooC-AoU-001",
        "SEooC-ASR-001",
        "SEooC-ASR-002",
        "RUST-SEooC-001",
        "RUST-SEooC-002",
    }
    missing_rules = expected_rules - rule_ids
    assert not missing_rules, f"Self-test missed expected rules: {missing_rules}"

    # 3. Verify synthetic TRLC SEooC + SW-FMEA fixture parsing
    trlc_file = tdir / "seooc.trlc"
    trlc_file.write_text(
        'package RpcSeooC\n'
        'SafetyReq.AssumedSystemReq ASR_BoundedQueue {\n'
        '    description = "Outbound queue shall be bounded."\n'
        '    safety = SafetyReq.Asil.B\n'
        '    rationale = "Prevents heap exhaustion."\n'
        '}\n'
        'SafetyReq.FailureMode FM_Drop {\n'
        '    guidewords = [ SafetyReq.Guideword.LossOfFunction ]\n'
        '    description = "Queue full drops frame."\n'
        '    failureeffect = "Frame not sent."\n'
        '    safety = SafetyReq.Asil.B\n'
        '    rationale = "Mitigated by ASR_BoundedQueue and AoU_Supervise."\n'
        '    interface = "Runtime::send"\n'
        '}\n'
        'SafetyReq.AoU AoU_Supervise {\n'
        '    description = "Integrator shall supervise cyclic deadlines."\n'
        '    safety = SafetyReq.Asil.B\n'
        '    note = "Verify via link-disconnect fault injection."\n'
        '    mitigates = "FM_Drop"\n'
        '}\n',
        encoding="utf-8",
    )
    trlc_summary = validate_seooc_and_fmea(trlc_file)
    assert trlc_summary.asr_count == 1 and trlc_summary.aou_count == 1 and trlc_summary.failure_mode_count == 1
    assert trlc_summary.error_count == 0, f"Unexpected TRLC errors: {trlc_summary.findings}"

    print("SELF-TEST PASSED:")
    print(
        "  [1] Compliant Rust + SEooC/SW-FMEA fixture : PASS (0 errors, 4/4"
        " public boundaries covered, 4/4 closed-loop FMs)"
    )
    print(
        "  [2] Non-Compliant fixture negative tests   : PASS (detected"
        f" {len(noncomp_summary.findings)} findings across"
        f" {sorted(rule_ids)})"
    )
    print("  [3] Synthetic TRLC SEooC/SW-FMEA fixture   : PASS (1 ASR, 1 AoU, 1 FM, 0 errors)")
  return 0


def main(argv: list[str] | None = None) -> int:
  parser = argparse.ArgumentParser(
      description=(
          "Validate ISO 26262-10 Clause 9 SEooC & ISO 26262-6 Clause 7 SW-FMEA"
          " artifacts (.trlc, .yaml, .json, .md) and Rust crate public API"
          " boundaries."
      )
  )
  parser.add_argument(
      "--fmea-file",
      type=pathlib.Path,
      help=(
          "Path to a .trlc file/directory, or a YAML/JSON/Markdown SEooC + SW-FMEA"
          " artifact."
      ),
  )
  parser.add_argument(
      "--rust-crate",
      type=pathlib.Path,
      help=(
          "Optional path to a Rust crate directory or .rs file to cross-check"
          " public boundaries against the SW-FMEA."
      ),
  )
  parser.add_argument(
      "--dump-fixtures",
      type=pathlib.Path,
      help=(
          "Directory where compliant and non-compliant Rust + SEooC/SW-FMEA"
          " fixtures will be written."
      ),
  )
  parser.add_argument(
      "--self-test",
      action="store_true",
      help=(
          "Run built-in self-tests against synthetic fixtures and"
          " /path/to/safety_middleware/dependability/."
      ),
  )
  parser.add_argument(
      "--format",
      choices=["markdown", "json", "text"],
      default="markdown",
      help="Output report format (default: markdown).",
  )
  parser.add_argument(
      "--output",
      type=pathlib.Path,
      help="Optional file path to write the formatted report.",
  )
  parser.add_argument(
      "--strict",
      action="store_true",
      help="Exit with code 1 if any validation errors are found.",
  )

  args = parser.parse_args(argv)

  if args.dump_fixtures:
    comp_fmea, comp_rs, noncomp_fmea, noncomp_rs = dump_fixtures(
        args.dump_fixtures
    )
    print(f"Dumped fixtures to {args.dump_fixtures}:")
    print(f"  - {comp_fmea}")
    print(f"  - {comp_rs}")
    print(f"  - {noncomp_fmea}")
    print(f"  - {noncomp_rs}")
    if not args.self_test and not args.fmea_file and not args.rust_crate:
      return 0

  if args.self_test or (not args.fmea_file and not args.rust_crate):
    return run_self_test()

  summary = validate_seooc_and_fmea(args.fmea_file, args.rust_crate)
  report = format_report(summary, args.format)

  if args.output:
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(report, encoding="utf-8")
  else:
    print(report, end="")

  if args.strict and summary.error_count > 0:
    return 1
  return 0


if __name__ == "__main__":
  sys.exit(main())
