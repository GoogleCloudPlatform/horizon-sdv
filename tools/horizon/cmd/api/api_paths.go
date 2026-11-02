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

package api

import (
	"fmt"
	"sort"

	horizon "github.com/horizon-sdv/horizon/internal"
	"github.com/spf13/cobra"
)

func NewApiPathsCmd() *cobra.Command {
	var apiPathsCmd = &cobra.Command{
		Use:   "paths",
		Short: "List all API paths",
		Long:  "List all unique API paths available in the Horizon API.",
		Args:  cobra.NoArgs,
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			doc, err := horizon.LoadOpenAPIDocument()
			if err != nil {
				return err
			}

			paths := make(map[string]bool)
			for _, op := range doc.Operations {
				paths[op.Path] = true
			}

			uniquePaths := make([]string, 0, len(paths))
			for path := range paths {
				uniquePaths = append(uniquePaths, path)
			}
			sort.Strings(uniquePaths)

			if horizon.IsJSON() {
				b, err := horizon.PathsJSON(uniquePaths)
				if err != nil {
					return err
				}
				fmt.Println(string(b))
				return nil
			}

			for _, path := range uniquePaths {
				fmt.Println(path)
			}

			return nil
		},
	}

	return apiPathsCmd
}
