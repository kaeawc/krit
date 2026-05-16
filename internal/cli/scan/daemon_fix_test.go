package scan

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/daemon"
	"github.com/kaeawc/krit/internal/scanner"
)

// TestRunDaemonFix_AppliesTextFixLocally proves the daemon-routed
// --fix path preserves the "daemon never writes user files"
// invariant while still applying the fix the daemon computed. The
// daemon ships FindingColumns under AnalyzeProjectResult.Columns;
// runDaemonFix decodes them, runs pipeline.FixupPhase locally with
// Apply=true, and the on-disk source file ends up rewritten by the
// CLI's process — not the daemon's.
func TestRunDaemonFix_AppliesTextFixLocally(t *testing.T) {
	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "fix.kt")
	original := "foo()\n"
	if err := os.WriteFile(sourcePath, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	cols := scanner.CollectFindings([]scanner.Finding{{
		File:     sourcePath,
		Line:     1,
		Col:      1,
		RuleSet:  "test",
		Rule:     "TestRule",
		Severity: "warning",
		Message:  "swap foo for bar",
		Fix: &scanner.Fix{
			ByteMode:    true,
			StartByte:   0,
			EndByte:     3,
			Replacement: "bar",
		},
	}})
	columnsJSON, err := json.Marshal(cols)
	if err != nil {
		t.Fatalf("marshal columns: %v", err)
	}

	f := freshScanFlags(t)
	*f.Fix = true
	*f.FixLevel = "semantic"

	res := daemon.AnalyzeProjectResult{
		Findings: json.RawMessage(`{"rules":[],"line":[]}`),
		Columns:  json.RawMessage(columnsJSON),
	}
	handled, code := runDaemonFix(f, []string{dir}, res)
	if !handled {
		t.Fatalf("expected handled=true; got false")
	}
	if code != 0 {
		t.Errorf("expected exit 0; got %d", code)
	}
	got, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("read source: %v", err)
	}
	if string(got) != "bar()\n" {
		t.Errorf("file content = %q, want %q", string(got), "bar()\n")
	}
}

// TestRunDaemonFix_DryRunDoesNotWrite confirms the daemon-routed
// --fix --dry-run path mirrors the in-process CountOnly mode: the
// FixupPhase runs the MaxFixLevel filter and fixable counts, but
// the on-disk file is untouched.
func TestRunDaemonFix_DryRunDoesNotWrite(t *testing.T) {
	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "fix.kt")
	original := "foo()\n"
	if err := os.WriteFile(sourcePath, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	cols := scanner.CollectFindings([]scanner.Finding{{
		File:     sourcePath,
		Line:     1,
		Col:      1,
		RuleSet:  "test",
		Rule:     "TestRule",
		Severity: "warning",
		Message:  "swap foo for bar",
		Fix: &scanner.Fix{
			ByteMode:    true,
			StartByte:   0,
			EndByte:     3,
			Replacement: "bar",
		},
	}})
	columnsJSON, err := json.Marshal(cols)
	if err != nil {
		t.Fatalf("marshal columns: %v", err)
	}

	f := freshScanFlags(t)
	*f.Fix = true
	*f.DryRun = true
	*f.FixLevel = "semantic"

	res := daemon.AnalyzeProjectResult{
		Findings: json.RawMessage(`{"rules":[],"line":[]}`),
		Columns:  json.RawMessage(columnsJSON),
	}
	if _, code := runDaemonFix(f, []string{dir}, res); code != 0 && code != 1 {
		t.Errorf("expected exit 0 or 1; got %d", code)
	}
	got, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("read source: %v", err)
	}
	if string(got) != original {
		t.Errorf("--dry-run rewrote file: got %q, want %q", string(got), original)
	}
}

// TestRunDaemonFix_MaxFixLevelStripsBeforeApply proves the CLI-side
// MaxFixLevel filter runs against the daemon-shipped columns: a fix
// from a rule above the requested level is dropped, so the file
// stays unchanged even though --fix is set.
func TestRunDaemonFix_MaxFixLevelStripsBeforeApply(t *testing.T) {
	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "fix.kt")
	original := "foo()\n"
	if err := os.WriteFile(sourcePath, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	cols := scanner.CollectFindings([]scanner.Finding{{
		File:     sourcePath,
		Line:     1,
		Col:      1,
		RuleSet:  "test",
		Rule:     "UnknownRuleAboveCosmetic",
		Severity: "warning",
		Message:  "would swap foo for bar",
		Fix: &scanner.Fix{
			ByteMode:    true,
			StartByte:   0,
			EndByte:     3,
			Replacement: "bar",
		},
	}})
	columnsJSON, err := json.Marshal(cols)
	if err != nil {
		t.Fatalf("marshal columns: %v", err)
	}

	f := freshScanFlags(t)
	*f.Fix = true
	*f.FixLevel = "cosmetic" // unknown rules default to FixSemantic, > cosmetic → stripped

	res := daemon.AnalyzeProjectResult{
		Findings: json.RawMessage(`{"rules":[],"line":[]}`),
		Columns:  json.RawMessage(columnsJSON),
	}
	if _, code := runDaemonFix(f, []string{dir}, res); code != 0 && code != 1 {
		t.Errorf("expected exit 0 or 1; got %d", code)
	}
	got, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("read source: %v", err)
	}
	if string(got) != original {
		t.Errorf("MaxFixLevel filter failed to strip; file rewritten to %q", string(got))
	}
}

// TestRunDaemonRemoveDeadCode_DryRunReportsPlan exercises the
// daemon-routed --remove-dead-code dry-run path: the CLI replays
// RunDeadCodeRemovalColumns against the daemon-shipped columns and
// produces the standard report without rewriting disk.
func TestRunDaemonRemoveDeadCode_DryRunReportsPlan(t *testing.T) {
	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "dead.kt")
	original := "fun dead() {}\n"
	if err := os.WriteFile(sourcePath, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	cols := scanner.CollectFindings(nil)
	columnsJSON, err := json.Marshal(cols)
	if err != nil {
		t.Fatalf("marshal columns: %v", err)
	}

	f := freshScanFlags(t)
	*f.RemoveDeadCode = true
	*f.DryRun = true

	if code := runDaemonRemoveDeadCode(f, []string{dir}, json.RawMessage(columnsJSON)); code != 0 {
		t.Errorf("expected exit 0 on empty dry-run; got %d", code)
	}
	got, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatalf("read source: %v", err)
	}
	if string(got) != original {
		t.Errorf("--dry-run rewrote file: got %q, want %q", string(got), original)
	}
}

// TestRunDaemonFix_MissingColumnsErrors proves runDaemonFix surfaces
// a typed error when the daemon ships an empty columns payload (the
// CLI only sets IncludeColumns when the fix flags require it, so an
// empty columns segment means a daemon-side bug worth surfacing).
func TestRunDaemonFix_MissingColumnsErrors(t *testing.T) {
	f := freshScanFlags(t)
	*f.Fix = true

	res := daemon.AnalyzeProjectResult{
		Findings: json.RawMessage(`{"rules":[],"line":[]}`),
	}
	handled, code := runDaemonFix(f, []string{t.TempDir()}, res)
	if !handled {
		t.Fatalf("expected handled=true on column decode error")
	}
	if code != 2 {
		t.Errorf("expected exit 2 on decode error; got %d", code)
	}
}
