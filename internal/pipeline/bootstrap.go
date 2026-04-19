package pipeline

import (
	"context"
	"fmt"

	"github.com/kaeawc/krit/internal/rules"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// DefaultActiveRules returns the v2 rules enabled by default for this
// build. The LSP server, MCP server, and CLI all derive their active
// rule list from this function so a rule that ships disabled-by-default
// stays disabled in every entry point without special-case filtering.
func DefaultActiveRules() []*v2.Rule {
	active := make([]*v2.Rule, 0, len(v2.Registry))
	for _, r := range v2.Registry {
		if rules.IsDefaultActive(r.ID) {
			active = append(active, r)
		}
	}
	return active
}

// BuildDispatcher constructs a *rules.Dispatcher from the given v2 rules
// and an optional type resolver. Centralising this means the LSP and
// MCP servers no longer duplicate the rule-conversion and dispatcher
// construction logic (roadmap acceptance criterion: "LSP and MCP use
// no rule-dispatch logic of their own").
//
// Pass resolver=nil when no type-aware analysis is available; the
// dispatcher degrades gracefully.
func BuildDispatcher(activeRules []*v2.Rule, resolver typeinfer.TypeResolver) *rules.Dispatcher {
	return rules.NewDispatcherV2(activeRules, resolver)
}

// ParseSingle parses a single in-memory Kotlin buffer into a *scanner.File
// using the same tree-sitter parser ParsePhase uses internally. This is
// the single-file entry point the LSP and MCP servers use instead of
// re-implementing GetKotlinParser / NewParsedFile in each package.
//
// The returned File has FlatTree populated and Content set to the supplied
// bytes. SuppressionIdx is left nil — per-file @Suppress lookups are built
// lazily by the dispatcher for single-file analysis.
func ParseSingle(ctx context.Context, path string, content []byte) (*scanner.File, error) {
	if path == "" {
		path = "input.kt"
	}
	parser := scanner.GetKotlinParser()
	defer scanner.PutKotlinParser(parser)
	tree, err := parser.ParseCtx(ctx, nil, content)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return scanner.NewParsedFile(path, content, tree), nil
}

// SingleFileAnalyzer wraps the active v2 rule set and a pre-built
// dispatcher so LSP and MCP callers can run Parse → Dispatch on an
// in-memory buffer without reassembling the pipeline per edit. The
// struct is safe for concurrent use by the dispatcher (see
// rules.Dispatcher docs), though callers that need thread-safe access
// to ActiveRules should treat it as read-only after construction.
type SingleFileAnalyzer struct {
	ActiveRules []*v2.Rule
	Dispatcher  *rules.Dispatcher
}

// NewSingleFileAnalyzer constructs a SingleFileAnalyzer from the supplied
// active rule set. A nil slice falls back to DefaultActiveRules().
// Resolver may be nil; the dispatcher handles that gracefully.
func NewSingleFileAnalyzer(active []*v2.Rule, resolver typeinfer.TypeResolver) *SingleFileAnalyzer {
	if active == nil {
		active = DefaultActiveRules()
	}
	return &SingleFileAnalyzer{
		ActiveRules: active,
		Dispatcher:  BuildDispatcher(active, resolver),
	}
}

// AnalyzeBufferColumns parses the in-memory buffer at path and dispatches
// per-file rules, returning the findings in columnar form. Cross-file,
// module-aware, manifest, resource, and gradle rules are skipped — the
// single-file entry points (LSP didChange, MCP analyze tool) only see
// one buffer at a time and cannot satisfy those rules' inputs.
func (a *SingleFileAnalyzer) AnalyzeBufferColumns(ctx context.Context, path string, content []byte) (scanner.FindingColumns, *scanner.File, error) {
	file, err := ParseSingle(ctx, path, content)
	if err != nil {
		return scanner.FindingColumns{}, nil, err
	}
	columns, _ := a.Dispatcher.RunColumnsWithStats(file)
	return columns, file, nil
}

// AnalyzeFile dispatches per-file rules against an already-parsed file
// and returns the compatibility Finding slice. Used by LSP when the
// cached *scanner.File is reused across requests.
func (a *SingleFileAnalyzer) AnalyzeFile(file *scanner.File) []scanner.Finding {
	columns := a.Dispatcher.Run(file)
	return columns.Findings()
}
