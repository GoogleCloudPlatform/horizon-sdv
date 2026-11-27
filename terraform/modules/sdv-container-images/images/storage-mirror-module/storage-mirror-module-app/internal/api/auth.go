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
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"storage-mirror-module/internal/secrets"
)

type ctxKey int

const callerCtxKey ctxKey = 1

const (
	// CallerHorizonAPI is the Horizon API proxy (public routes only).
	CallerHorizonAPI = secrets.CallerHorizonAPI
	// CallerAgent is mirror-agent in workflow pods (internal routes only).
	CallerAgent = secrets.CallerAgent
	// CallerDefault is the operator token kept in the controller namespace (public routes).
	CallerDefault = secrets.CallerDefault
)

// APIToken is one accepted Bearer secret bound to a caller name.
type APIToken struct {
	Caller   string
	Token    string
	Previous bool
}

type callerTokenFile struct {
	Current  string `json:"current"`
	Previous string `json:"previous,omitempty"`
}

// ParseAPITokens loads the caller→token map (tokens.json of the token Secret). Invalid JSON is an error (fail closed).
func ParseAPITokens(tokensJSON string) ([]APIToken, error) {
	var out []APIToken
	tokensJSON = strings.TrimSpace(tokensJSON)
	if tokensJSON == "" {
		return out, nil
	}
	var m map[string]callerTokenFile
	if err := json.Unmarshal([]byte(tokensJSON), &m); err != nil {
		return nil, fmt.Errorf("API_TOKENS JSON: %w", err)
	}
	for caller, t := range m {
		if c := strings.TrimSpace(t.Current); c != "" {
			out = append(out, APIToken{Caller: caller, Token: c})
		}
		if p := strings.TrimSpace(t.Previous); p != "" {
			out = append(out, APIToken{Caller: caller, Token: p, Previous: true})
		}
	}
	return out, nil
}

// CallerFromRequest is the authenticated caller name, or empty.
func CallerFromRequest(r *http.Request) string {
	s, _ := r.Context().Value(callerCtxKey).(string)
	return s
}

// callerAllowed keeps each caller on its own route family.
func callerAllowed(caller, path string) bool {
	internal := strings.HasPrefix(path, "/internal/")
	switch caller {
	case CallerAgent:
		return internal
	case CallerHorizonAPI, CallerDefault:
		return strings.HasPrefix(path, "/api/v1/")
	}
	return false
}

func (s *Server) withAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(s.Tokens) == 0 {
			writeErr(w, http.StatusServiceUnavailable, "API authentication not configured")
			return
		}
		tok, ok := matchToken(s.Tokens, bearerToken(r.Header.Get("Authorization")))
		if !ok {
			writeErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		if !callerAllowed(tok.Caller, r.URL.Path) {
			writeErr(w, http.StatusForbidden, "caller is not permitted for this route")
			return
		}
		kind := "current"
		if tok.Previous {
			kind = "previous"
		}
		log.Printf("storage-mirror api %s %s caller=%s token=%s user=%q", r.Method, r.URL.Path, tok.Caller, kind, r.Header.Get(HeaderUser))
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), callerCtxKey, tok.Caller)))
	})
}

func matchToken(tokens []APIToken, got string) (APIToken, bool) {
	if got == "" {
		return APIToken{}, false
	}
	gotSum := sha256.Sum256([]byte(got))
	var matched APIToken
	ok := false
	for _, t := range tokens {
		sum := sha256.Sum256([]byte(t.Token))
		if subtle.ConstantTimeCompare(sum[:], gotSum[:]) == 1 {
			matched, ok = t, true
		}
	}
	return matched, ok
}

func bearerToken(h string) string {
	const prefix = "Bearer "
	if len(h) < len(prefix) || !strings.EqualFold(h[:len(prefix)], prefix) {
		return ""
	}
	return strings.TrimSpace(h[len(prefix):])
}
