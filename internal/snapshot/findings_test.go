package snapshot

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveLoadFindingsRoundTrip(t *testing.T) {
	root := t.TempDir()
	in := &Findings{
		SchemaVersion: FindingsSchemaVersion,
		CommitSHA:     "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
		RuleSetHash:   "ruleset-1",
		ByRule:        map[string]int{"MagicNumber": 3, "LongMethod": 1},
		ByRuleFile: map[string]map[string]int{
			"MagicNumber": {"src/Foo.kt": 2, "src/Bar.kt": 1},
			"LongMethod":  {"src/Foo.kt": 1},
		},
	}
	path, err := SaveFindings(root, in)
	if err != nil {
		t.Fatalf("SaveFindings: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("stat sidecar: %v", err)
	}
	if !filepath.IsAbs(path) && filepath.Base(path) != findingsFileName {
		t.Fatalf("unexpected path: %s", path)
	}

	got, err := LoadFindings(root, in.CommitSHA)
	if err != nil {
		t.Fatalf("LoadFindings: %v", err)
	}
	if got.RuleSetHash != in.RuleSetHash {
		t.Fatalf("RuleSetHash: got %q want %q", got.RuleSetHash, in.RuleSetHash)
	}
	if got.ByRule["MagicNumber"] != 3 || got.ByRule["LongMethod"] != 1 {
		t.Fatalf("ByRule mismatch: %+v", got.ByRule)
	}
	if got.ByRuleFile["MagicNumber"]["src/Foo.kt"] != 2 {
		t.Fatalf("ByRuleFile mismatch: %+v", got.ByRuleFile)
	}
}

func TestSaveFindingsRejectsEmptySHA(t *testing.T) {
	if _, err := SaveFindings(t.TempDir(), &Findings{}); err == nil {
		t.Fatal("expected error for empty CommitSHA")
	}
	if _, err := SaveFindings(t.TempDir(), nil); err == nil {
		t.Fatal("expected error for nil findings")
	}
}

func TestLoadFindingsMissingFile(t *testing.T) {
	root := t.TempDir()
	if _, err := LoadFindings(root, "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"); err == nil {
		t.Fatal("expected error when sidecar missing")
	}
}

func TestDiffFindingsByRule(t *testing.T) {
	root := t.TempDir()
	from := &Findings{
		SchemaVersion: FindingsSchemaVersion,
		CommitSHA:     "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		RuleSetHash:   "ruleset-1",
		ByRule:        map[string]int{"MagicNumber": 5, "Dead": 2},
	}
	to := &Findings{
		SchemaVersion: FindingsSchemaVersion,
		CommitSHA:     "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		RuleSetHash:   "ruleset-1",
		ByRule:        map[string]int{"MagicNumber": 3, "New": 1},
	}
	// Write minimal blobs so Diff can run.
	mustSaveStubBlob(t, root, from.CommitSHA)
	mustSaveStubBlob(t, root, to.CommitSHA)
	if _, err := SaveFindings(root, from); err != nil {
		t.Fatalf("save from: %v", err)
	}
	if _, err := SaveFindings(root, to); err != nil {
		t.Fatalf("save to: %v", err)
	}

	d, err := Diff(root, from.CommitSHA, to.CommitSHA)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if d.FindingsRuleSetMismatch {
		t.Fatalf("unexpected rule-set mismatch on identical hashes")
	}
	if d.FindingsByRule["MagicNumber"].Delta != -2 {
		t.Fatalf("MagicNumber delta: %+v", d.FindingsByRule["MagicNumber"])
	}
	if d.FindingsByRule["Dead"].Delta != -2 || d.FindingsByRule["Dead"].To != 0 {
		t.Fatalf("Dead delta: %+v", d.FindingsByRule["Dead"])
	}
	if d.FindingsByRule["New"].From != 0 || d.FindingsByRule["New"].To != 1 {
		t.Fatalf("New delta: %+v", d.FindingsByRule["New"])
	}
}

func TestDiffFindingsRuleSetMismatch(t *testing.T) {
	root := t.TempDir()
	from := &Findings{
		SchemaVersion: FindingsSchemaVersion,
		CommitSHA:     "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		RuleSetHash:   "ruleset-1",
		ByRule:        map[string]int{"MagicNumber": 5},
	}
	to := &Findings{
		SchemaVersion: FindingsSchemaVersion,
		CommitSHA:     "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		RuleSetHash:   "ruleset-2",
		ByRule:        map[string]int{"MagicNumber": 3},
	}
	mustSaveStubBlob(t, root, from.CommitSHA)
	mustSaveStubBlob(t, root, to.CommitSHA)
	if _, err := SaveFindings(root, from); err != nil {
		t.Fatalf("save from: %v", err)
	}
	if _, err := SaveFindings(root, to); err != nil {
		t.Fatalf("save to: %v", err)
	}

	d, err := Diff(root, from.CommitSHA, to.CommitSHA)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if !d.FindingsRuleSetMismatch {
		t.Fatal("expected FindingsRuleSetMismatch=true on differing RuleSetHash")
	}
	if len(d.FindingsByRule) != 0 {
		t.Fatalf("expected no findings delta when rule sets differ, got %+v", d.FindingsByRule)
	}
}

func mustSaveStubBlob(t *testing.T, root, sha string) {
	t.Helper()
	blob := &Blob{
		SchemaVersion: SchemaVersion,
		CommitSHA:     sha,
		CapturedAt:    1,
	}
	if _, err := Save(root, blob); err != nil {
		t.Fatalf("save stub blob: %v", err)
	}
}
