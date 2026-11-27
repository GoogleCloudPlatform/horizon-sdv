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
	"strings"
	"time"

	"mtk-connect-module/internal/mtk"
)

// keyPrefixLen is how many leading characters identify a key in the MTK Connect key list,
// which does not return the full key value.
const keyPrefixLen = 8

// findCurrentKey returns the entry of keys that matches the current key value.
func findCurrentKey(keys []mtk.Key, current string) (mtk.Key, bool) {
	if len(current) < keyPrefixLen {
		return mtk.Key{}, false
	}
	prefix := current[:keyPrefixLen]
	var match mtk.Key
	n := 0
	for _, k := range keys {
		if len(k.Value) >= keyPrefixLen && strings.HasPrefix(k.Value, prefix) {
			match = k
			n++
		}
	}
	// An ambiguous prefix must not be used to decide what to delete.
	return match, n == 1
}

// keysToDelete returns the ids of keys that were superseded more than overlap ago. A key is superseded
// when the next newer key was created, so with a rotation interval shorter than the overlap the older
// keys still go once they have been replaced for long enough, while the previous key stays valid for
// the whole overlap. It returns nothing when the current key cannot be identified or has no creation
// time, so a failed lookup can never delete the key in use.
func keysToDelete(keys []mtk.Key, current mtk.Key, found bool, overlap time.Duration, now time.Time) []string {
	if !found || current.ID == "" || current.Created.IsZero() {
		return nil
	}
	var ids []string
	for _, k := range keys {
		if k.ID == "" || k.ID == current.ID {
			continue
		}
		if k.Created.IsZero() || !k.Created.Before(current.Created) {
			continue // newer than or unknown relative to the current key: keep
		}
		// The key was superseded when the next newer key (at the latest the current key) appeared.
		supersededAt := current.Created
		for _, n := range keys {
			if n.Created.IsZero() || !n.Created.After(k.Created) {
				continue
			}
			if n.Created.Before(supersededAt) {
				supersededAt = n.Created
			}
		}
		if now.Before(supersededAt.Add(overlap)) {
			continue
		}
		ids = append(ids, k.ID)
	}
	return ids
}

// nextRotation returns when the next rotation is due given the last one.
func nextRotation(last time.Time, interval time.Duration) time.Time {
	return last.Add(interval)
}

// rotationDue reports whether a rotation must run now. force is true when the rotate-now annotation
// carries a token that has not been handled yet.
func rotationDue(last, now time.Time, interval time.Duration, force bool) bool {
	if force {
		return true
	}
	if last.IsZero() {
		return false
	}
	return !now.Before(nextRotation(last, interval))
}
