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

# Port Harness & Reference Container Oracle Templates

## 1. Multi-Gate `tools/verify.sh` Template

A production port harness runs every structural, lint, test, and provenance gate in a single deterministic script, supporting both full-phase runs (`./tools/verify.sh phase-03`) and single-gate runs (`./tools/verify.sh provenance`):

```bash
#!/usr/bin/env bash
set -uo pipefail
cd "$(dirname "$0")/.."
WS=$PWD
OUT=${RUST_DIR:-rust-port}
PHASE=${1:-}
FAILED=()

run() {
  local name=$1; shift
  if [[ -n $PHASE && ! $PHASE =~ ^(phase-|stage-)?[0-9]+$ && $PHASE != "$name" ]]; then return 0; fi
  printf '== %-14s ' "$name"
  local log; log=$(mktemp)
  if "$@" >"$log" 2>&1; then echo PASS; else echo FAIL; tail -n 25 "$log" | sed 's/^/   | /'; FAILED+=("$name"); fi
  rm -f "$log"
}

upstream_pin_only() {
  local st; st=$(git submodule status -- upstream/cpp-repo) || return 1
  [[ $st == -* ]] || { echo "upstream/cpp-repo is checked out: $st"; return 1; }
  [[ ! -e $(git rev-parse --git-common-dir)/modules/upstream ]] || { echo ".git/modules/upstream exists"; return 1; }
  [[ -z $(ls -A upstream/cpp-repo 2>/dev/null) ]] || { echo "upstream/cpp-repo is not empty"; return 1; }
}

run upstream-pin   upstream_pin_only
run input          python3 tools/sanitize_upstream.py --check
run fmt            cargo fmt --manifest-path "$OUT/Cargo.toml" --all -- --check
run clippy         cargo clippy --manifest-path "$OUT/Cargo.toml" --workspace --all-targets --locked -- -D warnings
run test           cargo test --manifest-path "$OUT/Cargo.toml" --workspace --locked
[[ -n $PHASE ]] && run provenance python3 skills/verifiable-cpp-to-rust-port-harness/scripts/verify_port_provenance.py \
  --workspace "$WS" --rust-dir "$OUT" --milestone "$PHASE" --strict

if ((${#FAILED[@]})); then echo "VERIFY FAIL: ${FAILED[*]}"; exit 1; fi
echo "VERIFY PASS${PHASE:+ ($PHASE)}"
```

---

## 2. Differential Binary Golden Fixture Generator Pattern (C++ $\rightarrow$ `.bin` $\rightarrow$ Rust)

Never rely solely on Rust round-trip unit tests (`serialize -> deserialize`) for wire or local IPC protocols. Compile a small fixture generator inside the C++ reference container:

```cpp
// interop/ipc_goldens/generate_ipc_fixtures.cpp (compiled inside C++ reference container)
#include <fstream>
#include <vector>
#include "protocol/send_command.hpp"

void write_fixture(const std::string& path, const std::vector<uint8_t>& buf) {
    std::ofstream out(path, std::ios::binary);
    out.write(reinterpret_cast<const char*>(buf.data()), buf.size());
}

int main() {
    rpc_daemon::protocol::send_command cmd;
    cmd.set_client(0x1234);
    cmd.set_instance(0x5678);
    cmd.set_reliable(true);
    cmd.set_target(0x9ABC);
    cmd.set_payload({0xDE, 0xAD, 0xBE, 0xEF});

    std::vector<uint8_t> serialized;
    cmd.serialize(serialized);
    write_fixture("send_command.bin", serialized);
    return 0;
}
```

Then verify byte-for-byte identity in a Rust integration test:

```rust
#[test]
fn send_command_matches_cpp_golden_fixture() {
    let cpp_bytes = std::fs::read("tests/data/ipc/send_command.bin").unwrap();
    let parsed = SendCommand::deserialize(&cpp_bytes).expect("must parse C++ fixture");
    assert_eq!(parsed.client(), 0x1234);
    assert_eq!(parsed.instance(), 0x5678);
    assert!(parsed.is_reliable());
    assert_eq!(parsed.target(), 0x9ABC);

    let mut rust_bytes = Vec::new();
    parsed.serialize(&mut rust_bytes).expect("must serialize");
    assert_eq!(rust_bytes, cpp_bytes, "Rust serialization diverged from C++ golden bytes");
}
```

---

## 3. `DEVIATIONS.md` & `OPTIMIZATIONS.md` Entry Schema

```markdown
### `DEV-01`: `MessageType` and `ReturnCode` Represented as `#[repr(transparent)]` `u8` Newtypes
- **Category**: Wire Fidelity & Safety Hardening
- **Upstream Files**: `include/rpc/enumeration_types.hpp`, `src/message/deserializer.cpp`
- **Rust Files**: `crates/rpc-core/src/types.rs`
- **Description**: Upstream C++ casts raw `uint8_t` header bytes directly to `message_type_e` and `return_code_e` without rejecting unallocated values during deserialization. Rust represents both as `#[repr(transparent)] pub struct MessageType(pub u8)` with associated constants so unknown raw byte values survive round-trip parsing without UB or premature packet rejection.

### `OPT-01`: Copy-on-Write `Bytes` Payload Storage
- **Class**: Class B (Post-Parity Internal Hot-Path Optimization)
- **Target Bottleneck**: Per-message `Vec<u8>` heap allocations in `Payload::new` and `Serializer` (Criterion baseline).
- **Equivalence Note**: Internal `bytes::Bytes` representation behind the unchanged public `Payload` trait; zero wire or API change.
```
