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

// Package mirror implements the create, sync, delete and init-volume operations on the mirror volume.
package mirror

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"storage-mirror-sync/internal/catalog"
)

// Config is one run, from the SM_* environment the WorkflowTemplates set.
type Config struct {
	RunID       string
	RequestID   string
	Mirror      string
	Directory   string
	Kind        string
	SubmittedBy string

	ManifestURL  string
	ManifestRef  string
	ManifestFile string
	SyncJobs     int

	GitURL      string
	GitBranches []string
	// GitCredentialsDir holds username and password files when a credentials Secret is mounted.
	GitCredentialsDir string

	LFS         string
	SyncTimeout time.Duration
	PruneRefs   []string
	ExcludeRefs []string
	GC          string
	Confirm     string

	MountPath  string
	RootSubdir string

	ControllerURL    string
	ControllerCAFile string
	ControllerToken  string
	ArgoProgressFile string

	// Defaults applied to empty parameters (Sensors map missing fields to "").
	DefaultManifestURL     string
	DefaultManifestRef     string
	DefaultManifestFile    string
	DefaultSyncJobs        int
	DefaultRepoSyncTimeout string
	DefaultGitBranches     []string
	DefaultGitSyncTimeout  string
	DefaultLFS             string

	// Retry policy (parity with utils.sh sync_mirror_with_retries).
	MaxAttempts int
	RetryDelay  time.Duration

	// allowLocalURLs lets tests use file:// upstreams; never set from the environment.
	allowLocalURLs bool
}

// FromEnv reads the run configuration.
func FromEnv(getenv func(string) string) Config {
	c := Config{
		RunID:                  getenv("SM_RUN_ID"),
		RequestID:              getenv("SM_REQUEST_ID"),
		Mirror:                 strings.TrimSpace(getenv("SM_MIRROR")),
		Directory:              strings.TrimSpace(getenv("SM_DIRECTORY")),
		Kind:                   strings.ToLower(strings.TrimSpace(getenv("SM_KIND"))),
		SubmittedBy:            getenv("SM_SUBMITTED_BY"),
		ManifestURL:            strings.TrimSpace(getenv("SM_MANIFEST_URL")),
		ManifestRef:            strings.TrimSpace(getenv("SM_MANIFEST_REF")),
		ManifestFile:           strings.TrimSpace(getenv("SM_MANIFEST_FILE")),
		SyncJobs:               atoi(getenv("SM_SYNC_JOBS")),
		GitURL:                 strings.TrimSpace(getenv("SM_GIT_URL")),
		GitBranches:            csv(getenv("SM_GIT_BRANCHES")),
		LFS:                    strings.ToLower(strings.TrimSpace(getenv("SM_LFS"))),
		PruneRefs:              csv(getenv("SM_PRUNE_REFS")),
		ExcludeRefs:            csv(getenv("SM_EXCLUDE_REFS")),
		GC:                     strings.ToLower(strings.TrimSpace(getenv("SM_GC"))),
		Confirm:                strings.TrimSpace(getenv("SM_CONFIRM")),
		MountPath:              orDefault(getenv("SM_MOUNT_PATH"), "/mnt/mirror-filestore-pvc"),
		RootSubdir:             orDefault(getenv("SM_ROOT_SUBDIR"), "horizon-mirrors"),
		ControllerURL:          strings.TrimRight(getenv("SM_CONTROLLER_URL"), "/"),
		ControllerCAFile:       getenv("SM_CONTROLLER_CA_FILE"),
		ControllerToken:        strings.TrimSpace(getenv("SM_CONTROLLER_TOKEN")),
		ArgoProgressFile:       getenv("ARGO_PROGRESS_FILE"),
		DefaultManifestURL:     getenv("SM_DEFAULT_MANIFEST_URL"),
		DefaultManifestRef:     getenv("SM_DEFAULT_MANIFEST_REF"),
		DefaultManifestFile:    orDefault(getenv("SM_DEFAULT_MANIFEST_FILE"), "default.xml"),
		DefaultSyncJobs:        atoi(getenv("SM_DEFAULT_SYNC_JOBS")),
		DefaultRepoSyncTimeout: orDefault(getenv("SM_DEFAULT_REPO_SYNC_TIMEOUT"), "20h"),
		DefaultGitBranches:     csv(getenv("SM_DEFAULT_GIT_BRANCHES")),
		DefaultGitSyncTimeout:  orDefault(getenv("SM_DEFAULT_GIT_SYNC_TIMEOUT"), "6h"),
		DefaultLFS:             orDefault(strings.ToLower(getenv("SM_DEFAULT_LFS")), "disabled"),
		MaxAttempts:            3,
		RetryDelay:             60 * time.Second,
	}
	if d := orDefault(getenv("SM_GIT_CREDENTIALS_DIR"), "/etc/storage-mirror/git-credentials"); dirExists(d) {
		c.GitCredentialsDir = d
	}
	c.SyncTimeout, _ = ParseTimeout(getenv("SM_SYNC_TIMEOUT"))
	if c.Directory == "" {
		c.Directory = c.Mirror
	}
	return c
}

// Root is the mirror root subdirectory.
func (c Config) Root() string { return filepath.Join(c.MountPath, c.RootSubdir) }

// Path is the mirror directory.
func (c Config) Path() string { return filepath.Join(c.Root(), c.Directory) }

// Catalog is the metadata.yaml of the root subdirectory.
func (c Config) Catalog() catalog.Catalog { return catalog.Catalog{Root: c.Root()} }

var dirRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,99}$`)

// ValidateDirectory rejects paths that would escape the root subdirectory or hit reserved names.
func ValidateDirectory(d string) error {
	if !dirRe.MatchString(d) || d == catalog.FileName || strings.Contains(d, "..") {
		return fmt.Errorf("invalid mirror directory %q", d)
	}
	return nil
}

// ApplyDefaults fills empty parameters, first from the catalog entry (sync of an existing mirror, like the Jenkins
// SYNC_ALL_EXISTING_MIRRORS path), then from the module defaults.
func (c *Config) ApplyDefaults(entry map[string]string) error {
	if entry != nil {
		if c.Kind == "" {
			c.Kind = entry["type"]
		}
		if c.Kind == "repo" {
			c.ManifestURL = orDefault(c.ManifestURL, entry["manifest_url"])
			c.ManifestRef = orDefault(c.ManifestRef, entry["manifest_ref"])
			c.ManifestFile = orDefault(c.ManifestFile, entry["manifest_file"])
			if c.SyncJobs == 0 {
				c.SyncJobs = atoi(entry["repo_sync_jobs"])
			}
		}
		if c.Kind == "git" {
			c.GitURL = orDefault(c.GitURL, entry["repository_url"])
			if len(c.GitBranches) == 0 {
				c.GitBranches = csv(entry["branches"])
			}
		}
		if c.LFS == "" {
			c.LFS = entry["lfs"]
		}
	}
	if c.Kind == "" {
		c.Kind = "repo"
	}
	if c.LFS == "" {
		c.LFS = c.DefaultLFS
	}
	switch c.Kind {
	case "repo":
		c.ManifestURL = orDefault(c.ManifestURL, c.DefaultManifestURL)
		c.ManifestRef = orDefault(c.ManifestRef, c.DefaultManifestRef)
		c.ManifestFile = orDefault(c.ManifestFile, c.DefaultManifestFile)
		if c.SyncJobs == 0 {
			c.SyncJobs = c.DefaultSyncJobs
		}
		if c.SyncJobs == 0 {
			c.SyncJobs = 3
		}
		if c.SyncTimeout == 0 {
			c.SyncTimeout, _ = ParseTimeout(c.DefaultRepoSyncTimeout)
		}
		if c.ManifestURL == "" || c.ManifestRef == "" {
			return fmt.Errorf("repo mirror needs manifestUrl and manifestRef")
		}
	case "git":
		if len(c.GitBranches) == 0 {
			c.GitBranches = c.DefaultGitBranches
		}
		if len(c.GitBranches) == 0 {
			c.GitBranches = []string{"*"}
		}
		if c.SyncTimeout == 0 {
			c.SyncTimeout, _ = ParseTimeout(c.DefaultGitSyncTimeout)
		}
		if !strings.HasPrefix(c.GitURL, "https://") && !(c.allowLocalURLs && strings.HasPrefix(c.GitURL, "file://")) {
			return fmt.Errorf("git mirror needs an https:// gitUrl")
		}
	default:
		return fmt.Errorf("kind must be repo or git, got %q", c.Kind)
	}
	switch c.LFS {
	case "disabled", "enabled", "auto":
	default:
		return fmt.Errorf("lfs must be disabled, enabled or auto, got %q", c.LFS)
	}
	if c.SyncTimeout == 0 {
		c.SyncTimeout = 20 * time.Hour
	}
	return nil
}

// ParseTimeout accepts GNU timeout(1) durations (20h, 90m, 3600, 1d) and Go durations.
func ParseTimeout(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	if d, err := time.ParseDuration(s); err == nil {
		return d, nil
	}
	unit := time.Second
	switch s[len(s)-1] {
	case 's':
		s = s[:len(s)-1]
	case 'm':
		unit, s = time.Minute, s[:len(s)-1]
	case 'h':
		unit, s = time.Hour, s[:len(s)-1]
	case 'd':
		unit, s = 24*time.Hour, s[:len(s)-1]
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || f <= 0 {
		return 0, fmt.Errorf("invalid timeout %q", s)
	}
	return time.Duration(f * float64(unit)), nil
}

func csv(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func atoi(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}

func orDefault(v, d string) string {
	if strings.TrimSpace(v) == "" {
		return d
	}
	return v
}

func dirExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.IsDir()
}
