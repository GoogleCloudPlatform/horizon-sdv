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

package controller

import (
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"mtk-connect-module/internal/profiles"
)

const (
	// KindDevice is the kind of the device custom resource.
	KindDevice = "MTKConnectDevice"

	// FinalizerDeviceCleanup makes the controller remove the device from MTK Connect before the resource goes away.
	FinalizerDeviceCleanup = "mtkconnect.horizon-sdv.io/device-cleanup"

	maxDeviceIndex = 64
)

// DeviceGVK identifies MTKConnectDevice.
var DeviceGVK = schema.GroupVersionKind{Group: Group, Version: Version, Kind: KindDevice}

// Device phases reported in status.phase.
const (
	DevPending    = "Pending"
	DevConfigured = "Configured"
	DevDeleting   = "Deleting"
	DevFailed     = "Failed"
)

// Device condition types.
const (
	DevCondSpecValid     = "SpecValid"
	DevCondInstanceReady = "InstanceReady"
	DevCondTestbench     = "TestbenchReady"
	DevCondConfigured    = "DeviceConfigured"
)

var deviceConditionOrder = []string{DevCondSpecValid, DevCondInstanceReady, DevCondTestbench, DevCondConfigured}

// DeviceAdb is the adb connection of an Adb device.
type DeviceAdb struct {
	Host             string `json:"host,omitempty"`
	Port             int64  `json:"port,omitempty"`
	TunnelCallerPort int64  `json:"tunnelCallerPort,omitempty"`
}

// DeviceHost holds the host terminal settings.
type DeviceHost struct {
	TerminalUser      string `json:"terminalUser,omitempty"`
	LaunchApplication string `json:"launchApplication,omitempty"`
}

// DeviceTunnel is one raw TCP tunnel.
type DeviceTunnel struct {
	Name string `json:"name,omitempty"`
	Port int64  `json:"port,omitempty"`
}

// DeviceSpec is the spec of MTKConnectDevice. testbenchRef, index and profile cannot change.
type DeviceSpec struct {
	TestbenchRef string         `json:"testbenchRef,omitempty"`
	Index        int64          `json:"index,omitempty"`
	Name         string         `json:"name,omitempty"`
	Profile      string         `json:"profile,omitempty"`
	Adb          *DeviceAdb     `json:"adb,omitempty"`
	Host         DeviceHost     `json:"host,omitempty"`
	TunnelHost   string         `json:"tunnelHost,omitempty"`
	Tunnels      []DeviceTunnel `json:"tunnels,omitempty"`
}

// profileDevice converts the spec to the description the payload builder works from.
func (s *DeviceSpec) profileDevice() profiles.Device {
	d := profiles.Device{
		Profile:           s.Profile,
		TerminalUser:      s.Host.TerminalUser,
		LaunchApplication: s.Host.LaunchApplication,
		TunnelHost:        s.TunnelHost,
	}
	if s.Adb != nil {
		d.AdbHost = s.Adb.Host
		d.AdbPort = int(s.Adb.Port)
		d.TunnelCallerPort = int(s.Adb.TunnelCallerPort)
	}
	for _, t := range s.Tunnels {
		d.Tunnels = append(d.Tunnels, profiles.Tunnel{Name: t.Name, Port: int(t.Port)})
	}
	return d
}

func (s *DeviceSpec) validate() error {
	if strings.TrimSpace(s.TestbenchRef) == "" {
		return fmt.Errorf("spec.testbenchRef is required")
	}
	if s.Index < 1 || s.Index > maxDeviceIndex {
		return fmt.Errorf("spec.index %d must be between 1 and %d", s.Index, maxDeviceIndex)
	}
	if err := s.profileDevice().Validate(); err != nil {
		return fmt.Errorf("spec: %w", err)
	}
	return nil
}

// parseDeviceSpec reads, defaults and validates the spec of an MTKConnectDevice.
func parseDeviceSpec(obj *unstructured.Unstructured) (*DeviceSpec, error) {
	raw, found, err := unstructured.NestedMap(obj.Object, "spec")
	if err != nil {
		return nil, fmt.Errorf("read spec: %w", err)
	}
	if !found {
		return nil, fmt.Errorf("spec is missing")
	}
	var spec DeviceSpec
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(raw, &spec); err != nil {
		return nil, fmt.Errorf("decode spec: %w", err)
	}
	spec.TestbenchRef = strings.TrimSpace(spec.TestbenchRef)
	spec.Name = orDefault(spec.Name, fmt.Sprintf("%s %d", spec.TestbenchRef, spec.Index))
	if err := spec.validate(); err != nil {
		return nil, err
	}
	return &spec, nil
}
