package scanner

import (
	"reflect"
	"testing"
)

// TestAppendIndexDataBuffers_StableAcrossBufferShuffle asserts that
// merging per-worker `indexDataBuffer`s produces a canonical
// (path-sorted) Symbols/References slice regardless of which worker
// "wins" which file. Regression for #30: workers pulled jobs from a
// shared channel and accumulated into per-worker slot buffers, so
// the post-merge slice's contents varied across runs even though
// slot indices were deterministic.
func TestAppendIndexDataBuffers_StableAcrossBufferShuffle(t *testing.T) {
	// Six symbols across three files. Arbitrary partitions across
	// workers must all yield the same canonical merged order.
	canonical := []Symbol{
		{Name: "Alpha", File: "/a.kt", StartByte: 10},
		{Name: "Beta", File: "/a.kt", StartByte: 50},
		{Name: "Gamma", File: "/b.kt", StartByte: 5},
		{Name: "Delta", File: "/b.kt", StartByte: 20},
		{Name: "Epsilon", File: "/c.kt", StartByte: 1},
		{Name: "Zeta", File: "/c.kt", StartByte: 100},
	}
	canonicalRefs := []Reference{
		{Name: "ref-a", File: "/a.kt", Line: 1},
		{Name: "ref-b", File: "/a.kt", Line: 5},
		{Name: "ref-c", File: "/b.kt", Line: 2},
		{Name: "ref-d", File: "/c.kt", Line: 9},
	}

	// Each shuffle represents one possible worker partitioning.
	shuffles := [][]struct {
		syms []int
		refs []int
	}{
		{
			{syms: []int{0, 3, 5}, refs: []int{0, 2}},
			{syms: []int{1, 4}, refs: []int{1, 3}},
			{syms: []int{2}, refs: nil},
		},
		{
			{syms: []int{5, 4, 3, 2, 1, 0}, refs: []int{3, 2, 1, 0}},
			{syms: nil, refs: nil},
			{syms: nil, refs: nil},
		},
		{
			{syms: []int{2}, refs: []int{2}},
			{syms: []int{0}, refs: []int{0}},
			{syms: []int{1}, refs: []int{1}},
			{syms: []int{4}, refs: []int{3}},
			{syms: []int{3}, refs: nil},
			{syms: []int{5}, refs: nil},
		},
	}

	for k, shuffle := range shuffles {
		buffers := make([]indexDataBuffer, len(shuffle))
		for w, alloc := range shuffle {
			for _, idx := range alloc.syms {
				buffers[w].symbols = append(buffers[w].symbols, canonical[idx])
			}
			for _, idx := range alloc.refs {
				buffers[w].refs = append(buffers[w].refs, canonicalRefs[idx])
			}
		}

		gotSyms, gotRefs := appendIndexDataBuffers(nil, nil, buffers)
		if !reflect.DeepEqual(gotSyms, canonical) {
			t.Fatalf("shuffle %d: symbols differ\n  got:  %#v\n  want: %#v", k, gotSyms, canonical)
		}
		if !reflect.DeepEqual(gotRefs, canonicalRefs) {
			t.Fatalf("shuffle %d: refs differ\n  got:  %#v\n  want: %#v", k, gotRefs, canonicalRefs)
		}
	}
}

// TestSortIndexSymbols_TiebreakerOnFQN guards the FQN tiebreaker
// when two declarations share the same short Name (different
// packages, same File and StartByte are unrealistic in real code
// but exercise the comparator's total-order property).
func TestSortIndexSymbols_TiebreakerOnFQN(t *testing.T) {
	in := []Symbol{
		{Name: "Foo", File: "/x.kt", StartByte: 0, FQN: "z.Foo"},
		{Name: "Foo", File: "/x.kt", StartByte: 0, FQN: "a.Foo"},
	}
	SortIndexSymbols(in)
	if in[0].FQN != "a.Foo" {
		t.Fatalf("expected a.Foo first, got %s", in[0].FQN)
	}
}

// TestSortIndexReferences_TiebreakerOnName covers the final
// tiebreaker for refs at the same (File, Line).
func TestSortIndexReferences_TiebreakerOnName(t *testing.T) {
	in := []Reference{
		{Name: "Zebra", File: "/x.kt", Line: 1},
		{Name: "Alpha", File: "/x.kt", Line: 1},
	}
	SortIndexReferences(in)
	if in[0].Name != "Alpha" {
		t.Fatalf("expected Alpha first, got %s", in[0].Name)
	}
}

// TestAppendIndexDataBuffers_PreservesExistingThenSorts confirms
// the merge step accommodates an already-populated `symbols` slice
// (called repeatedly across phases) and yields a single canonical
// order over the union.
func TestAppendIndexDataBuffers_PreservesExistingThenSorts(t *testing.T) {
	existing := []Symbol{
		{Name: "Bravo", File: "/b.kt", StartByte: 5},
		{Name: "Alpha", File: "/a.kt", StartByte: 10},
	}
	buffers := []indexDataBuffer{
		{symbols: []Symbol{
			{Name: "Charlie", File: "/c.kt", StartByte: 1},
		}},
	}
	got, _ := appendIndexDataBuffers(existing, nil, buffers)
	want := []Symbol{
		{Name: "Alpha", File: "/a.kt", StartByte: 10},
		{Name: "Bravo", File: "/b.kt", StartByte: 5},
		{Name: "Charlie", File: "/c.kt", StartByte: 1},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}
