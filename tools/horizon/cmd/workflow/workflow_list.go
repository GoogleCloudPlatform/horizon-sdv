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

var (
	workflowListRunningOnly bool
	workflowListLimit       int
	workflowListContinue    string
	workflowListPhase       string
	workflowListOutput      string
)

func NewWorkflowListCmd() *cobra.Command {
	var workflowListCmd = &cobra.Command{
		Use:   "list",
		Short: "List running and history workflows",
		Args:  cobra.NoArgs,
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			c, _, err := horizon.LoadAPIClient()
			if err != nil {
				return err
			}
			outputMode := horizon.OutputTextFormat
			if workflowListOutput == "json" || horizon.IsJSON() {
				outputMode = horizon.OutputJsonFormat
			}
			return horizon.CmdWorkflowList(context.Background(), c, workflowListRunningOnly, workflowListLimit, workflowListContinue, workflowListPhase, outputMode)
		},
	}

	workflowListCmd.Flags().BoolVar(&workflowListRunningOnly, "running-only", false, "only GET /workflows/running")
	workflowListCmd.Flags().IntVar(&workflowListLimit, "limit", 50, "list limit")
	workflowListCmd.Flags().StringVar(&workflowListContinue, "continue", "", "history continue token")
	workflowListCmd.Flags().StringVar(&workflowListPhase, "phase", "", "history phase filter (comma-separated)")
	workflowListCmd.Flags().StringVar(&workflowListOutput, "output", "", "text | json | wide (default: TTY=text else json)")

	return workflowListCmd
}
