package pipeline

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestPhaseFunc_Name(t *testing.T) {
	p := PhaseFunc[int, int]{
		N:  "double",
		Fn: func(_ context.Context, n int) (int, error) { return n * 2, nil },
	}
	if p.Name() != "double" {
		t.Fatalf("Name() = %q, want %q", p.Name(), "double")
	}
}

func TestPhaseFunc_Run_Success(t *testing.T) {
	p := PhaseFunc[int, int]{
		N:  "double",
		Fn: func(_ context.Context, n int) (int, error) { return n * 2, nil },
	}
	got, err := p.Run(context.Background(), 7)
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if got != 14 {
		t.Fatalf("Run = %d, want 14", got)
	}
}

func TestPhaseFunc_Run_Error(t *testing.T) {
	sentinel := errors.New("boom")
	p := PhaseFunc[int, int]{
		N:  "fail",
		Fn: func(_ context.Context, _ int) (int, error) { return 0, sentinel },
	}
	_, err := p.Run(context.Background(), 0)
	if !errors.Is(err, sentinel) {
		t.Fatalf("Run err = %v, want sentinel", err)
	}
}

func TestWrapErr_NilPassthrough(t *testing.T) {
	if wrapErr("any", nil) != nil {
		t.Fatal("wrapErr(nil) must return nil")
	}
}

func TestWrapErr_WrapsOnce(t *testing.T) {
	base := errors.New("inner")
	wrapped := wrapErr("parse", base)

	var pe *PhaseError
	if !errors.As(wrapped, &pe) {
		t.Fatalf("expected *PhaseError, got %T", wrapped)
	}
	if pe.Phase != "parse" {
		t.Fatalf("Phase = %q, want %q", pe.Phase, "parse")
	}
	if !errors.Is(wrapped, base) {
		t.Fatal("errors.Is must return true for underlying error")
	}

	// Double-wrap must not nest.
	twice := wrapErr("index", wrapped)
	var pe2 *PhaseError
	if !errors.As(twice, &pe2) {
		t.Fatal("expected *PhaseError after second wrap")
	}
	if pe2.Phase != "parse" {
		t.Fatalf("Phase after second wrap = %q, want %q (no renesting)", pe2.Phase, "parse")
	}
}

func TestPhaseError_Error(t *testing.T) {
	err := &PhaseError{Phase: "dispatch", Err: errors.New("oops")}
	got := err.Error()
	want := "pipeline: dispatch: oops"
	if got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}
}

func TestRunPhase_PreCancel(t *testing.T) {
	called := false
	p := PhaseFunc[int, int]{
		N:  "should-not-run",
		Fn: func(_ context.Context, _ int) (int, error) { called = true; return 0, nil },
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before invocation

	_, err := runPhase[int, int](ctx, p, 1)
	if err == nil {
		t.Fatal("expected error from pre-cancelled context")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	if called {
		t.Fatal("phase Run must not be called when ctx is already cancelled")
	}
	var pe *PhaseError
	if !errors.As(err, &pe) || pe.Phase != "should-not-run" {
		t.Fatalf("error not tagged with phase name: %v", err)
	}
}

func TestRunPhase_WrapsInnerError(t *testing.T) {
	inner := errors.New("dispatch failed")
	p := PhaseFunc[int, int]{
		N:  "dispatch",
		Fn: func(_ context.Context, _ int) (int, error) { return 0, inner },
	}
	_, err := runPhase[int, int](context.Background(), p, 0)
	if err == nil {
		t.Fatal("expected non-nil error")
	}
	var pe *PhaseError
	if !errors.As(err, &pe) {
		t.Fatalf("want *PhaseError, got %T", err)
	}
	if pe.Phase != "dispatch" {
		t.Fatalf("Phase = %q, want dispatch", pe.Phase)
	}
	if !errors.Is(err, inner) {
		t.Fatal("inner error must be reachable via errors.Is")
	}
}

func TestFakePhase_RecordsCalls(t *testing.T) {
	f := NewFakePhase[string, int]("parse", 42)
	for _, in := range []string{"a", "b", "c"} {
		_, _ = f.Run(context.Background(), in)
	}
	if got := f.CallCount(); got != 3 {
		t.Fatalf("CallCount = %d, want 3", got)
	}
	calls := f.Calls()
	if len(calls) != 3 || calls[0] != "a" || calls[2] != "c" {
		t.Fatalf("Calls = %v, want [a b c]", calls)
	}
}

func TestFakePhase_CancelDuringDelay(t *testing.T) {
	f := &FakePhase[int, int]{N: "slow", Out: 99, Delay: 200 * time.Millisecond}
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		time.Sleep(10 * time.Millisecond)
		cancel()
	}()

	_, err := f.Run(ctx, 0)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func TestObservablePhase_RecordsDurationAndInput(t *testing.T) {
	// Deterministic clock: each call advances by step. Run reads the
	// clock twice (start, end), so each record's Duration == step.
	const step = 7 * time.Millisecond
	var tick time.Time
	clock := func() time.Time {
		tick = tick.Add(step)
		return tick
	}

	inner := PhaseFunc[string, int]{
		N:  "len",
		Fn: func(_ context.Context, s string) (int, error) { return len(s), nil },
	}
	o := &ObservablePhase[string, int]{Inner: inner, Now: clock}

	if _, err := o.Run(context.Background(), "hello"); err != nil {
		t.Fatal(err)
	}
	if _, err := o.Run(context.Background(), "world!"); err != nil {
		t.Fatal(err)
	}
	records := o.Records()
	if len(records) != 2 {
		t.Fatalf("Records len = %d, want 2", len(records))
	}
	if records[0].Phase != "len" || records[0].Input != "hello" {
		t.Errorf("record[0] = %+v, want phase=len input=hello", records[0])
	}
	if records[1].Input != "world!" {
		t.Errorf("record[1].Input = %q, want world!", records[1].Input)
	}
	for i, r := range records {
		if r.Duration != step {
			t.Errorf("record[%d].Duration = %v, want %v", i, r.Duration, step)
		}
	}
}
