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

# ISO 26262-6:2018 Clause 7.4.6 & Annex D — Freedom from Interference (FFI) Engineering Reference

When software elements of different ASIL ratings (for example, QM infotainment/diagnostics and ASIL-B chassis/steering control) co-exist within the same ECU, microprocessor, or middleware routing daemon, **ISO 26262-6:2018 Clause 7.4.5** mandates that all elements be treated according to the highest ASIL **unless** the software architectural design establishes **Freedom from Interference (FFI)** in accordance with **Clause 7.4.6** and **Annex D**.

This reference maps every fault category in ISO 26262-6:2018 Annex D to concrete failure modes in multi-client automotive middleware and provides Rust isolation mechanisms across all three fault domains:
1. **Spatial / Memory Isolation (Annex D.2.2)**
2. **Temporal / Timing & Execution Isolation (Annex D.2.1)**
3. **Communication / Exchange of Information Isolation (Annex D.2.3 & D.2.4)**

---

## 1. ISO 26262-6 Annex D Fault Matrix & Middleware Mapping

| Annex D Clause | Fault Domain | Fault Effect in ISO 26262-6 Annex D | Typical C/C++ Middleware Failure Mode | Rust Isolation Mechanism |
|---|---|---|---|---|
| **D.2.1** | Timing & Execution | Blocking of execution (`deadlock` / `livelock` / thread sleep) | Fatal error macros executing `std::this_thread::sleep_for(...)` before `std::abort()` on shared worker threads. | Poll-budgeted async tasks (`PollBudgetedFuture`), lock-free or non-blocking actor channels, zero blocking sleeps on executor threads. |
| **D.2.1** | Timing & Execution | Incorrect allocation of execution time (CPU starvation by QM task) | Shared `boost::asio::io_context` worker threads processing high-rate QM UDP packets ahead of ASIL-B RPCs without priority preemption. | Priority-segregated executors (`asil_runtime` pinned to high-priority RT threads vs. `qm_runtime`) and biased channel polling. |
| **D.2.1** | Timing & Execution | Unintended termination or hang of shared execution environment | Protocol timeouts invoking `std::abort()` or intentional null-pointer traps (`int* p = NULL; *p = 0;`) inside shared libraries. | Per-client / per-service fault containment state machine; Logical Program Sequence Monitoring (alive counter + checkpoint watchdog). |
| **D.2.2** | Memory (Spatial) | Corruption of content (unauthorized write to ASIL memory) | Shared mutable buffers across C/C++ plugins or writable shared memory segments mapped into untrusted consumers. | Rust safe ownership/borrowing + OS MMU read-only `mmap` (`PROT_READ`) for subscribers. |
| **D.2.2** | Memory (Spatial) | Heap / stack exhaustion (OOM starvation of ASIL partition) | Unbounded nested `std::map` reassembly state + eager `vector::resize(header + offset)` on spoofed segments, or `MAX_MESSAGE_SIZE = 0` (unlimited) defaults. | Pre-allocated static slab pools (`PartitionedTpArena`) with strict per-partition (`QM` vs. `ASIL`) slot reservations and `NonZeroUsize` message size caps. |
| **D.2.3 / D.2.4** | Exchange of Information | Queue exhaustion, head-of-line blocking, or masquerading | Endpoint queues defaulting to `QUEUE_SIZE_UNLIMITED` (`0`), allowing a slow or chattering QM client to exhaust daemon memory. | Non-zero per-client byte/frame quotas, token-bucket ingress rate limiters, and monotonic debounce filters. |

---

## 2. Domain 1: Spatial / Memory Isolation (ISO 26262-6 Annex D.2.2)

### 2.1 Partition-Aware Segmented Reassembly Arena
In naive C++ transport-protocol reassemblers:
- Active reassembly sessions are stored in an unbounded nested `std::map` keyed by peer IP, port, and 64-bit message ID.
- When an out-of-order segment arrives with `offset > 0`, the buffer eagerly resizes to `HEADER_SIZE + offset`.
- A faulty QM node sending 64 spoofed first-seen frames with `offset = 1_048_560` across distinct message IDs allocates `> 64 MiB` of heap memory before the cleanup timer expires.

In an ASIL-B Rust routing daemon, replace dynamic heap maps with a **pre-allocated, partition-aware slab arena** using a fixed 64-bit segment bitmap:

```rust
use std::time::{Duration, Instant};

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum PartitionCriticality {
    Qm,
    AsilB,
    AsilD,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct TpReassemblyKey {
    pub peer_ip: u32,
    pub peer_port: u16,
    pub service_id: u16,
    pub instance_id: u16,
    pub method_id: u16,
    pub client_id: u16,
    pub session_id: u16,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum SpatialIsolationError {
    AlignmentViolation,
    OffsetOutOfBounds,
    SegmentCountExceeded,
    OverlappingOrDuplicateSegment,
    QmPartitionQuotaExhausted,
    ArenaExhausted,
}

pub struct TpSlot<const MAX_PDU: usize, const SEGMENT_STRIDE: usize> {
    pub in_use: bool,
    pub criticality: PartitionCriticality,
    pub key: TpReassemblyKey,
    pub started_at: Option<Instant>,
    pub received_mask: u64,
    pub expected_mask: Option<u64>,
    pub total_len: usize,
    pub buffer: [u8; MAX_PDU],
}

impl<const MAX_PDU: usize, const SEGMENT_STRIDE: usize> TpSlot<MAX_PDU, SEGMENT_STRIDE> {
    pub const fn new() -> Self {
        Self {
            in_use: false,
            criticality: PartitionCriticality::Qm,
            key: TpReassemblyKey {
                peer_ip: 0,
                peer_port: 0,
                service_id: 0,
                instance_id: 0,
                method_id: 0,
                client_id: 0,
                session_id: 0,
            },
            started_at: None,
            received_mask: 0,
            expected_mask: None,
            total_len: 0,
            buffer: [0u8; MAX_PDU],
        }
    }

    pub fn write_segment(
        &mut self,
        offset: usize,
        more_segments: bool,
        payload: &[u8],
    ) -> Result<Option<&[u8]>, SpatialIsolationError> {
        // Segmented transport offsets must be multiples of 16 bytes
        if offset % 16 != 0 || (more_segments && payload.len() % SEGMENT_STRIDE != 0) {
            return Err(SpatialIsolationError::AlignmentViolation);
        }
        let end = offset
            .checked_add(payload.len())
            .ok_or(SpatialIsolationError::OffsetOutOfBounds)?;
        if end > MAX_PDU || payload.is_empty() {
            return Err(SpatialIsolationError::OffsetOutOfBounds);
        }

        let seg_idx = offset / SEGMENT_STRIDE;
        if seg_idx >= 64 {
            return Err(SpatialIsolationError::SegmentCountExceeded);
        }
        let bit = 1u64 << seg_idx;
        if (self.received_mask & bit) != 0 {
            return Err(SpatialIsolationError::OverlappingOrDuplicateSegment);
        }

        self.buffer[offset..end].copy_from_slice(payload);
        self.received_mask |= bit;
        if end > self.total_len {
            self.total_len = end;
        }

        if !more_segments {
            let final_mask = if seg_idx == 63 { u64::MAX } else { (1u64 << (seg_idx + 1)) - 1 };
            self.expected_mask = Some(final_mask);
        }

        if let Some(expected) = self.expected_mask {
            if self.received_mask == expected {
                self.in_use = false;
                self.started_at = None;
                return Ok(Some(&self.buffer[..self.total_len]));
            }
        }
        Ok(None)
    }
}

pub struct PartitionedTpArena<const N_SLOTS: usize, const MAX_PDU: usize, const STRIDE: usize> {
    slots: [TpSlot<MAX_PDU, STRIDE>; N_SLOTS],
    qm_max_slots: usize, // Strictly < N_SLOTS so ASIL slots are guaranteed available
    reassembly_timeout: Duration,
}

impl<const N_SLOTS: usize, const MAX_PDU: usize, const STRIDE: usize>
    PartitionedTpArena<N_SLOTS, MAX_PDU, STRIDE>
{
    pub fn evict_expired(&mut self, now: Instant) -> usize {
        let mut evicted = 0;
        for slot in &mut self.slots {
            if slot.in_use
                && slot
                    .started_at
                    .is_some_and(|start| now.saturating_duration_since(start) >= self.reassembly_timeout)
            {
                slot.in_use = false;
                slot.started_at = None;
                slot.received_mask = 0;
                slot.expected_mask = None;
                evicted += 1;
            }
        }
        evicted
    }

    pub fn process_segment(
        &mut self,
        now: Instant,
        criticality: PartitionCriticality,
        key: TpReassemblyKey,
        offset: usize,
        more_segments: bool,
        payload: &[u8],
    ) -> Result<Option<&[u8]>, SpatialIsolationError> {
        self.evict_expired(now);

        // 1. Check if an active slot already exists for this TpReassemblyKey
        if let Some(idx) = self.slots.iter().position(|s| s.in_use && s.key == key) {
            return self.slots[idx].write_segment(offset, more_segments, payload);
        }

        // 2. Enforce spatial partition quota: QM cannot consume reserved ASIL slots
        let current_qm = self
            .slots
            .iter()
            .filter(|s| s.in_use && s.criticality == PartitionCriticality::Qm)
            .count();
        if criticality == PartitionCriticality::Qm && current_qm >= self.qm_max_slots {
            return Err(SpatialIsolationError::QmPartitionQuotaExhausted);
        }

        // 3. Allocate free slot from fixed pool (zero heap allocation)
        let free_idx = self
            .slots
            .iter()
            .position(|s| !s.in_use)
            .ok_or(SpatialIsolationError::ArenaExhausted)?;
        let slot = &mut self.slots[free_idx];
        slot.in_use = true;
        slot.criticality = criticality;
        slot.key = key;
        slot.started_at = Some(now);
        slot.received_mask = 0;
        slot.expected_mask = None;
        slot.total_len = 0;
        slot.write_segment(offset, more_segments, payload)
    }
}
```

### 2.2 Read-Only Shared Memory Isolation (Producer / Multi-Consumer IPC)
When zero-copy shared memory IPC is used between a Skeleton (Producer) and multiple Proxies (Consumers) of differing ASIL levels:
1. **Control vs. Data Plane Separation**: Separate the immutable sample data segment (mapped `PROT_READ` in all Consumer processes) from the per-consumer subscription control segment.
2. **Pre-Allocated Fixed Sample Containers**: Allocate all sample slots during initialization; forbid heap allocation during steady-state event transmission.

---

## 3. Domain 2: Temporal / Timing & Execution Isolation (ISO 26262-6 Annex D.2.1)

### 3.1 Poll-Budgeted Futures (Preventing QM Task Monopoly)
In cooperative async Rust (`tokio` or `mio` event loops), a single future that loops over a flooded QM UDP socket without yielding can starve ASIL-B tasks on the same worker thread. Wrap untrusted or QM futures in a `PollBudgetedFuture` that enforces a per-poll execution time budget (`WCET` slice):

```rust
use core::future::Future;
use core::pin::Pin;
use core::task::{Context, Poll};
use std::time::{Duration, Instant};

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct WcetViolation {
    pub task_id: u16,
    pub elapsed: Duration,
    pub budget: Duration,
}

pub struct PollBudgetedFuture<F> {
    inner: F,
    task_id: u16,
    max_poll_slice: Duration,
    total_wcet_budget: Duration,
    accumulated_time: Duration,
}

impl<F: Future> PollBudgetedFuture<F> {
    pub const fn new(
        inner: F,
        task_id: u16,
        max_poll_slice: Duration,
        total_wcet_budget: Duration,
    ) -> Self {
        Self {
            inner,
            task_id,
            max_poll_slice,
            total_wcet_budget,
            accumulated_time: Duration::ZERO,
        }
    }
}

impl<F: Future> Future for PollBudgetedFuture<F> {
    type Output = Result<F::Output, WcetViolation>;

    fn poll(self: Pin<&mut Self>, cx: &mut Context<'_>) -> Poll<Self::Output> {
        // SAFETY: `inner` is structurally pinned; other fields are Unpin scalars.
        let this = unsafe { self.get_unchecked_mut() };
        let inner_pin = unsafe { Pin::new_unchecked(&mut this.inner) };

        let start = Instant::now();
        let poll_res = inner_pin.poll(cx);
        let elapsed = start.elapsed();
        this.accumulated_time = this.accumulated_time.saturating_add(elapsed);

        if elapsed > this.max_poll_slice || this.accumulated_time > this.total_wcet_budget {
            return Poll::Ready(Err(WcetViolation {
                task_id: this.task_id,
                elapsed: this.accumulated_time,
                budget: this.total_wcet_budget,
            }));
        }

        match poll_res {
            Poll::Ready(val) => Poll::Ready(Ok(val)),
            Poll::Pending => Poll::Pending,
        }
    }
}
```

### 3.2 Logical Program Sequence & Alive Watchdog
When supervising local IPC clients, track ping/pong timestamps with a monotonic clock (`Instant::now()`) and disconnect unresponsive clients individually without terminating the central router. Under ISO 26262-6 Clause 7.4.12 & Annex D.2.1, combine:
1. **Alive Monitoring**: Periodic monotonic heartbeat counts within `[min_margin, max_margin]` per window.
2. **Deadline Monitoring**: Bounding the elapsed `Instant` between `SendPing` (`checkpoint_start`) and `OnPong` (`checkpoint_end`).
3. **Logical Program Sequence Monitoring**: Hashing ordered checkpoint transitions (`IngressVerify -> PolicyCheck -> RouteDispatch -> EgressComplete`) so jumped or skipped security/E2E stages trigger deterministic partition isolation.

```rust
use std::time::{Duration, Instant};

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum WatchdogFault {
    DeadlineExceeded { client_id: u16, elapsed: Duration },
    InvalidCheckpointSequence { expected_hash: u32, actual_hash: u32 },
}

pub struct PartitionWatchdog {
    expected_final_hash: u32,
    rolling_hash: u32,
    cycle_started_at: Option<Instant>,
    max_cycle_duration: Duration,
}

impl PartitionWatchdog {
    pub const fn new(expected_final_hash: u32, max_cycle_duration: Duration) -> Self {
        Self {
            expected_final_hash,
            rolling_hash: 0x811C_9DC5, // FNV-1a 32-bit offset basis
            cycle_started_at: None,
            max_cycle_duration,
        }
    }

    pub fn begin_cycle(&mut self, now: Instant) {
        self.rolling_hash = 0x811C_9DC5;
        self.cycle_started_at = Some(now);
    }

    pub fn record_checkpoint(&mut self, checkpoint_id: u32) {
        self.rolling_hash ^= checkpoint_id;
        self.rolling_hash = self.rolling_hash.wrapping_mul(0x0100_0193);
    }

    pub fn complete_cycle(&mut self, client_id: u16, now: Instant) -> Result<(), WatchdogFault> {
        let start = self.cycle_started_at.take().unwrap_or(now);
        let elapsed = now.saturating_duration_since(start);
        if elapsed > self.max_cycle_duration {
            return Err(WatchdogFault::DeadlineExceeded { client_id, elapsed });
        }
        if self.rolling_hash != self.expected_final_hash {
            return Err(WatchdogFault::InvalidCheckpointSequence {
                expected_hash: self.expected_final_hash,
                actual_hash: self.rolling_hash,
            });
        }
        Ok(())
    }
}
```

---

## 4. Domain 3: Communication / Exchange Isolation (ISO 26262-6 Annex D.2.3 & D.2.4)

### 4.1 Per-Client Byte Quotas & Token-Bucket Ingress Rate Limiter
Replace `QUEUE_SIZE_UNLIMITED` (`0`) defaults with an overflow-safe **Per-Client Byte Quota + Token-Bucket Rate Limiter + Monotonic Debounce Filter**:

```rust
use core::num::NonZeroUsize;
use std::time::{Duration, Instant};

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum IngressQuotaRejection {
    MessageExceedsMaxPdu { size: usize, max_pdu: usize },
    ByteQueueQuotaExceeded { queued: usize, incoming: usize, limit: usize },
    RateLimitExceeded { retry_after: Duration },
    DebounceIntervalNotElapsed { remaining: Duration },
}

pub struct ClientExchangeLimiter {
    max_pdu_bytes: NonZeroUsize,
    max_queue_bytes: NonZeroUsize,
    queued_bytes: usize,
    // Token bucket parameters (tokens in bytes)
    bucket_capacity_bytes: usize,
    tokens_bytes: usize,
    refill_bytes_per_ms: usize,
    last_refill: Instant,
    // Monotonic debounce filter
    min_debounce_interval: Duration,
    last_accepted: Option<Instant>,
}

impl ClientExchangeLimiter {
    pub fn new(
        max_pdu_bytes: NonZeroUsize,
        max_queue_bytes: NonZeroUsize,
        refill_bytes_per_ms: usize,
        min_debounce_interval: Duration,
        now: Instant,
    ) -> Self {
        let cap = max_queue_bytes.get();
        Self {
            max_pdu_bytes,
            max_queue_bytes,
            queued_bytes: 0,
            bucket_capacity_bytes: cap,
            tokens_bytes: cap,
            refill_bytes_per_ms,
            last_refill: now,
            min_debounce_interval,
            last_accepted: None,
        }
    }

    pub fn try_admit_frame(
        &mut self,
        frame_bytes: usize,
        now: Instant,
    ) -> Result<(), IngressQuotaRejection> {
        // 1. Non-zero maximum PDU size check
        if frame_bytes > self.max_pdu_bytes.get() {
            return Err(IngressQuotaRejection::MessageExceedsMaxPdu {
                size: frame_bytes,
                max_pdu: self.max_pdu_bytes.get(),
            });
        }

        // 2. Monotonic debounce check
        if let Some(prev) = self.last_accepted {
            let elapsed = now.saturating_duration_since(prev);
            if elapsed < self.min_debounce_interval {
                return Err(IngressQuotaRejection::DebounceIntervalNotElapsed {
                    remaining: self.min_debounce_interval.saturating_sub(elapsed),
                });
            }
        }

        // 3. Overflow-safe byte queue limit check
        let limit = self.max_queue_bytes.get();
        if limit.saturating_sub(self.queued_bytes) < frame_bytes {
            return Err(IngressQuotaRejection::ByteQueueQuotaExceeded {
                queued: self.queued_bytes,
                incoming: frame_bytes,
                limit,
            });
        }

        // 4. Token-bucket rate refill & consumption
        let elapsed_ms = now.saturating_duration_since(self.last_refill).as_millis() as usize;
        if elapsed_ms > 0 {
            let added = elapsed_ms.saturating_mul(self.refill_bytes_per_ms);
            self.tokens_bytes = self.tokens_bytes.saturating_add(added).min(self.bucket_capacity_bytes);
            self.last_refill = now;
        }
        if self.tokens_bytes < frame_bytes {
            let deficit = frame_bytes - self.tokens_bytes;
            let wait_ms = (deficit / self.refill_bytes_per_ms.max(1)).max(1) as u64;
            return Err(IngressQuotaRejection::RateLimitExceeded {
                retry_after: Duration::from_millis(wait_ms),
            });
        }

        self.tokens_bytes -= frame_bytes;
        self.queued_bytes += frame_bytes;
        self.last_accepted = Some(now);
        Ok(())
    }

    pub fn on_frame_drained(&mut self, frame_bytes: usize) {
        self.queued_bytes = self.queued_bytes.saturating_sub(frame_bytes);
    }
}
```
