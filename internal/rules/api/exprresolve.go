package api

import (
	"sort"

	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// ExpressionPosition identifies a single oracle expression-type query
// by 1-based line and column. Mirrors the key shape used by
// oracle.LookupExpression so resolved facts drop into the existing
// Expressions map at the same key.
type ExpressionPosition struct {
	Line int
	Col  int
}

// CollectExpressionPositions returns the union of every active rule's
// ExprPositions selector applied to every file, keyed by file.Path.
// Inner slices are deduplicated and sorted ascending by (line, col) so
// downstream consumers (RPC payload builders, fakes, snapshots) see a
// stable order.
//
// Selectors that return nil or whose Rule has ExprPositions == nil
// contribute nothing. A file with zero requested positions is omitted
// from the result map entirely (callers should treat absence as "no
// work to do for this file").
func CollectExpressionPositions(rules []*Rule, files []*scanner.File) map[string][]ExpressionPosition {
	if len(rules) == 0 || len(files) == 0 {
		return nil
	}
	out := make(map[string][]ExpressionPosition)
	for _, file := range files {
		if file == nil {
			continue
		}
		seen := make(map[ExpressionPosition]struct{})
		for _, r := range rules {
			if r == nil || r.ExprPositions == nil {
				continue
			}
			for _, idx := range r.ExprPositions(file) {
				pos := ExpressionPosition{
					Line: file.FlatRow(idx) + 1,
					Col:  file.FlatCol(idx) + 1,
				}
				seen[pos] = struct{}{}
			}
		}
		if len(seen) == 0 {
			continue
		}
		positions := make([]ExpressionPosition, 0, len(seen))
		for pos := range seen {
			positions = append(positions, pos)
		}
		sortPositions(positions)
		out[file.Path] = positions
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// ExpressionTypeResolver resolves a batched set of (file → positions)
// queries to (file → position → type). Implementations may dispatch to
// the krit-types daemon, a one-shot JVM call, or a test fake.
//
// A resolver that returns nil for an entry signals "no fact at this
// position" (vs. a present-but-nullable type). Callers must tolerate
// missing entries — selectors over-approximate, so not every requested
// position will have a fact.
type ExpressionTypeResolver interface {
	Resolve(positions map[string][]ExpressionPosition) (map[string]map[ExpressionPosition]*typeinfer.ResolvedType, error)
}

// ExpressionFactSink accepts resolved expression facts for injection
// into an oracle. The production sink (PR C) adapts oracle.Oracle's
// expressions map; tests use a local fake. Kept as an interface here
// so this package does not import oracle (which would be circular).
type ExpressionFactSink interface {
	SetExpressionFact(filePath string, line, col int, t *typeinfer.ResolvedType)
}

// ApplyResolvedExpressions writes every entry from results into sink.
// Called after the targeted-resolution RPC completes and before the
// dispatcher runs, so rules see the new facts via their resolver.
func ApplyResolvedExpressions(sink ExpressionFactSink, results map[string]map[ExpressionPosition]*typeinfer.ResolvedType) {
	if sink == nil {
		return
	}
	for filePath, byPos := range results {
		for pos, t := range byPos {
			if t == nil {
				continue
			}
			sink.SetExpressionFact(filePath, pos.Line, pos.Col, t)
		}
	}
}

// sortPositions orders by line, then col. Stable across runs so RPC
// payloads and assertions are deterministic.
func sortPositions(s []ExpressionPosition) {
	sort.Slice(s, func(i, j int) bool { return positionLess(s[i], s[j]) })
}

func positionLess(a, b ExpressionPosition) bool {
	if a.Line != b.Line {
		return a.Line < b.Line
	}
	return a.Col < b.Col
}
