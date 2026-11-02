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

package horizon

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

func testJWT(t *testing.T, payload any) string {
	t.Helper()
	b, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	enc := base64.RawURLEncoding.EncodeToString
	return enc([]byte(`{"alg":"none"}`)) + "." + enc(b) + ".sig"
}

func TestDecodeJWTPayload(t *testing.T) {
	claims, err := DecodeJWTPayload("  " + testJWT(t, map[string]any{"sub": "user", "exp": 10}) + "\n")
	if err != nil {
		t.Fatal(err)
	}
	if claims["sub"] != "user" {
		t.Fatalf("sub = %#v", claims["sub"])
	}
	if claims["exp"] != float64(10) {
		t.Fatalf("exp = %#v", claims["exp"])
	}
}

func TestDecodeJWTPayloadUnpaddedSegment(t *testing.T) {
	// Payload length is not a multiple of 4. Decoding must accept the raw
	// segment and also a copy that still carries '=' padding.
	token := testJWT(t, map[string]any{"a": 1})
	parts := strings.Split(token, ".")
	if len(parts[1])%4 == 0 {
		t.Fatal("fixture payload segment is padded; choose a payload whose length is not a multiple of 4")
	}
	if _, err := DecodeJWTPayload(token); err != nil {
		t.Fatal(err)
	}
	padded := parts[0] + "." + parts[1] + "==." + parts[2]
	claims, err := DecodeJWTPayload(padded)
	if err != nil {
		t.Fatal(err)
	}
	if claims["a"] != float64(1) {
		t.Fatalf("a = %#v", claims["a"])
	}
}

func TestDecodeJWTPayloadErrors(t *testing.T) {
	cases := []string{
		"",
		"abc",
		"a.b",
		"a.b.c.d",
		"a.***.c",
		testJWT(t, []any{1, 2}),
	}
	for _, token := range cases {
		if _, err := DecodeJWTPayload(token); err == nil {
			t.Fatalf("expected error for %q", token)
		}
	}
}
