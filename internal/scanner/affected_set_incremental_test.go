package scanner

import (
	"sort"
	"testing"
)

func affectedSetIncrementalSorted(current *CodeIndex, changed []string, removed *PriorRemovedContributions) []string {
	got := AffectedSetIncremental(current, changed, removed)
	sort.Strings(got)
	return got
}

func containsName(names []string, want string) bool {
	for _, n := range names {
		if n == want {
			return true
		}
	}
	return false
}

// TestAffectedSetIncremental_CurrentDeclarationToReferrers pins direction 1
// against the current index: a changed file's declaration affects every file
// that references it.
func TestAffectedSetIncremental_CurrentDeclarationToReferrers(t *testing.T) {
	syms := []Symbol{{Name: "S", File: "b.kt", FQN: "demo.S"}}
	refs := []Reference{
		{Name: "S", File: "a.kt"},
		{Name: "S", File: "c.kt"},
		{Name: "Unrelated", File: "d.kt"},
	}
	current := BuildIndexFromData(syms, refs)

	got := affectedSetIncrementalSorted(current, []string{"b.kt"}, nil)
	for _, want := range []string{"a.kt", "b.kt", "c.kt"} {
		if !hasFile(got, want) {
			t.Errorf("affected must include %q (current declaration->referrers); got %v", want, got)
		}
	}
	if hasFile(got, "d.kt") {
		t.Errorf("unrelated file d.kt must not be affected; got %v", got)
	}
}

// TestAffectedSetIncremental_CurrentReferenceToDeclarers pins direction 2: a
// changed file's reference affects the file that declares the name.
func TestAffectedSetIncremental_CurrentReferenceToDeclarers(t *testing.T) {
	syms := []Symbol{{Name: "S", File: "a.kt", FQN: "demo.S"}}
	refs := []Reference{{Name: "S", File: "b.kt"}}
	current := BuildIndexFromData(syms, refs)

	got := affectedSetIncrementalSorted(current, []string{"b.kt"}, nil)
	if !hasFile(got, "a.kt") {
		t.Errorf("affected must include declaring file a.kt (current reference->declarers); got %v", got)
	}
}

// TestAffectedSetIncremental_RemovedReferenceUsesRemoved is the critical #608
// case on the daemon path: the edit deleted B's only reference to S (declared
// in A). Current no longer links B to S — only the captured removed
// contributions reveal that A's "S is unused" finding could flip. The removed
// name is resolved against current, whose declaring endpoint (A) survives.
func TestAffectedSetIncremental_RemovedReferenceUsesRemoved(t *testing.T) {
	// current: a.kt still declares S; b.kt no longer references it.
	current := BuildIndexFromData(
		[]Symbol{{Name: "S", File: "a.kt", FQN: "demo.S"}},
		nil,
	)
	removed := &PriorRemovedContributions{
		Referenced: map[string][]string{"b.kt": {"S"}},
	}

	got := affectedSetIncrementalSorted(current, []string{"b.kt"}, removed)
	if !hasFile(got, "a.kt") {
		t.Errorf("removed reference must still affect the declaring file via removed; got %v", got)
	}

	// Precondition: without the removed contributions, current-only misses a.kt.
	currentOnly := AffectedSetIncremental(current, []string{"b.kt"}, nil)
	if hasFile(currentOnly, "a.kt") {
		t.Errorf("precondition: current-only should not reach a.kt; got %v", currentOnly)
	}
}

// TestAffectedSetIncremental_RemovedDeclarationUsesRemoved is the mirror case:
// the edit deleted B's declaration of S that A referenced. The removed
// declaration name resolves against current's surviving referrer (A).
func TestAffectedSetIncremental_RemovedDeclarationUsesRemoved(t *testing.T) {
	// current: b.kt no longer declares S; a.kt's now-dangling reference remains.
	current := BuildIndexFromData(
		nil,
		[]Reference{{Name: "S", File: "a.kt"}},
	)
	removed := &PriorRemovedContributions{
		Declared: map[string][]string{"b.kt": {"S"}},
	}

	got := affectedSetIncrementalSorted(current, []string{"b.kt"}, removed)
	if !hasFile(got, "a.kt") {
		t.Errorf("removed declaration must still affect the referencing file via removed; got %v", got)
	}
}

// TestAffectedSetIncremental_DeletedFile covers a fully deleted changed file:
// it has no current declarations or references, so every dependent is reached
// only through the captured removed contributions. A deleted file that both
// declared P (referenced by p.kt) and referenced Q (declared in q.kt) must
// affect both p.kt and q.kt.
func TestAffectedSetIncremental_DeletedFile(t *testing.T) {
	// current: gone.kt is deleted; p.kt's dangling reference to P and q.kt's
	// declaration of Q survive.
	current := BuildIndexFromData(
		[]Symbol{{Name: "Q", File: "q.kt", FQN: "demo.Q"}},
		[]Reference{{Name: "P", File: "p.kt"}},
	)
	removed := &PriorRemovedContributions{
		Declared:   map[string][]string{"gone.kt": {"P"}},
		Referenced: map[string][]string{"gone.kt": {"Q"}},
	}

	got := affectedSetIncrementalSorted(current, []string{"gone.kt"}, removed)
	if !hasFile(got, "p.kt") {
		t.Errorf("deleted file's removed declaration must affect referrer p.kt; got %v", got)
	}
	if !hasFile(got, "q.kt") {
		t.Errorf("deleted file's removed reference must affect declarer q.kt; got %v", got)
	}
}

// TestAffectedSetIncremental_FQNMatch verifies an FQN-spelled reference still
// links to the declaring file.
func TestAffectedSetIncremental_FQNMatch(t *testing.T) {
	syms := []Symbol{{Name: "S", File: "b.kt", FQN: "demo.S"}}
	refs := []Reference{{Name: "demo.S", File: "a.kt"}}
	current := BuildIndexFromData(syms, refs)

	got := affectedSetIncrementalSorted(current, []string{"b.kt"}, nil)
	if !hasFile(got, "a.kt") {
		t.Errorf("FQN-spelled reference must be linked to declaration; got %v", got)
	}
}

// TestAffectedSetIncremental_EdgeCases pins the degenerate inputs.
func TestAffectedSetIncremental_EdgeCases(t *testing.T) {
	idx := BuildIndexFromData([]Symbol{{Name: "S", File: "a.kt"}}, nil)

	if got := AffectedSetIncremental(idx, nil, nil); got != nil {
		t.Errorf("nil changedFiles must return nil; got %v", got)
	}
	if got := AffectedSetIncremental(idx, []string{""}, nil); got != nil {
		t.Errorf("empty-string-only changedFiles must return nil; got %v", got)
	}
	if got := AffectedSetIncremental(nil, []string{"x.kt"}, nil); !hasFile(got, "x.kt") || len(got) != 1 {
		t.Errorf("nil current must still echo the changed file; got %v", got)
	}
}

// TestAffectedSetIncremental_AlwaysIncludesChanged guarantees the changed files
// are always present even when they neither declare nor reference anything.
func TestAffectedSetIncremental_AlwaysIncludesChanged(t *testing.T) {
	idx := BuildIndexFromData([]Symbol{{Name: "Other", File: "z.kt"}}, nil)
	got := affectedSetIncrementalSorted(idx, []string{"new.kt", "new2.kt"}, nil)
	for _, want := range []string{"new.kt", "new2.kt"} {
		if !hasFile(got, want) {
			t.Errorf("changed file %q must always be present; got %v", want, got)
		}
	}
}

// TestSnapshotRemovedContributions_CapturesDeclaredAndReferenced verifies the
// snapshot records both declared names (bare + FQN) and referenced names for
// exactly the requested files, and ignores files outside the set.
func TestSnapshotRemovedContributions_CapturesDeclaredAndReferenced(t *testing.T) {
	idx := BuildIndexFromData(
		[]Symbol{
			{Name: "S", File: "b.kt", FQN: "demo.S"},
			{Name: "T", File: "other.kt", FQN: "demo.T"},
		},
		[]Reference{
			{Name: "X", File: "b.kt"},
			{Name: "Y", File: "b.kt"},
			{Name: "Z", File: "other.kt"},
		},
	)

	got := idx.SnapshotRemovedContributions(map[string]bool{"b.kt": true})

	if !containsName(got.Declared["b.kt"], "S") {
		t.Errorf("declared must include bare name S; got %v", got.Declared["b.kt"])
	}
	if !containsName(got.Declared["b.kt"], "demo.S") {
		t.Errorf("declared must include FQN demo.S; got %v", got.Declared["b.kt"])
	}
	if _, ok := got.Declared["other.kt"]; ok {
		t.Errorf("declared must not include files outside the set; got %v", got.Declared)
	}
	refs := got.Referenced["b.kt"]
	if !containsName(refs, "X") || !containsName(refs, "Y") {
		t.Errorf("referenced must include X and Y; got %v", refs)
	}
	if containsName(refs, "Z") {
		t.Errorf("referenced must not include other.kt's Z; got %v", refs)
	}
	if _, ok := got.Referenced["other.kt"]; ok {
		t.Errorf("referenced must not include files outside the set; got %v", got.Referenced)
	}
}

// TestSnapshotRemovedContributions_NilSafety covers the degenerate receivers.
func TestSnapshotRemovedContributions_NilSafety(t *testing.T) {
	var nilIdx *CodeIndex
	got := nilIdx.SnapshotRemovedContributions(map[string]bool{"b.kt": true})
	if len(got.Declared) != 0 || len(got.Referenced) != 0 {
		t.Errorf("nil index must produce empty contributions; got %+v", got)
	}

	idx := BuildIndexFromData([]Symbol{{Name: "S", File: "b.kt"}}, nil)
	empty := idx.SnapshotRemovedContributions(nil)
	if len(empty.Declared) != 0 || len(empty.Referenced) != 0 {
		t.Errorf("empty file set must produce empty contributions; got %+v", empty)
	}
}

// TestLastRemovedContributions_RoundTrip verifies setLastRemoved stores a copy
// that LastRemovedContributions returns, and that a fresh index reports nil.
func TestLastRemovedContributions_RoundTrip(t *testing.T) {
	idx := BuildIndexFromData([]Symbol{{Name: "S", File: "a.kt"}}, nil)
	if idx.LastRemovedContributions() != nil {
		t.Errorf("fresh index must report nil removed contributions")
	}

	snap := idx.SnapshotRemovedContributions(map[string]bool{"a.kt": true})
	idx.setLastRemoved(snap)
	got := idx.LastRemovedContributions()
	if got == nil {
		t.Fatalf("LastRemovedContributions must return the stored snapshot")
	}
	if !containsName(got.Declared["a.kt"], "S") {
		t.Errorf("stored snapshot must include declared S; got %v", got.Declared["a.kt"])
	}

	var nilIdx *CodeIndex
	if nilIdx.LastRemovedContributions() != nil {
		t.Errorf("nil index must report nil removed contributions")
	}
}
