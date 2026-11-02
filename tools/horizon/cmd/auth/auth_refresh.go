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
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/horizon-sdv/horizon/internal"
	"github.com/spf13/cobra"
)

var (
	authRefreshClientID     string
	authRefreshClientSecret string
)

func NewAuthRefreshCmd() *cobra.Command {
	var authRefreshCmd = &cobra.Command{
		Use:   "refresh",
		Short: "Refresh the access token from the token cache (no device flow)",
		Args:  cobra.NoArgs,
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			tc, err := horizon.LoadTokenCache()
			if err != nil {
				return err
			}
			if tc == nil || strings.TrimSpace(tc.RefreshToken) == "" {
				return fmt.Errorf("no refresh token in token cache: run `%s auth login`", horizon.ProgName())
			}

			kb := strings.TrimSuffix(strings.TrimSpace(tc.KeycloakBase), "/")
			realm := strings.TrimSpace(tc.Realm)
			clientID := strings.TrimSpace(tc.ClientID)

			cfg, cfgErr := horizon.LoadConfigRelaxed()
			if cfgErr == nil {
				horizon.ApplyFlagOverrides(cfg, horizon.GlobalFlagOverrides("", ""))
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
					clientID = horizon.EnvOr("KEYCLOAK_CLIENT_ID", "horizon-api")
				}
			} else {
				if d := strings.TrimSpace(horizon.FlagDomain); d != "" {
					kb = "https://" + d + "/auth"
				}
				if realm == "" {
					realm = horizon.EnvOr("KEYCLOAK_REALM", "horizon")
				}
				if clientID == "" {
					clientID = horizon.EnvOr("KEYCLOAK_CLIENT_ID", "horizon-api")
				}
				if kb == "" {
					return fmt.Errorf("Keycloak URL unknown (%v); set KEYCLOAK_BASE or HORIZON_DOMAIN, pass --domain, or run `%s auth login`", cfgErr, horizon.ProgName())
				}
			}

			if s := strings.TrimSpace(authRefreshClientID); s != "" {
				clientID = s
			}
			secret := strings.TrimSpace(os.Getenv("KEYCLOAK_CLIENT_SECRET"))
			if s := strings.TrimSpace(authRefreshClientSecret); s != "" {
				secret = s
			}
			if clientID == "" {
				return fmt.Errorf("Keycloak client id unknown (token cache empty and not set)")
			}

			oauth := &horizon.OAuthClient{HTTP: horizon.DefaultHTTPClientWithInsecure(horizon.IsInsecure())}
			nt, err := oauth.RefreshToken(context.Background(), kb, realm, clientID, secret, strings.TrimSpace(tc.RefreshToken))
			if err != nil {
				return err
			}
			nt.KeycloakBase = kb
			nt.Realm = realm
			nt.ClientID = clientID
			if err := horizon.SaveTokenCache(nt); err != nil {
				return err
			}
			fmt.Fprintln(os.Stderr, "Access token refreshed. Saved to ~/.config/horizon/token.json")
			return nil
		},
	}

	authRefreshCmd.Flags().StringVar(&authRefreshClientID, "client-id", "", "Keycloak client id (overrides token cache / KEYCLOAK_CLIENT_ID)")
	authRefreshCmd.Flags().StringVar(&authRefreshClientSecret, "client-secret", "", "Keycloak client secret (confidential client; or KEYCLOAK_CLIENT_SECRET)")

	return authRefreshCmd
}
