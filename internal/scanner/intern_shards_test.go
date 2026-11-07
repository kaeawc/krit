package scanner

import (
	"fmt"
	"sync"
	"testing"
	"unsafe"
)

func TestStringPoolReturnsOneCanonicalCopyAcrossConcurrentCallers(t *testing.T) {
	pool := NewStringPool()
	const names, workers = 500, 16
	results := make([][]string, workers)
	var wg sync.WaitGroup
	for w := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			out := make([]string, names)
			for i := range names {
				// A fresh buffer per call, as callers pass views over file bytes.
				out[i] = pool.Intern(string([]byte(fmt.Sprintf("name-%d", i))))
			}
			results[w] = out
		}()
	}
	wg.Wait()
	for i := range names {
		want := fmt.Sprintf("name-%d", i)
		first := results[0][i]
		if first != want {
			t.Fatalf("name %d interned as %q", i, first)
		}
		for w := 1; w < workers; w++ {
			if unsafe.StringData(results[w][i]) != unsafe.StringData(first) {
				t.Fatalf("name %d has two canonical copies", i)
			}
		}
	}
	if got := pool.Intern(""); got != "" {
		t.Fatalf("empty string interned as %q", got)
	}
}
