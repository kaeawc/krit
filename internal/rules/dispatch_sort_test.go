package rules

import (
	"reflect"
	"testing"
)

// TestSortDispatchErrors_StableTotalOrder asserts the comparator
// produces the same total ordering regardless of input slice
// permutation. Regression for #28: callers (dispatch, crossfile)
// rely on this helper to make panic-diagnostic output stable across
// runs.
func TestSortDispatchErrors_StableTotalOrder(t *testing.T) {
	canonical := []DispatchError{
		{FilePath: "/a.kt", Line: 10, RuleName: "rule-a", PanicValue: "boom"},
		{FilePath: "/a.kt", Line: 10, RuleName: "rule-b", PanicValue: "boom"},
		{FilePath: "/a.kt", Line: 20, RuleName: "rule-a", PanicValue: "boom"},
		{FilePath: "/b.kt", Line: 1, RuleName: "rule-c", PanicValue: "x"},
		{FilePath: "/b.kt", Line: 1, RuleName: "rule-c", PanicValue: "y"},
		{FilePath: "/c.kt", Line: 5, RuleName: "rule-z", PanicValue: nil},
	}

	// Try several permutations; all must produce the canonical order.
	permutations := [][]int{
		{5, 4, 3, 2, 1, 0},
		{0, 2, 1, 4, 3, 5},
		{3, 1, 5, 0, 2, 4},
		{2, 5, 0, 3, 1, 4},
	}
	for k, perm := range permutations {
		got := make([]DispatchError, len(perm))
		for i, p := range perm {
			got[i] = canonical[p]
		}
		SortDispatchErrors(got)
		if !reflect.DeepEqual(got, canonical) {
			t.Fatalf("perm %d: comparator did not yield canonical order\n  got:  %#v\n  want: %#v", k, got, canonical)
		}
	}
}

func TestSortDispatchErrors_EmptyAndSingleAreNoOps(t *testing.T) {
	SortDispatchErrors(nil)
	one := []DispatchError{{FilePath: "/x", Line: 1, RuleName: "r"}}
	SortDispatchErrors(one)
	if len(one) != 1 {
		t.Fatalf("expected len 1, got %d", len(one))
	}
}

// TestSortDispatchErrors_DistinguishesByPanicValue covers the final
// tiebreaker: identical (file, line, rule) but different panic
// payloads must not collide as "equal".
func TestSortDispatchErrors_DistinguishesByPanicValue(t *testing.T) {
	errs := []DispatchError{
		{FilePath: "/x.kt", Line: 1, RuleName: "r", PanicValue: "zzz"},
		{FilePath: "/x.kt", Line: 1, RuleName: "r", PanicValue: "aaa"},
	}
	SortDispatchErrors(errs)
	if errs[0].PanicValue != "aaa" {
		t.Fatalf("expected 'aaa' first, got %v", errs[0].PanicValue)
	}
}
