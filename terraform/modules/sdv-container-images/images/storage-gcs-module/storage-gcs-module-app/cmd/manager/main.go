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
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	gcsv1alpha1 "storage-gcs-module/api/v1alpha1"
	"storage-gcs-module/internal/api"
	"storage-gcs-module/internal/controller"

	"cloud.google.com/go/storage"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
)

func main() {
	ctrl.SetLogger(zap.New(zap.UseDevMode(false)))

	managerNS := os.Getenv("MANAGER_NAMESPACE")
	if managerNS == "" {
		managerNS = "horizon"
	}
	defaultBucket := os.Getenv("DEFAULT_ARTIFACT_BUCKET")
	gcpProjectID := os.Getenv("GOOGLE_CLOUD_PROJECT")
	leaderNS := os.Getenv("POD_NAMESPACE")
	if leaderNS == "" {
		leaderNS = managerNS
	}

	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(gcsv1alpha1.AddToScheme(scheme))

	cfg, err := ctrl.GetConfig()
	if err != nil {
		panic(fmt.Errorf("kubeconfig: %w", err))
	}

	gcsClient, err := storage.NewClient(context.Background())
	if err != nil {
		panic(fmt.Errorf("gcs client: %w", err))
	}

	signingEmail := os.Getenv("GCS_SIGNED_URL_SERVICE_ACCOUNT")
	if signingEmail == "" && gcpProjectID != "" {
		signingEmail = fmt.Sprintf("gke-storage-gcs-module-sa@%s.iam.gserviceaccount.com", gcpProjectID)
	}
	defaultExp := parseDurationEnv("GCS_SIGNED_URL_DEFAULT_EXPIRY", 48*time.Hour)
	maxExp := parseDurationEnv("GCS_SIGNED_URL_MAX_EXPIRY", 168*time.Hour)
	apiToken := strings.TrimSpace(os.Getenv("API_BEARER_TOKEN"))
	apiTokens, err := api.ParseAPITokens(os.Getenv("API_TOKENS"), apiToken)
	if err != nil {
		panic(fmt.Errorf("api tokens: %w", err))
	}
	ctrl.Log.Info("api auth tokens loaded", "count", len(apiTokens))
	staticBuckets := parseAllowedBuckets(os.Getenv("ALLOWED_BUCKETS"))
	if defaultBucket != "" {
		staticBuckets[defaultBucket] = struct{}{}
	}

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme,
		Cache: cache.Options{
			DefaultNamespaces: map[string]cache.Config{
				managerNS: {},
			},
		},
		Metrics: metricsserver.Options{
			BindAddress: "0",
		},
		HealthProbeBindAddress:  "0",
		LeaderElection:          true,
		LeaderElectionID:        "storage-gcs-module.gcsmanager.horizon.io",
		LeaderElectionNamespace: leaderNS,
	})
	if err != nil {
		panic(fmt.Errorf("manager: %w", err))
	}

	srv := &api.Server{
		K8s:                        mgr.GetClient(),
		GCS:                        gcsClient,
		ManagerNamespace:           managerNS,
		DefaultBucket:              defaultBucket,
		APIBearerToken:             apiToken,
		APITokens:                  apiTokens,
		StaticAllowedBuckets:       staticBuckets,
		ProjectID:                  gcpProjectID,
		SigningServiceAccountEmail: signingEmail,
		SignedURLDefaultExpiry:     defaultExp,
		SignedURLMaxExpiry:         maxExp,
	}
	listenCfg := api.ListenConfigFromEnv()
	healthSrv, apiSrv, err := api.StartHTTPServers(srv, listenCfg)
	if err != nil {
		panic(fmt.Errorf("http listen config: %w", err))
	}
	go func() {
		if err := healthSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			panic(fmt.Errorf("http health: %w", err))
		}
	}()
	if apiSrv != nil {
		go func() {
			if err := apiSrv.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
				panic(fmt.Errorf("https api: %w", err))
			}
		}()
		ctrl.Log.Info("storage-gcs api tls enabled", "addr", listenCfg.TLSAddr)
	} else {
		ctrl.Log.Info("storage-gcs api plain http", "addr", listenCfg.HTTPAddr)
	}

	if err := (&controller.GCSBucketReconciler{
		Client:    mgr.GetClient(),
		GCS:       gcsClient,
		ProjectID: gcpProjectID,
	}).SetupWithManager(mgr); err != nil {
		panic(fmt.Errorf("gcsbucket controller: %w", err))
	}

	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		panic(fmt.Errorf("manager start: %w", err))
	}
}

func parseDurationEnv(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return def
	}
	return d
}

func parseAllowedBuckets(csv string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, part := range strings.Split(csv, ",") {
		b := strings.TrimSpace(part)
		if b != "" {
			out[b] = struct{}{}
		}
	}
	return out
}
