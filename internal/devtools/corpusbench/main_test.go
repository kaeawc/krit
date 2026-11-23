package main

import (
	"strings"
	"testing"
)

func TestScanStderrCountsPanicsAndDedupesWarnings(t *testing.T) {
	var res Result
	scanStderr(strings.Join([]string{
		"verbose: Parsed 10 files",
		"warning: Java semantic facts unavailable",
		"krit: 3 file(s) failed to parse",
		"krit: 4 file(s) failed to parse",
		"panic: runtime error: index out of range",
		"goroutine 1 [running]:",
	}, "\n"), &res)
	if res.Panics != 2 {
		t.Errorf("Panics = %d, want 2", res.Panics)
	}
	want := []string{"warning: Java semantic facts unavailable", "krit: 3 file(s) failed to parse"}
	if strings.Join(res.Warnings, "|") != strings.Join(want, "|") {
		t.Errorf("Warnings = %q, want %q", res.Warnings, want)
	}
}

func TestCountSource(t *testing.T) {
	var res Result
	for _, name := range []string{"build.gradle.kts", "build.gradle", "A.java", "BUILD.bazel", "MODULE.bazel", "README.md"} {
		if countSource(name, &res) {
			t.Errorf("countSource(%q) reported Kotlin", name)
		}
	}
	if !countSource("Main.kt", &res) {
		t.Error("countSource(Main.kt) = false")
	}
	if res.GradleKts != 1 || res.GradleGrv != 1 || res.JavaFiles != 1 || res.BazelFiles != 2 {
		t.Errorf("counts = %+v", res)
	}
}

func TestRenderSummaryComparesRuns(t *testing.T) {
	prev := report{Results: []Result{{Name: "a", Status: "timeout", FindingsByRul: map[string]int{"X": 2}, Findings: 2}}}
	cur := report{Results: []Result{{Name: "a", Status: "ok", KtFiles: 10, ModuleCoverage: 1, FindingsByRul: map[string]int{"Y": 1}, Findings: 1}}}
	out := renderSummary(cur, &prev)
	for _, want := range []string{
		"| repos completed | 1/1 | 0/1 |",
		"| timeouts | 0 | 1 |",
		"| repos with ≥90% files in a module | 1/1 | 0/1 |",
		"| Y | 1 | 1 | 0 |",
		"| X | 0 | 0 | 2 |",
		"| -1 |",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("summary missing %q:\n%s", want, out)
		}
	}
}
