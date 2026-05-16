package traces

import (
	"path/filepath"
	"testing"
)

func TestStoreRoundTrip(t *testing.T) {
	root := t.TempDir()
	s, err := Load(root)
	if err != nil {
		t.Fatalf("load empty: %v", err)
	}
	if len(s.States) != 0 {
		t.Fatalf("empty store should have no states")
	}
	src := IngestSource{ID: "otel-aaa", Kind: SourceOTel, StartedAt: 1, CommitSHA: "deadbeef"}
	st := []RuntimeState{{Fingerprint: "fp1", TopSymbol: "Foo.bar", Role: RoleRequest, Count: 5, FirstSeen: 1, LastSeen: 9, Sources: []string{"otel-aaa"}}}
	tr := []RuntimeTransition{{FromFP: "fp1", ToFP: "fp2", Kind: KindCall, Count: 1}}
	s.Merge(src, st, tr)
	if err := s.Save(root); err != nil {
		t.Fatalf("save: %v", err)
	}
	got := filepath.Join(root, StoreDir, "store.json")
	if got == "" {
		t.Fatalf("missing path")
	}
	s2, err := Load(root)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if len(s2.States) != 1 || s2.States[0].Count != 5 {
		t.Fatalf("round-trip lost state: %+v", s2.States)
	}
	// Re-merge same fingerprint aggregates count.
	s2.Merge(src, st, nil)
	if s2.States[0].Count != 10 {
		t.Fatalf("merge should add counts, got %d", s2.States[0].Count)
	}
}

func TestStoreSetResolutionAndSuggestions(t *testing.T) {
	s := &Store{SchemaVersion: SchemaVersion, Resolutions: map[string]Resolution{}}
	s.SetResolution("fp1", ResolvedExact)
	s.AddSuggestions("fp2", []StateSuggestion{
		{Fingerprint: "fp2", CandidateSymbol: "com.acme.A.b", Evidence: "name-similarity", Confidence: 0.9},
		{Fingerprint: "fp2", CandidateSymbol: "com.acme.C.b", Evidence: "name-similarity", Confidence: 0.4},
	})
	if s.Resolutions["fp1"] != ResolvedExact {
		t.Fatalf("resolution not set")
	}
	if len(s.Suggestions) != 2 {
		t.Fatalf("suggestions not set: %d", len(s.Suggestions))
	}
}
