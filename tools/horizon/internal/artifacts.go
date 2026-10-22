// Copyright (c) 2026 Accenture, All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
//
//         http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package horizon

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

type ListBucketsResponse struct {
	Buckets []BucketInfo `json:"buckets"`
}

type BucketInfo struct {
	Name string `json:"name"`
}

type ListObjectsResponse struct {
	Objects []ObjectInfo `json:"objects"`
	// NextPageToken can be used to retrieve the next page of results
	NextPageToken string `json:"nextPageToken,omitempty"`
}

type ObjectInfo struct {
	Name        string `json:"name"`
	Size        int64  `json:"size"`
	ContentType string `json:"contentType,omitempty"`
	Updated     string `json:"updated,omitempty"`
	Generation  int64  `json:"generation,omitempty"`
	MD5Hash     string `json:"md5Hash,omitempty"`
}

func (c *Client) ListBuckets(ctx context.Context) (*ListBucketsResponse, error) {
	b, err := c.GetJSON(ctx, "/v1/artifacts/buckets")
	if err != nil {
		return nil, err
	}
	var resp ListBucketsResponse
	if err := json.Unmarshal(b, &resp); err != nil {
		return nil, fmt.Errorf("list buckets response: %w", err)
	}
	return &resp, nil
}

// ListObjects prefix filters objects by prefix (optional), pageToken retrieves the next page of results (optional), limit limits the number of results (optional)
func (c *Client) ListObjects(ctx context.Context, bucket string, prefix, pageToken string, limit int) (*ListObjectsResponse, error) {
	path := fmt.Sprintf("/v1/artifacts/buckets/%s/objects", url.PathEscape(bucket))
	query := url.Values{}
	if prefix != "" {
		query.Set("prefix", prefix)
	}
	if pageToken != "" {
		query.Set("pageToken", pageToken)
	}
	if limit > 0 {
		query.Set("limit", fmt.Sprintf("%d", limit))
	}
	if len(query) > 0 {
		path += "?" + query.Encode()
	}
	b, err := c.GetJSON(ctx, path)
	if err != nil {
		return nil, err
	}
	var resp ListObjectsResponse
	if err := json.Unmarshal(b, &resp); err != nil {
		return nil, fmt.Errorf("list objects response: %w", err)
	}
	return &resp, nil
}
