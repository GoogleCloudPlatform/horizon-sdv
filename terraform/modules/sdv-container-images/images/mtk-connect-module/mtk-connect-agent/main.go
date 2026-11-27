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

// Command mtkc-agent starts, reports and stops the MTK Connect node agent of one testbench on a device
// host or in the agent container. It accepts only the short-lived mtk-connect-agent credential from the
// <testbench>-agent Secret and never prints it. The registration token reaches the node agent through
// the child's environment only, never through its command line.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	agentUser = "mtk-connect-agent"

	registrationFile = "registration.name"
	pidFile          = "mtkc-agent.pid"
	logFile          = "mtkc-agent.log"

	stopGrace = 10 * time.Second
)

// domainName is a host name with an optional port; no scheme, path, user info or whitespace.
var domainName = regexp.MustCompile(`^[A-Za-z0-9]([-A-Za-z0-9]*[A-Za-z0-9])?(\.[A-Za-z0-9]([-A-Za-z0-9]*[A-Za-z0-9])?)*(:[0-9]{1,5})?$`)

// modeDefaults are the locations used by the MTK Connect agent installer (host) and by the agent
// container image (container).
var modeDefaults = map[string]struct{ configDir, agentDir, agentCmd string }{
	"host":      {configDir: "/usr/src/config", agentDir: "/opt/mtk-connect-agent", agentCmd: "node runAgent.js"},
	"container": {configDir: "/opt/mtk-connect-agent/config", agentDir: "/usr/src/app", agentCmd: "node index.js"},
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

type options struct {
	mode, domain, registration, credentials string
	configDir, agentDir, agentCmd           string
	timeout                                 time.Duration
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "usage: mtkc-agent start|status|stop")
		return 2
	}
	var o options
	fs := flag.NewFlagSet(args[0], flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&o.mode, "mode", "", "host or container")
	fs.StringVar(&o.domain, "domain", "", "Horizon domain")
	fs.StringVar(&o.registration, "registration", "", "registration name (status.registration of the testbench)")
	fs.StringVar(&o.credentials, "credentials", "", "the mounted <testbench>-agent Secret directory, or a JSON file with the same keys")
	fs.StringVar(&o.configDir, "config-dir", "", "agent config directory, where the agent writes registration.name")
	fs.StringVar(&o.agentDir, "agent-dir", "", "working directory of the node agent")
	fs.StringVar(&o.agentCmd, "agent-cmd", "", "node agent command line")
	fs.DurationVar(&o.timeout, "timeout", 2*time.Minute, "how long start waits for the agent to register")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}
	if err := o.applyDefaults(args[0]); err != nil {
		fmt.Fprintln(stderr, err.Error())
		return 1
	}
	switch args[0] {
	case "start":
		pid, err := start(o)
		if err != nil {
			fmt.Fprintln(stderr, err.Error())
			return 1
		}
		fmt.Fprintf(stdout, "mtkc-agent: agent running (%s, pid %d, registration %s)\n", o.mode, pid, o.registration)
		return 0
	case "status":
		pid, running := runningAgent(o.configDir)
		if !running {
			fmt.Fprintln(stdout, "mtkc-agent: not running")
			return 1
		}
		name, _ := os.ReadFile(filepath.Join(o.configDir, registrationFile))
		fmt.Fprintf(stdout, "mtkc-agent: running (pid %d, registration %s)\n", pid, strings.TrimSpace(string(name)))
		return 0
	case "stop":
		if err := stop(o.configDir); err != nil {
			fmt.Fprintln(stderr, err.Error())
			return 1
		}
		fmt.Fprintln(stdout, "mtkc-agent: stopped")
		return 0
	default:
		fmt.Fprintln(stderr, "usage: mtkc-agent start|status|stop")
		return 2
	}
}

// applyDefaults fills the directories and agent command from the mode. status and stop use the same
// config directory as start, so container mode does not read the host path; without a mode they use host.
func (o *options) applyDefaults(cmd string) error {
	mode := o.mode
	if mode == "" && cmd != "start" {
		mode = "host"
	}
	d, ok := modeDefaults[mode]
	if !ok {
		return fmt.Errorf("mode must be host or container")
	}
	if o.configDir == "" {
		o.configDir = d.configDir
	}
	if o.agentDir == "" {
		o.agentDir = d.agentDir
	}
	if o.agentCmd == "" {
		o.agentCmd = d.agentCmd
	}
	return nil
}

// start launches the node agent in its own process group, records its PID and waits until the agent
// has written registration.name for this registration.
func start(o options) (int, error) {
	if o.domain == "" || o.registration == "" || o.credentials == "" {
		return 0, fmt.Errorf("domain, registration and credentials are required")
	}
	if !domainName.MatchString(o.domain) {
		return 0, fmt.Errorf("domain %q must be a host name, without a scheme or path", o.domain)
	}
	cred, err := readCredentials(o.credentials)
	if err != nil {
		return 0, err
	}
	if cred.Username != agentUser {
		return 0, fmt.Errorf("refusing credentials for %q", cred.Username)
	}
	if cred.Password == "" {
		return 0, fmt.Errorf("credentials have no password")
	}
	if cred.Registration != o.registration {
		return 0, fmt.Errorf("credentials are for registration %q", cred.Registration)
	}
	if cred.Token == "" {
		return 0, fmt.Errorf("credentials have no registration token")
	}
	argv := strings.Fields(o.agentCmd)
	if len(argv) == 0 {
		return 0, fmt.Errorf("agent command is empty")
	}
	if pid, running := runningAgent(o.configDir); running {
		return 0, fmt.Errorf("an agent is already running (pid %d); run mtkc-agent stop first", pid)
	}
	if err := os.MkdirAll(o.configDir, 0o755); err != nil {
		return 0, err
	}
	// A registration.name left by an earlier run would look like a successful start.
	if err := os.Remove(filepath.Join(o.configDir, registrationFile)); err != nil && !os.IsNotExist(err) {
		return 0, err
	}
	logOut, err := os.OpenFile(filepath.Join(o.configDir, logFile), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return 0, err
	}
	defer logOut.Close()

	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = o.agentDir
	cmd.Env = agentEnv(os.Environ(), o.domain, o.registration, cred.Token)
	cmd.Stdout = logOut
	cmd.Stderr = logOut
	detach(cmd)
	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("start node agent: %w", err)
	}
	pid := cmd.Process.Pid
	if err := os.WriteFile(filepath.Join(o.configDir, pidFile), []byte(strconv.Itoa(pid)+"\n"+processIdentity(pid)+"\n"), 0o600); err != nil {
		_ = terminate(pid, true)
		return 0, err
	}
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()

	deadline := time.After(o.timeout)
	tick := time.NewTicker(250 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case err := <-exited:
			_ = os.Remove(filepath.Join(o.configDir, pidFile))
			return 0, fmt.Errorf("node agent exited before it registered (%v); see %s", err, filepath.Join(o.configDir, logFile))
		case <-deadline:
			_ = stop(o.configDir)
			return 0, fmt.Errorf("node agent did not register within %s; see %s", o.timeout, filepath.Join(o.configDir, logFile))
		case <-tick.C:
			b, err := os.ReadFile(filepath.Join(o.configDir, registrationFile))
			if err != nil {
				continue
			}
			name := strings.TrimSpace(string(b))
			if name == "" {
				continue
			}
			if name != o.registration {
				_ = stop(o.configDir)
				return 0, fmt.Errorf("node agent registered as %q, want %q", name, o.registration)
			}
			return pid, nil
		}
	}
}

// agentEnv is the node agent's environment: the caller's, without any inherited agent__ settings or MTK
// Connect credentials, plus the server URI, registration name and token.
//
// agent__uri and agent__log__appender are the names workloads/common/mtk-connect/mtk_connect.sh writes
// into the agent .env. agent__registration__name and agent__registration__token follow that same
// nesting. The node agent is not in this repo, so those two names are not confirmed by its source.
func agentEnv(base []string, domain, registration, token string) []string {
	env := make([]string, 0, len(base)+4)
	for _, kv := range base {
		if strings.HasPrefix(kv, "agent__") || strings.HasPrefix(kv, "MTK_CONNECT_PASSWORD=") {
			continue
		}
		env = append(env, kv)
	}
	return append(env,
		"agent__uri=https://"+domain+"/mtk-connect",
		"agent__log__appender=file",
		"agent__registration__name="+registration,
		"agent__registration__token="+token,
	)
}

// runningAgent returns the PID recorded by start if that process is still the node agent.
func runningAgent(configDir string) (int, bool) {
	b, err := os.ReadFile(filepath.Join(configDir, pidFile))
	if err != nil {
		return 0, false
	}
	lines := strings.SplitN(string(b), "\n", 3)
	pid, err := strconv.Atoi(strings.TrimSpace(lines[0]))
	if err != nil || pid <= 0 {
		return 0, false
	}
	identity := ""
	if len(lines) > 1 {
		identity = strings.TrimSpace(lines[1])
	}
	if !alive(pid) || (identity != "" && processIdentity(pid) != identity) {
		return pid, false
	}
	return pid, true
}

// stop terminates the node agent's process group, waits up to stopGrace, then kills it.
func stop(configDir string) error {
	pid, running := runningAgent(configDir)
	if running {
		if err := terminate(pid, false); err != nil {
			return fmt.Errorf("stop node agent (pid %d): %w", pid, err)
		}
		deadline := time.Now().Add(stopGrace)
		for alive(pid) && time.Now().Before(deadline) {
			time.Sleep(100 * time.Millisecond)
		}
		if alive(pid) {
			if err := terminate(pid, true); err != nil {
				return fmt.Errorf("kill node agent (pid %d): %w", pid, err)
			}
		}
	}
	for _, f := range []string{pidFile, registrationFile} {
		if err := os.Remove(filepath.Join(configDir, f)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

type credentials struct {
	Username     string `json:"username"`
	Password     string `json:"password"`
	Registration string `json:"registration"`
	Token        string `json:"token"`
}

// readCredentials reads a JSON file, or a directory of files as mounted from the host Secret.
// A read error names the path only, never the file contents.
func readCredentials(path string) (credentials, error) {
	info, err := os.Stat(path)
	if err != nil {
		return credentials{}, err
	}
	if !info.IsDir() {
		raw, err := os.ReadFile(path)
		if err != nil {
			return credentials{}, err
		}
		var cred credentials
		if err := json.Unmarshal(raw, &cred); err != nil {
			return credentials{}, fmt.Errorf("credentials file is not JSON")
		}
		return cred, nil
	}
	var cred credentials
	for _, f := range []struct {
		name string
		dst  *string
	}{
		{"username", &cred.Username},
		{"password", &cred.Password},
		{"registration", &cred.Registration},
		{"token", &cred.Token},
	} {
		if *f.dst, err = readKey(filepath.Join(path, f.name)); err != nil {
			return credentials{}, err
		}
	}
	return cred, nil
}

func readKey(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}
