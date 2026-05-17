package api

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// fakeSink captures SetExpressionFact calls so tests can assert what
// ApplyResolvedExpressions wrote and at what (line, col) keys.
type fakeSink struct {
	written map[string]map[ExpressionPosition]*typeinfer.ResolvedType
}

func newFakeSink() *fakeSink {
	return &fakeSink{written: make(map[string]map[ExpressionPosition]*typeinfer.ResolvedType)}
}

func (f *fakeSink) SetExpressionFact(filePath string, line, col int, t *typeinfer.ResolvedType) {
	if f.written[filePath] == nil {
		f.written[filePath] = make(map[ExpressionPosition]*typeinfer.ResolvedType)
	}
	f.written[filePath][ExpressionPosition{Line: line, Col: col}] = t
}

// fakeResolver returns canned type facts for a fixed (file → position)
// table. Records the inputs it was called with so tests can assert the
// pre-pass produced the right batched payload.
type fakeResolver struct {
	canned map[string]map[ExpressionPosition]*typeinfer.ResolvedType
	calls  []map[string][]ExpressionPosition
	err    error
}

func (f *fakeResolver) Resolve(positions map[string][]ExpressionPosition) (map[string]map[ExpressionPosition]*typeinfer.ResolvedType, error) {
	// Capture a copy so test assertions aren't affected by callers
	// mutating the map after the call returns.
	clone := make(map[string][]ExpressionPosition, len(positions))
	for k, v := range positions {
		clone[k] = append([]ExpressionPosition(nil), v...)
	}
	f.calls = append(f.calls, clone)
	if f.err != nil {
		return nil, f.err
	}
	return f.canned, nil
}

func TestCollectExpressionPositions_NoRules(t *testing.T) {
	files := []*scanner.File{parseTestFile(t, "package p\nfun f() {}\n")}
	got := CollectExpressionPositions(nil, files)
	if got != nil {
		t.Errorf("expected nil for no rules; got %v", got)
	}
}

func TestCollectExpressionPositions_NoFiles(t *testing.T) {
	rule := &Rule{ID: "X", ExprPositions: func(*scanner.File) []uint32 { return []uint32{0} }}
	got := CollectExpressionPositions([]*Rule{rule}, nil)
	if got != nil {
		t.Errorf("expected nil for no files; got %v", got)
	}
}

func TestCollectExpressionPositions_NoSelectors(t *testing.T) {
	rule := &Rule{ID: "X"} // ExprPositions is nil
	files := []*scanner.File{parseTestFile(t, "package p\nfun f() {}\n")}
	got := CollectExpressionPositions([]*Rule{rule}, files)
	if got != nil {
		t.Errorf("expected nil when no rule supplies a selector; got %v", got)
	}
}

func TestCollectExpressionPositions_DeduplicatesAcrossRules(t *testing.T) {
	file := parseTestFile(t, "package p\nfun f() { val x = 1 }\n")
	// Both rules ask for the same position to prove dedup works.
	r1 := &Rule{ID: "A", ExprPositions: func(*scanner.File) []uint32 { return []uint32{2} }}
	r2 := &Rule{ID: "B", ExprPositions: func(*scanner.File) []uint32 { return []uint32{2} }}
	got := CollectExpressionPositions([]*Rule{r1, r2}, []*scanner.File{file})
	if len(got[file.Path]) != 1 {
		t.Errorf("expected dedup to a single position; got %v", got[file.Path])
	}
}

func TestCollectExpressionPositions_SortsAscending(t *testing.T) {
	file := parseTestFile(t, "package p\nfun f() { val x = 1; val y = 2 }\n")
	// Selector returns indices out of order to prove the helper sorts.
	r := &Rule{ID: "A", ExprPositions: func(f *scanner.File) []uint32 {
		// Pick three valid indices likely to span different (line, col) keys.
		return []uint32{4, 1, 3}
	}}
	got := CollectExpressionPositions([]*Rule{r}, []*scanner.File{file})
	positions := got[file.Path]
	for i := 1; i < len(positions); i++ {
		if !positionLess(positions[i-1], positions[i]) && positions[i-1] != positions[i] {
			t.Errorf("positions not sorted at index %d: %v", i, positions)
			break
		}
	}
}

func TestCollectExpressionPositions_OmitsFilesWithNoRequests(t *testing.T) {
	a := parseTestFile(t, "package p\nfun a() {}\n")
	b := parseTestFile(t, "package p\nfun b() {}\n")
	// Selector returns positions only for file a (matches by path).
	r := &Rule{ID: "A", ExprPositions: func(f *scanner.File) []uint32 {
		if f.Path == a.Path {
			return []uint32{0}
		}
		return nil
	}}
	got := CollectExpressionPositions([]*Rule{r}, []*scanner.File{a, b})
	if _, ok := got[a.Path]; !ok {
		t.Errorf("expected file a to appear in output; got %v", got)
	}
	if _, ok := got[b.Path]; ok {
		t.Errorf("file b should be omitted entirely (no requested positions); got %v", got[b.Path])
	}
}

func TestApplyResolvedExpressions_WritesEveryFact(t *testing.T) {
	sink := newFakeSink()
	results := map[string]map[ExpressionPosition]*typeinfer.ResolvedType{
		"a.kt": {
			{Line: 1, Col: 2}: {Name: "Int", FQN: "kotlin.Int", Kind: typeinfer.TypeClass},
			{Line: 7, Col: 4}: {Name: "String", FQN: "kotlin.String", Kind: typeinfer.TypeClass},
		},
	}
	ApplyResolvedExpressions(sink, results)
	if !reflect.DeepEqual(sink.written["a.kt"], results["a.kt"]) {
		t.Errorf("written facts mismatch.\n  want: %v\n  got:  %v",
			results["a.kt"], sink.written["a.kt"])
	}
}

func TestApplyResolvedExpressions_NilSinkIsNoOp(t *testing.T) {
	// Should not panic.
	ApplyResolvedExpressions(nil, map[string]map[ExpressionPosition]*typeinfer.ResolvedType{
		"a.kt": {{Line: 1, Col: 1}: {Name: "Int"}},
	})
}

func TestApplyResolvedExpressions_SkipsNilTypes(t *testing.T) {
	// A resolver entry of nil means "no fact at this position" — the
	// sink should not be called for it (which would otherwise overwrite
	// a real fact set by another resolver).
	sink := newFakeSink()
	results := map[string]map[ExpressionPosition]*typeinfer.ResolvedType{
		"a.kt": {
			{Line: 1, Col: 1}: nil,
			{Line: 2, Col: 1}: {Name: "Int", Kind: typeinfer.TypeClass},
		},
	}
	ApplyResolvedExpressions(sink, results)
	if len(sink.written["a.kt"]) != 1 {
		t.Errorf("expected exactly one fact written (nil entries skipped); got %v", sink.written["a.kt"])
	}
	if _, ok := sink.written["a.kt"][ExpressionPosition{Line: 1, Col: 1}]; ok {
		t.Errorf("nil entry should not have been written")
	}
}

func TestExpressionTypeResolver_FakeRecordsCallsAndReturnsCanned(t *testing.T) {
	canned := map[string]map[ExpressionPosition]*typeinfer.ResolvedType{
		"a.kt": {{Line: 3, Col: 5}: {Name: "String", Kind: typeinfer.TypeClass}},
	}
	resolver := &fakeResolver{canned: canned}

	requested := map[string][]ExpressionPosition{
		"a.kt": {{Line: 3, Col: 5}},
	}
	got, err := resolver.Resolve(requested)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(got, canned) {
		t.Errorf("returned facts mismatch.\n  want: %v\n  got:  %v", canned, got)
	}
	if len(resolver.calls) != 1 || !reflect.DeepEqual(resolver.calls[0], requested) {
		t.Errorf("expected resolver to record one call matching the request; got %v", resolver.calls)
	}
}

func TestExpressionTypeResolver_FakePropagatesError(t *testing.T) {
	resolver := &fakeResolver{err: errors.New("daemon offline")}
	_, err := resolver.Resolve(map[string][]ExpressionPosition{"a.kt": {{Line: 1, Col: 1}}})
	if err == nil {
		t.Fatal("expected error to propagate")
	}
}

// End-to-end smoke: register a rule with a selector, walk a parsed
// file, run the fake resolver against the collected positions, apply
// to the sink, and confirm the fact landed at the right key. This is
// the integration shape PR C will wire into the production pipeline.
func TestEndToEnd_CollectResolveApply(t *testing.T) {
	file := parseTestFile(t, "package p\nfun f() { val x = 1 }\n")

	// Selector returns a fixed flat-node idx for this fixture.
	const targetIdx uint32 = 2
	rule := &Rule{ID: "Demo", ExprPositions: func(*scanner.File) []uint32 {
		return []uint32{targetIdx}
	}}

	requested := CollectExpressionPositions([]*Rule{rule}, []*scanner.File{file})
	if len(requested[file.Path]) != 1 {
		t.Fatalf("expected exactly one requested position; got %v", requested)
	}
	expectedPos := ExpressionPosition{
		Line: file.FlatRow(targetIdx) + 1,
		Col:  file.FlatCol(targetIdx) + 1,
	}
	if requested[file.Path][0] != expectedPos {
		t.Fatalf("requested position mismatch.\n  want: %v\n  got:  %v",
			expectedPos, requested[file.Path][0])
	}

	resolver := &fakeResolver{canned: map[string]map[ExpressionPosition]*typeinfer.ResolvedType{
		file.Path: {expectedPos: {Name: "Int", FQN: "kotlin.Int", Kind: typeinfer.TypeClass}},
	}}
	results, err := resolver.Resolve(requested)
	if err != nil {
		t.Fatalf("resolver error: %v", err)
	}

	sink := newFakeSink()
	ApplyResolvedExpressions(sink, results)
	got := sink.written[file.Path][expectedPos]
	if got == nil || got.Name != "Int" {
		t.Errorf("expected sink to receive Int fact at %v; got %v", expectedPos, got)
	}
}

// parseTestFile creates a temp .kt file and parses it with the real
// scanner so FlatRow/FlatCol return the actual byte→line/col mapping.
// Reusing scanner.ParseFile keeps these tests robust against parser
// internals changing.
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
