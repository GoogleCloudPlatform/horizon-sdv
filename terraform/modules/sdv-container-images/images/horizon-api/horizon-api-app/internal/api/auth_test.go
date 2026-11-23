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

package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/acn-horizon-sdv/horizon-api/internal/catalog"
	"github.com/acn-horizon-sdv/horizonauth"
	"github.com/acn-horizon-sdv/horizonauth/authtest"
)

func TestEndpointsAcceptOnlyTokensMeantForHorizonAPI(t *testing.T) {
	iss := authtest.NewIssuer(t)
	v, err := horizonauth.NewVerifier(context.Background(), iss.URL(), "horizon-api")
	if err != nil {
		t.Fatal(err)
	}
	s := New(Options{Catalog: catalog.New("module-manager", "workflows", "cluster", "cluster"), Verifier: v})

	tests := []struct {
		name   string
		header string
		want   int
	}{
		{"no token", "", http.StatusUnauthorized},
		{"not a JWT", "Bearer opaque-token", http.StatusUnauthorized},
		{"portal viewer", "Bearer " + iss.Token(t, iss.Claims(nil)), http.StatusOK},
		{"CLI developer", "Bearer " + iss.Token(t, iss.Claims(map[string]any{"azp": "horizon-api", "resource_access": authtest.Roles("developer")})), http.StatusOK},
		{"CI service account", "Bearer " + iss.Token(t, iss.Claims(map[string]any{"azp": "horizon-api-ci", "resource_access": authtest.Roles("developer")})), http.StatusOK},
		{"token issued to grafana", "Bearer " + iss.Token(t, iss.Claims(map[string]any{"azp": "grafana", "aud": []string{"account"}})), http.StatusUnauthorized},
		{"ID token", "Bearer " + iss.Token(t, iss.Claims(map[string]any{"typ": "ID", "azp": "horizon-api"})), http.StatusUnauthorized},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/v1/catalog", nil)
			if tc.header != "" {
				req.Header.Set("Authorization", tc.header)
			}
			rec := httptest.NewRecorder()
			s.srv.Handler.ServeHTTP(rec, req)
			if rec.Code != tc.want {
				t.Errorf("GET /v1/catalog = %d, want %d", rec.Code, tc.want)
			}
		})
	}
}
