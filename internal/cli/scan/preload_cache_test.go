package scan

import (
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/cache"
)

func TestPreloadAnalysisCacheReturnsLoadResult(t *testing.T) {
	want := &cache.Cache{}
	ch := preloadAnalysisCache(func() *cache.Cache { return want })
	select {
	case got := <-ch:
		if got != want {
			t.Fatalf("got %p; want %p", got, want)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for preload result")
	}
}

func TestPreloadAnalysisCacheRecoversPanic(t *testing.T) {
	ch := preloadAnalysisCache(func() *cache.Cache { panic("boom") })
	select {
	case got := <-ch:
		if got != nil {
			t.Fatalf("got %p; want nil after panic", got)
		}
	case <-time.After(time.Second):
		t.Fatal("preload deadlocked after panic — recover did not fire")
	}
}
