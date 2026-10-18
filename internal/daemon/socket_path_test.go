//go:build !windows

package daemon

import (
	"context"
	"errors"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// isolateTmp points TMPDIR at a short private dir and clears
// XDG_RUNTIME_DIR so fallback sockets never touch the real /tmp/krit-<uid>.
func isolateTmp(t *testing.T) string {
	t.Helper()
	tmp := shortTempDir(t)
	t.Setenv("TMPDIR", tmp)
	t.Setenv("XDG_RUNTIME_DIR", "")
	return tmp
}

func longRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join(shortTempDir(t), strings.Repeat("deeply-nested-checkout-", 6), "project")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestDefaultSocketPath_ShortRootUsesKritDir(t *testing.T) {
	isolateTmp(t)
	root := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	if got, want := DefaultSocketPath(root), filepath.Join(root, ".krit", "daemon.sock"); got != want {
		t.Fatalf("DefaultSocketPath(%q) = %q, want %q", root, got, want)
	}
}

// rootWithSocketLen returns a root dir whose conventional socket path is
// exactly n bytes.
func rootWithSocketLen(t *testing.T, n int) string {
	t.Helper()
	base := shortTempDir(t)
	if r, err := filepath.EvalSymlinks(base); err == nil {
		base = r
	}
	suffix := string(filepath.Separator) + ".krit" + string(filepath.Separator) + "daemon.sock"
	pad := n - len(base) - len(suffix) - 1 // -1 for the separator before the pad dir
	if pad < 1 {
		t.Fatalf("base too long for %d", n)
	}
	root := filepath.Join(base, strings.Repeat("a", pad))
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := len(filepath.Join(root, ".krit", "daemon.sock")); got != n {
		t.Fatalf("built %d-byte path, want %d", got, n)
	}
	return root
}

func TestDefaultSocketPath_Boundary(t *testing.T) {
	isolateTmp(t)
	at := rootWithSocketLen(t, maxSocketPath)
	if got := DefaultSocketPath(at); got != filepath.Join(at, ".krit", "daemon.sock") {
		t.Fatalf("%d-byte path must stay conventional, got %s", maxSocketPath, got)
	}
	over := rootWithSocketLen(t, maxSocketPath+1)
	got := DefaultSocketPath(over)
	if got == filepath.Join(over, ".krit", "daemon.sock") || !IsFallbackSocket(got) || len(got) > maxSocketPath {
		t.Fatalf("%d-byte path must use a fitting fallback, got %s", maxSocketPath+1, got)
	}
}

// A project nested deep enough that <root>/.krit/daemon.sock exceeds
// sun_path used to make bind fail with EINVAL.
func TestDefaultSocketPath_LongRootFallsBackAndBinds(t *testing.T) {
	isolateTmp(t)
	root := longRoot(t)
	sock := DefaultSocketPath(root)
	if len(sock) > maxSocketPath || !IsFallbackSocket(sock) {
		t.Fatalf("bad fallback socket path (%d bytes): %s", len(sock), sock)
	}
	if sock != DefaultSocketPath(root) {
		t.Fatalf("fallback path is not stable for the same root")
	}
	if other := DefaultSocketPath(root + "2"); other == sock {
		t.Fatalf("distinct roots share fallback socket %s", sock)
	}
	srv := NewServer(sock)
	if err := srv.Start(context.Background()); err != nil {
		t.Fatalf("Start on fallback socket: %v", err)
	}
	t.Cleanup(srv.Stop)
	if !Available(sock) {
		t.Fatalf("daemon not reachable at %s", sock)
	}
	if fi, err := os.Stat(filepath.Dir(sock)); err != nil || fi.Mode().Perm() != 0o700 {
		t.Fatalf("fallback dir not 0700: %v %v", fi, err)
	}
	if fi, err := os.Stat(sock); err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("socket not 0600: %v %v", fi, err)
	}
}

func TestResolveSocketPath_RelativeRootMatchesAbsolute(t *testing.T) {
	isolateTmp(t)
	root := longRoot(t)
	t.Chdir(root)
	rel, err := ResolveSocketPath(".")
	if err != nil {
		t.Fatal(err)
	}
	abs, err := ResolveSocketPath(root)
	if err != nil {
		t.Fatal(err)
	}
	if rel != abs {
		t.Fatalf("relative %q and absolute %q roots disagree", rel, abs)
	}
}

func TestResolveSocketPath_LongTMPDIRFallsBackToSlashTmp(t *testing.T) {
	long := filepath.Join(shortTempDir(t), strings.Repeat("l", 120))
	if err := os.MkdirAll(long, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", long)
	t.Setenv("XDG_RUNTIME_DIR", "")
	sys := redirectSystemTmp(t)
	p, err := ResolveSocketPath(longRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(p) > maxSocketPath || !strings.HasPrefix(p, filepath.Join(sys, "krit-")) {
		t.Fatalf("path %q should live under the system tmp %s and fit the limit", p, sys)
	}
}

func TestResolveSocketPath_PrefersXDGRuntimeDir(t *testing.T) {
	isolateTmp(t)
	xdg := shortTempDir(t)
	t.Setenv("XDG_RUNTIME_DIR", xdg)
	p, err := ResolveSocketPath(longRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(p, filepath.Join(xdg, "krit")+string(filepath.Separator)) {
		t.Fatalf("path %q should be under XDG_RUNTIME_DIR", p)
	}
}

func TestResolveSocketPath_SkipsUnsafeDir(t *testing.T) {
	isolateTmp(t)
	xdg := shortTempDir(t)
	if err := os.Symlink(shortTempDir(t), filepath.Join(xdg, "krit")); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_RUNTIME_DIR", xdg)
	p, err := ResolveSocketPath(longRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	if strings.HasPrefix(p, xdg) {
		t.Fatalf("unsafe XDG dir used: %s", p)
	}
}

func TestServer_StopRemovesSocketAndPointer(t *testing.T) {
	isolateTmp(t)
	root := longRoot(t)
	sock := DefaultSocketPath(root)
	srv := NewServer(sock)
	srv.PointerPath = PointerPath(root)
	if err := srv.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := LocateSocketPath(root); got != sock {
		t.Fatalf("LocateSocketPath = %q; want %q", got, sock)
	}
	if got, ok := readSocketPointer(root); !ok || got != sock {
		t.Fatalf("pointer = %q, %v; want %q", got, ok, sock)
	}
	if fi, err := os.Stat(srv.PointerPath); err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("pointer file: %v %v", fi, err)
	}
	srv.Stop()
	if _, err := os.Lstat(sock); !os.IsNotExist(err) {
		t.Fatalf("socket still present after Stop: %v", err)
	}
	if _, err := os.Lstat(srv.PointerPath); !os.IsNotExist(err) {
		t.Fatalf("pointer still present after Stop: %v", err)
	}
}

// startServerAt starts a bare server (no verbs beyond the built-ins) at sock.
func startServerAt(t *testing.T, sock string) *Server {
	t.Helper()
	srv := NewServer(sock)
	if err := srv.Start(context.Background()); err != nil {
		t.Fatalf("start %s: %v", sock, err)
	}
	t.Cleanup(srv.Stop)
	return srv
}

func TestLocateSocketPath_LivePointerWinsOverDerivation(t *testing.T) {
	isolateTmp(t)
	root := longRoot(t)
	// The daemon was started under a different TMPDIR than the client has.
	elsewhere := filepath.Join(shortTempDir(t), "krit", FallbackSocketName(root))
	startServerAt(t, elsewhere)
	if err := WriteSocketPointer(PointerPath(root), elsewhere); err != nil {
		t.Fatal(err)
	}
	if got := LocateSocketPath(root); got != elsewhere {
		t.Fatalf("LocateSocketPath = %q, want live pointer target %q", got, elsewhere)
	}
}

func TestLocateSocketPath_DeadPointerFallsThroughToDerived(t *testing.T) {
	isolateTmp(t)
	root := longRoot(t)
	derived := DefaultSocketPath(root)
	startServerAt(t, derived)
	dead := filepath.Join(shortTempDir(t), "krit", FallbackSocketName(root))
	if err := WriteSocketPointer(PointerPath(root), dead); err != nil {
		t.Fatal(err)
	}
	if got := SocketCandidates(root); len(got) != 2 || got[0] != dead || got[1] != derived {
		t.Fatalf("candidates = %v, want [pointer, derived]", got)
	}
	if got := LocateSocketPath(root); got != derived {
		t.Fatalf("LocateSocketPath = %q, want derived %q (pointer is dead)", got, derived)
	}
}

func TestReadSocketPointer_RejectsBadPointers(t *testing.T) {
	isolateTmp(t)
	root := longRoot(t)
	good := filepath.Join(shortTempDir(t), "krit", FallbackSocketName(root))
	ptr := PointerPath(root)
	write := func(sock string) {
		t.Helper()
		if err := WriteSocketPointer(ptr, sock); err != nil {
			t.Fatal(err)
		}
	}
	write(good)
	if p, ok := readSocketPointer(root); !ok || p != good {
		t.Fatalf("good pointer rejected: %q %v", p, ok)
	}
	for name, sock := range map[string]string{
		"wrong basename": filepath.Join(filepath.Dir(good), "0123456789abcdef.sock"),
		"not fallback":   "/var/run/other.sock",
		"relative":       "krit/" + FallbackSocketName(root),
	} {
		write(sock)
		if p, ok := readSocketPointer(root); ok {
			t.Errorf("%s: pointer %q must be ignored", name, p)
		}
	}
	// A symlinked pointer is ignored.
	_ = os.Remove(ptr)
	real := filepath.Join(shortTempDir(t), "real-pointer")
	if err := os.WriteFile(real, []byte(good+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, ptr); err != nil {
		t.Fatal(err)
	}
	if _, ok := readSocketPointer(root); ok {
		t.Error("symlinked pointer must be ignored")
	}
	// An oversize pointer is ignored.
	_ = os.Remove(ptr)
	if err := os.WriteFile(ptr, []byte(good+strings.Repeat("x", pointerMaxSize)), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, ok := readSocketPointer(root); ok {
		t.Error("oversize pointer must be ignored")
	}
	// A pointer owned by another user is ignored.
	if os.Getuid() == 0 {
		write(good)
		if err := os.Chown(ptr, 54321, -1); err != nil {
			t.Skipf("chown unsupported: %v", err)
		}
		if _, ok := readSocketPointer(root); ok {
			t.Error("foreign-owned pointer must be ignored")
		}
	}
}

// A short root's conventional daemon must stay discoverable whatever a stale
// or committed pointer file says.
func TestReadSocketPointer_IgnoredForShortRoot(t *testing.T) {
	isolateTmp(t)
	root := shortTempDir(t)
	good := filepath.Join(shortTempDir(t), "krit", FallbackSocketName(root))
	if err := WriteSocketPointer(PointerPath(root), good); err != nil {
		t.Fatal(err)
	}
	if _, ok := readSocketPointer(root); ok {
		t.Fatal("pointer must be ignored when the conventional path fits")
	}
	if got := SocketCandidates(root); len(got) != 1 || got[0] != ConventionalSocketPath(root) {
		t.Fatalf("candidates = %v, want only the conventional socket", got)
	}
	if err := ClearSocketPointer(root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(PointerPath(root)); !os.IsNotExist(err) {
		t.Fatalf("ClearSocketPointer left the pointer: %v", err)
	}
}

func TestRemoveSocketPointer_LeavesSuccessorAlone(t *testing.T) {
	ptr := filepath.Join(shortTempDir(t), "daemon.sock.path")
	if err := WriteSocketPointer(ptr, "/tmp/krit-0/aaaaaaaaaaaaaaaa.sock"); err != nil {
		t.Fatal(err)
	}
	if err := RemoveSocketPointer(ptr, "/tmp/krit-0/bbbbbbbbbbbbbbbb.sock"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(ptr); err != nil {
		t.Fatalf("successor's pointer was removed: %v", err)
	}
	if err := RemoveSocketPointer(ptr, "/tmp/krit-0/aaaaaaaaaaaaaaaa.sock"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(ptr); !os.IsNotExist(err) {
		t.Fatalf("own pointer not removed: %v", err)
	}
	// A symlink pointer is never followed or removed.
	real := filepath.Join(shortTempDir(t), "real")
	if err := os.WriteFile(real, []byte("/tmp/krit-0/aaaaaaaaaaaaaaaa.sock\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, ptr); err != nil {
		t.Fatal(err)
	}
	if err := RemoveSocketPointer(ptr, "/tmp/krit-0/aaaaaaaaaaaaaaaa.sock"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(ptr); err != nil {
		t.Fatalf("symlinked pointer should be left alone: %v", err)
	}
}

func TestDefaultSocketPath_DoesNotCreateDirectories(t *testing.T) {
	tmp := isolateTmp(t)
	root := longRoot(t)
	p := DefaultSocketPath(root)
	if _, err := os.Lstat(filepath.Dir(p)); !os.IsNotExist(err) {
		t.Fatalf("DefaultSocketPath created %s (err %v); lookups must be read-only", filepath.Dir(p), err)
	}
	if _, err := os.Lstat(filepath.Join(tmp, "krit-"+strconv.Itoa(os.Getuid()))); !os.IsNotExist(err) {
		t.Fatal("fallback dir exists after a read-only lookup")
	}
}

func TestResolveSocketPath_RelativeTMPDIRIsSkipped(t *testing.T) {
	t.Setenv("TMPDIR", "relative-tmp")
	t.Setenv("XDG_RUNTIME_DIR", "also/relative")
	sys := redirectSystemTmp(t)
	root := longRoot(t)
	if p := DefaultSocketPath(root); !filepath.IsAbs(p) {
		t.Fatalf("DefaultSocketPath = %q, want absolute", p)
	}
	p, err := ResolveSocketPath(root)
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(p) || !strings.HasPrefix(p, filepath.Join(sys, "krit-")) {
		t.Fatalf("ResolveSocketPath = %q, want an absolute path under %s", p, sys)
	}
}

func TestResolveSocketPath_ClearErrorWhenNothingUsable(t *testing.T) {
	// Every base is unusable: relative TMPDIR/XDG, and the system tmp's
	// krit-<uid> is a regular file.
	t.Setenv("TMPDIR", "relative-tmp")
	t.Setenv("XDG_RUNTIME_DIR", "")
	sys := redirectSystemTmp(t)
	if err := os.WriteFile(filepath.Join(sys, "krit-"+strconv.Itoa(os.Getuid())), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := ResolveSocketPath(longRoot(t))
	if err == nil || !strings.Contains(err.Error(), "no usable fallback directory") {
		t.Fatalf("want a clear no-usable-dir error, got %v", err)
	}
}

// redirectSystemTmp points the last-resort /tmp base at a private short dir.
func redirectSystemTmp(t *testing.T) string {
	t.Helper()
	d := shortTempDir(t)
	old := systemTmp
	systemTmp = d
	t.Cleanup(func() { systemTmp = old })
	return d
}

// Helper process for the foreign-uid tests: listens on $KRIT_TEST_SOCK.
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

// StartForeignListener runs a listener on sock as uid 54321 (needs root) and
// returns once the socket exists.
func startForeignListener(t *testing.T, sock string) {
	t.Helper()
	if os.Getuid() != 0 {
		t.Skip("needs root to run a listener as another uid")
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	dir := shortTempDir(t)
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(dir, "daemon.test")
	data, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bin, data, 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(bin, "-test.run=^TestHelperForeignListener$")
	cmd.Env = append(os.Environ(), "KRIT_TEST_SOCK="+sock)
	cmd.SysProcAttr = &syscall.SysProcAttr{Credential: &syscall.Credential{Uid: 54321, Gid: 54321}}
	if err := cmd.Start(); err != nil {
		t.Skipf("cannot run as another uid: %v", err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(sock); err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("foreign listener did not come up")
}

func TestProbe_ForeignUIDListenerIsRefused(t *testing.T) {
	dir := shortTempDir(t)
	if err := os.Chmod(dir, 0o777); err != nil {
		t.Fatal(err)
	}
	sock := filepath.Join(dir, "d.sock")
	startForeignListener(t, sock)
	err := Probe(sock)
	var pe *PeerUIDError
	if !errors.Is(err, ErrPeerUID) || !errors.As(err, &pe) || pe.UID != 54321 {
		t.Fatalf("Probe = %v, want PeerUIDError for uid 54321", err)
	}
	if !strings.Contains(err.Error(), "uid 54321") || !strings.Contains(err.Error(), "--no-daemon") {
		t.Fatalf("message not actionable: %v", err)
	}
	if Available(sock) {
		t.Fatal("Available must be false for a foreign listener")
	}
}

func TestServerStart_ReportsUnremovableStaleSocket(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root can unlink anything")
	}
	dir := shortTempDir(t)
	sock := filepath.Join(dir, "d.sock")
	if err := os.WriteFile(sock, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
	if err := NewServer(sock).Start(context.Background()); err == nil || !strings.Contains(err.Error(), "remove stale socket") {
		t.Fatalf("want remove-stale-socket error, got %v", err)
	}
}

// A fallback daemon that cannot publish its pointer must not come up: a
// client in another environment could not find it and would start a second.
func TestServerStart_FailsWhenPointerUnwritable(t *testing.T) {
	isolateTmp(t)
	root := longRoot(t)
	krit := filepath.Join(root, ".krit")
	if os.Getuid() == 0 {
		// Root ignores mode bits; make .krit a regular file instead so the
		// pointer's directory cannot be created.
		if err := os.WriteFile(krit, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	} else {
		if err := os.MkdirAll(krit, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(krit, 0o500); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(krit, 0o755) })
	}
	sock := DefaultSocketPath(root)
	srv := NewServer(sock)
	srv.PointerPath = PointerPath(root)
	err := srv.Start(context.Background())
	if err == nil || !strings.Contains(err.Error(), "socket pointer") {
		t.Fatalf("Start err = %v, want pointer error", err)
	}
	if _, serr := os.Lstat(sock); !os.IsNotExist(serr) {
		t.Fatalf("socket left behind after failed start: %v", serr)
	}
	if Available(sock) {
		t.Fatal("daemon reachable after failed start")
	}
}
