package fixer

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

func writeTestFile(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.kt")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	return path
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}
	return string(data)
}

// ApplyFixes / ApplyFixesWithValidation / ApplyFixesDetailed are test-only
// shims that preserve the old API shape by routing through the columnar
// fixer entry point. They exist so the test suite can continue to exercise
// per-file fix application without a wholesale rewrite.
func ApplyFixes(ctx context.Context, path string, findings []scanner.Finding, suffix string) (int, error) {
	return ApplyFixesWithValidation(ctx, path, findings, suffix, false)
}

func ApplyFixesWithValidation(ctx context.Context, path string, findings []scanner.Finding, suffix string, validate bool) (int, error) {
	res, err := ApplyFixesDetailed(ctx, path, findings, suffix, validate)
	return res.Applied, err
}

func ApplyFixesDetailed(ctx context.Context, path string, findings []scanner.Finding, suffix string, validate bool) (FixResult, error) {
	filtered := make([]scanner.Finding, 0, len(findings))
	for _, f := range findings {
		if f.File == path && f.Fix != nil {
			filtered = append(filtered, f)
		}
	}
	if len(filtered) == 0 {
		return FixResult{}, nil
	}
	columns := scanner.CollectFindings(filtered)
	rows := make([]int, 0, columns.Len())
	for i := 0; i < columns.Len(); i++ {
		if columns.HasFix(i) {
			rows = append(rows, i)
		}
	}
	return applyFixesDetailedColumns(ctx, path, &columns, rows, suffix, validate)
}

func ApplyAllFixes(ctx context.Context, findings []scanner.Finding, suffix string) (int, int, []error) {
	columns := scanner.CollectFindings(findings)
	return ApplyAllFixesColumns(ctx, &columns, suffix)
}

func TestApplyAllFixesColumns_TargetFile(t *testing.T) {
	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "build.gradle.kts")
	targetPath := filepath.Join(dir, "krit.yml")
	if err := os.WriteFile(sourcePath, []byte("dependencies {}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	columns := scanner.CollectFindings([]scanner.Finding{{
		File:     sourcePath,
		Line:     1,
		Col:      1,
		RuleSet:  "supply-chain",
		Rule:     "DependenciesInRootProject",
		Severity: "warning",
		Message:  "root dependencies",
		Fix: &scanner.Fix{
			TargetFile:  targetPath,
			ByteMode:    true,
			StartByte:   0,
			EndByte:     0,
			Replacement: "supply-chain:\n",
		},
	}})

	applied, modified, errs := ApplyAllFixesColumns(t.Context(), &columns, "")
	if len(errs) > 0 {
		t.Fatalf("ApplyAllFixesColumns errors: %v", errs)
	}
	if applied != 1 || modified != 1 {
		t.Fatalf("applied=%d modified=%d, want 1/1", applied, modified)
	}
	if got := readFile(t, targetPath); got != "supply-chain:\n" {
		t.Fatalf("target content = %q", got)
	}
	if got := readFile(t, sourcePath); got != "dependencies {}\n" {
		t.Fatalf("source content changed: %q", got)
	}
}

// splitByMode is a test-only helper kept after the production splitByMode
// was deleted with ApplyFixesDetailed. It mirrors the old semantics so
// the few remaining split-by-mode tests continue to pass.
func splitByMode(fixes []scanner.Finding) (byteFixes, lineFixes []scanner.Finding) {
	for _, f := range fixes {
		if f.Fix != nil && f.Fix.ByteMode {
			byteFixes = append(byteFixes, f)
		} else {
			lineFixes = append(lineFixes, f)
		}
	}
	return
}

func finding(file string, fix *scanner.Fix) scanner.Finding {
	return scanner.Finding{
		File:     file,
		Line:     1,
		RuleSet:  "test",
		Rule:     "test-rule",
		Severity: "warning",
		Message:  "test finding",
		Fix:      fix,
	}
}

// --------------- Byte-mode tests ---------------

func TestByteModeFix_SimpleReplace(t *testing.T) {
	path := writeTestFile(t, "hello world")
	findings := []scanner.Finding{
		finding(path, &scanner.Fix{
			StartByte:   0,
			EndByte:     5,
			Replacement: "world",
			ByteMode:    true,
		}),
	}

	n, err := ApplyFixes(t.Context(), path, findings, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 fix applied, got %d", n)
	}
	got := readFile(t, path)
	if got != "world world" {
		t.Fatalf("expected %q, got %q", "world world", got)
	}
}

func TestByteModeFix_Deletion(t *testing.T) {
	path := writeTestFile(t, "hello world")
	findings := []scanner.Finding{
		finding(path, &scanner.Fix{
			StartByte:   5,
			EndByte:     11,
			Replacement: "",
			ByteMode:    true,
		}),
	}

	n, err := ApplyFixes(t.Context(), path, findings, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 fix applied, got %d", n)
	}
	got := readFile(t, path)
	if got != "hello" {
		t.Fatalf("expected %q, got %q", "hello", got)
	}
}

func TestByteModeFix_Insertion(t *testing.T) {
	path := writeTestFile(t, "helloworld")
	findings := []scanner.Finding{
		finding(path, &scanner.Fix{
			StartByte:   5,
			EndByte:     5,
			Replacement: " ",
			ByteMode:    true,
		}),
	}

	n, err := ApplyFixes(t.Context(), path, findings, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 fix applied, got %d", n)
	}
	got := readFile(t, path)
	if got != "hello world" {
		t.Fatalf("expected %q, got %q", "hello world", got)
	}
}

func TestByteModeFix_MultipleNonOverlapping(t *testing.T) {
	path := writeTestFile(t, "aaa bbb ccc")
	findings := []scanner.Finding{
		finding(path, &scanner.Fix{
			StartByte:   0,
			EndByte:     3,
			Replacement: "AAA",
			ByteMode:    true,
		}),
		finding(path, &scanner.Fix{
			StartByte:   8,
			EndByte:     11,
			Replacement: "CCC",
			ByteMode:    true,
		}),
	}

	n, err := ApplyFixes(t.Context(), path, findings, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 2 {
		t.Fatalf("expected 2 fixes applied, got %d", n)
	}
	got := readFile(t, path)
	if got != "AAA bbb CCC" {
		t.Fatalf("expected %q, got %q", "AAA bbb CCC", got)
	}
}

func TestByteModeFix_OverlappingDedup(t *testing.T) {
	path := writeTestFile(t, "abcdefghij")
	findings := []scanner.Finding{
		finding(path, &scanner.Fix{
			StartByte:   2,
			EndByte:     6,
			Replacement: "XXXX",
			ByteMode:    true,
		}),
		finding(path, &scanner.Fix{
			StartByte:   4,
			EndByte:     8,
			Replacement: "YYYY",
			ByteMode:    true,
		}),
	}

	n, err := ApplyFixes(t.Context(), path, findings, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Only the surviving (non-overlapping) fix is counted.
	if n != 1 {
		t.Fatalf("expected 1 fix applied after overlap dedup, got %d", n)
	}
	got := readFile(t, path)
	// The fix with higher StartByte (4-8) is applied first in reverse order.
	// Then the fix (2-6) overlaps with the already-applied fix and is skipped.
	// So only "YYYY" is applied: "abcd" + "YYYY" + "ij" = "abcdYYYYij"
	expected := "abcdYYYYij"
	if got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

// --------------- Line-mode tests ---------------

func TestLineModeFix_ReplaceLine(t *testing.T) {
	path := writeTestFile(t, "line1\nline2\nline3")
	findings := []scanner.Finding{
		finding(path, &scanner.Fix{
			StartLine:   2,
			EndLine:     2,
			Replacement: "replaced",
		}),
	}

	n, err := ApplyFixes(t.Context(), path, findings, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 fix applied, got %d", n)
	}
	got := readFile(t, path)
	if got != "line1\nreplaced\nline3" {
		t.Fatalf("expected %q, got %q", "line1\nreplaced\nline3", got)
	}
}

func TestLineModeFix_DeleteLine(t *testing.T) {
	path := writeTestFile(t, "line1\nline2\nline3")
	findings := []scanner.Finding{
		finding(path, &scanner.Fix{
			StartLine:   2,
			EndLine:     2,
			Replacement: "",
		}),
	}

	n, err := ApplyFixes(t.Context(), path, findings, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 fix applied, got %d", n)
	}
	got := readFile(t, path)
	if got != "line1\nline3" {
		t.Fatalf("expected %q, got %q", "line1\nline3", got)
	}
}

func TestLineModeFix_MultiLine(t *testing.T) {
	path := writeTestFile(t, "line1\nline2\nline3\nline4\nline5")
	findings := []scanner.Finding{
		finding(path, &scanner.Fix{
			StartLine:   2,
			EndLine:     4,
			Replacement: "replaced",
		}),
	}

	n, err := ApplyFixes(t.Context(), path, findings, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 fix applied, got %d", n)
	}
	got := readFile(t, path)
	if got != "line1\nreplaced\nline5" {
		t.Fatalf("expected %q, got %q", "line1\nreplaced\nline5", got)
	}
}

// TestLineModeFix_TrailingNewlineIdempotent guards against a regression in
// applyLineFixes where strings.Split("// noop\n", "\n") yielded
// ["// noop", ""], so each --fix pass inserted an extra blank line. Two
// passes with the same Replacement must produce the same content as one.
func TestLineModeFix_TrailingNewlineIdempotent(t *testing.T) {
	path := writeTestFile(t, "line1\nline2\nline3")
	makeFinding := func() scanner.Finding {
		return finding(path, &scanner.Fix{
			StartLine:   2,
			EndLine:     2,
			Replacement: "// noop\n",
		})
	}

	if _, err := ApplyFixes(t.Context(), path, []scanner.Finding{makeFinding()}, ""); err != nil {
		t.Fatalf("first pass: %v", err)
	}
	afterFirst := readFile(t, path)
	if afterFirst != "line1\n// noop\nline3" {
		t.Fatalf("after first pass: got %q, want %q", afterFirst, "line1\n// noop\nline3")
	}

	if _, err := ApplyFixes(t.Context(), path, []scanner.Finding{makeFinding()}, ""); err != nil {
		t.Fatalf("second pass: %v", err)
	}
	afterSecond := readFile(t, path)
	if afterSecond != afterFirst {
		t.Fatalf("second pass not idempotent: got %q, want %q", afterSecond, afterFirst)
	}
}

func TestLineModeFix_PreservesCRLF(t *testing.T) {
	path := writeTestFile(t, "line1\r\nline2\r\nline3\r\n")
	findings := []scanner.Finding{
		finding(path, &scanner.Fix{
			StartLine:   2,
			EndLine:     2,
			Replacement: "replaced",
		}),
	}

	n, err := ApplyFixes(t.Context(), path, findings, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 fix applied, got %d", n)
	}
	got := readFile(t, path)
	want := "line1\r\nreplaced\r\nline3\r\n"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestLineModeFix_CRLF_MultiLineReplacement(t *testing.T) {
	path := writeTestFile(t, "line1\r\nline2\r\nline3\r\n")
	findings := []scanner.Finding{
		finding(path, &scanner.Fix{
			StartLine:   2,
			EndLine:     2,
			Replacement: "a\nb",
		}),
	}

	n, err := ApplyFixes(t.Context(), path, findings, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 fix applied, got %d", n)
	}
	got := readFile(t, path)
	want := "line1\r\na\r\nb\r\nline3\r\n"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestLineModeFix_CRLF_DeleteLine(t *testing.T) {
	path := writeTestFile(t, "line1\r\nline2\r\nline3\r\n")
	findings := []scanner.Finding{
		finding(path, &scanner.Fix{
			StartLine:   2,
			EndLine:     2,
			Replacement: "",
		}),
	}

	n, err := ApplyFixes(t.Context(), path, findings, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 fix applied, got %d", n)
	}
	got := readFile(t, path)
	want := "line1\r\nline3\r\n"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

// --------------- Edge cases ---------------

func TestNoFixesApplied(t *testing.T) {
	content := "unchanged content"
	path := writeTestFile(t, content)

	// Finding without a fix
	findings := []scanner.Finding{
		{File: path, Line: 1, Rule: "test", Message: "no fix"},
	}

	n, err := ApplyFixes(t.Context(), path, findings, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected 0 fixes applied, got %d", n)
	}
	got := readFile(t, path)
	if got != content {
		t.Fatalf("file should be unchanged, got %q", got)
	}
}

func TestIdenticalContent(t *testing.T) {
	path := writeTestFile(t, "hello")
	findings := []scanner.Finding{
		finding(path, &scanner.Fix{
			StartByte:   0,
			EndByte:     5,
			Replacement: "hello",
			ByteMode:    true,
		}),
	}

	n, err := ApplyFixes(t.Context(), path, findings, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Fix produces identical content, so 0 returned
	if n != 0 {
		t.Fatalf("expected 0 fixes applied (identical content), got %d", n)
	}
}

func TestSuffixMode(t *testing.T) {
	original := "original content"
	path := writeTestFile(t, original)
	findings := []scanner.Finding{
		finding(path, &scanner.Fix{
			StartByte:   0,
			EndByte:     8,
			Replacement: "modified",
			ByteMode:    true,
		}),
	}

	n, err := ApplyFixes(t.Context(), path, findings, ".new")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 fix applied, got %d", n)
	}

	// Original should be unchanged
	origContent := readFile(t, path)
	if origContent != original {
		t.Fatalf("original file should be unchanged, got %q", origContent)
	}

	// New file should have the fix
	newContent := readFile(t, path+".new")
	expected := "modified content"
	if newContent != expected {
		t.Fatalf("expected %q, got %q", expected, newContent)
	}
}

func TestApplyAllFixes_MultipleFiles(t *testing.T) {
	dir := t.TempDir()
	path1 := filepath.Join(dir, "file1.kt")
	path2 := filepath.Join(dir, "file2.kt")
	os.WriteFile(path1, []byte("aaa"), 0644)
	os.WriteFile(path2, []byte("bbb"), 0644)

	findings := []scanner.Finding{
		{
			File: path1, Line: 1, Rule: "r1", Message: "m1",
			Fix: &scanner.Fix{StartByte: 0, EndByte: 3, Replacement: "AAA", ByteMode: true},
		},
		{
			File: path2, Line: 1, Rule: "r2", Message: "m2",
			Fix: &scanner.Fix{StartByte: 0, EndByte: 3, Replacement: "BBB", ByteMode: true},
		},
	}

	totalFixes, filesModified, errs := ApplyAllFixes(t.Context(), findings, "")
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if totalFixes != 2 {
		t.Fatalf("expected 2 total fixes, got %d", totalFixes)
	}
	if filesModified != 2 {
		t.Fatalf("expected 2 files modified, got %d", filesModified)
	}

	got1 := readFile(t, path1)
	if got1 != "AAA" {
		t.Fatalf("file1: expected %q, got %q", "AAA", got1)
	}
	got2 := readFile(t, path2)
	if got2 != "BBB" {
		t.Fatalf("file2: expected %q, got %q", "BBB", got2)
	}
}

func TestApplyAllFixesColumns_MatchesSliceBehavior(t *testing.T) {
	makeFixture := func(t *testing.T) ([]scanner.Finding, scanner.FindingColumns, map[string]string) {
		t.Helper()

		dir := t.TempDir()
		path1 := filepath.Join(dir, "file1.kt")
		path2 := filepath.Join(dir, "file2.kt")
		if err := os.WriteFile(path1, []byte("aaa"), 0o644); err != nil {
			t.Fatalf("write file1: %v", err)
		}
		if err := os.WriteFile(path2, []byte("bbb"), 0o644); err != nil {
			t.Fatalf("write file2: %v", err)
		}

		findings := []scanner.Finding{
			{
				File: path1, Line: 1, Rule: "r1", Message: "m1",
				Fix: &scanner.Fix{StartByte: 0, EndByte: 3, Replacement: "AAA", ByteMode: true},
			},
			{
				File: path2, Line: 1, Rule: "r2", Message: "m2",
				Fix: &scanner.Fix{StartByte: 0, EndByte: 3, Replacement: "BBB", ByteMode: true},
			},
			{
				File: path2, Line: 1, Rule: "r3", Message: "no fix",
			},
		}

		columns := scanner.CollectFindings(findings)
		return findings, columns, map[string]string{
			path1: "AAA",
			path2: "BBB",
		}
	}

	sliceFindings, _, wantFiles := makeFixture(t)
	wantFixes, wantModified, wantErrs := ApplyAllFixes(t.Context(), sliceFindings, "")
	if len(wantErrs) > 0 {
		t.Fatalf("slice ApplyAllFixes errors: %v", wantErrs)
	}

	_, columns, gotFiles := makeFixture(t)
	gotFixes, gotModified, gotErrs := ApplyAllFixesColumns(t.Context(), &columns, "")
	if len(gotErrs) > 0 {
		t.Fatalf("columnar ApplyAllFixesColumns errors: %v", gotErrs)
	}

	if gotFixes != wantFixes {
		t.Fatalf("fix count mismatch: want %d, got %d", wantFixes, gotFixes)
	}
	if gotModified != wantModified {
		t.Fatalf("files modified mismatch: want %d, got %d", wantModified, gotModified)
	}
	for path, want := range wantFiles {
		if got := readFile(t, path); got != want {
			t.Fatalf("slice output for %s: want %q, got %q", path, want, got)
		}
	}
	for path, want := range gotFiles {
		if got := readFile(t, path); got != want {
			t.Fatalf("columnar output for %s: want %q, got %q", path, want, got)
		}
	}
}

func TestApplyFixesDetailedColumns_ReportsDroppedOverlapLikeSlicePath(t *testing.T) {
	makeFindings := func(path string) []scanner.Finding {
		return []scanner.Finding{
			findingWithRule(path, "rule-A", &scanner.Fix{
				StartByte:   2,
				EndByte:     6,
				Replacement: "XXXX",
				ByteMode:    true,
			}),
			findingWithRule(path, "rule-B", &scanner.Fix{
				StartByte:   4,
				EndByte:     8,
				Replacement: "YYYY",
				ByteMode:    true,
			}),
		}
	}

	path := writeTestFile(t, "abcdefghij")
	want, err := ApplyFixesDetailed(t.Context(), path, makeFindings(path), "", false)
	if err != nil {
		t.Fatalf("slice ApplyFixesDetailed error: %v", err)
	}

	path = writeTestFile(t, "abcdefghij")
	columns := scanner.CollectFindings(makeFindings(path))
	got, err := applyFixesDetailedColumns(t.Context(), path, &columns, []int{0, 1}, "", false)
	if err != nil {
		t.Fatalf("columnar applyFixesDetailedColumns error: %v", err)
	}

	if got.Applied != want.Applied {
		t.Fatalf("applied mismatch: want %d, got %d", want.Applied, got.Applied)
	}
	if len(got.DroppedFixes) != len(want.DroppedFixes) {
		t.Fatalf("dropped fix count mismatch: want %d, got %d", len(want.DroppedFixes), len(got.DroppedFixes))
	}
	for i := range want.DroppedFixes {
		if got.DroppedFixes[i].Rule != want.DroppedFixes[i].Rule || got.DroppedFixes[i].Line != want.DroppedFixes[i].Line {
			t.Fatalf("dropped fix mismatch at %d:\nwant: %#v\ngot:  %#v", i, want.DroppedFixes[i], got.DroppedFixes[i])
		}
	}
}

func TestBoundsCheck_InvalidRange(t *testing.T) {
	path := writeTestFile(t, "hello")
	findings := []scanner.Finding{
		// StartByte beyond file length
		finding(path, &scanner.Fix{
			StartByte:   100,
			EndByte:     200,
			Replacement: "x",
			ByteMode:    true,
		}),
	}

	n, err := ApplyFixes(t.Context(), path, findings, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Invalid range is skipped, content unchanged, so 0 fixes
	if n != 0 {
		t.Fatalf("expected 0 fixes (invalid range skipped), got %d", n)
	}
	got := readFile(t, path)
	if got != "hello" {
		t.Fatalf("file should be unchanged, got %q", got)
	}
}

func TestBoundsCheck_NegativeStartByte(t *testing.T) {
	path := writeTestFile(t, "hello")
	findings := []scanner.Finding{
		finding(path, &scanner.Fix{
			StartByte:   -1,
			EndByte:     3,
			Replacement: "x",
			ByteMode:    true,
		}),
	}

	n, err := ApplyFixes(t.Context(), path, findings, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected 0 fixes (negative start skipped), got %d", n)
	}
	got := readFile(t, path)
	if got != "hello" {
		t.Fatalf("file should be unchanged, got %q", got)
	}
}

func TestBoundsCheck_StartGreaterThanEnd(t *testing.T) {
	path := writeTestFile(t, "hello")
	findings := []scanner.Finding{
		finding(path, &scanner.Fix{
			StartByte:   4,
			EndByte:     2,
			Replacement: "x",
			ByteMode:    true,
		}),
	}

	n, err := ApplyFixes(t.Context(), path, findings, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected 0 fixes, got %d", n)
	}
}

func TestLineModeFix_InvalidLineRange(t *testing.T) {
	path := writeTestFile(t, "line1\nline2\nline3")
	findings := []scanner.Finding{
		finding(path, &scanner.Fix{
			StartLine:   10,
			EndLine:     20,
			Replacement: "x",
		}),
	}

	n, err := ApplyFixes(t.Context(), path, findings, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 0 {
		t.Fatalf("expected 0 fixes (out-of-bounds lines skipped), got %d", n)
	}
	got := readFile(t, path)
	if got != "line1\nline2\nline3" {
		t.Fatalf("file should be unchanged, got %q", got)
	}
}

// --------------- splitByMode tests ---------------

func TestSplitByMode_Empty(t *testing.T) {
	byteFixes, lineFixes := splitByMode(nil)
	if len(byteFixes) != 0 || len(lineFixes) != 0 {
		t.Fatal("expected empty slices for nil input")
	}
}

func TestSplitByMode_OnlyByte(t *testing.T) {
	fixes := []scanner.Finding{
		finding("f.kt", &scanner.Fix{StartByte: 0, EndByte: 5, ByteMode: true}),
		finding("f.kt", &scanner.Fix{StartByte: 10, EndByte: 15, ByteMode: true}),
	}
	byteFixes, lineFixes := splitByMode(fixes)
	if len(byteFixes) != 2 {
		t.Fatalf("expected 2 byte fixes, got %d", len(byteFixes))
	}
	if len(lineFixes) != 0 {
		t.Fatalf("expected 0 line fixes, got %d", len(lineFixes))
	}
}

func TestSplitByMode_OnlyLine(t *testing.T) {
	fixes := []scanner.Finding{
		finding("f.kt", &scanner.Fix{StartLine: 1, EndLine: 1}),
	}
	byteFixes, lineFixes := splitByMode(fixes)
	if len(byteFixes) != 0 {
		t.Fatalf("expected 0 byte fixes, got %d", len(byteFixes))
	}
	if len(lineFixes) != 1 {
		t.Fatalf("expected 1 line fix, got %d", len(lineFixes))
	}
}

func TestSplitByMode_Mixed(t *testing.T) {
	fixes := []scanner.Finding{
		finding("f.kt", &scanner.Fix{StartByte: 0, EndByte: 5, ByteMode: true}),
		finding("f.kt", &scanner.Fix{StartLine: 3, EndLine: 3}),
		finding("f.kt", &scanner.Fix{StartByte: 10, EndByte: 15, ByteMode: true}),
	}
	byteFixes, lineFixes := splitByMode(fixes)
	if len(byteFixes) != 2 {
		t.Fatalf("expected 2 byte fixes, got %d", len(byteFixes))
	}
	if len(lineFixes) != 1 {
		t.Fatalf("expected 1 line fix, got %d", len(lineFixes))
	}
}

// --------------- Mixed-mode rejection tests ---------------

func TestMixedMode_ReturnsError(t *testing.T) {
	path := writeTestFile(t, "aaa\nbbb\nccc")
	findings := []scanner.Finding{
		finding(path, &scanner.Fix{
			StartByte:   0,
			EndByte:     3,
			Replacement: "AAA",
			ByteMode:    true,
		}),
		finding(path, &scanner.Fix{
			StartLine:   3,
			EndLine:     3,
			Replacement: "CCC",
		}),
	}

	_, err := ApplyFixes(t.Context(), path, findings, "")
	if err == nil {
		t.Fatalf("expected mixed-mode error, got nil")
	}
	if !strings.Contains(err.Error(), "mixed-mode fixes") {
		t.Fatalf("expected mixed-mode error, got %v", err)
	}
	// File must remain unchanged when the mixed-mode conflict is detected.
	got := readFile(t, path)
	if got != "aaa\nbbb\nccc" {
		t.Fatalf("expected file unchanged, got %q", got)
	}
}

func TestMixedMode_OnlyByte_NoRejection(t *testing.T) {
	path := writeTestFile(t, "aaa bbb")
	findings := []scanner.Finding{
		finding(path, &scanner.Fix{
			StartByte:   0,
			EndByte:     3,
			Replacement: "AAA",
			ByteMode:    true,
		}),
	}
	n, err := ApplyFixes(t.Context(), path, findings, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 fix, got %d", n)
	}
	got := readFile(t, path)
	if got != "AAA bbb" {
		t.Fatalf("expected %q, got %q", "AAA bbb", got)
	}
}

func TestMixedMode_OnlyLine_NoRejection(t *testing.T) {
	path := writeTestFile(t, "line1\nline2\nline3")
	findings := []scanner.Finding{
		finding(path, &scanner.Fix{
			StartLine:   2,
			EndLine:     2,
			Replacement: "REPLACED",
		}),
	}
	n, err := ApplyFixes(t.Context(), path, findings, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 fix, got %d", n)
	}
	got := readFile(t, path)
	if got != "line1\nREPLACED\nline3" {
		t.Fatalf("expected %q, got %q", "line1\nREPLACED\nline3", got)
	}
}

// --------------- Post-fix validation tests ---------------

func TestValidateFixResult_ValidKotlin(t *testing.T) {
	original := []byte("fun main() { println(\"hello\") }")
	fixed := []byte("fun main() { println(\"world\") }")
	err := ValidateFixResult(t.Context(), "test.kt", original, fixed)
	if err != nil {
		t.Fatalf("expected no error for valid Kotlin, got: %v", err)
	}
}

func TestValidateFixResult_InvalidKotlin(t *testing.T) {
	original := []byte("fun main() { println(\"hello\") }")
	fixed := []byte("fun main() { println(\"hello\"")
	err := ValidateFixResult(t.Context(), "test.kt", original, fixed)
	if err == nil {
		t.Fatal("expected error for broken Kotlin syntax")
	}
	if !strings.Contains(err.Error(), "parse errors") {
		t.Fatalf("expected parse errors message, got: %v", err)
	}
}

func TestValidateFixResult_NonKotlinFile(t *testing.T) {
	// Non-Kotlin files are skipped
	err := ValidateFixResult(t.Context(), "test.java", []byte("broken{{{"), []byte("also broken{{{"))
	if err != nil {
		t.Fatalf("expected no error for non-Kotlin file, got: %v", err)
	}
}

func TestValidateFixResult_KtsFile(t *testing.T) {
	original := []byte("val x = 1")
	fixed := []byte("val x = {{{")
	err := ValidateFixResult(t.Context(), "build.gradle.kts", original, fixed)
	if err == nil {
		t.Fatal("expected error for broken .kts syntax")
	}
}

func TestApplyFixesWithValidation_RejectsInvalidFix(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.kt")
	original := "fun main() { println(\"hello\") }\n"
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	findings := []scanner.Finding{
		finding(path, &scanner.Fix{
			StartByte:   0,
			EndByte:     len(original) - 1,
			Replacement: "fun main() { println(\"hello\"", // missing closing brace/paren
			ByteMode:    true,
		}),
	}

	_, err := ApplyFixesWithValidation(t.Context(), path, findings, "", true)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "fix validation failed") {
		t.Fatalf("expected validation failure message, got: %v", err)
	}

	// Original file should be unchanged since validation failed before write
	got := readFile(t, path)
	if got != original {
		t.Fatalf("original file should be unchanged after validation failure, got %q", got)
	}
}

func TestApplyFixesWithValidation_AcceptsValidFix(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.kt")
	original := "fun main() { println(\"hello\") }\n"
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	replacement := "fun main() { println(\"world\") }\n"
	findings := []scanner.Finding{
		finding(path, &scanner.Fix{
			StartByte:   0,
			EndByte:     len(original),
			Replacement: replacement,
			ByteMode:    true,
		}),
	}

	n, err := ApplyFixesWithValidation(t.Context(), path, findings, "", true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 fix applied, got %d", n)
	}
	got := readFile(t, path)
	if got != replacement {
		t.Fatalf("expected %q, got %q", replacement, got)
	}
}

// --------------- Conflict detection tests ---------------

func findingWithRule(file, rule string, fix *scanner.Fix) scanner.Finding {
	return scanner.Finding{
		File:     file,
		Line:     1,
		RuleSet:  "test",
		Rule:     rule,
		Severity: "warning",
		Message:  "test finding",
		Fix:      fix,
	}
}

func TestByteConflictDetection_DroppedFixReported(t *testing.T) {
	path := writeTestFile(t, "abcdefghij")
	findings := []scanner.Finding{
		findingWithRule(path, "rule-A", &scanner.Fix{
			StartByte:   2,
			EndByte:     6,
			Replacement: "XXXX",
			ByteMode:    true,
		}),
		findingWithRule(path, "rule-B", &scanner.Fix{
			StartByte:   4,
			EndByte:     8,
			Replacement: "YYYY",
			ByteMode:    true,
		}),
	}

	res, err := ApplyFixesDetailed(t.Context(), path, findings, "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Applied is the post-dedup count: 2 submitted - 1 dropped = 1.
	if res.Applied != 1 {
		t.Fatalf("expected 1 fix applied (post-dedup), got %d", res.Applied)
	}
	if len(res.DroppedFixes) != 1 {
		t.Fatalf("expected 1 dropped fix, got %d", len(res.DroppedFixes))
	}
	if res.DroppedFixes[0].Rule != "rule-A" {
		t.Fatalf("expected dropped rule 'rule-A', got %q", res.DroppedFixes[0].Rule)
	}
	if res.DroppedFixes[0].Reason == "" {
		t.Fatalf("expected non-empty Reason on dropped fix, got %#v", res.DroppedFixes[0])
	}
}

func TestByteConflictDetection_NoOverlap_NoDrop(t *testing.T) {
	path := writeTestFile(t, "aaa bbb ccc")
	findings := []scanner.Finding{
		findingWithRule(path, "rule-A", &scanner.Fix{
			StartByte:   0,
			EndByte:     3,
			Replacement: "AAA",
			ByteMode:    true,
		}),
		findingWithRule(path, "rule-B", &scanner.Fix{
			StartByte:   8,
			EndByte:     11,
			Replacement: "CCC",
			ByteMode:    true,
		}),
	}

	res, err := ApplyFixesDetailed(t.Context(), path, findings, "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Applied != 2 {
		t.Fatalf("expected 2 fixes applied, got %d", res.Applied)
	}
	if len(res.DroppedFixes) != 0 {
		t.Fatalf("expected 0 dropped fixes, got %d", len(res.DroppedFixes))
	}
}

func TestLineConflictDetection_DroppedFixReported(t *testing.T) {
	path := writeTestFile(t, "line1\nline2\nline3\nline4\nline5")
	findings := []scanner.Finding{
		findingWithRule(path, "rule-A", &scanner.Fix{
			StartLine:   2,
			EndLine:     4,
			Replacement: "replaced-A",
		}),
		findingWithRule(path, "rule-B", &scanner.Fix{
			StartLine:   3,
			EndLine:     5,
			Replacement: "replaced-B",
		}),
	}

	res, err := ApplyFixesDetailed(t.Context(), path, findings, "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.DroppedFixes) != 1 {
		t.Fatalf("expected 1 dropped fix, got %d", len(res.DroppedFixes))
	}
	// The fix with higher StartLine (3-5) is applied first in reverse order.
	// Then the fix (2-4) overlaps (EndLine 4 > lastStart 3) and is dropped.
	if res.DroppedFixes[0].Rule != "rule-A" {
		t.Fatalf("expected dropped rule 'rule-A', got %q", res.DroppedFixes[0].Rule)
	}
}

func TestLineConflictDetection_NoOverlap_NoDrop(t *testing.T) {
	path := writeTestFile(t, "line1\nline2\nline3\nline4\nline5")
	findings := []scanner.Finding{
		findingWithRule(path, "rule-A", &scanner.Fix{
			StartLine:   1,
			EndLine:     1,
			Replacement: "REPLACED1",
		}),
		findingWithRule(path, "rule-B", &scanner.Fix{
			StartLine:   4,
			EndLine:     4,
			Replacement: "REPLACED4",
		}),
	}

	res, err := ApplyFixesDetailed(t.Context(), path, findings, "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.DroppedFixes) != 0 {
		t.Fatalf("expected 0 dropped fixes, got %d", len(res.DroppedFixes))
	}
	got := readFile(t, path)
	if got != "REPLACED1\nline2\nline3\nREPLACED4\nline5" {
		t.Fatalf("expected %q, got %q", "REPLACED1\nline2\nline3\nREPLACED4\nline5", got)
	}
}

func TestByteConflictDetection_MultipleDrops(t *testing.T) {
	path := writeTestFile(t, "abcdefghijklmnop")
	findings := []scanner.Finding{
		findingWithRule(path, "rule-A", &scanner.Fix{
			StartByte: 0, EndByte: 4, Replacement: "AAAA", ByteMode: true,
		}),
		findingWithRule(path, "rule-B", &scanner.Fix{
			StartByte: 2, EndByte: 6, Replacement: "BBBB", ByteMode: true,
		}),
		findingWithRule(path, "rule-C", &scanner.Fix{
			StartByte: 8, EndByte: 12, Replacement: "CCCC", ByteMode: true,
		}),
		findingWithRule(path, "rule-D", &scanner.Fix{
			StartByte: 10, EndByte: 14, Replacement: "DDDD", ByteMode: true,
		}),
	}

	res, err := ApplyFixesDetailed(t.Context(), path, findings, "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Sorted descending by StartByte: D(10-14), C(8-12), B(2-6), A(0-4)
	// D applied first, C overlaps with D (EndByte 12 > 10), dropped
	// B does not overlap with D (EndByte 6 < 10), applied
	// A overlaps with B (EndByte 4 > 2), dropped
	if len(res.DroppedFixes) != 2 {
		t.Fatalf("expected 2 dropped fixes, got %d", len(res.DroppedFixes))
	}
}

func TestDeduplicateByteFixesReverse_SingleFix(t *testing.T) {
	fixes := []scanner.Finding{
		findingWithRule("f.kt", "rule-A", &scanner.Fix{
			StartByte: 0, EndByte: 5, ByteMode: true,
		}),
	}
	kept, dropped := deduplicateFixesReverse(fixes, findingByteEnd, findingByteStart, findingDropped)
	if len(kept) != 1 {
		t.Fatalf("expected 1 kept, got %d", len(kept))
	}
	if len(dropped) != 0 {
		t.Fatalf("expected 0 dropped, got %d", len(dropped))
	}
}

// TestApplyFixes_AtomicWrite_OriginalPreservedOnFailure simulates a "kill
// mid-write" scenario by making the parent directory read-only so the atomic
// tempfile create fails. The fixer must surface the error AND leave the
// original file content untouched — no truncation, no leaked tempfile.
//
// Regression guard for the fsutil.WriteFileAtomic switch: a plain os.WriteFile
// would have either truncated the file before the write failed or, on a
// permission failure, returned without ever touching it. The atomic path is
// stronger — it guarantees the original is preserved on ANY write failure,
// not just the ones that happen before truncation.
func TestApplyFixes_AtomicWrite_OriginalPreservedOnFailure(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory permissions; cannot simulate write failure")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "test.kt")
	original := "val x = 1\nval y = 2\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatalf("write original: %v", err)
	}

	// Make the parent directory read+execute only so os.CreateTemp inside
	// WriteFileAtomic fails. t.Cleanup restores perms so TempDir cleanup works.
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatalf("chmod dir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })

	findings := []scanner.Finding{
		finding(path, &scanner.Fix{
			StartByte:   0,
			EndByte:     9,
			Replacement: "val x = 42",
			ByteMode:    true,
		}),
	}

	_, err := ApplyFixes(t.Context(), path, findings, "")
	if err == nil {
		t.Fatal("expected write failure, got nil")
	}

	got := readFile(t, path)
	if got != original {
		t.Fatalf("original file modified after failed write: got %q, want %q", got, original)
	}

	matches, err := filepath.Glob(filepath.Join(dir, ".test.kt.tmp-*"))
	if err != nil {
		t.Fatalf("Glob: %v", err)
	}
	if len(matches) > 0 {
		t.Errorf("tempfile leaked after failed atomic write: %v", matches)
	}
}

// TestApplyFixes_AtomicWrite_PreservesFileMode locks in that the atomic write
// path preserves the existing file's permission bits. os.WriteFile preserved
// perms implicitly when overwriting; WriteFileAtomic creates a fresh inode via
// tempfile+rename, so the fixer must stat and forward the existing perms.
func TestApplyFixes_AtomicWrite_PreservesFileMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.kt")
	if err := os.WriteFile(path, []byte("val x = 1\n"), 0o600); err != nil {
		t.Fatalf("write original: %v", err)
	}

	findings := []scanner.Finding{
		finding(path, &scanner.Fix{
			StartByte:   4,
			EndByte:     9,
			Replacement: "y = 2",
			ByteMode:    true,
		}),
	}
	if _, err := ApplyFixes(t.Context(), path, findings, ""); err != nil {
		t.Fatalf("ApplyFixes: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("perms not preserved: got %o, want 0600", got)
	}
}

func TestDeduplicateLineFixesReverse_SingleFix(t *testing.T) {
	fixes := []scanner.Finding{
		findingWithRule("f.kt", "rule-A", &scanner.Fix{
			StartLine: 1, EndLine: 2,
		}),
	}
	kept, dropped := deduplicateFixesReverse(fixes, findingLineEnd, findingLineStart, findingDropped)
	if len(kept) != 1 {
		t.Fatalf("expected 1 kept, got %d", len(kept))
	}
	if len(dropped) != 0 {
		t.Fatalf("expected 0 dropped, got %d", len(dropped))
	}
}
