package rules

import (
	"context"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

const callNameMemoSample = `package demo

fun f(a: Foo?) {
    println("x")
    a?.bar()?.baz { it }
    a!!.qux(1)(2)
    listOf(1).map { it + 1 }.filter(::isOdd)
    Foo.Companion.create<Int>()
    with(a) { run { this.toString() } }
    (a ?: b).let { it }
    "s".format(1)
    obj.` + "`fun`" + `()
}
`

// TestBuildCallNameMemoMatchesCompute checks that the memo holds, for every
// call_expression, the same name the uncached walk computes, and that
// flatCallExpressionName gives the uncached answer for every node (calls and
// non-calls) whether or not a memo is published.
func TestBuildCallNameMemoMatchesCompute(t *testing.T) {
	files := []*scanner.File{parseInlineKt(t, callNameMemoSample)}
	root := filepath.Join("..", "..", "tests", "fixtures")
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".kt") {
			return nil
		}
		if len(files) >= 400 {
			return filepath.SkipAll
		}
		f, perr := scanner.ParseFile(context.Background(), path)
		if perr == nil && f != nil && f.FlatTree != nil {
			files = append(files, f)
		}
		return nil
	})
	if len(files) < 50 {
		t.Fatalf("expected fixture files, got %d", len(files))
	}
	calls := 0
	for _, f := range files {
		memo := buildCallNameMemo(f)
		if memo != nil {
			for i, n := range memo.Nodes {
				if want := computeFlatCallExpressionName(f, n); memo.Names[i] != want {
					t.Fatalf("%s node %d: memo %q, compute %q", f.Path, n, memo.Names[i], want)
				}
				if f.FlatType(n) != "call_expression" {
					t.Fatalf("%s node %d: memo holds non-call %s", f.Path, n, f.FlatType(n))
				}
			}
			calls += len(memo.Nodes)
		}
		for _, published := range []*scanner.CallNameMemo{nil, memo} {
			f.SetCallNameMemo(published)
			for i := 0; i < f.FlatTree.Len(); i++ {
				idx := uint32(i)
				if got, want := flatCallExpressionName(f, idx), computeFlatCallExpressionName(f, idx); got != want {
					t.Fatalf("%s node %d (%s): got %q, want %q (memo %v)", f.Path, idx, f.FlatType(idx), got, want, published != nil)
				}
			}
		}
		f.SetCallNameMemo(nil)
	}
	if calls == 0 {
		t.Fatal("no call expressions memoized")
	}
}

// TestDispatcherPublishesCallNameMemoDuringRun checks that rules see the memo
// while the dispatcher runs a file and that it is cleared afterwards.
func TestDispatcherPublishesCallNameMemoDuringRun(t *testing.T) {
	file := parseInlineKt(t, callNameMemoSample)
	var seen, names int
	rule := api.FakeRule("CallNameMemoProbe",
		api.WithNodeTypes("call_expression"),
		api.WithCheck(func(ctx *api.Context) {
			if memo := ctx.File.CallNameMemo(); memo != nil {
				if _, ok := memo.Lookup(ctx.Idx); ok {
					seen++
				}
			}
			if flatCallExpressionName(ctx.File, ctx.Idx) != "" {
				names++
			}
		}))
	d := NewDispatcher([]*api.Rule{rule}, nil)
	d.Run(file)
	if seen == 0 || names == 0 {
		t.Fatalf("rule saw memo for %d calls, names for %d", seen, names)
	}
	if file.CallNameMemo() != nil {
		t.Fatal("memo still published after Run")
	}
}
