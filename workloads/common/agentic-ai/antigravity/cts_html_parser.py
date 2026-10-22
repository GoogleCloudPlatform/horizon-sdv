# Copyright (c) 2026 Accenture, All Rights Reserved.
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

"""CTS HTML Report Pre-processor for Horizon SDV CI Failure Analysis Agent.

Extracts suite summary metrics and module breakdown tables from Android CTS
`test_result_failures_suite.html` and formats them into a compact, ranked,
token-efficient plain-text summary (`cts_module_summary.txt`).
"""

from __future__ import annotations

import argparse
import html
import logging
import os
import re
import sys
from html.parser import HTMLParser
from pathlib import Path
from typing import Any, NamedTuple

LOG = logging.getLogger("horizon.agy.cts_html_parser")


class ModuleInfo(NamedTuple):
    name: str
    passed: int
    failed: int
    assumption_failures: int
    ignored: int
    total: int
    done: bool
    time: str
    raw_cells: list[str]


class _CTSSummaryHTMLParser(HTMLParser):
    """Streaming HTML parser targeting `<table class="summary">` and `<table class="testsummary">`."""

    def __init__(self, stop_after_summary: bool = True) -> None:
        super().__init__()
        self.stop_after_summary = stop_after_summary

        # Parsing states
        self.current_table_type: str | None = None  # "summary" or "testsummary"
        self.in_row = False
        self.in_cell = False
        self.current_cell_tag: str | None = None  # "th" or "td"
        self.current_cell_text: list[str] = []
        self.current_row_cells: list[str] = []

        # Extracted data
        self.summary_rows: list[list[str]] = []
        self.testsummary_rows: list[list[str]] = []

        # Early termination flag
        self.completed = False

    def handle_starttag(self, tag: str, attrs: list[tuple[str, str | None]]) -> None:
        if self.completed:
            return

        tag_lower = tag.lower()
        attr_dict = {k.lower(): (v or "") for k, v in attrs}

        if tag_lower == "table":
            class_val = attr_dict.get("class", "").lower()
            id_val = attr_dict.get("id", "").lower()
            combined = f"{class_val} {id_val}"
            if any(k in combined for k in ("testsummary", "test-summary", "modulesummary", "module-summary")):
                self.current_table_type = "testsummary"
            elif any(k in combined for k in ("summary", "suite-summary", "suitesummary")):
                self.current_table_type = "summary"
            else:
                # Format may not use standard classes; inspect table header rows dynamically
                self.current_table_type = "candidate"

        elif self.current_table_type:
            if tag_lower == "tr":
                self.in_row = True
                self.current_row_cells = []
            elif tag_lower in ("td", "th") and self.in_row:
                self.in_cell = True
                self.current_cell_tag = tag_lower
                self.current_cell_text = []

    def handle_endtag(self, tag: str) -> None:
        if self.completed:
            return

        tag_lower = tag.lower()
        if self.current_table_type:
            if tag_lower in ("td", "th") and self.in_cell:
                self.in_cell = False
                cell_text = " ".join("".join(self.current_cell_text).split())
                self.current_row_cells.append(cell_text)
                self.current_cell_text = []
            elif tag_lower == "tr" and self.in_row:
                self.in_row = False
                if self.current_row_cells:
                    if self.current_table_type == "candidate":
                        row_lower = " ".join(self.current_row_cells).lower()
                        if "module" in row_lower and any(w in row_lower for w in ("pass", "fail", "total", "done", "status")):
                            self.current_table_type = "testsummary"
                            self.testsummary_rows.append(self.current_row_cells)
                        elif any(w in row_lower for w in ("suite", "plan", "tests passed", "tests failed", "modules done", "modules completed")):
                            self.current_table_type = "summary"
                            self.summary_rows.append(self.current_row_cells)
                    elif self.current_table_type == "summary":
                        self.summary_rows.append(self.current_row_cells)
                    elif self.current_table_type == "testsummary":
                        self.testsummary_rows.append(self.current_row_cells)
            elif tag_lower == "table":
                self.current_table_type = None
                if (
                    self.stop_after_summary
                    and self.summary_rows
                    and self.testsummary_rows
                ):
                    self.completed = True

    def handle_data(self, data: str) -> None:
        if self.completed:
            return
        if self.in_cell:
            self.current_cell_text.append(data)

    def handle_entityref(self, name: str) -> None:
        if self.completed:
            return
        if self.in_cell:
            self.current_cell_text.append(html.unescape(f"&{name};"))

    def handle_charref(self, name: str) -> None:
        if self.completed:
            return
        if self.in_cell:
            self.current_cell_text.append(html.unescape(f"&#{name};"))


def _parse_int(val: str, default: int = 0) -> int:
    clean = re.sub(r"[^\d]", "", val)
    if clean:
        try:
            return int(clean)
        except ValueError:
            return default
    return default


def _parse_bool(val: str) -> bool:
    return val.strip().lower() in ("true", "yes", "1", "done", "pass", "passed", "completed")


def parse_cts_html(html_content: str) -> tuple[dict[str, str], list[ModuleInfo]]:
    """Parse HTML and extract suite metrics and module breakdown."""
    parser = _CTSSummaryHTMLParser(stop_after_summary=True)
    try:
        parser.feed(html_content)
    except Exception as exc:
        LOG.debug("HTML parser feed encountered: %s", exc)

    # 1. Process summary metrics
    metrics: dict[str, str] = {}
    for row in parser.summary_rows:
        if len(row) >= 2:
            key = row[0].strip()
            val = row[1].strip()
            if key and val:
                metrics[key] = val

    # 2. Process testsummary table
    modules: list[ModuleInfo] = []
    if parser.testsummary_rows:
        header_row_idx = 0
        headers = [h.lower() for h in parser.testsummary_rows[0]]

        # Determine column indices
        mod_col = 0
        pass_col = 1
        fail_col = 2
        assumption_col: int | None = None
        ignored_col: int | None = None
        total_col = 3
        done_col = 4
        time_col = 5

        for idx, h in enumerate(headers):
            h_clean = h.strip().lower()
            if "module" in h_clean or "package" in h_clean:
                mod_col = idx
            elif "assumption" in h_clean:
                assumption_col = idx
            elif "ignore" in h_clean:
                ignored_col = idx
            elif "pass" in h_clean:
                pass_col = idx
            elif "fail" in h_clean or "error" in h_clean:
                fail_col = idx
            elif "total" in h_clean:
                total_col = idx
            elif "done" in h_clean or "complete" in h_clean or "status" in h_clean:
                done_col = idx
            elif "time" in h_clean or "runtime" in h_clean or "duration" in h_clean:
                time_col = idx

        # Extract data rows
        for row in parser.testsummary_rows[header_row_idx + 1:]:
            if not row or len(row) <= mod_col:
                continue
            mod_name = row[mod_col].strip()
            if not mod_name or mod_name.lower() in ("module", "total"):
                continue

            passed = _parse_int(row[pass_col]) if pass_col is not None and pass_col < len(row) else 0
            failed = _parse_int(row[fail_col]) if fail_col is not None and fail_col < len(row) else 0
            assumption_failures = (
                _parse_int(row[assumption_col])
                if assumption_col is not None and assumption_col < len(row)
                else 0
            )
            ignored = (
                _parse_int(row[ignored_col])
                if ignored_col is not None and ignored_col < len(row)
                else 0
            )
            total = (
                _parse_int(row[total_col])
                if total_col is not None and total_col < len(row)
                else (passed + failed + assumption_failures + ignored)
            )
            done = _parse_bool(row[done_col]) if done_col is not None and done_col < len(row) else True
            time_str = row[time_col].strip() if time_col is not None and time_col < len(row) else ""

            modules.append(
                ModuleInfo(
                    name=mod_name,
                    passed=passed,
                    failed=failed,
                    assumption_failures=assumption_failures,
                    ignored=ignored,
                    total=total,
                    done=done,
                    time=time_str,
                    raw_cells=row,
                )
            )

    return metrics, modules


def format_cts_summary(metrics: dict[str, str], modules: list[ModuleInfo]) -> str:
    """Format extracted metrics and module breakdowns into a compact plain-text summary."""
    lines: list[str] = []
    divider = "=" * 80

    lines.append(divider)
    lines.append("CTS SUITE SUMMARY (Pre-processed from test_result_failures_suite.html)")
    lines.append(divider)

    has_assumptions = any(m.assumption_failures > 0 for m in modules)
    has_ignored = any(m.ignored > 0 for m in modules)

    # Standard metrics formatted
    if metrics:
        for k, v in metrics.items():
            lines.append(f"{k:<24}: {v}")
    else:
        # Synthesize from modules if summary table was absent
        total_passed = sum(m.passed for m in modules)
        total_failed = sum(m.failed for m in modules)
        total_assumptions = sum(m.assumption_failures for m in modules)
        total_ignored = sum(m.ignored for m in modules)
        done_count = sum(1 for m in modules if m.done)
        lines.append(f"{'Tests Passed':<24}: {total_passed}")
        lines.append(f"{'Tests Failed':<24}: {total_failed}")
        if total_assumptions > 0:
            lines.append(f"{'Assumption Skips':<24}: {total_assumptions}")
        if total_ignored > 0:
            lines.append(f"{'Ignored Skips':<24}: {total_ignored}")
        lines.append(f"{'Modules Done':<24}: {done_count}")
        lines.append(f"{'Modules Total':<24}: {len(modules)}")

    lines.append("")
    lines.append("CRITICAL RULE ON TEST STATUSES:")
    lines.append("- 'Failed' represents genuine functional test defects (RCA target).")
    lines.append("- 'Assumption Failure' and 'Ignored' are CONDITIONAL SKIPS (e.g. missing optional")
    lines.append("  hardware/features like work profiles, biometric sensors), NOT defects or regressions.")
    lines.append("- NEVER investigate, report, or formulate fixes for assumption failures or ignored tests.")

    # Calculate module counts
    failing_modules = [m for m in modules if m.failed > 0]
    # Sort failing modules descending by failed count, then total count
    failing_modules.sort(key=lambda m: (-m.failed, -m.total, m.name))

    incomplete_modules = [m for m in modules if not m.done]
    incomplete_modules.sort(key=lambda m: (-m.failed, m.name))

    lines.append("")
    lines.append(divider)
    lines.append("TOP FAILING MODULES (Ranked by Genuine Failure Count - Priority for Track A)")
    lines.append(divider)

    if failing_modules:
        for rank, m in enumerate(failing_modules, 1):
            status = "DONE" if m.done else "INCOMPLETE"
            time_str = f", Time: {m.time}" if m.time else ""
            extra_parts = []
            if m.assumption_failures > 0:
                extra_parts.append(f"Assumptions: {m.assumption_failures} (skip)")
            if m.ignored > 0:
                extra_parts.append(f"Ignored: {m.ignored} (skip)")
            extra_str = f" | {' | '.join(extra_parts)}" if extra_parts else ""
            lines.append(
                f"{rank:2d}. {m.name}\n"
                f"    Failed: {m.failed:<4} | Passed: {m.passed:<5} | Total: {m.total:<5} | Status: {status}{time_str}{extra_str}"
            )
    else:
        lines.append("No modules with genuine test failures detected.")

    lines.append("")
    lines.append(divider)
    lines.append("INCOMPLETE MODULES (Done = false - Priority for Track B)")
    lines.append(divider)

    if incomplete_modules:
        for rank, m in enumerate(incomplete_modules, 1):
            time_str = f", Time: {m.time}" if m.time else ""
            lines.append(
                f"{rank:2d}. {m.name}\n"
                f"    Failed: {m.failed:<4} | Passed: {m.passed:<5} | Total: {m.total:<5}{time_str}"
            )
    else:
        lines.append("All modules completed execution (no incomplete modules).")

    lines.append("")
    lines.append(divider)
    lines.append("ALL MODULES BREAKDOWN")
    lines.append(divider)

    # Table format
    if has_assumptions or has_ignored:
        hdr = f"{'Module':<42} | {'Passed':>7} | {'Failed':>7} | {'Assump':>6} | {'Ignored':>7} | {'Total':>7} | {'Done':>5}"
        lines.append(hdr)
        lines.append("-" * len(hdr))

        all_sorted = sorted(modules, key=lambda m: (-m.failed, not m.done, m.name))
        for m in all_sorted:
            display_name = m.name if len(m.name) <= 42 else m.name[:39] + "..."
            lines.append(
                f"{display_name:<42} | {m.passed:>7} | {m.failed:>7} | {m.assumption_failures:>6} | {m.ignored:>7} | {m.total:>7} | {str(m.done).lower():>5}"
            )
    else:
        hdr = f"{'Module':<52} | {'Passed':>7} | {'Failed':>7} | {'Total':>7} | {'Done':>5}"
        lines.append(hdr)
        lines.append("-" * len(hdr))

        all_sorted = sorted(modules, key=lambda m: (-m.failed, not m.done, m.name))
        for m in all_sorted:
            display_name = m.name if len(m.name) <= 52 else m.name[:49] + "..."
            lines.append(
                f"{display_name:<52} | {m.passed:>7} | {m.failed:>7} | {m.total:>7} | {str(m.done).lower():>5}"
            )

    lines.append(divider)
    return "\n".join(lines) + "\n"


def extract_cts_html_summary(
    html_path: Path | str,
    output_path: Path | str | None = None,
) -> str:
    """Read CTS HTML report, extract summary, optionally write to output_path, and return text."""
    p = Path(html_path)
    if not p.is_file():
        raise FileNotFoundError(f"CTS HTML file not found: {html_path}")

    # Read bounded content or full file
    # Even multi-MB HTML files have summary tables in the first 512 KB
    content = p.read_text(encoding="utf-8", errors="replace")
    metrics, modules = parse_cts_html(content)
    formatted = format_cts_summary(metrics, modules)

    if output_path:
        out_p = Path(output_path)
        out_p.parent.mkdir(parents=True, exist_ok=True)
        out_p.write_text(formatted, encoding="utf-8")
        LOG.info("Wrote CTS module summary to %s", out_p)

    return formatted


def generate_cts_summary(workspace: Path | str) -> Path | None:
    """Find test_result_failures_suite.html under workspace, parse it, and write cts_module_summary.txt."""
    ws = Path(workspace).resolve()
    target_out = ws / "test-results" / "cts_module_summary.txt"

    # Search common candidate locations
    candidates = [
        ws / "test-results" / "test_result_failures_suite.html",
        ws / "test-results" / "android-cts-results-html" / "test_result_failures_suite.html",
        ws / "android-cts-results-html" / "test_result_failures_suite.html",
        ws / "test-results" / "cts" / "test_result_failures_suite.html",
    ]

    found_html: Path | None = None
    for c in candidates:
        if c.is_file():
            found_html = c
            break

    if not found_html:
        # Bounded search under test-results/
        tr = ws / "test-results"
        if tr.is_dir():
            for root, _, files in os.walk(tr):
                if "test_result_failures_suite.html" in files:
                    found_html = Path(root) / "test_result_failures_suite.html"
                    break

    if not found_html:
        LOG.debug("No test_result_failures_suite.html found under %s", ws)
        return None

    try:
        formatted = extract_cts_html_summary(found_html, target_out)

        # Also write into gemini-assist directory for artifact publishing/archival
        gemini_out = ws / "gemini-assist" / "cts_module_summary.txt"
        try:
            gemini_out.parent.mkdir(parents=True, exist_ok=True)
            gemini_out.write_text(formatted, encoding="utf-8")
            LOG.info("Wrote CTS module summary to %s", gemini_out)
        except Exception as write_exc:
            LOG.warning("Failed to write CTS summary to %s: %s", gemini_out, write_exc)

        return target_out
    except Exception as exc:
        LOG.warning("Failed to generate CTS summary from %s: %s", found_html, exc)
        return None


def main() -> int:
    logging.basicConfig(level=logging.INFO, format="%(levelname)s: %(message)s")
    parser = argparse.ArgumentParser(
        description="Extract CTS summary & module rankings from test_result_failures_suite.html"
    )
    parser.add_argument("html_path", nargs="?", help="Path to test_result_failures_suite.html")
    parser.add_argument("output_path", nargs="?", help="Path to output cts_module_summary.txt")
    parser.add_argument("--workspace", help="Workspace root directory")

    args = parser.parse_args()

    if args.html_path:
        out = args.output_path or "cts_module_summary.txt"
        extract_cts_html_summary(args.html_path, out)
        print(f"Summary written to {out}")
        return 0

    ws = args.workspace or os.getenv("WORKSPACE", os.getcwd())
    generated = generate_cts_summary(ws)
    if generated:
        print(f"Summary generated at: {generated}")
        return 0

    print("No test_result_failures_suite.html found to process.", file=sys.stderr)
    return 1


if __name__ == "__main__":
    sys.exit(main())
