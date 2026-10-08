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

	"gopkg.in/yaml.v3"
)

func isolateConfigEnv(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	for _, k := range []string{
		"MODULE", "TEMPLATE", "KEYCLOAK_REALM",
		"RUNNING_POLL_LIMIT", "WAIT_NEW_ATTEMPTS", "WAIT_NEW_SLEEP",
		"WAIT_TERMINAL_SECS", "TERMINAL_POLL_INTERVAL",
		"LOG_STREAM_MAX_SECS", "LOG_WAIT_POD_SECS", "HORIZON_LOG_STREAM_FORMAT",
		"HORIZON_API_BASE_URL", "HORIZON_DOMAIN", "KEYCLOAK_BASE",
		"KEYCLOAK_CLIENT_SECRET", "KEYCLOAK_CLIENT_ID", "HORIZON_ACCESS_TOKEN",
	} {
		t.Setenv(k, "")
	}
	return dir
}

func TestEnvOrAndEnvInt(t *testing.T) {
	t.Setenv("HORIZON_TEST_STR", "  value  ")
	if got := EnvOr("HORIZON_TEST_STR", "def"); got != "value" {
		t.Fatalf("EnvOr = %q", got)
	}
	t.Setenv("HORIZON_TEST_STR", "   ")
	if got := EnvOr("HORIZON_TEST_STR", "def"); got != "def" {
		t.Fatalf("blank EnvOr = %q", got)
	}

	t.Setenv("HORIZON_TEST_INT", "")
	if got := envInt("HORIZON_TEST_INT", 7); got != 7 {
		t.Fatalf("empty envInt = %d", got)
	}
	t.Setenv("HORIZON_TEST_INT", "nope")
	if got := envInt("HORIZON_TEST_INT", 7); got != 7 {
		t.Fatalf("invalid envInt = %d", got)
	}
	t.Setenv("HORIZON_TEST_INT", " 0 ")
	if got := envInt("HORIZON_TEST_INT", 7); got != 0 {
		t.Fatalf("zero envInt = %d", got)
	}
}

func TestModeFlags(t *testing.T) {
	prevDebug, prevJSON, prevBanner, prevInsecure := IsDebug(), IsJSON(), IsNoBanner(), IsInsecure()
	t.Cleanup(func() {
		*TargetDebug() = prevDebug
		*TargetJSON() = prevJSON
		*TargetNoBanner() = prevBanner
		*TargetInsecure() = prevInsecure
	})
	*TargetDebug() = true
	*TargetJSON() = true
	*TargetNoBanner() = true
	*TargetInsecure() = true
	if !IsDebug() || !IsJSON() || !IsNoBanner() || !IsInsecure() {
		t.Fatal("mode flags did not stick")
	}
}

func TestDefaultAndMergeFile(t *testing.T) {
	cfg := defaultConfig()
	if cfg.Module != "sample" || cfg.Template != "sample-smoke-test" || cfg.KeycloakRealm != "horizon" {
		t.Fatalf("defaults = %+v", cfg)
	}
	if cfg.RunningPollLimit != 500 || cfg.LogStreamFormat != "FORMATTED" {
		t.Fatalf("numeric defaults = %+v", cfg)
	}

	limit := 0
	sleep := 9
	mergeFileIntoConfig(cfg, &fileConfig{
		Domain:           "  horizon.example  ",
		APIBaseURL:       " https://api.example/horizon-api/ ",
		KeycloakBase:     " https://auth.example/ ",
		KeycloakRealm:    "   ",
		Module:           "mod",
		Template:         "",
		RunningPollLimit: &limit,
		WaitNewSleepSec:  &sleep,
		LogStreamFormat:  " raw ",
	})
	if cfg.Domain != "horizon.example" {
		t.Fatalf("domain = %q", cfg.Domain)
	}
	if cfg.BaseURL != "https://api.example/horizon-api" {
		t.Fatalf("base = %q", cfg.BaseURL)
	}
	if cfg.KeycloakBase != "https://auth.example" || cfg.KeycloakRealm != "horizon" {
		t.Fatalf("keycloak = %q %q", cfg.KeycloakBase, cfg.KeycloakRealm)
	}
	if cfg.Module != "mod" || cfg.Template != "sample-smoke-test" {
		t.Fatalf("module/template = %q %q", cfg.Module, cfg.Template)
	}
	if cfg.RunningPollLimit != 0 || cfg.WaitNewSleepSec != 9 || cfg.LogStreamFormat != "RAW" {
		t.Fatalf("merged numbers = %+v", cfg)
	}

	mergeFileIntoConfig(cfg, nil)
	if cfg.Module != "mod" {
		t.Fatal("nil file config should be a no-op")
	}
}

func TestMergeEnvAndFinalize(t *testing.T) {
	isolateConfigEnv(t)
	cfg := defaultConfig()
	cfg.Domain = "from-file.example"
	cfg.BaseURL = "https://from-file.example/horizon-api"
	cfg.KeycloakBase = "https://from-file.example/auth"

	t.Setenv("MODULE", " env-mod ")
	t.Setenv("TEMPLATE", "env-tmpl")
	t.Setenv("KEYCLOAK_REALM", "env-realm")
	t.Setenv("RUNNING_POLL_LIMIT", "bogus")
	t.Setenv("WAIT_NEW_ATTEMPTS", "3")
	t.Setenv("HORIZON_LOG_STREAM_FORMAT", " raw ")
	t.Setenv("HORIZON_API_BASE_URL", "https://override.example/api/")
	t.Setenv("HORIZON_DOMAIN", "ignored.example")
	t.Setenv("KEYCLOAK_BASE", "https://kc.example/")

	mergeEnvIntoConfig(cfg)
	if cfg.Module != "env-mod" || cfg.Template != "env-tmpl" || cfg.KeycloakRealm != "env-realm" {
		t.Fatalf("env strings = %+v", cfg)
	}
	if cfg.RunningPollLimit != 500 {
		t.Fatalf("invalid int should keep default, got %d", cfg.RunningPollLimit)
	}
	if cfg.WaitNewAttempts != 3 || cfg.LogStreamFormat != "RAW" {
		t.Fatalf("env overrides = %+v", cfg)
	}
	if cfg.BaseURL != "https://override.example/api" || cfg.Domain != "" {
		t.Fatalf("api url should replace domain, got %q %q", cfg.BaseURL, cfg.Domain)
	}
	if cfg.KeycloakBase != "https://kc.example" {
		t.Fatalf("keycloak = %q", cfg.KeycloakBase)
	}

	out, err := finalizeConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if out.BaseURL != cfg.BaseURL {
		t.Fatalf("finalized base = %q", out.BaseURL)
	}

	if _, err := finalizeConfig(&Config{}); err == nil {
		t.Fatal("expected missing API URL error")
	}
	if _, err := finalizeConfig(&Config{BaseURL: "http://bad host"}); err == nil {
		t.Fatal("expected invalid URL error")
	}
}

func TestFinalizeDerivesURLsFromDomain(t *testing.T) {
	cfg := defaultConfig()
	cfg.Domain = "horizon.example"
	out, err := finalizeConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if out.BaseURL != "https://horizon.example/horizon-api" {
		t.Fatalf("base = %q", out.BaseURL)
	}
	if out.KeycloakBase != "https://horizon.example/auth" {
		t.Fatalf("keycloak = %q", out.KeycloakBase)
	}
}

func TestFinalizeConfigRelaxed(t *testing.T) {
	isolateConfigEnv(t)
	cfg := defaultConfig()
	cfg.Domain = "horizon.example"
	out, err := finalizeConfigRelaxed(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if out.KeycloakBase != "https://horizon.example/auth" || out.BaseURL != "https://horizon.example/horizon-api" {
		t.Fatalf("relaxed = %+v", out)
	}

	t.Setenv("HORIZON_DOMAIN", "env.example")
	onlyAPI := defaultConfig()
	onlyAPI.BaseURL = "https://api.example"
	out, err = finalizeConfigRelaxed(onlyAPI)
	if err != nil {
		t.Fatal(err)
	}
	if out.KeycloakBase != "https://env.example/auth" {
		t.Fatalf("keycloak from env = %q", out.KeycloakBase)
	}

	t.Setenv("HORIZON_DOMAIN", "")
	if _, err := finalizeConfigRelaxed(defaultConfig()); err == nil {
		t.Fatal("expected missing Keycloak URL error")
	}
}

func TestApplyFlagOverrides(t *testing.T) {
	cfg := defaultConfig()
	ApplyFlagOverrides(cfg, nil)
	if cfg.Module != "sample" {
		t.Fatal("nil overrides should be a no-op")
	}

	ApplyFlagOverrides(cfg, &FlagOverrides{
		APIBaseURL: " https://cli.example/api/ ",
		Domain:     "should-not-win.example",
		Module:     " cli-mod ",
		Template:   "cli-tmpl",
	})
	if cfg.BaseURL != "https://cli.example/api" || cfg.Domain != "" {
		t.Fatalf("api flag = %q %q", cfg.BaseURL, cfg.Domain)
	}
	if cfg.Module != "cli-mod" || cfg.Template != "cli-tmpl" {
		t.Fatalf("module flags = %q %q", cfg.Module, cfg.Template)
	}
	if err := RevalidateConfig(cfg); err != nil {
		t.Fatal(err)
	}

	cfg = defaultConfig()
	ApplyFlagOverrides(cfg, &FlagOverrides{Domain: "cli.example"})
	if cfg.Domain != "cli.example" || cfg.BaseURL != "https://cli.example/horizon-api" || cfg.KeycloakBase != "https://cli.example/auth" {
		t.Fatalf("domain flag = %+v", cfg)
	}
}

func TestLoadConfigFileAndEnv(t *testing.T) {
	dir := isolateConfigEnv(t)
	cfgDir := filepath.Join(dir, "horizon")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := []byte("domain: file.example\nmodule: file-mod\nrunning_poll_limit: 12\nlog_stream_format: raw\n")
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MODULE", "env-mod")
	t.Setenv("RUNNING_POLL_LIMIT", "4")

	cfg, err := LoadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Domain != "file.example" || cfg.BaseURL != "https://file.example/horizon-api" {
		t.Fatalf("file domain = %+v", cfg)
	}
	if cfg.Module != "env-mod" || cfg.RunningPollLimit != 4 || cfg.LogStreamFormat != "RAW" {
		t.Fatalf("env wins = %+v", cfg)
	}
	if cfg.KeycloakBase != "https://file.example/auth" || cfg.KeycloakRealm != "horizon" {
		t.Fatalf("derived keycloak = %+v", cfg)
	}

	if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(":\n  -"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfig(); err == nil {
		t.Fatal("expected YAML parse error")
	}
}

func TestLoadConfigRelaxedFromDomain(t *testing.T) {
	dir := isolateConfigEnv(t)
	cfgDir := filepath.Join(dir, "horizon")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte("domain: login.example\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfigRelaxed()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.KeycloakBase != "https://login.example/auth" {
		t.Fatalf("keycloak = %q", cfg.KeycloakBase)
	}
}

func TestReadFileConfigMissing(t *testing.T) {
	fc, err := readFileConfig(filepath.Join(t.TempDir(), "missing.yaml"))
	if err != nil || fc != nil {
		t.Fatalf("missing file: fc=%v err=%v", fc, err)
	}
	path := filepath.Join(t.TempDir(), "bad.yaml")
	if err := os.WriteFile(path, []byte("domain: ["), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readFileConfig(path); err == nil || !strings.Contains(err.Error(), "parse ") {
		t.Fatalf("err = %v", err)
	}
}

func TestMergeLoginIntoConfigFile(t *testing.T) {
	dir := isolateConfigEnv(t)
	if err := MergeLoginIntoConfigFile(" login.example ", "https://api.example/", "https://auth.example/", " custom "); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "horizon", "config.yaml")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var fc fileConfig
	if err := yaml.Unmarshal(b, &fc); err != nil {
		t.Fatal(err)
	}
	if fc.Domain != "login.example" || fc.APIBaseURL != "https://api.example" || fc.KeycloakBase != "https://auth.example" || fc.KeycloakRealm != "custom" {
		t.Fatalf("written = %+v", fc)
	}

	if err := MergeLoginIntoConfigFile("", "", "", "other"); err != nil {
		t.Fatal(err)
	}
	b, err = os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	fc = fileConfig{}
	if err := yaml.Unmarshal(b, &fc); err != nil {
		t.Fatal(err)
	}
	if fc.Domain != "login.example" || fc.KeycloakRealm != "other" {
		t.Fatalf("merge should keep domain, got %+v", fc)
	}
}

func TestHorizonConfigPaths(t *testing.T) {
	dir := isolateConfigEnv(t)
	got, err := HorizonConfigDir()
	if err != nil {
		t.Fatal(err)
	}
	if got != filepath.Join(dir, "horizon") {
		t.Fatalf("config dir = %q", got)
	}
	file, err := HorizonConfigFilePath()
	if err != nil {
		t.Fatal(err)
	}
	if file != filepath.Join(dir, "horizon", "config.yaml") {
		t.Fatalf("config file = %q", file)
	}
}
