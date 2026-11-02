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

package horizon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestAddAPIFlagsAndOverrides(t *testing.T) {
	oldAPI, oldDomain := FlagAPI, FlagDomain
	t.Cleanup(func() {
		FlagAPI, FlagDomain = oldAPI, oldDomain
	})
	cmd := &cobra.Command{Use: "root"}
	AddAPIFlags(cmd)
	if cmd.PersistentFlags().Lookup("api") == nil || cmd.PersistentFlags().Lookup("domain") == nil {
		t.Fatal("missing persistent flags")
	}
	FlagAPI = " https://cli.example/api/ "
	FlagDomain = "ignored.example"
	cfg := defaultConfig()
	ApplyFlagOverrides(cfg, GlobalFlagOverrides(" mod ", " tmpl "))
	if cfg.BaseURL != "https://cli.example/api" || cfg.Domain != "" || cfg.Module != "mod" || cfg.Template != "tmpl" {
		t.Fatalf("cfg = %+v", cfg)
	}
}

func TestProgName(t *testing.T) {
	if ProgName() == "" {
		t.Fatal("empty prog name")
	}
	old := os.Args
	t.Cleanup(func() { os.Args = old })
	os.Args = []string{filepath.Join("bin", "horizon.exe")}
	if ProgName() != "horizon.exe" {
		t.Fatalf("got %q", ProgName())
	}
	os.Args = nil
	if ProgName() != "horizon" {
		t.Fatalf("fallback = %q", ProgName())
	}
}

func TestLoadAPIClientRequiresConfig(t *testing.T) {
	isolateConfigEnv(t)
	oldAPI, oldDomain := FlagAPI, FlagDomain
	t.Cleanup(func() {
		FlagAPI, FlagDomain = oldAPI, oldDomain
	})
	FlagAPI, FlagDomain = "", ""
	if _, _, err := LoadAPIClient(); err == nil || !strings.Contains(err.Error(), "HORIZON_API_BASE_URL") {
		t.Fatalf("err = %v", err)
	}
}

func TestLoadExampleAPIBaseOfflineAndUnauthenticated(t *testing.T) {
	dir := isolateConfigEnv(t)
	oldAPI, oldDomain := FlagAPI, FlagDomain
	t.Cleanup(func() {
		FlagAPI, FlagDomain = oldAPI, oldDomain
	})
	FlagAPI, FlagDomain = "", ""

	base, err := LoadExampleAPIBase()
	if err != nil {
		t.Fatal(err)
	}
	if base != "{api}" {
		t.Fatalf("unset base = %q", base)
	}

	// A client secret would force LoadAPIClient into a live token call.
	t.Setenv("KEYCLOAK_CLIENT_SECRET", "secret")
	t.Setenv("HORIZON_API_BASE_URL", "https://env.example/horizon-api/")
	base, err = LoadExampleAPIBase()
	if err != nil {
		t.Fatal(err)
	}
	if base != "https://env.example/horizon-api" {
		t.Fatalf("env base = %q", base)
	}

	t.Setenv("HORIZON_API_BASE_URL", "")
	FlagAPI = "https://cli.example/horizon-api/"
	base, err = LoadExampleAPIBase()
	if err != nil {
		t.Fatal(err)
	}
	if base != "https://cli.example/horizon-api" {
		t.Fatalf("flag base = %q", base)
	}

	FlagAPI = ""
	FlagDomain = "cli.example"
	base, err = LoadExampleAPIBase()
	if err != nil {
		t.Fatal(err)
	}
	if base != "https://cli.example/horizon-api" {
		t.Fatalf("domain flag base = %q", base)
	}

	FlagDomain = ""
	cfgDir := filepath.Join(dir, "horizon")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte("domain: file.example\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	base, err = LoadExampleAPIBase()
	if err != nil {
		t.Fatal(err)
	}
	if base != "https://file.example/horizon-api" {
		t.Fatalf("file domain base = %q", base)
	}
}

func TestLoadAPIClientAppliesAPIFlagBeforeURLCheck(t *testing.T) {
	isolateConfigEnv(t)
	oldAPI, oldDomain := FlagAPI, FlagDomain
	t.Cleanup(func() {
		FlagAPI, FlagDomain = oldAPI, oldDomain
	})
	FlagAPI = "https://cli.example/horizon-api/"
	FlagDomain = ""
	_, _, err := LoadAPIClient()
	if err == nil || !strings.Contains(err.Error(), "not authenticated") || strings.Contains(err.Error(), "HORIZON_API_BASE_URL") {
		t.Fatalf("err = %v", err)
	}

	t.Setenv("HORIZON_ACCESS_TOKEN", "test-token")
	c, cfg, err := LoadAPIClient()
	if err != nil {
		t.Fatalf("LoadAPIClient: %v", err)
	}
	if cfg.BaseURL != "https://cli.example/horizon-api" {
		t.Fatalf("BaseURL = %q", cfg.BaseURL)
	}
	if c == nil || c.bearer != "test-token" {
		t.Fatalf("client = %#v", c)
	}
}
