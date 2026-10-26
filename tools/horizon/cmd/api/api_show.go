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

func NewApiShowCmd() *cobra.Command {
	var apiShowCmd = &cobra.Command{
		Use:   "show <operationId|METHOD PATH>",
		Short: "Show operation details and example request",
		Long:  "Show detailed information about a specific API operation including parameters and example request. Accepts either an operationId or METHOD PATH (e.g., 'GET /workflows').",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			doc, err := horizon.LoadOpenAPIDocument()
			if err != nil {
				return err
			}

			method, path, operationID, err := horizon.ParseOperationRef(args)
			if err != nil {
				return err
			}

			op, err := horizon.FindOperation(doc.Operations, method, path, operationID)
			if err != nil {
				return err
			}

			_, cfg, err := horizon.LoadAPIClient()
			if err != nil {
				return err
			}
			apiBase := horizon.ExampleAPIBase(cfg)

			if horizon.IsJSON() {
				b, err := horizon.OperationShowJSON(doc, *op, apiBase)
				if err != nil {
					return err
				}
				fmt.Println(string(b))
				return nil
			}

			fmt.Print(horizon.FormatOperationShow(doc, *op, apiBase))
			return nil
		},
	}

	return apiShowCmd
}
