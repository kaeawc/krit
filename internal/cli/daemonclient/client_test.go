package daemonclient

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/daemon"
)

// startEchoDaemon spins up a minimal daemon at a short tmp socket so
// we can exercise the client. The daemon registers a passthrough
// echo verb for AnalyzeBuffer and the standard Status / Shutdown
// verbs.
func startEchoDaemon(t *testing.T) (string, *daemon.Server) {
	t.Helper()
	socketDir, err := os.MkdirTemp("/tmp", "krit-cli-")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(socketDir) })
	socket := filepath.Join(socketDir, "d.sock")

	srv := daemon.NewServer(socket)
	srv.Register(daemon.VerbStatus, func(_ context.Context, _ json.RawMessage) (any, error) {
		return daemon.StatusResult{Ready: true, Files: 7}, nil
	})
	srv.Register(daemon.VerbShutdown, func(_ context.Context, _ json.RawMessage) (any, error) {
		return map[string]bool{"ok": true}, nil
	})
	srv.Register(daemon.VerbAnalyzeBuffer, func(_ context.Context, raw json.RawMessage) (any, error) {
		var args daemon.AnalyzeBufferArgs
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, err
		}
		return daemon.AnalyzeBufferResult{
			Findings: json.RawMessage(`{"path":"` + args.Path + `","len":` + itoa(len(args.Content)) + `}`),
			CacheHit: false,
		}, nil
	})
	if err := srv.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(srv.Stop)

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if daemon.Available(socket) {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	return socketDir, srv
}

// itoa is the smallest possible integer-to-string used to build a
// JSON test fixture without pulling strconv into a hot test path.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func TestDiscover_FindsRunningDaemon(t *testing.T) {
	socketDir, srv := startEchoDaemon(t)
	// Place a fake .krit/daemon.sock symlink so Discover with the
	// project root finds the running daemon.
	root, err := os.MkdirTemp("/tmp", "krit-root-")
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	if err := os.MkdirAll(filepath.Join(root, ".krit"), 0o755); err != nil {
		t.Fatalf("mkdir krit: %v", err)
	}
	expected := daemon.DefaultSocketPath(root)
	if err := os.Symlink(filepath.Join(socketDir, "d.sock"), expected); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	c, ok := Discover(root)
	if !ok {
		t.Fatal("Discover returned no client")
	}
	if c.SocketPath() != expected {
		t.Errorf("socket path mismatch: got %q, want %q", c.SocketPath(), expected)
	}
	_ = srv // keep server alive for cleanup
}

func TestDiscover_NoDaemonReturnsFalse(t *testing.T) {
	root, err := os.MkdirTemp("/tmp", "krit-root-")
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	if c, ok := Discover(root); ok {
		t.Fatalf("Discover returned a client (%q) when no daemon is running", c.SocketPath())
	}
}

func TestClient_AnalyzeBufferRoundTrip(t *testing.T) {
	socketDir, _ := startEchoDaemon(t)
	c := &Client{socketPath: filepath.Join(socketDir, "d.sock")}
	got, err := c.AnalyzeBuffer(daemon.AnalyzeBufferArgs{Path: "Foo.kt", Content: "fun f()"})
	if err != nil {
		t.Fatalf("AnalyzeBuffer: %v", err)
	}
	var payload struct {
		Path string `json:"path"`
		Len  int    `json:"len"`
	}
	if err := json.Unmarshal(got.Findings, &payload); err != nil {
		t.Fatalf("decode findings: %v", err)
	}
	if payload.Path != "Foo.kt" || payload.Len != len("fun f()") {
		t.Errorf("echo round-trip mismatch: %+v", payload)
	}
}

func TestClient_StatusRoundTrip(t *testing.T) {
	socketDir, _ := startEchoDaemon(t)
	c := &Client{socketPath: filepath.Join(socketDir, "d.sock")}
	st, err := c.Status()
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !st.Ready || st.Files != 7 {
		t.Errorf("status mismatch: %+v", st)
	}
}

// startEchoDaemonWithStatus is startEchoDaemon plus a Status verb that
// returns the configured BinaryHash so tests can simulate
// version-mismatch handshakes.
func startEchoDaemonWithStatus(t *testing.T, daemonHash string) string {
	t.Helper()
	socketDir, err := os.MkdirTemp("/tmp", "krit-cli-")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(socketDir) })
	socket := filepath.Join(socketDir, "d.sock")

	srv := daemon.NewServer(socket)
	srv.Register(daemon.VerbStatus, func(_ context.Context, _ json.RawMessage) (any, error) {
		return daemon.StatusResult{Ready: true, BinaryHash: daemonHash}, nil
	})
	srv.Register(daemon.VerbShutdown, func(_ context.Context, _ json.RawMessage) (any, error) {
		return map[string]bool{"ok": true}, nil
	})
	if err := srv.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(srv.Stop)
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if daemon.Available(socket) {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	return socket
}

// linkSocket creates root/.krit/daemon.sock pointing at socket so
// Discover can find it.
func linkSocket(t *testing.T, root, socket string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, ".krit"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Symlink(socket, daemon.DefaultSocketPath(root)); err != nil {
		t.Fatalf("symlink: %v", err)
	}
}

func TestEnsureCompatible_MatchingHashKeepsClient(t *testing.T) {
	root, err := os.MkdirTemp("/tmp", "krit-root-")
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	socket := startEchoDaemonWithStatus(t, currentBinaryHash())
	linkSocket(t, root, socket)

	c, ok, err := EnsureCompatible(root, SpawnOptions{})
	if err != nil {
		t.Fatalf("EnsureCompatible: %v", err)
	}
	if !ok || c == nil {
		t.Fatalf("expected client retained on hash match, got ok=%v c=%v", ok, c)
	}
}

func TestEnsureCompatible_MismatchShutsDownNoRestart(t *testing.T) {
	root, err := os.MkdirTemp("/tmp", "krit-root-")
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	socket := startEchoDaemonWithStatus(t, "deadbeefdeadbeefdeadbeefdeadbeef")
	linkSocket(t, root, socket)

	c, ok, err := EnsureCompatible(root, SpawnOptions{})
	if err != nil {
		t.Fatalf("EnsureCompatible: %v", err)
	}
	if ok || c != nil {
		t.Fatalf("expected nil client on mismatch+no-restart, got ok=%v", ok)
	}
}

func TestEnsureCompatible_MissingDaemonNoSpawn(t *testing.T) {
	root, err := os.MkdirTemp("/tmp", "krit-root-")
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	c, ok, err := EnsureCompatible(root, SpawnOptions{})
	if err != nil {
		t.Fatalf("EnsureCompatible: %v", err)
	}
	if ok || c != nil {
		t.Fatalf("expected nil client when no daemon and no autoRestart, got ok=%v", ok)
	}
}

func TestClient_NilSafety(t *testing.T) {
	var c *Client
	if c.SocketPath() != "" {
		t.Errorf("nil socketPath should be empty")
	}
	if _, err := c.AnalyzeBuffer(daemon.AnalyzeBufferArgs{}); err == nil {
		t.Errorf("AnalyzeBuffer on nil client should error")
	}
	if _, err := c.Status(); err == nil {
		t.Errorf("Status on nil client should error")
	}
	if err := c.Shutdown(); err != nil {
		t.Errorf("Shutdown on nil client should be a no-op, got %v", err)
	}
}
