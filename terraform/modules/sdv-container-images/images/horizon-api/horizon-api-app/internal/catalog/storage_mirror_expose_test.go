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
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
)

// TestStorageMirrorTemplatesNotInCatalog renders the storage-mirror WorkflowTemplates and runs them through the
// catalog ingestion: none may be listed (mirror changes go through /v1/storage-mirror/* and its admin role).
func TestStorageMirrorTemplatesNotInCatalog(t *testing.T) {
	chart := findUp(t, filepath.Join("gitops", "modules", "storage-mirror-module", "argo-workflows"))
	helm := os.Getenv("HELM")
	if helm == "" {
		helm, _ = exec.LookPath("helm")
	}
	if helm == "" {
		t.Skip("helm not found (set HELM)")
	}
	out, err := exec.Command(helm, "template", "smw", chart, "--set", "syncImage=img").Output()
	if err != nil {
		t.Fatalf("helm template: %v", err)
	}
	dec := utilyaml.NewYAMLOrJSONDecoder(bytes.NewReader(out), 4096)
	var templates []*unstructured.Unstructured
	for {
		u := &unstructured.Unstructured{}
		if err := dec.Decode(&u.Object); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			t.Fatal(err)
		}
		if u.GetKind() == "WorkflowTemplate" {
			templates = append(templates, u)
		}
	}
	if len(templates) != 4 {
		t.Fatalf("rendered %d WorkflowTemplates, want 4", len(templates))
	}
	c := New("module-manager", "workflows", "state", "catalog")
	enabled := map[string]bool{"storage-mirror": true}
	next := map[string]Entry{}
	for _, wt := range templates {
		c.ingestExposedTemplate(next, enabled, wt, wt.GetNamespace())
	}
	if len(next) != 0 {
		t.Fatalf("storage-mirror templates listed by the Horizon API catalog: %v", next)
	}
	// The same template with the expose label set would be listed, so the check above is meaningful.
	wt := templates[0].DeepCopy()
	labels := wt.GetLabels()
	labels[labelExpose] = labelExposeVal
	wt.SetLabels(labels)
	c.ingestExposedTemplate(next, enabled, wt, wt.GetNamespace())
	if len(next) != 1 {
		t.Fatal("control: an exposed template was not listed")
	}
}

func findUp(t *testing.T, rel string) string {
	t.Helper()
	dir, _ := os.Getwd()
	for i := 0; i < 12 && dir != "/"; i++ {
		if st, err := os.Stat(filepath.Join(dir, rel)); err == nil && st.IsDir() {
			return filepath.Join(dir, rel)
		}
		dir = filepath.Dir(dir)
	}
	t.Skip(rel + " not found (image build context)")
	return ""
}
