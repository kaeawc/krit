package iterutil

import (
	"reflect"
	"testing"
)

func TestSortedKeys_StableAcrossRuns(t *testing.T) {
	m := map[string]int{
		"foo": 1, "bar": 2, "baz": 3, "qux": 4, "zzz": 5,
		"aaa": 6, "mmm": 7, "ccc": 8, "ddd": 9, "eee": 10,
	}
	want := []string{"aaa", "bar", "baz", "ccc", "ddd", "eee", "foo", "mmm", "qux", "zzz"}

	// Run many iterations to amplify any scheduler-driven non-determinism.
	for i := 0; i < 200; i++ {
		got := SortedKeys(m)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("iteration %d: got %v, want %v", i, got, want)
		}
	}
}

func TestSortedKeys_EmptyMap(t *testing.T) {
	if got := SortedKeys(map[string]int{}); got != nil {
		t.Fatalf("empty map: want nil, got %v", got)
	}
	var nilMap map[string]int
	if got := SortedKeys(nilMap); got != nil {
		t.Fatalf("nil map: want nil, got %v", got)
	}
}

func TestSortedKeys_IntKeys(t *testing.T) {
	m := map[int]string{3: "c", 1: "a", 2: "b"}
	got := SortedKeys(m)
	want := []int{1, 2, 3}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestForEachSorted_VisitOrder(t *testing.T) {
	m := map[string]int{"c": 3, "a": 1, "b": 2}
	for i := 0; i < 100; i++ {
		var keys []string
		var vals []int
		ForEachSorted(m, func(k string, v int) {
			keys = append(keys, k)
			vals = append(vals, v)
		})
		if !reflect.DeepEqual(keys, []string{"a", "b", "c"}) {
			t.Fatalf("iter %d: keys = %v", i, keys)
		}
		if !reflect.DeepEqual(vals, []int{1, 2, 3}) {
			t.Fatalf("iter %d: vals = %v", i, vals)
		}
	}
}

func TestForEachSorted_EmptyMapNoOp(t *testing.T) {
	called := false
	ForEachSorted(map[string]int{}, func(string, int) { called = true })
	if called {
		t.Fatal("ForEachSorted invoked fn on empty map")
	}
}
