package pipeline

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// fakeResolver returns canned facts and records the request it was
// called with, so tests can prove the pre-pass actually batches the
// positions selectors produced.
type fakeResolver struct {
	canned map[string]map[api.ExpressionPosition]*typeinfer.ResolvedType
	calls  []map[string][]api.ExpressionPosition
	err    error
}

func (f *fakeResolver) Resolve(positions map[string][]api.ExpressionPosition) (map[string]map[api.ExpressionPosition]*typeinfer.ResolvedType, error) {
	clone := make(map[string][]api.ExpressionPosition, len(positions))
	for k, v := range positions {
		clone[k] = append([]api.ExpressionPosition(nil), v...)
	}
	f.calls = append(f.calls, clone)
	if f.err != nil {
		return nil, f.err
	}
	return f.canned, nil
}

// fakeSink captures SetExpressionFact calls so tests can assert which
// facts the pre-pass injected.
type fakeSink struct {
	written map[string]map[api.ExpressionPosition]*typeinfer.ResolvedType
}

func newFakeSink() *fakeSink {
	return &fakeSink{written: make(map[string]map[api.ExpressionPosition]*typeinfer.ResolvedType)}
}

func (f *fakeSink) SetExpressionFact(filePath string, line, col int, t *typeinfer.ResolvedType) {
	if f.written[filePath] == nil {
		f.written[filePath] = make(map[api.ExpressionPosition]*typeinfer.ResolvedType)
	}
	f.written[filePath][api.ExpressionPosition{Line: line, Col: col}] = t
}

func TestRunTargetedResolutionPass_NoResolverIsNoOp(t *testing.T) {
	rule := &api.Rule{ID: "X", ExprPositions: func(*scanner.File) []uint32 { return []uint32{0} }}
	file := parseTestFile(t, "package p\nfun f() {}\n")
	err := RunTargetedResolutionPass(TargetedResolutionInput{
		ActiveRules: []*api.Rule{rule},
		Files:       []*scanner.File{file},
		Resolver:    nil,
		Sink:        newFakeSink(),
	})
	if err != nil {
		t.Errorf("expected nil error when resolver is nil; got %v", err)
	}
}

func TestRunTargetedResolutionPass_NoSinkIsNoOp(t *testing.T) {
	resolver := &fakeResolver{}
	rule := &api.Rule{ID: "X", ExprPositions: func(*scanner.File) []uint32 { return []uint32{0} }}
	file := parseTestFile(t, "package p\nfun f() {}\n")
	err := RunTargetedResolutionPass(TargetedResolutionInput{
		ActiveRules: []*api.Rule{rule},
		Files:       []*scanner.File{file},
		Resolver:    resolver,
		Sink:        nil,
	})
	if err != nil {
		t.Errorf("expected nil error when sink is nil; got %v", err)
	}
	if len(resolver.calls) != 0 {
		t.Errorf("resolver should not be called when sink is nil; got %d calls", len(resolver.calls))
	}
}

func TestRunTargetedResolutionPass_NoSelectorsSkipsResolver(t *testing.T) {
	resolver := &fakeResolver{}
	rule := &api.Rule{ID: "X"} // ExprPositions nil
	file := parseTestFile(t, "package p\nfun f() {}\n")
	err := RunTargetedResolutionPass(TargetedResolutionInput{
		ActiveRules: []*api.Rule{rule},
		Files:       []*scanner.File{file},
		Resolver:    resolver,
		Sink:        newFakeSink(),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resolver.calls) != 0 {
		t.Errorf("resolver should not be called when no rule selects positions; got %d calls", len(resolver.calls))
	}
}

func TestRunTargetedResolutionPass_AppliesResolvedFacts(t *testing.T) {
	file := parseTestFile(t, "package p\nfun f() { val x = 1 }\n")
	const targetIdx uint32 = 2
	rule := &api.Rule{ID: "Demo", ExprPositions: func(*scanner.File) []uint32 {
		return []uint32{targetIdx}
	}}
	expectedPos := api.ExpressionPosition{
		Line: file.FlatRow(targetIdx) + 1,
		Col:  file.FlatCol(targetIdx) + 1,
	}
	resolver := &fakeResolver{
		canned: map[string]map[api.ExpressionPosition]*typeinfer.ResolvedType{
			file.Path: {expectedPos: {Name: "Int", FQN: "kotlin.Int", Kind: typeinfer.TypeClass}},
		},
	}
	sink := newFakeSink()

	err := RunTargetedResolutionPass(TargetedResolutionInput{
		ActiveRules: []*api.Rule{rule},
		Files:       []*scanner.File{file},
		Resolver:    resolver,
		Sink:        sink,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Resolver received exactly the positions the selector produced.
	if len(resolver.calls) != 1 {
		t.Fatalf("expected resolver called once; got %d", len(resolver.calls))
	}
	if !reflect.DeepEqual(resolver.calls[0][file.Path], []api.ExpressionPosition{expectedPos}) {
		t.Errorf("resolver got wrong positions: %v", resolver.calls[0])
	}
	// Sink received the canned fact at the right key.
	got := sink.written[file.Path][expectedPos]
	if got == nil || got.Name != "Int" {
		t.Errorf("expected sink to receive Int fact at %v; got %v", expectedPos, got)
	}
}

func TestRunTargetedResolutionPass_PropagatesResolverError(t *testing.T) {
	resolver := &fakeResolver{err: errors.New("daemon offline")}
	file := parseTestFile(t, "package p\nfun f() { val x = 1 }\n")
	rule := &api.Rule{ID: "X", ExprPositions: func(*scanner.File) []uint32 { return []uint32{2} }}
	sink := newFakeSink()
	err := RunTargetedResolutionPass(TargetedResolutionInput{
		ActiveRules: []*api.Rule{rule},
		Files:       []*scanner.File{file},
		Resolver:    resolver,
		Sink:        sink,
	})
	if err == nil {
		t.Fatal("expected error to propagate from resolver")
	}
	if len(sink.written) != 0 {
		t.Errorf("sink should not be written when resolver errors; got %v", sink.written)
	}
}

func TestDaemonExpressionResolver_NilDaemonReturnsNil(t *testing.T) {
	r := DaemonExpressionResolver{Daemon: nil}
	got, err := r.Resolve(map[string][]api.ExpressionPosition{"/a.kt": {{Line: 1, Col: 1}}})
	if err != nil {
		t.Errorf("nil daemon should not error; got %v", err)
	}
	if got != nil {
		t.Errorf("nil daemon should return nil result; got %v", got)
	}
}

// parseTestFile creates a temp .kt file and parses it with the real
// scanner so FlatRow/FlatCol return the actual byte → line/col mapping.
func parseTestFile(t *testing.T, source string) *scanner.File {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.kt")
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := scanner.ParseFile(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	return f
}
