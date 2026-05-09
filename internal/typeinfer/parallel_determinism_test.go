package typeinfer

import (
	"testing"
)

// TestMergeFileResults_StableUnderFileOrderShuffle asserts that the
// resolver's class table is identical regardless of the order in
// which the parallel extraction step delivered FileTypeInfo entries.
// Regression for #35: previously `r.classes[ci.Name]` was decided by
// last-in-results; that ordering tracked the caller's `files` slice,
// so any caller passing files via map iteration (cf. permodule.go)
// would yield different short-name resolution per run.
func TestMergeFileResults_StableUnderFileOrderShuffle(t *testing.T) {
	makeFI := func(path, fqn string) *FileTypeInfo {
		return &FileTypeInfo{
			Path:    path,
			Classes: []*ClassInfo{{Name: "Foo", FQN: fqn}},
		}
	}

	// Two files, both declaring a short-named "Foo" with different
	// FQNs. The merge contract: lex-largest (path, FQN) wins, and the
	// winner must be the same across all input permutations.
	a := makeFI("/src/main/kotlin/a/Foo.kt", "a.Foo")
	b := makeFI("/src/main/kotlin/b/Foo.kt", "b.Foo")
	c := makeFI("/src/main/kotlin/c/Foo.kt", "c.Foo")

	permutations := [][]*FileTypeInfo{
		{a, b, c},
		{c, b, a},
		{b, c, a},
		{a, c, b},
		{c, a, b},
	}

	var refFQN string
	for k, perm := range permutations {
		r := NewResolver()
		r.mergeFileResults(perm)
		got := r.classes["Foo"]
		if got == nil {
			t.Fatalf("perm %d: classes[Foo] is nil", k)
		}
		if k == 0 {
			refFQN = got.FQN
			continue
		}
		if got.FQN != refFQN {
			t.Fatalf("perm %d: classes[Foo] = %s, want %s (input order should not affect result)", k, got.FQN, refFQN)
		}
	}

	// Independent witness: documented contract is "lex-largest path
	// wins" — verify so the property is pinned.
	if refFQN != "c.Foo" {
		t.Fatalf("expected lex-largest path winner c.Foo, got %s", refFQN)
	}
}

// TestMergeFileResults_NilEntriesIgnored confirms results with nil
// slots (failed/skipped extractions) don't disturb the merge.
func TestMergeFileResults_NilEntriesIgnored(t *testing.T) {
	a := &FileTypeInfo{Path: "/a.kt", Classes: []*ClassInfo{{Name: "X", FQN: "p.X"}}}
	for _, perm := range [][]*FileTypeInfo{
		{nil, a, nil},
		{a, nil, nil},
		{nil, nil, a},
	} {
		r := NewResolver()
		r.mergeFileResults(perm)
		if r.classes["X"] == nil || r.classes["X"].FQN != "p.X" {
			t.Fatalf("perm %v: expected X at p.X", perm)
		}
	}
}

// TestMergeFileResults_SortedClassesWithinFile ensures intra-file
// class order is canonical, so a file containing two classes with
// identical short names but different FQNs has a deterministic
// last-write winner under the existing semantics.
func TestMergeFileResults_SortedClassesWithinFile(t *testing.T) {
	// One file, two classes with the same short name "Foo" (rare but
	// possible via inner classes / nested objects — and exercises the
	// comparator). Test in two input orders.
	makeFI := func(orderA bool) *FileTypeInfo {
		fi := &FileTypeInfo{Path: "/x.kt"}
		if orderA {
			fi.Classes = []*ClassInfo{
				{Name: "Foo", FQN: "p.Outer.Foo"},
				{Name: "Foo", FQN: "p.Inner.Foo"},
			}
		} else {
			fi.Classes = []*ClassInfo{
				{Name: "Foo", FQN: "p.Inner.Foo"},
				{Name: "Foo", FQN: "p.Outer.Foo"},
			}
		}
		return fi
	}

	for _, orderA := range []bool{true, false} {
		r := NewResolver()
		r.mergeFileResults([]*FileTypeInfo{makeFI(orderA)})
		got := r.classes["Foo"]
		// FQN-asc sort + last-write-wins ⇒ FQN-largest wins.
		if got == nil || got.FQN != "p.Outer.Foo" {
			t.Fatalf("orderA=%v: got %v, want FQN p.Outer.Foo", orderA, got)
		}
	}
}
