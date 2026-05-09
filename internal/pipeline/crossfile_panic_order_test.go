package pipeline

import (
	"reflect"
	"testing"

	"github.com/kaeawc/krit/internal/rules"
)

// TestMergeSortedLocalErrs_StableAcrossWorkerPermutations asserts the
// helper produces the canonical sorted slice regardless of which
// worker contributed which DispatchError, mirroring the production
// shape where workers pull from a shared job channel and complete in
// a non-deterministic order. Regression for #29.
func TestMergeSortedLocalErrs_StableAcrossWorkerPermutations(t *testing.T) {
	wantCanonical := []rules.DispatchError{
		{FilePath: "/a.kt", Line: 1, RuleName: "rule-a", PanicValue: "x"},
		{FilePath: "/a.kt", Line: 7, RuleName: "rule-d", PanicValue: "x"},
		{FilePath: "/b.kt", Line: 2, RuleName: "rule-b", PanicValue: "y"},
		{FilePath: "/c.kt", Line: 3, RuleName: "rule-c", PanicValue: "z"},
		{FilePath: "/c.kt", Line: 9, RuleName: "rule-e", PanicValue: "z"},
	}

	// Each test case represents one possible worker-completion shape:
	// the same five errors split across 3 workers in different
	// allocations. All shapes must produce wantCanonical.
	cases := [][][]rules.DispatchError{
		{
			{wantCanonical[0], wantCanonical[3]},
			{wantCanonical[1]},
			{wantCanonical[2], wantCanonical[4]},
		},
		{
			{wantCanonical[4], wantCanonical[2]},
			{wantCanonical[3], wantCanonical[1], wantCanonical[0]},
			{},
		},
		{
			{},
			{wantCanonical[1], wantCanonical[4], wantCanonical[3]},
			{wantCanonical[2], wantCanonical[0]},
		},
		{
			{wantCanonical[0]},
			{wantCanonical[1]},
			{wantCanonical[2]},
			{wantCanonical[3]},
			{wantCanonical[4]},
		},
	}

	for i, c := range cases {
		got := mergeSortedLocalErrs(c)
		if !reflect.DeepEqual(got, wantCanonical) {
			t.Fatalf("case %d:\n  got:  %#v\n  want: %#v", i, got, wantCanonical)
		}
	}
}

func TestMergeSortedLocalErrs_EmptyAndNil(t *testing.T) {
	if got := mergeSortedLocalErrs(nil); got != nil {
		t.Fatalf("nil input: want nil, got %v", got)
	}
	if got := mergeSortedLocalErrs([][]rules.DispatchError{nil, {}, nil}); got != nil {
		t.Fatalf("all-empty input: want nil, got %v", got)
	}
}
