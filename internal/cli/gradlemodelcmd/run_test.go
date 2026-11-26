package gradlemodelcmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/gradlemodel"
)

func TestKeyPrintsWritesAndLists(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "settings.gradle.kts"), []byte(""), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"key", "--root", root, "--write"}, &stdout, &stderr); code != 1 ||
		!strings.Contains(stderr.String(), "run kritExportModel first") {
		t.Fatalf("--write without a model: code %d, stderr %q", code, stderr.String())
	}
	modelDir := filepath.Join(root, ".krit", "gradle-model")
	if err := os.MkdirAll(modelDir, 0o755); err != nil {
		t.Fatal(err)
	}
	stdout.Reset()
	if code := run([]string{"key", "--root", root, "--write"}, &stdout, &stderr); code != 0 {
		t.Fatalf("code %d, stderr %q", code, stderr.String())
	}
	want, err := gradlemodel.InputsKey(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.TrimSpace(stdout.String()); got != want {
		t.Fatalf("printed %q, want %q", got, want)
	}
	if got := gradlemodel.ReadInputsKey(modelDir); got != want {
		t.Fatalf("stamp %q, want %q", got, want)
	}
	stdout.Reset()
	if code := run([]string{"key", "--root", root, "--list"}, &stdout, &stderr); code != 0 || stdout.String() != "settings.gradle.kts\n" {
		t.Fatalf("--list: code %d, stdout %q", code, stdout.String())
	}
	if code := run(nil, &stdout, &stderr); code != 2 {
		t.Fatalf("no subcommand: code %d", code)
	}
}
