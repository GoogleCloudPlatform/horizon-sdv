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

// Package catalog reads and writes the on-disk metadata.yaml in the format of the Jenkins mirror scripts
// (workloads/android/pipelines/environment/mirror/utils/utils.sh): one root key (the basename of the root
// subdirectory), one mapping per mirror directory, every value a string. Unknown keys and key order survive.
package catalog

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"syscall"
	"time"

	"gopkg.in/yaml.v3"
)

// FileName is the catalog file name under the root subdirectory.
const FileName = "metadata.yaml"

// JenkinsTimeLayout is date(1)'s default output, used by utils.sh for last_successful_sync_time.
const JenkinsTimeLayout = "Mon Jan _2 15:04:05 MST 2006"

// Doc is a parsed catalog.
type Doc struct {
	root    *yaml.Node
	rootKey string
}

// Catalog is the metadata.yaml of one root subdirectory.
type Catalog struct {
	// Root is the mirror root subdirectory (for example /mnt/mirror-filestore-pvc/horizon-mirrors).
	Root string
	// LockTimeout bounds the wait for the catalog lock.
	LockTimeout time.Duration
}

// Path is the metadata.yaml path.
func (c Catalog) Path() string { return filepath.Join(c.Root, FileName) }

// RootKey is the catalog root key.
func (c Catalog) RootKey() string { return filepath.Base(c.Root) }

func (c Catalog) lockPath() string { return filepath.Join(c.Root, ".storage-mirror", "catalog.lock") }

// Parse reads catalog bytes; empty content is an empty catalog.
func Parse(content []byte, rootKey string) (*Doc, error) {
	d := &Doc{rootKey: rootKey}
	if len(bytes.TrimSpace(content)) == 0 {
		d.root = &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{{Kind: yaml.MappingNode}}}
	} else {
		var n yaml.Node
		if err := yaml.Unmarshal(content, &n); err != nil {
			return nil, fmt.Errorf("parse %s: %w", FileName, err)
		}
		if n.Kind != yaml.DocumentNode || len(n.Content) == 0 || n.Content[0].Kind != yaml.MappingNode {
			return nil, errors.New("metadata.yaml: top level must be a mapping")
		}
		d.root = &n
	}
	if d.rootMap() == nil {
		top := d.root.Content[0]
		top.Content = append(top.Content, str(rootKey), &yaml.Node{Kind: yaml.MappingNode})
	}
	return d, nil
}

func (d *Doc) rootMap() *yaml.Node {
	top := d.root.Content[0]
	for i := 0; i+1 < len(top.Content); i += 2 {
		if top.Content[i].Value == d.rootKey {
			v := top.Content[i+1]
			if v.Kind != yaml.MappingNode {
				// "horizon-mirrors: {}" or "~" both become an empty block mapping.
				*v = yaml.Node{Kind: yaml.MappingNode}
			}
			return v
		}
	}
	return nil
}

func (d *Doc) entry(name string, create bool) *yaml.Node {
	m := d.rootMap()
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == name {
			return m.Content[i+1]
		}
	}
	if !create {
		return nil
	}
	m.Style = 0
	e := &yaml.Node{Kind: yaml.MappingNode}
	m.Content = append(m.Content, str(name), e)
	return e
}

// Names lists the mirror directories, sorted.
func (d *Doc) Names() []string {
	m := d.rootMap()
	var out []string
	for i := 0; i+1 < len(m.Content); i += 2 {
		out = append(out, m.Content[i].Value)
	}
	sort.Strings(out)
	return out
}

// Has reports whether the catalog has an entry for the directory.
func (d *Doc) Has(name string) bool { return d.entry(name, false) != nil }

// Get returns a field of an entry.
func (d *Doc) Get(name, key string) string {
	e := d.entry(name, false)
	if e == nil {
		return ""
	}
	for i := 0; i+1 < len(e.Content); i += 2 {
		if e.Content[i].Value == key {
			return e.Content[i+1].Value
		}
	}
	return ""
}

// Entry returns all fields of an entry.
func (d *Doc) Entry(name string) map[string]string {
	e := d.entry(name, false)
	if e == nil {
		return nil
	}
	out := map[string]string{}
	for i := 0; i+1 < len(e.Content); i += 2 {
		out[e.Content[i].Value] = e.Content[i+1].Value
	}
	return out
}

// Set writes fields in the given order, creating the entry when missing. Existing keys keep their position.
func (d *Doc) Set(name string, kv ...string) {
	e := d.entry(name, true)
	e.Style = 0
	for j := 0; j+1 < len(kv); j += 2 {
		key, val := kv[j], kv[j+1]
		found := false
		for i := 0; i+1 < len(e.Content); i += 2 {
			if e.Content[i].Value == key {
				e.Content[i+1] = str(val)
				found = true
				break
			}
		}
		if !found {
			e.Content = append(e.Content, str(key), str(val))
		}
	}
}

// Delete removes an entry.
func (d *Doc) Delete(name string) {
	m := d.rootMap()
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == name {
			m.Content = append(m.Content[:i], m.Content[i+2:]...)
			return
		}
	}
}

// Bytes encodes the catalog with two-space indentation (yq's default).
func (d *Doc) Bytes() ([]byte, error) {
	m := d.rootMap()
	if len(m.Content) == 0 {
		m.Style = yaml.FlowStyle
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(d.root); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func str(v string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: v}
}

// Read loads the catalog without locking. A missing file is an empty catalog.
func (c Catalog) Read() (*Doc, []byte, error) {
	b, err := os.ReadFile(c.Path())
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, nil, err
	}
	d, err := Parse(b, c.RootKey())
	return d, b, err
}

// Update runs fn under an exclusive flock and writes the result atomically (temp file and rename).
func (c Catalog) Update(fn func(*Doc) error) ([]byte, error) {
	if err := os.MkdirAll(filepath.Dir(c.lockPath()), 0o2775); err != nil {
		return nil, err
	}
	lf, err := os.OpenFile(c.lockPath(), os.O_CREATE|os.O_RDWR, 0o664)
	if err != nil {
		return nil, err
	}
	defer lf.Close()
	timeout := c.LockTimeout
	if timeout == 0 {
		timeout = 5 * time.Minute
	}
	deadline := time.Now().Add(timeout)
	for {
		err := syscall.Flock(int(lf.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			break
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("catalog lock %s: %w", c.lockPath(), err)
		}
		time.Sleep(500 * time.Millisecond)
	}
	defer func() { _ = syscall.Flock(int(lf.Fd()), syscall.LOCK_UN) }()

	d, _, err := c.Read()
	if err != nil {
		return nil, err
	}
	if err := fn(d); err != nil {
		return nil, err
	}
	out, err := d.Bytes()
	if err != nil {
		return nil, err
	}
	tmp, err := os.CreateTemp(c.Root, ".metadata.yaml.*")
	if err != nil {
		return nil, err
	}
	if _, err := tmp.Write(out); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return nil, err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return nil, err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return nil, err
	}
	_ = os.Chmod(tmp.Name(), 0o664)
	if err := os.Rename(tmp.Name(), c.Path()); err != nil {
		os.Remove(tmp.Name())
		return nil, err
	}
	return out, nil
}
