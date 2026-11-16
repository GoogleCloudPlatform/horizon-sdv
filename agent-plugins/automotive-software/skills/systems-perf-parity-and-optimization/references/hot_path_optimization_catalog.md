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

# C++-to-Rust Hot-Path Optimization Catalog

## 1. Zero-Copy `Bytes` / `Arc<[u8]>` Slicing & Single-Allocation IPC Framing

### Anti-Pattern (Unoptimized 1-to-1 Port)
Every `Payload::new(&data)` allocates a new `Mutex<Vec<u8>>`, and every `Message::serialize()` allocates a fresh `Vec<u8>`, copies the wire header, and copies the payload bytes again before wrapping the result in an outer local IPC command buffer.

### Recommended Pattern (Post-Parity Class B Optimization)
1. Store immutable received/shared payloads as `bytes::Bytes` (or `Arc<[u8]>`) with copy-on-write mutation only when `set_data` / `get_data_mut` is called.
2. Pre-size the outer local IPC command buffer (IPC command header + protocol header + payload length) in a **single allocation** (`BytesMut::with_capacity(total_len)`), serializing header and payload slices directly into the target frame.

```rust
use bytes::{BufMut, Bytes, BytesMut};

pub fn serialize_local_send_frame(
    client: u16,
    instance: u16,
    reliable: bool,
    target: u16,
    wire_header: &[u8; 16],
    payload: &[u8],
) -> Bytes {
    let msg_len = wire_header.len() + payload.len();
    let ipc_payload_size = (2 + 1 + 4 + 2 + msg_len) as u32;
    let total_frame_len = 9usize + ipc_payload_size as usize;

    let mut buf = BytesMut::with_capacity(total_frame_len);
    buf.put_u8(0x06); // SEND_CMD
    buf.put_u16_ne(0x0000);
    buf.put_u16_ne(client);
    buf.put_u32_ne(ipc_payload_size);
    buf.put_u16_ne(instance);
    buf.put_u8(u8::from(reliable));
    buf.put_u32_ne(0);
    buf.put_u16_ne(target);
    buf.put_slice(wire_header);
    buf.put_slice(payload);
    buf.freeze()
}
```

---

## 2. Non-Blocking `try_write` Fast Path + `write_vectored` (`writev`) Batch Draining

### Anti-Pattern (Unoptimized 1-to-1 Port)
Every `Endpoint::send()` pushes a `Vec<u8>` into an `mpsc` channel and wakes a background writer task/thread that issues one `write_all()` syscall per frame. Under 64-byte streaming bursts, channel wakeups and per-frame syscalls saturate CPU.

### Recommended Pattern (Post-Parity Class B Optimization)
1. Maintain a bounded `Mutex<VecDeque<Bytes>>` outbound queue on the endpoint.
2. When `send()` is called and the queue is currently empty, attempt a non-blocking `try_write(&frame)` directly on the caller thread. If all bytes are written, return immediately with **zero wakeups**.
3. When bytes remain or multiple frames accumulate, drain up to 64 `IoSlice` slices in a single `try_write_vectored` (`writev`) syscall.
4. **Re-Entrancy Safety**: Always drop the `VecDeque` lock *before* invoking any disconnect or error callback on `BrokenPipe` / `ConnectionReset`.

```rust
use std::collections::VecDeque;
use std::io::IoSlice;
use bytes::{Buf, Bytes};

pub fn try_drain_vectored(
    stream: &tokio::net::UnixStream,
    queue: &mut VecDeque<Bytes>,
) -> std::io::Result<()> {
    while !queue.is_empty() {
        let mut slices = [IoSlice::new(&[]); 64];
        let count = queue.len().min(64);
        for (i, buf) in queue.iter().take(count).enumerate() {
            slices[i] = IoSlice::new(buf.as_ref());
        }
        let written = stream.try_write_vectored(&slices[..count])?;
        if written == 0 {
            break;
        }
        let mut rem = written;
        while rem > 0 {
            if let Some(front) = queue.front_mut() {
                if rem >= front.len() {
                    rem -= front.len();
                    queue.pop_front();
                } else {
                    front.advance(rem);
                    rem = 0;
                }
            }
        }
    }
    Ok(())
}
```

---

## 3. Stack-Allocated `SmallVec` Fan-Out & `RwLock` Routing Lookups

In event notification fan-out (1 publisher $\rightarrow$ $N$ subscribers), holding a `Mutex` over the routing table while sending or allocating a `Vec<ClientId>` on the heap for every packet adds unnecessary allocator and lock contention:
- Store routing and subscriber tables under `RwLock` (read-heavy: >99.9% lookups, <0.1% subscription changes).
- Collect target subscriber IDs into a stack-allocated `SmallVec<[u16; 8]>` under the `RwLockReadGuard`, drop the read guard immediately, and dispatch frames without holding any routing lock.

---

## 4. Diagnosing Remaining Kernel-Floor Cells (`Layout A` & `Layout B`)

After eliminating allocation, lock, and syscall batching bottlenecks, any remaining `5%`–`15%` differences on sub-100 $\mu\text{s}$ synchronous ping-pong cells typically trace to one of four structural root causes:
1. **Single-Socket Epoll Wake-Up Tax (`~6`–`9 us` per 2-hop RTT)**: When split reader/writer tasks are used vs. writing directly from the readiness reactor thread, two extra cross-task wakeups add `~3–4 us` per hop.
2. **Container veth Bridge Jitter (`~10`–`15 us`)**: On sub-200 $\mu\text{s}$ UDP request/response over a Linux bridge (`Layout B`), host scheduling jitter across `veth` pairs spans `±10%`.
3. **Coalesced Burst Framing Overhead**: Adding a length prefix or IPC command header slightly increases total wire bytes on small 64 B messages when batching is active.
4. **C++ `-O2` vs `-O3` Baseline Variance**: Always verify whether C++ `-O3` experienced container scheduling noise on a single repetition by comparing against C++ `-O2` and checking the 95% bootstrap confidence interval.

---

## 5. Process-Wide `poll(2)` Socket Reactors (`eventfd` Wakeup), `Arc::new_cyclic` & `OnceLock`

### Anti-Pattern ($O(N)$ Thread-per-Socket Endpoint Model)
1. Spawning dedicated OS threads per UDP socket (`50 ms` `poll()` timeout) and 2–3 OS threads per TCP connection (blocking `connect()`, reader, and channel writer) causes thread count and idle wake-ups to scale $O(N)$ with peer count.
2. Storing self-references as `self_weak: Mutex<Weak<Self>>` (populated post-construction via `init()`) and write-once references as `Mutex<Option<Arc<T>>>` adds lock acquisitions on hot dispatch paths.

### Recommended Pattern
1. Multiplex all pollable UDP, TCP, and UDS sockets (`pollable_fd().is_some()`) onto shared process-wide `poll(2)`/`mio` readiness reactors (`256 KiB` thread stack) blocked with infinite timeout (`-1`) and slot `0` reserved for an `eventfd` `WakeFd` (`0.0 / s` idle wake-ups, $O(1)$ thread count across `50–100+` peers).
2. Cache `Vec<libc::pollfd>` and `Vec<ReactorToken>` arrays across wakeups (rebuilding only when the registration generation counter changes), use ABA-safe 64-bit `ReactorToken`s (`(generation << 32) | slot_index`), snapshot `Weak<dyn ReactorHandler>` before releasing the registry lock, and verify `ep.pollable_fd() == Some(entry.fd)` + `(revents & libc::POLLNVAL) == 0` before reading.
3. For non-blocking TCP writes (`connect_nonblocking` + `POLLOUT`), maintain a single `TcpOutboundQueue` (`VecDeque<Entry>` with `front_offset: usize`) under the endpoint's leaf lock: fast-path inline `send()` when empty, advance `front_offset` on partial writes, and arm `POLLOUT` under the same lock to prevent race-induced `POLLOUT` wedges.
4. Construct self-referential `Arc` types via `Arc::new_cyclic` with lock-free `Weak<Self>`, and store write-once component handles in `std::sync::OnceLock<Arc<T>>`.
