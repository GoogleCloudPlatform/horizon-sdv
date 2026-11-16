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

# 1-to-1 C++-to-Rust API Mirror & Deterministic Testkit Patterns

## 1. `#[repr(transparent)]` Wire Enums vs. Strict Rust `enum`

In C++ public headers (e.g., `include/rpc/enumeration_types.hpp`), wire enums like `message_type_e` and `return_code_e` are backed by `uint8_t`, and wire deserializers cast raw bytes directly without rejecting unknown values. Translating these as a strict Rust `#[repr(u8)] pub enum MessageType` causes `try_from(raw_byte)` to reject forward-compatible or negative-test wire values.

```rust
/// Transparent 1-byte wrapper that preserves unknown wire values while providing
/// ergonomic named constants and pattern matching.
#[repr(transparent)]
#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Hash, Default)]
#[doc(alias = "message_type_e")]
pub struct MessageType(pub u8);

impl MessageType {
    pub const REQUEST: Self = Self(0x00);
    pub const REQUEST_NO_RETURN: Self = Self(0x01);
    pub const NOTIFICATION: Self = Self(0x02);
    pub const RESPONSE: Self = Self(0x80);
    pub const ERROR: Self = Self(0x81);
    pub const UNKNOWN: Self = Self(0xFF);
}
```

---

## 2. Scoped Closure Accessor for Mutable Buffers (`get_data_mut`)

In C++, `uint8_t* payload::get_data()` returns a raw mutable pointer into the internal `std::vector<uint8_t>`, which callers mutate in place (`buf->get_data()[0] = 0x42`). Returning `&mut [u8]` from `&self` in Rust is unsound without synchronization, while returning a cloned `Vec<u8>` silently discards caller mutations.

Use a scoped closure accessor (`get_data_mut`) that acquires the internal write lock (materializing `Owned(Vec<u8>)` on copy-on-write storage) for the exact duration of the closure:

```rust
pub trait Payload: Send + Sync {
    /// Read-only snapshot.
    fn get_data(&self) -> Vec<u8>;

    /// Mutates the underlying payload buffer in place without leaking internal lock guards.
    fn get_data_mut(&self, f: &mut dyn FnMut(&mut [u8]));
}
```

---

## 3. Parallel-Safe Resource Namespacing in In-Process `testkit` Crates

When running multiple `cargo test` binaries concurrently (e.g., workspace test runs or parallel verifiers), tests that share default network names (`"bus0"`) collide on `/tmp/bus0.lck` (`fcntl` daemon election) and `/tmp/bus0-0` UDS sockets.

Salt the network name per process and test instance inside the testkit builder:

```rust
use std::sync::atomic::{AtomicUsize, Ordering};

static TEST_NET_COUNTER: AtomicUsize = AtomicUsize::new(1);

pub fn unique_test_network_name(base: &str, node_name: &str) -> String {
    let pid = std::process::id();
    let seq = TEST_NET_COUNTER.fetch_add(1, Ordering::Relaxed);
    format!("{base}_{node_name}_{pid}_{seq}")
}
```

---

## 4. `pollable_fd()` Reactor Seam for Hybrid OS vs. In-Memory Testkit Transports

When production endpoints multiplex sockets onto a single `poll(2)` / `mio` readiness reactor, in-memory `testkit` sockets lack kernel file descriptors. Provide `fn pollable_fd(&self) -> Option<i32>` (returning `None` by default on the trait):

```rust
pub trait TransportSocket: Send + Sync {
    fn pollable_fd(&self) -> Option<i32> {
        None // Overridden by OS Udp/Tcp/Uds sockets to return Some(as_raw_fd())
    }
    fn read_nonblocking(&self, buf: &mut [u8]) -> std::io::Result<usize>;
}
```

Production endpoints register on the shared `poll(2)` reactor whenever `socket.pollable_fd().is_some()`, and fall back to an in-memory blocking reader closure only when `pollable_fd().is_none()` under `testkit`.
