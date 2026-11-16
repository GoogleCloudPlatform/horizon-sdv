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

# ISO 26262-6 Clause 7 Inductive Software FMEA (SW-FMEA): HAZOP Guidewords & Rust Middleware Mapping

This reference provides an inductive Software Failure Mode and Effects Analysis (SW-FMEA) methodology aligned with **ISO 26262-6:2018 Clause 7.4.10 (Table 4)** and **ISO 26262-9:2018 Clause 8** for automotive Rust middleware crates.

---

## 1. HAZOP Guideword Taxonomy for Software Architectural Boundaries

ISO 26262-6 Clause 7.4.10 requires safety analysis at the **Software Architectural Design** level to identify failure modes that could violate a technical or assumed safety requirement, establish root causes, and specify **Prevention Mechanisms** and **Detection / Mitigation Mechanisms**.

Applying 5 structured HAZOP guidewords to every Rust public function/trait ensures systematic coverage:

| HAZOP Guideword | ISO 26262-6 / Annex D Fault Class | Question Asked at Each `pub fn` / `pub trait` Boundary | Primary Rust Prevention Mechanism | Primary Rust Detection / Mitigation Mechanism |
|---|---|---|---|---|
| **`LossOfFunction`** | Omission / Silent Drop / Unavailability | What happens if this function fails to execute, silently drops an outbound/inbound frame, or never invokes a registered callback? | `#[must_use] Result<T, E>` (no `void` returns); exhaustive state machine transitions. | Explicit error variant (`ReturnCode::NotReady`, `SendError::QueueFull`) + `AoU` consumer deadline timer (`Error::Timeout`). |
| **`UnintendedFunction`** | Commission / Misrouting / Wrong Target / Aliasing | What happens if this function operates on the wrong `(ServiceId, InstanceId, MethodId)`, delivers data to the wrong subscriber, or accesses aliased memory? | Newtype wrappers (`ServiceId(u16)`, `InstanceId(u16)`), borrow checker (`&mut`), lifetime-bound smart pointers (`SampleGuard<'a, T>`). | E2E `DataId` pseudo-header check (`E2eError::WrongDataId`), UID/GID ACL check (`SecAclResult::PermDenied`). |
| **`DelayedFunction`** (`EarlyOrLateExecution`) | Timing / WCET Overrun / Deadlock / Stale Data | What happens if this function blocks longer than expected, deadlocks on a mutex, or processes a message after its deadline? | Lock-free / actor channels (eliminating `std::recursive_mutex`), bounded poll budgets, non-blocking socket I/O. | Monotonic clock (`std::time::Instant`) age check (`E2eError::StaleDeadline`) + watchdog heartbeat (`AoU-TIM-001`). |
| **`PartialFunction`** (`PartialOrCorruptedExecution`) | Corruption / Truncation / Inconsistent State | What happens if this function writes only part of a payload, mutates header bytes after CRC calculation, or notifies only a subset of subscribers? | Typestate builders (`UnprotectedFrame -> ProtectedFrame`), atomic commit in shared memory, strict length validation before parsing. | Public-domain CRC-8/16/32 verification (`E2eError::CorruptedCrc`), header length check (`ReturnCode::MalformedMessage`). |
| **`ExceedingFunction`** | Babbling Idiot / Resource Exhaustion / Double Invocation | What happens if this function allocates beyond memory limits, invokes a callback repeatedly for one event, or is called re-entrantly? | Fixed-capacity slab pools (`max_tp_reassembly_slots`), per-client byte quotas, `&mut self` preventing re-entrancy. | Token-bucket / debounce rate limiter, sequence counter duplicate check (`E2eError::RepeatedCounter`). |

---

## 2. Subsystem-by-Subsystem SW-FMEA Matrix for Automotive RPC & IPC Middleware

### 2.1 Subsystem 1: `runtime`

| FM ID | Public Rust Boundary | HAZOP Guideword | Failure Mode & Typical C/C++ Root Cause | Local / System Effect (ASIL) | Rust Prevention & Detection Mechanisms | Closed-Loop Mitigation (`ASR-*` / `AoU-*`) |
|---|---|---|---|---|---|---|
| `FM-RT-001` | `Runtime::init(cfg)` | `LossOfFunction` | In-memory configuration diverges from JSON/TOML manifest or silently ignores unknown fields. | Misconfigured routing ports, security policies, or E2E Data IDs (`ASIL-B`). | **Prevention**: Strongly typed serde schema validation with `#[serde(deny_unknown_fields)]` returning `Result<Runtime, ConfigError>`. **Detection**: Startup integrity hash check over config file. | `ASR-CFG-001` + `AoU-CFG-001` (read-only/dm-verity filesystem). |
| `FM-RT-002` | `Runtime::poll_dispatch(max_events)` | `DelayedFunction` | Dispatcher loop blocks indefinitely on a slow user callback or mutex contention. | Starves all other services sharing the runtime executor (`ASIL-B`). | **Prevention**: Non-blocking bounded poll batch (`max_events`) + lock-free actor command queue. **Detection**: Cyclic watchdog callback using monotonic `Instant`. | `ASR-TIM-001` + `AoU-TIM-001` (OS real-time priority & watchdog supervision). |
| `FM-RT-003` | `Runtime::register_message_handler` | `UnintendedFunction` | Wildcard handler (`ANY_SERVICE`, `ANY_METHOD`) shadows a safety-critical exact handler, or callback panics. | Safety-critical RPC request routed to wrong handler or unwinds runtime thread (`ASIL-B`). | **Prevention**: Deterministic handler table separating exact `(ServiceId, InstanceId, MethodId)` keys from wildcards; reject duplicate registration unless explicit. **Detection**: `Result<(), HandlerError::AlreadyRegistered>`. | `ASR-RT-002` + `ASR-ERR-001`. |

### 2.2 Subsystem 2: `routing`

| FM ID | Public Rust Boundary | HAZOP Guideword | Failure Mode & Typical C/C++ Root Cause | Local / System Effect (ASIL) | Rust Prevention & Detection Mechanisms | Closed-Loop Mitigation (`ASR-*` / `AoU-*`) |
|---|---|---|---|---|---|---|
| `FM-ROU-001` | `RoutingManager::offer_service` | `LossOfFunction` | Service offer is silently dropped (`void` return) or offered without an initial field value. | Consumers never discover service or read uninitialized field state (`ASIL-B`). | **Prevention**: Mandatory initial field values in `ServiceSkeletonBuilder` and `#[must_use] Result<OfferGuard, RoutingError>`. **Detection**: Consumer availability monitor (`AvailabilityState::Unavailable`). | `ASR-ROU-001` + `AoU-AVAIL-001` (consumer FTTI safe-state). |
| `FM-ROU-002` | `RoutingManager::offer_service` | `UnintendedFunction` | Skeleton offers a `(ServiceId, InstanceId)` already owned by another process or with mismatched `MajorVersion`. | Overwrites active routing table entry or delivers incompatible payload layout (`ASIL-B`). | **Prevention**: Track active owner `(ClientId, Uid)` per `(ServiceId, InstanceId)` and reject colliding offers with `RoutingError::ServiceAlreadyOffered`. **Detection**: Wire header check `ReturnCode::WrongInterfaceVersion (0x08)`. | `ASR-ROU-002` + `ASR-SEC-001` (UID/GID offer ACL). |
| `FM-ROU-003` | `RoutingManager::notify` | `ExceedingFunction` | Chattering QM producer floods event notifications faster than cyclic interval, starving ASIL routing queues. | Exhausts downstream subscriber queues and CPU budget (`ASIL-B`). | **Prevention**: Per-event monotonic debounce filter (`Instant::now().duration_since(last_tx) < min_interval`) + per-client byte quota. **Detection**: `NotifyError::RateLimited` counter increment. | `ASR-FFI-002` (monotonic debounce & quota enforcement). |
| `FM-ROU-004` | `OfferGuard::drop` / `stop_offer_service` | `LossOfFunction` / `UnintendedFunction` | Producer crashes or drops without withdrawing offer, or frees another instance's routing slot. | Stale route remains advertised or active route is severed (`ASIL-B`). | **Prevention**: RAII `OfferGuard` bound to unique generational slot handle (`SlotId { index, generation }`) + UDS peer disconnect cleanup (`EPOLLHUP`). **Detection**: Offer TTL expiration + cyclic E2E timeout. | `ASR-ROU-003` + `AoU-AVAIL-001`. |

### 2.3 Subsystem 3: `endpoints`

| FM ID | Public Rust Boundary | HAZOP Guideword | Failure Mode & Typical C/C++ Root Cause | Local / System Effect (ASIL) | Rust Prevention & Detection Mechanisms | Closed-Loop Mitigation (`ASR-*` / `AoU-*`) |
|---|---|---|---|---|---|---|
| `FM-EP-001` | `Endpoint::send_frame` | `LossOfFunction` | Outbound frame dropped when endpoint queue limit is reached, or `queue_size + size` overflows when `QUEUE_SIZE_UNLIMITED` is misconfigured. | RPC request or event notification lost prior to socket transmission (`ASIL-B`). | **Prevention**: Mandatory `NonZeroUsize` byte quota per endpoint + checked arithmetic (`checked_add`). **Detection**: Returns `Err(EndpointError::QueueFull)` immediately to caller. | `ASR-EP-001` + `AoU-AVAIL-001`. |
| `FM-EP-002` | `TpReassembler::process_segment` | `ExceedingFunction` | Remote node sends spoofed segmented frames across varying 64-bit IDs with large offsets, causing unbounded map growth and eager buffer resize heap exhaustion. | OOM crash of central routing daemon, severing all ASIL communication (`ASIL-B`). | **Prevention**: Fixed-capacity pre-allocated slab (`TpReassemblyPool<MAX_CONCURRENT_TP, MAX_PDU_LEN>`) with strict 16-byte alignment check. **Detection**: Returns `Err(TpError::ReassemblyPoolExhausted)` without heap allocation. | `ASR-EP-002` + `AoU-MEM-001`. |
| `FM-EP-003` | `FrameDecoder::decode_header` | `PartialFunction` | Truncated or oversized frame header (`Length < 8` or `Length > max_message_length`, bad `ProtocolVersion != 0x01`). | Buffer over-read or deserialization of garbage payload (`ASIL-B`). | **Prevention**: Zero-copy slice parser using `buf.get(..16)` before constructing `HeaderView`. **Detection**: Emits `ReturnCode::MalformedMessage (0x09)` or `ReturnCode::WrongProtocolVersion (0x07)`. | `ASR-EP-003`. |

### 2.4 Subsystem 4: `service_discovery`

| FM ID | Public Rust Boundary | HAZOP Guideword | Failure Mode & Typical C/C++ Root Cause | Local / System Effect (ASIL) | Rust Prevention & Detection Mechanisms | Closed-Loop Mitigation (`ASR-*` / `AoU-*`) |
|---|---|---|---|---|---|---|
| `FM-SD-001` | `ServiceDiscovery::on_resume_timeout` | `ExceedingFunction` | Suspend/resume timeout calls `std::abort()` when a single remote service fails to send an Offer after resume. | Single missing remote service kills the entire local routing daemon (`ASIL-B`). | **Prevention**: Forbid `std::process::abort()` / `exit()` in discovery state machine (`#![deny(clippy::exit)]`). **Detection**: Transition only the affected service entry to `AvailabilityState::Unavailable` and invoke `availability_handler`. | `ASR-SD-001` + `AoU-AVAIL-001`. |
| `FM-SD-002` | `ServiceDiscovery::detect_peer_reboot` | `UnintendedFunction` | Coupled boolean expression on `reboot_flag` and `is_multicast` fails to detect remote ECU reboot or falsely resets active subscriptions. | Stale subscription state persists after remote ECU reboot, causing silent event loss (`ASIL-B`). | **Prevention**: Decomposed MC/DC-verified reboot state machine tracking `(session_id, reboot_flag)` per `(peer_ip, is_multicast)` channel. **Detection**: Triggers immediate re-subscription and notifies `subscription_status_handler`. | `ASR-SD-002`. |

### 2.5 Subsystem 5: `e2e_black_channel`

| FM ID | Public Rust Boundary | HAZOP Guideword | Failure Mode & Typical C/C++ Root Cause | Local / System Effect (ASIL) | Rust Prevention & Detection Mechanisms | Closed-Loop Mitigation (`ASR-*` / `AoU-*`) |
|---|---|---|---|---|---|---|
| `FM-E2E-001` | `OutboundProtector::protect_in_place` | `PartialFunction` | Outbound router mutates `ClientId` bytes `8..9` **after** computing the E2E CRC, corrupting the CRC whenever protection covers header bytes `8..16`. | Receiver rejects every valid protected frame with CRC mismatch (`ASIL-B`). | **Prevention**: Typestate serialization pipeline (`FinalizedHeader -> E2eProtectedWireFrame`) that freezes header bytes `0..16` before computing the CRC. **Detection**: Round-trip self-check in unit/integration tests. | `ASR-E2E-001`. |
| `FM-E2E-002` | `InboundChecker::verify_and_unwrap` | `UnintendedFunction` | Middleware logs E2E check failure at `INFO` and still delivers the corrupted message to the application handler. | Application processes corrupted safety-critical payload if caller omits manual status check (`ASIL-B`). | **Prevention**: Private payload field inside `UnverifiedFrame`; only `verify_and_unwrap(&mut channel_state)` yields `Result<VerifiedPayload<'_>, E2eFault>`. **Detection**: Public-domain CRC-8/16/32 + sequence delta + `DataId` + monotonic reception deadline. | `ASR-E2E-002` + `AoU-E2E-001`. |

### 2.6 Subsystem 6: `security_ffi` & `logging`

| FM ID | Public Rust Boundary | HAZOP Guideword | Failure Mode & Typical C/C++ Root Cause | Local / System Effect (ASIL) | Rust Prevention & Detection Mechanisms | Closed-Loop Mitigation (`ASR-*` / `AoU-*`) |
|---|---|---|---|---|---|---|
| `FM-FFI-001` | `security_policy_is_client_allowed_to_offer` | `UnintendedFunction` | C caller passes `NULL` or unaligned `const security_policy_client_t*`, or Rust policy evaluation panics across `extern "C"` ABI boundary. | Undefined Behavior or crash in routing daemon security hook (`ASIL-B`). | **Prevention**: `std::panic::catch_unwind` barrier at `extern "C"` entry + explicit `!client.is_null() && client.is_aligned()` check before `unsafe { &*client }`. **Detection**: Returns `SECURITY_POLICY_PERM_DENIED` on null/invalid pointer or caught panic. | `ASR-FFI-001`. |
| `FM-LOG-001` | `DiagLogger::log_write_raw` | `DelayedFunction` / `ExceedingFunction` | Fatal log macro triggers null-pointer crash trap, or blocks ASIL thread on a full diagnostic daemon FIFO pipe. | Crashes ASIL process or causes WCET deadline violation during diagnostic logging (`ASIL-B`). | **Prevention**: Write logs into a lock-free bounded ring buffer drained asynchronously by a low-priority QM telemetry task. **Detection**: Atomic `overflow_counter` increment. | `ASR-LOG-001` + `AoU-LOG-001`. |

---

## 3. Typed Rust Error Enumerations as SW-FMEA Detection Mechanisms

In ISO 26262-6 Clause 7.4.10, every SW-FMEA row requires a concrete software **Detection Mechanism**. Map protocol and black-channel error conditions into strongly typed `#[must_use]` Rust error enums so every failure mode produces a deterministic, testable detection signal:

```rust
/// Wire-level RPC Return Codes used as protocol-level SW-FMEA detection signals.
#[repr(u8)]
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum RpcReturnCode {
    Ok = 0x00,
    NotOk = 0x01,
    UnknownService = 0x02,         // Detects FM-ROU-002 / FM-SD-003 (service not registered)
    UnknownMethod = 0x03,          // Detects FM-ROU-003 (unregistered method ID)
    NotReady = 0x04,               // Detects FM-RT-001 / FM-EP-001 (endpoint or routing not ready)
    NotReachable = 0x05,           // Detects FM-EP-001 (remote transport unreachable)
    Timeout = 0x06,                // Detects FM-RT-002 / FM-SD-001 (monotonic deadline expired)
    WrongProtocolVersion = 0x07,   // Detects FM-EP-003 (protocol version != 0x01)
    WrongInterfaceVersion = 0x08,  // Detects FM-ROU-002 (major version mismatch)
    MalformedMessage = 0x09,       // Detects FM-EP-003 (truncated header or payload length mismatch)
    WrongMessageType = 0x0A,       // Detects FM-EP-003 (unexpected message type variant)
    Unknown = 0xFF,
}

/// Clean-room End-to-End (E2E) Black-Channel Verification Status.
#[repr(u8)]
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum E2eCheckError {
    RepeatedCounter = 0x01,        // Detects Annex D.2.4 Repetition (delta == 0)
    WrongSequence = 0x02,          // Detects Annex D.2.4 Loss / Insertion (delta > max_delta)
    CorruptedCrc = 0x03,           // Detects Annex D.2.4 Corruption / FM-E2E-001
    NotAvailable = 0x04,           // Detects Annex D.2.4 Loss / ProducerUnavailable
    NoNewData = 0x05,              // Detects Annex D.2.4 Stale / Unupdated sample
    WrongDataId = 0x06,            // Detects Annex D.2.4 Masquerading / Misrouting
    StaleDeadline = 0x07,          // Detects Annex D.2.4 Asymmetric Delay / Timeout
}
```

---

## 4. Rust Module Template with Embedded SW-FMEA & SEooC Traceability

Below is a Rust trait boundary showing how `pub trait` and `pub fn` items embed machine-verifiable `@satisfies ASR-*`, `@fmea FM-*`, and `@aou AoU-*` tags checked by `scripts/validate_seooc_fmea.py`:

```rust
#![forbid(unsafe_code)]
#![deny(clippy::unwrap_used, clippy::expect_used, clippy::indexing_slicing, clippy::arithmetic_side_effects)]

use core::num::NonZeroUsize;
use std::time::{Duration, Instant};

#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub struct ServiceId(pub u16);

#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub struct InstanceId(pub u16);

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum OfferError {
    InvalidServiceId,
    ServiceAlreadyOffered,
    RoutingQueueFull,
}

#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum SendError {
    QueueFull,
    PayloadExceedsMaxPdu { len: usize, max: usize },
    EndpointDisconnected,
}

/// Core SEooC Runtime boundary (`ISO 26262-10 Clause 9` / `ISO 26262-6 Clause 7.4.10`).
///
/// # Safety Requirements
/// - `@satisfies ASR-COM-001` (ASIL-B): Zero panics or unbounded allocations across all runtime APIs.
/// - `@satisfies ASR-ROU-001` (ASIL-B): Deterministic service ownership and bounded transmit queues.
///
/// # Assumptions of Use
/// - `@aou AoU-AVAIL-001` (ASIL-B, mitigates `FM-ROU-001`, `FM-EP-001`): Integrator must monitor
///   service availability and handle `OfferError` / `SendError` within the item's FTTI.
/// - `@aou AoU-TIM-001` (ASIL-B, mitigates `FM-RT-002`): Integrator must execute `poll_dispatch`
///   under a real-time scheduler priority supervised by an external deadline watchdog.
pub trait RpcSeooCRuntime {
    /// Offers a service instance to local and remote consumers.
    ///
    /// # Safety Requirements
    /// - `@satisfies ASR-ROU-001` (ASIL-B)
    /// - `@satisfies ASR-ROU-002` (ASIL-B)
    ///
    /// # Failure Modes (SW-FMEA)
    /// - `@fmea FM-ROU-001` (`LossOfFunction`): Returns `Err(OfferError::RoutingQueueFull)` if the
    ///   command channel is saturated.
    /// - `@fmea FM-ROU-002` (`UnintendedFunction`): Returns `Err(OfferError::ServiceAlreadyOffered)`
    ///   if `(service, instance)` is already owned by an active provider.
    ///
    /// # Assumptions of Use
    /// - `@aou AoU-AVAIL-001` (ASIL-B, mitigates `FM-ROU-001`)
    /// - `@aou AoU-CFG-001` (ASIL-B, mitigates `FM-ROU-002`)
    #[must_use = "ASIL-B AoU-AVAIL-001: caller must verify that the service offer succeeded"]
    fn offer_service(
        &mut self,
        service: ServiceId,
        instance: InstanceId,
        major_version: u8,
        minor_version: u32,
    ) -> Result<(), OfferError>;

    /// Enqueues a bounded payload for transmission.
    ///
    /// # Safety Requirements
    /// - `@satisfies ASR-COM-001` (ASIL-B)
    /// - `@satisfies ASR-EP-001` (ASIL-B)
    ///
    /// # Failure Modes (SW-FMEA)
    /// - `@fmea FM-EP-001` (`LossOfFunction`): Returns `Err(SendError::QueueFull)` when the per-client
    ///   byte quota is reached.
    /// - `@fmea FM-EP-003` (`PartialFunction`): Returns `Err(SendError::PayloadExceedsMaxPdu)` if
    ///   `payload.len()` exceeds the configured maximum PDU size.
    ///
    /// # Assumptions of Use
    /// - `@aou AoU-AVAIL-001` (ASIL-B, mitigates `FM-EP-001`)
    /// - `@aou AoU-MEM-001` (ASIL-B, mitigates `FM-EP-003`)
    #[must_use = "ASIL-B AoU-AVAIL-001: caller must handle SendError to detect communication loss"]
    fn send(
        &mut self,
        service: ServiceId,
        instance: InstanceId,
        method: u16,
        payload: &[u8],
    ) -> Result<u16, SendError>;

    /// Polls up to `max_batch` pending events and updates the monotonic watchdog timestamp.
    ///
    /// # Safety Requirements
    /// - `@satisfies ASR-TIM-001` (ASIL-B)
    ///
    /// # Failure Modes (SW-FMEA)
    /// - `@fmea FM-RT-002` (`DelayedFunction`): Bounded by `max_batch` to prevent unbounded loop
    ///   starvation; external OS preemption is supervised via `watchdog_now`.
    ///
    /// # Assumptions of Use
    /// - `@aou AoU-TIM-001` (ASIL-B, mitigates `FM-RT-002`)
    #[must_use = "ASIL-B AoU-TIM-001: caller must inspect processed count and watchdog status"]
    fn poll_dispatch(
        &mut self,
        max_batch: NonZeroUsize,
        watchdog_now: Instant,
        max_cycle_budget: Duration,
    ) -> Result<usize, RpcReturnCode>;
}
```
