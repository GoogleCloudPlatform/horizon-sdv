// Copyright (c) 2026 Accenture, All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
//
//         http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Command horizon is the Horizon API CLI (CI workflows, catalog, auth).
package main

import (
	"context"
	"flag"
	"fmt"
	"net/url"
	"os"
	"strings"
)

func printUsage() {
	p := progName()
	fmt.Fprintf(os.Stderr, `%s — Horizon API CLI

Usage:
  %s config <init|get|set> ...
  %s auth <login|logout|refresh|whoami> [flags]
  %s catalog get [--output text|json] [flags]   (default output: text)
  %s version [--output text|json]
  %s workflow <submit|wait|logs|abort|delete|show|list|get|running|history|download-artifact> ... [flags]

  %s ci <submit|wait|logs|abort|delete|show|list|get|...> ...   (alias for %s workflow)

Environment (common):
  HORIZON_DOMAIN or HORIZON_API_BASE_URL
  HORIZON_ACCESS_TOKEN or KEYCLOAK_CLIENT_SECRET (CI)
  KEYCLOAK_BASE, KEYCLOAK_REALM, KEYCLOAK_CLIENT_ID, KEYCLOAK_CLIENT_SECRET
  Optional defaults file: ~/.config/horizon/config.yaml (see %s config init)
  MODULE, TEMPLATE or --module / --template on submit (parameters must match GET /v1/catalog for that template)

Auth:
  %s auth login [--device|--client-credentials] [--domain HOST] [--write-config]
  %s auth logout
  %s auth refresh [--domain HOST]   (OAuth refresh_token from token cache; no device flow)
  %s auth whoami

Workflow (stateless — pass workflow name from submit output; no WORKFLOW_STATE_DIR):
  %s workflow submit [--module NAME] [--template NAME] [--params-file|-] [--params-json] [--wait] [--logs] [--output text|json] ...
  %s workflow wait [--logs] <workflowName>
  %s workflow logs <workflowName>
  %s workflow abort <workflowName>
  %s workflow delete <workflowName>
  %s workflow show <workflowName> [--output text|json]
  %s workflow list [--running-only] [--output text|json|wide]
  %s workflow get <workflowName>
  %s workflow running [--limit N] [--api URL] [--domain HOST]
  %s workflow history [--limit N] [--continue TOKEN] [--phase P] [--api URL] [--domain HOST]
  %s workflow download-artifact [--generate-signed-url] [--duration SECONDS] [--template-name NAME] [-o PATH] [-stdout] [-q] [--output text|json] [--api URL] [--domain HOST] <workflowName> <artifactName>

Examples (submit + wait):
  # Block until terminal phase (exit 0=Succeeded, 1=Failed/Error, 2=Aborted):
  %s workflow submit --module sample --template sample-smoke-test --params-json '{"sampleEnv":"jenkins","sampleBuildId":"build-1","sampleNote":"note"}' --wait --logs

  # Async submit, then wait by name (parse workflowName from JSON; -q is quieter stderr):
  %s workflow submit --module sample --template sample-smoke-test --params-json '{"sampleEnv":"jenkins"}' --output json -q
  %s workflow wait <workflowName>

`, p, p, p, p, p, p, p, p, p, p, p, p, p, p, p, p, p, p, p, p, p, p, p, p, p, p, p)
}

func main() {
	args := os.Args[1:]
	if len(args) == 0 {
		printUsage()
		os.Exit(2)
	}

	switch args[0] {
	case "-h", "--help", "help":
		printUsage()
		os.Exit(0)
	case "ci":
		args = append([]string{"workflow"}, args[1:]...)
	}

	if len(args) == 0 {
		printUsage()
		os.Exit(2)
	}

	switch args[0] {
	case "config":
		if err := runConfig(args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "config: %v\n", err)
			os.Exit(1)
		}
	case "auth":
		if err := runAuth(args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "auth: %v\n", err)
			os.Exit(1)
		}
	case "catalog":
		if err := runCatalog(args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "catalog: %v\n", err)
			os.Exit(1)
		}
	case "version":
		if err := runVersion(args[1:]); err != nil {
			fmt.Fprintf(os.Stderr, "version: %v\n", err)
			os.Exit(1)
		}
	case "workflow":
		code, err := runWorkflow(args[1:])
		if err != nil {
			fmt.Fprintf(os.Stderr, "workflow: %v\n", err)
			os.Exit(1)
		}
		os.Exit(code)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", args[0])
		printUsage()
		os.Exit(2)
	}
}

func runCatalog(args []string) error {
	if len(args) < 1 || args[0] != "get" {
		return fmt.Errorf("usage: %s catalog get [--output text|json] [-raw] [--api URL] [--domain HOST]", progName())
	}
	fs := flag.NewFlagSet("catalog get", flag.ExitOnError)
	out := fs.String("output", "text", "text | json (default text; with json, -raw is compact one line)")
	raw := fs.Bool("raw", false, "with -output json: single-line compact JSON")
	api := fs.String("api", "", "Horizon API base URL (overrides env / config)")
	domain := fs.String("domain", "", "Horizon domain; derives API URL")
	_ = fs.Parse(args[1:])

	cfg, err := LoadConfig()
	if err != nil {
		return err
	}
	ApplyFlagOverrides(cfg, &FlagOverrides{APIBaseURL: *api, Domain: *domain})
	if err := revalidateConfig(cfg); err != nil {
		return err
	}
	c, err := newClient(cfg)
	if err != nil {
		return err
	}
	mode := strings.TrimSpace(*out)
	if mode == "" {
		mode = "text"
	}
	return cmdCatalogGet(context.Background(), c, mode, *raw)
}

func revalidateConfig(cfg *Config) error {
	if cfg.BaseURL == "" && cfg.Domain != "" {
		cfg.BaseURL = "https://" + cfg.Domain + "/horizon-api"
	}
	if cfg.BaseURL == "" {
		return fmt.Errorf("set HORIZON_API_BASE_URL, HORIZON_DOMAIN, or use --api / --domain")
	}
	if cfg.KeycloakBase == "" && cfg.Domain != "" {
		cfg.KeycloakBase = "https://" + cfg.Domain + "/auth"
	}
	if _, err := url.Parse(cfg.BaseURL); err != nil {
		return fmt.Errorf("Horizon API base URL: %w", err)
	}
	return nil
}

func runWorkflow(args []string) (int, error) {
	if len(args) < 1 {
		return 2, fmt.Errorf("usage: %s workflow <submit|wait|logs|abort|delete|show|list|get|...> ...", progName())
	}

	ctx := context.Background()

	switch args[0] {
	case "submit":
		fs := flag.NewFlagSet("workflow submit", flag.ExitOnError)
		paramsFile := fs.String("params-file", os.Getenv("WORKFLOW_PARAMETERS_JSON_FILE"), "JSON parameters object (file, or - for stdin)")
		paramsEnv := fs.String("params-json", os.Getenv("WORKFLOW_PARAMETERS_JSON"), "JSON parameters object (inline)")
		tpl := fs.String("template", "", "workflow template (default: env / config)")
		mod := fs.String("module", "", "module name (default: env / config)")
		wait := fs.Bool("wait", false, "after submit, poll until terminal (exit 0/1/2 by phase)")
		logs := fs.Bool("logs", false, "with --wait: stream workflow logs concurrently until done (stdout; progress on stderr)")
		out := fs.String("output", "text", "text | json")
		quiet := fs.Bool("q", false, "less stderr noise (still prints workflow name / JSON)")
		api := fs.String("api", "", "Horizon API base URL")
		domain := fs.String("domain", "", "Horizon domain")
		_ = fs.Parse(args[1:])
		if *logs && !*wait {
			return 2, fmt.Errorf("--logs requires --wait")
		}

		cfg, err := LoadConfig()
		if err != nil {
			return 1, err
		}
		ApplyFlagOverrides(cfg, &FlagOverrides{APIBaseURL: *api, Domain: *domain, Module: *mod, Template: *tpl})
		if err := revalidateConfig(cfg); err != nil {
			return 1, err
		}
		c, err := newClient(cfg)
		if err != nil {
			return 1, err
		}
		mode := submitOutText
		if *out == "json" {
			mode = submitOutJSON
		}
		return cmdSubmit(c, cfg, *paramsFile, *paramsEnv, *wait, *logs, mode, *quiet)

	case "wait":
		fs := flag.NewFlagSet("workflow wait", flag.ExitOnError)
		logs := fs.Bool("logs", false, "stream workflow logs concurrently until terminal (stdout; progress on stderr)")
		api := fs.String("api", "", "Horizon API base URL")
		domain := fs.String("domain", "", "Horizon domain")
		_ = fs.Parse(args[1:])
		rem := fs.Args()
		if len(rem) < 1 {
			return 2, fmt.Errorf("usage: %s workflow wait [--logs] <workflowName>", progName())
		}
		cfg, err := LoadConfig()
		if err != nil {
			return 1, err
		}
		ApplyFlagOverrides(cfg, &FlagOverrides{APIBaseURL: *api, Domain: *domain})
		if err := revalidateConfig(cfg); err != nil {
			return 1, err
		}
		c, err := newClient(cfg)
		if err != nil {
			return 1, err
		}
		return cmdWait(c, cfg, rem[0], *logs)

	case "logs":
		fs := flag.NewFlagSet("workflow logs", flag.ExitOnError)
		api := fs.String("api", "", "Horizon API base URL")
		domain := fs.String("domain", "", "Horizon domain")
		_ = fs.Parse(args[1:])
		rem := fs.Args()
		if len(rem) < 1 {
			return 2, fmt.Errorf("usage: %s workflow logs <workflowName>", progName())
		}
		cfg, err := LoadConfig()
		if err != nil {
			return 1, err
		}
		ApplyFlagOverrides(cfg, &FlagOverrides{APIBaseURL: *api, Domain: *domain})
		if err := revalidateConfig(cfg); err != nil {
			return 1, err
		}
		c, err := newClient(cfg)
		if err != nil {
			return 1, err
		}
		return 0, cmdLogsForWorkflow(c, cfg, rem[0])

	case "abort":
		fs := flag.NewFlagSet("workflow abort", flag.ExitOnError)
		api := fs.String("api", "", "Horizon API base URL")
		domain := fs.String("domain", "", "Horizon domain")
		_ = fs.Parse(args[1:])
		rem := fs.Args()
		if len(rem) < 1 {
			return 2, fmt.Errorf("usage: %s workflow abort <workflowName>", progName())
		}
		cfg, err := LoadConfig()
		if err != nil {
			return 1, err
		}
		ApplyFlagOverrides(cfg, &FlagOverrides{APIBaseURL: *api, Domain: *domain})
		if err := revalidateConfig(cfg); err != nil {
			return 1, err
		}
		c, err := newClient(cfg)
		if err != nil {
			return 1, err
		}
		return 0, cmdAbortWorkflowName(c, rem[0])

	case "delete":
		fs := flag.NewFlagSet("workflow delete", flag.ExitOnError)
		api := fs.String("api", "", "Horizon API base URL")
		domain := fs.String("domain", "", "Horizon domain")
		_ = fs.Parse(args[1:])
		rem := fs.Args()
		if len(rem) < 1 {
			return 2, fmt.Errorf("usage: %s workflow delete <workflowName>", progName())
		}
		cfg, err := LoadConfig()
		if err != nil {
			return 1, err
		}
		ApplyFlagOverrides(cfg, &FlagOverrides{APIBaseURL: *api, Domain: *domain})
		if err := revalidateConfig(cfg); err != nil {
			return 1, err
		}
		c, err := newClient(cfg)
		if err != nil {
			return 1, err
		}
		return 0, cmdDeleteWorkflowName(ctx, c, rem[0])

	case "show":
		fs := flag.NewFlagSet("workflow show", flag.ExitOnError)
		out := fs.String("output", "text", "text | json")
		api := fs.String("api", "", "Horizon API base URL")
		domain := fs.String("domain", "", "Horizon domain")
		_ = fs.Parse(args[1:])
		rem := fs.Args()
		if len(rem) < 1 {
			return 2, fmt.Errorf("usage: %s workflow show <workflowName>", progName())
		}
		cfg, err := LoadConfig()
		if err != nil {
			return 1, err
		}
		ApplyFlagOverrides(cfg, &FlagOverrides{APIBaseURL: *api, Domain: *domain})
		if err := revalidateConfig(cfg); err != nil {
			return 1, err
		}
		c, err := newClient(cfg)
		if err != nil {
			return 1, err
		}
		return 0, cmdWorkflowShow(ctx, c, rem[0], *out)

	case "list":
		fs := flag.NewFlagSet("workflow list", flag.ExitOnError)
		runningOnly := fs.Bool("running-only", false, "only GET /workflows/running")
		limit := fs.Int("limit", 50, "list limit")
		cont := fs.String("continue", "", "history continue token")
		phase := fs.String("phase", "", "history phase filter (comma-separated)")
		out := fs.String("output", "", "text | json | wide (default: TTY=text else json)")
		api := fs.String("api", "", "Horizon API base URL")
		domain := fs.String("domain", "", "Horizon domain")
		_ = fs.Parse(args[1:])
		cfg, err := LoadConfig()
		if err != nil {
			return 1, err
		}
		ApplyFlagOverrides(cfg, &FlagOverrides{APIBaseURL: *api, Domain: *domain})
		if err := revalidateConfig(cfg); err != nil {
			return 1, err
		}
		c, err := newClient(cfg)
		if err != nil {
			return 1, err
		}
		return 0, cmdWorkflowList(ctx, c, *runningOnly, *limit, *cont, *phase, *out)

	case "get":
		fs := flag.NewFlagSet("workflow get", flag.ExitOnError)
		api := fs.String("api", "", "Horizon API base URL")
		domain := fs.String("domain", "", "Horizon domain")
		_ = fs.Parse(args[1:])
		rem := fs.Args()
		if len(rem) < 1 {
			return 2, fmt.Errorf("usage: %s workflow get <workflowName> (same as show --output json)", progName())
		}
		cfg, err := LoadConfig()
		if err != nil {
			return 1, err
		}
		ApplyFlagOverrides(cfg, &FlagOverrides{APIBaseURL: *api, Domain: *domain})
		if err := revalidateConfig(cfg); err != nil {
			return 1, err
		}
		c, err := newClient(cfg)
		if err != nil {
			return 1, err
		}
		return 0, cmdWorkflowGet(ctx, c, rem[0])

	case "running":
		fs := flag.NewFlagSet("workflow running", flag.ExitOnError)
		limit := fs.Int("limit", 50, "list limit")
		api := fs.String("api", "", "Horizon API base URL")
		domain := fs.String("domain", "", "Horizon domain")
		_ = fs.Parse(args[1:])
		cfg, err := LoadConfig()
		if err != nil {
			return 1, err
		}
		ApplyFlagOverrides(cfg, &FlagOverrides{APIBaseURL: *api, Domain: *domain})
		if err := revalidateConfig(cfg); err != nil {
			return 1, err
		}
		c, err := newClient(cfg)
		if err != nil {
			return 1, err
		}
		return 0, cmdWorkflowRunning(ctx, c, *limit)

	case "history":
		fs := flag.NewFlagSet("workflow history", flag.ExitOnError)
		limit := fs.Int("limit", 50, "list limit")
		cont := fs.String("continue", "", "continue token")
		phase := fs.String("phase", "", "phase filter (comma-separated)")
		api := fs.String("api", "", "Horizon API base URL")
		domain := fs.String("domain", "", "Horizon domain")
		_ = fs.Parse(args[1:])
		cfg, err := LoadConfig()
		if err != nil {
			return 1, err
		}
		ApplyFlagOverrides(cfg, &FlagOverrides{APIBaseURL: *api, Domain: *domain})
		if err := revalidateConfig(cfg); err != nil {
			return 1, err
		}
		c, err := newClient(cfg)
		if err != nil {
			return 1, err
		}
		return 0, cmdWorkflowHistory(ctx, c, *limit, *cont, *phase)

	case "download-artifact":
		fs := flag.NewFlagSet("workflow download-artifact", flag.ExitOnError)
		genURL := fs.Bool("generate-signed-url", false, "print signed-url: <URL> on stdout (or --output json); do not download bytes")
		dur := fs.Int("duration", 0, "optional signed URL lifetime in seconds (maps to durationSeconds query; server clamps)")
		templateName := fs.String("template-name", "", "disambiguate artifact (maps to templateName query; see workflow show outputArtifacts)")
		nodeID := fs.String("node-id", "", "disambiguate artifact by node ID from 409 candidates list (maps to nodeId query; see workflow show outputArtifacts)")
		outPath := fs.String("o", "", "write downloaded bytes to this path")
		toStdout := fs.Bool("stdout", false, "write downloaded bytes to stdout")
		quiet := fs.Bool("q", false, "when downloading: no progress on stderr")
		outFmt := fs.String("output", "text", "with --generate-signed-url: text (signed-url line) or json")
		api := fs.String("api", "", "Horizon API base URL")
		domain := fs.String("domain", "", "Horizon domain")
		_ = fs.Parse(args[1:])
		rem := fs.Args()
		if len(rem) < 2 {
			return 2, fmt.Errorf("usage: %s workflow download-artifact [flags] <workflowName> <artifactName>", progName())
		}
		cfg, err := LoadConfig()
		if err != nil {
			return 1, err
		}
		ApplyFlagOverrides(cfg, &FlagOverrides{APIBaseURL: *api, Domain: *domain})
		if err := revalidateConfig(cfg); err != nil {
			return 1, err
		}
		c, err := newClient(cfg)
		if err != nil {
			return 1, err
		}
		err = cmdWorkflowDownloadArtifact(ctx, c, rem[0], rem[1], *genURL, *dur, *templateName, *nodeID, *outPath, *toStdout, *outFmt, *quiet)
		if err != nil {
			return 1, err
		}
		return 0, nil

	case "poll":
		return 2, fmt.Errorf("workflow poll was removed; use: %s workflow wait <workflowName> (name is printed by submit)", progName())

	case "finalize":
		return 2, fmt.Errorf("workflow finalize was removed; use: %s workflow wait <workflowName> and/or %s workflow show <workflowName>", progName(), progName())

	default:
		return 2, fmt.Errorf("unknown workflow subcommand %q", args[0])
	}
}
