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

package workflow

import (
	"context"

	horizon "github.com/horizon-sdv/horizon/internal"
	"github.com/spf13/cobra"
)

var workflowShowOutput string

func NewWorkflowShowCmd() *cobra.Command {
	var workflowShowCmd = &cobra.Command{
		Use:   "show WORKFLOW_NAME",
		Short: "Show workflow detail (text summary or JSON)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			c, _, err := horizon.LoadAPIClient()
			if err != nil {
				return err
			}
			outputMode := horizon.OutputTextFormat
			if workflowShowOutput == "json" || horizon.IsJSON() {
				outputMode = horizon.OutputJsonFormat
			}
			return horizon.CmdWorkflowShow(context.Background(), c, args[0], outputMode)
		},
	}

	workflowShowCmd.Flags().StringVar(&workflowShowOutput, "output", "text", "text | json")

	return workflowShowCmd
}
