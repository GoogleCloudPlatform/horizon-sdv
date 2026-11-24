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

package mtk

import "strings"

// named is one element of a list response that is looked up by name: the name the server reports (empty
// when the server left it out) and the raw identifier to send back.
type named[T any] struct {
	name string
	val  T
}

// pickByName chooses the element that was asked for from a list response. The list endpoints take a
// query, but the server may match it loosely (partial or case-insensitive), so acting on the first
// result could delete or grant the wrong object. An element whose reported name equals want
// (case-insensitively) is chosen. When the server did not report names at all and returned exactly one
// element, that element is trusted, as the query was an exact filter.
func pickByName[T any](items []named[T], want string) (T, bool) {
	for _, it := range items {
		if it.name != "" && strings.EqualFold(it.name, want) {
			return it.val, true
		}
	}
	if len(items) == 1 && items[0].name == "" {
		return items[0].val, true
	}
	var zero T
	return zero, false
}
