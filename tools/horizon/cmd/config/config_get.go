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

package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	horizon "github.com/horizon-sdv/horizon/internal"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var configGetMerged bool

func NewConfigGetCmd() *cobra.Command {
	var configGetCmd = &cobra.Command{
		Use:   "get [key]",
		Short: "Print config.yaml, or one key; --merged prints effective file+env values",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			key := ""
			if len(args) > 0 {
				key = strings.TrimSpace(args[0])
			}

			if configGetMerged || horizon.IsJSON() {
				cfg, err := horizon.LoadConfig()
				if err != nil {
					return err
				}
				m := map[string]any{
					"domain":              cfg.Domain,
					"api_base_url":        cfg.BaseURL,
					"keycloak_base":       cfg.KeycloakBase,
					"keycloak_realm":      cfg.KeycloakRealm,
					"module":              cfg.Module,
					"template":            cfg.Template,
					"running_poll_limit":  cfg.RunningPollLimit,
					"wait_terminal_secs":  cfg.WaitTerminalSecs,
					"log_stream_max_secs": cfg.LogStreamMaxSecs,
					"log_stream_format":   cfg.LogStreamFormat,
				}
				if key != "" {
					if v, ok := m[key]; ok {
						fmt.Println(v)
						return nil
					}
					return fmt.Errorf("unknown key %q", key)
				}
				b, err := json.MarshalIndent(m, "", "  ")
				if err != nil {
					return err
				}
				fmt.Println(string(b))
				return nil
			}

			path, err := horizon.HorizonConfigFilePath()
			if err != nil {
				return err
			}
			b, err := os.ReadFile(path)
			if err != nil {
				if os.IsNotExist(err) {
					fmt.Fprintf(os.Stderr, "no file at %s (run `%s config init`)\n", path, horizon.ProgName())
					return nil
				}
				return err
			}
			if key != "" {
				var root map[string]any
				if err := yaml.Unmarshal(b, &root); err != nil {
					return err
				}
				v, ok := root[key]
				if !ok {
					return fmt.Errorf("key %q not in %s", key, path)
				}
				fmt.Println(v)
				return nil
			}
			fmt.Print(string(b))
			return nil
		},
	}

	configGetCmd.Flags().BoolVar(&configGetMerged, "merged", false, "print effective values (file+env), secrets redacted")

	return configGetCmd
}
