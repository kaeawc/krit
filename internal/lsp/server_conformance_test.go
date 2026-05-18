package lsp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

// TestPostShutdownRequestRejected verifies the LSP shutdown gate: after a
// successful shutdown request, every subsequent request other than `exit`
// must be rejected with InvalidRequest (-32600). Reproduces bug A.
func TestPostShutdownRequestRejected(t *testing.T) {
	initReq := buildRequest(1, "initialize", map[string]interface{}{
		"processId":    1234,
		"rootUri":      "file:///tmp/test",
		"capabilities": map[string]interface{}{},
	})
	initializedNotif := buildRequest(nil, "initialized", map[string]interface{}{})
	shutdownReq := buildRequest(2, "shutdown", nil)
	codeActionReq := buildRequest(3, "textDocument/codeAction", map[string]interface{}{
		"textDocument": map[string]interface{}{"uri": "file:///tmp/test/Test.kt"},
		"range": map[string]interface{}{
			"start": map[string]interface{}{"line": 0, "character": 0},
			"end":   map[string]interface{}{"line": 0, "character": 0},
		},
		"context": map[string]interface{}{"diagnostics": []interface{}{}},
	})

	var input bytes.Buffer
	input.Write(initReq)
	input.Write(initializedNotif)
	input.Write(shutdownReq)
	input.Write(codeActionReq)

	var output bytes.Buffer
	server := NewServer(bufio.NewReader(&input), &output)
	server.Run()

	msgs, _ := collectMessages(output.Bytes())

	// Find the response for id=3 (the post-shutdown codeAction).
	var postShutdownResp Response
	found := false
	for _, msg := range msgs {
		var resp Response
		if err := json.Unmarshal(msg, &resp); err == nil && resp.ID == float64(3) {
			postShutdownResp = resp
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("no response for post-shutdown request id=3 (got %d messages)", len(msgs))
	}
	if postShutdownResp.Error == nil {
		t.Fatalf("expected error response after shutdown, got result=%v", postShutdownResp.Result)
	}
	if postShutdownResp.Error.Code != -32600 {
		t.Errorf("expected error code -32600 (InvalidRequest), got %d", postShutdownResp.Error.Code)
	}
}

// TestNotificationGetsNoResponse verifies that a known method sent as a
// notification (no `id` field) does NOT trigger a JSON-RPC response. Per
// JSON-RPC 2.0, notifications MUST NOT receive a response. Reproduces bug B.
//
// `textDocument/documentSymbol` is normally a request — but if a buggy client
// (or a test) sends it without an id, the previous server would happily emit
// `{"id":null,"result":[]}`. The fix is to gate sendResponse on a non-nil id.
func TestNotificationGetsNoResponse(t *testing.T) {
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
	// documentSymbol sent without an id → must be treated as a notification
	// and produce no response.
	symbolNotif := buildRequest(nil, "textDocument/documentSymbol", map[string]interface{}{
		"textDocument": map[string]interface{}{"uri": "file:///tmp/test/Test.kt"},
	})

	var input bytes.Buffer
	input.Write(initReq)
	input.Write(initializedNotif)
	input.Write(didOpenReq)
	input.Write(symbolNotif)

	var output bytes.Buffer
	server := NewServer(bufio.NewReader(&input), &output)
	server.Run()

	msgs, _ := collectMessages(output.Bytes())

	for i, msg := range msgs {
		var resp Response
		if err := json.Unmarshal(msg, &resp); err != nil {
			continue
		}
		// A response always has a "method"-less shape; a notification has a
		// method. Filter out the publishDiagnostics notification by checking
		// for a non-empty Method field.
		var probe struct {
			Method string      `json:"method"`
			ID     interface{} `json:"id"`
		}
		_ = json.Unmarshal(msg, &probe)
		if probe.Method != "" {
			// real notification (publishDiagnostics), ignore
			continue
		}
		// Anything left is a response. None of them should have id=null —
		// the server must never reply to a notification.
		if probe.ID == nil {
			t.Errorf("message %d is a response with null id, expected no response for notification: %s", i, string(msg))
		}
	}
}

// TestPreInitializeRequestRejected verifies the LSP initialization gate: a
// request other than initialize/initialized/exit, sent before the client has
// completed the initialize handshake, must be rejected with
// ServerNotInitialized (-32002). Reproduces bug C.
func TestPreInitializeRequestRejected(t *testing.T) {
	// didOpen sent as a request (with an id) before any initialize. The
	// server must respond -32002 rather than dispatching to handleDidOpen.
	didOpenReq := buildRequest(7, "textDocument/didOpen", map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri":        "file:///tmp/test/Early.kt",
			"languageId": "kotlin",
			"version":    1,
			"text":       "package com.example\n",
		},
	})

	var input bytes.Buffer
	input.Write(didOpenReq)

	var output bytes.Buffer
	server := NewServer(bufio.NewReader(&input), &output)
	server.Run()

	msgs, _ := collectMessages(output.Bytes())
	if len(msgs) < 1 {
		t.Fatal("expected at least 1 response to pre-initialize request")
	}

	var resp Response
	if err := json.Unmarshal(msgs[0], &resp); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if resp.ID != float64(7) {
		t.Errorf("response id: got %v, want 7", resp.ID)
	}
	if resp.Error == nil {
		t.Fatalf("expected error response, got result=%v", resp.Result)
	}
	if resp.Error.Code != -32002 {
		t.Errorf("expected error code -32002 (ServerNotInitialized), got %d", resp.Error.Code)
	}
}

// TestPreInitializeNotificationDropped confirms that a notification arriving
// before initialize is silently dropped — no response, no analyzer side
// effects. The pre-init gate must not emit anything when req.ID is nil.
func TestPreInitializeNotificationDropped(t *testing.T) {
	didOpenNotif := buildRequest(nil, "textDocument/didOpen", map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri":        "file:///tmp/test/Early.kt",
			"languageId": "kotlin",
			"version":    1,
			"text":       "package com.example\n",
		},
	})

	var input bytes.Buffer
	input.Write(didOpenNotif)

	var output bytes.Buffer
	server := NewServer(bufio.NewReader(&input), &output)
	server.Run()

	msgs, _ := collectMessages(output.Bytes())
	if len(msgs) != 0 {
		t.Errorf("expected no messages for pre-init notification, got %d: %v", len(msgs), msgs)
	}
}

// TestResolveFixPositionsPreservesCRLF verifies that resolveFixPositions
// rewrites bare-LF replacement text to match a CRLF document's line endings.
// Reproduces bug D: rules emit Replacement with "\n", but on Windows
// checkouts the document uses "\r\n" — returning the LF text verbatim
// produces mixed-ending edits that corrupt the buffer.
func TestResolveFixPositionsPreservesCRLF(t *testing.T) {
	t.Run("byte-mode multi-line replacement", func(t *testing.T) {
		content := "package com.example\r\n\r\nfun main() {}\r\n"
		fix := &scanner.Fix{
			ByteMode:    true,
			StartByte:   0,
			EndByte:     len(content),
			Replacement: "package com.example\n\nfun greet() {}\n",
		}
		_, _, newText, ok := resolveFixPositions(content, fix)
		if !ok {
			t.Fatal("resolveFixPositions returned ok=false")
		}
		if strings.Contains(newText, "\r\n") == false && strings.Contains(newText, "\n") {
			t.Errorf("expected CRLF in newText for CRLF document, got %q", newText)
		}
		// And specifically: no bare "\n" without a preceding "\r".
		for i := 0; i < len(newText); i++ {
			if newText[i] == '\n' && (i == 0 || newText[i-1] != '\r') {
				t.Errorf("bare LF at offset %d in CRLF replacement: %q", i, newText)
				break
			}
		}
	})

	t.Run("line-mode multi-line replacement", func(t *testing.T) {
		content := "line1\r\nline2\r\nline3\r\n"
		fix := &scanner.Fix{
			ByteMode:    false,
			StartLine:   2,
			EndLine:     2,
			Replacement: "replaced-a\nreplaced-b\n",
		}
		_, _, newText, ok := resolveFixPositions(content, fix)
		if !ok {
			t.Fatal("resolveFixPositions returned ok=false")
		}
		if !strings.Contains(newText, "\r\n") {
			t.Errorf("expected CRLF in newText for CRLF document, got %q", newText)
		}
		for i := 0; i < len(newText); i++ {
			if newText[i] == '\n' && (i == 0 || newText[i-1] != '\r') {
				t.Errorf("bare LF at offset %d in CRLF replacement: %q", i, newText)
				break
			}
		}
	})

	t.Run("LF document is untouched", func(t *testing.T) {
		content := "line1\nline2\nline3\n"
		fix := &scanner.Fix{
			ByteMode:    false,
			StartLine:   2,
			EndLine:     2,
			Replacement: "replaced-a\nreplaced-b\n",
		}
		_, _, newText, ok := resolveFixPositions(content, fix)
		if !ok {
			t.Fatal("resolveFixPositions returned ok=false")
		}
		if newText != "replaced-a\nreplaced-b\n" {
			t.Errorf("LF document should leave Replacement intact, got %q", newText)
		}
	})

	t.Run("single-line CRLF replacement with no newline is untouched", func(t *testing.T) {
		content := "line1\r\nline2\r\n"
		fix := &scanner.Fix{
			ByteMode:    true,
			StartByte:   0,
			EndByte:     5,
			Replacement: "LINE1",
		}
		_, _, newText, ok := resolveFixPositions(content, fix)
		if !ok {
			t.Fatal("resolveFixPositions returned ok=false")
		}
		if newText != "LINE1" {
			t.Errorf("single-line replacement should be unchanged, got %q", newText)
		}
	})
}

// TestFormattingPreservesCRLFInTextEdit drives the handleFormatting path
// end-to-end with a CRLF document and verifies the emitted TextEdit.newText
// uses CRLF terminators, matching the document convention.
func TestFormattingPreservesCRLFInTextEdit(t *testing.T) {
	// Build a server with a hand-crafted document that has a CRLF body and
	// a pre-populated fixable finding whose Replacement is bare LF.
	server := &Server{
		docs: make(map[string]*Document),
	}
	uri := "file:///tmp/test/CRLF.kt"
	content := "package com.example\r\n\r\nval x = 1\r\n"
	findings := []scanner.Finding{
		{
			File:     "/tmp/test/CRLF.kt",
			Line:     1,
			Col:      0,
			RuleSet:  "style",
			Rule:     "CRLFLineFix",
			Severity: "warning",
			Message:  "force LF replacement",
			Fix: &scanner.Fix{
				ByteMode:    false,
				StartLine:   1,
				EndLine:     1,
				Replacement: "package com.example.renamed\n",
			},
		},
	}
	columns := scanner.CollectFindings(findings)
	server.docs[uri] = &Document{
		URI:      uri,
		Content:  []byte(content),
		Findings: columns,
	}

	params := DocumentFormattingParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Options:      FormattingOptions{TabSize: 4, InsertSpaces: true},
	}
	paramsBytes, _ := json.Marshal(params)
	req := &Request{JSONRPC: "2.0", ID: float64(10), Method: "textDocument/formatting", Params: paramsBytes}

	var output bytes.Buffer
	server.writer = &output
	server.handleFormatting(req)

	msgs, _ := collectMessages(output.Bytes())
	if len(msgs) != 1 {
		t.Fatalf("expected 1 response, got %d", len(msgs))
	}
	var resp Response
	if err := json.Unmarshal(msgs[0], &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}
	resultBytes, _ := json.Marshal(resp.Result)
	var edits []TextEdit
	if err := json.Unmarshal(resultBytes, &edits); err != nil {
		t.Fatalf("unmarshal edits: %v", err)
	}
	if len(edits) != 1 {
		t.Fatalf("expected 1 edit, got %d", len(edits))
	}
	if !strings.Contains(edits[0].NewText, "\r\n") {
		t.Errorf("expected CRLF in TextEdit.newText for CRLF document, got %q", edits[0].NewText)
	}
	for i := 0; i < len(edits[0].NewText); i++ {
		if edits[0].NewText[i] == '\n' && (i == 0 || edits[0].NewText[i-1] != '\r') {
			t.Errorf("bare LF at offset %d in TextEdit.newText: %q", i, edits[0].NewText)
			break
		}
	}
}
