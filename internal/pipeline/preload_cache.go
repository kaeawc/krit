package pipeline

import (
	"sync"
	"time"

	"github.com/kaeawc/krit/internal/cache"
)

// AnalysisCacheLoadFuture memoizes a background analysis-cache load
// kicked off before IndexPhase needs it. A panic inside load is
// recovered and converted to nil so callers can fall back to a
// synchronous load without deadlocking.
type AnalysisCacheLoadFuture struct {
	load func() *cache.Cache

	once sync.Once
	done chan struct{}

	cache *cache.Cache
	dur   time.Duration
}

func NewAnalysisCacheLoadFuture(load func() *cache.Cache) *AnalysisCacheLoadFuture {
	return &AnalysisCacheLoadFuture{
		load: load,
		done: make(chan struct{}),
	}
}

func (f *AnalysisCacheLoadFuture) Start() {
	if f == nil {
		return
	}
	f.once.Do(func() {
		go func() {
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

func (f *AnalysisCacheLoadFuture) Await() *cache.Cache {
	if f == nil {
		return nil
	}
	f.Start()
	<-f.done
	return f.cache
}

func (f *AnalysisCacheLoadFuture) Duration() time.Duration {
	if f == nil {
		return 0
	}
	f.Start()
	<-f.done
	return f.dur
}
