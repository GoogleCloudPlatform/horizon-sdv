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
	"strings"
	"testing"

	pb "cloud.google.com/go/workstations/apiv1/workstationspb"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	wsv1alpha1 "cloud-workstations-module/api/v1alpha1"
)

func TestDesiredPostureDefaults(t *testing.T) {
	got := DesiredPosture(wsv1alpha1.WorkstationConfigSpec{})
	if !got.DisableSsh {
		t.Fatal("nil disableSsh must default to true")
	}
	if len(got.AllowedPorts) != 2 || got.AllowedPorts[0] != (wsv1alpha1.PortRange{First: 80, Last: 80}) || got.AllowedPorts[1] != (wsv1alpha1.PortRange{First: 1024, Last: 65535}) {
		t.Fatalf("default ports = %+v", got.AllowedPorts)
	}
	if len(got.EphemeralDirectories) != 0 {
		t.Fatalf("default ephemeral = %+v", got.EphemeralDirectories)
	}
}

func TestDesiredPostureKeepsExplicitFalseAndEmptyEphemeralFields(t *testing.T) {
	disabled := false
	readOnly := true
	got := DesiredPosture(wsv1alpha1.WorkstationConfigSpec{
		DisableSsh:   &disabled,
		AllowedPorts: []wsv1alpha1.PortRange{{First: 22, Last: 22}},
		EphemeralDirectories: []wsv1alpha1.EphemeralDirectory{{
			MountPath: "/tmp",
			GcePD:     &wsv1alpha1.EphemeralPersistentDisk{SourceSnapshot: "snap", ReadOnly: &readOnly},
		}},
	})
	if got.DisableSsh {
		t.Fatal("explicit false disableSsh was overwritten")
	}
	if got.EphemeralDirectories[0].DiskType != nil {
		t.Fatal("unset disk type must stay nil so the server default is not a diff")
	}
	if got.EphemeralDirectories[0].SourceSnapshot == nil || *got.EphemeralDirectories[0].SourceSnapshot != "snap" {
		t.Fatalf("source snapshot = %+v", got.EphemeralDirectories[0].SourceSnapshot)
	}
}

func TestBuildPostureUpdateRejectsKCCOwnedMask(t *testing.T) {
	err := ValidateUpdateMask(&fieldmaskpb.FieldMask{Paths: []string{"idle_timeout"}})
	if err == nil || !strings.Contains(err.Error(), "idle_timeout") {
		t.Fatalf("expected idle_timeout rejection, got %v", err)
	}
	err = ValidateUpdateMask(&fieldmaskpb.FieldMask{Paths: []string{"host.gce_instance.machine_type"}})
	if err == nil {
		t.Fatal("machine_type mask was accepted")
	}
}

func TestBuildPostureUpdatePopulatesOnlyPostureFields(t *testing.T) {
	readOnly := false
	disk := "pd-balanced"
	req, err := BuildPostureUpdate("projects/p/locations/l/workstationClusters/c/workstationConfigs/cfg", "etag-1", Posture{
		DisableSsh:   true,
		AllowedPorts: []wsv1alpha1.PortRange{{First: 1024, Last: 65535}, {First: 80, Last: 80}},
		EphemeralDirectories: []EphemeralDirectory{{
			MountPath: "/tmp",
			DiskType:  &disk,
			ReadOnly:  &readOnly,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	gotMask := append([]string(nil), req.GetUpdateMask().GetPaths()...)
	wantMask := []string{MaskDisableSSH, MaskAllowedPorts, MaskEphemeralDirectories}
	if strings.Join(gotMask, ",") != strings.Join(wantMask, ",") {
		t.Fatalf("mask = %v", gotMask)
	}
	cfg := req.GetWorkstationConfig()
	if cfg.GetEtag() != "etag-1" || !cfg.GetHost().GetGceInstance().GetDisableSsh() {
		t.Fatalf("config identity/posture = %s disable=%v", cfg.GetName(), cfg.GetHost().GetGceInstance().GetDisableSsh())
	}
	if cfg.GetAllowedPorts()[0].GetFirst() != 80 || cfg.GetAllowedPorts()[1].GetFirst() != 1024 {
		t.Fatalf("ports were not normalized: %+v", cfg.GetAllowedPorts())
	}
	for _, path := range populatedFieldPaths(cfg.ProtoReflect(), "") {
		if !posturePathAllowed(path) {
			t.Errorf("populated field %q is outside the posture allowlist", path)
		}
	}
	if cfg.GetIdleTimeout() != nil || cfg.GetContainer() != nil || cfg.GetHost().GetGceInstance().GetMachineType() != "" {
		t.Fatal("KCC-owned fields were populated on the update")
	}
}

func TestPostureMatchesIgnoresServerDiskDefaultAndPortOrder(t *testing.T) {
	live := &pb.WorkstationConfig{
		Host: &pb.WorkstationConfig_Host{Config: &pb.WorkstationConfig_Host_GceInstance_{
			GceInstance: &pb.WorkstationConfig_Host_GceInstance{DisableSsh: true},
		}},
		AllowedPorts: []*pb.WorkstationConfig_PortRange{{First: 1024, Last: 65535}, {First: 80, Last: 80}},
		EphemeralDirectories: []*pb.WorkstationConfig_EphemeralDirectory{{
			MountPath: "/tmp",
			DirectoryType: &pb.WorkstationConfig_EphemeralDirectory_GcePd{GcePd: &pb.WorkstationConfig_EphemeralDirectory_GcePersistentDisk{
				DiskType: "pd-standard",
			}},
		}},
	}
	desired := DesiredPosture(wsv1alpha1.WorkstationConfigSpec{
		AllowedPorts:         []wsv1alpha1.PortRange{{First: 80, Last: 80}, {First: 1024, Last: 65535}},
		EphemeralDirectories: []wsv1alpha1.EphemeralDirectory{{MountPath: "/tmp"}},
	})
	if !PostureMatches(live, desired) {
		t.Fatal("server disk default and port order were treated as drift")
	}
	desired.EphemeralDirectories[0].DiskType = proto.String("pd-ssd")
	if PostureMatches(live, desired) {
		t.Fatal("explicit disk type mismatch was ignored")
	}
}

func posturePathAllowed(path string) bool {
	switch path {
	case "name", "etag", "host", "host.gce_instance", "host.gce_instance.disable_ssh", "allowed_ports", "ephemeral_directories":
		return true
	default:
		return false
	}
}

func populatedFieldPaths(msg protoreflect.Message, prefix string) []string {
	var out []string
	msg.Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		path := string(fd.Name())
		if prefix != "" {
			path = prefix + "." + path
		}
		out = append(out, path)
		if fd.Kind() == protoreflect.MessageKind && !fd.IsList() && !fd.IsMap() {
			out = append(out, populatedFieldPaths(v.Message(), path)...)
		}
		return true
	})
	return out
}
