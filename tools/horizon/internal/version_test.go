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

package horizon

import (
	"strings"
	"testing"
)

func TestFormatBuildDate(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", "unknown"},
		{"not-a-date", "unknown"},
		{"2026-03-04T15:04:05Z", "04/03/2026"},
	}
	for _, tc := range cases {
		if got := formatBuildDate(tc.in); got != tc.want {
			t.Fatalf("formatBuildDate(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestFormatVersion(t *testing.T) {
	info := versionInfo{Name: "horizon", Version: "1.2.3", BuildDate: "2026-03-04T15:04:05Z"}

	text, err := formatVersion(" Text ", info, false)
	if err != nil {
		t.Fatal(err)
	}
	if text != "horizon 1.2.3 (built 04/03/2026)\n" {
		t.Fatalf("text = %q", text)
	}

	pretty, err := formatVersion("JSON", info, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(pretty, "\n  \"version\": \"1.2.3\"") || !strings.HasSuffix(pretty, "\n") {
		t.Fatalf("pretty json = %q", pretty)
	}

	raw, err := formatVersion("json", info, true)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(raw, "\n  ") {
		t.Fatalf("raw json should be compact, got %q", raw)
	}
	if !strings.Contains(raw, `"buildDate":"2026-03-04T15:04:05Z"`) {
		t.Fatalf("raw json = %q", raw)
	}

	if _, err := formatVersion("yaml", info, false); err == nil {
		t.Fatal("expected unknown output mode")
	}
}

func TestFormatVersionUsesStampedValues(t *testing.T) {
	prevVersion, prevDate := version, buildDate
	t.Cleanup(func() {
		version, buildDate = prevVersion, prevDate
	})
	version = "9.9.9"
	buildDate = "unknown"

	got, err := FormatVersion("", false)
	if err != nil {
		t.Fatal(err)
	}
	if got != "horizon 9.9.9 (built unknown)\n" {
		t.Fatalf("got %q", got)
	}
	if GetCurrentName() != "horizon" || GetCurrentVersion() != "9.9.9" || GetCurrentBuildDate() != "unknown" {
		t.Fatalf("accessors = %s %s %s", GetCurrentName(), GetCurrentVersion(), GetCurrentBuildDate())
	}
}
