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
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"storage-mirror-sync/internal/catalog"
	"storage-mirror-sync/internal/progress"
	"storage-mirror-sync/internal/report"
	"storage-mirror-sync/internal/runner"
)

// ManagedBy marks catalog entries written by the module.
const ManagedBy = "storage-mirror-module"

// gitLockErr is the git lock error utils.sh retries after removing stale locks.
var gitLockErr = regexp.MustCompile(`cannot lock ref '[^']*': .*File exists|Unable to create '[^']*\.lock': File exists`)

// RunFunc runs one external command (runner.Run in production).
type RunFunc func(ctx context.Context, opts runner.Options, name string, args ...string) (runner.Result, error)

// Agent runs one operation.
type Agent struct {
	Cfg     Config
	Tracker *progress.Tracker
	Rep     *report.Reporter
	Run     RunFunc
	Sleep   func(time.Duration)
	Now     func() time.Time
	Out     io.Writer
	// env is added to every git and repo command (credentials, prompts off).
	env []string
}

func (a *Agent) now() time.Time {
	if a.Now != nil {
		return a.Now()
	}
	return time.Now()
}

func (a *Agent) sleep(d time.Duration) {
	if a.Sleep != nil {
		a.Sleep(d)
		return
	}
	time.Sleep(d)
}

func (a *Agent) out() io.Writer {
	if a.Out != nil {
		return a.Out
	}
	return os.Stdout
}

func (a *Agent) run(ctx context.Context, opts runner.Options, name string, args ...string) (runner.Result, error) {
	opts.Env = append(opts.Env, a.env...)
	if opts.Out == nil {
		opts.Out = a.out()
	}
	if a.Run != nil {
		return a.Run(ctx, opts, name, args...)
	}
	return runner.Run(ctx, opts, name, args...)
}

// gitQuiet runs a git plumbing command and returns its stdout (not logged: ref lists can be huge).
func (a *Agent) gitQuiet(ctx context.Context, dir string, stdin string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	cmd.Env = append(append(os.Environ(), a.env...), "GIT_TERMINAL_PROMPT=0")
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	b, err := cmd.Output()
	return string(b), err
}

func (a *Agent) logf(format string, args ...interface{}) {
	fmt.Fprintf(a.out(), "[storage-mirror] "+format+"\n", args...)
}

// Create initialises a mirror and runs the first sync.
func (a *Agent) Create(ctx context.Context) error { return a.syncFlow(ctx, true) }

// Sync updates a mirror; a missing mirror is created first (parity with the Jenkins Sync Mirror job).
func (a *Agent) Sync(ctx context.Context) error { return a.syncFlow(ctx, false) }

func (a *Agent) syncFlow(ctx context.Context, create bool) (err error) {
	c := &a.Cfg
	if c.Mirror == "" {
		return errors.New("parameter mirror is required")
	}
	if err := ValidateDirectory(c.Directory); err != nil {
		return err
	}
	cat := c.Catalog()
	doc, _, err := cat.Read()
	if err != nil {
		return err
	}
	entry := doc.Entry(c.Directory)
	if create && entry != nil {
		a.logf("catalog entry %s exists; create continues as sync", c.Directory)
	}
	if err := c.ApplyDefaults(entry); err != nil {
		return err
	}
	if entry != nil {
		if err := immutableMatches(c, entry); err != nil {
			return err
		}
	}
	a.Tracker = progress.NewTracker(c.Kind, int32(c.MaxAttempts), a.Now)
	if a.Rep != nil {
		loopCtx, stop := context.WithCancel(ctx)
		defer stop()
		go a.Rep.Loop(loopCtx, a.Tracker, 5*time.Second)
	}
	defer func() { a.finish(context.WithoutCancel(ctx), err) }()

	if err := a.setupCredentials(); err != nil {
		return err
	}
	if _, err := cat.Update(func(d *catalog.Doc) error {
		if !d.Has(c.Directory) {
			d.Set(c.Directory, insertFields(c, a.createdBy())...)
		}
		d.Set(c.Directory, "managed_by", ManagedBy, "lfs", c.LFS)
		return nil
	}); err != nil {
		return err
	}
	if err := os.MkdirAll(c.Path(), 0o2775); err != nil {
		return err
	}

	a.Tracker.SetPhase(progress.PhaseInit)
	if c.Kind == "repo" {
		err = a.initRepo(ctx)
	} else {
		err = a.initGit(ctx)
	}
	if err != nil {
		return err
	}
	if _, err := cat.Update(func(d *catalog.Doc) error {
		if c.Kind == "repo" {
			d.Set(c.Directory, "manifest_ref", c.ManifestRef, "manifest_file", c.ManifestFile, "repo_sync_jobs", strconv.Itoa(c.SyncJobs))
		} else {
			d.Set(c.Directory, "branches", strings.Join(c.GitBranches, ","))
		}
		return nil
	}); err != nil {
		return err
	}
	if err := a.fetchWithRetries(ctx); err != nil {
		return err
	}
	if err := a.pruneRefs(ctx); err != nil {
		return err
	}
	if err := a.lfs(ctx); err != nil {
		return err
	}
	if err := a.gc(ctx); err != nil {
		return err
	}
	now := a.now().UTC()
	_, err = cat.Update(func(d *catalog.Doc) error {
		d.Set(c.Directory,
			"status", "ready",
			"last_successful_sync_time", now.Format(catalog.JenkinsTimeLayout),
			"last_successful_sync_time_rfc3339", now.Format(time.RFC3339))
		return nil
	})
	return err
}

func (a *Agent) createdBy() string {
	if a.Cfg.SubmittedBy != "" {
		return a.Cfg.SubmittedBy
	}
	return ManagedBy
}

func insertFields(c *Config, by string) []string {
	if c.Kind == "git" {
		return []string{"name", c.Directory, "type", "git", "repository_url", c.GitURL, "branches", strings.Join(c.GitBranches, ","),
			"status", "uninitialized", "last_successful_sync_time", "", "created_by", by}
	}
	return []string{"name", c.Directory, "type", "repo", "manifest_url", c.ManifestURL, "manifest_ref", c.ManifestRef,
		"manifest_file", c.ManifestFile, "repo_sync_jobs", strconv.Itoa(c.SyncJobs), "status", "uninitialized",
		"last_successful_sync_time", "", "created_by", by}
}

// immutableMatches enforces "the manifest URL of an existing mirror cannot change" (utils.sh:350-369).
func immutableMatches(c *Config, e map[string]string) error {
	if t := e["type"]; t != "" && t != c.Kind {
		return fmt.Errorf("mirror %s is of type %s, not %s", c.Directory, t, c.Kind)
	}
	if c.Kind == "repo" && e["manifest_url"] != "" && e["manifest_url"] != c.ManifestURL {
		return fmt.Errorf("you cannot change the manifest URL of an existing mirror (%s); create a new mirror instead", e["manifest_url"])
	}
	if c.Kind == "git" && e["repository_url"] != "" && e["repository_url"] != c.GitURL {
		return fmt.Errorf("you cannot change the repository URL of an existing mirror (%s); create a new mirror instead", runner.Mask(e["repository_url"]))
	}
	return nil
}

func (a *Agent) finish(ctx context.Context, err error) {
	c := a.Cfg
	state := progress.StateSucceeded
	msg := ""
	switch {
	case errors.Is(err, context.Canceled):
		state, msg = progress.StateCancelled, "cancelled"
	case err != nil:
		state, msg = progress.StateFailed, err.Error()
	}
	if err != nil {
		if _, uerr := c.Catalog().Update(func(d *catalog.Doc) error {
			if d.Has(c.Directory) {
				d.Set(c.Directory, "status", "error")
			}
			return nil
		}); uerr != nil {
			log.Printf("catalog status: %v", uerr)
		}
	}
	if a.Tracker != nil {
		a.Tracker.Update(func(p *progress.Progress) { p.State, p.Message = state, runner.Mask(msg) })
	}
	a.reportCatalog(ctx)
	if a.Rep != nil && a.Tracker != nil {
		a.Rep.Publish(ctx, a.Tracker.Snapshot(), true)
	}
}

func (a *Agent) reportCatalog(ctx context.Context) {
	if a.Rep == nil {
		return
	}
	_, b, err := a.Cfg.Catalog().Read()
	if err != nil {
		log.Printf("catalog read: %v", err)
		return
	}
	usage, _ := report.StatFS(a.Cfg.MountPath)
	a.Rep.Catalog(ctx, b, usage)
}

func (a *Agent) setupCredentials() error {
	a.env = append(a.env, "GIT_TERMINAL_PROMPT=0")
	dir := a.Cfg.GitCredentialsDir
	if dir == "" || a.Cfg.Kind != "git" {
		return nil
	}
	user, err1 := os.ReadFile(filepath.Join(dir, "username"))
	pass, err2 := os.ReadFile(filepath.Join(dir, "password"))
	if err1 != nil || err2 != nil {
		return fmt.Errorf("credentials Secret must have username and password keys")
	}
	home, err := os.MkdirTemp("", "sm-git-")
	if err != nil {
		return err
	}
	host := strings.TrimPrefix(a.Cfg.GitURL, "https://")
	if i := strings.IndexByte(host, '/'); i >= 0 {
		host = host[:i]
	}
	store := filepath.Join(home, "credentials")
	line := fmt.Sprintf("https://%s:%s@%s\n", urlEscape(strings.TrimSpace(string(user))), urlEscape(strings.TrimSpace(string(pass))), host)
	if err := os.WriteFile(store, []byte(line), 0o600); err != nil {
		return err
	}
	cfg := filepath.Join(home, "gitconfig")
	if err := os.WriteFile(cfg, []byte("[credential]\n\thelper = store --file="+store+"\n"), 0o600); err != nil {
		return err
	}
	a.env = append(a.env, "GIT_CONFIG_GLOBAL="+cfg)
	return nil
}

func urlEscape(s string) string {
	r := strings.NewReplacer("%", "%25", "@", "%40", ":", "%3A", "/", "%2F")
	return r.Replace(s)
}

func (a *Agent) initRepo(ctx context.Context) error {
	c := a.Cfg
	if dirExists(filepath.Join(c.Path(), ".repo")) {
		a.logf("'.repo' found in %s; reusing the existing mirror", c.Path())
		return nil
	}
	args := []string{"init", "-u", c.ManifestURL, "-b", c.ManifestRef, "-m", c.ManifestFile, "--mirror"}
	if c.LFS == "enabled" {
		args = append(args, "--git-lfs")
	}
	if _, err := a.run(ctx, runner.Options{Dir: c.Path(), Timeout: time.Hour}, "repo", args...); err != nil {
		return fmt.Errorf("repo init: %w", err)
	}
	return nil
}

func isBareRepo(p string) bool {
	return fileExists(filepath.Join(p, "HEAD")) && dirExists(filepath.Join(p, "objects"))
}

func (a *Agent) initGit(ctx context.Context) error {
	c := a.Cfg
	p := c.Path()
	if isBareRepo(p) {
		url, _ := a.gitQuiet(ctx, p, "", "config", "--get", "remote.origin.url")
		if u := strings.TrimSpace(url); u != "" && u != c.GitURL {
			return fmt.Errorf("existing mirror %s fetches %s, not %s", c.Directory, runner.Mask(u), runner.Mask(c.GitURL))
		}
		return a.configureRefspecs(ctx)
	}
	entries, _ := os.ReadDir(p)
	if len(entries) > 0 {
		return fmt.Errorf("directory %s is not empty and not a git mirror", p)
	}
	if allBranches(c.GitBranches) {
		// git clone --mirror + git remote update (acceptance criterion); the clone is the first fetch.
		if _, err := a.run(ctx, runner.Options{TTY: true, Timeout: c.SyncTimeout, OnLine: a.onLine}, "git", "clone", "--mirror", "--progress", c.GitURL, p); err != nil {
			return fmt.Errorf("git clone --mirror: %w", err)
		}
		return nil
	}
	if _, err := a.run(ctx, runner.Options{}, "git", "init", "--bare", p); err != nil {
		return err
	}
	if _, err := a.gitQuiet(ctx, p, "", "remote", "add", "origin", c.GitURL); err != nil {
		return fmt.Errorf("git remote add: %w", err)
	}
	return a.configureRefspecs(ctx)
}

func allBranches(b []string) bool { return len(b) == 1 && b[0] == "*" }

// configureRefspecs maps the branch patterns onto remote.origin.fetch; tags are always mirrored.
func (a *Agent) configureRefspecs(ctx context.Context) error {
	c := a.Cfg
	p := c.Path()
	if allBranches(c.GitBranches) {
		_, _ = a.gitQuiet(ctx, p, "", "config", "--unset-all", "remote.origin.fetch")
		_, err := a.gitQuiet(ctx, p, "", "config", "--add", "remote.origin.fetch", "+refs/*:refs/*")
		if err == nil {
			_, err = a.gitQuiet(ctx, p, "", "config", "remote.origin.mirror", "true")
		}
		return err
	}
	_, _ = a.gitQuiet(ctx, p, "", "config", "--unset-all", "remote.origin.fetch")
	_, _ = a.gitQuiet(ctx, p, "", "config", "--unset", "remote.origin.mirror")
	for _, b := range c.GitBranches {
		if strings.ContainsAny(b, " \t:") {
			return fmt.Errorf("invalid branch pattern %q", b)
		}
		if _, err := a.gitQuiet(ctx, p, "", "config", "--add", "remote.origin.fetch", fmt.Sprintf("+refs/heads/%s:refs/heads/%s", b, b)); err != nil {
			return err
		}
	}
	_, err := a.gitQuiet(ctx, p, "", "config", "--add", "remote.origin.fetch", "+refs/tags/*:refs/tags/*")
	return err
}

func (a *Agent) onLine(line string) { a.Tracker.ParseLine(line) }

// fetchWithRetries is utils.sh sync_mirror_with_retries: 3 attempts, lock cleanup on git lock errors,
// otherwise jobs drop to 3 and then 1 with a 60 s pause, and no retry after a timeout.
func (a *Agent) fetchWithRetries(ctx context.Context) error {
	c := a.Cfg
	jobs := c.SyncJobs
	if jobs < 1 {
		jobs = 1
	}
	if n := runtime.NumCPU(); jobs > n {
		a.logf("repo_sync_jobs %d is higher than the %d cores; using %d", jobs, n, n)
		jobs = n
	}
	var lastErr error
	for attempt := 1; attempt <= c.MaxAttempts; attempt++ {
		a.Tracker.SetPhase(progress.PhaseFetch)
		a.Tracker.Update(func(p *progress.Progress) { p.Attempt = int32(attempt) })
		if _, err := c.Catalog().Update(func(d *catalog.Doc) error {
			d.Set(c.Directory, "status", "syncing", "last_sync_attempt_time_rfc3339", a.now().UTC().Format(time.RFC3339))
			return nil
		}); err != nil {
			return err
		}
		a.logf("fetch attempt %d of %d", attempt, c.MaxAttempts)
		var res runner.Result
		var err error
		if c.Kind == "repo" {
			res, err = a.run(ctx, runner.Options{Dir: c.Path(), TTY: true, Timeout: c.SyncTimeout, OnLine: a.onLine},
				"repo", "sync", "-j"+strconv.Itoa(jobs), "--optimized-fetch", "--prune", "--retry-fetches=3", "--auto-gc", "--no-clone-bundle")
		} else {
			res, err = a.run(ctx, runner.Options{Dir: c.Path(), TTY: true, Timeout: c.SyncTimeout, OnLine: a.onLine},
				"git", "remote", "update", "--prune")
		}
		if err == nil {
			return nil
		}
		lastErr = err
		if errors.Is(err, context.Canceled) {
			a.removeLocks()
			return err
		}
		if errors.Is(err, runner.ErrTimeout) {
			a.removeLocks()
			return fmt.Errorf("fetch timed out after %s; not retrying", c.SyncTimeout)
		}
		if attempt == c.MaxAttempts {
			break
		}
		a.Tracker.Update(func(p *progress.Progress) { p.State, p.Message = progress.StateRetrying, err.Error() })
		if gitLockErr.MatchString(res.Tail) {
			a.logf("git lock error detected; removing stale lock files")
			a.removeLocks()
			continue
		}
		if c.Kind == "repo" {
			if attempt == 1 && jobs > 3 {
				jobs = 3
			} else if attempt == c.MaxAttempts-1 {
				jobs = 1
			}
			a.logf("reducing parallel sync jobs to %d", jobs)
			_, _ = c.Catalog().Update(func(d *catalog.Doc) error {
				d.Set(c.Directory, "repo_sync_jobs", strconv.Itoa(jobs))
				return nil
			})
		}
		a.logf("waiting %s before the next attempt", c.RetryDelay)
		a.sleep(c.RetryDelay)
	}
	return fmt.Errorf("fetch failed after %d attempts: %w", c.MaxAttempts, lastErr)
}

// removeLocks deletes *.lock files under the mirror (utils.sh remove_stale_git_locks).
func (a *Agent) removeLocks() {
	_ = filepath.WalkDir(a.Cfg.Path(), func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(d.Name(), ".lock") {
			if os.Remove(p) == nil {
				a.logf("removed %s", p)
			}
		}
		return nil
	})
}

// Projects lists the bare repositories of the mirror (the mirror itself for git kind).
func (a *Agent) Projects() []string {
	p := a.Cfg.Path()
	if a.Cfg.Kind == "git" {
		return []string{p}
	}
	var out []string
	_ = filepath.WalkDir(p, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return nil
		}
		if d.Name() == ".repo" && filepath.Dir(path) == p {
			return filepath.SkipDir
		}
		if strings.HasSuffix(d.Name(), ".git") && isBareRepo(path) {
			out = append(out, path)
			return filepath.SkipDir
		}
		return nil
	})
	return out
}

func (a *Agent) forProjects(ctx context.Context, phase string, fn func(ctx context.Context, project string) error) error {
	a.Tracker.SetPhase(phase)
	projects := a.Projects()
	for i, p := range projects {
		if err := ctx.Err(); err != nil {
			return err
		}
		rel, _ := filepath.Rel(a.Cfg.Path(), p)
		a.Tracker.SetCounts(int64(i), int64(len(projects)), rel)
		if err := fn(ctx, p); err != nil {
			return fmt.Errorf("%s %s: %w", phase, rel, err)
		}
	}
	a.Tracker.SetCounts(int64(len(projects)), int64(len(projects)), "")
	return nil
}

func (a *Agent) pruneRefs(ctx context.Context) error {
	patterns := append([]string{}, a.Cfg.PruneRefs...)
	if a.Cfg.Kind == "git" {
		patterns = append(patterns, a.Cfg.ExcludeRefs...)
	}
	if len(patterns) == 0 {
		return nil
	}
	// for-each-ref wildcards do not cross "/"; "refs/changes/*" means every ref under refs/changes/.
	for i, p := range patterns {
		if strings.HasSuffix(p, "/*") {
			patterns[i] = strings.TrimSuffix(p, "*")
		}
	}
	return a.forProjects(ctx, progress.PhasePruneRefs, func(ctx context.Context, p string) error {
		refs, err := a.gitQuiet(ctx, p, "", append([]string{"for-each-ref", "--format=%(refname)"}, patterns...)...)
		if err != nil || strings.TrimSpace(refs) == "" {
			return err
		}
		var stdin strings.Builder
		for _, r := range strings.Fields(refs) {
			stdin.WriteString("delete " + r + "\n")
		}
		_, err = a.gitQuiet(ctx, p, stdin.String(), "update-ref", "--stdin")
		return err
	})
}

// lfsUsed reports whether any branch tip has a .gitattributes with filter=lfs.
func (a *Agent) lfsUsed(ctx context.Context, project string) bool {
	tips, err := a.gitQuiet(ctx, project, "", "for-each-ref", "--format=%(objectname)", "refs/heads")
	if err != nil {
		return false
	}
	for _, sha := range strings.Fields(tips) {
		attrs, err := a.gitQuiet(ctx, project, "", "cat-file", "-p", sha+":.gitattributes")
		if err == nil && strings.Contains(attrs, "filter=lfs") {
			return true
		}
	}
	return false
}

func (a *Agent) lfs(ctx context.Context) error {
	mode := a.Cfg.LFS
	if mode == "disabled" || mode == "" {
		return nil
	}
	return a.forProjects(ctx, progress.PhaseLFS, func(ctx context.Context, p string) error {
		if mode == "auto" && !a.lfsUsed(ctx, p) {
			return nil
		}
		if a.Rep != nil {
			a.Rep.SetLFSDetected()
		}
		heads, err := a.gitQuiet(ctx, p, "", "for-each-ref", "--format=%(refname)", "refs/heads")
		if err != nil {
			return err
		}
		refs := strings.Fields(heads)
		for len(refs) > 0 {
			n := min(50, len(refs))
			args := append([]string{"lfs", "fetch", "origin"}, refs[:n]...)
			if _, err := a.run(ctx, runner.Options{Dir: p, OnLine: a.onLine, Timeout: a.Cfg.SyncTimeout}, "git", args...); err != nil {
				return err
			}
			refs = refs[n:]
		}
		return nil
	})
}

func (a *Agent) gc(ctx context.Context) error {
	var args []string
	switch a.Cfg.GC {
	case "none":
		return nil
	case "auto":
		args = []string{"gc", "--auto", "--quiet"}
	default:
		args = []string{"gc", "--aggressive", "--prune=all", "--quiet"}
	}
	return a.forProjects(ctx, progress.PhaseGC, func(ctx context.Context, p string) error {
		_, err := a.gitQuiet(ctx, p, "", args...)
		return err
	})
}

// Delete removes the mirror directory and its catalog entry (Jenkins Delete Mirror, single directory).
func (a *Agent) Delete(ctx context.Context) (err error) {
	c := a.Cfg
	if err := ValidateDirectory(c.Directory); err != nil {
		return err
	}
	if c.Confirm != c.Directory {
		return fmt.Errorf("confirm must equal the mirror directory %q", c.Directory)
	}
	a.Tracker = progress.NewTracker("delete", 1, a.Now)
	if a.Rep != nil {
		loopCtx, stop := context.WithCancel(ctx)
		defer stop()
		go a.Rep.Loop(loopCtx, a.Tracker, 5*time.Second)
	}
	defer func() { a.finishDelete(context.WithoutCancel(ctx), err) }()
	a.Tracker.SetPhase(progress.PhaseDelete)
	before, _ := report.StatFS(c.MountPath)
	if !dirExists(c.Path()) {
		a.logf("mirror directory %s does not exist; removing the catalog entry only", c.Path())
	} else {
		stop := make(chan struct{})
		go func() {
			t := time.NewTicker(10 * time.Second)
			defer t.Stop()
			for {
				select {
				case <-stop:
					return
				case <-t.C:
					if u, err := report.StatFS(c.MountPath); err == nil {
						a.Tracker.Update(func(p *progress.Progress) { p.BytesFreed = u.AvailableBytes - before.AvailableBytes })
					}
				}
			}
		}()
		err := os.RemoveAll(c.Path())
		close(stop)
		if err != nil {
			return fmt.Errorf("delete %s: %w", c.Path(), err)
		}
		if u, err := report.StatFS(c.MountPath); err == nil {
			a.Tracker.Update(func(p *progress.Progress) { p.BytesFreed = max(0, u.AvailableBytes-before.AvailableBytes) })
		}
	}
	_ = os.Remove(filepath.Join(c.Root(), ".storage-mirror", "progress", c.Directory+".json"))
	_, err = c.Catalog().Update(func(d *catalog.Doc) error {
		d.Delete(c.Directory)
		return nil
	})
	return err
}

func (a *Agent) finishDelete(ctx context.Context, err error) {
	state := progress.StateSucceeded
	msg := ""
	if err != nil {
		state, msg = progress.StateFailed, err.Error()
	}
	a.Tracker.Update(func(p *progress.Progress) { p.State, p.Message, p.PhasePercent = state, msg, 100 })
	a.reportCatalog(ctx)
	if a.Rep != nil {
		a.Rep.Publish(ctx, a.Tracker.Snapshot(), true)
	}
}

// InitVolume creates the layout and the empty catalog if missing, then reports the catalog.
func (a *Agent) InitVolume(ctx context.Context) error {
	c := a.Cfg
	if err := os.MkdirAll(filepath.Join(c.Root(), ".storage-mirror", "progress"), 0o2775); err != nil {
		return err
	}
	if _, err := c.Catalog().Update(func(*catalog.Doc) error { return nil }); err != nil {
		return err
	}
	a.logf("catalog %s ready", c.Catalog().Path())
	a.reportCatalog(ctx)
	return nil
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}
