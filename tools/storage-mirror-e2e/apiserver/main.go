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

// Command apiserver starts a throwaway kube-apiserver and etcd (controller-runtime envtest) on 127.0.0.1 with the
// given CRD directories installed, writes an admin kubeconfig and runs until SIGTERM. Used by run.sh
// --apiserver-only; it needs KUBEBUILDER_ASSETS (setup-envtest) and no container runtime.
//
//	apiserver KUBECONFIG_OUT CRD_DIR...
package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: apiserver KUBECONFIG_OUT CRD_DIR...")
		os.Exit(2)
	}
	env := &envtest.Environment{CRDDirectoryPaths: os.Args[2:], ErrorIfCRDPathMissing: true}
	cfg, err := env.Start()
	if err != nil {
		fmt.Fprintln(os.Stderr, "start:", err)
		os.Exit(1)
	}
	defer func() { _ = env.Stop() }()
	user, err := env.AddUser(envtest.User{Name: "e2e-admin", Groups: []string{"system:masters"}}, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "user:", err)
		return
	}
	kc, err := user.KubeConfig()
	if err == nil {
		err = os.WriteFile(os.Args[1], kc, 0o600)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "kubeconfig:", err)
		return
	}
	fmt.Println("ready", cfg.Host)
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGTERM, syscall.SIGINT)
	<-sig
}
