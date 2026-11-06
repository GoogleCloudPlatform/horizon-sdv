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
	"context"
	"net/http"

	"sigs.k8s.io/controller-runtime/pkg/client"

	gcsv1alpha1 "storage-gcs-module/api/v1alpha1"
	"storage-gcs-module/internal/gcsname"
)

// requireBucket validates the bucket name and allowlist. Returns false if the response was already written.
func (s *Server) requireBucket(w http.ResponseWriter, r *http.Request, bucket string) bool {
	if len(bucket) < 3 {
		writeErr(w, http.StatusBadRequest, "invalid bucket name")
		return false
	}
	if s.ProjectID != "" && !gcsname.HasProjectPrefix(s.ProjectID, bucket) {
		writeErr(w, http.StatusForbidden, "bucket not allowed")
		return false
	}
	ok, err := s.bucketAllowed(r.Context(), bucket)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "bucket allowlist check failed")
		return false
	}
	if !ok {
		writeErr(w, http.StatusForbidden, "bucket not allowed")
		return false
	}
	return true
}

// bucketAllowed is true if bucket is in the static allowlist or matches a GCSBucket CR in ManagerNamespace.
func (s *Server) bucketAllowed(ctx context.Context, bucket string) (bool, error) {
	if s.ProjectID != "" && !gcsname.HasProjectPrefix(s.ProjectID, bucket) {
		return false, nil
	}
	if _, ok := s.StaticAllowedBuckets[bucket]; ok {
		return true, nil
	}
	if s.K8s == nil {
		return false, nil
	}
	var list gcsv1alpha1.GCSBucketList
	if err := s.K8s.List(ctx, &list, client.InNamespace(s.ManagerNamespace)); err != nil {
		return false, err
	}
	for i := range list.Items {
		name := list.Items[i].Spec.BucketName
		if name == bucket && (s.ProjectID == "" || gcsname.HasProjectPrefix(s.ProjectID, name)) {
			return true, nil
		}
	}
	return false, nil
}
