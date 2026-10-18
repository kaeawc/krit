//go:build !windows

package daemon

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsurePrivateDir_CreatesAndAccepts0700(t *testing.T) {
	dir := filepath.Join(shortTempDir(t), "krit-x")
	if err := ensurePrivateDir(dir); err != nil {
		t.Fatalf("create: %v", err)
	}
	if fi, _ := os.Stat(dir); fi.Mode().Perm() != 0o700 {
		t.Fatalf("mode = %o, want 700", fi.Mode().Perm())
	}
	if err := ensurePrivateDir(dir); err != nil {
		t.Fatalf("reuse good dir: %v", err)
	}
}

func TestEnsurePrivateDir_TightensLooseOwnedDir(t *testing.T) {
	dir := filepath.Join(shortTempDir(t), "krit-x")
	if err := os.Mkdir(dir, 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o777); err != nil {
		t.Fatal(err)
	}
	if err := ensurePrivateDir(dir); err != nil {
		t.Fatalf("own dir with loose mode should be tightened: %v", err)
	}
	if fi, _ := os.Stat(dir); fi.Mode().Perm() != 0o700 {
		t.Fatalf("mode = %o, want 700", fi.Mode().Perm())
	}
}

func TestEnsurePrivateDir_RejectsSymlink(t *testing.T) {
	base := shortTempDir(t)
	target := filepath.Join(base, "target")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "krit-x")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if err := ensurePrivateDir(link); err == nil {
		t.Fatal("symlinked socket dir must be rejected")
	}
}

func TestEnsurePrivateDir_RejectsRegularFile(t *testing.T) {
	f := filepath.Join(shortTempDir(t), "krit-x")
	if err := os.WriteFile(f, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ensurePrivateDir(f); err == nil {
		t.Fatal("regular file must be rejected")
	}
}

func TestEnsurePrivateDir_RejectsForeignOwner(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("needs root to chown to another uid")
	}
	dir := filepath.Join(shortTempDir(t), "krit-x")
	if err := os.Mkdir(dir, 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.Chown(dir, 54321, -1); err != nil {
		t.Skipf("chown unsupported: %v", err)
	}
	if err := ensurePrivateDir(dir); err == nil || !strings.Contains(err.Error(), "not owned") {
		t.Fatalf("foreign-owned dir must be rejected, got %v", err)
	}
}

func longRoot(t *testing.T) string {
	t.Helper()
	root := filepath.Join(shortTempDir(t), strings.Repeat("deeply-nested-checkout-", 6), "project")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestResolveSocketPath_RelativeRootMatchesAbsolute(t *testing.T) {
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
	if len(p) > maxSocketPath || !strings.HasPrefix(p, "/tmp/krit-") {
		t.Fatalf("path %q should live under /tmp and fit the limit", p)
	}
}

func TestResolveSocketPath_PrefersXDGRuntimeDir(t *testing.T) {
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

func TestResolveSocketPath_SkipsUnsafeDirAndErrorsWhenNoneUsable(t *testing.T) {
	// A symlink where the XDG dir would be is skipped in favor of tmp.
	xdg := shortTempDir(t)
	other := shortTempDir(t)
	if err := os.Symlink(other, filepath.Join(xdg, "krit")); err != nil {
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
