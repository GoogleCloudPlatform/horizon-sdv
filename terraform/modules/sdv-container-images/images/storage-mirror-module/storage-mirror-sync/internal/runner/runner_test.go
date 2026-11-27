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

package runner

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestMask(t *testing.T) {
	if got := Mask("fetch https://bob:s3cret@example.com/x.git"); got != "fetch https://***@example.com/x.git" {
		t.Fatal(got)
	}
}

func TestLinesAndExitCode(t *testing.T) {
	var lines []string
	var out bytes.Buffer
	res, err := Run(context.Background(), Options{Out: &out, OnLine: func(l string) { lines = append(lines, l) }}, "sh", "-c", `printf 'a\rb\nc\n'; exit 3`)
	if err == nil || res.ExitCode != 3 {
		t.Fatalf("exit: %v %+v", err, res)
	}
	if strings.Join(lines, ",") != "a,b,c" || !strings.Contains(res.Tail, "c") {
		t.Fatalf("lines %v tail %q", lines, res.Tail)
	}
}

func TestTTYAndTimeout(t *testing.T) {
	var got []string
	_, err := Run(context.Background(), Options{TTY: true, Out: &bytes.Buffer{}, OnLine: func(l string) { got = append(got, l) }}, "sh", "-c", `[ -t 1 ] && echo tty`)
	if err != nil || len(got) == 0 || got[0] != "tty" {
		t.Fatalf("pty: %v %v", err, got)
	}
	start := time.Now()
	_, err = Run(context.Background(), Options{Timeout: 200 * time.Millisecond, Grace: 100 * time.Millisecond, Out: &bytes.Buffer{}}, "sh", "-c", "sleep 30")
	if !errors.Is(err, ErrTimeout) || time.Since(start) > 10*time.Second {
		t.Fatalf("timeout: %v after %s", err, time.Since(start))
	}
}
