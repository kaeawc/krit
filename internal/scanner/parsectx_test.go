package scanner

import (
	"context"
	"errors"
	"strings"
	"testing"

	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/kotlin"
)

// racyCtx reports Done as already closed while Err stays nil, reproducing the
// window where ParseCtx's watcher goroutine sees ctx.Done() after the parse
// has finished but before cancellation is observable via Err().
type racyCtx struct {
	context.Context
	done chan struct{}
}

func newRacyCtx() racyCtx {
	d := make(chan struct{})
	close(d)
	return racyCtx{Context: context.Background(), done: d}
}

func (c racyCtx) Done() <-chan struct{} { return c.done }
func (c racyCtx) Err() error            { return nil }

func newTestKotlin() *sitter.Parser {
	p := sitter.NewParser()
	p.SetLanguage(kotlin.GetLanguage())
	return p
}

var bigSrc = []byte(strings.Repeat("val x = 1\n", 5000))

// poison parses with racyCtx until the cancel flag sticks on the parser.
func poison(t *testing.T, p *sitter.Parser) {
	t.Helper()
	for i := 0; i < 200; i++ {
		tree, err := p.ParseCtx(newRacyCtx(), nil, bigSrc)
		if err == nil {
			tree.Close()
			continue
		}
		if !errors.Is(err, sitter.ErrOperationLimit) {
			t.Fatalf("unexpected error: %v", err)
		}
		// Drop the halted parse state; the stuck cancel flag remains.
		p.Reset()
		return
	}
	t.Fatal("could not poison parser")
}

func TestParseCtxPooled_RecoversFromPoisonedParser(t *testing.T) {
	p := newTestKotlin()
	poison(t, p)

	fresh := 0
	tree, reusable, err := ParseCtxPooled(context.Background(), p, func() *sitter.Parser {
		fresh++
		return newTestKotlin()
	}, bigSrc)
	if err != nil {
		t.Fatalf("expected recovery, got %v", err)
	}
	defer tree.Close()
	if tree.RootNode() == nil || tree.RootNode().HasError() {
		t.Fatal("expected valid tree")
	}
	if fresh != 1 {
		t.Fatalf("expected exactly one fresh parser, got %d", fresh)
	}
	if reusable == nil || reusable == p {
		t.Fatal("poisoned parser must not be returned for reuse")
	}
	// The returned parser is healthy for the next parse.
	tree2, again, err := ParseCtxPooled(context.Background(), reusable, newTestKotlin, bigSrc)
	if err != nil || again != reusable {
		t.Fatalf("reuse failed: %v", err)
	}
	tree2.Close()
	again.Close()
}

func TestParseCtxPooled_OtherErrorDiscardsParser(t *testing.T) {
	p := sitter.NewParser() // no language set: ErrNoLanguage
	_, reusable, err := ParseCtxPooled(context.Background(), p, newTestKotlin, []byte("val x = 1"))
	if !errors.Is(err, sitter.ErrNoLanguage) {
		t.Fatalf("expected ErrNoLanguage, got %v", err)
	}
	if reusable != nil {
		t.Fatal("errored parser must not be reused")
	}
}
