package jsonrpc

import (
	"errors"
	"log/slog"
	"sync"
	"testing"

	"github.com/kaeawc/krit/internal/logger"
)

// failingWriter is an io.Writer that always errors. Used to drive
// WriteMessage's write-header / write-body failure paths.
type failingWriter struct{ err error }

func (w failingWriter) Write([]byte) (int, error) { return 0, w.err }

// TestWriteMessageMarshalErrorLogged verifies a marshal failure on an
// unencodable value routes through the package Logger at Error level.
// chan types are not JSON-encodable.
func TestWriteMessageMarshalErrorLogged(t *testing.T) {
	prev := pkgLog
	cap := logger.NewCapture(slog.LevelDebug)
	SetLogger(cap)
	t.Cleanup(func() { SetLogger(prev) })

	var buf failingWriter // not actually written to (marshal fails first)
	var mu sync.Mutex
	WriteMessage(buf, &mu, make(chan int))

	if !cap.HasMessage("marshal error") {
		t.Fatalf("expected marshal error record, got %+v", cap.Records())
	}
	if got := cap.FilterLevel(slog.LevelError); len(got) != 1 {
		t.Errorf("expected exactly 1 Error record, got %d: %+v", len(got), cap.Records())
	}
}

// TestWriteMessageWriteHeaderErrorLogged verifies that a transport
// failure during the Content-Length header write routes through the
// package Logger at Error level.
func TestWriteMessageWriteHeaderErrorLogged(t *testing.T) {
	prev := pkgLog
	cap := logger.NewCapture(slog.LevelDebug)
	SetLogger(cap)
	t.Cleanup(func() { SetLogger(prev) })

	w := failingWriter{err: errors.New("connection reset")}
	var mu sync.Mutex
	WriteMessage(w, &mu, map[string]string{"hello": "world"})

	if !cap.HasMessage("write header error") {
		t.Fatalf("expected write header error record, got %+v", cap.Records())
	}
}
