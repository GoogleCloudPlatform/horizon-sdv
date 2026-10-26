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
	"context"
	"fmt"

	horizon "github.com/horizon-sdv/horizon/internal"
	"github.com/spf13/cobra"
)

func NewApiRefreshCmd() *cobra.Command {
	var apiRefreshCmd = &cobra.Command{
		Use:   "refresh",
		Short: "Refresh OpenAPI cache from API",
		Long:  "Fetch the latest OpenAPI specification from the Horizon API and cache it locally. This updates the embedded OpenAPI snapshot with live data from {api}/openapi.json.",
		Args:  cobra.NoArgs,
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			c, _, err := horizon.LoadAPIClient()
			if err != nil {
				return err
			}

			path, err := horizon.RefreshOpenAPICache(context.Background(), c)
			if err != nil {
				return err
			}

			fmt.Printf("OpenAPI cache refreshed and saved to: %s\n", path)
			return nil
		},
	}

	return apiRefreshCmd
}
