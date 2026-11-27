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

package dispatch

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestParamOrderMatchesChart keeps the controller and the Sensors (index-mapped) on the same parameter order.
func TestParamOrderMatchesChart(t *testing.T) {
	dir, _ := os.Getwd()
	var helpers string
	for i := 0; i < 12 && dir != "/"; i++ {
		p := filepath.Join(dir, "gitops", "modules", "storage-mirror-module", "argo-workflows", "templates", "_helpers.tpl")
		if _, err := os.Stat(p); err == nil {
			helpers = p
			break
		}
		dir = filepath.Dir(dir)
	}
	if helpers == "" {
		t.Skip("chart not found (image build context)")
	}
	b, err := os.ReadFile(helpers)
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string][]string{"mirror": MirrorParams, "delete": DeleteParams, "volume-init": VolumeInitParams} {
		re := regexp.MustCompile(`define "smw\.params\.` + regexp.QuoteMeta(name) + `" -}}\s*\n([^\n{]+)`)
		m := re.FindSubmatch(b)
		if m == nil {
			t.Fatalf("smw.params.%s not found", name)
		}
		if got := strings.Fields(string(m[1])); strings.Join(got, " ") != strings.Join(want, " ") {
			t.Errorf("%s: chart %v, controller %v", name, got, want)
		}
	}
}
