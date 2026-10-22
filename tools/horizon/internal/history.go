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
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	HistoryDefaultLimit    = 10
	HistoryMaxLimit        = 100
	HistoryFileMaxLimit    = 1000        // Maximum entries to keep in the history file
	HistoryMaxSizeBytes    = 1024 * 1024 // 1MB maximum file size for rotation
	HistoryRetentionMonths = 3           // Number of months to keep history files
	historyFileName        = "history.json"
	redactedValue          = "***"
)

type HistoryEntry struct {
	Timestamp string `json:"timestamp"`
	Command   string `json:"command"`
}

func CliHistoryFilePath() (string, error) {
	dir, err := HorizonConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, historyFileName), nil
}

func AppendInvocation(argv []string) error {
	path, err := CliHistoryFilePath()
	if err != nil {
		return err
	}
	// Migrate existing history.json to time-based format if needed
	if err := migrateToTimeBased(path); err != nil {
		return err
	}
	return appendInvocation(path, argv, time.Now().UTC())
}

// Moves existing history.json to the current month's file
func migrateToTimeBased(path string) error {
	// Check if the default history file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil // No migration needed
	}

	dir := filepath.Dir(path)
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	nameWithoutExt := strings.TrimSuffix(base, ext)
	currentMonth := time.Now().Format("2006-01")
	currentMonthFile := filepath.Join(dir, nameWithoutExt+"-"+currentMonth+ext)

	// If current month file already exists, don't migrate
	if _, err := os.Stat(currentMonthFile); err == nil {
		return nil
	}

	// Read existing entries from the default file
	entries, err := readInvocations(path, 0)
	if err != nil {
		return err
	}

	// If there are entries, move them to the current month file
	if len(entries) > 0 {
		if err := writeInvocations(currentMonthFile, entries); err != nil {
			return err
		}
	}

	// Remove the old file
	return os.Remove(path)
}

// Check if the history file exceeds the maximum size, rotate it if needed
func rotateBySize(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if info.Size() > HistoryMaxSizeBytes {
		// n=0 reads every line. The display cap (HistoryMaxLimit) must not
		// apply here; file rotation keeps HistoryFileMaxLimit entries.
		entries, err := readInvocations(path, 0)
		if err != nil {
			return err
		}
		if len(entries) > HistoryFileMaxLimit {
			entries = entries[len(entries)-HistoryFileMaxLimit:]
		}
		return writeInvocations(path, entries)
	}
	return nil
}

// Keep only the most recent N entries in the history file
func rotateByCount(path string) error {
	// Read all entries to check if truncation is needed
	allEntries, err := readInvocations(path, 0) // 0 means read all
	if err != nil {
		return err
	}
	// Only rewrite if we actually need to truncate
	if len(allEntries) > HistoryFileMaxLimit {
		// Keep only the most recent entries
		entries := allEntries[len(allEntries)-HistoryFileMaxLimit:]
		return writeInvocations(path, entries)
	}
	return nil
}

// Rotate history files by date, keeping only the last N months
func rotateByTime(path string) error {
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	nameWithoutExt := strings.TrimSuffix(base, ext)

	// Clean up old month files (keep last N months)
	files, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	cutoffDate := time.Now().AddDate(0, -HistoryRetentionMonths, 0)
	for _, file := range files {
		if file.IsDir() {
			continue
		}
		name := file.Name()
		if strings.HasPrefix(name, nameWithoutExt+"-") && strings.HasSuffix(name, ext) {
			// Extract date from filename (format: nameWithoutExt-YYYY-MM.ext)
			dateStr := strings.TrimPrefix(name, nameWithoutExt+"-")
			dateStr = strings.TrimSuffix(dateStr, ext)
			fileDate, err := time.Parse("2006-01", dateStr)
			if err != nil {
				continue
			}
			if fileDate.Before(cutoffDate) {
				oldPath := filepath.Join(dir, name)
				os.Remove(oldPath)
			}
		}
	}

	return nil
}

// Write the given entries to the history file
func writeInvocations(path string, entries []HistoryEntry) error {
	if len(entries) == 0 {
		// If no entries, remove the file
		os.Remove(path)
		return nil
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	for _, entry := range entries {
		b, err := json.Marshal(entry)
		if err != nil {
			return err
		}
		if _, err := f.Write(append(b, '\n')); err != nil {
			return err
		}
	}
	return nil
}

func appendInvocation(path string, argv []string, at time.Time) error {
	if len(argv) == 0 {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}

	// Determine the target file (current month)
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	nameWithoutExt := strings.TrimSuffix(base, ext)
	currentMonth := at.Format("2006-01")
	currentMonthFile := filepath.Join(dir, nameWithoutExt+"-"+currentMonth+ext)

	// Apply rotation strategies to the target file
	if err := rotateBySize(currentMonthFile); err != nil {
		return err
	}
	if err := rotateByCount(currentMonthFile); err != nil {
		return err
	}
	if err := rotateByTime(path); err != nil {
		return err
	}

	entry := HistoryEntry{
		Timestamp: at.Format(time.RFC3339),
		Command:   FormatInvocation(argv),
	}
	b, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(currentMonthFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(b, '\n'))
	return err
}

func ReadInvocations(n int) ([]HistoryEntry, error) {
	path, err := CliHistoryFilePath()
	if err != nil {
		return nil, err
	}
	// Read from the current month's file
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	nameWithoutExt := strings.TrimSuffix(base, ext)
	currentMonth := time.Now().Format("2006-01")
	currentMonthFile := filepath.Join(dir, nameWithoutExt+"-"+currentMonth+ext)

	// If the current month file exists, read from it; otherwise read from the default file
	if _, err := os.Stat(currentMonthFile); err == nil {
		return readInvocations(currentMonthFile, n)
	}
	// Fall back to the default file if month file doesn't exist
	return readInvocations(path, n)
}

func readInvocations(path string, n int) ([]HistoryEntry, error) {
	// n=0 means read all entries (no limit)
	if n < 0 {
		return []HistoryEntry{}, nil
	}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []HistoryEntry{}, nil
		}
		return nil, err
	}
	defer f.Close()

	var all []HistoryEntry
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var e HistoryEntry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			continue
		}
		all = append(all, e)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	// If n is 0, return all entries
	if n == 0 {
		return all, nil
	}
	// Apply the read limit
	if n > HistoryMaxLimit {
		n = HistoryMaxLimit
	}
	if n >= len(all) {
		return all, nil
	}
	return all[len(all)-n:], nil
}

func FormatInvocation(argv []string) string {
	redacted := RedactArgs(argv)
	if len(redacted) == 0 {
		return ProgName()
	}
	redacted[0] = ProgName()
	parts := make([]string, len(redacted))
	for i, a := range redacted {
		parts[i] = quoteArg(a)
	}
	return strings.Join(parts, " ")
}

func quoteArg(s string) string {
	if s == "" || strings.ContainsAny(s, " \t\n\"'") {
		return strconv.Quote(s)
	}
	return s
}

func RedactArgs(argv []string) []string {
	out := make([]string, len(argv))
	copy(out, argv)
	for i := 0; i < len(out); i++ {
		a := out[i]
		if name, val, ok := strings.Cut(a, "="); ok && isSecretFlag(name) {
			if val != "" {
				out[i] = name + "=" + redactedValue
			}
			continue
		}
		if isSecretFlag(a) {
			if i+1 < len(out) && !looksLikeFlag(out[i+1]) {
				out[i+1] = redactedValue
				i++
			}
			continue
		}
		if looksLikeToken(a) {
			out[i] = redactedValue
		}
	}
	return out
}

func isSecretFlag(s string) bool {
	name := strings.TrimLeft(s, "-")
	if name == "" || name == s {
		return false
	}
	name = strings.ToLower(name)
	// Inline workflow parameters may embed credentials. Keep the flag and redact the value.
	if name == "params-json" {
		return true
	}
	for _, part := range strings.FieldsFunc(name, func(r rune) bool {
		return r == '-' || r == '_'
	}) {
		switch part {
		case "secret", "token", "password", "passwd", "authorization":
			return true
		}
	}
	return false
}

func looksLikeToken(s string) bool {
	if strings.HasPrefix(strings.ToLower(s), "bearer ") {
		return true
	}
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return false
	}
	if !strings.HasPrefix(s, "eyJ") {
		return false
	}
	return len(s) >= 40
}

func looksLikeFlag(s string) bool {
	if !strings.HasPrefix(s, "-") {
		return false
	}
	// Remove leading dashes
	trimmed := strings.TrimLeft(s, "-")
	if trimmed == "" {
		return false
	}
	// Validate if flag names contain only alphanumeric characters, hyphens, and underscores
	for _, r := range trimmed {
		if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_') {
			return false
		}
	}
	// Flag names shall be short (not 100+ characters)
	return len(trimmed) <= 50
}

func ParseHistoryLimit(arg string) (int, error) {
	n, err := strconv.Atoi(strings.TrimSpace(arg))
	if err != nil {
		return 0, fmt.Errorf("history count must be an integer between 1 and %d", HistoryMaxLimit)
	}
	if n < 1 || n > HistoryMaxLimit {
		return 0, fmt.Errorf("history count must be between 1 and %d", HistoryMaxLimit)
	}
	return n, nil
}
