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
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = old }()
	fn()
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestMergeSubmitParamsFromCatalog(t *testing.T) {
	entry := &catalogEntry{
		Module: "mod", TemplateName: "tmpl",
		Parameters: []catalogParameter{
			{Name: "image", Default: "latest"},
			{Name: "note", Default: ""},
			{Name: "count", Default: "1"},
		},
	}
	got, err := mergeSubmitParamsFromCatalog(map[string]any{"count": ""}, entry)
	if err != nil {
		t.Fatal(err)
	}
	if got["image"] != "latest" || got["note"] != "" || got["count"] != "1" {
		t.Fatalf("defaults = %#v", got)
	}
	got, err = mergeSubmitParamsFromCatalog(map[string]any{"note": "", "image": "custom", "count": 3}, entry)
	if err != nil {
		t.Fatal(err)
	}
	if got["note"] != "" || got["image"] != "custom" || got["count"] != 3 {
		t.Fatalf("overrides = %#v", got)
	}
	if _, err := mergeSubmitParamsFromCatalog(map[string]any{"extra": true}, entry); err == nil || !strings.Contains(err.Error(), "unknown workflow parameter") {
		t.Fatalf("err = %v", err)
	}
	if !catalogHasParam(entry, "image") || catalogHasParam(entry, "missing") {
		t.Fatal("catalogHasParam")
	}
}

func TestStringParamEmptyAndFromAny(t *testing.T) {
	if !stringParamEmpty(nil) || !stringParamEmpty("") || stringParamEmpty("x") || stringParamEmpty(0) {
		t.Fatal("stringParamEmpty")
	}
	if stringFromAny(nil) != "" || stringFromAny("  hi ") != "hi" || stringFromAny(float64(2)) != "2" {
		t.Fatalf("stringFromAny nil=%q str=%q num=%q", stringFromAny(nil), stringFromAny("  hi "), stringFromAny(float64(2)))
	}
	if singleLine(" a\r\nb ") != "a  b" {
		t.Fatalf("singleLine = %q", singleLine(" a\r\nb "))
	}
}

func TestReadParamsJSON(t *testing.T) {
	got, err := readParamsJSON("", "")
	if err != nil || len(got) != 0 {
		t.Fatalf("default = %#v %v", got, err)
	}
	got, err = readParamsJSON("", " \n{\"a\":1}\n ")
	if err != nil || got["a"] != float64(1) {
		t.Fatalf("env = %#v %v", got, err)
	}
	got, err = readParamsJSON("", "null")
	if err != nil || len(got) != 0 {
		t.Fatalf("null = %#v %v", got, err)
	}
	if _, err := readParamsJSON("", "{"); err == nil {
		t.Fatal("expected JSON error")
	}

	path := filepath.Join(t.TempDir(), "params.json")
	if err := os.WriteFile(path, []byte(`{"from":"file"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err = readParamsJSON(path, `{"from":"env"}`)
	if err != nil || got["from"] != "file" {
		t.Fatalf("file wins = %#v %v", got, err)
	}

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = old }()
	if _, err := w.Write([]byte(`{"from":"stdin"}`)); err != nil {
		t.Fatal(err)
	}
	w.Close()
	got, err = readParamsJSON("-", "")
	if err != nil || got["from"] != "stdin" {
		t.Fatalf("stdin = %#v %v", got, err)
	}
}

func TestPickNewNameAndPhases(t *testing.T) {
	if pickNewName([]string{"a"}, []string{"a", "b", "c"}) != "b" {
		t.Fatal("expected first unseen name")
	}
	if pickNewName([]string{"a"}, []string{"a"}) != "" || pickNewName(nil, []string{"z"}) != "z" {
		t.Fatal("pickNewName edges")
	}
	for _, ph := range []string{"Succeeded", "Failed", "Error", "Aborted"} {
		if !terminalPhase(ph) {
			t.Fatalf("%s should be terminal", ph)
		}
	}
	if terminalPhase("Running") || terminalPhase("") {
		t.Fatal("non-terminal phase")
	}
	cases := map[string]int{"Succeeded": 0, "Aborted": 2, "Failed": 1, "Error": 1, "Running": 1, "": 1}
	for ph, want := range cases {
		if got := phaseToExitCode(ph); got != want {
			t.Fatalf("phase %q -> %d, want %d", ph, got, want)
		}
	}
}

func TestWorkflowResult(t *testing.T) {
	if err := WorkflowResult(0, nil); err != nil {
		t.Fatal(err)
	}
	sentinel := errors.New("boom")
	if err := WorkflowResult(0, sentinel); !errors.Is(err, sentinel) {
		t.Fatalf("err = %v", err)
	}
	err := WorkflowResult(2, nil)
	var exit *ExitError
	if !errors.As(err, &exit) || exit.Code != 2 || exit.Error() != "exit status 2" {
		t.Fatalf("err = %#v", err)
	}
}

func TestFormatNDJSONLine(t *testing.T) {
	if out, ok := formatNDJSONLine(nil, []byte(" \n"), true); ok || out != nil {
		t.Fatalf("blank = %q %v", out, ok)
	}
	out, ok := formatNDJSONLine([]byte("raw"), []byte("raw"), false)
	if !ok || string(out) != "raw\n" {
		t.Fatalf("raw = %q %v", out, ok)
	}
	out, ok = formatNDJSONLine([]byte("not-json"), []byte("not-json\n"), true)
	if !ok || string(out) != "not-json\n" {
		t.Fatalf("passthrough = %q %v", out, ok)
	}
	hb := []byte(`{"heartbeat":true}`)
	if out, ok = formatNDJSONLine(hb, append(append([]byte{}, hb...), '\n'), true); ok || out != nil {
		t.Fatalf("heartbeat = %q %v", out, ok)
	}
	done := []byte(`{"result":"done","workflowStatus":"Succeeded","detail":"all good"}`)
	out, ok = formatNDJSONLine(done, done, true)
	if !ok || string(out) != "[Succeeded] [-] [all good]\n" {
		t.Fatalf("done = %q %v", out, ok)
	}
	out, ok = formatNDJSONLine([]byte(`{"result":"done"}`), []byte(`{"result":"done"}`), true)
	if !ok || string(out) != "[done] [-] [-]\n" {
		t.Fatalf("empty done = %q %v", out, ok)
	}
	line := []byte(`{"ts":"12:00","msg":"hello\nworld","displayName":"step"}`)
	out, ok = formatNDJSONLine(line, line, true)
	if !ok || string(out) != "[step] [12:00] [hello world]\n" {
		t.Fatalf("log = %q %v", out, ok)
	}
	line = []byte(`{"msg":"x","podName":"pod"}`)
	out, ok = formatNDJSONLine(line, line, true)
	if !ok || string(out) != "[pod] [-] [x]\n" {
		t.Fatalf("pod = %q %v", out, ok)
	}
	if trimCRLFLine([]byte("a\r\n")) == nil || string(trimCRLFLine([]byte("a\r\n"))) != "a" {
		t.Fatalf("trim = %q", trimCRLFLine([]byte("a\r\n")))
	}
}

func TestFormatIECBytesAndShortenPath(t *testing.T) {
	cases := map[int64]string{
		-1:                 "?",
		0:                  "0 B",
		1023:               "1023 B",
		1024:               "1.00 KiB",
		1536:               "1.50 KiB",
		100 * 1024:         "100 KiB",
		1024 * 1024:        "1.00 MiB",
		1024 * 1024 * 1024: "1.00 GiB",
	}
	for n, want := range cases {
		if got := formatIECBytes(n); got != want {
			t.Fatalf("formatIECBytes(%d) = %q, want %q", n, got, want)
		}
	}
	if shortenPath("hello", 10) != "hello" {
		t.Fatal("short path")
	}
	if shortenPath("abcdefghijklmnop", 4) != "abcd" {
		t.Fatalf("tiny max = %q", shortenPath("abcdefghijklmnop", 4))
	}
	if got := shortenPath("abcdefghijklmnopqrstuvwxyz", 12); got != "abcd...vwxyz" {
		t.Fatalf("shorten = %q", got)
	}
}

func TestCountingWriterAndDownloadProgress(t *testing.T) {
	var n int64
	var buf bytes.Buffer
	cw := &countingWriter{w: &buf, n: &n}
	if _, err := cw.Write([]byte("ab")); err != nil {
		t.Fatal(err)
	}
	if _, err := cw.Write(nil); err != nil {
		t.Fatal(err)
	}
	if n != 2 || buf.String() != "ab" {
		t.Fatalf("n=%d buf=%q", n, buf.String())
	}

	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	var progress bytes.Buffer
	done := make(chan struct{})
	go func() {
		_, _ = io.Copy(&progress, pr)
		close(done)
	}()
	var out bytes.Buffer
	written, err := copyDownloadWithProgress(strings.NewReader("hello"), &out, 5, "dest.bin", pw)
	pw.Close()
	<-done
	if err != nil || written != 5 || out.String() != "hello" {
		t.Fatalf("written=%d out=%q err=%v", written, out.String(), err)
	}
	if !strings.Contains(progress.String(), "download complete: 5 B (5 B)") || !strings.Contains(progress.String(), "dest.bin") {
		t.Fatalf("progress = %q", progress.String())
	}
}

func TestCatalogAndWorkflowParsing(t *testing.T) {
	var gotPath string
	c, _ := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		switch r.URL.Path {
		case "/v1/catalog":
			_, _ = w.Write([]byte(`{"entries":[{"module":"mod","templateName":"tmpl","parameters":[{"name":"image","default":"latest"}]}]}`))
		case "/v1/workflows/running":
			_, _ = w.Write([]byte(`{"items":[{"name":"old"},{"name":""},{"name":"new"}]}`))
		case "/v1/workflows/wf":
			_, _ = w.Write([]byte(`{"phase":"Running","nodes":[{"podName":"pod-1"},{"podName":"  "},{"podName":null}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	entry, err := findCatalogEntry(context.Background(), c, "mod", "tmpl")
	if err != nil || entry.Parameters[0].Default != "latest" {
		t.Fatalf("entry = %+v %v", entry, err)
	}
	if _, err := findCatalogEntry(context.Background(), c, "mod", "missing"); err == nil {
		t.Fatal("expected missing catalog entry")
	}
	names, err := runningNames(context.Background(), c, &Config{RunningPollLimit: 7})
	if err != nil || strings.Join(names, ",") != "old,new" {
		t.Fatalf("names = %#v %v", names, err)
	}
	if !strings.Contains(gotPath, "/v1/workflows/running") {
		t.Fatalf("path = %q", gotPath)
	}
	ph, err := workflowPhase(context.Background(), c, "wf")
	if err != nil || ph != "Running" {
		t.Fatalf("phase = %q %v", ph, err)
	}
	ok, err := hasPodHint(context.Background(), c, "wf")
	if err != nil || !ok {
		t.Fatalf("pod hint = %v %v", ok, err)
	}
}

func TestWaitHelpers(t *testing.T) {
	c, _ := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/running"):
			_, _ = w.Write([]byte(`{"items":[{"name":"old"},{"name":"new"}]}`))
		default:
			_, _ = w.Write([]byte(`{"phase":"Succeeded"}`))
		}
	}))
	cfg := &Config{RunningPollLimit: 10, WaitNewAttempts: 1, WaitNewSleepSec: 0, WaitTerminalSecs: 30, TerminalPollInterval: 1}
	name, err := waitForNewRunning(context.Background(), c, cfg, []string{"old"})
	if err != nil || name != "new" {
		t.Fatalf("name = %q %v", name, err)
	}
	code, err := CmdWait(c, cfg, "wf", false)
	if err != nil || code != 0 {
		t.Fatalf("wait = %d %v", code, err)
	}

	same, _ := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"items":[{"name":"old"}]}`))
	}))
	if _, err := waitForNewRunning(context.Background(), same, cfg, []string{"old"}); err == nil {
		t.Fatal("expected timeout waiting for a new workflow")
	}

	stuck, _ := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"phase":"Running"}`))
	}))
	if _, err := pollUntilTerminal(context.Background(), stuck, &Config{WaitTerminalSecs: 0, TerminalPollInterval: 1}, "wf"); err == nil {
		t.Fatal("expected terminal timeout")
	}
}

func TestPrintHelpers(t *testing.T) {
	out := captureStdout(t, func() {
		if err := printCatalogText([]byte(`{"entries":[]}`)); err != nil {
			t.Fatal(err)
		}
	})
	if out != "Catalog: (no entries)\n" {
		t.Fatalf("empty catalog = %q", out)
	}
	out = captureStdout(t, func() {
		err := printCatalogText([]byte(`{"entries":[{"module":"mod","templateName":"tmpl","namespace":"ns","parameters":[{"name":"image","default":"latest","description":"which image"}]}]}`))
		if err != nil {
			t.Fatal(err)
		}
	})
	for _, part := range []string{"Catalog (1 entry)", "mod / tmpl", "namespace: ns", `image (default: "latest")`, "which image"} {
		if !strings.Contains(out, part) {
			t.Fatalf("missing %q in %q", part, out)
		}
	}
	if err := printCatalogText([]byte("{")); err == nil {
		t.Fatal("expected catalog JSON error")
	}

	out = captureStdout(t, func() {
		err := printWorkflowShowText([]byte(`{"name":"wf","phase":"Succeeded","archivedLogs":{"combined":{"gcsUri":"gs://b/combined"},"steps":[{"gcsUri":"gs://b/step"}]},"outputArtifacts":[{"name":"report","gcsUri":"gs://b/report"}]}`))
		if err != nil {
			t.Fatal(err)
		}
	})
	for _, part := range []string{"Workflow detail", "gs://b/combined", "gs://b/step", "report gs://b/report"} {
		if !strings.Contains(out, part) {
			t.Fatalf("missing %q in %q", part, out)
		}
	}

	out = captureStdout(t, func() {
		printSubmitAckHuman([]byte(`{"status":"dispatched"}`), OutputTextFormat, false)
	})
	if !strings.Contains(out, "Submit accepted") {
		t.Fatalf("ack = %q", out)
	}
	out = captureStdout(t, func() {
		printSubmitAckHuman([]byte(`not-json`), OutputTextFormat, false)
	})
	if !strings.Contains(out, "not-json") {
		t.Fatalf("raw ack = %q", out)
	}
	out = captureStdout(t, func() {
		printSubmitAckHuman([]byte(`{"status":"dispatched"}`), OutputTextFormat, true)
	})
	if out != "" {
		t.Fatalf("quiet ack = %q", out)
	}
}

func TestCmdCatalogAndAbort(t *testing.T) {
	var posts atomic.Int32
	c, _ := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			posts.Add(1)
		}
		switch {
		case strings.Contains(r.URL.Path, "/abort"):
			_, _ = w.Write([]byte(`{"ok":true}`))
		case strings.HasSuffix(r.URL.Path, "/wf"):
			_, _ = w.Write([]byte(`{"phase":"Running"}`))
		case strings.HasSuffix(r.URL.Path, "/done"):
			_, _ = w.Write([]byte(`{"phase":"Succeeded"}`))
		default:
			_, _ = w.Write([]byte(`{"entries":[]}`))
		}
	}))
	if err := CmdCatalogGet(context.Background(), c, "yaml", false); err == nil {
		t.Fatal("expected output error")
	}
	if err := CmdCatalogGet(context.Background(), c, "text", true); err == nil {
		t.Fatal("expected raw/text error")
	}
	out := captureStdout(t, func() {
		if err := CmdCatalogGet(context.Background(), c, "json", true); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, `"entries"`) {
		t.Fatalf("catalog = %q", out)
	}

	if err := CmdAbortWorkflowName(c, "done"); err != nil {
		t.Fatal(err)
	}
	if posts.Load() != 0 {
		t.Fatalf("terminal workflow was aborted (%d)", posts.Load())
	}
	if err := CmdAbortWorkflowName(c, "wf"); err != nil {
		t.Fatal(err)
	}
	if posts.Load() != 1 {
		t.Fatalf("posts = %d", posts.Load())
	}
}

func TestCmdArtifactCommands(t *testing.T) {
	if err := CmdWorkflowDownloadArtifact(context.Background(), nil, "wf", "art", true, 0, "", "", "out.bin", false, OutputTextFormat, false); err == nil {
		t.Fatal("expected signed-url conflict")
	}
	if err := CmdWorkflowDownloadArtifact(context.Background(), nil, "wf", "art", false, 0, "", "", "out.bin", true, OutputTextFormat, false); err == nil {
		t.Fatal("expected stdout/-o conflict")
	}

	c, _ := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/artifacts/buckets":
			_, _ = w.Write([]byte(`{"buckets":[{"name":"logs"}]}`))
		default:
			_, _ = w.Write([]byte(`{"objects":[{"name":"a.bin","size":2048,"contentType":"application/octet-stream"}],"nextPageToken":"next"}`))
		}
	}))
	out := captureStdout(t, func() {
		if err := CmdArtifactsListBuckets(context.Background(), c, OutputTextFormat); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "logs") {
		t.Fatalf("buckets = %q", out)
	}
	out = captureStdout(t, func() {
		if err := CmdArtifactsListObjects(context.Background(), c, "logs", "", "", 0, OutputTextFormat); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "a.bin") || !strings.Contains(out, "2.00 KiB") || !strings.Contains(out, "next") {
		t.Fatalf("objects = %q", out)
	}
}

func TestCmdWorkflowListText(t *testing.T) {
	c, _ := testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("limit") != "50" {
			t.Errorf("limit = %q", r.URL.Query().Get("limit"))
		}
		_, _ = w.Write([]byte(`{"items":[{"name":"wf","phase":"Running"}]}`))
	}))
	out := captureStdout(t, func() {
		if err := CmdWorkflowList(context.Background(), c, true, 0, "", "", OutputTextFormat); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "wf") || !strings.Contains(out, "Running") || !strings.Contains(out, "—") {
		t.Fatalf("list = %q", out)
	}
}

func TestDownloadConflictAutoselect(t *testing.T) {
	var sawNode string
	var srv *httptest.Server
	var c *Client
	c, srv = testClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/downloadArtifact/") {
			if r.URL.Query().Get("nodeId") == "" {
				w.WriteHeader(http.StatusConflict)
				_, _ = w.Write([]byte(`{"candidates":[{"nodeId":"n1","displayName":"first"},{"nodeId":"n2","displayName":"last"}]}`))
				return
			}
			sawNode = r.URL.Query().Get("nodeId")
			_, _ = w.Write([]byte(`{"url":"` + srv.URL + `/file","fileName":"report.txt"}`))
			return
		}
		_, _ = w.Write([]byte("payload"))
	}))
	dest := filepath.Join(t.TempDir(), "out.bin")
	err := CmdWorkflowDownloadArtifact(context.Background(), c, "wf", "report", false, 15, "tmpl", "", dest, false, OutputTextFormat, true)
	if err != nil {
		t.Fatal(err)
	}
	if sawNode != "n2" {
		t.Fatalf("node = %q", sawNode)
	}
	b, err := os.ReadFile(dest)
	if err != nil || string(b) != "payload" {
		t.Fatalf("file = %q %v", b, err)
	}
}
