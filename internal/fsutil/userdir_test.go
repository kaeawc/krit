package fsutil

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestUserKritDirCreatesPrivateDirectory(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmp)
	if runtime.GOOS == "darwin" {
		// On darwin os.UserCacheDir uses HOME/Library/Caches; redirect via HOME.
		t.Setenv("HOME", tmp)
	}

	dir, err := UserKritDir()
	if err != nil {
		t.Fatalf("UserKritDir: %v", err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat %s: %v", dir, err)
	}
	if !info.IsDir() {
		t.Fatalf("UserKritDir returned non-directory %s", dir)
	}
	// 0700 isolates the dir from other users on multi-user hosts. Windows
	// reports a different bit mask; only enforce on Unix.
	if runtime.GOOS != "windows" {
		if mode := info.Mode().Perm(); mode&0o077 != 0 {
			t.Errorf("UserKritDir %s mode %o leaks bits to group/other; want owner-only", dir, mode)
		}
	}
}

func TestCreateUserKritFileWritesUnderUserDir(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", tmp)
	if runtime.GOOS == "darwin" {
		t.Setenv("HOME", tmp)
	}

	f, path, err := CreateUserKritFile("regression.log")
	if err != nil {
		t.Fatalf("CreateUserKritFile: %v", err)
	}
	defer func() { _ = f.Close() }()
	if _, err := f.Write([]byte("entry\n")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	dir, err := UserKritDir()
	if err != nil {
		t.Fatalf("UserKritDir: %v", err)
	}
	if filepath.Dir(path) != dir {
		t.Errorf("CreateUserKritFile path %s not under UserKritDir %s", path, dir)
	}

	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat: %v", err)
		}
		if mode := info.Mode().Perm(); mode&0o077 != 0 {
			t.Errorf("CreateUserKritFile %s mode %o leaks bits to group/other", path, mode)
		}
	}

	// Second call truncates so prior content is replaced — important for
	// log files that should not grow unbounded across daemon restarts.
	f2, _, err := CreateUserKritFile("regression.log")
	if err != nil {
		t.Fatalf("second CreateUserKritFile: %v", err)
	}
	if err := f2.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("read after truncate: %v", err)
	}
	if len(data) != 0 {
		t.Errorf("expected truncated file, got %d bytes", len(data))
	}
}
