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

package volume

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	smv1 "storage-mirror-module/api/v1alpha1"
)

var spec = smv1.VolumeSpec{CapacityGb: 2048, Zone: "europe-west1-d", Network: "sdv-network"}

func TestFilestoreInstanceAbandonAndShape(t *testing.T) {
	u := FilestoreInstance("horizon", "proj", spec)
	if u.GetAnnotations()["cnrm.cloud.google.com/deletion-policy"] != "abandon" {
		t.Fatal("instance must be abandoned on CR delete")
	}
	if id, _, _ := unstructured.NestedString(u.Object, "spec", "resourceID"); id != "sdv-storage-mirror" {
		t.Fatalf("resourceID %q", id)
	}
	if InstanceCapacityGb(u) != 2048 {
		t.Fatal("capacity")
	}
	SetInstanceCapacityGb(u, 2304)
	if InstanceCapacityGb(u) != 2304 {
		t.Fatal("resize")
	}
	adopt := spec
	adopt.ExistingInstance = "pvc-1234"
	if InstanceID(adopt) != "pvc-1234" {
		t.Fatal("existing instance must win")
	}
}

func TestInstanceIPAndReady(t *testing.T) {
	u := FilestoreInstance("horizon", "proj", spec)
	if InstanceIP(u) != "" || InstanceReady(u) {
		t.Fatal("fresh instance has no IP and is not ready")
	}
	_ = unstructured.SetNestedSlice(u.Object, []interface{}{map[string]interface{}{"type": "Ready", "status": "True"}}, "status", "conditions")
	_ = unstructured.SetNestedSlice(u.Object, []interface{}{map[string]interface{}{"ipAddresses": []interface{}{"10.1.2.3"}}}, "status", "observedState", "networks")
	if InstanceIP(u) != "10.1.2.3" || !InstanceReady(u) {
		t.Fatal("ip or ready")
	}
}

func TestConsumersAndPV(t *testing.T) {
	cs := Consumers(smv1.StorageMirrorConfigSpec{Consumers: smv1.ConsumersSpec{Jenkins: smv1.JenkinsConsumer{Enabled: true, ReadOnly: true}}})
	if len(cs) != 3 || cs[2].ClaimName != "sdv-mirror-filestore-pvc" || cs[2].Namespace != "jenkins" || !cs[2].ReadOnly {
		t.Fatalf("consumers %+v", cs)
	}
	pv := PersistentVolume(cs[1], spec, "10.1.2.3")
	if !pv.Spec.CSI.ReadOnly || pv.Spec.AccessModes[0] != corev1.ReadOnlyMany || pv.Spec.PersistentVolumeReclaimPolicy != corev1.PersistentVolumeReclaimRetain {
		t.Fatal("ro pv must be read-only and retained")
	}
	if pv.Spec.CSI.VolumeHandle != "modeInstance/europe-west1-d/sdv-storage-mirror/mirror" || pv.Spec.ClaimRef.Name != "storage-mirror-ro" {
		t.Fatalf("pv %+v", pv.Spec)
	}
	pvc := PersistentVolumeClaim(cs[0])
	if pvc.Spec.VolumeName != "storage-mirror-workflows-rw" || *pvc.Spec.StorageClassName != StorageClassName {
		t.Fatal("pvc binding")
	}
	// The claim request does not follow the share size (static claims are not resizable); the PV does.
	big := spec
	big.CapacityGb = 20480
	if q := pvc.Spec.Resources.Requests[corev1.ResourceStorage]; q.String() != "1Ti" {
		t.Fatalf("claim request %s", q.String())
	}
	if q := PersistentVolume(cs[0], big, "10.1.2.3").Spec.Capacity[corev1.ResourceStorage]; q.String() != "20Ti" {
		t.Fatalf("pv capacity %s", q.String())
	}
}

func TestValidateCapacity(t *testing.T) {
	for gb, ok := range map[int64]bool{1024: true, 2048: true, 1000: false, 1100: false, 9984: true, 10240: true, 10496: false, 12800: true, 102400: true, 102656: false} {
		if err := ValidateCapacity(gb); (err == nil) != ok {
			t.Errorf("%d: %v", gb, err)
		}
	}
}
