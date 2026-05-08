package pipeline

import (
	"context"
	"errors"
	"testing"
)

// chain composes six phases with the same I/O type so the foundation
// test can validate sequencing, short-circuit on error, and ctx propagation
// without depending on any real phase implementations yet.
type chainPhases struct {
	parse     Phase[int, int]
	index     Phase[int, int]
	dispatch  Phase[int, int]
	crossFile Phase[int, int]
	fixup     Phase[int, int]
	output    Phase[int, int]
}

func runChain(ctx context.Context, c chainPhases, seed int) (int, error) {
	v, err := runPhase[int, int](ctx, c.parse, seed)
	if err != nil {
		return 0, err
	}
	if v, err = runPhase[int, int](ctx, c.index, v); err != nil {
		return 0, err
	}
	if v, err = runPhase[int, int](ctx, c.dispatch, v); err != nil {
		return 0, err
	}
	if v, err = runPhase[int, int](ctx, c.crossFile, v); err != nil {
		return 0, err
	}
	if v, err = runPhase[int, int](ctx, c.fixup, v); err != nil {
		return 0, err
	}
	if v, err = runPhase[int, int](ctx, c.output, v); err != nil {
		return 0, err
	}
	return v, nil
}

// addPhase returns a phase that adds inc to its input.
func addPhase(name string, inc int) Phase[int, int] {
	return PhaseFunc[int, int]{
		N:  name,
		Fn: func(_ context.Context, n int) (int, error) { return n + inc, nil },
	}
}

func TestPipeline_AllSixPhases_ChainInOrder(t *testing.T) {
	c := chainPhases{
		parse:     addPhase("parse", 1),
		index:     addPhase("index", 2),
		dispatch:  addPhase("dispatch", 4),
		crossFile: addPhase("crossfile", 8),
		fixup:     addPhase("fixup", 16),
		output:    addPhase("output", 32),
	}
	got, err := runChain(context.Background(), c, 0)
	if err != nil {
		t.Fatalf("runChain error: %v", err)
	}
	// 0 + 1 + 2 + 4 + 8 + 16 + 32 = 63
	if got != 63 {
		t.Fatalf("result = %d, want 63", got)
	}
}

func TestPipeline_ErrorInPhaseN_SkipsLaterPhases(t *testing.T) {
	// dispatch fails; crossfile, fixup, output must not run.
	boom := errors.New("dispatch boom")
	laterCalled := 0

	mkLater := func(name string) Phase[int, int] {
		return PhaseFunc[int, int]{
			N: name,
			Fn: func(_ context.Context, n int) (int, error) {
				laterCalled++
				return n, nil
			},
		}
	}

	c := chainPhases{
		parse:    addPhase("parse", 1),
		index:    addPhase("index", 1),
		dispatch: PhaseFunc[int, int]{N: "dispatch", Fn: func(_ context.Context, _ int) (int, error) { return 0, boom }},

		crossFile: mkLater("crossfile"),
		fixup:     mkLater("fixup"),
		output:    mkLater("output"),
	}
	_, err := runChain(context.Background(), c, 0)
	if err == nil {
		t.Fatal("expected error")
	}
	var pe *PhaseError
	if !errors.As(err, &pe) {
		t.Fatalf("want *PhaseError, got %T", err)
	}
	if pe.Phase != "dispatch" {
		t.Fatalf("tagged phase = %q, want dispatch", pe.Phase)
	}
	if !errors.Is(err, boom) {
		t.Fatal("original error must be reachable via errors.Is")
	}
	if laterCalled != 0 {
		t.Fatalf("later phases ran %d times, expected 0", laterCalled)
	}
}

func TestPipeline_CancellationBetweenPhases(t *testing.T) {
	// Parse completes, then ctx is cancelled; subsequent phases must
	// not execute.
	ctx, cancel := context.WithCancel(context.Background())
	laterRan := false

	parse := PhaseFunc[int, int]{
		N: "parse",
		Fn: func(_ context.Context, n int) (int, error) {
			cancel() // cancel during parse so the next runPhase sees it
			return n, nil
		},
	}
	mkLater := func(name string) Phase[int, int] {
		return PhaseFunc[int, int]{
			N:  name,
			Fn: func(_ context.Context, n int) (int, error) { laterRan = true; return n, nil },
		}
	}

	c := chainPhases{
		parse:     parse,
		index:     mkLater("index"),
		dispatch:  mkLater("dispatch"),
		crossFile: mkLater("crossfile"),
		fixup:     mkLater("fixup"),
		output:    mkLater("output"),
	}
	_, err := runChain(ctx, c, 0)
	if err == nil {
		t.Fatal("expected cancellation error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	var pe *PhaseError
	if !errors.As(err, &pe) || pe.Phase != "index" {
		t.Fatalf("want PhaseError from 'index' (first post-parse), got %v", err)
	}
	if laterRan {
		t.Fatal("no later phase should run after cancellation")
	}
}

func TestPipeline_ObservablePhaseSeesAllInputs(t *testing.T) {
	parse := &ObservablePhase[int, int]{Inner: addPhase("parse", 10)}
	index := &ObservablePhase[int, int]{Inner: addPhase("index", 100)}

	v, err := runPhase[int, int](context.Background(), parse, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runPhase[int, int](context.Background(), index, v); err != nil {
		t.Fatal(err)
	}
	if got := parse.Records(); len(got) != 1 || got[0].Input != 1 {
		t.Errorf("parse records = %+v", got)
	}
	if got := index.Records(); len(got) != 1 || got[0].Input != 11 {
		t.Errorf("index records = %+v (expected input=11 = 1+10)", got)
	}
}
