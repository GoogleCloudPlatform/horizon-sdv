---
name: iso26262-ffi-isolation
description: >-
  Designs, implements, and audits ISO 26262-6 Clause 7.4.6 and Annex D Freedom
  from Interference (FFI) across mixed-criticality (QM and ASIL) partitions
  alongside safe Rust-to-C/C++ extern "C" boundaries. Enforces spatial memory
  isolation (bounded slab pools, read-only shm), temporal isolation (poll
  budgets, monotonic watchdogs, fault containment), communication quotas (rate
  limiters, debouncing), and C ABI safety (catch_unwind, repr(C) layout proofs,
  pointer/slice checks, packed access). Use when: partitioning mixed QM/ASIL
  daemons, porting C/C++ plugins or IPC bridges to Rust, or auditing extern "C"
  and Annex D boundaries. Don't use for: pure-Rust Clippy linting (use
  rust-automotive), E2E CRC envelopes (use iso26262-e2e-black-channel), or
  Boost.Asio migration (use cpp-asio-to-rust-migration).
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

# ISO 26262 Freedom from Interference (Annex D) & Rust `extern "C"` FFI Isolation

In safety-critical automotive Rust engineering, **FFI** denotes two intersecting safety boundaries that must both be enforced whenever an ASIL-B/D middleware daemon (such as a SOME/IP routing manager, IPC broker, or DLT daemon) interacts with untrusted QM peers or legacy C/C++ libraries:

1. **ISO 26262-6:2018 Clause 7.4.6 & Annex D — Freedom from Interference (FFI)**: Proving that a cascading fault in a QM or lower-ASIL software element cannot corrupt or starve an ASIL element across **Spatial / Memory** (Annex D.2.2), **Temporal / Execution** (Annex D.2.1), or **Communication / Exchange** (Annex D.2.3 & D.2.4) fault domains. Unless Freedom from Interference is established, ISO 26262-6 Clause 7.4.5 requires all co-existing software elements in the partition to be developed to the **highest ASIL**.
2. **Rust-to-C/C++ Foreign Function Interface (`extern "C"`) Boundary Safety**: Enforcing ABI-level memory, alignment, layout, and unwind isolation across `extern "C"` call boundaries (such as C security policy plugins, packed binary wire headers, shared-memory C++ IPC bridges, and C callback registries) so that neither side triggers Undefined Behavior (UB) in the other.

---

## 1. Quick Start & Automated Boundary Audit

Run the bundled lexical/regex static analyzer `scripts/check_ffi_boundaries.py` (alongside `rustc`, `cargo clippy`, and `cargo miri`) to audit Rust crates and C/C++ automotive repositories for both `extern "C"` ABI hazards (`FFI-ABI-001`..`005`) and ISO 26262-6 Annex D isolation violations (`FFI-ISO-001`..`002`):

```bash
# 1. Run built-in self-test suite against compliant & non-compliant FFI/Annex-D fixtures
python3 skills/iso26262-ffi-isolation/scripts/check_ffi_boundaries.py --self-test

# 2. Audit a Rust/C++ workspace for extern "C" ABI hazards and Annex D isolation defects
python3 skills/iso26262-ffi-isolation/scripts/check_ffi_boundaries.py \
  --path /path/to/crate_or_repo \
  --check-annex-d \
  --strict \
  --format markdown \
  --output /tmp/ffi_isolation_report.md

# 3. Materialize standalone reference fixtures to disk for inspection
python3 skills/iso26262-ffi-isolation/scripts/check_ffi_boundaries.py --dump-fixtures /tmp/ffi_fixtures
```

---

## 2. Pillar 1: ISO 26262-6 Clause 7.4.6 & Annex D Freedom from Interference

Shared C/C++ routing or logging daemons frequently violate Annex D when a central broker serves both QM infotainment clients and ASIL-B chassis/ADAS clients. Enforce the following three isolation domains when architecting or auditing Rust middleware:

### 2.1 Spatial / Memory Isolation (`FFI-ISO-002` — Annex D.2.2)

- **Why Unbounded Reassembly & Receive Buffers Fail**:
  - Storing fragment reassembly state in an unbounded nested `std::map` and eagerly calling `message.resize(header_size + wire_offset, 0)` on out-of-order segments lets a malfunctioning QM peer allocate megabytes of heap memory per spoofed packet, starving ASIL partitions.
  - Leaving `MAX_LOCAL_MESSAGE_SIZE` or `MAX_TCP_MESSAGE_SIZE` at `0` (`unlimited`) lets a local QM stream grow `Vec` buffers until OOM termination crashes the shared daemon.

#### Anti-Pattern: Unbounded Nested Map & Eager Offset Resize
```cpp
// VIOLATION (FFI-ISO-002): Unbounded heap map + eager resize from untrusted wire offset
std::map<IpAddress, std::map<uint16_t, std::map<uint64_t, TpSession>>> reassembly_sessions_;

if (wire_offset != 0) {
    // Single spoofed QM frame with offset = 9 MiB immediately allocates 9 MiB on heap!
    message_buf.resize(HEADER_SIZE + wire_offset, 0x0);
}
```

#### Recommended Pattern: Pre-Allocated Fixed Slab Pool with Per-Partition Slot Quotas
```rust
// Compliant (ISO 26262-6 Annex D.2.2): Zero runtime heap allocation; strict QM vs ASIL slot caps
pub struct TpReassemblyArena<const MAX_SLOTS: usize, const MAX_PDU: usize> {
    slots: [TpSlot<MAX_PDU>; MAX_SLOTS],
    qm_active_slots: usize,
    qm_max_slots: usize, // e.g., QM capped at 12 slots; 4 slots reserved exclusively for ASIL
}

impl<const MAX_SLOTS: usize, const MAX_PDU: usize> TpReassemblyArena<MAX_SLOTS, MAX_PDU> {
    pub fn ingest_segment(
        &mut self,
        partition: PartitionAsil,
        key: TpSessionKey,
        offset: usize,
        payload: &[u8],
    ) -> Result<Option<&[u8]>, SpatialIsolationError> {
        let end = offset.checked_add(payload.len()).ok_or(SpatialIsolationError::OffsetOverflow)?;
        if end > MAX_PDU || (offset % 16 != 0) {
            return Err(SpatialIsolationError::BoundsExceeded);
        }
        if partition == PartitionAsil::Qm && self.qm_active_slots >= self.qm_max_slots {
            return Err(SpatialIsolationError::QmQuotaExhausted); // ASIL reserve remains untouched
        }
        // Copy into pre-allocated fixed slot buffer + update 64-bit coverage bitmap
        let slot = self.acquire_or_find_slot(partition, key)?;
        slot.write_window(offset, payload)
    }
}
```

### 2.2 Temporal / Execution Isolation (`FFI-ISO-001` — Annex D.2.1)

- **Why Process-Killing Macros on Peer Faults Fail**:
  - Calling `sleep_for(...)` + `std::abort()` (or intentional null-pointer `SIGSEGV` traps) inside a shared daemon when a remote QM peer misses an offer/heartbeat blocks the timer thread and terminates the entire broker—severing all ASIL communication.

#### Anti-Pattern: Aborting the Shared Daemon on a Missing Peer Offer
```cpp
// VIOLATION (FFI-ISO-001): Blocking thread + aborting entire process on peer timeout
void check_offer_services(bool timed_out, uint32_t offers_received) {
    if (timed_out && offers_received == 0) {
        PROCESS_TERMINATE("No offer received after resume"); // Sleeps + calls std::abort()!
    }
}
```

#### Recommended Pattern: Per-Client Fault Containment & Poll-Budgeted Execution
```rust
// Compliant (ISO 26262-6 Annex D.2.1): Isolate timed-out peer without halting ASIL routing
pub fn check_post_resume_offers(
    services: &mut ServiceRegistry,
    now: std::time::Instant,
    deadline: std::time::Instant,
) -> WatchdogOutcome {
    if now < deadline {
        return WatchdogOutcome::Pending;
    }
    let mut degraded_qm = 0usize;
    for svc in services.iter_mut() {
        if !svc.offer_received_since_resume {
            svc.state = ServiceHealth::TimedOutDegraded; // Evict/notify only affected subscribers
            degraded_qm += 1;
        }
    }
    WatchdogOutcome::Contained { degraded_services: degraded_qm }
}
```

### 2.3 Communication / Exchange Isolation (`FFI-ISO-002` / `FFI-ISO-003` — Annex D.2.3 & D.2.4)

- **Why Default Unlimited Queues & Shared FIFOs Fail**:
  - Initializing endpoint queue limits to `0` (`QUEUE_SIZE_UNLIMITED`) disables backpressure checks by default.
  - Sharing a single FIFO queue between QM and ASIL traffic lets a burst of QM diagnostic notifications cause head-of-line blocking for ASIL control events.
- **Required Controls**:
  1. **Mandatory Non-Zero Per-Client Byte & Frame Quotas**: Reject `0` / `UNLIMITED` queue configurations at startup (`NonZeroUsize`).
  2. **Priority-Segregated Channels**: Dequeue `asil_rx` before polling `qm_rx` using biased selection and bounded poll budgets so QM floods never delay ASIL frames.
  3. **Token-Bucket Ingress Rate Limiting & Monotonic Debouncing**: Enforce per-client token buckets and monotonic minimum intervals (`std::time::Instant`).

For complete ISO 26262-6 Clause 7.4.6 and Annex D architectural blueprints, fault tables, poll-budgeted futures, and watchdog designs, read **[references/annex_d_freedom_from_interference.md](references/annex_d_freedom_from_interference.md)**.

---

## 3. Pillar 2: Rust-to-C/C++ (`extern "C"`) Boundary Safety

When exposing Rust functions to C/C++ (`#[no_mangle] pub unsafe extern "C" fn` or `#[unsafe(no_mangle)]`) or calling C/C++ functions from Rust (`unsafe extern "C"`), enforce all 5 boundary rules below:

| Rule ID | Hazard | Requirement |
|---|---|---|
| **`FFI-ABI-001`** | Unwinding a Rust panic across `extern "C"` into C/C++ frames is immediate Undefined Behavior. | Wrap every `extern "C" fn` body in `std::panic::catch_unwind(AssertUnwindSafe(|| ...))` and translate panics into explicit C error return codes (`SECURITY_POLICY_PERM_DENIED`, `DLT_RETURN_ERROR`, `false`). |
| **`FFI-ABI-002`** | Dereferencing null/unaligned pointers or calling `slice::from_raw_parts(null, 0)` (which is UB in Rust even when `len == 0`!). | Validate `!ptr.is_null()` and `ptr.is_aligned()` before dereferencing. For byte slices, check `len == 0` **first** (returning `&[]`) before checking `ptr.is_null()` and calling `slice::from_raw_parts`. |
| **`FFI-ABI-003`** | Passing non-`#[repr(C)]` structs/enums or C++ `std::shared_ptr` across `extern "C"`. | Annotate all FFI structs with `#[repr(C)]`, enums with `#[repr(C)]` or `#[repr(u32)]` (validating raw integer discriminants before conversion), and add compile-time `size_of` / `align_of` / `offset_of!` assertions. |
| **`FFI-ABI-004`** | Taking a reference (`&hdr.len`) to a field of a `#[repr(C, packed)]` wire header struct. | Never create `&T` or `&mut T` to packed fields; copy unaligned fields by value (`let len = u16::from_be(hdr.len);`) or use `core::ptr::addr_of!(hdr.len).read_unaligned()`. |
| **`FFI-ABI-005`** | Undocumented `unsafe extern "C" fn` contracts or `unsafe` blocks lacking verification invariants. | Document `# Safety` preconditions on every `pub unsafe extern "C" fn` and precede every `unsafe` block with a `// SAFETY:` justification verifying pointer provenance, alignment, lifetime, and thread safety. |

### 3.1 Panic Barriers & Pointer Validation (`FFI-ABI-001` & `FFI-ABI-002`)

C/C++ callers frequently pass `(NULL, 0)` for empty buffers or invoke `extern "C"` callbacks from non-Rust worker threads where unwinding is immediate Undefined Behavior. In Rust, calling `core::slice::from_raw_parts(ptr, 0)` when `ptr` is `NULL` or unaligned is Undefined Behavior even when `len == 0`. Always validate `len == 0` **before** calling `slice::from_raw_parts`, validate `!ptr.is_null()` and `ptr.is_aligned()`, and isolate the body inside `std::panic::catch_unwind`.

#### Anti-Pattern: Unchecked Pointer Dereference, Null Zero-Length Slice & Uncaught Panic
```rust
// VIOLATION (FFI-ABI-001, FFI-ABI-002, FFI-ABI-005):
// 1. Missing catch_unwind -> panic in assert!/unwrap() unwinds across extern "C" into C!
// 2. Raw pointer `client` dereferenced without .is_null() and .is_aligned() checks!
// 3. slice::from_raw_parts(payload, 0) is UB in Rust when payload == NULL!
#[no_mangle]
pub unsafe extern "C" fn security_policy_authenticate_router_bad(
    client: *const security_policy_client_t,
    payload: *const u8,
    len: usize,
) -> security_policy_acl_result_t {
    let c = unsafe { &*client }; // UB if client is NULL or unaligned!
    let bytes = unsafe { core::slice::from_raw_parts(payload, len) }; // UB if payload is NULL even when len == 0!
    assert!(!bytes.is_empty()); // Panic unwinds across C ABI!
    if c.port == 0 { security_policy_acl_result_t::SECURITY_POLICY_OK } else { security_policy_acl_result_t::SECURITY_POLICY_PERM_DENIED }
}
```

#### Recommended Pattern: Panic Barrier + Null/Alignment/Zero-Length Validation
```rust
/// Authenticates a router connection against the loaded security policy.
///
/// # Safety
/// - `client` must be either `NULL` (rejected with `SECURITY_POLICY_PERM_DENIED`) or a valid,
///   properly aligned pointer to an initialized `security_policy_client_t` valid for the call duration.
/// - `payload` must be valid for reads of `len` bytes when `len > 0`.
#[no_mangle]
pub unsafe extern "C" fn security_policy_authenticate_router(
    client: *const security_policy_client_t,
    payload: *const u8,
    len: usize,
) -> security_policy_acl_result_t {
    let result = std::panic::catch_unwind(std::panic::AssertUnwindSafe(|| {
        if client.is_null() || !client.is_aligned() {
            return security_policy_acl_result_t::SECURITY_POLICY_PERM_DENIED;
        }
        // SAFETY: `client` is verified non-null and aligned to `align_of::<security_policy_client_t>()`.
        let client_ref = unsafe { &*client };

        let slice: &[u8] = if len == 0 {
            &[] // Never pass a potentially NULL pointer to slice::from_raw_parts when len == 0!
        } else {
            if payload.is_null() || len > MAX_SEC_TOKEN_BYTES {
                return security_policy_acl_result_t::SECURITY_POLICY_PERM_DENIED;
            }
            // SAFETY: `payload` is non-null, `u8` has alignment 1, and `len` is bounded.
            unsafe { core::slice::from_raw_parts(payload, len) }
        };

        evaluate_router_credentials(client_ref, slice)
    }));
    result.unwrap_or(security_policy_acl_result_t::SECURITY_POLICY_PERM_DENIED)
}
```

### 3.2 Packed Wire Structs & Opaque Plugin Handles (`FFI-ABI-003` & `FFI-ABI-004`)

C wire headers declared with `__attribute__((aligned(1), packed))` map to `#[repr(C, packed)]` in Rust, which reduces struct alignment to `1`. Taking a reference `&hdr.len` to a `u16`/`u32` field creates an unaligned reference, which is immediate Undefined Behavior (and rejected as hard compiler error `rustc E0793`).

#### Anti-Pattern: Unaligned Reference to `#[repr(C, packed)]` Field & Returning `std::shared_ptr` Across `extern "C"`
```rust
// VIOLATION (FFI-ABI-004): Taking a reference to a packed struct field is instant UB
#[repr(C, packed)]
pub struct LoggingStandardHeader {
    pub htyp: u8,
    pub mcnt: u8,
    pub len: u16,
}

pub fn read_dlt_len_bad(hdr: &LoggingStandardHeader) -> u16 {
    let len_ref: &u16 = &hdr.len; // UB: unaligned reference to packed field!
    u16::from_be(*len_ref)
}
```

#### Recommended Pattern: Unaligned Value Read (`addr_of!`) & Compile-Time Layout Proofs
```rust
// Compliant (FFI-ABI-003 & FFI-ABI-004): Unaligned read via addr_of! + static layout assertions
#[repr(C, packed)]
#[derive(Copy, Clone)]
pub struct LoggingStandardHeader {
    pub htyp: u8,
    pub mcnt: u8,
    pub len_be: u16,
}

const _: () = {
    assert!(core::mem::size_of::<LoggingStandardHeader>() == 4);
    assert!(core::mem::align_of::<LoggingStandardHeader>() == 1);
    assert!(core::mem::offset_of!(LoggingStandardHeader, len_be) == 2);
};

pub fn read_dlt_len_safe(hdr: &LoggingStandardHeader) -> u16 {
    // SAFETY: `addr_of!` computes the field pointer without creating an intermediate reference,
    // and `read_unaligned` safely loads the 2-byte big-endian integer from alignment 1.
    let raw_be = unsafe { core::ptr::addr_of!(hdr.len_be).read_unaligned() };
    u16::from_be(raw_be)
}
```

For complete `#[repr(C)]` bindings, safe slice/string conversion helpers (`slice_from_c_bytes`, `cstr_from_ptr_bounded`), and opaque plugin vtables, read **[references/rust_c_abi_boundary_safety.md](references/rust_c_abi_boundary_safety.md)**.

---

## 4. End-to-End Workflow: Validate → Fix → Repeat

1. **Inventory Cross-Partition & Cross-Language Boundaries**:
   - Identify every IPC/network ingress point where QM and ASIL partitions share memory, queues, or threads (see [references/annex_d_freedom_from_interference.md](references/annex_d_freedom_from_interference.md)).
   - Identify every `extern "C"` export (`#[no_mangle]`, `#[export_name]`) and import (`unsafe extern "C" { ... }`) (see [references/rust_c_abi_boundary_safety.md](references/rust_c_abi_boundary_safety.md)).
2. **Run Static FFI & Annex D Audit**:
   ```bash
   python3 skills/iso26262-ffi-isolation/scripts/check_ffi_boundaries.py --path <target_dir> --check-annex-d
   ```
3. **Remediate Findings by Rule ID**:
   - **`FFI-ABI-001`**: Wrap `extern "C" fn` bodies in `ffi_boundary!` / `std::panic::catch_unwind`.
   - **`FFI-ABI-002`**: Replace raw `slice::from_raw_parts` and `*ptr` dereferences with `slice_from_c_bytes` and `ref_from_c_ptr` validating null, alignment, zero-length, and upper bounds.
   - **`FFI-ABI-003`**: Add `#[repr(C)]` and compile-time `size_of` / `align_of` / `offset_of!` assertions; replace C++ `std::shared_ptr` / Rust `String` across FFI with opaque handle tables or `#[repr(C)]` vtables.
   - **`FFI-ABI-004`**: Replace `&packed.field` with `core::ptr::addr_of!(packed.field).read_unaligned()`.
   - **`FFI-ABI-005`**: Add `# Safety` rustdoc headers and `// SAFETY:` block comments.
   - **`FFI-ISO-001`**: Replace `PROCESS_TERMINATE`, `std::abort()`, `std::exit()`, `process::abort()`, and `LOG_FATAL_RESET_TRAP` in peer/protocol handlers with per-client error containment and watchdog state transitions.
   - **`FFI-ISO-002` / `FFI-ISO-003`**: Replace `QUEUE_SIZE_UNLIMITED`, `MAX_*_MESSAGE_SIZE = 0`, `mpsc::unbounded_channel`, and unbounded reassembly maps with fixed-capacity slab pools, priority-segregated channels, and token-bucket rate limiters.
4. **Re-Verify Until Zero Unmitigated Violations Remain**:
   ```bash
   python3 skills/iso26262-ffi-isolation/scripts/check_ffi_boundaries.py --path <target_dir> --check-annex-d --strict
   ```

---

## 5. Deep-Dive References

- **[references/annex_d_freedom_from_interference.md](references/annex_d_freedom_from_interference.md)** — ISO 26262-6 Clause 7.4.6 & Annex D engineering reference mapping Spatial (D.2.2), Temporal (D.2.1), and Communication (D.2.3/D.2.4) fault effects to production-ready Rust isolation implementations.
- **[references/rust_c_abi_boundary_safety.md](references/rust_c_abi_boundary_safety.md)** — Rust-to-C/C++ `extern "C"` boundary safety guide with `ffi_boundary!` macros, safe pointer/slice/CStr conversion helpers, `#[repr(C)]` C-ABI bindings, opaque plugin vtables, and compile-time layout proofs.
