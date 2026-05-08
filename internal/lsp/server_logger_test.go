package lsp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/logger"
)

// TestServerLogsReadErrorViaLogger verifies Server.Run routes a non-EOF
// read error through the injected Logger at Error level rather than the
// standard log package.
func TestServerLogsReadErrorViaLogger(t *testing.T) {
	// Malformed Content-Length header → ReadMessage returns a parse
	// error (not io.EOF), driving Run into the s.log.Error path.
	reader := bufio.NewReader(strings.NewReader("Content-Length: notanumber\r\n\r\n"))
	s := NewServer(reader, &bytes.Buffer{})

	cap := logger.NewCapture(slog.LevelDebug)
	s.SetLogger(cap)

	s.Run() // returns when the read error fires

	if !cap.HasMessage("read error") {
		t.Fatalf("expected 'read error' record, got %+v", cap.Records())
	}
	if got := cap.FilterLevel(slog.LevelError); len(got) == 0 {
		t.Errorf("expected at least one Error record, got %+v", cap.Records())
	}
}

// TestServerLogsParamsErrorViaLogger drives an LSP method handler with
// malformed JSON params and verifies the params-error log call routes
// through the injected Logger at Warn level.
func TestServerLogsParamsErrorViaLogger(t *testing.T) {
	s := NewServer(bufio.NewReader(strings.NewReader("")), &bytes.Buffer{})
	cap := logger.NewCapture(slog.LevelDebug)
	s.SetLogger(cap)

	// didOpen with non-JSON params → unmarshal fails → s.log.Warn fires.
	req := &Request{
		Method: "textDocument/didOpen",
		Params: json.RawMessage(`not valid json`),
	}
	s.handleDidOpen(req)

	warns := cap.FilterLevel(slog.LevelWarn)
	if len(warns) == 0 {
		t.Fatalf("expected at least one Warn record, got %+v", cap.Records())
	}
	if !cap.HasMessage("didOpen params error") {
		t.Errorf("expected 'didOpen params error' record, got %+v", cap.Records())
	}
}

// TestServerLogInfoGatedByVerbose confirms logInfo is silent when
// Verbose=false (the default) and emits an Info record otherwise.
func TestServerLogInfoGatedByVerbose(t *testing.T) {
	s := NewServer(bufio.NewReader(strings.NewReader("")), &bytes.Buffer{})
	cap := logger.NewCapture(slog.LevelDebug)
	s.SetLogger(cap)

	s.logInfo("starting up")
	if got := len(cap.Records()); got != 0 {
		t.Errorf("Verbose=false should suppress logInfo, got %d records", got)
	}

	s.Verbose = true
	s.logInfo("session ready: %s", "ok")
	infos := cap.FilterLevel(slog.LevelInfo)
	if len(infos) != 1 {
		t.Fatalf("expected 1 Info record, got %d: %+v", len(infos), cap.Records())
	}
	if !strings.Contains(infos[0].Msg, "session ready: ok") {
		t.Errorf("Info message = %q, want substring 'session ready: ok'", infos[0].Msg)
	}
}
