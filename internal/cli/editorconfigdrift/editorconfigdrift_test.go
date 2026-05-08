package editorconfigdrift

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunReportsEditorConfigDrift(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".editorconfig"), `root = true

[*.kt]
indent_style = space
indent_size = 4
max_line_length = 20
charset = utf-8
insert_final_newline = true
trim_trailing_whitespace = true
`)
	writeFile(t, filepath.Join(root, "src", "main", "kotlin", "Example.kt"), "package test\n\nfun main() {\n  val longName = \"this line is too long\"  \n}")

	stdout, stderr, code := captureRun(t, []string{root})
	if code != 0 {
		t.Fatalf("Run code = %d, stderr=%s", code, stderr)
	}
	for _, want := range []string{
		".editorconfig says indent_size=4; 1 files use 2-space indentation.",
		".editorconfig says max_line_length=20; 1 files exceed it on 1 lines.",
		".editorconfig says insert_final_newline=true; 1 files are missing a final newline.",
		".editorconfig says trim_trailing_whitespace=true; 1 files contain trailing whitespace on 1 lines.",
	} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("stdout missing %q:\n%s", want, stdout)
		}
	}
}

func TestRunReportsNoDrift(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, ".editorconfig"), `root = true

[*.kt]
indent_style = space
indent_size = 2
max_line_length = 100
charset = utf-8
insert_final_newline = true
trim_trailing_whitespace = true
`)
	writeFile(t, filepath.Join(root, "src", "main", "kotlin", "Clean.kt"), "package test\n\nfun main() {\n  val x = 1\n}\n")

	stdout, stderr, code := captureRun(t, []string{root})
	if code != 0 {
		t.Fatalf("Run code = %d, stderr=%s", code, stderr)
	}
	if strings.TrimSpace(stdout) != "No editorconfig drift found." {
		t.Fatalf("unexpected stdout:\n%s", stdout)
	}
}

func captureRun(t *testing.T, args []string) (string, string, int) {
	t.Helper()
	origStdout := os.Stdout
	origStderr := os.Stderr
	outR, outW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	errR, errW, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = outW
	os.Stderr = errW
	code := Run(args)
	outW.Close()
	errW.Close()
	os.Stdout = origStdout
	os.Stderr = origStderr

	var outBuf, errBuf bytes.Buffer
	if _, err := outBuf.ReadFrom(outR); err != nil {
		t.Fatal(err)
	}
	if _, err := errBuf.ReadFrom(errR); err != nil {
		t.Fatal(err)
	}
	return outBuf.String(), errBuf.String(), code
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
