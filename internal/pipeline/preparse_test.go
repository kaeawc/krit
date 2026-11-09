package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/perf"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func preparseFixture(t *testing.T) (ProjectArgs, ProjectHostState) {
	t.Helper()
	dir := t.TempDir()
	var paths []string
	for _, name := range []string{"A.kt", "B.kt"} {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte("package p\nclass "+name[:1]+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, p)
	}
	args := ProjectArgs{
		Paths:       []string{dir},
		KotlinPaths: paths,
		JavaPaths:   []string{},
		Workers:     2,
		ActiveRules: []*api.Rule{{ID: "CrossFileRule", Needs: api.NeedsCrossFile}},
	}
	return args, ProjectHostState{}
}

func parseInputFor(args ProjectArgs, host ProjectHostState) ParseInput {
	return projectParseInput(args, host, args.KotlinPaths, args.JavaPaths, false)
}

func TestPreparseServesTheMatchingParseOnce(t *testing.T) {
	args, host := preparseFixture(t)
	f := StartPreparse(context.Background(), args, host)
	got, ok := f.Take(parseInputFor(args, host))
	if !ok {
		t.Fatal("Take refused a matching input")
	}
	if len(got.KotlinFiles) != 2 {
		t.Fatalf("parsed %d files, want 2", len(got.KotlinFiles))
	}
	if _, ok := f.Take(parseInputFor(args, host)); ok {
		t.Fatal("a parse result must be taken only once")
	}
}

func TestPreparseRefusesADifferentParse(t *testing.T) {
	args, host := preparseFixture(t)
	f := StartPreparse(context.Background(), args, host)

	fewer := parseInputFor(args, host)
	fewer.KotlinPaths = args.KotlinPaths[:1]
	if _, ok := f.Take(fewer); ok {
		t.Fatal("Take served a different path list")
	}
	skip := parseInputFor(args, host)
	skip.SkipJavaCollection = true
	if _, ok := f.Take(skip); ok {
		t.Fatal("Take served a different Java collection mode")
	}
	if _, ok := f.Take(parseInputFor(args, host)); !ok {
		t.Fatal("a refused request must not consume the result")
	}
}

func TestPreparseIsNilWhenTheParseCannotBePredicted(t *testing.T) {
	args, host := preparseFixture(t)
	thorough := args
	thorough.TargetedResolution = true
	if StartPreparse(context.Background(), thorough, host) != nil {
		t.Fatal("thorough runs resolve their own rules, so the parse cannot be predicted")
	}
	noPaths := args
	noPaths.KotlinPaths = nil
	if StartPreparse(context.Background(), noPaths, host) != nil {
		t.Fatal("a run without collected Kotlin paths must not preparse")
	}
	var none *PreparseFuture
	if _, ok := none.Take(parseInputFor(args, host)); ok {
		t.Fatal("nil future served a result")
	}
	if none.takeCodeIndex(IndexInput{}, &IndexResult{}) {
		t.Fatal("nil future served a code index")
	}
}

func TestPreparseFallsBackWhenTheParseFails(t *testing.T) {
	args, host := preparseFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	f := StartPreparse(ctx, args, host)
	if _, ok := f.Take(parseInputFor(args, host)); ok {
		t.Fatal("a failed background parse must not be served")
	}
}

func TestPreparseServesTheCodeIndexForTheSameFiles(t *testing.T) {
	args, host := preparseFixture(t)
	f := StartPreparse(context.Background(), args, host)
	parsed, ok := f.Take(parseInputFor(args, host))
	if !ok {
		t.Fatal("Take refused a matching input")
	}
	in := IndexInput{ParseResult: parsed, CrossFileJobsFlag: args.Workers, CrossFileJavaPaths: args.JavaPaths}

	other := in
	other.CrossFileJobsFlag = 7
	if f.takeCodeIndex(other, &IndexResult{}) {
		t.Fatal("served an index built with different settings")
	}
	reordered := in
	reordered.KotlinFiles = append([]*scanner.File(nil), parsed.KotlinFiles...)
	if f.takeCodeIndex(reordered, &IndexResult{}) {
		t.Fatal("served an index for a different file slice")
	}

	var result IndexResult
	in.CrossFileParentTracker = perf.New(true)
	if !f.takeCodeIndex(in, &result) {
		t.Fatal("did not serve the prebuilt code index")
	}
	if result.CodeIndex == nil {
		t.Fatal("served a nil code index")
	}
	if f.takeCodeIndex(in, &IndexResult{}) {
		t.Fatal("a code index must be served only once")
	}
}

func TestPreparseSkipsTheCodeIndexWithoutCrossFileRules(t *testing.T) {
	args, host := preparseFixture(t)
	args.ActiveRules = nil
	f := StartPreparse(context.Background(), args, host)
	parsed, ok := f.Take(parseInputFor(args, host))
	if !ok {
		t.Fatal("Take refused a matching input")
	}
	if f.takeCodeIndex(IndexInput{ParseResult: parsed, CrossFileJobsFlag: args.Workers, CrossFileJavaPaths: args.JavaPaths}, &IndexResult{}) {
		t.Fatal("served a code index nobody asked to build")
	}
}
