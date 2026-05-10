package scan

import (
	"sync"
	"time"

	"github.com/kaeawc/krit/internal/cache"
)

// AnalysisCacheLoadFuture memoizes a background analysis-cache load
// kicked off as soon as cacheFilePath is known, in parallel with
// collectFiles / projectModel / filterRules. Mirrors the shape of
// android.ResourceScanFuture so call-sites have one familiar pattern
// for "start work in the background, await it later" — Start once,
// Await any number of times, Duration() reports the wall-time the
// load actually consumed (visible to perf, unlike the pipeline's
// cacheLoad tracker, which wraps the receive and reads ~0 ms on
// preloaded paths).
//
// A panic inside load is recovered and converted to a nil result so
// the pipeline's cacheLoad receiver (PreloadedAnalysisCache==nil
// branch) falls back to a synchronous cache.Load — losing the
// preload's parallelism but never deadlocking.
type AnalysisCacheLoadFuture struct {
	load func() *cache.Cache

	once sync.Once
	done chan struct{}

	cache *cache.Cache
	dur   time.Duration
}

// NewAnalysisCacheLoadFuture builds an unstarted future. Call Start
// to kick off the goroutine; Await blocks until it completes.
func NewAnalysisCacheLoadFuture(load func() *cache.Cache) *AnalysisCacheLoadFuture {
	return &AnalysisCacheLoadFuture{
		load: load,
		done: make(chan struct{}),
	}
}

// Start launches the background load. Idempotent: only the first
// call kicks off the goroutine. Safe to call from Await for
// fire-and-forget patterns where the caller doesn't want to track
// Start/Await separately.
func (f *AnalysisCacheLoadFuture) Start() {
	if f == nil {
		return
	}
	f.once.Do(func() {
		go func() {
			// Recover-into-nil: leave f.cache == nil on panic so Await
			// callers fall back to synchronous load. close(f.done)
			// always runs so receivers never deadlock.
			defer func() {
				_ = recover()
				close(f.done)
			}()
			start := time.Now()
			if f.load != nil {
				f.cache = f.load()
			}
			f.dur = time.Since(start)
		}()
	})
}

// Await blocks until the load goroutine completes and returns the
// cache (nil on panic or no-op). Auto-starts the future, so callers
// who only ever Await never miss the kickoff.
func (f *AnalysisCacheLoadFuture) Await() *cache.Cache {
	if f == nil {
		return nil
	}
	f.Start()
	<-f.done
	return f.cache
}

// Duration reports the wall-clock time the load goroutine spent.
// Zero before Start; final value after Await returns.
func (f *AnalysisCacheLoadFuture) Duration() time.Duration {
	if f == nil {
		return 0
	}
	f.Start()
	<-f.done
	return f.dur
}
