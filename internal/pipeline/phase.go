// Package pipeline composes the Krit analysis workflow into a sequence of
// named, independently testable phases. The CLI, LSP server, and MCP server
// share the same phase implementations rather than each re-implementing
// rule loading, dispatch, and output.
//
// The six phases are:
//
//	Parse     → ParseResult      // read + flat-parse Kotlin/Java sources
//	Index     → IndexResult      // type resolver, oracle, cross-file/module/android indices
//	Dispatch  → DispatchResult   // per-file rule dispatch (single AST walk)
//	CrossFile → CrossFileResult  // cross-file + module + android rules, suppression applied
//	Fixup     → FixupResult      // auto-fix application
//	Output    → (side effect)    // JSON / plain / SARIF / Checkstyle + baseline + diff filter
//
// Each phase is a Go value implementing Phase[In, Out]. A concrete default
// implementation exists for each phase, but tests and alternative callers
// (LSP re-using a cached IndexResult, for example) can substitute a fake
// with FakePhase[In, Out].
package pipeline

import (
	"context"
	"fmt"
)

// Phase is a single named stage of the analysis pipeline. Phases compose
// by chaining the Out of one into the In of the next.
//
// Phases may have side effects (Output writes to an io.Writer, Fixup
// modifies files on disk), but those effects must be reflected in the Out
// type or in fields of the In value — never hidden global state.
type Phase[In, Out any] interface {
	// Name returns a short, stable identifier used for timing and error
	// reporting. It must not vary between calls.
	Name() string

	// Run executes the phase. If ctx is cancelled, Run should return
	// ctx.Err() as soon as practical. On failure, Run returns the
	// zero Out value and a non-nil error.
	Run(ctx context.Context, in In) (Out, error)
}

// PhaseFunc adapts an ordinary function to the Phase interface. Used for
// quick composition and in tests.
type PhaseFunc[In, Out any] struct {
	N  string
	Fn func(context.Context, In) (Out, error)
}

// Name returns the phase name.
func (p PhaseFunc[In, Out]) Name() string { return p.N }

// Run invokes the underlying function.
func (p PhaseFunc[In, Out]) Run(ctx context.Context, in In) (Out, error) {
	return p.Fn(ctx, in)
}

// PhaseError wraps an error returned from a phase with the phase's name,
// so diagnostics upstream can identify which stage failed.
type PhaseError struct {
	Phase string
	Err   error
}

// Error implements the error interface.
func (e *PhaseError) Error() string {
	return fmt.Sprintf("pipeline: %s: %v", e.Phase, e.Err)
}

// Unwrap returns the underlying error for errors.Is / errors.As.
func (e *PhaseError) Unwrap() error { return e.Err }

// wrapErr tags err with phaseName unless err is nil or already a PhaseError.
func wrapErr(phaseName string, err error) error {
	if err == nil {
		return nil
	}
	if _, ok := err.(*PhaseError); ok {
		return err
	}
	return &PhaseError{Phase: phaseName, Err: err}
}

// runPhase is the single place that enforces context checks and error
// wrapping for every phase invocation in the pipeline.
func runPhase[In, Out any](ctx context.Context, p Phase[In, Out], in In) (Out, error) {
	var zero Out
	if err := ctx.Err(); err != nil {
		return zero, wrapErr(p.Name(), err)
	}
	out, err := p.Run(ctx, in)
	if err != nil {
		return zero, wrapErr(p.Name(), err)
	}
	return out, nil
}
