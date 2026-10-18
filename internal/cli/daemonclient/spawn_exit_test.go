//go:build !windows

package daemonclient

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/daemon"
)

func tmpDir(t *testing.T) string {
	t.Helper()
	d, err := os.MkdirTemp("/tmp", "krit-spawn-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(d) })
	return d
}

// writeScript writes an executable shell script and returns its path.
func writeScript(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(tmpDir(t), "fake-krit")
	if err := os.WriteFile(p, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

// A spawned daemon that exits before binding its socket must fail
// EnsureRunning promptly, not after the full WaitTimeout.
func TestEnsureRunning_FailsFastWhenDaemonExits(t *testing.T) {
	root := tmpDir(t)
	log := filepath.Join(root, ".krit", "daemon.log")
	bin := writeScript(t, "echo boom >&2; exit 1")
	start := time.Now()
	c, err := EnsureRunning(root, SpawnOptions{Binary: bin, WaitTimeout: 20 * time.Second, LogPath: log})
	if err == nil {
		t.Fatalf("expected error, got client %v", c)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("waited %s after the daemon exited; want fail-fast (err: %v)", elapsed, err)
	}
	for _, want := range []string{"exited before becoming reachable", "exit status 1", log, "boom"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q missing %q", err, want)
		}
	}
}

func TestEnsureRunning_NoLogPathOmitsHint(t *testing.T) {
	_, err := EnsureRunning(tmpDir(t), SpawnOptions{Binary: "/bin/false", WaitTimeout: 20 * time.Second})
	if err == nil || strings.Contains(err.Error(), "(see") {
		t.Fatalf("want error without a log hint, got %v", err)
	}
}

// A child that exits 0 (for example it lost the single-instance race to a
// daemon that is in fact serving) is not a failure when a daemon is reachable.
func TestEnsureRunning_ChildExitsZeroButDaemonReachable(t *testing.T) {
	root := tmpDir(t)
	srv := daemon.NewServer(daemon.DefaultSocketPath(root))
	if err := srv.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(srv.Stop)
	// Discover finds it first, so force the spawn path through the waiter.
	c, err := waitForDaemon(daemon.DefaultSocketPath(root), root, 5*time.Second, 10*time.Millisecond, closedExit(nil), "")
	if err != nil || c == nil {
		t.Fatalf("want reachable client, got %v, %v", c, err)
	}
	// And with nothing listening, a clean exit is still reported as an error.
	_, err = EnsureRunning(tmpDir(t), SpawnOptions{Binary: "/bin/true", WaitTimeout: 20 * time.Second})
	if err == nil || !strings.Contains(err.Error(), "exited before becoming reachable") {
		t.Fatalf("exit 0 without a daemon must still fail, got %v", err)
	}
}

func closedExit(err error) <-chan error {
	ch := make(chan error, 1)
	ch <- err
	return ch
}

// A child that stays alive and binds the socket returns a working client,
// and receives the parent's resolved socket via --socket.
func TestEnsureRunning_ChildBindsSuccessfully(t *testing.T) {
	root := tmpDir(t)
	argsFile := filepath.Join(tmpDir(t), "args")
	bin := writeScript(t, `echo "$@" > `+argsFile+`; sleep 5`)
	want, err := daemon.ResolveSocketPath(root)
	if err != nil {
		t.Fatal(err)
	}
	srvErr := make(chan error, 1)
	go func() {
		time.Sleep(150 * time.Millisecond)
		srv := daemon.NewServer(want)
		srvErr <- srv.Start(context.Background())
		t.Cleanup(srv.Stop)
	}()
	c, err := EnsureRunning(root, SpawnOptions{Binary: bin, WaitTimeout: 10 * time.Second})
	if err != nil || c == nil {
		t.Fatalf("EnsureRunning: %v, %v", c, err)
	}
	if e := <-srvErr; e != nil {
		t.Fatal(e)
	}
	if c.SocketPath() != want {
		t.Fatalf("client socket %q, want %q", c.SocketPath(), want)
	}
	data, _ := os.ReadFile(argsFile)
	if got := string(data); !strings.Contains(got, "serve --root "+root+" --socket "+want) {
		t.Fatalf("child args %q do not pass --socket %s", got, want)
	}
}

// startRootServer serves Status reporting root at sock.
func startRootServer(t *testing.T, sock, root string) {
	t.Helper()
	srv := daemon.NewServer(sock)
	srv.Register(daemon.VerbStatus, func(_ context.Context, _ json.RawMessage) (any, error) {
		return daemon.StatusResult{Ready: true, Root: root}, nil
	})
	if err := srv.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(srv.Stop)
}

func longRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join(tmpDir(t), strings.Repeat("deeply-nested-checkout-", 6), "project")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

// Two roots whose fallback socket collides must not be confused: a daemon
// answering on our fallback socket for a different root is refused and no
// new daemon is spawned over it.
func TestClient_RefusesDaemonServingDifferentRoot(t *testing.T) {
	t.Setenv("TMPDIR", tmpDir(t))
	t.Setenv("XDG_RUNTIME_DIR", "")
	root := longRoot(t)
	sock, err := daemon.ResolveSocketPath(root)
	if err != nil || !daemon.IsFallbackSocket(sock) {
		t.Fatalf("expected fallback socket, got %q, %v", sock, err)
	}
	startRootServer(t, sock, "/some/other/project")

	if c, ok := Discover(root); ok || c != nil {
		t.Fatal("Discover must treat a different-root daemon as absent")
	}
	marker := filepath.Join(tmpDir(t), "spawned")
	bin := writeScript(t, "touch "+marker)
	_, err = EnsureRunning(root, SpawnOptions{Binary: bin, WaitTimeout: 5 * time.Second})
	if !errors.Is(err, ErrRootMismatch) {
		t.Fatalf("EnsureRunning err = %v, want ErrRootMismatch", err)
	}
	_, ok, err := EnsureCompatible(root, SpawnOptions{Binary: bin, AutoRestart: true, WaitTimeout: 5 * time.Second})
	if ok || !errors.Is(err, ErrRootMismatch) {
		t.Fatalf("EnsureCompatible = ok %v, err %v; want ErrRootMismatch", ok, err)
	}
	time.Sleep(200 * time.Millisecond)
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("a daemon was spawned over the colliding one")
	}
}

func TestClient_AcceptsDaemonServingSameRoot(t *testing.T) {
	t.Setenv("TMPDIR", tmpDir(t))
	t.Setenv("XDG_RUNTIME_DIR", "")
	root := longRoot(t)
	sock, err := daemon.ResolveSocketPath(root)
	if err != nil {
		t.Fatal(err)
	}
	startRootServer(t, sock, root)
	if _, ok := Discover(root); !ok {
		t.Fatal("Discover must accept a daemon serving the same root")
	}
}
