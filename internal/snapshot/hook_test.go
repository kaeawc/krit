package snapshot

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallHookWritesTaggedScript(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".git", "hooks"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	path, err := InstallHook(repo, false)
	if err != nil {
		t.Fatalf("InstallHook: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read installed hook: %v", err)
	}
	if !strings.Contains(string(data), HookMarker) {
		t.Fatalf("installed hook missing marker: %s", data)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm()&0o100 == 0 {
		t.Fatalf("installed hook not executable: mode=%v", info.Mode())
	}
}

func TestInstallHookRefusesUnmanagedExisting(t *testing.T) {
	repo := t.TempDir()
	hooks := filepath.Join(repo, ".git", "hooks")
	if err := os.MkdirAll(hooks, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := HookPath(repo)
	if err := os.WriteFile(path, []byte("#!/bin/sh\n# user's own hook\n"), 0o755); err != nil {
		t.Fatalf("seed user hook: %v", err)
	}
	if _, err := InstallHook(repo, false); err == nil {
		t.Fatal("expected error refusing to overwrite user hook")
	}
	if _, err := InstallHook(repo, true); err != nil {
		t.Fatalf("InstallHook with force: %v", err)
	}
}

func TestUninstallHookOnlyRemovesKritManaged(t *testing.T) {
	repo := t.TempDir()
	hooks := filepath.Join(repo, ".git", "hooks")
	if err := os.MkdirAll(hooks, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if _, err := InstallHook(repo, false); err != nil {
		t.Fatalf("InstallHook: %v", err)
	}
	if _, err := UninstallHook(repo); err != nil {
		t.Fatalf("UninstallHook: %v", err)
	}
	if _, err := os.Stat(HookPath(repo)); !os.IsNotExist(err) {
		t.Fatalf("expected hook to be gone, stat: %v", err)
	}

	// Now seed a non-krit hook and verify UninstallHook refuses.
	if err := os.WriteFile(HookPath(repo), []byte("#!/bin/sh\necho mine\n"), 0o755); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := UninstallHook(repo); err == nil {
		t.Fatal("expected error refusing to remove unmanaged hook")
	}
	if _, err := os.Stat(HookPath(repo)); err != nil {
		t.Fatalf("user's hook should still exist: %v", err)
	}
}

func TestUninstallHookOnMissingFileIsNoop(t *testing.T) {
	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".git", "hooks"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if _, err := UninstallHook(repo); err != nil {
		t.Fatalf("UninstallHook on missing: %v", err)
	}
}
