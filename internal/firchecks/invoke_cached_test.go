package firchecks

import (
	"os"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

func TestFakeFirChecker_RecordsCall(t *testing.T) {
	fake := NewFakeFirChecker()
	fake.Findings = []scanner.Finding{
		{File: "/src/A.kt", Line: 5, Col: 1, Rule: "FLOW_COLLECT_IN_ON_CREATE", Severity: "warning", Message: "use repeatOnLifecycle"},
	}

	res, err := fake.Check([]string{"/src/A.kt"}, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(res.Findings))
	}
	if len(fake.Called) != 1 {
		t.Fatalf("expected 1 recorded call, got %d", len(fake.Called))
	}
}

func TestMergeFindings_DeduplicatesOnCollision(t *testing.T) {
	existing := []scanner.Finding{
		{File: "/src/A.kt", Line: 10, Col: 1, Rule: "SomeGoRule", RuleSet: "kotlin"},
		{File: "/src/A.kt", Line: 20, Col: 5, Rule: "FIR_RULE", RuleSet: "fir"},
	}
	fir := []scanner.Finding{
		{File: "/src/A.kt", Line: 20, Col: 5, Rule: "FIR_RULE", RuleSet: "fir"}, // duplicate
		{File: "/src/A.kt", Line: 30, Col: 1, Rule: "NEW_FIR_RULE", RuleSet: "fir"},
	}
	merged := MergeFindings(existing, fir)
	if len(merged) != 3 {
		t.Errorf("expected 3 findings after dedup, got %d", len(merged))
	}
}

func TestMergeFindings_GoWinsOnCollision(t *testing.T) {
	goFinding := scanner.Finding{File: "/src/A.kt", Line: 10, Col: 1, Rule: "BOTH", RuleSet: "kotlin", Message: "go message"}
	firFinding := scanner.Finding{File: "/src/A.kt", Line: 10, Col: 1, Rule: "BOTH", RuleSet: "fir", Message: "fir message"}
	merged := MergeFindings([]scanner.Finding{goFinding}, []scanner.Finding{firFinding})
	if len(merged) != 1 {
		t.Fatalf("expected 1 finding (dedup), got %d", len(merged))
	}
	if merged[0].Message != "go message" {
		t.Errorf("expected Go finding to win, got message: %q", merged[0].Message)
	}
}

func TestInvokeCached_EmptyFilesReturnsEmpty(t *testing.T) {
	res, err := InvokeCached("krit-fir.jar", nil, nil, nil, nil, t.TempDir(), false, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Findings) != 0 {
		t.Errorf("expected 0 findings for empty input")
	}
}

func TestInvokeCached_AllCacheHits(t *testing.T) {
	tmp := t.TempDir()

	// Write a .kt file with known content.
	ktFile := tmp + "/Cached.kt"
	content := []byte("fun cached() { flow.collect {} }")
	if err := os.WriteFile(ktFile, content, 0644); err != nil {
		t.Fatal(err)
	}
	hash, err := ContentHash(ktFile)
	if err != nil {
		t.Fatal(err)
	}

	cacheDir, _ := CacheDir(tmp)
	entry := &FirCacheEntry{
		V:           FirCacheVersion,
		ContentHash: hash,
		FilePath:    ktFile,
		Findings: []FirFinding{
			{Path: ktFile, Line: 1, Col: 14, Rule: "FLOW_COLLECT_IN_ON_CREATE", Severity: "warning", Message: "use repeatOnLifecycle", Confidence: 1.0},
		},
	}
	if err := WriteCacheEntry(cacheDir, entry); err != nil {
		t.Fatal(err)
	}

	// InvokeCached should serve from cache — no jar path needed.
	res, err := InvokeCached("", []string{ktFile}, nil, nil, nil, tmp, false, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Findings) != 1 {
		t.Fatalf("expected 1 finding from cache, got %d", len(res.Findings))
	}
	if res.Findings[0].Rule != "CollectInOnCreateWithoutLifecycle" {
		t.Errorf("unexpected rule: %q", res.Findings[0].Rule)
	}
}

func TestToScannerFinding_SetsRuleSetFirForUnknownDiagnostic(t *testing.T) {
	fir := FirFinding{Path: "/src/A.kt", Line: 5, Col: 2, Rule: "SOME_RULE", Severity: "warning", Message: "msg", Confidence: 0.9}
	f := ToScannerFinding(fir)
	if f.RuleSet != "fir" {
		t.Errorf("expected RuleSet=fir, got %q", f.RuleSet)
	}
	if f.Confidence != 0.9 {
		t.Errorf("expected confidence 0.9, got %f", f.Confidence)
	}
}

func TestToScannerFinding_MapsKnownDiagnosticToCatalogRule(t *testing.T) {
	fir := FirFinding{Path: "/src/A.kt", Line: 5, Col: 2, Rule: "INJECT_DISPATCHER", Severity: "warning", Message: "msg", Confidence: 0.9}
	f := ToScannerFinding(fir)
	if f.Rule != "InjectDispatcher" {
		t.Errorf("expected mapped Rule=InjectDispatcher, got %q", f.Rule)
	}
	if f.RuleSet != "coroutines" {
		t.Errorf("expected mapped RuleSet=coroutines, got %q", f.RuleSet)
	}
}

func TestMergeFindings_DeduplicatesPilotRuleOnSameLine(t *testing.T) {
	goFinding := scanner.Finding{File: "/src/A.kt", Line: 10, Col: 5, Rule: "CollectInOnCreateWithoutLifecycle", RuleSet: "coroutines", Message: "go message"}
	firFinding := scanner.Finding{File: "/src/A.kt", Line: 10, Col: 12, Rule: "CollectInOnCreateWithoutLifecycle", RuleSet: "coroutines", Message: "fir message"}
	merged := MergeFindings([]scanner.Finding{goFinding}, []scanner.Finding{firFinding})
	if len(merged) != 1 {
		t.Fatalf("expected 1 finding after line-level pilot dedup, got %d", len(merged))
	}
	if merged[0].Message != "go message" {
		t.Errorf("expected Go finding to win, got message: %q", merged[0].Message)
	}
}

func TestToScannerFinding_EmptySeverityDefaultsToWarning(t *testing.T) {
	fir := FirFinding{Severity: ""}
	f := ToScannerFinding(fir)
	if f.Severity != "warning" {
		t.Errorf("expected severity=warning for empty input, got %q", f.Severity)
	}
}
