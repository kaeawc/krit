package lsp

import (
	"encoding/json"
	"strings"
	"testing"
)

// These tests pin down the JSON wire shape and LSP-spec constants. Editors
// rely on exact field names ("rangeLength", "selectionRange", numeric
// SymbolKind values per spec); a silent rename would break them.

func TestPosition_JSONShape(t *testing.T) {
	p := Position{Line: 4, Character: 12}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	want := `{"line":4,"character":12}`
	if string(data) != want {
		t.Fatalf("Position JSON = %s, want %s", string(data), want)
	}
}

func TestRange_RoundTrip(t *testing.T) {
	in := Range{
		Start: Position{Line: 1, Character: 0},
		End:   Position{Line: 1, Character: 10},
	}
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var back Range
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if back != in {
		t.Errorf("Range round-trip mismatch: got %+v, want %+v", back, in)
	}
}

func TestDiagnostic_OmitsOptionalWhenEmpty(t *testing.T) {
	d := Diagnostic{
		Range:   Range{Start: Position{}, End: Position{}},
		Message: "msg",
	}
	data, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	got := string(data)
	for _, banned := range []string{`"severity"`, `"code"`, `"source"`} {
		if strings.Contains(got, banned) {
			t.Errorf("empty %s should be omitted, got %s", banned, got)
		}
	}
	if !strings.Contains(got, `"message":"msg"`) {
		t.Errorf("message must always be present, got %s", got)
	}
}

func TestServerCapabilities_OmitsZeroProviders(t *testing.T) {
	c := ServerCapabilities{HoverProvider: false, CodeActionProvider: false}
	data, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	got := string(data)
	if strings.Contains(got, "hoverProvider") || strings.Contains(got, "codeActionProvider") {
		t.Errorf("false-valued bool providers should be omitted, got %s", got)
	}
}

func TestServerCapabilities_IncludesTrueProviders(t *testing.T) {
	c := ServerCapabilities{HoverProvider: true, DefinitionProvider: true}
	data, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	got := string(data)
	for _, want := range []string{`"hoverProvider":true`, `"definitionProvider":true`} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %s in JSON, got %s", want, got)
		}
	}
}

func TestTextDocumentSyncOptions_AlwaysIncludesOpenCloseAndChange(t *testing.T) {
	// Unlike most server capability fields, OpenClose and Change must
	// always appear so the client knows whether to send didOpen/didClose
	// and which sync kind to use.
	o := TextDocumentSyncOptions{OpenClose: false, Change: 0}
	data, err := json.Marshal(o)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	got := string(data)
	if !strings.Contains(got, `"openClose":false`) || !strings.Contains(got, `"change":0`) {
		t.Errorf("openClose and change must always be present, got %s", got)
	}
}

func TestInitializeParams_NullProcessID(t *testing.T) {
	// LSP spec: processId may be null (parentless server). The Go shape
	// uses *int so the field can be null on the wire.
	data := []byte(`{"processId":null,"capabilities":{}}`)
	var p InitializeParams
	if err := json.Unmarshal(data, &p); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if p.ProcessID != nil {
		t.Errorf("ProcessID should be nil for null processId, got %v", *p.ProcessID)
	}
}

func TestInitializeParams_ProcessID(t *testing.T) {
	data := []byte(`{"processId":1234,"capabilities":{}}`)
	var p InitializeParams
	if err := json.Unmarshal(data, &p); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if p.ProcessID == nil || *p.ProcessID != 1234 {
		t.Errorf("ProcessID = %v, want 1234", p.ProcessID)
	}
}

func TestInitOptions_OmitsOptional(t *testing.T) {
	o := InitOptions{}
	data, err := json.Marshal(o)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if string(data) != `{}` {
		t.Errorf("empty InitOptions should marshal to {}, got %s", string(data))
	}
}

func TestInitOptions_BoolPointerFields(t *testing.T) {
	// IndexOnInitialize and UseOracleDaemon use *bool so the client can
	// distinguish "not set" from "explicitly false".
	tru := true
	o := InitOptions{IndexOnInitialize: &tru}
	data, err := json.Marshal(o)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.Contains(string(data), `"indexOnInitialize":true`) {
		t.Errorf("expected indexOnInitialize:true, got %s", string(data))
	}

	in := []byte(`{"indexOnInitialize":false}`)
	var back InitOptions
	if err := json.Unmarshal(in, &back); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if back.IndexOnInitialize == nil || *back.IndexOnInitialize {
		t.Errorf("IndexOnInitialize = %v, want explicit false", back.IndexOnInitialize)
	}
}

func TestSymbolKindConstantsMatchLSPSpec(t *testing.T) {
	// LSP spec values: Class=5, Property=7, Function=12. If these drift,
	// every editor renders the wrong icon.
	cases := []struct {
		name string
		got  int
		want int
	}{
		{"SymbolKindClass", SymbolKindClass, 5},
		{"SymbolKindProperty", SymbolKindProperty, 7},
		{"SymbolKindFunction", SymbolKindFunction, 12},
	}
	for _, tc := range cases {
		if tc.got != tc.want {
			t.Errorf("%s = %d, LSP spec value is %d", tc.name, tc.got, tc.want)
		}
	}
}

func TestMessageTypeConstantsMatchLSPSpec(t *testing.T) {
	cases := []struct {
		name string
		got  int
		want int
	}{
		{"Error", MessageTypeError, 1},
		{"Warning", MessageTypeWarning, 2},
		{"Info", MessageTypeInfo, 3},
		{"Log", MessageTypeLog, 4},
	}
	for _, tc := range cases {
		if tc.got != tc.want {
			t.Errorf("MessageType%s = %d, LSP spec value is %d", tc.name, tc.got, tc.want)
		}
	}
}

func TestCompletionItemKindConstantsMatchLSPSpec(t *testing.T) {
	cases := []struct {
		name string
		got  int
		want int
	}{
		{"Text", CompletionKindText, 1},
		{"Class", CompletionKindClass, 7},
		{"Keyword", CompletionKindKeyword, 14},
	}
	for _, tc := range cases {
		if tc.got != tc.want {
			t.Errorf("CompletionKind%s = %d, LSP spec value is %d", tc.name, tc.got, tc.want)
		}
	}
}

func TestCodeAction_OmitsAbsentFields(t *testing.T) {
	a := CodeAction{Title: "Fix", Kind: "quickfix"}
	data, err := json.Marshal(a)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	got := string(data)
	for _, banned := range []string{"description", "diagnostics", "edit", "command", "data"} {
		if strings.Contains(got, banned) {
			t.Errorf("empty %s should be omitted, got %s", banned, got)
		}
	}
}

func TestWorkspaceEdit_ChangesKeyAlwaysPresent(t *testing.T) {
	// "changes" is the top-level map clients look at; even when empty it
	// helps clients distinguish "no changes" from a missing field.
	e := WorkspaceEdit{Changes: map[string][]TextEdit{}}
	data, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.Contains(string(data), `"changes":{}`) {
		t.Errorf("expected changes:{}, got %s", string(data))
	}
}

func TestPublishDiagnosticsParams_RoundTrip(t *testing.T) {
	in := PublishDiagnosticsParams{
		URI: "file:///a/b.kt",
		Diagnostics: []Diagnostic{
			{
				Range:    Range{Start: Position{Line: 0, Character: 0}, End: Position{Line: 0, Character: 5}},
				Severity: 1,
				Code:     "X.Y",
				Source:   "krit",
				Message:  "broken",
			},
		},
	}
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var back PublishDiagnosticsParams
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if back.URI != in.URI {
		t.Errorf("URI lost in round-trip")
	}
	if len(back.Diagnostics) != 1 || back.Diagnostics[0].Code != "X.Y" {
		t.Errorf("Diagnostic lost in round-trip: %+v", back.Diagnostics)
	}
}

func TestDocumentSymbol_SelectionRangeFieldName(t *testing.T) {
	// LSP spec uses "selectionRange" (camelCase, single word). A typo here
	// silently breaks symbol navigation in editors.
	d := DocumentSymbol{
		Name:           "Foo",
		Kind:           SymbolKindClass,
		Range:          Range{},
		SelectionRange: Range{},
	}
	data, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.Contains(string(data), `"selectionRange"`) {
		t.Errorf("selectionRange field name missing, got %s", string(data))
	}
}

func TestHover_OptionalRange(t *testing.T) {
	h := Hover{Contents: MarkupContent{Kind: "markdown", Value: "hi"}}
	data, err := json.Marshal(h)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if strings.Contains(string(data), `"range"`) {
		t.Errorf("nil range should be omitted, got %s", string(data))
	}
}
