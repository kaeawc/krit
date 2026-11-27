package scan

import (
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

func TestFilterColumnsNewSince(t *testing.T) {
	columns := scanner.CollectFindings([]scanner.Finding{
		{File: "/repo/src/A.kt", Line: 1, Col: 1, Rule: "RuleA", RuleSet: "style", Severity: "warning", Message: "old"},
		{File: "/repo/src/B.kt", Line: 2, Col: 1, Rule: "RuleB", RuleSet: "style", Severity: "warning", Message: "new"},
	})
	baseIDs := map[string]bool{
		deltaFindingID("src/A.kt", "RuleA", "old"): true,
	}
	filtered := filterColumnsNewSince(&columns, baseIDs, "/repo")
	if filtered.Len() != 1 {
		t.Fatalf("filtered.Len() = %d, want 1", filtered.Len())
	}
	if got := filtered.RuleAt(0); got != "RuleB" {
		t.Fatalf("remaining rule = %q", got)
	}
}

func TestMapPathToWorktree(t *testing.T) {
	got := mapPathToWorktree("/repo", "/tmp/base", "/repo/app/src")
	if got != "/tmp/base/app/src" {
		t.Fatalf("mapPathToWorktree() = %q", got)
	}
}

// PR mode under --delta (#1660): the base scan gets the current side's
// files, mapped into its worktree, before the scan paths.
func TestDeltaBaseScanArgsCarryThePRModeFiles(t *testing.T) {
	f := freshScanFlags(t)
	*f.Verbose = true
	args := deltaSnapshotArgsForFlags(f, []string{"/repo/app"}, "/repo", "/tmp/base", "/tmp/list/pr-files.txt")
	at := slices.Index(args, "--pr-files")
	if at < 0 || at+1 >= len(args) || args[at+1] != "/tmp/list/pr-files.txt" {
		t.Fatalf("args lack --pr-files: %v", args)
	}
	if !slices.Contains(args, "-v") {
		t.Fatalf("a verbose --delta does not ask the base scan for its log: %v", args)
	}
	if last := args[len(args)-1]; last != "/tmp/base/app" || at > len(args)-2 {
		t.Fatalf("scan path must come last, after the flags: %v", args)
	}
	if args := deltaSnapshotArgsForFlags(freshScanFlags(t), nil, "/repo", "/tmp/base", ""); slices.Contains(args, "--pr-files") || slices.Contains(args, "-v") {
		t.Fatalf("whole-repository base scan args = %v", args)
	}

	dir := t.TempDir()
	list := filepath.Join(dir, "pr-files.txt")
	if err := writePRFileList(list, []string{"/repo/app/src/A.kt", "/repo/lib/B.kt"}, "/repo", "/tmp/base"); err != nil {
		t.Fatal(err)
	}
	got, err := readPRFileList(list)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"/tmp/base/app/src/A.kt": true, "/tmp/base/lib/B.kt": true}
	if !maps.Equal(got, want) {
		t.Fatalf("round-tripped list = %v, want %v", got, want)
	}
}

// The Gradle model is untracked, so the base worktree gets the current
// one, unless the base ref commits its own or the model would not rebase.
func TestDeltaCopiesTheGradleModelIntoTheBaseWorktree(t *testing.T) {
	repo, tmp := t.TempDir(), t.TempDir()
	write := func(path, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(repo, "settings.gradle.kts"), "")
	write(filepath.Join(repo, ".krit", "gradle-model", "model.json"), `{"schema":2}`)
	read := func(path string) string {
		t.Helper()
		data, err := os.ReadFile(path)
		if err != nil {
			return ""
		}
		return string(data)
	}
	copied := filepath.Join(tmp, ".krit", "gradle-model", "model.json")

	if err := copyGradleModelToWorktree(freshScanFlags(t), []string{repo}, repo, tmp); err != nil {
		t.Fatal(err)
	}
	if got := read(copied); got != `{"schema":2}` {
		t.Fatalf("copied model = %q", got)
	}
	// A model the base ref commits is its own.
	write(copied, `{"schema":2,"base":true}`)
	if err := copyGradleModelToWorktree(freshScanFlags(t), []string{repo}, repo, tmp); err != nil {
		t.Fatal(err)
	}
	if got := read(copied); got != `{"schema":2,"base":true}` {
		t.Fatalf("the base ref's model was overwritten: %q", got)
	}

	noModel := freshScanFlags(t)
	*noModel.NoGradleModel = true
	other := t.TempDir()
	if err := copyGradleModelToWorktree(noModel, []string{repo}, repo, other); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(other, ".krit")); err == nil {
		t.Fatal("--no-gradle-model copied a model")
	}
	// A model outside the repository is read as written; it is not copied.
	outside := freshScanFlags(t)
	*outside.GradleModel = filepath.Join(t.TempDir(), ".krit", "gradle-model")
	write(filepath.Join(*outside.GradleModel, "model.json"), `{}`)
	if err := copyGradleModelToWorktree(outside, []string{repo}, repo, other); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(other, ".krit")); err == nil {
		t.Fatal("a model outside the repository was copied")
	}
}

func TestForwardBaseScanLogMarksEachLine(t *testing.T) {
	var b strings.Builder
	forwardBaseScanLog(&b, "verbose: one\n\nverbose: two\n")
	if got, want := b.String(), "--delta base: verbose: one\n--delta base: verbose: two\n"; got != want {
		t.Fatalf("forwarded log = %q, want %q", got, want)
	}
}
