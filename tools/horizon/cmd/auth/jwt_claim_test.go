// Copyright (c) 2026 Accenture, All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//         http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package auth

import (
	"bufio"
	"strings"
	"testing"
	"time"
)

func TestJWTClaimExpUnix(t *testing.T) {
	if _, ok := jwtClaimExpUnix(map[string]any{}); ok {
		t.Fatal("missing exp")
	}
	if _, ok := jwtClaimExpUnix(map[string]any{"exp": float64(0)}); ok {
		t.Fatal("zero exp")
	}
	if _, ok := jwtClaimExpUnix(map[string]any{"exp": "10"}); ok {
		t.Fatal("string exp")
	}
	got, ok := jwtClaimExpUnix(map[string]any{"exp": float64(42)})
	if !ok || got != 42 {
		t.Fatalf("float64: %d %v", got, ok)
	}
	got, ok = jwtClaimExpUnix(map[string]any{"exp": int64(7)})
	if !ok || got != 7 {
		t.Fatalf("int64: %d %v", got, ok)
	}
	got, ok = jwtClaimExpUnix(map[string]any{"exp": 3})
	if !ok || got != 3 {
		t.Fatalf("int: %d %v", got, ok)
	}
}

func TestFormatJWTExpHuman(t *testing.T) {
	future := time.Now().Add(2 * time.Hour).Unix()
	got := formatJWTExpHuman(future)
	if !strings.Contains(got, "(valid for ") {
		t.Fatalf("future: %q", got)
	}
	past := time.Now().Add(-2 * time.Hour).Unix()
	got = formatJWTExpHuman(past)
	if !strings.Contains(got, "(expired ") || !strings.Contains(got, " ago)") {
		t.Fatalf("past: %q", got)
	}
}

func TestPromptLine(t *testing.T) {
	in := bufio.NewReader(strings.NewReader("\n"))
	if got := promptLine(in, "realm", "horizon"); got != "horizon" {
		t.Fatalf("empty line should keep default, got %q", got)
	}
	in = bufio.NewReader(strings.NewReader("  custom  \n"))
	if got := promptLine(in, "realm", "horizon"); got != "custom" {
		t.Fatalf("got %q", got)
	}
	in = bufio.NewReader(strings.NewReader("only\n"))
	if got := promptLine(in, "realm", ""); got != "only" {
		t.Fatalf("got %q", got)
	}
}
