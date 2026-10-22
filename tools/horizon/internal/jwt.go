// Copyright (c) 2026 Accenture, All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
//
//         http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package horizon

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

// decodeJWTPayload returns the middle segment of a JWT as JSON (display-only, not verified).
func DecodeJWTPayload(token string) (map[string]any, error) {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("not a JWT")
	}
	// JWT segments are base64url and unpadded. RawURLEncoding decodes unpadded
	// input directly; the previous code appended '=' padding and then used
	// RawURLEncoding, which rejects padding and failed with "illegal base64 data
	// at input byte N" whenever the payload length was not a multiple of 4
	// (TAA-1799). Trim any stray padding for robustness, then decode.
	payload := strings.TrimRight(parts[1], "=")
	b, err := base64.RawURLEncoding.DecodeString(payload)
	if err != nil {
		return nil, err
	}
	var o map[string]any
	if err := json.Unmarshal(b, &o); err != nil {
		return nil, err
	}
	return o, nil
}
