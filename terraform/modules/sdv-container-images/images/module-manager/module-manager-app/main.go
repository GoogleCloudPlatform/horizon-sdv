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

package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/discovery"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	horizonv1alpha1 "github.com/acn-horizon-sdv/module-manager/api/v1alpha1"
	"github.com/acn-horizon-sdv/module-manager/internal/api"
	"github.com/acn-horizon-sdv/module-manager/internal/controller"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(horizonv1alpha1.AddToScheme(scheme))
}

func main() {
	var (
		metricsAddr        string
		probeAddr          string
		apiAddr            string
		namespace          string
		argocdNamespace    string
		stateCRName        string
		catalogCRName      string
		argocdProject      string
		repoURL            string
		targetRevision     string
		gitopsRootAppNames string
	)
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "Address for metrics.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "Address for health probes.")
	flag.StringVar(&apiAddr, "api-bind-address", ":8082", "Address for REST API.")
	flag.StringVar(&namespace, "namespace", "module-manager", "Namespace for Module Manager and state.")
	flag.StringVar(&argocdNamespace, "argocd-namespace", "argocd", "Namespace where ArgoCD Applications are created.")
	flag.StringVar(&stateCRName, "state-cr-name", "cluster", "Name of the singleton ModuleManagerState CR for persisted state.")
	flag.StringVar(&catalogCRName, "catalog-cr-name", "cluster", "Name of the singleton ModuleCatalog CR for the module catalog.")
	flag.StringVar(&argocdProject, "argocd-project", "horizon-sdv", "ArgoCD AppProject for module Applications.")
	flag.StringVar(&repoURL, "repo-url", "", "Git repo URL for module charts (override from catalog).")
	flag.StringVar(&targetRevision, "target-revision", "HEAD", "Git target revision (e.g. HEAD, main).")
	flag.StringVar(&gitopsRootAppNames, "gitops-root-app-name", "",
		"Comma-separated names of root Argo CD app-of-apps Applications (e.g. prefix+horizon-sdv). Falls back to GITOPS_ROOT_APP_NAME when empty.")
	flag.Parse()

	crlog.SetLogger(zap.New(zap.UseDevMode(false)))

	if repoURL == "" {
		repoURL = os.Getenv("REPO_URL")
	}
	if repoURL == "" {
		log.Fatal("repo-url or REPO_URL must be set")
	}

	moduleConfig := os.Getenv("MODULE_CONFIG")

	rootAppNameList := splitCommaTrim(gitopsRootAppNames)
	if len(rootAppNameList) == 0 {
		rootAppNameList = splitCommaTrim(os.Getenv("GITOPS_ROOT_APP_NAME"))
	}

	ctx := ctrl.SetupSignalHandler()

	cfg := ctrl.GetConfigOrDie()

	// Restrict the informer cache per resource type so that each resource type is
	// only listed/watched in the namespace where it actually lives.  Using a blanket
	// DefaultNamespaces that covers both namespaces causes controller-runtime to try
	// to list every registered type in every namespace, which triggers RBAC denials
	// (e.g. Applications in module-manager) and causes the cache to time out and the
	// manager to crash.
	argoAppObj := &unstructured.Unstructured{}
	argoAppObj.SetGroupVersionKind(schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "Application"})

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsserver.Options{BindAddress: metricsAddr},
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         false,
		Cache: cache.Options{
			// Default: custom CRDs (ModuleManagerState, ModuleCatalog) live only in module-manager namespace.
			DefaultNamespaces: map[string]cache.Config{
				namespace: {},
			},
			// Override for ArgoCD Applications: watched only in the argocd namespace.
			ByObject: map[client.Object]cache.ByObject{
				argoAppObj: {
					Namespaces: map[string]cache.Config{
						argocdNamespace: {},
					},
				},
			},
		},
	})
	if err != nil {
		log.Fatalf("creating manager: %v", err)
	}

	// State and catalog live in the same namespace as the controller.
	stateStore := controller.NewStateStoreCR(mgr.GetClient(), namespace, stateCRName)
	catalogStore := controller.NewCatalogStoreCR(mgr.GetClient(), namespace, catalogCRName)

	apiReader := mgr.GetAPIReader()

	// ModuleCatalog is the authoritative desired-state source; ModuleManagerState holds
	// runtime state. The REST API handler and ModuleCatalogReconciler build Argo
	// Applications deterministically from catalog entries (see controller.ApplicationName).

	if err := mgr.Add(&moduleHelmStartup{
		client:       mgr.GetClient(),
		apiReader:    apiReader,
		argocdNS:     argocdNamespace,
		repoURL:      repoURL,
		defaultRev:   targetRevision,
		moduleConfig: moduleConfig,
		stateStore:   stateStore,
	}); err != nil {
		log.Fatalf("module argo startup sync: %v", err)
	}

	if err := mgr.Add(&controller.TargetRevisionStartupSync{
		Client:          mgr.GetClient(),
		APIReader:       apiReader,
		ArgoNS:          argocdNamespace,
		DefaultRevision: targetRevision,
		StateStore:      stateStore,
	}); err != nil {
		log.Fatalf("target-revision startup sync: %v", err)
	}

	catalogReconciler := &controller.ModuleCatalogReconciler{
		Client:          mgr.GetClient(),
		APIReader:       apiReader,
		Namespace:       namespace,
		ArgoCDNamespace: argocdNamespace,
		StateStore:      stateStore,
		CatalogStore:    catalogStore,
	}
	if err = catalogReconciler.SetupWithManager(mgr); err != nil {
		log.Fatalf("setting up ModuleCatalog reconciler: %v", err)
	}

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		log.Fatalf("creating discovery client: %v", err)
	}

	drainer := &controller.PlatformDrainer{
		Client:          mgr.GetClient(),
		APIReader:       apiReader,
		DiscoveryClient: discoveryClient,
		StateStore:      stateStore,
		CatalogStore:    catalogStore,
		Namespace:       namespace,
		ArgoCDNamespace: argocdNamespace,
		ArgoCDProject:   argocdProject,
	}

	rootFin := &controller.RootGitOpsApplicationReconciler{
		Client:             mgr.GetClient(),
		Drainer:            drainer,
		ArgoCDNamespace:    argocdNamespace,
		GitOpsRootAppNames: rootAppNameList,
	}
	if err := rootFin.SetupWithManager(mgr); err != nil {
		log.Fatalf("setting up root GitOps Application finalizer reconciler: %v", err)
	}

	if err := mgr.AddHealthzCheck("health", healthz.Ping); err != nil {
		log.Fatalf("adding health check: %v", err)
	}
	if err := mgr.AddReadyzCheck("ready", healthz.Ping); err != nil {
		log.Fatalf("adding ready check: %v", err)
	}

	// REST API server (runs in goroutine, uses the same client and stores).
	apiHandler := api.NewHandler(mgr.GetClient(), apiReader, stateStore, catalogStore, namespace, argocdNamespace, argocdProject, repoURL, targetRevision, moduleConfig, rootAppNameList)
	srv := &http.Server{Addr: apiAddr, Handler: apiHandler.Routes(), ReadHeaderTimeout: 10 * time.Second}

	go func() {
		log.Printf("REST API listening on %s", apiAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("API server error: %v", err)
		}
	}()

	if err := mgr.Start(ctx); err != nil {
		log.Printf("manager stopped: %v", err)
	}
	_ = srv.Shutdown(context.Background())
}

func splitCommaTrim(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(s, ",") {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
