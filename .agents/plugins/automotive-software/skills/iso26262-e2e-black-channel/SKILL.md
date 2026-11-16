---
name: iso26262-e2e-black-channel
description: >-
  Designs, implements, and verifies ISO 26262-6:2018 Annex D.2.4 end-to-end
  (E2E) black-channel communication protection in Rust using clean-room traits
  and public-domain CRCs (SAE-J1850 CRC-8, ITU-T X.25 CRC-16, IEEE 802.3 CRC-32,
  and Castagnoli CRC-32C). Use when: protecting safety-related network or IPC
  frames against the 7 Annex D.2.4 communication faults (repetition, loss,
  insertion, incorrect sequence, corruption, delay/timeout, masquerading),
  designing SealedFrame and VerifiedPayload type-state envelopes, or fixing
  post-seal header mutation and silent fault-forwarding bugs. Don't use for:
  extern "C" FFI boundaries (use iso26262-ffi-isolation) or SEooC/SW-FMEA tables
  (use iso26262-seooc-and-sw-fmea).
---

<!-- Copyright 2026 Google LLC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

        http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License. -->

# ISO 26262-6 Annex D.2.4 Clean-Room E2E Black-Channel Protection

## Quick Start

| Goal | Command |
|---|---|
| **Run built-in CRC, 7-fault & architectural audit self-test** | `python3 skills/iso26262-e2e-black-channel/scripts/verify_e2e_black_channel.py --self-test` |
| **Audit a Rust/C++ crate for Annex D.2.4 E2E safety (`--strict`)** | `python3 skills/iso26262-e2e-black-channel/scripts/verify_e2e_black_channel.py --audit-path crates/e2e-protector --strict` |
| **Verify Rocksoft `"123456789"` CRC vectors & simulate 7 faults** | `python3 skills/iso26262-e2e-black-channel/scripts/verify_e2e_black_channel.py --verify-crc --simulate-faults` |

## 1. Architectural Mandate (`ISO 26262-6:2018` Clause 7.4.6 & Annex D.2.4)

Under **ISO 26262-6:2018 Clause 7.4.6 and Annex D.2.4**, when safety-related software elements (ASIL A–D) exchange information over an underlying transport or routing layer that is not qualified to the highest applicable ASIL (such as UDP/IP, TCP/IP, Unix Domain Sockets, shared memory ring buffers, or a QM routing daemon), the transport path must be treated as an untrusted **"Black Channel"**. Safety assurance relies on an end-to-end (E2E) safety envelope computed at the sender and verified at the receiver before data consumption, backed by **internationally standardized, public-domain CRCs** (SAE-J1850, ITU-T X.25 / CCITT-FALSE, IEEE 802.3, and IETF RFC 3720 Castagnoli).

---

## 2. ISO 26262-6:2018 Annex D.2.4 Seven-Fault Model

Every Black-Channel implementation must detect all **7 communication faults** defined in ISO 26262-6:2018 Annex D.2.4:

| # | Annex D.2.4 Fault Type | Physical / Software Root Cause | Mandatory Detection Mechanism | Fault Variant Returned |
|---|---|---|---|---|
| **1** | **Repetition of information** | Stuck TX DMA ring, switch packet duplication, or re-sent buffer | Monotonic sequence counter (`seq`) where `delta == 0` is rejected on cyclic/event frames | `BlackChannelFault::Repetition { seq }` |
| **2** | **Loss of information** | Dropped UDP packet, full socket queue, or skipped TX cycle | Modular counter delta check: `1 < delta <= max_delta` flags tolerated loss count; `delta > max_delta` triggers sequence loss fault | `BlackChannelFault::MessageLoss { lost_frames, max_delta }` |
| **3** | **Insertion of information** | Spurious frame from malfunctioning QM peer or replay outside window | Implicit 32-bit `DataId` pseudo-header mixed into CRC + counter window check (`delta > max_delta`) | `BlackChannelFault::SpuriousInsertion { delta }` |
| **4** | **Incorrect sequence** | Out-of-order UDP routing, multi-queue thread race, or backward jump | Unsigned modular arithmetic `delta = rx_seq.wrapping_sub(last_seq)`; backward jump maps to `delta > (COUNTER_MODULO / 2)` | `BlackChannelFault::IncorrectSequence { expected_next, received }` |
| **5** | **Corruption of information** | Bit flips in RAM/bus/NIC, partial DMA write, or post-CRC header mutation | Public-domain CRC (SAE-J1850 CRC-8, ITU-T CRC-16, IEEE 802.3 CRC-32, or CRC-32C) over pseudo-header + upper frame header + payload | `BlackChannelFault::CorruptedCrc { expected, computed }` |
| **6** | **Asymmetric / Delay of information** | Starved TX/RX thread, network congestion, or dead publisher | Receiver-side monotonic clock deadline monitor (`Instant::now().duration_since(last_valid_rx) > deadline_timeout`) | `BlackChannelFault::DeadlineTimeout { elapsed_us, budget_us }` |
| **7** | **Masquerading / Incorrect addressing** | Misrouted frame on shared UDP port or wrong Service/Method/Instance | Mix 32-bit `DataId` + `InstanceId` + `InterfaceVersion` into the CRC state *before* payload bytes without trusting wire IDs alone | `BlackChannelFault::Masquerading { expected_data_id, wire_data_id }` |

---

## 3. Public-Domain CRC Catalog

Use internationally standardized, public-domain CRC polynomials verified against canonical Rocksoft `"123456789"` vectors:

| Public Standard | Width | Normal `Poly` | `Init` | `RefIn` / `RefOut` | `XorOut` | `Check("123456789")` | Recommended Payload Bound |
|---|---|---|---|---|---|---|---|
| **SAE-J1850 CRC-8** | 8-bit | `0x1D` | `0xFF` | `false` / `false` | `0xFF` | `0x4B` | Compact CAN/LIN-bridged actuator frames (`<= 16` B, $HD \ge 4$ up to 11 B) |
| **ITU-T X.25 / CCITT-FALSE CRC-16** | 16-bit | `0x1021` | `0xFFFF` | `false` / `false` | `0x0000` | `0x29B1` | Standard RPC/Event payloads (`<= 256` B, $HD = 4$ up to 4,094 B) |
| **IEEE 802.3 / V.42 CRC-32** | 32-bit | `0x04C11DB7` | `0xFFFFFFFF` | `true` / `true` | `0xFFFFFFFF` | `0xCBF43926` | General Ethernet MTU UDP/TCP frames (`<= 4,096` B, $HD \ge 4$ up to 11,450 B) |
| **iSCSI / Castagnoli CRC-32C (RFC 3720)** | 32-bit | `0x1EDC6F41` | `0xFFFFFFFF` | `true` / `true` | `0xFFFFFFFF` | `0xE3069283` | High-integrity ASIL-B/D frames (`<= 4,096` B, $HD = 6$ up to 512 B; HW `crc32c` on x86 SSE4.2 / ARMv8) |

---

## 4. Preventing Common E2E Architectural Bugs (`Anti-Pattern` vs. `Recommended Pattern`)

### 4.1 Post-Seal Header Mutation
Mutating any header field covered by the CRC (such as overwriting `ClientId` or `SessionId` bytes *after* sealing the E2E envelope) guarantees receiver CRC failures whenever the routing daemon rewrites client identifiers.

```rust
/// Type-state wrapper proving the frame's wire header and E2E envelope are sealed.
/// Exposes only an immutable `&[u8]` slice so routing cannot mutate header bytes after CRC calculation.
pub struct SealedFrame<'a> {
    wire_bytes: &'a [u8],
}

impl<'a> SealedFrame<'a> {
    #[inline]
    pub fn as_wire_slice(&self) -> &'a [u8] {
        self.wire_bytes
    }
}

pub fn prepare_and_seal_outbound<'a, P: PayloadProtector>(
    frame_buf: &'a mut [u8],
    client_id: u16,
    instance_id: u16,
    protector: &mut P,
) -> Result<SealedFrame<'a>, BlackChannelFault> {
    if frame_buf.len() < 16 {
        return Err(BlackChannelFault::EnvelopeTooShort { actual: frame_buf.len(), min: 16 });
    }
    // 1. Finalize ALL header fields (including Client ID at bytes 8..10) BEFORE E2E protection
    frame_buf[8..10].copy_from_slice(&client_id.to_be_bytes());

    // 2. Seal the frame in-place; return an immutable SealedFrame<'a>
    protector.protect_in_place(frame_buf, instance_id)?;
    Ok(SealedFrame { wire_bytes: frame_buf })
}
```

### 4.2 Opt-In `bool is_valid_crc()` vs. Type-Safe `VerifiedPayload<'a>`
Forwarding unverified frames to application handlers with an opt-in `msg.is_valid_crc()` boolean getter lets callers accidentally consume corrupted payloads. At ASIL application boundaries, gate payload access behind a `VerifiedPayload<'a>` token whose inner slice is private and constructible only by `verify_frame()`. (If a QM routing daemon forwards failed frames for legacy diagnostics, keep the protector behind an opt-in Cargo feature and enforce `VerifiedPayload<'a>` quarantine at the safety consumer.)

### 4.3 Implicit 32-Bit `DataId` Pseudo-Header & Strict `1 <= delta <= max_delta`
Computing a CRC solely over wire payload bytes misses **Masquerading** (`F7`) when two topics share identical payload sizes, and allowing `delta == 0` misses **Repetition** (`F1`). Always fold the 32-bit `DataId` + `InstanceId` + `InterfaceVersion` pseudo-header into the CRC before payload bytes and reject `delta == 0`.

---

## 5. Verification Workflow

```bash
# 1. Verify Rocksoft "123456789" CRC-8/16/32/32C vectors and run 7-fault injection simulation
python3 skills/iso26262-e2e-black-channel/scripts/verify_e2e_black_channel.py \
  --verify-crc --simulate-faults --format markdown

# 2. Materialize clean-room Rust reference module and audit it under --strict
python3 skills/iso26262-e2e-black-channel/scripts/verify_e2e_black_channel.py \
  --dump-fixtures /tmp/black_channel_fixtures
python3 skills/iso26262-e2e-black-channel/scripts/verify_e2e_black_channel.py \
  --audit-path /tmp/black_channel_fixtures --strict
```

---

## 6. Deep-Dive References

- **[references/annex_d24_fault_matrix_and_trait_design.md](references/annex_d24_fault_matrix_and_trait_design.md)**: Complete ISO 26262-6:2018 Annex D.2.4 7-fault coverage matrix, modular sequence counter math, monotonic deadline state machine, `#![no_std]` Rust `PayloadProtector` trait, `SealedFrame` / `VerifiedPayload` type-states, and pluggable routing adapter.
- **[references/public_domain_crc_catalog.md](references/public_domain_crc_catalog.md)**: Rocksoft parameterization table, Hamming distance ($HD$) vs. payload length selection guide, and `const fn` 256-entry lookup-table Rust implementations for SAE-J1850 CRC-8, ITU-T X.25 CRC-16, IEEE 802.3 CRC-32, and Castagnoli CRC-32C.
