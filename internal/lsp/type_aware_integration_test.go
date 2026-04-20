package lsp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLSPTypeAwareRuleIntegration exercises the LSP server entry point
// end-to-end and asserts that a type-aware rule fires. The payload —
// a data class with a `val` property typed as `MutableList<Int>` — only
// produces a DataClassShouldBeImmutable finding when ctx.Resolver is
// non-nil (see DataClassShouldBeImmutableRule: the `val`-with-mutable-
// type branch is gated on `ctx.Resolver != nil`). So if the LSP ever
// regresses to passing a nil resolver into pipeline.NewSingleFileAnalyzer,
// this test fails — which is the regression gate #297 asks for.
func TestLSPTypeAwareRuleIntegration(t *testing.T) {
	tmpDir := t.TempDir()

	// Activate DataClassShouldBeImmutable (default-inactive) so it runs
	// through the LSP dispatcher during didOpen.
	configContent := `style:
  DataClassShouldBeImmutable:
    active: true
`
	if err := os.WriteFile(filepath.Join(tmpDir, "krit.yml"), []byte(configContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	kotlinPath := filepath.Join(tmpDir, "Foo.kt")
	kotlinContent := "package com.example\n\ndata class Foo(val items: MutableList<Int>)\n"
	if err := os.WriteFile(kotlinPath, []byte(kotlinContent), 0644); err != nil {
		t.Fatalf("write kotlin: %v", err)
	}

	rootURI := "file://" + tmpDir
	docURI := "file://" + kotlinPath

	initReq := buildRequest(1, "initialize", map[string]interface{}{
		"processId":    1234,
		"rootUri":      rootURI,
		"capabilities": map[string]interface{}{},
	})
	initializedNotif := buildRequest(nil, "initialized", map[string]interface{}{})
	didOpenReq := buildRequest(nil, "textDocument/didOpen", map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri":        docURI,
			"languageId": "kotlin",
			"version":    1,
			"text":       kotlinContent,
		},
	})

	var input bytes.Buffer
	input.Write(initReq)
	input.Write(initializedNotif)
	input.Write(didOpenReq)

	var output bytes.Buffer
	server := NewServer(bufio.NewReader(&input), &output)
	server.Run()

	// Sanity: resolver must be wired; a nil resolver would silently
	// degrade every NeedsResolver rule to its fallback path.
	if server.resolver == nil {
		t.Fatal("expected LSP server to wire up a type resolver")
	}

	msgs, err := collectMessages(output.Bytes())
	if err != nil {
		t.Fatalf("collectMessages: %v", err)
	}

	// Walk all notifications, find the publishDiagnostics for our URI,
	// and assert a DataClassShouldBeImmutable diagnostic is present.
	targetCode := "style/DataClassShouldBeImmutable"
	found := false
	var seenCodes []string
	for _, raw := range msgs {
		var notif Notification
		if err := json.Unmarshal(raw, &notif); err != nil {
			continue
		}
		if notif.Method != "textDocument/publishDiagnostics" {
			continue
		}
		paramsBytes, _ := json.Marshal(notif.Params)
		var params PublishDiagnosticsParams
		if err := json.Unmarshal(paramsBytes, &params); err != nil {
			t.Fatalf("unmarshal publishDiagnostics params: %v", err)
		}
		if params.URI != docURI {
			continue
		}
		for _, d := range params.Diagnostics {
			seenCodes = append(seenCodes, d.Code)
			if d.Code == targetCode {
				if !strings.Contains(d.Message, "mutable") {
					t.Errorf("unexpected message for %s: %q", targetCode, d.Message)
				}
				found = true
			}
		}
	}

	if !found {
		t.Fatalf("expected diagnostic with code %q; got %v", targetCode, seenCodes)
	}
}
