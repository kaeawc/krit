package rules

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

func TestStringLiteralContentJavaStringLiteral(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Config.java")
	if err := os.WriteFile(path, []byte(`package test;

class Config {
  static final String API = "http://localhost:8080/api/v1";
}
`), 0644); err != nil {
		t.Fatal(err)
	}
	file, err := scanner.ParseJavaFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var content string
	file.FlatWalkAllNodes(0, func(idx uint32) {
		if content == "" && file.FlatType(idx) == "string_literal" {
			if flatContainsStringInterpolation(file, idx) {
				t.Fatal("Java string literal should not be treated as interpolated")
			}
			content = stringLiteralContent(file, idx)
		}
	})
	if content != "http://localhost:8080/api/v1" {
		t.Fatalf("stringLiteralContent() = %q", content)
	}
}
