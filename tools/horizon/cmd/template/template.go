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

package template

import (
	"strings"
	"unicode"

	horizon "github.com/horizon-sdv/horizon/internal"
	"github.com/spf13/cobra"
)

func NewCustomHelpTemplate() string {
	cobra.AddTemplateFunc("useSuppress", func() bool {
		return horizon.IsNoBanner()
	})
	cobra.AddTemplateFunc("useBanner", func() string {
		return "\033[1;34m" + `
██╗  ██╗ ██████╗ ██████╗ ██╗███████╗ ██████╗ ███╗   ██╗
██║  ██║██╔═══██╗██╔══██╗██║╚══███╔╝██╔═══██╗████╗  ██║
███████║██║   ██║██████╔╝██║  ███╔╝ ██║   ██║██╔██╗ ██║
██╔══██║██║   ██║██╔══██╗██║ ███╔╝  ██║   ██║██║╚██╗██║
██║  ██║╚██████╔╝██║  ██║██║███████╗╚██████╔╝██║ ╚████║
╚═╝  ╚═╝ ╚═════╝ ╚═╝  ╚═╝╚═╝╚══════╝ ╚═════╝ ╚═╝  ╚═══╝
` + "\033[0m"
	})

	cobra.AddTemplateFunc("useVersion", func() string {
		name := SentenceCase(horizon.GetCurrentName())
		version := horizon.GetCurrentVersion()
		buildDate := horizon.GetCurrentBuildDate()

		return "\033[90m" + name + " CLI " + version + " (built " + buildDate + ")" + "\033[0m"
	})

	const customHelpTemplate = `{{if useSuppress}}
Usage:{{if .Runnable}}
   {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
   {{.CommandPath}} [command] [flag]{{end}}{{if gt (len .Aliases) 0}}

Aliases:
   {{.NameAndAliases}}{{end}}{{if .HasExample}}

Examples:
   {{.Example}}{{end}}{{if .HasAvailableSubCommands -}}{{$cmds := .Commands}}

Available Commands:{{range $cmds}}{{if (and .IsAvailableCommand)}}
   {{rpad .Name .NamePadding}} {{with (or .Long .Short | trimTrailingWhitespaces)}}{{.}}{{end}}
   {{- end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

Additional help topics:{{range .Commands}}{{if .IsAdditionalHelpTopicsCommand}}
   {{rpad .CommandPath .CommandPathPadding}} {{with (or .Long .Short | trimTrailingWhitespaces)}}{{.}}
   {{end}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}{{else}}
{{useBanner}}
{{useVersion}}

Usage:{{if .Runnable}}
   {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
   {{.CommandPath}} [command] [flag]{{end}}{{if gt (len .Aliases) 0}}

Aliases:
   {{.NameAndAliases}}{{end}}{{if .HasExample}}

Examples:
   {{.Example}}{{end}}{{if .HasAvailableSubCommands -}}{{$cmds := .Commands}}

Available Commands:{{range $cmds}}{{if (and .IsAvailableCommand)}}
   {{rpad .Name .NamePadding}} {{with (or .Long .Short | trimTrailingWhitespaces)}}{{.}}{{end}}
   {{- end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}

Global Flags:
{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasHelpSubCommands}}

Additional help topics:{{range .Commands}}{{if .IsAdditionalHelpTopicsCommand}}
   {{rpad .CommandPath .CommandPathPadding}} {{with (or .Long .Short | trimTrailingWhitespaces)}}{{.}}
   {{end}}{{end}}{{end}}{{end}}{{if .HasAvailableSubCommands}}

Use "{{.CommandPath}} [command] --help" for more information about a command.{{end}}{{end}}
`
	return customHelpTemplate
}

func SentenceCase(s string) string {
	if s == "" {
		return ""
	}

	s = strings.ToLower(s)
	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])

	return string(runes)
}
