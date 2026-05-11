package pipeline

import (
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/cache"
)

func TestAnalysisCacheLoadFuture_ReturnsLoadResult(t *testing.T) {
	want := &cache.Cache{}
	f := NewAnalysisCacheLoadFuture(func() *cache.Cache { return want })
	f.Start()
	got := f.Await()
	if got != want {
		t.Fatalf("got %p; want %p", got, want)
	}
}

func TestAnalysisCacheLoadFuture_RecoversPanic(t *testing.T) {
	f := NewAnalysisCacheLoadFuture(func() *cache.Cache { panic("boom") })
	f.Start()
	done := make(chan struct{})
	go func() {
		defer close(done)
		if got := f.Await(); got != nil {
			t.Errorf("got %p; want nil after panic", got)
		}
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Await deadlocked after panic")
	}
}

func TestAnalysisCacheLoadFuture_AwaitAutoStarts(t *testing.T) {
	want := &cache.Cache{}
	f := NewAnalysisCacheLoadFuture(func() *cache.Cache { return want })
	got := f.Await()
	if got != want {
		t.Fatalf("got %p; want %p", got, want)
	}
}

func TestAnalysisCacheLoadFuture_AwaitIsRepeatable(t *testing.T) {
	calls := 0
	f := NewAnalysisCacheLoadFuture(func() *cache.Cache {
		calls++
		return &cache.Cache{}
	})
	first := f.Await()
	second := f.Await()
	if first != second {
		t.Errorf("repeated Await returned different caches: %p vs %p", first, second)
	}
	if calls != 1 {
		t.Errorf("load fn called %d times; expected exactly once", calls)
	}
}

func TestAnalysisCacheLoadFuture_NilSafe(t *testing.T) {
	var f *AnalysisCacheLoadFuture
	f.Start()
	if got := f.Await(); got != nil {
		t.Errorf("nil future Await = %p; want nil", got)
	}
	if d := f.Duration(); d != 0 {
		t.Errorf("nil future Duration = %v; want 0", d)
	}
}
