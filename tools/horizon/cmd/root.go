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

package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/horizon-sdv/horizon/cmd/api"
	"github.com/horizon-sdv/horizon/cmd/artifacts"
	"github.com/horizon-sdv/horizon/cmd/auth"
	"github.com/horizon-sdv/horizon/cmd/catalog"
	"github.com/horizon-sdv/horizon/cmd/config"
	"github.com/horizon-sdv/horizon/cmd/history"
	"github.com/horizon-sdv/horizon/cmd/template"
	"github.com/horizon-sdv/horizon/cmd/version"
	"github.com/horizon-sdv/horizon/cmd/workflow"
	horizon "github.com/horizon-sdv/horizon/internal"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "horizon",
	Short: "Horizon API CLI (CI workflows, catalog, auth)",
}

func Execute() {
	rootCmd.InitDefaultHelpCmd()
	rootCmd.InitDefaultCompletionCmd()

	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "completion" {
			cmd.Long = "Generate the autocompletion script for the specified shell"
		}
	}

	if err := rootCmd.Execute(); err != nil {
		var ee *horizon.ExitError
		if errors.As(err, &ee) {
			os.Exit(ee.Code)
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	setupGlobalFlags()
	setupParentCommands()
	setupCustomHelpTemplate()

	rootCmd.PersistentPreRun = func(cmd *cobra.Command, args []string) {
		cmd.SilenceUsage = true
		rootCmd.SilenceErrors = true
		horizon.SetDebugEnabled(horizon.IsDebug())

		if cmd.Name() == "history" {
			return
		}

		if err := horizon.AppendInvocation(os.Args); err != nil && horizon.IsDebug() {
			fmt.Fprintf(os.Stderr, "history: %v\n", err)
		}
	}
}

func setupGlobalFlags() {
	rootCmd.PersistentFlags().BoolVarP(horizon.TargetDebug(), "debug", "d", false, "Enable debug output")
	rootCmd.PersistentFlags().BoolVarP(horizon.TargetJSON(), "json", "j", false, "Output as JSON")
	rootCmd.PersistentFlags().BoolVarP(horizon.TargetNoBanner(), "no-banner", "b", false, "Suppress ASCII logo and banner")
	rootCmd.PersistentFlags().BoolVarP(horizon.TargetInsecure(), "insecure", "i", false, "Allow insecure TLS (self-signed certificates) - dev only")
}

func setupParentCommands() {
	rootCmd.AddCommand(api.NewApiCmd())
	rootCmd.AddCommand(auth.NewAuthCmd())
	rootCmd.AddCommand(catalog.NewCatalogCmd())
	rootCmd.AddCommand(config.NewConfigCmd())
	rootCmd.AddCommand(artifacts.NewArtifactsCmd())
	rootCmd.AddCommand(history.NewHistoryCmd())
	rootCmd.AddCommand(workflow.NewWorkflowCmd())
	rootCmd.AddCommand(version.NewVersionCmd())
}

func setupCustomHelpTemplate() {
	rootCmd.SetHelpTemplate(template.NewCustomHelpTemplate())
}
