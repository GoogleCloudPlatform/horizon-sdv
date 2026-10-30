// Copyright (c) 2026 Accenture, All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package wsapi

import (
	"fmt"
	"sort"

	pb "cloud.google.com/go/workstations/apiv1/workstationspb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	wsv1alpha1 "cloud-workstations-module/api/v1alpha1"
)

const (
	// MaskDisableSSH, MaskAllowedPorts and MaskEphemeralDirectories are the only
	// WorkstationConfig update paths this client may send.
	MaskDisableSSH           = "host.gce_instance.disable_ssh"
	MaskAllowedPorts         = "allowed_ports"
	MaskEphemeralDirectories = "ephemeral_directories"

	// serverDefaultEphemeralDiskType is the disk type Google fills when the user leaves it unset.
	serverDefaultEphemeralDiskType = "pd-standard"
)

// Posture is the API-owned workstation configuration slice.
type Posture struct {
	DisableSsh           bool
	AllowedPorts         []wsv1alpha1.PortRange
	EphemeralDirectories []EphemeralDirectory
}

// EphemeralDirectory is one ephemeral mount. A nil disk type matches an empty
// value or pd-standard. A nil snapshot or source image matches only an empty
// value. A nil read-only flag matches only false.
type EphemeralDirectory struct {
	MountPath      string
	DiskType       *string
	SourceSnapshot *string
	SourceImage    *string
	ReadOnly       *bool
}

// DesiredPosture applies Horizon defaults. A nil disableSsh means true. Nil
// allowed ports means port 80 and 1024-65535.
func DesiredPosture(spec wsv1alpha1.WorkstationConfigSpec) Posture {
	disable := true
	if spec.DisableSsh != nil {
		disable = *spec.DisableSsh
	}
	ports := spec.AllowedPorts
	if ports == nil {
		ports = []wsv1alpha1.PortRange{{First: 80, Last: 80}, {First: 1024, Last: 65535}}
	}
	out := Posture{DisableSsh: disable, AllowedPorts: append([]wsv1alpha1.PortRange(nil), ports...)}
	for _, dir := range spec.EphemeralDirectories {
		item := EphemeralDirectory{MountPath: dir.MountPath}
		if dir.GcePD != nil {
			if dir.GcePD.DiskType != "" {
				v := dir.GcePD.DiskType
				item.DiskType = &v
			}
			if dir.GcePD.SourceSnapshot != "" {
				v := dir.GcePD.SourceSnapshot
				item.SourceSnapshot = &v
			}
			if dir.GcePD.SourceImage != "" {
				v := dir.GcePD.SourceImage
				item.SourceImage = &v
			}
			if dir.GcePD.ReadOnly != nil {
				v := *dir.GcePD.ReadOnly
				item.ReadOnly = &v
			}
		}
		out.EphemeralDirectories = append(out.EphemeralDirectories, item)
	}
	return out
}

// PostureMatches reports whether the live config already has the desired posture.
// Ports are compared as a set. A cleared ephemeral disk field is drift unless
// the live value is the empty or server default described on EphemeralDirectory.
func PostureMatches(live *pb.WorkstationConfig, desired Posture) bool {
	if live == nil || live.GetHost().GetGceInstance().GetDisableSsh() != desired.DisableSsh {
		return false
	}
	if !portsEqual(live.GetAllowedPorts(), desired.AllowedPorts) {
		return false
	}
	return ephemeralEqual(live.GetEphemeralDirectories(), desired.EphemeralDirectories)
}

// BuildPostureUpdate builds an update that carries only the posture fields.
func BuildPostureUpdate(name, etag string, desired Posture) (*pb.UpdateWorkstationConfigRequest, error) {
	mask := &fieldmaskpb.FieldMask{Paths: []string{MaskDisableSSH, MaskAllowedPorts, MaskEphemeralDirectories}}
	if err := ValidateUpdateMask(mask); err != nil {
		return nil, err
	}
	return &pb.UpdateWorkstationConfigRequest{
		WorkstationConfig: postureProto(name, etag, desired),
		UpdateMask:        mask,
	}, nil
}

// ValidateUpdateMask rejects any path outside the posture allowlist.
func ValidateUpdateMask(mask *fieldmaskpb.FieldMask) error {
	if mask == nil || len(mask.GetPaths()) == 0 {
		return fmt.Errorf("update mask is required")
	}
	allowed := map[string]struct{}{
		MaskDisableSSH:           {},
		MaskAllowedPorts:         {},
		MaskEphemeralDirectories: {},
	}
	for _, path := range mask.GetPaths() {
		if _, ok := allowed[path]; !ok {
			return fmt.Errorf("update mask path %q is not an API-owned posture field", path)
		}
	}
	return nil
}

func postureProto(name, etag string, desired Posture) *pb.WorkstationConfig {
	ports := append([]wsv1alpha1.PortRange(nil), desired.AllowedPorts...)
	sort.Slice(ports, func(i, j int) bool {
		if ports[i].First == ports[j].First {
			return ports[i].Last < ports[j].Last
		}
		return ports[i].First < ports[j].First
	})
	pbPorts := make([]*pb.WorkstationConfig_PortRange, 0, len(ports))
	for _, port := range ports {
		pbPorts = append(pbPorts, &pb.WorkstationConfig_PortRange{First: port.First, Last: port.Last})
	}
	dirs := make([]*pb.WorkstationConfig_EphemeralDirectory, 0, len(desired.EphemeralDirectories))
	for _, dir := range desired.EphemeralDirectories {
		item := &pb.WorkstationConfig_EphemeralDirectory{MountPath: dir.MountPath}
		// GcePd is the only directory type this API supports, so it is always attached, even when
		// every override field is nil (the user cleared all overrides). Gating this on "any field
		// set" would omit DirectoryType entirely for a cleared directory, sending an incomplete,
		// ambiguous update instead of the explicit empty-fields payload diskTypeMatches /
		// optionalStringMatches / readOnlyMatches above already treat as "use the server default".
		disk := &pb.WorkstationConfig_EphemeralDirectory_GcePersistentDisk{}
		if dir.DiskType != nil {
			disk.DiskType = *dir.DiskType
		}
		if dir.SourceSnapshot != nil {
			disk.SourceSnapshot = *dir.SourceSnapshot
		}
		if dir.SourceImage != nil {
			disk.SourceImage = *dir.SourceImage
		}
		if dir.ReadOnly != nil {
			disk.ReadOnly = *dir.ReadOnly
		}
		item.DirectoryType = &pb.WorkstationConfig_EphemeralDirectory_GcePd{GcePd: disk}
		dirs = append(dirs, item)
	}
	return &pb.WorkstationConfig{
		Name: name,
		Etag: etag,
		Host: &pb.WorkstationConfig_Host{
			Config: &pb.WorkstationConfig_Host_GceInstance_{
				GceInstance: &pb.WorkstationConfig_Host_GceInstance{DisableSsh: desired.DisableSsh},
			},
		},
		AllowedPorts:         pbPorts,
		EphemeralDirectories: dirs,
	}
}

func portsEqual(live []*pb.WorkstationConfig_PortRange, desired []wsv1alpha1.PortRange) bool {
	if len(live) != len(desired) {
		return false
	}
	left := make([]wsv1alpha1.PortRange, len(live))
	for i, port := range live {
		left[i] = wsv1alpha1.PortRange{First: port.GetFirst(), Last: port.GetLast()}
	}
	right := append([]wsv1alpha1.PortRange(nil), desired...)
	sort.Slice(left, func(i, j int) bool {
		if left[i].First == left[j].First {
			return left[i].Last < left[j].Last
		}
		return left[i].First < left[j].First
	})
	sort.Slice(right, func(i, j int) bool {
		if right[i].First == right[j].First {
			return right[i].Last < right[j].Last
		}
		return right[i].First < right[j].First
	})
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func ephemeralEqual(live []*pb.WorkstationConfig_EphemeralDirectory, desired []EphemeralDirectory) bool {
	if len(live) != len(desired) {
		return false
	}
	byMount := map[string]*pb.WorkstationConfig_EphemeralDirectory{}
	for _, dir := range live {
		byMount[dir.GetMountPath()] = dir
	}
	if len(byMount) != len(live) {
		return false
	}
	// The CRD does not enforce unique mountPath across ephemeralDirectories (no
	// x-kubernetes-list-type: map), so desired can arrive with duplicates. Without this check, two
	// desired entries sharing a mountPath would both be compared against the same live entry,
	// masking drift on whichever one does not match the API server's authoritative single value.
	seenDesired := make(map[string]struct{}, len(desired))
	for _, want := range desired {
		if _, dup := seenDesired[want.MountPath]; dup {
			return false
		}
		seenDesired[want.MountPath] = struct{}{}
	}
	for _, want := range desired {
		got, ok := byMount[want.MountPath]
		if !ok {
			return false
		}
		var liveType, liveSnapshot, liveImage string
		var liveReadOnly bool
		if disk := got.GetGcePd(); disk != nil {
			liveType = disk.GetDiskType()
			liveSnapshot = disk.GetSourceSnapshot()
			liveImage = disk.GetSourceImage()
			liveReadOnly = disk.GetReadOnly()
		}
		if !diskTypeMatches(want.DiskType, liveType) ||
			!optionalStringMatches(want.SourceSnapshot, liveSnapshot) ||
			!optionalStringMatches(want.SourceImage, liveImage) ||
			!readOnlyMatches(want.ReadOnly, liveReadOnly) {
			return false
		}
	}
	return true
}

func diskTypeMatches(want *string, live string) bool {
	if want == nil {
		return live == "" || live == serverDefaultEphemeralDiskType
	}
	return live == *want
}

func optionalStringMatches(want *string, live string) bool {
	if want == nil {
		return live == ""
	}
	return live == *want
}

func readOnlyMatches(want *bool, live bool) bool {
	if want == nil {
		return !live
	}
	return live == *want
}
