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

package auth

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	horizon "github.com/horizon-sdv/horizon/internal"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var (
	authLoginDevice       bool
	authLoginClientCreds  bool
	authLoginOpenBrowser  bool
	authLoginNoBrowser    bool
	authLoginScope        string
	authLoginClientID     string
	authLoginClientSecret string
	authLoginWriteConfig  bool
)

func NewAuthLoginCmd() *cobra.Command {
	var authLoginCmd = &cobra.Command{
		Use:   "login",
		Short: "Authenticate with Keycloak (password grant)",
		Long:  `Authenticate with Keycloak using the OAuth 2.0 device flow (public client) or the client_credentials grant (CI / confidential client). The access token is saved to ~/.config/horizon/token.json.`,
		Args:  cobra.NoArgs,
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			cfg, err := horizon.LoadConfigRelaxed()
			if err != nil {
				return err
			}
			horizon.ApplyFlagOverrides(cfg, horizon.GlobalFlagOverrides("", ""))
			if cfg.KeycloakBase == "" && cfg.Domain != "" {
				cfg.KeycloakBase = "https://" + cfg.Domain + "/auth"
			}
			if cfg.KeycloakBase == "" {
				return fmt.Errorf("Keycloak URL unknown: set KEYCLOAK_BASE or HORIZON_DOMAIN, or put domain in ~/.config/horizon/config.yaml, or pass --domain (e.g. %s auth login --device --domain horizon.example.com)", horizon.ProgName())
			}
			kb := cfg.KeycloakBase
			realm := cfg.KeycloakRealm

			in := bufio.NewReader(os.Stdin)
			useTTY := term.IsTerminal(int(os.Stdin.Fd()))
			autoOpen := useTTY && !authLoginNoBrowser && !authLoginOpenBrowser
			if authLoginOpenBrowser {
				autoOpen = true
			}

			defaultClientCI := "horizon-api-ci"
			defaultClientPub := "horizon-api"
			mode := "auto"
			if authLoginDevice {
				mode = "device"
			}
			if authLoginClientCreds {
				mode = "client_credentials"
			}

			secret := strings.TrimSpace(os.Getenv("KEYCLOAK_CLIENT_SECRET"))
			clientID := strings.TrimSpace(os.Getenv("KEYCLOAK_CLIENT_ID"))
			if s := strings.TrimSpace(authLoginClientSecret); s != "" {
				secret = s
			}
			if s := strings.TrimSpace(authLoginClientID); s != "" {
				clientID = s
			}

			if mode == "auto" {
				if secret != "" {
					mode = "client_credentials"
				} else {
					mode = "device"
				}
			}

			if clientID == "" {
				def := defaultClientPub
				if mode == "client_credentials" {
					def = defaultClientCI
				}
				// Device flow: use KEYCLOAK_CLIENT_ID or default (horizon-api); never prompt.
				if useTTY && mode != "device" {
					clientID = promptLine(in, "Keycloak client ID", def)
				} else {
					clientID = def
				}
			}
			if clientID == "" {
				return fmt.Errorf("client ID required")
			}

			oauth := &horizon.OAuthClient{HTTP: horizon.DefaultHTTPClientWithInsecure(horizon.IsInsecure())}
			ctx := context.Background()

			var tc *horizon.TokenCache
			switch mode {
			case "client_credentials":
				if secret == "" {
					if !useTTY {
						return fmt.Errorf("KEYCLOAK_CLIENT_SECRET required non-interactively")
					}
					secret, err = promptSecret("Keycloak client secret")
					if err != nil {
						return err
					}
				}
				tc, err = oauth.ClientCredentials(ctx, kb, realm, clientID, secret)
				if err != nil {
					return err
				}
			case "device":
				var opener func(string) error
				if autoOpen {
					op := browserOpener()
					opener = func(u string) error { return op(u) }
				}
				tc, err = oauth.DeviceFlow(ctx, kb, realm, clientID, authLoginScope, opener)
				if err != nil {
					return err
				}
			default:
				return fmt.Errorf("unknown auth mode %q", mode)
			}

			tc.KeycloakBase = strings.TrimSuffix(kb, "/")
			tc.Realm = realm
			tc.ClientID = clientID
			if err := horizon.SaveTokenCache(tc); err != nil {
				return err
			}
			p, err := horizon.TokenCachePath()
			if err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "Logged in. Token saved to %s\n", p)

			if authLoginWriteConfig {
				apiBase := cfg.BaseURL
				if err := horizon.MergeLoginIntoConfigFile(cfg.Domain, apiBase, kb, realm); err != nil {
					return fmt.Errorf("write config: %w", err)
				}
				p, _ := horizon.HorizonConfigFilePath()
				fmt.Fprintf(os.Stderr, "Wrote defaults to %s\n", p)
			}
			return nil
		},
	}

	authLoginCmd.Flags().BoolVar(&authLoginDevice, "device", false, "use OAuth 2.0 device flow (public client)")
	authLoginCmd.Flags().BoolVar(&authLoginClientCreds, "client-credentials", false, "use client_credentials grant")
	authLoginCmd.Flags().BoolVar(&authLoginOpenBrowser, "open-browser", false, "open verification URL in a browser")
	authLoginCmd.Flags().BoolVar(&authLoginNoBrowser, "no-browser", false, "never open a browser")
	authLoginCmd.Flags().StringVar(&authLoginScope, "scope", horizon.EnvOr("KEYCLOAK_DEVICE_SCOPE", "openid"), "OAuth scope (device flow)")
	authLoginCmd.Flags().StringVar(&authLoginClientID, "client-id", "", "Keycloak client id (overrides KEYCLOAK_CLIENT_ID)")
	authLoginCmd.Flags().StringVar(&authLoginClientSecret, "client-secret", "", "Keycloak client secret (overrides KEYCLOAK_CLIENT_SECRET)")
	authLoginCmd.Flags().BoolVar(&authLoginWriteConfig, "write-config", false, "write domain/keycloak defaults to ~/.config/horizon/config.yaml")

	return authLoginCmd
}

func promptLine(in *bufio.Reader, label, def string) string {
	if def != "" {
		fmt.Fprintf(os.Stderr, "%s [%s]: ", label, def)
	} else {
		fmt.Fprintf(os.Stderr, "%s: ", label)
	}
	line, _ := in.ReadString('\n')
	line = strings.TrimSpace(line)
	if line == "" {
		return def
	}
	return line
}

func promptSecret(label string) (string, error) {
	fmt.Fprintf(os.Stderr, "%s: ", label)
	b, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func browserOpener() func(string) error {
	return func(url string) error {
		switch runtime.GOOS {
		case "linux":
			return exec.Command("xdg-open", url).Start()
		case "darwin":
			return exec.Command("open", url).Start()
		case "windows":
			return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
		default:
			return fmt.Errorf("unsupported OS for opening browser")
		}
	}
}
