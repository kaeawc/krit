package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/hashutil"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

const dupIDLayout = `<?xml version="1.0" encoding="utf-8"?>
<LinearLayout xmlns:android="http://schemas.android.com/apk/res/android"
    android:orientation="vertical"
    android:layout_width="match_parent"
    android:layout_height="match_parent">
    <TextView android:id="@+id/title"
        android:layout_width="wrap_content"
        android:layout_height="wrap_content" />
    <TextView android:id="@+id/title"
        android:layout_width="wrap_content"
        android:layout_height="wrap_content" />
</LinearLayout>
`

const cleanLayout = `<?xml version="1.0" encoding="utf-8"?>
<LinearLayout xmlns:android="http://schemas.android.com/apk/res/android"
    android:orientation="vertical"
    android:layout_width="match_parent"
    android:layout_height="match_parent">
    <TextView android:id="@+id/title"
        android:layout_width="wrap_content"
        android:layout_height="wrap_content" />
    <TextView android:id="@+id/subtitle"
        android:layout_width="wrap_content"
        android:layout_height="wrap_content" />
</LinearLayout>
`

func writeLayout(t *testing.T, resDir, name, body string) string {
	t.Helper()
	layoutDir := filepath.Join(resDir, "layout")
	if err := os.MkdirAll(layoutDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(layoutDir, name+".xml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write layout: %v", err)
	}
	return path
}

func runResourcePhase(t *testing.T, resDir, cacheDir string, writer *scanner.AndroidCacheWriter) AndroidResult {
	t.Helper()
	rule := findV2RuleForTest(t, "DuplicateIdsResource")
	dispatcher := rules.NewDispatcher([]*api.Rule{rule}, nil)
	res, err := (AndroidPhase{}).Run(context.Background(), AndroidInput{
		Project:        &android.Project{ResDirs: []string{resDir}},
		ActiveRules:    []*api.Rule{rule},
		Dispatcher:     dispatcher,
		RuleHash:       "test-rule-hash",
		LibraryFactsFP: "test-lf",
		CacheDir:       cacheDir,
		CacheWriter:    writer,
	})
	if err != nil {
		t.Fatalf("AndroidPhase.Run: %v", err)
	}
	return res
}

// TestAndroidResourceCache_HitMatchesMiss ensures cached findings byte-equal
// fresh findings for an unchanged res dir.
func TestAndroidResourceCache_HitMatchesMiss(t *testing.T) {
	root := t.TempDir()
	resDir := filepath.Join(root, "res")
	writeLayout(t, resDir, "main", dupIDLayout)
	cacheDir := filepath.Join(root, ".krit", "android-findings-cache")

	writer := scanner.NewAndroidCacheWriter(2)
	miss := runResourcePhase(t, resDir, cacheDir, writer)
	if err := writer.Close(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	if len(miss.Findings.Findings()) == 0 {
		t.Fatal("expected at least one DuplicateIdsResource finding on first run")
	}

	hitWriter := scanner.NewAndroidCacheWriter(2)
	hit := runResourcePhase(t, resDir, cacheDir, hitWriter)
	if err := hitWriter.Close(); err != nil {
		t.Fatalf("flush hit writer: %v", err)
	}
	if got, want := len(hit.Findings.Findings()), len(miss.Findings.Findings()); got != want {
		t.Fatalf("hit findings count differs: got %d want %d", got, want)
	}
	for i, f := range miss.Findings.Findings() {
		if hit.Findings.Findings()[i].Rule != f.Rule {
			t.Fatalf("rule mismatch at %d: got %q want %q", i, hit.Findings.Findings()[i].Rule, f.Rule)
		}
	}
}

// TestAndroidResourceCache_FileChangeInvalidates ensures rewriting any file
// in the resDir produces a new fingerprint.
func TestAndroidResourceCache_FileChangeInvalidates(t *testing.T) {
	root := t.TempDir()
	resDir := filepath.Join(root, "res")
	layoutPath := writeLayout(t, resDir, "main", dupIDLayout)
	cacheDir := filepath.Join(root, ".krit", "android-findings-cache")

	writer := scanner.NewAndroidCacheWriter(2)
	first := runResourcePhase(t, resDir, cacheDir, writer)
	if err := writer.Close(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	if len(first.Findings.Findings()) == 0 {
		t.Fatal("expected duplicate-id finding on dirty layout")
	}

	if err := os.WriteFile(layoutPath, []byte(cleanLayout), 0o644); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	hashutil.ResetDefault()
	secondWriter := scanner.NewAndroidCacheWriter(2)
	second := runResourcePhase(t, resDir, cacheDir, secondWriter)
	if err := secondWriter.Close(); err != nil {
		t.Fatalf("flush second writer: %v", err)
	}
	if got := len(second.Findings.Findings()); got != 0 {
		t.Fatalf("expected 0 findings after fixing duplicate ids, got %d (cache served stale data?)", got)
	}
}

// TestResDirContentFingerprint_StableWhenUnchanged guards the helper
// directly: same directory content → same fingerprint.
func TestResDirContentFingerprint_StableWhenUnchanged(t *testing.T) {
	root := t.TempDir()
	resDir := filepath.Join(root, "res")
	writeLayout(t, resDir, "a", cleanLayout)
	writeLayout(t, resDir, "b", dupIDLayout)
	hashutil.ResetDefault()
	fp1 := resDirContentFingerprint(resDir)
	hashutil.ResetDefault()
	fp2 := resDirContentFingerprint(resDir)
	if fp1 != fp2 {
		t.Fatalf("fingerprint not stable across calls: %s vs %s", fp1, fp2)
	}
}

// TestResDirContentFingerprint_NewFileChangesFP ensures adding a file
// invalidates the fingerprint.
func TestResDirContentFingerprint_NewFileChangesFP(t *testing.T) {
	root := t.TempDir()
	resDir := filepath.Join(root, "res")
	writeLayout(t, resDir, "a", cleanLayout)
	hashutil.ResetDefault()
	before := resDirContentFingerprint(resDir)
	writeLayout(t, resDir, "b", cleanLayout)
	hashutil.ResetDefault()
	after := resDirContentFingerprint(resDir)
	if before == after {
		t.Fatal("expected fingerprint to change after adding a layout file")
	}
}

func TestMergedResourceIndexFingerprintWithUsesCachedDirFPs(t *testing.T) {
	missingA := filepath.Join(t.TempDir(), "missing-a")
	missingB := filepath.Join(t.TempDir(), "missing-b")
	fps := resDirFingerprints{
		values: map[string]string{
			missingA: "fp-a",
			missingB: "fp-b",
		},
	}
	first := mergedResourceIndexFingerprintWith([]string{missingB, missingA}, &fps)
	second := mergedResourceIndexFingerprintWith([]string{missingA, missingB}, &fps)
	if first != second {
		t.Fatalf("fingerprint should be order-independent: %q vs %q", first, second)
	}
	if len(fps.values) != 2 {
		t.Fatalf("cached fingerprint map grew unexpectedly: got %d entries", len(fps.values))
	}
}

func TestResDirFingerprintCache_InvalidatesOnMetadataChange(t *testing.T) {
	root := t.TempDir()
	resDir := filepath.Join(root, "res")
	layoutPath := writeLayout(t, resDir, "a", cleanLayout)

	cache := newResDirFingerprintCache(filepath.Join(root, ".krit", "android-findings-cache"))
	first := cache.fingerprint(resDir)
	if first == "" {
		t.Fatal("empty first fingerprint")
	}
	if _, err := os.Stat(cache.path); err != nil {
		t.Fatalf("expected fingerprint cache file: %v", err)
	}

	secondCache := newResDirFingerprintCache(filepath.Join(root, ".krit", "android-findings-cache"))
	second := secondCache.fingerprint(resDir)
	if second != first {
		t.Fatalf("cached fingerprint changed without file changes: %q vs %q", first, second)
	}

	if err := os.WriteFile(layoutPath, []byte(dupIDLayout), 0o644); err != nil {
		t.Fatalf("rewrite layout: %v", err)
	}
	hashutil.ResetDefault()
	thirdCache := newResDirFingerprintCache(filepath.Join(root, ".krit", "android-findings-cache"))
	third := thirdCache.fingerprint(resDir)
	if third == first {
		t.Fatal("expected fingerprint to change after resource metadata/content change")
	}
}

// TestResourceCacheKey_DepsMaskInvalidates ensures flipping the active
// resource-deps mask produces a different cache key, so a change to
// rule selection (e.g., enabling a values-aware rule) doesn't false-hit.
func TestResourceCacheKey_DepsMaskInvalidates(t *testing.T) {
	in := AndroidInput{RuleHash: "rh", LibraryFactsFP: "lf"}
	a := in.resourceKey("/res", "fp", rules.AndroidDepLayout, android.ValuesScanNone)
	b := in.resourceKey("/res", "fp", rules.AndroidDepLayout|rules.AndroidDepValues, android.ValuesScanStrings)
	if a == b {
		t.Fatal("deps/valueKinds change failed to alter the cache key")
	}
}
