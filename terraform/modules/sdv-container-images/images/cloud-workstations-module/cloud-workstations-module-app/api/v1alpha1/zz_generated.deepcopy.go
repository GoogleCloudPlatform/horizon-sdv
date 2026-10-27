// Copyright (c) 2026 Accenture, All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// --- PrivateClusterConfig ---

func (in *PrivateClusterConfig) DeepCopyInto(out *PrivateClusterConfig) {
	*out = *in
	if in.EnablePrivateEndpoint != nil {
		in, out := &in.EnablePrivateEndpoint, &out.EnablePrivateEndpoint
		*out = new(bool)
		**out = **in
	}
	if in.AllowedProjects != nil {
		in, out := &in.AllowedProjects, &out.AllowedProjects
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

func (in *PrivateClusterConfig) DeepCopy() *PrivateClusterConfig {
	if in == nil {
		return nil
	}
	out := new(PrivateClusterConfig)
	in.DeepCopyInto(out)
	return out
}

// --- WorkstationClusterSpec ---

func (in *WorkstationClusterSpec) DeepCopyInto(out *WorkstationClusterSpec) {
	*out = *in
	if in.PrivateClusterConfig != nil {
		in, out := &in.PrivateClusterConfig, &out.PrivateClusterConfig
		*out = new(PrivateClusterConfig)
		(*in).DeepCopyInto(*out)
	}
	if in.Labels != nil {
		in, out := &in.Labels, &out.Labels
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.Annotations != nil {
		in, out := &in.Annotations, &out.Annotations
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
}

func (in *WorkstationClusterSpec) DeepCopy() *WorkstationClusterSpec {
	if in == nil {
		return nil
	}
	out := new(WorkstationClusterSpec)
	in.DeepCopyInto(out)
	return out
}

// --- WorkstationClusterStatus ---

func (in *WorkstationClusterStatus) DeepCopyInto(out *WorkstationClusterStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]metav1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.KCC != nil {
		in, out := &in.KCC, &out.KCC
		*out = (*in).DeepCopy()
	}
}

func (in *WorkstationClusterStatus) DeepCopy() *WorkstationClusterStatus {
	if in == nil {
		return nil
	}
	out := new(WorkstationClusterStatus)
	in.DeepCopyInto(out)
	return out
}

// --- WorkstationCluster ---

func (in *WorkstationCluster) DeepCopyInto(out *WorkstationCluster) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

func (in *WorkstationCluster) DeepCopy() *WorkstationCluster {
	if in == nil {
		return nil
	}
	out := new(WorkstationCluster)
	in.DeepCopyInto(out)
	return out
}

func (in *WorkstationCluster) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// --- WorkstationClusterList ---

func (in *WorkstationClusterList) DeepCopyInto(out *WorkstationClusterList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]WorkstationCluster, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

func (in *WorkstationClusterList) DeepCopy() *WorkstationClusterList {
	if in == nil {
		return nil
	}
	out := new(WorkstationClusterList)
	in.DeepCopyInto(out)
	return out
}

func (in *WorkstationClusterList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// --- ClusterRef ---

func (in *ClusterRef) DeepCopyInto(out *ClusterRef) {
	*out = *in
}

func (in *ClusterRef) DeepCopy() *ClusterRef {
	if in == nil {
		return nil
	}
	out := new(ClusterRef)
	in.DeepCopyInto(out)
	return out
}

// --- ContainerEnvVar ---

func (in *ContainerEnvVar) DeepCopyInto(out *ContainerEnvVar) {
	*out = *in
}

func (in *ContainerEnvVar) DeepCopy() *ContainerEnvVar {
	if in == nil {
		return nil
	}
	out := new(ContainerEnvVar)
	in.DeepCopyInto(out)
	return out
}

// --- ContainerConfig ---

func (in *ContainerConfig) DeepCopyInto(out *ContainerConfig) {
	*out = *in
	if in.Command != nil {
		in, out := &in.Command, &out.Command
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Args != nil {
		in, out := &in.Args, &out.Args
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Env != nil {
		in, out := &in.Env, &out.Env
		*out = make([]ContainerEnvVar, len(*in))
		copy(*out, *in)
	}
	if in.RunAsUser != nil {
		in, out := &in.RunAsUser, &out.RunAsUser
		*out = new(int64)
		**out = **in
	}
}

func (in *ContainerConfig) DeepCopy() *ContainerConfig {
	if in == nil {
		return nil
	}
	out := new(ContainerConfig)
	in.DeepCopyInto(out)
	return out
}

// --- GceServiceAccountRef ---

func (in *GceServiceAccountRef) DeepCopyInto(out *GceServiceAccountRef) {
	*out = *in
}

func (in *GceServiceAccountRef) DeepCopy() *GceServiceAccountRef {
	if in == nil {
		return nil
	}
	out := new(GceServiceAccountRef)
	in.DeepCopyInto(out)
	return out
}

// --- ShieldedInstanceConfig ---

func (in *ShieldedInstanceConfig) DeepCopyInto(out *ShieldedInstanceConfig) {
	*out = *in
	if in.EnableIntegrityMonitoring != nil {
		in, out := &in.EnableIntegrityMonitoring, &out.EnableIntegrityMonitoring
		*out = new(bool)
		**out = **in
	}
	if in.EnableSecureBoot != nil {
		in, out := &in.EnableSecureBoot, &out.EnableSecureBoot
		*out = new(bool)
		**out = **in
	}
	if in.EnableVTPM != nil {
		in, out := &in.EnableVTPM, &out.EnableVTPM
		*out = new(bool)
		**out = **in
	}
}

func (in *ShieldedInstanceConfig) DeepCopy() *ShieldedInstanceConfig {
	if in == nil {
		return nil
	}
	out := new(ShieldedInstanceConfig)
	in.DeepCopyInto(out)
	return out
}

// --- ConfidentialInstanceConfig ---

func (in *ConfidentialInstanceConfig) DeepCopyInto(out *ConfidentialInstanceConfig) {
	*out = *in
	if in.EnableConfidentialCompute != nil {
		in, out := &in.EnableConfidentialCompute, &out.EnableConfidentialCompute
		*out = new(bool)
		**out = **in
	}
}

func (in *ConfidentialInstanceConfig) DeepCopy() *ConfidentialInstanceConfig {
	if in == nil {
		return nil
	}
	out := new(ConfidentialInstanceConfig)
	in.DeepCopyInto(out)
	return out
}

// --- GceInstanceConfig ---

func (in *GceInstanceConfig) DeepCopyInto(out *GceInstanceConfig) {
	*out = *in
	if in.BootDiskSizeGB != nil {
		in, out := &in.BootDiskSizeGB, &out.BootDiskSizeGB
		*out = new(int64)
		**out = **in
	}
	if in.ServiceAccountScopes != nil {
		in, out := &in.ServiceAccountScopes, &out.ServiceAccountScopes
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.DisablePublicIPAddresses != nil {
		in, out := &in.DisablePublicIPAddresses, &out.DisablePublicIPAddresses
		*out = new(bool)
		**out = **in
	}
	if in.EnableNestedVirtualization != nil {
		in, out := &in.EnableNestedVirtualization, &out.EnableNestedVirtualization
		*out = new(bool)
		**out = **in
	}
	if in.PoolSize != nil {
		in, out := &in.PoolSize, &out.PoolSize
		*out = new(int64)
		**out = **in
	}
	if in.Tags != nil {
		in, out := &in.Tags, &out.Tags
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.ShieldedInstanceConfig != nil {
		in, out := &in.ShieldedInstanceConfig, &out.ShieldedInstanceConfig
		*out = new(ShieldedInstanceConfig)
		(*in).DeepCopyInto(*out)
	}
	if in.ConfidentialInstanceConfig != nil {
		in, out := &in.ConfidentialInstanceConfig, &out.ConfidentialInstanceConfig
		*out = new(ConfidentialInstanceConfig)
		(*in).DeepCopyInto(*out)
	}
}

func (in *GceInstanceConfig) DeepCopy() *GceInstanceConfig {
	if in == nil {
		return nil
	}
	out := new(GceInstanceConfig)
	in.DeepCopyInto(out)
	return out
}

// --- HostConfig ---

func (in *HostConfig) DeepCopyInto(out *HostConfig) {
	*out = *in
	if in.GceInstance != nil {
		in, out := &in.GceInstance, &out.GceInstance
		*out = new(GceInstanceConfig)
		(*in).DeepCopyInto(*out)
	}
}

func (in *HostConfig) DeepCopy() *HostConfig {
	if in == nil {
		return nil
	}
	out := new(HostConfig)
	in.DeepCopyInto(out)
	return out
}

// --- GcePersistentDisk ---

func (in *GcePersistentDisk) DeepCopyInto(out *GcePersistentDisk) {
	*out = *in
	if in.SizeGB != nil {
		in, out := &in.SizeGB, &out.SizeGB
		*out = new(int64)
		**out = **in
	}
}

func (in *GcePersistentDisk) DeepCopy() *GcePersistentDisk {
	if in == nil {
		return nil
	}
	out := new(GcePersistentDisk)
	in.DeepCopyInto(out)
	return out
}

// --- PersistentDirectory ---

func (in *PersistentDirectory) DeepCopyInto(out *PersistentDirectory) {
	*out = *in
	if in.GcePD != nil {
		in, out := &in.GcePD, &out.GcePD
		*out = new(GcePersistentDisk)
		(*in).DeepCopyInto(*out)
	}
}

func (in *PersistentDirectory) DeepCopy() *PersistentDirectory {
	if in == nil {
		return nil
	}
	out := new(PersistentDirectory)
	in.DeepCopyInto(out)
	return out
}

// --- ReadinessCheck ---

func (in *ReadinessCheck) DeepCopyInto(out *ReadinessCheck) {
	*out = *in
}

func (in *ReadinessCheck) DeepCopy() *ReadinessCheck {
	if in == nil {
		return nil
	}
	out := new(ReadinessCheck)
	in.DeepCopyInto(out)
	return out
}

// --- CryptoKeyRef ---

func (in *CryptoKeyRef) DeepCopyInto(out *CryptoKeyRef) {
	*out = *in
}

func (in *CryptoKeyRef) DeepCopy() *CryptoKeyRef {
	if in == nil {
		return nil
	}
	out := new(CryptoKeyRef)
	in.DeepCopyInto(out)
	return out
}

// --- EncryptionKeyConfig ---

func (in *EncryptionKeyConfig) DeepCopyInto(out *EncryptionKeyConfig) {
	*out = *in
	if in.KmsCryptoKeyRef != nil {
		in, out := &in.KmsCryptoKeyRef, &out.KmsCryptoKeyRef
		*out = new(CryptoKeyRef)
		**out = **in
	}
	if in.ServiceAccountRef != nil {
		in, out := &in.ServiceAccountRef, &out.ServiceAccountRef
		*out = new(GceServiceAccountRef)
		**out = **in
	}
}

func (in *EncryptionKeyConfig) DeepCopy() *EncryptionKeyConfig {
	if in == nil {
		return nil
	}
	out := new(EncryptionKeyConfig)
	in.DeepCopyInto(out)
	return out
}

// --- PortRange ---

func (in *PortRange) DeepCopyInto(out *PortRange) {
	*out = *in
}

func (in *PortRange) DeepCopy() *PortRange {
	if in == nil {
		return nil
	}
	out := new(PortRange)
	in.DeepCopyInto(out)
	return out
}

// --- EphemeralPersistentDisk ---

func (in *EphemeralPersistentDisk) DeepCopyInto(out *EphemeralPersistentDisk) {
	*out = *in
	if in.ReadOnly != nil {
		in, out := &in.ReadOnly, &out.ReadOnly
		*out = new(bool)
		**out = **in
	}
}

func (in *EphemeralPersistentDisk) DeepCopy() *EphemeralPersistentDisk {
	if in == nil {
		return nil
	}
	out := new(EphemeralPersistentDisk)
	in.DeepCopyInto(out)
	return out
}

// --- EphemeralDirectory ---

func (in *EphemeralDirectory) DeepCopyInto(out *EphemeralDirectory) {
	*out = *in
	if in.GcePD != nil {
		in, out := &in.GcePD, &out.GcePD
		*out = new(EphemeralPersistentDisk)
		(*in).DeepCopyInto(*out)
	}
}

func (in *EphemeralDirectory) DeepCopy() *EphemeralDirectory {
	if in == nil {
		return nil
	}
	out := new(EphemeralDirectory)
	in.DeepCopyInto(out)
	return out
}

// --- WorkstationConfigSpec ---

func (in *WorkstationConfigSpec) DeepCopyInto(out *WorkstationConfigSpec) {
	*out = *in
	out.ClusterRef = in.ClusterRef
	if in.ReplicaZones != nil {
		in, out := &in.ReplicaZones, &out.ReplicaZones
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	if in.Container != nil {
		in, out := &in.Container, &out.Container
		*out = new(ContainerConfig)
		(*in).DeepCopyInto(*out)
	}
	if in.Host != nil {
		in, out := &in.Host, &out.Host
		*out = new(HostConfig)
		(*in).DeepCopyInto(*out)
	}
	if in.PersistentDirectories != nil {
		in, out := &in.PersistentDirectories, &out.PersistentDirectories
		*out = make([]PersistentDirectory, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.ReadinessChecks != nil {
		in, out := &in.ReadinessChecks, &out.ReadinessChecks
		*out = make([]ReadinessCheck, len(*in))
		copy(*out, *in)
	}
	if in.EncryptionKey != nil {
		in, out := &in.EncryptionKey, &out.EncryptionKey
		*out = new(EncryptionKeyConfig)
		(*in).DeepCopyInto(*out)
	}
	if in.Labels != nil {
		in, out := &in.Labels, &out.Labels
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.Annotations != nil {
		in, out := &in.Annotations, &out.Annotations
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.DisableSsh != nil {
		in, out := &in.DisableSsh, &out.DisableSsh
		*out = new(bool)
		**out = **in
	}
	if in.AllowedPorts != nil {
		in, out := &in.AllowedPorts, &out.AllowedPorts
		*out = make([]PortRange, len(*in))
		copy(*out, *in)
	}
	if in.EphemeralDirectories != nil {
		in, out := &in.EphemeralDirectories, &out.EphemeralDirectories
		*out = make([]EphemeralDirectory, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.Admins != nil {
		in, out := &in.Admins, &out.Admins
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

func (in *WorkstationConfigSpec) DeepCopy() *WorkstationConfigSpec {
	if in == nil {
		return nil
	}
	out := new(WorkstationConfigSpec)
	in.DeepCopyInto(out)
	return out
}

// --- WorkstationConfigStatus ---

func (in *WorkstationConfigPostureStatus) DeepCopyInto(out *WorkstationConfigPostureStatus) {
	*out = *in
	if in.AllowedPorts != nil {
		in, out := &in.AllowedPorts, &out.AllowedPorts
		*out = make([]PortRange, len(*in))
		copy(*out, *in)
	}
	if in.EphemeralDirectories != nil {
		in, out := &in.EphemeralDirectories, &out.EphemeralDirectories
		*out = make([]EphemeralDirectory, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

func (in *WorkstationConfigPostureStatus) DeepCopy() *WorkstationConfigPostureStatus {
	if in == nil {
		return nil
	}
	out := new(WorkstationConfigPostureStatus)
	in.DeepCopyInto(out)
	return out
}

func (in *WorkstationConfigStatus) DeepCopyInto(out *WorkstationConfigStatus) {
	*out = *in
	if in.Posture != nil {
		in, out := &in.Posture, &out.Posture
		*out = new(WorkstationConfigPostureStatus)
		(*in).DeepCopyInto(*out)
	}
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]metav1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.KCC != nil {
		in, out := &in.KCC, &out.KCC
		*out = (*in).DeepCopy()
	}
}

func (in *WorkstationConfigStatus) DeepCopy() *WorkstationConfigStatus {
	if in == nil {
		return nil
	}
	out := new(WorkstationConfigStatus)
	in.DeepCopyInto(out)
	return out
}

// --- WorkstationConfig ---

func (in *WorkstationConfig) DeepCopyInto(out *WorkstationConfig) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

func (in *WorkstationConfig) DeepCopy() *WorkstationConfig {
	if in == nil {
		return nil
	}
	out := new(WorkstationConfig)
	in.DeepCopyInto(out)
	return out
}

func (in *WorkstationConfig) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// --- WorkstationConfigList ---

func (in *WorkstationConfigList) DeepCopyInto(out *WorkstationConfigList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]WorkstationConfig, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

func (in *WorkstationConfigList) DeepCopy() *WorkstationConfigList {
	if in == nil {
		return nil
	}
	out := new(WorkstationConfigList)
	in.DeepCopyInto(out)
	return out
}

func (in *WorkstationConfigList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// --- ConfigRef ---

func (in *ConfigRef) DeepCopyInto(out *ConfigRef) {
	*out = *in
}

func (in *ConfigRef) DeepCopy() *ConfigRef {
	if in == nil {
		return nil
	}
	out := new(ConfigRef)
	in.DeepCopyInto(out)
	return out
}

// --- WorkstationSpec ---

func (in *WorkstationSpec) DeepCopyInto(out *WorkstationSpec) {
	*out = *in
	out.ConfigRef = in.ConfigRef
	if in.Labels != nil {
		in, out := &in.Labels, &out.Labels
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.Annotations != nil {
		in, out := &in.Annotations, &out.Annotations
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.Users != nil {
		in, out := &in.Users, &out.Users
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
}

func (in *WorkstationSpec) DeepCopy() *WorkstationSpec {
	if in == nil {
		return nil
	}
	out := new(WorkstationSpec)
	in.DeepCopyInto(out)
	return out
}

// --- WorkstationStatus ---

func (in *WorkstationStatus) DeepCopyInto(out *WorkstationStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]metav1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.KCC != nil {
		in, out := &in.KCC, &out.KCC
		*out = (*in).DeepCopy()
	}
}

func (in *WorkstationStatus) DeepCopy() *WorkstationStatus {
	if in == nil {
		return nil
	}
	out := new(WorkstationStatus)
	in.DeepCopyInto(out)
	return out
}

// --- Workstation ---

func (in *Workstation) DeepCopyInto(out *Workstation) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

func (in *Workstation) DeepCopy() *Workstation {
	if in == nil {
		return nil
	}
	out := new(Workstation)
	in.DeepCopyInto(out)
	return out
}

func (in *Workstation) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// --- WorkstationList ---

func (in *WorkstationList) DeepCopyInto(out *WorkstationList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Workstation, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

func (in *WorkstationList) DeepCopy() *WorkstationList {
	if in == nil {
		return nil
	}
	out := new(WorkstationList)
	in.DeepCopyInto(out)
	return out
}

func (in *WorkstationList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}
