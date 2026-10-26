// Copyright (c) 2026 Accenture, All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package horizon

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// horizonConfigDir returns ~/.config/horizon (or XDG_CONFIG_HOME/horizon).
func HorizonConfigDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		home, e := os.UserHomeDir()
		if e != nil {
			return "", fmt.Errorf("config dir: %w", err)
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "horizon"), nil
}

// HorizonConfigFilePath is the default path for optional CLI defaults.
func HorizonConfigFilePath() (string, error) {
	dir, err := HorizonConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yaml"), nil
}

// fileConfig is the on-disk YAML shape (snake_case). Empty strings are ignored when merging.
type fileConfig struct {
	Domain               string `yaml:"domain"`
	APIBaseURL           string `yaml:"api_base_url"`
	KeycloakBase         string `yaml:"keycloak_base"`
	KeycloakRealm        string `yaml:"keycloak_realm"`
	Module               string `yaml:"module"`
	Template             string `yaml:"template"`
	RunningPollLimit     *int   `yaml:"running_poll_limit"`
	WaitNewAttempts      *int   `yaml:"wait_new_attempts"`
	WaitNewSleepSec      *int   `yaml:"wait_new_sleep_sec"`
	WaitTerminalSecs     *int   `yaml:"wait_terminal_secs"`
	TerminalPollInterval *int   `yaml:"terminal_poll_interval"`
	LogStreamMaxSecs     *int   `yaml:"log_stream_max_secs"`
	LogWaitPodSecs       *int   `yaml:"log_wait_pod_secs"`
	LogStreamFormat      string `yaml:"log_stream_format"`
}

func readFileConfig(path string) (*fileConfig, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var fc fileConfig
	if err := yaml.Unmarshal(b, &fc); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &fc, nil
}

func mergeFileIntoConfig(cfg *Config, fc *fileConfig) {
	if fc == nil {
		return
	}
	if s := strings.TrimSpace(fc.Domain); s != "" {
		cfg.Domain = s
	}
	if s := strings.TrimSpace(fc.APIBaseURL); s != "" {
		cfg.BaseURL = strings.TrimSuffix(s, "/")
	}
	if s := strings.TrimSpace(fc.KeycloakBase); s != "" {
		cfg.KeycloakBase = strings.TrimSuffix(s, "/")
	}
	if s := strings.TrimSpace(fc.KeycloakRealm); s != "" {
		cfg.KeycloakRealm = s
	}
	if s := strings.TrimSpace(fc.Module); s != "" {
		cfg.Module = s
	}
	if s := strings.TrimSpace(fc.Template); s != "" {
		cfg.Template = s
	}
	if fc.RunningPollLimit != nil {
		cfg.RunningPollLimit = *fc.RunningPollLimit
	}
	if fc.WaitNewAttempts != nil {
		cfg.WaitNewAttempts = *fc.WaitNewAttempts
	}
	if fc.WaitNewSleepSec != nil {
		cfg.WaitNewSleepSec = *fc.WaitNewSleepSec
	}
	if fc.WaitTerminalSecs != nil {
		cfg.WaitTerminalSecs = *fc.WaitTerminalSecs
	}
	if fc.TerminalPollInterval != nil {
		cfg.TerminalPollInterval = *fc.TerminalPollInterval
	}
	if fc.LogStreamMaxSecs != nil {
		cfg.LogStreamMaxSecs = *fc.LogStreamMaxSecs
	}
	if fc.LogWaitPodSecs != nil {
		cfg.LogWaitPodSecs = *fc.LogWaitPodSecs
	}
	if s := strings.TrimSpace(fc.LogStreamFormat); s != "" {
		cfg.LogStreamFormat = strings.ToUpper(s)
	}
}

// LoadConfig reads defaults, optional ~/.config/horizon/config.yaml, then environment (env overrides file).
func LoadConfig() (*Config, error) {
	path, _ := HorizonConfigFilePath()
	var fc *fileConfig
	if path != "" {
		var err error
		fc, err = readFileConfig(path)
		if err != nil {
			return nil, err
		}
	}
	return buildConfig(fc)
}

// LoadConfigRelaxed is for auth login: Keycloak URL is required; Horizon API base URL is optional.
func LoadConfigRelaxed() (*Config, error) {
	path, _ := HorizonConfigFilePath()
	var fc *fileConfig
	if path != "" {
		var err error
		fc, err = readFileConfig(path)
		if err != nil {
			return nil, err
		}
	}
	cfg := defaultConfig()
	mergeFileIntoConfig(cfg, fc)
	mergeEnvIntoConfig(cfg)
	return finalizeConfigRelaxed(cfg)
}

func finalizeConfigRelaxed(cfg *Config) (*Config, error) {
	if cfg.BaseURL == "" && cfg.Domain != "" {
		cfg.BaseURL = "https://" + cfg.Domain + "/horizon-api"
	}
	if cfg.KeycloakBase == "" && cfg.Domain != "" {
		cfg.KeycloakBase = "https://" + cfg.Domain + "/auth"
	}
	if cfg.KeycloakBase == "" {
		d := strings.TrimSpace(os.Getenv("HORIZON_DOMAIN"))
		if d != "" {
			cfg.KeycloakBase = "https://" + d + "/auth"
		}
	}
	if cfg.KeycloakBase == "" {
		return nil, fmt.Errorf("Keycloak URL unknown: set KEYCLOAK_BASE or HORIZON_DOMAIN, or domain in ~/.config/horizon/config.yaml, or pass --domain (e.g. %s auth login --device --domain horizon.example.com)", ProgName())
	}
	return cfg, nil
}

func buildConfig(fc *fileConfig) (*Config, error) {
	cfg := defaultConfig()
	mergeFileIntoConfig(cfg, fc)
	mergeEnvIntoConfig(cfg)
	return finalizeConfig(cfg)
}

func defaultConfig() *Config {
	return &Config{
		Module:               "sample",
		Template:             "sample-smoke-test",
		KeycloakRealm:        "horizon",
		RunningPollLimit:     500,
		WaitNewAttempts:      120,
		WaitNewSleepSec:      2,
		WaitTerminalSecs:     3600,
		TerminalPollInterval: 5,
		LogStreamMaxSecs:     7200,
		LogWaitPodSecs:       60,
		LogStreamFormat:      "FORMATTED",
	}
}

func mergeEnvIntoConfig(cfg *Config) {
	if s := strings.TrimSpace(os.Getenv("MODULE")); s != "" {
		cfg.Module = s
	}
	if s := strings.TrimSpace(os.Getenv("TEMPLATE")); s != "" {
		cfg.Template = s
	}
	if s := strings.TrimSpace(os.Getenv("KEYCLOAK_REALM")); s != "" {
		cfg.KeycloakRealm = s
	}
	cfg.RunningPollLimit = envInt("RUNNING_POLL_LIMIT", cfg.RunningPollLimit)
	cfg.WaitNewAttempts = envInt("WAIT_NEW_ATTEMPTS", cfg.WaitNewAttempts)
	cfg.WaitNewSleepSec = envInt("WAIT_NEW_SLEEP", cfg.WaitNewSleepSec)
	cfg.WaitTerminalSecs = envInt("WAIT_TERMINAL_SECS", cfg.WaitTerminalSecs)
	cfg.TerminalPollInterval = envInt("TERMINAL_POLL_INTERVAL", cfg.TerminalPollInterval)
	cfg.LogStreamMaxSecs = envInt("LOG_STREAM_MAX_SECS", cfg.LogStreamMaxSecs)
	cfg.LogWaitPodSecs = envInt("LOG_WAIT_POD_SECS", cfg.LogWaitPodSecs)
	if s := strings.TrimSpace(os.Getenv("HORIZON_LOG_STREAM_FORMAT")); s != "" {
		cfg.LogStreamFormat = strings.ToUpper(s)
	}

	base := strings.TrimSpace(os.Getenv("HORIZON_API_BASE_URL"))
	base = strings.TrimSuffix(base, "/")
	if base != "" {
		cfg.BaseURL = base
		cfg.Domain = ""
	} else if d := strings.TrimSpace(os.Getenv("HORIZON_DOMAIN")); d != "" {
		cfg.Domain = d
		cfg.BaseURL = "https://" + d + "/horizon-api"
	}

	kb := strings.TrimSpace(os.Getenv("KEYCLOAK_BASE"))
	kb = strings.TrimSuffix(kb, "/")
	if kb != "" {
		cfg.KeycloakBase = kb
	}
}

func finalizeConfig(cfg *Config) (*Config, error) {
	if cfg.BaseURL == "" && cfg.Domain != "" {
		cfg.BaseURL = "https://" + cfg.Domain + "/horizon-api"
	}
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("set HORIZON_API_BASE_URL, HORIZON_DOMAIN, or api_base_url in ~/.config/horizon/config.yaml (or use --api / --domain)")
	}
	if cfg.KeycloakBase == "" && cfg.Domain != "" {
		cfg.KeycloakBase = "https://" + cfg.Domain + "/auth"
	}
	if _, err := url.Parse(cfg.BaseURL); err != nil {
		return nil, fmt.Errorf("Horizon API base URL: %w", err)
	}
	return cfg, nil
}

// FlagOverrides are applied after file+env (CLI wins).
type FlagOverrides struct {
	APIBaseURL string
	Domain     string
	Module     string
	Template   string
}

// RevalidateConfig re-derives API/Keycloak URLs after CLI flag overrides.
func RevalidateConfig(cfg *Config) error {
	_, err := finalizeConfig(cfg)
	return err
}

func ApplyFlagOverrides(cfg *Config, fo *FlagOverrides) {
	if fo == nil {
		return
	}
	if s := strings.TrimSpace(fo.APIBaseURL); s != "" {
		cfg.BaseURL = strings.TrimSuffix(s, "/")
		cfg.Domain = ""
	} else if d := strings.TrimSpace(fo.Domain); d != "" {
		cfg.Domain = d
		cfg.BaseURL = "https://" + d + "/horizon-api"
		cfg.KeycloakBase = "https://" + d + "/auth"
	}
	if s := strings.TrimSpace(fo.Module); s != "" {
		cfg.Module = s
	}
	if s := strings.TrimSpace(fo.Template); s != "" {
		cfg.Template = s
	}
}

// MergeLoginIntoConfigFile writes domain/keycloak fields into config.yaml (for auth login --write-config).
func MergeLoginIntoConfigFile(domain, apiBase, keycloakBase, realm string) error {
	dir, err := HorizonConfigDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, "config.yaml")

	var fc fileConfig
	if b, err := os.ReadFile(path); err == nil {
		_ = yaml.Unmarshal(b, &fc)
	}
	if d := strings.TrimSpace(domain); d != "" {
		fc.Domain = d
	}
	if s := strings.TrimSpace(apiBase); s != "" {
		fc.APIBaseURL = strings.TrimSuffix(s, "/")
	}
	if s := strings.TrimSpace(keycloakBase); s != "" {
		fc.KeycloakBase = strings.TrimSuffix(s, "/")
	}
	if s := strings.TrimSpace(realm); s != "" {
		fc.KeycloakRealm = s
	}
	out, err := yaml.Marshal(&fc)
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0o644)
}
