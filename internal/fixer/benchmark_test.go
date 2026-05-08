package fixer

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

func writeBenchFile(b *testing.B, content string) string {
	b.Helper()
	path := filepath.Join(b.TempDir(), "bench.kt")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		b.Fatalf("failed to write bench file: %v", err)
	}
	return path
}

func benchFinding(file string, fix *scanner.Fix) scanner.Finding {
	return scanner.Finding{
		File:     file,
		Line:     1,
		RuleSet:  "bench",
		Rule:     "bench-rule",
		Severity: "warning",
		Message:  "bench finding",
		Fix:      fix,
	}
}

func BenchmarkApplyFixes_ByteMode_Single(b *testing.B) {
	content := "val x = 1\nval y = 2\nval z = 3\n"
	for i := 0; i < b.N; i++ {
		path := writeBenchFile(b, content)
		findings := []scanner.Finding{
			benchFinding(path, &scanner.Fix{
				StartByte:   4,
				EndByte:     9,
				Replacement: "result = 42",
				ByteMode:    true,
			}),
		}
		_, err := ApplyFixes(path, findings, ".fixed")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkApplyFixes_ByteMode_Multiple(b *testing.B) {
	content := "val a = 1\nval b = 2\nval c = 3\nval d = 4\nval e = 5\n"
	for i := 0; i < b.N; i++ {
		path := writeBenchFile(b, content)
		findings := []scanner.Finding{
			benchFinding(path, &scanner.Fix{StartByte: 4, EndByte: 9, Replacement: "x = 10", ByteMode: true}),
			benchFinding(path, &scanner.Fix{StartByte: 14, EndByte: 19, Replacement: "y = 20", ByteMode: true}),
			benchFinding(path, &scanner.Fix{StartByte: 24, EndByte: 29, Replacement: "z = 30", ByteMode: true}),
			benchFinding(path, &scanner.Fix{StartByte: 34, EndByte: 39, Replacement: "w = 40", ByteMode: true}),
			benchFinding(path, &scanner.Fix{StartByte: 44, EndByte: 49, Replacement: "v = 50", ByteMode: true}),
		}
		_, err := ApplyFixes(path, findings, ".fixed")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkApplyFixes_LineMode_Single(b *testing.B) {
	content := "val x = 1\nval y = 2\nval z = 3\n"
	for i := 0; i < b.N; i++ {
		path := writeBenchFile(b, content)
		findings := []scanner.Finding{
			benchFinding(path, &scanner.Fix{
				StartLine:   1,
				EndLine:     1,
				Replacement: "val x = 42",
			}),
		}
		_, err := ApplyFixes(path, findings, ".fixed")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkApplyFixes_LineMode_Multiple(b *testing.B) {
	content := "val a = 1\nval b = 2\nval c = 3\nval d = 4\nval e = 5\n"
	for i := 0; i < b.N; i++ {
		path := writeBenchFile(b, content)
		findings := []scanner.Finding{
			benchFinding(path, &scanner.Fix{StartLine: 1, EndLine: 1, Replacement: "val a = 10"}),
			benchFinding(path, &scanner.Fix{StartLine: 2, EndLine: 2, Replacement: "val b = 20"}),
			benchFinding(path, &scanner.Fix{StartLine: 3, EndLine: 3, Replacement: "val c = 30"}),
			benchFinding(path, &scanner.Fix{StartLine: 4, EndLine: 4, Replacement: "val d = 40"}),
			benchFinding(path, &scanner.Fix{StartLine: 5, EndLine: 5, Replacement: "val e = 50"}),
		}
		_, err := ApplyFixes(path, findings, ".fixed")
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkValidateFixResult(b *testing.B) {
	original := []byte("fun main() {\n    println(\"hello\")\n}\n")
	fixed := []byte("fun main() {\n    println(\"world\")\n}\n")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ValidateFixResult("test.kt", original, fixed)
	}
}

func BenchmarkSplitByMode(b *testing.B) {
	rows := make([]textFixRow, 100)
	for i := range rows {
		if i%2 == 0 {
			rows[i] = textFixRow{fix: scanner.Fix{ByteMode: true, StartByte: i, EndByte: i + 1, Replacement: "x"}}
		} else {
			rows[i] = textFixRow{fix: scanner.Fix{StartLine: i, EndLine: i, Replacement: "x"}}
		}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		splitTextFixRowsByMode(rows)
	}
}
