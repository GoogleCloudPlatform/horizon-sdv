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

package api

import "testing"

func TestInvalidObjectPath(t *testing.T) {
	allow := []string{
		"backup..2026.tar",
		"v1..2/notes.txt",
		"dir/..hidden",
		"path/to/object",
		"..hidden",
		"foo.",
	}
	for _, p := range allow {
		if invalidObjectPath(p) {
			t.Errorf("allowed path rejected: %q", p)
		}
	}
	deny := []string{
		"/absolute",
		"..",
		"foo/../bar",
		"../secret",
		"a/../b/c",
	}
	for _, p := range deny {
		if !invalidObjectPath(p) {
			t.Errorf("invalid path allowed: %q", p)
		}
	}
}
