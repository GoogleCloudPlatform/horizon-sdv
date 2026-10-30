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
	"strings"
	"testing"
	"time"
)

func TestRedactArgs(t *testing.T) {
	jwt := "eyJhbGciOiJub25lIn0.eyJzdWIiOiJ1c2VyIn0.signature-padding-to-forty"
	if len(jwt) < 40 {
		t.Fatalf("fixture jwt len = %d", len(jwt))
	}
	in := []string{
		"horizon",
		"--token", "s3cret",
		"--password=hunter2",
		"--token=",
		"--client-secret", "abc",
		"--client-secret", "-abc_Secret123",
		"--token", "--abc_Secret123",
		"--client-secret=-abc_Secret123",
		"--my_password", "xyz",
		"--token", "--my_password", "xyz",
		"--api-token", "--next",
		"--password", "--page_token", "cursor",
		"Bearer session",
		jwt,
		"plain",
		"https://user:pass@example.com/v1",
		"https://:s3cret@redis.local/0",
		"--api=https://admin:s3cret@horizon.example/v1",
		"https://example.com/no-user",
		"https://example.com:8080",
		"--params-json", `{"sampleEnv":"jenkins","apiToken":"s3cret"}`,
		`--params-json={"password":"hunter2"}`,
		"--params-json=",
		"--params-json", "--wait",
		"--page-token", "token1",
		"--page-token=token2",
	}
	got := RedactArgs(in)
	if in[2] != "s3cret" {
		t.Fatal("RedactArgs mutated the input")
	}
	want := []string{
		"horizon",
		"--token", redactedValue,
		"--password=" + redactedValue,
		"--token=",
		"--client-secret", redactedValue,
		"--client-secret", redactedValue,
		"--token", redactedValue,
		"--client-secret=" + redactedValue,
		"--my_password", redactedValue,
		"--token", "--my_password", redactedValue,
		"--api-token", "--next",
		"--password", "--page_token", "cursor",
		redactedValue,
		redactedValue,
		"plain",
		"https://" + redactedValue + ":" + redactedValue + "@example.com/v1",
		"https://" + redactedValue + ":" + redactedValue + "@redis.local/0",
		"--api=https://" + redactedValue + ":" + redactedValue + "@horizon.example/v1",
		"https://example.com/no-user",
		"https://example.com:8080",
		"--params-json", redactedValue,
		"--params-json=" + redactedValue,
		"--params-json=",
		"--params-json", "--wait",
		"--page-token", "token1",
		"--page-token=token2",
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("got %#v\nwant %#v", got, want)
	}
}

func TestSecretAndFlagHeuristics(t *testing.T) {
	if isSecretFlag("token") || isSecretFlag("--") || isSecretFlag("--tokenizer") {
		t.Fatal("non-secret values classified as secrets")
	}
	if !isSecretFlag("--Authorization") || !isSecretFlag("--token-file") {
		t.Fatal("expected secret flag")
	}
	if !isSecretFlag("--params-json") || !isSecretFlag("--Params-Json") {
		t.Fatal("expected params-json to be redacted")
	}
	if isSecretFlag("params-json") || isSecretFlag("--params-file") || isSecretFlag("--params-json-file") {
		t.Fatal("non-flag or unrelated params flags classified as secrets")
	}
	if isSecretFlag("--page-token") || isSecretFlag("--page_token") || isSecretFlag("--Page-Token") {
		t.Fatal("page token classified as secret")
	}
	if !looksLikeFlag("--foo") || !looksLikeFlag("-n") || !looksLikeFlag("--page-token") || !looksLikeFlag("--page_token") || !looksLikeFlag("--Authorization") {
		t.Fatal("expected flag")
	}
	long := "--" + strings.Repeat("a", 51)
	if looksLikeFlag("-") || looksLikeFlag("--") || looksLikeFlag("--foo bar") || looksLikeFlag(long) || looksLikeFlag("foo") {
		t.Fatal("unexpected flag match")
	}
	for _, secret := range []string{"-abc_Secret123", "--abc_Secret123", "--abc-Secret123", "-abc", "--9abc", "--a1b2c3"} {
		if looksLikeFlag(secret) {
			t.Fatalf("secret-shaped value classified as flag: %s", secret)
		}
	}
	if !looksLikeToken("Bearer abc") || !looksLikeToken("bearer abc") {
		t.Fatal("expected bearer token")
	}
	if looksLikeToken("eyJ.short.sig") || looksLikeToken("aaa.bbb.ccc") {
		t.Fatal("short or non-jwt values should not match")
	}
}

func TestDashPrefixedSecretAndURLNotStored(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.json")
	secret := "-abc_Secret123"
	embedded := "--abc_Secret123"
	at := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	argv := []string{
		"horizon", "auth", "login",
		"--client-secret", secret,
		"--token", embedded,
		"--api", "https://user:pass@example.com/v1",
	}
	if err := appendInvocation(path, argv, at); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "history-2026-10.json"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, leak := range []string{secret, embedded, "user:pass", "pass@"} {
		if strings.Contains(text, leak) {
			t.Fatalf("plaintext secret in history file: %s", text)
		}
	}
	if !strings.Contains(text, "example.com/v1") || !strings.Contains(text, redactedValue) {
		t.Fatalf("history lost the host or redaction marker: %s", text)
	}
}

func TestFormatInvocation(t *testing.T) {
	got := FormatInvocation(nil)
	if got != ProgName() {
		t.Fatalf("empty argv = %q", got)
	}
	got = FormatInvocation([]string{"ignored", "workflow", "submit", "--token", "secret", "has space"})
	want := ProgName() + " workflow submit --token " + redactedValue + ` "has space"`
	if got != want {
		t.Fatalf("got %q\nwant %q", got, want)
	}
	payload := `{"sampleEnv":"jenkins","note":"has space"}`
	got = FormatInvocation([]string{"ignored", "workflow", "submit", "--params-json", payload, "--wait"})
	want = ProgName() + " workflow submit --params-json " + redactedValue + " --wait"
	if got != want {
		t.Fatalf("got %q\nwant %q", got, want)
	}
	if strings.Contains(got, "jenkins") || strings.Contains(got, "has space") {
		t.Fatalf("params-json leaked into invocation: %q", got)
	}
	if quoteArg("") != `""` || quoteArg("plain") != "plain" {
		t.Fatalf("quoteArg empty/plain = %q %q", quoteArg(""), quoteArg("plain"))
	}
}

func TestParamsJSONRedactedInHistory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.json")
	payload := `{"sampleEnv":"jenkins","apiToken":"s3cret-json","note":"has space"}`
	at := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	argv := []string{
		"horizon", "workflow", "submit",
		"--module", "sample",
		"--params-json", payload,
		"--params-json=" + payload,
		"--wait",
	}
	if err := appendInvocation(path, argv, at); err != nil {
		t.Fatal(err)
	}
	month := filepath.Join(dir, "history-2026-10.json")
	raw, err := os.ReadFile(month)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, leak := range []string{"s3cret-json", "jenkins", "has space", payload} {
		if strings.Contains(text, leak) {
			t.Fatalf("plaintext params-json in history file: %s", text)
		}
	}
	entries, err := readInvocations(month, 0)
	if err != nil || len(entries) != 1 {
		t.Fatalf("entries = %#v %v", entries, err)
	}
	want := ProgName() + " workflow submit --module sample --params-json " + redactedValue + " --params-json=" + redactedValue + " --wait"
	if entries[0].Command != want {
		t.Fatalf("command = %q\nwant %q", entries[0].Command, want)
	}
}

func TestResolveHistoryMonth(t *testing.T) {
	now := time.Date(2026, 10, 1, 0, 30, 0, 0, time.FixedZone("CET", 3600))
	got, err := resolveHistoryMonth("", now)
	if err != nil {
		t.Fatal(err)
	}
	if got.Format("2006-01") != "2026-09" {
		t.Fatalf("current UTC month = %s", got.Format("2006-01"))
	}
	got, err = resolveHistoryMonth(" 2026-08 ", now)
	if err != nil || got.Format("2006-01") != "2026-08" {
		t.Fatalf("retained month = %s %v", got.Format("2006-01"), err)
	}
	got, err = resolveHistoryMonth("2026-07", now)
	if err != nil || got.Format("2006-01") != "2026-07" {
		t.Fatalf("oldest retained = %s %v", got.Format("2006-01"), err)
	}
	for _, month := range []string{"2026-06", "2026-10", "2026-9", "October"} {
		if _, err := resolveHistoryMonth(month, now); err == nil {
			t.Fatalf("expected error for %q", month)
		}
	}
}

func TestReadMonthInvocations(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.json")
	now := time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)
	files := map[string]string{
		"history.json":         "legacy",
		"history-2026-08.json": "aug",
		"history-2026-09.json": "sep",
		"history-2026-10.json": "oct",
	}
	for name, command := range files {
		entry := HistoryEntry{Timestamp: "2026-10-01T00:00:00Z", Command: command}
		if err := writeInvocations(filepath.Join(dir, name), []HistoryEntry{entry}); err != nil {
			t.Fatal(err)
		}
	}

	current, err := readMonthInvocations(path, 10, "", now)
	if err != nil || len(current) != 1 || current[0].Command != "oct" {
		t.Fatalf("current = %#v %v", current, err)
	}
	prev, err := readMonthInvocations(path, 10, "2026-09", now)
	if err != nil || len(prev) != 1 || prev[0].Command != "sep" {
		t.Fatalf("previous = %#v %v", prev, err)
	}
	oldest, err := readMonthInvocations(path, 1, "2026-08", now)
	if err != nil || len(oldest) != 1 || oldest[0].Command != "aug" {
		t.Fatalf("oldest = %#v %v", oldest, err)
	}
	if _, err := readMonthInvocations(path, 10, "2026-07", now); err == nil {
		t.Fatal("expected retention error")
	}

	if err := os.Remove(filepath.Join(dir, "history-2026-09.json")); err != nil {
		t.Fatal(err)
	}
	missing, err := readMonthInvocations(path, 10, "2026-09", now)
	if err != nil || len(missing) != 0 {
		t.Fatalf("missing previous month = %#v %v", missing, err)
	}

	if err := os.Remove(filepath.Join(dir, "history-2026-10.json")); err != nil {
		t.Fatal(err)
	}
	legacy, err := readMonthInvocations(path, 10, "", now)
	if err != nil || len(legacy) != 1 || legacy[0].Command != "legacy" {
		t.Fatalf("legacy fallback = %#v %v", legacy, err)
	}
	explicitCurrent, err := readMonthInvocations(path, 10, "2026-10", now)
	if err != nil || len(explicitCurrent) != 1 || explicitCurrent[0].Command != "legacy" {
		t.Fatalf("explicit current fallback = %#v %v", explicitCurrent, err)
	}
}

func TestParseHistoryLimit(t *testing.T) {
	n, err := ParseHistoryLimit(" 5 ")
	if err != nil || n != 5 {
		t.Fatalf("got %d %v", n, err)
	}
	if _, err := ParseHistoryLimit("0"); err == nil {
		t.Fatal("expected range error")
	}
	if _, err := ParseHistoryLimit("101"); err == nil {
		t.Fatal("expected max error")
	}
	if _, err := ParseHistoryLimit("nope"); err == nil {
		t.Fatal("expected parse error")
	}
	if _, err := ParseHistoryLimit("1"); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseHistoryLimit("100"); err != nil {
		t.Fatal(err)
	}
}

func TestHistoryReadWriteAndLimits(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.json")

	if err := appendInvocation(path, nil, time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	if err := appendInvocation(path, []string{"horizon", "version"}, at); err != nil {
		t.Fatal(err)
	}
	if err := appendInvocation(path, []string{"horizon", "auth", "--token", "s3cret"}, at.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	month := filepath.Join(dir, "history-2026-10.json")
	entries, err := readInvocations(month, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("entries = %#v", entries)
	}
	if !strings.Contains(entries[1].Command, redactedValue) || strings.Contains(entries[1].Command, "s3cret") {
		t.Fatalf("command = %q", entries[1].Command)
	}
	if entries[0].Timestamp != "2026-10-09T12:00:00Z" {
		t.Fatalf("timestamp = %q", entries[0].Timestamp)
	}

	last, err := readInvocations(month, 1)
	if err != nil || len(last) != 1 || last[0].Timestamp != entries[1].Timestamp {
		t.Fatalf("last = %#v %v", last, err)
	}
	capped, err := readInvocations(month, HistoryMaxLimit+10)
	if err != nil || len(capped) != 2 {
		t.Fatalf("capped = %#v %v", capped, err)
	}
	if got, err := readInvocations(month, -1); err != nil || len(got) != 0 {
		t.Fatalf("negative = %#v %v", got, err)
	}
	missing, err := readInvocations(filepath.Join(dir, "nope.json"), 5)
	if err != nil || len(missing) != 0 {
		t.Fatalf("missing = %#v %v", missing, err)
	}

	if err := os.WriteFile(filepath.Join(dir, "dirty.json"), []byte("\nnot-json\n{\"timestamp\":\"t\",\"command\":\"ok\"}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	dirty, err := readInvocations(filepath.Join(dir, "dirty.json"), 0)
	if err != nil || len(dirty) != 1 || dirty[0].Command != "ok" {
		t.Fatalf("dirty = %#v %v", dirty, err)
	}

	if err := writeInvocations(month, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(month); !os.IsNotExist(err) {
		t.Fatalf("empty write should remove file, err=%v", err)
	}
}

func TestRotateByCount(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history-2026-10.json")
	entries := make([]HistoryEntry, HistoryFileMaxLimit+3)
	for i := range entries {
		entries[i] = HistoryEntry{Timestamp: "2026-10-09T00:00:00Z", Command: "cmd"}
	}
	if err := writeInvocations(path, entries); err != nil {
		t.Fatal(err)
	}
	if err := rotateByCount(path); err != nil {
		t.Fatal(err)
	}
	got, err := readInvocations(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != HistoryFileMaxLimit {
		t.Fatalf("len = %d", len(got))
	}
	if err := rotateByCount(path); err != nil {
		t.Fatal(err)
	}
	again, err := readInvocations(path, 0)
	if err != nil || len(again) != HistoryFileMaxLimit {
		t.Fatalf("second rotate len = %d %v", len(again), err)
	}
}

func TestRotateBySize(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history-2026-10.json")
	entries := make([]HistoryEntry, HistoryFileMaxLimit+1)
	// Stay under the line scanner limit while making the file larger than the rotation threshold.
	entries[0] = HistoryEntry{Timestamp: "2026-10-01T00:00:00Z", Command: strings.Repeat("a", 1000*1024)}
	for i := 1; i < len(entries); i++ {
		entries[i] = HistoryEntry{Timestamp: "2026-10-02T00:00:00Z", Command: "keep"}
	}
	if err := writeInvocations(path, entries); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() <= HistoryMaxSizeBytes {
		t.Fatalf("fixture size = %d", info.Size())
	}
	if err := rotateBySize(path); err != nil {
		t.Fatal(err)
	}
	got, err := readInvocations(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != HistoryFileMaxLimit {
		t.Fatalf("len = %d", len(got))
	}
	for _, e := range got {
		if e.Command != "keep" {
			t.Fatal("rotation kept the oversized prefix")
		}
	}
	info, err = os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() > HistoryMaxSizeBytes {
		t.Fatalf("rotated size = %d", info.Size())
	}
	if err := rotateBySize(filepath.Join(dir, "missing.json")); err != nil {
		t.Fatal(err)
	}
}

func TestRotateByTime(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.json")
	keep := []string{
		"history-" + time.Now().Format("2006-01") + ".json",
		"history-notes.json",
		"other.json",
	}
	drop := []string{
		"history-" + time.Now().AddDate(-2, 0, 0).Format("2006-01") + ".json",
	}
	for _, name := range append(append([]string{}, keep...), drop...) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("{}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "history-2020-01.json.dir"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := rotateByTime(path); err != nil {
		t.Fatal(err)
	}
	for _, name := range keep {
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Fatalf("expected to keep %s: %v", name, err)
		}
	}
	for _, name := range drop {
		if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Fatalf("expected to drop %s, err=%v", name, err)
		}
	}
}

func TestMigrateToTimeBased(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "history.json")
	if err := migrateToTimeBased(path); err != nil {
		t.Fatal(err)
	}
	entries := []HistoryEntry{{Timestamp: "2026-10-09T00:00:00Z", Command: "old"}}
	if err := writeInvocations(path, entries); err != nil {
		t.Fatal(err)
	}
	if err := migrateToTimeBased(path); err != nil {
		t.Fatal(err)
	}
	month := filepath.Join(dir, "history-"+time.Now().Format("2006-01")+".json")
	got, err := readInvocations(month, 0)
	if err != nil || len(got) != 1 || got[0].Command != "old" {
		t.Fatalf("migrated = %#v %v", got, err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("legacy file still present: %v", err)
	}

	if err := writeInvocations(path, entries); err != nil {
		t.Fatal(err)
	}
	if err := migrateToTimeBased(path); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("legacy file should stay when month file exists: %v", err)
	}
}
