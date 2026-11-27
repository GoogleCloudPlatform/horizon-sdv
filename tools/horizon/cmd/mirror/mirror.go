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

// Package mirror implements "horizon mirror": Storage Mirror management through the Horizon API.
package mirror

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	horizon "github.com/horizon-sdv/horizon/internal"
	"github.com/spf13/cobra"
)

// NewMirrorCmd returns the mirror command group.
func NewMirrorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "mirror",
		Aliases: []string{"storage-mirror"},
		Short:   "Manage AOSP repo and git mirrors (storage-mirror module)",
		Long:    "Create, list, sync and delete mirrors on the shared mirror volume, and follow sync progress. Needs the storage-mirror module enabled.",
	}
	horizon.AddAPIFlags(cmd)
	cmd.AddCommand(listCmd(), getCmd(), createCmd(), updateCmd(), syncCmd(), statusCmd(), runsCmd(), cancelCmd(), deleteCmd(), importCmd(), volumeCmd(), configCmd())
	return cmd
}

func client() (*horizon.Client, error) {
	c, _, err := horizon.LoadAPIClient()
	return c, err
}

func listCmd() *cobra.Command {
	var kind, phase string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List mirrors (managed and catalog-only)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			return horizon.CmdMirrorList(cmd.Context(), c, kind, phase, cmd.OutOrStdout())
		},
	}
	cmd.Flags().StringVar(&kind, "kind", "", "repo or git")
	cmd.Flags().StringVar(&phase, "phase", "", "Pending, Initializing, Syncing, Ready, Error or Deleting")
	return cmd
}

func getCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get NAME",
		Short: "Show one mirror (spec, status, catalog entry)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			b, err := c.MirrorDo(cmd.Context(), http.MethodGet, horizon.MirrorPath(args[0], ""), nil)
			if err != nil {
				return err
			}
			return horizon.CmdMirrorPrint(cmd.OutOrStdout(), b, nil)
		},
	}
}

func createCmd() *cobra.Command {
	var (
		kind, directory, manifestURL, manifestRef, manifestFile string
		gitURL, credentials, lfs, schedule, timeout             string
		branches                                                []string
		jobs                                                    int32
		wait                                                    bool
		interval                                                time.Duration
	)
	cmd := &cobra.Command{
		Use:   "create NAME",
		Short: "Create a mirror and start its first sync",
		Example: "  horizon mirror create aosp-16 --kind repo --manifest-ref android-16.0.0_r3\n" +
			"  horizon mirror create kernel --kind git --url https://android.googlesource.com/kernel/common --branches android-mainline --watch",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			body := map[string]interface{}{"name": args[0], "kind": kind}
			if directory != "" {
				body["directory"] = directory
			}
			switch kind {
			case "repo":
				repo := map[string]interface{}{}
				setIf(repo, "manifestUrl", manifestURL)
				setIf(repo, "manifestRef", manifestRef)
				setIf(repo, "manifestFile", manifestFile)
				if jobs > 0 {
					repo["syncJobs"] = jobs
				}
				body["repo"] = repo
			case "git":
				if gitURL == "" {
					return fmt.Errorf("--url is required for --kind git")
				}
				git := map[string]interface{}{"url": gitURL}
				if len(branches) > 0 {
					git["branches"] = branches
				}
				setIf(git, "credentialsSecretName", credentials)
				body["git"] = git
			default:
				return fmt.Errorf("--kind must be repo or git")
			}
			setIf(body, "lfs", lfs)
			setIf(body, "schedule", schedule)
			setIf(body, "syncTimeout", timeout)
			c, err := client()
			if err != nil {
				return err
			}
			b, err := c.MirrorDo(cmd.Context(), http.MethodPost, "/mirrors", body)
			if err != nil {
				return err
			}
			if err := horizon.CmdMirrorPrint(cmd.OutOrStdout(), b, func(m map[string]interface{}) string {
				return fmt.Sprintf("mirror %v created (request %v)", m["name"], m["requestId"])
			}); err != nil {
				return err
			}
			if wait {
				return horizon.CmdMirrorStatus(cmd.Context(), c, args[0], true, interval, cmd.OutOrStdout())
			}
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&kind, "kind", "repo", "repo (AOSP manifest mirror) or git (single repository)")
	f.StringVar(&directory, "directory", "", "Directory on the volume (default: NAME)")
	f.StringVar(&manifestURL, "manifest-url", "", "repo: manifest repository URL (default from the module)")
	f.StringVar(&manifestRef, "manifest-ref", "", "repo: manifest branch or tag")
	f.StringVar(&manifestFile, "manifest-file", "", "repo: manifest file")
	f.Int32Var(&jobs, "sync-jobs", 0, "repo: parallel repo sync jobs")
	f.StringVar(&gitURL, "url", "", "git: https URL of the repository")
	f.StringSliceVar(&branches, "branches", nil, "git: branch patterns (default all)")
	f.StringVar(&credentials, "credentials-secret", "", "git: basic-auth Secret name in the workflows namespace")
	f.StringVar(&lfs, "lfs", "", "inherit, disabled, enabled or auto")
	f.StringVar(&schedule, "schedule", "", "inherit, none or a cron expression")
	f.StringVar(&timeout, "sync-timeout", "", "Maximum fetch time, e.g. 20h")
	f.BoolVar(&wait, "watch", false, "Follow progress until the first sync finishes")
	f.DurationVar(&interval, "interval", 5*time.Second, "Poll interval for --watch")
	return cmd
}

func setIf(m map[string]interface{}, k, v string) {
	if v != "" {
		m[k] = v
	}
}

func updateCmd() *cobra.Command {
	var manifestRef, manifestFile, lfs, schedule, timeout string
	var branches []string
	var jobs int32
	var suspend, resume bool
	cmd := &cobra.Command{
		Use:   "update NAME",
		Short: "Change mutable mirror settings",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			body := map[string]interface{}{}
			f := cmd.Flags()
			if f.Changed("manifest-ref") {
				body["manifestRef"] = manifestRef
			}
			if f.Changed("manifest-file") {
				body["manifestFile"] = manifestFile
			}
			if f.Changed("sync-jobs") {
				body["syncJobs"] = jobs
			}
			if f.Changed("branches") {
				body["branches"] = branches
			}
			if f.Changed("lfs") {
				body["lfs"] = lfs
			}
			if f.Changed("schedule") {
				body["schedule"] = schedule
			}
			if f.Changed("sync-timeout") {
				body["syncTimeout"] = timeout
			}
			if suspend && resume {
				return fmt.Errorf("--suspend and --resume are exclusive")
			}
			if suspend || resume {
				body["suspend"] = suspend
			}
			if len(body) == 0 {
				return fmt.Errorf("nothing to update")
			}
			c, err := client()
			if err != nil {
				return err
			}
			b, err := c.MirrorDo(cmd.Context(), http.MethodPatch, horizon.MirrorPath(args[0], ""), body)
			if err != nil {
				return err
			}
			return horizon.CmdMirrorPrint(cmd.OutOrStdout(), b, func(map[string]interface{}) string { return "mirror " + args[0] + " updated" })
		},
	}
	f := cmd.Flags()
	f.StringVar(&manifestRef, "manifest-ref", "", "repo: manifest branch or tag")
	f.StringVar(&manifestFile, "manifest-file", "", "repo: manifest file")
	f.Int32Var(&jobs, "sync-jobs", 0, "repo: parallel repo sync jobs")
	f.StringSliceVar(&branches, "branches", nil, "git: branch patterns")
	f.StringVar(&lfs, "lfs", "", "inherit, disabled, enabled or auto")
	f.StringVar(&schedule, "schedule", "", "inherit, none or a cron expression")
	f.StringVar(&timeout, "sync-timeout", "", "Maximum fetch time, e.g. 20h")
	f.BoolVar(&suspend, "suspend", false, "Stop scheduled and requested syncs")
	f.BoolVar(&resume, "resume", false, "Resume a suspended mirror")
	return cmd
}

func syncCmd() *cobra.Command {
	var all, watch bool
	var interval time.Duration
	cmd := &cobra.Command{
		Use:   "sync [NAME...]",
		Short: "Sync one or more mirrors (or --all)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if all == (len(args) > 0) {
				return fmt.Errorf("give mirror names or --all")
			}
			c, err := client()
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			var b []byte
			if len(args) == 1 {
				b, err = c.MirrorDo(ctx, http.MethodPost, horizon.MirrorPath(args[0], "/sync"), nil)
			} else {
				body := map[string]interface{}{"all": all}
				if !all {
					body["mirrors"] = args
				}
				b, err = c.MirrorDo(ctx, http.MethodPost, "/syncs", body)
			}
			if err != nil {
				return err
			}
			if err := horizon.CmdMirrorPrint(cmd.OutOrStdout(), b, nil); err != nil {
				return err
			}
			if watch {
				for _, n := range args {
					if err := horizon.CmdMirrorStatus(ctx, c, n, true, interval, cmd.OutOrStdout()); err != nil {
						return err
					}
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "Sync every mirror that is not suspended")
	cmd.Flags().BoolVar(&watch, "watch", false, "Follow progress of the named mirrors until done")
	cmd.Flags().DurationVar(&interval, "interval", 5*time.Second, "Poll interval for --watch")
	return cmd
}

func statusCmd() *cobra.Command {
	var watch bool
	var interval time.Duration
	cmd := &cobra.Command{
		Use:   "status NAME",
		Short: "Show sync status and live progress",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			return horizon.CmdMirrorStatus(cmd.Context(), c, args[0], watch, interval, cmd.OutOrStdout())
		},
	}
	cmd.Flags().BoolVarP(&watch, "watch", "w", false, "Poll until no run is active; exit non-zero if it failed")
	cmd.Flags().DurationVar(&interval, "interval", 5*time.Second, "Poll interval")
	return cmd
}

func runsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "runs NAME",
		Short: "List recent runs (Workflow names; logs via horizon workflow logs)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			return horizon.CmdMirrorRuns(cmd.Context(), c, args[0], cmd.OutOrStdout())
		},
	}
}

func cancelCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cancel NAME",
		Short: "Stop the active run of a mirror",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			b, err := c.MirrorDo(cmd.Context(), http.MethodPost, horizon.MirrorPath(args[0], "/sync/cancel"), nil)
			if err != nil {
				return err
			}
			return horizon.CmdMirrorPrint(cmd.OutOrStdout(), b, nil)
		},
	}
}

func deleteCmd() *cobra.Command {
	var purge, yes bool
	cmd := &cobra.Command{
		Use:   "delete NAME",
		Short: "Unregister a mirror (keeps data); --purge also deletes its data",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			path := horizon.MirrorPath(name, "")
			if purge {
				if !yes {
					ok, err := confirm(cmd, fmt.Sprintf("This deletes all data of mirror %q. Type the mirror name to confirm: ", name), name)
					if err != nil {
						return err
					}
					if !ok {
						return fmt.Errorf("aborted")
					}
				}
				path += "?purge=true&confirm=" + url.QueryEscape(name)
			}
			c, err := client()
			if err != nil {
				return err
			}
			b, err := c.MirrorDo(cmd.Context(), http.MethodDelete, path, nil)
			if err != nil {
				return err
			}
			return horizon.CmdMirrorPrint(cmd.OutOrStdout(), b, func(map[string]interface{}) string {
				if purge {
					return "mirror " + name + ": delete run requested (data will be removed)"
				}
				return "mirror " + name + " unregistered; its data stays on the volume (horizon mirror import to manage it again)"
			})
		},
	}
	cmd.Flags().BoolVar(&purge, "purge", false, "Also delete the mirror directory and its catalog entry")
	cmd.Flags().BoolVar(&yes, "yes", false, "Do not ask for confirmation with --purge")
	return cmd
}

func confirm(cmd *cobra.Command, prompt, want string) (bool, error) {
	fmt.Fprint(cmd.ErrOrStderr(), prompt)
	in := cmd.InOrStdin()
	if in == nil {
		in = os.Stdin
	}
	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && line == "" {
		return false, err
	}
	return strings.TrimSpace(line) == want, nil
}

func importCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "import [DIRECTORY...]",
		Short: "Manage catalog-only mirrors (for example created by the Jenkins jobs)",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := client()
			if err != nil {
				return err
			}
			body := map[string]interface{}{}
			if len(args) > 0 {
				body["mirrors"] = args
			}
			b, err := c.MirrorDo(cmd.Context(), http.MethodPost, "/import", body)
			if err != nil {
				return err
			}
			return horizon.CmdMirrorPrint(cmd.OutOrStdout(), b, nil)
		},
	}
}

func volumeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "volume",
		Short: "Show the mirror volume (Filestore instance, capacity, usage)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return getAndPrint(cmd, "/volume")
		},
	}
}

func configCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "config",
		Short: "Show the effective StorageMirrorConfig (read-only; Git is the writer)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return getAndPrint(cmd, "/config")
		},
	}
}

func getAndPrint(cmd *cobra.Command, path string) error {
	c, err := client()
	if err != nil {
		return err
	}
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	b, err := c.MirrorDo(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
	return horizon.CmdMirrorPrint(cmd.OutOrStdout(), b, nil)
}
