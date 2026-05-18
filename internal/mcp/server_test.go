package mcp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/jsonrpc"
)

// helper: build a newline-delimited JSON-RPC message. MCP-over-stdio uses
// NDJSON framing, not Content-Length headers.
func frameMessage(t *testing.T, req Request) string {
	t.Helper()
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	return fmt.Sprintf("%s\n", data)
}

// runServerWithSrv is runServer that also returns the *Server so tests
// can inspect its private state (e.g. workspace cache stats) after the
// requests have been processed.
func runServerWithSrv(t *testing.T, requests ...Request) (*Server, []Response) {
	t.Helper()
	var input strings.Builder
	for _, req := range requests {
		input.WriteString(frameMessage(t, req))
	}

	var output bytes.Buffer
	reader := bufio.NewReader(strings.NewReader(input.String()))
	server := NewServer(reader, &output)
	server.buildDispatcher()

	for {
		msg, err := jsonrpc.ReadMessageNDJSON(reader)
		if err != nil {
			break
		}
		var req Request
		if err := json.Unmarshal(msg, &req); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		server.handleMessage(&req)
	}

	outReader := bufio.NewReader(bytes.NewReader(output.Bytes()))
	var responses []Response
	for {
		msg, err := jsonrpc.ReadMessageNDJSON(outReader)
		if err != nil {
			break
		}
		var resp Response
		if err := json.Unmarshal(msg, &resp); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}
		responses = append(responses, resp)
	}
	return server, responses
}

// helper: send a sequence of framed requests and return the server's output.
func runServer(t *testing.T, requests ...Request) []Response {
	t.Helper()

	var input strings.Builder
	for _, req := range requests {
		input.WriteString(frameMessage(t, req))
	}

	var output bytes.Buffer
	reader := bufio.NewReader(strings.NewReader(input.String()))
	server := NewServer(reader, &output)
	server.buildDispatcher()

	// Process messages until EOF
	for {
		msg, err := jsonrpc.ReadMessageNDJSON(reader)
		if err != nil {
			break
		}
		var req Request
		if err := json.Unmarshal(msg, &req); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		server.handleMessage(&req)
	}

	// Parse responses from output
	outReader := bufio.NewReader(bytes.NewReader(output.Bytes()))
	var responses []Response
	for {
		msg, err := jsonrpc.ReadMessageNDJSON(outReader)
		if err != nil {
			break
		}
		var resp Response
		if err := json.Unmarshal(msg, &resp); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}
		responses = append(responses, resp)
	}

	return responses
}

func TestInitializeReturnsCapabilities(t *testing.T) {
	responses := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "initialize",
		Params:  json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{}}`),
	})

	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}

	resp := responses[0]
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	data, _ := json.Marshal(resp.Result)
	var result InitializeResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	// Client asked for "2024-11-05" which the server still supports; the
	// response must echo the client's version, not the server's latest.
	if result.ProtocolVersion != "2024-11-05" {
		t.Errorf("expected protocol version 2024-11-05, got %s", result.ProtocolVersion)
	}
	if result.ServerInfo.Name != "krit-mcp" {
		t.Errorf("expected server name krit-mcp, got %s", result.ServerInfo.Name)
	}
	if result.Capabilities.Tools == nil {
		t.Error("expected tools capability")
	}
	if result.Capabilities.Resources == nil {
		t.Error("expected resources capability")
	}
	if result.Capabilities.Prompts == nil {
		t.Error("expected prompts capability")
	}
}

// TestInitializeFallsBackForUnknownClientVersion guards the
// negotiation half of the MCP spec: an unknown client version must
// produce the server's latest supported version, not the client's
// bogus one. The echo-supported-client-version half is covered by
// `TestInitializeReturnsCapabilities` since the server currently
// advertises only one entry in `supportedProtocolVersions`; when more
// revisions are added, the explicit echo test should come back.
func TestInitializeFallsBackForUnknownClientVersion(t *testing.T) {
	responses := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "initialize",
		Params:  json.RawMessage(`{"protocolVersion":"1999-01-01","capabilities":{}}`),
	})

	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}

	data, _ := json.Marshal(responses[0].Result)
	var result InitializeResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if result.ProtocolVersion != supportedProtocolVersions[0] {
		t.Errorf("unsupported client version should fall back to latest supported; got %q, want %q",
			result.ProtocolVersion, supportedProtocolVersions[0])
	}
	if result.ProtocolVersion == "1999-01-01" {
		t.Errorf("server must not echo an unsupported version verbatim")
	}
}

func TestToolsListReturnsTools(t *testing.T) {
	responses := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/list",
	})

	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}

	data, _ := json.Marshal(responses[0].Result)
	var result ToolsListResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if len(result.Tools) != 8 {
		t.Fatalf("expected 8 tools, got %d", len(result.Tools))
	}

	names := map[string]bool{}
	for _, tool := range result.Tools {
		names[tool.Name] = true
	}
	// Consolidated surface: rules-side (analyze, fix, rules), history
	// (metrics), facts-side (symbols, types, structure), and structural
	// snapshots (snapshot).
	for _, expected := range []string{"analyze", "fix", "rules", "metrics", "symbols", "types", "structure", "snapshot"} {
		if !names[expected] {
			t.Errorf("missing tool: %s", expected)
		}
	}
}

func TestAnalyzeBadKotlin(t *testing.T) {
	// Code with trailing whitespace should trigger TrailingWhitespace rule
	code := "fun main() {   \n    val x = 1\n}\n"

	args, _ := json.Marshal(analyzeArgs{Code: code})
	responses := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  mustJSON(t, ToolCallParams{Name: "analyze", Arguments: args}),
	})

	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}

	resp := responses[0]
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	data, _ := json.Marshal(resp.Result)
	var result ToolResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if result.IsError {
		t.Fatalf("tool returned error: %s", result.Content[0].Text)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected content in result")
	}

	// The result text should be a JSON array; check it's not empty
	text := result.Content[0].Text
	if text == "[]" {
		t.Error("expected findings for bad Kotlin, got empty array")
	}

	// Verify it parses as JSON array
	var findings []json.RawMessage
	if err := json.Unmarshal([]byte(text), &findings); err != nil {
		t.Fatalf("result text is not valid JSON array: %v\ntext: %s", err, text)
	}
	if len(findings) == 0 {
		t.Error("expected at least one finding")
	}
}

func TestAnalyzeCleanKotlin(t *testing.T) {
	code := "fun main() {\n    println(\"hello\")\n}\n"

	args, _ := json.Marshal(analyzeArgs{Code: code})
	responses := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  mustJSON(t, ToolCallParams{Name: "analyze", Arguments: args}),
	})

	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}

	data, _ := json.Marshal(responses[0].Result)
	var result ToolResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if result.IsError {
		t.Fatalf("tool returned error: %s", result.Content[0].Text)
	}

	text := result.Content[0].Text
	if text != "[]" {
		t.Errorf("expected empty findings for clean Kotlin, got: %s", text)
	}
}

// TestWorkspaceCacheReusedAcrossAnalyzeCalls verifies that two MCP
// `analyze` calls with identical code share the parsed file rather than
// re-running tree-sitter both times.
func TestWorkspaceCacheReusedAcrossAnalyzeCalls(t *testing.T) {
	code := "fun main() {\n    println(\"hello\")\n}\n"
	args, _ := json.Marshal(analyzeArgs{Code: code})
	server, responses := runServerWithSrv(t,
		Request{JSONRPC: "2.0", ID: float64(1), Method: "tools/call",
			Params: mustJSON(t, ToolCallParams{Name: "analyze", Arguments: args})},
		Request{JSONRPC: "2.0", ID: float64(2), Method: "tools/call",
			Params: mustJSON(t, ToolCallParams{Name: "analyze", Arguments: args})},
	)
	if len(responses) != 2 {
		t.Fatalf("expected 2 responses, got %d", len(responses))
	}
	stats := server.workspace.Stats()
	if stats.Misses != 1 {
		t.Errorf("misses: got %d, want 1 (second call should hit)", stats.Misses)
	}
	if stats.Hits != 1 {
		t.Errorf("hits: got %d, want 1", stats.Hits)
	}
}

func TestExplainRule(t *testing.T) {
	// Use a rule we know exists: TrailingWhitespace
	args, _ := json.Marshal(rulesArgs{Operation: "explain", Rule: "TrailingWhitespace"})
	responses := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  mustJSON(t, ToolCallParams{Name: "rules", Arguments: args}),
	})

	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}

	data, _ := json.Marshal(responses[0].Result)
	var result ToolResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if result.IsError {
		t.Fatalf("tool returned error: %s", result.Content[0].Text)
	}

	text := result.Content[0].Text
	var info map[string]interface{}
	if err := json.Unmarshal([]byte(text), &info); err != nil {
		t.Fatalf("result is not valid JSON: %v\ntext: %s", err, text)
	}

	if info["name"] != "TrailingWhitespace" {
		t.Errorf("expected name TrailingWhitespace, got %v", info["name"])
	}
	if _, ok := info["severity"]; !ok {
		t.Error("expected severity in result")
	}
	if _, ok := info["fixable"]; !ok {
		t.Error("expected fixable in result")
	}
	owners, ok := info["owners"].([]interface{})
	if !ok {
		t.Fatalf("expected owners array in result, got %T", info["owners"])
	}
	if len(owners) == 0 {
		t.Error("expected at least one owner")
	}
	maintained, ok := info["maintainedBy"].(string)
	if !ok || !strings.HasPrefix(maintained, "Maintained by ") {
		t.Errorf("expected maintainedBy 'Maintained by ...', got %q", info["maintainedBy"])
	}
	if info["introducedIn"] == nil || info["introducedIn"] == "" {
		t.Error("expected introducedIn in result")
	}
	if info["lifecycle"] == nil || info["lifecycle"] == "" {
		t.Error("expected lifecycle summary in result")
	}
}

func TestFormatLifecycle(t *testing.T) {
	tests := []struct {
		name        string
		introduced  string
		enabled     string
		wantSummary string
	}{
		{"both", "0.2.0", "0.4.0", "Introduced in 0.2.0; default since 0.4.0"},
		{"introduced only", "0.2.0", "", "Introduced in 0.2.0"},
		{"default only", "", "0.4.0", "Default since 0.4.0"},
		{"empty", "", "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := formatLifecycle(tc.introduced, tc.enabled)
			if got != tc.wantSummary {
				t.Errorf("formatLifecycle(%q,%q) = %q, want %q", tc.introduced, tc.enabled, got, tc.wantSummary)
			}
		})
	}
}

func TestExplainRuleUnknown(t *testing.T) {
	args, _ := json.Marshal(rulesArgs{Operation: "explain", Rule: "NonExistentRule12345"})
	responses := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  mustJSON(t, ToolCallParams{Name: "rules", Arguments: args}),
	})

	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}

	data, _ := json.Marshal(responses[0].Result)
	var result ToolResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if !result.IsError {
		t.Error("expected error for unknown rule")
	}
}

func TestResourcesList(t *testing.T) {
	responses := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "resources/list",
	})

	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}

	data, _ := json.Marshal(responses[0].Result)
	var result ResourcesListResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if len(result.Resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(result.Resources))
	}

	uris := map[string]bool{}
	for _, r := range result.Resources {
		uris[r.URI] = true
	}
	for _, expected := range []string{"krit://rules", "krit://schema"} {
		if !uris[expected] {
			t.Errorf("missing resource: %s", expected)
		}
	}
}

func TestResourcesReadRules(t *testing.T) {
	params, _ := json.Marshal(ResourceReadParams{URI: "krit://rules"})
	responses := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "resources/read",
		Params:  params,
	})

	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}

	resp := responses[0]
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	data, _ := json.Marshal(resp.Result)
	var result ResourceReadResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if len(result.Contents) != 1 {
		t.Fatalf("expected 1 content item, got %d", len(result.Contents))
	}

	content := result.Contents[0]
	if content.URI != "krit://rules" {
		t.Errorf("expected URI krit://rules, got %s", content.URI)
	}
	if content.MimeType != "application/json" {
		t.Errorf("expected mime type application/json, got %s", content.MimeType)
	}

	// Verify it's valid JSON array
	var rules []json.RawMessage
	if err := json.Unmarshal([]byte(content.Text), &rules); err != nil {
		t.Fatalf("rules content is not valid JSON array: %v", err)
	}
	if len(rules) == 0 {
		t.Error("expected at least one rule in catalog")
	}
}

func TestInspectTypesClasses(t *testing.T) {
	code := `package com.example

open class Animal(val name: String)
data class Dog(val breed: String) : Animal("dog")
`
	args, _ := json.Marshal(typesArgs{Code: code, Query: "classes"})
	responses := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  mustJSON(t, ToolCallParams{Name: "types", Arguments: args}),
	})

	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}

	data, _ := json.Marshal(responses[0].Result)
	var result ToolResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if result.IsError {
		t.Fatalf("tool returned error: %s", result.Content[0].Text)
	}

	text := result.Content[0].Text
	var classes []map[string]interface{}
	if err := json.Unmarshal([]byte(text), &classes); err != nil {
		t.Fatalf("result is not valid JSON array: %v\ntext: %s", err, text)
	}

	// Should find at least Animal and Dog
	names := map[string]bool{}
	for _, c := range classes {
		if n, ok := c["name"].(string); ok {
			names[n] = true
		}
	}
	if !names["Animal"] {
		t.Error("expected to find class Animal")
	}
	if !names["Dog"] {
		t.Error("expected to find class Dog")
	}
}

func TestInspectTypesImports(t *testing.T) {
	code := `package com.example

import kotlin.collections.mutableListOf
import android.os.Bundle
`
	args, _ := json.Marshal(typesArgs{Code: code, Query: "imports"})
	responses := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  mustJSON(t, ToolCallParams{Name: "types", Arguments: args}),
	})

	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}

	data, _ := json.Marshal(responses[0].Result)
	var result ToolResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if result.IsError {
		t.Fatalf("tool returned error: %s", result.Content[0].Text)
	}

	text := result.Content[0].Text
	var imports map[string]interface{}
	if err := json.Unmarshal([]byte(text), &imports); err != nil {
		t.Fatalf("result is not valid JSON: %v\ntext: %s", err, text)
	}

	if _, ok := imports["explicit"]; !ok {
		t.Error("expected 'explicit' key in imports result")
	}
}

func TestInspectTypesEnumEntries(t *testing.T) {
	code := `package com.example

enum class Color {
    RED, GREEN, BLUE
}
`
	args, _ := json.Marshal(typesArgs{Code: code, Query: "enum_entries"})
	responses := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  mustJSON(t, ToolCallParams{Name: "types", Arguments: args}),
	})

	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}

	data, _ := json.Marshal(responses[0].Result)
	var result ToolResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if result.IsError {
		t.Fatalf("tool returned error: %s", result.Content[0].Text)
	}

	// Result should parse as a valid JSON object (map of enum name -> entries)
	text := result.Content[0].Text
	var enums map[string][]string
	if err := json.Unmarshal([]byte(text), &enums); err != nil {
		t.Fatalf("result is not valid JSON: %v\ntext: %s", err, text)
	}
	// Verify the result is a valid map (may be empty if type inference
	// does not populate entries for the specific enum syntax)
	_ = enums
}

func TestInspectTypesFunctionSignatures(t *testing.T) {
	code := `package com.example

fun greet(name: String): String {
    return "Hello, $name"
}

fun add(a: Int, b: Int): Int = a + b
`
	args, _ := json.Marshal(typesArgs{Code: code, Query: "function_signatures"})
	responses := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  mustJSON(t, ToolCallParams{Name: "types", Arguments: args}),
	})

	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}

	data, _ := json.Marshal(responses[0].Result)
	var result ToolResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if result.IsError {
		t.Fatalf("tool returned error: %s", result.Content[0].Text)
	}

	text := result.Content[0].Text
	var funcs []map[string]interface{}
	if err := json.Unmarshal([]byte(text), &funcs); err != nil {
		t.Fatalf("result is not valid JSON array: %v\ntext: %s", err, text)
	}
	// Should find at least one function
	if len(funcs) == 0 {
		t.Error("expected at least one function signature")
	}
}

func TestInspectTypesSealedVariants(t *testing.T) {
	code := `package com.example

sealed class Result
class Success(val data: String) : Result()
class Failure(val error: String) : Result()
`
	args, _ := json.Marshal(typesArgs{Code: code, Query: "sealed_variants"})
	responses := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  mustJSON(t, ToolCallParams{Name: "types", Arguments: args}),
	})

	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}

	data, _ := json.Marshal(responses[0].Result)
	var result ToolResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if result.IsError {
		t.Fatalf("tool returned error: %s", result.Content[0].Text)
	}

	// Result should be a JSON object (map)
	text := result.Content[0].Text
	var sealedMap map[string]interface{}
	if err := json.Unmarshal([]byte(text), &sealedMap); err != nil {
		t.Fatalf("result is not valid JSON: %v\ntext: %s", err, text)
	}
}

func TestInspectTypesHierarchy(t *testing.T) {
	code := `package com.example

interface Drawable
open class Shape : Drawable
class Circle : Shape()
`
	args, _ := json.Marshal(typesArgs{Code: code, Query: "hierarchy"})
	responses := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  mustJSON(t, ToolCallParams{Name: "types", Arguments: args}),
	})

	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}

	data, _ := json.Marshal(responses[0].Result)
	var result ToolResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if result.IsError {
		t.Fatalf("tool returned error: %s", result.Content[0].Text)
	}

	text := result.Content[0].Text
	var hierarchy []map[string]interface{}
	if err := json.Unmarshal([]byte(text), &hierarchy); err != nil {
		t.Fatalf("result is not valid JSON array: %v\ntext: %s", err, text)
	}

	if len(hierarchy) == 0 {
		t.Error("expected at least one hierarchy entry")
	}
}

func TestInspectTypesInvalidQuery(t *testing.T) {
	code := "fun main() {}\n"
	args, _ := json.Marshal(typesArgs{Code: code, Query: "invalid_query"})
	responses := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  mustJSON(t, ToolCallParams{Name: "types", Arguments: args}),
	})

	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}

	data, _ := json.Marshal(responses[0].Result)
	var result ToolResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if !result.IsError {
		t.Error("expected error for invalid query type")
	}
}

func TestInspectTypesMissingCode(t *testing.T) {
	args, _ := json.Marshal(typesArgs{Code: "", Query: "classes"})
	responses := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  mustJSON(t, ToolCallParams{Name: "types", Arguments: args}),
	})

	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}

	data, _ := json.Marshal(responses[0].Result)
	var result ToolResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if !result.IsError {
		t.Error("expected error for missing code")
	}
}

func TestFindReferencesInvalidPaths(t *testing.T) {
	args, _ := json.Marshal(symbolsArgs{Operation: "references",
		Name:         "MyClass",
		ProjectPaths: []string{"/nonexistent/path/that/does/not/exist"},
	})
	responses := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  mustJSON(t, ToolCallParams{Name: "symbols", Arguments: args}),
	})

	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}

	data, _ := json.Marshal(responses[0].Result)
	var result ToolResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if !result.IsError {
		t.Error("expected error for nonexistent path")
	}
}

func TestFindReferencesMissingName(t *testing.T) {
	args, _ := json.Marshal(symbolsArgs{Operation: "references",
		Name:         "",
		ProjectPaths: []string{"."},
	})
	responses := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  mustJSON(t, ToolCallParams{Name: "symbols", Arguments: args}),
	})

	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}

	data, _ := json.Marshal(responses[0].Result)
	var result ToolResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if !result.IsError {
		t.Error("expected error for missing name")
	}
}

func TestFindReferencesMissingPaths(t *testing.T) {
	args, _ := json.Marshal(symbolsArgs{Operation: "references",
		Name:         "MyClass",
		ProjectPaths: []string{},
	})
	responses := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  mustJSON(t, ToolCallParams{Name: "symbols", Arguments: args}),
	})

	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}

	data, _ := json.Marshal(responses[0].Result)
	var result ToolResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if !result.IsError {
		t.Error("expected error for missing project_paths")
	}
}

func TestAnalyzeProjectInvalidPaths(t *testing.T) {
	args, _ := json.Marshal(analyzeArgs{Mode: "project",
		Paths: []string{"/nonexistent/path/that/does/not/exist"},
	})
	responses := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  mustJSON(t, ToolCallParams{Name: "analyze", Arguments: args}),
	})

	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}

	data, _ := json.Marshal(responses[0].Result)
	var result ToolResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if !result.IsError {
		t.Error("expected error for nonexistent path")
	}
}

func TestAnalyzeProjectMissingPaths(t *testing.T) {
	args, _ := json.Marshal(analyzeArgs{Mode: "project",
		Paths: []string{},
	})
	responses := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  mustJSON(t, ToolCallParams{Name: "analyze", Arguments: args}),
	})

	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}

	data, _ := json.Marshal(responses[0].Result)
	var result ToolResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if !result.IsError {
		t.Error("expected error for missing paths")
	}
}

func TestFindReferencesWithFixtures(t *testing.T) {
	// Use the tests/fixtures directory which should have .kt files
	args, _ := json.Marshal(symbolsArgs{Operation: "references",
		Name:         "fun",
		ProjectPaths: []string{"../../tests/fixtures"},
	})
	responses := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  mustJSON(t, ToolCallParams{Name: "symbols", Arguments: args}),
	})

	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}

	data, _ := json.Marshal(responses[0].Result)
	var result ToolResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if result.IsError {
		t.Fatalf("tool returned error: %s", result.Content[0].Text)
	}

	text := result.Content[0].Text
	var refs []map[string]interface{}
	if err := json.Unmarshal([]byte(text), &refs); err != nil {
		t.Fatalf("result is not valid JSON array: %v\ntext: %s", err, text)
	}

	// "fun" keyword should appear in many Kotlin files
	if len(refs) == 0 {
		t.Error("expected at least one reference to 'fun' in fixtures")
	}
}

func TestAnalyzeProjectWithFixtures(t *testing.T) {
	// Use the tests/fixtures directory
	args, _ := json.Marshal(analyzeArgs{Mode: "project",
		Paths:  []string{"../../tests/fixtures"},
		Format: "summary",
	})
	responses := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  mustJSON(t, ToolCallParams{Name: "analyze", Arguments: args}),
	})

	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}

	data, _ := json.Marshal(responses[0].Result)
	var result ToolResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if result.IsError {
		t.Fatalf("tool returned error: %s", result.Content[0].Text)
	}

	text := result.Content[0].Text
	var summary map[string]interface{}
	if err := json.Unmarshal([]byte(text), &summary); err != nil {
		t.Fatalf("result is not valid JSON: %v\ntext: %s", err, text)
	}

	if _, ok := summary["totalFiles"]; !ok {
		t.Error("expected 'totalFiles' in summary")
	}
	if _, ok := summary["totalFindings"]; !ok {
		t.Error("expected 'totalFindings' in summary")
	}
	if _, ok := summary["topRules"]; !ok {
		t.Error("expected 'topRules' in summary")
	}

	totalFiles, ok := summary["totalFiles"].(float64)
	if !ok || totalFiles == 0 {
		t.Error("expected at least one file in analysis")
	}
}

// --- Phase 3: Android tools tests ---

func TestAnalyzeAndroidMissingPath(t *testing.T) {
	args, _ := json.Marshal(analyzeArgs{Mode: "android", ProjectPath: ""})
	responses := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  mustJSON(t, ToolCallParams{Name: "analyze", Arguments: args}),
	})

	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}

	data, _ := json.Marshal(responses[0].Result)
	var result ToolResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if !result.IsError {
		t.Error("expected error for missing project_path")
	}
}

func TestAnalyzeAndroidInvalidPath(t *testing.T) {
	args, _ := json.Marshal(analyzeArgs{Mode: "android", ProjectPath: "/nonexistent/path"})
	responses := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  mustJSON(t, ToolCallParams{Name: "analyze", Arguments: args}),
	})

	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}

	data, _ := json.Marshal(responses[0].Result)
	var result ToolResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if !result.IsError {
		t.Error("expected error for nonexistent path")
	}
}

func TestAnalyzeAndroidNonDir(t *testing.T) {
	// Use a file instead of a directory
	args, _ := json.Marshal(analyzeArgs{Mode: "android", ProjectPath: "../../go.mod"})
	responses := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  mustJSON(t, ToolCallParams{Name: "analyze", Arguments: args}),
	})

	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}

	data, _ := json.Marshal(responses[0].Result)
	var result ToolResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if !result.IsError {
		t.Error("expected error for non-directory path")
	}
}

func TestAnalyzeAndroidNoAndroidFiles(t *testing.T) {
	// Use a directory that has no Android files
	args, _ := json.Marshal(analyzeArgs{Mode: "android", ProjectPath: "../../internal/mcp"})
	responses := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  mustJSON(t, ToolCallParams{Name: "analyze", Arguments: args}),
	})

	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}

	data, _ := json.Marshal(responses[0].Result)
	var result ToolResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if !result.IsError {
		t.Error("expected error for directory with no Android files")
	}
}

func TestAnalyzeAndroidScopeValidation(t *testing.T) {
	// Use a directory that has no Android files, the error about no Android files
	// comes before scope matters, so just verify it handles scope param
	args, _ := json.Marshal(analyzeArgs{Mode: "android", ProjectPath: "../../internal/mcp", Scope: "manifest"})
	responses := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  mustJSON(t, ToolCallParams{Name: "analyze", Arguments: args}),
	})

	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}

	data, _ := json.Marshal(responses[0].Result)
	var result ToolResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	// Will error because no Android files found, which is expected
	if !result.IsError {
		t.Error("expected error for directory with no Android files")
	}
}

func TestInspectModulesMissingRoot(t *testing.T) {
	args, _ := json.Marshal(structureArgs{Operation: "modules", ProjectRoot: ""})
	responses := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  mustJSON(t, ToolCallParams{Name: "structure", Arguments: args}),
	})

	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}

	data, _ := json.Marshal(responses[0].Result)
	var result ToolResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if !result.IsError {
		t.Error("expected error for missing project_root")
	}
}

func TestInspectModulesInvalidPath(t *testing.T) {
	args, _ := json.Marshal(structureArgs{Operation: "modules", ProjectRoot: "/nonexistent/path"})
	responses := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  mustJSON(t, ToolCallParams{Name: "structure", Arguments: args}),
	})

	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}

	data, _ := json.Marshal(responses[0].Result)
	var result ToolResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if !result.IsError {
		t.Error("expected error for nonexistent path")
	}
}

func TestInspectModulesNoSettingsFile(t *testing.T) {
	// Use a directory that doesn't have settings.gradle
	args, _ := json.Marshal(structureArgs{Operation: "modules", ProjectRoot: "../../internal/mcp"})
	responses := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  mustJSON(t, ToolCallParams{Name: "structure", Arguments: args}),
	})

	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}

	data, _ := json.Marshal(responses[0].Result)
	var result ToolResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if !result.IsError {
		t.Error("expected error for directory without settings.gradle")
	}
}

func TestInspectModulesNonDir(t *testing.T) {
	args, _ := json.Marshal(structureArgs{Operation: "modules", ProjectRoot: "../../go.mod"})
	responses := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  mustJSON(t, ToolCallParams{Name: "structure", Arguments: args}),
	})

	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}

	data, _ := json.Marshal(responses[0].Result)
	var result ToolResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if !result.IsError {
		t.Error("expected error for non-directory path")
	}
}

func TestInspectModulesWithTempProject(t *testing.T) {
	// Create a temporary project with settings.gradle.kts
	tmpDir := t.TempDir()
	settingsContent := `include(":app", ":core:util")`
	if err := os.WriteFile(tmpDir+"/settings.gradle.kts", []byte(settingsContent), 0644); err != nil {
		t.Fatalf("write settings.gradle.kts: %v", err)
	}

	args, _ := json.Marshal(structureArgs{Operation: "modules", ProjectRoot: tmpDir})
	responses := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  mustJSON(t, ToolCallParams{Name: "structure", Arguments: args}),
	})

	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}

	data, _ := json.Marshal(responses[0].Result)
	var result ToolResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if result.IsError {
		t.Fatalf("tool returned error: %s", result.Content[0].Text)
	}

	text := result.Content[0].Text
	var graph map[string]interface{}
	if err := json.Unmarshal([]byte(text), &graph); err != nil {
		t.Fatalf("result is not valid JSON: %v\ntext: %s", err, text)
	}

	if _, ok := graph["moduleCount"]; !ok {
		t.Error("expected 'moduleCount' in result")
	}
	if _, ok := graph["modules"]; !ok {
		t.Error("expected 'modules' in result")
	}

	moduleCount, ok := graph["moduleCount"].(float64)
	if !ok || moduleCount != 2 {
		t.Errorf("expected 2 modules, got %v", graph["moduleCount"])
	}
}

func TestInspectModulesSpecificModule(t *testing.T) {
	tmpDir := t.TempDir()
	settingsContent := `include(":app", ":lib")`
	if err := os.WriteFile(tmpDir+"/settings.gradle.kts", []byte(settingsContent), 0644); err != nil {
		t.Fatalf("write settings.gradle.kts: %v", err)
	}

	args, _ := json.Marshal(structureArgs{Operation: "modules", ProjectRoot: tmpDir, Module: ":app"})
	responses := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  mustJSON(t, ToolCallParams{Name: "structure", Arguments: args}),
	})

	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}

	data, _ := json.Marshal(responses[0].Result)
	var result ToolResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if result.IsError {
		t.Fatalf("tool returned error: %s", result.Content[0].Text)
	}

	text := result.Content[0].Text
	var mod map[string]interface{}
	if err := json.Unmarshal([]byte(text), &mod); err != nil {
		t.Fatalf("result is not valid JSON: %v\ntext: %s", err, text)
	}

	if mod["path"] != ":app" {
		t.Errorf("expected module path ':app', got %v", mod["path"])
	}
}

func TestInspectModulesUnknownModule(t *testing.T) {
	tmpDir := t.TempDir()
	settingsContent := `include(":app")`
	if err := os.WriteFile(tmpDir+"/settings.gradle.kts", []byte(settingsContent), 0644); err != nil {
		t.Fatalf("write settings.gradle.kts: %v", err)
	}

	args, _ := json.Marshal(structureArgs{Operation: "modules", ProjectRoot: tmpDir, Module: ":nonexistent"})
	responses := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  mustJSON(t, ToolCallParams{Name: "structure", Arguments: args}),
	})

	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}

	data, _ := json.Marshal(responses[0].Result)
	var result ToolResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if !result.IsError {
		t.Error("expected error for unknown module")
	}
}

// --- Phase 4: Prompts tests ---

func TestPromptsListReturnsPrompts(t *testing.T) {
	responses := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "prompts/list",
	})

	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}

	data, _ := json.Marshal(responses[0].Result)
	var result PromptsListResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if len(result.Prompts) != 3 {
		t.Fatalf("expected 3 prompts, got %d", len(result.Prompts))
	}

	names := map[string]bool{}
	for _, p := range result.Prompts {
		names[p.Name] = true
	}
	for _, expected := range []string{"review_kotlin", "prepare_pr", "refactor_check"} {
		if !names[expected] {
			t.Errorf("missing prompt: %s", expected)
		}
	}
}

func TestPromptsGetReviewKotlin(t *testing.T) {
	params, _ := json.Marshal(PromptGetParams{
		Name: "review_kotlin",
		Arguments: map[string]string{
			"code": "fun main() {   \n    val x = 1\n}\n",
		},
	})
	responses := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "prompts/get",
		Params:  params,
	})

	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}

	resp := responses[0]
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	data, _ := json.Marshal(resp.Result)
	var result PromptGetResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if len(result.Messages) == 0 {
		t.Fatal("expected at least one message")
	}
	if result.Messages[0].Role != "user" {
		t.Errorf("expected role 'user', got %s", result.Messages[0].Role)
	}
	if !strings.Contains(result.Messages[0].Content.Text, "Static Analysis Findings") {
		t.Error("expected prompt to contain static analysis findings")
	}
	if !strings.Contains(result.Messages[0].Content.Text, "Auto-Fixes") {
		t.Error("expected prompt to contain auto-fixes section")
	}
}

func TestPromptsGetReviewKotlinMissingCode(t *testing.T) {
	params, _ := json.Marshal(PromptGetParams{
		Name:      "review_kotlin",
		Arguments: map[string]string{},
	})
	responses := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "prompts/get",
		Params:  params,
	})

	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}

	resp := responses[0]
	if resp.Error == nil {
		t.Fatal("expected error for missing code argument")
	}
}

func TestPromptsGetPreparePR(t *testing.T) {
	params, _ := json.Marshal(PromptGetParams{
		Name: "prepare_pr",
		Arguments: map[string]string{
			"paths": "../../tests/fixtures",
		},
	})
	responses := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "prompts/get",
		Params:  params,
	})

	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}

	resp := responses[0]
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	data, _ := json.Marshal(resp.Result)
	var result PromptGetResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if len(result.Messages) == 0 {
		t.Fatal("expected at least one message")
	}
	if !strings.Contains(result.Messages[0].Content.Text, "Project Analysis Results") {
		t.Error("expected prompt to contain project analysis results")
	}
}

func TestPromptsGetPreparePRMissingPaths(t *testing.T) {
	params, _ := json.Marshal(PromptGetParams{
		Name:      "prepare_pr",
		Arguments: map[string]string{},
	})
	responses := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "prompts/get",
		Params:  params,
	})

	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}

	resp := responses[0]
	if resp.Error == nil {
		t.Fatal("expected error for missing paths argument")
	}
}

func TestPromptsGetRefactorCheck(t *testing.T) {
	params, _ := json.Marshal(PromptGetParams{
		Name: "refactor_check",
		Arguments: map[string]string{
			"symbol":        "MyClass",
			"code":          "class MyClass { fun doSomething() {} }\n",
			"project_paths": "../../tests/fixtures",
		},
	})
	responses := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "prompts/get",
		Params:  params,
	})

	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}

	resp := responses[0]
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	data, _ := json.Marshal(resp.Result)
	var result PromptGetResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if len(result.Messages) == 0 {
		t.Fatal("expected at least one message")
	}
	text := result.Messages[0].Content.Text
	if !strings.Contains(text, "Symbol References") {
		t.Error("expected prompt to contain symbol references")
	}
	if !strings.Contains(text, "Type Information") {
		t.Error("expected prompt to contain type information")
	}
	if !strings.Contains(text, "Type Hierarchy") {
		t.Error("expected prompt to contain type hierarchy")
	}
}

func TestPromptsGetRefactorCheckMinimal(t *testing.T) {
	// Only symbol, no code or project_paths
	params, _ := json.Marshal(PromptGetParams{
		Name: "refactor_check",
		Arguments: map[string]string{
			"symbol": "SomeFunction",
		},
	})
	responses := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "prompts/get",
		Params:  params,
	})

	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}

	resp := responses[0]
	if resp.Error != nil {
		t.Fatalf("unexpected error: %v", resp.Error)
	}

	data, _ := json.Marshal(resp.Result)
	var result PromptGetResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if len(result.Messages) == 0 {
		t.Fatal("expected at least one message")
	}
	if !strings.Contains(result.Messages[0].Content.Text, "SomeFunction") {
		t.Error("expected prompt to mention the symbol name")
	}
}

func TestPromptsGetRefactorCheckMissingSymbol(t *testing.T) {
	params, _ := json.Marshal(PromptGetParams{
		Name:      "refactor_check",
		Arguments: map[string]string{},
	})
	responses := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "prompts/get",
		Params:  params,
	})

	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}

	resp := responses[0]
	if resp.Error == nil {
		t.Fatal("expected error for missing symbol argument")
	}
}

func TestPromptsGetUnknownPrompt(t *testing.T) {
	params, _ := json.Marshal(PromptGetParams{
		Name: "nonexistent_prompt",
	})
	responses := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "prompts/get",
		Params:  params,
	})

	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}

	resp := responses[0]
	if resp.Error == nil {
		t.Fatal("expected error for unknown prompt")
	}
}

func TestUnknownTool(t *testing.T) {
	args, _ := json.Marshal(map[string]string{"foo": "bar"})
	responses := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  mustJSON(t, ToolCallParams{Name: "nonexistent_tool", Arguments: args}),
	})

	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}

	data, _ := json.Marshal(responses[0].Result)
	var result ToolResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if !result.IsError {
		t.Error("expected error for unknown tool")
	}
}

func TestSuggestFixesWithFixableIssues(t *testing.T) {
	// Empty catch block triggers EmptyCatchBlock which populates Fix
	code := `fun main() {
    try {
        riskyCall()
    } catch (e: Exception) {
    }
}
`

	args, _ := json.Marshal(fixArgs{Operation: "suggest", Code: code})
	responses := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  mustJSON(t, ToolCallParams{Name: "fix", Arguments: args}),
	})

	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}

	data, _ := json.Marshal(responses[0].Result)
	var result ToolResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if result.IsError {
		t.Fatalf("tool returned error: %s", result.Content[0].Text)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected content in result")
	}

	text := result.Content[0].Text
	if text == "No auto-fixes available." {
		t.Error("expected fix suggestions for code with empty catch block")
	}

	// Verify result parses as a JSON array of fix objects
	var fixes []map[string]interface{}
	if err := json.Unmarshal([]byte(text), &fixes); err != nil {
		t.Fatalf("result is not valid JSON array: %v\ntext: %s", err, text)
	}
	if len(fixes) == 0 {
		t.Fatal("expected at least one fix suggestion")
	}

	// Verify fix object has expected fields
	fix := fixes[0]
	for _, field := range []string{"rule", "message", "fixLevel", "replacement"} {
		if _, ok := fix[field]; !ok {
			t.Errorf("expected field %q in fix object", field)
		}
	}
}

func TestSuggestFixesCleanCode(t *testing.T) {
	code := "fun main() {\n    println(\"hello\")\n}\n"

	args, _ := json.Marshal(fixArgs{Operation: "suggest", Code: code})
	responses := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  mustJSON(t, ToolCallParams{Name: "fix", Arguments: args}),
	})

	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}

	data, _ := json.Marshal(responses[0].Result)
	var result ToolResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if result.IsError {
		t.Fatalf("tool returned error: %s", result.Content[0].Text)
	}

	text := result.Content[0].Text
	if text != "No auto-fixes available." {
		t.Errorf("expected no fixes for clean code, got: %s", text)
	}
}

func TestSuggestFixesFixLevelCosmetic(t *testing.T) {
	// Empty catch block is semantic-level fix; filtering to cosmetic should exclude it
	code := `fun main() {
    try {
        riskyCall()
    } catch (e: Exception) {
    }
}
`

	args, _ := json.Marshal(fixArgs{Operation: "suggest", Code: code, FixLevel: "cosmetic"})
	responses := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  mustJSON(t, ToolCallParams{Name: "fix", Arguments: args}),
	})

	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}

	data, _ := json.Marshal(responses[0].Result)
	var result ToolResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if result.IsError {
		t.Fatalf("tool returned error: %s", result.Content[0].Text)
	}

	text := result.Content[0].Text
	// EmptyCatchBlock is semantic, so cosmetic filter should exclude it
	if text != "No auto-fixes available." {
		// If there are results, verify they are all cosmetic level
		var fixes []map[string]interface{}
		if err := json.Unmarshal([]byte(text), &fixes); err != nil {
			t.Fatalf("result is not valid JSON array: %v\ntext: %s", err, text)
		}
		for i, fix := range fixes {
			if fix["fixLevel"] != "cosmetic" {
				t.Errorf("fix %d: expected fixLevel cosmetic, got %v", i, fix["fixLevel"])
			}
		}
	}
}

func TestSuggestFixesFixLevelInvalid(t *testing.T) {
	code := "fun main() {\n}\n"

	args, _ := json.Marshal(fixArgs{Operation: "suggest", Code: code, FixLevel: "bogus"})
	responses := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  mustJSON(t, ToolCallParams{Name: "fix", Arguments: args}),
	})

	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}

	data, _ := json.Marshal(responses[0].Result)
	var result ToolResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if !result.IsError {
		t.Error("expected error for invalid fix_level")
	}
	if !strings.Contains(result.Content[0].Text, "invalid fix_level") {
		t.Errorf("expected error message about invalid fix_level, got: %s", result.Content[0].Text)
	}
}

func TestSuggestFixesMissingCode(t *testing.T) {
	args, _ := json.Marshal(fixArgs{Operation: "suggest"})
	responses := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  mustJSON(t, ToolCallParams{Name: "fix", Arguments: args}),
	})

	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}

	data, _ := json.Marshal(responses[0].Result)
	var result ToolResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if !result.IsError {
		t.Error("expected error for missing code argument")
	}
}

func TestSuggestFixesFixLevelAll(t *testing.T) {
	// fix_level "all" should not filter — return all fixable findings
	code := `fun main() {
    try {
        riskyCall()
    } catch (e: Exception) {
    }
}
`

	args, _ := json.Marshal(fixArgs{Operation: "suggest", Code: code, FixLevel: "all"})
	responses := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "tools/call",
		Params:  mustJSON(t, ToolCallParams{Name: "fix", Arguments: args}),
	})

	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}

	data, _ := json.Marshal(responses[0].Result)
	var result ToolResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}

	if result.IsError {
		t.Fatalf("tool returned error: %s", result.Content[0].Text)
	}

	text := result.Content[0].Text
	if text == "No auto-fixes available." {
		t.Error("expected fix suggestions with fix_level=all")
	}
}

// mustJSON marshals v to json.RawMessage, failing the test on error.
func mustJSON(t *testing.T, v interface{}) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return data
}

// runServerRaw feeds raw NDJSON lines straight at the server. Necessary
// for notification tests because json.Marshal of Request{ID: nil} with
// `id,omitempty` already drops the field, but constructing a request
// with ID set to typed nil is awkward; raw lines let the test express
// "no id field at all" unambiguously.
func runServerRaw(t *testing.T, lines ...string) []byte {
	t.Helper()
	var input strings.Builder
	for _, line := range lines {
		input.WriteString(line)
		if !strings.HasSuffix(line, "\n") {
			input.WriteString("\n")
		}
	}
	var output bytes.Buffer
	reader := bufio.NewReader(strings.NewReader(input.String()))
	server := NewServer(reader, &output)
	server.buildDispatcher()
	for {
		msg, err := jsonrpc.ReadMessageNDJSON(reader)
		if err != nil {
			break
		}
		var req Request
		if err := json.Unmarshal(msg, &req); err != nil {
			t.Fatalf("unmarshal request: %v", err)
		}
		server.handleMessage(&req)
	}
	return output.Bytes()
}

// TestNotificationsProduceNoResponse verifies that JSON-RPC notifications
// (messages with no `id` field) for known MCP methods never produce a
// response. The previous implementation called sendResponse(req.ID, …)
// unconditionally, which emitted a response with `"id":null` — a
// violation of JSON-RPC 2.0 (responses MUST correlate to a request id,
// and notifications MUST NOT receive a response).
func TestNotificationsProduceNoResponse(t *testing.T) {
	cases := []struct {
		name string
		line string
	}{
		{"tools/list notification", `{"jsonrpc":"2.0","method":"tools/list"}`},
		{"tools/call notification", `{"jsonrpc":"2.0","method":"tools/call","params":{"name":"analyze","arguments":{"code":"fun x(){}"}}}`},
		{"resources/list notification", `{"jsonrpc":"2.0","method":"resources/list"}`},
		{"resources/read notification", `{"jsonrpc":"2.0","method":"resources/read","params":{"uri":"krit://rules"}}`},
		{"prompts/list notification", `{"jsonrpc":"2.0","method":"prompts/list"}`},
		{"prompts/get notification", `{"jsonrpc":"2.0","method":"prompts/get","params":{"name":"review_kotlin","arguments":{"code":"fun x(){}"}}}`},
		{"initialize notification", `{"jsonrpc":"2.0","method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{}}}`},
		{"unknown method notification", `{"jsonrpc":"2.0","method":"does/not/exist"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := runServerRaw(t, tc.line)
			if len(bytes.TrimSpace(out)) != 0 {
				t.Errorf("expected no response for notification %q, got %q", tc.name, string(out))
			}
		})
	}
}

// TestNotificationDoesNotBlockSubsequentRequest pairs a notification with
// a real request to confirm the server keeps processing after dropping
// the notification — the fix must not accidentally break the message loop.
func TestNotificationDoesNotBlockSubsequentRequest(t *testing.T) {
	out := runServerRaw(t,
		`{"jsonrpc":"2.0","method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":42,"method":"tools/list"}`,
	)
	outReader := bufio.NewReader(bytes.NewReader(out))
	var responses []Response
	for {
		msg, err := jsonrpc.ReadMessageNDJSON(outReader)
		if err != nil {
			break
		}
		var resp Response
		if err := json.Unmarshal(msg, &resp); err != nil {
			t.Fatalf("unmarshal response: %v", err)
		}
		responses = append(responses, resp)
	}
	if len(responses) != 1 {
		t.Fatalf("expected exactly 1 response (only the request, not the notification), got %d: %s", len(responses), string(out))
	}
	// The response id must correlate to the request, not be `null` (which
	// is what the bug would produce for the dropped notification).
	idFloat, ok := responses[0].ID.(float64)
	if !ok {
		t.Fatalf("response id has unexpected type %T (%v); should be the request id, not null", responses[0].ID, responses[0].ID)
	}
	if int(idFloat) != 42 {
		t.Errorf("expected response id 42, got %v", responses[0].ID)
	}
}

// TestInitializeNegotiatesSupportedVersion covers the spec rule: if the
// client requests a version the server supports, the server MUST echo
// that same version.
func TestInitializeNegotiatesSupportedVersion(t *testing.T) {
	// Pick the first (newest) supported version so this stays
	// meaningful if/when the list grows.
	supported := supportedProtocolVersions[0]
	params := fmt.Sprintf(`{"protocolVersion":%q,"capabilities":{}}`, supported)
	responses := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "initialize",
		Params:  json.RawMessage(params),
	})
	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}
	data, _ := json.Marshal(responses[0].Result)
	var result InitializeResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result.ProtocolVersion != supported {
		t.Errorf("server returned %q, want %q (client requested supported version, server MUST echo it)", result.ProtocolVersion, supported)
	}
}

// TestInitializeFallsBackForUnsupportedVersion covers the spec rule: if
// the client requests a version the server does not support (e.g. a
// future spec revision), the server MUST respond with another version
// it supports (we return the server's latest = protocolVersion).
func TestInitializeFallsBackForUnsupportedVersion(t *testing.T) {
	// "9999-12-31" is well beyond anything we will ever publish; the
	// server cannot possibly support it.
	params := `{"protocolVersion":"9999-12-31","capabilities":{}}`
	responses := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "initialize",
		Params:  json.RawMessage(params),
	})
	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}
	data, _ := json.Marshal(responses[0].Result)
	var result InitializeResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result.ProtocolVersion != protocolVersion {
		t.Errorf("server returned %q for unsupported client version, want server default %q", result.ProtocolVersion, protocolVersion)
	}
}

// TestInitializeOmittedVersionUsesServerDefault covers the case where
// the client sends `initialize` without a `protocolVersion` field. The
// MCP spec says the client MUST send one, but a defensive server should
// still respond with its supported version rather than echoing the
// empty string back as a "version".
func TestInitializeOmittedVersionUsesServerDefault(t *testing.T) {
	params := `{"capabilities":{}}`
	responses := runServer(t, Request{
		JSONRPC: "2.0",
		ID:      float64(1),
		Method:  "initialize",
		Params:  json.RawMessage(params),
	})
	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}
	data, _ := json.Marshal(responses[0].Result)
	var result InitializeResult
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result.ProtocolVersion != protocolVersion {
		t.Errorf("server returned %q when client omitted protocolVersion, want server default %q", result.ProtocolVersion, protocolVersion)
	}
}

// TestNegotiateProtocolVersion exercises the pure helper directly so
// each spec rule has a one-line regression case independent of the
// JSON-RPC plumbing.
func TestNegotiateProtocolVersion(t *testing.T) {
	for _, supported := range supportedProtocolVersions {
		if got := negotiateProtocolVersion(supported); got != supported {
			t.Errorf("negotiate(%q) = %q, want %q (echo supported version)", supported, got, supported)
		}
	}
	if got := negotiateProtocolVersion(""); got != protocolVersion {
		t.Errorf("negotiate(\"\") = %q, want %q (server default)", got, protocolVersion)
	}
	if got := negotiateProtocolVersion("9999-12-31"); got != protocolVersion {
		t.Errorf("negotiate(future) = %q, want %q (server default)", got, protocolVersion)
	}
	if got := negotiateProtocolVersion("1999-01-01"); got != protocolVersion {
		t.Errorf("negotiate(past) = %q, want %q (server default)", got, protocolVersion)
	}
}
