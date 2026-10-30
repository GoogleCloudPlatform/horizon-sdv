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

	horizon "github.com/horizon-sdv/horizon/internal"
	"github.com/spf13/cobra"
)

var (
	apiListTag        string
	apiListMethod     string
	apiListPathPrefix string
)

func NewApiListCmd() *cobra.Command {
	var apiListCmd = &cobra.Command{
		Use:   "list",
		Short: "List API operations with method, path, and summary",
		Long:  "List Horizon API operations showing method, path, and summary. Supports filtering by tag, method, and path prefix.",
		Args:  cobra.NoArgs,
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			doc, err := horizon.LoadOpenAPIDocument()
			if err != nil {
				return err
			}

			filter := horizon.OperationListFilter{
				Tag:        apiListTag,
				Method:     apiListMethod,
				PathPrefix: apiListPathPrefix,
			}

			ops := horizon.FilterOperations(doc.Operations, filter)

			if horizon.IsJSON() {
				b, err := horizon.OperationsListJSON(ops)
				if err != nil {
					return err
				}
				fmt.Println(string(b))
				return nil
			}

			fmt.Print(horizon.FormatOperationList(ops))
			return nil
		},
	}

	apiListCmd.Flags().StringVar(&apiListTag, "tag", "", "Filter by tag")
	apiListCmd.Flags().StringVar(&apiListMethod, "method", "", "Filter by HTTP method (GET, POST, etc.)")
	apiListCmd.Flags().StringVar(&apiListPathPrefix, "path-prefix", "", "Filter by path prefix")

	return apiListCmd
}
