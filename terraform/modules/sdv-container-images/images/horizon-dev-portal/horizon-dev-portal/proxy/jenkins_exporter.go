// Copyright (c) 2024-2026 Accenture, All Rights Reserved.
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

// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

type JenkinsBuildItem struct {
	ID          string            `json:"id"`
	JobPath     string            `json:"jobPath"`
	Category    string            `json:"category"`
	BuildNumber int               `json:"buildNumber"`
	Result      string            `json:"result"`
	TimestampMs int64             `json:"timestampMs"`
	StartedAt   string            `json:"startedAt"`
	DurationMs  int64             `json:"durationMs"`
	Node        string            `json:"node,omitempty"`
	WebPath     string            `json:"webPath"`
	Parameters  map[string]string `json:"parameters,omitempty"`
}

var (
	xmlResultTagRegex    = regexp.MustCompile(`<result>\s*([^<\s]+)\s*</result>`)
	xmlTimestampTagRegex = regexp.MustCompile(`<timestamp>\s*([0-9]+)\s*</timestamp>`)
	xmlDurationTagRegex  = regexp.MustCompile(`<duration>\s*([0-9]+)\s*</duration>`)
	xmlBuiltOnTagRegex   = regexp.MustCompile(`<builtOn>\s*([^<]*)\s*</builtOn>`)
	xmlNodeTagRegex      = regexp.MustCompile(`<node>\s*([^<]+)\s*</node>`)
	xmlParamPairRegex    = regexp.MustCompile(`(?s)<hudson\.model\.(?:String|Boolean|Text)ParameterValue>.*?<name>\s*([^<]+)\s*</name>.*?<value>\s*([^<]*)\s*</value>.*?</hudson\.model\.(?:String|Boolean|Text)ParameterValue>`)
)

func runJenkinsExporterServer(listenAddr, jobsDir string) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /builds", func(w http.ResponseWriter, r *http.Request) {
		builds, err := scanJenkinsBuildsDir(jobsDir)
		if err != nil {
			log.Printf("scan jenkins builds in %q: %v", jobsDir, err)
			http.Error(w, "failed to scan jenkins builds", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"items": builds,
		})
	})
	log.Printf("jenkins exporter listening on %s (jobsDir=%s)", listenAddr, jobsDir)
	log.Fatal(http.ListenAndServe(listenAddr, mux))
}

func scanJenkinsBuildsDir(jobsDir string) ([]JenkinsBuildItem, error) {
	cleanRoot := filepath.Clean(jobsDir)
	if _, err := os.Stat(cleanRoot); err != nil {
		return nil, err
	}
	var builds []JenkinsBuildItem
	err := filepath.WalkDir(cleanRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if d.IsDir() || d.Name() != "build.xml" {
			return nil
		}
		item, ok := parseJenkinsBuildFile(cleanRoot, path)
		if !ok {
			return nil
		}
		builds = append(builds, item)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(builds, func(i, j int) bool {
		if builds[i].TimestampMs != builds[j].TimestampMs {
			return builds[i].TimestampMs > builds[j].TimestampMs
		}
		return builds[i].ID > builds[j].ID
	})
	return builds, nil
}

func parseJenkinsBuildFile(rootDir, filePath string) (JenkinsBuildItem, bool) {
	relPath, err := filepath.Rel(rootDir, filePath)
	if err != nil {
		return JenkinsBuildItem{}, false
	}
	slashRel := filepath.ToSlash(relPath)
	buildsIdx := strings.LastIndex(slashRel, "/builds/")
	if buildsIdx <= 0 {
		return JenkinsBuildItem{}, false
	}
	rawJobSection := slashRel[:buildsIdx]
	remainder := slashRel[buildsIdx+len("/builds/"):]
	buildNumStr := strings.TrimSuffix(remainder, "/build.xml")
	buildNum, err := strconv.Atoi(buildNumStr)
	if err != nil || buildNum <= 0 {
		return JenkinsBuildItem{}, false
	}

	rawSegments := strings.Split(rawJobSection, "/jobs/")
	var cleanSegments []string
	var urlSegments []string
	for _, seg := range rawSegments {
		trimmed := strings.TrimSpace(seg)
		if trimmed == "" {
			continue
		}
		cleanSegments = append(cleanSegments, trimmed)
		urlSegments = append(urlSegments, "job/"+url.PathEscape(trimmed))
	}
	if len(cleanSegments) == 0 {
		return JenkinsBuildItem{}, false
	}

	rawBytes, err := os.ReadFile(filePath)
	if err != nil {
		return JenkinsBuildItem{}, false
	}

	timestampMs := extractFirstInt64(xmlTimestampTagRegex, rawBytes)
	durationMs := extractFirstInt64(xmlDurationTagRegex, rawBytes)
	result := extractTopLevelResult(rawBytes, durationMs)
	node := extractExecutionNode(rawBytes)
	params := extractInterestingParams(rawBytes)

	startedAt := ""
	if timestampMs > 0 {
		startedAt = time.UnixMilli(timestampMs).UTC().Format(time.RFC3339)
	}
	jobPath := strings.Join(cleanSegments, " / ")
	webPath := "/jenkins/" + strings.Join(urlSegments, "/") + "/" + strconv.Itoa(buildNum) + "/"

	return JenkinsBuildItem{
		ID:          jobPath + " #" + strconv.Itoa(buildNum),
		JobPath:     jobPath,
		Category:    cleanSegments[0],
		BuildNumber: buildNum,
		Result:      result,
		TimestampMs: timestampMs,
		StartedAt:   startedAt,
		DurationMs:  durationMs,
		Node:        node,
		WebPath:     webPath,
		Parameters:  params,
	}, true
}

func extractFirstInt64(re *regexp.Regexp, content []byte) int64 {
	match := re.FindSubmatch(content)
	if len(match) < 2 {
		return 0
	}
	val, err := strconv.ParseInt(string(match[1]), 10, 64)
	if err != nil {
		return 0
	}
	return val
}

func extractTopLevelResult(content []byte, durationMs int64) string {
	tailStart := 0
	if len(content) > 2048 {
		tailStart = len(content) - 2048
	}
	tailMatches := xmlResultTagRegex.FindAllSubmatch(content[tailStart:], -1)
	if len(tailMatches) > 0 {
		last := tailMatches[len(tailMatches)-1]
		return strings.ToUpper(strings.TrimSpace(string(last[1])))
	}
	allMatches := xmlResultTagRegex.FindAllSubmatch(content, -1)
	if len(allMatches) > 0 && durationMs > 0 {
		last := allMatches[len(allMatches)-1]
		return strings.ToUpper(strings.TrimSpace(string(last[1])))
	}
	return "RUNNING"
}

func extractExecutionNode(content []byte) string {
	builtOn := xmlBuiltOnTagRegex.FindSubmatch(content)
	if len(builtOn) >= 2 {
		candidate := strings.TrimSpace(string(builtOn[1]))
		if candidate != "" {
			return candidate
		}
	}
	nodes := xmlNodeTagRegex.FindAllSubmatch(content, -1)
	for i := len(nodes) - 1; i >= 0; i-- {
		candidate := strings.TrimSpace(string(nodes[i][1]))
		if candidate != "" {
			return candidate
		}
	}
	return ""
}

func extractInterestingParams(content []byte) map[string]string {
	if !bytes.Contains(content, []byte("ParametersAction")) {
		return nil
	}
	matches := xmlParamPairRegex.FindAllSubmatch(content, 12)
	if len(matches) == 0 {
		return nil
	}
	allowedKeys := map[string]bool{
		"JENKINS_GCE_CLOUD_LABEL": true,
		"CUTTLEFISH_DOWNLOAD_URL": true,
		"AAOS_LUNCH_TARGET":       true,
		"AAOS_REVISION":           true,
		"AAOS_ARCH":               true,
		"WORKSTATION_NAME":        true,
		"WORKSTATION_CONFIG_NAME": true,
		"CLUSTER_NAME":            true,
	}
	params := make(map[string]string)
	for _, m := range matches {
		if len(m) < 3 {
			continue
		}
		k := strings.TrimSpace(string(m[1]))
		v := strings.TrimSpace(string(m[2]))
		if allowedKeys[k] && v != "" {
			params[k] = v
		}
	}
	if len(params) == 0 {
		return nil
	}
	return params
}
