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
"""Static analyzer for C++ Boost.Asio primitives, enable_shared_from_this lifecycles, and lock hierarchies.

Scans C/C++ source trees to inventory:
1. Boost.Asio primitives (io_context, steady_timer, async_* operations, post/dispatch,
   UDP/TCP/UDS sockets & acceptors, executor_work_guard, strand).
2. Object lifecycle hazards (std::enable_shared_from_this classes and shared_from_this() call sites).
3. Synchronization primitives and re-entrancy hotspots (std::recursive_mutex, std::mutex,
   std::scoped_lock, std::unique_lock, std::condition_variable, std::shared_mutex, and manual
   .unlock() calls that signal lock-order-inversion workarounds).
"""

from __future__ import annotations

import argparse
from collections import defaultdict
from dataclasses import asdict, dataclass, field
import json
import os
import re
import sys
from typing import Dict, List, Optional, Tuple

CPP_EXTENSIONS = (".cpp", ".hpp", ".h", ".c", ".cc", ".cxx", ".ipp")


@dataclass(frozen=True)
class PatternSpec:
    key: str
    label: str
    category: str
    regex: re.Pattern[str]
    rust_recommendation: str


PATTERN_SPECS: Tuple[PatternSpec, ...] = (
    PatternSpec(
        key="io_context",
        label="boost::asio::io_context",
        category="asio",
        regex=re.compile(r"\b(?:boost::asio::)?io_context\b"),
        rust_recommendation="tokio::runtime::Runtime / Handle or mio::Poll",
    ),
    PatternSpec(
        key="async_ops",
        label="Async I/O & timer ops (async_*)",
        category="asio",
        regex=re.compile(
            r"\b(?:async_receive|async_receive_from|async_send|async_send_to|"
            r"async_write|async_read|async_accept|async_connect|async_wait)\b"
        ),
        rust_recommendation="Cancel-safe read_buf, recv_from, accept in tokio::select! + split writer task",
    ),
    PatternSpec(
        key="steady_timer",
        label="boost::asio::steady_timer",
        category="asio",
        regex=re.compile(r"\b(?:boost::asio::)?steady_timer\b"),
        rust_recommendation="tokio::pin!(Sleep) + .as_mut().reset(), Interval, or SdTtlWheel / DelayQueue",
    ),
    PatternSpec(
        key="post_dispatch",
        label="boost::asio::post / dispatch",
        category="asio",
        regex=re.compile(r"\bboost::asio::(?:post|dispatch)\b|\.(?:post|dispatch)\s*\("),
        rust_recommendation="Bounded tokio::sync::mpsc::Sender::send / try_send to actor task",
    ),
    PatternSpec(
        key="udp_socket",
        label="ip::udp::socket / socket_type",
        category="asio",
        regex=re.compile(r"\bip::udp::socket\b|\budp::socket\b|\bsocket_type\b"),
        rust_recommendation="socket2::Socket -> Arc<tokio::net::UdpSocket> (lock-free &self recv_from/send_to)",
    ),
    PatternSpec(
        key="uds_stream_protocol",
        label="local::stream_protocol (Unix Domain Sockets)",
        category="asio",
        regex=re.compile(r"\blocal::stream_protocol\b|\bstream_protocol::(?:socket|acceptor)\b"),
        rust_recommendation="tokio::net::UnixStream::into_split() / UnixListener",
    ),
    PatternSpec(
        key="tcp_socket",
        label="ip::tcp::socket",
        category="asio",
        regex=re.compile(r"\bip::tcp::socket\b|\btcp::socket\b"),
        rust_recommendation="tokio::net::TcpStream::into_split() (OwnedReadHalf + OwnedWriteHalf)",
    ),
    PatternSpec(
        key="tcp_acceptor",
        label="ip::tcp::acceptor",
        category="asio",
        regex=re.compile(r"\bip::tcp::acceptor\b|\btcp::acceptor\b"),
        rust_recommendation="tokio::net::TcpListener::accept() with CancellationToken",
    ),
    PatternSpec(
        key="work_guard",
        label="executor_work_guard / io_context::work",
        category="asio",
        regex=re.compile(r"\bexecutor_work_guard\b|\bio_context::work\b"),
        rust_recommendation="Actor mpsc::Receiver::recv() loop + CancellationToken",
    ),
    PatternSpec(
        key="strand",
        label="io_context::strand",
        category="asio",
        regex=re.compile(r"\b(?:boost::asio::)?(?:io_context::)?strand\b"),
        rust_recommendation="Single-owner actor task (sequential state mutation without strand wrapper)",
    ),
    PatternSpec(
        key="enable_shared_from_this",
        label="std::enable_shared_from_this<T>",
        category="lifecycle",
        regex=re.compile(r"\benable_shared_from_this\b"),
        rust_recommendation="Task-owned state + JoinSet + CancellationToken (avoid cyclic Arc callbacks)",
    ),
    PatternSpec(
        key="shared_from_this_call",
        label="shared_from_this() call sites",
        category="lifecycle",
        regex=re.compile(r"\bshared_from_this\s*\(\s*\)"),
        rust_recommendation="Pass lightweight IDs / mpsc::Sender handles instead of self-owning Arcs",
    ),
    PatternSpec(
        key="recursive_mutex",
        label="std::recursive_mutex",
        category="concurrency",
        regex=re.compile(r"\bstd::recursive_mutex\b"),
        rust_recommendation="Eliminate re-entrant locking via actor message passing or split unlocked helpers",
    ),
    PatternSpec(
        key="std_mutex",
        label="std::mutex",
        category="concurrency",
        regex=re.compile(r"\bstd::mutex\b"),
        rust_recommendation="Actor task exclusive ownership or leaf-only std::sync::Mutex (never held across .await)",
    ),
    PatternSpec(
        key="scoped_lock",
        label="std::scoped_lock",
        category="concurrency",
        regex=re.compile(r"\bstd::scoped_lock\b"),
        rust_recommendation="Sequential actor command handling over bounded mpsc::channel",
    ),
    PatternSpec(
        key="unique_lock",
        label="std::unique_lock",
        category="concurrency",
        regex=re.compile(r"\bstd::unique_lock\b"),
        rust_recommendation="Replace manual unlock-before-callback with upward mpsc event channels",
    ),
    PatternSpec(
        key="condition_variable",
        label="std::condition_variable",
        category="concurrency",
        regex=re.compile(r"\bstd::condition_variable(?:_any)?\b"),
        rust_recommendation="tokio::sync::Notify or oneshot::channel",
    ),
    PatternSpec(
        key="shared_mutex",
        label="std::shared_mutex",
        category="concurrency",
        regex=re.compile(r"\bstd::shared_mutex\b"),
        rust_recommendation="Arc<RwLock<T>> or immutable ArcSwap / Arc<PolicySnapshot>",
    ),
)

ENABLE_SHARED_DECL_RE = re.compile(
    r"\benable_shared_from_this\s*<\s*([A-Za-z_][A-Za-z0-9_:<> ]*)\s*>"
)
RECURSIVE_MUTEX_DECL_RE = re.compile(
    r"\b(?:mutable\s+)?std::recursive_mutex\s+([A-Za-z_][A-Za-z0-9_]*)\s*;"
)
MANUAL_UNLOCK_RE = re.compile(r"\b([A-Za-z_][A-Za-z0-9_]*)\.unlock\s*\(\s*\)\s*;")


@dataclass
class OccurrenceDetail:
    file: str
    line: int
    snippet: str
    symbol: str = ""


@dataclass
class PatternSummary:
    key: str
    label: str
    category: str
    total_count: int = 0
    file_count: int = 0
    by_submodule: Dict[str, int] = field(default_factory=dict)
    rust_recommendation: str = ""


@dataclass
class AnalysisReport:
    root_path: str
    total_files_scanned: int
    total_lines_scanned: int
    summaries: Dict[str, PatternSummary]
    recursive_mutex_sites: List[OccurrenceDetail]
    enable_shared_from_this_sites: List[OccurrenceDetail]
    manual_unlock_sites: List[OccurrenceDetail]
    top_hotspot_files: List[Tuple[str, int, int, int]]


def infer_submodule(rel_path: str) -> str:
    parts = rel_path.replace("\\", "/").split("/")
    if not parts:
        return "root"
    if parts[0] == "implementation" and len(parts) > 1:
        return parts[1]
    return parts[0]


def analyze_tree(root_path: str) -> AnalysisReport:
    abs_root = os.path.abspath(root_path)
    counts: Dict[str, int] = defaultdict(int)
    files_per_key: Dict[str, set[str]] = defaultdict(set)
    submod_counts: Dict[str, Dict[str, int]] = defaultdict(lambda: defaultdict(int))

    recursive_mutex_sites: List[OccurrenceDetail] = []
    enable_shared_sites: List[OccurrenceDetail] = []
    manual_unlock_sites: List[OccurrenceDetail] = []
    file_scores: List[Tuple[str, int, int, int]] = []

    total_files = 0
    total_lines = 0

    for dirpath, dirnames, filenames in os.walk(abs_root):
        dirnames[:] = sorted(d for d in dirnames if not d.startswith("."))
        for fname in sorted(filenames):
            if not fname.endswith(CPP_EXTENSIONS):
                continue
            fpath = os.path.join(dirpath, fname)
            rel_path = os.path.relpath(fpath, abs_root).replace("\\", "/")
            submodule = infer_submodule(rel_path)
            total_files += 1

            try:
                with open(fpath, "r", encoding="utf-8", errors="replace") as fh:
                    lines = fh.readlines()
            except OSError:
                continue

            total_lines += len(lines)
            file_asio = 0
            file_locks = 0
            file_lifecycle = 0

            for line_no, raw_line in enumerate(lines, start=1):
                stripped = raw_line.strip()
                for spec in PATTERN_SPECS:
                    matches = spec.regex.findall(raw_line)
                    if matches:
                        n_matches = len(matches)
                        counts[spec.key] += n_matches
                        files_per_key[spec.key].add(rel_path)
                        submod_counts[spec.key][submodule] += n_matches
                        if spec.category == "asio":
                            file_asio += n_matches
                        elif spec.category == "concurrency":
                            file_locks += n_matches
                        elif spec.category == "lifecycle":
                            file_lifecycle += n_matches

                if "std::recursive_mutex" in raw_line:
                    decl_match = RECURSIVE_MUTEX_DECL_RE.search(raw_line)
                    sym = decl_match.group(1) if decl_match else "std::recursive_mutex"
                    recursive_mutex_sites.append(
                        OccurrenceDetail(
                            file=rel_path,
                            line=line_no,
                            snippet=stripped,
                            symbol=sym,
                        )
                    )

                if "enable_shared_from_this" in raw_line:
                    es_match = ENABLE_SHARED_DECL_RE.search(raw_line)
                    sym = es_match.group(1).strip() if es_match else "enable_shared_from_this"
                    enable_shared_sites.append(
                        OccurrenceDetail(
                            file=rel_path,
                            line=line_no,
                            snippet=stripped,
                            symbol=sym,
                        )
                    )

                unlock_match = MANUAL_UNLOCK_RE.search(raw_line)
                if unlock_match:
                    manual_unlock_sites.append(
                        OccurrenceDetail(
                            file=rel_path,
                            line=line_no,
                            snippet=stripped,
                            symbol=unlock_match.group(1),
                        )
                    )

            if file_asio + file_locks + file_lifecycle > 0:
                file_scores.append((rel_path, file_asio, file_locks, file_lifecycle))

    file_scores.sort(key=lambda item: (item[1] + item[2] + item[3]), reverse=True)

    summaries: Dict[str, PatternSummary] = {}
    for spec in PATTERN_SPECS:
        sorted_submods = dict(
            sorted(submod_counts[spec.key].items(), key=lambda kv: (-kv[1], kv[0]))
        )
        summaries[spec.key] = PatternSummary(
            key=spec.key,
            label=spec.label,
            category=spec.category,
            total_count=counts[spec.key],
            file_count=len(files_per_key[spec.key]),
            by_submodule=sorted_submods,
            rust_recommendation=spec.rust_recommendation,
        )

    return AnalysisReport(
        root_path=abs_root,
        total_files_scanned=total_files,
        total_lines_scanned=total_lines,
        summaries=summaries,
        recursive_mutex_sites=recursive_mutex_sites,
        enable_shared_from_this_sites=enable_shared_sites,
        manual_unlock_sites=manual_unlock_sites,
        top_hotspot_files=file_scores[:15],
    )


def format_markdown(report: AnalysisReport) -> str:
    out: List[str] = []
    out.append("# C++ Boost.Asio & Concurrency Migration Inventory Report")
    out.append("")
    out.append(f"- **Scanned Root**: `{report.root_path}`")
    out.append(f"- **C/C++ Files Scanned**: `{report.total_files_scanned}`")
    out.append(f"- **Total Lines Scanned**: `{report.total_lines_scanned}`")
    out.append("")
    out.append("## 1. Summary Inventory (Boost.Asio, Lifecycle & Lock Primitives)")
    out.append("")
    out.append(
        "| Category | C++ Construct | Total Count | Files | Submodule Breakdown | Target Rust Pattern |"
    )
    out.append("|---|---|---:|---:|---|---|")
    for spec in PATTERN_SPECS:
        s = report.summaries[spec.key]
        submod_str = ", ".join(f"`{k}`: {v}" for k, v in s.by_submodule.items()) or "-"
        out.append(
            f"| `{s.category}` | `{s.label}` | **{s.total_count}** | {s.file_count} | "
            f"{submod_str} | {s.rust_recommendation} |"
        )

    out.append("")
    out.append(
        f"## 2. `std::recursive_mutex` Hotspots ({len(report.recursive_mutex_sites)} Occurrences)"
    )
    out.append("")
    out.append("| # | File:Line | Symbol | Verbatim C++ Snippet |")
    out.append("|---:|---|---|---|")
    for idx, site in enumerate(report.recursive_mutex_sites, start=1):
        snippet_clean = site.snippet.replace("|", "\\|")
        out.append(f"| {idx} | `{site.file}:{site.line}` | `{site.symbol}` | `{snippet_clean}` |")

    out.append("")
    out.append(
        f"## 3. `std::enable_shared_from_this` Classes & Sites ({len(report.enable_shared_from_this_sites)} Occurrences)"
    )
    out.append("")
    out.append("| # | File:Line | Class / Type | Verbatim C++ Snippet |")
    out.append("|---:|---|---|---|")
    for idx, site in enumerate(report.enable_shared_from_this_sites, start=1):
        snippet_clean = site.snippet.replace("|", "\\|")
        out.append(f"| {idx} | `{site.file}:{site.line}` | `{site.symbol}` | `{snippet_clean}` |")

    out.append("")
    out.append(
        f"## 4. Manual `.unlock()` Lock-Inversion Hazard Sites ({len(report.manual_unlock_sites)} Total, Showing Top 20)"
    )
    out.append("")
    out.append("| # | File:Line | Lock Variable | Verbatim C++ Snippet |")
    out.append("|---:|---|---|---|")
    for idx, site in enumerate(report.manual_unlock_sites[:20], start=1):
        snippet_clean = site.snippet.replace("|", "\\|")
        out.append(f"| {idx} | `{site.file}:{site.line}` | `{site.symbol}` | `{snippet_clean}` |")

    out.append("")
    out.append("## 5. Top 15 Migration Hotspot Files by Combined Asio + Lock + Lifecycle Density")
    out.append("")
    out.append("| Rank | File | Asio Primitives | Lock Primitives | Lifecycle (`shared_from_this`) | Total Score |")
    out.append("|---:|---|---:|---:|---:|---:|")
    for rank, (fpath, asio_c, lock_c, life_c) in enumerate(report.top_hotspot_files, start=1):
        total = asio_c + lock_c + life_c
        out.append(f"| {rank} | `{fpath}` | {asio_c} | {lock_c} | {life_c} | **{total}** |")

    return "\n".join(out) + "\n"


def format_text(report: AnalysisReport) -> str:
    lines: List[str] = [
        f"Boost.Asio & Concurrency Inventory for: {report.root_path}",
        f"Files scanned: {report.total_files_scanned} ({report.total_lines_scanned} lines)",
        "-" * 78,
    ]
    for spec in PATTERN_SPECS:
        s = report.summaries[spec.key]
        lines.append(f"{s.label:42s}: {s.total_count:5d} across {s.file_count:3d} files")
    lines.append("-" * 78)
    lines.append(f"std::recursive_mutex occurrences          : {len(report.recursive_mutex_sites)}")
    lines.append(f"enable_shared_from_this occurrences       : {len(report.enable_shared_from_this_sites)}")
    lines.append(f"manual .unlock() lock-inversion sites     : {len(report.manual_unlock_sites)}")
    return "\n".join(lines) + "\n"


def format_json(report: AnalysisReport) -> str:
    payload = {
        "root_path": report.root_path,
        "total_files_scanned": report.total_files_scanned,
        "total_lines_scanned": report.total_lines_scanned,
        "summaries": {k: asdict(v) for k, v in report.summaries.items()},
        "recursive_mutex_sites": [asdict(x) for x in report.recursive_mutex_sites],
        "enable_shared_from_this_sites": [asdict(x) for x in report.enable_shared_from_this_sites],
        "manual_unlock_sites_count": len(report.manual_unlock_sites),
        "manual_unlock_sites_sample": [asdict(x) for x in report.manual_unlock_sites[:25]],
        "top_hotspot_files": [
            {
                "file": f,
                "asio_count": a,
                "lock_count": l,
                "lifecycle_count": lc,
                "total_score": a + l + lc,
            }
            for (f, a, l, lc) in report.top_hotspot_files
        ],
    }
    return json.dumps(payload, indent=2) + "\n"


def run_self_test() -> int:
    import tempfile

    with tempfile.TemporaryDirectory(prefix="asio_concurrency_selftest_") as tmpdir:
        sample_cpp = os.path.join(tmpdir, "sample_endpoint.cpp")
        with open(sample_cpp, "w", encoding="utf-8") as fh:
            fh.write(
                """#include <memory>
#include <mutex>
#include <shared_mutex>
#include <condition_variable>

class SampleEndpoint : public std::enable_shared_from_this<SampleEndpoint> {
public:
    void start() {
        auto self = shared_from_this();
        std::scoped_lock<std::mutex> guard(state_mutex_);
        std::unique_lock<std::mutex> ulock(cb_mutex_);
        ulock.unlock();
        boost::asio::post(io_, [self]() {});
        timer_.async_wait([self](const auto&) {});
        udp_sock_.async_receive_from(buf_, ep_, [self](const auto&, std::size_t) {});
        tcp_sock_.async_write(buf_, [self](const auto&, std::size_t) {});
    }
private:
    boost::asio::io_context io_;
    boost::asio::steady_timer timer_;
    boost::asio::io_context::strand strand_;
    boost::asio::executor_work_guard<int> work_guard_;
    ip::udp::socket udp_sock_;
    ip::tcp::socket tcp_sock_;
    ip::tcp::acceptor tcp_acc_;
    local::stream_protocol::socket uds_sock_;
    mutable std::recursive_mutex endpoint_mutex_;
    std::mutex state_mutex_;
    std::mutex cb_mutex_;
    std::condition_variable cv_;
    std::shared_mutex rw_mutex_;
};
"""
            )
        syn_report = analyze_tree(tmpdir)
        assert syn_report.total_files_scanned == 1, f"Expected 1 file, got {syn_report.total_files_scanned}"
        expected_nonzero = (
            "io_context",
            "async_ops",
            "steady_timer",
            "post_dispatch",
            "udp_socket",
            "uds_stream_protocol",
            "tcp_socket",
            "tcp_acceptor",
            "work_guard",
            "strand",
            "enable_shared_from_this",
            "shared_from_this_call",
            "recursive_mutex",
            "std_mutex",
            "scoped_lock",
            "unique_lock",
            "condition_variable",
            "shared_mutex",
        )
        for key in expected_nonzero:
            cnt = syn_report.summaries[key].total_count
            assert cnt > 0, f"Self-test expected count > 0 for {key}, got {cnt}"
        assert len(syn_report.recursive_mutex_sites) == 1
        assert len(syn_report.enable_shared_from_this_sites) == 1
        assert len(syn_report.manual_unlock_sites) == 1

    print(
        "[SELF-TEST PASS] Verified all 18 Boost.Asio, enable_shared_from_this, and lock patterns "
        "on synthetic C++17 fixture."
    )
    return 0


def parse_args(argv: Optional[List[str]] = None) -> argparse.Namespace:
    default_root = "."
    parser = argparse.ArgumentParser(
        description=(
            "Scan C/C++ source files to inventory Boost.Asio primitives, "
            "std::enable_shared_from_this lifecycles, and std::recursive_mutex / lock hierarchies."
        )
    )
    parser.add_argument(
        "root_pos",
        nargs="?",
        default=None,
        help="Optional positional root directory of C/C++ source tree to scan.",
    )
    parser.add_argument(
        "--root",
        default=None,
        help=f"Root directory of C/C++ source tree to scan (default: {default_root}).",
    )
    parser.add_argument(
        "--format",
        choices=("markdown", "json", "text"),
        default="markdown",
        help="Output report format (default: %(default)s).",
    )
    parser.add_argument(
        "--output",
        "-o",
        default=None,
        help="Optional file path to write the report to (writes to stdout if omitted).",
    )
    parser.add_argument(
        "--self-test",
        action="store_true",
        help="Run built-in self-test against a synthetic C++17 Boost.Asio fixture.",
    )
    args = parser.parse_args(argv)
    args.root = args.root or args.root_pos or default_root
    return args


def main(argv: Optional[List[str]] = None) -> int:
    args = parse_args(argv)
    if args.self_test:
        return run_self_test()

    if not os.path.exists(args.root):
        sys.stderr.write(f"Error: root path does not exist: {args.root}\n")
        return 1

    report = analyze_tree(args.root)
    if args.format == "markdown":
        rendered = format_markdown(report)
    elif args.format == "json":
        rendered = format_json(report)
    else:
        rendered = format_text(report)

    if args.output:
        out_dir = os.path.dirname(os.path.abspath(args.output))
        if out_dir:
            os.makedirs(out_dir, exist_ok=True)
        with open(args.output, "w", encoding="utf-8") as fh:
            fh.write(rendered)
    else:
        sys.stdout.write(rendered)

    return 0


if __name__ == "__main__":
    sys.exit(main())

