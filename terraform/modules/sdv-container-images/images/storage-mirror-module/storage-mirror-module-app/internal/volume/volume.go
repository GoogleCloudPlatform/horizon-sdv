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

// Package volume builds the Filestore instance, static PersistentVolumes and claims for the mirror share.
package volume

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	smv1 "storage-mirror-module/api/v1alpha1"
)

const (
	// FilestoreInstanceName is the KCC object name in the controller namespace.
	FilestoreInstanceName = "storage-mirror"
	// StorageClassName is a binding key only; no provisioner serves it.
	StorageClassName = "storage-mirror-static"
	CSIDriver        = "filestore.csi.storage.gke.io"

	AccessRW = "rw"
	AccessRO = "ro"

	// ClaimRequestGb is the storage request of every consumer claim. The claims bind by volumeName to a static PV,
	// so the request is only a lower bound for binding; Kubernetes does not resize statically bound claims. The PV
	// carries the share size and follows resizes.
	ClaimRequestGb = 1024
)

// FilestoreGVK is the Config Connector Filestore kind.
var FilestoreGVK = schema.GroupVersionKind{Group: "filestore.cnrm.cloud.google.com", Version: "v1beta1", Kind: "FilestoreInstance"}

// Consumer is one claim on the share.
type Consumer struct {
	PVName    string
	Namespace string
	ClaimName string
	ReadOnly  bool
}

// InstanceID is the Filestore instance name in GCP.
func InstanceID(v smv1.VolumeSpec) string {
	if v.ExistingInstance != "" {
		return v.ExistingInstance
	}
	if v.ResourceID != "" {
		return v.ResourceID
	}
	return "sdv-storage-mirror"
}

// ShareName is the Filestore file share name.
func ShareName(v smv1.VolumeSpec) string {
	if v.ShareName != "" {
		return v.ShareName
	}
	return "mirror"
}

// Consumers returns the claims to maintain for the config.
func Consumers(spec smv1.StorageMirrorConfigSpec) []Consumer {
	wf := spec.Consumers.Workflows
	ns := defaultString(wf.Namespace, "workflows")
	out := []Consumer{
		{PVName: "storage-mirror-workflows-rw", Namespace: ns, ClaimName: defaultString(wf.RWClaimName, "storage-mirror-rw")},
		{PVName: "storage-mirror-workflows-ro", Namespace: ns, ClaimName: defaultString(wf.ROClaimName, "storage-mirror-ro"), ReadOnly: true},
	}
	if j := spec.Consumers.Jenkins; j.Enabled {
		out = append(out, Consumer{
			PVName:    "storage-mirror-jenkins",
			Namespace: defaultString(j.Namespace, "jenkins"),
			ClaimName: defaultString(j.ClaimName, "sdv-mirror-filestore-pvc"),
			ReadOnly:  j.ReadOnly,
		})
	}
	return out
}

// FilestoreInstance renders the KCC object. deletion-policy abandon keeps the instance when the CR goes away.
func FilestoreInstance(namespace, projectID string, v smv1.VolumeSpec) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(FilestoreGVK)
	u.SetName(FilestoreInstanceName)
	u.SetNamespace(namespace)
	u.SetLabels(map[string]string{smv1.LabelModule: smv1.ModuleName})
	u.SetAnnotations(map[string]string{
		"cnrm.cloud.google.com/project-id":      projectID,
		"cnrm.cloud.google.com/deletion-policy": "abandon",
	})
	network := map[string]interface{}{
		"networkRef": map[string]interface{}{"external": fmt.Sprintf("projects/%s/global/networks/%s", projectID, v.Network)},
		"modes":      []interface{}{"MODE_IPV4"},
	}
	if v.ReservedIPRange != "" {
		network["reservedIpRange"] = v.ReservedIPRange
	}
	tier := v.Tier
	if tier == "" {
		tier = "ZONAL"
	}
	u.Object["spec"] = map[string]interface{}{
		"resourceID": InstanceID(v),
		"location":   v.Zone,
		"tier":       tier,
		"fileShares": []interface{}{map[string]interface{}{"name": ShareName(v), "capacityGb": v.CapacityGb}},
		"networks":   []interface{}{network},
	}
	return u
}

// InstanceCapacityGb reads spec.fileShares[0].capacityGb.
func InstanceCapacityGb(u *unstructured.Unstructured) int64 {
	shares, _, _ := unstructured.NestedSlice(u.Object, "spec", "fileShares")
	if len(shares) == 0 {
		return 0
	}
	m, _ := shares[0].(map[string]interface{})
	switch n := m["capacityGb"].(type) {
	case int64:
		return n
	case float64:
		return int64(n)
	case int:
		return int64(n)
	}
	return 0
}

// SetInstanceCapacityGb sets spec.fileShares[0].capacityGb.
func SetInstanceCapacityGb(u *unstructured.Unstructured, gb int64) {
	shares, _, _ := unstructured.NestedSlice(u.Object, "spec", "fileShares")
	if len(shares) == 0 {
		return
	}
	m, _ := shares[0].(map[string]interface{})
	m["capacityGb"] = gb
	shares[0] = m
	_ = unstructured.SetNestedSlice(u.Object, shares, "spec", "fileShares")
}

// InstanceReady reports the KCC Ready condition.
func InstanceReady(u *unstructured.Unstructured) bool {
	conds, _, _ := unstructured.NestedSlice(u.Object, "status", "conditions")
	for _, c := range conds {
		m, _ := c.(map[string]interface{})
		if m["type"] == "Ready" && m["status"] == "True" {
			return true
		}
	}
	return false
}

// InstanceIP returns the first IPv4 address KCC reports for the instance. Field placement differs between
// KCC releases (status.observedState, status, or output-only spec fields), so every known location is checked.
func InstanceIP(u *unstructured.Unstructured) string {
	paths := [][]string{
		{"status", "observedState", "networks"},
		{"status", "networks"},
		{"spec", "networks"},
	}
	for _, p := range paths {
		nets, _, _ := unstructured.NestedSlice(u.Object, p...)
		for _, n := range nets {
			m, _ := n.(map[string]interface{})
			ips, _ := m["ipAddresses"].([]interface{})
			for _, ip := range ips {
				if s, ok := ip.(string); ok && s != "" {
					return s
				}
			}
		}
	}
	return ""
}

// PersistentVolume renders the static PV for a consumer, pre-bound to its claim.
func PersistentVolume(c Consumer, v smv1.VolumeSpec, ip string) *corev1.PersistentVolume {
	access := corev1.ReadWriteMany
	accessLabel := AccessRW
	if c.ReadOnly {
		access = corev1.ReadOnlyMany
		accessLabel = AccessRO
	}
	return &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:   c.PVName,
			Labels: map[string]string{smv1.LabelModule: smv1.ModuleName, smv1.LabelAccess: accessLabel},
		},
		Spec: corev1.PersistentVolumeSpec{
			Capacity:                      corev1.ResourceList{corev1.ResourceStorage: Quantity(v.CapacityGb)},
			AccessModes:                   []corev1.PersistentVolumeAccessMode{access},
			PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimRetain,
			StorageClassName:              StorageClassName,
			ClaimRef:                      &corev1.ObjectReference{Kind: "PersistentVolumeClaim", APIVersion: "v1", Namespace: c.Namespace, Name: c.ClaimName},
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				CSI: &corev1.CSIPersistentVolumeSource{
					Driver:           CSIDriver,
					VolumeHandle:     fmt.Sprintf("modeInstance/%s/%s/%s", v.Zone, InstanceID(v), ShareName(v)),
					VolumeAttributes: map[string]string{"ip": ip, "volume": ShareName(v)},
					ReadOnly:         c.ReadOnly,
				},
			},
		},
	}
}

// TestNFS replaces the Filestore instance with a plain NFS export (kind end-to-end tests only; see
// tools/storage-mirror-e2e). Never set in a platform deployment.
type TestNFS struct {
	Server string
	Path   string
}

// PersistentVolumeTestNFS renders the consumer PV on a plain NFS export instead of the Filestore CSI driver.
func PersistentVolumeTestNFS(c Consumer, v smv1.VolumeSpec, nfs TestNFS) *corev1.PersistentVolume {
	pv := PersistentVolume(c, v, nfs.Server)
	path := nfs.Path
	if path == "" {
		path = "/"
	}
	pv.Spec.PersistentVolumeSource = corev1.PersistentVolumeSource{
		NFS: &corev1.NFSVolumeSource{Server: nfs.Server, Path: path, ReadOnly: c.ReadOnly},
	}
	pv.Labels["storagemirror.horizon-sdv.io/test-volume"] = "nfs"
	return pv
}

// PersistentVolumeClaim renders the claim bound to the consumer PV by name.
func PersistentVolumeClaim(c Consumer) *corev1.PersistentVolumeClaim {
	access := corev1.ReadWriteMany
	if c.ReadOnly {
		access = corev1.ReadOnlyMany
	}
	sc := StorageClassName
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      c.ClaimName,
			Namespace: c.Namespace,
			Labels:    map[string]string{smv1.LabelModule: smv1.ModuleName},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes:      []corev1.PersistentVolumeAccessMode{access},
			StorageClassName: &sc,
			VolumeName:       c.PVName,
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: Quantity(ClaimRequestGb)},
			},
		},
	}
}

// Quantity converts GiB to a resource quantity.
func Quantity(gb int64) resource.Quantity {
	return resource.MustParse(fmt.Sprintf("%dGi", gb))
}

// ValidateCapacity applies the zonal tier steps on top of the CRD validation.
func ValidateCapacity(gb int64) error {
	if gb < 1024 || gb > 102400 {
		return fmt.Errorf("capacityGb %d outside 1024..102400", gb)
	}
	if gb < 10240 {
		if gb%256 != 0 {
			return fmt.Errorf("capacityGb %d must be a multiple of 256 below 10 TiB", gb)
		}
		if gb > 9984 {
			return fmt.Errorf("capacityGb %d: zonal tier stops at 9984 GiB below 10 TiB", gb)
		}
		return nil
	}
	if gb%2560 != 0 {
		return fmt.Errorf("capacityGb %d must be a multiple of 2560 from 10 TiB", gb)
	}
	return nil
}

func defaultString(v, d string) string {
	if v == "" {
		return d
	}
	return v
}
