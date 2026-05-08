package harvest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseTarget(t *testing.T) {
	target, err := ParseTarget("/tmp/Sample.kt:42")
	if err != nil {
		t.Fatalf("ParseTarget returned error: %v", err)
	}
	if target.Path != "/tmp/Sample.kt" {
		t.Fatalf("expected path /tmp/Sample.kt, got %q", target.Path)
	}
	if target.Line != 42 {
		t.Fatalf("expected line 42, got %d", target.Line)
	}
}

func TestExtractFixture_MagicNumber(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Sample.kt")
	src := "package test\n\nfun answer() {\n    println(42)\n}\n"
	if err := os.WriteFile(path, []byte(src), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := ExtractFixture(Target{Path: path, Line: 4}, "MagicNumber")
	if err != nil {
		t.Fatalf("ExtractFixture returned error: %v", err)
	}
	if result.Rule != "MagicNumber" {
		t.Fatalf("expected MagicNumber finding, got %s", result.Rule)
	}
	if result.Line != 4 {
		t.Fatalf("expected line 4 finding, got %d", result.Line)
	}
	if !strings.Contains(string(result.Content), "42") {
		t.Fatalf("expected extracted content to contain 42, got %q", string(result.Content))
	}
}

func TestWriteFixture_AppendsTrailingNewline(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "fixtures", "sample.kt")

	err := WriteFixture(outPath, Result{Content: []byte("42")})
	if err != nil {
		t.Fatalf("WriteFixture returned error: %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "42\n" {
		t.Fatalf("expected trailing newline, got %q", string(data))
	}
}
