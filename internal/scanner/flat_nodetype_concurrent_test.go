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

// A reader that sees an ID through the lock-free index must also be able to
// resolve its name, so the name table has to be published first.
func TestInternNodeTypeIDResolvesWhileInterning(t *testing.T) {
	const names = 2000
	var wg sync.WaitGroup
	done := make(chan struct{})
	errs := make(chan string, 1)
	for r := 0; r < 4; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				default:
				}
				for i := names - 1; i >= 0; i-- {
					name := fmt.Sprintf("publish_order_type_%d", i)
					id, ok := LookupFlatNodeType(name)
					if !ok {
						continue
					}
					if got := nodeTypeName(id); got != name {
						select {
						case errs <- fmt.Sprintf("id %d for %s resolved to %q", id, name, got):
						default:
						}
						return
					}
				}
			}
		}()
	}
	for i := 0; i < names; i++ {
		internNodeType(fmt.Sprintf("publish_order_type_%d", i))
	}
	close(done)
	wg.Wait()
	select {
	case msg := <-errs:
		t.Fatal(msg)
	default:
	}
}
