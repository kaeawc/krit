package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

func startTestServer(t *testing.T) *Server {
	t.Helper()
	socket := filepath.Join(t.TempDir(), "d.sock")
	srv := NewServer(socket)
	srv.Register("echo", func(_ context.Context, raw json.RawMessage) (any, error) {
		var v any
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &v); err != nil {
				return nil, err
			}
		}
		return v, nil
	})
	srv.Register("boom", func(_ context.Context, _ json.RawMessage) (any, error) {
		return nil, errors.New("kaboom")
	})
	if err := srv.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(srv.Stop)
	// Wait briefly for socket to be reachable.
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if Available(socket) {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	return srv
}

func TestServerRoundTrip(t *testing.T) {
	srv := startTestServer(t)

	var out string
	if err := Call(srv.SocketPath(), "echo", "hi", &out); err != nil {
		t.Fatalf("call: %v", err)
	}
	if out != "hi" {
		t.Fatalf("echo returned %q", out)
	}
}

func TestServerHandlerError(t *testing.T) {
	srv := startTestServer(t)
	if err := Call(srv.SocketPath(), "boom", nil, nil); err == nil || err.Error() != "kaboom" {
		t.Fatalf("expected kaboom, got %v", err)
	}
}

func TestServerUnknownVerb(t *testing.T) {
	srv := startTestServer(t)
	if err := Call(srv.SocketPath(), "no-such-verb", nil, nil); err == nil {
		t.Fatalf("expected error")
	}
}

func TestAvailableMissing(t *testing.T) {
	if Available(filepath.Join(t.TempDir(), "nope.sock")) {
		t.Fatalf("Available should be false for missing socket")
	}
}

func TestShutdownClosesServer(t *testing.T) {
	socket := filepath.Join(t.TempDir(), "d.sock")
	srv := NewServer(socket)
	srv.Register(VerbShutdown, func(_ context.Context, _ json.RawMessage) (any, error) {
		return map[string]bool{"ok": true}, nil
	})
	if err := srv.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := Call(socket, VerbShutdown, nil, nil); err != nil {
		t.Fatalf("shutdown call: %v", err)
	}
	select {
	case <-time.After(2 * time.Second):
		t.Fatalf("server did not stop")
	case <-waitClosed(srv):
	}
}

func waitClosed(s *Server) <-chan struct{} {
	ch := make(chan struct{})
	go func() {
		s.Wait()
		close(ch)
	}()
	return ch
}
