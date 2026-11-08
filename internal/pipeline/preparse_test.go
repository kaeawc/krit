package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func preparseFixture(t *testing.T) ParseInput {
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
	return ParseInput{Paths: []string{dir}, KotlinPaths: paths, JavaPaths: []string{}, Workers: 2}
}

func TestPreparseFutureServesMatchingInput(t *testing.T) {
	in := preparseFixture(t)
	f := NewPreparseFuture(ParsePhase{Workers: 2}, in)
	f.Start(context.Background())
	got, ok := f.Take(in)
	if !ok {
		t.Fatal("Take refused a matching input")
	}
	want, err := (ParsePhase{Workers: 2}).Run(context.Background(), in)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.KotlinFiles) != len(want.KotlinFiles) || len(got.KotlinFiles) != 2 {
		t.Fatalf("parsed %d files, want %d", len(got.KotlinFiles), len(want.KotlinFiles))
	}
	if _, ok := f.Take(in); ok {
		t.Fatal("a result must be taken only once")
	}
}

func TestPreparseFutureRefusesOtherInput(t *testing.T) {
	in := preparseFixture(t)
	f := NewPreparseFuture(ParsePhase{Workers: 2}, in)
	f.Start(context.Background())

	fewer := in
	fewer.KotlinPaths = in.KotlinPaths[:1]
	if _, ok := f.Take(fewer); ok {
		t.Fatal("Take served a different path list")
	}
	skip := in
	skip.SkipJavaCollection = true
	if _, ok := f.Take(skip); ok {
		t.Fatal("Take served a different Java collection mode")
	}
	if _, ok := f.Take(in); !ok {
		t.Fatal("a refused request must not consume the result")
	}
}

func TestPreparseFutureNeverStartedOrNil(t *testing.T) {
	in := preparseFixture(t)
	if _, ok := NewPreparseFuture(ParsePhase{}, in).Take(in); ok {
		t.Fatal("Take waited on a future that was never started")
	}
	var nilFuture *PreparseFuture
	nilFuture.Start(context.Background())
	if _, ok := nilFuture.Take(in); ok {
		t.Fatal("nil future served a result")
	}
}

func TestPreparseFutureFallsBackWhenParseFails(t *testing.T) {
	in := preparseFixture(t)
	f := NewPreparseFuture(ParsePhase{Workers: 2}, in)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	f.Start(ctx)
	if _, ok := f.Take(in); ok {
		t.Fatal("a failed background parse must not be served")
	}
}
