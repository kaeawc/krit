package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/android"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func TestAndroidProjectBundleCache_HitReturnsCachedFindings(t *testing.T) {
	root := t.TempDir()
	cacheDir := filepath.Join(root, ".krit", "android-findings-cache")
	manifestPath := filepath.Join(root, "src", "main", "AndroidManifest.xml")
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(manifestPath, []byte(`<manifest/>`), 0o644); err != nil {
		t.Fatal(err)
	}
	rule := &api.Rule{ID: "ManifestRule", Needs: api.NeedsManifest}
	in := AndroidInput{
		Project: &android.Project{
			ManifestPaths: []string{manifestPath},
		},
		ActiveRules: []*api.Rule{rule},
		CacheDir:    cacheDir,
		RuleHash:    "rules",
	}
	needs := classifyAndroidPhaseNeeds(in.ActiveRules)
	key, ok := in.androidProjectBundleKey(needs, 0, 0, nil)
	if !ok {
		t.Fatal("expected project bundle key")
	}
	want := scanner.CollectFindings([]scanner.Finding{{
		File:     manifestPath,
		Line:     1,
		Col:      1,
		Rule:     "ManifestRule",
		Severity: "warning",
		Message:  "cached",
	}})
	if err := scanner.SaveAndroidFindings(cacheDir, key, want); err != nil {
		t.Fatalf("SaveAndroidFindings: %v", err)
	}

	got, err := (AndroidPhase{}).Run(context.Background(), in)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got.Findings.Len() != 1 {
		t.Fatalf("Findings.Len() = %d, want 1", got.Findings.Len())
	}
	if msg := got.Findings.MessageAt(0); msg != "cached" {
		t.Fatalf("MessageAt(0) = %q, want cached", msg)
	}
}

func TestAndroidProjectBundleKey_ChangesWhenInputChanges(t *testing.T) {
	root := t.TempDir()
	manifestPath := filepath.Join(root, "AndroidManifest.xml")
	if err := os.WriteFile(manifestPath, []byte(`<manifest package="a"/>`), 0o644); err != nil {
		t.Fatal(err)
	}
	in := AndroidInput{
		Project: &android.Project{
			ManifestPaths: []string{manifestPath},
		},
		ActiveRules: []*api.Rule{{ID: "ManifestRule", Needs: api.NeedsManifest}},
		CacheDir:    filepath.Join(root, ".krit", "android-findings-cache"),
		RuleHash:    "rules",
	}
	needs := classifyAndroidPhaseNeeds(in.ActiveRules)
	first, ok := in.androidProjectBundleKey(needs, 0, 0, nil)
	if !ok {
		t.Fatal("expected first key")
	}
	if err := os.WriteFile(manifestPath, []byte(`<manifest package="b"/>`), 0o644); err != nil {
		t.Fatal(err)
	}
	second, ok := in.androidProjectBundleKey(needs, 0, 0, nil)
	if !ok {
		t.Fatal("expected second key")
	}
	if first == second {
		t.Fatal("project bundle key did not change after manifest content changed")
	}
}

func TestAndroidPhaseBundleKeys_AreSeparatedByKind(t *testing.T) {
	in := AndroidInput{
		RuleHash:            "rules",
		LibraryFactsFP:      "libs",
		JavaSemanticFactsFP: "java",
	}
	fp := "same-fingerprint"
	keys := []string{
		in.manifestBundleKey(fp),
		in.resourceBundleKey(fp, 1, 1),
		in.gradleBundleKey(fp),
		in.iconBundleKey(fp),
		in.resourceSourceBundleKey(fp, fp),
	}
	seen := make(map[string]bool, len(keys))
	for _, key := range keys {
		if key == "" {
			t.Fatal("bundle key must not be empty")
		}
		if seen[key] {
			t.Fatalf("bundle key collision for %q", key)
		}
		seen[key] = true
	}
}

func TestAndroidPhaseBundleKeys_InvalidateOnlyTheirInputs(t *testing.T) {
	in := AndroidInput{RuleHash: "rules", LibraryFactsFP: "libs"}
	manifestA := in.manifestBundleKey("manifest-a")
	manifestB := in.manifestBundleKey("manifest-b")
	gradleA := in.gradleBundleKey("gradle-a")
	gradleB := in.gradleBundleKey("gradle-b")
	resourceA := in.resourceBundleKey("res-a", 1, 1)
	resourceB := in.resourceBundleKey("res-b", 1, 1)
	if manifestA == manifestB {
		t.Fatal("manifest bundle key did not change with manifest fingerprint")
	}
	if gradleA == gradleB {
		t.Fatal("gradle bundle key did not change with gradle fingerprint")
	}
	if resourceA == resourceB {
		t.Fatal("resource bundle key did not change with resource fingerprint")
	}
	if manifestA == gradleA || manifestA == resourceA || gradleA == resourceA {
		t.Fatal("phase bundle keys should be separated by kind")
	}
}
