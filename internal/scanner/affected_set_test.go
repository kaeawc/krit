package scanner

import (
	"sort"
	"testing"
)

func affectedSetSorted(prior, current *CodeIndex, changed []string) []string {
	got := AffectedSet(prior, current, changed)
	sort.Strings(got)
	return got
}

func hasFile(set []string, f string) bool {
	for _, s := range set {
		if s == f {
			return true
		}
	}
	return false
}

// TestAffectedSet_DeclarationToReferrers pins direction 1: when a changed file
// declares a name, every file that references that name is affected.
func TestAffectedSet_DeclarationToReferrers(t *testing.T) {
	// b.kt declares S; a.kt and c.kt reference S; d.kt is unrelated.
	syms := []Symbol{{Name: "S", File: "b.kt", FQN: "demo.S"}}
	refs := []Reference{
		{Name: "S", File: "a.kt"},
		{Name: "S", File: "c.kt"},
		{Name: "Unrelated", File: "d.kt"},
	}
	idx := BuildIndexFromData(syms, refs)

	got := affectedSetSorted(idx, idx, []string{"b.kt"})
	for _, want := range []string{"a.kt", "b.kt", "c.kt"} {
		if !hasFile(got, want) {
			t.Errorf("affected must include %q (declaration->referrers); got %v", want, got)
		}
	}
	if hasFile(got, "d.kt") {
		t.Errorf("unrelated file d.kt must not be affected; got %v", got)
	}
}

// TestAffectedSet_ReferenceToDeclarers pins direction 2: when a changed file
// references a name, every file that declares that name is affected. This is
// the unused-declaration-flip case — the declaring file's finding can change
// even though it was not itself edited.
func TestAffectedSet_ReferenceToDeclarers(t *testing.T) {
	// a.kt declares S; b.kt (changed) references S.
	syms := []Symbol{{Name: "S", File: "a.kt", FQN: "demo.S"}}
	refs := []Reference{{Name: "S", File: "b.kt"}}
	idx := BuildIndexFromData(syms, refs)

	got := affectedSetSorted(idx, idx, []string{"b.kt"})
	if !hasFile(got, "a.kt") {
		t.Errorf("affected must include declaring file a.kt (reference->declarers); got %v", got)
	}
}

// TestAffectedSet_RemovedReferenceUsesPrior is the critical #608 case: the
// changed file B *removed* its only reference to S (declared in A). The current
// index no longer links B to S, so only the PRIOR index reveals that A's
// finding (e.g. "S is unused") could flip. AffectedSet must consult prior.
func TestAffectedSet_RemovedReferenceUsesPrior(t *testing.T) {
	// prior: a.kt declares S, b.kt references S.
	prior := BuildIndexFromData(
		[]Symbol{{Name: "S", File: "a.kt", FQN: "demo.S"}},
		[]Reference{{Name: "S", File: "b.kt"}},
	)
	// current: a.kt still declares S, b.kt no longer references it.
	current := BuildIndexFromData(
		[]Symbol{{Name: "S", File: "a.kt", FQN: "demo.S"}},
		nil,
	)

	got := affectedSetSorted(prior, current, []string{"b.kt"})
	if !hasFile(got, "a.kt") {
		t.Errorf("removed reference must still affect the declaring file via prior; got %v", got)
	}

	// Sanity: current-only would MISS a.kt, demonstrating why prior is required.
	currentOnly := AffectedSet(nil, current, []string{"b.kt"})
	if hasFile(currentOnly, "a.kt") {
		t.Errorf("precondition: current-only should not reach a.kt; got %v", currentOnly)
	}
}

// TestAffectedSet_RemovedDeclarationUsesPrior covers the mirror case: the
// changed file B *removed* a declaration of S that A referenced. Only prior
// links B's declaration to A, so prior must be consulted.
func TestAffectedSet_RemovedDeclarationUsesPrior(t *testing.T) {
	// prior: b.kt declares S, a.kt references S.
	prior := BuildIndexFromData(
		[]Symbol{{Name: "S", File: "b.kt", FQN: "demo.S"}},
		[]Reference{{Name: "S", File: "a.kt"}},
	)
	// current: b.kt no longer declares S; a.kt's (now dangling) reference remains.
	current := BuildIndexFromData(
		nil,
		[]Reference{{Name: "S", File: "a.kt"}},
	)

	got := affectedSetSorted(prior, current, []string{"b.kt"})
	if !hasFile(got, "a.kt") {
		t.Errorf("removed declaration must still affect the referencing file via prior; got %v", got)
	}
}

// TestAffectedSet_AddedReferenceUsesCurrent confirms the current index covers
// the add case symmetrically: B added a reference to S declared in A.
func TestAffectedSet_AddedReferenceUsesCurrent(t *testing.T) {
	prior := BuildIndexFromData(
		[]Symbol{{Name: "S", File: "a.kt", FQN: "demo.S"}},
		nil,
	)
	current := BuildIndexFromData(
		[]Symbol{{Name: "S", File: "a.kt", FQN: "demo.S"}},
		[]Reference{{Name: "S", File: "b.kt"}},
	)
	got := affectedSetSorted(prior, current, []string{"b.kt"})
	if !hasFile(got, "a.kt") {
		t.Errorf("added reference must affect the declaring file via current; got %v", got)
	}
}

// TestAffectedSet_FQNMatch verifies a reference written as the FQN still links
// to the declaring file, since declsByFile records both the bare name and FQN.
func TestAffectedSet_FQNMatch(t *testing.T) {
	syms := []Symbol{{Name: "S", File: "b.kt", FQN: "demo.S"}}
	refs := []Reference{{Name: "demo.S", File: "a.kt"}}
	idx := BuildIndexFromData(syms, refs)

	got := affectedSetSorted(idx, idx, []string{"b.kt"})
	if !hasFile(got, "a.kt") {
		t.Errorf("FQN-spelled reference must be linked to declaration; got %v", got)
	}
}

// TestAffectedSet_EdgeCases pins the degenerate inputs.
func TestAffectedSet_EdgeCases(t *testing.T) {
	idx := BuildIndexFromData([]Symbol{{Name: "S", File: "a.kt"}}, nil)

	if got := AffectedSet(idx, idx, nil); got != nil {
		t.Errorf("nil changedFiles must return nil; got %v", got)
	}
	if got := AffectedSet(idx, idx, []string{""}); got != nil {
		t.Errorf("empty-string-only changedFiles must return nil; got %v", got)
	}
	if got := AffectedSet(nil, nil, []string{"x.kt"}); !hasFile(got, "x.kt") || len(got) != 1 {
		t.Errorf("nil indexes must still echo the changed file; got %v", got)
	}
}

// TestAffectedSet_AlwaysIncludesChanged guarantees the changed files are always
// in the result, even when they neither declare nor reference anything indexed.
func TestAffectedSet_AlwaysIncludesChanged(t *testing.T) {
	idx := BuildIndexFromData([]Symbol{{Name: "Other", File: "z.kt"}}, nil)
	got := affectedSetSorted(idx, idx, []string{"new.kt", "new2.kt"})
	for _, want := range []string{"new.kt", "new2.kt"} {
		if !hasFile(got, want) {
			t.Errorf("changed file %q must always be present; got %v", want, got)
		}
	}
}
