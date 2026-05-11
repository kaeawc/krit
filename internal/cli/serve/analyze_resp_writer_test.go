package serve

import (
	"bytes"
	"errors"
	"testing"
)

// TestAnalyzeRespWriter_HeadDeferredUntilFirstByte covers the
// envelope-head-flush invariant: nothing reaches the underlying
// writer until the first non-empty payload arrives. Empty Writes are
// no-ops so a pre-OutputPhase pipeline error can still rewrite the
// envelope as {"ok":false,...}.
func TestAnalyzeRespWriter_HeadDeferredUntilFirstByte(t *testing.T) {
	var out bytes.Buffer
	w := &analyzeRespWriter{out: &out, head: []byte("HEAD:")}

	if _, err := w.Write(nil); err != nil {
		t.Fatalf("empty Write err: %v", err)
	}
	if out.Len() != 0 {
		t.Errorf("empty Write should not flush head; got %q", out.String())
	}
	if w.headWritten {
		t.Error("headWritten should remain false on empty Write")
	}

	if _, err := w.Write([]byte("body")); err != nil {
		t.Fatalf("Write body: %v", err)
	}
	if got := out.String(); got != "HEAD:body" {
		t.Errorf("first non-empty Write should flush head + body; got %q", got)
	}
	if !w.headWritten {
		t.Error("headWritten should be true after first non-empty Write")
	}
}

// TestAnalyzeRespWriter_StripsTrailingNewline is the load-bearing
// case: json.Encoder appends a trailing '\n' to its value, but the
// daemon wire protocol is line-delimited so an internal '\n' would
// truncate the response on the client side. The writer must hold
// the trailing newline back and silently drop it when the stream
// ends.
func TestAnalyzeRespWriter_StripsTrailingNewline(t *testing.T) {
	var out bytes.Buffer
	w := &analyzeRespWriter{out: &out, head: []byte("HEAD:")}

	if _, err := w.Write([]byte("payload\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if got := out.String(); got != "HEAD:payload" {
		t.Errorf("trailing newline should be stripped; got %q", got)
	}
	if !w.heldNewline {
		t.Error("heldNewline should be true after trailing-\\n payload")
	}
}

// TestAnalyzeRespWriter_FlushesHeldNewlineBetweenChunks confirms a
// held newline is reissued before subsequent bytes when the stream
// continues. Only the *final* held newline is dropped — internal
// newlines (mid-stream) survive.
func TestAnalyzeRespWriter_FlushesHeldNewlineBetweenChunks(t *testing.T) {
	var out bytes.Buffer
	w := &analyzeRespWriter{out: &out, head: []byte("HEAD:")}

	if _, err := w.Write([]byte("a\n")); err != nil {
		t.Fatalf("Write 1: %v", err)
	}
	if _, err := w.Write([]byte("b")); err != nil {
		t.Fatalf("Write 2: %v", err)
	}
	if got := out.String(); got != "HEAD:a\nb" {
		t.Errorf("internal newline should be flushed before next chunk; got %q", got)
	}
}

// TestAnalyzeRespWriter_PropagatesWriteError surfaces underlying
// writer errors so dispatchAndWrite can close a broken connection
// instead of silently dropping bytes.
func TestAnalyzeRespWriter_PropagatesWriteError(t *testing.T) {
	sentinel := errors.New("boom")
	w := &analyzeRespWriter{out: errWriter{err: sentinel}, head: []byte("HEAD:")}

	_, err := w.Write([]byte("body"))
	if !errors.Is(err, sentinel) {
		t.Errorf("expected sentinel error; got %v", err)
	}
}

type errWriter struct{ err error }

func (e errWriter) Write(_ []byte) (int, error) { return 0, e.err }
