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
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"storage-mirror-sync/internal/report"
	"storage-mirror-sync/internal/runner"
)

// TestGitLFSAutoDetectAndFetch needs git-lfs on PATH; the upstream is a local file:// repository, which git-lfs 3.x
// serves through its standalone file transfer agent.
func TestGitLFSAutoDetectAndFetch(t *testing.T) {
	if _, err := exec.LookPath("git-lfs"); err != nil {
		t.Skip("git-lfs not installed")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("GIT_CONFIG_GLOBAL", filepath.Join(home, ".gitconfig"))
	git(t, home, "lfs", "install", "--skip-smudge")

	up := t.TempDir()
	work := t.TempDir()
	git(t, up, "init", "--bare", "-b", "main", ".")
	git(t, work, "clone", up, ".")
	git(t, work, "lfs", "track", "*.bin")
	blob := bytes.Repeat([]byte("lfs-payload-"), 1000)
	if err := os.WriteFile(filepath.Join(work, "big.bin"), blob, 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, work, "add", ".")
	git(t, work, "commit", "-m", "lfs")
	git(t, work, "push", "origin", "HEAD:main")
	oid := strings.TrimSpace(strings.Fields(git(t, work, "lfs", "ls-files", "--long"))[0])

	for _, mode := range []string{"auto", "enabled"} {
		c := baseConfig(t)
		c.Kind, c.GitURL, c.allowLocalURLs, c.LFS = "git", "file://"+up, true, mode
		rep := &report.Reporter{}
		a := &Agent{Cfg: c, Out: &bytes.Buffer{}, Run: runner.Run, Rep: rep}
		if err := a.Create(context.Background()); err != nil {
			t.Fatalf("%s: create: %v\n%s", mode, err, a.Out)
		}
		obj := filepath.Join(c.Path(), "lfs", "objects", oid[0:2], oid[2:4], oid)
		if _, err := os.Stat(obj); err != nil {
			t.Fatalf("%s: LFS object %s not fetched: %v\n%s", mode, obj, err, a.Out)
		}
		if !rep.LFSDetected() {
			t.Fatalf("%s: lfsDetected not reported", mode)
		}
		d, _, _ := c.Catalog().Read()
		if d.Get("m", "lfs") != mode || d.Get("m", "status") != "ready" {
			t.Fatalf("%s: catalog %v", mode, d.Entry("m"))
		}
	}

	// Auto on a repository without LFS attributes does not fetch and does not report LFS.
	plainUp := t.TempDir()
	plainWork := t.TempDir()
	git(t, plainUp, "init", "--bare", "-b", "main", ".")
	git(t, plainWork, "clone", plainUp, ".")
	_ = os.WriteFile(filepath.Join(plainWork, "a.txt"), []byte("a"), 0o644)
	git(t, plainWork, "add", ".")
	git(t, plainWork, "commit", "-m", "a")
	git(t, plainWork, "push", "origin", "HEAD:main")
	c := baseConfig(t)
	c.Kind, c.GitURL, c.allowLocalURLs, c.LFS = "git", "file://"+plainUp, true, "auto"
	rep := &report.Reporter{}
	a := &Agent{Cfg: c, Out: &bytes.Buffer{}, Run: runner.Run, Rep: rep}
	if err := a.Create(context.Background()); err != nil {
		t.Fatalf("plain create: %v\n%s", err, a.Out)
	}
	if rep.LFSDetected() {
		t.Fatal("auto must not detect LFS without filter=lfs")
	}
}
