package scanner

import (
	"os"
	"path/filepath"
	"testing"
)

// The same file must get the same baseline ID however the base path and the
// finding path are spelled (#1600).
func TestBaselineIDIgnoresPathSpelling(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "app", "src", "A.kt")
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("class A"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(root)
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(root, link); err != nil {
		t.Fatal(err)
	}
	want := "R:" + filepath.Join("app", "src", "A.kt") + ":$R$m"
	for _, tc := range []struct{ name, base, file string }{
		{"relative base, relative file", ".", filepath.Join("app", "src", "A.kt")},
		{"relative base, absolute file", ".", file},
		{"absolute base, relative file", root, filepath.Join("app", "src", "A.kt")},
		{"absolute base, absolute file", root, file},
		{"dot-slash file", "./", "./" + filepath.ToSlash(filepath.Join("app", "src", "A.kt"))},
		{"base through a symlink", link, file},
		{"file through a symlink", root, filepath.Join(link, "app", "src", "A.kt")},
	} {
		if got := baselineIDParts(tc.file, "R", "m", "", tc.base); got != want {
			t.Errorf("%s: id = %q, want %q", tc.name, got, want)
		}
	}
	if got := baselineIDParts(file, "R", "m", "", ""); got != "R:A.kt:$R$m" {
		t.Errorf("no base path: id = %q, want the base-name form", got)
	}
	outside := filepath.Join(t.TempDir(), "B.kt")
	if got := baselineIDParts(outside, "R", "m", "", root); got == "R:B.kt:$R$m" {
		t.Errorf("a file outside the base must keep its relative path, got the base-name form %q", got)
	}
}
