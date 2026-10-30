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
	"gopkg.in/yaml.v3"
)

func NewConfigSetCmd() *cobra.Command {
	var configSetCmd = &cobra.Command{
		Use:   "set KEY VALUE",
		Short: "Set a key in ~/.config/horizon/config.yaml",
		Args:  cobra.ExactArgs(2),
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			k, v := args[0], args[1]
			allowed := map[string]bool{
				"domain": true, "api_base_url": true, "keycloak_base": true, "keycloak_realm": true,
				"module": true, "template": true,
			}
			if !allowed[k] {
				return fmt.Errorf("unsupported key %q (allowed: %v)", k, keysOf(allowed))
			}

			dir, err := horizon.HorizonConfigDir()
			if err != nil {
				return err
			}
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return err
			}
			path := filepath.Join(dir, "config.yaml")

			var root map[string]any
			if b, err := os.ReadFile(path); err == nil {
				_ = yaml.Unmarshal(b, &root)
			}
			if root == nil {
				root = map[string]any{}
			}
			root[k] = v
			out, err := yaml.Marshal(root)
			if err != nil {
				return err
			}
			if err := os.WriteFile(path, out, 0o644); err != nil {
				return err
			}
			fmt.Println(path)
			return nil
		},
	}

	return configSetCmd
}

func keysOf(m map[string]bool) []string {
	s := make([]string, 0, len(m))
	for k := range m {
		s = append(s, k)
	}
	return s
}
