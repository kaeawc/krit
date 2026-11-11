package iterutil

import (
	"strconv"
	"sync/atomic"
	"testing"
)

func TestForEachParallelVisitsEveryItemOnce(t *testing.T) {
	items := make([]string, 500)
	index := map[string]int{}
	for i := range items {
		items[i] = "item" + strconv.Itoa(i)
		index[items[i]] = i
	}
	var count atomic.Int64
	seen := make([]atomic.Int32, len(items))
	ForEachParallel(items, func(item string) {
		count.Add(1)
		seen[index[item]].Add(1)
	})
	if int(count.Load()) != len(items) {
		t.Fatalf("visited %d items, want %d", count.Load(), len(items))
	}
	for i := range seen {
		if seen[i].Load() != 1 {
			t.Fatalf("item %d visited %d times", i, seen[i].Load())
		}
	}
}

func TestForEachParallelHandlesEmptyAndSingle(t *testing.T) {
	ForEachParallel(nil, func(string) { t.Fatal("no item expected") })
	var n int
	ForEachParallel([]string{"only"}, func(string) { n++ })
	if n != 1 {
		t.Fatalf("single item visited %d times", n)
	}
}
