---
name: cpp-asio-to-rust-migration
description: >-
  Translates multi-threaded C++14/C++17 Boost.Asio networking (io_context,
  steady_timer, strand, enable_shared_from_this callback chains, UDP/TCP/Unix
  domain endpoints) and re-entrant lock hierarchies (std::mutex,
  std::recursive_mutex) into deadlock-free, cancel-safe Rust (poll/mio readiness
  reactors or tokio bounded actors, split read/write tasks, and timer wheels).
  Use when: porting C++ Boost.Asio daemons or automotive RPC/IPC middleware to
  async or event-driven Rust, eliminating recursive mutexes and lock-order
  inversions, or auditing C++ concurrency hotspots prior to migration. Don't use
  for: ISO 26262 Freedom from Interference or extern "C" FFI boundary checks
  (use iso26262-ffi-isolation) or general safety-critical Rust Clippy lints (use
  rust-automotive).
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

# C++ Boost.Asio to Rust Migration (`cpp-asio-to-rust-migration`)

Translates multi-threaded C++14/C++17 `boost::asio` networking and fine-grained mutex hierarchies into deadlock-free, cancel-safe Rust (`poll(2)` / `mio` readiness reactors or `tokio` actors), aligned with **[Google's FOSS Rust Style Guide](https://google.github.io/styleguide/rustguide.html)** ([`#avoid-rc-and-arc`](https://google.github.io/styleguide/rustguide.html#avoid-rc-and-arc), [`#dont-start-new-threads-through-the-rust-standard-library-chrome`](https://google.github.io/styleguide/rustguide.html#dont-start-new-threads-through-the-rust-standard-library-chrome), [`#dont-use-async--await-chrome`](https://google.github.io/styleguide/rustguide.html#dont-use-async--await-chrome)) and **[Comprehensive Rust](https://google.github.io/comprehensive-rust/)**.

## Quick Start

| Goal | Command |
|---|---|
| **Run built-in Boost.Asio analyzer self-test** | `python3 skills/cpp-asio-to-rust-migration/scripts/analyze_asio_concurrency.py --self-test` |
| **Inventory C++ `io_context`, timers & `recursive_mutex` hazards** | `python3 skills/cpp-asio-to-rust-migration/scripts/analyze_asio_concurrency.py --root /path/to/cpp/src --format markdown --output /tmp/asio_inventory.md` |
| **Audit resulting Rust reactor, thread & lock topology (`1–10`)** | `python3 skills/rust-systems-hardening-audit/scripts/audit_rust_architecture.py --workspace /path/to/rust-workspace` |

## Why Direct 1:1 Line Translation Fails

Porting C++ Boost.Asio classes 1:1 into `Arc<Mutex<EndpointState>>` with `tokio::spawn` reproduces every C++ concurrency hazard and introduces new Rust async deadlocks:

1. **Callback Lock-Order Inversion**: Downward calls (`EndpointManager` $\to$ `ClientEndpoint::send`) lock manager state then endpoint state, while upward error callbacks (`send_cbk` $\to$ `notify_disconnect()` $\to$ `on_disconnect()`) execute inside the endpoint lock and call back into the manager. C++ workarounds use `std::recursive_mutex` and manual `lock.unlock()` before callbacks, creating Time-of-Check to Time-of-Use (TOCTOU) state corruption.
2. **Guard Held Across `.await` Deadlocks**: Translating `std::recursive_mutex` to `tokio::sync::Mutex` and calling `.lock().await` inside a nested helper function on the same task deadlocks immediately because `tokio::sync::Mutex` is non-reentrant. Holding `std::sync::MutexGuard` across `.await` either fails to compile (`!Send`) or deadlocks worker threads when a task yields.
3. **Completion vs. Poll Readiness & Cancel Safety**: Boost.Asio is proactor/completion-based (a buffer handed to `async_receive_from` or `async_write` stays owned by the reactor until the handler runs). Rust futures are poll-based and cancelled by dropping them in `tokio::select!`. Using non-cancel-safe futures (`AsyncReadExt::read_exact`, `AsyncWriteExt::write_all`) inside `tokio::select!` silently drops partially read/written frames.
4. **`shared_from_this()` Zombie Lifecycles & Self-Join Deadlocks**: Passing `shared_from_this()` into `steady_timer::async_wait` extends object lifetimes past `stop()` and creates reference cycles. Joining dispatcher threads from inside a callback running on a dispatcher thread causes self-join deadlocks.

---

## Migration Workflow (`Inventory -> Partition -> Translate -> Verify`)

### Step 1: Run the Static Concurrency & Asio Inventory Script

Before refactoring any C++ subsystem, run the bundled analyzer to inventory every `boost::asio` primitive, `enable_shared_from_this` lifecycle, `std::recursive_mutex`, and manual `unlock()` lock-inversion hazard:

```bash
python3 skills/cpp-asio-to-rust-migration/scripts/analyze_asio_concurrency.py \
  --root /path/to/cpp/src \
  --format markdown \
  --output /tmp/asio_concurrency_inventory.md
```

Review the generated report in three priority tiers:
- **Tier 1 (Critical Re-Entrancy Hotspots)**: Classes containing `std::recursive_mutex` and manual `.unlock()` calls before upward callbacks.
- **Tier 2 (Lifecycle & Timer Cycles)**: Classes inheriting `std::enable_shared_from_this<T>` that bind `shared_from_this()` into `steady_timer::async_wait` or `strand` dispatches.
- **Tier 3 (Transport & Framing Primitives)**: UDP unicast/multicast sockets (`ip::udp::socket`), TCP stream framing (`ip::tcp::socket`), and Unix Domain Sockets (`local::stream_protocol`).

### Step 2: Select Target Runtime (`poll(2)`/`mio` Readiness Reactor vs. `tokio` vs. `no_std`)

> [!IMPORTANT]
> **Avoid the "Tokio + Blocking Threads" Hybrid Trap**: When porting a C++ library whose public API mirror is **synchronous** (`app.start()` blocks the caller thread, `send()`/`offer()` are non-`async` `&self` methods), introducing `tokio` often causes teams to mix an underused `tokio` runtime with $O(N)$ blocking OS reader/writer threads and aspirational `ARCHITECTURE.md` claims. Pick **one** coherent runtime model upfront and keep `ARCHITECTURE.md` 100% synchronized with `src/`:

| Criterion | `poll(2)` / `mio` Shared Readiness Reactors (**Recommended for Sync C++ API Mirrors & Embedded Daemons**) | `tokio` Async Executor (Greenfield Async-Native Services) | Strict ASIL / `no_std` / RTOS Event Loop |
|---|---|---|---|
| **Execution Model** | Process-wide non-blocking `poll(2)`/`epoll` reactor threads + slot-0 `eventfd` wakeup + completion `IoContext` | Multi-threaded work-stealing or `current_thread` async executor (`tokio::select!`, `JoinSet`) | Single-threaded, allocation-free `mio::Poll` readiness loop |
| **Thread & Dep Footprint** | **`O(1)` threads** across dozens of peers, **`0.0/s` idle wake-ups**, lower RSS, **`0` `tokio` crates** (POSIX `sigwait` for signals) | Fixed worker pool; pulls in `tokio` + `tokio-util` + `mio` dependency graph | Zero runtime heap allocation after `init()` |
| **Best Fit** | 1-to-1 C++ middleware/daemon ports with synchronous public APIs and strict binary/RSS budgets | Async-first Rust services/gateways where public traits are `async fn` | `ASIL-B/D` safety partitions where dynamic future state machines are restricted |
| **Timer & Signal Strategy** | Shared `TimerReactor` min-heap (`std::time::Instant`) + POSIX `pthread_sigmask` + `sigwait` | `tokio::pin!(Sleep)` + `.as_mut().reset()`, `Interval`, or `DelayQueue` | Monotonic `Instant` min-heap driving `Poll::poll` timeout |

### Step 3: Replace Lock Hierarchies & `shared_from_this()` with Clean Ownership

1. **Eliminate `std::recursive_mutex` via Exclusive Actor Ownership or Strict Lock Tiers**: Either move mutable endpoint/routing tables into a single actor task fed by bounded `mpsc::channel(CAPACITY)`, or enforce a numbered N-tier lock hierarchy where upward callbacks are invoked only after dropping all locks.
2. **Split Stream Sockets or Multiplex via Non-Blocking Queues**: In `tokio`, split streams with `TcpStream::into_split()`. In `poll(2)`/`mio` reactors, pair non-blocking `send()` with a bounded `VecDeque` (`front_offset: usize`) and `POLLOUT` registration under the endpoint's leaf lock.
3. **Replace `shared_from_this()` with `Arc::new_cyclic` or `CancellationToken`**: Construct self-referential `Arc` types via `Arc::new_cyclic(|weak| ...)` with a plain lock-free `Weak<Self>` (never `Mutex<Weak<Self>>`), and store write-once `init()` handles in `std::sync::OnceLock`.

---

## Key Translation Patterns (`Anti-Pattern` vs. `Recommended Pattern`)

### Pattern 1: Ban on `ReentrantMutex<RefCell<T>>` & The Collect-Before-Callback Rule

**Never** translate `std::recursive_mutex` into `parking_lot::ReentrantMutex<RefCell<T>>`: `ReentrantMutex::lock()` yields `&T`, and if a callback re-enters on the same call stack and calls `.borrow_mut()` while an outer `RefMut` is alive, Rust **panics at runtime with `BorrowMutError`**. Instead, clone the handler `Arc` (or collect pending actions into a `SmallVec`) under a standard non-recursive lock and **drop the guard before invoking the callback**:

```rust
use std::collections::HashMap;
use std::sync::{Arc, Mutex};

pub type MessageHandler = Arc<dyn Fn(&[u8]) + Send + Sync>;

pub struct HandlerTable {
    handlers: Mutex<HashMap<(u16, u16, u16), MessageHandler>>,
}

impl HandlerTable {
    pub fn dispatch(&self, service: u16, instance: u16, method: u16, payload: &[u8]) {
        let handler_opt = {
            let guard = self.handlers.lock().unwrap_or_else(|e| e.into_inner());
            guard.get(&(service, instance, method)).cloned()
        };
        if let Some(handler) = handler_opt {
            handler(payload); // Zero locks held across user callback
        }
    }
}
```

### Pattern 2: Cancel-Safe TCP Framing & Pinned Resettable Timers in `tokio::select!`

`AsyncReadExt::read_exact` is **not** cancel-safe inside `tokio::select!` (dropping it mid-header discards already-read bytes). Always use `reader.read_buf(&mut read_buf)` with `bytes::BytesMut` and a `tokio::pin!(sleep)` reset via `sleep.as_mut().reset(deadline)`.

### Pattern 3: Non-Blocking Write Re-Entrancy & Registry-to-Socket Lock Ordering

1. **Drop Queue Lock Before Disconnect Escalation**: When a non-blocking `try_write` / `writev` fast path hits `BrokenPipe` or `ConnectionReset`, clear the queue, drop the queue lock, and only then invoke upward `on_disconnect` handlers so re-entrant `stop()`/`send()` calls cannot deadlock.
2. **Socket $\rightarrow$ Registry Lock Order**: When tearing down all sockets in a `SocketManager`, clone target `Arc<Socket>` handles into a local `Vec` under the registry lock, drop the registry lock, and only then lock individual sockets.
3. **Stale-FD Eviction on Rapid UDS/TCP Reconnects**: In custom `poll(2)` reactors, evict stale entries with the same raw `fd` on `register()`, and verify `endpoint.pollable_fd() == Some(entry.fd)` and `(revents & libc::POLLNVAL) == 0` after dropping the registry lock and *before* calling `read_some()`.

### Pattern 4: Dual Endianness (Big-Endian Network Wire vs. Native-Endian Local IPC)

C++ daemons often use **Big-Endian** (`from_be_bytes`/`to_be_bytes`) for external network frames while serializing internal Unix Domain Socket IPC headers via `std::memcpy` in **Host / Native Byte Order** (`from_ne_bytes`/`to_ne_bytes`). Verify endianness per framing layer against differential C++ `.bin` fixtures.

---

## Deep-Dive References

- **[asio_to_tokio_mio_mapping.md](references/asio_to_tokio_mio_mapping.md)**: Primitive mapping table (`io_context`, `strand`, `steady_timer`, UDP/TCP/UDS sockets, `socket2` multicast & `SO_RCVBUFFORCE` / `SO_BINDTODEVICE`), `tokio::select!` cancel-safety matrix, `DelayQueue` timer wheels, dual-endianness wire vs. local IPC framing, UDS `peer_cred()`, and `serde` hex-or-decimal JSON config deserializers.
- **[lock_hierarchy_and_lifecycle_refactoring.md](references/lock_hierarchy_and_lifecycle_refactoring.md)**: Refactoring blueprints for `std::recursive_mutex` elimination, `enable_shared_from_this` lifecycles, and resolving callback lock-order inversions, TOCTOU races, circular `shared_ptr` timers, and dispatcher self-join deadlocks.
