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

const dynamicVersionGradle = `dependencies {
    implementation("androidx.appcompat:appcompat:1.+")
}
`

const pinnedVersionGradle = `dependencies {
    implementation("androidx.appcompat:appcompat:1.7.0")
}
`

func writeGradle(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write gradle: %v", err)
	}
	return path
}

func runGradlePhase(t *testing.T, gradlePath, cacheDir string, writer *scanner.AndroidCacheWriter) AndroidResult {
	t.Helper()
	rule := findV2RuleForTest(t, "GradleDynamicVersion")
	dispatcher := rules.NewDispatcher([]*api.Rule{rule}, nil)
	res, err := (AndroidPhase{}).Run(context.Background(), AndroidInput{
		Project:        &android.Project{GradlePaths: []string{gradlePath}},
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

// TestAndroidGradleCache_HitMatchesMiss ensures cached findings byte-equal
// fresh findings.
func TestAndroidGradleCache_HitMatchesMiss(t *testing.T) {
	root := t.TempDir()
	gradlePath := writeGradle(t, root, "build.gradle.kts", dynamicVersionGradle)
	cacheDir := filepath.Join(root, ".krit", "android-findings-cache")

	writer := scanner.NewAndroidCacheWriter(2)
	miss := runGradlePhase(t, gradlePath, cacheDir, writer)
	if err := writer.Close(); err != nil {
		t.Fatalf("flush: %v", err)
	}

	hitWriter := scanner.NewAndroidCacheWriter(2)
	hit := runGradlePhase(t, gradlePath, cacheDir, hitWriter)
	if err := hitWriter.Close(); err != nil {
		t.Fatalf("flush hit: %v", err)
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

// TestAndroidGradleCache_FileChangeInvalidates ensures rewriting a build
// script changes the cache key so stale findings aren't served.
func TestAndroidGradleCache_FileChangeInvalidates(t *testing.T) {
	root := t.TempDir()
	gradlePath := writeGradle(t, root, "build.gradle.kts", dynamicVersionGradle)
	cacheDir := filepath.Join(root, ".krit", "android-findings-cache")

	writer := scanner.NewAndroidCacheWriter(2)
	first := runGradlePhase(t, gradlePath, cacheDir, writer)
	if err := writer.Close(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	if len(first.Findings.Findings()) == 0 {
		t.Fatal("expected at least one GradleDynamicVersion finding on '1.+' dep")
	}

	if err := os.WriteFile(gradlePath, []byte(pinnedVersionGradle), 0o644); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	hashutil.ResetDefault()
	secondWriter := scanner.NewAndroidCacheWriter(2)
	second := runGradlePhase(t, gradlePath, cacheDir, secondWriter)
	if err := secondWriter.Close(); err != nil {
		t.Fatalf("flush second: %v", err)
	}
	if got := len(second.Findings.Findings()); got != 0 {
		t.Fatalf("expected 0 findings after pinning the version, got %d (cache served stale data?)", got)
	}
}

// TestAndroidGradleCache_LibraryFactsFPInvalidates ensures a libraryFactsFP
// change forces a miss (e.g., a `libs.versions.toml` edit shifts every
// dependency-aware finding).
func TestAndroidGradleCache_LibraryFactsFPInvalidates(t *testing.T) {
	keyA := scanner.AndroidFindingsKey(scanner.AndroidFindingsKeyInputs{
		Kind:           scanner.AndroidFindingsKindGradle,
		LibraryFactsFP: "lf-a",
		InputFP:        "same",
		Extra:          "/path",
	})
	keyB := scanner.AndroidFindingsKey(scanner.AndroidFindingsKeyInputs{
		Kind:           scanner.AndroidFindingsKindGradle,
		LibraryFactsFP: "lf-b",
		InputFP:        "same",
		Extra:          "/path",
	})
	if keyA == keyB {
		t.Fatal("libraryFactsFP change failed to alter the gradle cache key")
	}
}

// TestAndroidGradleCache_PathFoldedIntoKey ensures two gradle files with
// identical bodies but different paths get distinct entries.
func TestAndroidGradleCache_PathFoldedIntoKey(t *testing.T) {
	a := scanner.AndroidFindingsKey(scanner.AndroidFindingsKeyInputs{
		Kind:    scanner.AndroidFindingsKindGradle,
		InputFP: "samebody",
		Extra:   "/proj/build.gradle",
	})
	b := scanner.AndroidFindingsKey(scanner.AndroidFindingsKeyInputs{
		Kind:    scanner.AndroidFindingsKindGradle,
		InputFP: "samebody",
		Extra:   "/proj/module/build.gradle",
	})
	if a == b {
		t.Fatal("identical-body gradle files at different paths must not share a cache key")
	}
}
