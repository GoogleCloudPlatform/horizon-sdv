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

# Public-Domain CRC Catalog & `const fn` Lookup-Table Implementations

## 1. Internationally Standardized Public-Domain Polynomials

Select end-to-end (E2E) communication CRCs exclusively from open, internationally standardized telecommunications and automotive specifications:

| Polynomial (Normal) | Polynomial (Reflected) | Width | Public Standard | Typical Frame Scope |
|---|---|---|---|---|
| `0x1D` | `0xB8` | 8-bit | **SAE J1850** (1995 automotive standard) | Compact CAN/LIN-bridged actuator commands ($\le 16$ B) |
| `0x1021` | `0x8408` | 16-bit | **ITU-T Rec. X.25 / V.41 (CCITT-FALSE)** | Medium RPC requests & cyclic sensor notifications ($\le 256$ B) |
| `0x04C11DB7` | `0xEDB88320` | 32-bit | **IEEE 802.3 / ITU-T V.42 / ISO 3309** | Full Ethernet MTU UDP/TCP frames ($\le 4,096$ B) |
| `0x1EDC6F41` | `0x82F63B78` | 32-bit | **IETF RFC 3720 (iSCSI) / RFC 4960 (SCTP) Castagnoli** | High-integrity ASIL-B/D frames ($HD = 6$ up to 512 B; hardware `crc32c` on x86 SSE4.2 & ARMv8.1-A) |

---

## 2. Rocksoft Parameterization & Verification Matrix

Verify every CRC implementation against the canonical 9-byte ASCII test string `"123456789"` (`[0x31..0x39]`) and an empty slice `b""` at compile time and in unit tests:

| Algorithm | Width | Normal `Poly` | Reflected `Poly` | `Init` | `RefIn` | `RefOut` | `XorOut` | `Check("")` | `Check("123456789")` | Magic Residue |
|---|---|---|---|---|---|---|---|---|---|---|
| **SAE-J1850 CRC-8** | 8 | `0x1D` | `0xB8` | `0xFF` | `false` | `false` | `0xFF` | `0x00` | **`0x4B`** | `0xC4` |
| **ITU-T CCITT-FALSE CRC-16** | 16 | `0x1021` | `0x8408` | `0xFFFF` | `false` | `false` | `0x0000` | `0xFFFF` | **`0x29B1`** | `0x0000` |
| **IEEE 802.3 CRC-32** | 32 | `0x04C11DB7` | `0xEDB88320` | `0xFFFFFFFF` | `true` | `true` | `0xFFFFFFFF` | `0x00000000` | **`0xCBF43926`** | `0xDEBB20E3` |
| **Castagnoli CRC-32C** | 32 | `0x1EDC6F41` | `0x82F63B78` | `0xFFFFFFFF` | `true` | `true` | `0xFFFFFFFF` | `0x00000000` | **`0xE3069283`** | `0xB798B438` |

---

## 3. Hamming Distance ($HD$) vs. Payload Size Selection Guide

Per Koopman & Chakravarty (*"Cyclic Redundancy Code (CRC) Polynomial Selection For Embedded Networks"*, DSN 2004), select the CRC width and polynomial according to the maximum protected data length (implicit pseudo-header + upper wire header + sequence/freshness fields + application payload):

| Algorithm | $HD = 6$ Max Data Length | $HD = 4$ Max Data Length | Burst Error Detection | Target Automotive Use Case |
|---|---|---|---|---|
| **SAE-J1850 CRC-8** (`0x1D`) | N/A ($HD = 4$ up to 11 B) | 11 bytes (93 bits) | 100% of bursts $\le 8$ bits | Ultra-compact CAN/LIN actuator commands |
| **ITU-T CCITT-FALSE CRC-16** (`0x1021`) | N/A ($HD = 4$ up to 4,094 B) | 4,094 bytes (32,751 bits) | 100% of bursts $\le 16$ bits | Standard UDP RPC requests and cyclic sensor notifications ($\le 256$ B) |
| **IEEE 802.3 CRC-32** (`0x04C11DB7`) | 33 bytes (268 bits) | 11,450 bytes (91,607 bits) | 100% of bursts $\le 32$ bits | Full Ethernet MTU UDP/TCP payloads ($\le 1,400$ B) |
| **Castagnoli CRC-32C** (`0x1EDC6F41`) | **512 bytes (4,096 bits)** | 65,535+ bytes | 100% of bursts $\le 32$ bits | ASIL-B/D radar/camera/chassis frames & segmented payloads (detects all 5-bit errors up to 512 B) |

---

## 4. `const fn` 256-Entry Table-Driven `#![no_std]` Rust Implementations

Generate all four 256-entry lookup tables at compile time using `const fn` with compile-time Rocksoft `assert!` checks:

```rust
#![cfg_attr(not(test), no_std)]
#![forbid(unsafe_code)]

const fn build_crc8_sae_j1850_table() -> [u8; 256] {
    let mut table = [0u8; 256];
    let mut i = 0usize;
    while i < 256 {
        let mut crc = i as u8;
        let mut bit = 0;
        while bit < 8 {
            crc = if (crc & 0x80) != 0 { (crc << 1) ^ 0x1D } else { crc << 1 };
            bit += 1;
        }
        table[i] = crc;
        i += 1;
    }
    table
}

pub static CRC8_SAE_J1850_TABLE: [u8; 256] = build_crc8_sae_j1850_table();

#[inline]
pub const fn crc8_sae_j1850(data: &[u8]) -> u8 {
    let mut crc: u8 = 0xFF;
    let mut i = 0usize;
    while i < data.len() {
        crc = CRC8_SAE_J1850_TABLE[(crc ^ data[i]) as usize];
        i += 1;
    }
    crc ^ 0xFF
}

const fn build_crc16_ccitt_false_table() -> [u16; 256] {
    let mut table = [0u16; 256];
    let mut i = 0usize;
    while i < 256 {
        let mut crc = (i as u16) << 8;
        let mut bit = 0;
        while bit < 8 {
            crc = if (crc & 0x8000) != 0 { (crc << 1) ^ 0x1021 } else { crc << 1 };
            bit += 1;
        }
        table[i] = crc;
        i += 1;
    }
    table
}

pub static CRC16_CCITT_FALSE_TABLE: [u16; 256] = build_crc16_ccitt_false_table();

#[inline]
pub const fn crc16_ccitt_false(data: &[u8]) -> u16 {
    let mut crc: u16 = 0xFFFF;
    let mut i = 0usize;
    while i < data.len() {
        let idx = (((crc >> 8) as u8) ^ data[i]) as usize;
        crc = (crc << 8) ^ CRC16_CCITT_FALSE_TABLE[idx];
        i += 1;
    }
    crc
}

const fn build_crc32_ieee8023_table() -> [u32; 256] {
    let mut table = [0u32; 256];
    let mut i = 0usize;
    while i < 256 {
        let mut crc = i as u32;
        let mut bit = 0;
        while bit < 8 {
            crc = if (crc & 1) != 0 { (crc >> 1) ^ 0xEDB8_8320 } else { crc >> 1 };
            bit += 1;
        }
        table[i] = crc;
        i += 1;
    }
    table
}

pub static CRC32_IEEE8023_TABLE: [u32; 256] = build_crc32_ieee8023_table();

#[inline]
pub const fn crc32_ieee8023(data: &[u8]) -> u32 {
    let mut crc: u32 = 0xFFFF_FFFF;
    let mut i = 0usize;
    while i < data.len() {
        let idx = ((crc as u8) ^ data[i]) as usize;
        crc = (crc >> 8) ^ CRC32_IEEE8023_TABLE[idx];
        i += 1;
    }
    crc ^ 0xFFFF_FFFF
}

const fn build_crc32c_castagnoli_table() -> [u32; 256] {
    let mut table = [0u32; 256];
    let mut i = 0usize;
    while i < 256 {
        let mut crc = i as u32;
        let mut bit = 0;
        while bit < 8 {
            crc = if (crc & 1) != 0 { (crc >> 1) ^ 0x82F6_3B78 } else { crc >> 1 };
            bit += 1;
        }
        table[i] = crc;
        i += 1;
    }
    table
}

pub static CRC32C_CASTAGNOLI_TABLE: [u32; 256] = build_crc32c_castagnoli_table();

#[inline]
pub const fn crc32c_castagnoli(data: &[u8]) -> u32 {
    let mut crc: u32 = 0xFFFF_FFFF;
    let mut i = 0usize;
    while i < data.len() {
        let idx = ((crc as u8) ^ data[i]) as usize;
        crc = (crc >> 8) ^ CRC32C_CASTAGNOLI_TABLE[idx];
        i += 1;
    }
    crc ^ 0xFFFF_FFFF
}

const _: () = {
    assert!(crc8_sae_j1850(b"123456789") == 0x4B);
    assert!(crc16_ccitt_false(b"123456789") == 0x29B1);
    assert!(crc32_ieee8023(b"123456789") == 0xCBF4_3926);
    assert!(crc32c_castagnoli(b"123456789") == 0xE306_9283);
};
```
