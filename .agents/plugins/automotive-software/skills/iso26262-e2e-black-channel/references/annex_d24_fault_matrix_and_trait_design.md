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

# ISO 26262-6:2018 Annex D.2.4 Fault Matrix & Clean-Room Rust Trait Design

## 1. ISO 26262-6:2018 Annex D.2.4 Seven-Fault Coverage Matrix

ISO 26262-6:2018 Clause 7.4.6 and Annex D.2.4 (*Exchange of information*) require that when safety-related elements exchange data across an underlying channel not developed to the target ASIL (e.g., UDP/TCP sockets, Unix Domain Sockets, or a QM SOME/IP routing daemon), the end-to-end safety mechanism must detect all seven communication faults below:

| Fault ID | ISO 26262-6 Annex D.2.4 Fault | SOME/IP / IPC Failure Scenario | Clean-Room Detection Rule | Mathematical Predicate | Rust `BlackChannelFault` Variant |
|---|---|---|---|---|---|
| **F1** | **Repetition of information** | Ethernet switch duplicates UDP packet; stuck DMA descriptor re-transmits previous frame | Reject zero sequence delta on initialized channel | `state.initialized && delta == 0` | `BlackChannelFault::Repetition { seq }` |
| **F2** | **Loss of information** | Socket receive queue overflow (`SO_RCVBUF`); dropped cyclic SOME/IP notification | Modular sequence jump exceeds configured `max_delta` window | `1 < delta && delta <= half_mod && delta > config.max_delta` | `BlackChannelFault::MessageLoss { lost_frames: delta - 1, max_delta }` |
| **F3** | **Insertion of information** | Spurious packet injected by malfunctioning QM node on shared UDP port | Implicit 32-bit `DataId` + `InstanceId` pseudo-header mixed into CRC + counter window gate | `rx_crc != calc_crc(pseudo_hdr \|\| frame)` or `delta > config.max_delta` | `BlackChannelFault::SpuriousInsertion { delta }` / `Masquerading` |
| **F4** | **Incorrect sequence** | Out-of-order UDP datagrams across multi-queue NIC or thread pool | Modular wrap-around delta resides in upper half of counter ring (`> Modulo / 2`) | `delta > (COUNTER_MODULO / 2)` | `BlackChannelFault::IncorrectSequence { expected_next, received }` |
| **F5** | **Corruption of information** | Bit-flip in transit/RAM, truncated payload, or post-seal header overwrite | Public-domain CRC-8/16/32/32C computed over pseudo-header + upper frame header + payload | `rx_crc != expected_crc` | `BlackChannelFault::CorruptedCrc { expected, computed }` |
| **F6** | **Asymmetric information / Delay** | Starved publisher thread, priority inversion, or stalled network switch queue | Receiver monotonic clock check (`Instant`) + sender microsecond timestamp delta | `now_us.saturating_sub(last_rx_us) > config.deadline_timeout_us` | `BlackChannelFault::DeadlineTimeout { elapsed_us, budget_us }` |
| **F7** | **Masquerading / Incorrect addressing** | Valid frame from Service A routed to Service B handler, or corrupted Service/Method ID | Implicit 32-bit `DataId` (`(service_id as u32) << 16 \| method_id as u32`) + `instance_id` + `interface_version` folded into CRC | `wire_data_id != config.data_id` or `rx_crc != expected_crc` | `BlackChannelFault::Masquerading { expected_data_id, wire_data_id }` |

---

## 2. Wire Envelope Layout & Header Coverage

### 2.1 16-Byte RPC Frame Structure & Protection Base (`offset = 8` vs. `offset = 16`)
A standard 16-byte service-oriented RPC header consists of:
- **Bytes `0..4` (`Message ID`)**: `Service ID` (`u16` BE, bytes `0..2`) + `Method/Event ID` (`u16` BE, bytes `2..4`).
- **Bytes `4..8` (`Length`)**: `u32` BE payload length covering bytes `8..end`.
- **Bytes `8..12` (`Request ID`)**: `Client ID` (`u16` BE, bytes `8..10`) + `Session ID` (`u16` BE, bytes `10..12`).
- **Byte `12`**: `Protocol Version` (`0x01`).
- **Byte `13`**: `Interface Version` (`u8`).
- **Byte `14`**: `Message Type` (`u8`).
- **Byte `15`**: `Return Code` (`u8`).
- **Bytes `16..16 + E2E_HDR_LEN` (`BlackChannelEnvelope`)**: 8-byte safety envelope placed at the start of the payload.
- **Bytes `16 + E2E_HDR_LEN..end`**: Application payload bytes.

```
Byte:  0               2               4                               8
      +---------------+---------------+-------------------------------+
      |   Service ID  |   Method ID   |          Length (u32)         |  <-- Mixed into CRC via
      +---------------+---------------+---------------+---------------+      Implicit Pseudo-Header
Byte:  8              10              12      13      14      15
      +---------------+---------------+-------+-------+-------+-------+
      |   Client ID   |   Session ID  |ProtoV |IfaceV |MsgType|RetCode|  <-- Protected In-Place
      +---------------+---------------+-------+-------+-------+-------+      (Bytes 8..16)
Byte: 16              20              22              24
      +-------------------------------+---------------+---------------+
      |         CRC-32 / CRC-32C      |  SeqCounter   |  FreshnessLo  |  <-- Clean-Room 8-Byte
      |           (u32 BE)            |   (u16 BE)    |   (u16 BE)    |      BlackChannelHeader
      +-------------------------------+---------------+---------------+
Byte: 24..N
      +---------------------------------------------------------------+
      |                   Application Payload Bytes                   |  <-- Protected Payload
      +---------------------------------------------------------------+
```

### 2.2 Common Integration Defects to Eliminate
1. **Heap Reallocations & Post-Seal Header Mutation on TX**:
   - Copying `data[base..]` into a temporary `Vec<u8>`, sealing, and inserting header bytes performs two unnecessary heap allocations per packet.
   - Overwriting `ClientId` (`bytes 8..10`) *after* `protect()` has sealed `bytes 8..end` invalidates the CRC on the wire.
2. **Unchecked Slice Underflow & Silent Corrupt Delivery on RX**:
   - Slicing `data[base..len]` without guarding `len >= MIN_PROTECTED_FRAME_SIZE` panics or underflows on runt frames.
   - Logging CRC failures at `INFO` and forwarding unverified bytes behind an opt-in `is_valid_crc()` boolean lets application callbacks read corrupted state.

---

## 3. Modular Counter Arithmetic & Monotonic Deadline State Machine

### 3.1 Unsigned Modular Sequence Delta (`u8` or `u16`)
For a $W$-bit unsigned rollover counter ($M = 2^W$, where $W = 16 \implies M = 65536$ or $W = 8 \implies M = 256$):
$$\Delta = (\text{rx\_seq} - \text{last\_seq}) \bmod 2^W$$
In Rust, `rx_seq.wrapping_sub(last_seq)` computes $\Delta \in [0, 2^W - 1]$ without overflow panics:
- $\Delta = 0$: **Fault F1 (`Repetition`)** — duplicate frame.
- $\Delta = 1$: **Nominal (`Normal`)** — zero lost frames.
- $1 < \Delta \le \text{max\_delta}$: **Tolerated Gap (`OkWithGap`)** or **Fault F2 (`MessageLoss`)** depending on channel strictness (`allow_gaps`).
- $\text{max\_delta} < \Delta \le 2^{W-1}$: **Fault F2/F3 (`MessageLoss` / `SpuriousInsertion`)** — unacceptably large forward gap.
- $\Delta > 2^{W-1}$: **Fault F4 (`IncorrectSequence`)** — backward jump / out-of-order stale frame from the past.

---

## 4. Complete `#![no_std]` Rust Implementation

Self-contained module with zero heap allocations, compile-time `SealedFrame<'a>` header immutability, and `VerifiedPayload<'a>` read gating:

```rust
#![cfg_attr(not(test), no_std)]
#![forbid(unsafe_code)]
#![deny(
    clippy::unwrap_used,
    clippy::expect_used,
    clippy::indexing_slicing,
    clippy::arithmetic_side_effects
)]

/// 16-byte RPC frame header constants
pub const SOMEIP_FULL_HEADER_SIZE: usize = 16;
pub const SOMEIP_UPPER_HEADER_OFFSET: usize = 8;
pub const BLACK_CHANNEL_HEADER_SIZE: usize = 8;
pub const MIN_PROTECTED_FRAME_SIZE: usize = SOMEIP_FULL_HEADER_SIZE + BLACK_CHANNEL_HEADER_SIZE;

/// Selects one of the 4 public-domain CRC algorithms.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum CrcAlgorithm {
    /// SAE-J1850 CRC-8: Poly=0x1D, Init=0xFF, RefIn=false, RefOut=false, XorOut=0xFF
    SaeJ1850Crc8,
    /// ITU-T X.25 / CCITT-FALSE CRC-16: Poly=0x1021, Init=0xFFFF, RefIn=false, RefOut=false, XorOut=0x0000
    ItuX25Crc16,
    /// IEEE 802.3 CRC-32: Poly=0x04C11DB7, Init=0xFFFFFFFF, RefIn=true, RefOut=true, XorOut=0xFFFFFFFF
    Ieee8023Crc32,
    /// iSCSI / Castagnoli CRC-32C (RFC 3720): Poly=0x1EDC6F41, Init=0xFFFFFFFF, RefIn=true, RefOut=true, XorOut=0xFFFFFFFF
    CastagnoliCrc32c,
}

/// Static configuration for an ISO 26262-6 Annex D.2.4 Black-Channel endpoint.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct BlackChannelConfig {
    /// Logical 32-bit channel identifier: `(service_id as u32) << 16 | (method_id as u32)`
    pub data_id: u32,
    /// Expected SOME/IP service instance ID (mixed into implicit pseudo-header)
    pub instance_id: u16,
    /// Expected SOME/IP interface version
    pub interface_version: u8,
    /// Public-domain CRC algorithm
    pub crc_algorithm: CrcAlgorithm,
    /// Maximum tolerated sequence counter jump (`1` = zero frame loss tolerated)
    pub max_delta: u16,
    /// Maximum inter-arrival deadline in microseconds (`0` disables timeout check)
    pub deadline_timeout_us: u64,
}

/// Exhaustive enumeration of ISO 26262-6:2018 Annex D.2.4 communication faults.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum BlackChannelFault {
    /// Frame buffer is shorter than `MIN_PROTECTED_FRAME_SIZE` (24 bytes)
    EnvelopeTooShort { actual: usize, min: usize },
    /// Annex D.2.4 Fault 1: Repetition of information (`delta == 0`)
    Repetition { seq: u16 },
    /// Annex D.2.4 Fault 2: Loss of information (`max_delta < delta <= 32768`)
    MessageLoss { lost_frames: u16, max_delta: u16 },
    /// Annex D.2.4 Fault 3: Insertion of spurious information
    SpuriousInsertion { delta: u16 },
    /// Annex D.2.4 Fault 4: Incorrect sequence / backward counter jump (`delta > 32768`)
    IncorrectSequence { expected_next: u16, received: u16 },
    /// Annex D.2.4 Fault 5: Corruption of information (CRC mismatch)
    CorruptedCrc { expected: u32, computed: u32 },
    /// Annex D.2.4 Fault 6: Asymmetric delay / reception deadline timeout exceeded
    DeadlineTimeout { elapsed_us: u64, budget_us: u64 },
    /// Annex D.2.4 Fault 7: Masquerading / incorrect addressing (Service/Method/Version mismatch)
    Masquerading { expected_data_id: u32, wire_data_id: u32 },
}

/// Type-state token proving an outbound frame has had its header finalized
/// and its E2E Black-Channel CRC sealed. Prevents post-seal header mutation bugs.
#[derive(Debug)]
pub struct SealedFrame<'a> {
    wire_bytes: &'a [u8],
}

impl<'a> SealedFrame<'a> {
    #[inline]
    pub fn as_wire_slice(&self) -> &'a [u8] {
        self.wire_bytes
    }
}

/// Type-state token proving an inbound frame passed all 7 Annex D.2.4 checks.
/// Eliminates opt-in `is_valid_crc()` unchecked payload delivery hazards.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct VerifiedPayload<'a> {
    data_id: u32,
    instance_id: u16,
    sequence_counter: u16,
    freshness_tick: u16,
    lost_frames_in_window: u16,
    payload: &'a [u8],
}

impl<'a> VerifiedPayload<'a> {
    #[inline]
    pub fn payload(&self) -> &'a [u8] {
        self.payload
    }
    #[inline]
    pub fn sequence_counter(&self) -> u16 {
        self.sequence_counter
    }
    #[inline]
    pub fn data_id(&self) -> u32 {
        self.data_id
    }
    #[inline]
    pub fn lost_frames_in_window(&self) -> u16 {
        self.lost_frames_in_window
    }
}

/// Clean-room trait for sealing and verifying Annex D.2.4 safety envelopes.
pub trait PayloadProtector {
    /// Finalizes the outbound SOME/IP header fields, writes the 8-byte Black-Channel header
    /// at bytes `16..24`, computes the public-domain CRC over the implicit pseudo-header +
    /// upper SOME/IP header (`8..16`) + counter/freshness (`20..24`) + application payload (`24..`),
    /// and returns an immutable `SealedFrame<'a>`.
    fn seal_frame<'a>(
        &mut self,
        frame_buf: &'a mut [u8],
        client_id: u16,
        session_id: u16,
        now_monotonic_us: u64,
    ) -> Result<SealedFrame<'a>, BlackChannelFault>;

    /// Verifies an inbound SOME/IP frame against all 7 ISO 26262-6 Annex D.2.4 faults.
    /// Returns `Ok(VerifiedPayload<'a>)` only if every check succeeds.
    fn verify_frame<'a>(
        &mut self,
        frame_bytes: &'a [u8],
        now_monotonic_us: u64,
    ) -> Result<VerifiedPayload<'a>, BlackChannelFault>;
}

/// Per-channel state machine for sender and receiver.
#[derive(Debug, Clone)]
pub struct BlackChannelEndpoint {
    config: BlackChannelConfig,
    tx_seq: u16,
    rx_initialized: bool,
    rx_last_seq: u16,
    rx_last_valid_us: u64,
}

impl BlackChannelEndpoint {
    pub const fn new(config: BlackChannelConfig) -> Self {
        Self {
            config,
            tx_seq: 0,
            rx_initialized: false,
            rx_last_seq: 0,
            rx_last_valid_us: 0,
        }
    }

    /// Periodic watchdog check for Annex D.2.4 Fault 6 (Deadline Timeout) even when zero packets arrive.
    pub fn poll_deadline(&self, now_monotonic_us: u64) -> Result<(), BlackChannelFault> {
        if self.rx_initialized && self.config.deadline_timeout_us > 0 {
            let elapsed_us = now_monotonic_us.saturating_sub(self.rx_last_valid_us);
            if elapsed_us > self.config.deadline_timeout_us {
                return Err(BlackChannelFault::DeadlineTimeout {
                    elapsed_us,
                    budget_us: self.config.deadline_timeout_us,
                });
            }
        }
        Ok(())
    }

    fn compute_envelope_crc(&self, upper_someip_hdr: &[u8], seq_fresh_bytes: &[u8], app_payload: &[u8]) -> u32 {
        // 8-byte implicit pseudo-header binds DataId (ServiceId || MethodId), InstanceId, and InterfaceVersion
        let data_id_be = self.config.data_id.to_be_bytes();
        let inst_be = self.config.instance_id.to_be_bytes();
        let pseudo_hdr: [u8; 8] = [
            data_id_be[0],
            data_id_be[1],
            data_id_be[2],
            data_id_be[3],
            inst_be[0],
            inst_be[1],
            0x01, // SOME/IP Protocol Version
            self.config.interface_version,
        ];
        compute_segmented_crc(
            self.config.crc_algorithm,
            &[&pseudo_hdr, upper_someip_hdr, seq_fresh_bytes, app_payload],
        )
    }
}

impl PayloadProtector for BlackChannelEndpoint {
    fn seal_frame<'a>(
        &mut self,
        frame_buf: &'a mut [u8],
        client_id: u16,
        session_id: u16,
        now_monotonic_us: u64,
    ) -> Result<SealedFrame<'a>, BlackChannelFault> {
        if frame_buf.len() < MIN_PROTECTED_FRAME_SIZE {
            return Err(BlackChannelFault::EnvelopeTooShort {
                actual: frame_buf.len(),
                min: MIN_PROTECTED_FRAME_SIZE,
            });
        }

        // 1. Write ServiceId, MethodId, Length, ClientId, SessionId, ProtoVer, InterfaceVer BEFORE CRC
        let data_id_be = self.config.data_id.to_be_bytes();
        let length_val = (frame_buf.len().saturating_sub(SOMEIP_UPPER_HEADER_OFFSET)) as u32;
        let length_be = length_val.to_be_bytes();
        let client_be = client_id.to_be_bytes();
        let session_be = session_id.to_be_bytes();

        if let Some(hdr) = frame_buf.get_mut(0..SOMEIP_FULL_HEADER_SIZE) {
            hdr.copy_from_slice(&[
                data_id_be[0],
                data_id_be[1],
                data_id_be[2],
                data_id_be[3],
                length_be[0],
                length_be[1],
                length_be[2],
                length_be[3],
                client_be[0],
                client_be[1],
                session_be[0],
                session_be[1],
                0x01,
                self.config.interface_version,
                hdr.get(14).copied().unwrap_or(0x02), // Preserve or default to NOTIFICATION (0x02)
                hdr.get(15).copied().unwrap_or(0x00), // E_OK (0x00)
            ]);
        }

        // 2. Write Sequence Counter and Freshness Tick (bytes 20..24)
        let seq = self.tx_seq;
        self.tx_seq = self.tx_seq.wrapping_add(1);
        let freshness_tick = ((now_monotonic_us / 1_000) & 0xFFFF) as u16;
        let seq_be = seq.to_be_bytes();
        let fresh_be = freshness_tick.to_be_bytes();

        if let Some(seq_slice) = frame_buf.get_mut(20..24) {
            seq_slice.copy_from_slice(&[seq_be[0], seq_be[1], fresh_be[0], fresh_be[1]]);
        }

        // 3. Compute CRC over implicit pseudo-header + upper SOME/IP header (8..16) + (20..24) + payload (24..)
        let crc = {
            let upper_hdr = frame_buf.get(8..16).unwrap_or(&[]);
            let seq_fresh = frame_buf.get(20..24).unwrap_or(&[]);
            let app_payload = frame_buf.get(24..).unwrap_or(&[]);
            self.compute_envelope_crc(upper_hdr, seq_fresh, app_payload)
        };

        let crc_be = crc.to_be_bytes();
        if let Some(crc_slice) = frame_buf.get_mut(16..20) {
            crc_slice.copy_from_slice(&crc_be);
        }

        Ok(SealedFrame { wire_bytes: frame_buf })
    }

    fn verify_frame<'a>(
        &mut self,
        frame_bytes: &'a [u8],
        now_monotonic_us: u64,
    ) -> Result<VerifiedPayload<'a>, BlackChannelFault> {
        // 0. Bounds check before slicing header or envelope
        if frame_bytes.len() < MIN_PROTECTED_FRAME_SIZE {
            return Err(BlackChannelFault::EnvelopeTooShort {
                actual: frame_bytes.len(),
                min: MIN_PROTECTED_FRAME_SIZE,
            });
        }

        // 1. Check Masquerading / Addressing on wire ServiceId || MethodId and InterfaceVersion (Fault 7)
        let wire_data_id = u32::from_be_bytes([
            frame_bytes.first().copied().unwrap_or(0),
            frame_bytes.get(1).copied().unwrap_or(0),
            frame_bytes.get(2).copied().unwrap_or(0),
            frame_bytes.get(3).copied().unwrap_or(0),
        ]);
        let wire_iface_ver = frame_bytes.get(13).copied().unwrap_or(0);
        if wire_data_id != self.config.data_id || wire_iface_ver != self.config.interface_version {
            return Err(BlackChannelFault::Masquerading {
                expected_data_id: self.config.data_id,
                wire_data_id,
            });
        }

        // 2. Check CRC Integrity (Fault 5 & implicit Fault 7 pseudo-header)
        let rx_crc = u32::from_be_bytes([
            frame_bytes.get(16).copied().unwrap_or(0),
            frame_bytes.get(17).copied().unwrap_or(0),
            frame_bytes.get(18).copied().unwrap_or(0),
            frame_bytes.get(19).copied().unwrap_or(0),
        ]);
        let upper_hdr = frame_bytes.get(8..16).unwrap_or(&[]);
        let seq_fresh = frame_bytes.get(20..24).unwrap_or(&[]);
        let app_payload = frame_bytes.get(24..).unwrap_or(&[]);
        let expected_crc = self.compute_envelope_crc(upper_hdr, seq_fresh, app_payload);

        if rx_crc != expected_crc {
            return Err(BlackChannelFault::CorruptedCrc {
                expected: expected_crc,
                computed: rx_crc,
            });
        }

        // 3. Check Reception Deadline Timeout (Fault 6: Asymmetric / Delay of information)
        self.poll_deadline(now_monotonic_us)?;

        // 4. Extract Sequence Counter and evaluate Annex D.2.4 Faults 1, 2, 3, 4
        let rx_seq = u16::from_be_bytes([
            frame_bytes.get(20).copied().unwrap_or(0),
            frame_bytes.get(21).copied().unwrap_or(0),
        ]);
        let freshness_tick = u16::from_be_bytes([
            frame_bytes.get(22).copied().unwrap_or(0),
            frame_bytes.get(23).copied().unwrap_or(0),
        ]);

        let lost_frames_in_window = if !self.rx_initialized {
            self.rx_initialized = true;
            self.rx_last_seq = rx_seq;
            self.rx_last_valid_us = now_monotonic_us;
            0
        } else {
            let delta = rx_seq.wrapping_sub(self.rx_last_seq);
            if delta == 0 {
                // Fault 1: Repetition
                return Err(BlackChannelFault::Repetition { seq: rx_seq });
            } else if delta > 0x8000 {
                // Fault 4: Incorrect sequence (backward counter jump / out-of-order frame)
                return Err(BlackChannelFault::IncorrectSequence {
                    expected_next: self.rx_last_seq.wrapping_add(1),
                    received: rx_seq,
                });
            } else if delta > self.config.max_delta {
                // Fault 2 / Fault 3: Message loss exceeding window or spurious insertion outside window
                if delta > self.config.max_delta.saturating_mul(4) {
                    return Err(BlackChannelFault::SpuriousInsertion { delta });
                }
                return Err(BlackChannelFault::MessageLoss {
                    lost_frames: delta.saturating_sub(1),
                    max_delta: self.config.max_delta,
                });
            } else {
                self.rx_last_seq = rx_seq;
                self.rx_last_valid_us = now_monotonic_us;
                delta.saturating_sub(1)
            }
        };

        Ok(VerifiedPayload {
            data_id: self.config.data_id,
            instance_id: self.config.instance_id,
            sequence_counter: rx_seq,
            freshness_tick,
            lost_frames_in_window,
            payload: app_payload,
        })
    }
}

/// Dispatches segmented CRC calculation across the 4 public-domain standards.
pub fn compute_segmented_crc(alg: CrcAlgorithm, segments: &[&[u8]]) -> u32 {
    match alg {
        CrcAlgorithm::SaeJ1850Crc8 => {
            let mut crc: u8 = 0xFF;
            for seg in segments {
                for &b in *seg {
                    let mut x = crc ^ b;
                    let mut bit = 0;
                    while bit < 8 {
                        x = if (x & 0x80) != 0 { (x << 1) ^ 0x1D } else { x << 1 };
                        bit += 1;
                    }
                    crc = x;
                }
            }
            (crc ^ 0xFF) as u32
        }
        CrcAlgorithm::ItuX25Crc16 => {
            let mut crc: u16 = 0xFFFF;
            for seg in segments {
                for &b in *seg {
                    let mut x = crc ^ ((b as u16) << 8);
                    let mut bit = 0;
                    while bit < 8 {
                        x = if (x & 0x8000) != 0 { (x << 1) ^ 0x1021 } else { x << 1 };
                        bit += 1;
                    }
                    crc = x;
                }
            }
            crc as u32
        }
        CrcAlgorithm::Ieee8023Crc32 => {
            let mut crc: u32 = 0xFFFF_FFFF;
            for seg in segments {
                for &b in *seg {
                    let mut x = (crc ^ (b as u32)) & 0xFF;
                    let mut bit = 0;
                    while bit < 8 {
                        x = if (x & 1) != 0 { (x >> 1) ^ 0xEDB8_8320 } else { x >> 1 };
                        bit += 1;
                    }
                    crc = (crc >> 8) ^ x;
                }
            }
            crc ^ 0xFFFF_FFFF
        }
        CrcAlgorithm::CastagnoliCrc32c => {
            let mut crc: u32 = 0xFFFF_FFFF;
            for seg in segments {
                for &b in *seg {
                    let mut x = (crc ^ (b as u32)) & 0xFF;
                    let mut bit = 0;
                    while bit < 8 {
                        x = if (x & 1) != 0 { (x >> 1) ^ 0x82F6_3B78 } else { x >> 1 };
                        bit += 1;
                    }
                    crc = (crc >> 8) ^ x;
                }
            }
            crc ^ 0xFFFF_FFFF
        }
    }
}
```

---

## 5. Bridging a Routing Crate's Pluggable `PayloadProtector` Hook (`std` Adapter)

In a modular middleware workspace, the core routing crate defines a minimal `RoutingPayloadProtector` plugin trait (`add_configuration`, `is_protected`, `is_checked`, `protection_base_offset`, `protect`, `check`) with zero built-in E2E dependencies, while an opt-in ISO 26262-6 Annex D.2.4 crate wraps `BlackChannelEndpoint` to implement that hook:

```rust
use std::collections::HashMap;
use std::sync::Mutex;
use std::time::Instant;

/// Vendor-neutral configuration passed through from optional JSON `"e2e"` blocks.
#[derive(Debug, Clone, Default)]
pub struct CustomProtectionConfig {
    pub enabled: bool,
    pub variant: String,
    pub profile: String,
    pub service_id: u16,
    pub event_id: u16,
    pub custom_parameters: HashMap<String, String>,
}

/// Core routing crate plugin trait (`PluginManager::register_payload_protector`).
pub trait RoutingPayloadProtector: Send + Sync {
    fn add_configuration(&self, config: &CustomProtectionConfig) -> bool;
    fn is_protected(&self, service: u16, method: u16) -> bool;
    fn is_checked(&self, service: u16, method: u16) -> bool;
    fn protection_base_offset(&self, service: u16, method: u16) -> usize;
    fn protect(&self, service: u16, method: u16, instance: u16, buffer: &mut Vec<u8>);
    fn check(&self, service: u16, method: u16, instance: u16, buffer: &[u8]) -> u8;
}

/// Thread-safe adapter bridging `BlackChannelEndpoint` into the routing manager's plugin hook.
pub struct AnnexD24PluginAdapter {
    boot_instant: Instant,
    endpoints: Mutex<HashMap<(u16, u16), (bool, bool, BlackChannelEndpoint)>>,
}

impl AnnexD24PluginAdapter {
    pub fn new() -> Self {
        Self {
            boot_instant: Instant::now(),
            endpoints: Mutex::new(HashMap::new()),
        }
    }

    fn monotonic_us(&self) -> u64 {
        u64::try_from(self.boot_instant.elapsed().as_micros()).unwrap_or(u64::MAX)
    }
}

impl RoutingPayloadProtector for AnnexD24PluginAdapter {
    fn add_configuration(&self, config: &CustomProtectionConfig) -> bool {
        if !config.enabled {
            return false;
        }
        let is_prot = config.variant.contains("protector") || config.variant == "both";
        let is_chk = config.variant.contains("checker") || config.variant == "both";
        let data_id = (u32::from(config.service_id) << 16) | u32::from(config.event_id);
        let bc_cfg = BlackChannelConfig {
            data_id,
            instance_id: 1,
            expected_interface_version: 1,
            crc_algorithm: CrcAlgorithm::CastagnoliCrc32c,
            max_delta: 3,
            deadline_timeout_us: 100_000,
        };
        if let Ok(mut guard) = self.endpoints.lock() {
            guard.insert(
                (config.service_id, config.event_id),
                (is_prot, is_chk, BlackChannelEndpoint::new(bc_cfg)),
            );
            true
        } else {
            false
        }
    }

    fn is_protected(&self, service: u16, method: u16) -> bool {
        self.endpoints
            .lock()
            .ok()
            .and_then(|g| g.get(&(service, method)).map(|(p, _, _)| *p))
            .unwrap_or(false)
    }

    fn is_checked(&self, service: u16, method: u16) -> bool {
        self.endpoints
            .lock()
            .ok()
            .and_then(|g| g.get(&(service, method)).map(|(_, c, _)| *c))
            .unwrap_or(false)
    }

    fn protection_base_offset(&self, _service: u16, _method: u16) -> usize {
        SOMEIP_HEADER_SIZE + BLACK_CHANNEL_HEADER_SIZE
    }

    fn protect(&self, service: u16, method: u16, _instance: u16, buffer: &mut Vec<u8>) {
        // NOTE: RoutingManagerImpl::send MUST finalize ClientId (bytes 8..10) and SessionId (10..12)
        // BEFORE invoking protect() so the sealed CRC covers the final wire header.
        if buffer.len() < MIN_PROTECTED_FRAME_SIZE {
            return;
        }
        let client_id = u16::from_be_bytes([buffer[8], buffer[9]]);
        let session_id = u16::from_be_bytes([buffer[10], buffer[11]]);
        let now_us = self.monotonic_us();
        if let Ok(mut guard) = self.endpoints.lock() {
            if let Some((true, _, ep)) = guard.get_mut(&(service, method)) {
                let _ = ep.seal_frame(buffer.as_mut_slice(), client_id, session_id, now_us);
            }
        }
    }

    fn check(&self, service: u16, method: u16, _instance: u16, buffer: &[u8]) -> u8 {
        let now_us = self.monotonic_us();
        if let Ok(mut guard) = self.endpoints.lock() {
            if let Some((_, true, ep)) = guard.get_mut(&(service, method)) {
                return match ep.verify_frame(buffer, now_us) {
                    Ok(_) => 0x00, // Valid Annex D.2.4 envelope
                    Err(_) => 0x01, // Reject / mark check_result != 0
                };
            }
        }
        0x00
    }
}
```

