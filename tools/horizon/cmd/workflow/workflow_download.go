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
	workflowDownloadGenURL       bool
	workflowDownloadDuration     int
	workflowDownloadTemplateName string
	workflowDownloadNodeID       string
	workflowDownloadOutPath      string
	workflowDownloadStdout       bool
	workflowDownloadQuiet        bool
	workflowDownloadOutput       string
)

func NewWorkflowDownloadCmd() *cobra.Command {
	var workflowDownloadCmd = &cobra.Command{
		Use:   "download-artifact WORKFLOW_NAME ARTIFACT_NAME",
		Short: "Download a workflow artifact, or print a signed URL",
		Args:  cobra.ExactArgs(2),
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			c, _, err := horizon.LoadAPIClient()
			if err != nil {
				return err
			}
			outputMode := horizon.OutputTextFormat
			if workflowDownloadOutput == "json" || horizon.IsJSON() {
				outputMode = horizon.OutputJsonFormat
			}
			return horizon.CmdWorkflowDownloadArtifact(context.Background(), c, args[0], args[1], workflowDownloadGenURL, workflowDownloadDuration, workflowDownloadTemplateName, workflowDownloadNodeID, workflowDownloadOutPath, workflowDownloadStdout, outputMode, workflowDownloadQuiet)
		},
	}

	workflowDownloadCmd.Flags().BoolVar(&workflowDownloadGenURL, "generate-signed-url", false, "print signed-url: <URL> on stdout (or --output json); do not download bytes")
	workflowDownloadCmd.Flags().IntVar(&workflowDownloadDuration, "duration", 0, "optional signed URL lifetime in seconds (maps to durationSeconds query; server clamps)")
	workflowDownloadCmd.Flags().StringVar(&workflowDownloadTemplateName, "template-name", "", "disambiguate artifact (maps to templateName query; see workflow show outputArtifacts)")
	workflowDownloadCmd.Flags().StringVar(&workflowDownloadNodeID, "node-id", "", "disambiguate artifact by node ID from 409 candidates list (maps to nodeId query; see workflow show outputArtifacts)")
	workflowDownloadCmd.Flags().StringVarP(&workflowDownloadOutPath, "o", "o", "", "write downloaded bytes to this path")
	workflowDownloadCmd.Flags().BoolVar(&workflowDownloadStdout, "stdout", false, "write downloaded bytes to stdout")
	workflowDownloadCmd.Flags().BoolVarP(&workflowDownloadQuiet, "q", "q", false, "when downloading: no progress on stderr")
	workflowDownloadCmd.Flags().StringVar(&workflowDownloadOutput, "output", "text", "with --generate-signed-url: text (signed-url line) or json")

	return workflowDownloadCmd
}
