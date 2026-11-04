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

package v1alpha1

import (
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// DeletionPolicyRetain leaves the GCP bucket when the CR is deleted (default).
	DeletionPolicyRetain = "Retain"
	// DeletionPolicyDelete empties and deletes the GCP bucket when the CR is deleted.
	DeletionPolicyDelete = "Delete"
)

// GCSBucketSpec defines desired GCS bucket configuration.
type GCSBucketSpec struct {
	// BucketName is the global GCS bucket name. Must be "{projectId}-...".
	BucketName string `json:"bucketName"`
	// Location is the GCS location or region (e.g. US, EU, us-central1).
	Location string `json:"location"`
	// UniformBucketLevelAccess defaults to true when omitted.
	// +optional
	UniformBucketLevelAccess *bool `json:"uniformBucketLevelAccess,omitempty"`
	// Labels are optional GCP labels on the bucket.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`
	// DeletionPolicy is Retain (default) or Delete. Retain keeps the GCP bucket
	// when the CR is removed; Delete empties all object generations then deletes the bucket.
	// +optional
	DeletionPolicy string `json:"deletionPolicy,omitempty"`
}

// DeleteGCPBucketOnCRDelete is true only for an explicit Delete policy.
func (s GCSBucketSpec) DeleteGCPBucketOnCRDelete() bool {
	return strings.EqualFold(strings.TrimSpace(s.DeletionPolicy), DeletionPolicyDelete)
}

// GCSBucketStatus is written by the controller after reconciling with GCS.
type GCSBucketStatus struct {
	Phase   string `json:"phase,omitempty"`
	Message string `json:"message,omitempty"`
	GsURI   string `json:"gsUri,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// GCSBucket is the Schema for the gcsbuckets API.
type GCSBucket struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GCSBucketSpec   `json:"spec,omitempty"`
	Status GCSBucketStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// GCSBucketList contains a list of GCSBucket.
type GCSBucketList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GCSBucket `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GCSBucket{}, &GCSBucketList{})
}
