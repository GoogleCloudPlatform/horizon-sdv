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

# Post-Parity Rust Systems Hardening Playbook

Concrete before/after refactoring blueprints for turning a functionally equivalent 1-to-1 C/C++ port into idiomatic, embedded-grade Rust (`6.0 / 10` → `9.5 / 10`), aligned with **[Google's FOSS Rust Style Guide](https://google.github.io/styleguide/rustguide.html)** and **[Rust API Guidelines](https://rust-lang.github.io/api-guidelines/)**.

---

## 1. Dropping Vestigial `tokio` in Synchronous Daemons via POSIX `sigwait` (`WP-A`)

### Problem
When a C++ daemon's public API is synchronous (`app.start()` blocks caller thread) and sockets run on `poll(2)` / `mio` reactors, keeping `tokio` in `[dependencies]` solely for `tokio::signal::unix::signal` inflates the normal dependency graph and binary size while tempting binary roots to call `unsafe` libc APIs directly.

### Solution (`src/sys/daemon.rs` + `#![forbid(unsafe_code)]` in `src/bin/daemon.rs`)
1. Quarantine `sigemptyset`, `sigaddset`, `pthread_sigmask(SIG_BLOCK)`, and `sigwait` inside `core::sys::signal` with `// SAFETY:` proofs.
2. Call `SignalSetWaiter::block_daemon_signals()` on `main()` **before** spawning any worker threads so all threads inherit the blocked signal mask.
3. Spawn the signal-waiter thread via checked OS thread creation; if `pthread_create` fails, stop the application and return non-zero immediately so the daemon never runs deaf to `SIGTERM`.

```rust
// In crates/core/src/sys/daemon.rs (#[allow(unsafe_code)] inside audited mod sys):
use std::io;

pub struct SignalSetWaiter {
    set: libc::sigset_t,
}

impl SignalSetWaiter {
    /// Blocks `SIGINT`, `SIGTERM`, `SIGUSR1`, and `SIGUSR2` on the calling thread.
    /// Must be called on `main` BEFORE spawning any worker threads.
    pub fn block_daemon_signals() -> io::Result<Self> {
        let mut set = std::mem::MaybeUninit::<libc::sigset_t>::uninit();
        // SAFETY: `set` points to valid stack memory for `sigset_t`; signal constants are valid POSIX signals.
        let set = unsafe {
            if libc::sigemptyset(set.as_mut_ptr()) != 0 {
                return Err(io::Error::last_os_error());
            }
            for sig in [libc::SIGINT, libc::SIGTERM, libc::SIGUSR1, libc::SIGUSR2] {
                if libc::sigaddset(set.as_mut_ptr(), sig) != 0 {
                    return Err(io::Error::last_os_error());
                }
            }
            let initialized = set.assume_init();
            if libc::pthread_sigmask(libc::SIG_BLOCK, &raw const initialized, std::ptr::null_mut()) != 0 {
                return Err(io::Error::last_os_error());
            }
            initialized
        };
        Ok(Self { set })
    }

    pub fn wait_next(&self) -> io::Result<i32> {
        let mut sig: libc::c_int = 0;
        // SAFETY: `self.set` was initialized via `sigemptyset`/`sigaddset` and `sig` is a valid out-pointer.
        let rc = unsafe { libc::sigwait(&raw const self.set, &raw mut sig) };
        if rc != 0 {
            return Err(io::Error::from_raw_os_error(rc));
        }
        Ok(sig)
    }
}
```

---

## 2. Three-Class `let _ =` Triage & Checked Thread Spawning (`WP-B`)

Grounded in [Google Rust Style Guide `#reporting-errors`](https://google.github.io/styleguide/rustguide.html#reporting-errors) and [`#must_use-on-functions`](https://google.github.io/styleguide/rustguide.html#must_use-on-functions):

1. Enable `let_underscore_must_use = "deny"` in `[workspace.lints.clippy]`.
2. Add `#[must_use]` (or convert `-> bool` / `-> u32` to `Result<T, DomainError>`) on internal fallible helpers so `let _ = helper()` cannot silently bypass the lint.
3. Classify every `let _ =` site into one of three explicit buckets:
   - **Class (i) — Intentional Upstream Parity Discard**: Upstream C++ explicitly ignores the error (e.g., best-effort `unlink` before `bind`). Keep `let _ =` with both an upstream `file:line` comment and `#[expect(clippy::let_underscore_must_use, reason = "Ignored as upstream (local_server.cpp:94)")]`.
   - **Class (ii) — Observable Diagnostic**: Log at `debug!` / `warn!` (`if let Err(e) = sock.set_nodelay(true) { log_debug!("TCP_NODELAY failed: {e}"); }`).
   - **Class (iii) — Propagate or Roll Back**: Propagate with `?`, or for OS thread creation route through `spawn_worker_with_rollback` and roll back `AtomicBool` running state on failure:

```rust
use std::io;
use std::sync::atomic::{AtomicBool, Ordering};
use std::thread::{self, JoinHandle};

pub fn spawn_worker_with_rollback<F>(
    name: &str,
    stack_bytes: usize,
    running_flag: &AtomicBool,
    f: F,
) -> io::Result<JoinHandle<()>>
where
    F: FnOnce() + Send + 'static,
{
    running_flag.store(true, Ordering::Release);
    match thread::Builder::new().name(name.into()).stack_size(stack_bytes).spawn(f) {
        Ok(handle) => Ok(handle),
        Err(err) => {
            running_flag.store(false, Ordering::Release);
            Err(err)
        }
    }
}
```

---

## 3. Replacing `saturating_*` with Checked Wire Cursor Helpers (`WP-B`)

Never use `.saturating_add()` / `.saturating_sub()` / `.saturating_mul()` on wire, IPC, segmentation, or Netlink offsets to appease `clippy::arithmetic_side_effects`. Saturation clamps `usize::MAX + 8` to `usize::MAX` or underflows to `0`, hiding protocol framing bugs.

```rust
#[derive(Debug, Clone, Copy, PartialEq, Eq, thiserror::Error)]
pub enum CodecError {
    #[error("frame truncated at offset {offset} (needed {needed}, len {actual})")]
    Truncated { offset: usize, needed: usize, actual: usize },
    #[error("wire length or cursor arithmetic overflow")]
    ArithmeticOverflow,
}

#[inline]
pub fn read_wire_slice<'a>(
    buf: &'a [u8],
    cursor: &mut usize,
    len: usize,
) -> Result<&'a [u8], CodecError> {
    let start = *cursor;
    let end = start.checked_add(len).ok_or(CodecError::ArithmeticOverflow)?;
    let slice = buf.get(start..end).ok_or(CodecError::Truncated {
        offset: start,
        needed: len,
        actual: buf.len(),
    })?;
    *cursor = end;
    Ok(slice)
}
```

---

## 4. Copy-on-Write `Bytes` Payload Backing Preserving C++ `capacity()` (`WP-C`)

Grounded in [Google Rust Style Guide `#prefer-borrows-over-deep-copies`](https://google.github.io/styleguide/rustguide.html#prefer-borrows-over-deep-copies) and [`#more-on-accessors-that-accept-closures`](https://google.github.io/styleguide/rustguide.html#more-on-accessors-that-accept-closures):

```rust
use bytes::Bytes;
use parking_lot::RwLock;

enum PayloadStorage {
    Shared(Bytes),
    Owned(Vec<u8>),
}

struct PayloadState {
    storage: PayloadStorage,
    logical_capacity: usize, // Preserves observable C++ std::vector::capacity() semantics
}

pub struct PayloadImpl {
    state: RwLock<PayloadState>,
}

impl PayloadImpl {
    pub fn from_bytes(bytes: Bytes) -> Self {
        let cap = bytes.len();
        Self {
            state: RwLock::new(PayloadState {
                storage: PayloadStorage::Shared(bytes),
                logical_capacity: cap,
            }),
        }
    }

    /// Zero-allocation slice inspection for internal serializers, routers, and `equals()`.
    pub(crate) fn with_slice<R>(&self, f: impl FnOnce(&[u8]) -> R) -> R {
        let guard = self.state.read();
        match &guard.storage {
            PayloadStorage::Shared(b) => f(b.as_ref()),
            PayloadStorage::Owned(v) => f(v.as_slice()),
        }
    }

    /// Scoped mutable access: materializes `Owned(Vec<u8>)` only on actual mutation.
    pub fn get_data_mut(&self, f: &mut dyn FnMut(&mut [u8])) {
        let mut guard = self.state.write();
        let cap = guard.logical_capacity;
        if let PayloadStorage::Shared(b) = &mut guard.storage {
            let mut vec = Vec::with_capacity(cap.max(b.len()));
            vec.extend_from_slice(b);
            guard.storage = PayloadStorage::Owned(vec);
        }
        if let PayloadStorage::Owned(v) = &mut guard.storage {
            f(v.as_mut_slice());
        }
    }
}
```

---

## 5. `Arc::new_cyclic`, `OnceLock` & Stale-FD-Safe `poll(2)` Reactors (`WP-D`)

1. **`enable_shared_from_this` → `Arc::new_cyclic`**:
   Replace `self_weak: Mutex<Weak<Self>>` + two-step `init()` with `Arc::new_cyclic(|weak| Self { self_weak: weak.clone(), ... })` so `self.self_weak.upgrade()` is completely lock-free and never observes an uninitialized `Weak`.
2. **Write-Once Handles → `std::sync::OnceLock`**:
   Replace `routing: Mutex<Option<Arc<RoutingManager>>>` populated once in `init()` with `OnceLock<Arc<RoutingManager>>`.
3. **Process-Wide `poll(2)` Readiness Reactor + Stale-FD Reconnect Guard**:
   - Reserve `pollfds[0]` for an `eventfd` `WakeFd` (`0.0 / s` idle wake-ups; instant `stop()` wakeup).
   - Cache the `Vec<libc::pollfd>` and `Vec<ReactorToken>` arrays across iterations, rebuilding only when a registration generation counter changes.
   - On `register(fd, weak_ep)`: evict any existing slot holding the same raw `fd` or a dead `Weak` handle.
   - On `poll()` wakeup: snapshot `Weak::upgrade()` **and** verify `ep.pollable_fd() == Some(entry.fd)` and `(revents & libc::POLLNVAL) == 0` *after* releasing the reactor registry lock and *before* calling `read_some()`, preventing blocked reads on recycled file descriptors during rapid UDS/TCP reconnect storms.
