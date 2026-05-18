package cacheutil_test

import (
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/cacheutil"
)

func TestAsyncWriter_CloseFlushesQueuedJobs(t *testing.T) {
	w := cacheutil.NewAsyncWriter(2, 8)
	var ran atomic.Int64
	for i := 0; i < 16; i++ {
		if !w.Submit(func() (int64, error) {
			ran.Add(1)
			return 0, nil
		}) {
			ran.Add(1)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if got := ran.Load(); got != 16 {
		t.Fatalf("ran %d jobs, want 16", got)
	}
	if w.Submit(func() (int64, error) { return 0, nil }) {
		t.Fatal("Submit after Close returned true")
	}
}

func TestAsyncWriter_SubmitFallsBackWhenQueueFull(t *testing.T) {
	w := cacheutil.NewAsyncWriter(1, 1)
	block := make(chan struct{})
	if !w.Submit(func() (int64, error) {
		<-block
		return 0, nil
	}) {
		t.Fatal("first Submit returned false")
	}

	var queued int
	for i := 0; i < 64; i++ {
		if w.Submit(func() (int64, error) { return 0, nil }) {
			queued++
		}
	}
	if queued > 1 {
		t.Fatalf("queued %d jobs into a size-1 queue while worker was blocked", queued)
	}
	close(block)
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestAsyncWriter_CloseReturnsJobErrors(t *testing.T) {
	w := cacheutil.NewAsyncWriter(1, 4)
	want := errors.New("boom")
	if !w.Submit(func() (int64, error) { return 0, want }) {
		t.Fatal("Submit returned false")
	}
	err := w.Close()
	if !errors.Is(err, want) {
		t.Fatalf("Close error = %v, want %v", err, want)
	}
}

func TestAsyncWriter_CloseWhileSubmittingDoesNotPanic(t *testing.T) {
	w := cacheutil.NewAsyncWriter(1, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		deadline := time.After(20 * time.Millisecond)
		for {
			select {
			case <-deadline:
				return
			default:
				_ = w.Submit(func() (int64, error) { return 0, nil })
			}
		}
	}()
	time.Sleep(time.Millisecond)
	_ = w.Close()
	<-done
}

// TestAsyncWriter_ConcurrentRecordErrAndFlush exercises the mutex
// contract around `firstErr`: many workers reporting errors at the
// same time as another goroutine reading them via Flush must not
// race, deadlock, or drop any error from the joined chain. The
// `-race` detector is the primary guard here; the post-flush
// equality check on Close ensures every reported error is also
// surfaced when there is no remaining in-flight work.
func TestAsyncWriter_ConcurrentRecordErrAndFlush(t *testing.T) {
	const (
		workers    = 8
		jobsPerSub = 64
	)
	w := cacheutil.NewAsyncWriter(workers, workers*jobsPerSub)
	want := errors.New("boom")

	var submitted atomic.Int64
	submit := func() {
		for i := 0; i < jobsPerSub; i++ {
			if w.Submit(func() (int64, error) { return 0, want }) {
				submitted.Add(1)
			}
		}
	}

	var wg = make(chan struct{}, workers)
	for i := 0; i < workers; i++ {
		go func() {
			submit()
			wg <- struct{}{}
		}()
	}
	// Race a stream of Flush() calls against the in-flight workers.
	stopFlush := make(chan struct{})
	flushDone := make(chan struct{})
	go func() {
		defer close(flushDone)
		for {
			select {
			case <-stopFlush:
				return
			default:
				_ = w.Flush()
			}
		}
	}()

	for i := 0; i < workers; i++ {
		<-wg
	}
	close(stopFlush)
	<-flushDone

	closeErr := w.Close()
	if !errors.Is(closeErr, want) {
		t.Fatalf("expected joined error chain to contain %q after Close, got %v", want, closeErr)
	}
	if got := submitted.Load(); got != int64(workers*jobsPerSub) {
		t.Fatalf("submitted %d jobs, want %d", got, workers*jobsPerSub)
	}
	stats := w.Stats()
	if stats.Failed != int64(workers*jobsPerSub) {
		t.Fatalf("Failed = %d, want %d", stats.Failed, workers*jobsPerSub)
	}
}
