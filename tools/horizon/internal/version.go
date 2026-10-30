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
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Stamped by mkdist / Docker ldflags. Local `go build` keeps these defaults.
var (
	version   = "dev"
	buildDate = "unknown"
)

type versionInfo struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	BuildDate string `json:"buildDate"`
}

func currentVersion() versionInfo {
	return versionInfo{Name: "horizon", Version: version, BuildDate: buildDate}
}

func formatBuildDate(value string) string {
	if value == "" {
		return "unknown"
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return "unknown"
	}
	return t.Format("02/01/2006")
}

func formatVersion(mode string, info versionInfo, raw bool) (string, error) {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", "text":
		return fmt.Sprintf("%s %s (built %s)\n", info.Name, info.Version, formatBuildDate(info.BuildDate)), nil
	case "json":
		var b []byte
		var err error
		if raw {
			b, err = json.Marshal(info)
		} else {
			b, err = json.MarshalIndent(info, "", "  ")
		}
		if err != nil {
			return "", err
		}
		return string(b) + "\n", nil
	default:
		return "", fmt.Errorf("unknown --output %q (want text or json)", mode)
	}
}

func FormatVersion(mode string, raw bool) (string, error) {
	return formatVersion(mode, currentVersion(), raw)
}

func GetCurrentName() string {
	return currentVersion().Name
}

func GetCurrentVersion() string {
	return currentVersion().Version
}

func GetCurrentBuildDate() string {
	return formatBuildDate(currentVersion().BuildDate)
}
