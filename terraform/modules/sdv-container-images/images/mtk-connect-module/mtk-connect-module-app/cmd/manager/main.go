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

package main

import (
	"net/http"
	"os"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"mtk-connect-module/internal/controller"
	"mtk-connect-module/internal/kexec"
	"mtk-connect-module/internal/keycloak"
	"mtk-connect-module/internal/metrics"
	"mtk-connect-module/internal/mtk"
)

func main() {
	ctrl.SetLogger(zap.New(zap.UseDevMode(false)))
	setupLog := ctrl.Log.WithName("setup")
	// fatal logs a startup failure the same way as every other line, then exits.
	fatal := func(msg string, err error) {
		setupLog.Error(err, msg)
		os.Exit(1)
	}
	managerNS := os.Getenv("MANAGER_NAMESPACE")

	if managerNS == "" {
		managerNS = "mtk-connect"
	}
	leaderNS := os.Getenv("POD_NAMESPACE")
	if leaderNS == "" {
		leaderNS = managerNS
	}
	metricsAddr := envOr("METRICS_BIND_ADDRESS", ":8080")
	probeAddr := envOr("HEALTH_PROBE_BIND_ADDRESS", ":8081")

	// MTKConnectTestbench resources are watched in these namespaces only (comma separated); the
	// operator has a Role in each of them. Without it, only the module namespace is watched.
	testbenchNamespaces := splitList(os.Getenv("WATCH_NAMESPACES"))
	if len(testbenchNamespaces) == 0 {
		testbenchNamespaces = []string{managerNS}
	}
	testbenchNS := map[string]cache.Config{}
	for _, ns := range testbenchNamespaces {
		testbenchNS[ns] = cache.Config{}
	}
	testbenchObj := &unstructured.Unstructured{}
	testbenchObj.SetGroupVersionKind(controller.TestbenchGVK)
	deviceObj := &unstructured.Unstructured{}
	deviceObj.SetGroupVersionKind(controller.DeviceGVK)

	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	cfg, err := ctrl.GetConfig()
	if err != nil {
		fatal("kubeconfig", err)
	}

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme,
		// The MTKConnectInstance is cached in the module namespace and MTKConnectTestbench in the watched
		// namespaces; Secrets, Pods and Deployments in several namespaces are read through the direct
		// client below.
		Cache: cache.Options{
			DefaultNamespaces: map[string]cache.Config{managerNS: {}},
			ByObject: map[client.Object]cache.ByObject{
				testbenchObj: {Namespaces: testbenchNS},
				deviceObj:    {Namespaces: testbenchNS},
			},
		},
		Metrics:                 metricsserver.Options{BindAddress: metricsAddr},
		HealthProbeBindAddress:  probeAddr,
		LeaderElection:          true,
		LeaderElectionID:        "mtk-connect-module.mtkconnect.horizon-sdv.io",
		LeaderElectionNamespace: leaderNS,
	})
	if err != nil {
		fatal("manager", err)
	}

	direct, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		fatal("direct client", err)
	}

	mtkHTTP := &http.Client{Timeout: 30 * time.Second, Transport: metrics.Instrument(nil)}
	kcHTTP := &http.Client{Timeout: 30 * time.Second}

	if err := (&controller.InstanceReconciler{
		Client:       mgr.GetClient(),
		Direct:       direct,
		Exec:         &kexec.Executor{Config: cfg, Timeout: 2 * time.Minute},
		InstanceName: os.Getenv("INSTANCE_NAME"),
		NewKeycloak: func(baseURL string) controller.KeycloakAPI {
			return keycloak.New(baseURL, kcHTTP)
		},
		NewMTK: func(serverURL, username, key string) controller.MTKAPI {
			return mtk.New(serverURL, username, key, mtkHTTP)
		},
	}).SetupWithManager(mgr); err != nil {
		fatal("instance controller", err)
	}

	if err := (&controller.TestbenchReconciler{
		Client:           mgr.GetClient(),
		Direct:           direct,
		ManagerNamespace: managerNS,
		InstanceName:     os.Getenv("INSTANCE_NAME"),
		URLPath:          os.Getenv("TESTBENCH_URL_PATH"),
		WatchNamespaces:  testbenchNamespaces,
		NewAPI: func(serverURL, username, key string) controller.TestbenchAPI {
			return mtk.New(serverURL, username, key, mtkHTTP)
		},
	}).SetupWithManager(mgr); err != nil {
		fatal("testbench controller", err)
	}

	if err := (&controller.DeviceReconciler{
		Client:           mgr.GetClient(),
		Direct:           direct,
		ManagerNamespace: managerNS,
		InstanceName:     os.Getenv("INSTANCE_NAME"),
		NewAPI: func(serverURL, username, key string) controller.DeviceAPI {
			return mtk.New(serverURL, username, key, mtkHTTP)
		},
	}).SetupWithManager(mgr); err != nil {
		fatal("device controller", err)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		fatal("healthz", err)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		fatal("readyz", err)
	}

	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		fatal("manager start", err)
	}
}

func splitList(s string) []string {
	var out []string
	for _, v := range strings.Split(s, ",") {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
