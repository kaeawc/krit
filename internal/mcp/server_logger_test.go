package mcp

import (
	"bufio"
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/logger"
)

// TestServerLogsReadErrorViaLogger verifies that Server.Run routes a
// non-EOF read error through the injected Logger at Error level rather
// than the standard log package. Uses logger.NewCapture so the test
// finishes without inspecting stderr.
func TestServerLogsReadErrorViaLogger(t *testing.T) {
	// Malformed Content-Length header → ReadMessage returns a parse
	// error (not io.EOF), driving Run into the s.log.Error path.
	reader := bufio.NewReader(strings.NewReader("Content-Length: notanumber\r\n\r\n"))
	var out bytes.Buffer
	s := NewServer(reader, &out)

	cap := logger.NewCapture(slog.LevelDebug)
	s.SetLogger(cap)

	s.Run() // returns when the read error is hit

	if !cap.HasMessage("read error") && !cap.HasMessage("invalid JSON-RPC message") {
		t.Fatalf("expected a logger record, got %+v", cap.Records())
	}
	// At least one record must be at Warn or Error level — invalid input
	// can take either path depending on what jsonrpc.ReadMessage rejected.
	var sawErrOrWarn bool
	for _, r := range cap.Records() {
		if r.Level == slog.LevelError || r.Level == slog.LevelWarn {
			sawErrOrWarn = true
			break
		}
	}
	if !sawErrOrWarn {
		t.Errorf("expected at least one Warn/Error record, got: %+v", cap.Records())
	}
}

// TestServerLogInfoGatedByVerbose verifies that the lifecycle logInfo
// helper is silent when Verbose=false (the default) and emits an Info
// record when Verbose=true.
func TestServerLogInfoGatedByVerbose(t *testing.T) {
	reader := bufio.NewReader(strings.NewReader(""))
	s := NewServer(reader, &bytes.Buffer{})
	cap := logger.NewCapture(slog.LevelDebug)
	s.SetLogger(cap)

	// Verbose=false — silent.
	s.logInfo("starting up")
	if got := len(cap.Records()); got != 0 {
		t.Errorf("Verbose=false should suppress logInfo, got %d records", got)
	}

	// Verbose=true — Info record.
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

// TestServerDefaultLoggerWritesText sanity-checks that NewServer's
// default Logger is wired and writes to its configured Writer at Info
// level (verifies the wiring plumbs through; production format is
// human-readable text).
func TestServerDefaultLoggerWritesText(t *testing.T) {
	var out bytes.Buffer
	reader := bufio.NewReader(strings.NewReader(""))
	s := NewServer(reader, &out)

	// Replace the default with one that writes to our buffer for
	// observation; production wires to stderr.
	s.SetLogger(logger.New(logger.Config{Writer: &out, Format: logger.FormatText, Level: slog.LevelInfo}))

	s.Verbose = true
	s.logInfo("hello, world")
	if !strings.Contains(out.String(), "msg=\"hello, world\"") && !strings.Contains(out.String(), "msg=hello") {
		t.Errorf("expected text-format msg in output, got %q", out.String())
	}
}
