package mcp

import (
	"encoding/json"
	"strings"
	"testing"
)

// These tests guard the JSON wire shape of the MCP protocol types. Field
// names, omitempty behaviour, and the precise envelope of tool/resource
// responses are part of the public contract with MCP clients (Claude
// Desktop, IDE extensions); silent changes there break every consumer.

func TestInitializeResult_JSONShape(t *testing.T) {
	r := InitializeResult{
		ProtocolVersion: "2024-11-05",
		Capabilities: ServerCaps{
			Tools:     &ToolsCap{},
			Resources: &ResourcesCap{},
			Prompts:   &PromptsCap{},
		},
		ServerInfo: ServerInfo{Name: "krit-mcp", Version: "0.1.0"},
	}
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	got := string(data)
	for _, want := range []string{
		`"protocolVersion":"2024-11-05"`,
		`"tools":{}`,
		`"resources":{}`,
		`"prompts":{}`,
		`"serverInfo":{"name":"krit-mcp","version":"0.1.0"}`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("InitializeResult JSON missing %q\nfull: %s", want, got)
		}
	}
}

func TestServerCaps_OmitsNilCapabilities(t *testing.T) {
	r := InitializeResult{
		ProtocolVersion: "v1",
		Capabilities:    ServerCaps{Tools: &ToolsCap{}},
		ServerInfo:      ServerInfo{Name: "x", Version: "y"},
	}
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	got := string(data)
	if !strings.Contains(got, `"tools":{}`) {
		t.Errorf("expected tools cap to be present, got %s", got)
	}
	if strings.Contains(got, `"resources"`) {
		t.Errorf("nil Resources cap should be omitted, got %s", got)
	}
	if strings.Contains(got, `"prompts"`) {
		t.Errorf("nil Prompts cap should be omitted, got %s", got)
	}
}

func TestToolDefinition_RoundTrip(t *testing.T) {
	def := ToolDefinition{
		Name:        "analyze",
		Description: "Run static analysis",
		InputSchema: map[string]any{"type": "object"},
	}
	data, err := json.Marshal(def)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var back ToolDefinition
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if back.Name != def.Name || back.Description != def.Description {
		t.Errorf("round-trip mismatch: got %+v, want %+v", back, def)
	}
	if back.InputSchema == nil {
		t.Errorf("InputSchema lost in round-trip")
	}
}

func TestToolResult_IsErrorOmittedWhenFalse(t *testing.T) {
	ok := ToolResult{Content: []ContentBlock{{Type: "text", Text: "hi"}}}
	data, err := json.Marshal(ok)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if strings.Contains(string(data), "isError") {
		t.Errorf("isError should be omitted when false, got %s", string(data))
	}

	bad := ToolResult{Content: []ContentBlock{{Type: "text", Text: "boom"}}, IsError: true}
	data, err = json.Marshal(bad)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if !strings.Contains(string(data), `"isError":true`) {
		t.Errorf("isError should be present when true, got %s", string(data))
	}
}

func TestToolCallParams_ArgumentsRawMessageIsPreserved(t *testing.T) {
	raw := `{"path":"src/Main.kt","ruleId":"X.Y"}`
	in := []byte(`{"name":"analyze","arguments":` + raw + `}`)

	var p ToolCallParams
	if err := json.Unmarshal(in, &p); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if p.Name != "analyze" {
		t.Errorf("Name = %q, want analyze", p.Name)
	}
	// Arguments are passed through as RawMessage so each tool can decode
	// into its own schema. Round-trip should preserve the exact bytes.
	var roundtrip map[string]string
	if err := json.Unmarshal(p.Arguments, &roundtrip); err != nil {
		t.Fatalf("decode arguments: %v", err)
	}
	if roundtrip["path"] != "src/Main.kt" || roundtrip["ruleId"] != "X.Y" {
		t.Errorf("arguments lost: %+v", roundtrip)
	}
}

func TestResourceDefinition_OmitsOptional(t *testing.T) {
	minimal := ResourceDefinition{URI: "krit://rules", Name: "rules"}
	data, err := json.Marshal(minimal)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	got := string(data)
	if strings.Contains(got, "description") || strings.Contains(got, "mimeType") {
		t.Errorf("optional fields should be omitted when empty, got %s", got)
	}
}

func TestResourceContent_OmitsEmptyFields(t *testing.T) {
	c := ResourceContent{URI: "krit://x"}
	data, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	got := string(data)
	if strings.Contains(got, "mimeType") || strings.Contains(got, `"text":`) {
		t.Errorf("empty optional fields should be omitted, got %s", got)
	}
}

func TestPromptGetResult_RoundTrip(t *testing.T) {
	r := PromptGetResult{
		Description: "explain a rule",
		Messages: []PromptMessage{
			{Role: "user", Content: ContentBlock{Type: "text", Text: "Why?"}},
		},
	}
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var back PromptGetResult
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if back.Description != r.Description {
		t.Errorf("description lost in round-trip")
	}
	if len(back.Messages) != 1 || back.Messages[0].Content.Text != "Why?" {
		t.Errorf("messages lost in round-trip: %+v", back.Messages)
	}
}

func TestPromptDefinition_OmitsEmptyArguments(t *testing.T) {
	d := PromptDefinition{Name: "p", Description: "d"}
	data, err := json.Marshal(d)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if strings.Contains(string(data), "arguments") {
		t.Errorf("empty arguments slice should be omitted, got %s", string(data))
	}
}

func TestClientInfo_OmittedWhenAbsent(t *testing.T) {
	p := InitializeParams{ProtocolVersion: "v1"}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if strings.Contains(string(data), "clientInfo") {
		t.Errorf("nil ClientInfo should be omitted, got %s", string(data))
	}
}
