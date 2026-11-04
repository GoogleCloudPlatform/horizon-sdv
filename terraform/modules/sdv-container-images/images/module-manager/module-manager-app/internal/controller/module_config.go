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
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// NamespacePrefixFromModuleConfig reads namespacePrefix from MODULE_CONFIG-style YAML/JSON.
// If moduleConfig is empty, falls back to the MODULE_CONFIG environment variable (same source
// as the Module Manager Deployment) so controller callers without inline config still resolve prefix.
func NamespacePrefixFromModuleConfig(moduleConfig string) string {
	moduleConfig = strings.TrimSpace(moduleConfig)
	if moduleConfig == "" {
		moduleConfig = strings.TrimSpace(os.Getenv("MODULE_CONFIG"))
	}
	if moduleConfig == "" {
		return ""
	}
	var m map[string]interface{}
	switch {
	case yaml.Unmarshal([]byte(moduleConfig), &m) == nil:
	case json.Unmarshal([]byte(moduleConfig), &m) == nil:
	default:
		return ""
	}
	if m == nil {
		return ""
	}
	v, ok := m["namespacePrefix"]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	default:
		return strings.TrimSpace(fmt.Sprint(t))
	}
}
