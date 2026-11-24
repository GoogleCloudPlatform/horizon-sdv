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

// Package profiles builds the MTK Connect device interface payload for a device profile.
//
// It is a port of configureDevice() in workloads/common/mtk-connect/create-testbench.js. The payloads
// are pinned by golden files recorded from that script (testdata/), so a change here that alters what the
// script sends fails the tests.
package profiles

import (
	"fmt"
	"regexp"
	"strings"
)

// Device profiles.
const (
	// ProfileAdb is an Android device reached over adb (MTK_CONNECT_HOST_ONLY=false).
	ProfileAdb = "Adb"
	// ProfileHostOnly is a device with only a host file system and terminal (MTK_CONNECT_HOST_ONLY=true).
	ProfileHostOnly = "HostOnly"
	// ProfileTunnelCarrier is the dedicated host-only device that carries the raw TCP tunnels of a testbench
	// whose other devices are adb devices (MTK_CONNECT_TUNNEL_DEVICE_NAME).
	ProfileTunnelCarrier = "TunnelCarrier"

	// DefaultTunnelCallerPort is the caller port of the adb tunnel (MTK_CONNECT_TUNNEL_PORT default).
	DefaultTunnelCallerPort = 8555

	maxPort = 65535

	// defaultTerminalCommand is the host terminal command when no terminal user is set: builder, else jenkins.
	defaultTerminalCommand = "[ -d /home/builder ] && { cd /home/builder; su builder; bash --login; } || { cd /home/jenkins; su jenkins; bash --login; }"
)

var (
	userPattern   = regexp.MustCompile(`^[a-z_][a-z0-9_-]{0,31}$`)
	hostPattern   = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,253}$`)
	tunnelPattern = regexp.MustCompile(`^[A-Za-z0-9._-]{1,63}$`)
)

// Tunnel is one raw TCP tunnel to the device host (an entry of MTK_CONNECT_TUNNEL_LIST).
type Tunnel struct {
	Name string
	Port int
}

// Device is everything the payload of one device depends on.
type Device struct {
	Profile string

	// AdbHost, AdbPort and TunnelCallerPort apply to the Adb profile only.
	AdbHost          string
	AdbPort          int
	TunnelCallerPort int

	// TerminalUser is the host terminal login user (MTK_CONNECT_TERMINAL_USER).
	TerminalUser string
	// LaunchApplication runs an application instead of a login shell (MTK_CONNECT_LAUNCH_APPLICATION_NAME).
	// Host-only devices only.
	LaunchApplication string

	// TunnelHost is the host the tunnels connect to. It defaults to AdbHost for the Adb profile.
	TunnelHost string
	Tunnels    []Tunnel
}

// Validate reports a device description that cannot be turned into a payload, or that would carry
// settings the profile ignores.
func (d Device) Validate() error {
	switch d.Profile {
	case ProfileAdb, ProfileHostOnly, ProfileTunnelCarrier:
	default:
		return fmt.Errorf("profile %q must be %s, %s or %s", d.Profile, ProfileAdb, ProfileHostOnly, ProfileTunnelCarrier)
	}

	if d.Profile == ProfileAdb {
		if !hostPattern.MatchString(d.AdbHost) {
			return fmt.Errorf("adb.host %q is not a valid host", d.AdbHost)
		}
		if d.AdbPort < 1 || d.AdbPort > maxPort {
			return fmt.Errorf("adb.port %d must be between 1 and %d", d.AdbPort, maxPort)
		}
		if d.TunnelCallerPort < 0 || d.TunnelCallerPort > maxPort {
			return fmt.Errorf("adb.tunnelCallerPort %d must be between 1 and %d", d.TunnelCallerPort, maxPort)
		}
		if d.LaunchApplication != "" {
			return fmt.Errorf("host.launchApplication is not used by the %s profile", ProfileAdb)
		}
	} else {
		if d.AdbHost != "" || d.AdbPort != 0 || d.TunnelCallerPort != 0 {
			return fmt.Errorf("adb is only used by the %s profile", ProfileAdb)
		}
		if d.Profile == ProfileTunnelCarrier && d.LaunchApplication != "" {
			return fmt.Errorf("host.launchApplication is not used by the %s profile", ProfileTunnelCarrier)
		}
	}

	if d.TerminalUser != "" && !userPattern.MatchString(d.TerminalUser) {
		return fmt.Errorf("host.terminalUser %q is not a valid user name", d.TerminalUser)
	}
	if strings.ContainsRune(d.LaunchApplication, 0) || len(d.LaunchApplication) > 1024 {
		return fmt.Errorf("host.launchApplication must be at most 1024 characters and contain no NUL")
	}

	if d.Profile == ProfileTunnelCarrier && len(d.Tunnels) == 0 {
		return fmt.Errorf("tunnels: the %s profile needs at least one tunnel", ProfileTunnelCarrier)
	}
	if len(d.Tunnels) > 0 {
		if d.tunnelHost() == "" || !hostPattern.MatchString(d.tunnelHost()) {
			return fmt.Errorf("tunnelHost %q is not a valid host", d.tunnelHost())
		}
		names := map[string]bool{}
		ports := map[int]bool{}
		for _, t := range d.Tunnels {
			if !tunnelPattern.MatchString(t.Name) {
				return fmt.Errorf("tunnel name %q is not valid", t.Name)
			}
			if t.Port < 1 || t.Port > maxPort {
				return fmt.Errorf("tunnel %q port %d must be between 1 and %d", t.Name, t.Port, maxPort)
			}
			if names[t.Name] || ports[t.Port] {
				return fmt.Errorf("tunnel %q: names and ports must be unique", t.Name)
			}
			names[t.Name], ports[t.Port] = true, true
		}
	}
	return nil
}

func (d Device) tunnelHost() string {
	if d.TunnelHost != "" {
		return d.TunnelHost
	}
	if d.Profile == ProfileAdb {
		return d.AdbHost
	}
	return ""
}

func (d Device) callerPort() int {
	if d.TunnelCallerPort == 0 {
		return DefaultTunnelCallerPort
	}
	return d.TunnelCallerPort
}

// terminalArgs is the host terminal command of host-only and tunnel-carrier devices.
func (d Device) terminalArgs() []interface{} {
	if d.TerminalUser != "" {
		return []interface{}{"-c", fmt.Sprintf("cd /home/%s; su %s; bash --login", d.TerminalUser, d.TerminalUser), ""}
	}
	return []interface{}{"-c", defaultTerminalCommand, ""}
}

// adbTerminalArgs is the host terminal command of adb devices: a plain login shell unless a user is set.
func (d Device) adbTerminalArgs() []interface{} {
	if d.TerminalUser != "" {
		return d.terminalArgs()
	}
	return []interface{}{"-c", "cd ~/; bash --login", ""}
}

// launchArgs runs the application as builder. The script passes the application through an environment
// variable that the shell expands inside double quotes; quoting it here gives the same command line for any
// application text.
func (d Device) launchArgs() []interface{} {
	return []interface{}{"-c", `cd /home/builder; su builder -c "eval ` + quoteForDoubleQuotes(d.LaunchApplication) + `"`, ""}
}

// quoteForDoubleQuotes escapes the characters a shell treats specially inside double quotes.
func quoteForDoubleQuotes(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '\\', '"', '$', '`':
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

func (d Device) tcpTunnelTypes() []interface{} {
	types := make([]interface{}, 0, len(d.Tunnels))
	for _, t := range d.Tunnels {
		types = append(types, map[string]interface{}{
			"name":   t.Name,
			"driver": "tcp",
			"host":   d.tunnelHost(),
			"port":   t.Port,
			"caller": map[string]interface{}{"port": t.Port},
		})
	}
	return types
}

func hostFS() map[string]interface{} {
	return map[string]interface{}{"types": []interface{}{
		map[string]interface{}{"name": "HOST", "driver": "native", "root": "/root"},
	}}
}

func hostTerminal(args []interface{}) map[string]interface{} {
	return map[string]interface{}{"types": []interface{}{
		map[string]interface{}{"name": "HOST", "driver": "spawn", "command": "bash", "args": args},
	}}
}

// Payload returns the body of PATCH /api/v1/agents/{id}/devices/{index}: {"interface": {...}}.
func (d Device) Payload() (map[string]interface{}, error) {
	if err := d.Validate(); err != nil {
		return nil, err
	}
	var iface map[string]interface{}
	switch d.Profile {
	case ProfileTunnelCarrier:
		iface = map[string]interface{}{
			"fs":       hostFS(),
			"terminal": hostTerminal(d.terminalArgs()),
			"tunnel":   map[string]interface{}{"types": d.tcpTunnelTypes()},
		}
	case ProfileHostOnly:
		args := d.terminalArgs()
		if d.LaunchApplication != "" {
			args = d.launchArgs()
		}
		iface = map[string]interface{}{
			"fs":       hostFS(),
			"terminal": hostTerminal(args),
		}
		if len(d.Tunnels) > 0 {
			iface["tunnel"] = map[string]interface{}{"types": d.tcpTunnelTypes()}
		}
	default:
		tunnels := []interface{}{
			map[string]interface{}{"name": "adb", "driver": "adb", "caller": map[string]interface{}{"port": d.callerPort()}},
		}
		tunnels = append(tunnels, d.tcpTunnelTypes()...)
		iface = map[string]interface{}{
			"adb":    map[string]interface{}{"mode": "tcp", "port": d.AdbPort, "host": d.AdbHost},
			"button": map[string]interface{}{"driver": "adb", "skin": "Default Android"},
			"exec": map[string]interface{}{"types": []interface{}{
				map[string]interface{}{"name": "device", "driver": "adb", "cwd": "/"},
			}},
			"fs": map[string]interface{}{"types": []interface{}{
				map[string]interface{}{"name": "adb", "driver": "adb", "root": "/"},
				map[string]interface{}{"name": "HOST", "driver": "native", "root": "/root"},
			}},
			"log": map[string]interface{}{"types": []interface{}{
				map[string]interface{}{"name": "logcat", "driver": "logcat"},
			}},
			"mjpeg": map[string]interface{}{"types": []interface{}{
				map[string]interface{}{"name": "screen", "driver": "minicap", "scale": 1.0},
			}},
			"terminal": map[string]interface{}{"types": []interface{}{
				map[string]interface{}{"name": "adb", "driver": "adb", "icon": "adb"},
				map[string]interface{}{"name": "HOST", "driver": "spawn", "command": "bash", "args": d.adbTerminalArgs()},
			}},
			"touch":  map[string]interface{}{"driver": "adb", "native": true},
			"tunnel": map[string]interface{}{"types": tunnels},
		}
	}
	return map[string]interface{}{"interface": iface}, nil
}
