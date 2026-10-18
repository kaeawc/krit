//go:build !windows

package fsutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEnsurePrivateDir_CreatesAndAccepts0700(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "krit-x")
	if err := EnsurePrivateDir(dir, true); err != nil {
		t.Fatalf("create: %v", err)
	}
	if fi, _ := os.Stat(dir); fi.Mode().Perm() != 0o700 {
		t.Fatalf("mode = %o, want 700", fi.Mode().Perm())
	}
	if err := EnsurePrivateDir(dir, true); err != nil {
		t.Fatalf("reuse good dir: %v", err)
	}
}

func TestEnsurePrivateDir_TightensLooseOwnedDir(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "krit-x")
	if err := os.Mkdir(dir, 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o777); err != nil {
		t.Fatal(err)
	}
	if err := EnsurePrivateDir(dir, true); err != nil {
		t.Fatalf("own dir with loose mode should be tightened: %v", err)
	}
	if fi, _ := os.Stat(dir); fi.Mode().Perm() != 0o700 {
		t.Fatalf("mode = %o, want 700", fi.Mode().Perm())
	}
}

func TestEnsurePrivateDir_RejectsSymlink(t *testing.T) {
	base := t.TempDir()
	target := filepath.Join(base, "target")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(base, "krit-x")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if err := EnsurePrivateDir(link, true); err == nil {
		t.Fatal("symlinked dir must be rejected")
	}
}

func TestEnsurePrivateDir_RejectsRegularFile(t *testing.T) {
	f := filepath.Join(t.TempDir(), "krit-x")
	if err := os.WriteFile(f, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := EnsurePrivateDir(f, true); err == nil {
		t.Fatal("regular file must be rejected")
	}
}

func TestEnsurePrivateDir_RejectsForeignOwner(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("needs root to chown to another uid")
	}
	dir := filepath.Join(t.TempDir(), "krit-x")
	if err := os.Mkdir(dir, 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.Chown(dir, 54321, -1); err != nil {
		t.Skipf("chown unsupported: %v", err)
	}
	if err := EnsurePrivateDir(dir, true); err == nil || !strings.Contains(err.Error(), "not owned") {
		t.Fatalf("foreign-owned dir must be rejected, got %v", err)
	}
}

func TestEnsurePrivateDir_ParentWritableWithoutSticky(t *testing.T) {
	parent := t.TempDir()
	if err := os.Chmod(parent, 0o777); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(parent, "krit-x")
	if err := EnsurePrivateDir(dir, true); err == nil || !strings.Contains(err.Error(), "sticky") {
		t.Fatalf("world-writable non-sticky parent must be rejected, got %v", err)
	}
	if _, err := os.Lstat(dir); err == nil {
		t.Fatal("dir must not be created under an unsafe parent")
	}
	// Sticky bit (like /tmp) makes it acceptable.
	if err := os.Chmod(parent, 0o777|os.ModeSticky); err != nil {
		t.Fatal(err)
	}
	if err := EnsurePrivateDir(dir, true); err != nil {
		t.Fatalf("sticky parent should be accepted: %v", err)
	}
	// Without checkParent the writable parent is not examined.
	if err := os.Chmod(parent, 0o777); err != nil {
		t.Fatal(err)
	}
	if err := EnsurePrivateDir(filepath.Join(parent, "other"), false); err != nil {
		t.Fatalf("checkParent=false: %v", err)
	}
}

func TestUserKritDir_TempFallbackIsPrivate(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	t.Setenv("XDG_CACHE_HOME", "")
	t.Setenv("HOME", "")
	dir, err := UserKritDir()
	if err != nil {
		t.Skipf("UserCacheDir available, fallback not exercised: %v", err)
	}
	if fi, err := os.Stat(dir); err != nil || fi.Mode().Perm()&0o077 != 0 {
		t.Fatalf("dir %s not private: %v %v", dir, fi, err)
	}
}

func TestEnsurePrivateDir_RejectsParentOwnedByOtherUser(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("needs root to chown")
	}
	parent := t.TempDir()
	if err := os.Chmod(parent, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chown(parent, 54321, -1); err != nil {
		t.Skipf("chown unsupported: %v", err)
	}
	dir := filepath.Join(parent, "krit-x")
	if err := EnsurePrivateDir(dir, true); err == nil || !strings.Contains(err.Error(), "neither us nor root") {
		t.Fatalf("want parent-owner rejection, got %v", err)
	}
}

func TestCheckPrivateDir_ReadOnly(t *testing.T) {
	base := t.TempDir()
	missing := filepath.Join(base, "krit-x")
	if err := CheckPrivateDir(missing, true); err != nil {
		t.Fatalf("missing dir should pass: %v", err)
	}
	if _, err := os.Lstat(missing); err == nil {
		t.Fatal("CheckPrivateDir must not create the dir")
	}
	loose := filepath.Join(base, "loose")
	if err := os.Mkdir(loose, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := CheckPrivateDir(loose, true); err != nil {
		t.Fatalf("own loose dir should pass: %v", err)
	}
	if fi, _ := os.Stat(loose); fi.Mode().Perm() != 0o755 {
		t.Fatal("CheckPrivateDir must not chmod")
	}
	link := filepath.Join(base, "link")
	if err := os.Symlink(loose, link); err != nil {
		t.Fatal(err)
	}
	if err := CheckPrivateDir(link, true); err == nil {
		t.Fatal("symlink must fail")
	}
}
