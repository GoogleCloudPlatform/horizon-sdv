// Copyright (c) 2026 Accenture, All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
//
//         http://www.apache.org/licenses/LICENSE-2.0
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

func NewArtifactsBucketsCmd() *cobra.Command {
	var artifactsBucketsCmd = &cobra.Command{
		Use:   "buckets",
		Short: "List allowed GCS buckets",
		Long:  "List GCS buckets that are accessible through the Horizon API. Only buckets in the allowlist or matching GCSBucket CRs are returned.",
		Args:  cobra.NoArgs,
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			c, _, err := horizon.LoadAPIClient()
			if err != nil {
				return err
			}
			outputMode := horizon.OutputTextFormat
			if horizon.IsJSON() {
				outputMode = horizon.OutputJsonFormat
			}
			return horizon.CmdArtifactsListBuckets(context.Background(), c, outputMode)
		},
	}

	return artifactsBucketsCmd
}
