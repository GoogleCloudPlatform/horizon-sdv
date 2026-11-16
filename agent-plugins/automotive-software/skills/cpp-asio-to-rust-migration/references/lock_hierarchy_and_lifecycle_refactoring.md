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

# Lock Hierarchy, `std::recursive_mutex`, and `enable_shared_from_this` Refactoring Blueprint

Architectural patterns for replacing C++14/C++17 `std::recursive_mutex`, `std::enable_shared_from_this<T>`, callback lock-order inversions, and dispatcher self-join deadlocks with deadlock-free Rust ownership.

---

## 1. Common `std::recursive_mutex` Archetypes & Rust Replacements

In C++ networking daemons, `std::recursive_mutex` almost always appears because an operation holding a lock invokes a helper or upward callback that re-enters the same object on the same thread:

| Archetype | Why C++ Used `std::recursive_mutex` | Rust Architectural Replacement |
|---|---|---|
| **1. Client / Stream Endpoint** | `send()` $\to$ `flush()` $\to$ error path or `on_connect()` $\to$ `arm_timer()` re-enters methods that lock the endpoint mutex | Dedicated `EndpointActor` task, or non-blocking leaf queue lock that drops its guard before escalating disconnect/error events |
| **2. Endpoint / Connection Manager** | `find_or_create_endpoint()` locks `endpoints_mutex` and calls `find_endpoint()` which locks `endpoints_mutex` again | Split public locking entry points (`pub fn find_or_create(&self)`) from private unlocked helpers (`fn find_unlocked(&State)`) |
| **3. Discovery / Subscription Engine** | Routing manager acquires discovery's subscription lock externally and calls back into discovery methods | Encapsulate subscription state inside the discovery module/actor; never expose internal mutex references across module boundaries |
| **4. Callback / Handler Registry** | Completion handler invokes user callback while holding `handlers_mutex`, and the user callback registers or removes a handler | Clone target `Arc<dyn Fn(...) + Send + Sync>` (or take `oneshot::Sender`) under the lock, drop the guard, then invoke |
| **5. Plugin / Component Registry** | `get_plugin()` locks `plugins_mutex` and calls `load_plugin()` which re-locks `plugins_mutex` | Initialize immutable registry once via `std::sync::OnceLock<PluginRegistry>` or separate `get_or_load` from `load_unlocked` |

---

## 2. Replacing `std::enable_shared_from_this<T>` Lifecycles

C++ Boost.Asio classes inherit `std::enable_shared_from_this<T>` so completion handlers (`[self = shared_from_this()](auto ec) { ... }`) keep the object alive in flight. Direct translation into `self_weak: Mutex<Weak<Self>>` populated after `Arc::new` creates unnecessary lock traffic and initialization races.

Use one of two idiomatic Rust patterns depending on your runtime model:
1. **Shared Readiness Reactor / Sync Facade (`Arc::new_cyclic`)**:
   ```rust
   use std::sync::{Arc, Weak};

   pub struct Endpoint {
       self_weak: Weak<Self>, // Plain Weak<Self>: zero Mutex overhead
       // ...
   }

   impl Endpoint {
       pub fn new() -> Arc<Self> {
           Arc::new_cyclic(|weak| Self {
               self_weak: weak.clone(),
           })
       }
   }
   ```
2. **Async Actor Model (`JoinSet` + `CancellationToken`)**:
   - Parent manager owns `JoinSet<()>` + per-endpoint `CancellationToken` + bounded `mpsc::Sender<Cmd>`. Endpoint state lives directly on the task stack without `Arc<Self>` cycles.

---

## 3. Six Classic C++ Networking Deadlocks & Their Rust Fixes

1. **Upward Callback Lock-Order Inversion (`Endpoint` $\leftrightarrow$ `Manager`)**:
   - *Hazard*: Downward `Manager::send` locks `Manager` then `Endpoint`; upward `Endpoint::on_write_error` holds `Endpoint` and calls `Manager::on_disconnect` (locking `Manager`). Manual `lock.unlock()` before the upward call creates TOCTOU races where another thread removes or restarts the endpoint mid-call.
   - *Fix*: Emit upward `EndpointUpwardEvent::Disconnected { key, generation }` over a bounded channel (or return `FlushOutcome::Disconnected` after dropping the endpoint lock) and validate the endpoint generation counter in the manager.
2. **Parent/Child Per-Object Mutex Inversion (`SubscriptionTable` $\leftrightarrow$ `AckObject`)**:
   - *Hazard*: Giving individual acknowledgement/session objects their own `Mutex` alongside a table `Mutex` invites `Table -> Ack` vs. `Ack -> Table` deadlocks.
   - *Fix*: Store `HashMap<AckId, AckState>` by value inside a single lock domain or actor task, indexed by `Copy` handles (`AckId`).
3. **One-Shot Cleanup Re-Entrancy**:
   - *Hazard*: Invoking a cleanup closure while holding an internal mutex deadlocks if the closure calls back into `close()` or `status()`.
   - *Fix*: Take the `Option<Box<dyn FnOnce() + Send>>` (or `oneshot::Sender`) via `.take()` inside the lock scope, drop the guard, and invoke once outside all locks.
4. **Circular `Arc` Ownership in Callback Timers**:
   - *Hazard*: Storing `Arc<Timer>` on `Service` while `Timer`'s closure holds `Arc<Service>` leaks both objects permanently.
   - *Fix*: Store `Weak<Service>` in timer callbacks on a shared `TimerReactor`, or poll `tokio::time::Sleep` inside the owning task.
5. **Dispatcher Thread Self-Join Deadlock on `stop()`**:
   - *Hazard*: When an application callback executing on dispatcher thread `D` calls `app.stop()`, joining `D` synchronously inside `stop()` deadlocks (`D` attempts to join itself or blocks `main()`).
   - *Fix*: Make `stop()` signal an `AtomicBool` (`Ordering::Release`) + `Condvar::notify_all()` / `eventfd` wakeup (or `CancellationToken::cancel()`), and join worker threads only on the external owner thread (skipping `thread::current().id() == handle.thread().id()`, queuing self-exiting dispatchers onto a dedicated reaper list).
6. **Ban on `parking_lot::ReentrantMutex<RefCell<T>>`**:
   - *Hazard*: `ReentrantMutex` only yields `&T`. Pairing it with `RefCell<T>` converts recursive calls on the same stack into immediate runtime `BorrowMutError` panics (`panic = "abort"`).
   - *Fix*: Enforce a numbered N-tier lock hierarchy (`Tier 1..N`) documented in `docs/ARCHITECTURE.md` and verified in CI with `parking_lot`'s `deadlock_detection` watchdog thread.

---

## 4. Blueprint: Generation-Tagged `EndpointManagerActor`

```rust
use std::collections::HashMap;
use std::net::SocketAddr;
use bytes::Bytes;
use tokio::sync::{mpsc, oneshot};
use tokio::task::JoinSet;
use tokio_util::sync::CancellationToken;

#[derive(Debug, Clone, Copy, PartialEq, Eq, Hash)]
pub struct EndpointKey {
    pub remote: SocketAddr,
    pub partition_id: u16,
}

pub enum EndpointManagerCmd {
    SendToRemote { key: EndpointKey, payload: Bytes },
    RestartEndpoint { key: EndpointKey },
    RemoveEndpoint { key: EndpointKey, reply: oneshot::Sender<bool> },
}

pub enum EndpointUpwardEvent {
    Disconnected { key: EndpointKey, generation: u64 },
    MessageReceived { key: EndpointKey, payload: Bytes },
}

struct ManagedEndpointHandle {
    generation: u64,
    cmd_tx: mpsc::Sender<Bytes>,
    cancel: CancellationToken,
}

pub struct EndpointManagerActor {
    next_generation: u64,
    endpoints: HashMap<EndpointKey, ManagedEndpointHandle>,
    cmd_rx: mpsc::Receiver<EndpointManagerCmd>,
    event_rx: mpsc::Receiver<EndpointUpwardEvent>,
    event_tx: mpsc::Sender<EndpointUpwardEvent>,
    tasks: JoinSet<()>,
    root_cancel: CancellationToken,
}

impl EndpointManagerActor {
    pub fn new(
        cmd_rx: mpsc::Receiver<EndpointManagerCmd>,
        root_cancel: CancellationToken,
    ) -> Self {
        let (event_tx, event_rx) = mpsc::channel(256);
        Self {
            next_generation: 1,
            endpoints: HashMap::new(),
            cmd_rx,
            event_rx,
            event_tx,
            tasks: JoinSet::new(),
            root_cancel,
        }
    }

    pub async fn run(mut self) {
        loop {
            tokio::select! {
                biased;
                _ = self.root_cancel.cancelled() => {
                    for (_, ep) in self.endpoints.drain() {
                        ep.cancel.cancel();
                    }
                    while self.tasks.join_next().await.is_some() {}
                    break;
                }
                Some(cmd) = self.cmd_rx.recv() => self.handle_cmd(cmd),
                Some(ev) = self.event_rx.recv() => self.handle_upward_event(ev),
            }
        }
    }

    fn handle_cmd(&mut self, cmd: EndpointManagerCmd) {
        match cmd {
            EndpointManagerCmd::SendToRemote { key, payload } => {
                let handle = self.get_or_spawn_endpoint(key);
                let _ = handle.cmd_tx.try_send(payload);
            }
            EndpointManagerCmd::RestartEndpoint { key } => {
                if let Some(old) = self.endpoints.remove(&key) {
                    old.cancel.cancel();
                }
                let _ = self.get_or_spawn_endpoint(key);
            }
            EndpointManagerCmd::RemoveEndpoint { key, reply } => {
                let existed = self.endpoints.remove(&key).map(|old| old.cancel.cancel()).is_some();
                let _ = reply.send(existed);
            }
        }
    }

    fn handle_upward_event(&mut self, ev: EndpointUpwardEvent) {
        if let EndpointUpwardEvent::Disconnected { key, generation } = ev {
            if self.endpoints.get(&key).is_some_and(|cur| cur.generation == generation) {
                if let Some(old) = self.endpoints.remove(&key) {
                    old.cancel.cancel();
                }
                let _ = self.get_or_spawn_endpoint(key);
            }
        }
    }

    fn get_or_spawn_endpoint(&mut self, key: EndpointKey) -> &ManagedEndpointHandle {
        self.endpoints.entry(key).or_insert_with(|| {
            let generation = self.next_generation;
            self.next_generation += 1;
            let (cmd_tx, mut cmd_rx) = mpsc::channel::<Bytes>(64);
            let cancel = self.root_cancel.child_token();
            let task_cancel = cancel.clone();
            let event_tx = self.event_tx.clone();

            self.tasks.spawn(async move {
                loop {
                    tokio::select! {
                        biased;
                        _ = task_cancel.cancelled() => break,
                        maybe_frame = cmd_rx.recv() => match maybe_frame {
                            Some(_frame) => { /* bounded async write */ }
                            None => break,
                        }
                    }
                }
                let _ = event_tx.try_send(EndpointUpwardEvent::Disconnected { key, generation });
            });

            ManagedEndpointHandle { generation, cmd_tx, cancel }
        })
    }
}
```
