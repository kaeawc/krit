package lsp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/oracle"
)

func TestJARContentReturnsStub(t *testing.T) {
	rootURI := "file://" + t.TempDir()
	initReq := buildRequest(1, "initialize", map[string]interface{}{
		"processId":    1234,
		"rootUri":      rootURI,
		"capabilities": map[string]interface{}{},
	})

	uri := BuildJARURI(JARRef{
		JARPath: "/no/such-coroutines-1.7.3.jar",
		FQN:     "kotlinx.coroutines.CoroutineScope",
	})
	initializedNotif := buildRequest(nil, "initialized", map[string]interface{}{})
	contentReq := buildRequest(2, "krit/jarContent", map[string]interface{}{
		"uri": uri,
	})
	exitReq := buildRequest(nil, "exit", nil)

	var input, output bytes.Buffer
	input.Write(initReq)
	input.Write(initializedNotif)
	input.Write(contentReq)
	input.Write(exitReq)

	oldExit := exitFunc
	exitFunc = func(int) {}
	defer func() { exitFunc = oldExit }()

	server := NewServer(bufio.NewReader(&input), &output)
	server.SetJARLookup(func(fqn string) *oracle.Class {
		if fqn == "kotlinx.coroutines.CoroutineScope" {
			return &oracle.Class{FQN: fqn, Kind: "interface"}
		}
		return nil
	})
	server.Run()
	server.waitForIndexShutdown(2 * time.Second)

	msgs, err := collectMessages(output.Bytes())
	if err != nil {
		t.Fatalf("collect: %v", err)
	}

	var got *Response
	for _, m := range msgs {
		var resp Response
		if err := json.Unmarshal(m, &resp); err != nil {
			continue
		}
		if id, ok := resp.ID.(float64); ok && int(id) == 2 {
			got = &resp
			break
		}
	}
	if got == nil {
		t.Fatalf("no response for jarContent request; messages: %s", output.String())
	}
	if got.Error != nil {
		t.Fatalf("unexpected error: %+v", got.Error)
	}

	resultBytes, err := json.Marshal(got.Result)
	if err != nil {
		t.Fatal(err)
	}
	var result JARContentResult
	if err := json.Unmarshal(resultBytes, &result); err != nil {
		t.Fatal(err)
	}
	if result.Language != "kotlin" {
		t.Errorf("language = %q, want kotlin", result.Language)
	}
	if result.URI != uri {
		t.Errorf("uri mismatch: got %q want %q", result.URI, uri)
	}
	if !strings.Contains(result.Text, "interface CoroutineScope") {
		t.Errorf("expected stub to contain interface CoroutineScope, got:\n%s", result.Text)
	}
}

func TestDidOpenJARURIPopulatesVirtualDocument(t *testing.T) {
	server := NewServer(nil, &bytes.Buffer{})
	server.SetJARLookup(func(fqn string) *oracle.Class {
		if fqn == "kotlinx.coroutines.CoroutineScope" {
			return &oracle.Class{FQN: fqn, Kind: "interface"}
		}
		return nil
	})
	uri := BuildJARURI(JARRef{
		JARPath: "/no/such-coroutines-1.7.3.jar",
		FQN:     "kotlinx.coroutines.CoroutineScope",
	})
	params := DidOpenTextDocumentParams{TextDocument: TextDocumentItem{
		URI:        uri,
		LanguageID: "kotlin",
		Version:    1,
		Text:       "",
	}}
	raw, _ := json.Marshal(params)

	server.handleDidOpen(&Request{JSONRPC: "2.0", Method: "textDocument/didOpen", Params: raw})

	server.docsMu.Lock()
	doc := server.docs[uri]
	server.docsMu.Unlock()
	if doc == nil {
		t.Fatal("jar document was not opened")
	}
	if !strings.Contains(string(doc.Content), "interface CoroutineScope") {
		t.Fatalf("jar document content did not come from decompile cache:\n%s", doc.Content)
	}
}

func TestJARContentRejectsNonJARURI(t *testing.T) {
	rootURI := "file://" + t.TempDir()
	initReq := buildRequest(1, "initialize", map[string]interface{}{
		"processId":    1234,
		"rootUri":      rootURI,
		"capabilities": map[string]interface{}{},
	})
	initializedNotif := buildRequest(nil, "initialized", map[string]interface{}{})
	badReq := buildRequest(2, "krit/jarContent", map[string]interface{}{
		"uri": "file:///tmp/Foo.kt",
	})
	exitReq := buildRequest(nil, "exit", nil)

	var input, output bytes.Buffer
	input.Write(initReq)
	input.Write(initializedNotif)
	input.Write(badReq)
	input.Write(exitReq)

	oldExit := exitFunc
	exitFunc = func(int) {}
	defer func() { exitFunc = oldExit }()

	server := NewServer(bufio.NewReader(&input), &output)
	server.Run()
	server.waitForIndexShutdown(2 * time.Second)

	msgs, _ := collectMessages(output.Bytes())
	for _, m := range msgs {
		var resp Response
		if err := json.Unmarshal(m, &resp); err != nil {
			continue
		}
		if id, ok := resp.ID.(float64); ok && int(id) == 2 {
			if resp.Error == nil {
				t.Fatalf("expected error response for file:// URI, got result %+v", resp.Result)
			}
			return
		}
	}
	t.Fatal("no response for id=2")
}
