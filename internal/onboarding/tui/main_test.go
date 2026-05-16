package tui

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// binPath points at a freshly built krit binary used by the tui
// integration tests that exercise the `krit init` subprocess path.
var binPath string

func TestMain(m *testing.M) {
	tmp, err := os.MkdirTemp("", "krit-tui-integration-*")
	if err != nil {
		log.Fatal(err)
	}
	binPath = filepath.Join(tmp, "krit-test")
	cmd := exec.Command("go", "build", "-o", binPath, "github.com/kaeawc/krit/cmd/krit")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Fatalf("failed to build krit binary: %v", err)
	}
	if err := os.Setenv("KRIT_NO_DAEMON_AUTOSTART", "1"); err != nil {
		log.Fatalf("failed to set test env: %v", err)
	}

	code := m.Run()
	os.RemoveAll(tmp)
	os.Exit(code)
}
