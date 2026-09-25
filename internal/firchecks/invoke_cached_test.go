package firchecks

import (
	"os"
	"testing"

	_ "github.com/kaeawc/krit/internal/rules"
	"github.com/kaeawc/krit/internal/scanner"
)

func TestFakeFirChecker_RecordsCall(t *testing.T) {
	fake := NewFakeFirChecker()
	fake.Findings = []scanner.Finding{
		{File: "/src/A.kt", Line: 5, Col: 1, Rule: "CollectInOnCreateWithoutLifecycle", Severity: "warning", Message: "use repeatOnLifecycle"},
	}

	res, err := fake.Check([]string{"/src/A.kt"}, nil, nil, nil, nil)
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
	res, err := InvokeCached("krit-fir.jar", nil, nil, nil, nil, nil, t.TempDir(), false, false)
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
		V:                  FirCacheVersion,
		ContentHash:        hash,
		FilePath:           ktFile,
		ClosureFingerprint: FirInvocationFingerprint(nil, "", nil, nil),
		Findings: []FirFinding{
			{Path: ktFile, Line: 1, Col: 14, Rule: "CollectInOnCreateWithoutLifecycle", Severity: "warning", Message: "use repeatOnLifecycle", Confidence: 1.0},
		},
	}
	if err := WriteCacheEntry(cacheDir, entry); err != nil {
		t.Fatal(err)
	}

	// InvokeCached should serve from cache — no jar path needed.
	res, err := InvokeCached("", []string{ktFile}, nil, nil, nil, nil, tmp, false, false)
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
	fir := FirFinding{Path: "/src/A.kt", Line: 5, Col: 2, StartByte: 12, EndByte: 23, Rule: "InjectDispatcher", Severity: "warning", Message: "msg", Confidence: 0.9}
	f := ToScannerFinding(fir)
	if f.Rule != "InjectDispatcher" {
		t.Errorf("expected mapped Rule=InjectDispatcher, got %q", f.Rule)
	}
	if f.RuleSet != "coroutines" {
		t.Errorf("expected mapped RuleSet=coroutines, got %q", f.RuleSet)
	}
	if f.StartByte != 12 || f.EndByte != 23 {
		t.Errorf("expected byte range 12..23, got %d..%d", f.StartByte, f.EndByte)
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

func TestMergeFindings_ByteRangesAvoidPilotLineDedupe(t *testing.T) {
	goFinding := scanner.Finding{File: "/src/A.kt", Line: 10, Col: 5, StartByte: 100, EndByte: 110, Rule: "CollectInOnCreateWithoutLifecycle", RuleSet: "coroutines", Message: "go message"}
	firFinding := scanner.Finding{File: "/src/A.kt", Line: 10, Col: 12, StartByte: 130, EndByte: 140, Rule: "CollectInOnCreateWithoutLifecycle", RuleSet: "coroutines", Message: "fir message"}
	merged := MergeFindings([]scanner.Finding{goFinding}, []scanner.Finding{firFinding})
	if len(merged) != 2 {
		t.Fatalf("expected byte-distinct findings to survive line dedup, got %d", len(merged))
	}
}

func TestMergeFindings_PilotLineDedupeWhenExistingHasNoByteRange(t *testing.T) {
	goFinding := scanner.Finding{File: "/src/A.kt", Line: 10, Col: 5, Rule: "CollectInOnCreateWithoutLifecycle", RuleSet: "coroutines", Message: "go message"}
	firFinding := scanner.Finding{File: "/src/A.kt", Line: 10, Col: 12, StartByte: 130, EndByte: 140, Rule: "CollectInOnCreateWithoutLifecycle", RuleSet: "coroutines", Message: "fir message"}
	merged := MergeFindings([]scanner.Finding{goFinding}, []scanner.Finding{firFinding})
	if len(merged) != 1 {
		t.Fatalf("expected old line dedupe to remain when existing finding has no byte range, got %d", len(merged))
	}
}

// Line-level dedupe applies to any FIR rule whose ID is a registered Go rule
// (it has a tree-sitter twin), with no per-rule table to keep in sync.
func TestMergeFindings_LineDedupeIsGenericForRegisteredRules(t *testing.T) {
	const twin = "MagicNumber"
	if catalogRule(twin) == nil {
		t.Fatalf("test precondition: %s must be a registered Go rule", twin)
	}
	goFinding := scanner.Finding{File: "/src/A.kt", Line: 10, Col: 5, Rule: twin, Message: "go message"}
	firFinding := scanner.Finding{File: "/src/A.kt", Line: 10, Col: 12, Rule: twin, Message: "fir message"}
	merged := MergeFindings([]scanner.Finding{goFinding}, []scanner.Finding{firFinding})
	if len(merged) != 1 || merged[0].Message != "go message" {
		t.Fatalf("expected the registered rule's same-line FIR finding to collapse into the Go one, got %+v", merged)
	}
}

func TestMergeFindings_UnregisteredFirRuleGetsOnlyExactDedupe(t *testing.T) {
	const firOnly = "FirOnlyRuleWithoutGoTwin"
	if catalogRule(firOnly) != nil {
		t.Fatalf("test precondition: %s must not be a registered Go rule", firOnly)
	}
	existing := []scanner.Finding{{File: "/src/A.kt", Line: 10, Col: 5, Rule: firOnly}}
	fir := []scanner.Finding{
		{File: "/src/A.kt", Line: 10, Col: 12, Rule: firOnly}, // same line, different col: kept
		{File: "/src/A.kt", Line: 10, Col: 5, Rule: firOnly},  // exact duplicate: dropped
	}
	merged := MergeFindings(existing, fir)
	if len(merged) != 2 {
		t.Fatalf("expected only the exact duplicate to be dropped, got %+v", merged)
	}
}

func TestToScannerFindingWithRange_DerivesByteRange(t *testing.T) {
	tmp := t.TempDir()
	ktFile := tmp + "/A.kt"
	if err := os.WriteFile(ktFile, []byte("fun main() {\n    Dispatchers.IO\n}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	fir := FirFinding{Path: ktFile, Line: 2, Col: 5, Rule: "InjectDispatcher", Severity: "warning", Message: "msg"}
	f := toScannerFindingWithRange(fir, map[string][]byte{})
	if f.StartByte != 17 || f.EndByte != 31 {
		t.Fatalf("expected Dispatchers.IO byte range 17..31, got %d..%d", f.StartByte, f.EndByte)
	}
}

func TestToScannerFinding_EmptySeverityDefaultsToWarning(t *testing.T) {
	fir := FirFinding{Severity: ""}
	f := ToScannerFinding(fir)
	if f.Severity != "warning" {
		t.Errorf("expected severity=warning for empty input, got %q", f.Severity)
	}
}
