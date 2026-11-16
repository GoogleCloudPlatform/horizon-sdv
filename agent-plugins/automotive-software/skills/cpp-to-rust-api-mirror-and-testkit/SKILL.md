---
name: cpp-to-rust-api-mirror-and-testkit
description: >-
  Designs and verifies 1-to-1 C++-to-Rust public API mirrors, compile-only
  signature lock tests, and deterministic in-process fake-socket/timer network
  testkits so upstream C++ unit, fake-socket, and multi-process integration test
  suites transliterate into Rust without control flow changes. Use when:
  translating C++ public header trees into a source-compatible Rust facade,
  resolving C++ method overloads and shared_ptr callbacks deterministically,
  preserving raw wire enum bytes via #[repr(transparent)] newtypes, or building
  a deterministic in-memory multi-node testkit to replace flaky Boost.Asio GTest
  suites. Don't use for: extern "C" ABI shims (use iso26262-ffi-isolation),
  general Clippy safety lints (use rust-automotive), or post-parity hot-path
  performance tuning (use systems-perf-parity-and-optimization).
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

# C++-to-Rust 1-to-1 API Mirror & Deterministic Testkit (`cpp-to-rust-api-mirror-and-testkit`)

Establishes a deterministic, mechanical mapping from an object-oriented C++ public API to a safe Rust API mirror, paired with an in-process deterministic network `testkit`, aligned with **[Google's FOSS Rust Style Guide](https://google.github.io/styleguide/rustguide.html)** ([`#newtypes-versus-type-aliases`](https://google.github.io/styleguide/rustguide.html#newtypes-versus-type-aliases), [`#more-on-accessors-that-accept-closures`](https://google.github.io/styleguide/rustguide.html#more-on-accessors-that-accept-closures)).

## Quick Start

| Goal | Command |
|---|---|
| **Run built-in API mirror auditor self-test** | `python3 skills/cpp-to-rust-api-mirror-and-testkit/scripts/audit_api_mirror.py --self-test` |
| **Audit C++ public header coverage, `api_signatures.rs` & wire enums** | `python3 skills/cpp-to-rust-api-mirror-and-testkit/scripts/audit_api_mirror.py --cpp-interface-dir include --rust-facade-dir crates --architecture-doc docs/ARCHITECTURE.md --strict` |

## Why 1-to-1 API Mirroring Must Precede Idiomatic Redesign

When porting a large C++ library or daemon to Rust, redesigning the public API into "idiomatic Rust" (e.g., async traits, builder chains, newtype IDs everywhere) during the initial port creates a fatal verification gap: **upstream's existing integration tests, network test suites, and sample applications can no longer be transliterated mechanically.** Rewriting test logic from scratch to fit a new API invalidates the tests as an independent oracle.

Instead, enforce a two-layer strategy:
1. **Layer 1 (Class A — Mandatory Through Parity)**: A mechanical 1-to-1 Rust API mirror where an application or test ported from C++ to Rust requires **zero control-flow or state-machine changes**.
2. **Layer 2 (Class C — Optional Post-Parity)**: An additive, opt-in idiomatic wrapper layered on top after functional parity is locked.

---

## Pillar 1: Mechanical C++-to-Rust API Translation Rules

Apply this fixed mapping table across every public header (`include/**/*.hpp` / `interface/**/*.hpp`):

| C++ Construct | Rust Mirror Rule | Rationale |
|---|---|---|
| **Method & Constant Names** | Exact C++ `snake_case` / `SCREAMING_SNAKE_CASE` names | 1-to-1 call-site transliteration in ported apps and tests |
| **Types & Enum Variants** | `UpperCamelCase` + `#[doc(alias = "cpp_snake_name")]` | Satisfies Rust naming lints while preserving searchable C++ symbol names |
| **Primitive ID Typedefs** (`service_t`, `instance_t`, `method_t`) | Plain type aliases (`pub type ServiceId = u16;`) | Allows upstream test arithmetic, bitmasks, and hex literals (`0x1234`, `ANY_SERVICE`) without `.0` unwrapping noise |
| **Default Arguments** | Explicit parameters (or `_ext` overload suffix when rarely used) | Rust has no default function arguments; keeps parameter order identical |
| **`const` / Non-`const` Overload Pair** | `name(&self)` and `name_mut(&mut self)` | Standard Rust mutability split |
| **Arity / Type Overloads** | Deterministic suffix taxonomy: `_ext`, `_sec`, `_state`, `_with_path`, `_owned` | Eliminates ad-hoc naming divergence across work packages |
| **Abstract Classes (`class application`)** | `pub trait Application: Send + Sync` behind `Arc<dyn Application>` | Matches `std::shared_ptr<application>` thread-safe sharing |
| **`std::shared_ptr<T>` Parameters & Returns** | `&Arc<dyn T>` parameter; `Arc<dyn T>` (or `Option<Arc<dyn T>>` if nullable) return | Avoids unnecessary `Arc::clone` at call sites while representing nullability explicitly |
| **Mutable Setters on Shared Objects** | `&self` methods backed by internal synchronization | Callers holding `Arc<dyn Application>` or `Arc<dyn Message>` can mutate state without `Arc::get_mut` |
| **Raw Mutable Buffer Pointer (`uint8_t* get_data()`)** | Closure accessor: `fn get_data_mut(&self, f: &mut dyn FnMut(&mut [u8]))` | Prevents leaking internal lock guards or invalidating copy-on-write buffers across threads |
| **Wire / IPC Byte Enums (`enum class message_type_e : uint8_t`)** | `#[repr(transparent)] pub struct MessageType(pub u8);` + associated `pub const` | C++ casts raw `uint8_t` wire bytes directly to enum types; a strict Rust `enum` rejects unknown bytes and breaks round-trip wire fidelity |
| **Ordered Containers (`std::set`, `std::map`)** | `BTreeSet<T>`, `BTreeMap<K, V>` | Preserves deterministic iteration order relied upon by C++ tests |
| **Blocking Event Loop (`app->start()`)** | Synchronous `fn start(&self)` that blocks caller thread | Matches `boost::asio::io_context::run()` lifecycle; `stop(&self)` is non-blocking from any callback |

---

## Pillar 2: Compile-Only Signature Lock (`tests/api_signatures.rs`)

Document every C++ header symbol and its Rust counterpart in a markdown table in `docs/ARCHITECTURE.md`, and lock the signatures with a compile-only integration test (`crates/<facade>/tests/api_signatures.rs`) that binds every trait method and type alias:

```rust
// crates/rpc-facade/tests/api_signatures.rs
use std::sync::Arc;
use rpc_facade::{Application, Message, Payload, Runtime, ServiceId, InstanceId};

#[allow(dead_code)]
fn verify_application_signatures(
    rt: &Arc<dyn Runtime>,
    app: &Arc<dyn Application>,
    msg: &Arc<dyn Message>,
    payload: &Arc<dyn Payload>,
) {
    let _created: Arc<dyn Application> = rt.create_application("app");
    let _init: bool = app.init();
    app.offer_service(0x1234 as ServiceId, 0x0001 as InstanceId, 0, 0);
    app.send(msg);
    payload.get_data_mut(&mut |bytes: &mut [u8]| {
        if let Some(first) = bytes.first_mut() {
            *first = 0xFF;
        }
    });
}
```

---

## Pillar 3: Deterministic In-Process Network Testkit (`testkit` Pattern)

Upstream C++ networking suites that use fake sockets or multi-node simulations often suffer from timing races, global singletons, and OS resource collisions. Replace raw Boost.Asio socket mocks with a dedicated `*-testkit` crate that obeys five design rules:

1. **In-Memory Virtual Network Bus (`SocketManager`)**:
   - Routes virtual UDS streams, TCP connections, UDP unicast/multicast datagrams, and Netlink interface state transitions entirely in memory between named virtual nodes (`node_1`, `node_2`).
2. **Process- & Counter-Namespaced OS Paths**:
   - Whenever a test exercises real filesystem lockfiles or UDS paths (e.g., `/tmp/{network}.lck`), automatically salt the network name in test setup with `std::process::id()` and a monotonic `AtomicUsize` counter (`{network}_{name}_{pid}_{counter}`) so parallel `cargo test` processes never collide on `/tmp/*.lck`.
3. **Strict Socket $\rightarrow$ Registry Lock Ordering**:
   - Individual fake sockets lock their own state (`FakeSocket::inner`) before querying `SocketManager::state`. Whenever `SocketManager` tears down listeners or closes pipes, it **must** clone the target `Arc<FakeSocket>` handles into a local `Vec` and drop `SocketManager::state` *before* locking `FakeSocket::inner`.
4. **`pollable_fd() -> Option<RawFd>` Reactor Seam**:
   - Expose `fn pollable_fd(&self) -> Option<RawFd>` on abstract socket traits: OS sockets return `Some(fd)` so `poll(2)`/`mio` reactors multiplex them on a shared thread, while in-memory `testkit` sockets return `None` and transparently use an in-memory fallback reader closure without dummy OS file descriptors.
5. **Copy-on-Write `Bytes` / `Owned(Vec<u8>)` Internal Backing Behind Frozen Mirror Traits**:
   - Back `PayloadImpl` internally with `RwLock<PayloadState { storage: Shared(Bytes) | Owned(Vec<u8>), capacity }>` plus crate-internal `with_slice` / `as_bytes` accessors so routing and `equals()` run with zero heap allocations while preserving observable C++ `std::vector::capacity()` unit tests.

---

## Bundled API Mirror Auditor (`scripts/audit_api_mirror.py`)

Run the bundled auditor to verify that public C++ headers, the `docs/ARCHITECTURE.md` mapping table, and the Rust facade crate stay synchronized, and that wire/IPC byte enums use `#[repr(transparent)]` newtypes:

```bash
python3 skills/cpp-to-rust-api-mirror-and-testkit/scripts/audit_api_mirror.py \
  --cpp-interface-dir include \
  --rust-facade-dir crates \
  --architecture-doc docs/ARCHITECTURE.md \
  --strict
```

### Supported CLI Flags
- `--cpp-interface-dir <dir>`: Directory containing upstream C++ public headers.
- `--rust-facade-dir <dir>`: Directory containing Rust crates implementing the mirror API.
- `--architecture-doc <file>`: Path to `ARCHITECTURE.md` containing the C++ $\rightarrow$ Rust mapping table.
- `--format {markdown,json,text}`: Output format (default: `markdown`).
- `--strict`: Exit `1` if any public C++ header is unmapped, `api_signatures.rs` is missing, or a wire byte enum uses a fragile strict `#[repr(u8)] enum` without unknown-byte preservation.
- `--self-test`: Runs built-in self-tests against temporary C++/Rust mirror fixtures.

---

## Deep-Dive References

- **[api_mirror_and_testkit_patterns.md](references/api_mirror_and_testkit_patterns.md)**: Complete Rust patterns for `#[repr(transparent)]` wire enums (`MessageType`, `ReturnCode`), `get_data_mut` closure accessors, parallel-safe resource namespacing, and `pollable_fd()` reactor seams.
