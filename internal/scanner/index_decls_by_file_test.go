package scanner

import (
	"sort"
	"testing"
)

// sortedDeclaredNames returns DeclaredNames(file) sorted for stable comparison.
func sortedDeclaredNames(idx *CodeIndex, file string) []string {
	got := idx.DeclaredNames(file)
	sort.Strings(got)
	return got
}

func eqStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestDeclaredNames_BuildFromData pins the forward declaration map produced by
// the normal build path: each file maps to the distinct names and FQNs it
// declares. The FQN is included alongside the bare name so a changed file can
// be matched against references by either spelling.
func TestDeclaredNames_BuildFromData(t *testing.T) {
	symbols := []Symbol{
		{Name: "helperFunc", File: "a.kt", FQN: "demo.helperFunc"},
		{Name: "HelperClass", File: "b.kt", FQN: "demo.HelperClass"},
		{Name: "topLevel", File: "b.kt", FQN: ""}, // no FQN: name only
	}
	idx := BuildIndexFromData(symbols, nil)

	if got, want := sortedDeclaredNames(idx, "a.kt"), []string{"demo.helperFunc", "helperFunc"}; !eqStrings(got, want) {
		t.Errorf("a.kt declared = %v, want %v", got, want)
	}
	if got, want := sortedDeclaredNames(idx, "b.kt"), []string{"HelperClass", "demo.HelperClass", "topLevel"}; !eqStrings(got, want) {
		t.Errorf("b.kt declared = %v, want %v", got, want)
	}
	if got := idx.DeclaredNames("missing.kt"); got != nil {
		t.Errorf("unknown file must return nil; got %v", got)
	}
}

// TestDeclaredNames_NilSafety verifies the accessor tolerates a nil receiver
// and a zero-value index without a declsByFile map.
func TestDeclaredNames_NilSafety(t *testing.T) {
	var idx *CodeIndex
	if got := idx.DeclaredNames("a.kt"); got != nil {
		t.Errorf("nil receiver must return nil; got %v", got)
	}
	if got := (&CodeIndex{}).DeclaredNames("a.kt"); got != nil {
		t.Errorf("zero-value index must return nil; got %v", got)
	}
}

// TestDeclaredNames_FreshCopy verifies the returned slice is owned by the
// caller: mutating it must not corrupt the internal multiset.
func TestDeclaredNames_FreshCopy(t *testing.T) {
	idx := BuildIndexFromData([]Symbol{{Name: "f", File: "a.kt"}}, nil)
	got := idx.DeclaredNames("a.kt")
	if len(got) != 1 {
		t.Fatalf("expected 1 name, got %v", got)
	}
	got[0] = "mutated"
	if again := idx.DeclaredNames("a.kt"); len(again) != 1 || again[0] != "f" {
		t.Errorf("mutating returned slice corrupted internal state: %v", again)
	}
}

// TestDeclaredNames_IncrementalAddRemove pins the exact-count multiset
// behavior across the incremental overlay path: overloads sharing a name must
// require exactly as many removals as additions before the name disappears.
func TestDeclaredNames_IncrementalAddRemove(t *testing.T) {
	// Two overloads of overloaded() in a.kt — same Name, distinct FQN/Arity.
	base := BuildIndexFromData([]Symbol{
		{Name: "overloaded", File: "a.kt", FQN: "demo.overloaded", Arity: 0},
		{Name: "overloaded", File: "a.kt", FQN: "demo.overloaded", Arity: 1},
		{Name: "keep", File: "a.kt", FQN: "demo.keep"},
	}, nil)

	// Whole-file removal must drop every name for the file.
	idx := BuildIndexIncremental(base, map[string]bool{"a.kt": true}, nil, nil)
	if got := idx.DeclaredNames("a.kt"); got != nil {
		t.Fatalf("whole-file removal must clear declarations; got %v", got)
	}

	// Re-add the same overload set, then verify the name survives one removal.
	idx = BuildIndexIncremental(idx, nil, []Symbol{
		{Name: "overloaded", File: "a.kt", FQN: "demo.overloaded", Arity: 0},
		{Name: "overloaded", File: "a.kt", FQN: "demo.overloaded", Arity: 1},
	}, nil)
	// removeFileContributions is whole-file; exercise the per-symbol decrement
	// directly to prove the count multiset, which the affected-set needs when a
	// single overload is dropped from a still-present file.
	idx.removeSymbolFromLookups(Symbol{Name: "overloaded", File: "a.kt", FQN: "demo.overloaded", Arity: 0})
	if got, want := sortedDeclaredNames(idx, "a.kt"), []string{"demo.overloaded", "overloaded"}; !eqStrings(got, want) {
		t.Errorf("after dropping one overload, name must remain; got %v want %v", got, want)
	}
	idx.removeSymbolFromLookups(Symbol{Name: "overloaded", File: "a.kt", FQN: "demo.overloaded", Arity: 1})
	if got := idx.DeclaredNames("a.kt"); got != nil {
		t.Errorf("after dropping both overloads, file must be empty; got %v", got)
	}
}

// TestDeclaredNames_CacheRoundTrip verifies the forward map is reconstructed by
// the warm cache-unpack path (unpackFull), which populates the lookup maps
// directly and so relies on rebuildDeclsByFile.
func TestDeclaredNames_CacheRoundTrip(t *testing.T) {
	symbols := []Symbol{
		{Name: "helperFunc", Kind: "function", Visibility: "public", File: "a.kt", Line: 10, StartByte: 3, EndByte: 18, Language: LangKotlin, Package: "demo", FQN: "demo.helperFunc", Signature: "<package>#helperFunc/0"},
		{Name: "HelperClass", Kind: "class", Visibility: "internal", File: "b.kt", Line: 2, StartByte: 0, EndByte: 12, Language: LangJava, Package: "demo", FQN: "demo.HelperClass", Signature: "demo.HelperClass", IsFinal: true},
	}
	refs := []Reference{
		{Name: "helperFunc", File: "a.kt", Line: 10},
		{Name: "HelperClass", File: "c.kt", Line: 7},
	}

	src := BuildIndexFromData(symbols, refs)
	payload := packPayloadWithIndex(src)
	idx, ok := payload.unpackFull()
	if !ok {
		t.Fatalf("unpackFull failed")
	}
	if got, want := sortedDeclaredNames(idx, "a.kt"), []string{"demo.helperFunc", "helperFunc"}; !eqStrings(got, want) {
		t.Errorf("a.kt declared after round-trip = %v, want %v", got, want)
	}
	if got, want := sortedDeclaredNames(idx, "b.kt"), []string{"HelperClass", "demo.HelperClass"}; !eqStrings(got, want) {
		t.Errorf("b.kt declared after round-trip = %v, want %v", got, want)
	}
}
