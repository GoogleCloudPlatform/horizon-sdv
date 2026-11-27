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
	"context"
	"regexp"
	"strconv"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	smv1 "storage-mirror-module/api/v1alpha1"
	"storage-mirror-module/internal/catalog"
)

var dnsLabelInvalid = regexp.MustCompile(`[^a-z0-9-]+`)

// CRName maps a catalog directory to a DNS-1123 label (Jenkins allowed any directory name).
func CRName(dir string) string {
	n := dnsLabelInvalid.ReplaceAllString(strings.ToLower(dir), "-")
	n = strings.Trim(n, "-")
	if len(n) > 63 {
		n = strings.TrimRight(n[:63], "-")
	}
	if n == "" {
		n = "mirror"
	}
	return n
}

// MirrorFromEntry builds a Retain StorageMirror for an unmanaged catalog entry.
func MirrorFromEntry(dir string, e catalog.Entry, namespace string) *smv1.StorageMirror {
	m := &smv1.StorageMirror{
		ObjectMeta: metav1.ObjectMeta{
			Name:        CRName(dir),
			Namespace:   namespace,
			Labels:      map[string]string{},
			Annotations: map[string]string{AnnotationImportedStatus: e["status"]},
		},
		Spec: smv1.StorageMirrorSpec{Directory: dir, DeletionPolicy: smv1.DeletionPolicyRetain, LFS: smv1.LFSInherit, Schedule: smv1.ScheduleInherit},
	}
	if e["created_by"] != "" {
		m.Annotations[smv1.AnnotationCreatedBy] = e["created_by"]
	}
	if e["type"] == "git" {
		m.Spec.Kind = smv1.KindGit
		m.Spec.Git = &smv1.GitSpec{URL: e["repository_url"], Branches: splitCSV(e["branches"])}
	} else {
		m.Spec.Kind = smv1.KindRepo
		jobs, _ := strconv.Atoi(e["repo_sync_jobs"])
		m.Spec.Repo = &smv1.RepoSpec{ManifestURL: e["manifest_url"], ManifestRef: e["manifest_ref"], ManifestFile: e["manifest_file"], SyncJobs: int32(jobs)}
	}
	m.Labels[smv1.LabelKind] = strings.ToLower(m.Spec.Kind)
	return m
}

// Import creates Retain CRs for catalog entries without one. Empty names imports every entry.
func Import(ctx context.Context, c client.Client, namespace string, snap *catalog.Snapshot, names []string, source string) ([]string, error) {
	var list smv1.StorageMirrorList
	if err := c.List(ctx, &list, client.InNamespace(namespace)); err != nil {
		return nil, err
	}
	managed := map[string]bool{}
	for _, m := range list.Items {
		managed[m.DirectoryOrName()] = true
	}
	want := map[string]bool{}
	for _, n := range names {
		want[n] = true
	}
	var created []string
	for _, dir := range snap.Names() {
		if managed[dir] || (len(want) > 0 && !want[dir]) {
			continue
		}
		e := snap.Entries[dir]
		if e["type"] == "git" && e["repository_url"] == "" {
			continue
		}
		m := MirrorFromEntry(dir, e, namespace)
		m.Annotations["storagemirror.horizon-sdv.io/imported-by"] = source
		if err := c.Create(ctx, m); err != nil {
			if apierrors.IsAlreadyExists(err) {
				continue
			}
			return created, err
		}
		created = append(created, m.Name)
	}
	return created, nil
}
