//go:build !windows

package daemonclient

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
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
	_, err := EnsureRunning(tmpDir(t), SpawnOptions{Binary: writeScript(t, "exit 1"), WaitTimeout: 20 * time.Second})
	if err == nil || strings.Contains(err.Error(), "(see") {
		t.Fatalf("want error without a log hint, got %v", err)
	}
}

// A child that exits 0 (for example a daemon that came up concurrently
// was already serving) is not a failure when a daemon is reachable.
func TestEnsureRunning_ChildExitsZeroButDaemonReachable(t *testing.T) {
	root := tmpDir(t)
	startRootServer(t, daemon.DefaultSocketPath(root), root)
	// Discover finds it first, so force the spawn path through the waiter.
	c, err := waitForDaemon(daemon.DefaultSocketPath(root), root, 5*time.Second, 10*time.Millisecond, closedExit(nil), "", nil)
	if err != nil || c == nil {
		t.Fatalf("want reachable client, got %v, %v", c, err)
	}
	// And with nothing listening, a clean exit is still reported as an error.
	_, err = EnsureRunning(tmpDir(t), SpawnOptions{Binary: writeScript(t, "exit 0"), WaitTimeout: 20 * time.Second})
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
	srv := daemon.NewServer(want)
	srv.Register(daemon.VerbStatus, func(_ context.Context, _ json.RawMessage) (any, error) {
		return daemon.StatusResult{Ready: true, Root: root}, nil
	})
	t.Cleanup(srv.Stop) // registered before the goroutine can signal
	srvErr := make(chan error, 1)
	go func() {
		time.Sleep(150 * time.Millisecond)
		srvErr <- srv.Start(context.Background())
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

// A child that exits with the "repo lock held" status means another daemon is
// coming up for this repo; the client keeps waiting for it instead of failing
// or spawning more.
func TestEnsureRunning_ChildAlreadyRunningWaitsForWinner(t *testing.T) {
	root := tmpDir(t)
	bin := writeScript(t, "exit 2")
	want := daemon.DefaultSocketPath(root)
	srv := daemon.NewServer(want)
	srv.Register(daemon.VerbStatus, func(_ context.Context, _ json.RawMessage) (any, error) {
		return daemon.StatusResult{Ready: true, Root: root}, nil
	})
	t.Cleanup(srv.Stop)
	go func() {
		time.Sleep(400 * time.Millisecond)
		_ = srv.Start(context.Background())
	}()
	c, err := EnsureRunning(root, SpawnOptions{Binary: bin, WaitTimeout: 10 * time.Second})
	if err != nil || c == nil {
		t.Fatalf("EnsureRunning = %v, %v; want the winner's client", c, err)
	}
}

// A stale or committed pointer must never hide a live conventional daemon:
// Discover finds it and nothing is spawned.
func TestDiscover_ShortRootIgnoresBogusPointer(t *testing.T) {
	t.Setenv("TMPDIR", tmpDir(t))
	t.Setenv("XDG_RUNTIME_DIR", "")
	root := tmpDir(t)
	if r, err := filepath.EvalSymlinks(root); err == nil {
		root = r
	}
	startRootServer(t, daemon.ConventionalSocketPath(root), root)
	bogus := filepath.Join(tmpDir(t), "krit", "0123456789abcdef.sock")
	if err := daemon.WriteSocketPointer(daemon.PointerPath(root), bogus); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(tmpDir(t), "spawned")
	bin := writeScript(t, "touch "+marker)
	c, err := EnsureRunning(root, SpawnOptions{Binary: bin, WaitTimeout: 2 * time.Second})
	if err != nil || c == nil || c.SocketPath() != daemon.ConventionalSocketPath(root) {
		t.Fatalf("EnsureRunning = %v, %v; want the conventional daemon", c, err)
	}
	time.Sleep(200 * time.Millisecond)
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("spawned a daemon despite a live one")
	}
}

// Even the conventional socket is root-checked: .krit can be a symlink
// committed to a repository.
func TestDiscover_ConventionalSocketServingOtherRootIsRefused(t *testing.T) {
	root := tmpDir(t)
	startRootServer(t, daemon.ConventionalSocketPath(root), "/some/other/project")
	if _, ok := Discover(root); ok {
		t.Fatal("Discover must refuse a conventional-socket daemon serving another root")
	}
	if _, err := EnsureRunning(root, SpawnOptions{Binary: writeScript(t, "exit 1")}); !errors.Is(err, ErrRootMismatch) {
		t.Fatalf("EnsureRunning err = %v, want ErrRootMismatch", err)
	}
}

// Helper process: listens on $KRIT_TEST_SOCK (run as another uid).
func TestHelperForeignListener(t *testing.T) {
	sock := os.Getenv("KRIT_TEST_SOCK")
	if sock == "" {
		t.Skip("helper process only")
	}
	l, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	_ = os.Chmod(sock, 0o666)
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			_ = c.Close()
		}
	}()
	time.Sleep(30 * time.Second)
}

// A daemon owned by another user must stop EnsureRunning/EnsureCompatible
// with a clear message instead of spawning over its socket.
func TestEnsure_ForeignUIDDaemonIsNotSpawnedOver(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("needs root to run a listener as another uid")
	}
	root := tmpDir(t)
	if err := os.Chmod(root, 0o755); err != nil { // uid 54321 must traverse it
		t.Fatal(err)
	}
	krit := filepath.Join(root, ".krit")
	if err := os.MkdirAll(krit, 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(krit, 0o777); err != nil {
		t.Fatal(err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	bindir := tmpDir(t)
	if err := os.Chmod(bindir, 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(bindir, "daemonclient.test")
	if err := os.WriteFile(bin, data, 0o755); err != nil {
		t.Fatal(err)
	}
	sock := daemon.ConventionalSocketPath(root)
	cmd := exec.Command(bin, "-test.run=^TestHelperForeignListener$")
	cmd.Env = append(os.Environ(), "KRIT_TEST_SOCK="+sock)
	cmd.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: 54321, Gid: 54321}}
	if err := cmd.Start(); err != nil {
		t.Skipf("cannot run as another uid: %v", err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(sock); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("foreign listener did not come up")
		}
		time.Sleep(10 * time.Millisecond)
	}

	if _, ok := Discover(root); ok {
		t.Fatal("Discover must report absent for a foreign daemon")
	}
	marker := filepath.Join(tmpDir(t), "spawned")
	spawn := writeScript(t, "touch "+marker)
	_, err = EnsureRunning(root, SpawnOptions{Binary: spawn, WaitTimeout: 2 * time.Second})
	if !errors.Is(err, daemon.ErrPeerUID) || !strings.Contains(err.Error(), "uid 54321") || !strings.Contains(err.Error(), "--no-daemon") {
		t.Fatalf("EnsureRunning err = %v, want actionable ErrPeerUID", err)
	}
	if _, _, err := EnsureCompatible(root, SpawnOptions{Binary: spawn, AutoRestart: true}); !errors.Is(err, daemon.ErrPeerUID) {
		t.Fatalf("EnsureCompatible err = %v, want ErrPeerUID", err)
	}
	time.Sleep(200 * time.Millisecond)
	if _, err := os.Stat(marker); err == nil {
		t.Fatal("spawned over a foreign daemon")
	}
}

// A child that loses the repo lock to a daemon that is shutting down (the
// version-mismatch restart window) must be re-spawned once the lock is
// released, instead of waiting out the timeout for a daemon that never comes.
func TestEnsureRunning_RespawnsAfterLockHolderExits(t *testing.T) {
	root := tmpDir(t)
	if r, err := filepath.EvalSymlinks(root); err == nil {
		root = r
	}
	lock, err := daemon.AcquireRepoLock(root)
	if err != nil {
		t.Fatal(err)
	}
	first := filepath.Join(tmpDir(t), "first")
	respawned := filepath.Join(tmpDir(t), "respawned")
	// First spawn: loses the lock. Later spawns: record and stay alive.
	bin := writeScript(t, `if [ ! -e `+first+` ]; then touch `+first+`; exit 2; fi; touch `+respawned+`; sleep 5`)
	go func() {
		time.Sleep(300 * time.Millisecond)
		_ = lock.Close() // the incumbent finishes shutting down
	}()
	sock := daemon.DefaultSocketPath(root)
	srv := daemon.NewServer(sock)
	srv.Register(daemon.VerbStatus, func(_ context.Context, _ json.RawMessage) (any, error) {
		return daemon.StatusResult{Ready: true, Root: root}, nil
	})
	t.Cleanup(srv.Stop)
	go func() {
		// Stand in for the respawned daemon binding its socket.
		deadline := time.Now().Add(8 * time.Second)
		for time.Now().Before(deadline) {
			if _, err := os.Stat(respawned); err == nil {
				_ = srv.Start(context.Background())
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()
	c, err := EnsureRunning(root, SpawnOptions{Binary: bin, WaitTimeout: 8 * time.Second})
	if err != nil || c == nil {
		t.Fatalf("EnsureRunning = %v, %v; want a client after respawn", c, err)
	}
	if _, err := os.Stat(respawned); err != nil {
		t.Fatalf("daemon was not re-spawned after the lock was released: %v", err)
	}
}

// logHint reads only a bounded tail of a large append-only log.
func TestLogHint_ReadsBoundedTail(t *testing.T) {
	p := filepath.Join(tmpDir(t), "daemon.log")
	big := strings.Repeat("old noise line\n", 50000) + "one\ntwo\nthree\nlast error\n"
	if err := os.WriteFile(p, []byte(big), 0o644); err != nil {
		t.Fatal(err)
	}
	got := logHint(p)
	if !strings.HasSuffix(got, ": two; three; last error)") {
		t.Fatalf("logHint = %q", got)
	}
	tail, err := readTail(p, logHintBytes)
	if err != nil || int64(len(tail)) > logHintBytes {
		t.Fatalf("readTail returned %d bytes, err %v", len(tail), err)
	}
}
