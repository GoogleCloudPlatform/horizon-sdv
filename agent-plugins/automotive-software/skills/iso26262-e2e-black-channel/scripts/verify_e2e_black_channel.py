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
"""ISO 26262-6:2018 Annex D.2.4 Clean-Room E2E Black-Channel Verifier & Auditor.

Capabilities:
1. --verify-crc: Computes and verifies the 4 public-domain CRCs (SAE-J1850 CRC-8,
   ITU-T X.25 / CCITT-FALSE CRC-16, IEEE 802.3 CRC-32, and Castagnoli CRC-32C)
   against the canonical Rocksoft "123456789" check vectors (0x4B, 0x29B1,
   0xCBF43926, 0xE3069283), empty vectors, and 16-byte RPC frame envelopes.
2. --simulate-faults: Executes a deterministic fault-injection simulation across
   all 7 ISO 26262-6 Annex D.2.4 communication faults (Repetition, Loss,
   Insertion, Incorrect Sequence, Corruption [single-bit, burst, and post-seal
   ClientId header mutation], Asymmetric Delay/Timeout, and Masquerading).
3. --audit-path / --audit-rust: Audits a C/C++ or Rust directory/file for
   Annex D.2.4 E2E safety anti-patterns (post-seal header mutation, wall-clock
   SystemTime timeout hazards, opt-in is_valid_crc() unchecked payload delivery,
   and silent fault forwarding) and verifies coverage of all 7 Annex D.2.4 fault
   checks.
4. --dump-fixtures: Materializes a clean-room Rust reference implementation
   (black_channel.rs) and deterministic test vectors (test_vectors.json).
5. --self-test: Runs end-to-end verification of all CRC, fault-injection, and
   static audit capabilities.
"""

from __future__ import annotations

import argparse
import dataclasses
import json
import pathlib
import re
import struct
import sys
import tempfile
from typing import Any


# ============================================================================
# 1. Public-Domain CRC Engines (Pure Python 3.13 Reference Implementations)
# ============================================================================

@dataclasses.dataclass(frozen=True)
class CrcSpec:
    name: str
    standard: str
    width: int
    poly: int
    init: int
    ref_in: bool
    ref_out: bool
    xor_out: int
    expected_check_123456789: int
    expected_check_empty: int


PUBLIC_DOMAIN_CRC_SPECS: tuple[CrcSpec, ...] = (
    CrcSpec(
        name="SAE-J1850 CRC-8",
        standard="SAE J1850 (1995)",
        width=8,
        poly=0x1D,
        init=0xFF,
        ref_in=False,
        ref_out=False,
        xor_out=0xFF,
        expected_check_123456789=0x4B,
        expected_check_empty=0x00,
    ),
    CrcSpec(
        name="ITU-T X.25 / CCITT-FALSE CRC-16",
        standard="ITU-T Rec. X.25 / V.41",
        width=16,
        poly=0x1021,
        init=0xFFFF,
        ref_in=False,
        ref_out=False,
        xor_out=0x0000,
        expected_check_123456789=0x29B1,
        expected_check_empty=0xFFFF,
    ),
    CrcSpec(
        name="IEEE 802.3 CRC-32",
        standard="IEEE 802.3 / ITU-T V.42",
        width=32,
        poly=0x04C11DB7,
        init=0xFFFFFFFF,
        ref_in=True,
        ref_out=True,
        xor_out=0xFFFFFFFF,
        expected_check_123456789=0xCBF43926,
        expected_check_empty=0x00000000,
    ),
    CrcSpec(
        name="Castagnoli CRC-32C",
        standard="IETF RFC 3720 (iSCSI) / RFC 4960",
        width=32,
        poly=0x1EDC6F41,
        init=0xFFFFFFFF,
        ref_in=True,
        ref_out=True,
        xor_out=0xFFFFFFFF,
        expected_check_123456789=0xE3069283,
        expected_check_empty=0x00000000,
    ),
)


def _reflect_bits(val: int, width: int) -> int:
    result = 0
    for i in range(width):
        if val & (1 << i):
            result |= 1 << (width - 1 - i)
    return result


def build_crc_lookup_table(spec: CrcSpec) -> list[int]:
    mask = (1 << spec.width) - 1
    top_bit = 1 << (spec.width - 1)
    ref_poly = _reflect_bits(spec.poly, spec.width) if spec.ref_in else spec.poly
    table: list[int] = []
    for i in range(256):
        if spec.ref_in:
            crc = i
            for _ in range(8):
                if crc & 1:
                    crc = (crc >> 1) ^ ref_poly
                else:
                    crc >>= 1
            table.append(crc & mask)
        else:
            crc = i << (spec.width - 8)
            for _ in range(8):
                if crc & top_bit:
                    crc = ((crc << 1) ^ spec.poly) & mask
                else:
                    crc = (crc << 1) & mask
            table.append(crc)
    return table


def compute_crc(spec: CrcSpec, data: bytes, table: list[int] | None = None) -> int:
    if table is None:
        table = build_crc_lookup_table(spec)
    mask = (1 << spec.width) - 1
    crc = spec.init & mask
    if spec.ref_in:
        for b in data:
            idx = (crc ^ b) & 0xFF
            crc = ((crc >> 8) ^ table[idx]) & mask
    else:
        shift = spec.width - 8
        for b in data:
            idx = ((crc >> shift) ^ b) & 0xFF
            crc = (((crc << 8) & mask) ^ table[idx]) & mask
    if spec.ref_in != spec.ref_out:
        crc = _reflect_bits(crc, spec.width)
    return (crc ^ spec.xor_out) & mask


def verify_all_public_domain_crcs() -> dict[str, Any]:
    results = []
    canonical_input = b"123456789"
    someip_sample = (
        b"\x12\x34\x00\x42"  # DataId = 0x12340042
        b"\x00\x01\x01\x01"  # Instance = 0x0001, Proto=1, Iface=1
        b"\x03\x01\x00\x07\x01\x01\x02\x00"  # Upper SOME/IP header
        b"\x00\x01\x03\xe8"  # Seq=1, Freshness=1000
        b"\xde\xad\xbe\xef"  # Payload
    )
    all_passed = True
    for spec in PUBLIC_DOMAIN_CRC_SPECS:
        table = build_crc_lookup_table(spec)
        check_val = compute_crc(spec, canonical_input, table)
        empty_val = compute_crc(spec, b"", table)
        frame_val = compute_crc(spec, someip_sample, table)
        passed = (
            check_val == spec.expected_check_123456789
            and empty_val == spec.expected_check_empty
        )
        if not passed:
            all_passed = False
        hex_fmt = f"0x{{:0{spec.width // 4}X}}"
        results.append({
            "name": spec.name,
            "standard": spec.standard,
            "width": spec.width,
            "poly": hex_fmt.format(spec.poly),
            "init": hex_fmt.format(spec.init),
            "ref_in": spec.ref_in,
            "ref_out": spec.ref_out,
            "xor_out": hex_fmt.format(spec.xor_out),
            "computed_123456789": hex_fmt.format(check_val),
            "expected_123456789": hex_fmt.format(spec.expected_check_123456789),
            "computed_empty": hex_fmt.format(empty_val),
            "expected_empty": hex_fmt.format(spec.expected_check_empty),
            "someip_sample_crc": hex_fmt.format(frame_val),
            "passed": passed,
        })
    return {
        "all_passed": all_passed,
        "algorithms": results,
    }


# ============================================================================
# 2. ISO 26262-6 Annex D.2.4 Black-Channel Simulator (All 7 Fault Types)
# ============================================================================

CRC32C_SPEC = PUBLIC_DOMAIN_CRC_SPECS[3]
CRC32C_TABLE = build_crc_lookup_table(CRC32C_SPEC)


@dataclasses.dataclass
class BlackChannelChannelConfig:
    data_id: int = 0x1234_0042  # ServiceId=0x1234, MethodId=0x0042
    instance_id: int = 0x0001
    interface_version: int = 0x01
    max_delta: int = 2
    deadline_timeout_us: int = 50_000  # 50 ms


class CleanRoomBlackChannelEndpoint:
    """Reference clean-room Black-Channel endpoint implementing all 7 Annex D.2.4 checks."""

    def __init__(self, config: BlackChannelChannelConfig) -> None:
        self.config = config
        self.tx_seq: int = 0
        self.rx_initialized: bool = False
        self.rx_last_seq: int = 0
        self.rx_last_valid_us: int = 0

    def _compute_envelope_crc(
        self, upper_someip_hdr: bytes, seq_fresh_bytes: bytes, app_payload: bytes
    ) -> int:
        pseudo_hdr = struct.pack(
            ">IHBB",
            self.config.data_id,
            self.config.instance_id,
            0x01,
            self.config.interface_version,
        )
        return compute_crc(
            CRC32C_SPEC,
            pseudo_hdr + upper_someip_hdr + seq_fresh_bytes + app_payload,
            CRC32C_TABLE,
        )

    def seal_frame(
        self,
        payload: bytes,
        client_id: int,
        session_id: int,
        now_monotonic_us: int,
        message_type: int = 0x02,
        return_code: int = 0x00,
    ) -> bytearray:
        seq = self.tx_seq
        self.tx_seq = (self.tx_seq + 1) & 0xFFFF
        freshness = (now_monotonic_us // 1000) & 0xFFFF

        # Upper SOME/IP header (bytes 8..16): ClientId(2), SessionId(2), Proto(1), Iface(1), MsgType(1), RetCode(1)
        upper_hdr = struct.pack(
            ">HHBBBB",
            client_id,
            session_id,
            0x01,
            self.config.interface_version,
            message_type,
            return_code,
        )
        seq_fresh = struct.pack(">HH", seq, freshness)
        crc = self._compute_envelope_crc(upper_hdr, seq_fresh, payload)

        # SOME/IP Length field covers bytes 8..end (8 bytes upper hdr + 8 bytes E2E hdr + payload)
        someip_length = 8 + 8 + len(payload)
        lower_hdr = struct.pack(">II", self.config.data_id, someip_length)
        e2e_hdr = struct.pack(">I", crc) + seq_fresh
        return bytearray(lower_hdr + upper_hdr + e2e_hdr + payload)

    def verify_frame(
        self, frame: bytes | bytearray, now_monotonic_us: int
    ) -> tuple[bool, str, dict[str, Any]]:
        if len(frame) < 24:
            return (
                False,
                "EnvelopeTooShort",
                {"actual": len(frame), "min": 24},
            )

        wire_data_id, _someip_len = struct.unpack(">II", frame[0:8])
        wire_iface_ver = frame[13]

        # Annex D.2.4 Fault 7: Masquerading / Incorrect addressing
        if (
            wire_data_id != self.config.data_id
            or wire_iface_ver != self.config.interface_version
        ):
            return (
                False,
                "Masquerading",
                {
                    "expected_data_id": f"0x{self.config.data_id:08X}",
                    "wire_data_id": f"0x{wire_data_id:08X}",
                },
            )

        # Annex D.2.4 Fault 5: Corruption of information (CRC-32C check over pseudo-hdr + upper hdr + payload)
        (rx_crc,) = struct.unpack(">I", frame[16:20])
        upper_hdr = bytes(frame[8:16])
        seq_fresh = bytes(frame[20:24])
        app_payload = bytes(frame[24:])
        expected_crc = self._compute_envelope_crc(upper_hdr, seq_fresh, app_payload)
        if rx_crc != expected_crc:
            return (
                False,
                "CorruptedCrc",
                {
                    "expected": f"0x{expected_crc:08X}",
                    "computed": f"0x{rx_crc:08X}",
                },
            )

        # Annex D.2.4 Fault 6: Asymmetric / Delay of information (monotonic deadline timeout)
        if self.rx_initialized and self.config.deadline_timeout_us > 0:
            elapsed_us = max(0, now_monotonic_us - self.rx_last_valid_us)
            if elapsed_us > self.config.deadline_timeout_us:
                return (
                    False,
                    "DeadlineTimeout",
                    {
                        "elapsed_us": elapsed_us,
                        "budget_us": self.config.deadline_timeout_us,
                    },
                )

        rx_seq, freshness = struct.unpack(">HH", seq_fresh)
        if not self.rx_initialized:
            self.rx_initialized = True
            self.rx_last_seq = rx_seq
            self.rx_last_valid_us = now_monotonic_us
            return (
                True,
                "Valid",
                {"seq": rx_seq, "freshness": freshness, "lost_frames": 0},
            )

        delta = (rx_seq - self.rx_last_seq) & 0xFFFF
        if delta == 0:
            # Annex D.2.4 Fault 1: Repetition of information
            return False, "Repetition", {"seq": rx_seq, "delta": 0}
        if delta > 0x8000:
            # Annex D.2.4 Fault 4: Incorrect sequence (backward jump / out-of-order)
            return (
                False,
                "IncorrectSequence",
                {
                    "expected_next": (self.rx_last_seq + 1) & 0xFFFF,
                    "received": rx_seq,
                },
            )
        if delta > self.config.max_delta:
            # Distinguish Fault 2 (MessageLoss just above max_delta) vs Fault 3 (SpuriousInsertion far outside window)
            if delta > self.config.max_delta * 4:
                return False, "SpuriousInsertion", {"delta": delta, "rx_seq": rx_seq}
            return (
                False,
                "MessageLoss",
                {
                    "lost_frames": delta - 1,
                    "max_delta": self.config.max_delta,
                },
            )

        self.rx_last_seq = rx_seq
        self.rx_last_valid_us = now_monotonic_us
        return (
            True,
            "Valid",
            {"seq": rx_seq, "freshness": freshness, "lost_frames": delta - 1},
        )


def run_seven_fault_simulation() -> dict[str, Any]:
    cfg = BlackChannelChannelConfig()
    sender = CleanRoomBlackChannelEndpoint(cfg)
    receiver = CleanRoomBlackChannelEndpoint(cfg)

    scenarios: list[dict[str, Any]] = []

    # Baseline nominal frame (seq=0 at t=10_000 us)
    f0 = sender.seal_frame(b"\x01\x02\x03\x04", client_id=0x0301, session_id=1, now_monotonic_us=10_000)
    ok0, status0, details0 = receiver.verify_frame(f0, now_monotonic_us=10_050)
    scenarios.append({
        "id": "BASELINE",
        "annex_d24_fault": "Nominal Frame (Initial Sync)",
        "injected_condition": "Valid SOME/IP + Black-Channel frame (seq=0, t=10.05ms)",
        "expected_status": "Valid",
        "observed_status": status0,
        "detected": ok0 and status0 == "Valid",
        "details": details0,
    })

    # Fault 1: Repetition of information (re-deliver f0 with seq=0)
    ok1, status1, details1 = receiver.verify_frame(f0, now_monotonic_us=20_000)
    scenarios.append({
        "id": "F1_REPETITION",
        "annex_d24_fault": "1. Repetition of information",
        "injected_condition": "Duplicate delivery of frame seq=0 (delta == 0)",
        "expected_status": "Repetition",
        "observed_status": status1,
        "detected": (not ok1) and status1 == "Repetition",
        "details": details1,
    })

    # Fault 2: Loss of information (skip 3 frames: seal f1, f2, f3, deliver f4 -> delta=4 > max_delta=2)
    _f1 = sender.seal_frame(b"\x01\x02\x03\x04", client_id=0x0301, session_id=2, now_monotonic_us=20_000)
    _f2 = sender.seal_frame(b"\x01\x02\x03\x04", client_id=0x0301, session_id=3, now_monotonic_us=30_000)
    _f3 = sender.seal_frame(b"\x01\x02\x03\x04", client_id=0x0301, session_id=4, now_monotonic_us=40_000)
    f4 = sender.seal_frame(b"\x01\x02\x03\x04", client_id=0x0301, session_id=5, now_monotonic_us=50_000)
    ok2, status2, details2 = receiver.verify_frame(f4, now_monotonic_us=50_050)
    scenarios.append({
        "id": "F2_MESSAGE_LOSS",
        "annex_d24_fault": "2. Loss of information",
        "injected_condition": "3 consecutive frames dropped (seq jumps 0 -> 4, max_delta=2)",
        "expected_status": "MessageLoss",
        "observed_status": status2,
        "detected": (not ok2) and status2 == "MessageLoss",
        "details": details2,
    })

    # Advance receiver normally with f1 (seq=1) and f2 (seq=2)
    receiver.verify_frame(_f1, now_monotonic_us=20_050)
    receiver.verify_frame(_f2, now_monotonic_us=30_050)

    # Fault 3: Insertion of information (spurious valid-CRC frame with seq=50 far outside window)
    spurious_sender = CleanRoomBlackChannelEndpoint(cfg)
    spurious_sender.tx_seq = 50
    f_spurious = spurious_sender.seal_frame(
        b"\x01\x02\x03\x04", client_id=0x0301, session_id=99, now_monotonic_us=35_000
    )
    ok3, status3, details3 = receiver.verify_frame(f_spurious, now_monotonic_us=35_050)
    scenarios.append({
        "id": "F3_INSERTION",
        "annex_d24_fault": "3. Insertion of information",
        "injected_condition": "Spurious frame injected at seq=50 while receiver at seq=2 (delta=48)",
        "expected_status": "SpuriousInsertion",
        "observed_status": status3,
        "detected": (not ok3) and status3 == "SpuriousInsertion",
        "details": details3,
    })

    # Fault 4: Incorrect sequence of information (deliver old frame f0 with seq=0 when receiver is at seq=2)
    ok4, status4, details4 = receiver.verify_frame(f0, now_monotonic_us=36_000)
    scenarios.append({
        "id": "F4_INCORRECT_SEQUENCE",
        "annex_d24_fault": "4. Incorrect sequence of information",
        "injected_condition": "Out-of-order backward jump (rx_seq=0 delivered after seq=2)",
        "expected_status": "IncorrectSequence",
        "observed_status": status4,
        "detected": (not ok4) and status4 == "IncorrectSequence",
        "details": details4,
    })

    # Fault 5a: Corruption of information (single-bit flip in payload of _f3)
    f3_bitflip = bytearray(_f3)
    f3_bitflip[-1] ^= 0x01
    ok5a, status5a, details5a = receiver.verify_frame(f3_bitflip, now_monotonic_us=40_050)
    scenarios.append({
        "id": "F5A_PAYLOAD_BITFLIP",
        "annex_d24_fault": "5a. Corruption of information (Payload Bit-Flip)",
        "injected_condition": "1-bit flip in application payload byte (XOR 0x01)",
        "expected_status": "CorruptedCrc",
        "observed_status": status5a,
        "detected": (not ok5a) and status5a == "CorruptedCrc",
        "details": details5a,
    })

    # Fault 5b: Post-Seal ClientId Header Mutation Bug
    f3_post_seal_mutation = bytearray(_f3)
    f3_post_seal_mutation[8] = 0x00  # Overwrite ClientId byte 8 after seal_frame()
    f3_post_seal_mutation[9] = 0x00  # Overwrite ClientId byte 9 after seal_frame()
    ok5b, status5b, details5b = receiver.verify_frame(f3_post_seal_mutation, now_monotonic_us=40_050)
    scenarios.append({
        "id": "F5B_POST_PROTECT_CLIENT_ID_BUG",
        "annex_d24_fault": "5b. Corruption via Post-Seal Header Mutation",
        "injected_condition": "Zeroed header ClientId bytes [8..10] after CRC calculation",
        "expected_status": "CorruptedCrc",
        "observed_status": status5b,
        "detected": (not ok5b) and status5b == "CorruptedCrc",
        "details": details5b,
    })

    # Fault 6: Asymmetric information / Delay of information (deliver _f3 at t=100_000 us -> elapsed=69_950 us > 50_000 us)
    ok6, status6, details6 = receiver.verify_frame(_f3, now_monotonic_us=100_000)
    scenarios.append({
        "id": "F6_DEADLINE_TIMEOUT",
        "annex_d24_fault": "6. Asymmetric / Delay of information",
        "injected_condition": "Frame delayed by 69.95 ms (exceeding 50.0 ms monotonic budget)",
        "expected_status": "DeadlineTimeout",
        "observed_status": status6,
        "detected": (not ok6) and status6 == "DeadlineTimeout",
        "details": details6,
    })

    # Fault 7: Masquerading / Incorrect addressing (valid frame from different DataId=0x9999_0001 routed to receiver)
    other_sender = CleanRoomBlackChannelEndpoint(
        BlackChannelChannelConfig(data_id=0x9999_0001, instance_id=0x0001)
    )
    f_masquerade = other_sender.seal_frame(
        b"\x01\x02\x03\x04", client_id=0x0301, session_id=1, now_monotonic_us=32_000
    )
    ok7, status7, details7 = receiver.verify_frame(f_masquerade, now_monotonic_us=32_050)
    scenarios.append({
        "id": "F7_MASQUERADING",
        "annex_d24_fault": "7. Masquerading / Incorrect addressing",
        "injected_condition": "Frame sealed for DataId 0x99990001 delivered to 0x12340042 endpoint",
        "expected_status": "Masquerading",
        "observed_status": status7,
        "detected": (not ok7) and status7 == "Masquerading",
        "details": details7,
    })

    all_detected = all(s["detected"] for s in scenarios)
    return {
        "all_detected": all_detected,
        "total_scenarios": len(scenarios),
        "detected_count": sum(1 for s in scenarios if s["detected"]),
        "scenarios": scenarios,
    }


# ============================================================================
# 3. E2E Safety & Architectural Static Auditor (--audit-path / --audit-rust)
# ============================================================================

E2E_SAFETY_HAZARD_PATTERNS: tuple[tuple[str, str, re.Pattern[str]], ...] = (
    (
        "WALL_CLOCK_DEADLINE_HAZARD",
        "Wall-clock SystemTime / system_clock used in E2E module instead of monotonic Instant / steady_clock",
        re.compile(r"\b(?:SystemTime::now|UNIX_EPOCH|std::chrono::system_clock)\b"),
    ),
    (
        "UNBOUNDED_ALLOC_IN_E2E",
        "Unbounded channel or unchecked Vec::with_capacity in E2E envelope path",
        re.compile(r"\b(?:unbounded_channel|Vec::with_capacity\s*\(\s*wire_len)\b"),
    ),
)

ARCHITECTURAL_DEFECT_PATTERNS: tuple[tuple[str, str, re.Pattern[str]], ...] = (
    (
        "POST_PROTECT_HEADER_MUTATION",
        "Header ClientId bytes mutated after protect() call (invalidates sealed CRC)",
        re.compile(
            r"->protect\([^;]+\);[\s\S]{1,450}?CLIENT_POS_MIN",
            re.MULTILINE,
        ),
    ),
    (
        "SILENT_CORRUPT_DELIVERY",
        "Failed E2E check logged only at LOG_INFO without quarantining payload",
        re.compile(
            r"check_status\s*!=\s*[\w:]*E2E_OK[\s\S]{1,350}?LOG_INFO",
            re.MULTILINE,
        ),
    ),
    (
        "OPT_IN_IS_VALID_CRC",
        "Opt-in boolean is_valid_crc() getter exposes unverified payload to callers",
        re.compile(r"\bis_valid_crc\s*\(\s*\)"),
    ),
)

ANNEX_D24_RUST_MECHANISM_CHECKS: tuple[tuple[str, str, re.Pattern[str]], ...] = (
    ("F1_Repetition", "Repetition check (delta == 0)", re.compile(r"Repetition|delta\s*==\s*0")),
    ("F2_MessageLoss", "MessageLoss check (delta > max_delta)", re.compile(r"MessageLoss|max_delta")),
    ("F3_SpuriousInsertion", "SpuriousInsertion check", re.compile(r"SpuriousInsertion")),
    ("F4_IncorrectSequence", "IncorrectSequence / backward jump check", re.compile(r"IncorrectSequence|wrapping_sub")),
    ("F5_CorruptedCrc", "CorruptedCrc check", re.compile(r"CorruptedCrc|crc8_sae_j1850|crc16_ccitt_false|crc32c_castagnoli")),
    ("F6_DeadlineTimeout", "DeadlineTimeout monotonic check", re.compile(r"DeadlineTimeout|deadline_timeout_us")),
    ("F7_Masquerading", "Masquerading / DataId pseudo-header check", re.compile(r"Masquerading|data_id")),
)


def audit_codebase(target_path: pathlib.Path) -> dict[str, Any]:
    files_to_scan: list[pathlib.Path] = []
    exts = {".rs", ".cpp", ".hpp", ".c", ".h", ".md"}
    if target_path.is_file():
        files_to_scan.append(target_path)
    elif target_path.is_dir():
        for p in sorted(target_path.rglob("*")):
            if p.is_file() and p.suffix in exts and ".git" not in p.parts:
                files_to_scan.append(p)

    ip_violations: list[dict[str, Any]] = []
    arch_defects: list[dict[str, Any]] = []
    rust_files_count = 0
    combined_rust_source = []

    for fpath in files_to_scan:
        try:
            content = fpath.read_text(encoding="utf-8", errors="replace")
        except OSError:
            continue

        rel_path = (
            str(fpath.relative_to(target_path))
            if target_path.is_dir()
            else fpath.name
        )
        if fpath.suffix == ".rs":
            rust_files_count += 1
            combined_rust_source.append(content)

        lines = content.splitlines()
        for line_idx, line in enumerate(lines, start=1):
            for rule_id, desc, pat in E2E_SAFETY_HAZARD_PATTERNS:
                if pat.search(line):
                    ip_violations.append({
                        "file": rel_path,
                        "line": line_idx,
                        "rule_id": rule_id,
                        "description": desc,
                        "snippet": line.strip()[:120],
                    })

        for rule_id, desc, pat in ARCHITECTURAL_DEFECT_PATTERNS:
            for m in pat.finditer(content):
                line_no = content.count("\n", 0, m.start()) + 1
                arch_defects.append({
                    "file": rel_path,
                    "line": line_no,
                    "rule_id": rule_id,
                    "description": desc,
                })

    rust_blob = "\n".join(combined_rust_source)
    annex_d24_coverage: list[dict[str, Any]] = []
    if rust_files_count > 0:
        for fault_key, desc, pat in ANNEX_D24_RUST_MECHANISM_CHECKS:
            annex_d24_coverage.append({
                "fault": fault_key,
                "description": desc,
                "present": bool(pat.search(rust_blob)),
            })

    return {
        "target_path": str(target_path),
        "files_scanned": len(files_to_scan),
        "rust_files_scanned": rust_files_count,
        "clean_room_ip_compliant": len(ip_violations) == 0,
        "ip_violation_count": len(ip_violations),
        "ip_violations": ip_violations,
        "architectural_defect_count": len(arch_defects),
        "architectural_defects": arch_defects,
        "annex_d24_rust_coverage": annex_d24_coverage,
    }


# ============================================================================
# 4. Fixture Generator (--dump-fixtures) & Self-Test (--self-test)
# ============================================================================

CLEAN_ROOM_RUST_FIXTURE = """\
//! Clean-room ISO 26262-6:2018 Annex D.2.4 E2E Black-Channel implementation.
//! Uses public-domain SAE-J1850 CRC-8, ITU-T X.25 CRC-16, IEEE 802.3 CRC-32,
//! and Castagnoli CRC-32C.

#![cfg_attr(not(test), no_std)]
#![forbid(unsafe_code)]

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum BlackChannelFault {
    EnvelopeTooShort { actual: usize, min: usize },
    Repetition { seq: u16 },
    MessageLoss { lost_frames: u16, max_delta: u16 },
    SpuriousInsertion { delta: u16 },
    IncorrectSequence { expected_next: u16, received: u16 },
    CorruptedCrc { expected: u32, computed: u32 },
    DeadlineTimeout { elapsed_us: u64, budget_us: u64 },
    Masquerading { expected_data_id: u32, wire_data_id: u32 },
}

pub struct SealedFrame<'a> {
    wire_bytes: &'a [u8],
}

impl<'a> SealedFrame<'a> {
    pub fn as_wire_slice(&self) -> &'a [u8] {
        self.wire_bytes
    }
}

pub struct VerifiedPayload<'a> {
    pub data_id: u32,
    pub sequence_counter: u16,
    pub payload: &'a [u8],
}

pub const fn crc8_sae_j1850(data: &[u8]) -> u8 {
    let mut crc: u8 = 0xFF;
    let mut i = 0usize;
    while i < data.len() {
        let mut x = crc ^ data[i];
        let mut bit = 0;
        while bit < 8 {
            x = if (x & 0x80) != 0 { (x << 1) ^ 0x1D } else { x << 1 };
            bit += 1;
        }
        crc = x;
        i += 1;
    }
    crc ^ 0xFF
}

pub const fn crc16_ccitt_false(data: &[u8]) -> u16 {
    let mut crc: u16 = 0xFFFF;
    let mut i = 0usize;
    while i < data.len() {
        let mut x = crc ^ ((data[i] as u16) << 8);
        let mut bit = 0;
        while bit < 8 {
            x = if (x & 0x8000) != 0 { (x << 1) ^ 0x1021 } else { x << 1 };
            bit += 1;
        }
        crc = x;
        i += 1;
    }
    crc
}

pub const fn crc32c_castagnoli(data: &[u8]) -> u32 {
    let mut crc: u32 = 0xFFFF_FFFF;
    let mut i = 0usize;
    while i < data.len() {
        let mut x = (crc ^ (data[i] as u32)) & 0xFF;
        let mut bit = 0;
        while bit < 8 {
            x = if (x & 1) != 0 { (x >> 1) ^ 0x82F6_3B78 } else { x >> 1 };
            bit += 1;
        }
        crc = (crc >> 8) ^ x;
        i += 1;
    }
    crc ^ 0xFFFF_FFFF
}

pub fn check_sequence_and_deadline(
    rx_seq: u16,
    last_seq: u16,
    max_delta: u16,
    elapsed_us: u64,
    deadline_timeout_us: u64,
    wire_data_id: u32,
    expected_data_id: u32,
    rx_crc: u32,
    expected_crc: u32,
) -> Result<u16, BlackChannelFault> {
    if wire_data_id != expected_data_id {
        return Err(BlackChannelFault::Masquerading { expected_data_id, wire_data_id });
    }
    if rx_crc != expected_crc {
        return Err(BlackChannelFault::CorruptedCrc { expected: expected_crc, computed: rx_crc });
    }
    if deadline_timeout_us > 0 && elapsed_us > deadline_timeout_us {
        return Err(BlackChannelFault::DeadlineTimeout { elapsed_us, budget_us: deadline_timeout_us });
    }
    let delta = rx_seq.wrapping_sub(last_seq);
    if delta == 0 {
        return Err(BlackChannelFault::Repetition { seq: rx_seq });
    }
    if delta > 0x8000 {
        return Err(BlackChannelFault::IncorrectSequence {
            expected_next: last_seq.wrapping_add(1),
            received: rx_seq,
        });
    }
    if delta > max_delta {
        if delta > max_delta.saturating_mul(4) {
            return Err(BlackChannelFault::SpuriousInsertion { delta });
        }
        return Err(BlackChannelFault::MessageLoss {
            lost_frames: delta.saturating_sub(1),
            max_delta,
        });
    }
    Ok(delta.saturating_sub(1))
}
"""


def dump_fixtures(target_dir: pathlib.Path) -> dict[str, Any]:
    target_dir.mkdir(parents=True, exist_ok=True)
    rust_file = target_dir / "black_channel.rs"
    rust_file.write_text(CLEAN_ROOM_RUST_FIXTURE, encoding="utf-8")

    crc_report = verify_all_public_domain_crcs()
    sim_report = run_seven_fault_simulation()
    vectors_file = target_dir / "test_vectors.json"
    vectors_file.write_text(
        json.dumps({"crc_vectors": crc_report, "fault_simulation": sim_report}, indent=2),
        encoding="utf-8",
    )
    return {
        "fixture_dir": str(target_dir),
        "files_written": [str(rust_file), str(vectors_file)],
    }


def run_self_test() -> dict[str, Any]:
    crc_res = verify_all_public_domain_crcs()
    sim_res = run_seven_fault_simulation()

    with tempfile.TemporaryDirectory(prefix="e2e_selftest_") as tmpdir:
        tmp_path = pathlib.Path(tmpdir)
        clean_dir = tmp_path / "clean_rust"
        dump_fixtures(clean_dir)
        clean_audit = audit_codebase(clean_dir)

        flawed_dir = tmp_path / "flawed_cpp"
        flawed_dir.mkdir(parents=True, exist_ok=True)
        bad_cpp = flawed_dir / "bad_e2e.cpp"
        bad_cpp.write_text(
            "auto now = std::chrono::system_clock::now();\n"
            "bool is_valid_crc() { return true; }\n"
            "void send_to() {\n"
            "    protector_->protect(id, buf, inst);\n"
            "    data[CLIENT_POS_MIN] = 0;\n"
            "}\n",
            encoding="utf-8",
        )
        bad_audit = audit_codebase(flawed_dir)

    clean_ok = (
        clean_audit["clean_room_ip_compliant"]
        and clean_audit["architectural_defect_count"] == 0
        and all(item["present"] for item in clean_audit["annex_d24_rust_coverage"])
    )
    contaminated_detected = (
        not bad_audit["clean_room_ip_compliant"]
        and bad_audit["ip_violation_count"] >= 1
        and bad_audit["architectural_defect_count"] >= 2
    )

    overall_passed = (
        crc_res["all_passed"]
        and sim_res["all_detected"]
        and clean_ok
        and contaminated_detected
    )
    return {
        "self_test_passed": overall_passed,
        "crc_verification_passed": crc_res["all_passed"],
        "seven_fault_simulation_passed": sim_res["all_detected"],
        "clean_fixture_audit_passed": clean_ok,
        "contaminated_fixture_detection_passed": contaminated_detected,
    }


# ============================================================================
# 5. Report Formatting & CLI Entrypoint
# ============================================================================

def format_markdown_report(report: dict[str, Any]) -> str:
    lines: list[str] = [
        "# ISO 26262-6:2018 Annex D.2.4 E2E Black-Channel Verification Report",
        "",
    ]
    if "crc_verification" in report:
        crc = report["crc_verification"]
        status = "PASS" if crc["all_passed"] else "FAIL"
        lines.append(f"## 1. Public-Domain CRC Verification (`{status}`)")
        lines.append("")
        lines.append(
            "| Algorithm | Standard | Width | Poly | Init | RefIn/Out | XorOut | `Check(\"123456789\")` | Expected | SOME/IP Frame CRC | Status |"
        )
        lines.append("|---|---|---|---|---|---|---|---|---|---|---|")
        for alg in crc["algorithms"]:
            st = "PASS" if alg["passed"] else "FAIL"
            lines.append(
                f"| **{alg['name']}** | {alg['standard']} | {alg['width']} | `{alg['poly']}` | "
                f"`{alg['init']}` | `{alg['ref_in']}/{alg['ref_out']}` | `{alg['xor_out']}` | "
                f"`{alg['computed_123456789']}` | `{alg['expected_123456789']}` | `{alg['someip_sample_crc']}` | **{st}** |"
            )
        lines.append("")

    if "fault_simulation" in report:
        sim = report["fault_simulation"]
        status = "PASS" if sim["all_detected"] else "FAIL"
        lines.append(
            f"## 2. ISO 26262-6 Annex D.2.4 Seven-Fault Injection Simulation (`{status}` — {sim['detected_count']}/{sim['total_scenarios']})"
        )
        lines.append("")
        lines.append(
            "| Scenario ID | Annex D.2.4 Fault Type | Injected Condition | Expected | Observed | Result |"
        )
        lines.append("|---|---|---|---|---|---|")
        for sc in sim["scenarios"]:
            res = "PASS" if sc["detected"] else "FAIL"
            lines.append(
                f"| `{sc['id']}` | {sc['annex_d24_fault']} | {sc['injected_condition']} | "
                f"`{sc['expected_status']}` | `{sc['observed_status']}` | **{res}** |"
            )
        lines.append("")

    if "audit" in report:
        aud = report["audit"]
        lines.append(f"## 3. E2E Safety & Architectural Audit (`{aud['target_path']}`)")
        lines.append("")
        lines.append(f"- **Files Scanned**: {aud['files_scanned']} (Rust files: {aud['rust_files_scanned']})")
        lines.append(f"- **Safety Hazard Free**: `{aud['clean_room_ip_compliant']}`")
        lines.append(f"- **E2E Safety Hazards Found**: {aud['ip_violation_count']}")
        lines.append(f"- **Architectural E2E Defects Found**: {aud['architectural_defect_count']}")
        lines.append("")
        if aud["architectural_defects"]:
            lines.append("### Architectural E2E Defects Detected")
            lines.append("| File | Line | Rule ID | Description |")
            lines.append("|---|---|---|---|")
            for d in aud["architectural_defects"]:
                lines.append(f"| `{d['file']}` | {d['line']} | `{d['rule_id']}` | {d['description']} |")
            lines.append("")
        if aud["ip_violations"]:
            lines.append("### E2E Safety Hazards Detected (First 15 Shown)")
            lines.append("| File | Line | Rule ID | Snippet |")
            lines.append("|---|---|---|---|")
            for v in aud["ip_violations"][:15]:
                lines.append(f"| `{v['file']}` | {v['line']} | `{v['rule_id']}` | `{v['snippet']}` |")
            lines.append("")
        if aud["annex_d24_rust_coverage"]:
            lines.append("### Annex D.2.4 Seven-Fault Mechanism Coverage in Rust")
            lines.append("| Fault ID | Mechanism Description | Present |")
            lines.append("|---|---|---|")
            for cov in aud["annex_d24_rust_coverage"]:
                lines.append(f"| `{cov['fault']}` | {cov['description']} | `{cov['present']}` |")
            lines.append("")

    if "self_test" in report:
        st = report["self_test"]
        lines.append(f"## 4. Self-Test Summary (`{'PASS' if st['self_test_passed'] else 'FAIL'}`)")
        lines.append(f"- **CRC Vector Verification**: `{st['crc_verification_passed']}`")
        lines.append(f"- **7-Fault Injection Simulation**: `{st['seven_fault_simulation_passed']}`")
        lines.append(f"- **Clean-Room Rust Fixture Audit**: `{st['clean_fixture_audit_passed']}`")
        lines.append(f"- **Contaminated C++ Fixture Detection**: `{st['contaminated_fixture_detection_passed']}`")
        lines.append("")

    return "\n".join(lines)


def main(argv: list[str] | None = None) -> int:
    parser = argparse.ArgumentParser(
        description="Verify ISO 26262-6:2018 Annex D.2.4 clean-room E2E Black-Channel CRCs, fault simulations, and codebase compliance."
    )
    parser.add_argument(
        "--verify-crc",
        "--verify-crcs",
        dest="verify_crc",
        action="store_true",
        help="Verify all 4 public-domain CRCs (SAE-J1850, ITU-T X.25, IEEE 802.3, Castagnoli CRC-32C) against Rocksoft check vectors.",
    )
    parser.add_argument(
        "--simulate-faults",
        action="store_true",
        help="Run deterministic fault-injection simulation across all 7 ISO 26262-6 Annex D.2.4 communication faults.",
    )
    parser.add_argument(
        "--audit-path",
        "--audit",
        "--audit-rust",
        "--scan-dir",
        dest="audit_path",
        type=pathlib.Path,
        default=None,
        help="Audit a Rust or C/C++ directory/file for clean-room E2E compliance and Annex D.2.4 fault coverage.",
    )
    parser.add_argument(
        "--strict",
        action="store_true",
        help="Exit 1 when --audit-path finds E2E safety hazards, architectural defects, or missing Annex D.2.4 fault checks.",
    )
    parser.add_argument(
        "--dump-fixtures",
        type=pathlib.Path,
        default=None,
        help="Materialize a clean-room Rust black_channel.rs reference module and test_vectors.json to the target directory.",
    )
    parser.add_argument(
        "--self-test",
        action="store_true",
        help="Run full built-in verification suite (CRC vectors, 7-fault simulation, and clean vs. contaminated audits).",
    )
    parser.add_argument(
        "--format",
        choices=("markdown", "json", "text"),
        default="markdown",
        help="Output report format (default: markdown).",
    )
    parser.add_argument(
        "--output",
        type=pathlib.Path,
        default=None,
        help="Optional file path to write the output report.",
    )

    args = parser.parse_args(argv)

    default_mode = not (
        args.verify_crc
        or args.simulate_faults
        or args.audit_path is not None
        or args.dump_fixtures is not None
        or args.self_test
    )

    report: dict[str, Any] = {}
    if args.dump_fixtures is not None:
        report["fixtures"] = dump_fixtures(args.dump_fixtures)

    if args.verify_crc or default_mode or args.self_test:
        report["crc_verification"] = verify_all_public_domain_crcs()

    if args.simulate_faults or default_mode or args.self_test:
        report["fault_simulation"] = run_seven_fault_simulation()

    if args.audit_path is not None:
        report["audit"] = audit_codebase(args.audit_path)

    if args.self_test:
        report["self_test"] = run_self_test()

    if args.format == "json":
        rendered = json.dumps(report, indent=2)
    else:
        rendered = format_markdown_report(report)

    if args.output is not None:
        args.output.parent.mkdir(parents=True, exist_ok=True)
        args.output.write_text(rendered + "\n", encoding="utf-8")

    print(rendered)

    if "crc_verification" in report and not report["crc_verification"]["all_passed"]:
        return 1
    if "fault_simulation" in report and not report["fault_simulation"]["all_detected"]:
        return 1
    if "self_test" in report and not report["self_test"]["self_test_passed"]:
        return 1
    if args.strict and "audit" in report:
        aud = report["audit"]
        if not aud.get("clean_room_compliant", True) or aud.get("ip_violations") or aud.get("architectural_defects"):
            return 1
        cov = aud.get("annex_d24_rust_coverage") or []
        if cov and not all(item.get("present", False) for item in cov):
            return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
