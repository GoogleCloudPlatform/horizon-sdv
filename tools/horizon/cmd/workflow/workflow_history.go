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
	workflowHistoryLimit    int
	workflowHistoryContinue string
	workflowHistoryPhase    string
)

func NewWorkflowHistoryCmd() *cobra.Command {
	var workflowHistoryCmd = &cobra.Command{
		Use:   "history",
		Short: "List workflow history as JSON",
		Args:  cobra.NoArgs,
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			c, _, err := horizon.LoadAPIClient()
			if err != nil {
				return err
			}
			return horizon.CmdWorkflowHistory(context.Background(), c, workflowHistoryLimit, workflowHistoryContinue, workflowHistoryPhase)
		},
	}

	workflowHistoryCmd.Flags().IntVar(&workflowHistoryLimit, "limit", 50, "list limit")
	workflowHistoryCmd.Flags().StringVar(&workflowHistoryContinue, "continue", "", "continue token")
	workflowHistoryCmd.Flags().StringVar(&workflowHistoryPhase, "phase", "", "phase filter (comma-separated)")

	return workflowHistoryCmd
}
