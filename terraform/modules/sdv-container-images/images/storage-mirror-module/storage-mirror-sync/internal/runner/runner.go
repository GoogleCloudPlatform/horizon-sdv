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

// Package runner runs tools in their own process group with a hard timeout, optionally under a pseudo-terminal
// (repo and git only draw progress on a TTY), and hands every output line to a callback.
package runner

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
)

// ErrTimeout is returned when the hard timeout stopped the command (parity with timeout(1) exit 124).
var ErrTimeout = errors.New("timed out")

// Options configure one command.
type Options struct {
	Dir     string
	Env     []string
	TTY     bool
	Timeout time.Duration
	// Grace between TERM and KILL (30 s in utils.sh cleanup_repo_sync_process_group).
	Grace  time.Duration
	OnLine func(string)
	// Out receives the (masked) output; defaults to os.Stdout.
	Out io.Writer
}

// Result is the outcome of one command.
type Result struct {
	ExitCode int
	// Tail is the last 64 KiB of output (for error detection such as git lock errors).
	Tail string
}

var userinfoRe = regexp.MustCompile(`(https?://)[^/@\s:]+(:[^/@\s]*)?@`)

// Mask removes credentials from URLs in text.
func Mask(s string) string { return userinfoRe.ReplaceAllString(s, "${1}***@") }

// Run executes name with args.
func Run(ctx context.Context, opts Options, name string, args ...string) (Result, error) {
	if opts.Grace == 0 {
		opts.Grace = 30 * time.Second
	}
	out := opts.Out
	if out == nil {
		out = os.Stdout
	}
	runCtx := ctx
	var cancel context.CancelFunc
	if opts.Timeout > 0 {
		runCtx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}
	cmd := exec.Command(name, args...)
	cmd.Dir = opts.Dir
	cmd.Env = append(os.Environ(), opts.Env...)
	fmt.Fprintf(out, "+ %s %s\n", name, Mask(strings.Join(args, " ")))

	var reader io.Reader
	var closer func()
	done := make(chan error, 1)
	if opts.TTY {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true}
		f, err := pty.StartWithSize(cmd, &pty.Winsize{Rows: 50, Cols: 250})
		if err != nil {
			return Result{ExitCode: -1}, err
		}
		reader, closer = f, func() { f.Close() }
		go func() { done <- cmd.Wait() }()
	} else {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		pr, pw := io.Pipe()
		cmd.Stdout, cmd.Stderr = pw, pw
		if err := cmd.Start(); err != nil {
			return Result{ExitCode: -1}, err
		}
		reader, closer = pr, func() {}
		go func() {
			err := cmd.Wait()
			pw.Close()
			done <- err
		}()
	}

	tail := &tailBuffer{max: 64 << 10}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		sc := bufio.NewScanner(reader)
		sc.Buffer(make([]byte, 64<<10), 1<<20)
		sc.Split(splitCRLF)
		for sc.Scan() {
			line := Mask(sc.Text())
			tail.Write([]byte(line + "\n"))
			fmt.Fprintln(out, line)
			if opts.OnLine != nil {
				opts.OnLine(line)
			}
		}
	}()

	var timedOut bool
	select {
	case <-done:
	case <-runCtx.Done():
		timedOut = errors.Is(runCtx.Err(), context.DeadlineExceeded)
		stopGroup(cmd, opts.Grace, done)
	}
	closer()
	wg.Wait()
	res := Result{Tail: tail.String()}
	if cmd.ProcessState != nil {
		res.ExitCode = cmd.ProcessState.ExitCode()
	}
	if timedOut {
		return res, ErrTimeout
	}
	if ctx.Err() != nil {
		return res, ctx.Err()
	}
	if res.ExitCode != 0 {
		return res, fmt.Errorf("%s exited with %d", name, res.ExitCode)
	}
	return res, nil
}

// stopGroup sends TERM to the process group, then KILL after grace.
func stopGroup(cmd *exec.Cmd, grace time.Duration, done <-chan error) {
	if cmd.Process == nil {
		return
	}
	pgid := cmd.Process.Pid
	_ = syscall.Kill(-pgid, syscall.SIGTERM)
	select {
	case <-done:
		return
	case <-time.After(grace):
	}
	_ = syscall.Kill(-pgid, syscall.SIGKILL)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
	}
}

func splitCRLF(data []byte, atEOF bool) (int, []byte, error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.IndexAny(data, "\r\n"); i >= 0 {
		return i + 1, data[:i], nil
	}
	if atEOF {
		return len(data), data, nil
	}
	return 0, nil, nil
}

type tailBuffer struct {
	mu  sync.Mutex
	buf []byte
	max int
}

func (t *tailBuffer) Write(p []byte) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.buf = append(t.buf, p...)
	if len(t.buf) > t.max {
		t.buf = t.buf[len(t.buf)-t.max:]
	}
}

func (t *tailBuffer) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return string(t.buf)
}
