package scanner

import (
	"fmt"
	"math/rand/v2"
	"reflect"
	"testing"
)

func TestSortIndexReferencesParallelRunsMatchStableSort(t *testing.T) {
	rng := rand.New(rand.NewPCG(3, 5))
	for _, files := range []int{1, 10, 500, 2000} {
		// One contiguous run per file, in random file order, with
		// unsorted, duplicate-heavy contents inside each run.
		perm := rng.Perm(files)
		var refs []Reference
		for _, f := range perm {
			for n := rng.IntN(12); n >= 0; n-- {
				refs = append(refs, Reference{
					Name:      fmt.Sprintf("n%d", rng.IntN(4)),
					File:      fmt.Sprintf("f%05d.kt", f),
					Line:      rng.IntN(6),
					InComment: rng.IntN(2) == 0,
				})
			}
		}
		want := append([]Reference(nil), refs...)
		referenceSortRefs(want)
		SortIndexReferences(refs)
		if !reflect.DeepEqual(refs, want) {
			t.Fatalf("%d files: parallel run sort differs from stable sort", files)
		}
	}
}
