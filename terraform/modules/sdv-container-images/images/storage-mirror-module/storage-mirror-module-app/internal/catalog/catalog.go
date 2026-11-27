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

// Package catalog reads snapshots of the on-disk metadata.yaml catalog that mirror-agent reports.
// The agent is the only writer of the file; the controller keeps the last snapshot in a ConfigMap.
package catalog

import (
	"context"
	"fmt"
	"sort"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	smv1 "storage-mirror-module/api/v1alpha1"
)

const (
	// ConfigMapName holds the latest catalog snapshot in the controller namespace.
	ConfigMapName = "storage-mirror-catalog"
	KeyMetadata   = "metadata.yaml"
	KeyUsage      = "usage.json"
	KeyReportedAt = "reportedAt"
)

// Entry is one mirror as written by the Jenkins scripts and mirror-agent (all values are strings).
type Entry map[string]string

// Snapshot is a parsed metadata.yaml.
type Snapshot struct {
	RootKey    string
	Entries    map[string]Entry
	ReportedAt time.Time
}

// Parse reads metadata.yaml content with the given root key (the root subdirectory name).
func Parse(content []byte, rootKey string) (*Snapshot, error) {
	var doc map[string]map[string]map[string]interface{}
	if err := yaml.Unmarshal(content, &doc); err != nil {
		return nil, fmt.Errorf("parse metadata.yaml: %w", err)
	}
	s := &Snapshot{RootKey: rootKey, Entries: map[string]Entry{}}
	for name, fields := range doc[rootKey] {
		e := Entry{}
		for k, v := range fields {
			if v == nil {
				e[k] = ""
				continue
			}
			e[k] = fmt.Sprint(v)
		}
		s.Entries[name] = e
	}
	return s, nil
}

// Names returns the sorted entry names.
func (s *Snapshot) Names() []string {
	out := make([]string, 0, len(s.Entries))
	for n := range s.Entries {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// Load reads the snapshot ConfigMap. A missing ConfigMap is an empty snapshot.
func Load(ctx context.Context, c client.Reader, namespace, rootKey string) (*Snapshot, error) {
	var cm corev1.ConfigMap
	if err := c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: ConfigMapName}, &cm); err != nil {
		if apierrors.IsNotFound(err) {
			return &Snapshot{RootKey: rootKey, Entries: map[string]Entry{}}, nil
		}
		return nil, err
	}
	s, err := Parse([]byte(cm.Data[KeyMetadata]), rootKey)
	if err != nil {
		return nil, err
	}
	if t, err := time.Parse(time.RFC3339, cm.Data[KeyReportedAt]); err == nil {
		s.ReportedAt = t
	}
	return s, nil
}

// Store writes a snapshot reported by the agent.
func Store(ctx context.Context, c client.Client, namespace string, metadata []byte, usageJSON []byte, now time.Time) error {
	var cm corev1.ConfigMap
	err := c.Get(ctx, client.ObjectKey{Namespace: namespace, Name: ConfigMapName}, &cm)
	data := map[string]string{
		KeyMetadata:   string(metadata),
		KeyUsage:      string(usageJSON),
		KeyReportedAt: now.UTC().Format(time.RFC3339),
	}
	if apierrors.IsNotFound(err) {
		cm = corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name: ConfigMapName, Namespace: namespace,
				Labels: map[string]string{smv1.LabelModule: smv1.ModuleName},
			},
			Data: data,
		}
		return c.Create(ctx, &cm)
	}
	if err != nil {
		return err
	}
	cm.Data = data
	return c.Update(ctx, &cm)
}
