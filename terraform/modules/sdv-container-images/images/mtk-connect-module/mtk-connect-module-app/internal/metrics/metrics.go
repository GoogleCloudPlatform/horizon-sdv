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

// Package metrics holds the Prometheus metrics exposed by the MTK Connect controller.
package metrics

import (
	"net/http"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

var (
	// RotationFailures counts failed API key rotations.
	RotationFailures = prometheus.NewCounter(prometheus.CounterOpts{
		Name: "mtkc_rotation_failures_total",
		Help: "Number of failed MTK Connect API key rotations.",
	})

	// KeyExpirySeconds is the number of seconds until the current API key expires.
	KeyExpirySeconds = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "mtkc_key_expiry_seconds",
		Help: "Seconds until the current MTK Connect service-account API key expires.",
	})

	// Testbenches is the number of MTKConnectTestbench resources by phase.
	Testbenches = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: "mtkc_testbenches",
		Help: "Number of MTKConnectTestbench resources by phase.",
	}, []string{"phase"})

	// APIRequests counts MTK Connect REST requests by HTTP status code ("error" when no response).
	APIRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "mtkc_api_requests_total",
		Help: "Number of MTK Connect REST requests by HTTP status code.",
	}, []string{"code"})
)

func init() {
	ctrlmetrics.Registry.MustRegister(RotationFailures, KeyExpirySeconds, Testbenches, APIRequests)
}

type countingTransport struct {
	base http.RoundTripper
}

func (t countingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(req)
	code := "error"
	if err == nil {
		code = strconv.Itoa(resp.StatusCode)
	}
	APIRequests.WithLabelValues(code).Inc()
	return resp, err
}

// Instrument wraps base (http.DefaultTransport when nil) so every request increments APIRequests.
func Instrument(base http.RoundTripper) http.RoundTripper {
	if base == nil {
		base = http.DefaultTransport
	}
	return countingTransport{base: base}
}
