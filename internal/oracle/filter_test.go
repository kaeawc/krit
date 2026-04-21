package oracle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

// mkFile builds a scanner.File just well enough for CollectOracleFiles —
// only Path and Content are inspected by the filter. A real tree-sitter
// flat tree is not required because the filter works on raw bytes.
func mkFile(t *testing.T, name, body string) *scanner.File {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	return &scanner.File{Path: p, Content: []byte(body)}
}

func sortedAbs(t *testing.T, paths ...string) []string {
	t.Helper()
	out := make([]string, len(paths))
	for i, p := range paths {
		abs, err := filepath.Abs(p)
		if err != nil {
			t.Fatalf("abs %s: %v", p, err)
		}
		out[i] = abs
	}
	return out
}

func TestCollectOracleFiles_NoRules(t *testing.T) {
	f := mkFile(t, "A.kt", "class A")
	got := CollectOracleFiles(nil, []*scanner.File{f})
	if got.MarkedFiles != 0 {
		t.Errorf("MarkedFiles = %d, want 0", got.MarkedFiles)
	}
	if got.AllFiles {
		t.Errorf("AllFiles = true, want false")
	}
	if got.TotalFiles != 1 {
		t.Errorf("TotalFiles = %d, want 1", got.TotalFiles)
	}
}

func TestCollectOracleFiles_TreeSitterOnly(t *testing.T) {
	f1 := mkFile(t, "A.kt", "class A { fun f() {} }")
	f2 := mkFile(t, "B.kt", "suspend fun g() {}")
	rules := []OracleFilterRule{
		{Name: "EmptyCatchBlock", Filter: &OracleFilterSpec{}},
	}
	got := CollectOracleFiles(rules, []*scanner.File{f1, f2})
	if got.MarkedFiles != 0 {
		t.Errorf("MarkedFiles = %d, want 0 (tree-sitter only rule should mark no files)", got.MarkedFiles)
	}
	if got.AllFiles {
		t.Errorf("AllFiles = true, want false")
	}
	if got.Paths == nil {
		t.Errorf("Paths = nil, want empty slice to distinguish from AllFiles case")
	}
}

func TestCollectOracleFiles_FilteredByIdentifier(t *testing.T) {
	f1 := mkFile(t, "A.kt", "class A { fun f() {} }")                     // no suspend
	f2 := mkFile(t, "B.kt", "suspend fun g() { delay(10) }")              // has suspend
	f3 := mkFile(t, "C.kt", "class C { val x = \"suspend literal\" }")    // contains substring
	rules := []OracleFilterRule{
		{Name: "RedundantSuspendModifier", Filter: &OracleFilterSpec{Identifiers: []string{"suspend"}}},
	}
	got := CollectOracleFiles(rules, []*scanner.File{f1, f2, f3})
	if got.AllFiles {
		t.Errorf("AllFiles = true, want false")
	}
	if got.MarkedFiles != 2 {
		t.Errorf("MarkedFiles = %d, want 2 (B.kt, C.kt)", got.MarkedFiles)
	}
	want := sortedAbs(t, f2.Path, f3.Path)
	if !equalStrings(got.Paths, want) {
		t.Errorf("Paths = %v, want %v", got.Paths, want)
	}
}

func TestCollectOracleFiles_AllFilesShortCircuits(t *testing.T) {
	f1 := mkFile(t, "A.kt", "class A")
	f2 := mkFile(t, "B.kt", "class B")
	rules := []OracleFilterRule{
		{Name: "Deprecation", Filter: &OracleFilterSpec{AllFiles: true}},
		// An identifier rule that would only match B.kt — ignored because
		// the AllFiles rule above short-circuits.
		{Name: "Filtered", Filter: &OracleFilterSpec{Identifiers: []string{"B"}}},
	}
	got := CollectOracleFiles(rules, []*scanner.File{f1, f2})
	if !got.AllFiles {
		t.Errorf("AllFiles = false, want true")
	}
	if got.MarkedFiles != 2 {
		t.Errorf("MarkedFiles = %d, want 2", got.MarkedFiles)
	}
	if got.Paths != nil {
		t.Errorf("Paths = %v, want nil when AllFiles", got.Paths)
	}
}

func TestCollectOracleFiles_NilFilterDefaultsToAllFiles(t *testing.T) {
	f1 := mkFile(t, "A.kt", "class A")
	rules := []OracleFilterRule{
		{Name: "UnknownRule", Filter: nil},
	}
	got := CollectOracleFiles(rules, []*scanner.File{f1})
	if !got.AllFiles {
		t.Errorf("AllFiles = false, want true (nil filter should be conservative)")
	}
}

func TestCollectOracleFiles_UnionOfIdentifierFilters(t *testing.T) {
	f1 := mkFile(t, "A.kt", "class A") // nothing
	f2 := mkFile(t, "B.kt", "suspend fun g() {}")
	f3 := mkFile(t, "C.kt", "val x = y as Int")
	f4 := mkFile(t, "D.kt", "val x = y!!")
	rules := []OracleFilterRule{
		{Name: "RedundantSuspendModifier", Filter: &OracleFilterSpec{Identifiers: []string{"suspend"}}},
		{Name: "UnsafeCast", Filter: &OracleFilterSpec{Identifiers: []string{" as "}}},
		{Name: "UnnecessaryNotNullOperator", Filter: &OracleFilterSpec{Identifiers: []string{"!!"}}},
	}
	got := CollectOracleFiles(rules, []*scanner.File{f1, f2, f3, f4})
	if got.AllFiles {
		t.Errorf("AllFiles = true, want false")
	}
	if got.MarkedFiles != 3 {
		t.Errorf("MarkedFiles = %d, want 3", got.MarkedFiles)
	}
	want := sortedAbs(t, f2.Path, f3.Path, f4.Path)
	if !equalStrings(got.Paths, want) {
		t.Errorf("Paths = %v, want %v", got.Paths, want)
	}
}

func TestCollectOracleFiles_DedupsIdentifiers(t *testing.T) {
	f1 := mkFile(t, "A.kt", "suspend fun f() {}")
	rules := []OracleFilterRule{
		{Name: "R1", Filter: &OracleFilterSpec{Identifiers: []string{"suspend"}}},
		{Name: "R2", Filter: &OracleFilterSpec{Identifiers: []string{"suspend"}}},
	}
	got := CollectOracleFiles(rules, []*scanner.File{f1})
	if got.MarkedFiles != 1 {
		t.Errorf("MarkedFiles = %d, want 1", got.MarkedFiles)
	}
}

func TestWriteFilterListFile(t *testing.T) {
	dir := t.TempDir()
	summary := OracleFilterSummary{
		TotalFiles:  3,
		MarkedFiles: 2,
		Paths:       []string{"/abs/A.kt", "/abs/B.kt"},
	}
	p, err := WriteFilterListFile(summary, dir)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if p == "" {
		t.Fatalf("returned empty path")
	}
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	got := strings.TrimRight(string(b), "\n")
	want := "/abs/A.kt\n/abs/B.kt"
	if got != want {
		t.Errorf("contents = %q, want %q", got, want)
	}
}

func TestWriteFilterListFile_AllFilesReturnsEmpty(t *testing.T) {
	summary := OracleFilterSummary{AllFiles: true, TotalFiles: 10, MarkedFiles: 10}
	p, err := WriteFilterListFile(summary, "")
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	if p != "" {
		t.Errorf("path = %q, want empty (AllFiles should skip write)", p)
	}
}

func TestCollectOracleFiles_FingerprintStableAndDiffers(t *testing.T) {
	f1 := mkFile(t, "A.kt", "class A { fun f() {} }")
	f2 := mkFile(t, "B.kt", "suspend fun g() {}")
	f3 := mkFile(t, "C.kt", "suspend fun h() {}")
	rules := []OracleFilterRule{
		{Name: "RedundantSuspendModifier", Filter: &OracleFilterSpec{Identifiers: []string{"suspend"}}},
	}

	a := CollectOracleFiles(rules, []*scanner.File{f1, f2})
	b := CollectOracleFiles(rules, []*scanner.File{f1, f2})
	if a.Fingerprint == "" {
		t.Fatalf("Fingerprint empty, want non-empty for reduced set")
	}
	if a.Fingerprint != b.Fingerprint {
		t.Errorf("fingerprint not stable across identical inputs: %q vs %q", a.Fingerprint, b.Fingerprint)
	}

	c := CollectOracleFiles(rules, []*scanner.File{f1, f2, f3})
	if c.Fingerprint == a.Fingerprint {
		t.Errorf("fingerprint unchanged after adding a matching file: got %q for both", a.Fingerprint)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestStableFingerprint_InvariantUnderRootRelocation guards the CI
// oracle-fingerprint gate (issue #333): the fingerprint of the same
// relative path set must be identical regardless of where the repo
// is checked out on disk.
func TestStableFingerprint_InvariantUnderRootRelocation(t *testing.T) {
	root1 := "/home/runner/work/krit/krit"
	root2 := "/Users/jason/kaeawc/krit"
	rel := []string{"playground/app/Main.kt", "playground/app/Util.kt"}

	abs1 := make([]string, len(rel))
	abs2 := make([]string, len(rel))
	for i, r := range rel {
		abs1[i] = filepath.Join(root1, r)
		abs2[i] = filepath.Join(root2, r)
	}

	fp1 := StableFingerprint(abs1, root1)
	fp2 := StableFingerprint(abs2, root2)
	if fp1 != fp2 {
		t.Fatalf("fingerprint differs across roots: %s vs %s", fp1, fp2)
	}
	if fp1 == "" {
		t.Fatalf("fingerprint is empty")
	}
}

// TestStableFingerprint_ChangesWhenSetChanges confirms a dropped or
// added file moves the fingerprint — the CI gate depends on this.
func TestStableFingerprint_ChangesWhenSetChanges(t *testing.T) {
	root := "/tmp/x"
	a := StableFingerprint([]string{"/tmp/x/a.kt", "/tmp/x/b.kt"}, root)
	b := StableFingerprint([]string{"/tmp/x/a.kt"}, root)
	if a == b {
		t.Fatalf("fingerprint unchanged across different sets: %s", a)
	}
}
