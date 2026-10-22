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

func NewWorkflowCmd() *cobra.Command {
	var workflowCmd = &cobra.Command{
		Use:     "workflow",
		Aliases: []string{"ci", "job"},
		Short:   "Workflow/job commands",
	}

	horizon.AddAPIFlags(workflowCmd)
	workflowCmd.AddCommand(NewWorkflowAbortCmd())
	workflowCmd.AddCommand(NewWorkflowDeleteCmd())
	workflowCmd.AddCommand(NewWorkflowDownloadCmd())
	workflowCmd.AddCommand(NewWorkflowGetCmd())
	workflowCmd.AddCommand(NewWorkflowHistoryCmd())
	workflowCmd.AddCommand(NewWorkflowListCmd())
	workflowCmd.AddCommand(NewWorkflowLogsCmd())
	workflowCmd.AddCommand(NewWorkflowFinalizeCmd())
	workflowCmd.AddCommand(NewWorkflowPollCmd())
	workflowCmd.AddCommand(NewWorkflowRunningCmd())
	workflowCmd.AddCommand(NewWorkflowShowCmd())
	workflowCmd.AddCommand(NewWorkflowSubmitCmd())
	workflowCmd.AddCommand(NewWorkflowWaitCmd())

	return workflowCmd
}
