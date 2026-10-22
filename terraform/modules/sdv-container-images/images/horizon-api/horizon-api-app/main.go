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
	"flag"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	crlog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/acn-horizon-sdv/horizon-api/internal/api"
	"github.com/acn-horizon-sdv/horizon-api/internal/argo"
	"github.com/acn-horizon-sdv/horizon-api/internal/auth"
	"github.com/acn-horizon-sdv/horizon-api/internal/catalog"
	"github.com/acn-horizon-sdv/horizon-api/internal/controller"
	"github.com/acn-horizon-sdv/horizon-api/internal/workflow"
)

var scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
}

func main() {
	var (
		metricsAddr             string
		probeAddr               string
		httpAddr                string
		horizonNamespace        string
		moduleManagerNS         string
		gcsBucketsNS            string
		workflowsNS             string
		stateCRName             string
		catalogCRName           string
		oidcIssuerURL           string
		oidcClientID            string
		skipClientIDCheck       bool
		eventsWebhookURL        string
		argoBaseURL             string
		logReadIdle             time.Duration
		logMaxReconnect         int
		gcsArtifactBucket       string
		ttlAfterSuccess         int
		ttlAfterFailure         int
		ttlAfterCompletion      int
		gcsSigningSA            string
		workflowDeleteWait      time.Duration
		artifactsModuleURL      string
		artifactsGCSProjectID   string
		artifactsAllowedBuckets string
	)
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "Prometheus metrics")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "Health probes")
	flag.StringVar(&httpAddr, "http-bind-address", ":8082", "HTTP API")
	flag.StringVar(&horizonNamespace, "namespace", "horizon-api", "Namespace for this deployment")
	flag.StringVar(&moduleManagerNS, "module-manager-namespace", "module-manager", "Namespace for Module Manager CRs")
	flag.StringVar(&gcsBucketsNS, "gcs-buckets-namespace", "horizon", "Namespace for GCSBucket CRs (storage-gcs-module MANAGER_NAMESPACE)")
	flag.StringVar(&workflowsNS, "workflows-namespace", "", "Namespace for WorkflowTemplate / Workflow (defaults to {prefix}workflows from WORKFLOWS_NAMESPACE env)")
	flag.StringVar(&stateCRName, "module-manager-state-name", "cluster", "ModuleManagerState resource name")
	flag.StringVar(&catalogCRName, "module-manager-catalog-name", "cluster", "ModuleCatalog resource name in module-manager namespace")
	flag.StringVar(&oidcIssuerURL, "oidc-issuer-url", "", "Keycloak realm issuer, e.g. https://domain/auth/realms/horizon")
	flag.StringVar(&oidcClientID, "oidc-client-id", "", "Expected access-token audience / client (optional)")
	flag.BoolVar(&skipClientIDCheck, "oidc-skip-client-id-check", true, "Skip OIDC azp/aud client check (needed for some Keycloak access tokens)")
	flag.StringVar(&eventsWebhookURL, "events-webhook-url", "", "Argo Events EventSource POST URL (required, e.g. .../events/workflow)")
	flag.StringVar(&argoBaseURL, "argo-base-url", "", "Argo Server base URL with base href (e.g. http://argo-workflows-server.NS.svc:2746/workflows); ARGO_BASE_URL")
	flag.DurationVar(&logReadIdle, "log-read-idle-deadline", 30*time.Second, "Emit heartbeat if Argo sends no log line within this duration")
	flag.IntVar(&logMaxReconnect, "log-max-reconnect", 25, "Max reconnect attempts when opening per-pod log streams (pods often 404 until the container is ready)")
	flag.StringVar(&gcsArtifactBucket, "gcs-artifact-bucket", "", "Default GCS bucket for gs:// archived log links when artifact keys omit bucket; GCS_ARTIFACT_BUCKET")
	flag.IntVar(&ttlAfterSuccess, "workflow-ttl-seconds-after-success", 86400, "Echoed on workflow list APIs; mirror Argo ttlStrategy")
	flag.IntVar(&ttlAfterFailure, "workflow-ttl-seconds-after-failure", 259200, "Echoed on workflow list APIs; mirror Argo ttlStrategy")
	flag.IntVar(&ttlAfterCompletion, "workflow-ttl-seconds-after-completion", 86400, "Echoed on workflow list APIs; mirror Argo ttlStrategy")
	flag.StringVar(&gcsSigningSA, "gcs-signing-service-account", "", "Service account email for V4 signed GET URLs (artifact download); GCS_SIGNING_SERVICE_ACCOUNT")
	flag.DurationVar(&workflowDeleteWait, "workflow-delete-wait-timeout", 10*time.Minute, "max time DELETE /v1/workflows/{name} waits for the Workflow CR to disappear (finalizers); WORKFLOW_DELETE_WAIT_TIMEOUT env overrides")
	flag.StringVar(&artifactsModuleURL, "artifacts-module-url", "", "Storage GCS module HTTP endpoint for bucket/object listing proxy; ARTIFACTS_MODULE_URL")
	flag.StringVar(&artifactsGCSProjectID, "artifacts-gcs-project-id", "", "GCS project ID for bucket prefix filtering; ARTIFACTS_GCS_PROJECT_ID")
	flag.StringVar(&artifactsAllowedBuckets, "artifacts-allowed-buckets", "", "Comma-separated static bucket allowlist; ARTIFACTS_ALLOWED_BUCKETS")
	flag.Parse()

	crlog.SetLogger(zap.New(zap.UseDevMode(false)))

	if workflowsNS == "" {
		workflowsNS = os.Getenv("WORKFLOWS_NAMESPACE")
	}
	if workflowsNS == "" {
		log.Fatal("workflows-namespace or WORKFLOWS_NAMESPACE must be set")
	}
	if oidcIssuerURL == "" {
		oidcIssuerURL = os.Getenv("OIDC_ISSUER_URL")
	}
	if oidcIssuerURL == "" {
		log.Fatal("oidc-issuer-url or OIDC_ISSUER_URL must be set")
	}
	if eventsWebhookURL == "" {
		eventsWebhookURL = os.Getenv("EVENTS_WEBHOOK_URL")
	}
	if eventsWebhookURL == "" {
		log.Fatal("events-webhook-url or EVENTS_WEBHOOK_URL is required (Argo Events dispatch only)")
	}
	if argoBaseURL == "" {
		argoBaseURL = os.Getenv("ARGO_BASE_URL")
	}
	if strings.TrimSpace(gcsArtifactBucket) == "" {
		gcsArtifactBucket = strings.TrimSpace(os.Getenv("GCS_ARTIFACT_BUCKET"))
	}
	if strings.TrimSpace(gcsSigningSA) == "" {
		gcsSigningSA = strings.TrimSpace(os.Getenv("GCS_SIGNING_SERVICE_ACCOUNT"))
	}
	if v := strings.TrimSpace(os.Getenv("WORKFLOW_TTL_SECONDS_AFTER_SUCCESS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			ttlAfterSuccess = n
		}
	}
	if v := strings.TrimSpace(os.Getenv("WORKFLOW_TTL_SECONDS_AFTER_FAILURE")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			ttlAfterFailure = n
		}
	}
	if v := strings.TrimSpace(os.Getenv("WORKFLOW_TTL_SECONDS_AFTER_COMPLETION")); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			ttlAfterCompletion = n
		}
	}
	if v := strings.TrimSpace(os.Getenv("LOG_MAX_RECONNECT")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			logMaxReconnect = n
		}
	}
	if v := strings.TrimSpace(os.Getenv("WORKFLOW_DELETE_WAIT_TIMEOUT")); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			workflowDeleteWait = d
		}
	}
	if strings.TrimSpace(artifactsModuleURL) == "" {
		artifactsModuleURL = strings.TrimSpace(os.Getenv("ARTIFACTS_MODULE_URL"))
	}
	if strings.TrimSpace(artifactsGCSProjectID) == "" {
		artifactsGCSProjectID = strings.TrimSpace(os.Getenv("ARTIFACTS_GCS_PROJECT_ID"))
	}
	if strings.TrimSpace(artifactsAllowedBuckets) == "" {
		artifactsAllowedBuckets = strings.TrimSpace(os.Getenv("ARTIFACTS_ALLOWED_BUCKETS"))
	}

	// Parse static allowed buckets
	staticAllowedBuckets := make(map[string]struct{})
	if artifactsAllowedBuckets != "" {
		for _, bucket := range strings.Split(artifactsAllowedBuckets, ",") {
			if bucket = strings.TrimSpace(bucket); bucket != "" {
				staticAllowedBuckets[bucket] = struct{}{}
			}
		}
	}

	var argoClient *argo.Client
	if strings.TrimSpace(argoBaseURL) != "" {
		argoClient = &argo.Client{
			BaseURL:    strings.TrimSpace(argoBaseURL),
			HTTPClient: argo.NewHTTPClient(),
		}
	}

	ctx := ctrl.SetupSignalHandler()
	authz, err := auth.NewOIDC(ctx, oidcIssuerURL, oidcClientID, skipClientIDCheck)
	if err != nil {
		log.Fatalf("oidc: %v", err)
	}

	cfg := ctrl.GetConfigOrDie()
	wfStore, err := workflow.NewStore(cfg, workflowsNS)
	if err != nil {
		log.Fatalf("workflow store: %v", err)
	}

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: metricsAddr,
		},
		HealthProbeBindAddress: probeAddr,
	})
	if err != nil {
		log.Fatalf("manager: %v", err)
	}
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		log.Fatal(err)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		log.Fatal(err)
	}

	k8s, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		log.Fatalf("client: %v", err)
	}

	cat := catalog.New(moduleManagerNS, workflowsNS, stateCRName, catalogCRName)

	cat.InjectClient(k8s)
	rec := &controller.CatalogReconciler{
		Catalog:         cat,
		ModuleManagerNS: moduleManagerNS,
		WorkflowsNS:     workflowsNS,
		StateName:       stateCRName,
		CatalogName:     catalogCRName,
	}
	if err := rec.SetupWithManager(mgr); err != nil {
		log.Fatalf("catalog controller: %v", err)
	}

	retention := workflow.Retention{
		SecondsAfterSuccess:    ttlAfterSuccess,
		SecondsAfterFailure:    ttlAfterFailure,
		SecondsAfterCompletion: ttlAfterCompletion,
		Explanation:            "Workflow CRs are removed from the cluster after Argo ttlStrategy; expect 404 when pruned.",
	}

	handler := api.New(api.Options{
		Catalog:                       cat,
		OIDC:                          authz,
		WorkflowsNS:                   workflowsNS,
		HTTPAddr:                      httpAddr,
		EventsWebhookURL:              eventsWebhookURL,
		Argo:                          argoClient,
		LogReadIdle:                   logReadIdle,
		LogMaxReconnect:               logMaxReconnect,
		WorkflowStore:                 wfStore,
		GCSArtifactBucket:             gcsArtifactBucket,
		GCSSigningServiceAccount:      gcsSigningSA,
		Retention:                     retention,
		WorkflowDeleteWaitTimeout:     workflowDeleteWait,
		K8sClient:                     k8s,
		ModuleManagerNS:               moduleManagerNS,
		GCSBucketsNS:                  gcsBucketsNS,
		StateCRName:                   stateCRName,
		CatalogCRName:                 catalogCRName,
		ArtifactsModuleURL:            artifactsModuleURL,
		ArtifactsModuleToken:          strings.TrimSpace(os.Getenv("ARTIFACTS_MODULE_TOKEN")),
		ArtifactsModuleCAFile:         strings.TrimSpace(os.Getenv("ARTIFACTS_MODULE_CA_FILE")),
		ArtifactsGCSProjectID:         artifactsGCSProjectID,
		ArtifactsStaticAllowedBuckets: staticAllowedBuckets,
	})
	if err := mgr.Add(handler); err != nil {
		log.Fatalf("http: %v", err)
	}

	log.Printf("Horizon API listening on %s (workflows=%s moduleManager=%s)", httpAddr, workflowsNS, moduleManagerNS)
	if err := mgr.Start(ctx); err != nil {
		log.Fatalf("start: %v", err)
	}
}
