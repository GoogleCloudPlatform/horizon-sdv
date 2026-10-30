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
	horizon "github.com/horizon-sdv/horizon/internal"
	"github.com/spf13/cobra"
)

var workflowWaitLogs bool

func NewWorkflowWaitCmd() *cobra.Command {
	var workflowWaitCmd = &cobra.Command{
		Use:   "wait WORKFLOW_NAME",
		Short: "Poll until a workflow reaches a terminal phase",
		Long:  "Poll until terminal. Exit 0=Succeeded, 1=Failed/Error, 2=Aborted.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			c, cfg, err := horizon.LoadAPIClient()
			if err != nil {
				return err
			}
			return horizon.WorkflowResult(horizon.CmdWait(c, cfg, args[0], workflowWaitLogs))
		},
	}

	workflowWaitCmd.Flags().BoolVar(&workflowWaitLogs, "logs", false, "stream workflow logs concurrently until terminal (stdout; progress on stderr)")

	return workflowWaitCmd
}
