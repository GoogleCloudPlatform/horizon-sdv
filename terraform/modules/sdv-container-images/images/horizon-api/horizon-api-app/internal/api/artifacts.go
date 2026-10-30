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
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/acn-horizon-sdv/horizon-api/internal/auth"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// errInvalidArtifactsBucket is returned when a bucket name cannot be a GCS bucket.
// Callers map it to 400. A well-formed name that is simply not allowlisted is
// (false, nil) and maps to 403.
var errInvalidArtifactsBucket = errors.New("invalid bucket name")

// readArtifactsTokenFile is os.ReadFile, replaced in tests.
var readArtifactsTokenFile = os.ReadFile

// artifactsTokenReadRetryWait is the pause between reads while kubelet is
// swapping a projected secret volume. Tests set it to zero.
var artifactsTokenReadRetryWait = 20 * time.Millisecond

// artifactsModuleProxyTimeout bounds the outbound call to storage-gcs-module.
// A zero http.Client timeout waits until the caller disconnects, so a stalled
// filter response holds the handler goroutine and the upstream connection.
// The deadline covers dial, response headers, and reading the JSON body, and
// still ends earlier if the inbound request context is canceled.
var artifactsModuleProxyTimeout = 60 * time.Second

// artifactsListMaxPageSize matches storage-gcs-module GET .../objects-list.
// GCS objects.list returns at most 1000 names per page.
const artifactsListMaxPageSize = 1000

// artifacts proxy types
type artifactsBucketInfo struct {
	Name string `json:"name"`
}

type artifactsListBucketsResponse struct {
	Buckets []artifactsBucketInfo `json:"buckets"`
}

type artifactsObjectInfo struct {
	Name        string `json:"name"`
	Size        int64  `json:"size"`
	ContentType string `json:"contentType,omitempty"`
	Updated     string `json:"updated,omitempty"`
	Generation  int64  `json:"generation,omitempty"`
	MD5Hash     string `json:"md5Hash,omitempty"`
}

type artifactsListObjectsResponse struct {
	Objects       []artifactsObjectInfo `json:"objects"`
	NextPageToken string                `json:"nextPageToken,omitempty"`
}

// artifactsModuleObject is one object from storage-gcs-module POST .../objects/filter.
// Size and ContentType are GCS object attributes. The metadata map holds only
// user-defined custom metadata and does not include those attributes.
type artifactsModuleObject struct {
	ObjectPath  string `json:"objectPath"`
	Size        int64  `json:"size"`
	ContentType string `json:"contentType,omitempty"`
	Updated     string `json:"updated,omitempty"`
}

func artifactsObjectsFromModule(objs []artifactsModuleObject) []artifactsObjectInfo {
	out := make([]artifactsObjectInfo, 0, len(objs))
	for _, obj := range objs {
		out = append(out, artifactsObjectInfo{
			Name:        obj.ObjectPath,
			Size:        obj.Size,
			ContentType: obj.ContentType,
			Updated:     obj.Updated,
		})
	}
	return out
}

// artifactsListPageSize is the storage page size for limit.
// An empty limit uses artifactsListMaxPageSize so the handler does not pull
// the whole bucket and then truncate.
func artifactsListPageSize(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return artifactsListMaxPageSize, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > artifactsListMaxPageSize {
		return 0, fmt.Errorf("limit must be between 1 and %d", artifactsListMaxPageSize)
	}
	return n, nil
}

// artifactsModuleToken returns the storage-gcs-module bearer token for this request.
// A configured token file is read every call. The secret may not exist when
// horizon-api starts, and kubelet updates the mounted file when it is created
// or rotated. An empty path falls back to the static token for tests and local runs.
func (s *Server) artifactsModuleToken() (string, error) {
	path := strings.TrimSpace(s.opt.ArtifactsModuleTokenFile)
	if path == "" {
		return strings.TrimSpace(s.opt.ArtifactsModuleToken), nil
	}
	// Kubelet swaps the secret volume's ..data symlink while the key file still
	// points at it, so one read can see ENOENT even though the secret exists.
	// Retry only when that projected volume is present. An optional secret that
	// has not been created yet is an empty directory and returns immediately.
	const attempts = 3
	var lastErr error
	for i := 0; i < attempts; i++ {
		b, err := readArtifactsTokenFile(path)
		if err == nil {
			return strings.TrimSpace(string(b)), nil
		}
		lastErr = err
		if !errors.Is(err, os.ErrNotExist) || !artifactsTokenVolumeProjected(path) || i == attempts-1 {
			return "", err
		}
		time.Sleep(artifactsTokenReadRetryWait)
	}
	return "", lastErr
}

// artifactsTokenVolumeProjected reports whether path sits on a kubelet secret
// volume. Those volumes publish the current data through a ..data symlink.
func artifactsTokenVolumeProjected(path string) bool {
	_, err := os.Lstat(filepath.Join(filepath.Dir(path), "..data"))
	return err == nil
}

func (s *Server) artifactsModuleClient() (*http.Client, error) {
	base, ok := http.DefaultTransport.(*http.Transport)
	if !ok {
		base = &http.Transport{}
	}
	transport := base.Clone()
	caFile := strings.TrimSpace(s.opt.ArtifactsModuleCAFile)
	if caFile != "" {
		pemBytes, err := os.ReadFile(caFile)
		if err != nil {
			return nil, fmt.Errorf("read artifacts module CA: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pemBytes) {
			return nil, errors.New("parse artifacts module CA")
		}
		transport.TLSClientConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
			RootCAs:    pool,
		}
	}
	return &http.Client{Timeout: artifactsModuleProxyTimeout, Transport: transport}, nil
}

// artifactsValidBucketName matches a single GCS bucket name: 3–63 characters,
// lowercase letters, digits, and interior '.', '_', or '-'. Rejecting anything
// else keeps the name from becoming an extra path segment on the module proxy.
func artifactsValidBucketName(bucket string) bool {
	if len(bucket) < 3 || len(bucket) > 63 {
		return false
	}
	for i := 0; i < len(bucket); i++ {
		c := bucket[i]
		switch {
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9':
		case c == '-' || c == '_' || c == '.':
			if i == 0 || i == len(bucket)-1 {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// artifactsProjectPrefixOK is true when no project id is configured, or bucket
// is "{projectID}-...". The prefix applies to the static allowlist and to
// GCSBucket CRs, matching storage-gcs-module.
func artifactsProjectPrefixOK(projectID, bucket string) bool {
	projectID = strings.TrimSpace(projectID)
	if projectID == "" {
		return true
	}
	return strings.HasPrefix(bucket, projectID+"-")
}

// artifactsCRBucketNames returns spec.bucketName values from GCSBucket CRs
// (gcsmanager.horizon.io/v1alpha1) in the GCS buckets namespace. That namespace
// is horizon, where storage-gcs-module watches GCSBucket.
// NotFound and Unauthorized are an empty result so the static allowlist still
// works. Forbidden is not empty: it means this service account cannot list
// gcsbuckets, and callers fail the request.
func (s *Server) artifactsCRBucketNames(ctx context.Context) ([]string, error) {
	if s.opt.K8sClient == nil || s.opt.GCSBucketsNS == "" {
		return nil, nil
	}
	var crList unstructured.UnstructuredList
	crList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "gcsmanager.horizon.io",
		Version: "v1alpha1",
		Kind:    "GCSBucketList",
	})
	if err := s.opt.K8sClient.List(ctx, &crList, client.InNamespace(s.opt.GCSBucketsNS)); err != nil {
		if apierrors.IsNotFound(err) || apierrors.IsUnauthorized(err) || meta.IsNoMatchError(err) {
			return nil, nil
		}
		return nil, err
	}
	names := make([]string, 0, len(crList.Items))
	for _, item := range crList.Items {
		bucketName, _, _ := unstructured.NestedString(item.Object, "spec", "bucketName")
		bucketName = strings.TrimSpace(bucketName)
		if bucketName != "" {
			names = append(names, bucketName)
		}
	}
	return names, nil
}

// artifactsAllowedBucketNames is the static allowlist plus GCSBucket CRs, after
// the project-prefix and bucket-name checks.
func (s *Server) artifactsAllowedBucketNames(ctx context.Context) ([]string, error) {
	seen := make(map[string]struct{})
	for bucket := range s.opt.ArtifactsStaticAllowedBuckets {
		if artifactsValidBucketName(bucket) && artifactsProjectPrefixOK(s.opt.ArtifactsGCSProjectID, bucket) {
			seen[bucket] = struct{}{}
		}
	}
	crNames, err := s.artifactsCRBucketNames(ctx)
	if err != nil {
		return nil, err
	}
	for _, name := range crNames {
		if artifactsValidBucketName(name) && artifactsProjectPrefixOK(s.opt.ArtifactsGCSProjectID, name) {
			seen[name] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for name := range seen {
		out = append(out, name)
	}
	sort.Strings(out)
	return out, nil
}

// artifactsBucketAllowed reports whether bucket may be listed or proxied.
// A configured project id requires the "{projectID}-" prefix even when the
// name is in the static allowlist. Lookup errors fail closed.
func (s *Server) artifactsBucketAllowed(ctx context.Context, bucket string) (bool, error) {
	bucket = strings.TrimSpace(bucket)
	if !artifactsValidBucketName(bucket) {
		return false, errInvalidArtifactsBucket
	}
	if !artifactsProjectPrefixOK(s.opt.ArtifactsGCSProjectID, bucket) {
		return false, nil
	}
	if _, ok := s.opt.ArtifactsStaticAllowedBuckets[bucket]; ok {
		return true, nil
	}
	crNames, err := s.artifactsCRBucketNames(ctx)
	if err != nil {
		return false, err
	}
	for _, name := range crNames {
		if name == bucket {
			return true, nil
		}
	}
	return false, nil
}

// handleArtifactsListBuckets lists allowed buckets from static allowlist and GCSBucket CRs
func (s *Server) handleArtifactsListBuckets(w http.ResponseWriter, r *http.Request, p *auth.Principal) {
	if r.Method != http.MethodGet {
		http.NotFound(w, r)
		return
	}

	names, err := s.artifactsAllowedBucketNames(r.Context())
	if err != nil {
		log.Printf("failed to list GCSBucket CRs: %v", err)
		http.Error(w, "failed to list buckets", http.StatusInternalServerError)
		return
	}

	bucketList := make([]artifactsBucketInfo, 0, len(names))
	for _, name := range names {
		bucketList = append(bucketList, artifactsBucketInfo{Name: name})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(artifactsListBucketsResponse{Buckets: bucketList})
}

// handleArtifactsListObjects lists objects through storage-gcs-module
// GET /api/v1/buckets/{bucket}/objects-list.
// POST .../objects/filter is the wrong API for this: filterType selects
// metadata ("specific metadata", "any metadata", "no metadata"), an empty
// path is rejected, and a path without a trailing slash is one object name.
func (s *Server) handleArtifactsListObjects(w http.ResponseWriter, r *http.Request, p *auth.Principal) {
	if r.Method != http.MethodGet {
		http.NotFound(w, r)
		return
	}

	if s.opt.ArtifactsModuleURL == "" {
		http.Error(w, "artifacts module not configured", http.StatusServiceUnavailable)
		return
	}

	bucket := strings.TrimSpace(r.PathValue("bucket"))
	if bucket == "" {
		http.Error(w, "bucket name required", http.StatusBadRequest)
		return
	}
	// Enforce the allowlist here. storage-gcs-module has its own check, but this
	// API must not proxy a bucket it would refuse to list.
	allowed, err := s.artifactsBucketAllowed(r.Context(), bucket)
	if errors.Is(err, errInvalidArtifactsBucket) {
		http.Error(w, "invalid bucket name", http.StatusBadRequest)
		return
	}
	if err != nil {
		log.Printf("artifacts bucket allowlist check failed: %v", err)
		http.Error(w, "bucket allowlist check failed", http.StatusInternalServerError)
		return
	}
	if !allowed {
		http.Error(w, "bucket not allowed", http.StatusForbidden)
		return
	}

	pageSize, err := artifactsListPageSize(r.URL.Query().Get("limit"))
	if err != nil {
		http.Error(w, "invalid limit", http.StatusBadRequest)
		return
	}
	token, err := s.artifactsModuleToken()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			http.Error(w, "artifacts module token not configured", http.StatusServiceUnavailable)
			return
		}
		log.Printf("artifacts module token: %v", err)
		http.Error(w, "failed to read artifacts module token", http.StatusBadGateway)
		return
	}
	if token == "" {
		http.Error(w, "artifacts module token not configured", http.StatusServiceUnavailable)
		return
	}

	// Prefix is a GCS object-name prefix. Leave it unchanged: an empty prefix
	// lists from the start of the bucket, and a prefix without a trailing
	// slash is not an exact object name.
	q := url.Values{}
	if prefix := strings.TrimSpace(r.URL.Query().Get("prefix")); prefix != "" {
		q.Set("prefix", prefix)
	}
	if pageToken := strings.TrimSpace(r.URL.Query().Get("pageToken")); pageToken != "" {
		q.Set("pageToken", pageToken)
	}
	q.Set("pageSize", strconv.Itoa(pageSize))
	proxyURL := fmt.Sprintf("%s/api/v1/buckets/%s/objects-list?%s",
		strings.TrimSuffix(s.opt.ArtifactsModuleURL, "/"), url.PathEscape(bucket), q.Encode())

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, proxyURL, nil)
	if err != nil {
		http.Error(w, "failed to create proxy request", http.StatusInternalServerError)
		return
	}
	req.Header.Set("Authorization", "Bearer "+token)

	httpClient, err := s.artifactsModuleClient()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			http.Error(w, "artifacts module CA not configured", http.StatusServiceUnavailable)
			return
		}
		log.Printf("artifacts module client: %v", err)
		http.Error(w, "failed to proxy to artifacts module", http.StatusBadGateway)
		return
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		log.Printf("artifacts module proxy error: %v", err)
		if errors.Is(err, context.DeadlineExceeded) {
			http.Error(w, "artifacts module timed out", http.StatusGatewayTimeout)
			return
		}
		http.Error(w, "failed to proxy to artifacts module", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Read response. Client.Timeout also aborts a stalled body.
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			http.Error(w, "artifacts module timed out", http.StatusGatewayTimeout)
			return
		}
		http.Error(w, "failed to read artifacts module response", http.StatusBadGateway)
		return
	}

	// Handle artifacts module errors
	if resp.StatusCode != http.StatusOK {
		log.Printf("artifacts module returned status %d: %s", resp.StatusCode, string(respBody))
		if resp.StatusCode == http.StatusBadRequest {
			http.Error(w, "invalid list request", http.StatusBadRequest)
			return
		}
		if resp.StatusCode == http.StatusUnauthorized {
			http.Error(w, "artifacts module unauthorized", http.StatusBadGateway)
			return
		}
		if resp.StatusCode == http.StatusForbidden {
			http.Error(w, "bucket not allowed", http.StatusForbidden)
			return
		}
		if resp.StatusCode == http.StatusNotFound {
			http.Error(w, "bucket not found", http.StatusNotFound)
			return
		}
		http.Error(w, "artifacts module error", http.StatusBadGateway)
		return
	}

	// Parse artifacts module response. size and contentType are GCS object
	// attributes on each object, not keys in the custom metadata map.
	var artifactsResp struct {
		Objects       []artifactsModuleObject `json:"objects"`
		NextPageToken string                  `json:"nextPageToken"`
	}

	if err := json.Unmarshal(respBody, &artifactsResp); err != nil {
		log.Printf("failed to parse artifacts module response: %v", err)
		http.Error(w, "invalid artifacts module response", http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(artifactsListObjectsResponse{
		Objects:       artifactsObjectsFromModule(artifactsResp.Objects),
		NextPageToken: artifactsResp.NextPageToken,
	})
}
