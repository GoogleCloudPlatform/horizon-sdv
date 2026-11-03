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

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// metadataDeletionTimestampRFC3339 returns metadata.deletionTimestamp as a non-empty string.
// Unstructured objects sometimes store the field as a string; other decodings use map[string]interface{}
// values that NestedString skips — both must be handled so uninstall detection works reliably.
func metadataDeletionTimestampRFC3339(obj *unstructured.Unstructured) string {
	if obj == nil {
		return ""
	}
	if ts, ok, _ := unstructured.NestedString(obj.Object, "metadata", "deletionTimestamp"); ok && ts != "" {
		return ts
	}
	raw, found, err := unstructured.NestedFieldNoCopy(obj.Object, "metadata", "deletionTimestamp")
	if !found || err != nil || raw == nil {
		return ""
	}
	switch v := raw.(type) {
	case string:
		return v
	default:
		s := fmt.Sprint(v)
		if s == "" || s == "<nil>" {
			return ""
		}
		return s
	}
}
