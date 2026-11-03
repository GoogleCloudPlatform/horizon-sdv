// Copyright (c) 2024-2026 Accenture, All Rights Reserved.
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
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.

package controller

import "strings"

// ArgoCDConfig holds ArgoCD connection and naming config used by the REST handler and by
// module enable/disable paths to build Applications deterministically from catalog module
// names.
type ArgoCDConfig struct {
	Namespace string
	Project   string
	RepoURL   string
	Revision  string
}

// ApplicationName returns the ArgoCD Application name for a module (e.g. mod-sample-module).
// The mapping is deterministic: module names with underscores are normalised to hyphens to
// match Kubernetes object naming rules.
func ApplicationName(moduleName string) string {
	return "mod-" + strings.ReplaceAll(moduleName, "_", "-")
}
