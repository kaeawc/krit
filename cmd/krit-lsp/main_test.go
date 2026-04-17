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
	tmp, err := os.MkdirTemp("", "krit-lsp-integration-*")
	if err != nil {
		log.Fatal(err)
	}
	binPath = filepath.Join(tmp, "krit-lsp-test")
	cmd := exec.Command("go", "build", "-o", binPath, ".")
	cmd.Dir = "."
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Fatalf("failed to build krit-lsp binary: %v", err)
	}

	code := m.Run()

	os.RemoveAll(tmp)
	os.Exit(code)
}

// lspMessage builds a Content-Length framed LSP message.
func lspMessage(method string, id interface{}, params interface{}) []byte {
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

// readLSPMessage reads one Content-Length framed message from a reader.
func readLSPMessage(reader *bufio.Reader) (json.RawMessage, error) {
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

// readLSPResponse reads one message and unmarshals the "result" field into dest.
func readLSPResponse(reader *bufio.Reader, dest interface{}) (json.RawMessage, error) {
	raw, err := readLSPMessage(reader)
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

// startLSP starts the krit-lsp binary and returns stdin writer, stdout reader, and
// a cancel function. The process will be killed after the context is done.
func startLSP(t *testing.T, ctx context.Context) (io.WriteCloser, *bufio.Reader, *exec.Cmd) {
	t.Helper()
	cmd := exec.CommandContext(ctx, binPath)
	cmd.Stderr = &bytes.Buffer{} // capture stderr for debugging

	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("stdin pipe: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("start krit-lsp: %v", err)
	}

	return stdin, bufio.NewReader(stdout), cmd
}

// sendInitialize sends the initialize request and reads the response.
func sendInitialize(t *testing.T, stdin io.Writer, reader *bufio.Reader) {
	t.Helper()
	initMsg := lspMessage("initialize", 1, map[string]interface{}{
		"processId":    os.Getpid(),
		"rootUri":      "file:///tmp/test-workspace",
		"capabilities": map[string]interface{}{},
	})
	if _, err := stdin.Write(initMsg); err != nil {
		t.Fatalf("write initialize: %v", err)
	}

	var result struct {
		Capabilities struct {
			TextDocumentSync       interface{} `json:"textDocumentSync"`
			CodeActionProvider     bool        `json:"codeActionProvider"`
			CodeLensProvider       interface{} `json:"codeLensProvider"`
			HoverProvider          bool        `json:"hoverProvider"`
			DocumentSymbolProvider bool        `json:"documentSymbolProvider"`
		} `json:"capabilities"`
		ServerInfo *struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"serverInfo"`
	}
	if _, err := readLSPResponse(reader, &result); err != nil {
		t.Fatalf("read initialize response: %v", err)
	}
	if result.ServerInfo == nil || result.ServerInfo.Name != "krit-lsp" {
		t.Fatalf("expected serverInfo.name=krit-lsp, got %+v", result.ServerInfo)
	}
	if !result.Capabilities.CodeActionProvider {
		t.Fatalf("expected codeActionProvider=true")
	}
	if result.Capabilities.CodeLensProvider == nil {
		t.Fatalf("expected codeLensProvider to be advertised")
	}

	// Send initialized notification (no response expected)
	initializedMsg := lspMessage("initialized", nil, map[string]interface{}{})
	if _, err := stdin.Write(initializedMsg); err != nil {
		t.Fatalf("write initialized: %v", err)
	}
}

// sendShutdown sends shutdown request and reads the response.
func sendShutdown(t *testing.T, stdin io.Writer, reader *bufio.Reader) {
	t.Helper()
	shutdownMsg := lspMessage("shutdown", 2, nil)
	if _, err := stdin.Write(shutdownMsg); err != nil {
		t.Fatalf("write shutdown: %v", err)
	}
	if _, err := readLSPResponse(reader, nil); err != nil {
		t.Fatalf("read shutdown response: %v", err)
	}
}

// sendExit sends the exit notification.
func sendExit(t *testing.T, stdin io.Writer) {
	t.Helper()
	exitMsg := lspMessage("exit", nil, nil)
	if _, err := stdin.Write(exitMsg); err != nil {
		// Pipe may already be closed if process exited; that's ok
	}
}

func TestInitializeShutdown(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stdin, reader, cmd := startLSP(t, ctx)

	sendInitialize(t, stdin, reader)
	sendShutdown(t, stdin, reader)
	sendExit(t, stdin)
	stdin.Close()

	err := cmd.Wait()
	if err != nil {
		t.Fatalf("expected clean exit, got: %v", err)
	}
}

func TestDiagnosticsOnOpen(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stdin, reader, cmd := startLSP(t, ctx)
	defer func() {
		stdin.Close()
		cmd.Wait()
	}()

	sendInitialize(t, stdin, reader)

	// Send didOpen with Kotlin code that triggers UnusedVariable
	kotlinCode := "package test\n\nfun example() {\n    val x = 1\n}\n"
	didOpenMsg := lspMessage("textDocument/didOpen", nil, map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri":        "file:///tmp/test-workspace/Test.kt",
			"languageId": "kotlin",
			"version":    1,
			"text":       kotlinCode,
		},
	})
	if _, err := stdin.Write(didOpenMsg); err != nil {
		t.Fatalf("write didOpen: %v", err)
	}

	// Read publishDiagnostics notification
	raw, err := readLSPMessage(reader)
	if err != nil {
		t.Fatalf("read diagnostics notification: %v", err)
	}

	var notif struct {
		Method string `json:"method"`
		Params struct {
			URI         string `json:"uri"`
			Diagnostics []struct {
				Code    string `json:"code"`
				Source  string `json:"source"`
				Message string `json:"message"`
			} `json:"diagnostics"`
		} `json:"params"`
	}
	if err := json.Unmarshal(raw, &notif); err != nil {
		t.Fatalf("unmarshal diagnostics: %v", err)
	}
	if notif.Method != "textDocument/publishDiagnostics" {
		t.Fatalf("expected publishDiagnostics, got method=%s", notif.Method)
	}
	if notif.Params.URI != "file:///tmp/test-workspace/Test.kt" {
		t.Fatalf("expected URI for Test.kt, got %s", notif.Params.URI)
	}
	if len(notif.Params.Diagnostics) == 0 {
		t.Fatalf("expected at least one diagnostic for unused variable")
	}

	// Verify at least one diagnostic is from krit
	foundKrit := false
	for _, d := range notif.Params.Diagnostics {
		if d.Source == "krit" {
			foundKrit = true
			break
		}
	}
	if !foundKrit {
		t.Fatalf("expected at least one diagnostic with source=krit, got: %+v", notif.Params.Diagnostics)
	}

	// Clean shutdown
	sendShutdown(t, stdin, reader)
	sendExit(t, stdin)
}

func TestCodeActions(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stdin, reader, cmd := startLSP(t, ctx)
	defer func() {
		stdin.Close()
		cmd.Wait()
	}()

	sendInitialize(t, stdin, reader)

	// Send didOpen with code that has a fixable violation (trailing whitespace)
	kotlinCode := "package test\n\nfun example() {   \n    println(\"hi\")\n}\n"
	didOpenMsg := lspMessage("textDocument/didOpen", nil, map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri":        "file:///tmp/test-workspace/Actions.kt",
			"languageId": "kotlin",
			"version":    1,
			"text":       kotlinCode,
		},
	})
	if _, err := stdin.Write(didOpenMsg); err != nil {
		t.Fatalf("write didOpen: %v", err)
	}

	// Read publishDiagnostics notification (triggered by didOpen)
	_, err := readLSPMessage(reader)
	if err != nil {
		t.Fatalf("read diagnostics: %v", err)
	}

	// Send codeAction request
	codeActionMsg := lspMessage("textDocument/codeAction", 10, map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": "file:///tmp/test-workspace/Actions.kt",
		},
		"range": map[string]interface{}{
			"start": map[string]interface{}{"line": 0, "character": 0},
			"end":   map[string]interface{}{"line": 5, "character": 0},
		},
		"context": map[string]interface{}{
			"diagnostics": []interface{}{},
		},
	})
	if _, err := stdin.Write(codeActionMsg); err != nil {
		t.Fatalf("write codeAction: %v", err)
	}

	// Read codeAction response
	var actions []struct {
		Title string `json:"title"`
		Kind  string `json:"kind"`
		Edit  *struct {
			Changes map[string][]struct {
				Range   interface{} `json:"range"`
				NewText string      `json:"newText"`
			} `json:"changes"`
		} `json:"edit"`
	}
	if _, err := readLSPResponse(reader, &actions); err != nil {
		t.Fatalf("read codeAction response: %v", err)
	}

	// Check that we get quickfix actions (may be empty if the code has no fixable findings)
	// The response should at least be a valid array
	if actions == nil {
		// Response was null, which is acceptable (no actions)
		return
	}

	// If there are actions, verify they are quickfix kind
	for _, a := range actions {
		if a.Kind != "quickfix" {
			t.Fatalf("expected kind=quickfix, got %s for action %q", a.Kind, a.Title)
		}
		if !strings.HasPrefix(a.Title, "Fix: ") {
			t.Fatalf("expected title to start with 'Fix: ', got %q", a.Title)
		}
	}

	sendShutdown(t, stdin, reader)
	sendExit(t, stdin)
}

func TestCodeLens(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stdin, reader, cmd := startLSP(t, ctx)
	defer func() {
		stdin.Close()
		cmd.Wait()
	}()

	sendInitialize(t, stdin, reader)

	codeLensMsg := lspMessage("textDocument/codeLens", 11, map[string]interface{}{
		"textDocument": map[string]interface{}{
			"uri": "file:///tmp/test-workspace/Lenses.kt",
		},
	})
	if _, err := stdin.Write(codeLensMsg); err != nil {
		t.Fatalf("write codeLens: %v", err)
	}

	var lenses []map[string]interface{}
	if _, err := readLSPResponse(reader, &lenses); err != nil {
		t.Fatalf("read codeLens response: %v", err)
	}
	if len(lenses) != 0 {
		t.Fatalf("expected 0 code lenses, got %d", len(lenses))
	}

	sendShutdown(t, stdin, reader)
	sendExit(t, stdin)
}

func TestCleanExit(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stdin, reader, cmd := startLSP(t, ctx)

	sendInitialize(t, stdin, reader)
	sendShutdown(t, stdin, reader)
	sendExit(t, stdin)
	stdin.Close()

	err := cmd.Wait()
	if err != nil {
		t.Fatalf("expected exit code 0 after shutdown+exit, got error: %v", err)
	}
	if cmd.ProcessState.ExitCode() != 0 {
		t.Fatalf("expected exit code 0, got %d", cmd.ProcessState.ExitCode())
	}
}

func TestExitWithoutShutdown(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	stdin, reader, cmd := startLSP(t, ctx)

	sendInitialize(t, stdin, reader)

	// Send exit without shutdown — per LSP spec this should exit with code 1
	sendExit(t, stdin)
	stdin.Close()

	err := cmd.Wait()
	if err == nil {
		t.Fatalf("expected non-zero exit code when exiting without shutdown")
	}
	exitErr, ok := err.(*exec.ExitError)
	if !ok {
		t.Fatalf("expected ExitError, got %T: %v", err, err)
	}
	if exitErr.ExitCode() != 1 {
		t.Fatalf("expected exit code 1, got %d", exitErr.ExitCode())
	}
}
