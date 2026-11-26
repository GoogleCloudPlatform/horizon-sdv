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

package artifactregistry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"golang.org/x/oauth2"
)

const testRepoPath = "proj/horizon-sdv/cloud-ws-workloads/cws-workflow"

func TestParseRef(t *testing.T) {
	ok := Ref{Host: "us-central1-docker.pkg.dev", Path: testRepoPath, Tag: "latest"}
	cases := []struct {
		image   string
		want    Ref
		wantErr bool
	}{
		{image: "us-central1-docker.pkg.dev/" + testRepoPath + ":latest", want: ok},
		{image: "us-central1-docker.pkg.dev/" + testRepoPath, wantErr: true},
		{image: "us-central1-docker.pkg.dev/" + testRepoPath + ":", wantErr: true},
		{image: "us-central1-docker.pkg.dev/" + testRepoPath + "@sha256:abc", wantErr: true},
		{image: ":latest", wantErr: true},
		{image: "", wantErr: true},
	}
	for _, tc := range cases {
		got, err := ParseRef(tc.image)
		if (err != nil) != tc.wantErr {
			t.Fatalf("ParseRef(%q) error = %v, wantErr %v", tc.image, err, tc.wantErr)
		}
		if got != tc.want {
			t.Fatalf("ParseRef(%q) = %+v, want %+v", tc.image, got, tc.want)
		}
	}
}

func TestChecker_Exists(t *testing.T) {
	cases := []struct {
		name    string
		status  int
		want    bool
		wantErr bool
	}{
		{name: "present", status: http.StatusOK, want: true},
		{name: "missing", status: http.StatusNotFound, want: false},
		{name: "denied is uncertain", status: http.StatusForbidden, wantErr: true},
		{name: "unauthorized is uncertain", status: http.StatusUnauthorized, wantErr: true},
		{name: "server error is uncertain", status: http.StatusInternalServerError, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodHead {
					t.Errorf("method = %s, want HEAD", r.Method)
				}
				if r.URL.Path != "/v2/"+testRepoPath+"/manifests/latest" {
					t.Errorf("path = %s", r.URL.Path)
				}
				user, pass, ok := r.BasicAuth()
				if !ok || user != "oauth2accesstoken" || pass != "test-token" {
					t.Errorf("basic auth = %q %q %v", user, pass, ok)
				}
				w.WriteHeader(tc.status)
			}))
			defer srv.Close()

			c := &Checker{
				client: srv.Client(),
				tokens: oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "test-token"}),
			}
			image := strings.TrimPrefix(srv.URL, "https://") + "/" + testRepoPath + ":latest"
			got, err := c.Exists(context.Background(), image)
			if (err != nil) != tc.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, tc.wantErr)
			}
			if got != tc.want {
				t.Fatalf("exists = %v, want %v", got, tc.want)
			}
		})
	}
}
