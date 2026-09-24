// Copyright (c) 2026 Accenture, All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
//
//         http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"golang.org/x/term"
)

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

func runAuthLogin(args []string) error {
	fs := flag.NewFlagSet("auth login", flag.ExitOnError)
	device := fs.Bool("device", false, "use OAuth 2.0 device flow (public client)")
	clientCreds := fs.Bool("client-credentials", false, "use client_credentials grant")
	openBrowser := fs.Bool("open-browser", false, "open verification URL in a browser")
	noBrowser := fs.Bool("no-browser", false, "never open a browser")
	scope := fs.String("scope", envOr("KEYCLOAK_DEVICE_SCOPE", "openid"), "OAuth scope (device flow)")
	api := fs.String("api", "", "Horizon API base URL (stored with --write-config)")
	domain := fs.String("domain", "", "Horizon domain (derives API URL; stored with --write-config)")
	clientIDFlag := fs.String("client-id", "", "Keycloak client id (overrides KEYCLOAK_CLIENT_ID)")
	clientSecretFlag := fs.String("client-secret", "", "Keycloak client secret (overrides KEYCLOAK_CLIENT_SECRET)")
	writeConfig := fs.Bool("write-config", false, "write domain/keycloak defaults to ~/.config/horizon/config.yaml")
	_ = fs.Parse(args)

	cfg, err := LoadConfigRelaxed()
	if err != nil {
		return err
	}
	ApplyFlagOverrides(cfg, &FlagOverrides{APIBaseURL: *api, Domain: *domain})
	if cfg.KeycloakBase == "" && cfg.Domain != "" {
		cfg.KeycloakBase = "https://" + cfg.Domain + "/auth"
	}
	if cfg.KeycloakBase == "" {
		return fmt.Errorf("Keycloak URL unknown: set KEYCLOAK_BASE or HORIZON_DOMAIN, or put domain in ~/.config/horizon/config.yaml, or pass --domain (e.g. %s auth login --device --domain horizon.example.com)", progName())
	}
	kb := cfg.KeycloakBase
	realm := cfg.KeycloakRealm

	in := bufio.NewReader(os.Stdin)
	useTTY := term.IsTerminal(int(os.Stdin.Fd()))
	autoOpen := useTTY && !*noBrowser && !*openBrowser
	if *openBrowser {
		autoOpen = true
	}

	defaultClientCI := "horizon-api-ci"
	defaultClientPub := "horizon-api"
	mode := "auto"
	if *device {
		mode = "device"
	}
	if *clientCreds {
		mode = "client_credentials"
	}

	secret := strings.TrimSpace(os.Getenv("KEYCLOAK_CLIENT_SECRET"))
	clientID := strings.TrimSpace(os.Getenv("KEYCLOAK_CLIENT_ID"))
	if s := strings.TrimSpace(*clientSecretFlag); s != "" {
		secret = s
	}
	if s := strings.TrimSpace(*clientIDFlag); s != "" {
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

	oauth := &OAuthClient{HTTP: defaultHTTPClient()}
	ctx := context.Background()

	var tc *TokenCache
	switch mode {
	case "client_credentials":
		if secret == "" {
			if !useTTY {
				return fmt.Errorf("KEYCLOAK_CLIENT_SECRET required non-interactively")
			}
			var err error
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
		tc, err = oauth.DeviceFlow(ctx, kb, realm, clientID, *scope, opener)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unknown auth mode %q", mode)
	}

	tc.KeycloakBase = strings.TrimSuffix(kb, "/")
	tc.Realm = realm
	tc.ClientID = clientID
	if err := saveTokenCache(tc); err != nil {
		return err
	}
	p, err := tokenCachePath()
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Logged in. Token saved to %s\n",p)
	if *writeConfig {
		apiBase := cfg.BaseURL
		if err := mergeLoginIntoConfigFile(cfg.Domain, apiBase, kb, realm); err != nil {
			return fmt.Errorf("write config: %w", err)
		}
		p, _ := HorizonConfigFilePath()
		fmt.Fprintf(os.Stderr, "Wrote defaults to %s\n", p)
	}
	return nil
}

func runAuthRefresh(args []string) error {
	fs := flag.NewFlagSet("auth refresh", flag.ExitOnError)
	api := fs.String("api", "", "Horizon API base URL (optional; same as login --api)")
	domain := fs.String("domain", "", "Horizon domain (optional; sets Keycloak URL if token cache lacks keycloak_base)")
	clientIDFlag := fs.String("client-id", "", "Keycloak client id (overrides token cache / KEYCLOAK_CLIENT_ID)")
	clientSecretFlag := fs.String("client-secret", "", "Keycloak client secret (confidential client; or KEYCLOAK_CLIENT_SECRET)")
	_ = fs.Parse(args)

	tc, err := loadTokenCache()
	if err != nil {
		return err
	}
	if tc == nil || strings.TrimSpace(tc.RefreshToken) == "" {
		return fmt.Errorf("no refresh token in token cache: run `%s auth login`", progName())
	}

	kb := strings.TrimSuffix(strings.TrimSpace(tc.KeycloakBase), "/")
	realm := strings.TrimSpace(tc.Realm)
	clientID := strings.TrimSpace(tc.ClientID)

	cfg, cfgErr := LoadConfigRelaxed()
	if cfgErr == nil {
		ApplyFlagOverrides(cfg, &FlagOverrides{APIBaseURL: *api, Domain: *domain})
		if cfg.KeycloakBase == "" && cfg.Domain != "" {
			cfg.KeycloakBase = "https://" + cfg.Domain + "/auth"
		}
		if kb == "" {
			kb = strings.TrimSuffix(strings.TrimSpace(cfg.KeycloakBase), "/")
		}
		if realm == "" {
			realm = cfg.KeycloakRealm
		}
		if clientID == "" {
			clientID = envOr("KEYCLOAK_CLIENT_ID", "horizon-api")
		}
	} else {
		if d := strings.TrimSpace(*domain); d != "" {
			kb = "https://" + d + "/auth"
		}
		if realm == "" {
			realm = envOr("KEYCLOAK_REALM", "horizon")
		}
		if clientID == "" {
			clientID = envOr("KEYCLOAK_CLIENT_ID", "horizon-api")
		}
		if kb == "" {
			return fmt.Errorf("Keycloak URL unknown (%v); set KEYCLOAK_BASE or HORIZON_DOMAIN, pass --domain, or run `%s auth login`", cfgErr, progName())
		}
	}

	if s := strings.TrimSpace(*clientIDFlag); s != "" {
		clientID = s
	}
	secret := strings.TrimSpace(os.Getenv("KEYCLOAK_CLIENT_SECRET"))
	if s := strings.TrimSpace(*clientSecretFlag); s != "" {
		secret = s
	}
	if clientID == "" {
		return fmt.Errorf("Keycloak client id unknown (token cache empty and not set)")
	}

	oauth := &OAuthClient{HTTP: defaultHTTPClient()}
	ctx := context.Background()
	nt, err := oauth.RefreshToken(ctx, kb, realm, clientID, secret, strings.TrimSpace(tc.RefreshToken))
	if err != nil {
		return err
	}
	nt.KeycloakBase = kb
	nt.Realm = realm
	nt.ClientID = clientID
	if err := saveTokenCache(nt); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "Access token refreshed. Saved to ~/.config/horizon/token.json")
	return nil
}

func runAuthLogout() error {
	if err := clearTokenCache(); err != nil {
		return err
	}
	p, err2 := tokenCachePath()
	if err2 != nil {
		return err2
	}
	fmt.Fprintf(os.Stderr, "Removed %s (if present).\n", p)
	return nil
}

// jwtClaimExpUnix returns JWT numeric exp (seconds since epoch) if present and valid.
func jwtClaimExpUnix(claims map[string]any) (int64, bool) {
	v, ok := claims["exp"]
	if !ok {
		return 0, false
	}
	switch x := v.(type) {
	case float64:
		if x <= 0 {
			return 0, false
		}
		return int64(x), true
	case int64:
		if x <= 0 {
			return 0, false
		}
		return x, true
	case int:
		if x <= 0 {
			return 0, false
		}
		return int64(x), true
	default:
		return 0, false
	}
}

func formatJWTExpHuman(expUnix int64) string {
	expAt := time.Unix(expUnix, 0).UTC()
	abs := expAt.Format(time.RFC3339)
	skew := time.Until(expAt).Round(time.Second)
	if skew > 0 {
		return fmt.Sprintf("%s (valid for %s)", abs, skew)
	}
	if skew < 0 {
		return fmt.Sprintf("%s (expired %s ago)", abs, (-skew).Round(time.Second).String())
	}
	return fmt.Sprintf("%s (expires now)", abs)
}

func runAuthWhoami() error {
	tok := strings.TrimSpace(os.Getenv("HORIZON_ACCESS_TOKEN"))
	if tok == "" {
		tc, err := loadTokenCache()
		if err != nil {
			return err
		}
		if tc == nil || tc.AccessToken == "" {
			return fmt.Errorf("no token: run `%s auth login` or set HORIZON_ACCESS_TOKEN", progName())
		}
		tok = tc.AccessToken
	}
	claims, err := decodeJWTPayload(tok)
	if err == nil {
		fmt.Println("JWT claims (unverified):")
		if s, ok := claims["sub"].(string); ok {
			fmt.Println("  sub:", s)
		}
		if s, ok := claims["preferred_username"].(string); ok {
			fmt.Println("  preferred_username:", s)
		}
		if expUnix, ok := jwtClaimExpUnix(claims); ok {
			fmt.Println("  exp:", formatJWTExpHuman(expUnix))
		}
	} else {
		fmt.Println("Token is not a JWT or could not be decoded:", err)
	}

	cfg, err := LoadConfig()
	if err != nil {
		fmt.Fprintln(os.Stderr, "(Skipping API check:", err, ")")
		return nil
	}
	c, err := newClient(cfg)
	if err != nil {
		return err
	}
	b, err := c.GetJSON(context.Background(), "/v1/catalog")
	if err != nil {
		return fmt.Errorf("catalog GET: %w", err)
	}
	fmt.Printf("Horizon API catalog: OK (%d bytes)\n", len(b))
	return nil
}

func runAuth(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("usage: %s auth <login|logout|refresh|whoami> [flags]", progName())
	}
	switch args[0] {
	case "login":
		return runAuthLogin(args[1:])
	case "logout":
		return runAuthLogout()
	case "refresh":
		return runAuthRefresh(args[1:])
	case "whoami":
		return runAuthWhoami()
	default:
		return fmt.Errorf("unknown auth subcommand %q", args[0])
	}
}
