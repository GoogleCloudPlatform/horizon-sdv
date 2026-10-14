#!/usr/bin/env python3
"""Converts AI review Markdown reports into Jenkins Safe-HTML fragments."""

from __future__ import annotations

from dataclasses import dataclass, field
import html
from pathlib import Path
import re
import sys

HEADING_TAGS: dict[int, str] = {
    1: "h2",
    2: "h3",
    3: "h4",
}
MULTILINE_BACKTICK_BULLET_PATTERN = re.compile(
    r"^[ \t]*[-*][ \t]+`([^`\n]+\n[^`]+)`[ \t]*$",
    re.MULTILINE,
)
INLINE_CODE_PATTERN = re.compile(r"`([^`]+)`")
BOLD_TEXT_PATTERN = re.compile(r"\*\*([^*]+)\*\*")


@dataclass
class HtmlRenderState:
    """Tracks block state while converting Markdown lines to HTML."""

    html_parts: list[str] = field(default_factory=lambda: ["<hr/>"])
    code_lines: list[str] = field(default_factory=list)
    is_in_code_block: bool = False
    is_in_bullet_list: bool = False

    def close_bullet_list(self) -> None:
        if not self.is_in_bullet_list:
            return
        self.html_parts.append("</ul>")
        self.is_in_bullet_list = False

    def toggle_code_block(self) -> None:
        self.close_bullet_list()
        if not self.is_in_code_block:
            self.is_in_code_block = True
            return
        escaped_code = html.escape("\n".join(self.code_lines), quote=False)
        self.html_parts.append(f"<pre><code>{escaped_code}</code></pre>")
        self.code_lines.clear()
        self.is_in_code_block = False

    def append_bullet_item(self, bullet_body: str) -> None:
        if not self.is_in_bullet_list:
            self.html_parts.append("<ul>")
            self.is_in_bullet_list = True
        self.html_parts.append(f"<li>{bullet_body}</li>")

    def finalize(self) -> str:
        self.close_bullet_list()
        if self.is_in_code_block and self.code_lines:
            escaped_code = html.escape("\n".join(self.code_lines), quote=False)
            self.html_parts.append(f"<pre><code>{escaped_code}</code></pre>")
            self.code_lines.clear()
            self.is_in_code_block = False
        return "\n".join(self.html_parts)


def select_markdown_report(assist_dir: Path) -> Path | None:
    """Finds the primary Markdown report inside the gemini-assist directory."""
    if not assist_dir.is_dir():
        return None

    preferred_path = assist_dir / "agy_analysis_and_proposed_fix.md"
    if preferred_path.is_file():
        return preferred_path

    markdown_files = sorted(assist_dir.glob("*.md"))
    if not markdown_files:
        return None
    return markdown_files[0]


def normalize_multiline_patches(markdown_text: str) -> str:
    """Converts multi-line backtick bullets into fenced code blocks."""
    return MULTILINE_BACKTICK_BULLET_PATTERN.sub(
        lambda match: f"```\n{match.group(1)}\n```",
        markdown_text,
    )


def format_inline_markdown(raw_line: str) -> str:
    """Escapes HTML entities and formats inline code and bold spans."""
    escaped = html.escape(raw_line, quote=False)
    with_code = INLINE_CODE_PATTERN.sub(r"<code>\1</code>", escaped)
    return BOLD_TEXT_PATTERN.sub(r"<b>\1</b>", with_code)


def format_heading_line(stripped_line: str) -> str:
    """Renders a Markdown heading line as an HTML heading or bold label."""
    hashes, _, title = stripped_line.partition(" ")
    level = len(hashes)
    formatted_title = format_inline_markdown(title.strip())
    tag = HEADING_TAGS.get(level)
    if tag is None:
        return f"<p><b>{formatted_title}</b></p>"
    return f"<{tag}>{formatted_title}</{tag}>"


def process_markdown_line(raw_line: str, state: HtmlRenderState) -> None:
    """Processes a single Markdown line into the HTML render state."""
    stripped_line = raw_line.strip()
    if stripped_line.startswith("```"):
        state.toggle_code_block()
        return

    if state.is_in_code_block:
        state.code_lines.append(raw_line)
        return

    if not stripped_line:
        state.close_bullet_list()
        return

    if stripped_line.startswith(("- ", "* ")):
        bullet_body = format_inline_markdown(stripped_line[2:].strip())
        state.append_bullet_item(bullet_body)
        return

    state.close_bullet_list()
    if stripped_line.startswith("#"):
        state.html_parts.append(format_heading_line(stripped_line))
        return

    state.html_parts.append(f"<p>{format_inline_markdown(stripped_line)}</p>")


def convert_markdown_to_safe_html(markdown_text: str) -> str:
    """Converts Markdown text into an HTML fragment compatible with Jenkins Safe HTML."""
    normalized_text = normalize_multiline_patches(markdown_text)
    state = HtmlRenderState()
    for raw_line in normalized_text.splitlines():
        process_markdown_line(raw_line, state)
    return state.finalize()


def main() -> int:
    if len(sys.argv) < 2:
        sys.stderr.write("Usage: render_summary_html.py <gemini-assist-dir>\n")
        return 1

    assist_dir = Path(sys.argv[1])
    report_path = select_markdown_report(assist_dir)
    if report_path is None:
        return 0

    markdown_text = report_path.read_text(encoding="utf-8", errors="replace")
    if not markdown_text.strip():
        return 0

    sys.stdout.write(convert_markdown_to_safe_html(markdown_text) + "\n")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
