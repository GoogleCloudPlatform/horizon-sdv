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

package workflow

import (
	"fmt"

	horizon "github.com/horizon-sdv/horizon/internal"
	"github.com/spf13/cobra"
)

func NewWorkflowFinalizeCmd() *cobra.Command {
	var workflowFinalizeCmd = &cobra.Command{
		Use:    "finalize",
		Short:  "Removed; use workflow wait or show",
		Hidden: true,
		Args:   cobra.ArbitraryArgs,
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			return fmt.Errorf("workflow finalize was removed; use: %s workflow wait <workflowName> and/or %s workflow show <workflowName>", horizon.ProgName(), horizon.ProgName())
		},
	}

	return workflowFinalizeCmd
}

func NewWorkflowPollCmd() *cobra.Command {
	var workflowPollCmd = &cobra.Command{
		Use:    "poll",
		Short:  "Removed; use workflow wait",
		Hidden: true,
		Args:   cobra.ArbitraryArgs,
		RunE: func(cobraCmd *cobra.Command, args []string) error {
			return fmt.Errorf("workflow poll was removed; use: %s workflow wait <workflowName> (name is printed by submit)", horizon.ProgName())
		},
	}

	return workflowPollCmd
}
