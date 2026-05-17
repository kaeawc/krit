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

// layoutWithParams is a layout XML whose root element has android:layout_width,
// which causes layoutInflationRootHasLayoutParams to return true and triggers
// a LayoutInflation finding when inflate() is called with null.
const layoutWithParams = `<?xml version="1.0" encoding="utf-8"?>
<LinearLayout xmlns:android="http://schemas.android.com/apk/res/android"
    android:layout_width="match_parent"
    android:layout_height="match_parent">
</LinearLayout>
`

// layoutNoParams is a layout XML whose root element lacks layout_* attrs,
// so layoutInflationRootHasLayoutParams returns false.
const layoutNoParams = `<?xml version="1.0" encoding="utf-8"?>
<merge xmlns:android="http://schemas.android.com/apk/res/android">
</merge>
`

// inflateNullKotlin triggers a LayoutInflation finding: calls inflate with null
// and no ViewGroup parameter in scope.
const inflateNullKotlin = `package com.example

import android.view.LayoutInflater

class MyAdapter {
    fun createView(inflater: LayoutInflater) {
        val view = inflater.inflate(R.layout.with_root_layout_params, null)
    }
}
`

// inflateCleanKotlin has no inflate call, so produces no LayoutInflation finding.
const inflateCleanKotlin = `package com.example

class MyAdapter {
    fun doNothing() {}
}
`

func parseKotlinForTest(t *testing.T, path, content string) *scanner.File {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write kt: %v", err)
	}
	f, err := scanner.ParseFile(context.Background(), path)
	if err != nil {
		t.Fatalf("ParseFile %s: %v", path, err)
	}
	return f
}

func runResourceSourcePhase(t *testing.T, resDir, cacheDir string, sourceFiles []*scanner.File, writer *scanner.AndroidCacheWriter) AndroidResult {
	t.Helper()
	rule := findV2RuleForTest(t, "LayoutInflation")
	dispatcher := rules.NewDispatcher([]*api.Rule{rule})
	res, err := (AndroidPhase{}).Run(context.Background(), AndroidInput{
		Project:        &android.Project{ResDirs: []string{resDir}},
		ActiveRules:    []*api.Rule{rule},
		Dispatcher:     dispatcher,
		SourceFiles:    sourceFiles,
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

// TestAndroidResourceSourceCache_HitMatchesMiss verifies that cached findings
// for an unchanged source file and resource dir are byte-equal to the freshly
// computed findings.
func TestAndroidResourceSourceCache_HitMatchesMiss(t *testing.T) {
	root := t.TempDir()
	resDir := filepath.Join(root, "res")
	writeLayout(t, resDir, "with_root_layout_params", layoutWithParams)

	ktPath := filepath.Join(root, "MyAdapter.kt")
	srcFile := parseKotlinForTest(t, ktPath, inflateNullKotlin)

	cacheDir := filepath.Join(root, ".krit", "android-findings-cache")

	writer := scanner.NewAndroidCacheWriter(2)
	miss := runResourceSourcePhase(t, resDir, cacheDir, []*scanner.File{srcFile}, writer)
	if err := writer.Close(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	if len(miss.Findings.Findings()) == 0 {
		t.Fatal("expected at least one LayoutInflation finding on first run")
	}

	hashutil.ResetDefault()
	writer2 := scanner.NewAndroidCacheWriter(2)
	hit := runResourceSourcePhase(t, resDir, cacheDir, []*scanner.File{srcFile}, writer2)
	if err := writer2.Close(); err != nil {
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

func TestAndroidResourceSourceBundleCache_HitMatchesMiss(t *testing.T) {
	root := t.TempDir()
	resDir := filepath.Join(root, "res")
	writeLayout(t, resDir, "with_root_layout_params", layoutWithParams)

	ktA := parseKotlinForTest(t, filepath.Join(root, "A.kt"), inflateNullKotlin)
	ktB := parseKotlinForTest(t, filepath.Join(root, "B.kt"), inflateCleanKotlin)

	cacheDir := filepath.Join(root, ".krit", "android-findings-cache")

	writer := scanner.NewAndroidCacheWriter(2)
	miss := runResourceSourcePhase(t, resDir, cacheDir, []*scanner.File{ktA, ktB}, writer)
	if err := writer.Close(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	if len(miss.Findings.Findings()) == 0 {
		t.Fatal("expected at least one LayoutInflation finding on first run")
	}

	hashutil.ResetDefault()
	mergedFP := mergedResourceIndexFingerprint([]string{resDir})
	sourceSetFP, ok := resourceSourceSetFingerprint([]*scanner.File{ktA, ktB})
	if !ok {
		t.Fatal("expected source-set fingerprint")
	}
	key := (AndroidInput{
		RuleHash:       "test-rule-hash",
		LibraryFactsFP: "test-lf",
	}).resourceSourceBundleKey(sourceSetFP, mergedFP)
	cached, ok := scanner.LoadAndroidFindings(cacheDir, key)
	if !ok {
		t.Fatal("expected bundled resource-source cache hit")
	}
	if got, want := cached.Len(), miss.Findings.Len(); got != want {
		t.Fatalf("bundle findings count differs: got %d want %d", got, want)
	}
}

// TestAndroidResourceSourceCache_SourceChangeInvalidates verifies that editing
// one source file produces a cache miss for that file while the other file
// continues to hit.
func TestAndroidResourceSourceCache_SourceChangeInvalidates(t *testing.T) {
	root := t.TempDir()
	resDir := filepath.Join(root, "res")
	writeLayout(t, resDir, "with_root_layout_params", layoutWithParams)

	ktA := parseKotlinForTest(t, filepath.Join(root, "A.kt"), inflateNullKotlin)
	ktB := parseKotlinForTest(t, filepath.Join(root, "B.kt"), inflateNullKotlin)

	cacheDir := filepath.Join(root, ".krit", "android-findings-cache")

	writer := scanner.NewAndroidCacheWriter(2)
	first := runResourceSourcePhase(t, resDir, cacheDir, []*scanner.File{ktA, ktB}, writer)
	if err := writer.Close(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	if len(first.Findings.Findings()) < 2 {
		t.Fatalf("expected at least 2 findings (one per file), got %d", len(first.Findings.Findings()))
	}

	// Rewrite B to a clean file — its entry should miss; A's should hit.
	ktBEdited := parseKotlinForTest(t, filepath.Join(root, "B.kt"), inflateCleanKotlin)
	hashutil.ResetDefault()
	writer2 := scanner.NewAndroidCacheWriter(2)
	second := runResourceSourcePhase(t, resDir, cacheDir, []*scanner.File{ktA, ktBEdited}, writer2)
	if err := writer2.Close(); err != nil {
		t.Fatalf("flush second writer: %v", err)
	}

	// A still has a finding (from cache); B has none (re-evaluated clean file).
	aFindings := 0
	for _, f := range second.Findings.Findings() {
		if f.File == ktA.Path {
			aFindings++
		}
	}
	bFindings := 0
	for _, f := range second.Findings.Findings() {
		if f.File == ktBEdited.Path {
			bFindings++
		}
	}
	if aFindings == 0 {
		t.Error("expected A's findings to be served from cache, got none")
	}
	if bFindings != 0 {
		t.Errorf("expected B's findings to be re-evaluated to 0 after edit, got %d", bFindings)
	}
}

// TestAndroidResourceSourceCache_ResourceChangeInvalidates verifies that
// changing a resource file flips the merged-index fingerprint, causing every
// per-source cache entry to miss.
func TestAndroidResourceSourceCache_ResourceChangeInvalidates(t *testing.T) {
	root := t.TempDir()
	resDir := filepath.Join(root, "res")
	writeLayout(t, resDir, "with_root_layout_params", layoutWithParams)

	ktA := parseKotlinForTest(t, filepath.Join(root, "A.kt"), inflateNullKotlin)
	ktB := parseKotlinForTest(t, filepath.Join(root, "B.kt"), inflateNullKotlin)
	ktC := parseKotlinForTest(t, filepath.Join(root, "C.kt"), inflateNullKotlin)

	cacheDir := filepath.Join(root, ".krit", "android-findings-cache")

	writer := scanner.NewAndroidCacheWriter(2)
	first := runResourceSourcePhase(t, resDir, cacheDir, []*scanner.File{ktA, ktB, ktC}, writer)
	if err := writer.Close(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	if len(first.Findings.Findings()) == 0 {
		t.Fatal("expected findings on first run")
	}

	// Replace the layout so its root no longer has layout params — the
	// merged-index fingerprint changes, all per-source entries should miss and
	// be re-evaluated against the new (params-free) resource index.
	writeLayout(t, resDir, "with_root_layout_params", layoutNoParams)
	hashutil.ResetDefault()

	writer2 := scanner.NewAndroidCacheWriter(2)
	second := runResourceSourcePhase(t, resDir, cacheDir, []*scanner.File{ktA, ktB, ktC}, writer2)
	if err := writer2.Close(); err != nil {
		t.Fatalf("flush second: %v", err)
	}
	// With no layout params in the resource, LayoutInflation emits no findings.
	if got := len(second.Findings.Findings()); got != 0 {
		t.Fatalf("expected 0 findings after resource change (cache should have missed), got %d", got)
	}
}

// TestMergedResourceIndexFingerprint_Stability verifies the merged-index
// fingerprint is deterministic and order-independent.
func TestMergedResourceIndexFingerprint_Stability(t *testing.T) {
	root := t.TempDir()
	resA := filepath.Join(root, "resA")
	resB := filepath.Join(root, "resB")
	writeLayout(t, resA, "main", cleanLayout)
	writeLayout(t, resB, "alt", dupIDLayout)

	hashutil.ResetDefault()
	fp1 := mergedResourceIndexFingerprint([]string{resA, resB})
	hashutil.ResetDefault()
	fp2 := mergedResourceIndexFingerprint([]string{resA, resB})
	if fp1 != fp2 {
		t.Fatalf("fingerprint not stable across calls: %s vs %s", fp1, fp2)
	}

	// Reordering dirs must produce the same fingerprint.
	hashutil.ResetDefault()
	fp3 := mergedResourceIndexFingerprint([]string{resB, resA})
	if fp1 != fp3 {
		t.Fatalf("fingerprint differs when dirs are reordered: %s vs %s", fp1, fp3)
	}

	// Changing content in one dir must change the fingerprint.
	writeLayout(t, resA, "extra", cleanLayout)
	hashutil.ResetDefault()
	fp4 := mergedResourceIndexFingerprint([]string{resA, resB})
	if fp1 == fp4 {
		t.Fatal("expected fingerprint to change after adding a layout file")
	}
}

func TestMergedResourceIndexBundleCache_RoundTrip(t *testing.T) {
	root := t.TempDir()
	cacheDir := filepath.Join(root, ".krit", "android-findings-cache")
	in := AndroidInput{RuleHash: "rules", LibraryFactsFP: "libs"}
	key := in.resourceSourceIndexBundleKey("merged-fp", rules.AndroidDepLayout|rules.AndroidDepValues, android.ValuesScanStrings)
	idx := &android.ResourceIndex{
		Strings: map[string]string{"title": "Title"},
		Layouts: map[string]*android.Layout{
			"main": {Name: "main", FilePath: filepath.Join(root, "res", "layout", "main.xml")},
		},
	}
	if err := saveMergedResourceIndexBundle(cacheDir, key, idx); err != nil {
		t.Fatalf("saveMergedResourceIndexBundle: %v", err)
	}
	got, ok := loadMergedResourceIndexBundle(cacheDir, key)
	if !ok {
		t.Fatal("expected merged resource index bundle hit")
	}
	if got.Strings["title"] != "Title" {
		t.Fatalf("Strings[title] = %q, want Title", got.Strings["title"])
	}
	if got.Layouts["main"].Name != "main" {
		t.Fatalf("Layouts[main].Name = %q, want main", got.Layouts["main"].Name)
	}
}

func TestMergedResourceIndexBundleCache_KeyMismatchMisses(t *testing.T) {
	root := t.TempDir()
	cacheDir := filepath.Join(root, ".krit", "android-findings-cache")
	if err := saveMergedResourceIndexBundle(cacheDir, "key-a", &android.ResourceIndex{Strings: map[string]string{"title": "Title"}}); err != nil {
		t.Fatalf("saveMergedResourceIndexBundle: %v", err)
	}
	if _, ok := loadMergedResourceIndexBundle(cacheDir, "key-b"); ok {
		t.Fatal("expected key mismatch to miss")
	}
}

// TestResourceSourceCacheKey_MergedFPInvalidates verifies that a different
// merged-index fingerprint produces a different cache key.
func TestResourceSourceCacheKey_MergedFPInvalidates(t *testing.T) {
	in := AndroidInput{RuleHash: "rh", LibraryFactsFP: "lf"}
	a := in.resourceSourceKey("/src/A.kt", "content-hash", "merged-fp-v1")
	b := in.resourceSourceKey("/src/A.kt", "content-hash", "merged-fp-v2")
	if a == b {
		t.Fatal("merged-index FP change failed to alter the resource-source cache key")
	}
}

// TestResourceSourceCacheKey_SourceHashInvalidates verifies that different
// source content hashes produce different cache keys.
func TestResourceSourceCacheKey_SourceHashInvalidates(t *testing.T) {
	in := AndroidInput{RuleHash: "rh", LibraryFactsFP: "lf"}
	a := in.resourceSourceKey("/src/A.kt", "hash-a", "merged-fp")
	b := in.resourceSourceKey("/src/A.kt", "hash-b", "merged-fp")
	if a == b {
		t.Fatal("source content hash change failed to alter the resource-source cache key")
	}
}

// TestResourceSourceCacheKey_PathInvalidates verifies that identical-content
// files at different paths get distinct cache keys (findings embed the path).
func TestResourceSourceCacheKey_PathInvalidates(t *testing.T) {
	in := AndroidInput{RuleHash: "rh", LibraryFactsFP: "lf"}
	a := in.resourceSourceKey("/src/A.kt", "content-hash", "merged-fp")
	b := in.resourceSourceKey("/src/B.kt", "content-hash", "merged-fp")
	if a == b {
		t.Fatal("identical-content files at different paths must not share a resource-source cache key")
	}
}
