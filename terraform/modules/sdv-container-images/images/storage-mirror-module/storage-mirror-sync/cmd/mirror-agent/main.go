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

// Command mirror-agent runs one Storage Mirror operation inside an Argo workflow pod:
//
//	mirror-agent create | sync | delete | init-volume
//
// Configuration comes from the SM_* environment set by the WorkflowTemplates.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"storage-mirror-sync/internal/mirror"
	"storage-mirror-sync/internal/report"
	"storage-mirror-sync/internal/runner"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: mirror-agent create|sync|delete|init-volume")
		os.Exit(2)
	}
	cfg := mirror.FromEnv(os.Getenv)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	rep := &report.Reporter{
		ControllerURL:    cfg.ControllerURL,
		Token:            cfg.ControllerToken,
		RunID:            cfg.RunID,
		Mirror:           cfg.Mirror,
		ArgoProgressFile: cfg.ArgoProgressFile,
		HTTP:             report.NewHTTPClient(cfg.ControllerCAFile),
	}
	if cfg.Directory != "" && mirror.ValidateDirectory(cfg.Directory) == nil {
		rep.SnapshotFile = filepath.Join(cfg.Root(), ".storage-mirror", "progress", cfg.Directory+".json")
	}
	a := &mirror.Agent{Cfg: cfg, Rep: rep, Run: runner.Run}

	var err error
	switch os.Args[1] {
	case "create":
		err = a.Create(ctx)
	case "sync":
		err = a.Sync(ctx)
	case "delete":
		err = a.Delete(ctx)
	case "init-volume":
		err = a.InitVolume(ctx)
	default:
		err = fmt.Errorf("unknown command %q", os.Args[1])
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "[storage-mirror] %s failed: %s\n", os.Args[1], runner.Mask(err.Error()))
		os.Exit(1)
	}
	fmt.Printf("[storage-mirror] %s succeeded\n", os.Args[1])
}
