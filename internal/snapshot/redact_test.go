package snapshot

import (
	"strings"
	"testing"
)

func TestRedactBlob_HashesIdentifiersAndPaths(t *testing.T) {
	b := &Blob{
		SchemaVersion: SchemaVersion,
		KritVersion:   "1.2.3",
		CommitSHA:     "abc",
		CapturedAt:    100,
		RepoRoot:      "/home/jane/proprietary-project",
		Files: []File{
			{Path: "src/main/kotlin/com/secret/Foo.kt", Language: "kotlin", Lines: 10, Bytes: 100, Module: ":app"},
		},
		Symbols: []Symbol{
			{
				Name: "computeProprietaryThing", Kind: "function", Visibility: "public",
				File: "src/main/kotlin/com/secret/Foo.kt", Line: 5, Language: "kotlin",
				Package: "com.secret", FQN: "com.secret.Foo.computeProprietaryThing",
				Owner: "Foo", Signature: "computeProprietaryThing(Int): String",
			},
		},
	}
	RedactBlob(b)

	if !b.Redacted {
		t.Fatalf("Redacted flag must be true after RedactBlob")
	}
	if b.RepoRoot != "" {
		t.Errorf("RepoRoot must be cleared; got %q", b.RepoRoot)
	}
	// Preserved fields: kind, visibility, line, language, lines, bytes, module
	if got := b.Symbols[0].Kind; got != "function" {
		t.Errorf("Kind must survive redaction; got %q", got)
	}
	if got := b.Symbols[0].Line; got != 5 {
		t.Errorf("Line must survive redaction; got %d", got)
	}
	if got := b.Files[0].Lines; got != 10 {
		t.Errorf("Lines must survive redaction; got %d", got)
	}
	if got := b.Files[0].Module; got != ":app" {
		t.Errorf("Module must survive redaction; got %q", got)
	}
	// Hashed fields all start with the marker and contain no source identifier.
	for _, field := range []string{
		b.Symbols[0].Name, b.Symbols[0].FQN, b.Symbols[0].Owner,
		b.Symbols[0].Package, b.Symbols[0].Signature,
	} {
		if !strings.HasPrefix(field, RedactionMarker) {
			t.Errorf("redacted field missing marker: %q", field)
		}
		if strings.Contains(field, "Proprietary") || strings.Contains(field, "secret") {
			t.Errorf("redacted field still contains source identifier: %q", field)
		}
	}
	// Path is per-segment hashed: depth survives, names don't.
	parts := strings.Split(b.Symbols[0].File, "/")
	if len(parts) != 6 {
		t.Errorf("path depth must survive redaction (want 6, got %d): %q", len(parts), b.Symbols[0].File)
	}
	for _, p := range parts {
		if !strings.HasPrefix(p, RedactionMarker) {
			t.Errorf("path segment missing marker: %q", p)
		}
	}
}

func TestRedactBlob_Idempotent(t *testing.T) {
	b := &Blob{
		Symbols: []Symbol{{Name: "x", FQN: "p.x"}},
		Files:   []File{{Path: "a/b.kt"}},
	}
	RedactBlob(b)
	first := b.Symbols[0].FQN
	RedactBlob(b) // second call must be a no-op
	if b.Symbols[0].FQN != first {
		t.Errorf("RedactBlob not idempotent: got %q, want %q", b.Symbols[0].FQN, first)
	}
}

func TestRedactBlob_StableHashAcrossCalls(t *testing.T) {
	a := &Blob{Symbols: []Symbol{{FQN: "com.example.X"}}}
	b := &Blob{Symbols: []Symbol{{FQN: "com.example.X"}}}
	RedactBlob(a)
	RedactBlob(b)
	if a.Symbols[0].FQN != b.Symbols[0].FQN {
		t.Errorf("hashing must be deterministic across blobs: a=%q b=%q",
			a.Symbols[0].FQN, b.Symbols[0].FQN)
	}
}

func TestRedactFindings_HashesPaths(t *testing.T) {
	f := &Findings{
		ByRule: map[string]int{"PrintlnInProduction": 3},
		ByRuleFile: map[string]map[string]int{
			"PrintlnInProduction": {
				"src/main/kotlin/com/secret/Foo.kt": 2,
				"src/main/kotlin/com/secret/Bar.kt": 1,
			},
		},
	}
	RedactFindings(f)

	if !f.Redacted {
		t.Fatalf("Findings.Redacted must be true after RedactFindings")
	}
	// Rule IDs stay clear.
	if f.ByRule["PrintlnInProduction"] != 3 {
		t.Errorf("rule ID totals must survive redaction; got %v", f.ByRule)
	}
	per := f.ByRuleFile["PrintlnInProduction"]
	if per == nil {
		t.Fatalf("rule per-file map missing after redaction")
	}
	totalCount := 0
	for path, count := range per {
		if !strings.HasPrefix(path, RedactionMarker) {
			t.Errorf("ByRuleFile path key missing marker: %q", path)
		}
		if strings.Contains(path, "secret") || strings.Contains(path, "Foo") {
			t.Errorf("ByRuleFile path still contains source identifier: %q", path)
		}
		totalCount += count
	}
	if totalCount != 3 {
		t.Errorf("redaction must preserve per-file count sums; got %d want 3", totalCount)
	}
}

func TestDiff_RefusesRedactedVsRaw(t *testing.T) {
	root := t.TempDir()
	rawSHA := "1111111111111111111111111111111111111111"
	redSHA := "2222222222222222222222222222222222222222"

	if _, err := Save(root, &Blob{
		SchemaVersion: SchemaVersion, CommitSHA: rawSHA, CapturedAt: 1,
		Symbols: []Symbol{{FQN: "p.X"}},
	}); err != nil {
		t.Fatalf("save raw: %v", err)
	}
	redacted := &Blob{
		SchemaVersion: SchemaVersion, CommitSHA: redSHA, CapturedAt: 2,
		Symbols: []Symbol{{FQN: "p.X"}},
	}
	RedactBlob(redacted)
	if _, err := Save(root, redacted); err != nil {
		t.Fatalf("save redacted: %v", err)
	}

	_, err := Diff(root, rawSHA, redSHA)
	if err == nil {
		t.Fatalf("Diff must refuse to compare redacted vs raw")
	}
	if !strings.Contains(err.Error(), "redacted vs raw") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestDiff_AllowsRedactedVsRedacted(t *testing.T) {
	root := t.TempDir()
	makeRedacted := func(sha, fqn string, capturedAt int64) {
		b := &Blob{
			SchemaVersion: SchemaVersion, CommitSHA: sha, CapturedAt: capturedAt,
			Symbols: []Symbol{{FQN: fqn}},
		}
		RedactBlob(b)
		if _, err := Save(root, b); err != nil {
			t.Fatalf("save %s: %v", sha, err)
		}
	}
	from := "1111111111111111111111111111111111111111"
	to := "2222222222222222222222222222222222222222"
	makeRedacted(from, "com.example.X", 1)
	makeRedacted(to, "com.example.X", 2)

	result, err := Diff(root, from, to)
	if err != nil {
		t.Fatalf("Diff of two redacted snapshots failed: %v", err)
	}
	// Same FQN on both sides → no added/removed symbols.
	if len(result.AddedSymbols)+len(result.RemovedSymbols) != 0 {
		t.Errorf("expected no symbol churn for identical inputs; got added=%d removed=%d",
			len(result.AddedSymbols), len(result.RemovedSymbols))
	}
}
