package scan

import "github.com/kaeawc/krit/internal/cache"

// preloadAnalysisCache runs load in a background goroutine and returns a
// buffered channel that yields its result. A panic inside load is
// recovered and converted to a nil send so the pipeline's cacheLoad
// receiver never deadlocks; nil is the same shape that
// pipeline.IndexPhase already treats as "fall back to a synchronous
// cache.Load" via the PreloadedAnalysisCache==nil branch.
func preloadAnalysisCache(load func() *cache.Cache) <-chan *cache.Cache {
	ch := make(chan *cache.Cache, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				ch <- nil
			}
		}()
		ch <- load()
	}()
	return ch
}
