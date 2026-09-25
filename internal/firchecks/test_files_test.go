package firchecks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// The pass classifies each requested file with scanner.IsTestFile, the same
// call (and the same configured test paths) Go rules use, and sends the test
// ones as testFiles, spelled as requested.
func TestRunPassSendsTestFileClassification(t *testing.T) {
	scanner.InitTestPaths([]string{"/fixtures-tests/"}, nil)
	t.Cleanup(func() { scanner.InitTestPaths(nil, nil) })
	p := newVerdictProject(t, map[string]string{
		"src/main/kotlin/M.kt":    "package m\n",
		"src/test/kotlin/T.kt":    "package t\n",
		"lib/fixtures-tests/C.kt": "package c\n",
	}, nil)
	checker := NewFakeFirChecker()
	RunPass(PassOptions{
		Enabled:     true,
		Checker:     checker,
		ActiveRules: []*api.Rule{{ID: verdictRule}},
		KotlinPaths: []string{"src/main/kotlin/M.kt", "src/test/kotlin/T.kt", "lib/fixtures-tests/C.kt"},
	}, nil)
	if len(checker.CalledTestFiles) != 1 {
		t.Fatalf("Check calls = %d, want 1", len(checker.CalledTestFiles))
	}
	want := []string{p.abs("lib/fixtures-tests/C.kt"), p.abs("src/test/kotlin/T.kt")}
	if got := checker.CalledTestFiles[0]; !reflect.DeepEqual(got, want) {
		t.Fatalf("testFiles = %v, want %v (src/main must not be sent)", got, want)
	}
	for _, path := range checker.CalledTestFiles[0] {
		if !slices.Contains(checker.Called[0], path) {
			t.Fatalf("test file %s is not spelled like any requested file %v", path, checker.Called[0])
		}
	}
}

// Classification uses the scan's spelling of a path, as Go rules do, not the
// absolute path: a checkout under a directory named "test" must not turn
// every file into a test file for FIR while Go still checks it.
func TestTestFilesOfClassifiesTheScanSpelling(t *testing.T) {
	requested := []string{"/work/test/app/src/main/kotlin/M.kt", "/work/test/app/src/test/kotlin/T.kt"}
	display := map[string]string{
		requested[0]: "src/main/kotlin/M.kt",
		requested[1]: "src/test/kotlin/T.kt",
	}
	if got := testFilesOf(requested, display); !reflect.DeepEqual(got, requested[1:]) {
		t.Fatalf("testFilesOf = %v, want %v", got, requested[1:])
	}
}

func TestCheckSendsTestFilesOnTheWire(t *testing.T) {
	d, requests := fakeFirDaemon(t, `{"id":1,"succeeded":2,"skipped":0,"findings":[],"rules":[],"crashed":{}}`)
	files := []fileRef{{Path: "/p/src/main/kotlin/M.kt"}, {Path: "/p/src/test/kotlin/T.kt"}}
	if _, err := d.Check(files, nil, nil, []string{"StateFlowMutableLeak"}, nil, FileFacts{TestFiles: []string{"/p/src/test/kotlin/T.kt"}}); err != nil {
		t.Fatalf("Check: %v", err)
	}
	var sent struct {
		TestFiles []string `json:"testFiles"`
	}
	raw := <-requests
	if err := json.Unmarshal(raw, &sent); err != nil {
		t.Fatalf("request is not JSON: %v\n%s", err, raw)
	}
	if !reflect.DeepEqual(sent.TestFiles, []string{"/p/src/test/kotlin/T.kt"}) {
		t.Fatalf("testFiles = %v\n%s", sent.TestFiles, raw)
	}

	d, requests = fakeFirDaemon(t, `{"id":1,"succeeded":1,"skipped":0,"findings":[],"rules":[],"crashed":{}}`)
	if _, err := d.Check(files[:1], nil, nil, []string{"StateFlowMutableLeak"}, nil, FileFacts{}); err != nil {
		t.Fatalf("Check: %v", err)
	}
	if raw := <-requests; strings.Contains(string(raw), "testFiles") {
		t.Fatalf("testFiles must be omitted when no file is a test file: %s", raw)
	}
}

func TestFileFactsForFilesKeepsOnlyRequestedFiles(t *testing.T) {
	facts := FileFacts{
		TestFiles: []string{"/a/T1.kt", "/a/T2.kt"},
		ScanPaths: map[string]string{"/a/T1.kt": "T1.kt", "/a/M.kt": "M.kt"},
	}
	got := facts.forFiles([]string{"/a/M.kt", "/a/T2.kt"})
	want := FileFacts{TestFiles: []string{"/a/T2.kt"}, ScanPaths: map[string]string{"/a/M.kt": "M.kt"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("forFiles = %+v, want %+v", got, want)
	}
	if got := (FileFacts{}).forFiles([]string{"/a/M.kt"}); !reflect.DeepEqual(got, FileFacts{}) {
		t.Fatalf("empty forFiles = %+v", got)
	}
}

// The pass sends the scan's own spelling of each requested file whose
// spelling differs from the absolute path it requests, so a checker applies
// Go's path heuristics to the same string the Go rule tests.
func TestRunPassSendsScanPaths(t *testing.T) {
	p := newVerdictProject(t, map[string]string{
		"samples/proj/src/X.kt": "package x\n",
		"src/main/kotlin/M.kt":  "package m\n",
	}, nil)
	absolute := p.abs("src/main/kotlin/M.kt")
	checker := NewFakeFirChecker()
	RunPass(PassOptions{
		Enabled:     true,
		Checker:     checker,
		ActiveRules: []*api.Rule{{ID: verdictRule}},
		KotlinPaths: []string{"samples/proj/src/X.kt", absolute},
	}, nil)
	if len(checker.CalledScanPaths) != 1 {
		t.Fatalf("Check calls = %d, want 1", len(checker.CalledScanPaths))
	}
	want := map[string]string{p.abs("samples/proj/src/X.kt"): "samples/proj/src/X.kt"}
	if got := checker.CalledScanPaths[0]; !reflect.DeepEqual(got, want) {
		t.Fatalf("scanPaths = %v, want %v (a file scanned by its absolute path needs no entry)", got, want)
	}
}

func TestCheckSendsScanPathsOnTheWire(t *testing.T) {
	d, requests := fakeFirDaemon(t, `{"id":1,"succeeded":1,"skipped":0,"findings":[],"rules":[],"crashed":{}}`)
	files := []fileRef{{Path: "/p/samples/proj/src/X.kt"}}
	facts := FileFacts{ScanPaths: map[string]string{"/p/samples/proj/src/X.kt": "samples/proj/src/X.kt"}}
	if _, err := d.Check(files, nil, nil, []string{"PrintlnInProduction"}, nil, facts); err != nil {
		t.Fatalf("Check: %v", err)
	}
	var sent struct {
		ScanPaths map[string]string `json:"scanPaths"`
	}
	raw := <-requests
	if err := json.Unmarshal(raw, &sent); err != nil {
		t.Fatalf("request is not JSON: %v\n%s", err, raw)
	}
	if !reflect.DeepEqual(sent.ScanPaths, facts.ScanPaths) {
		t.Fatalf("scanPaths = %v\n%s", sent.ScanPaths, raw)
	}
}

// Scanning the same files from another working directory changes their scan
// spelling, and so possibly a path-based verdict: it must miss the FIR cache.
func TestFirInvocationFingerprintIncludesScanPaths(t *testing.T) {
	jar := filepath.Join(t.TempDir(), "krit-fir.jar")
	rules := []string{"PrintlnInProduction"}
	none := FirInvocationFingerprint(nil, jar, rules, nil, FileFacts{})
	relative := FirInvocationFingerprint(nil, jar, rules, nil, FileFacts{ScanPaths: map[string]string{"/p/samples/X.kt": "samples/X.kt"}})
	nested := FirInvocationFingerprint(nil, jar, rules, nil, FileFacts{ScanPaths: map[string]string{"/p/samples/X.kt": "X.kt"}})
	if none == relative || relative == nested {
		t.Fatal("a different scan spelling must change the fingerprint")
	}
}

// Reclassifying a file (e.g. a testSourcePaths edit) changes checker verdicts
// without changing any source, so it must miss the FIR cache.
func TestInvokeCached_TestFileClassificationChangeMisses(t *testing.T) {
	tmp := t.TempDir()
	ktFile := filepath.Join(tmp, "Leak.kt")
	if err := os.WriteFile(ktFile, []byte("val leak = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	hash, err := ContentHash(ktFile)
	if err != nil {
		t.Fatal(err)
	}
	rules := []string{"StateFlowMutableLeak"}
	cacheDir, _ := CacheDir(tmp)
	if err := WriteCacheEntry(cacheDir, &FirCacheEntry{
		V: FirCacheVersion, ContentHash: hash, FilePath: ktFile, Rules: rules,
		ClosureFingerprint: CheckCacheFingerprint(nil, []string{ktFile}, nil, "", rules, nil, FileFacts{}),
	}); err != nil {
		t.Fatal(err)
	}

	if _, err := InvokeCached("", []string{ktFile}, nil, nil, rules, nil, FileFacts{}, tmp, false, false); err != nil {
		t.Fatalf("same classification must hit the cache: %v", err)
	}
	// A miss needs the jar, which "" cannot provide: the error proves the miss.
	_, err = InvokeCached("", []string{ktFile}, nil, nil, rules, nil, FileFacts{TestFiles: []string{ktFile}}, tmp, false, false)
	if err == nil || !strings.Contains(err.Error(), "krit-fir.jar not found") {
		t.Fatalf("reclassified file must miss the cache, got err=%v", err)
	}

	jar := filepath.Join(tmp, "krit-fir.jar")
	if FirInvocationFingerprint(nil, jar, rules, nil, FileFacts{TestFiles: []string{"/b.kt", "/a.kt"}}) !=
		FirInvocationFingerprint(nil, jar, rules, nil, FileFacts{TestFiles: []string{"/a.kt", "/b.kt", "/a.kt"}}) {
		t.Fatal("testFiles order or duplicates must not change the fingerprint")
	}
}
