package pipeline

import (
	"context"
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
)

// PreparseFuture parses the project's sources in the background while a slow
// producer that does not read them (the oracle JVM) runs. The parse phase takes
// the result when its own input matches, so a cold scan stops paying for the
// parse after the JVM finishes. Any mismatch or failure falls back to a normal
// parse, so findings never depend on it.
type PreparseFuture struct {
	input ParseInput
	phase ParsePhase

	once    sync.Once
	started atomic.Bool
	done    chan struct{}

	result ParseResult
	err    error

	taken bool
}

// NewPreparseFuture prepares a background parse of in. Start begins it.
func NewPreparseFuture(phase ParsePhase, in ParseInput) *PreparseFuture {
	return &PreparseFuture{input: in, phase: phase, done: make(chan struct{})}
}

// Start begins the parse once. It is safe to call on a nil future.
func (f *PreparseFuture) Start(ctx context.Context) {
	if f == nil {
		return
	}
	f.once.Do(func() {
		f.started.Store(true)
		go func() {
			defer close(f.done)
			defer func() {
				if r := recover(); r != nil {
					f.err = fmt.Errorf("preparse panic: %v", r)
				}
			}()
			f.result, f.err = f.phase.Run(ctx, f.input)
		}()
	})
}

// Take waits for the background parse and returns its result when in asks for
// the same parse. It returns false when the future was never started, already
// taken, failed, or was built for a different input.
func (f *PreparseFuture) Take(in ParseInput) (ParseResult, bool) {
	if f == nil || !f.started.Load() || f.taken || !f.matches(in) {
		return ParseResult{}, false
	}
	<-f.done
	if f.err != nil {
		return ParseResult{}, false
	}
	f.taken = true
	return f.result, true
}

// matches reports whether in would produce the same parse as the future's input.
func (f *PreparseFuture) matches(in ParseInput) bool {
	want := f.input
	return in.Config == want.Config &&
		in.ParseCache == want.ParseCache &&
		in.ResidentFiles == nil && want.ResidentFiles == nil &&
		in.IncludeGenerated == want.IncludeGenerated &&
		in.SkipJavaCollection == want.SkipJavaCollection &&
		in.Workers == want.Workers &&
		slices.Equal(in.Paths, want.Paths) &&
		slices.Equal(in.Excludes, want.Excludes) &&
		slices.Equal(in.GeneratedSourceDirs, want.GeneratedSourceDirs) &&
		slices.Equal(in.IncludeGeneratedAllowlist, want.IncludeGeneratedAllowlist) &&
		in.KotlinPaths != nil && want.KotlinPaths != nil && slices.Equal(in.KotlinPaths, want.KotlinPaths) &&
		(in.JavaPaths == nil) == (want.JavaPaths == nil) && slices.Equal(in.JavaPaths, want.JavaPaths) &&
		slices.Equal(in.ActiveRules, want.ActiveRules)
}
