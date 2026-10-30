// Copyright (c) 2026 Accenture, All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package artifacts

import (
	"context"

	horizon "github.com/horizon-sdv/horizon/internal"
	"github.com/spf13/cobra"
)

var (
	storageListObjectsPrefix    string
	storageListObjectsLimit     int
	storageListObjectsPageToken string
)

func NewArtifactsObjectsCmd() *cobra.Command {
	var artifactsObjectsCmd = &cobra.Command{
		Use:   "objects BUCKET",
		Short: "List objects in a GCS bucket",
		Long:  "List objects in a GCS bucket through the Horizon API. Supports prefix filtering and pagination.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			c, _, err := horizon.LoadAPIClient()
			if err != nil {
				return err
			}
			outputMode := horizon.OutputTextFormat
			if horizon.IsJSON() {
				outputMode = horizon.OutputJsonFormat
			}
			return horizon.CmdArtifactsListObjects(context.Background(), c, args[0], storageListObjectsPrefix, storageListObjectsPageToken, storageListObjectsLimit, outputMode)
		},
	}

	artifactsObjectsCmd.Flags().StringVar(&storageListObjectsPrefix, "prefix", "", "Filter objects by prefix")
	artifactsObjectsCmd.Flags().IntVar(&storageListObjectsLimit, "limit", 0, "Maximum number of objects to return")
	artifactsObjectsCmd.Flags().StringVar(&storageListObjectsPageToken, "page-token", "", "Pagination token for next page")

	return artifactsObjectsCmd
}
