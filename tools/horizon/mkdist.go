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

//go:build ignore

// Cross-compile Horizon CLI binaries and write manifest.json.
//
//	go run mkdist.go <outDir> [version]
//
// Run from tools/horizon (directory that contains go.mod). Version defaults to "dev".
// Optional env: HORIZON_CLI_COMMIT (git SHA; default unknown).
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

type artifact struct {
	OS           string `json:"os"`
	Arch         string `json:"arch"`
	Filename     string `json:"filename"`
	SHA256       string `json:"sha256"`
	SizeBytes    int64  `json:"sizeBytes"`
	DownloadPath string `json:"downloadPath"`
}

type manifest struct {
	Name      string     `json:"name"`
	Version   string     `json:"version"`
	BuildDate string     `json:"buildDate"`
	Artifacts []artifact `json:"artifacts"`
}

var targets = []struct {
	goos, goarch, filename string
}{
	{"linux", "amd64", "horizon"},
	{"linux", "arm64", "horizon"},
	{"darwin", "arm64", "horizon"},
	{"windows", "amd64", "horizon.exe"},
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: go run mkdist.go <outDir> [version]\n")
		os.Exit(2)
	}
	outDir := os.Args[1]
	version := "dev"
	if len(os.Args) >= 3 && os.Args[2] != "" {
		version = os.Args[2]
	}
	buildDate := time.Now().UTC().Format(time.RFC3339)
	root, err := os.Getwd()
	if err != nil {
		fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		fatal(fmt.Errorf("go.mod not found in %s; run from tools/horizon", root))
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fatal(err)
	}

	var arts []artifact
	for _, t := range targets {
		destDir := filepath.Join(outDir, t.goos, t.goarch)
		if err := os.MkdirAll(destDir, 0o755); err != nil {
			fatal(err)
		}
		dest := filepath.Join(destDir, t.filename)
		ld := fmt.Sprintf("-s -w -X main.version=%s -X main.buildDate=%s", version, buildDate)
		cmd := exec.Command("go", "build", "-trimpath", "-ldflags", ld, "-o", dest, ".")
		cmd.Dir = root
		cmd.Env = append(os.Environ(),
			"CGO_ENABLED=0",
			"GOOS="+t.goos,
			"GOARCH="+t.goarch,
			"GOFLAGS=",
		)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		fmt.Fprintf(os.Stderr, "mkdist: %s/%s -> %s (go %s)\n", t.goos, t.goarch, dest, runtime.Version())
		if err := cmd.Run(); err != nil {
			fatal(fmt.Errorf("build %s/%s: %w", t.goos, t.goarch, err))
		}
		sum, size, err := sha256File(dest)
		if err != nil {
			fatal(err)
		}
		arts = append(arts, artifact{
			OS:           t.goos,
			Arch:         t.goarch,
			Filename:     t.filename,
			SHA256:       sum,
			SizeBytes:    size,
			DownloadPath: "/api/cli/v1/download/" + t.goos + "/" + t.goarch,
		})
	}

	man := manifest{Name: "horizon", Version: version, BuildDate: buildDate, Artifacts: arts}
	raw, err := json.MarshalIndent(man, "", "  ")
	if err != nil {
		fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "manifest.json"), append(raw, '\n'), 0o644); err != nil {
		fatal(err)
	}
}

func sha256File(path string) (string, int64, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(h, f)
	if err != nil {
		return "", 0, err
	}
	return hex.EncodeToString(h.Sum(nil)), n, nil
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "mkdist: %v\n", err)
	os.Exit(1)
}
