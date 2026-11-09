package scanner

import (
	"fmt"
	"sync"
	"testing"
)

func TestInternNodeTypeConcurrentLookup(t *testing.T) {
	const workers = 8
	const perWorker = 50
	var wg sync.WaitGroup
	ids := make([][]uint16, workers)
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < perWorker; i++ {
				ids[w] = append(ids[w], internNodeType(fmt.Sprintf("concurrent_type_%d", i)))
			}
		}(w)
	}
	wg.Wait()
	for i := 0; i < perWorker; i++ {
		name := fmt.Sprintf("concurrent_type_%d", i)
		got, ok := LookupFlatNodeType(name)
		if !ok {
			t.Fatalf("%s not found", name)
		}
		if nodeTypeName(got) != name {
			t.Fatalf("name mismatch for %s", name)
		}
		for w := 0; w < workers; w++ {
			if ids[w][i] != got {
				t.Fatalf("worker %d id for %s = %d, want %d", w, name, ids[w][i], got)
			}
		}
	}
	if _, ok := LookupFlatNodeType("never_interned_type"); ok {
		t.Fatal("unexpected hit")
	}
}
