---
name: iso26262-seooc-and-sw-fmea
description: >-
  Guides developing and auditing automotive Rust middleware and libraries as an
  ISO 26262-10:2018 Clause 9 Software Safety Element out of Context (SEooC) and
  conducting ISO 26262-6:2018 Clause 7 / ISO 26262-9 Clause 8 inductive Software
  FMEA (SW-FMEA). Extracts Assumed Safety Requirements (ASR-*), Assumptions of
  Use (AoU-*), and HAZOP failure modes (FM-*) from Rust public APIs and
  validates closed-loop mitigation chains across TRLC, YAML, Markdown, and
  rustdoc. Use when: qualifying a reusable Rust middleware, logging, or IPC
  crate without a vehicle Item Definition, writing a Safety Manual, or
  validating ASR/AoU/FM linkage. Don't use for: general traceability matrices
  (use traceability-audit), MC/DC coverage (use iso26262-mcdc-and-coverage), or
  E2E CRC wire framing (use iso26262-e2e-black-channel).
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

# ISO 26262-10 Clause 9 SEooC & ISO 26262-6 Clause 7 Software FMEA for Automotive Rust

## Quick Start

| Goal | Command |
|---|---|
| **Run built-in SEooC & SW-FMEA validator self-test** | `python3 skills/iso26262-seooc-and-sw-fmea/scripts/validate_seooc_fmea.py --self-test` |
| **Validate closed-loop `ASR-*` / `AoU-*` / `FM-*` linkage** | `python3 skills/iso26262-seooc-and-sw-fmea/scripts/validate_seooc_fmea.py --fmea-file path/to/seooc_fmea.yaml --rust-crate path/to/crate/src` |

Reusable automotive middleware crates — such as service-oriented RPC routers, diagnostic logging clients, or shared-memory inter-process communication (IPC) bindings — are developed without a single fixed vehicle **Item Definition** (ISO 26262-3 Clause 5). Under **ISO 26262-10:2018 Clause 9**, such components are developed and qualified as a **Software Safety Element out of Context (SEooC)** paired with an inductive **Software Failure Mode and Effects Analysis (SW-FMEA)** per **ISO 26262-6:2018 Clause 7.4.10 (Table 4)** and **ISO 26262-9:2018 Clause 8**.

Without structured methodology, safety artifacts often degenerate into generic prose caveats ("the integrator must use this library safely") while leaving public Rust APIs (`pub fn`, `pub trait`, `unsafe extern "C" fn`) unanalyzed and unlinked to verifiable mitigations.

---

## 1. ISO 26262-10:2018 Clause 9 — Software SEooC Lifecycle & Artifact Chain

When developing a software component out of context (ISO 26262-10 Clause 9.2.3), establish two distinct, machine-traceable sets of assumptions at the crate boundary:

1. **Assumed Safety Requirements (`ASR-*`)**:
   - **Assumed System-Level Safety Requirements** and derived **Assumed Software Safety Requirements** that define the functional and safety behavior the crate implements internally.
   - **Verification Owner**: The **SEooC Crate Developer** verifies every `ASR-*` inside the repository via unit tests (ISO 26262-6 Clause 9), fault-injection tests (Clause 10), or formal/Miri/Kani proofs before release.
2. **Assumptions of Use (`AoU-*`)**:
   - External constraints on the execution environment, operating system, hardware, configuration files, or caller behavior that the crate *cannot* enforce at compile time or runtime, but upon which its safety argument relies.
   - **Verification Owner**: Exported in the crate's **Safety Manual** and `# Assumptions of Use` rustdoc sections; the **OEM / Tier-1 System Integrator** verifies every `AoU-*` during Item Integration (ISO 26262-4 Clause 7 / ISO 26262-6 Clause 11) and checks validity against the actual vehicle Item Definition (ISO 26262-10 Figure 10).

```
[Assumed System Context & Hazards]
       │
       ├──► Assumed Safety Requirements (ASR-*) ──► Crate Design & Prevention (SWE.2/SWE.3) ──► Crate Tests (SWE.4–SWE.6)
       │
       └──► Inductive SW-FMEA (FM-* per pub fn/trait via HAZOP Guidewords)
                   │
                   ├──► Mitigated Internally by ASR-* + Rust Type/Runtime Mechanism + Verification Test
                   └──► Mitigated Externally by AoU-* (mitigates = "FM-*") ──► Safety Manual & Integrator Check
```

---

## 2. How Rust's Type System Reshapes SEooC `AoU-*` & SW-FMEA Failure Modes

When migrating C++17 automotive middleware to Rust, avoid blindly copying C++ memory/lifetime Assumptions of Use into the Rust Safety Manual instead of converting them into **compile-time Prevention Mechanisms** (`ASR-*`):

| Legacy C++ Failure Mode / Integrator `AoU` | Why C++ Needed an External `AoU` | Rust SEooC Transformation (`ASR-*` vs. Residual `AoU-*`) |
|---|---|---|
| Dangling `SamplePtr` / `AllocateePtr` after `Unsubscribe()` or `Skeleton` destruction | C++ smart-pointer wrappers can dangle if the subscription or producer is destroyed while a sample reference is held. | **Eliminated by Borrow Checker (`ASR-*`)**: `SampleGuard<'sub, T>` borrows `&'sub Subscription<T>`, or holds an `Arc<ShmRegion>` keep-alive token. Dangling access fails to compile (`E0505`/`E0597`). |
| Non-reentrant instance methods called concurrently from multiple threads | C++ cannot statically prevent calling non-thread-safe instance methods concurrently without external integrator discipline. | **Eliminated by `&mut self` & `Send`/`Sync` (`ASR-*`)**: State-mutating methods take `&mut self` (requiring exclusive access at compile time) or dispatch over bounded actor channels. |
| User callbacks throwing exceptions across middleware locks/destructors | C++ callbacks throwing exceptions unwind through middleware invariants and trigger `std::terminate()`. | **Eliminated by Type System (`ASR-*`)**: Rust has no exceptions; panics in user closures or `extern "C"` callbacks are isolated via `catch_unwind` or forbidden via `panic = "abort"` + lint gates. |
| Opt-in `msg.is_valid_crc()` check forgotten by application handler | Delivering untrusted messages with a boolean getter allows corrupted payloads through whenever a caller omits the check. | **Eliminated by Typed Envelope (`ASR-*`)**: `VerifiedFrame<T>` cannot be constructed unless `PayloadProtector::verify_frame()` succeeds; corrupted frames return `Err(E2eError)` instead of silent delivery. |
| Unbounded RPC latency or lost messages under network drop / CPU starvation | OS scheduling starvation, physical link severance, or peer crash can delay or drop responses indefinitely. | **Remains a Mandatory `AoU-*` + Bounded Timeout (`ASR-*`)**: Combine internal monotonic deadline timeouts (`ASR-*`) with an explicit `AoU-*` requiring system-level watchdog supervision and safe-state transition on `Error::Timeout`. |

---

## 3. Side-by-Side Anti-Patterns vs. Recommended Patterns

### Pattern A: Unchecked `void` Public API vs. Type-Enforced Rust SEooC Boundary with `# Assumptions of Use`

#### Anti-Pattern: Silent `void` Return, Optional CRC Check, and Vague Prose Caveat
```rust
// Bad: Uses untyped void return and vague prose instead of traceable AoU/ASR IDs.
/// Sends an RPC message.
/// Note: Make sure the system is configured properly and check `msg.is_valid_crc()` on receive.
pub fn send(&self, msg: Message) {
    // Silently drops message if outbound queue is full or service is not offered!
    let _ = self.tx_queue.try_send(msg);
}
```

#### Recommended Pattern: Closed-Loop `ASR-*` / `AoU-*` Rustdoc Contract & Typed Failure Modes
```rust
/// Enqueues a verified message for transmission over the bound transport endpoint.
///
/// # Safety Requirements
/// - `@satisfies ASR-COM-004` (ASIL-B): Non-blocking bounded transmission queue; never panics or
///   allocates unbounded heap memory on peer backpressure (`FM-EP-002`: `ExceedingFunction`).
/// - `@satisfies ASR-COM-007` (ASIL-B): Computes E2E black-channel header in-place *after* setting
///   `ClientId`/`SessionId` (`FM-E2E-001`: `PartialOrCorruptedExecution`).
///
/// # Failure Modes (SW-FMEA)
/// - `@fmea FM-EP-001` (`LossOfFunction`): Returns [`SendError::QueueFull`] if the per-client quota is
///   exhausted, or [`SendError::EndpointNotReady`] if the transport is disconnected.
/// - `@fmea FM-EP-003` (`DelayedFunction`): Transmission latency depends on OS socket scheduling.
///
/// # Assumptions of Use
/// - `@aou AoU-COM-002` (ASIL-B, mitigates `FM-EP-001`): The integrating application MUST inspect
///   the returned [`Result`] and transition to its degraded/safe state if [`SendError::QueueFull`]
///   or [`SendError::EndpointNotReady`] persists beyond the item's fault-tolerant time interval (FTTI).
/// - `@aou AoU-OS-001` (ASIL-B, mitigates `FM-EP-003`): The system integrator MUST execute the
///   routing worker thread under a deterministic real-time priority (`SCHED_FIFO`) monitored by a
///   deadline watchdog.
#[must_use = "ASIL-B AoU-COM-002: caller must handle SendError to detect communication loss"]
pub fn send(&self, msg: ValidatedMessage) -> Result<SessionId, SendError> {
    self.tx_queue.try_enqueue(msg).map_err(|_| SendError::QueueFull)
}
```

### Pattern B: Unlinked SW-FMEA Prose vs. Closed-Loop TRLC / YAML SEooC & SW-FMEA Artifacts

#### Anti-Pattern: Orphan Failure Modes and Empty `mitigates` / `rationale` Fields
```trlc
// Bad: Empty rationale on FailureMode and empty `mitigates = ""` on AoU break ISO 26262-10 Clause 9 linkage!
SafetyReq.FailureMode ServiceNotFound {
    guidewords = [ SafetyReq.Guideword.LossOfFunction ]
    description = "A proxy does not find a service instance, even though it was offered."
    failureeffect = "No communication between processes will happen."
    version = 1
    safety = SafetyReq.Asil.B
    rationale = ""
    interface = "ipc.Proxy.FindService"
}

SafetyReq.AoU SameCompilerSettingsForProviderAndConsumerSide {
    description = "All compiler settings influencing binary layout have to be identical."
    safety = SafetyReq.Asil.B
    version = 1
    mitigates = ""
}
```

#### Recommended Pattern: Closed-Loop `FailureMode <-> ASR / AoU <-> Verification Test`
```trlc
package RpcRouterSeooC
import SafetyReq

SafetyReq.AssumedSystemReq ASR_DeterministicWireLayout {
    description = "The RPC crate shall serialize all headers and payloads in deterministic big-endian wire format independent of host CPU endianness or C++ compiler ABI."
    safety = SafetyReq.Asil.B
    rationale = "Prevents cross-ECU or cross-compiler binary layout mismatches."
    version = 1
}

SafetyReq.FailureMode FM_SD_ServiceNotDiscovered {
    guidewords = [ SafetyReq.Guideword.LossOfFunction ]
    description = "Client proxy fails to discover an offered remote service instance within the configured Initial Wait + Repetition phase."
    failureeffect = "Safety-critical consumer receives no events or RPC responses."
    version = 1
    safety = SafetyReq.Asil.B
    rationale = "Mitigated internally by deterministic TTL/Offer state machine (ASR_SafeServiceDiscovery) and externally by AoU_NoGuaranteeInServiceAvailability requiring consumer timeout supervision."
    interface = "rpc_runtime::Runtime::request_service"
}

SafetyReq.AoU AoU_NoGuaranteeInServiceAvailability {
    description = "The system integrator shall ensure that the absence or sudden termination of a remote service provider is detected by an application-level deadline monitor within the item's FTTI and triggers a safe state."
    safety = SafetyReq.Asil.B
    note = "Service discovery operates over UDP multicast; physical link loss or remote ECU power loss cannot be prevented by local software. Verify via fault-injection test disabling remote Offer frames."
    version = 1
    mitigates = "FM_SD_ServiceNotDiscovered, FM_SD_RemoteProducerSilentTermination"
}
```

---

## 4. Inductive SW-FMEA Workflow with HAZOP Guidewords (ISO 26262-6 Clause 7.4.10)

Follow this 5-step workflow whenever qualifying a new or migrated Rust automotive crate:

1. **Inventory All Public Crate Boundaries (`SWE.2` / `SWE.3`)**:
   - Enumerate every public module, `pub trait`, `pub fn`, and `unsafe extern "C" fn` across the crate (e.g., `Runtime`, `RoutingManager`, `Endpoint`, `ServiceDiscovery`, `PayloadProtector`, `SecurityPolicyFfi`).
2. **Apply the 5 HAZOP Guidewords to Every Public Function**:
   - **`LossOfFunction`**: Function fails to execute, drops a message/subscription silently, or returns `Err` when service should succeed.
   - **`UnintendedFunction`**: Function routes data to the wrong `(ServiceId, InstanceId, MethodId)`, invokes a callback for an unsubscribed event, or frees the wrong slot.
   - **`DelayedFunction` (`EarlyOrLateExecution`)**: Function blocks longer than WCET budget, processes stale TTLs, or invokes a callback before initialization completes.
   - **`PartialFunction` (`PartialOrCorruptedExecution`)**: Function transmits truncated segments, mutates header bytes after CRC computation, or updates only a subset of subscribers.
   - **`ExceedingFunction`**: Function allocates beyond pool quota, invokes callbacks repeatedly for a single event, or allows re-entrant/out-of-context API calls.
3. **Classify Mitigation Ownership (`ASR-*` vs. `AoU-*`)**:
   - Prefer **Internal Prevention/Detection (`ASR-*`)** first (compile-time types, `#[must_use]`, bounded pools, CRC checks).
   - Export an **`AoU-*`** only when the root cause lies outside the software crate's control (OS scheduler priority, hardware clock drift, filesystem integrity, application FTTI safe-state transition). Every `AoU-*` MUST specify how the system integrator verifies it during item integration.
4. **Annotate Rust Source Code & Safety Manual**:
   - Add `# Safety Requirements` (`@satisfies ASR-*`), `# Failure Modes (SW-FMEA)` (`@fmea FM-*`), and `# Assumptions of Use` (`@aou AoU-*`) sections to public rustdoc comments so code and safety analysis stay synchronized.
5. **Run Closed-Loop Validation (`Validate → Fix → Repeat`)**:
   - Execute `scripts/validate_seooc_fmea.py` to verify that every public Rust API boundary is analyzed, every `FM-*` has a valid HAZOP guideword and non-empty mitigation (`ASR-*` or `AoU-*`), and every `AoU-*` has a non-empty `mitigates` target and integrator verification check.

```bash
# 1. Validate an existing TRLC / YAML / Markdown SEooC + SW-FMEA directory or file:
python3 skills/iso26262-seooc-and-sw-fmea/scripts/validate_seooc_fmea.py \
  --fmea-file /path/to/dependability_dir/ \
  --format markdown

# 2. Cross-check a Rust crate's public API boundaries against its SW-FMEA artifact:
python3 skills/iso26262-seooc-and-sw-fmea/scripts/validate_seooc_fmea.py \
  --fmea-file path/to/seooc_fmea.yaml \
  --rust-crate path/to/rust_crate/src \
  --format markdown

# 3. Run the built-in self-test suite:
python3 skills/iso26262-seooc-and-sw-fmea/scripts/validate_seooc_fmea.py \
  --self-test
```

---

## 5. Deep-Dive References

- **[references/seooc_asr_aou_methodology.md](references/seooc_asr_aou_methodology.md)**: ISO 26262-10:2018 Clause 9 SEooC engineering guide and Safety Manual template for automotive Rust middleware crates, including `.trlc`, YAML, and Markdown templates and rustdoc `# Assumptions of Use` conventions.
- **[references/sw_fmea_hazop_guidewords_and_rust_mapping.md](references/sw_fmea_hazop_guidewords_and_rust_mapping.md)**: ISO 26262-6:2018 Clause 7.4.10 SW-FMEA methodology mapping all 5 HAZOP guidewords across middleware subsystems (`runtime`, `routing`, `endpoints`, `service_discovery`, `e2e_black_channel`, `security_ffi`), tracing each `FM-*` to local/system effects, ASIL rating, Rust prevention/detection mechanisms, `ASR-*` / `AoU-*` mitigations, and verification tests.
