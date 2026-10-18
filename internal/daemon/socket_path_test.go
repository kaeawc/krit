//go:build !windows

package daemon

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	p, err := ResolveSocketPath(longRoot(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(filepath.Dir(p)) })
	if len(p) > maxSocketPath || !strings.HasPrefix(p, "/tmp/krit-") {
		t.Fatalf("path %q should live under /tmp and fit the limit", p)
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
	if got, err := LocateSocketPath(root); err != nil || got != sock {
		t.Fatalf("LocateSocketPath = %q, %v; want pointer target %q", got, err, sock)
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

func TestLocateSocketPath_PointerWinsOverDerivation(t *testing.T) {
	isolateTmp(t)
	root := longRoot(t)
	// Daemon started under a different TMPDIR than the client now has.
	other := shortTempDir(t)
	elsewhere := filepath.Join(other, "krit", "0123456789abcdef.sock")
	if err := WriteSocketPointer(PointerPath(root), elsewhere); err != nil {
		t.Fatal(err)
	}
	if got, err := LocateSocketPath(root); err != nil || got != elsewhere {
		t.Fatalf("LocateSocketPath = %q, %v; want %q", got, err, elsewhere)
	}
	// A pointer to an arbitrary path (not a krit fallback socket) is ignored.
	if err := WriteSocketPointer(PointerPath(root), "/var/run/other.sock"); err != nil {
		t.Fatal(err)
	}
	if got, _ := LocateSocketPath(root); got == "/var/run/other.sock" {
		t.Fatal("pointer to a foreign path must be ignored")
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
