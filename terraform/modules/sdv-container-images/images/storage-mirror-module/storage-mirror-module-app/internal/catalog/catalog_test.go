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

package catalog

import "testing"

// jenkinsMetadata is what utils.sh insert_new_mirror_metadata_entry and update_mirror_metadata_value write.
const jenkinsMetadata = `horizon-mirrors:
  my-mirror:
    name: my-mirror
    type: repo
    manifest_url: https://android.googlesource.com/platform/manifest
    manifest_ref: android-16.0.0_r3
    manifest_file: default.xml
    repo_sync_jobs: "4"
    status: ready
    last_successful_sync_time: "Wed Nov 25 03:12:44 UTC 2026"
    created_by: jane.doe
  kernel:
    name: kernel
    type: git
    repository_url: https://example.com/kernel.git
    repo_sync_jobs: 3
    status: ""
`

func TestParseJenkinsCatalog(t *testing.T) {
	s, err := Parse([]byte(jenkinsMetadata), "horizon-mirrors")
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Names(); len(got) != 2 || got[0] != "kernel" || got[1] != "my-mirror" {
		t.Fatalf("names %v", got)
	}
	m := s.Entries["my-mirror"]
	if m["repo_sync_jobs"] != "4" || m["status"] != "ready" || m["created_by"] != "jane.doe" {
		t.Fatalf("entry %v", m)
	}
	if s.Entries["kernel"]["repo_sync_jobs"] != "3" || s.Entries["kernel"]["status"] != "" {
		t.Fatalf("numbers and empty strings must read as strings: %v", s.Entries["kernel"])
	}
}

func TestParseEmptyAndWrongRoot(t *testing.T) {
	s, err := Parse([]byte("horizon-mirrors: {}\n"), "horizon-mirrors")
	if err != nil || len(s.Entries) != 0 {
		t.Fatalf("empty catalog: %v %v", err, s)
	}
	s, err = Parse([]byte(jenkinsMetadata), "other-root")
	if err != nil || len(s.Entries) != 0 {
		t.Fatal("other root key must be empty")
	}
	if _, err := Parse([]byte("a: [unclosed"), "x"); err == nil {
		t.Fatal("invalid yaml must fail")
	}
}
