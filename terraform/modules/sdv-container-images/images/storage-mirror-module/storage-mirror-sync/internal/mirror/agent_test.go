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

package mirror

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"storage-mirror-sync/internal/runner"
)

func baseConfig(t *testing.T) Config {
	t.Helper()
	mount := t.TempDir()
	return Config{
		Mirror: "m", Directory: "m", MountPath: mount, RootSubdir: "horizon-mirrors",
		DefaultManifestURL: "https://android.googlesource.com/platform/manifest", DefaultManifestRef: "android-16.0.0_r3",
		DefaultManifestFile: "default.xml", DefaultLFS: "disabled", GC: "none",
		MaxAttempts: 3, RetryDelay: time.Minute, SubmittedBy: "jane",
	}
}

type call struct {
	name string
	args []string
}

func TestRepoRetriesFollowJenkinsPolicy(t *testing.T) {
	c := baseConfig(t)
	c.Kind, c.SyncJobs = "repo", 4
	if err := os.MkdirAll(filepath.Join(c.Path(), ".repo"), 0o755); err != nil {
		t.Fatal(err)
	}
	var calls []call
	var slept []time.Duration
	attempt := 0
	a := &Agent{Cfg: c, Out: &bytes.Buffer{}, Sleep: func(d time.Duration) { slept = append(slept, d) },
		Run: func(_ context.Context, _ runner.Options, name string, args ...string) (runner.Result, error) {
			calls = append(calls, call{name, args})
			attempt++
			switch attempt {
			case 1:
				_ = os.WriteFile(filepath.Join(c.Path(), "packed-refs.lock"), nil, 0o644)
				return runner.Result{Tail: "error: cannot lock ref 'refs/heads/main.lock': File exists"}, errors.New("exit 1")
			case 2:
				return runner.Result{Tail: "HTTP 429"}, errors.New("exit 1")
			}
			return runner.Result{}, nil
		}}
	if err := a.Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 3 || calls[0].name != "repo" || calls[0].args[0] != "sync" {
		t.Fatalf("calls %+v", calls)
	}
	if !strings.HasPrefix(calls[2].args[1], "-j1") {
		t.Fatalf("last attempt must use -j1: %v", calls[2].args)
	}
	if len(slept) != 1 {
		t.Fatalf("lock retry has no pause, the generic retry pauses once: %v", slept)
	}
	if _, err := os.Stat(filepath.Join(c.Path(), "packed-refs.lock")); err == nil {
		t.Fatal("stale lock not removed")
	}
	d, _, _ := c.Catalog().Read()
	e := d.Entry("m")
	if e["status"] != "ready" || e["type"] != "repo" || e["repo_sync_jobs"] != "1" || e["created_by"] != "jane" || e["managed_by"] != ManagedBy || !strings.Contains(e["last_successful_sync_time"], "UTC") {
		t.Fatalf("catalog %v", e)
	}
}

func TestTimeoutIsNotRetried(t *testing.T) {
	c := baseConfig(t)
	c.Kind = "repo"
	_ = os.MkdirAll(filepath.Join(c.Path(), ".repo"), 0o755)
	n := 0
	a := &Agent{Cfg: c, Out: &bytes.Buffer{}, Sleep: func(time.Duration) {},
		Run: func(context.Context, runner.Options, string, ...string) (runner.Result, error) {
			n++
			return runner.Result{}, runner.ErrTimeout
		}}
	if err := a.Sync(context.Background()); err == nil || n != 1 {
		t.Fatalf("timeout: err=%v calls=%d", err, n)
	}
	d, _, _ := c.Catalog().Read()
	if d.Get("m", "status") != "error" {
		t.Fatal("status must be error")
	}
}

func TestManifestURLIsImmutable(t *testing.T) {
	c := baseConfig(t)
	c.Kind, c.ManifestURL = "repo", "https://android.googlesource.com/platform/manifest"
	_ = os.MkdirAll(c.Root(), 0o755)
	_ = os.WriteFile(c.Catalog().Path(), []byte("horizon-mirrors:\n  m:\n    type: repo\n    manifest_url: https://example.com/other\n"), 0o644)
	a := &Agent{Cfg: c, Out: &bytes.Buffer{}, Run: func(context.Context, runner.Options, string, ...string) (runner.Result, error) {
		t.Fatal("must not run")
		return runner.Result{}, nil
	}}
	if err := a.Sync(context.Background()); err == nil || !strings.Contains(err.Error(), "cannot change the manifest URL") {
		t.Fatalf("got %v", err)
	}
}

func TestSyncUsesCatalogParameters(t *testing.T) {
	c := baseConfig(t)
	_ = os.MkdirAll(filepath.Join(c.Path(), ".repo"), 0o755)
	_ = os.WriteFile(c.Catalog().Path(), []byte("horizon-mirrors:\n  m:\n    type: repo\n    manifest_url: https://example.com/manifest\n    manifest_ref: main\n    manifest_file: x.xml\n    repo_sync_jobs: \"2\"\n"), 0o644)
	var args []string
	a := &Agent{Cfg: c, Out: &bytes.Buffer{}, Run: func(_ context.Context, _ runner.Options, _ string, a ...string) (runner.Result, error) {
		args = a
		return runner.Result{}, nil
	}}
	if err := a.Sync(context.Background()); err != nil {
		t.Fatal(err)
	}
	if args[1] != "-j2" {
		t.Fatalf("jobs from catalog: %v", args)
	}
	d, _, _ := c.Catalog().Read()
	if d.Get("m", "manifest_url") != "https://example.com/manifest" || d.Get("m", "manifest_file") != "x.xml" {
		t.Fatalf("%v", d.Entry("m"))
	}
}

func TestDeleteNeedsConfirm(t *testing.T) {
	c := baseConfig(t)
	_ = os.MkdirAll(filepath.Join(c.Path(), "x.git"), 0o755)
	_ = os.WriteFile(c.Catalog().Path(), []byte("horizon-mirrors:\n  m:\n    type: repo\n  keep:\n    type: repo\n"), 0o644)
	a := &Agent{Cfg: c, Out: &bytes.Buffer{}}
	if err := a.Delete(context.Background()); err == nil {
		t.Fatal("delete without confirm must fail")
	}
	a.Cfg.Confirm = "m"
	if err := a.Delete(context.Background()); err != nil {
		t.Fatal(err)
	}
	if p := a.Tracker.Snapshot(); p.BytesReceived != 0 || p.BytesFreed < 0 || p.Phase != "delete" || p.State != "succeeded" {
		t.Fatalf("delete progress %+v", p)
	}
	if _, err := os.Stat(c.Path()); !os.IsNotExist(err) {
		t.Fatal("directory still there")
	}
	d, _, _ := c.Catalog().Read()
	if d.Has("m") || !d.Has("keep") {
		t.Fatalf("catalog %v", d.Names())
	}
}

func TestValidateDirectory(t *testing.T) {
	for d, ok := range map[string]bool{"aosp-16": true, "My_Mirror.1": true, "../x": false, ".hidden": false, "a/b": false, "metadata.yaml": false, "": false} {
		if (ValidateDirectory(d) == nil) != ok {
			t.Errorf("%q", d)
		}
	}
}

func TestParseTimeout(t *testing.T) {
	for in, want := range map[string]time.Duration{"20h": 20 * time.Hour, "90m": 90 * time.Minute, "3600": time.Hour, "1d": 24 * time.Hour, "1h30m": 90 * time.Minute} {
		if got, err := ParseTimeout(in); err != nil || got != want {
			t.Errorf("%s: %v %v", in, got, err)
		}
	}
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

// TestGitMirrorEndToEnd runs create, sync and delete against a local upstream with the real git binary.
func TestGitMirrorEndToEnd(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	up := t.TempDir()
	work := t.TempDir()
	git(t, up, "init", "--bare", "-b", "main", ".")
	git(t, work, "clone", up, ".")
	_ = os.WriteFile(filepath.Join(work, "a.txt"), []byte("a"), 0o644)
	git(t, work, "add", ".")
	git(t, work, "commit", "-m", "a")
	git(t, work, "push", "origin", "HEAD:main", "HEAD:refs/heads/release", "HEAD:refs/pull/1/head")
	git(t, work, "tag", "v1")
	git(t, work, "push", "origin", "v1")

	c := baseConfig(t)
	c.Kind, c.GitURL, c.allowLocalURLs = "git", "file://"+up, true
	c.ExcludeRefs = []string{"refs/pull/*"}
	c.GC = "auto"
	a := &Agent{Cfg: c, Out: &bytes.Buffer{}, Run: runner.Run}
	if err := a.Create(context.Background()); err != nil {
		t.Fatalf("create: %v\n%s", err, a.Out)
	}
	refs := git(t, c.Path(), "for-each-ref", "--format=%(refname)")
	if !strings.Contains(refs, "refs/heads/main") || !strings.Contains(refs, "refs/tags/v1") || strings.Contains(refs, "refs/pull/") {
		t.Fatalf("refs after create:\n%s", refs)
	}
	d, _, _ := c.Catalog().Read()
	if e := d.Entry("m"); e["type"] != "git" || e["status"] != "ready" || e["repository_url"] != "file://"+up || e["branches"] != "*" {
		t.Fatalf("catalog %v", e)
	}

	_ = os.WriteFile(filepath.Join(work, "b.txt"), []byte("b"), 0o644)
	git(t, work, "add", ".")
	git(t, work, "commit", "-m", "b")
	git(t, work, "push", "origin", "HEAD:main")
	want := git(t, work, "rev-parse", "HEAD")
	a2 := &Agent{Cfg: c, Out: &bytes.Buffer{}, Run: runner.Run}
	if err := a2.Sync(context.Background()); err != nil {
		t.Fatalf("sync: %v\n%s", err, a2.Out)
	}
	if got := git(t, c.Path(), "rev-parse", "refs/heads/main"); got != want {
		t.Fatalf("main %s, want %s", got, want)
	}
	if a2.Tracker.Snapshot().OverallPercent != 100 {
		t.Fatalf("final progress %+v", a2.Tracker.Snapshot())
	}

	c.Confirm = "m"
	if err := (&Agent{Cfg: c, Out: &bytes.Buffer{}}).Delete(context.Background()); err != nil {
		t.Fatal(err)
	}
	d, _, _ = c.Catalog().Read()
	if d.Has("m") {
		t.Fatal("entry left after delete")
	}
}

// TestGitBranchSubset mirrors only the selected branches (git init --bare + refspecs path).
func TestGitBranchSubset(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	up := t.TempDir()
	work := t.TempDir()
	git(t, up, "init", "--bare", "-b", "main", ".")
	git(t, work, "clone", up, ".")
	_ = os.WriteFile(filepath.Join(work, "a.txt"), []byte("a"), 0o644)
	git(t, work, "add", ".")
	git(t, work, "commit", "-m", "a")
	git(t, work, "push", "origin", "HEAD:main", "HEAD:refs/heads/release/1", "HEAD:refs/heads/feature")
	c := baseConfig(t)
	c.Kind, c.GitURL, c.allowLocalURLs = "git", "file://"+up, true
	c.GitBranches = []string{"main", "release/*"}
	a := &Agent{Cfg: c, Out: &bytes.Buffer{}, Run: runner.Run}
	if err := a.Create(context.Background()); err != nil {
		t.Fatalf("create: %v\n%s", err, a.Out)
	}
	refs := git(t, c.Path(), "for-each-ref", "--format=%(refname)", "refs/heads")
	if !strings.Contains(refs, "refs/heads/main") || !strings.Contains(refs, "refs/heads/release/1") || strings.Contains(refs, "feature") {
		t.Fatalf("refs:\n%s", refs)
	}
}
