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
"""ISO 26262-6 Clause 7.4.6 / Annex D & Rust `extern "C"` FFI Boundary Auditor.

Audits Rust (`.rs`) and C/C++ (`.h`, `.hpp`, `.c`, `.cpp`) source files for:
- FFI-ABI-001: `extern "C" fn` missing `catch_unwind` / `ffi_boundary!` panic barrier
  or invoking panicking operations (`assert!`, `unwrap()`, `expect()`, `panic!`).
- FFI-ABI-002: Raw pointer parameters (`*const T`, `*mut T`) dereferenced or passed
  to `slice::from_raw_parts` / `CStr::from_ptr` without null, alignment, or
  zero-length (`len == 0`) validation.
- FFI-ABI-003: Non-FFI-safe types (`String`, `Vec`, `Result`, `std::shared_ptr`) or
  structs/enums lacking `#[repr(C)]` / `#[repr(transparent)]` / `#[repr(u*/i*)]`
  used across `extern "C"` boundaries.
- FFI-ABI-004: Reference (`&packed.field` / `&mut packed.field`) taken to a field
  of a `#[repr(C, packed)]` or `#[repr(packed)]` struct (unaligned reference UB).
- FFI-ABI-005: `unsafe extern "C" fn` or `extern "C" fn` containing `unsafe` blocks
  missing `# Safety` rustdoc or `// SAFETY:` invariant comments.
- FFI-ISO-001 (ISO 26262-6 Annex D.2.1 Temporal Isolation): Process-killing calls or
  macros (`PROCESS_TERMINATE`, `std::abort()`, `std::exit()`, `LOG_FATAL_RESET_TRAP`,
  `process::abort()`, `process::exit()`) reachable from runtime/SD/IPC/FFI paths.
- FFI-ISO-002 (ISO 26262-6 Annex D.2.2/D.2.3 Spatial & Communication Isolation):
  Unbounded memory/queue configurations (`QUEUE_SIZE_UNLIMITED`,
  `MAX_LOCAL_MESSAGE_SIZE = 0`, `MAX_TCP_MESSAGE_SIZE = 0`,
  `mpsc::unbounded_channel`, nested `std::map` fragment reassembly tables,
  or eager `buffer.resize(header_size + offset)` allocation).
"""

from __future__ import annotations

import argparse
from dataclasses import asdict, dataclass
import json
from pathlib import Path
import re
import sys
import tempfile
from typing import Dict, List, Optional, Sequence, Set, Tuple

RUST_EXTENSIONS = {".rs"}
CPP_EXTENSIONS = {".h", ".hpp", ".c", ".cpp", ".cc", ".cxx", ".in"}
SKIP_DIRS = {
    ".git",
    "build",
    "target",
    "CMakeFiles",
    "node_modules",
    "__pycache__",
    ".venv",
}
DEFAULT_SKIP_TEST_DIRS = {"test", "tests", "examples", "benchmarks"}

PRIMITIVE_FFI_TYPES = {
    "()",
    "bool",
    "u8",
    "u16",
    "u32",
    "u64",
    "u128",
    "usize",
    "i8",
    "i16",
    "i32",
    "i64",
    "i128",
    "isize",
    "f32",
    "f64",
    "c_void",
    "c_char",
    "c_schar",
    "c_uchar",
    "c_short",
    "c_ushort",
    "c_int",
    "c_uint",
    "c_long",
    "c_ulong",
    "c_longlong",
    "c_ulonglong",
    "uid_t",
    "gid_t",
    "size_t",
    "ssize_t",
}

FORBIDDEN_RUST_FFI_TYPES = ("String", "Vec<", "Result<", "Box<dyn", "Arc<", "Rc<", "&str", "&[")


@dataclass
class Finding:
    rule_id: str
    severity: str
    category: str
    iso_clause: str
    file: str
    line: int
    symbol: str
    message: str
    remediation: str
    snippet: str


@dataclass
class TypeDefInfo:
    name: str
    kind: str  # "struct" or "enum"
    line: int
    has_repr_c: bool
    has_repr_packed: bool
    fields: List[str]


COMPLIANT_RUST_FIXTURE = r'''// SPDX-License-Identifier: Apache-2.0
//! Compliant Rust `extern "C"` FFI boundary + ISO 26262-6 Annex D isolation fixture.

use core::mem::{align_of, offset_of, size_of};
use core::num::NonZeroUsize;
use core::ptr::addr_of;
use std::time::{Duration, Instant};

macro_rules! ffi_boundary {
    ($err_ret:expr, $body:block) => {{
        match ::std::panic::catch_unwind(::std::panic::AssertUnwindSafe(move || $body)) {
            Ok(val) => val,
            Err(_) => $err_ret,
        }
    }};
}

#[repr(C)]
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct security_policy_client_t {
    pub user: u32,
    pub group: u32,
    pub host: u32,
    pub port: u16,
}

const _: () = {
    assert!(size_of::<security_policy_client_t>() == 16);
    assert!(align_of::<security_policy_client_t>() == 4);
    assert!(offset_of!(security_policy_client_t, port) == 12);
};

#[repr(C)]
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum security_policy_acl_result_t {
    SECURITY_POLICY_OK = 0,
    SECURITY_POLICY_PERM_DENIED = 1,
}

#[repr(C, packed)]
#[derive(Copy, Clone)]
pub struct LoggingStandardHeader {
    pub htyp: u8,
    pub mcnt: u8,
    pub len_be: u16,
}

impl LoggingStandardHeader {
    pub fn read_len(&self) -> u16 {
        // SAFETY: `addr_of!` reads the packed 16-bit field without creating an unaligned reference.
        let raw = unsafe { addr_of!(self.len_be).read_unaligned() };
        u16::from_be(raw)
    }
}

/// Authenticates router credentials and optional security token bytes across C ABI.
///
/// # Safety
/// - `client` must be NULL or a valid, 4-byte aligned pointer to `security_policy_client_t`.
/// - `token` must be valid for reads of `token_len` bytes when `token_len > 0`.
#[no_mangle]
pub unsafe extern "C" fn security_policy_authenticate_router(
    client: *const security_policy_client_t,
    token: *const u8,
    token_len: usize,
) -> security_policy_acl_result_t {
    ffi_boundary!(security_policy_acl_result_t::SECURITY_POLICY_PERM_DENIED, {
        if client.is_null() || !client.is_aligned() {
            return security_policy_acl_result_t::SECURITY_POLICY_PERM_DENIED;
        }
        // SAFETY: `client` is verified non-null and aligned above.
        let c = unsafe { &*client };

        let token_slice: &[u8] = if token_len == 0 {
            &[]
        } else {
            if token.is_null() || !token.is_aligned() || token_len > 4096 {
                return security_policy_acl_result_t::SECURITY_POLICY_PERM_DENIED;
            }
            // SAFETY: `token` is non-null, aligned, and bounded to <= 4096 bytes.
            unsafe { core::slice::from_raw_parts(token, token_len) }
        };

        if c.port != 0 || (!token_slice.is_empty() && c.user != u32::MAX) {
            security_policy_acl_result_t::SECURITY_POLICY_OK
        } else {
            security_policy_acl_result_t::SECURITY_POLICY_PERM_DENIED
        }
    })
}

pub struct BoundedPartitionGuard {
    pub max_queue_bytes: NonZeroUsize,
    pub max_pdu_bytes: NonZeroUsize,
    pub watchdog_timeout: Duration,
    pub last_heartbeat: Instant,
}
'''

NON_COMPLIANT_RUST_FIXTURE = r'''// Non-compliant Rust FFI + Annex D fixture exercising FFI-ABI-001..005 and FFI-ISO-001..002.

use std::ffi::CStr;
use std::os::raw::c_char;

// Missing #[repr(C)] on struct passed across extern "C" (FFI-ABI-003)
pub struct UnreprClient {
    pub uid: u32,
    pub gid: u32,
}

#[repr(C, packed)]
pub struct PackedLoggingHeader {
    pub htyp: u8,
    pub mcnt: u8,
    pub len: u16,
}

pub fn read_packed_ref_bad(hdr: &PackedLoggingHeader) -> u16 {
    // FFI-ABI-004: taking reference `&hdr.len` to packed struct field is UB
    let r: &u16 = &hdr.len;
    u16::from_be(*r)
}

// Missing `# Safety` doc comment (FFI-ABI-005), missing `catch_unwind` (FFI-ABI-001),
// non-repr(C) parameter `UnreprClient` (FFI-ABI-003), unchecked pointer deref & slice::from_raw_parts (FFI-ABI-002),
// and process::abort() temporal isolation violation (FFI-ISO-001).
#[no_mangle]
pub unsafe extern "C" fn bad_ffi_authenticate(
    client: *const UnreprClient,
    payload: *const u8,
    len: usize,
    name: *const c_char,
) -> bool {
    let c = unsafe { &*client };
    let slice = unsafe { core::slice::from_raw_parts(payload, len) };
    let cname = unsafe { CStr::from_ptr(name).to_str().unwrap() };
    assert!(!cname.is_empty());
    if slice.is_empty() && c.uid == 0 {
        std::process::abort();
    }
    true
}

pub fn create_unbounded_qm_queue() {
    // FFI-ISO-002: unbounded channel allows QM producer to exhaust heap memory
    let (_tx, _rx) = tokio::sync::mpsc::unbounded_channel::<Vec<u8>>();
}
'''

NON_COMPLIANT_CPP_FIXTURE = r'''// Non-compliant C/C++ Annex D & Plugin ABI fixture.
#include <cstdlib>
#include <map>
#include <memory>
#include <vector>

constexpr size_t MAX_LOCAL_MESSAGE_SIZE = 0;
constexpr size_t MAX_TCP_MESSAGE_SIZE = 0;
constexpr size_t QUEUE_SIZE_UNLIMITED = 0;

#define PROCESS_TERMINATE(reason) \
    do { \
        std::abort(); \
    } while (0)

#define LOG_FATAL_RESET_TRAP(LOGLEVEL) \
    do { \
        int* p = NULL; \
        *p = 0; \
    } while (0)

class plugin;
typedef std::shared_ptr<plugin> (*create_plugin_func)();
#define AUTOMOTIVE_PLUGIN(class_name) \
    extern "C" { \
        create_plugin_func automotive_plugin_init() { return nullptr; } \
    }

struct reassembler_bad {
    std::map<uint32_t, std::map<uint16_t, std::map<uint64_t, std::vector<uint8_t>>>> sessions_;
    void on_segment(uint32_t wire_offset) {
        std::vector<uint8_t> message_buf;
        message_buf.resize(16 + wire_offset, 0x0);
    }
};
'''


def _strip_comments_preserve_lines(line: str) -> str:
    idx = line.find("//")
    if idx != -1:
        return line[:idx]
    return line


def _parse_rust_types(lines: Sequence[str]) -> Tuple[Dict[str, TypeDefInfo], Set[str]]:
    """Extracts struct/enum definitions, their `#[repr(...)]` attributes, and type aliases."""
    type_defs: Dict[str, TypeDefInfo] = {}
    safe_aliases: Set[str] = set()
    pending_repr_c = False
    pending_repr_packed = False

    repr_re = re.compile(r"#\s*\[\s*repr\s*\(([^)]+)\)\s*\]")
    type_decl_re = re.compile(
        r"^\s*(?:pub(?:\([^)]*\))?\s+)?(struct|enum)\s+([A-Za-z_][A-Za-z0-9_]*)"
    )
    alias_re = re.compile(
        r"^\s*(?:pub(?:\([^)]*\))?\s+)?type\s+([A-Za-z_][A-Za-z0-9_]*)\s*=\s*([^;]+);"
    )
    field_re = re.compile(r"^\s*(?:pub(?:\([^)]*\))?\s+)?([a-zA-Z_][a-zA-Z0-9_]*)\s*:")

    i = 0
    n = len(lines)
    while i < n:
        raw = lines[i]
        clean = _strip_comments_preserve_lines(raw).strip()
        if not clean:
            i += 1
            continue

        for m in repr_re.finditer(clean):
            tokens = [t.strip() for t in m.group(1).split(",")]
            for tok in tokens:
                if tok in {"C", "transparent"} or re.match(r"^[ui](?:8|16|32|64|128|size)$", tok):
                    pending_repr_c = True
                if tok == "packed" or tok.startswith("packed("):
                    pending_repr_packed = True

        m_alias = alias_re.match(clean)
        if m_alias:
            alias_name = m_alias.group(1)
            target = m_alias.group(2).strip()
            base_target = target.split("::")[-1]
            if (
                base_target in PRIMITIVE_FFI_TYPES
                or target.startswith("*const")
                or target.startswith("*mut")
                or "extern \"C\" fn" in target
            ):
                safe_aliases.add(alias_name)
            pending_repr_c = False
            pending_repr_packed = False
            i += 1
            continue

        m_type = type_decl_re.match(clean)
        if m_type:
            kind = m_type.group(1)
            name = m_type.group(2)
            start_line = i + 1
            fields: List[str] = []
            brace_depth = clean.count("{") - clean.count("}")
            j = i + 1
            while j < n and brace_depth > 0:
                f_clean = _strip_comments_preserve_lines(lines[j])
                brace_depth += f_clean.count("{") - f_clean.count("}")
                if kind == "struct" and brace_depth >= 1:
                    fm = field_re.match(f_clean)
                    if fm:
                        fields.append(fm.group(1))
                j += 1

            type_defs[name] = TypeDefInfo(
                name=name,
                kind=kind,
                line=start_line,
                has_repr_c=pending_repr_c,
                has_repr_packed=pending_repr_packed,
                fields=fields,
            )
            pending_repr_c = False
            pending_repr_packed = False
            i = j
            continue

        if not clean.startswith("#[") and not clean.startswith("//") and not clean.startswith("/*"):
            pending_repr_c = False
            pending_repr_packed = False
        i += 1

    return type_defs, safe_aliases


def _extract_block(lines: Sequence[str], start_idx: int) -> Tuple[int, int, str]:
    """Finds the opening `{` from `start_idx` and returns `(sig_end_idx, body_end_idx, full_text)`."""
    n = len(lines)
    idx = start_idx
    brace_depth = 0
    found_open = False
    sig_end_idx = start_idx

    while idx < n:
        clean = _strip_comments_preserve_lines(lines[idx])
        if not found_open:
            if ";" in clean and "{" not in clean:
                return idx, idx, ""
            if "{" in clean:
                found_open = True
                sig_end_idx = idx
                brace_depth += clean.count("{") - clean.count("}")
                if brace_depth <= 0:
                    return sig_end_idx, idx, "\n".join(lines[start_idx : idx + 1])
        else:
            brace_depth += clean.count("{") - clean.count("}")
            if brace_depth <= 0:
                return sig_end_idx, idx, "\n".join(lines[start_idx : idx + 1])
        idx += 1

    return sig_end_idx, n - 1, "\n".join(lines[start_idx:])


def _parse_fn_params_and_ret(sig_text: str) -> Tuple[str, List[Tuple[str, str]], str]:
    """Extracts `(fn_name, [(param_name, param_type), ...], return_type)` from a Rust fn signature."""
    m_name = re.search(r"\bfn\s+([A-Za-z_][A-Za-z0-9_]*)\s*\(", sig_text)
    if not m_name:
        return ("unknown_fn", [], "()")
    fn_name = m_name.group(1)
    open_paren = m_name.end() - 1
    depth = 0
    close_paren = -1
    for idx in range(open_paren, len(sig_text)):
        ch = sig_text[idx]
        if ch == "(":
            depth += 1
        elif ch == ")":
            depth -= 1
            if depth == 0:
                close_paren = idx
                break
    if close_paren == -1:
        return (fn_name, [], "()")

    params_raw = sig_text[open_paren + 1 : close_paren].strip()
    after_paren = sig_text[close_paren + 1 :]
    ret_type = "()"
    m_ret = re.search(r"->\s*([^{;]+)", after_paren)
    if m_ret:
        ret_type = m_ret.group(1).strip()

    params: List[Tuple[str, str]] = []
    if params_raw:
        parts: List[str] = []
        cur: List[str] = []
        angle = 0
        paren = 0
        for ch in params_raw:
            if ch in "<([":
                angle += 1
            elif ch in ">)]":
                angle = max(0, angle - 1)
            if ch == "," and angle == 0 and paren == 0:
                parts.append("".join(cur).strip())
                cur = []
            else:
                cur.append(ch)
        if cur:
            parts.append("".join(cur).strip())

        for p in parts:
            if not p or p in {"self", "&self", "&mut self"}:
                continue
            if ":" in p:
                pname, ptype = p.split(":", 1)
                pname = pname.strip().lstrip("mut ").strip()
                ptype = ptype.strip()
                params.append((pname, ptype))

    return (fn_name, params, ret_type)


def _check_type_ffi_safe(
    type_str: str,
    type_defs: Dict[str, TypeDefInfo],
    safe_aliases: Set[str],
) -> Optional[str]:
    """Returns an error description if `type_str` is non-FFI-safe or lacks `#[repr(C)]`."""
    t = type_str.strip()
    if not t or t == "()":
        return None

    for bad in FORBIDDEN_RUST_FFI_TYPES:
        if bad in t:
            return f"Type `{t}` uses non-FFI-safe Rust layout (`{bad.rstrip('<')}`) across `extern \"C\"`"

    # Strip pointer/reference prefixes to inspect underlying struct/enum if defined in this file
    if t.startswith("*const ") or t.startswith("*mut "):
        inner = re.sub(r"^\*(?:const|mut)\s+", "", t).strip()
        base = inner.split("::")[-1]
        if base in type_defs and not type_defs[base].has_repr_c:
            return f"Pointee type `{base}` in `{t}` is defined at line {type_defs[base].line} without `#[repr(C)]`"
        return None

    if t.startswith("&"):
        inner = re.sub(r"^&(?:'[a-zA-Z0-9_]+\s+)?(?:mut\s+)?", "", t).strip()
        base = inner.split("::")[-1]
        if base in type_defs and not type_defs[base].has_repr_c:
            return f"Referenced type `{base}` in `{t}` is defined at line {type_defs[base].line} without `#[repr(C)]`"
        return None

    base = t.split("::")[-1]
    if base in PRIMITIVE_FFI_TYPES or base in safe_aliases:
        return None

    if base in type_defs and not type_defs[base].has_repr_c:
        return (
            f"{type_defs[base].kind.capitalize()} `{base}` passed by value across `extern \"C\"` "
            f"(defined at line {type_defs[base].line}) lacks `#[repr(C)]`, `#[repr(transparent)]`, or `#[repr(u*/i*)]`"
        )
    return None


def scan_rust_file(path: Path, check_annex_d: bool = True) -> List[Finding]:
    """Scans a single Rust `.rs` file for `FFI-ABI-001`..`005` and `FFI-ISO-001`..`002`."""
    findings: List[Finding] = []
    try:
        text = path.read_text(encoding="utf-8", errors="replace")
    except OSError:
        return findings

    lines = text.splitlines()
    type_defs, safe_aliases = _parse_rust_types(lines)

    # Track variables holding packed struct references/values for FFI-ABI-004
    packed_type_names = {name: td for name, td in type_defs.items() if td.has_repr_packed}
    packed_vars: Dict[str, str] = {}
    for line in lines:
        clean = _strip_comments_preserve_lines(line)
        for ptype in packed_type_names:
            for m in re.finditer(
                rf"\b([a-zA-Z_][a-zA-Z0-9_]*)\s*:\s*(?:&(?:'[a-zA-Z0-9_]+\s+)?(?:mut\s+)?)?{ re.escape(ptype) }\b",
                clean,
            ):
                packed_vars[m.group(1)] = ptype

    # Check FFI-ABI-004 (unaligned reference `&packed_var.field` without `addr_of!`)
    if packed_vars:
        for idx, raw_line in enumerate(lines):
            clean = _strip_comments_preserve_lines(raw_line)
            if "addr_of!" in clean or "addr_of_mut!" in clean:
                continue
            for var_name, ptype in packed_vars.items():
                td = packed_type_names[ptype]
                for field in td.fields:
                    pattern = rf"&\s*(?:mut\s+)?{re.escape(var_name)}\s*\.\s*{re.escape(field)}\b"
                    if re.search(pattern, clean):
                        findings.append(
                            Finding(
                                rule_id="FFI-ABI-004",
                                severity="HIGH",
                                category="Packed Struct Unaligned Reference UB",
                                iso_clause="ISO 26262-6:2018 Table 1 (1b) / Clause 7.4.6",
                                file=str(path),
                                line=idx + 1,
                                symbol=f"{ptype}.{field}",
                                message=(
                                    f"Direct reference `&{var_name}.{field}` taken to a field of "
                                    f"`#[repr(C, packed)]` struct `{ptype}` (unaligned reference Undefined Behavior)."
                                ),
                                remediation=(
                                    f"Use `unsafe {{ core::ptr::addr_of!({var_name}.{field}).read_unaligned() }}` "
                                    "or copy the scalar field by value before byte-swapping."
                                ),
                                snippet=raw_line.strip(),
                            )
                        )

    # Scan `extern "C" fn` definitions (with `{` body, not extern block declarations ending in `;`)
    extern_fn_re = re.compile(
        r"^\s*(?:pub(?:\([^)]*\))?\s+)?(?:unsafe\s+)?extern\s+\"C(?:-unwind)?\"\s+fn\s+([A-Za-z_][A-Za-z0-9_]*)"
    )

    i = 0
    n = len(lines)
    while i < n:
        clean = _strip_comments_preserve_lines(lines[i])
        m_ext = extern_fn_re.match(clean)
        if not m_ext:
            i += 1
            continue

        sig_end_idx, body_end_idx, full_fn_text = _extract_block(lines, i)
        if not full_fn_text:
            # Foreign declaration inside `extern "C" { ... }` ending with `;`
            i = sig_end_idx + 1
            continue

        sig_text = " ".join(_strip_comments_preserve_lines(lines[k]).strip() for k in range(i, sig_end_idx + 1))
        fn_name, params, ret_type = _parse_fn_params_and_ret(sig_text)
        body_lines = lines[sig_end_idx : body_end_idx + 1]
        body_clean = "\n".join(_strip_comments_preserve_lines(bl) for bl in body_lines)

        # Look backwards above `i` for doc comments and attributes (`#[no_mangle]`, `# Safety`)
        doc_lookback_start = max(0, i - 25)
        k = i - 1
        while k >= doc_lookback_start:
            s = lines[k].strip()
            if s.startswith("///") or s.startswith("//!") or s.startswith("#[") or s.startswith("*") or s.startswith("/**") or not s:
                k -= 1
            else:
                break
        preceding_docs = "\n".join(lines[k + 1 : i])

        # Rule FFI-ABI-001: Panic barrier check (`catch_unwind` or `ffi_boundary!`)
        has_catch_unwind = ("catch_unwind" in body_clean) or ("ffi_boundary!" in body_clean)
        has_panic_calls = bool(
            re.search(r"\b(?:unwrap|expect)\s*\(|\b(?:panic|assert|assert_eq|assert_ne|todo|unimplemented)!\s*\(", body_clean)
        )
        if not has_catch_unwind:
            findings.append(
                Finding(
                    rule_id="FFI-ABI-001",
                    severity="CRITICAL",
                    category="Unwind Across FFI Boundary",
                    iso_clause="ISO 26262-6:2018 Clause 7.4.6 & Annex D.2.1",
                    file=str(path),
                    line=i + 1,
                    symbol=fn_name,
                    message=(
                        f"`extern \"C\" fn {fn_name}` lacks a `std::panic::catch_unwind` / `ffi_boundary!` "
                        "panic barrier"
                        + (" and invokes panicking operations (`assert!`/`unwrap()`)." if has_panic_calls else ".")
                    ),
                    remediation=(
                        "Wrap the `extern \"C\"` function body in `std::panic::catch_unwind(AssertUnwindSafe(|| ...))` "
                        "(or `ffi_boundary!`) and translate panics into an explicit C error code."
                    ),
                    snippet=lines[i].strip(),
                )
            )

        # Rule FFI-ABI-002: Raw pointer validation before dereference / slice construction / CStr::from_ptr
        ptr_params = [(pname, ptype) for (pname, ptype) in params if ptype.startswith("*const") or ptype.startswith("*mut")]
        for pname, ptype in ptr_params:
            checked_null = bool(
                re.search(
                    rf"\b{re.escape(pname)}\s*\.\s*(?:is_null|as_ref|as_mut)\s*\(|\b(?:ref_from_c_ptr|slice_from_c_parts|slice_from_c_parts_mut|cstr_from_ptr_bounded)\s*\(\s*{re.escape(pname)}\b",
                    body_clean,
                )
            )
            checked_align = bool(
                re.search(
                    rf"\b{re.escape(pname)}\s*\.\s*is_aligned\s*\(|\b(?:ref_from_c_ptr|slice_from_c_parts|slice_from_c_parts_mut|cstr_from_ptr_bounded)\s*\(\s*{re.escape(pname)}\b",
                    body_clean,
                )
            )
            deref_used = bool(
                re.search(
                    rf"(?:\*\s*{re.escape(pname)}\b|\bfrom_raw_parts(?:_mut)?\s*\(\s*{re.escape(pname)}\b|\bCStr::from_ptr\s*\(\s*{re.escape(pname)}\b|\bBox::from_raw\s*\(\s*{re.escape(pname)}\b)",
                    body_clean,
                )
            )
            from_raw_parts_used = bool(
                re.search(rf"\bfrom_raw_parts(?:_mut)?\s*\(\s*{re.escape(pname)}\b", body_clean)
            )
            zero_len_guarded = bool(
                re.search(r"\b\w+\s*==\s*0\b|\bslice_from_c_parts(?:_mut)?\b", body_clean)
            )

            if deref_used and not checked_null:
                findings.append(
                    Finding(
                        rule_id="FFI-ABI-002",
                        severity="CRITICAL",
                        category="Unchecked Raw Pointer Dereference",
                        iso_clause="ISO 26262-6:2018 Clause 7.4.6 & Annex D.2.2",
                        file=str(path),
                        line=i + 1,
                        symbol=f"{fn_name}({pname}: {ptype})",
                        message=(
                            f"Raw pointer parameter `{pname}: {ptype}` in `extern \"C\" fn {fn_name}` is "
                            "dereferenced or passed to `from_raw_parts`/`CStr::from_ptr` without a `.is_null()` check."
                        ),
                        remediation=(
                            f"Validate `!{pname}.is_null()` and `{pname}.is_aligned()` (or use `ref_from_c_ptr` / "
                            "`slice_from_c_parts`) before dereferencing."
                        ),
                        snippet=lines[i].strip(),
                    )
                )
            elif from_raw_parts_used and not zero_len_guarded:
                findings.append(
                    Finding(
                        rule_id="FFI-ABI-002",
                        severity="HIGH",
                        category="Zero-Length Null Slice UB Hazard",
                        iso_clause="ISO 26262-6:2018 Clause 7.4.6 & Annex D.2.2",
                        file=str(path),
                        line=i + 1,
                        symbol=f"{fn_name}({pname}: {ptype})",
                        message=(
                            f"`slice::from_raw_parts({pname}, len)` in `{fn_name}` is called without guarding `len == 0` "
                            "or alignment (`slice::from_raw_parts(NULL, 0)` is immediate UB in Rust)."
                        ),
                        remediation=(
                            "Check `if len == 0 { return &[]; }` before checking `ptr.is_null()` and `ptr.is_aligned()`, "
                            "or use `slice_from_c_parts`."
                        ),
                        snippet=lines[i].strip(),
                    )
                )
            elif deref_used and not checked_align and "c_char" not in ptype and "*const u8" not in ptype and "*mut u8" not in ptype:
                findings.append(
                    Finding(
                        rule_id="FFI-ABI-002",
                        severity="MEDIUM",
                        category="Missing Pointer Alignment Check",
                        iso_clause="ISO 26262-6:2018 Clause 7.4.6 & Annex D.2.2",
                        file=str(path),
                        line=i + 1,
                        symbol=f"{fn_name}({pname}: {ptype})",
                        message=(
                            f"Raw pointer parameter `{pname}: {ptype}` in `{fn_name}` is dereferenced without "
                            "verifying alignment (`ptr.is_aligned()`)."
                        ),
                        remediation=f"Check `{pname}.is_aligned()` or use `ref_from_c_ptr({pname})` prior to `&*{pname}`.",
                        snippet=lines[i].strip(),
                    )
                )

        # Also check for unchecked `CStr::from_ptr(some_c_call())` inside any FFI or helper method
        # (handled below globally as well)

        # Rule FFI-ABI-003: Check parameter and return types for `#[repr(C)]` and FFI-safety
        for pname, ptype in params:
            err = _check_type_ffi_safe(ptype, type_defs, safe_aliases)
            if err:
                findings.append(
                    Finding(
                        rule_id="FFI-ABI-003",
                        severity="HIGH",
                        category="Non-repr(C) or Non-FFI-Safe Parameter Type",
                        iso_clause="ISO 26262-6:2018 Clause 7.4.6 & Clause 8.4.4",
                        file=str(path),
                        line=i + 1,
                        symbol=f"{fn_name}({pname}: {ptype})",
                        message=err,
                        remediation="Add `#[repr(C)]` and compile-time `size_of`/`align_of` assertions to all FFI types.",
                        snippet=lines[i].strip(),
                    )
                )
        ret_err = _check_type_ffi_safe(ret_type, type_defs, safe_aliases)
        if ret_err:
            findings.append(
                Finding(
                    rule_id="FFI-ABI-003",
                    severity="HIGH",
                    category="Non-repr(C) or Non-FFI-Safe Return Type",
                    iso_clause="ISO 26262-6:2018 Clause 7.4.6 & Clause 8.4.4",
                    file=str(path),
                    line=i + 1,
                    symbol=f"{fn_name} -> {ret_type}",
                    message=ret_err,
                    remediation="Return a `#[repr(C)]` status enum/struct or opaque pointer across `extern \"C\"`.",
                    snippet=lines[i].strip(),
                )
            )

        # Rule FFI-ABI-005: `# Safety` doc comment and `// SAFETY:` block justification
        is_unsafe_fn = bool(re.search(r"\bunsafe\s+extern\s+\"C", sig_text))
        has_safety_doc = "# Safety" in preceding_docs or "# safety" in preceding_docs.lower()
        has_unsafe_block_in_fn = bool(re.search(r"\bunsafe\s*\{", body_clean))
        has_safety_comment_in_fn = bool(re.search(r"//\s*SAFETY\s*:", "\n".join(body_lines), re.IGNORECASE))

        if is_unsafe_fn and not has_safety_doc:
            findings.append(
                Finding(
                    rule_id="FFI-ABI-005",
                    severity="MEDIUM",
                    category="Missing `# Safety` Contract on `unsafe extern \"C\" fn`",
                    iso_clause="ISO 26262-6:2018 Clause 7.4.6 & Table 1 (1f)",
                    file=str(path),
                    line=i + 1,
                    symbol=fn_name,
                    message=f"`unsafe extern \"C\" fn {fn_name}` is missing a `/// # Safety` rustdoc section.",
                    remediation=(
                        "Document pointer validity, alignment, lifetime, and thread-safety preconditions in a "
                        "`/// # Safety` doc comment above the function."
                    ),
                    snippet=lines[i].strip(),
                )
            )
        if has_unsafe_block_in_fn and not has_safety_comment_in_fn:
            findings.append(
                Finding(
                    rule_id="FFI-ABI-005",
                    severity="MEDIUM",
                    category="Undocumented `unsafe` Block in FFI Function",
                    iso_clause="ISO 26262-6:2018 Clause 7.4.6 & Table 1 (1f)",
                    file=str(path),
                    line=i + 1,
                    symbol=fn_name,
                    message=f"`extern \"C\" fn {fn_name}` contains an `unsafe {{ ... }}` block without a `// SAFETY:` justification.",
                    remediation="Precede every `unsafe` block with a `// SAFETY:` comment explaining why preconditions hold.",
                    snippet=lines[i].strip(),
                )
            )

        i = body_end_idx + 1

    # Global line-level checks in Rust files (unchecked CStr::from_ptr(fn_call()), Annex D isolation checks)
    for idx, raw_line in enumerate(lines):
        clean = _strip_comments_preserve_lines(raw_line)
        if not clean.strip():
            continue

        # Detect direct `CStr::from_ptr(some_ffi_call(...))` where return pointer is not checked for null
        m_cstr = re.search(r"\bCStr::from_ptr\s*\(\s*([A-Za-z0-9_:]+)\s*\(", clean)
        if m_cstr:
            findings.append(
                Finding(
                    rule_id="FFI-ABI-002",
                    severity="HIGH",
                    category="Unchecked Pointer Returned from C FFI Call Passed to `CStr::from_ptr`",
                    iso_clause="ISO 26262-6:2018 Clause 7.4.6 & Annex D.2.2",
                    file=str(path),
                    line=idx + 1,
                    symbol=m_cstr.group(1),
                    message=(
                        f"Return value of `{m_cstr.group(1)}(...)` is passed directly to `CStr::from_ptr` "
                        "without checking for `NULL` or bounding string length."
                    ),
                    remediation=(
                        "Bind the returned `*const c_char` pointer, verify `!ptr.is_null()`, and parse via "
                        "`cstr_from_ptr_bounded(ptr, MAX_LEN)` without `.unwrap()`."
                    ),
                    snippet=raw_line.strip(),
                )
            )

        if check_annex_d:
            # FFI-ISO-001: process::abort() / process::exit()
            m_abort = re.search(r"\b(?:std::)?process::(abort|exit)\s*\(", clean)
            if m_abort:
                findings.append(
                    Finding(
                        rule_id="FFI-ISO-001",
                        severity="HIGH",
                        category="Process Termination in Shared Partition (Temporal Isolation Hazard)",
                        iso_clause="ISO 26262-6:2018 Clause 7.4.6 & Annex D.2.1",
                        file=str(path),
                        line=idx + 1,
                        symbol=f"process::{m_abort.group(1)}",
                        message=(
                            f"Call to `process::{m_abort.group(1)}()` terminates the entire OS process, "
                            "allowing a fault in one callback/client to kill co-hosted ASIL partitions."
                        ),
                        remediation=(
                            "Return a typed error discriminant across the boundary and isolate/evict only the "
                            "faulting client or service instance."
                        ),
                        snippet=raw_line.strip(),
                    )
                )

            # FFI-ISO-002: unbounded_channel
            if "unbounded_channel" in clean:
                findings.append(
                    Finding(
                        rule_id="FFI-ISO-002",
                        severity="HIGH",
                        category="Unbounded IPC/Actor Queue (Spatial & Communication Isolation Hazard)",
                        iso_clause="ISO 26262-6:2018 Clause 7.4.6 & Annex D.2.2 / D.2.3",
                        file=str(path),
                        line=idx + 1,
                        symbol="unbounded_channel",
                        message=(
                            "`unbounded_channel` permits a flooding QM publisher to exhaust heap memory "
                            "and starve ASIL partitions."
                        ),
                        remediation=(
                            "Replace `unbounded_channel` with a bounded `mpsc::channel(CAPACITY)` paired with "
                            "per-client byte quotas (`ClientExchangeLimiter`)."
                        ),
                        snippet=raw_line.strip(),
                    )
                )

    return findings


def scan_cpp_file(path: Path, check_annex_d: bool = True) -> List[Finding]:
    """Scans a C/C++ `.h`/`.hpp`/`.c`/`.cpp` file for `FFI-ABI-003` and `FFI-ISO-001`..`002`."""
    findings: List[Finding] = []
    try:
        text = path.read_text(encoding="utf-8", errors="replace")
    except OSError:
        return findings

    lines = text.splitlines()
    for idx, raw_line in enumerate(lines):
        clean = _strip_comments_preserve_lines(raw_line)
        if not clean.strip():
            continue

        # Rule FFI-ABI-003: C++ std::shared_ptr returned across extern "C" plugin init (`plugin.hpp:30, 68-73`)
        if re.search(r"typedef\s+std::shared_ptr<[^>]+>\s*\(\s*\*\s*create_plugin_func\s*\)", clean) or re.search(
            r"#\s*define\s+AUTOMOTIVE_PLUGIN\b", clean
        ):
            findings.append(
                Finding(
                    rule_id="FFI-ABI-003",
                    severity="CRITICAL",
                    category="C++ `std::shared_ptr` Returned Across `extern \"C\"` Plugin Boundary",
                    iso_clause="ISO 26262-6:2018 Clause 7.4.6 & Clause 8.4.4",
                    file=str(path),
                    line=idx + 1,
                    symbol="AUTOMOTIVE_PLUGIN / create_plugin_func",
                    message=(
                        "`AUTOMOTIVE_PLUGIN` / `create_plugin_func` returns `std::shared_ptr<plugin>` across an "
                        "`extern \"C\"` (`dlsym`) boundary, causing ABI Undefined Behavior across compilers or Rust FFI."
                    ),
                    remediation=(
                        "Replace `std::shared_ptr<plugin>` across `extern \"C\"` with a `#[repr(C)]` vtable struct "
                        "(`AutomotivePluginVTable`) and opaque handle pointers."
                    ),
                    snippet=raw_line.strip(),
                )
            )

        if not check_annex_d:
            continue

        # Rule FFI-ISO-001: PROCESS_TERMINATE definition or invocation, std::abort(), std::exit(), LOG_FATAL_RESET_TRAP
        if re.search(r"#\s*define\s+PROCESS_TERMINATE\b|\bPROCESS_TERMINATE\s*\(", clean):
            findings.append(
                Finding(
                    rule_id="FFI-ISO-001",
                    severity="CRITICAL",
                    category="Process-Aborting Macro in Shared Routing Daemon (Temporal FFI Violation)",
                    iso_clause="ISO 26262-6:2018 Clause 7.4.6 & Annex D.2.1",
                    file=str(path),
                    line=idx + 1,
                    symbol="PROCESS_TERMINATE",
                    message=(
                        "`PROCESS_TERMINATE` blocks the calling thread (`sleep_for(2s)`) and calls `std::abort()`, "
                        "allowing a missing peer offer (`service_discovery_impl.cpp:3411`) to kill the entire routing daemon."
                    ),
                    remediation=(
                        "Eliminate `PROCESS_TERMINATE`; transition only the affected remote service/client to a "
                        "`TimedOutDegraded` state without blocking or aborting the daemon."
                    ),
                    snippet=raw_line.strip(),
                )
            )
        elif re.search(r"#\s*define\s+LOG_FATAL_RESET_TRAP\b", clean) or re.search(
            r"\*\s*p\s*=\s*0\s*;", clean
        ):
            if "NULL" in "\n".join(lines[max(0, idx - 3) : idx + 2]) or "LOG_FATAL_RESET_TRAP" in clean:
                findings.append(
                    Finding(
                        rule_id="FFI-ISO-001",
                        severity="CRITICAL",
                        category="Intentional NULL Pointer Dereference Crash Trap (Temporal FFI Violation)",
                        iso_clause="ISO 26262-6:2018 Clause 7.4.6 & Annex D.2.1 / D.2.2",
                        file=str(path),
                        line=idx + 1,
                        symbol="LOG_FATAL_RESET_TRAP",
                        message=(
                            "`LOG_FATAL_RESET_TRAP` intentionally dereferences `NULL` (`int* p = NULL; *p = 0;`) "
                            "to crash the process on `DLT_LOG_FATAL`."
                        ),
                        remediation=(
                            "Never trigger intentional `SIGSEGV` traps in library code; propagate a typed error code "
                            "(`DLT_RETURN_ERROR`) to the caller."
                        ),
                        snippet=raw_line.strip(),
                    )
                )
        elif re.search(r"\bstd::(?:abort|exit)\s*\(", clean):
            findings.append(
                Finding(
                    rule_id="FFI-ISO-001",
                    severity="HIGH",
                    category="Unconditional Process Termination (`std::abort` / `std::exit`)",
                    iso_clause="ISO 26262-6:2018 Clause 7.4.6 & Annex D.2.1",
                    file=str(path),
                    line=idx + 1,
                    symbol="std::abort / std::exit",
                    message=(
                        "Direct call to `std::abort()` or `std::exit()` terminates all co-hosted partitions in the process."
                    ),
                    remediation="Propagate a recoverable error state or isolate the faulting endpoint.",
                    snippet=raw_line.strip(),
                )
            )

        # Rule FFI-ISO-002: Unbounded message size / queue size defaults & TP reassembler unbounded map / resize
        if re.search(r"\b(?:\w+_)?MAX_(?:LOCAL|TCP|IPC)_MESSAGE_SIZE\s*=\s*0\b", clean) or re.search(
            r"\bmax_(?:configured|local|reliable|unreliable)_message_size_\s*\{\s*0\s*\}", clean
        ):
            findings.append(
                Finding(
                    rule_id="FFI-ISO-002",
                    severity="HIGH",
                    category="Unlimited Default Message Size Cap (Spatial FFI Violation)",
                    iso_clause="ISO 26262-6:2018 Clause 7.4.6 & Annex D.2.2",
                    file=str(path),
                    line=idx + 1,
                    symbol="MAX_*_MESSAGE_SIZE = 0",
                    message=(
                        "Defaulting maximum message size to `0` (unlimited) allows a malfunctioning QM peer "
                        "to grow receive buffers until heap exhaustion."
                    ),
                    remediation="Enforce a compile-time and runtime `NonZeroUsize` maximum PDU size per endpoint.",
                    snippet=raw_line.strip(),
                )
            )

        if re.search(r"endpoint_queue_limit_(?:external|local)_\s*\{\s*QUEUE_SIZE_UNLIMITED\s*\}", clean) or re.search(
            r"\bQUEUE_SIZE_UNLIMITED\s*=\s*0\b", clean
        ):
            findings.append(
                Finding(
                    rule_id="FFI-ISO-002",
                    severity="HIGH",
                    category="Unlimited Endpoint Queue Quota Default (Communication FFI Violation)",
                    iso_clause="ISO 26262-6:2018 Clause 7.4.6 & Annex D.2.3 / D.2.4",
                    file=str(path),
                    line=idx + 1,
                    symbol="QUEUE_SIZE_UNLIMITED",
                    message=(
                        "Initializing endpoint queue limits to `QUEUE_SIZE_UNLIMITED` disables byte-quota backpressure "
                        "(`local_endpoint.cpp:177`), allowing QM floods to exhaust memory."
                    ),
                    remediation=(
                        "Require non-zero per-client byte quotas (`NonZeroUsize`) and priority-segregated ASIL vs. QM queues."
                    ),
                    snippet=raw_line.strip(),
                )
            )

        if re.search(r"std::map\s*<[^{;]*std::map\s*<[^{;]*std::map\s*<", clean):
            findings.append(
                Finding(
                    rule_id="FFI-ISO-002",
                    severity="CRITICAL",
                    category="Unbounded Nested `std::map` Fragment Reassembly Pool (Spatial FFI Violation)",
                    iso_clause="ISO 26262-6:2018 Clause 7.4.6 & Annex D.2.2",
                    file=str(path),
                    line=idx + 1,
                    symbol="nested std::map<...>",
                    message=(
                        "Unbounded 3-level nested `std::map` reassembly state allows spoofed fragment headers "
                        "from a QM node to allocate unbounded heap nodes."
                    ),
                    remediation=(
                        "Replace nested `std::map` tables with a fixed-capacity slab arena (`PartitionedTpArena`) "
                        "reserving dedicated slots for ASIL traffic."
                    ),
                    snippet=raw_line.strip(),
                )
            )

        if re.search(r"\b\w+\.resize\s*\([^;]*\+\s*(?:its_|wire_|seg_|frag_)?offset\b", clean):
            findings.append(
                Finding(
                    rule_id="FFI-ISO-002",
                    severity="CRITICAL",
                    category="Eager Vector Resize From Untrusted Wire Offset (Spatial FFI Violation)",
                    iso_clause="ISO 26262-6:2018 Clause 7.4.6 & Annex D.2.2",
                    file=str(path),
                    line=idx + 1,
                    symbol="buffer.resize(header + offset)",
                    message=(
                        "Eager `resize(header_size + offset, 0)` immediately allocates and "
                        "zero-fills heap memory up to an untrusted wire fragment offset on the first received frame."
                    ),
                    remediation=(
                        "Use a pre-allocated fixed-size slot buffer (`[u8; MAX_PDU]`) with a 64-bit segment coverage bitmap."
                    ),
                    snippet=raw_line.strip(),
                )
            )

    return findings


def scan_path(
    target: Path,
    check_annex_d: bool = True,
    include_tests: bool = False,
) -> Tuple[int, int, List[Finding]]:
    """Recursively scans `target` and returns `(rust_files_scanned, cpp_files_scanned, findings)`."""
    rust_count = 0
    cpp_count = 0
    findings: List[Finding] = []

    if target.is_file():
        files = [target]
    else:
        files = []
        for p in sorted(target.rglob("*")):
            if not p.is_file():
                continue
            rel_parts = set(p.relative_to(target).parts[:-1])
            if rel_parts & SKIP_DIRS:
                continue
            if not include_tests and (rel_parts & DEFAULT_SKIP_TEST_DIRS):
                continue
            files.append(p)

    for f in files:
        suffix = f.suffix.lower()
        if suffix in RUST_EXTENSIONS:
            rust_count += 1
            findings.extend(scan_rust_file(f, check_annex_d=check_annex_d))
        elif suffix in CPP_EXTENSIONS or f.name.endswith(".h.in"):
            cpp_count += 1
            findings.extend(scan_cpp_file(f, check_annex_d=check_annex_d))

    return rust_count, cpp_count, findings


def format_report(
    target: Path,
    rust_count: int,
    cpp_count: int,
    findings: Sequence[Finding],
    fmt: str,
) -> str:
    """Formats the audit report as Markdown, JSON, or plain text."""
    by_rule: Dict[str, int] = {}
    by_sev: Dict[str, int] = {}
    for fd in findings:
        by_rule[fd.rule_id] = by_rule.get(fd.rule_id, 0) + 1
        by_sev[fd.severity] = by_sev.get(fd.severity, 0) + 1

    if fmt == "json":
        payload = {
            "target": str(target),
            "rust_files_scanned": rust_count,
            "cpp_files_scanned": cpp_count,
            "total_findings": len(findings),
            "counts_by_rule": dict(sorted(by_rule.items())),
            "counts_by_severity": dict(sorted(by_sev.items())),
            "findings": [asdict(f) for f in findings],
        }
        return json.dumps(payload, indent=2)

    if fmt == "text":
        out = [
            f"ISO 26262 FFI & Annex D Audit Report: {target}",
            f"Files scanned: {rust_count} Rust, {cpp_count} C/C++ | Total findings: {len(findings)}",
            f"By rule: {dict(sorted(by_rule.items()))}",
            "-" * 80,
        ]
        for fd in findings:
            out.append(
                f"[{fd.rule_id}] ({fd.severity}) {fd.file}:{fd.line} [{fd.symbol}] - {fd.message}\n"
                f"  Clause: {fd.iso_clause}\n"
                f"  Fix:    {fd.remediation}"
            )
        return "\n".join(out)

    # Default: markdown
    lines = [
        "# ISO 26262-6 Annex D & `extern \"C\"` FFI Boundary Audit Report",
        "",
        f"- **Target Path**: `{target}`",
        f"- **Files Scanned**: `{rust_count}` Rust (`.rs`), `{cpp_count}` C/C++ (`.h`/`.hpp`/`.c`/`.cpp`)",
        f"- **Total Findings**: `{len(findings)}`",
        "",
        "## Summary by Rule ID",
        "",
        "| Rule ID | Count | ISO 26262-6 Reference | Description |",
        "|---|---:|---|---|",
    ]
    rule_descriptions = {
        "FFI-ABI-001": ("Clause 7.4.6 / Annex D.2.1", "Missing `catch_unwind` / `ffi_boundary!` panic barrier on `extern \"C\" fn`"),
        "FFI-ABI-002": ("Clause 7.4.6 / Annex D.2.2", "Unchecked raw pointer dereference, alignment, or `(NULL, 0)` slice hazard"),
        "FFI-ABI-003": ("Clause 7.4.6 / Clause 8.4.4", "Missing `#[repr(C)]` or non-FFI-safe type (`std::shared_ptr`, `String`, `Vec`) across FFI"),
        "FFI-ABI-004": ("Table 1 (1b) / Clause 7.4.6", "Unaligned reference `&packed.field` taken to `#[repr(C, packed)]` struct field"),
        "FFI-ABI-005": ("Table 1 (1f) / Clause 7.4.6", "Missing `# Safety` rustdoc or `// SAFETY:` block invariant in FFI function"),
        "FFI-ISO-001": ("Clause 7.4.6 / Annex D.2.1", "Process-killing call/macro (`PROCESS_TERMINATE`, `std::abort`, `LOG_FATAL_RESET_TRAP`, `process::abort`)"),
        "FFI-ISO-002": ("Clause 7.4.6 / Annex D.2.2–D.2.4", "Unbounded memory/queue quota (`QUEUE_SIZE_UNLIMITED`, `MAX_MESSAGE_SIZE=0`, nested `std::map`, `unbounded_channel`)"),
    }
    for rid, (clause, desc) in sorted(rule_descriptions.items()):
        cnt = by_rule.get(rid, 0)
        lines.append(f"| `{rid}` | {cnt} | {clause} | {desc} |")

    if findings:
        lines.extend(
            [
                "",
                "## Detailed Findings",
                "",
                "| Rule | Severity | Location | Symbol | Finding & Remediation |",
                "|---|---|---|---|---|",
            ]
        )
        for fd in findings[:150]:
            loc = f"`{Path(fd.file).name}:{fd.line}`"
            msg = f"{fd.message} **Fix:** {fd.remediation}"
            lines.append(f"| `{fd.rule_id}` | `{fd.severity}` | {loc} | `{fd.symbol}` | {msg} |")
    else:
        lines.extend(["", "**Zero FFI or Annex D isolation violations detected.**"])

    return "\n".join(lines)


def dump_fixtures(dest_dir: Path) -> Tuple[Path, Path, Path]:
    """Writes compliant and non-compliant Rust & C/C++ fixtures to `dest_dir`."""
    dest_dir.mkdir(parents=True, exist_ok=True)
    compliant_rs = dest_dir / "compliant_ffi_isolation.rs"
    non_compliant_rs = dest_dir / "non_compliant_ffi_isolation.rs"
    non_compliant_cpp = dest_dir / "non_compliant_annex_d.hpp"

    compliant_rs.write_text(COMPLIANT_RUST_FIXTURE, encoding="utf-8")
    non_compliant_rs.write_text(NON_COMPLIANT_RUST_FIXTURE, encoding="utf-8")
    non_compliant_cpp.write_text(NON_COMPLIANT_CPP_FIXTURE, encoding="utf-8")
    return compliant_rs, non_compliant_rs, non_compliant_cpp


def run_self_test() -> int:
    """Executes deterministic self-tests verifying all 7 rule IDs (`FFI-ABI-001`..`005`, `FFI-ISO-001`..`002`)."""
    with tempfile.TemporaryDirectory(prefix="iso26262_ffi_selftest_") as tmp:
        tmp_path = Path(tmp)
        compliant_rs, non_compliant_rs, non_compliant_cpp = dump_fixtures(tmp_path)

        # 1. Verify compliant fixture produces 0 findings
        _, _, comp_findings = scan_path(compliant_rs, check_annex_d=True, include_tests=True)
        if comp_findings:
            print(f"[FAIL] Compliant fixture produced unexpected findings: {comp_findings}", file=sys.stderr)
            return 1

        # 2. Verify non-compliant Rust + C++ fixtures trigger every required rule ID
        _, _, bad_rs_findings = scan_path(non_compliant_rs, check_annex_d=True, include_tests=True)
        _, _, bad_cpp_findings = scan_path(non_compliant_cpp, check_annex_d=True, include_tests=True)
        all_bad = bad_rs_findings + bad_cpp_findings
        detected_rules = {f.rule_id for f in all_bad}

        expected_rules = {
            "FFI-ABI-001",
            "FFI-ABI-002",
            "FFI-ABI-003",
            "FFI-ABI-004",
            "FFI-ABI-005",
            "FFI-ISO-001",
            "FFI-ISO-002",
        }
        missing = expected_rules - detected_rules
        if missing:
            print(f"[FAIL] Self-test missed expected rules: {sorted(missing)}", file=sys.stderr)
            return 1

        print("[PASS] `check_ffi_boundaries.py --self-test` succeeded:")
        print(f"  - Compliant fixture findings: {len(comp_findings)} (expected 0)")
        print(f"  - Non-compliant fixture findings: {len(all_bad)} across rules {sorted(detected_rules)}")
        for rid in sorted(expected_rules):
            cnt = sum(1 for f in all_bad if f.rule_id == rid)
            print(f"    * {rid}: {cnt} finding(s) verified")
        return 0


def main(argv: Optional[Sequence[str]] = None) -> int:
    parser = argparse.ArgumentParser(
        prog="check_ffi_boundaries.py",
        description=(
            "Audits Rust and C/C++ automotive code for ISO 26262-6 Clause 7.4.6 / Annex D "
            "Freedom from Interference (FFI) hazards and Rust `extern \"C\"` ABI boundary safety."
        ),
    )
    parser.add_argument(
        "positional_path",
        nargs="?",
        default=None,
        help="Optional positional path to a directory or file to scan.",
    )
    parser.add_argument(
        "--path",
        "--scan-dir",
        "-p",
        dest="flag_path",
        default=None,
        help="Path to a directory or file to scan (default: current directory).",
    )
    parser.add_argument(
        "--check-annex-d",
        action="store_true",
        default=True,
        help="Enable ISO 26262-6 Annex D spatial/temporal/communication isolation checks (enabled by default).",
    )
    parser.add_argument(
        "--no-annex-d",
        action="store_false",
        dest="check_annex_d",
        help="Disable ISO 26262-6 Annex D checks and run only `extern \"C\"` ABI checks.",
    )
    parser.add_argument(
        "--include-tests",
        action="store_true",
        help="Include `test/`, `tests/`, and `examples/` directories when scanning a repository.",
    )
    parser.add_argument(
        "--dump-fixtures",
        metavar="DIR",
        default=None,
        help="Materialize compliant and non-compliant Rust/C++ FFI & Annex D test fixtures into DIR.",
    )
    parser.add_argument(
        "--self-test",
        action="store_true",
        help="Run built-in self-test suite verifying detection of FFI-ABI-001..005 and FFI-ISO-001..002.",
    )
    parser.add_argument(
        "--format",
        "-f",
        choices=("markdown", "json", "text"),
        default="markdown",
        help="Output report format (default: markdown).",
    )
    parser.add_argument(
        "--output",
        "-o",
        default=None,
        help="Optional file path to write the formatted report.",
    )
    parser.add_argument(
        "--strict",
        action="store_true",
        help="Exit with code 1 if any violations are found (default exits 0 after emitting report).",
    )

    args = parser.parse_args(argv)

    if args.dump_fixtures:
        dest = Path(args.dump_fixtures).resolve()
        c_rs, nc_rs, nc_cpp = dump_fixtures(dest)
        print(f"Dumped fixtures to {dest}:")
        print(f"  - {c_rs}")
        print(f"  - {nc_rs}")
        print(f"  - {nc_cpp}")
        if not args.self_test and not args.flag_path and not args.positional_path:
            return 0

    if args.self_test:
        rc = run_self_test()
        if rc != 0 or (not args.flag_path and not args.positional_path):
            return rc

    target_str = args.flag_path or args.positional_path or "."
    target = Path(target_str).resolve()
    if not target.exists():
        print(f"Error: path does not exist: {target}", file=sys.stderr)
        return 2

    rust_count, cpp_count, findings = scan_path(
        target,
        check_annex_d=args.check_annex_d,
        include_tests=args.include_tests,
    )
    report = format_report(target, rust_count, cpp_count, findings, args.format)

    if args.output:
        out_path = Path(args.output).resolve()
        out_path.parent.mkdir(parents=True, exist_ok=True)
        out_path.write_text(report + "\n", encoding="utf-8")

    print(report)
    if args.strict and findings:
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
