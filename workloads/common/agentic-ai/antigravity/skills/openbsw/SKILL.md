---
name: openbsw
description: Analyze Horizon SDV OpenBSW compile, link, configuration, unit-test, integration-test, and embedded software build failures.
---
<!-- Copyright (c) 2026 Accenture, All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

        http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License. -->

# Horizon OpenBSW Analysis

Use this skill when the failing target belongs to OpenBSW or its embedded software
build/test pipeline.

## Goal

Identify the first causal OpenBSW error, trace it to source/configuration, and
propose a minimal fix plus verification steps.

## Workflow

1. Find the earliest causal compile/link/test/config error in the build log.
2. Classify it as:
   - C/C++ compile
   - linker
   - generated/configuration
   - dependency/include
   - unit/integration test
   - toolchain/environment
3. Retrieve a tight log window around the first error.
4. Inspect the referenced file, declaration, build definition, or configuration.
5. For symbol/link problems, identify both consumer and expected provider.
6. For compile errors, distinguish syntax/type/API mismatch from generated-header
   or feature-flag issues.
7. For tests, inspect the failed assertion and production path before changing
   test expectations.
8. Return the smallest credible fix and targeted verification commands.

## OpenBSW-specific heuristics

- Do not treat the final linker/compiler exit status as the root cause.
- For missing includes, verify include path/build metadata and file existence.
- For undefined references, verify conditional compilation and target-specific
  source inclusion.
- For duplicate symbols, identify both definitions before proposing removal.
- For generated artifacts, verify generator/config inputs before editing generated
  output.
- For embedded configuration failures, consider target/variant feature flags and
  generated configuration consistency.
- Avoid speculative hardware/runtime causes unless logs support them.

## Evidence standard

Use log evidence for every diagnosis. For a source or build-configuration fix,
verify the relevant repository state using the source MCP tools whenever possible.
Set `classification` to `openbsw`.
