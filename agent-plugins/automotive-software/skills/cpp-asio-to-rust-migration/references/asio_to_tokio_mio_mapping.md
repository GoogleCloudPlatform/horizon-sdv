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

# Boost.Asio to Tokio & Mio Primitive Mapping Reference

Primitive-by-primitive translation rules, socket configuration recipes (`socket2`), cancel-safety matrix, and timer wheel patterns for migrating C++14/C++17 `boost::asio` networking daemons to Rust (`tokio` and `mio`).

---

## 1. Boost.Asio Construct Mapping Table

| Boost.Asio Construct | Primary Rust (`tokio`) Equivalent | Primary Rust (`mio` / `poll(2)`) Equivalent |
|---|---|---|
| `boost::asio::io_context` | `tokio::runtime::Runtime` / `Handle` | `mio::Poll` + `mio::Events` (or shared `poll(2)` reactor) |
| `async_receive*` / `async_send*` / `async_read` / `async_write` / `async_accept` / `async_connect` | Cancel-safe `AsyncReadExt::read_buf`, `UdpSocket::recv_from`, `TcpListener::accept` | Non-blocking `std::io::Read` / `Write` after readiness event |
| `boost::asio::steady_timer` | `tokio::pin!(Sleep)` + `.as_mut().reset()`, `Interval`, or `DelayQueue<T>` | Monotonic `BTreeMap` / `BinaryHeap` (`std::time::Instant`) driving `Poll::poll` timeout |
| `boost::asio::post` / `dispatch` | `mpsc::Sender::send` / `try_send` to actor task | `mio::Waker::wake()` / `eventfd` + bounded MPSC queue |
| `ip::udp::socket` | `tokio::net::UdpSocket` (configured via `socket2::Socket`) | `mio::net::UdpSocket` |
| `local::stream_protocol::socket` / `acceptor` | `tokio::net::UnixStream` / `UnixListener` (`into_split()`) | `mio::net::UnixStream` / `UnixListener` |
| `ip::tcp::socket` | `tokio::net::TcpStream` (`into_split()` $\to$ `OwnedReadHalf` + `OwnedWriteHalf`) | `mio::net::TcpStream` + bounded non-blocking outbound queue |
| `ip::tcp::acceptor` | `tokio::net::TcpListener` | `mio::net::TcpListener` |
| `executor_work_guard` | Actor `mpsc::Receiver::recv()` loop + `CancellationToken` | Event loop running flag `while !shutdown.load(Ordering::Acquire)` |
| `io_context::strand` | Single-owner actor task + split read/write halves | Single-threaded readiness reactor loop |

---

## 2. Proactor (Boost.Asio) vs. Poll Readiness (`tokio`) Cancel-Safety Matrix

In `boost::asio`, initiating `async_receive_from` or `async_write` hands the buffer to the reactor until completion (`operation_aborted` on close). In `tokio`, dropping a `Future` inside `tokio::select!` cancels it immediately at its last `.await` yield point.

| Operation / Method | Cancel-Safe in `tokio::select!`? | Failure Mode if Cancelled Mid-Flight | Mandatory Safe Pattern |
|---|---|---|---|
| `tokio::net::UdpSocket::recv_from(&mut buf)` | **Yes** | None: either a complete datagram is copied into `buf` or nothing is read | Poll directly in `tokio::select!`; validate minimum/maximum datagram length |
| `tokio::io::AsyncReadExt::read_buf(&mut bytes_mut)` | **Yes** | None: appends any bytes read so far to `BytesMut` and advances write cursor | Use `reader.read_buf(&mut read_buf)` in `tokio::select!`, then parse complete frames |
| `tokio::io::AsyncReadExt::read_exact(&mut [u8])` | **NO** | **Frame Corruption**: bytes read into the stack slice before cancellation are discarded | **Never** place `read_exact` in a `tokio::select!` branch; use `read_buf` or `FramedRead` |
| `tokio::io::AsyncWriteExt::write_all(&[u8])` | **NO** | **Partial Frame Write**: if cancelled after writing $k < N$ bytes over TCP/UDS, the stream desynchronizes | Isolate stream writes in a dedicated Writer Task (`OwnedWriteHalf`), or wrap with `tokio::time::timeout` outside `select!` and reconnect on timeout |
| `tokio::sync::mpsc::Receiver::recv()` | **Yes** | None: no message is lost from the channel queue | Use as the primary actor command intake branch in `tokio::select!` |
| `TcpListener::accept()` / `UnixListener::accept()` | **Yes** | None: connection remains in kernel accept backlog | Poll directly in `tokio::select!` alongside `CancellationToken::cancelled()` |

---

## 3. Low-Level UDP Socket Options & Multicast Configuration (`socket2` + `tokio`)

Initializing automotive or industrial UDP endpoints frequently requires configuring `SO_REUSEADDR`, `SO_BINDTODEVICE`, `SO_RCVBUF` / `SO_RCVBUFFORCE`, and explicit IPv4/IPv6 multicast interfaces before binding:

```rust
use std::io;
use std::net::{IpAddr, Ipv4Addr, SocketAddr};
use socket2::{Domain, Protocol, SockAddr, Socket, Type};
use tokio::net::UdpSocket;

pub struct UdpEndpointConfig {
    pub local_addr: SocketAddr,
    pub bind_device: Option<String>,
    pub recv_buffer_size: usize,
    pub ipv6_interface_index: u32, // Explicit OS interface index (if_nametoindex)
}

pub fn create_configured_udp_socket(cfg: &UdpEndpointConfig) -> io::Result<UdpSocket> {
    let domain = match cfg.local_addr {
        SocketAddr::V4(_) => Domain::IPV4,
        SocketAddr::V6(_) => Domain::IPV6,
    };

    let socket = Socket::new(domain, Type::DGRAM, Some(Protocol::UDP))?;
    socket.set_nonblocking(true)?;
    socket.set_reuse_address(true)?;
    socket.set_broadcast(true)?;

    #[cfg(any(target_os = "linux", target_os = "nto"))]
    if let Some(ref dev) = cfg.bind_device {
        if !dev.is_empty() {
            let _ = socket.bind_device(Some(dev.as_bytes()));
        }
    }

    let _ = socket.set_recv_buffer_size(cfg.recv_buffer_size);
    #[cfg(target_os = "linux")]
    if let Ok(actual) = socket.recv_buffer_size() {
        if actual < cfg.recv_buffer_size {
            let size_c: libc::c_int = cfg.recv_buffer_size.try_into().unwrap_or(i32::MAX);
            // SAFETY: socket.as_raw_fd() is a valid open socket descriptor and size_c is stack-allocated c_int.
            unsafe {
                use std::os::fd::AsRawFd;
                let _ = libc::setsockopt(
                    socket.as_raw_fd(),
                    libc::SOL_SOCKET,
                    libc::SO_RCVBUFFORCE,
                    (&raw const size_c).cast::<libc::c_void>(),
                    std::mem::size_of::<libc::c_int>() as libc::socklen_t,
                );
            }
        }
    }

    socket.bind(&SockAddr::from(cfg.local_addr))?;

    match cfg.local_addr.ip() {
        IpAddr::V4(v4) => socket.set_multicast_if_v4(&v4)?,
        IpAddr::V6(_) => socket.set_multicast_if_v6(cfg.ipv6_interface_index)?,
    }

    UdpSocket::from_std(socket.into())
}

pub fn join_multicast_v4(
    sock: &UdpSocket,
    multicast_v4: Ipv4Addr,
    interface_v4: Ipv4Addr,
) -> io::Result<()> {
    sock.join_multicast_v4(multicast_v4, interface_v4)
}
```

> **Shared `Arc<UdpSocket>` Without Locks**: In both `std::net::UdpSocket` and `tokio::net::UdpSocket`, `recv_from(&self)` and `send_to(&self)` take `&self` because the kernel socket is thread-safe for concurrent `recvfrom(2)` and `sendto(2)` syscalls.

---

## 4. Translating `boost::asio::steady_timer` Patterns

### Pattern A: Single Re-Armable Watchdog / Batch-Flush Timer
- **C++ Hazard**: Calling `timer.expires_after(...)` followed by `timer.async_wait([self = shared_from_this()](auto ec) { ... })` queues an `operation_aborted` completion handler for the old expiry. If the callback does not verify both `ec != operation_aborted` and the current timer generation, stale callbacks fire spuriously.
- **Rust Solution**: Use a single `tokio::pin!(sleep)` inside the actor loop and call `sleep.as_mut().reset(new_deadline)` with zero heap allocations.

### Pattern B: Periodic Keepalive / Cyclic Announce Timer
- **Rust Solution**: Use `tokio::time::interval_at(start, period)` with `MissedTickBehavior::Delay` (or `Skip`), or schedule recurring entries on a shared monotonic `TimerReactor`.

### Pattern C: Dynamic Per-Subscription / Per-Session TTL Wheel
Manage hundreds of independent subscription or request TTLs in a single `BTreeMap<(Instant, Key), ()>` without per-item timer tasks or mutex contention:

```rust
use std::collections::{BTreeMap, HashMap};
use std::time::{Duration, Instant};

#[derive(Debug, Clone, Copy, PartialEq, Eq, PartialOrd, Ord, Hash)]
pub struct SubscriptionKey {
    pub service_id: u16,
    pub instance_id: u16,
    pub group_id: u16,
    pub client_id: u16,
}

pub struct TtlWheel {
    deadlines_by_key: HashMap<SubscriptionKey, Instant>,
    wheel: BTreeMap<(Instant, SubscriptionKey), ()>,
}

impl TtlWheel {
    pub fn new() -> Self {
        Self {
            deadlines_by_key: HashMap::new(),
            wheel: BTreeMap::new(),
        }
    }

    pub fn schedule_or_refresh(&mut self, key: SubscriptionKey, ttl: Duration) {
        let new_deadline = Instant::now() + ttl;
        if let Some(old_deadline) = self.deadlines_by_key.insert(key, new_deadline) {
            self.wheel.remove(&(old_deadline, key));
        }
        self.wheel.insert((new_deadline, key), ());
    }

    pub fn cancel(&mut self, key: &SubscriptionKey) {
        if let Some(old_deadline) = self.deadlines_by_key.remove(key) {
            self.wheel.remove(&(old_deadline, *key));
        }
    }

    pub fn next_deadline(&self) -> Option<Instant> {
        self.wheel.keys().next().map(|(instant, _)| *instant)
    }

    pub fn pop_expired(&mut self, now: Instant) -> Vec<SubscriptionKey> {
        let mut expired = Vec::new();
        while let Some((&(deadline, key), _)) = self.wheel.iter().next() {
            if deadline > now {
                break;
            }
            self.wheel.remove(&(deadline, key));
            self.deadlines_by_key.remove(&key);
            expired.push(key);
        }
        expired
    }
}
```

---

## 5. Deterministic Single-Threaded `mio::Poll` Event Loop

When running inside a constrained partition where multi-threaded work-stealing executors are undesirable, drive non-blocking sockets and `TtlWheel` deadlines from a single `mio::Poll` loop:

```rust
use std::io;
use std::time::Instant;
use mio::{Events, Interest, Poll, Token};
use mio::net::UdpSocket;

const UDP_TOKEN: Token = Token(0);
const WAKER_TOKEN: Token = Token(1);

pub fn run_deterministic_mio_loop(
    mut udp_socket: UdpSocket,
    mut ttl_wheel: TtlWheel,
) -> io::Result<()> {
    let mut poll = Poll::new()?;
    let mut events = Events::with_capacity(128);
    poll.registry().register(&mut udp_socket, UDP_TOKEN, Interest::READABLE)?;

    let mut recv_buf = [0u8; 1500];
    loop {
        let _expired = ttl_wheel.pop_expired(Instant::now());
        let timeout = ttl_wheel
            .next_deadline()
            .map(|dl| dl.saturating_duration_since(Instant::now()));

        poll.poll(&mut events, timeout)?;
        for event in events.iter() {
            match event.token() {
                UDP_TOKEN => loop {
                    match udp_socket.recv_from(&mut recv_buf) {
                        Ok((len, _peer)) if len >= 16 => { /* process frame */ }
                        Ok(_) => continue,
                        Err(ref e) if e.kind() == io::ErrorKind::WouldBlock => break,
                        Err(e) => return Err(e),
                    }
                },
                WAKER_TOKEN => return Ok(()),
                _ => {}
            }
        }
    }
}
```

---

## 6. Dual Endianness, UDS `SO_PEERCRED` & Hex-or-Decimal JSON Deserializers

1. **Dual Endianness**: Keep external network protocol headers in **Big-Endian** (`u16::from_be_bytes`, `u32::from_be_bytes`) while keeping `std::memcpy`-serialized local Unix Domain Socket IPC headers in **Native Byte Order** (`u16::from_ne_bytes`, `u32::from_ne_bytes`).
2. **UDS Peer Credentials (`SO_PEERCRED`)**: Extract kernel-verified `uid`, `gid`, and `pid` immediately upon `UnixListener::accept()` via `stream.peer_cred()` before admitting local clients into ACL tables.
3. **Hex-or-Decimal JSON Configuration Values**: Support `"0x1234"`, `"4660"`, and `4660` interchangeably via a custom `serde::de::Visitor`:

```rust
use std::fmt;
use serde::de::{self, Deserializer, Visitor};

pub fn deserialize_u16_hex_or_dec<'de, D>(deserializer: D) -> Result<u16, D::Error>
where
    D: Deserializer<'de>,
{
    struct HexOrDecU16Visitor;

    impl<'de> Visitor<'de> for HexOrDecU16Visitor {
        type Value = u16;

        fn expecting(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
            f.write_str("a u16 integer or a hex/decimal string like \"0x1234\" or \"4660\"")
        }

        fn visit_u64<E: de::Error>(self, v: u64) -> Result<Self::Value, E> {
            u16::try_from(v).map_err(|_| E::custom(format!("u16 out of range: {v}")))
        }

        fn visit_i64<E: de::Error>(self, v: i64) -> Result<Self::Value, E> {
            u16::try_from(v).map_err(|_| E::custom(format!("u16 out of range: {v}")))
        }

        fn visit_str<E: de::Error>(self, s: &str) -> Result<Self::Value, E> {
            let trimmed = s.trim();
            if let Some(hex) = trimmed.strip_prefix("0x").or_else(|| trimmed.strip_prefix("0X")) {
                u16::from_str_radix(hex, 16).map_err(E::custom)
            } else {
                trimmed.parse::<u16>().map_err(E::custom)
            }
        }
    }

    deserializer.deserialize_any(HexOrDecU16Visitor)
}
```
