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
	horizon "github.com/horizon-sdv/horizon/internal"
	"github.com/spf13/cobra"
)

func NewApiCmd() *cobra.Command {
	var apiCmd = &cobra.Command{
		Use:   "api",
		Short: "Discover Horizon API paths and examples",
	}

	horizon.AddAPIFlags(apiCmd)
	apiCmd.AddCommand(NewApiListCmd())
	apiCmd.AddCommand(NewApiPathsCmd())
	apiCmd.AddCommand(NewApiRefreshCmd())
	apiCmd.AddCommand(NewApiShowCmd())

	return apiCmd
}
