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

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	testPassword = "sekret-value"
	testToken    = "0f8fad5b-d9cb-469f-a165-70867728950e"
)

// TestHelperAgent stands in for the node agent when the test binary is started by mtkc-agent. It records
// its command line and environment, then registers as the node agent does by writing registration.name.
func TestHelperAgent(t *testing.T) {
	dir := os.Getenv("MTKC_HELPER_CONFIG")
	if dir == "" {
		t.Skip("only runs as the fake node agent")
	}
	seen := strings.Join(os.Args, " ") + "\n" + os.Getenv("agent__uri") + "\n" + os.Getenv("agent__registration__token")
	_ = os.WriteFile(filepath.Join(dir, "seen"), []byte(seen), 0o600)
	if os.Getenv("MTKC_HELPER_EXIT") == "1" {
		os.Exit(3)
	}
	_ = os.WriteFile(filepath.Join(dir, registrationFile), []byte(os.Getenv("agent__registration__name")+"\n"), 0o600)
	time.Sleep(time.Minute)
	os.Exit(0)
}

func helperCmd(t *testing.T) string {
	if strings.ContainsAny(os.Args[0], " \t") {
		t.Skip("test binary path contains spaces")
	}
	return os.Args[0] + " -test.run=^TestHelperAgent$"
}

func writeSecretDir(t *testing.T, values map[string]string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "secret")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, value := range values {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(value+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func agentSecret(t *testing.T) string {
	return writeSecretDir(t, map[string]string{"username": agentUser, "password": testPassword, "registration": "hz-1", "token": testToken})
}

func TestStartRunsAndStopStopsTheAgentInHostAndContainerMode(t *testing.T) {
	for _, mode := range []string{"host", "container"} {
		t.Run(mode, func(t *testing.T) {
			cfg := filepath.Join(t.TempDir(), "config")
			t.Setenv("MTKC_HELPER_CONFIG", cfg)
			common := []string{"--mode", mode, "--config-dir", cfg}
			startArgs := append([]string{"start"}, common...)
			startArgs = append(startArgs, "--domain", "example.com", "--registration", "hz-1", "--credentials", agentSecret(t),
				"--agent-dir", t.TempDir(), "--agent-cmd", helperCmd(t), "--timeout", "30s")

			var out, errb bytes.Buffer
			if code := run(startArgs, &out, &errb); code != 0 {
				t.Fatalf("start: code %d stderr %s", code, errb.String())
			}
			t.Cleanup(func() { _ = stop(cfg) })
			if all := out.String() + errb.String(); strings.Contains(all, testPassword) || strings.Contains(all, testToken) {
				t.Fatalf("start printed a secret: %q", all)
			}
			pid, running := runningAgent(cfg)
			if !running {
				t.Fatal("no running agent after start")
			}

			seen, err := os.ReadFile(filepath.Join(cfg, "seen"))
			if err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(string(seen), "\n")
			if strings.Contains(lines[0], testToken) {
				t.Fatalf("the token is on the agent's command line: %q", lines[0])
			}
			if lines[1] != "https://example.com/mtk-connect" || lines[2] != testToken {
				t.Fatalf("agent environment uri %q token present %v", lines[1], lines[2] == testToken)
			}

			out.Reset()
			if code := run([]string{"status", "--mode", mode, "--config-dir", cfg}, &out, &errb); code != 0 || !strings.Contains(out.String(), "registration hz-1") {
				t.Fatalf("status: code %d out %q", code, out.String())
			}
			errb.Reset()
			if code := run(startArgs, &out, &errb); code == 0 || !strings.Contains(errb.String(), "already running") {
				t.Fatalf("a second start must be refused: code %d stderr %q", code, errb.String())
			}

			if code := run(append([]string{"stop"}, common...), &out, &errb); code != 0 {
				t.Fatalf("stop: code %d stderr %s", code, errb.String())
			}
			deadline := time.Now().Add(5 * time.Second)
			for alive(pid) && time.Now().Before(deadline) {
				time.Sleep(50 * time.Millisecond)
			}
			if alive(pid) {
				t.Fatalf("agent pid %d still alive after stop", pid)
			}
			out.Reset()
			if code := run([]string{"status", "--mode", mode, "--config-dir", cfg}, &out, &errb); code == 0 {
				t.Fatalf("status after stop: code %d out %q", code, out.String())
			}
		})
	}
}

func TestStartRefusesWithoutLaunching(t *testing.T) {
	cases := map[string]struct {
		secret map[string]string
		domain string
		want   string
	}{
		"admin key": {
			secret: map[string]string{"username": "mtk-connect-admin", "password": "admin-key", "registration": "hz-1", "token": testToken},
			domain: "example.com", want: "refusing",
		},
		"no token": {
			secret: map[string]string{"username": agentUser, "password": testPassword, "registration": "hz-1", "token": ""},
			domain: "example.com", want: "no registration token",
		},
		"other registration": {
			secret: map[string]string{"username": agentUser, "password": testPassword, "registration": "hz-2", "token": testToken},
			domain: "example.com", want: "registration",
		},
		"domain with scheme": {
			secret: map[string]string{"username": agentUser, "password": testPassword, "registration": "hz-1", "token": testToken},
			domain: "https://example.com", want: "host name",
		},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			cfg := filepath.Join(t.TempDir(), "config")
			t.Setenv("MTKC_HELPER_CONFIG", cfg)
			var out, errb bytes.Buffer
			code := run([]string{"start", "--mode", "host", "--domain", c.domain, "--registration", "hz-1",
				"--credentials", writeSecretDir(t, c.secret), "--config-dir", cfg, "--agent-cmd", helperCmd(t)}, &out, &errb)
			if code == 0 || !strings.Contains(errb.String(), c.want) {
				t.Fatalf("code %d stderr %q, want %q", code, errb.String(), c.want)
			}
			if all := out.String() + errb.String(); strings.Contains(all, "admin-key") || strings.Contains(all, testPassword) || strings.Contains(all, testToken) {
				t.Fatalf("a secret was printed: %q", all)
			}
			if _, err := os.Stat(filepath.Join(cfg, "seen")); err == nil {
				t.Fatal("the agent was launched")
			}
		})
	}
}

func TestStartReportsAnAgentThatExits(t *testing.T) {
	cfg := filepath.Join(t.TempDir(), "config")
	t.Setenv("MTKC_HELPER_CONFIG", cfg)
	t.Setenv("MTKC_HELPER_EXIT", "1")
	var out, errb bytes.Buffer
	code := run([]string{"start", "--mode", "container", "--domain", "example.com", "--registration", "hz-1",
		"--credentials", agentSecret(t), "--config-dir", cfg, "--agent-dir", t.TempDir(), "--agent-cmd", helperCmd(t), "--timeout", "30s"}, &out, &errb)
	if code == 0 || !strings.Contains(errb.String(), "exited before it registered") {
		t.Fatalf("code %d stderr %q", code, errb.String())
	}
	if _, running := runningAgent(cfg); running {
		t.Fatal("a PID is still recorded for an agent that exited")
	}
}

func TestModeDefaults(t *testing.T) {
	for _, c := range []struct{ cmd, mode, dir string }{
		{"status", "container", "/opt/mtk-connect-agent/config"},
		{"stop", "host", "/usr/src/config"},
		{"status", "", "/usr/src/config"},
	} {
		o := options{mode: c.mode}
		if err := o.applyDefaults(c.cmd); err != nil || o.configDir != c.dir {
			t.Fatalf("%s --mode %q: dir %q err %v", c.cmd, c.mode, o.configDir, err)
		}
	}
	o := options{}
	if err := o.applyDefaults("start"); err == nil {
		t.Fatal("start without a mode must fail")
	}
}
