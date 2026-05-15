package daemon

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

func TestCompareIsCleanForIdenticalInputs(t *testing.T) {
	rows := []scanner.Finding{
		{File: "a.kt", Line: 10, Col: 4, RuleSet: "perf", Rule: "Foo", Severity: "warning", Message: "boom"},
		{File: "b.kt", Line: 3, Col: 1, RuleSet: "style", Rule: "Bar", Severity: "info", Message: "tidy"},
	}
	daemon := scanner.CollectFindings(rows)
	baseline := scanner.CollectFindings(rows)
	diff := Compare(&daemon, &baseline)
	if !diff.IsClean() {
		t.Fatalf("expected clean diff, got added=%+v dropped=%+v", diff.AddedByDaemon, diff.DroppedByDaemon)
	}
	if diff.PathsTouched != nil {
		t.Fatalf("expected empty PathsTouched, got %v", diff.PathsTouched)
	}
}

func TestCompareDetectsAddedAndDropped(t *testing.T) {
	common := scanner.Finding{File: "a.kt", Line: 10, Col: 4, RuleSet: "perf", Rule: "Foo", Severity: "warning", Message: "boom"}
	onlyDaemon := scanner.Finding{File: "a.kt", Line: 20, Col: 1, RuleSet: "perf", Rule: "Foo", Severity: "warning", Message: "second"}
	onlyBaseline := scanner.Finding{File: "b.kt", Line: 5, Col: 2, RuleSet: "style", Rule: "Bar", Severity: "info", Message: "third"}

	daemon := scanner.CollectFindings([]scanner.Finding{common, onlyDaemon})
	baseline := scanner.CollectFindings([]scanner.Finding{common, onlyBaseline})

	diff := Compare(&daemon, &baseline)
	if diff.IsClean() {
		t.Fatalf("expected non-clean diff")
	}
	if len(diff.AddedByDaemon) != 1 || diff.AddedByDaemon[0].Line != 20 {
		t.Fatalf("unexpected AddedByDaemon: %+v", diff.AddedByDaemon)
	}
	if len(diff.DroppedByDaemon) != 1 || diff.DroppedByDaemon[0].File != "b.kt" {
		t.Fatalf("unexpected DroppedByDaemon: %+v", diff.DroppedByDaemon)
	}
	wantPaths := []string{"a.kt", "b.kt"}
	if !reflect.DeepEqual(diff.PathsTouched, wantPaths) {
		t.Fatalf("PathsTouched = %v, want %v", diff.PathsTouched, wantPaths)
	}
}

func TestCompareIsMultisetAware(t *testing.T) {
	dup := scanner.Finding{File: "a.kt", Line: 1, Col: 1, RuleSet: "perf", Rule: "Foo", Severity: "warning", Message: "dup"}
	daemon := scanner.CollectFindings([]scanner.Finding{dup, dup, dup})
	baseline := scanner.CollectFindings([]scanner.Finding{dup})

	diff := Compare(&daemon, &baseline)
	if len(diff.AddedByDaemon) != 2 {
		t.Fatalf("expected 2 added rows from duplicate, got %d", len(diff.AddedByDaemon))
	}
	if len(diff.DroppedByDaemon) != 0 {
		t.Fatalf("expected 0 dropped rows, got %d", len(diff.DroppedByDaemon))
	}
}

func TestCompareIgnoresRowOrdering(t *testing.T) {
	a := scanner.Finding{File: "a.kt", Line: 1, RuleSet: "perf", Rule: "Foo", Severity: "warning", Message: "x"}
	b := scanner.Finding{File: "b.kt", Line: 2, RuleSet: "perf", Rule: "Foo", Severity: "warning", Message: "y"}

	daemon := scanner.CollectFindings([]scanner.Finding{a, b})
	baseline := scanner.CollectFindings([]scanner.Finding{b, a})

	diff := Compare(&daemon, &baseline)
	if !diff.IsClean() {
		t.Fatalf("expected reordered inputs to compare equal, got added=%+v dropped=%+v", diff.AddedByDaemon, diff.DroppedByDaemon)
	}
}

func TestCompareHandlesNilColumns(t *testing.T) {
	row := scanner.Finding{File: "a.kt", Line: 1, RuleSet: "perf", Rule: "Foo", Severity: "warning", Message: "x"}
	daemon := scanner.CollectFindings([]scanner.Finding{row})

	diff := Compare(&daemon, nil)
	if len(diff.AddedByDaemon) != 1 || len(diff.DroppedByDaemon) != 0 {
		t.Fatalf("nil baseline: added=%d dropped=%d", len(diff.AddedByDaemon), len(diff.DroppedByDaemon))
	}

	diff = Compare(nil, &daemon)
	if len(diff.AddedByDaemon) != 0 || len(diff.DroppedByDaemon) != 1 {
		t.Fatalf("nil daemon: added=%d dropped=%d", len(diff.AddedByDaemon), len(diff.DroppedByDaemon))
	}

	diff = Compare(nil, nil)
	if !diff.IsClean() {
		t.Fatalf("expected nil/nil compare to be clean")
	}
}

func TestCompareWithInjectedWorkspaceMutation(t *testing.T) {
	staleRow := scanner.Finding{
		File: "src/Auth.kt", Line: 12, Col: 8,
		RuleSet: "security", Rule: "InsecureCrypto",
		Severity: "error", Message: "MD5 detected",
	}
	freshRow := scanner.Finding{
		File: "src/Auth.kt", Line: 14, Col: 8,
		RuleSet: "security", Rule: "InsecureCrypto",
		Severity: "error", Message: "MD5 detected",
	}

	daemonCols := scanner.CollectFindings([]scanner.Finding{staleRow})
	baselineCols := scanner.CollectFindings([]scanner.Finding{freshRow})

	diff := Compare(&daemonCols, &baselineCols)
	if diff.IsClean() {
		t.Fatalf("expected mutation to surface divergence")
	}
	if len(diff.AddedByDaemon) != 1 || diff.AddedByDaemon[0].Line != 12 {
		t.Fatalf("AddedByDaemon = %+v, want stale row at line 12", diff.AddedByDaemon)
	}
	if len(diff.DroppedByDaemon) != 1 || diff.DroppedByDaemon[0].Line != 14 {
		t.Fatalf("DroppedByDaemon = %+v, want fresh row at line 14", diff.DroppedByDaemon)
	}
}

func TestDiffWriteLogIsHumanGreppable(t *testing.T) {
	added := scanner.Finding{File: "a.kt", Line: 1, Col: 1, RuleSet: "perf", Rule: "Foo", Severity: "warning", Message: "added"}
	dropped := scanner.Finding{File: "b.kt", Line: 2, Col: 1, RuleSet: "style", Rule: "Bar", Severity: "info", Message: "dropped"}
	diff := Diff{
		AddedByDaemon:   []scanner.Finding{added},
		DroppedByDaemon: []scanner.Finding{dropped},
		PathsTouched:    []string{"a.kt", "b.kt"},
	}

	dir := t.TempDir()
	logPath := filepath.Join(dir, "nested", "diff.log")
	if err := diff.WriteLog(logPath); err != nil {
		t.Fatalf("WriteLog: %v", err)
	}

	f, err := os.Open(logPath)
	if err != nil {
		t.Fatalf("open log: %v", err)
	}
	defer f.Close()

	var rows []divergenceLogRow
	scanr := bufio.NewScanner(f)
	for scanr.Scan() {
		var row divergenceLogRow
		if err := json.Unmarshal(scanr.Bytes(), &row); err != nil {
			t.Fatalf("each line must be valid JSON: %v\nline=%q", err, scanr.Text())
		}
		rows = append(rows, row)
	}
	if err := scanr.Err(); err != nil {
		t.Fatalf("scan log: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0].Side != "daemon" || rows[0].File != "a.kt" {
		t.Fatalf("first row = %+v, want daemon-side a.kt", rows[0])
	}
	if rows[1].Side != "baseline" || rows[1].File != "b.kt" {
		t.Fatalf("second row = %+v, want baseline-side b.kt", rows[1])
	}
}

func TestDiffWriteLogCleanProducesEmptyFile(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "diff.log")
	if err := (Diff{}).WriteLog(logPath); err != nil {
		t.Fatalf("WriteLog: %v", err)
	}
	info, err := os.Stat(logPath)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Size() != 0 {
		t.Fatalf("expected empty log for clean diff, got %d bytes", info.Size())
	}
}

func TestNextDivergenceLogPathSequencesNumbers(t *testing.T) {
	repo := t.TempDir()

	first, err := NextDivergenceLogPath(repo)
	if err != nil {
		t.Fatalf("first NextDivergenceLogPath: %v", err)
	}
	wantFirst := filepath.Join(repo, ".krit", "daemon-divergence-0000.log")
	if first != wantFirst {
		t.Fatalf("first path = %s, want %s", first, wantFirst)
	}
	// Materialize the file so the next call sees it.
	if err := os.WriteFile(first, nil, 0o644); err != nil {
		t.Fatalf("touch: %v", err)
	}

	second, err := NextDivergenceLogPath(repo)
	if err != nil {
		t.Fatalf("second NextDivergenceLogPath: %v", err)
	}
	wantSecond := filepath.Join(repo, ".krit", "daemon-divergence-0001.log")
	if second != wantSecond {
		t.Fatalf("second path = %s, want %s", second, wantSecond)
	}

	// Unrelated files in the same dir must be ignored.
	if err := os.WriteFile(filepath.Join(repo, ".krit", "stray.log"), nil, 0o644); err != nil {
		t.Fatalf("stray: %v", err)
	}
	third, err := NextDivergenceLogPath(repo)
	if err != nil {
		t.Fatalf("third NextDivergenceLogPath: %v", err)
	}
	if third != filepath.Join(repo, ".krit", "daemon-divergence-0001.log") {
		t.Fatalf("third path = %s; stray file should not bump the sequence", third)
	}
}

func TestCompareDeterministicOrdering(t *testing.T) {
	rows := []scanner.Finding{
		{File: "z.kt", Line: 1, RuleSet: "perf", Rule: "Foo", Severity: "warning", Message: "z"},
		{File: "a.kt", Line: 2, RuleSet: "perf", Rule: "Foo", Severity: "warning", Message: "a"},
		{File: "m.kt", Line: 5, RuleSet: "perf", Rule: "Foo", Severity: "warning", Message: "m"},
	}
	daemon := scanner.CollectFindings(rows)
	baseline := scanner.CollectFindings(nil)
	diff := Compare(&daemon, &baseline)

	want := []string{"a.kt", "m.kt", "z.kt"}
	got := make([]string, len(diff.AddedByDaemon))
	for i, f := range diff.AddedByDaemon {
		got[i] = f.File
	}
	if !sort.StringsAreSorted(got) {
		t.Fatalf("AddedByDaemon not sorted by file: %v", got)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("AddedByDaemon order = %v, want %v", got, want)
	}
}
