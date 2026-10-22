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
	"time"

	"github.com/horizon-sdv/horizon/internal"
	"github.com/spf13/cobra"
)

func NewAuthWhoamiCmd() *cobra.Command {
	var authWhoamiCmd = &cobra.Command{
		Use:   "whoami",
		Short: "Show JWT claims and verify the token against the Horizon API",
		Args:  cobra.NoArgs,
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			tok := strings.TrimSpace(os.Getenv("HORIZON_ACCESS_TOKEN"))
			if tok == "" {
				tc, err := horizon.LoadTokenCache()
				if err != nil {
					return err
				}
				if tc == nil || tc.AccessToken == "" {
					return fmt.Errorf("no token: run `%s auth login` or set HORIZON_ACCESS_TOKEN", horizon.ProgName())
				}
				tok = tc.AccessToken
			}
			claims, err := horizon.DecodeJWTPayload(tok)
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

			cfg, err := horizon.LoadConfig()
			if err != nil {
				fmt.Fprintln(os.Stderr, "(Skipping API check:", err, ")")
				return nil
			}
			horizon.ApplyFlagOverrides(cfg, horizon.GlobalFlagOverrides("", ""))
			if err := horizon.RevalidateConfig(cfg); err != nil {
				fmt.Fprintln(os.Stderr, "(Skipping API check:", err, ")")
				return nil
			}
			c, err := horizon.NewClient(cfg)
			if err != nil {
				return err
			}
			b, err := c.GetJSON(context.Background(), "/v1/catalog")
			if err != nil {
				return fmt.Errorf("catalog GET: %w", err)
			}
			fmt.Printf("Horizon API catalog: OK (%d bytes)\n", len(b))
			return nil
		},
	}

	return authWhoamiCmd
}

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
