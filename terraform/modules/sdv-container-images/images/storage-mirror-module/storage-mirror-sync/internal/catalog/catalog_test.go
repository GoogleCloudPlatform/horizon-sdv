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

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// As written by utils.sh (yq) after create_metadata_file_with_root_key and insert_new_mirror_metadata_entry.
const jenkins = `horizon-mirrors:
  my-mirror:
    name: my-mirror
    type: repo
    manifest_url: https://android.googlesource.com/platform/manifest
    manifest_ref: android-16.0.0_r3
    manifest_file: default.xml
    repo_sync_jobs: "4"
    status: ready
    last_successful_sync_time: Wed Nov 25 03:12:44 UTC 2026
    created_by: jane.doe
    custom_key: kept
`

func TestRoundTripKeepsJenkinsFormat(t *testing.T) {
	d, err := Parse([]byte(jenkins), "horizon-mirrors")
	if err != nil {
		t.Fatal(err)
	}
	out, err := d.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != jenkins {
		t.Fatalf("round trip changed the file:\n%s", out)
	}
}

func TestSetKeepsOrderAndQuotesNumbers(t *testing.T) {
	d, _ := Parse([]byte(jenkins), "horizon-mirrors")
	d.Set("my-mirror", "status", "syncing", "repo_sync_jobs", "3", "managed_by", "storage-mirror-module")
	d.Set("kernel", "name", "kernel", "type", "git", "status", "uninitialized", "last_successful_sync_time", "")
	out, _ := d.Bytes()
	s := string(out)
	for _, want := range []string{"    status: syncing\n", `    repo_sync_jobs: "3"` + "\n", "    custom_key: kept\n    managed_by: storage-mirror-module\n", "  kernel:\n    name: kernel\n", `    last_successful_sync_time: ""`} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in\n%s", want, s)
		}
	}
	if strings.Index(s, "status: syncing") > strings.Index(s, "last_successful_sync_time: Wed") {
		t.Error("existing keys must keep their position")
	}
	d.Delete("my-mirror")
	if d.Has("my-mirror") || len(d.Names()) != 1 {
		t.Fatal("delete")
	}
}

func TestEmptyRootFromJenkins(t *testing.T) {
	d, err := Parse([]byte("horizon-mirrors: {}\n"), "horizon-mirrors")
	if err != nil {
		t.Fatal(err)
	}
	if out, _ := d.Bytes(); string(out) != "horizon-mirrors: {}\n" {
		t.Fatalf("empty catalog: %q", out)
	}
	d.Set("a", "name", "a")
	if out, _ := d.Bytes(); string(out) != "horizon-mirrors:\n  a:\n    name: a\n" {
		t.Fatalf("first entry: %q", out)
	}
}

func TestUpdateCreatesAndRewritesAtomically(t *testing.T) {
	root := filepath.Join(t.TempDir(), "horizon-mirrors")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	c := Catalog{Root: root}
	if _, err := c.Update(func(d *Doc) error { d.Set("m", "name", "m", "status", "ready"); return nil }); err != nil {
		t.Fatal(err)
	}
	d, _, err := c.Read()
	if err != nil || d.Get("m", "status") != "ready" {
		t.Fatalf("read back: %v %v", err, d.Entry("m"))
	}
	left, _ := filepath.Glob(filepath.Join(root, ".metadata.yaml.*"))
	if len(left) != 0 {
		t.Fatalf("temp files left: %v", left)
	}
}
