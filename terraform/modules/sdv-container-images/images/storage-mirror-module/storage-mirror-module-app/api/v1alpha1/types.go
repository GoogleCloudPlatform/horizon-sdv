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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// ConfigName is the only accepted StorageMirrorConfig name.
	ConfigName = "default"

	KindRepo = "Repo"
	KindGit  = "Git"

	LFSInherit  = "Inherit"
	LFSDisabled = "Disabled"
	LFSEnabled  = "Enabled"
	LFSAuto     = "Auto"

	DeletionPolicyRetain = "Retain"
	DeletionPolicyDelete = "Delete"

	ScheduleInherit = "Inherit"
	ScheduleNone    = "None"

	PhasePending      = "Pending"
	PhaseInitializing = "Initializing"
	PhaseSyncing      = "Syncing"
	PhaseReady        = "Ready"
	PhaseError        = "Error"
	PhaseDeleting     = "Deleting"

	// Finalizer guards mirror data: the CR goes away only after Retain or a successful purge.
	Finalizer = "storagemirror.horizon-sdv.io/data"

	AnnotationSyncRequestedAt = "storagemirror.horizon-sdv.io/sync-requested-at"
	AnnotationDeleteRequestID = "storagemirror.horizon-sdv.io/delete-request-id"
	// AnnotationDeleteRequestedBy records who deleted the mirror (submitter of the purge run).
	AnnotationDeleteRequestedBy = "storagemirror.horizon-sdv.io/delete-requested-by"
	AnnotationCreatedBy         = "storagemirror.horizon-sdv.io/created-by"

	LabelModule    = "horizon-sdv.io/module"
	LabelMirror    = "storagemirror.horizon-sdv.io/mirror"
	LabelRequestID = "storagemirror.horizon-sdv.io/request-id"
	LabelKind      = "storagemirror.horizon-sdv.io/kind"
	LabelAccess    = "storagemirror.horizon-sdv.io/access"
	ModuleName     = "storage-mirror"
)

// VolumeSpec describes the Filestore instance that holds every mirror.
type VolumeSpec struct {
	// ResourceID is the Filestore instance name.
	// +kubebuilder:default=sdv-storage-mirror
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="resourceID is immutable"
	// +optional
	ResourceID string `json:"resourceID,omitempty"`
	// ExistingInstance adopts an existing instance (for example a CSI-created pvc-<uid>) instead of ResourceID.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="existingInstance is immutable"
	// +optional
	ExistingInstance string `json:"existingInstance,omitempty"`
	// CapacityGb of the share. Increase only. Zonal tier steps: multiples of 256 up to 9984 below 10 TiB, multiples
	// of 2560 from 10240 (same rule as volume.ValidateCapacity).
	// +kubebuilder:validation:Minimum=1024
	// +kubebuilder:validation:Maximum=102400
	// +kubebuilder:validation:XValidation:rule="self < 10240 ? (self % 256 == 0 && self <= 9984) : self % 2560 == 0",message="capacityGb must be a multiple of 256 up to 9984, or a multiple of 2560 from 10240"
	// +kubebuilder:validation:XValidation:rule="self >= oldSelf",message="capacityGb cannot be decreased"
	CapacityGb int64 `json:"capacityGb"`
	// +kubebuilder:default=ZONAL
	// +optional
	Tier string `json:"tier,omitempty"`
	// Zone of the zonal instance.
	Zone string `json:"zone"`
	// Network is the VPC name the instance attaches to.
	Network string `json:"network"`
	// +optional
	ReservedIPRange string `json:"reservedIpRange,omitempty"`
	// ShareName of the single file share.
	// +kubebuilder:default=mirror
	// +optional
	ShareName string `json:"shareName,omitempty"`
	// AllowDestroy is the Git half of the two-key volume destroy.
	// +optional
	AllowDestroy bool `json:"allowDestroy,omitempty"`
}

// LayoutSpec is the on-disk layout shared with the Jenkins mirror jobs. The mount path in pods is a chart value
// (mirror.layout.mountPath) rendered into the WorkflowTemplates; the controller never mounts the share.
// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="layout is immutable"
type LayoutSpec struct {
	// RootSubdir is the directory under the share root that holds the mirrors and metadata.yaml; its name is also
	// the metadata.yaml root key, so it is a single path component.
	// +kubebuilder:default=horizon-mirrors
	// +kubebuilder:validation:Pattern=`^[A-Za-z0-9][A-Za-z0-9._-]{0,99}$`
	// +optional
	RootSubdir string `json:"rootSubdir,omitempty"`
}

// RepoDefaults are used for repo mirrors when the mirror leaves a field empty.
type RepoDefaults struct {
	// +optional
	ManifestURL string `json:"manifestUrl,omitempty"`
	// +optional
	ManifestRef string `json:"manifestRef,omitempty"`
	// +optional
	ManifestFile string `json:"manifestFile,omitempty"`
	// +optional
	SyncJobs int32 `json:"syncJobs,omitempty"`
	// +optional
	SyncTimeout string `json:"syncTimeout,omitempty"`
}

// GitDefaults are used for git mirrors when the mirror leaves a field empty.
type GitDefaults struct {
	// Branches are head patterns; ["*"] mirrors every branch. Tags are always fetched.
	// +optional
	Branches []string `json:"branches,omitempty"`
	// ExcludeRefs are deleted after each fetch.
	// +optional
	ExcludeRefs []string `json:"excludeRefs,omitempty"`
	// +optional
	SyncTimeout string `json:"syncTimeout,omitempty"`
}

// MirrorDefaults hold the global defaults for new mirrors.
type MirrorDefaults struct {
	// +optional
	Repo RepoDefaults `json:"repo,omitempty"`
	// +optional
	Git GitDefaults `json:"git,omitempty"`
	// +kubebuilder:validation:Enum=Disabled;Enabled;Auto
	// +kubebuilder:default=Disabled
	// +optional
	LFS string `json:"lfs,omitempty"`
}

// ScheduleSpec is a cron schedule applied to mirrors selected by Selector.
type ScheduleSpec struct {
	Name string `json:"name"`
	Cron string `json:"cron"`
	// +kubebuilder:default=UTC
	// +optional
	TimeZone string `json:"timeZone,omitempty"`
	// Selector over StorageMirror labels; empty selects every mirror.
	// +optional
	Selector *metav1.LabelSelector `json:"selector,omitempty"`
}

// RetentionSpec controls history and repository housekeeping.
type RetentionSpec struct {
	// +kubebuilder:default=10
	// +optional
	SyncHistoryLimit int32 `json:"syncHistoryLimit,omitempty"`
	// +optional
	PruneRefs []string `json:"pruneRefs,omitempty"`
	// +kubebuilder:validation:Enum=Aggressive;Auto;None
	// +kubebuilder:default=Aggressive
	// +optional
	GC string `json:"gc,omitempty"`
	// +kubebuilder:default=14
	// +optional
	StaleAfterDays int32 `json:"staleAfterDays,omitempty"`
}

// JenkinsConsumer is the claim that existing Jenkins builders mount.
type JenkinsConsumer struct {
	// +optional
	Enabled bool `json:"enabled,omitempty"`
	// +kubebuilder:default=jenkins
	// +optional
	Namespace string `json:"namespace,omitempty"`
	// +kubebuilder:default=sdv-mirror-filestore-pvc
	// +optional
	ClaimName string `json:"claimName,omitempty"`
	// +optional
	ReadOnly bool `json:"readOnly,omitempty"`
}

// WorkflowsConsumer are the claims in the Argo Workflows namespace.
type WorkflowsConsumer struct {
	// +kubebuilder:default=workflows
	// +optional
	Namespace string `json:"namespace,omitempty"`
	// +kubebuilder:default=storage-mirror-rw
	// +optional
	RWClaimName string `json:"rwClaimName,omitempty"`
	// +kubebuilder:default=storage-mirror-ro
	// +optional
	ROClaimName string `json:"roClaimName,omitempty"`
}

// ConsumersSpec lists the namespaces that get a claim on the share.
type ConsumersSpec struct {
	// +optional
	Jenkins JenkinsConsumer `json:"jenkins,omitempty"`
	// +optional
	Workflows WorkflowsConsumer `json:"workflows,omitempty"`
}

// StorageMirrorConfigSpec is the top-level Storage Mirror configuration (Git is the only writer).
type StorageMirrorConfigSpec struct {
	Volume VolumeSpec `json:"volume"`
	// +optional
	Layout LayoutSpec `json:"layout,omitempty"`
	// +optional
	Defaults MirrorDefaults `json:"defaults,omitempty"`
	// +optional
	Schedules []ScheduleSpec `json:"schedules,omitempty"`
	// +optional
	Retention RetentionSpec `json:"retention,omitempty"`
	// +optional
	Consumers ConsumersSpec `json:"consumers,omitempty"`
	// AdoptExisting imports catalog entries without a CR (as Retain) whenever the catalog is reported.
	// +optional
	AdoptExisting bool `json:"adoptExisting,omitempty"`
}

// VolumeStatus is the observed volume state.
type VolumeStatus struct {
	// +optional
	Phase string `json:"phase,omitempty"`
	// +optional
	InstanceName string `json:"instanceName,omitempty"`
	// +optional
	IP string `json:"ip,omitempty"`
	// +optional
	CapacityGb int64 `json:"capacityGb,omitempty"`
	// +optional
	UsedBytes int64 `json:"usedBytes,omitempty"`
	// +optional
	AvailableBytes int64 `json:"availableBytes,omitempty"`
	// +optional
	UsageReportedAt *metav1.Time `json:"usageReportedAt,omitempty"`
}

// StorageMirrorConfigStatus is the observed state of the config.
type StorageMirrorConfigStatus struct {
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// +optional
	Volume VolumeStatus `json:"volume,omitempty"`
	// +optional
	VolumeInitRequestID string `json:"volumeInitRequestId,omitempty"`
	// +optional
	VolumeInitRequestedAt *metav1.Time `json:"volumeInitRequestedAt,omitempty"`
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=smc
// +kubebuilder:validation:XValidation:rule="self.metadata.name == 'default'",message="StorageMirrorConfig must be named default"
// +kubebuilder:printcolumn:name="Volume",type=string,JSONPath=`.status.volume.phase`
// +kubebuilder:printcolumn:name="Capacity",type=integer,JSONPath=`.status.volume.capacityGb`

// StorageMirrorConfig is the singleton that controls Storage Mirror.
type StorageMirrorConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   StorageMirrorConfigSpec   `json:"spec,omitempty"`
	Status StorageMirrorConfigStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// StorageMirrorConfigList contains a list of StorageMirrorConfig.
type StorageMirrorConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []StorageMirrorConfig `json:"items"`
}

// RepoSpec describes an AOSP repo manifest mirror.
type RepoSpec struct {
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="manifestUrl is immutable"
	// +optional
	ManifestURL string `json:"manifestUrl,omitempty"`
	// +optional
	ManifestRef string `json:"manifestRef,omitempty"`
	// +optional
	ManifestFile string `json:"manifestFile,omitempty"`
	// +kubebuilder:validation:Minimum=1
	// +optional
	SyncJobs int32 `json:"syncJobs,omitempty"`
}

// SecretRef names a Secret in the workflows namespace.
type SecretRef struct {
	Name string `json:"name"`
}

// GitSpec describes a single-repository mirror.
type GitSpec struct {
	// URL of the upstream repository (https only).
	// +kubebuilder:validation:Pattern=`^https://`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="url is immutable"
	URL string `json:"url"`
	// +optional
	Branches []string `json:"branches,omitempty"`
	// CredentialsSecretRef is a basic-auth Secret in the workflows namespace.
	// +optional
	CredentialsSecretRef *SecretRef `json:"credentialsSecretRef,omitempty"`
}

// StorageMirrorSpec defines one mirror.
// +kubebuilder:validation:XValidation:rule="self.kind == 'Repo' ? !has(self.git) : (has(self.git) && !has(self.repo))",message="spec.repo or spec.git must match spec.kind"
type StorageMirrorSpec struct {
	// +kubebuilder:validation:Enum=Repo;Git
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="kind is immutable"
	Kind string `json:"kind"`
	// Directory under the root subdirectory; defaults to metadata.name.
	// +kubebuilder:validation:Pattern=`^[A-Za-z0-9][A-Za-z0-9._-]{0,99}$`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="directory is immutable"
	// +kubebuilder:validation:XValidation:rule="self != 'metadata.yaml'",message="directory name is reserved"
	// +optional
	Directory string `json:"directory,omitempty"`
	// +optional
	Repo *RepoSpec `json:"repo,omitempty"`
	// +optional
	Git *GitSpec `json:"git,omitempty"`
	// +kubebuilder:validation:Enum=Inherit;Disabled;Enabled;Auto
	// +kubebuilder:default=Inherit
	// +optional
	LFS string `json:"lfs,omitempty"`
	// Schedule is Inherit (config schedules), None, or a cron expression.
	// +kubebuilder:default=Inherit
	// +optional
	Schedule string `json:"schedule,omitempty"`
	// +optional
	Suspend bool `json:"suspend,omitempty"`
	// SyncTimeout overrides the kind default (GNU timeout format, for example 20h).
	// +optional
	SyncTimeout string `json:"syncTimeout,omitempty"`
	// +kubebuilder:validation:Enum=Retain;Delete
	// +kubebuilder:default=Retain
	// +optional
	DeletionPolicy string `json:"deletionPolicy,omitempty"`
}

// Progress is the structured sync progress reported by mirror-agent.
type Progress struct {
	// +optional
	Phase string `json:"phase,omitempty"`
	// +optional
	State string `json:"state,omitempty"`
	// +optional
	Attempt int32 `json:"attempt,omitempty"`
	// +optional
	MaxAttempts int32 `json:"maxAttempts,omitempty"`
	// +optional
	ProjectsDone int64 `json:"projectsDone,omitempty"`
	// +optional
	ProjectsTotal int64 `json:"projectsTotal,omitempty"`
	// +optional
	CurrentProject string `json:"currentProject,omitempty"`
	// +optional
	RefsUpdated int64 `json:"refsUpdated,omitempty"`
	// +optional
	ObjectsReceived int64 `json:"objectsReceived,omitempty"`
	// +optional
	ObjectsTotal int64 `json:"objectsTotal,omitempty"`
	// +optional
	BytesReceived int64 `json:"bytesReceived,omitempty"`
	// BytesFreed is the space released so far by a delete run.
	// +optional
	BytesFreed int64 `json:"bytesFreed,omitempty"`
	// +optional
	LFSObjectsDone int64 `json:"lfsObjectsDone,omitempty"`
	// +optional
	LFSObjectsTotal int64 `json:"lfsObjectsTotal,omitempty"`
	// +optional
	PhasePercent int32 `json:"phasePercent,omitempty"`
	// +optional
	OverallPercent int32 `json:"overallPercent,omitempty"`
	// +optional
	ElapsedSeconds int64 `json:"elapsedSeconds,omitempty"`
	// +optional
	EtaSeconds int64 `json:"etaSeconds,omitempty"`
	// +optional
	UpdatedAt *metav1.Time `json:"updatedAt,omitempty"`
	// +optional
	Stale bool `json:"stale,omitempty"`
	// +optional
	Message string `json:"message,omitempty"`
}

// RunStatus is one Argo Workflow run for the mirror.
type RunStatus struct {
	// WorkflowName is the run ID.
	// +optional
	WorkflowName string `json:"workflowName,omitempty"`
	// +optional
	Template string `json:"template,omitempty"`
	// +optional
	RequestID string `json:"requestId,omitempty"`
	// +optional
	SubmittedBy string `json:"submittedBy,omitempty"`
	// +optional
	Phase string `json:"phase,omitempty"`
	// +optional
	StartedAt *metav1.Time `json:"startedAt,omitempty"`
	// +optional
	FinishedAt *metav1.Time `json:"finishedAt,omitempty"`
	// +optional
	Message string `json:"message,omitempty"`
	// +optional
	Progress *Progress `json:"progress,omitempty"`
}

// StorageMirrorStatus is the observed state of a mirror.
type StorageMirrorStatus struct {
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// +optional
	Phase string `json:"phase,omitempty"`
	// CatalogStatus is the status value in metadata.yaml.
	// +optional
	CatalogStatus string `json:"catalogStatus,omitempty"`
	// +optional
	LastSuccessfulSyncTime *metav1.Time `json:"lastSuccessfulSyncTime,omitempty"`
	// +optional
	LastHandledSyncRequest string `json:"lastHandledSyncRequest,omitempty"`
	// PendingRequestID is the last dispatched request not yet seen as a Workflow.
	// +optional
	PendingRequestID string `json:"pendingRequestId,omitempty"`
	// +optional
	PendingRequestAt *metav1.Time `json:"pendingRequestAt,omitempty"`
	// +optional
	LFSDetected bool `json:"lfsDetected,omitempty"`
	// +optional
	CurrentRun *RunStatus `json:"currentRun,omitempty"`
	// +optional
	LastRun *RunStatus `json:"lastRun,omitempty"`
	// +optional
	History []RunStatus `json:"history,omitempty"`
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=smir
// +kubebuilder:validation:XValidation:rule="size(self.metadata.name) <= 63",message="name must be at most 63 characters"
// +kubebuilder:printcolumn:name="Kind",type=string,JSONPath=`.spec.kind`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Progress",type=integer,JSONPath=`.status.currentRun.progress.overallPercent`
// +kubebuilder:printcolumn:name="Last-Sync",type=date,JSONPath=`.status.lastSuccessfulSyncTime`

// StorageMirror is one mirror on the shared volume.
type StorageMirror struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   StorageMirrorSpec   `json:"spec,omitempty"`
	Status StorageMirrorStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// StorageMirrorList contains a list of StorageMirror.
type StorageMirrorList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []StorageMirror `json:"items"`
}

func init() {
	SchemeBuilder.Register(&StorageMirrorConfig{}, &StorageMirrorConfigList{}, &StorageMirror{}, &StorageMirrorList{})
}

// DirectoryOrName returns the on-disk directory of the mirror.
func (m *StorageMirror) DirectoryOrName() string {
	if m.Spec.Directory != "" {
		return m.Spec.Directory
	}
	return m.Name
}
