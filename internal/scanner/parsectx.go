package scanner

import (
	"context"
	"errors"

	sitter "github.com/smacker/go-tree-sitter"
)

// ParseCtxPooled parses content with a parser borrowed from a pool and
// returns the parser that is safe to hand back to the pool (nil when it must
// be discarded).
//
// go-tree-sitter's ParseCtx starts a goroutine that sets the parser's cancel
// flag when ctx is done. If ctx is cancelled right after the parse completes
// but before that goroutine observes parseComplete, the flag can be set on an
// already-finished parser. The next ParseCtx on it then returns a nil tree
// immediately, reported as ErrOperationLimit (ctx.Err() is nil for the new
// caller), and the flag is never reset. Such a parser is poisoned: it is
// closed here and the parse is retried exactly once with a parser from
// newParser. Any parser whose parse returned an error is never returned for
// reuse.
func ParseCtxPooled(ctx context.Context, parser *sitter.Parser, newParser func() *sitter.Parser, content []byte) (*sitter.Tree, *sitter.Parser, error) {
	tree, err := parser.ParseCtx(ctx, nil, content)
	if err == nil {
		return tree, parser, nil
	}
	poisoned := errors.Is(err, sitter.ErrOperationLimit) && parser.OperationLimit() == 0 && ctx.Err() == nil
	parser.Close()
	if !poisoned {
		return nil, nil, err
	}
	fresh := newParser()
	tree, err = fresh.ParseCtx(ctx, nil, content)
	if err != nil {
		fresh.Close()
		return nil, nil, err
	}
	return tree, fresh, nil
}
