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

package profiles

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// The golden files in testdata/ were recorded by running workloads/common/mtk-connect/create-testbench.js
// against a mock MTK Connect with the environment listed in each file. Each case below is the same
// configuration written as devices.

const launchPlaceholder = "${MTK_CONNECT_LAUNCH_APPLICATION_NAME}"

type goldenDevice struct {
	Index     int                    `json:"index"`
	Name      string                 `json:"name"`
	Interface map[string]interface{} `json:"interface"`
}

type golden struct {
	Scenario string            `json:"scenario"`
	Env      map[string]string `json:"env"`
	Devices  []goldenDevice    `json:"devices"`
}

type namedDevice struct {
	name string
	dev  Device
}

var (
	tunnels2   = []Tunnel{{Name: "someip", Port: 30490}, {Name: "doip", Port: 13400}}
	tunnels1   = []Tunnel{{Name: "someip", Port: 30490}}
	hostOnly   = func(d Device) Device { d.Profile = ProfileHostOnly; d.TunnelHost = "10.0.0.1"; return d }
	adbDevice  = func(host string, port int) Device { return Device{Profile: ProfileAdb, AdbHost: host, AdbPort: port} }
	withUser   = func(d Device, u string) Device { d.TerminalUser = u; return d }
	withTunnel = func(d Device, t []Tunnel) Device { d.Tunnels = t; return d }
)

func cases() map[string][]namedDevice {
	return map[string][]namedDevice{
		"host-only-default-user":         {{"AAOS 1", hostOnly(Device{})}},
		"host-only-terminal-user":        {{"AAOS 1", withUser(hostOnly(Device{}), "jenkins")}},
		"host-only-launch-application":   {{"AAOS 1", hostOnly(Device{LaunchApplication: "openbsw-app --demo"})}},
		"host-only-tunnels":              {{"AAOS 1", withTunnel(hostOnly(Device{}), tunnels2)}},
		"host-only-all":                  {{"AAOS 1", withTunnel(withUser(hostOnly(Device{LaunchApplication: "run.sh"}), "builder"), tunnels1)}},
		"adb-two-devices":                {{"AAOS 1", adbDevice("10.0.0.1", 6520)}, {"AAOS 2", adbDevice("10.0.0.2", 6521)}},
		"adb-terminal-user":              {{"AAOS 1", withUser(adbDevice("10.0.0.1", 6520), "jenkins")}, {"AAOS 2", withUser(adbDevice("10.0.0.2", 6521), "jenkins")}},
		"adb-custom-names":               {{"Phone A", adbDevice("10.0.0.1", 6520)}, {"Phone B", adbDevice("10.0.0.2", 6521)}},
		"adb-custom-tunnel-port":         {{"AAOS 1", Device{Profile: ProfileAdb, AdbHost: "10.0.0.1", AdbPort: 6520, TunnelCallerPort: 9000}}, {"AAOS 2", Device{Profile: ProfileAdb, AdbHost: "10.0.0.2", AdbPort: 6521, TunnelCallerPort: 9000}}},
		"adb-ride-along-tunnels":         {{"AAOS 1", withTunnel(adbDevice("10.0.0.1", 6520), tunnels2)}, {"AAOS 2", adbDevice("10.0.0.2", 6521)}},
		"adb-tunnel-device":              tunnelDevice("builder", tunnels2),
		"adb-tunnel-device-default-user": tunnelDevice("", tunnels1),
	}
}

// tunnelDevice is the dedicated tunnel device at index 1 and the adb devices behind it. The script offsets
// the host, port and name lists by one, so the adb devices at index 2 and 3 take entries 1 and 2.
func tunnelDevice(user string, t []Tunnel) []namedDevice {
	return []namedDevice{
		{"Tunnels", withUser(Device{Profile: ProfileTunnelCarrier, TunnelHost: "10.0.0.1", Tunnels: t}, user)},
		{"AAOS 2", withUser(adbDevice("10.0.0.1", 6520), user)},
		{"AAOS 3", withUser(adbDevice("10.0.0.2", 6521), user)},
	}
}

func roundTrip(t *testing.T, v interface{}) interface{} {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var out interface{}
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestPayloadsMatchScriptOutput(t *testing.T) {
	files, err := filepath.Glob("testdata/*.json")
	if err != nil || len(files) == 0 {
		t.Fatalf("no golden files: %v", err)
	}
	table := cases()
	seen := map[string]bool{}
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		var g golden
		if err := json.Unmarshal(raw, &g); err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		seen[g.Scenario] = true
		t.Run(g.Scenario, func(t *testing.T) {
			want, ok := table[g.Scenario]
			if !ok {
				t.Fatalf("golden scenario %q has no case", g.Scenario)
			}
			if len(want) != len(g.Devices) {
				t.Fatalf("%d devices, script produced %d", len(want), len(g.Devices))
			}
			app := g.Env["MTK_CONNECT_LAUNCH_APPLICATION_NAME"]
			for i, gd := range g.Devices {
				if want[i].name != gd.Name {
					t.Errorf("device %d name = %q, script used %q", gd.Index, want[i].name, gd.Name)
				}
				got, err := want[i].dev.Payload()
				if err != nil {
					t.Fatalf("device %d: %v", gd.Index, err)
				}
				// The script sends the application as the unexpanded variable reference, and the shell that
				// runs the terminal command substitutes the variable's value. Compare what runs.
				expected := roundTrip(t, map[string]interface{}{"interface": gd.Interface})
				b, _ := json.Marshal(expected)
				if app != "" {
					b = []byte(strings.ReplaceAll(string(b), launchPlaceholder, app))
				}
				if err := json.Unmarshal(b, &expected); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(roundTrip(t, got), expected) {
					g1, _ := json.MarshalIndent(roundTrip(t, got), "", "  ")
					g2, _ := json.MarshalIndent(expected, "", "  ")
					t.Errorf("device %d payload differs from the script\n got: %s\nwant: %s", gd.Index, g1, g2)
				}
			}
		})
	}
	for name := range table {
		if !seen[name] {
			t.Errorf("case %q has no golden file", name)
		}
	}
}

func TestCoveredBehaviours(t *testing.T) {
	// The acceptance criteria name these four; make sure the goldens really exercise them.
	need := map[string]string{
		"tunnels":                 "host-only-tunnels",
		"dedicated tunnel device": "adb-tunnel-device",
		"terminal user":           "host-only-terminal-user",
		"launch application":      "host-only-launch-application",
	}
	for what, scenario := range need {
		if _, ok := cases()[scenario]; !ok {
			t.Errorf("no case for %s (%s)", what, scenario)
		}
		if _, err := os.Stat(filepath.Join("testdata", scenario+".json")); err != nil {
			t.Errorf("no golden for %s: %v", what, err)
		}
	}
}

func TestLaunchApplicationRunsOneProgram(t *testing.T) {
	d := Device{Profile: ProfileHostOnly, LaunchApplication: "/opt/app/run.sh --port=8080 -v"}
	p, err := d.Payload()
	if err != nil {
		t.Fatal(err)
	}
	args := p["interface"].(map[string]interface{})["terminal"].(map[string]interface{})["types"].([]interface{})[0].(map[string]interface{})["args"].([]interface{})
	want := `cd /home/builder; su builder -c "eval /opt/app/run.sh --port=8080 -v"`
	if args[1] != want {
		t.Fatalf("args[1] = %q, want %q", args[1], want)
	}

	for _, app := range []string{
		"app; id", "app | id", "app & id", "app && id", "app || id", "app > /tmp/x", "app < /etc/passwd",
		"app $(id)", "app `id`", `app "x"`, "app 'x'", `app \x`, "app\nid", "app (x)", "app {x}", "app *",
		"app ~", "app #x", "app !x", "LD_PRELOAD=/tmp/x.so app", " app", "app ", "app  x",
	} {
		d := Device{Profile: ProfileHostOnly, LaunchApplication: app}
		if err := d.Validate(); err == nil {
			t.Errorf("launchApplication %q: want an error", app)
		}
	}
}

func TestValidate(t *testing.T) {
	ok := Device{Profile: ProfileAdb, AdbHost: "10.0.0.1", AdbPort: 6520}
	bad := map[string]Device{
		"no profile":              {},
		"adb without host":        {Profile: ProfileAdb, AdbPort: 6520},
		"adb port 0":              {Profile: ProfileAdb, AdbHost: "h"},
		"adb launch application":  {Profile: ProfileAdb, AdbHost: "h", AdbPort: 1, LaunchApplication: "x"},
		"host-only with adb":      {Profile: ProfileHostOnly, AdbHost: "h"},
		"carrier without tunnels": {Profile: ProfileTunnelCarrier, TunnelHost: "h"},
		"carrier launch":          {Profile: ProfileTunnelCarrier, TunnelHost: "h", Tunnels: tunnels1, LaunchApplication: "x"},
		"tunnels without host":    {Profile: ProfileHostOnly, Tunnels: tunnels1},
		"bad terminal user":       {Profile: ProfileHostOnly, TerminalUser: "a b; rm"},
		"duplicate tunnel port":   {Profile: ProfileHostOnly, TunnelHost: "h", Tunnels: []Tunnel{{"a", 1}, {"b", 1}}},
		"duplicate tunnel name":   {Profile: ProfileHostOnly, TunnelHost: "h", Tunnels: []Tunnel{{"a", 1}, {"a", 2}}},
		"tunnel port range":       {Profile: ProfileHostOnly, TunnelHost: "h", Tunnels: []Tunnel{{"a", 70000}}},
		"bad tunnel name":         {Profile: ProfileHostOnly, TunnelHost: "h", Tunnels: []Tunnel{{"a:b", 1}}},
		"bad host":                {Profile: ProfileAdb, AdbHost: "h;x", AdbPort: 1},
	}
	if err := ok.Validate(); err != nil {
		t.Fatalf("valid device rejected: %v", err)
	}
	for name, d := range bad {
		if err := d.Validate(); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
	// An adb device carries tunnels to its own host without a separate tunnelHost.
	d := withTunnel(ok, tunnels1)
	if err := d.Validate(); err != nil {
		t.Fatalf("adb ride-along tunnels rejected: %v", err)
	}
}
