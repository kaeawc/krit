package snapshot

import (
	"sync/atomic"
	"testing"
)

func TestGitSemaphoreCapacity(t *testing.T) {
	cases := []struct{ workers, want int }{
		{1, 1},
		{2, 1},
		{3, 1},
		{4, 2},
		{8, 4},
		{9, 4},
	}
	for _, tc := range cases {
		got := cap(gitSemaphore(tc.workers))
		if got != tc.want {
			t.Errorf("gitSemaphore(%d) cap = %d, want %d", tc.workers, got, tc.want)
		}
	}
}

// TestGitSemaphoreLimitsParallelism verifies that concurrent git ops are
// bounded to cap(sem) by tracking the peak observed concurrency.
func TestGitSemaphoreLimitsParallelism(t *testing.T) {
	const workers = 8
	sem := gitSemaphore(workers) // cap = 4

	var active, peak int64
	done := make(chan struct{})

	start := func() {
		sem <- struct{}{}
		n := atomic.AddInt64(&active, 1)
		for {
			cur := atomic.LoadInt64(&peak)
			if n <= cur || atomic.CompareAndSwapInt64(&peak, cur, n) {
				break
			}
		}
	}
	end := func() {
		atomic.AddInt64(&active, -1)
		<-sem
	}

	for range workers * 2 {
		go func() {
			start()
			end()
			done <- struct{}{}
		}()
	}
	for range workers * 2 {
		<-done
	}

	if p := atomic.LoadInt64(&peak); p > int64(cap(sem)) {
		t.Errorf("peak concurrent git ops = %d, want <= %d", p, cap(sem))
	}
}
