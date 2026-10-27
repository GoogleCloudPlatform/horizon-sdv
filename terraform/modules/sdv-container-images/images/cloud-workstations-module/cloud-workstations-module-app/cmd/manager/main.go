// Copyright (c) 2026 Accenture, All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	wsv1alpha1 "cloud-workstations-module/api/v1alpha1"
	"cloud-workstations-module/internal/controller"
	"cloud-workstations-module/internal/wsapi"
)

func main() {
	ctrl.SetLogger(zap.New(zap.UseDevMode(false)))

	managerNS := os.Getenv("MANAGER_NAMESPACE")
	if managerNS == "" {
		managerNS = "cloud-workstations"
	}
	leaderNS := os.Getenv("POD_NAMESPACE")
	if leaderNS == "" {
		leaderNS = managerNS
	}
	hostServiceAccount := os.Getenv("HOST_SERVICE_ACCOUNT")
	if hostServiceAccount == "" {
		panic("HOST_SERVICE_ACCOUNT is required")
	}

	scheme := runtime.NewScheme()
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(wsv1alpha1.AddToScheme(scheme))

	resync := 5 * time.Minute
	if raw := os.Getenv("API_RESYNC_INTERVAL"); raw != "" {
		parsed, err := time.ParseDuration(raw)
		if err != nil {
			panic(fmt.Errorf("API_RESYNC_INTERVAL: %w", err))
		}
		resync = parsed
	}
	wsClient, err := wsapi.New(context.Background())
	if err != nil {
		panic(fmt.Errorf("workstations client: %w", err))
	}
	defer func() { _ = wsClient.Close() }()

	cfg, err := ctrl.GetConfig()
	if err != nil {
		panic(fmt.Errorf("kubeconfig: %w", err))
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
		HealthProbeBindAddress:  ":8080",
		LeaderElection:          true,
		LeaderElectionID:        "cloud-workstations-module.workstations.horizon-sdv.io",
		LeaderElectionNamespace: leaderNS,
	})
	if err != nil {
		panic(fmt.Errorf("unable to create manager: %w", err))
	}

	// Health and readiness checks for kubelet probes on :8080
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		panic(fmt.Errorf("unable to set up health check: %w", err))
	}
	if err := mgr.AddReadyzCheck("readyz", func(_ *http.Request) error { return nil }); err != nil {
		panic(fmt.Errorf("unable to set up ready check: %w", err))
	}

	// Register WorkstationCluster controller
	if err := (&controller.WorkstationClusterReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		panic(fmt.Errorf("unable to create WorkstationCluster controller: %w", err))
	}

	// Register WorkstationConfig controller
	if err := (&controller.WorkstationConfigReconciler{
		Client:             mgr.GetClient(),
		Scheme:             mgr.GetScheme(),
		WS:                 wsClient,
		ResyncInterval:     resync,
		HostServiceAccount: hostServiceAccount,
	}).SetupWithManager(mgr); err != nil {
		panic(fmt.Errorf("unable to create WorkstationConfig controller: %w", err))
	}

	// Register Workstation controller
	if err := (&controller.WorkstationReconciler{
		Client:         mgr.GetClient(),
		Scheme:         mgr.GetScheme(),
		WS:             wsClient,
		ResyncInterval: resync,
	}).SetupWithManager(mgr); err != nil {
		panic(fmt.Errorf("unable to create Workstation controller: %w", err))
	}

	ctrl.Log.Info("starting Cloud Workstations Module controller manager", "namespace", managerNS)
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		panic(fmt.Errorf("problem running manager: %w", err))
	}
}
