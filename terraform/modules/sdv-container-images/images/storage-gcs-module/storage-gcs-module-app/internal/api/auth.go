// Copyright (c) 2026 Accenture, All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

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
)

type ctxKey int

const callerCtxKey ctxKey = 1
const horizonAPICaller = "horizon-api"

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

// ParseAPITokens loads caller→token map from tokens.json. legacyBearer (API_BEARER_TOKEN)
// is accepted as caller "default" only when JSON is empty. Non-empty invalid JSON is an error
// (fail closed; do not fall back to the legacy token).
func ParseAPITokens(tokensJSON, legacyBearer string) ([]APIToken, error) {
	out := make([]APIToken, 0, 4)
	tokensJSON = strings.TrimSpace(tokensJSON)
	if tokensJSON != "" {
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
	if b := strings.TrimSpace(legacyBearer); b != "" {
		out = append(out, APIToken{Caller: "default", Token: b})
	}
	return out, nil
}

// CallerFromRequest is the authenticated caller name, or empty.
func CallerFromRequest(r *http.Request) string {
	s, _ := r.Context().Value(callerCtxKey).(string)
	return s
}

// withAPIAuth requires Authorization: Bearer matching a configured caller token.
// GET /health stays open for kubelet probes. /ready requires auth (it calls GCS).
// Missing tokens fail closed (503).
func (s *Server) withAPIAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "/health" {
			next.ServeHTTP(w, r)
			return
		}
		if len(s.APITokens) == 0 && s.APIBearerToken == "" {
			writeErr(w, http.StatusServiceUnavailable, "API authentication not configured")
			return
		}
		got := bearerToken(r.Header.Get("Authorization"))
		tok, ok := s.matchAPIToken(got)
		if !ok {
			writeErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		if tok.Caller == horizonAPICaller && !isHorizonAPIListRoute(r) {
			writeErr(w, http.StatusForbidden, "caller is not permitted for this route")
			return
		}
		kind := "current"
		if tok.Previous {
			kind = "previous"
		}
		log.Printf("storage-gcs api %s %s caller=%s token=%s", r.Method, path, tok.Caller, kind)
		r = r.WithContext(context.WithValue(r.Context(), callerCtxKey, tok.Caller))
		next.ServeHTTP(w, r)
	})
}

// isHorizonAPIListRoute permits horizon-api to proxy only its bucket-object listing endpoint.
func isHorizonAPIListRoute(r *http.Request) bool {
	if r.Method != http.MethodGet {
		return false
	}
	const prefix = "/api/v1/buckets/"
	const suffix = "/objects-list"
	if !strings.HasPrefix(r.URL.Path, prefix) || !strings.HasSuffix(r.URL.Path, suffix) {
		return false
	}
	bucket := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, prefix), suffix)
	return bucket != "" && !strings.Contains(bucket, "/")
}

func (s *Server) matchAPIToken(got string) (APIToken, bool) {
	tokens := s.APITokens
	if len(tokens) == 0 && s.APIBearerToken != "" {
		tokens = []APIToken{{Caller: "default", Token: s.APIBearerToken}}
	}
	if got == "" {
		return APIToken{}, false
	}
	gotSum := sha256.Sum256([]byte(got))
	var matched APIToken
	ok := false
	for _, t := range tokens {
		sum := sha256.Sum256([]byte(t.Token))
		if subtle.ConstantTimeCompare(sum[:], gotSum[:]) == 1 {
			matched = t
			ok = true
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
