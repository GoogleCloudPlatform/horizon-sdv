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
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestArtifactsObjectsFromModuleUsesObjectAttributes(t *testing.T) {
	t.Parallel()
	const raw = `{
		"count": 1,
		"objects": [{
			"objectPath": "builds/image.tar",
			"gsUri": "gs://bucket/builds/image.tar",
			"size": 4096,
			"contentType": "application/x-tar",
			"metadata": {"workflow": "aaos-build"},
			"updated": "2026-10-05T12:00:00Z"
		}]
	}`
	var resp struct {
		Objects []artifactsModuleObject `json:"objects"`
	}
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatal(err)
	}
	got := artifactsObjectsFromModule(resp.Objects)
	if len(got) != 1 {
		t.Fatalf("objects: got %d want 1", len(got))
	}
	obj := got[0]
	if obj.Name != "builds/image.tar" || obj.Size != 4096 || obj.ContentType != "application/x-tar" || obj.Updated != "2026-10-05T12:00:00Z" {
		t.Fatalf("mapped object: %+v", obj)
	}
}

func TestHandleArtifactsListObjectsProxyTimeout(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Outlast the client deadline, then return so the test server can shut down.
		time.Sleep(time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(upstream.Close)

	prev := artifactsModuleProxyTimeout
	artifactsModuleProxyTimeout = 200 * time.Millisecond
	t.Cleanup(func() { artifactsModuleProxyTimeout = prev })

	s := &Server{opt: Options{
		ArtifactsModuleURL:            upstream.URL,
		ArtifactsModuleToken:          "test-token",
		ArtifactsStaticAllowedBuckets: map[string]struct{}{"my-bucket": {}},
	}}
	req := httptest.NewRequest(http.MethodGet, "/v1/artifacts/buckets/my-bucket/objects", nil)
	req.SetPathValue("bucket", "my-bucket")
	rr := httptest.NewRecorder()

	s.handleArtifactsListObjects(rr, req, nil)

	if rr.Code != http.StatusGatewayTimeout {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
}

func TestHandleArtifactsListObjectsAllowlist(t *testing.T) {
	t.Parallel()
	var calls int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != http.MethodGet {
			t.Errorf("method %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("authorization %q", r.Header.Get("Authorization"))
		}
		if !strings.HasSuffix(r.URL.Path, "/objects-list") {
			t.Errorf("path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"count":0,"objects":[]}`))
	}))
	t.Cleanup(upstream.Close)

	cr := &unstructured.Unstructured{}
	cr.SetAPIVersion("gcsmanager.horizon.io/v1alpha1")
	cr.SetKind("GCSBucket")
	cr.SetNamespace("horizon")
	cr.SetName("from-cr")
	if err := unstructured.SetNestedField(cr.Object, "proj-from-cr", "spec", "bucketName"); err != nil {
		t.Fatal(err)
	}
	other := &unstructured.Unstructured{}
	other.SetAPIVersion("gcsmanager.horizon.io/v1alpha1")
	other.SetKind("GCSBucket")
	other.SetNamespace("horizon")
	other.SetName("other")
	if err := unstructured.SetNestedField(other.Object, "other-from-cr", "spec", "bucketName"); err != nil {
		t.Fatal(err)
	}
	wrongNS := &unstructured.Unstructured{}
	wrongNS.SetAPIVersion("gcsmanager.horizon.io/v1alpha1")
	wrongNS.SetKind("GCSBucket")
	wrongNS.SetNamespace("module-manager")
	wrongNS.SetName("wrong-ns")
	if err := unstructured.SetNestedField(wrongNS.Object, "proj-wrong-ns", "spec", "bucketName"); err != nil {
		t.Fatal(err)
	}

	s := &Server{opt: Options{
		ArtifactsModuleURL:    upstream.URL,
		ArtifactsModuleToken:  "test-token",
		ArtifactsGCSProjectID: "proj",
		ArtifactsStaticAllowedBuckets: map[string]struct{}{
			"proj-static":  {},
			"other-static": {},
		},
		K8sClient:    fake.NewClientBuilder().WithObjects(cr, other, wrongNS).Build(),
		GCSBucketsNS: "horizon",
	}}

	listReq := httptest.NewRequest(http.MethodGet, "/v1/artifacts/buckets", nil)
	listRR := httptest.NewRecorder()
	s.handleArtifactsListBuckets(listRR, listReq, nil)
	if listRR.Code != http.StatusOK {
		t.Fatalf("list status %d body %s", listRR.Code, listRR.Body.String())
	}
	var listed artifactsListBucketsResponse
	if err := json.Unmarshal(listRR.Body.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	gotNames := make([]string, 0, len(listed.Buckets))
	for _, b := range listed.Buckets {
		gotNames = append(gotNames, b.Name)
	}
	if strings.Join(gotNames, ",") != "proj-from-cr,proj-static" {
		t.Fatalf("buckets: got %v", gotNames)
	}

	denied := httptest.NewRequest(http.MethodGet, "/v1/artifacts/buckets/other-static/objects", nil)
	denied.SetPathValue("bucket", "other-static")
	deniedRR := httptest.NewRecorder()
	s.handleArtifactsListObjects(deniedRR, denied, nil)
	if deniedRR.Code != http.StatusForbidden {
		t.Fatalf("denied status %d body %s", deniedRR.Code, deniedRR.Body.String())
	}

	injected := httptest.NewRequest(http.MethodGet, "/v1/artifacts/buckets/proj-static/objects", nil)
	injected.SetPathValue("bucket", "proj-static/../other")
	injectedRR := httptest.NewRecorder()
	s.handleArtifactsListObjects(injectedRR, injected, nil)
	if injectedRR.Code != http.StatusBadRequest {
		t.Fatalf("invalid status %d body %s", injectedRR.Code, injectedRR.Body.String())
	}

	allowed := httptest.NewRequest(http.MethodGet, "/v1/artifacts/buckets/proj-from-cr/objects", nil)
	allowed.SetPathValue("bucket", "proj-from-cr")
	allowedRR := httptest.NewRecorder()
	s.handleArtifactsListObjects(allowedRR, allowed, nil)
	if allowedRR.Code != http.StatusOK {
		t.Fatalf("allowed status %d body %s", allowedRR.Code, allowedRR.Body.String())
	}
	if calls != 1 {
		t.Fatalf("upstream calls: got %d want 1", calls)
	}

	ok, err := s.artifactsBucketAllowed(context.Background(), "other-from-cr")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("CR bucket without project prefix was allowed")
	}
	ok, err = s.artifactsBucketAllowed(context.Background(), "proj-wrong-ns")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("GCSBucket in module-manager was allowed")
	}
}

func TestHandleArtifactsListObjectsPrefixPage(t *testing.T) {
	t.Parallel()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("authorization %q", r.Header.Get("Authorization"))
		}
		if r.URL.Query().Get("prefix") != "builds" {
			t.Errorf("prefix %q", r.URL.Query().Get("prefix"))
		}
		if r.URL.Query().Get("pageToken") != "tok-1" {
			t.Errorf("pageToken %q", r.URL.Query().Get("pageToken"))
		}
		if r.URL.Query().Get("pageSize") != "2" {
			t.Errorf("pageSize %q", r.URL.Query().Get("pageSize"))
		}
		if r.Body != nil {
			b, _ := io.ReadAll(r.Body)
			if len(b) != 0 {
				t.Errorf("body %s", b)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"count":2,"nextPageToken":"tok-2","objects":[
			{"objectPath":"builds/a","size":1,"contentType":"text/plain","updated":"2026-10-05T12:00:00Z"},
			{"objectPath":"builds/b","size":2}
		]}`))
	}))
	t.Cleanup(upstream.Close)

	s := &Server{opt: Options{
		ArtifactsModuleURL:            upstream.URL,
		ArtifactsModuleToken:          "test-token",
		ArtifactsStaticAllowedBuckets: map[string]struct{}{"my-bucket": {}},
	}}
	req := httptest.NewRequest(http.MethodGet, "/v1/artifacts/buckets/my-bucket/objects?prefix=builds&pageToken=tok-1&limit=2", nil)
	req.SetPathValue("bucket", "my-bucket")
	rr := httptest.NewRecorder()
	s.handleArtifactsListObjects(rr, req, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
	var got artifactsListObjectsResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.NextPageToken != "tok-2" {
		t.Fatalf("nextPageToken %q", got.NextPageToken)
	}
	if len(got.Objects) != 2 || got.Objects[0].Name != "builds/a" || got.Objects[1].Name != "builds/b" {
		t.Fatalf("objects %+v", got.Objects)
	}
}

func TestHandleArtifactsListObjectsEmptyPrefixAndLimit(t *testing.T) {
	t.Parallel()
	var calls int
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Query().Get("prefix") != "" {
			t.Errorf("prefix %q", r.URL.Query().Get("prefix"))
		}
		if r.URL.Query().Get("pageSize") != "1000" {
			t.Errorf("pageSize %q", r.URL.Query().Get("pageSize"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"count":0,"objects":[]}`))
	}))
	t.Cleanup(upstream.Close)

	s := &Server{opt: Options{
		ArtifactsModuleURL:            upstream.URL,
		ArtifactsModuleToken:          "test-token",
		ArtifactsStaticAllowedBuckets: map[string]struct{}{"my-bucket": {}},
	}}
	req := httptest.NewRequest(http.MethodGet, "/v1/artifacts/buckets/my-bucket/objects", nil)
	req.SetPathValue("bucket", "my-bucket")
	rr := httptest.NewRecorder()
	s.handleArtifactsListObjects(rr, req, nil)
	if rr.Code != http.StatusOK {
		t.Fatalf("status %d body %s", rr.Code, rr.Body.String())
	}
	if calls != 1 {
		t.Fatalf("upstream calls: got %d want 1", calls)
	}

	bad := httptest.NewRequest(http.MethodGet, "/v1/artifacts/buckets/my-bucket/objects?limit=0", nil)
	bad.SetPathValue("bucket", "my-bucket")
	badRR := httptest.NewRecorder()
	s.handleArtifactsListObjects(badRR, bad, nil)
	if badRR.Code != http.StatusBadRequest {
		t.Fatalf("limit status %d body %s", badRR.Code, badRR.Body.String())
	}
	if calls != 1 {
		t.Fatalf("invalid limit called upstream: calls=%d", calls)
	}

	s.opt.ArtifactsModuleToken = ""
	noTok := httptest.NewRequest(http.MethodGet, "/v1/artifacts/buckets/my-bucket/objects", nil)
	noTok.SetPathValue("bucket", "my-bucket")
	noTokRR := httptest.NewRecorder()
	s.handleArtifactsListObjects(noTokRR, noTok, nil)
	if noTokRR.Code != http.StatusServiceUnavailable {
		t.Fatalf("missing token status %d body %s", noTokRR.Code, noTokRR.Body.String())
	}
	if calls != 1 {
		t.Fatalf("missing token called upstream: calls=%d", calls)
	}
}

func TestHandleArtifactsListObjectsTokenFile(t *testing.T) {
	dir := t.TempDir()
	tokenPath := filepath.Join(dir, "horizon-api")

	var calls int
	var gotAuth string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"count":0,"objects":[]}`))
	}))
	t.Cleanup(upstream.Close)

	s := &Server{opt: Options{
		ArtifactsModuleURL:            upstream.URL,
		ArtifactsModuleToken:          "stale-env-token",
		ArtifactsModuleTokenFile:      tokenPath,
		ArtifactsStaticAllowedBuckets: map[string]struct{}{"my-bucket": {}},
	}}
	call := func() *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/v1/artifacts/buckets/my-bucket/objects", nil)
		req.SetPathValue("bucket", "my-bucket")
		rr := httptest.NewRecorder()
		s.handleArtifactsListObjects(rr, req, nil)
		return rr
	}

	// Secret not created yet: do not fall back to a token frozen in the environment.
	missing := call()
	if missing.Code != http.StatusServiceUnavailable {
		t.Fatalf("missing file status %d body %s", missing.Code, missing.Body.String())
	}
	if calls != 0 {
		t.Fatalf("missing token file called upstream: calls=%d", calls)
	}

	if err := os.WriteFile(tokenPath, []byte("  \n"), 0o600); err != nil {
		t.Fatal(err)
	}
	blank := call()
	if blank.Code != http.StatusServiceUnavailable {
		t.Fatalf("blank token status %d body %s", blank.Code, blank.Body.String())
	}
	if calls != 0 {
		t.Fatalf("blank token called upstream: calls=%d", calls)
	}

	if err := os.WriteFile(tokenPath, []byte("fresh-token\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ok := call()
	if ok.Code != http.StatusOK {
		t.Fatalf("file token status %d body %s", ok.Code, ok.Body.String())
	}
	if gotAuth != "Bearer fresh-token" {
		t.Fatalf("authorization %q", gotAuth)
	}

	// Same process, secret rotated: the next request must send the new token.
	if err := os.WriteFile(tokenPath, []byte("rotated-token\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	rotated := call()
	if rotated.Code != http.StatusOK {
		t.Fatalf("rotated status %d body %s", rotated.Code, rotated.Body.String())
	}
	if gotAuth != "Bearer rotated-token" {
		t.Fatalf("rotated authorization %q", gotAuth)
	}
	if calls != 2 {
		t.Fatalf("upstream calls: got %d want 2", calls)
	}

	s.opt.ArtifactsModuleTokenFile = dir
	unreadable := call()
	if unreadable.Code != http.StatusBadGateway {
		t.Fatalf("unreadable token status %d body %s", unreadable.Code, unreadable.Body.String())
	}
	if calls != 2 {
		t.Fatalf("unreadable token called upstream: calls=%d", calls)
	}
}

func TestArtifactsModuleTokenRetriesProjectedVolume(t *testing.T) {
	dir := t.TempDir()
	// Presence of ..data is what kubelet uses to publish the current secret data.
	// A regular file is enough for the check; the test does not need a symlink.
	if err := os.WriteFile(filepath.Join(dir, "..data"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	tokenPath := filepath.Join(dir, "horizon-api")

	prevRead := readArtifactsTokenFile
	prevWait := artifactsTokenReadRetryWait
	var reads int
	readArtifactsTokenFile = func(name string) ([]byte, error) {
		if name != tokenPath {
			t.Errorf("read %s", name)
		}
		reads++
		if reads < 3 {
			return nil, os.ErrNotExist
		}
		return []byte("after-swap\n"), nil
	}
	artifactsTokenReadRetryWait = 0
	t.Cleanup(func() {
		readArtifactsTokenFile = prevRead
		artifactsTokenReadRetryWait = prevWait
	})

	s := &Server{opt: Options{
		ArtifactsModuleTokenFile: tokenPath,
		ArtifactsModuleToken:     "stale-env-token",
	}}
	tok, err := s.artifactsModuleToken()
	if err != nil {
		t.Fatal(err)
	}
	if tok != "after-swap" {
		t.Fatalf("token %q", tok)
	}
	if reads != 3 {
		t.Fatalf("reads: got %d want 3", reads)
	}
}
