package lsp

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/oracle"
)

// buildOracleIndexFixture returns a tiny multi-file oracle index. com.example.A
// lives in a.kt with one member foo and is called from b.kt at 5:9.
func buildOracleIndexFixture() *oracle.Index {
	return oracle.BuildIndex(&oracle.Data{
		Version: 1,
		Files: map[string]*oracle.File{
			"/tmp/proj/a.kt": {
				Package: "com.example",
				Declarations: []*oracle.Class{
					{
						FQN:  "com.example.A",
						Kind: "class",
						Members: []*oracle.Member{
							{Name: "foo", Kind: "function", ReturnType: "kotlin.Unit"},
						},
					},
				},
			},
			"/tmp/proj/b.kt": {
				Package: "com.example",
				Expressions: map[string]*oracle.ExpressionType{
					"5:9":  {Type: "kotlin.Unit", CallTarget: "com.example.A.foo"},
					"7:11": {Type: "kotlin.Unit", CallTarget: "com.example.A.foo"},
				},
			},
		},
	})
}

func TestDefinitionUsesOracleIndexAcrossFiles(t *testing.T) {
	server := &Server{docs: make(map[string]*Document)}
	server.SetOracleIndex(buildOracleIndexFixture())

	uri := "file:///tmp/proj/b.kt"
	server.docs[uri] = &Document{URI: uri, Content: []byte("package com.example\n\nfun call() {\n    A().foo()\n}\n")}

	params := DefinitionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 3, Character: 4},
	}
	paramsBytes, _ := json.Marshal(params)
	req := &Request{JSONRPC: "2.0", ID: float64(1), Method: "textDocument/definition", Params: paramsBytes}

	var output bytes.Buffer
	server.writer = &output
	server.handleDefinition(req)

	msgs, _ := collectMessages(output.Bytes())
	var resp Response
	if err := json.Unmarshal(msgs[0], &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	resultBytes, _ := json.Marshal(resp.Result)
	var loc Location
	if err := json.Unmarshal(resultBytes, &loc); err != nil {
		t.Fatalf("unmarshal location: %v", err)
	}
	if loc.URI != "file:///tmp/proj/a.kt" {
		t.Errorf("URI = %q, want file:///tmp/proj/a.kt", loc.URI)
	}
}

func TestDefinitionReturnsJARURIForDependencyDeclaration(t *testing.T) {
	server := &Server{docs: make(map[string]*Document)}
	jarPath := "/tmp/kotlinx-coroutines-core-1.7.3.jar"
	server.SetOracleIndex(oracle.BuildIndex(&oracle.Data{
		Version: 1,
		Dependencies: map[string]*oracle.Class{
			"kotlinx.coroutines.CoroutineScope": {
				FQN:     "kotlinx.coroutines.CoroutineScope",
				Kind:    "interface",
				JARPath: jarPath,
			},
		},
	}))

	uri := "file:///tmp/proj/use.kt"
	server.docs[uri] = &Document{URI: uri, Content: []byte("package test\n\nfun use(scope: CoroutineScope) = scope\n")}

	params := DefinitionParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 2, Character: 16},
	}
	paramsBytes, _ := json.Marshal(params)
	req := &Request{JSONRPC: "2.0", ID: float64(1), Method: "textDocument/definition", Params: paramsBytes}

	var output bytes.Buffer
	server.writer = &output
	server.handleDefinition(req)

	msgs, _ := collectMessages(output.Bytes())
	var resp Response
	if err := json.Unmarshal(msgs[0], &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	resultBytes, _ := json.Marshal(resp.Result)
	var loc Location
	if err := json.Unmarshal(resultBytes, &loc); err != nil {
		t.Fatalf("unmarshal location: %v", err)
	}
	ref, err := ParseJARURI(loc.URI)
	if err != nil {
		t.Fatalf("definition URI is not krit-jar: %q: %v", loc.URI, err)
	}
	if ref.JARPath != jarPath || ref.FQN != "kotlinx.coroutines.CoroutineScope" {
		t.Fatalf("jar ref = %+v", ref)
	}
}

func TestReferencesUsesOracleIndexAcrossFiles(t *testing.T) {
	server := &Server{docs: make(map[string]*Document)}
	server.SetOracleIndex(buildOracleIndexFixture())

	uri := "file:///tmp/proj/a.kt"
	server.docs[uri] = &Document{URI: uri, Content: []byte("package com.example\n\nclass A {\n    fun foo() {}\n}\n")}

	params := ReferenceParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 3, Character: 8},
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

	// Expect declaration in a.kt + 2 call sites in b.kt = 3.
	if len(locs) != 3 {
		t.Fatalf("expected 3 references, got %d (%+v)", len(locs), locs)
	}
	bCount := 0
	for _, l := range locs {
		if l.URI == "file:///tmp/proj/b.kt" {
			bCount++
		}
	}
	if bCount != 2 {
		t.Errorf("expected 2 cross-file refs in b.kt, got %d", bCount)
	}
}

func TestReferencesExcludeDeclarationOracle(t *testing.T) {
	server := &Server{docs: make(map[string]*Document)}
	server.SetOracleIndex(buildOracleIndexFixture())

	uri := "file:///tmp/proj/a.kt"
	server.docs[uri] = &Document{URI: uri, Content: []byte("package com.example\n\nclass A {\n    fun foo() {}\n}\n")}

	params := ReferenceParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 3, Character: 8},
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

	if len(locs) != 2 {
		t.Errorf("expected 2 refs without declaration, got %d", len(locs))
	}
}

func TestRenameUsesOracleIndexAcrossFiles(t *testing.T) {
	server := &Server{docs: make(map[string]*Document)}
	server.SetOracleIndex(buildOracleIndexFixture())

	uri := "file:///tmp/proj/a.kt"
	server.docs[uri] = &Document{URI: uri, Content: []byte("package com.example\n\nclass A {\n    fun foo() {}\n}\n")}

	params := RenameParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 3, Character: 8},
		NewName:      "bar",
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
	var ws WorkspaceEdit
	json.Unmarshal(resultBytes, &ws)

	if _, ok := ws.Changes["file:///tmp/proj/b.kt"]; !ok {
		t.Errorf("expected workspace edit in b.kt, got keys: %v", mapKeys(ws.Changes))
	}
	for _, edits := range ws.Changes {
		for _, edit := range edits {
			if edit.NewText != "bar" {
				t.Errorf("edit NewText = %q, want bar", edit.NewText)
			}
		}
	}
}

func TestRenameRejectsInvalidIdentifier(t *testing.T) {
	server := &Server{docs: make(map[string]*Document)}

	uri := "file:///tmp/proj/Test.kt"
	server.docs[uri] = &Document{URI: uri, Content: []byte("fun greet() {}\n")}

	params := RenameParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 0, Character: 4},
		NewName:      "1bad",
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
		t.Fatal("expected error for invalid Kotlin identifier")
	}
}

func TestRenameWithoutOracleEmitsShowMessage(t *testing.T) {
	server := &Server{docs: make(map[string]*Document)}

	uri := "file:///tmp/proj/Test.kt"
	server.docs[uri] = &Document{URI: uri, Content: []byte("fun greet() {\n    greet()\n}\n")}

	params := RenameParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 0, Character: 4},
		NewName:      "hello",
	}
	paramsBytes, _ := json.Marshal(params)
	req := &Request{JSONRPC: "2.0", ID: float64(1), Method: "textDocument/rename", Params: paramsBytes}

	var output bytes.Buffer
	server.writer = &output
	server.handleRename(req)

	if !bytes.Contains(output.Bytes(), []byte("window/showMessage")) {
		t.Error("expected window/showMessage warning when oracle is unavailable")
	}
	if !bytes.Contains(output.Bytes(), []byte("workspace index unavailable")) {
		t.Error("expected fallback message body to mention workspace index")
	}
}

func TestHoverIncludesOracleSymbolInfo(t *testing.T) {
	server := &Server{docs: make(map[string]*Document)}
	server.SetOracleIndex(buildOracleIndexFixture())

	uri := "file:///tmp/proj/b.kt"
	server.docs[uri] = &Document{URI: uri, Content: []byte("package com.example\n\nfun call() {\n    A()\n}\n")}

	params := HoverParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
		Position:     Position{Line: 3, Character: 4},
	}
	paramsBytes, _ := json.Marshal(params)
	req := &Request{JSONRPC: "2.0", ID: float64(1), Method: "textDocument/hover", Params: paramsBytes}

	var output bytes.Buffer
	server.writer = &output
	server.handleHover(req)

	msgs, _ := collectMessages(output.Bytes())
	var resp Response
	json.Unmarshal(msgs[0], &resp)
	if resp.Result == nil {
		t.Fatal("expected hover result, got nil")
	}
	resultBytes, _ := json.Marshal(resp.Result)
	var hover Hover
	json.Unmarshal(resultBytes, &hover)
	if !strings.Contains(hover.Contents.Value, "com.example.A") {
		t.Errorf("hover missing FQN: %q", hover.Contents.Value)
	}
	if !strings.Contains(hover.Contents.Value, "class") {
		t.Errorf("hover missing kind: %q", hover.Contents.Value)
	}
}

func TestPathToURI(t *testing.T) {
	cases := map[string]string{
		"/tmp/foo.kt":          "file:///tmp/foo.kt",
		"file:///tmp/foo.kt":   "file:///tmp/foo.kt",
		"relative/path.kt":     "file:///relative/path.kt",
		"https://example.com/": "https://example.com/",
	}
	for in, want := range cases {
		if got := pathToURI(in); got != want {
			t.Errorf("pathToURI(%q) = %q, want %q", in, got, want)
		}
	}
}

func mapKeys(m map[string][]TextEdit) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
