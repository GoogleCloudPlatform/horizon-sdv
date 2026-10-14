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
import time
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

from pydantic import BaseModel, ConfigDict, Field

try:
    from google.cloud import logging as cloud_logging
except ImportError:
    cloud_logging = None


LOG = logging.getLogger("horizon.agy")


class TimelineEvent(BaseModel):
    model_config = ConfigDict(extra="allow")

    timestamp: str
    event: str
    details: dict[str, Any] = Field(default_factory=dict)


class TurnRecord(BaseModel):
    model_config = ConfigDict(extra="allow")

    turn_number: int = 0
    duration_ms: float | None = None


class ToolCallRecord(BaseModel):
    model_config = ConfigDict(extra="allow")

    call_id: str | None = None
    server_name: str | None = None
    tool_name: str
    status: str = "unknown"
    duration_ms: float | None = None


class StepRecord(BaseModel):
    model_config = ConfigDict(extra="allow")
    step_id: int
    type: str
    content: str = ""
    raw_payload: dict[str, Any] | None = None


class TokenMetrics(BaseModel):
    model_config = ConfigDict(extra="allow")

    prompt_tokens: int = 0
    cached_tokens: int = 0
    output_tokens: int = 0
    thought_tokens: int = 0
    total_tokens: int = 0


class AgentTraceFile(BaseModel):
    model_config = ConfigDict(extra="allow")

    trace_id: str
    workflow_id: str
    session_id: str
    model: str
    status: str
    duration_seconds: float
    total_turns: int
    total_steps: int
    turns: list[TurnRecord] = Field(default_factory=list)
    tool_calls: list[ToolCallRecord] = Field(default_factory=list)
    steps: list[StepRecord] = Field(default_factory=list)
    timeline: list[TimelineEvent] = Field(default_factory=list)
    token_usage: TokenMetrics = Field(default_factory=TokenMetrics)


def utc_timestamp() -> str:
    return datetime.now(timezone.utc).isoformat().replace("+00:00", "Z")


def json_safe(value: Any) -> Any:
    """Convert arbitrary SDK / Pydantic values into JSON-safe values."""
    if value is None or isinstance(value, (str, int, float, bool)):
        return value

    if isinstance(value, Path):
        return str(value)

    if hasattr(value, "model_dump"):
        try:
            return json_safe(value.model_dump(mode="json"))
        except Exception:
            pass

    if hasattr(value, "to_dict"):
        try:
            return json_safe(value.to_dict())
        except Exception:
            pass

    if isinstance(value, dict):
        return {
            str(key): json_safe(item)
            for key, item in value.items()
        }

    if isinstance(value, (list, tuple, set)):
        return [json_safe(item) for item in value]

    try:
        json.dumps(value)
        return value
    except (TypeError, ValueError):
        return str(value)


def _safe_filename(value: str) -> str:
    value = str(value or "unknown")
    return "".join(
        c if c.isalnum() or c in "._-" else "_"
        for c in value
    )


class AgentCloudLoggingReporter:
    def __init__(
        self,
        *,
        project_id: str | None = None,
        trace_id: str,
        workflow_id: str,
        session_id: str,
        model: str = "unknown",
        trace_dir: Path | str = "traces",
    ):
        self.project_id = project_id or os.getenv("GOOGLE_CLOUD_PROJECT", "unknown-project")
        self.trace_id = trace_id
        self.workflow_id = workflow_id
        self.session_id = session_id
        self.model = model

        self.start_time = time.monotonic()

        self.trace_dir = Path(trace_dir)
        self.trace_dir.mkdir(
            parents=True,
            exist_ok=True,
        )

        self.turn_count = 0
        self.turn_started_at: float | None = None
        self._active_tool_calls: dict[str, dict[str, Any]] = {}

        self.timeline: list[TimelineEvent] = []
        self.turns: list[TurnRecord] = []
        self.tool_calls: list[ToolCallRecord] = []
        self.steps: list[StepRecord] = []
        self.token_usage = TokenMetrics()
        self.status = "RUNNING"
        self.recorded_tools: list[str] = []
        self.total_turns = 0
        self.total_steps = 0
        self.cloud_logger = None
        if cloud_logging is not None and self.project_id and self.project_id != "unknown-project":
            try:
                client = cloud_logging.Client(
                    project=self.project_id
                )
                self.cloud_logger = client.logger(
                    "sdlc-agent-runtime"
                )
            except Exception:
                LOG.exception(
                    "Cloud Logging initialization failed; "
                    "local trace will still be written."
                )

    # ------------------------------------------------------------------
    # Cloud Logging + timeline
    # ------------------------------------------------------------------

    def log(
        self,
        event: str,
        message: str = "",
        details: dict[str, Any] | None = None,
        severity: str = "INFO",
    ) -> None:
        details = details or {}
        timestamp = utc_timestamp()

        timeline_event = TimelineEvent(
            timestamp=timestamp,
            event=event,
            details=json_safe(details),
        )
        self.timeline.append(timeline_event)

        payload = {
            "trace_id": self.trace_id,
            "workflow_id": self.workflow_id,
            "session_id": self.session_id,
            "model": self.model,
            "event": event,
            "timestamp": timestamp,
            "details": json_safe(details),
        }
        if message:
            payload["message"] = message

        if self.cloud_logger is not None:
            try:
                self.cloud_logger.log_struct(payload, severity=severity)
            except Exception:
                LOG.exception(
                    "Failed to send trace event to Cloud Logging: %s",
                    event,
                )

    # ------------------------------------------------------------------
    # Model / tools
    # ------------------------------------------------------------------

    def set_model(self, model: str | None) -> None:
        if model and str(model).strip():
            self.model = str(model).strip()

    def record_tool_name(self, tool_name: str | None) -> None:
        if tool_name and tool_name not in self.recorded_tools:
            self.recorded_tools.append(tool_name)

    def start_tool_call(
        self,
        *,
        call_id: str | None = None,
        server_name: str | None = None,
        tool_name: str,
    ) -> None:
        self.record_tool_name(tool_name)
        key = call_id or tool_name
        self._active_tool_calls[key] = {
            "start_time": time.monotonic(),
            "server_name": server_name,
            "tool_name": tool_name,
        }

    def complete_tool_call(
        self,
        *,
        call_id: str | None = None,
        server_name: str | None = None,
        tool_name: str,
        status: str = "completed",
        duration_ms: float | None = None,
    ) -> None:
        self.record_tool_name(tool_name)
        key = call_id or tool_name
        active = self._active_tool_calls.pop(key, None)
        if duration_ms is None and active:
            duration_ms = (time.monotonic() - active["start_time"]) * 1000.0
        server = server_name or (active.get("server_name") if active else None)

        self.tool_calls.append(
            ToolCallRecord(
                call_id=call_id,
                server_name=server,
                tool_name=tool_name,
                status=status,
                duration_ms=round(duration_ms, 2) if duration_ms is not None else None,
            )
        )

    # ------------------------------------------------------------------
    # Turns / tokens
    # ------------------------------------------------------------------

    def start_turn(self) -> int:
        """Start a new Antigravity turn and return its 1-based number."""
        self.turn_count += 1
        self.turn_started_at = time.monotonic()
        return self.turn_count

    def finish_turn(self) -> tuple[int, float]:
        """Finish the current turn and return (turn_number, duration_ms)."""
        turn_number = self.turn_count
        if self.turn_started_at is None:
            duration_ms = 0.0
        else:
            duration_ms = (time.monotonic() - self.turn_started_at) * 1000.0
        self.turn_started_at = None
        return turn_number, duration_ms


    def set_total_usage(self, usage: Any) -> None:
        if usage is None:
            return

        self.token_usage = TokenMetrics(
            prompt_tokens=getattr(usage, "prompt_token_count", 0) or 0,
            cached_tokens=getattr(usage, "cached_content_token_count", 0) or 0,
            output_tokens=getattr(usage, "candidates_token_count", 0) or 0,
            thought_tokens=getattr(usage, "thoughts_token_count", 0) or 0,
            total_tokens=getattr(usage, "total_token_count", 0) or 0,
        )

    def set_total_turns_and_steps(self, 
        total_turns : int,
        total_steps : int) -> None:

        self.total_turns = total_turns
        self.total_steps = total_steps

    def record_turn(
        self,
        *,
        turn_number: int,
        duration_ms: float | None = None,
    ) -> None:
        self.turns.append(
            TurnRecord(
                turn_number=turn_number,
                duration_ms=round(duration_ms, 2) if duration_ms is not None else None,
            )
        )


    # ------------------------------------------------------------------
    # Steps
    # ------------------------------------------------------------------
    def record_step(
        self,
        *,
        step_type: str,
        content: str = "",
        raw_payload: Any = None,
        step_id: int | None = None,
    ) -> None:

        if step_id is None:
            step_id = len(self.steps)

        payload = json_safe(raw_payload)
        if payload is not None and not isinstance(payload, dict):
            payload = {"value": payload}

        self.steps.append(
            StepRecord(
                step_id=step_id,
                type=step_type,
                content=content,
                raw_payload=payload,
            )
        )


    # ------------------------------------------------------------------
    # Final status
    # ------------------------------------------------------------------

    def set_status(self, status: str) -> None:
        self.status = (status or "UNKNOWN").upper()

    # ------------------------------------------------------------------
    # Save trace file
    # ------------------------------------------------------------------

    def build_trace(self) -> AgentTraceFile:
        duration_seconds = time.monotonic() - self.start_time

        return AgentTraceFile(
            trace_id=self.trace_id,
            workflow_id=self.workflow_id,
            session_id=self.session_id,
            model=self.model,
            status=self.status,
            duration_seconds=round(duration_seconds, 2),
            total_turns=self.total_turns,
            total_steps=self.total_steps,
            turns=self.turns,
            tool_calls=self.tool_calls,
            steps=self.steps,
            timeline=self.timeline,
            token_usage=self.token_usage,
        )

    def save_to_json_file(self) -> Path:
        trace = self.build_trace()

        filename = (
            "agent_trace_"
            f"{_safe_filename(self.workflow_id)}_"
            f"{_safe_filename(self.session_id)}.json"
        )

        output_path = self.trace_dir / filename
        temp_path = output_path.with_suffix(".json.tmp")

        temp_path.write_text(
            json.dumps(
                trace.model_dump(mode="json"),
                indent=2,
                ensure_ascii=False,
            )
            + "\n",
            encoding="utf-8",
        )

        os.replace(temp_path, output_path)

        LOG.info(
            "Agent trace written to %s",
            output_path,
        )

        return output_path