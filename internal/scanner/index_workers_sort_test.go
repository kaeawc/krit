package scanner

import (
	"math/rand"
	"reflect"
	"sort"
	"strconv"
	"testing"
)

func referenceSortRefs(refs []Reference) {
	sort.SliceStable(refs, func(i, j int) bool {
		a, b := refs[i], refs[j]
		if a.File != b.File {
			return a.File < b.File
		}
		if a.Line != b.Line {
			return a.Line < b.Line
		}
		return a.Name < b.Name
	})
}

func TestSortIndexReferencesMatchesStableSort(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for trial := 0; trial < 200; trial++ {
		interleave := trial%4 == 0
		var refs []Reference
		for f := 0; f < 1+rng.Intn(30); f++ {
			file := "f" + strconv.Itoa(rng.Intn(40))
			for k := 0; k < rng.Intn(12); k++ {
				refs = append(refs, Reference{
					Name: "n" + strconv.Itoa(rng.Intn(4)), File: file, Line: rng.Intn(5),
					StartByte: rng.Intn(1000), InComment: rng.Intn(2) == 0,
				})
			}
		}
		if interleave {
			rng.Shuffle(len(refs), func(i, j int) { refs[i], refs[j] = refs[j], refs[i] })
		}
		want := append([]Reference(nil), refs...)
		referenceSortRefs(want)
		got := append([]Reference(nil), refs...)
		SortIndexReferences(got)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("trial %d: order differs from stable sort", trial)
		}
	}
}

func TestSortIndexSymbolsMatchesStableSort(t *testing.T) {
	rng := rand.New(rand.NewSource(2))
	for trial := 0; trial < 200; trial++ {
		var syms []Symbol
		for f := 0; f < 1+rng.Intn(30); f++ {
			file := "f" + strconv.Itoa(rng.Intn(40))
			for k := 0; k < rng.Intn(12); k++ {
				syms = append(syms, Symbol{
					Name: "n" + strconv.Itoa(rng.Intn(3)), FQN: "p" + strconv.Itoa(rng.Intn(3)),
					File: file, StartByte: rng.Intn(4), Line: rng.Intn(100),
				})
			}
		}
		if trial%4 == 0 {
			rng.Shuffle(len(syms), func(i, j int) { syms[i], syms[j] = syms[j], syms[i] })
		}
		want := append([]Symbol(nil), syms...)
		sort.SliceStable(want, func(i, j int) bool {
			a, b := want[i], want[j]
			if a.File != b.File {
				return a.File < b.File
			}
			if a.StartByte != b.StartByte {
				return a.StartByte < b.StartByte
			}
			if a.FQN != b.FQN {
				return a.FQN < b.FQN
			}
			return a.Name < b.Name
		})
		got := append([]Symbol(nil), syms...)
		SortIndexSymbols(got)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("trial %d: order differs from stable sort", trial)
		}
	}
}
