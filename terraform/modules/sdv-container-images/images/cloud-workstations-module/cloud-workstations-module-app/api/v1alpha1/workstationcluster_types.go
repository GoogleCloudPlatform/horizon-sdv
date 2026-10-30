// Copyright (c) 2026 Accenture, All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// PrivateClusterConfig contains configuration for private workstation clusters.
type PrivateClusterConfig struct {
	// EnablePrivateEndpoint sets whether workstations should be accessed via Private Service Connect.
	// +optional
	EnablePrivateEndpoint *bool `json:"enablePrivateEndpoint,omitempty"`

	// AllowedProjects contains additional GCP projects allowed to connect via Private Service Connect.
	// +optional
	AllowedProjects []string `json:"allowedProjects,omitempty"`
}

// WorkstationClusterSpec defines the desired state of a WorkstationCluster.
// +kubebuilder:validation:XValidation:rule="!has(oldSelf.resourceID) || (has(self.resourceID) && self.resourceID == oldSelf.resourceID)",message="resourceID is immutable once set"
type WorkstationClusterSpec struct {
	// Project is the GCP project ID that owns the workstation cluster.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="project is immutable"
	Project string `json:"project"`

	// Location is the GCP region where the workstation cluster is located (e.g. us-central1).
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="location is immutable"
	Location string `json:"location"`

	// Network is the VPC network reference or self-link for workstation VMs.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="network is immutable"
	Network string `json:"network"`

	// Subnetwork is the VPC subnetwork reference or self-link for workstation VMs.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="subnetwork is immutable"
	Subnetwork string `json:"subnetwork"`

	// DisplayName is a human-readable name for the workstation cluster.
	// +optional
	DisplayName string `json:"displayName,omitempty"`

	// ResourceID is the immutable GCP resource ID for the workstation cluster.
	// +optional
	ResourceID string `json:"resourceID,omitempty"`

	// PrivateClusterConfig configures private workstation cluster settings.
	// +optional
	PrivateClusterConfig *PrivateClusterConfig `json:"privateClusterConfig,omitempty"`

	// Labels are client-specified labels on the cluster.
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// Annotations are client-specified annotations on the cluster.
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// WorkstationClusterStatus defines the observed state of WorkstationCluster.
type WorkstationClusterStatus struct {
	// Phase is the current lifecycle phase of the cluster (Pending, Ready, Failed, Deleting).
	// +optional
	Phase string `json:"phase,omitempty"`

	// ObservedGeneration is the most recent generation observed for this resource.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions represent the latest available observations of the cluster state.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// KCC contains the projected Config Connector observed state and status.
	// +optional
	KCC *runtime.RawExtension `json:"kcc,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=wsc;wscluster
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Location",type="string",JSONPath=".spec.location"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// WorkstationCluster is the Schema for the workstationclusters API.
type WorkstationCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WorkstationClusterSpec   `json:"spec,omitempty"`
	Status WorkstationClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// WorkstationClusterList contains a list of WorkstationCluster.
type WorkstationClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WorkstationCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&WorkstationCluster{}, &WorkstationClusterList{})
}
