package scan

import (
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
)

// PR mode (#1660): a --diff scan analyzes the changed Kotlin files with FIR
// and the type oracle; --no-pr-mode and a scan without --diff analyze all.
func TestPRModeFilesAreTheChangedKotlinFiles(t *testing.T) {
	root := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "user.email=k@example.com", "-c", "user.name=k"}, args...)...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	write := func(rel, body string) string {
		t.Helper()
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	a := write("app/src/main/kotlin/A.kt", "package a\n\nfun a() = 1\n")
	b := write("app/src/main/kotlin/B.kt", "package a\n\nfun b() = 1\n")
	git("init", "-q")
	git("add", "-A")
	git("commit", "-qm", "init")
	write("app/src/main/kotlin/A.kt", "package a\n\nfun a() = 2\n")
	write("README.md", "changed, not Kotlin\n")
	t.Chdir(root)
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	a, b = filepath.Join(wd, "app/src/main/kotlin/A.kt"), filepath.Join(wd, "app/src/main/kotlin/B.kt")

	runner := func(args ...string) *runner {
		fs := flag.NewFlagSet("scan", flag.ContinueOnError)
		f := registerScanFlags(fs)
		if err := fs.Parse(args); err != nil {
			t.Fatal(err)
		}
		return &runner{f: f, paths: []string{wd}, files: []string{a, b}}
	}
	if got := runner("--diff", "HEAD").prModeFiles(); !slices.Equal(got, []string{a}) {
		t.Fatalf("PR mode files = %v, want only the changed %s", got, a)
	}
	// A relative spelling of the scanned files (`krit --diff HEAD .`) finds them too.
	rel := runner("--diff", "HEAD")
	rel.paths, rel.files = []string{"."}, []string{"app/src/main/kotlin/A.kt", "app/src/main/kotlin/B.kt"}
	if got := rel.prModeFiles(); !slices.Equal(got, []string{"app/src/main/kotlin/A.kt"}) {
		t.Fatalf("relative PR mode files = %v", got)
	}
	if got := runner("--diff", "HEAD", "--no-pr-mode").prModeFiles(); got != nil {
		t.Fatalf("--no-pr-mode: %v", got)
	}
	if got := runner().prModeFiles(); got != nil {
		t.Fatalf("without --diff: %v", got)
	}
	// --delta compiles the changed files' modules too, unless --no-pr-mode.
	if got := runner("--delta", "HEAD").prModeFiles(); !slices.Equal(got, []string{a}) {
		t.Fatalf("--delta PR mode files = %v, want only the changed %s", got, a)
	}
	if got := runner("--delta", "HEAD", "--no-pr-mode").prModeFiles(); got != nil {
		t.Fatalf("--delta --no-pr-mode: %v", got)
	}
	// --delta's base scan reads the current side's list (--pr-files).
	list := filepath.Join(t.TempDir(), "pr-files.txt")
	if err := os.WriteFile(list, []byte(b+"\n"+filepath.Join(wd, "Gone.kt")+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := runner("--pr-files", list).prModeFiles(); !slices.Equal(got, []string{b}) {
		t.Fatalf("--pr-files PR mode files = %v, want the listed %s", got, b)
	}
	if err := os.WriteFile(list, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if got := runner("--pr-files", list).prModeFiles(); got == nil || len(got) != 0 {
		t.Fatalf("an empty --pr-files list analyzes nothing, got %v", got)
	}
	git("add", "-A")
	git("commit", "-qm", "edit")
	if got := runner("--diff", "HEAD").prModeFiles(); got == nil || len(got) != 0 {
		t.Fatalf("no change analyzes nothing, got %v", got)
	}
}
