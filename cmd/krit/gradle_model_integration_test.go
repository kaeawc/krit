package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestExplicitGradleModelMissingFailsAtStartup(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "A.kt"), []byte("fun a() = 1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(binPath, "--no-daemon", "--gradle-model", filepath.Join(root, "missing"), root)
	out, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(out), "read gradle model") {
		t.Fatalf("expected startup error; err=%v output=%s", err, out)
	}
}

func TestInitGitignore(t *testing.T) {
	for _, hasIgnore := range []bool{true, false} {
		root := t.TempDir()
		path := filepath.Join(root, ".gitignore")
		if hasIgnore {
			if err := os.WriteFile(path, []byte("build/"), 0644); err != nil {
				t.Fatal(err)
			}
		}
		for i := 0; i < 2; i++ {
			cmd := exec.Command(binPath, "--init")
			cmd.Dir = root
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("init: %v %s", err, out)
			}
		}
		data, err := os.ReadFile(path)
		if hasIgnore && (err != nil || string(data) != "build/\n.krit/\n") {
			t.Fatalf("gitignore: %q %v", data, err)
		}
		if !hasIgnore && !os.IsNotExist(err) {
			t.Fatalf("unexpected .gitignore: %v", err)
		}
	}
}
