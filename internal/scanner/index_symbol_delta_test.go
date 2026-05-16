package scanner

import (
	"reflect"
	"sort"
	"testing"
)

// TestSymbolLookups_DeltaEqualsFullRebuild pins the regression-critical
// equivalence: applying add/remove deltas through BuildIndexIncremental
// produces the same lookup maps as building from the equivalent final
// symbol set with rebuildSymbolLookups. If this drifts, ReferenceCount,
// SymbolByFQN, and downstream rule queries silently disagree across
// daemon analyzes and one-shot CLI runs.
func TestSymbolLookups_DeltaEqualsFullRebuild(t *testing.T) {
	// Seed with a mixed set: same Name in multiple files (slice-bucket
	// in symbolsByName), FQN equal to Name and distinct from Name
	// (both branches of the addSymbolToLookups conditional).
	initial := []Symbol{
		{Name: "Foo", Kind: "class", File: "a.kt", FQN: "p.a.Foo"},
		{Name: "Foo", Kind: "class", File: "b.kt", FQN: "p.b.Foo"}, // same Name, different file/FQN
		{Name: "Bar", Kind: "function", File: "a.kt", FQN: "Bar"},  // FQN == Name
		{Name: "Baz", Kind: "object", File: "c.kt", FQN: "p.c.Baz"},
	}

	delta := BuildIndexFromData(append([]Symbol(nil), initial...), nil)
	BuildIndexIncremental(delta, map[string]bool{"b.kt": true}, []Symbol{
		{Name: "Quux", Kind: "class", File: "b.kt", FQN: "p.b.Quux"},
		{Name: "Baz", Kind: "object", File: "b.kt", FQN: "p.b.Baz"}, // same Name as existing
	}, nil)

	// Compute the equivalent post-delta symbol set explicitly and
	// build fresh — apples-to-apples comparison target.
	wanted := []Symbol{
		{Name: "Foo", Kind: "class", File: "a.kt", FQN: "p.a.Foo"},
		{Name: "Bar", Kind: "function", File: "a.kt", FQN: "Bar"},
		{Name: "Baz", Kind: "object", File: "c.kt", FQN: "p.c.Baz"},
		{Name: "Quux", Kind: "class", File: "b.kt", FQN: "p.b.Quux"},
		{Name: "Baz", Kind: "object", File: "b.kt", FQN: "p.b.Baz"},
	}
	want := BuildIndexFromData(wanted, nil)

	if !equalSymbolsByName(delta.symbolsByName, want.symbolsByName) {
		t.Errorf("symbolsByName diverged from full rebuild\n delta=%v\n want=%v", delta.symbolsByName, want.symbolsByName)
	}
	if !reflect.DeepEqual(delta.symbolsByFQN, want.symbolsByFQN) {
		t.Errorf("symbolsByFQN diverged from full rebuild\n delta=%v\n want=%v", delta.symbolsByFQN, want.symbolsByFQN)
	}
}

// TestRemoveAllInstancesDeletesKey confirms the slice-bucket cleanup —
// when every symbol under a name key is removed, the key must be
// deleted outright so callers iterating maps see no empty buckets.
// rebuildSymbolLookups never leaves empty slices either, so this
// matches the post-rebuild shape.
func TestRemoveAllInstancesDeletesKey(t *testing.T) {
	idx := BuildIndexFromData([]Symbol{
		{Name: "Only", Kind: "class", File: "a.kt", FQN: "p.Only"},
	}, nil)
	if _, ok := idx.symbolsByName["Only"]; !ok {
		t.Fatalf("setup: Only key missing before remove")
	}
	BuildIndexIncremental(idx, map[string]bool{"a.kt": true}, nil, nil)
	if syms, ok := idx.symbolsByName["Only"]; ok {
		t.Errorf("remove-all left dangling map key: %v", syms)
	}
	if _, ok := idx.symbolsByFQN["p.Only"]; ok {
		t.Errorf("FQN entry survived remove-all")
	}
}

// TestRemoveOneOfManyKeepsSurvivors confirms the slice-bucket pruning
// preserves untouched symbols. The current rebuildSymbolLookups path
// produces a slice in the original insertion order — the delta path
// must too so downstream order-sensitive callers (rare but possible)
// don't silently shift.
func TestRemoveOneOfManyKeepsSurvivors(t *testing.T) {
	idx := BuildIndexFromData([]Symbol{
		{Name: "Shared", File: "a.kt", FQN: "p.a.Shared"},
		{Name: "Shared", File: "b.kt", FQN: "p.b.Shared"},
		{Name: "Shared", File: "c.kt", FQN: "p.c.Shared"},
	}, nil)
	BuildIndexIncremental(idx, map[string]bool{"b.kt": true}, nil, nil)
	got := idx.symbolsByName["Shared"]
	wantFiles := []string{"a.kt", "c.kt"}
	gotFiles := make([]string, len(got))
	for i, s := range got {
		gotFiles[i] = s.File
	}
	if !reflect.DeepEqual(gotFiles, wantFiles) {
		t.Errorf("survivor order/contents diverged: got %v, want %v", gotFiles, wantFiles)
	}
}

func equalSymbolsByName(a, b map[string][]Symbol) bool {
	if len(a) != len(b) {
		return false
	}
	for k, va := range a {
		vb, ok := b[k]
		if !ok || len(va) != len(vb) {
			return false
		}
		ac := append([]Symbol(nil), va...)
		bc := append([]Symbol(nil), vb...)
		sort.Slice(ac, func(i, j int) bool { return ac[i].File < ac[j].File })
		sort.Slice(bc, func(i, j int) bool { return bc[i].File < bc[j].File })
		if !reflect.DeepEqual(ac, bc) {
			return false
		}
	}
	return true
}
