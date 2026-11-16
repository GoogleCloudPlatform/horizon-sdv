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

# MC/DC Truth-Table Derivation, Boolean Refactorings & Wire Framing Boundary Value Analysis

This reference provides the mathematical derivation of $N+1$ MC/DC independence-pair test vectors, concrete Rust refactorings and unit test suites for 6 representative multi-condition protocol boolean decisions, and ISO 26262-6:2018 Table 8 (`1b` Equivalence Classes, `1c` Boundary Value Analysis) test suites for binary RPC and segmented transport wire frames.

---

## 1. Deriving $N+1$ MC/DC Independence-Pair Test Vectors

Under ISO 26262-6:2018 Clause 9.4.5 Table 9 (`1c`), a decision $D(C_1, C_2, \dots, C_N)$ with $N$ atomic boolean conditions satisfies **Modified Condition/Decision Coverage (MC/DC)** iff:
1. Every condition $C_i$ has evaluated to both `true` and `false`.
2. The decision $D$ has evaluated to both `true` and `false`.
3. For each condition $C_i$, there exists an **independence pair** of executed test vectors $(V_a, V_b)$ such that:
   - $C_i(V_a) \neq C_i(V_b)$ (one is `true`, the other is `false`),
   - $D(V_a) \neq D(V_b)$ (the overall decision outcome flips), and
   - For every other condition $C_j$ ($j \neq i$), either $C_j(V_a) = C_j(V_b)$ (Unique-Cause MC/DC) or $C_j$ is short-circuited (`null` / `MCDC_DontCare` in LLVM `CoverageExporterJson.cpp`) on the branch where it is not evaluated.

### 1.1 Minimum $N+1$ Vector Construction Rules

- **Pure Conjunction ($C_1 \land C_2 \land \dots \land C_N$)**:
  - Base vector $V_0 = (\text{T}, \text{T}, \dots, \text{T}) \rightarrow \text{T}$.
  - Flip one condition at a time to `F`:
    - $V_1 = (\text{F}, -, -, \dots) \rightarrow \text{F}$ (proves $C_1$ paired with $V_0$)
    - $V_2 = (\text{T}, \text{F}, -, \dots) \rightarrow \text{F}$ (proves $C_2$ paired with $V_0$)
    - $\dots$
    - $V_N = (\text{T}, \text{T}, \dots, \text{F}) \rightarrow \text{F}$ (proves $C_N$ paired with $V_0$)
- **Pure Disjunction ($C_1 \lor C_2 \lor \dots \lor C_N$)**:
  - Base vector $V_0 = (\text{F}, \text{F}, \dots, \text{F}) \rightarrow \text{F}$.
  - Flip one condition at a time to `T`:
    - $V_1 = (\text{T}, -, -, \dots) \rightarrow \text{T}$ (proves $C_1$ paired with $V_0$)
    - $V_2 = (\text{F}, \text{T}, -, \dots) \rightarrow \text{T}$ (proves $C_2$ paired with $V_0$)
    - $\dots$
    - $V_N = (\text{F}, \text{F}, \dots, \text{T}) \rightarrow \text{T}$ (proves $C_N$ paired with $V_0$)

---

## 2. Concrete Refactorings & MC/DC Test Suites for 6 Protocol Decisions

### Decision 1: Peer Reboot Flag Transition

- **Coupled C++ Expression**:
  ```cpp
  if (reboot_flag
      && ((is_multicast && !prev_mc_reboot)
          || (!is_multicast && !prev_uc_reboot)))
  ```
- **Why MC/DC Fails on the Original Expression**:
  5 conditions (`C1: reboot_flag`, `C2: is_multicast`, `C3: !prev_mc_reboot`, `C4: !is_multicast`, `C5: !prev_uc_reboot`) where `C4 == !C2`. Because `C2` and `C4` are complements of the same variable, no test input can hold `C4` fixed while toggling `C2`.
- **Refactored Rust + Complete MC/DC Test Suite**:
  ```rust
  pub fn detect_peer_reboot(
      reboot_flag: bool,
      is_multicast: bool,
      prev_multicast_reboot: bool,
      prev_unicast_reboot: bool,
  ) -> bool {
      let prev_reboot_flag = if is_multicast {
          prev_multicast_reboot
      } else {
          prev_unicast_reboot
      };
      // Decision D1: C1 (reboot_flag) && C2 (!prev_reboot_flag)
      reboot_flag && !prev_reboot_flag
  }
  ```
  | Vector | `is_multicast` (Branch) | `C1: reboot_flag` | `C2: !prev_reboot_flag` | Result | Independence Pair |
  |---|---|---|---|---|---|
  | `V1` | `true` (`mc=false`) | `T` | `T` | `true` | Base `T` vector; covers `is_multicast == true` branch |
  | `V2` | `true` (`mc=false`) | `F` | `-` (`T`) | `false` | `(V1, V2)` proves `C1` (`reboot_flag`) |
  | `V3` | `false` (`uc=true`) | `T` | `F` | `false` | `(V1, V3)` proves `C2` (`!prev_reboot_flag`) & covers `is_multicast == false` branch |

---

### Decision 2: Secure Service Port Validation

- **5-Condition Guard**:
  $$D = A \land ((B \land C) \lor (D \land E))$$
  where:
  - $A$: `is_secure`
  - $B$: `reliable_port != ILLEGAL_PORT`
  - $C$: `!is_reliable_port_secure`
  - $D$: `unreliable_port != ILLEGAL_PORT`
  - $E$: `!is_unreliable_port_secure`
- **Complete 6-Vector ($N+1 = 6$) MC/DC Truth Table**:

  | Vector | $A$ | $B$ | $C$ | $D$ | $E$ | LLVM `conditions` Array | Result | Independence Pair Proven |
  |---|---|---|---|---|---|---|---|---|
  | `V1` | `T` | `T` | `T` | `F` | `F` | `[true, true, true, null, null]` | `true` | Pairs with `V2` ($A$), `V3` ($B$), `V4` ($C$) |
  | `V2` | `F` | `T` | `T` | `F` | `F` | `[false, null, null, null, null]` | `false` | **`(V1, V2)` proves $A$ (`is_secure`)** |
  | `V3` | `T` | `F` | `T` | `F` | `T` | `[true, false, null, false, null]` | `false` | **`(V1, V3)` proves $B$ (`reliable_port != ILLEGAL_PORT`)** |
  | `V4` | `T` | `T` | `F` | `F` | `T` | `[true, true, false, false, null]` | `false` | **`(V1, V4)` proves $C$ (`!is_reliable_port_secure`)** |
  | `V5` | `T` | `F` | `T` | `T` | `T` | `[true, false, null, true, true]` | `true` | **`(V3, V5)` proves $D$ (`unreliable_port != ILLEGAL_PORT`)** |
  | `V6` | `T` | `F` | `T` | `T` | `F` | `[true, false, null, true, false]` | `false` | **`(V5, V6)` proves $E$ (`!is_unreliable_port_secure`)** |

---

### Decision 3: Subscription Endpoint Readiness (10-Condition DNF)

- **Oversized & Coupled Expression**:
  ```cpp
  if ((reliable_ep && unreliable_ep && tcp_established && udp_established)
      || (reliable_ep && !unreliable_ep && tcp_established)
      || (unreliable_ep && !reliable_ep && udp_established))
  ```
- **Why MC/DC Fails on the Original Expression**:
  1. **10 atomic conditions** violate the $\le 6$-condition complexity threshold (and exceed LLVM 18's `--mcdc-max-conditions=6` default bitmap cap, $2^{10} = 1024$ vectors).
  2. Coupled variables `reliable_ep` (`C1`, `C5`, `!C9`) and `unreliable_ep` (`C2`, `!C6`, `C8`) make independent condition toggling impossible.
- **Refactored Rust Implementation**:
  Either use the exhaustive `match (has_reliable_ep, has_unreliable_ep)` table (see `SKILL.md` Pillar 2) or pre-compute single-occurrence implication guards:
  ```rust
  pub fn is_subscription_ready(
      has_reliable_ep: bool,
      has_unreliable_ep: bool,
      tcp_established: bool,
      udp_established: bool,
  ) -> bool {
      let any_endpoint_configured = has_reliable_ep || has_unreliable_ep;
      let tcp_ok = !has_reliable_ep || tcp_established;
      let udp_ok = !has_unreliable_ep || udp_established;
      any_endpoint_configured && tcp_ok && udp_ok
  }
  ```
  Each sub-decision now has **2 or 3 independent conditions** (well below the 6-condition limit) with zero coupled variables.

---

### Decision 4: Provider Reliability NACK

- **4-Condition Guard**: `!(first_port_set && !is_first_reliable) && !(second_port_set && !is_second_reliable)` (`A: first_port_set`, `B: !is_first_reliable`, `C: second_port_set`, `D: !is_second_reliable`).
- **Complete 5-Vector ($N+1 = 5$) MC/DC Truth Table**:

  | Vector | $A$ (`first_set`) | $B$ (`!first_rel`) | $C$ (`second_set`) | $D$ (`!second_rel`) | Result (`nack`) | Independence Pair Proven |
  |---|---|---|---|---|---|---|
  | `V1` | `F` | `T` | `F` | `T` | `true` | Base `T` vector (neither port offers UDP -> NACK) |
  | `V2` | `T` | `T` | `F` | `T` | `false` | **`(V1, V2)` proves $A$ (`first_port_set`)** |
  | `V3` | `T` | `F` | `F` | `T` | `true` | **`(V2, V3)` proves $B$ (`!is_first_reliable`)** |
  | `V4` | `F` | `T` | `T` | `T` | `false` | **`(V1, V4)` proves $C$ (`second_port_set`)** |
  | `V5` | `F` | `T` | `T` | `F` | `true` | **`(V4, V5)` proves $D$ (`!is_second_reliable`)** |

---

### Decision 5: Event Field Payload Caching (6-Condition Conjunction)

- **Boolean Structure**: Pure 6-condition conjunction (`!force && is_field && cycle_is_zero && !has_changed && !is_shadow && is_set`), sitting at the exact boundary of LLVM's `--mcdc-max-conditions=6`.
- **Complete 7-Vector ($N+1 = 7$) MC/DC Truth Table**:

  | Vector | `C1: !force` | `C2: is_field` | `C3: cycle==0` | `C4: !changed` | `C5: !shadow` | `C6: is_set` | Result (`suppress`) | Independence Pair |
  |---|---|---|---|---|---|---|---|---|
  | `V1` | `T` | `T` | `T` | `T` | `T` | `T` | `true` | Base `T` vector |
  | `V2` | `F` | `-` | `-` | `-` | `-` | `-` | `false` | `(V1, V2)` proves `C1` |
  | `V3` | `T` | `F` | `-` | `-` | `-` | `-` | `false` | `(V1, V3)` proves `C2` |
  | `V4` | `T` | `T` | `F` | `-` | `-` | `-` | `false` | `(V1, V4)` proves `C3` |
  | `V5` | `T` | `T` | `T` | `F` | `-` | `-` | `false` | `(V1, V5)` proves `C4` |
  | `V6` | `T` | `T` | `T` | `T` | `F` | `-` | `false` | `(V1, V6)` proves `C5` |
  | `V7` | `T` | `T` | `T` | `T` | `T` | `F` | `false` | `(V1, V7)` proves `C6` |

---

### Decision 6: Wildcard Major Version Matching (Hidden Constant-Aliasing Trap)

- **4-Condition Disjunction**:
  ```rust
  offered_major == req_major || req_major == DEFAULT_MAJOR || offered_major == ANY_MAJOR || req_major == ANY_MAJOR
  ```
- **Hidden Fixture Trap**:
  When `DEFAULT_MAJOR == 0x00` and `ANY_MAJOR == 0xFF`, if a test fixture configures the offered service with `offered_major = 0x00` (`DEFAULT_MAJOR`), then `C1` (`offered_major == req_major`) and `C2` (`req_major == DEFAULT_MAJOR`) become identical (`req_major == 0x00`), making `C1 = false, C2 = true` **unreachable**.
- **Mandatory Fixture Rule**: To achieve 100% MC/DC on `C1 || C2 || C3 || C4`, the baseline false vector `V1` must use a concrete non-default, non-wildcard version (e.g., `offered_major = 0x02, req_major = 0x03`):
  - `V1`: `(offered=0x02, req=0x03)` $\rightarrow$ `(F, F, F, F) -> false`
  - `V2`: `(offered=0x03, req=0x03)` $\rightarrow$ `(T, -, -, -) -> true` (proves `C1`)
  - `V3`: `(offered=0x02, req=0x00)` $\rightarrow$ `(F, T, -, -) -> true` (proves `C2`)
  - `V4`: `(offered=0xFF, req=0x03)` $\rightarrow$ `(F, F, T, -) -> true` (proves `C3`)
  - `V5`: `(offered=0x02, req=0xFF)` $\rightarrow$ `(F, F, F, T) -> true` (proves `C4`)

---

## 3. Equivalence Class & Boundary Value Analysis (ISO 26262-6 Table 8 `1b`/`1c`)

### 3.1 Binary RPC & Segmented Framing Boundary Matrix

| Parameter / Field | Equivalence Classes | Mandatory Boundary Test Values |
|---|---|---|
| **UDP Datagram Length (`frame.len()`)** | Invalid: `< 16`; Valid: `16..=1416` (UDP); Invalid UDP: `> 1416` | `0`, `1`, `15` (Err), `16` (Ok empty payload), `17` (Ok 1B payload), `1416` (Ok), `1417` (Err UDP) |
| **Header `Length` Field (`u32`, 8B tail + payload)** | Invalid: `< 8`; Mismatched: `!= frame.len() - 8`; Valid: `== frame.len() - 8` | `0`, `7` (Err `< 8`), `8` (Ok for 16B frame), `9` (Err on 16B frame, Ok on 17B frame), `u32::MAX` (Err overflow) |
| **Segmented Frame Length (`data_len`)** | Invalid: `< 20` (`16 + 4`); Valid: `>= 20` | `0`, `16`, `19` (Err), `20` (header-only segment, fails 0-byte segment check), `21` (Ok 1-byte final segment) |
| **Segment Payload (`seg_len`)** | Invalid: `0`; Valid final (`more=0`): `1..=1392`; Valid non-final (`more=1`): `{16, 32, ..., 1392}`; Invalid: `> 1392` | `0` (Err zero-len), `1` (Ok if `more=0`, Err if `more=1`), `15` (Err if `more=1`), `16` (Ok), `17` (Err if `more=1`), `1391` (Ok if `more=0`, Err if `more=1`), `1392` (`87*16`, Ok for both), `1393` (Err `> 1392`), `1400` (Err unaligned max) |
| **28-bit Shifted Byte Offset (`offset`)** | Valid: `offset % 16 == 0` and `offset + seg_len <= max_msg_size`; Invalid: overflow or `> max_msg_size` | `0`, `16`, `1392`, `max_msg_size - 16` (Ok with 16B seg), `max_msg_size - 15` (Err), `0xFFFF_FFF0` (max 28-bit shifted offset, tests `checked_add` overflow) |

---

## 4. Reference Rust Implementation & Parameterized MC/DC + BVA Test Suite

```rust
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum TpValidationError {
    FrameTooShort,
    TpFlagNotSet,
    LengthFieldMismatch,
    ZeroLengthSegment,
    SegmentExceedsMaxAligned,
    UnalignedNonFinalSegment,
    MessageSizeOverflow,
}

pub const RPC_HEADER_TAIL_BYTES: usize = 8;
pub const RPC_FULL_HEADER_BYTES: usize = 16;
pub const TP_HEADER_BYTES: usize = 4;
pub const TP_MIN_FRAME_BYTES: usize = RPC_FULL_HEADER_BYTES + TP_HEADER_BYTES; // 20
pub const TP_MAX_SEGMENT_BYTES: u32 = 87 * 16; // 1392
pub const TP_FLAG_MASK: u8 = 0x20;

pub fn validate_tp_segment(
    frame_len: u32,
    header_length_field: u32,
    message_type_byte: u8,
    tp_header_raw: u32,
    current_message_size: u32,
    max_message_size: u32,
) -> Result<(u32, u32, bool), TpValidationError> {
    if (frame_len as usize) < TP_MIN_FRAME_BYTES {
        return Err(TpValidationError::FrameTooShort);
    }
    if (message_type_byte & TP_FLAG_MASK) == 0 {
        return Err(TpValidationError::TpFlagNotSet);
    }
    let expected_frame_len = header_length_field
        .checked_add(RPC_HEADER_TAIL_BYTES as u32)
        .ok_or(TpValidationError::LengthFieldMismatch)?;
    if frame_len != expected_frame_len {
        return Err(TpValidationError::LengthFieldMismatch);
    }

    let segment_size = frame_len - (TP_MIN_FRAME_BYTES as u32);
    if segment_size == 0 {
        return Err(TpValidationError::ZeroLengthSegment);
    }
    if segment_size > TP_MAX_SEGMENT_BYTES {
        return Err(TpValidationError::SegmentExceedsMaxAligned);
    }

    let more_segments = (tp_header_raw & 0x01) != 0;
    let offset = tp_header_raw & 0xFFFF_FFF0;

    // Decision: more_segments && (segment_size % 16 != 0)
    if more_segments && (segment_size % 16 != 0) {
        return Err(TpValidationError::UnalignedNonFinalSegment);
    }

    let new_accumulated = current_message_size
        .checked_add(segment_size)
        .ok_or(TpValidationError::MessageSizeOverflow)?;
    let end_offset = offset
        .checked_add(segment_size)
        .ok_or(TpValidationError::MessageSizeOverflow)?;

    if new_accumulated > max_message_size || end_offset > max_message_size {
        return Err(TpValidationError::MessageSizeOverflow);
    }

    Ok((offset, segment_size, more_segments))
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_tp_segment_bva_and_mcdc_vectors() {
        let max_sz = 4096;
        // 1. Frame length boundary around 20 bytes: 19 -> Err, 20 -> ZeroLengthSegment, 21 -> Ok (if more=0)
        assert_eq!(
            validate_tp_segment(19, 11, 0x20, 0x0, 0, max_sz),
            Err(TpValidationError::FrameTooShort)
        );
        assert_eq!(
            validate_tp_segment(20, 12, 0x20, 0x0, 0, max_sz),
            Err(TpValidationError::ZeroLengthSegment)
        );
        assert_eq!(
            validate_tp_segment(21, 13, 0x20, 0x0, 0, max_sz),
            Ok((0, 1, false))
        );

        // 2. MC/DC independence pairs for `more_segments && (segment_size % 16 != 0)`:
        //    V1: (more=true,  seg_size=15 -> unaligned=true)  => Err(UnalignedNonFinalSegment)
        //    V2: (more=false, seg_size=15 -> unaligned=true)  => Ok (proves `more_segments`)
        //    V3: (more=true,  seg_size=16 -> unaligned=false) => Ok (proves `segment_size % 16 != 0`)
        assert_eq!(
            validate_tp_segment(20 + 15, 12 + 15, 0x20, 0x1, 0, max_sz),
            Err(TpValidationError::UnalignedNonFinalSegment)
        );
        assert_eq!(
            validate_tp_segment(20 + 15, 12 + 15, 0x20, 0x0, 0, max_sz),
            Ok((0, 15, false))
        );
        assert_eq!(
            validate_tp_segment(20 + 16, 12 + 16, 0x20, 0x1, 0, max_sz),
            Ok((0, 16, true))
        );

        // 3. Segment max length boundary around 1392 bytes: 1391 (more=0), 1392 (more=1), 1393 -> Err
        assert_eq!(
            validate_tp_segment(20 + 1391, 12 + 1391, 0x20, 0x0, 0, max_sz),
            Ok((0, 1391, false))
        );
        assert_eq!(
            validate_tp_segment(20 + 1392, 12 + 1392, 0x20, 0x1, 0, max_sz),
            Ok((0, 1392, true))
        );
        assert_eq!(
            validate_tp_segment(20 + 1393, 12 + 1393, 0x20, 0x0, 0, max_sz),
            Err(TpValidationError::SegmentExceedsMaxAligned)
        );

        // 4. Offset & max_message_size boundary + u32::MAX overflow guard:
        assert_eq!(
            validate_tp_segment(20 + 16, 12 + 16, 0x20, 4080, 0, 4096),
            Ok((4080, 16, false))
        );
        assert_eq!(
            validate_tp_segment(20 + 16, 12 + 16, 0x20, 4080, 0, 4095),
            Err(TpValidationError::MessageSizeOverflow)
        );
        assert_eq!(
            validate_tp_segment(20 + 16, 12 + 16, 0x20, 0xFFFF_FFF0, 0, max_sz),
            Err(TpValidationError::MessageSizeOverflow)
        );
    }
}
```
