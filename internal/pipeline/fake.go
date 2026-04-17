package pipeline

import (
	"context"
	"sync"
	"time"
)

// FakePhase is a test double for a Phase. It returns the configured Out
// (or Err) from Run and optionally sleeps for Delay so tests can
// exercise cancellation. Calls records every input Run was invoked with.
type FakePhase[In, Out any] struct {
	N     string
	Out   Out
	Err   error
	Delay time.Duration

	mu    sync.Mutex
	calls []In
}

// NewFakePhase builds a FakePhase that returns out on every Run.
func NewFakePhase[In, Out any](name string, out Out) *FakePhase[In, Out] {
	return &FakePhase[In, Out]{N: name, Out: out}
}

// Name returns the phase's configured name.
func (f *FakePhase[In, Out]) Name() string { return f.N }

// Run records in, respects Delay and ctx cancellation, then returns
// (Out, Err).
func (f *FakePhase[In, Out]) Run(ctx context.Context, in In) (Out, error) {
	f.mu.Lock()
	f.calls = append(f.calls, in)
	f.mu.Unlock()

	if f.Delay > 0 {
		select {
		case <-ctx.Done():
			var zero Out
			return zero, ctx.Err()
		case <-time.After(f.Delay):
		}
	}
	if err := ctx.Err(); err != nil {
		var zero Out
		return zero, err
	}
	if f.Err != nil {
		var zero Out
		return zero, f.Err
	}
	return f.Out, nil
}

// Calls returns a copy of every input Run received, in call order.
func (f *FakePhase[In, Out]) Calls() []In {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]In, len(f.calls))
	copy(out, f.calls)
	return out
}

// CallCount returns how many times Run has been invoked.
func (f *FakePhase[In, Out]) CallCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

// ObservablePhase wraps a real Phase and records per-call metadata:
// input captured, duration, and error. Used to verify phase ordering
// and cancellation propagation in integration tests without
// substituting fakes.
//
// Now is an optional clock. When nil, time.Now is used. Tests that
// assert on Duration should inject a deterministic clock rather than
// relying on real elapsed time.
type ObservablePhase[In, Out any] struct {
	Inner Phase[In, Out]
	Now   func() time.Time

	mu      sync.Mutex
	records []PhaseCallRecord[In]
}

// PhaseCallRecord is a single invocation's metadata.
type PhaseCallRecord[In any] struct {
	Phase    string
	Input    In
	Duration time.Duration
	Err      error
}

// Name delegates to the wrapped phase.
func (o *ObservablePhase[In, Out]) Name() string { return o.Inner.Name() }

// Run delegates to the wrapped phase, recording call metadata.
func (o *ObservablePhase[In, Out]) Run(ctx context.Context, in In) (Out, error) {
	now := o.Now
	if now == nil {
		now = time.Now
	}
	start := now()
	out, err := o.Inner.Run(ctx, in)
	rec := PhaseCallRecord[In]{
		Phase:    o.Inner.Name(),
		Input:    in,
		Duration: now().Sub(start),
		Err:      err,
	}
	o.mu.Lock()
	o.records = append(o.records, rec)
	o.mu.Unlock()
	return out, err
}

// Records returns a copy of every recorded call.
func (o *ObservablePhase[In, Out]) Records() []PhaseCallRecord[In] {
	o.mu.Lock()
	defer o.mu.Unlock()
	out := make([]PhaseCallRecord[In], len(o.records))
	copy(out, o.records)
	return out
}
