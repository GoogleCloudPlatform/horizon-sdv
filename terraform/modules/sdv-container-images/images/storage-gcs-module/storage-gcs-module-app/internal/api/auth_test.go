// Copyright (c) 2026 Accenture, All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.

package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWithAPIAuth(t *testing.T) {
	okHandler := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	})

	t.Run("health open", func(t *testing.T) {
		s := &Server{APIBearerToken: "secret"}
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		rr := httptest.NewRecorder()
		s.withAPIAuth(okHandler).ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status=%d", rr.Code)
		}
	})

	t.Run("ready requires token", func(t *testing.T) {
		s := &Server{APIBearerToken: "secret"}
		req := httptest.NewRequest(http.MethodGet, "/ready", nil)
		rr := httptest.NewRecorder()
		s.withAPIAuth(okHandler).ServeHTTP(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("status=%d", rr.Code)
		}
	})

	t.Run("api requires token", func(t *testing.T) {
		s := &Server{APIBearerToken: "secret"}
		req := httptest.NewRequest(http.MethodGet, "/api/v1/buckets/x/objects-meta/y", nil)
		rr := httptest.NewRecorder()
		s.withAPIAuth(okHandler).ServeHTTP(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("status=%d", rr.Code)
		}
	})

	t.Run("api accepts bearer", func(t *testing.T) {
		s := &Server{APIBearerToken: "secret"}
		req := httptest.NewRequest(http.MethodGet, "/api/v1/buckets/x/objects-meta/y", nil)
		req.Header.Set("Authorization", "Bearer secret")
		rr := httptest.NewRecorder()
		s.withAPIAuth(okHandler).ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status=%d", rr.Code)
		}
	})

	t.Run("per-caller token", func(t *testing.T) {
		s := &Server{APITokens: []APIToken{
			{Caller: "storage-gcs-internal", Token: "internal-secret"},
			{Caller: "other-module", Token: "other-secret"},
		}}
		req := httptest.NewRequest(http.MethodGet, "/api/v1/buckets/x/objects-meta/y", nil)
		req.Header.Set("Authorization", "Bearer other-secret")
		rr := httptest.NewRecorder()
		var gotCaller string
		inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotCaller = CallerFromRequest(r)
			w.WriteHeader(http.StatusOK)
		})
		s.withAPIAuth(inner).ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status=%d", rr.Code)
		}
		if gotCaller != "other-module" {
			t.Fatalf("caller=%q", gotCaller)
		}
	})

	t.Run("horizon-api can list objects only", func(t *testing.T) {
		s := &Server{APITokens: []APIToken{{Caller: horizonAPICaller, Token: "horizon-api-secret"}}}
		h := s.withAPIAuth(okHandler)

		listReq := httptest.NewRequest(http.MethodGet, "/api/v1/buckets/example/objects-list", nil)
		listReq.Header.Set("Authorization", "Bearer horizon-api-secret")
		listRR := httptest.NewRecorder()
		h.ServeHTTP(listRR, listReq)
		if listRR.Code != http.StatusOK {
			t.Fatalf("list status=%d", listRR.Code)
		}

		otherReq := httptest.NewRequest(http.MethodGet, "/api/v1/buckets/example/objects-meta/object", nil)
		otherReq.Header.Set("Authorization", "Bearer horizon-api-secret")
		otherRR := httptest.NewRecorder()
		h.ServeHTTP(otherRR, otherReq)
		if otherRR.Code != http.StatusForbidden {
			t.Fatalf("other status=%d", otherRR.Code)
		}
	})

	t.Run("previous token still accepted", func(t *testing.T) {
		s := &Server{APITokens: []APIToken{
			{Caller: "storage-gcs-internal", Token: "new-secret"},
			{Caller: "storage-gcs-internal", Token: "old-secret", Previous: true},
		}}
		req := httptest.NewRequest(http.MethodGet, "/api/v1/buckets/x/objects-meta/y", nil)
		req.Header.Set("Authorization", "Bearer old-secret")
		rr := httptest.NewRecorder()
		s.withAPIAuth(okHandler).ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("status=%d", rr.Code)
		}
	})

	t.Run("unknown token rejected", func(t *testing.T) {
		s := &Server{APITokens: []APIToken{
			{Caller: "storage-gcs-internal", Token: "internal-secret"},
		}}
		req := httptest.NewRequest(http.MethodGet, "/api/v1/buckets/x/objects-meta/y", nil)
		req.Header.Set("Authorization", "Bearer wrong")
		rr := httptest.NewRecorder()
		s.withAPIAuth(okHandler).ServeHTTP(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("status=%d", rr.Code)
		}
	})

	t.Run("missing token fails closed", func(t *testing.T) {
		s := &Server{}
		req := httptest.NewRequest(http.MethodGet, "/api/v1/buckets/x/objects-meta/y", nil)
		rr := httptest.NewRecorder()
		s.withAPIAuth(okHandler).ServeHTTP(rr, req)
		if rr.Code != http.StatusServiceUnavailable {
			t.Fatalf("status=%d", rr.Code)
		}
	})
}

func TestRequireBucketStaticAllowlist(t *testing.T) {
	s := &Server{
		StaticAllowedBuckets: map[string]struct{}{
			"proj-aaos": {},
		},
	}
	rr := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if !s.requireBucket(rr, req, "proj-aaos") {
		t.Fatal("expected allowed")
	}
	rr2 := httptest.NewRecorder()
	if s.requireBucket(rr2, req, "other-bucket") {
		t.Fatal("expected denied")
	}
	if rr2.Code != http.StatusForbidden {
		t.Fatalf("status=%d", rr2.Code)
	}
	rr3 := httptest.NewRecorder()
	if s.requireBucket(rr3, req, "ab") {
		t.Fatal("expected invalid")
	}
	if rr3.Code != http.StatusBadRequest {
		t.Fatalf("status=%d", rr3.Code)
	}

	s2 := &Server{
		ProjectID: "proj",
		StaticAllowedBuckets: map[string]struct{}{
			"proj-aaos":    {},
			"other-bucket": {},
		},
	}
	rr4 := httptest.NewRecorder()
	if !s2.requireBucket(rr4, req, "proj-aaos") {
		t.Fatal("expected prefixed static bucket allowed")
	}
	rr5 := httptest.NewRecorder()
	if s2.requireBucket(rr5, req, "other-bucket") {
		t.Fatal("expected unprefixed bucket denied")
	}
	if rr5.Code != http.StatusForbidden {
		t.Fatalf("status=%d", rr5.Code)
	}
}

func TestParseAPITokens(t *testing.T) {
	got, err := ParseAPITokens(`{"storage-gcs-internal":{"current":"cur","previous":"prev"}}`, "legacy")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("len=%d", len(got))
	}
	byTok := map[string]APIToken{}
	for _, tkn := range got {
		byTok[tkn.Token] = tkn
	}
	if byTok["cur"].Caller != "storage-gcs-internal" || byTok["cur"].Previous {
		t.Fatalf("current: %+v", byTok["cur"])
	}
	if byTok["prev"].Caller != "storage-gcs-internal" || !byTok["prev"].Previous {
		t.Fatalf("previous: %+v", byTok["prev"])
	}
	legacy, err := ParseAPITokens("", "only")
	if err != nil {
		t.Fatal(err)
	}
	if len(legacy) != 1 || legacy[0].Caller != "default" || legacy[0].Token != "only" {
		t.Fatalf("legacy: %+v", legacy)
	}
	if _, err := ParseAPITokens("{not-json", "fallback"); err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	emptyJSON, err := ParseAPITokens("{}", "legacy")
	if err != nil {
		t.Fatal(err)
	}
	if len(emptyJSON) != 0 {
		t.Fatalf("non-empty JSON must not fall back to legacy: %+v", emptyJSON)
	}
}

func TestNewUploadID(t *testing.T) {
	a, err := newUploadID()
	if err != nil {
		t.Fatal(err)
	}
	b, err := newUploadID()
	if err != nil {
		t.Fatal(err)
	}
	if len(a) != 32 {
		t.Fatalf("len=%d want 32 hex chars", len(a))
	}
	if a == b {
		t.Fatal("expected unique ids")
	}
}
