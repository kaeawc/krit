package fixer

import (
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

	n, err := ApplyFixes(path, findings, "")
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

	n, err := ApplyFixes(path, findings, "")
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

	n, err := ApplyFixes(path, findings, "")
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

	n, err := ApplyFixes(path, findings, "")
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

	n, err := ApplyFixes(path, findings, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Both findings are counted even though one is deduplicated
	if n != 2 {
		t.Fatalf("expected 2 fixes counted, got %d", n)
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

	n, err := ApplyFixes(path, findings, "")
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

	n, err := ApplyFixes(path, findings, "")
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

	n, err := ApplyFixes(path, findings, "")
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

// --------------- Edge cases ---------------

func TestNoFixesApplied(t *testing.T) {
	content := "unchanged content"
	path := writeTestFile(t, content)

	// Finding without a fix
	findings := []scanner.Finding{
		{File: path, Line: 1, Rule: "test", Message: "no fix"},
	}

	n, err := ApplyFixes(path, findings, "")
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

	n, err := ApplyFixes(path, findings, "")
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

	n, err := ApplyFixes(path, findings, ".new")
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

	totalFixes, filesModified, errs := ApplyAllFixes(findings, "")
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
	wantFixes, wantModified, wantErrs := ApplyAllFixes(sliceFindings, "")
	if len(wantErrs) > 0 {
		t.Fatalf("slice ApplyAllFixes errors: %v", wantErrs)
	}

	_, columns, gotFiles := makeFixture(t)
	gotFixes, gotModified, gotErrs := ApplyAllFixesColumns(&columns, "")
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
	want, err := ApplyFixesDetailed(path, makeFindings(path), "", false)
	if err != nil {
		t.Fatalf("slice ApplyFixesDetailed error: %v", err)
	}

	path = writeTestFile(t, "abcdefghij")
	columns := scanner.CollectFindings(makeFindings(path))
	got, err := applyFixesDetailedColumns(path, &columns, []int{0, 1}, "", false)
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

	n, err := ApplyFixes(path, findings, "")
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

	n, err := ApplyFixes(path, findings, "")
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

	n, err := ApplyFixes(path, findings, "")
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

	n, err := ApplyFixes(path, findings, "")
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

	_, err := ApplyFixes(path, findings, "")
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
	n, err := ApplyFixes(path, findings, "")
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
	n, err := ApplyFixes(path, findings, "")
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
	err := ValidateFixResult("test.kt", original, fixed)
	if err != nil {
		t.Fatalf("expected no error for valid Kotlin, got: %v", err)
	}
}

func TestValidateFixResult_InvalidKotlin(t *testing.T) {
	original := []byte("fun main() { println(\"hello\") }")
	fixed := []byte("fun main() { println(\"hello\"")
	err := ValidateFixResult("test.kt", original, fixed)
	if err == nil {
		t.Fatal("expected error for broken Kotlin syntax")
	}
	if !strings.Contains(err.Error(), "parse errors") {
		t.Fatalf("expected parse errors message, got: %v", err)
	}
}

func TestValidateFixResult_NonKotlinFile(t *testing.T) {
	// Non-Kotlin files are skipped
	err := ValidateFixResult("test.java", []byte("broken{{{"), []byte("also broken{{{"))
	if err != nil {
		t.Fatalf("expected no error for non-Kotlin file, got: %v", err)
	}
}

func TestValidateFixResult_KtsFile(t *testing.T) {
	original := []byte("val x = 1")
	fixed := []byte("val x = {{{")
	err := ValidateFixResult("build.gradle.kts", original, fixed)
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

	_, err := ApplyFixesWithValidation(path, findings, "", true)
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

	n, err := ApplyFixesWithValidation(path, findings, "", true)
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

	res, err := ApplyFixesDetailed(path, findings, "", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Applied != 2 {
		t.Fatalf("expected 2 fixes counted, got %d", res.Applied)
	}
	if len(res.DroppedFixes) != 1 {
		t.Fatalf("expected 1 dropped fix, got %d", len(res.DroppedFixes))
	}
	if res.DroppedFixes[0].Rule != "rule-A" {
		t.Fatalf("expected dropped rule 'rule-A', got %q", res.DroppedFixes[0].Rule)
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

	res, err := ApplyFixesDetailed(path, findings, "", false)
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

	res, err := ApplyFixesDetailed(path, findings, "", false)
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

	res, err := ApplyFixesDetailed(path, findings, "", false)
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

	res, err := ApplyFixesDetailed(path, findings, "", false)
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
