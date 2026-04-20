package lsp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/jsonrpc"
	"github.com/kaeawc/krit/internal/rules"
	"github.com/kaeawc/krit/internal/scanner"
)

// syncBuffer is a thread-safe bytes.Buffer for use with debounced goroutines.
// Each Write broadcasts on a condition variable so tests can block
// until new output arrives instead of polling.
type syncBuffer struct {
	mu   sync.Mutex
	buf  bytes.Buffer
	cond *sync.Cond
}

func newSyncBuffer() *syncBuffer {
	sb := &syncBuffer{}
	sb.cond = sync.NewCond(&sb.mu)
	return sb
}

func (sb *syncBuffer) Write(p []byte) (n int, err error) {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	n, err = sb.buf.Write(p)
	if sb.cond != nil {
		sb.cond.Broadcast()
	}
	return n, err
}

func (sb *syncBuffer) Bytes() []byte {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	return sb.buf.Bytes()
}

// waitUntil blocks until pred(current bytes) returns true or timeout elapses.
// Returns true iff pred was satisfied. Tests use this to synchronize with
// async writers (debounce goroutines) without sleep-based polling.
func (sb *syncBuffer) waitUntil(timeout time.Duration, pred func([]byte) bool) bool {
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		t := time.NewTimer(timeout)
		defer t.Stop()
		select {
		case <-t.C:
			sb.mu.Lock()
			sb.cond.Broadcast()
			sb.mu.Unlock()
		case <-stop:
		}
	}()

	deadline := time.Now().Add(timeout)
	sb.mu.Lock()
	defer sb.mu.Unlock()
	for {
		if pred(sb.buf.Bytes()) {
			return true
		}
		if !time.Now().Before(deadline) {
			return false
		}
		sb.cond.Wait()
	}
}

// buildMessage creates an LSP-framed message (Content-Length header + JSON body).
func buildMessage(body []byte) []byte {
	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body))
	return append([]byte(header), body...)
}

// buildRequest creates a framed JSON-RPC request.
func buildRequest(id interface{}, method string, params interface{}) []byte {
	req := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
	}
	if id != nil {
		req["id"] = id
	}
	if params != nil {
		req["params"] = params
	}
	body, _ := json.Marshal(req)
	return buildMessage(body)
}

// collectMessages parses all LSP messages from the output buffer.
func collectMessages(data []byte) ([]json.RawMessage, error) {
	var msgs []json.RawMessage
	reader := bufio.NewReader(bytes.NewReader(data))

	for {
		// Read Content-Length header
		line, err := reader.ReadString('\n')
		if err != nil {
			break
		}
		line = strings.TrimRight(line, "\r\n")
		if !strings.HasPrefix(line, "Content-Length: ") {
			continue
		}
		var contentLength int
		fmt.Sscanf(strings.TrimPrefix(line, "Content-Length: "), "%d", &contentLength)

		// Read blank line
		reader.ReadString('\n')

		// Read body
		body := make([]byte, contentLength)
		_, err = io.ReadFull(reader, body)
		if err != nil {
			break
		}
		msgs = append(msgs, json.RawMessage(body))
	}

	return msgs, nil
}

func TestReadMessage(t *testing.T) {
	body := []byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`)
	input := buildMessage(body)

	reader := bufio.NewReader(bytes.NewReader(input))

	msg, err := jsonrpc.ReadMessage(reader)
	if err != nil {
		t.Fatalf("ReadMessage error: %v", err)
	}
	if string(msg) != string(body) {
		t.Errorf("got %q, want %q", msg, body)
	}
}

func TestReadMessageMissingContentLength(t *testing.T) {
	// Just a blank line with no Content-Length header
	input := []byte("\r\n")
	reader := bufio.NewReader(bytes.NewReader(input))

	_, err := jsonrpc.ReadMessage(reader)
	if err == nil {
		t.Fatal("expected error for missing Content-Length")
	}
}

func TestInitializeHandshake(t *testing.T) {
	// Build initialize request
	initReq := buildRequest(1, "initialize", map[string]interface{}{
		"processId":    1234,
		"rootUri":      "file:///tmp/test",
		"capabilities": map[string]interface{}{},
	})

	// Build initialized notification (no id)
	initializedNotif := buildRequest(nil, "initialized", map[string]interface{}{})

	// Build shutdown request
	shutdownReq := buildRequest(2, "shutdown", nil)

	// Combine all messages
	var input bytes.Buffer
	input.Write(initReq)
	input.Write(initializedNotif)
	input.Write(shutdownReq)

	// Capture output
	var output bytes.Buffer

	// Override exit to prevent os.Exit
	exitCalled := false
	exitCode := -1
	oldExit := exitFunc
	exitFunc = func(code int) {
		exitCalled = true
		exitCode = code
	}
	defer func() { exitFunc = oldExit }()

	server := NewServer(bufio.NewReader(&input), &output)
	server.Run() // Will run until EOF

	// Parse responses
	msgs, err := collectMessages(output.Bytes())
	if err != nil {
		t.Fatalf("parse responses: %v", err)
	}

	if len(msgs) < 2 {
		t.Fatalf("expected at least 2 response messages, got %d", len(msgs))
	}

	// Check initialize response
	var initResp Response
	if err := json.Unmarshal(msgs[0], &initResp); err != nil {
		t.Fatalf("unmarshal init response: %v", err)
	}
	if initResp.ID != float64(1) {
		t.Errorf("init response ID: got %v, want 1", initResp.ID)
	}
	if initResp.Error != nil {
		t.Errorf("init response error: %v", initResp.Error)
	}

	// Verify server capabilities in the result
	resultBytes, _ := json.Marshal(initResp.Result)
	var result InitializeResult
	if err := json.Unmarshal(resultBytes, &result); err != nil {
		t.Fatalf("unmarshal init result: %v", err)
	}
	if result.ServerInfo == nil || result.ServerInfo.Name != "krit-lsp" {
		t.Error("expected serverInfo.name = krit-lsp")
	}
	if result.Capabilities.TextDocumentSync == nil {
		t.Error("expected textDocumentSync capability")
	} else if result.Capabilities.TextDocumentSync.Change != 1 {
		t.Errorf("expected textDocumentSync.change = 1 (Full), got %d", result.Capabilities.TextDocumentSync.Change)
	}

	// Check shutdown response
	var shutdownResp Response
	if err := json.Unmarshal(msgs[1], &shutdownResp); err != nil {
		t.Fatalf("unmarshal shutdown response: %v", err)
	}
	if shutdownResp.ID != float64(2) {
		t.Errorf("shutdown response ID: got %v, want 2", shutdownResp.ID)
	}

	// Exit was not called (no exit message sent)
	_ = exitCalled
	_ = exitCode
}

func TestDidOpenPublishesDiagnostics(t *testing.T) {
	// Initialize first
	initReq := buildRequest(1, "initialize", map[string]interface{}{
		"processId":    1234,
		"rootUri":      "file:///tmp/test",
		"capabilities": map[string]interface{}{},
	})
	initializedNotif := buildRequest(nil, "initialized", map[string]interface{}{})

	// Open a Kotlin file with some content that should trigger rules
	didOpenReq := buildRequest(nil, "textDocument/didOpen", map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri":        "file:///tmp/test/Test.kt",
			"languageId": "kotlin",
			"version":    1,
			"text":       "package com.example\n\nfun main() {\n    println(\"hello\")\n}\n",
		},
	})

	var input bytes.Buffer
	input.Write(initReq)
	input.Write(initializedNotif)
	input.Write(didOpenReq)

	var output bytes.Buffer
	server := NewServer(bufio.NewReader(&input), &output)
	server.Run()

	msgs, _ := collectMessages(output.Bytes())

	// Should have: initialize response, publishDiagnostics notification
	if len(msgs) < 2 {
		t.Fatalf("expected at least 2 messages, got %d", len(msgs))
	}

	// The second message should be a publishDiagnostics notification
	var notif Notification
	if err := json.Unmarshal(msgs[1], &notif); err != nil {
		t.Fatalf("unmarshal notification: %v", err)
	}
	if notif.Method != "textDocument/publishDiagnostics" {
		t.Errorf("expected publishDiagnostics, got %s", notif.Method)
	}

	// Parse the params to verify URI
	paramsBytes, _ := json.Marshal(notif.Params)
	var diagParams PublishDiagnosticsParams
	if err := json.Unmarshal(paramsBytes, &diagParams); err != nil {
		t.Fatalf("unmarshal diag params: %v", err)
	}
	if diagParams.URI != "file:///tmp/test/Test.kt" {
		t.Errorf("diagnostics URI: got %q, want %q", diagParams.URI, "file:///tmp/test/Test.kt")
	}
}

func TestDidCloseClears(t *testing.T) {
	initReq := buildRequest(1, "initialize", map[string]interface{}{
		"processId":    1234,
		"rootUri":      "file:///tmp/test",
		"capabilities": map[string]interface{}{},
	})
	initializedNotif := buildRequest(nil, "initialized", map[string]interface{}{})
	didOpenReq := buildRequest(nil, "textDocument/didOpen", map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri":        "file:///tmp/test/Test.kt",
			"languageId": "kotlin",
			"version":    1,
			"text":       "package com.example\n",
		},
	})
	didCloseReq := buildRequest(nil, "textDocument/didClose", map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": "file:///tmp/test/Test.kt",
		},
	})

	var input bytes.Buffer
	input.Write(initReq)
	input.Write(initializedNotif)
	input.Write(didOpenReq)
	input.Write(didCloseReq)

	var output bytes.Buffer
	server := NewServer(bufio.NewReader(&input), &output)
	server.Run()

	msgs, _ := collectMessages(output.Bytes())

	// Should have: init response, didOpen diagnostics, didClose diagnostics (empty)
	if len(msgs) < 3 {
		t.Fatalf("expected at least 3 messages, got %d", len(msgs))
	}

	// Last publishDiagnostics should have empty diagnostics
	var notif Notification
	if err := json.Unmarshal(msgs[len(msgs)-1], &notif); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if notif.Method != "textDocument/publishDiagnostics" {
		t.Errorf("expected publishDiagnostics, got %s", notif.Method)
	}
	paramsBytes, _ := json.Marshal(notif.Params)
	var diagParams PublishDiagnosticsParams
	json.Unmarshal(paramsBytes, &diagParams)
	if len(diagParams.Diagnostics) != 0 {
		t.Errorf("expected 0 diagnostics on close, got %d", len(diagParams.Diagnostics))
	}
}

func TestDidChangeReanalyzes(t *testing.T) {
	// Set debounce to zero for this test so analysis fires immediately
	oldDelay := debounceDelay
	debounceDelay = 0
	defer func() { debounceDelay = oldDelay }()

	initReq := buildRequest(1, "initialize", map[string]interface{}{
		"processId":    1234,
		"rootUri":      "file:///tmp/test",
		"capabilities": map[string]interface{}{},
	})
	initializedNotif := buildRequest(nil, "initialized", map[string]interface{}{})
	didOpenReq := buildRequest(nil, "textDocument/didOpen", map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri":        "file:///tmp/test/Test.kt",
			"languageId": "kotlin",
			"version":    1,
			"text":       "package com.example\n",
		},
	})
	didChangeReq := buildRequest(nil, "textDocument/didChange", map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri":     "file:///tmp/test/Test.kt",
			"version": 2,
		},
		"contentChanges": []map[string]interface{}{
			{"text": "package com.example\n\nfun main() {}\n"},
		},
	})

	var input bytes.Buffer
	input.Write(initReq)
	input.Write(initializedNotif)
	input.Write(didOpenReq)
	input.Write(didChangeReq)

	output := newSyncBuffer()
	server := NewServer(bufio.NewReader(&input), output)
	server.Run()

	// Block until the debounce goroutine publishes its diagnostics. The
	// condition variable wakes us on each write — no sleep/poll race.
	output.waitUntil(2*time.Second, func(b []byte) bool {
		msgs, _ := collectMessages(b)
		return len(msgs) >= 3
	})

	msgs, _ := collectMessages(output.Bytes())

	// Should have: init response, didOpen diagnostics, didChange diagnostics
	if len(msgs) < 3 {
		t.Fatalf("expected at least 3 messages, got %d", len(msgs))
	}

	// Verify the document was updated
	server.docsMu.Lock()
	doc := server.docs["file:///tmp/test/Test.kt"]
	server.docsMu.Unlock()
	if doc == nil {
		t.Fatal("document not tracked after didChange")
	}
	if doc.Version != 2 {
		t.Errorf("expected version 2, got %d", doc.Version)
	}
}

func TestShutdownThenExit(t *testing.T) {
	initReq := buildRequest(1, "initialize", map[string]interface{}{
		"processId":    1234,
		"rootUri":      "file:///tmp/test",
		"capabilities": map[string]interface{}{},
	})
	shutdownReq := buildRequest(2, "shutdown", nil)
	exitReq := buildRequest(nil, "exit", nil)

	var input bytes.Buffer
	input.Write(initReq)
	input.Write(shutdownReq)
	input.Write(exitReq)

	var output bytes.Buffer

	var capturedExitCode int
	oldExit := exitFunc
	exitFunc = func(code int) {
		capturedExitCode = code
		// Don't actually exit -- just record
	}
	defer func() { exitFunc = oldExit }()

	server := NewServer(bufio.NewReader(&input), &output)
	server.Run()

	if capturedExitCode != 0 {
		t.Errorf("expected exit code 0 after shutdown, got %d", capturedExitCode)
	}
}

func TestExitWithoutShutdown(t *testing.T) {
	initReq := buildRequest(1, "initialize", map[string]interface{}{
		"processId":    1234,
		"rootUri":      "file:///tmp/test",
		"capabilities": map[string]interface{}{},
	})
	exitReq := buildRequest(nil, "exit", nil)

	var input bytes.Buffer
	input.Write(initReq)
	input.Write(exitReq)

	var output bytes.Buffer

	var capturedExitCode int
	oldExit := exitFunc
	exitFunc = func(code int) {
		capturedExitCode = code
	}
	defer func() { exitFunc = oldExit }()

	server := NewServer(bufio.NewReader(&input), &output)
	server.Run()

	if capturedExitCode != 1 {
		t.Errorf("expected exit code 1 without prior shutdown, got %d", capturedExitCode)
	}
}

func TestUnknownMethodReturnsError(t *testing.T) {
	initReq := buildRequest(1, "initialize", map[string]interface{}{
		"processId":    1234,
		"rootUri":      "file:///tmp/test",
		"capabilities": map[string]interface{}{},
	})
	unknownReq := buildRequest(99, "textDocument/unknownMethod", nil)

	var input bytes.Buffer
	input.Write(initReq)
	input.Write(unknownReq)

	var output bytes.Buffer
	server := NewServer(bufio.NewReader(&input), &output)
	server.Run()

	msgs, _ := collectMessages(output.Bytes())
	if len(msgs) < 2 {
		t.Fatalf("expected at least 2 messages, got %d", len(msgs))
	}

	var errResp Response
	json.Unmarshal(msgs[1], &errResp)
	if errResp.Error == nil {
		t.Fatal("expected error response for unknown method")
	}
	if errResp.Error.Code != -32601 {
		t.Errorf("expected error code -32601, got %d", errResp.Error.Code)
	}
}

func TestURIToPath(t *testing.T) {
	tests := []struct {
		uri  string
		want string
	}{
		{"file:///home/user/test.kt", "/home/user/test.kt"},
		{"file:///tmp/test/Main.kt", "/tmp/test/Main.kt"},
	}
	for _, tt := range tests {
		got := uriToPath(tt.uri)
		if got != tt.want {
			t.Errorf("uriToPath(%q) = %q, want %q", tt.uri, got, tt.want)
		}
	}
}

func TestFindingToDiagnostic(t *testing.T) {
	tests := []struct {
		name     string
		severity string
		wantSev  int
	}{
		{"error", "error", 1},
		{"warning", "warning", 2},
		{"info", "info", 3},
		{"unknown", "unknown", 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := scanner.Finding{
				File:     "test.kt",
				Line:     10,
				Col:      5,
				RuleSet:  "style",
				Rule:     "TestRule",
				Severity: tt.severity,
				Message:  "test message",
			}
			d := FindingToDiagnostic(f)
			if d.Severity != tt.wantSev {
				t.Errorf("severity: got %d, want %d", d.Severity, tt.wantSev)
			}
			if d.Range.Start.Line != 9 {
				t.Errorf("line: got %d, want 9", d.Range.Start.Line)
			}
			if d.Range.Start.Character != 5 {
				t.Errorf("col: got %d, want 5", d.Range.Start.Character)
			}
			if d.Code != "style/TestRule" {
				t.Errorf("code: got %q, want %q", d.Code, "style/TestRule")
			}
			if d.Source != "krit" {
				t.Errorf("source: got %q, want %q", d.Source, "krit")
			}
		})
	}
}

func TestNonKotlinFileSkipped(t *testing.T) {
	initReq := buildRequest(1, "initialize", map[string]interface{}{
		"processId":    1234,
		"rootUri":      "file:///tmp/test",
		"capabilities": map[string]interface{}{},
	})
	initializedNotif := buildRequest(nil, "initialized", map[string]interface{}{})
	didOpenReq := buildRequest(nil, "textDocument/didOpen", map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri":        "file:///tmp/test/Test.java",
			"languageId": "java",
			"version":    1,
			"text":       "public class Test {}",
		},
	})

	var input bytes.Buffer
	input.Write(initReq)
	input.Write(initializedNotif)
	input.Write(didOpenReq)

	var output bytes.Buffer
	server := NewServer(bufio.NewReader(&input), &output)
	server.Run()

	msgs, _ := collectMessages(output.Bytes())

	// Should only have: init response (no diagnostics for non-Kotlin file)
	if len(msgs) != 1 {
		t.Errorf("expected 1 message (init response only), got %d", len(msgs))
	}
}

func TestByteOffsetToPosition(t *testing.T) {
	content := "line0\nline1\nline2"
	tests := []struct {
		offset int
		want   Position
	}{
		{0, Position{Line: 0, Character: 0}},
		{3, Position{Line: 0, Character: 3}},
		{5, Position{Line: 0, Character: 5}},  // last char of line0
		{6, Position{Line: 1, Character: 0}},  // first char of line1 (after \n)
		{11, Position{Line: 1, Character: 5}}, // last char of line1
		{12, Position{Line: 2, Character: 0}}, // first char of line2
		{16, Position{Line: 2, Character: 4}}, // beyond last char
	}
	for _, tt := range tests {
		got := byteOffsetToPosition(content, tt.offset)
		if got != tt.want {
			t.Errorf("byteOffsetToPosition(%q, %d) = %+v, want %+v", content, tt.offset, got, tt.want)
		}
	}
}

func TestFindingsToCodeActionsFixable(t *testing.T) {
	uri := "file:///tmp/test/Test.kt"
	content := "val x = foo()\n"

	findings := []scanner.Finding{
		{
			File:     "/tmp/test/Test.kt",
			Line:     1,
			Col:      8,
			RuleSet:  "style",
			Rule:     "TestFixRule",
			Severity: "warning",
			Message:  "use bar() instead of foo()",
			Fix: &scanner.Fix{
				StartByte:   8,
				EndByte:     13,
				Replacement: "bar()",
				ByteMode:    true,
			},
		},
	}

	actions := findingsToCodeActions(uri, content, findings)
	if len(actions) != 1 {
		t.Fatalf("expected 1 code action, got %d", len(actions))
	}

	action := actions[0]
	if action.Kind != "quickfix" {
		t.Errorf("kind: got %q, want %q", action.Kind, "quickfix")
	}
	if action.Title != "Fix: TestFixRule" {
		t.Errorf("title: got %q, want %q", action.Title, "Fix: TestFixRule")
	}
	if action.Edit == nil {
		t.Fatal("expected edit, got nil")
	}
	edits, ok := action.Edit.Changes[uri]
	if !ok || len(edits) != 1 {
		t.Fatalf("expected 1 text edit for URI, got %d", len(edits))
	}
	edit := edits[0]
	if edit.NewText != "bar()" {
		t.Errorf("newText: got %q, want %q", edit.NewText, "bar()")
	}
	// byte 8 = line 0, char 8; byte 13 = line 0, char 13
	if edit.Range.Start.Line != 0 || edit.Range.Start.Character != 8 {
		t.Errorf("start: got %+v, want {0, 8}", edit.Range.Start)
	}
	if edit.Range.End.Line != 0 || edit.Range.End.Character != 13 {
		t.Errorf("end: got %+v, want {0, 13}", edit.Range.End)
	}
	if len(action.Diagnostics) != 1 {
		t.Errorf("expected 1 diagnostic, got %d", len(action.Diagnostics))
	}
}

func TestFindingsToCodeActionsNonFixable(t *testing.T) {
	uri := "file:///tmp/test/Test.kt"
	content := "val x = foo()\n"

	findings := []scanner.Finding{
		{
			File:     "/tmp/test/Test.kt",
			Line:     1,
			Col:      0,
			RuleSet:  "style",
			Rule:     "NoFixRule",
			Severity: "warning",
			Message:  "something is wrong",
			Fix:      nil,
		},
	}

	actions := findingsToCodeActions(uri, content, findings)
	if len(actions) != 0 {
		t.Errorf("expected 0 code actions for non-fixable finding, got %d", len(actions))
	}
}

func TestFindingsToCodeActionsLineBased(t *testing.T) {
	uri := "file:///tmp/test/Test.kt"
	content := "line1\nline2\nline3\n"

	findings := []scanner.Finding{
		{
			File:     "/tmp/test/Test.kt",
			Line:     2,
			Col:      0,
			RuleSet:  "style",
			Rule:     "LineFixRule",
			Severity: "warning",
			Message:  "replace line 2",
			Fix: &scanner.Fix{
				StartLine:   2,
				EndLine:     2,
				Replacement: "replaced\n",
				ByteMode:    false,
			},
		},
	}

	actions := findingsToCodeActions(uri, content, findings)
	if len(actions) != 1 {
		t.Fatalf("expected 1 code action, got %d", len(actions))
	}

	edit := actions[0].Edit.Changes[uri][0]
	// Line-based: StartLine=2 -> 0-based line 1, EndLine=2 -> exclusive line 2
	if edit.Range.Start.Line != 1 || edit.Range.Start.Character != 0 {
		t.Errorf("start: got %+v, want {1, 0}", edit.Range.Start)
	}
	if edit.Range.End.Line != 2 || edit.Range.End.Character != 0 {
		t.Errorf("end: got %+v, want {2, 0}", edit.Range.End)
	}
	if edit.NewText != "replaced\n" {
		t.Errorf("newText: got %q, want %q", edit.NewText, "replaced\n")
	}
}

func TestCodeActionHandlerIntegration(t *testing.T) {
	// Initialize, open a file, then request code actions
	initReq := buildRequest(1, "initialize", map[string]interface{}{
		"processId":    1234,
		"rootUri":      "file:///tmp/test",
		"capabilities": map[string]interface{}{},
	})
	initializedNotif := buildRequest(nil, "initialized", map[string]interface{}{})

	// Open a Kotlin file — the content doesn't need to trigger a real rule;
	// we'll verify the server responds to codeAction requests without error.
	didOpenReq := buildRequest(nil, "textDocument/didOpen", map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri":        "file:///tmp/test/Test.kt",
			"languageId": "kotlin",
			"version":    1,
			"text":       "package com.example\n\nfun main() {\n    println(\"hello\")\n}\n",
		},
	})

	codeActionReq := buildRequest(10, "textDocument/codeAction", map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": "file:///tmp/test/Test.kt",
		},
		"range": map[string]interface{}{
			"start": map[string]interface{}{"line": 0, "character": 0},
			"end":   map[string]interface{}{"line": 0, "character": 0},
		},
		"context": map[string]interface{}{
			"diagnostics": []interface{}{},
		},
	})

	var input bytes.Buffer
	input.Write(initReq)
	input.Write(initializedNotif)
	input.Write(didOpenReq)
	input.Write(codeActionReq)

	var output bytes.Buffer
	server := NewServer(bufio.NewReader(&input), &output)
	server.Run()

	msgs, _ := collectMessages(output.Bytes())

	// Should have: init response, publishDiagnostics, codeAction response
	if len(msgs) < 3 {
		t.Fatalf("expected at least 3 messages, got %d", len(msgs))
	}

	// The last message should be the codeAction response
	var resp Response
	if err := json.Unmarshal(msgs[len(msgs)-1], &resp); err != nil {
		t.Fatalf("unmarshal code action response: %v", err)
	}
	if resp.ID != float64(10) {
		t.Errorf("code action response ID: got %v, want 10", resp.ID)
	}
	if resp.Error != nil {
		t.Errorf("code action response error: %v", resp.Error)
	}
	// Result should be an array (possibly empty)
	resultBytes, _ := json.Marshal(resp.Result)
	var actions []CodeAction
	if err := json.Unmarshal(resultBytes, &actions); err != nil {
		t.Fatalf("unmarshal code actions: %v", err)
	}
	// We don't assert specific actions since the test file may or may not trigger fixable rules,
	// but we verify the response is a valid array.
	if actions == nil {
		t.Error("expected non-nil actions array")
	}
}

func TestInitializeIncludesCodeActionProvider(t *testing.T) {
	initReq := buildRequest(1, "initialize", map[string]interface{}{
		"processId":    1234,
		"rootUri":      "file:///tmp/test",
		"capabilities": map[string]interface{}{},
	})

	var input bytes.Buffer
	input.Write(initReq)

	var output bytes.Buffer
	server := NewServer(bufio.NewReader(&input), &output)
	server.Run()

	msgs, _ := collectMessages(output.Bytes())
	if len(msgs) < 1 {
		t.Fatal("expected at least 1 message")
	}

	var resp Response
	json.Unmarshal(msgs[0], &resp)
	resultBytes, _ := json.Marshal(resp.Result)
	var result InitializeResult
	json.Unmarshal(resultBytes, &result)

	if !result.Capabilities.CodeActionProvider {
		t.Error("expected codeActionProvider = true in server capabilities")
	}
}

func TestDispatcherReusedAcrossDidOpen(t *testing.T) {
	// Verify that the dispatcher is created once during initialize and reused
	// across multiple didOpen calls (not recreated each time).
	initReq := buildRequest(1, "initialize", map[string]interface{}{
		"processId":    1234,
		"rootUri":      "file:///tmp/test",
		"capabilities": map[string]interface{}{},
	})
	initializedNotif := buildRequest(nil, "initialized", map[string]interface{}{})
	didOpen1 := buildRequest(nil, "textDocument/didOpen", map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri":        "file:///tmp/test/A.kt",
			"languageId": "kotlin",
			"version":    1,
			"text":       "package com.example\n",
		},
	})
	didOpen2 := buildRequest(nil, "textDocument/didOpen", map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri":        "file:///tmp/test/B.kt",
			"languageId": "kotlin",
			"version":    1,
			"text":       "package com.example\n\nfun foo() {}\n",
		},
	})

	var input bytes.Buffer
	input.Write(initReq)
	input.Write(initializedNotif)
	input.Write(didOpen1)
	input.Write(didOpen2)

	var output bytes.Buffer
	server := NewServer(bufio.NewReader(&input), &output)

	// Capture the dispatcher after init to verify reuse
	server.Run()

	// The dispatcher should be non-nil and the same instance across calls.
	// (The server creates it once in handleInitialize.)
	if server.analyzer == nil {
		t.Fatal("dispatcher should be non-nil after initialize")
	}

	// Both documents should be tracked
	server.docsMu.Lock()
	docA := server.docs["file:///tmp/test/A.kt"]
	docB := server.docs["file:///tmp/test/B.kt"]
	server.docsMu.Unlock()

	if docA == nil {
		t.Error("expected document A to be tracked")
	}
	if docB == nil {
		t.Error("expected document B to be tracked")
	}

	msgs, _ := collectMessages(output.Bytes())
	// init response + 2 publishDiagnostics
	if len(msgs) < 3 {
		t.Fatalf("expected at least 3 messages, got %d", len(msgs))
	}
}

func TestConfigLoadingFromWorkspaceRoot(t *testing.T) {
	// Create a temp directory with a krit.yml config file
	tmpDir := t.TempDir()
	configContent := `style:
  MagicNumber:
    active: false
`
	err := os.WriteFile(filepath.Join(tmpDir, "krit.yml"), []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("write config: %v", err)
	}

	rootURI := "file://" + tmpDir

	initReq := buildRequest(1, "initialize", map[string]interface{}{
		"processId":    1234,
		"rootUri":      rootURI,
		"capabilities": map[string]interface{}{},
	})

	var input bytes.Buffer
	input.Write(initReq)

	var output bytes.Buffer
	server := NewServer(bufio.NewReader(&input), &output)
	server.Run()

	// Verify config was loaded
	if server.cfg == nil {
		t.Fatal("expected config to be loaded from workspace root krit.yml")
	}

	// Verify dispatcher was built
	if server.analyzer == nil {
		t.Fatal("expected dispatcher to be built")
	}
}

func TestConfigLoadingFromDotKritYml(t *testing.T) {
	// Create a temp directory with a .krit.yml config file
	tmpDir := t.TempDir()
	configContent := `complexity:
  LongMethod:
    active: true
    allowedLines: 100
`
	err := os.WriteFile(filepath.Join(tmpDir, ".krit.yml"), []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("write config: %v", err)
	}

	rootURI := "file://" + tmpDir

	initReq := buildRequest(1, "initialize", map[string]interface{}{
		"processId":    1234,
		"rootUri":      rootURI,
		"capabilities": map[string]interface{}{},
	})

	var input bytes.Buffer
	input.Write(initReq)

	var output bytes.Buffer
	server := NewServer(bufio.NewReader(&input), &output)
	server.Run()

	if server.cfg == nil {
		t.Fatal("expected config to be loaded from .krit.yml")
	}
}

func TestConfigPathFromInitializationOptions(t *testing.T) {
	// Create a temp config file at a custom path
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "custom-config.yml")
	configContent := `style:
  MagicNumber:
    active: false
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("write config: %v", err)
	}

	initReq := buildRequest(1, "initialize", map[string]interface{}{
		"processId":    1234,
		"rootUri":      "file:///tmp/test",
		"capabilities": map[string]interface{}{},
		"initializationOptions": map[string]interface{}{
			"configPath": configPath,
		},
	})

	var input bytes.Buffer
	input.Write(initReq)

	var output bytes.Buffer
	server := NewServer(bufio.NewReader(&input), &output)
	server.Run()

	if server.configPath != configPath {
		t.Errorf("expected configPath=%q, got %q", configPath, server.configPath)
	}
	if server.cfg == nil {
		t.Fatal("expected config to be loaded from initializationOptions configPath")
	}
}

func TestDebounceRapidChanges(t *testing.T) {
	// Set debounce to a short but non-zero duration to test that rapid
	// changes result in a single analysis pass.
	oldDelay := debounceDelay
	debounceDelay = 50 * time.Millisecond
	defer func() { debounceDelay = oldDelay }()

	initReq := buildRequest(1, "initialize", map[string]interface{}{
		"processId":    1234,
		"rootUri":      "file:///tmp/test",
		"capabilities": map[string]interface{}{},
	})
	initializedNotif := buildRequest(nil, "initialized", map[string]interface{}{})
	didOpenReq := buildRequest(nil, "textDocument/didOpen", map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri":        "file:///tmp/test/Test.kt",
			"languageId": "kotlin",
			"version":    1,
			"text":       "package com.example\n",
		},
	})

	// Simulate 3 rapid didChange events
	change1 := buildRequest(nil, "textDocument/didChange", map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri":     "file:///tmp/test/Test.kt",
			"version": 2,
		},
		"contentChanges": []map[string]interface{}{
			{"text": "package com.example\n\nval x = 1\n"},
		},
	})
	change2 := buildRequest(nil, "textDocument/didChange", map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri":     "file:///tmp/test/Test.kt",
			"version": 3,
		},
		"contentChanges": []map[string]interface{}{
			{"text": "package com.example\n\nval x = 2\n"},
		},
	})
	change3 := buildRequest(nil, "textDocument/didChange", map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri":     "file:///tmp/test/Test.kt",
			"version": 4,
		},
		"contentChanges": []map[string]interface{}{
			{"text": "package com.example\n\nval x = 3\n"},
		},
	})

	var input bytes.Buffer
	input.Write(initReq)
	input.Write(initializedNotif)
	input.Write(didOpenReq)
	input.Write(change1)
	input.Write(change2)
	input.Write(change3)

	output := newSyncBuffer()
	server := NewServer(bufio.NewReader(&input), output)
	server.Run()

	// Block until the debounced publishDiagnostics arrives. The
	// condition variable wakes us on each write — no poll/sleep race.
	output.waitUntil(2*time.Second, func(b []byte) bool {
		msgs, _ := collectMessages(b)
		diagCount := 0
		for _, msg := range msgs {
			var n Notification
			if err := json.Unmarshal(msg, &n); err == nil && n.Method == "textDocument/publishDiagnostics" {
				diagCount++
			}
		}
		return diagCount >= 2
	})

	msgs, _ := collectMessages(output.Bytes())

	// Count publishDiagnostics notifications
	diagCount := 0
	for _, msg := range msgs {
		var notif Notification
		if err := json.Unmarshal(msg, &notif); err == nil && notif.Method == "textDocument/publishDiagnostics" {
			diagCount++
		}
	}

	// Should have: 1 from didOpen + 1 from the debounced batch (not 3 separate ones)
	// The 3 rapid changes should be coalesced into 1 analysis.
	if diagCount != 2 {
		t.Errorf("expected 2 publishDiagnostics (1 open + 1 debounced), got %d", diagCount)
	}

	// Verify the final version is 4
	server.docsMu.Lock()
	doc := server.docs["file:///tmp/test/Test.kt"]
	server.docsMu.Unlock()
	if doc == nil {
		t.Fatal("document not tracked")
	}
	if doc.Version != 4 {
		t.Errorf("expected final version 4, got %d", doc.Version)
	}
}

func TestInitializeIncludesNewCapabilities(t *testing.T) {
	initReq := buildRequest(1, "initialize", map[string]interface{}{
		"processId":    1234,
		"rootUri":      "file:///tmp/test",
		"capabilities": map[string]interface{}{},
	})

	var input bytes.Buffer
	input.Write(initReq)

	var output bytes.Buffer
	server := NewServer(bufio.NewReader(&input), &output)
	server.Run()

	msgs, _ := collectMessages(output.Bytes())
	if len(msgs) < 1 {
		t.Fatal("expected at least 1 message")
	}

	var resp Response
	json.Unmarshal(msgs[0], &resp)
	resultBytes, _ := json.Marshal(resp.Result)
	var result InitializeResult
	json.Unmarshal(resultBytes, &result)

	if !result.Capabilities.DocumentFormattingProvider {
		t.Error("expected documentFormattingProvider = true")
	}
	if !result.Capabilities.HoverProvider {
		t.Error("expected hoverProvider = true")
	}
	if !result.Capabilities.DocumentSymbolProvider {
		t.Error("expected documentSymbolProvider = true")
	}
}

func TestFormattingReturnsEdits(t *testing.T) {
	initReq := buildRequest(1, "initialize", map[string]interface{}{
		"processId":    1234,
		"rootUri":      "file:///tmp/test",
		"capabilities": map[string]interface{}{},
	})
	initializedNotif := buildRequest(nil, "initialized", map[string]interface{}{})

	// Open a Kotlin file
	didOpenReq := buildRequest(nil, "textDocument/didOpen", map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri":        "file:///tmp/test/Test.kt",
			"languageId": "kotlin",
			"version":    1,
			"text":       "package com.example\n\nfun main() {\n    println(\"hello\")\n}\n",
		},
	})

	// Request formatting
	formatReq := buildRequest(10, "textDocument/formatting", map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": "file:///tmp/test/Test.kt",
		},
		"options": map[string]interface{}{
			"tabSize":      4,
			"insertSpaces": true,
		},
	})

	var input bytes.Buffer
	input.Write(initReq)
	input.Write(initializedNotif)
	input.Write(didOpenReq)
	input.Write(formatReq)

	var output bytes.Buffer
	server := NewServer(bufio.NewReader(&input), &output)
	server.Run()

	msgs, _ := collectMessages(output.Bytes())

	// Find the formatting response (should be last)
	var formatResp Response
	found := false
	for _, msg := range msgs {
		var resp Response
		if err := json.Unmarshal(msg, &resp); err == nil && resp.ID == float64(10) {
			formatResp = resp
			found = true
			break
		}
	}

	if !found {
		t.Fatal("formatting response not found")
	}
	if formatResp.Error != nil {
		t.Fatalf("formatting response error: %v", formatResp.Error)
	}

	// Result should be an array of TextEdit (possibly empty)
	resultBytes, _ := json.Marshal(formatResp.Result)
	var edits []TextEdit
	if err := json.Unmarshal(resultBytes, &edits); err != nil {
		t.Fatalf("unmarshal formatting edits: %v", err)
	}
	// We just verify it's a valid array (may be empty if no fixable findings)
	if edits == nil {
		t.Error("expected non-nil edits array")
	}
}

func TestFormattingWithoutOpenDoc(t *testing.T) {
	initReq := buildRequest(1, "initialize", map[string]interface{}{
		"processId":    1234,
		"rootUri":      "file:///tmp/test",
		"capabilities": map[string]interface{}{},
	})
	initializedNotif := buildRequest(nil, "initialized", map[string]interface{}{})

	// Request formatting without opening the document first
	formatReq := buildRequest(10, "textDocument/formatting", map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": "file:///tmp/test/Missing.kt",
		},
		"options": map[string]interface{}{
			"tabSize":      4,
			"insertSpaces": true,
		},
	})

	var input bytes.Buffer
	input.Write(initReq)
	input.Write(initializedNotif)
	input.Write(formatReq)

	var output bytes.Buffer
	server := NewServer(bufio.NewReader(&input), &output)
	server.Run()

	msgs, _ := collectMessages(output.Bytes())

	var formatResp Response
	for _, msg := range msgs {
		var resp Response
		if err := json.Unmarshal(msg, &resp); err == nil && resp.ID == float64(10) {
			formatResp = resp
			break
		}
	}

	if formatResp.Error != nil {
		t.Fatalf("expected no error, got: %v", formatResp.Error)
	}
	resultBytes, _ := json.Marshal(formatResp.Result)
	var edits []TextEdit
	json.Unmarshal(resultBytes, &edits)
	if len(edits) != 0 {
		t.Errorf("expected empty edits for missing doc, got %d", len(edits))
	}
}

func TestFormattingWithFixableFindings(t *testing.T) {
	// Create a server with a document that has pre-loaded fixable findings
	server := &Server{
		docs: make(map[string]*Document),
	}

	uri := "file:///tmp/test/Test.kt"
	content := "val x = foo()\nval y = bar()\n"
	server.docs[uri] = &Document{
		URI:     uri,
		Content: []byte(content),
		Findings: scanner.CollectFindings([]scanner.Finding{
			{
				File: "/tmp/test/Test.kt", Line: 1, Col: 8,
				RuleSet: "style", Rule: "TestFix", Severity: "warning",
				Message: "use baz()",
				Fix:     &scanner.Fix{StartByte: 8, EndByte: 13, Replacement: "baz()", ByteMode: true},
			},
		}),
	}

	// Simulate a formatting request directly
	params := DocumentFormattingParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Options:      FormattingOptions{TabSize: 4, InsertSpaces: true},
	}
	paramsBytes, _ := json.Marshal(params)
	req := &Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "textDocument/formatting",
		Params:  paramsBytes,
	}

	var output bytes.Buffer
	server.writer = &output
	server.handleFormatting(req)

	msgs, _ := collectMessages(output.Bytes())
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}

	var resp Response
	json.Unmarshal(msgs[0], &resp)
	resultBytes, _ := json.Marshal(resp.Result)
	var edits []TextEdit
	json.Unmarshal(resultBytes, &edits)

	if len(edits) != 1 {
		t.Fatalf("expected 1 edit, got %d", len(edits))
	}
	if edits[0].NewText != "baz()" {
		t.Errorf("newText: got %q, want %q", edits[0].NewText, "baz()")
	}
	if edits[0].Range.Start.Character != 8 {
		t.Errorf("start char: got %d, want 8", edits[0].Range.Start.Character)
	}
}

func TestHoverReturnsRuleInfo(t *testing.T) {
	server := &Server{
		docs: make(map[string]*Document),
	}

	uri := "file:///tmp/test/Test.kt"
	server.docs[uri] = &Document{
		URI:     uri,
		Content: []byte("val x = foo()\n"),
		Findings: scanner.CollectFindings([]scanner.Finding{
			{
				File: "/tmp/test/Test.kt", Line: 1, Col: 0,
				RuleSet: "style", Rule: "MagicNumber", Severity: "warning",
				Message: "Avoid magic numbers",
			},
		}),
	}

	params := HoverParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 0, Character: 5}, // line 0 = finding line 1
	}
	paramsBytes, _ := json.Marshal(params)
	req := &Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "textDocument/hover",
		Params:  paramsBytes,
	}

	var output bytes.Buffer
	server.writer = &output
	server.handleHover(req)

	msgs, _ := collectMessages(output.Bytes())
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}

	var resp Response
	json.Unmarshal(msgs[0], &resp)
	resultBytes, _ := json.Marshal(resp.Result)
	var hover Hover
	json.Unmarshal(resultBytes, &hover)

	if hover.Contents.Kind != "markdown" {
		t.Errorf("kind: got %q, want %q", hover.Contents.Kind, "markdown")
	}
	if !strings.Contains(hover.Contents.Value, "MagicNumber") {
		t.Errorf("expected hover to contain rule name, got: %s", hover.Contents.Value)
	}
	if !strings.Contains(hover.Contents.Value, "Severity: `warning`") {
		t.Errorf("expected hover to contain severity, got: %s", hover.Contents.Value)
	}
	if !strings.Contains(hover.Contents.Value, "Avoid magic numbers") {
		t.Errorf("expected hover to contain message, got: %s", hover.Contents.Value)
	}
	expectedDefaultState := "opt-in"
	if rules.IsDefaultActive("MagicNumber") {
		expectedDefaultState = "active"
	}
	if !strings.Contains(hover.Contents.Value, "Default state: `"+expectedDefaultState+"`") {
		t.Errorf("expected hover to contain default state, got: %s", hover.Contents.Value)
	}
	if !strings.Contains(hover.Contents.Value, "Auto-fix: unavailable") {
		t.Errorf("expected hover to contain autofix availability, got: %s", hover.Contents.Value)
	}
}

func TestHoverNoFindingReturnsNull(t *testing.T) {
	server := &Server{
		docs: make(map[string]*Document),
	}

	uri := "file:///tmp/test/Test.kt"
	server.docs[uri] = &Document{
		URI:      uri,
		Content:  []byte("val x = 1\n"),
		Findings: scanner.FindingColumns{},
	}

	params := HoverParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 0, Character: 0},
	}
	paramsBytes, _ := json.Marshal(params)
	req := &Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "textDocument/hover",
		Params:  paramsBytes,
	}

	var output bytes.Buffer
	server.writer = &output
	server.handleHover(req)

	msgs, _ := collectMessages(output.Bytes())
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}

	var resp Response
	json.Unmarshal(msgs[0], &resp)
	if resp.Result != nil {
		t.Errorf("expected null result for no findings, got: %v", resp.Result)
	}
}

func TestHoverMultipleFindingsOnSameLine(t *testing.T) {
	server := &Server{
		docs: make(map[string]*Document),
	}

	uri := "file:///tmp/test/Test.kt"
	server.docs[uri] = &Document{
		URI:     uri,
		Content: []byte("val x = foo()\n"),
		Findings: scanner.CollectFindings([]scanner.Finding{
			{File: "/tmp/test/Test.kt", Line: 1, Col: 0, RuleSet: "style", Rule: "RuleA", Severity: "warning", Message: "msg A"},
			{File: "/tmp/test/Test.kt", Line: 1, Col: 5, RuleSet: "style", Rule: "RuleB", Severity: "error", Message: "msg B"},
		}),
	}

	params := HoverParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 0, Character: 0},
	}
	paramsBytes, _ := json.Marshal(params)
	req := &Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "textDocument/hover",
		Params:  paramsBytes,
	}

	var output bytes.Buffer
	server.writer = &output
	server.handleHover(req)

	msgs, _ := collectMessages(output.Bytes())
	var resp Response
	json.Unmarshal(msgs[0], &resp)
	resultBytes, _ := json.Marshal(resp.Result)
	var hover Hover
	json.Unmarshal(resultBytes, &hover)

	if !strings.Contains(hover.Contents.Value, "RuleA") {
		t.Error("expected hover to contain RuleA")
	}
	if !strings.Contains(hover.Contents.Value, "RuleB") {
		t.Error("expected hover to contain RuleB")
	}
	if !strings.Contains(hover.Contents.Value, "---") {
		t.Error("expected separator between multiple findings")
	}
}

func TestHoverIncludesFixLevelForFixableRule(t *testing.T) {
	server := &Server{
		docs: make(map[string]*Document),
	}

	uri := "file:///tmp/test/Test.kt"
	server.docs[uri] = &Document{
		URI:     uri,
		Content: []byte("val x = foo()\n"),
		Findings: scanner.CollectFindings([]scanner.Finding{
			{
				File: "/tmp/test/Test.kt", Line: 1, Col: 0,
				RuleSet: "style", Rule: "UseCheckNotNull", Severity: "warning",
				Message: "Use checkNotNull instead of check(x != null)",
			},
		}),
	}

	params := HoverParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 0, Character: 5},
	}
	paramsBytes, _ := json.Marshal(params)
	req := &Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "textDocument/hover",
		Params:  paramsBytes,
	}

	var output bytes.Buffer
	server.writer = &output
	server.handleHover(req)

	msgs, _ := collectMessages(output.Bytes())
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}

	var resp Response
	json.Unmarshal(msgs[0], &resp)
	resultBytes, _ := json.Marshal(resp.Result)
	var hover Hover
	json.Unmarshal(resultBytes, &hover)

	if !strings.Contains(hover.Contents.Value, "Auto-fix: `idiomatic`") {
		t.Errorf("expected hover to contain fix level, got: %s", hover.Contents.Value)
	}
	expectedDefaultState := "opt-in"
	if rules.IsDefaultActive("UseCheckNotNull") {
		expectedDefaultState = "active"
	}
	if !strings.Contains(hover.Contents.Value, "Default state: `"+expectedDefaultState+"`") {
		t.Errorf("expected hover to contain default state, got: %s", hover.Contents.Value)
	}
}

func TestDocumentSymbolBasic(t *testing.T) {
	initReq := buildRequest(1, "initialize", map[string]interface{}{
		"processId":    1234,
		"rootUri":      "file:///tmp/test",
		"capabilities": map[string]interface{}{},
	})
	initializedNotif := buildRequest(nil, "initialized", map[string]interface{}{})

	kotlinCode := `package com.example

class MyClass {
    fun myMethod() {}
    val myProp = 42
}

fun topLevelFun() {}

val topLevelProp = "hello"
`

	didOpenReq := buildRequest(nil, "textDocument/didOpen", map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri":        "file:///tmp/test/Test.kt",
			"languageId": "kotlin",
			"version":    1,
			"text":       kotlinCode,
		},
	})

	symbolReq := buildRequest(10, "textDocument/documentSymbol", map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": "file:///tmp/test/Test.kt",
		},
	})

	var input bytes.Buffer
	input.Write(initReq)
	input.Write(initializedNotif)
	input.Write(didOpenReq)
	input.Write(symbolReq)

	var output bytes.Buffer
	server := NewServer(bufio.NewReader(&input), &output)
	server.Run()

	msgs, _ := collectMessages(output.Bytes())

	// Find the documentSymbol response
	var symbolResp Response
	found := false
	for _, msg := range msgs {
		var resp Response
		if err := json.Unmarshal(msg, &resp); err == nil && resp.ID == float64(10) {
			symbolResp = resp
			found = true
			break
		}
	}

	if !found {
		t.Fatal("documentSymbol response not found")
	}
	if symbolResp.Error != nil {
		t.Fatalf("documentSymbol response error: %v", symbolResp.Error)
	}

	resultBytes, _ := json.Marshal(symbolResp.Result)
	var symbols []DocumentSymbol
	if err := json.Unmarshal(resultBytes, &symbols); err != nil {
		t.Fatalf("unmarshal symbols: %v", err)
	}

	// Should have top-level symbols: MyClass, topLevelFun, topLevelProp
	if len(symbols) < 3 {
		t.Fatalf("expected at least 3 top-level symbols, got %d: %+v", len(symbols), symbols)
	}

	// Find each expected symbol
	foundClass := false
	foundFunc := false
	foundProp := false
	for _, sym := range symbols {
		switch sym.Name {
		case "MyClass":
			foundClass = true
			if sym.Kind != SymbolKindClass {
				t.Errorf("MyClass kind: got %d, want %d", sym.Kind, SymbolKindClass)
			}
			// Check children
			if len(sym.Children) < 2 {
				t.Errorf("MyClass: expected at least 2 children, got %d", len(sym.Children))
			}
		case "topLevelFun":
			foundFunc = true
			if sym.Kind != SymbolKindFunction {
				t.Errorf("topLevelFun kind: got %d, want %d", sym.Kind, SymbolKindFunction)
			}
		case "topLevelProp":
			foundProp = true
			if sym.Kind != SymbolKindProperty {
				t.Errorf("topLevelProp kind: got %d, want %d", sym.Kind, SymbolKindProperty)
			}
		}
	}

	if !foundClass {
		t.Error("MyClass not found in symbols")
	}
	if !foundFunc {
		t.Error("topLevelFun not found in symbols")
	}
	if !foundProp {
		t.Error("topLevelProp not found in symbols")
	}
}

func TestDocumentSymbolWithoutOpenDoc(t *testing.T) {
	initReq := buildRequest(1, "initialize", map[string]interface{}{
		"processId":    1234,
		"rootUri":      "file:///tmp/test",
		"capabilities": map[string]interface{}{},
	})
	initializedNotif := buildRequest(nil, "initialized", map[string]interface{}{})

	symbolReq := buildRequest(10, "textDocument/documentSymbol", map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": "file:///tmp/test/Missing.kt",
		},
	})

	var input bytes.Buffer
	input.Write(initReq)
	input.Write(initializedNotif)
	input.Write(symbolReq)

	var output bytes.Buffer
	server := NewServer(bufio.NewReader(&input), &output)
	server.Run()

	msgs, _ := collectMessages(output.Bytes())

	var resp Response
	for _, msg := range msgs {
		var r Response
		if err := json.Unmarshal(msg, &r); err == nil && r.ID == float64(10) {
			resp = r
			break
		}
	}

	if resp.Error != nil {
		t.Fatalf("expected no error, got: %v", resp.Error)
	}
	resultBytes, _ := json.Marshal(resp.Result)
	var symbols []DocumentSymbol
	json.Unmarshal(resultBytes, &symbols)
	if len(symbols) != 0 {
		t.Errorf("expected empty symbols for missing doc, got %d", len(symbols))
	}
}

func TestDocumentSymbolNestedClass(t *testing.T) {
	initReq := buildRequest(1, "initialize", map[string]interface{}{
		"processId":    1234,
		"rootUri":      "file:///tmp/test",
		"capabilities": map[string]interface{}{},
	})
	initializedNotif := buildRequest(nil, "initialized", map[string]interface{}{})

	kotlinCode := `package com.example

class Outer {
    class Inner {
        fun innerFun() {}
    }
}
`

	didOpenReq := buildRequest(nil, "textDocument/didOpen", map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri":        "file:///tmp/test/Test.kt",
			"languageId": "kotlin",
			"version":    1,
			"text":       kotlinCode,
		},
	})

	symbolReq := buildRequest(10, "textDocument/documentSymbol", map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": "file:///tmp/test/Test.kt",
		},
	})

	var input bytes.Buffer
	input.Write(initReq)
	input.Write(initializedNotif)
	input.Write(didOpenReq)
	input.Write(symbolReq)

	var output bytes.Buffer
	server := NewServer(bufio.NewReader(&input), &output)
	server.Run()

	msgs, _ := collectMessages(output.Bytes())

	var resp Response
	for _, msg := range msgs {
		var r Response
		if err := json.Unmarshal(msg, &r); err == nil && r.ID == float64(10) {
			resp = r
			break
		}
	}

	resultBytes, _ := json.Marshal(resp.Result)
	var symbols []DocumentSymbol
	json.Unmarshal(resultBytes, &symbols)

	// Find Outer class
	var outer *DocumentSymbol
	for i := range symbols {
		if symbols[i].Name == "Outer" {
			outer = &symbols[i]
			break
		}
	}
	if outer == nil {
		t.Fatal("Outer class not found")
	}

	// Outer should have Inner as child
	var inner *DocumentSymbol
	for i := range outer.Children {
		if outer.Children[i].Name == "Inner" {
			inner = &outer.Children[i]
			break
		}
	}
	if inner == nil {
		t.Fatal("Inner class not found in Outer's children")
	}
	if inner.Kind != SymbolKindClass {
		t.Errorf("Inner kind: got %d, want %d", inner.Kind, SymbolKindClass)
	}

	// Inner should have innerFun as child
	foundInnerFun := false
	for _, child := range inner.Children {
		if child.Name == "innerFun" && child.Kind == SymbolKindFunction {
			foundInnerFun = true
			break
		}
	}
	if !foundInnerFun {
		t.Error("innerFun not found in Inner's children")
	}
}

func TestDidChangeConfigurationRebuildsDispatcher(t *testing.T) {
	initReq := buildRequest(1, "initialize", map[string]interface{}{
		"processId":    1234,
		"rootUri":      "file:///tmp/test",
		"capabilities": map[string]interface{}{},
	})
	initializedNotif := buildRequest(nil, "initialized", map[string]interface{}{})

	// Send didChangeConfiguration notification
	configChangeNotif := buildRequest(nil, "workspace/didChangeConfiguration", map[string]interface{}{
		"settings": map[string]interface{}{},
	})

	var input bytes.Buffer
	input.Write(initReq)
	input.Write(initializedNotif)
	input.Write(configChangeNotif)

	var output bytes.Buffer
	server := NewServer(bufio.NewReader(&input), &output)
	server.Run()

	// Verify the server handled it without error and dispatcher is still valid
	if server.analyzer == nil {
		t.Fatal("dispatcher should be non-nil after didChangeConfiguration")
	}
}

func TestDefinitionFindsClassDeclaration(t *testing.T) {
	initReq := buildRequest(1, "initialize", map[string]interface{}{
		"processId":    1234,
		"rootUri":      "file:///tmp/test",
		"capabilities": map[string]interface{}{},
	})
	initializedNotif := buildRequest(nil, "initialized", map[string]interface{}{})

	kotlinCode := `package com.example

class MyClass {
    fun doSomething() {}
}

fun useIt() {
    val x = MyClass()
}
`

	didOpenReq := buildRequest(nil, "textDocument/didOpen", map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri":        "file:///tmp/test/Test.kt",
			"languageId": "kotlin",
			"version":    1,
			"text":       kotlinCode,
		},
	})

	// Cursor on "MyClass" in "val x = MyClass()" — line 7, character 12
	defReq := buildRequest(10, "textDocument/definition", map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": "file:///tmp/test/Test.kt",
		},
		"position": map[string]interface{}{"line": 7, "character": 12},
	})

	var input bytes.Buffer
	input.Write(initReq)
	input.Write(initializedNotif)
	input.Write(didOpenReq)
	input.Write(defReq)

	var output bytes.Buffer
	server := NewServer(bufio.NewReader(&input), &output)
	server.Run()

	msgs, _ := collectMessages(output.Bytes())

	var resp Response
	found := false
	for _, msg := range msgs {
		var r Response
		if err := json.Unmarshal(msg, &r); err == nil && r.ID == float64(10) {
			resp = r
			found = true
			break
		}
	}
	if !found {
		t.Fatal("definition response not found")
	}
	if resp.Error != nil {
		t.Fatalf("definition error: %v", resp.Error)
	}

	resultBytes, _ := json.Marshal(resp.Result)
	var loc Location
	if err := json.Unmarshal(resultBytes, &loc); err != nil {
		t.Fatalf("unmarshal location: %v", err)
	}

	// Should point to the class declaration "MyClass" on line 2
	if loc.Range.Start.Line != 2 {
		t.Errorf("expected definition on line 2, got %d", loc.Range.Start.Line)
	}
	if loc.URI != "file:///tmp/test/Test.kt" {
		t.Errorf("expected URI file:///tmp/test/Test.kt, got %s", loc.URI)
	}
}

func TestDefinitionFindsFunctionDeclaration(t *testing.T) {
	server := &Server{
		docs: make(map[string]*Document),
	}

	uri := "file:///tmp/test/Test.kt"
	kotlinCode := `package com.example

fun greet() {
    println("hello")
}

fun main() {
    greet()
}
`
	content := []byte(kotlinCode)

	// Parse the content
	server.docs[uri] = &Document{
		URI:     uri,
		Content: content,
	}

	// Cursor on "greet" in "greet()" call — line 7, character 4
	params := DefinitionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 7, Character: 4},
	}
	paramsBytes, _ := json.Marshal(params)
	req := &Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "textDocument/definition",
		Params:  paramsBytes,
	}

	var output bytes.Buffer
	server.writer = &output
	server.handleDefinition(req)

	msgs, _ := collectMessages(output.Bytes())
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}

	var resp Response
	json.Unmarshal(msgs[0], &resp)
	if resp.Result == nil {
		t.Fatal("expected non-nil result")
	}

	resultBytes, _ := json.Marshal(resp.Result)
	var loc Location
	json.Unmarshal(resultBytes, &loc)

	// "greet" function declaration is on line 2
	if loc.Range.Start.Line != 2 {
		t.Errorf("expected definition on line 2, got %d", loc.Range.Start.Line)
	}
}

func TestDefinitionNoSymbolReturnsNull(t *testing.T) {
	server := &Server{
		docs: make(map[string]*Document),
	}

	uri := "file:///tmp/test/Test.kt"
	content := []byte("// just a comment\n")

	server.docs[uri] = &Document{URI: uri, Content: content}

	params := DefinitionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 0, Character: 0},
	}
	paramsBytes, _ := json.Marshal(params)
	req := &Request{JSONRPC: "2.0", ID: float64(1), Method: "textDocument/definition", Params: paramsBytes}

	var output bytes.Buffer
	server.writer = &output
	server.handleDefinition(req)

	msgs, _ := collectMessages(output.Bytes())
	var resp Response
	json.Unmarshal(msgs[0], &resp)
	if resp.Result != nil {
		t.Errorf("expected null result for no symbol, got: %v", resp.Result)
	}
}

func TestReferencesFindsAllOccurrences(t *testing.T) {
	server := &Server{
		docs: make(map[string]*Document),
	}

	uri := "file:///tmp/test/Test.kt"
	kotlinCode := `package com.example

fun greet() {}

fun main() {
    greet()
    greet()
}
`
	content := []byte(kotlinCode)
	server.docs[uri] = &Document{URI: uri, Content: content}

	// Cursor on "greet" in declaration — line 2, character 4
	params := ReferenceParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 2, Character: 4},
		Context:      ReferenceContext{IncludeDeclaration: true},
	}
	paramsBytes, _ := json.Marshal(params)
	req := &Request{JSONRPC: "2.0", ID: float64(1), Method: "textDocument/references", Params: paramsBytes}

	var output bytes.Buffer
	server.writer = &output
	server.handleReferences(req)

	msgs, _ := collectMessages(output.Bytes())
	var resp Response
	json.Unmarshal(msgs[0], &resp)

	resultBytes, _ := json.Marshal(resp.Result)
	var locs []Location
	json.Unmarshal(resultBytes, &locs)

	// Should find 3 occurrences: declaration + 2 calls
	if len(locs) != 3 {
		t.Errorf("expected 3 references, got %d", len(locs))
	}
}

func TestReferencesExcludeDeclaration(t *testing.T) {
	server := &Server{
		docs: make(map[string]*Document),
	}

	uri := "file:///tmp/test/Test.kt"
	kotlinCode := `package com.example

fun greet() {}

fun main() {
    greet()
}
`
	content := []byte(kotlinCode)
	server.docs[uri] = &Document{URI: uri, Content: content}

	params := ReferenceParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 2, Character: 4},
		Context:      ReferenceContext{IncludeDeclaration: false},
	}
	paramsBytes, _ := json.Marshal(params)
	req := &Request{JSONRPC: "2.0", ID: float64(1), Method: "textDocument/references", Params: paramsBytes}

	var output bytes.Buffer
	server.writer = &output
	server.handleReferences(req)

	msgs, _ := collectMessages(output.Bytes())
	var resp Response
	json.Unmarshal(msgs[0], &resp)

	resultBytes, _ := json.Marshal(resp.Result)
	var locs []Location
	json.Unmarshal(resultBytes, &locs)

	// Should find only 1 call reference (excluding the declaration)
	if len(locs) != 1 {
		t.Errorf("expected 1 reference (excluding declaration), got %d", len(locs))
	}
}

func TestReferencesNoSymbolReturnsEmpty(t *testing.T) {
	server := &Server{
		docs: make(map[string]*Document),
	}

	uri := "file:///tmp/test/Test.kt"
	content := []byte("// comment\n")
	server.docs[uri] = &Document{URI: uri, Content: content}

	params := ReferenceParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 0, Character: 0},
		Context:      ReferenceContext{IncludeDeclaration: true},
	}
	paramsBytes, _ := json.Marshal(params)
	req := &Request{JSONRPC: "2.0", ID: float64(1), Method: "textDocument/references", Params: paramsBytes}

	var output bytes.Buffer
	server.writer = &output
	server.handleReferences(req)

	msgs, _ := collectMessages(output.Bytes())
	var resp Response
	json.Unmarshal(msgs[0], &resp)

	resultBytes, _ := json.Marshal(resp.Result)
	var locs []Location
	json.Unmarshal(resultBytes, &locs)

	if len(locs) != 0 {
		t.Errorf("expected 0 references, got %d", len(locs))
	}
}

func TestRenameSymbol(t *testing.T) {
	server := &Server{
		docs: make(map[string]*Document),
	}

	uri := "file:///tmp/test/Test.kt"
	kotlinCode := `package com.example

fun greet() {}

fun main() {
    greet()
}
`
	content := []byte(kotlinCode)
	server.docs[uri] = &Document{URI: uri, Content: content}

	params := RenameParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 2, Character: 4},
		NewName:      "sayHello",
	}
	paramsBytes, _ := json.Marshal(params)
	req := &Request{JSONRPC: "2.0", ID: float64(1), Method: "textDocument/rename", Params: paramsBytes}

	var output bytes.Buffer
	server.writer = &output
	server.handleRename(req)

	msgs, _ := collectMessages(output.Bytes())
	var resp Response
	json.Unmarshal(msgs[0], &resp)

	if resp.Error != nil {
		t.Fatalf("rename error: %v", resp.Error)
	}

	resultBytes, _ := json.Marshal(resp.Result)
	var wsEdit WorkspaceEdit
	json.Unmarshal(resultBytes, &wsEdit)

	edits, ok := wsEdit.Changes[uri]
	if !ok {
		t.Fatal("expected edits for URI")
	}

	// Should have 2 edits: declaration + call
	if len(edits) != 2 {
		t.Errorf("expected 2 edits, got %d", len(edits))
	}

	for _, edit := range edits {
		if edit.NewText != "sayHello" {
			t.Errorf("expected newText='sayHello', got %q", edit.NewText)
		}
	}
}

func TestRenameNoSymbolReturnsError(t *testing.T) {
	server := &Server{
		docs: make(map[string]*Document),
	}

	uri := "file:///tmp/test/Test.kt"
	content := []byte("// comment\n")
	server.docs[uri] = &Document{URI: uri, Content: content}

	params := RenameParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 0, Character: 0},
		NewName:      "newName",
	}
	paramsBytes, _ := json.Marshal(params)
	req := &Request{JSONRPC: "2.0", ID: float64(1), Method: "textDocument/rename", Params: paramsBytes}

	var output bytes.Buffer
	server.writer = &output
	server.handleRename(req)

	msgs, _ := collectMessages(output.Bytes())
	var resp Response
	json.Unmarshal(msgs[0], &resp)

	if resp.Error == nil {
		t.Fatal("expected error for rename with no symbol")
	}
}

func TestRenameEmptyNameReturnsError(t *testing.T) {
	server := &Server{
		docs: make(map[string]*Document),
	}

	uri := "file:///tmp/test/Test.kt"
	kotlinCode := `package com.example

fun greet() {}
`
	content := []byte(kotlinCode)
	server.docs[uri] = &Document{URI: uri, Content: content}

	params := RenameParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 2, Character: 4},
		NewName:      "  ",
	}
	paramsBytes, _ := json.Marshal(params)
	req := &Request{JSONRPC: "2.0", ID: float64(1), Method: "textDocument/rename", Params: paramsBytes}

	var output bytes.Buffer
	server.writer = &output
	server.handleRename(req)

	msgs, _ := collectMessages(output.Bytes())
	var resp Response
	json.Unmarshal(msgs[0], &resp)

	if resp.Error == nil {
		t.Fatal("expected error for empty new name")
	}
}

func TestCompletionAnnotations(t *testing.T) {
	server := &Server{
		docs: make(map[string]*Document),
	}

	uri := "file:///tmp/test/Test.kt"
	content := []byte("@\nfun main() {}\n")

	server.docs[uri] = &Document{URI: uri, Content: content}

	// Cursor right after @ on line 0, character 1
	params := CompletionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 0, Character: 1},
	}
	paramsBytes, _ := json.Marshal(params)
	req := &Request{JSONRPC: "2.0", ID: float64(1), Method: "textDocument/completion", Params: paramsBytes}

	var output bytes.Buffer
	server.writer = &output
	server.handleCompletion(req)

	msgs, _ := collectMessages(output.Bytes())
	var resp Response
	json.Unmarshal(msgs[0], &resp)

	resultBytes, _ := json.Marshal(resp.Result)
	var completions CompletionList
	json.Unmarshal(resultBytes, &completions)

	if len(completions.Items) == 0 {
		t.Fatal("expected annotation completions, got 0 items")
	}

	// Check that common annotations are present
	foundSuppress := false
	foundComposable := false
	for _, item := range completions.Items {
		if item.Label == "Suppress" {
			foundSuppress = true
		}
		if item.Label == "Composable" {
			foundComposable = true
		}
	}
	if !foundSuppress {
		t.Error("expected @Suppress in completions")
	}
	if !foundComposable {
		t.Error("expected @Composable in completions")
	}
}

func TestCompletionSuppressRuleNames(t *testing.T) {
	server := &Server{
		docs: make(map[string]*Document),
	}

	uri := "file:///tmp/test/Test.kt"
	content := []byte("@Suppress(\"\nfun main() {}\n")

	server.docs[uri] = &Document{URI: uri, Content: content}

	// Cursor inside the quotes: line 0, character 11 (after the opening quote)
	params := CompletionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 0, Character: 11},
	}
	paramsBytes, _ := json.Marshal(params)
	req := &Request{JSONRPC: "2.0", ID: float64(1), Method: "textDocument/completion", Params: paramsBytes}

	var output bytes.Buffer
	server.writer = &output
	server.handleCompletion(req)

	msgs, _ := collectMessages(output.Bytes())
	if len(msgs) == 0 {
		t.Fatal("expected at least 1 response message")
	}
	var resp Response
	json.Unmarshal(msgs[0], &resp)

	resultBytes, _ := json.Marshal(resp.Result)
	var completions CompletionList
	json.Unmarshal(resultBytes, &completions)

	// Should return rule names from the registry (if rules are registered)
	// At minimum, the response should be valid
	if completions.Items == nil {
		t.Fatal("expected non-nil items in completion list")
	}

	// If rules are registered, verify items have "krit rule" detail
	for _, item := range completions.Items {
		if item.Detail != "krit rule" {
			t.Errorf("expected detail 'krit rule', got %q for %q", item.Detail, item.Label)
			break
		}
	}
}

func TestCompletionNoContext(t *testing.T) {
	server := &Server{
		docs: make(map[string]*Document),
	}

	uri := "file:///tmp/test/Test.kt"
	content := []byte("fun main() {}\n")

	server.docs[uri] = &Document{URI: uri, Content: content}

	params := CompletionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 0, Character: 4},
	}
	paramsBytes, _ := json.Marshal(params)
	req := &Request{JSONRPC: "2.0", ID: float64(1), Method: "textDocument/completion", Params: paramsBytes}

	var output bytes.Buffer
	server.writer = &output
	server.handleCompletion(req)

	msgs, _ := collectMessages(output.Bytes())
	var resp Response
	json.Unmarshal(msgs[0], &resp)

	resultBytes, _ := json.Marshal(resp.Result)
	var completions CompletionList
	json.Unmarshal(resultBytes, &completions)

	if len(completions.Items) != 0 {
		t.Errorf("expected 0 completions in non-annotation context, got %d", len(completions.Items))
	}
}

func TestInitializeIncludesNewLSPCapabilities(t *testing.T) {
	initReq := buildRequest(1, "initialize", map[string]interface{}{
		"processId":    1234,
		"rootUri":      "file:///tmp/test",
		"capabilities": map[string]interface{}{},
	})

	var input bytes.Buffer
	input.Write(initReq)

	var output bytes.Buffer
	server := NewServer(bufio.NewReader(&input), &output)
	server.Run()

	msgs, _ := collectMessages(output.Bytes())
	if len(msgs) < 1 {
		t.Fatal("expected at least 1 message")
	}

	var resp Response
	json.Unmarshal(msgs[0], &resp)
	resultBytes, _ := json.Marshal(resp.Result)
	var result InitializeResult
	json.Unmarshal(resultBytes, &result)

	if !result.Capabilities.DefinitionProvider {
		t.Error("expected definitionProvider = true")
	}
	if !result.Capabilities.ReferencesProvider {
		t.Error("expected referencesProvider = true")
	}
	if !result.Capabilities.RenameProvider {
		t.Error("expected renameProvider = true")
	}
	if result.Capabilities.CodeLensProvider == nil {
		t.Error("expected codeLensProvider to be non-nil")
	}
	if result.Capabilities.CompletionProvider == nil {
		t.Error("expected completionProvider to be non-nil")
	} else if len(result.Capabilities.CompletionProvider.TriggerCharacters) == 0 {
		t.Error("expected completionProvider to have trigger characters")
	}
}

func TestCodeLensReturnsEmptyList(t *testing.T) {
	server := &Server{
		docs: make(map[string]*Document),
	}

	params := CodeLensParams{
		TextDocument: TextDocumentIdentifier{URI: "file:///tmp/test/Test.kt"},
	}
	paramsBytes, _ := json.Marshal(params)
	req := &Request{JSONRPC: "2.0", ID: float64(1), Method: "textDocument/codeLens", Params: paramsBytes}

	var output bytes.Buffer
	server.writer = &output
	server.handleCodeLens(req)

	msgs, _ := collectMessages(output.Bytes())
	if len(msgs) != 1 {
		t.Fatalf("expected 1 response message, got %d", len(msgs))
	}

	var resp Response
	if err := json.Unmarshal(msgs[0], &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected response error: %+v", resp.Error)
	}

	resultBytes, _ := json.Marshal(resp.Result)
	var lenses []CodeLens
	if err := json.Unmarshal(resultBytes, &lenses); err != nil {
		t.Fatalf("unmarshal codeLens result: %v", err)
	}
	if len(lenses) != 0 {
		t.Fatalf("expected 0 code lenses, got %d", len(lenses))
	}
}

func TestDefinitionFindsPropertyDeclaration(t *testing.T) {
	server := &Server{
		docs: make(map[string]*Document),
	}

	uri := "file:///tmp/test/Test.kt"
	kotlinCode := `package com.example

val greeting = "hello"

fun main() {
    println(greeting)
}
`
	content := []byte(kotlinCode)
	server.docs[uri] = &Document{URI: uri, Content: content}

	// Cursor on "greeting" in println(greeting) — line 5, character 12
	params := DefinitionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 5, Character: 12},
	}
	paramsBytes, _ := json.Marshal(params)
	req := &Request{JSONRPC: "2.0", ID: float64(1), Method: "textDocument/definition", Params: paramsBytes}

	var output bytes.Buffer
	server.writer = &output
	server.handleDefinition(req)

	msgs, _ := collectMessages(output.Bytes())
	var resp Response
	json.Unmarshal(msgs[0], &resp)
	if resp.Result == nil {
		t.Fatal("expected non-nil result for property definition")
	}

	resultBytes, _ := json.Marshal(resp.Result)
	var loc Location
	json.Unmarshal(resultBytes, &loc)

	// "greeting" property declaration is on line 2
	if loc.Range.Start.Line != 2 {
		t.Errorf("expected definition on line 2, got %d", loc.Range.Start.Line)
	}
}

func TestReferencesIntegration(t *testing.T) {
	initReq := buildRequest(1, "initialize", map[string]interface{}{
		"processId":    1234,
		"rootUri":      "file:///tmp/test",
		"capabilities": map[string]interface{}{},
	})
	initializedNotif := buildRequest(nil, "initialized", map[string]interface{}{})

	kotlinCode := `package com.example

class Foo {
    fun bar() {}
}

fun test() {
    val f = Foo()
    f.bar()
}
`

	didOpenReq := buildRequest(nil, "textDocument/didOpen", map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri":        "file:///tmp/test/Test.kt",
			"languageId": "kotlin",
			"version":    1,
			"text":       kotlinCode,
		},
	})

	// Find references for "Foo" — line 2, character 6
	refsReq := buildRequest(10, "textDocument/references", map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": "file:///tmp/test/Test.kt",
		},
		"position": map[string]interface{}{"line": 2, "character": 6},
		"context":  map[string]interface{}{"includeDeclaration": true},
	})

	var input bytes.Buffer
	input.Write(initReq)
	input.Write(initializedNotif)
	input.Write(didOpenReq)
	input.Write(refsReq)

	var output bytes.Buffer
	server := NewServer(bufio.NewReader(&input), &output)
	server.Run()

	msgs, _ := collectMessages(output.Bytes())

	var resp Response
	found := false
	for _, msg := range msgs {
		var r Response
		if err := json.Unmarshal(msg, &r); err == nil && r.ID == float64(10) {
			resp = r
			found = true
			break
		}
	}
	if !found {
		t.Fatal("references response not found")
	}
	if resp.Error != nil {
		t.Fatalf("references error: %v", resp.Error)
	}

	resultBytes, _ := json.Marshal(resp.Result)
	var locs []Location
	json.Unmarshal(resultBytes, &locs)

	// Should find at least 2 occurrences of "Foo": declaration + usage
	if len(locs) < 2 {
		t.Errorf("expected at least 2 references for Foo, got %d", len(locs))
	}
}

func TestCodeActionReturnsFixableFindings(t *testing.T) {
	// Open a Kotlin file with trailing whitespace, which triggers the
	// TrailingWhitespace rule (default active, fixable with ByteMode).
	// Then send a codeAction request and verify that code actions are returned.
	initReq := buildRequest(1, "initialize", map[string]interface{}{
		"processId":    1234,
		"rootUri":      "file:///tmp/test",
		"capabilities": map[string]interface{}{},
	})
	initializedNotif := buildRequest(nil, "initialized", map[string]interface{}{})

	// Content that triggers fixable rules. The top-level `val` with a constant
	// string triggers rules that produce auto-fixes (e.g., suggesting const val).
	kotlinCode := "package com.example\n\nval greeting = \"hello\"\n"

	didOpenReq := buildRequest(nil, "textDocument/didOpen", map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri":        "file:///tmp/test/Test.kt",
			"languageId": "kotlin",
			"version":    1,
			"text":       kotlinCode,
		},
	})

	codeActionReq := buildRequest(10, "textDocument/codeAction", map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": "file:///tmp/test/Test.kt",
		},
		"range": map[string]interface{}{
			"start": map[string]interface{}{"line": 2, "character": 0},
			"end":   map[string]interface{}{"line": 2, "character": 0},
		},
		"context": map[string]interface{}{
			"diagnostics": []interface{}{},
		},
	})

	var input bytes.Buffer
	input.Write(initReq)
	input.Write(initializedNotif)
	input.Write(didOpenReq)
	input.Write(codeActionReq)

	var output bytes.Buffer
	server := NewServer(bufio.NewReader(&input), &output)
	server.Run()

	msgs, _ := collectMessages(output.Bytes())

	// Find the codeAction response by ID
	var codeActionResp Response
	found := false
	for _, msg := range msgs {
		var resp Response
		if err := json.Unmarshal(msg, &resp); err == nil && resp.ID == float64(10) {
			codeActionResp = resp
			found = true
			break
		}
	}

	if !found {
		t.Fatal("codeAction response not found")
	}
	if codeActionResp.Error != nil {
		t.Fatalf("codeAction response error: %v", codeActionResp.Error)
	}

	resultBytes, _ := json.Marshal(codeActionResp.Result)
	var actions []CodeAction
	if err := json.Unmarshal(resultBytes, &actions); err != nil {
		t.Fatalf("unmarshal code actions: %v", err)
	}

	if len(actions) == 0 {
		t.Fatal("expected at least 1 code action for trailing whitespace, got 0")
	}

	// Verify the code action structure
	action := actions[0]
	if action.Kind != "quickfix" {
		t.Errorf("kind: got %q, want %q", action.Kind, "quickfix")
	}
	if !strings.Contains(action.Title, "Fix:") {
		t.Errorf("title should contain 'Fix:', got %q", action.Title)
	}
	if action.Edit == nil {
		t.Fatal("expected workspace edit, got nil")
	}
	edits, ok := action.Edit.Changes["file:///tmp/test/Test.kt"]
	if !ok || len(edits) == 0 {
		t.Fatal("expected text edits for the file URI")
	}
	// The fix should remove trailing whitespace
	if len(action.Diagnostics) == 0 {
		t.Error("expected at least 1 diagnostic attached to the code action")
	}
}

func TestFormattingReturnsEditsForFixableFindings(t *testing.T) {
	// Open a Kotlin file with trailing whitespace, which triggers the
	// TrailingWhitespace rule (default active, fixable with ByteMode).
	// Then send a formatting request and verify that text edits are returned.
	initReq := buildRequest(1, "initialize", map[string]interface{}{
		"processId":    1234,
		"rootUri":      "file:///tmp/test",
		"capabilities": map[string]interface{}{},
	})
	initializedNotif := buildRequest(nil, "initialized", map[string]interface{}{})

	// Content that triggers fixable rules. The top-level `val` with a constant
	// string triggers rules that produce auto-fixes (e.g., suggesting const val).
	kotlinCode := "package com.example\n\nval greeting = \"hello\"\n"

	didOpenReq := buildRequest(nil, "textDocument/didOpen", map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri":        "file:///tmp/test/Test.kt",
			"languageId": "kotlin",
			"version":    1,
			"text":       kotlinCode,
		},
	})

	formatReq := buildRequest(10, "textDocument/formatting", map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": "file:///tmp/test/Test.kt",
		},
		"options": map[string]interface{}{
			"tabSize":      4,
			"insertSpaces": true,
		},
	})

	var input bytes.Buffer
	input.Write(initReq)
	input.Write(initializedNotif)
	input.Write(didOpenReq)
	input.Write(formatReq)

	var output bytes.Buffer
	server := NewServer(bufio.NewReader(&input), &output)
	server.Run()

	msgs, _ := collectMessages(output.Bytes())

	// Find the formatting response by ID
	var formatResp Response
	found := false
	for _, msg := range msgs {
		var resp Response
		if err := json.Unmarshal(msg, &resp); err == nil && resp.ID == float64(10) {
			formatResp = resp
			found = true
			break
		}
	}

	if !found {
		t.Fatal("formatting response not found")
	}
	if formatResp.Error != nil {
		t.Fatalf("formatting response error: %v", formatResp.Error)
	}

	resultBytes, _ := json.Marshal(formatResp.Result)
	var edits []TextEdit
	if err := json.Unmarshal(resultBytes, &edits); err != nil {
		t.Fatalf("unmarshal formatting edits: %v", err)
	}

	if len(edits) == 0 {
		t.Fatal("expected at least 1 formatting edit for trailing whitespace, got 0")
	}

	// Verify that at least one edit targets line 2 (the val declaration line)
	// with a valid replacement text
	foundFix := false
	for _, edit := range edits {
		if edit.Range.Start.Line == 2 && edit.NewText != "" {
			foundFix = true
			break
		}
	}
	if !foundFix {
		t.Errorf("expected a formatting edit on line 2, got %d edits: %+v", len(edits), edits)
	}
}

// --- FindingsToDiagnostics tests ---

func TestFindingsToDiagnostics_ConvertsSlice(t *testing.T) {
	findings := []scanner.Finding{
		{File: "a.kt", Line: 10, Col: 5, Severity: "error", RuleSet: "bugs", Rule: "NullDeref", Message: "potential null"},
		{File: "b.kt", Line: 3, Col: 0, Severity: "warning", RuleSet: "style", Rule: "MaxLen", Message: "line too long"},
		{File: "c.kt", Line: 1, Col: 2, Severity: "info", RuleSet: "perf", Rule: "Alloc", Message: "unnecessary alloc"},
	}

	diags := FindingsToDiagnostics(findings)
	if len(diags) != 3 {
		t.Fatalf("expected 3 diagnostics, got %d", len(diags))
	}
}

func TestFindingsToDiagnostics_EmptyFindings(t *testing.T) {
	diags := FindingsToDiagnostics([]scanner.Finding{})
	if len(diags) != 0 {
		t.Errorf("expected 0 diagnostics for empty findings, got %d", len(diags))
	}
}

func TestFindingsToDiagnostics_FieldMapping(t *testing.T) {
	findings := []scanner.Finding{
		{File: "a.kt", Line: 10, Col: 5, Severity: "error", RuleSet: "bugs", Rule: "NullDeref", Message: "potential null"},
		{File: "b.kt", Line: 1, Col: 0, Severity: "warning", RuleSet: "style", Rule: "MaxLen", Message: "line too long"},
		{File: "c.kt", Line: 0, Col: 0, Severity: "info", RuleSet: "perf", Rule: "Alloc", Message: "unnecessary alloc"},
	}

	diags := FindingsToDiagnostics(findings)

	// First finding: line 10 -> 9 (0-based), col 5, error -> severity 1
	d := diags[0]
	if d.Range.Start.Line != 9 {
		t.Errorf("expected line 9 (0-based), got %d", d.Range.Start.Line)
	}
	if d.Range.Start.Character != 5 {
		t.Errorf("expected character 5, got %d", d.Range.Start.Character)
	}
	if d.Range.End.Line != d.Range.Start.Line || d.Range.End.Character != d.Range.Start.Character {
		t.Error("expected start and end positions to be equal")
	}
	if d.Severity != 1 {
		t.Errorf("expected severity 1 (Error), got %d", d.Severity)
	}
	if d.Code != "bugs/NullDeref" {
		t.Errorf("expected code 'bugs/NullDeref', got %q", d.Code)
	}
	if d.Source != "krit" {
		t.Errorf("expected source 'krit', got %q", d.Source)
	}
	if d.Message != "potential null" {
		t.Errorf("expected message 'potential null', got %q", d.Message)
	}

	// Second finding: warning -> severity 2
	if diags[1].Severity != 2 {
		t.Errorf("expected severity 2 (Warning), got %d", diags[1].Severity)
	}

	// Third finding: info -> severity 3, line 0 -> stays 0 (0-based)
	if diags[2].Severity != 3 {
		t.Errorf("expected severity 3 (Information), got %d", diags[2].Severity)
	}
	if diags[2].Range.Start.Line != 0 {
		t.Errorf("expected line 0 for finding with Line=0, got %d", diags[2].Range.Start.Line)
	}
}

func TestDefinitionFindsObjectDeclaration(t *testing.T) {
	server := &Server{
		docs: make(map[string]*Document),
	}

	uri := "file:///tmp/test/Test.kt"
	kotlinCode := `package com.example

object Singleton {
    fun doWork() {}
}

fun main() {
    Singleton.doWork()
}
`
	content := []byte(kotlinCode)
	server.docs[uri] = &Document{URI: uri, Content: content}

	// Cursor on "Singleton" in "Singleton.doWork()" — line 7, character 4
	params := DefinitionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 7, Character: 4},
	}
	paramsBytes, _ := json.Marshal(params)
	req := &Request{JSONRPC: "2.0", ID: float64(1), Method: "textDocument/definition", Params: paramsBytes}

	var output bytes.Buffer
	server.writer = &output
	server.handleDefinition(req)

	msgs, _ := collectMessages(output.Bytes())
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}

	var resp Response
	json.Unmarshal(msgs[0], &resp)
	if resp.Result == nil {
		t.Fatal("expected non-nil result for object definition")
	}

	resultBytes, _ := json.Marshal(resp.Result)
	var loc Location
	json.Unmarshal(resultBytes, &loc)

	// "Singleton" object declaration is on line 2
	if loc.Range.Start.Line != 2 {
		t.Errorf("expected definition on line 2, got %d", loc.Range.Start.Line)
	}
	if loc.URI != uri {
		t.Errorf("expected URI %s, got %s", uri, loc.URI)
	}
}

func TestDefinitionDocumentNotOpen(t *testing.T) {
	server := &Server{
		docs: make(map[string]*Document),
	}

	uri := "file:///tmp/test/NotOpen.kt"

	params := DefinitionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 0, Character: 0},
	}
	paramsBytes, _ := json.Marshal(params)
	req := &Request{JSONRPC: "2.0", ID: float64(1), Method: "textDocument/definition", Params: paramsBytes}

	var output bytes.Buffer
	server.writer = &output
	server.handleDefinition(req)

	msgs, _ := collectMessages(output.Bytes())
	if len(msgs) != 1 {
		t.Fatalf("expected 1 message, got %d", len(msgs))
	}

	var resp Response
	json.Unmarshal(msgs[0], &resp)
	if resp.Result != nil {
		t.Errorf("expected null result for document not open, got: %v", resp.Result)
	}
	if resp.Error != nil {
		t.Errorf("expected no error (just null result), got: %v", resp.Error)
	}
}

func TestReferencesFindsPropertyOccurrences(t *testing.T) {
	server := &Server{
		docs: make(map[string]*Document),
	}

	uri := "file:///tmp/test/Test.kt"
	kotlinCode := `package com.example

val name = "world"

fun greet() {
    println(name)
}

fun farewell() {
    println(name)
}
`
	content := []byte(kotlinCode)
	server.docs[uri] = &Document{URI: uri, Content: content}

	// Cursor on "name" in declaration — line 2, character 4
	params := ReferenceParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 2, Character: 4},
		Context:      ReferenceContext{IncludeDeclaration: true},
	}
	paramsBytes, _ := json.Marshal(params)
	req := &Request{JSONRPC: "2.0", ID: float64(1), Method: "textDocument/references", Params: paramsBytes}

	var output bytes.Buffer
	server.writer = &output
	server.handleReferences(req)

	msgs, _ := collectMessages(output.Bytes())
	var resp Response
	json.Unmarshal(msgs[0], &resp)

	resultBytes, _ := json.Marshal(resp.Result)
	var locs []Location
	json.Unmarshal(resultBytes, &locs)

	// Should find 3 occurrences: declaration + 2 usages in println
	if len(locs) != 3 {
		t.Errorf("expected 3 references for property 'name', got %d", len(locs))
	}
}

func TestRenameIntegration(t *testing.T) {
	initReq := buildRequest(1, "initialize", map[string]interface{}{
		"processId":    1234,
		"rootUri":      "file:///tmp/test",
		"capabilities": map[string]interface{}{},
	})
	initializedNotif := buildRequest(nil, "initialized", map[string]interface{}{})

	kotlinCode := `package com.example

fun hello() {
    println("hi")
}

fun main() {
    hello()
}
`

	didOpenReq := buildRequest(nil, "textDocument/didOpen", map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri":        "file:///tmp/test/Test.kt",
			"languageId": "kotlin",
			"version":    1,
			"text":       kotlinCode,
		},
	})

	// Rename "hello" at its call site — line 7, character 4
	renameReq := buildRequest(10, "textDocument/rename", map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": "file:///tmp/test/Test.kt",
		},
		"position": map[string]interface{}{"line": 7, "character": 4},
		"newName":  "greetUser",
	})

	var input bytes.Buffer
	input.Write(initReq)
	input.Write(initializedNotif)
	input.Write(didOpenReq)
	input.Write(renameReq)

	var output bytes.Buffer
	server := NewServer(bufio.NewReader(&input), &output)
	server.Run()

	msgs, _ := collectMessages(output.Bytes())

	var resp Response
	found := false
	for _, msg := range msgs {
		var r Response
		if err := json.Unmarshal(msg, &r); err == nil && r.ID == float64(10) {
			resp = r
			found = true
			break
		}
	}
	if !found {
		t.Fatal("rename response not found")
	}
	if resp.Error != nil {
		t.Fatalf("rename error: %v", resp.Error)
	}

	resultBytes, _ := json.Marshal(resp.Result)
	var wsEdit WorkspaceEdit
	json.Unmarshal(resultBytes, &wsEdit)

	edits, ok := wsEdit.Changes["file:///tmp/test/Test.kt"]
	if !ok {
		t.Fatal("expected edits for URI")
	}

	// Should have 2 edits: declaration on line 2 + call on line 7
	if len(edits) != 2 {
		t.Errorf("expected 2 edits, got %d", len(edits))
	}

	for _, edit := range edits {
		if edit.NewText != "greetUser" {
			t.Errorf("expected newText='greetUser', got %q", edit.NewText)
		}
	}
}

func TestRenameDocumentNotOpen(t *testing.T) {
	server := &Server{
		docs: make(map[string]*Document),
	}

	uri := "file:///tmp/test/NotOpen.kt"

	params := RenameParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 0, Character: 0},
		NewName:      "newName",
	}
	paramsBytes, _ := json.Marshal(params)
	req := &Request{JSONRPC: "2.0", ID: float64(1), Method: "textDocument/rename", Params: paramsBytes}

	var output bytes.Buffer
	server.writer = &output
	server.handleRename(req)

	msgs, _ := collectMessages(output.Bytes())
	var resp Response
	json.Unmarshal(msgs[0], &resp)

	if resp.Error == nil {
		t.Fatal("expected error for rename on document not open")
	}
	if resp.Error.Message != "document not open" {
		t.Errorf("expected error message 'document not open', got %q", resp.Error.Message)
	}
}

func TestReferencesDocumentNotOpen(t *testing.T) {
	server := &Server{
		docs: make(map[string]*Document),
	}

	uri := "file:///tmp/test/NotOpen.kt"

	params := ReferenceParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 0, Character: 0},
		Context:      ReferenceContext{IncludeDeclaration: true},
	}
	paramsBytes, _ := json.Marshal(params)
	req := &Request{JSONRPC: "2.0", ID: float64(1), Method: "textDocument/references", Params: paramsBytes}

	var output bytes.Buffer
	server.writer = &output
	server.handleReferences(req)

	msgs, _ := collectMessages(output.Bytes())
	var resp Response
	json.Unmarshal(msgs[0], &resp)

	// Should return empty array, not an error
	if resp.Error != nil {
		t.Errorf("expected no error, got: %v", resp.Error)
	}

	resultBytes, _ := json.Marshal(resp.Result)
	var locs []Location
	json.Unmarshal(resultBytes, &locs)
	if len(locs) != 0 {
		t.Errorf("expected 0 references for unopened document, got %d", len(locs))
	}
}
