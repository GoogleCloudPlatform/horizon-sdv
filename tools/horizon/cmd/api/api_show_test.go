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
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	horizon "github.com/horizon-sdv/horizon/internal"
)

func isolateAPIShowEnv(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("APPDATA", dir)
	t.Setenv("HOME", dir)
	t.Setenv("USERPROFILE", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	for _, k := range []string{
		"HORIZON_API_BASE_URL", "HORIZON_DOMAIN", "KEYCLOAK_BASE",
		"KEYCLOAK_CLIENT_SECRET", "KEYCLOAK_CLIENT_ID", "HORIZON_ACCESS_TOKEN",
	} {
		t.Setenv(k, "")
	}
	oldAPI, oldDomain := horizon.FlagAPI, horizon.FlagDomain
	t.Cleanup(func() {
		horizon.FlagAPI, horizon.FlagDomain = oldAPI, oldDomain
	})
	horizon.FlagAPI, horizon.FlagDomain = "", ""
}

func captureStdout(t *testing.T, fn func() error) (string, error) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = old }()

	var buf bytes.Buffer
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(&buf, r)
		close(done)
	}()

	runErr := fn()
	_ = w.Close()
	<-done
	_ = r.Close()
	return buf.String(), runErr
}

func TestApiShowWorksOfflineAndUnauthenticated(t *testing.T) {
	isolateAPIShowEnv(t)

	cmd := NewApiCmd()
	cmd.SetArgs([]string{"show", "listCatalog"})
	out, err := captureStdout(t, cmd.Execute)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "{api}/v1/catalog") {
		t.Fatalf("output = %s", out)
	}

	t.Setenv("KEYCLOAK_CLIENT_SECRET", "secret")
	t.Setenv("HORIZON_API_BASE_URL", "https://env.example/horizon-api/")
	cmd = NewApiCmd()
	cmd.SetArgs([]string{"show", "listCatalog"})
	out, err = captureStdout(t, cmd.Execute)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "https://env.example/horizon-api/v1/catalog") {
		t.Fatalf("output = %s", out)
	}
	if strings.Contains(out, "{api}/v1/catalog") {
		t.Fatalf("configured origin was ignored: %s", out)
	}
}
