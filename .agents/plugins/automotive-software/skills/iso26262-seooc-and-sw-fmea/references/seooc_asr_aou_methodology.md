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

# ISO 26262-10:2018 Clause 9 Software SEooC Methodology: `ASR-*`, `AoU-*`, and Safety Manual Engineering

This reference provides the engineering methodology and artifact templates for qualifying an automotive Rust middleware crate (such as an RPC router, diagnostic telemetry client, or shared-memory IPC binding) as a **Software Safety Element out of Context (SEooC)** aligned with **ISO 26262-10:2018 Clause 9** and **ISO 26262-6:2018 Clauses 6, 7, and 11**.

---

## 1. ISO 26262-10:2018 Clause 9.2.3 — Software SEooC Development Lifecycle

Normally, ISO 26262 follows a top-down V-model starting from a vehicle-level **Item Definition** (ISO 26262-3 Clause 5), Hazard Analysis and Risk Assessment (HARA, Clause 6), Functional Safety Concept (FSC, Clause 7), and Technical Safety Concept (TSC, ISO 26262-4 Clause 6).

When developing a reusable middleware crate, the vehicle item does not exist yet. ISO 26262-10:2018 Clause 9.2.3 resolves this by requiring the SEooC developer to explicitly formulate and document the **assumed context** above the software boundary:

| ISO 26262-10 Clause 9 Step | Artifact ID Prefix | Engineering Content | Verification Owner & Phase |
|---|---|---|---|
| **Step 1: Assumed Boundary & Context** | `CTX-*` | Define the SEooC's purpose, target ASIL capability (e.g., ASIL-B per ISO 26262-6), hardware/OS platform (`Linux` / `QNX`), and external interfaces (`UDP/TCP sockets`, `Unix Domain Sockets`, `SHM`, `C ABI`). | SEooC Architect (ISO 26262-10 Clause 9.2.3.1) |
| **Step 2: Assumed System Safety Requirements** | `ASSR-*` / `ASR_*` | High-level assumptions about system-level hazards and how the system uses the middleware (e.g., `ASR_SafeCommunication`, `ASR_ErrorHandling`, `ASR_SupportForTimeBasedArchitectures`). | Validated by System Integrator against actual TSC during Item Integration (ISO 26262-4 Clause 7) |
| **Step 3: Assumed Software Safety Requirements** | `ASR-*` / `FEAT_*` | Concrete, testable software safety requirements allocated to the Rust crate (derived from `ASSR-*` and SW-FMEA preventive/detection measures). | Verified internally by SEooC Developer via unit/integration/fault-injection tests (ISO 26262-6 Clauses 9–10) |
| **Step 4: Inductive SW-FMEA & FTA** | `FM-*` / `FTA-*` | Systematic analysis of failure modes at every public API (`pub fn`, `pub trait`, `extern "C" fn`) using HAZOP guidewords (`ISO 26262-6 Clause 7.4.10`). | SEooC Safety Analyst; every `FM-*` must trace to an `ASR-*` or `AoU-*` |
| **Step 5: Assumptions of Use (Safety Manual)** | `AoU-*` | External constraints that the crate cannot enforce internally (e.g., OS thread priority, watchdog monitoring, application FTTI safe-state transition, filesystem integrity). | Exported in Safety Manual; verified by System Integrator during ISO 26262-6 Clause 11 / ISO 26262-4 integration |
| **Step 6: Validity Mismatch Resolution** | `CR-*` | During vehicle integration, if an `ASSR-*` or `AoU-*` conflicts with the actual vehicle architecture, perform an Impact Analysis (ISO 26262-10 Figure 10). | Joint OEM / Tier-1 Integrator & SEooC Supplier |

---

## 2. Baseline Assumed System Requirements & Rust Compile-Time `AoU` Reduction

### 2.1 Core `AssumedSystemReq` Categories for Automotive Middleware SEooCs

| Assumed System Category | ASIL | Core Assumption & Rationale | Rust Middleware `ASR-*` Realization |
|---|---|---|---|
| **Operational Error Handling** | `ASIL-B` | Exceptions/aborts shall not terminate shared processes on operational errors. | `ASR-ERR-001`: Zero panics/unwinds on operational errors; all fallible APIs return `#[must_use] Result<T, Error>`. |
| **Inter-Process Communication** | `ASIL-B` | Platform supports bounded IPC across multiple processes. | `ASR-IPC-001`: Bounded Unix domain socket / shared-memory routing between local clients and router. |
| **Intra-Process Dispatch** | `ASIL-B` | Platform supports intra-process communication between local components. | `ASR-IPC-002`: Zero-copy bounded channel dispatch for in-process services. |
| **External ECU Networking** | `ASIL-B` | Platform supports external ECU-to-ECU RPC, discovery, and segmentation over UDP/TCP. | `ASR-NET-001`: Deterministic big-endian wire framing over non-blocking UDP and TCP endpoints. |
| **End-to-End Safe Communication** | `ASIL-B` | Platform provides ISO 26262 Annex D.2.4 mechanisms to detect black-channel faults. | `ASR-E2E-001`: Pluggable `PayloadProtector` envelope (CRC, sequence counter, Data ID, reception deadline). |
| **Access Control & Partitioning** | `ASIL-B` | Information is protected against unauthorized manipulation by co-hosted QM apps. | `ASR-SEC-001`: Mandatory `SO_PEERCRED` (`uid_t`, `gid_t`) + IP/port ACL policy verification before offer/request/subscribe. |
| **Time-Triggered Cyclic Execution** | `ASIL-B` | Supports deterministic cyclic polling intervals with bounded storage. | `ASR-TIM-001`: Non-blocking cyclic poll API (`try_recv_samples`) and monotonic deadline timers (`Instant`). |
| **Language & ABI Interoperability** | `ASIL-B` | Supports safe Rust and C/C++ callers up to the target ASIL. | `ASR-FFI-001`: Native safe Rust API plus panic-isolated `#[repr(C)]` C/C++ FFI bindings. |

### 2.2 Converting Legacy C++ Integrator `AoU` Obligations into Compile-Time Rust `ASR-*` Guarantees

Because C++ cannot statically enforce lifetime validity, exclusive mutability, or non-throwing callbacks, C++ middleware Safety Manuals export dozens of manual rules to the human integrator. In Rust, convert these directly into compile-time `ASR-*` guarantees:

| Legacy C++ Integrator `AoU` | Mitigated `FailureMode` | How Rust Eliminates the External `AoU` via Compile-Time `ASR-*` |
|---|---|---|
| Do not call symbols from internal implementation namespaces | `MisusedApis` | **Module Privacy (`pub(crate)` / private `mod`)**: Internal modules are not `pub`; callers cannot invoke private APIs. |
| Do not initialize runtime objects in static constructors before `main()` | `MisusedApis` | **Explicit Value Construction**: Rust `const` initialization and explicit `Runtime::init()` values prevent static initialization order bugs. |
| Hold at most one active zero-copy `AllocateePtr` per publisher | `TooFewMemoryAllocated` | **Exclusive Mutable Borrow (`&mut EventPublisher<T>`)**: `allocate(&mut self) -> Result<SampleAllocatee<'_, T>, Error>` borrows the publisher mutably (`E0499` on second call). |
| Do not call copy-based `send()` while holding an `AllocateePtr` | `TooFewMemoryAllocated` | **Exclusive Mutable Borrow (`&mut EventPublisher<T>`)**: While `SampleAllocatee<'a, T>` is alive, `&'a mut EventPublisher<T>` remains borrowed (`E0502`). |
| Do not invoke non-thread-safe event methods concurrently | `FunctionCalledFromMultipleThreads` | **`Send` / `Sync` & `&mut self`**: Non-thread-safe handles omit `Sync` or require `&mut self`, rejecting concurrent access at compile time. |
| Keep `Skeleton` alive while `AllocateePtr` is in use | `ReturnsWrongData` | **Lifetime Parameter (`SampleAllocatee<'skel, T>`)**: Dropping the `Skeleton` while holding `SampleAllocatee` fails to compile (`E0505`). |
| Keep subscription active while holding `SamplePtr` | `CallbackInvokedWithWrongData` | **Lifetime or Ref-Counted Guard**: `unsubscribe(self)` consumes the subscription or requires `&mut self` while `SamplePtr<'sub, T>` borrows `&'sub self`. |
| Registered user callbacks shall never throw exceptions | `MisusedApis` | **No Exceptions + Panic Isolation**: Rust closures cannot throw C++ exceptions; FFI callbacks wrap execution in `std::panic::catch_unwind`. |
| Dereference `SamplePtr` only while smart pointer is valid | `ReturnsWrongData` | **`Deref<Target = T>` Lifetime Binding**: Dereferencing `SamplePtr<T>` yields `&T` strictly bound to `&SamplePtr<T>` (`E0597`). |

---

## 3. Residual `AoU-*` Catalog for an Automotive Rust Middleware SEooC Crate

Even with Rust's compile-time guarantees, a network/IPC middleware crate still relies on external system properties that **must** be documented as `AoU-*` items in the Safety Manual and verified by the system integrator:

| AoU ID | ASIL | Mitigated Failure Modes (`FM-*`) | Assumption of Use Description (Integrator Obligation) | Integrator Verification Method (ISO 26262-6 Clause 11) |
|---|---|---|---|---|
| `AoU-CFG-001` | `ASIL-B` | `FM-CFG-001`, `FM-SD-002` | Store the runtime configuration file on a read-only or integrity-protected (`dm-verity` / CRC) filesystem partition qualified to at least ASIL-B, and verify that `(ServiceId, InstanceId, MajorVersion, MinorVersion)` and unicast/multicast endpoints match across ECU manifests. | System integration review of rootfs mount options + automated schema cross-check between ECU service manifests. |
| `AoU-MEM-001` | `ASIL-B` | `FM-MEM-001`, `FM-MEM-002` | Dimension pre-allocated slab/pool capacities (`max_endpoints`, `max_pending_requests`, `max_tp_reassembly_slots`, `max_subscribers`) for worst-case bus load and lock process memory via `mlockall(MCL_CURRENT \| MCL_FUTURE)` with swap disabled. | Static memory budget calculation + integration soak test under peak burst load verifying zero pool exhaustion drops. |
| `AoU-AVAIL-001` | `ASIL-B` | `FM-SD-001`, `FM-SD-003`, `FM-SD-004` | Monitor service availability (`AvailabilityState::Unavailable`) and cyclic reception deadlines (`Error::Timeout`), transitioning the vehicle item to its degraded or safe state within the Fault-Tolerant Time Interval (FTTI). | Fault-injection test at system integration level (kill provider process / disconnect Ethernet link) verifying safe-state transition within FTTI. |
| `AoU-TIM-001` | `ASIL-B` | `FM-TIM-001`, `FM-RPC-002` | Assign the routing manager and ASIL client executor threads deterministic real-time scheduling priorities (`SCHED_FIFO`) and supervise safety-critical control loops with an external hardware or OS temporal watchdog. | WCET measurement on target ECU + watchdog starvation injection test verifying watchdog trigger. |
| `AoU-SEC-001` | `ASIL-B` | `FM-SEC-001`, `FM-SEC-002` | Execute processes of different ASIL partitions (QM vs. ASIL-B) under distinct POSIX UIDs/GIDs with security policy enforcement enabled so `SO_PEERCRED` (`uid_t`, `gid_t`) and IP/port ACLs maintain Freedom from Interference. | Integration security test attempting `offer_service` and `send` from an unprivileged QM UID and verifying `PERM_DENIED` rejection. |
| `AoU-E2E-001` | `ASIL-B` | `FM-E2E-001`, `FM-E2E-002` | Configure an ASIL-B `PayloadProtector` (CRC + counter + Data ID + timeout) end-to-end whenever safety messages traverse an untrusted communication channel (UDP/TCP Ethernet, non-qualified OS socket stack, or shared QM router). | Fault-injection test injecting bit-flips, repeated frames, and sequence drops on the wire and verifying 100% rejection at consumer boundary. |
| `AoU-LOG-001` | `ASIL-B` | `FM-LOG-001`, `FM-LOG-002` | Treat diagnostic log sinks as untrusted QM components: invoke logging from ASIL-B threads exclusively in non-blocking ring-buffer mode and never base safety-relevant control flow on logger return values or daemon availability. | Integration test stopping the diagnostic logging daemon while flooding logs from an ASIL-B task and verifying zero blocking or deadline misses. |

---

## 4. Artifact Templates

### 4.1 TRLC Schema & Artifact Template (`seooc_router.trlc`)

```trlc
package RpcRustSeooC

import SafetyReq

SafetyReq.AssumedSystemReq ASR_NonBlockingBoundedRouting {
    description = "The RPC Rust runtime shall process all outbound and inbound messages using pre-allocated bounded queues without blocking the caller thread or invoking std::process::abort()."
    safety = SafetyReq.Asil.B
    rationale = "Prevents temporal interference (Annex D.2.1) and spatial heap exhaustion (Annex D.2.2) across mixed QM/ASIL partitions."
    version = 1
}

SafetyReq.FailureMode FM_OfferServiceDropped {
    guidewords = [ SafetyReq.Guideword.LossOfFunction ]
    description = "Runtime::offer_service fails to register or multicast a service instance due to local routing queue saturation or invalid configuration."
    failureeffect = "Remote and local consumers do not discover the service instance; safety-critical data exchange does not start."
    version = 1
    safety = SafetyReq.Asil.B
    rationale = "Prevented from silent failure by returning #[must_use] Result<OfferHandle, OfferError> (ASR_NonBlockingBoundedRouting) and mitigated at item level by AoU_ServiceAvailabilitySupervision."
    interface = "rpc_runtime::Runtime::offer_service"
}

SafetyReq.AoU AoU_ServiceAvailabilitySupervision {
    description = "The system integrator shall ensure that the vehicle item supervises the availability of required services and transitions to a safe state within FTTI if a service is not offered or stops sending cyclic events."
    safety = SafetyReq.Asil.B
    note = "Verify during system integration by suppressing OfferService multicast packets and confirming safe-state activation within FTTI."
    version = 1
    mitigates = "FM_OfferServiceDropped"
}
```

### 4.2 YAML SEooC & SW-FMEA Artifact Template (`seooc_fmea.yaml`)

`scripts/validate_seooc_fmea.py` validates `.trlc`, `.yaml`/`.json`, and `.md` SEooC + SW-FMEA artifacts:

```yaml
seooc_metadata:
  crate_name: "rpc_runtime"
  target_asil: "ASIL-B"
  iso26262_clauses:
    - "ISO 26262-10:2018 Clause 9 (SEooC)"
    - "ISO 26262-6:2018 Clause 7.4.10 (SW-FMEA)"

assumed_safety_requirements:
  - id: "ASR-COM-001"
    asil: "ASIL-B"
    description: "All fallible public APIs shall return #[must_use] Result<T, Error> without panicking or calling abort()."
    rationale: "Eliminates sudden process termination and silent void error drops."
    verification_tests:
      - "tests::seooc::test_offer_service_returns_error_on_invalid_config"
      - "tests::seooc::test_send_returns_queue_full_on_backpressure"
  - id: "ASR-E2E-001"
    asil: "ASIL-B"
    description: "End-to-end black-channel verification shall run before message delivery and reject corrupted or out-of-sequence frames via the type system."
    rationale: "Prevents silent delivery of corrupted payloads."
    verification_tests:
      - "tests::e2e::test_corrupted_crc_frame_cannot_construct_verified_message"

assumptions_of_use:
  - id: "AoU-AVAIL-001"
    asil: "ASIL-B"
    description: "The system integrator shall supervise cyclic reception deadlines and transition to a safe state within FTTI if a service is unavailable."
    mitigates:
      - "FM-RT-001"
      - "FM-EP-001"
    integrator_verification: "Fault-injection test disconnecting provider endpoint and measuring time to safe-state transition <= FTTI."
  - id: "AoU-TIM-001"
    asil: "ASIL-B"
    description: "The system integrator shall configure deterministic real-time scheduling (SCHED_FIFO) and external watchdog monitoring for the routing thread."
    mitigates:
      - "FM-RT-003"
    integrator_verification: "Target ECU sched_setattr audit and watchdog timeout injection test."

failure_modes:
  - id: "FM-RT-001"
    interface: "Runtime::offer_service"
    guideword: "LossOfFunction"
    asil: "ASIL-B"
    description: "Service offer fails to register or transmit due to invalid service configuration or disconnected routing channel."
    local_effect: "Runtime::offer_service returns Err(OfferError)."
    system_effect: "Consumers do not discover the service instance."
    prevention_mechanism: "Typed ServiceConfig validation and #[must_use] Result return type."
    detection_mechanism: "Explicit OfferError::RoutingDisconnected / Consumer deadline monitor."
    mitigated_by:
      - "ASR-COM-001"
      - "AoU-AVAIL-001"
    verification_test: "tests::seooc::test_offer_service_returns_error_on_invalid_config"
  - id: "FM-EP-001"
    interface: "Runtime::send"
    guideword: "LossOfFunction"
    asil: "ASIL-B"
    description: "Outbound message is dropped because per-client bounded transmit queue is full."
    local_effect: "Runtime::send returns Err(SendError::QueueFull)."
    system_effect: "RPC request or event notification does not reach remote peer."
    prevention_mechanism: "Pre-allocated per-client bounded channel quota (Annex D.2.3)."
    detection_mechanism: "Caller receives SendError::QueueFull immediately; receiver E2E alive counter / deadline timer detects missing frame."
    mitigated_by:
      - "ASR-COM-001"
      - "AoU-AVAIL-001"
    verification_test: "tests::seooc::test_send_returns_queue_full_on_backpressure"
  - id: "FM-RT-003"
    interface: "Runtime::poll_dispatch"
    guideword: "DelayedFunction"
    asil: "ASIL-B"
    description: "Dispatch loop execution is delayed beyond expected cycle time due to OS thread preemption."
    local_effect: "Inbound messages remain queued in socket buffer longer than expected."
    system_effect: "Increased end-to-end communication latency."
    prevention_mechanism: "Bounded poll budget per iteration (max_messages_per_poll)."
    detection_mechanism: "Monotonic timestamp deadline check + external watchdog handler."
    mitigated_by:
      - "AoU-TIM-001"
    verification_test: "tests::seooc::test_poll_dispatch_respects_batch_budget"
  - id: "FM-E2E-001"
    interface: "PayloadProtector::verify_frame"
    guideword: "PartialOrCorruptedExecution"
    asil: "ASIL-B"
    description: "Wire payload or header bytes are corrupted in transit over the black channel."
    local_effect: "verify_frame returns Err(E2eError::CorruptedCrc) and refuses to construct VerifiedMessage."
    system_effect: "Corrupted frame is discarded; application is notified via E2eError counter."
    prevention_mechanism: "Type-state VerifiedMessage wrapper cannot be constructed without passing verify_frame."
    detection_mechanism: "Public-domain CRC-16/CRC-32 + sequence counter + DataId verification."
    mitigated_by:
      - "ASR-E2E-001"
    verification_test: "tests::e2e::test_corrupted_crc_frame_cannot_construct_verified_message"
```

---

## 5. ISO 26262-10 Clause 9 Safety Manual Checklist for Release Qualification

Before releasing an automotive Rust SEooC crate for integration into an ASIL-B/D item, verify that the crate's Safety Manual (`SAFETY_MANUAL.md` or TRLC/YAML export) contains all 8 sections:

1. **SEooC Identification & Version Lock**: Crate name, semantic version, Git commit SHA, Rust compiler version (`rustc` / Ferrocene qualification target), and target ASIL capability (`ASIL-B` or `ASIL-D`).
2. **Assumed System Context & Architectural Boundaries**: Block diagram showing the SEooC boundary, supported OS/hardware platforms, and excluded QM components.
3. **Assumed System & Software Safety Requirements (`ASR-*`)**: Complete table of all `ASR-*` IDs, ASIL ratings, rationales, and passing verification test targets.
4. **Closed-Loop SW-FMEA Summary (`FM-*`)**: Complete table of all public API failure modes derived via HAZOP guidewords, showing zero unmitigated ASIL failure modes.
5. **Catalog of Assumptions of Use (`AoU-*`)**: Complete table of all external integrator obligations, each linked to the `FM-*` IDs it mitigates and paired with an explicit **Integrator Verification Method**.
6. **Configured Compiler & Clippy Hardening Flags**: Exact `Cargo.toml` / `clippy.toml` / `RUSTFLAGS` required when compiling the crate into the final ECU binary (e.g., `panic = "abort"`, `-C overflow-checks=on`).
7. **Known Limitations & Anomaly List**: Explicit documentation of any unsupported features or protocol modes.
8. **Bidirectional Traceability Matrix**: Machine-verified output from `scripts/validate_seooc_fmea.py` confirming 100% linkage across `Public Rust API <-> FM-* <-> ASR-*/AoU-* <-> Verification Test`.
