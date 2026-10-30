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

package horizon

import (
	"os"
	"strconv"
	"strings"
)

var (
	debugMode    bool
	jsonMode     bool
	bannerMode   bool
	insecureMode bool
)

// Config holds merged CLI defaults: optional file, then environment, then flags.
type Config struct {
	Domain               string
	Module               string
	Template             string
	BaseURL              string
	KeycloakBase         string
	KeycloakRealm        string
	RunningPollLimit     int
	WaitNewAttempts      int
	WaitNewSleepSec      int
	WaitTerminalSecs     int
	TerminalPollInterval int
	LogStreamMaxSecs     int
	LogWaitPodSecs       int
	LogStreamFormat      string
}

func EnvOr(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func TargetDebug() *bool { return &debugMode }

func TargetJSON() *bool { return &jsonMode }

func TargetNoBanner() *bool { return &bannerMode }

func TargetInsecure() *bool { return &insecureMode }

func IsDebug() bool { return debugMode }

func IsJSON() bool { return jsonMode }

func IsNoBanner() bool { return bannerMode }

func IsInsecure() bool { return insecureMode }
