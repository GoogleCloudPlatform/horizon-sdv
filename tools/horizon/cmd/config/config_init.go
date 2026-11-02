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
	"fmt"
	"os"
	"path/filepath"

	horizon "github.com/horizon-sdv/horizon/internal"
	"github.com/spf13/cobra"
)

const configInitHelp = `# Horizon CLI defaults (optional). Precedence: CLI flags > environment > this file.
# domain: example.com
# api_base_url: https://example.com/horizon-api
# keycloak_base: https://example.com/auth
# keycloak_realm: horizon
# module: sample
# template: sample-smoke-test
`

var configInitForce bool

func NewConfigInitCmd() *cobra.Command {
	var configInitCmd = &cobra.Command{
		Use:   "init",
		Short: "Create a starter ~/.config/horizon/config.yaml",
		Args:  cobra.NoArgs,
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			dir, err := horizon.HorizonConfigDir()
			if err != nil {
				return err
			}
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return err
			}
			path := filepath.Join(dir, "config.yaml")
			if _, err := os.Stat(path); err == nil && !configInitForce {
				fmt.Fprintf(os.Stderr, "%s exists (use --force to overwrite)\n", path)
				return nil
			}
			if err := os.WriteFile(path, []byte(configInitHelp), 0o644); err != nil {
				return err
			}
			fmt.Println(path)
			return nil
		},
	}

	configInitCmd.Flags().BoolVar(&configInitForce, "force", false, "overwrite existing config.yaml")

	return configInitCmd
}
