// Copyright (c) 2026 Accenture, All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
)

// ClusterRef references the parent WorkstationCluster.
// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="clusterRef is immutable"
type ClusterRef struct {
	// Name of the WorkstationCluster in the same namespace.
	Name string `json:"name"`
}

// ContainerEnvVar defines an environment variable for the workstation container.
type ContainerEnvVar struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// ContainerConfig defines container runtime settings for workstations.
type ContainerConfig struct {
	// Image is the container image URI.
	// +optional
	Image string `json:"image,omitempty"`

	// Command overrides the container entrypoint.
	// +optional
	Command []string `json:"command,omitempty"`

	// Args provides entrypoint arguments.
	// +optional
	Args []string `json:"args,omitempty"`

	// Env provides environment variables.
	// +optional
	Env []ContainerEnvVar `json:"env,omitempty"`

	// RunAsUser is the UID the container runs as.
	// +optional
	RunAsUser *int64 `json:"runAsUser,omitempty"`

	// WorkingDir is the directory in the container to run commands from.
	// +optional
	WorkingDir string `json:"workingDir,omitempty"`
}

// GceServiceAccountRef references a Google service account.
type GceServiceAccountRef struct {
	// External email of the service account.
	// +optional
	External string `json:"external,omitempty"`

	// Name of a KCC IAMServiceAccount resource.
	// +optional
	Name string `json:"name,omitempty"`

	// Namespace of a KCC IAMServiceAccount resource.
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// ShieldedInstanceConfig defines shielded VM settings.
type ShieldedInstanceConfig struct {
	// +optional
	EnableIntegrityMonitoring *bool `json:"enableIntegrityMonitoring,omitempty"`
	// +optional
	EnableSecureBoot *bool `json:"enableSecureBoot,omitempty"`
	// +optional
	EnableVTPM *bool `json:"enableVTPM,omitempty"`
}

// ConfidentialInstanceConfig defines confidential compute settings.
type ConfidentialInstanceConfig struct {
	// +optional
	EnableConfidentialCompute *bool `json:"enableConfidentialCompute,omitempty"`
}

// GceInstanceConfig defines Compute Engine instance settings.
type GceInstanceConfig struct {
	// MachineType is the GCE machine type (e.g. e2-standard-4).
	// +optional
	MachineType string `json:"machineType,omitempty"`

	// BootDiskSizeGB is the size of the boot disk in GB.
	// +optional
	BootDiskSizeGB *int64 `json:"bootDiskSizeGB,omitempty"`

	// ServiceAccountScopes are OAuth scopes for the host VM service account.
	// +optional
	ServiceAccountScopes []string `json:"serviceAccountScopes,omitempty"`

	// DisablePublicIPAddresses disables public IPs on workstation VMs.
	// +optional
	DisablePublicIPAddresses *bool `json:"disablePublicIPAddresses,omitempty"`

	// EnableNestedVirtualization enables nested virtualization for VM-in-VM workloads.
	// +optional
	EnableNestedVirtualization *bool `json:"enableNestedVirtualization,omitempty"`

	// PoolSize is the number of pre-warmed VMs kept ready.
	// +optional
	PoolSize *int64 `json:"poolSize,omitempty"`

	// Tags are network tags applied to the VM instances.
	// +optional
	Tags []string `json:"tags,omitempty"`

	// ShieldedInstanceConfig defines shielded VM properties.
	// +optional
	ShieldedInstanceConfig *ShieldedInstanceConfig `json:"shieldedInstanceConfig,omitempty"`

	// ConfidentialInstanceConfig defines confidential compute settings.
	// +optional
	ConfidentialInstanceConfig *ConfidentialInstanceConfig `json:"confidentialInstanceConfig,omitempty"`
}

// HostConfig defines the host VM configuration.
type HostConfig struct {
	// GceInstance holds Compute Engine instance properties.
	// +optional
	GceInstance *GceInstanceConfig `json:"gceInstance,omitempty"`
}

// GcePersistentDisk defines persistent disk attributes.
type GcePersistentDisk struct {
	// +optional
	DiskType string `json:"diskType,omitempty"`
	// +optional
	FsType string `json:"fsType,omitempty"`
	// +optional
	ReclaimPolicy string `json:"reclaimPolicy,omitempty"`
	// +optional
	SizeGB *int64 `json:"sizeGB,omitempty"`
	// +optional
	SourceSnapshot string `json:"sourceSnapshot,omitempty"`
}

// PersistentDirectory defines a persistent disk mount.
type PersistentDirectory struct {
	MountPath string             `json:"mountPath"`
	GcePD     *GcePersistentDisk `json:"gcePD,omitempty"`
}

// ReadinessCheck defines a readiness probe on a workstation port/path.
type ReadinessCheck struct {
	Path string `json:"path"`
	Port int32  `json:"port"`
}

// CryptoKeyRef references a Cloud KMS crypto key.
type CryptoKeyRef struct {
	// +optional
	External string `json:"external,omitempty"`
	// +optional
	Name string `json:"name,omitempty"`
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// EncryptionKeyConfig defines customer-managed encryption key (CMEK) settings.
type EncryptionKeyConfig struct {
	// +optional
	KmsCryptoKeyRef *CryptoKeyRef `json:"kmsCryptoKeyRef,omitempty"`
	// +optional
	ServiceAccountRef *GceServiceAccountRef `json:"serviceAccountRef,omitempty"`
}

// PortRange specifies a range of allowed ports.
type PortRange struct {
	First int32 `json:"first"`
	Last  int32 `json:"last"`
}

// EphemeralPersistentDisk defines an ephemeral disk specification.
type EphemeralPersistentDisk struct {
	// +optional
	DiskType string `json:"diskType,omitempty"`
	// +optional
	SourceSnapshot string `json:"sourceSnapshot,omitempty"`
	// +optional
	SourceImage string `json:"sourceImage,omitempty"`
	// +optional
	ReadOnly *bool `json:"readOnly,omitempty"`
}

// EphemeralDirectory defines an ephemeral directory mounted into the workstation.
type EphemeralDirectory struct {
	MountPath string                   `json:"mountPath"`
	GcePD     *EphemeralPersistentDisk `json:"gcePD,omitempty"`
}

// WorkstationConfigSpec defines configuration for workstations created from this config.
// +kubebuilder:validation:XValidation:rule="!has(oldSelf.resourceID) || (has(self.resourceID) && self.resourceID == oldSelf.resourceID)",message="resourceID is immutable once set"
type WorkstationConfigSpec struct {
	// ClusterRef references the parent WorkstationCluster. (KCC-owned via parentRef)
	ClusterRef ClusterRef `json:"clusterRef"`

	// DisplayName is a human-readable name for the config. (KCC-owned)
	// +optional
	DisplayName string `json:"displayName,omitempty"`

	// IdleTimeout shuts down inactive workstations (e.g. 7200s). (KCC-owned)
	// +optional
	IdleTimeout string `json:"idleTimeout,omitempty"`

	// RunningTimeout shuts down running workstations after duration (e.g. 43200s). (KCC-owned)
	// +optional
	RunningTimeout string `json:"runningTimeout,omitempty"`

	// ReplicaZones within the region where workstations can run. (KCC-owned)
	// +optional
	ReplicaZones []string `json:"replicaZones,omitempty"`

	// Container runtime settings. (KCC-owned)
	// +optional
	Container *ContainerConfig `json:"container,omitempty"`

	// Host VM configuration. (KCC-owned)
	// +optional
	Host *HostConfig `json:"host,omitempty"`

	// PersistentDirectories mounted into workstations. (KCC-owned)
	// +optional
	PersistentDirectories []PersistentDirectory `json:"persistentDirectories,omitempty"`

	// ReadinessChecks for workstation health. (KCC-owned)
	// +optional
	ReadinessChecks []ReadinessCheck `json:"readinessChecks,omitempty"`

	// EncryptionKey for CMEK encryption. (KCC-owned)
	// +optional
	EncryptionKey *EncryptionKeyConfig `json:"encryptionKey,omitempty"`

	// ResourceID is the immutable GCP config ID. (KCC-owned)
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
	// =========================================================================

	// DisableSsh disables direct SSH access to workstations. Default true.
	// +horizon:api-owned
	// +optional
	DisableSsh *bool `json:"disableSsh,omitempty"`

	// AllowedPorts specifies gateway port ranges permitted. Defaults to 80 and 1024-65535.
	// +horizon:api-owned
	// +optional
	AllowedPorts []PortRange `json:"allowedPorts,omitempty"`

	// EphemeralDirectories mounted into the workstation.
	// +horizon:api-owned
	// +optional
	EphemeralDirectories []EphemeralDirectory `json:"ephemeralDirectories,omitempty"`

	// Admins granted workstations.admin on this config via IAM.
	// +horizon:api-owned
	// +optional
	Admins []string `json:"admins,omitempty"`
}

// WorkstationConfigPostureStatus is the posture read back from the Workstations API.
type WorkstationConfigPostureStatus struct {
	// DisableSsh is the live SSH setting.
	DisableSsh bool `json:"disableSsh"`

	// AllowedPorts are the live gateway port ranges.
	// +optional
	AllowedPorts []PortRange `json:"allowedPorts,omitempty"`

	// EphemeralDirectories are the live ephemeral mounts.
	// +optional
	EphemeralDirectories []EphemeralDirectory `json:"ephemeralDirectories,omitempty"`
}

// WorkstationConfigStatus defines the observed state of WorkstationConfig.
type WorkstationConfigStatus struct {
	// Phase is the current lifecycle phase (Pending, Ready, Failed, Deleting).
	// +optional
	Phase string `json:"phase,omitempty"`

	// ExternalRef is the GCP resource name copied from the Config Connector object.
	// +optional
	ExternalRef string `json:"externalRef,omitempty"`

	// Posture is the API-owned posture last read back from GCP.
	// +optional
	Posture *WorkstationConfigPostureStatus `json:"posture,omitempty"`

	// ObservedGeneration is the most recent generation observed.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// Conditions represent observations of the config state.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// KCC contains projected Config Connector state.
	// +optional
	KCC *runtime.RawExtension `json:"kcc,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=wscfg;wsconfig
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Cluster",type="string",JSONPath=".spec.clusterRef.name"
// +kubebuilder:printcolumn:name="MachineType",type="string",JSONPath=".spec.host.gceInstance.machineType"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// WorkstationConfig is the Schema for the workstationconfigs API.
type WorkstationConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WorkstationConfigSpec   `json:"spec,omitempty"`
	Status WorkstationConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// WorkstationConfigList contains a list of WorkstationConfig.
type WorkstationConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WorkstationConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&WorkstationConfig{}, &WorkstationConfigList{})
}
