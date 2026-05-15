package snapshot

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/breakage"
)

func TestSynthesizeFindingRegressionsCreatesEventsOnNewFindings(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".krit", "snapshots")

	prevSHA := "1111111111111111111111111111111111111111"
	curSHA := "2222222222222222222222222222222222222222"

	// Two captured manifests so loadPreviousFindings has something to
	// compare against.
	prevManifest := &Manifest{
		SchemaVersion: ManifestSchemaVersion,
		CommitSHA:     prevSHA,
		CapturedAt:    1,
		BlobSchema:    SchemaVersion,
	}
	curManifest := &Manifest{
		SchemaVersion: ManifestSchemaVersion,
		CommitSHA:     curSHA,
		CapturedAt:    2,
		BlobSchema:    SchemaVersion,
	}
	if _, err := SaveManifest(root, prevManifest); err != nil {
		t.Fatalf("SaveManifest(prev): %v", err)
	}
	if _, err := SaveManifest(root, curManifest); err != nil {
		t.Fatalf("SaveManifest(cur): %v", err)
	}

	prevFindings := &Findings{
		SchemaVersion: FindingsSchemaVersion,
		CommitSHA:     prevSHA,
		ByRule:        map[string]int{"old-rule": 1},
		ByRuleFile:    map[string]map[string]int{"old-rule": {"app/A.kt": 1}},
	}
	if _, err := SaveFindings(root, prevFindings); err != nil {
		t.Fatalf("SaveFindings(prev): %v", err)
	}

	curFindings := &Findings{
		SchemaVersion: FindingsSchemaVersion,
		CommitSHA:     curSHA,
		ByRule:        map[string]int{"old-rule": 1, "new-rule": 2},
		ByRuleFile: map[string]map[string]int{
			"old-rule": {"app/A.kt": 1},
			"new-rule": {"app/A.kt": 1, "app/B.kt": 1},
		},
	}

	added, err := SynthesizeFindingRegressions(root, curFindings, time.Unix(0, 0))
	if err != nil {
		t.Fatalf("SynthesizeFindingRegressions: %v", err)
	}
	if added != 2 {
		t.Fatalf("added=%d, want 2 (new-rule fires on two files)", added)
	}

	events, err := breakage.LoadAll(root)
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("events len=%d, want 2", len(events))
	}
	for _, e := range events {
		if e.FailureKind != breakage.KindKritFindingRegression {
			t.Errorf("event kind=%q, want %q", e.FailureKind, breakage.KindKritFindingRegression)
		}
		if e.CommitSHA != curSHA {
			t.Errorf("event commit=%q, want %q", e.CommitSHA, curSHA)
		}
		if e.Source != breakage.SourceKritFinding {
			t.Errorf("event source=%q, want %q", e.Source, breakage.SourceKritFinding)
		}
	}
}

func TestSynthesizeFindingRegressionsSkipsRuleSetMismatch(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".krit", "snapshots")

	prevSHA := "1111111111111111111111111111111111111111"
	curSHA := "2222222222222222222222222222222222222222"
	if _, err := SaveManifest(root, &Manifest{SchemaVersion: ManifestSchemaVersion, CommitSHA: prevSHA, CapturedAt: 1, BlobSchema: SchemaVersion}); err != nil {
		t.Fatalf("SaveManifest(prev): %v", err)
	}
	if _, err := SaveManifest(root, &Manifest{SchemaVersion: ManifestSchemaVersion, CommitSHA: curSHA, CapturedAt: 2, BlobSchema: SchemaVersion}); err != nil {
		t.Fatalf("SaveManifest(cur): %v", err)
	}
	if _, err := SaveFindings(root, &Findings{SchemaVersion: FindingsSchemaVersion, CommitSHA: prevSHA, RuleSetHash: "hash-a", ByRuleFile: map[string]map[string]int{}}); err != nil {
		t.Fatalf("SaveFindings(prev): %v", err)
	}
	cur := &Findings{
		SchemaVersion: FindingsSchemaVersion,
		CommitSHA:     curSHA,
		RuleSetHash:   "hash-b",
		ByRuleFile:    map[string]map[string]int{"new-rule": {"app/A.kt": 5}},
	}
	added, err := SynthesizeFindingRegressions(root, cur, time.Unix(0, 0))
	if err != nil {
		t.Fatalf("SynthesizeFindingRegressions: %v", err)
	}
	if added != 0 {
		t.Errorf("added=%d, want 0 (rule-set mismatch should suppress events)", added)
	}
}

func TestSynthesizeFindingRegressionsFirstCaptureNoOp(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, ".krit", "snapshots")
	cur := &Findings{
		SchemaVersion: FindingsSchemaVersion,
		CommitSHA:     "abc",
		ByRuleFile:    map[string]map[string]int{"new-rule": {"app/A.kt": 1}},
	}
	added, err := SynthesizeFindingRegressions(root, cur, time.Unix(0, 0))
	if err != nil {
		t.Fatalf("SynthesizeFindingRegressions: %v", err)
	}
	if added != 0 {
		t.Errorf("added=%d, want 0 (first capture should not emit regressions)", added)
	}
}
