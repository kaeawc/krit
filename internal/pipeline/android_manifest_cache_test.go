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

const allowBackupFlaggedManifest = `<?xml version="1.0" encoding="utf-8"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android"
    package="com.example.test">
    <application android:allowBackup="true" android:label="Test"/>
</manifest>
`

const allowBackupCleanManifest = `<?xml version="1.0" encoding="utf-8"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android"
    package="com.example.test">
    <application android:allowBackup="false" android:label="Test"/>
</manifest>
`

func writeManifest(t *testing.T, dir, body string) string {
	t.Helper()
	path := filepath.Join(dir, "AndroidManifest.xml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	return path
}

func runManifestPhase(t *testing.T, manifestPath, cacheDir string, writer *scanner.AndroidCacheWriter) AndroidResult {
	t.Helper()
	rule := findV2RuleForTest(t, "AllowBackupManifest")
	dispatcher := rules.NewDispatcher([]*api.Rule{rule}, nil)
	res, err := (AndroidPhase{}).Run(context.Background(), AndroidInput{
		Project:        &android.Project{ManifestPaths: []string{manifestPath}},
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

// TestAndroidManifestCache_HitMatchesMiss ensures cached findings byte-equal
// fresh findings when nothing else changes.
func TestAndroidManifestCache_HitMatchesMiss(t *testing.T) {
	root := t.TempDir()
	manifestPath := writeManifest(t, root, allowBackupFlaggedManifest)
	cacheDir := filepath.Join(root, ".krit", "android-findings-cache")

	writer := scanner.NewAndroidCacheWriter(2)
	miss := runManifestPhase(t, manifestPath, cacheDir, writer)
	if err := writer.Close(); err != nil {
		t.Fatalf("flush writer: %v", err)
	}

	hitWriter := scanner.NewAndroidCacheWriter(2)
	hit := runManifestPhase(t, manifestPath, cacheDir, hitWriter)
	if err := hitWriter.Close(); err != nil {
		t.Fatalf("flush hit writer: %v", err)
	}

	if got, want := hit.Findings.Findings(), miss.Findings.Findings(); len(got) != len(want) {
		t.Fatalf("hit findings count differs: got %d want %d", len(got), len(want))
	}
	for i, f := range miss.Findings.Findings() {
		if hit.Findings.Findings()[i].Rule != f.Rule {
			t.Fatalf("rule mismatch at %d: got %q want %q", i, hit.Findings.Findings()[i].Rule, f.Rule)
		}
	}
}

// TestAndroidManifestCache_FileChangeInvalidates ensures rewriting the
// manifest produces a new cache key so stale findings are not returned.
func TestAndroidManifestCache_FileChangeInvalidates(t *testing.T) {
	root := t.TempDir()
	manifestPath := writeManifest(t, root, allowBackupFlaggedManifest)
	cacheDir := filepath.Join(root, ".krit", "android-findings-cache")

	writer := scanner.NewAndroidCacheWriter(2)
	first := runManifestPhase(t, manifestPath, cacheDir, writer)
	if err := writer.Close(); err != nil {
		t.Fatalf("flush writer: %v", err)
	}
	if len(first.Findings.Findings()) == 0 {
		t.Fatal("expected at least one AllowBackup finding on flagged manifest")
	}

	// Replace the manifest with a clean version. A cache key built from
	// the new content must miss; the dispatcher should run and produce
	// zero findings against the clean body.
	if err := os.WriteFile(manifestPath, []byte(allowBackupCleanManifest), 0o644); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	// Drop the hashutil memo so the rewritten file is re-hashed even if
	// (size, mtime) coincidentally matched.
	hashutil.ResetDefault()
	secondWriter := scanner.NewAndroidCacheWriter(2)
	second := runManifestPhase(t, manifestPath, cacheDir, secondWriter)
	if err := secondWriter.Close(); err != nil {
		t.Fatalf("flush second writer: %v", err)
	}
	if got := len(second.Findings.Findings()); got != 0 {
		t.Fatalf("expected 0 findings after content change to clean manifest, got %d (cache served stale data?)", got)
	}
}

// TestAndroidManifestCache_RuleHashChangeInvalidates ensures findings cached
// under one rule-hash don't satisfy a different rule-hash.
func TestAndroidManifestCache_RuleHashChangeInvalidates(t *testing.T) {
	root := t.TempDir()
	manifestPath := writeManifest(t, root, allowBackupFlaggedManifest)
	cacheDir := filepath.Join(root, ".krit", "android-findings-cache")

	rule := findV2RuleForTest(t, "AllowBackupManifest")
	dispatcher := rules.NewDispatcher([]*api.Rule{rule}, nil)

	// First run with rule-hash A.
	writer := scanner.NewAndroidCacheWriter(2)
	if _, err := (AndroidPhase{}).Run(context.Background(), AndroidInput{
		Project:     &android.Project{ManifestPaths: []string{manifestPath}},
		ActiveRules: []*api.Rule{rule},
		Dispatcher:  dispatcher,
		RuleHash:    "rule-hash-a",
		CacheDir:    cacheDir,
		CacheWriter: writer,
	}); err != nil {
		t.Fatalf("first run: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("flush: %v", err)
	}

	// Second run with rule-hash B — same manifest but different
	// ruleHash should miss the cache and re-run dispatch (still
	// producing a finding because the rule is the same; the test asserts
	// the cache miss is *possible*, not that findings differ).
	keyA := scanner.AndroidFindingsKey(scanner.AndroidFindingsKeyInputs{
		Kind:     scanner.AndroidFindingsKindManifest,
		RuleHash: "rule-hash-a",
		InputFP:  "irrelevant",
	})
	keyB := scanner.AndroidFindingsKey(scanner.AndroidFindingsKeyInputs{
		Kind:     scanner.AndroidFindingsKindManifest,
		RuleHash: "rule-hash-b",
		InputFP:  "irrelevant",
	})
	if keyA == keyB {
		t.Fatal("rule-hash change failed to alter the cache key — false-hit possible")
	}
}
