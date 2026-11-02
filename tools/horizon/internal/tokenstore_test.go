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
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

func TestTokenCacheIsExpired(t *testing.T) {
	if !(*TokenCache)(nil).IsExpired(0) {
		t.Fatal("nil cache is expired")
	}
	if !(&TokenCache{}).IsExpired(0) {
		t.Fatal("empty token is expired")
	}
	if (&TokenCache{AccessToken: "t"}).IsExpired(time.Minute) {
		t.Fatal("missing expiry should be treated as usable")
	}
	fresh := &TokenCache{AccessToken: "t", ExpiresAt: time.Now().Add(2 * time.Minute).Unix()}
	if fresh.IsExpired(30 * time.Second) {
		t.Fatal("fresh token")
	}
	if !fresh.IsExpired(3 * time.Minute) {
		t.Fatal("skew should treat the token as expired")
	}
	past := &TokenCache{AccessToken: "t", ExpiresAt: time.Now().Add(-time.Second).Unix()}
	if !past.IsExpired(0) {
		t.Fatal("past token")
	}
}

func TestTokenCacheRoundTrip(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	loaded, err := LoadTokenCache()
	if err != nil || loaded != nil {
		t.Fatalf("missing cache: %+v %v", loaded, err)
	}

	exp := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC).Unix()
	in := &TokenCache{
		AccessToken:  "access",
		RefreshToken: "refresh",
		ExpiresAt:    exp,
		KeycloakBase: "https://auth.example",
		Realm:        "horizon",
		ClientID:     "cli",
	}
	if err := SaveTokenCache(in); err != nil {
		t.Fatal(err)
	}
	if in.ExpiresAtHuman != "2026-10-09T12:00:00Z" {
		t.Fatalf("human expiry = %q", in.ExpiresAtHuman)
	}
	path, err := TokenCachePath()
	if err != nil {
		t.Fatal(err)
	}
	if path != filepath.Join(home, ".config", "horizon", "token.json") {
		t.Fatalf("path = %q", path)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %o", info.Mode().Perm())
	}

	loaded, err = LoadTokenCache()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.AccessToken != "access" || loaded.RefreshToken != "refresh" || loaded.Realm != "horizon" || loaded.ExpiresAt != exp {
		t.Fatalf("loaded = %+v", loaded)
	}

	if err := ClearTokenCache(); err != nil {
		t.Fatal(err)
	}
	if err := ClearTokenCache(); err != nil {
		t.Fatal(err)
	}
	loaded, err = LoadTokenCache()
	if err != nil || loaded != nil {
		t.Fatalf("after clear: %+v %v", loaded, err)
	}
}

func TestLoadTokenCacheInvalidJSON(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	path, err := TokenCachePath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadTokenCache(); err == nil {
		t.Fatal("expected JSON error")
	}
}

func TestSaveTokenCacheClearsHumanExpiry(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	tok := &TokenCache{AccessToken: "access", ExpiresAtHuman: "stale"}
	if err := SaveTokenCache(tok); err != nil {
		t.Fatal(err)
	}
	if tok.ExpiresAtHuman != "" {
		t.Fatalf("human expiry = %q", tok.ExpiresAtHuman)
	}
	if err := SaveTokenCache(nil); err != nil {
		t.Fatal(err)
	}
}
