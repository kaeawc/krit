package firchecks

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	_ "github.com/kaeawc/krit/internal/rules"
	"github.com/kaeawc/krit/internal/scanner"
)

func TestFakeFirChecker_RecordsCall(t *testing.T) {
	fake := NewFakeFirChecker()
	fake.Findings = []scanner.Finding{
		{File: "/src/A.kt", Line: 5, Col: 1, Rule: "CollectInOnCreateWithoutLifecycle", Severity: "warning", Message: "use repeatOnLifecycle"},
	}

	res, err := fake.Check([]string{"/src/A.kt"}, nil, nil, nil, nil, FileFacts{})
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

func TestInvokeCached_EmptyFilesReturnsEmpty(t *testing.T) {
	res, err := InvokeCached("krit-fir.jar", nil, nil, nil, nil, nil, FileFacts{}, t.TempDir(), false, false)
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
		ClosureFingerprint: CheckCacheFingerprint(nil, []string{ktFile}, nil, "", nil, nil, FileFacts{}),
		Findings: []FirFinding{
			{Path: ktFile, Line: 1, Col: 14, Rule: "CollectInOnCreateWithoutLifecycle", Severity: "warning", Message: "use repeatOnLifecycle", Confidence: 1.0},
		},
	}
	if err := WriteCacheEntry(cacheDir, entry); err != nil {
		t.Fatal(err)
	}

	// InvokeCached should serve from cache — no jar path needed.
	res, err := InvokeCached("", []string{ktFile}, nil, nil, nil, nil, FileFacts{}, tmp, false, false)
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

// A cache hit carries the gating and the advertised rules, so a warm run
// applies the same verdict as the cold run that wrote the entries.
func TestInvokeCached_HitsCarryErrorFilesAndRules(t *testing.T) {
	tmp := t.TempDir()
	clean := filepath.Join(tmp, "Clean.kt")
	broken := filepath.Join(tmp, "Broken.kt")
	for _, p := range []string{clean, broken} {
		if err := os.WriteFile(p, []byte("fun "+filepath.Base(p)[:1]+"() {}\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	files := []string{clean, broken}
	rules := []string{"InjectDispatcher", "MagicNumber"}
	fp := CheckCacheFingerprint(nil, files, nil, "", rules, nil, FileFacts{})
	cacheDir, _ := CacheDir(tmp)
	resp := &CheckResponse{
		Rules:      []string{"InjectDispatcher"},
		ErrorFiles: map[string]string{broken: "Unresolved reference 'x'."},
	}
	if n := WriteFreshEntriesForFingerprint(cacheDir, files, resp, fp); n != 2 {
		t.Fatalf("wrote %d entries, want 2", n)
	}
	res, err := InvokeCached("", files, nil, nil, rules, nil, FileFacts{}, tmp, false, false)
	if err != nil {
		t.Fatalf("expected an all-hit run, got %v", err)
	}
	if want := map[string]string{broken: "Unresolved reference 'x'."}; !reflect.DeepEqual(res.ErrorFiles, want) {
		t.Fatalf("ErrorFiles = %v, want %v", res.ErrorFiles, want)
	}
	if want := []string{"InjectDispatcher"}; !reflect.DeepEqual(res.Rules, want) {
		t.Fatalf("Rules = %v, want %v", res.Rules, want)
	}
}

// krit-fir compiles the whole module, so editing a file that is not itself
// requested (a dependency under the source dirs) must invalidate the cached
// verdict of the files that were.
func TestCheckCacheFingerprint_DependencyEditInvalidatesDependent(t *testing.T) {
	src := t.TempDir()
	dependent := filepath.Join(src, "Dependent.kt")
	dependency := filepath.Join(src, "Dependency.kt")
	if err := os.WriteFile(dependent, []byte("class Dependent : Base()\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dependency, []byte("open class Base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cacheDir, _ := CacheDir(t.TempDir())
	before := CheckCacheFingerprint([]string{src}, []string{dependent}, nil, "", []string{"R"}, nil, FileFacts{})
	WriteFreshEntriesForFingerprint(cacheDir, []string{dependent}, &CheckResponse{}, before)
	if hits, _ := ClassifyFilesForFingerprint(cacheDir, []string{dependent}, before); len(hits) != 1 {
		t.Fatalf("expected a hit before the dependency edit")
	}
	if err := os.WriteFile(dependency, []byte("open class Base { fun changed() {} }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	after := CheckCacheFingerprint([]string{src}, []string{dependent}, nil, "", []string{"R"}, nil, FileFacts{})
	if hits, misses := ClassifyFilesForFingerprint(cacheDir, []string{dependent}, after); len(hits) != 0 || len(misses) != 1 {
		t.Fatalf("dependency edit must invalidate the dependent's entry; hits=%d misses=%d", len(hits), len(misses))
	}
}
