---
name: verifiable-cpp-to-rust-port-harness
description: >-
  Designs, scaffolds, and audits verifiable phased C/C++-to-Rust port harnesses
  with pin-only upstream gitlinks, deterministic out-of-workspace sanitizers,
  100% file-accounting provenance tables, black-box C++ reference container
  oracles (differential binary golden fixtures, cross-stack interop runners, OS
  lockfile/signal probes), and explicit deviation and optimization ledgers. Use
  when: setting up a C or C++ to Rust port repository, building automated
  milestone verification gates, generating differential C++ vs Rust binary
  wire/IPC fixtures, or auditing upstream file provenance and deviation ledgers.
  Don't use for: performing a fresh-context function-by-function code review
  (use cpp-to-rust-port-review) or translating Boost.Asio concurrency primitives
  (use cpp-asio-to-rust-migration).
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

# Verifiable C/C++-to-Rust Port Harness (`verifiable-cpp-to-rust-port-harness`)

Provides the repository architecture, verification harness, reference container oracle patterns, and ledger governance required to execute a phased C/C++-to-Rust port without silent feature drops, stubbed OS syscalls, or weakened test suites.

## Quick Start

| Goal | Command |
|---|---|
| **Run built-in provenance & ledger self-test** | `python3 skills/verifiable-cpp-to-rust-port-harness/scripts/verify_port_provenance.py --self-test` |
| **Audit upstream file provenance & deviation/optimization ledgers** | `python3 skills/verifiable-cpp-to-rust-port-harness/scripts/verify_port_provenance.py --workspace . --rust-dir ./rust-port --strict` |
| **Run single non-test gate (safe during parallel verifier runs)** | `./tools/verify.sh provenance` |

## Why Static Gates & Unit Tests Alone Fail on Large Ports

Even when an initial port passes `cargo fmt`, `cargo clippy`, unit tests, and header-count checks, automated translation routinely introduces three classes of silent defects:
1. **Silently Stubbed Syscalls**: Under a blanket `#![forbid(unsafe_code)]`, agents stub out `fcntl(F_SETLK)` daemon lockfile election, `atexit` cleanup, or signal/termination hooks so the crate compiles cleanly.
2. **Symmetric Round-Trip Blindness**: Rust-only round-trip tests (`serialize -> deserialize`) pass even when wire or local IPC fields are serialized with the wrong endianness or strict Rust `enum` types reject valid raw wire bytes.
3. **Test Weakening**: Multi-process lock and state-machine unit tests get quietly simplified into struct-field getter checks that always pass.

A production port harness closes these loopholes by combining **four structural pillars**:

---

## Pillar 1: Three-Tree Workspace & Pin-Only Upstream Quarantine

Structure the porting workspace so upstream inputs, sanitized reference code, and the Rust output workspace have cryptographically pinned, auditable relationships:

```text
workspace-root/
├── .gitmodules                  # spec submodules + optional pin-only upstream gitlink (update = none)
├── upstream/<cpp-repo>          # Empty dir; 160000 gitlink pinned to exact upstream tag/SHA
├── <cpp-repo>/                  # Read-only sanitized C/C++ reference tree (plain files, SHA-256 locked)
├── SANITIZATION.md              # Deterministic log of excluded/scrubbed upstream paths and rules
├── milestones/
│   ├── README.md                # Phase workflow and completion checklist
│   └── phase-01.md .. phase-NN.md # Per-phase upstream file tables, test counts, and exit criteria
├── tools/
│   ├── sanitize_upstream.py     # Out-of-workspace deterministic input sanitizer (--check / --rederive)
│   └── verify.sh                # Automated multi-gate verification runner
├── container/
│   └── Containerfile            # Multi-stage black-box C++ reference oracle image
├── interop/                     # C++ golden fixture generators, cross-stack interop & benchmarks
└── <rust-dir>/                  # Rust Cargo workspace (tracked directly, via git subtree, or submodule)
```

### Pin-Only Upstream Gitlink (`update = none`)
When the upstream C/C++ repository contains excluded subdirectories (e.g., third-party or out-of-scope plugins) that should never be checked out in the working tree:
1. Register the exact upstream commit SHA as a `160000` gitlink in the index without cloning it into the workspace:
   ```bash
   mkdir -p upstream/cpp-repo
   git update-index --add --cacheinfo 160000,<UPSTREAM_COMMIT_SHA>,upstream/cpp-repo
   git config -f .gitmodules submodule.upstream/cpp-repo.path upstream/cpp-repo
   git config -f .gitmodules submodule.upstream/cpp-repo.url <UPSTREAM_GIT_URL>
   git config -f .gitmodules submodule.upstream/cpp-repo.update none
   git config -f .gitmodules submodule.upstream/cpp-repo.shallow true
   ```
2. Enforce in `tools/verify.sh` (`upstream-pin` gate) that `git submodule status -- upstream/cpp-repo` starts with `-`, `.git/modules/upstream` does not exist, and `upstream/cpp-repo/` is empty.
3. `tools/sanitize_upstream.py` fetches `<UPSTREAM_COMMIT_SHA>` into an isolated `tempfile.TemporaryDirectory()` outside the workspace, applies deterministic exclusion rules, verifies the resulting directory SHA-256 digest (`--check` offline, `--rederive` online), and populates `<cpp-repo>/`.

---

## Pillar 2: Black-Box C++ Reference Container Oracle

Compile the pinned upstream commit inside a **multi-stage `Containerfile` build stage** and copy only the compiled shared libraries and oracle binaries into the runtime image. Use the container strictly as a **black-box behavioral oracle** across three layers:

| Oracle Layer | What It Proves | How It Works |
|---|---|---|
| **1. Differential Binary Golden Fixtures** | Byte-for-byte wire & IPC layout parity (field order, widths, endianness, padding, length calculation) | A small C++ generator runs inside the oracle container, links against the upstream C++ library, constructs canonical and boundary messages/commands, and writes `.bin` files. Rust integration tests deserialize `.bin` and re-serialize, asserting `rust_bytes == cpp_bytes`. |
| **2. Live Cross-Stack Interop (`Layout A` & `Layout B`)** | Protocol state machines, handshakes, framing, and multicast discovery | **Layout A (Single Container)**: Rust client $\leftrightarrow$ C++ daemon (and vice versa) over Unix Domain Sockets. **Layout B (Two Bridge-Connected Containers)**: Node 1 (`10.88.0.10`) running Rust $\leftrightarrow$ Node 2 (`10.88.0.11`) running C++ over real Linux `veth` UDP/TCP and multicast. |
| **3. Cross-Process OS Probes** | OS-level mutual exclusion, credentials, and lifecycle semantics | Spawn C++ and Rust processes concurrently against the same `/tmp/*.lck` file or UDS socket to verify `fcntl(F_SETLK)` daemon election and `SO_PEERCRED` uid/gid enforcement. |

---

## Pillar 3: Explicit `DEVIATIONS.md` & `OPTIMIZATIONS.md` Ledger Governance

Never allow implicit or undocumented differences between the C/C++ reference and the Rust port:

1. **`docs/DEVIATIONS.md` (`DEV-01`..`DEV-NN`)**:
   - Record every intentional deviation from upstream C/C++ with an ID (`DEV-xx`), category (`Build Profile`, `Safety Hardening`, `Concurrency Model`, `Clean-Room Replacement`), affected upstream files, Rust files, and technical rationale.
   - List upfront any `#ifdef` branch disabled in the reference container build profile (e.g., optional logging backends, systemd socket activation, or non-target OS branches).
2. **`docs/OPTIMIZATIONS.md` (`OPT-01`..`OPT-NN`)**:
   - Classify every structural or performance optimization into one of three classes:
     - **Class A (Allowed Before Parity)**: Safe Rust foundations (`#![deny(unsafe_code)]` with audited `pub(crate) mod sys`, `Result`-based internal parsers, 1-to-1 public API mirror).
     - **Class B (Post-Parity Only — Internal Hot-Path)**: Internal zero-copy buffers (`Bytes`, `Arc<[u8]>`), lock/allocation reductions (`RwLock`, `SmallVec`, `Arc::new_cyclic`, `OnceLock`), vectored I/O (`writev`), shared `poll(2)`/`eventfd` reactors, and structured internal error/lock-hierarchy hardening that preserve 100% public API and wire compatibility.
     - **Class C (Post-Parity Only — Opt-In Observable)**: New Cargo features (e.g., optional plugins or idiomatic async wrappers) that are strictly additive and opt-in.
3. **Zero Silent `let _ =` Swallowing (`clippy::let_underscore_must_use = "deny"`) & Checked Wire Math**:
   - Classify every `let _ =` site across ported crates into: (i) **Ignored as upstream** (`// Ignored as upstream (<file>.cpp:<line>)` + `#[expect(clippy::let_underscore_must_use, reason = "...")]`), (ii) **Observable** (structured debug/warning/error log), or (iii) **Propagate / Fail-Safe** (`?` or checked thread spawn with `AtomicBool` running-flag rollback on `pthread_create` failure).
   - Require `checked_add` / `checked_sub` / `checked_mul` + `?` across all wire, IPC, segmentation, and Netlink codecs—never allow blanket `saturating_*` calls to mask malformed frame offsets.
4. **Single-Step `tools/verify.sh <step>` & Parallel `cargo test` Isolation**:
   - Support passing an individual step name (`./tools/verify.sh fmt`, `clippy`, `provenance`) so fresh-context verifiers run fast checks without triggering concurrent `cargo test --workspace` processes on the same `target/` directory (which overwrite `target/debug/` binaries mid-test or collide on fixed loopback ports).
   - Once initial parity is complete in a standalone output repo, you can unify the workspace via `git subtree add --prefix=<rust-dir> <standalone-branch>` so harness, oracle, and full Rust commit history live in one repository while preserving standalone tags.

---

## Pillar 4: Bundled Provenance & Ledger Auditor (`scripts/verify_port_provenance.py`)

Run the bundled auditor to verify phase file accounting (`100%` of upstream files in `milestones/*.md` cited in Rust `Provenance:` headers or waived under `## Waivers`), `DEV-*` / `OPT-*` citation integrity, and pin-only submodule quarantine:

```bash
python3 skills/verifiable-cpp-to-rust-port-harness/scripts/verify_port_provenance.py \
  --workspace . \
  --rust-dir ./rust-port \
  --milestone phase-03 \
  --strict
```

### Supported CLI Flags
- `--workspace <dir>`: Path to the porting harness root (default: `.`).
- `--rust-dir <rel_or_abs_dir>`: Path to the Rust output workspace (default: `.`).
- `--milestone <id>`: Optional phase/milestone cap (e.g., `phase-03` or `3`; audits all cards in `milestones/` if omitted).
- `--format {markdown,json,text}`: Output format (default: `markdown`).
- `--strict`: Exit `1` if any uncited upstream file, invalid waiver, dangling `DEV-*`/`OPT-*` reference, or checked-out `update = none` gitlink is found.
- `--self-test`: Runs built-in self-tests against temporary compliant and non-compliant port workspaces.

---

## Deep-Dive References

- **[harness_and_oracle_templates.md](references/harness_and_oracle_templates.md)**: Complete templates for `tools/verify.sh`, pin-only `.gitmodules`, C++ differential binary fixture generators, and `DEVIATIONS.md` / `OPTIMIZATIONS.md` ledger entries.
