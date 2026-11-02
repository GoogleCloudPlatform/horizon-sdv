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

package catalog

import (
	"context"
	"fmt"
	"strings"

	"github.com/horizon-sdv/horizon/internal"
	"github.com/spf13/cobra"
)

var (
	catalogGetOutput string
	catalogGetRaw    bool
)

func NewCatalogGetCmd() *cobra.Command {
	var catalogGetCmd = &cobra.Command{
		Use:   "get",
		Short: "List catalog modules, templates, and parameters",
		Args:  cobra.NoArgs,
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			c, _, err := horizon.LoadAPIClient()
			if err != nil {
				return err
			}

			output, _ := cobraCmd.Flags().GetString("output")
			raw, _ := cobraCmd.Flags().GetBool("raw")

			if horizon.IsJSON() {
				output = string(horizon.OutputJsonFormat)
			}

			if raw && strings.ToLower(strings.TrimSpace(output)) != string(horizon.OutputJsonFormat) {
				return fmt.Errorf("--raw flag can only be used with --output json")
			}

			return horizon.CmdCatalogGet(context.Background(), c, output, catalogGetRaw)
		},
	}

	catalogGetCmd.Flags().StringVar(&catalogGetOutput, "output", "text", "text | json (default text; with json, --raw is compact one line)")
	catalogGetCmd.Flags().BoolVar(&catalogGetRaw, "raw", false, "with --output json: single-line compact JSON")

	return catalogGetCmd
}
