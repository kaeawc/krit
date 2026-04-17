package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var binPath string

func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "krit-mcp-integration-*")
	if err != nil {
		log.Fatal(err)
	}
	binPath = filepath.Join(tmp, "krit-mcp-test")
	cmd := exec.Command("go", "build", "-o", binPath, ".")
	cmd.Dir = "."
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Fatalf("failed to build krit-mcp binary: %v", err)
	}

	code := m.Run()

	os.RemoveAll(tmp)
	os.Exit(code)
}

// mcpMessage builds a Content-Length framed MCP/JSON-RPC message.
func mcpMessage(method string, id interface{}, params interface{}) []byte {
	msg := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  method,
	}
	if id != nil {
		msg["id"] = id
	}
	if params != nil {
		msg["params"] = params
	}
	body, _ := json.Marshal(msg)
	return []byte(fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), body))
}

// readMCPMessage reads one Content-Length framed message from a reader.
func readMCPMessage(reader *bufio.Reader) (json.RawMessage, error) {
	var contentLength int
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			break
		}
		if strings.HasPrefix(line, "Content-Length: ") {
			fmt.Sscanf(strings.TrimPrefix(line, "Content-Length: "), "%d", &contentLength)
		}
	}
	if contentLength == 0 {
		return nil, fmt.Errorf("missing Content-Length")
	}
	body := make([]byte, contentLength)
	_, err := io.ReadFull(reader, body)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(body), nil
}

// readMCPResponse reads one message and unmarshals the "result" field into dest.
func readMCPResponse(reader *bufio.Reader, dest interface{}) (json.RawMessage, error) {
	raw, err := readMCPMessage(reader)
	if err != nil {
		return nil, err
	}
	if dest != nil {
		var envelope struct {
			ID     interface{}     `json:"id"`
			Result json.RawMessage `json:"result"`
			Error  *struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if err := json.Unmarshal(raw, &envelope); err != nil {
			return raw, err
		}
		if envelope.Error != nil {
			return raw, fmt.Errorf("RPC error %d: %s", envelope.Error.Code, envelope.Error.Message)
		}
		if envelope.Result != nil {
			if err := json.Unmarshal(envelope.Result, dest); err != nil {
				return raw, err
			}
		}
	}
	return raw, nil
}

// startMCP starts the krit-mcp binary and returns stdin writer, stdout reader, and the command.
func startMCP(t *testing.T, ctx context.Context) (io.WriteCloser, *bufio.Reader, *exec.Cmd) {
	t.Helper()
	cmd := exec.CommandContext(ctx, binPath)
	cmd.Stderr = &bytes.Buffer{}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("stdin pipe: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("start krit-mcp: %v", err)
	}

	return stdin, bufio.NewReader(stdout), cmd
}

// sendInitialize sends the MCP initialize request and reads the response.
func sendInitialize(t *testing.T, stdin io.Writer, reader *bufio.Reader) json.RawMessage {
	t.Helper()
	initMsg := mcpMessage("initialize", 1, map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]interface{}{},
		"clientInfo": map[string]interface{}{
			"name":    "test-client",
			"version": "1.0.0",
		},
	})
	if _, err := stdin.Write(initMsg); err != nil {
		t.Fatalf("write initialize: %v", err)
	}

	raw, err := readMCPMessage(reader)
	if err != nil {
		t.Fatalf("read initialize response: %v", err)
	}

	// Send initialized notification
	initializedMsg := mcpMessage("initialized", nil, map[string]interface{}{})
	if _, err := stdin.Write(initializedMsg); err != nil {
		t.Fatalf("write initialized: %v", err)
	}

	return raw
}

// readPlaygroundFile reads a file from the playground directory and returns its content.
func readPlaygroundFile(t *testing.T, relPath string) string {
	t.Helper()
	// Resolve relative to the repo root (two levels up from cmd/krit-mcp/)
	base, err := filepath.Abs(filepath.Join("..", "..", relPath))
	if err != nil {
		t.Fatalf("resolve path %s: %v", relPath, err)
	}
	data, err := os.ReadFile(base)
	if err != nil {
		t.Fatalf("read %s: %v", base, err)
	}
	return string(data)
}

// playgroundAbsPath returns the absolute path to a playground directory.
func playgroundAbsPath(t *testing.T, relPath string) string {
	t.Helper()
	base, err := filepath.Abs(filepath.Join("..", "..", relPath))
	if err != nil {
		t.Fatalf("resolve path %s: %v", relPath, err)
	}
	return base
}

func TestMCPInitialize(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stdin, reader, cmd := startMCP(t, ctx)
	defer func() {
		stdin.Close()
		cmd.Wait()
	}()

	raw := sendInitialize(t, stdin, reader)

	var resp struct {
		Result struct {
			ProtocolVersion string `json:"protocolVersion"`
			Capabilities    struct {
				Tools     interface{} `json:"tools"`
				Resources interface{} `json:"resources"`
			} `json:"capabilities"`
			ServerInfo struct {
				Name    string `json:"name"`
				Version string `json:"version"`
			} `json:"serverInfo"`
		} `json:"result"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		t.Fatalf("unmarshal initialize response: %v", err)
	}

	if resp.Result.ServerInfo.Name != "krit-mcp" {
		t.Fatalf("expected serverInfo.name=krit-mcp, got %q", resp.Result.ServerInfo.Name)
	}
	if resp.Result.Capabilities.Tools == nil {
		t.Fatal("expected tools capability to be present")
	}
	if resp.Result.Capabilities.Resources == nil {
		t.Fatal("expected resources capability to be present")
	}

	// Now list tools to verify count
	toolsListMsg := mcpMessage("tools/list", 2, map[string]interface{}{})
	if _, err := stdin.Write(toolsListMsg); err != nil {
		t.Fatalf("write tools/list: %v", err)
	}

	var toolsResult struct {
		Tools []struct {
			Name string `json:"name"`
		} `json:"tools"`
	}
	if _, err := readMCPResponse(reader, &toolsResult); err != nil {
		t.Fatalf("read tools/list response: %v", err)
	}
	if len(toolsResult.Tools) != 8 {
		names := make([]string, len(toolsResult.Tools))
		for i, tool := range toolsResult.Tools {
			names[i] = tool.Name
		}
		t.Fatalf("expected 8 tools, got %d: %v", len(toolsResult.Tools), names)
	}

	// List resources to verify count
	resourcesListMsg := mcpMessage("resources/list", 3, map[string]interface{}{})
	if _, err := stdin.Write(resourcesListMsg); err != nil {
		t.Fatalf("write resources/list: %v", err)
	}

	var resourcesResult struct {
		Resources []struct {
			URI  string `json:"uri"`
			Name string `json:"name"`
		} `json:"resources"`
	}
	if _, err := readMCPResponse(reader, &resourcesResult); err != nil {
		t.Fatalf("read resources/list response: %v", err)
	}
	if len(resourcesResult.Resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(resourcesResult.Resources))
	}
}

func TestMCPAnalyzePlaygroundWebService(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stdin, reader, cmd := startMCP(t, ctx)
	defer func() {
		stdin.Close()
		cmd.Wait()
	}()

	sendInitialize(t, stdin, reader)

	code := readPlaygroundFile(t, "playground/kotlin-webservice/src/main/kotlin/com/example/services/UserService.kt")

	callMsg := mcpMessage("tools/call", 10, map[string]interface{}{
		"name": "analyze",
		"arguments": map[string]interface{}{
			"code": code,
			"path": "com/example/services/UserService.kt",
		},
	})
	if _, err := stdin.Write(callMsg); err != nil {
		t.Fatalf("write tools/call: %v", err)
	}

	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if _, err := readMCPResponse(reader, &result); err != nil {
		t.Fatalf("read analyze response: %v", err)
	}
	if result.IsError {
		t.Fatalf("analyze returned error: %s", result.Content[0].Text)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected content in analyze response")
	}

	// Parse the findings JSON
	var findings []struct {
		Rule    string `json:"rule"`
		Message string `json:"message"`
		Line    int    `json:"line"`
	}
	if err := json.Unmarshal([]byte(result.Content[0].Text), &findings); err != nil {
		t.Fatalf("unmarshal findings: %v", err)
	}
	if len(findings) == 0 {
		t.Fatal("expected at least one finding for UserService.kt")
	}

	// Verify MagicNumber is among the findings
	foundMagicNumber := false
	for _, f := range findings {
		if f.Rule == "MagicNumber" {
			foundMagicNumber = true
			break
		}
	}
	if !foundMagicNumber {
		rules := make([]string, len(findings))
		for i, f := range findings {
			rules[i] = f.Rule
		}
		t.Fatalf("expected MagicNumber finding, got rules: %v", rules)
	}
}

func TestMCPAnalyzePlaygroundAndroidApp(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stdin, reader, cmd := startMCP(t, ctx)
	defer func() {
		stdin.Close()
		cmd.Wait()
	}()

	sendInitialize(t, stdin, reader)

	code := readPlaygroundFile(t, "playground/android-app/src/main/kotlin/com/example/app/MainActivity.kt")

	callMsg := mcpMessage("tools/call", 10, map[string]interface{}{
		"name": "analyze",
		"arguments": map[string]interface{}{
			"code": code,
			"path": "com/example/app/MainActivity.kt",
		},
	})
	if _, err := stdin.Write(callMsg); err != nil {
		t.Fatalf("write tools/call: %v", err)
	}

	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if _, err := readMCPResponse(reader, &result); err != nil {
		t.Fatalf("read analyze response: %v", err)
	}
	if result.IsError {
		t.Fatalf("analyze returned error: %s", result.Content[0].Text)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected content in analyze response")
	}

	var findings []struct {
		Rule    string `json:"rule"`
		Message string `json:"message"`
		Line    int    `json:"line"`
	}
	if err := json.Unmarshal([]byte(result.Content[0].Text), &findings); err != nil {
		t.Fatalf("unmarshal findings: %v", err)
	}
	if len(findings) == 0 {
		t.Fatal("expected at least one finding for MainActivity.kt")
	}

	// Verify MagicNumber is among the findings
	foundMagicNumber := false
	for _, f := range findings {
		if f.Rule == "MagicNumber" {
			foundMagicNumber = true
			break
		}
	}
	if !foundMagicNumber {
		rules := make([]string, len(findings))
		for i, f := range findings {
			rules[i] = f.Rule
		}
		t.Fatalf("expected MagicNumber finding, got rules: %v", rules)
	}
}

func TestMCPSuggestFixes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stdin, reader, cmd := startMCP(t, ctx)
	defer func() {
		stdin.Close()
		cmd.Wait()
	}()

	sendInitialize(t, stdin, reader)

	// Use code with a fixable violation (e.g. trailing whitespace, empty function block)
	code := "package test\n\nfun example() {   \n    println(\"hi\")\n}\n\nfun unused() {\n}\n"

	callMsg := mcpMessage("tools/call", 10, map[string]interface{}{
		"name": "suggest_fixes",
		"arguments": map[string]interface{}{
			"code":      code,
			"fix_level": "all",
		},
	})
	if _, err := stdin.Write(callMsg); err != nil {
		t.Fatalf("write tools/call: %v", err)
	}

	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if _, err := readMCPResponse(reader, &result); err != nil {
		t.Fatalf("read suggest_fixes response: %v", err)
	}
	if result.IsError {
		t.Fatalf("suggest_fixes returned error: %s", result.Content[0].Text)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected content in suggest_fixes response")
	}

	text := result.Content[0].Text

	// If there are fixes, verify they have fix levels
	if text != "No auto-fixes available." {
		var fixes []struct {
			Rule     string `json:"rule"`
			FixLevel string `json:"fixLevel"`
		}
		if err := json.Unmarshal([]byte(text), &fixes); err != nil {
			t.Fatalf("unmarshal fixes: %v", err)
		}
		for _, fix := range fixes {
			if fix.FixLevel == "" {
				t.Fatalf("fix for rule %q missing fixLevel", fix.Rule)
			}
			validLevels := map[string]bool{"cosmetic": true, "idiomatic": true, "semantic": true}
			if !validLevels[fix.FixLevel] {
				t.Fatalf("fix for rule %q has invalid fixLevel %q", fix.Rule, fix.FixLevel)
			}
		}
	}
}

func TestMCPExplainRule(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stdin, reader, cmd := startMCP(t, ctx)
	defer func() {
		stdin.Close()
		cmd.Wait()
	}()

	sendInitialize(t, stdin, reader)

	callMsg := mcpMessage("tools/call", 10, map[string]interface{}{
		"name": "explain_rule",
		"arguments": map[string]interface{}{
			"rule": "MagicNumber",
		},
	})
	if _, err := stdin.Write(callMsg); err != nil {
		t.Fatalf("write tools/call: %v", err)
	}

	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if _, err := readMCPResponse(reader, &result); err != nil {
		t.Fatalf("read explain_rule response: %v", err)
	}
	if result.IsError {
		t.Fatalf("explain_rule returned error: %s", result.Content[0].Text)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected content in explain_rule response")
	}

	// Parse the rule metadata
	var ruleInfo struct {
		Name     string `json:"name"`
		RuleSet  string `json:"ruleSet"`
		Severity string `json:"severity"`
		Active   bool   `json:"active"`
		Fixable  bool   `json:"fixable"`
	}
	if err := json.Unmarshal([]byte(result.Content[0].Text), &ruleInfo); err != nil {
		t.Fatalf("unmarshal rule info: %v (text: %s)", err, result.Content[0].Text)
	}
	if ruleInfo.Name != "MagicNumber" {
		t.Fatalf("expected name=MagicNumber, got %q", ruleInfo.Name)
	}
	if ruleInfo.RuleSet == "" {
		t.Fatal("expected non-empty ruleSet")
	}
	if ruleInfo.Severity == "" {
		t.Fatal("expected non-empty severity")
	}
}

func TestMCPInspectTypes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stdin, reader, cmd := startMCP(t, ctx)
	defer func() {
		stdin.Close()
		cmd.Wait()
	}()

	sendInitialize(t, stdin, reader)

	code := `package test

data class User(
    val id: String,
    val name: String,
    val email: String,
    val age: Int
)
`

	callMsg := mcpMessage("tools/call", 10, map[string]interface{}{
		"name": "inspect_types",
		"arguments": map[string]interface{}{
			"code":  code,
			"query": "classes",
		},
	})
	if _, err := stdin.Write(callMsg); err != nil {
		t.Fatalf("write tools/call: %v", err)
	}

	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if _, err := readMCPResponse(reader, &result); err != nil {
		t.Fatalf("read inspect_types response: %v", err)
	}
	if result.IsError {
		t.Fatalf("inspect_types returned error: %s", result.Content[0].Text)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected content in inspect_types response")
	}

	var classes []struct {
		Name   string `json:"name"`
		Kind   string `json:"kind"`
		IsData bool   `json:"isData"`
	}
	if err := json.Unmarshal([]byte(result.Content[0].Text), &classes); err != nil {
		t.Fatalf("unmarshal classes: %v", err)
	}

	foundUser := false
	for _, c := range classes {
		if c.Name == "User" {
			foundUser = true
			if !c.IsData {
				t.Fatal("expected User class to be a data class")
			}
			break
		}
	}
	if !foundUser {
		t.Fatalf("expected to find User class, got: %+v", classes)
	}
}

func TestMCPFindReferences(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stdin, reader, cmd := startMCP(t, ctx)
	defer func() {
		stdin.Close()
		cmd.Wait()
	}()

	sendInitialize(t, stdin, reader)

	playgroundPath := playgroundAbsPath(t, "playground/kotlin-webservice")

	callMsg := mcpMessage("tools/call", 10, map[string]interface{}{
		"name": "find_references",
		"arguments": map[string]interface{}{
			"name":          "UserService",
			"project_paths": []string{playgroundPath},
		},
	})
	if _, err := stdin.Write(callMsg); err != nil {
		t.Fatalf("write tools/call: %v", err)
	}

	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if _, err := readMCPResponse(reader, &result); err != nil {
		t.Fatalf("read find_references response: %v", err)
	}
	if result.IsError {
		t.Fatalf("find_references returned error: %s", result.Content[0].Text)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected content in find_references response")
	}

	var refs []struct {
		File string `json:"file"`
		Line int    `json:"line"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(result.Content[0].Text), &refs); err != nil {
		t.Fatalf("unmarshal references: %v", err)
	}
	if len(refs) == 0 {
		t.Fatal("expected at least one reference to UserService in playground/kotlin-webservice")
	}

	// Verify at least one reference is in UserService.kt itself
	foundSelf := false
	for _, ref := range refs {
		if strings.Contains(ref.File, "UserService.kt") {
			foundSelf = true
			break
		}
	}
	if !foundSelf {
		t.Fatalf("expected a reference in UserService.kt, got files: %+v", refs)
	}
}

func TestMCPAnalyzeProject(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stdin, reader, cmd := startMCP(t, ctx)
	defer func() {
		stdin.Close()
		cmd.Wait()
	}()

	sendInitialize(t, stdin, reader)

	playgroundPath := playgroundAbsPath(t, "playground/kotlin-webservice")

	callMsg := mcpMessage("tools/call", 10, map[string]interface{}{
		"name": "analyze_project",
		"arguments": map[string]interface{}{
			"paths": []string{playgroundPath},
		},
	})
	if _, err := stdin.Write(callMsg); err != nil {
		t.Fatalf("write tools/call: %v", err)
	}

	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if _, err := readMCPResponse(reader, &result); err != nil {
		t.Fatalf("read analyze_project response: %v", err)
	}
	if result.IsError {
		t.Fatalf("analyze_project returned error: %s", result.Content[0].Text)
	}
	if len(result.Content) == 0 {
		t.Fatal("expected content in analyze_project response")
	}

	var summary struct {
		TotalFiles    int `json:"totalFiles"`
		TotalFindings int `json:"totalFindings"`
		TopRules      []struct {
			Rule  string `json:"rule"`
			Count int    `json:"count"`
		} `json:"topRules"`
	}
	if err := json.Unmarshal([]byte(result.Content[0].Text), &summary); err != nil {
		t.Fatalf("unmarshal summary: %v", err)
	}
	if summary.TotalFiles == 0 {
		t.Fatal("expected totalFiles > 0")
	}
	if summary.TotalFindings == 0 {
		t.Fatal("expected totalFindings > 0 for playground/kotlin-webservice")
	}
	if len(summary.TopRules) == 0 {
		t.Fatal("expected topRules to be non-empty")
	}
}

func TestMCPResourcesRules(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stdin, reader, cmd := startMCP(t, ctx)
	defer func() {
		stdin.Close()
		cmd.Wait()
	}()

	sendInitialize(t, stdin, reader)

	readMsg := mcpMessage("resources/read", 10, map[string]interface{}{
		"uri": "krit://rules",
	})
	if _, err := stdin.Write(readMsg); err != nil {
		t.Fatalf("write resources/read: %v", err)
	}

	var result struct {
		Contents []struct {
			URI      string `json:"uri"`
			MimeType string `json:"mimeType"`
			Text     string `json:"text"`
		} `json:"contents"`
	}
	if _, err := readMCPResponse(reader, &result); err != nil {
		t.Fatalf("read resources/read response: %v", err)
	}
	if len(result.Contents) == 0 {
		t.Fatal("expected contents in resources/read response")
	}
	if result.Contents[0].URI != "krit://rules" {
		t.Fatalf("expected URI=krit://rules, got %q", result.Contents[0].URI)
	}
	if result.Contents[0].MimeType != "application/json" {
		t.Fatalf("expected mimeType=application/json, got %q", result.Contents[0].MimeType)
	}

	// Parse the rules JSON array
	var rules []struct {
		Name     string `json:"name"`
		RuleSet  string `json:"ruleSet"`
		Severity string `json:"severity"`
		Active   bool   `json:"active"`
		Fixable  bool   `json:"fixable"`
	}
	if err := json.Unmarshal([]byte(result.Contents[0].Text), &rules); err != nil {
		t.Fatalf("unmarshal rules: %v", err)
	}
	if len(rules) == 0 {
		t.Fatal("expected non-empty rules array")
	}

	// Verify MagicNumber is in the list
	foundMagicNumber := false
	for _, r := range rules {
		if r.Name == "MagicNumber" {
			foundMagicNumber = true
			if r.RuleSet == "" {
				t.Fatal("expected MagicNumber to have non-empty ruleSet")
			}
			break
		}
	}
	if !foundMagicNumber {
		t.Fatal("expected MagicNumber in rules list")
	}
}
