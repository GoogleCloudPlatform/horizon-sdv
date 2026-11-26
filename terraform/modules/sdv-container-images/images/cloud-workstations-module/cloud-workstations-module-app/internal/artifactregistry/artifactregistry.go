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
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const cloudPlatformScope = "https://www.googleapis.com/auth/cloud-platform"

// Ref is a tagged image reference such as us-central1-docker.pkg.dev/project/repo/image:tag.
type Ref struct {
	Host string
	Path string
	Tag  string
}

// ParseRef splits a tagged image reference. Digest references are rejected.
func ParseRef(image string) (Ref, error) {
	host, rest, ok := strings.Cut(image, "/")
	if !ok || host == "" {
		return Ref{}, fmt.Errorf("image %q has no host and path", image)
	}
	i := strings.LastIndex(rest, ":")
	if i <= 0 || i == len(rest)-1 || strings.ContainsAny(rest, "@") {
		return Ref{}, fmt.Errorf("image %q must be a tagged reference", image)
	}
	return Ref{Host: host, Path: rest[:i], Tag: rest[i+1:]}, nil
}

func (r Ref) manifestURL() string {
	return fmt.Sprintf("https://%s/v2/%s/manifests/%s", r.Host, r.Path, r.Tag)
}

// Checker answers whether a tagged image exists, using the pod's application default credentials.
type Checker struct {
	client *http.Client
	tokens oauth2.TokenSource
}

// NewChecker returns a Checker for production use.
func NewChecker() *Checker {
	return &Checker{
		client: &http.Client{Timeout: 30 * time.Second},
		tokens: adcTokenSource{},
	}
}

// Exists reports whether the tag exists. Only 200 and 404 are definite answers.
// Any other status or transport error is returned as an error so callers never treat it as missing.
func (c *Checker) Exists(ctx context.Context, image string) (bool, error) {
	ref, err := ParseRef(image)
	if err != nil {
		return false, err
	}
	tok, err := c.tokens.Token()
	if err != nil {
		return false, fmt.Errorf("application default credentials: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, ref.manifestURL(), nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Accept", strings.Join([]string{
		"application/vnd.docker.distribution.manifest.v2+json",
		"application/vnd.docker.distribution.manifest.list.v2+json",
		"application/vnd.oci.image.manifest.v1+json",
		"application/vnd.oci.image.index.v1+json",
	}, ", "))
	req.SetBasicAuth("oauth2accesstoken", tok.AccessToken)
	resp, err := c.client.Do(req)
	if err != nil {
		return false, err
	}
	defer func() { _ = resp.Body.Close() }()
	switch resp.StatusCode {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	default:
		return false, fmt.Errorf("manifest %s: unexpected HTTP %d", ref.manifestURL(), resp.StatusCode)
	}
}

// adcTokenSource resolves credentials on each call, so a metadata server that is not ready yet
// is retried by the caller instead of failing for the life of the process.
type adcTokenSource struct{}

func (adcTokenSource) Token() (*oauth2.Token, error) {
	src, err := google.DefaultTokenSource(context.Background(), cloudPlatformScope)
	if err != nil {
		return nil, err
	}
	return src.Token()
}
