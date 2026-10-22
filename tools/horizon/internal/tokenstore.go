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
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// TokenCache is persisted after interactive login (0600).
type TokenCache struct {
	AccessToken    string `json:"access_token"`
	RefreshToken   string `json:"refresh_token,omitempty"`
	ExpiresAt      int64  `json:"expires_at_unix,omitempty"`
	ExpiresAtHuman string `json:"expires_at,omitempty"` // RFC3339 UTC; set on save from ExpiresAt
	KeycloakBase   string `json:"keycloak_base,omitempty"`
	Realm          string `json:"realm,omitempty"`
	ClientID       string `json:"client_id,omitempty"`
}

func TokenCachePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, ".config", "horizon")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return filepath.Join(dir, "token.json"), nil
}

func LoadTokenCache() (*TokenCache, error) {
	p, err := TokenCachePath()
	if err != nil {
		return nil, err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var t TokenCache
	if err := json.Unmarshal(b, &t); err != nil {
		return nil, fmt.Errorf("token cache: %w", err)
	}
	return &t, nil
}

func SaveTokenCache(t *TokenCache) error {
	if t != nil {
		if t.ExpiresAt > 0 {
			t.ExpiresAtHuman = time.Unix(t.ExpiresAt, 0).UTC().Format(time.RFC3339)
		} else {
			t.ExpiresAtHuman = ""
		}
	}
	p, err := TokenCachePath()
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(p, b, 0o600); err != nil {
		return err
	}
	return nil
}

func ClearTokenCache() error {
	p, err := TokenCachePath()
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (t *TokenCache) IsExpired(skew time.Duration) bool {
	if t == nil || t.AccessToken == "" {
		return true
	}
	if t.ExpiresAt <= 0 {
		return false
	}
	return time.Now().Unix()+int64(skew.Seconds()) >= t.ExpiresAt
}
