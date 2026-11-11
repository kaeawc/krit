package iterutil

import (
	"runtime"
	"sync"
)

// ForEachParallel runs fn for every item on a bounded pool of workers and waits for all of them.
func ForEachParallel[T any](items []T, fn func(T)) {
	workers := min(runtime.GOMAXPROCS(0), len(items))
	if workers <= 1 {
		for _, item := range items {
			fn(item)
		}
		return
	}
	var wg sync.WaitGroup
	next := make(chan T)
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range next {
				fn(item)
			}
		}()
	}
	for _, item := range items {
		next <- item
	}
	close(next)
	wg.Wait()
}
