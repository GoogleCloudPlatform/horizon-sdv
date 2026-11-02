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
	"fmt"
	"os"

	horizon "github.com/horizon-sdv/horizon/internal"
	"github.com/spf13/cobra"
)

var (
	workflowSubmitParamsFile string
	workflowSubmitParamsJSON string
	workflowSubmitTemplate   string
	workflowSubmitModule     string
	workflowSubmitWait       bool
	workflowSubmitLogs       bool
	workflowSubmitOutput     string
	workflowSubmitQuiet      bool
)

func NewWorkflowSubmitCmd() *cobra.Command {
	var workflowSubmitCmd = &cobra.Command{
		Use:   "submit",
		Short: "Submit a workflow from a catalog template",
		Long:  `Submit a workflow. Prints the workflow name. With --wait, polls until a terminal phase and exits 0=Succeeded, 1=Failed/Error, 2=Aborted.`,
		Args:  cobra.NoArgs,
		Example: `horizon workflow submit --module sample --template sample-smoke-test --params-json '{"sampleEnv":"jenkins"}' --wait --logs
   horizon workflow submit --module sample --template sample-smoke-test --params-json '{"sampleEnv":"jenkins"}' --output json -q`,
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			if workflowSubmitLogs && !workflowSubmitWait {
				return fmt.Errorf("--logs requires --wait")
			}
			c, cfg, err := horizon.LoadAPIClientWith(workflowSubmitModule, workflowSubmitTemplate)
			if err != nil {
				return err
			}
			outputMode := horizon.OutputTextFormat
			if workflowSubmitOutput == "json" || horizon.IsJSON() {
				outputMode = horizon.OutputJsonFormat
			}
			return horizon.WorkflowResult(horizon.CmdSubmit(c, cfg, workflowSubmitParamsFile, workflowSubmitParamsJSON, workflowSubmitWait, workflowSubmitLogs, outputMode, workflowSubmitQuiet))
		},
	}

	workflowSubmitCmd.Flags().StringVar(&workflowSubmitParamsFile, "params-file", os.Getenv("WORKFLOW_PARAMETERS_JSON_FILE"), "JSON parameters object (file, or - for stdin)")
	workflowSubmitCmd.Flags().StringVar(&workflowSubmitParamsJSON, "params-json", os.Getenv("WORKFLOW_PARAMETERS_JSON"), "JSON parameters object (inline)")
	workflowSubmitCmd.Flags().StringVar(&workflowSubmitTemplate, "template", "", "workflow template (default: env / config)")
	workflowSubmitCmd.Flags().StringVar(&workflowSubmitModule, "module", "", "module name (default: env / config)")
	workflowSubmitCmd.Flags().BoolVar(&workflowSubmitWait, "wait", false, "after submit, poll until terminal (exit 0/1/2 by phase)")
	workflowSubmitCmd.Flags().BoolVar(&workflowSubmitLogs, "logs", false, "with --wait: stream workflow logs concurrently until done (stdout; progress on stderr)")
	workflowSubmitCmd.Flags().StringVar(&workflowSubmitOutput, "output", "text", "text | json")
	workflowSubmitCmd.Flags().BoolVarP(&workflowSubmitQuiet, "q", "q", false, "less stderr noise (still prints workflow name / JSON)")

	return workflowSubmitCmd
}
