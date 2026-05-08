package scan

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectConfigForScanArgs(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "krit.yml")
	if err := os.WriteFile(configPath, []byte("style:\n  MaxLineLength:\n    active: false\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if got := detectConfigForScanArgs([]string{dir}); got != configPath {
		t.Fatalf("detectConfigForScanArgs(dir) = %q, want %q", got, configPath)
	}
	filePath := filepath.Join(dir, "Example.kt")
	if err := os.WriteFile(filePath, []byte("class Example\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if got := detectConfigForScanArgs([]string{filePath}); got != configPath {
		t.Fatalf("detectConfigForScanArgs(file) = %q, want %q", got, configPath)
	}
}
