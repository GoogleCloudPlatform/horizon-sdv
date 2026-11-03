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

package api

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
)

// HealthHandler serves GET /health only (kubelet probes over plain HTTP).
func (s *Server) HealthHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	return mux
}

// ListenConfig from API_TLS_* env (cert/key required together).
type ListenConfig struct {
	TLSCertFile string
	TLSKeyFile  string
	TLSAddr     string
	HTTPAddr    string
}

func ListenConfigFromEnv() ListenConfig {
	addr := os.Getenv("API_TLS_ADDR")
	if addr == "" {
		addr = ":8443"
	}
	httpAddr := os.Getenv("API_HTTP_ADDR")
	if httpAddr == "" {
		httpAddr = ":8080"
	}
	return ListenConfig{
		TLSCertFile: os.Getenv("API_TLS_CERT_FILE"),
		TLSKeyFile:  os.Getenv("API_TLS_KEY_FILE"),
		TLSAddr:     addr,
		HTTPAddr:    httpAddr,
	}
}

func (c ListenConfig) TLSEnabled() bool {
	return c.TLSCertFile != "" && c.TLSKeyFile != ""
}

func (c ListenConfig) buildTLSConfig() (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(c.TLSCertFile, c.TLSKeyFile)
	if err != nil {
		return nil, fmt.Errorf("load api tls cert: %w", err)
	}
	cfg := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}
	if caPath := os.Getenv("API_TLS_CLIENT_CA_FILE"); caPath != "" {
		caPEM, err := os.ReadFile(caPath)
		if err != nil {
			return nil, fmt.Errorf("read client ca: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caPEM) {
			return nil, fmt.Errorf("parse client ca")
		}
		cfg.ClientCAs = pool
		cfg.ClientAuth = tls.RequireAndVerifyClientCert
	}
	return cfg, nil
}

// StartHTTPServers listens on plain HTTP for /health and, when TLS is configured, HTTPS for the full API.
func StartHTTPServers(srv *Server, cfg ListenConfig) (health *http.Server, api *http.Server, err error) {
	health = &http.Server{Addr: cfg.HTTPAddr, Handler: srv.HealthHandler()}
	if !cfg.TLSEnabled() {
		health.Handler = srv.Handler()
		return health, nil, nil
	}
	tlsCfg, err := cfg.buildTLSConfig()
	if err != nil {
		return nil, nil, err
	}
	api = &http.Server{
		Addr:      cfg.TLSAddr,
		Handler:   srv.Handler(),
		TLSConfig: tlsCfg,
	}
	return health, api, nil
}
