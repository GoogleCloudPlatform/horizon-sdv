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
	"github.com/spf13/cobra"
)

var (
	FlagAPI    string
	FlagDomain string
)

func AddAPIFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().StringVar(&FlagAPI, "api", "", "Horizon API base URL (overrides env / config)")
	cmd.PersistentFlags().StringVar(&FlagDomain, "domain", "", "Horizon domain; derives API URL")
}

func GlobalFlagOverrides(module, template string) *FlagOverrides {
	return &FlagOverrides{
		APIBaseURL: FlagAPI,
		Domain:     FlagDomain,
		Module:     module,
		Template:   template,
	}
}

func LoadAPIClient() (*Client, *Config, error) {
	return LoadAPIClientWith("", "")
}

func LoadAPIClientWith(module, template string) (*Client, *Config, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return nil, nil, err
	}
	ApplyFlagOverrides(cfg, GlobalFlagOverrides(module, template))
	if err := RevalidateConfig(cfg); err != nil {
		return nil, nil, err
	}
	c, err := NewClient(cfg)
	if err != nil {
		return nil, nil, err
	}
	return c, cfg, nil
}
