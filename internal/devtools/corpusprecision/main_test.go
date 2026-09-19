package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestNormalizeFindings(t *testing.T) {
	root := t.TempDir()
	corpusRoot := filepath.Join(root, "playground", "synthetic")
	sourcePath := filepath.Join(corpusRoot, "src", "Example.kt")
	if err := os.MkdirAll(filepath.Dir(sourcePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(sourcePath, []byte("package synthetic\n  val answer = 42  \n"), 0o644); err != nil {
		t.Fatal(err)
	}

	raw := []rawFinding{
		{File: "playground/synthetic/src/Example.kt", Line: 2, Column: 7, RuleSet: "style", Rule: "MagicNumber", Severity: "warning"},
		{File: filepath.Join(corpusRoot, "missing.kt"), Line: 9, Column: 1, RuleSet: "style", Rule: "MissingLine", Severity: "info"},
	}
	got, err := normalizeFindings(root, corpusRoot, raw)
	if err != nil {
		t.Fatal(err)
	}

	hash := sha256.Sum256([]byte("val answer = 42"))
	want := []normalizedFinding{
		{Rule: "MissingLine", RuleSet: "style", RelPath: "missing.kt", Line: 9, Col: 1, Severity: "info", LineHash: "unreadable"},
		{Rule: "MagicNumber", RuleSet: "style", RelPath: "src/Example.kt", Line: 2, Col: 7, Severity: "warning", LineHash: fmt.Sprintf("%x", hash[:])[:12]},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("normalizeFindings() = %#v, want %#v", got, want)
	}
}

func TestDiffFindings(t *testing.T) {
	unchanged := normalizedFinding{Rule: "RuleA", RelPath: "A.kt", Line: 2, Col: 1, Severity: "warning", LineHash: "aaaaaaaaaaaa"}
	removed := normalizedFinding{Rule: "RuleB", RelPath: "B.kt", Line: 4, Col: 2, Severity: "warning", LineHash: "bbbbbbbbbbbb"}
	added := normalizedFinding{Rule: "RuleB", RelPath: "C.kt", Line: 8, Col: 3, Severity: "error", LineHash: "cccccccccccc"}

	got := diffFindings([]normalizedFinding{unchanged, removed}, []normalizedFinding{unchanged, added})
	want := map[string]findingDiff{
		"RuleB": {Added: []normalizedFinding{added}, Removed: []normalizedFinding{removed}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("diffFindings() = %#v, want %#v", got, want)
	}
}

func TestComputePrecision(t *testing.T) {
	findings := []normalizedFinding{
		{Rule: "RuleA", RelPath: "A.kt", LineHash: "aaaaaaaaaaaa"},
		{Rule: "RuleA", RelPath: "B.kt", LineHash: "bbbbbbbbbbbb"},
		{Rule: "RuleB", RelPath: "C.kt", LineHash: "cccccccccccc"},
		{Rule: "RuleB", RelPath: "D.kt", LineHash: "dddddddddddd"},
	}
	labels := []label{
		{Rule: "RuleA", RelPath: "A.kt", LineHash: "aaaaaaaaaaaa", Verdict: "tp"},
		{Rule: "RuleA", RelPath: "B.kt", LineHash: "bbbbbbbbbbbb", Verdict: "fp"},
		{Rule: "RuleB", RelPath: "C.kt", LineHash: "cccccccccccc", Verdict: "unknown"},
	}

	got, overall := computePrecision(findings, labels)
	want := []precisionCount{
		{Rule: "RuleA", TP: 1, FP: 1},
		{Rule: "RuleB", Unlabeled: 2},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("computePrecision() = %#v, want %#v", got, want)
	}
	if wantOverall := (precisionCount{Rule: "overall", TP: 1, FP: 1, Unlabeled: 2}); overall != wantOverall {
		t.Fatalf("overall = %#v, want %#v", overall, wantOverall)
	}
	if got := precisionValue(got[0]); got != "0.500" {
		t.Fatalf("precisionValue(RuleA) = %q, want 0.500", got)
	}
	if got := precisionValue(got[1]); got != "n/a" {
		t.Fatalf("precisionValue(RuleB) = %q, want n/a", got)
	}
}

func TestLoadLabelsRejectsInvalidVerdict(t *testing.T) {
	path := filepath.Join(t.TempDir(), "labels.json")
	if err := os.WriteFile(path, []byte(`[{"rule":"RuleA","relPath":"A.kt","lineHash":"aaaaaaaaaaaa","verdict":"maybe"}]`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := loadLabels(path); err == nil {
		t.Fatal("loadLabels() succeeded with an invalid verdict")
	}
}
