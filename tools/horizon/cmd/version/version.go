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

package version

import (
	"fmt"
	"strings"

	horizon "github.com/horizon-sdv/horizon/internal"
	"github.com/spf13/cobra"
)

var (
	versionGetOutput string
	versionRaw       bool
)

func NewVersionCmd() *cobra.Command {
	var versionCmd = &cobra.Command{
		Use:   "version",
		Short: "Show CLI version",
		Args:  cobra.NoArgs,
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			output, _ := cobraCmd.Flags().GetString("output")
			raw, _ := cobraCmd.Flags().GetBool("raw")

			if horizon.IsJSON() {
				output = string(horizon.OutputJsonFormat)
			}

			if raw && strings.ToLower(strings.TrimSpace(output)) != string(horizon.OutputJsonFormat) {
				return fmt.Errorf("--raw flag can only be used with --output json")
			}

			s, err := horizon.FormatVersion(output, raw)
			if err != nil {
				return err
			}
			fmt.Fprint(cobraCmd.OutOrStdout(), s)
			return nil
		},
	}

	versionCmd.Flags().StringVar(&versionGetOutput, "output", "text", "text | json (default text)")
	versionCmd.Flags().BoolVar(&versionRaw, "raw", false, "compact JSON output (only with --output json)")

	return versionCmd
}
