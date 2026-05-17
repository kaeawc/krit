package scanner

import (
	"reflect"
	"testing"
)

// fixtureForTransitiveDependents builds a small index with three names
// referenced across known files. Each helper test below builds its own
// fixture so subtests stay independent and Parallel-safe.
func fixtureForTransitiveDependents() *CodeIndex {
	symbols := []Symbol{
		{Name: "Alpha", Kind: "class", Visibility: "public", File: "alpha.kt", Line: 1},
		{Name: "Beta", Kind: "class", Visibility: "public", File: "beta.kt", Line: 1},
	}
	refs := []Reference{
		// Alpha is declared in alpha.kt and referenced from b.kt and c.kt.
		{Name: "Alpha", File: "alpha.kt", Line: 1},
		{Name: "Alpha", File: "b.kt", Line: 5},
		{Name: "Alpha", File: "c.kt", Line: 7},
		// Beta is referenced from c.kt (overlap with Alpha) and d.kt.
		{Name: "Beta", File: "beta.kt", Line: 1},
		{Name: "Beta", File: "c.kt", Line: 9},
		{Name: "Beta", File: "d.kt", Line: 3},
	}
	return BuildIndexFromData(symbols, refs)
}

func TestTransitiveDependents_EmptyNames(t *testing.T) {
	t.Parallel()
	idx := fixtureForTransitiveDependents()
	if got := idx.TransitiveDependents(nil, ""); got != nil {
		t.Fatalf("TransitiveDependents(nil, \"\") = %v, want nil", got)
	}
	if got := idx.TransitiveDependents([]string{}, "alpha.kt"); got != nil {
		t.Fatalf("TransitiveDependents([], excludeFile) = %v, want nil", got)
	}
}

func TestTransitiveDependents_NilIndex(t *testing.T) {
	t.Parallel()
	var idx *CodeIndex
	if got := idx.TransitiveDependents([]string{"Alpha"}, ""); got != nil {
		t.Fatalf("nil idx TransitiveDependents = %v, want nil", got)
	}
}

func TestTransitiveDependents_ExcludesOriginatingFile(t *testing.T) {
	t.Parallel()
	idx := fixtureForTransitiveDependents()

	got := idx.TransitiveDependents([]string{"Alpha"}, "alpha.kt")
	want := []string{"b.kt", "c.kt"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("TransitiveDependents(Alpha, alpha.kt) = %v, want %v", got, want)
	}

	// b.kt is itself a referencing file — exclude it and the other dependent
	// (c.kt) plus the declaring file (alpha.kt) should both appear.
	gotB := idx.TransitiveDependents([]string{"Alpha"}, "b.kt")
	wantB := []string{"alpha.kt", "c.kt"}
	if !reflect.DeepEqual(gotB, wantB) {
		t.Fatalf("TransitiveDependents(Alpha, b.kt) = %v, want %v", gotB, wantB)
	}
}

func TestTransitiveDependents_MultipleNamesDedupe(t *testing.T) {
	t.Parallel()
	idx := fixtureForTransitiveDependents()

	// Alpha is in b.kt + c.kt (excluding alpha.kt). Beta is in c.kt + d.kt
	// (excluding beta.kt). Querying both with no exclude should produce
	// the union {alpha.kt, b.kt, beta.kt, c.kt, d.kt} with c.kt deduped.
	got := idx.TransitiveDependents([]string{"Alpha", "Beta"}, "")
	want := []string{"alpha.kt", "b.kt", "beta.kt", "c.kt", "d.kt"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("TransitiveDependents(Alpha+Beta, \"\") = %v, want %v", got, want)
	}

	// Exclude alpha.kt (the declaring file of Alpha) — alpha.kt should drop
	// from the union but c.kt should still appear once even though both
	// names reference it.
	gotExA := idx.TransitiveDependents([]string{"Alpha", "Beta"}, "alpha.kt")
	wantExA := []string{"b.kt", "beta.kt", "c.kt", "d.kt"}
	if !reflect.DeepEqual(gotExA, wantExA) {
		t.Fatalf("TransitiveDependents(Alpha+Beta, alpha.kt) = %v, want %v", gotExA, wantExA)
	}
}

func TestTransitiveDependents_BloomMiss(t *testing.T) {
	t.Parallel()
	idx := fixtureForTransitiveDependents()

	// "Gamma" was never added to the index, so the bloom filter rejects
	// it and refFilesByName has no entry. Result for just that name must
	// be nil (no allocation, deterministic empty output).
	if got := idx.TransitiveDependents([]string{"Gamma"}, ""); got != nil {
		t.Fatalf("TransitiveDependents(Gamma) = %v, want nil", got)
	}

	// Mixing Gamma with a known name shouldn't pollute the result with
	// any "Gamma" matches — only Alpha's files appear.
	got := idx.TransitiveDependents([]string{"Gamma", "Alpha"}, "alpha.kt")
	want := []string{"b.kt", "c.kt"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("TransitiveDependents(Gamma+Alpha, alpha.kt) = %v, want %v", got, want)
	}

	// Empty string in the input list must be skipped silently rather than
	// triggering a lookup on the empty-string key.
	gotEmpty := idx.TransitiveDependents([]string{"", "Alpha"}, "alpha.kt")
	if !reflect.DeepEqual(gotEmpty, want) {
		t.Fatalf("TransitiveDependents(\"\"+Alpha, alpha.kt) = %v, want %v", gotEmpty, want)
	}
}

func TestTransitiveDependents_DeterministicOrder(t *testing.T) {
	t.Parallel()
	idx := fixtureForTransitiveDependents()

	// Two consecutive runs with the same input must produce byte-identical
	// output — order matters for downstream hash-based caches.
	a := idx.TransitiveDependents([]string{"Alpha", "Beta"}, "")
	b := idx.TransitiveDependents([]string{"Beta", "Alpha"}, "")
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("non-deterministic order: %v vs %v", a, b)
	}

	// Verify the output is actually sorted ascending.
	for i := 1; i < len(a); i++ {
		if a[i-1] >= a[i] {
			t.Fatalf("output not sorted ascending: %v", a)
		}
	}
}
