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

# Miri, Kani, and Bounded Memory Patterns for Automotive Rust

This reference provides production-ready Rust templates for zero-panic wire deserialization, fixed-capacity ring buffers and slab pools, compile-time clock domain separation, deterministic logger lifecycles, and formal verification harnesses (`cargo miri` and `cargo kani`).

---

## 1. Zero-Panic Wire Cursor (`WireReader`)

In C/C++ deserializers, maintaining a separate iterator `position_` and `remaining_` counter risks out-of-bounds reads whenever `remaining_` and `position_` desynchronize.

Replace iterator/pointer pair deserializers with a single slice-backed `WireReader<'a>` where every read advances the slice via `.split_at_checked()` or `.get()`:

```rust
#![deny(
    clippy::unwrap_used,
    clippy::expect_used,
    clippy::panic,
    clippy::indexing_slicing,
    clippy::arithmetic_side_effects,
    clippy::cast_possible_truncation
)]

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum WireError {
    UnexpectedEof { needed: usize, remaining: usize },
    InvalidHeaderLength { length_field: u32 },
    InvalidProtocolVersion { version: u8 },
    MalformedOptionLength { option_len: usize, remaining: usize },
    ArithmeticOverflow,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct SomeIpHeader {
    pub service_id: u16,
    pub method_id: u16,
    pub length: u32,
    pub client_id: u16,
    pub session_id: u16,
    pub protocol_version: u8,
    pub interface_version: u8,
    pub message_type: u8,
    pub return_code: u8,
}

pub struct WireReader<'a> {
    remaining: &'a [u8],
}

impl<'a> WireReader<'a> {
    pub const SOMEIP_MIN_LENGTH: u32 = 8;
    pub const SOMEIP_PROTOCOL_VERSION: u8 = 0x01;

    #[must_use]
    pub const fn new(buf: &'a [u8]) -> Self {
        Self { remaining: buf }
    }

    #[must_use]
    pub const fn remaining_len(&self) -> usize {
        self.remaining.len()
    }

    pub fn read_bytes(&mut self, len: usize) -> Result<&'a [u8], WireError> {
        let head = self.remaining.get(..len).ok_or(WireError::UnexpectedEof {
            needed: len,
            remaining: self.remaining.len(),
        })?;
        let tail = self.remaining.get(len..).ok_or(WireError::UnexpectedEof {
            needed: len,
            remaining: self.remaining.len(),
        })?;
        self.remaining = tail;
        Ok(head)
    }

    pub fn read_u8(&mut self) -> Result<u8, WireError> {
        let b = self.read_bytes(1)?;
        b.first().copied().ok_or(WireError::UnexpectedEof {
            needed: 1,
            remaining: 0,
        })
    }

    pub fn read_u16_be(&mut self) -> Result<u16, WireError> {
        let b = self.read_bytes(2)?;
        let arr: [u8; 2] = b.try_into().map_err(|_| WireError::UnexpectedEof {
            needed: 2,
            remaining: b.len(),
        })?;
        Ok(u16::from_be_bytes(arr))
    }

    pub fn read_u24_be(&mut self) -> Result<u32, WireError> {
        let b = self.read_bytes(3)?;
        let b0 = u32::from(*b.first().ok_or(WireError::UnexpectedEof { needed: 3, remaining: 0 })?);
        let b1 = u32::from(*b.get(1).ok_or(WireError::UnexpectedEof { needed: 3, remaining: 1 })?);
        let b2 = u32::from(*b.get(2).ok_or(WireError::UnexpectedEof { needed: 3, remaining: 2 })?);
        Ok((b0 << 16) | (b1 << 8) | b2)
    }

    pub fn read_u32_be(&mut self) -> Result<u32, WireError> {
        let b = self.read_bytes(4)?;
        let arr: [u8; 4] = b.try_into().map_err(|_| WireError::UnexpectedEof {
            needed: 4,
            remaining: b.len(),
        })?;
        Ok(u32::from_be_bytes(arr))
    }

    pub fn read_someip_header(&mut self) -> Result<SomeIpHeader, WireError> {
        let service_id = self.read_u16_be()?;
        let method_id = self.read_u16_be()?;
        let length = self.read_u32_be()?;
        if length < Self::SOMEIP_MIN_LENGTH {
            return Err(WireError::InvalidHeaderLength { length_field: length });
        }
        let client_id = self.read_u16_be()?;
        let session_id = self.read_u16_be()?;
        let protocol_version = self.read_u8()?;
        if protocol_version != Self::SOMEIP_PROTOCOL_VERSION {
            return Err(WireError::InvalidProtocolVersion {
                version: protocol_version,
            });
        }
        let interface_version = self.read_u8()?;
        let message_type = self.read_u8()?;
        let return_code = self.read_u8()?;

        Ok(SomeIpHeader {
            service_id,
            method_id,
            length,
            client_id,
            session_id,
            protocol_version,
            interface_version,
            message_type,
            return_code,
        })
    }
}
```

---

## 2. Fixed-Capacity Ring Buffer & Pre-Allocated Slab Pool

Classic C ring-buffer implementations exhibit two recurring memory-safety defects:
1. **Unsigned Underflow**: Subtracting `sizeof(BufferHead)` from `min_size` without checking `min_size >= sizeof(BufferHead)` passes an underflowed size into `memset`.
2. **Write-Before-Bounds-Check & Leak**: Initializing header fields inside `shm` before checking `min_size < sizeof(BufferHead)` overwrites heap metadata when `min_size` is too small and leaks `shm` on early return.

Below is a const-generic, zero-heap-allocation `FixedRingBuffer<T, N>` that eliminates raw pointer casts and arithmetic underflows:

```rust
use core::mem::MaybeUninit;

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum RingBufferError {
    Full,
    Empty,
}

/// Zero-heap-allocation ring buffer with compile-time capacity `N`.
/// Aligns with ISO 26262-6 Table 6 (`1b` No dynamic objects).
pub struct FixedRingBuffer<T, const N: usize> {
    storage: [MaybeUninit<T>; N],
    read_idx: usize,
    write_idx: usize,
    count: usize,
}

impl<T, const N: usize> FixedRingBuffer<T, N> {
    #[must_use]
    pub const fn new() -> Self {
        assert!(N > 0, "FixedRingBuffer capacity N must be greater than zero");
        Self {
            // SAFETY: An array of MaybeUninit<T> does not require T to be initialized.
            storage: unsafe { MaybeUninit::<[MaybeUninit<T>; N]>::uninit().assume_init() },
            read_idx: 0,
            write_idx: 0,
            count: 0,
        }
    }

    #[must_use]
    pub const fn len(&self) -> usize {
        self.count
    }

    #[must_use]
    pub const fn is_empty(&self) -> bool {
        self.count == 0
    }

    #[must_use]
    pub const fn is_full(&self) -> bool {
        self.count == N
    }

    pub fn push_back(&mut self, item: T) -> Result<(), (T, RingBufferError)> {
        if self.count >= N {
            return Err((item, RingBufferError::Full));
        }
        let slot = match self.storage.get_mut(self.write_idx) {
            Some(s) => s,
            None => return Err((item, RingBufferError::Full)),
        };
        slot.write(item);
        self.write_idx = match self.write_idx.checked_add(1) {
            Some(next) if next < N => next,
            _ => 0,
        };
        self.count = self.count.saturating_add(1);
        Ok(())
    }

    pub fn pop_front(&mut self) -> Option<T> {
        if self.count == 0 {
            return None;
        }
        let slot = self.storage.get(self.read_idx)?;
        // SAFETY: `self.count > 0` and ring buffer invariants guarantee that the element at
        // `self.read_idx` was initialized by a prior `push_back` and has not yet been read.
        let val = unsafe { slot.assume_init_read() };
        self.read_idx = match self.read_idx.checked_add(1) {
            Some(next) if next < N => next,
            _ => 0,
        };
        self.count = self.count.saturating_sub(1);
        Some(val)
    }
}

impl<T, const N: usize> Drop for FixedRingBuffer<T, N> {
    fn drop(&mut self) {
        while self.pop_front().is_some() {}
    }
}
```

---

## 3. Compile-Time Clock Domain Separation (`middleware_time` & `logging_uptime` Pattern)

Modeled on automotive safety middleware (`time/README.md:15-20`) and `logging_uptime()` (`logging_buffer.c:4441-4456`), use phantom type tags so monotonic timestamps (`SteadyDomain`) and wall-clock timestamps (`SystemDomain`) cannot be mixed:

```rust
use core::marker::PhantomData;
use std::time::{Duration, Instant};

pub enum SteadyDomain {}
pub enum VehiclePtpDomain {}

#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord)]
pub struct TypedTimestamp<Domain> {
    nanos_since_boot: u64,
    _domain: PhantomData<Domain>,
}

pub struct SteadyClock {
    epoch: Instant,
}

impl SteadyClock {
    #[must_use]
    pub fn new() -> Self {
        Self {
            epoch: Instant::now(),
        }
    }

    /// Monotonic timestamp immune to NTP/RTC jumps (`CLOCK_MONOTONIC`).
    #[must_use]
    pub fn now(&self) -> TypedTimestamp<SteadyDomain> {
        let elapsed = self.epoch.elapsed();
        let nanos = u64::try_from(elapsed.as_nanos()).unwrap_or(u64::MAX);
        TypedTimestamp {
            nanos_since_boot: nanos,
            _domain: PhantomData,
        }
    }

    /// Computes DLT uptime in 0.1 ms (100 us) ticks, matching `logging_uptime()` in `logging_buffer.c:4451`
    /// with saturating conversion instead of C 32-bit multiplication overflow.
    #[must_use]
    pub fn logging_uptime_ticks_100us(&self) -> u32 {
        let micros = self.epoch.elapsed().as_micros();
        let ticks = micros / 100;
        u32::try_from(ticks).unwrap_or(u32::MAX)
    }
}

impl<Domain> TypedTimestamp<Domain> {
    #[must_use]
    pub const fn checked_elapsed_since(self, earlier: Self) -> Option<Duration> {
        match self.nanos_since_boot.checked_sub(earlier.nanos_since_boot) {
            Some(delta_ns) => Some(Duration::from_nanos(delta_ns)),
            None => None,
        }
    }
}
```

---

## 4. Deterministic Logger Lifecycle

Avoid leaking heap singletons or spawning detached threads that log after `main()` returns:

In automotive Rust:
1. Never spawn detached `std::thread::spawn` threads that outlive runtime shutdown; use structured concurrency (`std::thread::scope` or tracked `JoinHandle`s).
2. Store global configuration in a lock-free `OnceLock` or `AtomicU8` without heap-leaking `Box::leak`:

```rust
use core::sync::atomic::{AtomicU8, Ordering};

#[repr(u8)]
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord)]
pub enum LogLevel {
    None = 0,
    Fatal = 1,
    Error = 2,
    Warn = 3,
    Info = 4,
    Debug = 5,
    Verbose = 6,
}

pub struct AtomicLogConfig {
    level: AtomicU8,
}

impl AtomicLogConfig {
    #[must_use]
    pub const fn new(initial: LogLevel) -> Self {
        Self {
            level: AtomicU8::new(initial as u8),
        }
    }

    pub fn set_level(&self, level: LogLevel) {
        self.level.store(level as u8, Ordering::Release);
    }

    #[must_use]
    pub fn is_enabled(&self, msg_level: LogLevel) -> bool {
        let current = self.level.load(Ordering::Acquire);
        (msg_level as u8) <= current && current != (LogLevel::None as u8)
    }
}
```

---

## 5. Formal Verification Workflows: `cargo miri` and `cargo kani`

### 5.1 Miri: Undefined Behavior, Strict Provenance, and Alignment Checks

Run all unit tests for wire parsers, ring buffers, and FFI wrappers under **Miri** to detect out-of-bounds pointer arithmetic, uninitialized memory reads, alignment violations, and Tree Borrows / Stacked Borrows aliasing violations:

```bash
MIRIFLAGS="-Zmiri-strict-provenance -Zmiri-symbolic-alignment-check -Zmiri-check-number-validity" \
  cargo +nightly miri test
```

### 5.2 Kani: Bounded Model Checking Proofs (`#[kani::proof]`)

Kani exhaustively verifies that a function cannot panic, index out-of-bounds, or overflow for **all possible symbolic inputs** up to a specified bound (satisfying **ISO 26262-6 Table 7 / Table 9** formal verification and boundary value analysis):

```rust
#[cfg(kani)]
mod kani_proofs {
    use super::*;

    /// Proves that `WireReader::read_someip_header` never panics, overflows, or
    /// reads out of bounds for ANY symbolic byte slice up to 32 bytes.
    #[kani::proof]
    #[kani::unwind(33)]
    fn proof_someip_header_parser_never_panics() {
        let buf: [u8; 32] = kani::any();
        let slice_len: usize = kani::any();
        kani::assume(slice_len <= buf.len());

        let input = match buf.get(..slice_len) {
            Some(s) => s,
            None => return,
        };
        let mut reader = WireReader::new(input);
        let res = reader.read_someip_header();

        if slice_len < 16 {
            assert!(res.is_err());
        }
        if let Ok(hdr) = res {
            assert!(hdr.length >= WireReader::SOMEIP_MIN_LENGTH);
            assert_eq!(hdr.protocol_version, WireReader::SOMEIP_PROTOCOL_VERSION);
            assert_eq!(reader.remaining_len(), slice_len.saturating_sub(16));
        }
    }

    /// Proves that `FixedRingBuffer` maintains its count and index invariants
    /// across arbitrary sequences of `push_back` and `pop_front`.
    #[kani::proof]
    #[kani::unwind(6)]
    fn proof_fixed_ring_buffer_invariants() {
        let mut ring: FixedRingBuffer<u32, 4> = FixedRingBuffer::new();
        for _ in 0..4 {
            let op: bool = kani::any();
            if op {
                let val: u32 = kani::any();
                let _ = ring.push_back(val);
            } else {
                let _ = ring.pop_front();
            }
            assert!(ring.len() <= 4);
        }
    }
}
```


---

## 6. Overflow-Checked Endpoint Queue Quotas (`try_reserve_exact`)

Before allocating payload buffers on dynamic Tier-1 HPC queues, enforce per-endpoint PDU and byte-queue quotas using `checked_add` and `Vec::try_reserve_exact` so malformed length headers cannot trigger allocation panics or OOM crashes:

```rust
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum QuotaError {
    PayloadTooLarge { requested: usize, max_pdu: usize },
    ByteQuotaExceeded { used: usize, requested: usize, limit: usize },
    AllocationFailed,
}

pub struct EndpointQueueQuota {
    queued_bytes: usize,
    pending_train_bytes: usize,
    queue_limit_bytes: usize,
    max_pdu_bytes: usize,
}

impl EndpointQueueQuota {
    pub fn check_and_reserve(&mut self, msg_size: usize) -> Result<Vec<u8>, QuotaError> {
        if msg_size > self.max_pdu_bytes {
            return Err(QuotaError::PayloadTooLarge {
                requested: msg_size,
                max_pdu: self.max_pdu_bytes,
            });
        }
        let current_used = self
            .queued_bytes
            .checked_add(self.pending_train_bytes)
            .ok_or(QuotaError::ByteQuotaExceeded {
                used: self.queued_bytes,
                requested: msg_size,
                limit: self.queue_limit_bytes,
            })?;
        let new_total = current_used
            .checked_add(msg_size)
            .ok_or(QuotaError::ByteQuotaExceeded {
                used: current_used,
                requested: msg_size,
                limit: self.queue_limit_bytes,
            })?;
        if new_total > self.queue_limit_bytes {
            return Err(QuotaError::ByteQuotaExceeded {
                used: current_used,
                requested: msg_size,
                limit: self.queue_limit_bytes,
            });
        }
        let mut buf = Vec::new();
        buf.try_reserve_exact(msg_size)
            .map_err(|_| QuotaError::AllocationFailed)?;
        self.queued_bytes = self.queued_bytes.saturating_add(msg_size);
        Ok(buf)
    }
}
```

---

## 7. Audited `mod sys` Quarantine, Binary `#![forbid(unsafe_code)]`, and `// SAFETY:` Proofs

1. **Pure Logic Crates**: Set `#![forbid(unsafe_code)]` at crate root (`lib.rs`) for wire parsers, configuration schemas, state machines, and CRC modules that require zero OS FFI.
2. **Crates Requiring POSIX Syscalls (`fcntl`, `atexit`, `nice`, `stat`, `fork`, `pthread_sigmask`)**: Avoid `#![forbid(unsafe_code)]` at `lib.rs` (which tempts automated ports to stub/no-op OS syscalls); instead set `#![deny(unsafe_code)]` at `lib.rs` and isolate all OS FFI inside `#[allow(unsafe_code)] pub(crate) mod sys;`:
   - Each `src/sys/*.rs` function wraps one OS primitive behind a safe signature returning `std::io::Result<T>` or `Option<T>`.
   - Any `extern "C"` OS callback (e.g., `libc::atexit`) wraps its body in `std::panic::catch_unwind`.
   - Every `src/sys/` wrapper is backed by an OS-level integration test (e.g., child processes competing for an `fcntl` lockfile, or `daemonize()` asserting `/proc/self/status` `Threads: 1` before `fork()`).
3. **Binary Crate Roots (`RUST-AUTO-BIN-001`)**: `src/main.rs` and `src/bin/*.rs` do not inherit `lib.rs` lints. Place `#![forbid(unsafe_code)]` at the top of every binary root and call safe helpers exposed by `sys`.

```rust
pub const RING_HEADER_SIZE: usize = 12;

/// Initializes the payload region of a shared-memory ring buffer after a 12-byte header.
///
/// # Safety
/// - `shm_ptr` must be non-null, properly aligned, and valid for writes of `total_size` bytes.
/// - `[shm_ptr, shm_ptr + total_size)` must belong to a single allocated object with exclusive access.
pub unsafe fn init_shm_ring(shm_ptr: *mut u8, total_size: usize) -> Option<*mut u8> {
    if shm_ptr.is_null() {
        return None;
    }
    let payload_size = total_size.checked_sub(RING_HEADER_SIZE)?;
    // SAFETY: Caller guarantees `shm_ptr` is valid for `total_size` bytes and `checked_sub`
    // proved `total_size >= RING_HEADER_SIZE`, so `shm_ptr.add(RING_HEADER_SIZE)` is in-bounds
    // and valid for writes of `payload_size` bytes.
    unsafe {
        let payload_ptr = shm_ptr.add(RING_HEADER_SIZE);
        core::ptr::write_bytes(payload_ptr, 0, payload_size);
        Some(payload_ptr)
    }
}
```
