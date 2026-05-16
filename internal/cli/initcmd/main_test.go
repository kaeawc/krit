package initcmd

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// binPath points at a freshly built krit binary used by the initcmd
// integration tests that exercise the `krit init` subprocess path.
var binPath string

func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "krit-initcmd-integration-*")
	if err != nil {
		log.Fatal(err)
	}
	binPath = filepath.Join(tmp, "krit-test")
	cmd := exec.Command("go", "build", "-o", binPath, "github.com/kaeawc/krit/cmd/krit")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Fatalf("failed to build krit binary: %v", err)
	}

	code := m.Run()
	os.RemoveAll(tmp)
	os.Exit(code)
}

// copyDir performs a recursive copy from src to dst, preserving
// permissions on regular files. Used to stage a throwaway copy of a
// playground project for each integration run.
func copyDir(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode())
	})
	if err != nil {
		t.Fatalf("copyDir: %v", err)
	}
}
