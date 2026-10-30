// Copyright (c) 2026 Accenture, All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// ConfigRef references the parent WorkstationConfig.
// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="configRef is immutable"
type ConfigRef struct {
	// Name of the WorkstationConfig in the same namespace.
	Name string `json:"name"`
}

// WorkstationSpec defines the desired state of an individual Workstation.
// +kubebuilder:validation:XValidation:rule="!has(oldSelf.resourceID) || (has(self.resourceID) && self.resourceID == oldSelf.resourceID)",message="resourceID is immutable once set"
type WorkstationSpec struct {
	// ConfigRef references the parent Horizon WorkstationConfig. (KCC-owned via parentRef)
	ConfigRef ConfigRef `json:"configRef"`

	// DisplayName is a human-readable name for the workstation. (KCC-owned)
	// +optional
	DisplayName string `json:"displayName,omitempty"`

	// ResourceID is the immutable GCP resource ID. (KCC-owned)
	// +optional
	ResourceID string `json:"resourceID,omitempty"`

	// Labels are client-specified labels. (KCC-owned)
	// +optional
	Labels map[string]string `json:"labels,omitempty"`

	// Annotations are client-specified annotations. (KCC-owned)
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`

	// =========================================================================
	// API-owned fields: Actuated via the Workstations API.
	// Never written to Config Connector (KCC) objects.
	// Power is not a spec field. Start and stop are one-shot workflows.
	// =========================================================================

	// Users granted workstations.user on this workstation via IAM.
	// +horizon:api-owned
	// +optional
	Users []string `json:"users,omitempty"`
}

// WorkstationStatus defines the observed state of Workstation.
type WorkstationStatus struct {
	// Phase is the current lifecycle phase (Pending, Ready, Failed, Deleting).
	// +optional
	Phase string `json:"phase,omitempty"`

	// Host is the hostname or IP of the running workstation.
	// +optional
	Host string `json:"host,omitempty"`

	// State is the detailed workstation state copied from Config Connector observedState.
	// It is read-only. The controller does not start or stop from this value.
	// +optional
	State string `json:"state,omitempty"`

	// ExternalRef is the GCP resource name copied from the Config Connector object.
	// +optional
	ExternalRef string `json:"externalRef,omitempty"`

	// ObservedGeneration is the most recent generation observed.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions represent observations of the workstation state.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// KCC contains projected Config Connector state.
	// +optional
	KCC *runtime.RawExtension `json:"kcc,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=ws
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Config",type="string",JSONPath=".spec.configRef.name"
// +kubebuilder:printcolumn:name="Host",type="string",JSONPath=".status.host"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// Workstation is the Schema for the workstations API.
type Workstation struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WorkstationSpec   `json:"spec,omitempty"`
	Status WorkstationStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// WorkstationList contains a list of Workstation.
type WorkstationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Workstation `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Workstation{}, &WorkstationList{})
}
