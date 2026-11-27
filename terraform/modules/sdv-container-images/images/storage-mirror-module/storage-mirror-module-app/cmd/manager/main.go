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

// Command manager runs the Storage Mirror controller and its REST API.
package main

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"os"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	smv1 "storage-mirror-module/api/v1alpha1"
	"storage-mirror-module/internal/api"
	"storage-mirror-module/internal/controller"
	"storage-mirror-module/internal/dispatch"
	"storage-mirror-module/internal/secrets"
	"storage-mirror-module/internal/state"
	"storage-mirror-module/internal/volume"
)

func main() {
	ctrl.SetLogger(zap.New(zap.UseDevMode(false)))
	ns := env("MANAGER_NAMESPACE", "horizon")
	wfNS := env("WORKFLOW_NAMESPACE", "workflows")
	jenkinsNS := env("JENKINS_NAMESPACE", "jenkins")
	projectID := os.Getenv("GOOGLE_CLOUD_PROJECT")
	webhook := os.Getenv("EVENTS_WEBHOOK_URL")
	ctx := ctrl.SetupSignalHandler()

	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(smv1.AddToScheme(scheme))

	cfg, err := ctrl.GetConfig()
	if err != nil {
		fail("kubeconfig", err)
	}
	direct, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		fail("client", err)
	}
	material, err := secrets.Ensure(ctx, direct, secrets.Config{
		Namespace: ns, WorkflowNamespace: wfNS, HorizonAPINamespace: env("HORIZON_API_NAMESPACE", "horizon-api"),
		TokenSecret: env("API_TOKEN_SECRET", "storage-mirror-module-api-token"), TLSSecret: env("API_TLS_SECRET", "storage-mirror-module-api-tls"),
		ServiceName: env("API_SERVICE_NAME", "storage-mirror-module"),
	})
	if err != nil {
		fail("api secrets", err)
	}
	tokens, err := api.ParseAPITokens(material.TokensJSON)
	if err != nil {
		fail("api tokens", err)
	}
	cert, err := tls.X509KeyPair(material.CertPEM, material.KeyPEM)
	if err != nil {
		fail("api certificate", err)
	}
	// Everything is cached in the controller namespace only, except Workflows (workflows namespace) and the
	// consumer claims, so list/watch RBAC outside the controller namespace stays limited to those two kinds (the API
	// Secrets above are read once, uncached).
	claimNamespaces := map[string]cache.Config{ns: {}, wfNS: {}}
	if jenkinsNS != "" {
		claimNamespaces[jenkinsNS] = cache.Config{}
	}
	wf := &unstructured.Unstructured{}
	wf.SetGroupVersionKind(controller.WorkflowGVK)
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme,
		Cache: cache.Options{
			DefaultNamespaces: map[string]cache.Config{ns: {}},
			ByObject: map[client.Object]cache.ByObject{
				&corev1.PersistentVolumeClaim{}: {Namespaces: claimNamespaces},
				wf:                              {Namespaces: map[string]cache.Config{wfNS: {}}},
			},
		},
		Client:                  client.Options{Cache: &client.CacheOptions{Unstructured: true}},
		Metrics:                 metricsserver.Options{BindAddress: "0"},
		HealthProbeBindAddress:  "0",
		LeaderElection:          true,
		LeaderElectionID:        "storage-mirror-module.storagemirror.horizon-sdv.io",
		LeaderElectionNamespace: env("POD_NAMESPACE", ns),
	})
	if err != nil {
		fail("manager", err)
	}

	d := &dispatch.Dispatcher{WebhookURL: webhook, WorkflowNamespace: wfNS}
	progress := state.NewStore()
	mirrorEvents := make(chan event.GenericEvent, 256)
	configEvents := make(chan event.GenericEvent, 16)

	var testNFS *volume.TestNFS
	if server := os.Getenv("TEST_NFS_SERVER"); server != "" {
		// kind end-to-end harness only (tools/storage-mirror-e2e): no Filestore, no Config Connector.
		testNFS = &volume.TestNFS{Server: server, Path: env("TEST_NFS_PATH", "/")}
		ctrl.Log.Info("TEST VOLUME DRIVER: plain NFS instead of Filestore", "server", server, "path", testNFS.Path)
	}
	if err := (&controller.ConfigReconciler{
		Client: mgr.GetClient(), Namespace: ns, WorkflowNamespace: wfNS, ProjectID: projectID,
		Dispatcher: d, Events: configEvents, TestNFS: testNFS,
	}).SetupWithManager(mgr); err != nil {
		fail("config controller", err)
	}
	if err := (&controller.MirrorReconciler{
		Client: mgr.GetClient(), Namespace: ns, WorkflowNamespace: wfNS,
		Dispatcher: d, Progress: progress, Events: mirrorEvents,
	}).SetupWithManager(mgr); err != nil {
		fail("mirror controller", err)
	}
	if err := mgr.Add(&controller.Scheduler{Client: mgr.GetClient(), Namespace: ns}); err != nil {
		fail("scheduler", err)
	}

	srv := &api.Server{
		Client: mgr.GetClient(), Reader: mgr.GetClient(), APIReader: mgr.GetAPIReader(), Namespace: ns, WorkflowNamespace: wfNS,
		Tokens: tokens, Progress: progress, MirrorEvents: mirrorEvents, ConfigEvents: configEvents,
	}
	health := &http.Server{Addr: env("API_HTTP_ADDR", ":8080"), Handler: api.HealthHandler(), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		if err := health.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fail("http health", err)
		}
	}()
	apiSrv := &http.Server{
		Addr:              env("API_TLS_ADDR", ":8443"),
		Handler:           srv.Handler(),
		TLSConfig:         &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{cert}},
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		if err := apiSrv.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
			fail("https api", err)
		}
	}()

	ctrl.Log.Info("storage-mirror-module starting", "namespace", ns, "workflows", wfNS, "tokens", len(tokens))
	if err := mgr.Start(ctx); err != nil {
		fail("manager start", err)
	}
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func fail(what string, err error) {
	fmt.Fprintf(os.Stderr, "%s: %v\n", what, err)
	os.Exit(1)
}
