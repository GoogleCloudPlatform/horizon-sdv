// Copyright (c) 2026 Accenture, All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package horizonauth

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
)

// BearerToken returns the token from an "Authorization: Bearer <token>" header.
func BearerToken(r *http.Request) (string, error) {
	scheme, raw, ok := strings.Cut(strings.TrimSpace(r.Header.Get("Authorization")), " ")
	if !ok || !strings.EqualFold(scheme, "bearer") {
		return "", errors.New("missing bearer token")
	}
	return strings.TrimSpace(raw), nil
}

// Forbidden is the JSON body of a 403 response.
type Forbidden struct {
	Code         string `json:"error"`
	RequiredRole string `json:"requiredRole"`
	CurrentRole  string `json:"currentRole"`
	Reason       string `json:"reason"`
}

// WriteForbidden writes f as a 403 response.
func WriteForbidden(w http.ResponseWriter, f *Forbidden) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	_ = json.NewEncoder(w).Encode(f)
}
