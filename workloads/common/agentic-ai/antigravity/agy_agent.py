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

import json
import logging
import os
from contextvars import ContextVar
import re
from pathlib import Path
from typing import Any, Literal

from pydantic import BaseModel, Field

from google.antigravity import Agent, LocalAgentConfig, policy, types
from google.antigravity.hooks import hooks

from trace import AgentCloudLoggingReporter, json_safe


LOG = logging.getLogger("horizon.agy")

# Ordered unique tool names observed during the current analyze_failure run.
_RECORDED_TOOLS: list[str] = []
_LAST_RESPONSE_TEXT: str = ""

DEFAULT_SKILLS_DIR = Path(__file__).resolve().parent / "skills"
DEFAULT_MCP_CONFIG = Path(__file__).resolve().parent / "mcp_config.json"
COMMON_SYSTEM_INSTRUCTIONS_PATH = Path(__file__).resolve().parent / "common_system_instructions.txt"


# ---------------------------------------------------------------------------
# Per-run reporter context
# ---------------------------------------------------------------------------

_active_reporter: ContextVar[
    AgentCloudLoggingReporter | None
] = ContextVar(
    "horizon_active_reporter",
    default=None,
)


def set_active_reporter(
    reporter: AgentCloudLoggingReporter,
) -> None:
    _active_reporter.set(reporter)


def clear_active_reporter() -> None:
    _active_reporter.set(None)


def get_reporter() -> AgentCloudLoggingReporter:
    reporter = _active_reporter.get()
    if reporter is None:
        raise RuntimeError(
            "Horizon Cloud Logging reporter is not initialized"
        )
    return reporter


# Ordered unique tool names observed during the current analyze_failure run.
_RECORDED_TOOLS: list[str] = []


class Evidence(BaseModel):
    source: str = Field(description="Tool, log, source file, or artifact that supports the finding.")
    detail: str = Field(description="Concise factual evidence.")
    location: str | None = Field(default=None, description="File path, test name, log range, etc.")


class RecommendedFix(BaseModel):
    summary: str
    files: list[str] = Field(default_factory=list)
    commands: list[str] = Field(
        default_factory=list,
        description="Suggested commands only. They must not be claimed as executed.",
    )
    patches: list[str] = Field(default_factory=list)
    verification: list[str] = Field(default_factory=list)


class Finding(BaseModel):
    failure_id: str = Field(
        description="Test#Method, guest/host failure ID, or build blocker ID."
    )
    category: str
    summary: str
    root_cause: str
    confidence: float = Field(ge=0.0, le=1.0)
    evidence: list[Evidence] = Field(default_factory=list)
    recommended_fix: list[RecommendedFix] = Field(default_factory=list)


class HorizonAnalysis(BaseModel):
    status: Literal["analyzed", "insufficient_evidence", "unsupported"]
    model: str = Field(default="")
    classification: str
    confidence: float = Field(ge=0.0, le=1.0)
    summary: str
    root_cause: str
    findings: list[Finding] = Field(default_factory=list)
    evidence: list[Evidence] = Field(default_factory=list)
    recommended_fix: RecommendedFix | None = None
    tools_used: list[str] = Field(
        default_factory=list,
        description="Filled by the harness from actual tool calls, not the LLM.",
    )
    assumptions: list[str] = Field(default_factory=list)


from incomplete_response_parser import (
    extract_response_text,
    extract_text_from_turn,
    parse_fallback_analysis,
    parse_incomplete_response,
)

_parse_fallback_analysis = parse_incomplete_response


COMMON_SYSTEM_INSTRUCTIONS = (
    COMMON_SYSTEM_INSTRUCTIONS_PATH.read_text(encoding="utf-8").strip()
)

# ---------------------------------------------------------------------------
# Runtime tool tracking
# ---------------------------------------------------------------------------

def _tool_name(obj: Any) -> str:
    name = getattr(obj, "name", None)
    if name is None:
        return ""
    value = getattr(name, "value", name)
    return str(value).strip()


def _record_tool(name: str) -> None:
    if not name:
        return
    if name not in _RECORDED_TOOLS:
        _RECORDED_TOOLS.append(name)
    try:
        reporter = _active_reporter.get()
        if reporter is not None:
            reporter.record_tool_name(name)
    except Exception:
        pass


def discover_skill_paths(skills_dir: Path) -> list[str]:
    """Return deterministic absolute paths for a skill or skills parent."""
    if not skills_dir.exists():
        raise FileNotFoundError(f"Skills directory does not exist: {skills_dir}")

    if (skills_dir / "SKILL.md").is_file():
        return [str(skills_dir.resolve())]

    found = sorted(
        str(p.resolve())
        for p in skills_dir.iterdir()
        if p.is_dir() and (p / "SKILL.md").is_file()
    )
    if not found:
        raise RuntimeError(f"No SKILL.md files found below {skills_dir}")
    return found


def _expand_env(value: str) -> str:
    return os.path.expandvars(value)


def load_mcp_servers(config_path: Path | None) -> list[types.McpStdioServer]:
    """
    Load stdio MCP servers from mcp_config.json.

    Expected shape:
      {
        "mcpServers": {
          "name": {
            "transport": "stdio",
            "command": "python3",
            "args": ["..."],
            "env": {"KEY": "VALUE"}
          }
        }
      }
    """
    if config_path is None:
        return []
    if not config_path.is_file():
        raise FileNotFoundError(f"MCP config does not exist: {config_path}")

    raw = json.loads(config_path.read_text(encoding="utf-8"))
    servers: list[types.McpStdioServer] = []

    for name, cfg in raw.get("mcpServers", {}).items():
        transport = cfg.get("transport", cfg.get("type", "stdio"))
        if transport != "stdio":
            raise ValueError(
                f"MCP server {name!r} uses {transport!r}; this Horizon reference "
                "implementation intentionally permits stdio only."
            )

        env = cfg.get("env")
        if env:
            env = {str(k): _expand_env(str(v)) for k, v in env.items()}

        kwargs: dict[str, Any] = {
            "name": name,
            "command": _expand_env(str(cfg["command"])),
            "args": [_expand_env(str(x)) for x in cfg.get("args", [])],
            "env": env,
        }

        servers.append(types.McpStdioServer(**kwargs))

    return servers


# ---------------------------------------------------------------------------
# Antigravity hooks
# ---------------------------------------------------------------------------

@hooks.on_session_start
async def _session_start() -> None:
    reporter = get_reporter()
    LOG.info(
        "Antigravity session started workflow=%s session=%s",
        reporter.workflow_id,
        reporter.session_id,
    )
    reporter.log(
        "SESSION_START",
        "Agent session initialized",
        details={
            "workflow_id": reporter.workflow_id,
            "session_id": reporter.session_id,
            "model": reporter.model,
        },
    )


@hooks.on_session_end
async def _session_end() -> None:
    reporter = get_reporter()
    LOG.info(
        "Antigravity session ended workflow=%s session=%s",
        reporter.workflow_id,
        reporter.session_id,
    )
    reporter.log(
        "SESSION_END",
        "Agent session terminated successfully",
        details={},
    )


@hooks.pre_tool_call_decide
async def _pre_tool_call(
    call: types.ToolCall,
) -> types.HookResult:
    """
    Final fail-closed guardrail.
    mcp_config.json already hides non-allowed MCP tools. This hook additionally
    denies dangerous built-in tool names and unconstrained runaway queries.
    """
    reporter = get_reporter()

    name = _tool_name(call)
    call_id = getattr(call, "id", None)
    step_id = getattr(call, "step_id", None)
    server_name = getattr(call, "server_name", None) or "builtin"
    args = getattr(call, "args", None)

    tool_data = {
        "call_id": call_id,
        "step_id": step_id,
        "server_name": server_name,
        "tool_name": name,
        "args": json_safe(args),
    }

    reporter.log(
        "TOOL_INVOCATION",
        "Tool Requested",
        details=tool_data,
    )

    reporter.start_tool_call(
        call_id=call_id,
        server_name=server_name,
        tool_name=name,
    )

    # MCP tools, if configured, must be added to the allowed_tools list.
    # If not the MCP tool will be rejected by this hook
    allowed_tools = {
        types.BuiltinTools.LIST_DIR.value,
        types.BuiltinTools.SEARCH_DIR.value,
        types.BuiltinTools.FIND_FILE.value,
        types.BuiltinTools.VIEW_FILE.value,
        types.BuiltinTools.FINISH.value,
    }

    if name not in allowed_tools:
        LOG.warning("Tool denied by read-only policy: %s", name)
        reporter.complete_tool_call(
            call_id=call_id,
            server_name=server_name,
            tool_name=name,
            status="denied",
        )
        reporter.log(
            "TOOL_DENIED",
            "Tool denied by Horizon policy",
            details={
                **tool_data,
                "reason": "read_only_policy",
            },
            severity="WARNING",
        )
        return types.HookResult(allow=False)

    call_args = getattr(call, "args", {}) or {}
    if isinstance(call_args, str):
        try:
            call_args = json.loads(call_args)
        except Exception:
            call_args = {}

    # Guard against broad generic searches that blow up context and exhaust TPM quota
    if name in {"search_directory", "search_dir"}:
        query = str(call_args.get("Query") or call_args.get("query") or "").strip().strip("'\"").lower()
        if query in {"fail", "failed", "error", "errors", "exception", "crash"}:
            msg = (
                f"tool_denied: search query {query!r} across {call_args.get('SearchPath') or call_args.get('search_path')!r} "
                "is too broad. Please consult the relevant classification SKILL.md (e.g. skills/cts_execution/SKILL.md) "
                "for recommended query patterns and artifact triage workflow."
            )
            LOG.warning(msg)
            reporter.complete_tool_call(
                call_id=call_id,
                server_name=server_name,
                tool_name=name,
                status="denied",
            )
            reporter.log(
                "TOOL_DENIED",
                msg,
                details={**tool_data, "reason": "search_query_too_broad", "hint": "Consult the classification SKILL.md"},
                severity="WARNING",
            )
            return types.HookResult(allow=False)
        search_path_raw = str(call_args.get("SearchPath") or call_args.get("search_path") or "")
        search_path_lower = search_path_raw.lower()
        if search_path_lower.endswith((".txt", ".xml", ".html", ".htm", ".json", ".log")):
            parent_dir = str(Path(search_path_raw).parent)
            repaired = False
            raw_args = getattr(call, "args", None)
            if isinstance(raw_args, dict):
                for k in ("SearchPath", "search_path"):
                    if k in raw_args:
                        raw_args[k] = parent_dir
                        repaired = True
            elif isinstance(raw_args, str):
                try:
                    d = json.loads(raw_args)
                    for k in ("SearchPath", "search_path"):
                        if k in d:
                            d[k] = parent_dir
                            call.args = json.dumps(d)
                            repaired = True
                except Exception:
                    pass

            if repaired:
                LOG.info(
                    "auto-corrected search_directory SearchPath from file %r to directory %r",
                    search_path_raw,
                    parent_dir,
                )
            else:
                LOG.warning(
                    "tool_denied: search_directory SearchPath %r is a file, not a directory",
                    search_path_lower,
                )
                return types.HookResult(allow=False)

    # Guard against viewing massive HTML suite reports
    if name in {"view_file", "read_file"}:
        target_path = str(
            call_args.get("AbsolutePath")
            or call_args.get("filePath")
            or call_args.get("file_path")
            or call_args.get("path")
            or ""
        ).lower()
        if target_path.endswith((".html", ".htm")):
            msg = (
                f"tool_denied: viewing HTML suite report {target_path!r} is forbidden; "
                "use compact invocation_summary.txt, cts_module_summary.txt, or targeted XML queries per SKILL.md"
            )
            LOG.warning(msg)
            reporter.complete_tool_call(
                call_id=call_id,
                server_name=server_name,
                tool_name=name,
                status="denied",
            )
            reporter.log(
                "TOOL_DENIED",
                msg,
                details={**tool_data, "reason": "html_view_forbidden", "hint": "Use invocation_summary.txt or cts_module_summary.txt"},
                severity="WARNING",
            )
            return types.HookResult(allow=False)

    _record_tool(name)
    return types.HookResult(allow=True)


@hooks.post_tool_call
async def _post_tool_call(result: Any) -> None:
    reporter = get_reporter()
    name = _tool_name(result)
    call_id = getattr(result, "id", None)
    step_id = getattr(result, "step_id", None)
    server_name = getattr(result, "server_name", None)
    error = getattr(result, "error", None)

    tool_data = {
        "call_id": call_id,
        "step_id": step_id,
        "server_name": server_name,
        "tool_name": name,
        "args": json_safe(getattr(result, "args", None)),
        "result": json_safe(getattr(result, "result", None)),
        "error": json_safe(error),
    }

    status = "error" if error else "completed"
    reporter.complete_tool_call(
        call_id=call_id,
        server_name=server_name,
        tool_name=name,
        status=status,
    )

    reporter.log(
        "TOOL_COMPLETED" if not error else "TOOL_ERROR",
        "Tool execution completed" if not error else "Tool execution failed",
        details=tool_data,
        severity="ERROR" if error else "INFO",
    )

    _record_tool(name)


@hooks.on_tool_error
async def _tool_error(error: Exception) -> None:
    reporter = get_reporter()
    call_id = getattr(error, "call_id", None)
    tool_name = getattr(error, "tool_name", "unknown")

    reporter.complete_tool_call(
        call_id=call_id,
        server_name=None,
        tool_name=tool_name,
        status="error",
    )
    reporter.log(
        "TOOL_ERROR",
        "Tool execution failed",
        details={
            "tool_name": tool_name,
            "call_id": call_id,
            "error": str(error),
            "error_type": type(error).__name__,
        },
        severity="ERROR",
    )
    LOG.exception("Antigravity tool failed: %s", error)


@hooks.pre_turn
async def _pre_turn(data: Any) -> types.HookResult:
    reporter = get_reporter()
    turn_number = reporter.start_turn()

    prompt_len = len(data) if isinstance(data, (str, list, dict)) else 0
    reporter.log(
        "TURN_STARTED",
        f"Started turn {turn_number}",
        details={
            "turn_number": turn_number,
            "prompt_length": prompt_len,
        },
    )
    return types.HookResult(allow=True)


@hooks.post_turn
async def _post_turn(turn: Any) -> None:
    global _LAST_RESPONSE_TEXT

    # 1. Finalize Cloud Logging reporter turn metrics
    reporter = get_reporter()
    turn_number, duration_ms = reporter.finish_turn()
    reporter.record_turn(
        turn_number=turn_number,
        duration_ms=duration_ms,
    )
    reporter.log(
        "TURN_COMPLETED",
        f"Completed turn {turn_number}",
        details={
            "turn_number": turn_number,
            "duration_ms": round(duration_ms, 2),
        },
    )

    # 2. Log turn info to standard logger
    token_usage = getattr(turn, "token_usage", None)
    LOG.info(
        "TURN_COMPLETED turn=%s tokens=%s duration_ms=%s",
        getattr(turn, "turn_number", None) or turn_number,
        getattr(token_usage, "total_tokens", None),
        getattr(turn, "duration_ms", None) or round(duration_ms, 2),
    )

    # 3. Cache latest response text for fallback parser resiliency
    extracted = extract_text_from_turn(turn)
    if extracted:
        _LAST_RESPONSE_TEXT = extracted


def _model_used(config: LocalAgentConfig, agent: Any = None) -> str:
    env = (os.getenv("GEMINI_MODEL") or "").strip()
    if env:
        return env
    for src in (agent, config):
        if src is None:
            continue
        model = getattr(src, "model", None)
        if isinstance(model, str) and model.strip():
            return model.strip()
        name = getattr(model, "name", None) if model is not None else None
        if name:
            return str(name)
    try:
        from google.antigravity.models import DEFAULT_MODEL
        return str(DEFAULT_MODEL)
    except ImportError:
        return "unknown"


def _stamp_runtime_fields(analysis: HorizonAnalysis, model_used: str) -> HorizonAnalysis:
    return analysis.model_copy(
        update={"model": model_used, "tools_used": list(_RECORDED_TOOLS)}
    )


def build_agent_config(
    *,
    skills_dir: Path = DEFAULT_SKILLS_DIR,
    mcp_config: Path | None = DEFAULT_MCP_CONFIG,
    workspace: Path | None = None,
) -> LocalAgentConfig:
    skill_paths = discover_skill_paths(skills_dir)
    mcp_servers = load_mcp_servers(mcp_config)
    workspace = (workspace or Path.cwd()).resolve()
    if not workspace.is_dir():
        raise NotADirectoryError(f"Workspace does not exist: {workspace}")

    kwargs: dict[str, Any] = {
        "skills_paths": skill_paths,
        "mcp_servers": mcp_servers,
        "workspaces": [str(workspace)],
        "system_instructions": COMMON_SYSTEM_INSTRUCTIONS,
        "response_schema": HorizonAnalysis,
        "hooks": [
            _session_start,
            _session_end,
            _pre_tool_call,
            _post_tool_call,
            _tool_error,
            _pre_turn,
            _post_turn,
        ],
        # Explicitly retain read-only built-in filesystem/navigation capabilities.
        "capabilities": types.CapabilitiesConfig(
            enabled_tools=[
                types.BuiltinTools.LIST_DIR,
                types.BuiltinTools.SEARCH_DIR,
                types.BuiltinTools.FIND_FILE,
                types.BuiltinTools.VIEW_FILE,
                types.BuiltinTools.FINISH,
            ],
            enable_subagents=False,
        ),
        "policies": [
            policy.deny_all(),
            policy.allow(types.BuiltinTools.LIST_DIR.value),
            policy.allow(types.BuiltinTools.SEARCH_DIR.value),
            policy.allow(types.BuiltinTools.FIND_FILE.value),
            policy.allow(types.BuiltinTools.VIEW_FILE.value),
            policy.allow(types.BuiltinTools.FINISH.value),
        ],
    }

    # For GEAP/Vertex deployments, prefer ADC/workload identity.
    if os.getenv("GOOGLE_GENAI_USE_VERTEXAI", "").lower() in {"1", "true", "yes"} or \
       os.getenv("GOOGLE_GENAI_USE_ENTERPRISE", "").lower() in {"1", "true", "yes"}:
        kwargs["vertex"] = True
        if os.getenv("GOOGLE_CLOUD_PROJECT"):
            kwargs["project"] = os.environ["GOOGLE_CLOUD_PROJECT"]
        if os.getenv("GOOGLE_CLOUD_LOCATION"):
            kwargs["location"] = os.environ["GOOGLE_CLOUD_LOCATION"]

    gemini_model = (os.getenv("GEMINI_MODEL") or "").strip()
    if gemini_model:
        kwargs["model"] = gemini_model

    return LocalAgentConfig(**kwargs)


async def generate_agent_trace(agent) -> None:
    reporter = get_reporter()
    # Clean compile across all internal history entries
    for idx, entry in enumerate(agent.conversation.history):

        if hasattr(entry, 'tool_calls') and entry.tool_calls:
            step_type = "tool_call"
        elif hasattr(entry, 'role') and entry.role == "function":
            step_type = "tool_response"
        elif hasattr(entry, 'role') and entry.role == "model":
            step_type = "model_turn"
        else:
            step_type = "thought"

        content_text = ""
        if hasattr(entry, 'content'):
            content_text = str(entry.content)
        elif hasattr(entry, 'parts'):
            # Extracts pure text out of Multi-part Gemini payloads if present
            content_text = " ".join(p.text for p in entry.parts if getattr(p, "text", None))
        else:
            content_text = str(entry)

        reporter.record_step(
            step_type=step_type,
            content=str(content_text.strip()),
            raw_payload=entry,
        )


async def analyze_failure(
    failure_context: str,
    *,
    workflow_id: str,
    session_id: str,
    trace_id: str,
    skills_dir: Path = DEFAULT_SKILLS_DIR,
    mcp_config: Path | None = DEFAULT_MCP_CONFIG,
    workspace: Path | None = None,
) -> HorizonAnalysis:
    global _LAST_RESPONSE_TEXT
    if not failure_context.strip():
        raise ValueError("failure_context must not be empty")

    _RECORDED_TOOLS.clear()
    _LAST_RESPONSE_TEXT = ""

    config = build_agent_config(
        skills_dir=skills_dir,
        mcp_config=mcp_config,
        workspace=workspace,
    )

    model_used = _model_used(config)

    trace_dir = (Path(workspace) / "traces") if workspace else Path("traces")

    reporter = AgentCloudLoggingReporter(
        project_id=os.getenv("GOOGLE_CLOUD_PROJECT", "unknown-project"),
        workflow_id=workflow_id,
        session_id=session_id,
        trace_id=trace_id,
        model=model_used,
        trace_dir=trace_dir,
    )

    prompt = f"""
    Analyze this Horizon SDV CI failure.

    <ci_failure_context>
    {failure_context}
    </ci_failure_context>

    MANDATORY FIRST ACTION:
    Identify the relevant classification from the failure context and use view_file to inspect
    its SKILL.md (e.g. skills/<classification>/SKILL.md)
    as your very first action BEFORE performing any searches or opening raw test logs.

    Investigate autonomously using the appropriate installed skill and the allowed
    built-in workspace tools (list, search, find, view). Return the required structured
    result. Prefer targeted pattern searches over viewing large files. Avoid unnecessary
    tool calls and keep findings concise to conserve context tokens.

    CRITICAL CTS INVESTIGATION REQUIREMENTS:
    - If the classification is cts_execution and test failures exist (FAILED > 0 in cts_module_summary.txt or invocation_summary.txt),
      you MUST investigate each of the top 2-3 failing modules and report specific Test#Method functional findings (Track A).
    - Do NOT stop prematurely after discovering a setup error or incomplete module. A setup error on attempt 0
      does not excuse omitting root-cause analysis for genuine test failures in the same or other modules.
    - Returning findings that only report incomplete modules when functional test failures exist is strictly forbidden.
    """

    set_active_reporter(reporter)

    try:
        reporter.log(
            "AGENT_INITIALIZED",
            "Antigravity agent initialized",
            details={
                "workflow_id": workflow_id,
                "session_id": session_id,
                "model": model_used,
            },
        )

        async with Agent(config) as agent:
            response = await agent.chat(prompt)

            usage = getattr(agent.conversation, "total_usage", None)
            reporter.set_total_usage(usage)
            LOG.info(
                "Token usage prompt_token_count=%s",
                getattr(usage, "prompt_token_count", None),
            )
            reporter.log(
                "TOTAL_USAGE",
                "Total tokens usage information",
                details={
                    "prompt_tokens": getattr(usage, "prompt_token_count", None),
                    "cached_tokens": getattr(usage, "cached_content_token_count", None),
                    "output_tokens": getattr(usage, "candidates_token_count", None),
                    "thought_tokens": getattr(usage, "thoughts_token_count", None),
                    "total_tokens": getattr(usage, "total_token_count", None),
                },
            )
            total_turns = getattr(agent.conversation, "turn_count", None) or 0
            total_steps = len(getattr(agent.conversation, "history", None) or [])
            reporter.set_total_turns_and_steps(total_turns, total_steps)

            await generate_agent_trace(agent)

            # Record the actual model selected by the runtime.
            runtime_model = _model_used(config, agent)
            reporter.set_model(runtime_model or reporter.model)

            analysis: HorizonAnalysis | None = None
            data = None
            try:
                data = await response.structured_output()
            except Exception as exc:
                LOG.warning("Failed to retrieve structured output from agent response: %r", exc)

            if data:
                try:
                    if isinstance(data, HorizonAnalysis):
                        analysis = data
                    else:
                        analysis = HorizonAnalysis.model_validate(data)
                except Exception as exc:
                    LOG.warning("Structured output failed validation: %r. Attempting fallback parse.", exc)

            # Fallback: Agent emitted markdown / text (e.g. from forced invocation cutoff or direct answer)
            if analysis is None:
                response_text = await extract_response_text(response, fallback_text=_LAST_RESPONSE_TEXT)
                LOG.warning(
                    "Agent structured output unavailable. Falling back to response text parser (%d chars).",
                    len(response_text),
                )
                analysis = parse_incomplete_response(response_text, failure_context)

            analysis = _stamp_runtime_fields(analysis, reporter.model)

            reporter.set_status("COMPLETED")

            reporter.log(
                "ANALYSIS_COMPLETED",
                "AI Review completed",
                details={
                    "status": analysis.status,
                    "classification": analysis.classification,
                    "confidence": analysis.confidence,
                },
            )

            return analysis

    except Exception as exc:
        reporter.set_status("FAILED")

        reporter.log(
            "ANALYSIS_FAILED",
            "AI Review failed",
            details={
                "error_type": type(exc).__name__,
                "error": str(exc),
            },
            severity="ERROR",
        )
        raise

    finally:
        try:
            reporter.save_to_json_file()
        except Exception:
            LOG.exception("Failed to write Horizon AI Review trace")
        clear_active_reporter()
