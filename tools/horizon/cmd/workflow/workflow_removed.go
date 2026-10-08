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
