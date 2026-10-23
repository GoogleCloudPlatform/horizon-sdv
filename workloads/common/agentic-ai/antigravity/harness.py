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

from __future__ import annotations

import argparse
import asyncio
import json
import logging
import os
import re
import sys
import uuid
from pathlib import Path
from typing import Any

LOG = logging.getLogger("horizon.harness")


def apply_output_defaults(args: argparse.Namespace) -> argparse.Namespace:
    if not getattr(args, "markdown_output", None):
        args.markdown_output = str(Path("gemini-assist") / "agy_analysis_and_proposed_fix.md")
    return args


def _read_text(path: str | None, *, max_bytes: int) -> str:
    if not path:
        return ""
    p = Path(path)
    if not p.is_file():
        raise FileNotFoundError(path)

    data = p.read_bytes()
    if len(data) > max_bytes:
        # Keep the tail: build/test root-cause messages are commonly near the end.
        data = data[-max_bytes:]
        prefix = f"[... log truncated to final {max_bytes} bytes ...]\n"
    else:
        prefix = ""
    return prefix + data.decode("utf-8", errors="replace")


def _artifact_inventory(
    workspace: Path,
    *,
    max_entries: int = 80,
) -> list[str]:
    """Return a bounded test-results inventory without reading file content.

    Skip the walk when test-results/ is absent so AAOS/OpenBSW workspaces are
    not scanned for arbitrary .log/.txt files.
    """
    root = workspace / "test-results"
    if not root.is_dir():
        return []

    relevant_suffixes = {".json", ".log", ".txt", ".xml", ".zip"}
    excluded_dirs = {".git", ".repo", "out", "target", "node_modules", ".gemini"}
    excluded_names = {
        "logo.png",
        "compatibility_result.css",
        "diff.css",
        "package.json",
        "package-lock.json",
    }
    entries: list[str] = []

    for current, dirs, files in os.walk(root):
        dirs[:] = [
            name
            for name in sorted(dirs)
            if name not in excluded_dirs and not name.startswith("out_")
        ]
        for name in sorted(files):
            if name in excluded_names:
                continue
            path = Path(current) / name
            if path.suffix.lower() not in relevant_suffixes and name != "logcat":
                continue
            try:
                relative = path.relative_to(workspace)
                size = path.stat().st_size
            except (OSError, ValueError):
                continue
            entries.append(f"{relative.as_posix()} ({size} bytes)")
            if len(entries) >= max_entries:
                entries.append(f"[inventory truncated after {max_entries} files]")
                return entries
    return entries


def _safe_trace_component(
    value: str,
    *,
    default: str = "unknown",
) -> str:
    """Make a CI value safe for trace IDs and filenames."""
    value = (value or "").strip()

    if not value:
        return default

    value = re.sub(r"[^A-Za-z0-9._-]+", "_", value)
    value = re.sub(r"_+", "_", value)
    value = value.strip("._-")

    return value or default


def _resolve_ci_identity(
    args: argparse.Namespace,
) -> tuple[str, str, str]:
    """Resolve the CI system, job name and job/build ID."""

    job_name = (
        getattr(args, "job", None)
        or os.getenv("JOB_NAME")
        or "unknown-job"
    )

    job_id = (
        getattr(args, "build_id", None)
        or os.getenv("BUILD_NUMBER")
        or os.getenv("BUILD_ID")
        or "unknown-build"
    )

    return job_name, job_id


def build_trace_identity(
    args: argparse.Namespace,
) -> tuple[str, str, str]:
    """Create trace/workflow/session IDs for this CI run.

    trace_id:
        Stable for the CI job/build and suitable for correlation across
        Antigravity, Cloud Logging and the saved JSON trace.

    workflow_id:
        Logical CI workflow identifier.

    session_id:
        Unique Antigravity session identifier. Multiple agent sessions can
        therefore exist within one CI workflow while sharing the same trace_id.
    """

    job_name, job_id = _resolve_ci_identity(args)

    safe_job_name = _safe_trace_component(job_name)
    safe_job_id = _safe_trace_component(job_id)

    # trace_id is derived from CI job name + CI job ID.
    trace_id = (f"trace-{safe_job_name}-{safe_job_id}")

    # Keep the logical workflow identity separate from the trace ID.
    workflow_id = (f"wf-{job_name}:{job_id}")

    # One CI workflow can theoretically create more than one Antigravity
    # session, so keep the session ID unique.
    session_id = (f"session-{uuid.uuid4().hex[:12]}")

    return trace_id, workflow_id, session_id
def _preprocess_cts_artifacts(workspace: Path) -> None:
    """Extract compact module failure rankings and metrics from CTS HTML reports if present."""
    try:
        from cts_html_parser import generate_cts_summary

        summary = generate_cts_summary(workspace)
        if summary and summary.is_file():
            LOG.info("Pre-processed CTS HTML report into %s", summary)
    except Exception as exc:
        LOG.warning("Failed to pre-process CTS HTML report: %s", exc)


def build_failure_context(args: argparse.Namespace) -> str:
    """Build a bounded seed context. Remaining evidence is retrieved via workspace tools."""
    job_name, job_id = _resolve_ci_identity(args)

    parts: list[str] = [
        f"job={job_name}",
        f"build_id={job_id}",
        (
            "workspace="
            f"{args.workspace or os.getenv('WORKSPACE', os.getcwd())}"
        ),
    ]

    if args.failure_summary:
        parts.append("\nFailure summary:\n" + args.failure_summary.strip())

    log_text = _read_text(
        getattr(args, "log_file", None),
        max_bytes=getattr(args, "seed_log_bytes", 16 * 1024),
    )
    if log_text:
        parts.append("\nSeed failure log (bounded tail):\n" + log_text)

    if args.context_json:
        context: Any = json.loads(Path(args.context_json).read_text(encoding="utf-8"))
        parts.append(
            "\nCI metadata:\n"
            + json.dumps(context, indent=2, ensure_ascii=False, sort_keys=True)
        )

    workspace = Path(args.workspace or os.getenv("WORKSPACE", os.getcwd())).resolve()
    _preprocess_cts_artifacts(workspace)
    inventory = _artifact_inventory(workspace)
    if inventory:
        parts.append("\nAvailable staged artifacts:\n" + "\n".join(inventory))

    return "\n".join(parts)


async def _run(args: argparse.Namespace) -> int:
    try:
        from agy_agent import analyze_failure
    except ImportError as exc:
        raise RuntimeError(
            "Antigravity SDK dependencies are unavailable. "
            "Install dependencies before running an analysis."
        ) from exc

    # ------------------------------------------------------------------
    # Create the CI correlation IDs ONCE for this Harness invocation.
    # ------------------------------------------------------------------
    trace_id, workflow_id, session_id = build_trace_identity(args)

    LOG.info(
        "Starting Horizon Antigravity AI review: "
        "trace_id=%s workflow_id=%s session_id=%s",
        trace_id,
        workflow_id,
        session_id,
    )

    context = build_failure_context(args)
    workspace = Path(args.workspace or os.getenv("WORKSPACE", os.getcwd()))

    analysis = await analyze_failure(
        context,
        workflow_id=workflow_id,
        session_id=session_id,
        trace_id=trace_id,
        mcp_config=(
            None
            if args.no_mcp
            else Path(args.mcp_config)
        ),
        workspace=workspace,
    )

    payload = analysis.model_dump(mode="json")
    output = json.dumps(payload, indent=2, ensure_ascii=False)

    print(output)

    if args.markdown_output:
        markdown = Path(args.markdown_output)
        markdown.parent.mkdir(parents=True, exist_ok=True)
        markdown.write_text(render_markdown(payload), encoding="utf-8")
        LOG.info("markdown analysis written to %s (trace_id=%s)", markdown, trace_id)

        cts_summary = workspace / "test-results" / "cts_module_summary.txt"
        target_summary = markdown.parent / cts_summary.name
        if cts_summary.is_file() and not target_summary.is_file():
            try:
                target_summary.write_text(cts_summary.read_text(encoding="utf-8"), encoding="utf-8")
                LOG.info("Copied CTS summary into %s", target_summary)
            except Exception as copy_exc:
                LOG.warning("Failed to copy CTS summary to %s: %s", target_summary, copy_exc)

    LOG.info(
        "Horizon Antigravity AI review completed: "
        "trace_id=%s workflow_id=%s session_id=%s",
        trace_id,
        workflow_id,
        session_id,
    )

    # Analysis success is separate from original build success/failure.
    # Returning 0 lets ARGO/Jenkins publish the RCA artifact reliably.
    return 0


def parse_args() -> argparse.Namespace:
    here = Path(__file__).resolve().parent

    p = argparse.ArgumentParser(
        description="Horizon SDV Antigravity CI failure-analysis harness"
    )
    p.add_argument("--log-file", help="Failed build/test log to seed the analysis")
    p.add_argument("--failure-summary", help="Short failure/error string from CI")
    p.add_argument("--context-json", help="Optional CI metadata JSON file")
    p.add_argument("--job")
    p.add_argument("--build-id")
    p.add_argument("--workspace")
    p.add_argument(
        "--seed-log-bytes",
        type=int,
        default=16 * 1024,
        help="Maximum bytes copied from the tail of --log-file into the initial prompt",
    )
    p.add_argument(
        "--mcp-config",
        default=str(here / "mcp_config.json"),
    )
    p.add_argument(
        "--markdown-output",
        default=os.getenv("HORIZON_AGY_MARKDOWN_OUTPUT"),
    )
    p.add_argument(
        "--no-mcp",
        action="store_true",
        help="Disable MCP servers and use built-in read-only workspace tools only.",
    )
    p.add_argument(
        "--verbose",
        action="store_true",
    )
    return apply_output_defaults(p.parse_args())


def render_markdown(payload: dict[str, Any]) -> str:
    sections = [
        "# Antigravity Analysis",
        f"- **Model:** {payload.get('model') or 'unknown'}",
        f"- **Tools used:** {', '.join(payload.get('tools_used') or []) or 'none'}",
        f"- **Status:** {payload.get('status', 'unknown')}",
        f"- **Classification:** {payload.get('classification', 'unknown')}",
        f"- **Confidence:** {payload.get('confidence', 0)}",
        "",
        "## Summary",
        str(payload.get("summary", "")),
        "",
        "## Root Cause",
        str(payload.get("root_cause", "")),
        "",
    ]
    assumptions = payload.get("assumptions") or []
    if assumptions:
        sections.extend(["", "## Assumptions"])
        for assumption in assumptions:
            sections.append(
                f"- **Assumption:** {assumption}"
            )
    findings = payload.get("findings") or []
    has_finding_fixes = False

    if findings:
        sections.extend(["", "## Findings"])
        for finding in findings:
            sections.extend(
                [
                    "",
                    f"### {finding.get('failure_id', 'Unknown failure')}",
                    f"- **Category:** {finding.get('category', 'unknown')}",
                    f"- **Confidence:** {finding.get('confidence', 0)}",
                    f"- **Summary:** {finding.get('summary', '')}",
                    f"- **Root cause:** {finding.get('root_cause', '')}",
                ]
            )
            # Safe evidence formatting (supports dict or string)
            for item in finding.get("evidence") or []:
                if isinstance(item, dict):
                    source = item.get("source") or "unknown"
                    location = f" ({item['location']})" if item.get("location") else ""
                    detail = item.get("detail") or ""
                    sections.append(
                        f"- **Evidence:** `{source}`{location}: {detail}"
                    )
                elif isinstance(item, str) and item.strip():
                    sections.append(f"- **Evidence:** {item.strip()}")

            # Per-finding recommended fixes
            fixes = finding.get("recommended_fix") or []
            if isinstance(fixes, dict):
                fixes = [fixes]
            for fix in fixes:
                has_finding_fixes = True
                sections.extend(["", "### Recommended Fix", str(fix.get("summary", ""))])
                for label, values in (
                    ("Files", fix.get("files") or []),
                    ("Suggested Commands", fix.get("commands") or []),
                    ("Suggested Patches", fix.get("patches") or []),
                    ("Verification", fix.get("verification") or []),
                ):
                    if values:
                        sections.extend(["", f"#### {label}", *[f"- `{item}`" if label != "Verification" else f"- {item}" for item in values]])

    # Top-level Evidence 
    top_level_evidence = list(payload.get("evidence") or [])
    evidence_lines: list[str] = []
    for item in top_level_evidence:
        if isinstance(item, dict):
            source = item.get("source") or "unknown"
            location = f" ({item['location']})" if item.get("location") else ""
            detail = item.get("detail") or ""
            evidence_lines.append(f"- `{source}`{location}: {detail}")
        elif isinstance(item, str) and item.strip():
            evidence_lines.append(f"- {item.strip()}")
    if evidence_lines:
        sections.extend(["", "## Evidence", *evidence_lines])
    elif not findings:
        sections.extend(["", "## Evidence", "- No supporting evidence was returned."])

    # Top-level Recommended Fix (one object; a list is accepted from older output)
    global_fix = payload.get("recommended_fix") or {}
    if isinstance(global_fix, list):
        dicts = [item for item in global_fix if isinstance(item, dict)]
        global_fix = dicts[0] if dicts else {}
    if isinstance(global_fix, str) and global_fix.strip():
        global_fix = {"summary": global_fix.strip()}

    if global_fix and (not has_finding_fixes or not findings):
        summary = str(global_fix.get("summary", "")).strip()
        if summary:
            sections.extend(["", "## Recommended Fix", summary])
            files = global_fix.get("files") or []
            commands = global_fix.get("commands") or []
            patches = global_fix.get("patches") or []
            verification = global_fix.get("verification") or []
            if files:
                sections.extend(["", "### Files", *[f"- `{item}`" for item in files]])
            if commands:
                sections.extend(["", "### Suggested Commands", *[f"- `{item}`" for item in commands]])
            if patches:
                sections.extend(["", "### Suggested Patches", *[f"- `{item}`" for item in patches]])
            if verification:
                sections.extend(["", "### Verification", *[f"- {item}" for item in verification]])

    sections.append("")
    return "\n".join(sections)


def main() -> int:
    args = parse_args()
    logging.basicConfig(
        level=logging.DEBUG if args.verbose else logging.INFO,
        format="%(asctime)s %(levelname)s %(name)s %(message)s",
    )

    try:
        return asyncio.run(_run(args))
    except Exception:
        LOG.exception("Horizon Antigravity analysis failed")
        return 2


if __name__ == "__main__":
    sys.exit(main())
