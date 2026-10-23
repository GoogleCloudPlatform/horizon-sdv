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

"""
Parser for incomplete or fallback responses emitted by the Horizon SDV
CI Failure Analysis Agent (e.g. from forced invocation cutoffs or text-based fallbacks).
"""

from __future__ import annotations

import inspect
import json
import logging
import re
from typing import TYPE_CHECKING, Any

try:
    import yaml
except ImportError:
    yaml = None  # type: ignore[assignment]

if TYPE_CHECKING:
    from agy_agent import Evidence, Finding, HorizonAnalysis, RecommendedFix

LOG = logging.getLogger("horizon.agy.parser")

try:
    from agy_agent import Evidence, Finding, HorizonAnalysis, RecommendedFix
except ImportError:
    Evidence = Finding = HorizonAnalysis = RecommendedFix = None  # type: ignore


def _resolve_models():
    """Lazily resolve models from agy_agent to avoid circular import issues."""
    global Evidence, Finding, HorizonAnalysis, RecommendedFix
    if HorizonAnalysis is None:
        from agy_agent import (  # type: ignore
            Evidence as _Ev,
            Finding as _Fi,
            HorizonAnalysis as _Ha,
            RecommendedFix as _Rf,
        )
        Evidence, Finding, HorizonAnalysis, RecommendedFix = _Ev, _Fi, _Ha, _Rf
    return Evidence, Finding, RecommendedFix, HorizonAnalysis


def extract_text_from_turn(turn: Any) -> str:
    """Extract candidate response text from a post_turn payload."""
    if turn is None:
        return ""
    candidates: list[Any] = []
    if isinstance(turn, dict):
        candidates.extend([turn.get("responseText"), turn.get("response_text"), turn.get("text"), turn.get("content")])
        post_args = turn.get("postTurnArgs") or turn.get("post_turn_args")
        if isinstance(post_args, dict):
            candidates.extend([post_args.get("responseText"), post_args.get("response_text"), post_args.get("text")])
    else:
        for attr in ("responseText", "response_text", "text", "content"):
            candidates.append(getattr(turn, attr, None))
        post_args = getattr(turn, "postTurnArgs", None) or getattr(turn, "post_turn_args", None)
        if post_args is not None:
            if isinstance(post_args, dict):
                candidates.extend([post_args.get("responseText"), post_args.get("response_text"), post_args.get("text")])
            else:
                for attr in ("responseText", "response_text", "text"):
                    candidates.append(getattr(post_args, attr, None))
    for c in candidates:
        if isinstance(c, str) and c.strip():
            return c.strip()
    return ""


async def extract_response_text(response: Any, fallback_text: str = "") -> str:
    """Safely extract response text from the agent response object or recorded turn fallback."""
    if response is not None:
        for attr in ("text", "response_text", "responseText", "content", "output"):
            val = getattr(response, attr, None)
            if val is None:
                continue
            if callable(val):
                try:
                    res = val()
                    if inspect.isawaitable(res):
                        res = await res
                    if isinstance(res, str) and res.strip():
                        return res.strip()
                except Exception as exc:
                    LOG.debug("Calling response.%s() failed: %r", attr, exc)
            elif isinstance(val, str) and val.strip():
                return val.strip()

    if fallback_text and fallback_text.strip():
        return fallback_text.strip()

    if response is not None:
        try:
            s = str(response)
            if s and not s.startswith("<"):
                return s
        except Exception:
            pass

    return ""


def _sanitize_evidence_list(raw_evidence: Any) -> list[Evidence]:
    Evidence, _, _, _ = _resolve_models()
    if not isinstance(raw_evidence, list):
        return []
    cleaned: list[Evidence] = []
    for ev in raw_evidence:
        if isinstance(ev, dict) and "source" in ev and "detail" in ev:
            cleaned.append(
                Evidence(
                    source=str(ev["source"]),
                    detail=str(ev["detail"]),
                    location=str(ev["location"]) if ev.get("location") is not None else None,
                )
            )
        elif isinstance(ev, str) and ev.strip():
            cleaned.append(Evidence(source="artifact", detail=ev.strip(), location=None))
    return cleaned


def _sanitize_finding(f: dict[str, Any]) -> Finding:
    _, Finding, _, _ = _resolve_models()
    fid = str(f.get("failure_id") or "UNKNOWN_FAILURE")
    cat = str(f.get("category") or "NEW_REGRESSION")
    summ = str(f.get("summary") or fid)
    rc = str(f.get("root_cause") or summ)
    conf = f.get("confidence")
    if not isinstance(conf, (int, float)):
        conf = 0.8
    else:
        conf = max(0.0, min(1.0, float(conf)))

    evidence = _sanitize_evidence_list(f.get("evidence", []))
    return Finding(
        failure_id=fid,
        category=cat,
        summary=summ,
        root_cause=rc,
        confidence=conf,
        evidence=evidence,
        recommended_fix=_sanitize_fix_list(f.get("recommended_fix")),
    )


def _sanitize_recommended_fix(raw_fix: Any) -> RecommendedFix:
    _, _, RecommendedFix, _ = _resolve_models()
    if isinstance(raw_fix, str):
        return RecommendedFix(summary=raw_fix)
    if isinstance(raw_fix, dict):
        summary = str(raw_fix.get("summary") or "Address findings identified during test execution.")
        files = [str(f) for f in raw_fix.get("files", []) if isinstance(f, (str, int))]
        commands = [str(c) for c in raw_fix.get("commands", []) if isinstance(c, (str, int))]
        verification = [str(v) for v in raw_fix.get("verification", []) if isinstance(v, (str, int))]
        patches = [str(p) for p in raw_fix.get("patches", []) if isinstance(p, (str, int))]
        return RecommendedFix(
            summary=summary,
            files=files,
            commands=commands,
            verification=verification,
            patches=patches,
        )
    return RecommendedFix(summary="Address findings identified during test execution.")

def _sanitize_fix_list(raw_fix: Any) -> list[RecommendedFix]:
    if isinstance(raw_fix, list):
        items = raw_fix
    elif isinstance(raw_fix, (dict, str)):
        items = [raw_fix]
    else:
        return []
    return [_sanitize_recommended_fix(item) for item in items]

def _sanitize_analysis_dict(data: dict[str, Any], default_classification: str) -> HorizonAnalysis:
    _, _, _, HorizonAnalysis = _resolve_models()
    classification = str(data.get("classification") or default_classification)
    status = data.get("status")
    if status not in {"analyzed", "insufficient_evidence", "unsupported"}:
        status = "analyzed" if (data.get("root_cause") or data.get("findings")) else "insufficient_evidence"

    conf = data.get("confidence")
    if not isinstance(conf, (int, float)):
        conf = 0.8
    else:
        conf = max(0.0, min(1.0, float(conf)))

    summary = str(data.get("summary") or f"MODE: {classification.upper()}. Analysis recovered.")
    root_cause = str(data.get("root_cause") or "No root cause identified.")

    raw_findings = data.get("findings", [])
    findings: list[Finding] = []
    if isinstance(raw_findings, list):
        for f in raw_findings:
            if isinstance(f, dict):
                findings.append(_sanitize_finding(f))

    evidence = _sanitize_evidence_list(data.get("evidence", []))
    recommended_fix = _sanitize_recommended_fix(data.get("recommended_fix"))

    raw_assumptions = data.get("assumptions", [])
    assumptions: list[str] = []
    if isinstance(raw_assumptions, list):
        assumptions = [str(a) for a in raw_assumptions if isinstance(a, (str, int))]

    return HorizonAnalysis(
        status=status,  # type: ignore[arg-type]
        classification=classification,
        confidence=conf,
        summary=summary,
        root_cause=root_cause,
        findings=findings,
        evidence=evidence,
        recommended_fix=recommended_fix,
        assumptions=assumptions,
    )


def parse_incomplete_response(response_text: str, failure_context: str) -> HorizonAnalysis:
    """
    Recover a valid HorizonAnalysis when the agent emitted an incomplete response,
    markdown, or text instead of a structured JSON tool call (e.g. after a forced invocation cutoff).
    """
    _, _, RecommendedFix, HorizonAnalysis = _resolve_models()
    text = (response_text or "").strip()
    ctx_lower = failure_context.lower()
    text_lower = text.lower()

    if "cts" in text_lower or "cts" in ctx_lower:
        classification = "cts"
    elif "cvd" in text_lower or "cvd" in ctx_lower or "cuttlefish" in text_lower:
        classification = "cvd_launcher"
    elif "bsw" in text_lower or "bsw" in ctx_lower or "openbsw" in text_lower:
        classification = "openbsw"
    elif "aaos_build" in text_lower or "aaos" in ctx_lower:
        classification = "aaos_build"
    else:
        classification = "unknown"

    if not text:
        return HorizonAnalysis(
            status="insufficient_evidence",
            classification=classification,
            confidence=0.0,
            summary="Agent execution ended without structured output or response text.",
            root_cause="Execution was truncated before analysis output could be produced.",
            findings=[],
            evidence=[],
            recommended_fix=RecommendedFix(
                summary="Inspect raw test-results artifacts or rerun with a narrower query scope."
            ),
            assumptions=["Structured output tool call was omitted; response text was empty."],
        )

    # Try parsing text as YAML / JSON structured output
    candidates: list[str] = [text]
    for block in re.findall(r"```(?:yaml|json)?\s*\n(.*?)\n```", text, re.DOTALL | re.IGNORECASE):
        if block.strip():
            candidates.append(block.strip())

    for key in ("CtsFailureAnalysis:", "HorizonAnalysis:", "analysis:", "FailureAnalysis:"):
        idx = text.find(key)
        if idx != -1:
            candidates.append(text[idx:].strip())

    json_start = text.find("{")
    json_end = text.rfind("}")
    if json_start != -1 and json_end > json_start:
        candidates.append(text[json_start : json_end + 1].strip())

    for candidate in candidates:
        parsed = None
        if yaml is not None:
            try:
                parsed = yaml.safe_load(candidate)
            except Exception:
                pass
        if parsed is None:
            try:
                parsed = json.loads(candidate)
            except Exception:
                pass

        if isinstance(parsed, dict):
            for key in ("CtsFailureAnalysis", "HorizonAnalysis", "analysis", "Analysis", "failure_analysis", "FailureAnalysis"):
                if key in parsed:
                    inner = parsed[key]
                    if isinstance(inner, list) and inner and isinstance(inner[0], dict):
                        parsed = inner[0]
                        break
                    elif isinstance(inner, dict):
                        parsed = inner
                        break

            if isinstance(parsed, dict) and any(k in parsed for k in ("root_cause", "summary", "findings", "recommended_fix")):
                try:
                    analysis = _sanitize_analysis_dict(parsed, classification)
                    LOG.info("Successfully recovered HorizonAnalysis from structured YAML/JSON fallback.")
                    return analysis
                except Exception as exc:
                    LOG.debug("Failed building HorizonAnalysis from dict: %r", exc)

        elif isinstance(parsed, list) and parsed and isinstance(parsed[0], dict):
            first = parsed[0]
            if any(k in first for k in ("root_cause", "summary", "findings", "recommended_fix")):
                try:
                    analysis = _sanitize_analysis_dict(first, classification)
                    LOG.info("Successfully recovered HorizonAnalysis from structured YAML/JSON list fallback.")
                    return analysis
                except Exception as exc:
                    LOG.debug("Failed building HorizonAnalysis from list item: %r", exc)

    # Extract Root Cause section
    rc_match = re.search(
        r"#{1,4}\s*Root\s*Cause\s*\n+(.*?)(?=\n+#{1,4}|$)",
        text,
        re.DOTALL | re.IGNORECASE,
    )
    root_cause = rc_match.group(1).strip() if rc_match else ""

    # Extract Recommended Fix section
    fix_match = re.search(
        r"#{1,4}\s*Recommended\s*Fix\s*\n+(.*?)(?=\n+#{1,4}|$)",
        text,
        re.DOTALL | re.IGNORECASE,
    )
    fix_text = fix_match.group(1).strip() if fix_match else ""

    # Extract Findings section
    findings_match = re.search(
        r"#{1,4}\s*Findings\s*\n+(.*?)(?=\n+#{1,4}|$)",
        text,
        re.DOTALL | re.IGNORECASE,
    )
    findings_text = findings_match.group(1).strip() if findings_match else ""

    # Parse findings from table rows or bullets
    _, Finding, _, _ = _resolve_models()
    findings: list[Finding] = []
    if findings_text:
        for line in findings_text.splitlines():
            line = line.strip()
            if not line.startswith("|") or line.startswith("| ---") or line.startswith("|---"):
                continue
            cols = [c.strip().strip("`") for c in line.split("|")[1:-1]]
            if len(cols) >= 3:
                fid, cat, summ = cols[0], cols[1], cols[2]
                if fid.lower() in {"failure id", "failure_id", "id"}:
                    continue
                findings.append(
                    Finding(
                        failure_id=fid,
                        category=cat or "NEW_REGRESSION",
                        summary=summ or fid,
                        root_cause=root_cause[:300] if root_cause else summ,
                        confidence=0.8,
                        evidence=[],
                    )
                )

    if not findings and findings_text:
        for line in findings_text.splitlines():
            line = line.strip()
            bullet_match = re.match(r"^[-*]\s*`?([^`:]+)`?:\s*(.*)$", line)
            if bullet_match:
                fid, summ = bullet_match.group(1).strip(), bullet_match.group(2).strip()
                findings.append(
                    Finding(
                        failure_id=fid,
                        category="NEW_REGRESSION",
                        summary=summ or fid,
                        root_cause=root_cause[:300] if root_cause else summ,
                        confidence=0.75,
                        evidence=[],
                    )
                )

    # Extract commands from fix_text
    commands: list[str] = []
    for cmd_block in re.findall(r"```(?:bash|sh)?\n(.*?)```", fix_text, re.DOTALL):
        for cmd_line in cmd_block.splitlines():
            cmd_line = cmd_line.strip()
            if cmd_line and not cmd_line.startswith("#"):
                commands.append(cmd_line)

    mode_match = re.search(r"(MODE:\s*(?:CTS_SUITE|CVD_ONLY).*?)(?:\n\n|\n#{1,4}|$)", text, re.IGNORECASE)
    if mode_match:
        summary = mode_match.group(1).strip().replace("\n", " ")
    else:
        first_para = text.split("\n\n")[0].strip().replace("\n", " ")
        summary = first_para

    if len(summary) > 120:
        summary = summary[:117] + "..."

    return HorizonAnalysis(
        status="analyzed" if (root_cause or findings) else "insufficient_evidence",
        classification=classification,
        confidence=0.75 if (root_cause or findings) else 0.2,
        summary=summary or f"MODE: {classification.upper()}. Analysis recovered from response text.",
        root_cause=root_cause or (text[:300] if text else "No root cause identified."),
        findings=findings,
        evidence=[],
        recommended_fix=RecommendedFix(
            summary=fix_text or "Address findings identified during test execution.",
            commands=commands,
        ),
        assumptions=["Structured output tool call was omitted; response recovered from markdown text."],
    )


# Backwards compatibility aliases
parse_fallback_analysis = parse_incomplete_response
_parse_fallback_analysis = parse_incomplete_response
